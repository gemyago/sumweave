package finance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
)

const (
	FXProviderFrankfurter = "frankfurter"
	FXProviderNBP         = "nbp"
	FXProviderECB         = "ecb"
	FXSyncJobType         = "finance.fx_rates_sync"

	FXSyncRequesterSourceOperator = "operator"
	FXSyncRequesterSourceSystem   = "system"
	maxFXErrorBodyBytes           = 2048
)

var ErrFXProviderNotImplemented = errors.New("fx provider not implemented")

type FXProviderQuery struct {
	BaseCurrency    string
	QuoteCurrencies []string
	StartDate       time.Time
	EndDate         time.Time
}

type FXRatesProvider interface {
	Name() string
	FetchHistoricalRates(ctx context.Context, query FXProviderQuery) ([]domain.FXRate, error)
}

type StaticFXProvider struct {
	name  string
	rates []domain.FXRate
}

func NewStaticFXProvider(name string, rates []domain.FXRate) *StaticFXProvider {
	return &StaticFXProvider{name: strings.TrimSpace(name), rates: rates}
}

func (p *StaticFXProvider) Name() string { return p.name }

func (p *StaticFXProvider) FetchHistoricalRates(
	ctx context.Context,
	query FXProviderQuery,
) ([]domain.FXRate, error) {
	_ = ctx
	if err := ValidateRequiredTimestampRange(query.StartDate, query.EndDate); err != nil {
		return nil, err
	}
	quotes := make(map[string]struct{}, len(query.QuoteCurrencies))
	for _, quote := range query.QuoteCurrencies {
		quotes[strings.ToUpper(strings.TrimSpace(quote))] = struct{}{}
	}
	items := make([]domain.FXRate, 0)
	for _, rate := range p.rates {
		if !strings.EqualFold(rate.BaseCurrency, strings.TrimSpace(query.BaseCurrency)) {
			continue
		}
		if _, ok := quotes[strings.ToUpper(rate.QuoteCurrency)]; !ok {
			continue
		}
		if !query.StartDate.IsZero() && rate.RateDate.Before(query.StartDate) {
			continue
		}
		if !query.EndDate.IsZero() && rate.RateDate.After(query.EndDate) {
			continue
		}
		items = append(items, rate)
	}
	return items, nil
}

type StubFXProvider struct{ name string }

func NewNBPFXProvider(client *http.Client, baseURL string) *StubFXProvider {
	_ = client
	_ = baseURL
	return &StubFXProvider{name: FXProviderNBP}
}

func NewECBFXProvider(client *http.Client, baseURL string) *StubFXProvider {
	_ = client
	_ = baseURL
	return &StubFXProvider{name: FXProviderECB}
}

func (p *StubFXProvider) Name() string { return p.name }

func (p *StubFXProvider) FetchHistoricalRates(
	ctx context.Context,
	query FXProviderQuery,
) ([]domain.FXRate, error) {
	_ = ctx
	if err := ValidateRequiredTimestampRange(query.StartDate, query.EndDate); err != nil {
		return nil, err
	}
	return nil, ErrFXProviderNotImplemented
}

type FrankfurterFXProvider struct {
	client  *http.Client
	baseURL string
}

func NewFrankfurterFXProvider(client *http.Client, baseURL string) *FrankfurterFXProvider {
	if client == nil {
		client = http.DefaultClient
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.frankfurter.dev"
	}
	return &FrankfurterFXProvider{client: client, baseURL: strings.TrimRight(baseURL, "/")}
}

func (p *FrankfurterFXProvider) Name() string { return FXProviderFrankfurter }

