package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/flows"
	"github.com/gemyago/signal-foundry/runtime/venueedge"
)

type historicalBackfillRunner interface {
	Run(
		ctx context.Context,
		request flows.HistoricalRawCandleBackfillRequest,
	) (flows.HistoricalRawCandleBackfillResult, error)
}

type WorkerDeps struct {
	Store    *Store
	Runner   historicalBackfillRunner
	Logger   *slog.Logger
	Clock    func() time.Time
	Config   WorkerConfig
	WorkerID string
}

type Worker struct {
	store    *Store
	runner   historicalBackfillRunner
	logger   *slog.Logger
	clock    func() time.Time
	config   WorkerConfig
	workerID string

	wake chan string
	wg   sync.WaitGroup
	stop context.CancelFunc
}

func NewWorker(deps WorkerDeps) (*Worker, error) {
	if deps.Store == nil {
		return nil, errors.New("jobs store is required")
	}
	if deps.Runner == nil {
		return nil, errors.New("historical backfill runner is required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Clock == nil {
		deps.Clock = time.Now
	}
	if deps.WorkerID == "" {
		deps.WorkerID = "jobs-worker"
	}
	return &Worker{
		store:    deps.Store,
		runner:   deps.Runner,
		logger:   deps.Logger,
		clock:    deps.Clock,
		config:   normalizeWorkerConfig(deps.Config),
		workerID: deps.WorkerID,
		wake:     make(chan string, 32),
	}, nil
}

func (w *Worker) SignalWake(jobID string) {
	select {
	case w.wake <- jobID:
	default:
	}
}

func (w *Worker) Start(ctx context.Context) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.store.RecoverStaleRunning(ctx, w.clock(), w.config.MaxAttempts); err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	w.stop = cancel
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(w.config.PollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case jobID := <-w.wake:
				_ = w.ProcessJob(runCtx, jobID)
			case <-ticker.C:
				_ = w.pollOnce(runCtx)
			}
		}
	}()
	return nil
}

func (w *Worker) Stop(_ context.Context) error {
	if w.stop != nil {
		w.stop()
	}
	w.wg.Wait()
	return nil
}

func (w *Worker) pollOnce(ctx context.Context) error {
	result, err := w.store.List(ctx, ListParams{
		Statuses: []JobStatus{JobStatusQueued},
		JobTypes: []JobType{JobTypeHistoricalRawCandleBackfill},
		Limit:    w.config.MaxConcurrentHistoricalBackfill,
	})
	if err != nil {
		return err
	}
	for _, job := range result.Items {
		processErr := w.ProcessJob(ctx, job.ID)
		if processErr != nil {
			return processErr
		}
	}
	return nil
}

func (w *Worker) ProcessJob(ctx context.Context, jobID string) error {
	job, err := w.store.Get(ctx, jobID)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			return nil
		}
		return err
	}
	if job.Status == JobStatusSucceeded || job.Status == JobStatusFailed {
		return nil
	}
	claimed, err := w.store.ClaimQueued(ctx, jobID, w.workerID, w.clock())
	if err != nil {
		if errors.Is(err, ErrJobNotQueued) {
			return nil
		}
		return err
	}
	request := flows.HistoricalRawCandleBackfillRequest{
		RunID:      claimed.Input.IngestionRunID,
		Venue:      domain.Venue(claimed.Input.Venue),
		Symbol:     domain.Symbol(claimed.Input.Symbol),
		AssetClass: domain.AssetClass(claimed.Input.AssetClass),
		Timeframe:  domain.Timeframe(claimed.Input.Timeframe),
		TimeRange:  claimed.Input.TimeRange,
		PageSize:   claimed.Input.PageSize,
	}
	runResult, runErr := w.runner.Run(ctx, request)
	if runErr != nil {
		return w.store.MarkFailed(ctx, claimed.ID, w.workerID, jobErrorFromExecution(runErr), w.clock())
	}
	result := HistoricalRawCandleBackfillResult{
		IngestionRunID:            runResult.RunID,
		PersistedCount:            runResult.Report.PersistedCount,
		ExpectedCount:             runResult.Report.ExpectedCount,
		MissingIntervalCount:      runResult.Report.MissingIntervalCount,
		DuplicateNaturalKeyCount:  runResult.Report.DuplicateNaturalKeyCount,
		FirstPersistedStart:       runResult.Report.FirstPersistedStart,
		LastPersistedEnd:          runResult.Report.LastPersistedEnd,
		RawPayloadCount:           runResult.Report.RawPayloadCount,
		MissingIntervalPreviewCap: runResult.Report.MissingIntervalPreviewLimit,
	}
	for _, interval := range runResult.Report.MissingIntervalPreview {
		result.MissingIntervalPreview = append(result.MissingIntervalPreview, jobTimeRange{
			Start: interval.Start.UTC(),
			End:   interval.End.UTC(),
		})
	}
	return w.store.MarkSucceeded(ctx, claimed.ID, w.workerID, result, w.clock())
}

