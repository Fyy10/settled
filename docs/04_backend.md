# Settled Go Backend Implementation Design

This document defines the implementation-level design for the first-release Go
backend. It translates the product, architecture, entity, and API decisions in
`00_project.md` through `03_API.md` into concrete Go package boundaries,
security mechanisms, database workflows, transaction rules, and test strategy.

This document is the implementation authority for the backend. `03_API.md`
remains the authority for the public HTTP contract. If an older file under
`docs/plans/` conflicts with this document or `03_API.md`, the older plan is
obsolete for that point. In particular:

- Authentication is a JWT stored in an HttpOnly cookie, not a token returned for
  frontend storage.
- Expense and repayment replacement endpoints use `PUT`, not `PATCH`.
- Health endpoints are `/api/health/live` and `/api/health/ready`.
- Unsafe requests require both an allowed `Origin` and a valid CSRF token.

## Design Goals

- Keep the server direct, standard-library first, and easy to scan.
- Make authorization and accounting rules explicit and testable.
- Keep API instances stateless and interchangeable.
- Protect multi-row invariants in PostgreSQL transactions.
- Keep HTTP, business rules, and SQL access separate without introducing generic
  repositories or dependency-injection machinery.
- Make security-sensitive behavior deterministic and configurable.
- Leave no implementation choices unresolved for the first release.

## Technology And Dependencies

The backend uses Go 1.25 and the module already declared in `server/go.mod`.

The only required non-standard-library modules are:

| Module | Purpose |
| --- | --- |
| `github.com/jackc/pgx/v5/stdlib` | PostgreSQL driver through `database/sql` |
| `golang.org/x/crypto/argon2` | Argon2id password hashing |
| `github.com/golang-jwt/jwt/v5` | JWT creation and strict validation |

The implementation must pin the versions selected by `go mod tidy`. It must not
add a web framework, router, ORM, migration framework, UUID library,
validation framework, logging framework, or test-container library.

Use the standard library for:

- `net/http` routing and serving.
- `log/slog` structured logs.
- `database/sql` connection pooling and transactions.
- `crypto/rand` for UUIDs, salts, JWT IDs, CSRF nonces, and join codes.
- `crypto/hmac` and `crypto/sha256` for CSRF signatures.
- `encoding/json` for API payloads.
- `os/signal` and `context` for graceful shutdown.

## Project Layout

The backend should use this controlled layout:

```text
server/
  cmd/
    settled/
      main.go
  internal/
    auth/
      password.go
      session.go
      csrf.go
      service.go
    config/
      config.go
    expenses/
      service.go
      splits.go
      types.go
    groups/
      service.go
      types.go
    httpapi/
      api.go
      auth_handlers.go
      group_handlers.go
      expense_handlers.go
      repayment_handlers.go
      settlement_handlers.go
      health_handlers.go
      middleware.go
      json.go
      errors.go
      dto.go
    repayments/
      service.go
      types.go
    settlements/
      calculator.go
      types.go
    store/
      store.go
      users.go
      groups.go
      expenses.go
      repayments.go
      settlements.go
      errors.go
  scripts/
    test-integration.sh
  sql/
    schema.sql
```

Tests should live beside the package they test. PostgreSQL integration tests use
files ending in `_integration_test.go` and the `integration` build tag.

### Package Responsibilities

`cmd/settled`

- Load and validate configuration.
- Construct the logger, database, stores, services, session and CSRF utilities,
  and HTTP API.
- Start the HTTP server and coordinate graceful shutdown.
- Contain no business rules or SQL.

`internal/config`

- Read environment variables exactly once at startup.
- Apply development-safe defaults.
- Reject missing, malformed, duplicated, or unsafe production configuration.
- Return a typed immutable configuration value.

`internal/httpapi`

- Register routes on `http.ServeMux`.
- Decode path, header, query, and JSON inputs.
- Apply authentication, origin, CSRF, and authorization-aware middleware.
- Convert API DTOs to domain inputs and domain outputs to API DTOs.
- Map typed domain/store errors to the response contract in `03_API.md`.
- Never contain SQL, password hashing, split calculation, or settlement logic.

`internal/auth`

- Normalize and validate registration/login inputs.
- Hash and verify passwords.
- Issue and validate session JWTs.
- Create, reuse, rotate, and validate CSRF bindings.
- Coordinate registration and login through a narrow user store interface.

`internal/groups`, `internal/expenses`, and `internal/repayments`

- Define workflow inputs, outputs, validation, and narrow store interfaces.
- Call explicit atomic store methods for database workflows.
- Contain no HTTP response decisions.

`internal/settlements`

- Define normalized debt entries and public transfer results.
- Implement pairwise aggregation and netting as deterministic pure Go logic.
- Know nothing about HTTP or PostgreSQL.

`internal/store`

- Implement concrete PostgreSQL queries and transaction workflows.
- Translate database errors to a small stable set of store errors.
- Protect cross-row invariants under the transaction and locking rules below.
- Avoid a generic CRUD repository, query builder, or domain-agnostic transaction
  abstraction.

## Process Wiring And Lifecycle

`main.go` constructs dependencies in this order:

1. Load `config.Config`.
2. Create a JSON `slog.Logger` writing to standard output.
3. Open PostgreSQL through `sql.Open("pgx", cfg.DatabaseURL)`.
4. Apply connection-pool settings and verify the database with a five-second
   `PingContext`.
5. Construct `store.Store`.
6. Construct the password hasher, session manager, CSRF manager, domain services,
   settlement calculator, and HTTP API.
