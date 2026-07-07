package providers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/google/uuid"
)

var (
	_ WindowSyncStore = (*ProviderWindowSyncStore)(nil)

	ErrWindowSyncPersistenceRequired = errors.New("window sync persistence is required")
)

type ProviderWindowSyncStoreOption func(*ProviderWindowSyncStore)

type ProviderWindowSyncStore struct {
	persistence WindowSyncPersistence
	idGenerator func() string
	now         func() time.Time
}

func WithWindowSyncStoreIDGenerator(idGenerator func() string) ProviderWindowSyncStoreOption {
	return func(store *ProviderWindowSyncStore) {
		store.idGenerator = idGenerator
	}
}

func WithWindowSyncStoreNow(now func() time.Time) ProviderWindowSyncStoreOption {
	return func(store *ProviderWindowSyncStore) {
		store.now = now
	}
}

func NewProviderWindowSyncStore(
	persistence WindowSyncPersistence,
	opts ...ProviderWindowSyncStoreOption,
) (*ProviderWindowSyncStore, error) {
	store := &ProviderWindowSyncStore{persistence: persistence}
	for _, opt := range opts {
		if opt != nil {
			opt(store)
		}
	}
	if store.persistence == nil {
		return nil, ErrWindowSyncPersistenceRequired
	}
	if store.idGenerator == nil {
		store.idGenerator = uuid.NewString
	}
	if store.now == nil {
		store.now = func() time.Time {
			return time.Now().UTC()
		}
	}
	return store, nil
}

func (s *ProviderWindowSyncStore) LoadExistingWindow(
	ctx context.Context,
	connection domain.ProviderConnectionRef,
	window domain.ProviderSyncWindow,
) (ExistingWindowSnapshot, error) {
	accounts, err := s.persistence.ListConnectionProviderAccounts(ctx, connection.ConnectionID)
	if err != nil {
		return ExistingWindowSnapshot{}, fmt.Errorf("list connection provider accounts: %w", err)
	}

	transactions, err := s.persistence.ListProviderTransactionsInWindow(
		ctx,
		mappedFinanceAccountIDs(accounts),
		window,
	)
	if err != nil {
		return ExistingWindowSnapshot{}, fmt.Errorf("list provider transactions in window: %w", err)
	}

	matches, err := s.persistence.ListProviderTransactionMatchesByTransactionIDs(
		ctx,
		connection.ConnectionID,
		transactionIDs(transactions),
	)
	if err != nil {
		return ExistingWindowSnapshot{}, fmt.Errorf(
			"list provider transaction matches by transaction ids: %w",
			err,
		)
	}

	return ExistingWindowSnapshot{
		Connection:     connection,
		SnapshotWindow: window,
		Accounts:       append([]domain.ConnectionProviderAccount(nil), accounts...),
		Transactions:   append([]domain.Transaction(nil), transactions...),
		Matches:        append([]domain.ProviderTransactionMatch(nil), matches...),
	}, nil
}

func (s *ProviderWindowSyncStore) ApplySync(
	ctx context.Context,
	diffPlan ProviderDiffPlan,
	applyPlan ApplyPlan,
) error {
	snapshot, err := s.LoadExistingWindow(ctx, diffPlan.Connection, diffPlan.SnapshotWindow)
	if err != nil {
		return fmt.Errorf("load existing apply snapshot: %w", err)
	}

	now := s.now().UTC()
	return s.persistence.WithTransaction(ctx, func(store WindowSyncApplyStore) error {
		providerAccounts := providerAccountsByProviderID(snapshot.Accounts)
		err = s.saveObservedAccounts(
			ctx,
			store,
			providerAccounts,
			diffPlan.Connection,
			diffPlan.AccountObservations,
			now,
		)
		if err != nil {
			return err
		}
		err = s.saveBalanceSnapshots(
			ctx,
			store,
			providerAccounts,
			diffPlan.Connection,
			diffPlan.BalanceObservations,
		)
		if err != nil {
			return err
		}
		err = s.saveRawPayloads(
			ctx,
			store,
			diffPlan.Connection,
			diffPlan.RawPayloadObservations,
		)
		if err != nil {
			return err
		}
		return s.saveTransactionWrites(
			ctx,
			store,
			providerAccounts,
			diffPlan.Connection,
			snapshot,
			applyPlan.TransactionWrites,
			now,
		)
	})
}

