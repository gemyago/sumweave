---
type: Historical record
title: "2026-06-23-fix-bank-provider-token-connection: spec"
description: "Historical spec record for 2026-06-23-fix-bank-provider-token-connection."
status: deprecated
superseded_by: /requirements/finance-management.md
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-06-23-fix-bank-provider-token-connection/specs/finance-management/spec.md"
    title: "Original openspec/changes/archive/2026-06-23-fix-bank-provider-token-connection/specs/finance-management/spec.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## MODIFIED Requirements

### Requirement: Finance Sync, Secrets, And Imports
The finance module SHALL support secure provider linking plus explicit async sync/import workflows.

#### Scenario: Provider credentials are encrypted and redacted
- **WHEN** bank connections or provider sessions are stored and later surfaced through logs, APIs, jobs, or admin screens
- **THEN** provider credentials and decrypted secrets MUST be encrypted at rest, MUST NOT be logged or returned by default, and MUST keep enough metadata to expose re-authentication needs

#### Scenario: Supported bank linking methods are explicit
- **WHEN** a tenant member starts a bank-linking workflow
- **THEN** the system MUST expose monobank as a token-based bank connection option
- **AND** the system MUST expose PKO as a redirect/SCA bank connection option implemented through Enable Banking
- **AND** token linking MUST reject any bank provider other than monobank before storing secrets or calling a provider
- **AND** redirect/SCA linking MUST reject any bank provider other than PKO before storing secrets or calling a provider
- **AND** local or fake-provider PKO redirect/SCA validation MUST NOT require a browser-trusted HTTPS certificate when the callback URL uses localhost or a loopback address

#### Scenario: PKO redirect link is completed from a pending start
- **WHEN** a tenant member returns from Enable Banking SCA for PKO with a valid state and code
- **THEN** the system MUST resolve the server-side pending PKO link start for the same tenant and actor
- **AND** the system MUST create the bank connection through the Enable Banking finish-link path
- **AND** the system MUST consume or expire the pending link start so the same state cannot create duplicate connections

#### Scenario: Bank sync remains idempotent and job-backed
- **WHEN** a tenant member triggers or schedules a bank sync for a linked monobank or PKO connection
- **THEN** the system MUST execute the sync through durable finance jobs, persist normalized accounts/transactions, balance snapshots, provider-original identifiers, and raw payloads, and deduplicate by provider/connection/account/provider-transaction identity plus safe fallback fingerprints when needed

#### Scenario: Scheduled sync management scope stays explicit
- **WHEN** finance scheduling features are surfaced
- **THEN** tenant-facing finance workflows MUST manage per-connection bank sync schedules, schedule state, and next/last sync visibility from finance connection screens
- **AND** admin/diagnostics workflows MUST provide sanitized cross-cutting visibility plus global FX schedule controls without becoming the primary tenant workflow for bank-connection schedule editing

#### Scenario: CSV import uses preview then durable execution
- **WHEN** a tenant member imports account or transaction CSV data
- **THEN** the system MUST require header detection, mapping confirmation, validation preview, duplicate/would-create visibility, and explicit confirmation before it enqueues `finance.csv_import` or `finance.account_import`
- **AND** durable import execution MUST expose progress, result summary, and rejected-row visibility

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
