package venueedge

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
)

const (
	sandboxMinPositiveValue     = 0.01
	sandboxCandleHighOffset     = 0.5
	sandboxCandleOpenFactor     = 0.25
	sandboxCandleCloseFactor    = 0.75
	sandboxTradeSlotsPerMinute  = 3
	sandboxTradeSlotSpacingSecs = 20
	sandboxTradeFirstOffsetSecs = 5
	sandboxFloatScaleModulo     = 1000000
	sandboxFloatScaleDivisor    = 10000
	sandboxCandleVolumeFactor   = 100
	sandboxTradePriceFactor     = 100
	sandboxTradeSizeFactor      = 5
	sandboxFiveMinuteDuration   = 5 * time.Minute
	sandboxFifteenMinuteDuraton = 15 * time.Minute
	sandboxFourHourDuration     = 4 * time.Hour
	sandboxOneDayDuration       = 24 * time.Hour
)

type sandboxInstrument struct {
	instrument domain.Instrument
}

// SandboxVenueParams configures a deterministic synthetic venue for tests.
type SandboxVenueParams struct {
	Seed                int64
	Venue               domain.Venue
	Instruments         []SandboxInstrumentParams
	SupportedTimeframes []domain.Timeframe
	DefaultPageSize     int
}

// SandboxInstrumentParams describes one stable sandbox instrument.
type SandboxInstrumentParams struct {
	Symbol     domain.Symbol
	AssetClass domain.AssetClass
	Active     bool
}

// SandboxVenue emits deterministic canonical market data for one synthetic venue.
type SandboxVenue struct {
	seed                int64
	venue               domain.Venue
	instruments         []sandboxInstrument
	instrumentBySymbol  map[domain.Symbol]domain.Instrument
	supportedTimeframes map[domain.Timeframe]time.Duration
	defaultPageSize     int
}

// NewSandboxVenue validates and canonicalizes a deterministic sandbox venue.
func NewSandboxVenue(params SandboxVenueParams) (*SandboxVenue, error) {
	venue, err := canonicalizeVenue(params.Venue)
	if err != nil {
		return nil, validationError("sandbox venue is required")
	}

	if len(params.Instruments) == 0 {
		return nil, validationError("sandbox instruments are required")
	}

	instruments := make([]sandboxInstrument, 0, len(params.Instruments))
	instrumentBySymbol := make(map[domain.Symbol]domain.Instrument, len(params.Instruments))
	for _, rawInstrument := range params.Instruments {
		symbol, symbolErr := domain.NewSymbol(rawInstrument.Symbol.String())
		if symbolErr != nil {
			return nil, validationError("sandbox instruments must use canonical symbols")
		}

		assetClass, assetClassErr := domain.NewAssetClass(rawInstrument.AssetClass.String())
		if assetClassErr != nil {
			return nil, validationError("sandbox instruments must use canonical asset classes")
		}

		instrument, instrumentErr := domain.NewInstrument(domain.InstrumentParams{
			Venue:      venue,
			Symbol:     symbol,
			AssetClass: assetClass,
			Active:     rawInstrument.Active,
		})
		if instrumentErr != nil {
			return nil, validationError("sandbox instruments must be valid")
		}

		if _, exists := instrumentBySymbol[instrument.Symbol]; exists {
			return nil, validationError("sandbox symbols must be unique")
		}

		instruments = append(instruments, sandboxInstrument{instrument: instrument})
		instrumentBySymbol[instrument.Symbol] = instrument
	}

	if len(params.SupportedTimeframes) == 0 {
		return nil, validationError("sandbox supported timeframes are required")
	}

	supportedTimeframes := make(map[domain.Timeframe]time.Duration, len(params.SupportedTimeframes))
	for _, rawTimeframe := range params.SupportedTimeframes {
		timeframe, timeframeErr := domain.NewTimeframe(rawTimeframe.String())
		if timeframeErr != nil {
			return nil, validationError("sandbox timeframes must be canonical")
		}

		duration, durationErr := timeframeDuration(timeframe)
		if durationErr != nil {
			return nil, durationErr
		}

		supportedTimeframes[timeframe] = duration
	}

	defaultPageSize := params.DefaultPageSize
	if defaultPageSize < 0 {
		return nil, validationError("sandbox default page size must be zero or positive")
	}

	return &SandboxVenue{
		seed:                params.Seed,
		venue:               venue,
		instruments:         instruments,
		instrumentBySymbol:  instrumentBySymbol,
		supportedTimeframes: supportedTimeframes,
		defaultPageSize:     defaultPageSize,
	}, nil
}

