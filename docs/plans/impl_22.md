# Mini Task 22: Expense Create, Edit, Delete, and Activity UI

## Goal

Deliver the complete mobile-first expense workflow and correctly refresh authoritative balances.

## Prerequisites

Mini Tasks 11, 19, and 21.

## Implementation Scope

- Implement the new-expense and `[expenseId]/edit` routes using shared ExpenseForm and SplitEditor components.
- Follow the field order, default payer/date/equal/all-members behavior, live review, and sticky mobile footer in `05_frontend.md`.
- Confirm mode/participant changes that discard manual values; use dirty internal-navigation and `beforeunload` protection without persisting drafts.
- Use `POST` for create and full-replacement `PUT` for edit; map all backend field errors and unknown fields.
- Add edit-only deletion through AlertDialog and the soft-delete API.
- Wait for server confirmation; on success, explicitly refresh expenses and settlements and replace history with activity/balances. Do not use optimistic accounting.
- Render semantic Activity expense rows with edit actions.

## Exclusions

- Do not implement `PATCH`, attachments, recurring expenses, draft storage, or authoritative settlement calculation.

## Validation

- Run `pnpm check`, `pnpm test`, and `pnpm build`.
- Component-test all split modes, remainders, mode/participant confirmation, first-invalid-field focus, dirty navigation, `422` mapping, deletion, and revalidation.
- Manually complete an ordinary equal split within one minute at 320 px, then edit/delete exact and percentage expenses.

## Completion Criteria

- After save, Activity and Balances show current backend results; failure preserves the draft and never reports success.
