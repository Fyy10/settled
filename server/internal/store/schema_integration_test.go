//go:build integration

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	aliceID = "00000000-0000-4000-8000-000000000001"
	bobID   = "00000000-0000-4000-8000-000000000002"
	carolID = "00000000-0000-4000-8000-000000000003"
	daveID  = "00000000-0000-4000-8000-000000000004"
	erinID  = "00000000-0000-4000-8000-000000000005"

	groupOneID = "10000000-0000-4000-8000-000000000001"
	groupTwoID = "10000000-0000-4000-8000-000000000002"

	expenseOneID     = "20000000-0000-4000-8000-000000000001"
	expenseTwoID     = "20000000-0000-4000-8000-000000000002"
	expenseThreeID   = "20000000-0000-4000-8000-000000000003"
	expenseFourID    = "20000000-0000-4000-8000-000000000004"
	repaymentOneID   = "30000000-0000-4000-8000-000000000001"
	repaymentTwoID   = "30000000-0000-4000-8000-000000000002"
	repaymentThreeID = "30000000-0000-4000-8000-000000000003"
)

func TestSchemaObjects(t *testing.T) {
	db := openIntegrationDatabase(t)

	relations := map[string]string{
		"users":                    "r",
		"groups":                   "r",
		"group_memberships":        "r",
		"expenses":                 "r",
		"expense_splits":           "r",
		"repayments":               "r",
		"active_group_memberships": "v",
		"active_expenses":          "v",
		"active_repayments":        "v",
		"settlement_debt_entries":  "v",
		"pairwise_gross_balances":  "v",
		"pairwise_net_balances":    "v",
	}
	for name, wantKind := range relations {
		var gotKind string
		err := db.QueryRow(`
			SELECT c.relkind::text
			FROM pg_catalog.pg_class c
			JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = 'public' AND c.relname = $1
		`, name).Scan(&gotKind)
		if err != nil {
			t.Fatalf("look up relation %s: %v", name, err)
		}
		if gotKind != wantKind {
			t.Errorf("relation %s kind = %q, want %q", name, gotKind, wantKind)
		}
	}

	wantIndexes := []string{
		"users_email_lower_unique_idx",
		"groups_join_code_unique_idx",
		"groups_owner_user_id_idx",
		"groups_active_idx",
		"group_memberships_user_active_idx",
		"group_memberships_group_active_idx",
		"expenses_group_date_idx",
		"expenses_group_paid_by_idx",
		"expenses_group_created_by_idx",
		"expense_splits_group_user_idx",
		"repayments_group_date_idx",
		"repayments_group_from_user_idx",
		"repayments_group_to_user_idx",
	}
	assertCatalogNames(t, db, `
		SELECT indexname
		FROM pg_catalog.pg_indexes
		WHERE schemaname = 'public'
	`, wantIndexes)

	wantConstraints := []string{
		"users_email_not_blank",
		"users_display_name_not_blank",
		"users_display_name_length",
		"groups_name_not_blank",
		"groups_name_length",
		"groups_join_code_not_blank",
		"groups_join_code_length",
		"group_memberships_removed_after_joined",
		"expenses_amount_positive",
		"expenses_currency_usd",
		"expenses_description_not_blank",
		"expenses_description_length",
		"expenses_deleted_after_created",
		"expenses_id_group_unique",
		"expenses_paid_by_membership_fk",
		"expenses_created_by_membership_fk",
		"expense_splits_amount_positive",
		"expense_splits_user_membership_fk",
		"expense_splits_expense_group_fk",
		"repayments_amount_positive",
		"repayments_currency_usd",
		"repayments_users_different",
		"repayments_note_length",
		"repayments_deleted_after_created",
		"repayments_from_membership_fk",
		"repayments_to_membership_fk",
		"repayments_created_by_membership_fk",
	}
	assertCatalogNames(t, db, `
		SELECT conname
		FROM pg_catalog.pg_constraint c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.connamespace
		WHERE n.nspname = 'public'
	`, wantConstraints)
}

