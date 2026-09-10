package domain

import "time"

type CashFlowGroupBy string

const (
	CashFlowGroupByDay   CashFlowGroupBy = "day"
	CashFlowGroupByMonth CashFlowGroupBy = "month"
)

type CashFlowPeriod struct {
	StartDate time.Time
	EndDate   time.Time
}

type CashFlowSeriesBucket struct {
	StartDate    time.Time
	EndDate      time.Time
	IncomeMinor  int64
	ExpenseMinor int64
}

type CashFlowMissingFXDiagnostic struct {
	Provider                 string
	BaseCurrency             string
	QuoteCurrency            string
	AffectedTransactionCount int
}

type CashFlowSeries struct {
	Period          CashFlowPeriod
	GroupBy         CashFlowGroupBy
	DisplayCurrency string
	Complete        bool
	MissingFX       []CashFlowMissingFXDiagnostic
	Buckets         []CashFlowSeriesBucket
}
