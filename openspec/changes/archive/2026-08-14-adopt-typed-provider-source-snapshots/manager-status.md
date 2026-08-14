# Manager Status

## Current State

- Phase: submission
- Task reference: apply `adopt-typed-provider-source-snapshots` in ordered chunks; implementation and final acceptance complete
- Change slug: adopt-typed-provider-source-snapshots
- Last updated: post-archive validation passed; submission in progress
  (2026-08-14)

## Workflow Board

- Planning: complete
- Implementation: complete
- User review/correction: complete
- Archive: complete
- Submission: in progress

## Standard Artifacts

- Planning review: `review-planning.md`
- Final review: `review-final.md`
- Chunk reviews:
  - `enable-banking-typed-contract`: `review-chunk-enable-banking-typed-contract.md`
  - `provider-snapshot-domain-persistence`: `review-chunk-provider-snapshot-domain-persistence.md`
  - `connector-mapping-sync-coordination`: `review-chunk-connector-mapping-sync-coordination.md`
  - `protected-finance-api`: `review-chunk-protected-finance-api.md`
  - `finance-operator-ui`: `review-chunk-finance-operator-ui.md`
  - `legacy-removal-documentation`: `review-chunk-legacy-removal-documentation.md`

## Chunk Ledger

### `enable-banking-typed-contract`

- Scope: tasks 1.1-1.2, complete Enable Banking typed source-response DTO contract
- Status: complete
- Review file: `review-chunk-enable-banking-typed-contract.md`
- Commit: none; user requested no commit

### `provider-snapshot-domain-persistence`

- Scope: tasks 2.1-2.3, provider snapshot domain, dedicated persistence, and reads
- Status: complete
- Review file: `review-chunk-provider-snapshot-domain-persistence.md`
- Commit: none; user requested no commit

### `connector-mapping-sync-coordination`

- Scope: tasks 3.1-3.3, connector snapshots and atomic link/sync persistence
- Status: complete
- Review file: `review-chunk-connector-mapping-sync-coordination.md`
- Commit: none; user requested no commit

### `protected-finance-api`

- Scope: tasks 4.1-4.2, protected provider-snapshot API and Sumweave composition
- Status: complete
- Review file: `review-chunk-protected-finance-api.md`
- Commit: none yet
- Notes: account and transaction source-data routes now use only protected
  `/provider-snapshots` metadata/detail contracts; app composition uses the
  focused `ProviderSnapshotService`

### `finance-operator-ui`

- Scope: tasks 5.1-5.2, finance UI source-data disclosure and wireframes
- Status: complete
- Review file: `review-chunk-finance-operator-ui.md`
- Commit: none; user requested no commit

### `legacy-removal-documentation`

- Scope: task 6.1, legacy removal, documentation, final verification, and Mock ASPSP E2E
- Status: complete; implementation/docs and Mock ASPSP E2E acceptance passed
- Review file: `review-chunk-legacy-removal-documentation.md`
- Commit: none yet

## Agent Runs

### planning review — crew-p4-high

- Scope: review the externally prepared change and propose serialized chunks
- Status: complete
- Notes: ready for implementation; source is `tmp/crew-manager/adopt-typed-provider-source-snapshots-001-crew-p4-high-notes.md`

### implementation and chunk finalization — crew-p3-high

- Scope: chunk `enable-banking-typed-contract`, tasks 1.1-1.2
- Status: complete
- Notes: tasks 1.1-1.2 complete; shallow finalization found no blocking issue and the user-requested no-commit instruction leaves the clean commit gate deferred

### implementation and chunk finalization — crew-p3-high

- Scope: chunk `provider-snapshot-domain-persistence`, tasks 2.1-2.3
- Status: complete
- Notes: current provider snapshot domain, dedicated GORM store/schema, tenant-authorized list/detail service, and connection-owned cleanup are complete; connector and API work remain deferred to their ordered chunks

### independent-review finding resolution — crew-p3-high

- Scope: resolve the snapshot-persistence independent-review findings before chunk 3
- Status: complete
- Notes: expanded credential-like sanitization, save-time ownership validation, transaction-account consistency, and subject-kind validation are covered and verified; chunk 3 is unblocked

### implementation and coverage completion — crew-p3-high

- Scope: chunk `connector-mapping-sync-coordination`, tasks 3.1-3.3
- Status: complete
- Notes: typed connector snapshots and atomic link/active-sync/provider-window persistence are complete; obsolete active evidence/raw writes and unused link raw-payload dependency were removed; coverage gate, finance lint/test, and repository completion checks passed

### independent-review remediation — crew-p3-high

- Scope: resolve connector mapping and sync coordination review findings
- Status: complete
- Notes: provider-window connection snapshots, Monobank stable/fallback identities, and required atomic link snapshot persistence are covered and verified; API chunk 4 is unblocked

### implementation and chunk finalization — crew-p3-high

- Scope: chunk `protected-finance-api`, tasks 4.1-4.2
- Status: complete
- Notes: regenerated OpenAPI routes expose metadata and `data` detail documents
  only at `/provider-snapshots`; registered-route coverage verifies authorization,
  cross-tenant denial, missing data, sanitized source documents, and absent
  `/evidence` aliases

### implementation and visual verification — crew-p4

- Scope: chunk `finance-operator-ui`, tasks 5.1-5.2
- Status: complete
- Notes: UI mappings and disclosures now use current provider snapshots and
  provider source-data terminology; account and transaction states cover lazy
  metadata/detail loading, distinct kinds, explicit reveal, bounded recovery,
  and responsive visual verification

### whole-change review remediation — crew-p3-high

- Scope: resolve journal 016 pending-document sanitization and stale
  architecture-note findings
- Status: complete
- Notes: pending redirect-start documents are sanitized and re-encoded before
  persistence; invalid documents cannot save, pending starts do not use current
  snapshot storage, nested credential-like values are covered, and the stale
  public DTO note is removed

### final Mock ASPSP acceptance — crew-p2-high

- Scope: final configured Mock ASPSP E2E acceptance after whole-change review
  remediation
- Status: complete
- Notes: PKO authorization/linking, durable sync, typed source snapshots,
  secret-safe API/UI checks, authenticated legacy-route 404s, and final error
  checks passed; see `tmp/crew-manager/adopt-typed-provider-source-snapshots-019-crew-p2-high-notes.md`

## Open Decisions / Blockers

- Archive command completed successfully and promoted the approved finance
  management and operator UI spec deltas. Post-archive validation passed;
  submission is in progress.
