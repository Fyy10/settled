# Mini Task 2: Backend Process, Configuration, and Health Checks

## Goal

Establish a runnable, safely stoppable, and independently testable Go API process.

## Prerequisites

Mini Task 1.

## Implementation Scope

- Create `cmd/settled`, `internal/config`, and the minimum `internal/httpapi` structure defined by `docs/04_backend.md`.
- Implement complete configuration parsing and production rejection rules, JSON `slog`, database pool settings, and graceful shutdown.
- Use `net/http` with the documented timeouts and request/header limits.
- Implement `GET /api/health/live` and database-backed `GET /api/health/ready` using `PingContext`.
- Preserve injectable time and randomness boundaries without introducing a general dependency-injection framework.

## Exclusions

- Do not implement `/healthz`, business routes, automatic schema migration, or frontend serving.
- Do not add a Go web framework, router, or logging framework.

## Validation

- Run `gofmt` on every added Go file.
- Run `go vet ./...` and `go test ./...` from `server/`.
- Use `httptest` to cover configuration defaults and rejection rules, live `200`, and ready `200`/`503`.
- Start the server locally and confirm both health endpoints return the JSON defined by `03_API.md`.

## Completion Criteria

- Missing or unsafe production configuration fails before the process starts listening.
- The process shuts down gracefully; liveness does not query PostgreSQL, while readiness uses a two-second database timeout.
