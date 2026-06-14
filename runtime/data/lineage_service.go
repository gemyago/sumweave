package data

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/gemyago/signal-foundry/runtime/domain"
)

type lineageStore interface {
	UpsertIngestionRun(ctx context.Context, run IngestionRun) (IngestionRun, error)
	UpsertRawVenuePayload(ctx context.Context, payload RawVenuePayload) (RawVenuePayload, error)
	UpsertNormalizationRun(ctx context.Context, run NormalizationRun) (NormalizationRun, error)
	UpsertDataBatch(ctx context.Context, batch DataBatch) (DataBatch, error)
	LinkRawPayloadToInstrument(ctx context.Context, rawPayloadID string, instrument domain.Instrument) error
	LinkRawPayloadToCandle(ctx context.Context, rawPayloadID string, candle domain.Candle) error
	LinkRawPayloadToTrade(ctx context.Context, rawPayloadID string, trade domain.Trade) error
	ListInstrumentRawPayloadIDs(ctx context.Context, instrument domain.Instrument) ([]string, error)
	ListCandleRawPayloadIDs(ctx context.Context, candle domain.Candle) ([]string, error)
	ListTradeRawPayloadIDs(ctx context.Context, trade domain.Trade) ([]string, error)
	GetDataBatchAudit(ctx context.Context, batchID string) (DataBatchAudit, error)
	ReplayCandlesByDataBatch(ctx context.Context, batchID string) ([]ReplayCandle, error)
	ReplayTradesByDataBatch(ctx context.Context, batchID string) ([]ReplayTrade, error)
}

// RawPayloadBlobStore persists and reads raw response bodies by reference.
type RawPayloadBlobStore interface {
	StoreRawPayloadBody(ctx context.Context, payloadID string, body []byte) (RawPayloadBody, error)
	ReadRawPayloadBody(ctx context.Context, ref string) ([]byte, error)
}

// LineageServiceDeps configures lineage service dependencies.
type LineageServiceDeps struct {
	Store     lineageStore
	BlobStore RawPayloadBlobStore
}

// LineageService validates lineage records and delegates persistence or reads.
type LineageService struct {
	store     lineageStore
	blobStore RawPayloadBlobStore
}

