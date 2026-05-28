from app.celery_app import celery


@celery.task
def example_task():
    return {"status": "ok", "message": "Example task completed"}
