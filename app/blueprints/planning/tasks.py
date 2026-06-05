import json
import logging
import os
import shutil
import tempfile
from datetime import date, timedelta
from urllib.parse import urljoin

import requests
from bs4 import BeautifulSoup

from app.celery_app import celery
from app.config import Config

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
        new_references = []
        unanalysed_references = []

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
                # Queue analysis for updated cases that haven't been analysed
                if not existing.ai_analysis:
                    unanalysed_references.append(case_data["reference"])
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
                new_references.append(case_data["reference"])
                cases_added += 1

        db.session.commit()

        # Queue AI analysis for new cases and updated cases without analysis
        analysis_refs = new_references + unanalysed_references
        for ref in analysis_refs:
            analyse_planning_application.delay(ref)
        if analysis_refs:
            logger.info(
                f"Queued AI analysis for {len(new_references)} new and "
                f"{len(unanalysed_references)} unanalysed existing cases"
            )
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


# ---------------------------------------------------------------------------
# AI Analysis Task
# ---------------------------------------------------------------------------

PLANNING_APP_BASE_URL = (
    "https://planning.plymouth.gov.uk/online-applications/"
)
DOCUMENTS_TAB_URL = (
    "https://planning.plymouth.gov.uk/online-applications/"
    "applicationDetails.do?activeTab=documents&keyVal={key_val}"
)


def _get_application_key_val(reference, session):
    """Search for a planning application and extract its internal keyVal ID.

    The Idox system uses an internal keyVal parameter to identify applications.
    We need to load the search form first to establish a session, extract the
    CSRF token, then POST the search to get results.
    """
    import re
    from urllib.parse import quote

    # Step 1: Load the simple search form to establish session cookie and get CSRF
    form_url = f"{PLANNING_APP_BASE_URL}search.do?action=simple"
    form_response = session.get(form_url, timeout=30)
    form_response.raise_for_status()

    # Step 2: Extract CSRF token
    soup = BeautifulSoup(form_response.text, "html.parser")
    csrf_input = soup.find("input", {"name": "_csrf"})
    csrf_token = csrf_input["value"] if csrf_input else ""

    # Step 3: Submit the search as a POST with form data
    form_data = {
        "_csrf": csrf_token,
        "searchCriteria.reference": reference,
        "searchCriteria.planningPortalReference": "",
        "searchCriteria.alternativeReference": "",
        "searchType": "Application",
    }

    response = session.post(
        f"{PLANNING_APP_BASE_URL}simpleSearchResults.do?action=firstPage",
        data=form_data,
        headers={
            "Referer": form_url,
            "Origin": "https://planning.plymouth.gov.uk",
        },
        timeout=30,
    )
    response.raise_for_status()

    soup = BeautifulSoup(response.text, "html.parser")

    # Try to find the application link in search results
    link = soup.find("a", href=re.compile(r"keyVal="))
    if link:
        href = link.get("href", "")
        match = re.search(r"keyVal=([A-Z0-9_]+)", href)
        if match:
            return match.group(1)

    # If we landed directly on the application page (single result redirect)
    key_match = re.search(r"keyVal=([A-Z0-9_]+)", response.url)
    if key_match:
        return key_match.group(1)

    return None


def _collect_application_metadata(key_val, session):
    """Collect metadata from the application summary tab."""
    summary_url = (
        f"{PLANNING_APP_BASE_URL}applicationDetails.do"
        f"?activeTab=summary&keyVal={key_val}"
    )
    response = session.get(summary_url, timeout=30)
    response.raise_for_status()

    soup = BeautifulSoup(response.text, "html.parser")
    metadata = {}

    # Extract all table rows from the summary details
    rows = soup.find_all("tr")
    for row in rows:
        header = row.find("th")
        value = row.find("td")
        if header and value:
            key = header.get_text(strip=True).rstrip(":")
            val = value.get_text(strip=True)
            if key and val:
                metadata[key] = val

    return metadata


