package data

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
)

const (
	defaultRawPayloadMetadataLimit = 50
	maxRawPayloadMetadataLimit     = 200
	rawPayloadPreviewByteLimit     = 4096
	rawPayloadCursorPartCount      = 2
)

// ErrRawPayloadNotFound marks a missing raw payload lookup result.
var ErrRawPayloadNotFound = errors.New("raw payload not found")

// RawPayloadMetadata carries raw payload metadata without response body bytes.
type RawPayloadMetadata struct {
	ID                 string
	IngestionRunID     string
	Source             string
	Venue              domain.Venue
	Endpoint           string
	RequestType        string
	RequestPayloadHash string
	RequestMetadata    map[string]string
	RequestAt          time.Time
	ResponseAt         time.Time
	HTTPStatus         int
	ResponseBodyHash   string
	PayloadBodyRef     string
	EntityHint         string
	Instrument         *BatchInstrumentRef
	Timeframe          domain.Timeframe
	TimeRange          *domain.TimeRange
	ReceivedAt         time.Time
}

// RawPayloadMetadataListQueryParams configures raw payload metadata list filtering.
type RawPayloadMetadataListQueryParams struct {
	Venue          domain.Venue
	Symbol         domain.Symbol
	AssetClass     domain.AssetClass
	Timeframe      domain.Timeframe
	StartAt        time.Time
	EndAt          time.Time
	IngestionRunID string
	EntityHint     string
	Endpoint       string
	RequestType    string
	Limit          int
	Cursor         string
}

// RawPayloadMetadataListQuery carries canonical metadata list filters.
type RawPayloadMetadataListQuery struct {
	Venue          domain.Venue
	Instrument     *BatchInstrumentRef
	Timeframe      domain.Timeframe
	TimeRange      *domain.TimeRange
	IngestionRunID string
	EntityHint     string
	Endpoint       string
	RequestType    string
	Limit          int
	Cursor         string
	cursor         rawPayloadListCursor
}

// RawPayloadMetadataListResult carries one page of metadata rows.
type RawPayloadMetadataListResult struct {
	Items      []RawPayloadMetadata
	NextCursor string
}

// RawPayloadDetail carries one raw payload metadata record plus body preview data.
type RawPayloadDetail struct {
	Metadata                     RawPayloadMetadata
	ResponseBodySizeBytes        int
	ResponseBodyPreview          []byte
	ResponseBodyPreviewTruncated bool
}

// CandleLinkedRawPayloadsQueryParams configures exact candle-linked raw payload lookup.
type CandleLinkedRawPayloadsQueryParams struct {
	Venue              domain.Venue
	Symbol             domain.Symbol
	AssetClass         domain.AssetClass
	Timeframe          domain.Timeframe
	StartAt            time.Time
	EndAt              time.Time
	ProvenanceSource   string
	ProvenanceIdentity string
}

// CandleLinkedRawPayloadsQuery carries the canonical candle natural key.
type CandleLinkedRawPayloadsQuery struct {
	Venue              domain.Venue
	Symbol             domain.Symbol
	AssetClass         domain.AssetClass
	Timeframe          domain.Timeframe
	TimeRange          domain.TimeRange
	ProvenanceSource   string
	ProvenanceIdentity string
}

type rawPayloadListCursor struct {
	ReceivedAt time.Time
	ID         string
}

// NewRawPayloadMetadataListQuery validates and canonicalizes list filters.
func NewRawPayloadMetadataListQuery(params RawPayloadMetadataListQueryParams) (RawPayloadMetadataListQuery, error) {
	return canonicalizeRawPayloadMetadataListQuery(RawPayloadMetadataListQuery{
		Venue: params.Venue,
		Instrument: &BatchInstrumentRef{
			Symbol:     params.Symbol,
			AssetClass: params.AssetClass,
		},
		Timeframe:      params.Timeframe,
		TimeRange:      rawPayloadTimeRangePointer(params.StartAt, params.EndAt),
		IngestionRunID: params.IngestionRunID,
		EntityHint:     params.EntityHint,
		Endpoint:       params.Endpoint,
		RequestType:    params.RequestType,
		Limit:          params.Limit,
		Cursor:         params.Cursor,
	})
}