func (s *ProviderWindowSyncStore) saveObservedAccounts(
	ctx context.Context,
	store WindowSyncApplyStore,
	providerAccounts map[string]domain.ConnectionProviderAccount,
	connection domain.ProviderConnectionRef,
	observations []domain.ProviderAccountObservation,
	now time.Time,
) error {
	for _, observation := range observations {
		existingAccount, err := resolveProviderAccount(providerAccounts, observation.ProviderAccountID)
		if err != nil {
			return err
		}
		account, err := s.buildObservedProviderAccount(providerAccounts, connection, observation, now)
		if err != nil {
			return err
		}
		savedAccount, err := store.SaveConnectionProviderAccount(ctx, account)
		if err != nil {
			return fmt.Errorf("save connection provider account: %w", err)
		}
		if err = s.refreshLinkedFinanceAccount(ctx, store, existingAccount, savedAccount, now); err != nil {
			return err
		}
		providerAccounts[savedAccount.ProviderAccountID] = savedAccount
	}
	return nil
}

func (s *ProviderWindowSyncStore) refreshLinkedFinanceAccount(
	ctx context.Context,
	store WindowSyncApplyStore,
	existing domain.ConnectionProviderAccount,
	observed domain.ConnectionProviderAccount,
	now time.Time,
) error {
	if observed.FinanceAccountID == "" {
		return nil
	}
	account, err := store.GetAccount(ctx, observed.FinanceAccountID)
	if err != nil {
		return fmt.Errorf("get linked finance account: %w", err)
	}
	if account == nil || account.LinkedAccount == nil ||
		account.LinkedAccount.ProviderAccountID != observed.ProviderAccountID {
		return nil
	}

	updated := *account
	if shouldRefreshLinkedAccountName(account.Name, existing, observed) {
		updated.Name = providerAccountDisplayName(observed)
	}
	if shouldRefreshLinkedAccountCurrency(account.Currency, existing) {
		updated.Currency = strings.ToUpper(strings.TrimSpace(observed.Currency))
	}
	if updated.Name == account.Name && updated.Currency == account.Currency {
		return nil
	}
	updated.UpdatedAt = now
	if _, err = store.SaveAccount(ctx, updated); err != nil {
		return fmt.Errorf("save linked finance account: %w", err)
	}
	return nil
}

func shouldRefreshLinkedAccountName(
	current string,
	existing domain.ConnectionProviderAccount,
	observed domain.ConnectionProviderAccount,
) bool {
	trimmed := strings.TrimSpace(current)
	return trimmed == "" || trimmed == strings.TrimSpace(existing.Name) ||
		trimmed == strings.TrimSpace(observed.ProviderAccountID)
}

func shouldRefreshLinkedAccountCurrency(
	current string,
	existing domain.ConnectionProviderAccount,
) bool {
	trimmed := strings.ToUpper(strings.TrimSpace(current))
	return trimmed == "" || trimmed == strings.ToUpper(strings.TrimSpace(existing.Currency))
}

