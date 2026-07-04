# SmartPlymouth Backend

Go backend for the SmartPlymouth platform, providing APIs for:

- **Emergency Wait Times** — live ED/UTC/MIU wait times from Plymouth hospitals
- **Planning Applications** — weekly list scraping, AI analysis, objection/support generation, letter writing
- **Bird Monitoring** — BirdNET-Pi webhook ingestion and sighting queries

## Architecture

| Component | Purpose |
|-----------|---------|
| `cmd/api` | HTTP API server (Chi router) |
| `cmd/worker` | Distributed task worker (Asynq/Redis) |
| `cmd/scheduler` | Periodic task scheduler (replaces Celery Beat) |
| `cmd/migrate` | Database migration runner |

## Prerequisites

- Go 1.23+
- PostgreSQL 16+
- Redis 7+

## Development

```bash
# Run migrations
go run ./cmd/migrate up

# Start the API server
go run ./cmd/api

# Start a worker (processes async tasks)
go run ./cmd/worker

# Start the scheduler (periodic tasks)
go run ./cmd/scheduler
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `postgresql://postgres:postgres@localhost:5432/smartplymouth` | PostgreSQL connection string |
| `REDIS_URL` | `redis://localhost:6379/0` | Redis connection string |
| `SECRET_KEY` | `change-me-in-production` | Application secret |
| `NSCALE_BASE_URL` | `https://inference.api.nscale.com/v1` | LLM API base URL |
| `NSCALE_TOKEN` | | LLM API token |
| `LLM_MODEL` | `Qwen/Qwen3-32B` | LLM model name |
| `EMBEDDING_MODEL` | `Qwen/Qwen3-Embedding-8B` | Embedding model name |
| `POLICY_VECTORSTORE_DIR` | `data/policy_vectorstore` | Path to ChromaDB vectorstore |
| `WORKER_CONCURRENCY` | `4` | Number of concurrent task workers |
| `PORT` | `5000` | API server port |

## Building

```bash
# Build all binaries
go build -o bin/api ./cmd/api
go build -o bin/worker ./cmd/worker
go build -o bin/scheduler ./cmd/scheduler
go build -o bin/migrate ./cmd/migrate

# Or use Docker
docker build -t smartplymouth/backend .
```

## Deployment

Kubernetes manifests are in `deploy.yml`. The application runs as:

- 1x API pod
- 1x Worker pod (concurrency=4, handles all tasks including AI analysis)
- 1x Scheduler pod (enqueues periodic tasks)
- 1x PostgreSQL pod
- 1x Redis pod

## Migrations

Migrations are versioned Go files in `internal/migrations/`. Each migration registers itself via `init()` and provides `Up`/`Down` functions. The `schema_migrations` table tracks applied versions.

```bash
# Apply all pending migrations
go run ./cmd/migrate up

# Revert the last migration
go run ./cmd/migrate down
```
