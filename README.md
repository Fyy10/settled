# Settled

Settled is a lightweight app for trusted groups of friends to track shared
expenses, record repayments made outside the app, and see who owes whom.

The first release is intentionally focused:

- Register and log in with email and password.
- Create a private group or join one with a group code.
- Add collaboratively managed expenses with equal, exact, or percentage splits.
- Record off-app repayments.
- Show pairwise USD balances.
- Let group owners rename or dissolve a group, view its code, and remove unused
  members.

Payment processing, multiple currencies, public group discovery, global
minimum-transfer optimization, and native mobile or desktop shells are outside
the first-release scope.

## Technology

- Frontend: SvelteKit, Svelte, Tailwind CSS, and shadcn-svelte conventions.
- API: Go 1.25.1 using the standard `net/http` package.
- Database: PostgreSQL.
- Application model: PWA-first web app.
- Release direction: containerized deployment; no deployment platform is
  selected yet.

## Repository Layout

```text
.
├── src/                 SvelteKit frontend source
│   ├── lib/             Shared frontend components and utilities
│   └── routes/          Frontend routes and global styles
├── static/              Static frontend assets
├── server/              Go API, PostgreSQL schema, and integration tooling
├── docs/                Product, architecture, API, backend, and frontend designs
├── docs/plans/          Ordered implementation tasks
├── package.json         Frontend scripts and dependencies
└── components.json      shadcn-svelte project configuration
```

The frontend, API, and database are separate runtime responsibilities. Run
frontend commands from the repository root and Go commands from `server/`.
PostgreSQL runs as an external process.

## Prerequisites

Current frontend development requires:

- Node.js and `pnpm`.

Backend and database development requires:

- Go 1.25.1, matching `server/go.mod`.
- PostgreSQL and the `psql` client for local schema application.
- Docker, `psql`, and Go for the PostgreSQL integration suite.

## Install Frontend Dependencies

From the repository root:

```sh
pnpm install
```

## Frontend Development

The current repository contains a runnable SvelteKit starter. These commands
exist in `package.json`:

```sh
pnpm dev
pnpm check
pnpm build
pnpm preview
```

- `pnpm dev` starts the Vite development server.
- `pnpm check` runs SvelteKit synchronization and Svelte type checking.
- `pnpm build` creates the current production output.
- `pnpm preview` serves that output locally after `pnpm build`.

`pnpm test` is not available yet. A frontend test runner and the `test` script
will be added with the first behavior tests.

## API Development

The Go API entry point is `server/cmd/settled`. It currently exposes liveness
and PostgreSQL-backed readiness checks:

```sh
cd server
go run ./cmd/settled
go test ./...
```

The process requires `DATABASE_URL`, `JWT_SECRET_BASE64`, and
`CSRF_SECRET_BASE64`. Both secrets must be distinct Base64-encoded values of at
least 32 decoded bytes. PostgreSQL must be reachable before the API starts.

Once running, check:

```sh
curl http://localhost:8080/api/health/live
curl http://localhost:8080/api/health/ready
```

Both return `{"status":"ok"}` while the API and database are available.

## Database Development

PostgreSQL will persist users, groups, memberships, expenses, splits, and
repayments. It will also provide the active-ledger views used to verify
settlement calculations.

Apply the first-release schema to an empty PostgreSQL database from the
repository root:

```sh
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f server/sql/schema.sql
```

`schema.sql` is a fresh-database schema wrapped in one transaction. It is not
an idempotent migration, and the API does not apply it automatically.

Run the schema and Store integration suite from `server/`:

```sh
cd server
./scripts/test-integration.sh
```

The script starts a uniquely named PostgreSQL 17 container without a persistent
volume, applies the schema through the host `psql` client, runs the
`integration`-tagged Go tests, and removes only that exact container when it
finishes or is interrupted. It does not provide persistent local PostgreSQL or
deployment container configuration.

## Configuration

The API currently reads:

- Runtime mode and listen address: `APP_ENV` and `HTTP_ADDR`.
- PostgreSQL connection and pool settings: `DATABASE_URL`,
  `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME`, and
  `DB_CONN_MAX_IDLE_TIME`.
- Browser security: `ALLOWED_ORIGINS`, `COOKIE_SECURE`, `COOKIE_SAME_SITE`, and
  `COOKIE_DOMAIN`.
- Independent signing secrets: `JWT_SECRET_BASE64` and `CSRF_SECRET_BASE64`.
- Structured logging threshold: `LOG_LEVEL`.

The frontend API origin variable, `PUBLIC_API_BASE_URL`, will be introduced with
the static application shell.

Keep secrets out of committed files. The repository ignores `.env` and `.env.*`
files except explicit example and test templates.

## Current Status

| Component | Status |
| --- | --- |
| Frontend starter | Present; development, check, build, and preview scripts exist |
| Frontend tests | Not implemented; no `pnpm test` script |
| Go API | Runnable health service at `server/cmd/settled` with unit and race tests |
| PostgreSQL schema and scripts | Fresh schema and ephemeral integration harness present |
| Containers and deployment | Not implemented |

The design documents in `docs/` define the intended first release. The
dependency-ordered implementation plan is in `docs/plans/README.md`.
