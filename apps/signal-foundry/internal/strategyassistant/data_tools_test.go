package strategyassistant

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	app "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	"github.com/gemyago/signal-foundry/runtime/agent"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeReadServiceCall struct {
	instrument domain.Instrument
	timeframe  domain.Timeframe
	timeRange  domain.TimeRange
}

type fakeReadService struct {
	listCalls   []data.CandleAvailabilityListQuery
	replayCalls []fakeReadServiceCall
	listFunc    func(context.Context, data.CandleAvailabilityListQuery) (data.CandleAvailabilityListResult, error)
	replayFunc  func(context.Context, domain.Instrument, domain.Timeframe, domain.TimeRange) ([]data.ReplayCandle, error)
}

func (f *fakeReadService) ListCandleAvailability(
	ctx context.Context,
	query data.CandleAvailabilityListQuery,
) (data.CandleAvailabilityListResult, error) {
	f.listCalls = append(f.listCalls, query)
	if f.listFunc == nil {
		return data.CandleAvailabilityListResult{}, errors.New("unexpected list call")
	}
	return f.listFunc(ctx, query)
}

func (f *fakeReadService) ReplayCandles(
	ctx context.Context,
	instrument domain.Instrument,
	timeframe domain.Timeframe,
	timeRange domain.TimeRange,
) ([]data.ReplayCandle, error) {
	f.replayCalls = append(f.replayCalls, fakeReadServiceCall{
		instrument: instrument,
		timeframe:  timeframe,
		timeRange:  timeRange,
	})
	if f.replayFunc == nil {
		return nil, errors.New("unexpected replay call")
	}
	return f.replayFunc(ctx, instrument, timeframe, timeRange)
}

type fakeLineageService struct {
	calls    []data.CandleLinkedRawPayloadsQuery
	listFunc func(context.Context, data.CandleLinkedRawPayloadsQuery) ([]data.RawPayloadMetadata, error)
}

func (f *fakeLineageService) ListCandleLinkedRawPayloadMetadata(
	ctx context.Context,
	query data.CandleLinkedRawPayloadsQuery,
) ([]data.RawPayloadMetadata, error) {
	f.calls = append(f.calls, query)
	if f.listFunc == nil {
		return nil, errors.New("unexpected lineage call")
	}
	return f.listFunc(ctx, query)
}

