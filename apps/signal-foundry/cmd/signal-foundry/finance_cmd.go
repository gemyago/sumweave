package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	financepkg "github.com/gemyago/signal-foundry/finance"
	"github.com/gemyago/signal-foundry/finance/credentials"
	"github.com/gemyago/signal-foundry/finance/domain"
	financefixtures "github.com/gemyago/signal-foundry/finance/fixtures"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/spf13/cobra"
	"go.uber.org/dig"
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
	financeCommandName          = "finance"
	financeFixturesCommandName  = "fixtures"
	financeGenerateCommandName  = "generate"
	realisticScenarioName       = "realistic"
	fixtureScenarioProviderName = "scenario-provider"
	fixtureMonobankProviderName = "monobank"
)

type financeFixturesCommandDeps struct {
	Generate             func(context.Context, financeFixturesRuntimeConfig, financeFixturesGenerateParams) (financefixtures.Summary, error)
	ResolveRuntimeConfig func(*cobra.Command) (financeFixturesRuntimeConfig, error)
	Now                  func() time.Time
}

type financeFixturesRuntimeConfig struct {
	DatabaseDSN     string
	JobsTablePrefix string
	JWTSigningKey   string
	MonobankBaseURL string
}

func newFinanceCmd(deps financeFixturesCommandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: financeCommandName, Short: "Finance utilities"}
	cmd.AddCommand(newFinanceFixturesCmd(deps))
	return cmd
}

func newFinanceFixturesCmd(deps financeFixturesCommandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: financeFixturesCommandName, Short: "Finance fixtures helpers"}
	cmd.AddCommand(newFinanceFixturesGenerateCmd(deps))
	return cmd
}

