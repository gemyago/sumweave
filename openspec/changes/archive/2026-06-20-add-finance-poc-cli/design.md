## Context

The existing backend is a Go/Cobra application with a root command under `apps/signal-foundry/cmd/signal-foundry`. Issue #26 explicitly asks for POC-grade CLI access to two external banking APIs and prefers extending the existing `signal-foundry` binary instead of adding new product services. The repository architecture still treats `runtime/` as the deterministic trading core and `apps/signal-foundry/` as the Go API/jobs app; this change should not introduce a new product finance domain or persistence architecture.

## Goals / Non-Goals

**Goals:**

- Provide `signal-foundry finance-poc enable-banking ...` and `signal-foundry finance-poc monobank ...` commands that can be run with `go run ./cmd/signal-foundry ...` from `apps/signal-foundry`.
- Authenticate Enable Banking calls with RS256 JWTs whose header includes `kid = ENABLE_BANKING_APP_ID`, and whose body includes `iss`, `aud`, `iat`, and `exp`.
- Support Enable Banking ASPSP discovery, authorization/session persistence, accounts/details/balances, and transaction pagination for linked PKO accounts.
- Support monobank client-info account listing and statement transaction fetching with 31-day-plus-1-hour chunking and safe rate-limit sleep defaults.
- Preserve raw provider payloads and add thin normalized summaries in machine-readable JSON output.
- Add docs and fake-server/unit coverage so CI never needs live bank credentials.
- Keep secrets and live bank data out of logs and git.

**Non-Goals:**

- No UI, product HTTP API, assistant tool, scheduler, background job, database schema, transaction categorization, budgets, payment initiation, Plaid integration, commercial multi-user consent management, or broad financial domain normalization.
- No attempt to automate PKO strong customer authentication; the human must complete browser/bank authorization or provide an already-authorized session file.
- No live bank API calls in default tests or CI.

## Decisions

1. Keep implementation isolated in the existing Cobra binary.

   Add the command tree under `apps/signal-foundry/cmd/signal-foundry` rather than separate binaries. Use descriptive files such as `finance_poc_cmd.go`, `finance_poc_enable_banking.go`, and `finance_poc_monobank.go`. Keep the implementation POC-local and avoid app DI, runtime slices, product HTTP APIs, and persistence wiring.

2. Use direct `net/http` clients with explicit configuration.

   Provider clients should accept an injected `*http.Client`, base URL, timeout/context from the command, and credential material read from env vars or flags. This keeps fake-server tests simple and avoids adding architecture for a temporary POC. Errors should include status code and a bounded response body excerpt, but must not include tokens, private key contents, JWTs, session files, or full live bank payloads.

3. Prefer JSON stdout and stderr progress summaries.

   Commands should support `--json` and keep stdout machine-readable when it is set. Progress messages, authorization URLs, wait/status messages, and summary counts belong on stderr unless they are part of the documented JSON output. Output envelopes should include provider, operation, fetchedAt, summary, items, and raw payloads where applicable.

4. Enable Banking authorization supports explicit manual start/finish plus local callback convenience.

   Implement `start-auth --country ... --aspsp-name ... --psu-type ... --valid-days ... --redirect-url ... --auth-file ... --json` as the manual first step. It must generate a cryptographically random state, call `POST /auth`, persist the pending auth file before printing the authorization URL, and include provider, kind `pending-auth`, createdAt, state, request fields (country, ASPSP name, PSU type, valid days, redirect URL), normalized authorization URL, normalized provider auth/session identifier when present, and the raw `POST /auth` payload needed to build the later `POST /sessions` request. Implement `finish-session --auth-file ... --code ... --state ... --session-file ... --json` as the only manual completion contract: it loads the pending auth file, verifies `--state` against the stored state, calls `POST /sessions` using the code plus persisted pending auth data, and saves the documented session file. `connect` may remain as a one-process convenience flow for local callbacks: it performs the same start step in memory, waits on `--callback-listen-addr`, verifies callback state, exchanges the code, and writes the session file. `--open-browser` may open the URL only when explicitly set; no auto-open by default.

5. Session and output files are local artifacts, not product persistence.

   The Enable Banking session file and transaction exports are JSON files written only when the operator passes a path. The docs should prefer the repo-root ignored data directory when commands are run from `apps/signal-foundry` (for example `../../data/financial-poc/...`), or explicitly add ignore coverage for any module-local `./data/financial-poc/...` examples. File writes should create parent directories when needed, use owner-only permissions where practical, and never commit generated files.

6. monobank statements are chunked safely.

   Parse `--from` and `--to` as `YYYY-MM-DD`, convert to Unix seconds, split requests into chunks no larger than 31 days plus 1 hour, and sleep between chunks. Use a safe default such as `61s`; tests can override to `0s` or a short duration through `--sleep-between-requests`. The client must send `X-Token` without printing it.

