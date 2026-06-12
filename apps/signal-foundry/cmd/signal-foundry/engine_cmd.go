package main

import (
	"fmt"

	signalfoundry "github.com/gemyago/signal-foundry/apps/signal-foundry"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal"
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

func newEngineFromRoot(root *cobra.Command, container *dig.Container) (*signalfoundry.Engine, error) {
	fs := root.PersistentFlags()
	jsonLogs, err := fs.GetBool("json-logs")
	if err != nil {
		return nil, fmt.Errorf("json-logs: %w", err)
	}
	logsFile, err := fs.GetString("logs-file")
	if err != nil {
		return nil, fmt.Errorf("logs-file: %w", err)
	}
	defaultLogLevel, err := fs.GetString("log-level")
	if err != nil {
		return nil, fmt.Errorf("log-level: %w", err)
	}
	env, err := fs.GetString("env")
	if err != nil {
		return nil, fmt.Errorf("env: %w", err)
	}

	opts := []signalfoundry.EngineOpt{
		signalfoundry.WithEngineLogsFormatJSON(jsonLogs),
		signalfoundry.WithEngineLogsOutputFile(logsFile),
		signalfoundry.WithEngineDefaultLogLevel(defaultLogLevel),
		internal.WithEngineContainer(container),
	}
	if env != "" {
		opts = append(opts, signalfoundry.WithEngineEnv(env))
	}
	return signalfoundry.NewEngine(opts...)
}
