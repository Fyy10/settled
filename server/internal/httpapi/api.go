package httpapi

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/Fyy10/settled/server/internal/auth"
	"github.com/Fyy10/settled/server/internal/expenses"
	"github.com/Fyy10/settled/server/internal/groups"
	"github.com/Fyy10/settled/server/internal/repayments"
	"github.com/Fyy10/settled/server/internal/settlements"
)

type Pinger interface {
	PingContext(context.Context) error
}

type SessionValidator interface {
	Validate(string) (auth.Session, error)
}

type AuthService interface {
	Register(context.Context, auth.RegisterInput) (auth.AuthResult, error)
	Login(context.Context, string, string) (auth.AuthResult, error)
	FindUser(context.Context, string) (auth.User, error)
}

type GroupService interface {
	Create(context.Context, string, string) (groups.Group, error)
	List(context.Context, string) ([]groups.Group, error)
	Join(context.Context, string, string) (groups.Group, error)
	Get(context.Context, string, string) (groups.Detail, error)
	Rename(context.Context, string, string, string) (groups.Group, error)
	Dissolve(context.Context, string, string) error
	GetJoinCode(context.Context, string, string) (string, error)
	RemoveMember(context.Context, string, string, string) error
}

type ExpenseService interface {
	List(context.Context, string, string) (expenses.ListResult, error)
	Create(
		context.Context,
		string,
		string,
		expenses.MutationInput,
	) (expenses.Expense, error)
	Get(context.Context, string, string, string) (expenses.Expense, error)
	Replace(
		context.Context,
		string,
		string,
		string,
		expenses.MutationInput,
	) (expenses.Expense, error)
	Delete(context.Context, string, string, string) error
}

type RepaymentService interface {
	List(context.Context, string, string) (repayments.ListResult, error)
	Create(
		context.Context,
		string,
		string,
		repayments.MutationInput,
	) (repayments.Repayment, error)
	Get(context.Context, string, string, string) (repayments.Repayment, error)
	Replace(
		context.Context,
		string,
		string,
		string,
		repayments.MutationInput,
	) (repayments.Repayment, error)
	Delete(context.Context, string, string, string) error
}

type SettlementService interface {
	List(context.Context, string, string) (settlements.Result, error)
}

type CSRFProtector interface {
	IssueOrReuse(string, *auth.Session) (auth.CSRFBinding, error)
	RotateAnonymous() (auth.CSRFBinding, error)
	RotateAuthenticated(auth.Session) (auth.CSRFBinding, error)
	ValidateAnonymous(string, string) error
	ValidateAuthenticated(string, string, auth.Session) error
}

type Options struct {
	AllowedOrigins []string
	Auth           AuthService
	Groups         GroupService
	Expenses       ExpenseService
	Repayments     RepaymentService
	Settlements    SettlementService
	Sessions       SessionValidator
	CSRF           CSRFProtector
	SessionCookies auth.SessionCookies
	CSRFCookies    auth.CSRFCookies
	RequestIDBytes io.Reader
}

type API struct {
	db                Pinger
	logger            *slog.Logger
	authService       AuthService
	groupService      GroupService
	expenseService    ExpenseService
	repaymentService  RepaymentService
	settlementService SettlementService
	sessions          SessionValidator
	csrf              CSRFProtector
	sessionCookies    auth.SessionCookies
	csrfCookies       auth.CSRFCookies
	origins           OriginPolicy
	requestIDBytes    io.Reader
	mux               *http.ServeMux
	handler           http.Handler
}