def _download_documents(key_val, session, download_dir):
    """Download all documents associated with a planning application.

    Returns a list of dicts with filename and local path for each document.
    """
    docs_url = (
        f"{PLANNING_APP_BASE_URL}applicationDetails.do"
        f"?activeTab=documents&keyVal={key_val}"
    )
    response = session.get(docs_url, timeout=30)
    response.raise_for_status()

    soup = BeautifulSoup(response.text, "html.parser")
    downloaded = []

    # Find document links - they typically point to /online-applications/files/
    doc_links = soup.find_all("a", href=True)
    for link in doc_links:
        href = link.get("href", "")
        if "/files/" not in href and "/document/" not in href:
            continue

        doc_url = href if href.startswith("http") else urljoin(
            "https://planning.plymouth.gov.uk", href
        )
        filename = link.get_text(strip=True) or os.path.basename(href)
        # Sanitise filename
        filename = "".join(
            c for c in filename if c.isalnum() or c in " ._-"
        ).strip()
        if not filename:
            filename = f"document_{len(downloaded)}"

        try:
            doc_response = session.get(doc_url, timeout=60, stream=True)
            doc_response.raise_for_status()

            # Determine file extension from content-type if not present
            if "." not in filename:
                content_type = doc_response.headers.get("Content-Type", "")
                if "pdf" in content_type:
                    filename += ".pdf"
                elif "image" in content_type:
                    filename += ".png"
                else:
                    filename += ".bin"

            filepath = os.path.join(download_dir, filename)
            with open(filepath, "wb") as f:
                for chunk in doc_response.iter_content(chunk_size=8192):
                    f.write(chunk)

            downloaded.append({
                "filename": filename,
                "path": filepath,
                "url": doc_url,
            })
            logger.debug(f"Downloaded: {filename}")
        except requests.RequestException as e:
            logger.warning(f"Failed to download document {doc_url}: {e}")

    return downloaded


def _extract_text_from_pdf(filepath):
    """Extract text content from a PDF file."""
    try:
        from pypdf import PdfReader

        reader = PdfReader(filepath)
        text_parts = []
        for page in reader.pages:
            text = page.extract_text()
            if text:
                text_parts.append(text)
        return "\n".join(text_parts)
    except Exception as e:
        logger.warning(f"Failed to extract text from {filepath}: {e}")
        return ""


def _ensure_ollama_model(base_url, model_name):
    """Check if the model exists on the Ollama server and pull it if not.

    Returns True if the model is available, False if it could not be obtained.
    """
    # Check if model exists
    try:
        response = requests.get(f"{base_url}/api/tags", timeout=30)
        response.raise_for_status()
        models = response.json().get("models", [])
        model_names = [m.get("name", "").split(":")[0] for m in models]
        if model_name in model_names or f"{model_name}:latest" in [
            m.get("name", "") for m in models
        ]:
            logger.info(f"Model '{model_name}' already available on Ollama server")
            return True
    except requests.RequestException as e:
        logger.error(f"Failed to check Ollama models: {e}")
        return False

    # Model not found, pull it
    logger.info(f"Model '{model_name}' not found, pulling from Ollama...")
    try:
        pull_response = requests.post(
            f"{base_url}/api/pull",
            json={"name": model_name},
            timeout=600,  # Model downloads can take a while
            stream=True,
        )
        pull_response.raise_for_status()
        # Consume the stream to wait for completion
        for line in pull_response.iter_lines():
            if line:
                status = json.loads(line)
                if status.get("status") == "success":
                    logger.info(f"Model '{model_name}' pulled successfully")
                    return True
        # If we got here without error, assume success
        return True
    except requests.RequestException as e:
        logger.error(f"Failed to pull model '{model_name}': {e}")
        return False


