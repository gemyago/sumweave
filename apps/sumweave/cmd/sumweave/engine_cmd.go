package main

import (
	sumweave "github.com/gemyago/sumweave/apps/sumweave"
	"github.com/spf13/cobra"
)

func newEngineFromRoot(root *cobra.Command) (*sumweave.Engine, error) {
	options, err := engineOptionsFromRoot(root)
	if err != nil {
		return nil, err
	}

	opts := make([]sumweave.EngineOpt, 0, 4)
	if options.JSONLogs != nil {
		opts = append(opts, sumweave.WithEngineLogsFormatJSON(*options.JSONLogs))
	}
	if options.LogsFile != nil {
		opts = append(opts, sumweave.WithEngineLogsOutputFile(*options.LogsFile))
	}
	if options.DefaultLogLevel != nil {
		opts = append(opts, sumweave.WithEngineDefaultLogLevel(*options.DefaultLogLevel))
	}
	if options.Environment != "" {
		opts = append(opts, sumweave.WithEngineEnv(options.Environment))
	}
	return sumweave.NewEngine(opts...)
}

func engineOptionsFromRoot(root *cobra.Command) (commandRootOptions, error) {
	return commandRootOptionsFromRoot(root)
}
