package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	enableBankingRawSessionKey = "session"
	enableBankingStatusBoth    = "both"
	enableBankingStatusBooked  = "booked"
	enableBankingStatusPending = "pending"
)

type enableBankingAccountsParams struct {
	SessionFile     string
	IncludeDetails  bool
	IncludeBalances bool
}

type enableBankingTransactionsParams struct {
	SessionFile string
	AccountID   string
	From        string
	To          string
	Strategy    string
	Status      string
}

type enableBankingAccountsOutput struct {
	Provider  string                      `json:"provider"`
	Operation string                      `json:"operation"`
	FetchedAt string                      `json:"fetchedAt"`
	SessionID string                      `json:"sessionId"`
	Accounts  []enableBankingAccountEntry `json:"accounts"`
	Raw       map[string]any              `json:"raw"`
}

type enableBankingAccountEntry struct {
	UID         string           `json:"uid"`
	Name        string           `json:"name,omitempty"`
	IBAN        string           `json:"iban,omitempty"`
	Currency    string           `json:"currency,omitempty"`
	Details     map[string]any   `json:"details,omitempty"`
	Balances    []map[string]any `json:"balances,omitempty"`
	Raw         map[string]any   `json:"raw"`
	DetailsRaw  map[string]any   `json:"detailsRaw,omitempty"`
	BalancesRaw []map[string]any `json:"balancesRaw,omitempty"`
}

type enableBankingTransactionsOutput struct {
	Provider         string                          `json:"provider"`
	Operation        string                          `json:"operation"`
	FetchedAt        string                          `json:"fetchedAt"`
	SessionID        string                          `json:"sessionId"`
	AccountID        string                          `json:"accountId"`
	From             string                          `json:"from"`
	To               string                          `json:"to"`
	Strategy         string                          `json:"strategy,omitempty"`
	Status           string                          `json:"status"`
	TransactionCount int                             `json:"transactionCount"`
	Transactions     []enableBankingTransactionEntry `json:"transactions"`
	Raw              map[string]any                  `json:"raw"`
}

type enableBankingTransactionEntry struct {
	TransactionID         string         `json:"transactionId,omitempty"`
	Status                string         `json:"status,omitempty"`
	BookingDate           string         `json:"bookingDate,omitempty"`
	ValueDate             string         `json:"valueDate,omitempty"`
	Amount                string         `json:"amount,omitempty"`
	Currency              string         `json:"currency,omitempty"`
	CreditDebitIndicator  string         `json:"creditDebitIndicator,omitempty"`
	RemittanceInformation string         `json:"remittanceInformation,omitempty"`
	Raw                   map[string]any `json:"raw"`
}

func newEnableBankingAccountsCmd(
	deps financePOCCommandDeps,
	requestParams *enableBankingCommandParams,
) *cobra.Command {
	commandParams := enableBankingAccountsParams{}
	cmd := &cobra.Command{
		Use:   enableBankingAccountsOp,
		Short: "List session accounts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			request := newEnableBankingBaseRequest(cmd, requestParams)
			request.Operation = enableBankingAccountsOp

			result, err := runEnableBankingAccounts(
				cmd.Context(),
				cmd.ErrOrStderr(),
				deps,
				request,
				commandParams,
			)
			if err != nil {
				return err
			}
			return writeFinancePOCJSONPayload(
				cmd.OutOrStdout(),
				request.OutputFile,
				request.JSON,
				result,
				fmt.Sprintf("accounts: %d\n", len(result.Accounts)),
			)
		},
	}
	cmd.Flags().StringVar(&commandParams.SessionFile, "session-file", "", "Saved session file")
	cmd.Flags().BoolVar(
		&commandParams.IncludeDetails,
		"include-details",
		false,
		"Include account details",
	)
	cmd.Flags().BoolVar(
		&commandParams.IncludeBalances,
		"include-balances",
		false,
		"Include account balances",
	)
	_ = cmd.MarkFlagRequired("session-file")
	return cmd
}