def _run_ai_analysis(metadata, document_texts, reference):
    """Run AI analysis using LangChain with Ollama (deepseek-r1).

    Returns a dict with potential_impact_score, estimated_size, and tags.
    """
    from langchain_ollama import OllamaLLM
    from langchain_core.prompts import ChatPromptTemplate

    llm = OllamaLLM(
        model=Config.OLLAMA_MODEL,
        base_url=Config.OLLAMA_BASE_URL,
        temperature=0.1,
    )

    # Build context from metadata and documents
    metadata_text = "\n".join(f"- {k}: {v}" for k, v in metadata.items())

    # Truncate document text to avoid overwhelming the model
    combined_docs = "\n\n---\n\n".join(document_texts[:10])  # Max 10 documents
    if len(combined_docs) > 15000:
        combined_docs = combined_docs[:15000] + "\n\n[... truncated ...]"

    prompt = ChatPromptTemplate.from_messages([
        ("system", (
            "You are an expert planning analyst. "
            "Analyse the following planning application and provide a structured assessment. "
            "You must respond with ONLY valid JSON, no other text or explanation.\n\n"
            "The JSON must have exactly these fields:\n"
            "- potential_impact_score: integer 1-10 (1=minimal impact, 10=transformative/major impact)\n"
            "- estimated_size: integer 1-10 (1=very small e.g. minor alteration, 10=massive e.g. large housing estate)\n"
            "- tags: array of lowercase string tags describing the application "
            "(e.g. residential, commercial, change-of-use, demolition, new-build, extension, "
            "listed-building, conservation-area, HMO, retail, industrial, infrastructure)\n"
            "- rationalisation: a string containing several paragraphs explaining WHY you "
            "chose the given impact score and size score. Justify your reasoning by referencing "
            "specific details from the application metadata and documents. Explain what factors "
            "led to the scores and why they are appropriate for this type of application.\n"
            "- pros: an array of strings listing the positive aspects and benefits of the "
            "application (e.g. new housing supply, job creation, improved accessibility, "
            "energy efficiency, regeneration of derelict land, community facilities). "
            "Each entry should be a concise statement of one benefit.\n"
            "- cons: an array of strings listing the negative aspects and drawbacks of the "
            "application (e.g. increased traffic, loss of green space, overlooking neighbours, "
            "noise during construction, strain on local services, out of character with area). "
            "Each entry should be a concise statement of one drawback.\n\n"
            "## Scoring Rules (you MUST follow these strictly):\n\n"
            "### Minor homeowner changes (extensions, loft conversions, garages, porches, "
            "alterations to a single existing dwelling):\n"
            "- potential_impact_score: MUST be 1 or 2\n"
            "- estimated_size: MUST be 1 or 2\n"
            "- These are individual homeowners making changes to their own property. "
            "Do not over-score these.\n\n"
            "### Tree works (trimming, felling, pruning one or a small number of trees):\n"
            "- potential_impact_score: MUST be 1 or 2\n"
            "- estimated_size: MUST be 1 or 2\n\n"
            "### Construction of a single new dwelling:\n"
            "- potential_impact_score: MUST NOT exceed 3\n"
            "- estimated_size: MUST NOT exceed 3\n"
            "- NOTE: HMOs (Houses in Multiple Occupation) are NOT single dwellings. "
            "The single dwelling cap does NOT apply to HMOs. Treat HMOs as larger/complex applications.\n\n"
            "### Larger/complex applications (multiple residential units, large developments, "
            "commercial/business developments, HMOs, mixed-use schemes, infrastructure):\n"
            "- potential_impact_score: MUST start at minimum 2 or 3, no upper cap\n"
            "- estimated_size: MUST start at minimum 2 or 3, no upper cap\n"
            "- Score higher based on: number of dwellings, floor area, height, "
            "environmental impact, traffic generation, community impact, "
            "heritage considerations, and scale of construction.\n"
        )),
        ("human", (
            "Planning Application Reference: {reference}\n\n"
            "## Application Metadata\n{metadata}\n\n"
            "## Document Contents\n{documents}\n\n"
            "Provide your analysis as JSON only."
        )),
    ])

    chain = prompt | llm

    try:
        response = chain.invoke({
            "reference": reference,
            "metadata": metadata_text,
            "documents": combined_docs if combined_docs else "No documents available.",
        })

        # Parse the JSON response - handle potential markdown wrapping
        response_text = response.strip()
        # Remove markdown code fences if present
        if response_text.startswith("```"):
            lines = response_text.split("\n")
            # Remove first and last lines (``` markers)
            lines = [l for l in lines if not l.strip().startswith("```")]
            response_text = "\n".join(lines)

        # Try to extract JSON from the response
        # Sometimes the model wraps in <think> tags or adds preamble
        json_start = response_text.find("{")
        json_end = response_text.rfind("}") + 1
        if json_start != -1 and json_end > json_start:
            response_text = response_text[json_start:json_end]

        result = json.loads(response_text)

        # Validate and clamp values
        impact_score = max(1, min(10, int(result.get("potential_impact_score", 5))))
        size = max(1, min(10, int(result.get("estimated_size", 5))))
        tags = result.get("tags", [])
        if not isinstance(tags, list):
            tags = []
        tags = [str(t).lower().strip() for t in tags if t]
        rationalisation = result.get("rationalisation", "")
        if not isinstance(rationalisation, str):
            rationalisation = str(rationalisation)
        pros = result.get("pros", [])
        if not isinstance(pros, list):
            pros = []
        pros = [str(p).strip() for p in pros if p]
        cons = result.get("cons", [])
        if not isinstance(cons, list):
            cons = []
        cons = [str(c).strip() for c in cons if c]

        return {
            "potential_impact_score": impact_score,
            "estimated_size": size,
            "tags": tags,
            "ai_rationalisation": rationalisation,
            "pros": pros,
            "cons": cons,
        }
    except (json.JSONDecodeError, ValueError, KeyError) as e:
        logger.error(f"Failed to parse AI response for {reference}: {e}")
        logger.debug(f"Raw response: {response_text[:500] if 'response_text' in dir() else 'N/A'}")
        return None


