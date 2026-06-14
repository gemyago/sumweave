package data

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
)

// ErrLineageParentNotFound marks a missing persisted parent lineage record.
var ErrLineageParentNotFound = fmt.Errorf("%w: lineage parent not found", ErrValidation)

// IngestionRunStatus identifies a supported ingestion lineage state.
type IngestionRunStatus string

const (
	IngestionRunStatusStarted   IngestionRunStatus = "started"
	IngestionRunStatusSucceeded IngestionRunStatus = "succeeded"
	IngestionRunStatusFailed    IngestionRunStatus = "failed"
)

// NormalizationRunStatus identifies a supported normalization lineage state.
type NormalizationRunStatus string

const (
	NormalizationRunStatusStarted   NormalizationRunStatus = "started"
	NormalizationRunStatusSucceeded NormalizationRunStatus = "succeeded"
	NormalizationRunStatusFailed    NormalizationRunStatus = "failed"
)

// LineageRecordKind identifies a supported canonical record family.
type LineageRecordKind string

const (
	LineageRecordKindCandle LineageRecordKind = "candle"
	LineageRecordKindTrade  LineageRecordKind = "trade"
)

// IngestionRun describes a stable ingestion attempt lineage record.
type IngestionRun struct {
	ID           string
	Source       string
	Venue        domain.Venue
	Status       IngestionRunStatus
	StartedAt    time.Time
	CompletedAt  time.Time
	RecordCount  int
	ErrorSummary string
}

// IngestionRunParams configures ingestion run construction.
type IngestionRunParams struct {
	ID           string
	Source       string
	Venue        domain.Venue
	Status       IngestionRunStatus
	StartedAt    time.Time
	CompletedAt  time.Time
	RecordCount  int
	ErrorSummary string
}

// RawVenuePayload stores raw venue response lineage for audit and replay.
type RawVenuePayload struct {
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
	ResponseBody       []byte
	ResponseBodyHash   string
	PayloadBodyRef     string
	EntityHint         string
	Instrument         *BatchInstrumentRef
	Timeframe          domain.Timeframe
	TimeRange          *domain.TimeRange
	ReceivedAt         time.Time
}

// RawVenuePayloadParams configures raw payload construction.
type RawVenuePayloadParams struct {
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
	ResponseBody       []byte
	ResponseBodyHash   string
	PayloadBodyRef     string
	EntityHint         string
	Instrument         *BatchInstrumentRef
	Timeframe          domain.Timeframe
	TimeRange          *domain.TimeRange
	ReceivedAt         time.Time
}

// NormalizationRun describes a stable normalization lineage record.
type NormalizationRun struct {
	ID                   string
	RawPayloadIDs        []string
	Status               NormalizationRunStatus
	StartedAt            time.Time
	CompletedAt          time.Time
	RecordKind           LineageRecordKind
	SourceRecordCount    int
	CanonicalRecordCount int
	ErrorSummary         string
}

// NormalizationRunParams configures normalization run construction.
type NormalizationRunParams struct {
	ID                   string
	RawPayloadIDs        []string
	Status               NormalizationRunStatus
	StartedAt            time.Time
	CompletedAt          time.Time
	RecordKind           LineageRecordKind
	SourceRecordCount    int
	CanonicalRecordCount int
	ErrorSummary         string
}

// BatchInstrumentRef identifies an optional instrument attached to a batch.
type BatchInstrumentRef struct {
	Symbol     domain.Symbol
	AssetClass domain.AssetClass
}

// DataBatch describes a stable persisted canonical batch lineage record.
//
//nolint:revive // OpenSpec lineage terminology requires DataBatch naming.
type DataBatch struct {
	ID                 string
	NormalizationRunID string
	Venue              domain.Venue
	Instrument         *BatchInstrumentRef
	RecordKind         LineageRecordKind
	TimeRange          domain.TimeRange
	Quality            domain.DataQuality
	RecordCount        int
	Summary            string
}

