# Mini Task 14: Backend Hardening and Release Quality Gate

## Goal

Validate failure paths, concurrency consistency, and production configuration across the complete API before frontend integration.

## Prerequisites

Mini Tasks 2 through 13.

## Implementation Scope

- Complete handler coverage for success DTOs, strict decoding, combined UUID/path checks, authentication, Origin, CSRF, and error mapping.
- Complete integration coverage for fresh-schema application, Store ordering, rollback, soft-state hiding, and lock order.
- Verify logs exclude database URLs, cookies, Authorization, CSRF tokens, passwords, join codes, and password hashes.
- Verify readiness, request errors, and lifecycle behavior while PostgreSQL is temporarily unavailable.
- Update backend local-run, integration-test, and environment-variable documentation.

## Exclusions

- Do not add rate limiting, caching, queues, automatic migrations, monitoring SaaS, or new architecture layers.
- Do not implement frontend or container images.

## Validation

- Run `gofmt`, `go vet ./...`, `go test ./...`, `go test -race ./...`, and `./scripts/test-integration.sh` from `server/`.
- Manually complete a two-user auth → group → expense → repayment → settlement → delete/dissolve API workflow.
- Check coverage against the authorization matrix and hidden-state rules in `03_API.md`.

## Completion Criteria

- Every command passes; if an external prerequisite is missing, record the command, missing prerequisite, and unverified risk.
- There are no known endpoint, method, status-code, or security deviations from `03-04`.