7. Create `http.Server` and start `ListenAndServe` in a goroutine.
8. Wait for `SIGINT`, `SIGTERM`, or an unexpected server error.
9. Call `Server.Shutdown` with a ten-second deadline.
10. Close the database and exit non-zero for startup or unexpected serve errors.

`http.ErrServerClosed` during an intentional shutdown is not an error.

Recommended server defaults:

```text
ReadHeaderTimeout: 5 seconds
ReadTimeout:       15 seconds
WriteTimeout:      30 seconds
IdleTimeout:       60 seconds
MaxHeaderBytes:    1 MiB
Shutdown timeout:  10 seconds
```

The 30-second write timeout leaves room for an Argon2id login/register request
without allowing indefinitely slow responses.

## Configuration

### Environment Variables

| Variable | Required | Default | Rules |
| --- | --- | --- | --- |
| `APP_ENV` | no | `development` | One of `development`, `test`, `production` |
| `HTTP_ADDR` | no | `:8080` | Non-blank listen address |
| `DATABASE_URL` | yes | none | PostgreSQL URL; never log the full value |
| `ALLOWED_ORIGINS` | production | local frontend origins in development | Comma-separated exact origins |
| `JWT_SECRET_BASE64` | yes | none | Base64-decoded value must be at least 32 bytes |
| `CSRF_SECRET_BASE64` | yes | none | Base64-decoded value must be at least 32 bytes and differ from JWT secret |
| `COOKIE_SECURE` | no | `false` outside production, `true` in production | Production must be `true` |
| `COOKIE_SAME_SITE` | no | `lax` outside production, `none` in production | `none` requires secure cookies |
| `COOKIE_DOMAIN` | no | unset | Normally unset so cookies are host-only |
| `DB_MAX_OPEN_CONNS` | no | `10` | Positive integer |
| `DB_MAX_IDLE_CONNS` | no | `10` | Between zero and max open |
| `DB_CONN_MAX_LIFETIME` | no | `30m` | Positive duration |
| `DB_CONN_MAX_IDLE_TIME` | no | `5m` | Positive duration |
| `LOG_LEVEL` | no | `info` | `debug`, `info`, `warn`, or `error` |

Local frontend defaults may include `http://localhost:5173` and
`http://127.0.0.1:5173`. They remain separate exact origins; an origin is never
matched by suffix, wildcard, regular expression, or substring.

`DATABASE_URL`, JWT secret, and CSRF secret remain required in development. Tests
that construct individual utilities may bypass the environment loader by
building a typed config directly.

### Production Validation

Production startup fails before listening when:

- `DATABASE_URL` or either secret is absent.
- Either decoded secret is shorter than 32 bytes.
- JWT and CSRF secrets contain identical bytes.
- No allowed frontend origin is configured.
- An allowed origin is not an absolute `https` origin without path, query,
  fragment, or user information.
- `COOKIE_SECURE` is false.
- `COOKIE_SAME_SITE` is not `none`.
- A connection-pool value is invalid.

Development accepts explicit `http` origins and insecure cookies but must not
silently weaken production values.

## Identifiers, Time, Dates, And Money

### UUIDs

Generate RFC 4122 version 4 UUID strings with 16 random bytes from `crypto/rand`,
setting the version and variant bits before canonical lowercase formatting.
Failure to obtain cryptographic randomness fails the workflow; there is no weak
random fallback.

UUID parsing at the HTTP boundary must verify:

- Canonical `8-4-4-4-12` hyphen placement.
- Exactly 32 hexadecimal digits.
- A valid RFC 4122 variant.

The API may accept upper- or lowercase hexadecimal input and normalize it to
lowercase.

### Clock

Use an injectable clock in services and security utilities:

```go
type Clock interface {
    Now() time.Time
}
```

Production uses `time.Now().UTC()`. Tests use a fixed clock. Store writes receive
the workflow time from the service so related `created_at`, `updated_at`,
`removed_at`, `deleted_at`, and `dissolved_at` values are consistent.

### Dates

Parse API dates strictly with `time.Parse("2006-01-02", value)`. Reject
timestamps, partial dates, and impossible calendar dates. Persist PostgreSQL
`date` values and serialize them as `YYYY-MM-DD`.

### Timestamps

Convert timestamps to UTC before returning them. Encode them using RFC 3339 with
the precision returned by PostgreSQL. Tests compare instants rather than assuming
a fixed fractional-second width.

### Money

Use `int64` cents throughout the domain, store, and API. Reject values less than
or equal to zero. Split calculations must check addition and multiplication for
`int64` overflow before performing them. Currency is a constant `USD` and is not
accepted as an editable request field.

## User Input Rules

### Email

Registration and login normalize email identically:

1. Remove leading and trailing Unicode whitespace.
2. Convert the full value to lowercase.
3. Require 3 through 254 UTF-8 bytes.
4. Require exactly one `@`.
5. Require non-empty local and domain parts.
6. Require a dot in neither edge of the domain, no whitespace or control
   characters, and no consecutive dots in the domain.

This is deliberately a login identifier check rather than complete RFC mailbox
validation. The normalized value is stored and returned by the API.

### Display Name

Trim leading and trailing Unicode whitespace, require a non-empty result, reject
control characters, and limit the stored value to 120 Unicode code points. Use
the same normalized value for validation and persistence.

### Password

- Measure the raw UTF-8 byte sequence without trimming or normalization.
- Require 8 through 128 bytes inclusive.
- Reject invalid UTF-8 and NUL bytes.
- Do not require uppercase, lowercase, digit, symbol, or periodic rotation rules.
- Never log, format into an error, or retain a second string copy longer than
  required by the request.

