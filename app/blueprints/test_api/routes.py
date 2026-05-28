from flask import jsonify

from app.blueprints.test_api import test_api_bp


@test_api_bp.route("/", methods=["GET"])
def hello():
    return jsonify({"message": "Hello, World!", "service": "test-api", "version": "1.0"})
