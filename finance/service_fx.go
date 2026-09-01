package finance

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
)

type fxServiceStore interface {
	SaveCurrentFXRates(ctx context.Context, rates []domain.FXRate) error
	ListCurrentFXRates(ctx context.Context, params persistence.ListCurrentFXRatesParams) ([]domain.FXRate, error)
}

type requiredFXPairLister interface {
	ListRequiredFXPairs(context.Context) ([]persistence.RequiredFXPair, error)
}

type FXService struct {
	store             fxServiceStore
	now               func() time.Time
	fxProviders       map[string]FXRatesProvider
	defaultFXProvider string
	commandPublisher  SemanticCommandPublisher
	requiredPairs     requiredFXPairLister
}

type FXServiceOption func(*FXService)

func WithFXServiceNow(now func() time.Time) FXServiceOption {
	return func(service *FXService) {
		service.now = now
	}
}

func WithFXServiceProviders(providers ...FXRatesProvider) FXServiceOption {
	return func(service *FXService) {
		if service.fxProviders == nil {
			service.fxProviders = map[string]FXRatesProvider{}
		}
		for _, provider := range providers {
			if provider != nil {
				service.fxProviders[provider.Name()] = provider
			}
		}
	}
}

func WithFXServiceDefaultProvider(name string) FXServiceOption {
	return func(service *FXService) {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			service.defaultFXProvider = trimmed
		}
	}
}

func WithFXServiceCommandPublisher(publisher SemanticCommandPublisher) FXServiceOption {
	return func(service *FXService) {
		service.commandPublisher = publisher
	}
}

func WithFXServiceRequiredPairs(lister requiredFXPairLister) FXServiceOption {
	return func(service *FXService) { service.requiredPairs = lister }
}

func NewFXService(store fxServiceStore, opts ...FXServiceOption) *FXService {
	service := &FXService{
		store:             store,
		now:               time.Now,
		fxProviders:       defaultFXProviders(),
		defaultFXProvider: FXProviderFrankfurter,
	}
	for _, opt := range opts {
		opt(service)
	}
	return service
}

func (s *FXService) RefreshRequiredFXRates(
	ctx context.Context,
	params RefreshFXRatesParams,
) (RefreshFXRatesResult, error) {
	if s.requiredPairs == nil {
		return RefreshFXRatesResult{}, errors.New("required fx pair lister is required")
	}
	providerName, err := s.resolveFXProviderName(params.Provider)
	if err != nil {
		return RefreshFXRatesResult{}, NewTerminalFailure(
			err,
			"fx_provider_not_configured",
			"FX provider is not configured",
			"The requested FX provider is not available.",
		)
	}
	pairs, err := s.requiredPairs.ListRequiredFXPairs(ctx)
	if err != nil {
		return RefreshFXRatesResult{}, fmt.Errorf("refresh required fx rates: %w", err)
	}
	items := make([]domain.FXRate, 0, len(pairs))
	refreshAt := s.now()
	provider := s.fxProviders[providerName]
	for _, pair := range pairs {
		rates, fetchErr := provider.FetchLatestRates(ctx, FXProviderQuery{
			BaseCurrency: pair.BaseCurrency, QuoteCurrencies: []string{pair.QuoteCurrency},
		})
		if fetchErr != nil {
			return RefreshFXRatesResult{}, terminalFXRefreshFailure(
				fmt.Errorf("refresh required fx rates: %w", fetchErr),
			)
		}
		rate, validationErr := validateProviderFXRates(providerName, pair.BaseCurrency, pair.QuoteCurrency, rates)
		if validationErr != nil {
			return RefreshFXRatesResult{}, NewTerminalFailure(
				fmt.Errorf("refresh required fx rates: %w", validationErr),
				"fx_provider_response_invalid",
				"FX provider response is invalid",
				"The FX provider returned invalid rate data.",
			)
		}
		rate.LastSuccessfulRefreshAt = refreshAt
		items = append(items, rate)
	}
	if saveErr := s.store.SaveCurrentFXRates(ctx, items); saveErr != nil {
		return RefreshFXRatesResult{}, fmt.Errorf("refresh required fx rates: %w", saveErr)
	}
	return RefreshFXRatesResult{Provider: providerName, ImportedCount: len(items)}, nil
}

