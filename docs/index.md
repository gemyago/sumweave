---
okf_version: "0.3"
title: Sumweave knowledge
description: Current personal finance behavior, architecture, operations, verification, and experimental boundaries.
---

# Start here

Sumweave's direction is personal budgeting and finance tracking. Current alpha
features cover tracking; budget planning is not yet implemented. This OKF 0.3
bundle explains current behavior and limits. [Architecture](ARCHITECTURE.md) remains the authority
for product direction. Generic agents, Chat, and model-provider settings are
experimental; their presence does not imply a finance capability.

- [Personal finance capabilities](product/overview.md) - Current workflows,
  reporting semantics, and product limits.
- [Local development](operations/local-development.md) - Install dependencies,
  bootstrap PostgreSQL, and run the API, worker, and UI.
- [Domain terminology](domain-terminology.md) - Canonical finance, credential,
  command, event, and job vocabulary.

# Finance and interfaces

- [Finance topics](product/index.md) - Capabilities, classification, and transfer matching.
- [Classification](product/classification.md) - Ordered category and additive-tag
  rules, timestamp ranges, and accepted limits.
- [Transfer matching](product/transfer-matching.md) - Four deterministic pair rules,
  mutual uniqueness, persistence, and manual correction.
- [Direct access](integrations/direct-access.md) - Personal token lifecycle, exact
  route allowlist, idempotency, and HTTP-only CLI.

# Architecture and operations

- [Product architecture](ARCHITECTURE.md) - Product direction, module ownership,
  and required supporting infrastructure.
- [Architecture topics](architecture/index.md) - Dispatch/jobs, persistence,
  and provider synchronization.
- [Provider sync](finance-provider-sync-architecture.md) - Link identity,
  bounded sync windows, conservative merge, and atomic progress.
- [Operations](operations/index.md) - Local development, HTTPS, release, and deployment.

# Development and verification

- [Development](development/index.md) - Go/testing conventions, Nx checks, and documentation.
- [Manual E2E](manual-e2e/index.md) - Focused reproducible finance verification journeys.
- [Documentation conventions](development/documentation.md) - OKF 0.3 metadata,
  lifecycle, navigation, and retained OpenSpec workflow.

# Experiments and history

- [Experimental runtime](experiments/agent-runtime.md) - Retained agent, Chat,
  model-provider functionality and operational boundaries.
- [Historical material](experiments/history.md) - Superseded designs, harness
  planning, bank POCs, and archived changes.

# Repository references outside the bundle

These links work in the repository but are external to this `docs/` bundle.
OKF consumers must not read beyond the bundle from the local filesystem.

- [Contributor entry](../CONTRIBUTING.md) - Contribution and OpenSpec workflow.
- [Formal OpenSpec scenarios](../openspec/specs/) - Granular acceptance requirements.
- [Active changes and archive](../openspec/changes/) - Ongoing and historical change records.
- [OpenAPI](../apps/sumweave/internal/api/http/v1routes.yaml) - Exact HTTP contract.
- [Finance smoke suite](../tests/README.md) - Automated API and browser setup.
- [Release build](../build/README.md) and [deployment](../deploy/README.md) - Maintained runbooks.
- [UI design system](../apps/sumweave-ui/DESIGN.md) - Module-specific visual conventions.
