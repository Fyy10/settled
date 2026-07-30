# Mini Task 18: Group List, Create, and Join UI

## Goal

Let an authenticated user create a group or join one by code from a useful empty state.

## Prerequisites

Mini Tasks 8 and 17.

## Implementation Scope

- Implement `/groups` loading, empty, error, retry, and group-row/card states.
- Implement the create dialog with name validation, pending state, and successful refresh/navigation/toast.
- Implement the join dialog with a paste-friendly normal input, trim/uppercase behavior, privacy-preserving `404` copy, and already-active success behavior.
- Display only API-provided member count, owner badge, and update time; do not invent balance summaries.
- Add the currently needed shadcn-svelte Dialog, Field, Input, Card, Empty, Alert, Skeleton, and Toast components.
- Correctly manage initial focus, Escape/Cancel, and focus return after dialog close.

## Exclusions

- Do not implement the group workspace, owner settings, expenses, or a global groups cache.

## Validation

- Run `pnpm check`, `pnpm test`, and `pnpm build`.
- Component-test empty/error/loading states, create/join field errors, `404`, pending/double-submit prevention, and focus return.
- Manually create and join by pasted code against the backend and verify two-user list isolation.

## Completion Criteria

- Successful creation or joining navigates directly to the returned group URL; failures preserve user input in memory.
