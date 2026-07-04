# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go mod files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build all binaries
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/scheduler ./cmd/scheduler
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/migrate ./cmd/migrate

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata poppler-utils

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /bin/api /bin/api
COPY --from=builder /bin/worker /bin/worker
COPY --from=builder /bin/scheduler /bin/scheduler
COPY --from=builder /bin/migrate /bin/migrate

# Copy vectorstore data
COPY data/policy_vectorstore/ ./data/policy_vectorstore/

ENV TZ=Europe/London

EXPOSE 5000

# Default entrypoint runs the API
CMD ["/bin/api"]
