package v1controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/middleware"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/server"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestFinanceTokenReadAllowlist(t *testing.T) {
	fake := faker.New()
	tenantID := "tenant-" + fake.UUID().V4()
	accountID := "account-" + fake.UUID().V4()
	transactionID := "transaction-" + fake.UUID().V4()
	connectionID := "connection-" + fake.UUID().V4()
	snapshotID := "snapshot-" + fake.UUID().V4()
	importID := "import-" + fake.UUID().V4()
	ruleID := "rule-" + fake.UUID().V4()
	categoryID := "category-" + fake.UUID().V4()
	tagID := "tag-" + fake.UUID().V4()
	state := "state-" + fake.UUID().V4()

	type operation struct {
		method    string
		target    string
		tokenRead bool
	}
	operations := []operation{
		{http.MethodPost, "/api/v1/finance/invites/accept", false},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/archive", false},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/account-imports/" + importID + "/confirm", false},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/imports/" + importID + "/confirm", false},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/accounts", false},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/categories", false},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/classification-rules", false},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/tags", false},
		{http.MethodPost, "/api/v1/finance/tenants", false},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/invites", false},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/transactions", false},
		{http.MethodDelete, "/api/v1/finance/tenants/" + tenantID + "/categories/" + categoryID, false},
		{http.MethodDelete, "/api/v1/finance/tenants/" + tenantID + "/classification-rules/" + ruleID, false},
		{http.MethodDelete, "/api/v1/finance/tenants/" + tenantID + "/connections/" + connectionID, false},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/connections/link-redirect/finish", false},
		{http.MethodGet, "/api/v1/finance/tenants/" + tenantID + "/accounts/" + accountID, true},
		{http.MethodGet, "/api/v1/finance/tenants/" + tenantID + "/account-imports/" + importID, false},
		{
			http.MethodGet,
			"/api/v1/finance/tenants/" + tenantID + "/accounts/" + accountID + "/provider-snapshots/" + snapshotID,
			true,
		},
		{http.MethodGet, "/api/v1/finance/tenants/" + tenantID + "/cash-flow-series", false},
		{http.MethodGet, "/api/v1/finance/tenants/" + tenantID + "/imports/" + importID, false},
		{http.MethodGet, "/api/v1/finance/tenants/" + tenantID + "/dashboard", false},
		{http.MethodGet, "/api/v1/finance/fx/diagnostics", false},
		{
			http.MethodGet,
			"/api/v1/finance/tenants/" + tenantID + "/connections/synthetic-link-states/state/" + state,
			false,
		},
		{http.MethodGet, "/api/v1/finance/tenants/" + tenantID, false},
		{http.MethodGet, "/api/v1/finance/tenants/" + tenantID + "/transactions/" + transactionID, true},
		{
			http.MethodGet,
			"/api/v1/finance/tenants/" + tenantID + "/transactions/" + transactionID + "/provider-snapshots/" + snapshotID,
			true,
		},
		{
			http.MethodGet,
			"/api/v1/finance/tenants/" + tenantID + "/transactions/" + transactionID + "/transfer-partner",
			false,
		},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/accounts/" + accountID + "/hide", false},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/connections/link-token", false},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/transactions/transfer-links", false},
		{
			http.MethodGet,
			"/api/v1/finance/tenants/" + tenantID + "/accounts/" + accountID + "/provider-snapshots",
			true,
		},
		{http.MethodGet, "/api/v1/finance/tenants/" + tenantID + "/accounts", true},
		{http.MethodGet, "/api/v1/finance/tenants/" + tenantID + "/categories", false},
		{http.MethodGet, "/api/v1/finance/tenants/" + tenantID + "/classification-rules", false},
		{http.MethodGet, "/api/v1/finance/tenants/" + tenantID + "/connections/" + connectionID + "/accounts", false},
		{http.MethodGet, "/api/v1/finance/tenants/" + tenantID + "/connections", true},
		{http.MethodGet, "/api/v1/finance/tenants/" + tenantID + "/tags", false},
		{http.MethodGet, "/api/v1/finance/tenants/" + tenantID + "/invites", false},
		{http.MethodGet, "/api/v1/finance/tenants/" + tenantID + "/members", false},
		{http.MethodGet, "/api/v1/finance/tenants", true},
		{
			http.MethodGet,
			"/api/v1/finance/tenants/" + tenantID + "/transactions/" + transactionID + "/provider-snapshots",
			true,
		},
		{http.MethodGet, "/api/v1/finance/tenants/" + tenantID + "/transactions", true},
		{
			http.MethodGet,
			"/api/v1/finance/tenants/" + tenantID + "/transactions/" + transactionID + "/transfer-candidates",
			false,
		},
		{http.MethodGet, "/api/v1/finance/tenants/" + tenantID + "/imports", false},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/classification-rules/" + ruleID + "/move", false},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/account-imports/preview", false},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/imports/preview", false},
		{
			http.MethodPut,
			"/api/v1/finance/tenants/" + tenantID + "/connections/synthetic-link-states/state/" + state,
			false,
		},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/connections/link-redirect/start", false},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/transactions/classify", false},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/transactions/match-transfers", false},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/connections/" + connectionID + "/sync", false},
		{http.MethodPost, "/api/v1/finance/fx/sync", false},
		{http.MethodPost, "/api/v1/finance/tenants/" + tenantID + "/accounts/" + accountID + "/unhide", false},
		{http.MethodDelete, "/api/v1/finance/tenants/" + tenantID + "/transactions/transfer-links", false},
		{http.MethodPatch, "/api/v1/finance/tenants/" + tenantID + "/accounts/" + accountID, false},
		{http.MethodPatch, "/api/v1/finance/tenants/" + tenantID + "/categories/" + categoryID, false},
		{http.MethodPut, "/api/v1/finance/tenants/" + tenantID + "/classification-rules/" + ruleID, false},
		{http.MethodPatch, "/api/v1/finance/tenants/" + tenantID + "/connections/" + connectionID, false},
		{http.MethodPatch, "/api/v1/finance/tenants/" + tenantID + "/tags/" + tagID, false},
		{http.MethodPatch, "/api/v1/finance/tenants/" + tenantID, false},
		{http.MethodPatch, "/api/v1/finance/tenants/" + tenantID + "/transactions/" + transactionID, false},
	}

	makeMiddleware := func(allowed bool) middleware.AuthMiddleware {
		return func(http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if allowed {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				w.WriteHeader(http.StatusForbidden)
			})
		}
	}
	makeHandler := func(defaultAllowed, tokenReadAllowed bool) http.Handler {
		controller := NewFinanceController(FinanceControllerDeps{
			AuthMiddleware:      makeMiddleware(defaultAllowed),
			TokenReadMiddleware: makeMiddleware(tokenReadAllowed),
		})
		return server.NewTestRootHandler().RegisterFinanceRoutes(controller)
	}

	for _, caller := range []struct {
		name                string
		defaultAllowed      bool
		tokenReadAllowed    bool
		expectsTokenAllowed bool
	}{
		{name: "session", defaultAllowed: true, tokenReadAllowed: true, expectsTokenAllowed: true},
		{name: "read-only token", tokenReadAllowed: true},
		{name: "read-write token", tokenReadAllowed: true},
	} {
		t.Run(caller.name, func(t *testing.T) {
			handler := makeHandler(caller.defaultAllowed, caller.tokenReadAllowed)
			for _, operation := range operations {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequest(operation.method, operation.target, nil))
				wantStatus := http.StatusForbidden
				if caller.expectsTokenAllowed || operation.tokenRead {
					wantStatus = http.StatusNoContent
				}
				require.Equal(t, wantStatus, response.Code, "%s %s", operation.method, operation.target)
			}
		})
	}
}
