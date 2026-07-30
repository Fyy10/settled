# Mini Task 8: Group Creation, Listing, Joining, and Detail API

## Goal

Deliver the core group read/write loop available to ordinary active members.

## Prerequisites

Mini Tasks 3, 4, 6, and 7.

## Implementation Scope

- Implement `GET /api/groups`, `POST /api/groups`, `POST /api/groups/join`, and `GET /api/groups/{groupId}`.
- Create the group and owner membership in one transaction; generate 12-character join codes from the documented alphabet using uniform sampling and retry collisions at most five times.
- Normalize join codes with trim + uppercase and lock the group/membership; first join, repeated join, and removed-membership reactivation all return `200`.
- Return group summary and active members ordered by owner, display name, then ID.
- Hide dissolved groups, removed memberships, and non-member resources; use the documented stable list ordering.
- Do not include the join code in the group creation response.

## Exclusions

- Do not implement rename, join-code access, member removal, dissolution, expenses, or frontend work.

## Validation

- Run `gofmt`, `go vet ./...`, `go test ./...`, `go test -race ./...`, and the integration script.
- Cover transactional rollback, join-code collision retries, idempotent join/reactivation, per-user list isolation, hidden non-member/dissolved results, ordering, and strict DTOs.
- Manually create, join, list, and view a group with two independent cookie jars.

## Completion Criteria

- Every protected read first proves active membership, and error differences cannot reveal private groups to non-members.
