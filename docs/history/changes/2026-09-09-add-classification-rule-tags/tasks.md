---
type: Historical record
title: "2026-09-09-add-classification-rule-tags: tasks"
description: "Historical tasks record for 2026-09-09-add-classification-rule-tags."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-09-09-add-classification-rule-tags/tasks.md"
    title: "Original openspec/changes/archive/2026-09-09-add-classification-rule-tags/tasks.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## 1. Rule tag persistence and lifecycle

- [x] 1.1 Persist optional classification rule tags. Follow TDD (write failing behavior tests -> implement -> verify) for the `TagIDs` domain field, GORM association model/migrator registration, batched rule-tag hydration, atomic create/replace/delete, empty-set clearing, and preserved ordering/filtering. Use PostgreSQL persistence tests and keep migration coverage to one shallow smoke scenario.
- [x] 1.2 Validate tenant-owned rule tag selections. Follow TDD (write failing tests -> implement -> verify) for create/update service parameters, required tag lookup dependency and wiring, duplicate/blank IDs, missing/hidden/cross-tenant tags, and rejected writes leaving stored rules intact; regenerate affected Mockery mocks.
- [x] 1.3 Protect tags referenced by classification rules. Follow TDD (write failing tests -> implement -> verify) for dedicated rule-tag reference lookup, catalog hide errors with rule IDs, persistence-level hide protection, release after retarget/delete, and preservation of historical transaction tags.

## 2. Atomic classification assignment

- [x] 2.1 Apply each winning rule's category and optional tags atomically. Follow TDD (write failing service and persistence tests -> implement -> verify) for conditional eligibility, additive conflict-safe tag insertion, overlap/category-only cases, rollback on failure, first-rule-only tags, unavailable-tag terminal failures versus operational errors, unchanged categorized-row exclusion, and per-transaction counts/retries. Wire required dependencies and mocks, and update the classification PRD/design with the schema and chosen behavior in the same task.

## 3. Application API

- [x] 3.1 Expose rule tags through the existing classification endpoints. Follow TDD (write failing registered-route tests -> implement -> verify) for optional request `tagIds`, required read arrays, omitted/empty replacement semantics, invalid-tag response mapping, tenant isolation, and unchanged minimal mutation responses; update OpenAPI, regenerate backend models/validation, and update controller mappings.

## 4. Rule editing and transaction prompts

- [x] 4.1 Add rule tag transport and editing controls. Follow TDD for Finance API serialization/mapping and form behavior, then the visual flow (adjust design -> verify) for optional tag selection in `FinanceRuleCreationForm.svelte` and `FinanceRules.svelte`, named tag badges in rule lists, empty selections, recoverable catalog/selection errors, and independent draft values. Update the corresponding Rules wireframe behavior alongside these changes.
- [x] 4.2 Replace shared-list automatic rule expansion with a compact persistent offer. Follow TDD for category -> tag/description save -> open -> rule submission, saved-value defaults, no automatic focus/scroll, dismissal, category clearing, replacement, tenant/result scope changes, and failed saves; implement the collapsed/editing state and follow the visual flow (adjust design -> verify). Update shared-list wireframe and E2E guidance alongside the change; apply the design's verification flow to ledger, dashboard, and account activity hosts as part of completion.
- [x] 4.3 Apply the compact offer to the full transaction create/edit flow. Follow TDD for saved category/tag prefilling, subsequent tag saves, unsaved form isolation, pending-save opening guard, dismissal, replacement, clearing, navigation scope, and recoverable rule-save errors; implement the full-editor behavior and follow the visual flow (adjust design -> verify). Update editor wireframe/E2E guidance and complete the design's required UI review flow as part of this task.
- [x] 4.4 Fix create-route follow-up saves. Follow TDD for exactly one initial create followed by an update of the created transaction, preserving its compact offer and opening it with the latest saved description and tags.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
