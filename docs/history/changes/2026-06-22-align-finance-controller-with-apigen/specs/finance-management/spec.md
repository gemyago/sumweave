---
type: Historical record
title: "2026-06-22-align-finance-controller-with-apigen: spec"
description: "Historical spec record for 2026-06-22-align-finance-controller-with-apigen."
status: deprecated
superseded_by: /requirements/finance-management.md
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-06-22-align-finance-controller-with-apigen/specs/finance-management/spec.md"
    title: "Original openspec/changes/archive/2026-06-22-align-finance-controller-with-apigen/specs/finance-management/spec.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## MODIFIED Requirements

### Requirement: Protected Finance HTTP API
The backend application SHALL expose finance APIs through the existing app under `/api/v1/finance/...` using the generated apigen handler contract as the source of truth for request parameters, request bodies, and response serialization.

#### Scenario: Finance APIs are authenticated and tenant-aware
- **WHEN** a caller without a valid authenticated identity calls finance endpoints
- **THEN** the system MUST reject the request as unauthorized
- **AND** when an authenticated caller accesses finance endpoints, tenant-scoped reads and writes MUST remain isolated to the caller's joined tenants

#### Scenario: Finance API contract is generated from OpenAPI
- **WHEN** a finance route accepts path parameters, query parameters, or a JSON request body
- **THEN** those inputs MUST be described in `apps/signal-foundry/internal/api/http/v1routes.yaml` so generated finance params cover the controller's live inputs
- **AND** the finance controller MUST implement the route through generated handler builders rather than a parallel handwritten wrapper that re-parses requests and re-serializes responses

#### Scenario: Finance transaction detail API preserves tenant scope and editor context
- **WHEN** an authenticated tenant member calls `GET /api/v1/finance/tenants/{tenantId}/transactions/{transactionId}`
- **THEN** the system MUST return the selected transaction in camelCase JSON with `providerOriginal` included when present
- **AND** the read flow MUST reject transaction identifiers that do not belong to the selected tenant

#### Scenario: Finance transaction update API preserves tenant scope and reporting semantics
- **WHEN** an authenticated tenant member calls `PATCH /api/v1/finance/tenants/{tenantId}/transactions/{transactionId}`
- **THEN** the system MUST update only the transaction's user-editable reporting fields `description`, `amountMinor`, `effectiveAt`, and nullable `categoryId`
- **AND** the response MUST return the updated transaction in camelCase JSON with `providerOriginal` included when present
- **AND** the update flow MUST reject transaction or category identifiers that do not belong to the selected tenant

#### Scenario: Finance API covers required product areas
- **WHEN** the first finance slice is implemented
- **THEN** the API surface MUST cover tenants, tenant members/invites, accounts, bank connections, transactions including list/detail/create/update flows, categories, tags, dashboard/reporting, FX diagnostics and sync, CSV import preview/confirm/status, and finance-related job deep-linking
- **AND** all operator-facing JSON fields MUST use camelCase

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
