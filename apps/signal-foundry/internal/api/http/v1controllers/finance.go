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

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/middleware"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/v1routes/handlers"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/v1routes/models"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	financepkg "github.com/gemyago/signal-foundry/finance"
	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/runtime/httpapi"
	"go.uber.org/dig"
)

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

type catalogService interface {
	CreateAccount(context.Context, financepkg.CreateAccountParams) (domain.Account, error)
	GetAccount(context.Context, financepkg.GetAccountParams) (domain.Account, error)
	ListAccounts(context.Context, financepkg.ListAccountsParams) ([]domain.Account, error)
	CreateCategory(context.Context, financepkg.CreateCategoryParams) (domain.Category, error)
	ListCategories(context.Context, financepkg.ListCategoriesParams) ([]domain.Category, error)
	CreateTag(context.Context, financepkg.CreateTagParams) (domain.Tag, error)
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
	ListTransactions(
		context.Context,
		financepkg.ListTransactionsParams,
	) ([]domain.Transaction, error)
}

type bankSyncService interface {
	ListBankConnections(
		context.Context,
		financepkg.ListBankConnectionsParams,
	) ([]financepkg.BankConnectionView, error)
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
	TriggerFXSync(context.Context, financepkg.TriggerFXSyncParams) (financepkg.FXSyncJobRef, error)
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
}

type financeService interface {
	tenantService
	catalogService
	ledgerService
	bankSyncService
	reportingService
	fxService
	csvImportService
}

