package appdispatch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	wmsql "github.com/ThreeDotsLabs/watermill-sql/v4/pkg/sql"
	wmmessage "github.com/ThreeDotsLabs/watermill/message"
)

const (
	DeadLetterTopic = "app.dispatch.dead-letter.v1"

	transportPayloadHashMetadataKey = "_appdispatchPayloadHash"
)

var (
	// ErrDuplicateMessageID reports an attempt to store an existing immutable
	// transport message identity.
	ErrDuplicateMessageID = errors.New("duplicate app dispatch message id")
	// ErrPublicationConflict reports reuse of an idempotency key for a different
	// semantic publication.
	ErrPublicationConflict = errors.New("app dispatch publication conflict")
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

func (c Config) MessagesTable() string {
	return c.TablePrefix + "app_dispatch_messages"
}

func (c Config) OffsetsTable() string {
	return c.TablePrefix + "app_dispatch_offsets"
}

// PublicationsTable returns the durable idempotent-publication table name.
func (c Config) PublicationsTable() string {
	return c.TablePrefix + "app_dispatch_publications"
}

// PublicationRequest describes a semantic message publication. IdempotencyKey
// is optional; when present, it identifies exactly one topic and payload.
type PublicationRequest struct {
	Topic          string
	Payload        []byte
	IdempotencyKey string
}

func (r PublicationRequest) validate() error {
	if r.Topic == "" {
		return errors.New("publication topic is required")
	}
	return nil
}

// PublicationReference identifies a durably published semantic message.
type PublicationReference struct {
	MessageID string
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
		if isDuplicateMessageIDError(err) {
			return fmt.Errorf("publish message on topic %s: %w", message.Topic, ErrDuplicateMessageID)
		}
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
		if isDuplicateMessageIDError(err) {
			return fmt.Errorf("publish message in transaction on topic %s: %w", message.Topic, ErrDuplicateMessageID)
		}
		return fmt.Errorf("publish message in transaction on topic %s: %w", message.Topic, err)
	}
	p.logger.InfoContext(ctx, "message published in transaction",
		slog.String("messageId", message.ID),
		slog.String("topic", message.Topic),
	)
	return nil
}

