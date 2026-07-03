# Enable Banking PKO proof of concept

Run these commands from `apps/signal-foundry` with `go run ./cmd/signal-foundry ...`.

## Setup

- Register an Enable Banking app in both the sandbox and production environments if you need both.
- Keep the Enable Banking private key outside the repo, for example under `~/.config/...`; do not commit it.
- The Enable Banking application id is used as the JWT `kid`, so the value behind `ENABLE_BANKING_APP_ID` must match the registered app. JWT `iss` is always `enablebanking.com` and `aud` is always `api.enablebanking.com` (fixed by the provider, not derived from env).
- The local `connect` flow redirects PKO back to `https://<listen-addr>/callback`. Create a browser-trusted certificate for that host once (see [Trusted local HTTPS certificate](#trusted-local-https-certificate-one-time)); without it, the CLI falls back to an ephemeral self-signed cert and the browser or PKO may block the callback.
- The current shared sandbox flow should use `APP_FINANCE_PROVIDERS_ENABLEBANKING_ASPSPNAME="Mock ASPSP"` on the backend. If that sandbox app changes, discover the available sandbox ASPSPs before treating a start-link failure as a callback problem, then update `APP_FINANCE_PROVIDERS_ENABLEBANKING_ASPSPNAME` to the exposed entry (`Mock ASPSP`, `Bank Millennium`, or another non-PKO name).
- `--callback-cert-file` and `--callback-key-file` must be provided together. When both are omitted, the CLI uses that self-signed fallback in memory only (nothing is written to disk).
- Use either:
  - an HTTPS localhost callback flow with `connect --callback-listen-addr 127.0.0.1:8085` when the browser runs on the same machine, ideally with a trusted certificate pair, or
  - a manual redirect URL plus a tunnel/public callback for `start-auth` and `finish-session` when localhost is not reachable or when the self-signed fallback is not accepted.
- PKO production access may stay restricted until the linked account is activated/approved in the bank and provider setup.

## Environment

- Required: `ENABLE_BANKING_APP_ID`
- Required: `ENABLE_BANKING_PRIVATE_KEY_PATH`
- Optional: `ENABLE_BANKING_BASE_URL` (defaults to the live Enable Banking API unless overridden)

Recommended artifact location from this working directory: `../../data/financial-poc/`.

## Trusted local HTTPS certificate (one-time)

Enable Banking requires an HTTPS redirect URL. `connect` serves the callback listener over TLS, but browsers and some bank flows reject untrusted certificates.

Recommended setup with [mkcert](https://github.com/FiloSottile/mkcert) (install via Homebrew on macOS, or see the mkcert README for Linux/Windows):

```bash
mkdir -p ~/.config/enable-banking
brew install mkcert
mkcert -install
mkcert \
  -cert-file ~/.config/enable-banking/localhost-cert.pem \
  -key-file ~/.config/enable-banking/localhost-key.pem \
  localhost 127.0.0.1 ::1
```

`mkcert -install` adds a local CA to your system trust store so Chrome/Safari/Firefox accept the certificate. Keep the key file outside the repo.

The certificate must cover the host in `--callback-listen-addr`. The examples below use `127.0.0.1:8085`, so `127.0.0.1` must be in the SAN list above. If you switch to `localhost:8085`, ensure `localhost` is included instead.

Pass the generated pair to `connect` with `--callback-cert-file` and `--callback-key-file` (both flags are required together).

## Discover PKO / ASPSPs

List Polish ASPSPs and confirm the sandbox-exposed entry before starting auth. On the current shared sandbox, expect `Mock ASPSP` rather than `PKO Bank Polski`:

```bash
go run ./cmd/signal-foundry finance-poc enable-banking aspsps --country PL --json
```

## Manual auth flow

1. Start auth and save the pending auth file:

```bash
go run ./cmd/signal-foundry finance-poc enable-banking start-auth \
  --country PL \
  --aspsp-name "PKO Bank Polski" \
  --psu-type personal \
  --valid-days 90 \
  --redirect-url https://localhost:8085/enable-banking/callback \
  --auth-file ../../data/financial-poc/pko.enable-banking-pending-auth.json \
  --json
```

2. A human must open the returned authorization URL, sign in to PKO, and complete strong customer authentication. An AI agent cannot complete PKO strong customer authentication.
3. After the callback returns `code` and `state`, finish the session:

```bash
go run ./cmd/signal-foundry finance-poc enable-banking finish-session \
  --auth-file ../../data/financial-poc/pko.enable-banking-pending-auth.json \
  --code '<callback code>' \
  --state '<callback state>' \
  --session-file ../../data/financial-poc/pko.enable-banking-session.json \
  --json
```

The manual setup is intentionally `start-auth` followed by `finish-session --auth-file ... --code ... --state ...`.

## Local callback convenience flow

If the human browser is local to the machine running the CLI, `connect` can create the HTTPS localhost callback URL for you:

Preferred: use the trusted certificate from [Trusted local HTTPS certificate](#trusted-local-https-certificate-one-time):

```bash
go run ./cmd/signal-foundry finance-poc enable-banking connect \
  --country PL \
  --aspsp-name "PKO Bank Polski" \
  --psu-type personal \
  --valid-days 90 \
  --callback-listen-addr 127.0.0.1:8085 \
  --callback-cert-file ~/.config/enable-banking/localhost-cert.pem \
  --callback-key-file ~/.config/enable-banking/localhost-key.pem \
  --session-file ../../data/financial-poc/pko.enable-banking-session.json \
  --open-browser \
  --json
```

If you omit both callback cert flags, the CLI generates a self-signed certificate in memory for this run only. That fallback does not write key material to disk, but the browser may warn and some flows may still refuse the callback:

```bash
go run ./cmd/signal-foundry finance-poc enable-banking connect \
  --country PL \
  --aspsp-name "PKO Bank Polski" \
  --psu-type personal \
  --valid-days 90 \
  --callback-listen-addr 127.0.0.1:8085 \
  --session-file ../../data/financial-poc/pko.enable-banking-session.json \
  --open-browser \
  --json
```

If local HTTPS is still blocked, use the manual `start-auth` / `finish-session` flow with a tunnel or another public HTTPS callback instead.

## Read accounts and transactions

```bash
go run ./cmd/signal-foundry finance-poc enable-banking accounts \
  --session-file ../../data/financial-poc/pko.enable-banking-session.json \
  --include-details \
  --include-balances \
  --json

go run ./cmd/signal-foundry finance-poc enable-banking transactions \
  --session-file ../../data/financial-poc/pko.enable-banking-session.json \
  --account-id '<account uid>' \
  --from 2026-06-01 \
  --to 2026-06-15 \
  --out ../../data/financial-poc/pko.enable-banking-transactions.json \
  --json
```

## AI-agent live test notes

- Agents can verify command wiring, env loading, file paths, and JSON structure offline.
- Live PKO testing still needs a human for bank login and SCA.
- Keep the private key and any generated session/export files out of git-tracked locations.
- For a shorter sandbox/operator runbook, see
  [Enable Banking / PKO sandbox operator notes](./enable-banking-pko-sandbox-notes.md).
