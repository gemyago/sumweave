package execution

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

	t.Run("CreateCommand", func(t *testing.T) {
		t.Parallel()

		t.Run("creates stable command from approved decision", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			service := NewService()
			decision := makeDecision(
				t,
				makeAction(
					t,
					makeStrategy(t, fake),
					domain.CandidateActionKindLong,
					randomTime(t, fake),
					domain.DataQualityValidated,
				),
				domain.GovernorDecisionStatusApproved,
				domain.GovernorDecisionReasonEligible,
			)
			requestTime := randomTime(t, fake)
			quantity := float64(fake.IntBetween(1, 100)) + 0.25

			firstCommand, err := service.CreateCommand(t.Context(), CreateCommandRequest{
				ApprovedDecision: decision,
				Quantity:         quantity,
				EventTime:        requestTime,
			})
			require.NoError(t, err)

			secondCommand, err := service.CreateCommand(t.Context(), CreateCommandRequest{
				ApprovedDecision: decision,
				Quantity:         quantity,
				EventTime:        requestTime,
			})
			require.NoError(t, err)

			require.Equal(t, firstCommand, secondCommand)
			require.Equal(t, decision, firstCommand.ApprovedDecision)
			require.Equal(t, domain.ExecutionCommandStatusCreated, firstCommand.Status)
			require.InDelta(t, quantity, firstCommand.Quantity, 0)
			require.Equal(t, requestTime.UTC(), firstCommand.EventTime.Time())
			require.NotEmpty(t, firstCommand.CommandID)
		})

		t.Run("rejects non approved malformed and invalid inputs", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			service := NewService()
			strategy := makeStrategy(t, fake)
			decisionTime := randomTime(t, fake)
			candidateAction := makeAction(
				t,
				strategy,
				domain.CandidateActionKindShort,
				decisionTime,
				domain.DataQualityValidated,
			)
			requestTime := randomTime(t, fake)

			testCases := []struct {
				name        string
				request     CreateCommandRequest
				expectedMsg string
			}{
				{
					name: "rejected decision",
					request: CreateCommandRequest{
						ApprovedDecision: makeDecision(
							t,
							candidateAction,
							domain.GovernorDecisionStatusRejected,
							domain.GovernorDecisionReasonDisallowedActionKind,
						),
						Quantity:  float64(fake.IntBetween(1, 10)),
						EventTime: requestTime,
					},
					expectedMsg: "approved governor decision is required",
				},
				{
					name: "blocked decision",
					request: CreateCommandRequest{
						ApprovedDecision: makeDecision(
							t,
							candidateAction,
							domain.GovernorDecisionStatusBlocked,
							domain.GovernorDecisionReasonApprovalLimitReached,
						),
						Quantity:  float64(fake.IntBetween(1, 10)),
						EventTime: requestTime,
					},
					expectedMsg: "approved governor decision is required",
				},
				{
					name: "missing decision",
					request: CreateCommandRequest{
						Quantity:  float64(fake.IntBetween(1, 10)),
						EventTime: requestTime,
					},
					expectedMsg: "approved governor decision is required",
				},
				{
					name: "malformed decision missing candidate action",
					request: CreateCommandRequest{
						ApprovedDecision: domain.GovernorDecision{
							Status:       domain.GovernorDecisionStatusApproved,
							Reason:       domain.GovernorDecisionReasonEligible,
							DecisionTime: domain.GovernorDecisionTime(decisionTime.UTC()),
						},
						Quantity:  float64(fake.IntBetween(1, 10)),
						EventTime: requestTime,
					},
					expectedMsg: "approved governor decision",
				},
				{
					name: "non positive quantity",
					request: CreateCommandRequest{
						ApprovedDecision: makeDecision(
							t,
							candidateAction,
							domain.GovernorDecisionStatusApproved,
							domain.GovernorDecisionReasonEligible,
						),
						Quantity:  0,
						EventTime: requestTime,
					},
					expectedMsg: "execution command quantity must be positive",
				},
				{
					name: "nan quantity",
					request: CreateCommandRequest{
						ApprovedDecision: makeDecision(
							t,
							candidateAction,
							domain.GovernorDecisionStatusApproved,
							domain.GovernorDecisionReasonEligible,
						),
						Quantity:  math.NaN(),
						EventTime: requestTime,
					},
					expectedMsg: "execution command quantity must be finite",
				},
				{
					name: "infinite quantity",
					request: CreateCommandRequest{
						ApprovedDecision: makeDecision(
							t,
							candidateAction,
							domain.GovernorDecisionStatusApproved,
							domain.GovernorDecisionReasonEligible,
						),
						Quantity:  math.Inf(1),
						EventTime: requestTime,
					},
					expectedMsg: "execution command quantity must be finite",
				},
				{
					name: "missing event time",
					request: CreateCommandRequest{
						ApprovedDecision: makeDecision(
							t,
							candidateAction,
							domain.GovernorDecisionStatusApproved,
							domain.GovernorDecisionReasonEligible,
						),
						Quantity: float64(fake.IntBetween(1, 10)),
					},
					expectedMsg: "execution event time is required",
				},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()

					command, err := service.CreateCommand(t.Context(), testCase.request)

					require.ErrorIs(t, err, ErrValidation)
					require.ErrorContains(t, err, testCase.expectedMsg)
					require.Equal(t, domain.ExecutionCommand{}, command)
				})
			}
		})
	})

	t.Run("RecordOrder", func(t *testing.T) {
		t.Parallel()

		t.Run("records stable local order", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			service := NewService()
			command, venue := makeCommand(
				t,
				fake,
				service,
				makeDecision,
				makeAction,
				makeStrategy,
				randomTime,
				randomWord,
			)
			clientOrderID := "  " + randomWord(t, fake, "client-order") + "  "
			eventTime := randomTime(t, fake)
			quantity := float64(fake.IntBetween(1, 100)) + 0.5

			firstOrder, err := service.RecordOrder(t.Context(), RecordOrderRequest{
				Command:       command,
				Venue:         venue,
				ClientOrderID: clientOrderID,
				Quantity:      quantity,
				EventTime:     eventTime,
			})
			require.NoError(t, err)

			secondOrder, err := service.RecordOrder(t.Context(), RecordOrderRequest{
				Command:       command,
				Venue:         venue,
				ClientOrderID: clientOrderID,
				Quantity:      quantity,
				EventTime:     eventTime,
			})
			require.NoError(t, err)

			require.Equal(t, firstOrder, secondOrder)
			require.Equal(t, command, firstOrder.Command)
			require.Equal(t, venue, firstOrder.Venue)
			require.Equal(t, strings.TrimSpace(clientOrderID), firstOrder.ClientOrderID)
			require.Equal(t, domain.ExecutionOrderStatusOpen, firstOrder.Status)
			require.InDelta(t, quantity, firstOrder.Quantity, 0)
			require.Equal(t, eventTime.UTC(), firstOrder.EventTime.Time())
			require.NotEmpty(t, firstOrder.OrderID)
		})

		t.Run("rejects invalid order inputs", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			service := NewService()
			command, venue := makeCommand(
				t,
				fake,
				service,
				makeDecision,
				makeAction,
				makeStrategy,
				randomTime,
				randomWord,
			)
			rejectedCommand := command
			rejectedCommand.ApprovedDecision = makeDecision(
				t,
				command.ApprovedDecision.CandidateAction,
				domain.GovernorDecisionStatusRejected,
				domain.GovernorDecisionReasonDisallowedActionKind,
			)
			eventTime := randomTime(t, fake)

			testCases := []struct {
				name        string
				request     RecordOrderRequest
				expectedMsg string
			}{
				{
					name: "missing command",
					request: RecordOrderRequest{
						Venue:         venue,
						ClientOrderID: randomWord(t, fake, "client-order"),
						Quantity:      1,
						EventTime:     eventTime,
					},
					expectedMsg: "execution order command",
				},
				{
					name: "command decision not approved",
					request: RecordOrderRequest{
						Command:       rejectedCommand,
						Venue:         venue,
						ClientOrderID: randomWord(t, fake, "client-order"),
						Quantity:      1,
						EventTime:     eventTime,
					},
					expectedMsg: "execution approved decision must be approved",
				},
				{
					name: "missing venue",
					request: RecordOrderRequest{
						Command:       command,
						ClientOrderID: randomWord(t, fake, "client-order"),
						Quantity:      1,
						EventTime:     eventTime,
					},
					expectedMsg: "execution order venue is required",
				},
				{
					name: "blank client order id",
					request: RecordOrderRequest{
						Command:       command,
						Venue:         venue,
						ClientOrderID: "   ",
						Quantity:      1,
						EventTime:     eventTime,
					},
					expectedMsg: "execution order client order id is required",
				},
				{
					name: "non positive quantity",
					request: RecordOrderRequest{
						Command:       command,
						Venue:         venue,
						ClientOrderID: randomWord(t, fake, "client-order"),
						Quantity:      0,
						EventTime:     eventTime,
					},
					expectedMsg: "execution order quantity must be positive",
				},
				{
					name: "nan quantity",
					request: RecordOrderRequest{
						Command:       command,
						Venue:         venue,
						ClientOrderID: randomWord(t, fake, "client-order"),
						Quantity:      math.NaN(),
						EventTime:     eventTime,
					},
					expectedMsg: "execution order quantity must be finite",
				},
				{
					name: "missing event time",
					request: RecordOrderRequest{
						Command:       command,
						Venue:         venue,
						ClientOrderID: randomWord(t, fake, "client-order"),
						Quantity:      1,
					},
					expectedMsg: "execution event time is required",
				},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()

					order, err := service.RecordOrder(t.Context(), testCase.request)

					require.ErrorIs(t, err, ErrValidation)
					require.ErrorContains(t, err, testCase.expectedMsg)
					require.Equal(t, domain.ExecutionOrder{}, order)
				})
			}
		})
	})

	t.Run("RecordFill", func(t *testing.T) {
		t.Parallel()

		t.Run("records local fill", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			service := NewService()
			_, order := makeOrder(t, fake, service, makeDecision, makeAction, makeStrategy, randomTime, randomWord)
			fillID := "  " + randomWord(t, fake, "fill") + "  "
			eventTime := randomTime(t, fake)
			quantity := float64(fake.IntBetween(1, 50)) + 0.25
			price := float64(fake.IntBetween(100, 500)) + 0.75

			fill, err := service.RecordFill(t.Context(), RecordFillRequest{
				Order:     order,
				FillID:    fillID,
				Quantity:  quantity,
				Price:     price,
				EventTime: eventTime,
			})
			require.NoError(t, err)

			require.Equal(t, strings.TrimSpace(fillID), string(fill.FillID))
			require.Equal(t, order, fill.Order)
			require.Equal(t, order.Command, fill.Order.Command)
			require.InDelta(t, quantity, fill.Quantity, 0)
			require.InDelta(t, price, fill.Price, 0)
			require.Equal(t, eventTime.UTC(), fill.EventTime.Time())
		})

		t.Run("rejects invalid fill inputs", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			service := NewService()
			_, order := makeOrder(t, fake, service, makeDecision, makeAction, makeStrategy, randomTime, randomWord)
			eventTime := randomTime(t, fake)

			testCases := []struct {
				name        string
				request     RecordFillRequest
				expectedMsg string
			}{
				{
					name: "missing order",
					request: RecordFillRequest{
						FillID:    randomWord(t, fake, "fill"),
						Quantity:  1,
						Price:     1,
						EventTime: eventTime,
					},
					expectedMsg: "execution fill order",
				},
				{
					name: "blank fill id",
					request: RecordFillRequest{
						Order:     order,
						FillID:    " ",
						Quantity:  1,
						Price:     1,
						EventTime: eventTime,
					},
					expectedMsg: "execution fill id is required",
				},
				{
					name: "non positive quantity",
					request: RecordFillRequest{
						Order:     order,
						FillID:    randomWord(t, fake, "fill"),
						Quantity:  0,
						Price:     1,
						EventTime: eventTime,
					},
					expectedMsg: "execution fill quantity must be positive",
				},
				{
					name: "non positive price",
					request: RecordFillRequest{
						Order:     order,
						FillID:    randomWord(t, fake, "fill"),
						Quantity:  1,
						Price:     0,
						EventTime: eventTime,
					},
					expectedMsg: "execution fill price must be positive",
				},
				{
					name: "nan quantity",
					request: RecordFillRequest{
						Order:     order,
						FillID:    randomWord(t, fake, "fill"),
						Quantity:  math.NaN(),
						Price:     1,
						EventTime: eventTime,
					},
					expectedMsg: "execution fill quantity must be finite",
				},
				{
					name: "infinite price",
					request: RecordFillRequest{
						Order:     order,
						FillID:    randomWord(t, fake, "fill"),
						Quantity:  1,
						Price:     math.Inf(1),
						EventTime: eventTime,
					},
					expectedMsg: "execution fill price must be finite",
				},
				{
					name: "missing event time",
					request: RecordFillRequest{
						Order:    order,
						FillID:   randomWord(t, fake, "fill"),
						Quantity: 1,
						Price:    1,
					},
					expectedMsg: "execution event time is required",
				},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()

					fill, err := service.RecordFill(t.Context(), testCase.request)

					require.ErrorIs(t, err, ErrValidation)
					require.ErrorContains(t, err, testCase.expectedMsg)
					require.Equal(t, domain.ExecutionFill{}, fill)
				})
			}
		})
	})

	t.Run("Reconcile", func(t *testing.T) {
		t.Parallel()

		t.Run("returns open status with no fills", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			service := NewService()
			_, order := makeOrder(t, fake, service, makeDecision, makeAction, makeStrategy, randomTime, randomWord)

			reconciliation, err := service.Reconcile(t.Context(), ReconcileRequest{Order: order})
			require.NoError(t, err)

			require.Equal(t, order, reconciliation.Order)
			require.Empty(t, reconciliation.Fills)
			require.Equal(t, domain.ExecutionOrderStatusOpen, reconciliation.Status)
			require.Zero(t, reconciliation.FilledQuantity)
			require.Equal(t, order.EventTime.Time(), reconciliation.EventTime.Time())
		})

		t.Run("returns sorted fills and deterministic statuses", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			service := NewService()
			_, order := makeOrder(t, fake, service, makeDecision, makeAction, makeStrategy, randomTime, randomWord)
			baseTime := randomTime(t, fake)

			lateFill := makeFill(t, order, randomWord(t, fake, "fill-c"), 2.5, 101.5, baseTime.Add(2*time.Minute))
			tieFillB := makeFill(t, order, "z-"+randomWord(t, fake, "fill-b"), 1.5, 102.5, baseTime)
			tieFillA := makeFill(t, order, "a-"+randomWord(t, fake, "fill-a"), 1.0, 103.5, baseTime)

			partialOrder := order
			partialOrder.Quantity = 6.0

			partial, err := service.Reconcile(t.Context(), ReconcileRequest{
				Order: partialOrder,
				Fills: []domain.ExecutionFill{lateFill, tieFillB, tieFillA},
			})
			require.NoError(t, err)

			require.Equal(t, []domain.ExecutionFill{tieFillA, tieFillB, lateFill}, partial.Fills)
			require.Equal(t, domain.ExecutionOrderStatusPartiallyFilled, partial.Status)
			require.InDelta(t, 5.0, partial.FilledQuantity, 0)
			require.Equal(t, lateFill.EventTime.Time(), partial.EventTime.Time())

			filledOrder := order
			filledOrder.Quantity = 5.0
			filled, err := service.Reconcile(t.Context(), ReconcileRequest{
				Order: filledOrder,
				Fills: []domain.ExecutionFill{lateFill, tieFillB, tieFillA},
			})
			require.NoError(t, err)
			require.Equal(t, domain.ExecutionOrderStatusFilled, filled.Status)

			overfilledOrder := order
			overfilledOrder.Quantity = 4.0
			overfilled, err := service.Reconcile(t.Context(), ReconcileRequest{
				Order: overfilledOrder,
				Fills: []domain.ExecutionFill{lateFill, tieFillB, tieFillA},
			})
			require.NoError(t, err)
			require.Equal(t, domain.ExecutionOrderStatusOverfilled, overfilled.Status)
		})

		t.Run("treats decimal rounding edge cases as filled", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			service := NewService()
			_, order := makeOrder(t, fake, service, makeDecision, makeAction, makeStrategy, randomTime, randomWord)

			decimalOrder := order
			decimalOrder.Quantity = 0.3

			firstFill := makeFill(t, decimalOrder, "a-"+randomWord(t, fake, "fill-a"), 0.1, 101.5, randomTime(t, fake))
			secondFill := makeFill(
				t,
				decimalOrder,
				"b-"+randomWord(t, fake, "fill-b"),
				0.2,
				102.5,
				randomTime(t, fake).Add(time.Minute),
			)

			reconciliation, err := service.Reconcile(t.Context(), ReconcileRequest{
				Order: decimalOrder,
				Fills: []domain.ExecutionFill{secondFill, firstFill},
			})
			require.NoError(t, err)

			require.Equal(t, domain.ExecutionOrderStatusFilled, reconciliation.Status)
			require.InDelta(t, 0.3, reconciliation.FilledQuantity, 1e-12)
		})
	})

	t.Run("execution boundary stays local only", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		service := NewService()
		command, venue := makeCommand(t, fake, service, makeDecision, makeAction, makeStrategy, randomTime, randomWord)
		orderTime := randomTime(t, fake)

		order, err := service.RecordOrder(t.Context(), RecordOrderRequest{
			Command:       command,
			Venue:         venue,
			ClientOrderID: randomWord(t, fake, "client-order"),
			Quantity:      2,
			EventTime:     orderTime,
		})
		require.NoError(t, err)

		fill, err := service.RecordFill(t.Context(), RecordFillRequest{
			Order:     order,
			FillID:    randomWord(t, fake, "fill"),
			Quantity:  2,
			Price:     111.25,
			EventTime: orderTime.Add(time.Minute),
		})
		require.NoError(t, err)

		reconciliation, err := service.Reconcile(t.Context(), ReconcileRequest{
			Order: order,
			Fills: []domain.ExecutionFill{fill},
		})
		require.NoError(t, err)
		require.Equal(t, domain.ExecutionOrderStatusFilled, reconciliation.Status)
	})
}

