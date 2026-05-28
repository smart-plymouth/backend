import re
import logging
from datetime import datetime, timezone

import requests
from bs4 import BeautifulSoup

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


def _parse_page(html):
    """Parse the wait times page and return a list of dicts with location data."""
    soup = BeautifulSoup(html, "html.parser")
    results = []

    # Each location is in an h2 heading followed by stat blocks
    headings = soup.find_all("h2")

    for heading in headings:
        heading_text = heading.get_text(strip=True)
        if heading_text not in HEADING_TO_LOCATION:
            continue

        location_name = HEADING_TO_LOCATION[heading_text]

        # Find the containing section - stats follow the h2
        # Walk siblings until next h2 or end
        longest_wait = 0
        patients_waiting = 0
        patients_in_department = 0

        sibling = heading.find_next_sibling()
        while sibling and sibling.name != "h2":
            text = sibling.get_text(" ", strip=True)

            if "longest waiting time" in text.lower():
                # The number is typically in the next element or same block
                longest_wait = _parse_int(text)
            elif "patients waiting to be seen" in text.lower():
                patients_waiting = _parse_int(text)
            elif "patients in the department" in text.lower():
                patients_in_department = _parse_int(text)

            sibling = sibling.find_next_sibling()

        results.append({
            "location_name": location_name,
            "longest_wait": longest_wait,
            "patients_waiting": patients_waiting,
            "patients_in_department": patients_in_department,
        })

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
