from flask import jsonify

from app.blueprints.emergency_wait_times import emergency_wait_times_bp
from app.blueprints.emergency_wait_times.models import Location, WaitTime


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
    wait_times = (
        WaitTime.query.filter_by(location_id=location_id)
        .order_by(WaitTime.timestamp.desc())
        .limit(100)
        .all()
    )
    return jsonify([wt.to_dict() for wt in wait_times])
