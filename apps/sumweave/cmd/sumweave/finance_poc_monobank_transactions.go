package main

import (
	"context"
	"encoding/json"
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
	monobankDefaultAccount              = "0"
	monobankDefaultSleepBetweenRequests = 61 * time.Second
	monobankStatementMaxChunkRange      = 31*24*time.Hour + time.Hour
	monobankRawStatementKey             = "statement"
)

type monobankTransactionsParams struct {
	Account              string
	From                 string
	To                   string
	SleepBetweenRequests time.Duration
}

type monobankTransactionsOutput struct {
	Provider         string                     `json:"provider"`
	Operation        string                     `json:"operation"`
	FetchedAt        string                     `json:"fetchedAt"`
	Account          string                     `json:"account"`
	From             string                     `json:"from"`
	To               string                     `json:"to"`
	TransactionCount int                        `json:"transactionCount"`
	Transactions     []monobankTransactionEntry `json:"transactions"`
	Raw              map[string]any             `json:"raw"`
}

type monobankTransactionEntry struct {
	ID              string         `json:"id,omitempty"`
	Time            int64          `json:"time,omitempty"`
	Description     string         `json:"description,omitempty"`
	MCC             int            `json:"mcc,omitempty"`
	Amount          int64          `json:"amount,omitempty"`
	OperationAmount int64          `json:"operationAmount,omitempty"`
	CurrencyCode    int            `json:"currencyCode,omitempty"`
	CommissionRate  int64          `json:"commissionRate,omitempty"`
	CashbackAmount  int64          `json:"cashbackAmount,omitempty"`
	Balance         int64          `json:"balance,omitempty"`
	Comment         string         `json:"comment,omitempty"`
	ReceiptID       string         `json:"receiptId,omitempty"`
	CounterEdrpou   string         `json:"counterEdrpou,omitempty"`
	CounterIban     string         `json:"counterIban,omitempty"`
	Hold            bool           `json:"hold,omitempty"`
	Raw             map[string]any `json:"raw"`
}

type monobankStatementChunk struct {
	Account  string
	FromUnix int64
	ToUnix   int64
}

