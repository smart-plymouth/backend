from datetime import date, datetime, timedelta

from flask import jsonify, request

from app import db
from app.blueprints.bird_monitoring import bird_monitoring_bp
from app.blueprints.bird_monitoring.models import (
    MonitoringSite,
    Species,
    SpeciesSighting,
)


def _parse_date(value):
    """Parse an ISO 8601 date string (YYYY-MM-DD). Returns None on failure."""
    try:
        return date.fromisoformat(value)
    except (ValueError, TypeError):
        return None


@bird_monitoring_bp.route("/webhook/<site_id>", methods=["POST"])
def receive_webhook(site_id):
    """Receive a bird detection webhook from an edge device.

    The site_id in the URL acts as authentication — if the site doesn't
    exist, the request is rejected with 401.

    POST body (JSON):
        common_name  - species common name (string, required)
        confidence   - detection confidence (float, required)
    """
    site = MonitoringSite.query.filter_by(site_id=site_id).first()
    if site is None:
        return jsonify({"error": "Unauthorized"}), 401

    data = request.get_json(silent=True)
    if not data:
        return jsonify({"error": "Request body must be JSON"}), 400

    common_name = data.get("common_name")
    confidence = data.get("confidence")

    if not common_name or confidence is None:
        return jsonify({"error": "common_name and confidence are required"}), 400

    try:
        confidence = float(confidence)
    except (ValueError, TypeError):
        return jsonify({"error": "confidence must be a number"}), 400

    # Find or create species
    species = Species.query.filter_by(common_name=common_name).first()
    if species is None:
        species = Species(common_name=common_name)
        db.session.add(species)
        db.session.flush()

    # Create sighting
    sighting = SpeciesSighting(
        site_id=site.site_id,
        species_id=species.species_id,
        confidence=confidence,
    )
    db.session.add(sighting)
    db.session.commit()

    return jsonify({
        "status": "recorded",
        "sighting_id": sighting.sighting_id,
        "species": species.to_dict(),
    }), 201


@bird_monitoring_bp.route("/sites", methods=["GET"])
def list_sites():
    """List all monitoring sites."""
    sites = MonitoringSite.query.order_by(MonitoringSite.name).all()
    return jsonify({
        "sites": [site.to_dict() for site in sites],
    })


@bird_monitoring_bp.route("/sightings", methods=["GET"])
def list_sightings():
    """List bird sightings with filtering and pagination.

    Query params:
        site_id   - filter by monitoring site UUID
        from_date - start of date range inclusive (YYYY-MM-DD)
        to_date   - end of date range inclusive (YYYY-MM-DD)
        page      - page number (default 1)
        per_page  - results per page (default 25, max 100)

    Date range must not exceed 31 days.
    """
    site_id = request.args.get("site_id")
    from_date_str = request.args.get("from_date")
    to_date_str = request.args.get("to_date")
    page = request.args.get("page", 1, type=int)
    per_page = request.args.get("per_page", 25, type=int)
    per_page = min(per_page, 100)

    query = SpeciesSighting.query

    # Site filter
    if site_id:
        query = query.filter(SpeciesSighting.site_id == site_id)

    # Date filters
    from_date = None
    to_date = None

    if from_date_str:
        from_date = _parse_date(from_date_str)
        if from_date is None:
            return jsonify({"error": "Invalid from_date format. Use YYYY-MM-DD."}), 400
        query = query.filter(
            SpeciesSighting.datetime >= datetime(from_date.year, from_date.month, from_date.day)
        )

    if to_date_str:
        to_date = _parse_date(to_date_str)
        if to_date is None:
            return jsonify({"error": "Invalid to_date format. Use YYYY-MM-DD."}), 400
        # Include the entire to_date day
        to_datetime = datetime(to_date.year, to_date.month, to_date.day) + timedelta(days=1)
        query = query.filter(SpeciesSighting.datetime < to_datetime)

    # Validate date range doesn't exceed 31 days
    if from_date and to_date:
        if to_date < from_date:
            return jsonify({"error": "to_date must be on or after from_date."}), 400
        if (to_date - from_date).days > 31:
            return jsonify({"error": "Date range must not exceed 31 days."}), 400

    pagination = query.order_by(SpeciesSighting.datetime.desc()).paginate(
        page=page, per_page=per_page, error_out=False
    )

    return jsonify({
        "sightings": [s.to_dict() for s in pagination.items],
        "total": pagination.total,
        "page": pagination.page,
        "pages": pagination.pages,
        "per_page": pagination.per_page,
    })
