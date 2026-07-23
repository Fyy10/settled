# Settled Database Entity Design

This document defines the first-release PostgreSQL schema design for Settled. It follows the product plan in `00_project.md` and the system architecture in `01_architecture.md`.

The design uses simple relational entities with database constraints for core integrity. It does not introduce event sourcing, audit logs, revision tables, materialized views, ORM assumptions, or migration tooling.

## Design Goals

- Keep the schema small, explicit, and easy to query from Go.
- Store enough durable data to recompute group balances from expenses, splits, and repayments.
- Use PostgreSQL constraints, foreign keys, checks, and indexes to protect common integrity rules.
- Keep settlement results derived, not persisted.
- Preserve current records with status timestamps instead of keeping historical versions.
- Leave authorization and cross-row workflow checks in the backend where PostgreSQL declarative constraints cannot express them cleanly.

## Conventions

### Identifiers

Use PostgreSQL `uuid` columns for entity identifiers.

The application should generate UUIDs before insertion unless the project later chooses a database UUID extension. This avoids requiring `pgcrypto` or `uuid-ossp` in the initial schema.

### Money

All money is stored as integer cents:

```sql
amount_cents bigint NOT NULL CHECK (amount_cents > 0)
```

The first release supports only USD:

```sql
currency char(3) NOT NULL DEFAULT 'USD' CHECK (currency = 'USD')
```

### Timestamps

Use `timestamptz` for all timestamps.

Common mutable entities include:

```sql
created_at timestamptz NOT NULL DEFAULT now()
updated_at timestamptz NOT NULL DEFAULT now()
```

`updated_at` should be maintained by application code in the first release. A trigger can be added later if repeated application updates become error-prone.

### Soft State

The schema keeps records current without historical versions:

- `groups.dissolved_at` marks a group as dissolved.
- `group_memberships.removed_at` marks a member as removed.
- `expenses.deleted_at` marks an expense as deleted.
- `repayments.deleted_at` marks a repayment as deleted.

Deleted expenses and repayments are excluded from normal list and settlement views. They remain available for developer inspection and possible future recovery, but they are not an audit trail.

## Tables

### users

Authenticated people who can join groups and create bill activity.

```sql
CREATE TABLE users (
  id uuid PRIMARY KEY,
  email text NOT NULL,
  password_hash text NOT NULL,
  display_name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT users_email_not_blank CHECK (btrim(email) <> ''),
  CONSTRAINT users_display_name_not_blank CHECK (btrim(display_name) <> ''),
  CONSTRAINT users_display_name_length CHECK (char_length(display_name) <= 120)
);
```

Indexes:

```sql
CREATE UNIQUE INDEX users_email_lower_unique_idx ON users (lower(email));
```

Notes:

- The backend should normalize email before display and login comparisons.
- The API must never return `password_hash`.
- Password rules and hash algorithm choice belong to the auth implementation, not the schema.

### groups

Private spaces where members share expenses.

```sql
CREATE TABLE groups (
  id uuid PRIMARY KEY,
  name text NOT NULL,
  join_code text NOT NULL,
  owner_user_id uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  dissolved_at timestamptz,

  CONSTRAINT groups_name_not_blank CHECK (btrim(name) <> ''),
  CONSTRAINT groups_name_length CHECK (char_length(name) <= 160),
  CONSTRAINT groups_join_code_not_blank CHECK (btrim(join_code) <> ''),
  CONSTRAINT groups_join_code_length CHECK (char_length(join_code) BETWEEN 8 AND 64)
);
```

Indexes:

```sql
CREATE UNIQUE INDEX groups_join_code_unique_idx ON groups (join_code);
CREATE INDEX groups_owner_user_id_idx ON groups (owner_user_id);
CREATE INDEX groups_active_idx ON groups (id) WHERE dissolved_at IS NULL;
```

Notes:

