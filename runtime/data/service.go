package data

import (
	"context"
	"errors"
	"fmt"

	"github.com/gemyago/signal-foundry/runtime/domain"
)

// ErrValidation marks rejected inputs that fail data-layer validation.
var ErrValidation = errors.New("data validation failed")

// ErrInstrumentNotFound marks a missing canonical instrument lookup result.
var ErrInstrumentNotFound = errors.New("instrument not found")

type instrumentUpsertStore interface {
	UpsertInstrument(ctx context.Context, instrument domain.Instrument) (domain.Instrument, error)
}

type candleUpsertStore interface {
	UpsertCandle(ctx context.Context, candle domain.Candle) (domain.Candle, error)
}

type tradeUpsertStore interface {
	UpsertTrade(ctx context.Context, trade domain.Trade) (domain.Trade, error)
}

type instrumentLookupStore interface {
	LookupInstrument(ctx context.Context, venue domain.Venue, symbol domain.Symbol) (*domain.Instrument, error)
}

type candleQueryStore interface {
	QueryCandles(
		ctx context.Context,
		instrument domain.Instrument,
		timeframe domain.Timeframe,
		timeRange domain.TimeRange,
	) ([]domain.Candle, error)
	ReplayCandles(
		ctx context.Context,
		instrument domain.Instrument,
		timeframe domain.Timeframe,
		timeRange domain.TimeRange,
	) ([]ReplayCandle, error)
}

type tradeQueryStore interface {
	QueryTrades(
		ctx context.Context,
		instrument domain.Instrument,
		timeRange domain.TimeRange,
	) ([]domain.Trade, error)
	ReplayTrades(
		ctx context.Context,
		instrument domain.Instrument,
		timeRange domain.TimeRange,
	) ([]ReplayTrade, error)
}

// IngestionServiceDeps configures ingestion dependencies.
type IngestionServiceDeps struct {
	InstrumentStore instrumentUpsertStore
	CandleStore     candleUpsertStore
	TradeStore      tradeUpsertStore
}

// IngestionService validates and persists canonical instruments and market data.
type IngestionService struct {
	instrumentStore instrumentUpsertStore
	candleStore     candleUpsertStore
	tradeStore      tradeUpsertStore
}

// ReadServiceDeps configures read dependencies.
type ReadServiceDeps struct {
	InstrumentStore instrumentLookupStore
	CandleStore     candleQueryStore
	TradeStore      tradeQueryStore
}

// ReadService exposes deterministic read operations.
type ReadService struct {
	instrumentStore instrumentLookupStore
	candleStore     candleQueryStore
	tradeStore      tradeQueryStore
}

// ReplayCandle carries a stable identity alongside a canonical candle for deterministic replay.
type ReplayCandle struct {
	Identity uint64
	Candle   domain.Candle
}

// ReplayTrade carries a stable identity alongside a canonical trade for deterministic replay.
type ReplayTrade struct {
	Identity uint64
	Trade    domain.Trade
}

// NewIngestionService creates an ingestion service with consumer-defined store dependencies.
func NewIngestionService(deps IngestionServiceDeps) (*IngestionService, error) {
	if deps.InstrumentStore == nil {
		return nil, errors.New("instrument store is required")
	}
	if deps.CandleStore == nil {
		return nil, errors.New("candle store is required")
	}
	if deps.TradeStore == nil {
		return nil, errors.New("trade store is required")
	}

	return &IngestionService{
		instrumentStore: deps.InstrumentStore,
		candleStore:     deps.CandleStore,
		tradeStore:      deps.TradeStore,
	}, nil
}

// NewReadService creates a read service with consumer-defined store dependencies.
func NewReadService(deps ReadServiceDeps) (*ReadService, error) {
	if deps.InstrumentStore == nil {
		return nil, errors.New("instrument store is required")
	}
	if deps.CandleStore == nil {
		return nil, errors.New("candle store is required")
	}
	if deps.TradeStore == nil {
		return nil, errors.New("trade store is required")
	}

	return &ReadService{
		instrumentStore: deps.InstrumentStore,
		candleStore:     deps.CandleStore,
		tradeStore:      deps.TradeStore,
	}, nil
}

