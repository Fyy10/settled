package repayments

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("repayment not found")

type ValidationError struct {
	Fields map[string]string
}

func (validation *ValidationError) Error() string {
	return "validation failed"
}

type MutationInput struct {
	FromUserID    string
	ToUserID      string
	AmountCents   int64
	Note          *string
	NotePresent   bool
	RepaymentDate string
}

type Repayment struct {
	ID              string
	GroupID         string
	FromUserID      string
	ToUserID        string
	AmountCents     int64
	Currency        string
	Note            *string
	RepaymentDate   time.Time
	CreatedByUserID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type MemberSummary struct {
	UserID      string
	DisplayName string
}

type ListResult struct {
	Repayments []Repayment
	Members    []MemberSummary
}

type CreateInput struct {
	ID            string
	ActorID       string
	GroupID       string
	FromUserID    string
	ToUserID      string
	AmountCents   int64
	Currency      string
	Note          *string
	RepaymentDate time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ReplaceInput struct {
	ActorID       string
	GroupID       string
	RepaymentID   string
	FromUserID    string
	ToUserID      string
	AmountCents   int64
	Currency      string
	Note          *string
	RepaymentDate time.Time
	UpdatedAt     time.Time
}

type DeleteInput struct {
	ActorID     string
	GroupID     string
	RepaymentID string
	DeletedAt   time.Time
}

type Store interface {
	CreateRepayment(context.Context, CreateInput) (Repayment, error)
	ReplaceRepayment(context.Context, ReplaceInput) (Repayment, error)
	DeleteRepayment(context.Context, DeleteInput) error
	GetRepayment(context.Context, string, string, string) (Repayment, error)
	ListRepayments(context.Context, string, string) (ListResult, error)
}
