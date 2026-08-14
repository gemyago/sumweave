# Planning Review

## Round 1 — 2026-08-14

### Scope

Review of the externally prepared `adopt-typed-provider-source-snapshots` change before implementation, based on `tmp/crew-manager/adopt-typed-provider-source-snapshots-001-crew-p4-high-notes.md`.

### Verdict

Approved for serialized implementation. The proposal, design, task list, and spec deltas consistently require typed provider-owned source documents, immutable provider DTOs during normalization, and an early-alpha breaking replacement of evidence/raw-payload storage and API terminology.

### Findings And Resolutions

- The review clarified the supported Enable Banking reference URL/date; stable snapshot identity; latest-snapshot replacement; atomic sync/link persistence; pending-start behavior; connection-owned deletion; Monobank and synthetic source-document rules; and the `data` API detail field.
- The review updated the change artifacts to capture those decisions and verified strict OpenSpec validation.
- No planning blockers remain.

### Strict Ordered Chunk Plan

1. `enable-banking-typed-contract` — tasks 1.1-1.2. Add failing complete semantic fixture round-trip tests, then complete the typed Enable Banking response DTO graph and remove client-side provider-field mutation.
2. `provider-snapshot-domain-persistence` — tasks 2.1-2.3. Add provider snapshot domain, dedicated GORM store, and authorized current-snapshot reads/deletion.
3. `connector-mapping-sync-coordination` — tasks 3.1-3.3. Emit typed snapshots from all connectors and persist them atomically through link and both sync paths.
4. `protected-finance-api` — tasks 4.1-4.2. Replace protected evidence contracts and compose the service in Sumweave.
5. `finance-operator-ui` — tasks 5.1-5.2. Replace UI evidence mappings/disclosures and perform visual verification.
6. `legacy-removal-documentation` — task 6.1. Remove retired paths, update active docs, run final checks, and complete recreated-DB Mock ASPSP acceptance.

Do not start a later chunk before the previous backend chunk has completed its review gate. No frontend chunk is independent before the backend API contract is generated.

### Verification

- `openspec instructions apply --change adopt-typed-provider-source-snapshots`: passed during planning review.
- `openspec validate adopt-typed-provider-source-snapshots --strict`: passed during planning review.
- `openspec status --change adopt-typed-provider-source-snapshots`: reported all four prepared change artifacts complete during planning review.

### Artifact Cleanup

Clean. This change now has only OpenSpec artifacts and standard manager/review files.

### Commit Status

No commit created: the user explicitly requested no commit. The implementation gate remains subject to the repository completion protocol; the no-commit instruction overrides the normal manager commit gate for this run.