// DataBatchParams configures data batch construction.
//
//nolint:revive // OpenSpec lineage terminology requires DataBatchParams naming.
type DataBatchParams struct {
	ID                 string
	NormalizationRunID string
	Venue              domain.Venue
	Instrument         *BatchInstrumentRef
	RecordKind         LineageRecordKind
	TimeRange          domain.TimeRange
	Quality            domain.DataQuality
	RecordCount        int
	Summary            string
}

// RawVenuePayloadAudit bundles a raw payload and its parent ingestion run.
type RawVenuePayloadAudit struct {
	Payload      RawVenuePayload
	IngestionRun *IngestionRun
}

// DataBatchAudit returns the lineage chain for one persisted batch.
//
//nolint:revive // OpenSpec lineage terminology requires DataBatchAudit naming.
type DataBatchAudit struct {
	Batch            DataBatch
	NormalizationRun NormalizationRun
	RawPayloads      []RawVenuePayloadAudit
}

// RawPayloadBody stores a raw body blob reference and checksum.
type RawPayloadBody struct {
	Ref  string
	Hash string
}

// NewIngestionRun validates and canonicalizes an ingestion run lineage record.
func NewIngestionRun(params IngestionRunParams) (IngestionRun, error) {
	run := IngestionRun(params)
	return canonicalizeIngestionRun(run)
}

// NewRawVenuePayload validates and canonicalizes a raw venue payload lineage record.
func NewRawVenuePayload(params RawVenuePayloadParams) (RawVenuePayload, error) {
	payload := RawVenuePayload(params)
	return canonicalizeRawVenuePayload(payload)
}

// NewNormalizationRun validates and canonicalizes a normalization run lineage record.
func NewNormalizationRun(params NormalizationRunParams) (NormalizationRun, error) {
	run := NormalizationRun(params)
	return canonicalizeNormalizationRun(run)
}

// NewDataBatch validates and canonicalizes a data batch lineage record.
func NewDataBatch(params DataBatchParams) (DataBatch, error) {
	batch := DataBatch(params)
	return canonicalizeDataBatch(batch)
}

func canonicalizeIngestionRun(run IngestionRun) (IngestionRun, error) {
	id := strings.TrimSpace(run.ID)
	if id == "" {
		return IngestionRun{}, validationError("ingestion run id is required")
	}

	source := strings.TrimSpace(run.Source)
	if source == "" {
		return IngestionRun{}, validationError("ingestion run source is required")
	}

	venue, err := domain.NewVenue(run.Venue.String())
	if err != nil {
		return IngestionRun{}, validationError("ingestion run venue is required")
	}

	status, err := newIngestionRunStatus(run.Status.String())
	if err != nil {
		return IngestionRun{}, err
	}

	startedAt, completedAt, err := canonicalizeLineageTimes(run.StartedAt, run.CompletedAt, "ingestion run")
	if err != nil {
		return IngestionRun{}, err
	}

	if run.RecordCount < 0 {
		return IngestionRun{}, validationError("ingestion run record count must be non-negative")
	}

	return IngestionRun{
		ID:           id,
		Source:       source,
		Venue:        venue,
		Status:       status,
		StartedAt:    startedAt,
		CompletedAt:  completedAt,
		RecordCount:  run.RecordCount,
		ErrorSummary: strings.TrimSpace(run.ErrorSummary),
	}, nil
}

