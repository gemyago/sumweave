---
type: Reference
title: "Historical planning and proofs of concept"
description: "Where retained plans and POCs fit relative to current finance direction."
status: stable
sources:
  - resource: "repository .planning, initial finance designs, bank POC notes, and archived OpenSpec changes"
---

# Historical planning and proofs of concept

Current direction is defined by [Architecture](../ARCHITECTURE.md); current
capabilities are described in [Personal finance](../product/overview.md).
These records explain prior work and decisions. They are not a current roadmap.
Older trading-tool ideas and the inherited agent harness are not product scope.

## Superseded design narratives

- [Initial finance design](../finances-management/design.md) retains the broad
  target, suggested routes, fixture shape, historical FX ambitions, and general
  CSV mapping ideas. Current contracts supersede these proposals.
- [Classification PRD](../enrichment-pipeline/classification-phase0-prd.md) and
  [matching PRD](../enrichment-pipeline/transfer-matching-phase0-prd.md) retain
  original requirements and follow-on decisions; current references and OpenSpec
  carry present behavior and acceptance detail.
- [Integration PRD](../integration-layer/integration-layer-prd.md) and
  [draft design](../integration-layer/integration-layer-design.md) retain the
  pre-implementation sketches. Phase 1 is implemented, with differences called
  out in [direct access](../integrations/direct-access.md).
- [Bank connection boundary analysis](../untangling-bank-connection-service-v2-boundary.md)
  is a historical analysis, not an authorized refactoring plan.

## Repository history outside this bundle

- [.planning/PROJECT.md](../../.planning/PROJECT.md),
  [requirements](../../.planning/REQUIREMENTS.md),
  [roadmap](../../.planning/ROADMAP.md), and [state](../../.planning/STATE.md)
  describe the older agent-harness initiative. Their [phase records](../../.planning/phases/)
  are retained unchanged as history, including old references and completion claims.
- [Bank POCs](../../apps/sumweave/doc/financial-poc/index.md) retain CLI sandbox
  experiments, external-provider constraints, and operator observations.
  Production bank linking uses the finance routes and current manual runbooks.
  The [financial-poc-cli scenarios](../../openspec/specs/financial-poc-cli/spec.md)
  describe that experimental CLI, not supported user-facing finance workflows.
- [Archived OpenSpec changes](../../openspec/changes/archive/) are immutable
  historical acceptance/change records. The current knowledge migration does
  not edit or replace them.

History is retained at its established location to preserve source detail and
incoming links. Do not treat historical live-provider names, sandbox state,
external limits, or old setup advice as verified current operational facts.
