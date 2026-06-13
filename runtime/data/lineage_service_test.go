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

type fakeLineageStore struct {
	upsertedIngestionRuns     []IngestionRun
	upsertedRawPayloads       []RawVenuePayload
	upsertedNormalizationRuns []NormalizationRun
	upsertedDataBatches       []DataBatch
	auditBatchIDs             []string
	replayCandleBatchIDs      []string
	replayTradeBatchIDs       []string
	auditValue                DataBatchAudit
	replayCandlesValue        []ReplayCandle
	replayTradesValue         []ReplayTrade
	upsertIngestionRunErr     error
	upsertRawPayloadErr       error
	upsertNormalizationErr    error
	upsertDataBatchErr        error
	auditErr                  error
	replayCandlesErr          error
	replayTradesErr           error
}

func (s *fakeLineageStore) UpsertIngestionRun(
	_ context.Context,
	run IngestionRun,
) (IngestionRun, error) {
	s.upsertedIngestionRuns = append(s.upsertedIngestionRuns, run)
	if s.upsertIngestionRunErr != nil {
		return IngestionRun{}, s.upsertIngestionRunErr
	}

	return run, nil
}

func (s *fakeLineageStore) UpsertRawVenuePayload(
	_ context.Context,
	payload RawVenuePayload,
) (RawVenuePayload, error) {
	s.upsertedRawPayloads = append(s.upsertedRawPayloads, payload)
	if s.upsertRawPayloadErr != nil {
		return RawVenuePayload{}, s.upsertRawPayloadErr
	}

	return payload, nil
}

func (s *fakeLineageStore) UpsertNormalizationRun(
	_ context.Context,
	run NormalizationRun,
) (NormalizationRun, error) {
	s.upsertedNormalizationRuns = append(s.upsertedNormalizationRuns, run)
	if s.upsertNormalizationErr != nil {
		return NormalizationRun{}, s.upsertNormalizationErr
	}

	return run, nil
}

func (s *fakeLineageStore) UpsertDataBatch(
	_ context.Context,
	batch DataBatch,
) (DataBatch, error) {
	s.upsertedDataBatches = append(s.upsertedDataBatches, batch)
	if s.upsertDataBatchErr != nil {
		return DataBatch{}, s.upsertDataBatchErr
	}

	return batch, nil
}

func (s *fakeLineageStore) GetDataBatchAudit(_ context.Context, batchID string) (DataBatchAudit, error) {
	s.auditBatchIDs = append(s.auditBatchIDs, batchID)
	if s.auditErr != nil {
		return DataBatchAudit{}, s.auditErr
	}

	return s.auditValue, nil
}

func (s *fakeLineageStore) ReplayCandlesByDataBatch(
	_ context.Context,
	batchID string,
) ([]ReplayCandle, error) {
	s.replayCandleBatchIDs = append(s.replayCandleBatchIDs, batchID)
	if s.replayCandlesErr != nil {
		return nil, s.replayCandlesErr
	}

	return s.replayCandlesValue, nil
}

