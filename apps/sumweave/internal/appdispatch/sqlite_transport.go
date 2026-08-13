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
	wmmessage "github.com/ThreeDotsLabs/watermill/message"
)

const (
	sqliteSubscriberBatchSize   = 100
	sqliteSubscriberLockTimeout = time.Second
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

type sqliteTransportPublisher struct {
	db                sqliteConnection
	messagesTableName string
	logger            watermill.LoggerAdapter

	mu     sync.Mutex
	closed bool
}

//nolint:ireturn // The app dispatch seam returns the Watermill publisher interface.
func newSQLiteTransportPublisher(
	config Config,
	db any,
	logger watermill.LoggerAdapter,
) (wmmessage.Publisher, error) {
	connection, ok := db.(sqliteConnection)
	if !ok {
		return nil, errors.New("sqlite publisher connection is required")
	}
	tables := sqliteTableNameGenerators(config)
	return &sqliteTransportPublisher{
		db:                connection,
		messagesTableName: tables.Messages(),
		logger:            logger,
	}, nil
}

func (p *sqliteTransportPublisher) Publish(topic string, messages ...*wmmessage.Message) error {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return errors.New("sqlite publisher is closed")
	}
	if len(messages) == 0 {
		return nil
	}
	if topic == "" {
		return errors.New("message topic is required")
	}

	var query strings.Builder
	query.WriteString(`INSERT INTO `)
	query.WriteString(quoteIdentifier(p.messagesTableName))
	query.WriteString(` (uuid, topic, created_at, payload, metadata) VALUES `)
	args := make([]any, 0, len(messages)*5)
	placeholders := make([]string, 0, len(messages))
	for _, msg := range messages {
		placeholders = append(placeholders, `(?,?,?,?,?)`)
		metadata, err := json.Marshal(msg.Metadata)
		if err != nil {
			return fmt.Errorf("unable to encode message %q metadata to JSON: %w", msg.UUID, err)
		}
		args = append(args, msg.UUID, topic, time.Now().Format(time.RFC3339), msg.Payload, metadata)
	}
	query.WriteString(strings.Join(placeholders, ","))
	_, err := p.db.ExecContext(messages[0].Context(), query.String(), args...)
	return err
}

func (p *sqliteTransportPublisher) Close() error {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	return nil
}

type sqliteTransportSubscriber struct {
	db            sqliteDatabase
	consumerGroup string
	pollInterval  time.Duration
	lockTimeout   time.Duration
	tables        sqliteTableGenerators
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
		consumerGroup: consumerGroup,
		pollInterval:  config.PollInterval,
		lockTimeout:   sqliteSubscriberLockTimeout,
		tables:        sqliteTableNameGenerators(config),
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
		`INSERT INTO `+quoteIdentifier(s.tables.Offsets())+
			` (topic, consumer_group, offset_acked, locked_until) VALUES (?, ?, 0, 0)`+
			` ON CONFLICT(topic, consumer_group) DO NOTHING`,
		topic,
		s.consumerGroup,
	); err != nil {
		return nil, err
	}

	sub := &sqliteSubscription{
		db:                 s.db,
		pollTicker:         time.NewTicker(s.pollInterval),
		lockTicker:         time.NewTicker(s.lockTimeout - 300*time.Millisecond),
		lockDuration:       s.lockTimeout - 300*time.Millisecond,
		lockTimeoutSeconds: int64(s.lockTimeout / time.Second),
		topic:              topic,
		consumerGroup:      s.consumerGroup,
		sqlLockConsumerGroup: `UPDATE ` + quoteIdentifier(s.tables.Offsets()) +
			` SET locked_until=(unixepoch()+?) WHERE topic=? AND consumer_group=?` +
			` AND locked_until < unixepoch() RETURNING offset_acked`,
		sqlExtendLock: `UPDATE ` + quoteIdentifier(s.tables.Offsets()) +
			` SET locked_until=(unixepoch()+?), offset_acked=? WHERE topic=? AND consumer_group=? AND offset_acked=?` +
			` AND locked_until>=unixepoch() RETURNING COALESCE(locked_until, 0)`,
		sqlNextMessageBatch: `SELECT "offset", uuid, payload, metadata FROM ` + quoteIdentifier(s.tables.Messages()) +
			fmt.Sprintf(` WHERE topic=? AND "offset">? ORDER BY offset LIMIT %d`, sqliteSubscriberBatchSize),
		sqlAcknowledgeMessages: `UPDATE ` + quoteIdentifier(s.tables.Offsets()) +
			` SET offset_acked=?, locked_until=0 WHERE topic=? AND consumer_group=? AND offset_acked=?`,
		destination: make(chan *wmmessage.Message),
		logger: s.logger.With(watermill.LogFields{
			"topic":          topic,
			"consumer_group": s.consumerGroup,
		}),
	}

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

	lock := tx.QueryRowContext(opCtx, s.sqlLockConsumerGroup, s.lockTimeoutSeconds, s.topic, s.consumerGroup)
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
	)
	if err := row.Scan(&lockedUntil); err != nil {
		return fmt.Errorf("unable to extend lock: %w", err)
	}
	s.lockTicker.Reset(s.lockDuration)
	s.lockedOffset = s.lastAckedOffset
	return nil
}

func (s *sqliteSubscription) ReleaseLock(ctx context.Context) error {
	_, err := s.db.ExecContext(
		context.WithoutCancel(ctx),
		s.sqlAcknowledgeMessages,
		s.lastAckedOffset,
		s.topic,
		s.consumerGroup,
		s.lockedOffset,
	)
	return err
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
			return s.ReleaseLock(ctx)
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
			continue
		}
	}

	releaseErr := s.ReleaseLock(ctx)
	if releaseErr != nil && !errors.Is(releaseErr, context.Canceled) {
		s.logger.Error("failed to acknowledge processed messages", releaseErr, nil)
	}
}
