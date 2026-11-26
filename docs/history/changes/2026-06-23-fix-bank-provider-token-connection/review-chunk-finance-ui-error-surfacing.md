---
type: Historical record
title: "2026-06-23-fix-bank-provider-token-connection: review chunk finance ui error surfacing"
description: "Historical review chunk finance ui error surfacing record for 2026-06-23-fix-bank-provider-token-connection."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-06-23-fix-bank-provider-token-connection/review-chunk-finance-ui-error-surfacing.md"
    title: "Original openspec/changes/archive/2026-06-23-fix-bank-provider-token-connection/review-chunk-finance-ui-error-surfacing.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

# Chunk Review: finance-ui-error-surfacing

## Round 1

- Scope: follow-up UI error surfacing
- Triggering input: follow-up fix chunk not yet started
- Verdict: pending
- Notes: awaiting implementation sub-agent

## Round 2

- Scope: follow-up UI error surfacing (Finance Connections page)
- Triggering input: implementation sub-agent completed the follow-up
- Verdict: clean

### Findings

- No functional issues found.
- Monobank token submit failures now set a page-level `error` message and remain displayed in the existing `<p class="error" role="alert">` area.
- PKO redirect start failures now set the page-level error and do not attempt navigation on failure.
- Non-`Error` failures in both paths use the provided fallback messages:
  - `Failed to link monobank connection`
  - `Failed to start PKO connection`
- Existing page flow and UI layout appear unchanged apart from error surfacing behavior: tenant selection, card rendering, link/sync actions, and PKO redirect start call path remain intact.
- New tests directly cover the new error scenarios and both fallback paths, plus existing flow checks retained.

### Artifact cleanup status

- Scope-artifact updates are limited to:
  - `apps/signal-ui/src/pages/FinanceConnections.svelte`
  - `apps/signal-ui/src/pages/FinanceConnections.test.ts`
- No additional product-code or docs were changed for this chunk.
- Review artifact `openspec/changes/fix-bank-provider-token-connection/review-chunk-finance-ui-error-surfacing.md` was updated as requested.
- `openspec/changes/fix-bank-provider-token-connection/manager-status.md` remains modified in the working tree from prior process steps and should be confirmed before final submission.

### Completion protocol status

- `make affected-lint-test`: pass (full run completed successfully)
- Focused test run succeeded for changed spec: `npx vitest run --no-coverage src/pages/FinanceConnections.test.ts` from `apps/signal-ui` (10 passing)

### Commit status

- No commit exists yet for this follow-up chunk (`commit: none`).
- A chunk-level commit is still required before gate pass.

### Safe to continue to next follow-up chunk?

- Yes.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
