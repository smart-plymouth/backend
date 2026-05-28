from flask import Blueprint

test_api_bp = Blueprint("test_api", __name__)

from app.blueprints.test_api import routes  # noqa: E402, F401
