package v1controllers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/middleware"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/server"
	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFinanceTransactionPagination(t *testing.T) {
	fake := faker.New()
	userID := "user-" + fake.UUID().V4()
	tenantID := "tenant-" + fake.UUID().V4()
	accountID := "account-" + fake.UUID().V4()
	start := time.Date(2026, time.September, 15, 10, 0, 0, 0, time.FixedZone("test", 2*60*60))
	end := start.Add(time.Hour)

	makeAuthMiddleware := func() middleware.AuthMiddleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				ctx := auth.ContextWithCaller(request.Context(), auth.Caller{
					UserID: userID, Credential: auth.CredentialKindSession,
				})
				next.ServeHTTP(w, request.WithContext(ctx))
			})
		}
	}
	makeHandler := func(service *mockfinanceService) http.Handler {
		controller := NewFinanceController(FinanceControllerDeps{
			LedgerService:        service,
			AuthMiddleware:       makeAuthMiddleware(),
			TokenReadMiddleware:  makeAuthMiddleware(),
			TokenWriteMiddleware: makeAuthMiddleware(),
		})
		return server.NewTestRootHandler().RegisterFinanceRoutes(controller)
	}

	t.Run("defaults and bounds the shared transaction limit while retaining filters and paging", func(t *testing.T) {
		service := newMockfinanceService(t)
		for _, testCase := range []struct {
			name      string
			query     string
			wantLimit int64
			wantPage  bool
		}{
			{name: "omitted limit defaults", wantLimit: 100},
			{name: "minimum limit", query: "?limit=1", wantLimit: 1},
			{name: "maximum limit and filters", query: "?accountId=" + accountID + "&source=provider&status=booked&kind=expense&startDate=" + url.QueryEscape(start.Format(time.RFC3339Nano)) + "&endDate=" + url.QueryEscape(end.Format(time.RFC3339Nano)) + "&sort=asc&includeHidden=true&limit=200&offset=7", wantLimit: 200, wantPage: true},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				service.EXPECT().
					ListTransactions(
						mock.Anything,
						mock.MatchedBy(func(params financepkg.ListTransactionsParams) bool {
							if params.ActorUserID != userID || params.TenantID != tenantID ||
								params.Limit != testCase.wantLimit {
								return false
							}
							if !testCase.wantPage {
								return params.AccountID == "" && params.Offset == 0 && !params.SortAscending
							}
							return params.AccountID == accountID &&
								params.Source == domain.TransactionSourceProvider &&
								params.Status == domain.TransactionStatusBooked &&
								params.Kind == domain.TransactionKindExpense &&
								params.StartDate.Equal(start) && params.EndDate.Equal(end) &&
								params.SortAscending && params.IncludeHidden && params.Offset == 7
						}),
					).
					Return([]domain.Transaction{}, nil).
					Once()
				response := httptest.NewRecorder()
				makeHandler(service).ServeHTTP(response, httptest.NewRequest(
					http.MethodGet,
					"/api/v1/finance/tenants/"+tenantID+"/transactions"+testCase.query,
					strings.NewReader(""),
				))
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			})
		}
	})

	t.Run("rejects an over-limit transaction page before the finance service", func(t *testing.T) {
		response := httptest.NewRecorder()
		makeHandler(newMockfinanceService(t)).ServeHTTP(response, httptest.NewRequest(
			http.MethodGet,
			"/api/v1/finance/tenants/"+tenantID+"/transactions?limit=201",
			nil,
		))
		require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
	})
}
