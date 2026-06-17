package strategyassistant

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	app "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	"github.com/gemyago/signal-foundry/runtime/agent"
	rtdata "github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
)

const (
	strategyAssistantSupportedDataVenue = "hyperliquid-perps"
	defaultAvailabilityToolLimit        = 20
	maxAvailabilityToolLimit            = 100
	defaultCandleEvidenceToolLimit      = 20
	maxCandleEvidenceToolLimit          = 100
	maxCandlesToolRows                  = 500
	maxCandleIntervals                  = 10_000
	defaultSelectionReason              = "Recommended first browse scope from current availability."
	availabilityPageSize                = 200
)

func handleListCandleAvailabilityTool(
	ctx *agent.ToolContext,
	deps RegisterDeps,
	input ListCandleAvailabilityRequest,
) (ListCandleAvailabilityResponse, error) {
	if deps.DataRead == nil {
		errResult, nextStepHint := placeholderToolErrorResult()
		return ListCandleAvailabilityResponse{
			Items:        []CandleAvailabilityRow{},
			Error:        errResult,
			NextStepHint: nextStepHint,
		}, nil
	}

	limit, err := normalizeLimit(input.Limit, defaultAvailabilityToolLimit, maxAvailabilityToolLimit)
	if err != nil {
		return ListCandleAvailabilityResponse{Items: []CandleAvailabilityRow{}, Error: toolErrorFrom(err)}, nil
	}

	offset, err := normalizeOffset(input.Offset)
	if err != nil {
		return ListCandleAvailabilityResponse{Items: []CandleAvailabilityRow{}, Error: toolErrorFrom(err)}, nil
	}

	venue, symbol, assetClass, err := validateAvailabilityFilters(input)
	if err != nil {
		return ListCandleAvailabilityResponse{Items: []CandleAvailabilityRow{}, Error: toolErrorFrom(err)}, nil
	}

	items, defaultSelection, hasMore, err := collectAvailabilityWindow(
		toolContextContext(ctx),
		deps.DataRead,
		venue,
		symbol,
		assetClass,
		offset,
		limit,
	)
	if err != nil {
		return ListCandleAvailabilityResponse{
			Items: []CandleAvailabilityRow{},
			Error: toolErrorFrom(mapDataToolError(err, "candle-availability")),
		}, nil
	}

	rows := make([]CandleAvailabilityRow, len(items))
	for i := range items {
		rows[i] = mapCandleAvailabilityRow(items[i], defaultSelection)
	}

	response := ListCandleAvailabilityResponse{Items: rows}
	if hasMore {
		nextOffset := offset + len(rows)
		response.Truncation = NewTruncation(limit, len(rows), nil, strconv.Itoa(nextOffset), nil)
		response.NextStepHint = fmt.Sprintf("Retry with offset=%d to continue browsing availability.", nextOffset)
	}

	return response, nil
}

func handleGetCandlesTool(
	ctx *agent.ToolContext,
	deps RegisterDeps,
	input GetCandlesRequest,
) (GetCandlesResponse, error) {
	if deps.DataRead == nil {
		errResult, nextStepHint := placeholderToolErrorResult()
		return GetCandlesResponse{Candles: []CandleRow{}, Error: errResult, NextStepHint: nextStepHint}, nil
	}

	instrument, timeframe, timeRange, err := validateExactCandleScope(input)
	if err != nil {
		return GetCandlesResponse{Candles: []CandleRow{}, Error: toolErrorFrom(err)}, nil
	}

	replayed, err := deps.DataRead.ReplayCandles(toolContextContext(ctx), instrument, timeframe, timeRange)
	if err != nil {
		return GetCandlesResponse{
			Candles: []CandleRow{},
			Error:   toolErrorFrom(mapDataToolError(err, input.Symbol)),
		}, nil
	}

	sort.SliceStable(replayed, func(i, j int) bool {
		left := replayed[i]
		right := replayed[j]
		if !left.Candle.TimeRange.Start.Equal(right.Candle.TimeRange.Start) {
			return left.Candle.TimeRange.Start.Before(right.Candle.TimeRange.Start)
		}
		if !left.Candle.TimeRange.End.Equal(right.Candle.TimeRange.End) {
			return left.Candle.TimeRange.End.Before(right.Candle.TimeRange.End)
		}
		return left.Identity < right.Identity
	})

	hasMore := len(replayed) > maxCandlesToolRows
	selected := replayed
	if hasMore {
		selected = replayed[:maxCandlesToolRows]
	}

	rows := make([]CandleRow, len(selected))
	for i := range selected {
		rows[i] = mapCandleRow(selected[i])
	}

	response := GetCandlesResponse{Candles: rows}
	if hasMore {
		nextRangeStart := rows[len(rows)-1].CloseTime
		response.Truncation = NewTruncation(maxCandlesToolRows, len(rows), nil, "", &nextRangeStart)
		response.NextStepHint = fmt.Sprintf(
			"Retry with start=%s and the same scope to continue reading candles.",
			nextRangeStart.UTC().Format(time.RFC3339),
		)
	}

	return response, nil
}

