# Settled

Settled is a simple app to settle up group bills with your friends.

## Stack

- Frontend: SvelteKit
- UI: shadcn-svelte
- Backend: Go
- Database: PostgreSQL

## Repository Layout

- `./`: SvelteKit frontend app and shared project configuration
- `server/`: Go backend module
- `docs/`: project dev docs

## Prerequisites

- `pnpm`
- Go
- PostgreSQL

## Local Configuration

Database tooling has not been added yet, but local development should use PostgreSQL. Future backend work should expect database connection settings to come from environment variables.

Common local variables:

```sh
DATABASE_URL=postgres://settled:settled@localhost:5432/settled?sslmode=disable
```

Keep secrets out of committed files. Use a local shell profile, ignored `.env` file, or process manager configuration when those settings become active.

## Frontend Development

Run these from the repository root.

```sh
pnpm dev
pnpm check
pnpm build
pnpm preview
```

- `pnpm dev` starts the Vite development server.
- `pnpm check` runs SvelteKit sync and Svelte type checking.
- `pnpm build` creates a production build.
- `pnpm preview` serves the production build locally after building.

## Backend Development

Run backend commands from `server/`.

```sh
cd server
go test ./...
go run ./cmd/server
```

The backend currently contains the Go module baseline. Until Go packages are added, `go test ./...` may report that there are no packages to test.
