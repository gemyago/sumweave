package backtest

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
)

// ErrValidation marks invalid backtest scaffold requests.
var ErrValidation = errors.New("backtest validation failed")

// ErrBacktestRunNotFound marks a missing persisted backtest run.
var ErrBacktestRunNotFound = errors.New("backtest run not found")

// RunQuery configures deterministic backtest run filtering.
type RunQuery struct {
	StrategyID string
	DatasetID  string
	Status     *domain.BacktestRunStatus
	TimeRange  *domain.TimeRange
}

// EvaluationReportQuery configures deterministic evaluation report filtering.
type EvaluationReportQuery struct {
	StrategyID string
	BacktestID string
	DatasetID  string
	Decision   *domain.EvaluationDecision
	TimeRange  *domain.TimeRange
}

// CompleteBacktestRunRequest configures a successful backtest completion.
type CompleteBacktestRunRequest struct {
	RunID   string
	Metrics *domain.VersionedMetrics
	EndedAt domain.BacktestRunTime
}

// FailBacktestRunRequest configures a failed backtest completion.
type FailBacktestRunRequest struct {
	RunID          string
	FailureReason  string
	FailureDetails string
	Metrics        *domain.VersionedMetrics
	EndedAt        domain.BacktestRunTime
}

// CreateEvaluationReportRequest configures report persistence and metrics assembly.
type CreateEvaluationReportRequest struct {
	EvaluationID         string
	StrategyID           string
	StrategyVersion      string
	StrategyArtifactHash string
	BacktestRunID        string
	DatasetID            string
	Decision             domain.EvaluationDecision
	FailureReasons       []string
	Notes                string
	CreatedAt            domain.EvaluationReportTime
	Fills                []domain.ExecutionFill
	GovernorDecisions    []domain.GovernorDecision
	PortfolioSnapshots   []domain.PortfolioSnapshot
}

type store interface {
	CreateDatasetReference(ctx context.Context, reference domain.DatasetReference) (domain.DatasetReference, error)
	CreateBacktestRun(ctx context.Context, run domain.BacktestRun) (domain.BacktestRun, error)
	GetBacktestRun(ctx context.Context, runID string) (*domain.BacktestRun, error)
	UpdateBacktestRun(ctx context.Context, run domain.BacktestRun) (domain.BacktestRun, error)
	QueryBacktestRuns(ctx context.Context, query RunQuery) ([]domain.BacktestRun, error)
	CreateEvaluationReport(ctx context.Context, report domain.EvaluationReport) (domain.EvaluationReport, error)
	QueryEvaluationReports(ctx context.Context, query EvaluationReportQuery) ([]domain.EvaluationReport, error)
}

// Service persists durable backtest and evaluation scaffold records.
type Service struct {
	store store
}

// NewService creates a backtest scaffold service with required persistence.
func NewService(store store) (*Service, error) {
	if store == nil {
		return nil, errors.New("backtest store is required")
	}

	return &Service{store: store}, nil
}

// CreateDatasetReference canonicalizes and persists a dataset reference.
func (s *Service) CreateDatasetReference(
	ctx context.Context,
	reference domain.DatasetReference,
) (domain.DatasetReference, error) {
	if err := ctx.Err(); err != nil {
		return domain.DatasetReference{}, err
	}

	canonical, err := domain.NewDatasetReference(domain.DatasetReferenceParams{
		DatasetID:        string(reference.DatasetID),
		EntityTypes:      reference.EntityTypes,
		Instruments:      reference.Instruments,
		Timeframes:       reference.Timeframes,
		TimeRange:        reference.TimeRange,
		SourceDataHashes: reference.SourceDataHashes,
		ReplayChecksum:   reference.ReplayChecksum,
		CreatedAt:        reference.CreatedAt.Time(),
		Metadata:         reference.Metadata,
	})
	if err != nil {
		return domain.DatasetReference{}, validationError(err.Error())
	}

	return s.store.CreateDatasetReference(ctx, canonical)
}