func handleGetCandleEvidenceTool(
	ctx *agent.ToolContext,
	deps RegisterDeps,
	input GetCandleEvidenceRequest,
) (GetCandleEvidenceResponse, error) {
	if deps.DataLineage == nil {
		errResult, nextStepHint := placeholderToolErrorResult()
		return GetCandleEvidenceResponse{
			Evidence:     []CandleEvidenceRow{},
			Error:        errResult,
			NextStepHint: nextStepHint,
		}, nil
	}

	limit, err := normalizeLimit(input.Limit, defaultCandleEvidenceToolLimit, maxCandleEvidenceToolLimit)
	if err != nil {
		return GetCandleEvidenceResponse{Evidence: []CandleEvidenceRow{}, Error: toolErrorFrom(err)}, nil
	}

	offset, err := normalizeOffset(input.Offset)
	if err != nil {
		return GetCandleEvidenceResponse{Evidence: []CandleEvidenceRow{}, Error: toolErrorFrom(err)}, nil
	}

	query, err := buildCandleEvidenceQuery(input)
	if err != nil {
		return GetCandleEvidenceResponse{Evidence: []CandleEvidenceRow{}, Error: toolErrorFrom(err)}, nil
	}

	items, err := deps.DataLineage.ListCandleLinkedRawPayloadMetadata(toolContextContext(ctx), query)
	if err != nil {
		return GetCandleEvidenceResponse{
			Evidence: []CandleEvidenceRow{},
			Error:    toolErrorFrom(mapDataToolError(err, input.ProvenanceID)),
		}, nil
	}

	sort.SliceStable(items, func(i, j int) bool {
		left := metadataCapturedAt(items[i])
		right := metadataCapturedAt(items[j])
		if !left.Equal(right) {
			return left.Before(right)
		}
		return items[i].ID < items[j].ID
	})

	start := min(offset, len(items))
	end := min(start+limit, len(items))
	hasMore := end < len(items)

	rows := make([]CandleEvidenceRow, end-start)
	for i := start; i < end; i++ {
		rows[i-start] = mapCandleEvidenceRow(items[i])
	}

	response := GetCandleEvidenceResponse{Evidence: rows}
	if hasMore {
		nextOffset := offset + len(rows)
		response.Truncation = NewTruncation(limit, len(rows), nil, strconv.Itoa(nextOffset), nil)
		response.NextStepHint = fmt.Sprintf("Retry with offset=%d to continue reading candle evidence.", nextOffset)
	}

	return response, nil
}

func validateAvailabilityFilters(
	input ListCandleAvailabilityRequest,
) (domain.Venue, domain.Symbol, domain.AssetClass, error) {
	var venue domain.Venue
	if canonicalVenue := strings.TrimSpace(input.Venue); canonicalVenue != "" {
		if domain.Venue(canonicalVenue) != domain.Venue(strategyAssistantSupportedDataVenue) {
			return "", "", "", app.NewErrInvalidInput("venue", "unsupported venue")
		}
		venue = domain.Venue(canonicalVenue)
	}

	var symbol domain.Symbol
	if canonicalSymbol := strings.TrimSpace(input.Symbol); canonicalSymbol != "" {
		symbol = domain.Symbol(canonicalSymbol)
	}

	var assetClass domain.AssetClass
	if strings.TrimSpace(input.AssetClass) != "" {
		canonicalAssetClass, err := domain.NewAssetClass(input.AssetClass)
		if err != nil {
			return "", "", "", app.NewErrInvalidInput("assetClass", err.Error())
		}
		assetClass = canonicalAssetClass
	}

	return venue, symbol, assetClass, nil
}

