BEGIN;

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

CREATE UNIQUE INDEX users_email_lower_unique_idx ON users (lower(email));

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

CREATE UNIQUE INDEX groups_join_code_unique_idx ON groups (join_code);
CREATE INDEX groups_owner_user_id_idx ON groups (owner_user_id);
CREATE INDEX groups_active_idx ON groups (id) WHERE dissolved_at IS NULL;

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

CREATE INDEX group_memberships_user_active_idx
  ON group_memberships (user_id, group_id)
  WHERE removed_at IS NULL;

CREATE INDEX group_memberships_group_active_idx
  ON group_memberships (group_id, user_id)
  WHERE removed_at IS NULL;

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

CREATE INDEX expenses_group_date_idx
  ON expenses (group_id, expense_date DESC, created_at DESC)
  WHERE deleted_at IS NULL;

CREATE INDEX expenses_group_paid_by_idx
  ON expenses (group_id, paid_by_user_id)
  WHERE deleted_at IS NULL;

CREATE INDEX expenses_group_created_by_idx
  ON expenses (group_id, created_by_user_id)
  WHERE deleted_at IS NULL;

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

CREATE INDEX expense_splits_group_user_idx
  ON expense_splits (group_id, user_id);

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

CREATE INDEX repayments_group_date_idx
  ON repayments (group_id, repayment_date DESC, created_at DESC)
  WHERE deleted_at IS NULL;

CREATE INDEX repayments_group_from_user_idx
  ON repayments (group_id, from_user_id)
  WHERE deleted_at IS NULL;

CREATE INDEX repayments_group_to_user_idx
  ON repayments (group_id, to_user_id)
  WHERE deleted_at IS NULL;

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

CREATE VIEW active_expenses AS
SELECT e.*
FROM expenses e
JOIN groups g ON g.id = e.group_id
WHERE e.deleted_at IS NULL
  AND g.dissolved_at IS NULL;

CREATE VIEW active_repayments AS
SELECT r.*
FROM repayments r
JOIN groups g ON g.id = r.group_id
WHERE r.deleted_at IS NULL
  AND g.dissolved_at IS NULL;

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

COMMIT;
