---
type: Architecture
title: "Product architecture"
description: "Product direction, module ownership, and required supporting infrastructure."
status: stable
sources:
  - resource: "repository product clarification of 2026-11-26 and current app composition"
---

# Architecture

Sumweave is a finance-only system. Its product domain is tenant-aware
financial management: members, accounts, categories and tags, ledger
transactions and transfers, CSV imports, bank connections and sync, current
provider snapshots, balances, reports, and FX.

## Boundaries

- `finance/` owns finance domain services and persistence models and remains
  independent from `runtime/`.
- `apps/sumweave/` composes finance, auth, durable jobs, dispatch, HTTP,
  migrations, telemetry, and embedded UI delivery.
- `apps/sumweave/internal/appdispatch` is the app-owned durable PostgreSQL pub/sub
  transport. It stores multiple topics in one message table and tracks offsets
  independently by topic and consumer group. It is the
  generic internal delivery mechanism for semantic imperative commands and
  factual domain events. Publication assigns and returns one immutable, stable
  message ID and creates no job record. It is the only durable publication,
  scheduling, and delivery path for background commands and events; producers
  do not choose a jobs queue.
- `apps/sumweave/internal/appevents` is the typed domain-event API over
  `appdispatch` and is intentionally retained as a separate typed adapter.
- `apps/sumweave/internal/jobs` adds opt-in product and API visibility to
  selected background commands at consumer registration. A job-observed
  consumer lazily creates a metadata-only lifecycle projection on first
  delivery; its job ID is the dispatch message ID. Background processing that
  needs no product or operational visibility uses `appdispatch` without a job
  record.
- Its application config is app-internal at `internal/config`; `internal/wireup`
  owns explicit command roots. Command and Engine entrypoints pass typed startup
  options, while components receive native values or constructed collaborators.
- `apps/sumweave-ui/` is the personal finance UI. Admin diagnostics and access-token
  management support the app; Chat and model-provider settings are experimental.
- Personal access tokens are app-owned, digest-only credentials for the exact
  documented finance and jobs allowlist. They authenticate as their owner but
  do not copy tenant grants; every finance read and trigger retains current
  membership checks. Session-only remains the default, including runtime and
  token management.
- `apps/sumweave/cmd/swmd` is the separate HTTP-only direct client for that
  allowlist. It calls the existing API rather than a second integration route
  tree and persists credentials only through its private local configuration.
- `runtime/` is experimental generic agent infrastructure: sessions, profiles,
  providers, agent execution, and HTTP APIs.
- `.platform-agents/skills/` is an intentionally empty stageable root until
  finance-specific platform skills are introduced.


## Product direction

Sumweave's product direction is personal budgeting and finance tracking. Current
delivery provides the tracking capabilities described here; budget planning is
not yet implemented. Trading tools, forecasting, and financial advice are not
current capabilities or promised additions.
Generic agent execution, sessions, profiles, Chat, model-provider settings,
and unused inherited functionality are experimental or potential extensions.
Their presence in the executable does not promote them into product scope.
Operational dependencies remain required even where they also serve experiments.

## Detailed references

- [Finance capabilities](product/overview.md) describes current workflows and limits.
- [Background processing](architecture/background-processing.md) defines dispatch,
  jobs, schedules, delivery, and recovery.
- [Provider sync](finance-provider-sync-architecture.md) defines bank identity,
  requested windows, conservative merges, and atomic commits.
- [Persistence](architecture/persistence.md) defines state ownership, secrets,
  migration, and filesystem boundaries.
- [Local development](operations/local-development.md) is the canonical setup guide.
- [Release and deployment](operations/release-deployment.md) links the maintained runbooks.
- [Experiments](experiments/agent-runtime.md) explains the retained generic runtime.

[OpenSpec requirements](development/documentation.md#openspec-and-current-knowledge)
retain granular acceptance scenarios. This page remains the product-direction
authority; the [bundle index](index.md) is the navigation entry.
