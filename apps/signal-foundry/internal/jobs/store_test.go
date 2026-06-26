package jobs

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	fake := faker.New()
	makeStore := func(t *testing.T, dbName string) *Store {
		t.Helper()
		store, err := NewStore(
			filepath.Join(t.TempDir(), dbName+".sqlite"),
			StoreOpts{TablePrefix: "test_"},
		)
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		return store
	}
	makeJob := func(now time.Time) Job {
		input := HistoricalRawCandleBackfillInput{
			IngestionRunID: fake.UUID().V4(),
			Venue:          string("hyperliquid-perps"),
			Symbol:         "BTC",
			AssetClass:     "future",
			Timeframe:      "1h",
			Start:          now.Add(-2 * time.Hour),
			End:            now.Add(-time.Hour),
			PageSize:       100,
		}
		hash, err := HashInput(input)
		require.NoError(t, err)
		return Job{
			ID:      fake.UUID().V4(),
			JobType: JobTypeHistoricalRawCandleBackfill,
			Status:  JobStatusQueued,
			Requester: Requester{
				UserID: "user-" + fake.UUID().V4(),
				Source: RequesterSourceOperator,
			},
			IdempotencyKey: "key-" + fake.UUID().V4(),
			InputHash:      hash,
			Input:          input,
			CreatedAt:      now.UTC(),
			UpdatedAt:      now.UTC(),
			QueuedAt:       now.UTC(),
			CorrelationID:  "corr-" + fake.UUID().V4(),
		}
	}

	t.Run("persists explicit columns and restart-visible rows", func(t *testing.T) {
		now := time.Now().UTC().Add(-time.Minute)
		dsn := filepath.Join(t.TempDir(), "store-columns.sqlite")
		store, err := NewStore(dsn, StoreOpts{TablePrefix: "test_"})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		job := makeJob(now)
		created, err := store.Create(t.Context(), job)
		require.NoError(t, err)
		require.Equal(t, job.ID, created.ID)
		columnTypes, err := store.db.Migrator().ColumnTypes(store.tableName)
		require.NoError(t, err)
		columnNames := make([]string, 0, len(columnTypes))
		for _, column := range columnTypes {
			columnNames = append(columnNames, column.Name())
		}
		expectedColumns := []string{
			"id",
			"job_type",
			"status",
			"requester_user_id",
			"requester_source",
			"agent_session_id",
			"agent_run_id",
			"idempotency_key",
			"canonical_input_hash",
			"input_json",
			"result_json",
			"progress_json",
			"error_code",
			"error_summary",
			"error_details",
			"created_at",
			"updated_at",
			"queued_at",
			"started_at",
			"completed_at",
			"worker_id",
			"attempt_count",
			"max_attempts",
			"last_attempt_time",
			"correlation_id",
			"schedule_id",
		}
		assert.ElementsMatch(t, expectedColumns, columnNames)
		reopened, err := NewStore(dsn, StoreOpts{TablePrefix: "test_"})
		require.NoError(t, err)
		sqlDB, err := reopened.db.DB()
		require.NoError(t, err)
		var journalMode string
		require.NoError(t, sqlDB.QueryRowContext(t.Context(), "PRAGMA journal_mode").Scan(&journalMode))
		assert.Equal(t, "wal", journalMode)
		persisted, err := reopened.Get(t.Context(), job.ID)
		require.NoError(t, err)
		require.NotNil(t, persisted)
		assert.Equal(t, created.ID, persisted.ID)
	})

	t.Run("supports transitions, worker metadata, and bounded error fields", func(t *testing.T) {
		now := time.Now().UTC().Add(-5 * time.Minute)
		store := makeStore(t, "store-transitions")
		job := makeJob(now)
		_, err := store.Create(t.Context(), job)
		require.NoError(t, err)
		claimed, err := store.ClaimQueued(t.Context(), job.ID, "worker-a", now.Add(time.Minute))
		require.NoError(t, err)
		require.NotNil(t, claimed)
		assert.Equal(t, JobStatusRunning, claimed.Status)
		assert.Equal(t, 1, claimed.AttemptCount)
		assert.Equal(t, "worker-a", claimed.WorkerID)
		result := HistoricalRawCandleBackfillResult{
			IngestionRunID: job.Input.IngestionRunID,
			PersistedCount: 2,
			ExpectedCount:  2,
		}
		require.NoError(t, store.MarkSucceeded(t.Context(), job.ID, "worker-a", result, now.Add(2*time.Minute)))
		succeeded, err := store.Get(t.Context(), job.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusSucceeded, succeeded.Status)
		require.NotNil(t, succeeded.Result)
		assert.Equal(t, 2, succeeded.Result.ExpectedCount)
		jobTwo := makeJob(now.Add(3 * time.Minute))
		_, err = store.Create(t.Context(), jobTwo)
		require.NoError(t, err)
		_, err = store.ClaimQueued(t.Context(), jobTwo.ID, "worker-b", now.Add(4*time.Minute))
		require.NoError(t, err)
		unsafe := &JobError{Code: "code", Summary: "summary", Details: strings.Repeat("x", 1200)}
		require.NoError(t, store.MarkFailed(t.Context(), jobTwo.ID, "worker-b", unsafe, now.Add(5*time.Minute)))
		failed, err := store.Get(t.Context(), jobTwo.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusFailed, failed.Status)
		require.NotNil(t, failed.Error)
		assert.LessOrEqual(t, len(failed.Error.Details), maxErrorDetailsLength)
	})

	t.Run("lists filtered pages and finds idempotency rows", func(t *testing.T) {
		store := makeStore(t, "store-list")
		now := time.Now().UTC()
		jobA := makeJob(now)
		jobB := makeJob(now.Add(time.Second))
		jobB.Status = JobStatusFailed
		jobB.Requester.Source = RequesterSourceAgent
		jobC := makeJob(now.Add(2 * time.Second))
		jobC.Status = JobStatusQueued
		for _, job := range []Job{jobA, jobB, jobC} {
			_, err := store.Create(t.Context(), job)
			require.NoError(t, err)
		}
		pageOne, err := store.List(t.Context(), ListParams{
			Statuses: []JobStatus{JobStatusQueued},
			Sources:  []RequesterSource{RequesterSourceOperator},
			Limit:    1,
		})
		require.NoError(t, err)
		require.Len(t, pageOne.Items, 1)
		assert.Equal(t, jobC.ID, pageOne.Items[0].ID)
		require.NotEmpty(t, pageOne.NextCursor)
		pageTwo, err := store.List(t.Context(), ListParams{
			Statuses: []JobStatus{JobStatusQueued},
			Sources:  []RequesterSource{RequesterSourceOperator},
			Limit:    1,
			Cursor:   pageOne.NextCursor,
		})
		require.NoError(t, err)
		require.Len(t, pageTwo.Items, 1)
		assert.Equal(t, jobA.ID, pageTwo.Items[0].ID)
		found, err := store.FindByIdempotencyKey(
			t.Context(),
			jobA.Requester,
			JobTypeHistoricalRawCandleBackfill,
			jobA.IdempotencyKey,
		)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, jobA.ID, found.ID)
	})

	t.Run("creates idempotent rows once across concurrent callers", func(t *testing.T) {
		store := makeStore(t, "store-idempotent-concurrent")
		require.NoError(t, store.db.Exec("PRAGMA busy_timeout = 5000").Error)
		now := time.Now().UTC()
		requester := Requester{UserID: "user-" + fake.UUID().V4(), Source: RequesterSourceOperator}
		idempotencyKey := "key-" + fake.UUID().V4()
		makeIdempotentJob := func(suffix string) Job {
			job := makeJob(now)
			job.ID = fake.UUID().V4()
			job.Requester = requester
			job.IdempotencyKey = idempotencyKey
			job.Input.IngestionRunID = "ingest-" + suffix + "-" + fake.UUID().V4()
			hash, err := HashInput(job.Input)
			require.NoError(t, err)
			job.InputHash = hash
			job.CorrelationID = "corr-" + suffix + "-" + fake.UUID().V4()
			return job
		}

		const callers = 6
		type createResult struct {
			job Job
			err error
		}
		results := make(chan createResult, callers)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := range callers {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				<-start
				created, _, err := store.CreateIdempotent(t.Context(), makeIdempotentJob(fmt.Sprintf("case-%d", index)))
				results <- createResult{job: created, err: err}
			}(i)
		}
		close(start)
		wg.Wait()
		close(results)

		createdIDs := map[string]struct{}{}
		for result := range results {
			require.NoError(t, result.err)
			createdIDs[result.job.ID] = struct{}{}
		}
		require.Len(t, createdIDs, 1)
		listed, err := store.List(t.Context(), ListParams{Limit: 10})
		require.NoError(t, err)
		require.Len(t, listed.Items, 1)
	})

	t.Run("claim queued waits through a transient sqlite writer lock", func(t *testing.T) {
		dsn := filepath.Join(t.TempDir(), "store-claim-locked.sqlite")
		store, err := NewStore(dsn, StoreOpts{TablePrefix: "test_"})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())

		now := time.Now().UTC()
		job := makeJob(now)
		_, err = store.Create(t.Context(), job)
		require.NoError(t, err)

		locker, err := sql.Open("sqlite", dsn)
		require.NoError(t, err)
		defer func() { require.NoError(t, locker.Close()) }()
		locker.SetMaxOpenConns(1)
		locker.SetMaxIdleConns(1)

		tx, err := locker.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		_, err = tx.ExecContext(
			t.Context(),
			"UPDATE test_jobs SET updated_at = ? WHERE id = ?",
			now.Add(time.Second),
			job.ID,
		)
		require.NoError(t, err)

		claimedCh := make(chan error, 1)
		go func() {
			_, claimErr := store.ClaimQueued(t.Context(), job.ID, "worker-lock", now.Add(2*time.Second))
			claimedCh <- claimErr
		}()

		time.Sleep(150 * time.Millisecond)
		require.NoError(t, tx.Commit())

		select {
		case claimErr := <-claimedCh:
			require.NoError(t, claimErr)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for claim queued to finish")
		}
	})

	t.Run("rejects empty idempotency keys for idempotent creates", func(t *testing.T) {
		store := makeStore(t, "store-idempotent-empty-key")
		job := makeJob(time.Now().UTC())
		job.IdempotencyKey = "   "
		_, _, err := store.CreateIdempotent(t.Context(), job)
		require.ErrorIs(t, err, ErrNoIdempotency)
	})

	t.Run("surfaces idempotency conflicts for reused keys with different canonical input", func(t *testing.T) {
		store := makeStore(t, "store-idempotent-conflict")
		now := time.Now().UTC()
		job := makeJob(now)
		_, createdNew, err := store.CreateIdempotent(t.Context(), job)
		require.NoError(t, err)
		assert.True(t, createdNew)

		conflict := makeJob(now.Add(time.Second))
		conflict.Requester = job.Requester
		conflict.IdempotencyKey = job.IdempotencyKey
		conflict.Input.Symbol = "ETH"
		hash, hashErr := HashInput(conflict.Input)
		require.NoError(t, hashErr)
		conflict.InputHash = hash

		loaded, createdNew, err := store.CreateIdempotent(t.Context(), conflict)
		require.Error(t, err)
		assert.False(t, createdNew)
		assert.True(t, IsIdempotencyConflict(err))
		assert.Equal(t, Job{}, loaded)
	})

	t.Run("covers not found and invalid cursor branches", func(t *testing.T) {
		store := makeStore(t, "store-errors")
		_, err := store.Get(t.Context(), fake.UUID().V4())
		require.ErrorIs(t, err, ErrJobNotFound)
		_, err = store.FindByIdempotencyKey(
			t.Context(),
			Requester{UserID: "u", Source: RequesterSourceOperator},
			JobTypeHistoricalRawCandleBackfill,
			"missing-key",
		)
		require.ErrorIs(t, err, ErrJobNotFound)
		_, err = store.FindByIdempotencyKey(
			t.Context(),
			Requester{UserID: "u", Source: RequesterSourceOperator},
			JobTypeHistoricalRawCandleBackfill,
			"",
		)
		require.ErrorIs(t, err, ErrNoIdempotency)
		_, err = store.List(t.Context(), ListParams{Cursor: "bad"})
		require.Error(t, err)
		input := HistoricalRawCandleBackfillInput{
			IngestionRunID: fake.UUID().V4(),
			Venue:          "hyperliquid-perps",
			Symbol:         "BTC",
			AssetClass:     "future",
			Timeframe:      "1h",
			Start:          time.Now().UTC().Add(-2 * time.Hour),
			End:            time.Now().UTC().Add(-time.Hour),
			PageSize:       100,
		}
		hash, hashErr := HashInput(input)
		require.NoError(t, hashErr)
		model, modelErr := newJobModel(Job{
			ID:        fake.UUID().V4(),
			JobType:   JobTypeHistoricalRawCandleBackfill,
			Status:    JobStatusSucceeded,
			Requester: Requester{UserID: "u", Source: RequesterSourceOperator},
			InputHash: hash,
			Input:     input,
			Result:    &HistoricalRawCandleBackfillResult{IngestionRunID: input.IngestionRunID},
			Error:     &JobError{Code: "c", Summary: "s", Details: "d"},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
			QueuedAt:  time.Now().UTC(),
		})
		require.NoError(t, modelErr)
		roundTrip, roundTripErr := jobFromModel(model)
		require.NoError(t, roundTripErr)
		assert.Equal(t, input.IngestionRunID, roundTrip.Result.IngestionRunID)
		otherInput := input
		otherInput.IngestionRunID = fake.UUID().V4()
		otherHash, otherHashErr := HashInput(otherInput)
		require.NoError(t, otherHashErr)
		assert.Equal(t, hash, otherHash)
		cursor := encodeCursor(time.Now().UTC(), "cursor-id")
		cursorTime, cursorID, decodeErr := decodeCursor(cursor)
		require.NoError(t, decodeErr)
		assert.Equal(t, "cursor-id", cursorID)
		assert.False(t, cursorTime.IsZero())
	})

	t.Run("surfaces store operation errors when the table is unavailable", func(t *testing.T) {
		store := makeStore(t, "store-broken")
		require.NoError(t, store.db.Exec("DROP TABLE "+store.tableName).Error)
		job := makeJob(time.Now().UTC())
		_, err := store.Create(t.Context(), job)
		require.Error(t, err)
		_, _, err = store.CreateIdempotent(t.Context(), job)
		require.Error(t, err)
		_, err = store.Get(t.Context(), job.ID)
		require.Error(t, err)
		_, err = store.FindByIdempotencyKey(
			t.Context(),
			job.Requester,
			JobTypeHistoricalRawCandleBackfill,
			job.IdempotencyKey,
		)
		require.Error(t, err)
		_, err = store.List(t.Context(), ListParams{Limit: 1})
		require.Error(t, err)
		_, err = store.ClaimQueued(t.Context(), job.ID, "worker", time.Now().UTC())
		require.Error(t, err)
		err = store.MarkSucceeded(
			t.Context(),
			job.ID,
			"worker",
			HistoricalRawCandleBackfillResult{IngestionRunID: job.Input.IngestionRunID},
			time.Now().UTC(),
		)
		require.Error(t, err)
		err = store.MarkFailed(
			t.Context(),
			job.ID,
			"worker",
			&JobError{Code: "c"},
			time.Now().UTC(),
		)
		require.Error(t, err)
		err = store.RecoverStaleRunning(t.Context(), time.Now().UTC(), 3)
		require.Error(t, err)
	})

	t.Run("supports transactions cancellation and result encoding helpers", func(t *testing.T) {
		store := makeStore(t, "store-transactions")
		require.NoError(t, store.WithTx(t.Context(), nil))

		now := time.Now().UTC()
		job := makeJob(now)
		require.NoError(t, store.WithTx(t.Context(), func(tx *StoreTx) error {
			require.NotNil(t, tx)
			require.NotNil(t, tx.SQLTx())
			_, createErr := tx.Create(t.Context(), job)
			return createErr
		}))

		persisted, err := store.Get(t.Context(), job.ID)
		require.NoError(t, err)
		assert.Equal(t, job.ID, persisted.ID)

		var nilTx *StoreTx
		assert.Nil(t, nilTx.SQLTx())

		nilJSON, err := resultJSONFromValue(nil)
		require.NoError(t, err)
		assert.Nil(t, nilJSON)

		rawJSON, err := resultJSONFromValue([]byte(`{"bytes":true}`))
		require.NoError(t, err)
		assert.JSONEq(t, `{"bytes":true}`, string(rawJSON))

		passthrough, err := resultJSONFromValue(rawJSON)
		require.NoError(t, err)
		assert.JSONEq(t, string(rawJSON), string(passthrough))

		_, err = resultJSONFromValue(func() {})
		require.Error(t, err)
	})

	t.Run("recovers stale running rows by requeueing or failing", func(t *testing.T) {
		store := makeStore(t, "store-recovery")
		now := time.Now().UTC().Add(-time.Hour)
		requeue := makeJob(now)
		requeue.Status = JobStatusRunning
		requeue.AttemptCount = 1
		startedAt := now.Add(2 * time.Minute)
		requeue.StartedAt = &startedAt
		requeue.WorkerID = "old-worker"
		requeue.LastAttemptAt = &startedAt
		exhausted := makeJob(now.Add(time.Minute))
		exhausted.Status = JobStatusRunning
		exhausted.AttemptCount = 3
		exhausted.StartedAt = &startedAt
		exhausted.WorkerID = "old-worker"
		exhausted.LastAttemptAt = &startedAt
		for _, job := range []Job{requeue, exhausted} {
			_, err := store.Create(t.Context(), job)
			require.NoError(t, err)
		}
		recoveryTime := now.Add(30 * time.Minute)
		require.NoError(t, store.RecoverStaleRunning(t.Context(), recoveryTime, 3))
		requeuedJob, err := store.Get(t.Context(), requeue.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusQueued, requeuedJob.Status)
		assert.Equal(t, requeue.AttemptCount, requeuedJob.AttemptCount)
		assert.Empty(t, requeuedJob.WorkerID)
		require.NotNil(t, requeuedJob.Error)
		assert.Equal(t, "stale_running_requeued", requeuedJob.Error.Code)
		exhaustedJob, err := store.Get(t.Context(), exhausted.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusFailed, exhaustedJob.Status)
		require.NotNil(t, exhaustedJob.Error)
		assert.Equal(t, "stale_running_attempts_exhausted", exhaustedJob.Error.Code)
	})

	t.Run("updates progress and schedule rows and lists due schedules", func(t *testing.T) {
		store := makeStore(t, "store-schedules-progress")
		now := time.Now().UTC()
		job := makeJob(now)
		_, err := store.Create(t.Context(), job)
		require.NoError(t, err)
		progressJSON := mustRegistryJSON(t, map[string]string{"stage": "queued"})
		require.NoError(t, store.UpdateProgress(t.Context(), job.ID, progressJSON, now.Add(time.Minute)))
		updated, err := store.Get(t.Context(), job.ID)
		require.NoError(t, err)
		require.NotNil(t, updated.ProgressJSON)

		schedule := Schedule{
			ID:        "sched-" + fake.UUID().V4(),
			JobType:   JobType("finance.fx_rates_sync"),
			Requester: Requester{UserID: "system", Source: RequesterSourceOperator},
			Interval:  time.Hour,
			NextRunAt: now.Add(-time.Minute),
			InputJSON: mustRegistryJSON(t, map[string]string{"scope": "daily"}),
		}
		require.NoError(t, store.UpsertSchedule(t.Context(), schedule))
		dueSchedules, err := store.ListDueSchedules(t.Context(), now)
		require.NoError(t, err)
		require.Len(t, dueSchedules, 1)
		assert.Equal(t, schedule.ID, dueSchedules[0].ID)

		require.NoError(t, store.MarkCanceled(t.Context(), job.ID, now.Add(2*time.Minute)))
		canceled, err := store.Get(t.Context(), job.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusCanceled, canceled.Status)

		schedule.NextRunAt = now.Add(2 * time.Hour)
		schedule.Enabled = false
		require.NoError(t, store.UpsertSchedule(t.Context(), schedule))
		dueSchedules, err = store.ListDueSchedules(t.Context(), now)
		require.NoError(t, err)
		assert.Empty(t, dueSchedules)
	})

	t.Run("surfaces malformed historical payload rows", func(t *testing.T) {
		_, err := jobFromModel(jobModel{
			JobType:   string(JobTypeHistoricalRawCandleBackfill),
			InputJSON: "{",
		})
		require.Error(t, err)

		_, err = jobFromModel(jobModel{
			JobType: string(JobTypeHistoricalRawCandleBackfill),
			InputJSON: string(mustRegistryJSON(t, HistoricalRawCandleBackfillInput{
				Venue:      "hyperliquid-perps",
				Symbol:     "BTC",
				AssetClass: "future",
				Timeframe:  "1m",
			})),
			ResultJSON: "{",
		})
		require.Error(t, err)
	})

	t.Run("list due schedules surfaces storage errors", func(t *testing.T) {
		store := makeStore(t, "store-due-schedule-errors")
		require.NoError(t, store.db.Exec("DROP TABLE "+store.scheduleTableName()).Error)
		_, err := store.ListDueSchedules(t.Context(), time.Now().UTC())
		require.Error(t, err)
	})
}
