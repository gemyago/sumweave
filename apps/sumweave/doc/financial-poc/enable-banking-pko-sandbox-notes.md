# Enable Banking / PKO sandbox operator notes

Use this as the short operator runbook for sandbox work. It complements
[`enable-banking-pko.md`](./enable-banking-pko.md) and
[`docs/manual-e2e.md`](../../../../docs/manual-e2e.md).

## Setup

- Register an Enable Banking app in the sandbox; register production only if you
  need it.
- Keep the private key outside the repo and never commit it.
- `ENABLE_BANKING_APP_ID` must match the registered app id; it is the JWT `kid`.
- Local app redirect start also reads optional
  `APP_FINANCE_PROVIDERS_ENABLEBANKING_CALLBACKBASEURL`
  when the registered callback origin differs from the current backend host.
- Local app redirect start defaults to the current shared sandbox ASPSP,
  `Mock ASPSP`.
- If the shared sandbox app changes, rediscover the exposed ASPSP names and
  update `APP_FINANCE_PROVIDERS_ENABLEBANKING_ASPSPNAME` before testing again.
- JWT `iss` is always `enablebanking.com` and `aud` is always
  `api.enablebanking.com`.
- Use `ENABLE_BANKING_BASE_URL` only when you need a sandbox or fake-provider
  endpoint.
- For local browser callback listening, prefer a trusted cert/key pair. If you
  pass one callback cert flag, pass the matching key flag too.

## Callback and return shape

- Backend callback endpoint: `/enable-banking/callback`.
- The product-facing PKO flow should hand the browser to `{origin}/#/finance/connections`.
- Enable Banking returns the browser as
  `{origin}/?code=...&state=...#/finance/connections`.
- The browser query string should be consumed only after a successful finish;
  on failure, keep `code`/`state` so the operator can refresh or reopen and
  retry from the same hash route.
- Treat the server-side pending link state as the source of truth; do not rely
  on browser-returned metadata beyond `code` and `state`.

## Manual operator flow

1. Confirm the shared sandbox ASPSP name first. Right now this should be
   `Mock ASPSP`:

   ```bash
   go run ./cmd/sumweave finance-poc enable-banking aspsps --country PL --json
   ```

2. Start auth with either:
   - `connect` for the same-machine browser flow, using `--callback-listen-addr`
     plus a trusted cert/key pair and `--open-browser`, or
   - `start-auth` when you need a tunnel/public callback, then open the auth URL
     manually and finish with `finish-session`.
3. A human must complete the external bank login and SCA.
4. After the browser returns with `code` and `state`, finish the session.
5. Only then read accounts or transactions.

## Gotchas

- AI agents cannot complete external bank SCA.
- Current shared sandbox/operator access uses `Mock ASPSP` instead of PKO; keep
  `APP_FINANCE_PROVIDERS_ENABLEBANKING_ASPSPNAME="Mock ASPSP"` set unless
  `aspsps --country PL`
  shows the sandbox inventory changed.
- The backend-driven callback URL `http://localhost:4501/enable-banking/callback`
  is accepted by the current shared sandbox; if start still fails before redirect,
  the usual cause is the ASPSP name, not the callback URL.
- For the current shared sandbox app, `PKO Bank Polski` may be unavailable while
  `Mock ASPSP` or `Bank Millennium` is available. Set
  `APP_FINANCE_PROVIDERS_ENABLEBANKING_ASPSPNAME` to the discovered sandbox
  entry before running the UI flow.
- Mock-bank sandbox flow can still require an Enable Banking sign-in and CAPTCHA,
  which is external to the repo and blocks unattended completion.
- If local HTTPS is blocked, use the manual `start-auth` / `finish-session`
  path with a reachable callback instead of forcing `connect`.
- Trusted local certs must cover the callback host (`localhost` vs
  `127.0.0.1`).
- PKO production access may stay restricted until the linked account is
  approved/activated in the bank and provider setup.
- Keep private keys, auth/session files, and exported data out of git-tracked
  paths.
