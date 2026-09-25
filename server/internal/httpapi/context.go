package httpapi

import (
	"context"

	"github.com/Fyy10/settled/server/internal/auth"
)

func contextWithSession(ctx context.Context, session auth.Session) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, session)
}

func contextWithUser(ctx context.Context, user auth.User) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}
