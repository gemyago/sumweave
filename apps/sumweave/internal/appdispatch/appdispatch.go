package appdispatch

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	wmsql "github.com/ThreeDotsLabs/watermill-sql/v4/pkg/sql"
	wmmessage "github.com/ThreeDotsLabs/watermill/message"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
)

const (
	DispatchTopicExecution  = "app.dispatch.execution.v1"
	EnvelopeVersionV1       = "v1"
	sqliteBusyTimeoutMillis = 5000
)

type TransportDriver string

const (
	TransportDriverSQLite   TransportDriver = "sqlite"
	TransportDriverPostgres TransportDriver = "postgres"
)

type ExecutionKind string

type Envelope struct {
	Version           string          `json:"version"`
	Kind              ExecutionKind   `json:"kind"`
	Payload           json.RawMessage `json:"payload"`
	ObservableJobID   string          `json:"observableJobId,omitempty"`
	CorrelationID     string          `json:"correlationId,omitempty"`
	RequesterID       string          `json:"requesterId,omitempty"`
	RequesterSource   string          `json:"requesterSource,omitempty"`
	ScheduleWindowKey string          `json:"scheduleWindowKey,omitempty"`
}

func (e Envelope) Topic() string {
	return DispatchTopicExecution
}

func (e Envelope) validate() error {
	if e.Version != EnvelopeVersionV1 {
		return fmt.Errorf("unsupported envelope version: %s", e.Version)
	}
	if e.Kind == "" {
		return errors.New("execution kind is required")
	}
	if len(e.Payload) == 0 {
		return errors.New("execution payload is required")
	}
	return nil
}

func EncodePayload(value any) ([]byte, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return payload, nil
}

type Config struct {
	DatabaseDSN  string
	TablePrefix  string
	ConsumerName string
	PollInterval time.Duration
}

func (c Config) normalize() Config {
	if c.ConsumerName == "" {
		c.ConsumerName = "sumweave-app-dispatch"
	}
	if c.PollInterval <= 0 {
		c.PollInterval = 100 * time.Millisecond
	}
	return c
}

func (c Config) Driver() TransportDriver {
	parsed, err := url.Parse(c.DatabaseDSN)
	if err == nil {
		switch parsed.Scheme {
		case "postgres", "postgresql":
			return TransportDriverPostgres
		case "file", "sqlite":
			return TransportDriverSQLite
		}
	}
	if strings.HasPrefix(c.DatabaseDSN, "postgres://") || strings.HasPrefix(c.DatabaseDSN, "postgresql://") {
		return TransportDriverPostgres
	}
	return TransportDriverSQLite
}

func (c Config) MessagesTable() string {
	return c.TablePrefix + "app_dispatch_messages"
}

func (c Config) OffsetsTable() string {
	return c.TablePrefix + "app_dispatch_offsets"
}

type registeredHandler interface {
	kind() ExecutionKind
	handle(context.Context, Envelope) error
}

type HandlerRegistry struct {
	mu       sync.RWMutex
	handlers map[ExecutionKind]registeredHandler
}

func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{handlers: map[ExecutionKind]registeredHandler{}}
}

func (r *HandlerRegistry) register(handler registeredHandler) error {
	if handler == nil {
		return errors.New("handler is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[handler.kind()]; exists {
		return fmt.Errorf("handler already registered: %s", handler.kind())
	}
	r.handlers[handler.kind()] = handler
	return nil
}

func (r *HandlerRegistry) Handle(ctx context.Context, envelope Envelope) error {
	if r == nil {
		return errors.New("handler registry is required")
	}
	r.mu.RLock()
	handler, ok := r.handlers[envelope.Kind]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("handler not registered: %s", envelope.Kind)
	}
	return handler.handle(ctx, envelope)
}

type TypedHandlerSpec[Payload any] struct {
	Kind ExecutionKind
	Run  func(context.Context, Envelope, Payload) error
}

func RegisterTypedHandler[Payload any](registry *HandlerRegistry, spec TypedHandlerSpec[Payload]) error {
	if registry == nil {
		return errors.New("handler registry is required")
	}
	if spec.Run == nil {
		return errors.New("handler run func is required")
	}
	return registry.register(typedHandler[Payload]{spec: spec})
}

type typedHandler[Payload any] struct {
	spec TypedHandlerSpec[Payload]
}

func (h typedHandler[Payload]) kind() ExecutionKind {
	return h.spec.Kind
}

func (h typedHandler[Payload]) handle(ctx context.Context, envelope Envelope) error {
	var payload Payload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return fmt.Errorf("decode envelope payload: %w", err)
	}
	return h.spec.Run(ctx, envelope, payload)
}

type Publisher struct {
	config    Config
	db        *sql.DB
	ownsDB    bool
	publisher wmmessage.Publisher
	logger    *slog.Logger
}

