---
type: Historical record
title: "2026-07-06-update-finance-tenants: spec"
description: "Historical spec record for 2026-07-06-update-finance-tenants."
status: deprecated
superseded_by: /requirements/finance-operator-ui.md
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-07-06-update-finance-tenants/specs/finance-operator-ui/spec.md"
    title: "Original openspec/changes/archive/2026-07-06-update-finance-tenants/specs/finance-operator-ui/spec.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## ADDED Requirements

### Requirement: Tenant Management Supports Updates And Bounded Currency Selection
The Finance tenants route SHALL let operators create and update tenants using predefined valid display-currency choices instead of free-text currency fields.

#### Scenario: Tenant create uses a supported currency selector
- **WHEN** an authenticated operator opens `#/finance/tenants` to create a finance tenant
- **THEN** the create form MUST present display currency as a select control populated from the predefined valid tenant currency-code list
- **AND** the form MUST submit the selected currency code rather than arbitrary free text

#### Scenario: Selected tenant can be updated
- **WHEN** an authenticated tenant member has selected a tenant on `#/finance/tenants`
- **THEN** the UI MUST provide an edit form for the selected tenant name and display currency
- **AND** the display-currency control MUST use the same predefined valid tenant currency-code list as tenant creation
- **AND** saving the form MUST call the tenant update API and refresh the visible selected tenant state after success

#### Scenario: Tenant update failures are recoverable
- **WHEN** tenant update fails because validation, authentication, authorization, or network handling rejects the request
- **THEN** the UI MUST keep the operator on `#/finance/tenants`
- **AND** it MUST show a recoverable error state without losing the current selected tenant context

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
