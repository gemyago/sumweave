package domain

import "time"

type Tenant struct {
	ID              string
	Name            string
	DisplayCurrency string
	ArchivedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type TenantMembership struct {
	TenantID  string
	UserID    string
	JoinedAt  time.Time
	CreatedAt time.Time
}

type TenantMembershipView struct {
	Tenant     Tenant
	Membership TenantMembership
}

type TenantInvite struct {
	ID               string
	TenantID         string
	Code             string
	Recipient        string
	CreatedByUserID  string
	AcceptedByUserID *string
	CreatedAt        time.Time
	AcceptedAt       *time.Time
}

type TenantMember struct {
	TenantID string
	UserID   string
	JoinedAt time.Time
}

type AccountKind string

const (
	AccountKindManual         AccountKind = "manual"
	AccountKindLinked         AccountKind = "linked"
	AccountKindImported       AccountKind = "imported"
	AccountKindReconciliation AccountKind = "reconciliation"
)

type LinkedAccount struct {
	Provider          string
	ProviderAccountID string
}

type Account struct {
	ID                  string
	TenantID            string
	Name                string
	Currency            string
	Kind                AccountKind
	BookedBalanceMinor  int64
	PendingBalanceMinor int64
	LinkedAccount       *LinkedAccount
	HiddenAt            *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type CategoryKind string

const (
	CategoryKindExpense CategoryKind = "expense"
	CategoryKindIncome  CategoryKind = "income"
)

type Category struct {
	ID            string
	TenantID      string
	Name          string
	Kind          CategoryKind
	SeededDefault bool
	HiddenAt      *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Tag struct {
	ID        string
	TenantID  string
	Name      string
	HiddenAt  *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TransactionSource string

const (
	TransactionSourceManual   TransactionSource = "manual"
	TransactionSourceProvider TransactionSource = "provider"
	TransactionSourceCSV      TransactionSource = "csv"
	TransactionSourceSystem   TransactionSource = "system"
)

type TransactionStatus string

const (
	TransactionStatusPending TransactionStatus = "pending"
	TransactionStatusBooked  TransactionStatus = "booked"
)

type TransactionKind string

const (
	TransactionKindRegular        TransactionKind = "regular"
	TransactionKindRefund         TransactionKind = "refund"
	TransactionKindTransfer       TransactionKind = "transfer"
	TransactionKindReconciliation TransactionKind = "reconciliation"
	TransactionKindOpeningBalance TransactionKind = "opening_balance"
)

type ProviderTransactionOriginal struct {
	AmountMinor int64
	Currency    string
	Description string
	EffectiveAt *time.Time
}

type Transaction struct {
	ID                string
	TenantID          string
	AccountID         string
	Source            TransactionSource
	Status            TransactionStatus
	Kind              TransactionKind
	AmountMinor       int64
	Currency          string
	Description       string
	EffectiveAt       time.Time
	CategoryID        *string
	TransferGroupID   *string
	TransferMatchedAt *time.Time
	HiddenAt          *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ProviderOriginal  *ProviderTransactionOriginal
}

type AccountBalance struct {
	AccountID           string
	BookedBalanceMinor  int64
	PendingBalanceMinor int64
	Currency            string
}

type TransactionSummary struct {
	IncomeMinor  int64
	ExpenseMinor int64
	NetMinor     int64
}
