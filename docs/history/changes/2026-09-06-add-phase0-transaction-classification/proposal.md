---
type: Historical record
title: "2026-09-06-add-phase0-transaction-classification: proposal"
description: "Historical proposal record for 2026-09-06-add-phase0-transaction-classification."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-09-06-add-phase0-transaction-classification/proposal.md"
    title: "Original openspec/changes/archive/2026-09-06-add-phase0-transaction-classification/proposal.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## Why

Uncategorized finance transactions currently have no deterministic, repeatable
classification path, leaving bank-sync and historical cleanup workflows dependent
on manual category assignment. Phase 0 adds an explainable ordered-rule foundation
that preserves every existing category while fitting the current appdispatch,
jobs, finance, and tenant-aware UI architecture.

## What Changes

- Add tenant-owned ordered classification rules with exact and literal-contains
  description matching, first-match precedence, and create, list, update, delete,
  and adjacent move operations.
- Add one shared range classifier for eligible uncategorized booked `regular`,
  `expense`, `income`, and `refund` transactions from manual, CSV, and provider
  sources, with bounded batching, conditional writes, per-attempt rule snapshots,
  and diagnostic counts.
- Add an authenticated explicit classification API that publishes a job-observed
  semantic command and returns the appdispatch message ID as the future job ID.
- Emit a durable bank-sync-window completion event atomically with each successful
  window's ledger writes and checkpoint, then classify that committed range through
  an ordinary event subscriber independently of the bank-sync job outcome.
- Block logical category removal while live classification rules reference the
  category and return the referencing rule IDs for actionable UI feedback.
- Add a Finance rules management route, ordering controls, explicit date-range
  classification with existing job lifecycle feedback, and optional rule creation
  after a successful manual transaction category assignment.
- Add deterministic synthetic-sync manual E2E coverage and repeat fix/retest loops
  until classification and UI findings are clean.

## Capabilities

### New Capabilities

- `transaction-classification`: Tenant rule management, deterministic matching and
  eligibility, explicit job-observed runs, committed-window automatic triggers,
  retry behavior, counts, and safety guarantees.

### Modified Capabilities

- `finance-management`: Category removal must honor classification-rule references,
  and each successful provider-sync window must atomically retain its classification
  completion event with ledger writes and checkpoint state.
- `finance-operator-ui`: Finance gains rule management, manual-assignment rule
  creation, and explicit classification controls with durable-job feedback.

## Impact

- Finance domain, focused services, dedicated persistence stores, GORM migration,
  provider window-apply transaction seam, and classification command contracts.
- Backend app typed events, command/event registration, worker lifecycle, finance
  OpenAPI routes/controllers, generated route code, and existing jobs integration.
- Finance SPA routing, shell navigation, API client, rule/transaction/category
  screens, job-status components, wireframe, and manual E2E documentation.
- PostgreSQL is the only affected database; no compatibility migration or new
  third-party dependency is required.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
