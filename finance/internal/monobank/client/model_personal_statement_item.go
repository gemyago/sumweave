package client

// PersonalStatementItem models a Monobank statement entry.
type PersonalStatementItem struct {
	ID              string `json:"id"`
	Time            int64  `json:"time"`
	Description     string `json:"description"`
	MCC             int    `json:"mcc,omitempty"`
	OriginalMCC     int    `json:"originalMcc,omitempty"`
	Hold            bool   `json:"hold,omitempty"`
	Amount          int64  `json:"amount"`
	OperationAmount int64  `json:"operationAmount,omitempty"`
	CurrencyCode    int    `json:"currencyCode,omitempty"`
	CommissionRate  int64  `json:"commissionRate,omitempty"`
	CashbackAmount  int64  `json:"cashbackAmount,omitempty"`
	Balance         int64  `json:"balance,omitempty"`
	Comment         string `json:"comment,omitempty"`
	ReceiptID       string `json:"receiptId,omitempty"`
	CounterEDRPOU   string `json:"counterEdrpou,omitempty"`
}
