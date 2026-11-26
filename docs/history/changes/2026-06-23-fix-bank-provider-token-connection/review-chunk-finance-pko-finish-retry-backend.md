---
type: Historical record
title: "2026-06-23-fix-bank-provider-token-connection: review chunk finance pko finish retry backend"
description: "Historical review chunk finance pko finish retry backend record for 2026-06-23-fix-bank-provider-token-connection."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-06-23-fix-bank-provider-token-connection/review-chunk-finance-pko-finish-retry-backend.md"
    title: "Original openspec/changes/archive/2026-06-23-fix-bank-provider-token-connection/review-chunk-finance-pko-finish-retry-backend.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

# Chunk Review: finance-pko-finish-retry-backend

## Round 1

- Scope: follow-up PKO finish retry backend recovery
- Verdict: clean
- Findings:
  - No blocking findings in the backend retry-recovery scope.
  - `FinishBankConnectionLink` now restores the consumed pending PKO start when `provider.FinishLink` fails, so a transient first failure remains retryable.
  - A second finish attempt after that transient failure is covered and succeeds in `TestProviderSyncInternals`.
  - A second finish attempt after a successful completion is still blocked by `ErrPendingBankConnectionLinkStartNotFound`; the restore path only runs on finish failure.
  - Restore-failure handling is sane: the returned error keeps the original finish failure and adds restore context, and the path is covered by test.
  - Focused validation passed: `direnv exec /Users/jenya/projects/signal-foundry/finance go test ./...`.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