def _load_policy_context(tags):
    """Load relevant policy extracts from the JLP and NPPF based on application tags.

    Extracts text from the policy PDFs and selects sections relevant to the
    application's characteristics (identified by tags). Returns a combined
    string of policy excerpts suitable for inclusion in an LLM prompt.
    """
    policy_dir = os.path.join(os.path.dirname(__file__), "..", "..", "..", "data", "policy")
    policy_dir = os.path.normpath(policy_dir)

    policy_texts = []

    # Define tag-to-keyword mappings for policy selection
    tag_keywords = {
        "residential": ["housing", "dwelling", "residential", "amenity", "DEV1", "DEV2", "DEV10"],
        "hmo": ["HMO", "houses in multiple occupation", "DEV11", "shared housing"],
        "commercial": ["employment", "economic", "commercial", "retail", "PLY30", "PLY31", "DEV15"],
        "retail": ["retail", "town centre", "PLY32", "DEV16"],
        "new-build": ["design", "layout", "density", "DEV20", "DEV23"],
        "extension": ["design", "amenity", "DEV1", "DEV2", "residential"],
        "conservation-area": ["conservation", "heritage", "historic", "DEV21", "DEV22"],
        "listed-building": ["listed building", "heritage", "historic", "DEV21", "DEV22"],
        "demolition": ["demolition", "conservation", "heritage", "DEV21"],
        "change-of-use": ["change of use", "mixed use", "employment", "DEV15"],
        "infrastructure": ["infrastructure", "transport", "DEV31", "DEV32"],
        "industrial": ["industrial", "employment", "PLY30", "DEV15"],
        "flood-risk": ["flood", "drainage", "water", "DEV35", "DEV37"],
        "ecology": ["biodiversity", "ecology", "wildlife", "habitat", "DEV26", "DEV28"],
        "green-space": ["green space", "open space", "recreation", "DEV27"],
        "transport": ["transport", "traffic", "parking", "highway", "DEV29", "DEV31"],
    }

    # Build a set of keywords from the application tags
    keywords = set()
    # Always include general design and amenity policies
    keywords.update(["design", "amenity", "DEV1", "DEV2", "DEV20", "sustainable development"])
    for tag in tags:
        tag_lower = tag.lower()
        if tag_lower in tag_keywords:
            keywords.update(tag_keywords[tag_lower])

    # Extract relevant sections from both PDFs
    for pdf_name, doc_label in [
        ("NPPF_December_2024.pdf", "NPPF"),
        ("Plymouth_SW_Devon_JLP_2019.pdf", "JLP"),
    ]:
        pdf_path = os.path.join(policy_dir, pdf_name)
        if not os.path.exists(pdf_path):
            logger.warning(f"Policy PDF not found: {pdf_path}")
            continue

        try:
            extracted = _extract_policy_sections(pdf_path, keywords, doc_label)
            if extracted:
                policy_texts.append(extracted)
        except Exception as e:
            logger.warning(f"Failed to extract policy from {pdf_name}: {e}")

    if not policy_texts:
        return "No policy context available."

    combined = "\n\n".join(policy_texts)
    # Cap total policy context to avoid overwhelming the model
    if len(combined) > 12000:
        combined = combined[:12000] + "\n\n[... policy context truncated ...]"

    return combined


