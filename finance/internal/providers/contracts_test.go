package providers

import (
	"context"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/credentials"
	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubConnector struct {
	connectorID      domain.ProviderConnectorID
	forceConnectorID bool
	capabilities     ConnectorCapabilities
	startResult      StartLinkResult
	finishResult     LinkResult
	tokenResult      LinkResult
	fetchResult      domain.ProviderSyncBatch
	fetchErr         error
	fetchCalls       int
	lastFetch        FetchRequest
	lastStart        StartLinkRequest
	lastFinish       FinishLinkRequest
	lastToken        LinkTokenRequest
}

func (c *stubConnector) ConnectorID() domain.ProviderConnectorID {
	if c.forceConnectorID || c.connectorID != "" {
		return c.connectorID
	}
	return domain.ProviderConnectorIDEnableBanking
}

func (c *stubConnector) Capabilities() ConnectorCapabilities {
	return c.capabilities
}

func (c *stubConnector) StartLink(_ context.Context, request StartLinkRequest) (StartLinkResult, error) {
	c.lastStart = request
	return c.startResult, nil
}

func (c *stubConnector) FinishLink(_ context.Context, request FinishLinkRequest) (LinkResult, error) {
	c.lastFinish = request
	return c.finishResult, nil
}

func (c *stubConnector) LinkToken(_ context.Context, request LinkTokenRequest) (LinkResult, error) {
	c.lastToken = request
	return c.tokenResult, nil
}

func (c *stubConnector) Fetch(_ context.Context, request FetchRequest) (domain.ProviderSyncBatch, error) {
	c.fetchCalls++
	c.lastFetch = request
	if c.fetchErr != nil {
		return domain.ProviderSyncBatch{}, c.fetchErr
	}
	return c.fetchResult, nil
}

func TestProviderSyncV2Contracts(t *testing.T) {
	t.Run("connector supports linking fetching capabilities and pko composition", func(t *testing.T) {
		fake := faker.New()
		profile := PKOProfile()
		connection := makeRandomProviderConnectionRef(fake, profile.ProviderID, profile.ConnectorID)
		window := domain.ProviderSyncWindow{
			Start: time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, time.June, 24, 0, 0, 0, 0, time.UTC),
		}
		secret := domain.ConnectionSecret{
			ID:        "secret-" + fake.UUID().V4(),
			Provider:  string(profile.ProviderID),
			Reference: "reference-" + fake.UUID().V4(),
			Envelope: credentials.Envelope{
				KeyVersion: "v1",
				Algorithm:  credentials.AlgorithmAESGCM,
				Nonce:      "nonce",
				Ciphertext: "ciphertext",
			},
		}
		connector := &stubConnector{
			capabilities: ConnectorCapabilities{
				SupportsStartLink:  true,
				SupportsFinishLink: true,
				SupportsTokenLink:  false,
				SupportsFetch:      true,
			},
			startResult: StartLinkResult{
				State:            "state-" + fake.UUID().V4(),
				AuthorizationURL: "https://example.test/link/" + fake.UUID().V4(),
			},
			finishResult: LinkResult{
				DisplayName:       "PKO " + fake.Company().Name(),
				ProviderReference: "provider-ref-" + fake.UUID().V4(),
				ExternalID:        "external-" + fake.UUID().V4(),
				Secret:            "secret-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
			},
			tokenResult: LinkResult{
				DisplayName:       "token-link-" + fake.Lorem().Word(),
				ProviderReference: "provider-ref-" + fake.UUID().V4(),
				ExternalID:        "external-" + fake.UUID().V4(),
				Secret:            "secret-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
			},
			fetchResult: domain.ProviderSyncBatch{Connection: connection, RequestedWindow: window},
		}

		var contract Connector = connector
		startResult, err := contract.StartLink(t.Context(), StartLinkRequest{
			Profile:            profile,
			RedirectURL:        "https://app.example.test/redirect/" + fake.UUID().V4(),
			BrowserCallbackURL: "http://localhost:5173/#/finance/connections",
		})
		require.NoError(t, err)
		finishResult, err := contract.FinishLink(t.Context(), FinishLinkRequest{
			Profile: profile,
			State:   startResult.State,
			Code:    "code-" + fake.UUID().V4(),
			Start:   startResult,
		})
		require.NoError(t, err)
		tokenResult, err := contract.LinkToken(t.Context(), LinkTokenRequest{
			Profile: profile,
			Token:   "token-" + fake.UUID().V4(),
		})
		require.NoError(t, err)
		batch, err := contract.Fetch(t.Context(), FetchRequest{
			Connection:      connection,
			Secret:          secret,
			RequestedWindow: window,
		})
		require.NoError(t, err)

		assert.Equal(t, domain.ProviderIDPKO, profile.ProviderID)
		assert.Equal(t, domain.ProviderConnectorIDEnableBanking, profile.ConnectorID)
		assert.True(t, contract.Capabilities().SupportsStartLink)
		assert.True(t, contract.Capabilities().SupportsFetch)
		assert.Equal(t, profile, connector.lastStart.Profile)
		assert.Equal(t, startResult.State, connector.lastFinish.State)
		assert.Equal(t, finishResult.ProviderReference, connector.finishResult.ProviderReference)
		assert.Equal(t, connector.finishResult.Secret, finishResult.Secret)
		assert.Equal(t, connector.tokenResult.Secret, tokenResult.Secret)
		assert.Equal(t, batch.Connection, connector.fetchResult.Connection)
		assert.Equal(t, connection, connector.lastFetch.Connection)
		assert.Equal(t, secret, connector.lastFetch.Secret)
	})

	t.Run("fetch requests and snapshots carry sync context", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake,
			domain.ProviderIDMonobank,
			domain.ProviderConnectorIDMonobank,
		)
		requestedWindow := domain.ProviderSyncWindow{
			Start: time.Date(2026, time.June, 21, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, time.June, 24, 0, 0, 0, 0, time.UTC),
		}
		snapshotWindow := domain.ProviderSyncWindow{
			Start: requestedWindow.Start.Add(-72 * time.Hour),
			End:   requestedWindow.End.Add(72 * time.Hour),
		}
		state := &domain.ProviderSyncState{
			Connection: connection,
			RunID:      "run-" + fake.UUID().V4(),
			JobID:      "job-" + fake.UUID().V4(),
			Window:     requestedWindow,
		}
		secret := domain.ConnectionSecret{
			ID:        "secret-" + fake.UUID().V4(),
			Provider:  string(connection.ProviderID),
			Reference: "reference-" + fake.UUID().V4(),
		}
		account := domain.ConnectionProviderAccount{
			ID:                "provider-account-row-" + fake.UUID().V4(),
			ConnectionID:      connection.ConnectionID,
			ProviderAccountID: "provider-account-" + fake.UUID().V4(),
			FinanceAccountID:  "finance-account-" + fake.UUID().V4(),
			Name:              "account-" + fake.Lorem().Word(),
			Currency:          "PLN",
		}
		transaction := domain.Transaction{
			ID:          "transaction-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			AccountID:   account.FinanceAccountID,
			Source:      domain.TransactionSourceProvider,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -50_00,
			Currency:    "PLN",
			Description: "transaction-" + fake.Lorem().Word(),
			EffectiveAt: time.Date(2026, time.June, 23, 10, 0, 0, 0, time.UTC),
		}
		match := domain.ProviderTransactionMatch{
			ID:                    "match-" + fake.UUID().V4(),
			ConnectionID:          connection.ConnectionID,
			ProviderAccountID:     account.ProviderAccountID,
			ProviderTransactionID: "provider-transaction-" + fake.UUID().V4(),
			Fingerprint:           "fingerprint-" + fake.UUID().V4(),
			TransactionID:         transaction.ID,
			Status:                domain.TransactionStatusBooked,
		}

		request := FetchRequest{
			Connection:      connection,
			Secret:          secret,
			RequestedWindow: requestedWindow,
			SyncState:       state,
		}
		snapshot := ExistingWindowSnapshot{
			Connection:     connection,
			SnapshotWindow: snapshotWindow,
			Accounts:       []domain.ConnectionProviderAccount{account},
			Transactions:   []domain.Transaction{transaction},
			Matches:        []domain.ProviderTransactionMatch{match},
		}

		assert.Equal(t, connection, request.Connection)
		assert.Equal(t, secret, request.Secret)
		assert.Equal(t, requestedWindow, request.RequestedWindow)
		require.NotNil(t, request.SyncState)
		assert.Equal(t, state.RunID, request.SyncState.RunID)
		assert.Equal(t, connection, snapshot.Connection)
		assert.Equal(t, snapshotWindow, snapshot.SnapshotWindow)
		require.Len(t, snapshot.Accounts, 1)
		require.Len(t, snapshot.Transactions, 1)
		require.Len(t, snapshot.Matches, 1)
	})

	t.Run("diff planner updates strong provider-id matches", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake,
			domain.ProviderIDMonobank,
			domain.ProviderConnectorIDMonobank,
		)
		providerAccountID := "provider-account-" + fake.UUID().V4()
		transaction := domain.Transaction{
			ID:          "transaction-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			AccountID:   "finance-account-" + fake.UUID().V4(),
			Source:      domain.TransactionSourceProvider,
			Status:      domain.TransactionStatusPending,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -75_00,
			Currency:    "PLN",
			Description: "transaction-" + fake.Lorem().Word(),
			EffectiveAt: time.Date(2026, time.June, 23, 10, 0, 0, 0, time.UTC),
		}
		observation := domain.ProviderTransactionObservation{
			Connection:            connection,
			ProviderAccountID:     providerAccountID,
			ProviderTransactionID: "provider-transaction-" + fake.UUID().V4(),
			Status:                domain.TransactionStatusBooked,
			AmountMinor:           -75_00,
			Currency:              "PLN",
			Description:           "provider-transaction-" + fake.Lorem().Word(),
			EffectiveAt:           time.Date(2026, time.June, 24, 10, 0, 0, 0, time.UTC),
			Fingerprint:           "fingerprint-" + fake.UUID().V4(),
		}
		plan := NewDiffPlanner().Plan(
			domain.ProviderSyncBatch{
				Connection:      connection,
				RequestedWindow: domain.ProviderSyncWindow{},
				Transactions:    []domain.ProviderTransactionObservation{observation},
			},
			ExistingWindowSnapshot{
				Connection: connection,
				Accounts: []domain.ConnectionProviderAccount{{
					ConnectionID:      connection.ConnectionID,
					ProviderAccountID: providerAccountID,
					FinanceAccountID:  transaction.AccountID,
				}},
				Transactions: []domain.Transaction{transaction},
				Matches: []domain.ProviderTransactionMatch{{
					ConnectionID:          connection.ConnectionID,
					ProviderAccountID:     providerAccountID,
					ProviderTransactionID: observation.ProviderTransactionID,
					Fingerprint:           observation.Fingerprint,
					TransactionID:         transaction.ID,
				}},
			},
		)

		require.Len(t, plan.TransactionActions, 1)
		assert.Equal(t, ProviderTransactionActionTypeUpdate, plan.TransactionActions[0].Type)
		assert.Equal(t, ProviderTransactionMatchStrategyProviderID, plan.TransactionActions[0].MatchStrategy)
		require.NotNil(t, plan.TransactionActions[0].ExistingTransaction)
		assert.Equal(t, transaction.ID, plan.TransactionActions[0].ExistingTransaction.ID)
		assert.Equal(t, 1, plan.Stats.UpdatedTransactions)
	})

	t.Run("diff planner updates unique fingerprint matches", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake,
			domain.ProviderIDMonobank,
			domain.ProviderConnectorIDMonobank,
		)
		providerAccountID := "provider-account-" + fake.UUID().V4()
		transaction := domain.Transaction{
			ID:          "transaction-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			AccountID:   "finance-account-" + fake.UUID().V4(),
			Source:      domain.TransactionSourceProvider,
			Status:      domain.TransactionStatusPending,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -80_00,
			Currency:    "PLN",
			Description: "transaction-" + fake.Lorem().Word(),
			EffectiveAt: time.Date(2026, time.June, 23, 11, 0, 0, 0, time.UTC),
		}
		observation := domain.ProviderTransactionObservation{
			Connection:        connection,
			ProviderAccountID: providerAccountID,
			Status:            domain.TransactionStatusBooked,
			AmountMinor:       -80_00,
			Currency:          "PLN",
			Description:       "provider-transaction-" + fake.Lorem().Word(),
			EffectiveAt:       time.Date(2026, time.June, 24, 11, 0, 0, 0, time.UTC),
			Fingerprint:       "fingerprint-" + fake.UUID().V4(),
		}
		plan := NewDiffPlanner().Plan(
			domain.ProviderSyncBatch{
				Connection:   connection,
				Transactions: []domain.ProviderTransactionObservation{observation},
			},
			ExistingWindowSnapshot{
				Connection: connection,
				Accounts: []domain.ConnectionProviderAccount{{
					ConnectionID:      connection.ConnectionID,
					ProviderAccountID: providerAccountID,
					FinanceAccountID:  transaction.AccountID,
				}},
				Transactions: []domain.Transaction{transaction},
				Matches: []domain.ProviderTransactionMatch{{
					ConnectionID:      connection.ConnectionID,
					ProviderAccountID: providerAccountID,
					Fingerprint:       observation.Fingerprint,
					TransactionID:     transaction.ID,
				}},
			},
		)

		require.Len(t, plan.TransactionActions, 1)
		assert.Equal(t, ProviderTransactionActionTypeUpdate, plan.TransactionActions[0].Type)
		assert.Equal(t, ProviderTransactionMatchStrategyFingerprint, plan.TransactionActions[0].MatchStrategy)
		assert.Equal(t, 1, plan.Stats.UpdatedTransactions)
	})

	t.Run("diff planner creates new transactions for weak matches", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake,
			domain.ProviderIDMonobank,
			domain.ProviderConnectorIDMonobank,
		)
		providerAccountID := "provider-account-" + fake.UUID().V4()
		accountID := "finance-account-" + fake.UUID().V4()
		observation := domain.ProviderTransactionObservation{
			Connection:        connection,
			ProviderAccountID: providerAccountID,
			Status:            domain.TransactionStatusBooked,
			AmountMinor:       -90_00,
			Currency:          "PLN",
			Description:       "provider-transaction-" + fake.Lorem().Word(),
			EffectiveAt:       time.Date(2026, time.June, 24, 12, 0, 0, 0, time.UTC),
		}
		plan := NewDiffPlanner().Plan(
			domain.ProviderSyncBatch{
				Connection:   connection,
				Transactions: []domain.ProviderTransactionObservation{observation},
			},
			ExistingWindowSnapshot{
				Connection: connection,
				Accounts: []domain.ConnectionProviderAccount{{
					ConnectionID:      connection.ConnectionID,
					ProviderAccountID: providerAccountID,
					FinanceAccountID:  accountID,
				}},
				Transactions: []domain.Transaction{{
					ID:          "transaction-" + fake.UUID().V4(),
					TenantID:    "tenant-" + fake.UUID().V4(),
					AccountID:   accountID,
					Source:      domain.TransactionSourceProvider,
					Status:      domain.TransactionStatusPending,
					Kind:        domain.TransactionKindRegular,
					AmountMinor: observation.AmountMinor,
					Currency:    observation.Currency,
					Description: "previous-transaction-" + fake.Lorem().Word(),
					EffectiveAt: observation.EffectiveAt,
				}},
			},
		)

		require.Len(t, plan.TransactionActions, 1)
		assert.Equal(t, ProviderTransactionActionTypeCreate, plan.TransactionActions[0].Type)
		assert.Equal(t, ProviderTransactionMatchStrategyWeakCandidate, plan.TransactionActions[0].MatchStrategy)
		assert.Equal(t, 1, plan.Stats.CreatedTransactions)
		assert.Equal(t, 1, plan.Stats.AmbiguousCreatedTransactions)
	})

	t.Run("diff planner creates new transactions for ambiguous matches", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake,
			domain.ProviderIDPKO,
			domain.ProviderConnectorIDEnableBanking,
		)
		providerAccountID := "provider-account-" + fake.UUID().V4()
		observation := domain.ProviderTransactionObservation{
			Connection:        connection,
			ProviderAccountID: providerAccountID,
			Status:            domain.TransactionStatusBooked,
			AmountMinor:       -95_00,
			Currency:          "PLN",
			Description:       "provider-transaction-" + fake.Lorem().Word(),
			EffectiveAt:       time.Date(2026, time.June, 24, 13, 0, 0, 0, time.UTC),
			Fingerprint:       "fingerprint-" + fake.UUID().V4(),
		}
		plan := NewDiffPlanner().Plan(
			domain.ProviderSyncBatch{
				Connection:   connection,
				Transactions: []domain.ProviderTransactionObservation{observation},
			},
			ExistingWindowSnapshot{
				Connection: connection,
				Accounts: []domain.ConnectionProviderAccount{{
					ConnectionID:      connection.ConnectionID,
					ProviderAccountID: providerAccountID,
					FinanceAccountID:  "finance-account-" + fake.UUID().V4(),
				}},
				Transactions: []domain.Transaction{{
					ID:        "transaction-a-" + fake.UUID().V4(),
					TenantID:  "tenant-" + fake.UUID().V4(),
					AccountID: "finance-account-a-" + fake.UUID().V4(),
				}, {
					ID:        "transaction-b-" + fake.UUID().V4(),
					TenantID:  "tenant-" + fake.UUID().V4(),
					AccountID: "finance-account-b-" + fake.UUID().V4(),
				}},
				Matches: []domain.ProviderTransactionMatch{{
					ConnectionID:      connection.ConnectionID,
					ProviderAccountID: providerAccountID,
					Fingerprint:       observation.Fingerprint,
					TransactionID:     "transaction-a",
				}, {
					ConnectionID:      connection.ConnectionID,
					ProviderAccountID: providerAccountID,
					Fingerprint:       observation.Fingerprint,
					TransactionID:     "transaction-b",
				}},
			},
		)

		require.Len(t, plan.TransactionActions, 1)
		assert.Equal(t, ProviderTransactionActionTypeCreate, plan.TransactionActions[0].Type)
		assert.Equal(t, ProviderTransactionMatchStrategyAmbiguous, plan.TransactionActions[0].MatchStrategy)
		assert.Equal(t, 1, plan.Stats.CreatedTransactions)
		assert.Equal(t, 1, plan.Stats.AmbiguousCreatedTransactions)
		require.Len(t, plan.Issues, 1)
		assert.Equal(t, domain.ProviderSyncIssueSeverityWarning, plan.Issues[0].Severity)
	})

	t.Run("diff planner creates new transactions when no candidates exist", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake,
			domain.ProviderIDMonobank,
			domain.ProviderConnectorIDMonobank,
		)
		observation := domain.ProviderTransactionObservation{
			Connection:        connection,
			ProviderAccountID: "provider-account-" + fake.UUID().V4(),
			Status:            domain.TransactionStatusBooked,
			AmountMinor:       -45_00,
			Currency:          "PLN",
			Description:       "provider-transaction-" + fake.Lorem().Word(),
			EffectiveAt:       time.Date(2026, time.June, 24, 14, 0, 0, 0, time.UTC),
		}

		plan := NewDiffPlanner().Plan(
			domain.ProviderSyncBatch{
				Connection:   connection,
				Transactions: []domain.ProviderTransactionObservation{observation},
			},
			ExistingWindowSnapshot{Connection: connection},
		)

		require.Len(t, plan.TransactionActions, 1)
		assert.Equal(t, ProviderTransactionActionTypeCreate, plan.TransactionActions[0].Type)
		assert.Equal(t, ProviderTransactionMatchStrategyNew, plan.TransactionActions[0].MatchStrategy)
		assert.Empty(t, plan.Issues)
	})

	t.Run("diff planner treats multiple provider-id matches as ambiguous", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake,
			domain.ProviderIDPKO,
			domain.ProviderConnectorIDEnableBanking,
		)
		providerAccountID := "provider-account-" + fake.UUID().V4()
		providerTransactionID := "provider-transaction-" + fake.UUID().V4()
		plan := NewDiffPlanner().Plan(
			domain.ProviderSyncBatch{
				Connection: connection,
				Transactions: []domain.ProviderTransactionObservation{{
					Connection:            connection,
					ProviderAccountID:     providerAccountID,
					ProviderTransactionID: providerTransactionID,
					Status:                domain.TransactionStatusBooked,
					AmountMinor:           -35_00,
					Currency:              "PLN",
					Description:           "provider-transaction-" + fake.Lorem().Word(),
					EffectiveAt:           time.Date(2026, time.June, 24, 15, 0, 0, 0, time.UTC),
				}},
			},
			ExistingWindowSnapshot{
				Connection: connection,
				Matches: []domain.ProviderTransactionMatch{{
					ConnectionID:          connection.ConnectionID,
					ProviderAccountID:     providerAccountID,
					ProviderTransactionID: providerTransactionID,
					TransactionID:         "transaction-a",
				}, {
					ConnectionID:          connection.ConnectionID,
					ProviderAccountID:     providerAccountID,
					ProviderTransactionID: providerTransactionID,
					TransactionID:         "transaction-b",
				}},
			},
		)

		require.Len(t, plan.TransactionActions, 1)
		assert.Equal(t, ProviderTransactionMatchStrategyAmbiguous, plan.TransactionActions[0].MatchStrategy)
		require.Len(t, plan.Issues, 1)
		assert.Equal(t, "ambiguous-transaction-match", plan.Issues[0].Code)
	})

	t.Run("diff planner helpers ignore unrelated snapshot entries", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake,
			domain.ProviderIDMonobank,
			domain.ProviderConnectorIDMonobank,
		)
		otherConnection := makeRandomProviderConnectionRef(
			fake,
			domain.ProviderIDMonobank,
			domain.ProviderConnectorIDMonobank,
		)
		observation := domain.ProviderTransactionObservation{
			Connection:            connection,
			ProviderAccountID:     "provider-account-" + fake.UUID().V4(),
			ProviderTransactionID: "provider-transaction-" + fake.UUID().V4(),
			Fingerprint:           "fingerprint-" + fake.UUID().V4(),
			AmountMinor:           -25_00,
			Currency:              "PLN",
			EffectiveAt:           time.Date(2026, time.June, 24, 16, 0, 0, 0, time.UTC),
		}
		matches := []domain.ProviderTransactionMatch{{
			ConnectionID:          otherConnection.ConnectionID,
			ProviderAccountID:     observation.ProviderAccountID,
			ProviderTransactionID: observation.ProviderTransactionID,
			Fingerprint:           observation.Fingerprint,
		}, {
			ConnectionID:          connection.ConnectionID,
			ProviderAccountID:     "other-provider-account",
			ProviderTransactionID: observation.ProviderTransactionID,
			Fingerprint:           observation.Fingerprint,
		}}

		assert.Empty(t, matchingProviderIDs(matches, observation))
		assert.Empty(t, matchingFingerprints(matches, observation))
		assert.False(t, hasWeakCandidate(ExistingWindowSnapshot{}, observation))
	})

	t.Run("apply planner refreshes provider-original fields and preserves user edits", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake,
			domain.ProviderIDPKO,
			domain.ProviderConnectorIDEnableBanking,
		)
		oldOriginalEffectiveAt := time.Date(2026, time.June, 22, 12, 0, 0, 0, time.UTC)
		newEffectiveAt := time.Date(2026, time.June, 24, 12, 0, 0, 0, time.UTC)
		existing := domain.Transaction{
			ID:          "transaction-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			AccountID:   "finance-account-" + fake.UUID().V4(),
			Source:      domain.TransactionSourceProvider,
			Status:      domain.TransactionStatusPending,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -120_00,
			Currency:    "PLN",
			Description: "user-edited-" + fake.Lorem().Word(),
			EffectiveAt: oldOriginalEffectiveAt,
			ProviderOriginal: &domain.ProviderTransactionOriginal{
				AmountMinor: -120_00,
				Currency:    "PLN",
				Description: "provider-original-" + fake.Lorem().Word(),
				EffectiveAt: &oldOriginalEffectiveAt,
			},
		}
		observation := domain.ProviderTransactionObservation{
			Connection:        connection,
			ProviderAccountID: "provider-account-" + fake.UUID().V4(),
			Status:            domain.TransactionStatusBooked,
			AmountMinor:       -140_00,
			Currency:          "PLN",
			Description:       "provider-updated-" + fake.Lorem().Word(),
			EffectiveAt:       newEffectiveAt,
			ProviderOriginal: &domain.ProviderTransactionOriginal{
				AmountMinor: -140_00,
				Currency:    "PLN",
				Description: "provider-updated-" + fake.Lorem().Word(),
				EffectiveAt: &newEffectiveAt,
			},
		}

		plan := NewApplyPlanner(NewMergePolicy()).Plan(ProviderDiffPlan{
			Connection: connection,
			TransactionActions: []ProviderTransactionAction{{
				Type:                ProviderTransactionActionTypeUpdate,
				MatchStrategy:       ProviderTransactionMatchStrategyProviderID,
				Observation:         observation,
				ExistingTransaction: &existing,
			}},
		})

		require.Len(t, plan.TransactionWrites, 1)
		require.NotNil(t, plan.TransactionWrites[0].MergedTransaction)
		merged := plan.TransactionWrites[0].MergedTransaction
		assert.Equal(t, existing.Description, merged.Description)
		assert.Equal(t, observation.AmountMinor, merged.AmountMinor)
		assert.Equal(t, newEffectiveAt, merged.EffectiveAt)
		assert.Equal(t, observation.Status, merged.Status)
		require.NotNil(t, merged.ProviderOriginal)
		assert.Equal(t, observation.ProviderOriginal.Description, merged.ProviderOriginal.Description)
	})

	t.Run("apply planner covers create writes and fallback originals", func(t *testing.T) {
		fake := faker.New()
		planner := NewApplyPlanner(nil)
		connection := makeRandomProviderConnectionRef(
			fake,
			domain.ProviderIDMonobank,
			domain.ProviderConnectorIDMonobank,
		)
		createObservation := domain.ProviderTransactionObservation{
			Connection:        connection,
			ProviderAccountID: "provider-account-" + fake.UUID().V4(),
			Status:            domain.TransactionStatusBooked,
			AmountMinor:       -15_00,
			Currency:          "EUR",
			Description:       "provider-transaction-" + fake.Lorem().Word(),
			EffectiveAt:       time.Date(2026, time.June, 24, 17, 0, 0, 0, time.UTC),
		}
		updateObservation := domain.ProviderTransactionObservation{
			Connection:        connection,
			ProviderAccountID: "provider-account-" + fake.UUID().V4(),
			Status:            domain.TransactionStatusBooked,
			AmountMinor:       -20_00,
			Currency:          "EUR",
			Description:       "provider-transaction-" + fake.Lorem().Word(),
			EffectiveAt:       time.Date(2026, time.June, 24, 18, 0, 0, 0, time.UTC),
		}
		existing := domain.Transaction{
			ID:          "transaction-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			AccountID:   "finance-account-" + fake.UUID().V4(),
			Source:      domain.TransactionSourceProvider,
			Status:      domain.TransactionStatusPending,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -10_00,
			Currency:    "USD",
			Description: "user-edited-" + fake.Lorem().Word(),
			EffectiveAt: time.Date(2026, time.June, 24, 18, 30, 0, 0, time.UTC),
		}

		plan := planner.Plan(ProviderDiffPlan{
			TransactionActions: []ProviderTransactionAction{{
				Type:          ProviderTransactionActionTypeCreate,
				MatchStrategy: ProviderTransactionMatchStrategyNew,
				Observation:   createObservation,
			}, {
				Type:                ProviderTransactionActionTypeUpdate,
				MatchStrategy:       ProviderTransactionMatchStrategyFingerprint,
				Observation:         updateObservation,
				ExistingTransaction: &existing,
			}},
		})

		require.Len(t, plan.TransactionWrites, 2)
		assert.Nil(t, plan.TransactionWrites[0].MergedTransaction)
		require.NotNil(t, plan.TransactionWrites[1].MergedTransaction)
		merged := plan.TransactionWrites[1].MergedTransaction
		assert.Equal(t, existing.AmountMinor, merged.AmountMinor)
		assert.Equal(t, existing.Description, merged.Description)
		require.NotNil(t, merged.ProviderOriginal)
		assert.Equal(t, updateObservation.Description, merged.ProviderOriginal.Description)
		assert.Equal(t, updateObservation.EffectiveAt, *merged.ProviderOriginal.EffectiveAt)
	})

	t.Run("merge policy updates untouched description fields", func(t *testing.T) {
		fake := faker.New()
		policy := NewMergePolicy()
		oldEffectiveAt := time.Date(2026, time.June, 23, 9, 0, 0, 0, time.UTC)
		newEffectiveAt := time.Date(2026, time.June, 24, 9, 0, 0, 0, time.UTC)
		oldDescription := "provider-original-" + fake.Lorem().Word()
		merged := policy.MergeTransaction(
			domain.Transaction{
				ID:          "transaction-" + fake.UUID().V4(),
				TenantID:    "tenant-" + fake.UUID().V4(),
				AccountID:   "finance-account-" + fake.UUID().V4(),
				Source:      domain.TransactionSourceProvider,
				Status:      domain.TransactionStatusPending,
				Kind:        domain.TransactionKindRegular,
				AmountMinor: -30_00,
				Currency:    "USD",
				Description: oldDescription,
				EffectiveAt: oldEffectiveAt,
				ProviderOriginal: &domain.ProviderTransactionOriginal{
					AmountMinor: -30_00,
					Currency:    "USD",
					Description: oldDescription,
					EffectiveAt: &oldEffectiveAt,
				},
			},
			domain.ProviderTransactionObservation{
				Status:      domain.TransactionStatusBooked,
				AmountMinor: -32_00,
				Currency:    "EUR",
				Description: "provider-updated-" + fake.Lorem().Word(),
				EffectiveAt: newEffectiveAt,
			},
		)

		assert.Equal(t, "EUR", merged.Currency)
		assert.NotEqual(t, oldDescription, merged.Description)
		assert.Equal(t, newEffectiveAt, merged.EffectiveAt)
	})

	t.Run("window sync executor request and result expose foundation fields", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake,
			domain.ProviderIDPKO,
			domain.ProviderConnectorIDEnableBanking,
		)
		window := domain.ProviderSyncWindow{
			Start: time.Date(2026, time.June, 22, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, time.June, 24, 0, 0, 0, 0, time.UTC),
		}
		secret := domain.ConnectionSecret{
			ID:        "secret-" + fake.UUID().V4(),
			Provider:  string(connection.ProviderID),
			Reference: "reference-" + fake.UUID().V4(),
		}
		request := WindowSyncRequest{
			Connection:      connection,
			Secret:          secret,
			RequestedWindow: window,
			JobID:           "job-" + fake.UUID().V4(),
			Reason:          "manual",
		}
		executor, err := NewWindowSyncExecutor(
			WithConnectors(&stubConnector{connectorID: domain.ProviderConnectorIDEnableBanking}),
			WithWindowSyncStore(&stubWindowSyncStore{}),
			WithRunIDGenerator(func() string {
				return "run-" + fake.UUID().V4()
			}),
		)
		require.NoError(t, err)

		result, err := executor.Execute(t.Context(), request)
		require.NoError(t, err)
		assert.Equal(t, connection, request.Connection)
		assert.Equal(t, secret, request.Secret)
		assert.Equal(t, window, request.RequestedWindow)
		assert.NotEmpty(t, request.JobID)
		assert.Equal(t, "manual", request.Reason)
		assert.NotEmpty(t, result.RunID)
		assert.Equal(t, connection, result.Batch.Connection)
		assert.Equal(t, window, result.Batch.RequestedWindow)
		assert.Equal(t, domain.ProviderSyncStats{}, result.Stats)
		assert.Nil(t, result.Issues)
	})
}
