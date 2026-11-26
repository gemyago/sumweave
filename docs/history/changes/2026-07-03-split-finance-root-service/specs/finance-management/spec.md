---
type: Historical record
title: "2026-07-03-split-finance-root-service: spec"
description: "Historical spec record for 2026-07-03-split-finance-root-service."
status: deprecated
superseded_by: /requirements/finance-management.md
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-07-03-split-finance-root-service/specs/finance-management/spec.md"
    title: "Original openspec/changes/archive/2026-07-03-split-finance-root-service/specs/finance-management/spec.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## ADDED Requirements

### Requirement: Finance Public Services Are Focused
The finance module SHALL expose public services by focused product responsibility instead of requiring app, controller, job, CLI, or fixture callers to depend on one broad root finance service.

#### Scenario: Finance module exposes focused services
- **WHEN** the finance module is composed through `finance.New`
- **THEN** the returned `Finance` instance MUST expose focused public services for tenant management, catalog management, ledger transactions, reporting, FX synchronization and diagnostics, CSV imports, bank-link workflows, and bank-sync workflows
- **AND** each exposed service MUST contain only the methods needed for its responsibility
- **AND** the existing bank-link service MUST remain separate from bank-sync listing, deletion, scheduling, and execution concerns

#### Scenario: App callers depend on focused services
- **WHEN** backend app registration wires finance HTTP controllers, finance jobs, callback handlers, CLI bootstrap, or fixture bootstrap
- **THEN** each caller MUST receive the focused finance service or consumer-defined interface required for its workflow
- **AND** active app code MUST NOT depend on a broad root `finance.Service` facade for unrelated finance methods

#### Scenario: Service dependencies stay narrow
- **WHEN** a focused finance service is constructed
- **THEN** it MUST accept only the store interfaces, providers, ciphers, clocks, ID generators, enqueuers, schedulers, and loggers required by that service
- **AND** store interfaces MUST be defined by the consuming service boundary
- **AND** new persistence needs MUST use dedicated responsibility-specific stores or narrow interfaces instead of extending the legacy broad persistence store as a god object

#### Scenario: External contracts stay unchanged during service split
- **WHEN** the root finance service is decomposed into focused services
- **THEN** existing finance HTTP routes, JSON field names, job types, database tables, and documented domain behavior MUST remain unchanged
- **AND** any required caller change MUST be limited to Go wiring and dependency interfaces inside the repository

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
