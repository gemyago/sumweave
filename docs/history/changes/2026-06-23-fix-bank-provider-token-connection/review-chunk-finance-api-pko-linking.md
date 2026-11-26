---
type: Historical record
title: "2026-06-23-fix-bank-provider-token-connection: review chunk finance api pko linking"
description: "Historical review chunk finance api pko linking record for 2026-06-23-fix-bank-provider-token-connection."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-06-23-fix-bank-provider-token-connection/review-chunk-finance-api-pko-linking.md"
    title: "Original openspec/changes/archive/2026-06-23-fix-bank-provider-token-connection/review-chunk-finance-api-pko-linking.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

# Chunk Review: finance-api-pko-linking

## Round 1

- Scope: task 2.1 + 2.2 / finance API PKO redirect-link contract and controller mapping
- Triggering input: implementation updated OpenAPI + controller/routes/tests
- Verdict: clean
- Findings:
  - 2.1 contract extension is present and consistent:
    - `finance.yaml` adds
      `POST /api/v1/finance/tenants/{tenantId}/connections/link-redirect/start`
      and
      `POST /api/v1/finance/tenants/{tenantId}/connections/link-redirect/finish`.
    - New models use the requested camelCase fields and types:
      `FinanceConnectionLinkRedirectStartRequest{provider,callbackUrl}`,
      `FinanceConnectionLinkRedirectFinishRequest{provider,code,state}`,
      `FinanceConnectionLinkRedirectStartResponse{provider,authorizationUrl,state}`.
    - Separate provider enums are introduced:
      `FinanceBankLinkTokenProvider == monobank` and `FinanceBankLinkRedirectProvider == pko`.
  - Redirect start validation behavior matches acceptance criteria:
    - route/controller tests cover valid callback shape and explicit bad shapes,
      including malformed path/target and insecure non-local `http` callbacks.
    - `validateFinanceRedirectCallbackURL` enforces absolute URLs, no user info,
      no query params, `#/finance/connections` fragment target, and allows
      loopback hosts for non-https callbacks (`localhost`, `127.0.0.1`, `[::1]`).
  - Redirect finish route/controller behavior is correct:
    - finish handler passes only `tenantId`, `provider`, `state`, and `code` to
      service start; no browser-supplied `start` metadata is consumed.
    - controller test explicitly sends extra `start` in request body and asserts
      service receives `financepkg.ProviderLinkStart{}`.
  - Explicit provider-method boundaries are tested at route level:
    - monobank is accepted for token route and rejected for redirect start/finish.
    - pko is accepted for redirect start/finish and rejected for token route.
  - Provider/backend errors are sanitized:
    - `sanitizeBankConnectionError` maps known provider/service errors and generic
      mapping hides unknown messages; test confirms provider error payload does not
      leak secret-like content.
  - Registered-route/controller test coherence:
    - route list and delegation matrix in `finance_test.go` include both new routes
      and assert auth-required behavior, service-call mapping, and successful payload
      mapping.

- Artifact cleanup status:
  - Reviewed generated route/controller artifacts are present where expected.
  - No out-of-scope `manager-status.md` edits were made for this chunk.

- Completion protocol status:
  - `make affected-lint-test` completed successfully (all relevant projects clean).

- Commit status: no commit yet for this chunk.
  - A chunk-level commit is still required before gate pass.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
