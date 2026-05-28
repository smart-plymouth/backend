from flask import Flask
from flask_sqlalchemy import SQLAlchemy

db = SQLAlchemy()


def create_app():
    app = Flask(__name__)
    app.config.from_object("app.config.Config")

    db.init_app(app)

    # Register blueprints
    from app.blueprints.test_api import test_api_bp

    app.register_blueprint(test_api_bp, url_prefix="/api/test-api/v1.0")

    return app
