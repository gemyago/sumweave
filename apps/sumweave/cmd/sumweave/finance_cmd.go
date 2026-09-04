package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	jobspkg "github.com/gemyago/sumweave/apps/sumweave/internal/jobs"
	"github.com/gemyago/sumweave/apps/sumweave/internal/wireup"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	financefixtures "github.com/gemyago/sumweave/finance/fixtures"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

type financeFixturesGenerateParams struct {
	Seed               int64
	Scenario           string
	Now                time.Time
	OwnerUserID        string
	MemberUserID       string
	ConnectionProvider string
}

const (
	financeCommandName           = "finance"
	financeFixturesCommandName   = "fixtures"
	financeGenerateCommandName   = "generate"
	realisticScenarioName        = "realistic"
	fixtureScenarioProviderName  = "scenario-provider"
	fixtureMonobankProviderName  = "monobank"
	fixturesEnableBankingBaseURL = "https://example.test"
	fixturesPSUTypePersonal      = "personal"
	fixtureFXBaseCurrency        = "EUR"
	fixtureFXQuoteCurrency       = "USD"
)

type financeFixturesRuntimeConfig struct {
	Database        *persistence.Database
	JobsStore       *jobspkg.Store
	JWTSigningKey   string
	MonobankBaseURL string
	close           func() error
}

func newFinanceCmd() *cobra.Command {
	cmd := &cobra.Command{Use: financeCommandName, Short: "Finance utilities"}
	cmd.AddCommand(newFinanceFixturesCmd())
	return cmd
}

func newFinanceFixturesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: financeFixturesCommandName, Short: "Finance fixtures helpers"}
	cmd.AddCommand(newFinanceFixturesGenerateCmd())
	return cmd
}

