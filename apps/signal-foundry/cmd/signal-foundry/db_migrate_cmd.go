package main

import (
	"context"
	"fmt"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal"
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

const dbMigrateCommandName = "db-migrate"

type databaseMigrationRunner interface {
	Migrate(context.Context) error
}

type databaseMigrationResolver func(*cobra.Command, *dig.Container) (databaseMigrationRunner, error)

func newDatabaseMigrateCmd(container *dig.Container) *cobra.Command {
	return newDatabaseMigrateCmdWithResolver(container, resolveDatabaseMigrator)
}

func newDatabaseMigrateCmdWithResolver(
	container *dig.Container,
	resolver databaseMigrationResolver,
) *cobra.Command {
	return &cobra.Command{
		Use:   dbMigrateCommandName,
		Short: "Run Signal Foundry-managed database schema migrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			migrator, err := resolver(cmd, container)
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

//nolint:ireturn // Cobra command wiring resolves the concrete migrator behind an interface.
func resolveDatabaseMigrator(cmd *cobra.Command, container *dig.Container) (databaseMigrationRunner, error) {
	if _, err := newEngineFromRootWithOpts(
		cmd.Root(),
		container,
		internal.WithEngineJobsWorkerAutoStart(false),
	); err != nil {
		return nil, err
	}

	var migrator *internal.DatabaseMigrator
	if err := container.Invoke(func(resolved *internal.DatabaseMigrator) {
		migrator = resolved
	}); err != nil {
		return nil, fmt.Errorf("resolve database migrator: %w", err)
	}

	return migrator, nil
}
