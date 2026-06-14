## Context

`docs/ARCHITECTURE.md` defines the deterministic path as `Data -> Analytics -> Strategy -> Governor -> Execution`, with strategy and governor as runtime slices and storage implemented through explicit GORM schemas supporting SQLite. `docs/IMPLEMENTATION_STATUS.md` calls out missing `StrategyArtifact` and `GovernorPolicy` product objects, while the existing `runtime/strategy` and `runtime/governor` packages already expose deterministic in-memory evaluation requests.

This change plans only the v0 artifact layer for those two slices. It intentionally does not add backend wiring, UI, HTTP routes, strategy output storage, governor decision storage, audit traces, paper-trading orchestration, or live execution policy behavior.

## Goals / Non-Goals

**Goals:**
- Persist immutable, versioned strategy and paper-governor policy definitions as runtime artifacts.
- Derive canonical JSON bytes and stable SHA-256 hashes from validated canonical artifact payloads.
- Make duplicate creates idempotent by canonical hash.
- Provide `create`, `get`, and `list` for strategy artifacts.
- Provide `create`, `get`, and `get-active` for governor policy artifacts.
- Keep Strategy DSL v0 strict: moving-average crossover only, no arbitrary code or unknown fields.
- Map canonical Strategy DSL plus an explicit evaluation time range to the existing `strategy.EvaluateRequest`.
- Validate governor policy artifacts against existing governor policy semantics and paper-only safety gates.
- Cover persistence behavior with SQLite-backed tests.

**Non-Goals:**
- No backend dependency injection, HTTP/OpenAPI changes, API handlers, or UI work.
- No live-trading policy support, wallet credentials, signing, private endpoints, or venue order routing.
- No arbitrary strategy code, expression language, plugin execution, scripts, AI-generated runtime code, or sandboxing.
- No persisted strategy outputs, candidate action storage, governor decision storage, audit trace store, or backtest/paper run storage.
- No new strategy kinds or governor rule families beyond the existing moving-average-crossover and governor policy fields.

## Decisions

### Keep artifacts slice-owned and minimal

Add artifact types and stores inside the existing `runtime/strategy` and `runtime/governor` slices rather than introducing a cross-slice artifact framework. `StrategyArtifact` belongs with strategy definition validation and mapping. `GovernorPolicyArtifact` belongs with governor policy validation and active paper policy selection. Shared `domain/` types should only be introduced if an artifact value must cross slice boundaries; otherwise keep persistence and artifact details local to the owning slice.

Alternative considered: add a generic `runtime/artifacts` package. This was rejected for v0 because only two artifact kinds are required, their validation is slice-specific, and a generic abstraction would add surface area before repeated patterns are proven.

### Use canonical typed payloads for JSON and hashes

Each artifact should be canonicalized into a typed payload with explicit `schemaVersion` and artifact kind fields before storage. Canonical JSON bytes should be produced from that typed canonical payload, not from caller-provided raw JSON. Arrays that affect identity, such as allowed action kinds, must be normalized into a stable order before marshaling. Time values, if any are later introduced, must be UTC-normalized before canonical JSON generation.

The artifact hash should be the lowercase hex SHA-256 digest of the canonical JSON bytes. The hash can serve as the stable artifact identifier for create/get/list behavior, with a unique database constraint on the hash. The store should persist the canonical bytes and hash together so retrieval does not rehydrate a different byte representation.

Alternative considered: hash caller-supplied JSON directly. This was rejected because semantically identical JSON with different object ordering or whitespace would produce different identities.

Alternative considered: invent a full RFC 8785 canonical JSON implementation. This was rejected for v0 because the payloads can be represented as typed structs without maps, with explicitly sorted slices and standard JSON marshaling giving a stable project-local canonical representation.

### Make duplicate creates idempotent by hash

Creating an artifact whose canonical hash already exists should return the existing immutable artifact instead of inserting a second row or mutating the existing row. If the create call asks to activate a duplicate governor policy, only the active selector may change; the policy artifact row and canonical bytes must remain unchanged.

Alternative considered: return a duplicate error on same hash. This was rejected because artifact creation is safer and easier to retry when canonical duplicates are idempotent.

