from celery import Celery
from celery.schedules import crontab
from app.config import Config

celery = Celery(
    "smartplymouth",
    broker=Config.CELERY_BROKER_URL,
    backend=Config.CELERY_RESULT_BACKEND,
)

celery.conf.update(
    task_serializer="json",
    accept_content=["json"],
    result_serializer="json",
    timezone="Europe/London",
    enable_utc=True,
    # Route AI analysis tasks to a dedicated queue with concurrency=1
    task_routes={
        "app.blueprints.planning.tasks.analyse_planning_application": {
            "queue": "planning_analysis",
        },
    },
    beat_schedule={
        "fetch-wait-times-every-5-minutes": {
            "task": "app.blueprints.emergency_wait_times.tasks.fetch_wait_times",
            "schedule": 300.0,  # every 5 minutes
        },
        "refresh-planning-applications-daily": {
            "task": "app.blueprints.planning.tasks.refresh_planning_applications",
            # "schedule": crontab(hour=7, minute=0),  # Every day at 07:00
        },
    },
)

# Auto-discover tasks in blueprint packages
celery.autodiscover_tasks([
    "app.blueprints.test_api",
    "app.blueprints.emergency_wait_times",
    "app.blueprints.planning",
])