type bankConnectionService interface {
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
	dig.In

	TenantService                tenantService
	CatalogService               catalogService
	LedgerService                ledgerService
	BankSyncService              bankSyncService
	ReportingService             reportingService
	FXService                    fxService
	CSVImportService             csvImportService
	BankConnectionService        bankConnectionService
	SyntheticLinkStateService    syntheticLinkStateService
	AuthMiddleware               middleware.AuthMiddleware
	EnableBankingCallbackBaseURL string `name:"config.finance.providers.enableBanking.callbackBaseURL" optional:"true"`
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

		return &models.FinanceTenantMember{
			TenantID: membership.TenantID,
			UserID:   membership.UserID,
			JoinedAt: membership.JoinedAt.UTC(),
		}, nil
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
				ActorUserID: userID,
				ImportID:    params.ImportID,
				Mapping:     params.Payload.Mapping,
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

		mapped := mapAccount(item)
		return &mapped, nil
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

		tenant, err := c.deps.TenantService.CreateTenant(
			ctx,
			financepkg.CreateTenantParams{
				ActorUserID:     userID,
				Name:            params.Payload.Name,
				DisplayCurrency: string(params.Payload.DisplayCurrency),
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
				TransferGroupID: params.Payload.TransferGroupID,
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
				ActorUserID: userID,
				TenantID:    params.TenantID,
				ImportID:    params.ImportID,
			},
		)
		if err != nil {
			return nil, mapCSVImportError(err)
		}

		return &models.FinanceCsvImportAuditResponse{
			ImportID:          item.ImportID,
			TenantID:          item.TenantID,
			ImportType:        string(item.ImportType),
			Status:            string(item.Status),
			JobID:             item.JobID,
			ConfirmedByUserID: item.ConfirmedByUserID,
			ImportedCount:     int64(item.ImportedCount),
			CreatedAt:         item.CreatedAt.UTC(),
			ConfirmedAt:       timeValueOrZero(item.ConfirmedAt),
			CompletedAt:       timeValueOrZero(item.CompletedAt),
		}, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) GetFinanceDashboard(
	builder handlers.HandlerBuilder[*models.GetFinanceDashboardParams, *map[string]interface{}],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.GetFinanceDashboardParams,
	) (*map[string]interface{}, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		startDate, endDate, err := parseDashboardDateValues(params.StartDate, params.EndDate)
		if err != nil {
			return nil, err
		}

		item, err := c.deps.ReportingService.GetDashboard(
			ctx,
			financepkg.DashboardParams{
				ActorUserID: userID,
				TenantID:    params.TenantID,
				Preset:      financepkg.DashboardPeriodPreset(params.Preset),
				StartDate:   startDate,
				EndDate:     endDate,
			},
		)
		if err != nil {
			return nil, err
		}

		return dashboardResponseMap(item)
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) GetFinanceFxDiagnostics(
	builder handlers.NoParamsHandlerBuilder[*models.FinanceFxDiagnosticsResponse],
) http.Handler {
	inner := builder.HandleWith(func(ctx context.Context) (*models.FinanceFxDiagnosticsResponse, error) {
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
	})

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
			return nil, sanitizeBankConnectionError(err, "bank connection link failed")
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
			return nil, sanitizeBankConnectionError(err, "bank connection link failed")
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

		return mapAccountsResponse(items), nil
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

		mapped := mapAccount(item)
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
			return nil, sanitizeBankConnectionError(err, "bank connection redirect start failed")
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
			mapped := models.FinanceTenantMember{
				TenantID: item.TenantID,
				UserID:   item.UserID,
				JoinedAt: item.JoinedAt.UTC(),
			}
			response.Items = append(response.Items, &mapped)
		}

		return &response, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) ListFinanceTenants(
	builder handlers.NoParamsHandlerBuilder[*models.FinanceTenantListResponse],
) http.Handler {
	inner := builder.HandleWith(func(ctx context.Context) (*models.FinanceTenantListResponse, error) {
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
	})

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

		categoryID := ""
		if params.Payload.CategoryID != nil {
			categoryID = *params.Payload.CategoryID
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
				CategoryID:    categoryID,
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
				ActorUserID: userID,
				TenantID:    params.TenantID,
				ImportType:  financepkg.CSVImportType(params.Payload.ImportType),
				FileName:    params.Payload.FileName,
				CSV:         params.Payload.Csv,
			},
		)
		if err != nil {
			return nil, err
		}

		mapped := mapCSVPreview(item)
		return &mapped, nil
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

		jobRef, err := c.deps.BankSyncService.TriggerBankConnectionSync(
			ctx,
			financepkg.TriggerBankConnectionSyncParams{
				ActorUserID:  userID,
				TenantID:     params.TenantID,
				ConnectionID: params.ConnectionID,
				Reason:       params.Payload.Reason,
				WindowStart:  timePointerOrNil(params.Payload.WindowStart),
				WindowEnd:    timePointerOrNil(params.Payload.WindowEnd),
			},
		)
		if err != nil {
			return nil, sanitizeBankConnectionError(err, "bank connection sync failed")
		}

		return &models.FinanceFxSyncResponse{JobID: jobRef.ID, JobType: jobRef.JobType}, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *FinanceController) TriggerFinanceFxSync(
	builder handlers.HandlerBuilder[*models.TriggerFinanceFxSyncParams, *models.FinanceFxSyncResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.TriggerFinanceFxSyncParams,
	) (*models.FinanceFxSyncResponse, error) {
		userID, err := operatorUserIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		jobRef, err := c.deps.FXService.TriggerFXSync(
			ctx,
			financepkg.TriggerFXSyncParams{
				RequestedByUserID: userID,
				Source:            "operator",
				Provider:          params.Payload.Provider,
				BaseCurrencies:    params.Payload.BaseCurrencies,
				QuoteCurrency:     params.Payload.QuoteCurrency,
				StartDate:         params.Payload.StartDate,
				EndDate:           params.Payload.EndDate,
			},
		)
		if err != nil {
			return nil, err
		}

		return &models.FinanceFxSyncResponse{
			JobID:    jobRef.ID,
			JobType:  jobRef.JobType,
			Provider: params.Payload.Provider,
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

func dashboardResponseMap(item financepkg.Dashboard) (*map[string]interface{}, error) {
	response := mapDashboard(item)
	raw, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("marshal dashboard response: %w", err)
	}

	payload := make(map[string]interface{})
	if unmarshalErr := json.Unmarshal(raw, &payload); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal dashboard response: %w", unmarshalErr)
	}

	return &payload, nil
}

func parseDashboardDates(req *http.Request) (time.Time, time.Time, error) {
	return parseDashboardDateValues(req.URL.Query().Get("startDate"), req.URL.Query().Get("endDate"))
}

func parseDashboardDateValues(startDateRaw string, endDateRaw string) (time.Time, time.Time, error) {
	parseDate := func(key string, value string) (time.Time, error) {
		value = strings.TrimSpace(value)
		if value == "" {
			return time.Time{}, nil
		}
		parsed, err := time.Parse(time.DateOnly, value)
		if err != nil {
			return time.Time{}, app.NewErrInvalidInput(key, err.Error())
		}
		return parsed.UTC(), nil
	}
	startDate, err := parseDate("startDate", startDateRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	endDate, err := parseDate("endDate", endDateRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return startDate, endDate, nil
}

func mapCSVImportError(err error) error {
	switch {
	case errors.Is(err, financepkg.ErrTenantAccessDenied):
		return app.NewErrUnauthorized(err.Error())
	case errors.Is(err, financepkg.ErrInvalidTenantDisplayCurrency):
		return app.NewErrInvalidInput("displayCurrency", err.Error())
	case errors.Is(err, financepkg.ErrCSVImportAlreadyConfirmed),
		errors.Is(err, financepkg.ErrCSVImportAlreadyCompleted):
		return app.NewErrConflict("csv import", err.Error())
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
		ArchivedAt:      timeValueOrZero(item.Tenant.ArchivedAt),
		JoinedAt:        item.Membership.JoinedAt.UTC(),
		CreatedAt:       item.Tenant.CreatedAt.UTC(),
		UpdatedAt:       item.Tenant.UpdatedAt.UTC(),
	}
}

func mapTenantInvite(item domain.TenantInvite) models.FinanceTenantInvite {
	return models.FinanceTenantInvite{
		ID:               item.ID,
		TenantID:         item.TenantID,
		Code:             item.Code,
		Recipient:        item.Recipient,
		CreatedByUserID:  item.CreatedByUserID,
		AcceptedByUserID: stringValueOrZero(item.AcceptedByUserID),
		CreatedAt:        item.CreatedAt.UTC(),
		AcceptedAt:       timeValueOrZero(item.AcceptedAt),
	}
}

func mapAccountsResponse(items []domain.Account) *models.FinanceAccountsResponse {
	response := models.FinanceAccountsResponse{Items: make([]*models.FinanceAccount, 0, len(items))}
	for _, item := range items {
		mapped := mapAccount(item)
		response.Items = append(response.Items, &mapped)
	}

	return &response
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

func mapAccount(item domain.Account) models.FinanceAccount {
	response := models.FinanceAccount{
		ID:                  item.ID,
		TenantID:            item.TenantID,
		Name:                item.Name,
		Currency:            item.Currency,
		Kind:                string(item.Kind),
		BookedBalanceMinor:  item.BookedBalanceMinor,
		PendingBalanceMinor: item.PendingBalanceMinor,
		HiddenAt:            timeValueOrZero(item.HiddenAt),
		CreatedAt:           item.CreatedAt.UTC(),
		UpdatedAt:           item.UpdatedAt.UTC(),
	}
	if item.LinkedAccount != nil {
		response.Provider = item.LinkedAccount.Provider
		response.ProviderAccountID = item.LinkedAccount.ProviderAccountID
	}
	return response
}

func mapCategory(item domain.Category) models.FinanceCategory {
	return models.FinanceCategory{
		ID:            item.ID,
		TenantID:      item.TenantID,
		Name:          item.Name,
		Kind:          string(item.Kind),
		SeededDefault: item.SeededDefault,
		HiddenAt:      timeValueOrZero(item.HiddenAt),
		CreatedAt:     item.CreatedAt.UTC(),
		UpdatedAt:     item.UpdatedAt.UTC(),
	}
}

func mapTag(item domain.Tag) models.FinanceTag {
	return models.FinanceTag{
		ID:        item.ID,
		TenantID:  item.TenantID,
		Name:      item.Name,
		HiddenAt:  timeValueOrZero(item.HiddenAt),
		CreatedAt: item.CreatedAt.UTC(),
		UpdatedAt: item.UpdatedAt.UTC(),
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
		EffectiveAt:       item.EffectiveAt.UTC(),
		CategoryID:        stringValueOrZero(item.CategoryID),
		TransferGroupID:   stringValueOrZero(item.TransferGroupID),
		TransferMatchedAt: timeValueOrZero(item.TransferMatchedAt),
		HiddenAt:          timeValueOrZero(item.HiddenAt),
		CreatedAt:         item.CreatedAt.UTC(),
		UpdatedAt:         item.UpdatedAt.UTC(),
	}
	if item.ProviderOriginal != nil {
		response.ProviderOriginal = &models.FinanceTransactionProviderOriginal{
			AmountMinor: item.ProviderOriginal.AmountMinor,
			Currency:    item.ProviderOriginal.Currency,
			Description: item.ProviderOriginal.Description,
			EffectiveAt: timeValueOrZero(item.ProviderOriginal.EffectiveAt),
		}
	}
	return response
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
		ExternalID:           item.Connection.ExternalID,
		State:                string(item.Connection.State),
		LastSyncJobID:        item.Connection.LastSyncJobID,
		LastSyncStartedAt:    timeValueOrZero(item.Connection.LastSyncStartedAt),
		LastSuccessfulSyncAt: timeValueOrZero(item.Connection.LastSuccessfulSyncAt),
		LastSyncError:        item.Connection.LastSyncError,
		CreatedAt:            item.Connection.CreatedAt.UTC(),
		UpdatedAt:            item.Connection.UpdatedAt.UTC(),
	}
	if item.Schedule != nil {
		response.Schedule = &models.FinanceBankConnectionSchedule{
			ConnectionID:    item.Schedule.ConnectionID,
			IntervalSeconds: int64(item.Schedule.Interval.Seconds()),
			NextRunAt:       timeValueOrZero(item.Schedule.NextRunAt),
			LastScheduledAt: timeValueOrZero(item.Schedule.LastScheduledAt),
			LastStartedAt:   timeValueOrZero(item.Schedule.LastStartedAt),
			LastCompletedAt: timeValueOrZero(item.Schedule.LastCompletedAt),
			LastJobID:       item.Schedule.LastJobID,
			Enabled:         item.Schedule.Enabled,
			CreatedAt:       item.Schedule.CreatedAt.UTC(),
			UpdatedAt:       item.Schedule.UpdatedAt.UTC(),
		}
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
		Period: models.FinanceDashboardPeriod{
			Preset:    string(item.Period.Preset),
			StartDate: item.Period.StartDate.UTC(),
			EndDate:   item.Period.EndDate.UTC(),
			Previous: models.FinanceDashboardPeriodWindow{
				StartDate: item.Period.Previous.StartDate.UTC(),
				EndDate:   item.Period.Previous.EndDate.UTC(),
			},
			Next: models.FinanceDashboardPeriodWindow{
				StartDate: item.Period.Next.StartDate.UTC(),
				EndDate:   item.Period.Next.EndDate.UTC(),
			},
		},
		Settled: models.FinanceDashboardMoneySummary{
			DisplayCurrency:  item.Settled.DisplayCurrency,
			IncomeMinor:      item.Settled.IncomeMinor,
			ExpenseMinor:     item.Settled.ExpenseMinor,
			NetMinor:         item.Settled.NetMinor,
			TransactionCount: item.Settled.TransactionCount,
			Complete:         item.Settled.Complete,
		},
		Pending: models.FinanceDashboardMoneySummary{
			DisplayCurrency:  item.Pending.DisplayCurrency,
			IncomeMinor:      item.Pending.IncomeMinor,
			ExpenseMinor:     item.Pending.ExpenseMinor,
			NetMinor:         item.Pending.NetMinor,
			TransactionCount: item.Pending.TransactionCount,
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
		Alerts:    make([]*models.FinanceDashboardAlert, 0, len(item.Alerts)),
		MissingFx: make([]*models.FinanceDashboardMissingFx, 0, len(item.MissingFX)),
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
			TransactionCount: breakdown.TransactionCount,
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
			Count:    alert.Count,
		}
		response.Alerts = append(response.Alerts, &mapped)
	}
	for _, missing := range item.MissingFX {
		mapped := models.FinanceDashboardMissingFx{
			Source:        string(missing.Source),
			TransactionID: missing.TransactionID,
			AccountID:     missing.AccountID,
			BaseCurrency:  missing.BaseCurrency,
			QuoteCurrency: missing.QuoteCurrency,
			RateDate:      missing.RateDate.UTC(),
			Provider:      missing.Provider,
		}
		response.MissingFx = append(response.MissingFx, &mapped)
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
	response := models.FinanceCsvImportPreviewResponse{
		ImportID:              item.ImportID,
		ImportType:            string(item.ImportType),
		Headers:               append([]string{}, item.Headers...),
		Mapping:               item.Mapping,
		DuplicateRows:         make([]map[string]interface{}, 0, len(item.DuplicateRows)),
		RejectedRows:          make([]map[string]interface{}, 0, len(item.RejectedRows)),
		WouldCreateAccounts:   append([]string{}, item.WouldCreateAccounts...),
		WouldCreateCategories: append([]string{}, item.WouldCreateCategories...),
		WouldCreateTags:       append([]string{}, item.WouldCreateTags...),
	}
	for _, row := range item.DuplicateRows {
		response.DuplicateRows = append(response.DuplicateRows, map[string]interface{}{
			"rowNumber": row.RowNumber,
			"reason":    row.Reason,
		})
	}
	for _, row := range item.RejectedRows {
		response.RejectedRows = append(response.RejectedRows, map[string]interface{}{
			"rowNumber": row.RowNumber,
			"reason":    row.Reason,
		})
	}
	return response
}

func stringValueOrZero(val *string) string {
	if val == nil {
		return ""
	}

	return *val
}

func timeValueOrZero(val *time.Time) time.Time {
	if val == nil {
		return time.Time{}
	}

	return val.UTC()
}

func timePointerOrNil(val time.Time) *time.Time {
	if val.IsZero() {
		return nil
	}

	utc := val.UTC()
	return &utc
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

func sanitizeBankConnectionError(err error, fallback string) error {
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
		return app.NewErrUnauthorized("tenant access denied")
	case errors.Is(err, financepkg.ErrUnsupportedBankProvider):
		return app.NewErrInvalidInput("provider", "unsupported bank provider")
	case errors.Is(err, financepkg.ErrBankProviderNotConfigured):
		return app.NewErrInvalidInput("provider", "bank provider not configured")
	case errors.Is(err, financepkg.ErrUnsupportedBankLinkingMethod):
		return app.NewErrInvalidInput("provider", "unsupported bank linking method")
	case errors.Is(err, financepkg.ErrPendingBankConnectionLinkStartNotFound):
		return app.NewErrInvalidInput("state", "pending bank link start not found or expired")
	case errors.Is(err, financepkg.ErrBankConnectionNotFound):
		return app.NewErrNotFound("bank connection", "requested resource")
	case errors.As(err, &providerResponseErr) && providerResponseErr.IsClientError():
		return app.NewErrInvalidInput("provider", humanizeProviderResponseError(providerResponseErr))
	default:
		return errors.New(fallback)
	}
}

func mapSyntheticLinkStateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, financepkg.ErrPendingSyntheticLinkStateNotFound) {
		return app.NewErrInvalidInput("state", "pending synthetic link state not found or expired")
	}
	if errors.Is(err, financepkg.ErrSyntheticConfiguredAccountNameRequired) {
		return app.NewErrInvalidInput("configuredAccounts", "configured account name is required")
	}
	if errors.Is(err, financepkg.ErrSyntheticConfiguredAccountCurrencyRequired) {
		return app.NewErrInvalidInput("configuredAccounts", "configured account currency is required")
	}
	return sanitizeBankConnectionError(err, "synthetic link state failed")
}

func humanizeProviderResponseError(err *financepkg.ProviderResponseError) string {
	if err == nil {
		return "provider request failed"
	}
	if err.IsEnableBankingWrongASPSP() {
		return "Enable Banking rejected the configured ASPSP name; this sandbox app may not expose PKO, so discover an available ASPSP and set finance.providers.enableBanking.aspspName (for example via APP_FINANCE_PROVIDERS_ENABLEBANKING_ASPSPNAME=Mock ASPSP)"
	}
	return err.Message
}
