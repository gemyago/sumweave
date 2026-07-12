package finance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
)

type reportingServiceStore interface {
	IsTenantMember(ctx context.Context, tenantID string, userID string) (bool, error)
	GetTenant(ctx context.Context, tenantID string) (*domain.Tenant, error)
	ListTransactions(
		ctx context.Context,
		tenantID string,
		accountID string,
		source domain.TransactionSource,
		status domain.TransactionStatus,
		includeHidden bool,
		page ...persistence.ListTransactionsPage,
	) ([]domain.Transaction, error)
	ListAccounts(ctx context.Context, tenantID string, includeHidden bool) ([]domain.Account, error)
	ListCategories(ctx context.Context, tenantID string, includeHidden bool) ([]domain.Category, error)
	ListFXRates(ctx context.Context, params persistence.ListFXRatesParams) ([]domain.FXRate, error)
}

type ReportingService struct {
	store             reportingServiceStore
	balanceStore      accountBalanceReadStore
	access            *accessGuard
	now               func() time.Time
	defaultFXProvider string
}

type ReportingServiceOption func(*ReportingService)

func WithReportingServiceAccountBalanceStore(store accountBalanceReadStore) ReportingServiceOption {
	return func(service *ReportingService) {
		service.balanceStore = store
	}
}

func WithReportingServiceNow(now func() time.Time) ReportingServiceOption {
	return func(service *ReportingService) {
		service.now = now
	}
}

func WithReportingServiceDefaultFXProvider(name string) ReportingServiceOption {
	return func(service *ReportingService) {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			service.defaultFXProvider = trimmed
		}
	}
}

func NewReportingService(store reportingServiceStore, opts ...ReportingServiceOption) *ReportingService {
	service := &ReportingService{
		store:             store,
		access:            newAccessGuard(store),
		now:               time.Now,
		defaultFXProvider: FXProviderFrankfurter,
	}
	for _, opt := range opts {
		opt(service)
	}
	assignAccountBalanceReadStore(store, &service.balanceStore)
	return service
}

func (s *ReportingService) GetDashboard(ctx context.Context, params DashboardParams) (Dashboard, error) {
	if err := ValidateDashboardParams(params); err != nil {
		return Dashboard{}, err
	}
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return Dashboard{}, err
	}
	data, err := s.loadDashboardData(ctx, strings.TrimSpace(params.TenantID), params)
	if err != nil {
		return Dashboard{}, err
	}
	computation := s.computeDashboard(data)
	accountBalances, balanceMissing := s.buildDashboardAccountBalances(data)
	missing := append([]DashboardMissingFXDiagnostic{}, computation.missing...)
	missing = append(missing, balanceMissing...)
	return Dashboard{
		Period:              data.period,
		Settled:             computation.settled,
		Pending:             computation.pending,
		CategoryBreakdowns:  computation.categoryBreakdowns,
		AccountBalances:     accountBalances,
		Alerts:              buildDashboardAlerts(missing, computation.pending.TransactionCount),
		MissingFX:           missing,
		NativeSettledTotals: computation.nativeSettledTotals,
	}, nil
}

func (s *ReportingService) loadDashboardData(
	ctx context.Context,
	tenantID string,
	params DashboardParams,
) (dashboardData, error) {
	tenant, err := s.store.GetTenant(ctx, tenantID)
	if err != nil {
		return dashboardData{}, fmt.Errorf("get dashboard: %w", err)
	}
	period := resolveDashboardPeriod(s.now(), params)
	transactions, err := s.store.ListTransactions(ctx, tenant.ID, "", "", "", false)
	if err != nil {
		return dashboardData{}, fmt.Errorf("get dashboard: %w", err)
	}
	accounts, err := s.store.ListAccounts(ctx, tenant.ID, false)
	if err != nil {
		return dashboardData{}, fmt.Errorf("get dashboard: %w", err)
	}
	balanceItems, err := s.balanceStore.ListAccountBalances(ctx, persistence.ListAccountBalancesParams{
		TenantID:              tenant.ID,
		AccountIDs:            accountIDs(accounts),
		EffectiveAtOnOrBefore: &period.EndDate,
	})
	if err != nil {
		return dashboardData{}, fmt.Errorf("get dashboard: %w", err)
	}
	categories, err := s.store.ListCategories(ctx, tenant.ID, true)
	if err != nil {
		return dashboardData{}, fmt.Errorf("get dashboard: %w", err)
	}
	fxRates, err := s.store.ListFXRates(ctx, persistence.ListFXRatesParams{
		Provider: s.defaultFXProvider,
		EndDate:  period.EndDate,
	})
	if err != nil {
		return dashboardData{}, fmt.Errorf("get dashboard: %w", err)
	}
	return dashboardData{
		tenant:       *tenant,
		period:       period,
		transactions: transactions,
		accounts:     accounts,
		balances:     balanceItems,
		categories:   categories,
		rateLookup:   newFXRateLookup(fxRates),
	}, nil
}

