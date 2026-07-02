from datetime import date, timedelta

from flask import jsonify, request
from sqlalchemy import cast, or_, String

from app.blueprints.planning import planning_bp
from app.blueprints.planning.models import PlanningCase, PlanningObjection, PlanningSupport
from app.config import Config


def _parse_date(value):
    """Parse an ISO 8601 date string (YYYY-MM-DD). Returns None on failure."""
    try:
        return date.fromisoformat(value)
    except (ValueError, TypeError):
        return None


@planning_bp.route("/cases", methods=["GET"])
def list_cases():
    """List planning cases with optional filtering and free text search.

    Query params:
        search         - free text search across proposal, address, dates, and tags
        status         - filter by status (partial match)
        validated_date - exact validated date (YYYY-MM-DD)
        validated_from - start of validated date range (inclusive)
        validated_to   - end of validated date range (inclusive)
        page           - page number (default 1)
        per_page       - results per page (default 25, max 100)
    """
    search = request.args.get("search")
    status = request.args.get("status")
    validated_date = request.args.get("validated_date")
    validated_from = request.args.get("validated_from")
    validated_to = request.args.get("validated_to")
    page = request.args.get("page", 1, type=int)
    per_page = request.args.get("per_page", 25, type=int)
    per_page = min(per_page, 100)  # cap at 100

    query = PlanningCase.query

    # Free text search across proposal, address, dates, and tags
    if search:
        like_term = f"%{search}%"
        query = query.filter(
            or_(
                PlanningCase.reference.ilike(like_term),
                PlanningCase.proposal.ilike(like_term),
                PlanningCase.address.ilike(like_term),
                cast(PlanningCase.received_date, String).ilike(like_term),
                cast(PlanningCase.validated_date, String).ilike(like_term),
                cast(PlanningCase.tags, String).ilike(like_term),
            )
        )

    if status:
        query = query.filter(PlanningCase.status.ilike(f"%{status}%"))

    # Exact validated date filter
    if validated_date:
        parsed = _parse_date(validated_date)
        if parsed is None:
            return jsonify({"error": "Invalid validated_date format. Use YYYY-MM-DD."}), 400
        query = query.filter(PlanningCase.validated_date == parsed)
    else:
        # Date range filters
        if validated_from:
            parsed = _parse_date(validated_from)
            if parsed is None:
                return jsonify({"error": "Invalid validated_from format. Use YYYY-MM-DD."}), 400
            query = query.filter(PlanningCase.validated_date >= parsed)

        if validated_to:
            parsed = _parse_date(validated_to)
            if parsed is None:
                return jsonify({"error": "Invalid validated_to format. Use YYYY-MM-DD."}), 400
            query = query.filter(PlanningCase.validated_date <= parsed)

    pagination = query.order_by(PlanningCase.validated_date.desc()).paginate(
        page=page, per_page=per_page, error_out=False
    )

    return jsonify({
        "cases": [case.to_dict() for case in pagination.items],
        "total": pagination.total,
        "page": pagination.page,
        "pages": pagination.pages,
        "per_page": pagination.per_page,
    })


'''@planning_bp.route("/fetch", methods=["POST"])
def trigger_fetch():
    """Trigger a task to fetch planning validations for a given week.

    Query params:
        date - a date (YYYY-MM-DD) within the week to fetch. The task will
               collect validations for the week containing this date
               (starting from Monday).
    """
    from app.blueprints.planning.tasks import fetch_weekly_planning_applications

    date_param = request.args.get("date")
    if not date_param:
        return jsonify({"error": "Missing required query parameter: date"}), 400

    parsed = _parse_date(date_param)
    if parsed is None:
        return jsonify({"error": "Invalid date format. Use YYYY-MM-DD."}), 400

    # Normalise to the Monday of the requested week
    days_since_monday = parsed.weekday()  # Monday=0
    week_monday = parsed - timedelta(days=days_since_monday)

    task = fetch_weekly_planning_applications.delay(week_monday.isoformat())

    return jsonify({
        "status": "accepted",
        "task_id": task.id,
        "week_start": week_monday.isoformat(),
    }), 202'''


@planning_bp.route("/refresh", methods=["POST"])
def trigger_refresh():
    """Trigger a full refresh of planning applications for the last 2 years."""
    from app.blueprints.planning.tasks import refresh_planning_applications

    task = refresh_planning_applications.delay()

    return jsonify({
        "status": "accepted",
        "task_id": task.id,
    }), 202


@planning_bp.route("/cases/<path:reference>", methods=["GET"])
def get_case(reference):
    """Get a single planning case by reference number.

    The reference contains forward slashes (e.g. 26/00747/CDM) so we use
    a path converter to capture the full value.
    """
    case = PlanningCase.query.get_or_404(reference)
    return jsonify(case.to_dict())


@planning_bp.route("/cases/<path:reference>/analyse", methods=["POST"])
def trigger_analysis(reference):
    """Trigger AI analysis for a specific planning application.

    Downloads documents from the planning portal, collects metadata,
    and uses deepseek-r1 via Ollama to assess scale and impact.
    """
    from app.blueprints.planning.tasks import analyse_planning_application

    case = PlanningCase.query.get_or_404(reference)

    task = analyse_planning_application.delay(reference)

    return jsonify({
        "status": "accepted",
        "task_id": task.id,
        "reference": reference,
    }), 202


