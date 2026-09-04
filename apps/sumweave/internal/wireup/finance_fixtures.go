package wireup

import (
	"fmt"
	"log/slog"
	"os"

	appinternal "github.com/gemyago/sumweave/apps/sumweave/internal"
	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	jobspkg "github.com/gemyago/sumweave/apps/sumweave/internal/jobs"
	"github.com/gemyago/sumweave/finance/persistence"
)

// FinanceFixturesOptions identifies the configuration layer for fixture generation.
type FinanceFixturesOptions struct {
	Environment string
}

// FinanceFixturesRoot owns the concrete storage capabilities required by one
// finance fixture command. It does not compose HTTP, jobs execution, finance
// application registration, or agent runtime.
type FinanceFixturesRoot struct {
	Database        *persistence.Database
	JobsStore       *jobspkg.Store
	JWTSigningKey   string
	MonobankBaseURL string

	closeDatabase func() error
}

// BuildFinanceFixtures loads typed configuration and constructs only fixture
// storage capabilities.
func BuildFinanceFixtures(
	options FinanceFixturesOptions,
) (*FinanceFixturesRoot, error) { // coverage-ignore // Real command-root behavior is covered directly with isolated storage.
	environment := options.Environment
	if environment == "" {
		environment = localEnvironment
	}
	values, err := config.LoadValues(config.ValuesLoadInput{Environment: environment})
	if err != nil {
		return nil, fmt.Errorf("load finance fixtures configuration: %w", err)
	}
	rootConfig, err := values.FinanceFixturesRoot(environment)
	if err != nil {
		return nil, err
	}

	database, err := appinternal.OpenApplicationSQLDB(rootConfig.Application.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("open finance fixtures application database: %w", err)
	}
	financeDatabase, err := persistence.NewDatabase(
		database,
		rootConfig.Application.Database.DSN,
		persistence.WithLogger(slog.New(slog.NewTextHandler(os.Stdout, nil))),
	)
	if err != nil {
		if closeErr := database.Close(); closeErr != nil {
			return nil, fmt.Errorf(
				"create finance fixtures database: %w; close application database: %w",
				err,
				closeErr,
			)
		}
		return nil, fmt.Errorf("create finance fixtures database: %w", err)
	}
	jobsStore, err := jobspkg.NewStore(database, rootConfig.Application.Database.DSN, jobspkg.StoreOpts{
		TablePrefix: rootConfig.Application.Database.TablePrefix + "jobs_",
	})
	if err != nil {
		if closeErr := database.Close(); closeErr != nil {
			return nil, fmt.Errorf(
				"create finance fixtures jobs store: %w; close application database: %w",
				err,
				closeErr,
			)
		}
		return nil, fmt.Errorf("create finance fixtures jobs store: %w", err)
	}
	return &FinanceFixturesRoot{
		Database:        financeDatabase,
		JobsStore:       jobsStore,
		JWTSigningKey:   rootConfig.Auth.JWTSigningKey,
		MonobankBaseURL: rootConfig.Finance.Providers.Monobank.BaseURL,
		closeDatabase:   database.Close,
	}, nil
}

// Close releases the application database opened for this command root.
func (root *FinanceFixturesRoot) Close() error {
	return root.closeDatabase()
}