func collectAvailabilityWindow(
	ctx context.Context,
	readSvc candleReadService,
	venue domain.Venue,
	symbol domain.Symbol,
	assetClass domain.AssetClass,
	offset int,
	limit int,
) ([]rtdata.CandleAvailabilityItem, *rtdata.CandleAvailabilityDefaultSelection, bool, error) {
	selected := make([]rtdata.CandleAvailabilityItem, 0, limit)
	remainingSkip := offset
	cursor := ""
	var defaultSelection *rtdata.CandleAvailabilityDefaultSelection

	for {
		result, err := fetchAvailabilityPage(ctx, readSvc, venue, symbol, assetClass, cursor)
		if err != nil {
			return nil, nil, false, err
		}
		if cursor == "" && result.DefaultSelection != nil {
			cloned := *result.DefaultSelection
			defaultSelection = &cloned
		}

		pageSelected, nextSkip, hasMore, done := consumeAvailabilityPage(
			selected,
			result.Items,
			remainingSkip,
			limit,
			result.NextCursor != "",
		)
		selected = pageSelected
		remainingSkip = nextSkip
		if done {
			return selected, defaultSelection, hasMore, nil
		}
		cursor = result.NextCursor
	}
}

func fetchAvailabilityPage(
	ctx context.Context,
	readSvc candleReadService,
	venue domain.Venue,
	symbol domain.Symbol,
	assetClass domain.AssetClass,
	cursor string,
) (rtdata.CandleAvailabilityListResult, error) {
	query, err := rtdata.NewCandleAvailabilityListQuery(rtdata.CandleAvailabilityListQueryParams{
		Venue:      venue,
		Symbol:     symbol,
		AssetClass: assetClass,
		Limit:      availabilityPageSize,
		Cursor:     cursor,
	})
	if err != nil {
		return rtdata.CandleAvailabilityListResult{}, err
	}

	return readSvc.ListCandleAvailability(ctx, query)
}

func consumeAvailabilityPage(
	selected []rtdata.CandleAvailabilityItem,
	pageItems []rtdata.CandleAvailabilityItem,
	remainingSkip int,
	limit int,
	hasNextPage bool,
) ([]rtdata.CandleAvailabilityItem, int, bool, bool) {
	if remainingSkip >= len(pageItems) {
		remainingSkip -= len(pageItems)
		return selected, remainingSkip, false, !hasNextPage
	}

	for i := remainingSkip; i < len(pageItems); i++ {
		if len(selected) >= limit {
			return selected, 0, true, true
		}
		selected = append(selected, pageItems[i])
	}

	if len(selected) >= limit {
		return selected, 0, hasNextPage, true
	}

	return selected, 0, false, !hasNextPage
}

func mapCandleAvailabilityRow(
	item rtdata.CandleAvailabilityItem,
	defaultSelection *rtdata.CandleAvailabilityDefaultSelection,
) CandleAvailabilityRow {
	timeframes := make([]CandleAvailabilityTimeframeSummary, len(item.Timeframes))
	for i := range item.Timeframes {
		timeframes[i] = CandleAvailabilityTimeframeSummary{
			Timeframe: item.Timeframes[i].Timeframe.String(),
			Count:     int(item.Timeframes[i].Count),
			Start:     item.Timeframes[i].StartAt,
			End:       item.Timeframes[i].EndAt,
		}
	}
	sort.SliceStable(timeframes, func(i, j int) bool {
		left, leftErr := timeframeDuration(domain.Timeframe(timeframes[i].Timeframe))
		right, rightErr := timeframeDuration(domain.Timeframe(timeframes[j].Timeframe))
		if leftErr == nil && rightErr == nil && left != right {
			return left < right
		}
		return timeframes[i].Timeframe < timeframes[j].Timeframe
	})

	row := CandleAvailabilityRow{
		Venue:      item.Venue.String(),
		Symbol:     item.Symbol.String(),
		AssetClass: item.AssetClass.String(),
		Timeframes: timeframes,
	}
	if defaultSelection != nil && defaultSelection.Venue == item.Venue && defaultSelection.Symbol == item.Symbol &&
		defaultSelection.AssetClass == item.AssetClass {
		row.DefaultSelection = &CandleAvailabilityDefaultSelection{
			Timeframe: defaultSelection.Timeframe.String(),
			Reason:    defaultSelectionReason,
		}
	}

	return row
}