func New(db Pinger, logger *slog.Logger, options Options) (*API, error) {
	if db == nil ||
		logger == nil ||
		options.Auth == nil ||
		options.Groups == nil ||
		options.Expenses == nil ||
		options.Repayments == nil ||
		options.Settlements == nil ||
		options.Sessions == nil ||
		options.CSRF == nil {
		return nil, errors.New("invalid HTTP API configuration")
	}

	origins, err := NewOriginPolicy(options.AllowedOrigins)
	if err != nil {
		return nil, err
	}
	requestIDBytes := options.RequestIDBytes
	if requestIDBytes == nil {
		requestIDBytes = rand.Reader
	}

	api := &API{
		db:                db,
		logger:            logger,
		authService:       options.Auth,
		groupService:      options.Groups,
		expenseService:    options.Expenses,
		repaymentService:  options.Repayments,
		settlementService: options.Settlements,
		sessions:          options.Sessions,
		csrf:              options.CSRF,
		sessionCookies:    options.SessionCookies,
		csrfCookies:       options.CSRFCookies,
		origins:           origins,
		requestIDBytes:    requestIDBytes,
		mux:               http.NewServeMux(),
	}
	api.register("GET /api/health/live", routePublic, api.live)
	api.register("GET /api/health/ready", routePublic, api.ready)
	api.register("GET /api/auth/csrf", routeOptionalSession, api.getCSRF)
	api.register("POST /api/auth/register", routeAnonymousUnsafe, api.registerUser)
	api.register("POST /api/auth/login", routeAnonymousUnsafe, api.login)
	api.register("POST /api/auth/logout", routeAuthenticatedUnsafe, api.logout)
	api.register("GET /api/me", routeAuthenticated, api.me)
	api.register("GET /api/groups", routeAuthenticated, api.listGroups)
	api.register("POST /api/groups", routeAuthenticatedUnsafe, api.createGroup)
	api.register("POST /api/groups/join", routeAuthenticatedUnsafe, api.joinGroup)
	api.register("GET /api/groups/{groupId}", routeAuthenticated, api.getGroup)
	api.register("PATCH /api/groups/{groupId}", routeAuthenticatedUnsafe, api.renameGroup)
	api.register("DELETE /api/groups/{groupId}", routeAuthenticatedUnsafe, api.dissolveGroup)
	api.register(
		"GET /api/groups/{groupId}/join-code",
		routeAuthenticated,
		api.getGroupJoinCode,
	)
	api.register(
		"DELETE /api/groups/{groupId}/members/{userId}",
		routeAuthenticatedUnsafe,
		api.removeGroupMember,
	)
	api.register(
		"GET /api/groups/{groupId}/expenses",
		routeAuthenticated,
		api.listExpenses,
	)
	api.register(
		"POST /api/groups/{groupId}/expenses",
		routeAuthenticatedUnsafe,
		api.createExpense,
	)
	api.register(
		"GET /api/groups/{groupId}/expenses/{expenseId}",
		routeAuthenticated,
		api.getExpense,
	)
	api.register(
		"PUT /api/groups/{groupId}/expenses/{expenseId}",
		routeAuthenticatedUnsafe,
		api.replaceExpense,
	)
	api.register(
		"DELETE /api/groups/{groupId}/expenses/{expenseId}",
		routeAuthenticatedUnsafe,
		api.deleteExpense,
	)
	api.register(
		"GET /api/groups/{groupId}/repayments",
		routeAuthenticated,
		api.listRepayments,
	)
	api.register(
		"POST /api/groups/{groupId}/repayments",
		routeAuthenticatedUnsafe,
		api.createRepayment,
	)
	api.register(
		"GET /api/groups/{groupId}/repayments/{repaymentId}",
		routeAuthenticated,
		api.getRepayment,
	)
	api.register(
		"PUT /api/groups/{groupId}/repayments/{repaymentId}",
		routeAuthenticatedUnsafe,
		api.replaceRepayment,
	)
	api.register(
		"DELETE /api/groups/{groupId}/repayments/{repaymentId}",
		routeAuthenticatedUnsafe,
		api.deleteRepayment,
	)
	api.register(
		"GET /api/groups/{groupId}/settlements",
		routeAuthenticated,
		api.listSettlements,
	)
	api.handler = api.withPanicRecovery(
		api.withRequestIDAndAccessLog(
			api.withSecurityHeaders(
				api.withCORS(http.HandlerFunc(api.serveRoutes)),
			),
		),
	)
	return api, nil
}

func (a *API) Handler() http.Handler {
	return a.handler
}
