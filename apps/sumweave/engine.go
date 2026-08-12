package sumweave

import (
	"context"
	"errors"
	"fmt"

	"github.com/gemyago/sumweave/apps/sumweave/internal/wireup"
	"github.com/gemyago/sumweave/runtime/agent"
)

type Engine struct {
	httpRoot *wireup.HTTPRoot
}

func NewEngine(opts ...EngineOpt) (*Engine, error) {
	o := &engineConfig{}
	o.Apply(opts...)
	root, err := wireup.BuildHTTP(context.Background(), wireup.HTTPOptions{
		Environment: o.environment, DefaultLogLevel: o.defaultLogLevel, JSONLogs: o.jsonLogs, LogsFile: o.logsFile,
	})
	if err != nil {
		return nil, fmt.Errorf("build HTTP engine: %w", err)
	}
	return &Engine{httpRoot: root}, nil
}

func (e *Engine) GetToolsRegistry() (*agent.ToolsRegistry, error) {
	if e == nil || e.httpRoot == nil || e.httpRoot.ToolsRegistry == nil {
		return nil, errors.New("engine tools registry is unavailable")
	}
	return e.httpRoot.ToolsRegistry, nil
}

func (e *Engine) GetAgentRunner() (*agent.Runner, error) {
	if e == nil || e.httpRoot == nil || e.httpRoot.Runner == nil {
		return nil, errors.New("engine agent runner is unavailable")
	}
	return e.httpRoot.Runner, nil
}

// Close releases resources owned by the eagerly constructed Engine. It is safe
// to call after StartHTTPServer returns and may be called more than once.
func (e *Engine) Close(ctx context.Context) error {
	if e == nil || e.httpRoot == nil {
		return errors.New("engine HTTP root is unavailable")
	}
	return e.httpRoot.Close(ctx)
}

// StartHTTPServer starts the HTTP server.
// Note: this method is blocking until the HTTP server is stopped.
// Note: this method is supposed to be called only once.
// It will also handle system signals and shutdown the HTTP server gracefully.
func (e *Engine) StartHTTPServer(ctx context.Context, opts ...EngineStartServerOpt) error {
	o := &engineStartServerCfg{
		noop: false,
	}
	for _, opt := range opts {
		opt.apply(o)
	}

	if e == nil || e.httpRoot == nil {
		return errors.New("engine HTTP root is unavailable")
	}
	return e.httpRoot.StartHTTPServer(ctx, o.noop)
}
