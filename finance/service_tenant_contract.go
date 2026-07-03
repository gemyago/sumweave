package finance

import (
	"errors"

	"github.com/gemyago/signal-foundry/finance/domain"
)

var (
	ErrInviteNotFound = errors.New("tenant invite not found")
	ErrInviteAccepted = errors.New("tenant invite already accepted")
)

const (
	defaultTenantTagTax          = "Tax"
	defaultTenantTagReimburse    = "Reimburse"
	defaultTenantTagSplit        = "Split"
	defaultTenantTagBusiness     = "Business"
	defaultTenantTagSubscription = "Subscription"
	defaultTenantTagTravel       = "Travel"
)

type CreateTenantParams struct {
	ActorUserID     string
	Name            string
	DisplayCurrency string
}

type ArchiveTenantParams struct {
	ActorUserID string
	TenantID    string
}

type CreateTenantInviteParams struct {
	ActorUserID string
	TenantID    string
	Recipient   string
}

type AcceptTenantInviteParams struct {
	ActorUserID string
	Code        string
}

type ListTenantMembersParams struct {
	ActorUserID string
	TenantID    string
}

type ListTenantInvitesParams struct {
	ActorUserID string
	TenantID    string
}

type defaultCategorySeed struct {
	Name string
	Kind domain.CategoryKind
}

func defaultTenantCategorySeeds() []defaultCategorySeed {
	return []defaultCategorySeed{
		{Name: "Paycheck", Kind: domain.CategoryKindIncome},
		{Name: "Bonus", Kind: domain.CategoryKindIncome},
		{Name: "Interest & Dividends", Kind: domain.CategoryKindIncome},
		{Name: "Business Income", Kind: domain.CategoryKindIncome},
		{Name: "Other Income", Kind: domain.CategoryKindIncome},
		{Name: "Housing", Kind: domain.CategoryKindExpense},
		{Name: "Utilities", Kind: domain.CategoryKindExpense},
		{Name: "Groceries", Kind: domain.CategoryKindExpense},
		{Name: "Dining & Coffee", Kind: domain.CategoryKindExpense},
		{Name: "Transportation", Kind: domain.CategoryKindExpense},
		{Name: "Health & Medical", Kind: domain.CategoryKindExpense},
		{Name: "Insurance", Kind: domain.CategoryKindExpense},
		{Name: "Education & Childcare", Kind: domain.CategoryKindExpense},
		{Name: "Pets", Kind: domain.CategoryKindExpense},
		{Name: "Personal Care", Kind: domain.CategoryKindExpense},
		{Name: "Entertainment", Kind: domain.CategoryKindExpense},
		{Name: "Shopping", Kind: domain.CategoryKindExpense},
		{Name: "Home Improvement & Furnishings", Kind: domain.CategoryKindExpense},
		{Name: "Travel & Vacation", Kind: domain.CategoryKindExpense},
		{Name: "Gifts & Donations", Kind: domain.CategoryKindExpense},
		{Name: "Taxes & Fees", Kind: domain.CategoryKindExpense},
		{Name: "Debt Payments", Kind: domain.CategoryKindExpense},
		{Name: "Miscellaneous", Kind: domain.CategoryKindExpense},
	}
}

func defaultTenantTags() []string {
	return []string{
		defaultTenantTagTax,
		defaultTenantTagReimburse,
		defaultTenantTagSplit,
		defaultTenantTagBusiness,
		defaultTenantTagSubscription,
		defaultTenantTagTravel,
	}
}
