---
type: Reference
title: "Experimental agent runtime"
description: "Scope and operational boundaries for retained agent, Chat, and model-provider functionality."
sources:
  - id: repository-policy
    resource: "repository approved product clarification and cleanup of 2026-11-26, root/runtime instructions, and app/UI composition"
    title: "repository approved product clarification and cleanup of 2026-11-26, root/runtime instructions, and app/UI composition"
  - id: documentation-snapshot
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/docs/experiments/agent-runtime.md"
    title: "Reviewed documentation checkpoint before this format migration"
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T12:06:29+00:00" }
---

# Experimental agent runtime

Generic agent execution, sessions, profiles, Chat, model-provider settings,
workspace tools, and unused template functionality are experimental extensions.
Sumweave's direction is personal budgeting and finance tracking; current delivery
covers tracking. Experimental code lives in `runtime/`, the app's protected
`/api/v1/runtime/*` routes, the UI's Chat/settings screens, and the agent harness
under `tests/agent/`. Bank POC CLI code remains under
`apps/sumweave/cmd/sumweave/finance_poc*`; it is diagnostic code rather than the
supported bank-linking workflow.[^documentation-snapshot][^repository-policy]

Finance requires auth, appdispatch, optional observed jobs, PostgreSQL, API/UI
delivery, release, and deployment support. Current app startup and migrations
also require the configured database-backed runtime sessions, profiles, and
provider settings. Keep their DSN and table-prefix configuration while that
composition exists; [persistence](../architecture/persistence.md) documents it.
The packaged `.platform-agents/skills` root is empty until finance-specific skills
are requested; harnesses use `.agents/skills` directly. Workspace shell execution
is disabled by default and uses an ephemeral workspace when enabled.[^repository-policy]

For deliberate experimental work, consult the repository's
[runtime instructions](../../runtime/AGENTS.md),
[runtime OpenAPI](../../runtime/internal/agentapi/openapi.yaml),
[UI architecture](../../apps/sumweave-ui/doc/architecture.md), and
[agent test setup](../../tests/README.md#running-agent-integration-tests).
Use the current [finance overview](../product/overview.md) and
[bank-linking runbook](../manual-e2e/bank-linking-e2e.md) for product behavior.
Retired plans and POC notes remain in Git history.[^repository-policy]

[^documentation-snapshot]: Reviewed documentation checkpoint for the experimental scope and retained code locations.
[^repository-policy]: Approved product clarification and cleanup of 2026-11-26, root/runtime instructions, and current app/UI composition.
