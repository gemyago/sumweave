package governor

import (
	"hash/fnv"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestService(t *testing.T) {
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

	makeStrategy := func(t *testing.T, instrument domain.Instrument) domain.StrategyIdentity {
		t.Helper()

		strategy, err := domain.NewStrategyIdentity(domain.StrategyIdentityParams{
			Instrument: instrument,
			Timeframe:  domain.Timeframe1m,
			Kind:       domain.StrategyKindMovingAverageCrossover,
		})
		require.NoError(t, err)

		return strategy
	}

	makeRange := func(t *testing.T, start time.Time, minutes int) domain.TimeRange {
		t.Helper()

		timeRange, err := domain.NewTimeRange(start, start.Add(time.Duration(minutes)*time.Minute))
		require.NoError(t, err)

		return timeRange
	}

	makeAction := func(
		t *testing.T,
		strategy domain.StrategyIdentity,
		kind domain.CandidateActionKind,
		decisionTime time.Time,
		quality domain.DataQuality,
	) domain.CandidateAction {
		t.Helper()

		action, err := domain.NewCandidateAction(domain.CandidateActionParams{
			Strategy:     strategy,
			Kind:         kind,
			DecisionTime: decisionTime,
			InputRange:   makeRange(t, decisionTime.Add(-5*time.Minute), 5),
			Quality:      quality,
		})
		require.NoError(t, err)

		return action
	}

	makeDecision := func(
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

	makeIntentInput := func(
		t *testing.T,
		fake faker.Faker,
		action domain.CandidateAction,
		strategyID string,
		strategyVersion string,
		strategyArtifactHash string,
		mode domain.DecisionMode,
		quantity float64,
		limitPrice float64,
	) IntentInput {
		t.Helper()

		intent, err := domain.NewOrderIntent(domain.OrderIntentParams{
			IntentID:                 randomWord(t, fake, "intent"),
			TraceID:                  randomWord(t, fake, "trace"),
			StrategyID:               strategyID,
			StrategyVersion:          strategyVersion,
			StrategyArtifactHash:     strategyArtifactHash,
			Mode:                     mode,
			Instrument:               action.Strategy.Instrument,
			Timeframe:                action.Strategy.Timeframe,
			ActionKind:               action.Kind,
			OrderType:                domain.OrderTypeLimit,
			RequestedQuantity:        quantity,
			RequestedNotional:        quantity * limitPrice,
			RequestedLimitPrice:      &limitPrice,
			SourceReasonCode:         string(domain.GovernorDecisionReasonOK),
			CandidateActionReference: randomWord(t, fake, "candidate-ref"),
			CreatedTime:              action.DecisionTime.Time(),
			Status:                   domain.OrderIntentStatusCreated,
			Metadata: map[string]string{
				"origin": randomWord(t, fake, "origin"),
			},
		})
		require.NoError(t, err)

		return IntentInput{
			CandidateAction:                   action,
			Intent:                            intent,
			CurrentStrategyExposureNotional:   0,
			CurrentInstrumentExposureNotional: 0,
			GovernorPolicyID:                  randomWord(t, fake, "policy-id"),
			GovernorPolicyVersion:             randomWord(t, fake, "policy-version"),
			GovernorPolicyHash:                randomWord(t, fake, "policy-hash"),
		}
	}

	basePolicy := func(
		instrument domain.Instrument,
		strategyID string,
	) Policy {
		return Policy{
			AllowedModes: []domain.DecisionMode{
				domain.DecisionModePaper,
				domain.DecisionModeBacktest,
			},
			AllowedVenues:      []domain.Venue{instrument.Venue},
			AllowedInstruments: []domain.Instrument{instrument},
			AllowedStrategyIDs: []string{strategyID},
			AllowedActionKinds: []domain.CandidateActionKind{
				domain.CandidateActionKindLong,
				domain.CandidateActionKindShort,
			},
			MinimumQuality:                    domain.DataQualityRaw,
			MaximumOrderNotional:              1000,
			MaximumStrategyExposureNotional:   2000,
			MaximumInstrumentExposureNotional: 2000,
			MaximumApprovedCount:              4,
		}
	}

	t.Run("NewService", func(t *testing.T) {
		t.Parallel()

		service := NewService()
		require.NotNil(t, service)
	})

	t.Run("Evaluate", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects invalid policy", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			service := NewService()
			instrument := makeInstrument(t, fake)
			strategy := makeStrategy(t, instrument)
			action := makeAction(
				t,
				strategy,
				domain.CandidateActionKindLong,
				randomTime(t, fake),
				domain.DataQualityValidated,
			)

			testCases := []struct {
				name        string
				policy      Policy
				expectedMsg string
			}{
				{
					name: "unsupported minimum quality",
					policy: Policy{
						AllowedActionKinds:   []domain.CandidateActionKind{domain.CandidateActionKindLong},
						MinimumQuality:       domain.DataQualitySuspect,
						MaximumApprovedCount: fake.IntBetween(0, 3),
					},
					expectedMsg: "unsupported minimum quality",
				},
				{
					name: "empty allowed action set",
					policy: Policy{
						MinimumQuality:       domain.DataQualityRaw,
						MaximumApprovedCount: fake.IntBetween(0, 3),
					},
					expectedMsg: "allowed action kinds are required",
				},
				{
					name: "negative maximum approved action count",
					policy: Policy{
						AllowedActionKinds:   []domain.CandidateActionKind{domain.CandidateActionKindLong},
						MinimumQuality:       domain.DataQualityRaw,
						MaximumApprovedCount: -fake.IntBetween(1, 3),
					},
					expectedMsg: "maximum approved action count must be zero or greater",
				},
				{
					name: "negative maximum order notional",
					policy: Policy{
						AllowedActionKinds:   []domain.CandidateActionKind{domain.CandidateActionKindLong},
						MinimumQuality:       domain.DataQualityRaw,
						MaximumOrderNotional: -1,
						MaximumApprovedCount: 1,
					},
					expectedMsg: "maximum order notional must be finite and zero or greater",
				},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()

					result, err := service.Evaluate(t.Context(), EvaluateRequest{
						CandidateActions: []domain.CandidateAction{
							action,
						},
						Policy: testCase.policy,
					})

					require.ErrorIs(t, err, ErrValidation)
					require.ErrorContains(t, err, testCase.expectedMsg)
					require.Equal(t, EvaluateResult{}, result)
				})
			}
		})

		t.Run("returns stable candidate-action decisions ordered by decision time", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			service := NewService()
			instrument := makeInstrument(t, fake)
			strategy := makeStrategy(t, instrument)
			baseTime := randomTime(t, fake)

			lateAction := makeAction(
				t,
				strategy,
				domain.CandidateActionKindLong,
				baseTime.Add(3*time.Minute),
				domain.DataQualityValidated,
			)
			earlyAction := makeAction(
				t,
				strategy,
				domain.CandidateActionKindShort,
				baseTime,
				domain.DataQualityValidated,
			)
			middleAction := makeAction(
				t,
				strategy,
				domain.CandidateActionKindLong,
				baseTime.Add(2*time.Minute),
				domain.DataQualityRaw,
			)

			request := EvaluateRequest{
				CandidateActions: []domain.CandidateAction{
					lateAction,
					earlyAction,
					middleAction,
				},
				Policy: Policy{
					AllowedActionKinds: []domain.CandidateActionKind{
						domain.CandidateActionKindLong,
						domain.CandidateActionKindShort,
					},
					MinimumQuality:       domain.DataQualityRaw,
					MaximumApprovedCount: 5,
				},
			}

			firstResult, err := service.Evaluate(t.Context(), request)
			require.NoError(t, err)

			secondResult, err := service.Evaluate(t.Context(), request)
			require.NoError(t, err)

			expected := EvaluateResult{Decisions: []domain.GovernorDecision{
				makeDecision(t, earlyAction, domain.GovernorDecisionStatusApproved, domain.GovernorDecisionReasonOK),
				makeDecision(t, middleAction, domain.GovernorDecisionStatusApproved, domain.GovernorDecisionReasonOK),
				makeDecision(t, lateAction, domain.GovernorDecisionStatusApproved, domain.GovernorDecisionReasonOK),
			}}

			require.Equal(t, expected, firstResult)
			require.Equal(t, firstResult, secondResult)
		})

		t.Run("applies candidate-action approval rejection and blocking rules", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			service := NewService()
			instrument := makeInstrument(t, fake)
			strategy := makeStrategy(t, instrument)
			baseTime := randomTime(t, fake)

			approvedAction := makeAction(
				t,
				strategy,
				domain.CandidateActionKindLong,
				baseTime,
				domain.DataQualityValidated,
			)
			disallowedAction := makeAction(
				t,
				strategy,
				domain.CandidateActionKindShort,
				baseTime.Add(1*time.Minute),
				domain.DataQualityValidated,
			)
			belowMinimumAction := makeAction(
				t,
				strategy,
				domain.CandidateActionKindLong,
				baseTime.Add(2*time.Minute),
				domain.DataQualityRaw,
			)
			blockedAction := makeAction(
				t,
				strategy,
				domain.CandidateActionKindLong,
				baseTime.Add(3*time.Minute),
				domain.DataQualityValidated,
			)

			result, err := service.Evaluate(t.Context(), EvaluateRequest{
				CandidateActions: []domain.CandidateAction{
					blockedAction,
					belowMinimumAction,
					disallowedAction,
					approvedAction,
				},
				Policy: Policy{
					AllowedActionKinds:   []domain.CandidateActionKind{domain.CandidateActionKindLong},
					MinimumQuality:       domain.DataQualityValidated,
					MaximumApprovedCount: 1,
				},
			})
			require.NoError(t, err)

			expected := EvaluateResult{Decisions: []domain.GovernorDecision{
				makeDecision(
					t,
					approvedAction,
					domain.GovernorDecisionStatusApproved,
					domain.GovernorDecisionReasonOK,
				),
				makeDecision(
					t,
					disallowedAction,
					domain.GovernorDecisionStatusRejected,
					domain.GovernorDecisionReasonActionKindNotAllowed,
				),
				makeDecision(
					t,
					belowMinimumAction,
					domain.GovernorDecisionStatusRejected,
					domain.GovernorDecisionReasonDataQualityTooLow,
				),
				makeDecision(
					t,
					blockedAction,
					domain.GovernorDecisionStatusBlocked,
					domain.GovernorDecisionReasonApprovalLimitReached,
				),
			}}

			require.Equal(t, expected, result)
		})

		t.Run("rejects invalid intent inputs with INVALID_INTENT", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			service := NewService()
			instrument := makeInstrument(t, fake)
			strategy := makeStrategy(t, instrument)
			action := makeAction(
				t,
				strategy,
				domain.CandidateActionKindLong,
				randomTime(t, fake),
				domain.DataQualityValidated,
			)
			strategyID := randomWord(t, fake, "strategy-id")
			strategyVersion := randomWord(t, fake, "strategy-version")
			strategyArtifactHash := randomWord(t, fake, "strategy-artifact")
			baseInput := makeIntentInput(
				t,
				fake,
				action,
				strategyID,
				strategyVersion,
				strategyArtifactHash,
				domain.DecisionModeBacktest,
				2,
				101,
			)
			policy := basePolicy(instrument, strategyID)

			testCases := []struct {
				name        string
				mutate      func(IntentInput) IntentInput
				expectedMsg string
			}{
				{
					name: "missing mode",
					mutate: func(input IntentInput) IntentInput {
						input.Intent.Mode = ""
						return input
					},
					expectedMsg: "order intent mode is required",
				},
				{
					name: "missing strategy id",
					mutate: func(input IntentInput) IntentInput {
						input.Intent.StrategyID = ""
						return input
					},
					expectedMsg: "order intent strategy id is required",
				},
				{
					name: "missing quantity and notional",
					mutate: func(input IntentInput) IntentInput {
						input.Intent.RequestedQuantity = 0
						input.Intent.RequestedNotional = 0
						return input
					},
					expectedMsg: "order intent requested quantity or requested notional is required",
				},
				{
					name: "missing limit price",
					mutate: func(input IntentInput) IntentInput {
						input.Intent.RequestedLimitPrice = nil
						return input
					},
					expectedMsg: "order intent requested limit price is required for limit orders",
				},
				{
					name: "missing policy hash",
					mutate: func(input IntentInput) IntentInput {
						input.GovernorPolicyHash = ""
						return input
					},
					expectedMsg: "governor policy hash is required",
				},
				{
					name: "invalid strategy exposure",
					mutate: func(input IntentInput) IntentInput {
						input.CurrentStrategyExposureNotional = math.NaN()
						return input
					},
					expectedMsg: "current strategy exposure notional must be finite",
				},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()

					result, err := service.Evaluate(t.Context(), EvaluateRequest{
						IntentInputs: []IntentInput{testCase.mutate(baseInput)},
						Policy:       policy,
					})

					require.ErrorIs(t, err, ErrValidation)
					require.ErrorContains(t, err, string(domain.GovernorDecisionReasonInvalidIntent))
					require.ErrorContains(t, err, testCase.expectedMsg)
					require.Equal(t, EvaluateResult{}, result)
				})
			}
		})

		t.Run("applies intent mode scope kill-switch and notional checks with canonical reasons", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			service := NewService()
			instrument := makeInstrument(t, fake)
			strategy := makeStrategy(t, instrument)
			action := makeAction(
				t,
				strategy,
				domain.CandidateActionKindLong,
				randomTime(t, fake),
				domain.DataQualityValidated,
			)
			strategyID := randomWord(t, fake, "strategy-id")
			strategyVersion := randomWord(t, fake, "strategy-version")
			strategyArtifactHash := randomWord(t, fake, "strategy-artifact")
			allowedPolicy := basePolicy(instrument, strategyID)
			baseInput := makeIntentInput(
				t,
				fake,
				action,
				strategyID,
				strategyVersion,
				strategyArtifactHash,
				domain.DecisionModeBacktest,
				2,
				100,
			)

			otherInstrument := makeInstrument(t, fake)
			otherVenue, err := domain.NewVenue(randomWord(t, fake, "other-venue"))
			require.NoError(t, err)

			testCases := []struct {
				name           string
				mutatePolicy   func(Policy) Policy
				mutateInput    func(IntentInput) IntentInput
				expectedStatus domain.GovernorDecisionStatus
				expectedReason domain.GovernorDecisionReason
			}{
				{
					name:           "approves valid input",
					mutatePolicy:   func(policy Policy) Policy { return policy },
					mutateInput:    func(input IntentInput) IntentInput { return input },
					expectedStatus: domain.GovernorDecisionStatusApproved,
					expectedReason: domain.GovernorDecisionReasonOK,
				},
				{
					name:         "rejects live mode",
					mutatePolicy: func(policy Policy) Policy { return policy },
					mutateInput: func(input IntentInput) IntentInput {
						input.Intent.Mode = domain.DecisionModeLive
						return input
					},
					expectedStatus: domain.GovernorDecisionStatusRejected,
					expectedReason: domain.GovernorDecisionReasonModeNotAllowed,
				},
				{
					name: "rejects disallowed venue",
					mutatePolicy: func(policy Policy) Policy {
						policy.AllowedVenues = []domain.Venue{otherVenue}
						return policy
					},
					mutateInput:    func(input IntentInput) IntentInput { return input },
					expectedStatus: domain.GovernorDecisionStatusRejected,
					expectedReason: domain.GovernorDecisionReasonVenueNotAllowed,
				},
				{
					name: "rejects disallowed instrument",
					mutatePolicy: func(policy Policy) Policy {
						policy.AllowedInstruments = []domain.Instrument{otherInstrument}
						return policy
					},
					mutateInput:    func(input IntentInput) IntentInput { return input },
					expectedStatus: domain.GovernorDecisionStatusRejected,
					expectedReason: domain.GovernorDecisionReasonInstrumentNotAllowed,
				},
				{
					name: "rejects disallowed strategy",
					mutatePolicy: func(policy Policy) Policy {
						policy.AllowedStrategyIDs = []string{randomWord(t, fake, "other-strategy")}
						return policy
					},
					mutateInput:    func(input IntentInput) IntentInput { return input },
					expectedStatus: domain.GovernorDecisionStatusRejected,
					expectedReason: domain.GovernorDecisionReasonStrategyNotAllowed,
				},
				{
					name: "rejects disallowed action kind",
					mutatePolicy: func(policy Policy) Policy {
						policy.AllowedActionKinds = []domain.CandidateActionKind{domain.CandidateActionKindShort}
						return policy
					},
					mutateInput:    func(input IntentInput) IntentInput { return input },
					expectedStatus: domain.GovernorDecisionStatusRejected,
					expectedReason: domain.GovernorDecisionReasonActionKindNotAllowed,
				},
				{
					name: "rejects low quality",
					mutatePolicy: func(policy Policy) Policy {
						policy.MinimumQuality = domain.DataQualityValidated
						return policy
					},
					mutateInput: func(input IntentInput) IntentInput {
						input.CandidateAction.Quality = domain.DataQualityRaw
						return input
					},
					expectedStatus: domain.GovernorDecisionStatusRejected,
					expectedReason: domain.GovernorDecisionReasonDataQualityTooLow,
				},
				{
					name: "blocks kill switch",
					mutatePolicy: func(policy Policy) Policy {
						policy.BlockNewRisk = true
						return policy
					},
					mutateInput:    func(input IntentInput) IntentInput { return input },
					expectedStatus: domain.GovernorDecisionStatusBlocked,
					expectedReason: domain.GovernorDecisionReasonKillSwitchActive,
				},
				{
					name: "rejects order notional over limit",
					mutatePolicy: func(policy Policy) Policy {
						policy.MaximumOrderNotional = 50
						return policy
					},
					mutateInput:    func(input IntentInput) IntentInput { return input },
					expectedStatus: domain.GovernorDecisionStatusRejected,
					expectedReason: domain.GovernorDecisionReasonOrderNotionalExceedsLimit,
				},
				{
					name: "rejects projected strategy exposure over limit",
					mutatePolicy: func(policy Policy) Policy {
						policy.MaximumStrategyExposureNotional = 150
						return policy
					},
					mutateInput: func(input IntentInput) IntentInput {
						input.CurrentStrategyExposureNotional = 75
						return input
					},
					expectedStatus: domain.GovernorDecisionStatusRejected,
					expectedReason: domain.GovernorDecisionReasonStrategyExposureExceedsLimit,
				},
				{
					name: "rejects projected instrument exposure over limit",
					mutatePolicy: func(policy Policy) Policy {
						policy.MaximumInstrumentExposureNotional = 150
						return policy
					},
					mutateInput: func(input IntentInput) IntentInput {
						input.CurrentInstrumentExposureNotional = 75
						return input
					},
					expectedStatus: domain.GovernorDecisionStatusRejected,
					expectedReason: domain.GovernorDecisionReasonInstrumentExposureExceedsLimit,
				},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()

					input := testCase.mutateInput(baseInput)
					result, evalErr := service.Evaluate(t.Context(), EvaluateRequest{
						IntentInputs: []IntentInput{input},
						Policy:       testCase.mutatePolicy(allowedPolicy),
					})
					require.NoError(t, evalErr)
					require.Equal(t, EvaluateResult{Decisions: []domain.GovernorDecision{
						makeDecision(t, input.CandidateAction, testCase.expectedStatus, testCase.expectedReason),
					}}, result)
					require.Equal(
						t,
						strings.ToUpper(testCase.expectedReason.String()),
						testCase.expectedReason.String(),
					)
				})
			}
		})

		t.Run("blocks after approval limit and stays deterministic for repeated intent evaluation", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			service := NewService()
			instrument := makeInstrument(t, fake)
			strategy := makeStrategy(t, instrument)
			strategyID := randomWord(t, fake, "strategy-id")
			strategyVersion := randomWord(t, fake, "strategy-version")
			strategyArtifactHash := randomWord(t, fake, "strategy-artifact")
			baseTime := randomTime(t, fake)

			firstAction := makeAction(
				t,
				strategy,
				domain.CandidateActionKindLong,
				baseTime,
				domain.DataQualityValidated,
			)
			secondAction := makeAction(
				t,
				strategy,
				domain.CandidateActionKindShort,
				baseTime.Add(time.Minute),
				domain.DataQualityValidated,
			)
			firstInput := makeIntentInput(
				t,
				fake,
				firstAction,
				strategyID,
				strategyVersion,
				strategyArtifactHash,
				domain.DecisionModePaper,
				1,
				100,
			)
			secondInput := makeIntentInput(
				t,
				fake,
				secondAction,
				strategyID,
				strategyVersion,
				strategyArtifactHash,
				domain.DecisionModePaper,
				1,
				100,
			)

			policy := basePolicy(instrument, strategyID)
			policy.MaximumApprovedCount = 1

			request := EvaluateRequest{
				IntentInputs: []IntentInput{secondInput, firstInput},
				Policy:       policy,
			}

			firstResult, err := service.Evaluate(t.Context(), request)
			require.NoError(t, err)

			secondResult, err := service.Evaluate(t.Context(), request)
			require.NoError(t, err)

			expected := EvaluateResult{Decisions: []domain.GovernorDecision{
				makeDecision(
					t,
					firstAction,
					domain.GovernorDecisionStatusApproved,
					domain.GovernorDecisionReasonOK,
				),
				makeDecision(
					t,
					secondAction,
					domain.GovernorDecisionStatusBlocked,
					domain.GovernorDecisionReasonApprovalLimitReached,
				),
			}}

			require.Equal(t, expected, firstResult)
			require.Equal(t, firstResult, secondResult)
		})
	})
}
