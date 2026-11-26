---
type: Historical record
title: "2026-09-09-add-classification-rule-tags: spec"
description: "Historical spec record for 2026-09-09-add-classification-rule-tags."
status: deprecated
superseded_by: /requirements/finance-management.md
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-09-09-add-classification-rule-tags/specs/finance-management/spec.md"
    title: "Original openspec/changes/archive/2026-09-09-add-classification-rule-tags/specs/finance-management/spec.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## ADDED Requirements

### Requirement: Tag Hiding Honors Classification References
The finance catalog SHALL prevent internal tag hiding while any live classification rule references the tag and SHALL preserve historical transaction tag assignments.

#### Scenario: Referenced tag hiding is blocked
- **WHEN** a catalog tag-hide caller or a direct persistence save attempts to hide a tag referenced by a live classification rule
- **THEN** the operation MUST fail before changing the tag
- **AND** the domain failure MUST identify the referencing rule IDs through the catalog service
- **AND** the tag and all rule and transaction associations MUST remain intact.

#### Scenario: Unreferenced tag can be hidden
- **WHEN** an otherwise authorized hide operation targets a tag without live rule references
- **THEN** the system MUST logically hide the tag while retaining historical transaction assignments
- **AND** later rule create/update requests MUST reject that hidden tag.

#### Scenario: Rule changes release the hide restriction
- **WHEN** every rule referencing a tag is deleted or updated to remove that tag
- **THEN** the tag MUST cease to be blocked by classification references
- **AND** existing transaction tag assignments MUST remain unchanged.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
