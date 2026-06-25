# AGENTS.md

This file defines how AI agents should work in this repository. Treat it as the project contract: keep the stack consistent, keep the architecture lightweight, and preserve a simple, elegant product experience.

## Project Direction

Settled is a simple app for settling shared bills with friends. The project is early, so agents should optimize for clear foundations over broad abstractions.

Core principles:

- Keep the system lightweight, understandable, and easy to change.
- Prefer the existing project patterns before introducing new structure.
- Preserve a consistent technical stack, design language, and architecture.
- Ask for explicit user approval before introducing any new major technology, framework, service, runtime, database, build system, or deployment model.

## Approved Stack

Use this stack by default:

- Backend: Go with the standard `net/http` package.
- Frontend: SvelteKit.
- UI: shadcn-svelte components and project conventions from `components.json`.
- Styling: Tailwind CSS and the existing design tokens in `src/routes/layout.css`.
- App model: PWA-first web app.
- Database: PostgreSQL.
- Release: containerized deployment.

Future possibilities, not current defaults:

- Tauri for a desktop app.
- Capacitor for a mobile app.

Do not add Tauri, Capacitor, a mobile shell, a desktop shell, or related dependencies unless the user explicitly approves that work.

## Architecture Rules

- Keep both frontend and backend lightweight and direct.
- Do not introduce large or heavy frameworks when the approved stack can solve the problem cleanly.
- Avoid premature layers, generic repositories, complex dependency injection systems, global state frameworks, or broad plugin architectures.
- Keep the directory structure controlled and easy to scan.
- Put business logic in clear, testable modules instead of scattering it through UI components or HTTP handlers.
- Keep API contracts stable and explicit. Coordinate frontend and backend shape changes together.
- Prefer standard library Go where practical. Add Go dependencies only when they clearly reduce risk or complexity.
- Prefer SvelteKit primitives and small local helpers before adding frontend libraries.
- Keep shared utilities small, named by what they do, and located near the code that uses them unless reuse is real.

## Frontend And Design

The product should feel simple, elegant, useful, and modern. It should prioritize user experience over decoration.

- Build actual app screens and workflows, not marketing pages, unless specifically requested.
- Use shadcn-svelte components before creating custom UI primitives.
- Keep visual hierarchy clear: restrained spacing, readable typography, strong alignment, and predictable interaction states.
- Keep component behavior accessible and keyboard-friendly.
- Use icons from the existing icon library when appropriate, especially for common actions.
- Avoid flashy, ornamental, or template-like layouts that do not serve the workflow.
- Do not create one-off visual styles when an existing token, component variant, or pattern fits.
- Make responsive layouts deliberate. Check that text, controls, and panels do not overlap or overflow on small screens.

## Backend Rules

- Build HTTP services around Go `net/http`.
- Keep handlers thin when practical: parse input, call focused logic, return clear responses.
- Use PostgreSQL as the persistent database.
- Keep database access explicit and testable.
- Do not introduce an ORM, web framework, GraphQL layer, RPC framework, job system, or message broker without explicit user approval.
- Keep errors useful for developers without leaking sensitive details to clients.

## Data And API Rules

- Design APIs around clear user workflows, not internal implementation details.
- Use consistent JSON shapes and status codes.
- Validate inputs at the boundary.
- Keep migrations and schema changes understandable and reviewable once database tooling exists.
- Do not add external hosted services unless the user explicitly approves them.

## Quality Gates

After every code change, agents must:

- Format all changed code with the appropriate project formatter.
- Run syntax, type, or build checks relevant to the changed area.
- Add or update relevant tests for changed behavior.
- Run the relevant tests.
- Report what was run and whether it passed.
- If a required check cannot be run, explain exactly why.

Current frontend checks:

- `pnpm check` for SvelteKit sync and type checking.
- `pnpm build` when changes affect routing, build configuration, PWA behavior, or production output.

Current backend checks:

- `go test ./...` from `server/` once Go code exists.
- `gofmt` on changed Go files.

If no formatter, test runner, or check command exists yet for a changed area, do not silently skip it. State the gap and, when appropriate, propose the smallest project-consistent tool or script to add.

## Dependency Policy

- Before adding a dependency, first check whether the approved stack or standard library already solves the problem.
- Add dependencies only for clear value: correctness, security, accessibility, interoperability, or significant complexity reduction.
- Avoid dependencies that pull in a large framework direction by stealth.
- Explain why any new dependency is necessary.
- Get explicit user approval before adding dependencies that change the architecture or technology stack.

## Agent Workflow

When starting a task:

- Read the relevant files before editing.
- Identify the existing pattern and follow it.
- Keep changes tightly scoped to the request.
- Ask the user before making architectural or stack-level decisions.

When editing:

- Prefer small, coherent changes.
- Do not rewrite unrelated code.
- Do not move files or reshape directories without a clear reason.
- Preserve user changes already present in the working tree.

When finishing:

- Summarize the change briefly.
- List validation commands run.
- Mention any tests or checks that could not be run.
- Call out follow-up work only when it is genuinely useful.

## Explicit Approval Required

Ask the user before:

- Introducing a new framework, ORM, state library, API style, database, queue, cache, hosted service, or deployment platform.
- Adding Tauri or Capacitor.
- Changing the approved stack.
- Reorganizing top-level project structure.
- Making large visual redesigns.
- Adding broad abstractions that future work would have to follow.

If unsure whether a change crosses these lines, ask first.
