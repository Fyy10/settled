# Settled API Design

This document defines the first-release HTTP API contract for Settled. It follows
the product scope in `00_project.md`, the runtime architecture in
`01_architecture.md`, and the database entity design in `02_entities.md`.

The API is a JSON REST-style API served by the Go backend. It is designed for the
static SvelteKit/PWA frontend and uses secure HttpOnly cookie authentication with
CSRF protection for state-changing requests.

## Design Goals

- Make frontend and backend implementation possible without guessing endpoint
  shapes.
- Keep the API small, explicit, and workflow-oriented.
- Keep authentication state out of frontend JavaScript.
- Expose only current first-release user-facing state:
  - active memberships
  - non-dissolved groups
  - non-deleted expenses
  - non-deleted repayments
- Keep settlement results derived and stable even if the internal settlement
  strategy changes later.

## Base URL And Versioning

All application endpoints are rooted under:

```text
/api
```

First-release endpoints are not versioned in the path. If future incompatible
changes become necessary, add versioning deliberately rather than preemptively.

## Common Request Conventions

### Content Type

JSON request bodies must use:

```http
Content-Type: application/json
```

JSON responses use:

```http
Content-Type: application/json
```

### Authentication

Authenticated endpoints require the browser to include the backend-issued
HttpOnly authentication cookie.

Frontend requests must use credentialed fetch behavior:

```ts
fetch(url, { credentials: "include" })
```

The frontend must not store bearer tokens in `localStorage`, `sessionStorage`, or
JavaScript-readable cookies.

### CSRF Protection

Unsafe methods require a CSRF token:

- `POST`
- `PUT`
- `PATCH`
- `DELETE`

Safe `GET` requests do not require a CSRF token, including authenticated reads.
They still require normal authentication, authorization, and CORS checks where
applicable.

The frontend obtains a token from `GET /api/auth/csrf` and sends it on unsafe
requests:

```http
X-CSRF-Token: <csrf_token>
```

The backend binds CSRF tokens to a browser cookie so login and registration can
be protected before an authenticated session exists. The CSRF cookie is not an
authentication cookie and grants no user access. When an authenticated session
exists, the current CSRF token is bound to that session.

Unsafe browser requests must pass both CSRF validation and configured frontend
origin validation.

CSRF flow:

1. The frontend calls `GET /api/auth/csrf` before any unsafe request.
2. If no authenticated session exists, the backend creates or refreshes an
   anonymous CSRF cookie and returns a matching token.
3. `POST /api/auth/register` and `POST /api/auth/login` require the returned
   token and a valid allowed `Origin`.
4. Successful registration or login creates the authenticated session and
   rotates the CSRF token.
5. Later unsafe authenticated requests use the current session-bound CSRF token.
6. Logout clears the authenticated session and clears or rotates the CSRF token.

If an unsafe request fails with `csrf_required` or `csrf_invalid`, the frontend
should call `GET /api/auth/csrf` to refresh the token and retry the original
request once. If the retry also fails, the frontend should surface the error
instead of retrying again.

### Common Headers

Public safe requests:

```http
Accept: application/json
```

Authenticated safe requests:

```http
Accept: application/json
Cookie: settled_session=<http_only_cookie>
```

Pre-auth unsafe requests, such as register and login:

```http
Accept: application/json
Content-Type: application/json
Cookie: settled_csrf=<http_only_cookie>
X-CSRF-Token: <csrf_token>
```

Authenticated unsafe requests:

```http
Accept: application/json
Content-Type: application/json
Cookie: settled_session=<http_only_cookie>; settled_csrf=<http_only_cookie>
X-CSRF-Token: <csrf_token>
```

Cookie names are implementation details, but examples in this document use
`settled_session` for authentication and `settled_csrf` for CSRF binding.

### IDs, Dates, And Money

- IDs are UUID strings.
- Dates use ISO `YYYY-MM-DD`.
- Timestamps use RFC 3339 strings.
- Money is represented as integer cents.
- First-release currency is always `USD`.

## Common Response Shapes

### Error Response

All error responses should use this shape:

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

Fields:

- `code`: stable machine-readable string.
- `message`: user-presentable summary.
- `fields`: optional map for field-level validation errors.

Recommended common error codes:

```text
bad_request
unauthorized
forbidden
not_found
conflict
csrf_required
csrf_invalid
validation_failed
internal_error
```

### Current User

```json
{
  "id": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
  "email": "alice@example.com",
  "displayName": "Alice",
  "createdAt": "2026-06-30T18:00:00Z",
  "updatedAt": "2026-06-30T18:00:00Z"
}
```

