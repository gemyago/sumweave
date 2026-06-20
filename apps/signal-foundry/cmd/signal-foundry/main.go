package main

import (
	"os"

	signalfoundry "github.com/gemyago/signal-foundry/apps/signal-foundry"
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

const startCommandName = "start"

func setupCommands() *cobra.Command {
	container := dig.New()
	rootCmd := newRootCmd()
	rootCmd.PersistentPreRunE = func(activeCmd *cobra.Command, _ []string) error {
		setPerCommandDefaults(activeCmd)
		return nil
	}
	rootCmd.AddCommand(
		newStartServerCmd(container),
		newDataCmd(container, nil),
		newFinancePOCCmd(financePOCCommandDeps{}),
		newUserCmd(container),
	)
	return rootCmd
}

func newRootCmd() *cobra.Command {
	logsOutputFile := ""

	cmd := &cobra.Command{
		Use:   "signal-foundry",
		Short: "Signal Foundry application (HTTP server and related commands)",
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
	noop       bool
	uiLocation string
}

func newStartServerCmd(container *dig.Container) *cobra.Command {
	cmd := &cobra.Command{
		Use:   startCommandName,
		Short: "Start the HTTP server",
	}
	params := startServerParams{
		noop:       false,
		uiLocation: "",
	}
	cmd.Flags().BoolVar(
		&params.noop,
		"noop",
		params.noop,
		"Do not start. Just setup params and exit. Useful for testing if setup is all working.",
	)
	cmd.Flags().StringVar(
		&params.uiLocation,
		"ui-location",
		params.uiLocation,
		"Path to pre-built UI static assets directory. When set, serves UI from this directory. Empty means API-only mode.",
	)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		engine, err := newEngineFromRoot(cmd.Root(), container)
		if err != nil {
			return err
		}
		opts := []signalfoundry.EngineStartServerOpt{
			signalfoundry.WithStartHTTPServerUILocation(params.uiLocation),
			signalfoundry.WithStartHTTPServerNoop(params.noop),
		}
		return engine.StartHTTPServer(cmd.Context(), opts...)
	}
	return cmd
}

func main() { // coverage-ignore
	rootCmd := setupCommands()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