7. Documentation carries required manual steps.

   Add `apps/signal-foundry/doc/financial-poc/enable-banking-pko.md` and `apps/signal-foundry/doc/financial-poc/monobank.md`. These docs must list setup steps, env vars, live command examples, provider limits, restricted production linked-account behavior, and AI-agent live testing constraints. The docs must state that an AI agent cannot complete PKO SCA, that Enable Banking manual auth uses `start-auth` followed by `finish-session --auth-file ... --code ... --state ...`, and that monobank repeated calls can hit 60-second limits.

8. Git ignore coverage is part of the security boundary.

   The existing root `.gitignore` already ignores `/data`, which covers repo-root POC output. Implementation should confirm whether this covers all documented examples from the documented working directory. If not, add explicit patterns for the documented financial POC directory plus `*.enable-banking-session.json`, `*.monobank-transactions.json`, and `*.enable-banking-transactions.json` without weakening existing ignore rules.

9. Tests stay offline and behavior-focused.

    Add tests in the same package as the command code. Use `httptest.Server` for provider interactions, injected clients/base URLs, generated test RSA keys, random non-secret test data, and zero/short sleeps. Required coverage includes JWT header/signing claims, Enable Banking endpoints and pagination, monobank token header and endpoint paths, date chunking, no-sleep command execution, command wiring, JSON output shape, and no token/JWT/private-key leakage in errors.

10. Enable Banking local callbacks use HTTPS for localhost.

    The `connect` convenience flow should generate an `https://<callback-listen-addr>/callback` redirect URL and serve the local callback listener over TLS. Keep the change local to the callback listener; do not alter manual `start-auth`/`finish-session`, provider session exchange, account, transaction, monobank, app HTTP API, or persistence behavior. Treat `--callback-cert-file` and `--callback-key-file` as a required pair: when both are supplied, `connect` must load and use those files for the TLS listener; when neither is supplied, it must generate an ephemeral self-signed localhost/IP-SAN certificate in memory and clearly warn on stderr that the browser may show an untrusted-certificate warning; when exactly one is supplied, it must fail clearly during command validation instead of silently falling back. User-supplied files let users provide a locally trusted certificate created outside the CLI, for example with `mkcert` or their OS/browser trust workflow. Do not write generated certificate material to disk and do not attempt to mutate the OS trust store.

## Risks / Trade-offs

- Provider docs or response shapes may differ from the issue summary. Mitigation: keep raw JSON passthrough, minimal normalized fields, and direct endpoint construction; implementation may verify external docs but should not broaden scope beyond issue #26.
- Enable Banking production redirects may reject localhost for PKO. Mitigation: document tunnel usage and provide manual `start-auth`/`finish-session`.
- Some providers require an HTTPS redirect URL even for localhost. Mitigation: make `connect` use HTTPS locally and document trusted-cert flags plus the self-signed fallback.
- A self-signed fallback is not equivalent to a trusted certificate. Browsers, bank apps, or embedded authorization flows may reject or warn on it. Mitigation: keep manual tunnel/public HTTPS guidance and document that a user-supplied locally trusted certificate is the reliable local option.
- Live bank testing depends on human-owned credentials and SCA. Mitigation: CI uses fake servers; live steps are manual and documented.
- Large transaction exports could contain sensitive data. Mitigation: ignored output paths, explicit security warnings, no committed live fixtures, and bounded logs.
- POC code in `package main` may grow. Mitigation: split by provider-focused files and keep interfaces local to tests/consumers; defer product-grade packages until a future accepted architecture change.

## Migration Plan

1. Add the isolated command tree and tests under `apps/signal-foundry/cmd/signal-foundry`.
2. Add provider setup docs under `apps/signal-foundry/doc/financial-poc/`.
3. Confirm/add ignore rules for financial POC local artifacts.
4. Apply the local HTTPS callback correction to the Enable Banking `connect` flow and docs, including paired trusted-cert flag behavior and one-sided flag validation.
5. Complete final implementation checks from the repository root with `make affected-lint-test` only after the HTTPS callback correction is implemented and its focused tests pass.
6. Check whether AGENTS.md needs updates when commands, workflows, or architecture changed, and update it only if needed.

Rollback is deleting the `finance-poc` command files/tests/docs and any added ignore patterns. No product data migration is required.

## Open Questions

- Exact Enable Banking PKO ASPSP naming should be discovered by the `aspsps --country PL` command during live manual testing; docs should instruct the tester to use the returned value.
- If Enable Banking requires additional optional `POST /auth` fields for PKO beyond the issue summary, implementation may expose narrow flags only as needed for the POC and document them.
