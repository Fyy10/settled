package groups

import (
	"context"
	"errors"
	"time"
)

const (
	RoleOwner  = "owner"
	RoleMember = "member"
)

var (
	ErrNotFound          = errors.New("group not found")
	ErrJoinCodeCollision = errors.New("join code collision")
)

type Group struct {
	ID              string
	Name            string
	OwnerUserID     string
	MemberCount     int64
	CurrentUserRole string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Member struct {
	UserID      string
	Email       string
	DisplayName string
	Role        string
	JoinedAt    time.Time
}

type Detail struct {
	Group   Group
	Members []Member
}

type NewGroup struct {
	ID          string
	Name        string
	JoinCode    string
	OwnerUserID string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type JoinGroupInput struct {
	UserID   string
	JoinCode string
	JoinedAt time.Time
}

type Store interface {
	CreateGroup(context.Context, NewGroup) (Group, error)
	ListGroups(context.Context, string) ([]Group, error)
	JoinGroup(context.Context, JoinGroupInput) (Group, error)
	GetGroup(context.Context, string, string) (Detail, error)
}