func terminalFXRefreshFailure(err error) error {
	if errors.Is(err, ErrFXProviderNotImplemented) {
		return NewTerminalFailure(
			err,
			"fx_provider_not_supported",
			"FX provider is not supported",
			"The requested FX provider cannot refresh rates.",
		)
	}
	var providerErr *ProviderResponseError
	if errors.As(err, &providerErr) && providerErr.IsTerminal() {
		return NewTerminalFailure(
			err,
			"fx_provider_rejected_request",
			"FX provider rejected the refresh request",
			"The FX provider rejected the rate refresh request.",
		)
	}
	return err
}

func (s *FXService) TriggerFXRefresh(
	ctx context.Context,
	params TriggerFXRefreshParams,
) (FXRefreshJobRef, error) {
	if s.commandPublisher == nil {
		return FXRefreshJobRef{}, errors.New("fx refresh command publisher is required")
	}
	providerName, err := s.resolveFXProviderName(params.Provider)
	if err != nil {
		return FXRefreshJobRef{}, err
	}
	command, commandErr := newSemanticCommand(FXRatesRefreshCommandTopic, FXRatesRefreshCommand{
		Provider: providerName,
		Requester: CommandRequester{
			UserID: strings.TrimSpace(params.RequestedByUserID),
			Source: strings.TrimSpace(params.Source),
		},
	}, "")
	if commandErr != nil {
		return FXRefreshJobRef{}, commandErr
	}
	reference, err := s.commandPublisher.PublishSemanticCommand(ctx, command)
	if err != nil {
		return FXRefreshJobRef{}, fmt.Errorf("trigger required fx refresh: %w", err)
	}
	return FXRefreshJobRef{ID: reference.MessageID, JobType: FXRefreshJobType, Provider: providerName}, nil
}

func validateProviderFXRates(
	providerName string,
	baseCurrency string,
	quoteCurrency string,
	rates []domain.FXRate,
) (domain.FXRate, error) {
	if len(rates) != 1 {
		return domain.FXRate{}, fmt.Errorf("provider returned %d rates for one requested pair", len(rates))
	}
	rate := rates[0]
	if strings.TrimSpace(rate.Provider) != providerName {
		return domain.FXRate{}, errors.New("provider rate identity does not match requested provider")
	}
	requestedBase := strings.ToUpper(strings.TrimSpace(baseCurrency))
	requestedQuote := strings.ToUpper(strings.TrimSpace(quoteCurrency))
	rate.BaseCurrency = strings.ToUpper(strings.TrimSpace(rate.BaseCurrency))
	rate.QuoteCurrency = strings.ToUpper(strings.TrimSpace(rate.QuoteCurrency))
	if rate.BaseCurrency != requestedBase || rate.QuoteCurrency != requestedQuote {
		return domain.FXRate{}, errors.New("provider rate pair does not match requested pair")
	}
	if rate.EffectiveAt.IsZero() {
		rate.EffectiveAt = rate.RateDate
	}
	if rate.EffectiveAt.IsZero() {
		return domain.FXRate{}, fmt.Errorf("%w: provider rate timestamp is required", ErrInvalidTimestampRange)
	}
	if rate.Rate <= 0 || math.IsNaN(rate.Rate) || math.IsInf(rate.Rate, 0) {
		return domain.FXRate{}, errors.New("provider rate must be positive and finite")
	}
	rate.Provider = providerName
	return rate, nil
}

func (s *FXService) GetFXAdminDiagnostics(
	ctx context.Context,
	params FXAdminDiagnosticsParams,
) (FXAdminDiagnostics, error) {
	_ = params
	storedRates, err := s.store.ListCurrentFXRates(ctx, persistence.ListCurrentFXRatesParams{})
	if err != nil {
		return FXAdminDiagnostics{}, fmt.Errorf("get fx admin diagnostics: %w", err)
	}
	providerNames := make([]string, 0, len(s.fxProviders))
	for name := range s.fxProviders {
		providerNames = append(providerNames, name)
	}
	sort.Strings(providerNames)
	providers := make([]FXAdminProviderDiagnostics, 0, len(providerNames))
	for _, name := range providerNames {
		providers = append(providers, FXAdminProviderDiagnostics{
			Name:    name,
			Default: name == s.defaultFXProvider,
			Ready:   true,
		})
	}
	return FXAdminDiagnostics{
		DefaultProvider:  s.defaultFXProvider,
		Providers:        providers,
		StoredRatesCount: len(storedRates),
	}, nil
}

func (s *FXService) resolveFXProviderName(name string) (string, error) {
	providerName := strings.TrimSpace(name)
	if providerName == "" {
		providerName = s.defaultFXProvider
	}
	if _, ok := s.fxProviders[providerName]; !ok {
		return "", fmt.Errorf("fx provider not configured: %s", providerName)
	}
	return providerName, nil
}