// ReadInstruments returns deterministic canonical sandbox instruments.
func (s *SandboxVenue) ReadInstruments(
	_ context.Context,
	request InstrumentReadRequest,
) (InstrumentReadResult, error) {
	canonicalRequest, err := NewInstrumentReadRequest(InstrumentReadRequestParams(request))
	if err != nil {
		return InstrumentReadResult{}, err
	}

	if canonicalRequest.Venue != s.venue {
		return InstrumentReadResult{}, validationError("sandbox venue does not serve the requested venue")
	}

	records := make([]domain.Instrument, 0, len(s.instruments))
	for _, instrument := range s.instruments {
		if len(canonicalRequest.Symbols) > 0 &&
			!containsSymbol(canonicalRequest.Symbols, instrument.instrument.Symbol) {
			continue
		}

		records = append(records, instrument.instrument)
	}

	page, nextPageToken, err := paginate(
		records,
		canonicalRequest.PageSize,
		s.defaultPageSize,
		canonicalRequest.PageToken,
	)
	if err != nil {
		return InstrumentReadResult{}, err
	}

	return NewInstrumentReadResult(page, nextPageToken)
}

// ReadCandles returns deterministic canonical sandbox candles for a half-open range.
func (s *SandboxVenue) ReadCandles(
	_ context.Context,
	request CandleReadRequest,
) (CandleReadResult, error) {
	canonicalRequest, err := NewCandleReadRequest(CandleReadRequestParams(request))
	if err != nil {
		return CandleReadResult{}, err
	}

	instrument, duration, err := s.validateInstrumentAndTimeframe(
		canonicalRequest.Instrument,
		canonicalRequest.Timeframe,
	)
	if err != nil {
		return CandleReadResult{}, err
	}

	records := make([]domain.Candle, 0)
	for bucketStart := alignCandleStart(canonicalRequest.TimeRange.Start, duration); bucketStart.Before(canonicalRequest.TimeRange.End); bucketStart = bucketStart.Add(duration) {
		candleEnd := bucketStart.Add(duration)
		bucketInstant := strconv.FormatInt(bucketStart.UnixNano(), 10)
		candleProvenance, provenanceErr := domain.NewSourceProvenance(
			string(s.venue)+"-sandbox",
			fmt.Sprintf(
				"candle:%d:%s:%s:%s",
				s.seed,
				instrument.Symbol,
				canonicalRequest.Timeframe,
				bucketInstant,
			),
		)
		if provenanceErr != nil {
			return CandleReadResult{}, fmt.Errorf("build candle provenance: %w", provenanceErr)
		}

		base := s.float64For(
			"candle-base",
			instrument.Symbol.String(),
			canonicalRequest.Timeframe.String(),
			bucketInstant,
		)
		span := s.float64For(
			"candle-span",
			instrument.Symbol.String(),
			canonicalRequest.Timeframe.String(),
			bucketInstant,
		)
		low := math.Max(sandboxMinPositiveValue, base)
		high := low + span + sandboxCandleHighOffset
		openPrice := low + span*sandboxCandleOpenFactor
		closePrice := low + span*sandboxCandleCloseFactor
		volume := s.float64For(
			"candle-volume",
			instrument.Symbol.String(),
			canonicalRequest.Timeframe.String(),
			bucketInstant,
		)*sandboxCandleVolumeFactor + 1

		candle, candleErr := domain.NewCandle(domain.CandleParams{
			Instrument: instrument,
			Timeframe:  canonicalRequest.Timeframe,
			TimeRange: domain.TimeRange{
				Start: bucketStart,
				End:   candleEnd,
			},
			Open:       openPrice,
			High:       high,
			Low:        low,
			Close:      closePrice,
			Volume:     volume,
			Quality:    domain.DataQualityValidated,
			Provenance: candleProvenance,
		})
		if candleErr != nil {
			return CandleReadResult{}, fmt.Errorf("build sandbox candle: %w", candleErr)
		}

		records = append(records, candle)
	}

	page, nextPageToken, err := paginate(
		records,
		canonicalRequest.PageSize,
		s.defaultPageSize,
		canonicalRequest.PageToken,
	)
	if err != nil {
		return CandleReadResult{}, err
	}

	return NewCandleReadResult(page, nextPageToken)
}

