//go:build integration

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/Fyy10/settled/server/internal/auth"
	"github.com/Fyy10/settled/server/internal/expenses"
	"github.com/Fyy10/settled/server/internal/groups"
	"github.com/Fyy10/settled/server/internal/httpapi"
	"github.com/Fyy10/settled/server/internal/repayments"
	"github.com/Fyy10/settled/server/internal/settlements"
)

func TestSettlementStoreLayeredDebtPipeline(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := settlementStoreTestFixture(t, db)
	store := New(db)
	settlementStoreTestSeedLayeredActivity(t, store, baseTime)

	entries, members, err := store.ListDebtEntries(
		context.Background(),
		carolID,
		groupOneID,
	)
	if err != nil {
		t.Fatalf("ListDebtEntries: %v", err)
	}
	wantEntries := settlementStoreTestLayeredEntries()
	if !reflect.DeepEqual(entries, wantEntries) {
		t.Errorf("debt entries = %#v, want %#v", entries, wantEntries)
	}
	if !reflect.DeepEqual(
		entries,
		settlementStoreTestQueryDebtEntries(t, db, groupOneID),
	) {
		t.Error("Store debt entries differ from settlement_debt_entries")
	}
	if len(entries) != 4 ||
		entries[2].FromUserID != aliceID ||
		entries[3].FromUserID != aliceID ||
		entries[2].ToUserID != bobID ||
		entries[3].ToUserID != bobID {
		t.Errorf(
			"raw duplicate direction was not preserved: %#v",
			entries,
		)
	}
	wantMembers := []settlements.MemberSummary{
		{UserID: aliceID, DisplayName: "Alice"},
		{UserID: bobID, DisplayName: "Bob"},
		{UserID: carolID, DisplayName: "Carol"},
	}
	if !reflect.DeepEqual(members, wantMembers) {
		t.Errorf("members = %#v, want %#v", members, wantMembers)
	}

	gross := queryBalances(
		t,
		db,
		"pairwise_gross_balances",
		groupOneID,
	)
	wantGross := []balance{
		{from: aliceID, to: bobID, amount: 2200, currency: "USD"},
		{from: bobID, to: aliceID, amount: 3000, currency: "USD"},
		{from: carolID, to: aliceID, amount: 3000, currency: "USD"},
	}
	if !reflect.DeepEqual(gross, wantGross) {
		t.Errorf("gross balances = %#v, want %#v", gross, wantGross)
	}
	if !reflect.DeepEqual(
		settlementStoreTestAggregateEntries(entries),
		settlementStoreTestBalanceMap(gross),
	) {
		t.Errorf(
			"independent entry aggregation = %#v, gross view = %#v",
			settlementStoreTestAggregateEntries(entries),
			settlementStoreTestBalanceMap(gross),
		)
	}

	net := queryBalances(
		t,
		db,
		"pairwise_net_balances",
		groupOneID,
	)
	wantNet := []balance{
		{from: bobID, to: aliceID, amount: 800, currency: "USD"},
		{from: carolID, to: aliceID, amount: 3000, currency: "USD"},
	}
	if !reflect.DeepEqual(net, wantNet) {
		t.Errorf("net balances = %#v, want %#v", net, wantNet)
	}
	settlementStoreTestAssertNetMatchesGross(t, gross, net)

	calculator := settlements.PairwiseCalculator{}
	transfers, err := calculator.Calculate(entries)
	if err != nil {
		t.Fatalf("PairwiseCalculator.Calculate: %v", err)
	}
	wantTransfers := []settlements.Transfer{
		{
			FromUserID:  carolID,
			ToUserID:    aliceID,
			AmountCents: 3000,
			Currency:    "USD",
		},
		{
			FromUserID:  bobID,
			ToUserID:    aliceID,
			AmountCents: 800,
			Currency:    "USD",
		},
	}
	if !reflect.DeepEqual(transfers, wantTransfers) {
		t.Errorf("calculator transfers = %#v, want %#v", transfers, wantTransfers)
	}
	if !reflect.DeepEqual(
		transfers,
		settlementStoreTestTransfers(net),
	) {
		t.Errorf(
			"calculator transfers = %#v, SQL net transfers = %#v",
			transfers,
			settlementStoreTestTransfers(net),
		)
	}

	service, err := settlements.NewService(store, calculator)
	if err != nil {
		t.Fatalf("settlements.NewService: %v", err)
	}
	result, err := service.List(
		context.Background(),
		bobID,
		groupOneID,
	)
	if err != nil {
		t.Fatalf("settlement Service.List: %v", err)
	}
	if !reflect.DeepEqual(result.Transfers, wantTransfers) ||
		!reflect.DeepEqual(result.Members, wantMembers) {
		t.Errorf("service result = %#v", result)
	}

	handler := settlementStoreTestHTTPHandler(t, db, service)
	response := settlementStoreTestCallEndpoint(
		t,
		handler,
		bobID,
		groupOneID,
	)
	if response.Code != http.StatusOK {
		t.Fatalf(
			"settlement endpoint = %d %s",
			response.Code,
			response.Body.String(),
		)
	}
	var body settlementStoreTestHTTPResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode settlement endpoint response: %v", err)
	}
	wantBody := settlementStoreTestHTTPResponse{
		Settlements: []settlementStoreTestHTTPTransfer{
			{
				FromUserID:  carolID,
				ToUserID:    aliceID,
				AmountCents: 3000,
				Currency:    "USD",
			},
			{
				FromUserID:  bobID,
				ToUserID:    aliceID,
				AmountCents: 800,
				Currency:    "USD",
			},
		},
		Members: []settlementStoreTestHTTPMember{
			{UserID: aliceID, DisplayName: "Alice"},
			{UserID: bobID, DisplayName: "Bob"},
			{UserID: carolID, DisplayName: "Carol"},
		},
	}
	if !reflect.DeepEqual(body, wantBody) {
		t.Errorf("settlement endpoint body = %#v, want %#v", body, wantBody)
	}

	for name, actorID := range map[string]string{
		"nonmember":   frankID,
		"other group": daveID,
	} {
		t.Run("endpoint hides "+name, func(t *testing.T) {
			hidden := settlementStoreTestCallEndpoint(
				t,
				handler,
				actorID,
				groupOneID,
			)
			settlementStoreTestAssertNotFoundResponse(t, hidden)
		})
	}
}

