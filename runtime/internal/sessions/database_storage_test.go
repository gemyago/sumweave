package sessions

import (
	"strconv"
	"testing"
	"time"

	"github.com/gemyago/sumweave/runtime/internal/gormsumweave"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/session"
)

func TestNewDatabaseSessionsStorage(t *testing.T) {
	t.Parallel()
	fake := faker.New()

	t.Run("prepared PostgreSQL schema returns storage without error", func(t *testing.T) {
		t.Parallel()
		s, err := NewDatabaseSessionsStorage(postgresTestDSN(t), postgresTestTablesOpts())
		require.NoError(t, err)
		require.NotNil(t, s)
	})

	t.Run("empty DSN returns error", func(t *testing.T) {
		t.Parallel()
		s, err := NewDatabaseSessionsStorage("", gormsumweave.GormSumweaveTablesOpts{})
		require.Error(t, err)
		require.Nil(t, s)
	})

	t.Run("invalid PostgreSQL DSN returns constructor errors", func(t *testing.T) {
		t.Parallel()
		badDSN := "postgres://localhost:" + strconv.Itoa(fake.RandomNumber(10000)) + "/" + fake.Lorem().Word()

		metadata, err := NewDatabaseSessionMetadataStore(badDSN, postgresTestTablesOpts())
		require.Error(t, err)
		require.Nil(t, metadata)

		storage, err := NewDatabaseSessionsStorage(badDSN, postgresTestTablesOpts())
		require.Error(t, err)
		require.Nil(t, storage)
	})

	t.Run("prepared schema Create succeeds", func(t *testing.T) {
		t.Parallel()
		s, err := NewDatabaseSessionsStorage(postgresTestDSN(t), postgresTestTablesOpts())
		require.NoError(t, err)

		ctx := t.Context()
		resp, err := s.Create(ctx, &session.CreateRequest{
			AppName:   fake.Lorem().Word(),
			UserID:    fake.UUID().V4(),
			SessionID: fake.UUID().V4(),
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("meta is DatabaseSessionMetadataStore", func(t *testing.T) {
		t.Parallel()
		s, err := NewDatabaseSessionsStorage(postgresTestDSN(t), postgresTestTablesOpts())
		require.NoError(t, err)
		require.NotNil(t, s.meta)
		_, ok := any(s.meta).(*DatabaseSessionMetadataStore)
		require.True(t, ok)
	})

	t.Run("SaveMetadata ListMetadata DeleteMetadata delegate to database store", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		s, err := NewDatabaseSessionsStorage(postgresTestDSN(t), postgresTestTablesOpts())
		require.NoError(t, err)

		app := fake.Lorem().Word() + "-app"
		user := fake.UUID().V4()
		sid := fake.UUID().V4()
		now := time.Now().Truncate(time.Second)
		meta := SessionMetadata{
			SessionID: sid,
			AppName:   app,
			UserID:    user,
			Title:     "title",
			CreatedAt: now,
			UpdatedAt: now,
		}
		require.NoError(t, s.SaveMetadata(ctx, meta))

		listed, err := s.ListMetadata(ctx, ListSessionMetadataParams{
			AppName: app,
			UserID:  user,
			Limit:   10,
			Offset:  0,
		})
		require.NoError(t, err)
		require.Equal(t, 1, listed.Total)
		require.Equal(t, meta, listed.Sessions[0])

		require.NoError(t, s.DeleteMetadata(ctx, app, user, sid))
		after, err := s.ListMetadata(ctx, ListSessionMetadataParams{
			AppName: app,
			UserID:  user,
			Limit:   10,
			Offset:  0,
		})
		require.NoError(t, err)
		require.Equal(t, 0, after.Total)
	})
}
