# Final Review

## Round 1

- Scope: `add-historical-data-browser-api-ui`
- Triggering input: whole-change implementation final review after all chunk reviews completed
- Findings:
  1. `apps/signal-ui/src/pages/Data.svelte` does not guard linked-evidence or raw-detail fetches against stale overlapping responses. If an operator selects candle A and then candle B before A's request resolves (or opens raw payload detail for row A and then row B), the older response can land last and overwrite the newer selection, leaving the evidence panel or drawer content mismatched with the currently selected item.
- Verdict: not yet ready; user correction required before final clean sign-off.
- Completion protocol status:
  - Whole-change review checks: artifact/code review only; no additional suites run in this round
  - Previously reported chunk checks: all chunk reviews recorded focused checks and `make affected-lint-test`
  - AGENTS.md update check: no changes needed
- Artifact cleanup status: `review-final.md` and `manager-status.md` updated for this round; archive still pending.

## Round 2

- Scope: `add-historical-data-browser-api-ui`
- Triggering input: whole-change re-review after the stale-response fix for linked-evidence/raw-detail requests
- Findings:
  1. `apps/signal-ui/src/pages/Data.svelte` still preserves stale top-level candle/raw table state across filter submissions. `handleSubmit()` resets selection/detail state, but it does not clear `candles` or `rawPayloads` before issuing the new `listCandles`/`listRawPayloads` requests, and it only replaces each collection on fulfilled responses. If query A loads successfully and query B is then submitted with one of the two reads failing, the page can show old normalized-candle results from A alongside fresh raw-payload results from B (or the inverse), with a mixed summary count under B's filters. This leaves the main browser panels inconsistent with the latest submitted filter set.
- Verdict: not yet ready; one more UI correction is required before clean sign-off.
- Completion protocol status:
  - Whole-change re-review checks: artifact/code review only; no additional suites run in this round
  - Implementation-reported verification for this correction round: `apps/signal-ui make lint`, `apps/signal-ui make test`, and repo-root `make affected-lint-test`
  - AGENTS.md update check: no changes needed
- Artifact cleanup status: `review-final.md` and `manager-status.md` updated for this round; archive still pending.

## Round 3

- Scope: `add-historical-data-browser-api-ui`
- Triggering input: whole-change re-review after the top-level stale-result correction in `Data.svelte`
- Findings: none.
- Notes:
  1. The latest-request guard now covers the top-level candle/raw metadata load path, so each valid Load clears prior summary/table state immediately and only applies responses from the newest submitted filter set.
  2. The earlier linked-evidence/raw-detail request-token guards remain in place, and `apps/signal-ui/ui-wireframe.md` now documents the latest-request replacement behavior for the `/data` route.
  3. The reviewed branch diff still matches the approved OpenSpec scope: authenticated read-only `/api/v1/data/*` endpoints, runtime/browser read models, and the protected `#/data` UI without mutation or out-of-scope workflow additions.
- Verdict: clean and ready for user review/correction; no further code changes are needed from this review round.
- Completion protocol status:
  - Whole-change re-review checks: artifact/code review only; no additional suites run in this round
  - Implementation-reported verification for the correction round: `apps/signal-ui make lint`, `apps/signal-ui make test`, and repo-root `make affected-lint-test`
  - AGENTS.md update check: no changes needed
- Artifact cleanup status: clean; only intended correction-round files and standard OpenSpec review artifacts remain modified, and archive is still pending.
- Commit status: the correction round is not committed yet; intended working-tree changes remain in `apps/signal-ui/src/pages/Data.svelte`, `apps/signal-ui/src/pages/Data.test.ts`, `apps/signal-ui/ui-wireframe.md`, `manager-status.md`, and this review log.

## Round 4

- Scope: `add-historical-data-browser-api-ui`
- Triggering input: user approval
- Exact user quote: `all good, submit`
- Derived action: archive the change, then submit by default
- Findings: none.
- Verdict: approved to archive and submit.
