package finance

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
)

var ErrInvalidDashboardPeriod = errors.New("invalid dashboard period")

type DashboardPeriodPreset string

const (
	DashboardPeriodPresetCurrentMonth  DashboardPeriodPreset = "current_month"
	DashboardPeriodPresetPreviousMonth DashboardPeriodPreset = "previous_month"
	DashboardPeriodPresetNextMonth     DashboardPeriodPreset = "next_month"
	DashboardPeriodPresetLast3Months   DashboardPeriodPreset = "last_3_months"
	DashboardPeriodPresetLast6Months   DashboardPeriodPreset = "last_6_months"
	DashboardPeriodPresetThisYear      DashboardPeriodPreset = "this_year"
	DashboardPeriodPresetPreviousYear  DashboardPeriodPreset = "previous_year"
	DashboardPeriodPresetCustom        DashboardPeriodPreset = "custom"

	dashboardAlertsCapacity = 2
)

type DashboardParams struct {
	ActorUserID string
	TenantID    string
	Preset      DashboardPeriodPreset
	StartDate   time.Time
	EndDate     time.Time
}

type Dashboard struct {
	Period              DashboardPeriod
	Settled             DashboardMoneySummary
	Pending             DashboardMoneySummary
	CategoryBreakdowns  []DashboardCategoryBreakdown
	AccountBalances     []DashboardAccountBalance
	Alerts              []DashboardAlert
	MissingFX           []DashboardMissingFXDiagnostic
	CurrentFXRates      []DashboardFXRate
	NativeSettledTotals []DashboardCurrencyTotal
}

type DashboardMissingFXSource string

const (
	DashboardMissingFXSourceTransaction DashboardMissingFXSource = "transaction"
	DashboardMissingFXSourceBalance     DashboardMissingFXSource = "balance"
)

type DashboardPeriod struct {
	Preset    DashboardPeriodPreset
	StartDate time.Time
	EndDate   time.Time
	Previous  DashboardPeriodWindow
	Next      DashboardPeriodWindow
}

type DashboardPeriodWindow struct {
	StartDate time.Time
	EndDate   time.Time
}

type DashboardMoneySummary struct {
	DisplayCurrency  string
	IncomeMinor      int64
	ExpenseMinor     int64
	NetMinor         int64
	TransactionCount int
	Complete         bool
}

type DashboardCategoryBreakdown struct {
	CategoryID       string
	CategoryName     string
	Kind             domain.CategoryKind
	IncomeMinor      int64
	ExpenseMinor     int64
	TransactionCount int
}

type DashboardAccountBalance struct {
	AccountID           string
	AccountName         string
	Currency            string
	NativeBookedMinor   int64
	NativePendingMinor  int64
	DisplayBookedMinor  *int64
	DisplayPendingMinor *int64
	MissingFX           bool
}

type DashboardAlert struct {
	Code     string
	Severity string
	Count    int
}

type DashboardMissingFXDiagnostic struct {
	Source        DashboardMissingFXSource
	TransactionID string
	AccountID     string
	BaseCurrency  string
	QuoteCurrency string
	RateDate      time.Time
	Provider      string
}

type DashboardFXRate struct {
	Provider                string
	BaseCurrency            string
	QuoteCurrency           string
	EffectiveAt             time.Time
	LastSuccessfulRefreshAt time.Time
	Stale                   bool
}

type DashboardCurrencyTotal struct {
	Currency     string
	IncomeMinor  int64
	ExpenseMinor int64
	NetMinor     int64
}

type dashboardData struct {
	tenant       domain.Tenant
	period       DashboardPeriod
	transactions []domain.Transaction
	accounts     []domain.Account
	balances     []domain.AccountBalance
	categories   []domain.Category
	rateLookup   fxRateLookup
}

type dashboardComputation struct {
	settled             DashboardMoneySummary
	pending             DashboardMoneySummary
	categoryBreakdowns  []DashboardCategoryBreakdown
	missing             []DashboardMissingFXDiagnostic
	nativeSettledTotals []DashboardCurrencyTotal
}

func ValidateDashboardParams(params DashboardParams) error {
	preset := params.Preset
	if preset == "" {
		preset = DashboardPeriodPresetCurrentMonth
	}
	switch preset {
	case DashboardPeriodPresetCurrentMonth,
		DashboardPeriodPresetPreviousMonth,
		DashboardPeriodPresetNextMonth,
		DashboardPeriodPresetLast3Months,
		DashboardPeriodPresetLast6Months,
		DashboardPeriodPresetThisYear,
		DashboardPeriodPresetPreviousYear:
		return nil
	case DashboardPeriodPresetCustom:
		return ValidateRequiredTimestampRange(params.StartDate, params.EndDate)
	default:
		return fmt.Errorf("%w: %q", ErrInvalidDashboardPeriod, preset)
	}
}