func newEnableBankingTransactionsCmd(
	deps financePOCCommandDeps,
	requestParams *enableBankingCommandParams,
) *cobra.Command {
	commandParams := enableBankingTransactionsParams{
		Status: enableBankingStatusBoth,
	}
	cmd := &cobra.Command{
		Use:   enableBankingTransactionsOp,
		Short: "List account transactions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			request := newEnableBankingBaseRequest(cmd, requestParams)
			request.Operation = enableBankingTransactionsOp

			result, err := runEnableBankingTransactions(
				cmd.Context(),
				cmd.ErrOrStderr(),
				deps,
				request,
				commandParams,
			)
			if err != nil {
				return err
			}
			return writeFinancePOCJSONPayload(
				cmd.OutOrStdout(),
				request.OutputFile,
				request.JSON,
				result,
				fmt.Sprintf("transactions: %d\n", result.TransactionCount),
			)
		},
	}
	cmd.Flags().StringVar(&commandParams.SessionFile, "session-file", "", "Saved session file")
	cmd.Flags().StringVar(&commandParams.AccountID, "account-id", "", "Account identifier")
	cmd.Flags().StringVar(
		&commandParams.From,
		"from",
		"",
		"Inclusive start date (YYYY-MM-DD)",
	)
	cmd.Flags().StringVar(
		&commandParams.To,
		"to",
		"",
		"Inclusive end date (YYYY-MM-DD)",
	)
	cmd.Flags().StringVar(&commandParams.Strategy, "strategy", "", "Optional provider strategy")
	cmd.Flags().StringVar(&commandParams.Status, "status", commandParams.Status, "booked, pending, or both")
	_ = cmd.MarkFlagRequired("session-file")
	_ = cmd.MarkFlagRequired("account-id")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

func runEnableBankingAccounts(
	ctx context.Context,
	stderr io.Writer,
	deps financePOCCommandDeps,
	request financePOCProviderRequest,
	params enableBankingAccountsParams,
) (enableBankingAccountsOutput, error) {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	ctx, cancel := withFinancePOCTimeout(ctx, request.Timeout)
	defer cancel()

	sessionID, sessionRaw, err := loadEnableBankingConfirmedSession(
		ctx,
		request,
		params.SessionFile,
		deps.Now,
	)
	if err != nil {
		return enableBankingAccountsOutput{}, err
	}

	accountsRaw := extractEnableBankingAccounts(sessionRaw)
	accounts := make([]enableBankingAccountEntry, 0, len(accountsRaw))
	detailsRaw := map[string]any{}
	balancesRaw := map[string]any{}
	for _, item := range accountsRaw {
		account := enableBankingAccountEntry{
			UID:      extractEnableBankingString(item, "uid", "id"),
			Name:     extractEnableBankingString(item, "name"),
			IBAN:     extractEnableBankingString(item, "iban"),
			Currency: extractEnableBankingString(item, "currency"),
			Raw:      item,
		}
		if account.UID == "" {
			accounts = append(accounts, account)
			continue
		}

		if params.IncludeDetails {
			currentDetailsRaw, detailsErr := callEnableBankingJSONEndpoint(
				ctx,
				request,
				http.MethodGet,
				"/accounts/"+url.PathEscape(account.UID)+"/details",
				nil,
				deps.Now,
			)
			if detailsErr != nil {
				return enableBankingAccountsOutput{}, detailsErr
			}
			account.DetailsRaw = currentDetailsRaw
			account.Details = normalizeEnableBankingAccountDetails(currentDetailsRaw)
			detailsRaw[account.UID] = currentDetailsRaw
		}

		if params.IncludeBalances {
			currentBalancesRaw, balancesErr := callEnableBankingJSONEndpoint(
				ctx,
				request,
				http.MethodGet,
				"/accounts/"+url.PathEscape(account.UID)+"/balances",
				nil,
				deps.Now,
			)
			if balancesErr != nil {
				return enableBankingAccountsOutput{}, balancesErr
			}
			account.BalancesRaw = extractEnableBankingBalances(currentBalancesRaw)
			account.Balances = normalizeEnableBankingBalances(account.BalancesRaw)
			balancesRaw[account.UID] = currentBalancesRaw
		}

		accounts = append(accounts, account)
	}

	writeFinancePOCProgressf(stderr, "retrieved %d accounts", len(accounts))
	raw := map[string]any{enableBankingRawSessionKey: sessionRaw}
	if len(detailsRaw) > 0 {
		raw["details"] = detailsRaw
	}
	if len(balancesRaw) > 0 {
		raw["balances"] = balancesRaw
	}

	return enableBankingAccountsOutput{
		Provider:  enableBankingCommandName,
		Operation: enableBankingAccountsOp,
		FetchedAt: deps.Now().Format(time.RFC3339),
		SessionID: sessionID,
		Accounts:  accounts,
		Raw:       raw,
	}, nil
}

