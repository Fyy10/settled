# Mini Task 6: CSRF, CORS, and Shared HTTP Security

## Goal

Establish the browser-security and JSON-protocol foundation that every later route composes explicitly.

## Prerequisites

Mini Tasks 2, 4, and 5.

## Implementation Scope

- Implement anonymous- and session-bound signed double-submit CSRF cookies and header tokens, including reuse, rotation, and expiration.
- Implement exact Origin canonicalization/allowlisting, credentialed CORS, preflight handling, and Origin checks for every unsafe method.
- Add panic recovery, request IDs/access logging, security headers, and explicit per-route middleware composition.
- Implement strict 1 MiB JSON decoding, unknown-field and multiple-value rejection, `204` handling, common JSON encoding, and the stable error model.
- Register `GET /api/auth/csrf`, JSON `404` for unknown paths, and JSON `405` for unsupported methods.

## Exclusions

- Do not implement registration, login, or business handlers.
- Do not use wildcard CORS or log cookies, Authorization, CSRF tokens, join codes, or request bodies.

## Validation

- Run `gofmt`, `go vet ./...`, `go test ./...`, and `go test -race ./...` from `server/`.
- Use `httptest` to cover anonymous/session CSRF, tampering, expiration, cross-session rejection, missing/disallowed Origin, preflight, headers, log redaction, panic recovery, strict JSON, and every common error mapping.

## Completion Criteria

- Tests make it impossible to register an unsafe route without its explicit Origin and CSRF middleware composition.
- `GET /api/auth/csrf` creates neither a session nor a user.
