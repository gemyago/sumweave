---
type: Historical record
title: "2026-07-04-add-synthetic-provider-linking-ui: proposal"
description: "Historical proposal record for 2026-07-04-add-synthetic-provider-linking-ui."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-07-04-add-synthetic-provider-linking-ui/proposal.md"
    title: "Original openspec/changes/archive/2026-07-04-add-synthetic-provider-linking-ui/proposal.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## Why

Synthetic provider sync is useful for local finance development, but it currently requires a temporary Go test helper to create the initial connection. We should expose synthetic setup as a first-class local link flow so API and UI e2e checks can exercise the same provider-sync path without hand-written test files.

## What Changes

- Promote synthetic from core-only setup to a protected local redirect-style link flow.
- Add synthetic provider support to redirect link start/finish while keeping Monobank token linking and PKO external SCA behavior intact.
- Use the synthetic link `ProviderReference` as the stable key for pending/final synthetic provider state.
- Add protected synthetic configuration API endpoints for reading and updating configured accounts against a pending synthetic link state.
- Update the finance UI connections screen with a synthetic setup route/form that starts the link, configures accounts, finishes the link, and returns to the connection list.
- Update manual API e2e documentation to remove the temporary Go-test workaround and verify the new API flow.
- Add manual UI e2e documentation for synthetic setup through the browser, including run/report/fix iteration expectations.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finance-management`: expose synthetic provider setup through protected API link/configuration flows and sync by `ProviderReference`.
- `finance-operator-ui`: expose synthetic setup as a supported finance connection workflow.

## Impact

- Affected code: `finance/`, `apps/signal-foundry/internal/api/http`, OpenAPI route/model generation, `apps/signal-ui`, and manual e2e docs.
- Affected API: existing redirect link start/finish provider enums gain `synthetic`; new tenant-scoped synthetic link configuration endpoints are added under `/api/v1/finance/...`.
- Affected storage: synthetic provider state is keyed by a provider reference/state key rather than the final bank connection id.
- Affected behavior: synthetic connections can be created via API/UI, then listed, synced, and deleted through existing connection workflows.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
