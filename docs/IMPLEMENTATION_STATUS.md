# Implementation Status

This document is a repo-local status snapshot.

Use [ARCHITECTURE.md](./ARCHITECTURE.md) as the product source of truth. This file only clarifies what is already implemented in the repository versus what is still missing for the MVP.

## Status meanings

| Status | Meaning |
| --- | --- |
| Implemented foundation | The repo contains a real slice or module foundation, but not a complete product workflow. |
| Partial | Some implementation exists, but important product-facing behavior is still missing. |
| Deferred | The architecture expects this area, but active implementation has not started yet. |
| Not implemented | The planned MVP capability is currently absent from the repo. |

## Current status map

| Area | Status | Current repo reality | MVP gaps |
| --- | --- | --- | --- |
| Shared domain | Implemented foundation | `runtime/domain` defines canonical records shared across deterministic slices, including market data, analytics, strategy, governor, and execution types. | Higher-level product artifacts such as `StrategyArtifact` and `GovernorPolicy` do not exist yet. |
| Data | Implemented foundation | `runtime/data` provides canonical ingestion, deterministic reads, replay behavior, and database persistence. `apps/signal-foundry/internal/data_layer.go` wires the data store and services into the backend foundation. | No production scheduling or job orchestration for ingestion, no operator workflow around data health, and no raw venue payload capture. |
| Venue edge | Partial | `runtime/venueedge` contains real market-data adapter work, including Hyperliquid support, behind the canonical data boundary. | No raw payload archive, no production ingestion scheduling, and no live trading adapter behavior. |
| Analytics | Implemented foundation | `runtime/analytics` exists as an on-demand deterministic calculation service over replay candles. | No persisted analytics outputs, no backtesting flow, no backend API integration, and no UI integration. |
| Strategy | Implemented foundation | `runtime/strategy` exists as an on-demand deterministic evaluation service over analytics inputs. | No `StrategyArtifact`, no strategy storage, no backtesting flow, and no backend or UI integration. |
| Governor | Implemented foundation | `runtime/governor` exists as an on-demand deterministic policy gate over candidate actions. | No `GovernorPolicy` product object, no persisted policy management, no audit traces, and no backend or UI integration. |
| Execution | Implemented foundation | `runtime/execution` exists as an on-demand local execution primitive for approved decisions, command creation, local orders, fills, and reconciliation. | No paper trading flow, no live order submission, no execution ledger, no audit traces, and no backend or UI integration. |
| Backend integration | Partial | `apps/signal-foundry` is a real backend foundation, but today it mainly exposes auth, agent runtime, and data-layer wiring. | Analytics, strategy, governor, and execution are not yet wired into backend DI, HTTP routes, or cross-slice orchestration. |
| UI | Partial | `apps/signal-ui` is a real UI foundation with login, chat, session history, theme support, and provider management. | The operator UI described by the product direction does not exist yet for data, analytics, strategy, governor, execution, or trading operations. |

## Important clarification

The repository already contains meaningful deterministic runtime foundations beyond the original agent and template scaffolding.

That does not mean the trading MVP is complete.

In particular, `runtime/analytics`, `runtime/strategy`, `runtime/governor`, and `runtime/execution` are currently on-demand primitives. They establish deterministic contracts and core behavior, but they are not yet assembled into a complete product workflow.

## Missing MVP capabilities

The main missing capabilities called out by current architecture and archived OpenSpec work are:

- `StrategyArtifact`
- `GovernorPolicy`
- backtesting
- paper trading
- audit traces
- operator UI for trading workflows
- raw venue payload capture

## Scope note

This document intentionally does not change architectural direction or add new decisions. It only makes the current implementation boundary easier to read from the repository itself.
