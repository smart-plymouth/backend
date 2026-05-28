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
)

# Auto-discover tasks in blueprint packages
celery.autodiscover_tasks(["app.blueprints.test_api"])
