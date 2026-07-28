package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
)

const (
	FXProviderFrankfurter    = "frankfurter"
	FXProviderNBP            = "nbp"
	FXProviderECB            = "ecb"
	FXRefreshJobType         = "finance.fx_rates_refresh"
	FXDailyRefreshScheduleID = "finance.fx_rates_daily_refresh"

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
	FetchLatestRates(ctx context.Context, query FXProviderQuery) ([]domain.FXRate, error)
}

type StaticFXProvider struct {
	name  string
	rates []domain.FXRate
}

func NewStaticFXProvider(name string, rates []domain.FXRate) *StaticFXProvider {
	return &StaticFXProvider{name: strings.TrimSpace(name), rates: rates}
}

func (p *StaticFXProvider) Name() string { return p.name }

func (p *StaticFXProvider) FetchLatestRates(
	ctx context.Context,
	query FXProviderQuery,
) ([]domain.FXRate, error) {
	_ = ctx
	quotes := make(map[string]struct{}, len(query.QuoteCurrencies))
	for _, quote := range query.QuoteCurrencies {
		quotes[strings.ToUpper(strings.TrimSpace(quote))] = struct{}{}
	}
	latestByQuote := make(map[string]domain.FXRate, len(quotes))
	for _, rate := range p.rates {
		if !strings.EqualFold(rate.BaseCurrency, strings.TrimSpace(query.BaseCurrency)) {
			continue
		}
		quote := strings.ToUpper(strings.TrimSpace(rate.QuoteCurrency))
		if _, ok := quotes[quote]; !ok {
			continue
		}
		if rate.EffectiveAt.IsZero() {
			rate.EffectiveAt = rate.RateDate
		}
		if current, found := latestByQuote[quote]; found && !rate.EffectiveAt.After(current.EffectiveAt) {
			continue
		}
		rate.Provider = p.name
		rate.BaseCurrency = strings.ToUpper(strings.TrimSpace(rate.BaseCurrency))
		rate.QuoteCurrency = quote
		latestByQuote[quote] = rate
	}
	items := make([]domain.FXRate, 0, len(latestByQuote))
	for _, rate := range latestByQuote {
		items = append(items, rate)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].QuoteCurrency < items[j].QuoteCurrency })
	return items, nil
}

