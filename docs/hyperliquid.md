# Hyperliquid Reference

Short reference for Signal Foundry venue integration planning.

## Current repo context

- Current Hyperliquid code in [runtime/venueedge/hyperliquid_perps.go](../runtime/venueedge/hyperliquid_perps.go) is market-data only.
- The adapter currently uses public `POST /info` requests for:
  - `meta`
  - `candleSnapshot`
  - `recentTrades`
- Authenticated trading, account state, and wallet approval flows are not implemented yet.
- The manual live runtime smoke is documented in [live-hyperliquid-smoke.md](./live-hyperliquid-smoke.md).

## Account model and onboarding

- Hyperliquid does **not** use a typical centralized-exchange API key and secret pair.
- The user account is a blockchain address. The normal onboarding flow is:
  - Connect a wallet or log in with email
  - Click `Enable Trading`
  - Deposit funds
- Email login creates a new blockchain address for that email login.
- Mainnet example API URL: `https://api.hyperliquid.xyz`
- Testnet example API URL: `https://api.hyperliquid-testnet.xyz`

## API wallets

- Hyperliquid calls authenticated signer wallets `API wallets` or `agent wallets`.
- A master account approves an API wallet by sending an `approveAgent` action to `/exchange`.
- API wallets are for **signing only**.
- Read queries for balances, positions, or account state must use the real master, subaccount, or vault address rather than the agent wallet address.
- One account can have:
  - `1` unnamed approved API wallet
  - up to `3` named approved API wallets
  - plus `2` additional named agents per subaccount

## Nonces and signing

- Hyperliquid recommends using an existing SDK for signing rather than hand-rolling signatures.
- Nonces are tracked per **signer** address.
- When using an API wallet, nonce tracking belongs to the API wallet address, not the master account.
- Hyperliquid stores the `100` highest nonces per signer.
- A new nonce must be unused and larger than the smallest nonce in that stored set.
- Nonces must be within `(T - 2 days, T + 1 day)`, where `T` is the block timestamp in unix milliseconds.
- The docs recommend using the current unix millisecond timestamp as the nonce.
- Use a separate API wallet per trading process and preferably per subaccount to avoid nonce collisions.

## API wallet lifecycle risks

- API wallets can effectively disappear for future use when:
  - they are replaced by a new approval
  - they expire
  - the registering account no longer has funds
- Hyperliquid explicitly recommends **not** reusing old API wallet addresses.
- Once an old wallet is pruned, its nonce state may also be pruned, which can make previously signed actions replayable.

## Testnet and manual testing implications

- Hyperliquid's current testnet faucet requires a **mainnet deposit from the same address** before testnet funds can be claimed.
- If email login is used, Hyperliquid says mainnet and testnet use different generated wallets, so the email wallet may need to be exported and imported into a wallet extension for testnet use.
- New HyperCore accounts also incur a one-time activation fee of `1` quote token on the first transaction sent to that new account.
- For Signal Foundry, this means the cleanest manual live-test setup is:
  - keep a dedicated master wallet for venue testing
  - bootstrap it once on mainnet with a minimal deposit
  - use testnet for routine live smoke tests
  - generate a fresh API wallet for each manual test session or runner

## Suggested manual live-test scope

- Public-data smoke:
  - read market metadata
  - read candles
  - read recent trades
- Auth smoke:
  - approve a fresh API wallet
  - verify `userRole` for master and agent addresses
  - query account state using the master address
  - submit a signed `noop` action to prove signing and nonce flow without trading side effects
- Optional trading smoke later:
  - move a small balance into the required trading wallet
  - place one tiny test order
  - verify status
  - cancel it

## Sources

- API overview: <https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api>
- Account onboarding: <https://hyperliquid.gitbook.io/hyperliquid-docs/onboarding/how-to-start-trading>
- Testnet faucet: <https://hyperliquid.gitbook.io/hyperliquid-docs/onboarding/testnet-faucet>
- API wallets and nonces: <https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/nonces-and-api-wallets>
- Exchange endpoint: <https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/exchange-endpoint>
- Signing: <https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/signing>
- Rate limits: <https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/rate-limits-and-user-limits>
- Activation gas fee: <https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/activation-gas-fee>
