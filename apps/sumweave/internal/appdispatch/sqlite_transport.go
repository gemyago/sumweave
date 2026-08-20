package appdispatch

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	wmsql "github.com/ThreeDotsLabs/watermill-sql/v4/pkg/sql"
	wmmessage "github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
)

const (
	sqliteSubscriberBatchSize   = 100
	sqliteSubscriberLockTimeout = time.Second
)

var (
	errSQLiteMessageNacked        = errors.New("sqlite message was not acknowledged")
	errSQLiteDeliveryLeaseExpired = errors.New("sqlite message delivery lease expired")
	errSQLiteDeliveryLeaseLost    = errors.New("sqlite message delivery lease lost")
)

type sqliteConnection interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type sqliteDatabase interface {
	sqliteConnection
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

type sqliteTableGenerators struct {
	Messages func() string
	Offsets  func() string
}

// sqlitePublisherSchema embeds Watermill's default schema because its publisher
// depends on a schema interface that also contains subscriber-only methods.
// Auto-initialization is disabled and only InsertQuery is used.
type sqlitePublisherSchema struct {
	wmsql.DefaultMySQLSchema
}

func makeSQLitePublisherSchema(config Config) sqlitePublisherSchema {
	return sqlitePublisherSchema{DefaultMySQLSchema: wmsql.DefaultMySQLSchema{
		//nolint:gocritic // Every topic intentionally uses the configured shared table.
		GenerateMessagesTableName: func(string) string {
			return quoteIdentifier(config.MessagesTable())
		},
	}}
}

func (s sqlitePublisherSchema) InsertQuery(params wmsql.InsertQueryParams) (wmsql.Query, error) {
	var query strings.Builder
	query.WriteString(`INSERT INTO `)
	query.WriteString(s.MessagesTable(params.Topic))
	query.WriteString(` (uuid, topic, created_at, payload, metadata, payload_hash) VALUES `)
	args := make([]any, 0, len(params.Msgs)*6)
	placeholders := make([]string, 0, len(params.Msgs))
	for _, msg := range params.Msgs {
		placeholders = append(placeholders, `(?,?,?,?,?,?)`)
		metadata, err := json.Marshal(transportMessageMetadata(msg.Metadata))
		if err != nil {
			return wmsql.Query{}, fmt.Errorf("encode message %q metadata to JSON: %w", msg.UUID, err)
		}
		args = append(
			args,
			msg.UUID,
			params.Topic,
			time.Now().Format(time.RFC3339),
			msg.Payload,
			metadata,
			transportMessagePayloadHash(msg),
		)
	}
	query.WriteString(strings.Join(placeholders, ","))
	return wmsql.Query{Query: query.String(), Args: args}, nil
}

type sqliteTransportSubscriber struct {
	db            sqliteDatabase
	config        Config
	consumerGroup string
	logger        watermill.LoggerAdapter

	closed        chan struct{}
	subscriptions sync.WaitGroup
	mu            sync.Mutex
	closedFlag    bool
}

//nolint:ireturn // The app dispatch seam returns the Watermill subscriber interface.
func newSQLiteTransportSubscriber(
	config Config,
	db *sql.DB,
	consumerGroup string,
	logger watermill.LoggerAdapter,
) (wmmessage.Subscriber, error) {
	if db == nil {
		return nil, errors.New("sqlite subscriber database is required")
	}
	return &sqliteTransportSubscriber{
		db:            db,
		config:        config,
		consumerGroup: consumerGroup,
		logger:        logger,
		closed:        make(chan struct{}),
	}, nil
}

func (s *sqliteTransportSubscriber) Subscribe(ctx context.Context, topic string) (<-chan *wmmessage.Message, error) {
	s.mu.Lock()
	closed := s.closedFlag
	s.mu.Unlock()
	if closed {
		return nil, errors.New("sqlite subscriber is closed")
	}

	if _, err := s.db.ExecContext(
		ctx,
		`INSERT INTO `+quoteIdentifier(s.config.OffsetsTable())+
			` (topic, consumer_group, offset_acked, locked_until, lease_id) VALUES (?, ?, 0, 0, '')`+
			` ON CONFLICT(topic, consumer_group) DO NOTHING`,
		topic,
		s.consumerGroup,
	); err != nil {
		return nil, err
	}

	sub := newSQLiteSubscription(s.config, s.db, s.consumerGroup, topic, s.logger)

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-s.closed
		cancel()
	}()

	s.subscriptions.Add(1)
	go func() {
		defer s.subscriptions.Done()
		defer close(sub.destination)
		defer cancel()
		sub.Run(ctx)
	}()

	return sub.destination, nil
}