func TestSettlementStoreVisibilityEmptyArraysAndMemberOrdering(t *testing.T) {
	db := openIntegrationDatabase(t)
	settlementStoreTestFixture(t, db)
	store := New(db)

	if _, err := db.Exec(`
		UPDATE users
		SET display_name = CASE id
			WHEN $1 THEN 'Zulu Owner'
			WHEN $2 THEN 'alice'
			WHEN $3 THEN 'Alice'
			ELSE display_name
		END
		WHERE id IN ($1, $2, $3)
	`, aliceID, bobID, carolID); err != nil {
		t.Fatalf("update fixture display names: %v", err)
	}

	entries, members, err := store.ListDebtEntries(
		context.Background(),
		bobID,
		groupOneID,
	)
	if err != nil {
		t.Fatalf("ListDebtEntries empty group: %v", err)
	}
	if entries == nil || len(entries) != 0 {
		t.Errorf("empty entries = %#v, want non-nil empty", entries)
	}
	wantMembers := []settlements.MemberSummary{
		{UserID: aliceID, DisplayName: "Zulu Owner"},
		{UserID: bobID, DisplayName: "alice"},
		{UserID: carolID, DisplayName: "Alice"},
	}
	if !reflect.DeepEqual(members, wantMembers) {
		t.Errorf("members = %#v, want %#v", members, wantMembers)
	}

	otherEntries, otherMembers, err := store.ListDebtEntries(
		context.Background(),
		aliceID,
		groupTwoID,
	)
	if err != nil {
		t.Fatalf("ListDebtEntries second empty group: %v", err)
	}
	if otherEntries == nil || len(otherEntries) != 0 {
		t.Errorf(
			"second-group entries = %#v, want non-nil empty",
			otherEntries,
		)
	}
	wantOtherMembers := []settlements.MemberSummary{
		{UserID: daveID, DisplayName: "Dave"},
		{UserID: bobID, DisplayName: "alice"},
		{UserID: carolID, DisplayName: "Alice"},
		{UserID: aliceID, DisplayName: "Zulu Owner"},
	}
	if !reflect.DeepEqual(otherMembers, wantOtherMembers) {
		t.Errorf(
			"second-group members = %#v, want %#v",
			otherMembers,
			wantOtherMembers,
		)
	}

	tests := []struct {
		name    string
		actorID string
		groupID string
	}{
		{
			name:    "nonmember",
			actorID: frankID,
			groupID: groupOneID,
		},
		{
			name:    "removed member",
			actorID: erinID,
			groupID: groupOneID,
		},
		{
			name:    "missing group",
			actorID: aliceID,
			groupID: repaymentMissingGroupID,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotEntries, gotMembers, err := store.ListDebtEntries(
				context.Background(),
				test.actorID,
				test.groupID,
			)
			if !errors.Is(err, settlements.ErrNotFound) {
				t.Errorf("error = %v, want ErrNotFound", err)
			}
			if gotEntries != nil || gotMembers != nil {
				t.Errorf(
					"hidden result = (%#v, %#v), want nil slices",
					gotEntries,
					gotMembers,
				)
			}
		})
	}
}

