package models

import "time"

type FinanceFxProviderDiagnostic = FinanceFxDiagnosticsResponseProvidersInner

type FinanceDashboardResponse struct {
	Period              FinanceDashboardPeriod               `json:"period"`
	Settled             FinanceDashboardMoneySummary         `json:"settled"`
	Pending             FinanceDashboardMoneySummary         `json:"pending"`
	CategoryBreakdowns  []*FinanceDashboardCategoryBreakdown `json:"categoryBreakdowns"`
	AccountBalances     []*FinanceDashboardAccountBalance    `json:"accountBalances"`
	Alerts              []*FinanceDashboardAlert             `json:"alerts"`
	MissingFx           []*FinanceDashboardMissingFx         `json:"missingFx"`
	NativeSettledTotals []*FinanceDashboardCurrencyTotal     `json:"nativeSettledTotals"`
}

type FinanceDashboardPeriod struct {
	Preset    string                       `json:"preset"`
	StartDate time.Time                    `json:"startDate"`
	EndDate   time.Time                    `json:"endDate"`
	Previous  FinanceDashboardPeriodWindow `json:"previous"`
	Next      FinanceDashboardPeriodWindow `json:"next"`
}

type FinanceDashboardPeriodWindow struct {
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`
}

type FinanceDashboardMoneySummary struct {
	DisplayCurrency  string `json:"displayCurrency"`
	IncomeMinor      int64  `json:"incomeMinor"`
	ExpenseMinor     int64  `json:"expenseMinor"`
	NetMinor         int64  `json:"netMinor"`
	TransactionCount int    `json:"transactionCount"`
	Complete         bool   `json:"complete"`
}

type FinanceDashboardCategoryBreakdown struct {
	CategoryID       string `json:"categoryId"`
	CategoryName     string `json:"categoryName"`
	Kind             string `json:"kind"`
	IncomeMinor      int64  `json:"incomeMinor"`
	ExpenseMinor     int64  `json:"expenseMinor"`
	TransactionCount int    `json:"transactionCount"`
}

type FinanceDashboardAccountBalance struct {
	AccountID           string `json:"accountId"`
	AccountName         string `json:"accountName"`
	Currency            string `json:"currency"`
	NativeBookedMinor   int64  `json:"nativeBookedMinor"`
	NativePendingMinor  int64  `json:"nativePendingMinor"`
	DisplayBookedMinor  *int64 `json:"displayBookedMinor,omitempty"`
	DisplayPendingMinor *int64 `json:"displayPendingMinor,omitempty"`
	MissingFx           bool   `json:"missingFx"`
}

type FinanceDashboardAlert struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Count    int    `json:"count"`
}

type FinanceDashboardMissingFx struct {
	Source        string    `json:"source"`
	TransactionID string    `json:"transactionId,omitempty"`
	AccountID     string    `json:"accountId,omitempty"`
	BaseCurrency  string    `json:"baseCurrency"`
	QuoteCurrency string    `json:"quoteCurrency"`
	RateDate      time.Time `json:"rateDate"`
	Provider      string    `json:"provider"`
}

type FinanceDashboardCurrencyTotal struct {
	Currency     string `json:"currency"`
	IncomeMinor  int64  `json:"incomeMinor"`
	ExpenseMinor int64  `json:"expenseMinor"`
	NetMinor     int64  `json:"netMinor"`
}