func TestSchemaConstraints(t *testing.T) {
	db := openIntegrationDatabase(t)

	t.Run("case insensitive email uniqueness", func(t *testing.T) {
		resetIntegrationDatabase(t, db)
		insertUser(t, db, aliceID, "Alice@Example.com", "Alice")

		_, err := db.Exec(`
			INSERT INTO users (id, email, password_hash, display_name)
			VALUES ($1, $2, 'hash', 'Other Alice')
		`, bobID, "alice@example.COM")
		requireConstraint(t, err, "users_email_lower_unique_idx")
		if got := databaseErrorKindOf(translateDatabaseError(err)); got != databaseErrorDuplicateEmail {
			t.Errorf("translated error kind = %d, want duplicate email", got)
		}
	})

	t.Run("join code uniqueness", func(t *testing.T) {
		resetIntegrationDatabase(t, db)
		insertUser(t, db, aliceID, "alice@example.com", "Alice")
		insertGroup(t, db, groupOneID, aliceID, "FIRSTCODE123")

		_, err := db.Exec(`
			INSERT INTO groups (id, name, join_code, owner_user_id)
			VALUES ($1, 'Other group', 'FIRSTCODE123', $2)
		`, groupTwoID, aliceID)
		requireConstraint(t, err, "groups_join_code_unique_idx")
		if got := databaseErrorKindOf(translateDatabaseError(err)); got != databaseErrorJoinCodeCollision {
			t.Errorf("translated error kind = %d, want join code collision", got)
		}
	})

	t.Run("membership removal timestamp", func(t *testing.T) {
		resetIntegrationDatabase(t, db)
		seedUsersAndGroups(t, db)
		joinedAt := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

		_, err := db.Exec(`
			INSERT INTO group_memberships (
				group_id,
				user_id,
				joined_at,
				removed_at
			)
			VALUES ($1, $2, $3, $4)
		`, groupOneID, erinID, joinedAt, joinedAt.Add(-time.Second))
		requireConstraint(t, err, "group_memberships_removed_after_joined")
	})

	t.Run("expense checks and memberships", func(t *testing.T) {
		resetIntegrationDatabase(t, db)
		seedUsersAndGroups(t, db)

		_, err := insertExpense(
			db,
			expenseOneID,
			groupOneID,
			aliceID,
			aliceID,
			0,
			"USD",
			nil,
		)
		requireConstraint(t, err, "expenses_amount_positive")

		_, err = insertExpense(
			db,
			expenseOneID,
			groupOneID,
			aliceID,
			aliceID,
			100,
			"EUR",
			nil,
		)
		requireConstraint(t, err, "expenses_currency_usd")

		_, err = insertExpense(
			db,
			expenseOneID,
			groupOneID,
			daveID,
			aliceID,
			100,
			"USD",
			nil,
		)
		requireConstraint(t, err, "expenses_paid_by_membership_fk")

		_, err = insertExpense(
			db,
			expenseOneID,
			groupOneID,
			aliceID,
			daveID,
			100,
			"USD",
			nil,
		)
		requireConstraint(t, err, "expenses_created_by_membership_fk")
	})

	t.Run("split checks group integrity and cascade", func(t *testing.T) {
		resetIntegrationDatabase(t, db)
		seedUsersAndGroups(t, db)
		if _, err := insertExpense(
			db,
			expenseOneID,
			groupOneID,
			aliceID,
			aliceID,
			100,
			"USD",
			nil,
		); err != nil {
			t.Fatalf("insert expense: %v", err)
		}

		_, err := db.Exec(`
			INSERT INTO expense_splits (expense_id, group_id, user_id, amount_cents)
			VALUES ($1, $2, $3, 0)
		`, expenseOneID, groupOneID, bobID)
		requireConstraint(t, err, "expense_splits_amount_positive")

		_, err = db.Exec(`
			INSERT INTO expense_splits (expense_id, group_id, user_id, amount_cents)
			VALUES ($1, $2, $3, 100)
		`, expenseOneID, groupOneID, daveID)
		requireConstraint(t, err, "expense_splits_user_membership_fk")

		_, err = db.Exec(`
			INSERT INTO expense_splits (expense_id, group_id, user_id, amount_cents)
			VALUES ($1, $2, $3, 100)
		`, expenseOneID, groupTwoID, daveID)
		requireConstraint(t, err, "expense_splits_expense_group_fk")

		if _, err := db.Exec(`
			INSERT INTO expense_splits (expense_id, group_id, user_id, amount_cents)
			VALUES ($1, $2, $3, 100)
		`, expenseOneID, groupOneID, bobID); err != nil {
			t.Fatalf("insert valid split: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM expenses WHERE id = $1`, expenseOneID); err != nil {
			t.Fatalf("delete expense: %v", err)
		}

		var splitCount int
		if err := db.QueryRow(`
			SELECT count(*) FROM expense_splits WHERE expense_id = $1
		`, expenseOneID).Scan(&splitCount); err != nil {
			t.Fatalf("count cascaded splits: %v", err)
		}
		if splitCount != 0 {
			t.Errorf("split count after expense deletion = %d, want zero", splitCount)
		}
	})

	t.Run("repayment checks and memberships", func(t *testing.T) {
		resetIntegrationDatabase(t, db)
		seedUsersAndGroups(t, db)

		_, err := insertRepayment(
			db,
			repaymentOneID,
			groupOneID,
			aliceID,
			bobID,
			aliceID,
			0,
			"USD",
			nil,
		)
		requireConstraint(t, err, "repayments_amount_positive")

		_, err = insertRepayment(
			db,
			repaymentOneID,
			groupOneID,
			aliceID,
			bobID,
			aliceID,
			100,
			"EUR",
			nil,
		)
		requireConstraint(t, err, "repayments_currency_usd")

		_, err = insertRepayment(
			db,
			repaymentOneID,
			groupOneID,
			aliceID,
			aliceID,
			aliceID,
			100,
			"USD",
			nil,
		)
		requireConstraint(t, err, "repayments_users_different")

		_, err = insertRepayment(
			db,
			repaymentOneID,
			groupOneID,
			daveID,
			bobID,
			aliceID,
			100,
			"USD",
			nil,
		)
		requireConstraint(t, err, "repayments_from_membership_fk")

		_, err = insertRepayment(
			db,
			repaymentOneID,
			groupOneID,
			aliceID,
			daveID,
			aliceID,
			100,
			"USD",
			nil,
		)
		requireConstraint(t, err, "repayments_to_membership_fk")

		_, err = insertRepayment(
			db,
			repaymentOneID,
			groupOneID,
			aliceID,
			bobID,
			daveID,
			100,
			"USD",
			nil,
		)
		requireConstraint(t, err, "repayments_created_by_membership_fk")

		longNote := strings.Repeat("x", 241)
		_, err = db.Exec(`
			INSERT INTO repayments (
				id,
				group_id,
				from_user_id,
				to_user_id,
				amount_cents,
				currency,
				note,
				repayment_date,
				created_by_user_id
			)
			VALUES ($1, $2, $3, $4, 100, 'USD', $5, '2026-06-30', $6)
		`, repaymentOneID, groupOneID, aliceID, bobID, longNote, aliceID)
		requireConstraint(t, err, "repayments_note_length")
	})

	t.Run("soft deletion timestamps", func(t *testing.T) {
		resetIntegrationDatabase(t, db)
		seedUsersAndGroups(t, db)
		createdAt := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
		deletedAt := createdAt.Add(-time.Second)

		_, err := insertExpense(
			db,
			expenseOneID,
			groupOneID,
			aliceID,
			aliceID,
			100,
			"USD",
			&recordTimes{
				createdAt: createdAt,
				deletedAt: &deletedAt,
			},
		)
		requireConstraint(t, err, "expenses_deleted_after_created")

		_, err = insertRepayment(
			db,
			repaymentOneID,
			groupOneID,
			aliceID,
			bobID,
			aliceID,
			100,
			"USD",
			&recordTimes{
				createdAt: createdAt,
				deletedAt: &deletedAt,
			},
		)
		requireConstraint(t, err, "repayments_deleted_after_created")
	})
}

func TestActiveAndSettlementViews(t *testing.T) {
	db := openIntegrationDatabase(t)
	resetIntegrationDatabase(t, db)
	seedSettlementFixture(t, db)

	type activeMember struct {
		userID      string
		isOwner     bool
		email       string
		displayName string
	}
	var activeMembers []activeMember
	rows, err := db.Query(`
		SELECT user_id::text, is_owner, email, display_name
		FROM active_group_memberships
		WHERE group_id = $1
		ORDER BY user_id
	`, groupOneID)
	if err != nil {
		t.Fatalf("query active memberships: %v", err)
	}
	for rows.Next() {
		var member activeMember
		if err := rows.Scan(
			&member.userID,
			&member.isOwner,
			&member.email,
			&member.displayName,
		); err != nil {
			rows.Close()
			t.Fatalf("scan active membership: %v", err)
		}
		activeMembers = append(activeMembers, member)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close active memberships: %v", err)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate active memberships: %v", err)
	}
	wantMembers := []activeMember{
		{userID: aliceID, isOwner: true, email: "alice@example.com", displayName: "Alice"},
		{userID: bobID, email: "bob@example.com", displayName: "Bob"},
		{userID: carolID, email: "carol@example.com", displayName: "Carol"},
	}
	if !reflect.DeepEqual(activeMembers, wantMembers) {
		t.Errorf("active members = %#v, want %#v", activeMembers, wantMembers)
	}

	assertIDs(
		t,
		db,
		`SELECT id::text FROM active_expenses ORDER BY id`,
		[]string{expenseOneID, expenseTwoID},
	)
	assertIDs(
		t,
		db,
		`SELECT id::text FROM active_repayments ORDER BY id`,
		[]string{repaymentOneID},
	)

	type debt struct {
		from       string
		to         string
		amount     int64
		currency   string
		occurredOn string
	}
	var debtEntries []debt
	rows, err = db.Query(`
		SELECT
			from_user_id::text,
			to_user_id::text,
			amount_cents,
			currency::text,
			occurred_on::text
		FROM settlement_debt_entries
		WHERE group_id = $1
		ORDER BY from_user_id, to_user_id, amount_cents
	`, groupOneID)
	if err != nil {
		t.Fatalf("query debt entries: %v", err)
	}
	for rows.Next() {
		var entry debt
		if err := rows.Scan(
			&entry.from,
			&entry.to,
			&entry.amount,
			&entry.currency,
			&entry.occurredOn,
		); err != nil {
			rows.Close()
			t.Fatalf("scan debt entry: %v", err)
		}
		debtEntries = append(debtEntries, entry)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close debt entries: %v", err)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate debt entries: %v", err)
	}
	wantDebtEntries := []debt{
		{from: aliceID, to: bobID, amount: 1000, currency: "USD", occurredOn: "2026-06-30"},
		{from: aliceID, to: bobID, amount: 1200, currency: "USD", occurredOn: "2026-06-30"},
		{from: bobID, to: aliceID, amount: 3000, currency: "USD", occurredOn: "2026-06-30"},
		{from: carolID, to: aliceID, amount: 3000, currency: "USD", occurredOn: "2026-06-30"},
	}
	if !reflect.DeepEqual(debtEntries, wantDebtEntries) {
		t.Errorf("debt entries = %#v, want %#v", debtEntries, wantDebtEntries)
	}

	wantGross := []balance{
		{from: aliceID, to: bobID, amount: 2200, currency: "USD"},
		{from: bobID, to: aliceID, amount: 3000, currency: "USD"},
		{from: carolID, to: aliceID, amount: 3000, currency: "USD"},
	}
	if got := queryBalances(t, db, "pairwise_gross_balances", groupOneID); !reflect.DeepEqual(got, wantGross) {
		t.Errorf("gross balances = %#v, want %#v", got, wantGross)
	}

	wantNet := []balance{
		{from: bobID, to: aliceID, amount: 800, currency: "USD"},
		{from: carolID, to: aliceID, amount: 3000, currency: "USD"},
	}
	if got := queryBalances(t, db, "pairwise_net_balances", groupOneID); !reflect.DeepEqual(got, wantNet) {
		t.Errorf("net balances = %#v, want %#v", got, wantNet)
	}

	for _, view := range []string{
		"active_group_memberships",
		"active_expenses",
		"active_repayments",
		"settlement_debt_entries",
		"pairwise_gross_balances",
		"pairwise_net_balances",
	} {
		var count int
		query := fmt.Sprintf("SELECT count(*) FROM %s WHERE group_id = $1", view)
		if err := db.QueryRow(query, groupTwoID).Scan(&count); err != nil {
			t.Fatalf("count dissolved group rows in %s: %v", view, err)
		}
		if count != 0 {
			t.Errorf("%s dissolved group row count = %d, want zero", view, count)
		}
	}
}

