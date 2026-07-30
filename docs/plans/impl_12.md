# Mini Task 12: Repayment Store and Complete API

## Goal

Deliver repayment CRUD that records only off-app payments and follows the same hidden-state and lock-order rules as expenses.

## Prerequisites

Mini Tasks 3, 6, and 8.

## Implementation Scope

- Implement the repayment service, narrow Store interface, list, create, item read, `PUT` full replacement, and soft deletion.
- Lock the active group, then actor/from/to memberships in sorted ID order; all must be active, and sender and recipient must differ.
- Require a positive amount and strict date; normalize a blank note to SQL `NULL` and limit non-empty notes to 240 code points.
- Derive `createdByUserId` only from the actor, who may differ from `fromUserId`; replacement preserves immutable metadata.
- Load member summaries once for lists, preserve stable ordering, and hide cross-group, deleted, dissolved, and unauthorized resources with `404`.

## Exclusions

- Do not implement `PATCH`, payment processing, payment verification, approval workflows, or settlement calculation.

## Validation

- Run `gofmt`, `go vet ./...`, `go test ./...`, `go test -race ./...`, and the integration script.
- Cover create/read/list/replace/delete, same-user rejection, nullable notes, inactive members, concurrent removal, recording for another member, soft deletion, and strict responses.
- Manually exercise all five endpoints and ensure no copy implies that Settled sent or verified a payment.

## Completion Criteria

- Any active member can collaboratively maintain group repayments, and invalid participants or amounts never partially commit.
