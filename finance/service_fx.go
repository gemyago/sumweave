package finance

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
)

type fxServiceStore interface {
	SaveFXRates(ctx context.Context, rates []domain.FXRate) error
	ListFXRates(ctx context.Context, params persistence.ListFXRatesParams) ([]domain.FXRate, error)
}

type FXService struct {
	store             fxServiceStore
	now               func() time.Time
	fxProviders       map[string]FXRatesProvider
	defaultFXProvider string
	fxJobEnqueuer     FXSyncJobEnqueuer
	fxScheduleWriter  FXSyncScheduleWriter
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

func WithFXServiceJobEnqueuer(enqueuer FXSyncJobEnqueuer) FXServiceOption {
	return func(service *FXService) {
		service.fxJobEnqueuer = enqueuer
	}
}

func WithFXServiceScheduleWriter(writer FXSyncScheduleWriter) FXServiceOption {
	return func(service *FXService) {
		service.fxScheduleWriter = writer
	}
}

func NewFXService(store fxServiceStore, opts ...FXServiceOption) *FXService {
	service := &FXService{
		store:             store,
		now:               func() time.Time { return time.Now().UTC() },
		fxProviders:       defaultFXProviders(),
		defaultFXProvider: FXProviderFrankfurter,
	}
	for _, opt := range opts {
		opt(service)
	}
	return service
}

func (s *FXService) SyncFXRates(
	ctx context.Context,
	params SyncFXRatesParams,
) (SyncFXRatesResult, error) {
	providerName, err := s.resolveFXProviderName(params.Provider)
	if err != nil {
		return SyncFXRatesResult{}, err
	}
	provider := s.fxProviders[providerName]
	items := make([]domain.FXRate, 0)
	for _, baseCurrency := range canonicalizeCurrencies(params.BaseCurrencies) {
		rates, fetchErr := provider.FetchHistoricalRates(ctx, FXProviderQuery{
			BaseCurrency:    baseCurrency,
			QuoteCurrencies: []string{strings.ToUpper(strings.TrimSpace(params.QuoteCurrency))},
			StartDate:       startOfDay(params.StartDate),
			EndDate:         startOfDay(params.EndDate),
		})
		if fetchErr != nil {
			return SyncFXRatesResult{}, fmt.Errorf("sync fx rates: %w", fetchErr)
		}
		items = append(items, rates...)
	}
	if saveErr := s.store.SaveFXRates(ctx, items); saveErr != nil {
		return SyncFXRatesResult{}, fmt.Errorf("sync fx rates: %w", saveErr)
	}
	return SyncFXRatesResult{Provider: providerName, ImportedCount: len(items)}, nil
}

func (s *FXService) TriggerFXSync(
	ctx context.Context,
	params TriggerFXSyncParams,
) (FXSyncJobRef, error) {
	if s.fxJobEnqueuer == nil {
		return FXSyncJobRef{}, errors.New("fx job enqueuer is required")
	}
	providerName, err := s.resolveFXProviderName(params.Provider)
	if err != nil {
		return FXSyncJobRef{}, err
	}
	jobRef, err := s.fxJobEnqueuer.EnqueueFXSync(ctx, FXSyncJobRequest{
		JobType: FXSyncJobType,
		Requester: FXSyncRequester{
			UserID: strings.TrimSpace(params.RequestedByUserID),
			Source: strings.TrimSpace(params.Source),
		},
		Input: SyncFXRatesParams{
			Provider:       providerName,
			BaseCurrencies: canonicalizeCurrencies(params.BaseCurrencies),
			QuoteCurrency:  strings.ToUpper(strings.TrimSpace(params.QuoteCurrency)),
			StartDate:      startOfDay(params.StartDate),
			EndDate:        startOfDay(params.EndDate),
		},
	})
	if err != nil {
		return FXSyncJobRef{}, fmt.Errorf("trigger fx sync: %w", err)
	}
	return jobRef, nil
}

func (s *FXService) EnsureFXSyncSchedule(
	ctx context.Context,
	params EnsureFXSyncScheduleParams,
) (FXSyncSchedule, error) {
	if s.fxScheduleWriter == nil {
		return FXSyncSchedule{}, errors.New("fx schedule writer is required")
	}
	providerName, err := s.resolveFXProviderName(params.Provider)
	if err != nil {
		return FXSyncSchedule{}, err
	}
	now := startOfDay(s.now())
	schedule := FXSyncSchedule{
		ScheduleID: strings.TrimSpace(params.ScheduleID),
		JobType:    FXSyncJobType,
		Requester: FXSyncRequester{
			UserID: strings.TrimSpace(params.RequestedByUser),
			Source: FXSyncRequesterSourceSystem,
		},
		Interval: params.Interval,
		Input: SyncFXRatesParams{
			Provider:       providerName,
			BaseCurrencies: canonicalizeCurrencies(params.BaseCurrencies),
			QuoteCurrency:  strings.ToUpper(strings.TrimSpace(params.QuoteCurrency)),
			StartDate:      now,
			EndDate:        now,
		},
		Enabled: true,
	}
	if upsertErr := s.fxScheduleWriter.UpsertFXSyncSchedule(ctx, schedule); upsertErr != nil {
		return FXSyncSchedule{}, fmt.Errorf("ensure fx sync schedule: %w", upsertErr)
	}
	return schedule, nil
}

func (s *FXService) GetFXAdminDiagnostics(
	ctx context.Context,
	params FXAdminDiagnosticsParams,
) (FXAdminDiagnostics, error) {
	_ = params
	storedRates, err := s.store.ListFXRates(ctx, persistence.ListFXRatesParams{})
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
