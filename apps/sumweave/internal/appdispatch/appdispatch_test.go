package appdispatch

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	wmsql "github.com/ThreeDotsLabs/watermill-sql/v4/pkg/sql"
	wmmessage "github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/gemyago/sumweave/apps/sumweave/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppDispatch(t *testing.T) {
	fake := faker.New()
	logger := slog.New(slog.DiscardHandler)
	makeConfig := func(t *testing.T) Config {
		t.Helper()
		return Config{
			DatabaseDSN:  filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"),
			TablePrefix:  "dispatch_",
			PollInterval: 10 * time.Millisecond,
		}
	}
	openMigrated := func(t *testing.T, config Config) *sql.DB {
		t.Helper()
		db, err := sqlconn.Open(config.DatabaseDSN)
		require.NoError(t, err)
		require.NoError(t, AutoMigrate(t.Context(), config, db))
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		return db
	}
	makePublisher := func(t *testing.T, config Config, db *sql.DB) *Publisher {
		t.Helper()
		publisher, err := NewPublisher(config, db, logger)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })
		return publisher
	}
	receive := func(t *testing.T, messages <-chan *Message) Message {
		t.Helper()
		select {
		case message := <-messages:
			return *message
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for message")
			return Message{}
		}
	}

	t.Run("prepares a topic-aware schema explicitly", func(t *testing.T) {
		config := makeConfig(t)
		db, err := sqlconn.Open(config.DatabaseDSN)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })

		publisher, err := NewPublisher(config, db, logger)
		require.NoError(t, err)
		require.Error(t, publisher.Publish(t.Context(), NewMessage("topic."+fake.UUID().V4(), []byte("payload"))))

		require.NoError(t, AutoMigrate(t.Context(), config, db))
		require.NoError(t, AutoMigrate(t.Context(), config, db))

		columns := make(map[string]int)
		rows, err := db.QueryContext(t.Context(), `PRAGMA table_info(`+quoteIdentifier(config.OffsetsTable())+`)`)
		require.NoError(t, err)
		defer func() { require.NoError(t, rows.Close()) }()
		for rows.Next() {
			var cid, notNull, primaryKey int
			var name, columnType string
			var defaultValue sql.NullString
			require.NoError(t, rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey))
			columns[name] = primaryKey
		}
		require.NoError(t, rows.Err())
		assert.Equal(t, 1, columns["topic"])
		assert.Equal(t, 2, columns["consumer_group"])

		queries, err := buildPostgresMigrationQueries(Config{TablePrefix: "dispatch_"})
		require.NoError(t, err)
		require.Len(t, queries, 4)
	})

	t.Run("upgrades an existing sqlite transport schema", func(t *testing.T) {
		config := makeConfig(t)
		db, err := sqlconn.Open(config.DatabaseDSN)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		legacyMessageID := fake.UUID().V4()
		_, err = db.ExecContext(t.Context(), `CREATE TABLE `+quoteIdentifier(config.MessagesTable())+` (
			"offset" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
			uuid TEXT NOT NULL,
			topic TEXT NOT NULL,
			created_at TEXT NOT NULL,
			payload BLOB,
			metadata JSON NOT NULL
		)`)
		require.NoError(t, err)
		for range 2 {
			_, err = db.ExecContext(
				t.Context(),
				`INSERT INTO `+quoteIdentifier(config.MessagesTable())+
					` (uuid, topic, created_at, payload, metadata) VALUES (?, ?, ?, ?, ?)`,
				legacyMessageID,
				"topic."+fake.UUID().V4(),
				time.Now().Format(time.RFC3339),
				[]byte("payload-"+fake.UUID().V4()),
				[]byte(`{}`),
			)
			require.NoError(t, err)
		}

		migrator, err := NewMigrator(config, db)
		require.NoError(t, err)
		require.NoError(t, migrator.Migrate(t.Context()))
		require.NoError(t, migrator.Migrate(t.Context()))

		var messageCount int
		require.NoError(t, db.QueryRowContext(
			t.Context(),
			`SELECT COUNT(*) FROM `+quoteIdentifier(config.MessagesTable())+` WHERE uuid=?`,
			legacyMessageID,
		).Scan(&messageCount))
		assert.Equal(t, 1, messageCount)

		publisher := makePublisher(t, config, db)
		require.ErrorIs(t, publisher.Publish(t.Context(), Message{
			ID:      legacyMessageID,
			Topic:   "topic." + fake.UUID().V4(),
			Payload: []byte("payload-" + fake.UUID().V4()),
		}), ErrDuplicateMessageID)
	})

	t.Run("publishes explicit messages and preserves transaction boundaries", func(t *testing.T) {
		config := makeConfig(t)
		db := openMigrated(t, config)
		publisher := makePublisher(t, config, db)
		topic := "topic." + fake.UUID().V4()
		message := NewMessage(topic, []byte("payload-"+fake.UUID().V4()))
		message.Metadata = map[string]string{"traceId": fake.UUID().V4()}
		require.NoError(t, publisher.Publish(t.Context(), message))

		var storedTopic, storedID string
		require.NoError(t, db.QueryRowContext(
			t.Context(),
			`SELECT topic, uuid FROM `+quoteIdentifier(config.MessagesTable())+` WHERE uuid=?`,
			message.ID,
		).Scan(&storedTopic, &storedID))
		assert.Equal(t, message.Topic, storedTopic)
		assert.Equal(t, message.ID, storedID)

		committed := NewMessage(topic, []byte("committed"))
		tx, err := db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		require.NoError(t, publisher.PublishInTx(t.Context(), tx, committed))
		require.NoError(t, tx.Commit())

		rolledBack := NewMessage(topic, []byte("rolled-back"))
		tx, err = db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		require.NoError(t, publisher.PublishInTx(t.Context(), tx, rolledBack))
		require.NoError(t, tx.Rollback())

		var count int
		require.NoError(t, db.QueryRowContext(
			t.Context(),
			`SELECT COUNT(*) FROM `+quoteIdentifier(config.MessagesTable())+` WHERE uuid IN (?, ?)`,
			committed.ID,
			rolledBack.ID,
		).Scan(&count))
		assert.Equal(t, 1, count)

		require.EqualError(t, publisher.Publish(t.Context(), Message{Topic: topic}), "message id is required")
		require.EqualError(
			t,
			publisher.Publish(t.Context(), Message{ID: fake.UUID().V4()}),
			"message topic is required",
		)
		require.EqualError(t, publisher.PublishInTx(t.Context(), nil, message), "publish transaction is required")
	})

	t.Run("publishes generic requests with stable idempotent references", func(t *testing.T) {
		config := makeConfig(t)
		db := openMigrated(t, config)
		publisher := makePublisher(t, config, db)
		topic := "topic." + fake.UUID().V4()
		key := "key." + fake.UUID().V4()
		request := PublicationRequest{
			Topic:          topic,
			Payload:        []byte(`{"first":"value","second":2}`),
			IdempotencyKey: key,
		}

		first, err := publisher.PublishRequest(t.Context(), request)
		require.NoError(t, err)
		second, err := publisher.PublishRequest(t.Context(), PublicationRequest{
			Topic:          topic,
			Payload:        []byte(`{"second":2,"first":"value"}`),
			IdempotencyKey: key,
		})
		require.NoError(t, err)
		assert.Equal(t, first, second)

		_, err = publisher.PublishRequest(t.Context(), PublicationRequest{
			Topic:          topic,
			Payload:        []byte(`{"first":"different"}`),
			IdempotencyKey: key,
		})
		require.ErrorIs(t, err, ErrPublicationConflict)
		_, err = publisher.PublishRequest(t.Context(), PublicationRequest{
			Topic:          "topic." + fake.UUID().V4(),
			Payload:        request.Payload,
			IdempotencyKey: key,
		})
		require.ErrorIs(t, err, ErrPublicationConflict)

		t.Run("preserves large JSON integer precision for idempotency conflicts", func(t *testing.T) {
			largeIntegerKey := "key." + fake.UUID().V4()
			_, err = publisher.PublishRequest(t.Context(), PublicationRequest{
				Topic:          topic,
				Payload:        []byte(`{"id":9007199254740993}`),
				IdempotencyKey: largeIntegerKey,
			})
			require.NoError(t, err)

			_, err = publisher.PublishRequest(t.Context(), PublicationRequest{
				Topic:          topic,
				Payload:        []byte(`{"id":9007199254740992}`),
				IdempotencyKey: largeIntegerKey,
			})
			require.ErrorIs(t, err, ErrPublicationConflict)
		})

		t.Run("returns the same reference for equal semantic JSON", func(t *testing.T) {
			semanticKey := "key." + fake.UUID().V4()
			var semanticFirst, semanticSecond PublicationReference
			semanticFirst, err = publisher.PublishRequest(t.Context(), PublicationRequest{
				Topic:          topic,
				Payload:        []byte(`{"nested":{"second":2,"first":1},"items":[true,null]}`),
				IdempotencyKey: semanticKey,
			})
			require.NoError(t, err)

			semanticSecond, err = publisher.PublishRequest(t.Context(), PublicationRequest{
				Topic:          topic,
				Payload:        []byte(`{"items":[true,null],"nested":{"first":1,"second":2}}`),
				IdempotencyKey: semanticKey,
			})
			require.NoError(t, err)
			assert.Equal(t, semanticFirst, semanticSecond)
		})

		committed := PublicationRequest{Topic: topic, Payload: []byte(`{"committed":true}`)}
		tx, err := db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		committedReference, err := publisher.PublishRequestInTx(t.Context(), tx, committed)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())

		rolledBack := PublicationRequest{Topic: topic, Payload: []byte(`{"rolledBack":true}`)}
		tx, err = db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		rolledBackReference, err := publisher.PublishRequestInTx(t.Context(), tx, rolledBack)
		require.NoError(t, err)
		require.NoError(t, tx.Rollback())

		var count int
		require.NoError(t, db.QueryRowContext(
			t.Context(),
			`SELECT COUNT(*) FROM `+quoteIdentifier(config.MessagesTable())+` WHERE uuid IN (?, ?)`,
			committedReference.MessageID,
			rolledBackReference.MessageID,
		).Scan(&count))
		assert.Equal(t, 1, count)

		message := NewMessage(topic, []byte("duplicate-"+fake.UUID().V4()))
		require.NoError(t, publisher.Publish(t.Context(), message))
		require.ErrorIs(t, publisher.Publish(t.Context(), message), ErrDuplicateMessageID)

		unkeyedReference, err := publisher.PublishRequest(t.Context(), PublicationRequest{
			Topic:   topic,
			Payload: []byte("non-json-" + fake.UUID().V4()),
		})
		require.NoError(t, err)
		assert.NotEmpty(t, unkeyedReference.MessageID)
		_, err = publisher.PublishRequestInTx(t.Context(), nil, PublicationRequest{Topic: topic})
		require.EqualError(t, err, "publish transaction is required")
		_, err = publisher.PublishRequest(t.Context(), PublicationRequest{})
		require.EqualError(t, err, "publication topic is required")
	})

	t.Run("builds topic-scoped postgres publisher and subscriber queries", func(t *testing.T) {
		config := Config{TablePrefix: "dispatch_"}
		topic := "topic." + fake.UUID().V4()
		group := "group." + fake.UUID().V4()
		wmMessage := wmmessage.NewMessage(fake.UUID().V4(), []byte("payload"))
		wmMessage.Metadata.Set("traceId", fake.UUID().V4())
		sqliteInsert, err := makeSQLitePublisherSchema(config).InsertQuery(wmsql.InsertQueryParams{
			Topic: topic,
			Msgs:  wmmessage.Messages{wmMessage},
		})
		require.NoError(t, err)
		assert.Equal(t, topic, sqliteInsert.Args[1])

		insert, err := postgresSchema(config).InsertQuery(wmsql.InsertQueryParams{
			Topic: topic,
			Msgs:  wmmessage.Messages{wmMessage},
		})
		require.NoError(t, err)
		assert.Equal(t, topic, insert.Args[1])

		offsets := postgresOffsets(config)
		selected, err := postgresSchema(config).SelectQuery(wmsql.SelectQueryParams{
			Topic: topic, ConsumerGroup: group, OffsetsAdapter: offsets,
		})
		require.NoError(t, err)
		assert.Equal(t, []any{topic, group}, selected.Args)

		before, err := offsets.BeforeSubscribingQueries(wmsql.BeforeSubscribingQueriesParams{
			Topic: topic, ConsumerGroup: group,
		})
		require.NoError(t, err)
		require.Len(t, before, 1)
		assert.Equal(t, []any{topic, group}, before[0].Args)

		ack, err := offsets.AckMessageQuery(wmsql.AckMessageQueryParams{
			Topic:         topic,
			ConsumerGroup: group,
			LastRow:       wmsql.Row{Offset: 7, ExtraData: map[string]any{"transaction_id": wmsql.XID8(9)}},
		})
		require.NoError(t, err)
		assert.Equal(t, topic, ack.Args[2])
		assert.Equal(t, group, ack.Args[3])

		_, err = offsets.AckMessageQuery(wmsql.AckMessageQueryParams{LastRow: wmsql.Row{}})
		require.EqualError(t, err, "transaction_id not found in message row")
	})

	t.Run("isolates topics and consumer groups and resumes offsets", func(t *testing.T) {
		config := makeConfig(t)
		db := openMigrated(t, config)
		publisher := makePublisher(t, config, db)
		factory, err := NewRouterFactory(config, db, publisher, logger)
		require.NoError(t, err)

		topicA := "topic.a." + fake.UUID().V4()
		topicB := "topic.b." + fake.UUID().V4()
		groupA := "group.a." + fake.UUID().V4()
		groupB := "group.b." + fake.UUID().V4()
		messageA := NewMessage(topicA, []byte("a"))
		messageB := NewMessage(topicB, []byte("b"))
		require.NoError(t, publisher.Publish(t.Context(), messageA))
		require.NoError(t, publisher.Publish(t.Context(), messageB))

		subscribe := func(t *testing.T, group string, topic string) (<-chan *Message, context.CancelFunc) {
			t.Helper()
			router, routerErr := factory.NewRouter(group)
			require.NoError(t, routerErr)
			results := make(chan *Message, 4)
			handler, handlerErr := NewHandler(topic, func(_ context.Context, message Message) error {
				messageCopy := message
				results <- &messageCopy
				return nil
			})
			require.NoError(t, handlerErr)
			require.NoError(t, router.Handle(handler))
			ctx, cancel := context.WithCancel(t.Context())
			go func() { _ = router.Run(ctx) }()
			t.Cleanup(func() {
				cancel()
				require.NoError(t, router.Close())
			})
			return results, cancel
		}

		groupAMessages, stopGroupA := subscribe(t, groupA, topicA)
		groupBMessages, _ := subscribe(t, groupB, topicA)
		groupATopicBMessages, _ := subscribe(t, groupA, topicB)
		assert.Equal(t, messageA.ID, receive(t, groupAMessages).ID)
		assert.Equal(t, messageA.ID, receive(t, groupBMessages).ID)
		assert.Equal(t, messageB.ID, receive(t, groupATopicBMessages).ID)

		stopGroupA()
		second := NewMessage(topicA, []byte("second"))
		require.NoError(t, publisher.Publish(t.Context(), second))
		resumed, _ := subscribe(t, groupA, topicA)
		assert.Equal(t, second.ID, receive(t, resumed).ID)
	})

	t.Run("coordinates same-group router instances", func(t *testing.T) {
		config := makeConfig(t)
		db := openMigrated(t, config)
		publisher := makePublisher(t, config, db)
		factory, err := NewRouterFactory(config, db, publisher, logger)
		require.NoError(t, err)
		topic := "topic." + fake.UUID().V4()
		group := "group." + fake.UUID().V4()
		var handled atomic.Int32
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		for range 2 {
			router, routerErr := factory.NewRouter(group)
			require.NoError(t, routerErr)
			handler, handlerErr := NewHandler(topic, func(context.Context, Message) error {
				handled.Add(1)
				return nil
			})
			require.NoError(t, handlerErr)
			require.NoError(t, router.Handle(handler))
			go func() { _ = router.Run(ctx) }()
			t.Cleanup(func() { require.NoError(t, router.Close()) })
		}
		require.NoError(t, publisher.Publish(t.Context(), NewMessage(topic, []byte("payload"))))
		require.Eventually(t, func() bool { return handled.Load() == 1 }, 5*time.Second, 20*time.Millisecond)
		time.Sleep(100 * time.Millisecond)
		assert.Equal(t, int32(1), handled.Load())
	})

	t.Run("does not let a stale lease holder release its successor", func(t *testing.T) {
		config := makeConfig(t)
		db := openMigrated(t, config)
		publisher := makePublisher(t, config, db)
		topic := "topic." + fake.UUID().V4()
		group := "group." + fake.UUID().V4()
		require.NoError(t, publisher.Publish(t.Context(), NewMessage(topic, []byte("payload-"+fake.UUID().V4()))))
		_, err := db.ExecContext(
			t.Context(),
			`INSERT INTO `+quoteIdentifier(config.OffsetsTable())+
				` (topic, consumer_group, offset_acked, locked_until, lease_id) VALUES (?, ?, 0, 0, '')`,
			topic,
			group,
		)
		require.NoError(t, err)

		first := newSQLiteSubscription(config, db, group, topic, watermill.NewSlogLogger(logger))
		t.Cleanup(func() {
			first.pollTicker.Stop()
			first.lockTicker.Stop()
		})
		firstBatch, err := first.NextBatch(t.Context())
		require.NoError(t, err)
		require.Len(t, firstBatch, 1)
		_, err = db.ExecContext(
			t.Context(),
			`UPDATE `+quoteIdentifier(config.OffsetsTable())+` SET locked_until=0 WHERE topic=? AND consumer_group=?`,
			topic,
			group,
		)
		require.NoError(t, err)

		second := newSQLiteSubscription(config, db, group, topic, watermill.NewSlogLogger(logger))
		t.Cleanup(func() {
			second.pollTicker.Stop()
			second.lockTicker.Stop()
		})
		secondBatch, err := second.NextBatch(t.Context())
		require.NoError(t, err)
		require.Len(t, secondBatch, 1)
		first.lastAckedOffset = firstBatch[0].Offset
		require.ErrorIs(t, first.ReleaseLock(t.Context()), errSQLiteDeliveryLeaseLost)

		var offset int64
		var leaseID string
		require.NoError(t, db.QueryRowContext(
			t.Context(),
			`SELECT offset_acked, lease_id FROM `+quoteIdentifier(config.OffsetsTable())+
				` WHERE topic=? AND consumer_group=?`,
			topic,
			group,
		).Scan(&offset, &leaseID))
		assert.Zero(t, offset)
		assert.Equal(t, second.leaseID, leaseID)
		assert.NotEqual(t, first.leaseID, leaseID)
	})

	t.Run("routes topics with bounded retry panic recovery and dead letters", func(t *testing.T) {
		config := makeConfig(t)
		db := openMigrated(t, config)
		publisher := makePublisher(t, config, db)
		factory, err := NewRouterFactory(config, db, publisher, logger)
		require.NoError(t, err)
		group := "group." + fake.UUID().V4()
		topicRetry := "topic.retry." + fake.UUID().V4()
		topicPanic := "topic.panic." + fake.UUID().V4()
		topicHealthy := "topic.healthy." + fake.UUID().V4()
		router, err := factory.NewRouter(group)
		require.NoError(t, err)

		var retryCalls atomic.Int32
		retryHandler, err := NewHandler(topicRetry, func(context.Context, Message) error {
			if retryCalls.Add(1) < 2 {
				return errors.New("transient")
			}
			return nil
		})
		require.NoError(t, err)
		require.NoError(t, router.Handle(retryHandler))

		panicHandler, err := NewHandler(topicPanic, func(context.Context, Message) error {
			panic("handler panic")
		})
		require.NoError(t, err)
		require.NoError(t, router.Handle(panicHandler))

		healthy := make(chan string, 1)
		healthyHandler, err := NewHandler(topicHealthy, func(_ context.Context, message Message) error {
			healthy <- message.ID
			return nil
		})
		require.NoError(t, err)
		require.NoError(t, router.Handle(healthyHandler))

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go func() { _ = router.Run(ctx) }()
		retryMessage := NewMessage(topicRetry, []byte("retry"))
		panicMessage := NewMessage(topicPanic, []byte("panic"))
		healthyMessage := NewMessage(topicHealthy, []byte("healthy"))
		require.NoError(t, publisher.Publish(t.Context(), retryMessage))
		require.NoError(t, publisher.Publish(t.Context(), panicMessage))
		require.NoError(t, publisher.Publish(t.Context(), healthyMessage))
		require.Equal(t, healthyMessage.ID, <-healthy)
		require.Eventually(t, func() bool {
			var count int
			queryErr := db.QueryRowContext(
				t.Context(),
				`SELECT COUNT(*) FROM `+quoteIdentifier(config.MessagesTable())+` WHERE topic=? AND `+
					`json_extract(metadata, '$.`+originalMessageIDMetadataKey+`')=?`,
				DeadLetterTopic,
				panicMessage.ID,
			).Scan(&count)
			return queryErr == nil && count == 1
		}, 8*time.Second, 50*time.Millisecond)
		assert.Equal(t, int32(2), retryCalls.Load())

		var poisonedTopic, reason, originalMessageID string
		require.NoError(t, db.QueryRowContext(
			t.Context(),
			`SELECT json_extract(metadata, '$.`+middleware.PoisonedTopicKey+`'),`+
				` json_extract(metadata, '$.`+middleware.ReasonForPoisonedKey+`'),`+
				` json_extract(metadata, '$.`+originalMessageIDMetadataKey+`') FROM `+
				quoteIdentifier(config.MessagesTable())+` WHERE topic=? AND `+
				`json_extract(metadata, '$.`+originalMessageIDMetadataKey+`')=?`,
			DeadLetterTopic,
			panicMessage.ID,
		).Scan(&poisonedTopic, &reason, &originalMessageID))
		assert.Equal(t, topicPanic, poisonedTopic)
		assert.Contains(t, reason, "handler panic")
		assert.Equal(t, panicMessage.ID, originalMessageID)
	})

	t.Run("keeps a failed dead-letter publication unacknowledged", func(t *testing.T) {
		config := makeConfig(t)
		db := openMigrated(t, config)
		publisher := makePublisher(t, config, db)
		factory, err := NewRouterFactory(config, db, publisher, logger)
		require.NoError(t, err)
		topic := "topic." + fake.UUID().V4()
		group := "group." + fake.UUID().V4()
		router, err := factory.NewRouter(group)
		require.NoError(t, err)
		started := make(chan struct{})
		var once sync.Once
		handler, err := NewHandler(topic, func(context.Context, Message) error {
			once.Do(func() { close(started) })
			return errors.New("always fails")
		})
		require.NoError(t, err)
		require.NoError(t, router.Handle(handler))
		message := NewMessage(topic, []byte("payload"))
		require.NoError(t, publisher.Publish(t.Context(), message))
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go func() { _ = router.Run(ctx) }()
		<-started
		require.NoError(t, publisher.Close())

		require.Eventually(t, func() bool {
			var offset int
			query := `SELECT offset_acked FROM ` + quoteIdentifier(config.OffsetsTable()) +
				` WHERE topic=? AND consumer_group=?`
			queryErr := db.QueryRowContext(
				t.Context(),
				query,
				topic,
				group,
			).Scan(&offset)
			return queryErr == nil && offset == 0
		}, 5*time.Second, 50*time.Millisecond)
	})

	t.Run("validates constructors and driver selection", func(t *testing.T) {
		assert.Equal(t, TransportDriverSQLite, Config{DatabaseDSN: ":memory:"}.Driver())
		assert.Equal(t, TransportDriverPostgres, Config{DatabaseDSN: "postgres://example.invalid/db"}.Driver())
		_, err := NewPublisher(Config{}, nil, logger)
		require.EqualError(t, err, "sql database is required")
		_, err = NewPublisher(Config{}, &sql.DB{}, nil)
		require.EqualError(t, err, "logger is required")
		_, err = NewRouterFactory(Config{}, nil, &Publisher{}, logger)
		require.EqualError(t, err, "sql database is required")
		require.NoError(t, (*Publisher)(nil).Close())
		require.NoError(t, (*Router)(nil).Close())
		assert.Equal(t, `"a""b"`, quoteIdentifier(`a"b`))
		postgresPublisher := &Publisher{config: Config{DatabaseDSN: "postgres://example.invalid/database"}}
		assert.Equal(t, "$1", postgresPublisher.publicationPlaceholder(1))
		assert.Equal(t, "($1, $2)", postgresPublisher.publicationPlaceholders(2))
		assert.Equal(
			t,
			Message{
				ID:       "id",
				Topic:    testTopic,
				Payload:  []byte("payload"),
				Metadata: map[string]string{},
			},
			makeMessage(testTopic, wmmessage.NewMessage("id", []byte("payload"))),
		)
		privateMetadataMessage := wmmessage.NewMessage("id", []byte("payload"))
		privateMetadataMessage.Metadata.Set(transportPayloadHashMetadataKey, fake.UUID().V4())
		assert.Empty(t, makeMessage(testTopic, privateMetadataMessage).Metadata)
		assert.Empty(t, transportMessagePayloadHash(&wmmessage.Message{}))
	})

	t.Run("does not leak Watermill into domain or runtime packages", func(t *testing.T) {
		for _, root := range []string{
			filepath.Clean(filepath.Join("..", "..", "..", "..", "finance")),
			filepath.Clean(filepath.Join("..", "..", "..", "..", "runtime")),
		} {
			require.NoError(t, assertNoWatermillImports(root))
		}
	})
}