// ReadTrades returns deterministic canonical sandbox trades for a half-open range.
func (s *SandboxVenue) ReadTrades(
	_ context.Context,
	request TradeReadRequest,
) (TradeReadResult, error) {
	canonicalRequest, err := NewTradeReadRequest(TradeReadRequestParams(request))
	if err != nil {
		return TradeReadResult{}, err
	}

	instrument, _, err := s.validateInstrumentAndTimeframe(canonicalRequest.Instrument, domain.Timeframe1m)
	if err != nil {
		return TradeReadResult{}, err
	}

	records := make([]domain.Trade, 0)
	for minuteStart := canonicalRequest.TimeRange.Start.Truncate(time.Minute); minuteStart.Before(canonicalRequest.TimeRange.End); minuteStart = minuteStart.Add(time.Minute) {
		minuteInstant := strconv.FormatInt(minuteStart.UnixNano(), 10)
		for slot := range sandboxTradeSlotsPerMinute {
			eventTime := minuteStart.Add(
				time.Duration(slot*sandboxTradeSlotSpacingSecs+sandboxTradeFirstOffsetSecs) * time.Second,
			)
			if eventTime.Before(canonicalRequest.TimeRange.Start) || !eventTime.Before(canonicalRequest.TimeRange.End) {
				continue
			}

			tradeProvenance, provenanceErr := domain.NewSourceProvenance(
				string(s.venue)+"-sandbox",
				fmt.Sprintf(
					"trade:%d:%s:%s:%d",
					s.seed,
					instrument.Symbol,
					minuteInstant,
					slot,
				),
			)
			if provenanceErr != nil {
				return TradeReadResult{}, fmt.Errorf("build trade provenance: %w", provenanceErr)
			}

			price := math.Max(
				sandboxMinPositiveValue,
				s.float64For(
					"trade-price",
					instrument.Symbol.String(),
					minuteInstant,
					strconv.Itoa(slot),
				)*sandboxTradePriceFactor,
			)
			size := math.Max(
				0,
				s.float64For(
					"trade-size",
					instrument.Symbol.String(),
					minuteInstant,
					strconv.Itoa(slot),
				)*sandboxTradeSizeFactor,
			)

			trade, tradeErr := domain.NewTrade(domain.TradeParams{
				Instrument: instrument,
				EventTime:  eventTime,
				Price:      price,
				Size:       size,
				Quality:    domain.DataQualityValidated,
				Provenance: tradeProvenance,
			})
			if tradeErr != nil {
				return TradeReadResult{}, fmt.Errorf("build sandbox trade: %w", tradeErr)
			}

			records = append(records, trade)
		}
	}

	page, nextPageToken, err := paginate(
		records,
		canonicalRequest.PageSize,
		s.defaultPageSize,
		canonicalRequest.PageToken,
	)
	if err != nil {
		return TradeReadResult{}, err
	}

	return NewTradeReadResult(page, nextPageToken)
}

