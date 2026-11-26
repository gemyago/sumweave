---
type: Historical record
title: "2026-06-29-implement-bank-provider-v2-connectors: tasks"
description: "Historical tasks record for 2026-06-29-implement-bank-provider-v2-connectors."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-06-29-implement-bank-provider-v2-connectors/tasks.md"
    title: "Original openspec/changes/archive/2026-06-29-implement-bank-provider-v2-connectors/tasks.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## 1. Monobank V2 Connector

- [ ] 1.1 Add the Monobank provider sync v2 connector in `finance/internal/monobank`, and must follow TDD flow by first writing failing connector or focused finance-owned link-wiring tests proving `ConnectorID`, capability flags, redirect-method rejection, token-link success/error behavior, secure token handoff through the existing connection-secret encryption path (`encryptAndSaveConnectionSecret` / `SaveConnectionSecret` -> `domain.ConnectionSecret.Envelope`), plaintext-free persisted link evidence/state, and fetch observation mapping for accounts, transactions, provider-original values, and raw payloads before implementing standalone connector-owned link and fetch behavior without calling legacy root `finance` provider code.
- [ ] 1.2 Add balance-observation, capture-time, and provider-local normalization coverage for the Monobank connector, and must follow TDD flow by first writing failing focused tests proving current and available balances map into v2 observations with deterministic capture timestamps and connector-owned mapping helpers before implementing the minimal clock-backed provider-local mapping.

## 2. Enable Banking V2 Connector

- [ ] 2.1 Add the Enable Banking provider sync v2 connector in `finance/internal/enablebanking`, and must follow TDD flow by first writing failing connector tests proving `ConnectorID`, capability flags, token-link rejection, legacy redirect auth start/finish support, signed official redirect auth start/finish support, secure handling of returned Enable Banking credentials through the existing connection-secret encryption path (`encryptAndSaveConnectionSecret` / `SaveConnectionSecret` -> `domain.ConnectionSecret.Envelope`), plaintext-free persisted link evidence, and bounded unsupported errors for every auth branch outside that named matrix before implementing standalone connector-owned redirect flows without calling legacy root `finance` provider code.
- [ ] 2.2 Add fetch observation mapping for Enable Banking, and must follow TDD flow by first writing failing focused tests proving legacy bearer-secret fetch (`/accounts` + per-account balances/transactions) and signed official session fetch (`/sessions/{sessionID}` + per-account balances + paged transactions) emit account, balance, transaction, provider-original, and secret-safe raw-payload observations without direct persistence, while mixed-mode or missing-input fetch branches return bounded unsupported errors, before implementing provider-local v2 batch mapping for the supported fetch modes.

## 3. Finance V2 Connector Composition

- [ ] 3.1 Keep real-bank connectors compatible with the shared registry and executor seam, and must follow TDD flow by first writing failing registry, profile, or executor tests proving Monobank resolves through technical connector `monobank`, `providers.PKOProfile()` still resolves product provider `pko` through technical connector `enable-banking`, `enable-banking` is not surfaced as a product provider choice/profile, and both connectors can be registered and fetched through the common `WindowSyncExecutor` path without product-provider branching or legacy-provider wrappers before implementing the minimal finance-owned composition wiring needed.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
