package finance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/google/uuid"
)

type bankSyncFocusedStore interface {
	IsTenantMember(ctx context.Context, tenantID string, userID string) (bool, error)
	ListAccounts(ctx context.Context, tenantID string, includeHidden bool) ([]domain.Account, error)
	SaveAccount(ctx context.Context, account domain.Account) (domain.Account, error)
	GetTransaction(ctx context.Context, transactionID string) (*domain.Transaction, error)
	SaveTransaction(ctx context.Context, transaction domain.Transaction) (domain.Transaction, error)
	bankSyncStore
	connectionSecretStore
}

type BankSyncService struct {
	store                  bankSyncFocusedStore
	access                 *accessGuard
	now                    func() time.Time
	newID                  func() string
	connectionSecretCipher connectionSecretCipher
	bankProviders          map[string]BankConnectionProvider
	bankSyncJobEnqueuer    BankConnectionSyncJobEnqueuer
	bankSyncScheduleWriter BankConnectionSyncScheduleWriter
	logger                 *slog.Logger
}

const (
	bankSyncInitialBackfillYears = 3
	bankSyncRecentRefreshDays    = 30
)

type BankSyncServiceOption func(*BankSyncService)

func WithBankSyncServiceNow(now func() time.Time) BankSyncServiceOption {
	return func(service *BankSyncService) { service.now = now }
}

func WithBankSyncServiceIDGenerator(newID func() string) BankSyncServiceOption {
	return func(service *BankSyncService) { service.newID = newID }
}

func WithBankSyncServiceConnectionSecretCipher(cipher connectionSecretCipher) BankSyncServiceOption {
	return func(service *BankSyncService) { service.connectionSecretCipher = cipher }
}