func canonicalizeRawVenuePayload(payload RawVenuePayload) (RawVenuePayload, error) {
	id := strings.TrimSpace(payload.ID)
	if id == "" {
		return RawVenuePayload{}, validationError("raw payload id is required")
	}

	ingestionRunID := strings.TrimSpace(payload.IngestionRunID)

	source := strings.TrimSpace(payload.Source)
	if source == "" {
		return RawVenuePayload{}, validationError("raw payload source is required")
	}

	venue, err := domain.NewVenue(payload.Venue.String())
	if err != nil {
		return RawVenuePayload{}, validationError("raw payload venue is required")
	}

	endpoint := strings.TrimSpace(payload.Endpoint)
	if endpoint == "" {
		return RawVenuePayload{}, validationError("raw payload endpoint is required")
	}

	requestType := strings.TrimSpace(payload.RequestType)
	if requestType == "" {
		return RawVenuePayload{}, validationError("raw payload request type is required")
	}

	requestPayloadHash := strings.TrimSpace(payload.RequestPayloadHash)
	if requestPayloadHash == "" {
		return RawVenuePayload{}, validationError("raw payload request payload hash is required")
	}

	requestAt, responseAt, err := canonicalizeRawPayloadExchangeTimes(payload.RequestAt, payload.ResponseAt)
	if err != nil {
		return RawVenuePayload{}, err
	}

	if payload.HTTPStatus < 100 || payload.HTTPStatus > 999 {
		return RawVenuePayload{}, validationError("raw payload http status must be a three-digit code")
	}

	responseBody, responseBodyHash, payloadBodyRef, err := canonicalizeRawPayloadBody(payload)
	if err != nil {
		return RawVenuePayload{}, err
	}

	if payload.ReceivedAt.IsZero() {
		return RawVenuePayload{}, validationError("raw payload received time is required")
	}

	requestMetadata := canonicalizeMetadataMap(payload.RequestMetadata)
	instrument, timeframe, timeRange, err := canonicalizeRawPayloadScope(payload)
	if err != nil {
		return RawVenuePayload{}, err
	}

	return RawVenuePayload{
		ID:                 id,
		IngestionRunID:     ingestionRunID,
		Source:             source,
		Venue:              venue,
		Endpoint:           endpoint,
		RequestType:        requestType,
		RequestPayloadHash: requestPayloadHash,
		RequestMetadata:    requestMetadata,
		RequestAt:          requestAt,
		ResponseAt:         responseAt,
		HTTPStatus:         payload.HTTPStatus,
		ResponseBody:       responseBody,
		ResponseBodyHash:   responseBodyHash,
		PayloadBodyRef:     payloadBodyRef,
		EntityHint:         strings.TrimSpace(payload.EntityHint),
		Instrument:         instrument,
		Timeframe:          timeframe,
		TimeRange:          timeRange,
		ReceivedAt:         payload.ReceivedAt.UTC(),
	}, nil
}

func canonicalizeRawPayloadExchangeTimes(
	requestAt time.Time,
	responseAt time.Time,
) (time.Time, time.Time, error) {
	if requestAt.IsZero() {
		return time.Time{}, time.Time{}, validationError("raw payload request time is required")
	}
	if responseAt.IsZero() {
		return time.Time{}, time.Time{}, validationError("raw payload response time is required")
	}

	canonicalRequestAt := requestAt.UTC()
	canonicalResponseAt := responseAt.UTC()
	if canonicalResponseAt.Before(canonicalRequestAt) {
		return time.Time{}, time.Time{}, validationError(
			"raw payload response time must not be before request time",
		)
	}

	return canonicalRequestAt, canonicalResponseAt, nil
}

func canonicalizeRawPayloadBody(
	payload RawVenuePayload,
) ([]byte, string, string, error) {
	responseBody := slices.Clone(payload.ResponseBody)
	responseBodyHash := strings.TrimSpace(payload.ResponseBodyHash)
	payloadBodyRef := strings.TrimSpace(payload.PayloadBodyRef)

	if responseBodyHash == "" && len(responseBody) == 0 {
		return nil, "", "", validationError("raw payload response body hash is required")
	}
	if payloadBodyRef == "" && len(responseBody) == 0 {
		return nil, "", "", validationError("raw payload response body or body ref is required")
	}

	return responseBody, responseBodyHash, payloadBodyRef, nil
}

