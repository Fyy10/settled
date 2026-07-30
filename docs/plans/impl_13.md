# Mini Task 13: Pairwise Settlement Engine and Endpoint

## Goal

Return authoritative pairwise balances through a replaceable pure-Go boundary and verify them at each SQL-view layer.

## Prerequisites

Mini Tasks 3, 11, and 12.

## Implementation Scope

- Define `Calculator.Calculate(entries []DebtEntry)` and `Transfer`, then implement the pairwise calculator.
- Validate IDs, self-debt, positive amounts, USD, and overflow; aggregate directed debt, offset unordered pairs, and omit zero balances.
- Sort by amount descending, then from-user ID and to-user ID.
- Load visible-group entries from `settlement_debt_entries` along with member summaries.
- Implement `GET /api/groups/{groupId}/settlements` with the stable DTO from `03_API.md`.
- The production endpoint must invoke the Go calculator rather than selecting final results from `pairwise_net_balances`.

## Exclusions

- Do not implement global minimum-transfer optimization, precomputed/materialized settlement, pagination, or multiple currencies.

## Validation

- Run `gofmt`, `go vet ./...`, `go test ./...`, `go test -race ./...`, and the integration script.
- Unit-test bidirectional netting, repayment reversal, zero omission, invalid entries, overflow, and sorting.
- Use the Alice/Bob/Carol fixture from `04_backend.md` to compare debt entries → gross view → net view → Go calculator → HTTP endpoint, including deletion, dissolution, and cross-group isolation.

## Completion Criteria

- SQL diagnostic views, the Go calculator, and the endpoint agree exactly on direction, amount, and ordering for all representative fixtures.
