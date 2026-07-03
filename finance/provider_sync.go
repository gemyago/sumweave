package finance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/credentials"
	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
)

const (
	BankConnectionSyncJobType         = "finance.bank_connection_sync"
	BankConnectionSyncReasonManual    = "manual"
	BankConnectionSyncReasonScheduled = "scheduled"
	bankProviderMonobank              = "monobank"
	bankProviderPKO                   = "pko"
	bankConnectorEnableBanking        = "enable-banking"
	pendingBankConnectionLinkStartTTL = 15 * time.Minute
)

var (
	ErrBankConnectionNotFound                 = errors.New("bank connection not found")
	ErrPendingBankConnectionLinkStartNotFound = errors.New("pending bank connection link start not found")
	ErrBankProviderNotConfigured              = errors.New("bank provider not configured")
	ErrUnsupportedBankProvider                = errors.New("unsupported bank provider")
	ErrUnsupportedBankLinkingMethod           = errors.New("unsupported bank linking method")
)

type connectionSecretCipher interface {
	SealString(plaintext string) (credentials.Envelope, error)
	OpenString(envelope credentials.Envelope) (string, error)
}

type BankConnectionSyncJobEnqueuer interface {
	EnqueueBankConnectionSync(
		ctx context.Context,
		request BankConnectionSyncJobRequest,
	) (BankConnectionSyncJobRef, error)
}

type BankConnectionSyncJobRequest struct {
	JobType string
	Input   BankConnectionSyncJobInput
	Reason  string
	Actor   string
}

type BankConnectionSyncJobInput struct {
	ConnectionID string
	Reason       string
	WindowStart  *time.Time
	WindowEnd    *time.Time
}

type BankConnectionSyncJobRef struct {
	ID      string
	JobType string
}

type BankConnectionProvider interface {
	Name() string
	StartLink(ctx context.Context, params ProviderStartLinkParams) (ProviderLinkStart, error)
	FinishLink(ctx context.Context, params ProviderFinishLinkParams) (ProviderLinkResult, error)
	LinkToken(ctx context.Context, params ProviderTokenLinkParams) (ProviderTokenLinkResult, error)
	Sync(ctx context.Context, params ProviderSyncParams) (ProviderSyncResult, error)
}

type ProviderStartLinkParams struct {
	RedirectURL string
}

type ProviderFinishLinkParams struct {
	State string
	Code  string
	Start ProviderLinkStart
}

type ProviderTokenLinkParams struct {
	Token string
}

type ProviderLinkResult struct {
	DisplayName       string
	ProviderReference string
	ExternalID        string
	Secret            string
	State             domain.BankConnectionState
	RawPayloads       []ProviderRawPayload
}

type ProviderTokenLinkResult = ProviderLinkResult

type ProviderNormalizedAccount struct {
	ProviderAccountID     string
	Name                  string
	Currency              string
	IBAN                  string
	MaskedPAN             string
	CurrentBalanceMinor   *int64
	AvailableBalanceMinor *int64
}

type ProviderNormalizedTransaction struct {
	ProviderAccountID     string
	ProviderTransactionID string
	Status                domain.TransactionStatus
	AmountMinor           int64
	Currency              string
	Description           string
	EffectiveAt           time.Time
	Fingerprint           string
	ProviderOriginal      *domain.ProviderTransactionOriginal
	RawPayloadJSON        []byte
}

type ProviderScheduledRunMetadata struct {
	ScheduledAt time.Time
	NextRunAt   *time.Time
}

type ProviderSyncParams struct {
	Secret      string
	ExternalID  string
	WindowStart time.Time
	WindowEnd   time.Time
}

type ProviderSyncResult struct {
	SyncKey      string
	Accounts     []ProviderNormalizedAccount
	Transactions []ProviderNormalizedTransaction
	RawPayloads  []ProviderRawPayload
	Reauth       *domain.ConnectionReauthMetadata
	ScheduledRun *ProviderScheduledRunMetadata
}

type UpsertBankConnectionScheduleParams struct {
	ActorUserID  string
	TenantID     string
	ConnectionID string
	Interval     time.Duration
	NextRunAt    time.Time
}

type PauseBankConnectionScheduleParams struct {
	ActorUserID  string
	TenantID     string
	ConnectionID string
}

type ResumeBankConnectionScheduleParams struct {
	ActorUserID  string
	TenantID     string
	ConnectionID string
	NextRunAt    time.Time
}

type TriggerBankConnectionSyncParams struct {
	ActorUserID  string
	TenantID     string
	ConnectionID string
	Reason       string
	WindowStart  *time.Time
	WindowEnd    *time.Time
}

type DeleteBankConnectionParams struct {
	ActorUserID  string
	TenantID     string
	ConnectionID string
}

type RunBankConnectionSyncParams struct {
	ConnectionID string
	JobID        string
	Reason       string
	WindowStart  time.Time
	WindowEnd    time.Time
}

