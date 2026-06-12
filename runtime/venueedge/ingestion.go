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

// IngestionFlow reads canonical venue-edge records and persists them through the data slice.
type IngestionFlow struct {
	sink marketDataSink
}

// NewIngestionFlow creates a paging-aware venue ingestion flow.
func NewIngestionFlow(sink marketDataSink) (*IngestionFlow, error) {
	if sink == nil {
		return nil, errors.New("market data sink is required")
	}

	return &IngestionFlow{sink: sink}, nil
}

// IngestInstruments reads canonical instruments from a venue and upserts them through the data slice.
func (f *IngestionFlow) IngestInstruments(
	ctx context.Context,
	venue MarketDataVenue,
	request InstrumentReadRequest,
) ([]domain.Instrument, error) {
	if venue == nil {
		return nil, errors.New("market data venue is required")
	}

	persisted := make([]domain.Instrument, 0)
	pageRequest := request
	for {
		result, err := venue.ReadInstruments(ctx, pageRequest)
		if err != nil {
			return nil, fmt.Errorf("read instruments: %w", err)
		}

		for _, instrument := range result.Instruments {
			stored, persistErr := f.sink.UpsertInstrument(ctx, instrument)
			if persistErr != nil {
				return nil, fmt.Errorf("persist instrument: %w", persistErr)
			}
			persisted = append(persisted, stored)
		}

		if result.NextPageToken == "" {
			return persisted, nil
		}

		pageRequest.PageToken = result.NextPageToken
	}
}

// IngestCandles reads canonical candles from a venue and persists them through the data slice.
func (f *IngestionFlow) IngestCandles(
	ctx context.Context,
	venue MarketDataVenue,
	request CandleReadRequest,
) ([]domain.Candle, error) {
	if venue == nil {
		return nil, errors.New("market data venue is required")
	}

	persisted := make([]domain.Candle, 0)
	pageRequest := request
	for {
		result, err := venue.ReadCandles(ctx, pageRequest)
		if err != nil {
			return nil, fmt.Errorf("read candles: %w", err)
		}

		for _, candle := range result.Candles {
			stored, persistErr := f.sink.IngestCandle(ctx, candle)
			if persistErr != nil {
				return nil, fmt.Errorf("persist candle: %w", persistErr)
			}
			persisted = append(persisted, stored)
		}

		if result.NextPageToken == "" {
			return persisted, nil
		}

		pageRequest.PageToken = result.NextPageToken
	}
}

// IngestTrades reads canonical trades from a venue and persists them through the data slice.
func (f *IngestionFlow) IngestTrades(
	ctx context.Context,
	venue MarketDataVenue,
	request TradeReadRequest,
) ([]domain.Trade, error) {
	if venue == nil {
		return nil, errors.New("market data venue is required")
	}

	persisted := make([]domain.Trade, 0)
	pageRequest := request
	for {
		result, err := venue.ReadTrades(ctx, pageRequest)
		if err != nil {
			return nil, fmt.Errorf("read trades: %w", err)
		}

		for _, trade := range result.Trades {
			stored, persistErr := f.sink.IngestTrade(ctx, trade)
			if persistErr != nil {
				return nil, fmt.Errorf("persist trade: %w", persistErr)
			}
			persisted = append(persisted, stored)
		}

		if result.NextPageToken == "" {
			return persisted, nil
		}

		pageRequest.PageToken = result.NextPageToken
	}
}
