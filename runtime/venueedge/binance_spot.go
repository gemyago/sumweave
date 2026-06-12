package venueedge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
)

const (
	binanceFiveMinuteDuration    = 5 * time.Minute
	binanceFifteenMinuteDuration = 15 * time.Minute
	binanceFourHourDuration      = 4 * time.Hour
	binanceOneDayDuration        = 24 * time.Hour
	binanceMinKlineFieldCount    = 6
	binanceDefaultReadLimit      = 1000
)

// BinanceSpotVenueName is the canonical venue used by the first real HTTP adapter.
const BinanceSpotVenueName domain.Venue = "binance-spot"

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// BinanceSpotVenueParams configures the concrete Binance Spot market-data adapter.
type BinanceSpotVenueParams struct {
	BaseURL    string
	HTTPClient httpDoer
}

type binanceVenueError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type binanceExchangeInfoResponse struct {
	Symbols []struct {
		Symbol string `json:"symbol"`
		Status string `json:"status"`
	} `json:"symbols"`
}

type binanceAggTrade struct {
	AggregateTradeID int64  `json:"a"`
	Price            string `json:"p"`
	Quantity         string `json:"q"`
	Timestamp        int64  `json:"T"`
}

// BinanceSpotVenue adapts documented Binance Spot market-data endpoints into canonical records.
type BinanceSpotVenue struct {
	baseURL    string
	httpClient httpDoer
}

// NewBinanceSpotVenue creates a concrete mocked-HTTP-friendly Binance Spot adapter.
func NewBinanceSpotVenue(params BinanceSpotVenueParams) (*BinanceSpotVenue, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(params.BaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("base URL is required")
	}
	if params.HTTPClient == nil {
		return nil, errors.New("http client is required")
	}

	return &BinanceSpotVenue{
		baseURL:    baseURL,
		httpClient: params.HTTPClient,
	}, nil
}

// ReadInstruments maps Binance exchangeInfo symbols into canonical instruments.
func (v *BinanceSpotVenue) ReadInstruments(
	ctx context.Context,
	request InstrumentReadRequest,
) (InstrumentReadResult, error) {
	canonicalRequest, err := NewInstrumentReadRequest(InstrumentReadRequestParams(request))
	if err != nil {
		return InstrumentReadResult{}, err
	}
	if canonicalRequest.Venue != BinanceSpotVenueName {
		return InstrumentReadResult{}, validationError("binance spot adapter only serves the binance-spot venue")
	}

	query := url.Values{}
	if len(canonicalRequest.Symbols) == 1 {
		query.Set("symbol", canonicalRequest.Symbols[0].String())
	}
	if len(canonicalRequest.Symbols) > 1 {
		symbols := make([]string, 0, len(canonicalRequest.Symbols))
		for _, symbol := range canonicalRequest.Symbols {
			symbols = append(symbols, symbol.String())
		}
		payload, marshalErr := json.Marshal(symbols)
		if marshalErr != nil {
			return InstrumentReadResult{}, fmt.Errorf("marshal symbols: %w", marshalErr)
		}
		query.Set("symbols", string(payload))
	}

	var response binanceExchangeInfoResponse
	getErr := v.getJSON(ctx, "/api/v3/exchangeInfo", query, &response)
	if getErr != nil {
		return InstrumentReadResult{}, getErr
	}

	instruments := make([]domain.Instrument, 0, len(response.Symbols))
	for _, item := range response.Symbols {
		instrument, instrumentErr := domain.NewInstrument(domain.InstrumentParams{
			Venue:      BinanceSpotVenueName,
			Symbol:     domain.Symbol(item.Symbol),
			AssetClass: domain.AssetClassCrypto,
			Active:     item.Status == "TRADING",
		})
		if instrumentErr != nil {
			return InstrumentReadResult{}, validationError("binance exchangeInfo returned an invalid instrument")
		}
		instruments = append(instruments, instrument)
	}

	return NewInstrumentReadResult(instruments, "")
}

