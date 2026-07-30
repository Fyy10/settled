# Mini Task 24: PWA, Containers, and First-Release Validation

## Goal

Complete an installable PWA, separate frontend/backend container artifacts, and end-to-end release gates without changing the approved architecture.

## Prerequisites

Mini Tasks 14 through 23.

## Implementation Scope

- Use `vite-plugin-pwa` to add a manifest, 192/512 regular and maskable icons, a static-asset service worker, and controlled update prompts.
- Precache only the app shell and versioned static assets; all API and credentialed cross-origin requests must be network-only.
- Add a global offline banner, read-only access to already-rendered data, disabled mutations, and in-memory draft retention; never queue or replay mutations.
- Complete responsive, safe-area, keyboard, screen-reader, contrast, reduced-motion, 320 px, and 200% zoom checks.
- Add the minimum backend container, static-frontend container/hosting configuration, and deployment instructions that apply the schema before API startup; keep frontend and backend as separate runtimes.
- Configure static fallbacks and appropriate CSP/security headers, with `connect-src` limited to the configured API origin.

## Exclusions

- Do not add Tauri/Capacitor, SSR, hosted SaaS, offline data caching/sync, analytics, or error-reporting services.
- Do not automatically migrate the database during application startup.

## Validation

- Backend: run `gofmt`, `go vet ./...`, `go test ./...`, `go test -race ./...`, and the integration script.
- Frontend: run `pnpm check`, `pnpm test`, and `pnpm build`.
- Build and start the containers, apply the schema to fresh PostgreSQL, and complete the 13 manual workflow checks in `05_frontend.md`.
- Verify PWA install/update/offline reload, direct dynamic routes, and the absence of `/api` data from service-worker caches.

## Completion Criteria

- Two users can complete registration, group create/join, all expense modes, repayment, settlement, and owner management end to end.
- Every automated command passes; any environment-dependent check not run has its exact reason and risk documented.
