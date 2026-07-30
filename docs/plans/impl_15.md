# Mini Task 15: Static SvelteKit Shell, Design Tokens, and Test Baseline

## Goal

Establish the client-only static application, controlled route skeleton, visual foundation, and a working frontend test command.

## Prerequisites

Mini Task 1. This task may run alongside backend business work, but integration depends on Mini Task 14.

## Implementation Scope

- Use `adapter-static`, an SPA fallback, and root-layout `ssr=false`/`prerender=true`.
- Validate `PUBLIC_API_BASE_URL`: use the local API origin by default and require HTTPS with no trailing slash in production.
- Create the route groups, root/error/auth/app shells, and title strategy from `05_frontend.md`, initially using route-state placeholders.
- Update semantic tokens, typography, spacing, focus, and reduced-motion behavior in `layout.css`; do not create a parallel design system.
- Create `copy/en.ts` and keep user-visible product copy in English.
- Add Vitest, Testing Library, and jsdom only where required, define `pnpm test`, and install only the shadcn-svelte components needed by this task.

## Exclusions

- Do not implement the API client, authentication, business pages, PWA, or a marketing page.

## Validation

- Run `pnpm check`, `pnpm test`, and `pnpm build`.
- Directly visit dynamic-route fallbacks and inspect 320 px, desktop, keyboard-focus, and reduced-motion behavior.

## Completion Criteria

- Static output has no SSR dependency, and route groups do not alter public URLs.
- The test command exists and covers runtime configuration, root-redirect helpers, and baseline accessibility.