func TestSettlementStoreDeleteExclusions(t *testing.T) {
	db := openIntegrationDatabase(t)
	store := New(db)

	t.Run("expense", func(t *testing.T) {
		baseTime := settlementStoreTestFixture(t, db)
		settlementStoreTestSeedLayeredActivity(t, store, baseTime)
		if err := store.DeleteExpense(
			context.Background(),
			expenses.DeleteInput{
				ActorID:   carolID,
				GroupID:   groupOneID,
				ExpenseID: expenseOneID,
				DeletedAt: baseTime.Add(5 * time.Hour),
			},
		); err != nil {
			t.Fatalf("DeleteExpense: %v", err)
		}

		entries, _, err := store.ListDebtEntries(
			context.Background(),
			aliceID,
			groupOneID,
		)
		if err != nil {
			t.Fatalf("ListDebtEntries: %v", err)
		}
		wantEntries := []settlements.DebtEntry{
			{
				FromUserID:  aliceID,
				ToUserID:    bobID,
				AmountCents: 1200,
				Currency:    "USD",
			},
			{
				FromUserID:  aliceID,
				ToUserID:    bobID,
				AmountCents: 1000,
				Currency:    "USD",
			},
		}
		if !reflect.DeepEqual(entries, wantEntries) {
			t.Errorf("entries after expense delete = %#v, want %#v", entries, wantEntries)
		}
		settlementStoreTestAssertAllViews(
			t,
			db,
			groupOneID,
			2,
			1,
			1,
		)
	})

	t.Run("repayment", func(t *testing.T) {
		baseTime := settlementStoreTestFixture(t, db)
		settlementStoreTestSeedLayeredActivity(t, store, baseTime)
		if err := store.DeleteRepayment(
			context.Background(),
			repayments.DeleteInput{
				ActorID:     carolID,
				GroupID:     groupOneID,
				RepaymentID: repaymentOneID,
				DeletedAt:   baseTime.Add(5 * time.Hour),
			},
		); err != nil {
			t.Fatalf("DeleteRepayment: %v", err)
		}

		entries, _, err := store.ListDebtEntries(
			context.Background(),
			aliceID,
			groupOneID,
		)
		if err != nil {
			t.Fatalf("ListDebtEntries: %v", err)
		}
		wantEntries := settlementStoreTestLayeredEntries()[:3]
		if !reflect.DeepEqual(entries, wantEntries) {
			t.Errorf(
				"entries after repayment delete = %#v, want %#v",
				entries,
				wantEntries,
			)
		}
		settlementStoreTestAssertAllViews(
			t,
			db,
			groupOneID,
			3,
			3,
			2,
		)
	})
}

