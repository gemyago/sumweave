---
type: Historical record
title: "2026-06-24-add-provider-sync-connection-resolution: spec"
description: "Historical spec record for 2026-06-24-add-provider-sync-connection-resolution."
status: deprecated
superseded_by: /requirements/finance-management.md
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-06-24-add-provider-sync-connection-resolution/specs/finance-management/spec.md"
    title: "Original openspec/changes/archive/2026-06-24-add-provider-sync-connection-resolution/specs/finance-management/spec.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## ADDED Requirements

### Requirement: Provider Sync V2 Coordinator Resolves Technical Connectors
The finance module SHALL resolve the provider sync v2 fetch connector from the persisted bank connection's technical `ConnectorID` before it performs provider fetch orchestration.

#### Scenario: Monobank connections resolve their direct connector
- **WHEN** provider sync v2 coordinates a bank connection whose `ConnectorID` is `monobank`
- **THEN** the coordinator MUST resolve the `monobank` technical connector before fetch begins
- **AND** it MUST use that resolved connector instead of branching on product-provider-specific sync code

#### Scenario: PKO connections sync through Enable Banking
- **WHEN** provider sync v2 coordinates a bank connection whose product provider is `pko` and whose `ConnectorID` is `enable-banking`
- **THEN** the coordinator MUST resolve the `enable-banking` technical connector for the fetch step
- **AND** PKO MUST remain modeled as a product provider composed through the Enable Banking connector rather than as a user-visible technical connector

#### Scenario: Unknown connectors fail before provider fetch
- **WHEN** provider sync v2 coordinates a bank connection whose `ConnectorID` is empty, unknown, or not configured in the runtime registry
- **THEN** the coordinator MUST fail before any provider fetch call is attempted
- **AND** the failure MUST identify connector resolution as the cause without exposing secrets or raw provider payload content

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
