package data

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type fakeRawPayloadBlobStore struct {
	storedValue  RawPayloadBody
	bodyByRef    map[string][]byte
	storeErr     error
	readErr      error
	storedIDs    []string
	storedBodies [][]byte
}

func (s *fakeRawPayloadBlobStore) StoreRawPayloadBody(
	_ context.Context,
	payloadID string,
	body []byte,
) (RawPayloadBody, error) {
	s.storedIDs = append(s.storedIDs, payloadID)
	s.storedBodies = append(s.storedBodies, append([]byte(nil), body...))
	if s.storeErr != nil {
		return RawPayloadBody{}, s.storeErr
	}
	if s.bodyByRef == nil {
		s.bodyByRef = map[string][]byte{}
	}
	s.bodyByRef[s.storedValue.Ref] = append([]byte(nil), body...)
	return s.storedValue, nil
}

func (s *fakeRawPayloadBlobStore) ReadRawPayloadBody(_ context.Context, ref string) ([]byte, error) {
	if s.readErr != nil {
		return nil, s.readErr
	}
	return append([]byte(nil), s.bodyByRef[ref]...), nil
}

type fakeLineageStore struct {
	upsertedIngestionRuns     []IngestionRun
	upsertedRawPayloads       []RawVenuePayload
	upsertedNormalizationRuns []NormalizationRun
	upsertedDataBatches       []DataBatch
	auditBatchIDs             []string
	replayCandleBatchIDs      []string
	replayTradeBatchIDs       []string
	linkedInstrumentPayloads  []string
	linkedCandlePayloads      []string
	linkedTradePayloads       []string
	auditValue                DataBatchAudit
	replayCandlesValue        []ReplayCandle
	replayTradesValue         []ReplayTrade
	instrumentRawPayloadIDs   []string
	candleRawPayloadIDs       []string
	tradeRawPayloadIDs        []string
	upsertIngestionRunErr     error
	upsertRawPayloadErr       error
	upsertNormalizationErr    error
	upsertDataBatchErr        error
	auditErr                  error
	replayCandlesErr          error
	replayTradesErr           error
	linkInstrumentErr         error
	linkCandleErr             error
	linkTradeErr              error
	listInstrumentErr         error
	listCandleErr             error
	listTradeErr              error
}

func (s *fakeLineageStore) UpsertIngestionRun(_ context.Context, run IngestionRun) (IngestionRun, error) {
	s.upsertedIngestionRuns = append(s.upsertedIngestionRuns, run)
	if s.upsertIngestionRunErr != nil {
		return IngestionRun{}, s.upsertIngestionRunErr
	}
	return run, nil
}

func (s *fakeLineageStore) UpsertRawVenuePayload(_ context.Context, payload RawVenuePayload) (RawVenuePayload, error) {
	s.upsertedRawPayloads = append(s.upsertedRawPayloads, payload)
	if s.upsertRawPayloadErr != nil {
		return RawVenuePayload{}, s.upsertRawPayloadErr
	}
	return payload, nil
}

func (s *fakeLineageStore) UpsertNormalizationRun(_ context.Context, run NormalizationRun) (NormalizationRun, error) {
	s.upsertedNormalizationRuns = append(s.upsertedNormalizationRuns, run)
	if s.upsertNormalizationErr != nil {
		return NormalizationRun{}, s.upsertNormalizationErr
	}
	return run, nil
}

func (s *fakeLineageStore) UpsertDataBatch(_ context.Context, batch DataBatch) (DataBatch, error) {
	s.upsertedDataBatches = append(s.upsertedDataBatches, batch)
	if s.upsertDataBatchErr != nil {
		return DataBatch{}, s.upsertDataBatchErr
	}
	return batch, nil
}

func (s *fakeLineageStore) LinkRawPayloadToInstrument(
	_ context.Context,
	rawPayloadID string,
	_ domain.Instrument,
) error {
	s.linkedInstrumentPayloads = append(s.linkedInstrumentPayloads, rawPayloadID)
	return s.linkInstrumentErr
}

func (s *fakeLineageStore) LinkRawPayloadToCandle(
	_ context.Context,
	rawPayloadID string,
	_ domain.Candle,
) error {
	s.linkedCandlePayloads = append(s.linkedCandlePayloads, rawPayloadID)
	return s.linkCandleErr
}

