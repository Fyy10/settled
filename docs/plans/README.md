# Settled First-Release Implementation Task Index

This directory breaks `docs/00_project.md` through `docs/05_frontend.md` into increments that can be implemented and verified independently. If a task summary conflicts with those design documents, the design documents take precedence:

- `03_API.md` is authoritative for the public HTTP contract.
- `04_backend.md` is authoritative for backend implementation and security behavior.
- `05_frontend.md` is authoritative for frontend routing, experience, and validation.

## Execution Rules

- Follow declared dependencies rather than inferring order from filenames. Every task must leave existing checks passing.
- Add the tests required by each task as part of that task; do not defer testing to the final release task.
- Backend changes run at least `gofmt`, `go vet ./...`, and `go test ./...`. Run the race detector for concurrency/security boundaries and the integration script for PostgreSQL behavior.
- Frontend changes run `pnpm check` and `pnpm test`. Also run `pnpm build` for routes, adapters, PWA behavior, or production-output changes.
- If Docker, PostgreSQL, browser, or system-tool prerequisites prevent a required check, record the command, exact missing prerequisite, and remaining risk.
- Do not introduce frameworks, ORMs, state libraries, services, or deployment models that `docs/00-05` does not approve.

## Task List

| # | Task | Primary Verifiable Deliverable |
| --- | --- | --- |
| 1 | [Project baseline and local guide](impl_1.md) | Accurate README and baseline commands |
| 2 | [Backend process, configuration, and health](impl_2.md) | Runnable API with live/ready checks |
| 3 | [Schema, Store, and integration harness](impl_3.md) | Complete fresh-database schema and ephemeral PostgreSQL tests |
| 4 | [Backend boundary rules](impl_4.md) | Pure UUID, time, input, and money logic |
| 5 | [Passwords and session JWTs](impl_5.md) | Argon2id, HS256, and HttpOnly cookies |
| 6 | [CSRF, CORS, and HTTP security](impl_6.md) | Browser security, JSON/errors, and middleware |
| 7 | [Authentication API](impl_7.md) | csrf/register/login/logout/me |
| 8 | [Core group API](impl_8.md) | create/list/join/detail |
| 9 | [Owner management API](impl_9.md) | rename/code/remove/dissolve |
| 10 | [Expense split engine](impl_10.md) | Three modes converted to exact cents |
| 11 | [Expense API](impl_11.md) | list/create/read/PUT/delete |
| 12 | [Repayment API](impl_12.md) | list/create/read/PUT/delete |
| 13 | [Pairwise settlement](impl_13.md) | Layered SQL verification, Go calculator, and endpoint |
| 14 | [Backend quality gate](impl_14.md) | Full API security, concurrency, and failure-path verification |
| 15 | [Static frontend and test baseline](impl_15.md) | SPA fallback, route shell, and design tokens |
| 16 | [API client and authentication state](impl_16.md) | Credentialed fetch and one-retry CSRF |
| 17 | [Authentication UI](impl_17.md) | register/login/logout/guards |
| 18 | [Group list UI](impl_18.md) | list/create/join |
| 19 | [Group workspace](impl_19.md) | balances/activity/members and partial errors |
| 20 | [Owner settings UI](impl_20.md) | rename/code/remove/dissolve |
| 21 | [Expense draft model](impl_21.md) | Pure money/date/split-preview logic |
| 22 | [Expense UI](impl_22.md) | create/edit/delete/activity/revalidation |
| 23 | [Repayment and settlement UI](impl_23.md) | settlement prefill, repayments, and complete activity |
| 24 | [PWA, containers, and release validation](impl_24.md) | install/offline/static cache/containers/end-to-end |

## Dependency Spine

```text
1 -> 2 -> 3
     2 -> 4 -> 5 -> 6
3 + 4 + 5 + 6 -> 7 -> 8 -> 9
4 -> 10
3 + 6 + 8 + 10 -> 11
3 + 6 + 8 -> 12
3 + 11 + 12 -> 13 -> 14

1 -> 15
7 + 15 -> 16 -> 17 -> 18
8 + 13 + 18 -> 19 -> 20
10 + 15 -> 21
11 + 19 + 21 -> 22
12 + 13 + 19 + 22 -> 23
14 + 15..23 -> 24
```

Parallelizable boundaries only indicate that dependencies permit parallel work. Each task must still be completed and validated as an independent deliverable.

## Conflicts Removed from the Previous Plans

- Authentication uses a backend-issued HttpOnly JWT cookie; no bearer token is returned or stored by the browser.
- Unsafe requests require both an allowed `Origin` and anonymous/session-bound CSRF.
- Login uses UTF-8 Basic Auth; registration uses a JSON body.
- Health checks are `/api/health/live` and `/api/health/ready`, not `/healthz`.
- Expense and repayment updates are full `PUT` replacements, not `PATCH`.
- Expense and repayment deletion is soft deletion; groups are dissolved and memberships are softly removed.
- Dissolved, removed, deleted, and unauthorized resources follow hidden `404` rules.
- The settlement endpoint loads normalized debt entries and calls the pure Go calculator; the SQL net view is diagnostic and used for layered verification only.