func providerAccountDisplayName(account domain.ConnectionProviderAccount) string {
	for _, value := range []string{account.Name, account.IBAN, account.ProviderAccountID} {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (s *ProviderWindowSyncStore) saveBalanceSnapshots(
	ctx context.Context,
	store WindowSyncApplyStore,
	providerAccounts map[string]domain.ConnectionProviderAccount,
	connection domain.ProviderConnectionRef,
	observations []domain.ProviderBalanceObservation,
) error {
	for _, observation := range observations {
		providerAccount, err := resolveProviderAccount(providerAccounts, observation.ProviderAccountID)
		if err != nil {
			return err
		}
		_, err = store.SaveBalanceSnapshot(ctx, domain.BalanceSnapshot{
			ID:                    s.idGenerator(),
			ConnectionID:          connection.ConnectionID,
			ProviderAccountID:     observation.ProviderAccountID,
			FinanceAccountID:      providerAccount.FinanceAccountID,
			Currency:              observation.Currency,
			CurrentBalanceMinor:   observation.CurrentBalanceMinor,
			AvailableBalanceMinor: observation.AvailableBalanceMinor,
			CapturedAt:            observation.CapturedAt,
		})
		if err != nil {
			return fmt.Errorf("save balance snapshot: %w", err)
		}
	}
	return nil
}

func (s *ProviderWindowSyncStore) saveRawPayloads(
	ctx context.Context,
	store WindowSyncApplyStore,
	connection domain.ProviderConnectionRef,
	observations []domain.ProviderRawPayloadObservation,
) error {
	for _, observation := range observations {
		_, err := store.SaveRawPayload(ctx, domain.RawPayload{
			ID:               s.idGenerator(),
			ConnectionID:     connection.ConnectionID,
			Scope:            observation.Scope,
			ProviderObjectID: observation.ProviderObjectID,
			PayloadJSON:      append([]byte(nil), observation.PayloadJSON...),
			CapturedAt:       observation.CapturedAt,
		})
		if err != nil {
			return fmt.Errorf("save raw payload: %w", err)
		}
	}
	return nil
}

func (s *ProviderWindowSyncStore) saveTransactionWrites(
	ctx context.Context,
	store WindowSyncApplyStore,
	providerAccounts map[string]domain.ConnectionProviderAccount,
	connection domain.ProviderConnectionRef,
	snapshot ExistingWindowSnapshot,
	writes []ApplyTransactionWrite,
	now time.Time,
) error {
	for _, write := range writes {
		transaction, err := s.buildTransactionWrite(write, providerAccounts, snapshot, now)
		if err != nil {
			return err
		}
		savedTransaction, err := store.SaveTransaction(ctx, transaction)
		if err != nil {
			return fmt.Errorf("save transaction: %w", err)
		}
		match := s.buildProviderTransactionMatch(
			connection,
			write,
			savedTransaction,
			snapshot.Matches,
			now,
		)
		if _, err = store.SaveProviderTransactionMatch(ctx, match); err != nil {
			return fmt.Errorf("save provider transaction match: %w", err)
		}
	}
	return nil
}

func (s *ProviderWindowSyncStore) buildObservedProviderAccount(
	existingAccounts map[string]domain.ConnectionProviderAccount,
	connection domain.ProviderConnectionRef,
	observation domain.ProviderAccountObservation,
	now time.Time,
) (domain.ConnectionProviderAccount, error) {
	existing, err := resolveProviderAccount(existingAccounts, observation.ProviderAccountID)
	if err != nil {
		return domain.ConnectionProviderAccount{}, err
	}

	return domain.ConnectionProviderAccount{
		ID:                   existing.ID,
		ConnectionID:         connection.ConnectionID,
		ProviderAccountID:    observation.ProviderAccountID,
		FinanceAccountID:     existing.FinanceAccountID,
		Name:                 observation.Name,
		Currency:             observation.Currency,
		IBAN:                 observation.IBAN,
		MaskedPAN:            observation.MaskedPAN,
		LastSuccessfulSyncAt: timePointerUTC(now),
		CreatedAt:            existing.CreatedAt,
		UpdatedAt:            now,
	}, nil
}

func (s *ProviderWindowSyncStore) buildTransactionWrite(
	write ApplyTransactionWrite,
	providerAccounts map[string]domain.ConnectionProviderAccount,
	snapshot ExistingWindowSnapshot,
	now time.Time,
) (domain.Transaction, error) {
	if write.MergedTransaction != nil {
		merged := *write.MergedTransaction
		merged.UpdatedAt = now
		return merged, nil
	}

	providerAccount, err := resolveProviderAccount(
		providerAccounts,
		write.Action.Observation.ProviderAccountID,
	)
	if err != nil {
		return domain.Transaction{}, err
	}

	tenantID, err := tenantIDForFinanceAccount(
		providerAccount.FinanceAccountID,
		snapshot.Transactions,
	)
	if err != nil {
		return domain.Transaction{}, err
	}

	return domain.Transaction{
		ID:               s.idGenerator(),
		TenantID:         tenantID,
		AccountID:        providerAccount.FinanceAccountID,
		Source:           domain.TransactionSourceProvider,
		Status:           write.Action.Observation.Status,
		Kind:             domain.TransactionKindRegular,
		AmountMinor:      write.Action.Observation.AmountMinor,
		Currency:         write.Action.Observation.Currency,
		Description:      write.Action.Observation.Description,
		EffectiveAt:      write.Action.Observation.EffectiveAt,
		CreatedAt:        now,
		UpdatedAt:        now,
		ProviderOriginal: buildProviderOriginal(write.Action.Observation),
	}, nil
}

func (s *ProviderWindowSyncStore) buildProviderTransactionMatch(
	connection domain.ProviderConnectionRef,
	write ApplyTransactionWrite,
	transaction domain.Transaction,
	snapshotMatches []domain.ProviderTransactionMatch,
	now time.Time,
) domain.ProviderTransactionMatch {
	existingMatch := existingSnapshotMatch(write.Action, snapshotMatches)
	matchID, createdAt := s.providerTransactionMatchIdentity(existingMatch, now)
	match := domain.ProviderTransactionMatch{
		ID:                    matchID,
		ConnectionID:          connection.ConnectionID,
		ProviderAccountID:     write.Action.Observation.ProviderAccountID,
		ProviderTransactionID: write.Action.Observation.ProviderTransactionID,
		Fingerprint:           write.Action.Observation.Fingerprint,
		TransactionID:         transaction.ID,
		Status:                transaction.Status,
		CreatedAt:             createdAt,
		UpdatedAt:             now,
	}
	return match
}

func (s *ProviderWindowSyncStore) providerTransactionMatchIdentity(
	existingMatch *domain.ProviderTransactionMatch,
	now time.Time,
) (string, time.Time) {
	if existingMatch != nil {
		return existingMatch.ID, existingMatch.CreatedAt
	}
	return s.idGenerator(), now
}

func providerAccountsByProviderID(
	accounts []domain.ConnectionProviderAccount,
) map[string]domain.ConnectionProviderAccount {
	result := make(map[string]domain.ConnectionProviderAccount, len(accounts))
	for _, account := range accounts {
		result[account.ProviderAccountID] = account
	}
	return result
}

func mappedFinanceAccountIDs(accounts []domain.ConnectionProviderAccount) []string {
	result := make([]string, 0, len(accounts))
	seen := make(map[string]struct{}, len(accounts))
	for _, account := range accounts {
		if account.FinanceAccountID == "" {
			continue
		}
		if _, ok := seen[account.FinanceAccountID]; ok {
			continue
		}
		seen[account.FinanceAccountID] = struct{}{}
		result = append(result, account.FinanceAccountID)
	}
	return result
}

func transactionIDs(transactions []domain.Transaction) []string {
	result := make([]string, 0, len(transactions))
	seen := make(map[string]struct{}, len(transactions))
	for _, transaction := range transactions {
		if transaction.ID == "" {
			continue
		}
		if _, ok := seen[transaction.ID]; ok {
			continue
		}
		seen[transaction.ID] = struct{}{}
		result = append(result, transaction.ID)
	}
	return result
}

func resolveProviderAccount(
	providerAccounts map[string]domain.ConnectionProviderAccount,
	providerAccountID string,
) (domain.ConnectionProviderAccount, error) {
	account, ok := providerAccounts[providerAccountID]
	if !ok || account.FinanceAccountID == "" {
		return domain.ConnectionProviderAccount{}, fmt.Errorf(
			"provider account mapping not found: %s",
			providerAccountID,
		)
	}
	return account, nil
}

func tenantIDForFinanceAccount(
	financeAccountID string,
	transactions []domain.Transaction,
) (string, error) {
	for _, transaction := range transactions {
		if transaction.AccountID == financeAccountID && transaction.TenantID != "" {
			return transaction.TenantID, nil
		}
	}
	return "", fmt.Errorf("tenant id not found for finance account: %s", financeAccountID)
}

func existingSnapshotMatch(
	action ProviderTransactionAction,
	matches []domain.ProviderTransactionMatch,
) *domain.ProviderTransactionMatch {
	if action.ExistingTransaction == nil {
		return nil
	}

	candidates := make([]domain.ProviderTransactionMatch, 0, len(matches))
	for _, match := range matches {
		if match.TransactionID != action.ExistingTransaction.ID {
			continue
		}
		if match.ProviderAccountID != action.Observation.ProviderAccountID {
			continue
		}
		switch action.MatchStrategy {
		case ProviderTransactionMatchStrategyProviderID:
			if match.ProviderTransactionID != action.Observation.ProviderTransactionID {
				continue
			}
		case ProviderTransactionMatchStrategyFingerprint:
			if match.Fingerprint != action.Observation.Fingerprint {
				continue
			}
		case ProviderTransactionMatchStrategyNew,
			ProviderTransactionMatchStrategyWeakCandidate,
			ProviderTransactionMatchStrategyAmbiguous:
			continue
		default:
			continue
		}
		candidates = append(candidates, match)
	}
	if len(candidates) != 1 {
		return nil
	}
	match := candidates[0]
	return &match
}

func timePointerUTC(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utcValue := value.UTC()
	return &utcValue
}
