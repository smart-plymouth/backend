import logging
from datetime import date, timedelta

import requests
from bs4 import BeautifulSoup

from app.celery_app import celery

logger = logging.getLogger(__name__)

WEEKLY_LIST_URL = (
    "https://planning.plymouth.gov.uk/online-applications/search.do"
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

    The Idox Public Access system uses a two-step process:
    1. GET the search form to establish a session
    2. POST the form with the week date to get results
    """
    # Step 1: Load the search form to get cookies/session
    form_url = f"{WEEKLY_LIST_URL}?action=weeklyList"
    session.get(form_url, timeout=30)

    # Step 2: Submit the weekly list search
    # The form posts back to the same URL with the week start date
    form_data = {
        "searchType": "Application",
        "week": week_start.strftime("%d+%b+%Y"),
        "dateType": "DC_Validated",
        "action": "firstPage",
    }

    response = session.post(
        f"{WEEKLY_LIST_URL}?action=firstPage",
        data=form_data,
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
            f"{WEEKLY_LIST_URL}?action=page&searchCriteria.page={page}",
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
        # Reference number is typically in an anchor tag within the result
        ref_link = result_element.find("a")
        if not ref_link:
            return None

        reference = ref_link.get_text(strip=True)

        # Address and description are in separate spans/paragraphs
        # The structure typically has: reference, address, proposal, status
        address_el = result_element.find("p", class_="address")
        address = address_el.get_text(strip=True) if address_el else ""

        proposal_el = result_element.find("p", class_="description")
        proposal = proposal_el.get_text(strip=True) if proposal_el else ""

        status_el = result_element.find("span", class_="status")
        status = status_el.get_text(strip=True) if status_el else "Pending"

        # Try to extract dates from metadata spans
        received_date = None
        validated_date = None

        meta_items = result_element.find_all("span", class_="metaInfo")
        for meta in meta_items:
            text = meta.get_text(strip=True)
            if "Received:" in text:
                date_str = text.replace("Received:", "").strip()
                received_date = _parse_date(date_str)
            elif "Validated:" in text:
                date_str = text.replace("Validated:", "").strip()
                validated_date = _parse_date(date_str)

        if not reference:
            return None

        return {
            "reference": reference,
            "address": address,
            "proposal": proposal,
            "status": status,
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
        session.headers.update({
            "User-Agent": (
                "SmartPlymouth/1.0 "
                "(https://github.com/SmartPlymouth; community project)"
            ),
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
