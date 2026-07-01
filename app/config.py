import os


class Config:
    SECRET_KEY = os.environ.get("SECRET_KEY", "change-me-in-production")
    SQLALCHEMY_DATABASE_URI = os.environ.get(
        "DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/smartplymouth"
    )
    SQLALCHEMY_TRACK_MODIFICATIONS = False
    CELERY_BROKER_URL = os.environ.get("CELERY_BROKER_URL", "redis://localhost:6379/0")
    CELERY_RESULT_BACKEND = os.environ.get(
        "CELERY_RESULT_BACKEND", "redis://localhost:6379/0"
    )
    NSCALE_BASE_URL = os.environ.get(
        "NSCALE_BASE_URL", "https://inference.api.nscale.com/v1"
    )
    NSCALE_TOKEN = os.environ.get("NSCALE_TOKEN", "")
    LLM_MODEL = os.environ.get("LLM_MODEL", "Qwen/Qwen3-32B")
    EMBEDDING_MODEL = os.environ.get("EMBEDDING_MODEL", "Qwen/Qwen3-Embedding-8B")
    POLICY_VECTORSTORE_DIR = os.environ.get(
        "POLICY_VECTORSTORE_DIR",
        os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "data", "policy_vectorstore"),
    )
