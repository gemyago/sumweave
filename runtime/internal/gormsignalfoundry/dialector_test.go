package gormsignalfoundry

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewGormDialectorWithConn(t *testing.T) {
	t.Run("falls back to dsn-based dialector when conn is nil", func(t *testing.T) {
		dialector := NewGormDialectorWithConn(":memory:", nil)
		require.NotNil(t, dialector.Dialector)
	})

	t.Run("uses provided postgres connection", func(t *testing.T) {
		conn := &sql.DB{}
		dialector := NewGormDialectorWithConn(
			"postgres://signal-foundry:secret@example.invalid:5432/signal_foundry?sslmode=disable",
			conn,
		)
		require.NotNil(t, dialector.Dialector)
	})
}
