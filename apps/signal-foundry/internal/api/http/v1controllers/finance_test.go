package v1controllers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/middleware"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/server"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	financepkg "github.com/gemyago/signal-foundry/finance"
	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/runtime/httpapi"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFinanceController(t *testing.T) {
	fake := faker.New()

	makeAuthMiddleware := func(userID string) middleware.AuthMiddleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") == "" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				ctx := httpapi.ContextWithCallerIdentity(
					r.Context(),
					&testCallerIdentity{userID: userID},
				)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}
	}

	newHandler := func(
		service financeService,
		bankConnections bankConnectionService,
		auth middleware.AuthMiddleware,
	) http.Handler {
		ctrl := NewFinanceController(
			FinanceControllerDeps{
				TenantService:         service,
				CatalogService:        service,
				LedgerService:         service,
				BankSyncService:       service,
				ReportingService:      service,
				FXService:             service,
				CSVImportService:      service,
				BankConnectionService: bankConnections,
				AuthMiddleware:        auth,
			},
		)
		return server.NewTestRootHandler().RegisterFinanceRoutes(ctrl)
	}

	newRequest := func(method, target, body string, authenticated bool) *http.Request {
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		req = req.WithContext(t.Context())
		req.Header.Set("Content-Type", "application/json")
		if authenticated {
			req.Header.Set("Authorization", "Bearer "+fake.UUID().V4())
		}
		return req
	}

	decode := func(t *testing.T, resp *httptest.ResponseRecorder) map[string]any {
		t.Helper()
		var payload map[string]any
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
		return payload
	}

	t.Run("finance endpoints require auth", func(t *testing.T) {
		handler := newHandler(
			newMockfinanceService(t),
			newMockbankConnectionService(t),
			makeAuthMiddleware(fake.UUID().V4()),
		)
		for _, tc := range []struct {
			method string
			target string
		}{
			{method: http.MethodGet, target: "/api/v1/finance/tenants"},
			{method: http.MethodGet, target: "/api/v1/finance/fx/diagnostics"},
			{method: http.MethodPost, target: "/api/v1/finance/invites/accept"},
		} {
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, newRequest(tc.method, tc.target, "", false))
			require.Equal(t, http.StatusUnauthorized, resp.Code)
		}
	})

	t.Run("finance routes reject missing caller identity", func(t *testing.T) {
		service := newMockfinanceService(t)
		handler := newHandler(
			service,
			newMockbankConnectionService(t),
			func(next http.Handler) http.Handler { return next },
		)

		for _, tc := range []struct {
			name   string
			method string
			target string
			body   string
		}{
			{name: "list tenants", method: http.MethodGet, target: "/api/v1/finance/tenants"},
			{name: "create tenant", method: http.MethodPost, target: "/api/v1/finance/tenants", body: `{"name":"Household","displayCurrency":"USD"}`},
			{name: "get tenant", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a"},
			{name: "list members", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/members"},
			{name: "list invites", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/invites"},
			{name: "create invite", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/invites", body: `{"recipient":"friend@example.com"}`},
			{name: "accept invite", method: http.MethodPost, target: "/api/v1/finance/invites/accept", body: `{"code":"invite-code"}`},
			{name: "list accounts", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/accounts?includeHidden=true"},
			{name: "create account", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/accounts", body: `{"name":"Checking","currency":"USD","kind":"manual"}`},
			{name: "get account", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/accounts/account-a"},
			{name: "list categories", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/categories?includeHidden=true"},
			{name: "create category", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/categories", body: `{"name":"Groceries","kind":"expense"}`},
			{name: "list tags", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/tags?includeHidden=true"},
			{name: "create tag", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/tags", body: `{"name":"Household"}`},
			{name: "list transactions", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/transactions?accountId=account-a&source=manual&status=booked&includeHidden=true"},
			{name: "create transaction", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/transactions", body: `{"accountId":"account-a","source":"manual","status":"booked","kind":"regular","amountMinor":-2500,"currency":"USD","description":"Coffee","effectiveAt":"2026-06-20T14:00:00Z"}`},
			{name: "get transaction", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/transactions/transaction-a"},
			{name: "update transaction", method: http.MethodPatch, target: "/api/v1/finance/tenants/tenant-a/transactions/transaction-a", body: `{"description":"Coffee update","amountMinor":-3100,"effectiveAt":"2026-06-21T10:00:00Z"}`},
			{name: "list connections", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/connections"},
			{name: "delete connection", method: http.MethodDelete, target: "/api/v1/finance/tenants/tenant-a/connections/connection-a"},
			{name: "link connection", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/connections/link-token", body: `{"provider":"monobank","token":"token-1"}`},
			{name: "start redirect connection", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/connections/link-redirect/start", body: `{"provider":"pko","callbackUrl":"https://app.example.test/#/finance/connections"}`},
			{name: "finish redirect connection", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/connections/link-redirect/finish", body: `{"provider":"pko","state":"state-1","code":"code-1"}`},
			{name: "trigger connection sync", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/connections/connection-a/sync", body: `{"reason":"manual"}`},
			{name: "dashboard", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/dashboard?preset=current_month&startDate=2026-06-01&endDate=2026-06-30"},
			{name: "fx diagnostics", method: http.MethodGet, target: "/api/v1/finance/fx/diagnostics"},
			{name: "fx sync", method: http.MethodPost, target: "/api/v1/finance/fx/sync", body: `{"provider":"nbp","baseCurrencies":["EUR"],"quoteCurrency":"USD","startDate":"2026-06-01T00:00:00Z","endDate":"2026-06-21T00:00:00Z"}`},
			{name: "preview import", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/imports/preview", body: `{"importType":"transactions","fileName":"demo.csv","csv":"account,amount\nChecking,100"}`},
			{name: "confirm import", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/imports/import-a/confirm", body: `{"mapping":{"account":"account"}}`},
			{name: "get import audit", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/imports/import-a"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				resp := httptest.NewRecorder()
				handler.ServeHTTP(resp, newRequest(tc.method, tc.target, tc.body, true))
				require.Equal(t, http.StatusUnauthorized, resp.Code)
			})
		}
	})

	t.Run("tenant routes delegate into finance service", func(t *testing.T) {
		userID := fake.UUID().V4()
		now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
		tenantID := "tenant-" + fake.UUID().V4()
		inviteID := "invite-" + fake.UUID().V4()
		inviteCode := "code-" + fake.UUID().V4()
		service := newMockfinanceService(t)
		handler := newHandler(
			service,
			newMockbankConnectionService(t),
			makeAuthMiddleware(userID),
		)

		tenants := []domain.TenantMembershipView{{
			Tenant: domain.Tenant{
				ID:              tenantID,
				Name:            "tenant-" + fake.Lorem().Word(),
				DisplayCurrency: "USD",
				CreatedAt:       now,
				UpdatedAt:       now,
			},
			Membership: domain.TenantMembership{
				TenantID:  tenantID,
				UserID:    userID,
				JoinedAt:  now,
				CreatedAt: now,
			},
		}}
		service.EXPECT().ListTenantsForUser(mock.Anything, userID).Return(tenants, nil).Twice()
		service.EXPECT().CreateTenant(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, params financepkg.CreateTenantParams) (domain.Tenant, error) {
				require.Equal(t, userID, params.ActorUserID)
				require.Equal(t, "USD", params.DisplayCurrency)
				return domain.Tenant{
					ID:              tenantID,
					Name:            params.Name,
					DisplayCurrency: params.DisplayCurrency,
					CreatedAt:       now,
					UpdatedAt:       now,
				}, nil
			},
		)
		service.EXPECT().ListTenantMembers(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, params financepkg.ListTenantMembersParams) ([]domain.TenantMember, error) {
				require.Equal(t, tenantID, params.TenantID)
				return []domain.TenantMember{
					{TenantID: tenantID, UserID: userID, JoinedAt: now},
				}, nil
			},
		)
		service.EXPECT().ListTenantInvites(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, params financepkg.ListTenantInvitesParams) ([]domain.TenantInvite, error) {
				require.Equal(t, tenantID, params.TenantID)
				return []domain.TenantInvite{{
					ID:              inviteID,
					TenantID:        tenantID,
					Code:            inviteCode,
					Recipient:       "friend@example.com",
					CreatedByUserID: userID,
					CreatedAt:       now,
				}}, nil
			},
		)
		service.EXPECT().CreateTenantInvite(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, params financepkg.CreateTenantInviteParams) (domain.TenantInvite, error) {
				require.Equal(t, tenantID, params.TenantID)
				return domain.TenantInvite{
					ID:              inviteID,
					TenantID:        tenantID,
					Code:            inviteCode,
					Recipient:       params.Recipient,
					CreatedByUserID: userID,
					CreatedAt:       now,
				}, nil
			},
		)
		service.EXPECT().AcceptTenantInvite(
			mock.Anything,
			financepkg.AcceptTenantInviteParams{ActorUserID: userID, Code: inviteCode},
		).Return(
			domain.TenantMembership{TenantID: tenantID, UserID: userID, JoinedAt: now},
			nil,
		)

		listResp := httptest.NewRecorder()
		handler.ServeHTTP(listResp, newRequest(http.MethodGet, "/api/v1/finance/tenants", "", true))
		require.Equal(t, http.StatusOK, listResp.Code)
		listPayload := decode(t, listResp)
		require.Len(t, listPayload["items"].([]any), 1)
		assert.Contains(t, listPayload["items"].([]any)[0].(map[string]any), "displayCurrency")

		createResp := httptest.NewRecorder()
		handler.ServeHTTP(
			createResp,
			newRequest(
				http.MethodPost,
				"/api/v1/finance/tenants",
				`{"name":"Household","displayCurrency":"USD"}`,
				true,
			),
		)
		require.Equal(t, http.StatusOK, createResp.Code)
		assert.Equal(t, tenantID, decode(t, createResp)["id"])

		getResp := httptest.NewRecorder()
		handler.ServeHTTP(
			getResp,
			newRequest(http.MethodGet, "/api/v1/finance/tenants/"+tenantID, "", true),
		)
		require.Equal(t, http.StatusOK, getResp.Code)
		assert.Equal(t, tenantID, decode(t, getResp)["id"])

		membersResp := httptest.NewRecorder()
		handler.ServeHTTP(
			membersResp,
			newRequest(
				http.MethodGet,
				"/api/v1/finance/tenants/"+tenantID+"/members",
				"",
				true,
			),
		)
		require.Equal(t, http.StatusOK, membersResp.Code)
		assert.Len(t, decode(t, membersResp)["items"].([]any), 1)

		invitesResp := httptest.NewRecorder()
		handler.ServeHTTP(
			invitesResp,
			newRequest(http.MethodGet, "/api/v1/finance/tenants/"+tenantID+"/invites", "", true),
		)
		require.Equal(t, http.StatusOK, invitesResp.Code)

		createInviteResp := httptest.NewRecorder()
		handler.ServeHTTP(
			createInviteResp,
			newRequest(
				http.MethodPost,
				"/api/v1/finance/tenants/"+tenantID+"/invites",
				`{"recipient":"friend@example.com"}`,
				true,
			),
		)
		require.Equal(t, http.StatusOK, createInviteResp.Code)
		assert.Equal(t, inviteID, decode(t, createInviteResp)["id"])

		acceptResp := httptest.NewRecorder()
		handler.ServeHTTP(
			acceptResp,
			newRequest(
				http.MethodPost,
				"/api/v1/finance/invites/accept",
				`{"code":"`+inviteCode+`"}`,
				true,
			),
		)
		require.Equal(t, http.StatusOK, acceptResp.Code)
		assert.Equal(t, tenantID, decode(t, acceptResp)["tenantId"])
	})

	t.Run("tenant management routes create list and archive active tenants", func(t *testing.T) {
		userID := fake.UUID().V4()
		now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
		tenantID := "tenant-" + fake.UUID().V4()
		tenantName := "tenant-" + fake.Lorem().Word()
		activeViews := []domain.TenantMembershipView{{
			Tenant: domain.Tenant{
				ID:              tenantID,
				Name:            tenantName,
				DisplayCurrency: "USD",
				CreatedAt:       now,
				UpdatedAt:       now,
			},
			Membership: domain.TenantMembership{
				TenantID:  tenantID,
				UserID:    userID,
				JoinedAt:  now,
				CreatedAt: now,
			},
		}}
		service := newMockfinanceService(t)
		handler := newHandler(
			service,
			newMockbankConnectionService(t),
			makeAuthMiddleware(userID),
		)

		service.EXPECT().CreateTenant(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, params financepkg.CreateTenantParams) (domain.Tenant, error) {
				require.Equal(t, userID, params.ActorUserID)
				require.Equal(t, tenantName, params.Name)
				require.Equal(t, "USD", params.DisplayCurrency)
				return activeViews[0].Tenant, nil
			},
		).Once()
		service.EXPECT().ListTenantsForUser(mock.Anything, userID).Return(activeViews, nil).Once()
		service.EXPECT().ArchiveTenant(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, params financepkg.ArchiveTenantParams) (domain.Tenant, error) {
				require.Equal(t, userID, params.ActorUserID)
				require.Equal(t, tenantID, params.TenantID)
				return domain.Tenant{ID: tenantID}, nil
			},
		).Once()
		service.EXPECT().ListTenantsForUser(mock.Anything, userID).Return(nil, nil).Once()

		createResp := httptest.NewRecorder()
		handler.ServeHTTP(
			createResp,
			newRequest(
				http.MethodPost,
				"/api/v1/finance/tenants",
				`{"name":"`+tenantName+`","displayCurrency":"USD"}`,
				true,
			),
		)
		require.Equal(t, http.StatusOK, createResp.Code)
		createPayload := decode(t, createResp)
		assert.Equal(t, tenantID, createPayload["id"])
		assert.Equal(t, tenantName, createPayload["name"])

		listResp := httptest.NewRecorder()
		handler.ServeHTTP(listResp, newRequest(http.MethodGet, "/api/v1/finance/tenants", "", true))
		require.Equal(t, http.StatusOK, listResp.Code)
		listPayload := decode(t, listResp)
		items := listPayload["items"].([]any)
		require.Len(t, items, 1)
		assert.Equal(t, tenantID, items[0].(map[string]any)["id"])

		archiveResp := httptest.NewRecorder()
		handler.ServeHTTP(
			archiveResp,
			newRequest(http.MethodPost, "/api/v1/finance/tenants/"+tenantID+"/archive", "", true),
		)
		require.Equal(t, http.StatusNoContent, archiveResp.Code)
		assert.Empty(t, archiveResp.Body.String())

		postArchiveListResp := httptest.NewRecorder()
		handler.ServeHTTP(postArchiveListResp, newRequest(http.MethodGet, "/api/v1/finance/tenants", "", true))
		require.Equal(t, http.StatusOK, postArchiveListResp.Code)
		assert.Empty(t, decode(t, postArchiveListResp)["items"].([]any))
	})

	t.Run("catalog and transaction routes keep camelCase and filters", func(t *testing.T) {
		userID := fake.UUID().V4()
		now := time.Date(2026, time.June, 20, 14, 0, 0, 0, time.UTC)
		providerEffectiveAt := now.Add(-2 * time.Hour)
		tenantID := "tenant-" + fake.UUID().V4()
		accountID := "account-" + fake.UUID().V4()
		categoryID := "category-" + fake.UUID().V4()
		updatedCategoryID := "category-updated-" + fake.UUID().V4()
		tagID := "tag-" + fake.UUID().V4()
		transactionID := "tx-" + fake.UUID().V4()
		service := newMockfinanceService(t)
		handler := newHandler(
			service,
			newMockbankConnectionService(t),
			makeAuthMiddleware(userID),
		)

		service.EXPECT().
			ListAccounts(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, params financepkg.ListAccountsParams) ([]domain.Account, error) {
				require.True(t, params.IncludeHidden)
				return []domain.Account{
					{
						ID:                  accountID,
						TenantID:            tenantID,
						Name:                "Checking",
						Currency:            "USD",
						Kind:                domain.AccountKindManual,
						BookedBalanceMinor:  12500,
						PendingBalanceMinor: -1200,
						CreatedAt:           now,
						UpdatedAt:           now,
					},
				}, nil
			})
		service.EXPECT().
			GetAccount(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, params financepkg.GetAccountParams) (domain.Account, error) {
				require.Equal(t, tenantID, params.TenantID)
				require.Equal(t, accountID, params.AccountID)
				return domain.Account{
					ID:                  accountID,
					TenantID:            tenantID,
					Name:                "Checking",
					Currency:            "USD",
					Kind:                domain.AccountKindManual,
					BookedBalanceMinor:  12500,
					PendingBalanceMinor: -1200,
					CreatedAt:           now,
					UpdatedAt:           now,
				}, nil
			})
		service.EXPECT().
			CreateAccount(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, params financepkg.CreateAccountParams) (domain.Account, error) {
				require.Equal(t, domain.AccountKindManual, params.Kind)
				return domain.Account{
					ID:        accountID,
					TenantID:  tenantID,
					Name:      params.Name,
					Currency:  params.Currency,
					Kind:      params.Kind,
					CreatedAt: now,
					UpdatedAt: now,
				}, nil
			})
		service.EXPECT().
			ListCategories(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, params financepkg.ListCategoriesParams) ([]domain.Category, error) {
				require.True(t, params.IncludeHidden)
				return []domain.Category{
					{
						ID:        categoryID,
						TenantID:  tenantID,
						Name:      "Groceries",
						Kind:      domain.CategoryKindExpense,
						CreatedAt: now,
						UpdatedAt: now,
					},
				}, nil
			})
		service.EXPECT().
			CreateCategory(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, params financepkg.CreateCategoryParams) (domain.Category, error) {
				require.Equal(t, domain.CategoryKindExpense, params.Kind)
				return domain.Category{
					ID:        categoryID,
					TenantID:  tenantID,
					Name:      params.Name,
					Kind:      params.Kind,
					CreatedAt: now,
					UpdatedAt: now,
				}, nil
			})
		service.EXPECT().
			ListTags(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, params financepkg.ListTagsParams) ([]domain.Tag, error) {
				require.True(t, params.IncludeHidden)
				return []domain.Tag{
					{
						ID:        tagID,
						TenantID:  tenantID,
						Name:      "Household",
						CreatedAt: now,
						UpdatedAt: now,
					},
				}, nil
			})
		service.EXPECT().
			CreateTag(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, params financepkg.CreateTagParams) (domain.Tag, error) {
				return domain.Tag{
					ID:        tagID,
					TenantID:  tenantID,
					Name:      params.Name,
					CreatedAt: now,
					UpdatedAt: now,
				}, nil
			})
		service.EXPECT().
			ListTransactions(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, params financepkg.ListTransactionsParams) ([]domain.Transaction, error) {
				require.Equal(t, accountID, params.AccountID)
				require.Equal(t, domain.TransactionSourceManual, params.Source)
				require.Equal(t, domain.TransactionStatusBooked, params.Status)
				require.True(t, params.IncludeHidden)
				return []domain.Transaction{
					{
						ID:          transactionID,
						TenantID:    tenantID,
						AccountID:   accountID,
						Source:      domain.TransactionSourceManual,
						Status:      domain.TransactionStatusBooked,
						Kind:        domain.TransactionKindRegular,
						AmountMinor: -2500,
						Currency:    "USD",
						Description: "Coffee",
						EffectiveAt: now,
						ProviderOriginal: &domain.ProviderTransactionOriginal{
							AmountMinor: -2600,
							Currency:    "USD",
							Description: "Provider coffee",
							EffectiveAt: ptrTime(providerEffectiveAt),
						},
						CreatedAt: now,
						UpdatedAt: now,
					},
				}, nil
			})
		service.EXPECT().
			GetTransaction(mock.Anything, financepkg.GetTransactionParams{
				ActorUserID:   userID,
				TenantID:      tenantID,
				TransactionID: transactionID,
			}).
			Return(
				domain.Transaction{
					ID:          transactionID,
					TenantID:    tenantID,
					AccountID:   accountID,
					Source:      domain.TransactionSourceProvider,
					Status:      domain.TransactionStatusBooked,
					Kind:        domain.TransactionKindRegular,
					AmountMinor: -2500,
					Currency:    "USD",
					Description: "Coffee",
					EffectiveAt: now,
					CategoryID:  ptrString(categoryID),
					ProviderOriginal: &domain.ProviderTransactionOriginal{
						AmountMinor: -2600,
						Currency:    "USD",
						Description: "Provider coffee",
						EffectiveAt: ptrTime(providerEffectiveAt),
					},
					CreatedAt: now,
					UpdatedAt: now,
				},
				nil,
			)
		service.EXPECT().
			RecordTransaction(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, params financepkg.RecordTransactionParams) (domain.Transaction, error) {
				require.Equal(t, int64(-2500), params.AmountMinor)
				return domain.Transaction{
					ID:              transactionID,
					TenantID:        tenantID,
					AccountID:       params.AccountID,
					Source:          params.Source,
					Status:          params.Status,
					Kind:            params.Kind,
					AmountMinor:     params.AmountMinor,
					Currency:        params.Currency,
					Description:     params.Description,
					EffectiveAt:     params.EffectiveAt,
					CategoryID:      ptrString(params.CategoryID),
					TransferGroupID: ptrString(params.TransferGroupID),
					CreatedAt:       now,
					UpdatedAt:       now,
				}, nil
			})
		service.EXPECT().
			UpdateTransaction(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, params financepkg.UpdateTransactionParams) (domain.Transaction, error) {
				require.Equal(t, transactionID, params.TransactionID)
				require.Equal(t, int64(-3100), params.AmountMinor)
				require.Equal(t, updatedCategoryID, params.CategoryID)
				return domain.Transaction{
					ID:          transactionID,
					TenantID:    tenantID,
					AccountID:   accountID,
					Source:      domain.TransactionSourceProvider,
					Status:      domain.TransactionStatusBooked,
					Kind:        domain.TransactionKindRegular,
					AmountMinor: params.AmountMinor,
					Currency:    "USD",
					Description: params.Description,
					EffectiveAt: params.EffectiveAt,
					CategoryID:  ptrString(params.CategoryID),
					ProviderOriginal: &domain.ProviderTransactionOriginal{
						AmountMinor: -2600,
						Currency:    "USD",
						Description: "Provider coffee",
						EffectiveAt: ptrTime(providerEffectiveAt),
					},
					CreatedAt: now,
					UpdatedAt: now.Add(time.Minute),
				}, nil
			})

		for _, tc := range []struct {
			method string
			target string
			body   string
			field  string
			want   any
		}{
			{method: http.MethodGet, target: "/api/v1/finance/tenants/" + tenantID + "/accounts?includeHidden=true", field: "items", want: 1},
			{method: http.MethodPost, target: "/api/v1/finance/tenants/" + tenantID + "/accounts", body: `{"name":"Checking","currency":"USD","kind":"manual"}`, field: "id", want: accountID},
			{method: http.MethodGet, target: "/api/v1/finance/tenants/" + tenantID + "/accounts/" + accountID, field: "bookedBalanceMinor", want: float64(12500)},
			{method: http.MethodGet, target: "/api/v1/finance/tenants/" + tenantID + "/categories?includeHidden=true", field: "items", want: 1},
			{method: http.MethodPost, target: "/api/v1/finance/tenants/" + tenantID + "/categories", body: `{"name":"Groceries","kind":"expense"}`, field: "id", want: categoryID},
			{method: http.MethodGet, target: "/api/v1/finance/tenants/" + tenantID + "/tags?includeHidden=true", field: "items", want: 1},
			{method: http.MethodPost, target: "/api/v1/finance/tenants/" + tenantID + "/tags", body: `{"name":"Household"}`, field: "id", want: tagID},
			{method: http.MethodGet, target: "/api/v1/finance/tenants/" + tenantID + "/transactions?accountId=" + accountID + "&source=manual&status=booked&includeHidden=true", field: "items", want: 1},
			{method: http.MethodGet, target: "/api/v1/finance/tenants/" + tenantID + "/transactions/" + transactionID, field: "id", want: transactionID},
			{method: http.MethodPost, target: "/api/v1/finance/tenants/" + tenantID + "/transactions", body: `{"accountId":"` + accountID + `","source":"manual","status":"booked","kind":"regular","amountMinor":-2500,"currency":"USD","description":"Coffee","effectiveAt":"2026-06-20T14:00:00Z","categoryId":"` + categoryID + `","transferGroupId":"group-1"}`, field: "id", want: transactionID},
			{method: http.MethodPatch, target: "/api/v1/finance/tenants/" + tenantID + "/transactions/" + transactionID, body: `{"description":"Coffee update","amountMinor":-3100,"effectiveAt":"2026-06-21T10:00:00Z","categoryId":"` + updatedCategoryID + `"}`, field: "id", want: transactionID},
		} {
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, newRequest(tc.method, tc.target, tc.body, true))
			require.Equal(t, http.StatusOK, resp.Code)
			payload := decode(t, resp)
			if tc.field == "items" {
				assert.Len(t, payload[tc.field].([]any), tc.want.(int))
				if strings.Contains(tc.target, "/accounts?") {
					item := payload[tc.field].([]any)[0].(map[string]any)
					assert.InDelta(t, 12500, item["bookedBalanceMinor"], 0)
					assert.InDelta(t, -1200, item["pendingBalanceMinor"], 0)
				}
				continue
			}
			assert.Equal(t, tc.want, payload[tc.field])
			if strings.Contains(tc.target, "/transactions") && tc.field == "id" {
				if providerOriginal, ok := payload["providerOriginal"].(map[string]any); ok {
					assert.Equal(t, "Provider coffee", providerOriginal["description"])
					assert.Equal(t, "USD", providerOriginal["currency"])
				}
			}
		}

		serviceForNullCategory := newMockfinanceService(t)
		handlerForNullCategory := newHandler(
			serviceForNullCategory,
			newMockbankConnectionService(t),
			makeAuthMiddleware(userID),
		)
		serviceForNullCategory.EXPECT().
			UpdateTransaction(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, params financepkg.UpdateTransactionParams) (domain.Transaction, error) {
				require.Empty(t, params.CategoryID)
				return domain.Transaction{
					ID:          transactionID,
					TenantID:    tenantID,
					AccountID:   accountID,
					Source:      domain.TransactionSourceProvider,
					Status:      domain.TransactionStatusBooked,
					Kind:        domain.TransactionKindRegular,
					AmountMinor: params.AmountMinor,
					Currency:    "USD",
					Description: params.Description,
					EffectiveAt: params.EffectiveAt,
					CreatedAt:   now,
					UpdatedAt:   now,
				}, nil
			})

		nullCategoryResp := httptest.NewRecorder()
		handlerForNullCategory.ServeHTTP(
			nullCategoryResp,
			newRequest(
				http.MethodPatch,
				"/api/v1/finance/tenants/"+tenantID+"/transactions/"+transactionID,
				`{"description":"Coffee cleared","amountMinor":-3200,"effectiveAt":"2026-06-21T11:00:00Z","categoryId":null}`,
				true,
			),
		)
		require.Equal(t, http.StatusOK, nullCategoryResp.Code)
		assert.Equal(t, transactionID, decode(t, nullCategoryResp)["id"])

		invalidBodyResp := httptest.NewRecorder()
		handler.ServeHTTP(
			invalidBodyResp,
			newRequest(
				http.MethodPatch,
				"/api/v1/finance/tenants/"+tenantID+"/transactions/"+transactionID,
				`{"description":`,
				true,
			),
		)
		require.Equal(t, http.StatusBadRequest, invalidBodyResp.Code)
	})

	t.Run(
		"connections dashboard fx and import routes delegate into finance service",
		func(t *testing.T) {
			userID := fake.UUID().V4()
			now := time.Date(2026, time.June, 21, 9, 0, 0, 0, time.UTC)
			tenantID := "tenant-" + fake.UUID().V4()
			connectionID := "connection-" + fake.UUID().V4()
			importID := "import-" + fake.UUID().V4()
			service := newMockfinanceService(t)
			bankConnections := newMockbankConnectionService(t)
			handler := newHandler(
				service,
				bankConnections,
				makeAuthMiddleware(userID),
			)

			service.EXPECT().
				ListBankConnections(mock.Anything, financepkg.ListBankConnectionsParams{ActorUserID: userID, TenantID: tenantID}).
				Return([]financepkg.BankConnectionView{{
					Connection: domain.BankConnection{
						ID:                connectionID,
						TenantID:          tenantID,
						Provider:          "monobank",
						DisplayName:       "Primary",
						ExternalID:        "ext-1",
						ProviderReference: "ref-1",
						State:             domain.BankConnectionStateActive,
						CreatedAt:         now,
						UpdatedAt:         now,
					},
					Schedule: &domain.BankConnectionSchedule{
						ConnectionID: connectionID,
						Interval:     15 * time.Minute,
						Enabled:      true,
						CreatedAt:    now,
						UpdatedAt:    now,
					},
				}}, nil)
			bankConnections.EXPECT().
				LinkTokenBankConnection(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, params financepkg.LinkTokenBankConnectionParams) (domain.BankConnection, error) {
					require.Equal(t, userID, params.ActorUserID)
					require.Equal(t, tenantID, params.TenantID)
					require.Equal(t, "monobank", params.Provider)
					return domain.BankConnection{
						ID:                connectionID,
						TenantID:          tenantID,
						Provider:          params.Provider,
						DisplayName:       "Linked",
						ExternalID:        "ext-2",
						ProviderReference: "ref-2",
						State:             domain.BankConnectionStateActive,
						CreatedAt:         now,
						UpdatedAt:         now,
					}, nil
				})
			startState := "state-" + fake.UUID().V4()
			startAuthorizationURL := "https://enable-banking.example.test/sessions/" + fake.UUID().V4()
			bankConnections.EXPECT().
				StartBankConnectionLink(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, params financepkg.StartBankConnectionLinkParams) (financepkg.ProviderLinkStart, error) {
					require.Equal(t, userID, params.ActorUserID)
					require.Equal(t, tenantID, params.TenantID)
					require.Equal(t, "pko", params.Provider)
					require.Equal(t, "http://example.com/enable-banking/callback", params.RedirectURL)
					require.Equal(t, "https://app.example.test/#/finance/connections", params.BrowserCallbackURL)
					return financepkg.ProviderLinkStart{
						State:            startState,
						AuthorizationURL: startAuthorizationURL,
					}, nil
				})
			bankConnections.EXPECT().
				FinishBankConnectionLink(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, params financepkg.FinishBankConnectionLinkParams) (domain.BankConnection, error) {
					require.Equal(t, userID, params.ActorUserID)
					require.Equal(t, tenantID, params.TenantID)
					require.Equal(t, "pko", params.Provider)
					require.Equal(t, startState, params.State)
					require.Equal(t, "finish-code", params.Code)
					require.Equal(t, financepkg.ProviderLinkStart{}, params.Start)
					return domain.BankConnection{
						ID:                connectionID,
						TenantID:          tenantID,
						Provider:          params.Provider,
						DisplayName:       "PKO Linked",
						ExternalID:        "ext-redirect",
						ProviderReference: "ref-redirect",
						State:             domain.BankConnectionStateActive,
						CreatedAt:         now,
						UpdatedAt:         now,
					}, nil
				})
			windowStart := now.Add(-24 * time.Hour)
			windowEnd := now
			service.EXPECT().
				TriggerBankConnectionSync(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, params financepkg.TriggerBankConnectionSyncParams) (financepkg.BankConnectionSyncJobRef, error) {
					require.Equal(t, userID, params.ActorUserID)
					require.Equal(t, tenantID, params.TenantID)
					require.Equal(t, connectionID, params.ConnectionID)
					require.Equal(t, financepkg.BankConnectionSyncReasonManual, params.Reason)
					require.Equal(t, windowStart, *params.WindowStart)
					require.Equal(t, windowEnd, *params.WindowEnd)
					return financepkg.BankConnectionSyncJobRef{
						ID:      "job-sync-1",
						JobType: financepkg.BankConnectionSyncJobType,
					}, nil
				})
			service.EXPECT().
				DeleteBankConnection(mock.Anything, financepkg.DeleteBankConnectionParams{
					ActorUserID:  userID,
					TenantID:     tenantID,
					ConnectionID: connectionID,
				}).
				Return(nil)
			service.EXPECT().
				GetDashboard(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, params financepkg.DashboardParams) (financepkg.Dashboard, error) {
					require.Equal(t, financepkg.DashboardPeriodPresetCurrentMonth, params.Preset)
					require.Equal(t, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), params.StartDate)
					require.Equal(t, time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC), params.EndDate)
					return financepkg.Dashboard{
						Period: financepkg.DashboardPeriod{
							Preset:    params.Preset,
							StartDate: params.StartDate,
							EndDate:   params.EndDate,
						},
						Settled: financepkg.DashboardMoneySummary{
							DisplayCurrency:  "USD",
							TransactionCount: 2,
							Complete:         true,
						},
					}, nil
				})
			service.EXPECT().
				GetFXAdminDiagnostics(mock.Anything, financepkg.FXAdminDiagnosticsParams{}).
				Return(financepkg.FXAdminDiagnostics{DefaultProvider: "nbp", StoredRatesCount: 3, Providers: []financepkg.FXAdminProviderDiagnostics{{Name: "nbp", Default: true, Ready: true}}}, nil)
			service.EXPECT().
				TriggerFXSync(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, params financepkg.TriggerFXSyncParams) (financepkg.FXSyncJobRef, error) {
					require.Equal(t, userID, params.RequestedByUserID)
					return financepkg.FXSyncJobRef{
						ID:      "job-fx-1",
						JobType: financepkg.FXSyncJobType,
					}, nil
				})
			service.EXPECT().
				PreviewCSVImport(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, params financepkg.PreviewCSVImportParams) (financepkg.CSVImportPreview, error) {
					require.Equal(t, financepkg.CSVImportTypeTransactions, params.ImportType)
					return financepkg.CSVImportPreview{
						ImportID:            importID,
						ImportType:          params.ImportType,
						Headers:             []string{"account", "amount"},
						Mapping:             map[string]string{"account": "account"},
						WouldCreateAccounts: []string{"Checking"},
					}, nil
				})
			service.EXPECT().
				ConfirmCSVImport(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, params financepkg.ConfirmCSVImportParams) (financepkg.CSVImportConfirmation, error) {
					require.Equal(t, importID, params.ImportID)
					return financepkg.CSVImportConfirmation{
						ImportID: importID,
						JobID:    "job-import-1",
						JobType:  financepkg.CSVImportJobTypeTransactions,
					}, nil
				})
			service.EXPECT().
				GetCSVImportAudit(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, params financepkg.GetCSVImportAuditParams) (financepkg.CSVImportAudit, error) {
					require.Equal(t, importID, params.ImportID)
					confirmedAt := now.Add(time.Minute)
					completedAt := now.Add(2 * time.Minute)
					return financepkg.CSVImportAudit{
						ImportID:          importID,
						TenantID:          tenantID,
						ImportType:        financepkg.CSVImportTypeTransactions,
						Status:            financepkg.CSVImportStatusCompleted,
						JobID:             "job-import-1",
						ConfirmedByUserID: userID,
						ImportedCount:     4,
						CreatedAt:         now,
						ConfirmedAt:       &confirmedAt,
						CompletedAt:       &completedAt,
					}, nil
				})

			cases := []struct {
				method string
				target string
				body   string
				field  string
				want   any
				status int
			}{
				{
					method: http.MethodGet,
					target: "/api/v1/finance/tenants/" + tenantID + "/connections",
					field:  "items",
					want:   1,
					status: http.StatusOK,
				},
				{
					method: http.MethodDelete,
					target: "/api/v1/finance/tenants/" + tenantID + "/connections/" + connectionID,
					status: http.StatusNoContent,
				},
				{
					method: http.MethodPost,
					target: "/api/v1/finance/tenants/" + tenantID + "/connections/link-token",
					body:   `{"provider":"monobank","token":"token-1"}`,
					field:  "id",
					want:   connectionID,
					status: http.StatusOK,
				},
				{
					method: http.MethodPost,
					target: "/api/v1/finance/tenants/" + tenantID + "/connections/link-redirect/start",
					body:   `{"provider":"pko","callbackUrl":"https://app.example.test/#/finance/connections"}`,
					field:  "authorizationUrl",
					want:   startAuthorizationURL,
					status: http.StatusOK,
				},
				{
					method: http.MethodPost,
					target: "/api/v1/finance/tenants/" + tenantID + "/connections/link-redirect/finish",
					body:   `{"provider":"pko","state":"` + startState + `","code":"finish-code","start":{"state":"browser-state","authorizationUrl":"https://evil.example.test"}}`,
					field:  "id",
					want:   connectionID,
					status: http.StatusOK,
				},
				{
					method: http.MethodPost,
					target: "/api/v1/finance/tenants/" + tenantID + "/connections/" + connectionID + "/sync",
					body:   `{"reason":"manual","windowStart":"2026-06-20T09:00:00Z","windowEnd":"2026-06-21T09:00:00Z"}`,
					field:  "jobId",
					want:   "job-sync-1",
					status: http.StatusOK,
				},
				{
					method: http.MethodGet,
					target: "/api/v1/finance/tenants/" + tenantID + "/dashboard?preset=current_month&startDate=2026-06-01&endDate=2026-06-30",
					field:  "settled",
					want:   true,
					status: http.StatusOK,
				},
				{
					method: http.MethodGet,
					target: "/api/v1/finance/fx/diagnostics",
					field:  "storedRatesCount",
					want:   float64(3),
					status: http.StatusOK,
				},
				{
					method: http.MethodPost,
					target: "/api/v1/finance/fx/sync",
					body:   `{"provider":"nbp","baseCurrencies":["EUR"],"quoteCurrency":"USD","startDate":"2026-06-01T00:00:00Z","endDate":"2026-06-21T00:00:00Z"}`,
					field:  "jobId",
					want:   "job-fx-1",
					status: http.StatusOK,
				},
				{
					method: http.MethodPost,
					target: "/api/v1/finance/tenants/" + tenantID + "/imports/preview",
					body:   `{"importType":"transactions","fileName":"demo.csv","csv":"account,amount\nChecking,100"}`,
					field:  "importId",
					want:   importID,
					status: http.StatusOK,
				},
				{
					method: http.MethodPost,
					target: "/api/v1/finance/tenants/" + tenantID + "/imports/" + importID + "/confirm",
					body:   `{"mapping":{"account":"account"}}`,
					field:  "jobId",
					want:   "job-import-1",
					status: http.StatusOK,
				},
				{
					method: http.MethodGet,
					target: "/api/v1/finance/tenants/" + tenantID + "/imports/" + importID,
					field:  "importedCount",
					want:   float64(4),
					status: http.StatusOK,
				},
			}

			for _, tc := range cases {
				resp := httptest.NewRecorder()
				handler.ServeHTTP(resp, newRequest(tc.method, tc.target, tc.body, true))
				require.Equal(t, tc.status, resp.Code)
				if tc.status == http.StatusNoContent {
					assert.Empty(t, resp.Body.String())
					continue
				}
				payload := decode(t, resp)
				if tc.field == "items" {
					assert.Len(t, payload[tc.field].([]any), tc.want.(int))
					continue
				}
				if tc.field == "settled" {
					assert.Contains(t, payload, tc.field)
					continue
				}
				if tc.field == "authorizationUrl" {
					assert.Equal(t, tc.want, payload[tc.field])
					assert.Equal(t, startState, payload["state"])
					continue
				}
				assert.Equal(t, tc.want, payload[tc.field])
			}

			t.Run("provider and callback validation stay explicit", func(t *testing.T) {
				validationService := newMockfinanceService(t)
				validationBankConnections := newMockbankConnectionService(t)
				validationHandler := newHandler(
					validationService,
					validationBankConnections,
					makeAuthMiddleware(userID),
				)

				for _, tc := range []struct {
					name   string
					body   string
					target string
				}{
					{
						name:   "token route rejects pko",
						target: "/api/v1/finance/tenants/" + tenantID + "/connections/link-token",
						body:   `{"provider":"pko","token":"token-1"}`,
					},
					{
						name:   "redirect start rejects monobank",
						target: "/api/v1/finance/tenants/" + tenantID + "/connections/link-redirect/start",
						body:   `{"provider":"monobank","callbackUrl":"https://app.example.test/#/finance/connections"}`,
					},
					{
						name:   "redirect finish rejects monobank",
						target: "/api/v1/finance/tenants/" + tenantID + "/connections/link-redirect/finish",
						body:   `{"provider":"monobank","state":"state-1","code":"code-1"}`,
					},
					{
						name:   "redirect start rejects wrong callback target",
						target: "/api/v1/finance/tenants/" + tenantID + "/connections/link-redirect/start",
						body:   `{"provider":"pko","callbackUrl":"https://app.example.test/#/finance/other"}`,
					},
					{
						name:   "redirect start rejects insecure non local callback",
						target: "/api/v1/finance/tenants/" + tenantID + "/connections/link-redirect/start",
						body:   `{"provider":"pko","callbackUrl":"http://app.example.test/#/finance/connections"}`,
					},
				} {
					t.Run(tc.name, func(t *testing.T) {
						resp := httptest.NewRecorder()
						validationHandler.ServeHTTP(resp, newRequest(http.MethodPost, tc.target, tc.body, true))
						require.Equal(t, http.StatusBadRequest, resp.Code)
					})
				}

				for _, callbackURL := range []string{
					"http://localhost:4173/#/finance/connections",
					"http://127.0.0.1:4173/#/finance/connections",
					"http://[::1]:4173/#/finance/connections",
				} {
					t.Run(callbackURL, func(t *testing.T) {
						localValidationService := newMockfinanceService(t)
						localBankConnections := newMockbankConnectionService(t)
						localBankConnections.EXPECT().
							StartBankConnectionLink(mock.Anything, mock.Anything).
							RunAndReturn(func(_ context.Context, params financepkg.StartBankConnectionLinkParams) (financepkg.ProviderLinkStart, error) {
								require.Equal(t, "http://example.com/enable-banking/callback", params.RedirectURL)
								require.Equal(t, callbackURL, params.BrowserCallbackURL)
								return financepkg.ProviderLinkStart{
									State:            "state-" + fake.UUID().V4(),
									AuthorizationURL: "https://enable-banking.example.test/" + fake.UUID().V4(),
								}, nil
							})
						resp := httptest.NewRecorder()
						newHandler(
							localValidationService,
							localBankConnections,
							makeAuthMiddleware(userID),
						).ServeHTTP(
							resp,
							newRequest(
								http.MethodPost,
								"/api/v1/finance/tenants/"+tenantID+"/connections/link-redirect/start",
								`{"provider":"pko","callbackUrl":"`+callbackURL+`"}`,
								true,
							),
						)
						require.Equal(t, http.StatusOK, resp.Code)
					})
				}
			})
		},
	)

	t.Run("controller maps finance domain and decode errors safely", func(t *testing.T) {
		userID := fake.UUID().V4()

		t.Run("accept invite not found returns 404", func(t *testing.T) {
			service := newMockfinanceService(t)
			service.EXPECT().
				AcceptTenantInvite(mock.Anything, financepkg.AcceptTenantInviteParams{ActorUserID: userID, Code: "missing"}).
				Return(domain.TenantMembership{}, app.NewErrNotFound("invite", "missing"))
			resp := httptest.NewRecorder()
			newHandler(
				service,
				newMockbankConnectionService(t),
				makeAuthMiddleware(userID),
			).ServeHTTP(resp, newRequest(http.MethodPost, "/api/v1/finance/invites/accept", `{"code":"missing"}`, true))
			require.Equal(t, http.StatusNotFound, resp.Code)
		})

		t.Run("confirm import conflict returns 409", func(t *testing.T) {
			service := newMockfinanceService(t)
			service.EXPECT().
				ConfirmCSVImport(mock.Anything, mock.Anything).
				Return(financepkg.CSVImportConfirmation{}, financepkg.ErrCSVImportAlreadyConfirmed)
			resp := httptest.NewRecorder()
			newHandler(
				service,
				newMockbankConnectionService(t),
				makeAuthMiddleware(userID),
			).ServeHTTP(resp, newRequest(http.MethodPost, "/api/v1/finance/tenants/tenant-a/imports/import-a/confirm", `{"mapping":{"name":"name"}}`, true))
			require.Equal(t, http.StatusConflict, resp.Code)
		})

		t.Run("audit denial returns 401", func(t *testing.T) {
			service := newMockfinanceService(t)
			service.EXPECT().
				GetCSVImportAudit(mock.Anything, mock.Anything).
				Return(financepkg.CSVImportAudit{}, financepkg.ErrTenantAccessDenied)
			resp := httptest.NewRecorder()
			newHandler(
				service,
				newMockbankConnectionService(t),
				makeAuthMiddleware(userID),
			).ServeHTTP(resp, newRequest(http.MethodGet, "/api/v1/finance/tenants/tenant-a/imports/import-a", "", true))
			require.Equal(t, http.StatusUnauthorized, resp.Code)
		})

		t.Run("preview invalid json returns 400", func(t *testing.T) {
			service := newMockfinanceService(t)
			resp := httptest.NewRecorder()
			newHandler(
				service,
				newMockbankConnectionService(t),
				makeAuthMiddleware(userID),
			).ServeHTTP(resp, newRequest(http.MethodPost, "/api/v1/finance/tenants/tenant-a/imports/preview", `{"importType":`, true))
			require.Equal(t, http.StatusBadRequest, resp.Code)
		})

		t.Run("dashboard invalid date returns 400", func(t *testing.T) {
			service := newMockfinanceService(t)
			resp := httptest.NewRecorder()
			newHandler(
				service,
				newMockbankConnectionService(t),
				makeAuthMiddleware(userID),
			).ServeHTTP(resp, newRequest(http.MethodGet, "/api/v1/finance/tenants/tenant-a/dashboard?startDate=bad-date", "", true))
			require.Equal(t, http.StatusBadRequest, resp.Code)
		})

		t.Run("create tenant invalid body returns 400", func(t *testing.T) {
			service := newMockfinanceService(t)
			resp := httptest.NewRecorder()
			newHandler(
				service,
				newMockbankConnectionService(t),
				makeAuthMiddleware(userID),
			).ServeHTTP(resp, newRequest(http.MethodPost, "/api/v1/finance/tenants", `{"name":`, true))
			require.Equal(t, http.StatusBadRequest, resp.Code)
		})

		t.Run("preview propagates unexpected internal error", func(t *testing.T) {
			service := newMockfinanceService(t)
			service.EXPECT().
				PreviewCSVImport(mock.Anything, mock.Anything).
				Return(financepkg.CSVImportPreview{}, errors.New("boom"))
			resp := httptest.NewRecorder()
			newHandler(
				service,
				newMockbankConnectionService(t),
				makeAuthMiddleware(userID),
			).ServeHTTP(resp, newRequest(http.MethodPost, "/api/v1/finance/tenants/tenant-a/imports/preview", `{"importType":"transactions","fileName":"demo.csv","csv":"h\n1"}`, true))
			require.Equal(t, http.StatusInternalServerError, resp.Code)
		})

		t.Run("bank provider errors stay sanitized", func(t *testing.T) {
			service := newMockfinanceService(t)
			bankConnections := newMockbankConnectionService(t)
			secret := "secret-" + fake.UUID().V4()
			bankConnections.EXPECT().
				StartBankConnectionLink(mock.Anything, mock.Anything).
				Return(financepkg.ProviderLinkStart{}, errors.New("provider failed with "+secret))
			resp := httptest.NewRecorder()
			newHandler(
				service,
				bankConnections,
				makeAuthMiddleware(userID),
			).ServeHTTP(
				resp,
				newRequest(
					http.MethodPost,
					"/api/v1/finance/tenants/tenant-a/connections/link-redirect/start",
					`{"provider":"pko","callbackUrl":"https://app.example.test/#/finance/connections"}`,
					true,
				),
			)
			require.Equal(t, http.StatusInternalServerError, resp.Code)
			assert.NotContains(t, resp.Body.String(), secret)
		})

		t.Run("unconfigured bank providers return sanitized client error", func(t *testing.T) {
			service := newMockfinanceService(t)
			bankConnections := newMockbankConnectionService(t)
			bankConnections.EXPECT().
				StartBankConnectionLink(mock.Anything, mock.Anything).
				Return(financepkg.ProviderLinkStart{}, fmt.Errorf("%w: pko -> enable-banking", financepkg.ErrBankProviderNotConfigured))
			resp := httptest.NewRecorder()
			newHandler(
				service,
				bankConnections,
				makeAuthMiddleware(userID),
			).ServeHTTP(
				resp,
				newRequest(
					http.MethodPost,
					"/api/v1/finance/tenants/tenant-a/connections/link-redirect/start",
					`{"provider":"pko","callbackUrl":"https://app.example.test/#/finance/connections"}`,
					true,
				),
			)
			require.Equal(t, http.StatusBadRequest, resp.Code)
		})
	})

	t.Run(
		"helper mappers preserve connection schedule dashboard details and date parsing",
		func(t *testing.T) {
			now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60))
			mappedConnection := mapConnection(financepkg.BankConnectionView{
				Connection: domain.BankConnection{
					ID:                   "connection-1",
					TenantID:             "tenant-1",
					Provider:             "provider-1",
					DisplayName:          "Primary",
					ProviderReference:    "ref-1",
					ExternalID:           "ext-1",
					State:                domain.BankConnectionStateActive,
					LastSyncJobID:        "job-1",
					LastSyncStartedAt:    ptrTime(now),
					LastSuccessfulSyncAt: ptrTime(now.Add(time.Minute)),
					CreatedAt:            now,
					UpdatedAt:            now,
				},
				Schedule: &domain.BankConnectionSchedule{
					ConnectionID:    "connection-1",
					Interval:        15 * time.Minute,
					NextRunAt:       ptrTime(now.Add(15 * time.Minute)),
					LastScheduledAt: ptrTime(now.Add(-15 * time.Minute)),
					LastStartedAt:   ptrTime(now.Add(-10 * time.Minute)),
					LastCompletedAt: ptrTime(now.Add(-5 * time.Minute)),
					LastJobID:       "job-1",
					Enabled:         true,
					CreatedAt:       now,
					UpdatedAt:       now,
				},
			})
			require.NotNil(t, mappedConnection.Schedule)
			assert.Equal(t, int64(900), mappedConnection.Schedule.IntervalSeconds)
			assert.Equal(t, now.UTC(), mappedConnection.CreatedAt)

			mappedDashboard := mapDashboard(financepkg.Dashboard{
				Period: financepkg.DashboardPeriod{
					Preset:    financepkg.DashboardPeriodPresetCurrentMonth,
					StartDate: now,
					EndDate:   now.Add(24 * time.Hour),
					Previous: financepkg.DashboardPeriodWindow{
						StartDate: now.Add(-24 * time.Hour),
						EndDate:   now.Add(-time.Hour),
					},
					Next: financepkg.DashboardPeriodWindow{
						StartDate: now.Add(48 * time.Hour),
						EndDate:   now.Add(72 * time.Hour),
					},
				},
				Settled: financepkg.DashboardMoneySummary{
					DisplayCurrency:  "USD",
					TransactionCount: 2,
					Complete:         true,
				},
				Pending: financepkg.DashboardMoneySummary{
					DisplayCurrency:  "USD",
					TransactionCount: 1,
				},
				CategoryBreakdowns: []financepkg.DashboardCategoryBreakdown{
					{
						CategoryID:       "cat-1",
						CategoryName:     "Groceries",
						Kind:             domain.CategoryKindExpense,
						ExpenseMinor:     1200,
						TransactionCount: 1,
					},
				},
				AccountBalances: []financepkg.DashboardAccountBalance{
					{
						AccountID:          "acc-1",
						AccountName:        "Checking",
						Currency:           "USD",
						NativeBookedMinor:  100,
						DisplayBookedMinor: ptrInt64(100),
						MissingFX:          true,
					},
				},
				Alerts: []financepkg.DashboardAlert{
					{Code: "missing-fx", Severity: "warning", Count: 1},
				},
				MissingFX: []financepkg.DashboardMissingFXDiagnostic{
					{
						Source:        financepkg.DashboardMissingFXSourceTransaction,
						TransactionID: "tx-1",
						BaseCurrency:  "EUR",
						QuoteCurrency: "USD",
						RateDate:      now,
						Provider:      "nbp",
					},
				},
				NativeSettledTotals: []financepkg.DashboardCurrencyTotal{
					{Currency: "USD", NetMinor: 100},
				},
			})
			assert.Equal(
				t,
				string(financepkg.DashboardPeriodPresetCurrentMonth),
				mappedDashboard.Period.Preset,
			)
			require.Len(t, mappedDashboard.CategoryBreakdowns, 1)
			require.Len(t, mappedDashboard.AccountBalances, 1)
			require.Len(t, mappedDashboard.Alerts, 1)
			require.Len(t, mappedDashboard.MissingFx, 1)
			require.Len(t, mappedDashboard.NativeSettledTotals, 1)

			req := httptest.NewRequest(
				http.MethodGet,
				"/?startDate=2026-06-01&endDate=2026-06-30",
				nil,
			)
			startDate, endDate, err := parseDashboardDates(req)
			require.NoError(t, err)
			assert.Equal(t, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), startDate)
			assert.Equal(t, time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC), endDate)

			req = httptest.NewRequest(http.MethodGet, "/?startDate=bad-date", nil)
			_, _, err = parseDashboardDates(req)
			require.Error(t, err)

			mappedAccount := mapAccount(domain.Account{
				ID:       "account-1",
				TenantID: "tenant-1",
				Name:     "Linked",
				Currency: "USD",
				Kind:     domain.AccountKindLinked,
				LinkedAccount: &domain.LinkedAccount{
					Provider:          "provider-1",
					ProviderAccountID: "provider-account-1",
				},
				HiddenAt:  ptrTime(now),
				CreatedAt: now,
				UpdatedAt: now,
			})
			assert.Equal(t, "provider-1", mappedAccount.Provider)
			assert.Equal(t, "provider-account-1", mappedAccount.ProviderAccountID)

			mappedPreview := mapCSVPreview(financepkg.CSVImportPreview{
				ImportID:   "import-1",
				ImportType: financepkg.CSVImportTypeTransactions,
				Headers:    []string{"account", "amount"},
				Mapping:    map[string]string{"account": "account"},
				DuplicateRows: []financepkg.CSVImportRejectedRow{
					{RowNumber: 2, Reason: "duplicate"},
				},
				RejectedRows: []financepkg.CSVImportRejectedRow{
					{RowNumber: 3, Reason: "invalid amount"},
				},
			})
			require.Len(t, mappedPreview.DuplicateRows, 1)
			require.Len(t, mappedPreview.RejectedRows, 1)
			assert.Equal(t, 2, mappedPreview.DuplicateRows[0]["rowNumber"])

			boom := errors.New("boom")
			assert.Same(t, boom, mapCSVImportError(boom))
			assert.Nil(t, timePointerOrNil(time.Time{}))
		},
	)

	t.Run("redirect callback helpers validate browser targets and derive backend callbacks", func(t *testing.T) {
		callbackURL, err := buildFinanceProviderRedirectURL(
			func() *http.Request {
				req := httptest.NewRequest(
					http.MethodPost,
					"/api/v1/finance/tenants/tenant-a/connections/link-redirect/start",
					http.NoBody,
				)
				req.Header.Set("X-Forwarded-Proto", "https")
				req.Header.Set("X-Forwarded-Host", "backend.example.test")
				return req
			}(),
			"https://app.example.test/#/finance/connections",
			"",
		)
		require.NoError(t, err)
		assert.Equal(t, "https://backend.example.test/enable-banking/callback", callbackURL)

		req := httptest.NewRequest(http.MethodGet, "/callback", http.NoBody)
		req.Host = "127.0.0.1:4501"
		callbackURL, err = buildFinanceProviderRedirectURL(
			req,
			"http://localhost:5173/#/finance/connections",
			"",
		)
		require.NoError(t, err)
		assert.Equal(t, "http://127.0.0.1:4501/enable-banking/callback", callbackURL)

		callbackURL, err = buildFinanceProviderRedirectURL(
			req,
			"http://localhost:5173/#/finance/connections",
			"http://localhost:6060",
		)
		require.NoError(t, err)
		assert.Equal(t, "http://localhost:6060/enable-banking/callback", callbackURL)

		_, err = buildEnableBankingCallbackURLFromBase("/relative")
		require.Error(t, err)

		req = httptest.NewRequest(http.MethodGet, "/callback", http.NoBody)
		req.TLS = &tls.ConnectionState{}
		req.Host = "secure.example.test"
		assert.Equal(t, urlSchemeHTTPS, forwardedRequestScheme(req))

		req = httptest.NewRequest(http.MethodGet, "/callback", http.NoBody)
		req.Header.Set("Forwarded", `for=127.0.0.1;proto=https;host="proxy.example.test"`)
		assert.Equal(t, urlSchemeHTTPS, forwardedRequestScheme(req))
		assert.Equal(t, "proxy.example.test", forwardedRequestHost(req))

		req = httptest.NewRequest(
			http.MethodGet,
			"https://url-host.example.test/callback",
			http.NoBody,
		)
		req.Host = ""
		assert.Equal(t, "url-host.example.test", forwardedRequestHost(req))
		assert.Equal(t, urlSchemeHTTP, forwardedRequestScheme(nil))
		assert.Empty(t, forwardedRequestHost(nil))

		for _, raw := range []string{
			"http://[::1",
			"/relative#/finance/connections",
			"https://app.example.test/path#/finance/connections",
			"https://app.example.test/#/finance/other",
			"https://user@app.example.test/#/finance/connections",
			"https://app.example.test/?code=1#/finance/connections",
			"http://app.example.test/#/finance/connections",
			"mailto:test@example.com",
		} {
			require.Error(t, ValidateFinanceRedirectCallbackURL(raw))
		}
		require.NoError(
			t,
			ValidateFinanceRedirectCallbackURL(
				"http://127.0.0.1:5173/#/finance/connections",
			),
		)

		invalidInputErr := app.NewErrInvalidInput("provider", "bad provider")
		assert.Same(
			t,
			invalidInputErr,
			sanitizeBankConnectionError(invalidInputErr, "fallback"),
		)
		require.NoError(t, sanitizeBankConnectionError(nil, "fallback"))
		assert.Contains(
			t,
			sanitizeBankConnectionError(
				financepkg.ErrUnsupportedBankProvider,
				"fallback",
			).Error(),
			"unsupported bank provider",
		)
		assert.Contains(
			t,
			sanitizeBankConnectionError(
				financepkg.ErrUnsupportedBankLinkingMethod,
				"fallback",
			).Error(),
			"unsupported bank linking method",
		)
		assert.Contains(
			t,
			sanitizeBankConnectionError(
				financepkg.ErrPendingBankConnectionLinkStartNotFound,
				"fallback",
			).Error(),
			"pending bank link start not found or expired",
		)
		assert.Contains(
			t,
			sanitizeBankConnectionError(
				financepkg.ErrBankConnectionNotFound,
				"fallback",
			).Error(),
			"requested resource",
		)
		assert.Contains(
			t,
			humanizeProviderResponseError(&financepkg.ProviderResponseError{
				Provider:   "enable-banking",
				Code:       "WRONG_ASPSP_PROVIDED",
				StatusCode: http.StatusUnprocessableEntity,
				Message:    "Wrong ASPSP name provided",
			}),
			"APP_FINANCE_PROVIDERS_ENABLEBANKING_ASPSPNAME",
		)
		assert.Contains(
			t,
			sanitizeBankConnectionError(
				&financepkg.ProviderResponseError{
					Provider:   "enable-banking",
					Code:       "WRONG_ASPSP_PROVIDED",
					StatusCode: http.StatusUnprocessableEntity,
					Message:    "Wrong ASPSP name provided",
				},
				"fallback",
			).Error(),
			"APP_FINANCE_PROVIDERS_ENABLEBANKING_ASPSPNAME",
		)
	})
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

func ptrInt64(value int64) *int64 {
	return &value
}

func ptrString(value string) *string {
	return &value
}