func TestSettlementStoreDissolutionHidesAllLayers(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := settlementStoreTestFixture(t, db)
	store := New(db)
	settlementStoreTestSeedLayeredActivity(t, store, baseTime)

	if err := store.DissolveGroup(
		context.Background(),
		groups.DissolveGroupInput{
			ActorID:     aliceID,
			GroupID:     groupOneID,
			DissolvedAt: baseTime.Add(6 * time.Hour),
		},
	); err != nil {
		t.Fatalf("DissolveGroup: %v", err)
	}
	entries, members, err := store.ListDebtEntries(
		context.Background(),
		aliceID,
		groupOneID,
	)
	if !errors.Is(err, settlements.ErrNotFound) {
		t.Errorf("dissolved error = %v, want ErrNotFound", err)
	}
	if entries != nil || members != nil {
		t.Errorf(
			"dissolved result = (%#v, %#v), want nil slices",
			entries,
			members,
		)
	}
	settlementStoreTestAssertAllViews(t, db, groupOneID, 0, 0, 0)

	service, err := settlements.NewService(
		store,
		settlements.PairwiseCalculator{},
	)
	if err != nil {
		t.Fatalf("settlements.NewService: %v", err)
	}
	hidden := settlementStoreTestCallEndpoint(
		t,
		settlementStoreTestHTTPHandler(t, db, service),
		aliceID,
		groupOneID,
	)
	settlementStoreTestAssertNotFoundResponse(t, hidden)
}

func TestSettlementStoreCrossGroupIsolation(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := settlementStoreTestFixture(t, db)
	store := New(db)
	settlementStoreTestSeedLayeredActivity(t, store, baseTime)

	if _, err := store.CreateExpense(
		context.Background(),
		expenses.CreateInput{
			ID:           expenseThreeID,
			ActorID:      daveID,
			GroupID:      groupTwoID,
			PaidByUserID: aliceID,
			Description:  "Other group expense",
			AmountCents:  600,
			Currency:     "USD",
			ExpenseDate: time.Date(
				2026,
				time.July,
				29,
				0,
				0,
				0,
				0,
				time.UTC,
			),
			Splits: []expenses.Split{
				{UserID: aliceID, AmountCents: 300},
				{UserID: bobID, AmountCents: 300},
			},
			CreatedAt: baseTime.Add(30 * time.Minute),
			UpdatedAt: baseTime.Add(30 * time.Minute),
		},
	); err != nil {
		t.Fatalf("CreateExpense second group: %v", err)
	}
	if _, err := store.CreateRepayment(
		context.Background(),
		repayments.CreateInput{
			ID:          repaymentTwoID,
			ActorID:     daveID,
			GroupID:     groupTwoID,
			FromUserID:  bobID,
			ToUserID:    aliceID,
			AmountCents: 100,
			Currency:    "USD",
			RepaymentDate: time.Date(
				2026,
				time.July,
				29,
				0,
				0,
				0,
				0,
				time.UTC,
			),
			CreatedAt: baseTime.Add(45 * time.Minute),
			UpdatedAt: baseTime.Add(45 * time.Minute),
		},
	); err != nil {
		t.Fatalf("CreateRepayment second group: %v", err)
	}

	firstEntries, _, err := store.ListDebtEntries(
		context.Background(),
		aliceID,
		groupOneID,
	)
	if err != nil {
		t.Fatalf("ListDebtEntries first group: %v", err)
	}
	if !reflect.DeepEqual(
		firstEntries,
		settlementStoreTestLayeredEntries(),
	) {
		t.Errorf("first-group entries leaked: %#v", firstEntries)
	}

	secondEntries, secondMembers, err := store.ListDebtEntries(
		context.Background(),
		bobID,
		groupTwoID,
	)
	if err != nil {
		t.Fatalf("ListDebtEntries second group: %v", err)
	}
	wantSecondEntries := []settlements.DebtEntry{
		{
			FromUserID:  bobID,
			ToUserID:    aliceID,
			AmountCents: 300,
			Currency:    "USD",
		},
		{
			FromUserID:  aliceID,
			ToUserID:    bobID,
			AmountCents: 100,
			Currency:    "USD",
		},
	}
	if !reflect.DeepEqual(secondEntries, wantSecondEntries) {
		t.Errorf(
			"second-group entries = %#v, want %#v",
			secondEntries,
			wantSecondEntries,
		)
	}
	wantSecondMembers := []settlements.MemberSummary{
		{UserID: daveID, DisplayName: "Dave"},
		{UserID: aliceID, DisplayName: "Alice"},
		{UserID: bobID, DisplayName: "Bob"},
		{UserID: carolID, DisplayName: "Carol"},
	}
	if !reflect.DeepEqual(secondMembers, wantSecondMembers) {
		t.Errorf(
			"second-group members = %#v, want %#v",
			secondMembers,
			wantSecondMembers,
		)
	}
	settlementStoreTestAssertAllViews(t, db, groupOneID, 4, 3, 2)
	settlementStoreTestAssertAllViews(t, db, groupTwoID, 2, 2, 1)
}

