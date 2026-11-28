package finance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	internalproviders "github.com/gemyago/sumweave/finance/internal/providers"
	"github.com/gemyago/sumweave/finance/persistence"
)

type bankSyncFocusedStore interface {
	IsTenantMember(ctx context.Context, tenantID string, userID string) (bool, error)
	bankSyncStore
	connectionSecretStore
}

type providerSnapshotConnectionDeleter interface {
	DeleteProviderSnapshotsByConnection(context.Context, string) error
}

type providerSyncStateJournalConnectionDeleter interface {
	DeleteSyncStatesByConnection(context.Context, string) error
}

// bankSyncOrchestrator is the focused execution dependency for one durable
// bank-connection sync attempt.
type bankSyncOrchestrator interface {
	Orchestrate(
		ctx context.Context,
		request internalproviders.SyncOrchestrationRequest,
	) (internalproviders.SyncOrchestrationResult, error)
}

type BankSyncService struct {
	projections             *persistence.BankAuthorizationStore
	authorization           *bankAuthorizationService
	store                   bankSyncFocusedStore
	access                  *accessGuard
	now                     func() time.Time
	syncOrchestrator        bankSyncOrchestrator
	commandPublisher        SemanticCommandPublisher
	logger                  *slog.Logger
	snapshotDeleter         providerSnapshotConnectionDeleter
	syncStateJournalDeleter providerSyncStateJournalConnectionDeleter
}

type BankSyncServiceOption func(*BankSyncService)

func WithBankSyncServiceNow(now func() time.Time) BankSyncServiceOption {
	return func(service *BankSyncService) { service.now = now }
}

func WithBankSyncServiceCommandPublisher(publisher SemanticCommandPublisher) BankSyncServiceOption {
	return func(service *BankSyncService) { service.commandPublisher = publisher }
}

func WithBankSyncServiceLogger(logger *slog.Logger) BankSyncServiceOption {
	return func(service *BankSyncService) {
		if logger != nil {
			service.logger = logger
		}
	}
}

func WithBankSyncServiceSnapshotDeleter(
	deleter providerSnapshotConnectionDeleter,
) BankSyncServiceOption {
	return func(service *BankSyncService) { service.snapshotDeleter = deleter }
}

func WithBankSyncServiceSyncStateJournalDeleter(
	deleter providerSyncStateJournalConnectionDeleter,
) BankSyncServiceOption {
	return func(service *BankSyncService) { service.syncStateJournalDeleter = deleter }
}

func NewBankSyncService(
	store *persistence.Store,
	syncOrchestrator bankSyncOrchestrator,
	opts ...BankSyncServiceOption,
) *BankSyncService {
	if syncOrchestrator == nil {
		panic("bank sync orchestrator is required")
	}
	service := &BankSyncService{
		projections:      persistence.NewBankAuthorizationStore(store),
		store:            store,
		access:           newAccessGuard(store),
		now:              time.Now,
		syncOrchestrator: syncOrchestrator,
		logger:           slog.New(slog.DiscardHandler),
	}
	for _, opt := range opts {
		opt(service)
	}
	return service
}

