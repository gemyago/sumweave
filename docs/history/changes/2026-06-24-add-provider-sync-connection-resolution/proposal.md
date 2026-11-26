---
type: Historical record
title: "2026-06-24-add-provider-sync-connection-resolution: proposal"
description: "Historical proposal record for 2026-06-24-add-provider-sync-connection-resolution."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-06-24-add-provider-sync-connection-resolution/proposal.md"
    title: "Original openspec/changes/archive/2026-06-24-add-provider-sync-connection-resolution/proposal.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## Why

Provider sync v2 already defines fetch, diff, and apply seams, but the coordinator still cannot choose the correct technical connector for a bank connection. This change is needed now so sync orchestration can start from the connection's stored connector identity instead of hardcoded or legacy provider-specific branching.

## What Changes

- Define provider sync v2 coordinator requirements for resolving a technical connector from the bank connection before fetch begins.
- Add a finance-owned connector registry contract so coordinator code can resolve `monobank`, `enable-banking`, and future technical connectors without direct constructor branching.
- Define failure behavior for unknown or unsupported connector IDs so the sync path fails early, audibly, and without invoking the wrong provider integration.
- Keep the product-provider versus technical-connector split explicit so PKO continues to sync through Enable Banking while monobank continues to sync through its own connector.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `finance-management`: extend provider sync v2 requirements to cover coordinator connector resolution and unsupported-connector handling.

## Impact

- Affected code: `finance/internal/providers/`, finance sync service orchestration, and connector registration wire-up.
- Affected systems: bank connection sync execution for provider sync v2, especially PKO via Enable Banking and monobank direct sync selection.
- Dependencies: existing provider connectors and provider sync v2 domain models will need a coordinator-facing registry contract, but this change does not add new persistence behavior.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