func NewPublisher(config Config, db *sql.DB, logger *slog.Logger) (*Publisher, error) {
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	return newPublisher(config, db, false, logger)
}

func newPublisher(config Config, db *sql.DB, ownsDB bool, logger *slog.Logger) (*Publisher, error) {
	config = config.normalize()
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	if db == nil {
		return nil, errors.New("sql database is required")
	}
	logger = logger.WithGroup("appdispatch")
	publisher, err := newMessagePublisher(config, db, logger)
	if err != nil {
		return nil, err
	}
	return &Publisher{config: config, db: db, ownsDB: ownsDB, publisher: publisher, logger: logger}, nil
}

func (p *Publisher) Close() error {
	if p == nil {
		return nil
	}
	var dbErr error
	if p.ownsDB {
		dbErr = closeDB(p.db)
	}
	return errors.Join(closeIfPresent(p.publisher), dbErr)
}

func (p *Publisher) Publish(ctx context.Context, envelope Envelope) error {
	if err := envelope.validate(); err != nil {
		return err
	}
	msg, err := envelopeMessage(ctx, envelope)
	if err != nil {
		return err
	}
	if err = p.publisher.Publish(DispatchTopicExecution, msg); err != nil {
		return fmt.Errorf("publish dispatch envelope: %w", err)
	}
	p.logger.DebugContext(ctx, "message envelope published",
		slog.String("messageId", msg.UUID),
		slog.String("requesterId", envelope.RequesterID),
	)

	return nil
}

func (p *Publisher) PublishInTx(ctx context.Context, tx *sql.Tx, envelope Envelope) error {
	if tx == nil {
		return errors.New("publish transaction is required")
	}
	if err := envelope.validate(); err != nil {
		return err
	}
	publisher, err := newMessagePublisher(p.config, tx, p.logger)
	if err != nil {
		return err
	}
	defer func() { _ = closeIfPresent(publisher) }()
	msg, err := envelopeMessage(ctx, envelope)
	if err != nil {
		return err
	}
	if err = publisher.Publish(DispatchTopicExecution, msg); err != nil {
		return fmt.Errorf("publish dispatch envelope in tx: %w", err)
	}
	p.logger.DebugContext(ctx, "message envelope published in tx",
		slog.String("messageId", msg.UUID),
		slog.String("requesterId", envelope.RequesterID),
	)
	return nil
}

type Consumer struct {
	db         *sql.DB
	ownsDB     bool
	subscriber wmmessage.Subscriber
	registry   *HandlerRegistry
	logger     *slog.Logger
}

func NewConsumer(config Config, db *sql.DB, registry *HandlerRegistry, logger *slog.Logger) (*Consumer, error) {
	if registry == nil {
		return nil, errors.New("handler registry is required")
	}
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	return newConsumer(config, db, false, registry, logger)
}

//nolint:golines // Internal constructor keeps the ownership and dependency shape explicit.
func newConsumer(config Config, db *sql.DB, ownsDB bool, registry *HandlerRegistry, logger *slog.Logger) (*Consumer, error) {
	config = config.normalize()
	if registry == nil {
		return nil, errors.New("handler registry is required")
	}
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	if db == nil {
		return nil, errors.New("sql database is required")
	}
	logger = logger.WithGroup("appdispatch")
	subscriber, err := newMessageSubscriber(config, db, logger)
	if err != nil {
		return nil, err
	}
	return &Consumer{
		db:         db,
		ownsDB:     ownsDB,
		subscriber: subscriber,
		registry:   registry,
		logger:     logger,
	}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	messages, err := c.subscriber.Subscribe(ctx, DispatchTopicExecution)
	if err != nil {
		return fmt.Errorf("subscribe dispatch topic: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-messages:
			if !ok {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return nil
			}
			envelope, decodeErr := decodeEnvelope(msg.Payload)
			if decodeErr != nil {
				msg.Nack()
				return decodeErr
			}
			handlerCtx := context.WithoutCancel(msg.Context())
			msgCtx := telemetry.SetLogAttributesToContext(
				handlerCtx,
				telemetry.LogAttributes{
					CorrelationID: slog.StringValue(msg.UUID),
				},
			)
			c.logger.InfoContext(msgCtx, "processing message",
				slog.String("messageId", msg.UUID),
				slog.String("requesterId", envelope.RequesterID),
				slog.String("MessageCorrelationID", envelope.CorrelationID),
				slog.Any("metadata", msg.Metadata),
			)
			if handleErr := c.registry.Handle(msgCtx, envelope); handleErr != nil {
				msg.Nack()
				return handleErr
			}
			msg.Ack()
		}
	}
}
func (c *Consumer) Close() error {
	if c == nil {
		return nil
	}
	var dbErr error
	if c.ownsDB {
		dbErr = closeDB(c.db)
	}
	return errors.Join(closeIfPresent(c.subscriber), dbErr)
}