// NewCandleLinkedRawPayloadsQuery validates and canonicalizes candle-linked lookup inputs.
func NewCandleLinkedRawPayloadsQuery(params CandleLinkedRawPayloadsQueryParams) (CandleLinkedRawPayloadsQuery, error) {
	timeRange, err := domain.NewTimeRange(params.StartAt, params.EndAt)
	if err != nil {
		return CandleLinkedRawPayloadsQuery{}, validationError("candle raw payload query " + err.Error())
	}

	return canonicalizeCandleLinkedRawPayloadsQuery(CandleLinkedRawPayloadsQuery{
		Venue:              params.Venue,
		Symbol:             params.Symbol,
		AssetClass:         params.AssetClass,
		Timeframe:          params.Timeframe,
		TimeRange:          timeRange,
		ProvenanceSource:   params.ProvenanceSource,
		ProvenanceIdentity: params.ProvenanceIdentity,
	})
}

func canonicalizeRawPayloadMetadataListQuery(query RawPayloadMetadataListQuery) (RawPayloadMetadataListQuery, error) {
	venue, err := domain.NewVenue(query.Venue.String())
	if err != nil {
		return RawPayloadMetadataListQuery{}, validationError("raw payload query venue is required")
	}

	instrument, hasInstrument, err := canonicalizeOptionalBatchInstrumentRef(query.Instrument, "raw payload query")
	if err != nil {
		return RawPayloadMetadataListQuery{}, err
	}

	var timeframe domain.Timeframe
	if strings.TrimSpace(query.Timeframe.String()) != "" {
		canonicalTimeframe, timeframeErr := domain.NewTimeframe(query.Timeframe.String())
		if timeframeErr != nil {
			return RawPayloadMetadataListQuery{}, validationError("raw payload query timeframe is invalid")
		}
		timeframe = canonicalTimeframe
	}

	timeRange, hasTimeRange, err := canonicalizeOptionalTimeRange(query.TimeRange, "raw payload query")
	if err != nil {
		return RawPayloadMetadataListQuery{}, err
	}

	var canonicalInstrument *BatchInstrumentRef
	if hasInstrument {
		canonicalInstrument = &instrument
	}

	var canonicalTimeRange *domain.TimeRange
	if hasTimeRange {
		canonicalTimeRange = &timeRange
	}

	cursor, canonicalCursor, err := decodeRawPayloadListCursor(query.Cursor)
	if err != nil {
		return RawPayloadMetadataListQuery{}, err
	}

	return RawPayloadMetadataListQuery{
		Venue:          venue,
		Instrument:     canonicalInstrument,
		Timeframe:      timeframe,
		TimeRange:      canonicalTimeRange,
		IngestionRunID: strings.TrimSpace(query.IngestionRunID),
		EntityHint:     strings.TrimSpace(query.EntityHint),
		Endpoint:       strings.TrimSpace(query.Endpoint),
		RequestType:    strings.TrimSpace(query.RequestType),
		Limit:          canonicalizeRawPayloadMetadataLimit(query.Limit),
		Cursor:         canonicalCursor,
		cursor:         cursor,
	}, nil
}

func canonicalizeCandleLinkedRawPayloadsQuery(
	query CandleLinkedRawPayloadsQuery,
) (CandleLinkedRawPayloadsQuery, error) {
	venue, symbol, err := canonicalizeInstrumentIdentity(query.Venue, query.Symbol)
	if err != nil {
		if strings.Contains(err.Error(), "venue") {
			return CandleLinkedRawPayloadsQuery{}, validationError("candle raw payload query venue is required")
		}
		return CandleLinkedRawPayloadsQuery{}, validationError("candle raw payload query symbol is required")
	}

	assetClass, err := domain.NewAssetClass(query.AssetClass.String())
	if err != nil {
		return CandleLinkedRawPayloadsQuery{}, validationError("candle raw payload query asset class is required")
	}

	timeframe, err := domain.NewTimeframe(query.Timeframe.String())
	if err != nil {
		return CandleLinkedRawPayloadsQuery{}, validationError("candle raw payload query timeframe is required")
	}

	timeRange, err := domain.NewTimeRange(query.TimeRange.Start, query.TimeRange.End)
	if err != nil {
		return CandleLinkedRawPayloadsQuery{}, validationError("candle raw payload query " + err.Error())
	}

	provenanceSource := strings.TrimSpace(query.ProvenanceSource)
	if provenanceSource == "" {
		return CandleLinkedRawPayloadsQuery{}, validationError(
			"candle raw payload query provenance source is required",
		)
	}

	provenanceIdentity := strings.TrimSpace(query.ProvenanceIdentity)
	if provenanceIdentity == "" {
		return CandleLinkedRawPayloadsQuery{}, validationError(
			"candle raw payload query provenance identity is required",
		)
	}

	return CandleLinkedRawPayloadsQuery{
		Venue:              venue,
		Symbol:             symbol,
		AssetClass:         assetClass,
		Timeframe:          timeframe,
		TimeRange:          timeRange,
		ProvenanceSource:   provenanceSource,
		ProvenanceIdentity: provenanceIdentity,
	}, nil
}