def _extract_policy_sections(pdf_path, keywords, doc_label):
    """Extract pages from a policy PDF that contain any of the given keywords.

    Returns a formatted string with relevant page content labelled by document.
    """
    try:
        from pypdf import PdfReader

        reader = PdfReader(pdf_path)
        relevant_pages = []

        for i, page in enumerate(reader.pages):
            text = page.extract_text()
            if not text:
                continue

            text_lower = text.lower()
            # Check if any keyword appears on this page
            if any(kw.lower() in text_lower for kw in keywords):
                relevant_pages.append((i + 1, text))

            # Limit how many pages we extract to keep context manageable
            if len(relevant_pages) >= 15:
                break

        if not relevant_pages:
            return ""

        sections = [f"### {doc_label} - Relevant Policy Extracts\n"]
        for page_num, text in relevant_pages:
            # Trim excessively long pages
            if len(text) > 2000:
                text = text[:2000] + "..."
            sections.append(f"[{doc_label} Page {page_num}]\n{text}\n")

        return "\n".join(sections)

    except Exception as e:
        logger.warning(f"Failed to read PDF {pdf_path}: {e}")
        return ""


def _generate_objections(metadata, document_texts, reference, analysis):
    """Generate potential reasons for objection using AI.

    Only called when impact or size score is 3 or greater.
    Returns a list of dicts with 'objection' and 'ai_rationalisation' keys.

    Includes relevant policy context from the Plymouth & South West Devon
    Joint Local Plan (JLP) and the National Planning Policy Framework (NPPF).
    """
    from langchain_ollama import OllamaLLM
    from langchain_core.prompts import ChatPromptTemplate

    llm = OllamaLLM(
        model=Config.OLLAMA_MODEL,
        base_url=Config.OLLAMA_BASE_URL,
        temperature=0.2,
    )

    metadata_text = "\n".join(f"- {k}: {v}" for k, v in metadata.items())

    combined_docs = "\n\n---\n\n".join(document_texts[:10])
    if len(combined_docs) > 15000:
        combined_docs = combined_docs[:15000] + "\n\n[... truncated ...]"

    # Load relevant policy context from JLP and NPPF
    policy_context = _load_policy_context(analysis.get("tags", []))

    prompt = ChatPromptTemplate.from_messages([
        ("system", (
            "You are an expert planning analyst. "
            "A planning application has been assessed with a high impact or size score. "
            "Your task is to identify potential legitimate grounds for objection that "
            "members of the public or community groups might raise.\n\n"
            "You have been provided with relevant policy extracts from:\n"
            "- The Plymouth and South West Devon Joint Local Plan (JLP, adopted 2019)\n"
            "- The National Planning Policy Framework (NPPF, December 2024)\n\n"
            "When generating objections, you MUST reference specific policy numbers "
            "where applicable (e.g. 'JLP Policy DEV1', 'NPPF paragraph 135'). "
            "Ground your objections in the policy framework.\n\n"
            "You must respond with ONLY valid JSON, no other text or explanation.\n\n"
            "The JSON must be an array of objects, each with exactly these fields:\n"
            "- objection: a concise statement of the grounds for objection "
            "(e.g. 'Increased traffic congestion on residential streets')\n"
            "- ai_rationalisation: one or two paragraphs explaining WHY this is a "
            "valid potential objection, referencing specific details from the application "
            "metadata, documents, and relevant planning policies (JLP/NPPF). "
            "Cite specific policy numbers where applicable.\n\n"
            "## Guidelines:\n"
            "- Only include legitimate planning grounds for objection (not personal preferences)\n"
            "- Valid grounds include: traffic/parking impact, overlooking/privacy, "
            "noise/disturbance, visual impact, loss of light, impact on conservation areas, "
            "flood risk, strain on local infrastructure, loss of green space, "
            "overdevelopment, impact on wildlife/ecology, heritage concerns, "
            "inadequate amenity space, out of character with area, conflict with "
            "JLP or NPPF policies\n"
            "- Do NOT include objections based on: property values, competition with "
            "existing businesses, personal disputes, or loss of private views\n"
            "- Provide between 1 and 5 objections depending on the complexity of the application\n"
            "- Each objection should be distinct and address a different concern\n"
            "- Where possible, tie each objection to a specific JLP or NPPF policy\n"
        )),
        ("human", (
            "Planning Application Reference: {reference}\n\n"
            "## AI Assessment\n"
            "- Impact Score: {impact_score}/10\n"
            "- Size Score: {size_score}/10\n"
            "- Tags: {tags}\n\n"
            "## Application Metadata\n{metadata}\n\n"
            "## Document Contents\n{documents}\n\n"
            "## Relevant Planning Policy Context\n{policy_context}\n\n"
            "Provide potential grounds for objection as a JSON array."
        )),
    ])

    chain = prompt | llm

    try:
        response = chain.invoke({
            "reference": reference,
            "impact_score": analysis["potential_impact_score"],
            "size_score": analysis["estimated_size"],
            "tags": ", ".join(analysis["tags"]),
            "metadata": metadata_text,
            "documents": combined_docs if combined_docs else "No documents available.",
            "policy_context": policy_context,
        })

        response_text = response.strip()
        # Remove markdown code fences if present
        if response_text.startswith("```"):
            lines = response_text.split("\n")
            lines = [l for l in lines if not l.strip().startswith("```")]
            response_text = "\n".join(lines)

        # Extract JSON array from response
        json_start = response_text.find("[")
        json_end = response_text.rfind("]") + 1
        if json_start != -1 and json_end > json_start:
            response_text = response_text[json_start:json_end]

        objections = json.loads(response_text)

        if not isinstance(objections, list):
            logger.error(f"Objections response is not a list for {reference}")
            return []

        valid_objections = []
        for obj in objections:
            if isinstance(obj, dict) and "objection" in obj and "ai_rationalisation" in obj:
                valid_objections.append({
                    "objection": str(obj["objection"]).strip(),
                    "ai_rationalisation": str(obj["ai_rationalisation"]).strip(),
                })

        return valid_objections

    except (json.JSONDecodeError, ValueError, KeyError) as e:
        logger.error(f"Failed to parse objections response for {reference}: {e}")
        return []