func (s *SandboxVenue) validateInstrumentAndTimeframe(
	requestedInstrument domain.Instrument,
	requestedTimeframe domain.Timeframe,
) (domain.Instrument, time.Duration, error) {
	instrument, err := canonicalizeInstrument(requestedInstrument)
	if err != nil {
		return domain.Instrument{}, 0, validationError("sandbox instrument is invalid")
	}

	if instrument.Venue != s.venue {
		return domain.Instrument{}, 0, validationError("sandbox venue does not serve the requested instrument venue")
	}

	configuredInstrument, ok := s.instrumentBySymbol[instrument.Symbol]
	if !ok {
		return domain.Instrument{}, 0, validationError("sandbox venue does not know the requested instrument symbol")
	}

	if requestedTimeframe == "" {
		return configuredInstrument, 0, nil
	}

	timeframe, err := domain.NewTimeframe(requestedTimeframe.String())
	if err != nil {
		return domain.Instrument{}, 0, validationError("sandbox timeframe is invalid")
	}

	duration, ok := s.supportedTimeframes[timeframe]
	if !ok {
		return domain.Instrument{}, 0, validationError("sandbox venue does not support the requested timeframe")
	}

	return configuredInstrument, duration, nil
}

func (s *SandboxVenue) float64For(parts ...string) float64 {
	value := s.uint64For(parts...)
	return float64(value%sandboxFloatScaleModulo) / sandboxFloatScaleDivisor
}

func (s *SandboxVenue) uint64For(parts ...string) uint64 {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(strconv.FormatInt(s.seed, 10)))
	for _, part := range parts {
		_, _ = hasher.Write([]byte{0})
		_, _ = hasher.Write([]byte(part))
	}

	return hasher.Sum64()
}

func paginate[T any](
	records []T,
	requestPageSize int,
	defaultPageSize int,
	pageToken string,
) ([]T, string, error) {
	offset, err := parsePageToken(pageToken)
	if err != nil {
		return nil, "", err
	}

	if offset > len(records) {
		return nil, "", validationError("page token is outside the available range")
	}

	pageSize := requestPageSize
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	if pageSize == 0 {
		return append([]T(nil), records[offset:]...), "", nil
	}

	end := offset + pageSize
	if end >= len(records) {
		return append([]T(nil), records[offset:]...), "", nil
	}

	return append([]T(nil), records[offset:end]...), strconv.Itoa(end), nil
}

func parsePageToken(pageToken string) (int, error) {
	token := strings.TrimSpace(pageToken)
	if token == "" {
		return 0, nil
	}

	offset, err := strconv.Atoi(token)
	if err != nil || offset < 0 {
		return 0, validationError("page token must be a non-negative integer")
	}

	return offset, nil
}

func containsSymbol(symbols []domain.Symbol, target domain.Symbol) bool {
	return slices.Contains(symbols, target)
}

func timeframeDuration(timeframe domain.Timeframe) (time.Duration, error) {
	switch timeframe {
	case domain.Timeframe1m:
		return time.Minute, nil
	case domain.Timeframe5m:
		return sandboxFiveMinuteDuration, nil
	case domain.Timeframe15m:
		return sandboxFifteenMinuteDuraton, nil
	case domain.Timeframe1h:
		return time.Hour, nil
	case domain.Timeframe4h:
		return sandboxFourHourDuration, nil
	case domain.Timeframe1d:
		return sandboxOneDayDuration, nil
	default:
		return 0, validationError("sandbox timeframe is invalid")
	}
}

func alignCandleStart(start time.Time, duration time.Duration) time.Time {
	aligned := start.Truncate(duration)
	if aligned.Before(start) {
		return aligned.Add(duration)
	}

	return aligned
}
