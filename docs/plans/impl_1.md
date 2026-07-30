# Mini Task 1: Project Baseline and Local Development Guide

## Goal

Turn the SvelteKit starter baseline into an accurate development entry point for Settled without changing product behavior.

## Prerequisites

None. Use `docs/00_project.md`, the current `package.json`, and `server/go.mod` as the source of truth.

## Implementation Scope

- Rewrite `README.md` to describe Settled’s first-release scope, frontend and backend directories, PostgreSQL prerequisites, and common commands.
- Document how the frontend, API, and database are started and validated separately, including the categories of environment variables later tasks will use.
- Clearly identify commands and components that do not exist yet, including `pnpm test`, the backend executable, and database scripts.
- Preserve the existing dependencies and project structure; do not implement product behavior.

## Exclusions

- Do not add dependencies, container files, a database schema, APIs, or routes.
- Do not promise a deployment platform that is not established by `docs/00-05`.

## Validation

- Run `pnpm check`.
- Run `go test ./...` from `server/`; if no Go packages exist, record the exact result.
- Manually compare every README command with `package.json`, `server/go.mod`, and the actual directory structure.

## Completion Criteria

- A new contributor can distinguish the frontend, API, and database responsibilities and working directories.
- Every documented current command is executable, and future commands are explicitly marked as not yet implemented.
