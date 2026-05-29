from datetime import date, timedelta

from flask import jsonify, request

from app.blueprints.planning import planning_bp
from app.blueprints.planning.models import PlanningCase


def _parse_date(value):
    """Parse an ISO 8601 date string (YYYY-MM-DD). Returns None on failure."""
    try:
        return date.fromisoformat(value)
    except (ValueError, TypeError):
        return None


@planning_bp.route("/cases", methods=["GET"])
def list_cases():
    """List planning cases with optional filtering.

    Query params:
        status         - filter by status (partial match)
        validated_date - exact validated date (YYYY-MM-DD)
        validated_from - start of validated date range (inclusive)
        validated_to   - end of validated date range (inclusive)
        page           - page number (default 1)
        per_page       - results per page (default 25, max 100)
    """
    status = request.args.get("status")
    validated_date = request.args.get("validated_date")
    validated_from = request.args.get("validated_from")
    validated_to = request.args.get("validated_to")
    page = request.args.get("page", 1, type=int)
    per_page = request.args.get("per_page", 25, type=int)
    per_page = min(per_page, 100)  # cap at 100

    query = PlanningCase.query

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


@planning_bp.route("/fetch", methods=["POST"])
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
    }), 202


@planning_bp.route("/cases/<path:reference>", methods=["GET"])
def get_case(reference):
    """Get a single planning case by reference number.

    The reference contains forward slashes (e.g. 26/00747/CDM) so we use
    a path converter to capture the full value.
    """
    case = PlanningCase.query.get_or_404(reference)
    return jsonify(case.to_dict())
