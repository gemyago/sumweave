package venueedge

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gemyago/signal-foundry/runtime/domain"
)

// ErrValidation marks venue-edge inputs or outputs that fail canonical validation.
var ErrValidation = errors.New("venue edge validation failed")

// MarketDataVenue exposes narrow canonical market-data reads from a venue edge.
type MarketDataVenue interface {
	ReadInstruments(ctx context.Context, request InstrumentReadRequest) (InstrumentReadResult, error)
	ReadCandles(ctx context.Context, request CandleReadRequest) (CandleReadResult, error)
	ReadTrades(ctx context.Context, request TradeReadRequest) (TradeReadResult, error)
}

// InstrumentReadRequest scopes canonical instrument reads for a venue.
type InstrumentReadRequest struct {
	Venue     domain.Venue
	Symbols   []domain.Symbol
	PageSize  int
	PageToken string
}

// CandleReadRequest scopes canonical candle reads for one instrument and half-open range.
type CandleReadRequest struct {
	Instrument domain.Instrument
	Timeframe  domain.Timeframe
	TimeRange  domain.TimeRange
	PageSize   int
	PageToken  string
}

// TradeReadRequest scopes canonical trade reads for one instrument and half-open range.
type TradeReadRequest struct {
	Instrument domain.Instrument
	TimeRange  domain.TimeRange
	PageSize   int
	PageToken  string
}

// ReadResultMetadata carries optional non-canonical read metadata.
type ReadResultMetadata struct {
	RawPayloadIDs []string
}

// InstrumentReadResult returns canonical venue instruments plus optional paging state.
type InstrumentReadResult struct {
	Instruments   []domain.Instrument
	NextPageToken string
	Metadata      ReadResultMetadata
}

// CandleReadResult returns canonical candles plus optional paging state.
type CandleReadResult struct {
	Candles       []domain.Candle
	NextPageToken string
	Metadata      ReadResultMetadata
}

// TradeReadResult returns canonical trades plus optional paging state.
type TradeReadResult struct {
	Trades        []domain.Trade
	NextPageToken string
	Metadata      ReadResultMetadata
}

// InstrumentReadRequestParams holds raw inputs for a canonical instrument read request.
type InstrumentReadRequestParams struct {
	Venue     domain.Venue
	Symbols   []domain.Symbol
	PageSize  int
	PageToken string
}

// CandleReadRequestParams holds raw inputs for a canonical candle read request.
type CandleReadRequestParams struct {
	Instrument domain.Instrument
	Timeframe  domain.Timeframe
	TimeRange  domain.TimeRange
	PageSize   int
	PageToken  string
}

// TradeReadRequestParams holds raw inputs for a canonical trade read request.
type TradeReadRequestParams struct {
	Instrument domain.Instrument
	TimeRange  domain.TimeRange
	PageSize   int
	PageToken  string
}

// NewInstrumentReadRequest validates and canonicalizes a venue instrument read request.
func NewInstrumentReadRequest(params InstrumentReadRequestParams) (InstrumentReadRequest, error) {
	venue, err := canonicalizeVenue(params.Venue)
	if err != nil {
		return InstrumentReadRequest{}, validationError("instrument read venue is required")
	}

	symbols, err := canonicalizeSymbols(params.Symbols)
	if err != nil {
		return InstrumentReadRequest{}, err
	}

	pageSize, pageToken, err := canonicalizePage(params.PageSize, params.PageToken)
	if err != nil {
		return InstrumentReadRequest{}, err
	}

	return InstrumentReadRequest{
		Venue:     venue,
		Symbols:   symbols,
		PageSize:  pageSize,
		PageToken: pageToken,
	}, nil
}

// NewCandleReadRequest validates and canonicalizes a venue candle read request.
func NewCandleReadRequest(params CandleReadRequestParams) (CandleReadRequest, error) {
	instrument, err := canonicalizeInstrument(params.Instrument)
	if err != nil {
		return CandleReadRequest{}, validationError("candle read instrument is required")
	}

	timeframe, err := domain.NewTimeframe(params.Timeframe.String())
	if err != nil {
		return CandleReadRequest{}, validationError("candle read timeframe is required")
	}

	timeRange, err := domain.NewTimeRange(params.TimeRange.Start, params.TimeRange.End)
	if err != nil {
		return CandleReadRequest{}, validationError("candle read time range is required")
	}

	pageSize, pageToken, err := canonicalizePage(params.PageSize, params.PageToken)
	if err != nil {
		return CandleReadRequest{}, err
	}

	return CandleReadRequest{
		Instrument: instrument,
		Timeframe:  timeframe,
		TimeRange:  timeRange,
		PageSize:   pageSize,
		PageToken:  pageToken,
	}, nil
}

// NewTradeReadRequest validates and canonicalizes a venue trade read request.
func NewTradeReadRequest(params TradeReadRequestParams) (TradeReadRequest, error) {
	instrument, err := canonicalizeInstrument(params.Instrument)
	if err != nil {
		return TradeReadRequest{}, validationError("trade read instrument is required")
	}

	timeRange, err := domain.NewTimeRange(params.TimeRange.Start, params.TimeRange.End)
	if err != nil {
		return TradeReadRequest{}, validationError("trade read time range is required")
	}

	pageSize, pageToken, err := canonicalizePage(params.PageSize, params.PageToken)
	if err != nil {
		return TradeReadRequest{}, err
	}

	return TradeReadRequest{
		Instrument: instrument,
		TimeRange:  timeRange,
		PageSize:   pageSize,
		PageToken:  pageToken,
	}, nil
}