- `join_code` is globally unique for the first release.
- `owner_user_id` is the single source of truth for group ownership.
- The backend must create the group and owner membership in one transaction.
- The backend must prevent removing the current owner from the group.
- Dissolved groups should not accept new members, expenses, or repayments.

### group_memberships

Membership records for group access.

```sql
CREATE TABLE group_memberships (
  group_id uuid NOT NULL REFERENCES groups(id),
  user_id uuid NOT NULL REFERENCES users(id),
  joined_at timestamptz NOT NULL DEFAULT now(),
  removed_at timestamptz,

  PRIMARY KEY (group_id, user_id),
  CONSTRAINT group_memberships_removed_after_joined CHECK (
    removed_at IS NULL OR removed_at >= joined_at
  )
);
```

Indexes:

```sql
CREATE INDEX group_memberships_user_active_idx
  ON group_memberships (user_id, group_id)
  WHERE removed_at IS NULL;

CREATE INDEX group_memberships_group_active_idx
  ON group_memberships (group_id, user_id)
  WHERE removed_at IS NULL;
```

Notes:

- A user has at most one membership row per group.
- Rejoining a group reactivates the existing row by clearing `removed_at` instead of inserting a second row.
- Ownership is derived only from `groups.owner_user_id`.
- PostgreSQL foreign keys cannot directly require "the referenced membership is active" without triggers. The backend must enforce active membership checks for protected workflows.
- Owner transfer is out of scope for the first release.

### expenses

An expense records money paid by one group member for selected participants.

```sql
CREATE TABLE expenses (
  id uuid PRIMARY KEY,
  group_id uuid NOT NULL REFERENCES groups(id),
  paid_by_user_id uuid NOT NULL REFERENCES users(id),
  description text NOT NULL,
  amount_cents bigint NOT NULL,
  currency char(3) NOT NULL DEFAULT 'USD',
  expense_date date NOT NULL,
  created_by_user_id uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT expenses_amount_positive CHECK (amount_cents > 0),
  CONSTRAINT expenses_currency_usd CHECK (currency = 'USD'),
  CONSTRAINT expenses_description_not_blank CHECK (btrim(description) <> ''),
  CONSTRAINT expenses_description_length CHECK (char_length(description) <= 240),
  CONSTRAINT expenses_deleted_after_created CHECK (
    deleted_at IS NULL OR deleted_at >= created_at
  ),
  CONSTRAINT expenses_id_group_unique UNIQUE (id, group_id),
  CONSTRAINT expenses_paid_by_membership_fk
    FOREIGN KEY (group_id, paid_by_user_id)
    REFERENCES group_memberships(group_id, user_id),
  CONSTRAINT expenses_created_by_membership_fk
    FOREIGN KEY (group_id, created_by_user_id)
    REFERENCES group_memberships(group_id, user_id)
);
```

Indexes:

```sql
CREATE INDEX expenses_group_date_idx
  ON expenses (group_id, expense_date DESC, created_at DESC)
  WHERE deleted_at IS NULL;

CREATE INDEX expenses_group_paid_by_idx
  ON expenses (group_id, paid_by_user_id)
  WHERE deleted_at IS NULL;

CREATE INDEX expenses_group_created_by_idx
  ON expenses (group_id, created_by_user_id)
  WHERE deleted_at IS NULL;
```

Notes:

- `paid_by_user_id` and `created_by_user_id` must belong to the expense group.
- The backend must additionally verify those memberships are active and the group is not dissolved when creating or editing.
- Editing an expense updates this row and replaces its split rows in one transaction.
- Deleting an expense sets `deleted_at`.

### expense_splits

The participant shares for an expense.

