package strategy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gemyago/signal-foundry/runtime/domain"
)

type canonicalStrategyDSLV0 struct {
	Instrument domain.Instrument
	Timeframe  domain.Timeframe
	Kind       domain.StrategyKind
	Parameters MovingAverageCrossoverParams
}

type dslV0Payload struct {
	Kind       string          `json:"kind"`
	Instrument dslV0Instrument `json:"instrument"`
	Timeframe  string          `json:"timeframe"`
	Parameters dslV0Parameters `json:"parameters"`
}

type dslV0Instrument struct {
	Venue      string `json:"venue"`
	Symbol     string `json:"symbol"`
	AssetClass string `json:"assetClass"`
	Active     bool   `json:"active"`
}

type dslV0Parameters struct {
	FastWindow int `json:"fastWindow"`
	SlowWindow int `json:"slowWindow"`
}

func parseDSLV0(raw []byte) (canonicalStrategyDSLV0, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return canonicalStrategyDSLV0{}, validationError("strategy DSL payload is required")
	}

	var payload dslV0Payload
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		return canonicalStrategyDSLV0{}, validationError(
			fmt.Sprintf("decode strategy DSL v0 payload: %s", err.Error()),
		)
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return canonicalStrategyDSLV0{}, validationError(
			"decode strategy DSL v0 payload: unexpected trailing content",
		)
	}

	canonicalKind, err := domain.NewStrategyKind(payload.Kind)
	if err != nil {
		return canonicalStrategyDSLV0{}, validationError(err.Error())
	}

	canonicalStrategy, err := domain.NewStrategyIdentity(domain.StrategyIdentityParams{
		Instrument: domain.Instrument{
			Venue:      domain.Venue(payload.Instrument.Venue),
			Symbol:     domain.Symbol(payload.Instrument.Symbol),
			AssetClass: domain.AssetClass(payload.Instrument.AssetClass),
			Active:     payload.Instrument.Active,
		},
		Timeframe: domain.Timeframe(payload.Timeframe),
		Kind:      canonicalKind,
	})
	if err != nil {
		return canonicalStrategyDSLV0{}, validationError(err.Error())
	}

	canonicalParams, err := NewMovingAverageCrossoverParams(MovingAverageCrossoverParams{
		FastWindow: payload.Parameters.FastWindow,
		SlowWindow: payload.Parameters.SlowWindow,
	})
	if err != nil {
		return canonicalStrategyDSLV0{}, validationError(err.Error())
	}

	return canonicalStrategyDSLV0{
		Instrument: canonicalStrategy.Instrument,
		Timeframe:  canonicalStrategy.Timeframe,
		Kind:       canonicalStrategy.Kind,
		Parameters: canonicalParams,
	}, nil
}

func mapDSLV0ToEvaluateRequest(
	canonicalDSL canonicalStrategyDSLV0,
	evaluationRange domain.TimeRange,
) (EvaluateRequest, error) {
	canonicalRange, err := domain.NewTimeRange(evaluationRange.Start, evaluationRange.End)
	if err != nil {
		return EvaluateRequest{}, validationError(
			fmt.Sprintf("strategy evaluation time range: %s", err.Error()),
		)
	}

	return EvaluateRequest{
		Instrument:   canonicalDSL.Instrument,
		Timeframe:    canonicalDSL.Timeframe,
		TimeRange:    canonicalRange,
		StrategyKind: canonicalDSL.Kind,
		Parameters:   canonicalDSL.Parameters,
	}, nil
}