func TestSettlementStoreEqualOpposingBalancesOmitNetTransfer(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := settlementStoreTestFixture(t, db)
	store := New(db)

	inputs := []expenses.CreateInput{
		{
			ID:           expenseOneID,
			ActorID:      aliceID,
			GroupID:      groupOneID,
			PaidByUserID: aliceID,
			Description:  "Bob owes Alice",
			AmountCents:  1000,
			Currency:     "USD",
			ExpenseDate: time.Date(
				2026,
				time.August,
				1,
				0,
				0,
				0,
				0,
				time.UTC,
			),
			Splits: []expenses.Split{
				{UserID: bobID, AmountCents: 1000},
			},
			CreatedAt: baseTime.Add(time.Hour),
			UpdatedAt: baseTime.Add(time.Hour),
		},
		{
			ID:           expenseTwoID,
			ActorID:      bobID,
			GroupID:      groupOneID,
			PaidByUserID: bobID,
			Description:  "Alice owes Bob",
			AmountCents:  1000,
			Currency:     "USD",
			ExpenseDate: time.Date(
				2026,
				time.August,
				2,
				0,
				0,
				0,
				0,
				time.UTC,
			),
			Splits: []expenses.Split{
				{UserID: aliceID, AmountCents: 1000},
			},
			CreatedAt: baseTime.Add(2 * time.Hour),
			UpdatedAt: baseTime.Add(2 * time.Hour),
		},
	}
	for _, input := range inputs {
		if _, err := store.CreateExpense(
			context.Background(),
			input,
		); err != nil {
			t.Fatalf("CreateExpense %s: %v", input.ID, err)
		}
	}

	entries, _, err := store.ListDebtEntries(
		context.Background(),
		carolID,
		groupOneID,
	)
	if err != nil {
		t.Fatalf("ListDebtEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("debt entry count = %d, want 2", len(entries))
	}
	gross := queryBalances(
		t,
		db,
		"pairwise_gross_balances",
		groupOneID,
	)
	if len(gross) != 2 {
		t.Fatalf("gross balance count = %d, want 2", len(gross))
	}
	net := queryBalances(
		t,
		db,
		"pairwise_net_balances",
		groupOneID,
	)
	if len(net) != 0 {
		t.Errorf("net balances = %#v, want empty", net)
	}
	transfers, err := (settlements.PairwiseCalculator{}).Calculate(entries)
	if err != nil {
		t.Fatalf("PairwiseCalculator.Calculate: %v", err)
	}
	if transfers == nil || len(transfers) != 0 {
		t.Errorf("transfers = %#v, want non-nil empty", transfers)
	}
}

func settlementStoreTestFixture(t *testing.T, db *sql.DB) time.Time {
	t.Helper()

	resetIntegrationDatabase(t, db)
	groupTestInsertUsers(t, db)
	baseTime := time.Date(2026, time.August, 3, 10, 0, 0, 0, time.UTC)
	groupTestInsertGroup(
		t,
		db,
		groupOneID,
		aliceID,
		"Settlement Group",
		"SETTLEGRP23",
		baseTime,
		nil,
	)
	groupTestInsertMembership(t, db, groupOneID, aliceID, baseTime, nil)
	groupTestInsertMembership(
		t,
		db,
		groupOneID,
		bobID,
		baseTime.Add(time.Minute),
		nil,
	)
	groupTestInsertMembership(
		t,
		db,
		groupOneID,
		carolID,
		baseTime.Add(2*time.Minute),
		nil,
	)
	removedAt := baseTime.Add(4 * time.Minute)
	groupTestInsertMembership(
		t,
		db,
		groupOneID,
		erinID,
		baseTime.Add(3*time.Minute),
		&removedAt,
	)

	groupTestInsertGroup(
		t,
		db,
		groupTwoID,
		daveID,
		"Other Settlement Group",
		"SETTLEGRP24",
		baseTime,
		nil,
	)
	groupTestInsertMembership(t, db, groupTwoID, daveID, baseTime, nil)
	groupTestInsertMembership(
		t,
		db,
		groupTwoID,
		aliceID,
		baseTime.Add(time.Minute),
		nil,
	)
	groupTestInsertMembership(
		t,
		db,
		groupTwoID,
		bobID,
		baseTime.Add(2*time.Minute),
		nil,
	)
	groupTestInsertMembership(
		t,
		db,
		groupTwoID,
		carolID,
		baseTime.Add(3*time.Minute),
		nil,
	)
	return baseTime
}

func settlementStoreTestSeedLayeredActivity(
	t *testing.T,
	store *Store,
	baseTime time.Time,
) {
	t.Helper()

	expenseInputs := []expenses.CreateInput{
		{
			ID:           expenseOneID,
			ActorID:      aliceID,
			GroupID:      groupOneID,
			PaidByUserID: aliceID,
			Description:  "Alice pays for everyone",
			AmountCents:  9000,
			Currency:     "USD",
			ExpenseDate: time.Date(
				2026,
				time.July,
				30,
				0,
				0,
				0,
				0,
				time.UTC,
			),
			Splits: []expenses.Split{
				{UserID: aliceID, AmountCents: 3000},
				{UserID: bobID, AmountCents: 3000},
				{UserID: carolID, AmountCents: 3000},
			},
			CreatedAt: baseTime.Add(time.Hour),
			UpdatedAt: baseTime.Add(time.Hour),
		},
		{
			ID:           expenseTwoID,
			ActorID:      bobID,
			GroupID:      groupOneID,
			PaidByUserID: bobID,
			Description:  "Bob pays for Alice",
			AmountCents:  2400,
			Currency:     "USD",
			ExpenseDate: time.Date(
				2026,
				time.July,
				31,
				0,
				0,
				0,
				0,
				time.UTC,
			),
			Splits: []expenses.Split{
				{UserID: aliceID, AmountCents: 1200},
				{UserID: bobID, AmountCents: 1200},
			},
			CreatedAt: baseTime.Add(2 * time.Hour),
			UpdatedAt: baseTime.Add(2 * time.Hour),
		},
	}
	for _, input := range expenseInputs {
		if _, err := store.CreateExpense(
			context.Background(),
			input,
		); err != nil {
			t.Fatalf("CreateExpense %s: %v", input.ID, err)
		}
	}

	if _, err := store.CreateRepayment(
		context.Background(),
		repayments.CreateInput{
			ID:          repaymentOneID,
			ActorID:     bobID,
			GroupID:     groupOneID,
			FromUserID:  bobID,
			ToUserID:    aliceID,
			AmountCents: 1000,
			Currency:    "USD",
			RepaymentDate: time.Date(
				2026,
				time.August,
				1,
				0,
				0,
				0,
				0,
				time.UTC,
			),
			CreatedAt: baseTime.Add(3 * time.Hour),
			UpdatedAt: baseTime.Add(3 * time.Hour),
		},
	); err != nil {
		t.Fatalf("CreateRepayment: %v", err)
	}
}

func settlementStoreTestLayeredEntries() []settlements.DebtEntry {
	return []settlements.DebtEntry{
		{
			FromUserID:  bobID,
			ToUserID:    aliceID,
			AmountCents: 3000,
			Currency:    "USD",
		},
		{
			FromUserID:  carolID,
			ToUserID:    aliceID,
			AmountCents: 3000,
			Currency:    "USD",
		},
		{
			FromUserID:  aliceID,
			ToUserID:    bobID,
			AmountCents: 1200,
			Currency:    "USD",
		},
		{
			FromUserID:  aliceID,
			ToUserID:    bobID,
			AmountCents: 1000,
			Currency:    "USD",
		},
	}
}

func settlementStoreTestQueryDebtEntries(
	t *testing.T,
	db *sql.DB,
	groupID string,
) []settlements.DebtEntry {
	t.Helper()

	rows, err := db.Query(`
		SELECT
			from_user_id::text,
			to_user_id::text,
			amount_cents,
			currency::text
		FROM settlement_debt_entries
		WHERE group_id = $1
		ORDER BY
			occurred_on,
			created_at,
			from_user_id,
			to_user_id,
			amount_cents,
			currency
	`, groupID)
	if err != nil {
		t.Fatalf("query settlement_debt_entries: %v", err)
	}
	defer rows.Close()

	result := make([]settlements.DebtEntry, 0)
	for rows.Next() {
		var entry settlements.DebtEntry
		if err := rows.Scan(
			&entry.FromUserID,
			&entry.ToUserID,
			&entry.AmountCents,
			&entry.Currency,
		); err != nil {
			t.Fatalf("scan settlement_debt_entries: %v", err)
		}
		result = append(result, entry)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate settlement_debt_entries: %v", err)
	}
	return result
}

type settlementStoreTestDirectedKey struct {
	fromUserID string
	toUserID   string
	currency   string
}

func settlementStoreTestAggregateEntries(
	entries []settlements.DebtEntry,
) map[settlementStoreTestDirectedKey]int64 {
	result := make(map[settlementStoreTestDirectedKey]int64)
	for _, entry := range entries {
		key := settlementStoreTestDirectedKey{
			fromUserID: entry.FromUserID,
			toUserID:   entry.ToUserID,
			currency:   entry.Currency,
		}
		result[key] += entry.AmountCents
	}
	return result
}

func settlementStoreTestBalanceMap(
	values []balance,
) map[settlementStoreTestDirectedKey]int64 {
	result := make(map[settlementStoreTestDirectedKey]int64)
	for _, value := range values {
		key := settlementStoreTestDirectedKey{
			fromUserID: value.from,
			toUserID:   value.to,
			currency:   value.currency,
		}
		result[key] = value.amount
	}
	return result
}

func settlementStoreTestAssertNetMatchesGross(
	t *testing.T,
	gross []balance,
	net []balance,
) {
	t.Helper()

	type pair struct {
		userAID  string
		userBID  string
		currency string
	}
	signed := make(map[pair]int64)
	for _, value := range gross {
		key := pair{
			userAID:  value.from,
			userBID:  value.to,
			currency: value.currency,
		}
		sign := int64(1)
		if key.userAID > key.userBID {
			key.userAID, key.userBID = key.userBID, key.userAID
			sign = -1
		}
		signed[key] += sign * value.amount
	}
	want := make(map[settlementStoreTestDirectedKey]int64)
	for key, amount := range signed {
		if amount == 0 {
			continue
		}
		fromUserID := key.userAID
		toUserID := key.userBID
		if amount < 0 {
			fromUserID, toUserID = toUserID, fromUserID
			amount = -amount
		}
		want[settlementStoreTestDirectedKey{
			fromUserID: fromUserID,
			toUserID:   toUserID,
			currency:   key.currency,
		}] = amount
	}
	if got := settlementStoreTestBalanceMap(net); !reflect.DeepEqual(got, want) {
		t.Errorf("net view = %#v, independently derived net = %#v", got, want)
	}
}

func settlementStoreTestTransfers(
	values []balance,
) []settlements.Transfer {
	result := make([]settlements.Transfer, len(values))
	for index, value := range values {
		result[index] = settlements.Transfer{
			FromUserID:  value.from,
			ToUserID:    value.to,
			AmountCents: value.amount,
			Currency:    value.currency,
		}
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].AmountCents != result[right].AmountCents {
			return result[left].AmountCents > result[right].AmountCents
		}
		if result[left].FromUserID != result[right].FromUserID {
			return result[left].FromUserID < result[right].FromUserID
		}
		if result[left].ToUserID != result[right].ToUserID {
			return result[left].ToUserID < result[right].ToUserID
		}
		return result[left].Currency < result[right].Currency
	})
	return result
}

