package auth

import (
	"database/sql"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	"github.com/stretchr/testify/require"
)

func TestStoreValidation(t *testing.T) {
	t.Run("rejects missing dependencies without opening persistence", func(t *testing.T) {
		_, err := NewUserStore(UserStoreDeps{})
		require.ErrorContains(t, err, "user id generator is required")
		_, err = NewUserStore(UserStoreDeps{IDGen: ident.NewDefaultGenerator()})
		require.ErrorContains(t, err, "auth user store logger is required")
		_, err = NewUserStore(UserStoreDeps{
			IDGen: ident.NewDefaultGenerator(), Logger: telemetry.RootTestLogger(),
		})
		require.ErrorContains(t, err, "auth sql database is required")
		_, err = NewRefreshTokenStore(RefreshTokenStoreDeps{})
		require.ErrorContains(t, err, "refresh token store logger is required")
		_, err = NewRefreshTokenStore(RefreshTokenStoreDeps{Logger: telemetry.RootTestLogger()})
		require.ErrorContains(t, err, "auth sql database is required")
	})

	t.Run("validates table-prefix characters", func(t *testing.T) {
		require.Error(t, validateTablePrefix("invalid-prefix-"))
		require.NoError(t, validateTablePrefix("sumweave_auth_"))
		_, err := openAuthDatabase(&sql.DB{}, "", "sumweave_auth_")
		require.ErrorContains(t, err, "auth database dsn is required")
	})
}
