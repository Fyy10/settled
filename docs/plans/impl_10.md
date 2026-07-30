# Mini Task 10: Pure Go Expense Split Engine

## Goal

Independently convert equal, exact, and percentage inputs into deterministic exact-cent persistence values.

## Prerequisites

Mini Task 4.

## Implementation Scope

- Define discriminated split inputs and ordered, positive exact-split output.
- Equal mode requires explicit participant IDs and assigns remainder cents in request order; reject zero participants or an amount smaller than the participant count.
- Exact mode requires positive values, unique users, overflow-safe summation, and a total exactly equal to the expense amount.
- Percentage mode requires positive basis points totaling 10000, calculates floor shares with `math/big.Int`, and distributes remainder cents in request order.
- Reject zero-cent output, invalid UUIDs, duplicate participants, and unsupported modes.
- Do not preserve original mode or percentages in the persistence model.

## Exclusions

- Do not access HTTP, PostgreSQL, or memberships; the expense transaction validates active memberships.
- Do not use floating-point arithmetic.

## Validation

- Run `gofmt`, `go vet ./...`, `go test ./...`, and `go test -race ./...` from `server/`.
- Use table-driven and boundary tests for remainders, participant order, zero shares, duplicates, invalid totals, maximum `int64`, arithmetic overflow, and cross-mode equivalent results.

## Completion Criteria

- Every successful result contains only positive, unique, stable-order splits whose sum exactly equals the expense total.
