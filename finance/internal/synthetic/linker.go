package synthetic

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
)

const ConnectionDisplayName = "Synthetic"

var (
	ErrUnsupportedConfiguredBankProvider     = errors.New("unsupported configured bank provider")
	ErrConfiguredBankAccountNameRequired     = errors.New("configured bank account name is required")
	ErrConfiguredBankAccountCurrencyRequired = errors.New("configured bank account currency is required")
)

type ConfiguredAccount struct {
	Name     string
	Currency string
}

type LinkConfiguredBankConnectionParams struct {
	ActorUserID string
	TenantID    string
	Provider    string
	Accounts    []ConfiguredAccount
}

type LinkerDeps struct {
	RequireTenantMember  func(ctx context.Context, tenantID string, actorUserID string) error
	SaveConnectionSecret func(
		ctx context.Context,
		provider string,
		reference string,
		secret string,
	) (string, error)
	DeleteConnectionSecret func(ctx context.Context, secretID string) error
	SaveBankConnection     func(
		ctx context.Context,
		connection domain.BankConnection,
	) (domain.BankConnection, error)
	DeleteBankConnectionOwnedMetadata func(
		ctx context.Context,
		connection domain.BankConnection,
	) error
	SaveSyntheticProviderState func(
		ctx context.Context,
		state domain.SyntheticProviderState,
	) (domain.SyntheticProviderState, error)
	Now   func() time.Time
	NewID func() string
}

type Linker struct {
	deps LinkerDeps
}

func NewLinker(deps LinkerDeps) *Linker {
	return &Linker{deps: deps}
}

func (l *Linker) LinkConfiguredBankConnection(
	ctx context.Context,
	params LinkConfiguredBankConnectionParams,
) (domain.BankConnection, error) {
	if err := l.deps.RequireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return domain.BankConnection{}, err
	}
	if strings.TrimSpace(params.Provider) != string(domain.ProviderIDSynthetic) {
		return domain.BankConnection{}, fmt.Errorf(
			"%w: %s",
			ErrUnsupportedConfiguredBankProvider,
			strings.TrimSpace(params.Provider),
		)
	}
	configuredAccounts, err := makeConfiguredAccounts(params.Accounts, l.deps.NewID)
	if err != nil {
		return domain.BankConnection{}, err
	}
	now := l.deps.Now().UTC()
	providerReference := "synthetic-configured-" + l.deps.NewID()
	secretID, err := l.deps.SaveConnectionSecret(
		ctx,
		string(domain.ProviderIDSynthetic),
		providerReference,
		"",
	)
	if err != nil {
		return domain.BankConnection{}, err
	}
	connection := domain.BankConnection{
		ID:                l.deps.NewID(),
		TenantID:          strings.TrimSpace(params.TenantID),
		Provider:          string(domain.ProviderIDSynthetic),
		DisplayName:       ConnectionDisplayName,
		ProviderReference: providerReference,
		SecretID:          secretID,
		State:             domain.BankConnectionStateActive,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	savedConnection, err := l.deps.SaveBankConnection(ctx, connection)
	if err != nil {
		if cleanupErr := l.deps.DeleteConnectionSecret(ctx, secretID); cleanupErr != nil {
			return domain.BankConnection{}, errors.Join(
				fmt.Errorf("save bank connection: %w", err),
				fmt.Errorf("cleanup synthetic link secret: %w", cleanupErr),
			)
		}
		return domain.BankConnection{}, fmt.Errorf("save bank connection: %w", err)
	}
	_, err = l.deps.SaveSyntheticProviderState(ctx, domain.SyntheticProviderState{
		ConnectionID: savedConnection.ID,
		Envelope: domain.SyntheticProviderStateEnvelope{
			Version:            domain.SyntheticProviderStateVersion1,
			ConfiguredAccounts: configuredAccounts,
			WindowHistory:      []domain.SyntheticWindowHistoryEntry{},
			SequenceCounters:   []domain.SyntheticAccountDaySequenceCounter{},
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		if cleanupErr := l.deps.DeleteBankConnectionOwnedMetadata(ctx, savedConnection); cleanupErr != nil {
			return domain.BankConnection{}, errors.Join(
				fmt.Errorf("save synthetic provider state: %w", err),
				fmt.Errorf("cleanup synthetic bank connection: %w", cleanupErr),
			)
		}
		return domain.BankConnection{}, fmt.Errorf("save synthetic provider state: %w", err)
	}
	return savedConnection, nil
}

func makeConfiguredAccounts(
	accounts []ConfiguredAccount,
	newID func() string,
) ([]domain.SyntheticConfiguredAccount, error) {
	configuredAccounts := make([]domain.SyntheticConfiguredAccount, 0, len(accounts))
	for index, account := range accounts {
		name := strings.TrimSpace(account.Name)
		if name == "" {
			return nil, fmt.Errorf("configured account %d: %w", index, ErrConfiguredBankAccountNameRequired)
		}
		currency := strings.TrimSpace(account.Currency)
		if currency == "" {
			return nil, fmt.Errorf("configured account %d: %w", index, ErrConfiguredBankAccountCurrencyRequired)
		}
		accountKey := strings.TrimSpace(newID())
		if accountKey == "" {
			accountKey = strconv.Itoa(index + 1)
		}
		configuredAccounts = append(configuredAccounts, domain.SyntheticConfiguredAccount{
			Key:      "synthetic-account-" + accountKey,
			Name:     name,
			Currency: currency,
		})
	}
	return configuredAccounts, nil
}
