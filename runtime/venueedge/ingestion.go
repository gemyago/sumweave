package venueedge

import (
	"context"
	"errors"
	"fmt"

	"github.com/gemyago/signal-foundry/runtime/domain"
)

type marketDataSink interface {
	UpsertInstrument(ctx context.Context, instrument domain.Instrument) (domain.Instrument, error)
	IngestCandle(ctx context.Context, candle domain.Candle) (domain.Candle, error)
	IngestTrade(ctx context.Context, trade domain.Trade) (domain.Trade, error)
}

type rawPayloadLineageSink interface {
	LinkRawPayloadToInstrument(ctx context.Context, rawPayloadID string, instrument domain.Instrument) error
	LinkRawPayloadToCandle(ctx context.Context, rawPayloadID string, candle domain.Candle) error
	LinkRawPayloadToTrade(ctx context.Context, rawPayloadID string, trade domain.Trade) error
}

// IngestionFlow reads canonical venue-edge records and persists them through the data slice.
type IngestionFlow struct {
	sink        marketDataSink
	lineageSink rawPayloadLineageSink
}

// NewIngestionFlow creates a paging-aware venue ingestion flow.
func NewIngestionFlow(sink marketDataSink) (*IngestionFlow, error) {
	if sink == nil {
		return nil, errors.New("market data sink is required")
	}

	return &IngestionFlow{sink: sink}, nil
}

// WithRawPayloadLineage configures optional raw payload-to-record linkage.
func (f *IngestionFlow) WithRawPayloadLineage(lineageSink rawPayloadLineageSink) *IngestionFlow {
	f.lineageSink = lineageSink
	return f
}

func (f *IngestionFlow) persistRawPayloadLinksForInstrument(
	ctx context.Context,
	rawPayloadIDs []string,
	instrument domain.Instrument,
) error {
	for _, rawPayloadID := range rawPayloadIDs {
		if linkErr := f.lineageSink.LinkRawPayloadToInstrument(ctx, rawPayloadID, instrument); linkErr != nil {
			return fmt.Errorf("link raw payload to instrument: %w", linkErr)
		}
	}

	return nil
}

func (f *IngestionFlow) persistRawPayloadLinksForCandle(
	ctx context.Context,
	rawPayloadIDs []string,
	candle domain.Candle,
) error {
	for _, rawPayloadID := range rawPayloadIDs {
		if linkErr := f.lineageSink.LinkRawPayloadToCandle(ctx, rawPayloadID, candle); linkErr != nil {
			return fmt.Errorf("link raw payload to candle: %w", linkErr)
		}
	}

	return nil
}

func (f *IngestionFlow) persistRawPayloadLinksForTrade(
	ctx context.Context,
	rawPayloadIDs []string,
	trade domain.Trade,
) error {
	for _, rawPayloadID := range rawPayloadIDs {
		if linkErr := f.lineageSink.LinkRawPayloadToTrade(ctx, rawPayloadID, trade); linkErr != nil {
			return fmt.Errorf("link raw payload to trade: %w", linkErr)
		}
	}

	return nil
}

// IngestInstruments reads canonical instruments from a venue and upserts them through the data slice.
//
//nolint:dupl // ingestion loops intentionally share same pagination pattern
func (f *IngestionFlow) IngestInstruments(
	ctx context.Context,
	venue MarketDataVenue,
	request InstrumentReadRequest,
) ([]domain.Instrument, error) {
	if venue == nil {
		return nil, errors.New("market data venue is required")
	}

	return ingestRecords(
		ctx,
		request,
		venue.ReadInstruments,
		func(result InstrumentReadResult) string { return result.NextPageToken },
		func(request *InstrumentReadRequest, pageToken string) { request.PageToken = pageToken },
		func(result InstrumentReadResult) []domain.Instrument { return result.Instruments },
		func(result InstrumentReadResult) []string { return result.Metadata.RawPayloadIDs },
		func(ctx context.Context, instrument domain.Instrument) (domain.Instrument, error) {
			return f.sink.UpsertInstrument(ctx, instrument)
		},
		func(ctx context.Context, rawPayloadIDs []string, instrument domain.Instrument) error {
			if len(rawPayloadIDs) == 0 || f.lineageSink == nil {
				return nil
			}
			return f.persistRawPayloadLinksForInstrument(ctx, rawPayloadIDs, instrument)
		},
		"read instruments",
		"persist instrument",
	)
}

