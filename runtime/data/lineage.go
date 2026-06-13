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
	ID             string
	IngestionRunID string
	Source         string
	Venue          domain.Venue
	ContentType    string
	Body           []byte
	Checksum       string
	ReceivedAt     time.Time
	RequestKey     string
	SourceRecordID string
	Metadata       map[string]string
}

// RawVenuePayloadParams configures raw payload construction.
type RawVenuePayloadParams struct {
	ID             string
	IngestionRunID string
	Source         string
	Venue          domain.Venue
	ContentType    string
	Body           []byte
	Checksum       string
	ReceivedAt     time.Time
	RequestKey     string
	SourceRecordID string
	Metadata       map[string]string
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
	IngestionRun IngestionRun
}

// DataBatchAudit returns the lineage chain for one persisted batch.
//
//nolint:revive // OpenSpec lineage terminology requires DataBatchAudit naming.
type DataBatchAudit struct {
	Batch            DataBatch
	NormalizationRun NormalizationRun
	RawPayloads      []RawVenuePayloadAudit
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
	if ingestionRunID == "" {
		return RawVenuePayload{}, validationError("raw payload ingestion run id is required")
	}

	source := strings.TrimSpace(payload.Source)
	if source == "" {
		return RawVenuePayload{}, validationError("raw payload source is required")
	}

	venue, err := domain.NewVenue(payload.Venue.String())
	if err != nil {
		return RawVenuePayload{}, validationError("raw payload venue is required")
	}

	contentType := strings.TrimSpace(payload.ContentType)
	if contentType == "" {
		return RawVenuePayload{}, validationError("raw payload content type is required")
	}

	if len(payload.Body) == 0 {
		return RawVenuePayload{}, validationError("raw payload body is required")
	}

	checksum := strings.TrimSpace(payload.Checksum)
	if checksum == "" {
		return RawVenuePayload{}, validationError("raw payload checksum is required")
	}

	if payload.ReceivedAt.IsZero() {
		return RawVenuePayload{}, validationError("raw payload received time is required")
	}

	metadata := canonicalizeMetadataMap(payload.Metadata)
	body := slices.Clone(payload.Body)

	return RawVenuePayload{
		ID:             id,
		IngestionRunID: ingestionRunID,
		Source:         source,
		Venue:          venue,
		ContentType:    contentType,
		Body:           body,
		Checksum:       checksum,
		ReceivedAt:     payload.ReceivedAt.UTC(),
		RequestKey:     strings.TrimSpace(payload.RequestKey),
		SourceRecordID: strings.TrimSpace(payload.SourceRecordID),
		Metadata:       metadata,
	}, nil
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