func newSQLiteSubscription(
	config Config,
	db sqliteDatabase,
	consumerGroup string,
	topic string,
	logger watermill.LoggerAdapter,
) *sqliteSubscription {
	tables := sqliteTableNameGenerators(config)
	return &sqliteSubscription{
		db:                 db,
		pollTicker:         time.NewTicker(config.PollInterval),
		lockTicker:         time.NewTicker(sqliteSubscriberLockTimeout - 300*time.Millisecond),
		lockDuration:       sqliteSubscriberLockTimeout - 300*time.Millisecond,
		lockTimeoutSeconds: int64(sqliteSubscriberLockTimeout / time.Second),
		topic:              topic,
		consumerGroup:      consumerGroup,
		sqlLockConsumerGroup: `UPDATE ` + quoteIdentifier(tables.Offsets()) +
			` SET locked_until=(unixepoch()+?), lease_id=? WHERE topic=? AND consumer_group=?` +
			` AND locked_until < unixepoch() RETURNING offset_acked`,
		sqlExtendLock: `UPDATE ` + quoteIdentifier(tables.Offsets()) +
			` SET locked_until=(unixepoch()+?), offset_acked=? WHERE topic=? AND consumer_group=? AND offset_acked=? AND lease_id=?` +
			` AND locked_until>=unixepoch() RETURNING COALESCE(locked_until, 0)`,
		sqlNextMessageBatch: `SELECT "offset", uuid, payload, metadata FROM ` + quoteIdentifier(tables.Messages()) +
			fmt.Sprintf(` WHERE topic=? AND "offset">? ORDER BY offset LIMIT %d`, sqliteSubscriberBatchSize),
		sqlAcknowledgeMessages: `UPDATE ` + quoteIdentifier(tables.Offsets()) +
			` SET offset_acked=?, locked_until=0, lease_id='' WHERE topic=? AND consumer_group=? AND offset_acked=? AND lease_id=?`,
		destination: make(chan *wmmessage.Message),
		logger: logger.With(watermill.LogFields{
			"topic":          topic,
			"consumer_group": consumerGroup,
		}),
	}
}

func (s *sqliteTransportSubscriber) Close() error {
	s.mu.Lock()
	if !s.closedFlag {
		s.closedFlag = true
		close(s.closed)
	}
	s.mu.Unlock()
	s.subscriptions.Wait()
	return nil
}

type sqliteSubscription struct {
	db                 sqliteDatabase
	pollTicker         *time.Ticker
	lockTicker         *time.Ticker
	lockDuration       time.Duration
	lockTimeoutSeconds int64
	topic              string
	consumerGroup      string

	sqlLockConsumerGroup   string
	sqlExtendLock          string
	sqlNextMessageBatch    string
	sqlAcknowledgeMessages string

	lockedOffset    int64
	lastAckedOffset int64
	leaseID         string
	destination     chan *wmmessage.Message
	logger          watermill.LoggerAdapter
}

type sqliteRawMessage struct {
	Offset   int64
	UUID     string
	Payload  []byte
	Metadata wmmessage.Metadata
}

func (s *sqliteSubscription) NextBatch(ctx context.Context) ([]sqliteRawMessage, error) {
	opCtx := context.WithoutCancel(ctx)
	tx, err := s.db.BeginTx(opCtx, nil)
	if err != nil {
		return nil, err
	}

	leaseID := uuid.NewString()
	lock := tx.QueryRowContext(opCtx, s.sqlLockConsumerGroup, s.lockTimeoutSeconds, leaseID, s.topic, s.consumerGroup)
	if err = lock.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("unable to acquire row lock: %w", err), rollbackSQLiteTx(tx))
	}
	if err = lock.Scan(&s.lockedOffset); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, rollbackSQLiteTx(tx)
		}
		return nil, errors.Join(fmt.Errorf("unable to scan offset_acked value: %w", err), rollbackSQLiteTx(tx))
	}
	s.lastAckedOffset = s.lockedOffset
	s.leaseID = leaseID

	rows, err := tx.QueryContext(opCtx, s.sqlNextMessageBatch, s.topic, s.lockedOffset)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("unable to query next message batch: %w", err), rollbackSQLiteTx(tx))
	}
	batch, err := buildSQLiteBatch(rows)
	if err != nil {
		return nil, errors.Join(err, rollbackSQLiteTx(tx))
	}
	if len(batch) == 0 {
		return nil, rollbackSQLiteTx(tx)
	}
	if err = tx.Commit(); err != nil {
		return nil, errors.Join(err, rollbackSQLiteTx(tx))
	}
	return batch, nil
}