func newFinanceFixturesGenerateCmd() *cobra.Command {
	seed := int64(1)
	scenario := realisticScenarioName
	ownerUserID := ""
	memberUserID := ""
	connectionProvider := fixtureScenarioProviderName
	cmd := &cobra.Command{
		Use:   financeGenerateCommandName,
		Short: "Generate realistic finance fixtures",
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			runtimeConfig, err := resolveFinanceFixturesRuntimeConfig(cmd.Root())
			if err != nil {
				return err
			}
			defer func() { err = closeFinanceFixturesRuntimeConfig(err, runtimeConfig) }()
			summary, err := runFinanceFixturesGenerate(
				cmd.Context(),
				runtimeConfig,
				financeFixturesGenerateParams{
					Seed:               seed,
					Scenario:           scenario,
					Now:                time.Now(),
					OwnerUserID:        ownerUserID,
					MemberUserID:       memberUserID,
					ConnectionProvider: connectionProvider,
				},
			)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(summary)
		},
	}
	cmd.Flags().Int64Var(&seed, "seed", seed, "Deterministic fixture seed")
	cmd.Flags().StringVar(&scenario, "scenario", scenario, "Fixture scenario name")
	cmd.Flags().StringVar(
		&ownerUserID,
		"owner-user-id",
		ownerUserID,
		"Optional owner user id override",
	)
	cmd.Flags().StringVar(
		&memberUserID,
		"member-user-id",
		memberUserID,
		"Optional member user id override",
	)
	cmd.Flags().StringVar(
		&connectionProvider,
		"connection-provider",
		connectionProvider,
		"Bank connection provider for realistic fixtures",
	)
	return cmd
}
func runFinanceFixturesGenerate(
	ctx context.Context,
	runtimeConfig financeFixturesRuntimeConfig,
	params financeFixturesGenerateParams,
) (financefixtures.Summary, error) {
	migrateErr := persistence.NewMigrator(runtimeConfig.Database).Migrate(ctx)
	if migrateErr != nil {
		return financefixtures.Summary{}, migrateErr
	}
	store := persistence.NewStore(runtimeConfig.Database)
	if autoMigrateErr := runtimeConfig.JobsStore.AutoMigrate(); autoMigrateErr != nil {
		return financefixtures.Summary{}, autoMigrateErr
	}
	cipherKey := []byte("12345678901234567890123456789012")
	cipherPurpose := "fixtures-cli"
	if jwtKey := strings.TrimSpace(runtimeConfig.JWTSigningKey); jwtKey != "" {
		sum := sha256.Sum256([]byte(jwtKey))
		cipherKey = sum[:]
		cipherPurpose = "sumweave-finance"
	}
	cipher, err := credentials.NewAESGCMCipher(cipherKey, cipherPurpose)
	if err != nil {
		return financefixtures.Summary{}, err
	}
	switch strings.TrimSpace(params.ConnectionProvider) {
	case "", fixtureScenarioProviderName, fixtureMonobankProviderName:
	default:
		return financefixtures.Summary{}, fmt.Errorf(
			"unsupported finance fixture connection provider: %s",
			params.ConnectionProvider,
		)
	}
	if params.Scenario != realisticScenarioName {
		return financefixtures.Summary{}, fmt.Errorf(
			"unsupported finance fixture scenario: %s",
			params.Scenario,
		)
	}
	if strings.TrimSpace(params.ConnectionProvider) == fixtureScenarioProviderName {
		params.ConnectionProvider = fixtureMonobankProviderName
	}
	bootstrap := financefixtures.NewBootstrapper(
		financefixtures.NewService(financefixtures.NewPersistenceRepository(store)),
	)
	monobankServer := newFinanceFixturesMonobankServer()
	defer monobankServer.Close()
	financeModule, err := newFinanceFixturesModule(
		runtimeConfig.Database,
		params,
		cipher,
		monobankServer.Client(),
		monobankServer.URL,
	)
	if err != nil {
		return financefixtures.Summary{}, err
	}
	return financefixtures.GenerateRealisticScenario(
		ctx,
		bootstrap,
		financeFixturesScenarioService{
			tenantService:         financeModule.TenantService,
			catalogService:        financeModule.CatalogService,
			ledgerService:         financeModule.LedgerService,
			csvImportService:      financeModule.CSVImportService,
			bankConnectionService: financeModule.BankConnectionService,
			bankSyncService:       financeModule.BankSyncService,
			fxRefreshService: newFinanceFixturesFXRefreshService(
				persistence.NewCurrentFXRateStore(runtimeConfig.Database),
				financeFixturesFXProvider{now: func() time.Time { return params.Now }},
				func() time.Time { return params.Now },
			),
		},
		financefixtures.Config{
			Seed:               params.Seed,
			Now:                params.Now,
			Scenario:           params.Scenario,
			OwnerUserID:        params.OwnerUserID,
			MemberUserID:       params.MemberUserID,
			ConnectionProvider: params.ConnectionProvider,
		},
	)
}

func newFinanceFixturesModule(
	database *persistence.Database,
	params financeFixturesGenerateParams,
	cipher interface {
		SealString(string) (credentials.Envelope, error)
		OpenString(credentials.Envelope) (string, error)
	},
	httpClient *http.Client,
	monobankBaseURL string,
) (*financepkg.Finance, error) {
	return financepkg.New(&financepkg.Config{
		Database:               database,
		Logger:                 slog.New(slog.DiscardHandler),
		Now:                    func() time.Time { return params.Now },
		NewID:                  uuid.NewString,
		HTTPClient:             httpClient,
		ConnectionSecretCipher: cipher,
		FXProviders: []financepkg.FXRatesProvider{
			financeFixturesFXProvider{now: func() time.Time { return params.Now }},
		},
		DefaultFXProvider: financepkg.FXProviderFrankfurter,
		Monobank: financepkg.MonobankConfig{
			BaseURL: monobankBaseURL,
		},
		EnableBanking: financepkg.EnableBankingConfig{
			BaseURL:        fixturesEnableBankingBaseURL,
			AppID:          "fixtures-app",
			PrivateKeyPath: "fixtures-private-key.pem",
			ASPSPs: []financepkg.EnableBankingASPSP{{
				ProviderID: domain.ProviderIDPKO,
				Name:       "Fixtures Bank",
				Country:    "PL",
				PSUType:    fixturesPSUTypePersonal,
				ValidDays:  90,
			}},
		},
	})
}

