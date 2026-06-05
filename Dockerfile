FROM python:3.12-slim

WORKDIR /app

# Install system dependencies for psycopg2 and native builds
RUN apt-get update && apt-get install -y --no-install-recommends \
    libpq-dev gcc libffi-dev \
    && rm -rf /var/lib/apt/lists/*

# Install Python dependencies in smaller layers to stay under 100MB per layer

# Core web framework and database
RUN pip install --no-cache-dir \
    Flask==3.1.0 \
    Flask-Cors==5.0.1 \
    Flask-SQLAlchemy==3.1.1 \
    SQLAlchemy==2.0.36 \
    alembic==1.14.1 \
    psycopg2-binary==2.9.10 \
    gunicorn==23.0.0

# Task queue and utilities
RUN pip install --no-cache-dir \
    celery[redis]==5.4.0 \
    redis==5.2.1 \
    flower==2.0.1 \
    requests==2.32.5 \
    beautifulsoup4==4.12.3 \
    pypdf==5.4.0 \
    cryptography==44.0.3

# LangChain core and Ollama
RUN pip install --no-cache-dir \
    langchain-core==1.4.1 \
    langchain-ollama==1.1.0

# ChromaDB heavy dependencies split across layers to stay under 100MB each
RUN pip install --no-cache-dir \
    numpy==2.2.6

RUN pip install --no-cache-dir \
    onnxruntime==1.22.0

RUN pip install --no-cache-dir \
    chromadb==1.5.9

# LangChain integrations
RUN pip install --no-cache-dir \
    langchain-chroma==1.1.0 \
    langchain-community==0.4.2 \
    langchain-text-splitters==1.1.2

# Copy application code
COPY app/ ./app/
COPY migrations/ ./migrations/
COPY scripts/ ./scripts/
COPY wsgi.py alembic.ini ./

# Copy vectorstore as a separate layer
COPY data/policy_vectorstore/ ./data/policy_vectorstore/

ENV FLASK_APP=wsgi.py
ENV PYTHONUNBUFFERED=1

EXPOSE 5000

# Default entrypoint runs the API via gunicorn
CMD ["gunicorn", "--bind", "0.0.0.0:5000", "--workers", "4", "wsgi:app"]
