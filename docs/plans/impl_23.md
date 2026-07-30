# Mini Task 23: Repayments, Settlement Shortcut, and Complete Activity

## Goal

Complete the first-release loop: understand who should pay whom, record an off-app payment, and see authoritative balances update.

## Prerequisites

Mini Tasks 12, 13, 19, and 22.

## Implementation Scope

- Complete the directional settlement row and its `Record payment` shortcut.
- Implement repayment new/edit routes and a shared form for sender, recipient, amount, date, and optional note.
- Accept query prefill only after validating active group members, distinct users, and a positive amount.
- State clearly that Settled records a payment made elsewhere and does not send money; allow an active member to record for another member.
- Refresh repayments and settlements after create/edit/delete and replace history with balances/activity as documented.
- Merge repayment rows into Activity with direction, optional note, `Recorded outside Settled`, and edit action.
- Preserve partial-panel errors, loading/empty states, and the offline mutation-guard interface.

## Exclusions

- Do not integrate payment processing, claim sent/completed/verified status, or calculate authoritative balances in the client.

## Validation

- Run `pnpm check`, `pnpm test`, and `pnpm build`.
- Test same sender/recipient, tampered query values, pending state, field errors, settlement prefill, mutation refresh, Activity ordering, and partial failure.
- Manually create a repayment from a settlement row and confirm direction, amount, and updated balance.

## Completion Criteria

- Repayment effects match backend settlement, and the interface always describes them accurately as off-app payment records.