// UpsertInstrument validates and persists a canonical instrument.
func (s *IngestionService) UpsertInstrument(
	ctx context.Context,
	instrument domain.Instrument,
) (domain.Instrument, error) {
	canonicalInstrument, err := canonicalizeInstrument(instrument)
	if err != nil {
		return domain.Instrument{}, err
	}

	persistedInstrument, err := s.instrumentStore.UpsertInstrument(ctx, canonicalInstrument)
	if err != nil {
		return domain.Instrument{}, fmt.Errorf("upsert instrument: %w", err)
	}

	return persistedInstrument, nil
}

// IngestCandle validates a canonical candle, upserts its instrument, and persists the candle.
func (s *IngestionService) IngestCandle(ctx context.Context, candle domain.Candle) (domain.Candle, error) {
	canonicalCandle, err := canonicalizeCandle(candle)
	if err != nil {
		return domain.Candle{}, err
	}

	persistedInstrument, err := s.instrumentStore.UpsertInstrument(ctx, canonicalCandle.Instrument)
	if err != nil {
		return domain.Candle{}, fmt.Errorf("upsert candle instrument: %w", err)
	}

	canonicalCandle.Instrument = persistedInstrument

	persistedCandle, err := s.candleStore.UpsertCandle(ctx, canonicalCandle)
	if err != nil {
		return domain.Candle{}, fmt.Errorf("upsert candle: %w", err)
	}

	return persistedCandle, nil
}

// IngestTrade validates a canonical trade, upserts its instrument, and persists the trade.
func (s *IngestionService) IngestTrade(ctx context.Context, trade domain.Trade) (domain.Trade, error) {
	canonicalTrade, err := canonicalizeTrade(trade)
	if err != nil {
		return domain.Trade{}, err
	}

	persistedInstrument, err := s.instrumentStore.UpsertInstrument(ctx, canonicalTrade.Instrument)
	if err != nil {
		return domain.Trade{}, fmt.Errorf("upsert trade instrument: %w", err)
	}

	canonicalTrade.Instrument = persistedInstrument

	persistedTrade, err := s.tradeStore.UpsertTrade(ctx, canonicalTrade)
	if err != nil {
		return domain.Trade{}, fmt.Errorf("upsert trade: %w", err)
	}

	return persistedTrade, nil
}

