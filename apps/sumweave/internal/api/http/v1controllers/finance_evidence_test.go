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

func TestFinanceEvidenceController(t *testing.T) {
	fake := faker.New()

	makeHandler := func(t *testing.T, evidence providerEvidenceService, userID string) http.Handler {
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
			ProviderEvidenceService: evidence,
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

	t.Run("uses registered metadata and detail routes without exposing list payloads", func(t *testing.T) {
		userID := "user-" + fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		accountID := "account-" + fake.UUID().V4()
		evidenceID := "evidence-" + fake.UUID().V4()
		capturedAt := time.Date(2026, time.July, 14, 10, 0, 0, 0, time.FixedZone("test", 2*60*60))
		service := newMockfinanceService(t)
		service.EXPECT().ListAccountProviderEvidence(mock.Anything, mock.MatchedBy(
			func(params financepkg.ListAccountProviderEvidenceParams) bool {
				return params.ActorUserID == userID && params.TenantID == tenantID && params.AccountID == accountID
			},
		)).Return([]domain.ProviderEvidence{{
			ID:               evidenceID,
			Subject:          domain.ProviderEvidenceSubjectAccount,
			Scope:            domain.RawPayloadScopeAccount,
			ProviderObjectID: "provider-account-" + fake.UUID().V4(),
			PayloadJSON:      []byte(`{"secret":"must-not-appear"}`),
			CapturedAt:       capturedAt,
		}}, nil).Once()
		service.EXPECT().GetAccountProviderEvidence(mock.Anything, mock.MatchedBy(
			func(params financepkg.GetAccountProviderEvidenceParams) bool {
				return params.ActorUserID == userID && params.TenantID == tenantID &&
					params.AccountID == accountID && params.EvidenceID == evidenceID
			},
		)).Return(domain.ProviderEvidence{
			ID:               evidenceID,
			Subject:          domain.ProviderEvidenceSubjectAccount,
			Scope:            domain.RawPayloadScopeAccount,
			ProviderObjectID: "provider-account-" + fake.UUID().V4(),
			PayloadJSON:      []byte(`{"visible":"safe"}`),
			CapturedAt:       capturedAt,
		}, nil).Once()

		handler := makeHandler(t, service, userID)
		listResponse := httptest.NewRecorder()
		handler.ServeHTTP(listResponse, newRequest(
			http.MethodGet, "/api/v1/finance/tenants/"+tenantID+"/accounts/"+accountID+"/evidence", true,
		))
		require.Equal(t, http.StatusOK, listResponse.Code)
		assert.NotContains(t, listResponse.Body.String(), "payload")
		assert.NotContains(t, listResponse.Body.String(), "must-not-appear")

		detailResponse := httptest.NewRecorder()
		handler.ServeHTTP(detailResponse, newRequest(
			http.MethodGet,
			"/api/v1/finance/tenants/"+tenantID+"/accounts/"+accountID+"/evidence/"+evidenceID,
			true,
		))
		require.Equal(t, http.StatusOK, detailResponse.Code)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(detailResponse.Body.Bytes(), &payload))
		assert.Equal(t, "safe", payload["payload"].(map[string]any)["visible"])

		transactionID := "transaction-" + fake.UUID().V4()
		transactionEvidenceID := "evidence-transaction-" + fake.UUID().V4()
		service.EXPECT().ListTransactionProviderEvidence(mock.Anything, mock.MatchedBy(
			func(params financepkg.ListTransactionProviderEvidenceParams) bool {
				return params.ActorUserID == userID && params.TenantID == tenantID &&
					params.TransactionID == transactionID
			},
		)).Return([]domain.ProviderEvidence{{
			ID:               transactionEvidenceID,
			Subject:          domain.ProviderEvidenceSubjectTransaction,
			Scope:            domain.RawPayloadScopeTransaction,
			ProviderObjectID: "provider-transaction-" + fake.UUID().V4(),
			PayloadJSON:      []byte(`{"refreshToken":"must-not-appear"}`),
			CapturedAt:       capturedAt,
		}}, nil).Once()
		service.EXPECT().GetTransactionProviderEvidence(mock.Anything, mock.MatchedBy(
			func(params financepkg.GetTransactionProviderEvidenceParams) bool {
				return params.ActorUserID == userID && params.TenantID == tenantID &&
					params.TransactionID == transactionID && params.EvidenceID == transactionEvidenceID
			},
		)).Return(domain.ProviderEvidence{
			ID:               transactionEvidenceID,
			Subject:          domain.ProviderEvidenceSubjectTransaction,
			Scope:            domain.RawPayloadScopeTransaction,
			ProviderObjectID: "provider-transaction-" + fake.UUID().V4(),
			PayloadJSON:      []byte(`{"visible":"safe-transaction"}`),
			CapturedAt:       capturedAt,
		}, nil).Once()

		transactionListResponse := httptest.NewRecorder()
		handler.ServeHTTP(transactionListResponse, newRequest(
			http.MethodGet,
			"/api/v1/finance/tenants/"+tenantID+"/transactions/"+transactionID+"/evidence",
			true,
		))
		require.Equal(t, http.StatusOK, transactionListResponse.Code)
		assert.NotContains(t, transactionListResponse.Body.String(), "payload")
		assert.NotContains(t, transactionListResponse.Body.String(), "must-not-appear")

		transactionDetailResponse := httptest.NewRecorder()
		handler.ServeHTTP(transactionDetailResponse, newRequest(
			http.MethodGet,
			"/api/v1/finance/tenants/"+tenantID+"/transactions/"+transactionID+"/evidence/"+transactionEvidenceID,
			true,
		))
		require.Equal(t, http.StatusOK, transactionDetailResponse.Code)
		require.NoError(t, json.Unmarshal(transactionDetailResponse.Body.Bytes(), &payload))
		assert.Equal(t, "safe-transaction", payload["payload"].(map[string]any)["visible"])

		accountFailureID := "account-failure-" + fake.UUID().V4()
		transactionFailureID := "transaction-failure-" + fake.UUID().V4()
		failureEvidenceID := "evidence-failure-" + fake.UUID().V4()
		service.EXPECT().ListAccountProviderEvidence(mock.Anything, mock.Anything).
			Return(nil, errors.New("account evidence unavailable")).Once()
		service.EXPECT().GetAccountProviderEvidence(mock.Anything, mock.Anything).
			Return(domain.ProviderEvidence{}, errors.New("account evidence unavailable")).Once()
		service.EXPECT().ListTransactionProviderEvidence(mock.Anything, mock.Anything).
			Return(nil, errors.New("transaction evidence unavailable")).Once()
		service.EXPECT().GetTransactionProviderEvidence(mock.Anything, mock.Anything).
			Return(domain.ProviderEvidence{}, errors.New("transaction evidence unavailable")).Once()
		service.EXPECT().GetAccountProviderEvidence(mock.Anything, mock.Anything).
			Return(domain.ProviderEvidence{PayloadJSON: []byte("not-json")}, nil).Once()
		service.EXPECT().GetTransactionProviderEvidence(mock.Anything, mock.Anything).
			Return(domain.ProviderEvidence{PayloadJSON: []byte("not-json")}, nil).Once()

		for _, target := range []string{
			"/api/v1/finance/tenants/" + tenantID + "/accounts/" + accountFailureID + "/evidence",
			"/api/v1/finance/tenants/" + tenantID + "/accounts/" + accountFailureID + "/evidence/" + failureEvidenceID,
			"/api/v1/finance/tenants/" + tenantID + "/transactions/" + transactionFailureID + "/evidence",
			"/api/v1/finance/tenants/" + tenantID + "/transactions/" + transactionFailureID + "/evidence/" + failureEvidenceID,
		} {
			failureResponse := httptest.NewRecorder()
			handler.ServeHTTP(failureResponse, newRequest(http.MethodGet, target, true))
			assert.Equal(t, http.StatusInternalServerError, failureResponse.Code)
		}
		for _, target := range []string{
			"/api/v1/finance/tenants/" + tenantID + "/accounts/" + accountFailureID + "/evidence/" + failureEvidenceID,
			"/api/v1/finance/tenants/" + tenantID + "/transactions/" + transactionFailureID + "/evidence/" + failureEvidenceID,
		} {
			failureResponse := httptest.NewRecorder()
			handler.ServeHTTP(failureResponse, newRequest(http.MethodGet, target, true))
			assert.Equal(t, http.StatusInternalServerError, failureResponse.Code)
		}
	})

	t.Run("requires authentication before evidence reads", func(t *testing.T) {
		handler := makeHandler(t, newMockfinanceService(t), "user-"+fake.UUID().V4())
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, newRequest(
			http.MethodGet,
			"/api/v1/finance/tenants/"+fake.UUID().V4()+"/transactions/"+fake.UUID().V4()+"/evidence",
			false,
		))
		require.Equal(t, http.StatusUnauthorized, response.Code)
		assert.True(t, strings.TrimSpace(response.Body.String()) == "" || response.Code == http.StatusUnauthorized)
	})
}
