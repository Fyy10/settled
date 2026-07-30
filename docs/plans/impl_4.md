# Mini Task 4: Backend Identifier, Time, Input, and Money Rules

## Goal

Implement cross-cutting boundary rules as pure, table-tested Go logic before business workflows depend on them.

## Prerequisites

Mini Task 2.

## Implementation Scope

- Implement RFC 4122 version 4 UUIDs, random IDs, and secure join codes using `crypto/rand`.
- Implement UUID parsing, an injectable UTC clock, strict dates, and RFC 3339 output rules.
- Implement normalization and limits for email, display name, password, group name, expense description, and repayment note.
- Represent money as positive `int64` cents with overflow-safe helpers; currency is always `USD`.
- Return stable field keys that later handlers can map to API validation errors.

## Exclusions

- Do not implement HTTP, password hashing, JWTs, CSRF, database writes, or business workflows.
- Do not accept floating-point money or editable currency.

## Validation

- Run `gofmt`, `go vet ./...`, and `go test ./...` from `server/`.
- Cover Unicode and control characters, UTF-8 byte boundaries, invalid dates, UUID variants, random-source failures, money overflow, and the join-code alphabet with table-driven tests.

## Completion Criteria

- Every relevant valid and invalid boundary in the “Identifiers, Time, Dates, And Money” and “User Input Rules” sections of `04_backend.md` has automated coverage.
