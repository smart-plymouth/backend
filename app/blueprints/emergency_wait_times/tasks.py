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


def _extract_section_text(soup, h2_tag, next_h2_tag):
    """Extract all text between two h2 tags as a single string."""
    texts = []
    current = h2_tag.next_element
    while current:
        if current == next_h2_tag:
            break
        if hasattr(current, "get_text"):
            # Skip if this is the next_h2_tag or contains it
            if next_h2_tag and current == next_h2_tag:
                break
        if isinstance(current, str):
            texts.append(current.strip())
        current = current.next_element
        # Safety: also break if we encounter the next h2 as a parent
        if next_h2_tag and hasattr(current, "name") and current == next_h2_tag:
            break
    return " ".join(t for t in texts if t)


def _parse_page(html):
    """Parse the wait times page and return a list of dicts with location data."""
    soup = BeautifulSoup(html, "html.parser")
    results = []

    # Find all h2 tags that match our known locations
    all_h2 = soup.find_all("h2")
    location_h2s = []
    for h2 in all_h2:
        text = h2.get_text(strip=True)
        if text in HEADING_TO_LOCATION:
            location_h2s.append(h2)

    for i, h2 in enumerate(location_h2s):
        heading_text = h2.get_text(strip=True)
        next_h2 = location_h2s[i + 1] if i + 1 < len(location_h2s) else None

        # Get the full text of this section
        section_text = _extract_section_text(soup, h2, next_h2)

        # Parse stats using regex on the section text
        # Pattern: number followed by "minutes" for longest wait
        longest_match = re.search(r"(\d+)\s*minutes", section_text)
        longest_wait = int(longest_match.group(1)) if longest_match else 0

        # Pattern: number followed by "patients" - first occurrence is waiting,
        # second is in department
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
