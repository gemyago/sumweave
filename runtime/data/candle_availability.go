package data

import (
	"encoding/base64"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
)

const (
	defaultCandleAvailabilityLimit    = 50
	maxCandleAvailabilityLimit        = 200
	maxDefaultCandleSliceIntervals    = 500
	candleAvailabilityCursorPartCount = 4
	timeframeDuration5m               = 5 * time.Minute
	timeframeDuration15m              = 15 * time.Minute
	timeframeDuration4h               = 4 * time.Hour
	timeframeDuration1d               = 24 * time.Hour
)

// CandleAvailabilityListQueryParams configures normalized candle availability reads.
type CandleAvailabilityListQueryParams struct {
	Venue      domain.Venue
	Symbol     domain.Symbol
	AssetClass domain.AssetClass
	Limit      int
	Cursor     string
}

// CandleAvailabilityListQuery carries canonical availability list filters.
type CandleAvailabilityListQuery struct {
	Venue      domain.Venue
	Symbol     domain.Symbol
	AssetClass domain.AssetClass
	Limit      int
	Cursor     string
	cursor     candleAvailabilityListCursor
}

// CandleAvailabilityTimeframeSummary carries one persisted timeframe summary.
type CandleAvailabilityTimeframeSummary struct {
	Timeframe domain.Timeframe
	StartAt   time.Time
	EndAt     time.Time
	Count     int64
}

// CandleAvailabilityDefaultSlice carries one deterministic exact candle read scope.
type CandleAvailabilityDefaultSlice struct {
	Timeframe domain.Timeframe
	StartAt   time.Time
	EndAt     time.Time
}

// CandleAvailabilityItem carries one venue, symbol, and asset class availability entry.
type CandleAvailabilityItem struct {
	Venue        domain.Venue
	Symbol       domain.Symbol
	AssetClass   domain.AssetClass
	Timeframes   []CandleAvailabilityTimeframeSummary
	DefaultSlice CandleAvailabilityDefaultSlice
}

// CandleAvailabilityDefaultSelection carries the first-page default browse scope.
type CandleAvailabilityDefaultSelection struct {
	Venue      domain.Venue
	Symbol     domain.Symbol
	AssetClass domain.AssetClass
	Timeframe  domain.Timeframe
	StartAt    time.Time
	EndAt      time.Time
}

// CandleAvailabilityListResult carries one deterministic availability page.
type CandleAvailabilityListResult struct {
	Items            []CandleAvailabilityItem
	NextCursor       string
	DefaultSelection *CandleAvailabilityDefaultSelection
}

type candleAvailabilityListCursor struct {
	LatestEnd  time.Time
	Venue      domain.Venue
	Symbol     domain.Symbol
	AssetClass domain.AssetClass
}

// NewCandleAvailabilityListQuery validates and canonicalizes availability filters.
func NewCandleAvailabilityListQuery(
	params CandleAvailabilityListQueryParams,
) (CandleAvailabilityListQuery, error) {
	return canonicalizeCandleAvailabilityListQuery(CandleAvailabilityListQuery{
		Venue:      params.Venue,
		Symbol:     params.Symbol,
		AssetClass: params.AssetClass,
		Limit:      params.Limit,
		Cursor:     params.Cursor,
	})
}

func canonicalizeCandleAvailabilityListQuery(
	query CandleAvailabilityListQuery,
) (CandleAvailabilityListQuery, error) {
	venue, err := canonicalizeOptionalVenueFilter(query.Venue, "candle availability query")
	if err != nil {
		return CandleAvailabilityListQuery{}, err
	}

	symbol, err := canonicalizeOptionalSymbolFilter(query.Symbol, "candle availability query")
	if err != nil {
		return CandleAvailabilityListQuery{}, err
	}

	assetClass, err := canonicalizeOptionalAssetClassFilter(query.AssetClass, "candle availability query")
	if err != nil {
		return CandleAvailabilityListQuery{}, err
	}

	limit, err := canonicalizeCandleAvailabilityLimit(query.Limit)
	if err != nil {
		return CandleAvailabilityListQuery{}, err
	}

	cursor, canonicalCursor, err := decodeCandleAvailabilityListCursor(query.Cursor)
	if err != nil {
		return CandleAvailabilityListQuery{}, err
	}

	return CandleAvailabilityListQuery{
		Venue:      venue,
		Symbol:     symbol,
		AssetClass: assetClass,
		Limit:      limit,
		Cursor:     canonicalCursor,
		cursor:     cursor,
	}, nil
}

