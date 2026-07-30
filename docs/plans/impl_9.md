# Mini Task 9: Group Owner Management API

## Goal

Complete owner-only group management while protecting consistency between member removal and the current ledger.

## Prerequisites

Mini Task 8.

## Implementation Scope

- Implement `PATCH /api/groups/{groupId}`, `DELETE /api/groups/{groupId}`, `GET /join-code`, and `DELETE /members/{userId}`.
- Lock the active group and verify both `owner_user_id` and the owner’s active membership for every workflow.
- Rename updates timestamps; dissolution is soft state, and a repeated operation returns hidden `404`.
- Lock membership rows in user-ID order during member removal and prohibit removal of the owner.
- Return `409 conflict` if the target is referenced by a current expense payer/split or repayment sender/recipient; creator-only references do not block removal.
- Set `removed_at` on success and retain historical rows.

## Exclusions

- Do not implement ownership transfer, join-code rotation, archive/recovery, or read-only dissolved groups.

## Validation

- Run `gofmt`, `go vet ./...`, `go test ./...`, `go test -race ./...`, and the integration script.
- Cover owner/member permissions, owner self-removal, in-use-member conflict, unused-member removal, dissolved hidden state, repeated operations, and concurrent state changes.
- Manually call all four workflows as both an owner and a member.

## Completion Criteria

- Members cannot read the join code or perform owner mutations.
- A successfully removed user immediately loses access to every ordinary group API.