func newFinanceFixturesGenerateCmd(
	deps financeFixturesCommandDeps,
) *cobra.Command {
	seed := int64(1)
	scenario := realisticScenarioName
	ownerUserID := ""
	memberUserID := ""
	connectionProvider := fixtureScenarioProviderName
	cmd := &cobra.Command{
		Use:   financeGenerateCommandName,
		Short: "Generate realistic finance fixtures",
		RunE: func(cmd *cobra.Command, _ []string) error {
			now := time.Now().UTC()
			if deps.Now != nil {
				now = deps.Now().UTC()
			}
			resolveRuntimeConfig := deps.ResolveRuntimeConfig
			runtimeConfig := financeFixturesRuntimeConfig{}
			if resolveRuntimeConfig != nil {
				var err error
				runtimeConfig, err = resolveRuntimeConfig(cmd)
				if err != nil {
					return err
				}
			}
			generate := deps.Generate
			if generate == nil {
				generate = runFinanceFixturesGenerate
			}
			summary, err := generate(
				cmd.Context(),
				runtimeConfig,
				financeFixturesGenerateParams{
					Seed:               seed,
					Scenario:           scenario,
					Now:                now,
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
	store, err := persistence.NewStore(strings.TrimSpace(runtimeConfig.DatabaseDSN))
	if err != nil {
		return financefixtures.Summary{}, err
	}
	migrateErr := store.Migrate(ctx)
	if migrateErr != nil {
		return financefixtures.Summary{}, migrateErr
	}
	jobsStore, err := jobspkg.NewStore(
		runtimeConfig.DatabaseDSN,
		jobspkg.StoreOpts{TablePrefix: strings.TrimSpace(runtimeConfig.JobsTablePrefix)},
	)
	if err != nil {
		return financefixtures.Summary{}, err
	}
	if autoMigrateErr := jobsStore.AutoMigrate(); autoMigrateErr != nil {
		return financefixtures.Summary{}, autoMigrateErr
	}
	cipherKey := []byte("12345678901234567890123456789012")
	cipherPurpose := "fixtures-cli"
	if jwtKey := strings.TrimSpace(runtimeConfig.JWTSigningKey); jwtKey != "" {
		sum := sha256.Sum256([]byte(jwtKey))
		cipherKey = sum[:]
		cipherPurpose = "signal-foundry-finance"
	}
	cipher, err := credentials.NewAESGCMCipher(cipherKey, cipherPurpose)
	if err != nil {
		return financefixtures.Summary{}, err
	}
	serviceOpts := []financepkg.ServiceOption{
		financepkg.WithNow(func() time.Time { return params.Now }),
		financepkg.WithConnectionSecretCipher(cipher),
		financepkg.WithBankConnectionSyncScheduleWriter(fixturesScheduleWriter{store: jobsStore}),
		financepkg.WithFXProviders(financepkg.NewStaticFXProvider(
			financepkg.FXProviderFrankfurter,
			financefixtures.RealisticScenarioStaticFXRates(financepkg.FXProviderFrankfurter, params.Now),
		)),
	}
	switch strings.TrimSpace(params.ConnectionProvider) {
	case "", fixtureScenarioProviderName:
		serviceOpts = append(serviceOpts, financepkg.WithBankProviders(financeFixturesProvider{}))
	case fixtureMonobankProviderName:
		baseURL := strings.TrimSpace(runtimeConfig.MonobankBaseURL)
		if baseURL == "" {
			return financefixtures.Summary{}, errors.New(
				"finance.providers.monobank.baseURL is required for connection-provider=monobank",
			)
		}
		serviceOpts = append(
			serviceOpts,
			financepkg.WithBankProviders(
				financepkg.NewMonobankProvider(financepkg.MonobankProviderConfig{BaseURL: baseURL}),
			),
		)
	default:
		return financefixtures.Summary{}, fmt.Errorf(
			"unsupported finance fixture connection provider: %s",
			params.ConnectionProvider,
		)
	}
	service := financepkg.NewService(store, serviceOpts...)
	bootstrap := financefixtures.NewBootstrapper(
		financefixtures.NewService(financefixtures.NewPersistenceRepository(store)),
	)
	if params.Scenario != realisticScenarioName {
		return financefixtures.Summary{}, fmt.Errorf(
			"unsupported finance fixture scenario: %s",
			params.Scenario,
		)
	}
	if strings.TrimSpace(params.ConnectionProvider) == fixtureScenarioProviderName {
		params.ConnectionProvider = fixtureMonobankProviderName
	}
	return financefixtures.GenerateRealisticScenario(
		ctx,
		bootstrap,
		service,
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

func resolveFinanceFixturesRuntimeConfig(
	root *cobra.Command,
	container *dig.Container,
) (financeFixturesRuntimeConfig, error) {
	if _, err := newEngineFromRoot(root, container); err != nil {
		return financeFixturesRuntimeConfig{}, err
	}
	type configDeps struct {
		dig.In

		DatabaseDSN     string `name:"config.finance.fixtures.database.dsn"`
		JobsTablePrefix string `name:"config.finance.fixtures.database.jobsTablePrefix"`
		JWTKey          string `name:"config.auth.jwtSigningKey"                        optional:"true"`
		MonoURL         string `name:"config.finance.providers.monobank.baseURL"        optional:"true"`
	}
	var runtimeConfig financeFixturesRuntimeConfig
	if err := container.Invoke(func(deps configDeps) {
		runtimeConfig = financeFixturesRuntimeConfig{
			DatabaseDSN:     deps.DatabaseDSN,
			JobsTablePrefix: deps.JobsTablePrefix,
			JWTSigningKey:   deps.JWTKey,
			MonobankBaseURL: deps.MonoURL,
		}
	}); err != nil {
		return financeFixturesRuntimeConfig{}, fmt.Errorf("resolve finance fixtures config: %w", err)
	}
	return runtimeConfig, nil
}

type fixturesScheduleWriter struct{ store *jobspkg.Store }

func (w fixturesScheduleWriter) UpsertBankConnectionSyncSchedule(
	ctx context.Context,
	schedule financepkg.BankConnectionSyncSchedule,
) error {
	inputJSON := []byte(
		fmt.Sprintf(
			`{"connectionId":%s,"reason":%s}`,
			strconv.Quote(strings.TrimSpace(schedule.ConnectionID)),
			strconv.Quote(financepkg.BankConnectionSyncReasonScheduled),
		),
	)
	var nextRunAt time.Time
	if schedule.Enabled {
		nextRunAt = time.Now().UTC()
	}
	if schedule.NextRunAt != nil {
		nextRunAt = schedule.NextRunAt.UTC()
	}
	return w.store.UpsertSchedule(ctx, jobspkg.Schedule{
		ID:      strings.TrimSpace(schedule.ScheduleID),
		JobType: jobspkg.JobType(financepkg.BankConnectionSyncJobType),
		Requester: jobspkg.Requester{
			UserID: strings.TrimSpace(schedule.ActorUserID),
			Source: jobspkg.RequesterSourceOperator,
		},
		InputJSON: inputJSON,
		Interval:  schedule.Interval,
		NextRunAt: nextRunAt,
		Enabled:   schedule.Enabled,
	})
}

type financeFixturesProvider struct{}

func (financeFixturesProvider) Name() string { return fixtureMonobankProviderName }

func (financeFixturesProvider) StartLink(
	context.Context,
	financepkg.ProviderStartLinkParams,
) (financepkg.ProviderLinkStart, error) {
	return financepkg.ProviderLinkStart{}, nil
}

func (financeFixturesProvider) FinishLink(
	context.Context,
	financepkg.ProviderFinishLinkParams,
) (financepkg.ProviderLinkResult, error) {
	return financepkg.ProviderLinkResult{}, nil
}

func (financeFixturesProvider) LinkToken(
	context.Context,
	financepkg.ProviderTokenLinkParams,
) (financepkg.ProviderTokenLinkResult, error) {
	return financepkg.ProviderTokenLinkResult{
		DisplayName:       "Fixture Connection",
		ProviderReference: "fixture-reference",
		ExternalID:        "fixture-external",
		Secret:            "fixture-secret",
		State:             domain.BankConnectionStateActive,
	}, nil
}

func (financeFixturesProvider) Sync(
	context.Context,
	financepkg.ProviderSyncParams,
) (financepkg.ProviderSyncResult, error) {
	return financepkg.ProviderSyncResult{}, nil
}