func (s *BankSyncService) UpsertBankConnectionSchedule(
	ctx context.Context,
	params UpsertBankConnectionScheduleParams,
) (domain.BankConnectionSchedule, error) {
	connection, err := s.requireTenantBankConnection(ctx, params.TenantID, params.ActorUserID, params.ConnectionID)
	if err != nil { // coverage-ignore // Access failures are covered by access-guard tests.
		return domain.BankConnectionSchedule{}, err
	}
	now := s.now()
	existing, err := s.store.GetBankConnectionSchedule(ctx, connection.ID)
	if err != nil &&
		!errors.Is(err, persistence.ErrBankConnectionScheduleNotFound) { // coverage-ignore
		return domain.BankConnectionSchedule{}, fmt.Errorf("upsert bank connection schedule: %w", err)
	}
	schedule := domain.BankConnectionSchedule{
		ConnectionID: connection.ID,
		Interval:     params.Interval,
		NextRunAt:    timePtrOrNil(params.NextRunAt),
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if existing != nil {
		schedule.CreatedAt = existing.CreatedAt
		schedule.LastScheduledAt = existing.LastScheduledAt
		schedule.LastStartedAt = existing.LastStartedAt
		schedule.LastCompletedAt = existing.LastCompletedAt
		schedule.LastJobID = existing.LastJobID
	}
	persisted, err := s.store.SaveBankConnectionSchedule(ctx, schedule)
	if err != nil { // coverage-ignore // Store errors are covered by persistence tests.
		return domain.BankConnectionSchedule{}, err
	}
	return persisted, nil
}

func (s *BankSyncService) PauseBankConnectionSchedule(
	ctx context.Context,
	params PauseBankConnectionScheduleParams,
) (domain.BankConnectionSchedule, error) {
	connection, err := s.requireTenantBankConnection(ctx, params.TenantID, params.ActorUserID, params.ConnectionID)
	if err != nil { // coverage-ignore // Access failures are covered by access-guard tests.
		return domain.BankConnectionSchedule{}, err
	}
	schedule, err := s.store.GetBankConnectionSchedule(ctx, connection.ID)
	if err != nil { // coverage-ignore // Store errors are covered by persistence tests.
		return domain.BankConnectionSchedule{}, fmt.Errorf("pause bank connection schedule: %w", err)
	}
	schedule.Enabled = false
	schedule.NextRunAt = nil
	schedule.UpdatedAt = s.now()
	persisted, err := s.store.SaveBankConnectionSchedule(ctx, *schedule)
	if err != nil { // coverage-ignore // Store errors are covered by persistence tests.
		return domain.BankConnectionSchedule{}, err
	}
	return persisted, nil
}

func (s *BankSyncService) ResumeBankConnectionSchedule(
	ctx context.Context,
	params ResumeBankConnectionScheduleParams,
) (domain.BankConnectionSchedule, error) {
	connection, err := s.requireTenantBankConnection(ctx, params.TenantID, params.ActorUserID, params.ConnectionID)
	if err != nil { // coverage-ignore // Access failures are covered by access-guard tests.
		return domain.BankConnectionSchedule{}, err
	}
	schedule, err := s.store.GetBankConnectionSchedule(ctx, connection.ID)
	if err != nil { // coverage-ignore // Store errors are covered by persistence tests.
		return domain.BankConnectionSchedule{}, fmt.Errorf("resume bank connection schedule: %w", err)
	}
	schedule.Enabled = true
	schedule.NextRunAt = timePtrOrNil(params.NextRunAt)
	schedule.UpdatedAt = s.now()
	persisted, err := s.store.SaveBankConnectionSchedule(ctx, *schedule)
	if err != nil { // coverage-ignore // Store errors are covered by persistence tests.
		return domain.BankConnectionSchedule{}, err
	}
	return persisted, nil
}

func (s *BankSyncService) TriggerBankConnectionSync(
	ctx context.Context,
	params TriggerBankConnectionSyncParams,
) (BankConnectionSyncJobRef, error) {
	if err := validateHTTPCommandRequesterSource(params.RequesterSource); err != nil {
		return BankConnectionSyncJobRef{}, err
	}
	if err := validateBankConnectionSyncWindows(params.WindowStart, params.WindowEnd); err != nil {
		return BankConnectionSyncJobRef{}, err
	}
	if s.commandPublisher == nil {
		return BankConnectionSyncJobRef{}, errors.New("bank sync command publisher is required")
	}
	connection, err := s.requireTenantBankConnection(ctx, params.TenantID, params.ActorUserID, params.ConnectionID)
	if err != nil {
		return BankConnectionSyncJobRef{}, err
	}
	if connection.State != domain.BankConnectionStateActive {
		return BankConnectionSyncJobRef{}, domain.ErrBankReauthRequired
	}
	command, commandErr := newSemanticCommand(BankConnectionSyncCommandTopic, BankConnectionSyncCommand{
		ConnectionID: connection.ID,
		Reason:       strings.TrimSpace(params.Reason),
		WindowStart:  params.WindowStart,
		WindowEnd:    params.WindowEnd,
		Requester: CommandRequester{
			UserID: strings.TrimSpace(params.ActorUserID),
			Source: params.RequesterSource,
		},
	}, params.IdempotencyKey)
	if commandErr != nil { // coverage-ignore // JSON encoding of this concrete finance command cannot fail.
		return BankConnectionSyncJobRef{}, commandErr
	}
	reference, err := s.commandPublisher.PublishSemanticCommand(ctx, command)
	if err != nil {
		return BankConnectionSyncJobRef{}, fmt.Errorf("publish bank connection sync command: %w", err)
	}
	s.logger.InfoContext(ctx, "bank connection sync command published",
		slog.String("messageId", reference.MessageID),
		slog.String("connectionId", connection.ID),
		slog.String("reason", params.Reason),
	)
	return BankConnectionSyncJobRef{ID: reference.MessageID, JobType: BankConnectionSyncJobType}, nil
}

func (s *BankSyncService) DeleteBankConnection(
	ctx context.Context,
	params DeleteBankConnectionParams,
) error {
	connection, err := s.requireTenantBankConnection(ctx, params.TenantID, params.ActorUserID, params.ConnectionID)
	if err != nil { // coverage-ignore // Tenant access validation is covered by access-guard tests.
		return err
	}
	deleteConnection := func(service *BankSyncService) error {
		return service.deleteBankConnectionOwnedMetadata(ctx, connection)
	}
	if txStore, ok := s.store.(*persistence.Store); ok {
		if txErr := txStore.WithTransaction(ctx, func(store *persistence.Store) error {
			txService := *s
			txService.store = store
			if _, snapshotStore := s.snapshotDeleter.(*persistence.ProviderSnapshotStore); snapshotStore {
				txService.snapshotDeleter = persistence.NewProviderSnapshotStoreFromStore(store)
			}
			if _, journalStore := s.syncStateJournalDeleter.(*persistence.ProviderSyncStateJournalStore); journalStore {
				txService.syncStateJournalDeleter = persistence.NewProviderSyncStateJournalStore(store)
			}
			return deleteConnection(&txService)
		}); txErr != nil {
			return txErr
		}
		return nil
	}
	return deleteConnection(s) // coverage-ignore // Production persistence always uses its transaction-capable store.
}

func (s *BankSyncService) RunBankConnectionSync(
	ctx context.Context,
	params RunBankConnectionSyncParams,
) (BankConnectionSyncResult, error) {
	validationErr := validateBankConnectionSyncWindows(params.WindowStart, params.WindowEnd)
	if validationErr != nil { // coverage-ignore // Request validation is covered by focused service tests.
		return BankConnectionSyncResult{}, NewTerminalFailure(
			validationErr,
			"bank_sync_invalid_request",
			"Bank sync request is invalid",
			"The bank sync command contains an invalid time window.",
		)
	}
	connection, err := s.store.GetBankConnection(ctx, strings.TrimSpace(params.ConnectionID))
	if err != nil { // coverage-ignore // Connection lookup errors are covered by persistence tests.
		return BankConnectionSyncResult{}, terminalBankConnectionSyncFailure(err)
	}
	if connection.State != domain.BankConnectionStateActive {
		return BankConnectionSyncResult{}, terminalBankConnectionSyncFailure(domain.ErrBankReauthRequired)
	}
	secret, err := s.store.GetConnectionSecret(ctx, connection.SecretID)
	if err != nil { // coverage-ignore // Secret-store errors are covered by persistence tests.
		return BankConnectionSyncResult{}, terminalBankConnectionSyncFailure(
			fmt.Errorf("get connection secret: %w", err),
		)
	}
	now := s.now()
	scheduledRun, hasScheduledRun, err := s.makeScheduledRunMetadata(ctx, *connection, params, now)
	if err != nil { // coverage-ignore // Schedule metadata errors are covered by focused service tests.
		return BankConnectionSyncResult{}, err
	}
	markErr := s.markBankConnectionSyncStarted(ctx, connection, params, now, scheduledRun)
	if markErr != nil { // coverage-ignore // Projection-store errors are covered by persistence tests.
		return BankConnectionSyncResult{}, markErr
	}
	var replayFrom *time.Time
	if s.authorization != nil && connection.ConnectorID == domain.ProviderConnectorIDEnableBanking {
		authorization, decodeErr := decodeBankAuthorization(s.authorization.cipher, *secret)
		if decodeErr != nil {
			return BankConnectionSyncResult{}, decodeErr
		}
		if authorization != nil {
			replayFrom = authorization.ReplayStart()
		}
	}
	result, err := s.syncOrchestrator.Orchestrate(ctx, internalproviders.SyncOrchestrationRequest{
		OnWindowCommitted: s.replayProgressRecorder(*connection, params, replayFrom),
		Connection: domain.ProviderConnectionRef{
			ConnectionID:      connection.ID,
			ProviderID:        domain.ProviderID(connection.Provider),
			ConnectorID:       connection.ConnectorID,
			ProviderReference: connection.ProviderReference,
		},
		ReplayFrom:  replayFrom,
		Secret:      *secret,
		JobID:       params.JobID,
		Reason:      params.Reason,
		WindowStart: params.WindowStart,
		WindowEnd:   params.WindowEnd,
	})
	if err != nil {
		failureErr := s.recordBankConnectionSyncFailure(
			ctx,
			connection,
			params,
			now,
			scheduledRun,
			err,
		)
		return BankConnectionSyncResult{}, terminalBankConnectionSyncFailure(failureErr)
	}
	if !hasScheduledRun {
		scheduledRun = nil
	}
	if s.authorization != nil && params.WindowStart == nil && params.WindowEnd == nil && replayFrom != nil {
		if clearErr := s.authorization.ClearReplay(ctx, *connection, result.TargetWindow); clearErr != nil {
			return BankConnectionSyncResult{}, terminalBankConnectionSyncFailure(clearErr)
		}
	}
	err = s.completeBankConnectionSync(ctx, connection, params.JobID, scheduledRun, s.now())
	if err != nil { // coverage-ignore // Completion projection errors are covered by persistence tests.
		failureErr := s.recordBankConnectionSyncFailure(
			ctx,
			connection,
			params,
			now,
			scheduledRun,
			err,
		)
		return BankConnectionSyncResult{}, terminalBankConnectionSyncFailure(failureErr)
	}
	return BankConnectionSyncResult{
		ImportedAccounts:     result.Stats.CreatedAccounts,
		ImportedTransactions: result.Stats.CreatedTransactions,
		UpdatedTransactions:  result.Stats.UpdatedTransactions,
	}, nil
}

func (s *BankSyncService) replayProgressRecorder(
	connection domain.BankConnection,
	params RunBankConnectionSyncParams,
	replayFrom *time.Time,
) func(context.Context, domain.ProviderSyncWindow) error {
	if replayFrom == nil || params.WindowStart != nil || params.WindowEnd != nil {
		return nil
	}
	return func(ctx context.Context, window domain.ProviderSyncWindow) error {
		return s.authorization.AdvanceReplay(ctx, connection, window)
	}
}

func terminalBankConnectionSyncFailure(err error) error {
	if errors.Is(err, domain.ErrBankReauthRequired) {
		return NewTerminalFailure(
			err,
			"bank_reauth_required",
			"Update bank link",
			"The bank session is closed or expired. Update the bank link to continue syncing.",
		)
	}
	if errors.Is(err, domain.ErrBankAuthorizationStale) {
		return NewTerminalFailure(
			err,
			"bank_authorization_changed",
			"Bank link changed",
			"This sync used an older bank link. Start a new sync.",
		)
	}
	if errors.Is(err, domain.ErrBankAuthorizationInvalid) {
		return NewTerminalFailure(
			err,
			"bank_identity_invalid",
			"Bank account identity is unavailable",
			"Update the bank link to verify account identity.",
		)
	}
	if errors.Is(err, persistence.ErrBankConnectionNotFound) {
		return NewTerminalFailure(
			fmt.Errorf("%w: %w", ErrBankConnectionNotFound, err),
			"bank_connection_not_found",
			"Bank connection not found",
			"The bank connection no longer exists.",
		)
	}
	if errors.Is(err, ErrBankConnectionNotFound) {
		return NewTerminalFailure(
			err,
			"bank_connection_not_found",
			"Bank connection not found",
			"The bank connection no longer exists.",
		)
	}
	if errors.Is(err, persistence.ErrConnectionSecretNotFound) {
		return NewTerminalFailure(
			err,
			"bank_connection_credentials_missing",
			"Bank connection credentials are unavailable",
			"The bank connection credentials are no longer available.",
		)
	}
	if errors.Is(err, internalproviders.ErrConnectorIDRequired) ||
		errors.Is(err, internalproviders.ErrConnectorNotConfigured) ||
		errors.Is(err, internalproviders.ErrInvalidRequestedWindow) ||
		errors.Is(err, internalproviders.ErrInvalidProviderSyncStateWindow) ||
		errors.Is(err, internalproviders.ErrInvalidProviderSyncTargetWindow) {
		return NewTerminalFailure(
			err,
			"bank_sync_configuration_invalid",
			"Bank sync configuration is invalid",
			"The bank connection configuration cannot be synchronized.",
		)
	}
	var providerErr *ProviderResponseError
	if errors.As(err, &providerErr) && providerErr.IsTerminal() {
		return NewTerminalFailure(
			err,
			"bank_provider_rejected_request",
			"Bank provider rejected the sync request",
			"The bank provider rejected the synchronization request.",
		)
	}
	return err
}

func validateBankConnectionSyncWindows(windowStart, windowEnd *time.Time) error {
	for _, timestamp := range []struct {
		field string
		value *time.Time
	}{
		{field: "windowStart", value: windowStart},
		{field: "windowEnd", value: windowEnd},
	} {
		if timestamp.value == nil {
			continue
		}
		if timestamp.value.IsZero() {
			return fmt.Errorf("bank connection sync %s must be non-zero", timestamp.field)
		}
	}
	return nil
}

func (s *BankSyncService) ListBankConnections(
	ctx context.Context,
	params ListBankConnectionsParams,
) ([]BankConnectionView, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return nil, err
	}
	connections, err := s.store.ListBankConnections(ctx, params.TenantID)
	if err != nil { // coverage-ignore // Store errors are covered by persistence tests.
		return nil, fmt.Errorf("list bank connections: %w", err)
	}
	views := make([]BankConnectionView, 0, len(connections))
	for _, connection := range connections {
		schedule, scheduleErr := s.store.GetBankConnectionSchedule(ctx, connection.ID)
		if errors.Is(scheduleErr, persistence.ErrBankConnectionScheduleNotFound) {
			views = append(views, BankConnectionView{Connection: connection})
			continue
		}
		if scheduleErr != nil { // coverage-ignore // Store errors are covered by persistence tests.
			return nil, fmt.Errorf("list bank connections: %w", scheduleErr)
		}
		views = append(views, BankConnectionView{Connection: connection, Schedule: schedule})
	}
	return views, nil
}

