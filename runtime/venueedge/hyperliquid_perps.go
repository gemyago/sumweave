package venueedge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/google/uuid"
)

const (
	hyperliquidDefaultReadLimit = 500
	hyperliquidInfoTypeCandle   = "candleSnapshot"
	hyperliquidInfoTypeKey      = "type"
	hyperliquidInfoTypeMeta     = "meta"
	hyperliquidInfoTypeTrades   = "recentTrades"
	hyperliquidInfoEndpoint     = "/info"
	hyperliquidRequestCoinKey   = "coin"
)

// HyperliquidPerpsVenueName is the canonical venue used by the Hyperliquid perps adapter.
const HyperliquidPerpsVenueName domain.Venue = "hyperliquid-perps"

// HyperliquidPerpsVenueParams configures the concrete Hyperliquid perps market-data adapter.
type HyperliquidPerpsVenueParams struct {
	BaseURL                 string
	HTTPClient              httpDoer
	RawEvidenceRecorder     HyperliquidRawEvidenceRecorder
	RawEvidenceIngestionRun string
}

// HyperliquidRawEvidenceRecorder persists one Hyperliquid HTTP exchange.
type HyperliquidRawEvidenceRecorder interface {
	RecordHyperliquidRawEvidence(ctx context.Context, capture HyperliquidRawEvidenceCapture) (string, error)
}

// HyperliquidRawEvidenceCapture carries one raw Hyperliquid `/info` exchange.
type HyperliquidRawEvidenceCapture struct {
	ID                 string
	IngestionRunID     string
	Venue              domain.Venue
	Endpoint           string
	RequestType        string
	RequestPayloadHash string
	RequestMetadata    map[string]string
	RequestAt          time.Time
	ResponseAt         time.Time
	HTTPStatus         int
	ResponseBody       []byte
	EntityHint         string
	Instrument         *domain.Instrument
	Timeframe          domain.Timeframe
	TimeRange          *domain.TimeRange
	ReceivedAt         time.Time
}

type hyperliquidMetaResponse struct {
	Universe []struct {
		Name       string `json:"name"`
		IsDelisted bool   `json:"isDelisted"`
	} `json:"universe"`
}

type hyperliquidCandle struct {
	CloseTime int64                   `json:"T"`
	Close     hyperliquidDecimalValue `json:"c"`
	High      hyperliquidDecimalValue `json:"h"`
	Interval  string                  `json:"i"`
	Low       hyperliquidDecimalValue `json:"l"`
	Open      hyperliquidDecimalValue `json:"o"`
	Symbol    string                  `json:"s"`
	OpenTime  int64                   `json:"t"`
	Volume    hyperliquidDecimalValue `json:"v"`
}

type hyperliquidTrade struct {
	Symbol string                  `json:"coin"`
	Price  hyperliquidDecimalValue `json:"px"`
	Size   hyperliquidDecimalValue `json:"sz"`
	Time   int64                   `json:"time"`
	TID    hyperliquidInt64Value   `json:"tid"`
}

type hyperliquidVenueError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// HyperliquidPerpsVenue adapts documented Hyperliquid Info market-data reads into canonical records.
type HyperliquidPerpsVenue struct {
	baseURL                 string
	httpClient              httpDoer
	rawEvidenceRecorder     HyperliquidRawEvidenceRecorder
	rawEvidenceIngestionRun string
}

type hyperliquidInfoCaptureScope struct {
	entityHint string
	instrument *domain.Instrument
	timeframe  domain.Timeframe
	timeRange  *domain.TimeRange
}

// NewHyperliquidPerpsVenue creates a concrete mocked-HTTP-friendly Hyperliquid perps adapter.
func NewHyperliquidPerpsVenue(params HyperliquidPerpsVenueParams) (*HyperliquidPerpsVenue, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(params.BaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("base URL is required")
	}
	if params.HTTPClient == nil {
		return nil, errors.New("http client is required")
	}

	return &HyperliquidPerpsVenue{
		baseURL:                 baseURL,
		httpClient:              params.HTTPClient,
		rawEvidenceRecorder:     params.RawEvidenceRecorder,
		rawEvidenceIngestionRun: strings.TrimSpace(params.RawEvidenceIngestionRun),
	}, nil
}