@celery.task(queue="planning_analysis")
def analyse_planning_application(reference):
    """Analyse a planning application using AI.

    This task:
    1. Creates a temporary working directory
    2. Visits the planning portal for the given application
    3. Downloads all associated documents
    4. Collects metadata from the portal
    5. Uses LangChain + Ollama (deepseek-r1) to analyse the application
    6. Scores the application for scale and potential impact
    7. Extracts descriptive tags

    Concurrency is limited to 1 to avoid overwhelming the Ollama server.

    Args:
        reference: The planning application reference (e.g. "26/00747/CDM")
    """
    from app import create_app, db
    from app.blueprints.planning.models import PlanningCase

    app = create_app()
    with app.app_context():
        case = PlanningCase.query.get(reference)
        if not case:
            logger.error(f"Planning case {reference} not found in database")
            return {"status": "error", "message": f"Case {reference} not found"}

        # Create temporary working directory
        tmp_dir = tempfile.mkdtemp(prefix="smartplymouth_planning_")
        logger.info(
            f"Analysing planning application {reference} "
            f"(working dir: {tmp_dir})"
        )

        try:
            # Set up HTTP session for portal access
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

            # Step 1: Find the application's internal key
            key_val = _get_application_key_val(reference, session)
            if not key_val:
                logger.error(
                    f"Could not find keyVal for application {reference}"
                )
                return {
                    "status": "error",
                    "message": f"Application {reference} not found on portal",
                }

            # Step 2: Collect metadata from the portal
            logger.info(f"Collecting metadata for {reference}")
            metadata = _collect_application_metadata(key_val, session)

            # Step 3: Download all associated documents
            logger.info(f"Downloading documents for {reference}")
            download_dir = os.path.join(tmp_dir, "documents")
            os.makedirs(download_dir, exist_ok=True)
            documents = _download_documents(key_val, session, download_dir)
            logger.info(
                f"Downloaded {len(documents)} documents for {reference}"
            )

            # Step 4: Extract text from PDF documents
            document_texts = []
            for doc in documents:
                if doc["path"].lower().endswith(".pdf"):
                    text = _extract_text_from_pdf(doc["path"])
                    if text:
                        document_texts.append(
                            f"[{doc['filename']}]\n{text}"
                        )

            # Also include the proposal and metadata as context
            if case.proposal:
                document_texts.insert(
                    0, f"[Application Proposal]\n{case.proposal}"
                )

            # Step 5: Ensure the Ollama model is available
            logger.info("Checking Ollama model availability...")
            model_ready = _ensure_ollama_model(
                Config.OLLAMA_BASE_URL, Config.OLLAMA_MODEL
            )
            if not model_ready:
                return {
                    "status": "error",
                    "message": (
                        f"Ollama model '{Config.OLLAMA_MODEL}' is not available "
                        f"and could not be pulled"
                    ),
                }

            # Step 6: Run AI analysis
            logger.info(f"Running AI analysis for {reference}")
            analysis = _run_ai_analysis(metadata, document_texts, reference)

            if analysis is None:
                return {
                    "status": "error",
                    "message": "AI analysis failed to produce valid results",
                }

            # Step 7: Update the database record
            case.ai_analysis = True
            case.potential_impact_score = analysis["potential_impact_score"]
            case.estimated_size = analysis["estimated_size"]
            case.tags = analysis["tags"]
            case.ai_rationalisation = analysis["ai_rationalisation"]
            case.pros = analysis["pros"]
            case.cons = analysis["cons"]
            db.session.commit()

            # Step 8: Generate potential objections
            objections_generated = 0
            from app.blueprints.planning.models import PlanningObjection

            logger.info(
                f"Generating potential objections for {reference} "
                f"(impact={analysis['potential_impact_score']}, "
                f"size={analysis['estimated_size']})"
            )

            # Remove any existing objections for this case (re-analysis)
            PlanningObjection.query.filter_by(
                case_reference=reference
            ).delete()
            db.session.commit()

            objections = _generate_objections(
                metadata, document_texts, reference, analysis
            )

            for obj_data in objections:
                objection = PlanningObjection(
                    case_reference=reference,
                    objection=obj_data["objection"],
                    ai_rationalisation=obj_data["ai_rationalisation"],
                )
                db.session.add(objection)
                objections_generated += 1

            db.session.commit()
            logger.info(
                f"Generated {objections_generated} potential objections "
                f"for {reference}"
            )

            logger.info(
                f"Analysis complete for {reference}: "
                f"impact={analysis['potential_impact_score']}, "
                f"size={analysis['estimated_size']}, "
                f"tags={analysis['tags']}"
            )

            return {
                "status": "ok",
                "reference": reference,
                "potential_impact_score": analysis["potential_impact_score"],
                "estimated_size": analysis["estimated_size"],
                "tags": analysis["tags"],
                "documents_downloaded": len(documents),
                "documents_analysed": len(document_texts),
                "objections_generated": objections_generated,
            }

        except Exception as e:
            logger.exception(
                f"Unexpected error analysing application {reference}: {e}"
            )
            return {"status": "error", "message": str(e)}

        finally:
            # Clean up temporary directory
            shutil.rmtree(tmp_dir, ignore_errors=True)
            logger.debug(f"Cleaned up temporary directory: {tmp_dir}")