func encodeCandleAvailabilityListCursor(
	latestEnd time.Time,
	venue domain.Venue,
	symbol domain.Symbol,
	assetClass domain.AssetClass,
) string {
	encoded := strings.Join([]string{
		latestEnd.UTC().Format(time.RFC3339Nano),
		venue.String(),
		symbol.String(),
		assetClass.String(),
	}, "\n")

	return base64.RawURLEncoding.EncodeToString([]byte(encoded))
}

func decodeCandleAvailabilityListCursor(
	cursor string,
) (candleAvailabilityListCursor, string, error) {
	canonicalCursor := strings.TrimSpace(cursor)
	if canonicalCursor == "" {
		return candleAvailabilityListCursor{}, "", nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(canonicalCursor)
	if err != nil {
		return candleAvailabilityListCursor{}, "", validationError(
			"candle availability query cursor is invalid",
		)
	}

	parts := strings.Split(string(decoded), "\n")
	if len(parts) != candleAvailabilityCursorPartCount {
		return candleAvailabilityListCursor{}, "", validationError(
			"candle availability query cursor is invalid",
		)
	}

	latestEnd, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return candleAvailabilityListCursor{}, "", validationError(
			"candle availability query cursor is invalid",
		)
	}

	venue, err := domain.NewVenue(parts[1])
	if err != nil {
		return candleAvailabilityListCursor{}, "", validationError(
			"candle availability query cursor is invalid",
		)
	}

	symbol, err := domain.NewSymbol(parts[2])
	if err != nil {
		return candleAvailabilityListCursor{}, "", validationError(
			"candle availability query cursor is invalid",
		)
	}

	assetClass, err := domain.NewAssetClass(parts[3])
	if err != nil {
		return candleAvailabilityListCursor{}, "", validationError(
			"candle availability query cursor is invalid",
		)
	}

	return candleAvailabilityListCursor{
		LatestEnd:  latestEnd.UTC(),
		Venue:      venue,
		Symbol:     symbol,
		AssetClass: assetClass,
	}, canonicalCursor, nil
}

func canonicalizeCandleAvailabilityLimit(limit int) (int, error) {
	if limit == 0 {
		return defaultCandleAvailabilityLimit, nil
	}
	if limit < 0 || limit > maxCandleAvailabilityLimit {
		return 0, validationError("candle availability query limit must be between 1 and 200")
	}

	return limit, nil
}

func canonicalizeOptionalVenueFilter(
	venue domain.Venue,
	subject string,
) (domain.Venue, error) {
	if strings.TrimSpace(venue.String()) == "" {
		return "", nil
	}

	canonicalVenue, err := domain.NewVenue(venue.String())
	if err != nil {
		return "", validationError(subject + " venue is invalid")
	}

	return canonicalVenue, nil
}

func canonicalizeOptionalSymbolFilter(
	symbol domain.Symbol,
	subject string,
) (domain.Symbol, error) {
	if strings.TrimSpace(symbol.String()) == "" {
		return "", nil
	}

	canonicalSymbol, err := domain.NewSymbol(symbol.String())
	if err != nil {
		return "", validationError(subject + " symbol is invalid")
	}

	return canonicalSymbol, nil
}

func canonicalizeOptionalAssetClassFilter(
	assetClass domain.AssetClass,
	subject string,
) (domain.AssetClass, error) {
	if strings.TrimSpace(assetClass.String()) == "" {
		return "", nil
	}

	canonicalAssetClass, err := domain.NewAssetClass(assetClass.String())
	if err != nil {
		return "", validationError(subject + " asset class is invalid")
	}

	return canonicalAssetClass, nil
}

func candleAvailabilityTimeframeDuration(timeframe domain.Timeframe) (time.Duration, error) {
	switch timeframe {
	case domain.Timeframe1m:
		return time.Minute, nil
	case domain.Timeframe5m:
		return timeframeDuration5m, nil
	case domain.Timeframe15m:
		return timeframeDuration15m, nil
	case domain.Timeframe1h:
		return time.Hour, nil
	case domain.Timeframe4h:
		return timeframeDuration4h, nil
	case domain.Timeframe1d:
		return timeframeDuration1d, nil
	default:
		return 0, validationError("candle availability timeframe is invalid")
	}
}
