---
type: Historical record
title: "2026-06-29-implement-bank-provider-v2-connectors: proposal"
description: "Historical proposal record for 2026-06-29-implement-bank-provider-v2-connectors."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-06-29-implement-bank-provider-v2-connectors/proposal.md"
    title: "Original openspec/changes/archive/2026-06-29-implement-bank-provider-v2-connectors/proposal.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## Why

Provider sync v2 now has the shared execution shape, target-window planning, diff/apply planning, a sync-state journal, and one synthetic connector. The only supported real bank paths, monobank and PKO through Enable Banking, still exist only in the legacy finance provider seam.

That leaves a product gap in the v2 connector model. The executor can resolve technical connector IDs like `monobank` and `enable-banking`, but the finance module still has no real v2 connector implementation behind those IDs. We need to close that gap without reopening public API or UI scope, and without carrying legacy provider implementation dependencies into the new connector seam.

## What Changes

- Add a v2 `Connector` implementation for Monobank with token-link and fetch support.
- Add a v2 `Connector` implementation for Enable Banking with redirect start/finish and fetch support.
- Keep monobank as product provider `monobank` with technical connector `monobank`.
- Keep PKO as product provider `pko` composed through technical connector `enable-banking`.
- Keep any Monobank token or Enable Banking credential on the existing encrypted finance connection-secret path, never as plaintext-persisted connector state or raw payload evidence.
- Implement Monobank and Enable Banking connector behavior directly inside the v2 connector packages instead of wrapping or calling legacy root `finance` provider code.
- For Enable Banking v2, support only the existing legacy bearer-secret redirect/fetch branch and the signed official redirect/session-fetch branch; any other or mixed branch selection must fail with bounded unsupported errors.
- Convert real-bank fetch results into v2 `domain.ProviderSyncBatch` observations, including account, balance, transaction, provider-original, fingerprint, and raw-payload evidence.
- Keep connectors observation-only: no direct ledger persistence, no sync journal changes, and no new public HTTP or UI surface in this change.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finance-management`: provide real-bank Monobank and Enable Banking implementations behind the existing provider sync v2 connector seam.

## Impact

- Affected code: `finance/internal/monobank`, `finance/internal/enablebanking`, `finance/internal/providers`, and focused finance tests.
- Affected behavior: provider sync v2 can use the supported real bank connectors instead of only synthetic fetch implementations.
- Out of scope: new bank providers, UI/API workflow changes, provider discovery, and switching all runtime paths to v2 in the same change.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
