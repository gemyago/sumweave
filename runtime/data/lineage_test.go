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
				Status:       IngestionRunStatus("  SUCCEEDED  "),
				StartedAt:    startedAt,
				CompletedAt:  &completedAt,
				RecordCount:  fake.IntBetween(0, 1000),
				ErrorSummary: "  " + randomWord("error") + "  ",
			})
			require.NoError(t, err)

			require.Equal(t, strings.TrimSpace(run.ID), run.ID)
			require.Equal(t, strings.TrimSpace(run.Source), run.Source)
			require.Equal(t, domain.Venue(strings.TrimSpace(run.Venue.String())), run.Venue)
			require.Equal(t, IngestionRunStatusSucceeded, run.Status)
			require.Equal(t, startedAt, run.StartedAt)
			require.NotNil(t, run.CompletedAt)
			require.Equal(t, completedAt, *run.CompletedAt)
			require.Equal(t, strings.TrimSpace(run.ErrorSummary), run.ErrorSummary)
		})

		t.Run("requires completion only for terminal status", func(t *testing.T) {
			t.Parallel()

			startedAt := randomTime()
			base := IngestionRunParams{
				ID:        randomWord("run"),
				Source:    randomWord("source"),
				Venue:     domain.Venue(randomWord("venue")),
				StartedAt: startedAt,
			}

			started := base
			started.Status = IngestionRunStatusStarted
			completedAt := startedAt.Add(time.Minute)
			started.CompletedAt = &completedAt
			_, err := NewIngestionRun(started)
			require.ErrorIs(t, err, ErrValidation)

			succeeded := base
			succeeded.Status = IngestionRunStatusSucceeded
			_, err = NewIngestionRun(succeeded)
			require.ErrorIs(t, err, ErrValidation)

			zero := time.Time{}
			succeeded.CompletedAt = &zero
			_, err = NewIngestionRun(succeeded)
			require.ErrorIs(t, err, ErrValidation)
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

			requestAt := randomTime()
			responseAt := requestAt.Add(time.Minute)
			receivedAt := responseAt.Add(time.Minute)
			payloadBody := []byte("  " + randomWord("body") + "  ")
			metadata := map[string]string{
				"  " + randomWord("header") + "  ": "  " + randomWord("value") + "  ",
			}

			payload, err := NewRawVenuePayload(RawVenuePayloadParams{
				ID:                 "  " + randomWord("payload") + "  ",
				IngestionRunID:     "  " + randomWord("run") + "  ",
				Source:             "  " + randomWord("source") + "  ",
				Venue:              domain.Venue("  " + randomWord("venue") + "  "),
				Endpoint:           "  /info  ",
				RequestType:        "  " + randomWord("request") + "  ",
				RequestPayloadHash: "  " + randomWord("request-hash") + "  ",
				RequestMetadata:    metadata,
				RequestAt:          requestAt,
				ResponseAt:         responseAt,
				HTTPStatus:         200,
				ResponseBody:       payloadBody,
				EntityHint:         "  " + randomWord("entity") + "  ",
				ReceivedAt:         receivedAt,
			})
			require.NoError(t, err)

			require.Equal(t, strings.TrimSpace(payload.ID), payload.ID)
			require.Equal(t, strings.TrimSpace(payload.IngestionRunID), payload.IngestionRunID)
			require.Equal(t, strings.TrimSpace(payload.Source), payload.Source)
			require.Equal(t, domain.Venue(strings.TrimSpace(payload.Venue.String())), payload.Venue)
			require.Equal(t, "/info", payload.Endpoint)
			require.Equal(t, strings.TrimSpace(payload.RequestType), payload.RequestType)
			require.Equal(t, strings.TrimSpace(payload.RequestPayloadHash), payload.RequestPayloadHash)
			require.Equal(t, requestAt, payload.RequestAt)
			require.Equal(t, responseAt, payload.ResponseAt)
			require.Equal(t, receivedAt, payload.ReceivedAt)
			require.Equal(t, strings.TrimSpace(payload.EntityHint), payload.EntityHint)
			require.Len(t, payload.RequestMetadata, 1)
			for key, value := range payload.RequestMetadata {
				require.Equal(t, strings.TrimSpace(key), key)
				require.Equal(t, strings.TrimSpace(value), value)
			}

			payloadBody[0] = 'x'
			require.NotEqual(t, payloadBody, payload.ResponseBody)
		})

		t.Run("rejects missing request payload hash", func(t *testing.T) {
			t.Parallel()

			requestAt := randomTime()
			_, err := NewRawVenuePayload(RawVenuePayloadParams{
				ID:           randomWord("payload"),
				Source:       randomWord("source"),
				Venue:        domain.Venue(randomWord("venue")),
				Endpoint:     "/info",
				RequestType:  randomWord("request"),
				RequestAt:    requestAt,
				ResponseAt:   requestAt.Add(time.Minute),
				HTTPStatus:   200,
				ResponseBody: []byte(randomWord("body")),
				ReceivedAt:   requestAt.Add(2 * time.Minute),
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
				CompletedAt:          &completedAt,
				RecordKind:           LineageRecordKind("  TRADE  "),
				SourceRecordCount:    fake.IntBetween(0, 1000),
				CanonicalRecordCount: fake.IntBetween(0, 1000),
				ErrorSummary:         "  " + randomWord("error") + "  ",
			})
			require.NoError(t, err)

			require.Equal(t, NormalizationRunStatusSucceeded, run.Status)
			require.Equal(t, LineageRecordKindTrade, run.RecordKind)
			require.Equal(t, startedAt, run.StartedAt)
			require.NotNil(t, run.CompletedAt)
			require.Equal(t, completedAt, *run.CompletedAt)
			require.Len(t, run.RawPayloadIDs, 2)
			require.Equal(t, strings.TrimSpace(run.ErrorSummary), run.ErrorSummary)
		})

		t.Run("requires completion only for terminal status", func(t *testing.T) {
			t.Parallel()

			startedAt := randomTime()
			base := NormalizationRunParams{
				ID:            randomWord("normalization"),
				RawPayloadIDs: []string{randomWord("payload")},
				StartedAt:     startedAt,
				RecordKind:    LineageRecordKindCandle,
			}

			started := base
			started.Status = NormalizationRunStatusStarted
			completedAt := startedAt.Add(time.Minute)
			started.CompletedAt = &completedAt
			_, err := NewNormalizationRun(started)
			require.ErrorIs(t, err, ErrValidation)

			failed := base
			failed.Status = NormalizationRunStatusFailed
			_, err = NewNormalizationRun(failed)
			require.ErrorIs(t, err, ErrValidation)

			zero := time.Time{}
			failed.CompletedAt = &zero
			_, err = NewNormalizationRun(failed)
			require.ErrorIs(t, err, ErrValidation)
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
			require.NotEqual(t, time.UTC, batch.TimeRange.Start.Location())
			require.Equal(t, batch.TimeRange.Start.Location(), batch.TimeRange.End.Location())
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
