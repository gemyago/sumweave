---
type: Reference
title: "Historical documentation"
description: "Retained change records and retired requirements, separate from current delivery."
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:28:13+01:00" }
sources:
  - id: historical-inventory
    resource: "repository archived change and experimental requirement records at 3694d916ce230a3e3e0f6b98cb014b04abef5eec"
    title: "Archived documentation inventory before relocation"
---

# Historical documentation

The [change archive](changes/index.md) preserves proposals, designs, requirements
deltas, task lists, review reports, and recorded outcomes from earlier work.
Every record is deprecated: its original body is a historical source, not a new
instruction, current product plan, or record of tests performed in this migration.[^historical-inventory]

Current [requirements](../requirements/index.md) and [topic references](../product/index.md)
state the accepted contracts and existing implementation limits. Budget planning
remains a direction rather than a delivered feature; generic agents remain
experimental. [Earlier designs and POCs](../experiments/history.md) link retained
sources in this bundle and explicitly labeled repository history outside it.

The [financial POC requirements](requirements/financial-poc-cli.md) describe bank
CLI experiments rather than supported finance workflows. Old tool invocations,
absolute workspace paths, external provider observations, and review claims in
source bodies apply to their historical context. They are not operational advice.

## Retired entry points

These deprecated paths remain available for old links and point to their current
replacements:

* [Implementation snapshot](../IMPLEMENTATION_STATUS.md) - Current capabilities are maintained by topic.
* [Database state plan](../database-backed-state-plan.md) - The maintained persistence reference replaces this plan.
* [Classification design](../enrichment-pipeline/classification-phase0-design.md) - The detailed current reference replaces this design path.
* [Matching design](../enrichment-pipeline/transfer-matching-phase0-design.md) - The detailed current reference replaces this design path.

[^historical-inventory]: Original archived Markdown records and experimental CLI scenarios, retained with per-file pinned source URLs.
