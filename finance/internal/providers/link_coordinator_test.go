package providers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type linkConnectorFixture struct {
	mock         *MockConnector
	connectorID  domain.ProviderConnectorID
	capabilities ConnectorCapabilities
	startResult  StartLinkResult
	startErr     error
	finishResult LinkResult
	finishErr    error
	tokenResult  LinkResult
	tokenErr     error
	startCalls   int
	finishCalls  int
	tokenCalls   int
	lastStart    StartLinkRequest
	lastFinish   FinishLinkRequest
	lastToken    LinkTokenRequest
}

func newLinkConnectorFixture(t *testing.T, fixture linkConnectorFixture) *linkConnectorFixture {
	t.Helper()

	fixture.mock = NewMockConnector(t)
	connectorIDCall := fixture.mock.EXPECT().ConnectorID()
	connectorIDCall.RunAndReturn(func() domain.ProviderConnectorID {
		return fixture.connectorID
	})
	connectorIDCall.Maybe()

	capabilitiesCall := fixture.mock.EXPECT().Capabilities()
	capabilitiesCall.RunAndReturn(func() ConnectorCapabilities {
		return fixture.capabilities
	})
	capabilitiesCall.Maybe()

	startLinkCall := fixture.mock.EXPECT().StartLink(testifymock.Anything, testifymock.Anything)
	startLinkCall.RunAndReturn(
		func(_ context.Context, request StartLinkRequest) (StartLinkResult, error) {
			fixture.startCalls++
			fixture.lastStart = request
			if fixture.startErr != nil {
				return StartLinkResult{}, fixture.startErr
			}
			return fixture.startResult, nil
		},
	)
	startLinkCall.Maybe()

	finishLinkCall := fixture.mock.EXPECT().FinishLink(testifymock.Anything, testifymock.Anything)
	finishLinkCall.RunAndReturn(
		func(_ context.Context, request FinishLinkRequest) (LinkResult, error) {
			fixture.finishCalls++
			fixture.lastFinish = request
			if fixture.finishErr != nil {
				return LinkResult{}, fixture.finishErr
			}
			return fixture.finishResult, nil
		},
	)
	finishLinkCall.Maybe()

	linkTokenCall := fixture.mock.EXPECT().LinkToken(testifymock.Anything, testifymock.Anything)
	linkTokenCall.RunAndReturn(
		func(_ context.Context, request LinkTokenRequest) (LinkResult, error) {
			fixture.tokenCalls++
			fixture.lastToken = request
			if fixture.tokenErr != nil {
				return LinkResult{}, fixture.tokenErr
			}
			return fixture.tokenResult, nil
		},
	)
	linkTokenCall.Maybe()

	fetchCall := fixture.mock.EXPECT().Fetch(testifymock.Anything, testifymock.Anything)
	fetchCall.Return(
		domain.ProviderSyncBatch{},
		errors.New("fetch not used in link coordinator tests"),
	)
	fetchCall.Maybe()

	return &fixture
}

type pendingStartStoreFixture struct {
	mock                   *MockPendingStartStore
	saved                  []domain.PendingBankConnectionLinkStart
	consumed               []ConsumePendingStartRequest
	restored               []RestorePendingStartRequest
	consumeResult          *domain.PendingBankConnectionLinkStart
	consumeErr             error
	restoreErr             error
	saveErr                error
	consumeCallsByState    map[string]int
	restoreCallsByState    map[string]int
	savedByState           map[string]domain.PendingBankConnectionLinkStart
	consumedCurrentByState map[string]domain.PendingBankConnectionLinkStart
}

func newPendingStartStoreFixture(t *testing.T, fixture pendingStartStoreFixture) *pendingStartStoreFixture {
	t.Helper()

	fixture.mock = NewMockPendingStartStore(t)
	savePendingStartCall := fixture.mock.EXPECT().SavePendingStart(testifymock.Anything, testifymock.Anything)
	savePendingStartCall.RunAndReturn(
		func(_ context.Context, start domain.PendingBankConnectionLinkStart) (domain.PendingBankConnectionLinkStart, error) {
			if fixture.saveErr != nil {
				return domain.PendingBankConnectionLinkStart{}, fixture.saveErr
			}
			fixture.saved = append(fixture.saved, start)
			if fixture.savedByState == nil {
				fixture.savedByState = map[string]domain.PendingBankConnectionLinkStart{}
			}
			fixture.savedByState[start.State] = start
			return start, nil
		},
	)
	savePendingStartCall.Maybe()

	consumePendingStartCall := fixture.mock.EXPECT().ConsumePendingStart(
		testifymock.Anything,
		testifymock.Anything,
	)
	consumePendingStartCall.RunAndReturn(
		func(_ context.Context, request ConsumePendingStartRequest) (*domain.PendingBankConnectionLinkStart, error) {
			if fixture.consumeCallsByState == nil {
				fixture.consumeCallsByState = map[string]int{}
			}
			fixture.consumeCallsByState[request.State]++
			fixture.consumed = append(fixture.consumed, request)
			if fixture.consumeErr != nil {
				return nil, fixture.consumeErr
			}
			if fixture.savedByState != nil {
				start, ok := fixture.savedByState[request.State]
				if !ok {
					return nil, ErrPendingStartNotFound
				}
				if start.TenantID != request.TenantID ||
					start.ActorUserID != request.ActorUserID ||
					start.Provider != string(request.ProviderID) ||
					start.ConnectorID != request.ConnectorID {
					return nil, ErrPendingStartNotFound
				}
				delete(fixture.savedByState, request.State)
				start.ConsumedAt = &request.ConsumedAt
				if fixture.consumedCurrentByState == nil {
					fixture.consumedCurrentByState = map[string]domain.PendingBankConnectionLinkStart{}
				}
				fixture.consumedCurrentByState[request.State] = start
				copyStart := start
				return &copyStart, nil
			}
			if fixture.consumeResult == nil {
				return nil, ErrPendingStartNotFound
			}
			copyStart := *fixture.consumeResult
			copyStart.ConsumedAt = &request.ConsumedAt
			return &copyStart, nil
		},
	)
	consumePendingStartCall.Maybe()

	restorePendingStartCall := fixture.mock.EXPECT().RestorePendingStart(
		testifymock.Anything,
		testifymock.Anything,
	)
	restorePendingStartCall.RunAndReturn(
		func(_ context.Context, request RestorePendingStartRequest) error {
			if fixture.restoreCallsByState == nil {
				fixture.restoreCallsByState = map[string]int{}
			}
			fixture.restoreCallsByState[request.State]++
			fixture.restored = append(fixture.restored, request)
			if fixture.restoreErr != nil {
				return fixture.restoreErr
			}
			if fixture.consumedCurrentByState != nil {
				start, ok := fixture.consumedCurrentByState[request.State]
				if !ok {
					return ErrPendingStartNotFound
				}
				start.ConsumedAt = nil
				if fixture.savedByState == nil {
					fixture.savedByState = map[string]domain.PendingBankConnectionLinkStart{}
				}
				fixture.savedByState[request.State] = start
				delete(fixture.consumedCurrentByState, request.State)
			}
			return nil
		},
	)
	restorePendingStartCall.Maybe()

	return &fixture
}

type connectionSecretWriterFixture struct {
	mock      *MockConnectionSecretWriter
	saved     []savedSecret
	secretID  string
	secretIDs []string
	saveErr   error
}

type savedSecret struct {
	provider  string
	reference string
	secret    string
}

func newConnectionSecretWriterFixture(
	t *testing.T,
	fixture connectionSecretWriterFixture,
) *connectionSecretWriterFixture {
	t.Helper()

	fixture.mock = NewMockConnectionSecretWriter(t)
	fixture.mock.EXPECT().
		PrepareConnectionSecret(testifymock.Anything, testifymock.Anything, testifymock.Anything).
		RunAndReturn(func(provider string, reference string, secret string) (domain.ConnectionSecret, error) {
			if fixture.saveErr != nil {
				return domain.ConnectionSecret{}, fixture.saveErr
			}
			secretID := fixture.secretID
			if len(fixture.secretIDs) > len(fixture.saved) {
				secretID = fixture.secretIDs[len(fixture.saved)]
			}
			fixture.saved = append(
				fixture.saved,
				savedSecret{provider: provider, reference: reference, secret: secret},
			)
			return domain.ConnectionSecret{ID: secretID, Provider: provider, Reference: reference}, nil
		}).
		Maybe()

	return &fixture
}

