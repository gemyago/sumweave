## Why

Signal Foundry has deterministic data, analytics, and strategy slice contracts, but the critical path currently stops before the policy gate and order lifecycle boundary. Adding Governor and Execution layers completes the planned deterministic path from strategy candidate actions to approved execution commands and locally reconciled execution records.

## What Changes

- Add a `governor-layer` capability that evaluates strategy candidate actions against deterministic risk and policy rules before any execution behavior can occur.
- Add an `execution-layer` capability that accepts only governor-approved actions and manages execution commands, order records, fill records, and reconciliation state without owning upstream workflow orchestration.
- Define shared domain records needed at the Governor and Execution boundaries, keeping them independent from persistence metadata, venue payloads, AI systems, and external API concerns.
- Provide initial on-demand/runtime services for Governor and Execution without requiring new public HTTP endpoints, persisted materialized strategy outputs, or live trading integrations.

## Capabilities

### New Capabilities

- `governor-layer`: Deterministic risk and policy gate that turns strategy candidate actions into approved, rejected, or blocked decisions for downstream execution.
- `execution-layer`: Deterministic execution boundary that turns approved actions into execution commands and tracks order, fill, and reconciliation records after approval.

### Modified Capabilities

- None.

## Impact

- Affected code is expected under `runtime/domain`, `runtime/governor`, and `runtime/execution`.
- Existing `data-layer`, `analytics-layer`, `strategy-layer`, and `venue-edge` contracts remain unchanged.
- No new public backend API routes, UI screens, live venue trading calls, wallet signing, or AI-assisted execution path behavior are required for the initial capability.
- Tests should cover deterministic policy decisions, approval-only execution admission, order/fill state transitions, reconciliation behavior, and boundaries that keep venue mechanics and AI outside the critical path.
