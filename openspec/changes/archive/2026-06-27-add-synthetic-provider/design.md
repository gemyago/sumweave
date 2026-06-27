## Context

The finance module already has provider sync v2 domain types, a connector registry, target-window planning, a window sync executor seam, and an append-only provider sync state journal. Monobank and PKO/Enable Banking exist as real provider paths, while v2 execution is still becoming the durable foundation for provider fetch, diff, and apply.

This change adds a third provider for generated finance data. I will call it `synthetic`, because the provider creates realistic-enough provider observations without pretending to be a test fixture or an external bank. The iteration should stay core-only: it must not add UI, public HTTP setup flows, or product-facing copy yet.

The key missing infrastructure is synthetic-owned state for generation. Instead of adding a provider-specific blob to the common provider sync journal, this change should add dedicated synthetic-provider storage for static account configuration and mutable generation history. The existing `ProviderSyncState` journal should continue to capture generic attempted windows, outcomes, job/run identifiers, errors, and aggregate stats.

## Goals / Non-Goals

**Goals:**

- Add `synthetic` as both product provider and technical connector ID in the v2 provider model.
- Add a finance-internal core-only linking seam for synthetic connections that accepts account names and currencies.
- Persist synthetic link configuration as provider-owned connection data without exposing it through UI/API yet.
- Add dedicated synthetic-provider storage that can round-trip configured accounts and generation history.
- Let the synthetic provider load and update its own generation state on the successful fetch path without changing the common provider sync journal schema.
- Generate account, balance, transaction, and raw payload observations through the v2 connector contract.
- Implement the requested generation rule:
  - first run for a normalized requested window: 1 to 2 transactions per account per UTC day in that normalized span
  - repeated run for the same normalized requested window: 1 to 3 transactions per account for the normalized window's last UTC day only
- Make duplicate configured accounts safe by assigning a stable synthetic account key per configured entry instead of deriving identity from display name/currency alone.
- Keep randomness testable with injected clock/random sources where needed.

**Non-Goals:**

- No finance UI for configuring synthetic accounts.
- No public HTTP endpoint or OpenAPI enum exposure for synthetic linking in this iteration.
- No migration of monobank or PKO legacy sync behavior beyond shared v2 infrastructure needed by synthetic.
- No real external provider dependency, credentials, OAuth/SCA flow, or provider discovery.
- No long-term compatibility layer for previous alpha state shapes.

## Decisions

1. Use `synthetic` as the provider and connector identifier.

   `synthetic` describes the source as generated but still provider-shaped. It is clearer than `data-gen`, which sounds like a utility, and less misleading than a fake bank brand.

   Alternative considered: `data-gen`. Rejected because it reads as a tool rather than a provider identity and would likely age poorly in provider registries, logs, and state records.

2. Define a finance-internal core-only linking seam named `LinkConfiguredBankConnection`.

   The finance module already exposes public product flows such as `StartBankConnectionLink`, `FinishBankConnectionLink`, and `LinkTokenBankConnection`. Synthetic needs a different shape: core code must create an already-linked connection from provider-owned configuration instead of browser redirect or token input.

   This change should introduce a finance-internal seam named `LinkConfiguredBankConnection`. In this iteration it supports only provider `synthetic` and a typed list of configured synthetic accounts. The seam validates the config, creates an active bank connection, persists provider-owned synthetic configuration, and returns the linked connection outcome. It is intentionally not exposed through HTTP, OpenAPI, or finance UI.

   Alternative considered: overload `LinkTokenBankConnection` or `FinishBankConnectionLink` with synthetic-only branches. Rejected because synthetic does not use a token or redirect handshake, and reusing those flows would hide a different linking contract behind misleading entry points.

3. Store synthetic account configuration and generation history in dedicated synthetic storage.

   Linking synthetic requires account names and currencies before any sync window exists. Repeated-window behavior also needs mutable generation history. Both concerns are synthetic-provider state, so they should live in a dedicated finance-owned synthetic store keyed by bank connection.

   The synthetic store should keep a typed, versioned state envelope containing configured accounts, generated-window keys, repeat counts, and per-account/day transaction sequence counters. The state can be stored as JSON inside a synthetic-owned table because the structure belongs to this provider, not to the common sync journal.

   Each configured account entry should receive a stable synthetic account key when the connection is linked. That key should be persisted alongside display name and currency and should remain the identity source for later observations. Duplicate configured accounts with the same display name and currency are allowed; they remain distinct because provider account IDs derive from connection identity plus synthetic account key, not from human-readable fields alone.

   Alternative considered: append an initial journal row during linking. Rejected because existing provider sync v2 requirements say journal rows describe concrete attempted windows.

4. Keep provider sync v2 journal rows provider-agnostic.

   Do not add `ProviderStateJSON` or equivalent provider-specific blobs to `domain.ProviderSyncState` or the provider sync state journal table for this change. The journal remains the generic record of sync attempts and outcomes.

   Synthetic generation history belongs to the provider-owned store, but it should not be recorded at link time or on a failed fetch attempt. It should advance only on the successful synthetic fetch path for a requested window: load synthetic state, generate the batch, persist updated synthetic state, then return executor success. The common sync journal continues to append rows for every attempted window, including failures, after the executor path returns. That means a failed window can have a journal row without a new synthetic generated-window entry.

   If `WindowSyncExecutor` later grows more stages inside the same success path, the synthetic state write still belongs at the end of that successful executor path rather than in common journal rows.

   Alternative considered: add an opaque blob to common journal rows. Rejected because the state is only needed by synthetic right now, and adding generic provider-state plumbing would widen the common provider layer for one provider's needs.