The API must never return `password_hash`.

### Group Summary

```json
{
  "id": "9a41c3a3-169f-4219-998b-822d31352f90",
  "name": "Lake Trip",
  "ownerUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
  "memberCount": 3,
  "currentUserRole": "owner",
  "createdAt": "2026-06-30T18:00:00Z",
  "updatedAt": "2026-06-30T18:00:00Z"
}
```

`currentUserRole` is derived from active membership and is either `owner` or
`member`.

### Group Member

```json
{
  "userId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
  "email": "alice@example.com",
  "displayName": "Alice",
  "role": "owner",
  "joinedAt": "2026-06-30T18:00:00Z"
}
```

Expense and repayment list responses may include a member summary array so the
frontend can render names without a separate group detail request:

```json
{
  "userId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
  "displayName": "Alice"
}
```

### Expense

```json
{
  "id": "d87d80c8-c6bc-42b1-9e8f-6b99d29fa8d6",
  "groupId": "9a41c3a3-169f-4219-998b-822d31352f90",
  "paidByUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
  "description": "Groceries",
  "amountCents": 5400,
  "currency": "USD",
  "expenseDate": "2026-06-30",
  "createdByUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
  "splits": [
    {
      "userId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
      "amountCents": 1800
    },
    {
      "userId": "6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2",
      "amountCents": 1800
    },
    {
      "userId": "0c77b782-60f2-48e8-ad29-dd172a181d43",
      "amountCents": 1800
    }
  ],
  "createdAt": "2026-06-30T18:00:00Z",
  "updatedAt": "2026-06-30T18:00:00Z"
}
```

### Repayment

```json
{
  "id": "21feeb5e-ff5c-44ec-b0bc-8daae2d0522f",
  "groupId": "9a41c3a3-169f-4219-998b-822d31352f90",
  "fromUserId": "6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2",
  "toUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
  "amountCents": 2000,
  "currency": "USD",
  "note": "Venmo",
  "repaymentDate": "2026-06-30",
  "createdByUserId": "6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2",
  "createdAt": "2026-06-30T18:00:00Z",
  "updatedAt": "2026-06-30T18:00:00Z"
}
```

## Status Code Conventions

- `200 OK`: successful read or update with response body.
- `201 Created`: successful create with response body.
- `204 No Content`: successful delete/logout with no response body.
- `400 Bad Request`: malformed JSON, invalid path/query parameter, or invalid
  request shape.
- `401 Unauthorized`: missing or invalid authentication.
- `403 Forbidden`: authenticated but not allowed, including invalid CSRF token.
- `404 Not Found`: resource does not exist or is hidden from the current user.
- `409 Conflict`: request conflicts with current state, such as duplicate email or
  invalid group join state.
- `422 Unprocessable Entity`: syntactically valid request with validation errors.
- `500 Internal Server Error`: unexpected server failure.

Use `404 Not Found` instead of `403 Forbidden` when revealing that a group,
expense, or repayment exists would leak private information.

## Health Endpoints

### GET /api/health/live

Function: Liveness check. Confirms the HTTP process is running.

Authentication: none.

Response `200 OK`:

```json
{
  "status": "ok"
}
```

### GET /api/health/ready

Function: Readiness check. Confirms required dependencies, especially PostgreSQL,
are reachable.

Authentication: none.

Response `200 OK`:

```json
{
  "status": "ok"
}
```

Response `503 Service Unavailable`:

```json
{
  "status": "unavailable"
}
```

## Auth Endpoints

### GET /api/auth/csrf

Function: Returns a CSRF token for unsafe requests and creates or refreshes the
CSRF binding cookie.

Authentication: optional. The endpoint may be called before login so the frontend
can submit registration or login forms.

Headers:

```http
Accept: application/json
```

Response `200 OK`:

```json
{
  "csrfToken": "opaque_csrf_token"
}
```

Response headers may include:

```http
Set-Cookie: settled_csrf=<opaque>; HttpOnly; Secure; SameSite=None; Path=/api
```

Behavior:

- If no authenticated session exists, the token is bound to an anonymous CSRF
  cookie.
- If an authenticated session exists, the token is bound to that authenticated
  session.
- Calling this endpoint does not create an authenticated session.
- The returned token should expire and may be rotated by later unsafe auth
  workflows.

Errors:

- `500 Internal Server Error`

### POST /api/auth/register

Function: Creates a user account, creates an authenticated session, and returns
the current user.

Authentication: none.

Required headers:

```http
Content-Type: application/json
X-CSRF-Token: <csrf_token>
```