// ReadInstruments maps Hyperliquid meta universe entries into canonical instruments.
func (v *HyperliquidPerpsVenue) ReadInstruments(
	ctx context.Context,
	request InstrumentReadRequest,
) (InstrumentReadResult, error) {
	canonicalRequest, err := NewInstrumentReadRequest(InstrumentReadRequestParams(request))
	if err != nil {
		return InstrumentReadResult{}, err
	}
	if canonicalRequest.Venue != HyperliquidPerpsVenueName {
		return InstrumentReadResult{}, validationError(
			"hyperliquid perps adapter only serves the hyperliquid-perps venue",
		)
	}

	var response hyperliquidMetaResponse
	metadata, postErr := v.postInfoJSON(ctx, hyperliquidInfoTypeMeta, map[string]any{
		hyperliquidInfoTypeKey: hyperliquidInfoTypeMeta,
		"dex":                  "",
	}, hyperliquidInfoCaptureScope{entityHint: "instrument"}, &response)
	if postErr != nil {
		return InstrumentReadResult{}, postErr
	}

	instruments := make([]domain.Instrument, 0, len(response.Universe))
	for _, item := range response.Universe {
		instrument, instrumentErr := domain.NewInstrument(domain.InstrumentParams{
			Venue:      HyperliquidPerpsVenueName,
			Symbol:     domain.Symbol(item.Name),
			AssetClass: domain.AssetClassFuture,
			Active:     !item.IsDelisted,
		})
		if instrumentErr != nil {
			return InstrumentReadResult{}, validationError("hyperliquid meta returned an invalid instrument")
		}
		if len(canonicalRequest.Symbols) > 0 && !containsSymbol(canonicalRequest.Symbols, instrument.Symbol) {
			continue
		}
		instruments = append(instruments, instrument)
	}

	result, err := NewInstrumentReadResult(instruments, "")
	if err != nil {
		return InstrumentReadResult{}, err
	}
	result.Metadata = metadata

	return result, nil
}

// ReadCandles maps Hyperliquid candle snapshots into canonical candles with half-open time ranges.
func (v *HyperliquidPerpsVenue) ReadCandles(
	ctx context.Context,
	request CandleReadRequest,
) (CandleReadResult, error) {
	canonicalRequest, err := NewCandleReadRequest(CandleReadRequestParams(request))
	if err != nil {
		return CandleReadResult{}, err
	}
	if canonicalRequest.Instrument.Venue != HyperliquidPerpsVenueName {
		return CandleReadResult{}, validationError(
			"hyperliquid perps candle requests must target the hyperliquid-perps venue",
		)
	}

	interval, err := hyperliquidIntervalForTimeframe(canonicalRequest.Timeframe)
	if err != nil {
		return CandleReadResult{}, err
	}

	startTime := canonicalRequest.TimeRange.Start
	if canonicalRequest.PageToken != "" {
		startTime, err = parseHyperliquidStartTimePageToken(canonicalRequest.PageToken, "candle")
		if err != nil {
			return CandleReadResult{}, err
		}
	}

	var rows []hyperliquidCandle
	requestTimeRange := domain.TimeRange{
		Start: startTime,
		End:   canonicalRequest.TimeRange.End,
	}
	metadata, postErr := v.postInfoJSON(ctx, hyperliquidInfoTypeCandle, map[string]any{
		hyperliquidInfoTypeKey: hyperliquidInfoTypeCandle,
		"req": map[string]any{
			hyperliquidRequestCoinKey: canonicalRequest.Instrument.Symbol.String(),
			"interval":                interval,
			"startTime":               startTime.UnixMilli(),
			"endTime":                 canonicalRequest.TimeRange.End.UnixMilli(),
		},
	}, hyperliquidInfoCaptureScope{
		entityHint: "candle",
		instrument: &canonicalRequest.Instrument,
		timeframe:  canonicalRequest.Timeframe,
		timeRange:  &requestTimeRange,
	}, &rows)
	if postErr != nil {
		return CandleReadResult{}, postErr
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].OpenTime < rows[j].OpenTime
	})

	effectiveLimit := canonicalRequest.PageSize
	if effectiveLimit == 0 {
		effectiveLimit = hyperliquidDefaultReadLimit
	}

	candles, nextPageToken, err := buildHyperliquidCandlePage(canonicalRequest, rows, effectiveLimit)
	if err != nil {
		return CandleReadResult{}, err
	}

	result, err := NewCandleReadResult(candles, nextPageToken)
	if err != nil {
		return CandleReadResult{}, err
	}
	result.Metadata = metadata

	return result, nil
}

