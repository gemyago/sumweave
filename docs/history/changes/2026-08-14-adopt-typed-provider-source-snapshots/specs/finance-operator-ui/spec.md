---
type: Historical record
title: "2026-08-14-adopt-typed-provider-source-snapshots: spec"
description: "Historical spec record for 2026-08-14-adopt-typed-provider-source-snapshots."
status: deprecated
superseded_by: /requirements/finance-operator-ui.md
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-08-14-adopt-typed-provider-source-snapshots/specs/finance-operator-ui/spec.md"
    title: "Original openspec/changes/archive/2026-08-14-adopt-typed-provider-source-snapshots/specs/finance-operator-ui/spec.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## ADDED Requirements

### Requirement: Finance Details Expose Current Provider Source Data
The Finance UI SHALL expose current schema-derived provider snapshots as provider source data for linked accounts and provider-synced transactions.

#### Scenario: Account detail lists distinct current snapshot kinds
- **WHEN** a tenant member expands provider source data on a linked finance account
- **THEN** the UI MUST lazily load current provider snapshot metadata for that account
- **AND** it MUST present account and account-balance snapshots as distinct rows when both are available
- **AND** each row MUST identify its snapshot kind, provider object, and capture time

#### Scenario: Transaction detail exposes its complete supported provider item
- **WHEN** a tenant member expands provider source data on a provider-synced transaction
- **THEN** the UI MUST lazily list the current transaction snapshot and allow an explicit detail reveal
- **AND** the revealed data MUST be the sanitized schema-derived provider transaction document returned by the protected API

#### Scenario: Source-data terminology is explicit
- **WHEN** account or transaction provider snapshots are presented
- **THEN** user-facing labels MUST use “Provider source data” or “Provider snapshot” terminology
- **AND** the UI MUST explain that the displayed document is the latest schema-derived provider snapshot rather than a raw HTTP response
- **AND** evidence and raw-payload terminology MUST NOT remain on the affected account or transaction surfaces

#### Scenario: Snapshot access stays bounded and recoverable
- **WHEN** provider source metadata or document loading is pending, empty, or fails
- **THEN** the UI MUST preserve its collapsed-by-default disclosure behavior and show bounded loading, empty, or recoverable error feedback
- **AND** it MUST NOT expose a provider snapshot history timeline
- **AND** it MUST NOT display decrypted credentials, authorization material, or other provider secrets

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