func makeCommand(
	t *testing.T,
	fake faker.Faker,
	service *Service,
	makeDecision func(*testing.T, domain.CandidateAction, domain.GovernorDecisionStatus, domain.GovernorDecisionReason) domain.GovernorDecision,
	makeAction func(*testing.T, domain.StrategyIdentity, domain.CandidateActionKind, time.Time, domain.DataQuality) domain.CandidateAction,
	makeStrategy func(*testing.T, faker.Faker) domain.StrategyIdentity,
	randomTime func(*testing.T, faker.Faker) time.Time,
	randomWord func(*testing.T, faker.Faker, string) string,
) (domain.ExecutionCommand, domain.Venue) {
	t.Helper()

	decision := makeDecision(
		t,
		makeAction(
			t,
			makeStrategy(t, fake),
			domain.CandidateActionKindLong,
			randomTime(t, fake),
			domain.DataQualityValidated,
		),
		domain.GovernorDecisionStatusApproved,
		domain.GovernorDecisionReasonEligible,
	)

	command, err := service.CreateCommand(t.Context(), CreateCommandRequest{
		ApprovedDecision: decision,
		Quantity:         float64(fake.IntBetween(1, 10)) + 0.5,
		EventTime:        randomTime(t, fake),
	})
	require.NoError(t, err)

	venue, err := domain.NewVenue("  " + randomWord(t, fake, "execution-venue") + "  ")
	require.NoError(t, err)

	return command, venue
}

