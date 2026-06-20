package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonobankTransactionsCommand(t *testing.T) {
	fake := faker.New()

	type monobankTransactionResult struct {
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

	type monobankTransactionsCommandResult struct {
		Provider         string                      `json:"provider"`
		Operation        string                      `json:"operation"`
		FetchedAt        string                      `json:"fetchedAt"`
		Account          string                      `json:"account"`
		From             string                      `json:"from"`
		To               string                      `json:"to"`
		TransactionCount int                         `json:"transactionCount"`
		Transactions     []monobankTransactionResult `json:"transactions"`
		Raw              map[string]any              `json:"raw"`
	}

	makeRootCmd := func(t *testing.T, deps financePOCCommandDeps) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
		t.Helper()
		rootCmd := newRootCmd()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.AddCommand(newFinancePOCCmd(deps))
		return rootCmd, stdout, stderr
	}

	t.Run("defaults account to zero, chunks requests, exports output, and preserves raw items", func(t *testing.T) {
		fetchedAt := time.Date(2026, time.June, 18, 19, 0, 0, 0, time.UTC)
		envToken := fake.Internet().Password() + "-env"
		t.Setenv("MONOBANK_TOKEN", envToken)

		fromDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
		toDate := time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC)
		inclusiveToDate := toDate.Add(24*time.Hour - time.Second)
		firstChunkTo := fromDate.Add(monobankStatementMaxChunkRange)
		secondChunkFrom := firstChunkTo.Add(time.Second)
		expectedPaths := []string{
			fmt.Sprintf("/personal/statement/0/%d/%d", fromDate.Unix(), firstChunkTo.Unix()),
			fmt.Sprintf("/personal/statement/0/%d/%d", secondChunkFrom.Unix(), inclusiveToDate.Unix()),
		}
		requestedPaths := make([]string, 0, len(expectedPaths))
		requestedTokens := make([]string, 0, len(expectedPaths))
		sleepCalls := make([]time.Duration, 0, 1)
		outputFilePath := filepath.Join(t.TempDir(), fake.Lorem().Word(), "transactions.json")

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestedPaths = append(requestedPaths, r.URL.Path)
			requestedTokens = append(requestedTokens, r.Header.Get("X-Token"))
			w.Header().Set("Content-Type", "application/json")

			switch r.URL.Path {
			case expectedPaths[0]:
				_, _ = fmt.Fprintf(
					w,
					`[{"id":"txn-1","time":%d,"description":"salary","mcc":4829,"amount":1100,"operationAmount":1100,"currencyCode":980,"commissionRate":0,"cashbackAmount":0,"balance":2100,"comment":"month-1","receiptId":"receipt-1","counterEdrpou":"12345678","counterIban":"UA111111111111111111111111111","hold":false}]`,
					fromDate.Add(2*time.Hour).Unix(),
				)
			case expectedPaths[1]:
				_, _ = fmt.Fprintf(
					w,
					`[{"id":"txn-2","time":%d,"description":"coffee","mcc":5814,"amount":-250,"operationAmount":-250,"currencyCode":980,"commissionRate":0,"cashbackAmount":5,"balance":1850,"comment":"month-2","receiptId":"receipt-2","counterIban":"UA222222222222222222222222222","hold":true}]`,
					secondChunkFrom.Add(2*time.Hour).Unix(),
				)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		rootCmd, stdout, stderr := makeRootCmd(t, financePOCCommandDeps{
			Now: func() time.Time { return fetchedAt },
			Sleep: func(duration time.Duration) {
				sleepCalls = append(sleepCalls, duration)
			},
		})
		rootCmd.SetArgs([]string{
			"finance-poc", "monobank", "transactions", "--json",
			"--base-url", server.URL,
			"--from", fromDate.Format(financePOCDateLayout),
			"--to", toDate.Format(financePOCDateLayout),
			"--out", outputFilePath,
		})
		require.NoError(t, rootCmd.ExecuteContext(t.Context()))

		var got monobankTransactionsCommandResult
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
		assert.Equal(t, monobankCommandName, got.Provider)
		assert.Equal(t, monobankTransactionsOp, got.Operation)
		assert.Equal(t, fetchedAt.Format(time.RFC3339), got.FetchedAt)
		assert.Equal(t, "0", got.Account)
		assert.Equal(t, fromDate.Format(financePOCDateLayout), got.From)
		assert.Equal(t, toDate.Format(financePOCDateLayout), got.To)
		assert.Equal(t, expectedPaths, requestedPaths)
		assert.Equal(t, []string{envToken, envToken}, requestedTokens)
		assert.Equal(t, []time.Duration{monobankDefaultSleepBetweenRequests}, sleepCalls)
		require.Len(t, got.Transactions, 2)
		assert.Equal(t, "txn-1", got.Transactions[0].ID)
		assert.Equal(t, "salary", got.Transactions[0].Description)
		assert.Equal(t, int64(1100), got.Transactions[0].Amount)
		assert.False(t, got.Transactions[0].Hold)
		assert.Equal(t, "txn-2", got.Transactions[1].ID)
		assert.True(t, got.Transactions[1].Hold)
		require.Len(t, got.Raw["statement"].([]any), 2)
		assert.Contains(t, stderr.String(), "retrieved 2 monobank transactions")
		assert.NotContains(t, stdout.String(), envToken)
		assert.NotContains(t, stderr.String(), envToken)

		written, err := os.ReadFile(outputFilePath)
		require.NoError(t, err)
		assert.JSONEq(t, stdout.String(), string(written))
	})

	t.Run("sleep override flag is used between chunks", func(t *testing.T) {
		flagToken := fake.Internet().Password() + "-flag"
		fromDate := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
		toDate := time.Date(2026, time.April, 15, 0, 0, 0, 0, time.UTC)
		inclusiveToDate := toDate.Add(24*time.Hour - time.Second)
		firstChunkTo := fromDate.Add(monobankStatementMaxChunkRange)
		secondChunkFrom := firstChunkTo.Add(time.Second)
		requestedPaths := make([]string, 0, 2)
		sleepCalls := make([]time.Duration, 0, 1)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestedPaths = append(requestedPaths, r.URL.Path)
			assert.Equal(t, flagToken, r.Header.Get("X-Token"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `[]`)
		}))
		defer server.Close()

		rootCmd, _, _ := makeRootCmd(t, financePOCCommandDeps{
			Sleep: func(duration time.Duration) {
				sleepCalls = append(sleepCalls, duration)
			},
		})
		rootCmd.SetArgs([]string{
			"finance-poc", "monobank", "transactions", "--json",
			"--base-url", server.URL,
			"--token", flagToken,
			"--from", fromDate.Format(financePOCDateLayout),
			"--to", toDate.Format(financePOCDateLayout),
			"--sleep-between-requests", "1ms",
		})
		require.NoError(t, rootCmd.ExecuteContext(t.Context()))

		assert.Equal(t, []time.Duration{time.Millisecond}, sleepCalls)
		assert.Equal(t, []string{
			fmt.Sprintf("/personal/statement/0/%d/%d", fromDate.Unix(), firstChunkTo.Unix()),
			fmt.Sprintf("/personal/statement/0/%d/%d", secondChunkFrom.Unix(), inclusiveToDate.Unix()),
		}, requestedPaths)
	})

	t.Run("timeout cancels inter-chunk sleep before next request", func(t *testing.T) {
		fromDate := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
		toDate := time.Date(2026, time.June, 15, 0, 0, 0, 0, time.UTC)
		firstChunkTo := fromDate.Add(monobankStatementMaxChunkRange)
		secondChunkFrom := firstChunkTo.Add(time.Second)
		requestedPaths := make([]string, 0, 2)
		flagToken := fake.Internet().Password() + "-timeout"

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestedPaths = append(requestedPaths, r.URL.Path)
			assert.Equal(t, flagToken, r.Header.Get("X-Token"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `[]`)
		}))
		defer server.Close()

		rootCmd, _, _ := makeRootCmd(t, financePOCCommandDeps{})
		rootCmd.SilenceErrors = true
		rootCmd.SilenceUsage = true
		rootCmd.SetArgs([]string{
			"finance-poc", "monobank", "transactions", "--json",
			"--base-url", server.URL,
			"--token", flagToken,
			"--from", fromDate.Format(financePOCDateLayout),
			"--to", toDate.Format(financePOCDateLayout),
			"--timeout", "10ms",
			"--sleep-between-requests", "100ms",
		})

		start := time.Now()
		err := rootCmd.ExecuteContext(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, err, "sleep between monobank chunks")
		require.ErrorContains(t, err, context.DeadlineExceeded.Error())
		assert.Less(t, time.Since(start), 100*time.Millisecond)
		assert.Equal(t, []string{
			fmt.Sprintf("/personal/statement/0/%d/%d", fromDate.Unix(), firstChunkTo.Unix()),
		}, requestedPaths)
		assert.NotContains(
			t,
			requestedPaths,
			fmt.Sprintf(
				"/personal/statement/0/%d/%d",
				secondChunkFrom.Unix(),
				toDate.Add(24*time.Hour-time.Second).Unix(),
			),
		)
	})

	t.Run("provider errors stay redacted and helpers cover validation and fallbacks", func(t *testing.T) {
		secretToken := fake.Internet().Password() + "-token"
		fromDate := time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC)
		inclusiveToDate := time.Date(2026, time.January, 1, 23, 59, 59, 0, time.UTC)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(
				t,
				fmt.Sprintf("/personal/statement/acc-main/%d/%d", fromDate.Unix(), inclusiveToDate.Unix()),
				r.URL.Path,
			)
			assert.Equal(t, secretToken, r.Header.Get("X-Token"))
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprintf(w, `{"token":%q,"secret":%q}`, secretToken, "secret-"+fake.Internet().Password())
		}))
		defer server.Close()

		rootCmd, stdout, stderr := makeRootCmd(t, financePOCCommandDeps{})
		rootCmd.SilenceErrors = true
		rootCmd.SilenceUsage = true
		rootCmd.SetArgs([]string{
			"finance-poc", "monobank", "transactions", "--json",
			"--base-url", server.URL,
			"--token", secretToken,
			"--account", "acc-main",
			"--from", "2025-12-31",
			"--to", "2026-01-01",
			"--sleep-between-requests", "0s",
		})

		err := rootCmd.ExecuteContext(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, err, "status 401")
		assert.NotContains(t, err.Error(), secretToken)
		assert.NotContains(t, stdout.String(), secretToken)
		assert.NotContains(t, stderr.String(), secretToken)
		assert.Contains(t, err.Error(), `"token":"[REDACTED]"`)

		_, err = runMonobankTransactions(
			t.Context(),
			nil,
			financePOCCommandDeps{},
			financePOCProviderRequest{},
			monobankTransactionsParams{},
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "monobank token is required")

		_, err = callMonobankStatement(
			t.Context(),
			financePOCProviderRequest{
				BaseURL: "http://[::1",
				Token:   "token-" + fake.Lorem().Word(),
			},
			monobankStatementChunk{},
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "parse monobank statement URL")

		_, err = callMonobankStatement(
			t.Context(),
			financePOCProviderRequest{
				BaseURL: "http://127.0.0.1:1",
				Token:   "token-" + fake.Lorem().Word(),
			},
			monobankStatementChunk{Account: "0", FromUnix: 1, ToUnix: 2},
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "request monobank statement")

		badJSONServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `[{"id":`)
		}))
		defer badJSONServer.Close()

		_, err = callMonobankStatement(
			t.Context(),
			financePOCProviderRequest{
				BaseURL: badJSONServer.URL,
				Token:   "token-" + fake.Lorem().Word(),
			},
			monobankStatementChunk{Account: "0", FromUnix: 1, ToUnix: 2},
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "decode monobank statement response")

		_, _, _, err = validateMonobankTransactionsParams(monobankTransactionsParams{
			From: "2026-06-18",
			To:   "2026-06-17",
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "monobank --to must be on or after --from")

		assert.Nil(t, extractMonobankStatementItems(map[string]any{}))
		assert.Nil(t, extractMonobankStatementItems(map[string]any{"statement": "bad"}))
		assert.Empty(t, extractMonobankStatementItems(map[string]any{"statement": []any{"bad"}}))
		normalized := normalizeMonobankTransactions([]map[string]any{{
			"id":              "txn-" + fake.Lorem().Word(),
			"time":            json.Number("1710000000"),
			"description":     "desc-" + fake.Lorem().Word(),
			"mcc":             json.Number("5814"),
			"amount":          int32(-50),
			"operationAmount": int64(-50),
			"currencyCode":    float64(980),
			"commissionRate":  int(0),
			"cashbackAmount":  float32(3),
			"balance":         int64(1000),
			"comment":         "comment-" + fake.Lorem().Word(),
			"receiptId":       "receipt-" + fake.Lorem().Word(),
			"counterEdrpou":   "12345678",
			"counterIban":     "UA333333333333333333333333333",
			"hold":            true,
		}})
		require.Len(t, normalized, 1)
		assert.Equal(t, int64(1710000000), normalized[0].Time)
		assert.Equal(t, 5814, normalized[0].MCC)
		assert.Equal(t, int64(-50), normalized[0].Amount)
		assert.Equal(t, int64(3), normalized[0].CashbackAmount)
		assert.True(t, normalized[0].Hold)
		assert.False(t, extractMonobankBool(map[string]any{"hold": "bad"}, "hold"))
	})
}
