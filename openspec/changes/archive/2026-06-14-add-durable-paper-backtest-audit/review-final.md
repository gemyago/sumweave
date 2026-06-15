# Final Review: add-durable-paper-backtest-audit

## 2026-06-14 Whole-change review

- Verdict: **CHANGES REQUESTED**
- Scope reviewed: planning/task artifacts, all chunk review artifacts, and the `runtime/` implementation touched by this change.

### Whole-change findings

1. **Expanded governor policy checks are not preserved through the flow boundary.**
   - `runtime/flows/paper_backtest.go` canonicalizes `governor.Policy` into a reduced struct that keeps only `AllowedActionKinds`, `MinimumQuality`, and `MaximumApprovedCount`.
   - This silently drops the chunk-2 policy fields (`AllowedModes`, `AllowedVenues`, `AllowedInstruments`, `AllowedStrategyIDs`, `BlockNewRisk`, `MaximumOrderNotional`, `MaximumStrategyExposureNotional`, `MaximumInstrumentExposureNotional`) before the flow calls `governor.Evaluate`.
   - Result: the direct governor service supports the new checks, but the actual paper/durable backtest flows do not exercise or enforce most of them end-to-end.
   - Needed follow-up: add a small integration fix chunk that preserves the full canonicalized policy through the flow request path and adds flow-level tests proving the dropped policy fields are enforced.

2. **The final linkage flow still does not write back downstream audit references.**
   - `runtime/flows/durable_backtest_flow.go` links dataset/run metadata up front and later updates only order-intent status.
   - It does **not** append/store governor decision, execution command/order/fill, snapshot, or evaluation-report references on the audit records after those downstream records are created.
   - This falls short of the flow/audit requirements that the audit chain retain navigation references across the full durable path when downstream records exist.
   - Needed follow-up: extend the audit model/store update path for downstream linkage metadata or explicit reference fields, then cover it in the end-to-end durable flow test.

3. **Dataset provenance is run-scoped instead of input-scoped.**
   - `runtime/flows/durable_backtest_flow.go` builds `DatasetReference.DatasetID` and `ReplayChecksum` from `request.runID` as well as replay inputs.
   - That means two identical replay datasets executed under different run IDs produce different dataset identities/checksums, which weakens the reproducible dataset-reference scaffold added in chunk 5.
   - Needed follow-up: derive replay checksum (and ideally dataset identity) from replay inputs/provenance only, keeping run-specific linkage separate.

### Verification

- Reviewed `review-planning.md`, all `review-chunk-*.md` files, and `tasks.md`.
- Ran: `go test ./runtime/...` ✅

### Ready/continue decision

- **Not ready** for user review/correction yet.
- Recommended next step: one focused follow-up integration fix chunk covering:
  1. full governor-policy propagation,
  2. downstream audit-link write-back, and
  3. input-scoped dataset provenance/checksum behavior.

## 2026-06-14 Whole-change re-review after follow-up fix

- Verdict: **CHANGES REQUESTED**
- Scope reviewed: the prior whole-change findings plus the follow-up fix in `runtime/audit/` and `runtime/flows/`.

### Prior findings status

- ✅ Full governor-policy propagation is now preserved through the flow boundary and covered by end-to-end flow tests.
- ✅ Downstream audit references are now written back onto persisted traces/intents and covered by the durable flow integration test.
- ✅ Dataset identity/checksum provenance is now input-scoped rather than run-scoped and covered by targeted tests.

### Remaining finding

1. **`DurableBacktestResult.IntentContexts` is still returned in a stale pre-report state.**
   - `runtime/flows/durable_backtest_flow.go` now calls `writeDownstreamAuditReferences(...)` inside `projectAndReport`, but the updated contexts it returns are discarded.
   - `Run(...)` still returns the older `intentContexts` slice produced by `evaluateBacktest(...)`, so callers do not receive the final snapshot/report linkage in `DurableBacktestResult.IntentContexts` even though the persisted audit records were updated.
   - Needed follow-up: thread the updated intent contexts back into the final flow result (or stop returning this field if stale in-memory linkage is acceptable), and keep result/output expectations aligned with the persisted linkage behavior.

### Verification

- Re-reviewed the follow-up fix against the prior whole-change findings.
- Ran: `go test ./runtime/flows/... ./runtime/audit/...` ✅

### Ready/continue decision

- **Not ready** for user review/correction yet.
- Recommended next step: one small follow-up fix for the stale returned durable intent contexts.

## 2026-06-15 Whole-change re-review after final follow-up fix

- Verdict: **APPROVED**
- Scope reviewed: prior whole-change blockers plus the latest uncommitted follow-up in `runtime/audit/` and `runtime/flows/`.

### Prior findings status

- ✅ Full governor-policy propagation remains preserved through the flow boundary and is still covered by end-to-end flow tests.
- ✅ Downstream audit references are persisted onto traces/intents, including intent linkage, governor decision references, execution references, snapshot references, and evaluation-report linkage.
- ✅ Dataset identity/checksum provenance remains input-scoped rather than run-scoped.
- ✅ `DurableBacktestResult.IntentContexts` now returns the final refreshed contexts from `projectAndReport`, so the returned in-memory contexts match the persisted audit records.

### Remaining findings

- No blocking issues found in this review round.

### Verification

- Re-reviewed the final follow-up diff and the durable backtest integration assertions.
- Ran: `go test ./runtime/...` ✅

### Ready/continue decision

- **Ready** for user review/correction.
- No additional follow-up chunk is needed from this review pass.

## 2026-06-15 User approval

- User quote: `All good`
- Derived action: archive and submission
- Notes: user approved the completed change after final review.