func (s *ReportingService) computeDashboard(data dashboardData) dashboardComputation {
	categoryNames := make(map[string]domain.Category, len(data.categories))
	for _, category := range data.categories {
		categoryNames[category.ID] = category
	}
	settled := DashboardMoneySummary{DisplayCurrency: data.tenant.DisplayCurrency, Complete: true}
	pending := DashboardMoneySummary{DisplayCurrency: data.tenant.DisplayCurrency, Complete: true}
	categoryBreakdowns := map[string]*DashboardCategoryBreakdown{}
	nativeTotals := map[string]*DashboardCurrencyTotal{}
	missing := make([]DashboardMissingFXDiagnostic, 0)
	for _, transaction := range data.transactions {
		s.processDashboardTransaction(
			transaction,
			data,
			categoryNames,
			&settled,
			&pending,
			categoryBreakdowns,
			nativeTotals,
			&missing,
		)
	}
	settled.NetMinor = settled.IncomeMinor - settled.ExpenseMinor
	pending.NetMinor = pending.IncomeMinor - pending.ExpenseMinor
	return dashboardComputation{
		settled:             settled,
		pending:             pending,
		categoryBreakdowns:  sortCategoryBreakdowns(categoryBreakdowns),
		missing:             missing,
		nativeSettledTotals: sortCurrencyTotals(nativeTotals),
	}
}

func (s *ReportingService) processDashboardTransaction(
	transaction domain.Transaction,
	data dashboardData,
	categoryNames map[string]domain.Category,
	settled *DashboardMoneySummary,
	pending *DashboardMoneySummary,
	categoryBreakdowns map[string]*DashboardCategoryBreakdown,
	nativeTotals map[string]*DashboardCurrencyTotal,
	missing *[]DashboardMissingFXDiagnostic,
) {
	if transaction.HiddenAt != nil || !transactionInPeriod(transaction, data.period.StartDate, data.period.EndDate) {
		return
	}
	incomeContribution, expenseContribution, includeInReporting := reportingContribution(transaction)
	if !includeInReporting {
		return
	}
	if transaction.Status == domain.TransactionStatusBooked {
		addNativeTotal(nativeTotals, transaction.Currency, incomeContribution, expenseContribution)
	}
	convertedIncome, convertedExpense, converted := convertTransactionContribution(
		transaction,
		data.tenant.DisplayCurrency,
		s.defaultFXProvider,
		data.rateLookup,
		incomeContribution,
		expenseContribution,
	)
	if !converted {
		*missing = append(*missing, DashboardMissingFXDiagnostic{
			Source:        DashboardMissingFXSourceTransaction,
			TransactionID: transaction.ID,
			AccountID:     transaction.AccountID,
			BaseCurrency:  transaction.Currency,
			QuoteCurrency: data.tenant.DisplayCurrency,
			RateDate:      transaction.EffectiveAt,
			Provider:      s.defaultFXProvider,
		})
		markIncompleteDashboardSummary(transaction.Status, settled, pending)
		return
	}
	target := pending
	if transaction.Status == domain.TransactionStatusBooked {
		target = settled
	}
	applyDashboardContribution(target, convertedIncome, convertedExpense)
	target.TransactionCount++
	if transaction.Status == domain.TransactionStatusBooked {
		addDashboardCategoryContribution(
			categoryBreakdowns,
			categoryNames,
			transaction,
			convertedIncome,
			convertedExpense,
		)
	}
}

func (s *ReportingService) buildDashboardAccountBalances(
	data dashboardData,
) ([]DashboardAccountBalance, []DashboardMissingFXDiagnostic) {
	accountBalances := make([]DashboardAccountBalance, 0, len(data.accounts))
	missing := make([]DashboardMissingFXDiagnostic, 0)
	cutoffDate := data.period.EndDate
	balanceByAccountID := make(map[string]domain.AccountBalance, len(data.balances))
	for _, item := range data.balances {
		balanceByAccountID[item.AccountID] = item
	}
	for _, account := range data.accounts {
		balance := DashboardAccountBalance{
			AccountID:   account.ID,
			AccountName: account.Name,
			Currency:    account.Currency,
		}
		aggregate := balanceByAccountID[account.ID]
		balance.NativeBookedMinor = aggregate.BookedBalanceMinor
		balance.NativePendingMinor = aggregate.PendingBalanceMinor
		bookedDisplay, bookedOK := convertBalanceAmount(
			balance.NativeBookedMinor,
			account.Currency,
			data.tenant.DisplayCurrency,
			data.period.EndDate,
			s.defaultFXProvider,
			data.rateLookup,
		)
		pendingDisplay, pendingOK := convertBalanceAmount(
			balance.NativePendingMinor,
			account.Currency,
			data.tenant.DisplayCurrency,
			data.period.EndDate,
			s.defaultFXProvider,
			data.rateLookup,
		)
		if bookedOK {
			balance.DisplayBookedMinor = &bookedDisplay
		}
		if pendingOK {
			balance.DisplayPendingMinor = &pendingDisplay
		}
		balance.MissingFX =
			(!bookedOK && balance.NativeBookedMinor != 0) ||
				(!pendingOK && balance.NativePendingMinor != 0)
		if balance.MissingFX {
			missing = append(missing, DashboardMissingFXDiagnostic{
				Source:        DashboardMissingFXSourceBalance,
				AccountID:     account.ID,
				BaseCurrency:  account.Currency,
				QuoteCurrency: data.tenant.DisplayCurrency,
				RateDate:      cutoffDate,
				Provider:      s.defaultFXProvider,
			})
		}
		accountBalances = append(accountBalances, balance)
	}
	return accountBalances, missing
}
