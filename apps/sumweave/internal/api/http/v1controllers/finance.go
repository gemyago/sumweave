package v1controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/middleware"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/v1routes/handlers"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/v1routes/models"
	"github.com/gemyago/sumweave/apps/sumweave/internal/app"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/runtime/httpapi"
)

const providerRequestFailedMessage = "provider request failed"

type tenantService interface {
	CreateTenant(context.Context, financepkg.CreateTenantParams) (domain.Tenant, error)
	UpdateTenant(context.Context, financepkg.UpdateTenantParams) (domain.Tenant, error)
	ArchiveTenant(context.Context, financepkg.ArchiveTenantParams) (domain.Tenant, error)
	ListTenantsForUser(context.Context, string) ([]domain.TenantMembershipView, error)
	ListTenantMembers(
		context.Context,
		financepkg.ListTenantMembersParams,
	) ([]domain.TenantMember, error)
	ListTenantInvites(
		context.Context,
		financepkg.ListTenantInvitesParams,
	) ([]domain.TenantInvite, error)
	CreateTenantInvite(
		context.Context,
		financepkg.CreateTenantInviteParams,
	) (domain.TenantInvite, error)
	AcceptTenantInvite(
		context.Context,
		financepkg.AcceptTenantInviteParams,
	) (domain.TenantMembership, error)
}

type userDirectory interface {
	LookupUsername(context.Context, string) (string, bool, error)
}

type catalogService interface {
	CreateAccount(context.Context, financepkg.CreateAccountParams) (domain.Account, error)
	UpdateAccount(context.Context, financepkg.UpdateAccountParams) (domain.Account, error)
	HideAccount(context.Context, financepkg.HideAccountParams) error
	UnhideAccount(context.Context, financepkg.UnhideAccountParams) error
	GetAccount(context.Context, financepkg.GetAccountParams) (domain.Account, error)
	ListAccounts(context.Context, financepkg.ListAccountsParams) ([]domain.Account, error)
	CreateCategory(context.Context, financepkg.CreateCategoryParams) (domain.Category, error)
	UpdateCategory(context.Context, financepkg.UpdateCategoryParams) (domain.Category, error)
	ListCategories(context.Context, financepkg.ListCategoriesParams) ([]domain.Category, error)
	CreateTag(context.Context, financepkg.CreateTagParams) (domain.Tag, error)
	UpdateTag(context.Context, financepkg.UpdateTagParams) (domain.Tag, error)
	ListTags(context.Context, financepkg.ListTagsParams) ([]domain.Tag, error)
}

type ledgerService interface {
	RecordTransaction(
		context.Context,
		financepkg.RecordTransactionParams,
	) (domain.Transaction, error)
	GetTransaction(
		context.Context,
		financepkg.GetTransactionParams,
	) (domain.Transaction, error)
	UpdateTransaction(
		context.Context,
		financepkg.UpdateTransactionParams,
	) (domain.Transaction, error)
	LinkTransfers(context.Context, financepkg.LinkTransfersParams) error
	UnlinkTransfers(context.Context, financepkg.UnlinkTransfersParams) error
	ListTransactions(
		context.Context,
		financepkg.ListTransactionsParams,
	) ([]domain.Transaction, error)
}

type transferDetailService interface {
	ListTransferCandidates(context.Context, financepkg.ListTransferCandidatesParams) ([]domain.Transaction, error)
	GetTransferPartner(context.Context, financepkg.GetTransferPartnerParams) (domain.Transaction, error)
}

type bankSyncService interface {
	ListBankConnections(
		context.Context,
		financepkg.ListBankConnectionsParams,
	) ([]financepkg.BankConnectionView, error)
	ListBankConnectionSyncedAccounts(
		context.Context,
		financepkg.ListBankConnectionSyncedAccountsParams,
	) ([]financepkg.BankConnectionSyncedAccount, error)
	TriggerBankConnectionSync(
		context.Context,
		financepkg.TriggerBankConnectionSyncParams,
	) (financepkg.BankConnectionSyncJobRef, error)
	DeleteBankConnection(context.Context, financepkg.DeleteBankConnectionParams) error
}

type reportingService interface {
	GetDashboard(context.Context, financepkg.DashboardParams) (financepkg.Dashboard, error)
}

type fxService interface {
	GetFXAdminDiagnostics(
		context.Context,
		financepkg.FXAdminDiagnosticsParams,
	) (financepkg.FXAdminDiagnostics, error)
	TriggerFXRefresh(context.Context, financepkg.TriggerFXRefreshParams) (financepkg.FXRefreshJobRef, error)
}

type providerEvidenceService interface {
	ListAccountProviderEvidence(
		context.Context,
		financepkg.ListAccountProviderEvidenceParams,
	) ([]domain.ProviderEvidence, error)
	GetAccountProviderEvidence(
		context.Context,
		financepkg.GetAccountProviderEvidenceParams,
	) (domain.ProviderEvidence, error)
	ListTransactionProviderEvidence(
		context.Context,
		financepkg.ListTransactionProviderEvidenceParams,
	) ([]domain.ProviderEvidence, error)
	GetTransactionProviderEvidence(
		context.Context,
		financepkg.GetTransactionProviderEvidenceParams,
	) (domain.ProviderEvidence, error)
}

type csvImportService interface {
	PreviewCSVImport(
		context.Context,
		financepkg.PreviewCSVImportParams,
	) (financepkg.CSVImportPreview, error)
	ConfirmCSVImport(
		context.Context,
		financepkg.ConfirmCSVImportParams,
	) (financepkg.CSVImportConfirmation, error)
	GetCSVImportAudit(
		context.Context,
		financepkg.GetCSVImportAuditParams,
	) (financepkg.CSVImportAudit, error)
	ListRecentCSVImportAudits(
		context.Context,
		financepkg.ListRecentCSVImportAuditsParams,
	) ([]financepkg.CSVImportAudit, error)
}

type financeService interface {
	tenantService
	catalogService
	ledgerService
	bankSyncService
	reportingService
	fxService
	providerEvidenceService
	csvImportService
}

type bankConnectionService interface {
	UpdateBankConnection(context.Context, financepkg.UpdateBankConnectionParams) error
	LinkTokenBankConnection(
		context.Context,
		financepkg.LinkTokenBankConnectionParams,
	) (domain.BankConnection, error)
	StartBankConnectionLink(
		context.Context,
		financepkg.StartBankConnectionLinkParams,
	) (financepkg.ProviderLinkStart, error)
	FinishBankConnectionLink(
		context.Context,
		financepkg.FinishBankConnectionLinkParams,
	) (domain.BankConnection, error)
}

type syntheticLinkStateService interface {
	GetPendingSyntheticLinkState(
		context.Context,
		financepkg.GetPendingSyntheticLinkStateParams,
	) (financepkg.PendingSyntheticLinkState, error)
	SavePendingSyntheticLinkState(
		context.Context,
		financepkg.SavePendingSyntheticLinkStateParams,
	) (financepkg.PendingSyntheticLinkState, error)
}

type FinanceControllerDeps struct {
	TenantService                tenantService
	UserDirectory                userDirectory
	CatalogService               catalogService
	LedgerService                ledgerService
	TransferDetailService        transferDetailService
	BankSyncService              bankSyncService
	ReportingService             reportingService
	FXService                    fxService
	ProviderEvidenceService      providerEvidenceService
	CSVImportService             csvImportService
	BankConnectionService        bankConnectionService
	SyntheticLinkStateService    syntheticLinkStateService
	AuthMiddleware               middleware.AuthMiddleware
	EnableBankingCallbackBaseURL string
}

type FinanceController struct{ deps FinanceControllerDeps }

func NewFinanceController(deps FinanceControllerDeps) *FinanceController {
	return &FinanceController{deps: deps}
}

var _ handlers.FinanceController = (*FinanceController)(nil)