func settlementStoreTestAssertAllViews(
	t *testing.T,
	db *sql.DB,
	groupID string,
	wantEntries int,
	wantGross int,
	wantNet int,
) {
	t.Helper()

	tests := []struct {
		view string
		want int
	}{
		{view: "settlement_debt_entries", want: wantEntries},
		{view: "pairwise_gross_balances", want: wantGross},
		{view: "pairwise_net_balances", want: wantNet},
	}
	for _, test := range tests {
		var count int
		query := "SELECT count(*) FROM " + test.view + " WHERE group_id = $1"
		if err := db.QueryRow(query, groupID).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", test.view, err)
		}
		if count != test.want {
			t.Errorf("%s count = %d, want %d", test.view, count, test.want)
		}
	}
}

type settlementStoreTestHTTPTransfer struct {
	FromUserID  string `json:"fromUserId"`
	ToUserID    string `json:"toUserId"`
	AmountCents int64  `json:"amountCents"`
	Currency    string `json:"currency"`
}

type settlementStoreTestHTTPMember struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
}

type settlementStoreTestHTTPResponse struct {
	Settlements []settlementStoreTestHTTPTransfer `json:"settlements"`
	Members     []settlementStoreTestHTTPMember   `json:"members"`
}

type settlementStoreTestAuthService struct {
	httpapi.AuthService
}