### Group, Expense, And Repayment Text

- Group names and expense descriptions are trimmed before validation and
  persistence.
- Group names are 1–160 Unicode code points.
- Expense descriptions are 1–240 Unicode code points.
- Repayment notes are optional. A missing, null, empty, or whitespace-only note
  persists as SQL `NULL`; a non-empty normalized note is at most 240 Unicode code
  points.
- Reject control characters other than ordinary spaces.

## Password Hashing

Use Argon2id with these initial parameters:

```text
version:     19
memory:      65536 KiB
iterations:  3
parallelism: 4
salt length: 16 bytes
key length:  32 bytes
```

Persist a PHC-style value:

```text
$argon2id$v=19$m=65536,t=3,p=4$<base64-salt>$<base64-hash>
```

Use unpadded standard Base64 for salt and derived key fields. Verification must:

1. Parse a fixed six-field PHC structure.
2. Require algorithm `argon2id` and supported version 19.
3. Bound memory, iterations, parallelism, salt, and key length before allocating
   or hashing, preventing hostile stored values from exhausting the process.
4. Recalculate using the stored parameters.
5. Compare with `subtle.ConstantTimeCompare`.

Malformed stored hashes return an internal authentication error, not an invalid
credentials distinction to the client. Login returns the same `401 unauthorized`
response for an unknown email and an incorrect password. For an unknown email,
perform one Argon2id calculation against a process-start dummy hash so timing
does not trivially reveal account existence.

Password parameters are constants for the first release. The encoded parameter
fields permit later rehash-on-login without a schema change, but automatic
rehashing is not implemented now.

## Stateless Session JWT

### Claims And Signing

Use `github.com/golang-jwt/jwt/v5` with HS256 only. Define private claims by
embedding `jwt.RegisteredClaims`; do not put email, display name, role, group
membership, CSRF nonce, or password state into the JWT.

Required claims:

| Claim | Value |
| --- | --- |
| `sub` | Canonical user UUID |
| `jti` | 32 random bytes encoded as unpadded URL-safe Base64 |
| `iat` | Current UTC time |
| `nbf` | Same as `iat` |
| `exp` | `iat + 168h` |
| `iss` | `settled-api` |
| `aud` | `settled-web` |

Validation must:

- Supply `jwt.WithValidMethods([]string{"HS256"})`.
- Require expiration, issued-at, subject, issuer, and audience.
- Reject empty or invalid UUID subjects and empty JWT IDs.
- Allow at most 30 seconds of clock skew.
- Reject tokens whose lifetime exceeds 168 hours plus allowed clock skew.
- Treat all validation failures as unauthenticated without exposing details.

All API replicas share the same current signing secret. Key rotation and multiple
`kid` values are outside the first release.

### Session Cookie

Use:

```text
Name:     settled_session
Value:    signed JWT
Path:     /api
Domain:   configured value or omitted
HttpOnly: true
Secure:   configuration
SameSite: configuration
Max-Age:  604800
Expires:  JWT expiration
```

Registration and login always issue a new JWT and therefore a new `jti`.
Successful requests do not refresh or replace the cookie. An expired session
requires login again.

Logout overwrites the cookie with an empty value, `Max-Age=-1`, and an expiration
in the past using the same Path, Domain, Secure, HttpOnly, and SameSite
attributes. Because sessions are stateless, logout does not revoke a copied JWT.

The authentication middleware loads the current user from PostgreSQL after JWT
validation. A valid JWT whose user no longer exists is treated as unauthenticated.
The middleware places only the verified user ID and loaded current user in a
private request context key.

## CSRF Design

CSRF protection uses an HttpOnly binding cookie plus a JavaScript-readable token
returned in JSON. It is a signed double-submit design and does not require
server-side session storage.

### Binding Cookie

The `settled_csrf` cookie value has three dot-separated fields:

```text
<base64url-nonce>.<unix-expiration>.<base64url-cookie-mac>
```

- Nonce is 32 bytes from `crypto/rand`.
- Anonymous binding expiration is one hour after issuance.
- Authenticated binding expiration equals the current JWT expiration.
- `cookie-mac` is `HMAC-SHA256(CSRF_SECRET, version || nonce || expiration)`.
- All variable-width values are length-prefixed before HMAC input construction.
- Version is a fixed byte value `1`.

The Cookie uses the same Path, Domain, Secure, HttpOnly, and SameSite settings as
the session cookie. Its Max-Age matches its binding expiration.

### Header Token

The token returned as `csrfToken` is:

```text
v1.<base64url-token-mac>
```

`token-mac` is:

```text
HMAC-SHA256(
  CSRF_SECRET,
  "token-v1" || complete_csrf_cookie_value || binding_subject,
)
```

`binding_subject` is:

- `"anonymous"` when no valid authenticated session exists.
- The current session JWT `jti` when authenticated.

Use length-prefixed fields and constant-time MAC comparison.

### CSRF Lifecycle

`GET /api/auth/csrf`:

- Optionally validates the session JWT.
- Reuses the existing CSRF cookie only if its MAC and expiration are valid and
  its lifetime does not exceed the appropriate anonymous/session limit.
- Otherwise issues a new cookie.
- Returns a token bound to `anonymous` or the current session `jti`.
- Does not create a session or user.

Registration and login:

- Require an anonymous-bound token.
- After success, issue the session JWT, rotate the CSRF nonce, and return a token
  bound to the new session `jti`.

Authenticated unsafe requests:

- Require the session cookie, CSRF cookie, and `X-CSRF-Token`.
- Validate both CSRF MACs, expiration, and binding to the JWT `jti`.

Logout:

