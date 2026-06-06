from flask import Flask
from flask_cors import CORS
from flask_sqlalchemy import SQLAlchemy

db = SQLAlchemy()


def create_app():
    app = Flask(__name__)
    app.config.from_object("app.config.Config")

    db.init_app(app)
    CORS(app)

    # Register blueprints
    from app.blueprints.test_api import test_api_bp
    from app.blueprints.emergency_wait_times import emergency_wait_times_bp
    from app.blueprints.planning import planning_bp
    from app.blueprints.bird_monitoring import bird_monitoring_bp

    app.register_blueprint(test_api_bp, url_prefix="/api/test-api/v1.0")
    app.register_blueprint(
        emergency_wait_times_bp, url_prefix="/api/emergency-wait-times/v1.0"
    )
    app.register_blueprint(planning_bp, url_prefix="/api/planning/v1.0")
    app.register_blueprint(bird_monitoring_bp, url_prefix="/api/bird-monitoring/v1.0")

    return app
