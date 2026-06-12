# Architecture

This document records the current high-level product architecture direction for Signal Foundry.

It is intentionally brief. It captures the shape we agreed on, not detailed implementation design.

## Product stance

Signal Foundry is a deterministic trading platform with AI-assisted research around it.

AI may help with:
- research
- drafting strategy candidates
- critique
- explanation
- summaries

AI must stay outside the critical execution path.

The deterministic path is:

```text
Data -> Analytics -> Strategy -> Governor -> Execution
```

## Repository shape

The intended long-term product path remains:
- `runtime/` as the core product runtime
- `apps/signal-foundry/` as the Go API/jobs application
- `apps/signal-ui/` as the operator UI

Template-derived code outside those areas stays reference-only unless explicitly adopted.

## Original concept pages

The original product slice concepts were drafted in Notion. Those pages are useful for intent, vocabulary, and early design context, but they are not the source of truth for the repository.

The source of truth is the codebase, local architecture docs, `AGENTS.md` files, and accepted implementation decisions recorded in this repository.

Original conceptual design pages:
- Product overview: <https://app.notion.com/p/37b7d50e7d84806fa9f2e11ef2c37cae>
- Data slice: <https://app.notion.com/p/37c7d50e7d848199bf1df857f2982f93>
- Analytics slice: <https://app.notion.com/p/37c7d50e7d8481aeb7f3c8168f29f182>
- Strategy slice: <https://app.notion.com/p/37c7d50e7d8481eeb92bc3f8cd31fff9>
- Governor slice: <https://app.notion.com/p/37c7d50e7d848101bfbffdd426eeaa3d>
- Execution slice: <https://app.notion.com/p/37c7d50e7d8481a8bf3be4794a220e7b>
- Operator UI slice: <https://app.notion.com/p/37c7d50e7d8481b0ae2df67cba53c8f8>

## Runtime shape

`runtime/` should evolve as a shared-kernel plus slices design.

### Shared kernel

`domain/` holds canonical shared domain models and vocabulary used across slices.

It should stay small and stable. It is for product concepts, not orchestration or persistence concerns.

Examples of what belongs there:
- canonical identifiers and enums
- instruments, timeframes, and market data records
- analytics outputs
- strategy, risk, and execution records that cross slice boundaries

### Slices

The main runtime slices should be:
- `data`
- `analytics`
- `strategy`
- `governor`
- `execution`

Expected dependency direction is mostly bottom-to-top:

```text
data -> analytics -> strategy -> governor -> execution
```

Slices should stay as independent as practical. `data` is the main shared foundation because it owns canonical market/reference data, normalization, quality state, and replayable persistence.

### Orchestration

Cross-slice orchestration should stay thin and explicit in a small runtime area such as `runs/` or `flows/`.

`execution` should not become the place that owns the whole product workflow. It owns command/order/fill/reconciliation behavior after approval, while end-to-end backtest and paper flows should orchestrate across slices.

## Venue integration

Venue integration should be isolated, but kept simple.

Current decision:
- isolate venue implementation in its own package
- keep abstractions narrow
- use industry and product domain language at slice boundaries
- avoid a large generic venue framework until it is clearly needed

This means vendor-specific mechanics such as signing, nonces, symbol mapping, payloads, and reconciliation quirks belong at the venue edge, while slices should depend on small consumer-defined interfaces expressed in product terms.

## Storage

Persistence should start with GORM and support:
- SQLite for local development
- PostgreSQL for production when needed

This should stay disciplined:
- explicit schemas
- explicit column names
- explicit migrations
- UTC-first time handling

GORM persistence models should stay isolated from `domain/`. Even when a persistence model is structurally identical to a domain model, keep it separate so shared domain types are not polluted with GORM tags or persistence-only metadata.

We can replace specific hot paths with lower-level SQL later if needed without changing the higher-level slice boundaries.