- Requires the current authenticated CSRF token.
- Clears the session cookie.
- Rotates to a new one-hour anonymous CSRF binding.
- Returns `204` without returning the new token; the frontend must call the CSRF
  endpoint before its next unsafe anonymous request.

Missing cookie or header returns `csrf_required`; malformed, expired,
wrong-session, or invalid MAC returns `csrf_invalid`. Both use `403`.

## CORS And Origin Validation

Parse and canonicalize allowed origins at startup. An origin contains only
scheme, host, and optional port. Comparison is an exact string comparison after:

- Lowercasing scheme and hostname.
- Removing the default port (`:80` for HTTP and `:443` for HTTPS).
- Rejecting path, query, fragment, and user information.

For a request with an allowed `Origin`, return:

```http
Access-Control-Allow-Origin: <exact canonical origin>
Access-Control-Allow-Credentials: true
Vary: Origin
```

For preflight `OPTIONS`:

- Handle it before authentication and CSRF.
- Require an allowed Origin.
- Permit `GET, POST, PUT, PATCH, DELETE, OPTIONS`.
- Permit `Accept, Content-Type, Authorization, X-CSRF-Token`.
- Return `Access-Control-Max-Age: 600`.
- Return `204 No Content`.

The inclusion of `PATCH` preserves ordinary CORS compatibility even though the
first-release expense and repayment replacements use `PUT`.

Every unsafe method (`POST`, `PUT`, `PATCH`, `DELETE`) requires a present,
allowed `Origin`, including login and registration. A missing or disallowed
Origin returns `403 forbidden` before CSRF validation. Safe same-origin or
non-browser requests without `Origin` may proceed through normal authentication.

Do not trust `Referer`, `Host`, `X-Forwarded-Host`, or `Sec-Fetch-Site` as a
replacement for the configured Origin allowlist.

## HTTP API Construction

### Routing

Register Go 1.25 `ServeMux` patterns exactly matching `03_API.md`, for example:

```go
mux.HandleFunc("GET /api/health/live", api.live)
mux.HandleFunc("POST /api/auth/login", api.login)
mux.HandleFunc("GET /api/groups/{groupId}", api.getGroup)
mux.HandleFunc(
    "PUT /api/groups/{groupId}/expenses/{expenseId}",
    api.replaceExpense,
)
```

Unknown paths use the common JSON `404 not_found` response. Unsupported methods
use JSON `405 method_not_allowed` and an `Allow` header when ServeMux supplies
the method information. Redirecting path variants must not be relied on; the
documented paths are canonical.

### Middleware Order

The top-level chain is:

```text
panic recovery
  -> request ID and access logging
  -> API security headers
  -> CORS and preflight
  -> route handler
```

Each protected route then explicitly composes:

```text
required authentication
  -> unsafe Origin check, when applicable
  -> CSRF validation, when applicable
  -> handler
```

Auth CSRF retrieval uses optional authentication. Register and login use
anonymous CSRF validation. This explicit route composition prevents accidentally
making a new route public or omitting CSRF.

### Request IDs And Logging

- Accept an incoming `X-Request-ID` only when it is 1–128 visible ASCII
  characters; otherwise generate 16 random bytes as lowercase hex.
- Return the selected ID in `X-Request-ID`.
- Log one access event after completion with request ID, method, route pattern,
  status, response bytes, and duration.
- When authenticated, log the user UUID; never log cookies, Authorization,
  CSRF tokens, password, full request bodies, join codes, database URLs, or
  password hashes.
- Panic recovery logs the panic and stack with request ID, then returns the
  common `500 internal_error` response if headers are not committed.

### Security Headers

API responses include:

```http
X-Content-Type-Options: nosniff
Referrer-Policy: no-referrer
Cache-Control: no-store
```

JSON responses use `Content-Type: application/json; charset=utf-8`. `204`
responses have no body and no JSON content type.

### JSON Decoding

The shared decoder must:

1. Require `Content-Type: application/json` for endpoints with JSON bodies.
   Parameters such as `charset=utf-8` are allowed.
2. Wrap the body with `http.MaxBytesReader` at 1 MiB.
3. Call `DisallowUnknownFields`.
4. Decode exactly one top-level JSON object.
5. Reject trailing non-whitespace or a second JSON value.
6. Distinguish malformed or oversized JSON as `400 bad_request`.

Request DTO fields that must distinguish absent, null, and zero use pointer
fields. Domain services receive concrete values only after structural validation.

### JSON Encoding

Write the status before encoding. API DTOs carry explicit JSON tags and never
reuse database models. Time and date fields use dedicated DTO strings so wire
format does not depend on Go's default time encoding.

For error responses:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "One or more fields are invalid.",
    "fields": {
      "email": "Email is required."
    }
  }
}
```

Omit `fields` when empty. Field keys match API JSON request names.

## Error Model

Define stable error categories rather than matching database or error strings:

```go
var (
    ErrNotFound     = errors.New("not found")
    ErrUnauthorized = errors.New("unauthorized")
    ErrForbidden    = errors.New("forbidden")
    ErrConflict     = errors.New("conflict")
)