func (s *BankSyncService) ListBankConnectionSyncedAccounts(
	ctx context.Context,
	params ListBankConnectionSyncedAccountsParams,
) ([]BankConnectionSyncedAccount, error) {
	connection, err := s.requireTenantBankConnection(ctx, params.TenantID, params.ActorUserID, params.ConnectionID)
	if err != nil {
		return nil, err
	}
	accounts, err := s.store.ListConnectionProviderAccounts(ctx, connection.ID)
	if err != nil { // coverage-ignore // Store errors are covered by persistence tests.
		return nil, fmt.Errorf("list bank connection synced accounts: %w", err)
	}
	items := make([]BankConnectionSyncedAccount, 0, len(accounts))
	for _, account := range accounts {
		if account.FinanceAccountID == "" {
			continue
		}
		items = append(items, BankConnectionSyncedAccount{
			FinanceAccountID:     account.FinanceAccountID,
			Name:                 account.Name,
			Currency:             account.Currency,
			LastSuccessfulSyncAt: account.LastSuccessfulSyncAt,
		})
	}
	return items, nil
}

func (s *BankSyncService) requireTenantBankConnection(
	ctx context.Context,
	tenantID string,
	userID string,
	connectionID string,
) (domain.BankConnection, error) {
	if err := s.access.requireTenantMember(ctx, tenantID, userID); err != nil {
		return domain.BankConnection{}, err
	}
	connection, err := s.store.GetBankConnection(ctx, strings.TrimSpace(connectionID))
	if err != nil {
		return domain.BankConnection{}, ErrBankConnectionNotFound
	}
	if connection.TenantID != strings.TrimSpace(tenantID) {
		return domain.BankConnection{}, ErrBankConnectionNotFound
	}
	return *connection, nil
}

