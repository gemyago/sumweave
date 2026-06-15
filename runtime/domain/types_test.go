package domain

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestDomain(t *testing.T) {
	t.Parallel()

	fake := faker.New()

	randomWord := func(prefix string) string {
		return prefix + "-" + strings.ToLower(fake.Lorem().Word())
	}

	randomLocationTime := func() time.Time {
		zone := time.FixedZone(randomWord("zone"), fake.IntBetween(-11, 12)*3600)
		return time.Date(
			fake.IntBetween(2020, 2032),
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 28),
			fake.IntBetween(0, 23),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 999999999),
			zone,
		)
	}

	validAssetClasses := []AssetClass{
		AssetClassCrypto,
		AssetClassEquity,
		AssetClassFX,
		AssetClassFuture,
		AssetClassIndex,
		AssetClassOption,
	}
	validTimeframes := []Timeframe{
		Timeframe1m,
		Timeframe5m,
		Timeframe15m,
		Timeframe1h,
		Timeframe4h,
		Timeframe1d,
	}
	validQualities := []DataQuality{
		DataQualityRaw,
		DataQualityValidated,
		DataQualitySuspect,
	}
	validIndicatorKinds := []IndicatorKind{
		IndicatorKindMovingAverage,
		IndicatorKindPeriodReturn,
	}
	validStrategyKinds := []StrategyKind{
		StrategyKindMovingAverageCrossover,
	}
	validCandidateActionKinds := []CandidateActionKind{
		CandidateActionKindLong,
		CandidateActionKindShort,
	}
	validGovernorDecisionStatuses := []GovernorDecisionStatus{
		GovernorDecisionStatusApproved,
		GovernorDecisionStatusRejected,
		GovernorDecisionStatusBlocked,
	}
	validGovernorDecisionReasons := []GovernorDecisionReason{
		GovernorDecisionReasonEligible,
		GovernorDecisionReasonDisallowedActionKind,
		GovernorDecisionReasonBelowMinimumQuality,
		GovernorDecisionReasonApprovalLimitReached,
	}
	validDecisionModes := []DecisionMode{
		DecisionModePaper,
		DecisionModeBacktest,
		DecisionModeLive,
	}
	validDecisionTraceResults := []DecisionTraceResult{
		DecisionTraceResultNoAction,
		DecisionTraceResultIntentCreated,
		DecisionTraceResultBlockedBeforeIntent,
		DecisionTraceResultError,
	}
	validOrderTypes := []OrderType{OrderTypeLimit}
	validOrderIntentStatuses := []OrderIntentStatus{
		OrderIntentStatusCreated,
		OrderIntentStatusSentToGovernor,
		OrderIntentStatusApproved,
		OrderIntentStatusRejected,
		OrderIntentStatusBlocked,
		OrderIntentStatusExecutionCreated,
	}
	validExecutionCommandStatuses := []ExecutionCommandStatus{
		ExecutionCommandStatusCreated,
	}
	validExecutionOrderStatuses := []ExecutionOrderStatus{
		ExecutionOrderStatusOpen,
		ExecutionOrderStatusPartiallyFilled,
		ExecutionOrderStatusFilled,
		ExecutionOrderStatusOverfilled,
	}

	randomInstrument := func(t *testing.T) Instrument {
		t.Helper()

		venue, err := NewVenue(randomWord("venue"))
		require.NoError(t, err)

		symbol, err := NewSymbol(strings.ToUpper(randomWord("symbol")))
		require.NoError(t, err)

		instrument, err := NewInstrument(InstrumentParams{
			Venue:      venue,
			Symbol:     symbol,
			AssetClass: validAssetClasses[fake.IntBetween(0, len(validAssetClasses)-1)],
			Active:     fake.Bool(),
		})
		require.NoError(t, err)

		return instrument
	}

	randomProvenance := func(t *testing.T) SourceProvenance {
		t.Helper()

		provenance, err := NewSourceProvenance(randomWord("source"), "  "+randomWord("record")+"  ")
		require.NoError(t, err)

		return provenance
	}

	randomReplayIdentity := func() uint64 {
		return uint64(fake.IntBetween(1, 1_000_000))
	}

	randomCandidateAction := func(t *testing.T) CandidateAction {
		t.Helper()

		decisionTime := randomLocationTime()
		inputStart := randomLocationTime()
		inputEnd := inputStart.Add(time.Duration(fake.IntBetween(1, 180)) * time.Minute)
		identity, err := NewStrategyIdentity(StrategyIdentityParams{
			Instrument: randomInstrument(t),
			Timeframe:  validTimeframes[fake.IntBetween(0, len(validTimeframes)-1)],
			Kind:       validStrategyKinds[fake.IntBetween(0, len(validStrategyKinds)-1)],
		})
		require.NoError(t, err)

		action, err := NewCandidateAction(CandidateActionParams{
			Strategy:     identity,
			Kind:         validCandidateActionKinds[fake.IntBetween(0, len(validCandidateActionKinds)-1)],
			DecisionTime: decisionTime,
			InputRange: TimeRange{
				Start: inputStart,
				End:   inputEnd,
			},
			Quality: validQualities[fake.IntBetween(0, len(validQualities)-1)],
		})
		require.NoError(t, err)

		return action
	}

	randomApprovedDecision := func(t *testing.T) GovernorDecision {
		t.Helper()

		decision, err := NewGovernorDecision(GovernorDecisionParams{
			CandidateAction: randomCandidateAction(t),
			Status:          GovernorDecisionStatusApproved,
			Reason:          GovernorDecisionReasonEligible,
			DecisionTime:    randomLocationTime(),
		})
		require.NoError(t, err)

		return decision
	}

	t.Run("constructors validate canonical strings and enums", func(t *testing.T) {
		t.Parallel()

		venueText := "  " + randomWord("venue") + "  "
		symbolText := "  " + strings.ToUpper(randomWord("symbol")) + "  "
		assetClassText := "  " + strings.ToUpper(
			validAssetClasses[fake.IntBetween(0, len(validAssetClasses)-1)].String(),
		) + "  "
		timeframeText := "  " + strings.ToUpper(
			validTimeframes[fake.IntBetween(0, len(validTimeframes)-1)].String(),
		) + "  "
		qualityText := "  " + strings.ToUpper(
			validQualities[fake.IntBetween(0, len(validQualities)-1)].String(),
		) + "  "

		venue, err := NewVenue(venueText)
		require.NoError(t, err)
		require.Equal(t, strings.TrimSpace(venueText), venue.String())

		symbol, err := NewSymbol(symbolText)
		require.NoError(t, err)
		require.Equal(t, strings.TrimSpace(symbolText), symbol.String())

		assetClass, err := NewAssetClass(assetClassText)
		require.NoError(t, err)
		require.Equal(t, strings.ToLower(strings.TrimSpace(assetClassText)), assetClass.String())

		timeframe, err := NewTimeframe(timeframeText)
		require.NoError(t, err)
		require.Equal(t, strings.ToLower(strings.TrimSpace(timeframeText)), timeframe.String())

		quality, err := NewDataQuality(qualityText)
		require.NoError(t, err)
		require.Equal(t, strings.ToLower(strings.TrimSpace(qualityText)), quality.String())

		indicatorKindText := "  " + strings.ToUpper(
			validIndicatorKinds[fake.IntBetween(0, len(validIndicatorKinds)-1)].String(),
		) + "  "
		indicatorKind, err := NewIndicatorKind(indicatorKindText)
		require.NoError(t, err)
		require.Equal(t, strings.ToLower(strings.TrimSpace(indicatorKindText)), indicatorKind.String())

		_, err = NewAssetClass(randomWord("bad-asset-class"))
		require.Error(t, err)
		_, err = NewTimeframe(randomWord("bad-timeframe"))
		require.Error(t, err)
		_, err = NewDataQuality(randomWord("bad-quality"))
		require.Error(t, err)
		_, err = NewIndicatorKind(randomWord("bad-indicator-kind"))
		require.Error(t, err)
	})

	t.Run("canonical records normalize UTC timestamps and compare as whole values", func(t *testing.T) {
		t.Parallel()

		venue, err := NewVenue(randomWord("venue"))
		require.NoError(t, err)
		symbol, err := NewSymbol(strings.ToUpper(randomWord("symbol")))
		require.NoError(t, err)
		assetClass := validAssetClasses[fake.IntBetween(0, len(validAssetClasses)-1)]
		instrument, err := NewInstrument(InstrumentParams{
			Venue:      venue,
			Symbol:     symbol,
			AssetClass: assetClass,
			Active:     fake.Bool(),
		})
		require.NoError(t, err)

		provenance, err := NewSourceProvenance(randomWord("source"), "  "+randomWord("record")+"  ")
		require.NoError(t, err)

		start := randomLocationTime()
		end := start.Add(time.Duration(fake.IntBetween(1, 180)) * time.Minute)
		timeRange, err := NewTimeRange(start, end)
		require.NoError(t, err)
		require.Equal(t, time.UTC, timeRange.Start.Location())
		require.Equal(t, time.UTC, timeRange.End.Location())

		timeframe := validTimeframes[fake.IntBetween(0, len(validTimeframes)-1)]
		quality := validQualities[fake.IntBetween(0, len(validQualities)-1)]

		candle, err := NewCandle(CandleParams{
			Instrument: instrument,
			Timeframe:  timeframe,
			TimeRange:  timeRange,
			Open:       fake.Float64(2, 1, 9999),
			High:       fake.Float64(2, 1, 9999),
			Low:        fake.Float64(2, 1, 9999),
			Close:      fake.Float64(2, 1, 9999),
			Volume:     fake.Float64(4, 0, 99999),
			Quality:    quality,
			Provenance: provenance,
		})
		require.NoError(t, err)

		expectedCandle := Candle{
			Instrument: instrument,
			Timeframe:  timeframe,
			TimeRange: TimeRange{
				Start: start.UTC(),
				End:   end.UTC(),
			},
			Open:    candle.Open,
			High:    candle.High,
			Low:     candle.Low,
			Close:   candle.Close,
			Volume:  candle.Volume,
			Quality: quality,
			Provenance: SourceProvenance{
				Source:   provenance.Source,
				RecordID: strings.TrimSpace(provenance.RecordID),
			},
		}
		require.Equal(t, expectedCandle, candle)

		eventTime := randomLocationTime()
		trade, err := NewTrade(TradeParams{
			Instrument: instrument,
			EventTime:  eventTime,
			Price:      fake.Float64(4, 1, 99999),
			Size:       fake.Float64(6, 0, 99999),
			Quality:    quality,
			Provenance: provenance,
		})
		require.NoError(t, err)

		expectedTrade := Trade{
			Instrument: instrument,
			EventTime:  eventTime.UTC(),
			Price:      trade.Price,
			Size:       trade.Size,
			Quality:    quality,
			Provenance: SourceProvenance{
				Source:   provenance.Source,
				RecordID: strings.TrimSpace(provenance.RecordID),
			},
		}
		require.Equal(t, expectedTrade, trade)
		require.Equal(t, time.UTC, trade.EventTime.Location())
	})

	t.Run("audit records validate required fields and normalize UTC timestamps", func(t *testing.T) {
		t.Parallel()

		instrument := randomInstrument(t)
		decisionTime := randomLocationTime()
		createdAt := randomLocationTime()
		inputStart := randomLocationTime()
		inputEnd := inputStart.Add(time.Duration(fake.IntBetween(1, 180)) * time.Minute)

		trace, err := NewDecisionTrace(DecisionTraceParams{
			TraceID:              randomWord("trace-id"),
			Mode:                 validDecisionModes[fake.IntBetween(0, len(validDecisionModes)-1)],
			DecisionTime:         decisionTime,
			StrategyID:           randomWord("strategy-id"),
			StrategyVersion:      randomWord("strategy-version"),
			StrategyArtifactHash: randomWord("strategy-artifact"),
			Instrument:           instrument,
			Timeframe:            validTimeframes[fake.IntBetween(0, len(validTimeframes)-1)],
			InputRange:           TimeRange{Start: inputStart, End: inputEnd},
			DataQuality:          validQualities[fake.IntBetween(0, len(validQualities)-1)],
			EvaluatorName:        randomWord("evaluator-name"),
			EvaluatorVersion:     randomWord("evaluator-version"),
			Result:               validDecisionTraceResults[fake.IntBetween(0, len(validDecisionTraceResults)-1)],
			ReasonCodes:          []string{"OK"},
			Metadata:             map[string]string{"scope": "unit-test"},
		})
		require.NoError(t, err)
		require.Equal(t, decisionTime.UTC(), trace.DecisionTime.Time())
		require.Equal(t, inputStart.UTC(), trace.InputRange.Start)
		require.Equal(t, inputEnd.UTC(), trace.InputRange.End)

		limitPrice := fake.Float64(2, 1, 1000)
		intent, err := NewOrderIntent(OrderIntentParams{
			IntentID:                 randomWord("intent-id"),
			TraceID:                  string(trace.TraceID),
			StrategyID:               trace.StrategyID,
			StrategyVersion:          trace.StrategyVersion,
			StrategyArtifactHash:     trace.StrategyArtifactHash,
			Mode:                     trace.Mode,
			Instrument:               instrument,
			Timeframe:                trace.Timeframe,
			ActionKind:               validCandidateActionKinds[fake.IntBetween(0, len(validCandidateActionKinds)-1)],
			OrderType:                validOrderTypes[fake.IntBetween(0, len(validOrderTypes)-1)],
			RequestedQuantity:        fake.Float64(2, 1, 100),
			RequestedLimitPrice:      &limitPrice,
			SourceReasonCode:         "OK",
			CandidateActionReference: randomWord("candidate-action-ref"),
			CreatedTime:              createdAt,
			Status:                   validOrderIntentStatuses[fake.IntBetween(0, len(validOrderIntentStatuses)-1)],
			Metadata:                 map[string]string{"flow": "paper-backtest"},
		})
		require.NoError(t, err)
		require.Equal(t, createdAt.UTC(), intent.CreatedTime.Time())
		require.NotNil(t, intent.RequestedLimitPrice)

		_, err = NewOrderIntent(OrderIntentParams{
			IntentID:             randomWord("bad-intent-id"),
			TraceID:              string(trace.TraceID),
			StrategyID:           trace.StrategyID,
			StrategyVersion:      trace.StrategyVersion,
			StrategyArtifactHash: trace.StrategyArtifactHash,
			Mode:                 trace.Mode,
			Instrument:           instrument,
			Timeframe:            trace.Timeframe,
			ActionKind:           validCandidateActionKinds[0],
			OrderType:            OrderTypeLimit,
			RequestedQuantity:    1,
			CreatedTime:          createdAt,
			Status:               OrderIntentStatusCreated,
		})
		require.Error(t, err)
	})

	t.Run("domain structs remain persistence free", func(t *testing.T) {
		t.Parallel()

		forbiddenFields := map[string]struct{}{
			"ID":        {},
			"CreatedAt": {},
			"UpdatedAt": {},
			"DeletedAt": {},
		}
		structTypes := []reflect.Type{
			reflect.TypeFor[Instrument](),
			reflect.TypeFor[SourceProvenance](),
			reflect.TypeFor[TimeRange](),
			reflect.TypeFor[Candle](),
			reflect.TypeFor[Trade](),
			reflect.TypeFor[IndicatorParams](),
			reflect.TypeFor[AnalyticsSeriesIdentity](),
			reflect.TypeFor[AnalyticsSeries](),
			reflect.TypeFor[AnalyticsPointTime](),
			reflect.TypeFor[AnalyticsValueRange](),
			reflect.TypeFor[AnalyticsPoint](),
			reflect.TypeFor[StrategyIdentity](),
			reflect.TypeFor[CandidateAction](),
			reflect.TypeFor[DecisionTrace](),
			reflect.TypeFor[OrderIntent](),
			reflect.TypeFor[GovernorDecision](),
			reflect.TypeFor[ExecutionCommand](),
			reflect.TypeFor[ExecutionOrder](),
			reflect.TypeFor[ExecutionFill](),
			reflect.TypeFor[ExecutionReconciliation](),
		}

		for _, typ := range structTypes {
			_, hasValueTableName := typ.MethodByName("TableName")
			require.False(t, hasValueTableName, "%s should not expose TableName", typ.Name())

			_, hasPointerTableName := reflect.PointerTo(typ).MethodByName("TableName")
			require.False(t, hasPointerTableName, "%s should not expose pointer TableName", typ.Name())

			for _, field := range reflect.VisibleFields(typ) {
				require.Empty(t, field.Tag.Get("gorm"), "%s.%s should not carry gorm tags", typ.Name(), field.Name)

				_, forbidden := forbiddenFields[field.Name]
				require.False(t, forbidden, "%s.%s should not carry persistence-only fields", typ.Name(), field.Name)
			}
		}
	})

	t.Run("analytics identity canonicalizes embedded instrument and UTC timestamps", func(t *testing.T) {
		t.Parallel()

		start := randomLocationTime()
		end := start.Add(time.Duration(fake.IntBetween(1, 180)) * time.Minute)
		venueText := "  " + randomWord("venue") + "  "
		symbolText := "  " + strings.ToUpper(randomWord("symbol")) + "  "
		timeframeText := "  " + strings.ToUpper(
			validTimeframes[fake.IntBetween(0, len(validTimeframes)-1)].String(),
		) + "  "
		kindText := "  " + strings.ToUpper(
			validIndicatorKinds[fake.IntBetween(0, len(validIndicatorKinds)-1)].String(),
		) + "  "
		assetClassText := "  " + strings.ToUpper(
			validAssetClasses[fake.IntBetween(0, len(validAssetClasses)-1)].String(),
		) + "  "
		active := fake.Bool()
		kind, err := NewIndicatorKind(kindText)
		require.NoError(t, err)

		params := IndicatorParams{}
		switch kind {
		case IndicatorKindMovingAverage:
			params.Window = fake.IntBetween(1, 64)
		case IndicatorKindPeriodReturn:
			params.Lookback = fake.IntBetween(1, 64)
		}

		identity, err := NewAnalyticsSeriesIdentity(AnalyticsSeriesIdentityParams{
			Instrument: Instrument{
				Venue:      Venue(venueText),
				Symbol:     Symbol(symbolText),
				AssetClass: AssetClass(assetClassText),
				Active:     active,
			},
			Timeframe:  Timeframe(timeframeText),
			Kind:       IndicatorKind(kindText),
			Parameters: params,
			TimeRange: TimeRange{
				Start: start,
				End:   end,
			},
		})
		require.NoError(t, err)

		expectedVenue, err := NewVenue(venueText)
		require.NoError(t, err)
		expectedSymbol, err := NewSymbol(symbolText)
		require.NoError(t, err)
		expectedAssetClass, err := NewAssetClass(assetClassText)
		require.NoError(t, err)
		expectedInstrument, err := NewInstrument(InstrumentParams{
			Venue:      expectedVenue,
			Symbol:     expectedSymbol,
			AssetClass: expectedAssetClass,
			Active:     active,
		})
		require.NoError(t, err)
		expectedTimeframe, err := NewTimeframe(timeframeText)
		require.NoError(t, err)

		require.Equal(t, expectedInstrument, identity.Instrument)
		require.Equal(t, expectedTimeframe, identity.Timeframe)
		require.Equal(t, kind, identity.Kind)
		require.Equal(t, time.UTC, identity.TimeRange.Start.Location())
		require.Equal(t, time.UTC, identity.TimeRange.End.Location())
		require.Equal(t, start.UTC(), identity.TimeRange.Start)
		require.Equal(t, end.UTC(), identity.TimeRange.End)
		require.Equal(t, params, identity.Parameters)
	})

	t.Run("analytics identity rejects invalid embedded instrument asset class", func(t *testing.T) {
		t.Parallel()

		start := randomLocationTime()
		end := start.Add(time.Duration(fake.IntBetween(1, 180)) * time.Minute)

		_, err := NewAnalyticsSeriesIdentity(AnalyticsSeriesIdentityParams{
			Instrument: Instrument{
				Venue:      Venue(randomWord("venue")),
				Symbol:     Symbol(strings.ToUpper(randomWord("symbol"))),
				AssetClass: AssetClass("  " + randomWord("bad-asset-class") + "  "),
				Active:     fake.Bool(),
			},
			Timeframe:  Timeframe1m,
			Kind:       IndicatorKindMovingAverage,
			Parameters: IndicatorParams{Window: fake.IntBetween(1, 64)},
			TimeRange: TimeRange{
				Start: start,
				End:   end,
			},
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "asset class")
	})

	t.Run("indicator parameter validation is kind specific", func(t *testing.T) {
		t.Parallel()

		window := fake.IntBetween(1, 64)
		lookback := fake.IntBetween(1, 64)

		movingAverageParams, err := NewIndicatorParams(IndicatorKindMovingAverage, IndicatorParams{
			Window: window,
		})
		require.NoError(t, err)
		require.Equal(t, IndicatorParams{Window: window}, movingAverageParams)

		periodReturnParams, err := NewIndicatorParams(IndicatorKindPeriodReturn, IndicatorParams{
			Lookback: lookback,
		})
		require.NoError(t, err)
		require.Equal(t, IndicatorParams{Lookback: lookback}, periodReturnParams)

		_, err = NewIndicatorParams(IndicatorKindMovingAverage, IndicatorParams{})
		require.Error(t, err)
		_, err = NewIndicatorParams(IndicatorKindMovingAverage, IndicatorParams{
			Window:   window,
			Lookback: 1,
		})
		require.Error(t, err)
		_, err = NewIndicatorParams(IndicatorKindPeriodReturn, IndicatorParams{})
		require.Error(t, err)
		_, err = NewIndicatorParams(IndicatorKindPeriodReturn, IndicatorParams{
			Window:   1,
			Lookback: lookback,
		})
		require.Error(t, err)
	})

	t.Run("analytics points canonicalize UTC value ranges and times", func(t *testing.T) {
		t.Parallel()

		start := randomLocationTime()
		end := start.Add(time.Duration(fake.IntBetween(1, 180)) * time.Minute)
		pointTime := end.Add(time.Duration(fake.IntBetween(0, 10)) * time.Minute)
		provenance := randomProvenance(t)
		quality := validQualities[fake.IntBetween(0, len(validQualities)-1)]

		point, err := NewAnalyticsPoint(AnalyticsPointParams{
			Time: pointTime,
			ValueRange: AnalyticsValueRange{
				Start: start,
				End:   end,
			},
			Value:                fake.Float64(4, 1, 99999),
			Quality:              quality,
			SourceReplayIdentity: randomReplayIdentity(),
			SourceProvenance:     provenance,
		})
		require.NoError(t, err)

		require.Equal(t, pointTime.UTC(), point.Time.Time())
		require.Equal(t, time.UTC, point.Time.Time().Location())
		require.Equal(t, start.UTC(), point.ValueRange.Start)
		require.Equal(t, end.UTC(), point.ValueRange.End)
		require.Equal(t, time.UTC, point.ValueRange.Start.Location())
		require.Equal(t, time.UTC, point.ValueRange.End.Location())
		require.NotZero(t, point.SourceReplayIdentity)
		require.Equal(t, provenance.Source, point.SourceProvenance.Source)
		require.Equal(t, strings.TrimSpace(provenance.RecordID), point.SourceProvenance.RecordID)
	})

	t.Run("analytics points reject whitespace-only provenance source after trimming", func(t *testing.T) {
		t.Parallel()

		start := randomLocationTime()
		end := start.Add(time.Duration(fake.IntBetween(1, 180)) * time.Minute)
		pointTime := end.Add(time.Duration(fake.IntBetween(0, 10)) * time.Minute)

		_, err := NewAnalyticsPoint(AnalyticsPointParams{
			Time:                 pointTime,
			ValueRange:           AnalyticsValueRange{Start: start, End: end},
			Value:                fake.Float64(4, 1, 99999),
			Quality:              validQualities[fake.IntBetween(0, len(validQualities)-1)],
			SourceReplayIdentity: randomReplayIdentity(),
			SourceProvenance: SourceProvenance{
				Source:   " \t\n ",
				RecordID: randomWord("record"),
			},
		})
		require.Error(t, err)
	})

	t.Run("analytics series requires point ordering by time then source identity", func(t *testing.T) {
		t.Parallel()

		start := randomLocationTime()
		end := start.Add(time.Duration(fake.IntBetween(1, 180)) * time.Minute)
		window := fake.IntBetween(1, 64)
		identity, err := NewAnalyticsSeriesIdentity(AnalyticsSeriesIdentityParams{
			Instrument: randomInstrument(t),
			Timeframe:  validTimeframes[fake.IntBetween(0, len(validTimeframes)-1)],
			Kind:       IndicatorKindMovingAverage,
			Parameters: IndicatorParams{Window: window},
			TimeRange: TimeRange{
				Start: start,
				End:   end,
			},
		})
		require.NoError(t, err)

		sharedPointTime := end.Add(time.Duration(fake.IntBetween(1, 10)) * time.Minute)
		valueRange, err := NewAnalyticsValueRange(start, end)
		require.NoError(t, err)

		firstPoint, err := NewAnalyticsPoint(AnalyticsPointParams{
			Time:                 sharedPointTime,
			ValueRange:           valueRange,
			Value:                fake.Float64(4, 1, 99999),
			Quality:              DataQualityValidated,
			SourceReplayIdentity: randomReplayIdentity(),
			SourceProvenance: SourceProvenance{
				Source:   randomWord("source-z"),
				RecordID: "z-" + randomWord("record"),
			},
		})
		require.NoError(t, err)

		secondReplayIdentity := firstPoint.SourceReplayIdentity + uint64(fake.IntBetween(1, 100))
		secondPoint, err := NewAnalyticsPoint(AnalyticsPointParams{
			Time:                 sharedPointTime,
			ValueRange:           valueRange,
			Value:                fake.Float64(4, 1, 99999),
			Quality:              DataQualityValidated,
			SourceReplayIdentity: secondReplayIdentity,
			SourceProvenance: SourceProvenance{
				Source:   randomWord("source-a"),
				RecordID: "a-" + randomWord("record"),
			},
		})
		require.NoError(t, err)

		series, err := NewAnalyticsSeries(AnalyticsSeriesParams{
			Identity: identity,
			Points:   []AnalyticsPoint{firstPoint, secondPoint},
		})
		require.NoError(t, err)
		require.Equal(t, []AnalyticsPoint{firstPoint, secondPoint}, series.Points)

		_, err = NewAnalyticsSeries(AnalyticsSeriesParams{
			Identity: identity,
			Points:   []AnalyticsPoint{secondPoint, firstPoint},
		})
		require.Error(t, err)
	})

	t.Run("analytics points accept canonical quality values", func(t *testing.T) {
		t.Parallel()

		start := randomLocationTime()
		end := start.Add(time.Duration(fake.IntBetween(1, 180)) * time.Minute)
		pointTime := end.Add(time.Duration(fake.IntBetween(1, 10)) * time.Minute)
		provenance := randomProvenance(t)

		for _, quality := range validQualities {
			t.Run(quality.String(), func(t *testing.T) {
				t.Parallel()

				point, err := NewAnalyticsPoint(AnalyticsPointParams{
					Time: pointTime,
					ValueRange: AnalyticsValueRange{
						Start: start,
						End:   end,
					},
					Value:                fake.Float64(4, 1, 99999),
					Quality:              quality,
					SourceReplayIdentity: randomReplayIdentity(),
					SourceProvenance:     provenance,
				})
				require.NoError(t, err)
				require.Equal(t, quality, point.Quality)
			})
		}

		_, err := NewAnalyticsPoint(AnalyticsPointParams{
			Time: pointTime,
			ValueRange: AnalyticsValueRange{
				Start: start,
				End:   end,
			},
			Value: fake.Float64(4, 1, 99999),
			Quality: DataQuality(
				randomWord("bad-quality"),
			),
			SourceReplayIdentity: randomReplayIdentity(),
			SourceProvenance:     provenance,
		})
		require.Error(t, err)
	})

	t.Run("analytics points require source replay identity", func(t *testing.T) {
		t.Parallel()

		start := randomLocationTime()
		end := start.Add(time.Duration(fake.IntBetween(1, 180)) * time.Minute)
		pointTime := end.Add(time.Duration(fake.IntBetween(1, 10)) * time.Minute)

		_, err := NewAnalyticsPoint(AnalyticsPointParams{
			Time:       pointTime,
			ValueRange: AnalyticsValueRange{Start: start, End: end},
			Value:      fake.Float64(4, 1, 99999),
			Quality:    DataQualityValidated,
			SourceProvenance: SourceProvenance{
				Source:   randomWord("source"),
				RecordID: randomWord("record"),
			},
		})
		require.Error(t, err)
	})

	t.Run("strategy identity canonicalizes embedded instrument and kind", func(t *testing.T) {
		t.Parallel()

		venueText := "  " + randomWord("venue") + "  "
		symbolText := "  " + strings.ToUpper(randomWord("symbol")) + "  "
		assetClassText := "  " + strings.ToUpper(
			validAssetClasses[fake.IntBetween(0, len(validAssetClasses)-1)].String(),
		) + "  "
		timeframeText := "  " + strings.ToUpper(
			validTimeframes[fake.IntBetween(0, len(validTimeframes)-1)].String(),
		) + "  "
		kindText := "  " + strings.ToUpper(
			validStrategyKinds[fake.IntBetween(0, len(validStrategyKinds)-1)].String(),
		) + "  "
		active := fake.Bool()

		identity, err := NewStrategyIdentity(StrategyIdentityParams{
			Instrument: Instrument{
				Venue:      Venue(venueText),
				Symbol:     Symbol(symbolText),
				AssetClass: AssetClass(assetClassText),
				Active:     active,
			},
			Timeframe: Timeframe(timeframeText),
			Kind:      StrategyKind(kindText),
		})
		require.NoError(t, err)

		expectedVenue, err := NewVenue(venueText)
		require.NoError(t, err)
		expectedSymbol, err := NewSymbol(symbolText)
		require.NoError(t, err)
		expectedAssetClass, err := NewAssetClass(assetClassText)
		require.NoError(t, err)
		expectedInstrument, err := NewInstrument(InstrumentParams{
			Venue:      expectedVenue,
			Symbol:     expectedSymbol,
			AssetClass: expectedAssetClass,
			Active:     active,
		})
		require.NoError(t, err)
		expectedTimeframe, err := NewTimeframe(timeframeText)
		require.NoError(t, err)
		expectedKind, err := NewStrategyKind(kindText)
		require.NoError(t, err)

		require.Equal(t, expectedInstrument, identity.Instrument)
		require.Equal(t, expectedTimeframe, identity.Timeframe)
		require.Equal(t, expectedKind, identity.Kind)
	})

	t.Run("strategy identity rejects invalid kind", func(t *testing.T) {
		t.Parallel()

		_, err := NewStrategyIdentity(StrategyIdentityParams{
			Instrument: randomInstrument(t),
			Timeframe:  validTimeframes[fake.IntBetween(0, len(validTimeframes)-1)],
			Kind:       StrategyKind(randomWord("bad-strategy-kind")),
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "strategy kind")
	})

	t.Run("candidate actions canonicalize UTC decision time and input range", func(t *testing.T) {
		t.Parallel()

		decisionTime := randomLocationTime()
		inputStart := randomLocationTime()
		inputEnd := inputStart.Add(time.Duration(fake.IntBetween(1, 180)) * time.Minute)
		identity, err := NewStrategyIdentity(StrategyIdentityParams{
			Instrument: randomInstrument(t),
			Timeframe:  validTimeframes[fake.IntBetween(0, len(validTimeframes)-1)],
			Kind:       validStrategyKinds[fake.IntBetween(0, len(validStrategyKinds)-1)],
		})
		require.NoError(t, err)

		action, err := NewCandidateAction(CandidateActionParams{
			Strategy:     identity,
			Kind:         validCandidateActionKinds[fake.IntBetween(0, len(validCandidateActionKinds)-1)],
			DecisionTime: decisionTime,
			InputRange: TimeRange{
				Start: inputStart,
				End:   inputEnd,
			},
			Quality: validQualities[fake.IntBetween(0, len(validQualities)-1)],
		})
		require.NoError(t, err)

		require.Equal(t, decisionTime.UTC(), action.DecisionTime.Time())
		require.Equal(t, time.UTC, action.DecisionTime.Time().Location())
		require.Equal(t, inputStart.UTC(), action.InputRange.Start)
		require.Equal(t, inputEnd.UTC(), action.InputRange.End)
		require.Equal(t, time.UTC, action.InputRange.Start.Location())
		require.Equal(t, time.UTC, action.InputRange.End.Location())
		require.Equal(t, identity, action.Strategy)
	})

	t.Run("candidate actions reject invalid input range", func(t *testing.T) {
		t.Parallel()

		decisionTime := randomLocationTime()
		inputStart := randomLocationTime()
		identity, err := NewStrategyIdentity(StrategyIdentityParams{
			Instrument: randomInstrument(t),
			Timeframe:  validTimeframes[fake.IntBetween(0, len(validTimeframes)-1)],
			Kind:       validStrategyKinds[fake.IntBetween(0, len(validStrategyKinds)-1)],
		})
		require.NoError(t, err)

		_, err = NewCandidateAction(CandidateActionParams{
			Strategy:     identity,
			Kind:         validCandidateActionKinds[fake.IntBetween(0, len(validCandidateActionKinds)-1)],
			DecisionTime: decisionTime,
			InputRange: TimeRange{
				Start: inputStart,
				End:   inputStart,
			},
			Quality: validQualities[fake.IntBetween(0, len(validQualities)-1)],
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "input range")
	})

	t.Run("governor decisions canonicalize UTC time and retain candidate actions", func(t *testing.T) {
		t.Parallel()

		candidateAction := randomCandidateAction(t)
		decisionTime := randomLocationTime()
		status := validGovernorDecisionStatuses[fake.IntBetween(0, len(validGovernorDecisionStatuses)-1)]
		reason := validGovernorDecisionReasons[fake.IntBetween(0, len(validGovernorDecisionReasons)-1)]

		decision, err := NewGovernorDecision(GovernorDecisionParams{
			CandidateAction: candidateAction,
			Status:          status,
			Reason:          reason,
			DecisionTime:    decisionTime,
		})
		require.NoError(t, err)

		expected := GovernorDecision{
			CandidateAction: candidateAction,
			Status:          status,
			Reason:          reason,
			DecisionTime:    GovernorDecisionTime(decisionTime.UTC()),
		}
		require.Equal(t, expected, decision)
		require.Equal(t, time.UTC, decision.DecisionTime.Time().Location())
	})

	t.Run("governor decisions reject invalid values", func(t *testing.T) {
		t.Parallel()

		candidateAction := randomCandidateAction(t)

		_, err := NewGovernorDecision(GovernorDecisionParams{
			CandidateAction: candidateAction,
			Status:          GovernorDecisionStatus(randomWord("bad-status")),
			Reason:          GovernorDecisionReasonEligible,
			DecisionTime:    randomLocationTime(),
		})
		require.Error(t, err)

		_, err = NewGovernorDecision(GovernorDecisionParams{
			CandidateAction: candidateAction,
			Status:          GovernorDecisionStatusApproved,
			Reason:          GovernorDecisionReason(randomWord("bad-reason")),
			DecisionTime:    randomLocationTime(),
		})
		require.Error(t, err)

		_, err = NewGovernorDecision(GovernorDecisionParams{
			Status:       GovernorDecisionStatusApproved,
			Reason:       GovernorDecisionReasonEligible,
			DecisionTime: randomLocationTime(),
		})
		require.Error(t, err)

		_, err = NewGovernorDecision(GovernorDecisionParams{
			CandidateAction: candidateAction,
			Status:          GovernorDecisionStatusApproved,
			Reason:          GovernorDecisionReasonEligible,
		})
		require.Error(t, err)
	})

	t.Run("execution records canonicalize identifiers statuses quantities prices and UTC times", func(t *testing.T) {
		t.Parallel()

		approvedDecision := randomApprovedDecision(t)
		commandTime := randomLocationTime()
		commandIDText := "  " + randomWord("command") + "  "
		commandStatus := validExecutionCommandStatuses[fake.IntBetween(0, len(validExecutionCommandStatuses)-1)]
		quantity := fake.Float64(4, 1, 99999)

		command, err := NewExecutionCommand(ExecutionCommandParams{
			CommandID:        commandIDText,
			ApprovedDecision: approvedDecision,
			Status:           commandStatus,
			Quantity:         quantity,
			EventTime:        commandTime,
		})
		require.NoError(t, err)

		orderTime := randomLocationTime()
		orderIDText := "  " + randomWord("order") + "  "
		clientOrderIDText := "  " + randomWord("client-order") + "  "
		venue, err := NewVenue(randomWord("venue"))
		require.NoError(t, err)
		orderStatus := validExecutionOrderStatuses[fake.IntBetween(0, len(validExecutionOrderStatuses)-1)]

		order, err := NewExecutionOrder(ExecutionOrderParams{
			OrderID:       orderIDText,
			Command:       command,
			Venue:         venue,
			ClientOrderID: clientOrderIDText,
			Status:        orderStatus,
			Quantity:      quantity,
			EventTime:     orderTime,
		})
		require.NoError(t, err)

		fillTime := randomLocationTime()
		fillIDText := "  " + randomWord("fill") + "  "
		fillQuantity := fake.Float64(4, 1, 9999)
		price := fake.Float64(4, 1, 99999)

		fill, err := NewExecutionFill(ExecutionFillParams{
			FillID:    fillIDText,
			Order:     order,
			Quantity:  fillQuantity,
			Price:     price,
			EventTime: fillTime,
		})
		require.NoError(t, err)

		reconciliationTime := randomLocationTime()
		reconciliation, err := NewExecutionReconciliation(ExecutionReconciliationParams{
			Order:          order,
			Fills:          []ExecutionFill{fill},
			Status:         ExecutionOrderStatusFilled,
			FilledQuantity: fillQuantity,
			EventTime:      reconciliationTime,
		})
		require.NoError(t, err)

		require.Equal(t, ExecutionCommand{
			CommandID:        ExecutionCommandID(strings.TrimSpace(commandIDText)),
			ApprovedDecision: approvedDecision,
			Status:           commandStatus,
			Quantity:         quantity,
			EventTime:        ExecutionEventTime(commandTime.UTC()),
		}, command)
		require.Equal(t, approvedDecision, command.ApprovedDecision)
		require.Equal(t, time.UTC, command.EventTime.Time().Location())

		require.Equal(t, ExecutionOrder{
			OrderID:       ExecutionOrderID(strings.TrimSpace(orderIDText)),
			Command:       command,
			Venue:         venue,
			ClientOrderID: strings.TrimSpace(clientOrderIDText),
			Status:        orderStatus,
			Quantity:      quantity,
			EventTime:     ExecutionEventTime(orderTime.UTC()),
		}, order)
		require.Equal(t, time.UTC, order.EventTime.Time().Location())

		require.Equal(t, ExecutionFill{
			FillID:    ExecutionFillID(strings.TrimSpace(fillIDText)),
			Order:     order,
			Quantity:  fillQuantity,
			Price:     price,
			EventTime: ExecutionEventTime(fillTime.UTC()),
		}, fill)
		require.Equal(t, command, fill.Order.Command)
		require.Equal(t, time.UTC, fill.EventTime.Time().Location())

		require.Equal(t, ExecutionReconciliation{
			Order:          order,
			Fills:          []ExecutionFill{fill},
			Status:         ExecutionOrderStatusFilled,
			FilledQuantity: fillQuantity,
			EventTime:      ExecutionEventTime(reconciliationTime.UTC()),
		}, reconciliation)
		require.Equal(t, time.UTC, reconciliation.EventTime.Time().Location())
	})

	t.Run("execution records reject invalid values", func(t *testing.T) {
		t.Parallel()

		approvedDecision := randomApprovedDecision(t)
		quantity := fake.Float64(4, 1, 99999)

		_, err := NewExecutionCommand(ExecutionCommandParams{
			CommandID: randomWord("command"),
			Status:    ExecutionCommandStatusCreated,
			Quantity:  quantity,
			EventTime: randomLocationTime(),
		})
		require.Error(t, err)

		_, err = NewExecutionCommand(ExecutionCommandParams{
			CommandID:        randomWord("command"),
			ApprovedDecision: approvedDecision,
			Status:           ExecutionCommandStatus(randomWord("bad-command-status")),
			Quantity:         quantity,
			EventTime:        randomLocationTime(),
		})
		require.Error(t, err)

		rejectedDecision, err := NewGovernorDecision(GovernorDecisionParams{
			CandidateAction: randomCandidateAction(t),
			Status:          GovernorDecisionStatusRejected,
			Reason:          GovernorDecisionReasonDisallowedActionKind,
			DecisionTime:    randomLocationTime(),
		})
		require.NoError(t, err)

		_, err = NewExecutionCommand(ExecutionCommandParams{
			CommandID:        randomWord("command"),
			ApprovedDecision: rejectedDecision,
			Status:           ExecutionCommandStatusCreated,
			Quantity:         quantity,
			EventTime:        randomLocationTime(),
		})
		require.ErrorContains(t, err, "execution approved decision must be approved")

		_, err = NewExecutionCommand(ExecutionCommandParams{
			CommandID:        randomWord("command"),
			ApprovedDecision: approvedDecision,
			Status:           ExecutionCommandStatusCreated,
			Quantity:         math.NaN(),
			EventTime:        randomLocationTime(),
		})
		require.ErrorContains(t, err, "execution command quantity must be finite")

		_, err = NewExecutionCommand(ExecutionCommandParams{
			CommandID:        randomWord("command"),
			ApprovedDecision: approvedDecision,
			Status:           ExecutionCommandStatusCreated,
			Quantity:         math.Inf(1),
			EventTime:        randomLocationTime(),
		})
		require.ErrorContains(t, err, "execution command quantity must be finite")

		command, err := NewExecutionCommand(ExecutionCommandParams{
			CommandID:        randomWord("command"),
			ApprovedDecision: approvedDecision,
			Status:           ExecutionCommandStatusCreated,
			Quantity:         quantity,
			EventTime:        randomLocationTime(),
		})
		require.NoError(t, err)

		_, err = NewExecutionOrder(ExecutionOrderParams{
			OrderID:       randomWord("order"),
			Command:       command,
			ClientOrderID: randomWord("client-order"),
			Status:        ExecutionOrderStatusOpen,
			Quantity:      quantity,
			EventTime:     randomLocationTime(),
		})
		require.Error(t, err)

		venue, err := NewVenue(randomWord("venue"))
		require.NoError(t, err)
		order, err := NewExecutionOrder(ExecutionOrderParams{
			OrderID:       randomWord("order"),
			Command:       command,
			Venue:         venue,
			ClientOrderID: randomWord("client-order"),
			Status:        ExecutionOrderStatusOpen,
			Quantity:      quantity,
			EventTime:     randomLocationTime(),
		})
		require.NoError(t, err)

		_, err = NewExecutionOrder(ExecutionOrderParams{
			OrderID:       randomWord("order"),
			Command:       command,
			Venue:         venue,
			ClientOrderID: randomWord("client-order"),
			Status:        ExecutionOrderStatusOpen,
			Quantity:      math.NaN(),
			EventTime:     randomLocationTime(),
		})
		require.ErrorContains(t, err, "execution order quantity must be finite")

		_, err = NewExecutionFill(ExecutionFillParams{
			FillID:    randomWord("fill"),
			Order:     order,
			Quantity:  0,
			Price:     fake.Float64(4, 1, 99999),
			EventTime: randomLocationTime(),
		})
		require.Error(t, err)

		_, err = NewExecutionFill(ExecutionFillParams{
			FillID:    randomWord("fill"),
			Order:     order,
			Quantity:  math.Inf(1),
			Price:     fake.Float64(4, 1, 99999),
			EventTime: randomLocationTime(),
		})
		require.ErrorContains(t, err, "execution fill quantity must be finite")

		_, err = NewExecutionFill(ExecutionFillParams{
			FillID:    randomWord("fill"),
			Order:     order,
			Quantity:  fake.Float64(4, 1, 9999),
			Price:     math.NaN(),
			EventTime: randomLocationTime(),
		})
		require.ErrorContains(t, err, "execution fill price must be finite")

		fill, err := NewExecutionFill(ExecutionFillParams{
			FillID:    randomWord("fill"),
			Order:     order,
			Quantity:  fake.Float64(4, 1, 9999),
			Price:     fake.Float64(4, 1, 99999),
			EventTime: randomLocationTime(),
		})
		require.NoError(t, err)

		_, err = NewExecutionReconciliation(ExecutionReconciliationParams{
			Order:          order,
			Fills:          []ExecutionFill{fill},
			Status:         ExecutionOrderStatus(randomWord("bad-order-status")),
			FilledQuantity: fill.Quantity,
			EventTime:      randomLocationTime(),
		})
		require.Error(t, err)

		_, err = NewExecutionReconciliation(ExecutionReconciliationParams{
			Order:          order,
			Fills:          []ExecutionFill{fill},
			Status:         ExecutionOrderStatusFilled,
			FilledQuantity: math.NaN(),
			EventTime:      randomLocationTime(),
		})
		require.ErrorContains(t, err, "execution reconciliation filled quantity must be finite")
	})

	t.Run("candidate actions keep existing analytics and strategy contracts", func(t *testing.T) {
		t.Parallel()

		action := randomCandidateAction(t)
		decision, err := NewGovernorDecision(GovernorDecisionParams{
			CandidateAction: action,
			Status:          GovernorDecisionStatusApproved,
			Reason:          GovernorDecisionReasonEligible,
			DecisionTime:    randomLocationTime(),
		})
		require.NoError(t, err)

		require.Contains(t, validCandidateActionKinds, action.Kind)
		require.Contains(t, validQualities, action.Quality)
		require.Equal(t, action.Strategy, decision.CandidateAction.Strategy)
		require.Equal(t, action.Kind, decision.CandidateAction.Kind)
		require.Equal(t, action.Quality, decision.CandidateAction.Quality)
		require.True(t, action.InputRange.End.After(action.InputRange.Start))
	})
}
