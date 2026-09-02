//go:build postgres_test

package appdispatch

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestRouterRejectsConcurrentRunOnPreparedPostgresTransport(t *testing.T) {
	dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
	require.NotEmpty(t, dsn)
	config := Config{DatabaseDSN: dsn, TablePrefix: "sumweave_", PollInterval: time.Millisecond}
	db, err := sqlconn.Open(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	logger := slog.New(slog.DiscardHandler)
	publisher, err := NewPublisher(config, db, logger)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, publisher.Close()) })
	factory, err := NewRouterFactory(config, db, publisher, logger)
	require.NoError(t, err)
	router, err := factory.NewRouter("concurrent-run-group-" + faker.New().UUID().V4())
	require.NoError(t, err)
	handler, err := NewHandler(testTopic, func(context.Context, Message) error { return nil })
	require.NoError(t, err)
	require.NoError(t, router.Handle(handler))
	firstCtx, cancelFirst := context.WithCancel(t.Context())
	t.Cleanup(cancelFirst)
	firstRun := make(chan error, 1)
	go func() { firstRun <- router.Run(firstCtx) }()
	<-router.router.Running()
	require.ErrorContains(t, router.Run(t.Context()), "run message router")
	cancelFirst()
	<-firstRun
}