```sql
CREATE TABLE expense_splits (
  expense_id uuid NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
  group_id uuid NOT NULL,
  user_id uuid NOT NULL REFERENCES users(id),
  amount_cents bigint NOT NULL,

  PRIMARY KEY (expense_id, user_id),
  CONSTRAINT expense_splits_amount_positive CHECK (amount_cents > 0),
  CONSTRAINT expense_splits_user_membership_fk
    FOREIGN KEY (group_id, user_id)
    REFERENCES group_memberships(group_id, user_id),
  CONSTRAINT expense_splits_expense_group_fk
    FOREIGN KEY (expense_id, group_id)
    REFERENCES expenses(id, group_id)
);
```

Indexes:

```sql
CREATE INDEX expense_splits_group_user_idx
  ON expense_splits (group_id, user_id);
```

Notes:

- `group_id` is duplicated from `expenses` so the database can enforce that split users are members of the same group.
- Split participants must be active members when the expense is saved. The backend enforces activity.
- Split totals must equal `expenses.amount_cents`. PostgreSQL declarative constraints cannot express this aggregate check directly. The backend must validate it in the same transaction that writes the expense and splits.
- Percentage splits should be converted to exact cents before insertion.
- Remainders from percentage conversion should be assigned deterministically by participant order.

### repayments

A manual record that one member says they repaid another member outside the app.

```sql
CREATE TABLE repayments (
  id uuid PRIMARY KEY,
  group_id uuid NOT NULL REFERENCES groups(id),
  from_user_id uuid NOT NULL REFERENCES users(id),
  to_user_id uuid NOT NULL REFERENCES users(id),
  amount_cents bigint NOT NULL,
  currency char(3) NOT NULL DEFAULT 'USD',
  note text,
  repayment_date date NOT NULL,
  created_by_user_id uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT repayments_amount_positive CHECK (amount_cents > 0),
  CONSTRAINT repayments_currency_usd CHECK (currency = 'USD'),
  CONSTRAINT repayments_users_different CHECK (from_user_id <> to_user_id),
  CONSTRAINT repayments_note_length CHECK (note IS NULL OR char_length(note) <= 240),
  CONSTRAINT repayments_deleted_after_created CHECK (
    deleted_at IS NULL OR deleted_at >= created_at
  ),
  CONSTRAINT repayments_from_membership_fk
    FOREIGN KEY (group_id, from_user_id)
    REFERENCES group_memberships(group_id, user_id),
  CONSTRAINT repayments_to_membership_fk
    FOREIGN KEY (group_id, to_user_id)
    REFERENCES group_memberships(group_id, user_id),
  CONSTRAINT repayments_created_by_membership_fk
    FOREIGN KEY (group_id, created_by_user_id)
    REFERENCES group_memberships(group_id, user_id)
);
```

Indexes:

```sql
CREATE INDEX repayments_group_date_idx
  ON repayments (group_id, repayment_date DESC, created_at DESC)
  WHERE deleted_at IS NULL;

CREATE INDEX repayments_group_from_user_idx
  ON repayments (group_id, from_user_id)
  WHERE deleted_at IS NULL;

CREATE INDEX repayments_group_to_user_idx
  ON repayments (group_id, to_user_id)
  WHERE deleted_at IS NULL;
```

Notes:

- `from_user_id` is the member who made the repayment.
- `to_user_id` is the member who received the repayment.
- The backend must verify both users and the creator are active members and the group is not dissolved when creating or editing.
- Deleting a repayment sets `deleted_at`.

## Views

Views are ordinary PostgreSQL views used to simplify common queries. They are not materialized and do not persist settlement results.

### active_group_memberships

Active memberships joined to user profile data.

```sql
CREATE VIEW active_group_memberships AS
SELECT
  gm.group_id,
  gm.user_id,
  (gm.user_id = g.owner_user_id) AS is_owner,
  gm.joined_at,
  u.email,
  u.display_name
FROM group_memberships gm
JOIN users u ON u.id = gm.user_id
JOIN groups g ON g.id = gm.group_id
WHERE gm.removed_at IS NULL
  AND g.dissolved_at IS NULL;
```

Use cases:

