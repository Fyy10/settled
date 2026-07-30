# Mini Task 11: Expense Store and Complete API

## Goal

Deliver expense creation, reads, full replacement, and soft deletion while protecting split invariants transactionally.

## Prerequisites

Mini Tasks 3, 6, 8, and 10.

## Implementation Scope

- Implement the expense service, narrow Store interface, and list `GET`, create `POST`, item `GET`, item `PUT`, and item `DELETE`.
- Calculate exact splits before the transaction, then lock in group → sorted memberships → expense → splits order.
- Require active actor, payer, and participants, and a non-dissolved group.
- Require every editable field for `PUT`; replace split rows in one transaction while preserving ID, group, creator, and creation time.
- Set `deleted_at` on `DELETE`; repeated deletion and a child ID from another group return `404`.
- Bulk-load splits and member summaries for lists to avoid N+1 queries, and preserve API ordering.

## Exclusions

- Do not implement `PATCH`, multiple currencies, attachments, expense ownership restrictions, or settlements.

## Validation

- Run `gofmt`, `go vet ./...`, `go test ./...`, `go test -race ./...`, and the integration script.
- Cover all three split request modes, rollback, inactive/concurrently removed members, collaborative editing by any active member, strict replacement, soft deletion, cross-group hiding, assembly, and sorting.
- Manually exercise all five endpoints and confirm responses contain only exact splits.

## Completion Criteria

- No committed expense can have splits whose sum differs from its total.
- Deleted expenses appear in neither ordinary reads nor active settlement views.