func (c *FinanceController) AcceptFinanceTenantInvite(
	builder handlers.HandlerBuilder[*models.AcceptFinanceTenantInviteParams, *models.FinanceTenantMember],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.AcceptFinanceTenantInviteParams,
	) (*models.FinanceTenantMember, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		membership, err := c.deps.TenantService.AcceptTenantInvite(
			ctx,
			financepkg.AcceptTenantInviteParams{ActorUserID: userID, Code: params.Payload.Code},
		)
		if err != nil {
			return nil, err
		}

		mapped, err := c.mapTenantMember(ctx, domain.TenantMember{
			TenantID: membership.TenantID,
			UserID:   membership.UserID,
			JoinedAt: membership.JoinedAt,
		})
		if err != nil {
			return nil, err
		}
		return &mapped, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) ConfirmFinanceCsvImport(
	builder handlers.HandlerBuilder[*models.ConfirmFinanceCsvImportParams, *models.FinanceCsvImportConfirmResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ConfirmFinanceCsvImportParams,
	) (*models.FinanceCsvImportConfirmResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}
		item, err := c.deps.CSVImportService.ConfirmCSVImport(
			ctx,
			financepkg.ConfirmCSVImportParams{
				ActorUserID:        userID,
				ImportID:           params.ImportID,
				ExpectedImportType: financepkg.CSVImportTypeTransactions,
			},
		)
		if err != nil {
			return nil, mapCSVImportError(err)
		}

		return &models.FinanceCsvImportConfirmResponse{
			ImportID: item.ImportID,
			JobID:    item.JobID,
			JobType:  item.JobType,
		}, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) ConfirmFinanceAccountCsvImport(
	builder handlers.HandlerBuilder[
		*models.ConfirmFinanceAccountCsvImportParams,
		*models.FinanceCsvImportConfirmResponse,
	],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ConfirmFinanceAccountCsvImportParams,
	) (*models.FinanceCsvImportConfirmResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}
		item, err := c.deps.CSVImportService.ConfirmCSVImport(
			ctx,
			financepkg.ConfirmCSVImportParams{
				ActorUserID:        userID,
				ImportID:           params.ImportID,
				ExpectedImportType: financepkg.CSVImportTypeAccounts,
			},
		)
		if err != nil {
			return nil, mapCSVImportError(err)
		}
		return &models.FinanceCsvImportConfirmResponse{
			ImportID: item.ImportID,
			JobID:    item.JobID,
			JobType:  item.JobType,
		}, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) CreateFinanceAccount(
	builder handlers.HandlerBuilder[*models.CreateFinanceAccountParams, *models.FinanceAccount],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.CreateFinanceAccountParams,
	) (*models.FinanceAccount, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		item, err := c.deps.CatalogService.CreateAccount(
			ctx,
			financepkg.CreateAccountParams{
				ActorUserID: userID,
				TenantID:    params.TenantID,
				Name:        params.Payload.Name,
				Currency:    params.Payload.Currency,
				Kind:        domain.AccountKind(params.Payload.Kind),
			},
		)
		if err != nil {
			return nil, err
		}

		mapped, err := mapAccount(item)
		if err != nil {
			return nil, err
		}
		return &mapped, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) UpdateFinanceAccount(
	builder handlers.NoResponseHandlerBuilder[*models.UpdateFinanceAccountParams],
) http.Handler {
	inner := builder.HandleWith(func(ctx context.Context, params *models.UpdateFinanceAccountParams) error {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return err
		}
		_, err = c.deps.CatalogService.UpdateAccount(ctx, financepkg.UpdateAccountParams{
			ActorUserID: userID,
			TenantID:    params.TenantID,
			AccountID:   params.AccountID,
			Name:        params.Payload.Name,
		})
		return mapCatalogError(err)
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) HideFinanceAccount(
	builder handlers.NoResponseHandlerBuilder[*models.HideFinanceAccountParams],
) http.Handler {
	inner := builder.HandleWith(func(ctx context.Context, params *models.HideFinanceAccountParams) error {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return err
		}
		return mapCatalogError(c.deps.CatalogService.HideAccount(ctx, financepkg.HideAccountParams{
			ActorUserID: userID,
			TenantID:    params.TenantID,
			AccountID:   params.AccountID,
		}))
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) UnhideFinanceAccount(
	builder handlers.NoResponseHandlerBuilder[*models.UnhideFinanceAccountParams],
) http.Handler {
	inner := builder.HandleWith(func(ctx context.Context, params *models.UnhideFinanceAccountParams) error {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return err
		}
		return mapCatalogError(c.deps.CatalogService.UnhideAccount(ctx, financepkg.UnhideAccountParams{
			ActorUserID: userID,
			TenantID:    params.TenantID,
			AccountID:   params.AccountID,
		}))
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) CreateFinanceCategory(
	builder handlers.HandlerBuilder[*models.CreateFinanceCategoryParams, *models.FinanceCategory],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.CreateFinanceCategoryParams,
	) (*models.FinanceCategory, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		item, err := c.deps.CatalogService.CreateCategory(
			ctx,
			financepkg.CreateCategoryParams{
				ActorUserID: userID,
				TenantID:    params.TenantID,
				Name:        params.Payload.Name,
				Kind:        domain.CategoryKind(params.Payload.Kind),
			},
		)
		if err != nil {
			return nil, err
		}

		mapped := mapCategory(item)
		return &mapped, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) UpdateFinanceCategory(
	builder handlers.NoResponseHandlerBuilder[*models.UpdateFinanceCategoryParams],
) http.Handler {
	inner := builder.HandleWith(func(ctx context.Context, params *models.UpdateFinanceCategoryParams) error {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return err
		}
		_, err = c.deps.CatalogService.UpdateCategory(ctx, financepkg.UpdateCategoryParams{
			ActorUserID: userID,
			TenantID:    params.TenantID,
			CategoryID:  params.CategoryID,
			Name:        params.Payload.Name,
			Kind:        domain.CategoryKind(params.Payload.Kind),
		})
		return mapCatalogError(err)
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) CreateFinanceTag(
	builder handlers.HandlerBuilder[*models.CreateFinanceTagParams, *models.FinanceTag],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.CreateFinanceTagParams,
	) (*models.FinanceTag, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		item, err := c.deps.CatalogService.CreateTag(
			ctx,
			financepkg.CreateTagParams{
				ActorUserID: userID,
				TenantID:    params.TenantID,
				Name:        params.Payload.Name,
			},
		)
		if err != nil {
			return nil, err
		}

		mapped := mapTag(item)
		return &mapped, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) UpdateFinanceTag(
	builder handlers.NoResponseHandlerBuilder[*models.UpdateFinanceTagParams],
) http.Handler {
	inner := builder.HandleWith(func(ctx context.Context, params *models.UpdateFinanceTagParams) error {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return err
		}
		_, err = c.deps.CatalogService.UpdateTag(ctx, financepkg.UpdateTagParams{
			ActorUserID: userID,
			TenantID:    params.TenantID,
			TagID:       params.TagID,
			Name:        params.Payload.Name,
		})
		return mapCatalogError(err)
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) CreateFinanceTenant(
	builder handlers.HandlerBuilder[*models.CreateFinanceTenantParams, *models.FinanceTenantSummary],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.CreateFinanceTenantParams,
	) (*models.FinanceTenantSummary, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}
		if params.Payload.SeedDefaults == nil {
			return nil, app.NewErrInvalidInput("seedDefaults", "choice is required")
		}

		tenant, err := c.deps.TenantService.CreateTenant(
			ctx,
			financepkg.CreateTenantParams{
				ActorUserID:     userID,
				Name:            params.Payload.Name,
				DisplayCurrency: string(params.Payload.DisplayCurrency),
				SeedDefaults:    *params.Payload.SeedDefaults,
			},
		)
		if err != nil {
			return nil, mapCSVImportError(err)
		}

		mapped := mapTenantSummary(domain.TenantMembershipView{
			Tenant: tenant,
			Membership: domain.TenantMembership{
				TenantID:  tenant.ID,
				UserID:    userID,
				JoinedAt:  tenant.CreatedAt,
				CreatedAt: tenant.CreatedAt,
			},
		})

		return &mapped, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) UpdateFinanceTenant(
	builder handlers.NoResponseHandlerBuilder[*models.UpdateFinanceTenantParams],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.UpdateFinanceTenantParams,
	) error {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return err
		}

		_, err = c.deps.TenantService.UpdateTenant(
			ctx,
			financepkg.UpdateTenantParams{
				ActorUserID:     userID,
				TenantID:        params.TenantID,
				Name:            params.Payload.Name,
				DisplayCurrency: string(params.Payload.DisplayCurrency),
			},
		)
		if err != nil {
			return mapCSVImportError(err)
		}

		return nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) ArchiveFinanceTenant(
	builder handlers.NoResponseHandlerBuilder[*models.ArchiveFinanceTenantParams],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ArchiveFinanceTenantParams,
	) error {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return err
		}

		_, err = c.deps.TenantService.ArchiveTenant(
			ctx,
			financepkg.ArchiveTenantParams{ActorUserID: userID, TenantID: params.TenantID},
		)
		if err != nil {
			return mapCSVImportError(err)
		}
		return nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) CreateFinanceTenantInvite(
	builder handlers.HandlerBuilder[*models.CreateFinanceTenantInviteParams, *models.FinanceTenantInvite],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.CreateFinanceTenantInviteParams,
	) (*models.FinanceTenantInvite, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		invite, err := c.deps.TenantService.CreateTenantInvite(
			ctx,
			financepkg.CreateTenantInviteParams{
				ActorUserID: userID,
				TenantID:    params.TenantID,
				Recipient:   params.Payload.Recipient,
			},
		)
		if err != nil {
			return nil, mapCSVImportError(err)
		}

		mapped := mapTenantInvite(invite)
		return &mapped, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) CreateFinanceTransaction(
	builder handlers.HandlerBuilder[*models.CreateFinanceTransactionParams, *models.FinanceTransaction],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.CreateFinanceTransactionParams,
	) (*models.FinanceTransaction, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}
		if validationErr := validateFinanceTimestamp("effectiveAt", params.Payload.EffectiveAt); validationErr != nil {
			return nil, app.NewErrInvalidInput("effectiveAt", validationErr.Error())
		}

		item, err := c.deps.LedgerService.RecordTransaction(
			ctx,
			financepkg.RecordTransactionParams{
				ActorUserID:     userID,
				TenantID:        params.TenantID,
				AccountID:       params.Payload.AccountID,
				Source:          domain.TransactionSource(params.Payload.Source),
				Status:          domain.TransactionStatus(params.Payload.Status),
				Kind:            domain.TransactionKind(params.Payload.Kind),
				AmountMinor:     params.Payload.AmountMinor,
				Currency:        params.Payload.Currency,
				Description:     params.Payload.Description,
				EffectiveAt:     params.Payload.EffectiveAt,
				CategoryID:      params.Payload.CategoryID,
				TagIDs:          params.Payload.TagIDs,
				TransferGroupID: params.Payload.TransferGroupID,
			},
		)
		if err != nil {
			return nil, mapCatalogError(mapTransactionTagError(err))
		}

		mapped := mapTransaction(item)
		return &mapped, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) GetFinanceTransaction(
	builder handlers.HandlerBuilder[*models.GetFinanceTransactionParams, *models.FinanceTransaction],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.GetFinanceTransactionParams,
	) (*models.FinanceTransaction, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		item, err := c.deps.LedgerService.GetTransaction(
			ctx,
			financepkg.GetTransactionParams{
				ActorUserID:   userID,
				TenantID:      params.TenantID,
				TransactionID: params.TransactionID,
			},
		)
		if err != nil {
			return nil, err
		}

		mapped := mapTransaction(item)
		return &mapped, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) ListFinanceTransferCandidates(
	builder handlers.HandlerBuilder[
		*models.ListFinanceTransferCandidatesParams,
		*models.FinanceTransactionsResponse,
	],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ListFinanceTransferCandidatesParams,
	) (*models.FinanceTransactionsResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}
		items, err := c.deps.TransferDetailService.ListTransferCandidates(
			ctx,
			financepkg.ListTransferCandidatesParams{
				ActorUserID:     userID,
				TenantID:        params.TenantID,
				TransactionID:   params.TransactionID,
				EffectiveFrom:   params.EffectiveFrom,
				EffectiveBefore: params.EffectiveBefore,
				Limit:           params.Limit,
				Offset:          params.Offset,
			},
		)
		if err != nil {
			return nil, mapTransferDetailError(err)
		}
		return mapTransactionsResponse(items), nil
	})
	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) GetFinanceTransferPartner(
	builder handlers.HandlerBuilder[*models.GetFinanceTransferPartnerParams, *models.FinanceTransaction],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.GetFinanceTransferPartnerParams,
	) (*models.FinanceTransaction, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}
		item, err := c.deps.TransferDetailService.GetTransferPartner(
			ctx,
			financepkg.GetTransferPartnerParams{
				ActorUserID:   userID,
				TenantID:      params.TenantID,
				TransactionID: params.TransactionID,
			},
		)
		if err != nil {
			return nil, mapTransferDetailError(err)
		}
		mapped := mapTransaction(item)
		return &mapped, nil
	})
	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) LinkFinanceTransferPair(
	builder handlers.NoResponseHandlerBuilder[*models.LinkFinanceTransferPairParams],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.LinkFinanceTransferPairParams,
	) error {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return err
		}
		if linkErr := c.deps.LedgerService.LinkTransfers(ctx, financepkg.LinkTransfersParams{
			ActorUserID:         userID,
			TenantID:            params.TenantID,
			FirstTransactionID:  params.Payload.FirstTransactionID,
			SecondTransactionID: params.Payload.SecondTransactionID,
		}); linkErr != nil {
			return mapTransferPairError(linkErr)
		}
		return nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) UnlinkFinanceTransferPair(
	builder handlers.NoResponseHandlerBuilder[*models.UnlinkFinanceTransferPairParams],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.UnlinkFinanceTransferPairParams,
	) error {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return err
		}
		if unlinkErr := c.deps.LedgerService.UnlinkTransfers(ctx, financepkg.UnlinkTransfersParams{
			ActorUserID:         userID,
			TenantID:            params.TenantID,
			FirstTransactionID:  params.Payload.FirstTransactionID,
			SecondTransactionID: params.Payload.SecondTransactionID,
		}); unlinkErr != nil {
			return mapTransferPairError(unlinkErr)
		}
		return nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) GetFinanceAccountProviderEvidence(
	builder handlers.HandlerBuilder[
		*models.GetFinanceAccountProviderEvidenceParams,
		*models.FinanceProviderEvidence,
	],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.GetFinanceAccountProviderEvidenceParams,
	) (*models.FinanceProviderEvidence, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}
		item, err := c.deps.ProviderEvidenceService.GetAccountProviderEvidence(
			ctx,
			financepkg.GetAccountProviderEvidenceParams{
				ActorUserID: userID,
				TenantID:    params.TenantID,
				AccountID:   params.AccountID,
				EvidenceID:  params.EvidenceID,
			},
		)
		if err != nil {
			return nil, err
		}
		mapped, mapErr := mapProviderEvidence(item)
		if mapErr != nil {
			return nil, mapErr
		}
		return &mapped, nil
	})
	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) GetFinanceTransactionProviderEvidence(
	builder handlers.HandlerBuilder[
		*models.GetFinanceTransactionProviderEvidenceParams,
		*models.FinanceProviderEvidence,
	],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.GetFinanceTransactionProviderEvidenceParams,
	) (*models.FinanceProviderEvidence, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}
		item, err := c.deps.ProviderEvidenceService.GetTransactionProviderEvidence(
			ctx,
			financepkg.GetTransactionProviderEvidenceParams{
				ActorUserID:   userID,
				TenantID:      params.TenantID,
				TransactionID: params.TransactionID,
				EvidenceID:    params.EvidenceID,
			},
		)
		if err != nil {
			return nil, err
		}
		mapped, mapErr := mapProviderEvidence(item)
		if mapErr != nil {
			return nil, mapErr
		}
		return &mapped, nil
	})
	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) GetFinanceCsvImportAudit(
	builder handlers.HandlerBuilder[*models.GetFinanceCsvImportAuditParams, *models.FinanceCsvImportAuditResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.GetFinanceCsvImportAuditParams,
	) (*models.FinanceCsvImportAuditResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		item, err := c.deps.CSVImportService.GetCSVImportAudit(
			ctx,
			financepkg.GetCSVImportAuditParams{
				ActorUserID:        userID,
				TenantID:           params.TenantID,
				ImportID:           params.ImportID,
				ExpectedImportType: financepkg.CSVImportTypeTransactions,
			},
		)
		if err != nil {
			return nil, mapCSVImportError(err)
		}

		response := mapCSVImportAudit(item)
		return &response, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) ListRecentFinanceCsvImportAudits(
	builder handlers.HandlerBuilder[
		*models.ListRecentFinanceCsvImportAuditsParams,
		*models.FinanceCsvImportAuditsResponse,
	],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ListRecentFinanceCsvImportAuditsParams,
	) (*models.FinanceCsvImportAuditsResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}
		items, err := c.deps.CSVImportService.ListRecentCSVImportAudits(
			ctx,
			financepkg.ListRecentCSVImportAuditsParams{
				ActorUserID:        userID,
				TenantID:           params.TenantID,
				ExpectedImportType: financepkg.CSVImportTypeTransactions,
			},
		)
		if err != nil {
			return nil, mapCSVImportError(err)
		}
		return mapCSVImportAuditsResponse(items), nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) GetFinanceAccountCsvImportAudit(
	builder handlers.HandlerBuilder[
		*models.GetFinanceAccountCsvImportAuditParams,
		*models.FinanceCsvImportAuditResponse,
	],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.GetFinanceAccountCsvImportAuditParams,
	) (*models.FinanceCsvImportAuditResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}
		item, err := c.deps.CSVImportService.GetCSVImportAudit(
			ctx,
			financepkg.GetCSVImportAuditParams{
				ActorUserID:        userID,
				TenantID:           params.TenantID,
				ImportID:           params.ImportID,
				ExpectedImportType: financepkg.CSVImportTypeAccounts,
			},
		)
		if err != nil {
			return nil, mapCSVImportError(err)
		}
		response := mapCSVImportAudit(item)
		return &response, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) GetFinanceDashboard(
	builder handlers.HandlerBuilder[*models.GetFinanceDashboardParams, *models.FinanceDashboardResponse],
) http.Handler {
	inner := builder.HandleWithHTTP(func(
		_ http.ResponseWriter,
		req *http.Request,
		params *models.GetFinanceDashboardParams,
	) (*models.FinanceDashboardResponse, error) {
		userID, err := operatorUserIDFromContext(req.Context())
		if err != nil {
			return nil, err
		}

		var startDate, endDate time.Time
		parsedStart, startSupplied, err := parseOptionalTimestampQuery(req, "startDate", params.StartDate)
		if err != nil {
			return nil, err
		}
		if startSupplied {
			startDate = parsedStart
		}
		parsedEnd, endSupplied, err := parseOptionalTimestampQuery(req, "endDate", params.EndDate)
		if err != nil {
			return nil, err
		}
		if endSupplied {
			endDate = parsedEnd
		}

		dashboardParams := financepkg.DashboardParams{
			ActorUserID: userID,
			TenantID:    params.TenantID,
			Preset:      financepkg.DashboardPeriodPreset(params.Preset),
			StartDate:   startDate,
			EndDate:     endDate,
		}
		if validationErr := financepkg.ValidateDashboardParams(dashboardParams); validationErr != nil {
			return nil, mapFinanceRangeError(validationErr)
		}

		item, err := c.deps.ReportingService.GetDashboard(
			req.Context(),
			dashboardParams,
		)
		if err != nil {
			return nil, err
		}

		response := mapDashboard(item)
		return &response, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) GetFinanceFxDiagnostics(
	builder handlers.NoParamsHandlerBuilder[*models.FinanceFxDiagnosticsResponse],
) http.Handler {
	inner := builder.HandleWith(
		func(ctx context.Context) (*models.FinanceFxDiagnosticsResponse, error) {
			if _, err := operatorUserIDFromContext(ctx); err != nil {
				return nil, err
			}

			item, err := c.deps.FXService.GetFXAdminDiagnostics(
				ctx,
				financepkg.FXAdminDiagnosticsParams{},
			)
			if err != nil {
				return nil, err
			}

			response := models.FinanceFxDiagnosticsResponse{
				DefaultProvider:  item.DefaultProvider,
				StoredRatesCount: int64(item.StoredRatesCount),
				Providers:        make([]*models.FinanceFxProviderDiagnostic, 0, len(item.Providers)),
			}
			for _, provider := range item.Providers {
				mapped := models.FinanceFxProviderDiagnostic{
					Name:    provider.Name,
					Default: provider.Default,
					Ready:   provider.Ready,
				}
				response.Providers = append(response.Providers, &mapped)
			}

			return &response, nil
		},
	)

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) GetFinanceTenant(
	builder handlers.HandlerBuilder[*models.GetFinanceTenantParams, *models.FinanceTenantSummary],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.GetFinanceTenantParams,
	) (*models.FinanceTenantSummary, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		items, err := c.deps.TenantService.ListTenantsForUser(ctx, userID)
		if err != nil {
			return nil, err
		}

		for _, item := range items {
			if item.Tenant.ID == params.TenantID {
				mapped := mapTenantSummary(item)
				return &mapped, nil
			}
		}

		return nil, app.NewErrNotFound("tenant", params.TenantID)
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) LinkFinanceConnectionToken(
	builder handlers.HandlerBuilder[*models.LinkFinanceConnectionTokenParams, *models.FinanceBankConnection],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.LinkFinanceConnectionTokenParams,
	) (*models.FinanceBankConnection, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		item, err := c.deps.BankConnectionService.LinkTokenBankConnection(
			ctx,
			financepkg.LinkTokenBankConnectionParams{
				ActorUserID: userID,
				TenantID:    params.TenantID,
				Provider:    string(params.Payload.Provider),
				Token:       params.Payload.Token,
			},
		)
		if err != nil {
			return nil, mapBankConnectionError(err, "bank connection link failed")
		}

		mapped := mapConnection(financepkg.BankConnectionView{Connection: item})
		return &mapped, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) FinishFinanceConnectionRedirectLink(
	builder handlers.HandlerBuilder[*models.FinishFinanceConnectionRedirectLinkParams, *models.FinanceBankConnection],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.FinishFinanceConnectionRedirectLinkParams,
	) (*models.FinanceBankConnection, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		item, err := c.deps.BankConnectionService.FinishBankConnectionLink(
			ctx,
			financepkg.FinishBankConnectionLinkParams{
				ActorUserID: userID,
				TenantID:    params.TenantID,
				Provider:    string(params.Payload.Provider),
				State:       params.Payload.State,
				Code:        params.Payload.Code,
			},
		)
		if err != nil {
			return nil, mapBankConnectionError(err, "bank connection link failed")
		}

		mapped := mapConnection(financepkg.BankConnectionView{Connection: item})
		return &mapped, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) GetFinanceSyntheticLinkState(
	builder handlers.HandlerBuilder[*models.GetFinanceSyntheticLinkStateParams, *models.FinanceSyntheticLinkStateResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.GetFinanceSyntheticLinkStateParams,
	) (*models.FinanceSyntheticLinkStateResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		item, err := c.deps.SyntheticLinkStateService.GetPendingSyntheticLinkState(
			ctx,
			financepkg.GetPendingSyntheticLinkStateParams{
				ActorUserID: userID,
				TenantID:    params.TenantID,
				State:       params.State,
			},
		)
		if err != nil {
			return nil, mapSyntheticLinkStateError(err)
		}

		mapped := mapSyntheticLinkState(item)
		return &mapped, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) PutFinanceSyntheticLinkState(
	builder handlers.HandlerBuilder[*models.PutFinanceSyntheticLinkStateParams, *models.FinanceSyntheticLinkStateResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.PutFinanceSyntheticLinkStateParams,
	) (*models.FinanceSyntheticLinkStateResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		item, err := c.deps.SyntheticLinkStateService.SavePendingSyntheticLinkState(
			ctx,
			financepkg.SavePendingSyntheticLinkStateParams{
				ActorUserID:        userID,
				TenantID:           params.TenantID,
				State:              params.State,
				ConfiguredAccounts: mapSyntheticLinkStateAccountsRequest(params.Payload.ConfiguredAccounts),
			},
		)
		if err != nil {
			return nil, mapSyntheticLinkStateError(err)
		}

		mapped := mapSyntheticLinkState(item)
		return &mapped, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) ListFinanceAccounts(
	builder handlers.HandlerBuilder[*models.ListFinanceAccountsParams, *models.FinanceAccountsResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ListFinanceAccountsParams,
	) (*models.FinanceAccountsResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		items, err := c.deps.CatalogService.ListAccounts(
			ctx,
			financepkg.ListAccountsParams{
				ActorUserID:   userID,
				TenantID:      params.TenantID,
				IncludeHidden: params.IncludeHidden,
			},
		)
		if err != nil {
			return nil, err
		}

		return mapAccountsResponse(items)
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) ListFinanceAccountProviderEvidence(
	builder handlers.HandlerBuilder[
		*models.ListFinanceAccountProviderEvidenceParams,
		*models.FinanceProviderEvidenceListResponse,
	],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ListFinanceAccountProviderEvidenceParams,
	) (*models.FinanceProviderEvidenceListResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}
		items, err := c.deps.ProviderEvidenceService.ListAccountProviderEvidence(
			ctx,
			financepkg.ListAccountProviderEvidenceParams{
				ActorUserID: userID,
				TenantID:    params.TenantID,
				AccountID:   params.AccountID,
			},
		)
		if err != nil {
			return nil, err
		}
		return mapProviderEvidenceMetadataResponse(items), nil
	})
	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) GetFinanceAccount(
	builder handlers.HandlerBuilder[*models.GetFinanceAccountParams, *models.FinanceAccount],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.GetFinanceAccountParams,
	) (*models.FinanceAccount, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		item, err := c.deps.CatalogService.GetAccount(
			ctx,
			financepkg.GetAccountParams{
				ActorUserID: userID,
				TenantID:    params.TenantID,
				AccountID:   params.AccountID,
			},
		)
		if err != nil {
			return nil, err
		}

		mapped, err := mapAccount(item)
		if err != nil {
			return nil, err
		}
		return &mapped, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) StartFinanceConnectionRedirectLink(
	builder handlers.HandlerBuilder[*models.StartFinanceConnectionRedirectLinkParams, *models.FinanceConnectionLinkRedirectStartResponse],
) http.Handler {
	inner := builder.HandleWithHTTP(func(
		_ http.ResponseWriter,
		req *http.Request,
		params *models.StartFinanceConnectionRedirectLinkParams,
	) (*models.FinanceConnectionLinkRedirectStartResponse, error) {
		userID, err := operatorUserIDFromContext(req.Context())
		if err != nil {
			return nil, err
		}
		providerRedirectURL, callbackErr := buildFinanceProviderRedirectURL(
			req,
			params.Payload.CallbackUrl,
			c.deps.EnableBankingCallbackBaseURL,
		)
		if callbackErr != nil {
			return nil, callbackErr
		}

		item, err := c.deps.BankConnectionService.StartBankConnectionLink(
			req.Context(),
			financepkg.StartBankConnectionLinkParams{
				ActorUserID:        userID,
				TenantID:           params.TenantID,
				Provider:           string(params.Payload.Provider),
				RedirectURL:        providerRedirectURL,
				BrowserCallbackURL: params.Payload.CallbackUrl,
			},
		)
		if err != nil {
			return nil, mapBankConnectionError(err, "bank connection redirect start failed")
		}

		return &models.FinanceConnectionLinkRedirectStartResponse{
			Provider:         params.Payload.Provider,
			AuthorizationUrl: item.AuthorizationURL,
			State:            item.State,
		}, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func buildFinanceProviderRedirectURL(
	req *http.Request,
	browserCallbackURL string,
	configuredBaseURL string,
) (string, error) {
	if err := ValidateFinanceRedirectCallbackURL(browserCallbackURL); err != nil {
		return "", err
	}
	if override := firstNonEmptyTrimmedString(configuredBaseURL); override != "" {
		return buildEnableBankingCallbackURLFromBase(override)
	}
	host := forwardedRequestHost(req)
	if host == "" {
		return "", app.NewErrInvalidInput("callbackUrl", "request host is required")
	}
	redirectURL := &url.URL{
		Scheme: forwardedRequestScheme(req),
		Host:   host,
		Path:   "/enable-banking/callback",
	}
	return redirectURL.String(), nil
}

const (
	urlSchemeHTTP  = "http"
	urlSchemeHTTPS = "https"
)

func buildEnableBankingCallbackURLFromBase(rawBaseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawBaseURL))
	if err != nil {
		return "", fmt.Errorf("parse enable banking callback base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("enable banking callback base URL must be absolute")
	}
	if parsed.User != nil {
		return "", errors.New("enable banking callback base URL must not include user info")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("enable banking callback base URL must not include query or fragment")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", errors.New("enable banking callback base URL must target the origin root")
	}
	return (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host, Path: "/enable-banking/callback"}).String(), nil
}

func firstNonEmptyTrimmedString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (c *FinanceController) ListFinanceCategories(
	builder handlers.HandlerBuilder[*models.ListFinanceCategoriesParams, *models.FinanceCategoriesResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ListFinanceCategoriesParams,
	) (*models.FinanceCategoriesResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		items, err := c.deps.CatalogService.ListCategories(
			ctx,
			financepkg.ListCategoriesParams{
				ActorUserID:   userID,
				TenantID:      params.TenantID,
				IncludeHidden: params.IncludeHidden,
			},
		)
		if err != nil {
			return nil, err
		}

		return mapCategoriesResponse(items), nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) ListFinanceConnections(
	builder handlers.HandlerBuilder[*models.ListFinanceConnectionsParams, *models.FinanceConnectionsResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ListFinanceConnectionsParams,
	) (*models.FinanceConnectionsResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		items, err := c.deps.BankSyncService.ListBankConnections(
			ctx,
			financepkg.ListBankConnectionsParams{ActorUserID: userID, TenantID: params.TenantID},
		)
		if err != nil {
			return nil, err
		}

		response := models.FinanceConnectionsResponse{
			Items: make([]*models.FinanceBankConnection, 0, len(items)),
		}
		for _, item := range items {
			mapped := mapConnection(item)
			response.Items = append(response.Items, &mapped)
		}

		return &response, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) ListFinanceConnectionSyncedAccounts(
	builder handlers.HandlerBuilder[
		*models.ListFinanceConnectionSyncedAccountsParams,
		*models.FinanceConnectionSyncedAccountsResponse,
	],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ListFinanceConnectionSyncedAccountsParams,
	) (*models.FinanceConnectionSyncedAccountsResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}
		items, err := c.deps.BankSyncService.ListBankConnectionSyncedAccounts(
			ctx,
			financepkg.ListBankConnectionSyncedAccountsParams{
				ActorUserID: userID, TenantID: params.TenantID, ConnectionID: params.ConnectionID,
			},
		)
		if err != nil {
			return nil, mapBankConnectionError(err, "list finance connection synced accounts")
		}
		return mapConnectionSyncedAccountsResponse(items), nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) DeleteFinanceConnection(
	builder handlers.NoResponseHandlerBuilder[*models.DeleteFinanceConnectionParams],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.DeleteFinanceConnectionParams,
	) error {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return err
		}

		return c.deps.BankSyncService.DeleteBankConnection(ctx, financepkg.DeleteBankConnectionParams{
			ActorUserID:  userID,
			TenantID:     params.TenantID,
			ConnectionID: params.ConnectionID,
		})
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) UpdateFinanceConnection(
	builder handlers.NoResponseHandlerBuilder[*models.UpdateFinanceConnectionParams],
) http.Handler {
	inner := builder.HandleWith(func(ctx context.Context, params *models.UpdateFinanceConnectionParams) error {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return err
		}
		err = c.deps.BankConnectionService.UpdateBankConnection(ctx, financepkg.UpdateBankConnectionParams{
			ActorUserID:  userID,
			TenantID:     params.TenantID,
			ConnectionID: params.ConnectionID,
			Name:         params.Payload.Name,
		})
		return mapBankConnectionError(err, "rename finance connection failed")
	})
	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) ListFinanceTags(
	builder handlers.HandlerBuilder[*models.ListFinanceTagsParams, *models.FinanceTagsResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ListFinanceTagsParams,
	) (*models.FinanceTagsResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		items, err := c.deps.CatalogService.ListTags(
			ctx,
			financepkg.ListTagsParams{
				ActorUserID:   userID,
				TenantID:      params.TenantID,
				IncludeHidden: params.IncludeHidden,
			},
		)
		if err != nil {
			return nil, err
		}

		return mapTagsResponse(items), nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) ListFinanceTenantInvites(
	builder handlers.HandlerBuilder[*models.ListFinanceTenantInvitesParams, *models.FinanceTenantInvitesResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ListFinanceTenantInvitesParams,
	) (*models.FinanceTenantInvitesResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		items, err := c.deps.TenantService.ListTenantInvites(
			ctx,
			financepkg.ListTenantInvitesParams{ActorUserID: userID, TenantID: params.TenantID},
		)
		if err != nil {
			return nil, mapCSVImportError(err)
		}

		response := models.FinanceTenantInvitesResponse{
			Items: make([]*models.FinanceTenantInvite, 0, len(items)),
		}
		for _, item := range items {
			mapped := mapTenantInvite(item)
			response.Items = append(response.Items, &mapped)
		}

		return &response, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) ListFinanceTenantMembers(
	builder handlers.HandlerBuilder[*models.ListFinanceTenantMembersParams, *models.FinanceTenantMembersResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ListFinanceTenantMembersParams,
	) (*models.FinanceTenantMembersResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		items, err := c.deps.TenantService.ListTenantMembers(
			ctx,
			financepkg.ListTenantMembersParams{ActorUserID: userID, TenantID: params.TenantID},
		)
		if err != nil {
			return nil, mapCSVImportError(err)
		}

		response := models.FinanceTenantMembersResponse{
			Items: make([]*models.FinanceTenantMember, 0, len(items)),
		}
		for _, item := range items {
			mapped, mapErr := c.mapTenantMember(ctx, item)
			if mapErr != nil {
				return nil, mapErr
			}
			response.Items = append(response.Items, &mapped)
		}

		return &response, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) mapTenantMember(
	ctx context.Context,
	member domain.TenantMember,
) (models.FinanceTenantMember, error) {
	mapped := models.FinanceTenantMember{
		TenantID: member.TenantID,
		UserID:   member.UserID,
		JoinedAt: member.JoinedAt,
	}
	username, found, err := c.deps.UserDirectory.LookupUsername(ctx, member.UserID)
	if err != nil {
		return models.FinanceTenantMember{}, fmt.Errorf("lookup finance member username: %w", err)
	}
	if found {
		mapped.Username = &username
	}
	return mapped, nil
}

func (c *FinanceController) ListFinanceTenants(
	builder handlers.NoParamsHandlerBuilder[*models.FinanceTenantListResponse],
) http.Handler {
	inner := builder.HandleWith(
		func(ctx context.Context) (*models.FinanceTenantListResponse, error) {
			userID, err := operatorUserIDFromContext(ctx)
			if err != nil {
				return nil, err
			}

			items, err := c.deps.TenantService.ListTenantsForUser(ctx, userID)
			if err != nil {
				return nil, err
			}

			response := models.FinanceTenantListResponse{
				Items: make([]*models.FinanceTenantSummary, 0, len(items)),
			}
			for _, item := range items {
				mapped := mapTenantSummary(item)
				response.Items = append(response.Items, &mapped)
			}

			return &response, nil
		},
	)

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) ListFinanceTransactions(
	builder handlers.HandlerBuilder[*models.ListFinanceTransactionsParams, *models.FinanceTransactionsResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ListFinanceTransactionsParams,
	) (*models.FinanceTransactionsResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		items, err := c.deps.LedgerService.ListTransactions(
			ctx,
			financepkg.ListTransactionsParams{
				ActorUserID:   userID,
				TenantID:      params.TenantID,
				AccountID:     params.AccountID,
				Source:        domain.TransactionSource(params.Source),
				Status:        domain.TransactionStatus(params.Status),
				IncludeHidden: params.IncludeHidden,
				Limit:         params.Limit,
				Offset:        params.Offset,
			},
		)
		if err != nil {
			return nil, err
		}

		response := models.FinanceTransactionsResponse{
			Items: make([]*models.FinanceTransaction, 0, len(items)),
		}
		for _, item := range items {
			mapped := mapTransaction(item)
			response.Items = append(response.Items, &mapped)
		}

		return &response, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) ListFinanceTransactionProviderEvidence(
	builder handlers.HandlerBuilder[
		*models.ListFinanceTransactionProviderEvidenceParams,
		*models.FinanceProviderEvidenceListResponse,
	],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ListFinanceTransactionProviderEvidenceParams,
	) (*models.FinanceProviderEvidenceListResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}
		items, err := c.deps.ProviderEvidenceService.ListTransactionProviderEvidence(
			ctx,
			financepkg.ListTransactionProviderEvidenceParams{
				ActorUserID:   userID,
				TenantID:      params.TenantID,
				TransactionID: params.TransactionID,
			},
		)
		if err != nil {
			return nil, err
		}
		return mapProviderEvidenceMetadataResponse(items), nil
	})
	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) UpdateFinanceTransaction(
	builder handlers.HandlerBuilder[*models.UpdateFinanceTransactionParams, *models.FinanceTransaction],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.UpdateFinanceTransactionParams,
	) (*models.FinanceTransaction, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}
		if params.Payload.EffectiveAt != nil {
			validationErr := validateFinanceTimestamp("effectiveAt", *params.Payload.EffectiveAt)
			if validationErr != nil {
				return nil, app.NewErrInvalidInput("effectiveAt", validationErr.Error())
			}
		}
		if params.Payload.ClearCategory && params.Payload.CategoryID != "" {
			return nil, app.NewErrInvalidInput("categoryId", "must be omitted when clearCategory is true")
		}
		if params.Payload.TagIDs == nil {
			return nil, app.NewErrInvalidInput("tagIds", "is required")
		}

		item, err := c.deps.LedgerService.UpdateTransaction(
			ctx,
			financepkg.UpdateTransactionParams{
				ActorUserID:   userID,
				TenantID:      params.TenantID,
				TransactionID: params.TransactionID,
				Description:   params.Payload.Description,
				AmountMinor:   params.Payload.AmountMinor,
				EffectiveAt:   params.Payload.EffectiveAt,
				CategoryID:    params.Payload.CategoryID,
				ClearCategory: params.Payload.ClearCategory,
				TagIDs:        params.Payload.TagIDs,
			},
		)
		if err != nil {
			return nil, mapTransactionTagError(err)
		}

		mapped := mapTransaction(item)
		return &mapped, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) PreviewFinanceCsvImport(
	builder handlers.HandlerBuilder[*models.PreviewFinanceCsvImportParams, *models.FinanceCsvImportPreviewResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.PreviewFinanceCsvImportParams,
	) (*models.FinanceCsvImportPreviewResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		item, err := c.deps.CSVImportService.PreviewCSVImport(
			ctx,
			financepkg.PreviewCSVImportParams{
				ActorUserID:          userID,
				TenantID:             params.TenantID,
				ImportType:           financepkg.CSVImportTypeTransactions,
				FileName:             params.Payload.FileName,
				CSV:                  params.Payload.Csv,
				SelectedAccountNames: params.Payload.SelectedAccountNames,
			},
		)
		if err != nil {
			return nil, mapCSVImportError(err)
		}

		mapped := mapCSVPreview(item)
		return &mapped, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) PreviewFinanceAccountCsvImport(
	builder handlers.HandlerBuilder[
		*models.PreviewFinanceAccountCsvImportParams,
		*models.FinanceAccountCsvImportPreviewResponse,
	],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.PreviewFinanceAccountCsvImportParams,
	) (*models.FinanceAccountCsvImportPreviewResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}
		item, err := c.deps.CSVImportService.PreviewCSVImport(
			ctx,
			financepkg.PreviewCSVImportParams{
				ActorUserID: userID,
				TenantID:    params.TenantID,
				ImportType:  financepkg.CSVImportTypeAccounts,
				FileName:    params.Payload.FileName,
				CSV:         params.Payload.Csv,
			},
		)
		if err != nil {
			return nil, mapCSVImportError(err)
		}
		response := mapAccountCSVPreview(item)
		return &response, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) TriggerFinanceConnectionSync(
	builder handlers.HandlerBuilder[*models.TriggerFinanceConnectionSyncParams, *models.FinanceFxSyncResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.TriggerFinanceConnectionSyncParams,
	) (*models.FinanceFxSyncResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}
		for field, value := range map[string]*time.Time{
			"windowStart": params.Payload.WindowStart,
			"windowEnd":   params.Payload.WindowEnd,
		} {
			if value == nil {
				continue
			}
			if validationErr := validateFinanceTimestamp(field, *value); validationErr != nil {
				return nil, app.NewErrInvalidInput(field, validationErr.Error())
			}
		}

		jobRef, err := c.deps.BankSyncService.TriggerBankConnectionSync(
			ctx,
			financepkg.TriggerBankConnectionSyncParams{
				ActorUserID:  userID,
				TenantID:     params.TenantID,
				ConnectionID: params.ConnectionID,
				Reason:       params.Payload.Reason,
				WindowStart:  params.Payload.WindowStart,
				WindowEnd:    params.Payload.WindowEnd,
			},
		)
		if err != nil {
			return nil, mapBankConnectionError(err, "bank connection sync failed")
		}

		return &models.FinanceFxSyncResponse{JobID: jobRef.ID, JobType: jobRef.JobType}, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) TriggerFinanceFxRefresh(
	builder handlers.HandlerBuilder[*models.TriggerFinanceFxRefreshParams, *models.FinanceFxSyncResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.TriggerFinanceFxRefreshParams,
	) (*models.FinanceFxSyncResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}
		jobRef, err := c.deps.FXService.TriggerFXRefresh(
			ctx,
			financepkg.TriggerFXRefreshParams{
				RequestedByUserID: userID,
				Source:            "operator",
				Provider:          params.Payload.Provider,
			},
		)
		if err != nil {
			return nil, err
		}

		return &models.FinanceFxSyncResponse{
			JobID:    jobRef.ID,
			JobType:  jobRef.JobType,
			Provider: jobRef.Provider,
		}, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func operatorUserIDFromContext(ctx context.Context) (string, error) {
	identity := httpapi.CallerIdentityFromContext(ctx)
	if identity == nil || strings.TrimSpace(identity.UserID()) == "" {
		return "", app.NewErrUnauthorized("unauthorized")
	}

	return identity.UserID(), nil
}

func mapFinanceRangeError(err error) error {
	if errors.Is(err, financepkg.ErrInvalidTimestampRange) ||
		errors.Is(err, financepkg.ErrInvalidDashboardPeriod) {
		return app.NewErrInvalidInput("dateRange", err.Error())
	}
	return err
}

func mapTransactionTagError(err error) error {
	if errors.Is(err, financepkg.ErrDuplicateTagID) || errors.Is(err, financepkg.ErrTagNotAssignable) {
		return fmt.Errorf("%w: %w", app.NewErrInvalidInput("tagIds", err.Error()), err)
	}
	return err
}

func mapCatalogError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, financepkg.ErrTenantAccessDenied):
		return fmt.Errorf("%w: %w", app.NewErrUnauthorized("tenant access denied"), err)
	case errors.Is(err, financepkg.ErrAccountNotFound):
		return fmt.Errorf("%w: %w", app.NewErrNotFound("account", "requested resource"), err)
	case errors.Is(err, financepkg.ErrCategoryNotFound):
		return fmt.Errorf("%w: %w", app.NewErrNotFound("category", "requested resource"), err)
	case errors.Is(err, financepkg.ErrTagNotFound):
		return fmt.Errorf("%w: %w", app.NewErrNotFound("tag", "requested resource"), err)
	case errors.Is(err, financepkg.ErrHiddenAccount):
		return fmt.Errorf("%w: %w", app.NewErrConflict("account", "account is hidden"), err)
	default:
		return err
	}
}

