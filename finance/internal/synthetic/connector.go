package synthetic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/internal/providers"
	"github.com/google/uuid"
)

var (
	ErrProviderStateStoreRequired       = errors.New("synthetic provider state store is required")
	ErrProviderStateNotFound            = errors.New("synthetic provider state not found")
	ErrConfiguredSyntheticStateRequired = errors.New("configured synthetic state is required")
	ErrConnectorLinkUnsupported         = errors.New("synthetic connector link operation unsupported")
)

const (
	syntheticFirstWindowMaxTransactionsPerInterval    = 2
	syntheticRepeatedWindowMaxTransactionsPerInterval = 3
	syntheticBalanceBaseMinor                         = 100_000
	syntheticWindowStep                               = 24 * time.Hour
	syntheticAmountBaseMinor                          = 100
	syntheticAmountRangeMinor                         = 900
	syntheticPositiveNegativeRange                    = 2
	syntheticFirstSequence                            = 1
	syntheticNextSequenceAfterFirst                   = 2
)

type ProviderStateStore interface {
	SaveSyntheticProviderState(
		ctx context.Context,
		state domain.SyntheticProviderState,
	) (domain.SyntheticProviderState, error)
	GetSyntheticProviderState(
		ctx context.Context,
		providerReference string,
	) (*domain.SyntheticProviderState, error)
}

type ConnectorOption func(*Connector)

type Connector struct {
	stateStore ProviderStateStore
	logger     *slog.Logger
	now        func() time.Time
	stateID    func() string
	randomIntn func(int) int
}

func WithConnectorLogger(logger *slog.Logger) ConnectorOption {
	return func(connector *Connector) {
		connector.logger = logger
	}
}

func WithConnectorNow(now func() time.Time) ConnectorOption {
	return func(connector *Connector) {
		connector.now = now
	}
}

func WithConnectorRandomIntn(randomIntn func(int) int) ConnectorOption {
	return func(connector *Connector) {
		connector.randomIntn = randomIntn
	}
}

func WithConnectorStateGenerator(stateID func() string) ConnectorOption {
	return func(connector *Connector) {
		connector.stateID = stateID
	}
}

func NewConnector(stateStore ProviderStateStore, opts ...ConnectorOption) *Connector {
	connector := &Connector{
		stateStore: stateStore,
		logger:     slog.New(slog.DiscardHandler),
		now:        time.Now,
		stateID:    uuid.NewString,
		randomIntn: func(_ int) int { return 0 },
	}
	for _, opt := range opts {
		if opt != nil {
			opt(connector)
		}
	}
	if connector.logger == nil {
		connector.logger = slog.New(slog.DiscardHandler)
	}
	if connector.now == nil {
		connector.now = time.Now
	}
	if connector.stateID == nil {
		connector.stateID = uuid.NewString
	}
	if connector.randomIntn == nil {
		connector.randomIntn = func(_ int) int { return 0 }
	}
	return connector
}

func (c *Connector) ConnectorID() domain.ProviderConnectorID {
	return domain.ProviderConnectorIDSynthetic
}

func (c *Connector) Capabilities() providers.ConnectorCapabilities {
	return providers.ConnectorCapabilities{
		SupportsStartLink:  true,
		SupportsFinishLink: true,
		SupportsFetch:      true,
	}
}

func (c *Connector) StartLink(
	_ context.Context,
	_ providers.StartLinkRequest,
) (providers.StartLinkResult, error) {
	state := strings.TrimSpace(c.stateID())
	if state == "" {
		state = uuid.NewString()
	}
	return providers.StartLinkResult{
		State:             state,
		ProviderReference: state,
		AuthorizationURL:  "#/finance/connections/synthetic?state=" + state,
	}, nil
}

func (c *Connector) FinishLink(
	ctx context.Context,
	request providers.FinishLinkRequest,
) (providers.LinkResult, error) {
	if c.stateStore == nil {
		return providers.LinkResult{}, ErrProviderStateStoreRequired
	}
	providerReference := strings.TrimSpace(request.State)
	state, err := c.stateStore.GetSyntheticProviderState(ctx, providerReference)
	if err != nil {
		return providers.LinkResult{}, fmt.Errorf("load synthetic provider state: %w", err)
	}
	if state == nil {
		return providers.LinkResult{}, ErrProviderStateNotFound
	}
	if !hasConfiguredAccounts(state.Envelope.ConfiguredAccounts) {
		return providers.LinkResult{}, ErrConfiguredSyntheticStateRequired
	}
	return providers.LinkResult{
		DisplayName:       ConnectionDisplayName,
		ProviderReference: providerReference,
		State:             domain.BankConnectionStateActive,
	}, nil
}

