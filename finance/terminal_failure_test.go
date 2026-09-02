//go:build postgres_test

package finance

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	internalproviders "github.com/gemyago/sumweave/finance/internal/providers"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObservedServiceFailureClassification(t *testing.T) {
	fake := faker.New()
	assertTerminal := func(t *testing.T, err error, code string, details string) {
		t.Helper()
		failure, ok := TerminalFailureFrom(err)
		require.True(t, ok)
		assert.Equal(t, code, failure.Code)
		assert.Equal(t, details, failure.Details)
	}
	assertInfrastructure := func(t *testing.T, err error) {
		t.Helper()
		_, ok := TerminalFailureFrom(err)
		assert.False(t, ok)
	}
	require.NoError(t, NewTerminalFailure(nil, "", "", ""))

	t.Run("transaction and account CSV imports classify invalid state but not store failures", func(t *testing.T) {
		store := persistence.NewStore(openTestDatabase(t))
		service := NewCSVImportService(store, nil, nil)
		for _, importType := range []CSVImportType{CSVImportTypeTransactions, CSVImportTypeAccounts} {
			record := domain.CSVImportRecord{
				ID:        fake.UUID().V4(),
				Type:      importType,
				Status:    CSVImportStatusPreviewed,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			_, err := store.SaveCSVImport(t.Context(), record)
			require.NoError(t, err)

			_, err = service.RunCSVImportJob(t.Context(), RunCSVImportJobParams{ImportID: record.ID})
			assertTerminal(t, err, "csv_import_not_runnable", "The CSV import is not confirmed.")
		}
		assertInfrastructure(t, terminalCSVImportJobFailure(errors.New("store unavailable "+fake.UUID().V4())))
		assertTerminal(
			t,
			terminalCSVImportJobFailure(persistence.ErrCSVImportNotFound),
			"csv_import_not_found",
			"The CSV import no longer exists.",
		)
		assertTerminal(
			t,
			terminalCSVImportJobFailure(ErrInvalidCSVImport),
			"csv_import_invalid",
			"The stored CSV import data is invalid.",
		)
	})

	t.Run("bank sync classifies missing connection and provider rejection but not infrastructure", func(t *testing.T) {
		store := persistence.NewStore(openTestDatabase(t))
		service := NewBankSyncService(store, newMockbankSyncOrchestrator(t))

		_, err := service.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
			ConnectionID: fake.UUID().V4(),
		})
		assertTerminal(t, err, "bank_connection_not_found", "The bank connection no longer exists.")

		assertTerminal(
			t,
			terminalBankConnectionSyncFailure(&ProviderResponseError{
				Provider:   "provider-" + fake.Letter(),
				Operation:  "sync",
				StatusCode: http.StatusUnauthorized,
			}),
			"bank_provider_rejected_request",
			"The bank provider rejected the synchronization request.",
		)
		assertInfrastructure(t, terminalBankConnectionSyncFailure(errors.New("store unavailable "+fake.UUID().V4())))
		assertTerminal(
			t,
			terminalBankConnectionSyncFailure(persistence.ErrConnectionSecretNotFound),
			"bank_connection_credentials_missing",
			"The bank connection credentials are no longer available.",
		)
		assertTerminal(
			t,
			terminalBankConnectionSyncFailure(internalproviders.ErrConnectorNotConfigured),
			"bank_sync_configuration_invalid",
			"The bank connection configuration cannot be synchronized.",
		)
	})

	t.Run(
		"FX refresh classifies provider configuration and response but not provider availability",
		func(t *testing.T) {
			database := openTestDatabase(t)
			service := NewFXService(
				persistence.NewCurrentFXRateStore(database),
				WithFXServiceRequiredPairs(persistence.NewFXPairDiscoveryStore(database)),
			)

			_, err := service.RefreshRequiredFXRates(t.Context(), RefreshFXRatesParams{
				Provider: "missing-" + fake.UUID().V4(),
			})
			assertTerminal(t, err, "fx_provider_not_configured", "The requested FX provider is not available.")

			assertTerminal(
				t,
				terminalFXRefreshFailure(&ProviderResponseError{
					Provider:   "provider-" + fake.Letter(),
					Operation:  "fetch rates",
					StatusCode: http.StatusBadRequest,
				}),
				"fx_provider_rejected_request",
				"The FX provider rejected the rate refresh request.",
			)
			assertInfrastructure(t, terminalFXRefreshFailure(errors.New("provider unavailable "+fake.UUID().V4())))
		},
	)
}
