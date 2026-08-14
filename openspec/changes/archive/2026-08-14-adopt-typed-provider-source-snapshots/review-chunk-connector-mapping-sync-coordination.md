# Chunk Review: Connector Mapping And Sync Coordination

## Initial State

- Scope: tasks 3.1-3.3.
- Status: pending predecessor chunk completion.

## Implementation Status

- Status: complete; tasks 3.1-3.3 are checked.
- Enable Banking emits immutable typed connection, account, account-balance,
  and per-item transaction snapshots; Monobank and synthetic emit only their
  typed provider-owned documents and do not invent account-balance snapshots.
- Link finish atomically persists connection, encrypted secret, and final
  snapshot. Both active bank-sync and provider-window apply paths atomically
  attach snapshots after normalized records and matches.
- Active orchestration no longer writes evidence or raw payloads. The obsolete
  bank-sync evidence/raw writer paths and unused link raw-payload dependency
  were removed; the still-published legacy evidence read service remains for
  the later API/removal chunks.

## Coverage Gate Resolution

- The initially added tests did not raise all gated files because assertions
  were followed by early `return` statements, leaving the intended scenarios
  unreachable. Replaced them with meaningful legacy-read, rollback, mapped
  snapshot, and persistence-failure coverage.
- Removed unreachable obsolete bank-sync evidence/raw write code rather than
  adding coverage for behavior this chunk must no longer perform.
- The completion gate passes without changing thresholds: total coverage is
	`93.8% (5497/5858)` and every file meets the existing 90% threshold.

## Independent Review Remediation

- Provider-window apply now reads the linked connection and persists validated
  connection snapshots in the same transaction as account, balance, and
  transaction snapshots.
- Monobank uses `client-info` as the stable connection snapshot identity at
  link and fetch time. Transaction snapshots use the transaction fingerprint
  when Monobank omits a provider transaction ID.
- Link coordination requires the atomic connection/secret/snapshot persistence
  operation; the snapshot-dropping fallback was removed.
- Regression coverage verifies all four provider-window kinds, fingerprint
  transaction handling in both active and provider-window sync paths, and
  replacement of the stable Monobank connection snapshot identity.

## Finalization Checks

- `make lint` in `finance/` — passed with `0 issues`.
- `make test` in `finance/` — passed with all per-file coverage thresholds.
- `make lint` in `finance/` — passed with `0 issues` after remediation.
- `direnv exec /Users/jenya/projects/sumweave env -u
  APP_HTTPSERVER_TLS_CERTFILE -u APP_HTTPSERVER_TLS_KEYFILE make
  affected-lint-test` — passed for `finance`, `sumweave`, and
  `integration-cli`.
