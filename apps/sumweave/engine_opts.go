package sumweave

type engineConfig struct {
	environment     string
	defaultLogLevel *string
	jsonLogs        *bool
	logsFile        *string
}

func (cfg *engineConfig) Apply(opts ...EngineOpt) {
	for _, opt := range opts {
		opt.apply(cfg)
	}
}

// EngineOpt allows configuring the engine configuration.
type EngineOpt interface{ apply(*engineConfig) }

type engineOptFunc func(*engineConfig)

func (f engineOptFunc) apply(cfg *engineConfig) { f(cfg) }

// WithEngineLogsFormatJSON sets the logs format to JSON.
func WithEngineLogsFormatJSON(logsFormatJSON bool) EngineOpt { //nolint:ireturn
	return engineOptFunc(func(opts *engineConfig) {
		opts.jsonLogs = &logsFormatJSON
	})
}

// WithEngineLogsOutputFile sets the logs output file.
func WithEngineLogsOutputFile(logsOutputFile string) EngineOpt { //nolint:ireturn
	return engineOptFunc(func(opts *engineConfig) {
		opts.logsFile = &logsOutputFile
	})
}

// WithEngineDefaultLogLevel sets the default log level.
// Value must be [slog.Level] string representation.
func WithEngineDefaultLogLevel(defaultLogLevel string) EngineOpt { //nolint:ireturn
	return engineOptFunc(func(opts *engineConfig) {
		opts.defaultLogLevel = &defaultLogLevel
	})
}

// WithEngineEnv sets which embedded config layer loads (e.g. test, local).
func WithEngineEnv(env string) EngineOpt { //nolint:ireturn
	return engineOptFunc(func(opts *engineConfig) {
		opts.environment = env
	})
}

type engineStartServerCfg struct {
	noop bool
}

// EngineStartServerOpt allows configuring the start server operation.
type EngineStartServerOpt interface {
	apply(opts *engineStartServerCfg)
}

type engineStartServerOptFunc func(opts *engineStartServerCfg)

func (f engineStartServerOptFunc) apply(opts *engineStartServerCfg) {
	f(opts)
}

// WithStartHTTPServerNoop sets the noop flag for the HTTP server.
// Useful for testing if dependencies are setup correctly.
func WithStartHTTPServerNoop(noop bool) EngineStartServerOpt { //nolint:ireturn
	return engineStartServerOptFunc(func(opts *engineStartServerCfg) {
		opts.noop = noop
	})
}
