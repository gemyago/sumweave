package data

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestDatabaseStoreLineage(t *testing.T) {
	t.Parallel()

	fake := faker.New()

	makeStore := func(t *testing.T, tablePrefix string) *DatabaseStore {
		t.Helper()

		sqlDB, err := sqlconn.Open(":memory:")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

		store, err := NewDatabaseStore(sqlDB, ":memory:", DatabaseStoreOpts{TablePrefix: tablePrefix})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())

		return store
	}

	randomWord := func(prefix string) string {
		return prefix + "-" + strings.ToLower(fake.Lorem().Word())
	}

	randomTime := func() time.Time {
		return time.Date(
			fake.IntBetween(2020, 2035),
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 28),
			fake.IntBetween(0, 23),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 999999999),
			time.FixedZone(randomWord("zone"), fake.IntBetween(-11, 12)*3600),
		)
	}

	readCount := func(t *testing.T, store *DatabaseStore, tableName string) int64 {
		t.Helper()

		var count int64
		require.NoError(t, store.db.WithContext(t.Context()).Table(tableName).Count(&count).Error)

		return count
	}

	hasUniqueIndexWithColumns := func(t *testing.T, store *DatabaseStore, tableName string, want []string) bool {
		t.Helper()

		var indexes []sqliteIndexListRow
		require.NoError(t, store.db.Raw(fmt.Sprintf("PRAGMA index_list('%s')", tableName)).Scan(&indexes).Error)

		for _, indexRow := range indexes {
			if indexRow.Unique == 0 {
				continue
			}

			var columns []sqliteIndexInfoRow
			require.NoError(
				t,
				store.db.Raw(fmt.Sprintf("PRAGMA index_info('%s')", indexRow.Name)).Scan(&columns).Error,
			)

			got := make([]string, 0, len(columns))
			for _, column := range columns {
				got = append(got, column.Name)
			}

			if slices.Equal(got, want) {
				return true
			}
		}

		return false
	}

	makeIngestionRun := func(t *testing.T) IngestionRun {
		t.Helper()

		run, err := NewIngestionRun(IngestionRunParams{
			ID:          randomWord("ingestion-run"),
			Source:      randomWord("source"),
			Venue:       domain.Venue(randomWord("venue")),
			Status:      IngestionRunStatusStarted,
			StartedAt:   randomTime(),
			RecordCount: fake.IntBetween(0, 1000),
		})
		require.NoError(t, err)

		return run
	}

	makeRawVenuePayload := func(t *testing.T, ingestionRunID string) RawVenuePayload {
		t.Helper()

		requestAt := randomTime()
		responseAt := requestAt.Add(time.Duration(fake.IntBetween(1, 30)) * time.Second)
		receivedAt := responseAt.Add(time.Duration(fake.IntBetween(0, 5)) * time.Second)
		if responseAt.Before(requestAt) {
			responseAt = requestAt.Add(time.Second)
		}

		payload, err := NewRawVenuePayload(RawVenuePayloadParams{
			ID:                 randomWord("raw-payload"),
			IngestionRunID:     ingestionRunID,
			Source:             randomWord("source"),
			Venue:              domain.Venue(randomWord("venue")),
			Endpoint:           "/info",
			RequestType:        randomWord("request-type"),
			RequestPayloadHash: randomWord("request-hash"),
			RequestMetadata: map[string]string{
				randomWord("safe-key"): randomWord("safe-value"),
			},
			RequestAt:        requestAt,
			ResponseAt:       responseAt,
			HTTPStatus:       200,
			ResponseBodyHash: randomWord("body-hash"),
			PayloadBodyRef:   randomWord("payload-body-ref"),
			EntityHint:       randomWord("entity-hint"),
			ReceivedAt:       receivedAt,
		})
		require.NoError(t, err)

		return payload
	}

	makeNormalizationRun := func(t *testing.T, rawPayloadIDs ...string) NormalizationRun {
		t.Helper()

		run, err := NewNormalizationRun(NormalizationRunParams{
			ID:                   randomWord("normalization-run"),
			RawPayloadIDs:        rawPayloadIDs,
			Status:               NormalizationRunStatusStarted,
			StartedAt:            randomTime(),
			RecordKind:           LineageRecordKindTrade,
			SourceRecordCount:    fake.IntBetween(0, 1000),
			CanonicalRecordCount: fake.IntBetween(0, 1000),
		})
		require.NoError(t, err)

		return run
	}

	makeDataBatch := func(t *testing.T, normalizationRunID string) DataBatch {
		t.Helper()

		start := randomTime()
		timeRange, err := domain.NewTimeRange(
			start,
			start.Add(time.Duration(fake.IntBetween(1, 120))*time.Minute),
		)
		require.NoError(t, err)

		batch, err := NewDataBatch(DataBatchParams{
			ID:                 randomWord("data-batch"),
			NormalizationRunID: normalizationRunID,
			Venue:              domain.Venue(randomWord("venue")),
			Instrument: &BatchInstrumentRef{
				Symbol:     domain.Symbol(strings.ToUpper(randomWord("symbol"))),
				AssetClass: domain.AssetClassCrypto,
			},
			RecordKind:  LineageRecordKindTrade,
			TimeRange:   timeRange,
			Quality:     domain.DataQualityValidated,
			RecordCount: fake.IntBetween(0, 1000),
			Summary:     randomWord("summary"),
		})
		require.NoError(t, err)

		return batch
	}

	makeInstrument := func(t *testing.T, venue domain.Venue) domain.Instrument {
		t.Helper()

		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      venue,
			Symbol:     domain.Symbol(strings.ToUpper(randomWord("symbol"))),
			AssetClass: domain.AssetClassCrypto,
			Active:     true,
		})
		require.NoError(t, err)

		return instrument
	}

	makeCandle := func(t *testing.T, instrument domain.Instrument, timeRange domain.TimeRange) domain.Candle {
		t.Helper()

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
			Volume:     fake.Float64(4, 0, 10000),
			Quality:    domain.DataQualityValidated,
			Provenance: provenance,
		})
		require.NoError(t, err)

		return candle
	}

	makeTrade := func(t *testing.T, instrument domain.Instrument, eventTime time.Time) domain.Trade {
		t.Helper()

		provenance, err := domain.NewSourceProvenance(randomWord("source"), randomWord("record"))
		require.NoError(t, err)

		trade, err := domain.NewTrade(domain.TradeParams{
			Instrument: instrument,
			EventTime:  eventTime,
			Price:      fake.Float64(4, 1, 100000),
			Size:       fake.Float64(4, 0, 100000),
			Quality:    domain.DataQualityRaw,
			Provenance: provenance,
		})
		require.NoError(t, err)

		return trade
	}

	t.Run("AutoMigrate creates explicit lineage schema", func(t *testing.T) {
		t.Parallel()

		tablePrefix := strings.ReplaceAll(randomWord("sf"), "-", "_") + "_"
		store := makeStore(t, tablePrefix)

		assertColumns := func(tableName string, want []string) {
			t.Helper()

			var columns []sqliteTableInfoRow
			require.NoError(
				t,
				store.db.Raw(fmt.Sprintf("PRAGMA table_info('%s')", tableName)).Scan(&columns).Error,
			)
			require.Equal(t, want, columnNames(columns))
		}

		assertColumns(tablePrefix+"ingestion_runs", []string{
			"id",
			"source",
			"venue",
			"status",
			"started_at",
			"completed_at",
			"record_count",
			"error_summary",
			"created_at",
			"updated_at",
		})
		assertColumns(tablePrefix+"raw_venue_payloads", []string{
			"id",
			"ingestion_run_id",
			"source",
			"venue",
			"endpoint",
			"request_type",
			"request_payload_hash",
			"request_metadata_json",
			"request_at",
			"response_at",
			"http_status",
			"response_body_hash",
			"payload_body_ref",
			"entity_hint",
			"instrument_symbol",
			"instrument_asset_class",
			"timeframe",
			"start_at",
			"end_at",
			"received_at",
			"created_at",
			"updated_at",
		})
		assertColumns(tablePrefix+"normalization_runs", []string{
			"id",
			"status",
			"started_at",
			"completed_at",
			"record_kind",
			"source_record_count",
			"canonical_record_count",
			"error_summary",
			"created_at",
			"updated_at",
		})
		assertColumns(tablePrefix+"normalization_run_raw_payload_links", []string{
			"normalization_run_id",
			"raw_payload_id",
			"created_at",
		})
		assertColumns(tablePrefix+"data_batches", []string{
			"id",
			"normalization_run_id",
			"venue",
			"instrument_symbol",
			"instrument_asset_class",
			"record_kind",
			"start_at",
			"end_at",
			"quality",
			"record_count",
			"summary",
			"created_at",
			"updated_at",
		})

		require.True(t, hasUniqueIndexWithColumns(t, store, tablePrefix+"ingestion_runs", []string{"id"}))
		require.True(t, hasUniqueIndexWithColumns(t, store, tablePrefix+"raw_venue_payloads", []string{"id"}))
		require.True(t, hasUniqueIndexWithColumns(t, store, tablePrefix+"normalization_runs", []string{"id"}))
		require.True(t, hasUniqueIndexWithColumns(t, store, tablePrefix+"data_batches", []string{"id"}))
		require.True(
			t,
			hasUniqueIndexWithColumns(t, store, tablePrefix+"candles", []string{
				"instrument_id",
				"timeframe",
				"start_at",
				"end_at",
				"provenance_source",
				"provenance_identity_key",
			}),
		)
		require.True(
			t,
			hasUniqueIndexWithColumns(t, store, tablePrefix+"trades", []string{
				"instrument_id",
				"provenance_source",
				"provenance_identity_key",
			}),
		)
		require.True(
			t,
			hasUniqueIndexWithColumns(
				t,
				store,
				tablePrefix+"normalization_run_raw_payload_links",
				[]string{"normalization_run_id", "raw_payload_id"},
			),
		)
		require.True(
			t,
			hasUniqueIndexWithColumns(
				t,
				store,
				tablePrefix+"raw_payload_instrument_links",
				[]string{"raw_payload_id", "instrument_id"},
			),
		)
		require.True(
			t,
			hasUniqueIndexWithColumns(
				t,
				store,
				tablePrefix+"raw_payload_candle_links",
				[]string{"raw_payload_id", "candle_id"},
			),
		)
		require.True(
			t,
			hasUniqueIndexWithColumns(
				t,
				store,
				tablePrefix+"raw_payload_trade_links",
				[]string{"raw_payload_id", "trade_id"},
			),
		)

		assertColumns(tablePrefix+"candles", []string{
			"id",
			"instrument_id",
			"timeframe",
			"start_at",
			"end_at",
			"provenance_source",
			"provenance_identity_key",
			"open",
			"high",
			"low",
			"close",
			"volume",
			"quality",
			"provenance_record_id",
			"data_batch_id",
			"created_at",
			"updated_at",
		})
		assertColumns(tablePrefix+"trades", []string{
			"id",
			"instrument_id",
			"event_time",
			"price",
			"size",
			"quality",
			"provenance_source",
			"provenance_identity_key",
			"provenance_record_id",
			"data_batch_id",
			"created_at",
			"updated_at",
		})
		assertColumns(tablePrefix+"raw_payload_instrument_links", []string{
			"raw_payload_id",
			"instrument_id",
			"created_at",
		})
		assertColumns(tablePrefix+"raw_payload_candle_links", []string{
			"raw_payload_id",
			"candle_id",
			"created_at",
		})
		assertColumns(tablePrefix+"raw_payload_trade_links", []string{
			"raw_payload_id",
			"trade_id",
			"created_at",
		})
	})

	t.Run("UpsertIngestionRun updates one row and normalizes UTC timestamps", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, "")
		run := makeIngestionRun(t)

		persisted, err := store.UpsertIngestionRun(t.Context(), run)
		require.NoError(t, err)
		require.Equal(t, time.UTC, persisted.StartedAt.Location())

		updated, err := NewIngestionRun(IngestionRunParams{
			ID:           run.ID,
			Source:       run.Source,
			Venue:        run.Venue,
			Status:       IngestionRunStatusSucceeded,
			StartedAt:    run.StartedAt,
			CompletedAt:  run.StartedAt.Add(2 * time.Minute),
			RecordCount:  run.RecordCount + 7,
			ErrorSummary: randomWord("summary"),
		})
		require.NoError(t, err)

		persistedUpdated, err := store.UpsertIngestionRun(t.Context(), updated)
		require.NoError(t, err)
		require.Equal(t, updated, persistedUpdated)
		require.Equal(t, int64(1), readCount(t, store, "ingestion_runs"))
	})

	t.Run("UpsertRawVenuePayload supports standalone rows and preserves body refs", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, "")
		unknownParentPayload := makeRawVenuePayload(t, randomWord("missing-run"))

		_, err := store.UpsertRawVenuePayload(t.Context(), unknownParentPayload)
		require.ErrorIs(t, err, ErrLineageParentNotFound)
		require.Equal(t, int64(0), readCount(t, store, "raw_venue_payloads"))

		run := makeIngestionRun(t)
		_, err = store.UpsertIngestionRun(t.Context(), run)
		require.NoError(t, err)

		requestAt := randomTime()
		responseAt := requestAt.Add(time.Minute)
		receivedAt := responseAt.Add(time.Minute)

		payload, err := NewRawVenuePayload(RawVenuePayloadParams{
			ID:                 randomWord("raw-payload"),
			IngestionRunID:     run.ID,
			Source:             randomWord("source"),
			Venue:              domain.Venue(randomWord("venue")),
			Endpoint:           "/info",
			RequestType:        randomWord("request-type"),
			RequestPayloadHash: randomWord("request-hash"),
			RequestMetadata: map[string]string{
				"Authorization":         randomWord("auth"),
				"cookie":                randomWord("cookie"),
				"x-api-key":             randomWord("api-key"),
				"response-signature":    randomWord("signature"),
				"safe-request-id":       randomWord("request-id"),
				"safe-response-version": randomWord("version"),
			},
			RequestAt:        requestAt,
			ResponseAt:       responseAt,
			HTTPStatus:       200,
			ResponseBodyHash: randomWord("body-hash"),
			PayloadBodyRef:   randomWord("body-ref"),
			EntityHint:       randomWord("entity-hint"),
			ReceivedAt:       receivedAt,
		})
		require.NoError(t, err)

		persisted, err := store.UpsertRawVenuePayload(t.Context(), payload)
		require.NoError(t, err)
		require.Equal(t, time.UTC, persisted.ReceivedAt.Location())
		require.Contains(t, persisted.RequestMetadata, "safe-request-id")
		require.NotContains(t, persisted.RequestMetadata, "Authorization")
		require.NotContains(t, persisted.RequestMetadata, "cookie")
		require.NotContains(t, persisted.RequestMetadata, "x-api-key")
		require.NotContains(t, persisted.RequestMetadata, "response-signature")
		require.Empty(t, persisted.ResponseBody)

		updated, err := NewRawVenuePayload(RawVenuePayloadParams{
			ID:                 payload.ID,
			IngestionRunID:     payload.IngestionRunID,
			Source:             payload.Source,
			Venue:              payload.Venue,
			Endpoint:           payload.Endpoint,
			RequestType:        payload.RequestType,
			RequestPayloadHash: payload.RequestPayloadHash,
			RequestMetadata: map[string]string{
				"safe-request-id": payload.RequestMetadata["safe-request-id"],
			},
			RequestAt:        payload.RequestAt.Add(time.Minute),
			ResponseAt:       payload.ResponseAt.Add(2 * time.Minute),
			HTTPStatus:       503,
			ResponseBodyHash: randomWord("new-body-hash"),
			PayloadBodyRef:   randomWord("new-body-ref"),
			EntityHint:       payload.EntityHint,
			ReceivedAt:       payload.ReceivedAt.Add(3 * time.Minute),
		})
		require.NoError(t, err)

		persistedUpdated, err := store.UpsertRawVenuePayload(t.Context(), updated)
		require.NoError(t, err)
		require.Equal(t, payload.PayloadBodyRef, persistedUpdated.PayloadBodyRef)
		require.Equal(t, payload.ResponseBodyHash, persistedUpdated.ResponseBodyHash)
		require.Equal(t, updated.HTTPStatus, persistedUpdated.HTTPStatus)
		require.Equal(t, int64(1), readCount(t, store, "raw_venue_payloads"))

		standalonePayload := makeRawVenuePayload(t, "")
		persistedStandalone, err := store.UpsertRawVenuePayload(t.Context(), standalonePayload)
		require.NoError(t, err)
		require.Empty(t, persistedStandalone.IngestionRunID)

		var row struct {
			RequestMetadataJSON string `gorm:"column:request_metadata_json"`
			PayloadBodyRef      string `gorm:"column:payload_body_ref"`
			ResponseBodyHash    string `gorm:"column:response_body_hash"`
		}
		require.NoError(
			t,
			store.db.WithContext(t.Context()).
				Table("raw_venue_payloads").
				Where("id = ?", payload.ID).
				First(&row).Error,
		)
		require.Contains(t, row.RequestMetadataJSON, "safe-request-id")
		require.NotContains(t, row.RequestMetadataJSON, "Authorization")
		require.NotContains(t, row.RequestMetadataJSON, "cookie")
		require.NotContains(t, row.RequestMetadataJSON, "api-key")
		require.NotContains(t, row.RequestMetadataJSON, "signature")
		require.Equal(t, payload.PayloadBodyRef, row.PayloadBodyRef)
		require.Equal(t, payload.ResponseBodyHash, row.ResponseBodyHash)
	})

	t.Run("UpsertNormalizationRun rejects unknown raw payloads and handles duplicate writes", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, "")
		run := makeIngestionRun(t)
		_, err := store.UpsertIngestionRun(t.Context(), run)
		require.NoError(t, err)

		missingParent := makeNormalizationRun(t, randomWord("missing-payload"))
		_, err = store.UpsertNormalizationRun(t.Context(), missingParent)
		require.ErrorIs(t, err, ErrLineageParentNotFound)
		require.Equal(t, int64(0), readCount(t, store, "normalization_runs"))

		payloadOne := makeRawVenuePayload(t, run.ID)
		payloadTwo := makeRawVenuePayload(t, run.ID)
		_, err = store.UpsertRawVenuePayload(t.Context(), payloadOne)
		require.NoError(t, err)
		_, err = store.UpsertRawVenuePayload(t.Context(), payloadTwo)
		require.NoError(t, err)

		normalizationRun := makeNormalizationRun(t, payloadOne.ID, payloadTwo.ID)
		persisted, err := store.UpsertNormalizationRun(t.Context(), normalizationRun)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{payloadOne.ID, payloadTwo.ID}, persisted.RawPayloadIDs)
		require.Equal(t, time.UTC, persisted.StartedAt.Location())

		updated, err := NewNormalizationRun(NormalizationRunParams{
			ID:                   normalizationRun.ID,
			RawPayloadIDs:        []string{payloadTwo.ID, payloadOne.ID, payloadOne.ID},
			Status:               NormalizationRunStatusFailed,
			StartedAt:            normalizationRun.StartedAt,
			CompletedAt:          normalizationRun.StartedAt.Add(3 * time.Minute),
			RecordKind:           normalizationRun.RecordKind,
			SourceRecordCount:    normalizationRun.SourceRecordCount + 1,
			CanonicalRecordCount: normalizationRun.CanonicalRecordCount + 2,
			ErrorSummary:         randomWord("error"),
		})
		require.NoError(t, err)

		persistedUpdated, err := store.UpsertNormalizationRun(t.Context(), updated)
		require.NoError(t, err)
		require.Equal(t, updated.Status, persistedUpdated.Status)
		require.ElementsMatch(t, []string{payloadOne.ID, payloadTwo.ID}, persistedUpdated.RawPayloadIDs)
		require.Equal(t, int64(1), readCount(t, store, "normalization_runs"))
		require.Equal(t, int64(2), readCount(t, store, "normalization_run_raw_payload_links"))
	})

	t.Run(
		"UpsertDataBatch rejects unknown parents and returns batch audit in stable raw payload order",
		func(t *testing.T) {
			t.Parallel()

			store := makeStore(t, "")
			missingParent := makeDataBatch(t, randomWord("missing-normalization"))
			_, err := store.UpsertDataBatch(t.Context(), missingParent)
			require.ErrorIs(t, err, ErrLineageParentNotFound)
			require.Equal(t, int64(0), readCount(t, store, "data_batches"))

			runOne := makeIngestionRun(t)
			runTwo := makeIngestionRun(t)
			_, err = store.UpsertIngestionRun(t.Context(), runOne)
			require.NoError(t, err)
			_, err = store.UpsertIngestionRun(t.Context(), runTwo)
			require.NoError(t, err)

			receivedAt := randomTime()
			laterReceivedAt := receivedAt.Add(2 * time.Minute)
			if laterReceivedAt.Before(receivedAt) {
				laterReceivedAt = receivedAt.Add(2 * time.Minute)
			}

			payloadLater := makeRawVenuePayload(t, runTwo.ID)
			payloadLater.ID = "z-" + payloadLater.ID
			payloadLater.ReceivedAt = laterReceivedAt.UTC()

			payloadSameTimeSecondID := makeRawVenuePayload(t, runOne.ID)
			payloadSameTimeSecondID.ID = "b-" + payloadSameTimeSecondID.ID
			payloadSameTimeSecondID.ReceivedAt = receivedAt.UTC()

			payloadSameTimeFirstID := makeRawVenuePayload(t, runOne.ID)
			payloadSameTimeFirstID.ID = "a-" + payloadSameTimeFirstID.ID
			payloadSameTimeFirstID.ReceivedAt = receivedAt.UTC()

			for _, payload := range []RawVenuePayload{payloadLater, payloadSameTimeSecondID, payloadSameTimeFirstID} {
				_, err = store.UpsertRawVenuePayload(t.Context(), payload)
				require.NoError(t, err)
			}

			normalizationRun := makeNormalizationRun(
				t,
				payloadLater.ID,
				payloadSameTimeSecondID.ID,
				payloadSameTimeFirstID.ID,
			)
			_, err = store.UpsertNormalizationRun(t.Context(), normalizationRun)
			require.NoError(t, err)

			batch := makeDataBatch(t, normalizationRun.ID)
			persistedBatch, err := store.UpsertDataBatch(t.Context(), batch)
			require.NoError(t, err)

			updatedBatch, err := NewDataBatch(DataBatchParams{
				ID:                 batch.ID,
				NormalizationRunID: batch.NormalizationRunID,
				Venue:              batch.Venue,
				Instrument:         batch.Instrument,
				RecordKind:         batch.RecordKind,
				TimeRange:          batch.TimeRange,
				Quality:            domain.DataQualitySuspect,
				RecordCount:        batch.RecordCount + 5,
				Summary:            randomWord("summary"),
			})
			require.NoError(t, err)

			persistedUpdatedBatch, err := store.UpsertDataBatch(t.Context(), updatedBatch)
			require.NoError(t, err)
			require.Equal(t, updatedBatch, persistedUpdatedBatch)
			require.NotEqual(t, persistedBatch.Quality, persistedUpdatedBatch.Quality)
			require.Equal(t, int64(1), readCount(t, store, "data_batches"))

			audit, err := store.GetDataBatchAudit(t.Context(), batch.ID)
			require.NoError(t, err)
			require.Equal(t, updatedBatch, audit.Batch)
			require.Equal(t, normalizationRun.ID, audit.NormalizationRun.ID)
			require.Equal(
				t,
				[]string{payloadSameTimeFirstID.ID, payloadSameTimeSecondID.ID, payloadLater.ID},
				[]string{
					audit.RawPayloads[0].Payload.ID,
					audit.RawPayloads[1].Payload.ID,
					audit.RawPayloads[2].Payload.ID,
				},
			)
			require.Equal(t, runOne.ID, audit.RawPayloads[0].IngestionRun.ID)
			require.Equal(t, runOne.ID, audit.RawPayloads[1].IngestionRun.ID)
			require.Equal(t, runTwo.ID, audit.RawPayloads[2].IngestionRun.ID)
			require.Equal(t, time.UTC, audit.RawPayloads[0].Payload.ReceivedAt.Location())
			require.Equal(t, time.UTC, audit.RawPayloads[2].IngestionRun.StartedAt.Location())
		},
	)

	t.Run("batch-linked candle and trade writes keep optional batch linkage", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, "")
		run := makeIngestionRun(t)
		_, err := store.UpsertIngestionRun(t.Context(), run)
		require.NoError(t, err)

		payload := makeRawVenuePayload(t, run.ID)
		_, err = store.UpsertRawVenuePayload(t.Context(), payload)
		require.NoError(t, err)

		normalizationStartedAt := randomTime()
		normalizationRun, err := NewNormalizationRun(NormalizationRunParams{
			ID:                   randomWord("normalization-run"),
			RawPayloadIDs:        []string{payload.ID},
			Status:               NormalizationRunStatusSucceeded,
			StartedAt:            normalizationStartedAt,
			CompletedAt:          normalizationStartedAt.Add(time.Minute),
			RecordKind:           LineageRecordKindCandle,
			SourceRecordCount:    fake.IntBetween(1, 1000),
			CanonicalRecordCount: fake.IntBetween(1, 1000),
		})
		require.NoError(t, err)
		_, err = store.UpsertNormalizationRun(t.Context(), normalizationRun)
		require.NoError(t, err)

		batch := makeDataBatch(t, normalizationRun.ID)
		batch.RecordKind = LineageRecordKindCandle
		instrument := makeInstrument(t, batch.Venue)
		batch.Instrument = &BatchInstrumentRef{Symbol: instrument.Symbol, AssetClass: instrument.AssetClass}
		batch, err = NewDataBatch(DataBatchParams(batch))
		require.NoError(t, err)
		_, err = store.UpsertDataBatch(t.Context(), batch)
		require.NoError(t, err)

		_, err = store.UpsertInstrument(t.Context(), instrument)
		require.NoError(t, err)

		candle := makeCandle(t, instrument, batch.TimeRange)
		persistedCandle, err := store.UpsertCandleForDataBatch(t.Context(), batch.ID, candle)
		require.NoError(t, err)
		require.Equal(t, candle, persistedCandle)

		var candleRow struct {
			DataBatchID string `gorm:"column:data_batch_id"`
		}
		require.NoError(
			t,
			store.db.WithContext(t.Context()).
				Table("candles").
				Where("provenance_identity_key = ?", candle.Provenance.RecordID).
				First(&candleRow).Error,
		)
		require.Equal(t, batch.ID, candleRow.DataBatchID)

		persistedWithoutBatch, err := store.UpsertCandle(t.Context(), candle)
		require.NoError(t, err)
		require.Equal(t, candle, persistedWithoutBatch)
		require.NoError(
			t,
			store.db.WithContext(t.Context()).
				Table("candles").
				Where("provenance_identity_key = ?", candle.Provenance.RecordID).
				First(&candleRow).Error,
		)
		require.Equal(t, batch.ID, candleRow.DataBatchID)

		tradeNormalizationStartedAt := randomTime()
		tradeNormalizationRun, err := NewNormalizationRun(NormalizationRunParams{
			ID:                   randomWord("normalization-run"),
			RawPayloadIDs:        []string{payload.ID},
			Status:               NormalizationRunStatusSucceeded,
			StartedAt:            tradeNormalizationStartedAt,
			CompletedAt:          tradeNormalizationStartedAt.Add(time.Minute),
			RecordKind:           LineageRecordKindTrade,
			SourceRecordCount:    fake.IntBetween(1, 1000),
			CanonicalRecordCount: fake.IntBetween(1, 1000),
		})
		require.NoError(t, err)
		_, err = store.UpsertNormalizationRun(t.Context(), tradeNormalizationRun)
		require.NoError(t, err)

		tradeBatch := makeDataBatch(t, tradeNormalizationRun.ID)
		tradeBatch.RecordKind = LineageRecordKindTrade
		tradeBatch.Instrument = &BatchInstrumentRef{Symbol: instrument.Symbol, AssetClass: instrument.AssetClass}
		tradeBatch, err = NewDataBatch(DataBatchParams(tradeBatch))
		require.NoError(t, err)
		_, err = store.UpsertDataBatch(t.Context(), tradeBatch)
		require.NoError(t, err)

		trade := makeTrade(t, instrument, batch.TimeRange.Start.Add(time.Second))
		persistedTrade, err := store.UpsertTradeForDataBatch(t.Context(), tradeBatch.ID, trade)
		require.NoError(t, err)
		require.Equal(t, trade, persistedTrade)

		var tradeRow struct {
			DataBatchID string `gorm:"column:data_batch_id"`
		}
		require.NoError(
			t,
			store.db.WithContext(t.Context()).
				Table("trades").
				Where("provenance_identity_key = ?", trade.Provenance.RecordID).
				First(&tradeRow).Error,
		)
		require.Equal(t, tradeBatch.ID, tradeRow.DataBatchID)

		persistedTradeWithoutBatch, err := store.UpsertTrade(t.Context(), trade)
		require.NoError(t, err)
		require.Equal(t, trade, persistedTradeWithoutBatch)
		require.NoError(
			t,
			store.db.WithContext(t.Context()).
				Table("trades").
				Where("provenance_identity_key = ?", trade.Provenance.RecordID).
				First(&tradeRow).Error,
		)
		require.Equal(t, tradeBatch.ID, tradeRow.DataBatchID)
	})

	t.Run("rejects mismatched batch kinds before linking rows", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, "")
		run := makeIngestionRun(t)
		_, err := store.UpsertIngestionRun(t.Context(), run)
		require.NoError(t, err)

		payload := makeRawVenuePayload(t, run.ID)
		_, err = store.UpsertRawVenuePayload(t.Context(), payload)
		require.NoError(t, err)

		instrument := makeInstrument(t, run.Venue)
		_, err = store.UpsertInstrument(t.Context(), instrument)
		require.NoError(t, err)

		candleNormalizationStartedAt := randomTime()
		candleNormalizationRun, err := NewNormalizationRun(NormalizationRunParams{
			ID:                   randomWord("normalization-run"),
			RawPayloadIDs:        []string{payload.ID},
			Status:               NormalizationRunStatusSucceeded,
			StartedAt:            candleNormalizationStartedAt,
			CompletedAt:          candleNormalizationStartedAt.Add(time.Minute),
			RecordKind:           LineageRecordKindCandle,
			SourceRecordCount:    1,
			CanonicalRecordCount: 1,
		})
		require.NoError(t, err)
		_, err = store.UpsertNormalizationRun(t.Context(), candleNormalizationRun)
		require.NoError(t, err)

		tradeNormalizationStartedAt := randomTime()
		tradeNormalizationRun, err := NewNormalizationRun(NormalizationRunParams{
			ID:                   randomWord("normalization-run"),
			RawPayloadIDs:        []string{payload.ID},
			Status:               NormalizationRunStatusSucceeded,
			StartedAt:            tradeNormalizationStartedAt,
			CompletedAt:          tradeNormalizationStartedAt.Add(time.Minute),
			RecordKind:           LineageRecordKindTrade,
			SourceRecordCount:    1,
			CanonicalRecordCount: 1,
		})
		require.NoError(t, err)
		_, err = store.UpsertNormalizationRun(t.Context(), tradeNormalizationRun)
		require.NoError(t, err)

		candleBatchStart := randomTime().UTC()
		candleBatchTimeRange, err := domain.NewTimeRange(candleBatchStart, candleBatchStart.Add(time.Minute))
		require.NoError(t, err)

		candleBatch, err := NewDataBatch(DataBatchParams{
			ID:                 randomWord("data-batch"),
			NormalizationRunID: candleNormalizationRun.ID,
			Venue:              instrument.Venue,
			Instrument:         &BatchInstrumentRef{Symbol: instrument.Symbol, AssetClass: instrument.AssetClass},
			RecordKind:         LineageRecordKindCandle,
			TimeRange:          candleBatchTimeRange,
			Quality:            domain.DataQualityValidated,
			RecordCount:        1,
		})
		require.NoError(t, err)
		_, err = store.UpsertDataBatch(t.Context(), candleBatch)
		require.NoError(t, err)

		tradeBatchStart := randomTime().UTC()
		tradeBatchTimeRange, err := domain.NewTimeRange(tradeBatchStart, tradeBatchStart.Add(time.Minute))
		require.NoError(t, err)

		tradeBatch, err := NewDataBatch(DataBatchParams{
			ID:                 randomWord("data-batch"),
			NormalizationRunID: tradeNormalizationRun.ID,
			Venue:              instrument.Venue,
			Instrument:         &BatchInstrumentRef{Symbol: instrument.Symbol, AssetClass: instrument.AssetClass},
			RecordKind:         LineageRecordKindTrade,
			TimeRange:          tradeBatchTimeRange,
			Quality:            domain.DataQualityValidated,
			RecordCount:        1,
		})
		require.NoError(t, err)
		_, err = store.UpsertDataBatch(t.Context(), tradeBatch)
		require.NoError(t, err)

		wrongCandle := makeCandle(t, instrument, candleBatch.TimeRange)
		_, err = store.UpsertCandleForDataBatch(t.Context(), tradeBatch.ID, wrongCandle)
		require.ErrorIs(t, err, ErrValidation)
		require.ErrorContains(t, err, "data batch record kind")
		var candleCount int64
		candleCountQuery := store.db.WithContext(t.Context()).
			Table("candles").
			Where("provenance_identity_key = ?", wrongCandle.Provenance.RecordID)
		require.NoError(t, candleCountQuery.Count(&candleCount).Error)
		require.Zero(t, candleCount)

		wrongTrade := makeTrade(t, instrument, tradeBatch.TimeRange.Start.Add(time.Second))
		_, err = store.UpsertTradeForDataBatch(t.Context(), candleBatch.ID, wrongTrade)
		require.ErrorIs(t, err, ErrValidation)
		require.ErrorContains(t, err, "data batch record kind")
		var tradeCount int64
		tradeCountQuery := store.db.WithContext(t.Context()).
			Table("trades").
			Where("provenance_identity_key = ?", wrongTrade.Provenance.RecordID)
		require.NoError(t, tradeCountQuery.Count(&tradeCount).Error)
		require.Zero(t, tradeCount)
	})

	t.Run("raw payload normalized record links persist and read back stably", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, "")
		run := makeIngestionRun(t)
		_, err := store.UpsertIngestionRun(t.Context(), run)
		require.NoError(t, err)

		payloadOne := makeRawVenuePayload(t, run.ID)
		payloadTwo := makeRawVenuePayload(t, run.ID)
		_, err = store.UpsertRawVenuePayload(t.Context(), payloadOne)
		require.NoError(t, err)
		_, err = store.UpsertRawVenuePayload(t.Context(), payloadTwo)
		require.NoError(t, err)

		instrument := makeInstrument(t, run.Venue)
		persistedInstrument, err := store.UpsertInstrument(t.Context(), instrument)
		require.NoError(t, err)

		start := randomTime().UTC()
		candleRange, err := domain.NewTimeRange(start, start.Add(time.Minute))
		require.NoError(t, err)
		candle := makeCandle(t, persistedInstrument, candleRange)
		_, err = store.UpsertCandle(t.Context(), candle)
		require.NoError(t, err)

		trade := makeTrade(t, persistedInstrument, start.Add(time.Second))
		_, err = store.UpsertTrade(t.Context(), trade)
		require.NoError(t, err)

		err = store.LinkRawPayloadToInstrument(t.Context(), payloadOne.ID, persistedInstrument)
		require.NoError(t, err)
		err = store.LinkRawPayloadToInstrument(t.Context(), payloadTwo.ID, persistedInstrument)
		require.NoError(t, err)
		err = store.LinkRawPayloadToInstrument(t.Context(), payloadTwo.ID, persistedInstrument)
		require.NoError(t, err)

		err = store.LinkRawPayloadToCandle(t.Context(), payloadOne.ID, candle)
		require.NoError(t, err)
		err = store.LinkRawPayloadToTrade(t.Context(), payloadTwo.ID, trade)
		require.NoError(t, err)

		instrumentIDs, err := store.ListInstrumentRawPayloadIDs(t.Context(), persistedInstrument)
		require.NoError(t, err)
		require.Equal(t, []string{min(payloadOne.ID, payloadTwo.ID), max(payloadOne.ID, payloadTwo.ID)}, instrumentIDs)

		candleIDs, err := store.ListCandleRawPayloadIDs(t.Context(), candle)
		require.NoError(t, err)
		require.Equal(t, []string{payloadOne.ID}, candleIDs)

		tradeIDs, err := store.ListTradeRawPayloadIDs(t.Context(), trade)
		require.NoError(t, err)
		require.Equal(t, []string{payloadTwo.ID}, tradeIDs)
	})

	t.Run("raw payload normalized record links reject unknown raw payload ids", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, "")
		instrument := makeInstrument(t, domain.Venue(randomWord("venue")))
		persistedInstrument, err := store.UpsertInstrument(t.Context(), instrument)
		require.NoError(t, err)

		err = store.LinkRawPayloadToInstrument(t.Context(), randomWord("missing-payload"), persistedInstrument)
		require.ErrorIs(t, err, ErrLineageParentNotFound)
	})

	t.Run("ListRawPayloadMetadata filters ordered rows with deterministic pagination", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, "")
		run := makeIngestionRun(t)
		_, err := store.UpsertIngestionRun(t.Context(), run)
		require.NoError(t, err)

		makeScopedPayload := func(id string, receivedAt time.Time) RawVenuePayload {
			t.Helper()

			payload, payloadErr := NewRawVenuePayload(RawVenuePayloadParams{
				ID:                 id,
				IngestionRunID:     run.ID,
				Source:             randomWord("source"),
				Venue:              run.Venue,
				Endpoint:           "/info",
				RequestType:        "candleSnapshot",
				RequestPayloadHash: randomWord("request-hash"),
				RequestAt:          receivedAt.Add(-2 * time.Second),
				ResponseAt:         receivedAt.Add(-time.Second),
				HTTPStatus:         200,
				ResponseBodyHash:   randomWord("body-hash"),
				PayloadBodyRef:     randomWord("ref"),
				EntityHint:         "candle",
				Instrument: &BatchInstrumentRef{
					Symbol:     domain.Symbol("BTC-USD"),
					AssetClass: domain.AssetClassCrypto,
				},
				Timeframe:  domain.Timeframe1m,
				TimeRange:  &domain.TimeRange{Start: receivedAt.Add(-time.Minute), End: receivedAt},
				ReceivedAt: receivedAt,
			})
			require.NoError(t, payloadErr)

			return payload
		}

		baseReceivedAt := randomTime().UTC()
		matchingFirst := makeScopedPayload("payload-a", baseReceivedAt)
		matchingSecond := makeScopedPayload("payload-b", baseReceivedAt)
		wrongEndpoint := makeScopedPayload("payload-c", baseReceivedAt.Add(time.Minute))
		wrongEndpoint.Endpoint = "/other"
		wrongEndpoint, err = NewRawVenuePayload(RawVenuePayloadParams(wrongEndpoint))
		require.NoError(t, err)

		for _, payload := range []RawVenuePayload{matchingSecond, wrongEndpoint, matchingFirst} {
			_, err = store.UpsertRawVenuePayload(t.Context(), payload)
			require.NoError(t, err)
		}

		query, err := NewRawPayloadMetadataListQuery(RawPayloadMetadataListQueryParams{
			Venue:          run.Venue,
			Symbol:         domain.Symbol("BTC-USD"),
			AssetClass:     domain.AssetClassCrypto,
			Timeframe:      domain.Timeframe1m,
			StartAt:        matchingFirst.TimeRange.Start,
			EndAt:          matchingFirst.TimeRange.End.Add(time.Second),
			IngestionRunID: run.ID,
			EntityHint:     "candle",
			Endpoint:       "/info",
			RequestType:    "candleSnapshot",
			Limit:          1,
		})
		require.NoError(t, err)

		firstPage, err := store.ListRawPayloadMetadata(t.Context(), query)
		require.NoError(t, err)
		require.Len(t, firstPage.Items, 1)
		require.Equal(t, matchingFirst.ID, firstPage.Items[0].ID)
		require.NotEmpty(t, firstPage.NextCursor)

		secondPage, err := store.ListRawPayloadMetadata(t.Context(), RawPayloadMetadataListQuery{
			Venue:          run.Venue,
			Instrument:     query.Instrument,
			Timeframe:      query.Timeframe,
			TimeRange:      query.TimeRange,
			IngestionRunID: query.IngestionRunID,
			EntityHint:     query.EntityHint,
			Endpoint:       query.Endpoint,
			RequestType:    query.RequestType,
			Limit:          1,
			Cursor:         firstPage.NextCursor,
		})
		require.NoError(t, err)
		require.Len(t, secondPage.Items, 1)
		require.Equal(t, matchingSecond.ID, secondPage.Items[0].ID)
		require.Empty(t, secondPage.NextCursor)
	})

	t.Run("GetRawPayloadMetadata returns metadata only and reports not found", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, "")
		run := makeIngestionRun(t)
		_, err := store.UpsertIngestionRun(t.Context(), run)
		require.NoError(t, err)

		payload := makeRawVenuePayload(t, run.ID)
		payload.ResponseBody = []byte(randomWord("body"))
		payload.PayloadBodyRef = randomWord("ref")
		payload, err = NewRawVenuePayload(RawVenuePayloadParams(payload))
		require.NoError(t, err)
		_, err = store.UpsertRawVenuePayload(t.Context(), payload)
		require.NoError(t, err)

		metadata, err := store.GetRawPayloadMetadata(t.Context(), payload.ID)
		require.NoError(t, err)
		require.Equal(t, payload.ID, metadata.ID)
		require.Equal(t, payload.PayloadBodyRef, metadata.PayloadBodyRef)
		require.Equal(t, payload.ResponseBodyHash, metadata.ResponseBodyHash)

		_, err = store.GetRawPayloadMetadata(t.Context(), randomWord("missing-payload"))
		require.ErrorIs(t, err, ErrRawPayloadNotFound)
	})

	t.Run("ListCandleLinkedRawPayloadMetadata uses exact provenance-bearing candle key", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, "")
		run := makeIngestionRun(t)
		_, err := store.UpsertIngestionRun(t.Context(), run)
		require.NoError(t, err)

		payloadOne := makeRawVenuePayload(t, run.ID)
		payloadOne.ID = "payload-a"
		payloadOne.ReceivedAt = randomTime().UTC()
		payloadOne, err = NewRawVenuePayload(RawVenuePayloadParams(payloadOne))
		require.NoError(t, err)
		payloadTwo := makeRawVenuePayload(t, run.ID)
		payloadTwo.ID = "payload-b"
		payloadTwo.ReceivedAt = payloadOne.ReceivedAt
		payloadTwo, err = NewRawVenuePayload(RawVenuePayloadParams(payloadTwo))
		require.NoError(t, err)
		payloadOther := makeRawVenuePayload(t, run.ID)
		payloadOther.ID = "payload-c"
		payloadOther.ReceivedAt = payloadOne.ReceivedAt.Add(time.Minute)
		payloadOther, err = NewRawVenuePayload(RawVenuePayloadParams(payloadOther))
		require.NoError(t, err)

		for _, payload := range []RawVenuePayload{payloadTwo, payloadOther, payloadOne} {
			_, err = store.UpsertRawVenuePayload(t.Context(), payload)
			require.NoError(t, err)
		}

		instrument := makeInstrument(t, run.Venue)
		persistedInstrument, err := store.UpsertInstrument(t.Context(), instrument)
		require.NoError(t, err)

		start := randomTime().UTC()
		candleRange, err := domain.NewTimeRange(start, start.Add(time.Minute))
		require.NoError(t, err)
		selectedCandle := makeCandle(t, persistedInstrument, candleRange)
		_, err = store.UpsertCandle(t.Context(), selectedCandle)
		require.NoError(t, err)

		otherProvenance, err := domain.NewSourceProvenance(
			selectedCandle.Provenance.Source,
			randomWord("other-record"),
		)
		require.NoError(t, err)
		otherCandle, err := domain.NewCandle(domain.CandleParams{
			Instrument: persistedInstrument,
			Timeframe:  selectedCandle.Timeframe,
			TimeRange:  selectedCandle.TimeRange,
			Open:       selectedCandle.Open,
			High:       selectedCandle.High,
			Low:        selectedCandle.Low,
			Close:      selectedCandle.Close,
			Volume:     selectedCandle.Volume,
			Quality:    selectedCandle.Quality,
			Provenance: otherProvenance,
		})
		require.NoError(t, err)
		_, err = store.UpsertCandle(t.Context(), otherCandle)
		require.NoError(t, err)

		err = store.LinkRawPayloadToCandle(t.Context(), payloadTwo.ID, selectedCandle)
		require.NoError(t, err)
		err = store.LinkRawPayloadToCandle(t.Context(), payloadOne.ID, selectedCandle)
		require.NoError(t, err)
		err = store.LinkRawPayloadToCandle(t.Context(), payloadOther.ID, otherCandle)
		require.NoError(t, err)

		items, err := store.ListCandleLinkedRawPayloadMetadata(t.Context(), CandleLinkedRawPayloadsQuery{
			Venue:              persistedInstrument.Venue,
			Symbol:             persistedInstrument.Symbol,
			AssetClass:         persistedInstrument.AssetClass,
			Timeframe:          selectedCandle.Timeframe,
			TimeRange:          selectedCandle.TimeRange,
			ProvenanceSource:   selectedCandle.Provenance.Source,
			ProvenanceIdentity: candleIdentityKey(selectedCandle.Provenance),
		})
		require.NoError(t, err)
		require.Equal(t, []string{payloadOne.ID, payloadTwo.ID}, []string{items[0].ID, items[1].ID})

		emptyItems, err := store.ListCandleLinkedRawPayloadMetadata(t.Context(), CandleLinkedRawPayloadsQuery{
			Venue:              persistedInstrument.Venue,
			Symbol:             persistedInstrument.Symbol,
			AssetClass:         persistedInstrument.AssetClass,
			Timeframe:          selectedCandle.Timeframe,
			TimeRange:          selectedCandle.TimeRange,
			ProvenanceSource:   selectedCandle.Provenance.Source,
			ProvenanceIdentity: randomWord("missing-identity"),
		})
		require.NoError(t, err)
		require.Empty(t, emptyItems)
	})

	t.Run("replay by data batch returns stable canonical identities", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, "")
		run := makeIngestionRun(t)
		_, err := store.UpsertIngestionRun(t.Context(), run)
		require.NoError(t, err)

		payload := makeRawVenuePayload(t, run.ID)
		_, err = store.UpsertRawVenuePayload(t.Context(), payload)
		require.NoError(t, err)

		instrument := makeInstrument(t, run.Venue)
		_, err = store.UpsertInstrument(t.Context(), instrument)
		require.NoError(t, err)

		candleNormalizationStartedAt := randomTime()
		candleNormalizationRun, err := NewNormalizationRun(NormalizationRunParams{
			ID:                   randomWord("normalization-run"),
			RawPayloadIDs:        []string{payload.ID},
			Status:               NormalizationRunStatusSucceeded,
			StartedAt:            candleNormalizationStartedAt,
			CompletedAt:          candleNormalizationStartedAt.Add(time.Minute),
			RecordKind:           LineageRecordKindCandle,
			SourceRecordCount:    2,
			CanonicalRecordCount: 2,
		})
		require.NoError(t, err)
		_, err = store.UpsertNormalizationRun(t.Context(), candleNormalizationRun)
		require.NoError(t, err)

		start := randomTime().UTC()
		candleRange, err := domain.NewTimeRange(start, start.Add(3*time.Minute))
		require.NoError(t, err)

		candleBatch, err := NewDataBatch(DataBatchParams{
			ID:                 randomWord("data-batch"),
			NormalizationRunID: candleNormalizationRun.ID,
			Venue:              instrument.Venue,
			Instrument:         &BatchInstrumentRef{Symbol: instrument.Symbol, AssetClass: instrument.AssetClass},
			RecordKind:         LineageRecordKindCandle,
			TimeRange:          candleRange,
			Quality:            domain.DataQualityValidated,
			RecordCount:        2,
		})
		require.NoError(t, err)
		_, err = store.UpsertDataBatch(t.Context(), candleBatch)
		require.NoError(t, err)

		firstCandleRange, err := domain.NewTimeRange(start, start.Add(time.Minute))
		require.NoError(t, err)
		secondCandleRange, err := domain.NewTimeRange(start.Add(time.Minute), start.Add(2*time.Minute))
		require.NoError(t, err)
		firstCandle := makeCandle(t, instrument, firstCandleRange)
		secondCandle := makeCandle(t, instrument, secondCandleRange)
		_, err = store.UpsertCandleForDataBatch(t.Context(), candleBatch.ID, secondCandle)
		require.NoError(t, err)
		_, err = store.UpsertCandleForDataBatch(t.Context(), candleBatch.ID, firstCandle)
		require.NoError(t, err)

		firstReplay, err := store.ReplayCandlesByDataBatch(t.Context(), candleBatch.ID)
		require.NoError(t, err)
		secondReplay, err := store.ReplayCandlesByDataBatch(t.Context(), candleBatch.ID)
		require.NoError(t, err)
		require.Equal(t, firstReplay, secondReplay)
		require.Equal(
			t,
			[]domain.Candle{firstCandle, secondCandle},
			[]domain.Candle{firstReplay[0].Candle, firstReplay[1].Candle},
		)
		require.NotZero(t, firstReplay[0].Identity)
		require.NotZero(t, firstReplay[1].Identity)

		tradeNormalizationStartedAt := randomTime()
		tradeNormalizationRun, err := NewNormalizationRun(NormalizationRunParams{
			ID:                   randomWord("normalization-run"),
			RawPayloadIDs:        []string{payload.ID},
			Status:               NormalizationRunStatusSucceeded,
			StartedAt:            tradeNormalizationStartedAt,
			CompletedAt:          tradeNormalizationStartedAt.Add(time.Minute),
			RecordKind:           LineageRecordKindTrade,
			SourceRecordCount:    2,
			CanonicalRecordCount: 2,
		})
		require.NoError(t, err)
		_, err = store.UpsertNormalizationRun(t.Context(), tradeNormalizationRun)
		require.NoError(t, err)

		tradeRange, err := domain.NewTimeRange(start, start.Add(3*time.Second))
		require.NoError(t, err)
		tradeBatch, err := NewDataBatch(DataBatchParams{
			ID:                 randomWord("data-batch"),
			NormalizationRunID: tradeNormalizationRun.ID,
			Venue:              instrument.Venue,
			Instrument:         &BatchInstrumentRef{Symbol: instrument.Symbol, AssetClass: instrument.AssetClass},
			RecordKind:         LineageRecordKindTrade,
			TimeRange:          tradeRange,
			Quality:            domain.DataQualityValidated,
			RecordCount:        2,
		})
		require.NoError(t, err)
		_, err = store.UpsertDataBatch(t.Context(), tradeBatch)
		require.NoError(t, err)

		firstTrade := makeTrade(t, instrument, start.Add(time.Second))
		secondTrade := makeTrade(t, instrument, start.Add(2*time.Second))
		_, err = store.UpsertTradeForDataBatch(t.Context(), tradeBatch.ID, secondTrade)
		require.NoError(t, err)
		_, err = store.UpsertTradeForDataBatch(t.Context(), tradeBatch.ID, firstTrade)
		require.NoError(t, err)

		firstTradeReplay, err := store.ReplayTradesByDataBatch(t.Context(), tradeBatch.ID)
		require.NoError(t, err)
		secondTradeReplay, err := store.ReplayTradesByDataBatch(t.Context(), tradeBatch.ID)
		require.NoError(t, err)
		require.Equal(t, firstTradeReplay, secondTradeReplay)
		require.Equal(
			t,
			[]domain.Trade{firstTrade, secondTrade},
			[]domain.Trade{firstTradeReplay[0].Trade, firstTradeReplay[1].Trade},
		)
		require.NotZero(t, firstTradeReplay[0].Identity)
		require.NotZero(t, firstTradeReplay[1].Identity)
	})
}