func mapTransferPairError(err error) error {
	if errors.Is(err, financepkg.ErrInvalidTransferPair) {
		return fmt.Errorf("%w: %w", app.NewErrInvalidInput("transferPair", err.Error()), err)
	}
	if errors.Is(err, financepkg.ErrTransferNotLinked) {
		return fmt.Errorf("%w: %w", app.NewErrConflict("transfer pair", err.Error()), err)
	}
	return err
}

func mapTransferDetailError(err error) error {
	switch {
	case errors.Is(err, financepkg.ErrInvalidTransferCandidateQuery):
		return fmt.Errorf("%w: %w", app.NewErrInvalidInput("transferCandidates", err.Error()), err)
	case errors.Is(err, financepkg.ErrTransferPartnerNotFound), errors.Is(err, financepkg.ErrTransactionNotFound):
		return fmt.Errorf("%w: %w", app.NewErrNotFound("transfer partner", "not found"), err)
	case errors.Is(err, financepkg.ErrInvalidTransferPartner):
		return fmt.Errorf("%w: %w", app.NewErrConflict("transfer partner", "unavailable"), err)
	default:
		return err
	}
}

func validateFinanceTimestamp(field string, value time.Time) error {
	if value.IsZero() {
		return fmt.Errorf("%s must be a non-zero timestamp", field)
	}
	return nil
}