func (c *Connector) LinkToken(
	_ context.Context,
	_ providers.LinkTokenRequest,
) (providers.LinkResult, error) {
	return providers.LinkResult{}, ErrConnectorLinkUnsupported
}

func (c *Connector) Fetch(
	ctx context.Context,
	request providers.FetchRequest,
) (domain.ProviderSyncBatch, error) {
	if c.stateStore == nil {
		return domain.ProviderSyncBatch{}, ErrProviderStateStoreRequired
	}
	state, err := c.stateStore.GetSyntheticProviderState(ctx, request.Connection.ProviderReference)
	if err != nil {
		return domain.ProviderSyncBatch{}, fmt.Errorf("load synthetic provider state: %w", err)
	}
	if state == nil {
		return domain.ProviderSyncBatch{}, ErrProviderStateNotFound
	}

	windowKey := newWindowKey(request.RequestedWindow)
	repeatIndex := windowHistoryIndex(state.Envelope.WindowHistory, windowKey)
	mode := "firstWindow"
	generationInstants := windowInstants(windowKey)
	if repeatIndex >= 0 {
		mode = "repeatedWindow"
		if len(generationInstants) > 0 {
			generationInstants = generationInstants[len(generationInstants)-1:]
		}
	}

	updatedState := domain.SyntheticProviderState{
		ProviderReference: state.ProviderReference,
		Envelope:          cloneEnvelope(state.Envelope),
		CreatedAt:         state.CreatedAt,
		UpdatedAt:         c.now(),
	}
	if updatedState.CreatedAt.IsZero() {
		updatedState.CreatedAt = updatedState.UpdatedAt
	}

	batch, err := c.generateBatch(
		request.Connection,
		windowKey,
		generationInstants,
		mode,
		&updatedState.Envelope,
	)
	if err != nil {
		return domain.ProviderSyncBatch{}, err
	}

	if repeatIndex >= 0 {
		updatedState.Envelope.WindowHistory[repeatIndex].RepeatCount++
	} else {
		updatedState.Envelope.WindowHistory = append(
			updatedState.Envelope.WindowHistory,
			domain.SyntheticWindowHistoryEntry{Window: windowKey, RepeatCount: 1},
		)
	}
	c.sortSequenceCounters(updatedState.Envelope.SequenceCounters)
	_, err = c.stateStore.SaveSyntheticProviderState(ctx, updatedState)
	if err != nil {
		return domain.ProviderSyncBatch{}, fmt.Errorf("save synthetic provider state: %w", err)
	}

	c.logger.InfoContext(
		ctx,
		"fetched synthetic sync batch",
		slog.String("connectionId", request.Connection.ConnectionID),
		slog.String("generationMode", mode),
		slog.Int("observedAccounts", len(batch.Accounts)),
		slog.Int("observedTransactions", len(batch.Transactions)),
	)
	return batch, nil
}