Request body:

```json
{
  "email": "alice@example.com",
  "password": "correct horse battery staple",
  "displayName": "Alice"
}
```

Validation:

- `email` is required, valid enough for login, and unique case-insensitively.
- `password` is required and must satisfy backend password rules.
- `displayName` is required, non-blank, and at most 120 characters.

Response `201 Created`:

```json
{
  "csrfToken": "new_csrf_token",
  "user": {
    "id": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
    "email": "alice@example.com",
    "displayName": "Alice",
    "createdAt": "2026-06-30T18:00:00Z",
    "updatedAt": "2026-06-30T18:00:00Z"
  }
}
```

Response headers:

```http
Set-Cookie: settled_session=<opaque>; HttpOnly; Secure; SameSite=None; Path=/api
Set-Cookie: settled_csrf=<opaque>; HttpOnly; Secure; SameSite=None; Path=/api
```

Behavior:

- The request must include a valid pre-auth CSRF token and a valid allowed
  `Origin`.
- Successful registration creates the authenticated session and rotates the CSRF
  token. The response body returns the new token for later unsafe requests.

Errors:

- `403 Forbidden`: missing or invalid CSRF token.
- `409 Conflict`: email already exists.
- `422 Unprocessable Entity`: validation failed.

### POST /api/auth/login

Function: Authenticates a user, creates a session cookie, and returns the current
user.

Authentication: none.

Required headers:

```http
Authorization: Basic <base64_email_colon_password>
X-CSRF-Token: <csrf_token>
```

The Basic Auth username is the user's email address. The Basic Auth password is
the user's password. The request has no JSON body.

Example:

```http
Authorization: Basic YWxpY2VAZXhhbXBsZS5jb206Y29ycmVjdCBob3JzZSBiYXR0ZXJ5IHN0YXBsZQ==
```

Response `200 OK`:

```json
{
  "csrfToken": "new_csrf_token",
  "user": {
    "id": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
    "email": "alice@example.com",
    "displayName": "Alice",
    "createdAt": "2026-06-30T18:00:00Z",
    "updatedAt": "2026-06-30T18:00:00Z"
  }
}
```

Response headers:

```http
Set-Cookie: settled_session=<opaque>; HttpOnly; Secure; SameSite=None; Path=/api
Set-Cookie: settled_csrf=<opaque>; HttpOnly; Secure; SameSite=None; Path=/api
```

Behavior:

- The request must include a valid pre-auth CSRF token and a valid allowed
  `Origin`.
- Successful login creates the authenticated session and rotates the CSRF token.
  The response body returns the new token for later unsafe requests.

Errors:

- `400 Bad Request`: malformed Basic Auth header.
- `401 Unauthorized`: missing credentials, invalid email, or invalid password.
- `403 Forbidden`: missing or invalid CSRF token.

### POST /api/auth/logout

Function: Clears the current session cookie.

Authentication: required.

Required headers:

```http
X-CSRF-Token: <csrf_token>
```

Request body: none.

Response `204 No Content`

Response headers:

```http
Set-Cookie: settled_session=; HttpOnly; Secure; SameSite=None; Path=/api; Max-Age=0
Set-Cookie: settled_csrf=<opaque>; HttpOnly; Secure; SameSite=None; Path=/api
```

Behavior:

- Logout requires the current session-bound CSRF token.
- Successful logout clears the authenticated session and clears or rotates the
  CSRF token. The response body remains empty.

Errors:

- `401 Unauthorized`
- `403 Forbidden`: missing or invalid CSRF token.

### GET /api/me

Function: Returns the authenticated user. The frontend uses this endpoint as the
source of truth for logged-in state.

Authentication: required.

Response `200 OK`:

```json
{
  "user": {
    "id": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
    "email": "alice@example.com",
    "displayName": "Alice",
    "createdAt": "2026-06-30T18:00:00Z",
    "updatedAt": "2026-06-30T18:00:00Z"
  }
}
```

Errors:

- `401 Unauthorized`

## Group Endpoints

### GET /api/groups

Function: Lists non-dissolved groups where the current user has active
membership.

Authentication: required.

Response `200 OK`:

```json
{
  "groups": [
    {
      "id": "9a41c3a3-169f-4219-998b-822d31352f90",
      "name": "Lake Trip",
      "ownerUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
      "memberCount": 3,
      "currentUserRole": "owner",
      "createdAt": "2026-06-30T18:00:00Z",
      "updatedAt": "2026-06-30T18:00:00Z"
    }
  ]
}
```

Errors:

- `401 Unauthorized`

### POST /api/groups

