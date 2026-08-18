## Why

Production bank-connection sync currently bypasses the provider sync orchestrator and calls technical connectors through a legacy provider adapter. As a result, persisted connector identity, sync-state journal checkpoints, general window chunking, conservative diff planning, and failed-chunk resume behavior described by the finance architecture are not active in real jobs.

## What Changes

- Route manual and scheduled bank-connection sync jobs through the provider sync orchestrator and requested-window executor.
- Build each provider connection reference from the persisted product provider, technical connector, provider reference, and connection identity instead of re-deriving the connector from provider-specific branches.
- Add a concrete oldest-first window chunk policy with contiguous, half-open requested windows spanning at most 30 calendar days, while preserving optional target-window bounds from the existing API and job input.
- Make the provider window sync store support first-time account and transaction imports, deriving tenant ownership from the durable bank connection rather than existing transaction data.
- Commit successful chunk application and its authoritative journal progress atomically so at-least-once job delivery cannot commit finance writes without their checkpoint.
- Track created-account statistics so the existing sync job result continues to report imported accounts accurately when a target is split into multiple chunks.
- Preserve current scheduling metadata, connection status/error visibility, typed provider snapshots, conservative matching, and user-edit merge behavior around the orchestrated execution.
- **BREAKING** Remove the internal connector-backed `BankConnectionProvider.Sync` adapter, legacy `ProviderSyncResult` conversion/application path, and product-provider-to-connector sync selection once the orchestrated path is active.
- Keep existing finance HTTP routes, request/response JSON, durable job type, schedule behavior, and UI-facing connection fields unchanged.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `finance-management`: Require production bank sync execution to use persisted connector identity, orchestrator-owned target planning and chunking, the requested-window diff/apply path, retry-safe sync-state journaling, and first-sync-capable persistence.

## Impact

- Affects finance module composition, bank-sync service dependencies, provider sync orchestration, window policies, secret handoff, provider sync statistics, and provider sync persistence.
- Affects finance durable-job integration tests while preserving the public job and HTTP contracts.
- Removes obsolete internal v1 sync declarations and application code after cutover.
- Requires no new external dependency and no compatibility layer because the project is early alpha.
