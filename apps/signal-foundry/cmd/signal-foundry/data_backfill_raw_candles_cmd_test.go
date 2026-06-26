package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/flows"
	"github.com/gemyago/signal-foundry/runtime/venueedge"
	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

type fakeHistoricalRawCandleBackfillRunner struct {
	result   flows.HistoricalRawCandleBackfillResult
	err      error
	requests []flows.HistoricalRawCandleBackfillRequest
}

func (r *fakeHistoricalRawCandleBackfillRunner) Run(
	_ context.Context,
	request flows.HistoricalRawCandleBackfillRequest,
) (flows.HistoricalRawCandleBackfillResult, error) {
	r.requests = append(r.requests, request)
	return r.result, r.err
}

func makeBackfillRunnerFactory(
	runner *fakeHistoricalRawCandleBackfillRunner,
	beforeRun func(*cobra.Command, *dig.Container) error,
) backfillRawCandlesRunnerFactory {
	return func(cmd *cobra.Command, container *dig.Container) (*historicalRawCandleBackfillCommandRunner, error) {
		if beforeRun != nil {
			if err := beforeRun(cmd, container); err != nil {
				return nil, err
			}
		}

		return &historicalRawCandleBackfillCommandRunner{run: runner.Run}, nil
	}
}

func TestDataBackfillRawCandlesCmd(t *testing.T) {
	fake := faker.New()

	newRootWithFactory := func(factory backfillRawCandlesRunnerFactory) *cobra.Command {
		container := dig.New()
		rootCmd := newRootCmd()
		rootCmd.PersistentPreRunE = func(activeCmd *cobra.Command, _ []string) error {
			setPerCommandDefaults(activeCmd)
			return nil
		}
		rootCmd.AddCommand(newDataCmd(container, factory))
		return rootCmd
	}

	makeReport := func(t *testing.T, includeRawPayloadCount bool) flows.HistoricalRawCandleBackfillResult {
		t.Helper()

		start := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
		end := start.Add(3 * time.Minute)
		timeRange, err := domain.NewTimeRange(start, end)
		require.NoError(t, err)

		missingRange, err := domain.NewTimeRange(start.Add(2*time.Minute), end)
		require.NoError(t, err)

		var rawPayloadCount *int
		if includeRawPayloadCount {
			count := 2
			rawPayloadCount = &count
		}

		firstPersisted := start
		lastPersisted := start.Add(2 * time.Minute)

		return flows.HistoricalRawCandleBackfillResult{
			RunID: "run-" + fake.Lorem().Word(),
			Report: flows.HistoricalRawCandleBackfillReport{
				Venue:                       venueedge.HyperliquidPerpsVenueName,
				Symbol:                      domain.Symbol("BTC"),
				AssetClass:                  domain.AssetClassFuture,
				Timeframe:                   domain.Timeframe1m,
				TimeRange:                   timeRange,
				PersistedCount:              2,
				ExpectedCount:               3,
				MissingIntervalCount:        1,
				DuplicateNaturalKeyCount:    0,
				FirstPersistedStart:         &firstPersisted,
				LastPersistedEnd:            &lastPersisted,
				RawPayloadCount:             rawPayloadCount,
				MissingIntervalPreview:      []domain.TimeRange{missingRange},
				MissingIntervalPreviewLimit: 2,
			},
		}
	}

	makeExpectedOutput := func(runID string, includeRawPayloadCount bool) string {
		lines := []string{
			fmt.Sprintf("run_id: %s", runID),
			"venue: hyperliquid-perps",
			"symbol: BTC",
			"asset_class: future",
			"timeframe: 1m",
			"requested_start: 2026-01-02T03:04:05Z",
			"requested_end: 2026-01-02T03:07:05Z",
			"persisted_count: 2",
			"expected_count: 3",
			"gap_count: 1",
			"duplicate_natural_key_count: 0",
			"first_persisted_start: 2026-01-02T03:04:05Z",
			"last_persisted_end: 2026-01-02T03:06:05Z",
		}
		if includeRawPayloadCount {
			lines = append(lines, "raw_payload_count: 2")
		}
		lines = append(
			lines,
			"missing_interval_preview_count: 1",
			"missing_interval_preview_limit: 2",
			"missing_interval_1_start: 2026-01-02T03:06:05Z",
			"missing_interval_1_end: 2026-01-02T03:07:05Z",
		)
		return strings.Join(lines, "\n") + "\n"
	}

	t.Run("wires data backfill-raw-candles under the root command", func(t *testing.T) {
		rootCmd := setupCommands()

		dataCmd, _, err := rootCmd.Find([]string{"data", "backfill-raw-candles"})
		require.NoError(t, err)
		require.NotNil(t, dataCmd)
		assert.Equal(t, "backfill-raw-candles", dataCmd.Name())
	})

	t.Run("default runner factory returns engine setup errors", func(t *testing.T) {
		rootCmd := newRootCmd()
		require.NoError(t, rootCmd.PersistentFlags().Set("log-level", "definitely-not-a-level"))

		_, err := newHistoricalRawCandleBackfillRunnerFromContainer(rootCmd, dig.New())
		require.Error(t, err)
	})

	t.Run("default runner factory builds a real runner from app dependencies", func(t *testing.T) {
		chdirModuleRoot(t)

		dataDir := filepath.Join(t.TempDir(), "data")
		dataLayerDSN := filepath.Join(t.TempDir(), "data-layer.sqlite")
		t.Setenv("APP_DATADIR", dataDir)
		t.Setenv("APP_DATALAYER_DATABASE_DSN", dataLayerDSN)
		migrateAppDatabaseForTests(t, dataLayerDSN)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"upstream boom"}`))
		}))
		defer server.Close()

		serverClient := server.Client()
		httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			proxied := req.Clone(req.Context())
			proxied.URL.Scheme = "http"
			proxied.URL.Host = strings.TrimPrefix(server.URL, "http://")
			proxied.Host = proxied.URL.Host
			return serverClient.Transport.RoundTrip(proxied)
		})}

		rootCmd := newRootCmd()
		require.NoError(t, rootCmd.PersistentFlags().Set("env", "test"))
		require.NoError(t, rootCmd.PersistentFlags().Set("logs-file", testLogFile(t)))

		runner, err := newHistoricalRawCandleBackfillRunnerFromContainerWithHTTPClient(
			rootCmd,
			dig.New(),
			server.URL,
			httpClient,
		)
		require.NoError(t, err)

		timeRange, err := domain.NewTimeRange(
			time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
			time.Date(2026, time.January, 2, 3, 9, 5, 0, time.UTC),
		)
		require.NoError(t, err)

		_, err = runner.Run(t.Context(), flows.HistoricalRawCandleBackfillRequest{
			RunID:      "run-123",
			Venue:      venueedge.HyperliquidPerpsVenueName,
			Symbol:     domain.Symbol("BTC"),
			AssetClass: domain.AssetClassFuture,
			Timeframe:  domain.Timeframe1m,
			TimeRange:  timeRange,
		})
		require.Error(t, err)
	})

	t.Run("maps valid flags to a canonical runner request", func(t *testing.T) {
		start := time.Date(2026, time.February, 3, 4, 5, 6, 0, time.FixedZone("UTC+02", 2*60*60))
		end := start.Add(3 * time.Minute)

		runner := &fakeHistoricalRawCandleBackfillRunner{result: makeReport(t, true)}
		factoryCalls := 0
		rootCmd := newRootWithFactory(makeBackfillRunnerFactory(runner, func(*cobra.Command, *dig.Container) error {
			factoryCalls++
			return nil
		}))
		rootCmd.SilenceErrors = true
		rootCmd.SilenceUsage = true

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{
			"data",
			"backfill-raw-candles",
			"--run-id", "  run-123  ",
			"--venue", "hyperliquid-perps",
			"--symbol", "  BTC  ",
			"--asset-class", "future",
			"--timeframe", "1m",
			"--start", start.Format(time.RFC3339),
			"--end", end.Format(time.RFC3339),
			"--page-size", "250",
		})

		require.NoError(t, rootCmd.ExecuteContext(t.Context()))
		require.Len(t, runner.requests, 1)
		assert.Equal(t, 1, factoryCalls)

		requestedRange, err := domain.NewTimeRange(start.UTC(), end.UTC())
		require.NoError(t, err)
		assert.Equal(t, flows.HistoricalRawCandleBackfillRequest{
			RunID:      "run-123",
			Venue:      venueedge.HyperliquidPerpsVenueName,
			Symbol:     domain.Symbol("BTC"),
			AssetClass: domain.AssetClassFuture,
			Timeframe:  domain.Timeframe1m,
			TimeRange:  requestedRange,
			PageSize:   250,
		}, runner.requests[0])
	})

	t.Run("reuses app config, database, blob, and Hyperliquid defaults when wiring dependencies", func(t *testing.T) {
		chdirModuleRoot(t)

		dataDir := filepath.Join(t.TempDir(), "data")
		dataLayerDSN := filepath.Join(t.TempDir(), "data-layer.sqlite")
		t.Setenv("APP_DATADIR", dataDir)
		t.Setenv("APP_DATALAYER_DATABASE_DSN", dataLayerDSN)
		migrateAppDatabaseForTests(t, dataLayerDSN)

		runner := &fakeHistoricalRawCandleBackfillRunner{result: makeReport(t, false)}
		factoryCalls := 0
		factory := makeBackfillRunnerFactory(runner, func(cmd *cobra.Command, container *dig.Container) error {
			factoryCalls++

			_, err := newEngineFromRoot(cmd.Root(), container)
			require.NoError(t, err)

			type resolvedDeps struct {
				dig.In

				Runtime             *internal.Runtime
				ConfiguredDataDir   string `name:"config.dataDir"`
				ConfiguredDatabase  string `name:"config.dataLayer.database.dsn"`
				ConfiguredBlobStore string `name:"config.dataLayer.rawPayloadBlobStore.path"`
			}

			err = container.Invoke(func(deps resolvedDeps) {
				require.NotNil(t, deps.Runtime)
				require.NotNil(t, deps.Runtime.DataStore)
				require.NotNil(t, deps.Runtime.DataReadService)
				require.NotNil(t, deps.Runtime.DataLineageService)
				require.NotNil(t, deps.Runtime.VenueIngestionFlow)
				require.NotNil(t, deps.Runtime.HyperliquidRecorder)

				assert.Equal(t, dataDir, deps.ConfiguredDataDir)
				assert.Equal(t, dataLayerDSN, deps.ConfiguredDatabase)
				assert.Empty(t, deps.ConfiguredBlobStore)

				instrument, instrumentErr := domain.NewInstrument(domain.InstrumentParams{
					Venue:      venueedge.HyperliquidPerpsVenueName,
					Symbol:     domain.Symbol("BTC"),
					AssetClass: domain.AssetClassFuture,
					Active:     true,
				})
				require.NoError(t, instrumentErr)

				_, recordErr := deps.Runtime.HyperliquidRecorder.RecordHyperliquidRawEvidence(
					cmd.Context(),
					venueedge.HyperliquidRawEvidenceCapture{
						ID:                 "raw-" + fake.Lorem().Word(),
						Venue:              venueedge.HyperliquidPerpsVenueName,
						Endpoint:           "/info",
						RequestType:        "candleSnapshot",
						RequestPayloadHash: "req-" + fake.Lorem().Word(),
						RequestMetadata:    map[string]string{"method": "POST"},
						RequestAt:          time.Now().Add(-time.Second).UTC(),
						ResponseAt:         time.Now().UTC(),
						HTTPStatus:         200,
						ResponseBody:       []byte(`[{"t":1}]`),
						EntityHint:         "candle",
						Instrument:         &instrument,
						Timeframe:          domain.Timeframe1m,
						ReceivedAt:         time.Now().UTC(),
					},
				)
				require.NoError(t, recordErr)

				entries, readErr := os.ReadDir(filepath.Join(dataDir, "raw-payloads"))
				require.NoError(t, readErr)
				assert.NotEmpty(t, entries)
			})
			require.NoError(t, err)

			return nil
		})
		rootCmd := newRootWithFactory(factory)
		rootCmd.SilenceErrors = true
		rootCmd.SilenceUsage = true
		rootCmd.SetArgs([]string{
			"data",
			"backfill-raw-candles",
			"-e", "test",
			"--run-id", "run-123",
			"--venue", "hyperliquid-perps",
			"--symbol", "BTC",
			"--asset-class", "future",
			"--timeframe", "1m",
			"--start", "2026-01-02T03:04:05Z",
			"--end", "2026-01-02T03:07:05Z",
			"--logs-file", testLogFile(t),
		})

		require.NoError(t, rootCmd.ExecuteContext(t.Context()))
		assert.Equal(t, 1, factoryCalls)
	})

	t.Run("returns a non-zero error for invalid input and skips dependency wiring", func(t *testing.T) {
		factoryCalls := 0
		factory := func(
			_ *cobra.Command,
			_ *dig.Container,
		) (*historicalRawCandleBackfillCommandRunner, error) {
			factoryCalls++
			return &historicalRawCandleBackfillCommandRunner{run: (&fakeHistoricalRawCandleBackfillRunner{}).Run}, nil
		}
		rootCmd := newRootWithFactory(factory)
		rootCmd.SilenceErrors = true
		rootCmd.SilenceUsage = true
		rootCmd.SetArgs([]string{
			"data",
			"backfill-raw-candles",
			"--run-id", "run-123",
			"--venue", "hyperliquid-perps",
			"--symbol", "BTC",
			"--asset-class", "future",
			"--timeframe", "1m",
			"--start", "not-a-timestamp",
			"--end", "2026-01-02T03:07:05Z",
		})

		err := rootCmd.ExecuteContext(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, err, "parse --start")
		assert.Zero(t, factoryCalls)
	})

	t.Run("returns runner factory errors without executing the runner", func(t *testing.T) {
		factory := func(
			_ *cobra.Command,
			_ *dig.Container,
		) (*historicalRawCandleBackfillCommandRunner, error) {
			return nil, errors.New("factory boom")
		}
		rootCmd := newRootWithFactory(factory)
		rootCmd.SilenceErrors = true
		rootCmd.SilenceUsage = true
		rootCmd.SetArgs([]string{
			"data",
			"backfill-raw-candles",
			"--run-id", "run-123",
			"--venue", "hyperliquid-perps",
			"--symbol", "BTC",
			"--asset-class", "future",
			"--timeframe", "1m",
			"--start", "2026-01-02T03:04:05Z",
			"--end", "2026-01-02T03:07:05Z",
		})

		err := rootCmd.ExecuteContext(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, err, "factory boom")
	})

	t.Run("returns runner errors without rendering output", func(t *testing.T) {
		runner := &fakeHistoricalRawCandleBackfillRunner{err: errors.New("run boom")}
		rootCmd := newRootWithFactory(makeBackfillRunnerFactory(runner, nil))
		rootCmd.SilenceErrors = true
		rootCmd.SilenceUsage = true

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{
			"data",
			"backfill-raw-candles",
			"--run-id", "run-123",
			"--venue", "hyperliquid-perps",
			"--symbol", "BTC",
			"--asset-class", "future",
			"--timeframe", "1m",
			"--start", "2026-01-02T03:04:05Z",
			"--end", "2026-01-02T03:07:05Z",
		})

		err := rootCmd.ExecuteContext(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, err, "run boom")
		assert.Empty(t, stdout.String())
	})

	t.Run("returns a validation error when the requested range is not half-open", func(t *testing.T) {
		_, err := historicalRawCandleBackfillRequestFromFlags(backfillRawCandlesCmdParams{
			RunID:      "run-123",
			Venue:      "hyperliquid-perps",
			Symbol:     "BTC",
			AssetClass: "future",
			Timeframe:  "1m",
			Start:      "2026-01-02T03:07:05Z",
			End:        "2026-01-02T03:07:05Z",
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "build requested time range")
	})

	t.Run("returns a parse error for an invalid end timestamp", func(t *testing.T) {
		_, err := historicalRawCandleBackfillRequestFromFlags(backfillRawCandlesCmdParams{
			Start: "2026-01-02T03:04:05Z",
			End:   "bad-end",
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "parse --end")
	})

	t.Run("renders deterministic output including optional raw payload count", func(t *testing.T) {
		runner := &fakeHistoricalRawCandleBackfillRunner{result: makeReport(t, true)}
		rootCmd := newRootWithFactory(makeBackfillRunnerFactory(runner, nil))
		rootCmd.SilenceErrors = true
		rootCmd.SilenceUsage = true

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{
			"data",
			"backfill-raw-candles",
			"--run-id", "run-123",
			"--venue", "hyperliquid-perps",
			"--symbol", "BTC",
			"--asset-class", "future",
			"--timeframe", "1m",
			"--start", "2026-01-02T03:04:05Z",
			"--end", "2026-01-02T03:07:05Z",
		})

		require.NoError(t, rootCmd.ExecuteContext(t.Context()))
		assert.Equal(t, makeExpectedOutput(runner.result.RunID, true), stdout.String())
	})

	t.Run("omits raw payload count deterministically when it is unavailable", func(t *testing.T) {
		runner := &fakeHistoricalRawCandleBackfillRunner{result: makeReport(t, false)}
		rootCmd := newRootWithFactory(makeBackfillRunnerFactory(runner, nil))
		rootCmd.SilenceErrors = true
		rootCmd.SilenceUsage = true

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{
			"data",
			"backfill-raw-candles",
			"--run-id", "run-123",
			"--venue", "hyperliquid-perps",
			"--symbol", "BTC",
			"--asset-class", "future",
			"--timeframe", "1m",
			"--start", "2026-01-02T03:04:05Z",
			"--end", "2026-01-02T03:07:05Z",
		})

		require.NoError(t, rootCmd.ExecuteContext(t.Context()))
		assert.NotContains(t, stdout.String(), "raw_payload_count:")
		assert.Equal(t, makeExpectedOutput(runner.result.RunID, false), stdout.String())
	})

	t.Run("renders none for missing persisted candle boundaries", func(t *testing.T) {
		result := makeReport(t, false)
		result.Report.FirstPersistedStart = nil
		result.Report.LastPersistedEnd = nil

		var stdout bytes.Buffer
		require.NoError(t, renderHistoricalRawCandleBackfillResult(&stdout, result))
		assert.Contains(t, stdout.String(), "first_persisted_start: none\n")
		assert.Contains(t, stdout.String(), "last_persisted_end: none\n")
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
