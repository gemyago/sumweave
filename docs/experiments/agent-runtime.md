---
type: Reference
title: "Experimental agent runtime"
description: "Scope and operational boundaries for retained agent, Chat, and model-provider functionality."
status: stable
sources:
  - resource: "repository product clarification of 2026-11-26, runtime module instructions, and app/UI composition"
---

# Experimental agent runtime

Generic agent execution, sessions, profiles, model providers, Chat, workspace
tools, and unused inherited features are experimental or unused functionality
for possible future extensions. Sumweave's product is personal finance tracking.
Running code and historical harness plans do not make agents or trading tools
product commitments.

The executable currently composes `runtime/` with the app and exposes protected
`/api/v1/runtime/*` routes; the UI retains Chat and model-provider settings.
Regular and ACP stdio execution profiles, session listing/title generation,
SSE runs, and provider configuration belong to this experimental surface.
PATs cannot access it. Runtime OpenAPI and its generated UI types remain
implementation contracts for anyone deliberately working on those experiments.

Experimental scope does not authorize removal of dependencies. The current app
migrates database-backed runtime sessions, profiles, and model-provider settings;
startup and deployment configuration still need the corresponding DSN and table
prefix. [Persistence](../architecture/persistence.md) describes that operational
boundary. Shared auth, dispatch, jobs, PostgreSQL, API/UI delivery, releases,
and deployment remain required support for the finance app. Admin access-token
management and finance diagnostics are product support, not generic agent features.

The packaged `.platform-agents/skills/` root is intentionally empty and accepts
only explicitly requested finance-relevant platform skills. Harness-agnostic
repo skills live separately under `.agents/skills/`; the in-product runtime does
not load those by default. Workspace shell execution is disabled by default;
when explicitly enabled it uses an ephemeral writable workspace.

Repository references outside this bundle:

- [Runtime instructions](../../runtime/AGENTS.md) retain actionable API,
  persistence, generation, and testing details.
- [Runtime OpenAPI](../../runtime/internal/agentapi/openapi.yaml)
- [UI implementation](../../apps/sumweave-ui/doc/architecture.md) and
  [screen reference](../../apps/sumweave-ui/ui-wireframe.md) identify experimental sections.
- [Experimental agent tests](../../tests/README.md#running-agent-integration-tests)

[Historical material](history.md) retains harness plans and bank POCs without
promoting them into an active roadmap.
