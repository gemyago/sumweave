---
type: Historical record
title: "2026-07-06-use-enablebanking-typed-client: tasks"
description: "Historical tasks record for 2026-07-06-use-enablebanking-typed-client."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-07-06-use-enablebanking-typed-client/tasks.md"
    title: "Original openspec/changes/archive/2026-07-06-use-enablebanking-typed-client/tasks.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## 1. Typed Client Contract

- [x] 1.1 Add a typed Enable Banking connector client seam, and must follow TDD flow by first writing failing connector tests or compile-focused tests proving the connector dependency exposes typed auth, session, balance, and transaction operations instead of `DoRawObject`, then replacing the raw `apiClient` interface with a narrow typed interface backed by the generated client.
- [x] 1.2 Remove connector dependence on raw-only response fields, and must follow TDD flow by first writing failing connector tests proving values absent from existing typed client models are not recovered through raw maps, then updating connector mapping to use only existing generated typed model fields.

## 2. Connector Refactor

- [x] 2.1 Refactor redirect start and finish to use typed client methods, and must follow TDD flow by first writing failing connector tests proving `CreateAuth` and `CreateSession` are called and raw response maps are not read before implementing typed start/finish mapping.
- [x] 2.2 Refactor fetch to use typed session, balance, and transaction methods, and must follow TDD flow by first writing failing connector tests proving `GetSession`, typed account handling, `GetAccountBalances`, and paged `GetAccountTransactions` produce provider account, balance, transaction, provider-original, fingerprint, and continuation behavior without raw response access before implementing typed fetch mapping.

## 3. Cleanup And Verification

- [x] 3.1 Remove obsolete connector-local raw request helpers, and must follow TDD flow by first adding or updating focused tests that fail while normal connector behavior can still call raw transport helpers for start, finish, or fetch, then deleting unused raw path/query/request helpers from the connector while preserving finance-domain normalization helpers.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