func TestPairwiseNetOmitsEqualOpposingDebt(t *testing.T) {
	db := openIntegrationDatabase(t)
	resetIntegrationDatabase(t, db)

	insertUser(t, db, aliceID, "alice@example.com", "Alice")
	insertUser(t, db, bobID, "bob@example.com", "Bob")
	insertGroup(t, db, groupOneID, aliceID, "EQUALDEBT123")
	insertMembership(t, db, groupOneID, aliceID, nil)
	insertMembership(t, db, groupOneID, bobID, nil)

	if _, err := insertExpense(
		db,
		expenseOneID,
		groupOneID,
		aliceID,
		aliceID,
		1000,
		"USD",
		nil,
	); err != nil {
		t.Fatalf("insert Alice expense: %v", err)
	}
	insertSplit(t, db, expenseOneID, groupOneID, bobID, 1000)

	if _, err := insertExpense(
		db,
		expenseTwoID,
		groupOneID,
		bobID,
		bobID,
		1000,
		"USD",
		nil,
	); err != nil {
		t.Fatalf("insert Bob expense: %v", err)
	}
	insertSplit(t, db, expenseTwoID, groupOneID, aliceID, 1000)

	var grossCount int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM pairwise_gross_balances
		WHERE group_id = $1
	`, groupOneID).Scan(&grossCount); err != nil {
		t.Fatalf("count gross balances: %v", err)
	}
	if grossCount != 2 {
		t.Errorf("gross balance count = %d, want two opposing rows", grossCount)
	}

	var netCount int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM pairwise_net_balances
		WHERE group_id = $1
	`, groupOneID).Scan(&netCount); err != nil {
		t.Fatalf("count net balances: %v", err)
	}
	if netCount != 0 {
		t.Errorf("net balance count = %d, want zero", netCount)
	}
}

