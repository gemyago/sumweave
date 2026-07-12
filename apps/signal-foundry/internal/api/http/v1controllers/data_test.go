package v1controllers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/middleware"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/server"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/v1routes/models"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type replayReadServiceStub struct {
	listCandleAvailabilityFunc func(
		ctx context.Context,
		query data.CandleAvailabilityListQuery,
	) (data.CandleAvailabilityListResult, error)
	replayCandlesFunc func(
		ctx context.Context,
		instrument domain.Instrument,
		timeframe domain.Timeframe,
		timeRange domain.TimeRange,
	) ([]data.ReplayCandle, error)
	listAvailabilityCalls int
	lastAvailabilityQuery data.CandleAvailabilityListQuery
	calls                 int
	lastInstrument        domain.Instrument
	lastTimeframe         domain.Timeframe
	lastTimeRange         domain.TimeRange
}

func (s *replayReadServiceStub) ListCandleAvailability(
	ctx context.Context,
	query data.CandleAvailabilityListQuery,
) (data.CandleAvailabilityListResult, error) {
	s.listAvailabilityCalls++
	s.lastAvailabilityQuery = query
	if s.listCandleAvailabilityFunc == nil {
		return data.CandleAvailabilityListResult{}, errors.New("unexpected ListCandleAvailability call")
	}
	return s.listCandleAvailabilityFunc(ctx, query)
}

func (s *replayReadServiceStub) ReplayCandles(
	ctx context.Context,
	instrument domain.Instrument,
	timeframe domain.Timeframe,
	timeRange domain.TimeRange,
) ([]data.ReplayCandle, error) {
	s.calls++
	s.lastInstrument = instrument
	s.lastTimeframe = timeframe
	s.lastTimeRange = timeRange
	if s.replayCandlesFunc == nil {
		return nil, errors.New("unexpected ReplayCandles call")
	}
	return s.replayCandlesFunc(ctx, instrument, timeframe, timeRange)
}

type lineageBrowserServiceStub struct {
	listRawPayloadMetadataFunc func(
		ctx context.Context,
		query data.RawPayloadMetadataListQuery,
	) (data.RawPayloadMetadataListResult, error)
	getRawPayloadDetailFunc func(ctx context.Context, rawPayloadID string) (data.RawPayloadDetail, error)
	listCandleLinkedFunc    func(ctx context.Context, query data.CandleLinkedRawPayloadsQuery) ([]data.RawPayloadMetadata, error)
	listCalls               int
	detailCalls             int
	linkedCalls             int
	lastListQuery           data.RawPayloadMetadataListQuery
	lastDetailID            string
	lastLinkedQuery         data.CandleLinkedRawPayloadsQuery
}

func (s *lineageBrowserServiceStub) ListRawPayloadMetadata(
	ctx context.Context,
	query data.RawPayloadMetadataListQuery,
) (data.RawPayloadMetadataListResult, error) {
	s.listCalls++
	s.lastListQuery = query
	if s.listRawPayloadMetadataFunc == nil {
		return data.RawPayloadMetadataListResult{}, errors.New("unexpected ListRawPayloadMetadata call")
	}
	return s.listRawPayloadMetadataFunc(ctx, query)
}

func (s *lineageBrowserServiceStub) GetRawPayloadDetail(
	ctx context.Context,
	rawPayloadID string,
) (data.RawPayloadDetail, error) {
	s.detailCalls++
	s.lastDetailID = rawPayloadID
	if s.getRawPayloadDetailFunc == nil {
		return data.RawPayloadDetail{}, errors.New("unexpected GetRawPayloadDetail call")
	}
	return s.getRawPayloadDetailFunc(ctx, rawPayloadID)
}

func (s *lineageBrowserServiceStub) ListCandleLinkedRawPayloadMetadata(
	ctx context.Context,
	query data.CandleLinkedRawPayloadsQuery,
) ([]data.RawPayloadMetadata, error) {
	s.linkedCalls++
	s.lastLinkedQuery = query
	if s.listCandleLinkedFunc == nil {
		return nil, errors.New("unexpected ListCandleLinkedRawPayloadMetadata call")
	}
	return s.listCandleLinkedFunc(ctx, query)
}

func newDataHTTPHandler(ctrl *DataController) http.Handler {
	return server.NewTestRootHandler().RegisterDataRoutes(ctrl)
}

