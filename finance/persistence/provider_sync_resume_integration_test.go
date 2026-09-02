//go:build postgres_test

package persistence

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/internal/monobank"
	"github.com/gemyago/sumweave/finance/internal/providers"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderSyncResumeIntegration(t *testing.T) {
	t.Run("retains successful writes and checkpoints when a middle fetch fails then retries", func(t *testing.T) {
		fake := faker.New()
		store := NewStore(openTestDatabase(t))
		now := time.Date(2026, time.August, 18, 14, 0, 0, 0, time.FixedZone("CEST", 2*60*60))
		initialWindow := domain.ProviderSyncWindow{Start: now.AddDate(0, 0, -70), End: now}
		chunkPolicy := providers.NewOldestFirstWindowChunkPolicy()
		chunks, err := chunkPolicy.Split(initialWindow)
		require.NoError(t, err)
		require.Len(t, chunks, 3)
		connection := domain.ProviderConnectionRef{
			ConnectionID:      "connection-" + fake.UUID().V4(),
			ProviderID:        domain.ProviderIDMonobank,
			ConnectorID:       domain.ProviderConnectorIDMonobank,
			ProviderReference: "reference-" + fake.UUID().V4(),
		}
		tenantID := "tenant-" + fake.UUID().V4()
		secret := domain.ConnectionSecret{
			ID: "secret-" + fake.UUID().V4(), Provider: string(domain.ProviderIDMonobank),
			Reference: "secret-reference-" + fake.UUID().V4(),
			Envelope: credentials.Envelope{
				KeyVersion: "v1", Algorithm: credentials.AlgorithmAESGCM,
				Nonce: "nonce-" + fake.UUID().V4(), Ciphertext: "ciphertext-" + fake.UUID().V4(),
			},
		}
		_, err = store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID: connection.ConnectionID, TenantID: tenantID, Provider: string(connection.ProviderID),
			ConnectorID: connection.ConnectorID, ProviderReference: connection.ProviderReference, SecretID: secret.ID,
			State: domain.BankConnectionStateActive, CreatedAt: now, UpdatedAt: now,
		})
		require.NoError(t, err)
		providerAccountID := "provider-account-" + fake.UUID().V4()
		accessToken := "token-" + fake.UUID().V4()
		var fetchedWindows []domain.ProviderSyncWindow
		middleFetchFailed := false
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch {
			case request.URL.Path == "/personal/client-info":
				assert.Equal(t, accessToken, request.Header.Get("X-Token"))
				_, _ = fmt.Fprintf(
					writer,
					`{"name":"%s","accounts":[{"id":"%s","type":"black","currencyCode":980,"balance":12345}]}`,
					"client-"+fake.UUID().V4(),
					providerAccountID,
				)
			case strings.HasPrefix(request.URL.Path, "/personal/statement/"):
				pathParts := strings.Split(strings.TrimPrefix(request.URL.Path, "/personal/statement/"), "/")
				if !assert.Len(t, pathParts, 3) {
					return
				}
				assert.Equal(t, providerAccountID, pathParts[0])
				startUnix, parseErr := strconv.ParseInt(pathParts[1], 10, 64)
				if !assert.NoError(t, parseErr) {
					return
				}
				endUnix, parseErr := strconv.ParseInt(pathParts[2], 10, 64)
				if !assert.NoError(t, parseErr) {
					return
				}
				requestedWindow := domain.ProviderSyncWindow{
					Start: time.Unix(startUnix, 0).In(now.Location()),
					End:   time.Unix(endUnix, 0).In(now.Location()),
				}
				fetchedWindows = append(fetchedWindows, requestedWindow)
				if requestedWindow.Start.Equal(chunks[1].Start) && !middleFetchFailed {
					middleFetchFailed = true
					http.Error(writer, "middle fetch failed", http.StatusInternalServerError)
					return
				}
				_, _ = fmt.Fprintf(
					writer,
					`[{"id":"transaction-%d","time":%d,"description":"transaction-%s","currencyCode":980,"amount":-250,"balance":12095}]`,
					startUnix,
					startUnix+int64(time.Hour/time.Second),
					fake.Lorem().Word(),
				)
			default:
				http.NotFound(writer, request)
			}
		}))
		defer server.Close()
		connector := monobank.NewConnector(
			monobank.Args{BaseURL: server.URL, HTTPClient: server.Client(), Logger: slog.New(slog.DiscardHandler)},
			monobank.WithNow(func() time.Time { return now }),
			monobank.WithSecretTokenResolver(func(context.Context, domain.ConnectionSecret) (string, error) {
				return accessToken, nil
			}),
		)
		windowStore, err := providers.NewProviderWindowSyncStore(
			NewProviderWindowSyncPersistence(store),
			providers.WithWindowSyncStoreIDGenerator(func() string { return "id-" + fake.UUID().V4() }),
			providers.WithWindowSyncStoreNow(func() time.Time { return now }),
		)
		require.NoError(t, err)
		executor, err := providers.NewWindowSyncExecutor(
			providers.WithConnectors(connector),
			providers.WithWindowSyncStore(windowStore),
			providers.WithRunIDGenerator(func() string { return "run-" + fake.UUID().V4() }),
			providers.WithWindowSyncExecutorNow(func() time.Time { return now }),
		)
		require.NoError(t, err)
		journal := NewProviderSyncStateJournalStore(store)
		orchestrator, err := providers.NewSyncOrchestrator(providers.SyncOrchestratorParams{
			SyncStateJournal:   journal,
			TargetWindowPolicy: providers.NewCheckpointTargetWindowPolicy(),
			WindowChunkPolicy:  chunkPolicy,
			WindowExecutor:     executor,
			Logger:             slog.New(slog.DiscardHandler),
		}, providers.WithNow(func() time.Time { return now }))
		require.NoError(t, err)

		initialRequest := providers.SyncOrchestrationRequest{
			Connection: connection, Secret: secret, JobID: "job-" + fake.UUID().V4(), Reason: "scheduled",
			WindowStart: &initialWindow.Start, WindowEnd: &initialWindow.End,
		}
		_, err = orchestrator.Orchestrate(t.Context(), initialRequest)
		require.Error(t, err)
		require.True(t, middleFetchFailed)

		mappings, err := store.ListConnectionProviderAccounts(t.Context(), connection.ConnectionID)
		require.NoError(t, err)
		require.Len(t, mappings, 1)
		transactions, err := store.ListTransactions(
			t.Context(), tenantID, mappings[0].FinanceAccountID, domain.TransactionSourceProvider, "", false,
		)
		require.NoError(t, err)
		require.Len(t, transactions, 1)
		balanceSnapshots, err := store.ListBalanceSnapshots(t.Context(), connection.ConnectionID)
		require.NoError(t, err)
		require.Len(t, balanceSnapshots, 1)
		providerSnapshots, err := NewProviderSnapshotStoreFromStore(store).ListProviderSnapshotsByConnection(
			t.Context(),
			connection.ConnectionID,
		)
		require.NoError(t, err)
		require.Len(t, providerSnapshots, 3)
		var failedRunStates []providerSyncStateJournalModel
		require.NoError(t, store.db.WithContext(t.Context()).
			Where("connection_id = ?", connection.ConnectionID).
			Order("journal_id ASC").
			Find(&failedRunStates).Error)
		require.Len(t, failedRunStates, 2)
		assert.True(t, failedRunStates[0].WindowStart.Equal(chunks[0].Start))
		assert.True(t, failedRunStates[0].WindowEnd.Equal(chunks[0].End))
		assert.NotNil(t, failedRunStates[0].SucceededAt)
		assert.Empty(t, failedRunStates[0].ErrorSummary)
		assert.True(t, failedRunStates[1].WindowStart.Equal(chunks[1].Start))
		assert.Nil(t, failedRunStates[1].SucceededAt)
		assert.NotEmpty(t, failedRunStates[1].ErrorSummary)

		retryRequest := initialRequest
		retryRequest.WindowStart = nil
		retryRequest.WindowEnd = nil
		retryResult, err := orchestrator.Orchestrate(t.Context(), retryRequest)
		require.NoError(t, err)
		require.True(t, retryResult.TargetWindow.Start.Equal(chunks[1].Start))
		require.True(t, retryResult.TargetWindow.End.Equal(now))
		require.Len(t, retryResult.ExecutedWindows, 2)
		assert.True(t, retryResult.ExecutedWindows[0].Start.Equal(chunks[1].Start))
		assert.True(t, retryResult.ExecutedWindows[1].Start.Equal(chunks[2].Start))
		require.Len(t, fetchedWindows, 4)
		for index, expectedWindow := range []domain.ProviderSyncWindow{
			chunks[0], chunks[1], chunks[1], chunks[2],
		} {
			assert.True(t, fetchedWindows[index].Start.Equal(expectedWindow.Start))
			assert.True(t, fetchedWindows[index].End.Equal(expectedWindow.End))
		}

		mappings, err = store.ListConnectionProviderAccounts(t.Context(), connection.ConnectionID)
		require.NoError(t, err)
		require.Len(t, mappings, 1)
		transactions, err = store.ListTransactions(
			t.Context(), tenantID, mappings[0].FinanceAccountID, domain.TransactionSourceProvider, "", false,
		)
		require.NoError(t, err)
		require.Len(t, transactions, 3)
		balanceSnapshots, err = store.ListBalanceSnapshots(t.Context(), connection.ConnectionID)
		require.NoError(t, err)
		require.Len(t, balanceSnapshots, 3)
		providerSnapshots, err = NewProviderSnapshotStoreFromStore(store).ListProviderSnapshotsByConnection(
			t.Context(),
			connection.ConnectionID,
		)
		require.NoError(t, err)
		require.Len(t, providerSnapshots, 5)
		var completedRunStates []providerSyncStateJournalModel
		require.NoError(t, store.db.WithContext(t.Context()).
			Where("connection_id = ?", connection.ConnectionID).
			Order("journal_id ASC").
			Find(&completedRunStates).Error)
		require.Len(t, completedRunStates, 4)
		assert.True(t, completedRunStates[0].WindowStart.Equal(chunks[0].Start))
		assert.True(t, completedRunStates[1].WindowStart.Equal(chunks[1].Start))
		assert.True(t, completedRunStates[2].WindowStart.Equal(chunks[1].Start))
		assert.True(t, completedRunStates[3].WindowStart.Equal(chunks[2].Start))
		assert.NotNil(t, completedRunStates[0].SucceededAt)
		assert.Nil(t, completedRunStates[1].SucceededAt)
		assert.NotNil(t, completedRunStates[2].SucceededAt)
		assert.NotNil(t, completedRunStates[3].SucceededAt)
	})
}