// ReadTrades maps Hyperliquid recent trades into canonical trades.
func (v *HyperliquidPerpsVenue) ReadTrades(
	ctx context.Context,
	request TradeReadRequest,
) (TradeReadResult, error) {
	canonicalRequest, err := NewTradeReadRequest(TradeReadRequestParams(request))
	if err != nil {
		return TradeReadResult{}, err
	}
	if canonicalRequest.Instrument.Venue != HyperliquidPerpsVenueName {
		return TradeReadResult{}, validationError(
			"hyperliquid perps trade requests must target the hyperliquid-perps venue",
		)
	}

	if canonicalRequest.PageToken != "" {
		return TradeReadResult{}, validationError(
			"hyperliquid perps trade paging is unsupported in v0 because recentTrades does not advance deterministically by page token",
		)
	}

	var rows []hyperliquidTrade
	requestTimeRange := canonicalRequest.TimeRange
	metadata, postErr := v.postInfoJSON(ctx, hyperliquidInfoTypeTrades, map[string]any{
		hyperliquidInfoTypeKey:    hyperliquidInfoTypeTrades,
		hyperliquidRequestCoinKey: canonicalRequest.Instrument.Symbol.String(),
	}, hyperliquidInfoCaptureScope{
		entityHint: "trade",
		instrument: &canonicalRequest.Instrument,
		timeRange:  &requestTimeRange,
	}, &rows)
	if postErr != nil {
		return TradeReadResult{}, postErr
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Time == rows[j].Time {
			return int64(rows[i].TID) < int64(rows[j].TID)
		}
		return rows[i].Time < rows[j].Time
	})

	if rangeErr := validateHyperliquidTradeRangeSupport(canonicalRequest, rows); rangeErr != nil {
		return TradeReadResult{}, rangeErr
	}

	effectiveLimit := canonicalRequest.PageSize
	if effectiveLimit == 0 {
		effectiveLimit = hyperliquidDefaultReadLimit
	}

	trades := make([]domain.Trade, 0, smallerInt(len(rows), effectiveLimit))
	for _, row := range rows {
		trade, mapErr := mapHyperliquidTrade(canonicalRequest.Instrument, row)
		if mapErr != nil {
			return TradeReadResult{}, mapErr
		}
		if trade.EventTime.Before(canonicalRequest.TimeRange.Start) ||
			!trade.EventTime.Before(canonicalRequest.TimeRange.End) {
			continue
		}
		if len(trades) == effectiveLimit {
			return TradeReadResult{}, validationError(
				"hyperliquid perps trade reads that require paging are unsupported in v0 because recentTrades cannot advance deterministically",
			)
		}
		trades = append(trades, trade)
	}

	result, err := NewTradeReadResult(trades, "")
	if err != nil {
		return TradeReadResult{}, err
	}
	result.Metadata = metadata

	return result, nil
}

func validateHyperliquidTradeRangeSupport(
	request TradeReadRequest,
	rows []hyperliquidTrade,
) error {
	if len(rows) == 0 {
		return nil
	}

	earliestAvailable := time.UnixMilli(rows[0].Time)
	if request.TimeRange.Start.Before(earliestAvailable) {
		return validationError(
			"hyperliquid perps trade time range is unsupported in v0 because recentTrades only exposes the latest venue window",
		)
	}

	return nil
}

func buildHyperliquidCandlePage(
	request CandleReadRequest,
	rows []hyperliquidCandle,
	effectiveLimit int,
) ([]domain.Candle, string, error) {
	candles := make([]domain.Candle, 0, smallerInt(len(rows), effectiveLimit))
	var nextPageToken string
	for _, row := range rows {
		candle, err := mapHyperliquidCandle(request.Instrument, request.Timeframe, row)
		if err != nil {
			return nil, "", err
		}
		if candle.TimeRange.Start.Before(request.TimeRange.Start) ||
			!candle.TimeRange.Start.Before(request.TimeRange.End) {
			continue
		}
		if len(candles) == effectiveLimit {
			nextPageToken = fmt.Sprintf("startTime:%d", candle.TimeRange.Start.UnixMilli())
			break
		}
		candles = append(candles, candle)
	}

	if nextPageToken == "" && len(candles) > 0 && len(rows) >= hyperliquidDefaultReadLimit {
		nextStart := candles[len(candles)-1].TimeRange.End
		if nextStart.Before(request.TimeRange.End) {
			nextPageToken = fmt.Sprintf("startTime:%d", nextStart.UnixMilli())
		}
	}

	return candles, nextPageToken, nil
}

