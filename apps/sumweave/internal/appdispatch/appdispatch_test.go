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
		require.Len(t, queries, 3)
		assert.Contains(t, queries[0].Query, "topic VARCHAR(255) NOT NULL")
		assert.Contains(t, queries[2].Query, "PRIMARY KEY (topic, consumer_group)")
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

	t.Run("builds topic-scoped postgres publisher and subscriber queries", func(t *testing.T) {
		config := Config{TablePrefix: "dispatch_"}
		topic := "topic." + fake.UUID().V4()
		group := "group." + fake.UUID().V4()
		wmMessage := wmmessage.NewMessage(fake.UUID().V4(), []byte("payload"))
		wmMessage.Metadata.Set("traceId", fake.UUID().V4())

		insert, err := postgresSchema(config).InsertQuery(wmsql.InsertQueryParams{
			Topic: topic,
			Msgs:  wmmessage.Messages{wmMessage},
		})
		require.NoError(t, err)
		assert.Contains(t, insert.Query, "uuid, topic, payload, metadata, transaction_id")
		assert.Equal(t, topic, insert.Args[1])

		offsets := postgresOffsets(config)
		selected, err := postgresSchema(config).SelectQuery(wmsql.SelectQueryParams{
			Topic: topic, ConsumerGroup: group, OffsetsAdapter: offsets,
		})
		require.NoError(t, err)
		assert.Contains(t, selected.Query, "WHERE topic = $1")
		assert.Equal(t, []any{topic, group}, selected.Args)

		before, err := offsets.BeforeSubscribingQueries(wmsql.BeforeSubscribingQueriesParams{
			Topic: topic, ConsumerGroup: group,
		})
		require.NoError(t, err)
		require.Len(t, before, 1)
		assert.Contains(t, before[0].Query, "ON CONFLICT (topic, consumer_group)")
		assert.Equal(t, []any{topic, group}, before[0].Args)

		ack, err := offsets.AckMessageQuery(wmsql.AckMessageQueryParams{
			Topic:         topic,
			ConsumerGroup: group,
			LastRow:       wmsql.Row{Offset: 7, ExtraData: map[string]any{"transaction_id": wmsql.XID8(9)}},
		})
		require.NoError(t, err)
		assert.Contains(t, ack.Query, "WHERE topic=$3 AND consumer_group=$4")
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
				`SELECT COUNT(*) FROM `+quoteIdentifier(config.MessagesTable())+` WHERE topic=? AND uuid=?`,
				DeadLetterTopic,
				panicMessage.ID,
			).Scan(&count)
			return queryErr == nil && count == 1
		}, 8*time.Second, 50*time.Millisecond)
		assert.Equal(t, int32(2), retryCalls.Load())

		var poisonedTopic, reason string
		require.NoError(t, db.QueryRowContext(
			t.Context(),
			`SELECT json_extract(metadata, '$.`+middleware.PoisonedTopicKey+`'),`+
				` json_extract(metadata, '$.`+middleware.ReasonForPoisonedKey+`') FROM `+
				quoteIdentifier(config.MessagesTable())+` WHERE topic=? AND uuid=?`,
			DeadLetterTopic,
			panicMessage.ID,
		).Scan(&poisonedTopic, &reason))
		assert.Equal(t, topicPanic, poisonedTopic)
		assert.Contains(t, reason, "handler panic")
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