// ReadCandles maps Binance klines into canonical candles with half-open time ranges.
func (v *BinanceSpotVenue) ReadCandles(
	ctx context.Context,
	request CandleReadRequest,
) (CandleReadResult, error) {
	canonicalRequest, err := NewCandleReadRequest(CandleReadRequestParams(request))
	if err != nil {
		return CandleReadResult{}, err
	}
	if canonicalRequest.Instrument.Venue != BinanceSpotVenueName {
		return CandleReadResult{}, validationError("binance spot candle requests must target the binance-spot venue")
	}

	interval, duration, err := binanceIntervalForTimeframe(canonicalRequest.Timeframe)
	if err != nil {
		return CandleReadResult{}, err
	}

	startTime := canonicalRequest.TimeRange.Start
	if canonicalRequest.PageToken != "" {
		startTime, err = parseBinanceCandlePageToken(canonicalRequest.PageToken)
		if err != nil {
			return CandleReadResult{}, err
		}
	}

	limit := canonicalRequest.PageSize
	if limit == 0 {
		limit = binanceDefaultReadLimit
	}

	query := url.Values{}
	query.Set("symbol", canonicalRequest.Instrument.Symbol.String())
	query.Set("interval", interval)
	query.Set("startTime", strconv.FormatInt(startTime.UnixMilli(), 10))
	query.Set("endTime", strconv.FormatInt(canonicalRequest.TimeRange.End.UnixMilli(), 10))
	query.Set("limit", strconv.Itoa(limit))

	var rows [][]any
	getErr := v.getJSON(ctx, "/api/v3/klines", query, &rows)
	if getErr != nil {
		return CandleReadResult{}, getErr
	}

	candles := make([]domain.Candle, 0, len(rows))
	for _, row := range rows {
		candle, mapErr := mapBinanceKlineRow(
			canonicalRequest.Instrument,
			canonicalRequest.Timeframe,
			duration,
			row,
		)
		if mapErr != nil {
			return CandleReadResult{}, mapErr
		}
		if candle.TimeRange.Start.Before(canonicalRequest.TimeRange.Start) ||
			!candle.TimeRange.Start.Before(canonicalRequest.TimeRange.End) {
			continue
		}
		candles = append(candles, candle)
	}

	nextPageToken := ""
	if len(candles) == limit {
		nextStart := candles[len(candles)-1].TimeRange.Start.Add(duration)
		if nextStart.Before(canonicalRequest.TimeRange.End) {
			nextPageToken = fmt.Sprintf("start:%d", nextStart.UnixMilli())
		}
	}

	return NewCandleReadResult(candles, nextPageToken)
}

// ReadTrades maps Binance aggregate trades into canonical trades.
func (v *BinanceSpotVenue) ReadTrades(
	ctx context.Context,
	request TradeReadRequest,
) (TradeReadResult, error) {
	canonicalRequest, err := NewTradeReadRequest(TradeReadRequestParams(request))
	if err != nil {
		return TradeReadResult{}, err
	}
	if canonicalRequest.Instrument.Venue != BinanceSpotVenueName {
		return TradeReadResult{}, validationError("binance spot trade requests must target the binance-spot venue")
	}

	limit := canonicalRequest.PageSize
	if limit == 0 {
		limit = binanceDefaultReadLimit
	}

	query := url.Values{}
	query.Set("symbol", canonicalRequest.Instrument.Symbol.String())
	query.Set("limit", strconv.Itoa(limit))

	if canonicalRequest.PageToken == "" {
		query.Set("startTime", strconv.FormatInt(canonicalRequest.TimeRange.Start.UnixMilli(), 10))
		query.Set("endTime", strconv.FormatInt(canonicalRequest.TimeRange.End.Add(-time.Millisecond).UnixMilli(), 10))
	} else {
		fromID, tokenErr := parseBinanceTradePageToken(canonicalRequest.PageToken)
		if tokenErr != nil {
			return TradeReadResult{}, tokenErr
		}
		query.Set("fromId", strconv.FormatInt(fromID, 10))
	}

	var rows []binanceAggTrade
	getErr := v.getJSON(ctx, "/api/v3/aggTrades", query, &rows)
	if getErr != nil {
		return TradeReadResult{}, getErr
	}

	trades := make([]domain.Trade, 0, len(rows))
	var lastID int64
	for _, row := range rows {
		trade, mapErr := mapBinanceAggTrade(canonicalRequest.Instrument, row)
		if mapErr != nil {
			return TradeReadResult{}, mapErr
		}
		if trade.EventTime.Before(canonicalRequest.TimeRange.Start) ||
			!trade.EventTime.Before(canonicalRequest.TimeRange.End) {
			continue
		}
		lastID = row.AggregateTradeID
		trades = append(trades, trade)
	}

	nextPageToken := ""
	if len(rows) == limit && len(trades) > 0 {
		lastTime := trades[len(trades)-1].EventTime
		if lastTime.Before(canonicalRequest.TimeRange.End) {
			nextPageToken = fmt.Sprintf("fromId:%d", lastID+1)
		}
	}

	return NewTradeReadResult(trades, nextPageToken)
}

func (v *BinanceSpotVenue) getJSON(
	ctx context.Context,
	path string,
	query url.Values,
	target any,
) error {
	requestURL := v.baseURL + path
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var venueError binanceVenueError
		unmarshalErr := json.Unmarshal(body, &venueError)
		if unmarshalErr == nil && venueError.Msg != "" {
			return fmt.Errorf("binance spot error %d: %s", venueError.Code, venueError.Msg)
		}
		return fmt.Errorf("binance spot http status %d", resp.StatusCode)
	}

	unmarshalErr := json.Unmarshal(body, target)
	if unmarshalErr != nil {
		return fmt.Errorf("decode response body: %w", unmarshalErr)
	}

	return nil
}

