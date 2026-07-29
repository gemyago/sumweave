package gormsumweave

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestNewGormDialectorWithConn(t *testing.T) {
	t.Run("falls back to dsn-based dialector when conn is nil", func(t *testing.T) {
		dialector := NewGormDialectorWithConn(":memory:", nil)
		require.NotNil(t, dialector.Dialector)
	})

	t.Run("uses provided sqlite connection", func(t *testing.T) {
		baseDB, err := gorm.Open(NewGormDialector(":memory:"), &gorm.Config{})
		require.NoError(t, err)

		conn, err := baseDB.DB()
		require.NoError(t, err)
		defer func() { require.NoError(t, conn.Close()) }()

		dialector := NewGormDialectorWithConn(":memory:", conn)
		require.Equal(t, "sqlite", dialector.Name())

		db, err := gorm.Open(dialector, &gorm.Config{DisableAutomaticPing: true, DryRun: true})
		require.NoError(t, err)
		require.Equal(t, "sqlite", db.Dialector.Name())
	})

	t.Run("uses provided postgres connection", func(t *testing.T) {
		conn := &sql.DB{}
		dialector := NewGormDialectorWithConn(
			"postgres://sumweave:secret@example.invalid:5432/sumweave?sslmode=disable",
			conn,
		)
		require.NotNil(t, dialector.Dialector)

		db, err := gorm.Open(dialector, &gorm.Config{DisableAutomaticPing: true, DryRun: true})
		require.NoError(t, err)
		now := time.Now()
		query := db.Table("events").
			Where(InstantRangePredicate(db, "event_time"), now, now.Add(time.Hour)).
			Where(InstantOnOrAfterPredicate(db, "start_at"), now).
			Where(InstantOnOrBeforePredicate(db, "end_at"), now.Add(time.Hour)).
			Find(&[]struct{}{})
		require.NoError(t, query.Error)
	})
}

func TestDialectorTranslate(t *testing.T) {
	dialector := NewGormDialector(":memory:")
	err := errors.New("dialect error")

	require.Same(t, err, dialector.Translate(err))
}
