package governor

import (
	"hash/fnv"
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

	makeStrategy := func(t *testing.T, fake faker.Faker) domain.StrategyIdentity {
		t.Helper()

		strategy, err := domain.NewStrategyIdentity(domain.StrategyIdentityParams{
			Instrument: makeInstrument(t, fake),
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
			strategy := makeStrategy(t, fake)
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
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()

					result, err := service.Evaluate(t.Context(), EvaluateRequest{
						CandidateActions: []domain.CandidateAction{action},
						Policy:           testCase.policy,
					})

					require.ErrorIs(t, err, ErrValidation)
					require.ErrorContains(t, err, testCase.expectedMsg)
					require.Equal(t, EvaluateResult{}, result)
				})
			}
		})

		t.Run("returns stable decisions ordered by candidate decision time", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			service := NewService()
			strategy := makeStrategy(t, fake)
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
				CandidateActions: []domain.CandidateAction{lateAction, earlyAction, middleAction},
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

			expected := EvaluateResult{
				Decisions: []domain.GovernorDecision{
					makeDecision(
						t,
						earlyAction,
						domain.GovernorDecisionStatusApproved,
						domain.GovernorDecisionReasonEligible,
					),
					makeDecision(
						t,
						middleAction,
						domain.GovernorDecisionStatusApproved,
						domain.GovernorDecisionReasonEligible,
					),
					makeDecision(
						t,
						lateAction,
						domain.GovernorDecisionStatusApproved,
						domain.GovernorDecisionReasonEligible,
					),
				},
			}

			require.Equal(t, expected, firstResult)
			require.Equal(t, firstResult, secondResult)
		})

		t.Run("applies approval, rejection, and blocking policy rules", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			service := NewService()
			strategy := makeStrategy(t, fake)
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

			expected := EvaluateResult{
				Decisions: []domain.GovernorDecision{
					makeDecision(
						t,
						approvedAction,
						domain.GovernorDecisionStatusApproved,
						domain.GovernorDecisionReasonEligible,
					),
					makeDecision(
						t,
						disallowedAction,
						domain.GovernorDecisionStatusRejected,
						domain.GovernorDecisionReasonDisallowedActionKind,
					),
					makeDecision(
						t,
						belowMinimumAction,
						domain.GovernorDecisionStatusRejected,
						domain.GovernorDecisionReasonBelowMinimumQuality,
					),
					makeDecision(
						t,
						blockedAction,
						domain.GovernorDecisionStatusBlocked,
						domain.GovernorDecisionReasonApprovalLimitReached,
					),
				},
			}

			require.Equal(t, expected, result)
		})

		t.Run("evaluates canonical candidate actions without external dependencies", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			service := NewService()
			strategy := makeStrategy(t, fake)
			action := makeAction(
				t,
				strategy,
				domain.CandidateActionKindLong,
				randomTime(t, fake),
				domain.DataQualityValidated,
			)

			result, err := service.Evaluate(t.Context(), EvaluateRequest{
				CandidateActions: []domain.CandidateAction{action},
				Policy: Policy{
					AllowedActionKinds:   []domain.CandidateActionKind{domain.CandidateActionKindLong},
					MinimumQuality:       domain.DataQualityValidated,
					MaximumApprovedCount: 1,
				},
			})
			require.NoError(t, err)
			require.Equal(t, EvaluateResult{
				Decisions: []domain.GovernorDecision{
					makeDecision(
						t,
						action,
						domain.GovernorDecisionStatusApproved,
						domain.GovernorDecisionReasonEligible,
					),
				},
			}, result)
		})
	})
}