func TestDataTools(t *testing.T) {
	makeInstrument := func(t *testing.T, venue, symbol string, assetClass domain.AssetClass) domain.Instrument {
		t.Helper()
		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      domain.Venue(venue),
			Symbol:     domain.Symbol(symbol),
			AssetClass: assetClass,
			Active:     true,
		})
		require.NoError(t, err)
		return instrument
	}

	makeCandle := func(
		t *testing.T,
		instrument domain.Instrument,
		timeframe domain.Timeframe,
		start time.Time,
		identity uint64,
	) data.ReplayCandle {
		t.Helper()
		provenance, err := domain.NewSourceProvenance("source-a", fmt.Sprintf("prov-%d", identity))
		require.NoError(t, err)
		candle, err := domain.NewCandle(domain.CandleParams{
			Instrument: instrument,
			Timeframe:  timeframe,
			TimeRange:  domain.TimeRange{Start: start, End: start.Add(time.Minute)},
			Open:       float64(identity),
			High:       float64(identity) + 1,
			Low:        float64(identity) - 1,
			Close:      float64(identity) + 0.5,
			Volume:     float64(identity) * 10,
			Quality:    domain.DataQualityValidated,
			Provenance: provenance,
		})
		require.NoError(t, err)
		return data.ReplayCandle{Identity: identity, Candle: candle}
	}

	t.Run("list candle availability maps compact rows and exact filters", func(t *testing.T) {
		readSvc := &fakeReadService{}
		selectionStart := time.Now().UTC().Truncate(time.Second)
		selectionEnd := selectionStart.Add(24 * time.Hour)
		readSvc.listFunc = func(_ context.Context, query data.CandleAvailabilityListQuery) (data.CandleAvailabilityListResult, error) {
			assert.Equal(t, domain.Venue(strategyAssistantSupportedDataVenue), query.Venue)
			assert.Equal(t, domain.Symbol("BTCUSD"), query.Symbol)
			assert.Equal(t, domain.AssetClassCrypto, query.AssetClass)
			assert.Equal(t, availabilityPageSize, query.Limit)
			assert.Empty(t, query.Cursor)

			return data.CandleAvailabilityListResult{
				Items: []data.CandleAvailabilityItem{{
					Venue:      domain.Venue(strategyAssistantSupportedDataVenue),
					Symbol:     domain.Symbol("BTCUSD"),
					AssetClass: domain.AssetClassCrypto,
					Timeframes: []data.CandleAvailabilityTimeframeSummary{
						{Timeframe: domain.Timeframe1h, Count: 12, StartAt: selectionStart, EndAt: selectionEnd},
						{Timeframe: domain.Timeframe1m, Count: 720, StartAt: selectionStart, EndAt: selectionEnd},
					},
				}},
				DefaultSelection: &data.CandleAvailabilityDefaultSelection{
					Venue:      domain.Venue(strategyAssistantSupportedDataVenue),
					Symbol:     domain.Symbol("BTCUSD"),
					AssetClass: domain.AssetClassCrypto,
					Timeframe:  domain.Timeframe1m,
					StartAt:    selectionStart,
					EndAt:      selectionEnd,
				},
			}, nil
		}

		response, err := newListCandleAvailabilityTool(RegisterDeps{DataRead: readSvc}).Handler(
			nil,
			ListCandleAvailabilityRequest{
				Venue:      strategyAssistantSupportedDataVenue,
				Symbol:     "BTCUSD",
				AssetClass: domain.AssetClassCrypto.String(),
			},
		)
		require.NoError(t, err)
		require.Len(t, response.Items, 1)
		assert.Nil(t, response.Error)
		assert.Nil(t, response.Truncation)
		assert.Equal(t, []CandleAvailabilityTimeframeSummary{
			{Timeframe: domain.Timeframe1m.String(), Count: 720, Start: selectionStart, End: selectionEnd},
			{Timeframe: domain.Timeframe1h.String(), Count: 12, Start: selectionStart, End: selectionEnd},
		}, response.Items[0].Timeframes)
		assert.Equal(t, &CandleAvailabilityDefaultSelection{
			Timeframe: domain.Timeframe1m.String(),
			Reason:    defaultSelectionReason,
		}, response.Items[0].DefaultSelection)
		assert.Len(t, readSvc.listCalls, 1)
		assert.Empty(t, readSvc.replayCalls)
	})

	t.Run("list candle availability paginates and keeps truncation honest after skipped rows", func(t *testing.T) {
		readSvc := &fakeReadService{}
		nextCursor := base64.RawURLEncoding.EncodeToString([]byte(strings.Join([]string{
			time.Now().UTC().Format(time.RFC3339Nano),
			strategyAssistantSupportedDataVenue,
			"P1-199",
			domain.AssetClassCrypto.String(),
		}, "\n")))
		pageOne := make([]data.CandleAvailabilityItem, 200)
		pageTwo := make([]data.CandleAvailabilityItem, 10)
		for i := range pageOne {
			pageOne[i] = data.CandleAvailabilityItem{
				Venue:      domain.Venue(strategyAssistantSupportedDataVenue),
				Symbol:     domain.Symbol(fmt.Sprintf("P1-%03d", i)),
				AssetClass: domain.AssetClassCrypto,
			}
		}
		for i := range pageTwo {
			pageTwo[i] = data.CandleAvailabilityItem{
				Venue:      domain.Venue(strategyAssistantSupportedDataVenue),
				Symbol:     domain.Symbol(fmt.Sprintf("P2-%03d", i)),
				AssetClass: domain.AssetClassCrypto,
			}
		}
		readSvc.listFunc = func(_ context.Context, query data.CandleAvailabilityListQuery) (data.CandleAvailabilityListResult, error) {
			switch query.Cursor {
			case "":
				return data.CandleAvailabilityListResult{Items: pageOne, NextCursor: nextCursor}, nil
			case nextCursor:
				return data.CandleAvailabilityListResult{Items: pageTwo}, nil
			default:
				return data.CandleAvailabilityListResult{}, fmt.Errorf("unexpected cursor %q", query.Cursor)
			}
		}

		response, err := newListCandleAvailabilityTool(RegisterDeps{DataRead: readSvc}).Handler(
			nil,
			ListCandleAvailabilityRequest{Offset: 205, Limit: 10_000},
		)
		require.NoError(t, err)
		require.Len(t, response.Items, 5)
		assert.Nil(t, response.Error)
		assert.Nil(t, response.Truncation)
		assert.Equal(t, "P2-005", response.Items[0].Symbol)
		assert.Equal(t, "P2-009", response.Items[4].Symbol)
		assert.Len(t, readSvc.listCalls, 2)
	})

	t.Run("list candle availability caps limit and reports remaining rows", func(t *testing.T) {
		readSvc := &fakeReadService{}
		items := make([]data.CandleAvailabilityItem, maxAvailabilityToolLimit+1)
		for i := range items {
			items[i] = data.CandleAvailabilityItem{
				Venue:      domain.Venue(strategyAssistantSupportedDataVenue),
				Symbol:     domain.Symbol(fmt.Sprintf("BTC-%03d", i)),
				AssetClass: domain.AssetClassCrypto,
			}
		}
		readSvc.listFunc = func(_ context.Context, _ data.CandleAvailabilityListQuery) (data.CandleAvailabilityListResult, error) {
			return data.CandleAvailabilityListResult{Items: items}, nil
		}

		response, err := newListCandleAvailabilityTool(RegisterDeps{DataRead: readSvc}).Handler(
			nil,
			ListCandleAvailabilityRequest{Limit: maxAvailabilityToolLimit + 500},
		)
		require.NoError(t, err)
		require.Len(t, response.Items, maxAvailabilityToolLimit)
		require.NotNil(t, response.Truncation)
		assert.True(t, response.Truncation.IsTruncated)
		assert.Equal(t, maxAvailabilityToolLimit, response.Truncation.Limit)
		assert.Equal(t, maxAvailabilityToolLimit, response.Truncation.Returned)
		assert.Equal(t, "100", response.Truncation.NextCursor)
		assert.Contains(t, response.NextStepHint, "offset=100")
	})

	t.Run("list candle availability returns empty collection without mutation", func(t *testing.T) {
		readSvc := &fakeReadService{}
		readSvc.listFunc = func(_ context.Context, _ data.CandleAvailabilityListQuery) (data.CandleAvailabilityListResult, error) {
			return data.CandleAvailabilityListResult{}, nil
		}

		response, err := newListCandleAvailabilityTool(RegisterDeps{DataRead: readSvc}).Handler(
			nil,
			ListCandleAvailabilityRequest{},
		)
		require.NoError(t, err)
		assert.Empty(t, response.Items)
		assert.Nil(t, response.Error)
		assert.Len(t, readSvc.listCalls, 1)
		assert.Empty(t, readSvc.replayCalls)
	})

	t.Run("data tools return placeholder results when runtime deps are absent", func(t *testing.T) {
		availability, err := newListCandleAvailabilityTool(RegisterDeps{}).Handler(
			nil,
			ListCandleAvailabilityRequest{},
		)
		require.NoError(t, err)
		require.NotNil(t, availability.Error)
		assert.Equal(t, toolErrorCodeNotReady, availability.Error.Code)

		candles, err := newGetCandlesTool(RegisterDeps{}).Handler(
			nil,
			GetCandlesRequest{},
		)
		require.NoError(t, err)
		require.NotNil(t, candles.Error)
		assert.Equal(t, toolErrorCodeNotReady, candles.Error.Code)

		evidence, err := newGetCandleEvidenceTool(RegisterDeps{}).Handler(
			nil,
			GetCandleEvidenceRequest{},
		)
		require.NoError(t, err)
		require.NotNil(t, evidence.Error)
		assert.Equal(t, toolErrorCodeNotReady, evidence.Error.Code)
	})

	t.Run("list candle availability rejects invalid pagination and filters", func(t *testing.T) {
		tool := newListCandleAvailabilityTool(RegisterDeps{DataRead: &fakeReadService{}})

		cases := []struct {
			name  string
			input ListCandleAvailabilityRequest
			field string
		}{
			{name: "negative limit", input: ListCandleAvailabilityRequest{Limit: -1}, field: "limit"},
			{name: "negative offset", input: ListCandleAvailabilityRequest{Offset: -1}, field: "offset"},
			{name: "unsupported venue", input: ListCandleAvailabilityRequest{Venue: "binance"}, field: "venue"},
			{
				name:  "invalid asset class",
				input: ListCandleAvailabilityRequest{AssetClass: "commodities"},
				field: "assetClass",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				response, err := tool.Handler(nil, tc.input)
				require.NoError(t, err)
				require.NotNil(t, response.Error)
				assert.Equal(t, toolErrorCodeValidation, response.Error.Code)
				require.Len(t, response.Error.FieldErrors, 1)
				assert.Equal(t, tc.field, response.Error.FieldErrors[0].Field)
			})
		}
	})

	t.Run("list candle availability maps service validation errors safely", func(t *testing.T) {
		readSvc := &fakeReadService{}
		readSvc.listFunc = func(_ context.Context, _ data.CandleAvailabilityListQuery) (data.CandleAvailabilityListResult, error) {
			return data.CandleAvailabilityListResult{}, fmt.Errorf("wrapped: %w", data.ErrValidation)
		}

		response, err := newListCandleAvailabilityTool(RegisterDeps{DataRead: readSvc}).Handler(
			nil,
			ListCandleAvailabilityRequest{},
		)
		require.NoError(t, err)
		require.NotNil(t, response.Error)
		assert.Equal(t, toolErrorCodeValidation, response.Error.Code)
	})

	t.Run("get candles validates exact scope and range", func(t *testing.T) {
		tool := newGetCandlesTool(RegisterDeps{DataRead: &fakeReadService{}})
		start := time.Now().UTC().Truncate(time.Minute)

		cases := []struct {
			name   string
			input  GetCandlesRequest
			field  string
			reason string
		}{
			{
				name: "unsupported venue",
				input: GetCandlesRequest{
					Venue:      "binance",
					Symbol:     "BTCUSD",
					AssetClass: "crypto",
					Timeframe:  "1m",
					Start:      start,
					End:        start.Add(time.Minute),
				},
				field:  "venue",
				reason: "unsupported venue",
			},
			{
				name: "missing symbol",
				input: GetCandlesRequest{
					Venue:      strategyAssistantSupportedDataVenue,
					AssetClass: "crypto",
					Timeframe:  "1m",
					Start:      start,
					End:        start.Add(time.Minute),
				},
				field:  "symbol",
				reason: "symbol is required",
			},
			{
				name: "invalid range",
				input: GetCandlesRequest{
					Venue:      strategyAssistantSupportedDataVenue,
					Symbol:     "BTCUSD",
					AssetClass: "crypto",
					Timeframe:  "1m",
					Start:      start,
					End:        start,
				},
				field:  "range",
				reason: "time range end must be after start",
			},
			{
				name: "range too large",
				input: GetCandlesRequest{
					Venue:      strategyAssistantSupportedDataVenue,
					Symbol:     "BTCUSD",
					AssetClass: "crypto",
					Timeframe:  "1m",
					Start:      start,
					End:        start.Add(time.Duration(maxCandleIntervals+1) * time.Minute),
				},
				field:  "range",
				reason: fmt.Sprintf("requested range exceeds %d candle intervals", maxCandleIntervals),
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				response, err := tool.Handler(nil, tc.input)
				require.NoError(t, err)
				require.NotNil(t, response.Error)
				assert.Equal(t, toolErrorCodeValidation, response.Error.Code)
				require.Len(t, response.Error.FieldErrors, 1)
				assert.Equal(t, tc.field, response.Error.FieldErrors[0].Field)
				assert.Contains(t, response.Error.FieldErrors[0].Message, tc.reason)
			})
		}
	})

	t.Run("get candles sorts maps caps and truncates honestly", func(t *testing.T) {
		start := time.Now().UTC().Truncate(time.Minute)
		instrument := makeInstrument(t, strategyAssistantSupportedDataVenue, "BTCUSD", domain.AssetClassCrypto)
		readSvc := &fakeReadService{}
		readSvc.replayFunc = func(
			_ context.Context,
			actualInstrument domain.Instrument,
			actualTimeframe domain.Timeframe,
			actualRange domain.TimeRange,
		) ([]data.ReplayCandle, error) {
			assert.Equal(t, instrument, actualInstrument)
			assert.Equal(t, domain.Timeframe1m, actualTimeframe)
			assert.Equal(t, start, actualRange.Start)
			assert.Equal(t, start.Add(600*time.Minute), actualRange.End)

			rows := make([]data.ReplayCandle, 0, maxCandlesToolRows+1)
			for i := maxCandlesToolRows; i >= 0; i-- {
				rows = append(rows, makeCandle(
					t,
					instrument,
					domain.Timeframe1m,
					start.Add(time.Duration(i)*time.Minute),
					uint64(i+1),
				))
			}
			return rows, nil
		}

		response, err := newGetCandlesTool(RegisterDeps{DataRead: readSvc}).Handler(
			nil,
			GetCandlesRequest{
				Venue:      strategyAssistantSupportedDataVenue,
				Symbol:     "BTCUSD",
				AssetClass: domain.AssetClassCrypto.String(),
				Timeframe:  domain.Timeframe1m.String(),
				Start:      start,
				End:        start.Add(600 * time.Minute),
			},
		)
		require.NoError(t, err)
		require.Len(t, response.Candles, maxCandlesToolRows)
		assert.Equal(t, "1", response.Candles[0].CandleID)
		assert.Equal(t, start, response.Candles[0].OpenTime)
		assert.Equal(t, "500", response.Candles[maxCandlesToolRows-1].CandleID)
		assert.Equal(t, start.Add(499*time.Minute), response.Candles[maxCandlesToolRows-1].OpenTime)
		require.NotNil(t, response.Truncation)
		assert.True(t, response.Truncation.IsTruncated)
		require.NotNil(t, response.Truncation.NextRangeStart)
		assert.Equal(t, start.Add(500*time.Minute), *response.Truncation.NextRangeStart)
		assert.Contains(t, response.NextStepHint, start.Add(500*time.Minute).Format(time.RFC3339))
		assert.Len(t, readSvc.replayCalls, 1)
		assert.Empty(t, readSvc.listCalls)
	})

	t.Run("get candles returns empty instead of synthesizing rows", func(t *testing.T) {
		readSvc := &fakeReadService{}
		readSvc.replayFunc = func(_ context.Context, _ domain.Instrument, _ domain.Timeframe, _ domain.TimeRange) ([]data.ReplayCandle, error) {
			return []data.ReplayCandle{}, nil
		}
		start := time.Now().UTC().Truncate(time.Minute)

		response, err := newGetCandlesTool(RegisterDeps{DataRead: readSvc}).Handler(
			nil,
			GetCandlesRequest{
				Venue:      strategyAssistantSupportedDataVenue,
				Symbol:     "BTCUSD",
				AssetClass: domain.AssetClassCrypto.String(),
				Timeframe:  domain.Timeframe1m.String(),
				Start:      start,
				End:        start.Add(time.Minute),
			},
		)
		require.NoError(t, err)
		assert.Empty(t, response.Candles)
		assert.Nil(t, response.Error)
		assert.Nil(t, response.Truncation)
	})

	t.Run("get candles maps not found and internal service errors safely", func(t *testing.T) {
		start := time.Now().UTC().Truncate(time.Minute)
		baseReq := GetCandlesRequest{
			Venue:      strategyAssistantSupportedDataVenue,
			Symbol:     "BTCUSD",
			AssetClass: domain.AssetClassCrypto.String(),
			Timeframe:  domain.Timeframe1m.String(),
			Start:      start,
			End:        start.Add(time.Minute),
		}

		notFoundSvc := &fakeReadService{}
		notFoundSvc.replayFunc = func(_ context.Context, _ domain.Instrument, _ domain.Timeframe, _ domain.TimeRange) ([]data.ReplayCandle, error) {
			return nil, fmt.Errorf("wrapped: %w", data.ErrInstrumentNotFound)
		}
		notFoundResponse, err := newGetCandlesTool(RegisterDeps{DataRead: notFoundSvc}).Handler(
			nil,
			baseReq,
		)
		require.NoError(t, err)
		require.NotNil(t, notFoundResponse.Error)
		assert.Equal(t, toolErrorCodeNotFound, notFoundResponse.Error.Code)

		internalSvc := &fakeReadService{}
		internalSvc.replayFunc = func(_ context.Context, _ domain.Instrument, _ domain.Timeframe, _ domain.TimeRange) ([]data.ReplayCandle, error) {
			return nil, errors.New("gorm: SQLSTATE 42P01 raw secret")
		}
		internalResponse, err := newGetCandlesTool(RegisterDeps{DataRead: internalSvc}).Handler(
			nil,
			baseReq,
		)
		require.NoError(t, err)
		require.NotNil(t, internalResponse.Error)
		assert.Equal(t, toolErrorCodeInternal, internalResponse.Error.Code)
		assert.NotContains(t, strings.ToLower(internalResponse.Error.Message), "sql")
	})

	t.Run("get candle evidence validates required provenance and derived time range", func(t *testing.T) {
		lineageSvc := &fakeLineageService{}
		tool := newGetCandleEvidenceTool(RegisterDeps{DataLineage: lineageSvc})
		openTime := time.Now().UTC().Truncate(time.Minute)

		missingResponse, err := tool.Handler(
			nil,
			GetCandleEvidenceRequest{
				Venue:      strategyAssistantSupportedDataVenue,
				Symbol:     "BTCUSD",
				AssetClass: domain.AssetClassCrypto.String(),
				Timeframe:  domain.Timeframe1m.String(),
				OpenTime:   openTime,
			},
		)
		require.NoError(t, err)
		require.NotNil(t, missingResponse.Error)
		require.Len(t, missingResponse.Error.FieldErrors, 1)
		assert.Equal(t, "provenanceSource", missingResponse.Error.FieldErrors[0].Field)
		assert.Empty(t, lineageSvc.calls)

		lineageSvc.listFunc = func(_ context.Context, query data.CandleLinkedRawPayloadsQuery) ([]data.RawPayloadMetadata, error) {
			assert.Equal(t, openTime, query.TimeRange.Start)
			assert.Equal(t, openTime.Add(time.Minute), query.TimeRange.End)
			assert.Equal(t, "prov-source", query.ProvenanceSource)
			assert.Equal(t, "prov-id", query.ProvenanceIdentity)
			return []data.RawPayloadMetadata{}, nil
		}

		successResponse, err := tool.Handler(
			nil,
			GetCandleEvidenceRequest{
				Venue:            strategyAssistantSupportedDataVenue,
				Symbol:           "BTCUSD",
				AssetClass:       domain.AssetClassCrypto.String(),
				Timeframe:        domain.Timeframe1m.String(),
				OpenTime:         openTime,
				ProvenanceSource: "prov-source",
				ProvenanceID:     "prov-id",
			},
		)
		require.NoError(t, err)
		assert.Empty(t, successResponse.Evidence)
		assert.Nil(t, successResponse.Error)
		require.Len(t, lineageSvc.calls, 1)
	})

	t.Run("get candle evidence maps bounded metadata without raw payload bytes", func(t *testing.T) {
		lineageSvc := &fakeLineageService{}
		openTime := time.Now().UTC().Truncate(time.Minute)
		items := make([]data.RawPayloadMetadata, maxCandleEvidenceToolLimit+1)
		for i := range items {
			items[i] = data.RawPayloadMetadata{
				ID:             fmt.Sprintf("raw-%03d", i),
				Source:         "hyperliquid",
				Venue:          domain.Venue(strategyAssistantSupportedDataVenue),
				Endpoint:       "/candles",
				RequestType:    "GET",
				ResponseAt:     openTime.Add(time.Duration(i) * time.Second),
				ReceivedAt:     openTime.Add(time.Duration(i) * time.Second),
				PayloadBodyRef: "blob-ref-should-not-leak",
			}
		}
		lineageSvc.listFunc = func(_ context.Context, _ data.CandleLinkedRawPayloadsQuery) ([]data.RawPayloadMetadata, error) {
			return items, nil
		}

		response, err := newGetCandleEvidenceTool(RegisterDeps{DataLineage: lineageSvc}).Handler(
			nil,
			GetCandleEvidenceRequest{
				Venue:            strategyAssistantSupportedDataVenue,
				Symbol:           "BTCUSD",
				AssetClass:       domain.AssetClassCrypto.String(),
				Timeframe:        domain.Timeframe1m.String(),
				OpenTime:         openTime,
				ProvenanceSource: "prov-source",
				ProvenanceID:     "prov-id",
				Limit:            maxCandleEvidenceToolLimit + 50,
			},
		)
		require.NoError(t, err)
		require.Len(t, response.Evidence, maxCandleEvidenceToolLimit)
		require.NotNil(t, response.Truncation)
		assert.Equal(t, "raw-000", response.Evidence[0].RawPayloadID)
		assert.Equal(t, "hyperliquid", response.Evidence[0].SourceType)
		assert.Equal(t, "GET /candles", response.Evidence[0].Reference)
		assert.NotContains(t, fmt.Sprintf("%+v", response.Evidence[0]), "blob-ref-should-not-leak")
		assert.True(t, response.Truncation.IsTruncated)
		assert.Equal(t, maxCandleEvidenceToolLimit, response.Truncation.Limit)
		assert.Equal(t, maxCandleEvidenceToolLimit, response.Truncation.Returned)
		assert.Equal(t, "100", response.Truncation.NextCursor)
		assert.Contains(t, response.NextStepHint, "offset=100")
	})

	t.Run("get candle evidence validates pagination and maps service errors safely", func(t *testing.T) {
		tool := newGetCandleEvidenceTool(RegisterDeps{DataLineage: &fakeLineageService{}})
		openTime := time.Now().UTC().Truncate(time.Minute)

		invalidResponse, err := tool.Handler(
			nil,
			GetCandleEvidenceRequest{
				Venue:            strategyAssistantSupportedDataVenue,
				Symbol:           "BTCUSD",
				AssetClass:       domain.AssetClassCrypto.String(),
				Timeframe:        domain.Timeframe1m.String(),
				OpenTime:         openTime,
				ProvenanceSource: "prov-source",
				ProvenanceID:     "prov-id",
				Offset:           -1,
			},
		)
		require.NoError(t, err)
		require.NotNil(t, invalidResponse.Error)
		assert.Equal(t, "offset", invalidResponse.Error.FieldErrors[0].Field)

		validationSvc := &fakeLineageService{}
		validationSvc.listFunc = func(_ context.Context, _ data.CandleLinkedRawPayloadsQuery) ([]data.RawPayloadMetadata, error) {
			return nil, fmt.Errorf("wrapped: %w", data.ErrValidation)
		}
		validationResponse, err := newGetCandleEvidenceTool(RegisterDeps{DataLineage: validationSvc}).Handler(
			nil,
			GetCandleEvidenceRequest{
				Venue:            strategyAssistantSupportedDataVenue,
				Symbol:           "BTCUSD",
				AssetClass:       domain.AssetClassCrypto.String(),
				Timeframe:        domain.Timeframe1m.String(),
				OpenTime:         openTime,
				ProvenanceSource: "prov-source",
				ProvenanceID:     "prov-id",
			},
		)
		require.NoError(t, err)
		require.NotNil(t, validationResponse.Error)
		assert.Equal(t, toolErrorCodeValidation, validationResponse.Error.Code)
	})

	t.Run("helper functions map fallback branches deterministically", func(t *testing.T) {
		boom := errors.New("boom")
		assert.Equal(t, boom, mapDataToolError(boom, "id"))
		rawPayloadErr := mapDataToolError(fmt.Errorf("wrapped: %w", data.ErrRawPayloadNotFound), "raw-1")
		rawPayloadToolErr := toolErrorFrom(rawPayloadErr)
		require.NotNil(t, rawPayloadToolErr)
		assert.Equal(t, toolErrorCodeNotFound, rawPayloadToolErr.Code)

		ref := compactEvidenceReference(data.RawPayloadMetadata{ID: "raw-1"})
		assert.Equal(t, "raw-1", ref)

		captured := metadataCapturedAt(data.RawPayloadMetadata{ReceivedAt: time.Unix(10, 0).UTC()})
		assert.Equal(t, time.Unix(10, 0).UTC(), captured)

		_, err := normalizeLimit(-1, 2, 3)
		require.Error(t, err)
		_, err = normalizeOffset(-1)
		require.Error(t, err)

		_, err = timeframeDuration(domain.Timeframe("bad"))
		require.Error(t, err)

		toolErr := toolErrorFrom(app.NewErrInvalidInput("field", "because"))
		require.NotNil(t, toolErr)
		assert.Equal(t, toolErrorCodeValidation, toolErr.Code)

		validationErr := mapCandleEvidenceValidationError(
			errors.New("candle raw payload query provenance identity is required"),
		)
		toolErr = toolErrorFrom(validationErr)
		require.NotNil(t, toolErr)
		require.Len(t, toolErr.FieldErrors, 1)
		assert.Equal(t, "provenanceId", toolErr.FieldErrors[0].Field)

		symbolErr := mapCandleEvidenceValidationError(errors.New("candle raw payload query symbol is required"))
		toolErr = toolErrorFrom(symbolErr)
		require.NotNil(t, toolErr)
		assert.Equal(t, "symbol", toolErr.FieldErrors[0].Field)

		defaultErr := mapCandleEvidenceValidationError(errors.New("other validation"))
		toolErr = toolErrorFrom(defaultErr)
		require.NotNil(t, toolErr)
		assert.Equal(t, "request", toolErr.FieldErrors[0].Field)

		for _, timeframe := range []domain.Timeframe{
			domain.Timeframe5m,
			domain.Timeframe15m,
			domain.Timeframe1h,
			domain.Timeframe4h,
			domain.Timeframe1d,
		} {
			_, durationErr := timeframeDuration(timeframe)
			require.NoError(t, durationErr)
		}

		assert.NotNil(t, toolContextContext(nil))
		type testContextKey string
		baseContext := context.WithValue(context.Background(), testContextKey("key"), "value")
		assert.Equal(
			t,
			"value",
			toolContextContext(&agent.ToolContext{Context: baseContext}).Value(testContextKey("key")),
		)
	})

	t.Run("build candle evidence query rejects zero open time", func(t *testing.T) {
		_, err := buildCandleEvidenceQuery(GetCandleEvidenceRequest{
			Venue:            strategyAssistantSupportedDataVenue,
			Symbol:           "BTCUSD",
			AssetClass:       domain.AssetClassCrypto.String(),
			Timeframe:        domain.Timeframe1m.String(),
			ProvenanceSource: "prov-source",
			ProvenanceID:     "prov-id",
		})
		require.Error(t, err)
		toolErr := toolErrorFrom(err)
		require.NotNil(t, toolErr)
		assert.Equal(t, "openTime", toolErr.FieldErrors[0].Field)
	})

	t.Run("evidence sort falls back to id when captured times match", func(t *testing.T) {
		lineageSvc := &fakeLineageService{}
		openTime := time.Now().UTC().Truncate(time.Minute)
		lineageSvc.listFunc = func(_ context.Context, _ data.CandleLinkedRawPayloadsQuery) ([]data.RawPayloadMetadata, error) {
			return []data.RawPayloadMetadata{
				{
					ID:         "raw-b",
					Source:     "hyperliquid",
					Venue:      domain.Venue(strategyAssistantSupportedDataVenue),
					ReceivedAt: openTime,
				},
				{
					ID:         "raw-a",
					Source:     "hyperliquid",
					Venue:      domain.Venue(strategyAssistantSupportedDataVenue),
					ReceivedAt: openTime,
				},
			}, nil
		}

		response, err := newGetCandleEvidenceTool(RegisterDeps{DataLineage: lineageSvc}).Handler(
			nil,
			GetCandleEvidenceRequest{
				Venue:            strategyAssistantSupportedDataVenue,
				Symbol:           "BTCUSD",
				AssetClass:       domain.AssetClassCrypto.String(),
				Timeframe:        domain.Timeframe1m.String(),
				OpenTime:         openTime,
				ProvenanceSource: "prov-source",
				ProvenanceID:     "prov-id",
			},
		)
		require.NoError(t, err)
		require.Len(t, response.Evidence, 2)
		assert.Equal(t, "raw-a", response.Evidence[0].RawPayloadID)
		assert.Equal(t, "raw-b", response.Evidence[1].RawPayloadID)
	})

	t.Run("candle sort falls back to end time then identity", func(t *testing.T) {
		instrument := makeInstrument(t, strategyAssistantSupportedDataVenue, "BTCUSD", domain.AssetClassCrypto)
		start := time.Now().UTC().Truncate(time.Minute)
		provenance, err := domain.NewSourceProvenance("source-a", "prov-a")
		require.NoError(t, err)

		makeExactCandle := func(t *testing.T, end time.Time, identity uint64) data.ReplayCandle {
			t.Helper()
			candle, candleErr := domain.NewCandle(domain.CandleParams{
				Instrument: instrument,
				Timeframe:  domain.Timeframe1m,
				TimeRange:  domain.TimeRange{Start: start, End: end},
				Open:       1,
				High:       2,
				Low:        0,
				Close:      1,
				Volume:     1,
				Quality:    domain.DataQualityValidated,
				Provenance: provenance,
			})
			require.NoError(t, candleErr)
			return data.ReplayCandle{Identity: identity, Candle: candle}
		}

		readSvc := &fakeReadService{}
		readSvc.replayFunc = func(_ context.Context, _ domain.Instrument, _ domain.Timeframe, _ domain.TimeRange) ([]data.ReplayCandle, error) {
			return []data.ReplayCandle{
				makeExactCandle(t, start.Add(2*time.Minute), 2),
				makeExactCandle(t, start.Add(2*time.Minute), 1),
				makeExactCandle(t, start.Add(time.Minute), 3),
			}, nil
		}

		response, err := newGetCandlesTool(RegisterDeps{DataRead: readSvc}).Handler(
			nil,
			GetCandlesRequest{
				Venue:      strategyAssistantSupportedDataVenue,
				Symbol:     "BTCUSD",
				AssetClass: domain.AssetClassCrypto.String(),
				Timeframe:  domain.Timeframe1m.String(),
				Start:      start,
				End:        start.Add(3 * time.Minute),
			},
		)
		require.NoError(t, err)
		require.Len(t, response.Candles, 3)
		assert.Equal(t, []string{"3", "1", "2"}, []string{
			response.Candles[0].CandleID,
			response.Candles[1].CandleID,
			response.Candles[2].CandleID,
		})
	})
}