// CreateBacktestRun canonicalizes and persists a new pending backtest run.
func (s *Service) CreateBacktestRun(
	ctx context.Context,
	run domain.BacktestRun,
) (domain.BacktestRun, error) {
	if err := ctx.Err(); err != nil {
		return domain.BacktestRun{}, err
	}

	canonical, err := canonicalBacktestRun(run)
	if err != nil {
		return domain.BacktestRun{}, err
	}
	if canonical.Status != domain.BacktestRunStatusPending {
		return domain.BacktestRun{}, validationError("backtest run status must start at pending")
	}

	return s.store.CreateBacktestRun(ctx, canonical)
}

// StartBacktestRun transitions a persisted backtest run to running.
func (s *Service) StartBacktestRun(
	ctx context.Context,
	runID string,
	startedAt domain.BacktestRunTime,
) (domain.BacktestRun, error) {
	return s.transitionRun(ctx, runID, domain.BacktestRunStatusRunning, nil, "", "", startedAt.Time())
}

// CompleteBacktestRun transitions a persisted backtest run to completed.
func (s *Service) CompleteBacktestRun(
	ctx context.Context,
	request CompleteBacktestRunRequest,
) (domain.BacktestRun, error) {
	return s.transitionRun(
		ctx,
		request.RunID,
		domain.BacktestRunStatusCompleted,
		request.Metrics,
		"",
		"",
		request.EndedAt.Time(),
	)
}

// FailBacktestRun transitions a persisted backtest run to failed.
func (s *Service) FailBacktestRun(
	ctx context.Context,
	request FailBacktestRunRequest,
) (domain.BacktestRun, error) {
	return s.transitionRun(
		ctx,
		request.RunID,
		domain.BacktestRunStatusFailed,
		request.Metrics,
		request.FailureReason,
		request.FailureDetails,
		request.EndedAt.Time(),
	)
}

// QueryBacktestRuns returns deterministic persisted backtest runs.
func (s *Service) QueryBacktestRuns(
	ctx context.Context,
	query RunQuery,
) ([]domain.BacktestRun, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return s.store.QueryBacktestRuns(ctx, query)
}

// CreateEvaluationReport persists a compact evaluation report with assembled metrics.
func (s *Service) CreateEvaluationReport(
	ctx context.Context,
	request CreateEvaluationReportRequest,
) (domain.EvaluationReport, error) {
	if err := ctx.Err(); err != nil {
		return domain.EvaluationReport{}, err
	}

	metrics, err := buildEvaluationMetrics(request)
	if err != nil {
		return domain.EvaluationReport{}, err
	}

	report, err := domain.NewEvaluationReport(domain.EvaluationReportParams{
		EvaluationID:         request.EvaluationID,
		StrategyID:           request.StrategyID,
		StrategyVersion:      request.StrategyVersion,
		StrategyArtifactHash: request.StrategyArtifactHash,
		BacktestRunID:        request.BacktestRunID,
		DatasetID:            request.DatasetID,
		Decision:             request.Decision,
		Metrics:              metrics,
		FailureReasons:       request.FailureReasons,
		Notes:                request.Notes,
		CreatedAt:            request.CreatedAt.Time(),
	})
	if err != nil {
		return domain.EvaluationReport{}, validationError(err.Error())
	}

	return s.store.CreateEvaluationReport(ctx, report)
}

// QueryEvaluationReports returns deterministic persisted evaluation reports.
func (s *Service) QueryEvaluationReports(
	ctx context.Context,
	query EvaluationReportQuery,
) ([]domain.EvaluationReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return s.store.QueryEvaluationReports(ctx, query)
}

