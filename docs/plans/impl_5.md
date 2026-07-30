# Mini Task 5: Argon2id Passwords and Stateless Session JWTs

## Goal

Implement the authentication cryptography core independently of HTTP and PostgreSQL.

## Prerequisites

Mini Task 4.

## Implementation Scope

- Add `golang.org/x/crypto/argon2` and generate PHC-formatted Argon2id hashes using the fixed parameters in `04_backend.md`.
- Strictly parse and bound stored PHC parameters, use constant-time comparison, and create a process-level dummy hash for unknown-email timing protection.
- Add `github.com/golang-jwt/jwt/v5` and issue/validate session JWTs using HS256 only.
- Include only the required registered claims, `sub`, and random `jti`; use a fixed 168-hour lifetime and 30-second clock skew.
- Implement setting and clearing the `settled_session` cookie with the documented attributes and no sliding refresh.

## Exclusions

- Do not add refresh tokens, a server-side session table, revocation list, key rotation, or authentication endpoints.
- Do not include profile data, roles, memberships, or a CSRF nonce in the JWT.

## Validation

- Run `gofmt`, `go vet ./...`, `go test ./...`, and `go test -race ./...` from `server/`.
- Test correct and incorrect passwords, hostile PHC parameters, wrong algorithms/signatures/issuer/audience, future times, excessive lifetimes, invalid subjects/JWT IDs, and cookie-clearing attributes.

## Completion Criteria

- Validation failures expose only an unauthenticated result, never password or token details.
- Any API replica with the same configuration can validate a cookie issued by another replica.
