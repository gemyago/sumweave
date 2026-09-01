package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	sumweave "github.com/gemyago/sumweave/apps/sumweave"
	"github.com/spf13/cobra"
)

const startCommandName = "start"

func setupCommands() *cobra.Command {
	rootCmd := newRootCmd()
	rootCmd.PersistentPreRunE = func(activeCmd *cobra.Command, _ []string) error {
		setPerCommandDefaults(activeCmd)
		return nil
	}
	rootCmd.AddCommand(
		newStartServerCmd(),
		newStartAllCmd(),
		newDatabaseMigrateCmd(),
		newJobsCmd(),
		newFinanceCmd(financeFixturesCommandDeps{
			ResolveRuntimeConfig: func(cmd *cobra.Command) (financeFixturesRuntimeConfig, error) {
				return resolveFinanceFixturesRuntimeConfig(cmd.Root())
			},
		}),
		newFinancePOCCmd(financePOCCommandDeps{}),
		newUserCmd(),
	)
	return rootCmd
}

func newRootCmd() *cobra.Command {
	logsOutputFile := ""

	cmd := &cobra.Command{
		Use:   "sumweave",
		Short: "Sumweave application (HTTP server and related commands)",
	}
	cmd.SilenceUsage = true
	cmd.PersistentFlags().StringP("log-level", "l", "", "Produce logs with given level. Default is env specific.")
	cmd.PersistentFlags().StringVar(
		&logsOutputFile,
		"logs-file",
		"",
		"Produce logs to file instead of stdout. Used for tests only.",
	)
	cmd.PersistentFlags().Bool(
		"json-logs",
		false,
		"Indicates if logs should be in JSON format or text (default)",
	)
	cmd.PersistentFlags().StringP(
		"env",
		"e",
		"",
		"Env that the process is running in.",
	)
	return cmd
}

// This will set default values per command.
func setPerCommandDefaults(_ *cobra.Command) {
	// Reserved for future per-command defaults.
}

type startServerParams struct {
	noop bool
}

func newStartServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   startCommandName,
		Short: "Start the HTTP server",
	}
	params := startServerParams{
		noop: false,
	}
	cmd.Flags().BoolVar(
		&params.noop,
		"noop",
		params.noop,
		"Do not start. Just setup params and exit. Useful for testing if setup is all working.",
	)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		engine, err := newEngineFromRoot(cmd.Root())
		if err != nil {
			return err
		}
		opts := []sumweave.EngineStartServerOpt{
			sumweave.WithStartHTTPServerNoop(params.noop),
		}
		return engine.StartHTTPServer(cmd.Context(), opts...)
	}
	return cmd
}

func main() { // coverage-ignore
	rootCmd := setupCommands()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := rootCmd.ExecuteContext(ctx)
	stop()
	if err != nil {
		os.Exit(1)
	}
}