func (s *fakeLineageStore) ReplayTradesByDataBatch(
	_ context.Context,
	batchID string,
) ([]ReplayTrade, error) {
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

	makeStore := func() *fakeLineageStore {
		return &fakeLineageStore{}
	}

	makeService := func(t *testing.T, store *fakeLineageStore) *LineageService {
		t.Helper()

		svc, err := NewLineageService(LineageServiceDeps{
			Store: store,
		})
		require.NoError(t, err)
		return svc
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
		payload, err := NewRawVenuePayload(RawVenuePayloadParams{
			ID:             randomWord("payload"),
			IngestionRunID: randomWord("run"),
			Source:         randomWord("source"),
			Venue:          domain.Venue(randomWord("venue")),
			ContentType:    "application/json",
			Body:           []byte(randomWord("body")),
			Checksum:       randomWord("checksum"),
			ReceivedAt:     randomTime(),
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
			TimeRange: domain.TimeRange{
				Start: start,
				End:   start.Add(time.Duration(fake.IntBetween(1, 180)) * time.Minute),
			},
			Quality:     domain.DataQualityValidated,
			RecordCount: fake.IntBetween(0, 1000),
		})
		require.NoError(t, err)
		return batch
	}

	t.Run("NewLineageService", func(t *testing.T) {
		t.Parallel()

		_, err := NewLineageService(LineageServiceDeps{})
		require.Error(t, err)
	})

	t.Run("RecordIngestionRun", func(t *testing.T) {
		t.Parallel()

		store := makeStore()
		svc := makeService(t, store)

		run := makeIngestionRun()
		run.ID = "  " + run.ID + "  "

		persisted, err := svc.RecordIngestionRun(t.Context(), run)
		require.NoError(t, err)
		require.Equal(t, persisted, store.upsertedIngestionRuns[0])
		require.Equal(t, strings.TrimSpace(run.ID), persisted.ID)
	})

	t.Run("RecordRawVenuePayload", func(t *testing.T) {
		t.Parallel()

		store := makeStore()
		store.upsertRawPayloadErr = ErrLineageParentNotFound
		svc := makeService(t, store)

		_, err := svc.RecordRawVenuePayload(t.Context(), makeRawPayload())
		require.ErrorIs(t, err, ErrLineageParentNotFound)
	})

	t.Run("RecordNormalizationRun", func(t *testing.T) {
		t.Parallel()

		store := makeStore()
		svc := makeService(t, store)

		run := makeNormalizationRun()
		run.RawPayloadIDs = []string{"  " + run.RawPayloadIDs[0] + "  "}

		persisted, err := svc.RecordNormalizationRun(t.Context(), run)
		require.NoError(t, err)
		require.Equal(t, persisted, store.upsertedNormalizationRuns[0])
		require.Equal(t, strings.TrimSpace(run.RawPayloadIDs[0]), persisted.RawPayloadIDs[0])
	})

	t.Run("RecordDataBatch", func(t *testing.T) {
		t.Parallel()

		store := makeStore()
		svc := makeService(t, store)

		batch := makeDataBatch()
		persisted, err := svc.RecordDataBatch(t.Context(), batch)
		require.NoError(t, err)
		require.Equal(t, persisted, store.upsertedDataBatches[0])
	})

	t.Run("GetDataBatchAudit", func(t *testing.T) {
		t.Parallel()

		store := makeStore()
		store.auditValue = DataBatchAudit{Batch: makeDataBatch(), NormalizationRun: makeNormalizationRun()}
		svc := makeService(t, store)

		batchID := "  " + randomWord("batch") + "  "
		audit, err := svc.GetDataBatchAudit(t.Context(), batchID)
		require.NoError(t, err)
		require.Equal(t, store.auditValue, audit)
		require.Equal(t, []string{strings.TrimSpace(batchID)}, store.auditBatchIDs)
	})

	t.Run("ReplayCandlesByDataBatch", func(t *testing.T) {
		t.Parallel()

		store := makeStore()
		store.replayCandlesValue = []ReplayCandle{{Identity: uint64(fake.IntBetween(1, 1000))}}
		svc := makeService(t, store)

		batchID := "  " + randomWord("batch") + "  "
		replayRows, err := svc.ReplayCandlesByDataBatch(t.Context(), batchID)
		require.NoError(t, err)
		require.Equal(t, store.replayCandlesValue, replayRows)
		require.Equal(t, []string{strings.TrimSpace(batchID)}, store.replayCandleBatchIDs)
	})

	t.Run("ReplayTradesByDataBatch", func(t *testing.T) {
		t.Parallel()

		store := makeStore()
		store.replayTradesErr = errors.New(randomWord("replay-error"))
		svc := makeService(t, store)

		_, err := svc.ReplayTradesByDataBatch(t.Context(), randomWord("batch"))
		require.ErrorIs(t, err, store.replayTradesErr)
	})
}