- Group member list.
- Authorization helper queries.
- Owner permission checks.

### active_expenses

Non-deleted expenses in non-dissolved groups.

```sql
CREATE VIEW active_expenses AS
SELECT e.*
FROM expenses e
JOIN groups g ON g.id = e.group_id
WHERE e.deleted_at IS NULL
  AND g.dissolved_at IS NULL;
```

Use cases:

- Expense list.
- Expense details.
- Settlement inputs.

### active_repayments

Non-deleted repayments in non-dissolved groups.

```sql
CREATE VIEW active_repayments AS
SELECT r.*
FROM repayments r
JOIN groups g ON g.id = r.group_id
WHERE r.deleted_at IS NULL
  AND g.dissolved_at IS NULL;
```

Use cases:

- Repayment list.
- Settlement inputs.

### settlement_debt_entries

Normalized ledger-style debt entries used by pairwise settlement.

Each row means:

```text
from_user_id owes to_user_id amount_cents
```

```sql
CREATE VIEW settlement_debt_entries AS
SELECT
  e.group_id,
  es.user_id AS from_user_id,
  e.paid_by_user_id AS to_user_id,
  es.amount_cents,
  e.currency,
  e.expense_date AS occurred_on,
  e.created_at
FROM active_expenses e
JOIN expense_splits es ON es.expense_id = e.id
WHERE es.user_id <> e.paid_by_user_id

UNION ALL

SELECT
  r.group_id,
  r.to_user_id AS from_user_id,
  r.from_user_id AS to_user_id,
  r.amount_cents,
  r.currency,
  r.repayment_date AS occurred_on,
  r.created_at
FROM active_repayments r;
```

Notes:

- Expense splits generate debt from participant to payer.
- Repayments generate reverse debt from recipient to payer, which reduces the payer's debt to the recipient after pairwise netting.
- The view intentionally uses `UNION ALL` because entries should be aggregated by later views or queries.

### pairwise_gross_balances

Aggregated directional debt before netting opposing directions.

```sql
CREATE VIEW pairwise_gross_balances AS
SELECT
  group_id,
  from_user_id,
  to_user_id,
  currency,
  sum(amount_cents)::bigint AS amount_cents
FROM settlement_debt_entries
GROUP BY group_id, from_user_id, to_user_id, currency
HAVING sum(amount_cents) > 0;
```

Use cases:

- Debugging settlement inputs.
- Backend settlement query step.

### pairwise_net_balances

Final first-release pairwise settlement result.

Each row means:

```text
from_user_id should pay to_user_id amount_cents
```

```sql
CREATE VIEW pairwise_net_balances AS
WITH unordered_pairs AS (
  SELECT
    group_id,
    LEAST(from_user_id, to_user_id) AS user_a_id,
    GREATEST(from_user_id, to_user_id) AS user_b_id,
    currency,
    sum(
      CASE
        WHEN from_user_id = LEAST(from_user_id, to_user_id)
        THEN amount_cents
        ELSE -amount_cents
      END
    )::bigint AS user_a_owes_user_b_cents
  FROM pairwise_gross_balances
  GROUP BY group_id, user_a_id, user_b_id, currency
)
SELECT
  group_id,
  CASE
    WHEN user_a_owes_user_b_cents > 0 THEN user_a_id
    ELSE user_b_id
  END AS from_user_id,
  CASE
    WHEN user_a_owes_user_b_cents > 0 THEN user_b_id
    ELSE user_a_id
  END AS to_user_id,
  abs(user_a_owes_user_b_cents)::bigint AS amount_cents,
  currency
FROM unordered_pairs
WHERE user_a_owes_user_b_cents <> 0;
```

Use cases:

- `GET /api/groups/{groupID}/settlements`.
- Group settlement summary.

Notes:

- This is pairwise netting, not global minimum-transfer optimization.
- The response shape can remain stable if a later algorithm replaces this view with Go-calculated results.

