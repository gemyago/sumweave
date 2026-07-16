package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/lifecycle"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/telemetry"
	sloghttp "github.com/samber/slog-http"
	"go.uber.org/dig"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/middleware"
	financepkg "github.com/gemyago/signal-foundry/finance"
)

type HTTPServerDeps struct {
	dig.In `ignore-unexported:"true"`

	// services
	ShutdownHooks *lifecycle.ShutdownHooks

	RootLogger *slog.Logger

	// config
	Host              string        `name:"config.httpServer.host"`
	Port              int           `name:"config.httpServer.port"`
	TLSCertFile       string        `name:"config.httpServer.tls.certFile"`
	TLSKeyFile        string        `name:"config.httpServer.tls.keyFile"`
	IdleTimeout       time.Duration `name:"config.httpServer.idleTimeout"`
	ReadHeaderTimeout time.Duration `name:"config.httpServer.readHeaderTimeout"`
	ReadTimeout       time.Duration `name:"config.httpServer.readTimeout"`
	WriteTimeout      time.Duration `name:"config.httpServer.writeTimeout"`
	AccessLogsLevel   string        `name:"config.httpServer.accessLogsLevel"`

	// handler
	Handler http.Handler

	OTELMiddleware telemetry.OtelHTTPMiddleware

	// listeningSignal is an optional channel that Start will close when the server is listening.
	// Primarily for testing.
	listeningSignal chan struct{}
}

type HTTPServer struct {
	httpSrv *http.Server
	deps    HTTPServerDeps
	logger  *slog.Logger
}

// NewHTTPServer constructor factory for general use [*http.Server].
func NewHTTPServer(deps HTTPServerDeps) *HTTPServer {
	address := fmt.Sprintf("%s:%d", deps.Host, deps.Port)
	srv := &http.Server{
		Addr:              address,
		IdleTimeout:       deps.IdleTimeout,
		ReadHeaderTimeout: deps.ReadHeaderTimeout,
		ReadTimeout:       deps.ReadTimeout,
		WriteTimeout:      deps.WriteTimeout,
		Handler:           deps.Handler,
		ErrorLog:          slog.NewLogLogger(deps.RootLogger.Handler(), slog.LevelError),
	}

	deps.ShutdownHooks.Register("http-server", srv.Shutdown)

	return &HTTPServer{
		deps:    deps,
		httpSrv: srv,
		logger:  deps.RootLogger.WithGroup("http-server"),
	}
}

func (srv *HTTPServer) Start(ctx context.Context) error {
	if err := srv.configureTLS(); err != nil {
		return err
	}

	listenConfig := net.ListenConfig{}
	listener, err := listenConfig.Listen(ctx, "tcp", srv.httpSrv.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", srv.httpSrv.Addr, err)
	}

	actualAddr := listener.Addr().String()
	protocol := "http"
	if srv.httpSrv.TLSConfig != nil {
		protocol = "https"
	}
	srv.logger.InfoContext(ctx, "Started HTTP listener",
		slog.String("addr", actualAddr),
		slog.String("protocol", protocol),
		slog.String("idleTimeout", srv.deps.IdleTimeout.String()),
		slog.String("readHeaderTimeout", srv.deps.ReadHeaderTimeout.String()),
		slog.String("readTimeout", srv.deps.ReadTimeout.String()),
		slog.String("writeTimeout", srv.deps.WriteTimeout.String()),
		slog.String("accessLogsLevel", srv.deps.AccessLogsLevel),
	)

	if srv.deps.listeningSignal != nil {
		close(srv.deps.listeningSignal)
	}

	// http.Server.Serve always returns a non-nil error.
	// It returns http.ErrServerClosed when Shutdown or Close is called.
	if srv.httpSrv.TLSConfig != nil {
		listener = tls.NewListener(listener, srv.httpSrv.TLSConfig)
	}
	err = srv.httpSrv.Serve(listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("%s server Serve error: %w", protocol, err)
	}
	return nil
}

func (srv *HTTPServer) configureTLS() error {
	if srv.deps.TLSCertFile == "" && srv.deps.TLSKeyFile == "" {
		return nil
	}
	if srv.deps.TLSCertFile == "" || srv.deps.TLSKeyFile == "" {
		return errors.New("both HTTP TLS certificate and key files are required")
	}

	certificate, err := tls.LoadX509KeyPair(srv.deps.TLSCertFile, srv.deps.TLSKeyFile)
	if err != nil {
		return fmt.Errorf("load HTTP TLS certificate: %w", err)
	}
	srv.httpSrv.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS12,
	}
	return nil
}

type RouterMiddleware func(http.Handler) http.Handler

const maxFinanceCSVImportRequestBodyBytes = 2*financepkg.MaxCSVImportBytes + 1<<20

type RouterMiddlewareDeps struct {
	dig.In

	RootLogger *slog.Logger

	// config
	AccessLogsLevel string `name:"config.httpServer.accessLogsLevel"`

	OTELMiddleware telemetry.OtelHTTPMiddleware
	IDGen          ident.Generator
}

func NewRouterMiddleware(deps RouterMiddlewareDeps) RouterMiddleware {
	defaultLogLevel := slog.LevelInfo
	clientErrorLevel := slog.LevelWarn
	serverErrorLevel := slog.LevelError

	if deps.AccessLogsLevel != "" {
		if err := defaultLogLevel.UnmarshalText([]byte(deps.AccessLogsLevel)); err != nil {
			panic(fmt.Errorf("failed to unmarshal access logs level: %w", err))
		}
		clientErrorLevel = defaultLogLevel
		serverErrorLevel = defaultLogLevel
	}

	chain := middleware.Chain(
		middleware.Middleware(deps.OTELMiddleware), // otel goes first
		middleware.NewCorrelationMiddleware(deps.IDGen),
		// CSV JSON escaping can double the 64 MiB raw CSV size; reserve 1 MiB for envelope fields.
		middleware.NewRequestBodyLimitMiddleware(maxFinanceCSVImportRequestBodyBytes),
		sloghttp.NewWithConfig(deps.RootLogger, sloghttp.Config{
			DefaultLevel:     defaultLogLevel,
			ClientErrorLevel: clientErrorLevel,
			ServerErrorLevel: serverErrorLevel,

			WithUserAgent:      true,
			WithRequestID:      false, // We handle it ourselves (tracing middleware)
			WithRequestHeader:  true,
			WithResponseHeader: true,

			// Log handler will add those, we don't want them twice
			// see telemetry/slog.go for more details
			WithSpanID:  false,
			WithTraceID: false,
		}),
		middleware.NewRecovererMiddleware(deps.RootLogger),
	)
	return chain
}
