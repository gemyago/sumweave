package flows

import (
	"context"
	"errors"
	"hash/fnv"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/analytics"
	"github.com/gemyago/signal-foundry/runtime/audit"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/execution"
	"github.com/gemyago/signal-foundry/runtime/governor"
	"github.com/gemyago/signal-foundry/runtime/strategy"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type replayCall struct {
	instrument domain.Instrument
	timeframe  domain.Timeframe
	timeRange  domain.TimeRange
}

type fakeCandleReplayReader struct {
	callOrder *[]string
	calls     []replayCall
	result    []data.ReplayCandle
	err       error
}

func (f *fakeCandleReplayReader) ReplayCandles(
	_ context.Context,
	instrument domain.Instrument,
	timeframe domain.Timeframe,
	timeRange domain.TimeRange,
) ([]data.ReplayCandle, error) {
	if f.callOrder != nil {
		*f.callOrder = append(*f.callOrder, "replay")
	}

	f.calls = append(f.calls, replayCall{
		instrument: instrument,
		timeframe:  timeframe,
		timeRange:  timeRange,
	})
	if f.err != nil {
		return nil, f.err
	}

	return f.result, nil
}

type fakeAnalyticsCalculator struct {
	callOrder *[]string
	calls     []analytics.CalculateCandlesRequest
	results   []domain.AnalyticsSeries
	err       error
}

func (f *fakeAnalyticsCalculator) CalculateCandles(
	_ context.Context,
	request analytics.CalculateCandlesRequest,
) (domain.AnalyticsSeries, error) {
	if f.callOrder != nil {
		*f.callOrder = append(
			*f.callOrder,
			"analytics-window-"+strconv.Itoa(request.IndicatorParams.Window),
		)
	}

	f.calls = append(f.calls, request)
	if f.err != nil {
		return domain.AnalyticsSeries{}, f.err
	}

	if len(f.results) == 0 {
		return domain.AnalyticsSeries{}, nil
	}

	result := f.results[0]
	f.results = f.results[1:]

	return result, nil
}

type fakeStrategyEvaluator struct {
	callOrder *[]string
	calls     []strategy.EvaluateRequest
	result    strategy.EvaluateResult
	err       error
}

func (f *fakeStrategyEvaluator) Evaluate(
	_ context.Context,
	request strategy.EvaluateRequest,
) (strategy.EvaluateResult, error) {
	if f.callOrder != nil {
		*f.callOrder = append(*f.callOrder, "strategy")
	}

	f.calls = append(f.calls, request)
	if f.err != nil {
		return strategy.EvaluateResult{}, f.err
	}

	return f.result, nil
}

type fakeGovernorEvaluator struct {
	callOrder *[]string
	calls     []governor.EvaluateRequest
	result    governor.EvaluateResult
	err       error
}

func (f *fakeGovernorEvaluator) Evaluate(
	_ context.Context,
	request governor.EvaluateRequest,
) (governor.EvaluateResult, error) {
	if f.callOrder != nil {
		*f.callOrder = append(*f.callOrder, "governor")
	}

	f.calls = append(f.calls, request)
	if f.err != nil {
		return governor.EvaluateResult{}, f.err
	}

	return f.result, nil
}

type fakeAuditRecorder struct {
	callOrder         *[]string
	recordedTraces    []domain.DecisionTrace
	createdIntents    []domain.OrderIntent
	recordTraceCalls  int
	createIntentCalls int
	traceErr          error
	intentErr         error
}

func (f *fakeAuditRecorder) RecordTrace(
	_ context.Context,
	trace domain.DecisionTrace,
) (domain.DecisionTrace, error) {
	if f.callOrder != nil {
		*f.callOrder = append(*f.callOrder, "audit-record-trace")
	}
	if f.traceErr != nil {
		return domain.DecisionTrace{}, f.traceErr
	}

	f.recordTraceCalls++
	f.recordedTraces = append(f.recordedTraces, trace)

	return trace, nil
}

func (f *fakeAuditRecorder) CreateOrderIntent(
	_ context.Context,
	intent domain.OrderIntent,
) (domain.OrderIntent, error) {
	if f.callOrder != nil {
		*f.callOrder = append(*f.callOrder, "audit-create-intent")
	}
	if f.intentErr != nil {
		return domain.OrderIntent{}, f.intentErr
	}

	f.createIntentCalls++
	f.createdIntents = append(f.createdIntents, intent)

	return intent, nil
}

type fakeExecutionRecorder struct {
	callOrder             *[]string
	service               *execution.Service
	createCommandCalls    int
	recordOrderCalls      int
	recordFillCalls       int
	reconcileCalls        int
	createCommandRequests []execution.CreateCommandRequest
	recordOrderRequests   []execution.RecordOrderRequest
	recordFillRequests    []execution.RecordFillRequest
	reconcileRequests     []execution.ReconcileRequest
}

func (f *fakeExecutionRecorder) CreateCommand(
	ctx context.Context,
	request execution.CreateCommandRequest,
) (domain.ExecutionCommand, error) {
	if f.callOrder != nil {
		*f.callOrder = append(*f.callOrder, "execution-create-command")
	}

	f.createCommandCalls++
	f.createCommandRequests = append(f.createCommandRequests, request)

	service := f.service
	if service == nil {
		service = execution.NewService()
	}

	return service.CreateCommand(ctx, request)
}

func (f *fakeExecutionRecorder) RecordOrder(
	ctx context.Context,
	request execution.RecordOrderRequest,
) (domain.ExecutionOrder, error) {
	if f.callOrder != nil {
		*f.callOrder = append(*f.callOrder, "execution-record-order")
	}

	f.recordOrderCalls++
	f.recordOrderRequests = append(f.recordOrderRequests, request)

	service := f.service
	if service == nil {
		service = execution.NewService()
	}

	return service.RecordOrder(ctx, request)
}

func (f *fakeExecutionRecorder) RecordFill(
	ctx context.Context,
	request execution.RecordFillRequest,
) (domain.ExecutionFill, error) {
	if f.callOrder != nil {
		*f.callOrder = append(*f.callOrder, "execution-record-fill")
	}

	f.recordFillCalls++
	f.recordFillRequests = append(f.recordFillRequests, request)

	service := f.service
	if service == nil {
		service = execution.NewService()
	}

	return service.RecordFill(ctx, request)
}

func (f *fakeExecutionRecorder) Reconcile(
	ctx context.Context,
	request execution.ReconcileRequest,
) (domain.ExecutionReconciliation, error) {
	if f.callOrder != nil {
		*f.callOrder = append(*f.callOrder, "execution-reconcile")
	}

	f.reconcileCalls++
	f.reconcileRequests = append(f.reconcileRequests, request)

	service := f.service
	if service == nil {
		service = execution.NewService()
	}

	return service.Reconcile(ctx, request)
}

type replayOnlyInstrumentStore struct{}

var errReplayOnlyInstrumentLookupUnsupported = errors.New(
	"replay-only instrument lookup is unsupported",
)

func (s *replayOnlyInstrumentStore) LookupInstrument(
	_ context.Context,
	_ domain.Venue,
	_ domain.Symbol,
) (*domain.Instrument, error) {
	return nil, errReplayOnlyInstrumentLookupUnsupported
}

type replayOnlyTradeStore struct{}

func (s *replayOnlyTradeStore) QueryTrades(
	_ context.Context,
	_ domain.Instrument,
	_ domain.TimeRange,
) ([]domain.Trade, error) {
	return nil, nil
}

func (s *replayOnlyTradeStore) ReplayTrades(
	_ context.Context,
	_ domain.Instrument,
	_ domain.TimeRange,
) ([]data.ReplayTrade, error) {
	return nil, nil
}

type replayOnlyCandleStore struct {
	replayValue []data.ReplayCandle
	replayCalls []replayCall
}

func (s *replayOnlyCandleStore) QueryCandles(
	_ context.Context,
	_ domain.Instrument,
	_ domain.Timeframe,
	_ domain.TimeRange,
) ([]domain.Candle, error) {
	return nil, nil
}

func (s *replayOnlyCandleStore) ReplayCandles(
	_ context.Context,
	instrument domain.Instrument,
	timeframe domain.Timeframe,
	timeRange domain.TimeRange,
) ([]data.ReplayCandle, error) {
	s.replayCalls = append(s.replayCalls, replayCall{
		instrument: instrument,
		timeframe:  timeframe,
		timeRange:  timeRange,
	})

	return s.replayValue, nil
}

func TestPaperBacktestFlow(t *testing.T) {
	t.Parallel()

	newFake := func(t *testing.T) faker.Faker {
		t.Helper()

		hasher := fnv.New64a()
		_, _ = hasher.Write([]byte(t.Name()))

		return faker.NewWithSeedInt64(int64(hasher.Sum64()))
	}

	randomWord := func(t *testing.T, fake faker.Faker, prefix string) string {
		t.Helper()

		return prefix + "-" + strings.ToLower(fake.Lorem().Word()) + "-" + strconv.Itoa(fake.IntBetween(1000, 9999))
	}

	randomTime := func(t *testing.T, fake faker.Faker) time.Time {
		t.Helper()

		return time.Date(
			fake.IntBetween(2022, 2031),
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 28),
			fake.IntBetween(0, 23),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 999999999),
			time.FixedZone(randomWord(t, fake, "zone"), fake.IntBetween(-11, 12)*3600),
		)
	}

	makeInstrument := func(t *testing.T, fake faker.Faker) domain.Instrument {
		t.Helper()

		venue, err := domain.NewVenue("  " + randomWord(t, fake, "venue") + "  ")
		require.NoError(t, err)

		symbol, err := domain.NewSymbol("  " + strings.ToUpper(randomWord(t, fake, "symbol")) + "  ")
		require.NoError(t, err)

		assetClass, err := domain.NewAssetClass(domain.AssetClassCrypto.String())
		require.NoError(t, err)

		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      venue,
			Symbol:     symbol,
			AssetClass: assetClass,
			Active:     fake.Bool(),
		})
		require.NoError(t, err)

		return instrument
	}

	makeTimeRange := func(t *testing.T, fake faker.Faker) domain.TimeRange {
		t.Helper()

		start := randomTime(t, fake)
		timeRange, err := domain.NewTimeRange(
			start,
			start.Add(time.Duration(fake.IntBetween(1, 180))*time.Minute),
		)
		require.NoError(t, err)

		return timeRange
	}

	makeRequest := func(t *testing.T) PaperBacktestRequest {
		t.Helper()

		fake := newFake(t)

		return PaperBacktestRequest{
			RunID:                "  " + randomWord(t, fake, "run") + "  ",
			Mode:                 domain.DecisionModeBacktest,
			StrategyID:           "  " + randomWord(t, fake, "strategy-id") + "  ",
			StrategyVersion:      "  " + randomWord(t, fake, "strategy-version") + "  ",
			StrategyArtifactHash: "  " + randomWord(t, fake, "strategy-artifact") + "  ",
			Instrument:           makeInstrument(t, fake),
			Timeframe:            domain.Timeframe("  " + strings.ToUpper(domain.Timeframe1m.String()) + "  "),
			TimeRange:            makeTimeRange(t, fake),
			StrategyParameters: strategy.MovingAverageCrossoverParams{
				FastWindow: fake.IntBetween(1, 10),
				SlowWindow: fake.IntBetween(11, 25),
			},
			GovernorPolicy: governor.Policy{
				AllowedModes: []domain.DecisionMode{
					domain.DecisionModePaper,
					domain.DecisionModeBacktest,
				},
				AllowedActionKinds: []domain.CandidateActionKind{
					domain.CandidateActionKindLong,
					domain.CandidateActionKindShort,
				},
				MinimumQuality:       domain.DataQualityRaw,
				MaximumApprovedCount: fake.IntBetween(1, 5),
			},
			Quantity: float64(fake.IntBetween(1, 9)),
		}
	}

	makeStrategyIdentity := func(
		t *testing.T,
		instrument domain.Instrument,
		timeframe domain.Timeframe,
	) domain.StrategyIdentity {
		t.Helper()

		identity, err := domain.NewStrategyIdentity(domain.StrategyIdentityParams{
			Instrument: instrument,
			Timeframe:  timeframe,
			Kind:       domain.StrategyKindMovingAverageCrossover,
		})
		require.NoError(t, err)

		return identity
	}

	makeCandidateAction := func(
		t *testing.T,
		strategyIdentity domain.StrategyIdentity,
		kind domain.CandidateActionKind,
		decisionTime time.Time,
		inputRange domain.TimeRange,
		quality domain.DataQuality,
	) domain.CandidateAction {
		t.Helper()

		action, err := domain.NewCandidateAction(domain.CandidateActionParams{
			Strategy:     strategyIdentity,
			Kind:         kind,
			DecisionTime: decisionTime,
			InputRange:   inputRange,
			Quality:      quality,
		})
		require.NoError(t, err)

		return action
	}

	makeGovernorDecision := func(
		t *testing.T,
		action domain.CandidateAction,
		status domain.GovernorDecisionStatus,
		reason domain.GovernorDecisionReason,
	) domain.GovernorDecision {
		t.Helper()

		decision, err := domain.NewGovernorDecision(domain.GovernorDecisionParams{
			CandidateAction: action,
			Status:          status,
			Reason:          reason,
			DecisionTime:    action.DecisionTime.Time(),
		})
		require.NoError(t, err)

		return decision
	}

	makeReplayCandles := func(
		t *testing.T,
		fake faker.Faker,
		instrument domain.Instrument,
		timeframe domain.Timeframe,
		start time.Time,
		width time.Duration,
		closes []float64,
	) []data.ReplayCandle {
		t.Helper()

		replayed := make([]data.ReplayCandle, len(closes))
		identityBase := uint64(fake.IntBetween(100, 900))

		for idx, closeValue := range closes {
			provenance, err := domain.NewSourceProvenance(
				randomWord(t, fake, "replay-source"),
				randomWord(t, fake, "replay-record"),
			)
			require.NoError(t, err)

			candle, err := domain.NewCandle(domain.CandleParams{
				Instrument: instrument,
				Timeframe:  timeframe,
				TimeRange: domain.TimeRange{
					Start: start.Add(time.Duration(idx) * width),
					End:   start.Add(time.Duration(idx+1) * width),
				},
				Open:       closeValue - 0.5,
				High:       closeValue + 0.5,
				Low:        closeValue - 1,
				Close:      closeValue,
				Volume:     float64(fake.IntBetween(10, 5000)),
				Quality:    domain.DataQualityValidated,
				Provenance: provenance,
			})
			require.NoError(t, err)

			replayed[idx] = data.ReplayCandle{
				Identity: identityBase + uint64(idx) + 1,
				Candle:   candle,
			}
		}

		return replayed
	}

	type mockDeps struct {
		replayReader      *fakeCandleReplayReader
		analyticsCalc     *fakeAnalyticsCalculator
		strategyEvaluator *fakeStrategyEvaluator
		auditRecorder     *fakeAuditRecorder
		governorEvaluator *fakeGovernorEvaluator
		executionRecorder *fakeExecutionRecorder
		paperBacktestDeps PaperBacktestFlowDeps
	}

	makeMockDeps := func(callOrder *[]string) mockDeps {
		replayReader := &fakeCandleReplayReader{callOrder: callOrder}
		analyticsCalc := &fakeAnalyticsCalculator{callOrder: callOrder}
		strategyEvaluator := &fakeStrategyEvaluator{callOrder: callOrder}
		auditRecorder := &fakeAuditRecorder{callOrder: callOrder}
		governorEvaluator := &fakeGovernorEvaluator{callOrder: callOrder}
		executionRecorder := &fakeExecutionRecorder{callOrder: callOrder}

		return mockDeps{
			replayReader:      replayReader,
			analyticsCalc:     analyticsCalc,
			strategyEvaluator: strategyEvaluator,
			auditRecorder:     auditRecorder,
			governorEvaluator: governorEvaluator,
			executionRecorder: executionRecorder,
			paperBacktestDeps: PaperBacktestFlowDeps{
				CandleReplayReader:  replayReader,
				AnalyticsCalculator: analyticsCalc,
				StrategyEvaluator:   strategyEvaluator,
				AuditRecorder:       auditRecorder,
				GovernorEvaluator:   governorEvaluator,
				ExecutionRecorder:   executionRecorder,
			},
		}
	}

	t.Run("NewPaperBacktestFlow", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects missing dependencies", func(t *testing.T) {
			t.Parallel()

			testCases := []struct {
				name    string
				mutate  func(*PaperBacktestFlowDeps)
				message string
			}{
				{
					name: "candle replay reader",
					mutate: func(deps *PaperBacktestFlowDeps) {
						deps.CandleReplayReader = nil
					},
					message: "candle replay reader is required",
				},
				{
					name: "analytics calculator",
					mutate: func(deps *PaperBacktestFlowDeps) {
						deps.AnalyticsCalculator = nil
					},
					message: "analytics calculator is required",
				},
				{
					name: "strategy evaluator",
					mutate: func(deps *PaperBacktestFlowDeps) {
						deps.StrategyEvaluator = nil
					},
					message: "strategy evaluator is required",
				},
				{
					name: "audit recorder",
					mutate: func(deps *PaperBacktestFlowDeps) {
						deps.AuditRecorder = nil
					},
					message: "audit recorder is required",
				},
				{
					name: "governor evaluator",
					mutate: func(deps *PaperBacktestFlowDeps) {
						deps.GovernorEvaluator = nil
					},
					message: "governor evaluator is required",
				},
				{
					name: "execution recorder",
					mutate: func(deps *PaperBacktestFlowDeps) {
						deps.ExecutionRecorder = nil
					},
					message: "execution recorder is required",
				},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()

					deps := makeMockDeps(nil).paperBacktestDeps
					testCase.mutate(&deps)

					flow, err := NewPaperBacktestFlow(deps)

					require.Nil(t, flow)
					require.EqualError(t, err, testCase.message)
				})
			}
		})
	})

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		makeFlow := func(t *testing.T, deps PaperBacktestFlowDeps) *PaperBacktestFlow {
			t.Helper()

			flow, err := NewPaperBacktestFlow(deps)
			require.NoError(t, err)

			return flow
		}

		assertValidationError := func(t *testing.T, request PaperBacktestRequest, expected string) {
			t.Helper()

			deps := makeMockDeps(nil)
			flow := makeFlow(t, deps.paperBacktestDeps)

			result, err := flow.Run(t.Context(), request)

			require.Equal(t, PaperBacktestResult{}, result)
			require.ErrorIs(t, err, ErrValidation)
			require.ErrorContains(t, err, expected)
		}

		t.Run("rejects missing run identity", func(t *testing.T) {
			t.Parallel()

			request := makeRequest(t)
			request.RunID = " \t "

			assertValidationError(t, request, "run id is required")
		})

		t.Run("rejects missing strategy id", func(t *testing.T) {
			t.Parallel()

			request := makeRequest(t)
			request.StrategyID = " \t "

			assertValidationError(t, request, "strategy id is required")
		})

		t.Run("rejects invalid instrument", func(t *testing.T) {
			t.Parallel()

			request := makeRequest(t)
			request.Instrument.Venue = ""

			assertValidationError(t, request, "instrument venue is required")
		})

		t.Run("rejects invalid timeframe", func(t *testing.T) {
			t.Parallel()

			request := makeRequest(t)
			request.Timeframe = domain.Timeframe("  ")

			assertValidationError(t, request, "strategy timeframe is required")
		})

		t.Run("rejects invalid time range", func(t *testing.T) {
			t.Parallel()

			request := makeRequest(t)
			request.TimeRange.End = request.TimeRange.Start

			assertValidationError(t, request, "time range end must be after start")
		})

		t.Run("rejects invalid moving average parameters", func(t *testing.T) {
			t.Parallel()

			testCases := []struct {
				name     string
				params   strategy.MovingAverageCrossoverParams
				expected string
			}{
				{
					name: "non positive fast window",
					params: strategy.MovingAverageCrossoverParams{
						FastWindow: 0,
						SlowWindow: 5,
					},
					expected: "moving average crossover fast window must be positive",
				},
				{
					name: "non positive slow window",
					params: strategy.MovingAverageCrossoverParams{
						FastWindow: 1,
						SlowWindow: 0,
					},
					expected: "moving average crossover slow window must be positive",
				},
				{
					name: "fast window not less than slow window",
					params: strategy.MovingAverageCrossoverParams{
						FastWindow: 5,
						SlowWindow: 5,
					},
					expected: "moving average crossover fast window must be less than slow window",
				},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()

					request := makeRequest(t)
					request.StrategyParameters = testCase.params

					assertValidationError(t, request, testCase.expected)
				})
			}
		})

		t.Run("rejects invalid governor policy", func(t *testing.T) {
			t.Parallel()

			testCases := []struct {
				name     string
				mutate   func(*governor.Policy)
				expected string
			}{
				{
					name: "missing allowed action kinds",
					mutate: func(policy *governor.Policy) {
						policy.AllowedActionKinds = nil
					},
					expected: "allowed action kinds are required",
				},
				{
					name: "unsupported minimum quality",
					mutate: func(policy *governor.Policy) {
						policy.MinimumQuality = domain.DataQualitySuspect
					},
					expected: "unsupported minimum quality \"suspect\"",
				},
				{
					name: "negative approval limit",
					mutate: func(policy *governor.Policy) {
						policy.MaximumApprovedCount = -1
					},
					expected: "maximum approved action count must be zero or greater",
				},
				{
					name: "negative maximum order notional",
					mutate: func(policy *governor.Policy) {
						policy.MaximumOrderNotional = -1
					},
					expected: "maximum order notional must be finite and zero or greater",
				},
				{
					name: "blank allowed strategy id",
					mutate: func(policy *governor.Policy) {
						policy.AllowedStrategyIDs = []string{"  "}
					},
					expected: "allowed strategy ids must not be empty",
				},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()

					request := makeRequest(t)
					testCase.mutate(&request.GovernorPolicy)

					assertValidationError(t, request, testCase.expected)
				})
			}
		})

		t.Run("preserves expanded governor policy checks end to end", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			request := makeRequest(t)
			strategyIdentity := makeStrategyIdentity(t, request.Instrument, request.Timeframe)
			action := makeCandidateAction(
				t,
				strategyIdentity,
				domain.CandidateActionKindLong,
				request.TimeRange.Start.Add(5*time.Minute),
				domain.TimeRange{
					Start: request.TimeRange.Start,
					End:   request.TimeRange.Start.Add(5 * time.Minute),
				},
				domain.DataQualityValidated,
			)

			makeRealFlow := func(t *testing.T) *PaperBacktestFlow {
				t.Helper()

				replayReader := &fakeCandleReplayReader{result: makeReplayCandles(
					t,
					fake,
					request.Instrument,
					domain.Timeframe1m,
					request.TimeRange.Start,
					time.Minute,
					[]float64{10, 11, 12, 13, 14, 15, 16},
				)}
				analyticsCalc := &fakeAnalyticsCalculator{}
				strategyEvaluator := &fakeStrategyEvaluator{result: strategy.EvaluateResult{
					Strategy:   strategyIdentity,
					TimeRange:  request.TimeRange,
					Parameters: request.StrategyParameters,
					Actions:    []domain.CandidateAction{action},
				}}
				auditRecorder := &fakeAuditRecorder{}
				executionRecorder := &fakeExecutionRecorder{}

				return makeFlow(t, PaperBacktestFlowDeps{
					CandleReplayReader:  replayReader,
					AnalyticsCalculator: analyticsCalc,
					StrategyEvaluator:   strategyEvaluator,
					AuditRecorder:       auditRecorder,
					GovernorEvaluator:   governor.NewService(),
					ExecutionRecorder:   executionRecorder,
				})
			}

			testCases := []struct {
				name           string
				policy         governor.Policy
				expectedStatus domain.GovernorDecisionStatus
				expectedReason domain.GovernorDecisionReason
			}{
				{
					name: "mode allowlist",
					policy: governor.Policy{
						AllowedModes:       []domain.DecisionMode{domain.DecisionModePaper},
						AllowedVenues:      []domain.Venue{request.Instrument.Venue},
						AllowedInstruments: []domain.Instrument{request.Instrument},
						AllowedStrategyIDs: []string{request.StrategyID},
						AllowedActionKinds: []domain.CandidateActionKind{domain.CandidateActionKindLong},
						MinimumQuality:     domain.DataQualityValidated,
					},
					expectedStatus: domain.GovernorDecisionStatusRejected,
					expectedReason: domain.GovernorDecisionReasonModeNotAllowed,
				},
				{
					name: "venue allowlist",
					policy: governor.Policy{
						AllowedModes:       []domain.DecisionMode{domain.DecisionModeBacktest},
						AllowedVenues:      []domain.Venue{domain.Venue(randomWord(t, fake, "other-venue"))},
						AllowedInstruments: []domain.Instrument{request.Instrument},
						AllowedStrategyIDs: []string{request.StrategyID},
						AllowedActionKinds: []domain.CandidateActionKind{domain.CandidateActionKindLong},
						MinimumQuality:     domain.DataQualityValidated,
					},
					expectedStatus: domain.GovernorDecisionStatusRejected,
					expectedReason: domain.GovernorDecisionReasonVenueNotAllowed,
				},
				{
					name: "instrument allowlist",
					policy: governor.Policy{
						AllowedModes:       []domain.DecisionMode{domain.DecisionModeBacktest},
						AllowedVenues:      []domain.Venue{request.Instrument.Venue},
						AllowedInstruments: []domain.Instrument{makeInstrument(t, fake)},
						AllowedStrategyIDs: []string{request.StrategyID},
						AllowedActionKinds: []domain.CandidateActionKind{domain.CandidateActionKindLong},
						MinimumQuality:     domain.DataQualityValidated,
					},
					expectedStatus: domain.GovernorDecisionStatusRejected,
					expectedReason: domain.GovernorDecisionReasonInstrumentNotAllowed,
				},
				{
					name: "strategy allowlist",
					policy: governor.Policy{
						AllowedModes:       []domain.DecisionMode{domain.DecisionModeBacktest},
						AllowedVenues:      []domain.Venue{request.Instrument.Venue},
						AllowedInstruments: []domain.Instrument{request.Instrument},
						AllowedStrategyIDs: []string{randomWord(t, fake, "other-strategy")},
						AllowedActionKinds: []domain.CandidateActionKind{domain.CandidateActionKindLong},
						MinimumQuality:     domain.DataQualityValidated,
					},
					expectedStatus: domain.GovernorDecisionStatusRejected,
					expectedReason: domain.GovernorDecisionReasonStrategyNotAllowed,
				},
				{
					name: "kill switch",
					policy: governor.Policy{
						AllowedModes:       []domain.DecisionMode{domain.DecisionModeBacktest},
						AllowedVenues:      []domain.Venue{request.Instrument.Venue},
						AllowedInstruments: []domain.Instrument{request.Instrument},
						AllowedStrategyIDs: []string{request.StrategyID},
						AllowedActionKinds: []domain.CandidateActionKind{domain.CandidateActionKindLong},
						MinimumQuality:     domain.DataQualityValidated,
						BlockNewRisk:       true,
					},
					expectedStatus: domain.GovernorDecisionStatusBlocked,
					expectedReason: domain.GovernorDecisionReasonKillSwitchActive,
				},
				{
					name: "order notional limit",
					policy: governor.Policy{
						AllowedModes:         []domain.DecisionMode{domain.DecisionModeBacktest},
						AllowedVenues:        []domain.Venue{request.Instrument.Venue},
						AllowedInstruments:   []domain.Instrument{request.Instrument},
						AllowedStrategyIDs:   []string{request.StrategyID},
						AllowedActionKinds:   []domain.CandidateActionKind{domain.CandidateActionKindLong},
						MinimumQuality:       domain.DataQualityValidated,
						MaximumOrderNotional: 1,
					},
					expectedStatus: domain.GovernorDecisionStatusRejected,
					expectedReason: domain.GovernorDecisionReasonOrderNotionalExceedsLimit,
				},
				{
					name: "strategy exposure limit",
					policy: governor.Policy{
						AllowedModes:                    []domain.DecisionMode{domain.DecisionModeBacktest},
						AllowedVenues:                   []domain.Venue{request.Instrument.Venue},
						AllowedInstruments:              []domain.Instrument{request.Instrument},
						AllowedStrategyIDs:              []string{request.StrategyID},
						AllowedActionKinds:              []domain.CandidateActionKind{domain.CandidateActionKindLong},
						MinimumQuality:                  domain.DataQualityValidated,
						MaximumStrategyExposureNotional: 1,
					},
					expectedStatus: domain.GovernorDecisionStatusRejected,
					expectedReason: domain.GovernorDecisionReasonStrategyExposureExceedsLimit,
				},
				{
					name: "instrument exposure limit",
					policy: governor.Policy{
						AllowedModes:                      []domain.DecisionMode{domain.DecisionModeBacktest},
						AllowedVenues:                     []domain.Venue{request.Instrument.Venue},
						AllowedInstruments:                []domain.Instrument{request.Instrument},
						AllowedStrategyIDs:                []string{request.StrategyID},
						AllowedActionKinds:                []domain.CandidateActionKind{domain.CandidateActionKindLong},
						MinimumQuality:                    domain.DataQualityValidated,
						MaximumInstrumentExposureNotional: 1,
					},
					expectedStatus: domain.GovernorDecisionStatusRejected,
					expectedReason: domain.GovernorDecisionReasonInstrumentExposureExceedsLimit,
				},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					flow := makeRealFlow(t)
					caseRequest := request
					caseRequest.GovernorPolicy = testCase.policy

					result, err := flow.Run(t.Context(), caseRequest)

					require.NoError(t, err)
					require.Len(t, result.GovernorEvaluation.Decisions, 1)
					require.Equal(t, testCase.expectedStatus, result.GovernorEvaluation.Decisions[0].Status)
					require.Equal(t, testCase.expectedReason, result.GovernorEvaluation.Decisions[0].Reason)
				})
			}
		})

		t.Run("rejects non positive quantity", func(t *testing.T) {
			t.Parallel()

			testCases := []struct {
				name     string
				quantity float64
			}{
				{name: "zero", quantity: 0},
				{name: "negative", quantity: -1},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()

					request := makeRequest(t)
					request.Quantity = testCase.quantity

					assertValidationError(t, request, "quantity must be positive")
				})
			}
		})

		orchestrationName :=
			"evaluates strategy from the requested replay range and passes candidate actions unchanged into governor"

		t.Run(orchestrationName, func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			callOrder := make([]string, 0, 5)
			deps := makeMockDeps(&callOrder)

			request := makeRequest(t)
			request.TimeRange = makeTimeRange(t, fake)

			strategyIdentity := makeStrategyIdentity(t, request.Instrument, domain.Timeframe1m)
			firstAction := makeCandidateAction(
				t,
				strategyIdentity,
				domain.CandidateActionKindLong,
				request.TimeRange.Start.Add(5*time.Minute),
				domain.TimeRange{
					Start: request.TimeRange.Start,
					End:   request.TimeRange.Start.Add(5 * time.Minute),
				},
				domain.DataQualityValidated,
			)
			secondAction := makeCandidateAction(
				t,
				strategyIdentity,
				domain.CandidateActionKindShort,
				request.TimeRange.Start.Add(11*time.Minute),
				domain.TimeRange{
					Start: request.TimeRange.Start.Add(6 * time.Minute),
					End:   request.TimeRange.Start.Add(11 * time.Minute),
				},
				domain.DataQualityRaw,
			)
			deps.replayReader.result = makeReplayCandles(
				t,
				fake,
				request.Instrument,
				domain.Timeframe1m,
				request.TimeRange.Start,
				time.Minute,
				[]float64{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21},
			)

			deps.strategyEvaluator.result = strategy.EvaluateResult{
				Strategy:   strategyIdentity,
				TimeRange:  request.TimeRange,
				Parameters: request.StrategyParameters,
				Actions:    []domain.CandidateAction{firstAction, secondAction},
			}
			deps.governorEvaluator.result = governor.EvaluateResult{
				Decisions: []domain.GovernorDecision{
					makeGovernorDecision(
						t,
						secondAction,
						domain.GovernorDecisionStatusBlocked,
						domain.GovernorDecisionReasonApprovalLimitReached,
					),
					makeGovernorDecision(
						t,
						firstAction,
						domain.GovernorDecisionStatusApproved,
						domain.GovernorDecisionReasonEligible,
					),
				},
			}

			flow := makeFlow(t, deps.paperBacktestDeps)

			result, err := flow.Run(t.Context(), request)

			require.NoError(t, err)
			require.Equal(t, []string{
				"replay",
				"analytics-window-" + strconv.Itoa(request.StrategyParameters.FastWindow),
				"analytics-window-" + strconv.Itoa(request.StrategyParameters.SlowWindow),
				"strategy",
				"audit-record-trace",
				"audit-create-intent",
				"audit-record-trace",
				"audit-create-intent",
				"governor",
				"execution-create-command",
				"execution-record-order",
				"execution-record-fill",
				"execution-reconcile",
			}, callOrder)
			require.Len(t, deps.replayReader.calls, 1)
			require.Equal(t, request.TimeRange.Start.UTC(), deps.replayReader.calls[0].timeRange.Start)
			require.Equal(t, request.TimeRange.End.UTC(), deps.replayReader.calls[0].timeRange.End)
			require.Len(t, deps.analyticsCalc.calls, 2)
			require.Equal(t, request.TimeRange.Start.UTC(), deps.analyticsCalc.calls[0].TimeRange.Start)
			require.Equal(t, request.TimeRange.End.UTC(), deps.analyticsCalc.calls[0].TimeRange.End)
			require.Equal(t, request.TimeRange.Start.UTC(), deps.analyticsCalc.calls[1].TimeRange.Start)
			require.Equal(t, request.TimeRange.End.UTC(), deps.analyticsCalc.calls[1].TimeRange.End)
			require.Len(t, deps.strategyEvaluator.calls, 1)
			require.Equal(t, request.TimeRange.Start.UTC(), deps.strategyEvaluator.calls[0].TimeRange.Start)
			require.Equal(t, request.TimeRange.End.UTC(), deps.strategyEvaluator.calls[0].TimeRange.End)
			require.Len(t, deps.governorEvaluator.calls, 1)
			require.Len(t, deps.governorEvaluator.calls[0].IntentInputs, 2)
			require.Equal(
				t,
				result.IntentContexts[0].CandidateAction,
				deps.governorEvaluator.calls[0].IntentInputs[0].CandidateAction,
			)
			require.Equal(t, result.IntentContexts[0].Intent, deps.governorEvaluator.calls[0].IntentInputs[0].Intent)
			require.NotEmpty(t, deps.governorEvaluator.calls[0].IntentInputs[0].GovernorPolicyID)
			require.NotEmpty(t, deps.governorEvaluator.calls[0].IntentInputs[0].GovernorPolicyVersion)
			require.NotEmpty(t, deps.governorEvaluator.calls[0].IntentInputs[0].GovernorPolicyHash)
			require.Equal(t, deps.strategyEvaluator.result, result.StrategyEvaluation)
			require.Len(t, result.IntentContexts, 2)
			require.Equal(t, deps.auditRecorder.recordedTraces[0].TraceID, result.IntentContexts[0].Trace.TraceID)
			require.Equal(t, deps.auditRecorder.createdIntents[0].IntentID, result.IntentContexts[0].Intent.IntentID)
			require.Equal(t, result.IntentContexts[0].Trace.TraceID, result.IntentContexts[0].Intent.TraceID)
			require.Equal(t, deps.strategyEvaluator.result.Actions[0], result.IntentContexts[0].CandidateAction)
			require.Equal(t, domain.OrderIntentStatusCreated, result.IntentContexts[0].Intent.Status)
			require.Equal(t, request.Mode, result.IntentContexts[0].Trace.Mode)
			require.Equal(t, request.Mode, result.IntentContexts[0].Intent.Mode)
			require.NotNil(t, result.IntentContexts[0].Intent.RequestedLimitPrice)
			require.Equal(t, deps.governorEvaluator.result, result.GovernorEvaluation)
			require.Equal(t, deps.governorEvaluator.result.Decisions, result.GovernorEvaluation.Decisions)
			require.Equal(t, strings.TrimSpace(request.RunID), result.RunID)
			require.Len(t, result.PaperExecutions, 1)
			require.Equal(t, deps.governorEvaluator.result.Decisions[1], result.PaperExecutions[0].ApprovedDecision)
			require.Equal(t, 1, deps.executionRecorder.createCommandCalls)
			require.Equal(t, 1, deps.executionRecorder.recordOrderCalls)
			require.Equal(t, 1, deps.executionRecorder.recordFillCalls)
			require.Equal(t, 1, deps.executionRecorder.reconcileCalls)
		})

		t.Run("approved decisions create deterministic local paper execution records", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			deps := makeMockDeps(nil)
			request := makeRequest(t)
			request.TimeRange = makeTimeRange(t, fake)

			strategyIdentity := makeStrategyIdentity(t, request.Instrument, domain.Timeframe1m)
			firstApproved := makeCandidateAction(
				t,
				strategyIdentity,
				domain.CandidateActionKindLong,
				request.TimeRange.Start.Add(2*time.Minute),
				domain.TimeRange{
					Start: request.TimeRange.Start,
					End:   request.TimeRange.Start.Add(2 * time.Minute),
				},
				domain.DataQualityValidated,
			)
			secondApproved := makeCandidateAction(
				t,
				strategyIdentity,
				domain.CandidateActionKindShort,
				request.TimeRange.Start.Add(4*time.Minute),
				domain.TimeRange{
					Start: request.TimeRange.Start.Add(2 * time.Minute),
					End:   request.TimeRange.Start.Add(4 * time.Minute),
				},
				domain.DataQualityValidated,
			)
			deps.replayReader.result = makeReplayCandles(
				t,
				fake,
				request.Instrument,
				domain.Timeframe1m,
				request.TimeRange.Start,
				time.Minute,
				[]float64{100, 101, 102, 103, 104},
			)
			deps.strategyEvaluator.result = strategy.EvaluateResult{
				Strategy:   strategyIdentity,
				TimeRange:  request.TimeRange,
				Parameters: request.StrategyParameters,
				Actions:    []domain.CandidateAction{firstApproved, secondApproved},
			}
			deps.governorEvaluator.result = governor.EvaluateResult{
				Decisions: []domain.GovernorDecision{
					makeGovernorDecision(
						t,
						firstApproved,
						domain.GovernorDecisionStatusApproved,
						domain.GovernorDecisionReasonEligible,
					),
					makeGovernorDecision(
						t,
						secondApproved,
						domain.GovernorDecisionStatusApproved,
						domain.GovernorDecisionReasonEligible,
					),
				},
			}

			flow := makeFlow(t, deps.paperBacktestDeps)

			firstResult, err := flow.Run(t.Context(), request)
			require.NoError(t, err)

			secondResult, err := flow.Run(t.Context(), request)
			require.NoError(t, err)

			require.Equal(t, firstResult, secondResult)
			require.Len(t, firstResult.PaperExecutions, 2)
			for idx, paperExecution := range firstResult.PaperExecutions {
				require.NotEmpty(t, paperExecution.Command.CommandID)
				require.NotEmpty(t, paperExecution.Order.OrderID)
				require.NotEmpty(t, paperExecution.Order.ClientOrderID)
				require.NotEmpty(t, paperExecution.Fill.FillID)
				require.NotEmpty(t, paperExecution.ReconciliationID)
				require.Equal(t, paperExecution.Command, paperExecution.Order.Command)
				require.Equal(t, paperExecution.Order, paperExecution.Fill.Order)
				require.Equal(t, paperExecution.Order, paperExecution.Reconciliation.Order)
				require.Equal(t, []domain.ExecutionFill{paperExecution.Fill}, paperExecution.Reconciliation.Fills)
				require.Equal(t, domain.ExecutionOrderStatusFilled, paperExecution.Reconciliation.Status)
				require.InDelta(t, request.Quantity, paperExecution.Reconciliation.FilledQuantity, 0)
				require.Equal(t, deps.governorEvaluator.result.Decisions[idx], paperExecution.ApprovedDecision)
			}
			require.Equal(t, 4, deps.executionRecorder.createCommandCalls)
			require.Equal(t, 4, deps.executionRecorder.recordOrderCalls)
			require.Equal(t, 4, deps.executionRecorder.recordFillCalls)
			require.Equal(t, 4, deps.executionRecorder.reconcileCalls)
		})

		t.Run("rejected or blocked decisions create no execution records", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			deps := makeMockDeps(nil)
			request := makeRequest(t)
			request.TimeRange = makeTimeRange(t, fake)

			strategyIdentity := makeStrategyIdentity(t, request.Instrument, domain.Timeframe1m)
			blockedAction := makeCandidateAction(
				t,
				strategyIdentity,
				domain.CandidateActionKindLong,
				request.TimeRange.Start.Add(2*time.Minute),
				domain.TimeRange{
					Start: request.TimeRange.Start,
					End:   request.TimeRange.Start.Add(2 * time.Minute),
				},
				domain.DataQualityValidated,
			)
			rejectedAction := makeCandidateAction(
				t,
				strategyIdentity,
				domain.CandidateActionKindShort,
				request.TimeRange.Start.Add(4*time.Minute),
				domain.TimeRange{
					Start: request.TimeRange.Start.Add(2 * time.Minute),
					End:   request.TimeRange.Start.Add(4 * time.Minute),
				},
				domain.DataQualityValidated,
			)
			deps.replayReader.result = makeReplayCandles(
				t,
				fake,
				request.Instrument,
				domain.Timeframe1m,
				request.TimeRange.Start,
				time.Minute,
				[]float64{100, 101, 102, 103, 104},
			)
			deps.strategyEvaluator.result = strategy.EvaluateResult{
				Strategy:   strategyIdentity,
				TimeRange:  request.TimeRange,
				Parameters: request.StrategyParameters,
				Actions:    []domain.CandidateAction{blockedAction, rejectedAction},
			}
			deps.governorEvaluator.result = governor.EvaluateResult{
				Decisions: []domain.GovernorDecision{
					makeGovernorDecision(
						t,
						blockedAction,
						domain.GovernorDecisionStatusBlocked,
						domain.GovernorDecisionReasonApprovalLimitReached,
					),
					makeGovernorDecision(
						t,
						rejectedAction,
						domain.GovernorDecisionStatusRejected,
						domain.GovernorDecisionReasonDisallowedActionKind,
					),
				},
			}

			flow := makeFlow(t, deps.paperBacktestDeps)

			result, err := flow.Run(t.Context(), request)
			require.NoError(t, err)
			require.Empty(t, result.PaperExecutions)
			require.Zero(t, deps.executionRecorder.createCommandCalls)
			require.Zero(t, deps.executionRecorder.recordOrderCalls)
			require.Zero(t, deps.executionRecorder.recordFillCalls)
			require.Zero(t, deps.executionRecorder.reconcileCalls)
		})

		t.Run("missing fill-price candle fails the run", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			deps := makeMockDeps(nil)
			request := makeRequest(t)
			request.TimeRange = makeTimeRange(t, fake)

			strategyIdentity := makeStrategyIdentity(t, request.Instrument, domain.Timeframe1m)
			approvedAction := makeCandidateAction(
				t,
				strategyIdentity,
				domain.CandidateActionKindLong,
				request.TimeRange.Start.Add(3*time.Minute),
				domain.TimeRange{
					Start: request.TimeRange.Start,
					End:   request.TimeRange.Start.Add(3 * time.Minute),
				},
				domain.DataQualityValidated,
			)
			deps.replayReader.result = makeReplayCandles(
				t,
				fake,
				request.Instrument,
				domain.Timeframe1m,
				request.TimeRange.Start,
				time.Minute,
				[]float64{100, 101},
			)
			deps.strategyEvaluator.result = strategy.EvaluateResult{
				Strategy:   strategyIdentity,
				TimeRange:  request.TimeRange,
				Parameters: request.StrategyParameters,
				Actions:    []domain.CandidateAction{approvedAction},
			}
			deps.governorEvaluator.result = governor.EvaluateResult{
				Decisions: []domain.GovernorDecision{
					makeGovernorDecision(
						t,
						approvedAction,
						domain.GovernorDecisionStatusApproved,
						domain.GovernorDecisionReasonEligible,
					),
				},
			}

			flow := makeFlow(t, deps.paperBacktestDeps)

			result, err := flow.Run(t.Context(), request)

			require.Equal(t, PaperBacktestResult{}, result)
			require.EqualError(
				t,
				err,
				"prepare order intent 0 limit price: replay candle close price is required at decision time",
			)
			require.Zero(t, deps.executionRecorder.createCommandCalls)
			require.Zero(t, deps.executionRecorder.recordOrderCalls)
			require.Zero(t, deps.executionRecorder.recordFillCalls)
			require.Zero(t, deps.executionRecorder.reconcileCalls)
		})

		t.Run("wraps replay stage errors and stops downstream calls", func(t *testing.T) {
			t.Parallel()

			deps := makeMockDeps(nil)
			deps.replayReader.err = errors.New("boom-replay")
			flow := makeFlow(t, deps.paperBacktestDeps)

			result, err := flow.Run(t.Context(), makeRequest(t))

			require.Equal(t, PaperBacktestResult{}, result)
			require.EqualError(t, err, "replay candles: boom-replay")
			require.Empty(t, deps.analyticsCalc.calls)
			require.Empty(t, deps.strategyEvaluator.calls)
			require.Empty(t, deps.governorEvaluator.calls)
		})

		t.Run("wraps analytics stage errors and stops downstream calls", func(t *testing.T) {
			t.Parallel()

			deps := makeMockDeps(nil)
			deps.analyticsCalc.err = errors.New("boom-analytics")
			flow := makeFlow(t, deps.paperBacktestDeps)

			result, err := flow.Run(t.Context(), makeRequest(t))

			require.Equal(t, PaperBacktestResult{}, result)
			require.EqualError(t, err, "calculate fast moving average analytics: boom-analytics")
			require.Len(t, deps.analyticsCalc.calls, 1)
			require.Empty(t, deps.strategyEvaluator.calls)
			require.Empty(t, deps.governorEvaluator.calls)
		})

		t.Run("wraps strategy stage errors and stops downstream calls", func(t *testing.T) {
			t.Parallel()

			deps := makeMockDeps(nil)
			deps.strategyEvaluator.err = errors.New("boom-strategy")
			flow := makeFlow(t, deps.paperBacktestDeps)

			result, err := flow.Run(t.Context(), makeRequest(t))

			require.Equal(t, PaperBacktestResult{}, result)
			require.EqualError(t, err, "evaluate strategy: boom-strategy")
			require.Len(t, deps.analyticsCalc.calls, 2)
			require.Len(t, deps.strategyEvaluator.calls, 1)
			require.Empty(t, deps.governorEvaluator.calls)
		})

		t.Run("wraps governor stage errors and stops downstream calls", func(t *testing.T) {
			t.Parallel()

			deps := makeMockDeps(nil)
			deps.governorEvaluator.err = errors.New("boom-governor")
			flow := makeFlow(t, deps.paperBacktestDeps)

			result, err := flow.Run(t.Context(), makeRequest(t))

			require.Equal(t, PaperBacktestResult{}, result)
			require.EqualError(t, err, "evaluate governor: boom-governor")
			require.Len(t, deps.analyticsCalc.calls, 2)
			require.Len(t, deps.strategyEvaluator.calls, 1)
			require.Len(t, deps.governorEvaluator.calls, 1)
		})

		realScenarioName :=
			"real in-memory replay scenario exercises replay analytics strategy governor and local paper execution"

		t.Run(realScenarioName, func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			instrument := makeInstrument(t, fake)
			requestStart := time.Date(2026, 4, 5, 9, 0, 0, 0, time.FixedZone(randomWord(t, fake, "zone"), 0))
			requestRange, err := domain.NewTimeRange(requestStart, requestStart.Add(7*time.Minute))
			require.NoError(t, err)
			replayed := makeReplayCandles(
				t,
				fake,
				instrument,
				domain.Timeframe1m,
				requestStart,
				time.Minute,
				[]float64{10, 9, 8, 11, 12, 9, 8},
			)

			candleStore := &replayOnlyCandleStore{replayValue: replayed}
			readService, err := data.NewReadService(data.ReadServiceDeps{
				InstrumentStore: &replayOnlyInstrumentStore{},
				CandleStore:     candleStore,
				TradeStore:      &replayOnlyTradeStore{},
			})
			require.NoError(t, err)

			analyticsService, err := analytics.NewService(analytics.ServiceDeps{
				CandleReplayReader: readService,
			})
			require.NoError(t, err)

			strategyService, err := strategy.NewService(strategy.ServiceDeps{
				AnalyticsCalculator: analyticsService,
			})
			require.NoError(t, err)

			governorService := governor.NewService()
			auditStore, err := audit.NewDatabaseStore(":memory:", audit.DatabaseStoreOpts{})
			require.NoError(t, err)
			require.NoError(t, auditStore.AutoMigrate())
			auditService, err := audit.NewService(auditStore)
			require.NoError(t, err)
			executionRecorder := execution.NewService()
			flow := makeFlow(t, PaperBacktestFlowDeps{
				CandleReplayReader:  readService,
				AnalyticsCalculator: analyticsService,
				StrategyEvaluator:   strategyService,
				AuditRecorder:       auditService,
				GovernorEvaluator:   governorService,
				ExecutionRecorder:   executionRecorder,
			})

			result, err := flow.Run(t.Context(), PaperBacktestRequest{
				RunID:                "  " + randomWord(t, fake, "real-run") + "  ",
				Mode:                 domain.DecisionModeBacktest,
				StrategyID:           "  " + randomWord(t, fake, "real-strategy-id") + "  ",
				StrategyVersion:      "  " + randomWord(t, fake, "real-strategy-version") + "  ",
				StrategyArtifactHash: "  " + randomWord(t, fake, "real-strategy-artifact") + "  ",
				Instrument:           instrument,
				Timeframe:            domain.Timeframe1m,
				TimeRange:            requestRange,
				StrategyParameters: strategy.MovingAverageCrossoverParams{
					FastWindow: 2,
					SlowWindow: 3,
				},
				GovernorPolicy: governor.Policy{
					AllowedModes: []domain.DecisionMode{
						domain.DecisionModePaper,
						domain.DecisionModeBacktest,
					},
					AllowedActionKinds: []domain.CandidateActionKind{
						domain.CandidateActionKindLong,
						domain.CandidateActionKindShort,
					},
					MinimumQuality:       domain.DataQualityRaw,
					MaximumApprovedCount: 2,
				},
				Quantity: 1,
			})

			require.NoError(t, err)
			require.Len(t, candleStore.replayCalls, 5)
			require.Equal(t, strings.TrimSpace(result.RunID), result.RunID)
			require.Equal(t, requestRange, result.StrategyEvaluation.TimeRange)
			require.Equal(
				t,
				strategy.MovingAverageCrossoverParams{FastWindow: 2, SlowWindow: 3},
				result.StrategyEvaluation.Parameters,
			)
			require.Len(t, result.StrategyEvaluation.Actions, 2)
			require.Equal(t, domain.CandidateActionKindLong, result.StrategyEvaluation.Actions[0].Kind)
			require.Equal(
				t,
				requestStart.Add(4*time.Minute).UTC(),
				result.StrategyEvaluation.Actions[0].DecisionTime.Time(),
			)
			require.Equal(t, domain.CandidateActionKindShort, result.StrategyEvaluation.Actions[1].Kind)
			require.Equal(
				t,
				requestStart.Add(6*time.Minute).UTC(),
				result.StrategyEvaluation.Actions[1].DecisionTime.Time(),
			)
			require.Len(t, result.GovernorEvaluation.Decisions, 2)
			require.Equal(t, domain.GovernorDecisionStatusApproved, result.GovernorEvaluation.Decisions[0].Status)
			require.Equal(t, domain.GovernorDecisionStatusApproved, result.GovernorEvaluation.Decisions[1].Status)
			require.Len(t, result.PaperExecutions, 2)
			require.Equal(t, result.GovernorEvaluation.Decisions[0], result.PaperExecutions[0].ApprovedDecision)
			require.InDelta(t, 11.0, result.PaperExecutions[0].Fill.Price, 0)
			require.Equal(t, result.GovernorEvaluation.Decisions[1], result.PaperExecutions[1].ApprovedDecision)
			require.InDelta(t, 9.0, result.PaperExecutions[1].Fill.Price, 0)
		})
	})
}