## Integrity Rules

### Enforced By Declarative Database Constraints

- User emails are unique case-insensitively.
- Group join codes are unique.
- Group ownership has a single source of truth: `groups.owner_user_id`.
- Expense and repayment amounts are positive.
- Currency is USD.
- Repayment sender and recipient are different users.
- Expense payer, expense creator, split participants, repayment sender, repayment recipient, and repayment creator all belong to the referenced group.
- Split rows belong to the same group as their expense.
- Deleted or removed timestamps cannot precede creation or join timestamps.

### Enforced By Backend Transactions

The following rules require cross-row checks, aggregate checks, or workflow context and should be enforced by the Go backend in the same transaction as the write:

- Group creation also inserts the owner's active membership.
- A user can only join a non-dissolved group.
- A dissolved group cannot accept new expenses, repayments, joins, or edits.
- A removed member cannot create, edit, or participate in new expenses or repayments.
- Only the active group owner from `groups.owner_user_id` can rename, dissolve, remove members, or view/rotate join codes.
- The current owner cannot be removed from their group.
- A member cannot be removed while any current non-deleted expense or repayment uses that member as payer, split participant, repayment sender, or repayment recipient.
- Because removed members cannot participate in settlement-affecting fields, ordinary settlement results should only reference active members.
- Expense split totals equal the expense amount.
- Percentage split inputs total 100 percent before cent conversion.
- Expense and repayment edits replace the current values without creating historical versions.
- Soft-deleted expenses and repayments are excluded from user-facing workflows.

## Query Patterns And Index Coverage

### Current User Groups

Expected query shape:

```sql
SELECT g.*
FROM group_memberships gm
JOIN groups g ON g.id = gm.group_id
WHERE gm.user_id = $1
  AND gm.removed_at IS NULL
  AND g.dissolved_at IS NULL
ORDER BY g.updated_at DESC;
```

Covered primarily by:

- `group_memberships_user_active_idx`
- `groups_active_idx`

### Join Group By Code

Expected query shape:

```sql
SELECT *
FROM groups
WHERE join_code = $1
  AND dissolved_at IS NULL;
```

Covered primarily by:

- `groups_join_code_unique_idx`

### Group Expenses

Expected query shape:

```sql
SELECT *
FROM active_expenses
WHERE group_id = $1
ORDER BY expense_date DESC, created_at DESC;
```

Covered primarily by:

- `expenses_group_date_idx`

### Group Repayments

Expected query shape:

```sql
SELECT *
FROM active_repayments
WHERE group_id = $1
ORDER BY repayment_date DESC, created_at DESC;
```

Covered primarily by:

- `repayments_group_date_idx`

### Group Settlement

Expected query shape:

```sql
SELECT *
FROM pairwise_net_balances
WHERE group_id = $1
ORDER BY amount_cents DESC;
```

Covered by indexes on the base tables:

- `expenses_group_date_idx`
- `expense_splits_group_user_idx`
- `repayments_group_date_idx`

## Deletion Strategy

First-release user-facing deletes should be soft deletes for accounting records:

- Expense delete: set `expenses.deleted_at`.
- Repayment delete: set `repayments.deleted_at`.
- Member removal: set `group_memberships.removed_at`.
- Group dissolution: set `groups.dissolved_at`.

Hard deletes should be reserved for local development resets or future administrative tooling. User deletion and data export are out of scope for the first release.

## Open Implementation Notes

- If the project later chooses database-generated UUIDs, add one PostgreSQL extension decision and default expressions consistently.
- If maintaining `updated_at` in application code becomes noisy, add a small `updated_at` trigger in a migration.
- If settlement query performance becomes an issue, measure first. The first scaling move should be indexes or query tuning, not materialized views or precomputed settlement tables.
- If the product later needs a real audit trail, add a separate audit/revision design rather than overloading `updated_at` and soft-delete timestamps.