func (s *Service) transitionRun(
	ctx context.Context,
	runID string,
	status domain.BacktestRunStatus,
	metrics *domain.VersionedMetrics,
	failureReason string,
	failureDetails string,
	updatedAt time.Time,
) (domain.BacktestRun, error) {
	if err := ctx.Err(); err != nil {
		return domain.BacktestRun{}, err
	}
	if strings.TrimSpace(runID) == "" {
		return domain.BacktestRun{}, validationError("backtest run id is required")
	}
	current, err := s.store.GetBacktestRun(ctx, runID)
	if err != nil {
		if errors.Is(err, ErrBacktestRunNotFound) {
			return domain.BacktestRun{}, fmt.Errorf("%w: %s", ErrBacktestRunNotFound, runID)
		}

		return domain.BacktestRun{}, err
	}
	if err = validateRunTransition(current.Status, status); err != nil {
		return domain.BacktestRun{}, err
	}

	updated, err := domain.NewBacktestRun(domain.BacktestRunParams{
		RunID:                     current.RunID.String(),
		StrategyID:                current.StrategyID,
		StrategyVersion:           current.StrategyVersion,
		StrategyArtifactHash:      current.StrategyArtifactHash,
		DatasetID:                 current.DatasetID.String(),
		GovernorPolicyID:          current.GovernorPolicyID,
		GovernorPolicyVersion:     current.GovernorPolicyVersion,
		GovernorPolicyHash:        current.GovernorPolicyHash,
		Mode:                      current.Mode,
		TestedRange:               current.TestedRange,
		FeeModelID:                current.FeeModelID,
		FeeAssumptions:            current.FeeAssumptions,
		SlippageModelID:           current.SlippageModelID,
		SlippageAssumptions:       current.SlippageAssumptions,
		ExecutionSimulatorVersion: current.ExecutionSimulatorVersion,
		Status:                    status,
		Metrics:                   metrics,
		FailureReason:             failureReason,
		FailureDetails:            failureDetails,
		CreatedAt:                 current.CreatedAt.Time(),
		UpdatedAt:                 updatedAt,
	})
	if err != nil {
		return domain.BacktestRun{}, validationError(err.Error())
	}

	return s.store.UpdateBacktestRun(ctx, updated)
}

func canonicalBacktestRun(run domain.BacktestRun) (domain.BacktestRun, error) {
	canonical, err := domain.NewBacktestRun(domain.BacktestRunParams{
		RunID:                     run.RunID.String(),
		StrategyID:                run.StrategyID,
		StrategyVersion:           run.StrategyVersion,
		StrategyArtifactHash:      run.StrategyArtifactHash,
		DatasetID:                 run.DatasetID.String(),
		GovernorPolicyID:          run.GovernorPolicyID,
		GovernorPolicyVersion:     run.GovernorPolicyVersion,
		GovernorPolicyHash:        run.GovernorPolicyHash,
		Mode:                      run.Mode,
		TestedRange:               run.TestedRange,
		FeeModelID:                run.FeeModelID,
		FeeAssumptions:            run.FeeAssumptions,
		SlippageModelID:           run.SlippageModelID,
		SlippageAssumptions:       run.SlippageAssumptions,
		ExecutionSimulatorVersion: run.ExecutionSimulatorVersion,
		Status:                    run.Status,
		Metrics:                   run.Metrics,
		FailureReason:             run.FailureReason,
		FailureDetails:            run.FailureDetails,
		CreatedAt:                 run.CreatedAt.Time(),
		UpdatedAt:                 run.UpdatedAt.Time(),
	})
	if err != nil {
		return domain.BacktestRun{}, validationError(err.Error())
	}

	return canonical, nil
}

