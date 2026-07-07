# Review Chunk 4: manual e2e validation

## Round 1 - 2026-07-07

- Phase: initial implementation phase
- Scope: manual e2e validation only
- `openspec apply`:
  - attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change repair-enablebanking-client-contract --task 4.1 --task 4.2 --task 4.3 --task 4.4 --task 4.5`
  - result: failed because the installed CLI reports `unknown command 'apply'`
- Environment:
  - repo shell loaded through `direnv exec /Users/jenya/projects/signal-foundry ...`
  - user: `e2e-manual-20260616-1`
  - headed browser: `npx playwright-cli -s=repair-eb open --headed "http://127.0.0.1:5173/#/login"`
  - backend/frontend processes: PM2 `signal-foundry-api` and `signal-foundry-ui`
  - tenant created for this run: `Chunk4 Mock ASPSP 20260707-1305` (`tenantId=371ec657-7ba8-4a65-a0de-0561e1cb2c56`)
- Preparation and checks run:
  - `direnv exec /Users/jenya/projects/signal-foundry npx playwright-cli --version`
  - `direnv exec /Users/jenya/projects/signal-foundry pm2 status`
  - `direnv exec /Users/jenya/projects/signal-foundry go run ./apps/signal-foundry/cmd/signal-foundry db-migrate --env local`
  - `direnv exec /Users/jenya/projects/signal-foundry pm2 restart signal-foundry-api --update-env`
  - `direnv exec /Users/jenya/projects/signal-foundry pm2 restart signal-foundry-ui --update-env`
  - `curl -i "http://127.0.0.1:4501/health"`
- Validation steps and results:
  1. Signed in at `#/login` with the first `.local-users` entry, created tenant `Chunk4 Mock ASPSP 20260707-1305`, and confirmed `Connect PKO with bank login` was enabled on `#/finance/connections`.
  2. Started PKO linking from Finance connections, clicked `Continue authentication`, signed in to Enable Banking with primary AgentMail inbox `signalf-e2e-a1@agentmail.to`, opened the one-time email link in the same browser session, and reached Mock ASPSP authorization.
  3. In `https://enablebanking.com/cp/mock-aspsp`, inspected account `PKO E2E Sample`, added one balance row `Closing accounting balance / CLBD / 70.42 EUR`, and confirmed two existing booked debit transactions before initial sync:
     - `Signal Foundry test transaction 1` / `-12.34 EUR` / `2026-07-07`
     - `Oliver Virtanen-DBIT-3.36-gzros` / `-3.36 EUR` / `2026-07-06`
  4. Returned to the authorization tab, selected only `PKO E2E Sample`, clicked `Authorize`, and confirmed redirect back to `http://127.0.0.1:5173/#/finance/connections`.
  5. Verified linked connection card details on Finance connections:
     - provider label: `pko`
     - connection title: `Mock ASPSP`
     - state: `active`
     - provider ref: `482da3af-fec9-4dc0-960c-44227c769977`
  6. Triggered first sync by clicking `Sync now` on the new connection card. The UI surfaced `Open latest finance job`, which opened job `019f3c44-6bc9-7e98-8f48-d4e0e49daca2`.
  7. Confirmed first sync job succeeded on `#/finance/jobs/019f3c44-6bc9-7e98-8f48-d4e0e49daca2`:
     - job type: `finance.bank_connection_sync`
     - status: `succeeded`
     - worker: `jobs-worker`
     - attempt count: `1`
  8. Verified imported data after first sync:
     - Finance accounts UI showed `1 account`
     - account detail UI showed `2 transactions`
     - browser-captured `GET /api/v1/finance/tenants/371ec657-7ba8-4a65-a0de-0561e1cb2c56/accounts` response confirmed balance data was present with `bookedBalanceMinor=-1570` and `pendingBalanceMinor=0`
     - browser-captured `GET /api/v1/finance/tenants/371ec657-7ba8-4a65-a0de-0561e1cb2c56/transactions?accountId=7b61323d-5531-49c4-ae35-a1cc350289bc` response confirmed the two imported provider transactions with ids `a645df75-c2d5-4c6b-87ad-b509862a43ce` and `5e96d768-5bc2-45ca-b561-a879f5875d0e`
  9. Added one new Mock ASPSP transaction on the same account before re-sync:
     - `entryReference=chunk4-tx-20260707-1309`
     - description/remittance: `Chunk4 resync tx`
     - amount: `8.88 EUR` debit
     - booking/value date: `2026-07-07`
     - Mock ASPSP account page then showed `3` transaction rows.
  10. Triggered second sync from Finance connections with the same `Sync now` button. The UI surfaced a new latest job `019f3c46-54b8-7e9a-9b86-330ec72aa707`.
  11. Confirmed second sync job succeeded on `#/finance/jobs/019f3c46-54b8-7e9a-9b86-330ec72aa707`:
      - job type: `finance.bank_connection_sync`
      - status: `succeeded`
      - worker: `jobs-worker`
      - attempt count: `1`
  12. Verified re-sync results:
      - Finance account detail UI now showed `3 transactions`
      - newly added transaction appeared first as `Chunk4 resync tx` for `-8.88 EUR`
      - original imported transactions `Signal Foundry test transaction 1` and `Oliver Virtanen-DBIT-3.36-gzros` remained present
      - browser-captured accounts response now showed `bookedBalanceMinor=-2458`
      - browser-captured transactions response now contained three provider rows, including new transaction id `e62ea917-086a-4f39-8af1-bacc69dee5cc`
- OpenSpec task updates:
  - marked `4.1` complete in `tasks.md`
  - marked `4.2` complete in `tasks.md`
  - marked `4.3` complete in `tasks.md`
  - marked `4.4` complete in `tasks.md`
  - marked `4.5` complete in `tasks.md`
- Artifact cleanup status:
  - closed Playwright session `repair-eb`
  - no durable repo artifact was added outside standard OpenSpec files
  - existing repo-local `.playwright-cli/` temp directory predates this chunk and was left unchanged
- Blockers:
  - no blocking product issues in this chunk
  - minor tooling note only: `openspec apply` is unavailable in the installed CLI (`unknown command 'apply'`)
- Result: complete
