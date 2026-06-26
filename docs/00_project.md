# Settled Project Plan

Settled is a lightweight app for small groups of friends to track shared expenses, record repayments, and see who owes whom. The first release should be useful for a trusted private circle, simple enough to ship, and structured so the core accounting logic can grow without forcing a rewrite.

## Product Intent

Settled is not a public finance network or a full Splitwise clone. The first users are the project owner and friends who already trust each other. The app should make a common workflow easy:

1. A user creates an account.
2. A user creates a group or joins one with a unique group code.
3. Group members add shared expenses.
4. Group members optionally record repayments made outside the app.
5. The app shows the current pairwise balances for the group.

The product should feel clear, calm, and practical. It should prioritize correctness, understandable flows, and basic privacy over feature breadth.

## First Release Scope

### In Scope

- Email and password registration.
- Email and password login.
- JWT-based authentication.
- Stateless backend authentication suitable for containerized horizontal scaling.
- Group creation.
- Joining a group by unique group code.
- Group owner permissions:
  - Rename the group.
  - View the group code.
  - Remove members.
  - Dissolve the group.
- Member permissions:
  - View joined groups.
  - View group details.
  - Add expenses.
  - Edit any group expense.
  - Record repayments.
  - View pairwise settlement results.
- Expense splitting:
  - Default split across all group members.
  - Optional participant selection.
  - Manual split by exact amount.
  - Manual split by percentage.
  - Validation that split totals equal the expense total.
- Default currency: USD.
- Pairwise settlement calculation:
  - Aggregate what each member owes each other member.
  - Net each pair down to a single direction and amount.

### Out of Scope For The First Release

- OAuth.
- Email verification.
- Password reset.
- Two-factor authentication.
- Multiple currencies.
- Exchange rates.
- Payment processing.
- Tracking whether a real-world payment actually completed.
- Global minimum-transfer settlement optimization.
- Complex audit logs or approval workflows.
- Public group discovery.
- Invite links with expiration or approval queues.
- Mobile or desktop shells such as Capacitor or Tauri.

## Technology Direction

Use the approved project stack:

- Backend: Go with the standard `net/http` package.
- Frontend: SvelteKit.
- UI: shadcn-svelte project conventions.
- Styling: Tailwind CSS and existing tokens in `src/routes/layout.css`.
- Database: PostgreSQL.
- App model: PWA-first web app.
- Release direction: containerized deployment.

Avoid adding a web framework, ORM, GraphQL layer, RPC framework, queue, cache, hosted service, or broad client state library unless the need becomes explicit and is approved first.

## Domain Model

The first version should keep the domain small and explicit.

### User

A user represents an authenticated person.

Core fields:

- `id`
- `email`
- `password_hash`
- `display_name`
- `created_at`
- `updated_at`

Notes:

- Email should be unique.
- Passwords must be hashed server-side.
- The API should never return `password_hash`.

### Group

A group is a private bill-sharing space.

Core fields:

- `id`
- `name`
- `join_code`
- `owner_user_id`
- `created_at`
- `updated_at`
- `dissolved_at`

Notes:

- `join_code` should be unique and hard to guess.
- A dissolved group should no longer accept new expenses or members.
- A dissolved group may either disappear from normal views or be shown as read-only later. For the first release, hiding dissolved groups from primary group lists is acceptable.

### Group Membership

Membership controls group access.

Core fields:

- `group_id`
- `user_id`
- `role`
- `joined_at`
- `removed_at`

Roles:

- `owner`
- `member`

Rules:

- Users may only access groups where they have an active membership.
- Only the owner can rename the group, view or rotate the group code, remove members, or dissolve the group.
- Regular members can create expenses, edit expenses, record repayments, and view balances.
- Any active member can edit any expense in the group.

### Expense

An expense records money paid by one member on behalf of selected participants.

Core fields:

- `id`
- `group_id`
- `paid_by_user_id`
- `description`
- `amount_cents`
- `currency`
- `expense_date`
- `created_by_user_id`
- `created_at`
- `updated_at`

Notes:

- First release currency is always `USD`.
- `amount_cents` avoids floating-point money errors.
- `paid_by_user_id` must be an active group member when the expense is created.
- `created_by_user_id` should be stored even though any member may edit expenses.

### Expense Split

An expense split records how much of an expense belongs to each participant.

Core fields:

- `expense_id`
- `user_id`
- `amount_cents`

Rules:

- Every split user must be an active group member.
- Split amounts must sum exactly to the expense amount.
- Percentage-based input should be converted to exact cents before persistence.
- Rounding should be deterministic. If percentages do not divide evenly into cents, assign the remainder consistently, such as by participant order.

### Repayment

A repayment records that one member says they paid another member outside the app.

Core fields:

- `id`
- `group_id`
- `from_user_id`
- `to_user_id`
- `amount_cents`
- `currency`
- `note`
- `repayment_date`
- `created_by_user_id`
- `created_at`
- `updated_at`

Rules:

- Both users must be active group members when the repayment is recorded.
- Repayments are manual ledger entries only.
- The app does not verify payment completion.

## Settlement Model

The first settlement engine should use pairwise netting.

For each expense:

- The payer paid the full expense amount.
- Each split participant owes their split amount.
- If the payer is also a participant, their own share cancels out.
- For every non-payer participant, add `participant owes payer amount`.

For each repayment:

- Add `recipient owes payer amount` or subtract `payer owes recipient amount`, depending on the internal ledger representation.
- The effect should reduce the payer's debt to the recipient.

After all ledger entries are generated:

1. Aggregate debt by ordered pair.
2. For each unordered pair of users, compare both directions.
3. Return only the net direction and amount.
4. Omit zero balances.

Example:

- Alice owes Bob $30.
- Bob owes Alice $10.
- Result: Alice owes Bob $20.

The implementation should hide this behind a small settlement interface or package boundary so a future global minimum-transfer algorithm can replace the pairwise strategy.

Suggested Go shape:

```go
type SettlementCalculator interface {
    Calculate(groupID string) ([]Transfer, error)
}
```

The first implementation can be `PairwiseSettlementCalculator`. Future work can add `MinimumTransferSettlementCalculator` without changing the API response shape.

## Backend Organization

Keep the backend direct and standard-library first.

Suggested structure:

```text
server/
  cmd/
    settled/
      main.go
  internal/
    auth/
    groups/
    expenses/
    settlements/
    users/
    httpapi/
    store/
```

Guidance:

- `cmd/settled/main.go` wires configuration, database, routes, and server startup.
- `internal/httpapi` owns request parsing, response writing, middleware, and route registration.
- Domain packages own focused business logic.
- `internal/store` owns PostgreSQL access.
- Avoid a generic repository layer unless repeated patterns prove it useful.
- Keep HTTP handlers thin: authenticate, validate input, call domain logic, return JSON.

## API Shape

Use simple JSON over HTTP.

### Auth

- `POST /api/auth/register`
- `POST /api/auth/login`
- `GET /api/me`

Login and register should return a JWT and the current user shape.

Example response:

```json
{
  "token": "jwt",
  "user": {
    "id": "user_id",
    "email": "alice@example.com",
    "displayName": "Alice"
  }
}
```

### Groups

- `GET /api/groups`
- `POST /api/groups`
- `POST /api/groups/join`
- `GET /api/groups/{groupID}`
- `PATCH /api/groups/{groupID}`
- `DELETE /api/groups/{groupID}`
- `GET /api/groups/{groupID}/join-code`
- `DELETE /api/groups/{groupID}/members/{userID}`

Authorization:

- All group routes require JWT authentication.
- A user must be an active member to read group data.
- Owner-only routes must verify the active membership role.

### Expenses

- `GET /api/groups/{groupID}/expenses`
- `POST /api/groups/{groupID}/expenses`
- `GET /api/groups/{groupID}/expenses/{expenseID}`
- `PATCH /api/groups/{groupID}/expenses/{expenseID}`
- `DELETE /api/groups/{groupID}/expenses/{expenseID}`

Authorization:

- Any active group member can create, edit, or delete expenses.
- The first release accepts collaborative trust over strict ownership.

### Repayments

- `GET /api/groups/{groupID}/repayments`
- `POST /api/groups/{groupID}/repayments`
- `PATCH /api/groups/{groupID}/repayments/{repaymentID}`
- `DELETE /api/groups/{groupID}/repayments/{repaymentID}`

Authorization:

- Any active group member can record or edit repayments.

### Settlements

- `GET /api/groups/{groupID}/settlements`

Response shape:

```json
{
  "groupId": "group_id",
  "currency": "USD",
  "transfers": [
    {
      "fromUserId": "alice_id",
      "toUserId": "bob_id",
      "amountCents": 2000
    }
  ]
}
```

