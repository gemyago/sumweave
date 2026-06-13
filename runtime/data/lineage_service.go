package data

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type lineageStore interface {
	UpsertIngestionRun(ctx context.Context, run IngestionRun) (IngestionRun, error)
	UpsertRawVenuePayload(ctx context.Context, payload RawVenuePayload) (RawVenuePayload, error)
	UpsertNormalizationRun(ctx context.Context, run NormalizationRun) (NormalizationRun, error)
	UpsertDataBatch(ctx context.Context, batch DataBatch) (DataBatch, error)
	GetDataBatchAudit(ctx context.Context, batchID string) (DataBatchAudit, error)
	ReplayCandlesByDataBatch(ctx context.Context, batchID string) ([]ReplayCandle, error)
	ReplayTradesByDataBatch(ctx context.Context, batchID string) ([]ReplayTrade, error)
}

// LineageServiceDeps configures lineage service dependencies.
type LineageServiceDeps struct {
	Store lineageStore
}

// LineageService validates lineage records and delegates persistence or reads.
type LineageService struct {
	store lineageStore
}

// NewLineageService creates a lineage service with a consumer-defined store.
func NewLineageService(deps LineageServiceDeps) (*LineageService, error) {
	if deps.Store == nil {
		return nil, errors.New("lineage store is required")
	}

	return &LineageService{store: deps.Store}, nil
}

// RecordIngestionRun validates and persists an ingestion run lineage record.
func (s *LineageService) RecordIngestionRun(ctx context.Context, run IngestionRun) (IngestionRun, error) {
	canonicalRun, err := canonicalizeIngestionRun(run)
	if err != nil {
		return IngestionRun{}, err
	}

	persisted, err := s.store.UpsertIngestionRun(ctx, canonicalRun)
	if err != nil {
		return IngestionRun{}, fmt.Errorf("upsert ingestion run: %w", err)
	}

	return persisted, nil
}

// RecordRawVenuePayload validates and persists a raw payload lineage record.
func (s *LineageService) RecordRawVenuePayload(
	ctx context.Context,
	payload RawVenuePayload,
) (RawVenuePayload, error) {
	canonicalPayload, err := canonicalizeRawVenuePayload(payload)
	if err != nil {
		return RawVenuePayload{}, err
	}

	persisted, err := s.store.UpsertRawVenuePayload(ctx, canonicalPayload)
	if err != nil {
		return RawVenuePayload{}, fmt.Errorf("upsert raw venue payload: %w", err)
	}

	return persisted, nil
}

// RecordNormalizationRun validates and persists a normalization lineage record.
func (s *LineageService) RecordNormalizationRun(
	ctx context.Context,
	run NormalizationRun,
) (NormalizationRun, error) {
	canonicalRun, err := canonicalizeNormalizationRun(run)
	if err != nil {
		return NormalizationRun{}, err
	}

	persisted, err := s.store.UpsertNormalizationRun(ctx, canonicalRun)
	if err != nil {
		return NormalizationRun{}, fmt.Errorf("upsert normalization run: %w", err)
	}

	return persisted, nil
}

// RecordDataBatch validates and persists a data batch lineage record.
func (s *LineageService) RecordDataBatch(ctx context.Context, batch DataBatch) (DataBatch, error) {
	canonicalBatch, err := canonicalizeDataBatch(batch)
	if err != nil {
		return DataBatch{}, err
	}

	persisted, err := s.store.UpsertDataBatch(ctx, canonicalBatch)
	if err != nil {
		return DataBatch{}, fmt.Errorf("upsert data batch: %w", err)
	}

	return persisted, nil
}

// GetDataBatchAudit returns batch-scoped lineage audit data.
func (s *LineageService) GetDataBatchAudit(ctx context.Context, batchID string) (DataBatchAudit, error) {
	canonicalBatchID, err := canonicalizeBatchID(batchID)
	if err != nil {
		return DataBatchAudit{}, err
	}

	audit, err := s.store.GetDataBatchAudit(ctx, canonicalBatchID)
	if err != nil {
		return DataBatchAudit{}, fmt.Errorf("get data batch audit: %w", err)
	}

	return audit, nil
}

// ReplayCandlesByDataBatch returns stable candle replay rows for one batch.
func (s *LineageService) ReplayCandlesByDataBatch(
	ctx context.Context,
	batchID string,
) ([]ReplayCandle, error) {
	canonicalBatchID, err := canonicalizeBatchID(batchID)
	if err != nil {
		return nil, err
	}

	replayRows, err := s.store.ReplayCandlesByDataBatch(ctx, canonicalBatchID)
	if err != nil {
		return nil, fmt.Errorf("replay candles by data batch: %w", err)
	}

	return replayRows, nil
}

// ReplayTradesByDataBatch returns stable trade replay rows for one batch.
func (s *LineageService) ReplayTradesByDataBatch(
	ctx context.Context,
	batchID string,
) ([]ReplayTrade, error) {
	canonicalBatchID, err := canonicalizeBatchID(batchID)
	if err != nil {
		return nil, err
	}

	replayRows, err := s.store.ReplayTradesByDataBatch(ctx, canonicalBatchID)
	if err != nil {
		return nil, fmt.Errorf("replay trades by data batch: %w", err)
	}

	return replayRows, nil
}

func canonicalizeBatchID(batchID string) (string, error) {
	canonicalBatchID := strings.TrimSpace(batchID)
	if canonicalBatchID == "" {
		return "", validationError("data batch id is required")
	}

	return canonicalBatchID, nil
}
