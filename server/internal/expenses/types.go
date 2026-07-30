package expenses

import (
	"context"
	"errors"
	"time"
)

// SplitMode identifies the requested split calculation.
type SplitMode string

const (
	SplitModeEqual      SplitMode = "equal"
	SplitModeExact      SplitMode = "exact"
	SplitModePercentage SplitMode = "percentage"
)

// SplitInput contains exactly one split variant selected by Mode.
type SplitInput struct {
	Mode               SplitMode
	ParticipantUserIDs []string
	Splits             []ExactSplitInput
	PercentageSplits   []PercentageSplitInput
}

// ExactSplitInput assigns an exact cent amount to one participant.
type ExactSplitInput struct {
	UserID      string
	AmountCents int64
}

// PercentageSplitInput assigns positive basis points to one participant.
type PercentageSplitInput struct {
	UserID                string
	PercentageBasisPoints int64
}

// Split is the exact positive-cent persistence representation.
type Split struct {
	UserID      string
	AmountCents int64
}

var ErrNotFound = errors.New("expense not found")

type ValidationError struct {
	Fields map[string]string
}

func (validation *ValidationError) Error() string {
	return "validation failed"
}

type MutationInput struct {
	PaidByUserID string
	Description  string
	AmountCents  int64
	ExpenseDate  string
	SplitInput   SplitInput
}

type Expense struct {
	ID              string
	GroupID         string
	PaidByUserID    string
	Description     string
	AmountCents     int64
	Currency        string
	ExpenseDate     time.Time
	CreatedByUserID string
	Splits          []Split
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type MemberSummary struct {
	UserID      string
	DisplayName string
}

type ListResult struct {
	Expenses []Expense
	Members  []MemberSummary
}

type CreateInput struct {
	ID           string
	ActorID      string
	GroupID      string
	PaidByUserID string
	Description  string
	AmountCents  int64
	Currency     string
	ExpenseDate  time.Time
	Splits       []Split
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ReplaceInput struct {
	ActorID      string
	GroupID      string
	ExpenseID    string
	PaidByUserID string
	Description  string
	AmountCents  int64
	Currency     string
	ExpenseDate  time.Time
	Splits       []Split
	UpdatedAt    time.Time
}

type DeleteInput struct {
	ActorID   string
	GroupID   string
	ExpenseID string
	DeletedAt time.Time
}

type Store interface {
	CreateExpense(context.Context, CreateInput) (Expense, error)
	ReplaceExpense(context.Context, ReplaceInput) (Expense, error)
	DeleteExpense(context.Context, DeleteInput) error
	GetExpense(context.Context, string, string, string) (Expense, error)
	ListExpenses(context.Context, string, string) (ListResult, error)
}