### Define Strategy DSL v0 as data, not code

The v0 Strategy DSL should be a strict typed data object for the existing moving-average crossover strategy. It should include the canonical market scope needed by the strategy definition, the strategy kind, and `fastWindow`/`slowWindow` parameters. The decoder/validator must reject unknown fields; fields that look like arbitrary code entry points such as scripts, expressions, functions, imports, modules, prompts, or commands are rejected through the same strict unknown-field behavior and should also be covered explicitly in tests.

The DSL should not own the evaluation time range in v0. Mapping to `strategy.EvaluateRequest` should combine the canonical DSL with an explicit caller-supplied `domain.TimeRange`, preserving the existing model where a strategy definition can be evaluated over different ranges.

Alternative considered: store an entire `strategy.EvaluateRequest`, including the time range, as the artifact. This was rejected because it would make a strategy artifact represent one evaluation run rather than a reusable strategy definition.

Alternative considered: add an expression or plugin-based DSL. This was rejected because the deterministic path must avoid arbitrary code and AI/runtime script execution.

### Model governor policy artifacts as paper-only with a mutable active selector

The policy artifact itself must be immutable and paper-only. It should canonicalize to the existing `governor.Policy` fields: allowed action kinds, minimum quality, and maximum approved count, plus version/kind/mode metadata. Validation should reject unsupported qualities, empty allowed action sets, negative approval counts, non-paper modes, and unknown fields that imply live execution routing or credentials.

`GetActive` requires a mutable selector separate from immutable policy rows. The minimal write path is a create request that can atomically activate the created or existing duplicate policy artifact. The active selector is mutable state; the referenced policy artifact is not. If no policy has been activated, `GetActive` should return a not-found result rather than inventing a default policy.

Alternative considered: add a standalone `SetActive` operation. This was deferred because the gathered requirements only include create/get/get-active. It can be added later if the product needs activation separate from policy creation.

### Follow existing runtime database patterns

Database stores should follow the existing runtime GORM style: constructors accept DSN plus options such as table prefix, models use explicit column names, `AutoMigrate` creates the schema, and tests run against SQLite. Artifact tables should have unique hash indexes and store canonical JSON bytes as text or bytes in a column with explicit name. Active policy selection should use a small table keyed by a stable scope such as `paper` so the design can stay single-active-policy for v0 without introducing accounts, workspaces, or live execution scopes.

Alternative considered: use file storage for artifacts first. This was rejected because the requirements explicitly call for SQLite DB tests and the repo already has GORM database foundations.

## Risks / Trade-offs

- [Risk] Project-local canonical JSON may not satisfy every external canonicalization standard. → Mitigation: keep canonical payloads typed, map-free, explicitly sorted, versioned, and covered by hash stability tests.
- [Risk] Using hash as identity makes future non-hash identifiers harder to add. → Mitigation: treat hash as v0 identity and keep schema version in canonical bytes so a later ID can be introduced without mutating existing rows.
- [Risk] A create-time activation flag is less flexible than a separate activation API. → Mitigation: it satisfies create/get/get-active with minimal surface; add `SetActive` only when a real caller requires it.
- [Risk] Rejecting all unknown DSL and policy fields may be strict during experimentation. → Mitigation: this is intentional for deterministic safety; add versioned fields through future OpenSpec changes.

## Migration Plan

No production data migration or rollout sequencing is required for this planning change. Implementation should add `AutoMigrate` support for the new artifact tables and cover fresh SQLite creation in tests. Rollback before archive is removing the new OpenSpec change; rollback after implementation is reverting the artifact package changes and their schema usage before any backend starts depending on them.

## Open Questions

- The plan assumes a StrategyArtifact does not include an evaluation time range; mapping to `strategy.EvaluateRequest` accepts an explicit range from the caller. If product intent is that artifacts capture one complete evaluation request, the DSL spec should be adjusted before implementation.
- The plan assumes active governor policy selection is global for paper mode in v0. If activation must be scoped by user, workspace, strategy, venue, or environment, that scope must be specified before implementation.