type ApplyProviderSyncResultParams struct {
	ConnectionID string
	JobID        string
	Result       ProviderSyncResult
}

type BankConnectionSyncResult struct {
	ImportedAccounts     int
	ImportedTransactions int
	UpdatedTransactions  int
}

type ListBankConnectionsParams struct {
	ActorUserID string
	TenantID    string
}

type BankConnectionView struct {
	Connection domain.BankConnection
	Schedule   *domain.BankConnectionSchedule
}

type bankProviderRef struct {
	BankConnectionProvider

	bankID string
}

type bankLinkMethod string

const (
	bankLinkMethodToken    bankLinkMethod = "token"
	bankLinkMethodRedirect bankLinkMethod = "redirect"
)

type bankSyncStoreRef struct {
	bankSyncStore
}

type connectionSecretsStoreRef struct {
	connectionSecretStore
}

func WithConnectionSecretCipher(cipher connectionSecretCipher) ServiceOption {
	return func(service *Service) { service.connectionSecretCipher = cipher }
}

func WithBankProviders(providers ...BankConnectionProvider) ServiceOption {
	return func(service *Service) {
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

func WithBankSyncJobEnqueuer(enqueuer BankConnectionSyncJobEnqueuer) ServiceOption {
	return func(service *Service) { service.bankSyncJobEnqueuer = enqueuer }
}

type BankConnectionSyncScheduleWriter interface {
	UpsertBankConnectionSyncSchedule(context.Context, BankConnectionSyncSchedule) error
}

type BankConnectionSyncSchedule struct {
	ScheduleID   string
	ConnectionID string
	ActorUserID  string
	Interval     time.Duration
	NextRunAt    *time.Time
	Enabled      bool
}

func WithBankConnectionSyncScheduleWriter(writer BankConnectionSyncScheduleWriter) ServiceOption {
	return func(service *Service) { service.bankSyncScheduleWriter = writer }
}

func WithLogger(logger *slog.Logger) ServiceOption {
	return func(service *Service) {
		if logger != nil {
			service.logger = logger
		}
	}
}

type bankSyncStore interface {
	SaveBankConnection(
		ctx context.Context,
		connection domain.BankConnection,
	) (domain.BankConnection, error)
	GetBankConnection(ctx context.Context, connectionID string) (*domain.BankConnection, error)
	ListBankConnections(ctx context.Context, tenantID string) ([]domain.BankConnection, error)
	DeleteBankConnection(ctx context.Context, connectionID string) error
	SavePendingBankConnectionLinkStart(
		ctx context.Context,
		start domain.PendingBankConnectionLinkStart,
	) (domain.PendingBankConnectionLinkStart, error)
	ConsumePendingBankConnectionLinkStart(
		ctx context.Context,
		tenantID string,
		actorUserID string,
		provider string,
		state string,
		consumedAt time.Time,
	) (*domain.PendingBankConnectionLinkStart, error)
	RestorePendingBankConnectionLinkStart(
		ctx context.Context,
		tenantID string,
		actorUserID string,
		provider string,
		state string,
		restoredAt time.Time,
	) error
	GetPendingBankConnectionLinkStartByState(
		ctx context.Context,
		provider string,
		state string,
	) (*domain.PendingBankConnectionLinkStart, error)
	SaveBankConnectionSchedule(
		ctx context.Context,
		schedule domain.BankConnectionSchedule,
	) (domain.BankConnectionSchedule, error)
	GetBankConnectionSchedule(
		ctx context.Context,
		connectionID string,
	) (*domain.BankConnectionSchedule, error)
	DeleteBankConnectionSchedule(ctx context.Context, connectionID string) error
	SaveConnectionProviderAccount(
		ctx context.Context,
		account domain.ConnectionProviderAccount,
	) (domain.ConnectionProviderAccount, error)
	ListConnectionProviderAccounts(
		ctx context.Context,
		connectionID string,
	) ([]domain.ConnectionProviderAccount, error)
	DeleteConnectionProviderAccounts(ctx context.Context, connectionID string) error
	SaveBalanceSnapshot(
		ctx context.Context,
		snapshot domain.BalanceSnapshot,
	) (domain.BalanceSnapshot, error)
	ListBalanceSnapshots(ctx context.Context, connectionID string) ([]domain.BalanceSnapshot, error)
	DeleteBalanceSnapshots(ctx context.Context, connectionID string) error
	SaveRawPayload(ctx context.Context, payload domain.RawPayload) (domain.RawPayload, error)
	ListRawPayloads(ctx context.Context, connectionID string) ([]domain.RawPayload, error)
	DeleteRawPayloads(ctx context.Context, connectionID string) error
	SaveBankConnectionSyncRun(
		ctx context.Context,
		run domain.BankConnectionSyncRun,
	) (domain.BankConnectionSyncRun, error)
	ClaimBankConnectionSyncRun(
		ctx context.Context,
		run domain.BankConnectionSyncRun,
	) (bool, error)
	GetBankConnectionSyncRun(
		ctx context.Context,
		connectionID string,
		syncKey string,
	) (*domain.BankConnectionSyncRun, error)
	DeleteBankConnectionSyncRuns(ctx context.Context, connectionID string) error
	GetProviderTransactionMatchByProviderID(
		ctx context.Context,
		connectionID string,
		providerAccountID string,
		providerTransactionID string,
	) (*domain.ProviderTransactionMatch, error)
	GetProviderTransactionMatchByFingerprint(
		ctx context.Context,
		connectionID string,
		providerAccountID string,
		fingerprint string,
	) (*domain.ProviderTransactionMatch, error)
	SaveProviderTransactionMatch(
		ctx context.Context,
		match domain.ProviderTransactionMatch,
	) (domain.ProviderTransactionMatch, error)
	DeleteProviderTransactionMatches(ctx context.Context, connectionID string) error
}

type connectionSecretStore interface {
	SaveConnectionSecret(
		ctx context.Context,
		secret domain.ConnectionSecret,
	) (domain.ConnectionSecret, error)
	GetConnectionSecret(ctx context.Context, secretID string) (*domain.ConnectionSecret, error)
	DeleteConnectionSecret(ctx context.Context, secretID string) error
}

func (s *Service) LinkTokenBankConnection(
	ctx context.Context,
	params LinkTokenBankConnectionParams,
) (domain.BankConnection, error) {
	if err := s.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return domain.BankConnection{}, err
	}
	provider, err := s.bankProviderForLink(params.Provider, bankLinkMethodToken)
	if err != nil {
		return domain.BankConnection{}, err
	}
	result, err := provider.LinkToken(
		ctx,
		ProviderTokenLinkParams{Token: strings.TrimSpace(params.Token)},
	)
	if err != nil {
		return domain.BankConnection{}, fmt.Errorf("link token bank connection: %w", err)
	}
	connection, err := s.saveLinkedBankConnection(
		ctx,
		params.TenantID,
		provider.bankID,
		domain.ProviderConnectorID(strings.TrimSpace(provider.Name())),
		result,
	)
	if err != nil {
		return domain.BankConnection{}, err
	}
	s.logger.InfoContext(
		ctx,
		"linked bank connection",
		"connection_id",
		connection.ID,
		"provider",
		connection.Provider,
	)
	return connection, nil
}

func (s *Service) UpsertBankConnectionSchedule(
	ctx context.Context,
	params UpsertBankConnectionScheduleParams,
) (domain.BankConnectionSchedule, error) {
	connection, err := s.requireTenantBankConnection(
		ctx,
		params.TenantID,
		params.ActorUserID,
		params.ConnectionID,
	)
	if err != nil {
		return domain.BankConnectionSchedule{}, err
	}
	now := s.now().UTC()
	syncStore, err := s.bankSyncStore()
	if err != nil {
		return domain.BankConnectionSchedule{}, err
	}
	existing, err := syncStore.GetBankConnectionSchedule(ctx, connection.ID)
	if err != nil && !errors.Is(err, persistence.ErrBankConnectionScheduleNotFound) {
		return domain.BankConnectionSchedule{}, fmt.Errorf(
			"upsert bank connection schedule: %w",
			err,
		)
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
	persisted, err := syncStore.SaveBankConnectionSchedule(ctx, schedule)
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

func (s *Service) PauseBankConnectionSchedule(
	ctx context.Context,
	params PauseBankConnectionScheduleParams,
) (domain.BankConnectionSchedule, error) {
	connection, err := s.requireTenantBankConnection(
		ctx,
		params.TenantID,
		params.ActorUserID,
		params.ConnectionID,
	)
	if err != nil {
		return domain.BankConnectionSchedule{}, err
	}
	syncStore, err := s.bankSyncStore()
	if err != nil {
		return domain.BankConnectionSchedule{}, err
	}
	schedule, err := syncStore.GetBankConnectionSchedule(ctx, connection.ID)
	if err != nil {
		return domain.BankConnectionSchedule{}, fmt.Errorf(
			"pause bank connection schedule: %w",
			err,
		)
	}
	schedule.Enabled = false
	schedule.UpdatedAt = s.now().UTC()
	persisted, err := syncStore.SaveBankConnectionSchedule(ctx, *schedule)
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

func (s *Service) ResumeBankConnectionSchedule(
	ctx context.Context,
	params ResumeBankConnectionScheduleParams,
) (domain.BankConnectionSchedule, error) {
	connection, err := s.requireTenantBankConnection(
		ctx,
		params.TenantID,
		params.ActorUserID,
		params.ConnectionID,
	)
	if err != nil {
		return domain.BankConnectionSchedule{}, err
	}
	syncStore, err := s.bankSyncStore()
	if err != nil {
		return domain.BankConnectionSchedule{}, err
	}
	schedule, err := syncStore.GetBankConnectionSchedule(ctx, connection.ID)
	if err != nil {
		return domain.BankConnectionSchedule{}, fmt.Errorf(
			"resume bank connection schedule: %w",
			err,
		)
	}
	schedule.Enabled = true
	schedule.NextRunAt = timePtrUTC(params.NextRunAt)
	schedule.UpdatedAt = s.now().UTC()
	persisted, err := syncStore.SaveBankConnectionSchedule(ctx, *schedule)
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

func (s *Service) writeBankConnectionSyncSchedule(
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

func (s *Service) disableBankConnectionSyncSchedule(
	ctx context.Context,
	connection domain.BankConnection,
	actorUserID string,
	syncStore *bankSyncStoreRef,
) error {
	if syncStore == nil {
		return nil
	}
	schedule, err := syncStore.GetBankConnectionSchedule(ctx, connection.ID)
	if err != nil && !errors.Is(err, persistence.ErrBankConnectionScheduleNotFound) {
		return fmt.Errorf("disable bank connection sync schedule: %w", err)
	}
	interval := time.Duration(0)
	if schedule != nil {
		interval = schedule.Interval
	}
	if writeErr := s.writeBankConnectionSyncSchedule(ctx, BankConnectionSyncSchedule{
		ScheduleID:   bankConnectionSyncScheduleID(connection.ID),
		ConnectionID: connection.ID,
		ActorUserID:  actorUserID,
		Interval:     interval,
		Enabled:      false,
	}); writeErr != nil {
		return writeErr
	}
	return nil
}

func bankConnectionSyncScheduleID(connectionID string) string {
	return "finance.bank_connection_sync:" + strings.TrimSpace(connectionID)
}

func (s *Service) TriggerBankConnectionSync(
	ctx context.Context,
	params TriggerBankConnectionSyncParams,
) (BankConnectionSyncJobRef, error) {
	if s.bankSyncJobEnqueuer == nil {
		return BankConnectionSyncJobRef{}, errors.New("bank sync job enqueuer is required")
	}
	connection, err := s.requireTenantBankConnection(
		ctx,
		params.TenantID,
		params.ActorUserID,
		params.ConnectionID,
	)
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

func (s *Service) DeleteBankConnection(
	ctx context.Context,
	params DeleteBankConnectionParams,
) error {
	connection, err := s.requireTenantBankConnection(
		ctx,
		params.TenantID,
		params.ActorUserID,
		params.ConnectionID,
	)
	if err != nil {
		return err
	}
	syncStore, err := s.bankSyncStore()
	if err != nil {
		return err
	}
	if disableErr := s.disableBankConnectionSyncSchedule(
		ctx,
		connection,
		params.ActorUserID,
		syncStore,
	); disableErr != nil {
		return disableErr
	}
	deleteConnection := func(service *Service) error {
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

func (s *Service) RunBankConnectionSync(
	ctx context.Context,
	params RunBankConnectionSyncParams,
) (BankConnectionSyncResult, error) {
	syncStore, err := s.bankSyncStore()
	if err != nil {
		return BankConnectionSyncResult{}, err
	}
	connection, err := syncStore.GetBankConnection(ctx, strings.TrimSpace(params.ConnectionID))
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
	scheduledRun, hasScheduledRun, err := s.makeScheduledRunMetadata(
		ctx,
		syncStore,
		*connection,
		params,
		now,
	)
	if err != nil {
		return BankConnectionSyncResult{}, err
	}
	markErr := s.markBankConnectionSyncStarted(
		ctx,
		syncStore,
		connection,
		params,
		now,
		scheduledRun,
	)
	if markErr != nil {
		return BankConnectionSyncResult{}, markErr
	}
	result, err := provider.Sync(ctx, ProviderSyncParams{
		Secret:      secret,
		ExternalID:  connection.ExternalID,
		WindowStart: params.WindowStart,
		WindowEnd:   params.WindowEnd,
	})
	if err != nil {
		return BankConnectionSyncResult{}, s.recordBankConnectionSyncFailure(
			ctx,
			syncStore,
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
			syncStore,
			connection,
			params,
			now,
			scheduledRun,
			err,
		)
	}
	return applyResult, nil
}

func (s *Service) bankProviderForSync(bankID string) (*bankProviderRef, error) {
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

func (s *Service) ApplyProviderSyncResult(
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

func (s *Service) applyProviderSyncResult(
	ctx context.Context,
	params ApplyProviderSyncResultParams,
	atomic bool,
) (BankConnectionSyncResult, error) {
	syncStore, err := s.bankSyncStore()
	if err != nil {
		return BankConnectionSyncResult{}, err
	}
	now := s.now().UTC()
	if atomic {
		claimed, claimErr := s.claimSyncRun(
			ctx,
			syncStore,
			params.ConnectionID,
			params.Result.SyncKey,
			params.JobID,
			now,
		)
		if claimErr != nil {
			return BankConnectionSyncResult{}, claimErr
		}
		if !claimed {
			return BankConnectionSyncResult{}, nil
		}
	}
	if !atomic {
		alreadyApplied, appliedErr := s.syncRunAlreadyApplied(
			ctx,
			syncStore,
			params.ConnectionID,
			params.Result.SyncKey,
		)
		if appliedErr != nil {
			return BankConnectionSyncResult{}, appliedErr
		}
		if alreadyApplied {
			return BankConnectionSyncResult{}, nil
		}
	}
	connection, err := syncStore.GetBankConnection(ctx, params.ConnectionID)
	if err != nil {
		return BankConnectionSyncResult{}, ErrBankConnectionNotFound
	}
	result := BankConnectionSyncResult{}
	accountMap, importedAccounts, err := s.applyProviderAccounts(
		ctx,
		syncStore,
		*connection,
		params.Result.Accounts,
		now,
	)
	if err != nil {
		return BankConnectionSyncResult{}, err
	}
	result.ImportedAccounts = importedAccounts
	importedTransactions, updatedTransactions, err := s.applyProviderTransactions(
		ctx,
		syncStore,
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
		syncStore,
		connection.ID,
		params.Result.RawPayloads,
		now,
	); persistErr != nil {
		return BankConnectionSyncResult{}, persistErr
	}
	if completeErr := s.completeAppliedSync(ctx, syncStore, connection, params, now, atomic); completeErr != nil {
		return BankConnectionSyncResult{}, completeErr
	}
	s.logger.InfoContext(
		ctx,
		"bank connection sync completed",
		"connection_id",
		connection.ID,
		"provider",
		connection.Provider,
	)
	return result, nil
}

func (s *Service) ListBankConnections(
	ctx context.Context,
	params ListBankConnectionsParams,
) ([]BankConnectionView, error) {
	if err := s.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return nil, err
	}
	syncStore, err := s.bankSyncStore()
	if err != nil {
		return nil, err
	}
	connections, err := syncStore.ListBankConnections(ctx, params.TenantID)
	if err != nil {
		return nil, fmt.Errorf("list bank connections: %w", err)
	}
	views := make([]BankConnectionView, 0, len(connections))
	for _, connection := range connections {
		schedule, scheduleErr := syncStore.GetBankConnectionSchedule(ctx, connection.ID)
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

func (s *Service) saveLinkedBankConnection(
	ctx context.Context,
	tenantID string,
	providerName string,
	connectorID domain.ProviderConnectorID,
	result ProviderLinkResult,
) (domain.BankConnection, error) {
	secretID, err := s.encryptAndSaveConnectionSecret(
		ctx,
		providerName,
		result.ProviderReference,
		result.Secret,
	)
	if err != nil {
		return domain.BankConnection{}, err
	}
	syncStore, err := s.bankSyncStore()
	if err != nil {
		return domain.BankConnection{}, err
	}
	reusedConnection := domain.BankConnection{}
	if strings.TrimSpace(providerName) == bankProviderPKO {
		connections, listErr := syncStore.ListBankConnections(ctx, tenantID)
		if listErr != nil {
			return domain.BankConnection{}, fmt.Errorf("list bank connections: %w", listErr)
		}
		for _, existingConnection := range connections {
			if existingConnection.Provider == providerName {
				reusedConnection = existingConnection
				break
			}
		}
	}
	now := s.now().UTC()
	connection := domain.BankConnection{
		ID:                s.newID(),
		TenantID:          strings.TrimSpace(tenantID),
		Provider:          providerName,
		ConnectorID:       connectorID,
		DisplayName:       strings.TrimSpace(result.DisplayName),
		ProviderReference: strings.TrimSpace(result.ProviderReference),
		ExternalID:        strings.TrimSpace(result.ExternalID),
		SecretID:          secretID,
		State:             result.State,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if reusedConnection.ID != "" {
		connection.ID = reusedConnection.ID
		connection.CreatedAt = reusedConnection.CreatedAt
	}
	saved, err := syncStore.SaveBankConnection(ctx, connection)
	if err != nil {
		return domain.BankConnection{}, fmt.Errorf("save bank connection: %w", err)
	}
	for _, payload := range result.RawPayloads {
		_, rawErr := syncStore.SaveRawPayload(ctx, domain.RawPayload{
			ID:               s.newID(),
			ConnectionID:     saved.ID,
			Scope:            payload.Scope,
			ProviderObjectID: payload.ProviderObjectID,
			PayloadJSON:      payload.PayloadJSON,
			CapturedAt:       now,
		})
		if rawErr != nil {
			return domain.BankConnection{}, fmt.Errorf("save raw payload: %w", rawErr)
		}
	}
	return saved, nil
}

func (s *Service) encryptAndSaveConnectionSecret(
	ctx context.Context,
	providerName string,
	reference string,
	plaintext string,
) (string, error) {
	if s.connectionSecretCipher == nil {
		return "", errors.New("connection secret cipher is required")
	}
	envelope, err := s.connectionSecretCipher.SealString(strings.TrimSpace(plaintext))
	if err != nil {
		return "", fmt.Errorf("seal connection secret: %w", err)
	}
	secretID := s.newID()
	secretStore, err := s.connectionSecretsStore()
	if err != nil {
		return "", err
	}
	_, err = secretStore.SaveConnectionSecret(ctx, domain.ConnectionSecret{
		ID:        secretID,
		Provider:  providerName,
		Reference: reference,
		Envelope:  envelope,
		CreatedAt: s.now().UTC(),
		UpdatedAt: s.now().UTC(),
	})
	if err != nil {
		return "", fmt.Errorf("save connection secret: %w", err)
	}
	return secretID, nil
}

func (s *Service) decryptConnectionSecret(ctx context.Context, secretID string) (string, error) {
	if s.connectionSecretCipher == nil {
		return "", errors.New("connection secret cipher is required")
	}
	secretStore, err := s.connectionSecretsStore()
	if err != nil {
		return "", err
	}
	secret, err := secretStore.GetConnectionSecret(ctx, secretID)
	if err != nil {
		return "", fmt.Errorf("get connection secret: %w", err)
	}
	plaintext, err := s.connectionSecretCipher.OpenString(secret.Envelope)
	if err != nil {
		return "", fmt.Errorf("open connection secret: %w", err)
	}
	return plaintext, nil
}

func (s *Service) bankProvider(name string) (*bankProviderRef, error) {
	trimmedName := strings.TrimSpace(name)
	provider, ok := s.bankProviders[trimmedName]
	if !ok {
		return nil, bankProviderNotConfiguredError(trimmedName)
	}
	return &bankProviderRef{BankConnectionProvider: provider, bankID: trimmedName}, nil
}

func (s *Service) bankProviderForLink(bankID string, method bankLinkMethod) (*bankProviderRef, error) {
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

func bankProviderNotConfiguredError(providerName string) error {
	return fmt.Errorf("%w: %s", ErrBankProviderNotConfigured, strings.TrimSpace(providerName))
}

func bankProviderNotConfiguredForBankError(bankID string, providerName string) error {
	trimmedBankID := strings.TrimSpace(bankID)
	trimmedProviderName := strings.TrimSpace(providerName)
	if trimmedBankID == trimmedProviderName || trimmedProviderName == "" {
		return bankProviderNotConfiguredError(trimmedBankID)
	}
	return fmt.Errorf(
		"%w: bank provider %s requires connector %s",
		ErrBankProviderNotConfigured,
		trimmedBankID,
		trimmedProviderName,
	)
}

func configuredBankProviderName(bankID string, method bankLinkMethod) (string, error) {
	switch strings.TrimSpace(bankID) {
	case bankProviderMonobank:
		if method != bankLinkMethodToken {
			return "", unsupportedBankLinkingMethodError(bankProviderMonobank, method)
		}
		return bankProviderMonobank, nil
	case bankProviderPKO:
		if method != bankLinkMethodRedirect {
			return "", unsupportedBankLinkingMethodError(bankProviderPKO, method)
		}
		return bankConnectorEnableBanking, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedBankProvider, strings.TrimSpace(bankID))
	}
}

func unsupportedBankLinkingMethodError(bankID string, method bankLinkMethod) error {
	if method == bankLinkMethodToken {
		return fmt.Errorf(
			"%w: token linking unsupported for bank provider: %s",
			ErrUnsupportedBankLinkingMethod,
			bankID,
		)
	}
	return fmt.Errorf(
		"%w: redirect linking unsupported for bank provider: %s",
		ErrUnsupportedBankLinkingMethod,
		bankID,
	)
}

func (s *Service) requireTenantBankConnection(
	ctx context.Context,
	tenantID string,
	userID string,
	connectionID string,
) (domain.BankConnection, error) {
	if err := s.requireTenantMember(ctx, tenantID, userID); err != nil {
		return domain.BankConnection{}, err
	}
	syncStore, err := s.bankSyncStore()
	if err != nil {
		return domain.BankConnection{}, err
	}
	connection, err := syncStore.GetBankConnection(ctx, strings.TrimSpace(connectionID))
	if err != nil {
		return domain.BankConnection{}, ErrBankConnectionNotFound
	}
	if connection.TenantID != strings.TrimSpace(tenantID) {
		return domain.BankConnection{}, ErrBankConnectionNotFound
	}
	return *connection, nil
}

func (s *Service) upsertProviderAccount(
	ctx context.Context,
	connection domain.BankConnection,
	item ProviderNormalizedAccount,
	now time.Time,
) (domain.ConnectionProviderAccount, error) {
	syncStore, err := s.bankSyncStore()
	if err != nil {
		return domain.ConnectionProviderAccount{}, err
	}
	accounts, err := syncStore.ListConnectionProviderAccounts(ctx, connection.ID)
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
		financeAccount, accountErr := s.findOrCreateFinanceAccountForProviderAccount(
			ctx,
			connection,
			item,
			now,
		)
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
	return syncStore.SaveConnectionProviderAccount(ctx, providerAccount)
}

func (s *Service) findOrCreateFinanceAccountForProviderAccount(
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
		Name: firstNonEmpty(
			strings.TrimSpace(item.Name),
			strings.TrimSpace(item.IBAN),
			item.ProviderAccountID,
		),
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

func (s *Service) applyProviderTransaction(
	ctx context.Context,
	connection domain.BankConnection,
	providerAccount domain.ConnectionProviderAccount,
	item ProviderNormalizedTransaction,
	now time.Time,
) (bool, error) {
	var match *domain.ProviderTransactionMatch
	var err error
	syncStore, err := s.bankSyncStore()
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(item.ProviderTransactionID) != "" {
		match, err = syncStore.GetProviderTransactionMatchByProviderID(
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
		match, err = syncStore.GetProviderTransactionMatchByFingerprint(
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
	if _, saveErr := syncStore.SaveProviderTransactionMatch(ctx, providerMatch); saveErr != nil {
		return false, fmt.Errorf("save provider transaction match: %w", saveErr)
	}
	return updated, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func accountID(account *domain.ConnectionProviderAccount) string {
	if account == nil {
		return ""
	}
	return account.ID
}

func matchID(match *domain.ProviderTransactionMatch) string {
	if match == nil {
		return ""
	}
	return match.ID
}

func timePtrUTC(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utcValue := value.UTC()
	return &utcValue
}

func defaultLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func (s *Service) deleteBankConnectionOwnedMetadata(
	ctx context.Context,
	connection domain.BankConnection,
) error {
	syncStore, err := s.bankSyncStore()
	if err != nil {
		return err
	}
	secretStore, err := s.connectionSecretsStore()
	if err != nil {
		return err
	}
	for _, step := range []func(context.Context) error{
		func(ctx context.Context) error { return syncStore.DeleteProviderTransactionMatches(ctx, connection.ID) },
		func(ctx context.Context) error { return syncStore.DeleteBankConnectionSyncRuns(ctx, connection.ID) },
		func(ctx context.Context) error { return syncStore.DeleteRawPayloads(ctx, connection.ID) },
		func(ctx context.Context) error { return syncStore.DeleteBalanceSnapshots(ctx, connection.ID) },
		func(ctx context.Context) error { return syncStore.DeleteConnectionProviderAccounts(ctx, connection.ID) },
		func(ctx context.Context) error { return syncStore.DeleteBankConnectionSchedule(ctx, connection.ID) },
		func(ctx context.Context) error { return syncStore.DeleteBankConnection(ctx, connection.ID) },
		func(ctx context.Context) error { return secretStore.DeleteConnectionSecret(ctx, connection.SecretID) },
	} {
		if stepErr := step(ctx); stepErr != nil {
			return fmt.Errorf("delete bank connection: %w", stepErr)
		}
	}
	return nil
}

func (s *Service) bankSyncStore() (*bankSyncStoreRef, error) {
	syncStore, ok := s.store.(bankSyncStore)
	if !ok {
		return nil, errors.New("bank sync store is required")
	}
	return &bankSyncStoreRef{bankSyncStore: syncStore}, nil
}

func (s *Service) connectionSecretsStore() (*connectionSecretsStoreRef, error) {
	secretStore, ok := s.store.(connectionSecretStore)
	if !ok {
		return nil, errors.New("connection secret store is required")
	}
	return &connectionSecretsStoreRef{connectionSecretStore: secretStore}, nil
}

func (s *Service) syncRunAlreadyApplied(
	ctx context.Context,
	syncStore *bankSyncStoreRef,
	connectionID string,
	syncKey string,
) (bool, error) {
	if strings.TrimSpace(syncKey) == "" {
		return false, nil
	}
	existing, err := syncStore.GetBankConnectionSyncRun(ctx, connectionID, syncKey)
	if err != nil && !errors.Is(err, persistence.ErrBankConnectionSyncRunNotFound) {
		return false, fmt.Errorf("apply provider sync result: %w", err)
	}
	return existing != nil, nil
}

func (s *Service) claimSyncRun(
	ctx context.Context,
	syncStore *bankSyncStoreRef,
	connectionID string,
	syncKey string,
	jobID string,
	now time.Time,
) (bool, error) {
	if strings.TrimSpace(syncKey) == "" {
		return true, nil
	}
	claimed, err := syncStore.ClaimBankConnectionSyncRun(ctx, domain.BankConnectionSyncRun{
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

func (s *Service) makeScheduledRunMetadata(
	ctx context.Context,
	syncStore *bankSyncStoreRef,
	connection domain.BankConnection,
	params RunBankConnectionSyncParams,
	now time.Time,
) (*ProviderScheduledRunMetadata, bool, error) {
	if strings.TrimSpace(params.Reason) != BankConnectionSyncReasonScheduled {
		return nil, false, nil
	}
	metadata := &ProviderScheduledRunMetadata{ScheduledAt: now}
	schedule, err := syncStore.GetBankConnectionSchedule(ctx, connection.ID)
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

func (s *Service) markBankConnectionSyncStarted(
	ctx context.Context,
	syncStore *bankSyncStoreRef,
	connection *domain.BankConnection,
	params RunBankConnectionSyncParams,
	now time.Time,
	scheduledRun *ProviderScheduledRunMetadata,
) error {
	connection.LastSyncJobID = strings.TrimSpace(params.JobID)
	connection.LastSyncStartedAt = &now
	connection.LastSyncError = ""
	connection.UpdatedAt = now
	if _, err := syncStore.SaveBankConnection(ctx, *connection); err != nil {
		return fmt.Errorf("save bank connection: %w", err)
	}
	schedule, err := syncStore.GetBankConnectionSchedule(ctx, connection.ID)
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
	_, saveErr := syncStore.SaveBankConnectionSchedule(ctx, *schedule)
	if saveErr != nil {
		return fmt.Errorf("save bank connection schedule: %w", saveErr)
	}
	return nil
}

func (s *Service) recordBankConnectionSyncFailure(
	ctx context.Context,
	syncStore *bankSyncStoreRef,
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
	if _, err := syncStore.SaveBankConnection(ctx, *connection); err != nil {
		return fmt.Errorf("save bank connection: %w", err)
	}
	schedule, err := syncStore.GetBankConnectionSchedule(ctx, connection.ID)
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
	_, saveErr := syncStore.SaveBankConnectionSchedule(ctx, *schedule)
	if saveErr != nil {
		return fmt.Errorf("save bank connection schedule: %w", saveErr)
	}
	return fmt.Errorf("run bank connection sync: %w", syncErr)
}

func (s *Service) applyProviderAccounts(
	ctx context.Context,
	syncStore bankSyncStore,
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
		_, err = syncStore.SaveBalanceSnapshot(ctx, domain.BalanceSnapshot{
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

func (s *Service) applyProviderTransactions(
	ctx context.Context,
	syncStore bankSyncStore,
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
			syncStore,
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
		_, err = syncStore.SaveRawPayload(ctx, domain.RawPayload{
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

func (s *Service) resolveProviderAccountForTransaction(
	ctx context.Context,
	syncStore bankSyncStore,
	connectionID string,
	accountMap map[string]domain.ConnectionProviderAccount,
	providerAccountID string,
) (domain.ConnectionProviderAccount, error) {
	providerAccount, ok := accountMap[providerAccountID]
	if ok {
		return providerAccount, nil
	}
	accounts, err := syncStore.ListConnectionProviderAccounts(ctx, connectionID)
	if err != nil {
		return domain.ConnectionProviderAccount{}, fmt.Errorf("list provider accounts: %w", err)
	}
	for _, account := range accounts {
		if account.ProviderAccountID == providerAccountID {
			return account, nil
		}
	}
	return domain.ConnectionProviderAccount{}, errors.New(
		"provider account not found for transaction",
	)
}

func (s *Service) persistProviderRawPayloads(
	ctx context.Context,
	syncStore bankSyncStore,
	connectionID string,
	payloads []ProviderRawPayload,
	now time.Time,
) error {
	for _, payload := range payloads {
		_, err := syncStore.SaveRawPayload(ctx, domain.RawPayload{
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

func (s *Service) completeAppliedSync(
	ctx context.Context,
	syncStore *bankSyncStoreRef,
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
	if _, err := syncStore.SaveBankConnection(ctx, *connection); err != nil {
		return fmt.Errorf("save bank connection: %w", err)
	}
	schedule, err := syncStore.GetBankConnectionSchedule(ctx, connection.ID)
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
		if _, saveErr := syncStore.SaveBankConnectionSchedule(ctx, *schedule); saveErr != nil {
			return fmt.Errorf("save bank connection schedule: %w", saveErr)
		}
	}
	if atomic || strings.TrimSpace(params.Result.SyncKey) == "" {
		return nil
	}
	_, err = syncStore.SaveBankConnectionSyncRun(ctx, domain.BankConnectionSyncRun{
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