//nolint:ireturn // Watermill publisher is defined by the library interface.
func newMessagePublisher(config Config, db any, logger *slog.Logger) (wmmessage.Publisher, error) {
	wmLogger := watermill.NewSlogLogger(logger)
	if config.Driver() == TransportDriverPostgres {
		return wmsql.NewPublisher(asContextExecutor(db), wmsql.PublisherConfig{
			SchemaAdapter:        postgresSchema(config),
			AutoInitializeSchema: false,
		}, wmLogger)
	}
	return newSQLiteTransportPublisher(config, db, wmLogger)
}

//nolint:ireturn // Watermill subscriber is defined by the library interface.
func newMessageSubscriber(config Config, db *sql.DB, logger *slog.Logger) (wmmessage.Subscriber, error) {
	wmLogger := watermill.NewSlogLogger(logger)
	if config.Driver() == TransportDriverPostgres {
		return wmsql.NewSubscriber(wmsql.BeginnerFromStdSQL(db), wmsql.SubscriberConfig{
			ConsumerGroup:    config.ConsumerName,
			PollInterval:     config.PollInterval,
			SchemaAdapter:    postgresSchema(config),
			OffsetsAdapter:   postgresOffsets(config),
			InitializeSchema: false,
		}, wmLogger)
	}
	return newSQLiteTransportSubscriber(config, db, wmLogger)
}

func envelopeMessage(ctx context.Context, envelope Envelope) (*wmmessage.Message, error) {
	payload, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshal dispatch envelope: %w", err)
	}
	return wmmessage.NewMessageWithContext(ctx, watermill.NewUUID(), payload), nil
}

func decodeEnvelope(payload []byte) (Envelope, error) {
	var envelope Envelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return Envelope{}, fmt.Errorf("decode dispatch envelope: %w", err)
	}
	if err := envelope.validate(); err != nil {
		return Envelope{}, err
	}
	return envelope, nil
}

func sqliteTableNameGenerators(config Config) sqliteTableGenerators {
	return sqliteTableGenerators{
		Topic: func(string) string {
			return config.MessagesTable()
		},
		Offsets: func(string) string {
			return config.OffsetsTable()
		},
	}
}

func buildSQLiteMigrationQueries(config Config) []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS ` + quoteIdentifier(config.MessagesTable()) + ` (
			"offset" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
			uuid TEXT NOT NULL,
			created_at TEXT NOT NULL,
			payload BLOB,
			metadata JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS ` + quoteIdentifier(config.OffsetsTable()) + ` (
			consumer_group TEXT NOT NULL,
			offset_acked INTEGER NOT NULL,
			locked_until INTEGER NOT NULL,
			PRIMARY KEY(consumer_group)
		)`,
	}
}

func buildPostgresMigrationQueries(config Config) ([]wmsql.Query, error) {
	queries, err := postgresSchema(config).SchemaInitializingQueries(wmsql.SchemaInitializingQueriesParams{
		Topic: DispatchTopicExecution,
	})
	if err != nil {
		return nil, fmt.Errorf("build postgres messages schema queries: %w", err)
	}
	offsetQueries, err := postgresOffsets(config).SchemaInitializingQueries(
		wmsql.OffsetsSchemaInitializingQueriesParams{Topic: DispatchTopicExecution},
	)
	if err != nil {
		return nil, fmt.Errorf("build postgres offsets schema queries: %w", err)
	}
	return append(queries, offsetQueries...), nil
}

func postgresSchema(config Config) wmsql.DefaultPostgreSQLSchema {
	messagesTable := quoteIdentifier(config.MessagesTable())
	return wmsql.DefaultPostgreSQLSchema{
		GenerateMessagesTableName: func(string) string { return messagesTable },
	}
}

func postgresOffsets(config Config) wmsql.DefaultPostgreSQLOffsetsAdapter {
	offsetsTable := quoteIdentifier(config.OffsetsTable())
	return wmsql.DefaultPostgreSQLOffsetsAdapter{
		GenerateMessagesOffsetsTableName: func(string) string { return offsetsTable },
	}
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

//nolint:ireturn // Watermill SQL constructors accept the library executor interface.
func asContextExecutor(db any) wmsql.ContextExecutor {
	if tx, ok := db.(*sql.Tx); ok {
		return wmsql.TxFromStdSQL(tx)
	}
	executor, _ := db.(*sql.DB)
	return wmsql.BeginnerFromStdSQL(executor)
}

func closeIfPresent(closer interface{ Close() error }) error {
	if closer == nil {
		return nil
	}
	return closer.Close()
}

func closeDB(db *sql.DB) error {
	if db == nil {
		return nil
	}
	return db.Close()
}