func (v *HyperliquidPerpsVenue) postInfoJSON(
	ctx context.Context,
	requestType string,
	payload any,
	scope hyperliquidInfoCaptureScope,
	target any,
) (ReadResultMetadata, error) {
	requestBody, err := json.Marshal(payload)
	if err != nil {
		return ReadResultMetadata{}, fmt.Errorf("marshal request body: %w", err)
	}

	requestAt := time.Now()
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		v.baseURL+hyperliquidInfoEndpoint,
		bytes.NewReader(requestBody),
	)
	if err != nil {
		return ReadResultMetadata{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return ReadResultMetadata{}, fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ReadResultMetadata{}, fmt.Errorf("read response body: %w", err)
	}
	responseAt := time.Now()
	receivedAt := time.Now()

	metadata, err := v.recordRawEvidence(ctx, HyperliquidRawEvidenceCapture{
		ID:                 uuid.NewString(),
		IngestionRunID:     v.rawEvidenceIngestionRun,
		Venue:              HyperliquidPerpsVenueName,
		Endpoint:           hyperliquidInfoEndpoint,
		RequestType:        requestType,
		RequestPayloadHash: hashBytesSHA256(requestBody),
		RequestMetadata: map[string]string{
			"content-type": req.Header.Get("Content-Type"),
			"method":       req.Method,
		},
		RequestAt:    requestAt,
		ResponseAt:   responseAt,
		HTTPStatus:   resp.StatusCode,
		ResponseBody: body,
		EntityHint:   scope.entityHint,
		Instrument:   scope.instrument,
		Timeframe:    scope.timeframe,
		TimeRange:    scope.timeRange,
		ReceivedAt:   receivedAt,
	})
	if err != nil {
		return ReadResultMetadata{}, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ReadResultMetadata{}, hyperliquidHTTPError(resp.StatusCode, body)
	}

	if decodeErr := json.Unmarshal(body, target); decodeErr != nil {
		return ReadResultMetadata{}, fmt.Errorf("decode response body: %w", decodeErr)
	}

	return metadata, nil
}

func (v *HyperliquidPerpsVenue) recordRawEvidence(
	ctx context.Context,
	capture HyperliquidRawEvidenceCapture,
) (ReadResultMetadata, error) {
	if v.rawEvidenceRecorder == nil {
		return ReadResultMetadata{}, nil
	}

	rawPayloadID, err := v.rawEvidenceRecorder.RecordHyperliquidRawEvidence(ctx, capture)
	if err != nil {
		return ReadResultMetadata{}, fmt.Errorf("record hyperliquid raw evidence: %w", err)
	}

	canonicalRawPayloadID := strings.TrimSpace(rawPayloadID)
	if canonicalRawPayloadID == "" {
		return ReadResultMetadata{}, nil
	}

	return ReadResultMetadata{RawPayloadIDs: []string{canonicalRawPayloadID}}, nil
}

