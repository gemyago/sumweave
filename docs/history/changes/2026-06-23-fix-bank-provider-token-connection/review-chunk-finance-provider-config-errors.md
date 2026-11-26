---
type: Historical record
title: "2026-06-23-fix-bank-provider-token-connection: review chunk finance provider config errors"
description: "Historical review chunk finance provider config errors record for 2026-06-23-fix-bank-provider-token-connection."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-06-23-fix-bank-provider-token-connection/review-chunk-finance-provider-config-errors.md"
    title: "Original openspec/changes/archive/2026-06-23-fix-bank-provider-token-connection/review-chunk-finance-provider-config-errors.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

# Chunk Review: finance-provider-config-errors

## Round 1

- Scope: follow-up provider config error sanitization
- Triggering input: follow-up fix chunk not yet started
- Verdict: pending
- Notes: awaiting implementation sub-agent

## Round 2

- Scope: follow-up `finance-provider-config-errors`
- Verdict: clean
- Findings: no blocking issues
  - Scope note: `finance.go` is also modified to add the controller-side mapping for `ErrBankProviderNotConfigured`; this is required by this chunk's intent (client-facing 400 mapping) even though it wasn't in the initial file list.
  - `finance/provider_sync.go` now returns wrapped `ErrBankProviderNotConfigured` in a bounded, explicit form for all affected lookup paths (including mapped PKO connector case via `bankProviderNotConfiguredForBankError`), preserving explicit context (`pko` + `enable-banking`) without leaking secrets.
  - `apps/signal-foundry/internal/api/http/v1controllers/finance.go` maps `ErrBankProviderNotConfigured` in `sanitizeBankConnectionError` to `400` client input error, which prevents generic `500` fallback for this domain.
  - New tests in `finance/provider_sync_test.go` and `apps/signal-foundry/internal/api/http/v1controllers/finance_test.go` cover missing Monobank token config, missing PKO enable-banking config (link + sync), and controller-side sanitization.
- Artifact cleanup status: acceptable. No temp build artifacts or unrelated files added by this chunk; only expected code/tests/review updates are present.
- Completion protocol status: ✓ `make affected-lint-test` passes from repo root after implementation changes (full gate succeeded).
- Commit status: no chunk commit exists yet for this follow-up; commit is still required before this chunk can be finalized.
- Commit required before gate: yes.
- Safe to continue: yes (functionally clean), provided the chunk commit is created and pushed in the normal workflow.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
