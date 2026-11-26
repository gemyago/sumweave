---
type: Historical record
title: "2026-06-23-fix-bank-provider-token-connection: review chunk finance ui pko finish retry"
description: "Historical review chunk finance ui pko finish retry record for 2026-06-23-fix-bank-provider-token-connection."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-06-23-fix-bank-provider-token-connection/review-chunk-finance-ui-pko-finish-retry.md"
    title: "Original openspec/changes/archive/2026-06-23-fix-bank-provider-token-connection/review-chunk-finance-ui-pko-finish-retry.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

# Chunk Review: finance-ui-pko-finish-retry

## Round 1

- Scope: follow-up PKO finish retry recovery
- Triggering input: follow-up fix chunk not yet started
- Verdict: pending
- Notes: awaiting implementation sub-agent

## Round 2

- Verdict: **clean**
- Findings:
  - No blocking findings in the follow-up scope.
  - `finishRedirectIfReturned` now preserves `?code`/`?state` on failure and only clears on success, satisfying retry persistence.
  - `finishingRedirect` is reset in `finally`, so failure cannot leave the handler stuck in a finishing state.
  - Retry path works on reopen/reload: callback params remain in `window.location.search` after transient failure, and the wireframe now documents refresh/re-open retry behavior.
  - Added test covers the failure path and verifies that params survive one failure and succeed on reopen.
- Artifact cleanup status: **acceptable** (changes are limited to the scope files; docs tweak is aligned to behavior change).
- Completion protocol status:
  - Lint/tests were **not run in this review pass** by me.
  - Per root/module protocol, chunk should still satisfy `make affected-lint-test` in final closeout.
  - UI smoke/visual check for the changed flow still needs to be confirmed before final gate.
- Commit status:
  - No commit for this follow-up chunk yet (`git status` shows working-tree edits only).
  - A commit is now required before gate can pass.
- Safe to continue: **yes**, provided validation steps above are completed and commit is created.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