func WithBankSyncServiceProviders(providers ...BankConnectionProvider) BankSyncServiceOption {
	return func(service *BankSyncService) {
		if service.bankProviders == nil {
			service.bankProviders = map[string]BankConnectionProvider{}
		}
		for _, provider := range providers {
			if provider != nil {
				service.bankProviders[provider.Name()] = provider
			}
		}
	}
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

func NewBankSyncService(store bankSyncFocusedStore, opts ...BankSyncServiceOption) *BankSyncService {
	service := &BankSyncService{
		store:         store,
		access:        newAccessGuard(store),
		now:           func() time.Time { return time.Now().UTC() },
		newID:         uuid.NewString,
		bankProviders: map[string]BankConnectionProvider{},
		logger:        slog.New(slog.DiscardHandler),
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
	if err != nil {
		return domain.BankConnectionSchedule{}, err
	}
	now := s.now().UTC()
	existing, err := s.store.GetBankConnectionSchedule(ctx, connection.ID)
	if err != nil && !errors.Is(err, persistence.ErrBankConnectionScheduleNotFound) {
		return domain.BankConnectionSchedule{}, fmt.Errorf("upsert bank connection schedule: %w", err)
	}
	schedule := domain.BankConnectionSchedule{
		ConnectionID: connection.ID,
		Interval:     params.Interval,
		NextRunAt:    timePtrUTC(params.NextRunAt),
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
	if err != nil {
		return domain.BankConnectionSchedule{}, err
	}
	if writeErr := s.writeBankConnectionSyncSchedule(ctx, BankConnectionSyncSchedule{
		ScheduleID:   bankConnectionSyncScheduleID(connection.ID),
		ConnectionID: connection.ID,
		ActorUserID:  params.ActorUserID,
		Interval:     persisted.Interval,
		NextRunAt:    persisted.NextRunAt,
		Enabled:      persisted.Enabled,
	}); writeErr != nil {
		return domain.BankConnectionSchedule{}, writeErr
	}
	return persisted, nil
}

func (s *BankSyncService) PauseBankConnectionSchedule(
	ctx context.Context,
	params PauseBankConnectionScheduleParams,
) (domain.BankConnectionSchedule, error) {
	connection, err := s.requireTenantBankConnection(ctx, params.TenantID, params.ActorUserID, params.ConnectionID)
	if err != nil {
		return domain.BankConnectionSchedule{}, err
	}
	schedule, err := s.store.GetBankConnectionSchedule(ctx, connection.ID)
	if err != nil {
		return domain.BankConnectionSchedule{}, fmt.Errorf("pause bank connection schedule: %w", err)
	}
	schedule.Enabled = false
	schedule.UpdatedAt = s.now().UTC()
	persisted, err := s.store.SaveBankConnectionSchedule(ctx, *schedule)
	if err != nil {
		return domain.BankConnectionSchedule{}, err
	}
	if writeErr := s.writeBankConnectionSyncSchedule(ctx, BankConnectionSyncSchedule{
		ScheduleID:   bankConnectionSyncScheduleID(connection.ID),
		ConnectionID: connection.ID,
		ActorUserID:  params.ActorUserID,
		Interval:     persisted.Interval,
		NextRunAt:    persisted.NextRunAt,
		Enabled:      persisted.Enabled,
	}); writeErr != nil {
		return domain.BankConnectionSchedule{}, writeErr
	}
	return persisted, nil
}

func (s *BankSyncService) ResumeBankConnectionSchedule(
	ctx context.Context,
	params ResumeBankConnectionScheduleParams,
) (domain.BankConnectionSchedule, error) {
	connection, err := s.requireTenantBankConnection(ctx, params.TenantID, params.ActorUserID, params.ConnectionID)
	if err != nil {
		return domain.BankConnectionSchedule{}, err
	}
	schedule, err := s.store.GetBankConnectionSchedule(ctx, connection.ID)
	if err != nil {
		return domain.BankConnectionSchedule{}, fmt.Errorf("resume bank connection schedule: %w", err)
	}
	schedule.Enabled = true
	schedule.NextRunAt = timePtrUTC(params.NextRunAt)
	schedule.UpdatedAt = s.now().UTC()
	persisted, err := s.store.SaveBankConnectionSchedule(ctx, *schedule)
	if err != nil {
		return domain.BankConnectionSchedule{}, err
	}
	if writeErr := s.writeBankConnectionSyncSchedule(ctx, BankConnectionSyncSchedule{
		ScheduleID:   bankConnectionSyncScheduleID(connection.ID),
		ConnectionID: connection.ID,
		ActorUserID:  params.ActorUserID,
		Interval:     persisted.Interval,
		NextRunAt:    persisted.NextRunAt,
		Enabled:      persisted.Enabled,
	}); writeErr != nil {
		return domain.BankConnectionSchedule{}, writeErr
	}
	return persisted, nil
}

func (s *BankSyncService) TriggerBankConnectionSync(
	ctx context.Context,
	params TriggerBankConnectionSyncParams,
) (BankConnectionSyncJobRef, error) {
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
	return s.bankSyncJobEnqueuer.EnqueueBankConnectionSync(ctx, request)
}

func (s *BankSyncService) DeleteBankConnection(
	ctx context.Context,
	params DeleteBankConnectionParams,
) error {
	connection, err := s.requireTenantBankConnection(ctx, params.TenantID, params.ActorUserID, params.ConnectionID)
	if err != nil {
		return err
	}
	if disableErr := s.disableBankConnectionSyncSchedule(ctx, connection, params.ActorUserID); disableErr != nil {
		return disableErr
	}
	deleteConnection := func(service *BankSyncService) error {
		return service.deleteBankConnectionOwnedMetadata(ctx, connection)
	}
	if txStore, ok := s.store.(*persistence.Store); ok {
		if txErr := txStore.WithTransaction(ctx, func(store *persistence.Store) error {
			txService := *s
			txService.store = store
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
	connection, err := s.store.GetBankConnection(ctx, strings.TrimSpace(params.ConnectionID))
	if err != nil {
		return BankConnectionSyncResult{}, ErrBankConnectionNotFound
	}
	provider, err := s.bankProviderForSync(connection.Provider)
	if err != nil {
		return BankConnectionSyncResult{}, err
	}
	secret, err := s.decryptConnectionSecret(ctx, connection.SecretID)
	if err != nil {
		return BankConnectionSyncResult{}, err
	}
	now := s.now().UTC()
	windowStart, windowEnd := resolveBankConnectionSyncWindow(*connection, params, now)
	scheduledRun, hasScheduledRun, err := s.makeScheduledRunMetadata(ctx, *connection, params, now)
	if err != nil {
		return BankConnectionSyncResult{}, err
	}
	markErr := s.markBankConnectionSyncStarted(ctx, connection, params, now, scheduledRun)
	if markErr != nil {
		return BankConnectionSyncResult{}, markErr
	}
	result, err := provider.Sync(ctx, ProviderSyncParams{
		ConnectionID:      connection.ID,
		ProviderReference: connection.ProviderReference,
		Secret:            secret,
		ExternalID:        connection.ExternalID,
		WindowStart:       windowStart,
		WindowEnd:         windowEnd,
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
	if result.ScheduledRun == nil && hasScheduledRun {
		result.ScheduledRun = scheduledRun
	}
	applyResult, err := s.ApplyProviderSyncResult(ctx, ApplyProviderSyncResultParams{
		ConnectionID: connection.ID,
		JobID:        params.JobID,
		Result:       result,
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
	return applyResult, nil
}

func resolveBankConnectionSyncWindow(
	connection domain.BankConnection,
	params RunBankConnectionSyncParams,
	now time.Time,
) (time.Time, time.Time) {
	windowEnd := now.UTC()
	if !params.WindowEnd.IsZero() {
		windowEnd = params.WindowEnd.UTC()
	}
	if !params.WindowStart.IsZero() {
		return params.WindowStart.UTC(), windowEnd
	}
	recentStart := windowEnd.AddDate(0, 0, -bankSyncRecentRefreshDays)
	if connection.LastSuccessfulSyncAt == nil {
		return windowEnd.AddDate(-bankSyncInitialBackfillYears, 0, 0), windowEnd
	}
	checkpoint := connection.LastSuccessfulSyncAt.UTC()
	if checkpoint.Before(recentStart) {
		return checkpoint, windowEnd
	}
	return recentStart, windowEnd
}

func (s *BankSyncService) ApplyProviderSyncResult(
	ctx context.Context,
	params ApplyProviderSyncResultParams,
) (BankConnectionSyncResult, error) {
	if txStore, ok := s.store.(*persistence.Store); ok {
		var result BankConnectionSyncResult
		err := txStore.WithTransaction(ctx, func(store *persistence.Store) error {
			txService := *s
			txService.store = store
			var applyErr error
			result, applyErr = txService.applyProviderSyncResult(ctx, params, true)
			return applyErr
		})
		if err != nil {
			return BankConnectionSyncResult{}, err
		}
		return result, nil
	}
	return s.applyProviderSyncResult(ctx, params, false)
}

func (s *BankSyncService) ListBankConnections(
	ctx context.Context,
	params ListBankConnectionsParams,
) ([]BankConnectionView, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return nil, err
	}
	connections, err := s.store.ListBankConnections(ctx, params.TenantID)
	if err != nil {
		return nil, fmt.Errorf("list bank connections: %w", err)
	}
	views := make([]BankConnectionView, 0, len(connections))
	for _, connection := range connections {
		schedule, scheduleErr := s.store.GetBankConnectionSchedule(ctx, connection.ID)
		if errors.Is(scheduleErr, persistence.ErrBankConnectionScheduleNotFound) {
			views = append(views, BankConnectionView{Connection: connection})
			continue
		}
		if scheduleErr != nil {
			return nil, fmt.Errorf("list bank connections: %w", scheduleErr)
		}
		views = append(views, BankConnectionView{Connection: connection, Schedule: schedule})
	}
	return views, nil
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

func (s *BankSyncService) bankProviderForSync(bankID string) (*bankProviderRef, error) {
	trimmedBankID := strings.TrimSpace(bankID)
	switch trimmedBankID {
	case bankProviderMonobank:
		return s.bankProviderForLink(trimmedBankID, bankLinkMethodToken)
	case bankProviderPKO:
		return s.bankProviderForLink(trimmedBankID, bankLinkMethodRedirect)
	default:
		return s.bankProvider(trimmedBankID)
	}
}

func (s *BankSyncService) bankProvider(name string) (*bankProviderRef, error) {
	trimmedName := strings.TrimSpace(name)
	provider, ok := s.bankProviders[trimmedName]
	if !ok {
		return nil, bankProviderNotConfiguredError(trimmedName)
	}
	return &bankProviderRef{BankConnectionProvider: provider, bankID: trimmedName}, nil
}

func (s *BankSyncService) bankProviderForLink(bankID string, method bankLinkMethod) (*bankProviderRef, error) {
	trimmedBankID := strings.TrimSpace(bankID)
	providerName, err := configuredBankProviderName(trimmedBankID, method)
	if err != nil {
		return nil, err
	}
	provider, ok := s.bankProviders[providerName]
	if !ok {
		return nil, bankProviderNotConfiguredForBankError(trimmedBankID, providerName)
	}
	return &bankProviderRef{BankConnectionProvider: provider, bankID: trimmedBankID}, nil
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

func (s *BankSyncService) decryptConnectionSecret(ctx context.Context, secretID string) (string, error) {
	if s.connectionSecretCipher == nil {
		return "", errors.New("connection secret cipher is required")
	}
	secret, err := s.store.GetConnectionSecret(ctx, secretID)
	if err != nil {
		return "", fmt.Errorf("get connection secret: %w", err)
	}
	plaintext, err := s.connectionSecretCipher.OpenString(secret.Envelope)
	if err != nil {
		return "", fmt.Errorf("open connection secret: %w", err)
	}
	return plaintext, nil
}

func (s *BankSyncService) deleteBankConnectionOwnedMetadata(
	ctx context.Context,
	connection domain.BankConnection,
) error {
	for _, step := range []func(context.Context) error{
		func(ctx context.Context) error { return s.store.DeleteProviderTransactionMatches(ctx, connection.ID) },
		func(ctx context.Context) error { return s.store.DeleteBankConnectionSyncRuns(ctx, connection.ID) },
		func(ctx context.Context) error { return s.store.DeleteRawPayloads(ctx, connection.ID) },
		func(ctx context.Context) error { return s.store.DeleteBalanceSnapshots(ctx, connection.ID) },
		func(ctx context.Context) error { return s.store.DeleteConnectionProviderAccounts(ctx, connection.ID) },
		func(ctx context.Context) error { return s.store.DeleteBankConnectionSchedule(ctx, connection.ID) },
		func(ctx context.Context) error { return s.store.DeleteBankConnection(ctx, connection.ID) },
		func(ctx context.Context) error { return s.store.DeleteConnectionSecret(ctx, connection.SecretID) },
	} {
		if stepErr := step(ctx); stepErr != nil {
			return fmt.Errorf("delete bank connection: %w", stepErr)
		}
	}
	return nil
}

func (s *BankSyncService) syncRunAlreadyApplied(
	ctx context.Context,
	connectionID string,
	syncKey string,
) (bool, error) {
	if strings.TrimSpace(syncKey) == "" {
		return false, nil
	}
	existing, err := s.store.GetBankConnectionSyncRun(ctx, connectionID, syncKey)
	if err != nil && !errors.Is(err, persistence.ErrBankConnectionSyncRunNotFound) {
		return false, fmt.Errorf("apply provider sync result: %w", err)
	}
	return existing != nil, nil
}

func (s *BankSyncService) claimSyncRun(
	ctx context.Context,
	connectionID string,
	syncKey string,
	jobID string,
	now time.Time,
) (bool, error) {
	if strings.TrimSpace(syncKey) == "" {
		return true, nil
	}
	claimed, err := s.store.ClaimBankConnectionSyncRun(ctx, domain.BankConnectionSyncRun{
		ID:           s.newID(),
		ConnectionID: connectionID,
		SyncKey:      strings.TrimSpace(syncKey),
		JobID:        strings.TrimSpace(jobID),
		CreatedAt:    now,
	})
	if err != nil {
		return false, fmt.Errorf("apply provider sync result: %w", err)
	}
	return claimed, nil
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
	schedule, err := s.store.GetBankConnectionSchedule(ctx, connection.ID)
	if errors.Is(err, persistence.ErrBankConnectionScheduleNotFound) {
		return metadata, true, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("prepare bank connection sync schedule: %w", err)
	}
	if schedule.Enabled && schedule.Interval > 0 {
		nextRunAt := now.Add(schedule.Interval).UTC()
		metadata.NextRunAt = &nextRunAt
	}
	return metadata, true, nil
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
	if _, err := s.store.SaveBankConnection(ctx, *connection); err != nil {
		return fmt.Errorf("save bank connection: %w", err)
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
	connection.UpdatedAt = s.now().UTC()
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
	schedule.LastJobID = strings.TrimSpace(params.JobID)
	if scheduledRun != nil {
		schedule.LastScheduledAt = &scheduledRun.ScheduledAt
		schedule.NextRunAt = scheduledRun.NextRunAt
	}
	schedule.UpdatedAt = s.now().UTC()
	_, saveErr := s.store.SaveBankConnectionSchedule(ctx, *schedule)
	if saveErr != nil {
		return fmt.Errorf("save bank connection schedule: %w", saveErr)
	}
	return fmt.Errorf("run bank connection sync: %w", syncErr)
}

func (s *BankSyncService) applyProviderSyncResult(
	ctx context.Context,
	params ApplyProviderSyncResultParams,
	atomic bool,
) (BankConnectionSyncResult, error) {
	now := s.now().UTC()
	if atomic {
		claimed, claimErr := s.claimSyncRun(ctx, params.ConnectionID, params.Result.SyncKey, params.JobID, now)
		if claimErr != nil {
			return BankConnectionSyncResult{}, claimErr
		}
		if !claimed {
			return BankConnectionSyncResult{}, nil
		}
	}
	if !atomic {
		alreadyApplied, appliedErr := s.syncRunAlreadyApplied(ctx, params.ConnectionID, params.Result.SyncKey)
		if appliedErr != nil {
			return BankConnectionSyncResult{}, appliedErr
		}
		if alreadyApplied {
			return BankConnectionSyncResult{}, nil
		}
	}
	connection, err := s.store.GetBankConnection(ctx, params.ConnectionID)
	if err != nil {
		return BankConnectionSyncResult{}, ErrBankConnectionNotFound
	}
	result := BankConnectionSyncResult{}
	accountMap, importedAccounts, err := s.applyProviderAccounts(ctx, *connection, params.Result.Accounts, now)
	if err != nil {
		return BankConnectionSyncResult{}, err
	}
	result.ImportedAccounts = importedAccounts
	importedTransactions, updatedTransactions, err := s.applyProviderTransactions(
		ctx,
		*connection,
		accountMap,
		params.Result.Transactions,
		now,
	)
	if err != nil {
		return BankConnectionSyncResult{}, err
	}
	result.ImportedTransactions = importedTransactions
	result.UpdatedTransactions = updatedTransactions
	if persistErr := s.persistProviderRawPayloads(
		ctx,
		connection.ID,
		params.Result.RawPayloads,
		now,
	); persistErr != nil {
		return BankConnectionSyncResult{}, persistErr
	}
	if completeErr := s.completeAppliedSync(ctx, connection, params, now, atomic); completeErr != nil {
		return BankConnectionSyncResult{}, completeErr
	}
	s.logger.InfoContext(
		ctx,
		"bank connection sync completed",
		"connectionId",
		connection.ID,
		"provider",
		connection.Provider,
	)
	return result, nil
}

func (s *BankSyncService) upsertProviderAccount(
	ctx context.Context,
	connection domain.BankConnection,
	item ProviderNormalizedAccount,
	now time.Time,
) (domain.ConnectionProviderAccount, error) {
	accounts, err := s.store.ListConnectionProviderAccounts(ctx, connection.ID)
	if err != nil {
		return domain.ConnectionProviderAccount{}, fmt.Errorf("list provider accounts: %w", err)
	}
	var existing *domain.ConnectionProviderAccount
	for _, account := range accounts {
		if account.ProviderAccountID == item.ProviderAccountID {
			copyAccount := account
			existing = &copyAccount
			break
		}
	}
	financeAccountID := ""
	if existing != nil {
		financeAccountID = existing.FinanceAccountID
	}
	if financeAccountID == "" {
		financeAccount, accountErr := s.findOrCreateFinanceAccountForProviderAccount(ctx, connection, item, now)
		if accountErr != nil {
			return domain.ConnectionProviderAccount{}, accountErr
		}
		financeAccountID = financeAccount.ID
	}
	providerAccount := domain.ConnectionProviderAccount{
		ID:                   firstNonEmpty(accountID(existing), s.newID()),
		ConnectionID:         connection.ID,
		ProviderAccountID:    item.ProviderAccountID,
		FinanceAccountID:     financeAccountID,
		Name:                 item.Name,
		Currency:             item.Currency,
		IBAN:                 item.IBAN,
		MaskedPAN:            item.MaskedPAN,
		LastSuccessfulSyncAt: &now,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if existing != nil {
		providerAccount.CreatedAt = existing.CreatedAt
	}
	return s.store.SaveConnectionProviderAccount(ctx, providerAccount)
}

func (s *BankSyncService) findOrCreateFinanceAccountForProviderAccount(
	ctx context.Context,
	connection domain.BankConnection,
	item ProviderNormalizedAccount,
	now time.Time,
) (domain.Account, error) {
	accounts, err := s.store.ListAccounts(ctx, connection.TenantID, true)
	if err != nil {
		return domain.Account{}, fmt.Errorf("list accounts: %w", err)
	}
	for _, account := range accounts {
		if account.LinkedAccount != nil && account.LinkedAccount.Provider == connection.Provider &&
			account.LinkedAccount.ProviderAccountID == item.ProviderAccountID {
			return account, nil
		}
	}
	account := domain.Account{
		ID:       s.newID(),
		TenantID: connection.TenantID,
		Name:     firstNonEmpty(strings.TrimSpace(item.Name), strings.TrimSpace(item.IBAN), item.ProviderAccountID),
		Currency: strings.ToUpper(strings.TrimSpace(item.Currency)),
		Kind:     domain.AccountKindLinked,
		LinkedAccount: &domain.LinkedAccount{
			Provider:          connection.Provider,
			ProviderAccountID: item.ProviderAccountID,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	return s.store.SaveAccount(ctx, account)
}

func (s *BankSyncService) applyProviderTransaction(
	ctx context.Context,
	connection domain.BankConnection,
	providerAccount domain.ConnectionProviderAccount,
	item ProviderNormalizedTransaction,
	now time.Time,
) (bool, error) {
	var match *domain.ProviderTransactionMatch
	var err error
	if strings.TrimSpace(item.ProviderTransactionID) != "" {
		match, err = s.store.GetProviderTransactionMatchByProviderID(
			ctx,
			connection.ID,
			providerAccount.ProviderAccountID,
			item.ProviderTransactionID,
		)
		if err != nil && !errors.Is(err, persistence.ErrProviderTransactionMatchNotFound) {
			return false, fmt.Errorf("get provider transaction match: %w", err)
		}
	}
	if match == nil && strings.TrimSpace(item.Fingerprint) != "" {
		match, err = s.store.GetProviderTransactionMatchByFingerprint(
			ctx,
			connection.ID,
			providerAccount.ProviderAccountID,
			item.Fingerprint,
		)
		if err != nil && !errors.Is(err, persistence.ErrProviderTransactionMatchNotFound) {
			return false, fmt.Errorf("get provider transaction match: %w", err)
		}
	}
	updated := match != nil
	transaction := domain.Transaction{
		ID:               s.newID(),
		TenantID:         connection.TenantID,
		AccountID:        providerAccount.FinanceAccountID,
		Source:           domain.TransactionSourceProvider,
		Status:           item.Status,
		Kind:             domain.TransactionKindRegular,
		AmountMinor:      item.AmountMinor,
		Currency:         strings.ToUpper(strings.TrimSpace(item.Currency)),
		Description:      item.Description,
		EffectiveAt:      item.EffectiveAt.UTC(),
		CreatedAt:        now,
		UpdatedAt:        now,
		ProviderOriginal: item.ProviderOriginal,
	}
	if updated {
		existing, getErr := s.store.GetTransaction(ctx, match.TransactionID)
		if getErr != nil {
			return false, fmt.Errorf("get transaction: %w", getErr)
		}
		transaction.ID = existing.ID
		transaction.CreatedAt = existing.CreatedAt
	}
	if _, saveErr := s.store.SaveTransaction(ctx, transaction); saveErr != nil {
		return false, fmt.Errorf("save transaction: %w", saveErr)
	}
	providerMatch := domain.ProviderTransactionMatch{
		ID:                    firstNonEmpty(matchID(match), s.newID()),
		ConnectionID:          connection.ID,
		ProviderAccountID:     providerAccount.ProviderAccountID,
		ProviderTransactionID: item.ProviderTransactionID,
		Fingerprint:           item.Fingerprint,
		TransactionID:         transaction.ID,
		Status:                item.Status,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if match != nil {
		providerMatch.CreatedAt = match.CreatedAt
	}
	if _, saveErr := s.store.SaveProviderTransactionMatch(ctx, providerMatch); saveErr != nil {
		return false, fmt.Errorf("save provider transaction match: %w", saveErr)
	}
	return updated, nil
}

func (s *BankSyncService) applyProviderAccounts(
	ctx context.Context,
	connection domain.BankConnection,
	items []ProviderNormalizedAccount,
	now time.Time,
) (map[string]domain.ConnectionProviderAccount, int, error) {
	accountMap := map[string]domain.ConnectionProviderAccount{}
	importedCount := 0
	for _, item := range items {
		providerAccount, err := s.upsertProviderAccount(ctx, connection, item, now)
		if err != nil {
			return nil, 0, err
		}
		accountMap[item.ProviderAccountID] = providerAccount
		importedCount++
		if item.CurrentBalanceMinor == nil {
			continue
		}
		_, err = s.store.SaveBalanceSnapshot(ctx, domain.BalanceSnapshot{
			ID:                    s.newID(),
			ConnectionID:          connection.ID,
			ProviderAccountID:     item.ProviderAccountID,
			FinanceAccountID:      providerAccount.FinanceAccountID,
			Currency:              item.Currency,
			CurrentBalanceMinor:   *item.CurrentBalanceMinor,
			AvailableBalanceMinor: item.AvailableBalanceMinor,
			CapturedAt:            now,
		})
		if err != nil {
			return nil, 0, fmt.Errorf("save balance snapshot: %w", err)
		}
	}
	return accountMap, importedCount, nil
}

func (s *BankSyncService) applyProviderTransactions(
	ctx context.Context,
	connection domain.BankConnection,
	accountMap map[string]domain.ConnectionProviderAccount,
	items []ProviderNormalizedTransaction,
	now time.Time,
) (int, int, error) {
	importedCount := 0
	updatedCount := 0
	for _, item := range items {
		providerAccount, err := s.resolveProviderAccountForTransaction(
			ctx,
			connection.ID,
			accountMap,
			item.ProviderAccountID,
		)
		if err != nil {
			return 0, 0, err
		}
		updated, err := s.applyProviderTransaction(ctx, connection, providerAccount, item, now)
		if err != nil {
			return 0, 0, err
		}
		if updated {
			updatedCount++
		} else {
			importedCount++
		}
		if len(item.RawPayloadJSON) == 0 {
			continue
		}
		_, err = s.store.SaveRawPayload(ctx, domain.RawPayload{
			ID:               s.newID(),
			ConnectionID:     connection.ID,
			Scope:            domain.RawPayloadScopeTransaction,
			ProviderObjectID: firstNonEmpty(item.ProviderTransactionID, item.Fingerprint),
			PayloadJSON:      item.RawPayloadJSON,
			CapturedAt:       now,
		})
		if err != nil {
			return 0, 0, fmt.Errorf("save raw payload: %w", err)
		}
	}
	return importedCount, updatedCount, nil
}

func (s *BankSyncService) resolveProviderAccountForTransaction(
	ctx context.Context,
	connectionID string,
	accountMap map[string]domain.ConnectionProviderAccount,
	providerAccountID string,
) (domain.ConnectionProviderAccount, error) {
	if providerAccount, ok := accountMap[providerAccountID]; ok {
		return providerAccount, nil
	}
	accounts, err := s.store.ListConnectionProviderAccounts(ctx, connectionID)
	if err != nil {
		return domain.ConnectionProviderAccount{}, fmt.Errorf("list provider accounts: %w", err)
	}
	for _, account := range accounts {
		if account.ProviderAccountID == providerAccountID {
			return account, nil
		}
	}
	return domain.ConnectionProviderAccount{}, errors.New("provider account not found for transaction")
}

func (s *BankSyncService) persistProviderRawPayloads(
	ctx context.Context,
	connectionID string,
	payloads []ProviderRawPayload,
	now time.Time,
) error {
	for _, payload := range payloads {
		_, err := s.store.SaveRawPayload(ctx, domain.RawPayload{
			ID:               s.newID(),
			ConnectionID:     connectionID,
			Scope:            payload.Scope,
			ProviderObjectID: payload.ProviderObjectID,
			PayloadJSON:      payload.PayloadJSON,
			CapturedAt:       now,
		})
		if err != nil {
			return fmt.Errorf("save raw payload: %w", err)
		}
	}
	return nil
}

func (s *BankSyncService) completeAppliedSync(
	ctx context.Context,
	connection *domain.BankConnection,
	params ApplyProviderSyncResultParams,
	now time.Time,
	atomic bool,
) error {
	connection.LastSyncJobID = strings.TrimSpace(params.JobID)
	if connection.LastSyncStartedAt == nil {
		connection.LastSyncStartedAt = &now
	}
	connection.LastSuccessfulSyncAt = &now
	connection.LastSyncError = ""
	if params.Result.Reauth != nil {
		connection.State = domain.BankConnectionStateReauthRequired
		connection.Reauth = params.Result.Reauth
	} else {
		connection.State = domain.BankConnectionStateActive
		connection.Reauth = nil
	}
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
		schedule.LastJobID = strings.TrimSpace(params.JobID)
		if params.Result.ScheduledRun != nil {
			schedule.LastScheduledAt = &params.Result.ScheduledRun.ScheduledAt
			schedule.NextRunAt = params.Result.ScheduledRun.NextRunAt
		}
		schedule.UpdatedAt = now
		if _, saveErr := s.store.SaveBankConnectionSchedule(ctx, *schedule); saveErr != nil {
			return fmt.Errorf("save bank connection schedule: %w", saveErr)
		}
	}
	if atomic || strings.TrimSpace(params.Result.SyncKey) == "" {
		return nil
	}
	_, err = s.store.SaveBankConnectionSyncRun(ctx, domain.BankConnectionSyncRun{
		ID:           s.newID(),
		ConnectionID: connection.ID,
		SyncKey:      params.Result.SyncKey,
		JobID:        strings.TrimSpace(params.JobID),
		CreatedAt:    now,
	})
	if err != nil {
		return fmt.Errorf("save bank connection sync run: %w", err)
	}
	return nil
}
