# monobank proof of concept

Run these commands from `apps/sumweave` with `go run ./cmd/sumweave ...`.

## Setup

- Generate a monobank personal token from the monobank app/account settings flow.
- Store the token outside the repo and pass it through `MONOBANK_TOKEN` or `--token`; never commit it.
- Optional: set `MONOBANK_BASE_URL` only for non-default/fake-server testing.
- Recommended artifact location from this working directory: `../../data/financial-poc/`.

## Environment

- Required: `MONOBANK_TOKEN`
- Optional: `MONOBANK_BASE_URL`

## Accounts

```bash
go run ./cmd/sumweave finance-poc monobank accounts --json
```

Use the returned account ids when you need a specific account. If you omit `--account`, the transactions command uses the default account `0`.

## Transactions

```bash
go run ./cmd/sumweave finance-poc monobank transactions \
  --account 0 \
  --from 2026-06-01 \
  --to 2026-06-15 \
  --out ../../data/financial-poc/personal.monobank-transactions.json \
  --json
```

## Limits and live-call warnings

- monobank statement calls are capped to 31 days plus 1 hour per request range.
- The CLI splits longer ranges automatically and sleeps `61s` between chunks by default to stay clear of the 60-second endpoint limits.
- repeated live calls can hit monobank rate limits, so keep live ranges small and avoid loops against the real API.

## AI-agent live test notes

- An AI agent can safely run offline tests without real bank credentials.
- For live verification, keep the range bounded, list accounts first, then fetch one short statement window.
- A typical live sequence is `monobank accounts --json` followed by `monobank transactions --account 0 --from YYYY-MM-DD --to YYYY-MM-DD --json`.