func runEnableBankingTransactions(
	ctx context.Context,
	stderr io.Writer,
	deps financePOCCommandDeps,
	request financePOCProviderRequest,
	params enableBankingTransactionsParams,
) (enableBankingTransactionsOutput, error) {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	ctx, cancel := withFinancePOCTimeout(ctx, request.Timeout)
	defer cancel()

	fromDate, toDate, statusValue, err := validateEnableBankingTransactionsParams(params)
	if err != nil {
		return enableBankingTransactionsOutput{}, err
	}
	sessionID, sessionRaw, err := loadEnableBankingConfirmedSession(
		ctx,
		request,
		params.SessionFile,
		deps.Now,
	)
	if err != nil {
		return enableBankingTransactionsOutput{}, err
	}

	transactions, pages, err := fetchEnableBankingTransactionsPages(
		ctx,
		request,
		strings.TrimSpace(params.AccountID),
		fromDate,
		toDate,
		strings.TrimSpace(params.Strategy),
		statusValue,
		deps.Now,
	)
	if err != nil {
		return enableBankingTransactionsOutput{}, err
	}

	writeFinancePOCProgressf(stderr, "retrieved %d transactions", len(transactions))
	return enableBankingTransactionsOutput{
		Provider:         enableBankingCommandName,
		Operation:        enableBankingTransactionsOp,
		FetchedAt:        deps.Now().Format(time.RFC3339),
		SessionID:        sessionID,
		AccountID:        strings.TrimSpace(params.AccountID),
		From:             fromDate.Format(financePOCDateLayout),
		To:               toDate.Format(financePOCDateLayout),
		Strategy:         strings.TrimSpace(params.Strategy),
		Status:           normalizeEnableBankingRequestedStatus(params.Status),
		TransactionCount: len(transactions),
		Transactions:     transactions,
		Raw: map[string]any{
			enableBankingRawSessionKey: sessionRaw,
			"pages":                    pages,
		},
	}, nil
}

func loadEnableBankingConfirmedSession(
	ctx context.Context,
	request financePOCProviderRequest,
	sessionFilePath string,
	now func() time.Time,
) (string, map[string]any, error) {
	sessionFile, err := loadEnableBankingSessionFile(strings.TrimSpace(sessionFilePath))
	if err != nil {
		return "", nil, err
	}
	sessionID := strings.TrimSpace(sessionFile.SessionID)
	if sessionID == "" {
		return "", nil, errors.New("enable-banking session file missing session ID")
	}
	credentialsErr := validateEnableBankingCredentials(request)
	if credentialsErr != nil {
		return "", nil, credentialsErr
	}
	sessionRaw, err := callEnableBankingJSONEndpoint(
		ctx,
		request,
		http.MethodGet,
		"/sessions/"+url.PathEscape(sessionID),
		nil,
		now,
	)
	if err != nil {
		return "", nil, err
	}
	return sessionID, sessionRaw, nil
}

func validateEnableBankingTransactionsParams(
	params enableBankingTransactionsParams,
) (time.Time, time.Time, string, error) {
	fromDate, err := parseFinancePOCDate("--from", params.From)
	if err != nil {
		return time.Time{}, time.Time{}, "", err
	}
	toDate, err := parseFinancePOCDate("--to", params.To)
	if err != nil {
		return time.Time{}, time.Time{}, "", err
	}
	if toDate.Before(fromDate) {
		return time.Time{}, time.Time{}, "", errors.New("enable-banking --to must be on or after --from")
	}
	statusValue, err := resolveEnableBankingTransactionStatus(params.Status)
	if err != nil {
		return time.Time{}, time.Time{}, "", err
	}
	return fromDate, toDate, statusValue, nil
}

func fetchEnableBankingTransactionsPages(
	ctx context.Context,
	request financePOCProviderRequest,
	accountID string,
	fromDate time.Time,
	toDate time.Time,
	strategy string,
	statusValue string,
	now func() time.Time,
) ([]enableBankingTransactionEntry, []map[string]any, error) {
	pages := make([]map[string]any, 0, 1)
	transactions := make([]enableBankingTransactionEntry, 0)
	continuationKey := ""
	for {
		query := makeEnableBankingTransactionsQuery(
			fromDate,
			toDate,
			strategy,
			statusValue,
			continuationKey,
		)
		pageRaw, err := callEnableBankingJSONEndpointWithQuery(
			ctx,
			request,
			http.MethodGet,
			"/accounts/"+url.PathEscape(accountID)+"/transactions",
			query,
			nil,
			now,
		)
		if err != nil {
			return nil, nil, err
		}
		pages = append(pages, pageRaw)
		for _, item := range extractEnableBankingTransactionItems(pageRaw) {
			transactions = append(transactions, normalizeEnableBankingTransaction(item))
		}
		continuationKey = extractEnableBankingString(pageRaw, "continuation_key")
		if continuationKey == "" {
			break
		}
	}
	return transactions, pages, nil
}

