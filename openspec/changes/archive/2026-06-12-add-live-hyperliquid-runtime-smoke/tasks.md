## 1. Manual Live Test Lane

- [x] 1.1 Add a `live`-tagged Hyperliquid smoke test under `runtime/` and keep it excluded from default `go test ./...` behavior.
- [x] 1.2 Add compile-only entrypoints for the `live` lane so regular checks can verify that `live`-tagged tests still build without running the real network smoke.
- [x] 1.3 Add explicit manual execution entrypoints such as runtime-level and repo-level `test-live` targets.
- [x] 1.4 Document in the test lane itself that it is read-only, manual, and intentionally outside CI and default completion gates.

## 2. Real Hyperliquid To SQLite Smoke Path

- [x] 2.1 Build one smoke around the existing Hyperliquid adapter, `venueedge.IngestionFlow`, an ephemeral SQLite store, and canonical read services.
- [x] 2.2 Keep the smoke limited to public market-data reads and avoid wallet, account, or signing setup entirely.
- [x] 2.3 Ingest instruments, a recent fully closed candle window, and a short recent trade window for a liquid Hyperliquid symbol.

## 3. Stabilization Loop

- [x] 3.1 Run the live lane manually multiple times against the real API and refine the time windows until repeated intentional runs behave consistently.
- [x] 3.2 Tune assertions toward canonical invariants and readback correctness rather than brittle exact market snapshots.
- [x] 3.3 Make live failure messages clear enough to distinguish venue/network volatility from runtime canonicalization or persistence regressions.
- [x] 3.4 If a live run fails, investigate why, fix what can be fixed in the implementation, and report that the live lane must be rerun by the orchestrator until it reports success.

## 4. Follow-Through

- [x] 4.1 Document how to run the live lane and what it is meant to prove.
- [x] 4.2 Keep wallets, `approveAgent`, private account state, and trading flows explicitly out of scope for this change.
- [x] 4.3 Treat the compile-only `live` check as part of regular completion checks.
- [x] 4.4 Treat manual `test-live` execution as an orchestrated rerun step: run it, and if it fails after a fix, report the fix and require the orchestrator to rerun that lane until it succeeds.