Function: Creates a group and adds the current user as owner in one transaction.

Authentication: required.

Required headers:

```http
Content-Type: application/json
X-CSRF-Token: <csrf_token>
```

Request body:

```json
{
  "name": "Lake Trip"
}
```

Validation:

- `name` is required, non-blank, and at most 160 characters.

Response `201 Created`:

```json
{
  "group": {
    "id": "9a41c3a3-169f-4219-998b-822d31352f90",
    "name": "Lake Trip",
    "ownerUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
    "memberCount": 1,
    "currentUserRole": "owner",
    "createdAt": "2026-06-30T18:00:00Z",
    "updatedAt": "2026-06-30T18:00:00Z"
  }
}
```

Errors:

- `401 Unauthorized`
- `403 Forbidden`: missing or invalid CSRF token.
- `422 Unprocessable Entity`: validation failed.

### POST /api/groups/join

Function: Joins the current user to an existing non-dissolved group by unique
join code.

Authentication: required.

Required headers:

```http
Content-Type: application/json
X-CSRF-Token: <csrf_token>
```

Request body:

```json
{
  "joinCode": "ABCD1234"
}
```

Validation:

- `joinCode` is required and non-blank.

Behavior:

- If the user has an existing removed membership, rejoining clears
  `removed_at`.
- If the user is already an active member, return the current group summary.
- Dissolved groups are hidden and cannot be joined.

Response `200 OK`:

```json
{
  "group": {
    "id": "9a41c3a3-169f-4219-998b-822d31352f90",
    "name": "Lake Trip",
    "ownerUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
    "memberCount": 3,
    "currentUserRole": "member",
    "createdAt": "2026-06-30T18:00:00Z",
    "updatedAt": "2026-06-30T18:00:00Z"
  }
}
```

Errors:

- `401 Unauthorized`
- `403 Forbidden`: missing or invalid CSRF token.
- `404 Not Found`: join code does not match a joinable group.
- `422 Unprocessable Entity`: validation failed.

### GET /api/groups/{groupId}

Function: Returns details for a non-dissolved group where the current user has
active membership.

Authentication: required.

Path parameters:

- `groupId`: UUID.

Response `200 OK`:

```json
{
  "group": {
    "id": "9a41c3a3-169f-4219-998b-822d31352f90",
    "name": "Lake Trip",
    "ownerUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
    "memberCount": 3,
    "currentUserRole": "owner",
    "createdAt": "2026-06-30T18:00:00Z",
    "updatedAt": "2026-06-30T18:00:00Z"
  },
  "members": [
    {
      "userId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
      "email": "alice@example.com",
      "displayName": "Alice",
      "role": "owner",
      "joinedAt": "2026-06-30T18:00:00Z"
    }
  ]
}
```

Errors:

- `401 Unauthorized`
- `404 Not Found`: group does not exist, is dissolved, or is hidden from the
  current user.

### PATCH /api/groups/{groupId}

Function: Renames a group.

Authentication: owner only.

Path parameters:

- `groupId`: UUID.

Required headers:

```http
Content-Type: application/json
X-CSRF-Token: <csrf_token>
```

Request body:

```json
{
  "name": "Beach Trip"
}
```

Validation:

- `name` is required, non-blank, and at most 160 characters.

Response `200 OK`:

```json
{
  "group": {
    "id": "9a41c3a3-169f-4219-998b-822d31352f90",
    "name": "Beach Trip",
    "ownerUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
    "memberCount": 3,
    "currentUserRole": "owner",
    "createdAt": "2026-06-30T18:00:00Z",
    "updatedAt": "2026-06-30T19:00:00Z"
  }
}
```

Errors:

- `401 Unauthorized`
- `403 Forbidden`: not owner, or missing/invalid CSRF token.
- `404 Not Found`: group does not exist, is dissolved, or is hidden.
- `422 Unprocessable Entity`: validation failed.

### DELETE /api/groups/{groupId}

Function: Dissolves a group. Dissolved groups are hidden from ordinary first
release APIs and cannot accept joins, expenses, repayments, or edits.

Authentication: owner only.

Path parameters:

- `groupId`: UUID.

Required headers:

```http
X-CSRF-Token: <csrf_token>
```

Request body: none.

Response `204 No Content`

Errors:

- `401 Unauthorized`
- `403 Forbidden`: not owner, or missing/invalid CSRF token.
- `404 Not Found`: group does not exist, is dissolved, or is hidden.

### GET /api/groups/{groupId}/join-code

Function: Returns the group join code so the owner can invite trusted friends.

Authentication: owner only.

Path parameters:

- `groupId`: UUID.

Response `200 OK`:

```json
{
  "joinCode": "ABCD1234"
}
```

Errors:

- `401 Unauthorized`
- `403 Forbidden`: authenticated user is not the owner.
- `404 Not Found`: group does not exist, is dissolved, or is hidden.

Join code rotation is out of scope for the first release.

### DELETE /api/groups/{groupId}/members/{userId}

Function: Removes a member from a group by setting `removed_at`. This workflow is
intended for removing someone who was added to the wrong group before they
participate in shared accounting activity. Removed members lose access to the
group and its first-release API-visible history.

The backend must reject removal if the target member appears in any current
non-deleted expense or repayment for the group as payer, split participant,
repayment sender, or repayment recipient. The owner must first delete those
records or, where the relevant participant field is editable, update them so the
member is no longer part of any current accounting activity.

Authentication: owner only.

Path parameters:

- `groupId`: UUID.
- `userId`: UUID of the member to remove.

Required headers:

```http
X-CSRF-Token: <csrf_token>
```

Request body: none.

Response `204 No Content`

Errors:

- `401 Unauthorized`
- `403 Forbidden`: not owner, trying to remove the owner, or missing/invalid
  CSRF token.
- `404 Not Found`: group/member does not exist, group is dissolved, or group is
  hidden.
- `409 Conflict`: target member participates in a current expense or repayment.

## Expense Endpoints

### GET /api/groups/{groupId}/expenses

Function: Lists non-deleted expenses for a visible active group.

Authentication: active group member.

Path parameters:

- `groupId`: UUID.

Results are ordered by `expenseDate` descending, then `createdAt` descending,
then `id` descending.

Response `200 OK`:

```json
{
  "expenses": [
    {
      "id": "d87d80c8-c6bc-42b1-9e8f-6b99d29fa8d6",
      "groupId": "9a41c3a3-169f-4219-998b-822d31352f90",
      "paidByUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
      "description": "Groceries",
      "amountCents": 5400,
      "currency": "USD",
      "expenseDate": "2026-06-30",
      "createdByUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
      "splits": [
        {
          "userId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
          "amountCents": 1800
        }
      ],
      "createdAt": "2026-06-30T18:00:00Z",
      "updatedAt": "2026-06-30T18:00:00Z"
    }
  ],
  "members": [
    {
      "userId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
      "displayName": "Alice"
    }
  ]
}
```

`members` includes active group members needed to display payer and split
participant names for the returned expenses. It is provided as a display helper.
Authorization must still be based on the current session and server-side
membership checks.

Errors:

- `401 Unauthorized`
- `404 Not Found`: group does not exist, is dissolved, or is hidden.

### POST /api/groups/{groupId}/expenses

Function: Creates an expense with exact persisted split amounts.

Authentication: active group member.

Path parameters:

- `groupId`: UUID.

Required headers:

```http
Content-Type: application/json
X-CSRF-Token: <csrf_token>
```

Request body for exact split:

```json
{
  "paidByUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
  "description": "Groceries",
  "amountCents": 5400,
  "expenseDate": "2026-06-30",
  "splitMode": "exact",
  "splits": [
    {
      "userId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
      "amountCents": 1800
    },
    {
      "userId": "6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2",
      "amountCents": 1800
    },
    {
      "userId": "0c77b782-60f2-48e8-ad29-dd172a181d43",
      "amountCents": 1800
    }
  ]
}
```

Request body for equal split across selected participants:

```json
{
  "paidByUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
  "description": "Groceries",
  "amountCents": 5400,
  "expenseDate": "2026-06-30",
  "splitMode": "equal",
  "participantUserIds": [
    "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
    "6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2",
    "0c77b782-60f2-48e8-ad29-dd172a181d43"
  ]
}
```

For `equal`, `participantUserIds` is required. The frontend may offer an
"all members" shortcut, but it must expand that shortcut into explicit
participant IDs before calling the API.

Request body for percentage split:

```json
{
  "paidByUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
  "description": "Groceries",
  "amountCents": 5400,
  "expenseDate": "2026-06-30",
  "splitMode": "percentage",
  "percentageSplits": [
    {
      "userId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
      "percentageBasisPoints": 3334
    },
    {
      "userId": "6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2",
      "percentageBasisPoints": 3333
    },
    {
      "userId": "0c77b782-60f2-48e8-ad29-dd172a181d43",
      "percentageBasisPoints": 3333
    }
  ]
}
```

Validation:

- `paidByUserId` must be an active member of the group.
- `description` is required, non-blank, and at most 240 characters.
- `amountCents` must be greater than 0.
- `expenseDate` is required.
- `splitMode` is `equal`, `exact`, or `percentage`.
- Split participants must be active group members.
- Split participant lists must be non-empty and must not contain duplicate
  users.
- Equal split requests must include explicit `participantUserIds`.
- Exact split amounts must sum to `amountCents`.
- Percentage split basis points must sum to `10000`.
- The backend converts equal and percentage inputs to exact cents before
  persistence.
- Remainders are assigned deterministically by the request's participant order.

Response `201 Created`:

```json
{
  "expense": {
    "id": "d87d80c8-c6bc-42b1-9e8f-6b99d29fa8d6",
    "groupId": "9a41c3a3-169f-4219-998b-822d31352f90",
    "paidByUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
    "description": "Groceries",
    "amountCents": 5400,
    "currency": "USD",
    "expenseDate": "2026-06-30",
    "createdByUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
    "splits": [
      {
        "userId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
        "amountCents": 1800
      }
    ],
    "createdAt": "2026-06-30T18:00:00Z",
    "updatedAt": "2026-06-30T18:00:00Z"
  }
}
```

Errors:

- `401 Unauthorized`
- `403 Forbidden`: missing or invalid CSRF token.
- `404 Not Found`: group does not exist, is dissolved, or is hidden.
- `422 Unprocessable Entity`: validation failed.

### GET /api/groups/{groupId}/expenses/{expenseId}

Function: Returns a non-deleted expense.

Authentication: active group member.

Path parameters:

- `groupId`: UUID.
- `expenseId`: UUID.

Response `200 OK`:

```json
{
  "expense": {
    "id": "d87d80c8-c6bc-42b1-9e8f-6b99d29fa8d6",
    "groupId": "9a41c3a3-169f-4219-998b-822d31352f90",
    "paidByUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
    "description": "Groceries",
    "amountCents": 5400,
    "currency": "USD",
    "expenseDate": "2026-06-30",
    "createdByUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
    "splits": [
      {
        "userId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
        "amountCents": 1800
      }
    ],
    "createdAt": "2026-06-30T18:00:00Z",
    "updatedAt": "2026-06-30T18:00:00Z"
  }
}
```

Errors:

- `401 Unauthorized`
- `404 Not Found`: group or expense does not exist, is deleted/dissolved, or is
  hidden.

### PUT /api/groups/{groupId}/expenses/{expenseId}

Function: Replaces an expense and its splits in one transaction. Any active
group member can replace any group expense.

Authentication: active group member.

Path parameters:

- `groupId`: UUID.
- `expenseId`: UUID.

Required headers:

```http
Content-Type: application/json
X-CSRF-Token: <csrf_token>
```

Request body: same shape as `POST /api/groups/{groupId}/expenses`. This is a
full replacement; all editable fields are required.

Response `200 OK`:

```json
{
  "expense": {
    "id": "d87d80c8-c6bc-42b1-9e8f-6b99d29fa8d6",
    "groupId": "9a41c3a3-169f-4219-998b-822d31352f90",
    "paidByUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
    "description": "Dinner",
    "amountCents": 6000,
    "currency": "USD",
    "expenseDate": "2026-06-30",
    "createdByUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
    "splits": [
      {
        "userId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
        "amountCents": 2000
      }
    ],
    "createdAt": "2026-06-30T18:00:00Z",
    "updatedAt": "2026-06-30T19:00:00Z"
  }
}
```

Errors:

- `401 Unauthorized`
- `403 Forbidden`: missing or invalid CSRF token.
- `404 Not Found`: group or expense does not exist, is deleted/dissolved, or is
  hidden.
- `422 Unprocessable Entity`: validation failed.

### DELETE /api/groups/{groupId}/expenses/{expenseId}

Function: Soft-deletes an expense. Deleted expenses are hidden from all ordinary
first-release APIs and excluded from settlement results.

Authentication: active group member.

Path parameters:

- `groupId`: UUID.
- `expenseId`: UUID.

Required headers:

```http
X-CSRF-Token: <csrf_token>
```

Request body: none.

Response `204 No Content`

Errors:

- `401 Unauthorized`
- `403 Forbidden`: missing or invalid CSRF token.
- `404 Not Found`: group or expense does not exist, is deleted/dissolved, or is
  hidden.

## Repayment Endpoints

### GET /api/groups/{groupId}/repayments

Function: Lists non-deleted repayments for a visible active group.

Authentication: active group member.

Path parameters:

- `groupId`: UUID.

Results are ordered by `repaymentDate` descending, then `createdAt` descending,
then `id` descending.