func canonicalizeRawPayloadScope(
	payload RawVenuePayload,
) (*BatchInstrumentRef, domain.Timeframe, *domain.TimeRange, error) {
	var instrument *BatchInstrumentRef
	if payload.Instrument != nil {
		symbol, symbolErr := domain.NewSymbol(payload.Instrument.Symbol.String())
		if symbolErr != nil {
			return nil, "", nil, validationError("raw payload instrument symbol is required")
		}
		assetClass, assetClassErr := domain.NewAssetClass(payload.Instrument.AssetClass.String())
		if assetClassErr != nil {
			return nil, "", nil, validationError("raw payload instrument asset class is required")
		}
		instrument = &BatchInstrumentRef{Symbol: symbol, AssetClass: assetClass}
	}

	var timeframe domain.Timeframe
	if strings.TrimSpace(payload.Timeframe.String()) != "" {
		canonicalTimeframe, timeframeErr := domain.NewTimeframe(payload.Timeframe.String())
		if timeframeErr != nil {
			return nil, "", nil, validationError("raw payload timeframe is invalid")
		}
		timeframe = canonicalTimeframe
	}

	var timeRange *domain.TimeRange
	if payload.TimeRange != nil {
		canonicalTimeRange, timeRangeErr := domain.NewTimeRange(
			payload.TimeRange.Start,
			payload.TimeRange.End,
		)
		if timeRangeErr != nil {
			return nil, "", nil, validationError(timeRangeErr.Error())
		}
		timeRange = &canonicalTimeRange
	}

	return instrument, timeframe, timeRange, nil
}

func canonicalizeNormalizationRun(run NormalizationRun) (NormalizationRun, error) {
	id := strings.TrimSpace(run.ID)
	if id == "" {
		return NormalizationRun{}, validationError("normalization run id is required")
	}

	rawPayloadIDs := make([]string, 0, len(run.RawPayloadIDs))
	for _, rawPayloadID := range run.RawPayloadIDs {
		canonicalRawPayloadID := strings.TrimSpace(rawPayloadID)
		if canonicalRawPayloadID == "" {
			return NormalizationRun{}, validationError("normalization run raw payload id is required")
		}
		rawPayloadIDs = append(rawPayloadIDs, canonicalRawPayloadID)
	}
	if len(rawPayloadIDs) == 0 {
		return NormalizationRun{}, validationError("normalization run raw payload ids are required")
	}

	status, err := newNormalizationRunStatus(run.Status.String())
	if err != nil {
		return NormalizationRun{}, err
	}

	startedAt, completedAt, err := canonicalizeLineageTimes(run.StartedAt, run.CompletedAt, "normalization run")
	if err != nil {
		return NormalizationRun{}, err
	}

	recordKind, err := newLineageRecordKind(run.RecordKind.String())
	if err != nil {
		return NormalizationRun{}, err
	}

	if run.SourceRecordCount < 0 {
		return NormalizationRun{}, validationError("normalization run source record count must be non-negative")
	}
	if run.CanonicalRecordCount < 0 {
		return NormalizationRun{}, validationError("normalization run canonical record count must be non-negative")
	}

	return NormalizationRun{
		ID:                   id,
		RawPayloadIDs:        rawPayloadIDs,
		Status:               status,
		StartedAt:            startedAt,
		CompletedAt:          completedAt,
		RecordKind:           recordKind,
		SourceRecordCount:    run.SourceRecordCount,
		CanonicalRecordCount: run.CanonicalRecordCount,
		ErrorSummary:         strings.TrimSpace(run.ErrorSummary),
	}, nil
}

