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
- OpenSSL or another secure generator for the two signing secrets.
- Docker, `psql`, and Go for the PostgreSQL integration suite.

## Install Frontend Dependencies

From the repository root:

```sh
pnpm install
```

## Frontend Development

The frontend is a client-only SvelteKit application built with
`adapter-static`. These commands exist in `package.json`:

```sh
pnpm dev
pnpm check
pnpm test
pnpm build
pnpm preview
```

- `pnpm dev` starts the Vite development server.
- `pnpm check` runs SvelteKit synchronization and Svelte type checking.
- `pnpm test` runs the Vitest unit and component suite once.
- `pnpm build` creates static production output in `build/`.
- `pnpm preview` serves that output locally after `pnpm build`.

Production builds require an HTTPS API origin:

```sh
PUBLIC_API_BASE_URL=https://api.settled.example pnpm build
```

Development defaults to `http://localhost:8080`. Static hosting must rewrite
unknown application routes to `200.html` so direct visits to dynamic routes
load the client application.

## API Development

The Go API entry point is `server/cmd/settled`. It serves the first-release
authentication, group, expense, repayment, and pairwise-settlement workflows,
plus liveness and PostgreSQL-backed readiness checks.

For a first local run, create an empty database, generate two independent
secrets, apply the schema, and start the API. Adjust the database URL for the
credentials used by your local PostgreSQL installation:

```sh
cd server
createdb settled
export DATABASE_URL='postgres://localhost/settled?sslmode=disable'
export JWT_SECRET_BASE64="$(openssl rand -base64 32)"
export CSRF_SECRET_BASE64="$(openssl rand -base64 32)"
psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 -f sql/schema.sql
go run ./cmd/settled
```

Development defaults allow the frontend origins `http://localhost:5173` and
`http://127.0.0.1:5173`, listen on `:8080`, and use insecure `SameSite=Lax`
cookies. The three required variables above must still be set in development.
Run backend checks from `server/`:

```sh
go test ./...
go test -race ./...
go vet ./...
```

Once running, check:

```sh
curl http://localhost:8080/api/health/live
curl http://localhost:8080/api/health/ready
```

Both return `{"status":"ok"}` while the API and database are available.
PostgreSQL must be reachable during the five-second startup check. If it becomes
temporarily unavailable after startup:

- Liveness remains `200 OK` because the process is still running.
- Readiness returns `503 Service Unavailable`.
- Affected database-backed requests return the common JSON
  `500 internal_error` response without exposing database details.
- The same process and connection pool resume serving requests after PostgreSQL
  becomes available; a restart is not required.

`SIGINT` and `SIGTERM` initiate graceful HTTP shutdown with a ten-second
deadline.

## Database Development

PostgreSQL persists users, groups, memberships, expenses, splits, and
repayments. It also provides the active-ledger views used to verify
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

The script requires the Docker daemon, Go, and `psql` on the host. Homebrew
installs `libpq` as keg-only, so add its binary directory to `PATH` when needed:

```sh
brew install libpq
export PATH="$(brew --prefix libpq)/bin:$PATH"
./scripts/test-integration.sh
```

The script starts a uniquely named PostgreSQL 17 container without a persistent
volume, applies the schema through the host `psql` client, runs all
`integration`-tagged Go tests, and removes only that exact container when it
finishes or is interrupted. A `psql` binary inside the container does not
replace the host prerequisite. The harness does not provide persistent local
PostgreSQL or deployment container configuration.

## Configuration

| Variable | Required | Default | Rules |
| --- | --- | --- | --- |
| `APP_ENV` | no | `development` | `development`, `test`, or `production` |
| `HTTP_ADDR` | no | `:8080` | Must not be blank |
| `DATABASE_URL` | yes | none | PostgreSQL connection URL; never log or commit it |
| `ALLOWED_ORIGINS` | production | local frontend origins in development | Comma-separated exact HTTP(S) origins; production requires HTTPS |
| `JWT_SECRET_BASE64` | yes | none | Standard Base64 decoding to at least 32 bytes |
| `CSRF_SECRET_BASE64` | yes | none | At least 32 decoded bytes and different from the JWT secret |
| `COOKIE_SECURE` | no | `false` outside production; `true` in production | Must be `true` in production and with `SameSite=None` |
| `COOKIE_SAME_SITE` | no | `lax` outside production; `none` in production | `lax`, `strict`, or `none`; production requires `none` |
| `COOKIE_DOMAIN` | no | unset | Leave unset for host-only cookies unless deployment requires a domain |
| `DB_MAX_OPEN_CONNS` | no | `10` | Positive integer |
| `DB_MAX_IDLE_CONNS` | no | `10` | Between zero and max open connections |
| `DB_CONN_MAX_LIFETIME` | no | `30m` | Positive Go duration |
| `DB_CONN_MAX_IDLE_TIME` | no | `5m` | Positive Go duration |
| `LOG_LEVEL` | no | `info` | `debug`, `info`, `warn`, or `error` |

The frontend reads `PUBLIC_API_BASE_URL`. It defaults to
`http://localhost:8080` during local development; production builds require an
absolute HTTPS origin without a trailing slash.

Keep secrets out of committed files. The repository ignores `.env` and `.env.*`
files except explicit example and test templates.

### Production Configuration

Set `APP_ENV=production`, inject distinct signing secrets rather than storing
them in the image, and configure at least one exact HTTPS frontend origin in
`ALLOWED_ORIGINS`. Production startup fails before listening when required
values are absent or malformed, origins are insecure, cookies are not
`Secure`/`SameSite=None`, or pool settings are invalid.

The Go process serves HTTP rather than configuring TLS certificates. A
production deployment must provide HTTPS at its edge, set
`PUBLIC_API_BASE_URL` to the public HTTPS API origin, and use a TLS-enabled
PostgreSQL URL appropriate for that environment. Use `/api/health/live` for
process liveness and `/api/health/ready` for traffic readiness.

## Current Status

| Component | Status |
| --- | --- |
| Frontend static shell | Client-only route skeleton, semantic tokens, and SPA fallback present |
| Frontend tests | Vitest and Testing Library baseline present |
| Go API | First-release JSON API, browser security, health checks, and graceful lifecycle implemented |
| PostgreSQL schema and scripts | Fresh schema, Store implementation, and ephemeral integration harness present |
| Containers and deployment | Not implemented |

The design documents in `docs/` define the intended first release. The
dependency-ordered implementation plan is in `docs/plans/README.md`.
