package finance

import (
	"errors"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestProviderEvidenceService(t *testing.T) {
	fake := faker.New()

	type fixture struct {
		ownerUserID     string
		outsiderID      string
		tenant          domain.Tenant
		account         domain.Account
		otherAccount    domain.Account
		transaction     domain.Transaction
		connection      domain.BankConnection
		providerAccount domain.ConnectionProviderAccount
		store           *persistence.Store
		evidence        *persistence.ProviderEvidenceStore
		service         *ProviderEvidenceService
		now             time.Time
	}

	makeFixture := func(t *testing.T) fixture {
		t.Helper()
		database := openTestDatabase(t)
		store := persistence.NewStore(database)
		evidenceStore := persistence.NewProviderEvidenceStore(database)
		now := time.Date(2026, time.July, 14, 10, 0, 0, 0, time.FixedZone("fixture", 2*60*60))
		ownerUserID := "owner-" + fake.UUID().V4()
		tenant := domain.Tenant{
			ID:              "tenant-" + fake.UUID().V4(),
			Name:            "tenant-" + fake.Lorem().Word(),
			DisplayCurrency: "PLN",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		_, err := store.SaveTenant(t.Context(), tenant)
		require.NoError(t, err)
		_, err = store.SaveTenantMembership(t.Context(), domain.TenantMembership{
			TenantID: tenant.ID, UserID: ownerUserID, JoinedAt: now, CreatedAt: now,
		})
		require.NoError(t, err)
		makeAccount := func(prefix string) domain.Account {
			return domain.Account{
				ID: "account-" + prefix + "-" + fake.UUID().V4(), TenantID: tenant.ID,
				Name: prefix + "-" + fake.Lorem().Word(), Currency: "PLN", Kind: domain.AccountKindLinked,
				CreatedAt: now, UpdatedAt: now,
			}
		}
		account := makeAccount("primary")
		otherAccount := makeAccount("other")
		_, err = store.SaveAccount(t.Context(), account)
		require.NoError(t, err)
		_, err = store.SaveAccount(t.Context(), otherAccount)
		require.NoError(t, err)
		transaction := domain.Transaction{
			ID: "transaction-" + fake.UUID().V4(), TenantID: tenant.ID, AccountID: account.ID,
			Source: domain.TransactionSourceProvider, Status: domain.TransactionStatusBooked,
			Kind: domain.TransactionKindRegular, AmountMinor: -123, Currency: "PLN",
			Description: "transaction-" + fake.Lorem().Word(), EffectiveAt: now, CreatedAt: now, UpdatedAt: now,
		}
		_, err = store.SaveTransaction(t.Context(), transaction)
		require.NoError(t, err)
		connection := domain.BankConnection{
			ID: "connection-" + fake.UUID().V4(), TenantID: tenant.ID, Provider: "pko",
			ConnectorID: domain.ProviderConnectorIDEnableBanking, DisplayName: "provider-" + fake.Lorem().Word(),
			ProviderReference: "reference-" + fake.UUID().V4(), SecretID: "secret-" + fake.UUID().V4(),
			State: domain.BankConnectionStateActive, CreatedAt: now, UpdatedAt: now,
		}
		_, err = store.SaveBankConnection(t.Context(), connection)
		require.NoError(t, err)
		providerAccount := domain.ConnectionProviderAccount{
			ID: "provider-account-" + fake.UUID().V4(), ConnectionID: connection.ID,
			ProviderAccountID: "provider-account-id-" + fake.UUID().V4(), FinanceAccountID: account.ID,
			Name: account.Name, Currency: account.Currency, CreatedAt: now, UpdatedAt: now,
		}
		_, err = store.SaveConnectionProviderAccount(t.Context(), providerAccount)
		require.NoError(t, err)
		return fixture{
			ownerUserID: ownerUserID, outsiderID: "outsider-" + fake.UUID().V4(), tenant: tenant,
			account: account, otherAccount: otherAccount, transaction: transaction, connection: connection,
			providerAccount: providerAccount, store: store, evidence: evidenceStore,
			service: NewProviderEvidenceService(evidenceStore), now: now,
		}
	}

	t.Run("authorizes bounded reads and redacts nested credential-like values", func(t *testing.T) {
		data := makeFixture(t)
		accountEvidence := domain.ProviderEvidence{
			ID:               "account-evidence-" + fake.UUID().V4(),
			TenantID:         data.tenant.ID,
			ConnectionID:     data.connection.ID,
			FinanceAccountID: data.account.ID,
			Subject:          domain.ProviderEvidenceSubjectAccount,
			Scope:            domain.RawPayloadScopeAccount,
			ProviderObjectID: data.providerAccount.ProviderAccountID,
			PayloadJSON: []byte(
				`{"visible":"ok","access_token":"not-stored","nested":{"clientSecret":"not-stored"},"items":[{"authorization":"not-stored","name":"kept"}]}`,
			),
			CapturedAt: data.now,
		}
		_, err := data.evidence.SaveProviderEvidence(t.Context(), accountEvidence)
		require.NoError(t, err)
		_, err = data.evidence.SaveProviderEvidence(t.Context(), domain.ProviderEvidence{
			ID: "invalid-evidence-" + fake.UUID().V4(), TenantID: data.tenant.ID,
			PayloadJSON: []byte("not-json"), CapturedAt: data.now,
		})
		require.Error(t, err)
		repeatedEvidence, err := data.evidence.SaveProviderEvidence(t.Context(), accountEvidence)
		require.NoError(t, err)
		assert.Equal(t, accountEvidence.ID, repeatedEvidence.ID)
		_, err = data.evidence.SaveProviderEvidence(t.Context(), domain.ProviderEvidence{
			ID: "other-evidence-" + fake.UUID().V4(), TenantID: data.tenant.ID,
			ConnectionID: data.connection.ID, FinanceAccountID: data.otherAccount.ID,
			Subject: domain.ProviderEvidenceSubjectAccount, Scope: domain.RawPayloadScopeAccount,
			PayloadJSON: []byte(`{"visible":"other"}`), CapturedAt: data.now,
		})
		require.NoError(t, err)

		items, err := data.service.ListAccountProviderEvidence(t.Context(), ListAccountProviderEvidenceParams{
			ActorUserID: data.ownerUserID, TenantID: data.tenant.ID, AccountID: data.account.ID,
		})
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, accountEvidence.ID, items[0].ID)

		detail, err := data.service.GetAccountProviderEvidence(t.Context(), GetAccountProviderEvidenceParams{
			ActorUserID: data.ownerUserID, TenantID: data.tenant.ID, AccountID: data.account.ID,
			EvidenceID: accountEvidence.ID,
		})
		require.NoError(t, err)
		assert.JSONEq(t, `{"visible":"ok","nested":{},"items":[{"name":"kept"}]}`, string(detail.PayloadJSON))

		_, err = data.service.ListAccountProviderEvidence(t.Context(), ListAccountProviderEvidenceParams{
			ActorUserID: data.outsiderID, TenantID: data.tenant.ID, AccountID: data.account.ID,
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
		_, err = data.service.ListAccountProviderEvidence(t.Context(), ListAccountProviderEvidenceParams{
			ActorUserID: data.ownerUserID, TenantID: data.tenant.ID, AccountID: "missing-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrAccountNotFound)

		_, err = data.service.GetAccountProviderEvidence(t.Context(), GetAccountProviderEvidenceParams{
			ActorUserID: data.ownerUserID, TenantID: data.tenant.ID, AccountID: data.otherAccount.ID,
			EvidenceID: accountEvidence.ID,
		})
		require.ErrorIs(t, err, ErrProviderEvidenceNotFound)
		_, err = data.service.GetAccountProviderEvidence(t.Context(), GetAccountProviderEvidenceParams{
			ActorUserID: data.ownerUserID, TenantID: data.tenant.ID, AccountID: "missing-" + fake.UUID().V4(),
			EvidenceID: accountEvidence.ID,
		})
		require.ErrorIs(t, err, ErrAccountNotFound)
	})

	t.Run("captures entity observations separately and hides page and legacy evidence", func(t *testing.T) {
		data := makeFixture(t)
		evidenceIDs := make([]string, 8)
		for index := range evidenceIDs {
			evidenceIDs[index] = "evidence-" + fake.UUID().V4()
		}
		evidenceIDIndex := 0
		syncService := NewBankSyncService(
			data.store,
			WithBankSyncServiceNow(func() time.Time { return data.now }),
			WithBankSyncServiceIDGenerator(func() string {
				id := evidenceIDs[evidenceIDIndex]
				evidenceIDIndex++
				return id
			}),
			WithBankSyncServiceEvidenceWriter(data.evidence),
		)
		_, _, err := syncService.applyProviderTransactions(
			t.Context(), data.connection,
			map[string]domain.ConnectionProviderAccount{data.providerAccount.ProviderAccountID: data.providerAccount},
			[]ProviderNormalizedTransaction{{
				ProviderAccountID:     data.providerAccount.ProviderAccountID,
				ProviderTransactionID: "provider-transaction-" + fake.UUID().V4(),
				Status:                domain.TransactionStatusBooked, AmountMinor: -321, Currency: "PLN",
				Description: "transaction-" + fake.Lorem().Word(), EffectiveAt: data.now,
				Fingerprint:    "fingerprint-" + fake.UUID().V4(),
				RawPayloadJSON: []byte(`{"transaction":"observed","refreshToken":"not-stored"}`),
			}}, data.now,
		)
		require.NoError(t, err)
		transactions, err := data.store.ListTransactions(t.Context(), data.tenant.ID, data.account.ID, "", "", false)
		require.NoError(t, err)
		require.Len(t, transactions, 2)
		capturedTransactionID := transactions[0].ID
		if capturedTransactionID == data.transaction.ID {
			capturedTransactionID = transactions[1].ID
		}
		transactionEvidence, err := data.service.ListTransactionProviderEvidence(
			t.Context(),
			ListTransactionProviderEvidenceParams{
				ActorUserID:   data.ownerUserID,
				TenantID:      data.tenant.ID,
				TransactionID: capturedTransactionID,
			},
		)
		require.NoError(t, err)
		require.Len(t, transactionEvidence, 1)
		assert.Equal(t, domain.ProviderEvidenceSubjectTransaction, transactionEvidence[0].Subject)
		assert.Equal(t, domain.RawPayloadScopeTransaction, transactionEvidence[0].Scope)
		assert.NotContains(t, string(transactionEvidence[0].PayloadJSON), "not-stored")
		transactionDetail, err := data.service.GetTransactionProviderEvidence(
			t.Context(),
			GetTransactionProviderEvidenceParams{
				ActorUserID:   data.ownerUserID,
				TenantID:      data.tenant.ID,
				TransactionID: capturedTransactionID,
				EvidenceID:    transactionEvidence[0].ID,
			},
		)
		require.NoError(t, err)
		assert.Equal(t, transactionEvidence[0], transactionDetail)
		_, err = data.service.ListTransactionProviderEvidence(t.Context(), ListTransactionProviderEvidenceParams{
			ActorUserID: data.outsiderID, TenantID: data.tenant.ID, TransactionID: capturedTransactionID,
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
		_, err = data.service.ListTransactionProviderEvidence(t.Context(), ListTransactionProviderEvidenceParams{
			ActorUserID: data.ownerUserID, TenantID: data.tenant.ID, TransactionID: "missing-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrTransactionNotFound)
		_, err = data.service.GetTransactionProviderEvidence(t.Context(), GetTransactionProviderEvidenceParams{
			ActorUserID: data.ownerUserID, TenantID: data.tenant.ID, TransactionID: capturedTransactionID,
			EvidenceID: "missing-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrProviderEvidenceNotFound)

		pagePayload := []byte(`{"transactions":[{"id":"page-item"}],"token":"not-stored"}`)
		require.NoError(t, syncService.persistPageProviderEvidence(
			t.Context(), data.connection.ID, ProviderRawPayload{
				Scope: domain.RawPayloadScopeTransaction, ProviderObjectID: data.providerAccount.ProviderAccountID,
			}, pagePayload, data.now,
		))
		accountEvidence, err := data.service.ListAccountProviderEvidence(t.Context(), ListAccountProviderEvidenceParams{
			ActorUserID: data.ownerUserID, TenantID: data.tenant.ID, AccountID: data.account.ID,
		})
		require.NoError(t, err)
		assert.Empty(t, accountEvidence)
		_, err = data.service.GetAccountProviderEvidence(t.Context(), GetAccountProviderEvidenceParams{
			ActorUserID: data.ownerUserID, TenantID: data.tenant.ID, AccountID: data.account.ID,
			EvidenceID: evidenceIDs[4],
		})
		require.ErrorIs(t, err, ErrProviderEvidenceNotFound)
		_, err = data.service.GetTransactionProviderEvidence(t.Context(), GetTransactionProviderEvidenceParams{
			ActorUserID: data.ownerUserID, TenantID: data.tenant.ID, TransactionID: capturedTransactionID,
			EvidenceID: evidenceIDs[4],
		})
		require.ErrorIs(t, err, ErrProviderEvidenceNotFound)

		require.NoError(t, syncService.persistPageProviderEvidence(
			t.Context(), data.connection.ID, ProviderRawPayload{
				Scope: domain.RawPayloadScopeAccount, ProviderObjectID: data.providerAccount.ProviderAccountID,
			}, []byte(`{"account":"observed"}`), data.now,
		))
		accountEvidence, err = data.service.ListAccountProviderEvidence(t.Context(), ListAccountProviderEvidenceParams{
			ActorUserID: data.ownerUserID, TenantID: data.tenant.ID, AccountID: data.account.ID,
		})
		require.NoError(t, err)
		require.Len(t, accountEvidence, 1)
		assert.Equal(t, domain.ProviderEvidenceSubjectAccount, accountEvidence[0].Subject)
		assert.Equal(t, domain.RawPayloadScopeAccount, accountEvidence[0].Scope)

		legacyEvidence := domain.ProviderEvidence{
			ID: "legacy-evidence-" + fake.UUID().V4(), TenantID: data.tenant.ID,
			ConnectionID: data.connection.ID, FinanceAccountID: data.account.ID,
			FinanceTransactionID: capturedTransactionID, Scope: domain.RawPayloadScopeTransaction,
			PayloadJSON: []byte(`{"legacy":"retained"}`), CapturedAt: data.now,
		}
		_, err = data.evidence.SaveProviderEvidence(t.Context(), legacyEvidence)
		require.NoError(t, err)
		accountEvidence, err = data.service.ListAccountProviderEvidence(t.Context(), ListAccountProviderEvidenceParams{
			ActorUserID: data.ownerUserID, TenantID: data.tenant.ID, AccountID: data.account.ID,
		})
		require.NoError(t, err)
		require.Len(t, accountEvidence, 1)
		assert.Equal(t, domain.ProviderEvidenceSubjectAccount, accountEvidence[0].Subject)
		transactionEvidence, err = data.service.ListTransactionProviderEvidence(t.Context(),
			ListTransactionProviderEvidenceParams{
				ActorUserID: data.ownerUserID, TenantID: data.tenant.ID, TransactionID: capturedTransactionID,
			})
		require.NoError(t, err)
		require.Len(t, transactionEvidence, 1)
		assert.Equal(t, domain.ProviderEvidenceSubjectTransaction, transactionEvidence[0].Subject)

		require.NoError(t, syncService.DeleteBankConnection(t.Context(), DeleteBankConnectionParams{
			ActorUserID:  data.ownerUserID,
			TenantID:     data.tenant.ID,
			ConnectionID: data.connection.ID,
		}))
		accountEvidence, err = data.service.ListAccountProviderEvidence(
			t.Context(),
			ListAccountProviderEvidenceParams{
				ActorUserID: data.ownerUserID,
				TenantID:    data.tenant.ID,
				AccountID:   data.account.ID,
			},
		)
		require.NoError(t, err)
		assert.Empty(t, accountEvidence)
	})

	t.Run("keeps evidence and raw payload storage bounded across repeated sync observations", func(t *testing.T) {
		data := makeFixture(t)
		syncService := NewBankSyncService(
			data.store,
			WithBankSyncServiceNow(func() time.Time { return data.now }),
			WithBankSyncServiceIDGenerator(func() string { return "id-" + fake.UUID().V4() }),
			WithBankSyncServiceEvidenceWriter(data.evidence),
		)
		observation := ProviderNormalizedTransaction{
			ProviderAccountID:     data.providerAccount.ProviderAccountID,
			ProviderTransactionID: "provider-transaction-" + fake.UUID().V4(),
			Fingerprint:           "fingerprint-" + fake.UUID().V4(),
			Status:                domain.TransactionStatusBooked,
			AmountMinor:           -321,
			Currency:              "PLN",
			Description:           "transaction-" + fake.Lorem().Word(),
			EffectiveAt:           data.now,
			RawPayloadJSON:        []byte(`{"value":"first","accessToken":"not-stored"}`),
		}
		_, _, err := syncService.applyProviderTransactions(
			t.Context(),
			data.connection,
			map[string]domain.ConnectionProviderAccount{
				data.providerAccount.ProviderAccountID: data.providerAccount,
			},
			[]ProviderNormalizedTransaction{observation},
			data.now,
		)
		require.NoError(t, err)
		transactions, err := data.store.ListTransactions(
			t.Context(),
			data.tenant.ID,
			data.account.ID,
			"",
			"",
			false,
		)
		require.NoError(t, err)
		require.Len(t, transactions, 2)
		transactionID := transactions[0].ID
		if transactionID == data.transaction.ID {
			transactionID = transactions[1].ID
		}
		firstEvidence, err := data.service.ListTransactionProviderEvidence(
			t.Context(),
			ListTransactionProviderEvidenceParams{
				ActorUserID:   data.ownerUserID,
				TenantID:      data.tenant.ID,
				TransactionID: transactionID,
			},
		)
		require.NoError(t, err)
		require.Len(t, firstEvidence, 1)

		observation.RawPayloadJSON = []byte(`{"value":"latest","refreshToken":"not-stored"}`)
		_, _, err = syncService.applyProviderTransactions(
			t.Context(),
			data.connection,
			map[string]domain.ConnectionProviderAccount{
				data.providerAccount.ProviderAccountID: data.providerAccount,
			},
			[]ProviderNormalizedTransaction{observation},
			data.now.Add(time.Minute),
		)
		require.NoError(t, err)
		currentEvidence, err := data.service.ListTransactionProviderEvidence(
			t.Context(),
			ListTransactionProviderEvidenceParams{
				ActorUserID:   data.ownerUserID,
				TenantID:      data.tenant.ID,
				TransactionID: transactionID,
			},
		)
		require.NoError(t, err)
		require.Len(t, currentEvidence, 1)
		assert.Equal(t, firstEvidence[0].ID, currentEvidence[0].ID)
		assert.JSONEq(t, `{"value":"latest"}`, string(currentEvidence[0].PayloadJSON))
		rawPayloads, err := data.store.ListRawPayloads(t.Context(), data.connection.ID)
		require.NoError(t, err)
		require.Len(t, rawPayloads, 1)
		assert.JSONEq(t, `{"value":"latest"}`, string(rawPayloads[0].PayloadJSON))
	})

	t.Run("maps dependency and malformed payload failures without exposing evidence", func(t *testing.T) {
		actorUserID := "owner-" + fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		accountID := "account-" + fake.UUID().V4()
		transactionID := "transaction-" + fake.UUID().V4()
		evidenceID := "evidence-" + fake.UUID().V4()

		t.Run("membership check failure", func(t *testing.T) {
			store := newMockproviderEvidenceServiceStore(t)
			store.EXPECT().IsTenantMember(mock.Anything, tenantID, actorUserID).
				Return(false, errors.New("membership unavailable")).Once()
			_, err := NewProviderEvidenceService(store).ListAccountProviderEvidence(
				t.Context(),
				ListAccountProviderEvidenceParams{
					ActorUserID: actorUserID,
					TenantID:    tenantID,
					AccountID:   accountID,
				},
			)
			require.ErrorContains(t, err, "membership unavailable")
		})

		t.Run("list rejects malformed payload", func(t *testing.T) {
			store := newMockproviderEvidenceServiceStore(t)
			store.EXPECT().IsTenantMember(mock.Anything, tenantID, actorUserID).Return(true, nil).Once()
			store.EXPECT().ListAccountProviderEvidence(mock.Anything, tenantID, accountID).
				Return([]domain.ProviderEvidence{{ID: evidenceID, PayloadJSON: []byte("not-json")}}, nil).Once()
			_, err := NewProviderEvidenceService(store).ListAccountProviderEvidence(
				t.Context(),
				ListAccountProviderEvidenceParams{
					ActorUserID: actorUserID,
					TenantID:    tenantID,
					AccountID:   accountID,
				},
			)
			require.ErrorContains(t, err, "sanitize provider evidence response")
		})

		t.Run("detail maps unexpected store failures and malformed payloads", func(t *testing.T) {
			store := newMockproviderEvidenceServiceStore(t)
			service := NewProviderEvidenceService(store)
			store.EXPECT().IsTenantMember(mock.Anything, tenantID, actorUserID).Return(true, nil).Twice()
			store.EXPECT().GetTransactionProviderEvidence(mock.Anything, tenantID, transactionID, evidenceID).
				Return(domain.ProviderEvidence{}, errors.New("read unavailable")).Once()
			_, err := service.GetTransactionProviderEvidence(
				t.Context(),
				GetTransactionProviderEvidenceParams{
					ActorUserID:   actorUserID,
					TenantID:      tenantID,
					TransactionID: transactionID,
					EvidenceID:    evidenceID,
				},
			)
			require.ErrorContains(t, err, "read unavailable")

			store.EXPECT().GetTransactionProviderEvidence(mock.Anything, tenantID, transactionID, evidenceID).
				Return(domain.ProviderEvidence{PayloadJSON: []byte("not-json")}, nil).Once()
			_, err = service.GetTransactionProviderEvidence(
				t.Context(),
				GetTransactionProviderEvidenceParams{
					ActorUserID:   actorUserID,
					TenantID:      tenantID,
					TransactionID: transactionID,
					EvidenceID:    evidenceID,
				},
			)
			require.ErrorContains(t, err, "sanitize provider evidence response")
		})
	})
}