func binanceIntervalForTimeframe(timeframe domain.Timeframe) (string, time.Duration, error) {
	switch timeframe {
	case domain.Timeframe1m:
		return "1m", time.Minute, nil
	case domain.Timeframe5m:
		return "5m", binanceFiveMinuteDuration, nil
	case domain.Timeframe15m:
		return "15m", binanceFifteenMinuteDuration, nil
	case domain.Timeframe1h:
		return "1h", time.Hour, nil
	case domain.Timeframe4h:
		return "4h", binanceFourHourDuration, nil
	case domain.Timeframe1d:
		return "1d", binanceOneDayDuration, nil
	default:
		return "", 0, validationError("binance spot timeframe is unsupported")
	}
}

func parseBinanceCandlePageToken(pageToken string) (time.Time, error) {
	if !strings.HasPrefix(pageToken, "start:") {
		return time.Time{}, validationError("binance spot candle page token is invalid")
	}
	milliseconds, err := strconv.ParseInt(strings.TrimPrefix(pageToken, "start:"), 10, 64)
	if err != nil {
		return time.Time{}, validationError("binance spot candle page token is invalid")
	}

	return time.UnixMilli(milliseconds).UTC(), nil
}

func parseBinanceTradePageToken(pageToken string) (int64, error) {
	if !strings.HasPrefix(pageToken, "fromId:") {
		return 0, validationError("binance spot trade page token is invalid")
	}
	fromID, err := strconv.ParseInt(strings.TrimPrefix(pageToken, "fromId:"), 10, 64)
	if err != nil || fromID < 0 {
		return 0, validationError("binance spot trade page token is invalid")
	}

	return fromID, nil
}

func mapBinanceKlineRow(
	instrument domain.Instrument,
	timeframe domain.Timeframe,
	duration time.Duration,
	row []any,
) (domain.Candle, error) {
	if len(row) < binanceMinKlineFieldCount {
		return domain.Candle{}, errors.New("decode response body: invalid kline row")
	}

	openTime, err := jsonNumberToInt64(row[0])
	if err != nil {
		return domain.Candle{}, fmt.Errorf("decode response body: invalid kline open time: %w", err)
	}
	openPrice, err := jsonNumberToFloat64(row[1])
	if err != nil {
		return domain.Candle{}, fmt.Errorf("decode response body: invalid kline open price: %w", err)
	}
	highPrice, err := jsonNumberToFloat64(row[2])
	if err != nil {
		return domain.Candle{}, fmt.Errorf("decode response body: invalid kline high price: %w", err)
	}
	lowPrice, err := jsonNumberToFloat64(row[3])
	if err != nil {
		return domain.Candle{}, fmt.Errorf("decode response body: invalid kline low price: %w", err)
	}
	closePrice, err := jsonNumberToFloat64(row[4])
	if err != nil {
		return domain.Candle{}, fmt.Errorf("decode response body: invalid kline close price: %w", err)
	}
	volume, err := jsonNumberToFloat64(row[5])
	if err != nil {
		return domain.Candle{}, fmt.Errorf("decode response body: invalid kline volume: %w", err)
	}

	start := time.UnixMilli(openTime).UTC()
	provenance, err := domain.NewSourceProvenance(
		string(BinanceSpotVenueName)+"-rest",
		fmt.Sprintf("kline:%s:%s:%d", instrument.Symbol, timeframe, openTime),
	)
	if err != nil {
		return domain.Candle{}, fmt.Errorf("build kline provenance: %w", err)
	}

	return domain.NewCandle(domain.CandleParams{
		Instrument: instrument,
		Timeframe:  timeframe,
		TimeRange: domain.TimeRange{
			Start: start,
			End:   start.Add(duration),
		},
		Open:       openPrice,
		High:       highPrice,
		Low:        lowPrice,
		Close:      closePrice,
		Volume:     volume,
		Quality:    domain.DataQualityRaw,
		Provenance: provenance,
	})
}

func mapBinanceAggTrade(instrument domain.Instrument, row binanceAggTrade) (domain.Trade, error) {
	price, err := strconv.ParseFloat(row.Price, 64)
	if err != nil {
		return domain.Trade{}, fmt.Errorf("decode response body: invalid trade price: %w", err)
	}
	quantity, err := strconv.ParseFloat(row.Quantity, 64)
	if err != nil {
		return domain.Trade{}, fmt.Errorf("decode response body: invalid trade quantity: %w", err)
	}

	provenance, err := domain.NewSourceProvenance(
		string(BinanceSpotVenueName)+"-rest",
		fmt.Sprintf("aggTrade:%d", row.AggregateTradeID),
	)
	if err != nil {
		return domain.Trade{}, fmt.Errorf("build trade provenance: %w", err)
	}

	return domain.NewTrade(domain.TradeParams{
		Instrument: instrument,
		EventTime:  time.UnixMilli(row.Timestamp).UTC(),
		Price:      price,
		Size:       quantity,
		Quality:    domain.DataQualityRaw,
		Provenance: provenance,
	})
}

func jsonNumberToInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	default:
		return 0, fmt.Errorf("unexpected type %T", value)
	}
}

func jsonNumberToFloat64(value any) (float64, error) {
	switch typed := value.(type) {
	case float64:
		return typed, nil
	case string:
		return strconv.ParseFloat(typed, 64)
	default:
		return 0, fmt.Errorf("unexpected type %T", value)
	}
}