func (c *Connector) generateBatch(
	connection domain.ProviderConnectionRef,
	windowKey domain.SyntheticWindowKey,
	generationInstants []time.Time,
	mode string,
	envelope *domain.SyntheticProviderStateEnvelope,
) (domain.ProviderSyncBatch, error) {
	batch := domain.ProviderSyncBatch{
		Connection:      connection,
		RequestedWindow: domain.ProviderSyncWindow(windowKey),
	}
	sequenceCounters := envelope.SequenceCounters
	accountTotals := map[string]int64{}
	for accountIndex, configuredAccount := range envelope.ConfiguredAccounts {
		providerAccountID := providerAccountID(connection, configuredAccount.Key)
		batch.Accounts = append(batch.Accounts, domain.ProviderAccountObservation{
			Connection:        connection,
			ProviderAccountID: providerAccountID,
			Name:              configuredAccount.Name,
			Currency:          configuredAccount.Currency,
		})
		accountPayload, err := payloadJSON(map[string]any{
			"provider":          string(domain.ProviderIDSynthetic),
			"generationMode":    mode,
			"providerAccountId": providerAccountID,
			"accountKey":        configuredAccount.Key,
			"window":            windowPayload(windowKey),
		})
		if err != nil {
			return domain.ProviderSyncBatch{}, err
		}
		batch.RawPayloads = append(batch.RawPayloads, domain.ProviderRawPayloadObservation{
			Connection:       connection,
			Scope:            domain.RawPayloadScopeAccount,
			ProviderObjectID: providerAccountID,
			PayloadJSON:      accountPayload,
			CapturedAt:       c.now(),
		})

		for _, instant := range generationInstants {
			transactionsForInterval := syntheticFirstSequence +
				c.randomBounded(syntheticFirstWindowMaxTransactionsPerInterval)
			if mode == "repeatedWindow" {
				transactionsForInterval = syntheticFirstSequence +
					c.randomBounded(syntheticRepeatedWindowMaxTransactionsPerInterval)
			}
			for range transactionsForInterval {
				sequence := nextSequence(&sequenceCounters, configuredAccount.Key, instant)
				transaction, payload, makeTransactionErr := c.makeTransaction(
					connection,
					windowKey,
					mode,
					configuredAccount,
					providerAccountID,
					instant,
					sequence,
				)
				if makeTransactionErr != nil {
					return domain.ProviderSyncBatch{}, makeTransactionErr
				}
				batch.Transactions = append(batch.Transactions, transaction)
				batch.RawPayloads = append(batch.RawPayloads, payload)
				accountTotals[providerAccountID] += transaction.AmountMinor
			}
		}
		balanceBase := int64((accountIndex + syntheticFirstSequence) * syntheticBalanceBaseMinor)
		currentBalanceMinor := balanceBase + accountTotals[providerAccountID]
		batch.Balances = append(batch.Balances, domain.ProviderBalanceObservation{
			Connection:          connection,
			ProviderAccountID:   providerAccountID,
			Currency:            configuredAccount.Currency,
			CurrentBalanceMinor: currentBalanceMinor,
			CapturedAt:          c.now(),
		})
	}
	envelope.SequenceCounters = sequenceCounters
	return batch, nil
}

func (c *Connector) makeTransaction(
	connection domain.ProviderConnectionRef,
	windowKey domain.SyntheticWindowKey,
	mode string,
	configuredAccount domain.SyntheticConfiguredAccount,
	providerAccountID string,
	intervalStart time.Time,
	sequence int,
) (domain.ProviderTransactionObservation, domain.ProviderRawPayloadObservation, error) {
	intervalEnd := intervalStart.Add(syntheticWindowStep)
	if windowKey.End.Before(intervalEnd) {
		intervalEnd = windowKey.End
	}
	availableSeconds := int(intervalEnd.Sub(intervalStart) / time.Second)
	offset := time.Duration(c.randomBounded(availableSeconds)) * time.Second
	amountMinor := int64(syntheticAmountBaseMinor + c.randomBounded(syntheticAmountRangeMinor))
	transactionKind := "credit"
	if c.randomBounded(syntheticPositiveNegativeRange) == 0 {
		amountMinor *= -1
		transactionKind = "debit"
	}
	effectiveAt := intervalStart.Add(offset)
	description := fmt.Sprintf(
		"Synthetic %s %s #%d",
		transactionKind,
		effectiveAt.Format(time.RFC3339),
		sequence,
	)
	providerTransactionID := fmt.Sprintf(
		"synthetic-txn-%s-%d-%06d",
		sanitizeIDPart(providerAccountID),
		effectiveAt.UnixNano(),
		sequence,
	)
	providerOriginal := &domain.ProviderTransactionOriginal{
		AmountMinor: amountMinor,
		Currency:    configuredAccount.Currency,
		Description: description,
		EffectiveAt: &effectiveAt,
	}
	transaction := domain.ProviderTransactionObservation{
		Connection:            connection,
		ProviderAccountID:     providerAccountID,
		ProviderTransactionID: providerTransactionID,
		Status:                domain.TransactionStatusBooked,
		AmountMinor:           amountMinor,
		Currency:              configuredAccount.Currency,
		Description:           description,
		EffectiveAt:           effectiveAt,
		Fingerprint: fingerprint(
			connection.ConnectionID,
			configuredAccount.Key,
			providerTransactionID,
			amountMinor,
			configuredAccount.Currency,
		),
		ProviderOriginal: providerOriginal,
	}
	payload, err := payloadJSON(map[string]any{
		"provider":              string(domain.ProviderIDSynthetic),
		"generationMode":        mode,
		"window":                windowPayload(windowKey),
		"providerTransactionId": providerTransactionID,
		"providerAccountId":     providerAccountID,
		"accountKey":            configuredAccount.Key,
		"intervalStart":         intervalStart.Format(time.RFC3339Nano),
		"sequence":              sequence,
		"amountMinor":           amountMinor,
		"currency":              configuredAccount.Currency,
		"description":           description,
	})
	if err != nil {
		return domain.ProviderTransactionObservation{}, domain.ProviderRawPayloadObservation{}, err
	}
	return transaction, domain.ProviderRawPayloadObservation{
		Connection:       connection,
		Scope:            domain.RawPayloadScopeTransaction,
		ProviderObjectID: providerTransactionID,
		PayloadJSON:      payload,
		CapturedAt:       c.now(),
	}, nil
}

