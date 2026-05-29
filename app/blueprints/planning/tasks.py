import logging
from datetime import date, timedelta

import requests
from bs4 import BeautifulSoup

from app.celery_app import celery

logger = logging.getLogger(__name__)

WEEKLY_LIST_URL = (
    "https://planning.plymouth.gov.uk/online-applications/search.do"
)
WEEKLY_LIST_RESULTS_URL = (
    "https://planning.plymouth.gov.uk/online-applications/weeklyListResults.do"
)
PAGED_RESULTS_URL = (
    "https://planning.plymouth.gov.uk/online-applications/pagedSearchResults.do"
)


def _get_previous_week_monday(today=None):
    """Return the Monday of the previous week relative to today.

    When run on Monday 25th May 2026, returns Monday 18th May 2026.
    """
    if today is None:
        today = date.today()
    # today.weekday(): Monday=0, Sunday=6
    # Go back to the previous Monday (7 days before this Monday)
    days_since_monday = today.weekday()
    this_monday = today - timedelta(days=days_since_monday)
    previous_monday = this_monday - timedelta(weeks=1)
    return previous_monday


def _fetch_weekly_list_page(week_start, session):
    """Submit the weekly list form and return the results page HTML.

    The Idox Public Access system requires:
    1. A GET to the form page to establish a JSESSIONID cookie
    2. Extract the _csrf token from the form
    3. POST to weeklyListResults.do with the CSRF token and form data
    """
    # Step 1: Load the search form to establish session and get CSRF token
    form_url = f"{WEEKLY_LIST_URL}?action=weeklyList"
    form_response = session.get(form_url, timeout=30)
    form_response.raise_for_status()

    # Step 2: Extract the CSRF token from the form
    soup = BeautifulSoup(form_response.text, "html.parser")
    csrf_input = soup.find("input", {"name": "_csrf"})
    csrf_token = csrf_input["value"] if csrf_input else ""

    # Step 3: Submit the weekly list search form
    # Date format is "DD Mon YYYY" (e.g. "18 May 2026")
    form_data = {
        "_csrf": csrf_token,
        "searchCriteria.ward": "",
        "week": week_start.strftime("%d %b %Y"),
        "dateType": "DC_Validated",
        "searchType": "Application",
    }

    response = session.post(
        f"{WEEKLY_LIST_RESULTS_URL}?action=firstPage",
        data=form_data,
        headers={
            "Referer": form_url,
            "Origin": "https://planning.plymouth.gov.uk",
        },
        timeout=30,
    )
    response.raise_for_status()
    return response.text


def _fetch_all_pages(week_start, session):
    """Fetch all pages of results for the weekly list."""
    all_html = []

    # Get first page
    html = _fetch_weekly_list_page(week_start, session)
    all_html.append(html)

    # Check for additional pages and fetch them
    page = 2
    while True:
        soup = BeautifulSoup(html, "html.parser")
        # Look for a "next" page link
        next_link = soup.find("a", class_="next")
        if not next_link:
            break

        response = session.get(
            f"{PAGED_RESULTS_URL}?action=page&searchCriteria.page={page}",
            headers={
                "Referer": f"{WEEKLY_LIST_RESULTS_URL}?action=firstPage",
            },
            timeout=30,
        )
        response.raise_for_status()
        html = response.text
        all_html.append(html)
        page += 1

        # Safety limit to avoid infinite loops
        if page > 50:
            logger.warning("Reached page limit of 50, stopping pagination")
            break

    return all_html


def _parse_results(html_pages):
    """Parse planning application results from the search results pages.

    The Idox Public Access results page contains a list of applications
    with reference, address, proposal/description, and status.
    """
    cases = []

    for html in html_pages:
        soup = BeautifulSoup(html, "html.parser")

        # Results are in li elements with class 'searchresult'
        results = soup.find_all("li", class_="searchresult")

        for result in results:
            case = _parse_single_result(result)
            if case:
                cases.append(case)

    return cases


def _parse_single_result(result_element):
    """Parse a single search result element into a case dict."""
    try:
        # Proposal/description is in the summary link div
        summary_link = result_element.find("a", class_="summaryLink")
        proposal = ""
        if summary_link:
            div = summary_link.find("div")
            proposal = div.get_text(strip=True) if div else summary_link.get_text(strip=True)

        # Address is in <p class="address">
        address_el = result_element.find("p", class_="address")
        address = address_el.get_text(strip=True) if address_el else ""

        # Reference, dates, and status are all in <p class="metaInfo">
        meta_el = result_element.find("p", class_="metaInfo")
        reference = ""
        received_date = None
        validated_date = None
        status = ""

        if meta_el:
            meta_text = meta_el.get_text(" ", strip=True)

            # Extract reference number (after "Ref. No:")
            import re

            ref_match = re.search(r"Ref\.\s*No:\s*([\w/]+)", meta_text)
            if ref_match:
                reference = ref_match.group(1).strip()

            # Extract received date
            recv_match = re.search(
                r"Received:\s*\w+\s+(\d{1,2}\s+\w+\s+\d{4})", meta_text
            )
            if recv_match:
                received_date = _parse_date(recv_match.group(1))

            # Extract validated date
            val_match = re.search(
                r"Validated:\s*\w+\s+(\d{1,2}\s+\w+\s+\d{4})", meta_text
            )
            if val_match:
                validated_date = _parse_date(val_match.group(1))

            # Extract status (after "Status:")
            status_match = re.search(r"Status:\s*(.+?)(?:\s*\||\s*$)", meta_text)
            if status_match:
                status = status_match.group(1).strip()

        if not reference:
            return None

        return {
            "reference": reference,
            "address": address,
            "proposal": proposal,
            "status": status or "Pending",
            "received_date": received_date,
            "validated_date": validated_date,
        }
    except Exception as e:
        logger.warning(f"Failed to parse result element: {e}")
        return None


