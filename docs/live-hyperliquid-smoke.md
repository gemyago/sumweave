# Hyperliquid Live Smoke

Manual runbook for the opt-in live Hyperliquid runtime smoke.

## What it proves

The live smoke is a read-only proof that this path still works against the real public Hyperliquid API:

```text
real Hyperliquid API
  -> Hyperliquid venue-edge adapter
  -> venueedge.IngestionFlow
  -> ephemeral SQLite store
  -> canonical data read service
```

An intentional `make test-live` run proves that the runtime can:

- ingest `hyperliquid-perps` instrument data for a known liquid symbol
- ingest a recent fully closed candle window
- ingest a short recent trades window
- persist those records into SQLite
- read the canonicalized records back with stable invariants

## Scope

This lane is intentionally narrow.

Included:

- public market-data reads
- instrument ingestion
- candle ingestion
- recent-trade ingestion
- SQLite persistence and canonical readback

Explicitly out of scope for this change:

- wallets
- `approveAgent`
- private account state
- transfers
- signing
- order placement
- trading flows of any kind

## How to run it

Regular completion checks should keep the live lane compiling without turning on real network access:

```bash
make test-live-compile
```

That compile-only check is part of the regular completion expectation for this live lane.

Manual live execution is separate and intentional:

```bash
make test-live
```

Both repo-root targets delegate to the runtime module targets with the same names.

## Operator expectations

- `make test-live-compile` is the regular safety check. It must stay green as part of normal completion checks so `live`-tagged coverage does not silently rot.
- `make test-live` is not a default gate. It is a manual, orchestrated rerun step used to validate the real public venue path on demand.
- If `make test-live` fails because of a real implementation issue, fix the issue, report what changed, and have the orchestrator rerun `make test-live` until that lane succeeds.
- If `make test-live` fails because of venue or network volatility, treat that as a transient manual-lane failure and rerun it through the orchestrator instead of widening scope into private or trading flows.
