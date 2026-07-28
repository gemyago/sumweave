package main

import (
	"fmt"

	sumweave "github.com/gemyago/sumweave/apps/sumweave"
	"github.com/gemyago/sumweave/apps/sumweave/internal"
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

func newEngineFromRoot(root *cobra.Command, container *dig.Container) (*sumweave.Engine, error) {
	return newEngineFromRootWithOpts(root, container)
}

func newEngineFromRootWithOpts(
	root *cobra.Command,
	container *dig.Container,
	engineOpts ...sumweave.EngineOpt,
) (*sumweave.Engine, error) {
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

	opts := []sumweave.EngineOpt{
		sumweave.WithEngineLogsFormatJSON(jsonLogs),
		sumweave.WithEngineLogsOutputFile(logsFile),
		sumweave.WithEngineDefaultLogLevel(defaultLogLevel),
		internal.WithEngineContainer(container),
	}
	opts = append(opts, engineOpts...)
	if env != "" {
		opts = append(opts, sumweave.WithEngineEnv(env))
	}
	return sumweave.NewEngine(opts...)
}