type ValidationError struct {
    Fields map[string]string
}
```

Domain packages may define more specific errors that wrap one category, such as
duplicate email, owner removal, or member-in-use. PostgreSQL constraint names are
translated inside `internal/store`; they never escape to handlers.

HTTP mapping:

| Internal category | HTTP | API code |
| --- | --- | --- |
| Malformed JSON, UUID, Basic Auth, or query | 400 | `bad_request` |
| Missing/invalid session or credentials | 401 | `unauthorized` |
| Disallowed role or Origin | 403 | `forbidden` |
| Missing CSRF material | 403 | `csrf_required` |
| Invalid/expired CSRF material | 403 | `csrf_invalid` |
| Invisible, dissolved, removed, or deleted resource | 404 | `not_found` |
| Duplicate email or member currently in accounting data | 409 | `conflict` |
| Domain field validation | 422 | `validation_failed` |
| Unknown error | 500 | `internal_error` |

The handler logs the wrapped internal error only for `500`. Client messages are
stable summaries and do not expose SQL, constraint names, stack traces, or
security verification details.

## Store Design

### Store Type

Use one concrete store:

```go
type Store struct {
    db *sql.DB
}
```

Domain services declare narrow interfaces in the consuming package. `*Store`
implicitly implements those interfaces. Do not create one interface containing
every store method.

Queries receive a caller context and use `QueryContext`, `QueryRowContext`, or
`ExecContext`. Every result set is closed and checked for iteration errors.
Every scanned column is explicit; never use `SELECT *` in application queries.

### Database Errors

Inspect errors with `errors.As` into `*pgconn.PgError`. Translate only known
constraint names:

- `users_email_lower_unique_idx` to duplicate-email conflict.
- `groups_join_code_unique_idx` to an internal join-code collision eligible for
  retry during group creation.
- Defined check constraints to validation or an internal invariant failure,
  depending on whether the input was already validated.
- Foreign-key errors caused by concurrent state changes to the appropriate
  hidden/not-found or conflict result.

An unknown PostgreSQL error remains wrapped as an internal error.

### Schema Application

`server/sql/schema.sql` contains the complete first-release schema from
`02_entities.md`, including:

- All six tables.
- Named constraints and indexes.
- `active_group_memberships`, `active_expenses`, and `active_repayments`.
- `settlement_debt_entries`, `pairwise_gross_balances`, and
  `pairwise_net_balances`.

The file starts with `BEGIN;` and ends with `COMMIT;`. It targets a fresh
database and is not repeatedly idempotent. Apply it with:

```sh
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f server/sql/schema.sql
```

The application does not automatically create, inspect, or mutate the schema.
Deployment must apply the schema before starting the API. Later schema evolution
requires a separate migration decision.

## Transaction And Locking Rules

Use PostgreSQL `READ COMMITTED`, the default isolation level. Correctness comes
from conditional writes, row locks, and database constraints.

When a workflow touches multiple entity types, acquire locks in this order:

1. The `groups` row.
2. Relevant `group_memberships` rows sorted by `user_id`.
3. The target `expenses` or `repayments` row.
4. Child split rows when replacement requires deletion.

No workflow may acquire the same categories in reverse order. Sorting member IDs
prevents two multi-member writes from locking the same memberships differently.

Use an internal transaction helper only for commit/rollback boilerplate:

```go
func (s *Store) withinTx(
    ctx context.Context,
    fn func(*sql.Tx) error,
) error
```

It starts a `READ COMMITTED` transaction, rolls back on function error, commits
on success, and preserves the original error. It is private to `store` and is
not a public unit-of-work abstraction.

### Group Creation

Within one transaction:

1. Generate group and join-code candidates before the insert.
2. Insert the group.
3. Insert the owner's membership.
4. Query the resulting summary.

Join codes use 12 characters from:

```text
23456789ABCDEFGHJKLMNPQRSTUVWXYZ
```

Generate characters with rejection sampling so every character is uniformly
distributed. On the named unique-index collision, generate a new code and retry
the entire transaction, up to five attempts. Exhaustion is an internal error.

### Joining A Group

Within one transaction:

1. Normalize the join code to uppercase after trimming whitespace.
2. Lock the matching non-dissolved group row `FOR UPDATE`.
3. Lock any existing `(group_id, user_id)` membership.
4. Insert a membership if absent.
5. If present and removed, set `removed_at = NULL` and `joined_at = now`.
6. If already active, make no state change.
7. Return the current group summary.

A missing or dissolved join code returns hidden `not found`. The workflow is
idempotent for an already-active member.

### Rename And Dissolve

Owner workflows first lock the non-dissolved group and verify:

- `owner_user_id` equals the actor.
- The actor's membership is active.

Rename updates normalized name and `updated_at`.

Dissolve conditionally sets `dissolved_at` and `updated_at` only when
`dissolved_at IS NULL`. Repeated dissolution behaves as `404`, consistent with
hidden-state rules.

### Member Removal

Within one transaction:

1. Lock the active group.
2. Verify the actor is its active owner.
3. Lock actor and target membership rows in user-ID order.
4. Reject removal when the target is the owner.
5. Check non-deleted expenses for target as payer or split participant.
6. Check non-deleted repayments for target as sender or recipient.
7. If any current reference exists, return the member-in-use conflict.
8. Set `removed_at` using a conditional update.

Creator-only fields do not block removal by themselves. This follows
`02_entities.md`: removal is blocked by settlement-affecting participation, while
historical `created_by_user_id` may continue to identify a removed user.

Expense/repayment creation and replacement lock all participant memberships, so
they cannot commit new target participation after the removal check and before
the removal update.

### Expense Creation And Replacement

The expense service validates structural input and calculates exact splits
before entering the Store.

Within the Store transaction:

1. Lock the non-dissolved group.
2. Lock actor, payer, and all split-participant memberships in sorted order.
3. Require every locked membership to be active.
4. On replacement, lock the visible non-deleted expense and require its group to
   match the path group.
5. Recheck that split amounts are positive, unique by user, overflow-safe, and
   sum exactly to the expense total.
6. Insert or update the expense.
7. For replacement, delete all prior `expense_splits`.
8. Insert every replacement split.
9. Return the complete expense.

The replacement preserves `id`, `group_id`, `created_by_user_id`, and
`created_at`; it updates every editable field and `updated_at`.

Soft delete locks the group and expense, verifies active actor membership, then
conditionally sets `deleted_at` and `updated_at`. Repeated delete returns `404`.

### Repayment Creation And Replacement

Follow the same group/member locking order as expenses. Lock the actor, sender,
and recipient memberships sorted by ID and require all active. Sender and
recipient must differ.

Replacement preserves `id`, `group_id`, `created_by_user_id`, and `created_at`.
Soft delete uses the same hidden and repeated-delete behavior as expenses.

## Expense Split Calculation

All split modes return an ordered `[]Split` of exact positive cents. Request
order is significant for deterministic remainder assignment and response order.
Duplicate participant IDs are rejected before calculation.

### Equal

Given amount `A` and `N` participants:

```text
base      = A / N
remainder = A % N
```

Assign `base + 1` to the first `remainder` participants and `base` to the rest.
Reject the request if `N == 0` or `A < N`, because the schema disallows zero-cent
split rows.

### Exact

Require every amount to be positive. Add with explicit overflow checks and
require the final sum to equal the expense amount exactly.

### Percentage

Require each basis-point value to be positive and the overflow-safe total to
equal 10000.

For every participant in request order:

```text
floorShare[i] = floor(amountCents * basisPoints[i] / 10000)
```

Calculate multiplication without overflow by using `math/big.Int` for this
small boundary calculation, then require the quotient to fit `int64`. Sum the
floor shares and distribute the remaining cents one at a time in request order.
Reject a result containing a zero-cent share, matching the persistence rule.

The persisted and returned splits are always exact amounts. The original
`splitMode`, participant list, and percentages are not stored.

## Read Queries

Every protected group read first establishes visibility using the actor's active
membership and a non-dissolved group. A missing group and an inaccessible group
produce the same Store error.

Lists are unpaginated for the first release and have stable ordering:

- Groups: `updated_at DESC, id DESC`.
- Expenses: `expense_date DESC, created_at DESC, id DESC`.
- Repayments: `repayment_date DESC, created_at DESC, id DESC`.
- Members: owner first, then `lower(display_name)`, then `user_id`.

Expense list loading must avoid one query per expense. Load visible expenses,
all their splits in one query using expense IDs or a group join, and active
member summaries, then assemble in Go. Repayments and settlements likewise load
member summaries once.

No SQL query returns `password_hash` unless the authentication service explicitly
requests the internal credential record.

## Settlement Calculation

The Store reads normalized rows from `settlement_debt_entries` for a visible
group. Each row means `from_user_id` owes `to_user_id` `amount_cents`.

The calculator interface is:

```go
type Calculator interface {
    Calculate(entries []DebtEntry) ([]Transfer, error)
}
```

`DebtEntry` and `Transfer` use canonical user-ID strings, positive `int64`
amounts, and currency `USD`.

The pairwise calculator:

1. Rejects invalid IDs, self-debt, non-positive amounts, non-USD currency, or
   overflow.
2. Aggregates entries by ordered `(from, to)` pair using checked `int64`
   addition.
3. Converts each pair to a canonical unordered key `(minUserID, maxUserID)`.
4. Adds debt from min to max and subtracts debt from max to min.
5. Emits no transfer for zero.
6. Emits the positive direction and absolute amount for non-zero results.
7. Sorts transfers by `amountCents DESC`, then `fromUserId ASC`, then
   `toUserId ASC`.

The calculator does not query PostgreSQL. The service verifies group visibility,
loads entries and member display summaries from the Store, invokes the
calculator, and returns the result.

### Diagnostic Settlement Views

The settlement views form a verification pipeline:

```text
active expenses and repayments
  -> settlement_debt_entries
  -> pairwise_gross_balances
  -> pairwise_net_balances