func mapCSVImportError(err error) error {
	switch {
	case errors.Is(err, financepkg.ErrTenantAccessDenied):
		return fmt.Errorf("%w: %w", app.NewErrUnauthorized(err.Error()), err)
	case errors.Is(err, financepkg.ErrInvalidTenantDisplayCurrency):
		return fmt.Errorf("%w: %w", app.NewErrInvalidInput("displayCurrency", err.Error()), err)
	case errors.Is(err, financepkg.ErrCSVImportAlreadyConfirmed),
		errors.Is(err, financepkg.ErrCSVImportAlreadyCompleted):
		return fmt.Errorf("%w: %w", app.NewErrConflict("csv import", err.Error()), err)
	case errors.Is(err, financepkg.ErrCSVImportTypeMismatch):
		return fmt.Errorf("%w: %w", app.NewErrNotFound("csv import", "wrong import path"), err)
	case errors.Is(err, financepkg.ErrInvalidCSVImport):
		return app.NewErrInvalidInput(
			"csv",
			strings.TrimPrefix(err.Error(), financepkg.ErrInvalidCSVImport.Error()+": "),
		)
	default:
		return err
	}
}

func mapTenantSummary(
	item domain.TenantMembershipView,
) models.FinanceTenantSummary {
	return models.FinanceTenantSummary{
		ID:              item.Tenant.ID,
		Name:            item.Tenant.Name,
		DisplayCurrency: item.Tenant.DisplayCurrency,
		ArchivedAt:      item.Tenant.ArchivedAt,
		JoinedAt:        item.Membership.JoinedAt,
		CreatedAt:       item.Tenant.CreatedAt,
		UpdatedAt:       item.Tenant.UpdatedAt,
	}
}

