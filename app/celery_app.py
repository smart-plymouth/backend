from celery import Celery
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
    beat_schedule={
        "fetch-wait-times-every-5-minutes": {
            "task": "app.blueprints.emergency_wait_times.tasks.fetch_wait_times",
            "schedule": 300.0,  # every 5 minutes
        },
    },
)

# Auto-discover tasks in blueprint packages
celery.autodiscover_tasks([
    "app.blueprints.test_api",
    "app.blueprints.emergency_wait_times",
])