```

`pairwise_gross_balances` aggregates debt in each direction. Each row still
means `from_user_id` owes `to_user_id`, but all entries with the same group,
direction, and currency have been summed:

```sql
SELECT
  from_user_id,
  to_user_id,
  amount_cents,
  currency
FROM pairwise_gross_balances
WHERE group_id = $1
ORDER BY from_user_id, to_user_id, currency;
```

For example, if Alice owes Bob 3000 cents across several expenses and Bob owes
Alice 1000 cents through other activity, the gross view intentionally retains
both directional rows. It is used to verify entry aggregation and to diagnose
whether an incorrect result originated before or during pairwise netting.

`pairwise_net_balances` then offsets the two directions of every unordered user
pair. It returns only the final positive direction and omits zero balances:

```sql
SELECT
  from_user_id,
  to_user_id,
  amount_cents,
  currency
FROM pairwise_net_balances
WHERE group_id = $1
ORDER BY amount_cents DESC, from_user_id, to_user_id, currency;
```

The preceding gross example produces one net row saying Alice owes Bob 2000
cents.

These views have three first-release uses:

- `pairwise_gross_balances` verifies that normalized expense and repayment debt
  entries are aggregated correctly in each direction.
- `pairwise_net_balances` provides an implementation-independent reference for
  the final pairwise result returned by the Go calculator.
- Both views provide read-only diagnostic queries when a settlement result needs
  to be traced from final transfer back to directional debt.

The production settlement endpoint continues to read
`settlement_debt_entries` and invoke the Go calculator. It must not select its
result from `pairwise_net_balances`, because the Go calculation is the
replaceable business-logic boundary. Gross and net view queries are not added to
the production Store interface solely for testing; integration tests query them
through test-only SQL helpers.

When comparing SQL and Go results, tests normalize rows to the API's stable
order: `amount_cents DESC`, `from_user_id ASC`, `to_user_id ASC`, then currency.
They compare group ID, direction, amount, and currency exactly rather than
comparing only total amounts.

## Service Interfaces

Interfaces are declared by the package that consumes them and include only the
methods needed by that service. Representative shapes are:

```go
// auth
type UserStore interface {
    CreateUser(ctx context.Context, input NewUser) (User, error)
    FindCredentialsByEmail(ctx context.Context, email string) (Credentials, error)
    FindUserByID(ctx context.Context, userID string) (User, error)
}

