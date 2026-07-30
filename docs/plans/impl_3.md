# Mini Task 3: PostgreSQL Schema, Store Foundation, and Integration Harness

## Goal

Create the complete first-release data structure and a repeatable PostgreSQL integration-test foundation.

## Prerequisites

Mini Task 2.

## Implementation Scope

- Add `pgx/v5/stdlib` and create one concrete `store.Store` backed by `database/sql`.
- Create `server/sql/schema.sql` with all six tables, constraints, indexes, and six views defined by `docs/02_entities.md`.
- Wrap the fresh-database schema in `BEGIN`/`COMMIT`; the application must not apply it automatically.
- Implement private transaction boilerplate, known PostgreSQL constraint-error translation, and basic connection lifecycle behavior.
- Establish the `integration` build-tag convention and add `scripts/test-integration.sh` using an ephemeral PostgreSQL container.
- Document how to apply the schema locally.

## Exclusions

- Do not add an ORM, migration framework, database UUID extension, persistent volume, or Docker Compose.
- Do not implement user or business Store methods.

## Validation

- Run `gofmt`, then `go vet ./...` and `go test ./...` from `server/`.
- Run `./scripts/test-integration.sh` to apply the schema to a fresh database and verify key constraints and views.
- If Docker or `psql` is unavailable, report the exact missing prerequisite instead of silently skipping the test.

## Completion Criteria

- The schema fully matches `02_entities.md`, and settlement data remains derived rather than persisted.
- The integration script removes only the exact container it created on success, failure, or interruption.
