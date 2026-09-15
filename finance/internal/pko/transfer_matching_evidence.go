package pko

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

const (
	providerID  = "pko"
	connectorID = "enable-banking"

	directionDebit  = "DBIT"
	directionCredit = "CRDT"
	decimalParts    = 2
	minorScale      = 100
	currencyCodeLen = 3
)

// TransferMatchingEvidenceInput is the compact current-ledger, provenance, and
// provider-snapshot projection needed to validate one PKO exchange leg.
type TransferMatchingEvidenceInput struct {
	ProviderID                  string
	ConnectorID                 string
	TrackedAccountIBAN          string
	CurrentAmountMinor          int64
	CurrentCurrency             string
	ProviderOriginalAmountMinor *int64
	ProviderOriginalCurrency    *string
	SnapshotJSON                string
}

// TransferMatchingEvidence is the validated internal route of one PKO exchange
// leg. SourceIBAN and DestinationIBAN are the route used for pairing.
type TransferMatchingEvidence struct {
	Direction       string
	SourceIBAN      string
	DestinationIBAN string
	AmountMinor     int64
	Currency        string
}

type transferMatchingSnapshot struct {
	TransactionAmount struct {
		Amount   *string `json:"amount"`
		Currency *string `json:"currency"`
	} `json:"transaction_amount"`
	CreditDebitIndicator  *string  `json:"credit_debit_indicator"`
	RemittanceInformation []string `json:"remittance_information"`
	DebtorAccount         *struct {
		IBAN *string `json:"iban"`
	} `json:"debtor_account"`
	CreditorAccount *struct {
		IBAN *string `json:"iban"`
	} `json:"creditor_account"`
}

// ExtractTransferMatchingEvidence returns usable evidence only for an unedited
// PKO Enable Banking snapshot with the narrow internal-exchange signature.
func ExtractTransferMatchingEvidence(
	input TransferMatchingEvidenceInput,
) (TransferMatchingEvidence, bool) {
	if !hasRequiredInput(input) {
		return TransferMatchingEvidence{}, false
	}
	snapshot, ok := decodeSnapshot(input.SnapshotJSON)
	if !ok {
		return TransferMatchingEvidence{}, false
	}
	amount, ok := snapshotAmountMinor(snapshot, input.CurrentCurrency)
	if !ok {
		return TransferMatchingEvidence{}, false
	}
	direction, sourceIBAN, destinationIBAN, amount, ok := exchangeRoute(input, snapshot, amount)
	if !ok {
		return TransferMatchingEvidence{}, false
	}
	if input.CurrentAmountMinor != amount ||
		*input.ProviderOriginalAmountMinor != amount ||
		*input.ProviderOriginalCurrency != input.CurrentCurrency {
		return TransferMatchingEvidence{}, false
	}
	return TransferMatchingEvidence{
		Direction: direction, SourceIBAN: sourceIBAN, DestinationIBAN: destinationIBAN,
		AmountMinor: amount, Currency: input.CurrentCurrency,
	}, true
}

func hasRequiredInput(input TransferMatchingEvidenceInput) bool {
	return input.ProviderID == providerID && input.ConnectorID == connectorID &&
		input.TrackedAccountIBAN != "" && input.ProviderOriginalAmountMinor != nil &&
		input.ProviderOriginalCurrency != nil && input.SnapshotJSON != ""
}

func decodeSnapshot(value string) (transferMatchingSnapshot, bool) {
	var snapshot transferMatchingSnapshot
	if err := json.Unmarshal([]byte(value), &snapshot); err != nil ||
		snapshot.TransactionAmount.Amount == nil || snapshot.TransactionAmount.Currency == nil ||
		snapshot.CreditDebitIndicator == nil || snapshot.DebtorAccount == nil ||
		snapshot.DebtorAccount.IBAN == nil || *snapshot.DebtorAccount.IBAN == "" {
		return transferMatchingSnapshot{}, false
	}
	return snapshot, true
}

func snapshotAmountMinor(snapshot transferMatchingSnapshot, currency string) (int64, bool) {
	amount, ok := parseAmountMinor(*snapshot.TransactionAmount.Amount)
	snapshotCurrency, hasCurrency := canonicalProviderCurrency(*snapshot.TransactionAmount.Currency)
	if !ok || !hasCurrency || amount == 0 || amount == math.MinInt64 || snapshotCurrency != currency {
		return 0, false
	}
	return amount, true
}

func canonicalProviderCurrency(value string) (string, bool) {
	if len(value) != currencyCodeLen {
		return "", false
	}
	for _, character := range value {
		if (character < 'A' || character > 'Z') && (character < 'a' || character > 'z') {
			return "", false
		}
	}
	return strings.ToUpper(value), true
}

func exchangeRoute(
	input TransferMatchingEvidenceInput,
	snapshot transferMatchingSnapshot,
	amount int64,
) (string, string, string, int64, bool) {
	direction := *snapshot.CreditDebitIndicator
	sourceIBAN := *snapshot.DebtorAccount.IBAN
	switch direction {
	case directionDebit:
		if amount > 0 {
			amount = -amount
		}
		if !equalStrings(snapshot.RemittanceInformation, []string{"EXCHANGE", "TRANSFER"}) ||
			snapshot.CreditorAccount == nil || snapshot.CreditorAccount.IBAN == nil ||
			*snapshot.CreditorAccount.IBAN == "" || input.TrackedAccountIBAN != sourceIBAN {
			return "", "", "", 0, false
		}
		return direction, sourceIBAN, *snapshot.CreditorAccount.IBAN, amount, true
	case directionCredit:
		if !equalStrings(snapshot.RemittanceInformation, []string{"EXCHANGE", "TRANSFER-IN"}) || amount < 0 {
			return "", "", "", 0, false
		}
		return direction, sourceIBAN, input.TrackedAccountIBAN, amount, true
	}
	return "", "", "", 0, false
}

func equalStrings(first []string, second []string) bool {
	if len(first) != len(second) {
		return false
	}
	for index := range first {
		if first[index] != second[index] {
			return false
		}
	}
	return true
}

func parseAmountMinor(value string) (int64, bool) {
	if value == "" {
		return 0, false
	}
	sign := int64(1)
	switch value[0] {
	case '-':
		sign = -1
		value = value[1:]
	case '+':
		value = value[1:]
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, false
	}
	fraction := ""
	if len(parts) == decimalParts {
		fraction = parts[1]
	}
	if len(fraction) > decimalParts {
		return 0, false
	}
	for len(fraction) < 2 {
		fraction += "0"
	}
	whole, wholeErr := strconv.ParseInt(parts[0], 10, 64)
	minor, minorErr := strconv.ParseInt(fraction, 10, 64)
	if wholeErr != nil || minorErr != nil || whole > (math.MaxInt64-minor)/minorScale {
		return 0, false
	}
	result := whole*minorScale + minor
	if sign < 0 {
		result = -result
	}
	return result, true
}
