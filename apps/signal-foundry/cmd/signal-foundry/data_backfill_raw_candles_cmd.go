package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/flows"
	"github.com/gemyago/signal-foundry/runtime/venueedge"
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

const (
	dataCommandName                               = "data"
	backfillRawCandlesCommandName                 = "backfill-raw-candles"
	historicalRawCandleBackfillHyperliquidBaseURL = "https://api.hyperliquid.xyz"
)

type historicalRawCandleBackfillCommandRunner struct {
	run func(
		context.Context,
		flows.HistoricalRawCandleBackfillRequest,
	) (flows.HistoricalRawCandleBackfillResult, error)
}

func (r *historicalRawCandleBackfillCommandRunner) Run(
	ctx context.Context,
	request flows.HistoricalRawCandleBackfillRequest,
) (flows.HistoricalRawCandleBackfillResult, error) {
	return r.run(ctx, request)
}

type backfillRawCandlesRunnerFactory func(
	cmd *cobra.Command,
	container *dig.Container,
) (*historicalRawCandleBackfillCommandRunner, error)

type backfillRawCandlesCmdParams struct {
	RunID      string
	Venue      string
	Symbol     string
	AssetClass string
	Timeframe  string
	Start      string
	End        string
	PageSize   int
}

func newDataCmd(container *dig.Container, factory backfillRawCandlesRunnerFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   dataCommandName,
		Short: "Run manual data operations",
	}
	cmd.AddCommand(newDataBackfillRawCandlesCmd(container, factory))
	return cmd
}

func newDataBackfillRawCandlesCmd(container *dig.Container, factory backfillRawCandlesRunnerFactory) *cobra.Command {
	if factory == nil {
		factory = newHistoricalRawCandleBackfillRunnerFromContainer
	}

	params := backfillRawCandlesCmdParams{}
	cmd := &cobra.Command{
		Use:   backfillRawCandlesCommandName,
		Short: "Backfill historical Hyperliquid raw candles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			request, err := historicalRawCandleBackfillRequestFromFlags(params)
			if err != nil {
				return err
			}

			runner, err := factory(cmd, container)
			if err != nil {
				return err
			}

			result, err := runner.Run(cmd.Context(), request)
			if err != nil {
				return err
			}

			return renderHistoricalRawCandleBackfillResult(cmd.OutOrStdout(), result)
		},
	}

	cmd.Flags().StringVar(&params.RunID, "run-id", "", "Stable ingestion run ID for this backfill")
	cmd.Flags().StringVar(&params.Venue, "venue", "hyperliquid-perps", "Canonical venue name")
	cmd.Flags().StringVar(&params.Symbol, "symbol", "", "Instrument symbol")
	cmd.Flags().StringVar(&params.AssetClass, "asset-class", "future", "Canonical asset class")
	cmd.Flags().StringVar(&params.Timeframe, "timeframe", "", "Canonical candle timeframe")
	cmd.Flags().StringVar(&params.Start, "start", "", "Inclusive RFC3339 start timestamp")
	cmd.Flags().StringVar(&params.End, "end", "", "Exclusive RFC3339 end timestamp")
	cmd.Flags().IntVar(&params.PageSize, "page-size", 0, "Optional venue page size override")
	_ = cmd.MarkFlagRequired("run-id")
	_ = cmd.MarkFlagRequired("symbol")
	_ = cmd.MarkFlagRequired("timeframe")
	_ = cmd.MarkFlagRequired("start")
	_ = cmd.MarkFlagRequired("end")

	return cmd
}

func newHistoricalRawCandleBackfillRunnerFromContainer(
	cmd *cobra.Command,
	container *dig.Container,
) (*historicalRawCandleBackfillCommandRunner, error) {
	return newHistoricalRawCandleBackfillRunnerFromContainerWithHTTPClient(
		cmd,
		container,
		historicalRawCandleBackfillHyperliquidBaseURL,
		http.DefaultClient,
	)
}

func newHistoricalRawCandleBackfillRunnerFromContainerWithHTTPClient(
	cmd *cobra.Command,
	container *dig.Container,
	baseURL string,
	httpClient *http.Client,
) (*historicalRawCandleBackfillCommandRunner, error) {
	if _, err := newEngineFromRoot(cmd.Root(), container); err != nil {
		return nil, err
	}

	type resolvedDeps struct {
		dig.In

		Runtime *internal.Runtime
	}

	var runtimeDeps resolvedDeps
	if err := container.Invoke(func(deps resolvedDeps) {
		runtimeDeps = deps
	}); err != nil {
		return nil, fmt.Errorf("resolve runtime dependencies: %w", err)
	}

	runner, err := flows.NewHistoricalRawCandleBackfillRunner(flows.HistoricalRawCandleBackfillRunnerDeps{
		RecordIngestionRun: runtimeDeps.Runtime.DataLineageService.RecordIngestionRun,
		BuildVenue: func(
			_ context.Context,
			params flows.HistoricalRawCandleBackfillVenueBuildParams,
		) (venueedge.MarketDataVenue, error) {
			return venueedge.NewHyperliquidPerpsVenue(venueedge.HyperliquidPerpsVenueParams{
				BaseURL:                 baseURL,
				HTTPClient:              httpClient,
				RawEvidenceRecorder:     runtimeDeps.Runtime.HyperliquidRecorder,
				RawEvidenceIngestionRun: params.RawEvidenceIngestionRun,
			})
		},
		IngestCandles:          runtimeDeps.Runtime.VenueIngestionFlow.IngestCandles,
		ReadPersistedCandles:   runtimeDeps.Runtime.DataReadService.QueryCandles,
		ReplayPersistedCandles: runtimeDeps.Runtime.DataReadService.ReplayCandles,
	})
	if err != nil {
		return nil, fmt.Errorf("create historical raw candle backfill runner: %w", err)
	}

	return &historicalRawCandleBackfillCommandRunner{run: runner.Run}, nil
}