func (s *fakeLineageStore) LinkRawPayloadToTrade(
	_ context.Context,
	rawPayloadID string,
	_ domain.Trade,
) error {
	s.linkedTradePayloads = append(s.linkedTradePayloads, rawPayloadID)
	return s.linkTradeErr
}

func (s *fakeLineageStore) ListInstrumentRawPayloadIDs(
	_ context.Context,
	_ domain.Instrument,
) ([]string, error) {
	if s.listInstrumentErr != nil {
		return nil, s.listInstrumentErr
	}
	return s.instrumentRawPayloadIDs, nil
}

func (s *fakeLineageStore) ListCandleRawPayloadIDs(
	_ context.Context,
	_ domain.Candle,
) ([]string, error) {
	if s.listCandleErr != nil {
		return nil, s.listCandleErr
	}
	return s.candleRawPayloadIDs, nil
}

func (s *fakeLineageStore) ListTradeRawPayloadIDs(
	_ context.Context,
	_ domain.Trade,
) ([]string, error) {
	if s.listTradeErr != nil {
		return nil, s.listTradeErr
	}
	return s.tradeRawPayloadIDs, nil
}

func (s *fakeLineageStore) GetDataBatchAudit(_ context.Context, batchID string) (DataBatchAudit, error) {
	s.auditBatchIDs = append(s.auditBatchIDs, batchID)
	if s.auditErr != nil {
		return DataBatchAudit{}, s.auditErr
	}
	return s.auditValue, nil
}

func (s *fakeLineageStore) ReplayCandlesByDataBatch(_ context.Context, batchID string) ([]ReplayCandle, error) {
	s.replayCandleBatchIDs = append(s.replayCandleBatchIDs, batchID)
	if s.replayCandlesErr != nil {
		return nil, s.replayCandlesErr
	}
	return s.replayCandlesValue, nil
}

func (s *fakeLineageStore) ReplayTradesByDataBatch(_ context.Context, batchID string) ([]ReplayTrade, error) {
	s.replayTradeBatchIDs = append(s.replayTradeBatchIDs, batchID)
	if s.replayTradesErr != nil {
		return nil, s.replayTradesErr
	}
	return s.replayTradesValue, nil
}

