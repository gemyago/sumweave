package domain

import (
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderSnapshot(t *testing.T) {
	fake := faker.New()

	makeSnapshot := func() ProviderSnapshot {
		return ProviderSnapshot{
			ID:               "snapshot-" + fake.UUID().V4(),
			TenantID:         "tenant-" + fake.UUID().V4(),
			ConnectionID:     "connection-" + fake.UUID().V4(),
			Subject:          ProviderSnapshotSubjectAccount,
			FinanceAccountID: "account-" + fake.UUID().V4(),
			Kind:             ProviderSnapshotKindAccount,
			ProviderObjectID: "provider-account-" + fake.UUID().V4(),
			DocumentJSON: []byte(
				`{"account":"observed","accessToken":"not-stored","nested":{"private_key":"not-stored"}}`,
			),
			CapturedAt: time.Date(2026, time.August, 14, 10, 0, 0, 0, time.FixedZone("fixture", 2*60*60)),
		}
	}

	t.Run("validates required identity and subject attachments", func(t *testing.T) {
		connection := makeSnapshot()
		connection.Subject = ProviderSnapshotSubjectConnection
		connection.Kind = ProviderSnapshotKindConnection
		connection.FinanceAccountID = ""
		require.NoError(t, connection.Validate())

		account := makeSnapshot()
		require.NoError(t, account.Validate())

		transaction := makeSnapshot()
		transaction.Subject = ProviderSnapshotSubjectTransaction
		transaction.Kind = ProviderSnapshotKindTransaction
		transaction.FinanceTransactionID = "transaction-" + fake.UUID().V4()
		require.NoError(t, transaction.Validate())

		for _, invalid := range []ProviderSnapshot{
			func() ProviderSnapshot {
				item := connection
				item.FinanceAccountID = "account-" + fake.UUID().V4()
				return item
			}(),
			func() ProviderSnapshot {
				item := account
				item.FinanceTransactionID = "transaction-" + fake.UUID().V4()
				return item
			}(),
			func() ProviderSnapshot {
				item := transaction
				item.FinanceTransactionID = ""
				return item
			}(),
			func() ProviderSnapshot {
				item := account
				item.ProviderObjectID = " "
				return item
			}(),
			func() ProviderSnapshot {
				item := account
				item.Kind = ProviderSnapshotKind("unsupported")
				return item
			}(),
			func() ProviderSnapshot {
				item := connection
				item.Kind = ProviderSnapshotKindTransaction
				return item
			}(),
			func() ProviderSnapshot {
				item := account
				item.Kind = ProviderSnapshotKindTransaction
				return item
			}(),
			func() ProviderSnapshot {
				item := transaction
				item.Kind = ProviderSnapshotKindAccountBalance
				return item
			}(),
			func() ProviderSnapshot {
				item := account
				item.Subject = ProviderSnapshotSubject("unsupported")
				return item
			}(),
			func() ProviderSnapshot {
				item := account
				item.FinanceAccountID = ""
				return item
			}(),
			func() ProviderSnapshot {
				item := transaction
				item.FinanceAccountID = ""
				return item
			}(),
			func() ProviderSnapshot {
				item := account
				item.CapturedAt = time.Time{}
				return item
			}(),
			func() ProviderSnapshot {
				item := account
				item.DocumentJSON = []byte("not-json")
				return item
			}(),
		} {
			require.Error(t, invalid.Validate())
		}
	})

	t.Run("sanitizes documents without changing supported document values", func(t *testing.T) {
		snapshot := makeSnapshot()
		sanitized, err := SanitizeProviderSnapshotJSON(snapshot.DocumentJSON)
		require.NoError(t, err)
		assert.JSONEq(t, `{"account":"observed","nested":{}}`, string(sanitized))
		sanitized, err = SanitizeProviderSnapshotJSON([]byte(`[
            {
              "bearer":"not-stored",
              "api_key":"not-stored",
              "accessKey":"not-stored",
              "password":"not-stored",
              "credentials":"not-stored",
              "nested":{"passphrase":"not-stored","value":"kept"}
            }
        ]`))
		require.NoError(t, err)
		assert.JSONEq(t, `[{"nested":{"value":"kept"}}]`, string(sanitized))
		_, err = SanitizeProviderSnapshotJSON([]byte("not-json"))
		require.Error(t, err)
	})
}
