package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GetAccountTransactionsParams contains transaction lookup parameters.
type GetAccountTransactionsParams struct {
	AccountID       string
	DateFrom        time.Time
	DateTo          time.Time
	Strategy        string
	Status          string
	ContinuationKey string
}

// GetAccountTransactions gets transactions for an account.
func (c *Client) GetAccountTransactions(
	ctx context.Context,
	params GetAccountTransactionsParams,
) (*GetAccountTransactionsResponse, error) {
	query := url.Values{}
	if !params.DateFrom.IsZero() {
		query.Set("date_from", params.DateFrom.UTC().Format(time.DateOnly))
	}
	if !params.DateTo.IsZero() {
		query.Set("date_to", params.DateTo.UTC().Format(time.DateOnly))
	}
	if strings.TrimSpace(params.Strategy) != "" {
		query.Set("strategy", strings.TrimSpace(params.Strategy))
	}
	if strings.TrimSpace(params.Status) != "" {
		query.Set("status", strings.TrimSpace(params.Status))
	}
	if strings.TrimSpace(params.ContinuationKey) != "" {
		query.Set("continuation_key", strings.TrimSpace(params.ContinuationKey))
	}
	path := "/accounts/" + url.PathEscape(params.AccountID) + "/transactions"
	raw, err := c.DoRawObject(
		ctx,
		DoRawJSONParams{Method: http.MethodGet, Path: path, Query: query},
	)
	if err != nil {
		return nil, fmt.Errorf("get account transactions failed: %w", err)
	}
	items := objectSlice(raw, "transactions")
	transactions := make([]AccountTransaction, 0, len(items))
	for _, item := range items {
		amount := amountObject(item)
		transactions = append(transactions, AccountTransaction{
			TransactionID: firstNonEmpty(
				stringValue(item, "transactionId", "transaction_id"),
				stringValue(item, "id"),
			),
			ID:          stringValue(item, "id"),
			Status:      strings.ToLower(stringValue(item, "status")),
			BookingDate: stringValue(item, "booking_date", "bookingDate"),
			ValueDate:   stringValue(item, "value_date", "valueDate"),
			Currency: strings.ToUpper(
				firstNonEmpty(
					stringValue(item, "currency"),
					stringValue(amount, "currency"),
				),
			),
			AmountMinor: firstNonZeroInt64(
				int64Value(item, "amountMinor"),
				signedAmountMinor(item),
			),
			Description: stringValue(item, "description"),
			EffectiveAt: stringValue(item, "effectiveAt"),
			CreditDebitIndicator: stringValue(
				item,
				"credit_debit_indicator",
				"creditDebitIndicator",
			),
			RemittanceInformationUnstructured: stringValue(
				item,
				"remittance_information_unstructured",
				"remittanceInformationUnstructured",
			),
			Amount: &TransactionAmount{
				Amount:   stringValue(amount, "amount"),
				Currency: strings.ToUpper(stringValue(amount, "currency")),
				Raw:      amount,
			},
			Raw: item,
		})
	}
	return &GetAccountTransactionsResponse{
		ContinuationKey: stringValue(raw, "continuation_key"),
		Transactions:    transactions,
		Raw:             raw,
	}, nil
}

func signedAmountMinor(raw map[string]any) int64 {
	amount := decimalToMinor(stringValue(amountObject(raw), "amount"))
	if amount > 0 && strings.EqualFold(stringValue(raw, "credit_debit_indicator", "creditDebitIndicator"), "DBIT") {
		return -amount
	}
	return amount
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
