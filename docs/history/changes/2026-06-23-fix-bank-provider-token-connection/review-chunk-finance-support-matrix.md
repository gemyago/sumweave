---
type: Historical record
title: "2026-06-23-fix-bank-provider-token-connection: review chunk finance support matrix"
description: "Historical review chunk finance support matrix record for 2026-06-23-fix-bank-provider-token-connection."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-06-23-fix-bank-provider-token-connection/review-chunk-finance-support-matrix.md"
    title: "Original openspec/changes/archive/2026-06-23-fix-bank-provider-token-connection/review-chunk-finance-support-matrix.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

# Chunk Review: finance-support-matrix

## Round 1

- Scope: task 1.1 / finance support matrix
- Triggering input: implementation sub-agent completed chunk 1.1
- Verdict: clean
- Findings: no blocking or non-blocking issues found.
- Notes:
  - `finance/provider_sync.go` enforces explicit provider/method routing via
    `configuredBankProviderName`:
    - `monobank` accepts `token` only and maps to `bankProviderMonobank`.
    - `pko` accepts `redirect` only and maps to `bankConnectorEnableBanking`.
    - unsupported providers/methods return typed errors (`ErrUnsupportedBankProvider`,
      `ErrUnsupportedBankLinkingMethod`) before invoking provider-specific operations.
  - `StartBankConnectionLink`/`FinishBankConnectionLink` and
    `LinkTokenBankConnection` use the mapping consistently, so pko redirect start/finish
    persists/fetches a connection with `connection.Provider == "pko"` while using
    `EnableBanking` internally.
  - Error strings for unsupported provider/method paths do not include secret/token values,
    and existing tests assert non-leakage.
  - `provider_sync_test.go` and `provider_sync_internal_test.go` include positive and
    negative contract cases covering monobank token, pko redirect, and unsupported paths.
- Artifact status: reviewed artifact completed.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