func rawPayloadMetadataFromDomain(payload RawVenuePayload) RawPayloadMetadata {
	return RawPayloadMetadata{
		ID:                 payload.ID,
		IngestionRunID:     payload.IngestionRunID,
		Source:             payload.Source,
		Venue:              payload.Venue,
		Endpoint:           payload.Endpoint,
		RequestType:        payload.RequestType,
		RequestPayloadHash: payload.RequestPayloadHash,
		RequestMetadata:    payload.RequestMetadata,
		RequestAt:          payload.RequestAt,
		ResponseAt:         payload.ResponseAt,
		HTTPStatus:         payload.HTTPStatus,
		ResponseBodyHash:   payload.ResponseBodyHash,
		PayloadBodyRef:     payload.PayloadBodyRef,
		EntityHint:         payload.EntityHint,
		Instrument:         payload.Instrument,
		Timeframe:          payload.Timeframe,
		TimeRange:          payload.TimeRange,
		ReceivedAt:         payload.ReceivedAt,
	}
}

func encodeRawPayloadListCursor(receivedAt time.Time, id string) string {
	canonicalReceivedAt := receivedAt.UTC().Format(time.RFC3339Nano)
	return base64.RawURLEncoding.EncodeToString([]byte(canonicalReceivedAt + "\n" + id))
}

func decodeRawPayloadListCursor(cursor string) (rawPayloadListCursor, string, error) {
	canonicalCursor := strings.TrimSpace(cursor)
	if canonicalCursor == "" {
		return rawPayloadListCursor{}, "", nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(canonicalCursor)
	if err != nil {
		return rawPayloadListCursor{}, "", validationError("raw payload query cursor is invalid")
	}

	parts := strings.Split(string(decoded), "\n")
	if len(parts) != rawPayloadCursorPartCount {
		return rawPayloadListCursor{}, "", validationError("raw payload query cursor is invalid")
	}

	receivedAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return rawPayloadListCursor{}, "", validationError("raw payload query cursor is invalid")
	}

	id := strings.TrimSpace(parts[1])
	if id == "" {
		return rawPayloadListCursor{}, "", validationError("raw payload query cursor is invalid")
	}

	return rawPayloadListCursor{ReceivedAt: receivedAt.UTC(), ID: id}, canonicalCursor, nil
}

func canonicalizeRawPayloadMetadataLimit(limit int) int {
	if limit <= 0 {
		return defaultRawPayloadMetadataLimit
	}
	if limit > maxRawPayloadMetadataLimit {
		return maxRawPayloadMetadataLimit
	}
	return limit
}

func canonicalizeOptionalBatchInstrumentRef(
	instrument *BatchInstrumentRef,
	subject string,
) (BatchInstrumentRef, bool, error) {
	if instrument == nil {
		return BatchInstrumentRef{}, false, nil
	}

	symbolValue := strings.TrimSpace(instrument.Symbol.String())
	assetClassValue := strings.TrimSpace(instrument.AssetClass.String())
	if symbolValue == "" && assetClassValue == "" {
		return BatchInstrumentRef{}, false, nil
	}
	if symbolValue == "" || assetClassValue == "" {
		return BatchInstrumentRef{}, false, validationError(
			subject + " instrument symbol and asset class must both be provided",
		)
	}

	symbol, err := domain.NewSymbol(symbolValue)
	if err != nil {
		return BatchInstrumentRef{}, false, validationError(subject + " instrument symbol is required")
	}
	assetClass, err := domain.NewAssetClass(assetClassValue)
	if err != nil {
		return BatchInstrumentRef{}, false, validationError(subject + " instrument asset class is required")
	}

	return BatchInstrumentRef{Symbol: symbol, AssetClass: assetClass}, true, nil
}

func canonicalizeOptionalTimeRange(
	timeRange *domain.TimeRange,
	subject string,
) (domain.TimeRange, bool, error) {
	if timeRange == nil {
		return domain.TimeRange{}, false, nil
	}

	canonicalTimeRange, err := domain.NewTimeRange(timeRange.Start, timeRange.End)
	if err != nil {
		return domain.TimeRange{}, false, validationError(subject + " " + err.Error())
	}

	return canonicalTimeRange, true, nil
}

func rawPayloadTimeRangePointer(startAt, endAt time.Time) *domain.TimeRange {
	if startAt.IsZero() && endAt.IsZero() {
		return nil
	}
	return &domain.TimeRange{Start: startAt, End: endAt}
}