// IngestCandles reads canonical candles from a venue and persists them through the data slice.
//
//nolint:dupl // ingestion loops intentionally share same pagination pattern
func (f *IngestionFlow) IngestCandles(
	ctx context.Context,
	venue MarketDataVenue,
	request CandleReadRequest,
) ([]domain.Candle, error) {
	if venue == nil {
		return nil, errors.New("market data venue is required")
	}

	return ingestRecords(
		ctx,
		request,
		venue.ReadCandles,
		func(result CandleReadResult) string { return result.NextPageToken },
		func(request *CandleReadRequest, pageToken string) { request.PageToken = pageToken },
		func(result CandleReadResult) []domain.Candle { return result.Candles },
		func(result CandleReadResult) []string { return result.Metadata.RawPayloadIDs },
		func(ctx context.Context, candle domain.Candle) (domain.Candle, error) {
			return f.sink.IngestCandle(ctx, candle)
		},
		func(ctx context.Context, rawPayloadIDs []string, candle domain.Candle) error {
			if len(rawPayloadIDs) == 0 || f.lineageSink == nil {
				return nil
			}
			return f.persistRawPayloadLinksForCandle(ctx, rawPayloadIDs, candle)
		},
		"read candles",
		"persist candle",
	)
}

// IngestTrades reads canonical trades from a venue and persists them through the data slice.
//
//nolint:dupl // ingestion loops intentionally share same pagination pattern
func (f *IngestionFlow) IngestTrades(
	ctx context.Context,
	venue MarketDataVenue,
	request TradeReadRequest,
) ([]domain.Trade, error) {
	if venue == nil {
		return nil, errors.New("market data venue is required")
	}

	return ingestRecords(
		ctx,
		request,
		venue.ReadTrades,
		func(result TradeReadResult) string { return result.NextPageToken },
		func(request *TradeReadRequest, pageToken string) { request.PageToken = pageToken },
		func(result TradeReadResult) []domain.Trade { return result.Trades },
		func(result TradeReadResult) []string { return result.Metadata.RawPayloadIDs },
		func(ctx context.Context, trade domain.Trade) (domain.Trade, error) {
			return f.sink.IngestTrade(ctx, trade)
		},
		func(ctx context.Context, rawPayloadIDs []string, trade domain.Trade) error {
			if len(rawPayloadIDs) == 0 || f.lineageSink == nil {
				return nil
			}
			return f.persistRawPayloadLinksForTrade(ctx, rawPayloadIDs, trade)
		},
		"read trades",
		"persist trade",
	)
}

func ingestRecords[
	Req any,
	Res any,
	Row any,
](
	ctx context.Context,
	request Req,
	readPage func(context.Context, Req) (Res, error),
	readNextPageToken func(Res) string,
	setNextPageToken func(*Req, string),
	readRows func(Res) []Row,
	readRawPayloadIDs func(Res) []string,
	persistRow func(context.Context, Row) (Row, error),
	linkRows func(context.Context, []string, Row) error,
	readErrMsg string,
	persistErrMsg string,
) ([]Row, error) {
	persisted := make([]Row, 0)
	pageRequest := request
	for {
		result, err := readPage(ctx, pageRequest)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", readErrMsg, err)
		}

		rawPayloadIDs := readRawPayloadIDs(result)
		for _, row := range readRows(result) {
			stored, persistErr := persistRow(ctx, row)
			if persistErr != nil {
				return nil, fmt.Errorf("%s: %w", persistErrMsg, persistErr)
			}
			if linkErr := linkRows(ctx, rawPayloadIDs, stored); linkErr != nil {
				return nil, linkErr
			}
			persisted = append(persisted, stored)
		}

		nextPageToken := readNextPageToken(result)
		if nextPageToken == "" {
			return persisted, nil
		}

		setNextPageToken(&pageRequest, nextPageToken)
	}
}