func (p *FrankfurterFXProvider) FetchHistoricalRates(
	ctx context.Context,
	query FXProviderQuery,
) ([]domain.FXRate, error) {
	if err := ValidateRequiredTimestampRange(query.StartDate, query.EndDate); err != nil {
		return nil, err
	}
	startDate := query.StartDate
	endDate := query.EndDate
	path := startDate.Format(time.DateOnly)
	if !startDate.Equal(endDate) {
		path = path + ".." + endDate.Format(time.DateOnly)
	}
	reqURL, err := url.Parse(p.baseURL + "/" + path)
	if err != nil {
		return nil, fmt.Errorf("build frankfurter request: %w", err)
	}
	values := reqURL.Query()
	values.Set("from", strings.ToUpper(strings.TrimSpace(query.BaseCurrency)))
	values.Set("to", strings.Join(canonicalizeCurrencies(query.QuoteCurrencies), ","))
	reqURL.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create frankfurter request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch frankfurter rates: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxFXErrorBodyBytes))
		return nil, fmt.Errorf(
			"fetch frankfurter rates: status %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read frankfurter response: %w", err)
	}

	type singleDayResponse struct {
		Date  string             `json:"date"`
		Base  string             `json:"base"`
		Rates map[string]float64 `json:"rates"`
	}
	type rangeResponse struct {
		Base  string                        `json:"base"`
		Rates map[string]map[string]float64 `json:"rates"`
	}

	var single singleDayResponse
	if decodeErr := json.Unmarshal(body, &single); decodeErr == nil && single.Date != "" {
		rateDate, parseErr := time.ParseInLocation(time.DateOnly, single.Date, time.Local)
		if parseErr != nil {
			return nil, fmt.Errorf("parse frankfurter date: %w", parseErr)
		}
		return makeProviderRates(FXProviderFrankfurter, single.Base, rateDate, single.Rates), nil
	}

	var dateRange rangeResponse
	if decodeErr := json.Unmarshal(body, &dateRange); decodeErr != nil {
		return nil, fmt.Errorf("decode frankfurter response: %w", decodeErr)
	}
	items := make([]domain.FXRate, 0)
	for dateString, rates := range dateRange.Rates {
		rateDate, parseErr := time.ParseInLocation(time.DateOnly, dateString, time.Local)
		if parseErr != nil {
			return nil, fmt.Errorf("parse frankfurter date: %w", parseErr)
		}
		items = append(
			items,
			makeProviderRates(FXProviderFrankfurter, dateRange.Base, rateDate, rates)...)
	}
	return items, nil
}

func makeProviderRates(
	provider string,
	baseCurrency string,
	rateDate time.Time,
	rates map[string]float64,
) []domain.FXRate {
	items := make([]domain.FXRate, 0, len(rates))
	for quoteCurrency, rateValue := range rates {
		items = append(items, domain.FXRate{
			Provider:      provider,
			BaseCurrency:  strings.ToUpper(strings.TrimSpace(baseCurrency)),
			QuoteCurrency: strings.ToUpper(strings.TrimSpace(quoteCurrency)),
			RateDate:      rateDate,
			Rate:          rateValue,
		})
	}
	sort.Slice(items, func(i int, j int) bool {
		return items[i].QuoteCurrency < items[j].QuoteCurrency
	})
	return items
}

type FXSyncJobEnqueuer interface {
	EnqueueFXSync(ctx context.Context, request FXSyncJobRequest) (FXSyncJobRef, error)
}

type FXSyncScheduleWriter interface {
	UpsertFXSyncSchedule(ctx context.Context, schedule FXSyncSchedule) error
}

type FXSyncJobRequest struct {
	JobType   string
	Requester FXSyncRequester
	Input     SyncFXRatesParams
}

type FXSyncJobRef struct {
	ID      string
	JobType string
}

type FXSyncSchedule struct {
	ScheduleID string
	JobType    string
	Requester  FXSyncRequester
	Interval   time.Duration
	Input      SyncFXRatesParams
	Enabled    bool
}

type FXSyncRequester struct {
	UserID string
	Source string
}

type SyncFXRatesParams struct {
	Provider       string
	BaseCurrencies []string
	QuoteCurrency  string
	StartDate      time.Time
	EndDate        time.Time
}

type SyncFXRatesResult struct {
	Provider      string
	ImportedCount int
}

type TriggerFXSyncParams struct {
	RequestedByUserID string
	Source            string
	Provider          string
	BaseCurrencies    []string
	QuoteCurrency     string
	StartDate         time.Time
	EndDate           time.Time
}

type EnsureFXSyncScheduleParams struct {
	ScheduleID      string
	Provider        string
	BaseCurrencies  []string
	QuoteCurrency   string
	Interval        time.Duration
	RequestedByUser string
}

type FXAdminDiagnosticsParams struct{}

type FXAdminDiagnostics struct {
	DefaultProvider  string                       `json:"defaultProvider"`
	Providers        []FXAdminProviderDiagnostics `json:"providers"`
	StoredRatesCount int                          `json:"storedRatesCount"`
}

type FXAdminProviderDiagnostics struct {
	Name    string `json:"name"`
	Default bool   `json:"default"`
	Ready   bool   `json:"ready"`
}

func defaultFXProviders() map[string]FXRatesProvider {
	providers := map[string]FXRatesProvider{}
	for _, provider := range []FXRatesProvider{
		NewFrankfurterFXProvider(nil, ""),
		NewNBPFXProvider(nil, ""),
		NewECBFXProvider(nil, ""),
	} {
		providers[provider.Name()] = provider
	}
	return providers
}

func canonicalizeCurrencies(values []string) []string {
	set := map[string]struct{}{}
	items := make([]string, 0, len(values))
	for _, value := range values {
		currency := strings.ToUpper(strings.TrimSpace(value))
		if currency == "" {
			continue
		}
		if _, ok := set[currency]; ok {
			continue
		}
		set[currency] = struct{}{}
		items = append(items, currency)
	}
	return items
}