// NewLineageService creates a lineage service with a consumer-defined store.
func NewLineageService(deps LineageServiceDeps) (*LineageService, error) {
	if deps.Store == nil {
		return nil, errors.New("lineage store is required")
	}
	if deps.BlobStore == nil {
		return nil, errors.New("raw payload blob store is required")
	}

	return &LineageService{store: deps.Store, blobStore: deps.BlobStore}, nil
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

	if canonicalPayload.PayloadBodyRef == "" {
		responseBody := canonicalPayload.ResponseBody
		storedBody, storeErr := s.blobStore.StoreRawPayloadBody(
			ctx,
			canonicalPayload.ID,
			responseBody,
		)
		if storeErr != nil {
			return RawVenuePayload{}, fmt.Errorf("store raw venue payload body: %w", storeErr)
		}
		if canonicalPayload.ResponseBodyHash != "" && canonicalPayload.ResponseBodyHash != storedBody.Hash {
			return RawVenuePayload{}, validationError("raw payload response body hash does not match stored body")
		}
		canonicalPayload.PayloadBodyRef = storedBody.Ref
		if canonicalPayload.ResponseBodyHash == "" {
			canonicalPayload.ResponseBodyHash = storedBody.Hash
		}
	} else if canonicalPayload.ResponseBodyHash == "" {
		sum := sha256.Sum256(canonicalPayload.ResponseBody)
		canonicalPayload.ResponseBodyHash = hex.EncodeToString(sum[:])
	}

	canonicalPayload.ResponseBody = nil

	persisted, err := s.store.UpsertRawVenuePayload(ctx, canonicalPayload)
	if err != nil {
		return RawVenuePayload{}, fmt.Errorf("upsert raw venue payload: %w", err)
	}

	body, err := s.blobStore.ReadRawPayloadBody(ctx, persisted.PayloadBodyRef)
	if err != nil {
		return RawVenuePayload{}, fmt.Errorf("read raw venue payload body: %w", err)
	}
	persisted.ResponseBody = body

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

// LinkRawPayloadToInstrument persists one raw-payload to instrument audit link.
func (s *LineageService) LinkRawPayloadToInstrument(
	ctx context.Context,
	rawPayloadID string,
	instrument domain.Instrument,
) error {
	canonicalRawPayloadID, err := canonicalizeRawPayloadID(rawPayloadID)
	if err != nil {
		return err
	}
	canonicalInstrument, err := canonicalizeInstrument(instrument)
	if err != nil {
		return err
	}
	if linkErr := s.store.LinkRawPayloadToInstrument(ctx, canonicalRawPayloadID, canonicalInstrument); linkErr != nil {
		return fmt.Errorf("link raw payload to instrument: %w", linkErr)
	}
	return nil
}

// LinkRawPayloadToCandle persists one raw-payload to candle audit link.
func (s *LineageService) LinkRawPayloadToCandle(
	ctx context.Context,
	rawPayloadID string,
	candle domain.Candle,
) error {
	canonicalRawPayloadID, err := canonicalizeRawPayloadID(rawPayloadID)
	if err != nil {
		return err
	}
	canonicalCandle, err := canonicalizeCandle(candle)
	if err != nil {
		return err
	}
	if linkErr := s.store.LinkRawPayloadToCandle(ctx, canonicalRawPayloadID, canonicalCandle); linkErr != nil {
		return fmt.Errorf("link raw payload to candle: %w", linkErr)
	}
	return nil
}

// LinkRawPayloadToTrade persists one raw-payload to trade audit link.
func (s *LineageService) LinkRawPayloadToTrade(
	ctx context.Context,
	rawPayloadID string,
	trade domain.Trade,
) error {
	canonicalRawPayloadID, err := canonicalizeRawPayloadID(rawPayloadID)
	if err != nil {
		return err
	}
	canonicalTrade, err := canonicalizeTrade(trade)
	if err != nil {
		return err
	}
	if linkErr := s.store.LinkRawPayloadToTrade(ctx, canonicalRawPayloadID, canonicalTrade); linkErr != nil {
		return fmt.Errorf("link raw payload to trade: %w", linkErr)
	}
	return nil
}

// ListInstrumentRawPayloadIDs returns deterministic raw payload IDs linked to an instrument.
func (s *LineageService) ListInstrumentRawPayloadIDs(
	ctx context.Context,
	instrument domain.Instrument,
) ([]string, error) {
	canonicalInstrument, err := canonicalizeInstrument(instrument)
	if err != nil {
		return nil, err
	}
	ids, err := s.store.ListInstrumentRawPayloadIDs(ctx, canonicalInstrument)
	if err != nil {
		return nil, fmt.Errorf("list instrument raw payload ids: %w", err)
	}
	return ids, nil
}

// ListCandleRawPayloadIDs returns deterministic raw payload IDs linked to a candle.
func (s *LineageService) ListCandleRawPayloadIDs(ctx context.Context, candle domain.Candle) ([]string, error) {
	canonicalCandle, err := canonicalizeCandle(candle)
	if err != nil {
		return nil, err
	}
	ids, err := s.store.ListCandleRawPayloadIDs(ctx, canonicalCandle)
	if err != nil {
		return nil, fmt.Errorf("list candle raw payload ids: %w", err)
	}
	return ids, nil
}

// ListTradeRawPayloadIDs returns deterministic raw payload IDs linked to a trade.
func (s *LineageService) ListTradeRawPayloadIDs(ctx context.Context, trade domain.Trade) ([]string, error) {
	canonicalTrade, err := canonicalizeTrade(trade)
	if err != nil {
		return nil, err
	}
	ids, err := s.store.ListTradeRawPayloadIDs(ctx, canonicalTrade)
	if err != nil {
		return nil, fmt.Errorf("list trade raw payload ids: %w", err)
	}
	return ids, nil
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
	for i := range audit.RawPayloads {
		if audit.RawPayloads[i].Payload.PayloadBodyRef == "" {
			continue
		}
		body, readErr := s.blobStore.ReadRawPayloadBody(ctx, audit.RawPayloads[i].Payload.PayloadBodyRef)
		if readErr != nil {
			return DataBatchAudit{}, fmt.Errorf("read raw payload body for audit: %w", readErr)
		}
		audit.RawPayloads[i].Payload.ResponseBody = body
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

func canonicalizeRawPayloadID(rawPayloadID string) (string, error) {
	canonicalRawPayloadID := strings.TrimSpace(rawPayloadID)
	if canonicalRawPayloadID == "" {
		return "", validationError("raw payload id is required")
	}

	return canonicalRawPayloadID, nil
}