func buildDashboardAlerts(
	missing []DashboardMissingFXDiagnostic,
	pendingCount int,
) []DashboardAlert {
	alerts := make([]DashboardAlert, 0, dashboardAlertsCapacity)
	if len(missing) > 0 {
		alerts = append(
			alerts,
			DashboardAlert{Code: "missing_fx_rates", Severity: "warning", Count: len(missing)},
		)
	}
	if pendingCount > 0 {
		alerts = append(
			alerts,
			DashboardAlert{Code: "pending_transactions", Severity: "info", Count: pendingCount},
		)
	}
	return alerts
}

func addDashboardCategoryContribution(
	categoryBreakdowns map[string]*DashboardCategoryBreakdown,
	categoryNames map[string]domain.Category,
	transaction domain.Transaction,
	incomeMinor int64,
	expenseMinor int64,
) {
	if transaction.CategoryID == nil {
		return
	}
	category, found := categoryNames[*transaction.CategoryID]
	if !found {
		return
	}
	breakdown := categoryBreakdowns[category.ID]
	if breakdown == nil {
		breakdown = &DashboardCategoryBreakdown{
			CategoryID:   category.ID,
			CategoryName: category.Name,
			Kind:         category.Kind,
		}
		categoryBreakdowns[category.ID] = breakdown
	}
	applyCategoryContribution(breakdown, incomeMinor, expenseMinor)
	breakdown.TransactionCount++
}

func markIncompleteDashboardSummary(
	status domain.TransactionStatus,
	settled *DashboardMoneySummary,
	pending *DashboardMoneySummary,
) {
	if status == domain.TransactionStatusBooked {
		settled.Complete = false
		return
	}
	pending.Complete = false
}

func sortCategoryBreakdowns(
	categoryBreakdowns map[string]*DashboardCategoryBreakdown,
) []DashboardCategoryBreakdown {
	items := make([]DashboardCategoryBreakdown, 0, len(categoryBreakdowns))
	for _, breakdown := range categoryBreakdowns {
		items = append(items, *breakdown)
	}
	sort.Slice(items, func(i int, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind == domain.CategoryKindIncome
		}
		return items[i].CategoryName < items[j].CategoryName
	})
	return items
}

func sortCurrencyTotals(nativeTotals map[string]*DashboardCurrencyTotal) []DashboardCurrencyTotal {
	items := make([]DashboardCurrencyTotal, 0, len(nativeTotals))
	for _, total := range nativeTotals {
		items = append(items, *total)
	}
	sort.Slice(items, func(i int, j int) bool { return items[i].Currency < items[j].Currency })
	return items
}

func resolveDashboardPeriod(now time.Time, params DashboardParams) DashboardPeriod {
	preset := params.Preset
	if preset == "" {
		preset = DashboardPeriodPresetCurrentMonth
	}
	current := now
	currentMonthStart, _ := calendarMonthWindow(current)
	var startDate time.Time
	var endDate time.Time
	switch preset {
	case DashboardPeriodPresetCurrentMonth:
		startDate, endDate = calendarMonthWindow(currentMonthStart)
	case DashboardPeriodPresetPreviousMonth:
		startDate, endDate = calendarMonthWindow(currentMonthStart.AddDate(0, -1, 0))
	case DashboardPeriodPresetNextMonth:
		startDate, endDate = calendarMonthWindow(currentMonthStart.AddDate(0, 1, 0))
	case DashboardPeriodPresetLast3Months:
		startDate = current.AddDate(0, -3, 0)
		endDate = current
	case DashboardPeriodPresetLast6Months:
		startDate = current.AddDate(0, -6, 0)
		endDate = current
	case DashboardPeriodPresetThisYear:
		startDate = time.Date(
			current.Year(), time.January, 1,
			current.Hour(), current.Minute(), current.Second(), current.Nanosecond(), current.Location(),
		)
		endDate = current
	case DashboardPeriodPresetPreviousYear:
		currentYearStart := time.Date(
			current.Year(), time.January, 1,
			current.Hour(), current.Minute(), current.Second(), current.Nanosecond(), current.Location(),
		)
		startDate = currentYearStart.AddDate(-1, 0, 0)
		endDate = currentYearStart.Add(-time.Nanosecond)
	case DashboardPeriodPresetCustom:
		startDate = params.StartDate
		endDate = params.EndDate
	}
	previousStart, previousEnd, nextStart, nextEnd := shiftPeriodWindow(startDate, endDate)
	if preset == DashboardPeriodPresetCurrentMonth ||
		preset == DashboardPeriodPresetPreviousMonth ||
		preset == DashboardPeriodPresetNextMonth {
		previousStart, previousEnd = calendarMonthWindow(startDate.AddDate(0, -1, 0))
		nextStart, nextEnd = calendarMonthWindow(startDate.AddDate(0, 1, 0))
	}
	return DashboardPeriod{
		Preset:    preset,
		StartDate: startDate,
		EndDate:   endDate,
		Previous:  DashboardPeriodWindow{StartDate: previousStart, EndDate: previousEnd},
		Next:      DashboardPeriodWindow{StartDate: nextStart, EndDate: nextEnd},
	}
}

