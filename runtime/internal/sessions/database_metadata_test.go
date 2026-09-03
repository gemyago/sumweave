package sessions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gemyago/sumweave/runtime/internal/gormsumweave"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDatabaseSessionMetadataStore(t *testing.T) {
	t.Parallel()

	// jaswdr/faker backs randomness with a mutex (threadSafeRand); one instance is fine across parallel subtests.
	fake := faker.New()

	newStore := func(
		t *testing.T,
		opts gormsumweave.GormSumweaveTablesOpts,
	) *DatabaseSessionMetadataStore {
		t.Helper()
		_ = opts
		store, err := NewDatabaseSessionMetadataStore(postgresTestDSN(t), postgresTestTablesOpts())
		require.NoError(t, err)
		return store
	}

	// Recent() is a time in the past ~30 days (per faker); truncate to seconds for stable comparisons. Ordering tests add fixed deltas on top.
	randomTruncatedUTC := func(f faker.Faker) time.Time {
		return f.Time().Recent().UTC().Truncate(time.Second)
	}

	t.Run("Save creates new metadata entry", func(t *testing.T) {
		t.Parallel()
		store := newStore(t, gormsumweave.GormSumweaveTablesOpts{})

		app := fake.Lorem().Word() + fake.Lorem().Word()
		user := fake.UUID().V4()
		sid := fake.UUID().V4()
		created := randomTruncatedUTC(fake)
		meta := SessionMetadata{
			SessionID: sid,
			AppName:   app,
			UserID:    user,
			Title:     fake.Lorem().Sentence(4),
			CreatedAt: created,
			UpdatedAt: created,
		}
		require.NoError(t, store.Save(t.Context(), meta))

		res, err := store.List(t.Context(), ListSessionMetadataParams{
			AppName: app,
			UserID:  user,
			Limit:   10,
			Offset:  0,
		})
		require.NoError(t, err)
		require.Equal(t, 1, res.Total)
		require.Len(t, res.Sessions, 1)
		require.Equal(t, meta.SessionID, res.Sessions[0].SessionID)
		require.Equal(t, meta.AppName, res.Sessions[0].AppName)
		require.Equal(t, meta.UserID, res.Sessions[0].UserID)
		require.Equal(t, meta.Title, res.Sessions[0].Title)
		require.True(t, res.Sessions[0].CreatedAt.Equal(meta.CreatedAt))
		require.True(t, res.Sessions[0].UpdatedAt.Equal(meta.UpdatedAt))
		var stored sessionMetadataModel
		require.NoError(t, store.db.First(&stored, "session_id = ?", sid).Error)
	})

	t.Run("Save updates existing metadata entry (upsert)", func(t *testing.T) {
		t.Parallel()
		store := newStore(t, gormsumweave.GormSumweaveTablesOpts{})

		app := fake.Lorem().Word()
		user := fake.UUID().V4()
		sid := fake.UUID().V4()
		t0 := randomTruncatedUTC(fake)
		firstTitle := fake.Lorem().Sentence(fake.IntBetween(2, 6))
		first := SessionMetadata{
			SessionID: sid,
			AppName:   app,
			UserID:    user,
			Title:     firstTitle,
			CreatedAt: t0,
			UpdatedAt: t0,
		}
		require.NoError(t, store.Save(t.Context(), first))

		t1 := t0.Add(time.Duration(fake.IntBetween(1, 72)) * time.Hour)
		second := SessionMetadata{
			SessionID: sid,
			AppName:   app,
			UserID:    user,
			Title:     fake.Lorem().Sentence(fake.IntBetween(2, 6)),
			CreatedAt: t0,
			UpdatedAt: t1,
		}
		require.NoError(t, store.Save(t.Context(), second))

		res, err := store.List(t.Context(), ListSessionMetadataParams{
			AppName: app,
			UserID:  user,
			Limit:   10,
			Offset:  0,
		})
		require.NoError(t, err)
		require.Equal(t, 1, res.Total)
		require.Equal(t, second.SessionID, res.Sessions[0].SessionID)
		require.Equal(t, second.AppName, res.Sessions[0].AppName)
		require.Equal(t, second.UserID, res.Sessions[0].UserID)
		require.Equal(t, second.Title, res.Sessions[0].Title)
		require.True(t, res.Sessions[0].CreatedAt.Equal(second.CreatedAt))
		require.True(t, res.Sessions[0].UpdatedAt.Equal(second.UpdatedAt))
	})

	t.Run("List returns entries sorted by updatedAt desc", func(t *testing.T) {
		t.Parallel()
		store := newStore(t, gormsumweave.GormSumweaveTablesOpts{})

		app := fake.Lorem().Word()
		user := fake.UUID().V4()
		baseTime := randomTruncatedUTC(fake)
		newerUpdated := baseTime.Add(time.Duration(fake.IntBetween(2, 48*30)) * time.Hour)

		olderID := fake.UUID().V4()
		newerID := fake.UUID().V4()
		require.NoError(t, store.Save(t.Context(), SessionMetadata{
			SessionID: olderID,
			AppName:   app,
			UserID:    user,
			Title:     fake.Lorem().Sentence(fake.IntBetween(2, 5)),
			CreatedAt: baseTime,
			UpdatedAt: baseTime,
		}))
		require.NoError(t, store.Save(t.Context(), SessionMetadata{
			SessionID: newerID,
			AppName:   app,
			UserID:    user,
			Title:     fake.Lorem().Sentence(fake.IntBetween(2, 5)),
			CreatedAt: baseTime.Add(time.Duration(fake.IntBetween(1, 59)) * time.Minute),
			UpdatedAt: newerUpdated,
		}))

		res, err := store.List(t.Context(), ListSessionMetadataParams{
			AppName: app,
			UserID:  user,
			Limit:   10,
			Offset:  0,
		})
		require.NoError(t, err)
		require.Equal(t, 2, res.Total)
		require.Equal(t, newerID, res.Sessions[0].SessionID)
		require.Equal(t, olderID, res.Sessions[1].SessionID)
	})

	t.Run("List paginates canonical updates deterministically", func(t *testing.T) {
		t.Parallel()
		store := newStore(t, gormsumweave.GormSumweaveTablesOpts{})
		app := fake.Lorem().Word()
		user := fake.UUID().V4()
		earlier := time.Date(2025, time.December, 31, 23, 30, 0, 123, time.UTC)
		later := time.Date(2026, time.January, 1, 0, 0, 0, 456, time.FixedZone("zero", 0))
		require.True(t, earlier.Before(later))

		earlierID := fake.UUID().V4()
		laterID := fake.UUID().V4()
		for _, metadata := range []SessionMetadata{
			{SessionID: earlierID, AppName: app, UserID: user, Title: fake.Lorem().Sentence(3), CreatedAt: earlier, UpdatedAt: earlier},
			{SessionID: laterID, AppName: app, UserID: user, Title: fake.Lorem().Sentence(3), CreatedAt: later, UpdatedAt: later},
		} {
			require.NoError(t, store.Save(t.Context(), metadata))
		}

		firstPage, err := store.List(t.Context(), ListSessionMetadataParams{
			AppName: app,
			UserID:  user,
			Limit:   1,
		})
		require.NoError(t, err)
		require.Equal(t, 2, firstPage.Total)
		require.Len(t, firstPage.Sessions, 1)
		require.Equal(t, laterID, firstPage.Sessions[0].SessionID)

		secondPage, err := store.List(t.Context(), ListSessionMetadataParams{
			AppName: app,
			UserID:  user,
			Limit:   1,
			Offset:  1,
		})
		require.NoError(t, err)
		require.Len(t, secondPage.Sessions, 1)
		require.Equal(t, earlierID, secondPage.Sessions[0].SessionID)
	})

	t.Run("List returns empty slice when no sessions", func(t *testing.T) {
		t.Parallel()
		store := newStore(t, gormsumweave.GormSumweaveTablesOpts{})

		res, err := store.List(t.Context(), ListSessionMetadataParams{
			AppName: fake.Lorem().Word(),
			UserID:  fake.UUID().V4(),
			Limit:   5,
			Offset:  0,
		})
		require.NoError(t, err)
		require.Equal(t, 0, res.Total)
		require.NotNil(t, res.Sessions)
		require.Empty(t, res.Sessions)
	})

	t.Run("List returns a find error after scoped count succeeds", func(t *testing.T) {
		t.Parallel()
		store := newStore(t, gormsumweave.GormSumweaveTablesOpts{})
		store.db.Callback().Query().Before("gorm:query").Register("test:fail-find", func(tx *gorm.DB) {
			if _, ok := tx.Statement.Dest.(*[]sessionMetadataModel); ok {
				tx.AddError(errors.New("injected find failure"))
			}
		})

		_, err := store.List(t.Context(), ListSessionMetadataParams{
			AppName: fake.Lorem().Word(),
			UserID:  fake.UUID().V4(),
			Limit:   1,
		})
		require.ErrorContains(t, err, "list session metadata")
	})

	t.Run("List with offset and limit returns correct slice and total", func(t *testing.T) {
		t.Parallel()
		store := newStore(t, gormsumweave.GormSumweaveTablesOpts{})

		app := fake.Lorem().Word()
		user := fake.UUID().V4()
		ts := randomTruncatedUTC(fake)
		step := time.Duration(fake.IntBetween(1, 6)) * time.Hour
		ids := make([]string, 4)
		for i := range 4 {
			ids[i] = fake.UUID().V4()
			require.NoError(t, store.Save(t.Context(), SessionMetadata{
				SessionID: ids[i],
				AppName:   app,
				UserID:    user,
				Title:     fake.Lorem().Sentence(fake.IntBetween(2, 5)),
				CreatedAt: ts,
				UpdatedAt: ts.Add(time.Duration(i) * step),
			}))
		}

		res, err := store.List(t.Context(), ListSessionMetadataParams{
			AppName: app,
			UserID:  user,
			Limit:   2,
			Offset:  1,
		})
		require.NoError(t, err)
		require.Equal(t, 4, res.Total)
		require.Len(t, res.Sessions, 2)
		require.Equal(t, ids[2], res.Sessions[0].SessionID)
		require.Equal(t, ids[1], res.Sessions[1].SessionID)
	})

	t.Run("Delete removes entry", func(t *testing.T) {
		t.Parallel()
		store := newStore(t, gormsumweave.GormSumweaveTablesOpts{})

		app := fake.Lorem().Word()
		user := fake.UUID().V4()
		sid := fake.UUID().V4()
		now := randomTruncatedUTC(fake)
		require.NoError(t, store.Save(t.Context(), SessionMetadata{
			SessionID: sid,
			AppName:   app,
			UserID:    user,
			Title:     fake.Lorem().Sentence(fake.IntBetween(2, 5)),
			CreatedAt: now,
			UpdatedAt: now,
		}))

		require.NoError(t, store.Delete(t.Context(), app, user, sid))

		res, err := store.List(t.Context(), ListSessionMetadataParams{
			AppName: app,
			UserID:  user,
			Limit:   10,
			Offset:  0,
		})
		require.NoError(t, err)
		require.Equal(t, 0, res.Total)
	})

	t.Run("Delete of non-existent session does not error", func(t *testing.T) {
		t.Parallel()
		store := newStore(t, gormsumweave.GormSumweaveTablesOpts{})

		err := store.Delete(t.Context(), fake.Lorem().Word(), fake.UUID().V4(), fake.UUID().V4())
		require.NoError(t, err)
	})

	t.Run("List scopes data to the created app and user", func(t *testing.T) {
		t.Parallel()
		app := fake.Lorem().Word()
		user := fake.UUID().V4()
		sid := fake.UUID().V4()
		now := randomTruncatedUTC(fake)
		meta := SessionMetadata{
			SessionID: sid,
			AppName:   app,
			UserID:    user,
			Title:     fake.Lorem().Sentence(fake.IntBetween(2, 5)),
			CreatedAt: now,
			UpdatedAt: now,
		}

		store := newStore(t, postgresTestTablesOpts())
		require.NoError(t, store.Save(t.Context(), meta))
		require.NoError(t, store.Save(t.Context(), SessionMetadata{
			SessionID: fake.UUID().V4(),
			AppName:   app,
			UserID:    fake.UUID().V4(),
			Title:     fake.Lorem().Sentence(3),
			CreatedAt: now,
			UpdatedAt: now,
		}))

		res, err := store.List(t.Context(), ListSessionMetadataParams{
			AppName: app,
			UserID:  user,
			Limit:   10,
			Offset:  0,
		})
		require.NoError(t, err)
		require.Equal(t, 1, res.Total)
		require.Equal(t, meta.SessionID, res.Sessions[0].SessionID)
		require.Equal(t, meta.AppName, res.Sessions[0].AppName)
		require.Equal(t, meta.UserID, res.Sessions[0].UserID)
		require.Equal(t, meta.Title, res.Sessions[0].Title)
		require.True(t, res.Sessions[0].CreatedAt.Equal(meta.CreatedAt))
		require.True(t, res.Sessions[0].UpdatedAt.Equal(meta.UpdatedAt))
	})

	t.Run("Save returns context error when cancelled", func(t *testing.T) {
		t.Parallel()
		store := newStore(t, gormsumweave.GormSumweaveTablesOpts{})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		now := randomTruncatedUTC(fake)
		err := store.Save(ctx, SessionMetadata{
			SessionID: fake.UUID().V4(),
			AppName:   fake.Lorem().Word(),
			UserID:    fake.UUID().V4(),
			CreatedAt: now,
			UpdatedAt: now,
		})
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("List returns context error when cancelled", func(t *testing.T) {
		t.Parallel()
		store := newStore(t, gormsumweave.GormSumweaveTablesOpts{})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := store.List(ctx, ListSessionMetadataParams{
			AppName: fake.Lorem().Word(),
			UserID:  fake.UUID().V4(),
			Limit:   1,
			Offset:  0,
		})
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("Delete returns context error when cancelled", func(t *testing.T) {
		t.Parallel()
		store := newStore(t, gormsumweave.GormSumweaveTablesOpts{})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		err := store.Delete(ctx, fake.Lorem().Word(), fake.UUID().V4(), fake.UUID().V4())
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("NewDatabaseSessionMetadataStore rejects empty dsn", func(t *testing.T) {
		t.Parallel()
		_, err := NewDatabaseSessionMetadataStore("", gormsumweave.GormSumweaveTablesOpts{})
		require.Error(t, err)
	})

	t.Run("Save validation errors", func(t *testing.T) {
		t.Parallel()
		store := newStore(t, gormsumweave.GormSumweaveTablesOpts{})
		uid := fake.UUID().V4()
		app := fake.Lorem().Word()
		now := randomTruncatedUTC(fake)

		err := store.Save(t.Context(), SessionMetadata{
			SessionID: "",
			AppName:   app,
			UserID:    uid,
			CreatedAt: now,
			UpdatedAt: now,
		})
		require.Error(t, err)

		err = store.Save(t.Context(), SessionMetadata{
			SessionID: fake.UUID().V4(),
			AppName:   "",
			UserID:    uid,
			CreatedAt: now,
			UpdatedAt: now,
		})
		require.Error(t, err)

		err = store.Save(t.Context(), SessionMetadata{
			SessionID: fake.UUID().V4(),
			AppName:   app,
			UserID:    "",
			CreatedAt: now,
			UpdatedAt: now,
		})
		require.Error(t, err)
	})

	t.Run("List validation errors", func(t *testing.T) {
		t.Parallel()
		store := newStore(t, gormsumweave.GormSumweaveTablesOpts{})
		uid := fake.UUID().V4()
		app := fake.Lorem().Word()

		_, err := store.List(t.Context(), ListSessionMetadataParams{AppName: "", UserID: uid, Limit: 1, Offset: 0})
		require.Error(t, err)
		_, err = store.List(t.Context(), ListSessionMetadataParams{AppName: app, UserID: "", Limit: 1, Offset: 0})
		require.Error(t, err)
		_, err = store.List(t.Context(), ListSessionMetadataParams{AppName: app, UserID: uid, Limit: 0, Offset: 0})
		require.Error(t, err)
		_, err = store.List(t.Context(), ListSessionMetadataParams{AppName: app, UserID: uid, Limit: 101, Offset: 0})
		require.Error(t, err)
		_, err = store.List(t.Context(), ListSessionMetadataParams{AppName: app, UserID: uid, Limit: 1, Offset: -1})
		require.Error(t, err)
	})

	t.Run("Delete validation errors", func(t *testing.T) {
		t.Parallel()
		store := newStore(t, gormsumweave.GormSumweaveTablesOpts{})
		uid := fake.UUID().V4()
		app := fake.Lorem().Word()
		sid := fake.UUID().V4()

		err := store.Delete(t.Context(), "", uid, sid)
		require.Error(t, err)
		err = store.Delete(t.Context(), app, "", sid)
		require.Error(t, err)
		err = store.Delete(t.Context(), app, uid, "")
		require.Error(t, err)
	})

	t.Run("operations fail after underlying DB closed", func(t *testing.T) {
		t.Parallel()
		store, err := NewDatabaseSessionMetadataStore(postgresTestDSN(t), postgresTestTablesOpts())
		require.NoError(t, err)
		sqlDB, err := store.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		now := randomTruncatedUTC(fake)
		err = store.Save(t.Context(), SessionMetadata{
			SessionID: fake.UUID().V4(),
			AppName:   fake.Lorem().Word(),
			UserID:    fake.UUID().V4(),
			CreatedAt: now,
			UpdatedAt: now,
		})
		require.Error(t, err)

		_, err = store.List(t.Context(), ListSessionMetadataParams{
			AppName: fake.Lorem().Word(),
			UserID:  fake.UUID().V4(),
			Limit:   1,
			Offset:  0,
		})
		require.Error(t, err)

		err = store.Delete(t.Context(), fake.Lorem().Word(), fake.UUID().V4(), fake.UUID().V4())
		require.Error(t, err)
	})
}
