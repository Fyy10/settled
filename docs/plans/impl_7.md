# Mini Task 7: User Store and Complete Authentication API

## Goal

Deliver independently verifiable registration, login, logout, and current-user flows using cookie sessions.

## Prerequisites

Mini Tasks 3, 4, 5, and 6.

## Implementation Scope

- Implement a narrow `UserStore` for user creation, credential lookup by email, and public-user lookup by ID.
- Implement `POST /api/auth/register`, `POST /api/auth/login`, `POST /api/auth/logout`, and `GET /api/me`.
- Registration uses strict JSON; login uses UTF-8 Basic Auth and has no JSON body.
- Register/login validate allowed Origin and anonymous CSRF before expensive or identifying work, then issue a session, rotate CSRF, and return `{csrfToken,user}`.
- Authentication middleware validates the JWT and reloads the user from PostgreSQL; logout requires session-bound CSRF, clears the session, and rotates to an anonymous binding.
- Follow the status codes and response DTOs in `03_API.md` exactly, including indistinguishable invalid-credential messages.

## Exclusions

- Do not return bearer tokens or add OAuth, email verification, password reset, 2FA, or rate limiting.

## Validation

- Run `gofmt`, `go vet ./...`, `go test ./...`, `go test -race ./...`, and `./scripts/test-integration.sh`.
- Cover every success response, cookie/CSRF rotation, duplicate email, malformed Basic Auth, unknown user, wrong password, expired session, and `/api/me` database reload.
- Manually complete csrf → register/login → me → logout using an HTTP cookie jar.

## Completion Criteria

- The browser neither needs nor receives a session token for storage.
- Neither PostgreSQL-derived API responses nor client responses expose `password_hash`.