func TestDataController(t *testing.T) {
	fake := faker.New()
	validStart := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	validEnd := validStart.Add(time.Hour)
	validInstrument := domain.Instrument{
		Venue:      domain.Venue("hyperliquid-perps"),
		Symbol:     domain.Symbol("BTCUSD"),
		AssetClass: domain.AssetClassCrypto,
		Active:     true,
	}
	validTimeframe := domain.Timeframe1m

	makeReplayReadService := func() *replayReadServiceStub {
		return &replayReadServiceStub{}
	}
	makeLineageBrowserService := func() *lineageBrowserServiceStub {
		return &lineageBrowserServiceStub{}
	}
	makeAuthMiddleware := func() middleware.AuthMiddleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") == "" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
			})
		}
	}
	newController := func(
		readSvc *replayReadServiceStub,
		lineageSvc *lineageBrowserServiceStub,
		authMw middleware.AuthMiddleware,
	) *DataController {
		return NewDataController(DataControllerDeps{
			ReadService:    readSvc,
			LineageService: lineageSvc,
			AuthMiddleware: authMw,
		})
	}
	newRequest := func(method string, target string, authenticated bool) *http.Request {
		req := httptest.NewRequest(method, target, http.NoBody)
		req = req.WithContext(t.Context())
		if authenticated {
			req.Header.Set("Authorization", "Bearer "+fake.Lorem().Word())
		}
		return req
	}
	makeCandleURL := func(params map[string]string) string {
		query := url.Values{
			"venue":      []string{validInstrument.Venue.String()},
			"symbol":     []string{validInstrument.Symbol.String()},
			"assetClass": []string{validInstrument.AssetClass.String()},
			"timeframe":  []string{validTimeframe.String()},
			"start":      []string{validStart.Format(time.RFC3339)},
			"end":        []string{validEnd.Format(time.RFC3339)},
		}
		for key, value := range params {
			if value == "" {
				query.Del(key)
				continue
			}
			query.Set(key, value)
		}
		return "/api/v1/data/candles?" + query.Encode()
	}
	makeAvailabilityURL := func(params map[string]string) string {
		query := url.Values{}
		for key, value := range params {
			if value == "" {
				query.Del(key)
				continue
			}
			query.Set(key, value)
		}
		encoded := query.Encode()
		if encoded == "" {
			return "/api/v1/data/candle-availability"
		}
		return "/api/v1/data/candle-availability?" + encoded
	}
	makeRawPayloadListURL := func(params map[string]string) string {
		query := url.Values{
			"venue": []string{validInstrument.Venue.String()},
		}
		for key, value := range params {
			if value == "" {
				query.Del(key)
				continue
			}
			query.Set(key, value)
		}
		return "/api/v1/data/raw-payloads?" + query.Encode()
	}
	makeCandleRawPayloadURL := func(params map[string]string) string {
		query := url.Values{
			"venue":              []string{validInstrument.Venue.String()},
			"symbol":             []string{validInstrument.Symbol.String()},
			"assetClass":         []string{validInstrument.AssetClass.String()},
			"timeframe":          []string{validTimeframe.String()},
			"start":              []string{validStart.Format(time.RFC3339)},
			"end":                []string{validEnd.Format(time.RFC3339)},
			"provenanceSource":   []string{"hyperliquid"},
			"provenanceIdentity": []string{fake.UUID().V4()},
		}
		for key, value := range params {
			if value == "" {
				query.Del(key)
				continue
			}
			query.Set(key, value)
		}
		return "/api/v1/data/candle-raw-payloads?" + query.Encode()
	}
	makeRawPayloadMetadata := func(id string) data.RawPayloadMetadata {
		return data.RawPayloadMetadata{
			ID:                 id,
			IngestionRunID:     fake.UUID().V4(),
			Source:             "hyperliquid",
			Venue:              domain.Venue("hyperliquid-perps"),
			Endpoint:           "/candles",
			RequestType:        "GET",
			RequestPayloadHash: fake.Lorem().Word(),
			RequestAt:          validStart.Add(-2 * time.Minute),
			ResponseAt:         validStart.Add(-time.Minute),
			HTTPStatus:         http.StatusOK,
			ResponseBodyHash:   fake.Lorem().Word(),
			PayloadBodyRef:     "payloads/" + id + ".json",
			EntityHint:         "candle",
			Instrument: &data.BatchInstrumentRef{
				Symbol:     validInstrument.Symbol,
				AssetClass: validInstrument.AssetClass,
			},
			Timeframe:  validTimeframe,
			TimeRange:  &domain.TimeRange{Start: validStart, End: validEnd},
			ReceivedAt: validStart.Add(-30 * time.Second),
		}
	}

	t.Run("all data endpoints require auth", func(t *testing.T) {
		ctrl := newController(makeReplayReadService(), makeLineageBrowserService(), makeAuthMiddleware())
		handler := newDataHTTPHandler(ctrl)

		for _, tc := range []struct {
			name string
			url  string
		}{
			{name: "candle availability", url: makeAvailabilityURL(nil)},
			{name: "candles", url: makeCandleURL(nil)},
			{name: "raw payload list", url: makeRawPayloadListURL(nil)},
			{name: "raw payload detail", url: "/api/v1/data/raw-payloads/" + fake.UUID().V4()},
			{name: "candle raw payloads", url: makeCandleRawPayloadURL(nil)},
		} {
			t.Run(tc.name, func(t *testing.T) {
				resp := httptest.NewRecorder()
				handler.ServeHTTP(resp, newRequest(http.MethodGet, tc.url, false))
				assert.Equal(t, http.StatusUnauthorized, resp.Code)
			})
		}
	})

	t.Run("ListDataCandleAvailability", func(t *testing.T) {
		t.Run("first page maps exact filters and includes camelCase defaultSelection", func(t *testing.T) {
			readSvc := makeReplayReadService()
			availabilityStart := validStart.Add(-24 * time.Hour)
			availabilityEnd := validEnd.Add(24 * time.Hour)
			readSvc.listCandleAvailabilityFunc = func(
				_ context.Context,
				_ data.CandleAvailabilityListQuery,
			) (data.CandleAvailabilityListResult, error) {
				return data.CandleAvailabilityListResult{
					Items: []data.CandleAvailabilityItem{
						{
							Venue:      validInstrument.Venue,
							Symbol:     validInstrument.Symbol,
							AssetClass: validInstrument.AssetClass,
							Timeframes: []data.CandleAvailabilityTimeframeSummary{
								{
									Timeframe: domain.Timeframe1m,
									StartAt:   availabilityStart,
									EndAt:     availabilityEnd,
									Count:     321,
								},
								{
									Timeframe: domain.Timeframe5m,
									StartAt:   availabilityStart.Add(5 * time.Minute),
									EndAt:     availabilityEnd,
									Count:     99,
								},
							},
							DefaultSlice: data.CandleAvailabilityDefaultSlice{
								Timeframe: domain.Timeframe5m,
								StartAt:   availabilityEnd.Add(-2 * time.Hour),
								EndAt:     availabilityEnd,
							},
						},
					},
					NextCursor: fake.Lorem().Word(),
					DefaultSelection: &data.CandleAvailabilityDefaultSelection{
						Venue:      validInstrument.Venue,
						Symbol:     validInstrument.Symbol,
						AssetClass: validInstrument.AssetClass,
						Timeframe:  domain.Timeframe5m,
						StartAt:    availabilityEnd.Add(-2 * time.Hour),
						EndAt:      availabilityEnd,
					},
				}, nil
			}

			ctrl := newController(readSvc, makeLineageBrowserService(), makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(
				http.MethodGet,
				makeAvailabilityURL(map[string]string{
					"venue":      validInstrument.Venue.String(),
					"symbol":     validInstrument.Symbol.String(),
					"assetClass": validInstrument.AssetClass.String(),
					"limit":      "12",
				}),
				true,
			))

			require.Equal(t, http.StatusOK, resp.Code)
			var body models.CandleAvailabilityListResponse
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
			require.Len(t, body.Items, 1)
			assert.Equal(t, validInstrument.Venue.String(), body.Items[0].Venue)
			assert.Equal(t, validInstrument.Symbol.String(), body.Items[0].Symbol)
			assert.Equal(t, validInstrument.AssetClass.String(), body.Items[0].AssetClass)
			require.Len(t, body.Items[0].Timeframes, 2)
			assert.Equal(t, domain.Timeframe1m.String(), body.Items[0].Timeframes[0].Timeframe)
			assert.Equal(t, availabilityStart, body.Items[0].Timeframes[0].Start)
			assert.Equal(t, availabilityEnd, body.Items[0].Timeframes[0].End)
			assert.Equal(t, int64(321), body.Items[0].Timeframes[0].Count)
			require.NotNil(t, body.Items[0].DefaultSlice)
			assert.Equal(t, domain.Timeframe5m.String(), body.Items[0].DefaultSlice.Timeframe)
			require.NotNil(t, body.DefaultSelection)
			assert.Equal(t, domain.Timeframe5m.String(), body.DefaultSelection.Timeframe)
			assert.Equal(t, availabilityEnd, body.DefaultSelection.End)
			assert.Equal(t, 1, readSvc.listAvailabilityCalls)
			assert.Equal(t, validInstrument.Venue, readSvc.lastAvailabilityQuery.Venue)
			assert.Equal(t, validInstrument.Symbol, readSvc.lastAvailabilityQuery.Symbol)
			assert.Equal(t, validInstrument.AssetClass, readSvc.lastAvailabilityQuery.AssetClass)
			assert.Equal(t, 12, readSvc.lastAvailabilityQuery.Limit)
			assert.Empty(t, readSvc.lastAvailabilityQuery.Cursor)

			var raw map[string]any
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &raw))
			_, hasDefaultSelection := raw["defaultSelection"]
			assert.True(t, hasDefaultSelection)
			itemsRaw, ok := raw["items"].([]any)
			require.True(t, ok)
			itemRaw, ok := itemsRaw[0].(map[string]any)
			require.True(t, ok)
			_, hasAssetClass := itemRaw["assetClass"]
			_, hasDefaultSlice := itemRaw["defaultSlice"]
			assert.True(t, hasAssetClass)
			assert.True(t, hasDefaultSlice)
		})

		t.Run("cursor pages omit defaultSelection even when lower layer returns it", func(t *testing.T) {
			readSvc := makeReplayReadService()
			cursor := base64.RawURLEncoding.EncodeToString([]byte(
				validEnd.UTC().Format(time.RFC3339Nano) + "\n" +
					validInstrument.Venue.String() + "\n" +
					validInstrument.Symbol.String() + "\n" +
					validInstrument.AssetClass.String(),
			))
			readSvc.listCandleAvailabilityFunc = func(
				_ context.Context,
				_ data.CandleAvailabilityListQuery,
			) (data.CandleAvailabilityListResult, error) {
				return data.CandleAvailabilityListResult{
					Items: []data.CandleAvailabilityItem{
						{
							Venue:      validInstrument.Venue,
							Symbol:     validInstrument.Symbol,
							AssetClass: validInstrument.AssetClass,
							Timeframes: []data.CandleAvailabilityTimeframeSummary{{
								Timeframe: validTimeframe,
								StartAt:   validStart,
								EndAt:     validEnd,
								Count:     1,
							}},
							DefaultSlice: data.CandleAvailabilityDefaultSlice{
								Timeframe: validTimeframe,
								StartAt:   validStart,
								EndAt:     validEnd,
							},
						},
					},
					DefaultSelection: &data.CandleAvailabilityDefaultSelection{
						Venue:      validInstrument.Venue,
						Symbol:     validInstrument.Symbol,
						AssetClass: validInstrument.AssetClass,
						Timeframe:  validTimeframe,
						StartAt:    validStart,
						EndAt:      validEnd,
					},
				}, nil
			}

			ctrl := newController(readSvc, makeLineageBrowserService(), makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(
				http.MethodGet,
				makeAvailabilityURL(map[string]string{"cursor": cursor}),
				true,
			))

			require.Equal(t, http.StatusOK, resp.Code)
			assert.Equal(t, 1, readSvc.listAvailabilityCalls)
			assert.Equal(t, cursor, readSvc.lastAvailabilityQuery.Cursor)
			var raw map[string]any
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &raw))
			_, hasDefaultSelection := raw["defaultSelection"]
			assert.False(t, hasDefaultSelection)
		})

		t.Run("whitespace cursor keeps defaultSelection on canonical first page", func(t *testing.T) {
			readSvc := makeReplayReadService()
			readSvc.listCandleAvailabilityFunc = func(
				_ context.Context,
				_ data.CandleAvailabilityListQuery,
			) (data.CandleAvailabilityListResult, error) {
				return data.CandleAvailabilityListResult{
					Items: []data.CandleAvailabilityItem{{
						Venue:      validInstrument.Venue,
						Symbol:     validInstrument.Symbol,
						AssetClass: validInstrument.AssetClass,
						Timeframes: []data.CandleAvailabilityTimeframeSummary{{
							Timeframe: validTimeframe,
							StartAt:   validStart,
							EndAt:     validEnd,
							Count:     1,
						}},
						DefaultSlice: data.CandleAvailabilityDefaultSlice{
							Timeframe: validTimeframe,
							StartAt:   validStart,
							EndAt:     validEnd,
						},
					}},
					DefaultSelection: &data.CandleAvailabilityDefaultSelection{
						Venue:      validInstrument.Venue,
						Symbol:     validInstrument.Symbol,
						AssetClass: validInstrument.AssetClass,
						Timeframe:  validTimeframe,
						StartAt:    validStart,
						EndAt:      validEnd,
					},
				}, nil
			}

			ctrl := newController(readSvc, makeLineageBrowserService(), makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(
				http.MethodGet,
				makeAvailabilityURL(map[string]string{"cursor": "   "}),
				true,
			))

			require.Equal(t, http.StatusOK, resp.Code)
			assert.Equal(t, 1, readSvc.listAvailabilityCalls)
			assert.Empty(t, readSvc.lastAvailabilityQuery.Cursor)

			var body models.CandleAvailabilityListResponse
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
			require.NotNil(t, body.DefaultSelection)

			var raw map[string]any
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &raw))
			_, hasDefaultSelection := raw["defaultSelection"]
			assert.True(t, hasDefaultSelection)
		})

		t.Run("empty availability returns empty items without default selection", func(t *testing.T) {
			readSvc := makeReplayReadService()
			readSvc.listCandleAvailabilityFunc = func(
				_ context.Context,
				_ data.CandleAvailabilityListQuery,
			) (data.CandleAvailabilityListResult, error) {
				return data.CandleAvailabilityListResult{Items: []data.CandleAvailabilityItem{}}, nil
			}

			lineageSvc := makeLineageBrowserService()
			ctrl := newController(readSvc, lineageSvc, makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(http.MethodGet, makeAvailabilityURL(nil), true))

			require.Equal(t, http.StatusOK, resp.Code)
			var body models.CandleAvailabilityListResponse
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
			require.Empty(t, body.Items)
			assert.Nil(t, body.DefaultSelection)
			assert.Equal(t, 0, lineageSvc.listCalls)
			assert.Equal(t, 0, lineageSvc.detailCalls)
			assert.Equal(t, 0, lineageSvc.linkedCalls)
		})

		t.Run("unsupported availability query params are rejected before reads", func(t *testing.T) {
			readSvc := makeReplayReadService()
			ctrl := newController(readSvc, makeLineageBrowserService(), makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(
				http.MethodGet,
				makeAvailabilityURL(map[string]string{"timeframe": validTimeframe.String()}),
				true,
			))

			assert.Equal(t, http.StatusBadRequest, resp.Code)
			assert.Equal(t, 0, readSvc.listAvailabilityCalls)
			assert.Equal(t, 0, readSvc.calls)
		})
	})

	t.Run("ListDataCandles", func(t *testing.T) {
		t.Run("valid request replays candles and maps chart-ready response", func(t *testing.T) {
			readSvc := makeReplayReadService()
			identityOne := int64(101)
			identityTwo := int64(202)
			readSvc.replayCandlesFunc = func(
				_ context.Context,
				instrument domain.Instrument,
				timeframe domain.Timeframe,
				timeRange domain.TimeRange,
			) ([]data.ReplayCandle, error) {
				return []data.ReplayCandle{
					{
						Identity: uint64(identityOne),
						Candle: domain.Candle{
							Instrument: instrument,
							Timeframe:  timeframe,
							TimeRange:  domain.TimeRange{Start: timeRange.Start, End: timeRange.Start.Add(time.Minute)},
							Open:       100,
							High:       110,
							Low:        95,
							Close:      108,
							Volume:     7,
							Quality:    domain.DataQualityValidated,
							Provenance: domain.SourceProvenance{Source: "hyperliquid", RecordID: fake.UUID().V4()},
						},
					},
					{
						Identity: uint64(identityTwo),
						Candle: domain.Candle{
							Instrument: instrument,
							Timeframe:  timeframe,
							TimeRange: domain.TimeRange{
								Start: timeRange.Start.Add(time.Minute),
								End:   timeRange.Start.Add(2 * time.Minute),
							},
							Open:       108,
							High:       111,
							Low:        100,
							Close:      104,
							Volume:     9,
							Quality:    domain.DataQualitySuspect,
							Provenance: domain.SourceProvenance{Source: "hyperliquid", RecordID: fake.UUID().V4()},
						},
					},
				}, nil
			}

			ctrl := newController(readSvc, makeLineageBrowserService(), makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(http.MethodGet, makeCandleURL(nil), true))

			require.Equal(t, http.StatusOK, resp.Code)
			var body models.DataCandleListResponse
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
			require.Len(t, body.Items, 2)
			assert.Equal(t, identityOne, body.Items[0].IDentity)
			assert.Equal(t, validInstrument.Venue.String(), body.Items[0].Venue)
			assert.Equal(t, validInstrument.Symbol.String(), body.Items[0].Symbol)
			assert.Equal(t, validInstrument.AssetClass.String(), body.Items[0].AssetClass)
			assert.Equal(t, validTimeframe.String(), body.Items[0].Timeframe)
			assert.Equal(t, "hyperliquid", body.Items[0].ProvenanceSource)
			assert.NotEmpty(t, body.Items[0].ProvenanceIDentity)
			assert.Equal(t, identityTwo, body.Items[1].IDentity)
			assert.True(t, body.Items[0].Start.Before(body.Items[1].Start))
			assert.Equal(t, 1, readSvc.calls)
			assert.Equal(t, validInstrument.Venue, readSvc.lastInstrument.Venue)
			assert.Equal(t, validInstrument.Symbol, readSvc.lastInstrument.Symbol)
			assert.Equal(t, validInstrument.AssetClass, readSvc.lastInstrument.AssetClass)
			assert.Equal(t, validTimeframe, readSvc.lastTimeframe)
			assert.Equal(t, validStart, readSvc.lastTimeRange.Start)
			assert.Equal(t, validEnd, readSvc.lastTimeRange.End)
		})

		t.Run("exactly ten thousand intervals is accepted", func(t *testing.T) {
			readSvc := makeReplayReadService()
			readSvc.replayCandlesFunc = func(
				_ context.Context,
				_ domain.Instrument,
				_ domain.Timeframe,
				_ domain.TimeRange,
			) ([]data.ReplayCandle, error) {
				return []data.ReplayCandle{}, nil
			}
			ctrl := newController(readSvc, makeLineageBrowserService(), makeAuthMiddleware())
			start := validStart
			end := start.Add(10000 * time.Minute)
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(
				http.MethodGet,
				makeCandleURL(map[string]string{"start": start.Format(time.RFC3339), "end": end.Format(time.RFC3339)}),
				true,
			))

			assert.Equal(t, http.StatusOK, resp.Code)
			assert.Equal(t, 1, readSvc.calls)
		})

		t.Run("more than ten thousand intervals returns bad request before replay", func(t *testing.T) {
			readSvc := makeReplayReadService()
			ctrl := newController(readSvc, makeLineageBrowserService(), makeAuthMiddleware())
			start := validStart
			end := start.Add(10000*time.Minute + time.Second)
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(
				http.MethodGet,
				makeCandleURL(map[string]string{"start": start.Format(time.RFC3339), "end": end.Format(time.RFC3339)}),
				true,
			))

			assert.Equal(t, http.StatusBadRequest, resp.Code)
			assert.Equal(t, 0, readSvc.calls)
		})

		t.Run("invalid request variants return 4xx before replay", func(t *testing.T) {
			for _, tc := range []struct {
				name   string
				params map[string]string
			}{
				{name: "missing venue", params: map[string]string{"venue": ""}},
				{name: "missing symbol", params: map[string]string{"symbol": ""}},
				{name: "missing asset class", params: map[string]string{"assetClass": ""}},
				{name: "missing timeframe", params: map[string]string{"timeframe": ""}},
				{name: "missing start", params: map[string]string{"start": ""}},
				{name: "missing end", params: map[string]string{"end": ""}},
				{name: "invalid venue", params: map[string]string{"venue": "bad-venue"}},
				{name: "blank symbol", params: map[string]string{"symbol": "   "}},
				{name: "invalid asset class", params: map[string]string{"assetClass": "bad-asset-class"}},
				{name: "invalid timeframe", params: map[string]string{"timeframe": "bad-timeframe"}},
				{name: "invalid range", params: map[string]string{"start": validEnd.Format(time.RFC3339), "end": validStart.Format(time.RFC3339)}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					readSvc := makeReplayReadService()
					ctrl := newController(readSvc, makeLineageBrowserService(), makeAuthMiddleware())
					resp := httptest.NewRecorder()
					newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(http.MethodGet, makeCandleURL(tc.params), true))

					assert.True(t, resp.Code >= 400 && resp.Code < 500)
					assert.Equal(t, 0, readSvc.calls)
				})
			}
		})

		t.Run("missing candles are not synthesized", func(t *testing.T) {
			readSvc := makeReplayReadService()
			readSvc.replayCandlesFunc = func(
				_ context.Context,
				instrument domain.Instrument,
				timeframe domain.Timeframe,
				timeRange domain.TimeRange,
			) ([]data.ReplayCandle, error) {
				return []data.ReplayCandle{
					{
						Identity: 1,
						Candle: domain.Candle{
							Instrument: instrument,
							Timeframe:  timeframe,
							TimeRange:  domain.TimeRange{Start: timeRange.Start, End: timeRange.Start.Add(time.Minute)},
							Open:       1,
							High:       2,
							Low:        1,
							Close:      2,
							Volume:     3,
							Quality:    domain.DataQualityValidated,
							Provenance: domain.SourceProvenance{Source: "hyperliquid", RecordID: fake.UUID().V4()},
						},
					},
					{
						Identity: 2,
						Candle: domain.Candle{
							Instrument: instrument,
							Timeframe:  timeframe,
							TimeRange: domain.TimeRange{
								Start: timeRange.Start.Add(3 * time.Minute),
								End:   timeRange.Start.Add(4 * time.Minute),
							},
							Open:       2,
							High:       3,
							Low:        2,
							Close:      3,
							Volume:     4,
							Quality:    domain.DataQualityValidated,
							Provenance: domain.SourceProvenance{Source: "hyperliquid", RecordID: fake.UUID().V4()},
						},
					},
				}, nil
			}
			ctrl := newController(readSvc, makeLineageBrowserService(), makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(http.MethodGet, makeCandleURL(nil), true))

			require.Equal(t, http.StatusOK, resp.Code)
			var body models.DataCandleListResponse
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
			assert.Len(t, body.Items, 2)
			assert.Equal(t, validStart, body.Items[0].Start)
			assert.Equal(t, validStart.Add(3*time.Minute), body.Items[1].Start)
		})

		t.Run("blank venue returns bad request before replay", func(t *testing.T) {
			readSvc := makeReplayReadService()
			ctrl := newController(readSvc, makeLineageBrowserService(), makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(
				resp,
				newRequest(http.MethodGet, makeCandleURL(map[string]string{"venue": "   "}), true),
			)

			assert.Equal(t, http.StatusBadRequest, resp.Code)
			assert.Equal(t, 0, readSvc.calls)
		})

		t.Run("instrument not found maps to not found", func(t *testing.T) {
			readSvc := makeReplayReadService()
			readSvc.replayCandlesFunc = func(
				_ context.Context,
				_ domain.Instrument,
				_ domain.Timeframe,
				_ domain.TimeRange,
			) ([]data.ReplayCandle, error) {
				return nil, data.ErrInstrumentNotFound
			}
			ctrl := newController(readSvc, makeLineageBrowserService(), makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(http.MethodGet, makeCandleURL(nil), true))

			assert.Equal(t, http.StatusNotFound, resp.Code)
		})

		t.Run("replay identity overflow returns internal server error", func(t *testing.T) {
			readSvc := makeReplayReadService()
			readSvc.replayCandlesFunc = func(
				_ context.Context,
				instrument domain.Instrument,
				_ domain.Timeframe,
				_ domain.TimeRange,
			) ([]data.ReplayCandle, error) {
				return []data.ReplayCandle{{
					Identity: math.MaxUint64,
					Candle: domain.Candle{
						Instrument: instrument,
						Timeframe:  validTimeframe,
						TimeRange:  domain.TimeRange{Start: validStart, End: validStart.Add(time.Minute)},
						Open:       1,
						High:       1,
						Low:        1,
						Close:      1,
						Volume:     1,
						Quality:    domain.DataQualityValidated,
						Provenance: domain.SourceProvenance{Source: "hyperliquid", RecordID: fake.UUID().V4()},
					},
				}}, nil
			}
			ctrl := newController(readSvc, makeLineageBrowserService(), makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(http.MethodGet, makeCandleURL(nil), true))

			assert.Equal(t, http.StatusInternalServerError, resp.Code)
		})
	})

	t.Run("raw payload endpoints map metadata, detail, and linked evidence", func(t *testing.T) {
		t.Run("list returns metadata only and forwards filters and pagination", func(t *testing.T) {
			lineageSvc := makeLineageBrowserService()
			metadata := makeRawPayloadMetadata(fake.UUID().V4())
			returnedCursor := fake.Lorem().Word()
			requestedCursor := base64.RawURLEncoding.EncodeToString(
				[]byte(validStart.Format(time.RFC3339Nano) + "\n" + fake.UUID().V4()),
			)
			lineageSvc.listRawPayloadMetadataFunc = func(
				_ context.Context,
				_ data.RawPayloadMetadataListQuery,
			) (data.RawPayloadMetadataListResult, error) {
				return data.RawPayloadMetadataListResult{
					Items:      []data.RawPayloadMetadata{metadata},
					NextCursor: returnedCursor,
				}, nil
			}
			ctrl := newController(makeReplayReadService(), lineageSvc, makeAuthMiddleware())
			listURL := makeRawPayloadListURL(map[string]string{
				"symbol":         validInstrument.Symbol.String(),
				"assetClass":     validInstrument.AssetClass.String(),
				"timeframe":      validTimeframe.String(),
				"start":          validStart.Format(time.RFC3339),
				"end":            validEnd.Format(time.RFC3339),
				"ingestionRunId": metadata.IngestionRunID,
				"entityHint":     metadata.EntityHint,
				"endpoint":       metadata.Endpoint,
				"requestType":    metadata.RequestType,
				"limit":          "75",
				"cursor":         requestedCursor,
			})
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(http.MethodGet, listURL, true))

			require.Equal(t, http.StatusOK, resp.Code)
			var body models.RawPayloadMetadataListResponse
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
			require.Len(t, body.Items, 1)
			assert.Equal(t, metadata.ID, body.Items[0].ID)
			assert.Equal(t, metadata.Instrument.Symbol.String(), body.Items[0].Symbol)
			assert.Equal(t, metadata.Instrument.AssetClass.String(), body.Items[0].AssetClass)
			assert.Equal(t, returnedCursor, body.NextCursor)
			assert.Equal(t, 1, lineageSvc.listCalls)
			assert.Equal(t, validInstrument.Venue, lineageSvc.lastListQuery.Venue)
			require.NotNil(t, lineageSvc.lastListQuery.Instrument)
			assert.Equal(t, validInstrument.Symbol, lineageSvc.lastListQuery.Instrument.Symbol)
			assert.Equal(t, validInstrument.AssetClass, lineageSvc.lastListQuery.Instrument.AssetClass)
			assert.Equal(t, validTimeframe, lineageSvc.lastListQuery.Timeframe)
			require.NotNil(t, lineageSvc.lastListQuery.TimeRange)
			assert.Equal(t, validStart, lineageSvc.lastListQuery.TimeRange.Start)
			assert.Equal(t, validEnd, lineageSvc.lastListQuery.TimeRange.End)
			assert.Equal(t, metadata.IngestionRunID, lineageSvc.lastListQuery.IngestionRunID)
			assert.Equal(t, metadata.EntityHint, lineageSvc.lastListQuery.EntityHint)
			assert.Equal(t, metadata.Endpoint, lineageSvc.lastListQuery.Endpoint)
			assert.Equal(t, metadata.RequestType, lineageSvc.lastListQuery.RequestType)
			assert.Equal(t, 75, lineageSvc.lastListQuery.Limit)
			assert.Equal(t, requestedCursor, lineageSvc.lastListQuery.Cursor)
			assert.NotContains(t, resp.Body.String(), "responseBodyPreview")
		})

		t.Run("list omits optional time range when metadata has no range", func(t *testing.T) {
			lineageSvc := makeLineageBrowserService()
			metadata := makeRawPayloadMetadata(fake.UUID().V4())
			metadata.TimeRange = nil
			lineageSvc.listRawPayloadMetadataFunc = func(
				_ context.Context,
				_ data.RawPayloadMetadataListQuery,
			) (data.RawPayloadMetadataListResult, error) {
				return data.RawPayloadMetadataListResult{Items: []data.RawPayloadMetadata{metadata}}, nil
			}
			ctrl := newController(makeReplayReadService(), lineageSvc, makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(http.MethodGet, makeRawPayloadListURL(nil), true))

			require.Equal(t, http.StatusOK, resp.Code)
			assert.NotContains(t, resp.Body.String(), `"start"`)
			assert.NotContains(t, resp.Body.String(), `"end"`)

			var body models.RawPayloadMetadataListResponse
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
			require.Len(t, body.Items, 1)
			assert.Nil(t, body.Items[0].Start)
			assert.Nil(t, body.Items[0].End)
			assert.Nil(t, lineageSvc.lastListQuery.TimeRange)
		})

		t.Run("list preserves supplied fixed-offset bounds", func(t *testing.T) {
			lineageSvc := makeLineageBrowserService()
			lineageSvc.listRawPayloadMetadataFunc = func(
				_ context.Context,
				_ data.RawPayloadMetadataListQuery,
			) (data.RawPayloadMetadataListResult, error) {
				return data.RawPayloadMetadataListResult{}, nil
			}
			start := time.Date(2026, time.July, 10, 8, 30, 0, 123, time.FixedZone("request-east", 5*60*60+30*60))
			end := time.Date(2026, time.July, 10, 9, 30, 0, 456, time.FixedZone("request-west", -4*60*60))
			ctrl := newController(makeReplayReadService(), lineageSvc, makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(
				http.MethodGet,
				makeRawPayloadListURL(map[string]string{
					"start": start.Format(time.RFC3339Nano),
					"end":   end.Format(time.RFC3339Nano),
				}),
				true,
			))

			require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
			require.NotNil(t, lineageSvc.lastListQuery.TimeRange)
			assert.Equal(
				t,
				start.Format(time.RFC3339Nano),
				lineageSvc.lastListQuery.TimeRange.Start.Format(time.RFC3339Nano),
			)
			assert.Equal(
				t,
				end.Format(time.RFC3339Nano),
				lineageSvc.lastListQuery.TimeRange.End.Format(time.RFC3339Nano),
			)
		})

		t.Run("list rejects supplied invalid optional bounds before runtime reads", func(t *testing.T) {
			for _, testCase := range []struct {
				name  string
				query string
			}{
				{name: "empty", query: ""},
				{name: "null text", query: "null"},
				{name: "malformed", query: "not-a-timestamp"},
				{name: "year one", query: "0001-01-01T00:00:00Z"},
			} {
				t.Run(testCase.name, func(t *testing.T) {
					lineageSvc := makeLineageBrowserService()
					lineageSvc.listRawPayloadMetadataFunc = func(
						_ context.Context,
						_ data.RawPayloadMetadataListQuery,
					) (data.RawPayloadMetadataListResult, error) {
						return data.RawPayloadMetadataListResult{}, nil
					}
					query := url.Values{"venue": []string{validInstrument.Venue.String()}}
					query["start"] = []string{testCase.query}
					ctrl := newController(makeReplayReadService(), lineageSvc, makeAuthMiddleware())
					resp := httptest.NewRecorder()
					newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(
						http.MethodGet,
						"/api/v1/data/raw-payloads?"+query.Encode(),
						true,
					))

					assert.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
					assert.Equal(t, 0, lineageSvc.listCalls)
				})
			}
		})

		t.Run("list rejects unsupported venue", func(t *testing.T) {
			ctrl := newController(makeReplayReadService(), makeLineageBrowserService(), makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(
				resp,
				newRequest(http.MethodGet, makeRawPayloadListURL(map[string]string{"venue": "other-venue"}), true),
			)

			assert.Equal(t, http.StatusBadRequest, resp.Code)
		})

		t.Run("list rejects invalid filter combinations before runtime reads", func(t *testing.T) {
			lineageSvc := makeLineageBrowserService()
			ctrl := newController(makeReplayReadService(), lineageSvc, makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(
				resp,
				newRequest(
					http.MethodGet,
					makeRawPayloadListURL(map[string]string{"symbol": validInstrument.Symbol.String()}),
					true,
				),
			)

			assert.Equal(t, http.StatusBadRequest, resp.Code)
			assert.Equal(t, 0, lineageSvc.listCalls)
		})

		t.Run("list service errors surface as internal server error", func(t *testing.T) {
			lineageSvc := makeLineageBrowserService()
			lineageSvc.listRawPayloadMetadataFunc = func(
				_ context.Context,
				_ data.RawPayloadMetadataListQuery,
			) (data.RawPayloadMetadataListResult, error) {
				return data.RawPayloadMetadataListResult{}, errors.New("boom")
			}
			ctrl := newController(makeReplayReadService(), lineageSvc, makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(http.MethodGet, makeRawPayloadListURL(nil), true))

			assert.Equal(t, http.StatusInternalServerError, resp.Code)
		})

		t.Run("detail returns preview metadata and truncation fields", func(t *testing.T) {
			lineageSvc := makeLineageBrowserService()
			metadata := makeRawPayloadMetadata(fake.UUID().V4())
			lineageSvc.getRawPayloadDetailFunc = func(_ context.Context, _ string) (data.RawPayloadDetail, error) {
				return data.RawPayloadDetail{
					Metadata:                     metadata,
					ResponseBodySizeBytes:        123,
					ResponseBodyPreview:          []byte(`{"hello":"world"}`),
					ResponseBodyPreviewTruncated: true,
				}, nil
			}
			ctrl := newController(makeReplayReadService(), lineageSvc, makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(
				resp,
				newRequest(http.MethodGet, "/api/v1/data/raw-payloads/"+metadata.ID, true),
			)

			require.Equal(t, http.StatusOK, resp.Code)
			var body models.RawPayloadDetailResponse
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
			assert.Equal(t, metadata.ID, body.Metadata.ID)
			assert.Equal(t, int64(123), body.ResponseBodySizeBytes)
			assert.JSONEq(t, `{"hello":"world"}`, body.ResponseBodyPreview)
			assert.True(t, body.ResponseBodyPreviewTruncated)
			assert.Equal(t, 1, lineageSvc.detailCalls)
			assert.Equal(t, metadata.ID, lineageSvc.lastDetailID)
		})

		t.Run("detail missing id returns not found", func(t *testing.T) {
			lineageSvc := makeLineageBrowserService()
			missingID := fake.UUID().V4()
			lineageSvc.getRawPayloadDetailFunc = func(_ context.Context, _ string) (data.RawPayloadDetail, error) {
				return data.RawPayloadDetail{}, data.ErrRawPayloadNotFound
			}
			ctrl := newController(makeReplayReadService(), lineageSvc, makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(
				resp,
				newRequest(http.MethodGet, "/api/v1/data/raw-payloads/"+missingID, true),
			)

			assert.Equal(t, http.StatusNotFound, resp.Code)
		})

		t.Run("detail unknown failures surface as internal server error", func(t *testing.T) {
			lineageSvc := makeLineageBrowserService()
			lineageSvc.getRawPayloadDetailFunc = func(_ context.Context, _ string) (data.RawPayloadDetail, error) {
				return data.RawPayloadDetail{}, errors.New("boom")
			}
			ctrl := newController(makeReplayReadService(), lineageSvc, makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(
				resp,
				newRequest(http.MethodGet, "/api/v1/data/raw-payloads/"+fake.UUID().V4(), true),
			)

			assert.Equal(t, http.StatusInternalServerError, resp.Code)
		})

		t.Run("linked evidence maps provenance-bearing candle key", func(t *testing.T) {
			lineageSvc := makeLineageBrowserService()
			metadata := makeRawPayloadMetadata(fake.UUID().V4())
			provenanceIdentity := fake.UUID().V4()
			lineageSvc.listCandleLinkedFunc = func(
				_ context.Context,
				_ data.CandleLinkedRawPayloadsQuery,
			) ([]data.RawPayloadMetadata, error) {
				return []data.RawPayloadMetadata{metadata}, nil
			}
			ctrl := newController(makeReplayReadService(), lineageSvc, makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(
				http.MethodGet,
				makeCandleRawPayloadURL(map[string]string{"provenanceIdentity": provenanceIdentity}),
				true,
			))

			require.Equal(t, http.StatusOK, resp.Code)
			var body models.CandleRawPayloadMetadataListResponse
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
			require.Len(t, body.Items, 1)
			assert.Equal(t, metadata.ID, body.Items[0].ID)
			assert.Equal(t, 1, lineageSvc.linkedCalls)
			assert.Equal(t, validInstrument.Venue, lineageSvc.lastLinkedQuery.Venue)
			assert.Equal(t, validInstrument.Symbol, lineageSvc.lastLinkedQuery.Symbol)
			assert.Equal(t, validInstrument.AssetClass, lineageSvc.lastLinkedQuery.AssetClass)
			assert.Equal(t, validTimeframe, lineageSvc.lastLinkedQuery.Timeframe)
			assert.Equal(t, validStart, lineageSvc.lastLinkedQuery.TimeRange.Start)
			assert.Equal(t, validEnd, lineageSvc.lastLinkedQuery.TimeRange.End)
			assert.Equal(t, "hyperliquid", lineageSvc.lastLinkedQuery.ProvenanceSource)
			assert.Equal(t, provenanceIdentity, lineageSvc.lastLinkedQuery.ProvenanceIdentity)
		})

		t.Run("linked evidence service errors surface as internal server error", func(t *testing.T) {
			lineageSvc := makeLineageBrowserService()
			lineageSvc.listCandleLinkedFunc = func(
				_ context.Context,
				_ data.CandleLinkedRawPayloadsQuery,
			) ([]data.RawPayloadMetadata, error) {
				return nil, errors.New("boom")
			}
			ctrl := newController(makeReplayReadService(), lineageSvc, makeAuthMiddleware())
			resp := httptest.NewRecorder()
			newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(http.MethodGet, makeCandleRawPayloadURL(nil), true))

			assert.Equal(t, http.StatusInternalServerError, resp.Code)
		})

		t.Run("linked evidence rejects omitted or blank provenance", func(t *testing.T) {
			for _, tc := range []struct {
				name   string
				params map[string]string
			}{
				{name: "omitted provenance identity", params: map[string]string{"provenanceIdentity": ""}},
				{name: "blank provenance source", params: map[string]string{"provenanceSource": "   "}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					lineageSvc := makeLineageBrowserService()
					ctrl := newController(makeReplayReadService(), lineageSvc, makeAuthMiddleware())
					resp := httptest.NewRecorder()
					newDataHTTPHandler(ctrl).ServeHTTP(resp, newRequest(
						http.MethodGet,
						makeCandleRawPayloadURL(tc.params),
						true,
					))

					assert.Equal(t, http.StatusBadRequest, resp.Code)
					assert.Equal(t, 0, lineageSvc.linkedCalls)
				})
			}
		})
	})

	t.Run("helper functions cover unsupported branches", func(t *testing.T) {
		_, err := timeframeDuration(domain.Timeframe("bad-timeframe"))
		require.Error(t, err)

		_, err = validateSupportedVenue("bad-venue")
		require.Error(t, err)

		mapped := mapDataReadError(data.ErrValidation, "ignored")
		require.Error(t, mapped)
	})
}

