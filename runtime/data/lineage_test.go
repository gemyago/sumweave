package data

import (
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestLineageRecords(t *testing.T) {
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

	makeTimeRange := func() domain.TimeRange {
		start := randomTime()
		rangeValue, err := domain.NewTimeRange(start, start.Add(time.Duration(fake.IntBetween(1, 180))*time.Minute))
		require.NoError(t, err)
		return rangeValue
	}

	t.Run("NewIngestionRun", func(t *testing.T) {
		t.Parallel()

		t.Run("canonicalizes fields", func(t *testing.T) {
			t.Parallel()

			startedAt := randomTime()
			completedAt := startedAt.Add(time.Duration(fake.IntBetween(1, 180)) * time.Minute)

			run, err := NewIngestionRun(IngestionRunParams{
				ID:           "  " + randomWord("run") + "  ",
				Source:       "  " + randomWord("source") + "  ",
				Venue:        domain.Venue("  " + randomWord("venue") + "  "),
				Status:       IngestionRunStatus("  STARTED  "),
				StartedAt:    startedAt,
				CompletedAt:  completedAt,
				RecordCount:  fake.IntBetween(0, 1000),
				ErrorSummary: "  " + randomWord("error") + "  ",
			})
			require.NoError(t, err)

			require.Equal(t, strings.TrimSpace(run.ID), run.ID)
			require.Equal(t, strings.TrimSpace(run.Source), run.Source)
			require.Equal(t, domain.Venue(strings.TrimSpace(run.Venue.String())), run.Venue)
			require.Equal(t, IngestionRunStatusStarted, run.Status)
			require.Equal(t, startedAt.UTC(), run.StartedAt)
			require.Equal(t, completedAt.UTC(), run.CompletedAt)
			require.Equal(t, strings.TrimSpace(run.ErrorSummary), run.ErrorSummary)
		})

		t.Run("rejects missing required identity", func(t *testing.T) {
			t.Parallel()

			_, err := NewIngestionRun(IngestionRunParams{
				Source:    randomWord("source"),
				Venue:     domain.Venue(randomWord("venue")),
				Status:    IngestionRunStatusStarted,
				StartedAt: randomTime(),
			})
			require.ErrorIs(t, err, ErrValidation)
		})
	})

	t.Run("NewRawVenuePayload", func(t *testing.T) {
		t.Parallel()

		t.Run("canonicalizes fields", func(t *testing.T) {
			t.Parallel()

			receivedAt := randomTime()
			payloadBody := []byte("  " + randomWord("body") + "  ")
			metadata := map[string]string{
				"  " + randomWord("header") + "  ": "  " + randomWord("value") + "  ",
			}

			payload, err := NewRawVenuePayload(RawVenuePayloadParams{
				ID:             "  " + randomWord("payload") + "  ",
				IngestionRunID: "  " + randomWord("run") + "  ",
				Source:         "  " + randomWord("source") + "  ",
				Venue:          domain.Venue("  " + randomWord("venue") + "  "),
				ContentType:    "  application/json  ",
				Body:           payloadBody,
				Checksum:       "  " + randomWord("checksum") + "  ",
				ReceivedAt:     receivedAt,
				RequestKey:     "  " + randomWord("request") + "  ",
				SourceRecordID: "  " + randomWord("record") + "  ",
				Metadata:       metadata,
			})
			require.NoError(t, err)

			require.Equal(t, strings.TrimSpace(payload.ID), payload.ID)
			require.Equal(t, strings.TrimSpace(payload.IngestionRunID), payload.IngestionRunID)
			require.Equal(t, strings.TrimSpace(payload.Source), payload.Source)
			require.Equal(t, domain.Venue(strings.TrimSpace(payload.Venue.String())), payload.Venue)
			require.Equal(t, "application/json", payload.ContentType)
			require.Equal(t, receivedAt.UTC(), payload.ReceivedAt)
			require.Equal(t, strings.TrimSpace(payload.RequestKey), payload.RequestKey)
			require.Equal(t, strings.TrimSpace(payload.SourceRecordID), payload.SourceRecordID)
			require.Len(t, payload.Metadata, 1)
			for key, value := range payload.Metadata {
				require.Equal(t, strings.TrimSpace(key), key)
				require.Equal(t, strings.TrimSpace(value), value)
			}

			payloadBody[0] = 'x'
			require.NotEqual(t, payloadBody, payload.Body)
		})

		t.Run("rejects missing parent identity", func(t *testing.T) {
			t.Parallel()

			_, err := NewRawVenuePayload(RawVenuePayloadParams{
				ID:          randomWord("payload"),
				Source:      randomWord("source"),
				Venue:       domain.Venue(randomWord("venue")),
				ContentType: "application/json",
				Body:        []byte(randomWord("body")),
				Checksum:    randomWord("checksum"),
				ReceivedAt:  randomTime(),
			})
			require.ErrorIs(t, err, ErrValidation)
		})
	})

	t.Run("NewNormalizationRun", func(t *testing.T) {
		t.Parallel()

		t.Run("canonicalizes fields", func(t *testing.T) {
			t.Parallel()

			startedAt := randomTime()
			completedAt := startedAt.Add(time.Duration(fake.IntBetween(1, 180)) * time.Minute)

			run, err := NewNormalizationRun(NormalizationRunParams{
				ID: "  " + randomWord("normalization") + "  ",
				RawPayloadIDs: []string{
					"  " + randomWord("payload-a") + "  ",
					"  " + randomWord("payload-b") + "  ",
				},
				Status:               NormalizationRunStatus("  SUCCEEDED  "),
				StartedAt:            startedAt,
				CompletedAt:          completedAt,
				RecordKind:           LineageRecordKind("  TRADE  "),
				SourceRecordCount:    fake.IntBetween(0, 1000),
				CanonicalRecordCount: fake.IntBetween(0, 1000),
				ErrorSummary:         "  " + randomWord("error") + "  ",
			})
			require.NoError(t, err)

			require.Equal(t, NormalizationRunStatusSucceeded, run.Status)
			require.Equal(t, LineageRecordKindTrade, run.RecordKind)
			require.Equal(t, startedAt.UTC(), run.StartedAt)
			require.Equal(t, completedAt.UTC(), run.CompletedAt)
			require.Len(t, run.RawPayloadIDs, 2)
			require.Equal(t, strings.TrimSpace(run.ErrorSummary), run.ErrorSummary)
		})

		t.Run("rejects missing raw payload links", func(t *testing.T) {
			t.Parallel()

			_, err := NewNormalizationRun(NormalizationRunParams{
				ID:         randomWord("normalization"),
				Status:     NormalizationRunStatusStarted,
				StartedAt:  randomTime(),
				RecordKind: LineageRecordKindCandle,
			})
			require.ErrorIs(t, err, ErrValidation)
		})
	})

	t.Run("NewDataBatch", func(t *testing.T) {
		t.Parallel()

		t.Run("canonicalizes fields", func(t *testing.T) {
			t.Parallel()

			batch, err := NewDataBatch(DataBatchParams{
				ID:                 "  " + randomWord("batch") + "  ",
				NormalizationRunID: "  " + randomWord("normalization") + "  ",
				Venue:              domain.Venue("  " + randomWord("venue") + "  "),
				Instrument: &BatchInstrumentRef{
					Symbol:     domain.Symbol("  " + strings.ToUpper(randomWord("symbol")) + "  "),
					AssetClass: domain.AssetClass("  CRYPTO  "),
				},
				RecordKind:  LineageRecordKind("  CANDLE  "),
				TimeRange:   makeTimeRange(),
				Quality:     domain.DataQuality("  VALIDATED  "),
				RecordCount: fake.IntBetween(0, 1000),
				Summary:     "  " + randomWord("summary") + "  ",
			})
			require.NoError(t, err)

			require.Equal(t, LineageRecordKindCandle, batch.RecordKind)
			require.Equal(t, domain.DataQualityValidated, batch.Quality)
			require.NotNil(t, batch.Instrument)
			require.Equal(t, strings.TrimSpace(batch.Instrument.Symbol.String()), batch.Instrument.Symbol.String())
			require.Equal(t, strings.TrimSpace(batch.Summary), batch.Summary)
			require.Equal(t, batch.TimeRange.Start.UTC(), batch.TimeRange.Start)
			require.Equal(t, batch.TimeRange.End.UTC(), batch.TimeRange.End)
		})

		t.Run("rejects missing parent identity", func(t *testing.T) {
			t.Parallel()

			_, err := NewDataBatch(DataBatchParams{
				ID:         randomWord("batch"),
				Venue:      domain.Venue(randomWord("venue")),
				RecordKind: LineageRecordKindCandle,
				TimeRange:  makeTimeRange(),
				Quality:    domain.DataQualityValidated,
			})
			require.ErrorIs(t, err, ErrValidation)
		})
	})
}