func (settlementStoreTestAuthService) FindUser(
	_ context.Context,
	userID string,
) (auth.User, error) {
	return auth.User{ID: userID}, nil
}

type settlementStoreTestGroupService struct {
	httpapi.GroupService
}

type settlementStoreTestExpenseService struct {
	httpapi.ExpenseService
}

type settlementStoreTestRepaymentService struct {
	httpapi.RepaymentService
}

type settlementStoreTestCSRFProtector struct {
	httpapi.CSRFProtector
}

type settlementStoreTestSessionValidator struct{}

func (settlementStoreTestSessionValidator) Validate(
	token string,
) (auth.Session, error) {
	return auth.Session{
		UserID: token,
		JWTID:  "settlement-store-integration",
	}, nil
}

func settlementStoreTestHTTPHandler(
	t *testing.T,
	db *sql.DB,
	service *settlements.Service,
) http.Handler {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	api, err := httpapi.New(db, logger, httpapi.Options{
		Auth:        settlementStoreTestAuthService{},
		Groups:      settlementStoreTestGroupService{},
		Expenses:    settlementStoreTestExpenseService{},
		Repayments:  settlementStoreTestRepaymentService{},
		Settlements: service,
		Sessions:    settlementStoreTestSessionValidator{},
		CSRF:        settlementStoreTestCSRFProtector{},
	})
	if err != nil {
		t.Fatalf("httpapi.New: %v", err)
	}
	return api.Handler()
}

func settlementStoreTestCallEndpoint(
	t *testing.T,
	handler http.Handler,
	actorID string,
	groupID string,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/groups/"+groupID+"/settlements",
		nil,
	)
	request.AddCookie(&http.Cookie{
		Name:  auth.SessionCookieName,
		Value: actorID,
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func settlementStoreTestAssertNotFoundResponse(
	t *testing.T,
	response *httptest.ResponseRecorder,
) {
	t.Helper()

	if response.Code != http.StatusNotFound {
		t.Fatalf(
			"hidden settlement endpoint = %d %s",
			response.Code,
			response.Body.String(),
		)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode hidden settlement endpoint response: %v", err)
	}
	if body.Error.Code != "not_found" {
		t.Errorf("hidden endpoint code = %q, want not_found", body.Error.Code)
	}
}
