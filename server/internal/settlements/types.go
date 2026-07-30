package settlements

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("settlements not found")

type DebtEntry struct {
	FromUserID  string
	ToUserID    string
	AmountCents int64
	Currency    string
}

type Transfer struct {
	FromUserID  string
	ToUserID    string
	AmountCents int64
	Currency    string
}

type MemberSummary struct {
	UserID      string
	DisplayName string
}

type Result struct {
	Transfers []Transfer
	Members   []MemberSummary
}

type Calculator interface {
	Calculate([]DebtEntry) ([]Transfer, error)
}

type Store interface {
	ListDebtEntries(
		context.Context,
		string,
		string,
	) ([]DebtEntry, []MemberSummary, error)
}