func mapTenantInvite(item domain.TenantInvite) models.FinanceTenantInvite {
	return models.FinanceTenantInvite{
		ID:               item.ID,
		TenantID:         item.TenantID,
		Code:             item.Code,
		Recipient:        item.Recipient,
		CreatedByUserID:  item.CreatedByUserID,
		AcceptedByUserID: item.AcceptedByUserID,
		CreatedAt:        item.CreatedAt,
		AcceptedAt:       item.AcceptedAt,
	}
}

func mapAccountsResponse(items []domain.Account) (*models.FinanceAccountsResponse, error) {
	response := models.FinanceAccountsResponse{Items: make([]*models.FinanceAccount, 0, len(items))}
	for _, item := range items {
		mapped, err := mapAccount(item)
		if err != nil {
			return nil, err
		}
		response.Items = append(response.Items, &mapped)
	}

	return &response, nil
}

func mapCategoriesResponse(items []domain.Category) *models.FinanceCategoriesResponse {
	response := models.FinanceCategoriesResponse{
		Items: make([]*models.FinanceCategory, 0, len(items)),
	}
	for _, item := range items {
		mapped := mapCategory(item)
		response.Items = append(response.Items, &mapped)
	}

	return &response
}

func mapTagsResponse(items []domain.Tag) *models.FinanceTagsResponse {
	response := models.FinanceTagsResponse{Items: make([]*models.FinanceTag, 0, len(items))}
	for _, item := range items {
		mapped := mapTag(item)
		response.Items = append(response.Items, &mapped)
	}

	return &response
}

