## Context

`docs/ARCHITECTURE.md` defines Signal Foundry's deterministic path as `Data -> Analytics -> Strategy -> Governor -> Execution`, with cross-slice orchestration kept thin in `runtime/flows` or `runtime/runs`. The archived `2026-06-14-add-thin-paper-backtest-flow` change added the first in-memory flow that composes existing deterministic slices and local paper execution records. These five tickets ask for the next durable paper/backtest layer while preserving the same slice boundaries.

Notion was accessible while planning, and the ticket bodies were used as requirements. The repo-local architecture and accepted specs remain the source of truth when resolving scope.

## Goals / Non-Goals

**Goals:**
- Persist the minimal audit chain for strategy decisions and order intents.
- Expand governor checks for explicit paper/backtest intent context and safety policy fields.
- Persist paper execution commands, orders, and fills and simulate deterministic limit fills.
- Project paper position and portfolio snapshots from persisted fills.
- Persist reproducible backtest run and evaluation report scaffolding.
- Keep existing deterministic flow ordering and add only the linkage required to prove trace -> intent -> governor -> execution -> state/report relationships.

**Non-Goals:**
- No live trading enablement, live order placement, wallet signing, private venue APIs, or live account reconciliation.
- No backend HTTP/OpenAPI routes, dependency-injection wiring, scheduled jobs, or UI screens.
- No market orders, cancel/replace workflows, partial-fill simulator, leverage/margin/funding/liquidation modeling, advanced promotion thresholds, or portfolio optimization.
- No large unbounded market snapshots or report blobs in relational rows.
- No AI/agent call in the deterministic decision or execution path.

## Decisions

### Implement dependency foundation before parent ticket order

Although the audit-chain ticket is listed last, its own dependency note says it should preferably precede expanded governor and execution work. Implementation should therefore begin with `DecisionTrace` and `OrderIntent`, then proceed through the listed parent tickets: governor checks, execution ledger/simulator, paper state snapshots, and backtest/evaluation scaffold.

### Keep shared domain small and persistence models separate

Add only cross-slice concepts to `runtime/domain`: stable identifiers, mode/status/reason enums, intent/trace references, execution order-type/time-in-force concepts, snapshot records, and backtest/evaluation references that are consumed across packages. Persistence-specific fields, GORM tags, table names, indexes, and migration models must stay in slice packages such as `runtime/audit`, `runtime/execution`, or `runtime/backtest`.

### Use `OrderIntent` as the richer governor input

Do not add sizing, price, order type, mode, policy hash, or exposure fields to `domain.CandidateAction`. Strategy output remains directional. The audit/service layer creates an `OrderIntent` from strategy context plus explicit sizing and pricing inputs, and the governor evaluates an intent-like request that includes mode, venue, instrument, strategy/artifact references, quality, requested quantity/notional, optional limit price, current strategy exposure, current instrument exposure, and governor policy id/version/hash.

Existing candidate-action evaluation may remain as a compatibility helper for current tests and the thin flow, but the new paper/backtest path should exercise the intent-based evaluator.

### Constrain governor expansion to deterministic paper/backtest safety

The governor should reject or block live mode. The expanded policy must cover allowed modes, venue allowlist, instrument allowlist, strategy allowlist or eligibility reference, allowed action kind, minimum quality, kill switch/new-risk blocked state, maximum order notional, projected maximum strategy notional, projected maximum instrument notional, and maximum approved count. The result should continue to use `approved`, `rejected`, and `blocked`; no `approved_with_adjustment` is introduced.

Stable reason codes must cover the ticket-required set: `OK`, `MODE_NOT_ALLOWED`, `VENUE_NOT_ALLOWED`, `INSTRUMENT_NOT_ALLOWED`, `STRATEGY_NOT_ALLOWED`, `ACTION_KIND_NOT_ALLOWED`, `DATA_QUALITY_TOO_LOW`, `KILL_SWITCH_ACTIVE`, `ORDER_NOTIONAL_EXCEEDS_LIMIT`, `STRATEGY_EXPOSURE_EXCEEDS_LIMIT`, `INSTRUMENT_EXPOSURE_EXCEEDS_LIMIT`, `APPROVAL_LIMIT_REACHED`, and `INVALID_INTENT`.

Canonical persisted reason-code strings MUST use the uppercase snake-case values listed above exactly. Stores, audit references, reports, and tests must not remap these governor reason codes to lower-kebab, lowercase, localized, or display-label strings; any display formatting belongs outside the persisted domain value.

### Put paper ledger and limit simulation in execution

`runtime/execution` owns approval-admitted command/order/fill behavior after governor approval. Add ledger persistence there, not in the flow. Persist the command before order/fill side effects. Retrying the same command must not duplicate orders or fills, and deterministic client order ids must be stable for the same command.

The v0 simulator supports only limit orders using closed replay candles. A buy/long limit fills when a later eligible candle's low is less than or equal to the limit price. A sell/short limit fills when a later eligible candle's high is greater than or equal to the limit price. The v0 simulator creates a full fill at the limit price, records deterministic zero fee/slippage assumptions in metadata, and returns no fill when no eligible candle reaches the limit. Market and unsupported order types are validation errors.

### Project paper state from fills in execution

Position and portfolio snapshots are deterministic projections over execution fills ordered by event time and stable fill id tie-breaker. Long quantity is positive and short quantity is negative. Opening/increasing updates average entry; reducing realizes PnL from average entry; flattening sets quantity to zero and clears average entry. Reversal is deferred for v0 and should be rejected or surfaced as an unsupported projection case with tests.

Perps-specific assumptions are explicit: no intentional leverage, USDC collateral only when collateral is modeled, funding/liquidation/margin modeling deferred, and unrealized PnL emitted only when a deterministic mark/reference price is supplied.

### Add backtest/evaluation as a narrow persistence scaffold

Create a small runtime package such as `runtime/backtest` for dataset reference records, `BacktestRun` lifecycle, and `EvaluationReport` records. The canonical data provenance concept in this change is a compact dataset reference record, not an additional data capture object. It should not own the deterministic slice internals. Metrics are compact, versioned JSON. If a metric cannot be derived from available fills/snapshots, omit it and document it as deferred rather than emitting misleading zero values.

### Preserve thin flow ownership

`runtime/flows` may coordinate audit, intent creation, governor evaluation, execution ledger, state projection, and backtest/evaluation references for an integration path. It must remain a coordinator and must not duplicate strategy calculations, governor checks, execution ledger writes, fill simulation, or position math.

## Risks / Trade-offs

- [Risk] This is a large combined runtime change. Mitigation: strict ordered chunks and no backend/UI/live-trading expansion.
- [Risk] New domain records can bloat the shared kernel. Mitigation: only cross-slice vocabulary belongs in `domain`; persistence models stay in slice packages.
- [Risk] Closed-candle limit simulation is simplistic. Mitigation: document it as v0, full-fill-only, deterministic, and conservative enough for paper/backtest scaffolding.
- [Risk] Existing paper-backtest flow used immediate close-price fills. Mitigation: preserve tests for deterministic behavior while moving durable paper execution to the new limit simulator path.

## Migration Plan

Add new runtime persistence models and AutoMigrate behavior behind the new stores. SQLite tests must prove schema creation and idempotent writes. No production rollout or app startup migration is required in this OpenSpec change because backend wiring is out of scope.

## Open Questions

- Should the audit package be named `runtime/audit`, `runtime/runs`, or another domain-specific name? This plan recommends `runtime/audit` for trace/intent persistence and `runtime/backtest` for run/report persistence.