Response `200 OK`:

```json
{
  "repayments": [
    {
      "id": "21feeb5e-ff5c-44ec-b0bc-8daae2d0522f",
      "groupId": "9a41c3a3-169f-4219-998b-822d31352f90",
      "fromUserId": "6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2",
      "toUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
      "amountCents": 2000,
      "currency": "USD",
      "note": "Venmo",
      "repaymentDate": "2026-06-30",
      "createdByUserId": "6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2",
      "createdAt": "2026-06-30T18:00:00Z",
      "updatedAt": "2026-06-30T18:00:00Z"
    }
  ],
  "members": [
    {
      "userId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
      "displayName": "Alice"
    },
    {
      "userId": "6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2",
      "displayName": "Bob"
    }
  ]
}
```

`members` includes active group members needed to display repayment sender and
recipient names for the returned repayments. It is provided as a display helper.
Authorization must still be based on the current session and server-side
membership checks.

Errors:

- `401 Unauthorized`
- `404 Not Found`: group does not exist, is dissolved, or is hidden.

### POST /api/groups/{groupId}/repayments

Function: Records a manual repayment made outside the app.

Authentication: active group member.

Path parameters:

- `groupId`: UUID.

Required headers:

```http
Content-Type: application/json
X-CSRF-Token: <csrf_token>
```

Request body:

```json
{
  "fromUserId": "6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2",
  "toUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
  "amountCents": 2000,
  "note": "Venmo",
  "repaymentDate": "2026-06-30"
}
```

Validation:

- `fromUserId` and `toUserId` must be active group members.
- `fromUserId` and `toUserId` must be different.
- `amountCents` must be greater than 0.
- `note` is optional and at most 240 characters.
- `repaymentDate` is required.

`createdByUserId` is set by the backend to the current authenticated user. It may
be different from `fromUserId` when one member records a repayment on behalf of
another member.

Response `201 Created`:

```json
{
  "repayment": {
    "id": "21feeb5e-ff5c-44ec-b0bc-8daae2d0522f",
    "groupId": "9a41c3a3-169f-4219-998b-822d31352f90",
    "fromUserId": "6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2",
    "toUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
    "amountCents": 2000,
    "currency": "USD",
    "note": "Venmo",
    "repaymentDate": "2026-06-30",
    "createdByUserId": "6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2",
    "createdAt": "2026-06-30T18:00:00Z",
    "updatedAt": "2026-06-30T18:00:00Z"
  }
}
```

Errors:

- `401 Unauthorized`
- `403 Forbidden`: missing or invalid CSRF token.
- `404 Not Found`: group does not exist, is dissolved, or is hidden.
- `422 Unprocessable Entity`: validation failed.

### GET /api/groups/{groupId}/repayments/{repaymentId}

Function: Returns a non-deleted repayment.

Authentication: active group member.

Path parameters:

- `groupId`: UUID.
- `repaymentId`: UUID.

Response `200 OK`:

```json
{
  "repayment": {
    "id": "21feeb5e-ff5c-44ec-b0bc-8daae2d0522f",
    "groupId": "9a41c3a3-169f-4219-998b-822d31352f90",
    "fromUserId": "6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2",
    "toUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
    "amountCents": 2000,
    "currency": "USD",
    "note": "Venmo",
    "repaymentDate": "2026-06-30",
    "createdByUserId": "6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2",
    "createdAt": "2026-06-30T18:00:00Z",
    "updatedAt": "2026-06-30T18:00:00Z"
  }
}
```

Errors:

- `401 Unauthorized`
- `404 Not Found`: group or repayment does not exist, is deleted/dissolved, or
  is hidden.

### PUT /api/groups/{groupId}/repayments/{repaymentId}

Function: Replaces a repayment. Any active group member can replace any
repayment in the group.

Authentication: active group member.

Path parameters:

- `groupId`: UUID.
- `repaymentId`: UUID.

Required headers:

```http
Content-Type: application/json
X-CSRF-Token: <csrf_token>
```

Request body:

```json
{
  "fromUserId": "6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2",
  "toUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
  "amountCents": 2500,
  "note": "Venmo corrected",
  "repaymentDate": "2026-06-30"
}
```

This is a full replacement; all editable fields are required.

Response `200 OK`:

```json
{
  "repayment": {
    "id": "21feeb5e-ff5c-44ec-b0bc-8daae2d0522f",
    "groupId": "9a41c3a3-169f-4219-998b-822d31352f90",
    "fromUserId": "6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2",
    "toUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
    "amountCents": 2500,
    "currency": "USD",
    "note": "Venmo corrected",
    "repaymentDate": "2026-06-30",
    "createdByUserId": "6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2",
    "createdAt": "2026-06-30T18:00:00Z",
    "updatedAt": "2026-06-30T19:00:00Z"
  }
}
```

