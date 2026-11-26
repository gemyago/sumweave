---
type: Historical record
title: "2026-09-06-add-phase0-transaction-classification: spec"
description: "Historical spec record for 2026-09-06-add-phase0-transaction-classification."
status: deprecated
superseded_by: /requirements/finance-management.md
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-09-06-add-phase0-transaction-classification/specs/finance-management/spec.md"
    title: "Original openspec/changes/archive/2026-09-06-add-phase0-transaction-classification/specs/finance-management/spec.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## ADDED Requirements

### Requirement: Category Removal Honors Classification References
The finance catalog SHALL logically hide categories while preserving historical ledger references and SHALL prevent any hide path from hiding a category that is referenced by a live classification rule.

#### Scenario: Unreferenced category is removed
- **WHEN** a tenant member removes an existing same-tenant category that no live classification rule references
- **THEN** the category MUST be logically hidden rather than physically deleted
- **AND** existing transaction references and historical ledger records MUST remain intact.

#### Scenario: Referenced category removal is blocked
- **WHEN** the public category-removal API or an internal category-hide caller targets a category referenced by one or more live classification rules
- **THEN** the hide MUST be rejected before the category changes
- **AND** the public API MUST return `409` with code `category_referenced_by_classification_rules` and the referencing `ruleIds`.

## MODIFIED Requirements

### Requirement: Successful Requested-Window Progress Is Atomic
The finance module SHALL commit the writes for a successful requested window, its successful provider sync state checkpoint, and its bank-sync-window completion event in one database transaction.

#### Scenario: Window writes checkpoint and event succeed together
- **WHEN** connector fetch, diff planning, apply planning, successful checkpoint persistence, and completion-event publication succeed for a requested window
- **THEN** accounts, balances, transactions, matches, typed provider snapshots, the successful chunk state, and the completion event MUST commit atomically
- **AND** the state MUST record the attempted window, success time, run and dispatch identity, and aggregate stats
- **AND** the event MUST record the durable connection's tenant and connection, the requested window's unchanged start and exclusive end, and source sync message ID.

#### Scenario: Successful checkpoint persistence fails
- **WHEN** the success journal row cannot be persisted during requested-window apply
- **THEN** all finance writes and the completion event for that requested window MUST roll back
- **AND** the orchestrator MUST return a failure rather than report uncheckpointed progress.

#### Scenario: Completion event publication fails
- **WHEN** transaction-bound publication of the window completion event fails
- **THEN** all finance writes and the successful checkpoint for that requested window MUST roll back
- **AND** the orchestrator MUST return a failure through the existing bank-sync retry policy.

#### Scenario: Fetch or apply fails
- **WHEN** provider fetch, diff preparation, or transactional apply fails for a requested window
- **THEN** no partial finance writes, successful checkpoint, or completion event for that window may commit
- **AND** the orchestrator MUST append a failed attempt state containing the requested window, dispatch identity, and sanitized error summary.

#### Scenario: A later chunk fails after earlier chunks succeeded
- **WHEN** an oldest-first orchestration commits one or more chunks and a later chunk fails
- **THEN** the earlier successful chunk states, finance writes, and completion events MUST remain durable
- **AND** the next automatic target plan MUST derive its checkpoint from the failed window start before applying the existing recent-refresh rule.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