func (p *StaticFXProvider) FetchHistoricalRates(ctx context.Context, query FXProviderQuery) ([]domain.FXRate, error) {
	rates, err := p.FetchLatestRates(ctx, query)
	for index := range rates {
		rates[index].EffectiveAt = time.Time{}
	}
	return rates, err
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

func (p *StubFXProvider) FetchLatestRates(
	ctx context.Context,
	query FXProviderQuery,
) ([]domain.FXRate, error) {
	_ = ctx
	_ = query
	return nil, ErrFXProviderNotImplemented
}

func (p *StubFXProvider) FetchHistoricalRates(ctx context.Context, query FXProviderQuery) ([]domain.FXRate, error) {
	return p.FetchLatestRates(ctx, query)
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

func (p *FrankfurterFXProvider) FetchLatestRates(
	ctx context.Context,
	query FXProviderQuery,
) ([]domain.FXRate, error) {
	quotes := canonicalizeCurrencies(query.QuoteCurrencies)
	if len(quotes) == 0 {
		return nil, errors.New("fetch frankfurter rates: at least one quote currency is required")
	}
	baseCurrency := strings.ToUpper(strings.TrimSpace(query.BaseCurrency))
	if baseCurrency == "" {
		return nil, errors.New("fetch frankfurter rates: base currency is required")
	}

	reqURL, err := url.Parse(p.baseURL + "/v2/rates")
	if err != nil {
		return nil, fmt.Errorf("build frankfurter request: %w", err)
	}
	values := reqURL.Query()
	values.Set("base", baseCurrency)
	values.Set("quotes", strings.Join(quotes, ","))
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

	return decodeFrankfurterLatestRates(body, baseCurrency, quotes)
}

func decodeFrankfurterLatestRates(
	body []byte,
	baseCurrency string,
	quoteCurrencies []string,
) ([]domain.FXRate, error) {
	type latestRateResponse struct {
		Date  string  `json:"date"`
		Base  string  `json:"base"`
		Quote string  `json:"quote"`
		Rate  float64 `json:"rate"`
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var response []latestRateResponse
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("decode frankfurter latest response: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("decode frankfurter latest response: expected one JSON value")
	}

	expectedQuotes := make(map[string]struct{}, len(quoteCurrencies))
	for _, quoteCurrency := range quoteCurrencies {
		expectedQuotes[quoteCurrency] = struct{}{}
	}
	if len(response) != len(expectedQuotes) {
		return nil, fmt.Errorf(
			"decode frankfurter latest response: expected %d rates, got %d",
			len(expectedQuotes),
			len(response),
		)
	}

	rates := make([]domain.FXRate, 0, len(response))
	seenQuotes := make(map[string]struct{}, len(response))
	for _, item := range response {
		if strings.ToUpper(strings.TrimSpace(item.Base)) != baseCurrency {
			return nil, errors.New("decode frankfurter latest response: base does not match request")
		}
		quoteCurrency := strings.ToUpper(strings.TrimSpace(item.Quote))
		if _, ok := expectedQuotes[quoteCurrency]; !ok {
			return nil, errors.New("decode frankfurter latest response: quote does not match request")
		}
		if _, duplicate := seenQuotes[quoteCurrency]; duplicate {
			return nil, errors.New("decode frankfurter latest response: duplicate quote")
		}
		if item.Rate <= 0 || math.IsNaN(item.Rate) || math.IsInf(item.Rate, 0) {
			return nil, errors.New("decode frankfurter latest response: rate must be positive and finite")
		}
		effectiveAt, err := time.ParseInLocation(time.DateOnly, item.Date, time.Local)
		if err != nil {
			return nil, fmt.Errorf("parse frankfurter date: %w", err)
		}
		seenQuotes[quoteCurrency] = struct{}{}
		rates = append(rates, domain.FXRate{
			Provider:      FXProviderFrankfurter,
			BaseCurrency:  baseCurrency,
			QuoteCurrency: quoteCurrency,
			EffectiveAt:   effectiveAt,
			RateDate:      effectiveAt,
			Rate:          item.Rate,
		})
	}
	sort.Slice(rates, func(i, j int) bool { return rates[i].QuoteCurrency < rates[j].QuoteCurrency })
	return rates, nil
}

func (p *FrankfurterFXProvider) FetchHistoricalRates(
	ctx context.Context,
	query FXProviderQuery,
) ([]domain.FXRate, error) {
	rates, err := p.FetchLatestRates(ctx, query)
	for index := range rates {
		rates[index].EffectiveAt = time.Time{}
	}
	return rates, err
}

type FXRefreshJobEnqueuer interface {
	EnqueueFXRefresh(ctx context.Context, request FXRefreshJobRequest) (FXRefreshJobRef, error)
}

type FXRefreshScheduleWriter interface {
	UpsertFXRefreshSchedule(ctx context.Context, schedule FXRefreshSchedule) error
}

type FXRefreshJobRequest struct {
	JobType   string
	Requester FXSyncRequester
	Input     RefreshFXRatesParams
}

type FXRefreshJobRef struct {
	ID       string
	JobType  string
	Provider string
}

type FXRefreshSchedule struct {
	ScheduleID string
	JobType    string
	Requester  FXSyncRequester
	Interval   time.Duration
	Input      RefreshFXRatesParams
	Enabled    bool
}

type FXSyncRequester struct {
	UserID string
	Source string
}

type RefreshFXRatesParams struct{ Provider string }

type RefreshFXRatesResult struct {
	Provider      string
	ImportedCount int
}

type TriggerFXRefreshParams struct {
	RequestedByUserID string
	Source            string
	Provider          string
}

type EnsureFXRefreshScheduleParams struct {
	ScheduleID      string
	Provider        string
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
