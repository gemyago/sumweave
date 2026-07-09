package main

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/lifecycle"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/startupmode"
	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

func TestJobsCommand(t *testing.T) {
	chdirModuleRoot(t)
	fake := faker.New()

	t.Run("worker command supports one-shot worker mode", func(t *testing.T) {
		dsn := filepath.Join(t.TempDir(), "jobs-worker.sqlite")
		t.Setenv("APP_DATALAYER_DATABASE_DSN", dsn)
		t.Setenv("APP_JOBS_WORKER_ENABLED", "false")
		t.Setenv("APP_JOBS_WORKER_POLLINTERVAL", "10ms")
		migrateAppDatabaseForTests(t, dsn)
		rootCmd := setupCommands()
		rootCmd.SetArgs([]string{"jobs", "worker", "-e", "test", "--once", "--logs-file", testLogFile(t)})
		require.NoError(t, rootCmd.Execute())
	})

	t.Run("worker one-shot flag reflects consumer wording", func(t *testing.T) {
		workerCmd := newJobsWorkerCmd(dig.New())
		onceFlag := workerCmd.Flags().Lookup("once")
		require.NotNil(t, onceFlag)
		assert.NotContains(t, onceFlag.Usage, "polling")
		assert.Contains(t, onceFlag.Usage, "consumer")
	})

	t.Run("worker command leaves queued sqlite jobs untouched without a dispatch message", func(t *testing.T) {
		dsn := filepath.Join(t.TempDir(), "jobs-worker-exec.sqlite")
		t.Setenv("APP_DATALAYER_DATABASE_DSN", dsn)
		t.Setenv("APP_JOBS_WORKER_ENABLED", "true")
		t.Setenv("APP_JOBS_WORKER_POLLINTERVAL", "10ms")
		migrateAppDatabaseForTests(t, dsn)

		store, jobID := makeQueuedUnknownJob(t, dsn)

		rootCmd := setupCommands()
		rootCmd.SetArgs([]string{"jobs", "worker", "-e", "test", "--once", "--logs-file", testLogFile(t)})
		require.NoError(t, rootCmd.Execute())

		persisted, err := store.Get(t.Context(), jobID)
		require.NoError(t, err)
		assert.Equal(t, jobspkg.JobStatusQueued, persisted.Status)
		assert.Empty(t, persisted.WorkerID)
		assert.Equal(t, 0, persisted.AttemptCount)
		assert.Nil(t, persisted.Error)
	})

	t.Run("enqueue-due command enqueues only once per due window", func(t *testing.T) {
		dsn := filepath.Join(t.TempDir(), "jobs-scheduler.sqlite")
		t.Setenv("APP_DATALAYER_DATABASE_DSN", dsn)
		t.Setenv("APP_JOBS_WORKER_ENABLED", "false")
		migrateAppDatabaseForTests(t, dsn)
		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		defer func() { require.NoError(t, sqlDB.Close()) }()
		store, err := jobspkg.NewStore(sqlDB, dsn, jobspkg.StoreOpts{TablePrefix: "signal_foundry_data_jobs_"})
		require.NoError(t, err)
		runAt := time.Now().UTC().Add(-time.Minute)
		require.NoError(t, store.UpsertSchedule(t.Context(), jobspkg.Schedule{
			ID:        "sched-" + fake.UUID().V4(),
			JobType:   jobspkg.JobTypeHistoricalRawCandleBackfill,
			Requester: jobspkg.Requester{UserID: "system", Source: jobspkg.RequesterSourceOperator},
			Interval:  time.Hour,
			NextRunAt: runAt,
			InputJSON: mustMarshalCommandJSON(t, jobspkg.HistoricalRawCandleBackfillInput{
				IngestionRunID: fake.UUID().V4(),
				Venue:          "hyperliquid-perps",
				Symbol:         "BTC",
				AssetClass:     "future",
				Timeframe:      "1m",
				Start:          runAt.Add(-time.Minute),
				End:            runAt,
				PageSize:       10,
			}),
		}))
		rootCmd := setupCommands()
		rootCmd.SetArgs([]string{"jobs", "enqueue-due", "-e", "test", "--logs-file", testLogFile(t)})
		require.NoError(t, rootCmd.Execute())
		firstPage, err := store.List(t.Context(), jobspkg.ListParams{Limit: 10})
		require.NoError(t, err)
		require.Len(t, firstPage.Items, 1)
		assert.Equal(t, jobspkg.JobStatusQueued, firstPage.Items[0].Status)
		assert.Nil(t, firstPage.Items[0].StartedAt)
		assert.Nil(t, firstPage.Items[0].CompletedAt)
		rootCmd = setupCommands()
		rootCmd.SetArgs([]string{"jobs", "enqueue-due", "-e", "test", "--logs-file", testLogFile(t)})
		require.NoError(t, rootCmd.Execute())
		secondPage, err := store.List(t.Context(), jobspkg.ListParams{Limit: 10})
		require.NoError(t, err)
		require.Len(t, secondPage.Items, 1)
	})

	t.Run(
		"worker and scheduler resolvers keep startup auto-start disabled even when worker config is enabled",
		func(t *testing.T) {
			makeResolvedContainer := func(
				t *testing.T,
				commandName string,
				resolve func(*cobra.Command, *dig.Container) error,
			) *dig.Container {
				t.Helper()
				dsn := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
				t.Setenv("APP_DATALAYER_DATABASE_DSN", dsn)
				t.Setenv("APP_JOBS_WORKER_ENABLED", "true")
				migrateAppDatabaseForTests(t, dsn)

				container := dig.New()
				rootCmd := newRootCmd()
				rootCmd.SetContext(t.Context())
				require.NoError(t, rootCmd.PersistentFlags().Set("env", "test"))
				require.NoError(t, rootCmd.PersistentFlags().Set("logs-file", testLogFile(t)))
				rootCmd.AddCommand(newJobsCmd(container))
				command, _, err := rootCmd.Find([]string{"jobs", commandName})
				require.NoError(t, err)

				require.NoError(t, resolve(command, container))
				return container
			}

			container := makeResolvedContainer(t, "worker", func(cmd *cobra.Command, container *dig.Container) error {
				worker, err := resolveJobsWorker(cmd, container)
				require.NoError(t, err)
				_, ok := worker.(*jobspkg.Worker)
				require.True(t, ok)
				return err
			})
			assertNoJobsWorkerHook(t, container)

			container = makeResolvedContainer(
				t,
				enqueueDueCommandName,
				func(cmd *cobra.Command, container *dig.Container) error {
					scheduler, err := resolveJobsScheduler(cmd, container)
					require.NoError(t, err)
					require.NotNil(t, scheduler)
					return err
				},
			)
			assertNoJobsWorkerHook(t, container)
		},
	)

	t.Run("worker and scheduler commands surface resolver and runner errors", func(t *testing.T) {
		resolverErr := errors.New("resolver failed")
		workerCmd := newJobsWorkerCmdWithResolver(
			dig.New(),
			func(*cobra.Command, *dig.Container) (jobsWorkerRunner, error) {
				return nil, resolverErr
			},
		)
		workerCmd.SetContext(t.Context())
		require.ErrorIs(t, workerCmd.RunE(workerCmd, nil), resolverErr)

		workerCmd = newJobsWorkerCmdWithResolver(
			dig.New(),
			func(*cobra.Command, *dig.Container) (jobsWorkerRunner, error) {
				return workerRunnerStub{runErr: errors.New("run failed")}, nil
			},
		)
		workerCmd.SetContext(t.Context())
		require.EqualError(t, workerCmd.RunE(workerCmd, nil), "run failed")

		require.NoError(t, workerCmd.Flags().Set("once", "true"))
		workerCmd.SetContext(t.Context())
		require.EqualError(t, workerCmd.RunE(workerCmd, nil), "run failed")

		schedulerCmd := newJobsEnqueueDueCmdWithResolver(
			dig.New(),
			func(*cobra.Command, *dig.Container) (jobsSchedulerRunner, error) {
				return nil, resolverErr
			},
		)
		schedulerCmd.SetContext(t.Context())
		require.ErrorIs(t, schedulerCmd.RunE(schedulerCmd, nil), resolverErr)

		schedulerCmd = newJobsEnqueueDueCmdWithResolver(
			dig.New(),
			func(*cobra.Command, *dig.Container) (jobsSchedulerRunner, error) {
				return schedulerRunnerStub{err: errors.New("enqueue failed")}, nil
			},
		)
		schedulerCmd.SetContext(t.Context())
		require.EqualError(t, schedulerCmd.RunE(schedulerCmd, nil), "enqueue failed")
	})

	t.Run("worker and scheduler resolvers surface engine setup errors", func(t *testing.T) {
		rootCmd := &cobra.Command{Use: "signal-foundry"}
		jobsCmd := &cobra.Command{Use: jobsCommandName}
		workerCmd := &cobra.Command{Use: "worker"}
		schedulerCmd := &cobra.Command{Use: enqueueDueCommandName}
		rootCmd.AddCommand(jobsCmd)
		jobsCmd.AddCommand(workerCmd, schedulerCmd)

		_, err := resolveJobsWorker(workerCmd, dig.New())
		require.Error(t, err)

		_, err = resolveJobsScheduler(schedulerCmd, dig.New())
		require.Error(t, err)
	})

	t.Run("prime finance jobs tolerates nil and surfaces missing service", func(t *testing.T) {
		require.NoError(t, primeFinanceJobs(nil))
		require.Error(t, primeFinanceJobs(dig.New()))
	})
}

func mustMarshalCommandJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := jobspkg.EncodeJobPayload(value)
	require.NoError(t, err)
	return payload
}

type workerRunnerStub struct {
	runErr error
}

func (w workerRunnerStub) Run(context.Context) error     { return w.runErr }
func (w workerRunnerStub) RunOnce(context.Context) error { return w.runErr }

type schedulerRunnerStub struct {
	err error
}

func (s schedulerRunnerStub) EnqueueDue(context.Context) (int, error) { return 0, s.err }

func assertNoJobsWorkerHook(t *testing.T, container *dig.Container) {
	t.Helper()
	type autoStartDeps struct {
		dig.In

		AutoStart *startupmode.JobsWorkerAutoStart `name:"internal.jobs.worker.autoStart"`
	}
	var autoStart autoStartDeps
	require.NoError(t, container.Invoke(func(resolved autoStartDeps) {
		autoStart = resolved
	}))
	require.NotNil(t, autoStart.AutoStart)
	require.False(t, autoStart.AutoStart.Enabled)

	var hooks *lifecycle.ShutdownHooks
	require.NoError(t, container.Invoke(func(resolved *lifecycle.ShutdownHooks, _ *jobspkg.Worker) {
		hooks = resolved
	}))
	hooksField := reflect.ValueOf(hooks).Elem().FieldByName("hooks")
	require.True(t, hooksField.IsValid())
	for index := range hooksField.Len() {
		nameField := hooksField.Index(index).FieldByName("name")
		require.NotEqual(t, "jobs-worker", nameField.String())
	}
}