func TestLineageService(t *testing.T) {
	t.Parallel()

	fake := faker.New()

	randomWord := func(prefix string) string {
		return prefix + "-" + strings.ToLower(fake.Lorem().Word())
	}

	randomTime := func() time.Time {
		return time.Date(
			fake.IntBetween(2021, 2032),
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 28),
			fake.IntBetween(0, 23),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 999999999),
			time.FixedZone(randomWord("zone"), fake.IntBetween(-11, 12)*3600),
		)
	}

	makeStore := func() *fakeLineageStore { return &fakeLineageStore{} }
	makeBlobStore := func() *fakeRawPayloadBlobStore {
		return &fakeRawPayloadBlobStore{
			storedValue: RawPayloadBody{Ref: randomWord("ref"), Hash: randomWord("hash")},
			bodyByRef:   map[string][]byte{},
		}
	}
	makeService := func(t *testing.T, store *fakeLineageStore, blobStore *fakeRawPayloadBlobStore) *LineageService {
		t.Helper()
		svc, err := NewLineageService(LineageServiceDeps{Store: store, BlobStore: blobStore})
		require.NoError(t, err)
		return svc
	}

	makeInstrument := func() domain.Instrument {
		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      domain.Venue(randomWord("venue")),
			Symbol:     domain.Symbol(strings.ToUpper(randomWord("symbol"))),
			AssetClass: domain.AssetClassCrypto,
			Active:     true,
		})
		require.NoError(t, err)
		return instrument
	}

	makeIngestionRun := func() IngestionRun {
		run, err := NewIngestionRun(IngestionRunParams{
			ID:        randomWord("run"),
			Source:    randomWord("source"),
			Venue:     domain.Venue(randomWord("venue")),
			Status:    IngestionRunStatusStarted,
			StartedAt: randomTime(),
		})
		require.NoError(t, err)
		return run
	}

	makeRawPayload := func() RawVenuePayload {
		requestAt := randomTime()
		payload, err := NewRawVenuePayload(RawVenuePayloadParams{
			ID:                 randomWord("payload"),
			IngestionRunID:     randomWord("run"),
			Source:             randomWord("source"),
			Venue:              domain.Venue(randomWord("venue")),
			Endpoint:           "/info",
			RequestType:        randomWord("request"),
			RequestPayloadHash: randomWord("request-hash"),
			RequestAt:          requestAt,
			ResponseAt:         requestAt.Add(time.Minute),
			HTTPStatus:         200,
			ResponseBody:       []byte(randomWord("body")),
			ReceivedAt:         requestAt.Add(2 * time.Minute),
		})
		require.NoError(t, err)
		return payload
	}

	makeNormalizationRun := func() NormalizationRun {
		run, err := NewNormalizationRun(NormalizationRunParams{
			ID:                   randomWord("normalization"),
			RawPayloadIDs:        []string{randomWord("payload")},
			Status:               NormalizationRunStatusStarted,
			StartedAt:            randomTime(),
			RecordKind:           LineageRecordKindCandle,
			SourceRecordCount:    fake.IntBetween(0, 1000),
			CanonicalRecordCount: fake.IntBetween(0, 1000),
		})
		require.NoError(t, err)
		return run
	}

	makeDataBatch := func() DataBatch {
		start := randomTime()
		batch, err := NewDataBatch(DataBatchParams{
			ID:                 randomWord("batch"),
			NormalizationRunID: randomWord("normalization"),
			Venue:              domain.Venue(randomWord("venue")),
			RecordKind:         LineageRecordKindTrade,
			TimeRange:          domain.TimeRange{Start: start, End: start.Add(time.Minute)},
			Quality:            domain.DataQualityValidated,
			RecordCount:        fake.IntBetween(0, 1000),
		})
		require.NoError(t, err)
		return batch
	}

	makeCandle := func() domain.Candle {
		instrument := makeInstrument()
		start := randomTime().UTC()
		timeRange, err := domain.NewTimeRange(start, start.Add(time.Minute))
		require.NoError(t, err)
		provenance, err := domain.NewSourceProvenance(randomWord("source"), randomWord("record"))
		require.NoError(t, err)
		candle, err := domain.NewCandle(domain.CandleParams{
			Instrument: instrument,
			Timeframe:  domain.Timeframe1m,
			TimeRange:  timeRange,
			Open:       fake.Float64(2, 1, 1000),
			High:       fake.Float64(2, 1, 1000),
			Low:        fake.Float64(2, 0, 1000),
			Close:      fake.Float64(2, 1, 1000),
			Volume:     fake.Float64(2, 0, 1000),
			Quality:    domain.DataQualityValidated,
			Provenance: provenance,
		})
		require.NoError(t, err)
		return candle
	}

	makeTrade := func() domain.Trade {
		instrument := makeInstrument()
		provenance, err := domain.NewSourceProvenance(randomWord("source"), randomWord("record"))
		require.NoError(t, err)
		trade, err := domain.NewTrade(domain.TradeParams{
			Instrument: instrument,
			EventTime:  randomTime(),
			Price:      fake.Float64(2, 1, 1000),
			Size:       fake.Float64(2, 0, 1000),
			Quality:    domain.DataQualityRaw,
			Provenance: provenance,
		})
		require.NoError(t, err)
		return trade
	}

	t.Run("NewLineageService requires store and blob store", func(t *testing.T) {
		t.Parallel()
		_, err := NewLineageService(LineageServiceDeps{})
		require.Error(t, err)
	})

	t.Run("RecordIngestionRun canonicalizes ids", func(t *testing.T) {
		t.Parallel()
		store := makeStore()
		svc := makeService(t, store, makeBlobStore())
		run := makeIngestionRun()
		run.ID = "  " + run.ID + "  "
		persisted, err := svc.RecordIngestionRun(t.Context(), run)
		require.NoError(t, err)
		require.Equal(t, persisted, store.upsertedIngestionRuns[0])
	})

	t.Run("RecordRawVenuePayload writes blob before persistence and hydrates body on return", func(t *testing.T) {
		t.Parallel()
		store := makeStore()
		blobStore := makeBlobStore()
		svc := makeService(t, store, blobStore)
		payload := makeRawPayload()

		persisted, err := svc.RecordRawVenuePayload(t.Context(), payload)
		require.NoError(t, err)
		require.Len(t, blobStore.storedIDs, 1)
		require.Equal(t, blobStore.storedValue.Ref, store.upsertedRawPayloads[0].PayloadBodyRef)
		require.Equal(t, blobStore.storedValue.Hash, store.upsertedRawPayloads[0].ResponseBodyHash)
		require.Nil(t, store.upsertedRawPayloads[0].ResponseBody)
		require.Equal(t, payload.ResponseBody, persisted.ResponseBody)
	})

	t.Run("RecordRawVenuePayload returns wrapped store errors", func(t *testing.T) {
		t.Parallel()
		store := makeStore()
		store.upsertRawPayloadErr = ErrLineageParentNotFound
		svc := makeService(t, store, makeBlobStore())
		_, err := svc.RecordRawVenuePayload(t.Context(), makeRawPayload())
		require.ErrorIs(t, err, ErrLineageParentNotFound)
	})

	t.Run("RecordNormalizationRun and RecordDataBatch delegate canonical values", func(t *testing.T) {
		t.Parallel()
		store := makeStore()
		svc := makeService(t, store, makeBlobStore())

		normalizationRun := makeNormalizationRun()
		persistedRun, err := svc.RecordNormalizationRun(t.Context(), normalizationRun)
		require.NoError(t, err)
		require.Equal(t, persistedRun, store.upsertedNormalizationRuns[0])

		batch := makeDataBatch()
		persistedBatch, err := svc.RecordDataBatch(t.Context(), batch)
		require.NoError(t, err)
		require.Equal(t, persistedBatch, store.upsertedDataBatches[0])
	})

	t.Run("link and list raw payload ids", func(t *testing.T) {
		t.Parallel()
		store := makeStore()
		store.instrumentRawPayloadIDs = []string{randomWord("payload")}
		store.candleRawPayloadIDs = []string{randomWord("payload")}
		store.tradeRawPayloadIDs = []string{randomWord("payload")}
		svc := makeService(t, store, makeBlobStore())

		instrument := makeInstrument()
		candle := makeCandle()
		trade := makeTrade()

		require.NoError(t, svc.LinkRawPayloadToInstrument(t.Context(), randomWord("raw"), instrument))
		require.NoError(t, svc.LinkRawPayloadToCandle(t.Context(), randomWord("raw"), candle))
		require.NoError(t, svc.LinkRawPayloadToTrade(t.Context(), randomWord("raw"), trade))

		instrumentIDs, err := svc.ListInstrumentRawPayloadIDs(t.Context(), instrument)
		require.NoError(t, err)
		require.Equal(t, store.instrumentRawPayloadIDs, instrumentIDs)

		candleIDs, err := svc.ListCandleRawPayloadIDs(t.Context(), candle)
		require.NoError(t, err)
		require.Equal(t, store.candleRawPayloadIDs, candleIDs)

		tradeIDs, err := svc.ListTradeRawPayloadIDs(t.Context(), trade)
		require.NoError(t, err)
		require.Equal(t, store.tradeRawPayloadIDs, tradeIDs)
	})

	t.Run("GetDataBatchAudit hydrates payload bodies from blob store", func(t *testing.T) {
		t.Parallel()
		store := makeStore()
		blobStore := makeBlobStore()
		body := []byte(randomWord("body"))
		blobStore.bodyByRef[blobStore.storedValue.Ref] = body
		payload := makeRawPayload()
		payload.PayloadBodyRef = blobStore.storedValue.Ref
		payload.ResponseBodyHash = blobStore.storedValue.Hash
		payload.ResponseBody = nil
		store.auditValue = DataBatchAudit{
			Batch:            makeDataBatch(),
			NormalizationRun: makeNormalizationRun(),
			RawPayloads:      []RawVenuePayloadAudit{{Payload: payload}},
		}
		svc := makeService(t, store, blobStore)

		audit, err := svc.GetDataBatchAudit(t.Context(), "  "+randomWord("batch")+"  ")
		require.NoError(t, err)
		require.Equal(t, body, audit.RawPayloads[0].Payload.ResponseBody)
	})

	t.Run("Replay delegates batch ids", func(t *testing.T) {
		t.Parallel()
		store := makeStore()
		store.replayCandlesValue = []ReplayCandle{{Identity: uint64(fake.IntBetween(1, 1000))}}
		store.replayTradesErr = errors.New(randomWord("replay-error"))
		svc := makeService(t, store, makeBlobStore())

		candles, err := svc.ReplayCandlesByDataBatch(t.Context(), "  "+randomWord("batch")+"  ")
		require.NoError(t, err)
		require.Equal(t, store.replayCandlesValue, candles)

		_, err = svc.ReplayTradesByDataBatch(t.Context(), randomWord("batch"))
		require.ErrorIs(t, err, store.replayTradesErr)
	})
}