func makeOrder(
	t *testing.T,
	fake faker.Faker,
	service *Service,
	makeDecision func(*testing.T, domain.CandidateAction, domain.GovernorDecisionStatus, domain.GovernorDecisionReason) domain.GovernorDecision,
	makeAction func(*testing.T, domain.StrategyIdentity, domain.CandidateActionKind, time.Time, domain.DataQuality) domain.CandidateAction,
	makeStrategy func(*testing.T, faker.Faker) domain.StrategyIdentity,
	randomTime func(*testing.T, faker.Faker) time.Time,
	randomWord func(*testing.T, faker.Faker, string) string,
) (domain.ExecutionCommand, domain.ExecutionOrder) {
	t.Helper()

	command, venue := makeCommand(t, fake, service, makeDecision, makeAction, makeStrategy, randomTime, randomWord)
	order, err := service.RecordOrder(t.Context(), RecordOrderRequest{
		Command:       command,
		Venue:         venue,
		ClientOrderID: randomWord(t, fake, "client-order"),
		Quantity:      float64(fake.IntBetween(1, 10)) + 0.25,
		EventTime:     randomTime(t, fake),
	})
	require.NoError(t, err)

	return command, order
}

func makeFill(
	t *testing.T,
	order domain.ExecutionOrder,
	fillID string,
	quantity float64,
	price float64,
	eventTime time.Time,
) domain.ExecutionFill {
	t.Helper()

	fill, err := domain.NewExecutionFill(domain.ExecutionFillParams{
		FillID:    fillID,
		Order:     order,
		Quantity:  quantity,
		Price:     price,
		EventTime: eventTime,
	})
	require.NoError(t, err)

	return fill
}
