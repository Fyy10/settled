# Mini Task 20: Group Owner Settings UI

## Goal

Let owners safely rename a group, view/copy its code, remove members, and dissolve the group.

## Prerequisites

Mini Tasks 9 and 19.

## Implementation Scope

- Implement owner-only `/groups/{groupId}/settings`; after the API proves insufficient role, redirect direct non-owner visits back to the group.
- Refresh both group detail and group list after rename.
- Fetch the join code only when its section becomes visible; render it read-only, tabular, and selectable so manual copying remains possible after Clipboard API failure.
- Show member-removal actions only to owners and never for the owner row; use AlertDialog and the documented `409` conflict explanation.
- Require typing the current group name before final dissolution; clear group-local state, refresh the list, and replace history with `/groups` on success.
- Handle permission changes, `403`/`404`, pending state, toast, and destructive-action focus.

## Exclusions

- Do not implement code rotation, ownership transfer, conflict-record discovery, or recovery; do not label dissolution as deletion.

## Validation

- Run `pnpm check`, `pnpm test`, and `pnpm build`.
- Component-test direct-route guarding, lazy code loading, clipboard failure, member `409`, typed confirmation, hidden member controls, and permission changes.
- Manually complete all four workflows as both owner and member.

## Completion Criteria

- UI hiding is only a usability aid; backend decisions remain authoritative and hidden resources do not leak.
