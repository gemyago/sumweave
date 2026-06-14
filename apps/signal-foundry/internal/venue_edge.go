package internal

import (
	"context"
	"errors"
	"fmt"

	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/venueedge"
)

type hyperliquidRawEvidenceRecorder struct {
	lineageService *data.LineageService
}

func newHyperliquidRawEvidenceRecorder(lineageService *data.LineageService) (*hyperliquidRawEvidenceRecorder, error) {
	if lineageService == nil {
		return nil, errors.New("data lineage service is required")
	}

	return &hyperliquidRawEvidenceRecorder{lineageService: lineageService}, nil
}

func (r *hyperliquidRawEvidenceRecorder) RecordHyperliquidRawEvidence(
	ctx context.Context,
	capture venueedge.HyperliquidRawEvidenceCapture,
) (string, error) {
	payload, err := data.NewRawVenuePayload(data.RawVenuePayloadParams{
		ID:                 capture.ID,
		IngestionRunID:     capture.IngestionRunID,
		Source:             string(capture.Venue) + "-rest",
		Venue:              capture.Venue,
		Endpoint:           capture.Endpoint,
		RequestType:        capture.RequestType,
		RequestPayloadHash: capture.RequestPayloadHash,
		RequestMetadata:    capture.RequestMetadata,
		RequestAt:          capture.RequestAt,
		ResponseAt:         capture.ResponseAt,
		HTTPStatus:         capture.HTTPStatus,
		ResponseBody:       capture.ResponseBody,
		EntityHint:         capture.EntityHint,
		Instrument:         rawPayloadInstrumentRef(capture.Instrument),
		Timeframe:          capture.Timeframe,
		TimeRange:          capture.TimeRange,
		ReceivedAt:         capture.ReceivedAt,
	})
	if err != nil {
		return "", fmt.Errorf("build raw venue payload: %w", err)
	}

	persisted, err := r.lineageService.RecordRawVenuePayload(ctx, payload)
	if err != nil {
		return "", err
	}

	return persisted.ID, nil
}

func newVenueIngestionFlow(
	ingestionService *data.IngestionService,
	lineageService *data.LineageService,
) (*venueedge.IngestionFlow, error) {
	if ingestionService == nil {
		return nil, errors.New("data ingestion service is required")
	}

	flow, _ := venueedge.NewIngestionFlow(ingestionService)
	if lineageService != nil {
		flow.WithRawPayloadLineage(lineageService)
	}

	return flow, nil
}

func rawPayloadInstrumentRef(instrument *domain.Instrument) *data.BatchInstrumentRef {
	if instrument == nil {
		return nil
	}

	return &data.BatchInstrumentRef{
		Symbol:     instrument.Symbol,
		AssetClass: instrument.AssetClass,
	}
}