Errors:

- `401 Unauthorized`
- `403 Forbidden`: missing or invalid CSRF token.
- `404 Not Found`: group or repayment does not exist, is deleted/dissolved, or
  is hidden.
- `422 Unprocessable Entity`: validation failed.

### DELETE /api/groups/{groupId}/repayments/{repaymentId}

Function: Soft-deletes a repayment. Deleted repayments are hidden from all
ordinary first-release APIs and excluded from settlement results.

Authentication: active group member.

Path parameters:

- `groupId`: UUID.
- `repaymentId`: UUID.

Required headers:

```http
X-CSRF-Token: <csrf_token>
```

Request body: none.

Response `204 No Content`

Errors:

- `401 Unauthorized`
- `403 Forbidden`: missing or invalid CSRF token.
- `404 Not Found`: group or repayment does not exist, is deleted/dissolved, or
  is hidden.

## Settlement Endpoints

### GET /api/groups/{groupId}/settlements

Function: Returns current pairwise settlement results for a visible active group.

Authentication: active group member.

Path parameters:

- `groupId`: UUID.

Response `200 OK`:

```json
{
  "settlements": [
    {
      "fromUserId": "6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2",
      "toUserId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
      "amountCents": 2000,
      "currency": "USD"
    }
  ],
  "members": [
    {
      "userId": "0a4b8d9a-4f65-4a88-8e58-9467df994a61",
      "displayName": "Alice"
    },
    {
      "userId": "6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2",
      "displayName": "Bob"
    }
  ]
}
```

Behavior:

- Each settlement means `fromUserId` should pay `toUserId`.
- Zero balances are omitted.
- Results use pairwise netting for the first release.
- Deleted expenses, deleted repayments, and dissolved groups are excluded through
  the active data boundary.
- Removed members cannot access settlement results.
- Because removing a member is rejected while that member participates in any
  current expense or repayment, ordinary settlement results should only
  reference active members.

Errors:

- `401 Unauthorized`
- `404 Not Found`: group does not exist, is dissolved, or is hidden.

## Authorization Matrix

| Workflow | Unauthenticated | Active member | Owner |
| --- | --- | --- | --- |
| Register/login/CSRF | allowed | allowed | allowed |
| List own groups | no | yes | yes |
| Create group | no | yes | yes |
| Join group by code | no | yes | yes |
| View group details | no | own groups | own groups |
| Rename group | no | no | yes |
| View join code | no | no | yes |
| Remove members | no | no | yes |
| Dissolve group | no | no | yes |
| List/create/edit/delete expenses | no | yes | yes |
| List/create/edit/delete repayments | no | yes | yes |
| View settlements | no | yes | yes |

All owner actions also require the owner to be an active member of a
non-dissolved group.

## Hidden State Rules

For ordinary first-release APIs:

- Dissolved groups behave as not found.
- Removed members cannot access the group or its history.
- Deleted expenses behave as not found and are excluded from lists and
  settlements.
- Deleted repayments behave as not found and are excluded from lists and
  settlements.
- Removed members should not appear in active group member lists.
- Removing a member is rejected while the member participates in any current
  non-deleted expense or repayment.
- Because removable members have no current accounting activity, removing a
  member does not require changing existing expenses, repayments, or settlement
  results.

Future archive, audit, recovery, or admin APIs require a separate design.

## CORS And Cookie Requirements

For credentialed browser requests:

- `Access-Control-Allow-Origin` must echo a configured frontend origin, never
  `*`.
- `Access-Control-Allow-Credentials` must be `true`.
- Production session cookies must be `HttpOnly`, `Secure`, and `SameSite=None`.
- Local development may use environment-specific cookie settings, but production
  startup should fail fast if secure settings are missing.
- Unsafe requests must pass both CSRF validation and origin checks.

## Open Implementation Notes

- Password strength and password hashing details belong to auth implementation,
  not this API contract.
- Cookie expiration and refresh behavior should be explicit in implementation
  configuration. The frontend should continue to treat `GET /api/me` as the
  source of truth.
- Expense and repayment lists intentionally return all current records for the
  first release. If activity grows, pagination can be introduced later with a
  focused API revision.
- The API intentionally does not expose join code rotation, archive views,
  owner transfer, OAuth, password reset, payment processing, or global
  minimum-transfer settlement optimization.