func makeEnableBankingTransactionsQuery(
	fromDate time.Time,
	toDate time.Time,
	strategy string,
	statusValue string,
	continuationKey string,
) url.Values {
	query := url.Values{}
	query.Set("date_from", fromDate.Format(financePOCDateLayout))
	query.Set("date_to", toDate.Format(financePOCDateLayout))
	if strategy != "" {
		query.Set("strategy", strategy)
	}
	if statusValue != "" {
		query.Set("status", statusValue)
	}
	if continuationKey != "" {
		query.Set("continuation_key", continuationKey)
	}
	return query
}

func normalizeEnableBankingAccountDetails(raw map[string]any) map[string]any {
	account, ok := raw["account"].(map[string]any)
	if !ok {
		account = raw
	}
	details := map[string]any{}
	if ownerName := extractEnableBankingString(account, "owner_name", "ownerName"); ownerName != "" {
		details["ownerName"] = ownerName
	}
	if product := extractEnableBankingString(account, "product"); product != "" {
		details["product"] = product
	}
	if bic := extractEnableBankingString(account, "bic"); bic != "" {
		details["bic"] = bic
	}
	if len(details) == 0 {
		return nil
	}
	return details
}

func extractEnableBankingBalances(raw map[string]any) []map[string]any {
	items, ok := raw["balances"].([]any)
	if !ok {
		return nil
	}
	balances := make([]map[string]any, 0, len(items))
	for _, item := range items {
		balance, okCast := item.(map[string]any)
		if okCast {
			balances = append(balances, balance)
		}
	}
	return balances
}

func normalizeEnableBankingBalances(raw []map[string]any) []map[string]any {
	balances := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		balanceAmount, _ := item["balance_amount"].(map[string]any)
		balance := map[string]any{}
		if balanceType := extractEnableBankingString(item, "type"); balanceType != "" {
			balance["type"] = balanceType
		}
		if amount := extractEnableBankingString(balanceAmount, "amount"); amount != "" {
			balance["amount"] = amount
		}
		if currency := extractEnableBankingString(balanceAmount, "currency"); currency != "" {
			balance["currency"] = currency
		}
		balances = append(balances, balance)
	}
	if len(balances) == 0 {
		return nil
	}
	return balances
}

func normalizeEnableBankingRequestedStatus(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return enableBankingStatusBoth
	}
	return trimmed
}

func resolveEnableBankingTransactionStatus(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", enableBankingStatusBoth:
		return "", nil
	case enableBankingStatusBooked:
		return enableBankingStatusBooked, nil
	case enableBankingStatusPending:
		return enableBankingStatusPending, nil
	default:
		return "", errors.New("enable-banking status must be booked, pending, or both")
	}
}

func extractEnableBankingTransactionItems(raw map[string]any) []map[string]any {
	items, ok := raw["transactions"].([]any)
	if !ok {
		return nil
	}
	transactions := make([]map[string]any, 0, len(items))
	for _, item := range items {
		transaction, okCast := item.(map[string]any)
		if okCast {
			transactions = append(transactions, transaction)
		}
	}
	return transactions
}

func normalizeEnableBankingTransaction(raw map[string]any) enableBankingTransactionEntry {
	amountRaw, _ := raw["amount"].(map[string]any)
	return enableBankingTransactionEntry{
		TransactionID: extractEnableBankingString(
			raw,
			"transaction_id",
			"id",
		),
		Status:      strings.ToLower(extractEnableBankingString(raw, "status")),
		BookingDate: extractEnableBankingString(raw, "booking_date", "bookingDate"),
		ValueDate:   extractEnableBankingString(raw, "value_date", "valueDate"),
		Amount:      extractEnableBankingString(amountRaw, "amount"),
		Currency:    extractEnableBankingString(amountRaw, "currency"),
		CreditDebitIndicator: extractEnableBankingString(
			raw,
			"credit_debit_indicator",
			"creditDebitIndicator",
		),
		RemittanceInformation: extractEnableBankingString(
			raw,
			"remittance_information_unstructured",
			"remittanceInformationUnstructured",
		),
		Raw: raw,
	}
}