func buildSQLiteBatch(rows *sql.Rows) ([]sqliteRawMessage, error) {
	batch := make([]sqliteRawMessage, 0)
	rawMetadata := []byte{}
	for rows.Next() {
		next := sqliteRawMessage{}
		if err := rows.Scan(&next.Offset, &next.UUID, &next.Payload, &rawMetadata); err != nil {
			return nil, errors.Join(err, rows.Close())
		}
		if next.Payload == nil {
			next.Payload = []byte{}
		}
		if err := json.Unmarshal(rawMetadata, &next.Metadata); err != nil {
			return nil, errors.Join(fmt.Errorf("unable to parse metadata JSON: %w", err), rows.Close())
		}
		batch = append(batch, next)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(err, rows.Close())
	}
	return batch, rows.Close()
}

func rollbackSQLiteTx(tx *sql.Tx) error {
	if tx == nil {
		return nil
	}
	err := tx.Rollback()
	if errors.Is(err, sql.ErrTxDone) {
		return nil
	}
	return err
}

func (s *sqliteSubscription) ExtendLock(ctx context.Context) error {
	var lockedUntil int64
	row := s.db.QueryRowContext(
		context.WithoutCancel(ctx),
		s.sqlExtendLock,
		s.lockTimeoutSeconds,
		s.lastAckedOffset,
		s.topic,
		s.consumerGroup,
		s.lockedOffset,
		s.leaseID,
	)
	if err := row.Scan(&lockedUntil); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errSQLiteDeliveryLeaseLost
		}
		return fmt.Errorf("unable to extend lock: %w", err)
	}
	s.lockTicker.Reset(s.lockDuration)
	s.lockedOffset = s.lastAckedOffset
	return nil
}

func (s *sqliteSubscription) ReleaseLock(ctx context.Context) error {
	result, err := s.db.ExecContext(
		context.WithoutCancel(ctx),
		s.sqlAcknowledgeMessages,
		s.lastAckedOffset,
		s.topic,
		s.consumerGroup,
		s.lockedOffset,
		s.leaseID,
	)
	if err != nil {
		return fmt.Errorf("release sqlite message delivery lease: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect sqlite message delivery lease release: %w", err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("%w: expected one row, got %d", errSQLiteDeliveryLeaseLost, rowsAffected)
	}
	return nil
}

func (s *sqliteSubscription) Send(parent context.Context, next sqliteRawMessage) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	s.lockTicker.Reset(s.lockDuration)
	for {
		msg := wmmessage.NewMessage(next.UUID, next.Payload)
		msg.Metadata = next.Metadata
		msg.SetContext(ctx)

		select {
		case <-ctx.Done():
			return nil
		case <-s.lockTicker.C:
			if err := s.ReleaseLock(ctx); err != nil {
				return fmt.Errorf("release expired sqlite message delivery lease: %w", err)
			}
			return errSQLiteDeliveryLeaseExpired
		case s.destination <- msg:
		}

	waitForMessageAcknowledgement:
		select {
		case <-ctx.Done():
			msg.Nack()
			return nil
		case <-s.lockTicker.C:
			if err := s.ExtendLock(ctx); err != nil {
				return err
			}
			goto waitForMessageAcknowledgement
		case <-msg.Acked():
			s.lastAckedOffset = next.Offset
			return nil
		case <-msg.Nacked():
			return errSQLiteMessageNacked
		}
	}
}

func (s *sqliteSubscription) Run(ctx context.Context) {
	defer s.pollTicker.Stop()
	defer s.lockTicker.Stop()

	for {
		if s.runCycle(ctx) {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-s.pollTicker.C:
		}
	}
}

func (s *sqliteSubscription) runCycle(ctx context.Context) bool {
	if ctx.Err() != nil {
		return true
	}

	batch, err := s.NextBatch(ctx)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			s.logger.Error("next message batch query failed", err, nil)
		}
		return false
	}
	if len(batch) == 0 {
		return false
	}

	s.processBatch(ctx, batch)
	return false
}

func (s *sqliteSubscription) processBatch(ctx context.Context, batch []sqliteRawMessage) {
	for _, next := range batch {
		sendErr := s.Send(ctx, next)
		if sendErr != nil {
			if !errors.Is(sendErr, context.Canceled) {
				s.logger.Error("failed to process queued message", sendErr, nil)
			}
			break
		}
		if ctx.Err() != nil {
			break
		}
	}

	releaseErr := s.ReleaseLock(ctx)
	if releaseErr != nil && !errors.Is(releaseErr, context.Canceled) {
		s.logger.Error("failed to acknowledge processed messages", releaseErr, nil)
	}
}
