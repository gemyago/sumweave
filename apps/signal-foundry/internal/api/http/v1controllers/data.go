package v1controllers

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/middleware"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/v1routes/handlers"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/v1routes/models"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"go.uber.org/dig"
)

const maxCandleIntervals = 10_000

const supportedHistoricalDataVenue = "hyperliquid-perps"

type replayReadService interface {
	ReplayCandles(
		ctx context.Context,
		instrument domain.Instrument,
		timeframe domain.Timeframe,
		timeRange domain.TimeRange,
	) ([]data.ReplayCandle, error)
}

type lineageBrowserService interface {
	ListRawPayloadMetadata(
		ctx context.Context,
		query data.RawPayloadMetadataListQuery,
	) (data.RawPayloadMetadataListResult, error)
	GetRawPayloadDetail(ctx context.Context, rawPayloadID string) (data.RawPayloadDetail, error)
	ListCandleLinkedRawPayloadMetadata(
		ctx context.Context,
		query data.CandleLinkedRawPayloadsQuery,
	) ([]data.RawPayloadMetadata, error)
}

var _ replayReadService = (*data.ReadService)(nil)
var _ lineageBrowserService = (*data.LineageService)(nil)

type DataControllerDeps struct {
	dig.In

	ReadService    replayReadService
	LineageService lineageBrowserService
	AuthMiddleware middleware.AuthMiddleware
}

type DataController struct {
	deps DataControllerDeps
}

func NewDataController(deps DataControllerDeps) *DataController {
	return &DataController{deps: deps}
}

var _ handlers.DataController = (*DataController)(nil)