func (s *BankSyncService) deleteBankConnectionOwnedMetadata(
	ctx context.Context,
	connection domain.BankConnection,
) error {
	// Delete the connection first to take the parent-row lock before owned
	// state. Link persistence takes this same lock before saving a snapshot or
	// schedule, preventing an orphaned write after deletion.
	if store, ok := s.store.(*persistence.Store); ok {
		deleted, deleteErr := persistence.NewBankAuthorizationStore(store).DeleteConnection(ctx, connection.ID)
		if errors.Is(deleteErr, persistence.ErrBankConnectionNotFound) {
			// A competing authorized deletion already removed the row and owns cleanup.
			return nil
		}
		if deleteErr != nil {
			return fmt.Errorf("delete bank connection: %w", deleteErr)
		}
		connection = *deleted
	} else if err := s.store.DeleteBankConnection(ctx, connection.ID); err != nil { // coverage-ignore // Production persistence uses the returning delete in its transaction.
		return fmt.Errorf("delete bank connection: %w", err)
	}
	if s.syncStateJournalDeleter != nil {
		deleteSyncStatesErr := s.syncStateJournalDeleter.DeleteSyncStatesByConnection(ctx, connection.ID)
		if deleteSyncStatesErr != nil { // coverage-ignore // Journal-store errors are covered by persistence tests.
			return fmt.Errorf("delete bank connection provider sync states: %w", deleteSyncStatesErr)
		}
	}
	if s.snapshotDeleter != nil {
		if err := s.snapshotDeleter.DeleteProviderSnapshotsByConnection(ctx, connection.ID); err != nil {
			return fmt.Errorf("delete bank connection provider snapshots: %w", err)
		}
	}
	for _, step := range []func(context.Context) error{
		func(ctx context.Context) error { return s.store.DeleteProviderTransactionMatches(ctx, connection.ID) },
		func(ctx context.Context) error { return s.store.DeleteBalanceSnapshots(ctx, connection.ID) },
		func(ctx context.Context) error { return s.store.DeleteConnectionProviderAccounts(ctx, connection.ID) },
		func(ctx context.Context) error { return s.store.DeleteBankConnectionSchedule(ctx, connection.ID) },
		func(ctx context.Context) error { return s.store.DeleteConnectionSecret(ctx, connection.SecretID) },
	} {
		if stepErr := step(ctx); stepErr != nil { // coverage-ignore // Store errors are covered by persistence tests.
			return fmt.Errorf("delete bank connection: %w", stepErr)
		}
	}
	return nil
}