func buildEvaluationMetrics(
	request CreateEvaluationReportRequest,
) (*domain.VersionedMetrics, error) {
	params := domain.VersionedMetricsParams{SchemaVersion: "evaluation-report.v1"}

	if request.Fills != nil {
		tradeCount := len(request.Fills)
		params.TradeCount = &tradeCount
	}
	if request.GovernorDecisions != nil {
		blockedCount := 0
		rejectedCount := 0
		for idx, decision := range request.GovernorDecisions {
			canonical, err := domain.NewGovernorDecision(domain.GovernorDecisionParams{
				CandidateAction: decision.CandidateAction,
				Status:          decision.Status,
				Reason:          decision.Reason,
				DecisionTime:    decision.DecisionTime.Time(),
			})
			if err != nil {
				return nil, validationError(fmt.Sprintf("evaluation governor decision %d: %s", idx, err.Error()))
			}
			switch canonical.Status {
			case domain.GovernorDecisionStatusBlocked:
				blockedCount++
			case domain.GovernorDecisionStatusRejected:
				rejectedCount++
			case domain.GovernorDecisionStatusApproved:
			}
		}
		params.BlockedGovernorDecisionCount = &blockedCount
		params.RejectedGovernorDecisionCount = &rejectedCount
	}
	if request.PortfolioSnapshots != nil {
		maxDrawdown, err := calculateMaxDrawdown(request.PortfolioSnapshots)
		if err != nil {
			return nil, err
		}
		params.MaxDrawdown = &maxDrawdown
	}

	return domain.NewVersionedMetrics(params)
}

func calculateMaxDrawdown(snapshots []domain.PortfolioSnapshot) (float64, error) {
	canonical := make([]domain.PortfolioSnapshot, 0, len(snapshots))
	for idx, snapshot := range snapshots {
		next, err := domain.NewPortfolioSnapshot(domain.PortfolioSnapshotParams{
			SnapshotID:    snapshot.SnapshotID.String(),
			SourceFillID:  string(snapshot.SourceFillID),
			Mode:          snapshot.Mode,
			GrossExposure: snapshot.GrossExposure,
			NetExposure:   snapshot.NetExposure,
			RealizedPnL:   snapshot.RealizedPnL,
			UnrealizedPnL: snapshot.UnrealizedPnL,
			EventTime:     snapshot.EventTime.Time(),
			Metadata:      snapshot.Metadata,
		})
		if err != nil {
			return 0, validationError(fmt.Sprintf("evaluation portfolio snapshot %d: %s", idx, err.Error()))
		}
		canonical = append(canonical, next)
	}

	slices.SortStableFunc(canonical, func(left, right domain.PortfolioSnapshot) int {
		if comparison := left.EventTime.Time().Compare(right.EventTime.Time()); comparison != 0 {
			return comparison
		}

		return strings.Compare(left.SnapshotID.String(), right.SnapshotID.String())
	})

	peak := 0.0
	maxDrawdown := 0.0
	for _, snapshot := range canonical {
		totalPnL := snapshot.RealizedPnL
		if snapshot.UnrealizedPnL != nil {
			totalPnL += *snapshot.UnrealizedPnL
		}
		if totalPnL > peak {
			peak = totalPnL
		}
		drawdown := peak - totalPnL
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}
	if math.IsNaN(maxDrawdown) || math.IsInf(maxDrawdown, 0) {
		return 0, validationError("evaluation max drawdown must be finite")
	}

	return maxDrawdown, nil
}

func validateRunTransition(from domain.BacktestRunStatus, to domain.BacktestRunStatus) error {
	allowed := map[domain.BacktestRunStatus]map[domain.BacktestRunStatus]struct{}{
		domain.BacktestRunStatusPending: {
			domain.BacktestRunStatusRunning: {},
			domain.BacktestRunStatusFailed:  {},
		},
		domain.BacktestRunStatusRunning: {
			domain.BacktestRunStatusCompleted: {},
			domain.BacktestRunStatusFailed:    {},
		},
		domain.BacktestRunStatusCompleted: {},
		domain.BacktestRunStatusFailed:    {},
	}
	if _, ok := allowed[from][to]; !ok {
		return validationError(
			fmt.Sprintf("backtest run status transition %q -> %q is not allowed", from.String(), to.String()),
		)
	}

	return nil
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrValidation, message)
}
