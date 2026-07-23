package finance

import (
	"errors"

	"github.com/gemyago/signal-foundry/finance/domain"
)

var (
	ErrAccountNotFound  = errors.New("account not found")
	ErrCategoryNotFound = errors.New("category not found")
	ErrTagNotFound      = errors.New("tag not found")
)

type CreateAccountParams struct {
	ActorUserID string
	TenantID    string
	Name        string
	Currency    string
	Kind        domain.AccountKind
}

type UpdateAccountParams struct {
	ActorUserID string
	TenantID    string
	AccountID   string
	Name        string
}

type HideAccountParams struct {
	ActorUserID string
	TenantID    string
	AccountID   string
}

type UnhideAccountParams struct {
	ActorUserID string
	TenantID    string
	AccountID   string
}

type AttachLinkedAccountParams struct {
	ActorUserID       string
	TenantID          string
	AccountID         string
	Provider          string
	ProviderAccountID string
}

type GetAccountParams struct {
	ActorUserID string
	TenantID    string
	AccountID   string
}

type ListAccountsParams struct {
	ActorUserID   string
	TenantID      string
	IncludeHidden bool
}

type CreateCategoryParams struct {
	ActorUserID string
	TenantID    string
	Name        string
	Kind        domain.CategoryKind
}

type UpdateCategoryParams struct {
	ActorUserID string
	TenantID    string
	CategoryID  string
	Name        string
	Kind        domain.CategoryKind
}

type HideCategoryParams struct {
	ActorUserID string
	TenantID    string
	CategoryID  string
}

type ListCategoriesParams struct {
	ActorUserID   string
	TenantID      string
	IncludeHidden bool
}

type CreateTagParams struct {
	ActorUserID string
	TenantID    string
	Name        string
}

type UpdateTagParams struct {
	ActorUserID string
	TenantID    string
	TagID       string
	Name        string
}

type HideTagParams struct {
	ActorUserID string
	TenantID    string
	TagID       string
}

type ListTagsParams struct {
	ActorUserID   string
	TenantID      string
	IncludeHidden bool
}
