package finance

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	eb "github.com/gemyago/sumweave/finance/internal/enablebanking/client"
	"github.com/gemyago/sumweave/finance/internal/providers"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/google/uuid"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBankAuthorizationService(t *testing.T) {
	fake := faker.New()
	type fixture struct {
		service       *bankAuthorizationService
		store         *persistence.Store
		connection    domain.BankConnection
		start         domain.PendingBankConnectionLinkStart
		authorization domain.BankAuthorization
		publisher     *MockTransactionalSemanticCommandPublisher
		now           time.Time
	}
	makeBinding := func() domain.BankAccountBinding {
		id := fake.UUID().V4()
		return domain.BankAccountBinding{
			CanonicalID: id,
			RemoteID:    id,
			PrimaryHash: fake.UUID().V4(),
			Currency:    fake.RandomStringElement([]string{"EUR", "PLN", "USD"}),
			Selected:    true,
		}
	}
	makeFixture := func(t *testing.T) fixture {
		t.Helper()
		store := persistence.NewStore(openTestDatabase(t))
		now := time.Now().Truncate(time.Microsecond)
		key := sha256.Sum256([]byte(fake.UUID().V4()))
		cipher, err := credentials.NewAESGCMCipher(key[:], fake.UUID().V4())
		require.NoError(t, err)
		authorization := domain.BankAuthorization{
			Version:      1,
			ASPSPName:    fake.Company().Name(),
			ASPSPCountry: fake.Address().CountryCode(),
			Accounts:     []domain.BankAccountBinding{makeBinding()},
		}
		document, err := json.Marshal(authorization)
		require.NoError(t, err)
		writer := newBankConnectionSecretWriter(store, cipher, func() time.Time { return now }, uuid.NewString)
		secret, err := writer.PrepareConnectionSecret(string(domain.ProviderIDPKO), fake.UUID().V4(), string(document))
		require.NoError(t, err)
		_, err = store.SaveConnectionSecret(t.Context(), secret)
		require.NoError(t, err)
		connection, err := store.SaveBankConnection(
			t.Context(),
			domain.BankConnection{
				ID:                fake.UUID().V4(),
				TenantID:          fake.UUID().V4(),
				Provider:          string(domain.ProviderIDPKO),
				ConnectorID:       domain.ProviderConnectorIDEnableBanking,
				ProviderReference: secret.Reference,
				SecretID:          secret.ID,
				DisplayName:       fake.Company().Name(),
				State:             domain.BankConnectionStateActive,
				CreatedAt:         now.AddDate(-1, 0, 0),
				UpdatedAt:         now,
			},
		)
		require.NoError(t, err)
		due := now.Add(time.Hour)
		_, err = store.SaveBankConnectionSchedule(
			t.Context(),
			domain.BankConnectionSchedule{
				ConnectionID: connection.ID,
				Interval:     time.Hour,
				NextRunAt:    &due,
				Enabled:      true,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		)
		require.NoError(t, err)
		publisher := NewMockTransactionalSemanticCommandPublisher(t)
		service := &bankAuthorizationService{
			store:       store,
			persistence: persistence.NewBankAuthorizationStore(store),
			cipher:      cipher,
			writer:      writer,
			publisher:   publisher,
			now:         func() time.Time { return now },
			newID:       uuid.NewString,
		}
		start := domain.PendingBankConnectionLinkStart{
			ID:          fake.UUID().V4(),
			TenantID:    connection.TenantID,
			ActorUserID: fake.UUID().V4(),
			Provider:    connection.Provider,
			ConnectorID: connection.ConnectorID,
			StartResult: domain.PendingBankConnectionLinkStartResult{
				UpdateTarget: &domain.BankLinkUpdateTarget{
					ConnectionID:    connection.ID,
					ExpectedSession: connection.ProviderReference,
				},
			},
		}
		return fixture{service, store, connection, start, authorization, publisher, now}
	}
	makeCandidate := func(f fixture, add bool) providers.LinkResult {
		authorization := f.authorization
		authorization.Accounts = append([]domain.BankAccountBinding(nil), authorization.Accounts...)
		authorization.Accounts[0].RemoteID = fake.UUID().V4()
		if add {
			authorization.Accounts = append(authorization.Accounts, makeBinding())
		}
		ref := fake.UUID().V4()
		raw, err := json.Marshal(
			map[string]any{"session_id": ref, "accounts": []string{authorization.Accounts[0].RemoteID}},
		)
		require.NoError(t, err)
		return providers.LinkResult{
			ProviderReference: ref,
			State:             domain.BankConnectionStateActive,
			Authorization:     &authorization,
			ConnectionSnapshot: &domain.ProviderSnapshotObservation{
				ProviderObjectID: ref,
				DocumentJSON:     raw,
				CapturedAt:       f.now,
			},
		}
	}
	loadAuthorization := func(t *testing.T, f fixture) domain.BankAuthorization {
		t.Helper()
		c, err := f.store.GetBankConnection(t.Context(), f.connection.ID)
		require.NoError(t, err)
		secret, err := f.store.GetConnectionSecret(t.Context(), c.SecretID)
		require.NoError(t, err)
		auth, err := decodeBankAuthorization(f.service.cipher, *secret)
		require.NoError(t, err)
		require.NotNil(t, auth)
		return *auth
	}

	makeDeleteFixture := func(t *testing.T) (fixture, DeleteBankConnectionParams, *BankSyncService) {
		t.Helper()
		f := makeFixture(t)
		_, err := f.store.SaveTenantMembership(
			t.Context(),
			domain.TenantMembership{
				TenantID:  f.connection.TenantID,
				UserID:    f.start.ActorUserID,
				JoinedAt:  f.now,
				CreatedAt: f.now,
			},
		)
		require.NoError(t, err)
		service := NewBankSyncService(
			f.store,
			newMockbankSyncOrchestrator(t),
			WithBankSyncServiceSnapshotDeleter(persistence.NewProviderSnapshotStoreFromStore(f.store)),
			WithBankSyncServiceSyncStateJournalDeleter(persistence.NewProviderSyncStateJournalStore(f.store)),
		)
		return f, DeleteBankConnectionParams{
			TenantID:     f.connection.TenantID,
			ActorUserID:  f.start.ActorUserID,
			ConnectionID: f.connection.ID,
		}, service
	}

	t.Run("replacement and requested history publication commit together", func(t *testing.T) {
		f := makeFixture(t)
		candidate := makeCandidate(f, true)
		publishedID := fake.UUID().V4()
		schedule, err := f.store.GetBankConnectionSchedule(t.Context(), f.connection.ID)
		require.NoError(t, err)
		f.publisher.EXPECT().
			PublishSemanticCommandInTx(mock.Anything, mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, tx *sql.Tx, command SemanticCommand) (DispatchReference, error) {
				require.NotNil(t, tx)
				require.Equal(t, "bank-link-update:"+f.start.ID, command.IdempotencyKey)
				require.Equal(t, BankConnectionSyncCommandTopic, command.Topic)
				var input BankConnectionSyncCommand
				require.NoError(t, json.Unmarshal(command.Payload, &input))
				require.Equal(t, f.start.ActorUserID, input.Requester.UserID)
				require.Equal(t, f.connection.ID, input.ConnectionID)
				require.Nil(t, input.WindowStart)
				require.Nil(t, input.WindowEnd)
				return DispatchReference{MessageID: publishedID}, nil
			}).
			Once()
		saved, err := f.service.ReplaceLink(t.Context(), f.start, candidate)
		require.NoError(t, err)
		require.Equal(t, f.connection.ID, saved.ID)
		require.Equal(t, publishedID, saved.LastSyncJobID)
		installed, loadErr := f.store.GetBankConnection(t.Context(), saved.ID)
		require.NoError(t, loadErr)
		require.Equal(t, saved, *installed)
		require.Equal(t, f.connection.DisplayName, saved.DisplayName)
		require.Equal(t, f.connection.CreatedAt, saved.CreatedAt)
		require.Equal(t, candidate.ProviderReference, saved.ProviderReference)
		_, oldSecretErr := f.store.GetConnectionSecret(t.Context(), f.connection.SecretID)
		require.ErrorIs(t, oldSecretErr, persistence.ErrConnectionSecretNotFound)
		_, currentSecretErr := f.store.GetConnectionSecret(t.Context(), saved.SecretID)
		require.NoError(t, currentSecretErr)
		after, err := f.store.GetBankConnectionSchedule(t.Context(), saved.ID)
		require.NoError(t, err)
		require.Equal(t, schedule, after)
		auth := loadAuthorization(t, f)
		require.Equal(t, f.authorization.Accounts[0].CanonicalID, auth.Accounts[0].CanonicalID)
		floor := f.now.AddDate(-3, 0, 0)
		require.Equal(t, &floor, auth.ReplayStart())
		_, err = f.service.ReplaceLink(t.Context(), f.start, makeCandidate(f, false))
		require.ErrorIs(t, err, domain.ErrBankAuthorizationStale)
	})

	t.Run("publication failure leaves installed session secret snapshot and schedule intact", func(t *testing.T) {
		f := makeFixture(t)
		expected := errors.New(fake.UUID().V4())
		oldSecret, secretErr := f.store.GetConnectionSecret(t.Context(), f.connection.SecretID)
		require.NoError(t, secretErr)
		replacementSecretID := fake.UUID().V4()
		f.service.writer.newID = func() string { return replacementSecretID }
		f.publisher.EXPECT().
			PublishSemanticCommandInTx(mock.Anything, mock.Anything, mock.Anything).
			Return(DispatchReference{}, expected).
			Once()
		_, err := f.service.ReplaceLink(t.Context(), f.start, makeCandidate(f, true))
		require.ErrorIs(t, err, expected)
		c, err := f.store.GetBankConnection(t.Context(), f.connection.ID)
		require.NoError(t, err)
		require.Equal(t, f.connection, *c)
		snapshots, err := persistence.NewProviderSnapshotStoreFromStore(f.store).
			ListProviderSnapshotsByConnection(t.Context(), c.ID)
		require.NoError(t, err)
		require.Empty(t, snapshots)
		require.Equal(t, f.authorization, loadAuthorization(t, f))
		retained, retainErr := f.store.GetConnectionSecret(t.Context(), f.connection.SecretID)
		require.NoError(t, retainErr)
		require.Equal(t, oldSecret, retained)
		_, newSecretErr := f.store.GetConnectionSecret(t.Context(), replacementSecretID)
		require.ErrorIs(t, newSecretErr, persistence.ErrConnectionSecretNotFound)
	})

	t.Run("wrong tenant and stale expected session fail before exchange", func(t *testing.T) {
		f := makeFixture(t)
		for _, mutate := range []func(*domain.PendingBankConnectionLinkStart){
			func(s *domain.PendingBankConnectionLinkStart) { s.TenantID = fake.UUID().V4() },
			func(s *domain.PendingBankConnectionLinkStart) { s.Provider = fake.UUID().V4() },
			func(s *domain.PendingBankConnectionLinkStart) {
				s.ConnectorID = domain.ProviderConnectorID(fake.UUID().V4())
			},
		} {
			start := f.start
			mutate(&start)
			require.ErrorIs(t, f.service.ValidateUpdate(t.Context(), start), domain.ErrBankAuthorizationStale)
		}
		f.start.StartResult.UpdateTarget.ExpectedSession = fake.UUID().V4()
		require.ErrorIs(t, f.service.ValidateUpdate(t.Context(), f.start), domain.ErrBankAuthorizationStale)
	})

	t.Run(
		"only completed covering unbounded replay clears selected floors and late session cannot clear",
		func(t *testing.T) {
			f := makeFixture(t)
			floor := f.now.AddDate(-3, 0, 0)
			auth := f.authorization
			auth.Accounts[0].ReplayFrom = &floor
			inactive := makeBinding()
			inactive.Selected = false
			inactive.ReplayFrom = &floor
			auth.Accounts = append(auth.Accounts, inactive)
			document, err := json.Marshal(auth)
			require.NoError(t, err)
			envelope, err := f.service.cipher.SealString(string(document))
			require.NoError(t, err)
			require.NoError(
				t,
				f.service.persistence.WithConnection(
					t.Context(),
					bankConnectionRef(f.connection),
					func(tx *persistence.BankAuthorizationTransaction) error {
						return tx.SaveSecretEnvelope(t.Context(), envelope, f.now)
					},
				),
			)
			require.NoError(
				t,
				f.service.ClearReplay(
					t.Context(),
					f.connection,
					domain.ProviderSyncWindow{Start: floor.AddDate(0, 1, 0), End: f.now},
				),
			)
			require.Equal(t, &floor, loadAuthorization(t, f).Accounts[0].ReplayFrom)
			old := f.connection
			old.ProviderReference = fake.UUID().V4()
			require.ErrorIs(
				t,
				f.service.AdvanceReplay(t.Context(), old, domain.ProviderSyncWindow{Start: floor, End: f.now}),
				domain.ErrBankAuthorizationStale,
			)
			require.ErrorIs(
				t,
				f.service.ClearReplay(t.Context(), old, domain.ProviderSyncWindow{Start: floor, End: f.now}),
				domain.ErrBankAuthorizationStale,
			)
			require.NoError(
				t,
				f.service.ClearReplay(t.Context(), f.connection, domain.ProviderSyncWindow{Start: floor, End: f.now}),
			)
			actual := loadAuthorization(t, f)
			require.Nil(t, actual.Accounts[0].ReplayFrom)
			require.Equal(t, &floor, actual.Accounts[1].ReplayFrom)
		},
	)
	t.Run("empty legacy payload bootstraps only exact attached saved identity", func(t *testing.T) {
		f := makeFixture(t)
		binding := f.authorization.Accounts[0]
		account, err := f.store.SaveAccount(
			t.Context(),
			domain.Account{
				ID:        fake.UUID().V4(),
				TenantID:  f.connection.TenantID,
				Name:      fake.Company().Name(),
				Currency:  binding.Currency,
				Kind:      domain.AccountKindLinked,
				CreatedAt: f.now,
				UpdatedAt: f.now,
			},
		)
		require.NoError(t, err)
		_, err = f.store.SaveConnectionProviderAccount(
			t.Context(),
			domain.ConnectionProviderAccount{
				ID:                fake.UUID().V4(),
				ConnectionID:      f.connection.ID,
				ProviderAccountID: binding.CanonicalID,
				FinanceAccountID:  account.ID,
				Currency:          binding.Currency,
				Name:              account.Name,
				CreatedAt:         f.now,
				UpdatedAt:         f.now,
			},
		)
		require.NoError(t, err)
		snapshots := persistence.NewProviderSnapshotStoreFromStore(f.store)
		raw, err := json.Marshal(
			eb.SessionResponse{
				ASPSP:        eb.ASPSP{Name: f.authorization.ASPSPName, Country: f.authorization.ASPSPCountry},
				AccountsData: []eb.SessionAccount{{UID: binding.RemoteID, IdentificationHash: binding.PrimaryHash}},
			},
		)
		require.NoError(t, err)
		_, err = snapshots.SaveProviderSnapshot(
			t.Context(),
			domain.ProviderSnapshot{
				ID:               fake.UUID().V4(),
				TenantID:         f.connection.TenantID,
				ConnectionID:     f.connection.ID,
				Subject:          domain.ProviderSnapshotSubjectConnection,
				Kind:             domain.ProviderSnapshotKindConnection,
				ProviderObjectID: f.connection.ProviderReference,
				DocumentJSON:     raw,
				CapturedAt:       f.now,
			},
		)
		require.NoError(t, err)
		require.Error(
			t,
			f.service.persistence.WithConnection(
				t.Context(),
				bankConnectionRef(f.connection),
				func(tx *persistence.BankAuthorizationTransaction) error {
					_, sourceErr := tx.LoadAuthorizationBootstrap(t.Context())
					return sourceErr
				},
			),
		)
		raw, err = json.Marshal(
			eb.Account{UID: &binding.RemoteID, IdentificationHash: binding.PrimaryHash, Currency: binding.Currency},
		)
		require.NoError(t, err)
		_, err = snapshots.SaveProviderSnapshot(
			t.Context(),
			domain.ProviderSnapshot{
				ID:               fake.UUID().V4(),
				TenantID:         f.connection.TenantID,
				ConnectionID:     f.connection.ID,
				FinanceAccountID: account.ID,
				Subject:          domain.ProviderSnapshotSubjectAccount,
				Kind:             domain.ProviderSnapshotKindAccount,
				ProviderObjectID: binding.CanonicalID,
				DocumentJSON:     raw,
				CapturedAt:       f.now,
			},
		)
		require.NoError(t, err)
		empty, err := f.service.cipher.SealString("")
		require.NoError(t, err)
		require.NoError(
			t,
			f.service.persistence.WithConnection(
				t.Context(),
				bankConnectionRef(f.connection),
				func(tx *persistence.BankAuthorizationTransaction) error {
					return tx.SaveSecretEnvelope(t.Context(), empty, f.now)
				},
			),
		)
		candidate := makeCandidate(f, false)
		_, err = f.service.ReplaceLink(t.Context(), f.start, candidate)
		require.NoError(t, err)
		installed := loadAuthorization(t, f)
		require.Equal(t, binding.CanonicalID, installed.Accounts[0].CanonicalID)
		require.Equal(t, binding.PrimaryHash, installed.Accounts[0].PrimaryHash)
	})

	t.Run("legacy missing snapshots fail without replacement", func(t *testing.T) {
		f := makeFixture(t)
		empty, err := f.service.cipher.SealString("")
		require.NoError(t, err)
		require.NoError(
			t,
			f.service.persistence.WithConnection(
				t.Context(),
				bankConnectionRef(f.connection),
				func(tx *persistence.BankAuthorizationTransaction) error {
					return tx.SaveSecretEnvelope(t.Context(), empty, f.now)
				},
			),
		)
		_, err = f.service.ReplaceLink(t.Context(), f.start, makeCandidate(f, false))
		require.ErrorIs(t, err, domain.ErrBankAuthorizationInvalid)
		c, err := f.store.GetBankConnection(t.Context(), f.connection.ID)
		require.NoError(t, err)
		require.Equal(t, f.connection.ProviderReference, c.ProviderReference)
	})
	t.Run("public callback lookup rejects expired and consumed states", func(t *testing.T) {
		f := makeFixture(t)
		links := persistence.NewProviderLinkPersistence(f.store)
		service := &BankConnectionService{pendingStartLookup: links, now: func() time.Time { return f.now }}
		for _, state := range []string{"valid", "expired", "consumed"} {
			start := f.start
			start.ID = fake.UUID().V4()
			start.State = fake.UUID().V4()
			start.ExpiresAt = f.now.Add(time.Minute)
			start.CreatedAt = f.now
			start.UpdatedAt = f.now
			if state == "expired" {
				start.ExpiresAt = f.now
			}
			if state == "consumed" {
				start.ConsumedAt = &f.now
			}
			_, err := links.SavePendingStart(t.Context(), start)
			require.NoError(t, err)
			actual, err := service.GetPendingBankConnectionLinkStartByState(
				t.Context(),
				GetPendingBankConnectionLinkStartByStateParams{Provider: start.Provider, State: start.State},
			)
			if state == "valid" {
				require.NoError(t, err)
				require.Equal(t, start.StartResult.UpdateTarget, actual.StartResult.UpdateTarget)
			} else {
				require.ErrorIs(t, err, ErrPendingBankConnectionLinkStartNotFound)
			}
		}
	})

	t.Run("starting an update binds installed session and leaves it unchanged", func(t *testing.T) {
		f := makeFixture(t)
		_, err := f.store.SaveTenantMembership(
			t.Context(),
			domain.TenantMembership{
				TenantID:  f.connection.TenantID,
				UserID:    f.start.ActorUserID,
				JoinedAt:  f.now,
				CreatedAt: f.now,
			},
		)
		require.NoError(t, err)
		coordinator := newMockbankConnectionLinkCoordinator(t)
		coordinator.EXPECT().
			StartRedirectLink(mock.Anything, mock.MatchedBy(func(r providers.RedirectLinkStartRequest) bool {
				return r.UpdateTarget != nil && r.UpdateTarget.ConnectionID == f.connection.ID &&
					r.UpdateTarget.ExpectedSession == f.connection.ProviderReference
			})).
			Return(providers.StartLinkResult{State: fake.UUID().V4()}, nil).
			Once()
		service := &BankConnectionService{
			access:          newAccessGuard(f.store),
			authorization:   f.service,
			linkCoordinator: coordinator,
			logger:          slog.New(slog.DiscardHandler),
		}
		params := StartBankConnectionLinkParams{
			TenantID:     f.connection.TenantID,
			ActorUserID:  f.start.ActorUserID,
			Provider:     f.connection.Provider,
			ConnectionID: f.connection.ID,
		}
		_, err = service.StartBankConnectionLink(t.Context(), params)
		require.NoError(t, err)
		unchanged, err := f.store.GetBankConnection(t.Context(), f.connection.ID)
		require.NoError(t, err)
		require.Equal(t, f.connection, *unchanged)
		require.NoError(t, f.service.ValidateUpdate(t.Context(), f.start))
		params.Provider = fake.UUID().V4()
		_, err = service.StartBankConnectionLink(t.Context(), params)
		require.ErrorIs(t, err, ErrBankConnectionNotFound)
		params.Provider = f.connection.Provider
		params.ConnectionID = fake.UUID().V4()
		_, err = service.StartBankConnectionLink(t.Context(), params)
		require.ErrorIs(t, err, ErrBankConnectionNotFound)
		blocked := f.connection
		blocked.State = domain.BankConnectionStateDisconnected
		_, err = f.store.SaveBankConnection(t.Context(), blocked)
		require.NoError(t, err)
		params.ConnectionID = f.connection.ID
		_, err = service.StartBankConnectionLink(t.Context(), params)
		require.ErrorIs(t, err, domain.ErrBankAuthorizationStale)
		require.ErrorIs(t, f.service.ValidateUpdate(t.Context(), f.start), domain.ErrBankAuthorizationStale)
	})

	t.Run("invalid candidate and conflicting identity retain installed authorization", func(t *testing.T) {
		f := makeFixture(t)
		_, err := f.service.ReplaceLink(t.Context(), f.start, providers.LinkResult{})
		require.ErrorIs(t, err, domain.ErrBankAuthorizationInvalid)
		candidate := makeCandidate(f, false)
		candidate.ProviderReference = f.connection.ProviderReference
		_, err = f.service.ReplaceLink(t.Context(), f.start, candidate)
		require.ErrorIs(t, err, domain.ErrBankAuthorizationInvalid)
		candidate = makeCandidate(f, false)
		candidate.Authorization.ASPSPCountry = fake.UUID().V4()
		_, err = f.service.ReplaceLink(t.Context(), f.start, candidate)
		require.ErrorIs(t, err, domain.ErrBankAuthorizationInvalid)
		candidate = makeCandidate(f, false)
		candidate.ConnectionSnapshot.ProviderObjectID = ""
		_, err = f.service.ReplaceLink(t.Context(), f.start, candidate)
		require.Error(t, err)
		actual, err := f.store.GetBankConnection(t.Context(), f.connection.ID)
		require.NoError(t, err)
		require.Equal(t, f.connection, *actual)
	})

	t.Run("encrypted malformed or invalid authorization cannot be used", func(t *testing.T) {
		f := makeFixture(t)
		secret, err := f.store.GetConnectionSecret(t.Context(), f.connection.SecretID)
		require.NoError(t, err)
		for _, document := range []string{fake.UUID().V4(), "{}"} {
			secret.Envelope, err = f.service.cipher.SealString(document)
			require.NoError(t, err)
			_, err = decodeBankAuthorization(f.service.cipher, *secret)
			require.Error(t, err)
		}
		secret.Envelope.Ciphertext = fake.UUID().V4()
		_, err = decodeBankAuthorization(f.service.cipher, *secret)
		require.Error(t, err)
	})

	t.Run("bounded and interrupted sync retain replay until full unbounded success", func(t *testing.T) {
		for _, mode := range []string{"bounded", "complete", "committed then interrupted"} {
			t.Run(mode, func(t *testing.T) {
				f := makeFixture(t)
				floor := f.now.AddDate(-3, 0, 0)
				auth := f.authorization
				auth.Accounts[0].ReplayFrom = &floor
				document, err := json.Marshal(auth)
				require.NoError(t, err)
				envelope, err := f.service.cipher.SealString(string(document))
				require.NoError(t, err)
				require.NoError(
					t,
					f.service.persistence.WithConnection(
						t.Context(),
						bankConnectionRef(f.connection),
						func(tx *persistence.BankAuthorizationTransaction) error {
							return tx.SaveSecretEnvelope(t.Context(), envelope, f.now)
						},
					),
				)
				orchestrator := newMockbankSyncOrchestrator(t)
				service := NewBankSyncService(
					f.store,
					orchestrator,
					WithBankSyncServiceNow(func() time.Time { return f.now }),
				)
				service.authorization = f.service
				params := RunBankConnectionSyncParams{ConnectionID: f.connection.ID, JobID: fake.UUID().V4()}
				if mode == "bounded" {
					start := f.now.AddDate(0, 0, -30)
					params.WindowStart = &start
				}
				var expectedErr error
				if mode == "committed then interrupted" {
					expectedErr = errors.New(fake.UUID().V4())
				}
				orchestrator.EXPECT().
					Orchestrate(mock.Anything, mock.MatchedBy(func(r providers.SyncOrchestrationRequest) bool {
						return r.ReplayFrom != nil && r.ReplayFrom.Equal(floor)
					})).
					RunAndReturn(func(ctx context.Context, request providers.SyncOrchestrationRequest) (providers.SyncOrchestrationResult, error) {
						require.Equal(t, mode == "bounded", request.OnWindowCommitted == nil)
						if mode == "committed then interrupted" {
							// An out-of-order chunk cannot jump over an uncommitted gap.
							require.NoError(
								t,
								request.OnWindowCommitted(
									ctx,
									domain.ProviderSyncWindow{
										Start: floor.AddDate(0, 0, 30),
										End:   floor.AddDate(0, 0, 60),
									},
								),
							)
							require.Equal(t, &floor, loadAuthorization(t, f).ReplayStart())
							require.NoError(
								t,
								request.OnWindowCommitted(
									ctx,
									domain.ProviderSyncWindow{Start: floor, End: floor.AddDate(0, 0, 30)},
								),
							)
						}
						return providers.SyncOrchestrationResult{
							TargetWindow: domain.ProviderSyncWindow{Start: floor, End: f.now},
						}, expectedErr
					}).
					Once()
				_, err = service.RunBankConnectionSync(t.Context(), params)
				if expectedErr != nil {
					require.ErrorIs(t, err, expectedErr)
				} else {
					require.NoError(t, err)
				}
				installed := loadAuthorization(t, f)
				switch mode {
				case "committed then interrupted":
					resumed := floor.AddDate(0, 0, 30)
					require.Equal(t, &resumed, installed.ReplayStart())
					orchestrator.EXPECT().
						Orchestrate(mock.Anything, mock.MatchedBy(func(r providers.SyncOrchestrationRequest) bool {
							return r.ReplayFrom != nil && r.ReplayFrom.Equal(resumed)
						})).
						RunAndReturn(func(ctx context.Context, request providers.SyncOrchestrationRequest) (providers.SyncOrchestrationResult, error) {
							window := domain.ProviderSyncWindow{Start: resumed, End: f.now}
							require.NoError(t, request.OnWindowCommitted(ctx, window))
							return providers.SyncOrchestrationResult{TargetWindow: window}, nil
						}).
						Once()
					_, err = service.RunBankConnectionSync(t.Context(), params)
					require.NoError(t, err)
					require.Nil(t, loadAuthorization(t, f).ReplayStart())
				case "complete":
					require.Nil(t, installed.ReplayStart())
				default:
					require.Equal(t, &floor, installed.ReplayStart())
				}
			})
		}
	})

	t.Run("saved source parsing rejects missing conflicting and wrong attachments", func(t *testing.T) {
		f := makeFixture(t)
		binding := f.authorization.Accounts[0]
		accountID := fake.UUID().V4()
		mapping := domain.ConnectionProviderAccount{
			ProviderAccountID: binding.CanonicalID,
			FinanceAccountID:  accountID,
			Currency:          binding.Currency,
		}
		accountSource := func(document []byte) domain.ProviderSnapshot {
			return domain.ProviderSnapshot{
				TenantID:         f.connection.TenantID,
				Subject:          domain.ProviderSnapshotSubjectAccount,
				Kind:             domain.ProviderSnapshotKindAccount,
				FinanceAccountID: accountID,
				ProviderObjectID: binding.CanonicalID,
				DocumentJSON:     document,
			}
		}
		sessionSource := func(document []byte) domain.ProviderSnapshot {
			return domain.ProviderSnapshot{
				TenantID:         f.connection.TenantID,
				Subject:          domain.ProviderSnapshotSubjectConnection,
				Kind:             domain.ProviderSnapshotKindConnection,
				ProviderObjectID: f.connection.ProviderReference,
				DocumentJSON:     document,
			}
		}
		_, err := savedAccountBinding(f.connection, mapping, nil)
		require.ErrorIs(t, err, domain.ErrBankAuthorizationInvalid)
		raw, err := json.Marshal(
			eb.Account{UID: &binding.RemoteID, Currency: binding.Currency, IdentificationHash: binding.PrimaryHash},
		)
		require.NoError(t, err)
		wrong := accountSource(raw)
		wrong.FinanceAccountID = fake.UUID().V4()
		_, err = savedAccountBinding(f.connection, mapping, []domain.ProviderSnapshot{wrong})
		require.ErrorIs(t, err, domain.ErrBankAuthorizationInvalid)
		for _, document := range [][]byte{[]byte(fake.UUID().V4()), []byte("{}")} {
			_, err = savedAccountBinding(f.connection, mapping, []domain.ProviderSnapshot{accountSource(document)})
			require.Error(t, err)
		}
		_, err = savedSessionIdentity(f.connection, nil)
		require.ErrorIs(t, err, domain.ErrBankAuthorizationInvalid)
		_, err = savedSessionIdentity(f.connection, []domain.ProviderSnapshot{sessionSource([]byte(fake.UUID().V4()))})
		require.Error(t, err)
	})

	t.Run("another update requeues unfinished replay after fencing old in-flight work", func(t *testing.T) {
		f := makeFixture(t)
		floor := f.now.AddDate(-3, 0, 0)
		auth := f.authorization
		auth.Accounts[0].ReplayFrom = &floor
		document, err := json.Marshal(auth)
		require.NoError(t, err)
		envelope, err := f.service.cipher.SealString(string(document))
		require.NoError(t, err)
		require.NoError(
			t,
			f.service.persistence.WithConnection(
				t.Context(),
				bankConnectionRef(f.connection),
				func(tx *persistence.BankAuthorizationTransaction) error {
					return tx.SaveSecretEnvelope(t.Context(), envelope, f.now)
				},
			),
		)
		f.publisher.EXPECT().
			PublishSemanticCommandInTx(mock.Anything, mock.Anything, mock.Anything).
			Return(DispatchReference{MessageID: fake.UUID().V4()}, nil).
			Once()
		_, err = f.service.ReplaceLink(t.Context(), f.start, makeCandidate(f, false))
		require.NoError(t, err)
		require.Equal(t, &floor, loadAuthorization(t, f).ReplayStart())
	})

	t.Run("authoritative replacement rejects a wrong tenant and unavailable installed secret", func(t *testing.T) {
		f := makeFixture(t)
		candidate := makeCandidate(f, false)
		wrong := f.start
		wrong.TenantID = fake.UUID().V4()
		_, err := f.service.ReplaceLink(t.Context(), wrong, candidate)
		require.ErrorIs(t, err, domain.ErrBankAuthorizationStale)
		missing := f.start
		missing.StartResult.UpdateTarget = &domain.BankLinkUpdateTarget{
			ConnectionID:    fake.UUID().V4(),
			ExpectedSession: f.connection.ProviderReference,
		}
		require.Error(t, f.service.ValidateUpdate(t.Context(), missing))
		envelope, err := f.service.cipher.SealString(fake.UUID().V4())
		require.NoError(t, err)
		require.NoError(
			t,
			f.service.persistence.WithConnection(
				t.Context(),
				bankConnectionRef(f.connection),
				func(tx *persistence.BankAuthorizationTransaction) error {
					return tx.SaveSecretEnvelope(t.Context(), envelope, f.now)
				},
			),
		)
		_, err = f.service.ReplaceLink(t.Context(), f.start, candidate)
		require.Error(t, err)
		installed := f.connection
		installed.SecretID = fake.UUID().V4()
		_, err = f.store.SaveBankConnection(t.Context(), installed)
		require.NoError(t, err)
		_, err = f.service.ReplaceLink(t.Context(), f.start, candidate)
		require.ErrorIs(t, err, persistence.ErrConnectionSecretNotFound)
		installed.State = domain.BankConnectionStateReauthRequired
		_, err = f.store.SaveBankConnection(t.Context(), installed)
		require.NoError(t, err)
		require.ErrorIs(
			t,
			f.service.ClearReplay(
				t.Context(),
				installed,
				domain.ProviderSyncWindow{Start: f.now.AddDate(-3, 0, 0), End: f.now},
			),
			domain.ErrBankReauthRequired,
		)
	})

	t.Run("duplicate replacement secret cannot overwrite installed credentials", func(t *testing.T) {
		f := makeFixture(t)
		f.service.writer.newID = func() string { return f.connection.SecretID }
		_, err := f.service.ReplaceLink(t.Context(), f.start, makeCandidate(f, false))
		require.Error(t, err)
		installed, err := f.store.GetBankConnection(t.Context(), f.connection.ID)
		require.NoError(t, err)
		require.Equal(t, f.connection, *installed)
	})
	t.Run("repeated updates and connection deletion remove only their exact owned secrets", func(t *testing.T) {
		f, other := makeFixture(t), makeFixture(t)
		unrelated, err := other.store.GetConnectionSecret(t.Context(), other.connection.SecretID)
		require.NoError(t, err)
		unrelated.Reference = f.connection.ProviderReference
		other.connection.ProviderReference = f.connection.ProviderReference
		_, saveConnectionErr := other.store.SaveBankConnection(t.Context(), other.connection)
		require.NoError(t, saveConnectionErr)
		_, err = other.store.SaveConnectionSecret(t.Context(), *unrelated)
		require.NoError(t, err)
		retiredIDs := []string{f.connection.SecretID}
		for range 2 {
			installed, replaceErr := f.service.ReplaceLink(t.Context(), f.start, makeCandidate(f, false))
			require.NoError(t, replaceErr)
			for _, id := range retiredIDs {
				_, lookupErr := f.store.GetConnectionSecret(t.Context(), id)
				require.ErrorIs(t, lookupErr, persistence.ErrConnectionSecretNotFound)
			}
			f.connection = installed
			f.start.ID = fake.UUID().V4()
			f.start.StartResult.UpdateTarget = &domain.BankLinkUpdateTarget{
				ConnectionID:    installed.ID,
				ExpectedSession: installed.ProviderReference,
			}
			retiredIDs = append(retiredIDs, installed.SecretID)
		}
		_, err = f.store.SaveTenantMembership(
			t.Context(),
			domain.TenantMembership{
				TenantID:  f.connection.TenantID,
				UserID:    f.start.ActorUserID,
				JoinedAt:  f.now,
				CreatedAt: f.now,
			},
		)
		require.NoError(t, err)
		syncService := NewBankSyncService(
			f.store,
			newMockbankSyncOrchestrator(t),
			WithBankSyncServiceSnapshotDeleter(persistence.NewProviderSnapshotStoreFromStore(f.store)),
			WithBankSyncServiceSyncStateJournalDeleter(persistence.NewProviderSyncStateJournalStore(f.store)),
		)
		require.NoError(
			t,
			syncService.DeleteBankConnection(
				t.Context(),
				DeleteBankConnectionParams{
					TenantID:     f.connection.TenantID,
					ActorUserID:  f.start.ActorUserID,
					ConnectionID: f.connection.ID,
				},
			),
		)
		for _, id := range retiredIDs {
			_, lookupErr := f.store.GetConnectionSecret(t.Context(), id)
			require.ErrorIs(t, lookupErr, persistence.ErrConnectionSecretNotFound)
		}
		_, err = f.store.GetBankConnection(t.Context(), f.connection.ID)
		require.ErrorIs(t, err, persistence.ErrBankConnectionNotFound)
		retained, err := other.store.GetConnectionSecret(t.Context(), unrelated.ID)
		require.NoError(t, err)
		require.Equal(t, unrelated, retained)
		otherConnection, err := other.store.GetBankConnection(t.Context(), other.connection.ID)
		require.NoError(t, err)
		require.Equal(t, other.connection, *otherConnection)
	})

	t.Run("secret retirement failure rolls back the replacement", func(t *testing.T) {
		f := makeFixture(t)
		oldSecret, err := f.store.GetConnectionSecret(t.Context(), f.connection.SecretID)
		require.NoError(t, err)
		replacementSecretID := fake.UUID().V4()
		f.service.writer.newID = func() string { return replacementSecretID }
		expectedErr := errors.New(fake.UUID().V4())
		callbackName := "reject-secret-retirement-" + fake.UUID().V4()
		require.NoError(
			t,
			f.store.DB().Callback().Delete().Before("gorm:delete").Register(callbackName, func(tx *gorm.DB) {
				if tx.Statement.Table == "finance_connection_secrets" {
					tx.AddError(expectedErr)
				}
			}),
		)
		t.Cleanup(func() { require.NoError(t, f.store.DB().Callback().Delete().Remove(callbackName)) })
		_, err = f.service.ReplaceLink(t.Context(), f.start, makeCandidate(f, false))
		require.ErrorIs(t, err, expectedErr)
		installed, err := f.store.GetBankConnection(t.Context(), f.connection.ID)
		require.NoError(t, err)
		require.Equal(t, f.connection, *installed)
		retained, err := f.store.GetConnectionSecret(t.Context(), oldSecret.ID)
		require.NoError(t, err)
		require.Equal(t, oldSecret, retained)
		_, err = f.store.GetConnectionSecret(t.Context(), replacementSecretID)
		require.ErrorIs(t, err, persistence.ErrConnectionSecretNotFound)
		snapshots, err := persistence.NewProviderSnapshotStoreFromStore(f.store).
			ListProviderSnapshotsByConnection(t.Context(), f.connection.ID)
		require.NoError(t, err)
		require.Empty(t, snapshots)
	})
	t.Run(
		"deletion cleans the actual current secret after replacement commits between lookup and delete",
		func(t *testing.T) {
			f := makeFixture(t)
			_, err := f.store.SaveTenantMembership(
				t.Context(),
				domain.TenantMembership{
					TenantID:  f.connection.TenantID,
					UserID:    f.start.ActorUserID,
					JoinedAt:  f.now,
					CreatedAt: f.now,
				},
			)
			require.NoError(t, err)
			var replacement domain.BankConnection
			updated := false
			callbackName := "replace-after-delete-lookup-" + fake.UUID().V4()
			require.NoError(
				t,
				f.store.DB().Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
					if tx.Statement.Table != "finance_bank_connections" || updated {
						return
					}
					updated = true
					var replaceErr error
					replacement, replaceErr = f.service.ReplaceLink(t.Context(), f.start, makeCandidate(f, false))
					require.NoError(t, replaceErr)
				}),
			)
			t.Cleanup(func() { require.NoError(t, f.store.DB().Callback().Query().Remove(callbackName)) })
			syncService := NewBankSyncService(
				f.store,
				newMockbankSyncOrchestrator(t),
				WithBankSyncServiceSnapshotDeleter(persistence.NewProviderSnapshotStoreFromStore(f.store)),
				WithBankSyncServiceSyncStateJournalDeleter(persistence.NewProviderSyncStateJournalStore(f.store)),
			)
			require.NoError(
				t,
				syncService.DeleteBankConnection(
					t.Context(),
					DeleteBankConnectionParams{
						TenantID:     f.connection.TenantID,
						ActorUserID:  f.start.ActorUserID,
						ConnectionID: f.connection.ID,
					},
				),
			)
			require.True(t, updated)
			_, err = f.store.GetBankConnection(t.Context(), f.connection.ID)
			require.ErrorIs(t, err, persistence.ErrBankConnectionNotFound)
			_, err = f.store.GetConnectionSecret(t.Context(), replacement.SecretID)
			require.ErrorIs(t, err, persistence.ErrConnectionSecretNotFound)
			_, err = f.store.GetConnectionSecret(t.Context(), f.connection.SecretID)
			require.ErrorIs(t, err, persistence.ErrConnectionSecretNotFound)
		},
	)
	t.Run("already completed deletion succeeds without stale cleanup after authorized lookup", func(t *testing.T) {
		f, params, winner := makeDeleteFixture(t)
		staleCleanup := newMockproviderSnapshotConnectionDeleter(t)
		service := NewBankSyncService(
			f.store,
			newMockbankSyncOrchestrator(t),
			WithBankSyncServiceSnapshotDeleter(staleCleanup),
		)
		completed := false
		callbackName := "complete-other-delete-after-lookup-" + fake.UUID().V4()
		require.NoError(
			t,
			f.store.DB().Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
				if tx.Statement.Table != "finance_bank_connections" || completed {
					return
				}
				completed = true
				require.NoError(t, winner.DeleteBankConnection(t.Context(), params))
			}),
		)
		t.Cleanup(func() { require.NoError(t, f.store.DB().Callback().Query().Remove(callbackName)) })
		require.NoError(t, service.DeleteBankConnection(t.Context(), params))
		require.True(t, completed)
		staleCleanup.AssertNotCalled(t, "DeleteProviderSnapshotsByConnection", mock.Anything, f.connection.ID)
		_, err := f.store.GetBankConnection(t.Context(), f.connection.ID)
		require.ErrorIs(t, err, persistence.ErrBankConnectionNotFound)
		_, err = f.store.GetConnectionSecret(t.Context(), f.connection.SecretID)
		require.ErrorIs(t, err, persistence.ErrConnectionSecretNotFound)
	})

	t.Run("deletion preserves initial access checks and real returning delete errors", func(t *testing.T) {
		f, params, service := makeDeleteFixture(t)
		missing := params
		missing.ConnectionID = fake.UUID().V4()
		require.ErrorIs(t, service.DeleteBankConnection(t.Context(), missing), ErrBankConnectionNotFound)
		unauthorized := params
		unauthorized.ActorUserID = fake.UUID().V4()
		require.ErrorIs(t, service.DeleteBankConnection(t.Context(), unauthorized), ErrTenantAccessDenied)
		expectedErr := errors.New(fake.UUID().V4())
		callbackName := "fail-returning-delete-" + fake.UUID().V4()
		require.NoError(
			t,
			f.store.DB().Callback().Delete().Before("gorm:delete").Register(callbackName, func(tx *gorm.DB) {
				if tx.Statement.Table == "finance_bank_connections" {
					tx.AddError(expectedErr)
				}
			}),
		)
		t.Cleanup(func() { require.NoError(t, f.store.DB().Callback().Delete().Remove(callbackName)) })
		require.ErrorIs(t, service.DeleteBankConnection(t.Context(), params), expectedErr)
		retained, err := f.store.GetBankConnection(t.Context(), f.connection.ID)
		require.NoError(t, err)
		require.Equal(t, f.connection, *retained)
		_, err = f.store.GetConnectionSecret(t.Context(), f.connection.SecretID)
		require.NoError(t, err)
	})
}