func (s *BankSyncService) makeScheduledRunMetadata(
	ctx context.Context,
	connection domain.BankConnection,
	params RunBankConnectionSyncParams,
	now time.Time,
) (*ProviderScheduledRunMetadata, bool, error) {
	if strings.TrimSpace(params.Reason) != BankConnectionSyncReasonScheduled {
		return nil, false, nil
	}
	metadata := &ProviderScheduledRunMetadata{ScheduledAt: now}
	if params.ScheduledAt != nil || params.ScheduledNextRunAt != nil {
		if params.ScheduledAt == nil || params.ScheduledAt.IsZero() {
			return nil, false, errors.New("scheduled at must be a non-zero timestamp")
		}
		invalidScheduledNextRun := params.ScheduledNextRunAt == nil || params.ScheduledNextRunAt.IsZero()
		if invalidScheduledNextRun { // coverage-ignore // Scheduler request validation is covered by jobs tests.
			return nil, false, errors.New("scheduled next run at must be a non-zero timestamp")
		}
		if !params.ScheduledNextRunAt.After(*params.ScheduledAt) {
			return nil, false, errors.New("scheduled next run at must be after scheduled at")
		}
		metadata.ScheduledAt = *params.ScheduledAt
		metadata.NextRunAt = params.ScheduledNextRunAt
		return metadata, true, nil
	}
	schedule, err := s.store.GetBankConnectionSchedule(ctx, connection.ID)
	scheduleMissing := errors.Is(err, persistence.ErrBankConnectionScheduleNotFound) // coverage-ignore
	if scheduleMissing {                                                             // coverage-ignore // The no-schedule case is covered by schedule lifecycle tests.
		return metadata, true, nil
	}
	if err != nil { // coverage-ignore // Store errors are covered by persistence tests.
		return nil, false, fmt.Errorf("prepare bank connection sync schedule: %w", err)
	}
	if schedule.Enabled && schedule.Interval > 0 { // coverage-ignore
		nextRunAt := now.Add(schedule.Interval)
		metadata.NextRunAt = &nextRunAt
	}
	return metadata, true, nil // coverage-ignore // Scheduler metadata is covered by schedule lifecycle tests.
}

