package v1controllers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/middleware"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/server"
	"github.com/gemyago/sumweave/apps/sumweave/internal/app"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/runtime/httpapi"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

//nolint:cyclop,gocyclo // Registered-route scenarios intentionally share one controller fixture.
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

	type controllerOption func(*FinanceControllerDeps)
	withTransferDetails := func(service transferDetailService) controllerOption {
		return func(deps *FinanceControllerDeps) { deps.TransferDetailService = service }
	}
	withSyntheticLinkState := func(service syntheticLinkStateService) controllerOption {
		return func(deps *FinanceControllerDeps) { deps.SyntheticLinkStateService = service }
	}
	withUserDirectory := func(directory userDirectory) controllerOption {
		return func(deps *FinanceControllerDeps) { deps.UserDirectory = directory }
	}
	newHandler := func(
		service financeService,
		bankConnections bankConnectionService,
		auth middleware.AuthMiddleware,
		options ...controllerOption,
	) http.Handler {
		deps := FinanceControllerDeps{
			TenantService:         service,
			UserDirectory:         newMockuserDirectory(t),
			CatalogService:        service,
			LedgerService:         service,
			BankSyncService:       service,
			ReportingService:      service,
			FXService:             service,
			CSVImportService:      service,
			BankConnectionService: bankConnections,
			AuthMiddleware:        auth,
		}
		for _, option := range options {
			option(&deps)
		}
		ctrl := NewFinanceController(
			deps,
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
			{name: "update tenant", method: http.MethodPatch, target: "/api/v1/finance/tenants/tenant-a", body: `{"name":"Household Updated","displayCurrency":"PLN"}`},
			{name: "list members", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/members"},
			{name: "list invites", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/invites"},
			{name: "create invite", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/invites", body: `{"recipient":"friend@example.com"}`},
			{name: "accept invite", method: http.MethodPost, target: "/api/v1/finance/invites/accept", body: `{"code":"invite-code"}`},
			{name: "list accounts", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/accounts?includeHidden=true"},
			{name: "create account", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/accounts", body: `{"name":"Checking","currency":"USD","kind":"manual"}`},
			{name: "get account", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/accounts/account-a"},
			{name: "rename account", method: http.MethodPatch, target: "/api/v1/finance/tenants/tenant-a/accounts/account-a", body: `{"name":"Checking updated"}`},
			{name: "hide account", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/accounts/account-a/hide"},
			{name: "restore account", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/accounts/account-a/unhide"},
			{name: "list categories", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/categories?includeHidden=true"},
			{name: "create category", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/categories", body: `{"name":"Groceries","kind":"expense"}`},
			{name: "update category", method: http.MethodPatch, target: "/api/v1/finance/tenants/tenant-a/categories/category-a", body: `{"name":"Groceries updated","kind":"income"}`},
			{name: "list tags", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/tags?includeHidden=true"},
			{name: "create tag", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/tags", body: `{"name":"Household"}`},
			{name: "rename tag", method: http.MethodPatch, target: "/api/v1/finance/tenants/tenant-a/tags/tag-a", body: `{"name":"Household updated"}`},
			{name: "list transactions", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/transactions?accountId=account-a&source=manual&status=booked&includeHidden=true&limit=20"},
			{name: "create transaction", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/transactions", body: `{"accountId":"account-a","source":"manual","status":"booked","kind":"regular","amountMinor":-2500,"currency":"USD","description":"Coffee","effectiveAt":"2026-06-20T14:00:00Z"}`},
			{name: "get transaction", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/transactions/transaction-a"},
			{name: "list transfer candidates", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/transactions/transaction-a/transfer-candidates?effectiveFrom=2026-06-20T00:00:00Z&effectiveBefore=2026-06-21T00:00:00Z&limit=20"},
			{name: "get transfer partner", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/transactions/transaction-a/transfer-partner"},
			{name: "update transaction", method: http.MethodPatch, target: "/api/v1/finance/tenants/tenant-a/transactions/transaction-a", body: `{"description":"Coffee update","amountMinor":-3100,"effectiveAt":"2026-06-21T10:00:00Z","tagIds":[]}`},
			{name: "link transfer pair", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/transactions/transfer-links", body: `{"firstTransactionId":"transaction-a","secondTransactionId":"transaction-b"}`},
			{name: "unlink transfer pair", method: http.MethodDelete, target: "/api/v1/finance/tenants/tenant-a/transactions/transfer-links", body: `{"firstTransactionId":"transaction-a","secondTransactionId":"transaction-b"}`},
			{name: "list connections", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/connections"},
			{name: "list connection synced accounts", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/connections/connection-a/accounts"},
			{name: "delete connection", method: http.MethodDelete, target: "/api/v1/finance/tenants/tenant-a/connections/connection-a"},
			{name: "rename connection", method: http.MethodPatch, target: "/api/v1/finance/tenants/tenant-a/connections/connection-a", body: `{"name":"Connection updated"}`},
			{name: "link connection", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/connections/link-token", body: `{"provider":"monobank","token":"token-1"}`},
			{name: "start redirect connection", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/connections/link-redirect/start", body: `{"provider":"pko","callbackUrl":"https://app.example.test/#/finance/connections"}`},
			{name: "finish redirect connection", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/connections/link-redirect/finish", body: `{"provider":"pko","state":"state-1","code":"code-1"}`},
			{name: "trigger connection sync", method: http.MethodPost, target: "/api/v1/finance/tenants/tenant-a/connections/connection-a/sync", body: `{"reason":"manual"}`},
			{name: "dashboard", method: http.MethodGet, target: "/api/v1/finance/tenants/tenant-a/dashboard?preset=current_month&startDate=2026-06-01T00:00:00Z&endDate=2026-06-30T00:00:00Z"},
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

	t.Run("registered connection synced-account route returns only safe resolved rows", func(t *testing.T) {
		userID := "user-" + fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		connectionID := "connection-" + fake.UUID().V4()
		syncedAt := time.Date(2026, time.July, 15, 9, 0, 0, 0, time.FixedZone("test", 2*60*60))
		service := newMockfinanceService(t)
		service.EXPECT().
			ListBankConnectionSyncedAccounts(mock.Anything, financepkg.ListBankConnectionSyncedAccountsParams{
				ActorUserID: userID, TenantID: tenantID, ConnectionID: connectionID,
			}).
			Return([]financepkg.BankConnectionSyncedAccount{{
				FinanceAccountID: "account-" + fake.UUID().V4(), Name: "account-" + fake.Lorem().Word(),
				Currency: "USD", LastSuccessfulSyncAt: &syncedAt,
			}}, nil)
		response := httptest.NewRecorder()
		path := "/api/v1/finance/tenants/" + tenantID + "/connections/" + connectionID + "/accounts"
		newHandler(service, newMockbankConnectionService(t), makeAuthMiddleware(userID)).ServeHTTP(
			response,
			newRequest(http.MethodGet, path, "", true),
		)
		require.Equal(t, http.StatusOK, response.Code)
		payload := decode(t, response)
		items, ok := payload["items"].([]any)
		require.True(t, ok)
		require.Len(t, items, 1)
		item, ok := items[0].(map[string]any)
		require.True(t, ok)
		assert.Len(t, item, 4)
		assert.Contains(t, item, "financeAccountId")
		assert.Contains(t, item, "name")
		assert.Contains(t, item, "currency")
		assert.Contains(t, item, "lastSuccessfulSyncAt")
		assert.NotContains(t, item, "providerAccountId")
		assert.NotContains(t, item, "iban")
		assert.NotContains(t, item, "maskedPan")
		assert.NotContains(t, item, "balance")

		service = newMockfinanceService(t)
		service.EXPECT().ListBankConnectionSyncedAccounts(mock.Anything, mock.Anything).
			Return(nil, financepkg.ErrBankConnectionNotFound)
		response = httptest.NewRecorder()
		foreignPath := "/api/v1/finance/tenants/foreign-" + fake.UUID().V4() +
			"/connections/" + connectionID + "/accounts"
		newHandler(service, newMockbankConnectionService(t), makeAuthMiddleware(userID)).ServeHTTP(
			response,
			newRequest(http.MethodGet, foreignPath, "", true),
		)
		require.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("synthetic link-state endpoints map authenticated service state", func(t *testing.T) {
		userID := fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		state := "state-" + fake.UUID().V4()
		accounts := []financepkg.SyntheticLinkStateAccount{{
			Key:      "account-" + fake.UUID().V4(),
			Name:     "checking-" + fake.Lorem().Word(),
			Currency: "USD",
		}}
		expectedState := financepkg.PendingSyntheticLinkState{
			Provider:           "synthetic",
			State:              state,
			ConfiguredAccounts: accounts,
			CanFinish:          true,
		}
		syntheticLinkState := newMocksyntheticLinkStateService(t)
		syntheticLinkState.EXPECT().
			GetPendingSyntheticLinkState(mock.Anything, financepkg.GetPendingSyntheticLinkStateParams{
				ActorUserID: userID,
				TenantID:    tenantID,
				State:       state,
			}).
			Return(expectedState, nil).
			Once()
		syntheticLinkState.EXPECT().
			SavePendingSyntheticLinkState(mock.Anything, financepkg.SavePendingSyntheticLinkStateParams{
				ActorUserID:        userID,
				TenantID:           tenantID,
				State:              state,
				ConfiguredAccounts: accounts,
			}).
			Return(expectedState, nil).
			Once()

		handler := newHandler(
			newMockfinanceService(t),
			newMockbankConnectionService(t),
			makeAuthMiddleware(userID),
			withSyntheticLinkState(syntheticLinkState),
		)
		statePath := "/api/v1/finance/tenants/" + tenantID + "/connections/synthetic-link-states/state/" + state
		unauthorizedResponse := httptest.NewRecorder()
		handler.ServeHTTP(unauthorizedResponse, newRequest(http.MethodGet, statePath, "", false))
		require.Equal(t, http.StatusUnauthorized, unauthorizedResponse.Code)
		syntheticLinkState.AssertNotCalled(t, "GetPendingSyntheticLinkState", mock.Anything, mock.Anything)

		assertResponse := func(response *httptest.ResponseRecorder) {
			require.Equal(t, http.StatusOK, response.Code)
			assert.Equal(t, map[string]any{
				"provider": expectedState.Provider,
				"state":    expectedState.State,
				"configuredAccounts": []any{map[string]any{
					"key":      accounts[0].Key,
					"name":     accounts[0].Name,
					"currency": accounts[0].Currency,
				}},
				"canFinish": expectedState.CanFinish,
			}, decode(t, response))
		}

		getResponse := httptest.NewRecorder()
		handler.ServeHTTP(getResponse, newRequest(http.MethodGet, statePath, "", true))
		assertResponse(getResponse)

		putResponse := httptest.NewRecorder()
		handler.ServeHTTP(
			putResponse,
			newRequest(
				http.MethodPut,
				statePath,
				`{"configuredAccounts":[{"key":"`+accounts[0].Key+`","name":"`+accounts[0].Name+`","currency":"`+accounts[0].Currency+`"}]}`,
				true,
			),
		)
		assertResponse(putResponse)
	})

	t.Run("registered transfer-pair routes invoke tenant-authorized ledger operations", func(t *testing.T) {
		userID := "user-" + fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		firstTransactionID := "transaction-first-" + fake.UUID().V4()
		secondTransactionID := "transaction-second-" + fake.UUID().V4()
		service := newMockfinanceService(t)
		service.EXPECT().LinkTransfers(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, params financepkg.LinkTransfersParams) error {
				assert.Equal(t, userID, params.ActorUserID)
				assert.Equal(t, tenantID, params.TenantID)
				assert.Equal(t, firstTransactionID, params.FirstTransactionID)
				assert.Equal(t, secondTransactionID, params.SecondTransactionID)
				return nil
			},
		).Once()
		service.EXPECT().UnlinkTransfers(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, params financepkg.UnlinkTransfersParams) error {
				assert.Equal(t, userID, params.ActorUserID)
				assert.Equal(t, tenantID, params.TenantID)
				assert.Equal(t, firstTransactionID, params.FirstTransactionID)
				assert.Equal(t, secondTransactionID, params.SecondTransactionID)
				return nil
			},
		).Once()
		handler := newHandler(service, newMockbankConnectionService(t), makeAuthMiddleware(userID))
		body := fmt.Sprintf(
			`{"firstTransactionId":%q,"secondTransactionId":%q}`,
			firstTransactionID,
			secondTransactionID,
		)
		target := "/api/v1/finance/tenants/" + tenantID + "/transactions/transfer-links"

		for _, method := range []string{http.MethodPost, http.MethodDelete} {
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, newRequest(method, target, body, true))
			require.Equal(t, http.StatusNoContent, resp.Code)
			assert.Empty(t, resp.Body.String())
		}
	})

	t.Run("registered transfer-detail routes validate and delegate tenant-authorized reads", func(t *testing.T) {
		userID := "user-" + fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		transactionID := "transaction-" + fake.UUID().V4()
		partnerID := "transaction-partner-" + fake.UUID().V4()
		now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.FixedZone("controller", 2*60*60))
		service := newMockfinanceService(t)
		transferDetails := newMocktransferDetailService(t)
		candidate := domain.Transaction{
			ID: partnerID, TenantID: tenantID, AccountID: "account-" + fake.UUID().V4(),
			Source: domain.TransactionSourceManual, Status: domain.TransactionStatusPending,
			Kind: domain.TransactionKindRegular, AmountMinor: -123, Currency: "USD",
			Description: "candidate-" + fake.Lorem().Word(), EffectiveAt: now,
			CreatedAt: now, UpdatedAt: now,
		}
		transferDetails.EXPECT().ListTransferCandidates(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, params financepkg.ListTransferCandidatesParams) ([]domain.Transaction, error) {
				assert.Equal(t, userID, params.ActorUserID)
				assert.Equal(t, tenantID, params.TenantID)
				assert.Equal(t, transactionID, params.TransactionID)
				assert.True(t, now.Add(-time.Hour).Equal(params.EffectiveFrom))
				assert.True(t, now.Add(time.Hour).Equal(params.EffectiveBefore))
				assert.EqualValues(t, 20, params.Limit)
				assert.EqualValues(t, 1, params.Offset)
				return []domain.Transaction{candidate}, nil
			},
		).Once()
		transferDetails.EXPECT().GetTransferPartner(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, params financepkg.GetTransferPartnerParams) (domain.Transaction, error) {
				assert.Equal(t, userID, params.ActorUserID)
				assert.Equal(t, tenantID, params.TenantID)
				assert.Equal(t, transactionID, params.TransactionID)
				return candidate, nil
			},
		).Once()
		handler := newHandler(
			service,
			newMockbankConnectionService(t),
			makeAuthMiddleware(userID),
			withTransferDetails(transferDetails),
		)
		path := "/api/v1/finance/tenants/" + tenantID + "/transactions/" + transactionID

		candidates := httptest.NewRecorder()
		handler.ServeHTTP(candidates, newRequest(
			http.MethodGet,
			path+"/transfer-candidates?effectiveFrom="+url.QueryEscape(
				now.Add(-time.Hour).Format(time.RFC3339),
			)+"&effectiveBefore="+url.QueryEscape(
				now.Add(time.Hour).Format(time.RFC3339),
			)+"&limit=20&offset=1",
			"",
			true,
		))
		require.Equal(t, http.StatusOK, candidates.Code)
		assert.Equal(t, partnerID, decode(t, candidates)["items"].([]any)[0].(map[string]any)["id"])

		partner := httptest.NewRecorder()
		handler.ServeHTTP(partner, newRequest(http.MethodGet, path+"/transfer-partner", "", true))
		require.Equal(t, http.StatusOK, partner.Code)
		assert.Equal(t, partnerID, decode(t, partner)["id"])

		invalid := httptest.NewRecorder()
		handler.ServeHTTP(invalid, newRequest(http.MethodGet, path+"/transfer-candidates?limit=0", "", true))
		require.Equal(t, http.StatusBadRequest, invalid.Code)

		errorDetails := newMocktransferDetailService(t)
		errorDetails.EXPECT().
			ListTransferCandidates(mock.Anything, mock.Anything).
			Return(nil, financepkg.ErrInvalidTransferCandidateQuery).
			Once()
		errorDetails.EXPECT().
			GetTransferPartner(mock.Anything, mock.Anything).
			Return(domain.Transaction{}, financepkg.ErrTransferPartnerNotFound).
			Once()
		errorDetails.EXPECT().
			GetTransferPartner(mock.Anything, mock.Anything).
			Return(domain.Transaction{}, financepkg.ErrInvalidTransferPartner).
			Once()
		errorHandler := newHandler(
			service,
			newMockbankConnectionService(t),
			makeAuthMiddleware(userID),
			withTransferDetails(errorDetails),
		)
		for _, tc := range []struct {
			target string
			status int
		}{
			{target: path + "/transfer-candidates?effectiveFrom=" + url.QueryEscape(now.Add(-time.Hour).Format(time.RFC3339)) + "&effectiveBefore=" + url.QueryEscape(now.Add(time.Hour).Format(time.RFC3339)) + "&limit=20", status: http.StatusBadRequest},
			{target: path + "/transfer-partner", status: http.StatusNotFound},
			{target: path + "/transfer-partner", status: http.StatusConflict},
		} {
			response := httptest.NewRecorder()
			errorHandler.ServeHTTP(response, newRequest(http.MethodGet, tc.target, "", true))
			require.Equal(t, tc.status, response.Code)
		}
	})

	t.Run("transfer-pair routes map invalid link and unlink corrections", func(t *testing.T) {
		userID := "user-" + fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		body := `{"firstTransactionId":"transaction-first","secondTransactionId":"transaction-second"}`
		target := "/api/v1/finance/tenants/" + tenantID + "/transactions/transfer-links"

		linkService := newMockfinanceService(t)
		linkService.EXPECT().LinkTransfers(mock.Anything, mock.Anything).
			Return(financepkg.ErrInvalidTransferPair).Once()
		linkResponse := httptest.NewRecorder()
		newHandler(linkService, newMockbankConnectionService(t), makeAuthMiddleware(userID)).ServeHTTP(
			linkResponse,
			newRequest(http.MethodPost, target, body, true),
		)
		require.Equal(t, http.StatusBadRequest, linkResponse.Code)

		unlinkService := newMockfinanceService(t)
		unlinkService.EXPECT().UnlinkTransfers(mock.Anything, mock.Anything).
			Return(financepkg.ErrTransferNotLinked).Once()
		unlinkResponse := httptest.NewRecorder()
		newHandler(unlinkService, newMockbankConnectionService(t), makeAuthMiddleware(userID)).ServeHTTP(
			unlinkResponse,
			newRequest(http.MethodDelete, target, body, true),
		)
		require.Equal(t, http.StatusConflict, unlinkResponse.Code)
	})

	t.Run("transaction CSV preview returns 400 with an empty body for invalid input", func(t *testing.T) {
		userID := fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		service := newMockfinanceService(t)
		service.EXPECT().PreviewCSVImport(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, params financepkg.PreviewCSVImportParams) (financepkg.CSVImportPreview, error) {
				require.Equal(t, financepkg.CSVImportTypeTransactions, params.ImportType)
				return financepkg.CSVImportPreview{}, fmt.Errorf(
					"%w: currency %q must be one of USD, EUR, PLN, UAH",
					financepkg.ErrInvalidCSVImport,
					"gBp",
				)
			},
		).Once()

		resp := httptest.NewRecorder()
		newHandler(service, newMockbankConnectionService(t), makeAuthMiddleware(userID)).ServeHTTP(
			resp,
			newRequest(
				http.MethodPost,
				"/api/v1/finance/tenants/"+tenantID+"/imports/preview",
				`{"fileName":"invalid.csv","csv":"Date,Account,Category,Tags,Expense amount,Income amount,Currency\n29.05.26,Wallet,,,1,,gBp"}`,
				true,
			),
		)

		require.Equal(t, http.StatusBadRequest, resp.Code)
		assert.Empty(t, resp.Body.String())
	})

	t.Run("tenant routes delegate into finance service", func(t *testing.T) {
		userID := fake.UUID().V4()
		now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
		tenantID := "tenant-" + fake.UUID().V4()
		inviteID := "invite-" + fake.UUID().V4()
		inviteCode := "code-" + fake.UUID().V4()
		service := newMockfinanceService(t)
		directory := newMockuserDirectory(t)
		username := "user-" + fake.Internet().User()
		handler := newHandler(
			service,
			newMockbankConnectionService(t),
			makeAuthMiddleware(userID),
			withUserDirectory(directory),
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
				require.True(t, params.SeedDefaults)
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
		directory.EXPECT().LookupUsername(mock.Anything, userID).Return(username, true, nil).Twice()

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
				`{"name":"Household","displayCurrency":"USD","seedDefaults":true}`,
				true,
			),
		)
		require.Equal(t, http.StatusOK, createResp.Code)
		assert.Equal(t, tenantID, decode(t, createResp)["id"])

		missingChoiceResp := httptest.NewRecorder()
		handler.ServeHTTP(
			missingChoiceResp,
			newRequest(
				http.MethodPost,
				"/api/v1/finance/tenants",
				`{"name":"Household","displayCurrency":"USD"}`,
				true,
			),
		)
		assert.Equal(t, http.StatusBadRequest, missingChoiceResp.Code)

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
		membersPayload := decode(t, membersResp)["items"].([]any)
		require.Len(t, membersPayload, 1)
		assert.Equal(t, username, membersPayload[0].(map[string]any)["username"])

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

	t.Run("tenant members keep user ID when auth user is missing", func(t *testing.T) {
		userID := fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		service := newMockfinanceService(t)
		directory := newMockuserDirectory(t)
		service.EXPECT().ListTenantMembers(
			mock.Anything,
			financepkg.ListTenantMembersParams{ActorUserID: userID, TenantID: tenantID},
		).Return([]domain.TenantMember{{TenantID: tenantID, UserID: userID, JoinedAt: time.Now()}}, nil)
		directory.EXPECT().LookupUsername(mock.Anything, userID).Return("", false, nil)

		resp := httptest.NewRecorder()
		newHandler(
			service,
			newMockbankConnectionService(t),
			makeAuthMiddleware(userID),
			withUserDirectory(directory),
		).ServeHTTP(resp, newRequest(http.MethodGet, "/api/v1/finance/tenants/"+tenantID+"/members", "", true))

		require.Equal(t, http.StatusOK, resp.Code)
		item := decode(t, resp)["items"].([]any)[0].(map[string]any)
		assert.Equal(t, userID, item["userId"])
		assert.NotContains(t, item, "username")
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
				require.False(t, params.SeedDefaults)
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
				`{"name":"`+tenantName+`","displayCurrency":"USD","seedDefaults":false}`,
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

	t.Run("tenant update route returns no content and rejects invalid access or currency", func(t *testing.T) {
		userID := fake.UUID().V4()
		createdAt := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
		tenantID := "tenant-" + fake.UUID().V4()
		updatedName := "tenant-updated-" + fake.Lorem().Word()
		updatedTenant := domain.Tenant{
			ID:              tenantID,
			Name:            updatedName,
			DisplayCurrency: "PLN",
			CreatedAt:       createdAt,
			UpdatedAt:       createdAt.Add(24 * time.Hour),
		}

		t.Run("success", func(t *testing.T) {
			service := newMockfinanceService(t)
			handler := newHandler(
				service,
				newMockbankConnectionService(t),
				makeAuthMiddleware(userID),
			)

			service.EXPECT().UpdateTenant(mock.Anything, mock.Anything).RunAndReturn(
				func(_ context.Context, params financepkg.UpdateTenantParams) (domain.Tenant, error) {
					require.Equal(t, userID, params.ActorUserID)
					require.Equal(t, tenantID, params.TenantID)
					require.Equal(t, updatedName, params.Name)
					require.Equal(t, "PLN", params.DisplayCurrency)
					return updatedTenant, nil
				},
			).Once()

			resp := httptest.NewRecorder()
			handler.ServeHTTP(
				resp,
				newRequest(
					http.MethodPatch,
					"/api/v1/finance/tenants/"+tenantID,
					`{"name":"`+updatedName+`","displayCurrency":"PLN"}`,
					true,
				),
			)

			require.Equal(t, http.StatusNoContent, resp.Code)
			assert.Empty(t, resp.Body.String())
		})

		t.Run("non member returns unauthorized", func(t *testing.T) {
			service := newMockfinanceService(t)
			service.EXPECT().UpdateTenant(mock.Anything, mock.Anything).Return(
				domain.Tenant{},
				financepkg.ErrTenantAccessDenied,
			).Once()

			resp := httptest.NewRecorder()
			newHandler(
				service,
				newMockbankConnectionService(t),
				makeAuthMiddleware(userID),
			).ServeHTTP(
				resp,
				newRequest(
					http.MethodPatch,
					"/api/v1/finance/tenants/"+tenantID,
					`{"name":"`+updatedName+`","displayCurrency":"PLN"}`,
					true,
				),
			)

			require.Equal(t, http.StatusUnauthorized, resp.Code)
		})

		t.Run("invalid display currency returns bad request before controller logic", func(t *testing.T) {
			service := newMockfinanceService(t)
			resp := httptest.NewRecorder()
			newHandler(
				service,
				newMockbankConnectionService(t),
				makeAuthMiddleware(userID),
			).ServeHTTP(
				resp,
				newRequest(
					http.MethodPatch,
					"/api/v1/finance/tenants/"+tenantID,
					`{"name":"`+updatedName+`","displayCurrency":"BTC"}`,
					true,
				),
			)

			require.Equal(t, http.StatusBadRequest, resp.Code)
		})
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
				require.Equal(t, int64(20), params.Limit)
				require.Equal(t, int64(5), params.Offset)
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
				require.NotNil(t, params.EffectiveAt)
				require.False(t, params.ClearCategory)
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
					EffectiveAt: *params.EffectiveAt,
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
			{method: http.MethodGet, target: "/api/v1/finance/tenants/" + tenantID + "/transactions?accountId=" + accountID + "&source=manual&status=booked&includeHidden=true&limit=20&offset=5", field: "items", want: 1},
			{method: http.MethodGet, target: "/api/v1/finance/tenants/" + tenantID + "/transactions/" + transactionID, field: "id", want: transactionID},
			{method: http.MethodPost, target: "/api/v1/finance/tenants/" + tenantID + "/transactions", body: `{"accountId":"` + accountID + `","source":"manual","status":"booked","kind":"regular","amountMinor":-2500,"currency":"USD","description":"Coffee","effectiveAt":"2026-06-20T14:00:00Z","categoryId":"` + categoryID + `","transferGroupId":"group-1"}`, field: "id", want: transactionID},
			{method: http.MethodPatch, target: "/api/v1/finance/tenants/" + tenantID + "/transactions/" + transactionID, body: `{"description":"Coffee update","amountMinor":-3100,"effectiveAt":"2026-06-21T10:00:00Z","categoryId":"` + updatedCategoryID + `","tagIds":[]}`, field: "id", want: transactionID},
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
				require.True(t, params.ClearCategory)
				require.Empty(t, params.CategoryID)
				require.NotNil(t, params.EffectiveAt)
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
					EffectiveAt: *params.EffectiveAt,
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
				`{"description":"Coffee cleared","amountMinor":-3200,"effectiveAt":"2026-06-21T11:00:00Z","clearCategory":true,"tagIds":[]}`,
				true,
			),
		)
		require.Equal(t, http.StatusOK, nullCategoryResp.Code)
		assert.Equal(t, transactionID, decode(t, nullCategoryResp)["id"])

		serviceForOmittedCategory := newMockfinanceService(t)
		handlerForOmittedCategory := newHandler(
			serviceForOmittedCategory,
			newMockbankConnectionService(t),
			makeAuthMiddleware(userID),
		)
		serviceForOmittedCategory.EXPECT().
			UpdateTransaction(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, params financepkg.UpdateTransactionParams) (domain.Transaction, error) {
				require.False(t, params.ClearCategory)
				require.Empty(t, params.CategoryID)
				require.NotNil(t, params.EffectiveAt)
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
					EffectiveAt: *params.EffectiveAt,
					CategoryID:  &categoryID,
					CreatedAt:   now,
					UpdatedAt:   now,
				}, nil
			})

		omittedCategoryResp := httptest.NewRecorder()
		handlerForOmittedCategory.ServeHTTP(
			omittedCategoryResp,
			newRequest(
				http.MethodPatch,
				"/api/v1/finance/tenants/"+tenantID+"/transactions/"+transactionID,
				`{"description":"Coffee purchase","amountMinor":-3300,"effectiveAt":"2026-06-21T12:00:00Z","tagIds":[]}`,
				true,
			),
		)
		require.Equal(t, http.StatusOK, omittedCategoryResp.Code)
		assert.Equal(t, categoryID, decode(t, omittedCategoryResp)["categoryId"])

		contradictoryCategoryResp := httptest.NewRecorder()
		newHandler(
			newMockfinanceService(t),
			newMockbankConnectionService(t),
			makeAuthMiddleware(userID),
		).ServeHTTP(
			contradictoryCategoryResp,
			newRequest(
				http.MethodPatch,
				"/api/v1/finance/tenants/"+tenantID+"/transactions/"+transactionID,
				`{"description":"contradictory","amountMinor":-3300,"categoryId":"`+categoryID+`","clearCategory":true,"tagIds":[]}`,
				true,
			),
		)
		require.Equal(t, http.StatusBadRequest, contradictoryCategoryResp.Code)

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
			now := time.Date(2026, time.June, 21, 9, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60))
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
				UpdateBankConnection(mock.Anything, financepkg.UpdateBankConnectionParams{
					ActorUserID: userID, TenantID: tenantID, ConnectionID: connectionID, Name: "Renamed connection",
				}).
				Return(nil)
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
						ProviderReference: "ref-redirect",
						State:             domain.BankConnectionStateActive,
						CreatedAt:         now,
						UpdatedAt:         now,
					}, nil
				})
			windowStart := time.Date(2026, time.June, 20, 9, 0, 0, 0, time.UTC)
			windowEnd := time.Date(2026, time.June, 21, 9, 0, 0, 0, time.UTC)
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
					require.Equal(t, "2026-06-01T00:00:00Z", params.StartDate.Format(time.RFC3339))
					require.Equal(t, "2026-06-30T00:00:00Z", params.EndDate.Format(time.RFC3339))
					return financepkg.Dashboard{
						Period: financepkg.DashboardPeriod{
							Preset:    params.Preset,
							StartDate: params.StartDate,
							EndDate:   params.EndDate,
							Previous: financepkg.DashboardPeriodWindow{
								StartDate: params.StartDate,
								EndDate:   params.EndDate,
							},
							Next: financepkg.DashboardPeriodWindow{
								StartDate: params.StartDate,
								EndDate:   params.EndDate,
							},
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
				TriggerFXRefresh(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, params financepkg.TriggerFXRefreshParams) (financepkg.FXRefreshJobRef, error) {
					require.Equal(t, userID, params.RequestedByUserID)
					return financepkg.FXRefreshJobRef{
						ID:       "job-fx-1",
						JobType:  financepkg.FXRefreshJobType,
						Provider: "nbp",
					}, nil
				})
			service.EXPECT().
				PreviewCSVImport(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, params financepkg.PreviewCSVImportParams) (financepkg.CSVImportPreview, error) {
					require.Equal(t, financepkg.CSVImportTypeTransactions, params.ImportType)
					require.Equal(t, "demo.csv", params.FileName)
					require.Equal(t, []string{"Checking"}, params.SelectedAccountNames)
					return financepkg.CSVImportPreview{
						ImportID:        importID,
						ImportType:      params.ImportType,
						ImportableCount: 0,
						Headers: []string{
							"Date",
							"Account",
							"Category",
							"Tags",
							"Expense amount",
							"Income amount",
							"Currency",
							"Description",
						},
						RejectedRows: []financepkg.CSVImportRejectedRow{{
							RowNumber: 3,
							Field:     "currency",
							Reason:    "currency must be one of USD, EUR, PLN, UAH",
						}},
						WouldCreateAccounts: []string{"Checking"},
						AccountOptions: []financepkg.CSVImportAccountOption{{
							Name:           "Checking",
							SourceRowCount: 1,
							Selected:       true,
						}},
					}, nil
				})
			service.EXPECT().
				ConfirmCSVImport(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, params financepkg.ConfirmCSVImportParams) (financepkg.CSVImportConfirmation, error) {
					require.Equal(t, importID, params.ImportID)
					require.Equal(t, financepkg.CSVImportTypeTransactions, params.ExpectedImportType)
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
						RejectedRows: []financepkg.CSVImportRejectedRow{{
							RowNumber: 3,
							Field:     "currency",
							Reason:    "currency must be one of USD, EUR, PLN, UAH",
						}},
						RowOutcomes: []domain.CSVImportRowOutcome{{
							RowNumber: 2,
							Status:    domain.CSVImportRowOutcomeImported,
							CreatedAt: now,
							UpdatedAt: now,
						}},
						CreatedAt:   now,
						ConfirmedAt: &confirmedAt,
						CompletedAt: &completedAt,
					}, nil
				})
			service.EXPECT().
				ListRecentCSVImportAudits(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, params financepkg.ListRecentCSVImportAuditsParams) ([]financepkg.CSVImportAudit, error) {
					require.Equal(t, userID, params.ActorUserID)
					require.Equal(t, tenantID, params.TenantID)
					require.Equal(t, financepkg.CSVImportTypeTransactions, params.ExpectedImportType)
					return []financepkg.CSVImportAudit{{
						ImportID:          importID,
						TenantID:          tenantID,
						ImportType:        financepkg.CSVImportTypeTransactions,
						Status:            financepkg.CSVImportStatusCompleted,
						JobID:             "job-import-1",
						ConfirmedByUserID: userID,
						ImportedCount:     4,
						RejectedRows:      []financepkg.CSVImportRejectedRow{},
						RowOutcomes:       []domain.CSVImportRowOutcome{},
						CreatedAt:         now,
					}}, nil
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
					method: http.MethodPatch,
					target: "/api/v1/finance/tenants/" + tenantID + "/connections/" + connectionID,
					body:   `{"name":"Renamed connection"}`,
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
					target: "/api/v1/finance/tenants/" + tenantID + "/dashboard?preset=current_month&startDate=2026-06-01T00:00:00Z&endDate=2026-06-30T00:00:00Z",
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
					body:   `{"provider":"nbp"}`,
					field:  "jobId",
					want:   "job-fx-1",
					status: http.StatusOK,
				},
				{
					method: http.MethodPost,
					target: "/api/v1/finance/tenants/" + tenantID + "/imports/preview",
					body:   `{"fileName":"demo.csv","csv":"Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description\n29.05.26,Checking,,,1,,USD,Coffee","selectedAccountNames":["Checking"]}`,
					field:  "importId",
					want:   importID,
					status: http.StatusOK,
				},
				{
					method: http.MethodPost,
					target: "/api/v1/finance/tenants/" + tenantID + "/imports/" + importID + "/confirm",
					body:   `{}`,
					field:  "jobId",
					want:   "job-import-1",
					status: http.StatusOK,
				},
				{
					method: http.MethodGet,
					target: "/api/v1/finance/tenants/" + tenantID + "/imports",
					field:  "items",
					want:   1,
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
				if tc.target == "/api/v1/finance/tenants/"+tenantID+"/imports/preview" {
					assert.InDelta(t, 0, payload["importableCount"], 0)
					assert.Equal(t, "Checking", payload["accountOptions"].([]any)[0].(map[string]any)["name"])
				}
				if tc.field == "items" {
					assert.Len(t, payload[tc.field].([]any), tc.want.(int))
					continue
				}
				if tc.field == "settled" {
					assert.Contains(t, payload, tc.field)
					continue
				}
				if tc.field == "importedCount" {
					assert.Equal(t, now.Format(time.RFC3339), payload["createdAt"])
				}
				if tc.field == "authorizationUrl" {
					assert.Equal(t, tc.want, payload[tc.field])
					assert.Equal(t, startState, payload["state"])
					continue
				}
				assert.Equal(t, tc.want, payload[tc.field])
			}

			t.Run("FX refresh ignores obsolete pair input", func(t *testing.T) {
				refreshService := newMockfinanceService(t)
				refreshService.EXPECT().
					TriggerFXRefresh(mock.Anything, financepkg.TriggerFXRefreshParams{
						RequestedByUserID: userID,
						Source:            financepkg.FXSyncRequesterSourceOperator,
						Provider:          "nbp",
					}).
					Return(financepkg.FXRefreshJobRef{
						ID: "job-" + fake.UUID().V4(), JobType: financepkg.FXRefreshJobType, Provider: "nbp",
					}, nil)
				refreshHandler := newHandler(
					refreshService,
					newMockbankConnectionService(t),
					makeAuthMiddleware(userID),
				)
				resp := httptest.NewRecorder()
				refreshHandler.ServeHTTP(resp, newRequest(
					http.MethodPost,
					"/api/v1/finance/fx/sync",
					`{"provider":"nbp","baseCurrencies":["EUR"],"quoteCurrency":"USD"}`,
					true,
				))
				require.Equal(t, http.StatusOK, resp.Code)
			})

			t.Run("CSV confirm uses the fixed transaction contract without mapping", func(t *testing.T) {
				confirmationService := newMockfinanceService(t)
				confirmationService.EXPECT().ConfirmCSVImport(mock.Anything, mock.Anything).RunAndReturn(
					func(_ context.Context, params financepkg.ConfirmCSVImportParams) (financepkg.CSVImportConfirmation, error) {
						require.Equal(t, financepkg.CSVImportTypeTransactions, params.ExpectedImportType)
						return financepkg.CSVImportConfirmation{
							ImportID: importID,
							JobID:    "job-confirm",
							JobType:  financepkg.CSVImportJobTypeTransactions,
						}, nil
					},
				)
				resp := httptest.NewRecorder()
				newHandler(
					confirmationService,
					newMockbankConnectionService(t),
					makeAuthMiddleware(userID),
				).ServeHTTP(resp, newRequest(
					http.MethodPost,
					"/api/v1/finance/tenants/"+tenantID+"/imports/"+importID+"/confirm",
					`{}`,
					true,
				))
				require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
			})

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

	t.Run("registered connection rename route maps access not found and invalid names", func(t *testing.T) {
		userID := "user-" + fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		connectionID := "connection-" + fake.UUID().V4()
		target := "/api/v1/finance/tenants/" + tenantID + "/connections/" + connectionID

		for _, tc := range []struct {
			name   string
			err    error
			status int
		}{
			{name: "tenant access denied", err: financepkg.ErrTenantAccessDenied, status: http.StatusUnauthorized},
			{name: "connection not found", err: financepkg.ErrBankConnectionNotFound, status: http.StatusNotFound},
			{name: "blank name", err: financepkg.ErrBankConnectionNameRequired, status: http.StatusBadRequest},
		} {
			t.Run(tc.name, func(t *testing.T) {
				name := "Renamed " + fake.Lorem().Word()
				bankConnections := newMockbankConnectionService(t)
				bankConnections.EXPECT().UpdateBankConnection(
					mock.Anything,
					financepkg.UpdateBankConnectionParams{
						ActorUserID:  userID,
						TenantID:     tenantID,
						ConnectionID: connectionID,
						Name:         name,
					},
				).Return(tc.err).Once()

				resp := httptest.NewRecorder()
				newHandler(
					newMockfinanceService(t),
					bankConnections,
					makeAuthMiddleware(userID),
				).ServeHTTP(resp, newRequest(http.MethodPatch, target, `{"name":`+fmt.Sprintf("%q", name)+`}`, true))
				require.Equal(t, tc.status, resp.Code)
				assert.Empty(t, resp.Body.String())
			})
		}
	})

	t.Run("redirect finish keeps provider-specific code validation", func(t *testing.T) {
		userID := fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		syntheticState := "state-" + fake.UUID().V4()
		connectionID := "connection-" + fake.UUID().V4()
		service := newMockfinanceService(t)
		bankConnections := newMockbankConnectionService(t)
		bankConnections.EXPECT().
			FinishBankConnectionLink(mock.Anything, financepkg.FinishBankConnectionLinkParams{
				ActorUserID: userID,
				TenantID:    tenantID,
				Provider:    "pko",
				State:       "state-pko",
				Code:        "",
			}).
			Return(domain.BankConnection{}, app.NewErrInvalidInput("code", "code is required"))
		bankConnections.EXPECT().
			FinishBankConnectionLink(mock.Anything, financepkg.FinishBankConnectionLinkParams{
				ActorUserID: userID,
				TenantID:    tenantID,
				Provider:    "synthetic",
				State:       syntheticState,
				Code:        "",
			}).
			Return(domain.BankConnection{
				ID:                connectionID,
				TenantID:          tenantID,
				Provider:          "synthetic",
				DisplayName:       "Synthetic",
				ProviderReference: syntheticState,
				State:             domain.BankConnectionStateActive,
				CreatedAt:         time.Now().UTC(),
				UpdatedAt:         time.Now().UTC(),
			}, nil)

		handler := newHandler(service, bankConnections, makeAuthMiddleware(userID))

		pkoResp := httptest.NewRecorder()
		handler.ServeHTTP(
			pkoResp,
			newRequest(
				http.MethodPost,
				"/api/v1/finance/tenants/"+tenantID+"/connections/link-redirect/finish",
				`{"provider":"pko","state":"state-pko"}`,
				true,
			),
		)
		require.Equal(t, http.StatusBadRequest, pkoResp.Code)

		syntheticResp := httptest.NewRecorder()
		handler.ServeHTTP(
			syntheticResp,
			newRequest(
				http.MethodPost,
				"/api/v1/finance/tenants/"+tenantID+"/connections/link-redirect/finish",
				`{"provider":"synthetic","state":"`+syntheticState+`"}`,
				true,
			),
		)
		require.Equal(t, http.StatusOK, syntheticResp.Code)
		assert.Equal(t, syntheticState, decode(t, syntheticResp)["providerReference"])
	})

	t.Run("registered dashboard route preserves timestamp and nullable contract values", func(t *testing.T) {
		userID := "user-" + fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		accountID := "account-" + fake.UUID().V4()
		date := time.Date(
			2026, time.March, 29, 13, 47, 11, 123,
			time.FixedZone("UTC+14", 14*60*60),
		)
		zero := int64(0)
		service := newMockfinanceService(t)
		service.EXPECT().GetDashboard(mock.Anything, mock.Anything).Return(financepkg.Dashboard{
			Period: financepkg.DashboardPeriod{
				Preset:    financepkg.DashboardPeriodPresetCurrentMonth,
				StartDate: date,
				EndDate:   date,
				Previous:  financepkg.DashboardPeriodWindow{StartDate: date, EndDate: date},
				Next:      financepkg.DashboardPeriodWindow{StartDate: date, EndDate: date},
			},
			Settled: financepkg.DashboardMoneySummary{DisplayCurrency: "USD", Complete: false},
			Pending: financepkg.DashboardMoneySummary{DisplayCurrency: "USD", Complete: false},
			AccountBalances: []financepkg.DashboardAccountBalance{{
				AccountID: accountID, AccountName: "account-" + fake.Lorem().Word(), Currency: "USD",
				DisplayPendingMinor: &zero, MissingFX: false,
			}},
			FXCoverage: []financepkg.DashboardFXCoverage{{
				BaseCurrency: "EUR", QuoteCurrency: "USD", Provider: "frankfurter", AffectedAccountCount: 1,
			}},
		}, nil)
		handler := newHandler(service, newMockbankConnectionService(t), makeAuthMiddleware(userID))

		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, newRequest(
			http.MethodGet,
			"/api/v1/finance/tenants/"+tenantID+"/dashboard?preset=current_month",
			"",
			true,
		))

		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		payload := decode(t, resp)
		period := payload["period"].(map[string]any)
		assert.Equal(t, "2026-03-29T13:47:11.000000123+14:00", period["startDate"])
		balance := payload["accountBalances"].([]any)[0].(map[string]any)
		assert.Contains(t, balance, "displayBookedMinor")
		assert.Nil(t, balance["displayBookedMinor"])
		assert.Zero(t, balance["displayPendingMinor"])
		assert.Equal(t, false, balance["missingFx"])
		coverage := payload["fxCoverage"].([]any)[0].(map[string]any)
		assert.Equal(t, "EUR", coverage["baseCurrency"])
		assert.InDelta(t, 1, coverage["affectedAccountCount"], 0)
	})

	t.Run("registered dashboard route accepts next month preset", func(t *testing.T) {
		userID := "user-" + fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		service := newMockfinanceService(t)
		service.EXPECT().
			GetDashboard(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, params financepkg.DashboardParams) (financepkg.Dashboard, error) {
				require.Equal(t, financepkg.DashboardPeriodPresetNextMonth, params.Preset)
				return financepkg.Dashboard{
					Period: financepkg.DashboardPeriod{
						Preset:   params.Preset,
						Previous: financepkg.DashboardPeriodWindow{},
						Next:     financepkg.DashboardPeriodWindow{},
					},
				}, nil
			})
		resp := httptest.NewRecorder()
		newHandler(service, newMockbankConnectionService(t), makeAuthMiddleware(userID)).ServeHTTP(
			resp,
			newRequest(
				http.MethodGet,
				"/api/v1/finance/tenants/"+tenantID+"/dashboard?preset=next_month",
				"",
				true,
			),
		)
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	})

	t.Run("nullable finance responses omit absent state and preserve present state", func(t *testing.T) {
		userID := "user-" + fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		presentAt := fake.Time().Recent().In(time.FixedZone("response-offset", 5*60*60+30*60)).Truncate(time.Second)
		service := newMockfinanceService(t)
		handler := newHandler(service, newMockbankConnectionService(t), makeAuthMiddleware(userID))

		makeTenantView := func(id string, archivedAt *time.Time) domain.TenantMembershipView {
			return domain.TenantMembershipView{
				Tenant: domain.Tenant{
					ID: id, Name: "tenant-" + fake.Lorem().Word(), DisplayCurrency: "USD",
					ArchivedAt: archivedAt, CreatedAt: presentAt, UpdatedAt: presentAt,
				},
				Membership: domain.TenantMembership{TenantID: id, UserID: userID, JoinedAt: presentAt},
			}
		}
		activeTenant := makeTenantView(tenantID, nil)
		archivedTenant := makeTenantView("tenant-archived-"+fake.UUID().V4(), &presentAt)
		service.EXPECT().ListTenantsForUser(mock.Anything, userID).
			Return([]domain.TenantMembershipView{activeTenant, archivedTenant}, nil).Twice()

		acceptedByUserID := "user-accepted-" + fake.UUID().V4()
		service.EXPECT().ListTenantInvites(mock.Anything, mock.Anything).Return([]domain.TenantInvite{
			{
				ID: "invite-pending-" + fake.UUID().V4(), TenantID: tenantID, Code: "code-" + fake.UUID().V4(),
				Recipient: fake.Internet().Email(), CreatedByUserID: userID, CreatedAt: presentAt,
			},
			{
				ID: "invite-accepted-" + fake.UUID().V4(), TenantID: tenantID, Code: "code-" + fake.UUID().V4(),
				Recipient: fake.Internet().Email(), CreatedByUserID: userID, AcceptedByUserID: &acceptedByUserID,
				CreatedAt: presentAt, AcceptedAt: &presentAt,
			},
		}, nil)

		service.EXPECT().ListAccounts(mock.Anything, mock.Anything).Return([]domain.Account{
			{
				ID: "account-active-" + fake.UUID().V4(), TenantID: tenantID, Name: "account-" + fake.Lorem().Word(),
				Currency: "USD", Kind: domain.AccountKindManual, CreatedAt: presentAt, UpdatedAt: presentAt,
			},
			{
				ID: "account-hidden-" + fake.UUID().V4(), TenantID: tenantID, Name: "account-" + fake.Lorem().Word(),
				Currency: "USD", Kind: domain.AccountKindManual, HiddenAt: &presentAt,
				CreatedAt: presentAt, UpdatedAt: presentAt,
			},
		}, nil)
		service.EXPECT().ListCategories(mock.Anything, mock.Anything).Return([]domain.Category{
			{
				ID: "category-active-" + fake.UUID().V4(), TenantID: tenantID, Name: "category-" + fake.Lorem().Word(),
				Kind: domain.CategoryKindExpense, CreatedAt: presentAt, UpdatedAt: presentAt,
			},
			{
				ID: "category-hidden-" + fake.UUID().V4(), TenantID: tenantID, Name: "category-" + fake.Lorem().Word(),
				Kind: domain.CategoryKindExpense, HiddenAt: &presentAt, CreatedAt: presentAt, UpdatedAt: presentAt,
			},
		}, nil)
		service.EXPECT().ListTags(mock.Anything, mock.Anything).Return([]domain.Tag{
			{
				ID: "tag-active-" + fake.UUID().V4(), TenantID: tenantID, Name: "tag-" + fake.Lorem().Word(),
				CreatedAt: presentAt, UpdatedAt: presentAt,
			},
			{
				ID: "tag-hidden-" + fake.UUID().V4(), TenantID: tenantID, Name: "tag-" + fake.Lorem().Word(),
				HiddenAt: &presentAt, CreatedAt: presentAt, UpdatedAt: presentAt,
			},
		}, nil)

		categoryID := "category-" + fake.UUID().V4()
		transferGroupID := "transfer-group-" + fake.UUID().V4()
		makeTransaction := func(id string) domain.Transaction {
			return domain.Transaction{
				ID: id, TenantID: tenantID, AccountID: "account-" + fake.UUID().V4(),
				Source: domain.TransactionSourceProvider, Status: domain.TransactionStatusBooked,
				Kind: domain.TransactionKindRegular, AmountMinor: -1234, Currency: "USD",
				Description: "transaction-" + fake.Lorem().Word(), EffectiveAt: presentAt,
				CreatedAt: presentAt, UpdatedAt: presentAt,
			}
		}
		activeTransaction := makeTransaction("transaction-active-" + fake.UUID().V4())
		activeTransaction.ProviderOriginal = &domain.ProviderTransactionOriginal{
			AmountMinor: -1200, Currency: "USD", Description: "provider-" + fake.Lorem().Word(),
		}
		hiddenTransaction := makeTransaction("transaction-hidden-" + fake.UUID().V4())
		hiddenTransaction.CategoryID = &categoryID
		hiddenTransaction.TransferGroupID = &transferGroupID
		hiddenTransaction.TransferMatchedAt = &presentAt
		hiddenTransaction.HiddenAt = &presentAt
		hiddenTransaction.ProviderOriginal = &domain.ProviderTransactionOriginal{
			AmountMinor: -1200,
			Currency:    "USD",
			Description: "provider-" + fake.Lorem().Word(),
			EffectiveAt: &presentAt,
		}
		service.EXPECT().ListTransactions(mock.Anything, mock.Anything).
			Return([]domain.Transaction{activeTransaction, hiddenTransaction}, nil)

		makeConnection := func(id string) domain.BankConnection {
			return domain.BankConnection{
				ID: id, TenantID: tenantID, Provider: "provider-" + fake.Lorem().Word(),
				DisplayName: "connection-" + fake.Lorem().Word(), ProviderReference: "reference-" + fake.UUID().V4(),
				CreatedAt: presentAt, UpdatedAt: presentAt,
			}
		}
		unsyncedConnection := makeConnection("connection-unsynced-" + fake.UUID().V4())
		syncedConnection := makeConnection("connection-synced-" + fake.UUID().V4())
		syncedConnection.LastSyncStartedAt = &presentAt
		syncedConnection.LastSuccessfulSyncAt = &presentAt
		service.EXPECT().ListBankConnections(mock.Anything, mock.Anything).Return([]financepkg.BankConnectionView{
			{
				Connection: unsyncedConnection,
				Schedule: &domain.BankConnectionSchedule{
					ConnectionID: unsyncedConnection.ID,
					Interval:     time.Hour,
					CreatedAt:    presentAt,
					UpdatedAt:    presentAt,
				},
			},
			{
				Connection: syncedConnection,
				Schedule: &domain.BankConnectionSchedule{
					ConnectionID: syncedConnection.ID, Interval: time.Hour, NextRunAt: &presentAt,
					LastScheduledAt: &presentAt, LastStartedAt: &presentAt, LastCompletedAt: &presentAt,
					Enabled: true, CreatedAt: presentAt, UpdatedAt: presentAt,
				},
			},
		}, nil)

		previewImportID := "import-preview-" + fake.UUID().V4()
		confirmedImportID := "import-confirmed-" + fake.UUID().V4()
		completedImportID := "import-completed-" + fake.UUID().V4()
		service.EXPECT().GetCSVImportAudit(mock.Anything, mock.Anything).
			RunAndReturn(func(
				_ context.Context,
				params financepkg.GetCSVImportAuditParams,
			) (financepkg.CSVImportAudit, error) {
				result := financepkg.CSVImportAudit{
					ImportID:          params.ImportID,
					TenantID:          tenantID,
					ImportType:        financepkg.CSVImportTypeTransactions,
					JobID:             "job-" + fake.UUID().V4(),
					ConfirmedByUserID: userID,
					CreatedAt:         presentAt,
				}
				switch params.ImportID {
				case completedImportID:
					result.Status = financepkg.CSVImportStatusCompleted
					result.ConfirmedAt = &presentAt
					result.CompletedAt = &presentAt
				case confirmedImportID:
					result.Status = financepkg.CSVImportStatusConfirmed
					result.ConfirmedAt = &presentAt
				default:
					result.Status = financepkg.CSVImportStatusPreviewed
				}
				return result, nil
			}).
			Times(3)

		requestPayload := func(target string) map[string]any {
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, newRequest(http.MethodGet, target, "", true))
			require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
			require.NotContains(t, resp.Body.String(), "0001-01-01")
			return decode(t, resp)
		}
		itemAt := func(payload map[string]any, index int) map[string]any {
			items, ok := payload["items"].([]any)
			require.True(t, ok)
			item, ok := items[index].(map[string]any)
			require.True(t, ok)
			return item
		}
		assertAbsent := func(payload map[string]any, fields ...string) {
			for _, field := range fields {
				assert.NotContains(t, payload, field)
			}
		}
		assertPresentAt := func(payload map[string]any, fields ...string) {
			for _, field := range fields {
				assert.Equal(t, presentAt.Format(time.RFC3339), payload[field])
			}
		}

		tenantPayload := requestPayload("/api/v1/finance/tenants")
		assertAbsent(itemAt(tenantPayload, 0), "archivedAt")
		assertPresentAt(itemAt(tenantPayload, 1), "archivedAt")
		assertAbsent(requestPayload("/api/v1/finance/tenants/"+tenantID), "archivedAt")

		invitePayload := requestPayload("/api/v1/finance/tenants/" + tenantID + "/invites")
		assertAbsent(itemAt(invitePayload, 0), "acceptedAt", "acceptedByUserId")
		assertPresentAt(itemAt(invitePayload, 1), "acceptedAt")
		assert.Equal(t, acceptedByUserID, itemAt(invitePayload, 1)["acceptedByUserId"])

		for _, route := range []string{"accounts", "categories", "tags"} {
			payload := requestPayload("/api/v1/finance/tenants/" + tenantID + "/" + route + "?includeHidden=true")
			assertAbsent(itemAt(payload, 0), "hiddenAt")
			assertPresentAt(itemAt(payload, 1), "hiddenAt")
		}

		transactionPayload := requestPayload(
			"/api/v1/finance/tenants/" + tenantID + "/transactions?includeHidden=true&limit=20",
		)
		activeTransactionPayload := itemAt(transactionPayload, 0)
		assertAbsent(activeTransactionPayload, "categoryId", "transferGroupId", "transferMatchedAt", "hiddenAt")
		assertAbsent(activeTransactionPayload["providerOriginal"].(map[string]any), "effectiveAt")
		hiddenTransactionPayload := itemAt(transactionPayload, 1)
		assertPresentAt(hiddenTransactionPayload, "transferMatchedAt", "hiddenAt")
		assert.Equal(t, categoryID, hiddenTransactionPayload["categoryId"])
		assert.Equal(t, transferGroupID, hiddenTransactionPayload["transferGroupId"])
		assertPresentAt(hiddenTransactionPayload["providerOriginal"].(map[string]any), "effectiveAt")

		connectionPayload := requestPayload("/api/v1/finance/tenants/" + tenantID + "/connections")
		unsyncedPayload := itemAt(connectionPayload, 0)
		assertAbsent(unsyncedPayload, "lastSyncStartedAt", "lastSuccessfulSyncAt")
		assertAbsent(
			unsyncedPayload["schedule"].(map[string]any),
			"nextRunAt", "lastScheduledAt", "lastStartedAt", "lastCompletedAt",
		)
		syncedPayload := itemAt(connectionPayload, 1)
		assertPresentAt(syncedPayload, "lastSyncStartedAt", "lastSuccessfulSyncAt")
		assertPresentAt(
			syncedPayload["schedule"].(map[string]any),
			"nextRunAt", "lastScheduledAt", "lastStartedAt", "lastCompletedAt",
		)

		previewPayload := requestPayload(
			"/api/v1/finance/tenants/" + tenantID + "/imports/" + previewImportID,
		)
		assertAbsent(previewPayload, "confirmedAt", "completedAt")
		confirmedPayload := requestPayload(
			"/api/v1/finance/tenants/" + tenantID + "/imports/" + confirmedImportID,
		)
		assertPresentAt(confirmedPayload, "confirmedAt")
		assertAbsent(confirmedPayload, "completedAt")
		completedPayload := requestPayload(
			"/api/v1/finance/tenants/" + tenantID + "/imports/" + completedImportID,
		)
		assertPresentAt(completedPayload, "confirmedAt", "completedAt")
	})

	t.Run("registered finance routes reject zero timestamps and preserve fixed offsets", func(t *testing.T) {
		userID := "user-" + fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		accountID := "account-" + fake.UUID().V4()
		validAt := time.Date(2026, time.July, 10, 21, 30, 0, 123, time.FixedZone("request-offset", 5*60*60+30*60))
		zero := time.Time{}

		for _, testCase := range []struct {
			name    string
			account domain.Account
		}{
			{
				name: "nullable timestamp",
				account: domain.Account{
					ID: accountID, TenantID: tenantID, Name: fake.Company().Name(), Currency: "USD",
					Kind: domain.AccountKindManual, HiddenAt: &zero, CreatedAt: validAt, UpdatedAt: validAt,
				},
			},
			{
				name: "required timestamp",
				account: domain.Account{
					ID: accountID, TenantID: tenantID, Name: fake.Company().Name(), Currency: "USD",
					Kind: domain.AccountKindManual, CreatedAt: zero, UpdatedAt: validAt,
				},
			},
		} {
			t.Run("rejects corrupt "+testCase.name, func(t *testing.T) {
				corruptService := newMockfinanceService(t)
				corruptService.EXPECT().ListAccounts(mock.Anything, mock.Anything).
					Return([]domain.Account{testCase.account}, nil)
				corruptResponse := httptest.NewRecorder()
				newHandler(
					corruptService,
					newMockbankConnectionService(t),
					makeAuthMiddleware(userID),
				).ServeHTTP(
					corruptResponse,
					newRequest(
						http.MethodGet,
						"/api/v1/finance/tenants/"+tenantID+"/accounts?includeHidden=true",
						"",
						true,
					),
				)
				require.Equal(t, http.StatusInternalServerError, corruptResponse.Code, corruptResponse.Body.String())
				require.NotContains(t, corruptResponse.Body.String(), "0001-01-01")
			})
		}

		validService := newMockfinanceService(t)
		validService.EXPECT().RecordTransaction(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, params financepkg.RecordTransactionParams) (domain.Transaction, error) {
				require.Equal(t, int64(-1), params.AmountMinor)
				require.Equal(t, validAt.Format(time.RFC3339Nano), params.EffectiveAt.Format(time.RFC3339Nano))
				return domain.Transaction{
					ID: "transaction-" + fake.UUID().V4(), TenantID: tenantID, AccountID: accountID,
					Source: domain.TransactionSourceManual, Status: domain.TransactionStatusBooked,
					Kind: domain.TransactionKindRegular, Currency: "USD", Description: fake.Lorem().Sentence(2),
					EffectiveAt: params.EffectiveAt, CreatedAt: validAt, UpdatedAt: validAt,
				}, nil
			})
		validResponse := httptest.NewRecorder()
		validBody := `{"accountId":"` + accountID +
			`","source":"manual","status":"booked","kind":"regular",` +
			`"amountMinor":-1,"currency":"USD","description":"valid amount",` +
			`"effectiveAt":"` + validAt.Format(time.RFC3339Nano) + `"}`
		newHandler(
			validService,
			newMockbankConnectionService(t),
			makeAuthMiddleware(userID),
		).ServeHTTP(
			validResponse,
			newRequest(
				http.MethodPost,
				"/api/v1/finance/tenants/"+tenantID+"/transactions",
				validBody,
				true,
			),
		)
		require.Equal(t, http.StatusOK, validResponse.Code, validResponse.Body.String())

		zeroResponse := httptest.NewRecorder()
		newHandler(
			newMockfinanceService(t),
			newMockbankConnectionService(t),
			makeAuthMiddleware(userID),
		).ServeHTTP(
			zeroResponse,
			newRequest(
				http.MethodPost,
				"/api/v1/finance/tenants/"+tenantID+"/transactions",
				`{"accountId":"`+accountID+`","source":"manual","status":"booked","kind":"regular","amountMinor":0,"currency":"USD","description":"zero timestamp","effectiveAt":"0001-01-01T00:00:00Z"}`,
				true,
			),
		)
		require.Equal(t, http.StatusBadRequest, zeroResponse.Code, zeroResponse.Body.String())
	})

	t.Run("registered optional finance request timestamps preserve presence and validity", func(t *testing.T) {
		userID := "user-" + fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		transactionID := "transaction-" + fake.UUID().V4()
		connectionID := "connection-" + fake.UUID().V4()
		existingAt := time.Date(2026, time.July, 10, 8, 30, 0, 123, time.FixedZone("existing", -4*60*60))
		requestAt := time.Date(2026, time.July, 11, 18, 45, 0, 456, time.FixedZone("request", 5*60*60+30*60))

		t.Run("transaction omission and null mean no timestamp change", func(t *testing.T) {
			service := newMockfinanceService(t)
			call := service.EXPECT().UpdateTransaction(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, params financepkg.UpdateTransactionParams) (domain.Transaction, error) {
					return domain.Transaction{
						ID:          transactionID,
						TenantID:    tenantID,
						AccountID:   "account-" + fake.UUID().V4(),
						Source:      domain.TransactionSourceManual,
						Status:      domain.TransactionStatusBooked,
						Kind:        domain.TransactionKindRegular,
						Currency:    "USD",
						Description: params.Description,
						AmountMinor: params.AmountMinor,
						EffectiveAt: existingAt,
						CreatedAt:   existingAt,
						UpdatedAt:   existingAt,
					}, nil
				})
			call.Times(2)
			for _, body := range []string{
				`{"description":"omitted time","amountMinor":1,"tagIds":[]}`,
				`{"description":"null time","amountMinor":1,"effectiveAt":null,"tagIds":[]}`,
			} {
				resp := httptest.NewRecorder()
				newHandler(service, newMockbankConnectionService(t), makeAuthMiddleware(userID)).ServeHTTP(
					resp,
					newRequest(
						http.MethodPatch,
						"/api/v1/finance/tenants/"+tenantID+"/transactions/"+transactionID,
						body,
						true,
					),
				)
				require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
				assert.Equal(t, existingAt.Format(time.RFC3339Nano), decode(t, resp)["effectiveAt"])
			}
		})

		t.Run("transaction supplied fixed offset is preserved", func(t *testing.T) {
			service := newMockfinanceService(t)
			service.EXPECT().UpdateTransaction(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, params financepkg.UpdateTransactionParams) (domain.Transaction, error) {
					require.NotNil(t, params.EffectiveAt)
					require.Equal(t, requestAt.Format(time.RFC3339Nano), params.EffectiveAt.Format(time.RFC3339Nano))
					return domain.Transaction{
						ID:          transactionID,
						TenantID:    tenantID,
						AccountID:   "account-" + fake.UUID().V4(),
						Source:      domain.TransactionSourceManual,
						Status:      domain.TransactionStatusBooked,
						Kind:        domain.TransactionKindRegular,
						Currency:    "USD",
						Description: params.Description,
						AmountMinor: params.AmountMinor,
						EffectiveAt: *params.EffectiveAt,
						CreatedAt:   existingAt,
						UpdatedAt:   existingAt,
					}, nil
				})
			resp := httptest.NewRecorder()
			body := `{"description":"updated time","amountMinor":1,"effectiveAt":"` +
				requestAt.Format(time.RFC3339Nano) + `","tagIds":[]}`
			newHandler(service, newMockbankConnectionService(t), makeAuthMiddleware(userID)).ServeHTTP(
				resp,
				newRequest(
					http.MethodPatch,
					"/api/v1/finance/tenants/"+tenantID+"/transactions/"+transactionID,
					body,
					true,
				),
			)
			require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		})

		for _, testCase := range []struct {
			name  string
			value string
		}{
			{name: "empty", value: `""`},
			{name: "malformed", value: `"not-a-timestamp"`},
			{name: "year one", value: `"0001-01-01T00:00:00Z"`},
		} {
			t.Run("transaction rejects "+testCase.name, func(t *testing.T) {
				resp := httptest.NewRecorder()
				newHandler(
					newMockfinanceService(t),
					newMockbankConnectionService(t),
					makeAuthMiddleware(userID),
				).ServeHTTP(resp, newRequest(
					http.MethodPatch,
					"/api/v1/finance/tenants/"+tenantID+"/transactions/"+transactionID,
					`{"description":"invalid time","amountMinor":1,"effectiveAt":`+testCase.value+`,"tagIds":[]}`,
					true,
				))
				assert.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
			})
		}

		for _, testCase := range []struct {
			name       string
			body       string
			wantStart  *time.Time
			wantEnd    *time.Time
			wantStatus int
		}{
			{name: "omitted", body: `{}`, wantStatus: http.StatusOK},
			{
				name: "fixed offsets",
				body: `{"windowStart":"` + existingAt.Format(time.RFC3339Nano) +
					`","windowEnd":"` + requestAt.Format(time.RFC3339Nano) + `"}`,
				wantStart: &existingAt, wantEnd: &requestAt, wantStatus: http.StatusOK,
			},
			{name: "null", body: `{"windowStart":null}`, wantStatus: http.StatusOK},
			{name: "empty", body: `{"windowStart":""}`, wantStatus: http.StatusBadRequest},
			{name: "malformed", body: `{"windowStart":"not-a-timestamp"}`, wantStatus: http.StatusBadRequest},
			{name: "year one", body: `{"windowStart":"0001-01-01T00:00:00Z"}`, wantStatus: http.StatusBadRequest},
		} {
			t.Run("connection windows "+testCase.name, func(t *testing.T) {
				service := newMockfinanceService(t)
				call := service.EXPECT().TriggerBankConnectionSync(mock.Anything, mock.Anything).
					RunAndReturn(func(_ context.Context, params financepkg.TriggerBankConnectionSyncParams) (financepkg.BankConnectionSyncJobRef, error) {
						if testCase.wantStart == nil {
							require.Nil(t, params.WindowStart)
						} else {
							require.NotNil(t, params.WindowStart)
							require.Equal(
								t,
								testCase.wantStart.Format(time.RFC3339Nano),
								params.WindowStart.Format(time.RFC3339Nano),
							)
						}
						if testCase.wantEnd == nil {
							require.Nil(t, params.WindowEnd)
						} else {
							require.NotNil(t, params.WindowEnd)
							require.Equal(
								t,
								testCase.wantEnd.Format(time.RFC3339Nano),
								params.WindowEnd.Format(time.RFC3339Nano),
							)
						}
						return financepkg.BankConnectionSyncJobRef{
							ID:      "job-" + fake.UUID().V4(),
							JobType: financepkg.BankConnectionSyncJobType,
						}, nil
					})
				if testCase.wantStatus != http.StatusOK {
					call.Maybe()
				}
				resp := httptest.NewRecorder()
				newHandler(service, newMockbankConnectionService(t), makeAuthMiddleware(userID)).ServeHTTP(
					resp,
					newRequest(
						http.MethodPost,
						"/api/v1/finance/tenants/"+tenantID+"/connections/"+connectionID+"/sync",
						testCase.body,
						true,
					),
				)
				assert.Equal(t, testCase.wantStatus, resp.Code, resp.Body.String())
			})
		}
	})

	t.Run("registered finance routes reject invalid timestamp ranges before services", func(t *testing.T) {
		userID := "user-" + fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		handler := newHandler(
			newMockfinanceService(t),
			newMockbankConnectionService(t),
			makeAuthMiddleware(userID),
		)

		for _, testCase := range []struct {
			name   string
			method string
			target string
			body   string
		}{
			{
				name: "custom dashboard missing end", method: http.MethodGet,
				target: "/api/v1/finance/tenants/" + tenantID + "/dashboard?preset=custom&startDate=2026-06-01T00:00:00Z",
			},
			{
				name: "custom dashboard reversed", method: http.MethodGet,
				target: "/api/v1/finance/tenants/" + tenantID + "/dashboard?preset=custom&startDate=2026-06-02T00:00:00Z&endDate=2026-06-01T00:00:00Z",
			},
			{
				name: "dashboard unsupported preset", method: http.MethodGet,
				target: "/api/v1/finance/tenants/" + tenantID + "/dashboard?preset=rolling_month",
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, newRequest(testCase.method, testCase.target, testCase.body, true))
				require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
				require.NotContains(t, response.Body.String(), "response validation")
			})
		}
	})

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

		t.Run("provider client failures return an empty client error", func(t *testing.T) {
			service := newMockfinanceService(t)
			bankConnections := newMockbankConnectionService(t)
			bankConnections.EXPECT().
				StartBankConnectionLink(mock.Anything, mock.Anything).
				Return(financepkg.ProviderLinkStart{}, fmt.Errorf(
					"start bank connection link: %w",
					&financepkg.ProviderResponseError{
						Provider:   "enable-banking",
						Operation:  "auth",
						StatusCode: http.StatusBadRequest,
						Code:       "REDIRECT_URI_NOT_ALLOWED",
						Message:    "provider request failed",
					},
				))
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
			assert.Empty(t, resp.Body.String())
		})

		t.Run("redirect finish provider client failures return an empty client error", func(t *testing.T) {
			service := newMockfinanceService(t)
			bankConnections := newMockbankConnectionService(t)
			bankConnections.EXPECT().
				FinishBankConnectionLink(mock.Anything, mock.Anything).
				Return(domain.BankConnection{}, fmt.Errorf(
					"finish bank connection link: %w",
					&financepkg.ProviderResponseError{
						Provider:   "enable-banking",
						Operation:  "sessions",
						StatusCode: http.StatusBadRequest,
						Code:       "INVALID_AUTHORIZATION_CODE",
						Message:    "provider request failed",
					},
				))
			resp := httptest.NewRecorder()
			newHandler(
				service,
				bankConnections,
				makeAuthMiddleware(userID),
			).ServeHTTP(
				resp,
				newRequest(
					http.MethodPost,
					"/api/v1/finance/tenants/tenant-a/connections/link-redirect/finish",
					`{"provider":"pko","state":"state-1","code":"code-1"}`,
					true,
				),
			)

			require.Equal(t, http.StatusBadRequest, resp.Code)
			assert.Empty(t, resp.Body.String())
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
			encodedConnection, err := json.Marshal(mappedConnection)
			require.NoError(t, err)
			var publicConnection map[string]any
			require.NoError(t, json.Unmarshal(encodedConnection, &publicConnection))
			assert.NotContains(t, publicConnection, "externalId")
			assert.Equal(t, int64(900), mappedConnection.Schedule.IntervalSeconds)
			assert.Equal(t, now, mappedConnection.CreatedAt)
			assert.Equal(t, now, *mappedConnection.LastSyncStartedAt)
			assert.Equal(t, now, mappedConnection.Schedule.CreatedAt)

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
				FXCoverage: []financepkg.DashboardFXCoverage{
					{
						BaseCurrency: "EUR", QuoteCurrency: "USD", Provider: "nbp",
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
			assert.Equal(t, now, mappedDashboard.Period.StartDate)
			assert.Equal(t, "nbp", mappedDashboard.FxCoverage[0].Provider)
			require.Len(t, mappedDashboard.CategoryBreakdowns, 1)
			require.Len(t, mappedDashboard.AccountBalances, 1)
			require.Len(t, mappedDashboard.Alerts, 1)
			require.Len(t, mappedDashboard.FxCoverage, 1)
			require.Len(t, mappedDashboard.NativeSettledTotals, 1)

			mappedAccount, err := mapAccount(domain.Account{
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
			require.NoError(t, err)
			assert.Equal(t, "provider-1", mappedAccount.Provider)
			assert.Equal(t, "provider-account-1", mappedAccount.ProviderAccountID)

			mappedPreview := mapCSVPreview(financepkg.CSVImportPreview{
				ImportID:        "import-1",
				ImportType:      financepkg.CSVImportTypeTransactions,
				ImportableCount: 2,
				Headers:         []string{"account", "amount"},
				Mapping:         map[string]string{"account": "account"},
				DuplicateRows: []financepkg.CSVImportRejectedRow{
					{RowNumber: 2, Field: "description", Reason: "duplicate"},
				},
				RejectedRows: []financepkg.CSVImportRejectedRow{
					{RowNumber: 3, Reason: "invalid amount"},
				},
			})
			require.Len(t, mappedPreview.DuplicateRows, 1)
			require.Len(t, mappedPreview.RejectedRows, 1)
			assert.Equal(t, int64(2), mappedPreview.DuplicateRows[0].RowNumber)
			assert.Equal(t, "description", mappedPreview.DuplicateRows[0].Field)
			assert.Equal(t, int64(2), mappedPreview.ImportableCount)

			boom := errors.New("boom")
			assert.Same(t, boom, mapCSVImportError(boom))
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
			mapBankConnectionError(invalidInputErr, "fallback"),
		)
		require.NoError(t, mapBankConnectionError(nil, "fallback"))
		assert.Contains(
			t,
			mapBankConnectionError(
				financepkg.ErrUnsupportedBankProvider,
				"fallback",
			).Error(),
			"unsupported bank provider",
		)
		assert.Contains(
			t,
			mapBankConnectionError(
				financepkg.ErrUnsupportedBankLinkingMethod,
				"fallback",
			).Error(),
			"unsupported bank linking method",
		)
		assert.Contains(
			t,
			mapBankConnectionError(
				financepkg.ErrPendingBankConnectionLinkStartNotFound,
				"fallback",
			).Error(),
			"pending bank link start not found or expired",
		)
		assert.Contains(
			t,
			mapBankConnectionError(
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
			mapBankConnectionError(
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

	t.Run("transaction tag IDs map through registered create read list update and clear routes", func(t *testing.T) {
		userID := "user-" + fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		transactionID := "transaction-" + fake.UUID().V4()
		firstTagID := "tag-first-" + fake.UUID().V4()
		secondTagID := "tag-second-" + fake.UUID().V4()
		now := time.Date(2026, time.July, 12, 15, 0, 0, 0, time.FixedZone("test", 2*60*60))
		transaction := func(tagIDs []string) domain.Transaction {
			return domain.Transaction{
				ID:          transactionID,
				TenantID:    tenantID,
				AccountID:   "account-" + fake.UUID().V4(),
				Source:      domain.TransactionSourceManual,
				Status:      domain.TransactionStatusBooked,
				Kind:        domain.TransactionKindRegular,
				AmountMinor: -123,
				Currency:    "USD",
				Description: "transaction-" + fake.Lorem().Word(),
				EffectiveAt: now,
				TagIDs:      tagIDs,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
		}
		stringValues := func(values []any) []string {
			result := make([]string, 0, len(values))
			for _, value := range values {
				result = append(result, value.(string))
			}
			return result
		}
		service := newMockfinanceService(t)
		service.EXPECT().RecordTransaction(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, params financepkg.RecordTransactionParams) (domain.Transaction, error) {
				require.Equal(t, []string{secondTagID, firstTagID}, params.TagIDs)
				return transaction(params.TagIDs), nil
			},
		)
		service.EXPECT().GetTransaction(mock.Anything, mock.Anything).Return(
			transaction([]string{firstTagID, secondTagID}), nil,
		)
		service.EXPECT().ListTransactions(mock.Anything, mock.Anything).Return(
			[]domain.Transaction{transaction([]string{firstTagID, secondTagID})}, nil,
		)
		service.EXPECT().UpdateTransaction(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, params financepkg.UpdateTransactionParams) (domain.Transaction, error) {
				require.Empty(t, params.TagIDs)
				return transaction(params.TagIDs), nil
			},
		)
		handler := newHandler(service, newMockbankConnectionService(t), makeAuthMiddleware(userID))
		for _, testCase := range []struct {
			method string
			target string
			body   string
			want   []string
		}{
			{
				method: http.MethodPost,
				target: "/api/v1/finance/tenants/" + tenantID + "/transactions",
				body:   `{"accountId":"account","source":"manual","status":"booked","kind":"regular","amountMinor":-123,"currency":"USD","description":"created","effectiveAt":"2026-07-12T15:00:00+02:00","tagIds":["` + secondTagID + `","` + firstTagID + `"]}`,
				want:   []string{secondTagID, firstTagID},
			},
			{method: http.MethodGet, target: "/api/v1/finance/tenants/" + tenantID + "/transactions/" + transactionID, want: []string{firstTagID, secondTagID}},
			{method: http.MethodGet, target: "/api/v1/finance/tenants/" + tenantID + "/transactions?limit=20", want: []string{firstTagID, secondTagID}},
			{method: http.MethodPatch, target: "/api/v1/finance/tenants/" + tenantID + "/transactions/" + transactionID, body: `{"description":"cleared","amountMinor":-123,"tagIds":[]}`, want: []string{}},
		} {
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, newRequest(testCase.method, testCase.target, testCase.body, true))
			require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
			payload := decode(t, resp)
			if testCase.method == http.MethodGet && strings.HasSuffix(testCase.target, "?limit=20") {
				payload = payload["items"].([]any)[0].(map[string]any)
			}
			assert.Equal(t, testCase.want, stringValues(payload["tagIds"].([]any)))
		}

		missingTagIDsResp := httptest.NewRecorder()
		newHandler(newMockfinanceService(t), newMockbankConnectionService(t), makeAuthMiddleware(userID)).ServeHTTP(
			missingTagIDsResp,
			newRequest(
				http.MethodPatch,
				"/api/v1/finance/tenants/"+tenantID+"/transactions/"+transactionID,
				`{"description":"missing tag IDs","amountMinor":-123}`,
				true,
			),
		)
		require.Equal(t, http.StatusBadRequest, missingTagIDsResp.Code)
	})

	t.Run("account lifecycle and catalog update routes use narrow mutations", func(t *testing.T) {
		userID := "user-" + fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		accountID := "account-" + fake.UUID().V4()
		categoryID := "category-" + fake.UUID().V4()
		tagID := "tag-" + fake.UUID().V4()
		accountName := "account-" + fake.Lorem().Word()
		categoryName := "category-" + fake.Lorem().Word()
		categoryKind := domain.CategoryKindIncome
		tagName := "tag-" + fake.Lorem().Word()

		mutationService := newMockfinanceService(t)
		mutationService.EXPECT().UpdateAccount(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, params financepkg.UpdateAccountParams) (domain.Account, error) {
				assert.Equal(t, financepkg.UpdateAccountParams{
					ActorUserID: userID, TenantID: tenantID, AccountID: accountID, Name: accountName,
				}, params)
				return domain.Account{}, nil
			},
		).Once()
		mutationService.EXPECT().HideAccount(mock.Anything, financepkg.HideAccountParams{
			ActorUserID: userID, TenantID: tenantID, AccountID: accountID,
		}).Return(nil).Once()
		mutationService.EXPECT().UnhideAccount(mock.Anything, financepkg.UnhideAccountParams{
			ActorUserID: userID, TenantID: tenantID, AccountID: accountID,
		}).Return(nil).Once()
		mutationService.EXPECT().UpdateCategory(mock.Anything, financepkg.UpdateCategoryParams{
			ActorUserID: userID, TenantID: tenantID, CategoryID: categoryID, Name: categoryName, Kind: categoryKind,
		}).Return(domain.Category{}, nil).Once()
		mutationService.EXPECT().UpdateTag(mock.Anything, financepkg.UpdateTagParams{
			ActorUserID: userID, TenantID: tenantID, TagID: tagID, Name: tagName,
		}).Return(domain.Tag{}, nil).Once()

		handler := newHandler(mutationService, newMockbankConnectionService(t), makeAuthMiddleware(userID))
		for _, tc := range []struct {
			method string
			target string
			body   string
		}{
			{http.MethodPatch, "/api/v1/finance/tenants/" + tenantID + "/accounts/" + accountID, `{"name":"` + accountName + `"}`},
			{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/accounts/" + accountID + "/hide", ""},
			{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/accounts/" + accountID + "/unhide", ""},
			{http.MethodPatch, "/api/v1/finance/tenants/" + tenantID + "/categories/" + categoryID, `{"name":"` + categoryName + `","kind":"income"}`},
			{http.MethodPatch, "/api/v1/finance/tenants/" + tenantID + "/tags/" + tagID, `{"name":"` + tagName + `"}`},
		} {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, newRequest(tc.method, tc.target, tc.body, true))
			require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
			assert.Empty(t, response.Body.String())
		}

		t.Run("missing rename name is rejected by the registered validator", func(t *testing.T) {
			response := httptest.NewRecorder()
			newHandler(newMockfinanceService(t), newMockbankConnectionService(t), makeAuthMiddleware(userID)).ServeHTTP(
				response,
				newRequest(http.MethodPatch, "/api/v1/finance/tenants/"+tenantID+"/accounts/"+accountID, `{}`, true),
			)
			require.Equal(t, http.StatusBadRequest, response.Code)
		})

		t.Run("invalid category kind is rejected by the registered validator", func(t *testing.T) {
			response := httptest.NewRecorder()
			newHandler(newMockfinanceService(t), newMockbankConnectionService(t), makeAuthMiddleware(userID)).ServeHTTP(
				response,
				newRequest(
					http.MethodPatch,
					"/api/v1/finance/tenants/"+tenantID+"/categories/"+categoryID,
					`{"name":"name","kind":"transfer"}`,
					true,
				),
			)
			require.Equal(t, http.StatusBadRequest, response.Code)
		})

		t.Run("foreign catalog resources return safe not found responses", func(t *testing.T) {
			for _, tc := range []struct {
				name    string
				method  string
				target  string
				body    string
				prepare func(*mockfinanceService)
			}{
				{
					name: "account", method: http.MethodPatch,
					target: "/api/v1/finance/tenants/" + tenantID + "/accounts/foreign-" + fake.UUID().V4(), body: `{"name":"name"}`,
					prepare: func(service *mockfinanceService) {
						service.EXPECT().UpdateAccount(mock.Anything, mock.Anything).Return(domain.Account{}, financepkg.ErrAccountNotFound).Once()
					},
				},
				{
					name: "category", method: http.MethodPatch,
					target: "/api/v1/finance/tenants/" + tenantID + "/categories/foreign-" + fake.UUID().V4(), body: `{"name":"name","kind":"expense"}`,
					prepare: func(service *mockfinanceService) {
						service.EXPECT().UpdateCategory(mock.Anything, mock.Anything).Return(domain.Category{}, financepkg.ErrCategoryNotFound).Once()
					},
				},
				{
					name: "tag", method: http.MethodPatch,
					target: "/api/v1/finance/tenants/" + tenantID + "/tags/foreign-" + fake.UUID().V4(), body: `{"name":"name"}`,
					prepare: func(service *mockfinanceService) {
						service.EXPECT().UpdateTag(mock.Anything, mock.Anything).Return(domain.Tag{}, financepkg.ErrTagNotFound).Once()
					},
				},
			} {
				t.Run(tc.name, func(t *testing.T) {
					service := newMockfinanceService(t)
					tc.prepare(service)
					response := httptest.NewRecorder()
					newHandler(service, newMockbankConnectionService(t), makeAuthMiddleware(userID)).ServeHTTP(
						response,
						newRequest(tc.method, tc.target, tc.body, true),
					)
					require.Equal(t, http.StatusNotFound, response.Code)
				})
			}
		})

		t.Run("hidden accounts cannot receive direct transactions", func(t *testing.T) {
			service := newMockfinanceService(t)
			service.EXPECT().RecordTransaction(mock.Anything, mock.Anything).
				Return(domain.Transaction{}, financepkg.ErrHiddenAccount).Once()
			response := httptest.NewRecorder()
			newHandler(service, newMockbankConnectionService(t), makeAuthMiddleware(userID)).ServeHTTP(
				response,
				newRequest(
					http.MethodPost,
					"/api/v1/finance/tenants/"+tenantID+"/transactions",
					`{"accountId":"`+accountID+`","source":"manual","status":"booked","kind":"regular","amountMinor":-1,"currency":"USD","description":"hidden account","effectiveAt":"2026-07-17T12:00:00+02:00"}`,
					true,
				),
			)
			require.Equal(t, http.StatusConflict, response.Code)
		})
	})

	t.Run("account-only imports use a separately scoped route contract", func(t *testing.T) {
		userID := "user-" + fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		importID := "import-" + fake.UUID().V4()
		now := time.Date(2026, time.July, 13, 10, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60))
		service := newMockfinanceService(t)
		service.EXPECT().PreviewCSVImport(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, params financepkg.PreviewCSVImportParams) (financepkg.CSVImportPreview, error) {
				require.Equal(t, financepkg.CSVImportTypeAccounts, params.ImportType)
				return financepkg.CSVImportPreview{
					ImportID:            importID,
					Headers:             []string{"name", "currency", "kind"},
					WouldCreateAccounts: []string{"Wallet"},
				}, nil
			},
		)
		service.EXPECT().ConfirmCSVImport(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, params financepkg.ConfirmCSVImportParams) (financepkg.CSVImportConfirmation, error) {
				require.Equal(t, financepkg.CSVImportTypeAccounts, params.ExpectedImportType)
				return financepkg.CSVImportConfirmation{
					ImportID: importID,
					JobID:    "job-account",
					JobType:  financepkg.CSVImportJobTypeAccounts,
				}, nil
			},
		)
		service.EXPECT().GetCSVImportAudit(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, params financepkg.GetCSVImportAuditParams) (financepkg.CSVImportAudit, error) {
				require.Equal(t, financepkg.CSVImportTypeAccounts, params.ExpectedImportType)
				return financepkg.CSVImportAudit{
					ImportID:      importID,
					TenantID:      tenantID,
					Status:        financepkg.CSVImportStatusCompleted,
					JobID:         "job-account",
					ImportedCount: 1,
					CreatedAt:     now,
				}, nil
			},
		)
		handler := newHandler(service, newMockbankConnectionService(t), makeAuthMiddleware(userID))

		previewResp := httptest.NewRecorder()
		handler.ServeHTTP(previewResp, newRequest(
			http.MethodPost,
			"/api/v1/finance/tenants/"+tenantID+"/account-imports/preview",
			`{"fileName":"accounts.csv","csv":"name,currency,kind\nWallet,USD,manual"}`,
			true,
		))
		require.Equal(t, http.StatusOK, previewResp.Code, previewResp.Body.String())
		previewPayload := decode(t, previewResp)
		assert.NotContains(t, previewPayload, "mapping")

		confirmResp := httptest.NewRecorder()
		handler.ServeHTTP(confirmResp, newRequest(
			http.MethodPost,
			"/api/v1/finance/tenants/"+tenantID+"/account-imports/"+importID+"/confirm",
			`{}`,
			true,
		))
		require.Equal(t, http.StatusOK, confirmResp.Code, confirmResp.Body.String())

		auditResp := httptest.NewRecorder()
		handler.ServeHTTP(auditResp, newRequest(
			http.MethodGet,
			"/api/v1/finance/tenants/"+tenantID+"/account-imports/"+importID,
			"",
			true,
		))
		require.Equal(t, http.StatusOK, auditResp.Code, auditResp.Body.String())
		assert.NotContains(t, decode(t, auditResp), "importType")
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