@planning_bp.route("/cases/<path:reference>/objections", methods=["GET"])
def list_objections(reference):
    """List potential reasons for objection for a planning application.

    Only available for cases that have been AI-analysed and scored 5 or
    higher on impact or size.
    """
    case = PlanningCase.query.get_or_404(reference)

    objections = (
        PlanningObjection.query
        .filter_by(case_reference=reference)
        .order_by(PlanningObjection.created_at.desc())
        .all()
    )

    return jsonify({
        "reference": reference,
        "objections": [obj.to_dict() for obj in objections],
    })



@planning_bp.route("/cases/<path:reference>/supports", methods=["GET"])
def list_supports(reference):
    """List potential reasons for support for a planning application.

    Only available for cases that have been AI-analysed. Support reasons
    are backed by relevant planning policy references.
    """
    case = PlanningCase.query.get_or_404(reference)

    supports = (
        PlanningSupport.query
        .filter_by(case_reference=reference)
        .order_by(PlanningSupport.created_at.desc())
        .all()
    )

    return jsonify({
        "reference": reference,
        "supports": [s.to_dict() for s in supports],
    })


@planning_bp.route("/cases/<path:reference>/generate-letter", methods=["POST"])
def generate_letter(reference):
    """Generate an objection or support letter for a planning application.

    Synchronously calls the nscale LLM API to generate a formal letter
    based on the AI-generated objection or support reasons for the case.

    Request body (JSON):
        first_name  - Author's first name (required)
        last_name   - Author's last name (required)
        letter_type - Either "objection" or "support" (required)
    """
    from langchain_openai import ChatOpenAI
    from langchain_core.prompts import ChatPromptTemplate

    case = PlanningCase.query.get_or_404(reference)

    data = request.get_json()
    if not data:
        return jsonify({"error": "Request body must be JSON"}), 400

    first_name = data.get("first_name")
    last_name = data.get("last_name")
    letter_type = data.get("letter_type")

    if not first_name or not last_name:
        return jsonify({"error": "first_name and last_name are required"}), 400

    if letter_type not in ("objection", "support"):
        return jsonify({"error": "letter_type must be 'objection' or 'support'"}), 400

    # Gather the reasons for the letter
    if letter_type == "objection":
        records = (
            PlanningObjection.query
            .filter_by(case_reference=reference)
            .order_by(PlanningObjection.created_at.desc())
            .all()
        )
        if not records:
            return jsonify({
                "error": "No objection reasons available for this case. "
                         "Run AI analysis first."
            }), 404
        reasons_text = "\n\n".join(
            f"**{obj.objection}**\n{obj.ai_rationalisation}"
            for obj in records
        )
    else:
        records = (
            PlanningSupport.query
            .filter_by(case_reference=reference)
            .order_by(PlanningSupport.created_at.desc())
            .all()
        )
        if not records:
            return jsonify({
                "error": "No support reasons available for this case. "
                         "Run AI analysis first."
            }), 404
        reasons_text = "\n\n".join(
            f"**{s.support_reason}**\n{s.ai_rationalisation}"
            for s in records
        )

    # Build the prompt
    if letter_type == "objection":
        letter_instruction = (
            "Write a formal letter of objection to the planning authority regarding "
            "the planning application described below. The letter should clearly state "
            "the grounds for objection, referencing relevant planning policy where "
            "appropriate. The tone should be firm but polite and professional."
        )
    else:
        letter_instruction = (
            "Write a formal letter of support to the planning authority regarding "
            "the planning application described below. The letter should clearly state "
            "the reasons for support, referencing relevant planning policy where "
            "appropriate. The tone should be positive, constructive and professional."
        )

    prompt = ChatPromptTemplate.from_messages([
        ("system", (
            "You are an expert UK planning consultant who drafts formal letters to "
            "planning authorities on behalf of members of the public.\n\n"
            "Rules:\n"
            "- Write in formal letter format with appropriate structure\n"
            "- Format the entire letter in Markdown\n"
            "- Use **bold** for the recipient name and application reference\n"
            "- Use headings (##) for major sections where appropriate\n"
            "- Address it to: Planning Department, Plymouth City Council\n"
            "- Reference the planning application number clearly\n"
            "- Use the reasons provided as the basis for the letter\n"
            "- Cite specific planning policies where mentioned in the reasons\n"
            "- Keep the language accessible but professional\n"
            "- Sign off with the author's name\n"
            "- Do NOT invent additional reasons beyond those provided\n"
            "- Do NOT include the author's address (they will add it themselves)\n"
            "- Include today's date at the top of the letter\n"
        )),
        ("human", (
            "{instruction}\n\n"
            "## Application Details\n"
            "- Reference: {reference}\n"
            "- Address: {address}\n"
            "- Proposal: {proposal}\n\n"
            "## Reasons\n{reasons}\n\n"
            "## Author\n"
            "- Name: {first_name} {last_name}\n\n"
            "Generate the letter now."
        )),
    ])

    llm = ChatOpenAI(
        model=Config.LLM_MODEL,
        base_url=Config.NSCALE_BASE_URL,
        api_key=Config.NSCALE_TOKEN,
        temperature=0.3,
    )

    chain = prompt | llm

    try:
        response = chain.invoke({
            "instruction": letter_instruction,
            "reference": reference,
            "address": case.address,
            "proposal": case.proposal,
            "reasons": reasons_text,
            "first_name": first_name,
            "last_name": last_name,
        })

        letter_text = response.content if hasattr(response, 'content') else str(response)

        return jsonify({
            "reference": reference,
            "letter_type": letter_type,
            "author": f"{first_name} {last_name}",
            "letter": letter_text.strip(),
        })

    except Exception as e:
        return jsonify({
            "error": f"Failed to generate letter: {str(e)}"
        }), 502