type financeFixturesScenarioService struct {
	tenantService         *financepkg.TenantService
	catalogService        *financepkg.CatalogService
	ledgerService         *financepkg.LedgerService
	csvImportService      *financepkg.CSVImportService
	bankConnectionService *financepkg.BankConnectionService
	bankSyncService       *financepkg.BankSyncService
	fxRefreshService      *financepkg.FXService
}

func (s financeFixturesScenarioService) CreateTenant(
	ctx context.Context,
	params financepkg.CreateTenantParams,
) (domain.Tenant, error) {
	return s.tenantService.CreateTenant(ctx, params)
}

func (s financeFixturesScenarioService) CreateTenantInvite(
	ctx context.Context,
	params financepkg.CreateTenantInviteParams,
) (domain.TenantInvite, error) {
	return s.tenantService.CreateTenantInvite(ctx, params)
}

func (s financeFixturesScenarioService) AcceptTenantInvite(
	ctx context.Context,
	params financepkg.AcceptTenantInviteParams,
) (domain.TenantMembership, error) {
	return s.tenantService.AcceptTenantInvite(ctx, params)
}

func (s financeFixturesScenarioService) CreateAccount(
	ctx context.Context,
	params financepkg.CreateAccountParams,
) (domain.Account, error) {
	return s.catalogService.CreateAccount(ctx, params)
}

func (s financeFixturesScenarioService) ListCategories(
	ctx context.Context,
	params financepkg.ListCategoriesParams,
) ([]domain.Category, error) {
	return s.catalogService.ListCategories(ctx, params)
}

func (s financeFixturesScenarioService) ListTags(
	ctx context.Context,
	params financepkg.ListTagsParams,
) ([]domain.Tag, error) {
	return s.catalogService.ListTags(ctx, params)
}

func (s financeFixturesScenarioService) PreviewCSVImport(
	ctx context.Context,
	params financepkg.PreviewCSVImportParams,
) (financepkg.CSVImportPreview, error) {
	return s.csvImportService.PreviewCSVImport(ctx, params)
}

func (s financeFixturesScenarioService) RecordTransaction(
	ctx context.Context,
	params financepkg.RecordTransactionParams,
) (domain.Transaction, error) {
	return s.ledgerService.RecordTransaction(ctx, params)
}

func (s financeFixturesScenarioService) HideTransaction(
	ctx context.Context,
	params financepkg.HideTransactionParams,
) error {
	return s.ledgerService.HideTransaction(ctx, params)
}

func (s financeFixturesScenarioService) LinkTransfers(
	ctx context.Context,
	params financepkg.LinkTransfersParams,
) error {
	return s.ledgerService.LinkTransfers(ctx, params)
}

func (s financeFixturesScenarioService) LinkTokenBankConnection(
	ctx context.Context,
	params financepkg.LinkTokenBankConnectionParams,
) (domain.BankConnection, error) {
	return s.bankConnectionService.LinkTokenBankConnection(ctx, params)
}

func (s financeFixturesScenarioService) UpsertBankConnectionSchedule(
	ctx context.Context,
	params financepkg.UpsertBankConnectionScheduleParams,
) (domain.BankConnectionSchedule, error) {
	return s.bankSyncService.UpsertBankConnectionSchedule(ctx, params)
}

func (s financeFixturesScenarioService) RefreshRequiredFXRates(
	ctx context.Context,
	params financepkg.RefreshFXRatesParams,
) (financepkg.RefreshFXRatesResult, error) {
	return s.fxRefreshService.RefreshRequiredFXRates(ctx, params)
}

func newFinanceFixturesMonobankServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/personal/client-info" {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write([]byte(
			`{"name":"Fixture Connection","accounts":[{"id":"fixture-external","type":"black","currencyCode":980,"balance":101}]}`,
		))
	}))
}

type financeFixturesFXProvider struct{ now func() time.Time }

func (financeFixturesFXProvider) Name() string { return financepkg.FXProviderFrankfurter }

func (p financeFixturesFXProvider) FetchLatestRates(
	ctx context.Context,
	query financepkg.FXProviderQuery,
) ([]domain.FXRate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	baseCurrency := strings.ToUpper(strings.TrimSpace(query.BaseCurrency))
	rates := make([]domain.FXRate, 0, len(query.QuoteCurrencies))
	for _, quoteCurrency := range query.QuoteCurrencies {
		rates = append(rates, domain.FXRate{
			Provider:      p.Name(),
			BaseCurrency:  baseCurrency,
			QuoteCurrency: strings.ToUpper(strings.TrimSpace(quoteCurrency)),
			RateDate:      p.now(),
			Rate:          1.1,
		})
	}
	return rates, nil
}

type financeFixturesFXRateStore struct {
	store *persistence.CurrentFXRateStore
}

func (s financeFixturesFXRateStore) SaveCurrentFXRates(ctx context.Context, rates []domain.FXRate) error {
	return s.store.SaveCurrentFXRatesIfAbsent(ctx, rates)
}

func (s financeFixturesFXRateStore) ListCurrentFXRates(
	ctx context.Context,
	params persistence.ListCurrentFXRatesParams,
) ([]domain.FXRate, error) {
	return s.store.ListCurrentFXRates(ctx, params)
}

type financeFixturesFXPairLister struct{}

func (financeFixturesFXPairLister) ListRequiredFXPairs(context.Context) ([]persistence.RequiredFXPair, error) {
	return []persistence.RequiredFXPair{{
		BaseCurrency: fixtureFXBaseCurrency, QuoteCurrency: fixtureFXQuoteCurrency,
	}}, nil
}

func newFinanceFixturesFXRefreshService(
	rateStore *persistence.CurrentFXRateStore,
	provider financeFixturesFXProvider,
	now func() time.Time,
) *financepkg.FXService {
	return financepkg.NewFXService(
		financeFixturesFXRateStore{store: rateStore},
		financepkg.WithFXServiceNow(now),
		financepkg.WithFXServiceProviders(provider),
		financepkg.WithFXServiceDefaultProvider(provider.Name()),
		financepkg.WithFXServiceRequiredPairs(financeFixturesFXPairLister{}),
	)
}

func resolveFinanceFixturesRuntimeConfig(root *cobra.Command) (financeFixturesRuntimeConfig, error) {
	environment, err := commandEnvironmentFromRoot(root)
	if err != nil {
		return financeFixturesRuntimeConfig{}, err
	}
	fixturesRoot, err := wireup.BuildFinanceFixtures(wireup.FinanceFixturesOptions{
		Environment: environment,
	})
	if err != nil {
		return financeFixturesRuntimeConfig{}, fmt.Errorf("build finance fixtures root: %w", err)
	}
	return financeFixturesRuntimeConfig{
		Database:        fixturesRoot.Database,
		JobsStore:       fixturesRoot.JobsStore,
		JWTSigningKey:   fixturesRoot.JWTSigningKey,
		MonobankBaseURL: fixturesRoot.MonobankBaseURL,
		close:           fixturesRoot.Close,
	}, nil
}

func closeFinanceFixturesRuntimeConfig(commandErr error, runtimeConfig financeFixturesRuntimeConfig) error {
	if runtimeConfig.close == nil {
		return commandErr
	}
	if closeErr := runtimeConfig.close(); closeErr != nil {
		return errors.Join(commandErr, fmt.Errorf("close finance fixtures root: %w", closeErr))
	}
	return commandErr
}
