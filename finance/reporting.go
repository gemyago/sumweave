package finance

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
)

type DashboardPeriodPreset string

const (
	DashboardPeriodPresetCurrentMonth  DashboardPeriodPreset = "current_month"
	DashboardPeriodPresetPreviousMonth DashboardPeriodPreset = "previous_month"
	DashboardPeriodPresetLast3Months   DashboardPeriodPreset = "last_3_months"
	DashboardPeriodPresetLast6Months   DashboardPeriodPreset = "last_6_months"
	DashboardPeriodPresetThisYear      DashboardPeriodPreset = "this_year"
	DashboardPeriodPresetPreviousYear  DashboardPeriodPreset = "previous_year"
	DashboardPeriodPresetCustom        DashboardPeriodPreset = "custom"

	dashboardAlertsCapacity = 2
	hoursPerDay             = 24
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
	current := startOfDay(now)
	var startDate time.Time
	var endDate time.Time
	var shiftMonths int
	var shiftYears int
	switch preset {
	case DashboardPeriodPresetCurrentMonth:
		monthStart := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, time.UTC)
		startDate = monthStart
		endDate = monthStart.AddDate(0, 1, -1)
		shiftMonths = 1
	case DashboardPeriodPresetPreviousMonth:
		monthStart := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, time.UTC)
		startDate = monthStart.AddDate(0, -1, 0)
		endDate = monthStart.AddDate(0, 0, -1)
		shiftMonths = 1
	case DashboardPeriodPresetLast3Months:
		monthStart := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, time.UTC)
		startDate = monthStart.AddDate(0, -2, 0)
		endDate = monthStart.AddDate(0, 1, -1)
		shiftMonths = 3
	case DashboardPeriodPresetLast6Months:
		monthStart := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, time.UTC)
		startDate = monthStart.AddDate(0, -5, 0)
		endDate = monthStart.AddDate(0, 1, -1)
		shiftMonths = 6
	case DashboardPeriodPresetThisYear:
		startDate = time.Date(current.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
		endDate = time.Date(current.Year(), time.December, 31, 0, 0, 0, 0, time.UTC)
		shiftYears = 1
	case DashboardPeriodPresetPreviousYear:
		startDate = time.Date(current.Year()-1, time.January, 1, 0, 0, 0, 0, time.UTC)
		endDate = time.Date(current.Year()-1, time.December, 31, 0, 0, 0, 0, time.UTC)
		shiftYears = 1
	case DashboardPeriodPresetCustom:
		startDate = startOfDay(params.StartDate)
		endDate = startOfDay(params.EndDate)
	}
	previousStart, previousEnd, nextStart, nextEnd := shiftPeriodWindow(
		startDate,
		endDate,
		shiftMonths,
		shiftYears,
	)
	return DashboardPeriod{
		Preset:    preset,
		StartDate: startDate,
		EndDate:   endDate,
		Previous:  DashboardPeriodWindow{StartDate: previousStart, EndDate: previousEnd},
		Next:      DashboardPeriodWindow{StartDate: nextStart, EndDate: nextEnd},
	}
}

func shiftPeriodWindow(
	startDate time.Time,
	endDate time.Time,
	shiftMonths int,
	shiftYears int,
) (time.Time, time.Time, time.Time, time.Time) {
	if shiftMonths == 0 && shiftYears == 0 {
		spanDays := int(endDate.Sub(startDate).Hours()/hoursPerDay) + 1
		return startDate.AddDate(0, 0, -spanDays),
			endDate.AddDate(0, 0, -spanDays),
			startDate.AddDate(0, 0, spanDays),
			endDate.AddDate(0, 0, spanDays)
	}
	previousStart := startDate.AddDate(-shiftYears, -shiftMonths, 0)
	previousEnd := startDate.AddDate(0, 0, -1)
	nextStart := endDate.AddDate(0, 0, 1)
	nextEnd := nextStart.AddDate(shiftYears, shiftMonths, -1)
	return previousStart, previousEnd, nextStart, nextEnd
}

func transactionInPeriod(
	transaction domain.Transaction,
	startDate time.Time,
	endDate time.Time,
) bool {
	effectiveDate := startOfDay(transaction.EffectiveAt)
	return !effectiveDate.Before(startDate) && !effectiveDate.After(endDate)
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
	case domain.TransactionKindRegular:
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
	exact  map[string]float64
	latest map[string][]domain.FXRate
}

func newFXRateLookup(rates []domain.FXRate) fxRateLookup {
	lookup := fxRateLookup{exact: map[string]float64{}, latest: map[string][]domain.FXRate{}}
	for _, rate := range rates {
		exactKey := fmt.Sprintf(
			"%s|%s|%s|%s",
			rate.Provider,
			rate.BaseCurrency,
			rate.QuoteCurrency,
			startOfDay(rate.RateDate).Format(time.DateOnly),
		)
		lookup.exact[exactKey] = rate.Rate
		pairKey := fmt.Sprintf("%s|%s|%s", rate.Provider, rate.BaseCurrency, rate.QuoteCurrency)
		lookup.latest[pairKey] = append(lookup.latest[pairKey], rate)
	}
	for pairKey := range lookup.latest {
		sort.Slice(lookup.latest[pairKey], func(i int, j int) bool {
			return lookup.latest[pairKey][i].RateDate.Before(lookup.latest[pairKey][j].RateDate)
		})
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
	key := fmt.Sprintf(
		"%s|%s|%s|%s",
		provider,
		transaction.Currency,
		displayCurrency,
		startOfDay(transaction.EffectiveAt).Format(time.DateOnly),
	)
	rate, ok := lookup.exact[key]
	if !ok {
		return 0, 0, false
	}
	return int64(math.Round(float64(incomeMinor) * rate)),
		int64(math.Round(float64(expenseMinor) * rate)),
		true
}

func convertBalanceAmount(
	amount int64,
	baseCurrency string,
	displayCurrency string,
	endDate time.Time,
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
	rates := lookup.latest[pairKey]
	if len(rates) == 0 {
		return 0, false
	}
	asOfDate := startOfDay(endDate)
	selected := rates[0]
	for _, rate := range rates {
		if rate.RateDate.After(asOfDate) {
			break
		}
		selected = rate
	}
	if selected.RateDate.After(asOfDate) {
		return 0, false
	}
	return int64(math.Round(float64(amount) * selected.Rate)), true
}