func historicalRawCandleBackfillRequestFromFlags(
	params backfillRawCandlesCmdParams,
) (flows.HistoricalRawCandleBackfillRequest, error) {
	start, err := parseHistoricalRawCandleBackfillTimestamp("--start", params.Start)
	if err != nil {
		return flows.HistoricalRawCandleBackfillRequest{}, err
	}
	end, err := parseHistoricalRawCandleBackfillTimestamp("--end", params.End)
	if err != nil {
		return flows.HistoricalRawCandleBackfillRequest{}, err
	}

	timeRange, err := domain.NewTimeRange(start, end)
	if err != nil {
		return flows.HistoricalRawCandleBackfillRequest{}, fmt.Errorf("build requested time range: %w", err)
	}

	return flows.HistoricalRawCandleBackfillRequest{
		RunID:      strings.TrimSpace(params.RunID),
		Venue:      domain.Venue(strings.TrimSpace(params.Venue)),
		Symbol:     domain.Symbol(strings.TrimSpace(params.Symbol)),
		AssetClass: domain.AssetClass(strings.TrimSpace(params.AssetClass)),
		Timeframe:  domain.Timeframe(strings.TrimSpace(params.Timeframe)),
		TimeRange:  timeRange,
		PageSize:   params.PageSize,
	}, nil
}

func parseHistoricalRawCandleBackfillTimestamp(flagName string, value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %s: %w", flagName, err)
	}
	return parsed.UTC(), nil
}

func renderHistoricalRawCandleBackfillResult(
	out io.Writer,
	result flows.HistoricalRawCandleBackfillResult,
) error {
	report := result.Report

	lines := []string{
		fmt.Sprintf("run_id: %s", result.RunID),
		fmt.Sprintf("venue: %s", report.Venue),
		fmt.Sprintf("symbol: %s", report.Symbol),
		fmt.Sprintf("asset_class: %s", report.AssetClass),
		fmt.Sprintf("timeframe: %s", report.Timeframe),
		fmt.Sprintf("requested_start: %s", formatHistoricalRawCandleBackfillTime(report.TimeRange.Start)),
		fmt.Sprintf("requested_end: %s", formatHistoricalRawCandleBackfillTime(report.TimeRange.End)),
		fmt.Sprintf("persisted_count: %d", report.PersistedCount),
		fmt.Sprintf("expected_count: %d", report.ExpectedCount),
		fmt.Sprintf("gap_count: %d", report.MissingIntervalCount),
		fmt.Sprintf("duplicate_natural_key_count: %d", report.DuplicateNaturalKeyCount),
		fmt.Sprintf("first_persisted_start: %s", formatHistoricalRawCandleBackfillTimePtr(report.FirstPersistedStart)),
		fmt.Sprintf("last_persisted_end: %s", formatHistoricalRawCandleBackfillTimePtr(report.LastPersistedEnd)),
	}
	if report.RawPayloadCount != nil {
		lines = append(lines, fmt.Sprintf("raw_payload_count: %d", *report.RawPayloadCount))
	}
	lines = append(
		lines,
		fmt.Sprintf("missing_interval_preview_count: %d", len(report.MissingIntervalPreview)),
		fmt.Sprintf("missing_interval_preview_limit: %d", report.MissingIntervalPreviewLimit),
	)
	for i, missing := range report.MissingIntervalPreview {
		lines = append(
			lines,
			fmt.Sprintf("missing_interval_%d_start: %s", i+1, formatHistoricalRawCandleBackfillTime(missing.Start)),
			fmt.Sprintf("missing_interval_%d_end: %s", i+1, formatHistoricalRawCandleBackfillTime(missing.End)),
		)
	}

	_, err := io.WriteString(out, strings.Join(lines, "\n")+"\n")
	return err
}

func formatHistoricalRawCandleBackfillTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}

func formatHistoricalRawCandleBackfillTimePtr(value *time.Time) string {
	if value == nil {
		return "none"
	}
	return formatHistoricalRawCandleBackfillTime(*value)
}
