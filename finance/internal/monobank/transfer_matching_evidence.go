package monobank

import (
	"encoding/json"
	"math"
)

const monobankTransferMatchingMCC = 4829

// TransferMatchingEvidenceInput is the compact current-ledger and provider
// projection needed to validate one Monobank transaction snapshot.
type TransferMatchingEvidenceInput struct {
	ConnectorID                 string
	CurrentAmountMinor          int64
	CurrentCurrency             string
	ProviderOriginalAmountMinor *int64
	ProviderOriginalCurrency    *string
	SnapshotJSON                string
}

// TransferMatchingEvidence is the validated operation side of a Monobank
// cross-currency transaction snapshot.
type TransferMatchingEvidence struct {
	AmountMinor          int64
	OperationAmountMinor int64
	OperationCurrency    string
}

type transferMatchingSnapshot struct {
	Amount          *int64 `json:"amount"`
	OperationAmount *int64 `json:"operationAmount"`
	CurrencyCode    *int   `json:"currencyCode"`
	MCC             *int   `json:"mcc"`
	OriginalMCC     *int   `json:"originalMcc"`
}

// ExtractTransferMatchingEvidence returns usable evidence only for an
// unedited Monobank transfer snapshot with the narrow FX transfer signature.
func ExtractTransferMatchingEvidence(
	input TransferMatchingEvidenceInput,
) (TransferMatchingEvidence, bool) {
	if input.ConnectorID != "monobank" ||
		input.ProviderOriginalAmountMinor == nil ||
		input.ProviderOriginalCurrency == nil ||
		input.SnapshotJSON == "" {
		return TransferMatchingEvidence{}, false
	}
	var snapshot transferMatchingSnapshot
	if err := json.Unmarshal([]byte(input.SnapshotJSON), &snapshot); err != nil ||
		snapshot.Amount == nil ||
		snapshot.OperationAmount == nil ||
		snapshot.CurrencyCode == nil ||
		snapshot.MCC == nil ||
		*snapshot.Amount == 0 ||
		*snapshot.OperationAmount == 0 ||
		*snapshot.Amount == math.MinInt64 ||
		*snapshot.OperationAmount == math.MinInt64 ||
		*snapshot.MCC != monobankTransferMatchingMCC ||
		(snapshot.OriginalMCC != nil && *snapshot.OriginalMCC != monobankTransferMatchingMCC) {
		return TransferMatchingEvidence{}, false
	}
	operationCurrency := currencyCodeToISO(*snapshot.CurrencyCode)
	if operationCurrency == "" || operationCurrency == input.CurrentCurrency ||
		*input.ProviderOriginalAmountMinor != *snapshot.Amount ||
		*input.ProviderOriginalCurrency != input.CurrentCurrency ||
		input.CurrentAmountMinor != *input.ProviderOriginalAmountMinor ||
		(input.CurrentAmountMinor < 0) != (*snapshot.OperationAmount < 0) {
		return TransferMatchingEvidence{}, false
	}
	return TransferMatchingEvidence{
		AmountMinor: *snapshot.Amount, OperationAmountMinor: *snapshot.OperationAmount,
		OperationCurrency: operationCurrency,
	}, true
}
