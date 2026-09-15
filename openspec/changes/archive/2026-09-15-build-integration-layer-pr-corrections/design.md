## Context

`build-integration-layer` was archived at
`openspec/changes/archive/2026-09-15-build-integration-layer/` and submitted as
PR #28. The archive workflow has no reopen command (`openspec change` only
shows/lists active changes), so this smallest linked active correction change
preserves archive history while tracking post-archive review and CI repairs.

The reviewed defects violate existing Phase 1 requirements rather than changing
the approved product design: the direct client must protect credentials and
bound waiting, the configure flag is already presence-only, application
internal errors must be safely logged, and successful token mutation must
update visible metadata.

## Goals / Non-Goals

**Goals:**

- Repair only the concrete PR #28 blockers and the current required CI failure.
- Keep bearer values and secrets from being sent to unsafe redirect targets or
  logged by correction code and tests.
- Preserve existing request routes, OpenAPI contracts, token lifecycle rules,
  retry semantics, dashboard behavior, and archive history.

**Non-Goals:**

- Reopen, rewrite, or amend the archived integration-layer change.
- Add an OAuth flow, SDK, CLI option carrying a token argument, a new API route,
  schema migration, or a global test-timeout increase.
- Implement the owner's two explicitly optional observations in this correction
  round: expired-token UI revocation and formal OpenAPI modeling of
  `Idempotency-Key`.

## Decisions

### Use one linked correction change, not an archive rewrite

The installed repository-supported CLI creates active changes with
`openspec new change` and offers no archive reopen operation. This change links
to the archived slug in its artifacts and limits new OpenSpec spec deltas to
the two direct-client guarantees that need clearer durable requirements.

### Enforce security and deadline boundaries at the direct-client transport

The direct client will validate every redirect destination before a follow-up
request can forward credentials, preserving the existing loopback-only HTTP,
normal TLS verification, and finite-timeout rules. `job wait` will derive a
context bounded by its overall timeout so each `GetJob` request and sleep share
the same termination boundary. Focused HTTP-server tests will prove no
credential reaches an unsafe redirect target and that a blocking poll observes
context cancellation.

### Retain narrow existing contracts elsewhere

The configure command will distinguish a bare `--token-stdin` opt-in from any
explicit value and will reject `=true`, `=false`, and arbitrary values before
reading stdin or changing configuration. Middleware will return `401` only for
the known invalid-access-token sentinel; it will log and map unexpected access
token validation failures to the existing safe `500 internal_error` response.
The UI will refresh server metadata after rotation while retaining the one-time
replacement secret only in page state.

The dashboard correction is test-only. It will make the hierarchy test wait on
one deterministic rendered readiness condition or use a narrowly justified
test-local limit if the normal parallel suite demonstrably needs it; it must
not weaken global timeouts or alter Finance behavior.

## Risks / Trade-offs

- [A redirect policy mutates a caller-provided HTTP client or permits a custom
  policy to weaken validation] -> Apply an owned, enforced client policy and
  test both redirect rejection and ordinary allowed requests.
- [A deadline context breaks fake-clock polling tests] -> Retain the existing
  injected clock/sleep behavior for deterministic state transitions and add a
  real blocking-handler cancellation case for the request boundary.
- [Unexpected database errors become public details] -> Log the wrapped error
  with correlation context and return only the existing safe error envelope.
- [A token refresh clears the one-time secret] -> Keep issued-secret state
  separate from reloaded metadata and cover both in the page test.
- [The CI correction masks a real dashboard regression] -> Preserve all current
  hierarchy assertions and repair only their readiness/timing arrangement.