func canonicalizeDataBatch(batch DataBatch) (DataBatch, error) {
	id := strings.TrimSpace(batch.ID)
	if id == "" {
		return DataBatch{}, validationError("data batch id is required")
	}

	normalizationRunID := strings.TrimSpace(batch.NormalizationRunID)
	if normalizationRunID == "" {
		return DataBatch{}, validationError("data batch normalization run id is required")
	}

	venue, err := domain.NewVenue(batch.Venue.String())
	if err != nil {
		return DataBatch{}, validationError("data batch venue is required")
	}

	var instrument *BatchInstrumentRef
	if batch.Instrument != nil {
		symbol, symbolErr := domain.NewSymbol(batch.Instrument.Symbol.String())
		if symbolErr != nil {
			return DataBatch{}, validationError("data batch instrument symbol is required")
		}

		assetClass, assetClassErr := domain.NewAssetClass(batch.Instrument.AssetClass.String())
		if assetClassErr != nil {
			return DataBatch{}, validationError("data batch instrument asset class is required")
		}

		instrument = &BatchInstrumentRef{
			Symbol:     symbol,
			AssetClass: assetClass,
		}
	}

	recordKind, err := newLineageRecordKind(batch.RecordKind.String())
	if err != nil {
		return DataBatch{}, err
	}

	timeRange, err := domain.NewTimeRange(batch.TimeRange.Start, batch.TimeRange.End)
	if err != nil {
		return DataBatch{}, validationError(err.Error())
	}

	quality, err := domain.NewDataQuality(batch.Quality.String())
	if err != nil {
		return DataBatch{}, validationError("data batch quality is required")
	}

	if batch.RecordCount < 0 {
		return DataBatch{}, validationError("data batch record count must be non-negative")
	}

	return DataBatch{
		ID:                 id,
		NormalizationRunID: normalizationRunID,
		Venue:              venue,
		Instrument:         instrument,
		RecordKind:         recordKind,
		TimeRange:          timeRange,
		Quality:            quality,
		RecordCount:        batch.RecordCount,
		Summary:            strings.TrimSpace(batch.Summary),
	}, nil
}

func canonicalizeLineageTimes(startedAt, completedAt time.Time, subject string) (time.Time, time.Time, error) {
	if startedAt.IsZero() {
		return time.Time{}, time.Time{}, validationError(subject + " started time is required")
	}

	canonicalStartedAt := startedAt.UTC()
	canonicalCompletedAt := time.Time{}
	if !completedAt.IsZero() {
		canonicalCompletedAt = completedAt.UTC()
		if canonicalCompletedAt.Before(canonicalStartedAt) {
			return time.Time{}, time.Time{}, validationError(
				subject + " completed time must not be before started time",
			)
		}
	}

	return canonicalStartedAt, canonicalCompletedAt, nil
}

func canonicalizeMetadataMap(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}

	canonical := make(map[string]string, len(metadata))
	for key, value := range maps.Clone(metadata) {
		canonicalKey := strings.TrimSpace(key)
		if canonicalKey == "" {
			continue
		}
		canonical[canonicalKey] = strings.TrimSpace(value)
	}

	if len(canonical) == 0 {
		return nil
	}

	return canonical
}

func newIngestionRunStatus(value string) (IngestionRunStatus, error) {
	status := IngestionRunStatus(strings.ToLower(strings.TrimSpace(value)))
	if !status.isValid() {
		return "", validationError("ingestion run status is required")
	}
	return status, nil
}

func newNormalizationRunStatus(value string) (NormalizationRunStatus, error) {
	status := NormalizationRunStatus(strings.ToLower(strings.TrimSpace(value)))
	if !status.isValid() {
		return "", validationError("normalization run status is required")
	}
	return status, nil
}

func newLineageRecordKind(value string) (LineageRecordKind, error) {
	kind := LineageRecordKind(strings.ToLower(strings.TrimSpace(value)))
	if !kind.isValid() {
		return "", validationError("lineage record kind is required")
	}
	return kind, nil
}

func (s IngestionRunStatus) String() string { return string(s) }

func (s IngestionRunStatus) isValid() bool {
	switch s {
	case IngestionRunStatusStarted, IngestionRunStatusSucceeded, IngestionRunStatusFailed:
		return true
	default:
		return false
	}
}

func (s NormalizationRunStatus) String() string { return string(s) }

func (s NormalizationRunStatus) isValid() bool {
	switch s {
	case NormalizationRunStatusStarted, NormalizationRunStatusSucceeded, NormalizationRunStatusFailed:
		return true
	default:
		return false
	}
}

func (k LineageRecordKind) String() string { return string(k) }

func (k LineageRecordKind) isValid() bool {
	switch k {
	case LineageRecordKindCandle, LineageRecordKindTrade:
		return true
	default:
		return false
	}
}