func TestStoreWithinTx(t *testing.T) {
	db := openIntegrationDatabase(t)
	resetIntegrationDatabase(t, db)
	store := New(db)

	err := store.withinTx(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO users (id, email, password_hash, display_name)
			VALUES ($1, 'committed@example.com', 'hash', 'Committed')
		`, aliceID)
		return err
	})
	if err != nil {
		t.Fatalf("commit transaction: %v", err)
	}
	assertUserExists(t, db, aliceID, true)

	callbackError := errors.New("stop transaction")
	err = store.withinTx(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`
			INSERT INTO users (id, email, password_hash, display_name)
			VALUES ($1, 'rolled-back@example.com', 'hash', 'Rolled Back')
		`, bobID); err != nil {
			return err
		}
		return callbackError
	})
	if !errors.Is(err, callbackError) {
		t.Fatalf("rollback transaction error = %v, want callback error", err)
	}
	assertUserExists(t, db, bobID, false)
}

type balance struct {
	from     string
	to       string
	amount   int64
	currency string
}

type recordTimes struct {
	createdAt time.Time
	deletedAt *time.Time
}

func openIntegrationDatabase(t *testing.T) *sql.DB {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if strings.TrimSpace(databaseURL) == "" {
		t.Fatal("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open integration database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close integration database: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping integration database: %v", err)
	}

	var databaseName string
	if err := db.QueryRowContext(ctx, `SELECT current_database()`).Scan(&databaseName); err != nil {
		t.Fatalf("read integration database name: %v", err)
	}
	if databaseName != "settled_test" {
		t.Fatalf(
			"integration tests require the dedicated settled_test database, got %q",
			databaseName,
		)
	}
	return db
}

func resetIntegrationDatabase(t *testing.T, db *sql.DB) {
	t.Helper()

	_, err := db.Exec(`
		TRUNCATE TABLE
			expense_splits,
			repayments,
			expenses,
			group_memberships,
			groups,
			users
	`)
	if err != nil {
		t.Fatalf("truncate integration tables: %v", err)
	}
}

func seedUsersAndGroups(t *testing.T, db *sql.DB) {
	t.Helper()

	insertUser(t, db, aliceID, "alice@example.com", "Alice")
	insertUser(t, db, bobID, "bob@example.com", "Bob")
	insertUser(t, db, carolID, "carol@example.com", "Carol")
	insertUser(t, db, daveID, "dave@example.com", "Dave")
	insertUser(t, db, erinID, "erin@example.com", "Erin")

	insertGroup(t, db, groupOneID, aliceID, "GROUPONE1234")
	insertGroup(t, db, groupTwoID, daveID, "GROUPTWO1234")

	insertMembership(t, db, groupOneID, aliceID, nil)
	insertMembership(t, db, groupOneID, bobID, nil)
	insertMembership(t, db, groupOneID, carolID, nil)
	insertMembership(t, db, groupTwoID, daveID, nil)
}

func seedSettlementFixture(t *testing.T, db *sql.DB) {
	t.Helper()

	seedUsersAndGroups(t, db)
	joinedAt := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	removedAt := joinedAt.Add(time.Hour)
	insertMembership(t, db, groupOneID, erinID, &removedAt)
	insertMembership(t, db, groupTwoID, aliceID, nil)
	insertMembership(t, db, groupTwoID, bobID, nil)

	createdAt := time.Date(2026, 6, 30, 18, 0, 0, 0, time.UTC)
	if _, err := insertExpense(
		db,
		expenseOneID,
		groupOneID,
		aliceID,
		aliceID,
		9000,
		"USD",
		&recordTimes{createdAt: createdAt},
	); err != nil {
		t.Fatalf("insert Alice fixture expense: %v", err)
	}
	insertSplit(t, db, expenseOneID, groupOneID, aliceID, 3000)
	insertSplit(t, db, expenseOneID, groupOneID, bobID, 3000)
	insertSplit(t, db, expenseOneID, groupOneID, carolID, 3000)

	if _, err := insertExpense(
		db,
		expenseTwoID,
		groupOneID,
		bobID,
		bobID,
		2400,
		"USD",
		&recordTimes{createdAt: createdAt.Add(time.Minute)},
	); err != nil {
		t.Fatalf("insert Bob fixture expense: %v", err)
	}
	insertSplit(t, db, expenseTwoID, groupOneID, aliceID, 1200)
	insertSplit(t, db, expenseTwoID, groupOneID, bobID, 1200)

	deletedAt := createdAt.Add(2 * time.Minute)
	if _, err := insertExpense(
		db,
		expenseThreeID,
		groupOneID,
		aliceID,
		aliceID,
		500,
		"USD",
		&recordTimes{
			createdAt: createdAt,
			deletedAt: &deletedAt,
		},
	); err != nil {
		t.Fatalf("insert deleted fixture expense: %v", err)
	}
	insertSplit(t, db, expenseThreeID, groupOneID, bobID, 500)

	if _, err := insertExpense(
		db,
		expenseFourID,
		groupTwoID,
		daveID,
		daveID,
		2000,
		"USD",
		&recordTimes{createdAt: createdAt},
	); err != nil {
		t.Fatalf("insert second-group fixture expense: %v", err)
	}
	insertSplit(t, db, expenseFourID, groupTwoID, aliceID, 1000)
	insertSplit(t, db, expenseFourID, groupTwoID, daveID, 1000)

	if _, err := insertRepayment(
		db,
		repaymentOneID,
		groupOneID,
		bobID,
		aliceID,
		bobID,
		1000,
		"USD",
		&recordTimes{createdAt: createdAt},
	); err != nil {
		t.Fatalf("insert fixture repayment: %v", err)
	}
	if _, err := insertRepayment(
		db,
		repaymentTwoID,
		groupOneID,
		aliceID,
		bobID,
		aliceID,
		50,
		"USD",
		&recordTimes{
			createdAt: createdAt,
			deletedAt: &deletedAt,
		},
	); err != nil {
		t.Fatalf("insert deleted fixture repayment: %v", err)
	}
	if _, err := insertRepayment(
		db,
		repaymentThreeID,
		groupTwoID,
		aliceID,
		daveID,
		aliceID,
		100,
		"USD",
		&recordTimes{createdAt: createdAt},
	); err != nil {
		t.Fatalf("insert second-group fixture repayment: %v", err)
	}

	if _, err := db.Exec(`
		UPDATE groups
		SET dissolved_at = $2, updated_at = $2
		WHERE id = $1
	`, groupTwoID, createdAt.Add(3*time.Minute)); err != nil {
		t.Fatalf("dissolve second fixture group: %v", err)
	}
}

func insertUser(
	t *testing.T,
	db *sql.DB,
	id string,
	email string,
	displayName string,
) {
	t.Helper()

	if _, err := db.Exec(`
		INSERT INTO users (id, email, password_hash, display_name)
		VALUES ($1, $2, 'hash', $3)
	`, id, email, displayName); err != nil {
		t.Fatalf("insert user %s: %v", id, err)
	}
}

func insertGroup(
	t *testing.T,
	db *sql.DB,
	id string,
	ownerID string,
	joinCode string,
) {
	t.Helper()

	if _, err := db.Exec(`
		INSERT INTO groups (id, name, join_code, owner_user_id)
		VALUES ($1, 'Test group', $2, $3)
	`, id, joinCode, ownerID); err != nil {
		t.Fatalf("insert group %s: %v", id, err)
	}
}

func insertMembership(
	t *testing.T,
	db *sql.DB,
	groupID string,
	userID string,
	removedAt *time.Time,
) {
	t.Helper()

	joinedAt := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	if _, err := db.Exec(`
		INSERT INTO group_memberships (group_id, user_id, joined_at, removed_at)
		VALUES ($1, $2, $3, $4)
	`, groupID, userID, joinedAt, removedAt); err != nil {
		t.Fatalf("insert membership for %s: %v", userID, err)
	}
}

func insertExpense(
	db *sql.DB,
	id string,
	groupID string,
	paidByID string,
	createdByID string,
	amount int64,
	currency string,
	times *recordTimes,
) (sql.Result, error) {
	createdAt, deletedAt := resolvedTimes(times)
	return db.Exec(`
		INSERT INTO expenses (
			id,
			group_id,
			paid_by_user_id,
			description,
			amount_cents,
			currency,
			expense_date,
			created_by_user_id,
			created_at,
			updated_at,
			deleted_at
		)
		VALUES ($1, $2, $3, 'Test expense', $4, $5, '2026-06-30', $6, $7, $7, $8)
	`, id, groupID, paidByID, amount, currency, createdByID, createdAt, deletedAt)
}

func insertSplit(
	t *testing.T,
	db *sql.DB,
	expenseID string,
	groupID string,
	userID string,
	amount int64,
) {
	t.Helper()

	if _, err := db.Exec(`
		INSERT INTO expense_splits (expense_id, group_id, user_id, amount_cents)
		VALUES ($1, $2, $3, $4)
	`, expenseID, groupID, userID, amount); err != nil {
		t.Fatalf("insert expense split: %v", err)
	}
}

func insertRepayment(
	db *sql.DB,
	id string,
	groupID string,
	fromID string,
	toID string,
	createdByID string,
	amount int64,
	currency string,
	times *recordTimes,
) (sql.Result, error) {
	createdAt, deletedAt := resolvedTimes(times)
	return db.Exec(`
		INSERT INTO repayments (
			id,
			group_id,
			from_user_id,
			to_user_id,
			amount_cents,
			currency,
			note,
			repayment_date,
			created_by_user_id,
			created_at,
			updated_at,
			deleted_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'Test repayment', '2026-06-30', $7, $8, $8, $9)
	`, id, groupID, fromID, toID, amount, currency, createdByID, createdAt, deletedAt)
}

func resolvedTimes(times *recordTimes) (time.Time, *time.Time) {
	if times == nil {
		return time.Date(2026, 6, 30, 18, 0, 0, 0, time.UTC), nil
	}
	return times.createdAt, times.deletedAt
}

func requireConstraint(t *testing.T, err error, wantConstraint string) {
	t.Helper()

	if err == nil {
		t.Fatalf("operation succeeded, want constraint %s", wantConstraint)
	}
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		t.Fatalf("error type = %T, want *pgconn.PgError: %v", err, err)
	}
	if postgresError.ConstraintName != wantConstraint {
		t.Fatalf(
			"constraint = %q, want %q: %v",
			postgresError.ConstraintName,
			wantConstraint,
			err,
		)
	}
}

func assertCatalogNames(
	t *testing.T,
	db *sql.DB,
	query string,
	wantNames []string,
) {
	t.Helper()

	rows, err := db.Query(query)
	if err != nil {
		t.Fatalf("query catalog names: %v", err)
	}
	defer rows.Close()

	got := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan catalog name: %v", err)
		}
		got[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate catalog names: %v", err)
	}
	for _, name := range wantNames {
		if _, exists := got[name]; !exists {
			t.Errorf("catalog object %q is missing", name)
		}
	}
}

func assertIDs(
	t *testing.T,
	db *sql.DB,
	query string,
	want []string,
) {
	t.Helper()

	rows, err := db.Query(query)
	if err != nil {
		t.Fatalf("query IDs: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan ID: %v", err)
		}
		got = append(got, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate IDs: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("IDs = %v, want %v", got, want)
	}
}

func queryBalances(
	t *testing.T,
	db *sql.DB,
	view string,
	groupID string,
) []balance {
	t.Helper()

	query := fmt.Sprintf(`
		SELECT from_user_id::text, to_user_id::text, amount_cents, currency::text
		FROM %s
		WHERE group_id = $1
	`, view)
	rows, err := db.Query(query, groupID)
	if err != nil {
		t.Fatalf("query %s: %v", view, err)
	}
	defer rows.Close()

	var balances []balance
	for rows.Next() {
		var value balance
		if err := rows.Scan(
			&value.from,
			&value.to,
			&value.amount,
			&value.currency,
		); err != nil {
			t.Fatalf("scan %s: %v", view, err)
		}
		balances = append(balances, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s: %v", view, err)
	}
	sort.Slice(balances, func(i, j int) bool {
		if balances[i].from != balances[j].from {
			return balances[i].from < balances[j].from
		}
		if balances[i].to != balances[j].to {
			return balances[i].to < balances[j].to
		}
		return balances[i].amount < balances[j].amount
	})
	return balances
}

func assertUserExists(
	t *testing.T,
	db *sql.DB,
	userID string,
	want bool,
) {
	t.Helper()

	var exists bool
	if err := db.QueryRow(`
		SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)
	`, userID).Scan(&exists); err != nil {
		t.Fatalf("check user existence: %v", err)
	}
	if exists != want {
		t.Errorf("user %s exists = %v, want %v", userID, exists, want)
	}
}
