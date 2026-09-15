package pko

import (
	"fmt"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
)

func TestExtractTransferMatchingEvidence(t *testing.T) {
	fake := faker.New()
	stringPointer := func(value string) *string { return &value }
	int64Pointer := func(value int64) *int64 { return &value }
	makeInput := func(amount int64, currency string, trackedIBAN string, snapshot string) TransferMatchingEvidenceInput {
		return TransferMatchingEvidenceInput{
			ProviderID: providerID, ConnectorID: connectorID, TrackedAccountIBAN: trackedIBAN,
			CurrentAmountMinor: amount, CurrentCurrency: currency,
			ProviderOriginalAmountMinor: int64Pointer(amount), ProviderOriginalCurrency: stringPointer(currency),
			SnapshotJSON: snapshot,
		}
	}
	debitSnapshot := `{"transaction_amount":{"amount":"600.00","currency":"usd"},"credit_debit_indicator":"DBIT","remittance_information":["EXCHANGE","TRANSFER"],"debtor_account":{"iban":"PL46"},"creditor_account":{"iban":"PL04"}}`
	creditSnapshot := `{"transaction_amount":{"amount":"2054.88","currency":"pln"},"credit_debit_indicator":"CRDT","remittance_information":["EXCHANGE","TRANSFER-IN"],"debtor_account":{"iban":"PL46"}}`

	t.Run("returns the validated PKO exchange route", func(t *testing.T) {
		debit, debitOK := ExtractTransferMatchingEvidence(makeInput(-60_000, "USD", "PL46", debitSnapshot))
		credit, creditOK := ExtractTransferMatchingEvidence(makeInput(205_488, "PLN", "PL04", creditSnapshot))

		assert.True(t, debitOK)
		assert.Equal(t, TransferMatchingEvidence{
			Direction:       directionDebit,
			SourceIBAN:      "PL46",
			DestinationIBAN: "PL04",
			AmountMinor:     -60_000,
			Currency:        "USD",
		}, debit)
		assert.True(t, creditOK)
		assert.Equal(t, TransferMatchingEvidence{
			Direction:       directionCredit,
			SourceIBAN:      "PL46",
			DestinationIBAN: "PL04",
			AmountMinor:     205_488,
			Currency:        "PLN",
		}, credit)
	})

	t.Run("fails closed for incomplete or edited evidence", func(t *testing.T) {
		valid := makeInput(-60_000, "USD", "PL46", debitSnapshot)
		cases := []struct {
			name  string
			input TransferMatchingEvidenceInput
		}{
			{name: "provider", input: func() TransferMatchingEvidenceInput {
				item := valid
				item.ProviderID = "provider-" + fake.UUID().V4()
				return item
			}()},
			{name: "connector", input: func() TransferMatchingEvidenceInput {
				item := valid
				item.ConnectorID = "connector-" + fake.UUID().V4()
				return item
			}()},
			{name: "missing tracked iban", input: func() TransferMatchingEvidenceInput {
				item := valid
				item.TrackedAccountIBAN = ""
				return item
			}()},
			{name: "malformed snapshot", input: func() TransferMatchingEvidenceInput {
				item := valid
				item.SnapshotJSON = "{"
				return item
			}()},
			{name: "bad markers", input: func() TransferMatchingEvidenceInput {
				item := valid
				item.SnapshotJSON = fmt.Sprintf(
					`{"transaction_amount":{"amount":"600.00","currency":"USD"},"credit_debit_indicator":"DBIT","remittance_information":["EXCHANGE","%s"],"debtor_account":{"iban":"PL46"},"creditor_account":{"iban":"PL04"}}`,
					fake.Lorem().Word(),
				)
				return item
			}()},
			{name: "missing creditor iban", input: func() TransferMatchingEvidenceInput {
				item := valid
				item.SnapshotJSON = `{"transaction_amount":{"amount":"600.00","currency":"USD"},"credit_debit_indicator":"DBIT","remittance_information":["EXCHANGE","TRANSFER"],"debtor_account":{"iban":"PL46"}}`
				return item
			}()},
			{name: "inconsistent tracked iban", input: func() TransferMatchingEvidenceInput {
				item := valid
				item.TrackedAccountIBAN = "PL04"
				return item
			}()},
			{name: "amount edit", input: func() TransferMatchingEvidenceInput {
				item := valid
				item.CurrentAmountMinor++
				return item
			}()},
			{name: "currency edit", input: func() TransferMatchingEvidenceInput {
				item := valid
				item.CurrentCurrency = "PLN"
				return item
			}()},
			{name: "invalid snapshot currency", input: func() TransferMatchingEvidenceInput {
				item := valid
				item.SnapshotJSON = `{"transaction_amount":{"amount":"600.00","currency":"US1"},"credit_debit_indicator":"DBIT","remittance_information":["EXCHANGE","TRANSFER"],"debtor_account":{"iban":"PL46"},"creditor_account":{"iban":"PL04"}}`
				return item
			}()},
			{name: "unicode snapshot currency lookalike", input: func() TransferMatchingEvidenceInput {
				item := valid
				item.SnapshotJSON = `{"transaction_amount":{"amount":"600.00","currency":"uſd"},"credit_debit_indicator":"DBIT","remittance_information":["EXCHANGE","TRANSFER"],"debtor_account":{"iban":"PL46"},"creditor_account":{"iban":"PL04"}}`
				return item
			}()},
			{name: "provider original edit", input: func() TransferMatchingEvidenceInput {
				item := valid
				item.ProviderOriginalAmountMinor = int64Pointer(-1)
				return item
			}()},
			{name: "invalid amount", input: func() TransferMatchingEvidenceInput {
				item := valid
				item.SnapshotJSON = `{"transaction_amount":{"amount":"600.001","currency":"USD"},"credit_debit_indicator":"DBIT","remittance_information":["EXCHANGE","TRANSFER"],"debtor_account":{"iban":"PL46"},"creditor_account":{"iban":"PL04"}}`
				return item
			}()},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				_, ok := ExtractTransferMatchingEvidence(testCase.input)
				assert.False(t, ok)
			})
		}
	})

	t.Run("parses only representable two-decimal snapshot amounts", func(t *testing.T) {
		for _, testCase := range []struct {
			value string
			want  int64
			ok    bool
		}{
			{value: "600", want: 60_000, ok: true},
			{value: "+600.1", want: 60_010, ok: true},
			{value: "-600.01", want: -60_001, ok: true},
			{value: "", ok: false},
			{value: "+", ok: false},
			{value: "600.001", ok: false},
			{value: "600.0.1", ok: false},
			{value: "invalid", ok: false},
			{value: "92233720368547758.08", ok: false},
		} {
			t.Run(testCase.value, func(t *testing.T) {
				actual, ok := parseAmountMinor(testCase.value)
				assert.Equal(t, testCase.ok, ok)
				assert.Equal(t, testCase.want, actual)
			})
		}
	})
}
