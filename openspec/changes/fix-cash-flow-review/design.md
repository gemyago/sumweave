## Context

The archived `add-cash-flow-timeseries-chart` change is canonicalized, but PR #21 has seven unresolved actionable comments in four correction groups. The current dashboard tests cover a partial series with activity, the controller maps only the timestamp-range sentinel, the SQL query uses PostgreSQL `ROUND`, and the monthly validator advances from a previously clipped boundary. The existing dashboard conversion multiplies `float64` values and uses Go `math.Round`.

This follow-up is limited to those review comments. Finance owns reporting rules and PostgreSQL persistence; the app maps protected HTTP errors; the Bootstrap Finance dashboard owns chart state. No database schema additions or changes are required.

## Goals / Non-Goals

**Goals:**

- Surface missing-FX partial data before interpreting an all-zero series as no settled activity.
- Treat every controller-side cash-flow parameter-validation failure, including the bucket cap, as invalid client input before calling the service.
- Keep series converted contributions numerically identical to existing dashboard conversion for the same data.
- Count monthly request buckets with the same original-start anchor used by SQL.

**Non-Goals:**

- Altering the route, response schema, bucket limit, transaction eligibility, or missing-FX response shape.
- Changing dashboard conversion behavior, FX persistence, or historical valuation rules.
- Changing database tables, indexes, migrations, dependencies, or ECharts behavior.
- Broad dashboard layout or chart presentation work beyond the incomplete-state ordering.

## Decisions

### Preserve partial-data diagnostics before zero-activity presentation

After loading and error handling, the dashboard will branch on a present series, render its incomplete-data warning first, then independently choose zero-activity or chart content. This retains the existing warning, diagnostics link, and zero-activity message when every qualifying contribution was omitted for missing FX; it also preserves the textual bucket alternative for a loaded series. This is a control-flow correction, not a new UI state or API field.

### Map explicit route validation directly to invalid input

The controller will map an error returned by its own `ValidateCashFlowSeriesParams` call to the app invalid-input error before service delegation. That boundary has already parsed the client request, so it must not rely on the narrower `mapFinanceRangeError` sentinel mapping. Registered-route coverage will prove an over-366 request returns a 4xx and does not call the service.

### Match Go's conversion arithmetic and rounding in SQL

The series query will make the converted minor-unit product use double-precision arithmetic and replace PostgreSQL `ROUND` with a sign-and-absolute-value expression that rounds half values away from zero, matching `int64(math.Round(float64(amountMinor) * rate))`. Each converted contribution remains rounded before aggregation. A PostgreSQL integration regression will seed the reviewed half-unit and floating-boundary examples and compare the sum of series buckets to the existing dashboard settled totals for the identical tenant, period, rate, and transactions.

### Derive every monthly validation boundary from the original start

Monthly bucket counting will keep the input start immutable and calculate each candidate boundary from the original calendar day and time at its month index, clipping only that indexed target month to its last day. It will not use a previously clipped boundary as the next anchor. This matches SQL's `start_date + bucket_index * interval '1 month'`; focused month-end cases will cover the cap boundary and the accepted limit.

## Risks / Trade-offs

- [SQL and Go floating-point behavior could regress at boundaries] -> Assert series totals equal dashboard totals with reviewed half and floating-boundary inputs.
- [Month-end calendar rules are easy to reintroduce cumulatively] -> Cover original-anchor bucket counts at both the accepted and rejected cap.
- [A partial warning could replace the empty message] -> Render the warning and zero-activity content as independent loaded-series states and cover the all-zero incomplete response.

## Migration Plan

1. Correct finance conversion and monthly validation with focused regression coverage.
2. Correct controller invalid-input mapping and registered-route coverage.
3. Correct dashboard state ordering, wireframe wording, and required UI verification.

Deploy as ordinary application code. Rollback reverts the follow-up code only; no schema, data, or migration rollback is needed.

## Open Questions

None.