func mapAccount(item domain.Account) (models.FinanceAccount, error) {
	if err := validateFinanceTimestamp("account.createdAt", item.CreatedAt); err != nil {
		return models.FinanceAccount{}, err
	}
	if err := validateFinanceTimestamp("account.updatedAt", item.UpdatedAt); err != nil {
		return models.FinanceAccount{}, err
	}
	if item.HiddenAt != nil {
		if err := validateFinanceTimestamp("account.hiddenAt", *item.HiddenAt); err != nil {
			return models.FinanceAccount{}, err
		}
	}

	response := models.FinanceAccount{
		ID:                  item.ID,
		TenantID:            item.TenantID,
		Name:                item.Name,
		Currency:            item.Currency,
		Kind:                string(item.Kind),
		BookedBalanceMinor:  item.BookedBalanceMinor,
		PendingBalanceMinor: item.PendingBalanceMinor,
		HiddenAt:            item.HiddenAt,
		CreatedAt:           item.CreatedAt,
		UpdatedAt:           item.UpdatedAt,
	}
	if item.LinkedAccount != nil {
		response.Provider = item.LinkedAccount.Provider
		response.ProviderAccountID = item.LinkedAccount.ProviderAccountID
	}
	return response, nil
}

func mapCategory(item domain.Category) models.FinanceCategory {
	return models.FinanceCategory{
		ID:            item.ID,
		TenantID:      item.TenantID,
		Name:          item.Name,
		Kind:          string(item.Kind),
		SeededDefault: item.SeededDefault,
		HiddenAt:      item.HiddenAt,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

func mapTag(item domain.Tag) models.FinanceTag {
	return models.FinanceTag{
		ID:        item.ID,
		TenantID:  item.TenantID,
		Name:      item.Name,
		HiddenAt:  item.HiddenAt,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func mapTransaction(item domain.Transaction) models.FinanceTransaction {
	response := models.FinanceTransaction{
		ID:                item.ID,
		TenantID:          item.TenantID,
		AccountID:         item.AccountID,
		Source:            string(item.Source),
		Status:            string(item.Status),
		Kind:              string(item.Kind),
		AmountMinor:       item.AmountMinor,
		Currency:          item.Currency,
		Description:       item.Description,
		EffectiveAt:       item.EffectiveAt,
		CategoryID:        item.CategoryID,
		TagIDs:            nonNilTagIDs(item.TagIDs),
		TransferGroupID:   item.TransferGroupID,
		TransferMatchedAt: item.TransferMatchedAt,
		HiddenAt:          item.HiddenAt,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
	if item.ProviderOriginal != nil {
		response.ProviderOriginal = &models.FinanceTransactionProviderOriginal{
			AmountMinor: item.ProviderOriginal.AmountMinor,
			Currency:    item.ProviderOriginal.Currency,
			Description: item.ProviderOriginal.Description,
			EffectiveAt: item.ProviderOriginal.EffectiveAt,
		}
	}
	return response
}

func mapTransactionsResponse(items []domain.Transaction) *models.FinanceTransactionsResponse {
	response := &models.FinanceTransactionsResponse{Items: make([]*models.FinanceTransaction, 0, len(items))}
	for _, item := range items {
		mapped := mapTransaction(item)
		response.Items = append(response.Items, &mapped)
	}
	return response
}

func mapProviderEvidenceMetadataResponse(
	items []domain.ProviderEvidence,
) *models.FinanceProviderEvidenceListResponse {
	response := models.FinanceProviderEvidenceListResponse{
		Items: make([]*models.FinanceProviderEvidenceMetadata, 0, len(items)),
	}
	for _, item := range items {
		response.Items = append(response.Items, &models.FinanceProviderEvidenceMetadata{
			ID:               item.ID,
			Scope:            string(item.Scope),
			ProviderObjectID: item.ProviderObjectID,
			CapturedAt:       item.CapturedAt,
		})
	}
	return &response
}

func mapProviderEvidence(item domain.ProviderEvidence) (models.FinanceProviderEvidence, error) {
	payload := map[string]any{}
	if err := json.Unmarshal(item.PayloadJSON, &payload); err != nil {
		return models.FinanceProviderEvidence{}, fmt.Errorf("decode provider evidence payload: %w", err)
	}
	return models.FinanceProviderEvidence{
		ID:               item.ID,
		Scope:            string(item.Scope),
		ProviderObjectID: item.ProviderObjectID,
		CapturedAt:       item.CapturedAt,
		Payload:          payload,
	}, nil
}

func nonNilTagIDs(tagIDs []string) []string {
	if tagIDs == nil {
		return []string{}
	}
	return tagIDs
}

func mapConnection(
	item financepkg.BankConnectionView,
) models.FinanceBankConnection {
	response := models.FinanceBankConnection{
		ID:                   item.Connection.ID,
		TenantID:             item.Connection.TenantID,
		Provider:             item.Connection.Provider,
		DisplayName:          item.Connection.DisplayName,
		ProviderReference:    item.Connection.ProviderReference,
		State:                string(item.Connection.State),
		LastSyncJobID:        item.Connection.LastSyncJobID,
		LastSyncStartedAt:    item.Connection.LastSyncStartedAt,
		LastSuccessfulSyncAt: item.Connection.LastSuccessfulSyncAt,
		LastSyncError:        item.Connection.LastSyncError,
		CreatedAt:            item.Connection.CreatedAt,
		UpdatedAt:            item.Connection.UpdatedAt,
	}
	if item.Schedule != nil {
		response.Schedule = &models.FinanceBankConnectionSchedule{
			ConnectionID:    item.Schedule.ConnectionID,
			IntervalSeconds: int64(item.Schedule.Interval.Seconds()),
			NextRunAt:       item.Schedule.NextRunAt,
			LastScheduledAt: item.Schedule.LastScheduledAt,
			LastStartedAt:   item.Schedule.LastStartedAt,
			LastCompletedAt: item.Schedule.LastCompletedAt,
			LastJobID:       item.Schedule.LastJobID,
			Enabled:         item.Schedule.Enabled,
			CreatedAt:       item.Schedule.CreatedAt,
			UpdatedAt:       item.Schedule.UpdatedAt,
		}
	}
	return response
}

func mapConnectionSyncedAccountsResponse(
	items []financepkg.BankConnectionSyncedAccount,
) *models.FinanceConnectionSyncedAccountsResponse {
	response := &models.FinanceConnectionSyncedAccountsResponse{
		Items: make([]*models.FinanceConnectionSyncedAccount, 0, len(items)),
	}
	for _, item := range items {
		response.Items = append(response.Items, &models.FinanceConnectionSyncedAccount{
			FinanceAccountID:     item.FinanceAccountID,
			Name:                 item.Name,
			Currency:             item.Currency,
			LastSuccessfulSyncAt: item.LastSuccessfulSyncAt,
		})
	}
	return response
}

func mapSyntheticLinkState(
	item financepkg.PendingSyntheticLinkState,
) models.FinanceSyntheticLinkStateResponse {
	response := models.FinanceSyntheticLinkStateResponse{
		Provider: item.Provider,
		State:    item.State,
		ConfiguredAccounts: make(
			[]*models.FinanceSyntheticLinkStateConfiguredAccountResponse,
			0,
			len(item.ConfiguredAccounts),
		),
		CanFinish: item.CanFinish,
	}
	for _, account := range item.ConfiguredAccounts {
		mapped := models.FinanceSyntheticLinkStateConfiguredAccountResponse{
			Key:      account.Key,
			Name:     account.Name,
			Currency: account.Currency,
		}
		response.ConfiguredAccounts = append(response.ConfiguredAccounts, &mapped)
	}
	return response
}

func mapSyntheticLinkStateAccountsRequest(
	items []*models.FinanceSyntheticLinkStateConfiguredAccountRequest,
) []financepkg.SyntheticLinkStateAccount {
	result := make([]financepkg.SyntheticLinkStateAccount, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, financepkg.SyntheticLinkStateAccount{
			Key:      item.Key,
			Name:     item.Name,
			Currency: item.Currency,
		})
	}
	return result
}

func mapDashboard(item financepkg.Dashboard) models.FinanceDashboardResponse { //nolint:funlen
	response := models.FinanceDashboardResponse{
		Period: &models.FinanceDashboardPeriod{
			Preset:    string(item.Period.Preset),
			StartDate: item.Period.StartDate,
			EndDate:   item.Period.EndDate,
			Previous: &models.FinanceDashboardPeriodWindow{
				StartDate: item.Period.Previous.StartDate,
				EndDate:   item.Period.Previous.EndDate,
			},
			Next: &models.FinanceDashboardPeriodWindow{
				StartDate: item.Period.Next.StartDate,
				EndDate:   item.Period.Next.EndDate,
			},
		},
		Settled: &models.FinanceDashboardMoneySummary{
			DisplayCurrency:  item.Settled.DisplayCurrency,
			IncomeMinor:      item.Settled.IncomeMinor,
			ExpenseMinor:     item.Settled.ExpenseMinor,
			NetMinor:         item.Settled.NetMinor,
			TransactionCount: int64(item.Settled.TransactionCount),
			Complete:         item.Settled.Complete,
		},
		Pending: &models.FinanceDashboardMoneySummary{
			DisplayCurrency:  item.Pending.DisplayCurrency,
			IncomeMinor:      item.Pending.IncomeMinor,
			ExpenseMinor:     item.Pending.ExpenseMinor,
			NetMinor:         item.Pending.NetMinor,
			TransactionCount: int64(item.Pending.TransactionCount),
			Complete:         item.Pending.Complete,
		},
		CategoryBreakdowns: make(
			[]*models.FinanceDashboardCategoryBreakdown,
			0,
			len(item.CategoryBreakdowns),
		),
		AccountBalances: make(
			[]*models.FinanceDashboardAccountBalance,
			0,
			len(item.AccountBalances),
		),
		Alerts:     make([]*models.FinanceDashboardAlert, 0, len(item.Alerts)),
		FxCoverage: make([]*models.FinanceDashboardFxCoverage, 0, len(item.FXCoverage)),
		CurrentFxRates: make(
			[]*models.FinanceDashboardCurrentFxRate,
			0,
			len(item.CurrentFXRates),
		),
		NativeSettledTotals: make(
			[]*models.FinanceDashboardCurrencyTotal,
			0,
			len(item.NativeSettledTotals),
		),
	}
	for _, breakdown := range item.CategoryBreakdowns {
		mapped := models.FinanceDashboardCategoryBreakdown{
			CategoryID:       breakdown.CategoryID,
			CategoryName:     breakdown.CategoryName,
			Kind:             string(breakdown.Kind),
			IncomeMinor:      breakdown.IncomeMinor,
			ExpenseMinor:     breakdown.ExpenseMinor,
			TransactionCount: int64(breakdown.TransactionCount),
		}
		response.CategoryBreakdowns = append(response.CategoryBreakdowns, &mapped)
	}
	for _, balance := range item.AccountBalances {
		mapped := models.FinanceDashboardAccountBalance{
			AccountID:           balance.AccountID,
			AccountName:         balance.AccountName,
			Currency:            balance.Currency,
			NativeBookedMinor:   balance.NativeBookedMinor,
			NativePendingMinor:  balance.NativePendingMinor,
			DisplayBookedMinor:  balance.DisplayBookedMinor,
			DisplayPendingMinor: balance.DisplayPendingMinor,
			MissingFx:           balance.MissingFX,
		}
		response.AccountBalances = append(response.AccountBalances, &mapped)
	}
	for _, alert := range item.Alerts {
		mapped := models.FinanceDashboardAlert{
			Code:     alert.Code,
			Severity: alert.Severity,
			Count:    int64(alert.Count),
		}
		response.Alerts = append(response.Alerts, &mapped)
	}
	for _, coverage := range item.FXCoverage {
		mapped := models.FinanceDashboardFxCoverage{
			Provider:                 coverage.Provider,
			BaseCurrency:             coverage.BaseCurrency,
			QuoteCurrency:            coverage.QuoteCurrency,
			AffectedTransactionCount: int64(coverage.AffectedTransactionCount),
			AffectedAccountCount:     int64(coverage.AffectedAccountCount),
		}
		response.FxCoverage = append(response.FxCoverage, &mapped)
	}
	for _, rate := range item.CurrentFXRates {
		mapped := models.FinanceDashboardCurrentFxRate{
			Provider:                rate.Provider,
			BaseCurrency:            rate.BaseCurrency,
			QuoteCurrency:           rate.QuoteCurrency,
			EffectiveAt:             rate.EffectiveAt,
			LastSuccessfulRefreshAt: rate.LastSuccessfulRefreshAt,
			Stale:                   rate.Stale,
		}
		response.CurrentFxRates = append(response.CurrentFxRates, &mapped)
	}
	for _, total := range item.NativeSettledTotals {
		mapped := models.FinanceDashboardCurrencyTotal{
			Currency:     total.Currency,
			IncomeMinor:  total.IncomeMinor,
			ExpenseMinor: total.ExpenseMinor,
			NetMinor:     total.NetMinor,
		}
		response.NativeSettledTotals = append(response.NativeSettledTotals, &mapped)
	}
	return response
}

func mapCSVPreview(
	item financepkg.CSVImportPreview,
) models.FinanceCsvImportPreviewResponse {
	return models.FinanceCsvImportPreviewResponse{
		ImportID:              item.ImportID,
		ImportableCount:       int64(item.ImportableCount),
		Headers:               append([]string{}, item.Headers...),
		DuplicateRows:         mapCSVImportDiagnostics(item.DuplicateRows),
		RejectedRows:          mapCSVImportDiagnostics(item.RejectedRows),
		WouldCreateAccounts:   append([]string{}, item.WouldCreateAccounts...),
		WouldCreateCategories: append([]string{}, item.WouldCreateCategories...),
		WouldCreateTags:       append([]string{}, item.WouldCreateTags...),
		AccountOptions:        mapCSVImportAccountOptions(item.AccountOptions),
	}
}

func mapCSVImportAccountOptions(items []financepkg.CSVImportAccountOption) []*models.FinanceCsvImportAccountOption {
	result := make([]*models.FinanceCsvImportAccountOption, 0, len(items))
	for _, item := range items {
		result = append(result, &models.FinanceCsvImportAccountOption{
			Name:           item.Name,
			SourceRowCount: int64(item.SourceRowCount),
			Selected:       item.Selected,
		})
	}
	return result
}

func mapCSVImportDiagnostics(items []financepkg.CSVImportRejectedRow) []*models.FinanceCsvImportRowDiagnostic {
	result := make([]*models.FinanceCsvImportRowDiagnostic, 0, len(items))
	for _, item := range items {
		result = append(result, &models.FinanceCsvImportRowDiagnostic{
			RowNumber: int64(item.RowNumber),
			Field:     item.Field,
			Reason:    item.Reason,
		})
	}
	return result
}

func mapAccountCSVPreview(item financepkg.CSVImportPreview) models.FinanceAccountCsvImportPreviewResponse {
	return models.FinanceAccountCsvImportPreviewResponse{
		ImportID:            item.ImportID,
		Headers:             append([]string{}, item.Headers...),
		RejectedRows:        mapCSVImportDiagnostics(item.RejectedRows),
		WouldCreateAccounts: append([]string{}, item.WouldCreateAccounts...),
	}
}

func mapCSVImportAudit(item financepkg.CSVImportAudit) models.FinanceCsvImportAuditResponse {
	return models.FinanceCsvImportAuditResponse{
		ImportID:          item.ImportID,
		TenantID:          item.TenantID,
		Status:            string(item.Status),
		JobID:             item.JobID,
		ConfirmedByUserID: item.ConfirmedByUserID,
		ImportedCount:     int64(item.ImportedCount),
		CreatedAt:         item.CreatedAt,
		ConfirmedAt:       item.ConfirmedAt,
		CompletedAt:       item.CompletedAt,
		RejectedRows:      mapCSVImportDiagnostics(item.RejectedRows),
		RowOutcomes:       mapCSVImportRowOutcomes(item.RowOutcomes),
	}
}

func mapCSVImportAuditsResponse(items []financepkg.CSVImportAudit) *models.FinanceCsvImportAuditsResponse {
	response := &models.FinanceCsvImportAuditsResponse{
		Items: make([]*models.FinanceCsvImportAuditResponse, 0, len(items)),
	}
	for _, item := range items {
		mapped := mapCSVImportAudit(item)
		response.Items = append(response.Items, &mapped)
	}
	return response
}

func mapCSVImportRowOutcomes(items []domain.CSVImportRowOutcome) []*models.FinanceCsvImportRowOutcome {
	result := make([]*models.FinanceCsvImportRowOutcome, 0, len(items))
	for _, item := range items {
		result = append(result, &models.FinanceCsvImportRowOutcome{
			RowNumber:     int64(item.RowNumber),
			TransactionID: item.TransactionID,
			Status:        string(item.Status),
			Reason:        item.Reason,
			CreatedAt:     item.CreatedAt,
			UpdatedAt:     item.UpdatedAt,
		})
	}
	return result
}

func ValidateFinanceRedirectCallbackURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return app.NewErrInvalidInput("callbackUrl", "must be a valid callback URL")
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return app.NewErrInvalidInput("callbackUrl", "must be an absolute callback URL")
	}
	if parsed.User != nil {
		return app.NewErrInvalidInput("callbackUrl", "must not include user info")
	}
	if parsed.RawQuery != "" {
		return app.NewErrInvalidInput("callbackUrl", "must not include query parameters")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return app.NewErrInvalidInput("callbackUrl", "must target the app origin root")
	}
	if parsed.Fragment != "/finance/connections" {
		return app.NewErrInvalidInput("callbackUrl", "must target /#/finance/connections")
	}

	switch parsed.Scheme {
	case urlSchemeHTTPS:
		return nil
	case urlSchemeHTTP:
		if isLocalCallbackHost(parsed.Hostname()) {
			return nil
		}
		return app.NewErrInvalidInput(
			"callbackUrl",
			"must use https unless the callback host is localhost or loopback",
		)
	default:
		return app.NewErrInvalidInput("callbackUrl", "must use http or https")
	}
}

func forwardedRequestScheme(req *http.Request) string {
	if req == nil {
		return urlSchemeHTTP
	}
	if proto := strings.TrimSpace(req.Header.Get("X-Forwarded-Proto")); proto != "" {
		return strings.ToLower(strings.Split(proto, ",")[0])
	}
	if forwarded := strings.TrimSpace(req.Header.Get("Forwarded")); forwarded != "" {
		for _, part := range strings.Split(forwarded, ";") {
			key, value, found := strings.Cut(strings.TrimSpace(part), "=")
			if found && strings.EqualFold(strings.TrimSpace(key), "proto") {
				return strings.ToLower(strings.Trim(strings.TrimSpace(value), `"`))
			}
		}
	}
	if req.TLS != nil {
		return urlSchemeHTTPS
	}
	if req.URL != nil && req.URL.Scheme != "" {
		return strings.ToLower(req.URL.Scheme)
	}
	return urlSchemeHTTP
}

func forwardedRequestHost(req *http.Request) string {
	if req == nil {
		return ""
	}
	if host := strings.TrimSpace(req.Header.Get("X-Forwarded-Host")); host != "" {
		return strings.Split(host, ",")[0]
	}
	if forwarded := strings.TrimSpace(req.Header.Get("Forwarded")); forwarded != "" {
		for _, part := range strings.Split(forwarded, ";") {
			key, value, found := strings.Cut(strings.TrimSpace(part), "=")
			if found && strings.EqualFold(strings.TrimSpace(key), "host") {
				return strings.Trim(strings.TrimSpace(value), `"`)
			}
		}
	}
	if host := strings.TrimSpace(req.Host); host != "" {
		return host
	}
	if req.URL != nil {
		return strings.TrimSpace(req.URL.Host)
	}
	return ""
}

func isLocalCallbackHost(host string) bool {
	host = strings.TrimSpace(host)
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func mapBankConnectionError(err error, fallback string) error {
	if err == nil {
		return nil
	}

	var invalidInputErr *app.InvalidInputError
	var notFoundErr *app.NotFoundError
	var conflictErr *app.ConflictError
	var unauthorizedErr *app.UnauthorizedError
	var providerResponseErr *financepkg.ProviderResponseError

	switch {
	case errors.As(err, &invalidInputErr),
		errors.As(err, &notFoundErr),
		errors.As(err, &conflictErr),
		errors.As(err, &unauthorizedErr):
		return err
	case errors.Is(err, financepkg.ErrTenantAccessDenied):
		return fmt.Errorf("%w: %w", app.NewErrUnauthorized("tenant access denied"), err)
	case errors.Is(err, financepkg.ErrUnsupportedBankProvider):
		return fmt.Errorf("%w: %w", app.NewErrInvalidInput("provider", "unsupported bank provider"), err)
	case errors.Is(err, financepkg.ErrBankProviderNotConfigured):
		return fmt.Errorf("%w: %w", app.NewErrInvalidInput("provider", "bank provider not configured"), err)
	case errors.Is(err, financepkg.ErrUnsupportedBankLinkingMethod):
		return fmt.Errorf("%w: %w", app.NewErrInvalidInput("provider", "unsupported bank linking method"), err)
	case errors.Is(err, financepkg.ErrPendingBankConnectionLinkStartNotFound):
		return fmt.Errorf(
			"%w: %w",
			app.NewErrInvalidInput("state", "pending bank link start not found or expired"),
			err,
		)
	case errors.Is(err, financepkg.ErrBankConnectionNotFound):
		return fmt.Errorf("%w: %w", app.NewErrNotFound("bank connection", "requested resource"), err)
	case errors.Is(err, financepkg.ErrBankConnectionNameRequired):
		return fmt.Errorf("%w: %w", app.NewErrInvalidInput("name", "bank connection name is required"), err)
	case errors.As(err, &providerResponseErr) && providerResponseErr.IsClientError():
		return fmt.Errorf(
			"%w: %w",
			app.NewErrInvalidInput("provider", humanizeProviderResponseError(providerResponseErr)),
			err,
		)
	default:
		return fmt.Errorf("%s: %w", fallback, err)
	}
}

func mapSyntheticLinkStateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, financepkg.ErrPendingSyntheticLinkStateNotFound) {
		return fmt.Errorf(
			"%w: %w",
			app.NewErrInvalidInput("state", "pending synthetic link state not found or expired"),
			err,
		)
	}
	if errors.Is(err, financepkg.ErrSyntheticConfiguredAccountNameRequired) {
		return fmt.Errorf(
			"%w: %w",
			app.NewErrInvalidInput("configuredAccounts", "configured account name is required"),
			err,
		)
	}
	if errors.Is(err, financepkg.ErrSyntheticConfiguredAccountCurrencyRequired) {
		return fmt.Errorf(
			"%w: %w",
			app.NewErrInvalidInput("configuredAccounts", "configured account currency is required"),
			err,
		)
	}
	return mapBankConnectionError(err, "synthetic link state failed")
}

func humanizeProviderResponseError(err *financepkg.ProviderResponseError) string {
	if err == nil {
		return providerRequestFailedMessage
	}
	if err.IsEnableBankingWrongASPSP() {
		return "Enable Banking rejected the configured ASPSP name; this sandbox app may not expose PKO, so discover an available ASPSP and set finance.providers.enableBanking.aspspName (for example via APP_FINANCE_PROVIDERS_ENABLEBANKING_ASPSPNAME=Mock ASPSP)"
	}
	return err.Message
}