func newMonobankTransactionsCmd(deps financePOCCommandDeps, requestParams *monobankCommandParams) *cobra.Command {
	commandParams := monobankTransactionsParams{
		Account:              monobankDefaultAccount,
		SleepBetweenRequests: monobankDefaultSleepBetweenRequests,
	}
	cmd := &cobra.Command{
		Use:   monobankTransactionsOp,
		Short: "Fetch monobank statement transactions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			request := financePOCProviderRequest{
				Provider:  monobankCommandName,
				Operation: monobankTransactionsOp,
				BaseURL: firstNonEmpty(
					strings.TrimSpace(requestParams.BaseURL),
					strings.TrimSpace(cmd.Flag("base-url").Value.String()),
					financePOCEnv("MONOBANK_BASE_URL"),
					monobankDefaultBaseURL,
				),
				Timeout:    requestParams.Timeout,
				JSON:       requestParams.JSON,
				OutputFile: strings.TrimSpace(requestParams.OutputFile),
				Token: resolveFinancePOCStringSetting(
					strings.TrimSpace(requestParams.Token),
					financePOCEnv("MONOBANK_TOKEN"),
				),
				TokenSource: resolveFinancePOCStringSource(
					strings.TrimSpace(requestParams.Token),
					financePOCEnv("MONOBANK_TOKEN"),
				),
			}

			result, err := runMonobankTransactions(cmd.Context(), cmd.ErrOrStderr(), deps, request, commandParams)
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
	cmd.Flags().StringVar(&commandParams.Account, "account", commandParams.Account, "Account identifier")
	cmd.Flags().StringVar(&commandParams.From, "from", "", "Inclusive start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&commandParams.To, "to", "", "Inclusive end date (YYYY-MM-DD)")
	cmd.Flags().DurationVar(
		&commandParams.SleepBetweenRequests,
		"sleep-between-requests",
		commandParams.SleepBetweenRequests,
		"Sleep duration between chunked statement requests",
	)
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

func runMonobankTransactions(
	ctx context.Context,
	stderr io.Writer,
	deps financePOCCommandDeps,
	request financePOCProviderRequest,
	params monobankTransactionsParams,
) (monobankTransactionsOutput, error) {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if strings.TrimSpace(request.Token) == "" {
		return monobankTransactionsOutput{}, errors.New("monobank token is required")
	}

	ctx, cancel := withFinancePOCTimeout(ctx, request.Timeout)
	defer cancel()

	account, fromDate, toDate, err := validateMonobankTransactionsParams(params)
	if err != nil {
		return monobankTransactionsOutput{}, err
	}
	chunks := makeMonobankStatementChunks(account, fromDate, toDate)
	statementItems := make([]map[string]any, 0)
	transactions := make([]monobankTransactionEntry, 0)
	for index, chunk := range chunks {
		if index > 0 {
			sleepErr := sleepMonobankBetweenChunks(ctx, deps.Sleep, params.SleepBetweenRequests)
			if sleepErr != nil {
				return monobankTransactionsOutput{}, fmt.Errorf("sleep between monobank chunks: %w", sleepErr)
			}
		}

		items, statementErr := callMonobankStatement(ctx, request, chunk)
		if statementErr != nil {
			return monobankTransactionsOutput{}, statementErr
		}
		statementItems = append(statementItems, items...)
		transactions = append(transactions, normalizeMonobankTransactions(items)...)
	}

	writeFinancePOCProgressf(stderr, "retrieved %d monobank transactions", len(transactions))
	return monobankTransactionsOutput{
		Provider:         monobankCommandName,
		Operation:        monobankTransactionsOp,
		FetchedAt:        deps.Now().Format(time.RFC3339),
		Account:          account,
		From:             fromDate.Format(financePOCDateLayout),
		To:               toDate.Format(financePOCDateLayout),
		TransactionCount: len(transactions),
		Transactions:     transactions,
		Raw: map[string]any{
			monobankRawStatementKey: statementItems,
		},
	}, nil
}

func validateMonobankTransactionsParams(params monobankTransactionsParams) (string, time.Time, time.Time, error) {
	fromDate, err := parseFinancePOCDate("--from", params.From)
	if err != nil {
		return "", time.Time{}, time.Time{}, err
	}
	toDate, err := parseFinancePOCInclusiveEndDate("--to", params.To)
	if err != nil {
		return "", time.Time{}, time.Time{}, err
	}
	if toDate.Before(fromDate) {
		return "", time.Time{}, time.Time{}, errors.New("monobank --to must be on or after --from")
	}
	return firstNonEmpty(strings.TrimSpace(params.Account), monobankDefaultAccount), fromDate, toDate, nil
}

func sleepMonobankBetweenChunks(
	ctx context.Context,
	sleep func(time.Duration),
	duration time.Duration,
) error {
	if duration <= 0 {
		return nil
	}
	if sleep == nil {
		timer := time.NewTimer(duration)
		defer timer.Stop()
		select {
		case <-timer.C:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	done := make(chan struct{})
	go func() {
		sleep(duration)
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func makeMonobankStatementChunks(account string, fromDate time.Time, toDate time.Time) []monobankStatementChunk {
	chunks := make([]monobankStatementChunk, 0, 1)
	chunkFrom := fromDate
	for {
		chunkTo := chunkFrom.Add(monobankStatementMaxChunkRange)
		if chunkTo.After(toDate) {
			chunkTo = toDate
		}
		chunks = append(chunks, monobankStatementChunk{
			Account:  account,
			FromUnix: chunkFrom.Unix(),
			ToUnix:   chunkTo.Unix(),
		})
		if !chunkTo.Before(toDate) {
			break
		}
		chunkFrom = chunkTo.Add(time.Second)
		if chunkFrom.After(toDate) {
			break
		}
	}
	return chunks
}

func callMonobankStatement(
	ctx context.Context,
	request financePOCProviderRequest,
	chunk monobankStatementChunk,
) ([]map[string]any, error) {
	endpoint, err := url.Parse(
		fmt.Sprintf(
			"%s/personal/statement/%s/%d/%d",
			strings.TrimRight(request.BaseURL, "/"),
			url.PathEscape(chunk.Account),
			chunk.FromUnix,
			chunk.ToUnix,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("parse monobank statement URL: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build monobank statement request: %w", err)
	}
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("X-Token", request.Token)

	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("request monobank statement: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read monobank statement response: %w", err)
	}
	if response.StatusCode >= http.StatusBadRequest {
		return nil, newFinancePOCProviderResponseError(
			monobankCommandName,
			monobankTransactionsOp,
			response.StatusCode,
			body,
		)
	}

	var raw []map[string]any
	if unmarshalErr := json.Unmarshal(body, &raw); unmarshalErr != nil {
		return nil, fmt.Errorf("decode monobank statement response: %w", unmarshalErr)
	}
	return raw, nil
}

func normalizeMonobankTransactions(items []map[string]any) []monobankTransactionEntry {
	transactions := make([]monobankTransactionEntry, 0, len(items))
	for _, item := range items {
		transactions = append(transactions, monobankTransactionEntry{
			ID:              extractMonobankString(item, "id"),
			Time:            extractMonobankInt64(item, "time"),
			Description:     extractMonobankString(item, "description"),
			MCC:             extractMonobankInt(item, "mcc"),
			Amount:          extractMonobankInt64(item, "amount"),
			OperationAmount: extractMonobankInt64(item, "operationAmount"),
			CurrencyCode:    extractMonobankInt(item, "currencyCode"),
			CommissionRate:  extractMonobankInt64(item, "commissionRate"),
			CashbackAmount:  extractMonobankInt64(item, "cashbackAmount"),
			Balance:         extractMonobankInt64(item, "balance"),
			Comment:         extractMonobankString(item, "comment"),
			ReceiptID:       extractMonobankString(item, "receiptId"),
			CounterEdrpou:   extractMonobankString(item, "counterEdrpou"),
			CounterIban:     extractMonobankString(item, "counterIban"),
			Hold:            extractMonobankBool(item, "hold"),
			Raw:             item,
		})
	}
	return transactions
}

func extractMonobankStatementItems(raw map[string]any) []map[string]any {
	return extractMonobankObjects(raw, monobankRawStatementKey)
}

func extractMonobankBool(raw map[string]any, key string) bool {
	value, ok := raw[key]
	if !ok {
		return false
	}
	boolValue, ok := value.(bool)
	if !ok {
		return false
	}
	return boolValue
}
