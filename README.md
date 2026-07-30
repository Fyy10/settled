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
├── server/              Go API module; currently contains only go.mod
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

Backend and database work will require:

- Go 1.25.1, matching `server/go.mod`.
- PostgreSQL and its `psql` client.

No PostgreSQL version, container image, or repository-managed database startup
command has been established yet.

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

The Go module exists, but the backend executable and API packages have not been
implemented. There is currently no API process to start.

The planned entry point is `server/cmd/settled`, so the eventual command will be
`go run ./cmd/settled` from `server/`. That path does not exist yet and the
command is not currently runnable.

The baseline Go validation command is:

```sh
cd server
go test ./...
```

Because the module currently contains no Go packages or tests, the Go tool
reports that `./...` matched no packages. Later backend tasks will make this a
passing test suite.

## Database Development

PostgreSQL will persist users, groups, memberships, expenses, splits, and
repayments. It will also provide the active-ledger views used to verify
settlement calculations.

Database support has not been implemented yet:

- There is no schema or migration file.
- There is no database creation or schema-application script.
- There is no integration-test script.
- There is no Docker or Compose configuration.
- The application does not currently connect to PostgreSQL.

Start PostgreSQL independently using your preferred local installation when the
schema and API tasks are implemented. Those tasks will document the exact
database setup and validation commands.

## Planned Configuration

No application code currently consumes project-specific environment variables.
Later tasks will introduce configuration in these categories:

- Frontend API origin: `PUBLIC_API_BASE_URL`.
- Runtime mode and listen address: `APP_ENV` and `HTTP_ADDR`.
- PostgreSQL connection and pool settings, including `DATABASE_URL`.
- Exact browser origins allowed to make credentialed API requests.
- Independent JWT and CSRF signing secrets.
- Session-cookie security settings.
- Logging level.

Keep secrets out of committed files. The repository ignores `.env` and `.env.*`
files except explicit example and test templates.

## Current Status

| Component | Status |
| --- | --- |
| Frontend starter | Present; development, check, build, and preview scripts exist |
| Frontend tests | Not implemented; no `pnpm test` script |
| Go module | Present at `server/go.mod` |
| API executable and routes | Not implemented |
| PostgreSQL schema and scripts | Not implemented |
| Containers and deployment | Not implemented |

The design documents in `docs/` define the intended first release. The
dependency-ordered implementation plan is in `docs/plans/README.md`.
