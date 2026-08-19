package appdispatch

import (
	"context"
	"database/sql"
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
)

const (
	DeadLetterTopic = "app.dispatch.dead-letter.v1"
)

type TransportDriver string

const (
	TransportDriverSQLite   TransportDriver = "sqlite"
	TransportDriverPostgres TransportDriver = "postgres"
)

// Message is the app-owned durable transport contract. Payload and metadata
// are opaque to appdispatch; semantic packages own their encoding.
type Message struct {
	ID       string
	Topic    string
	Payload  []byte
	Metadata map[string]string
}

// NewMessage creates a message with a transport-safe identity.
func NewMessage(topic string, payload []byte) Message {
	return Message{ID: watermill.NewUUID(), Topic: topic, Payload: payload}
}

func (m Message) validate() error {
	if m.ID == "" {
		return errors.New("message id is required")
	}
	if m.Topic == "" {
		return errors.New("message topic is required")
	}
	return nil
}

type Config struct {
	DatabaseDSN  string
	TablePrefix  string
	PollInterval time.Duration
}

func (c Config) normalize() Config {
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

type Publisher struct {
	config    Config
	db        *sql.DB
	publisher wmmessage.Publisher
	logger    *slog.Logger

	closeOnce sync.Once
	closeErr  error
}

func NewPublisher(config Config, db *sql.DB, logger *slog.Logger) (*Publisher, error) {
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
		return nil, fmt.Errorf("create transport publisher: %w", err)
	}
	return &Publisher{config: config, db: db, publisher: publisher, logger: logger}, nil
}

func (p *Publisher) Close() error {
	if p == nil {
		return nil
	}
	p.closeOnce.Do(func() {
		p.closeErr = closeIfPresent(p.publisher)
	})
	return p.closeErr
}

func (p *Publisher) Publish(ctx context.Context, message Message) error {
	if err := message.validate(); err != nil {
		return err
	}
	wmMessage := makeWatermillMessage(ctx, message)
	if err := p.publisher.Publish(message.Topic, wmMessage); err != nil {
		return fmt.Errorf("publish message on topic %s: %w", message.Topic, err)
	}
	p.logger.InfoContext(ctx, "message published",
		slog.String("messageId", message.ID),
		slog.String("topic", message.Topic),
	)
	return nil
}

func (p *Publisher) PublishInTx(ctx context.Context, tx *sql.Tx, message Message) error {
	if tx == nil {
		return errors.New("publish transaction is required")
	}
	if err := message.validate(); err != nil {
		return err
	}
	publisher, err := newMessagePublisher(p.config, tx, p.logger)
	if err != nil {
		return fmt.Errorf("create transaction publisher: %w", err)
	}
	defer func() { _ = closeIfPresent(publisher) }()
	if err = publisher.Publish(message.Topic, makeWatermillMessage(ctx, message)); err != nil {
		return fmt.Errorf("publish message in transaction on topic %s: %w", message.Topic, err)
	}
	p.logger.InfoContext(ctx, "message published in transaction",
		slog.String("messageId", message.ID),
		slog.String("topic", message.Topic),
	)
	return nil
}

func makeWatermillMessage(ctx context.Context, message Message) *wmmessage.Message {
	wmMessage := wmmessage.NewMessageWithContext(ctx, message.ID, message.Payload)
	if len(message.Metadata) > 0 {
		wmMessage.Metadata = wmmessage.Metadata(message.Metadata)
	}
	return wmMessage
}

func makeMessage(topic string, message *wmmessage.Message) Message {
	metadata := make(map[string]string, len(message.Metadata))
	for key, value := range message.Metadata {
		metadata[key] = value
	}
	return Message{ID: message.UUID, Topic: topic, Payload: message.Payload, Metadata: metadata}
}

//nolint:ireturn // Watermill publisher is defined by the library interface.
func newMessagePublisher(config Config, db any, logger *slog.Logger) (wmmessage.Publisher, error) {
	wmLogger := watermill.NewSlogLogger(logger)
	schema := wmsql.SchemaAdapter(makeSQLitePublisherSchema(config))
	if config.Driver() == TransportDriverPostgres {
		schema = postgresSchema(config)
	}
	return wmsql.NewPublisher(asContextExecutor(db), wmsql.PublisherConfig{
		SchemaAdapter:        schema,
		AutoInitializeSchema: false,
	}, wmLogger)
}

//nolint:ireturn // Watermill subscriber is defined by the library interface.
func newMessageSubscriber(
	config Config,
	db *sql.DB,
	consumerGroup string,
	logger *slog.Logger,
) (wmmessage.Subscriber, error) {
	if consumerGroup == "" {
		return nil, errors.New("consumer group is required")
	}
	wmLogger := watermill.NewSlogLogger(logger)
	if config.Driver() == TransportDriverPostgres {
		return wmsql.NewSubscriber(wmsql.BeginnerFromStdSQL(db), wmsql.SubscriberConfig{
			ConsumerGroup:    consumerGroup,
			PollInterval:     config.PollInterval,
			SchemaAdapter:    postgresSchema(config),
			OffsetsAdapter:   postgresOffsets(config),
			InitializeSchema: false,
		}, wmLogger)
	}
	return newSQLiteTransportSubscriber(config, db, consumerGroup, wmLogger)
}

func sqliteTableNameGenerators(config Config) sqliteTableGenerators {
	return sqliteTableGenerators{
		Messages: config.MessagesTable,
		Offsets:  config.OffsetsTable,
	}
}

func buildSQLiteMigrationQueries(config Config) []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS ` + quoteIdentifier(config.MessagesTable()) + ` (
			"offset" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
			uuid TEXT NOT NULL,
			topic TEXT NOT NULL,
			created_at TEXT NOT NULL,
			payload BLOB,
			metadata JSON NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS ` + quoteIdentifier(config.MessagesTable()+"_topic_offset_idx") +
			` ON ` + quoteIdentifier(config.MessagesTable()) + ` (topic, "offset")`,
		`CREATE TABLE IF NOT EXISTS ` + quoteIdentifier(config.OffsetsTable()) + ` (
			topic TEXT NOT NULL,
			consumer_group TEXT NOT NULL,
			offset_acked INTEGER NOT NULL,
			locked_until INTEGER NOT NULL,
			lease_id TEXT NOT NULL DEFAULT '',
			PRIMARY KEY(topic, consumer_group)
		)`,
	}
}

func buildPostgresMigrationQueries(config Config) ([]wmsql.Query, error) {
	queries, err := postgresSchema(config).SchemaInitializingQueries(wmsql.SchemaInitializingQueriesParams{})
	if err != nil {
		return nil, fmt.Errorf("build postgres messages schema queries: %w", err)
	}
	offsetQueries, err := postgresOffsets(config).SchemaInitializingQueries(
		wmsql.OffsetsSchemaInitializingQueriesParams{},
	)
	if err != nil {
		return nil, fmt.Errorf("build postgres offsets schema queries: %w", err)
	}
	return append(queries, offsetQueries...), nil
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

//nolint:ireturn // Watermill SQL constructors accept the library executor interface.
func asContextExecutor(db any) wmsql.ContextExecutor {
	if tx, ok := db.(*sql.Tx); ok {
		return wmsql.TxFromStdSQL(tx)
	}
	if executor, ok := db.(*sql.DB); ok {
		return wmsql.BeginnerFromStdSQL(executor)
	}
	return nil
}

func closeIfPresent(closer interface{ Close() error }) error {
	if closer == nil {
		return nil
	}
	return closer.Close()
}
