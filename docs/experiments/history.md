---
type: Reference
title: "Historical planning and proofs of concept"
description: "Where retained plans and POCs fit relative to current finance direction."
sources:
  - id: repository-policy
    resource: "repository .planning, initial finance designs, bank POC notes, and archived change records"
    title: "repository .planning, initial finance designs, bank POC notes, and archived change records"
  - id: documentation-snapshot
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/docs/experiments/history.md"
    title: "Reviewed documentation checkpoint before this format migration"
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
---

# Historical planning and proofs of concept

Current direction is defined by [Architecture](../ARCHITECTURE.md); current
capabilities are described in [Personal finance](../product/overview.md).
These records explain prior work and decisions. They are not a current roadmap.
Older trading-tool ideas and the inherited agent harness are not product scope.[^documentation-snapshot][^repository-policy]

## Superseded design narratives

- [Initial finance design](../finances-management/design.md) retains the broad
  target, suggested routes, fixture shape, historical FX ambitions, and general
  CSV mapping ideas. Current contracts supersede these proposals.
- [Classification PRD](../enrichment-pipeline/classification-phase0-prd.md) and
  [matching PRD](../enrichment-pipeline/transfer-matching-phase0-prd.md) retain
  original requirements and follow-on decisions; current references and [requirements](../requirements/index.md)
  carry present behavior and acceptance detail.
- [Integration PRD](../integration-layer/integration-layer-prd.md) and
  [draft design](../integration-layer/integration-layer-design.md) retain the
  pre-implementation sketches. Phase 1 is implemented, with differences called
  out in [direct access](../integrations/direct-access.md).
- [Bank connection boundary analysis](../untangling-bank-connection-service-v2-boundary.md)
  is a historical analysis, not an authorized refactoring plan.

## Historical records

- [.planning/PROJECT.md](../../.planning/PROJECT.md),
  [requirements](../../.planning/REQUIREMENTS.md),
  [roadmap](../../.planning/ROADMAP.md), and [state](../../.planning/STATE.md)
  describe the older agent-harness initiative. Their [phase records](../../.planning/phases/)
  are retained unchanged as history, including old references and completion claims.
- [Bank POCs](../../apps/sumweave/doc/financial-poc/index.md) retain CLI sandbox
  experiments, external-provider constraints, and operator observations.
  Production bank linking uses the finance routes and current manual runbooks.
  The [financial-poc-cli scenarios](../history/requirements/financial-poc-cli.md)
  describe that experimental CLI, not supported user-facing finance workflows.
- [Archived changes](../history/changes/index.md) retain historical proposals,
  designs, acceptance deltas, tasks, and review reports in this bundle. Their
  deprecated status and source snapshots distinguish them from current requirements.

The `.planning/` and bank POC links above are repository sources outside this
bundle; OKF consumers must not read outside the bundle through the filesystem.
Change records are retained under `docs/history/` with repaired repository links. Do not treat historical live-provider names, sandbox state,
external limits, or old setup advice as verified current operational facts.[^documentation-snapshot]

[^documentation-snapshot]: Reviewed documentation checkpoint, preserved as the source of this section; format and navigation changes do not establish execution results.

[^repository-policy]: repository .planning, initial finance designs, bank POC notes, and archived change records.
