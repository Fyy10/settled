# Mini Task 19: Group Workspace and Independent Panel States

## Goal

Establish the group-detail data boundary, navigation, and Balances/Activity/Members workspace.

## Prerequisites

Mini Tasks 8, 13, and 18. Real empty states are acceptable before expense and repayment UI exists.

## Implementation Scope

- Load group detail/members once in the group layout, then load expenses, repayments, and settlements in parallel for the overview.
- Allow only `balances`, `activity`, and `members` in `?view=`; normalize missing or invalid values to balances.
- Implement the responsive group header, desktop actions, mobile safe-area action bar, and horizontally scrollable tabs.
- Render Balances only from backend settlement rows and Members in API order.
- Merge loaded expenses and repayments for Activity, sort by occurrence date, creation time, and ID, then group by date.
- Give each workflow panel independent loading/error/retry behavior; handle `401`, hidden group, and ordinary partial failure differently.

## Exclusions

- Do not calculate authoritative settlement in the frontend or implement mutation forms or owner settings.

## Validation

- Run `pnpm check`, `pnpm test`, and `pnpm build`.
- Test query normalization, activity merge/sort, member lookup/initials, partial-panel failure, hidden groups, and the empty-settlement state.
- Manually inspect 320 px, 200% zoom, tab focus, long names, and single-panel retry.

## Completion Criteria

- One failed panel does not hide successfully loaded data, and every displayed value comes from the API or presentation-only derivation.
