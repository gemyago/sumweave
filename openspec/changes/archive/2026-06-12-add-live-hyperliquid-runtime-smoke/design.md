## Context

`runtime/venueedge` already contains:

- a public Hyperliquid market-data adapter
- a canonical ingestion flow
- deterministic sandbox integration coverage
- mocked-HTTP real-adapter coverage

That foundation makes one higher-confidence step attractive: a manual smoke that replaces mocked HTTP with the real Hyperliquid public API while keeping the rest of the runtime path unchanged.

The existing Hyperliquid adapter scope is already a good fit. It reads public `POST /info` endpoints for metadata, candles, and recent trades. Those reads do not require signing, wallet approval, or a funded account. This means the first live smoke can stay tightly scoped to public market data while still validating the most important foundation path:

```text
real Hyperliquid API
  -> Hyperliquid venue-edge adapter
  -> venue ingestion flow
  -> SQLite data store
  -> canonical read service
```

## Goals / Non-Goals

**Goals**

- Add one opt-in manual `live` smoke path under `runtime/`.
- Exercise the real Hyperliquid public API through the existing adapter and data-layer ingestion path.
- Validate canonical persistence and readback in SQLite.
- Keep the live path stable enough for repeated human-triggered runs through careful time windows and non-brittle assertions.
- Keep normal automated test lanes fully offline.

**Non-Goals**

- No wallet setup, funded accounts, `approveAgent`, or private-account reads.
- No order placement, cancelation, transfers, or other state-changing actions.
- No full backend-app boot or operator-UI coverage.
- No CI integration for this live lane yet.
- No broad multi-venue live test framework beyond what this single smoke needs.

## Decisions

### Decision: Keep the live smoke under `runtime/`, not top-level `tests/`

The live smoke should live near the real product module and current venue-edge/data-layer code rather than in the template-derived top-level `tests/` area.

Rationale: the behavior under test is runtime foundation behavior, not the reference-only agent harness.

### Decision: Use `live` naming for folder, build tag, and make targets

The change should consistently use `live` rather than generic `e2e` naming.

Suggested shape:

```text
runtime/venueedge/live/
```

Suggested invocation shape:

- runtime target: `make test-live-compile`
- runtime target: `make test-live`
- repo target: `make test-live-compile`
- repo target: `make test-live`
- Go build tag: `live`

Rationale: these tests are not whole-product end-to-end tests. They are manual live venue/runtime smokes.

### Decision: Pair the `live` tag with a regular compile-only check

The live lane should stay behind a `live` build tag for execution, but the repository should still run a compile-only check for that lane during regular validation so it cannot silently rot.

Rationale: excluding live tests from default execution is correct because they require network access and human intent. However, excluding them from compilation entirely would allow straightforward type or import breakage to go unnoticed for too long.

Expected shape:

- compile-only regular check:
  - uses the `live` build tag
  - does not require network access
  - verifies the live package and test files still build
- manual execution lane:
  - uses the same `live` build tag
  - performs the actual real-API smoke

### Decision: Keep a single smoke path that includes the data layer

The first live smoke should run through the real Hyperliquid adapter, `venueedge.IngestionFlow`, an ephemeral SQLite `data.DatabaseStore`, and the canonical `data.ReadService`.

Rationale: the user's primary confidence target is the foundation path, especially the data layer. Testing adapter-only would be useful, but adapter plus SQLite persistence provides much stronger signal for nearly the same manual cost.

Alternative considered: add only adapter-level live tests first. That would be narrower, but it would leave the persistence and canonical readback path unproven against real venue responses.

### Decision: Keep the smoke read-only and public

The live smoke should use only public Hyperliquid market-data reads and should not require any wallet, address provisioning, or funded account.

Rationale: this keeps the first live lane operationally simple while still exercising real venue behavior.

### Decision: Default to mainnet public reads, with room for future override

The initial smoke should assume the public mainnet Hyperliquid API unless a later implementation detail adds an explicit override for debugging or future testnet use.

Rationale: mainnet public reads require no provisioning and are the closest thing to real production venue behavior for this slice.

### Decision: Use conservative, low-flake assertions

The smoke should assert structural and canonical correctness rather than brittle market specifics.

Expected assertion style:

- non-empty instrument results for a known liquid symbol
- ingested candle rows are non-empty for a recent fully closed time window
- ingested trade rows are non-empty for a short recent liquid-symbol range
- stored records read back through the canonical read service
- venue, symbol, UTC normalization, quality, ordering, and provenance expectations hold

The smoke should avoid:

- exact record counts
- assumptions about fixed trade volume
- assumptions that a partially open time bucket is already final

Rationale: real venue data shifts continuously, so stability depends on asserting invariants rather than exact market snapshots.

### Decision: Expect an explicit stabilization and rerun loop

This change is unusually validation-heavy relative to code volume. The implementation tasks should therefore include repeated manual runs against the real API and adjustment of time windows, symbol choices, and assertions until the lane is stable enough for intentional human use.

Rationale: the likely work is not broad feature coding. It is reducing flake and learning the real venue response edges. The implementation flow should assume a live run may fail, require investigation and fixes, and then require the orchestrator to rerun the live lane until it reports success.

## Proposed Smoke Shape

One manual test should cover:

1. Build the real Hyperliquid adapter.
2. Build an ephemeral SQLite store and auto-migrate it.
3. Build ingestion and read services.
4. Ingest:
   - instruments for `hyperliquid-perps`
   - candles for a liquid symbol over a recent fully closed window
   - trades for a short recent range on the same symbol
5. Read the persisted records back through the read service.
6. Assert canonical invariants and deterministic readback.

## Risks / Trade-offs

- Live trade windows may be narrower or more volatile than expected.
  - Mitigation: use a highly liquid symbol and a short recent window with tolerant assertions.
- A smoke that spans adapter plus SQLite may be harder to stabilize than adapter-only.
  - Mitigation: keep exactly one smoke and keep assertions modest.
- Public venue availability or transient rate-limit state can still fail manual runs.
  - Mitigation: keep the lane opt-in and provide clear failure messages.
- Mainnet live reads may expose edge cases not covered by mocked fixtures.
  - Mitigation: treat that as the value of the smoke, then harden offline tests afterward if new cases are discovered.

## Migration Plan

1. Add an OpenSpec delta allowing opt-in manual live smoke coverage while keeping default suites offline.
2. Add a `live`-tagged runtime test package for Hyperliquid public reads.
3. Wire the test through the existing adapter, ingestion flow, SQLite store, and canonical read service.
4. Add explicit runtime and repo make targets for manual invocation.
5. Run the lane repeatedly and tune time windows/assertions until it is stable for human-triggered use.

## Open Questions

- Should the first implementation include a simple environment override for the Hyperliquid base URL, or should it hardcode mainnet initially and add configurability only if needed?
- Is one live smoke enough, or will candles and trades need to split later if trade-window behavior proves materially less stable?
