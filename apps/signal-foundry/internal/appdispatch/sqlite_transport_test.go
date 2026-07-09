package appdispatch

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ThreeDotsLabs/watermill"
	wmmessage "github.com/ThreeDotsLabs/watermill/message"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestSQLiteTransportUnits(t *testing.T) {
	fake := faker.New()
	makeConfig := func() Config {
		dsn := fmt.Sprintf(
			"file:%s?mode=memory&cache=shared",
			"appdispatch-sqlite-transport-"+fake.UUID().V4(),
		)
		return Config{
			DatabaseDSN:  dsn,
			TablePrefix:  "signal_foundry_data_",
			ConsumerName: "consumer-" + fake.UUID().V4(),
			PollInterval: 10 * time.Millisecond,
		}
	}
	makeDB := func(t *testing.T, cfg Config) *sql.DB {
		t.Helper()
		db, err := sqlconn.Open(cfg.DatabaseDSN)
		require.NoError(t, err)
		require.NoError(t, AutoMigrate(t.Context(), cfg, db))
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		return db
	}
	makeSubscription := func(t *testing.T, cfg Config, db *sql.DB) *sqliteSubscription {
		t.Helper()
		tables := sqliteTableNameGenerators(cfg)
		topicTable := quoteIdentifier(tables.Topic(DispatchTopicExecution))
		offsetsTable := quoteIdentifier(tables.Offsets(DispatchTopicExecution))
		return &sqliteSubscription{
			db:                 db,
			pollTicker:         time.NewTicker(time.Hour),
			lockTicker:         time.NewTicker(5 * time.Millisecond),
			lockDuration:       5 * time.Millisecond,
			lockTimeoutSeconds: 1,
			consumerGroup:      cfg.ConsumerName,
			sqlLockConsumerGroup: `UPDATE ` + offsetsTable +
				` SET locked_until=(unixepoch()+?) WHERE consumer_group=? AND locked_until < unixepoch() RETURNING offset_acked`,
			sqlExtendLock: `UPDATE ` + offsetsTable +
				` SET locked_until=(unixepoch()+?), offset_acked=? WHERE consumer_group=? AND offset_acked=?` +
				` AND locked_until>=unixepoch() RETURNING COALESCE(locked_until, 0)`,
			sqlNextMessageBatch: `SELECT "offset", uuid, payload, metadata FROM ` + topicTable +
				fmt.Sprintf(` WHERE "offset">? ORDER BY offset LIMIT %d`, sqliteSubscriberBatchSize),
			sqlAcknowledgeMessages: `UPDATE ` + offsetsTable +
				` SET offset_acked=?, locked_until=0 WHERE consumer_group=? AND offset_acked=?`,
			destination: make(chan *wmmessage.Message),
			logger:      watermill.NopLogger{},
		}
	}
	insertOffsetRow := func(t *testing.T, db *sql.DB, cfg Config, offsetAcked int, lockedUntilExpr string) {
		t.Helper()
		query := `INSERT INTO ` + quoteIdentifier(cfg.OffsetsTable()) +
			` (consumer_group, offset_acked, locked_until) VALUES (?, ?, ` + lockedUntilExpr + `)`
		_, err := db.ExecContext(t.Context(), query, cfg.ConsumerName, offsetAcked)
		require.NoError(t, err)
	}

	t.Run("covers publisher and subscriber constructor guards", func(t *testing.T) {
		_, err := newSQLiteTransportPublisher(Config{}, struct{}{}, watermill.NopLogger{})
		require.EqualError(t, err, "sqlite publisher connection is required")

		_, err = newSQLiteTransportSubscriber(Config{}, nil, watermill.NopLogger{})
		require.EqualError(t, err, "sqlite subscriber database is required")
	})

	t.Run("covers publisher close and empty publish branches", func(t *testing.T) {
		cfg := makeConfig()
		db := makeDB(t, cfg)
		publisher, err := newSQLiteTransportPublisher(cfg, db, watermill.NopLogger{})
		require.NoError(t, err)
		require.NoError(t, publisher.Publish(DispatchTopicExecution))
		require.NoError(t, publisher.Close())
		msg := wmmessage.NewMessage(fake.UUID().V4(), []byte(`{"ok":true}`))
		require.EqualError(t, publisher.Publish(DispatchTopicExecution, msg), "sqlite publisher is closed")
	})

	t.Run("covers subscriber closed and offset init error branches", func(t *testing.T) {
		cfg := makeConfig()
		db := makeDB(t, cfg)
		subscriberRaw, err := newSQLiteTransportSubscriber(cfg, db, watermill.NopLogger{})
		require.NoError(t, err)
		subscriber := subscriberRaw.(*sqliteTransportSubscriber)
		require.NoError(t, subscriber.Close())
		_, err = subscriber.Subscribe(t.Context(), DispatchTopicExecution)
		require.EqualError(t, err, "sqlite subscriber is closed")

		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		subscriberRaw, err = newSQLiteTransportSubscriber(cfg, mockDB, watermill.NopLogger{})
		require.NoError(t, err)
		mock.ExpectExec("INSERT INTO").WillReturnError(errors.New("offset init boom"))
		_, err = subscriberRaw.Subscribe(t.Context(), DispatchTopicExecution)
		require.EqualError(t, err, "offset init boom")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("covers next batch and batch decode error branches", func(t *testing.T) {
		cfg := makeConfig()
		db := makeDB(t, cfg)
		sub := makeSubscription(t, cfg, db)
		defer sub.pollTicker.Stop()
		defer sub.lockTicker.Stop()

		t.Run("begin tx failure", func(t *testing.T) {
			broken := &sqliteSubscription{db: beginTxErrDB{err: errors.New("begin boom")}}
			_, err := broken.NextBatch(t.Context())
			require.EqualError(t, err, "begin boom")
		})

		t.Run("missing lock row returns empty batch", func(t *testing.T) {
			batch, err := sub.NextBatch(t.Context())
			require.NoError(t, err)
			require.Empty(t, batch)
		})

		t.Run("missing messages table bubbles query error", func(t *testing.T) {
			insertOffsetRow(t, db, cfg, 0, "0")
			_, err := db.ExecContext(t.Context(), `DROP TABLE `+quoteIdentifier(cfg.MessagesTable()))
			require.NoError(t, err)
			_, err = sub.NextBatch(t.Context())
			require.ErrorContains(t, err, "unable to query next message batch")
		})

		t.Run("bad metadata bubbles decode error", func(t *testing.T) {
			badMetadataCfg := makeConfig()
			badMetadataDB := makeDB(t, badMetadataCfg)
			badMetadataSub := makeSubscription(t, badMetadataCfg, badMetadataDB)
			defer badMetadataSub.pollTicker.Stop()
			defer badMetadataSub.lockTicker.Stop()
			insertOffsetRow(t, badMetadataDB, badMetadataCfg, 0, "0")
			_, err := badMetadataDB.ExecContext(
				t.Context(),
				`INSERT INTO `+quoteIdentifier(badMetadataCfg.MessagesTable())+
					` (uuid, created_at, payload, metadata) VALUES (?, CURRENT_TIMESTAMP, ?, ?)`,
				fake.UUID().V4(),
				[]byte(`payload`),
				[]byte(`bad-json`),
			)
			require.NoError(t, err)
			_, err = badMetadataSub.NextBatch(t.Context())
			require.ErrorContains(t, err, "unable to parse metadata JSON")
		})
	})

	t.Run("covers extend lock release lock send and run cycle branches", func(t *testing.T) {
		cfg := makeConfig()
		db := makeDB(t, cfg)
		sub := makeSubscription(t, cfg, db)
		defer sub.pollTicker.Stop()
		defer sub.lockTicker.Stop()
		insertOffsetRow(t, db, cfg, 1, "unixepoch()+10")
		sub.lockedOffset = 1
		sub.lastAckedOffset = 1

		require.NoError(t, sub.ExtendLock(t.Context()))
		require.NoError(t, sub.ReleaseLock(t.Context()))
		_, err := db.ExecContext(
			t.Context(),
			`UPDATE `+quoteIdentifier(cfg.OffsetsTable())+` SET locked_until=unixepoch()+10 WHERE consumer_group = ?`,
			cfg.ConsumerName,
		)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		go func() {
			msg := <-sub.destination
			time.Sleep(10 * time.Millisecond)
			msg.Ack()
		}()
		require.NoError(t, sub.Send(ctx, sqliteRawMessage{Offset: 2, UUID: fake.UUID().V4(), Payload: []byte(`ok`)}))

		brokenCfg := makeConfig()
		brokenDB := makeDB(t, brokenCfg)
		brokenSub := makeSubscription(t, brokenCfg, brokenDB)
		defer brokenSub.pollTicker.Stop()
		defer brokenSub.lockTicker.Stop()
		brokenSub.destination = make(chan *wmmessage.Message)
		brokenSub.sqlAcknowledgeMessages = `UPDATE ` + quoteIdentifier(brokenCfg.OffsetsTable()) +
			`_missing SET offset_acked=? WHERE consumer_group=?`
		brokenSub.processBatch(t.Context(), []sqliteRawMessage{{
			Offset:  1,
			UUID:    fake.UUID().V4(),
			Payload: []byte(`blocked`),
		}})

		cycleCfg := makeConfig()
		cycleDB := makeDB(t, cycleCfg)
		cycleSub := makeSubscription(t, cycleCfg, cycleDB)
		defer cycleSub.pollTicker.Stop()
		defer cycleSub.lockTicker.Stop()
		cycleSub.sqlLockConsumerGroup = `SELECT nope`
		stopped := cycleSub.runCycle(t.Context())
		require.False(t, stopped)
	})

	t.Run("covers rollback helper nil and done cases", func(t *testing.T) {
		require.NoError(t, rollbackSQLiteTx(nil))
		cfg := makeConfig()
		db := makeDB(t, cfg)
		tx, err := db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
		require.NoError(t, rollbackSQLiteTx(tx))
	})
}

type beginTxErrDB struct {
	err error
}

func (db beginTxErrDB) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, errors.New("not implemented")
}

func (db beginTxErrDB) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, errors.New("not implemented")
}

func (db beginTxErrDB) QueryRowContext(context.Context, string, ...any) *sql.Row {
	return &sql.Row{}
}

func (db beginTxErrDB) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	return nil, db.err
}
