package monobank

import (
	"fmt"
	"math"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
)

func TestExtractTransferMatchingEvidence(t *testing.T) {
	fake := faker.New()
	stringPointer := func(value string) *string { return &value }
	int64Pointer := func(value int64) *int64 { return &value }
	makeInput := func(amount int64, currency string, snapshot string) TransferMatchingEvidenceInput {
		return TransferMatchingEvidenceInput{
			ConnectorID:                 "monobank",
			CurrentAmountMinor:          amount,
			CurrentCurrency:             currency,
			ProviderOriginalAmountMinor: int64Pointer(amount),
			ProviderOriginalCurrency:    stringPointer(currency),
			SnapshotJSON:                snapshot,
		}
	}
	makeSnapshot := func(amount int64, operationAmount int64, currencyCode int, mcc int, originalMCC int) string {
		return fmt.Sprintf(
			`{"amount":%d,"operationAmount":%d,"currencyCode":%d,"mcc":%d,"originalMcc":%d}`,
			amount,
			operationAmount,
			currencyCode,
			mcc,
			originalMCC,
		)
	}

	t.Run("returns only complete unedited Monobank FX evidence", func(t *testing.T) {
		credit := makeInput(508_300, "UAH", makeSnapshot(508_300, 10_000, 978, 4829, 4829))
		debit := makeInput(-10_000, "EUR", `{"amount":-10000,"operationAmount":-508300,"currencyCode":980,"mcc":4829}`)

		creditEvidence, creditOK := ExtractTransferMatchingEvidence(credit)
		debitEvidence, debitOK := ExtractTransferMatchingEvidence(debit)

		assert.True(t, creditOK)
		assert.Equal(t, TransferMatchingEvidence{
			AmountMinor: 508_300, OperationAmountMinor: 10_000, OperationCurrency: "EUR",
		}, creditEvidence)
		assert.True(t, debitOK)
		assert.Equal(t, TransferMatchingEvidence{
			AmountMinor: -10_000, OperationAmountMinor: -508_300, OperationCurrency: "UAH",
		}, debitEvidence)
	})

	t.Run("fails closed for unusable provider evidence", func(t *testing.T) {
		valid := makeInput(508_300, "UAH", makeSnapshot(508_300, 10_000, 978, 4829, 4829))
		cases := []struct {
			name  string
			input TransferMatchingEvidenceInput
		}{
			{name: "unknown connector", input: func() TransferMatchingEvidenceInput {
				item := valid
				item.ConnectorID = "connector-" + fake.UUID().V4()
				return item
			}()},
			{name: "unknown currency", input: func() TransferMatchingEvidenceInput {
				item := valid
				item.SnapshotJSON = makeSnapshot(508_300, 10_000, 999, 4829, 4829)
				return item
			}()},
			{
				name:  "missing snapshot",
				input: func() TransferMatchingEvidenceInput { item := valid; item.SnapshotJSON = ""; return item }(),
			},
			{
				name:  "malformed snapshot",
				input: func() TransferMatchingEvidenceInput { item := valid; item.SnapshotJSON = "{"; return item }(),
			},
			{name: "zero snapshot amount", input: makeInput(0, "UAH", makeSnapshot(0, 10_000, 978, 4829, 4829))},
			{
				name:  "zero operation amount",
				input: makeInput(508_300, "UAH", makeSnapshot(508_300, 0, 978, 4829, 4829)),
			},
			{name: "wrong mcc", input: makeInput(508_300, "UAH", makeSnapshot(508_300, 10_000, 978, 5411, 4829))},
			{
				name:  "wrong original mcc",
				input: makeInput(508_300, "UAH", makeSnapshot(508_300, 10_000, 978, 4829, 5411)),
			},
			{name: "same currency", input: makeInput(508_300, "EUR", makeSnapshot(508_300, 10_000, 978, 4829, 4829))},
			{name: "provider amount edit", input: func() TransferMatchingEvidenceInput {
				item := valid
				item.ProviderOriginalAmountMinor = int64Pointer(1)
				return item
			}()},
			{name: "provider currency edit", input: func() TransferMatchingEvidenceInput {
				item := valid
				item.ProviderOriginalCurrency = stringPointer("EUR")
				return item
			}()},
			{
				name:  "current amount edit",
				input: func() TransferMatchingEvidenceInput { item := valid; item.CurrentAmountMinor++; return item }(),
			},
			{
				name:  "current currency edit",
				input: func() TransferMatchingEvidenceInput { item := valid; item.CurrentCurrency = "EUR"; return item }(),
			},
			{
				name:  "different signs",
				input: makeInput(508_300, "UAH", makeSnapshot(508_300, -10_000, 978, 4829, 4829)),
			},
			{
				name:  "amount negation boundary",
				input: makeInput(math.MinInt64, "UAH", makeSnapshot(math.MinInt64, -1, 978, 4829, 4829)),
			},
			{
				name:  "operation negation boundary",
				input: makeInput(-1, "UAH", makeSnapshot(-1, math.MinInt64, 978, 4829, 4829)),
			},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				_, ok := ExtractTransferMatchingEvidence(testCase.input)
				assert.False(t, ok)
			})
		}
	})
}