func TestDataControllerRegisteredRawPayloadRouteRegression(t *testing.T) {
	fake := faker.New()
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.FixedZone("e2e-new-york", -4*60*60))
	end := start.Add(12 * time.Hour)
	completedAt := end
	dsn := filepath.Join(t.TempDir(), "data-controller.sqlite")
	sqlDB, err := sqlconn.Open(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	store, err := data.NewDatabaseStore(sqlDB, dsn, data.DatabaseStoreOpts{})
	require.NoError(t, err)
	require.NoError(t, store.AutoMigrate())
	blobStore, err := data.NewLocalRawPayloadBlobStore(filepath.Join(t.TempDir(), "raw-payloads"))
	require.NoError(t, err)
	lineageService, err := data.NewLineageService(data.LineageServiceDeps{Store: store, BlobStore: blobStore})
	require.NoError(t, err)
	readService, err := data.NewReadService(data.ReadServiceDeps{
		InstrumentStore: store,
		CandleStore:     store,
		TradeStore:      store,
	})
	require.NoError(t, err)

	run, err := data.NewIngestionRun(data.IngestionRunParams{
		ID:          fake.UUID().V4(),
		Source:      "hyperliquid-perps-rest",
		Venue:       domain.Venue("hyperliquid-perps"),
		Status:      data.IngestionRunStatusSucceeded,
		StartedAt:   start,
		CompletedAt: &completedAt,
	})
	require.NoError(t, err)
	_, err = store.UpsertIngestionRun(t.Context(), run)
	require.NoError(t, err)

	payload, err := data.NewRawVenuePayload(data.RawVenuePayloadParams{
		ID:                 fake.UUID().V4(),
		IngestionRunID:     run.ID,
		Source:             run.Source,
		Venue:              run.Venue,
		Endpoint:           "/info",
		RequestType:        "candleSnapshot",
		RequestPayloadHash: fake.UUID().V4(),
		RequestAt:          start.Add(-2 * time.Second),
		ResponseAt:         start.Add(-time.Second),
		HTTPStatus:         http.StatusOK,
		ResponseBodyHash:   fake.UUID().V4(),
		PayloadBodyRef:     fake.UUID().V4(),
		EntityHint:         "candle",
		Timeframe:          domain.Timeframe1h,
		TimeRange:          &domain.TimeRange{Start: start, End: end},
		ReceivedAt:         start,
	})
	require.NoError(t, err)
	_, err = store.UpsertRawVenuePayload(t.Context(), payload)
	require.NoError(t, err)

	instrument, err := store.UpsertInstrument(t.Context(), domain.Instrument{
		Venue:      run.Venue,
		Symbol:     domain.Symbol("BTC"),
		AssetClass: domain.AssetClassFuture,
		Active:     true,
	})
	require.NoError(t, err)
	candle := domain.Candle{
		Instrument: instrument,
		Timeframe:  domain.Timeframe1h,
		TimeRange:  domain.TimeRange{Start: start, End: start.Add(time.Hour)},
		Open:       1,
		High:       2,
		Low:        1,
		Close:      2,
		Volume:     3,
		Quality:    domain.DataQualityValidated,
		Provenance: domain.SourceProvenance{Source: "hyperliquid", RecordID: fake.UUID().V4()},
	}
	_, err = store.UpsertCandle(t.Context(), candle)
	require.NoError(t, err)
	require.NoError(t, store.LinkRawPayloadToCandle(t.Context(), payload.ID, candle))

	authMiddleware := func(next http.Handler) http.Handler { return next }
	controller := NewDataController(DataControllerDeps{
		ReadService:    readService,
		LineageService: lineageService,
		AuthMiddleware: authMiddleware,
	})
	query := url.Values{
		"venue":      []string{run.Venue.String()},
		"symbol":     []string{instrument.Symbol.String()},
		"assetClass": []string{instrument.AssetClass.String()},
		"timeframe":  []string{domain.Timeframe1h.String()},
		"start":      []string{start.Format(time.RFC3339)},
		"end":        []string{end.Format(time.RFC3339)},
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/data/raw-payloads?"+query.Encode(), http.NoBody)
	response := httptest.NewRecorder()
	newDataHTTPHandler(controller).ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var body models.RawPayloadMetadataListResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Len(t, body.Items, 1)
	require.Equal(t, payload.ID, body.Items[0].ID)
}
