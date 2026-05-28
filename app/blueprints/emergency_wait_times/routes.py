from datetime import datetime, timedelta

from flask import jsonify, request

from app.blueprints.emergency_wait_times import emergency_wait_times_bp
from app.blueprints.emergency_wait_times.models import Location, WaitTime

MAX_DATE_RANGE_DAYS = 31


@emergency_wait_times_bp.route("/locations", methods=["GET"])
def list_locations():
    locations = Location.query.all()
    return jsonify([loc.to_dict() for loc in locations])


@emergency_wait_times_bp.route("/locations/<uuid:location_id>", methods=["GET"])
def get_location(location_id):
    location = Location.query.get_or_404(location_id)
    return jsonify(location.to_dict())


@emergency_wait_times_bp.route("/locations/<uuid:location_id>/wait-times", methods=["GET"])
def list_wait_times(location_id):
    Location.query.get_or_404(location_id)

    start = request.args.get("start")
    end = request.args.get("end")

    query = WaitTime.query.filter_by(location_id=location_id)

    if start:
        try:
            start_dt = datetime.fromisoformat(start)
        except ValueError:
            return jsonify({"error": "Invalid start date format. Use ISO 8601."}), 400
        query = query.filter(WaitTime.timestamp >= start_dt)

    if end:
        try:
            end_dt = datetime.fromisoformat(end)
        except ValueError:
            return jsonify({"error": "Invalid end date format. Use ISO 8601."}), 400
        query = query.filter(WaitTime.timestamp <= end_dt)

    # Validate date range does not exceed 31 days
    if start and end:
        if (end_dt - start_dt) > timedelta(days=MAX_DATE_RANGE_DAYS):
            return jsonify({
                "error": f"Date range must not exceed {MAX_DATE_RANGE_DAYS} days."
            }), 400

    wait_times = (
        query.order_by(WaitTime.timestamp.desc())
        .all()
    )
    return jsonify([wt.to_dict() for wt in wait_times])