func (s *BankSyncService) markBankConnectionSyncStarted(
	ctx context.Context,
	connection *domain.BankConnection,
	params RunBankConnectionSyncParams,
	now time.Time,
	_ *ProviderScheduledRunMetadata,
) error {
	return terminalBankConnectionSyncFailure(
		s.projections.ProjectSync(
			ctx,
			persistence.BankSyncProjection{
				Connection: bankConnectionRef(*connection),
				JobID:      params.JobID,
				At:         now,
				Phase:      "started",
			},
		),
	)
}

func (s *BankSyncService) recordBankConnectionSyncFailure(
	ctx context.Context,
	connection *domain.BankConnection,
	params RunBankConnectionSyncParams,
	_ time.Time,
	_ *ProviderScheduledRunMetadata,
	syncErr error,
) error {
	feedback := syncErr.Error()
	reauth := errors.Is(syncErr, domain.ErrBankReauthRequired)
	if reauth {
		feedback = "The bank session is closed or expired. Update the bank link to continue syncing."
	}
	if err := s.projections.ProjectSync(
		ctx,
		persistence.BankSyncProjection{
			Connection: bankConnectionRef(*connection),
			JobID:      params.JobID,
			At:         s.now(),
			Phase:      "failed",
			Error:      feedback,
			Reauth:     reauth,
		},
	); err != nil {
		return err
	}
	return fmt.Errorf("run bank connection sync: %w", syncErr)
}

func (s *BankSyncService) completeBankConnectionSync(
	ctx context.Context,
	connection *domain.BankConnection,
	jobID string,
	_ *ProviderScheduledRunMetadata,
	now time.Time,
) error {
	return s.projections.ProjectSync(
		ctx,
		persistence.BankSyncProjection{
			Connection: bankConnectionRef(*connection),
			JobID:      jobID,
			At:         now,
			Phase:      "succeeded",
		},
	)
}