type connectionStoreFixture struct {
	mock       *MockConnectionStore
	saved      []domain.BankConnection
	snapshots  []*domain.ProviderSnapshot
	listResult []domain.BankConnection
	saveErr    error
}

func newConnectionStoreFixture(t *testing.T, fixture connectionStoreFixture) *connectionStoreFixture {
	t.Helper()

	fixture.mock = NewMockConnectionStore(t)
	saveBankConnectionCall := fixture.mock.EXPECT().SaveLinkedConnectionWithSnapshot(
		testifymock.Anything,
		testifymock.Anything,
		testifymock.Anything,
		testifymock.Anything,
	)
	saveBankConnectionCall.RunAndReturn(
		func(
			_ context.Context,
			connection domain.BankConnection,
			_ domain.ConnectionSecret,
			snapshot *domain.ProviderSnapshot,
		) (domain.BankConnection, error) {
			if fixture.saveErr != nil {
				return domain.BankConnection{}, fixture.saveErr
			}
			for _, existing := range fixture.listResult {
				if hasSameProviderSessionIdentity(existing, connection) {
					return existing, nil
				}
			}
			fixture.saved = append(fixture.saved, connection)
			fixture.snapshots = append(fixture.snapshots, snapshot)
			return connection, nil
		},
	)
	saveBankConnectionCall.Maybe()

	return &fixture
}

func hasSameProviderSessionIdentity(existing domain.BankConnection, candidate domain.BankConnection) bool {
	if existing.Provider != candidate.Provider || existing.ConnectorID != candidate.ConnectorID {
		return false
	}
	return candidate.ProviderReference != "" &&
		candidate.ProviderReference == existing.ProviderReference
}

