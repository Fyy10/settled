# Mini Task 16: Frontend API Client, CSRF, and Authentication State

## Goal

Create a typed, credentialed, CSRF-safe HTTP layer independently of business pages.

## Prerequisites

Mini Tasks 7 and 15.

## Implementation Scope

- Mirror `03_API.md` DTOs exactly in `api/types.ts` and add API contract fixtures.
- Implement shared fetch behavior for API base URL, `Accept`, `credentials: "include"`, JSON/`204`, AbortSignal, and primitive response validation.
- Implement `ApiError` and field mapping while distinguishing `401`, CSRF, network, and unknown errors.
- Keep CSRF only in memory and implement request coalescing, successful-auth rotation, logout clearing, and exactly one refresh/retry for `csrf_required` or `csrf_invalid`.
- Implement Unicode-safe Basic Auth without persisting Authorization or password data.
- Implement current-user auth state, `GET /api/me`, safe `next` validation, and session-expiration behavior.

## Exclusions

- Do not implement login/register UI, a global business-data cache, localStorage, or sessionStorage.

## Validation

- Run `pnpm check` and `pnpm test`.
- Unit-test Unicode Basic Auth, every error shape, `204`, CSRF coalescing/rotation/one-retry, safe next paths, `401` versus network failure, and fixture camelCase/nullability.

## Completion Criteria

- Every API request explicitly includes credentials, and unsafe requests can only use the shared CSRF path.
- Sessions, CSRF tokens, API data, and passwords never enter persistent browser storage.
