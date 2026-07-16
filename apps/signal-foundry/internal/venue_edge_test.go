//go:build !release

package internal

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/venueedge"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestVenueEdgeWiring(t *testing.T) {
	t.Parallel()

	makeStore := func(t *testing.T) *data.DatabaseStore {
		t.Helper()

		sqlDB, err := sqlconn.Open(":memory:")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
		store, err := data.NewDatabaseStore(sqlDB, ":memory:", data.DatabaseStoreOpts{})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())

		return store
	}

	makeLineageService := func(t *testing.T) *data.LineageService {
		t.Helper()

		store := makeStore(t)
		service, err := data.NewLineageService(data.LineageServiceDeps{
			Store:     store,
			BlobStore: store,
		})
		require.NoError(t, err)

		return service
	}

	randomWord := func(prefix string) string {
		return prefix + faker.New().Lorem().Word()
	}

	makeInstrument := func(t *testing.T) domain.Instrument {
		t.Helper()

		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      domain.Venue(randomWord("venue-")),
			Symbol:     domain.Symbol(randomWord("symbol-")),
			AssetClass: domain.AssetClassFuture,
			Active:     true,
		})
		require.NoError(t, err)

		return instrument
	}

	makeCapture := func(t *testing.T, instrument *domain.Instrument) venueedge.HyperliquidRawEvidenceCapture {
		t.Helper()

		capture := venueedge.HyperliquidRawEvidenceCapture{
			ID:                 randomWord("raw-"),
			Venue:              venueedge.HyperliquidPerpsVenueName,
			Endpoint:           "/info",
			RequestType:        "meta",
			RequestPayloadHash: randomWord("request-hash-"),
			RequestMetadata:    map[string]string{"method": http.MethodPost},
			RequestAt:          time.Now().Add(-time.Second).UTC(),
			ResponseAt:         time.Now().UTC(),
			HTTPStatus:         http.StatusOK,
			EntityHint:         "instrument",
			Instrument:         instrument,
			ReceivedAt:         time.Now().UTC(),
		}
		if instrument != nil {
			capture.ResponseBody = []byte(fmt.Sprintf(
				`{"universe":[{"name":"%s","isDelisted":false}]}`,
				instrument.Symbol,
			))
		}

		return capture
	}

	t.Run("newHyperliquidRawEvidenceRecorder", func(t *testing.T) {
		t.Parallel()

		t.Run("requires lineage service", func(t *testing.T) {
			t.Parallel()

			recorder, err := newHyperliquidRawEvidenceRecorder(nil)
			require.Error(t, err)
			require.Nil(t, recorder)
		})

		t.Run("records raw payload evidence", func(t *testing.T) {
			t.Parallel()

			lineageService := makeLineageService(t)
			recorder, err := newHyperliquidRawEvidenceRecorder(lineageService)
			require.NoError(t, err)

			instrument := makeInstrument(t)
			capture := makeCapture(t, &instrument)

			payloadID, err := recorder.RecordHyperliquidRawEvidence(t.Context(), capture)
			require.NoError(t, err)
			require.NotEmpty(t, payloadID)
		})

		t.Run("returns build error for invalid capture", func(t *testing.T) {
			t.Parallel()

			lineageService := makeLineageService(t)
			recorder, err := newHyperliquidRawEvidenceRecorder(lineageService)
			require.NoError(t, err)

			instrument := makeInstrument(t)
			capture := makeCapture(t, &instrument)
			capture.HTTPStatus = 0

			payloadID, err := recorder.RecordHyperliquidRawEvidence(t.Context(), capture)
			require.Error(t, err)
			require.Empty(t, payloadID)
			require.ErrorContains(t, err, "build raw venue payload")
		})

		t.Run("returns lineage persistence error", func(t *testing.T) {
			t.Parallel()

			lineageService := makeLineageService(t)
			recorder, err := newHyperliquidRawEvidenceRecorder(lineageService)
			require.NoError(t, err)

			instrument := makeInstrument(t)
			capture := makeCapture(t, &instrument)
			capture.IngestionRunID = randomWord("missing-run-")

			payloadID, err := recorder.RecordHyperliquidRawEvidence(t.Context(), capture)
			require.Error(t, err)
			require.Empty(t, payloadID)
			require.ErrorContains(t, err, "lineage parent not found")
		})
	})

	t.Run("newVenueIngestionFlow", func(t *testing.T) {
		t.Parallel()

		t.Run("requires ingestion service", func(t *testing.T) {
			t.Parallel()

			flow, flowErr := newVenueIngestionFlow(nil, makeLineageService(t))
			require.Error(t, flowErr)
			require.Nil(t, flow)
		})

		t.Run("creates a lineage-enabled flow", func(t *testing.T) {
			t.Parallel()

			sharedStore := makeStore(t)
			lineageService, err := data.NewLineageService(data.LineageServiceDeps{
				Store:     sharedStore,
				BlobStore: sharedStore,
			})
			require.NoError(t, err)
			ingestionService, err := data.NewIngestionService(data.IngestionServiceDeps{
				InstrumentStore: sharedStore,
				CandleStore:     sharedStore,
				TradeStore:      sharedStore,
			})
			require.NoError(t, err)

			flow, flowErr := newVenueIngestionFlow(ingestionService, lineageService)
			require.NoError(t, flowErr)
			require.NotNil(t, flow)

			recorder, recErr := newHyperliquidRawEvidenceRecorder(lineageService)
			require.NoError(t, recErr)

			instrument := makeInstrument(t)
			rawPayloadID, recErr := recorder.RecordHyperliquidRawEvidence(
				t.Context(),
				makeCapture(t, &instrument),
			)
			require.NoError(t, recErr)

			readResult := venueedge.InstrumentReadResult{
				Instruments: []domain.Instrument{instrument},
				Metadata:    venueedge.ReadResultMetadata{RawPayloadIDs: []string{rawPayloadID}},
			}
			venue := &stubVenueForWiring{instrumentResult: readResult}

			persisted, flowErr := flow.IngestInstruments(
				t.Context(),
				venue,
				venueedge.InstrumentReadRequest{Venue: venueedge.HyperliquidPerpsVenueName},
			)
			require.NoError(t, flowErr)
			require.Len(t, persisted, 1)

			linkedIDs, linkedErr := lineageService.ListInstrumentRawPayloadIDs(t.Context(), instrument)
			require.NoError(t, linkedErr)
			require.Len(t, linkedIDs, 1)
			require.Equal(t, readResult.Metadata.RawPayloadIDs[0], linkedIDs[0])
		})

		t.Run("allows nil lineage sink", func(t *testing.T) {
			t.Parallel()

			sharedStore := makeStore(t)
			ingestionService, err := data.NewIngestionService(data.IngestionServiceDeps{
				InstrumentStore: sharedStore,
				CandleStore:     sharedStore,
				TradeStore:      sharedStore,
			})
			require.NoError(t, err)

			flow, flowErr := newVenueIngestionFlow(ingestionService, nil)
			require.NoError(t, flowErr)
			require.NotNil(t, flow)
		})
	})

	t.Run("rawPayloadInstrumentRef", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for nil instrument", func(t *testing.T) {
			t.Parallel()

			require.Nil(t, rawPayloadInstrumentRef(nil))
		})

		t.Run("converts instrument fields", func(t *testing.T) {
			t.Parallel()

			instrument := makeInstrument(t)
			ref := rawPayloadInstrumentRef(&instrument)
			require.NotNil(t, ref)
			require.Equal(t, instrument.Symbol, ref.Symbol)
			require.Equal(t, instrument.AssetClass, ref.AssetClass)
		})
	})
}

type stubVenueForWiring struct {
	instrumentResult venueedge.InstrumentReadResult
	instrumentErr    error
}

func (s *stubVenueForWiring) ReadInstruments(
	_ context.Context,
	_ venueedge.InstrumentReadRequest,
) (venueedge.InstrumentReadResult, error) {
	if s.instrumentErr != nil {
		return venueedge.InstrumentReadResult{}, s.instrumentErr
	}

	return s.instrumentResult, nil
}

func (s *stubVenueForWiring) ReadCandles(
	_ context.Context,
	_ venueedge.CandleReadRequest,
) (venueedge.CandleReadResult, error) {
	return venueedge.CandleReadResult{}, nil
}

func (s *stubVenueForWiring) ReadTrades(
	_ context.Context,
	_ venueedge.TradeReadRequest,
) (venueedge.TradeReadResult, error) {
	return venueedge.TradeReadResult{}, nil
}