func TestLinkCoordinator(t *testing.T) {
	t.Run("validates construction requirements and applies defaults", func(t *testing.T) {
		t.Run("requires all collaborators", func(t *testing.T) {
			_, err := NewLinkCoordinator(LinkCoordinatorArgs{})
			require.ErrorIs(t, err, ErrProviderProfileRegistryRequired)
			var coordinator *LinkCoordinator

			_, err = NewLinkCoordinator(LinkCoordinatorArgs{
				ProviderProfileRegistry: NewStaticProviderProfileRegistry(PKOProfile()),
			})
			require.ErrorIs(t, err, ErrConnectorRegistryRequired)

			_, err = NewLinkCoordinator(LinkCoordinatorArgs{
				ProviderProfileRegistry: NewStaticProviderProfileRegistry(PKOProfile()),
				ConnectorRegistry:       NewStaticConnectorRegistry(),
			})
			require.ErrorIs(t, err, ErrPendingStartStoreRequired)

			_, err = NewLinkCoordinator(LinkCoordinatorArgs{
				ProviderProfileRegistry: NewStaticProviderProfileRegistry(PKOProfile()),
				ConnectorRegistry:       NewStaticConnectorRegistry(),
				PendingStartStore:       newPendingStartStoreFixture(t, pendingStartStoreFixture{}).mock,
			})
			require.ErrorIs(t, err, ErrConnectionSecretWriterRequired)

			_, err = NewLinkCoordinator(LinkCoordinatorArgs{
				ProviderProfileRegistry: NewStaticProviderProfileRegistry(PKOProfile()),
				ConnectorRegistry:       NewStaticConnectorRegistry(),
				PendingStartStore:       newPendingStartStoreFixture(t, pendingStartStoreFixture{}).mock,
				ConnectionSecretWriter:  newConnectionSecretWriterFixture(t, connectionSecretWriterFixture{}).mock,
			})
			require.ErrorIs(t, err, ErrConnectionStoreRequired)

			coordinator, err = NewLinkCoordinator(LinkCoordinatorArgs{
				ProviderProfileRegistry: NewStaticProviderProfileRegistry(PKOProfile()),
				ConnectorRegistry:       NewStaticConnectorRegistry(),
				PendingStartStore:       newPendingStartStoreFixture(t, pendingStartStoreFixture{}).mock,
				ConnectionSecretWriter:  newConnectionSecretWriterFixture(t, connectionSecretWriterFixture{}).mock,
				ConnectionStore:         newConnectionStoreFixture(t, connectionStoreFixture{}).mock,
			})
			require.NoError(t, err)
			require.NotNil(t, coordinator)
		})
		t.Run("applies defaults for identity and ttl", func(t *testing.T) {
			fake := faker.New()
			connector := newLinkConnectorFixture(t, linkConnectorFixture{
				connectorID:  domain.ProviderConnectorIDEnableBanking,
				capabilities: ConnectorCapabilities{SupportsStartLink: true},
				startResult: StartLinkResult{
					State:            "state-" + fake.UUID().V4(),
					AuthorizationURL: "https://example.test/auth/" + fake.UUID().V4(),
				},
			})
			pendingStore := newPendingStartStoreFixture(t, pendingStartStoreFixture{})
			coordinator, err := NewLinkCoordinator(LinkCoordinatorArgs{
				ProviderProfileRegistry: NewStaticProviderProfileRegistry(PKOProfile()),
				ConnectorRegistry:       NewStaticConnectorRegistry(connector.mock),
				PendingStartStore:       pendingStore.mock,
				ConnectionSecretWriter: newConnectionSecretWriterFixture(t, connectionSecretWriterFixture{
					secretID: "secret-" + fake.UUID().V4(),
				}).mock,
				ConnectionStore: newConnectionStoreFixture(t, connectionStoreFixture{}).mock,
			})
			require.NoError(t, err)

			_, err = coordinator.StartRedirectLink(t.Context(), RedirectLinkStartRequest{
				TenantID:           "tenant-" + fake.UUID().V4(),
				ActorUserID:        "actor-" + fake.UUID().V4(),
				ProviderID:         domain.ProviderIDPKO,
				RedirectURL:        "https://app.example.test/redirect/" + fake.UUID().V4(),
				BrowserCallbackURL: "http://localhost:5173/#/finance/connections",
			})
			require.NoError(t, err)
			require.Len(t, pendingStore.saved, 1)
			assert.Equal(
				t, defaultPendingStartTTL,
				pendingStore.saved[0].ExpiresAt.Sub(pendingStore.saved[0].CreatedAt),
			)
		})

		t.Run("wraps start-link failures", func(t *testing.T) {
			fake := faker.New()
			connector := newLinkConnectorFixture(t, linkConnectorFixture{
				connectorID:  domain.ProviderConnectorIDEnableBanking,
				capabilities: ConnectorCapabilities{SupportsStartLink: true},
				startErr:     errors.New("start failed"),
			})
			pendingStore := newPendingStartStoreFixture(t, pendingStartStoreFixture{})
			coordinator, err := NewLinkCoordinator(LinkCoordinatorArgs{
				ProviderProfileRegistry: NewStaticProviderProfileRegistry(PKOProfile()),
				ConnectorRegistry:       NewStaticConnectorRegistry(connector.mock),
				PendingStartStore:       pendingStore.mock,
				ConnectionSecretWriter: newConnectionSecretWriterFixture(t, connectionSecretWriterFixture{
					secretID: "secret-" + fake.UUID().V4(),
				}).mock,
				ConnectionStore: newConnectionStoreFixture(t, connectionStoreFixture{}).mock,
			})
			require.NoError(t, err)

			_, err = coordinator.StartRedirectLink(t.Context(), RedirectLinkStartRequest{
				TenantID:           "tenant-" + fake.UUID().V4(),
				ActorUserID:        "actor-" + fake.UUID().V4(),
				ProviderID:         domain.ProviderIDPKO,
				RedirectURL:        "https://app.example.test/redirect/" + fake.UUID().V4(),
				BrowserCallbackURL: "http://localhost:5173/#/finance/connections",
			})
			require.ErrorContains(t, err, "start redirect link")

			connector.startErr = nil
			connector.startResult = StartLinkResult{
				State:            "state-" + fake.UUID().V4(),
				AuthorizationURL: "https://example.test/auth",
			}
			pendingStore.saveErr = errors.New("save pending failed")
			_, err = coordinator.StartRedirectLink(t.Context(), RedirectLinkStartRequest{
				TenantID:           "tenant-" + fake.UUID().V4(),
				ActorUserID:        "actor-" + fake.UUID().V4(),
				ProviderID:         domain.ProviderIDPKO,
				RedirectURL:        "https://app.example.test/redirect/" + fake.UUID().V4(),
				BrowserCallbackURL: "http://localhost:5173/#/finance/connections",
			})
			require.ErrorContains(t, err, "save pending start")
		})
	})

	t.Run("resolves product providers and routes token and redirect link methods", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.June, 29, 12, 0, 0, 0, time.UTC)
		monobankConnector := newLinkConnectorFixture(t, linkConnectorFixture{
			connectorID:  domain.ProviderConnectorIDMonobank,
			capabilities: ConnectorCapabilities{SupportsTokenLink: true},
			tokenResult: LinkResult{
				DisplayName:       "mono-" + fake.Person().FirstName(),
				ProviderReference: "provider-ref-" + fake.UUID().V4(),
				Secret:            "token-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
				ConnectionSnapshot: &domain.ProviderSnapshotObservation{
					Kind:             domain.ProviderSnapshotKindConnection,
					ProviderObjectID: "client-info",
					DocumentJSON:     []byte(`{"name":"typed"}`),
					CapturedAt:       now,
				},
			},
		})
		enableBankingConnector := newLinkConnectorFixture(t, linkConnectorFixture{
			connectorID:  domain.ProviderConnectorIDEnableBanking,
			capabilities: ConnectorCapabilities{SupportsStartLink: true},
			startResult: StartLinkResult{
				State:            "state-" + fake.UUID().V4(),
				AuthorizationURL: "https://example.test/auth/" + fake.UUID().V4(),
			},
		})
		pendingStore := newPendingStartStoreFixture(t, pendingStartStoreFixture{})
		secretWriter := newConnectionSecretWriterFixture(t, connectionSecretWriterFixture{
			secretID: "secret-" + fake.UUID().V4(),
		})
		connectionStore := newConnectionStoreFixture(t, connectionStoreFixture{})

		coordinator, err := NewLinkCoordinator(LinkCoordinatorArgs{
			ProviderProfileRegistry: NewStaticProviderProfileRegistry(
				ProviderProfile{ProviderID: domain.ProviderIDMonobank, ConnectorID: domain.ProviderConnectorIDMonobank},
				PKOProfile(),
			),
			ConnectorRegistry:      NewStaticConnectorRegistry(monobankConnector.mock, enableBankingConnector.mock),
			PendingStartStore:      pendingStore.mock,
			ConnectionSecretWriter: secretWriter.mock,
			ConnectionStore:        connectionStore.mock,
			Now:                    func() time.Time { return now },
			NewID:                  func() string { return "id-" + fake.UUID().V4() },
			PendingStartTTL:        15 * time.Minute,
		})
		require.NoError(t, err)

		_, err = coordinator.StartRedirectLink(t.Context(), RedirectLinkStartRequest{
			TenantID:           "tenant-" + fake.UUID().V4(),
			ActorUserID:        "actor-" + fake.UUID().V4(),
			ProviderID:         domain.ProviderIDMonobank,
			RedirectURL:        "https://app.example.test/redirect/" + fake.UUID().V4(),
			BrowserCallbackURL: "http://localhost:5173/#/finance/connections",
		})
		require.ErrorIs(t, err, ErrRedirectLinkUnsupported)
		assert.Zero(t, monobankConnector.startCalls)
		assert.Empty(t, secretWriter.saved)

		_, err = coordinator.LinkToken(t.Context(), TokenLinkRequest{
			TenantID:    "tenant-" + fake.UUID().V4(),
			ActorUserID: "actor-" + fake.UUID().V4(),
			ProviderID:  domain.ProviderIDPKO,
			Token:       "token-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrTokenLinkUnsupported)
		assert.Zero(t, enableBankingConnector.tokenCalls)
		assert.Empty(t, secretWriter.saved)

		connection, err := coordinator.LinkToken(t.Context(), TokenLinkRequest{
			TenantID:    "tenant-token-" + fake.UUID().V4(),
			ActorUserID: "actor-token-" + fake.UUID().V4(),
			ProviderID:  domain.ProviderIDMonobank,
			Token:       "token-" + fake.UUID().V4(),
		})
		require.NoError(t, err)
		assert.Equal(t, domain.ProviderIDMonobank, monobankConnector.lastToken.Profile.ProviderID)
		assert.Equal(t, domain.ProviderConnectorIDMonobank, monobankConnector.lastToken.Profile.ConnectorID)
		assert.Equal(t, domain.ProviderConnectorIDMonobank, connection.ConnectorID)
		require.Len(t, connectionStore.snapshots, 1)
		require.NotNil(t, connectionStore.snapshots[0])
		assert.Equal(t, domain.ProviderSnapshotKindConnection, connectionStore.snapshots[0].Kind)
		assert.Equal(t, "client-info", connectionStore.snapshots[0].ProviderObjectID)

		start, err := coordinator.StartRedirectLink(t.Context(), RedirectLinkStartRequest{
			TenantID:           "tenant-redirect-" + fake.UUID().V4(),
			ActorUserID:        "actor-redirect-" + fake.UUID().V4(),
			ProviderID:         domain.ProviderIDPKO,
			RedirectURL:        "https://app.example.test/redirect/" + fake.UUID().V4(),
			BrowserCallbackURL: "http://localhost:5173/#/finance/connections",
		})
		require.NoError(t, err)
		assert.Equal(t, PKOProfile(), enableBankingConnector.lastStart.Profile)
		require.Len(t, pendingStore.saved, 1)
		assert.Equal(t, string(domain.ProviderIDPKO), pendingStore.saved[0].Provider)
		assert.Equal(t, domain.ProviderConnectorIDEnableBanking, pendingStore.saved[0].ConnectorID)
		assert.Equal(t, start.State, pendingStore.saved[0].StartResult.State)

		_, err = coordinator.LinkToken(t.Context(), TokenLinkRequest{
			TenantID:    "tenant-missing-" + fake.UUID().V4(),
			ActorUserID: "actor-missing-" + fake.UUID().V4(),
			ProviderID:  domain.ProviderID("missing-" + fake.UUID().V4()),
			Token:       "token-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrProviderNotConfigured)
		assert.Equal(t, 1, monobankConnector.tokenCalls)
	})

	t.Run("sanitizes pending redirect-start documents without creating current snapshots", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.August, 14, 14, 0, 0, 0, time.UTC)
		credentialSentinel := "credential-" + fake.UUID().V4()
		pendingDocument := []byte(
			`{"name":"typed","nested":{"accessToken":"` +
				credentialSentinel +
				`","privateKey":"` +
				credentialSentinel +
				`"},"items":[{"password":"` +
				credentialSentinel +
				`","visible":"yes"}]}`,
		)
		connector := newLinkConnectorFixture(t, linkConnectorFixture{
			connectorID:  domain.ProviderConnectorIDEnableBanking,
			capabilities: ConnectorCapabilities{SupportsStartLink: true},
			startResult: StartLinkResult{
				State:            "state-" + fake.UUID().V4(),
				AuthorizationURL: "https://example.test/auth/" + fake.UUID().V4(),
				PendingDocument:  pendingDocument,
			},
		})
		pendingStore := newPendingStartStoreFixture(t, pendingStartStoreFixture{})
		connectionStore := newConnectionStoreFixture(t, connectionStoreFixture{})
		coordinator, err := NewLinkCoordinator(LinkCoordinatorArgs{
			ProviderProfileRegistry: NewStaticProviderProfileRegistry(PKOProfile()),
			ConnectorRegistry:       NewStaticConnectorRegistry(connector.mock),
			PendingStartStore:       pendingStore.mock,
			ConnectionSecretWriter:  newConnectionSecretWriterFixture(t, connectionSecretWriterFixture{}).mock,
			ConnectionStore:         connectionStore.mock,
			Now:                     func() time.Time { return now },
			NewID:                   func() string { return "pending-" + fake.UUID().V4() },
		})
		require.NoError(t, err)

		_, err = coordinator.StartRedirectLink(t.Context(), RedirectLinkStartRequest{
			TenantID:           "tenant-" + fake.UUID().V4(),
			ActorUserID:        "actor-" + fake.UUID().V4(),
			ProviderID:         domain.ProviderIDPKO,
			RedirectURL:        "https://app.example.test/redirect/" + fake.UUID().V4(),
			BrowserCallbackURL: "http://localhost:5173/#/finance/connections",
		})
		require.NoError(t, err)
		require.Len(t, pendingStore.saved, 1)
		assert.JSONEq(
			t,
			`{"name":"typed","nested":{},"items":[{"visible":"yes"}]}`,
			string(pendingStore.saved[0].StartResult.DocumentJSON),
		)
		assert.NotContains(t, string(pendingStore.saved[0].StartResult.DocumentJSON), credentialSentinel)
		assert.Empty(t, connectionStore.saved)
		assert.Empty(t, connectionStore.snapshots)
	})

	t.Run("rejects invalid pending redirect-start documents before persistence", func(t *testing.T) {
		fake := faker.New()
		connector := newLinkConnectorFixture(t, linkConnectorFixture{
			connectorID:  domain.ProviderConnectorIDEnableBanking,
			capabilities: ConnectorCapabilities{SupportsStartLink: true},
			startResult: StartLinkResult{
				State:            "state-" + fake.UUID().V4(),
				AuthorizationURL: "https://example.test/auth/" + fake.UUID().V4(),
				PendingDocument:  []byte(`{"invalid"`),
			},
		})
		pendingStore := newPendingStartStoreFixture(t, pendingStartStoreFixture{})
		connectionStore := newConnectionStoreFixture(t, connectionStoreFixture{})
		coordinator, err := NewLinkCoordinator(LinkCoordinatorArgs{
			ProviderProfileRegistry: NewStaticProviderProfileRegistry(PKOProfile()),
			ConnectorRegistry:       NewStaticConnectorRegistry(connector.mock),
			PendingStartStore:       pendingStore.mock,
			ConnectionSecretWriter:  newConnectionSecretWriterFixture(t, connectionSecretWriterFixture{}).mock,
			ConnectionStore:         connectionStore.mock,
		})
		require.NoError(t, err)

		_, err = coordinator.StartRedirectLink(t.Context(), RedirectLinkStartRequest{
			TenantID:           "tenant-" + fake.UUID().V4(),
			ActorUserID:        "actor-" + fake.UUID().V4(),
			ProviderID:         domain.ProviderIDPKO,
			RedirectURL:        "https://app.example.test/redirect/" + fake.UUID().V4(),
			BrowserCallbackURL: "http://localhost:5173/#/finance/connections",
		})
		require.ErrorContains(t, err, "sanitize pending start document")
		assert.Empty(t, pendingStore.saved)
		assert.Empty(t, connectionStore.saved)
		assert.Empty(t, connectionStore.snapshots)
	})

	t.Run("supports connector-declared redirect lifecycle requirements", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.June, 29, 15, 0, 0, 0, time.UTC)
		syntheticState := "synthetic-state-" + fake.UUID().V4()
		syntheticConnector := newLinkConnectorFixture(t, linkConnectorFixture{
			connectorID: domain.ProviderConnectorIDSynthetic,
			capabilities: ConnectorCapabilities{
				SupportsStartLink:  true,
				SupportsFinishLink: true,
			},
			startResult: StartLinkResult{
				State:             syntheticState,
				ProviderReference: syntheticState,
				AuthorizationURL:  "#/finance/connections/synthetic?state=" + syntheticState,
			},
			finishResult: LinkResult{
				DisplayName:       "Synthetic",
				ProviderReference: syntheticState,
				State:             domain.BankConnectionStateActive,
			},
		})
		pkoState := "pko-state-" + fake.UUID().V4()
		pkoConnector := newLinkConnectorFixture(t, linkConnectorFixture{
			connectorID:  domain.ProviderConnectorIDEnableBanking,
			capabilities: ConnectorCapabilities{SupportsFinishLink: true, RequiresRedirectCode: true},
			finishResult: LinkResult{
				DisplayName:       "PKO " + fake.Company().Name(),
				ProviderReference: "provider-ref-" + fake.UUID().V4(),
				Secret:            "secret-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
			},
		})
		pendingStore := newPendingStartStoreFixture(t, pendingStartStoreFixture{
			savedByState: map[string]domain.PendingBankConnectionLinkStart{
				pkoState: {
					ID:          "pending-pko-" + fake.UUID().V4(),
					TenantID:    "tenant-pko-" + fake.UUID().V4(),
					ActorUserID: "actor-pko-" + fake.UUID().V4(),
					Provider:    string(domain.ProviderIDPKO),
					ConnectorID: domain.ProviderConnectorIDEnableBanking,
					State:       pkoState,
					StartResult: domain.PendingBankConnectionLinkStartResult{State: pkoState},
					ExpiresAt:   now.Add(15 * time.Minute),
				},
			},
		})
		secretWriter := newConnectionSecretWriterFixture(t, connectionSecretWriterFixture{
			secretID: "secret-" + fake.UUID().V4(),
		})
		connectionStore := newConnectionStoreFixture(t, connectionStoreFixture{})
		coordinator, err := NewLinkCoordinator(LinkCoordinatorArgs{
			ProviderProfileRegistry: NewStaticProviderProfileRegistry(
				PKOProfile(),
				ProviderProfile{
					ProviderID:  domain.ProviderIDSynthetic,
					ConnectorID: domain.ProviderConnectorIDSynthetic,
				},
			),
			ConnectorRegistry:      NewStaticConnectorRegistry(syntheticConnector.mock, pkoConnector.mock),
			PendingStartStore:      pendingStore.mock,
			ConnectionSecretWriter: secretWriter.mock,
			ConnectionStore:        connectionStore.mock,
			Now:                    func() time.Time { return now },
			NewID:                  func() string { return "id-" + fake.UUID().V4() },
		})
		require.NoError(t, err)

		started, err := coordinator.StartRedirectLink(t.Context(), RedirectLinkStartRequest{
			TenantID:           "tenant-synthetic-" + fake.UUID().V4(),
			ActorUserID:        "actor-synthetic-" + fake.UUID().V4(),
			ProviderID:         domain.ProviderIDSynthetic,
			RedirectURL:        "https://backend.example.test/callback/" + fake.UUID().V4(),
			BrowserCallbackURL: "http://localhost:5173/#/finance/connections",
		})
		require.NoError(t, err)
		assert.Equal(t, syntheticState, started.State)
		require.Len(t, pendingStore.saved, 1)
		assert.Equal(t, syntheticState, pendingStore.saved[0].ProviderReference)
		assert.Equal(t, "#/finance/connections/synthetic?state="+syntheticState, pendingStore.saved[0].AuthorizationURL)

		finished, err := coordinator.FinishRedirectLink(t.Context(), RedirectLinkFinishRequest{
			TenantID:    pendingStore.saved[0].TenantID,
			ActorUserID: pendingStore.saved[0].ActorUserID,
			ProviderID:  domain.ProviderIDSynthetic,
			State:       syntheticState,
		})
		require.NoError(t, err)
		assert.Equal(t, syntheticState, syntheticConnector.lastFinish.State)
		assert.Empty(t, syntheticConnector.lastFinish.Code)
		assert.Equal(t, syntheticState, finished.ProviderReference)
		assert.Equal(t, domain.BankConnectionStateActive, finished.State)
		require.Len(t, secretWriter.saved, 1)
		assert.Equal(t, syntheticState, secretWriter.saved[0].reference)
		assert.Empty(t, secretWriter.saved[0].secret)

		consumedBeforePKOFinish := len(pendingStore.consumed)
		_, err = coordinator.FinishRedirectLink(t.Context(), RedirectLinkFinishRequest{
			TenantID:    pendingStore.savedByState[pkoState].TenantID,
			ActorUserID: pendingStore.savedByState[pkoState].ActorUserID,
			ProviderID:  domain.ProviderIDPKO,
			State:       pkoState,
		})
		require.ErrorContains(t, err, "redirect code is required")
		assert.Zero(t, pkoConnector.finishCalls)
		assert.Len(t, pendingStore.consumed, consumedBeforePKOFinish)
		assert.Contains(t, pendingStore.savedByState, pkoState)
		assert.Len(t, secretWriter.saved, 1)
	})

	t.Run("surfaces link coordinator resolver, unsupported, and persistence edge errors", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.June, 29, 13, 0, 0, 0, time.UTC)

		t.Run("returns resolve link connector errors", func(t *testing.T) {
			connector := newLinkConnectorFixture(t, linkConnectorFixture{
				connectorID:  domain.ProviderConnectorIDMonobank,
				capabilities: ConnectorCapabilities{SupportsStartLink: true},
			})
			coordinator, err := NewLinkCoordinator(LinkCoordinatorArgs{
				ProviderProfileRegistry: NewStaticProviderProfileRegistry(PKOProfile()),
				ConnectorRegistry:       NewStaticConnectorRegistry(connector.mock),
				PendingStartStore:       newPendingStartStoreFixture(t, pendingStartStoreFixture{}).mock,
				ConnectionSecretWriter: newConnectionSecretWriterFixture(t, connectionSecretWriterFixture{
					secretID: "secret-" + fake.UUID().V4(),
				}).mock,
				ConnectionStore: newConnectionStoreFixture(t, connectionStoreFixture{}).mock,
			})
			require.NoError(t, err)

			_, err = coordinator.StartRedirectLink(t.Context(), RedirectLinkStartRequest{
				TenantID:           "tenant-start-" + fake.UUID().V4(),
				ActorUserID:        "actor-start-" + fake.UUID().V4(),
				ProviderID:         domain.ProviderIDPKO,
				RedirectURL:        "https://app.example.test/redirect/" + fake.UUID().V4(),
				BrowserCallbackURL: "http://localhost:5173/#/finance/connections",
			})
			require.ErrorContains(t, err, "resolve link connector")

			_, err = coordinator.FinishRedirectLink(t.Context(), RedirectLinkFinishRequest{
				TenantID:    "tenant-finish-" + fake.UUID().V4(),
				ActorUserID: "actor-finish-" + fake.UUID().V4(),
				ProviderID:  domain.ProviderIDPKO,
				State:       "state-" + fake.UUID().V4(),
				Code:        "code-" + fake.UUID().V4(),
			})
			require.ErrorContains(t, err, "resolve link connector")

			_, err = coordinator.LinkToken(t.Context(), TokenLinkRequest{
				TenantID:    "tenant-token-" + fake.UUID().V4(),
				ActorUserID: "actor-token-" + fake.UUID().V4(),
				ProviderID:  domain.ProviderIDPKO,
				Token:       "token-" + fake.UUID().V4(),
			})
			require.ErrorContains(t, err, "resolve link connector")
		})

		t.Run("returns unsupported finish for connector without finish capability", func(t *testing.T) {
			providerProfile := ProviderProfile{
				ProviderID:  domain.ProviderIDMonobank,
				ConnectorID: domain.ProviderConnectorIDMonobank,
			}
			connector := newLinkConnectorFixture(t, linkConnectorFixture{
				connectorID:  domain.ProviderConnectorIDMonobank,
				capabilities: ConnectorCapabilities{SupportsFinishLink: false},
			})
			pendingStore := newPendingStartStoreFixture(t, pendingStartStoreFixture{})
			coordinator, err := NewLinkCoordinator(LinkCoordinatorArgs{
				ProviderProfileRegistry: NewStaticProviderProfileRegistry(providerProfile),
				ConnectorRegistry:       NewStaticConnectorRegistry(connector.mock),
				PendingStartStore:       pendingStore.mock,
				ConnectionSecretWriter: newConnectionSecretWriterFixture(t, connectionSecretWriterFixture{
					secretID: "secret-" + fake.UUID().V4(),
				}).mock,
				ConnectionStore: newConnectionStoreFixture(t, connectionStoreFixture{}).mock,
			})
			require.NoError(t, err)

			_, err = coordinator.FinishRedirectLink(t.Context(), RedirectLinkFinishRequest{
				TenantID:    "tenant-finish-unsupported-" + fake.UUID().V4(),
				ActorUserID: "actor-finish-unsupported-" + fake.UUID().V4(),
				ProviderID:  domain.ProviderIDMonobank,
				State:       "state-" + fake.UUID().V4(),
				Code:        "code-" + fake.UUID().V4(),
			})
			require.ErrorIs(t, err, ErrRedirectLinkUnsupported)
			require.Empty(t, pendingStore.consumed)
		})

		t.Run("returns consume pending start errors for generic storage failures", func(t *testing.T) {
			connector := newLinkConnectorFixture(t, linkConnectorFixture{
				connectorID:  domain.ProviderConnectorIDEnableBanking,
				capabilities: ConnectorCapabilities{SupportsFinishLink: true},
			})
			pendingStore := newPendingStartStoreFixture(t, pendingStartStoreFixture{
				consumeErr: errors.New("consume failed"),
			})
			coordinator, err := NewLinkCoordinator(LinkCoordinatorArgs{
				ProviderProfileRegistry: NewStaticProviderProfileRegistry(PKOProfile()),
				ConnectorRegistry:       NewStaticConnectorRegistry(connector.mock),
				PendingStartStore:       pendingStore.mock,
				ConnectionSecretWriter: newConnectionSecretWriterFixture(t, connectionSecretWriterFixture{
					secretID: "secret-" + fake.UUID().V4(),
				}).mock,
				ConnectionStore: newConnectionStoreFixture(t, connectionStoreFixture{}).mock,
			})
			require.NoError(t, err)

			_, err = coordinator.FinishRedirectLink(t.Context(), RedirectLinkFinishRequest{
				TenantID:    "tenant-consume-fail-" + fake.UUID().V4(),
				ActorUserID: "actor-consume-fail-" + fake.UUID().V4(),
				ProviderID:  domain.ProviderIDPKO,
				State:       "state-" + fake.UUID().V4(),
				Code:        "code-" + fake.UUID().V4(),
			})
			require.ErrorContains(t, err, "consume pending start")
		})

		t.Run("returns link token errors for connector and secret failures", func(t *testing.T) {
			connector := newLinkConnectorFixture(t, linkConnectorFixture{
				connectorID:  domain.ProviderConnectorIDMonobank,
				capabilities: ConnectorCapabilities{SupportsTokenLink: true},
				tokenErr:     errors.New("token connector failed"),
				tokenResult:  LinkResult{Secret: "secret-" + fake.UUID().V4()},
			})
			secretWriter := newConnectionSecretWriterFixture(t, connectionSecretWriterFixture{
				secretID: "secret-row-" + fake.UUID().V4(),
			})
			connectionStore := newConnectionStoreFixture(t, connectionStoreFixture{})
			providerProfiles := []ProviderProfile{
				PKOProfile(),
				{ProviderID: domain.ProviderIDMonobank, ConnectorID: domain.ProviderConnectorIDMonobank},
			}
			coordinator, err := NewLinkCoordinator(LinkCoordinatorArgs{
				ProviderProfileRegistry: NewStaticProviderProfileRegistry(providerProfiles...),
				ConnectorRegistry:       NewStaticConnectorRegistry(connector.mock),
				PendingStartStore:       newPendingStartStoreFixture(t, pendingStartStoreFixture{}).mock,
				ConnectionSecretWriter:  secretWriter.mock,
				ConnectionStore:         connectionStore.mock,
			})
			require.NoError(t, err)

			_, err = coordinator.LinkToken(t.Context(), TokenLinkRequest{
				TenantID:    "tenant-token-conn-fail-" + fake.UUID().V4(),
				ActorUserID: "actor-token-conn-fail-" + fake.UUID().V4(),
				ProviderID:  domain.ProviderIDMonobank,
				Token:       "token-" + fake.UUID().V4(),
			})
			require.ErrorContains(t, err, "link token")

			connector.tokenErr = nil
			connector.tokenResult = LinkResult{
				ProviderReference: "provider-ref-" + fake.UUID().V4(),
				Secret:            "token-secret-" + fake.UUID().V4(),
			}
			secretWriter.saveErr = errors.New("secret save failed")

			_, err = coordinator.LinkToken(t.Context(), TokenLinkRequest{
				TenantID:    "tenant-token-secret-fail-" + fake.UUID().V4(),
				ActorUserID: "actor-token-secret-fail-" + fake.UUID().V4(),
				ProviderID:  domain.ProviderIDMonobank,
				Token:       "token-" + fake.UUID().V4(),
			})
			require.ErrorContains(t, err, "save connection secret")
		})

		t.Run("restores failed finish flow when restore operation fails", func(t *testing.T) {
			state := "state-" + fake.UUID().V4()
			connector := newLinkConnectorFixture(t, linkConnectorFixture{
				connectorID:  domain.ProviderConnectorIDEnableBanking,
				capabilities: ConnectorCapabilities{SupportsFinishLink: true},
				finishErr:    errors.New("finish failed"),
			})
			pendingStore := newPendingStartStoreFixture(t, pendingStartStoreFixture{
				savedByState: map[string]domain.PendingBankConnectionLinkStart{
					state: {
						ID:          "pending-" + fake.UUID().V4(),
						TenantID:    "tenant-" + fake.UUID().V4(),
						ActorUserID: "actor-" + fake.UUID().V4(),
						Provider:    string(domain.ProviderIDPKO),
						ConnectorID: domain.ProviderConnectorIDEnableBanking,
						State:       state,
						StartResult: domain.PendingBankConnectionLinkStartResult{
							State:            "start-" + fake.UUID().V4(),
							AuthorizationURL: "https://example.test/start",
						},
						ExpiresAt: now.Add(15 * time.Minute),
					},
				},
				restoreErr: errors.New("restore failed"),
			})
			coordinator, err := NewLinkCoordinator(LinkCoordinatorArgs{
				ProviderProfileRegistry: NewStaticProviderProfileRegistry(PKOProfile()),
				ConnectorRegistry:       NewStaticConnectorRegistry(connector.mock),
				PendingStartStore:       pendingStore.mock,
				ConnectionSecretWriter: newConnectionSecretWriterFixture(t, connectionSecretWriterFixture{
					secretID: "secret-row-" + fake.UUID().V4(),
				}).mock,
				ConnectionStore: newConnectionStoreFixture(t, connectionStoreFixture{}).mock,
				Now:             func() time.Time { return now },
			})
			require.NoError(t, err)

			_, err = coordinator.FinishRedirectLink(t.Context(), RedirectLinkFinishRequest{
				TenantID:    pendingStore.savedByState[state].TenantID,
				ActorUserID: pendingStore.savedByState[state].ActorUserID,
				ProviderID:  domain.ProviderIDPKO,
				State:       state,
				Code:        "code-" + fake.UUID().V4(),
			})
			require.ErrorContains(t, err, "finish redirect link")
			require.ErrorContains(t, err, "restore pending bank connection link start")
			require.Equal(t, 1, pendingStore.restoreCallsByState[state])
		})

		t.Run("returns token-link persistence errors and save-fallback behavior", func(t *testing.T) {
			connector := newLinkConnectorFixture(t, linkConnectorFixture{
				connectorID:  domain.ProviderConnectorIDEnableBanking,
				capabilities: ConnectorCapabilities{SupportsTokenLink: true},
				tokenResult: LinkResult{
					ProviderReference: "provider-ref-" + fake.UUID().V4(),
					Secret:            "secret-" + fake.UUID().V4(),
				},
			})
			coordinatorWithListError, err := NewLinkCoordinator(LinkCoordinatorArgs{
				ProviderProfileRegistry: NewStaticProviderProfileRegistry(PKOProfile()),
				ConnectorRegistry:       NewStaticConnectorRegistry(connector.mock),
				PendingStartStore:       newPendingStartStoreFixture(t, pendingStartStoreFixture{}).mock,
				ConnectionSecretWriter: newConnectionSecretWriterFixture(t, connectionSecretWriterFixture{
					secretID: "secret-row-" + fake.UUID().V4(),
				}).mock,
				ConnectionStore: newConnectionStoreFixture(t, connectionStoreFixture{
					saveErr: errors.New("save failed"),
				}).mock,
				Now: func() time.Time { return now },
			})
			require.NoError(t, err)

			_, err = coordinatorWithListError.LinkToken(t.Context(), TokenLinkRequest{
				TenantID:    "tenant-list-error-" + fake.UUID().V4(),
				ActorUserID: "actor-list-error-" + fake.UUID().V4(),
				ProviderID:  domain.ProviderIDPKO,
				Token:       "token-" + fake.UUID().V4(),
			})
			require.ErrorContains(t, err, "save linked bank connection")

			connector.tokenErr = nil
			connector.tokenResult = LinkResult{
				ProviderReference: "provider-ref-" + fake.UUID().V4(),
				Secret:            "secret-" + fake.UUID().V4(),
			}
			connectionStore := newConnectionStoreFixture(t, connectionStoreFixture{
				saveErr: errors.New("save failed"),
			})
			providerProfile := ProviderProfile{
				ProviderID:  domain.ProviderIDMonobank,
				ConnectorID: domain.ProviderConnectorIDEnableBanking,
			}
			coordinatorWithSaveError, err := NewLinkCoordinator(LinkCoordinatorArgs{
				ProviderProfileRegistry: NewStaticProviderProfileRegistry(providerProfile),
				ConnectorRegistry:       NewStaticConnectorRegistry(connector.mock),
				PendingStartStore:       newPendingStartStoreFixture(t, pendingStartStoreFixture{}).mock,
				ConnectionSecretWriter: newConnectionSecretWriterFixture(t, connectionSecretWriterFixture{
					secretID: "secret-row-" + fake.UUID().V4(),
				}).mock,
				ConnectionStore: connectionStore.mock,
				Now:             func() time.Time { return now },
			})
			require.NoError(t, err)

			_, err = coordinatorWithSaveError.LinkToken(t.Context(), TokenLinkRequest{
				TenantID:    "tenant-save-error-" + fake.UUID().V4(),
				ActorUserID: "actor-save-error-" + fake.UUID().V4(),
				ProviderID:  domain.ProviderIDMonobank,
				Token:       "token-" + fake.UUID().V4(),
			})
			require.ErrorContains(t, err, "save linked bank connection")

			var coordinatorWithNow *LinkCoordinator
			providerProfile = ProviderProfile{
				ProviderID:  domain.ProviderIDMonobank,
				ConnectorID: domain.ProviderConnectorIDEnableBanking,
			}
			coordinatorWithNow, err = NewLinkCoordinator(LinkCoordinatorArgs{
				ProviderProfileRegistry: NewStaticProviderProfileRegistry(providerProfile),
				ConnectorRegistry:       NewStaticConnectorRegistry(connector.mock),
				PendingStartStore:       newPendingStartStoreFixture(t, pendingStartStoreFixture{}).mock,
				ConnectionSecretWriter: newConnectionSecretWriterFixture(t, connectionSecretWriterFixture{
					secretID: "secret-row-" + fake.UUID().V4(),
				}).mock,
				ConnectionStore: newConnectionStoreFixture(t, connectionStoreFixture{}).mock,
				Now:             func() time.Time { return now },
			})
			require.NoError(t, err)

			connection, err := coordinatorWithNow.LinkToken(t.Context(), TokenLinkRequest{
				TenantID:    "tenant-captured-now-" + fake.UUID().V4(),
				ActorUserID: "actor-captured-now-" + fake.UUID().V4(),
				ProviderID:  domain.ProviderIDMonobank,
				Token:       "token-" + fake.UUID().V4(),
			})
			require.NoError(t, err)
			require.Equal(t, string(domain.ProviderIDMonobank), connection.Provider)
		})
	})

	t.Run(
		"finishing redirect links consumes matching pending starts restores retries and prevents duplicates",
		func(t *testing.T) {
			fake := faker.New()
			now := time.Date(2026, time.June, 29, 14, 0, 0, 0, time.UTC)
			pendingStart := domain.PendingBankConnectionLinkStart{
				ID:          "pending-" + fake.UUID().V4(),
				TenantID:    "tenant-" + fake.UUID().V4(),
				ActorUserID: "actor-" + fake.UUID().V4(),
				Provider:    string(domain.ProviderIDPKO),
				ConnectorID: domain.ProviderConnectorIDEnableBanking,
				State:       "state-" + fake.UUID().V4(),
				StartResult: domain.PendingBankConnectionLinkStartResult{
					State:            "start-state-" + fake.UUID().V4(),
					AuthorizationURL: "https://example.test/start/" + fake.UUID().V4(),
					DocumentJSON:     []byte(`{"phase":"start"}`),
				},
				ExpiresAt: now.Add(15 * time.Minute),
			}
			connector := newLinkConnectorFixture(t, linkConnectorFixture{
				connectorID:  domain.ProviderConnectorIDEnableBanking,
				capabilities: ConnectorCapabilities{SupportsFinishLink: true},
				finishResult: LinkResult{
					DisplayName:       "pko-" + fake.Company().Name(),
					ProviderReference: "provider-ref-" + fake.UUID().V4(),
					Secret:            "secret-" + fake.UUID().V4(),
					State:             domain.BankConnectionStateActive,
				},
			})
			pendingStore := newPendingStartStoreFixture(
				t,
				pendingStartStoreFixture{
					savedByState: map[string]domain.PendingBankConnectionLinkStart{
						pendingStart.State: pendingStart,
					},
				},
			)
			connectionStore := newConnectionStoreFixture(t, connectionStoreFixture{})

			coordinator, err := NewLinkCoordinator(LinkCoordinatorArgs{
				ProviderProfileRegistry: NewStaticProviderProfileRegistry(PKOProfile()),
				ConnectorRegistry:       NewStaticConnectorRegistry(connector.mock),
				PendingStartStore:       pendingStore.mock,
				ConnectionSecretWriter: newConnectionSecretWriterFixture(t, connectionSecretWriterFixture{
					secretID: "secret-row-" + fake.UUID().V4(),
				}).mock,
				ConnectionStore: connectionStore.mock,
				Now:             func() time.Time { return now },
				NewID:           func() string { return "id-" + fake.UUID().V4() },
				PendingStartTTL: 15 * time.Minute,
			})
			require.NoError(t, err)

			connection, err := coordinator.FinishRedirectLink(t.Context(), RedirectLinkFinishRequest{
				TenantID:    pendingStart.TenantID,
				ActorUserID: pendingStart.ActorUserID,
				ProviderID:  domain.ProviderIDPKO,
				State:       pendingStart.State,
				Code:        "code-" + fake.UUID().V4(),
			})
			require.NoError(t, err)
			require.Len(t, pendingStore.consumed, 1)
			assert.Equal(t, domain.ProviderIDPKO, pendingStore.consumed[0].ProviderID)
			assert.Equal(t, domain.ProviderConnectorIDEnableBanking, pendingStore.consumed[0].ConnectorID)
			assert.Equal(t, pendingStart.StartResult.State, connector.lastFinish.Start.State)
			assert.Equal(t, domain.ProviderConnectorIDEnableBanking, connection.ConnectorID)

			_, err = coordinator.FinishRedirectLink(t.Context(), RedirectLinkFinishRequest{
				TenantID:    pendingStart.TenantID,
				ActorUserID: pendingStart.ActorUserID,
				ProviderID:  domain.ProviderIDPKO,
				State:       pendingStart.State,
				Code:        "code-duplicate-" + fake.UUID().V4(),
			})
			require.ErrorIs(t, err, ErrPendingStartNotFound)
			assert.Equal(t, 1, connector.finishCalls)

			connector.finishErr = errors.New("finish failed")
			retryState := "retry-state-" + fake.UUID().V4()
			pendingStore.savedByState[retryState] = pendingStart
			pendingStore.savedByState[retryState] = domain.PendingBankConnectionLinkStart{
				ID:          pendingStart.ID,
				TenantID:    pendingStart.TenantID,
				ActorUserID: pendingStart.ActorUserID,
				Provider:    pendingStart.Provider,
				ConnectorID: pendingStart.ConnectorID,
				State:       retryState,
				StartResult: pendingStart.StartResult,
				ExpiresAt:   pendingStart.ExpiresAt,
			}
			_, err = coordinator.FinishRedirectLink(t.Context(), RedirectLinkFinishRequest{
				TenantID:    pendingStart.TenantID,
				ActorUserID: pendingStart.ActorUserID,
				ProviderID:  domain.ProviderIDPKO,
				State:       retryState,
				Code:        "code-retry-" + fake.UUID().V4(),
			})
			require.ErrorContains(t, err, "finish redirect link")
			assert.Equal(t, 1, pendingStore.restoreCallsByState[retryState])

			connector.finishErr = nil
			connectionStore.saveErr = errors.New("save connection failed")
			restoreOnSaveState := "restore-save-" + fake.UUID().V4()
			pendingStore.savedByState[restoreOnSaveState] = domain.PendingBankConnectionLinkStart{
				ID:          pendingStart.ID,
				TenantID:    pendingStart.TenantID,
				ActorUserID: pendingStart.ActorUserID,
				Provider:    pendingStart.Provider,
				ConnectorID: pendingStart.ConnectorID,
				State:       restoreOnSaveState,
				StartResult: pendingStart.StartResult,
				ExpiresAt:   pendingStart.ExpiresAt,
			}
			_, err = coordinator.FinishRedirectLink(t.Context(), RedirectLinkFinishRequest{
				TenantID:    pendingStart.TenantID,
				ActorUserID: pendingStart.ActorUserID,
				ProviderID:  domain.ProviderIDPKO,
				State:       restoreOnSaveState,
				Code:        "code-save-" + fake.UUID().V4(),
			})
			require.ErrorContains(t, err, "save bank connection")
			assert.Equal(t, 1, pendingStore.restoreCallsByState[restoreOnSaveState])

			connector.finishErr = nil
			connectionStore.saveErr = nil
			successfulRetryState := "retry-" + fake.UUID().V4()
			pendingStore.savedByState[successfulRetryState] = domain.PendingBankConnectionLinkStart{
				ID:          pendingStart.ID,
				TenantID:    pendingStart.TenantID,
				ActorUserID: pendingStart.ActorUserID,
				Provider:    pendingStart.Provider,
				ConnectorID: pendingStart.ConnectorID,
				State:       successfulRetryState,
				StartResult: pendingStart.StartResult,
				ExpiresAt:   pendingStart.ExpiresAt,
			}
			_, err = coordinator.FinishRedirectLink(t.Context(), RedirectLinkFinishRequest{
				TenantID:    pendingStart.TenantID,
				ActorUserID: pendingStart.ActorUserID,
				ProviderID:  domain.ProviderIDPKO,
				State:       successfulRetryState,
				Code:        "code-retry-" + fake.UUID().V4(),
			})
			require.NoError(t, err)
		})

	t.Run(
		"keeps distinct PKO sessions separate and retries the same session idempotently",
		func(t *testing.T) {
			fake := faker.New()
			now := time.Date(2026, time.June, 29, 16, 0, 0, 0, time.UTC)
			pendingState := "state-" + fake.UUID().V4()
			connector := newLinkConnectorFixture(t, linkConnectorFixture{
				connectorID:  domain.ProviderConnectorIDEnableBanking,
				capabilities: ConnectorCapabilities{SupportsFinishLink: true},
				finishResult: LinkResult{
					DisplayName:       "pko-" + fake.Company().Name(),
					ProviderReference: "provider-ref-" + fake.UUID().V4(),
					Secret:            "secret-plain-" + fake.UUID().V4(),
					State:             domain.BankConnectionStateActive,
				},
			})
			existing := domain.BankConnection{
				ID:                "existing-" + fake.UUID().V4(),
				TenantID:          "tenant-" + fake.UUID().V4(),
				Provider:          string(domain.ProviderIDPKO),
				ConnectorID:       domain.ProviderConnectorIDEnableBanking,
				ProviderReference: "existing-provider-ref-" + fake.UUID().V4(),
				SecretID:          "existing-secret-" + fake.UUID().V4(),
				CreatedAt:         now.Add(-24 * time.Hour),
			}
			pendingStore := newPendingStartStoreFixture(
				t,
				pendingStartStoreFixture{
					savedByState: map[string]domain.PendingBankConnectionLinkStart{
						pendingState: {
							ID:          "pending-" + fake.UUID().V4(),
							TenantID:    existing.TenantID,
							ActorUserID: "actor-" + fake.UUID().V4(),
							Provider:    string(domain.ProviderIDPKO),
							ConnectorID: domain.ProviderConnectorIDEnableBanking,
							State:       pendingState,
							StartResult: domain.PendingBankConnectionLinkStartResult{
								State:            pendingState,
								AuthorizationURL: "https://example.test/start/" + fake.UUID().V4(),
								DocumentJSON:     []byte(`{"phase":"start"}`),
							},
							ExpiresAt: now.Add(15 * time.Minute),
						},
					},
				},
			)
			secretWriter := newConnectionSecretWriterFixture(t, connectionSecretWriterFixture{
				secretID: "sealed-secret-" + fake.UUID().V4(),
			})
			connectionStore := newConnectionStoreFixture(t, connectionStoreFixture{
				listResult: []domain.BankConnection{existing},
			})

			coordinator, err := NewLinkCoordinator(LinkCoordinatorArgs{
				ProviderProfileRegistry: NewStaticProviderProfileRegistry(PKOProfile()),
				ConnectorRegistry:       NewStaticConnectorRegistry(connector.mock),
				PendingStartStore:       pendingStore.mock,
				ConnectionSecretWriter:  secretWriter.mock,
				ConnectionStore:         connectionStore.mock,
				Now:                     func() time.Time { return now },
				NewID:                   func() string { return "id-" + fake.UUID().V4() },
				PendingStartTTL:         15 * time.Minute,
			})
			require.NoError(t, err)

			_, err = coordinator.FinishRedirectLink(t.Context(), RedirectLinkFinishRequest{
				TenantID:    existing.TenantID,
				ActorUserID: pendingStore.savedByState[pendingState].ActorUserID,
				ProviderID:  domain.ProviderIDPKO,
				State:       pendingState,
				Code:        "code-" + fake.UUID().V4(),
			})
			require.NoError(t, err)
			require.Len(t, secretWriter.saved, 1)
			assert.Equal(t, string(domain.ProviderIDPKO), secretWriter.saved[0].provider)
			assert.Equal(t, connector.finishResult.ProviderReference, secretWriter.saved[0].reference)
			assert.Equal(t, connector.finishResult.Secret, secretWriter.saved[0].secret)
			require.Len(t, connectionStore.saved, 1)
			firstSaved := connectionStore.saved[0]
			assert.NotEqual(t, existing.ID, firstSaved.ID)
			assert.NotEqual(t, existing.SecretID, firstSaved.SecretID)
			assert.Equal(t, connector.finishResult.ProviderReference, firstSaved.ProviderReference)
			assert.Equal(t, secretWriter.secretID, firstSaved.SecretID)
			assert.Equal(t, string(domain.ProviderIDPKO), firstSaved.Provider)
			assert.Equal(t, domain.ProviderConnectorIDEnableBanking, firstSaved.ConnectorID)
			assert.Equal(t, now, firstSaved.CreatedAt)
		})

	t.Run("matches PKO sessions by exact non-empty provider reference", func(t *testing.T) {
		fake := faker.New()
		reference := "reference-" + fake.UUID().V4()
		existing := domain.BankConnection{
			Provider:          string(domain.ProviderIDPKO),
			ConnectorID:       domain.ProviderConnectorIDEnableBanking,
			ProviderReference: reference,
		}

		assert.True(t, hasSameProviderSessionIdentity(existing, existing))
		assert.False(t, hasSameProviderSessionIdentity(existing, domain.BankConnection{
			Provider:          existing.Provider,
			ConnectorID:       existing.ConnectorID,
			ProviderReference: "conflicting-reference-" + fake.UUID().V4(),
		}))
		assert.False(t, hasSameProviderSessionIdentity(existing, domain.BankConnection{
			Provider:    existing.Provider,
			ConnectorID: existing.ConnectorID,
		}))
	})

	t.Run("persists conflicting PKO sessions without replacing the first connection", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.August, 10, 17, 0, 0, 0, time.FixedZone("test", 2*60*60))
		existing := domain.BankConnection{
			ID:                "connection-existing-" + fake.UUID().V4(),
			TenantID:          "tenant-" + fake.UUID().V4(),
			Provider:          string(domain.ProviderIDPKO),
			ConnectorID:       domain.ProviderConnectorIDEnableBanking,
			ProviderReference: "reference-" + fake.UUID().V4(),
			SecretID:          "secret-existing-" + fake.UUID().V4(),
			State:             domain.BankConnectionStateActive,
			CreatedAt:         now.Add(-time.Hour),
			UpdatedAt:         now.Add(-time.Hour),
		}
		secretWriter := newConnectionSecretWriterFixture(t, connectionSecretWriterFixture{
			secretIDs: []string{
				"secret-reference-conflict-" + fake.UUID().V4(),
				"secret-retry-" + fake.UUID().V4(),
			},
		})
		connectionStore := newConnectionStoreFixture(t, connectionStoreFixture{
			listResult: []domain.BankConnection{existing},
		})
		coordinator, err := NewLinkCoordinator(LinkCoordinatorArgs{
			ProviderProfileRegistry: NewStaticProviderProfileRegistry(PKOProfile()),
			ConnectorRegistry:       NewStaticConnectorRegistry(),
			PendingStartStore:       newPendingStartStoreFixture(t, pendingStartStoreFixture{}).mock,
			ConnectionSecretWriter:  secretWriter.mock,
			ConnectionStore:         connectionStore.mock,
			Now:                     func() time.Time { return now },
			NewID:                   func() string { return "connection-" + fake.UUID().V4() },
			PendingStartTTL:         time.Minute,
		})
		require.NoError(t, err)

		referenceConflict, err := coordinator.saveLinkedConnection(
			t.Context(),
			"test",
			existing.TenantID,
			PKOProfile(),
			LinkResult{
				ProviderReference: "reference-conflict-" + fake.UUID().V4(),
				Secret:            "secret-plain-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
			},
		)
		require.NoError(t, err)
		connectionStore.listResult = append(connectionStore.listResult, referenceConflict)
		retry, err := coordinator.saveLinkedConnection(
			t.Context(),
			"test",
			existing.TenantID,
			PKOProfile(),
			LinkResult{
				ProviderReference: referenceConflict.ProviderReference,
				Secret:            "secret-plain-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
			},
		)
		require.NoError(t, err)

		assert.NotEqual(t, existing.ID, referenceConflict.ID)
		assert.Equal(t, referenceConflict.ID, retry.ID)
		assert.NotEqual(t, existing.SecretID, referenceConflict.SecretID)
		assert.NotEqual(t, existing.SecretID, retry.SecretID)
		assert.Equal(t, existing, connectionStore.listResult[0])
	})
}
