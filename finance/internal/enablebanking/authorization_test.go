package enablebanking

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	client "github.com/gemyago/sumweave/finance/internal/enablebanking/client"
	"github.com/gemyago/sumweave/finance/internal/providers"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectorAuthorization(t *testing.T) {
	fake := faker.New()
	makeBinding := func() domain.BankAccountBinding {
		return domain.BankAccountBinding{
			CanonicalID: fake.UUID().V4(),
			RemoteID:    fake.UUID().V4(),
			PrimaryHash: fake.UUID().V4(),
			Currency:    fake.Currency().Code(),
			Selected:    true,
		}
	}
	makeRequest := func() providers.FetchRequest {
		return providers.FetchRequest{
			Connection: domain.ProviderConnectionRef{
				ConnectionID:      fake.UUID().V4(),
				ProviderReference: fake.UUID().V4(),
			},
			RequestedWindow: domain.ProviderSyncWindow{Start: time.Now().AddDate(0, 0, -30), End: time.Now()},
		}
	}

	t.Run("closed and expired session states fail before account fetch", func(t *testing.T) {
		for _, status := range []string{"CLOSED", "EXPIRED"} {
			t.Run(status, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Contains(t, r.URL.Path, "/sessions/")
					assert.NoError(t, json.NewEncoder(w).Encode(client.SessionResponse{Status: status}))
				}))
				defer server.Close()
				c := NewConnector(Args{BaseURL: server.URL, HTTPClient: server.Client()})
				_, err := c.fetchOfficial(t.Context(), makeRequest())
				require.ErrorIs(t, err, domain.ErrBankReauthRequired)
			})
		}
	})

	t.Run("structured session codes alone classify closure", func(t *testing.T) {
		// Fetch wraps errors from any account endpoint. An unsigned client lets the
		// HTTP fixture exercise this without introducing a dependency fake.
		for _, code := range []string{"CLOSED_SESSION", "EXPIRED_SESSION", "UNAUTHORIZED"} {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				assert.NoError(
					t,
					json.NewEncoder(w).Encode(map[string]string{"error": code, "message": fake.Lorem().Sentence(3)}),
				)
			}))
			c := NewConnector(Args{BaseURL: server.URL, HTTPClient: server.Client()})
			c.appID, c.privateKeyPath = fake.UUID().V4(), fake.UUID().V4()
			_, err := c.Fetch(t.Context(), makeRequest())
			require.Error(t, err)
			if code == "UNAUTHORIZED" {
				require.NotErrorIs(t, err, domain.ErrBankReauthRequired)
			} else {
				require.ErrorIs(t, err, domain.ErrBankReauthRequired)
			}
			server.Close()
		}
	})

	t.Run("remote fetch uses canonical transaction fingerprints and truthful source objects", func(t *testing.T) {
		binding := makeBinding()
		request := makeRequest()
		name := fake.Company().Name()
		transaction := client.AccountTransaction{
			Status:            "BOOKED",
			TransactionAmount: client.TransactionAmount{Amount: "12.34", Currency: binding.Currency},
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/accounts/" + binding.RemoteID + "/details":
				assert.NoError(
					t,
					json.NewEncoder(w).
						Encode(client.Account{UID: &binding.RemoteID, Name: &name, Currency: binding.Currency, IdentificationHash: binding.PrimaryHash}),
				)
			case "/accounts/" + binding.RemoteID + "/balances":
				assert.NoError(t, json.NewEncoder(w).Encode(client.GetAccountBalancesResponse{}))
			case "/accounts/" + binding.RemoteID + "/transactions":
				assert.NoError(
					t,
					json.NewEncoder(w).
						Encode(client.GetAccountTransactionsResponse{Transactions: []client.AccountTransaction{transaction}}),
				)
			default:
				t.Errorf("unexpected account fetch %s", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()
		authorization := domain.BankAuthorization{Accounts: []domain.BankAccountBinding{binding}}
		c := NewConnector(
			Args{BaseURL: server.URL, HTTPClient: server.Client()},
			WithAuthorizationResolver(
				func(context.Context, domain.ConnectionSecret) (*domain.BankAuthorization, error) {
					return &authorization, nil
				},
			),
		)
		session := &client.SessionResponse{
			AccountsData: []client.SessionAccount{{UID: binding.RemoteID, IdentificationHash: binding.PrimaryHash}},
		}
		batch, err := c.mapBatch(t.Context(), request, session)
		require.NoError(t, err)
		require.Equal(t, binding.CanonicalID, batch.Accounts[0].ProviderAccountID)
		require.Equal(t, binding.CanonicalID, batch.Balances[0].ProviderAccountID)
		require.Equal(
			t,
			normalizeTransaction(request.Connection, binding.CanonicalID, transaction),
			batch.Transactions[0],
		)
		for _, snapshot := range batch.Snapshots[1:] {
			require.Equal(t, binding.CanonicalID, snapshot.ProviderAccountID)
		}
		require.Equal(t, binding.RemoteID, batch.Snapshots[1].ProviderObjectID)
		var raw client.Account
		require.NoError(t, json.Unmarshal(batch.Snapshots[1].DocumentJSON, &raw))
		require.Equal(t, binding.RemoteID, *raw.UID)
		session.AccountsData[0].IdentificationHash = fake.UUID().V4()
		_, err = c.mapBatch(t.Context(), request, session)
		require.ErrorIs(t, err, domain.ErrBankAuthorizationInvalid)
		session.AccountsData = nil
		_, err = c.mapBatch(t.Context(), request, session)
		require.ErrorIs(t, err, domain.ErrBankAuthorizationInvalid)
	})
}
