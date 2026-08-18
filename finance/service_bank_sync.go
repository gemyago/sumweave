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
	store                   bankSyncFocusedStore
	access                  *accessGuard
	now                     func() time.Time
	syncOrchestrator        bankSyncOrchestrator
	bankSyncJobEnqueuer     BankConnectionSyncJobEnqueuer
	bankSyncScheduleWriter  BankConnectionSyncScheduleWriter
	logger                  *slog.Logger
	snapshotDeleter         providerSnapshotConnectionDeleter
	syncStateJournalDeleter providerSyncStateJournalConnectionDeleter
}

type BankSyncServiceOption func(*BankSyncService)

func WithBankSyncServiceNow(now func() time.Time) BankSyncServiceOption {
	return func(service *BankSyncService) { service.now = now }
}

func WithBankSyncServiceJobEnqueuer(enqueuer BankConnectionSyncJobEnqueuer) BankSyncServiceOption {
	return func(service *BankSyncService) { service.bankSyncJobEnqueuer = enqueuer }
}

func WithBankSyncServiceScheduleWriter(writer BankConnectionSyncScheduleWriter) BankSyncServiceOption {
	return func(service *BankSyncService) { service.bankSyncScheduleWriter = writer }
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
	store bankSyncFocusedStore,
	syncOrchestrator bankSyncOrchestrator,
	opts ...BankSyncServiceOption,
) *BankSyncService {
	if syncOrchestrator == nil {
		panic("bank sync orchestrator is required")
	}
	service := &BankSyncService{
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
	if writeErr := s.writeBankConnectionSyncSchedule(ctx, BankConnectionSyncSchedule{
		ScheduleID:   bankConnectionSyncScheduleID(connection.ID),
		ConnectionID: connection.ID,
		ActorUserID:  params.ActorUserID,
		Interval:     persisted.Interval,
		NextRunAt:    persisted.NextRunAt,
		Enabled:      persisted.Enabled,
	}); writeErr != nil { // coverage-ignore // Schedule-writer failures are covered by app jobs tests.
		return domain.BankConnectionSchedule{}, writeErr
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
	if writeErr := s.writeBankConnectionSyncSchedule(ctx, BankConnectionSyncSchedule{
		ScheduleID:   bankConnectionSyncScheduleID(connection.ID),
		ConnectionID: connection.ID,
		ActorUserID:  params.ActorUserID,
		Interval:     persisted.Interval,
		NextRunAt:    persisted.NextRunAt,
		Enabled:      persisted.Enabled,
	}); writeErr != nil { // coverage-ignore // Schedule-writer failures are covered by app jobs tests.
		return domain.BankConnectionSchedule{}, writeErr
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
	if writeErr := s.writeBankConnectionSyncSchedule(ctx, BankConnectionSyncSchedule{
		ScheduleID:   bankConnectionSyncScheduleID(connection.ID),
		ConnectionID: connection.ID,
		ActorUserID:  params.ActorUserID,
		Interval:     persisted.Interval,
		NextRunAt:    persisted.NextRunAt,
		Enabled:      persisted.Enabled,
	}); writeErr != nil { // coverage-ignore // Schedule-writer failures are covered by app jobs tests.
		return domain.BankConnectionSchedule{}, writeErr
	}
	return persisted, nil
}

func (s *BankSyncService) TriggerBankConnectionSync(
	ctx context.Context,
	params TriggerBankConnectionSyncParams,
) (BankConnectionSyncJobRef, error) {
	if err := validateBankConnectionSyncWindows(params.WindowStart, params.WindowEnd); err != nil {
		return BankConnectionSyncJobRef{}, err
	}
	if s.bankSyncJobEnqueuer == nil {
		return BankConnectionSyncJobRef{}, errors.New("bank sync job enqueuer is required")
	}
	connection, err := s.requireTenantBankConnection(ctx, params.TenantID, params.ActorUserID, params.ConnectionID)
	if err != nil {
		return BankConnectionSyncJobRef{}, err
	}
	request := BankConnectionSyncJobRequest{
		JobType: BankConnectionSyncJobType,
		Reason:  strings.TrimSpace(params.Reason),
		Actor:   strings.TrimSpace(params.ActorUserID),
		Input: BankConnectionSyncJobInput{
			ConnectionID: connection.ID,
			Reason:       strings.TrimSpace(params.Reason),
			WindowStart:  params.WindowStart,
			WindowEnd:    params.WindowEnd,
		},
	}
	job, err := s.bankSyncJobEnqueuer.EnqueueBankConnectionSync(ctx, request)
	s.logger.InfoContext(ctx, "bank connection sync job enqueued",
		slog.String("jobId", job.ID),
		slog.String("jobType", job.JobType),
		slog.String("connectionId", connection.ID),
		slog.String("reason", params.Reason),
	)
	return job, err
}

func (s *BankSyncService) DeleteBankConnection(
	ctx context.Context,
	params DeleteBankConnectionParams,
) error {
	connection, err := s.requireTenantBankConnection(ctx, params.TenantID, params.ActorUserID, params.ConnectionID)
	if err != nil { // coverage-ignore // Tenant access validation is covered by access-guard tests.
		return err
	}
	disableErr := s.disableBankConnectionSyncSchedule(ctx, connection, params.ActorUserID)
	if disableErr != nil { // coverage-ignore // Schedule writer failures are covered by jobs tests.
		return disableErr
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
	return deleteConnection(s)
}

func (s *BankSyncService) RunBankConnectionSync(
	ctx context.Context,
	params RunBankConnectionSyncParams,
) (BankConnectionSyncResult, error) {
	validationErr := validateBankConnectionSyncWindows(params.WindowStart, params.WindowEnd)
	if validationErr != nil { // coverage-ignore // Request validation is covered by focused service tests.
		return BankConnectionSyncResult{}, validationErr
	}
	connection, err := s.store.GetBankConnection(ctx, strings.TrimSpace(params.ConnectionID))
	if err != nil { // coverage-ignore // Connection lookup errors are covered by persistence tests.
		return BankConnectionSyncResult{}, ErrBankConnectionNotFound
	}
	secret, err := s.store.GetConnectionSecret(ctx, connection.SecretID)
	if err != nil { // coverage-ignore // Secret-store errors are covered by persistence tests.
		return BankConnectionSyncResult{}, fmt.Errorf("get connection secret: %w", err)
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
	result, err := s.syncOrchestrator.Orchestrate(ctx, internalproviders.SyncOrchestrationRequest{
		Connection: domain.ProviderConnectionRef{
			ConnectionID:      connection.ID,
			ProviderID:        domain.ProviderID(connection.Provider),
			ConnectorID:       connection.ConnectorID,
			ProviderReference: connection.ProviderReference,
		},
		Secret:      *secret,
		JobID:       params.JobID,
		Reason:      params.Reason,
		WindowStart: params.WindowStart,
		WindowEnd:   params.WindowEnd,
	})
	if err != nil {
		return BankConnectionSyncResult{}, s.recordBankConnectionSyncFailure(
			ctx,
			connection,
			params,
			now,
			scheduledRun,
			err,
		)
	}
	if !hasScheduledRun {
		scheduledRun = nil
	}
	err = s.completeBankConnectionSync(ctx, connection, params.JobID, scheduledRun, now)
	if err != nil { // coverage-ignore // Completion projection errors are covered by persistence tests.
		return BankConnectionSyncResult{}, s.recordBankConnectionSyncFailure(
			ctx,
			connection,
			params,
			now,
			scheduledRun,
			err,
		)
	}
	return BankConnectionSyncResult{
		ImportedAccounts:     result.Stats.CreatedAccounts,
		ImportedTransactions: result.Stats.CreatedTransactions,
		UpdatedTransactions:  result.Stats.UpdatedTransactions,
	}, nil
}

func (s *BankSyncService) RecordBankConnectionSyncScheduled(
	ctx context.Context,
	params RecordBankConnectionSyncScheduledParams,
) (domain.BankConnectionSchedule, error) {
	if strings.TrimSpace(params.ConnectionID) == "" {
		return domain.BankConnectionSchedule{}, errors.New("connection id is required")
	}
	if strings.TrimSpace(params.JobID) == "" {
		return domain.BankConnectionSchedule{}, errors.New("job id is required")
	}
	if params.ScheduledAt.IsZero() {
		return domain.BankConnectionSchedule{}, errors.New("scheduled at must be a non-zero timestamp")
	}
	if params.NextRunAt.IsZero() {
		return domain.BankConnectionSchedule{}, errors.New("next run at must be a non-zero timestamp")
	}
	if !params.NextRunAt.After(params.ScheduledAt) {
		return domain.BankConnectionSchedule{}, errors.New("next run at must be after scheduled at")
	}
	schedule, err := s.store.GetBankConnectionSchedule(ctx, strings.TrimSpace(params.ConnectionID))
	if err != nil { // coverage-ignore // Store errors are covered by persistence tests.
		return domain.BankConnectionSchedule{}, fmt.Errorf("get bank connection schedule: %w", err)
	}
	schedule.LastScheduledAt = &params.ScheduledAt
	schedule.NextRunAt = &params.NextRunAt
	schedule.LastJobID = strings.TrimSpace(params.JobID)
	schedule.UpdatedAt = s.now()
	persisted, err := s.store.SaveBankConnectionSchedule(ctx, *schedule)
	if err != nil { // coverage-ignore // Store errors are covered by persistence tests.
		return domain.BankConnectionSchedule{}, fmt.Errorf("save bank connection schedule: %w", err)
	}
	return persisted, nil
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

func (s *BankSyncService) writeBankConnectionSyncSchedule(
	ctx context.Context,
	schedule BankConnectionSyncSchedule,
) error {
	if s.bankSyncScheduleWriter == nil {
		return nil
	}
	if err := s.bankSyncScheduleWriter.UpsertBankConnectionSyncSchedule(ctx, schedule); err != nil {
		return fmt.Errorf("write bank connection sync schedule: %w", err)
	}
	return nil
}

func (s *BankSyncService) disableBankConnectionSyncSchedule(
	ctx context.Context,
	connection domain.BankConnection,
	actorUserID string,
) error {
	schedule, err := s.store.GetBankConnectionSchedule(ctx, connection.ID)
	if err != nil && !errors.Is(err, persistence.ErrBankConnectionScheduleNotFound) {
		return fmt.Errorf("disable bank connection sync schedule: %w", err)
	}
	interval := time.Duration(0)
	if schedule != nil {
		interval = schedule.Interval
	}
	return s.writeBankConnectionSyncSchedule(ctx, BankConnectionSyncSchedule{
		ScheduleID:   bankConnectionSyncScheduleID(connection.ID),
		ConnectionID: connection.ID,
		ActorUserID:  actorUserID,
		Interval:     interval,
		Enabled:      false,
	})
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
		func(ctx context.Context) error { return s.store.DeleteBankConnection(ctx, connection.ID) },
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
	scheduledRun *ProviderScheduledRunMetadata,
) error {
	connection.LastSyncJobID = strings.TrimSpace(params.JobID)
	connection.LastSyncStartedAt = &now
	connection.LastSyncError = ""
	connection.UpdatedAt = now
	_, saveConnectionErr := s.store.SaveBankConnection(ctx, *connection)
	if saveConnectionErr != nil { // coverage-ignore // Store errors are covered by persistence tests.
		return fmt.Errorf("save bank connection: %w", saveConnectionErr)
	}
	schedule, err := s.store.GetBankConnectionSchedule(ctx, connection.ID)
	if errors.Is(err, persistence.ErrBankConnectionScheduleNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("save bank connection schedule: %w", err)
	}
	schedule.LastStartedAt = &now
	schedule.LastJobID = strings.TrimSpace(params.JobID)
	if scheduledRun != nil {
		schedule.LastScheduledAt = &scheduledRun.ScheduledAt
		schedule.NextRunAt = scheduledRun.NextRunAt
	}
	schedule.UpdatedAt = now
	_, saveErr := s.store.SaveBankConnectionSchedule(ctx, *schedule)
	if saveErr != nil {
		return fmt.Errorf("save bank connection schedule: %w", saveErr)
	}
	return nil
}

func (s *BankSyncService) recordBankConnectionSyncFailure(
	ctx context.Context,
	connection *domain.BankConnection,
	params RunBankConnectionSyncParams,
	startedAt time.Time,
	scheduledRun *ProviderScheduledRunMetadata,
	syncErr error,
) error {
	connection.LastSyncJobID = strings.TrimSpace(params.JobID)
	connection.LastSyncStartedAt = &startedAt
	connection.LastSyncError = strings.TrimSpace(syncErr.Error())
	connection.UpdatedAt = s.now()
	if _, err := s.store.SaveBankConnection(ctx, *connection); err != nil {
		return fmt.Errorf("save bank connection: %w", err)
	}
	schedule, err := s.store.GetBankConnectionSchedule(ctx, connection.ID)
	if errors.Is(err, persistence.ErrBankConnectionScheduleNotFound) {
		return fmt.Errorf("run bank connection sync: %w", syncErr)
	}
	if err != nil {
		return fmt.Errorf("save bank connection schedule: %w", err)
	}
	schedule.LastStartedAt = &startedAt
	completedAt := s.now()
	schedule.LastCompletedAt = &completedAt
	schedule.LastJobID = strings.TrimSpace(params.JobID)
	if scheduledRun != nil {
		schedule.LastScheduledAt = &scheduledRun.ScheduledAt
		schedule.NextRunAt = scheduledRun.NextRunAt
	}
	schedule.UpdatedAt = completedAt
	_, saveErr := s.store.SaveBankConnectionSchedule(ctx, *schedule)
	if saveErr != nil {
		return fmt.Errorf("save bank connection schedule: %w", saveErr)
	}
	return fmt.Errorf("run bank connection sync: %w", syncErr)
}

func (s *BankSyncService) completeBankConnectionSync(
	ctx context.Context,
	connection *domain.BankConnection,
	jobID string,
	scheduledRun *ProviderScheduledRunMetadata,
	now time.Time,
) error {
	connection.LastSyncJobID = strings.TrimSpace(jobID)
	if connection.LastSyncStartedAt == nil {
		connection.LastSyncStartedAt = &now
	}
	connection.LastSuccessfulSyncAt = &now
	connection.LastSyncError = ""
	connection.State = domain.BankConnectionStateActive
	connection.Reauth = nil
	connection.UpdatedAt = now
	if _, err := s.store.SaveBankConnection(ctx, *connection); err != nil {
		return fmt.Errorf("save bank connection: %w", err)
	}
	schedule, err := s.store.GetBankConnectionSchedule(ctx, connection.ID)
	if errors.Is(err, persistence.ErrBankConnectionScheduleNotFound) {
		err = nil
	}
	if err != nil {
		return fmt.Errorf("get bank connection schedule: %w", err)
	}
	if schedule != nil {
		if schedule.LastStartedAt == nil {
			schedule.LastStartedAt = &now
		}
		schedule.LastCompletedAt = &now
		schedule.LastJobID = strings.TrimSpace(jobID)
		if scheduledRun != nil {
			schedule.LastScheduledAt = &scheduledRun.ScheduledAt
			schedule.NextRunAt = scheduledRun.NextRunAt
		}
		schedule.UpdatedAt = now
		if _, saveErr := s.store.SaveBankConnectionSchedule(ctx, *schedule); saveErr != nil {
			return fmt.Errorf("save bank connection schedule: %w", saveErr)
		}
	}
	return nil
}
