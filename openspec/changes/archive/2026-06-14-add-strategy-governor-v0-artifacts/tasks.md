## 1. Strategy DSL v0

- [x] 1.1 Add strict Strategy DSL v0 validation and typed canonicalization; must follow TDD flow (write test -> implement -> verify) by first adding tests for valid moving-average-crossover definitions, invalid instruments/timeframes/windows, unsupported strategy kinds, equivalent payload canonical values, and rejection of unknown or arbitrary-code-like fields, then implementing the minimal typed DSL canonicalizer without artifact metadata, canonical JSON bytes, or hashes.
- [x] 1.2 Add Strategy DSL to `strategy.EvaluateRequest` mapping; must follow TDD flow (write test -> implement -> verify) by first adding tests that a canonical DSL plus explicit half-open time range maps to the existing request fields and invalid ranges are rejected, then implementing the mapper without changing the strategy evaluation service behavior.

## 2. StrategyArtifact storage

- [x] 2.1 Add immutable StrategyArtifact value, versioned artifact metadata, canonical JSON bytes, and canonical hash creation; must follow TDD flow (write test -> implement -> verify) by first adding tests that valid canonical DSL produces versioned artifact metadata, canonical JSON bytes, lowercase SHA-256 hash, stable bytes/hashes for equivalent DSL values, and no artifact for invalid DSL, then implementing the minimal artifact constructor.
- [x] 2.2 Add SQLite-backed StrategyArtifact create/get/list persistence; must follow TDD flow (write test -> implement -> verify) by first adding database tests for migration, create, get, not-found, stable list ordering, idempotent duplicate create, unique hash enforcement, and unchanged canonical bytes/hash/created-at across duplicate/get/list operations, then implementing the GORM store with explicit columns.

## 3. GovernorPolicy artifact validation

- [x] 3.1 Add paper-only GovernorPolicy artifact validation and canonicalization; must follow TDD flow (write test -> implement -> verify) by first adding tests for valid paper policies mapping to `governor.Policy`, empty or unsupported allowed actions, reordered `allowedActionKinds` normalization to stable canonical bytes and hashes, unsupported minimum quality, negative maximum approval count, non-paper mode, and live-routing or credential-like fields, then implementing the minimal typed policy canonicalizer.

## 4. GovernorPolicy storage and active selection

- [x] 4.1 Add immutable GovernorPolicy artifact value and create/get/get-active persistence; must follow TDD flow (write test -> implement -> verify) by first adding SQLite-backed tests for migration, create, get, not-found, active-not-found, create-with-activate, duplicate create, duplicate create-with-activate, unique hash enforcement, and unchanged canonical bytes/hash/created-at across duplicate/get/get-active operations, then implementing the GORM store and paper active selector with explicit columns.
