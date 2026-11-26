---
type: Historical record
title: "2026-07-06-update-finance-tenants: proposal"
description: "Historical proposal record for 2026-07-06-update-finance-tenants."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-07-06-update-finance-tenants/proposal.md"
    title: "Original openspec/changes/archive/2026-07-06-update-finance-tenants/proposal.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## Why

Finance tenants can currently be created and archived, but operators cannot correct a tenant name or display currency after creation. Tenant display currency is also accepted as free text in the API and UI, which allows unsupported or typo-prone currency values into a core reporting field.

## What Changes

- Add a tenant update workflow for changing tenant name and display currency for joined tenant members.
- Constrain tenant display-currency values on tenant create and update to a predefined list of valid currency codes.
- Replace the free-text tenant display-currency input in tenant create/update UI with a select control backed by the same product-supported currency list.
- Keep existing tenant membership, invite, archive, account, transaction, and reporting semantics unchanged except that future display-currency reporting uses the updated tenant display currency.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finance-management`: Tenant management gains update behavior and tenant display-currency validation.
- `finance-operator-ui`: Tenant management UI gains tenant update controls and bounded currency selectors for create/update.

## Impact

- Affects `finance/`, especially tenant service contracts, validation, and tests.
- Affects `apps/signal-foundry/internal/api/http/v1routes.yaml`, generated v1 route bindings/models, finance controllers, and controller tests.
- Affects `apps/signal-ui/src/lib/finance/`, `apps/signal-ui/src/pages/FinanceTenants.svelte`, and related UI tests/docs.
- No database migration is expected because tenant name, display currency, and timestamps already exist on persisted tenant records.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