// expenses
type Store interface {
    CreateExpense(ctx context.Context, input CreateInput) (Expense, error)
    ReplaceExpense(ctx context.Context, input ReplaceInput) (Expense, error)
    DeleteExpense(ctx context.Context, input DeleteInput) error
    GetExpense(ctx context.Context, actorID, groupID, expenseID string) (Expense, error)
    ListExpenses(ctx context.Context, actorID, groupID string) (ListResult, error)
}

// settlements
type Store interface {
    ListDebtEntries(
        ctx context.Context,
        actorID string,
        groupID string,
    ) ([]DebtEntry, []MemberSummary, error)
}
```

The final code may split large input structs into domain files, but it must
preserve these boundaries. Services do not expose `*sql.DB`, `*sql.Tx`,
`http.Request`, or API DTO types.

## Endpoint Implementation Rules

The route table and JSON shapes are exactly those in `03_API.md`. Additional
implementation rules are:

- `POST /api/auth/register` validates the anonymous CSRF token before performing
  Argon2id work and before checking email uniqueness.
- `POST /api/auth/login` uses `Request.BasicAuth`; malformed or missing Basic
  Auth is handled exactly as specified in `03_API.md`.
- `GET /api/me` loads the user through authentication middleware and never
  trusts profile data in the JWT.
- Group creation does not expose the join code in its response. The owner obtains
  it from the dedicated join-code endpoint.
- Join is idempotent and always returns `200`, including the first successful
  join.
- Group, expense, and repayment path IDs are checked together; a child ID from a
  different group returns `404`.
- Any active group member may create, replace, or delete any expense or
  repayment.
- `createdByUserId` is always derived from the authenticated actor and is never
  accepted from a request.
- Complete replacements require every editable field. Omitted fields produce
  `422 validation_failed`.
- Soft-deleted accounting records and dissolved groups are never returned from
  ordinary APIs.

## Health Checks

`GET /api/health/live` returns:

```json
{"status":"ok"}
```

It does not query PostgreSQL.

`GET /api/health/ready` calls `db.PingContext` with a two-second child timeout.
It returns `200 {"status":"ok"}` on success and
`503 {"status":"unavailable"}` on failure. It logs the database error at warning
level without including credentials.

Health responses pass through request ID, logging, security-header, and CORS
middleware but do not require authentication or CSRF.

## Testing Strategy

### Unit Tests

Run with:

```sh
cd server
go test ./...
```

They require no network, Docker, or database and cover:

- Configuration defaults, parsing, secret decoding, and every production
  rejection.
- UUID, join-code, and random-ID formatting with injectable readers.
- Email, display-name, password, text, date, and money validation.
- Argon2id encoding, correct/incorrect passwords, malformed PHC input, excessive
  stored parameters, and constant behavior for unknown users.
- JWT claims, strict algorithm selection, issuer/audience, expiration, future
  times, excessive lifetime, invalid subject, and wrong signature.
- CSRF anonymous/session binding, reuse, rotation, expiry, tampering, and
  cross-session rejection.
- Equal, exact, and percentage splits, deterministic remainders, duplicates,
  zero shares, invalid totals, and overflow boundaries.
- Pairwise simple debt, payer's excluded self-share as represented by the view,
  bidirectional netting, repayment reversal, multiple groups of users, zero
  omission, invalid entries, overflow, and stable ordering.
- Handler success shapes, strict decoding, route IDs, authentication, Origin,
  CSRF, hidden `404` behavior, and every error mapping using fake services.
- Middleware preflight behavior, response headers, request IDs, panic recovery,
  and logging redaction.

Use table-driven tests where cases share setup. Security tests use fixed clocks
and deterministic random readers. Handler tests use `httptest` and compare
decoded JSON values rather than whitespace.

### PostgreSQL Integration Tests

Integration files start with:

```go
//go:build integration
```

They read `TEST_DATABASE_URL`, apply no schema automatically, and fail clearly if
the variable is missing. They may truncate tables between tests but must not
drop or alter a database outside the dedicated test container.

`server/scripts/test-integration.sh`:

1. Verifies `docker`, `psql`, and the Go toolchain are available.
2. Generates a unique container name beginning with
   `settled-postgres-test-`.
3. Starts an ephemeral supported PostgreSQL image with no host data volume and a
   random published host port.
4. Registers a shell `trap` that removes only that exact container.
5. Polls `pg_isready` with a 30-second total deadline.
6. Builds `TEST_DATABASE_URL` for the dedicated `settled_test` database.
7. Applies `server/sql/schema.sql` with `ON_ERROR_STOP`.
8. Runs `go test -tags=integration ./...`.
9. Removes the container on success, failure, or interruption.

The script does not use `docker compose`, a fixed container name, a fixed host
port, or a persistent volume.

Integration coverage includes:

- Applying the complete schema to a fresh database.
- Case-insensitive email uniqueness.
- Group creation and owner membership rollback together.
- Join idempotency and removed-member reactivation.
- Owner authorization, owner removal rejection, and member-in-use conflict.
- Expense and repayment create/replace rollback on invalid participants.
- Concurrent member removal versus expense or repayment creation.
- Soft-delete and dissolved-group hidden-state queries.
- List ordering and bulk split assembly.
- Settlement debt-entry semantics, including payer self-share exclusion and
  repayment reversal.
- Directional aggregation in `pairwise_gross_balances`.
- Opposing-direction offset and zero omission in
  `pairwise_net_balances`.
- Exact agreement between the Go pairwise calculator, the SQL net view, and the
  settlement endpoint for representative fixtures.

### Layered Settlement Verification

Integration tests must verify settlement behavior at every stage instead of
checking only the final endpoint. A representative fixture should include at
least:

1. Alice pays 9000 cents, split 3000 cents each among Alice, Bob, and Carol.
2. Bob pays 2400 cents, split 1200 cents each between Alice and Bob.
3. Bob records a 1000-cent repayment to Alice.

For this fixture, tests assert:

| Stage | Expected rows |
| --- | --- |
| Debt entries from Alice's expense | Bob owes Alice 3000; Carol owes Alice 3000; Alice's self-share is absent |
| Debt entries from Bob's expense | Alice owes Bob 1200; Bob's self-share is absent |
| Debt entry from Bob's repayment to Alice | Alice owes Bob 1000 |
| Gross balances | Bob owes Alice 3000; Carol owes Alice 3000; Alice owes Bob 2200 |
| Net balances | Bob owes Alice 800; Carol owes Alice 3000 |

The test procedure is:

1. Insert the fixture only through Store workflows so membership checks,
   transaction behavior, and persisted splits are exercised.
2. Query `settlement_debt_entries` and compare source-level rows, including the
   originating dates where useful. Do not assume view row order.
3. Query `pairwise_gross_balances` and compare an ordered-pair map keyed by
   `(group_id, from_user_id, to_user_id, currency)`.
4. Independently sum the queried debt entries in Go and assert that every
   directional total exactly equals the gross-view map. This verifies the gross
   view rather than merely snapshotting its output.
5. Query `pairwise_net_balances` and compare an unordered-pair net derived
   independently from the gross rows. Assert that each non-zero pair has exactly
   one direction and that zero pairs have no row.
6. Pass the original debt entries to the production Go calculator, normalize
   both outputs to the documented order, and require exact equality with the net
   view.
7. Call `GET /api/groups/{groupId}/settlements` as an active member and require
   its transfers to equal the Go calculator result. Separately confirm that a
   non-member receives `404`.

Additional settlement integration cases must:

- Create equal opposing gross balances and verify that the gross view retains
  both directions while the net view, Go calculator, and endpoint all omit the
  pair.
- Soft-delete an expense and a repayment in separate subtests, then verify that
  all three views and the Go result change immediately and consistently.
- Dissolve the group and verify that active debt, gross, and net views return no
  rows for it, while the endpoint returns `404`.
- Create activity in a second group using some of the same users and verify that
  every view and calculation remains isolated by `group_id`.
- Include multiple expenses and repayments in the same direction so the gross
  sum is verified across more than one source row.

If a layered assertion fails, report the first failing boundary:

- Wrong `settlement_debt_entries`: expense/repayment normalization or active
  filtering is incorrect.
- Correct debt entries but wrong gross rows: directional SQL aggregation is
  incorrect.
- Correct gross rows but wrong net rows: SQL pairwise offset is incorrect.
- Correct SQL net rows but wrong Go result: calculator aggregation, direction,
  overflow handling, or sorting is incorrect.
- Correct Go result but wrong endpoint: authorization, Store loading, DTO
  mapping, or response ordering is incorrect.

### Race And Static Checks

Run:

```sh
cd server
gofmt -w <changed-go-files>
go vet ./...
go test ./...
go test -race ./...
./scripts/test-integration.sh
```

The implementation handoff must report each command and result. If Docker or
PostgreSQL tools are unavailable, report the exact missing prerequisite rather
than silently skipping integration tests.

## Implementation Sequence

Implement the backend in this dependency order:

1. Configuration, logging, random ID helpers, process lifecycle, and health
   endpoints.
2. PostgreSQL connection, full schema file, concrete Store foundation, and
   integration-test script.
3. Password hashing, JWT sessions, CSRF, CORS, Origin validation, JSON helpers,
   and shared middleware.
4. User Store, registration, login, logout, and current-user endpoints.
5. Group creation/list/join/detail and owner workflows.
6. Expense split calculation and expense endpoints.
7. Repayment workflows and endpoints.
8. Pure Go settlement calculator and settlement endpoint.
9. Full integration, race, failure-path, and production-config checks.

Each step must leave `go test ./...` passing. Schema changes during this initial
implementation update the single fresh-database `schema.sql`; they do not add an
unapproved migration framework.

## Explicit First-Release Boundaries

- No server-side session table, JWT revocation list, refresh token, or sliding
  session.
- No key-ring or online signing-key rotation.
- No email verification, password reset, OAuth, or two-factor authentication.
- No rate limiter or distributed cache. Authentication rate limiting requires a
  separate horizontally consistent design.
- No ownership transfer or join-code rotation.
- No archive, recovery, audit, or hard-delete API.
- No pagination until real first-release activity requires an API revision.
- No global minimum-transfer settlement algorithm.
- No automatic database migrations.
- No frontend, deployment-container, Tauri, Capacitor, or offline mutation work
  in this backend design.