// NewInstrumentReadResult validates and canonicalizes canonical instrument results.
func NewInstrumentReadResult(
	instruments []domain.Instrument,
	nextPageToken string,
) (InstrumentReadResult, error) {
	canonicalInstruments, err := canonicalizeInstruments(instruments)
	if err != nil {
		return InstrumentReadResult{}, err
	}

	return InstrumentReadResult{
		Instruments:   canonicalInstruments,
		NextPageToken: strings.TrimSpace(nextPageToken),
		Metadata:      ReadResultMetadata{},
	}, nil
}

// NewCandleReadResult validates and canonicalizes canonical candle results.
func NewCandleReadResult(candles []domain.Candle, nextPageToken string) (CandleReadResult, error) {
	canonicalCandles, err := canonicalizeCandles(candles)
	if err != nil {
		return CandleReadResult{}, err
	}

	return CandleReadResult{
		Candles:       canonicalCandles,
		NextPageToken: strings.TrimSpace(nextPageToken),
		Metadata:      ReadResultMetadata{},
	}, nil
}

// NewTradeReadResult validates and canonicalizes canonical trade results.
func NewTradeReadResult(trades []domain.Trade, nextPageToken string) (TradeReadResult, error) {
	canonicalTrades, err := canonicalizeTrades(trades)
	if err != nil {
		return TradeReadResult{}, err
	}

	return TradeReadResult{
		Trades:        canonicalTrades,
		NextPageToken: strings.TrimSpace(nextPageToken),
		Metadata:      ReadResultMetadata{},
	}, nil
}

func canonicalizeVenue(value domain.Venue) (domain.Venue, error) {
	return domain.NewVenue(value.String())
}

func canonicalizeSymbols(values []domain.Symbol) ([]domain.Symbol, error) {
	symbols := make([]domain.Symbol, 0, len(values))
	for _, value := range values {
		symbol, err := domain.NewSymbol(value.String())
		if err != nil {
			return nil, validationError("instrument read symbols must be canonical")
		}
		symbols = append(symbols, symbol)
	}

	return symbols, nil
}

func canonicalizePage(pageSize int, pageToken string) (int, string, error) {
	if pageSize < 0 {
		return 0, "", validationError("page size must be zero or positive")
	}

	return pageSize, strings.TrimSpace(pageToken), nil
}

func canonicalizeInstrument(value domain.Instrument) (domain.Instrument, error) {
	venue, err := domain.NewVenue(value.Venue.String())
	if err != nil {
		return domain.Instrument{}, err
	}

	symbol, err := domain.NewSymbol(value.Symbol.String())
	if err != nil {
		return domain.Instrument{}, err
	}

	assetClass, err := domain.NewAssetClass(value.AssetClass.String())
	if err != nil {
		return domain.Instrument{}, err
	}

	return domain.NewInstrument(domain.InstrumentParams{
		Venue:      venue,
		Symbol:     symbol,
		AssetClass: assetClass,
		Active:     value.Active,
	})
}

func canonicalizeInstruments(values []domain.Instrument) ([]domain.Instrument, error) {
	instruments := make([]domain.Instrument, 0, len(values))
	for _, value := range values {
		instrument, err := canonicalizeInstrument(value)
		if err != nil {
			return nil, validationError("instrument result must contain canonical instruments")
		}
		instruments = append(instruments, instrument)
	}

	return instruments, nil
}

func canonicalizeCandles(values []domain.Candle) ([]domain.Candle, error) {
	candles := make([]domain.Candle, 0, len(values))
	for _, value := range values {
		instrument, err := canonicalizeInstrument(value.Instrument)
		if err != nil {
			return nil, validationError("candle result must contain canonical candles")
		}

		timeframe, err := domain.NewTimeframe(value.Timeframe.String())
		if err != nil {
			return nil, validationError("candle result must contain canonical candles")
		}

		timeRange, err := domain.NewTimeRange(value.TimeRange.Start, value.TimeRange.End)
		if err != nil {
			return nil, validationError("candle result must contain canonical candles")
		}

		quality, err := domain.NewDataQuality(value.Quality.String())
		if err != nil {
			return nil, validationError("candle result must contain canonical candles")
		}

		provenance, err := domain.NewSourceProvenance(
			value.Provenance.Source,
			value.Provenance.RecordID,
		)
		if err != nil {
			return nil, validationError("candle result must contain canonical candles")
		}

		candle, err := domain.NewCandle(domain.CandleParams{
			Instrument: instrument,
			Timeframe:  timeframe,
			TimeRange:  timeRange,
			Open:       value.Open,
			High:       value.High,
			Low:        value.Low,
			Close:      value.Close,
			Volume:     value.Volume,
			Quality:    quality,
			Provenance: provenance,
		})
		if err != nil {
			return nil, validationError("candle result must contain canonical candles")
		}

		candles = append(candles, candle)
	}

	return candles, nil
}

func canonicalizeTrades(values []domain.Trade) ([]domain.Trade, error) {
	trades := make([]domain.Trade, 0, len(values))
	for _, value := range values {
		instrument, err := canonicalizeInstrument(value.Instrument)
		if err != nil {
			return nil, validationError("trade result must contain canonical trades")
		}

		quality, err := domain.NewDataQuality(value.Quality.String())
		if err != nil {
			return nil, validationError("trade result must contain canonical trades")
		}

		provenance, err := domain.NewSourceProvenance(
			value.Provenance.Source,
			value.Provenance.RecordID,
		)
		if err != nil {
			return nil, validationError("trade result must contain canonical trades")
		}

		trade, err := domain.NewTrade(domain.TradeParams{
			Instrument: instrument,
			EventTime:  value.EventTime,
			Price:      value.Price,
			Size:       value.Size,
			Quality:    quality,
			Provenance: provenance,
		})
		if err != nil {
			return nil, validationError("trade result must contain canonical trades")
		}

		trades = append(trades, trade)
	}

	return trades, nil
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrValidation, message)
}