func hashBytesSHA256(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func hyperliquidIntervalForTimeframe(timeframe domain.Timeframe) (string, error) {
	switch timeframe {
	case domain.Timeframe1m:
		return "1m", nil
	case domain.Timeframe5m:
		return "5m", nil
	case domain.Timeframe15m:
		return "15m", nil
	case domain.Timeframe1h:
		return "1h", nil
	case domain.Timeframe4h:
		return "4h", nil
	case domain.Timeframe1d:
		return "1d", nil
	default:
		return "", validationError("hyperliquid perps timeframe is unsupported")
	}
}

func parseHyperliquidStartTimePageToken(pageToken, tokenKind string) (time.Time, error) {
	if !strings.HasPrefix(pageToken, "startTime:") {
		return time.Time{}, validationError(
			fmt.Sprintf("hyperliquid perps %s page token is invalid", tokenKind),
		)
	}

	milliseconds, err := strconv.ParseInt(strings.TrimPrefix(pageToken, "startTime:"), 10, 64)
	if err != nil || milliseconds < 0 {
		return time.Time{}, validationError(
			fmt.Sprintf("hyperliquid perps %s page token is invalid", tokenKind),
		)
	}

	return time.UnixMilli(milliseconds), nil
}

func hyperliquidHTTPError(statusCode int, body []byte) error {
	var venueError hyperliquidVenueError
	if unmarshalErr := json.Unmarshal(body, &venueError); unmarshalErr == nil {
		message := strings.TrimSpace(venueError.Error)
		if message == "" {
			message = strings.TrimSpace(venueError.Message)
		}
		if message != "" {
			return fmt.Errorf("hyperliquid perps error: %s", message)
		}
	}

	if message := strings.TrimSpace(string(body)); message != "" {
		return fmt.Errorf("hyperliquid perps http status %d: %s", statusCode, message)
	}

	return fmt.Errorf("hyperliquid perps http status %d", statusCode)
}

func mapHyperliquidCandle(
	instrument domain.Instrument,
	timeframe domain.Timeframe,
	row hyperliquidCandle,
) (domain.Candle, error) {
	if row.Symbol != instrument.Symbol.String() {
		return domain.Candle{}, errors.New("decode response body: candle symbol does not match request")
	}
	if row.Interval != timeframe.String() {
		return domain.Candle{}, errors.New("decode response body: candle interval does not match request")
	}

	start := time.UnixMilli(row.OpenTime)
	end := time.UnixMilli(row.CloseTime).Add(time.Millisecond)
	provenance, err := domain.NewSourceProvenance(
		string(HyperliquidPerpsVenueName)+"-rest",
		fmt.Sprintf("candle:%s:%s:%d", row.Symbol, row.Interval, row.OpenTime),
	)
	if err != nil {
		return domain.Candle{}, fmt.Errorf("build candle provenance: %w", err)
	}

	return domain.NewCandle(domain.CandleParams{
		Instrument: instrument,
		Timeframe:  timeframe,
		TimeRange: domain.TimeRange{
			Start: start,
			End:   end,
		},
		Open:       float64(row.Open),
		High:       float64(row.High),
		Low:        float64(row.Low),
		Close:      float64(row.Close),
		Volume:     float64(row.Volume),
		Quality:    domain.DataQualityRaw,
		Provenance: provenance,
	})
}

func mapHyperliquidTrade(instrument domain.Instrument, row hyperliquidTrade) (domain.Trade, error) {
	if row.Symbol != instrument.Symbol.String() {
		return domain.Trade{}, errors.New("decode response body: trade symbol does not match request")
	}

	provenance, err := domain.NewSourceProvenance(
		string(HyperliquidPerpsVenueName)+"-rest",
		fmt.Sprintf("trade:%s:%d:%d", row.Symbol, row.Time, int64(row.TID)),
	)
	if err != nil {
		return domain.Trade{}, fmt.Errorf("build trade provenance: %w", err)
	}

	return domain.NewTrade(domain.TradeParams{
		Instrument: instrument,
		EventTime:  time.UnixMilli(row.Time),
		Price:      float64(row.Price),
		Size:       float64(row.Size),
		Quality:    domain.DataQualityRaw,
		Provenance: provenance,
	})
}

type hyperliquidDecimalValue float64

func (v *hyperliquidDecimalValue) UnmarshalJSON(data []byte) error {
	text, err := unquoteOrKeepJSONScalar(data)
	if err != nil {
		return err
	}

	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return err
	}

	*v = hyperliquidDecimalValue(parsed)

	return nil
}

type hyperliquidInt64Value int64

func (v *hyperliquidInt64Value) UnmarshalJSON(data []byte) error {
	text, err := unquoteOrKeepJSONScalar(data)
	if err != nil {
		return err
	}

	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return err
	}

	*v = hyperliquidInt64Value(parsed)

	return nil
}

func unquoteOrKeepJSONScalar(data []byte) (string, error) {
	if len(data) == 0 {
		return "", errors.New("empty scalar")
	}

	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return value, nil
	}

	return string(data), nil
}

func smallerInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}