This response shape should stay stable when the internal algorithm later changes from pairwise netting to global minimum-transfer settlement.

## Frontend Organization

The frontend should build actual app workflows, not a marketing page.

Suggested route shape:

```text
src/routes/
  +layout.svelte
  +page.svelte
  login/
    +page.svelte
  register/
    +page.svelte
  groups/
    +page.svelte
    [groupId]/
      +page.svelte
      expenses/
        new/
          +page.svelte
        [expenseId]/
          +page.svelte
```

First release screens:

- Register.
- Login.
- Group list.
- Create group dialog or screen.
- Join group by code.
- Group detail:
  - Members.
  - Expenses.
  - Repayments.
  - Settlement summary.
- Add or edit expense.
- Add or edit repayment.
- Owner group settings.

Design guidance:

- Keep the app dense enough for repeated use.
- Use existing shadcn-svelte components where available.
- Use restrained spacing and clear alignment.
- Avoid decorative layouts that slow down the workflow.
- Make mobile layouts deliberate because this app will often be used while discussing expenses in person.

## Validation Rules

Important boundary validations:

- Email is required and unique.
- Password meets a small minimum length.
- JWT is present and valid for protected routes.
- Group join code exists.
- User is an active member before accessing group data.
- Owner-only actions check owner role.
- Expense amount is positive.
- Expense split participants are active group members.
- Expense split total equals expense amount.
- Percentage split totals equal 100% before conversion, allowing only a clearly defined precision.
- Repayment amount is positive.
- Repayment sender and recipient are different active group members.

## Implementation Order

### Phase 1: Foundation

1. Replace starter README content with project-specific setup notes.
2. Create backend server entry point with health route.
3. Add basic configuration loading.
4. Establish PostgreSQL connection.
5. Add initial database schema or migration approach.
6. Add password hashing and JWT utilities.

### Phase 2: Authentication

1. Implement registration.
2. Implement login.
3. Implement auth middleware.
4. Implement `GET /api/me`.
5. Add frontend login and register flows.
6. Store JWT client-side in a deliberate way and attach it to API requests.

### Phase 3: Groups And Membership

1. Implement group creation.
2. Implement group list scoped to current user.
3. Implement join by code.
4. Implement owner-only group settings.
5. Add group list, create group, join group, and group detail UI.

### Phase 4: Expenses

1. Implement expense create, list, read, update, and delete.
2. Implement split validation.
3. Add frontend expense form with equal split, participant selection, amount split, and percentage split modes.
4. Show expenses in the group detail view.

### Phase 5: Repayments And Settlements

1. Implement repayment create, list, update, and delete.
2. Implement pairwise settlement calculation.
3. Add settlement endpoint.
4. Show current settlement summary in the group detail view.
5. Add repayment form from settlement suggestions.

### Phase 6: Small-Group Release Polish

1. Add empty states and basic loading states.
2. Add clear error messages for auth, group joins, and invalid splits.
3. Check mobile layouts.
4. Add production build checks.
5. Prepare containerized deployment.

## Testing Strategy

Backend tests should focus on correctness-heavy logic:

- Auth input validation.
- Group membership and owner authorization.
- Expense split validation.
- Pairwise settlement calculation.
- Repayment effects on settlement output.

Frontend checks should start with:

- Svelte type checking.
- Production build when routes or app structure change.
- Focused component or integration tests only when the project has a test setup.

Quality gates after code changes:

- Run `gofmt` on changed Go files.
- Run `go test ./...` from `server/` once backend code exists.
- Run `pnpm check` for frontend changes.
- Run `pnpm build` when routing, production output, or PWA behavior changes.

## Open Decisions

These should be resolved before or during implementation:

- Exact JWT storage strategy in the browser.
- Token expiry duration and refresh behavior.
- Whether dissolved groups are hidden or shown read-only.
- Whether deleted expenses and repayments are hard-deleted or soft-deleted.
- Whether group owners can transfer ownership.
- Whether the join code can be rotated.
- How much split precision percentage input allows.

## Success Criteria

The first release is successful when:

- A friend can register, log in, and join a group with a code.
- A group member can add a shared expense in under a minute.
- The app prevents invalid splits before saving.
- A user only sees groups they belong to.
- Owner-only group actions are blocked for regular members.
- The group settlement view clearly shows current pairwise debts.
- Recording a repayment updates the settlement view correctly.
- The app can be deployed as a small containerized service backed by PostgreSQL.