func NewHistoricalBackfillRunner(
	lineageService *data.LineageService,
	readService *data.ReadService,
	ingestionFlow *venueedge.IngestionFlow,
) (*flows.HistoricalRawCandleBackfillRunner, error) {
	if lineageService == nil {
		return nil, errors.New("data lineage service is required")
	}
	if readService == nil {
		return nil, errors.New("data read service is required")
	}
	if ingestionFlow == nil {
		return nil, errors.New("venue ingestion flow is required")
	}
	recorder := &hyperliquidRawEvidenceRecorder{lineageService: lineageService}
	return flows.NewHistoricalRawCandleBackfillRunner(
		flows.HistoricalRawCandleBackfillRunnerDeps{
			RecordIngestionRun: lineageService.RecordIngestionRun,
			BuildVenue: func(
				_ context.Context,
				params flows.HistoricalRawCandleBackfillVenueBuildParams,
			) (venueedge.MarketDataVenue, error) {
				return venueedge.NewHyperliquidPerpsVenue(venueedge.HyperliquidPerpsVenueParams{
					BaseURL:                 defaultHistoricalBackfillBaseURL,
					HTTPClient:              http.DefaultClient,
					RawEvidenceRecorder:     recorder,
					RawEvidenceIngestionRun: params.RawEvidenceIngestionRun,
				})
			},
			IngestCandles:          ingestionFlow.IngestCandles,
			ReadPersistedCandles:   readService.QueryCandles,
			ReplayPersistedCandles: readService.ReplayCandles,
		},
	)
}

type hyperliquidRawEvidenceRecorder struct {
	lineageService *data.LineageService
}

func (r *hyperliquidRawEvidenceRecorder) RecordHyperliquidRawEvidence(
	ctx context.Context,
	capture venueedge.HyperliquidRawEvidenceCapture,
) (string, error) {
	payload, err := data.NewRawVenuePayload(data.RawVenuePayloadParams{
		ID:                 capture.ID,
		IngestionRunID:     capture.IngestionRunID,
		Source:             string(capture.Venue) + "-rest",
		Venue:              capture.Venue,
		Endpoint:           capture.Endpoint,
		RequestType:        capture.RequestType,
		RequestPayloadHash: capture.RequestPayloadHash,
		RequestMetadata:    capture.RequestMetadata,
		RequestAt:          capture.RequestAt,
		ResponseAt:         capture.ResponseAt,
		HTTPStatus:         capture.HTTPStatus,
		ResponseBody:       capture.ResponseBody,
		EntityHint:         capture.EntityHint,
		Timeframe:          capture.Timeframe,
		TimeRange:          capture.TimeRange,
		ReceivedAt:         capture.ReceivedAt,
	})
	if err != nil {
		return "", fmt.Errorf("build raw venue payload: %w", err)
	}
	persisted, err := r.lineageService.RecordRawVenuePayload(ctx, payload)
	if err != nil {
		return "", err
	}
	return persisted.ID, nil
}
