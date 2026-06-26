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