// PublishRequest publishes a semantic message and returns its durable identity.
// With an idempotency key, state and message publication are committed together.
func (p *Publisher) PublishRequest(ctx context.Context, request PublicationRequest) (PublicationReference, error) {
	if err := request.validate(); err != nil {
		return PublicationReference{}, err
	}
	if request.IdempotencyKey == "" {
		message := publicationMessage(request)
		if err := p.Publish(ctx, message); err != nil {
			return PublicationReference{}, err
		}
		return PublicationReference{MessageID: message.ID}, nil
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return PublicationReference{}, fmt.Errorf("begin publication transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	reference, err := p.PublishRequestInTx(ctx, tx, request)
	if err != nil {
		return PublicationReference{}, err
	}
	if err = tx.Commit(); err != nil {
		return PublicationReference{}, fmt.Errorf("commit publication transaction: %w", err)
	}
	return reference, nil
}

// PublishRequestInTx publishes a semantic message in the supplied transaction.
// A repeated idempotency key returns the original reference without publication.
func (p *Publisher) PublishRequestInTx(
	ctx context.Context,
	tx *sql.Tx,
	request PublicationRequest,
) (PublicationReference, error) {
	if tx == nil {
		return PublicationReference{}, errors.New("publish transaction is required")
	}
	if err := request.validate(); err != nil {
		return PublicationReference{}, err
	}

	message := publicationMessage(request)
	if request.IdempotencyKey == "" {
		if err := p.PublishInTx(ctx, tx, message); err != nil {
			return PublicationReference{}, err
		}
		return PublicationReference{MessageID: message.ID}, nil
	}

	claimed, err := p.claimPublication(ctx, tx, request, message.ID)
	if err != nil {
		return PublicationReference{}, err
	}
	if !claimed {
		return p.existingPublication(ctx, tx, request)
	}
	if err = p.PublishInTx(ctx, tx, message); err != nil {
		return PublicationReference{}, err
	}
	return PublicationReference{MessageID: message.ID}, nil
}

func publicationMessage(request PublicationRequest) Message {
	message := NewMessage(request.Topic, request.Payload)
	message.Metadata = map[string]string{transportPayloadHashMetadataKey: canonicalPayloadHash(request.Payload)}
	return message
}

func canonicalPayloadHash(payload []byte) string {
	canonical := payload
	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if decoder.Decode(&decoded) == nil {
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			return hashPayload(canonical)
		}
		if encoded, err := json.Marshal(decoded); err == nil {
			canonical = encoded
		}
	}
	return hashPayload(canonical)
}

func hashPayload(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (p *Publisher) claimPublication(
	ctx context.Context,
	tx *sql.Tx,
	request PublicationRequest,
	messageID string,
) (bool, error) {
	//nolint:gosec // Table names derive from trusted application configuration.
	query := `INSERT INTO ` + quoteIdentifier(p.config.PublicationsTable()) +
		` (idempotency_key, message_id, topic, payload_hash) VALUES ` + p.publicationPlaceholders(4) +
		` ON CONFLICT(idempotency_key) DO NOTHING`
	result, err := tx.ExecContext(
		ctx,
		query,
		request.IdempotencyKey,
		messageID,
		request.Topic,
		canonicalPayloadHash(request.Payload),
	)
	if err != nil {
		return false, fmt.Errorf("claim idempotent publication: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("inspect idempotent publication claim: %w", err)
	}
	return rows == 1, nil
}

func (p *Publisher) existingPublication(
	ctx context.Context,
	tx *sql.Tx,
	request PublicationRequest,
) (PublicationReference, error) {
	query := `SELECT message_id, topic, payload_hash FROM ` + quoteIdentifier(p.config.PublicationsTable()) +
		` WHERE idempotency_key=` + p.publicationPlaceholder(1)
	var messageID, topic, payloadHash string
	if err := tx.QueryRowContext(ctx, query, request.IdempotencyKey).Scan(
		&messageID,
		&topic,
		&payloadHash,
	); err != nil {
		return PublicationReference{}, fmt.Errorf("read idempotent publication: %w", err)
	}
	if topic != request.Topic || payloadHash != canonicalPayloadHash(request.Payload) {
		return PublicationReference{}, fmt.Errorf(
			"idempotency key %q: %w",
			request.IdempotencyKey,
			ErrPublicationConflict,
		)
	}
	return PublicationReference{MessageID: messageID}, nil
}

func (p *Publisher) publicationPlaceholders(count int) string {
	placeholders := make([]string, 0, count)
	for index := 1; index <= count; index++ {
		placeholders = append(placeholders, p.publicationPlaceholder(index))
	}
	return `(` + strings.Join(placeholders, `, `) + `)`
}

func (p *Publisher) publicationPlaceholder(index int) string {
	return fmt.Sprintf("$%d", index)
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
		if key == transportPayloadHashMetadataKey {
			continue
		}
		metadata[key] = value
	}
	return Message{ID: message.UUID, Topic: topic, Payload: message.Payload, Metadata: metadata}
}

func transportMessageMetadata(metadata wmmessage.Metadata) wmmessage.Metadata {
	result := make(wmmessage.Metadata, len(metadata))
	for key, value := range metadata {
		if key == transportPayloadHashMetadataKey {
			continue
		}
		result[key] = value
	}
	return result
}

func transportMessagePayloadHash(message *wmmessage.Message) string {
	if message.Metadata == nil {
		return ""
	}
	return message.Metadata.Get(transportPayloadHashMetadataKey)
}

func isDuplicateMessageIDError(err error) bool {
	return strings.Contains(err.Error(), "duplicate key")
}

//nolint:ireturn // Watermill publisher is defined by the library interface.
func newMessagePublisher(config Config, db any, logger *slog.Logger) (wmmessage.Publisher, error) {
	wmLogger := watermill.NewSlogLogger(logger)
	return wmsql.NewPublisher(asContextExecutor(db), wmsql.PublisherConfig{
		SchemaAdapter:        postgresSchema(config),
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
	return wmsql.NewSubscriber(wmsql.BeginnerFromStdSQL(db), wmsql.SubscriberConfig{
		ConsumerGroup:    consumerGroup,
		PollInterval:     config.PollInterval,
		SchemaAdapter:    postgresSchema(config),
		OffsetsAdapter:   postgresOffsets(config),
		InitializeSchema: false,
	}, wmLogger)
}

func buildPostgresMigrationQueries(
	config Config,
) ([]wmsql.Query, error) {
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