func (c *Connector) randomBounded(bound int) int {
	if bound <= 0 || c.randomIntn == nil {
		return 0
	}
	value := c.randomIntn(bound)
	if value < 0 {
		value = -value
	}
	return value % bound
}

func (c *Connector) sortSequenceCounters(counters []domain.SyntheticAccountInstantSequenceCounter) {
	sort.Slice(counters, func(i int, j int) bool {
		if counters[i].AccountKey == counters[j].AccountKey {
			return counters[i].Instant.Before(counters[j].Instant)
		}
		return counters[i].AccountKey < counters[j].AccountKey
	})
}

func newWindowKey(window domain.ProviderSyncWindow) domain.SyntheticWindowKey {
	return domain.SyntheticWindowKey(window)
}

func windowInstants(windowKey domain.SyntheticWindowKey) []time.Time {
	instants := make([]time.Time, 0)
	for instant := windowKey.Start; instant.Before(windowKey.End); instant = instant.Add(syntheticWindowStep) {
		instants = append(instants, instant)
	}
	return instants
}

func windowHistoryIndex(
	history []domain.SyntheticWindowHistoryEntry,
	windowKey domain.SyntheticWindowKey,
) int {
	for index, item := range history {
		if item.Window.Start.Equal(windowKey.Start) && item.Window.End.Equal(windowKey.End) {
			return index
		}
	}
	return -1
}

func nextSequence(
	counters *[]domain.SyntheticAccountInstantSequenceCounter,
	accountKey string,
	instant time.Time,
) int {
	for index := range *counters {
		if (*counters)[index].AccountKey == accountKey && (*counters)[index].Instant.Equal(instant) {
			sequence := (*counters)[index].NextSequence
			if sequence <= 0 {
				sequence = 1
			}
			(*counters)[index].NextSequence = sequence + 1
			return sequence
		}
	}
	*counters = append(*counters, domain.SyntheticAccountInstantSequenceCounter{
		AccountKey:   accountKey,
		Instant:      instant,
		NextSequence: syntheticNextSequenceAfterFirst,
	})
	return syntheticFirstSequence
}

func providerAccountID(connection domain.ProviderConnectionRef, accountKey string) string {
	return fmt.Sprintf(
		"synthetic-account-%s-%s",
		sanitizeIDPart(connection.ConnectionID),
		sanitizeIDPart(accountKey),
	)
}

func sanitizeIDPart(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.ReplaceAll(trimmed, ":", "-")
	trimmed = strings.ReplaceAll(trimmed, "/", "-")
	return trimmed
}

func windowPayload(windowKey domain.SyntheticWindowKey) map[string]string {
	return map[string]string{
		"start": windowKey.Start.Format(time.RFC3339Nano),
		"end":   windowKey.End.Format(time.RFC3339Nano),
	}
}

func payloadJSON(payload any) ([]byte, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal synthetic payload: %w", err)
	}
	return payloadJSON, nil
}

func cloneEnvelope(envelope domain.SyntheticProviderStateEnvelope) domain.SyntheticProviderStateEnvelope {
	return domain.SyntheticProviderStateEnvelope{
		Version:            envelope.Version,
		ConfiguredAccounts: append([]domain.SyntheticConfiguredAccount{}, envelope.ConfiguredAccounts...),
		WindowHistory:      append([]domain.SyntheticWindowHistoryEntry{}, envelope.WindowHistory...),
		SequenceCounters:   append([]domain.SyntheticAccountInstantSequenceCounter{}, envelope.SequenceCounters...),
	}
}

func fingerprint(parts ...any) string {
	hash := sha256.Sum256(fmt.Append(nil, parts...))
	return hex.EncodeToString(hash[:16])
}

func hasConfiguredAccounts(accounts []domain.SyntheticConfiguredAccount) bool {
	if len(accounts) == 0 {
		return false
	}
	for _, account := range accounts {
		if strings.TrimSpace(account.Name) == "" || strings.TrimSpace(account.Currency) == "" {
			return false
		}
	}
	return true
}
