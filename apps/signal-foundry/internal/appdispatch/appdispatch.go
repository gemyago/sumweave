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
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/telemetry"
	_ "github.com/glebarez/go-sqlite" // Register the repo-standard SQLite driver for app dispatch DB access.
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	DispatchTopicExecution  = "app.dispatch.execution.v1"
	EnvelopeVersionV1       = "v1"
	sqliteBusyTimeoutMillis = 5000
	sqliteMemoryDSN         = ":memory:"
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
		c.ConsumerName = "signal-foundry-app-dispatch"
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
	publisher wmmessage.Publisher
	logger    *slog.Logger
}

func NewPublisher(config Config, logger *slog.Logger) (*Publisher, error) {
	config = config.normalize()
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	logger = logger.WithGroup("appdispatch")
	db, err := openDatabase(config)
	if err != nil {
		return nil, err
	}
	publisher, err := newMessagePublisher(config, db, logger)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Publisher{config: config, db: db, publisher: publisher, logger: logger}, nil
}

func (p *Publisher) Close() error {
	if p == nil {
		return nil
	}
	return errors.Join(closeIfPresent(p.publisher), closeDB(p.db))
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
	subscriber wmmessage.Subscriber
	registry   *HandlerRegistry
	logger     *slog.Logger
}

func NewConsumer(config Config, registry *HandlerRegistry, logger *slog.Logger) (*Consumer, error) {
	config = config.normalize()
	if registry == nil {
		return nil, errors.New("handler registry is required")
	}
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	logger = logger.WithGroup("appdispatch")
	db, err := openDatabase(config)
	if err != nil {
		return nil, err
	}
	subscriber, err := newMessageSubscriber(config, db, logger)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Consumer{db: db, subscriber: subscriber, registry: registry, logger: logger}, nil
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
			msgCtx := telemetry.SetLogAttributesToContext(
				ctx,
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
	return errors.Join(closeIfPresent(c.subscriber), closeDB(c.db))
}

func AutoMigrate(ctx context.Context, config Config) error {
	config = config.normalize()
	db, err := openDatabase(config)
	if err != nil {
		return err
	}
	defer func() { _ = closeDB(db) }()

	if config.Driver() == TransportDriverPostgres {
		return migratePostgres(ctx, db, config)
	}
	return migrateSQLite(ctx, db, config)
}

func openDatabase(config Config) (*sql.DB, error) {
	if config.DatabaseDSN == "" {
		return nil, errors.New("database dsn is required")
	}
	driver := "sqlite"
	if config.Driver() == TransportDriverPostgres {
		driver = "pgx"
	}
	db, err := sql.Open(driver, config.DatabaseDSN)
	if err != nil {
		return nil, fmt.Errorf("open app dispatch database: %w", err)
	}
	if config.Driver() == TransportDriverSQLite {
		if err = applySQLiteConnectionDefaults(db, config.DatabaseDSN); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	return db, nil
}

func applySQLiteConnectionDefaults(db *sql.DB, dsn string) error {
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, execErr := db.ExecContext(
		context.Background(),
		fmt.Sprintf("PRAGMA busy_timeout = %d", sqliteBusyTimeoutMillis),
	); execErr != nil {
		return fmt.Errorf("set app dispatch sqlite busy timeout: %w", execErr)
	}
	if !supportsSQLiteWAL(dsn) {
		return nil
	}

	var journalMode string
	if scanErr := db.QueryRowContext(
		context.Background(),
		"PRAGMA journal_mode = WAL",
	).Scan(&journalMode); scanErr != nil {
		return fmt.Errorf("set app dispatch sqlite journal mode: %w", scanErr)
	}
	if !strings.EqualFold(strings.TrimSpace(journalMode), "wal") {
		return fmt.Errorf("set app dispatch sqlite journal mode: unexpected mode %q", journalMode)
	}
	return nil
}

func supportsSQLiteWAL(dsn string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(dsn))
	return trimmed != sqliteMemoryDSN &&
		!strings.Contains(trimmed, "mode=memory") &&
		!strings.Contains(trimmed, "cache=shared&mode=memory") &&
		!strings.Contains(trimmed, "mode=ro") &&
		!strings.Contains(trimmed, "immutable=1")
}

//nolint:ireturn // Watermill publisher is defined by the library interface.
func newMessagePublisher(config Config, db any, logger *slog.Logger) (wmmessage.Publisher, error) {
	wmLogger := watermill.NewSlogLogger(logger)
	if config.Driver() == TransportDriverPostgres {
		return wmsql.NewPublisher(asContextExecutor(db), wmsql.PublisherConfig{
			SchemaAdapter: postgresSchema(config),
		}, wmLogger)
	}
	return wmsql.NewPublisher(asContextExecutor(db), wmsql.PublisherConfig{
		SchemaAdapter: sqliteSchema{config: config},
	}, wmLogger)
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
	return wmsql.NewSubscriber(wmsql.BeginnerFromStdSQL(db), wmsql.SubscriberConfig{
		ConsumerGroup:    config.ConsumerName,
		PollInterval:     config.PollInterval,
		SchemaAdapter:    sqliteSchema{config: config},
		OffsetsAdapter:   sqliteOffsetsAdapter{config: config},
		InitializeSchema: false,
	}, wmLogger)
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

func migrateSQLite(ctx context.Context, db *sql.DB, config Config) error {
	schema := sqliteSchema{config: config}
	offsets := sqliteOffsetsAdapter{config: config}
	queries, err := schema.SchemaInitializingQueries(wmsql.SchemaInitializingQueriesParams{
		Topic: DispatchTopicExecution,
	})
	if err != nil {
		return fmt.Errorf("build sqlite app dispatch messages schema queries: %w", err)
	}
	offsetQueries, err := offsets.SchemaInitializingQueries(wmsql.OffsetsSchemaInitializingQueriesParams{
		Topic: DispatchTopicExecution,
	})
	if err != nil {
		return fmt.Errorf("build sqlite app dispatch offsets schema queries: %w", err)
	}
	for _, query := range append(queries, offsetQueries...) {
		if _, execErr := db.ExecContext(ctx, query.Query, query.Args...); execErr != nil {
			return fmt.Errorf("migrate sqlite app dispatch transport: %w", execErr)
		}
	}
	return nil
}

func migratePostgres(ctx context.Context, db *sql.DB, config Config) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin postgres transport migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	queries, err := buildPostgresMigrationQueries(config)
	if err != nil {
		return err
	}
	for _, query := range queries {
		if _, err = tx.ExecContext(ctx, query.Query, query.Args...); err != nil {
			return fmt.Errorf("exec postgres transport migration query: %w", err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit postgres transport migration: %w", err)
	}
	return nil
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

func defaultInsertArgs(messages []*wmmessage.Message) ([]any, error) {
	args := make([]any, 0, len(messages)*3)
	for _, msg := range messages {
		metadata, err := json.Marshal(msg.Metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal message metadata: %w", err)
		}
		args = append(args, msg.UUID, msg.Payload, metadata)
	}
	return args, nil
}

type sqliteSchema struct {
	config Config
}

func (s sqliteSchema) SchemaInitializingQueries(params wmsql.SchemaInitializingQueriesParams) ([]wmsql.Query, error) {
	return []wmsql.Query{{
		Query: `CREATE TABLE IF NOT EXISTS ` + s.MessagesTable(params.Topic) + ` (
			"offset" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
			"uuid" TEXT NOT NULL,
			"created_at" TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			"payload" BLOB,
			"metadata" JSON DEFAULT NULL
		)`,
	}}, nil
}

func (s sqliteSchema) InsertQuery(params wmsql.InsertQueryParams) (wmsql.Query, error) {
	args, err := defaultInsertArgs(params.Msgs)
	if err != nil {
		return wmsql.Query{}, err
	}
	return wmsql.Query{
		Query: `INSERT INTO ` + s.MessagesTable(params.Topic) +
			` (uuid, payload, metadata) VALUES ` +
			strings.TrimRight(strings.Repeat(`(?,?,?),`, len(params.Msgs)), ","),
		Args: args,
	}, nil
}

func (s sqliteSchema) SelectQuery(params wmsql.SelectQueryParams) (wmsql.Query, error) {
	nextOffsetQuery, err := params.OffsetsAdapter.NextOffsetQuery(wmsql.NextOffsetQueryParams{
		Topic:         params.Topic,
		ConsumerGroup: params.ConsumerGroup,
	})
	if err != nil {
		return wmsql.Query{}, err
	}
	return wmsql.Query{
		Query: `SELECT "offset", "uuid", "payload", "metadata" FROM ` +
			s.MessagesTable(params.Topic) +
			` WHERE "offset" > (` + nextOffsetQuery.Query +
			`) ORDER BY "offset" ASC LIMIT 100`,
		Args: nextOffsetQuery.Args,
	}, nil
}

func (s sqliteSchema) UnmarshalMessage(params wmsql.UnmarshalMessageParams) (wmsql.Row, error) {
	var row wmsql.Row
	if err := params.Row.Scan(&row.Offset, &row.UUID, &row.Payload, &row.Metadata); err != nil {
		return wmsql.Row{}, fmt.Errorf("could not scan sqlite message row: %w", err)
	}
	msg := wmmessage.NewMessage(string(row.UUID), row.Payload)
	if row.Metadata != nil {
		if err := json.Unmarshal(row.Metadata, &msg.Metadata); err != nil {
			return wmsql.Row{}, fmt.Errorf("could not unmarshal sqlite metadata: %w", err)
		}
	}
	row.Msg = msg
	return row, nil
}

func (s sqliteSchema) MessagesTable(string) string {
	return quoteIdentifier(s.config.MessagesTable())
}

func (s sqliteSchema) SubscribeIsolationLevel() sql.IsolationLevel {
	return sql.LevelSerializable
}

type sqliteOffsetsAdapter struct {
	config Config
}

func (a sqliteOffsetsAdapter) SchemaInitializingQueries(
	params wmsql.OffsetsSchemaInitializingQueriesParams,
) ([]wmsql.Query, error) {
	return []wmsql.Query{{
		Query: `CREATE TABLE IF NOT EXISTS ` + a.MessagesOffsetsTable(params.Topic) + ` (
			consumer_group TEXT NOT NULL PRIMARY KEY,
			offset_acked INTEGER NOT NULL,
			offset_consumed INTEGER NOT NULL
		)`,
	}}, nil
}

func (a sqliteOffsetsAdapter) AckMessageQuery(params wmsql.AckMessageQueryParams) (wmsql.Query, error) {
	return wmsql.Query{
		Query: `INSERT INTO ` + a.MessagesOffsetsTable(params.Topic) +
			` (consumer_group, offset_acked, offset_consumed) VALUES (?, ?, ?)` +
			` ON CONFLICT(consumer_group) DO UPDATE SET ` +
			`offset_acked=excluded.offset_acked, ` +
			`offset_consumed=excluded.offset_consumed`,
		Args: []any{params.ConsumerGroup, params.LastRow.Offset, params.LastRow.Offset},
	}, nil
}

func (a sqliteOffsetsAdapter) ConsumedMessageQuery(params wmsql.ConsumedMessageQueryParams) (wmsql.Query, error) {
	return wmsql.Query{
		Query: `INSERT INTO ` + a.MessagesOffsetsTable(params.Topic) +
			` (consumer_group, offset_acked, offset_consumed) VALUES (?, 0, ?)` +
			` ON CONFLICT(consumer_group) DO UPDATE SET ` +
			`offset_consumed=excluded.offset_consumed`,
		Args: []any{params.ConsumerGroup, params.Row.Offset},
	}, nil
}

func (a sqliteOffsetsAdapter) NextOffsetQuery(params wmsql.NextOffsetQueryParams) (wmsql.Query, error) {
	return wmsql.Query{
		Query: `SELECT COALESCE((SELECT offset_acked FROM ` +
			a.MessagesOffsetsTable(params.Topic) +
			` WHERE consumer_group=?), 0)`,
		Args: []any{params.ConsumerGroup},
	}, nil
}

func (a sqliteOffsetsAdapter) BeforeSubscribingQueries(
	params wmsql.BeforeSubscribingQueriesParams,
) ([]wmsql.Query, error) {
	return []wmsql.Query{
		{
			Query: `INSERT INTO ` + a.MessagesOffsetsTable(params.Topic) +
				` (consumer_group, offset_acked, offset_consumed) VALUES (?, 0, 0)` +
				` ON CONFLICT(consumer_group) DO NOTHING`,
			Args: []any{params.ConsumerGroup},
		},
	}, nil
}

func (a sqliteOffsetsAdapter) MessagesOffsetsTable(string) string {
	return quoteIdentifier(a.config.OffsetsTable())
}