5. Inject synthetic storage into the synthetic provider rather than exposing the journal to connectors.

   The synthetic connector or provider adapter should accept a consumer-defined synthetic state store dependency. During fetch, it should load the configured accounts and generation history by connection ID, generate observations, and persist the updated generation history in that store on the successful synthetic fetch path.

   The provider should not write ledger records directly. Writing synthetic provider configuration and generation history is allowed because those rows are provider-owned state, not normalized finance ledger records.

   Alternative considered: give synthetic direct access to the common sync journal. Rejected because the journal owns cross-provider sync attempts, not provider-specific generation state.

6. Define repeated synthetic generation by an exact normalized UTC day key.

   The synthetic store should record completed requested windows by a named internal key, `SyntheticWindowKey`, with `normalizedStartUTC` and `normalizedEndExclusiveUTC` fields. The key should represent the minimal UTC day span covering the half-open requested instant range `[Start, End)`.

   Normalization rules:
   - Convert `Start` and `End` to UTC first.
   - `normalizedStartUTC` is the UTC midnight at the start timestamp's calendar day.
   - `normalizedEndExclusiveUTC` is:
     - the exact end timestamp when the UTC end already lands on midnight, or
     - the next UTC midnight after the UTC end timestamp's calendar day when the end is intra-day.
   - The generated day set is every UTC day start `D` where `normalizedStartUTC <= D < normalizedEndExclusiveUTC`.
   - The repeated-run "last day" is the UTC day whose start is `normalizedEndExclusiveUTC - 24h`.

   Two windows are considered repeats only when both normalized boundaries match exactly. Overlap alone does not count as repeated.

   Example: `2026-01-10T15:00:00Z` to `2026-01-12T00:00:00Z` normalizes to days `2026-01-10` and `2026-01-11`. `2026-01-10T15:00:00Z` to `2026-01-12T15:00:00Z` normalizes to days `2026-01-10`, `2026-01-11`, and `2026-01-12`.

   Alternative considered: treat any overlap with prior generated days as repeated. Rejected for this iteration because partial-overlap semantics are harder to reason about and unnecessary for the requested behavior.

7. Generate stable account, balance, and transaction observations.

   Each successful synthetic fetch should return:
   - one provider account observation per configured account using a stable provider account ID derived from connection identity plus synthetic account key
   - one balance observation per configured account in the configured currency
   - transaction observations whose provider transaction IDs include provider account identity, UTC day, and a state-backed sequence counter so repeated runs create new observations instead of duplicating earlier IDs
   - raw payload observations that clearly identify synthetic generation and whether the batch came from a first or repeated normalized window

   Balance generation does not need a separate historical-balance subsystem in this iteration. It only needs to return a current synthetic balance snapshot per configured account that is internally consistent for the generated batch and uses the same account currency.

   Alternative considered: fully random IDs. Rejected because idempotency and repeated-window tests need predictable uniqueness and traceable provider-original identifiers.

8. Keep the implementation inside the finance product slice.

   Synthetic provider logic belongs in `finance/` and `finance/internal/providers`. The finance module must stay independent from `runtime/`, and this planning change should stay finance-scoped. No application-layer setup workflow, provider setup screen, or public API exposure belongs in this iteration.

   Alternative considered: implement synthetic as app-only fixture code. Rejected because the provider exercises common finance provider infrastructure and should live with the product slice it validates.

## Risks / Trade-offs

- [Synthetic state becomes an unversioned dumping ground] -> Include a small typed synthetic state envelope with a version field inside the synthetic provider boundary.
- [Random generation creates flaky tests] -> Inject random source and clock in the synthetic connector tests; assert ranges and invariants rather than exact amounts where appropriate.
- [Duplicate configured accounts can collide] -> Persist a stable synthetic account key per configured entry and derive provider account identities from that key.
- [Synthetic state can drift from sync attempts] -> Advance generated-window history only on the successful synthetic fetch path, keep generated-window keys tied to normalized requested windows, and keep the common journal responsible only for attempt/outcome audit.
- [Core-only linking shape may not match future UI needs] -> Keep the link config minimal: account name and ISO currency only, but put it behind `LinkConfiguredBankConnection` so later surfaces can wrap the same core seam.
- [Synthetic data could be mistaken for real provider data] -> Use provider/connector ID `synthetic`, raw payloads that identify synthetic generation, and no external-bank branding.

## Migration Plan

1. Extend `domain.ProviderID`, `domain.ProviderConnectorID`, and provider profile/registry code with `synthetic`.
2. Extend the shared `WindowSyncExecutor` fetch foundation so connectors can be resolved and fetched through the common v2 executor path before provider-specific end-to-end wiring is added.
3. Add dedicated synthetic-provider persistence models and a narrow store for configured accounts and generation history.
4. Add the finance-internal `LinkConfiguredBankConnection` seam for typed synthetic configured accounts without adding HTTP/UI exposure.
5. Add the synthetic provider connector with typed account config, `SyntheticWindowKey` normalization, balance generation, and typed synthetic-state envelope at the provider boundary.
6. Wire synthetic fetch to load and save generation history through synthetic storage while leaving common sync journal rows unchanged.
7. Add focused tests for duplicate configured-account identity handling, first-window generation, repeated normalized-window generation, balance coverage, successful-fetch state persistence, and connector registration.

Rollback before implementation dependencies land is removing the new provider and synthetic-provider tables from the alpha schema. The project is early alpha, so backward-compatible migration planning is not required.

## Open Questions

- Should the later UI expose synthetic as a first-class provider, or keep it behind local/dev tooling?
- Should synthetic generation eventually use seeded randomness per tenant/connection for reproducible demos, or stay random with persisted uniqueness guarantees?