def _parse_date(date_str):
    """Parse a date string in common UK formats (DD Mon YYYY or DD/MM/YYYY)."""
    from datetime import datetime

    formats = ["%d %b %Y", "%d/%m/%Y", "%d %B %Y"]
    for fmt in formats:
        try:
            return datetime.strptime(date_str, fmt).date()
        except ValueError:
            continue
    return None


@celery.task
def fetch_weekly_planning_applications(week_start_iso=None):
    """Fetch validated planning applications for a given week and store them.

    Args:
        week_start_iso: ISO date string (YYYY-MM-DD) for the Monday of the
            week to fetch. If None, defaults to the previous Monday.

    Scheduled to run every Monday. Can also be triggered manually via the API.
    """
    from app import create_app, db
    from app.blueprints.planning.models import PlanningCase

    app = create_app()
    with app.app_context():
        if week_start_iso:
            week_start = date.fromisoformat(week_start_iso)
        else:
            week_start = _get_previous_week_monday()
        logger.info(
            f"Fetching planning applications for week beginning {week_start}"
        )

        session = requests.Session()
        session.verify = False
        session.headers.update({
            "User-Agent": (
                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
                "AppleWebKit/537.36 (KHTML, like Gecko) "
                "Chrome/125.0.0.0 Safari/537.36"
            ),
            "Accept": (
                "text/html,application/xhtml+xml,application/xml;"
                "q=0.9,image/webp,*/*;q=0.8"
            ),
            "Accept-Language": "en-GB,en;q=0.9",
        })

        try:
            html_pages = _fetch_all_pages(week_start, session)
        except requests.RequestException as e:
            logger.error(f"Failed to fetch weekly planning list: {e}")
            return {"status": "error", "message": str(e)}

        cases = _parse_results(html_pages)

        if not cases:
            logger.warning(
                "No planning applications parsed - page structure may have changed"
            )
            return {
                "status": "warning",
                "message": "No applications parsed",
                "week_start": week_start.isoformat(),
                "cases_added": 0,
                "cases_updated": 0,
            }

        cases_added = 0
        cases_updated = 0

        for case_data in cases:
            existing = PlanningCase.query.get(case_data["reference"])
            if existing:
                # Update existing record
                existing.address = case_data["address"] or existing.address
                existing.proposal = case_data["proposal"] or existing.proposal
                existing.status = case_data["status"] or existing.status
                existing.received_date = (
                    case_data["received_date"] or existing.received_date
                )
                existing.validated_date = (
                    case_data["validated_date"] or existing.validated_date
                )
                cases_updated += 1
            else:
                new_case = PlanningCase(
                    reference=case_data["reference"],
                    address=case_data["address"] or "Unknown",
                    proposal=case_data["proposal"] or "No description available",
                    status=case_data["status"] or "Pending",
                    received_date=case_data["received_date"],
                    validated_date=case_data["validated_date"],
                )
                db.session.add(new_case)
                cases_added += 1

        db.session.commit()
        logger.info(
            f"Planning applications: {cases_added} added, {cases_updated} updated "
            f"for week beginning {week_start}"
        )

        return {
            "status": "ok",
            "week_start": week_start.isoformat(),
            "cases_added": cases_added,
            "cases_updated": cases_updated,
        }


@celery.task
def refresh_planning_applications():
    """Queue a fetch task for every week in the last 2 years.

    Runs daily to keep all cases up to date with status changes and
    any other modifications on the planning portal.
    """
    today = date.today()
    two_years_ago = today - timedelta(days=730)

    # Find the first Monday on or after two_years_ago
    days_until_monday = (7 - two_years_ago.weekday()) % 7
    current_monday = two_years_ago + timedelta(days=days_until_monday)

    weeks_queued = 0
    while current_monday <= today:
        fetch_weekly_planning_applications.delay(current_monday.isoformat())
        current_monday += timedelta(weeks=1)
        weeks_queued += 1

    logger.info(f"Queued {weeks_queued} weekly fetch tasks for the last 2 years")
    return {"status": "ok", "weeks_queued": weeks_queued}