// LookupInstrument validates venue and symbol inputs before reading a canonical instrument.
func (s *ReadService) LookupInstrument(
	ctx context.Context,
	venue domain.Venue,
	symbol domain.Symbol,
) (*domain.Instrument, error) {
	canonicalVenue, canonicalSymbol, err := canonicalizeInstrumentIdentity(venue, symbol)
	if err != nil {
		return nil, err
	}

	instrument, err := s.instrumentStore.LookupInstrument(ctx, canonicalVenue, canonicalSymbol)
	if err != nil {
		if errors.Is(err, ErrInstrumentNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("lookup instrument: %w", err)
	}

	return instrument, nil
}

// QueryCandles returns canonical candles for an instrument, timeframe, and half-open time range.
func (s *ReadService) QueryCandles(
	ctx context.Context,
	instrument domain.Instrument,
	timeframe domain.Timeframe,
	timeRange domain.TimeRange,
) ([]domain.Candle, error) {
	canonicalInstrument, canonicalTimeframe, canonicalTimeRange, err := canonicalizeCandleReadInputs(
		instrument,
		timeframe,
		timeRange,
	)
	if err != nil {
		return nil, err
	}

	candles, err := s.candleStore.QueryCandles(
		ctx,
		canonicalInstrument,
		canonicalTimeframe,
		canonicalTimeRange,
	)
	if err != nil {
		return nil, fmt.Errorf("query candles: %w", err)
	}

	return candles, nil
}

// QueryTrades returns canonical trades for an instrument and half-open time range.
func (s *ReadService) QueryTrades(
	ctx context.Context,
	instrument domain.Instrument,
	timeRange domain.TimeRange,
) ([]domain.Trade, error) {
	canonicalInstrument, canonicalTimeRange, err := canonicalizeTradeReadInputs(instrument, timeRange)
	if err != nil {
		return nil, err
	}

	trades, err := s.tradeStore.QueryTrades(ctx, canonicalInstrument, canonicalTimeRange)
	if err != nil {
		return nil, fmt.Errorf("query trades: %w", err)
	}

	return trades, nil
}

// ReplayCandles returns deterministic candle replay rows with stable identities.
func (s *ReadService) ReplayCandles(
	ctx context.Context,
	instrument domain.Instrument,
	timeframe domain.Timeframe,
	timeRange domain.TimeRange,
) ([]ReplayCandle, error) {
	canonicalInstrument, canonicalTimeframe, canonicalTimeRange, err := canonicalizeCandleReadInputs(
		instrument,
		timeframe,
		timeRange,
	)
	if err != nil {
		return nil, err
	}

	candles, err := s.candleStore.ReplayCandles(
		ctx,
		canonicalInstrument,
		canonicalTimeframe,
		canonicalTimeRange,
	)
	if err != nil {
		return nil, fmt.Errorf("replay candles: %w", err)
	}

	return candles, nil
}

// ReplayTrades returns deterministic trade replay rows with stable identities.
func (s *ReadService) ReplayTrades(
	ctx context.Context,
	instrument domain.Instrument,
	timeRange domain.TimeRange,
) ([]ReplayTrade, error) {
	canonicalInstrument, canonicalTimeRange, err := canonicalizeTradeReadInputs(instrument, timeRange)
	if err != nil {
		return nil, err
	}

	trades, err := s.tradeStore.ReplayTrades(ctx, canonicalInstrument, canonicalTimeRange)
	if err != nil {
		return nil, fmt.Errorf("replay trades: %w", err)
	}

	return trades, nil
}

func canonicalizeInstrument(instrument domain.Instrument) (domain.Instrument, error) {
	venue, err := domain.NewVenue(instrument.Venue.String())
	if err != nil {
		return domain.Instrument{}, validationError("instrument venue is required")
	}

	symbol, err := domain.NewSymbol(instrument.Symbol.String())
	if err != nil {
		return domain.Instrument{}, validationError("instrument symbol is required")
	}

	assetClass, err := domain.NewAssetClass(instrument.AssetClass.String())
	if err != nil {
		return domain.Instrument{}, validationError("instrument asset class is required")
	}

	canonicalInstrument, err := domain.NewInstrument(domain.InstrumentParams{
		Venue:      venue,
		Symbol:     symbol,
		AssetClass: assetClass,
		Active:     instrument.Active,
	})
	if err != nil {
		return domain.Instrument{}, validationError(err.Error())
	}

	return canonicalInstrument, nil
}

func canonicalizeInstrumentIdentity(
	venue domain.Venue,
	symbol domain.Symbol,
) (domain.Venue, domain.Symbol, error) {
	canonicalVenue, err := domain.NewVenue(venue.String())
	if err != nil {
		return "", "", validationError("instrument venue is required")
	}

	canonicalSymbol, err := domain.NewSymbol(symbol.String())
	if err != nil {
		return "", "", validationError("instrument symbol is required")
	}

	return canonicalVenue, canonicalSymbol, nil
}

func canonicalizeReadInstrument(instrument domain.Instrument) (domain.Instrument, error) {
	canonicalVenue, canonicalSymbol, err := canonicalizeInstrumentIdentity(
		instrument.Venue,
		instrument.Symbol,
	)
	if err != nil {
		return domain.Instrument{}, err
	}

	return domain.Instrument{
		Venue:  canonicalVenue,
		Symbol: canonicalSymbol,
	}, nil
}

func canonicalizeCandleReadInputs(
	instrument domain.Instrument,
	timeframe domain.Timeframe,
	timeRange domain.TimeRange,
) (domain.Instrument, domain.Timeframe, domain.TimeRange, error) {
	canonicalInstrument, err := canonicalizeReadInstrument(instrument)
	if err != nil {
		return domain.Instrument{}, "", domain.TimeRange{}, err
	}

	canonicalTimeframe, err := domain.NewTimeframe(timeframe.String())
	if err != nil {
		return domain.Instrument{}, "", domain.TimeRange{}, validationError("candle timeframe is required")
	}

	canonicalTimeRange, err := domain.NewTimeRange(timeRange.Start, timeRange.End)
	if err != nil {
		return domain.Instrument{}, "", domain.TimeRange{}, validationError(err.Error())
	}

	return canonicalInstrument, canonicalTimeframe, canonicalTimeRange, nil
}

func canonicalizeTradeReadInputs(
	instrument domain.Instrument,
	timeRange domain.TimeRange,
) (domain.Instrument, domain.TimeRange, error) {
	canonicalInstrument, err := canonicalizeReadInstrument(instrument)
	if err != nil {
		return domain.Instrument{}, domain.TimeRange{}, err
	}

	canonicalTimeRange, err := domain.NewTimeRange(timeRange.Start, timeRange.End)
	if err != nil {
		return domain.Instrument{}, domain.TimeRange{}, validationError(err.Error())
	}

	return canonicalInstrument, canonicalTimeRange, nil
}

func canonicalizeCandle(candle domain.Candle) (domain.Candle, error) {
	instrument, err := canonicalizeInstrument(candle.Instrument)
	if err != nil {
		return domain.Candle{}, err
	}

	timeframe, err := domain.NewTimeframe(candle.Timeframe.String())
	if err != nil {
		return domain.Candle{}, validationError("candle timeframe is required")
	}

	timeRange, err := domain.NewTimeRange(candle.TimeRange.Start, candle.TimeRange.End)
	if err != nil {
		return domain.Candle{}, validationError(err.Error())
	}

	quality, err := domain.NewDataQuality(candle.Quality.String())
	if err != nil {
		return domain.Candle{}, validationError("candle quality is required")
	}

	provenance, err := canonicalizeProvenance(candle.Provenance)
	if err != nil {
		return domain.Candle{}, err
	}

	if candle.Open < 0 {
		return domain.Candle{}, validationError("candle open must be non-negative")
	}
	if candle.High < 0 {
		return domain.Candle{}, validationError("candle high must be non-negative")
	}
	if candle.Low < 0 {
		return domain.Candle{}, validationError("candle low must be non-negative")
	}
	if candle.Close < 0 {
		return domain.Candle{}, validationError("candle close must be non-negative")
	}
	if candle.Volume < 0 {
		return domain.Candle{}, validationError("candle volume must be non-negative")
	}

	canonicalCandle, err := domain.NewCandle(domain.CandleParams{
		Instrument: instrument,
		Timeframe:  timeframe,
		TimeRange:  timeRange,
		Open:       candle.Open,
		High:       candle.High,
		Low:        candle.Low,
		Close:      candle.Close,
		Volume:     candle.Volume,
		Quality:    quality,
		Provenance: provenance,
	})
	if err != nil {
		return domain.Candle{}, validationError(err.Error())
	}

	return canonicalCandle, nil
}

func canonicalizeTrade(trade domain.Trade) (domain.Trade, error) {
	instrument, err := canonicalizeInstrument(trade.Instrument)
	if err != nil {
		return domain.Trade{}, err
	}

	quality, err := domain.NewDataQuality(trade.Quality.String())
	if err != nil {
		return domain.Trade{}, validationError("trade quality is required")
	}

	provenance, err := canonicalizeProvenance(trade.Provenance)
	if err != nil {
		return domain.Trade{}, err
	}

	if trade.EventTime.IsZero() {
		return domain.Trade{}, validationError("trade event time is required")
	}
	if trade.Price < 0 {
		return domain.Trade{}, validationError("trade price must be non-negative")
	}
	if trade.Size < 0 {
		return domain.Trade{}, validationError("trade size must be non-negative")
	}

	canonicalTrade, err := domain.NewTrade(domain.TradeParams{
		Instrument: instrument,
		EventTime:  trade.EventTime,
		Price:      trade.Price,
		Size:       trade.Size,
		Quality:    quality,
		Provenance: provenance,
	})
	if err != nil {
		return domain.Trade{}, validationError(err.Error())
	}

	return canonicalTrade, nil
}

func canonicalizeProvenance(provenance domain.SourceProvenance) (domain.SourceProvenance, error) {
	canonicalProvenance, err := domain.NewSourceProvenance(
		provenance.Source,
		provenance.RecordID,
	)
	if err != nil {
		return domain.SourceProvenance{}, validationError("record provenance source is required")
	}

	return canonicalProvenance, nil
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrValidation, message)
}
