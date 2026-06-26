# Chunk Review — finance-persistence-srp-cleanup

## Round 1

- Trigger: follow-up cleanup set review for chunk 9.3
- Verdict: directionally improved, but not review-clean yet
- Scope fit: yes, changes stay within finance persistence initialization, service factoring, and related artifact updates
- What is correct:
  - `finance/AGENTS.md`, design/spec artifacts, and persistence code now align on GORM auto-migrate instead of custom SQL migrations
  - the large finance service was usefully split into focused tenant/catalog/ledger/access collaborators, and `go test ./finance/...` stayed green after that refactor
- Blocking issue:
  - the auto-migrate model tags do not yet preserve key schema guarantees from the removed SQL migrations, including at least invite-code uniqueness (`tenantInviteModel.Code`) and the prior composite provider-match / balance-snapshot / raw-payload index shapes, so auto-migrate does not yet faithfully replace the removed schema contract
- Verification:
  - `go test ./finance/...` passed
  - schema review against the removed SQL migrations shows constraint/index parity gaps in the current GORM model tags
- Completion protocol: not satisfied for the chunk because the persistence replacement is not yet schema-equivalent where it needs to be
- Commit status: no commit yet is acceptable because the chunk is not review-clean