func (c *DataController) GetDataRawPayload(
	builder handlers.HandlerBuilder[*models.GetDataRawPayloadParams, *models.RawPayloadDetailResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.GetDataRawPayloadParams,
	) (*models.RawPayloadDetailResponse, error) {
		detail, err := c.deps.LineageService.GetRawPayloadDetail(ctx, params.ID)
		if err != nil {
			return nil, mapDataReadError(err, params.ID)
		}

		metadata := mapRawPayloadMetadata(detail.Metadata)

		return &models.RawPayloadDetailResponse{
			Metadata:                     &metadata,
			ResponseBodySizeBytes:        int64(detail.ResponseBodySizeBytes),
			ResponseBodyPreview:          string(detail.ResponseBodyPreview),
			ResponseBodyPreviewTruncated: detail.ResponseBodyPreviewTruncated,
		}, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *DataController) ListDataCandleRawPayloads(
	builder handlers.HandlerBuilder[
		*models.ListDataCandleRawPayloadsParams,
		*models.CandleRawPayloadMetadataListResponse,
	],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ListDataCandleRawPayloadsParams,
	) (*models.CandleRawPayloadMetadataListResponse, error) {
		venue, err := validateSupportedVenue(params.Venue)
		if err != nil {
			return nil, err
		}

		query, err := data.NewCandleLinkedRawPayloadsQuery(data.CandleLinkedRawPayloadsQueryParams{
			Venue:              venue,
			Symbol:             domain.Symbol(params.Symbol),
			AssetClass:         domain.AssetClass(params.AssetClass),
			Timeframe:          domain.Timeframe(params.Timeframe),
			StartAt:            params.Start,
			EndAt:              params.End,
			ProvenanceSource:   params.ProvenanceSource,
			ProvenanceIdentity: params.ProvenanceIDentity,
		})
		if err != nil {
			return nil, mapDataReadError(err, "candle-raw-payloads")
		}

		items, err := c.deps.LineageService.ListCandleLinkedRawPayloadMetadata(ctx, query)
		if err != nil {
			return nil, mapDataReadError(err, "candle-raw-payloads")
		}

		responseItems := make([]*models.RawPayloadMetadata, len(items))
		for i := range items {
			metadata := mapRawPayloadMetadata(items[i])
			responseItems[i] = &metadata
		}

		return &models.CandleRawPayloadMetadataListResponse{Items: responseItems}, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *DataController) ListDataCandles(
	builder handlers.HandlerBuilder[*models.ListDataCandlesParams, *models.DataCandleListResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ListDataCandlesParams,
	) (*models.DataCandleListResponse, error) {
		instrument, timeframe, timeRange, err := validateCandleRequest(params)
		if err != nil {
			return nil, err
		}

		replayed, err := c.deps.ReadService.ReplayCandles(ctx, instrument, timeframe, timeRange)
		if err != nil {
			return nil, mapDataReadError(err, params.Symbol)
		}

		items := make([]*models.DataCandle, len(replayed))
		for i := range replayed {
			candle, mapErr := mapReplayCandle(replayed[i])
			if mapErr != nil {
				return nil, mapErr
			}
			items[i] = &candle
		}

		return &models.DataCandleListResponse{Items: items}, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *DataController) ListDataRawPayloads(
	builder handlers.HandlerBuilder[
		*models.ListDataRawPayloadsParams,
		*models.RawPayloadMetadataListResponse,
	],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ListDataRawPayloadsParams,
	) (*models.RawPayloadMetadataListResponse, error) {
		venue, err := validateSupportedVenue(params.Venue)
		if err != nil {
			return nil, err
		}

		query, err := data.NewRawPayloadMetadataListQuery(data.RawPayloadMetadataListQueryParams{
			Venue:          venue,
			Symbol:         domain.Symbol(params.Symbol),
			AssetClass:     domain.AssetClass(params.AssetClass),
			Timeframe:      domain.Timeframe(params.Timeframe),
			StartAt:        params.Start,
			EndAt:          params.End,
			IngestionRunID: params.IngestionRunID,
			EntityHint:     params.EntityHint,
			Endpoint:       params.Endpoint,
			RequestType:    params.RequestType,
			Limit:          int(params.Limit),
			Cursor:         params.Cursor,
		})
		if err != nil {
			return nil, mapDataReadError(err, "raw-payloads")
		}

		result, err := c.deps.LineageService.ListRawPayloadMetadata(ctx, query)
		if err != nil {
			return nil, mapDataReadError(err, "raw-payloads")
		}

		items := make([]*models.RawPayloadMetadata, len(result.Items))
		for i := range result.Items {
			metadata := mapRawPayloadMetadata(result.Items[i])
			items[i] = &metadata
		}

		return &models.RawPayloadMetadataListResponse{Items: items, NextCursor: result.NextCursor}, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func validateCandleRequest(
	params *models.ListDataCandlesParams,
) (domain.Instrument, domain.Timeframe, domain.TimeRange, error) {
	venue, err := domain.NewVenue(params.Venue)
	if err != nil {
		return domain.Instrument{}, "", domain.TimeRange{}, app.NewErrInvalidInput("venue", err.Error())
	}
	if venue != domain.Venue(supportedHistoricalDataVenue) {
		return domain.Instrument{}, "", domain.TimeRange{}, app.NewErrInvalidInput("venue", "unsupported venue")
	}

	symbol, err := domain.NewSymbol(params.Symbol)
	if err != nil {
		return domain.Instrument{}, "", domain.TimeRange{}, app.NewErrInvalidInput("symbol", err.Error())
	}

	assetClass, err := domain.NewAssetClass(params.AssetClass)
	if err != nil {
		return domain.Instrument{}, "", domain.TimeRange{}, app.NewErrInvalidInput("assetClass", err.Error())
	}

	timeframe, err := domain.NewTimeframe(params.Timeframe)
	if err != nil {
		return domain.Instrument{}, "", domain.TimeRange{}, app.NewErrInvalidInput("timeframe", err.Error())
	}

	timeRange, err := domain.NewTimeRange(params.Start, params.End)
	if err != nil {
		return domain.Instrument{}, "", domain.TimeRange{}, app.NewErrInvalidInput("range", err.Error())
	}

	duration, err := timeframeDuration(timeframe)
	if err != nil {
		return domain.Instrument{}, "", domain.TimeRange{}, app.NewErrInvalidInput("timeframe", "unsupported timeframe")
	}

	if timeRange.End.Sub(timeRange.Start) > time.Duration(maxCandleIntervals)*duration {
		return domain.Instrument{}, "", domain.TimeRange{}, app.NewErrInvalidInput(
			"range",
			fmt.Sprintf("requested range exceeds %d candle intervals", maxCandleIntervals),
		)
	}

	return domain.Instrument{Venue: venue, Symbol: symbol, AssetClass: assetClass}, timeframe, timeRange, nil
}

func validateSupportedVenue(raw string) (domain.Venue, error) {
	venue, err := domain.NewVenue(raw)
	if err != nil {
		return "", app.NewErrInvalidInput("venue", err.Error())
	}
	if venue != domain.Venue(supportedHistoricalDataVenue) {
		return "", app.NewErrInvalidInput("venue", "unsupported venue")
	}
	return venue, nil
}

func timeframeDuration(timeframe domain.Timeframe) (time.Duration, error) {
	switch timeframe {
	case domain.Timeframe1m:
		return time.Minute, nil
	case domain.Timeframe5m:
		return 5 * time.Minute, nil
	case domain.Timeframe15m:
		return 15 * time.Minute, nil
	case domain.Timeframe1h:
		return time.Hour, nil
	case domain.Timeframe4h:
		return 4 * time.Hour, nil
	case domain.Timeframe1d:
		return 24 * time.Hour, nil
	default:
		return 0, errors.New("unsupported timeframe")
	}
}

func mapReplayCandle(item data.ReplayCandle) (models.DataCandle, error) {
	if item.Identity > math.MaxInt64 {
		return models.DataCandle{}, errors.New("replay candle identity exceeds int64 range")
	}

	return models.DataCandle{
		IDentity:           int64(item.Identity),
		Venue:              item.Candle.Instrument.Venue.String(),
		Symbol:             item.Candle.Instrument.Symbol.String(),
		AssetClass:         item.Candle.Instrument.AssetClass.String(),
		Timeframe:          item.Candle.Timeframe.String(),
		Start:              item.Candle.TimeRange.Start,
		End:                item.Candle.TimeRange.End,
		Open:               item.Candle.Open,
		High:               item.Candle.High,
		Low:                item.Candle.Low,
		Close:              item.Candle.Close,
		Volume:             item.Candle.Volume,
		Quality:            item.Candle.Quality.String(),
		ProvenanceSource:   item.Candle.Provenance.Source,
		ProvenanceIDentity: item.Candle.Provenance.RecordID,
	}, nil
}

func mapRawPayloadMetadata(item data.RawPayloadMetadata) models.RawPayloadMetadata {
	response := models.RawPayloadMetadata{
		ID:                 item.ID,
		IngestionRunID:     item.IngestionRunID,
		Source:             item.Source,
		Venue:              item.Venue.String(),
		Endpoint:           item.Endpoint,
		RequestType:        item.RequestType,
		RequestPayloadHash: item.RequestPayloadHash,
		RequestAt:          item.RequestAt,
		ResponseAt:         item.ResponseAt,
		HttpStatus:         int64(item.HTTPStatus),
		ResponseBodyHash:   item.ResponseBodyHash,
		PayloadBodyRef:     item.PayloadBodyRef,
		EntityHint:         item.EntityHint,
		Timeframe:          item.Timeframe.String(),
		ReceivedAt:         item.ReceivedAt,
	}

	if item.Instrument != nil {
		response.Symbol = item.Instrument.Symbol.String()
		response.AssetClass = item.Instrument.AssetClass.String()
	}
	if item.TimeRange != nil {
		response.Start = &item.TimeRange.Start
		response.End = &item.TimeRange.End
	}

	return response
}

func mapDataReadError(err error, resourceID string) error {
	switch {
	case errors.Is(err, data.ErrValidation):
		return app.NewErrInvalidInput("request", err.Error())
	case errors.Is(err, data.ErrInstrumentNotFound):
		return app.NewErrNotFound("instrument", resourceID)
	case errors.Is(err, data.ErrRawPayloadNotFound):
		return app.NewErrNotFound("raw payload", resourceID)
	default:
		return err
	}
}
