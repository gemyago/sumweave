## Context

The current deterministic product path has contracts and runtime code for data, analytics, and strategy. Strategy emits canonical `domain.CandidateAction` records, but there is no policy gate that decides whether an action is safe to execute and no execution boundary that owns order, fill, or reconciliation records after approval.

The architecture source of truth defines the deterministic path as `Data -> Analytics -> Strategy -> Governor -> Execution`, with AI outside the critical path and venue mechanics isolated at the edge. This change completes the missing runtime slices while preserving that direction: Governor evaluates strategy output; Execution handles only approved downstream behavior; orchestration remains outside both slices.

## Goals / Non-Goals

**Goals:**

- Add canonical shared domain records for governor decisions and execution records that are independent from persistence, venue payloads, AI prompts, and HTTP API concerns.
- Add a `runtime/governor` service that deterministically classifies candidate actions as approved, rejected, or blocked using explicit policy inputs.
- Add a `runtime/execution` service that accepts only approved governor decisions and creates deterministic execution commands and local order/fill/reconciliation records.
- Keep both slices on-demand and testable without adding database migrations, public API routes, UI screens, or live venue trading calls.

**Non-Goals:**

- No position accounting, portfolio optimization, account balances, margin modeling, or full risk engine beyond the initial policy gate.
- No live order placement, wallet signing, private venue authentication, or venue-specific trading adapter.
- No persisted execution ledger, order table, fill table, or reconciliation table in the initial capability.
- No cross-slice workflow runner; future backtest, paper, or live flows should orchestrate across slices from a thin orchestration area.

## Decisions

1. Model Governor and Execution as separate runtime slices.

Rationale: Governor is the risk and policy gate, while Execution owns command/order/fill/reconciliation behavior after approval. Combining them would blur approval rules with order lifecycle concerns and make the execution package the owner of upstream workflow.

Alternatives considered: Add approval logic inside `runtime/strategy`, or add policy checks inside `runtime/execution`. Both alternatives were rejected because they weaken the documented deterministic slice boundary and make it harder to test policy decisions independently.

2. Put only cross-slice records in `runtime/domain`.

Rationale: `domain` already holds canonical shared market, analytics, and strategy records. Governor decisions and execution records cross slice boundaries, so their identifiers, statuses, timestamps, and source records belong there. Request wrappers, service configuration, policy parameters, and dependency interfaces should stay in `runtime/governor` or `runtime/execution`.

Alternatives considered: Keep all types inside each slice, or add persistence models directly to domain. The first alternative would force downstream slices to import upstream implementation packages for canonical records. The second would violate the existing rule that domain records stay independent from persistence metadata.

3. Use explicit deterministic policy inputs for Governor v0.

Rationale: The initial governor service should be useful without hidden account state. A small policy input can cover action-kind allowlists, minimum acceptable data quality, and maximum approved action count per evaluation. The service can return per-action decisions with stable ordering and explicit reasons.

Alternatives considered: Introduce account/position state now, or approve everything by default. Account state is premature without an execution ledger and portfolio model. Approve-everything would create a named slice without meaningful gate behavior.

4. Keep Execution v0 as an approval-only local lifecycle boundary.

Rationale: Execution should prove the contract that only approved actions become execution commands, and that order/fill/reconciliation records are deterministic and validated. It should not place live orders until a future venue trading adapter and account model exist.

Alternatives considered: Add live venue order placement immediately, or defer execution entirely. Live placement would require private credentials, signing, and reconciliation complexity that are explicitly outside v0. Deferring execution would leave the deterministic path incomplete after Governor.

5. Preserve existing external API and persistence surfaces.

Rationale: The current capabilities are runtime contracts and services. They can be tested directly in Go without backend handler changes, migrations, UI work, or deployment sequencing.

Alternatives considered: Add API endpoints and database tables now. Both were rejected because there is not yet a product consumer requiring external access or persisted ledgers.

## Risks / Trade-offs

- Initial policy model is intentionally small -> Mitigation: make policy inputs explicit and keep account/position constraints out of v0 rather than implying support that does not exist.
- Execution records without persistence are not an audit ledger -> Mitigation: name this as an on-demand/local lifecycle capability and leave durable ledger work for a later change.
- Execution without live venue placement may look incomplete -> Mitigation: make the contract approval-only and local by design, with future venue trading integration isolated behind a separate capability.
- Shared domain growth can become too broad -> Mitigation: only add records consumed across slices and keep service-specific request/config types in slice packages.

## Migration Plan

- Add domain constructors and validation for governor and execution records.
- Add `runtime/governor` with unit tests for deterministic decisions and policy validation.
- Add `runtime/execution` with unit tests for approval-only command creation and order/fill/reconciliation validation.
- Keep backend, UI, persistence, and live venue behavior unchanged.
- Rollback is removing the new packages and domain records before any external consumers depend on them.

## Open Questions

- Which account and position state should Governor consume first when portfolio-aware risk is introduced?
- Should durable execution ledger persistence be owned by `runtime/execution` directly or by a thin orchestration/application layer?
- Which venue should provide the first private trading adapter once live execution becomes in scope?
