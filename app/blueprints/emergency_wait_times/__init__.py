from flask import Blueprint

emergency_wait_times_bp = Blueprint("emergency_wait_times", __name__)

from app.blueprints.emergency_wait_times import routes  # noqa: E402, F401