func validateExactCandleScope(
	input GetCandlesRequest,
) (domain.Instrument, domain.Timeframe, domain.TimeRange, error) {
	venue, err := domain.NewVenue(input.Venue)
	if err != nil {
		return domain.Instrument{}, "", domain.TimeRange{}, app.NewErrInvalidInput("venue", err.Error())
	}
	if venue != domain.Venue(strategyAssistantSupportedDataVenue) {
		return domain.Instrument{}, "", domain.TimeRange{}, app.NewErrInvalidInput("venue", "unsupported venue")
	}

	symbol, err := domain.NewSymbol(input.Symbol)
	if err != nil {
		return domain.Instrument{}, "", domain.TimeRange{}, app.NewErrInvalidInput("symbol", err.Error())
	}

	assetClass, err := domain.NewAssetClass(input.AssetClass)
	if err != nil {
		return domain.Instrument{}, "", domain.TimeRange{}, app.NewErrInvalidInput("assetClass", err.Error())
	}

	timeframe, err := domain.NewTimeframe(input.Timeframe)
	if err != nil {
		return domain.Instrument{}, "", domain.TimeRange{}, app.NewErrInvalidInput("timeframe", err.Error())
	}

	timeRange, err := domain.NewTimeRange(input.Start, input.End)
	if err != nil {
		return domain.Instrument{}, "", domain.TimeRange{}, app.NewErrInvalidInput("range", err.Error())
	}

	duration, _ := timeframeDuration(timeframe)
	if timeRange.End.Sub(timeRange.Start) > time.Duration(maxCandleIntervals)*duration {
		return domain.Instrument{}, "", domain.TimeRange{}, app.NewErrInvalidInput(
			"range",
			fmt.Sprintf("requested range exceeds %d candle intervals", maxCandleIntervals),
		)
	}

	instrument := domain.Instrument{
		Venue:      venue,
		Symbol:     symbol,
		AssetClass: assetClass,
		Active:     true,
	}

	return instrument, timeframe, timeRange, nil
}

func mapCandleRow(item rtdata.ReplayCandle) CandleRow {
	return CandleRow{
		CandleID:         strconv.FormatUint(item.Identity, 10),
		OpenTime:         item.Candle.TimeRange.Start,
		CloseTime:        item.Candle.TimeRange.End,
		Open:             item.Candle.Open,
		High:             item.Candle.High,
		Low:              item.Candle.Low,
		Close:            item.Candle.Close,
		Volume:           item.Candle.Volume,
		Quality:          item.Candle.Quality.String(),
		ProvenanceSource: item.Candle.Provenance.Source,
		ProvenanceID:     item.Candle.Provenance.RecordID,
	}
}

func buildCandleEvidenceQuery(input GetCandleEvidenceRequest) (rtdata.CandleLinkedRawPayloadsQuery, error) {
	venue, err := domain.NewVenue(input.Venue)
	if err != nil {
		return rtdata.CandleLinkedRawPayloadsQuery{}, app.NewErrInvalidInput("venue", err.Error())
	}
	if venue != domain.Venue(strategyAssistantSupportedDataVenue) {
		return rtdata.CandleLinkedRawPayloadsQuery{}, app.NewErrInvalidInput("venue", "unsupported venue")
	}

	symbol, err := domain.NewSymbol(input.Symbol)
	if err != nil {
		return rtdata.CandleLinkedRawPayloadsQuery{}, app.NewErrInvalidInput("symbol", err.Error())
	}

	assetClass, err := domain.NewAssetClass(input.AssetClass)
	if err != nil {
		return rtdata.CandleLinkedRawPayloadsQuery{}, app.NewErrInvalidInput("assetClass", err.Error())
	}

	timeframe, err := domain.NewTimeframe(input.Timeframe)
	if err != nil {
		return rtdata.CandleLinkedRawPayloadsQuery{}, app.NewErrInvalidInput("timeframe", err.Error())
	}
	duration, _ := timeframeDuration(timeframe)
	if input.OpenTime.IsZero() {
		return rtdata.CandleLinkedRawPayloadsQuery{}, app.NewErrInvalidInput("openTime", "time range start is required")
	}

	closeTime := input.OpenTime.UTC().Add(duration)
	query, err := rtdata.NewCandleLinkedRawPayloadsQuery(rtdata.CandleLinkedRawPayloadsQueryParams{
		Venue:              venue,
		Symbol:             symbol,
		AssetClass:         assetClass,
		Timeframe:          timeframe,
		StartAt:            input.OpenTime,
		EndAt:              closeTime,
		ProvenanceSource:   input.ProvenanceSource,
		ProvenanceIdentity: input.ProvenanceID,
	})
	if err != nil {
		return rtdata.CandleLinkedRawPayloadsQuery{}, mapCandleEvidenceValidationError(err)
	}

	return query, nil
}

