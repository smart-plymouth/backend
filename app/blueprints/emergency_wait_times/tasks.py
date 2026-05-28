import re
import logging
from datetime import datetime, timezone

import requests
from bs4 import BeautifulSoup, NavigableString

from app.celery_app import celery

logger = logging.getLogger(__name__)

# Mapping from page headings to location names in the database
HEADING_TO_LOCATION = {
    "Emergency Department": "Emergency Department",
    "UTC Dartmoor Building (Derriford)": "UTC Dartmoor",
    "UTC Cumberland Centre": "UTC Cumberland Centre",
    "MIU Tavistock": "MIU Tavistock",
    "MIU Kingsbridge (South Hams)": "MIU Kingsbridge",
}

WAIT_TIMES_URL = "https://www.plymouthhospitals.nhs.uk/urgent-waiting-times/"


def _parse_int(text):
    """Extract the first integer from a string, return 0 if none found."""
    match = re.search(r"(\d+)", text)
    return int(match.group(1)) if match else 0


def _get_section_html(full_html, heading_text, all_headings, index):
    """
    Extract the raw HTML between this heading and the next known heading.
    This avoids DOM traversal issues by working with the raw HTML string.
    """
    # Find the position of this heading in the HTML
    # Use a pattern that matches the h2 containing this text
    heading_pattern = re.compile(
        r"<h2[^>]*>.*?" + re.escape(heading_text) + r".*?</h2>",
        re.DOTALL | re.IGNORECASE,
    )
    match = heading_pattern.search(full_html)
    if not match:
        return ""

    start_pos = match.end()

    # Find the position of the next heading
    end_pos = len(full_html)
    if index + 1 < len(all_headings):
        next_heading_text = all_headings[index + 1]
        next_pattern = re.compile(
            r"<h2[^>]*>.*?" + re.escape(next_heading_text) + r".*?</h2>",
            re.DOTALL | re.IGNORECASE,
        )
        next_match = next_pattern.search(full_html, start_pos)
        if next_match:
            end_pos = next_match.start()

    return full_html[start_pos:end_pos]


def _parse_page(html):
    """Parse the wait times page and return a list of dicts with location data."""
    soup = BeautifulSoup(html, "html.parser")
    results = []

    # Determine which headings are present on the page
    all_h2 = soup.find_all("h2")
    found_headings = []
    for h2 in all_h2:
        text = h2.get_text(strip=True)
        if text in HEADING_TO_LOCATION:
            found_headings.append(text)

    for i, heading_text in enumerate(found_headings):
        # Extract the HTML section for this location
        section_html = _get_section_html(html, heading_text, found_headings, i)

        # Get plain text from the section HTML
        section_soup = BeautifulSoup(section_html, "html.parser")
        section_text = section_soup.get_text(" ", strip=True)

        # Parse stats using regex on the section text
        # Pattern: number followed by "minutes" (with possible whitespace)
        longest_match = re.search(r"(\d+)\s*minutes", section_text)
        longest_wait = int(longest_match.group(1)) if longest_match else 0

        # Pattern: number followed by "patients" - first is waiting, second is in dept
        patient_matches = re.findall(r"(\d+)\s*patients", section_text)
        patients_waiting = int(patient_matches[0]) if len(patient_matches) > 0 else 0
        patients_in_department = int(patient_matches[1]) if len(patient_matches) > 1 else 0

        location_name = HEADING_TO_LOCATION[heading_text]
        results.append({
            "location_name": location_name,
            "longest_wait": longest_wait,
            "patients_waiting": patients_waiting,
            "patients_in_department": patients_in_department,
        })

        logger.info(
            f"Parsed {location_name}: wait={longest_wait}min, "
            f"waiting={patients_waiting}, in_dept={patients_in_department}"
        )

    return results


@celery.task
def fetch_wait_times():
    """Fetch latest wait times from UHP website and store in the database."""
    from app import create_app, db
    from app.blueprints.emergency_wait_times.models import Location, WaitTime

    app = create_app()
    with app.app_context():
        try:
            response = requests.get(WAIT_TIMES_URL, timeout=30)
            response.raise_for_status()
        except requests.RequestException as e:
            logger.error(f"Failed to fetch wait times page: {e}")
            return {"status": "error", "message": str(e)}

        parsed = _parse_page(response.text)
        now = datetime.now(timezone.utc)

        if not parsed:
            logger.warning("No locations parsed from page - HTML structure may have changed")
            return {"status": "warning", "message": "No data parsed", "records_added": 0}

        locations = {loc.name: loc for loc in Location.query.all()}
        records_added = 0

        for entry in parsed:
            location = locations.get(entry["location_name"])
            if not location:
                logger.warning(f"Location not found in DB: {entry['location_name']}")
                continue

            wait_time = WaitTime(
                location_id=location.id,
                timestamp=now,
                longest_wait=entry["longest_wait"],
                patients_waiting=entry["patients_waiting"],
                patients_in_department=entry["patients_in_department"],
            )
            db.session.add(wait_time)
            records_added += 1

        db.session.commit()
        logger.info(f"Added {records_added} wait time records")
        return {"status": "ok", "records_added": records_added}
