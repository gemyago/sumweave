package main

import (
	"context"
	"fmt"

	"github.com/gemyago/sumweave/apps/sumweave/internal/wireup"
	"github.com/spf13/cobra"
)

const dbMigrateCommandName = "db-migrate"

type databaseMigrationRunner interface {
	Migrate(context.Context) error
}

type databaseMigrationResolver func(*cobra.Command) (databaseMigrationRunner, error)

func newDatabaseMigrateCmd() *cobra.Command {
	return newDatabaseMigrateCmdWithResolver(resolveDatabaseMigrator)
}

func newDatabaseMigrateCmdWithResolver(
	resolver databaseMigrationResolver,
) *cobra.Command {
	return &cobra.Command{
		Use:   dbMigrateCommandName,
		Short: "Run Sumweave-managed database schema migrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			migrator, err := resolver(cmd)
			if err != nil {
				return err
			}
			if err = migrator.Migrate(cmd.Context()); err != nil {
				return fmt.Errorf("run database migrations: %w", err)
			}
			return nil
		},
	}
}

//nolint:ireturn // Cobra command wiring keeps the narrow migration runner seam.
func resolveDatabaseMigrator(cmd *cobra.Command) (databaseMigrationRunner, error) {
	options, err := migrationOptionsFromRoot(cmd.Root())
	if err != nil {
		return nil, err
	}
	root, err := wireup.BuildMigration(cmd.Context(), options)
	if err != nil {
		return nil, fmt.Errorf("build database migration root: %w", err)
	}
	return root, nil
}

type commandRootOptions struct {
	Environment     string
	DefaultLogLevel *string
	JSONLogs        *bool
	LogsFile        *string
}

func commandRootOptionsFromRoot(root *cobra.Command) (commandRootOptions, error) {
	environment, err := commandEnvironmentFromRoot(root)
	if err != nil {
		return commandRootOptions{}, err
	}
	options := commandRootOptions{Environment: environment}
	flags := root.PersistentFlags()

	if flag := flags.Lookup("log-level"); flag != nil && flag.Changed {
		value, getErr := flags.GetString("log-level")
		if getErr != nil {
			return commandRootOptions{}, fmt.Errorf("log-level: %w", getErr)
		}
		options.DefaultLogLevel = &value
	}
	if flag := flags.Lookup("json-logs"); flag != nil && flag.Changed {
		value, getErr := flags.GetBool("json-logs")
		if getErr != nil {
			return commandRootOptions{}, fmt.Errorf("json-logs: %w", getErr)
		}
		options.JSONLogs = &value
	}
	if flag := flags.Lookup("logs-file"); flag != nil && flag.Changed {
		value, getErr := flags.GetString("logs-file")
		if getErr != nil {
			return commandRootOptions{}, fmt.Errorf("logs-file: %w", getErr)
		}
		options.LogsFile = &value
	}

	return options, nil
}

func commandEnvironmentFromRoot(root *cobra.Command) (string, error) {
	environment, err := root.PersistentFlags().GetString("env")
	if err != nil {
		return "", fmt.Errorf("env: %w", err)
	}
	return environment, nil
}

func migrationOptionsFromRoot(root *cobra.Command) (wireup.MigrationOptions, error) {
	options, err := commandRootOptionsFromRoot(root)
	if err != nil {
		return wireup.MigrationOptions{}, err
	}
	return wireup.MigrationOptions{
		Environment: options.Environment, DefaultLogLevel: options.DefaultLogLevel,
		JSONLogs: options.JSONLogs, LogsFile: options.LogsFile,
	}, nil
}
