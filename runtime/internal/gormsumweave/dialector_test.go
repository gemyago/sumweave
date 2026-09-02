package gormsumweave

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestNewGormDialectorWithConn(t *testing.T) {
	t.Run("uses postgres dialector for all DSN values when conn is nil", func(t *testing.T) {
		dialector := NewGormDialectorWithConn(":memory:", nil)
		require.Equal(t, "postgres", dialector.Name())
	})

	t.Run("uses postgres dialector for all DSN values", func(t *testing.T) {
		dialector := NewGormDialector(":memory:")
		require.Equal(t, "postgres", dialector.Name())
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
		require.Equal(t, "postgres", db.Dialector.Name())
	})
}

func TestDialectorTranslate(t *testing.T) {
	err := errors.New("dialect error")

	t.Run("postgres dialector forwards translation", func(t *testing.T) {
		dialector := NewGormDialector("postgres://sumweave:secret@example.invalid:5432/sumweave?sslmode=disable")
		require.Same(t, err, dialector.Translate(err))
	})

	t.Run("missing translator preserves the original error", func(t *testing.T) {
		dialector := Dialector{}
		require.Same(t, err, dialector.Translate(err))
	})
}