func calendarMonthWindow(date time.Time) (time.Time, time.Time) {
	startDate := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
	return startDate, startDate.AddDate(0, 1, 0).Add(-time.Nanosecond)
}

func shiftPeriodWindow(
	startDate time.Time,
	endDate time.Time,
) (time.Time, time.Time, time.Time, time.Time) {
	span := endDate.Sub(startDate) + time.Nanosecond
	previousStart := startDate.Add(-span)
	previousEnd := startDate.Add(-time.Nanosecond)
	nextStart := endDate.Add(time.Nanosecond)
	nextEnd := endDate.Add(span)
	return previousStart, previousEnd, nextStart, nextEnd
}

func transactionInPeriod(
	transaction domain.Transaction,
	startDate time.Time,
	endDate time.Time,
) bool {
	return !transaction.EffectiveAt.Before(startDate) && !transaction.EffectiveAt.After(endDate)
}

func reportingContribution(transaction domain.Transaction) (int64, int64, bool) {
	if transaction.HiddenAt != nil {
		return 0, 0, false
	}
	switch transaction.Kind {
	case domain.TransactionKindTransfer:
		if bookedMatchedTransfer(transaction) {
			return 0, 0, false
		}
		if transaction.AmountMinor > 0 {
			return transaction.AmountMinor, 0, true
		}
		if transaction.AmountMinor < 0 {
			return 0, -transaction.AmountMinor, true
		}
		return 0, 0, true
	case domain.TransactionKindRefund:
		return 0, -transaction.AmountMinor, true
	case domain.TransactionKindExpense,
		domain.TransactionKindIncome,
		domain.TransactionKindRegular:
		if transaction.AmountMinor > 0 {
			return transaction.AmountMinor, 0, true
		}
		if transaction.AmountMinor < 0 {
			return 0, -transaction.AmountMinor, true
		}
		return 0, 0, true
	case domain.TransactionKindReconciliation, domain.TransactionKindOpeningBalance:
		return 0, 0, false
	}
	return 0, 0, false
}

func applyDashboardContribution(
	summary *DashboardMoneySummary,
	incomeMinor int64,
	expenseMinor int64,
) {
	summary.IncomeMinor += incomeMinor
	summary.ExpenseMinor += expenseMinor
}

func applyCategoryContribution(
	breakdown *DashboardCategoryBreakdown,
	incomeMinor int64,
	expenseMinor int64,
) {
	breakdown.IncomeMinor += incomeMinor
	breakdown.ExpenseMinor += expenseMinor
}

func addNativeTotal(
	nativeTotals map[string]*DashboardCurrencyTotal,
	currency string,
	incomeMinor int64,
	expenseMinor int64,
) {
	total := nativeTotals[currency]
	if total == nil {
		total = &DashboardCurrencyTotal{Currency: currency}
		nativeTotals[currency] = total
	}
	total.IncomeMinor += incomeMinor
	total.ExpenseMinor += expenseMinor
	total.NetMinor = total.IncomeMinor - total.ExpenseMinor
}

type fxRateLookup struct {
	latest map[string]domain.FXRate
}

func newFXRateLookup(rates []domain.FXRate) fxRateLookup {
	lookup := fxRateLookup{latest: map[string]domain.FXRate{}}
	for _, rate := range rates {
		pairKey := fmt.Sprintf("%s|%s|%s", rate.Provider, rate.BaseCurrency, rate.QuoteCurrency)
		lookup.latest[pairKey] = rate
	}
	return lookup
}

func convertTransactionContribution(
	transaction domain.Transaction,
	displayCurrency string,
	provider string,
	lookup fxRateLookup,
	incomeMinor int64,
	expenseMinor int64,
) (int64, int64, bool) {
	if transaction.Currency == displayCurrency {
		return incomeMinor, expenseMinor, true
	}
	pairKey := fmt.Sprintf("%s|%s|%s", provider, transaction.Currency, displayCurrency)
	rate, found := lookup.latest[pairKey]
	if !found {
		return 0, 0, false
	}
	return int64(math.Round(float64(incomeMinor) * rate.Rate)),
		int64(math.Round(float64(expenseMinor) * rate.Rate)),
		true
}

func convertBalanceAmount(
	amount int64,
	baseCurrency string,
	displayCurrency string,
	provider string,
	lookup fxRateLookup,
) (int64, bool) {
	if amount == 0 {
		zero := int64(0)
		return zero, true
	}
	if baseCurrency == displayCurrency {
		return amount, true
	}
	pairKey := fmt.Sprintf("%s|%s|%s", provider, baseCurrency, displayCurrency)
	selected, found := lookup.latest[pairKey]
	if !found {
		return 0, false
	}
	return int64(math.Round(float64(amount) * selected.Rate)), true
}