func mapCandleEvidenceValidationError(err error) error {
	message := err.Error()
	switch {
	case strings.Contains(message, "venue"):
		return app.NewErrInvalidInput("venue", message)
	case strings.Contains(message, "symbol"):
		return app.NewErrInvalidInput("symbol", message)
	case strings.Contains(message, "asset class"):
		return app.NewErrInvalidInput("assetClass", message)
	case strings.Contains(message, "timeframe"):
		return app.NewErrInvalidInput("timeframe", message)
	case strings.Contains(message, "provenance source"):
		return app.NewErrInvalidInput("provenanceSource", message)
	case strings.Contains(message, "provenance identity"):
		return app.NewErrInvalidInput("provenanceId", message)
	default:
		return app.NewErrInvalidInput("request", message)
	}
}

func mapCandleEvidenceRow(item rtdata.RawPayloadMetadata) CandleEvidenceRow {
	return CandleEvidenceRow{
		RawPayloadID: item.ID,
		Venue:        item.Venue.String(),
		CapturedAt:   metadataCapturedAt(item),
		SourceType:   item.Source,
		Reference:    compactEvidenceReference(item),
	}
}

func compactEvidenceReference(item rtdata.RawPayloadMetadata) string {
	parts := make([]string, 0, 2)
	if trimmed := strings.TrimSpace(item.RequestType); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if trimmed := strings.TrimSpace(item.Endpoint); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if len(parts) == 0 {
		return item.ID
	}
	return strings.Join(parts, " ")
}

func metadataCapturedAt(item rtdata.RawPayloadMetadata) time.Time {
	if !item.ResponseAt.IsZero() {
		return item.ResponseAt
	}
	return item.ReceivedAt
}

func normalizeLimit(value int, defaultLimit int, maxLimit int) (int, error) {
	if value == 0 {
		return defaultLimit, nil
	}
	if value < 0 {
		return 0, app.NewErrInvalidInput("limit", "must be non-negative")
	}
	return min(value, maxLimit), nil
}

func normalizeOffset(value int) (int, error) {
	if value < 0 {
		return 0, app.NewErrInvalidInput("offset", "must be non-negative")
	}
	return value, nil
}

func timeframeDuration(timeframe domain.Timeframe) (time.Duration, error) {
	switch timeframe {
	case domain.Timeframe1m:
		return time.Minute, nil
	case domain.Timeframe5m:
		return 5 * time.Minute, nil
	case domain.Timeframe15m:
		return 15 * time.Minute, nil
	case domain.Timeframe1h:
		return time.Hour, nil
	case domain.Timeframe4h:
		return 4 * time.Hour, nil
	case domain.Timeframe1d:
		return 24 * time.Hour, nil
	default:
		return 0, errors.New("unsupported timeframe")
	}
}

func mapDataToolError(err error, resourceID string) error {
	switch {
	case errors.Is(err, rtdata.ErrValidation):
		return app.NewErrInvalidInput("request", err.Error())
	case errors.Is(err, rtdata.ErrInstrumentNotFound):
		return app.NewErrNotFound("instrument", resourceID)
	case errors.Is(err, rtdata.ErrRawPayloadNotFound):
		return app.NewErrNotFound("raw payload", resourceID)
	default:
		return err
	}
}

func toolErrorFrom(err error) *ToolError {
	toolErr, _ := resultMetaFromError(err, "")
	return toolErr
}

func toolContextContext(ctx *agent.ToolContext) context.Context {
	if ctx == nil || ctx.Context == nil {
		return context.Background()
	}
	return ctx.Context
}
