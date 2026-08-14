package v1controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/middleware"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/server"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/runtime/httpapi"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFinanceProviderSnapshotController(t *testing.T) {
	fake := faker.New()

	makeHandler := func(t *testing.T, snapshots providerSnapshotService, userID string) http.Handler {
		t.Helper()
		auth := middleware.AuthMiddleware(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.Header.Get("Authorization") == "" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				ctx := httpapi.ContextWithCallerIdentity(request.Context(), &testCallerIdentity{userID: userID})
				next.ServeHTTP(w, request.WithContext(ctx))
			})
		})
		controller := NewFinanceController(FinanceControllerDeps{
			ProviderSnapshotService: snapshots,
			AuthMiddleware:          auth,
		})
		return server.NewTestRootHandler().RegisterFinanceRoutes(controller)
	}

	newRequest := func(method string, target string, authenticated bool) *http.Request {
		req := httptest.NewRequest(method, target, nil).WithContext(t.Context())
		if authenticated {
			req.Header.Set("Authorization", "Bearer "+fake.UUID().V4())
		}
		return req
	}

	t.Run("uses provider snapshot metadata and detail routes without compatibility aliases", func(t *testing.T) {
		userID := "user-" + fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		accountID := "account-" + fake.UUID().V4()
		accountSnapshotID := "snapshot-account-" + fake.UUID().V4()
		transactionID := "transaction-" + fake.UUID().V4()
		transactionSnapshotID := "snapshot-transaction-" + fake.UUID().V4()
		capturedAt := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.FixedZone("test", 2*60*60))
		service := newMockproviderSnapshotService(t)
		service.EXPECT().ListAccountProviderSnapshots(mock.Anything, mock.MatchedBy(
			func(params financepkg.ListAccountProviderSnapshotsParams) bool {
				return params.ActorUserID == userID && params.TenantID == tenantID && params.AccountID == accountID
			},
		)).Return([]domain.ProviderSnapshot{{
			ID: accountSnapshotID, Kind: domain.ProviderSnapshotKindAccount,
			ProviderObjectID: "provider-account-" + fake.UUID().V4(),
			DocumentJSON:     []byte(`{"secret":"must-not-appear"}`), CapturedAt: capturedAt,
		}}, nil).Once()
		service.EXPECT().GetAccountProviderSnapshot(mock.Anything, mock.MatchedBy(
			func(params financepkg.GetAccountProviderSnapshotParams) bool {
				return params.ActorUserID == userID && params.TenantID == tenantID &&
					params.AccountID == accountID && params.SnapshotID == accountSnapshotID
			},
		)).Return(domain.ProviderSnapshot{
			ID: accountSnapshotID, Kind: domain.ProviderSnapshotKindAccount,
			ProviderObjectID: "provider-account-" + fake.UUID().V4(),
			DocumentJSON:     []byte(`{"visible":"safe"}`), CapturedAt: capturedAt,
		}, nil).Once()
		service.EXPECT().ListTransactionProviderSnapshots(mock.Anything, mock.MatchedBy(
			func(params financepkg.ListTransactionProviderSnapshotsParams) bool {
				return params.ActorUserID == userID && params.TenantID == tenantID &&
					params.TransactionID == transactionID
			},
		)).Return([]domain.ProviderSnapshot{{
			ID: transactionSnapshotID, Kind: domain.ProviderSnapshotKindTransaction,
			ProviderObjectID: "provider-transaction-" + fake.UUID().V4(),
			DocumentJSON:     []byte(`{"refreshToken":"must-not-appear"}`), CapturedAt: capturedAt,
		}}, nil).Once()
		service.EXPECT().GetTransactionProviderSnapshot(mock.Anything, mock.MatchedBy(
			func(params financepkg.GetTransactionProviderSnapshotParams) bool {
				return params.ActorUserID == userID && params.TenantID == tenantID &&
					params.TransactionID == transactionID && params.SnapshotID == transactionSnapshotID
			},
		)).Return(domain.ProviderSnapshot{
			ID: transactionSnapshotID, Kind: domain.ProviderSnapshotKindTransaction,
			ProviderObjectID: "provider-transaction-" + fake.UUID().V4(),
			DocumentJSON:     []byte(`{"visible":"safe-transaction"}`), CapturedAt: capturedAt,
		}, nil).Once()

		handler := makeHandler(t, service, userID)
		for _, target := range []string{
			"/api/v1/finance/tenants/" + tenantID + "/accounts/" + accountID + "/evidence",
			"/api/v1/finance/tenants/" + tenantID + "/transactions/" + transactionID + "/evidence",
		} {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, newRequest(http.MethodGet, target, true))
			require.Equal(t, http.StatusNotFound, response.Code)
		}

		listResponse := httptest.NewRecorder()
		handler.ServeHTTP(listResponse, newRequest(
			http.MethodGet,
			"/api/v1/finance/tenants/"+tenantID+"/accounts/"+accountID+"/provider-snapshots",
			true,
		))
		require.Equal(t, http.StatusOK, listResponse.Code)
		assert.NotContains(t, listResponse.Body.String(), "data")
		assert.NotContains(t, listResponse.Body.String(), "must-not-appear")
		assert.Contains(t, listResponse.Body.String(), `"kind":"account"`)

		detailResponse := httptest.NewRecorder()
		handler.ServeHTTP(detailResponse, newRequest(
			http.MethodGet,
			"/api/v1/finance/tenants/"+tenantID+"/accounts/"+accountID+"/provider-snapshots/"+accountSnapshotID,
			true,
		))
		require.Equal(t, http.StatusOK, detailResponse.Code)
		var responsePayload map[string]any
		require.NoError(t, json.Unmarshal(detailResponse.Body.Bytes(), &responsePayload))
		assert.Equal(t, "safe", responsePayload["data"].(map[string]any)["visible"])

		transactionListResponse := httptest.NewRecorder()
		handler.ServeHTTP(transactionListResponse, newRequest(
			http.MethodGet,
			"/api/v1/finance/tenants/"+tenantID+"/transactions/"+transactionID+"/provider-snapshots",
			true,
		))
		require.Equal(t, http.StatusOK, transactionListResponse.Code)
		assert.NotContains(t, transactionListResponse.Body.String(), "data")
		assert.Contains(t, transactionListResponse.Body.String(), `"kind":"transaction"`)

		transactionDetailResponse := httptest.NewRecorder()
		handler.ServeHTTP(transactionDetailResponse, newRequest(
			http.MethodGet,
			"/api/v1/finance/tenants/"+tenantID+"/transactions/"+transactionID+"/provider-snapshots/"+transactionSnapshotID,
			true,
		))
		require.Equal(t, http.StatusOK, transactionDetailResponse.Code)
		require.NoError(t, json.Unmarshal(transactionDetailResponse.Body.Bytes(), &responsePayload))
		assert.Equal(t, "safe-transaction", responsePayload["data"].(map[string]any)["visible"])
	})

	t.Run("maps access, missing source data, and service failures", func(t *testing.T) {
		userID := "user-" + fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		accountID := "account-" + fake.UUID().V4()
		transactionID := "transaction-" + fake.UUID().V4()
		snapshotID := "snapshot-" + fake.UUID().V4()
		service := newMockproviderSnapshotService(t)
		service.EXPECT().ListAccountProviderSnapshots(mock.Anything, mock.Anything).
			Return(nil, financepkg.ErrTenantAccessDenied).Once()
		service.EXPECT().GetAccountProviderSnapshot(mock.Anything, mock.Anything).
			Return(domain.ProviderSnapshot{}, financepkg.ErrProviderSnapshotNotFound).Once()
		service.EXPECT().ListTransactionProviderSnapshots(mock.Anything, mock.Anything).
			Return(nil, financepkg.ErrTransactionNotFound).Once()
		service.EXPECT().GetTransactionProviderSnapshot(mock.Anything, mock.Anything).
			Return(domain.ProviderSnapshot{}, errors.New("provider snapshot unavailable")).Once()
		handler := makeHandler(t, service, userID)

		for _, testCase := range []struct {
			target string
			status int
		}{
			{target: "/api/v1/finance/tenants/" + tenantID + "/accounts/" + accountID + "/provider-snapshots", status: http.StatusUnauthorized},
			{target: "/api/v1/finance/tenants/" + tenantID + "/accounts/" + accountID + "/provider-snapshots/" + snapshotID, status: http.StatusNotFound},
			{target: "/api/v1/finance/tenants/" + tenantID + "/transactions/" + transactionID + "/provider-snapshots", status: http.StatusNotFound},
			{target: "/api/v1/finance/tenants/" + tenantID + "/transactions/" + transactionID + "/provider-snapshots/" + snapshotID, status: http.StatusInternalServerError},
		} {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, newRequest(http.MethodGet, testCase.target, true))
			assert.Equal(t, testCase.status, response.Code)
		}
	})

	t.Run("requires authentication before provider snapshot reads", func(t *testing.T) {
		handler := makeHandler(t, newMockproviderSnapshotService(t), "user-"+fake.UUID().V4())
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, newRequest(
			http.MethodGet,
			"/api/v1/finance/tenants/"+fake.UUID().V4()+"/transactions/"+fake.UUID().V4()+"/provider-snapshots",
			false,
		))
		require.Equal(t, http.StatusUnauthorized, response.Code)
		assert.True(t, strings.TrimSpace(response.Body.String()) == "" || response.Code == http.StatusUnauthorized)
	})
}
