# Enable Banking API Client Endpoints

GET /aspsps
Client method: ListASPSPs(ctx, ListASPSPsParams)

POST /auth
Client method: CreateAuth(ctx, CreateAuthParams)

POST /sessions
Client method: CreateSession(ctx, CreateSessionParams)

GET /sessions/{sessionId}
Client method: GetSession(ctx, GetSessionParams)

GET /accounts
Client method: ListAccounts(ctx, ListAccountsParams)

GET /accounts/{accountId}/details
Client method: GetAccountDetails(ctx, GetAccountDetailsParams)

GET /accounts/{accountId}/balances
Client method: GetAccountBalances(ctx, GetAccountBalancesParams)

GET /accounts/{accountId}/transactions
Client method: GetAccountTransactions(ctx, GetAccountTransactionsParams)
