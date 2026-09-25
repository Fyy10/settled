package store

import (
	"strings"
	"testing"
)

func TestSettlementDebtEntryQueryUsesOnlyNormalizedSourceRows(t *testing.T) {
	t.Parallel()

	if !strings.Contains(
		settlementDebtEntriesQuery,
		"FROM settlement_debt_entries",
	) {
		t.Errorf(
			"debt-entry query does not use normalized source view: %s",
			settlementDebtEntriesQuery,
		)
	}
	for _, forbidden := range []string{
		"pairwise_gross_balances",
		"pairwise_net_balances",
	} {
		if strings.Contains(settlementDebtEntriesQuery, forbidden) {
			t.Errorf(
				"production debt-entry query uses diagnostic view %q: %s",
				forbidden,
				settlementDebtEntriesQuery,
			)
		}
	}
	if !strings.Contains(settlementDebtEntriesQuery, "ORDER BY") ||
		!strings.Contains(settlementDebtEntriesQuery, "occurred_on") ||
		!strings.Contains(settlementDebtEntriesQuery, "created_at") {
		t.Errorf(
			"debt-entry query lacks deterministic source ordering: %s",
			settlementDebtEntriesQuery,
		)
	}
}

func TestSettlementMemberQueryUsesStableAPIOrder(t *testing.T) {
	t.Parallel()

	required := []string{
		"membership.user_id = active_group.owner_user_id",
		"lower(member_user.display_name)",
		"membership.user_id",
		"membership.removed_at IS NULL",
	}
	for _, fragment := range required {
		if !strings.Contains(settlementMemberSummariesQuery, fragment) {
			t.Errorf(
				"member-summary query lacks %q: %s",
				fragment,
				settlementMemberSummariesQuery,
			)
		}
	}
}
