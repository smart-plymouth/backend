from flask import Blueprint

bird_monitoring_bp = Blueprint("bird_monitoring", __name__)

from app.blueprints.bird_monitoring import routes  # noqa: E402, F401
