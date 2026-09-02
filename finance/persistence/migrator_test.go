package persistence

import (
	"errors"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMigrator(t *testing.T) {
	makeMigrator := func(t *testing.T) (*Migrator, *mockautoMigrator) {
		t.Helper()

		autoMigrator := newMockautoMigrator(t)
		return &Migrator{autoMigrator: autoMigrator}, autoMigrator
	}

	t.Run("wraps finance schema migration errors", func(t *testing.T) {
		migrator, autoMigrator := makeMigrator(t)
		migrationErr := errors.New(faker.New().Lorem().Sentence(3))
		autoMigrator.EXPECT().AutoMigrate(mock.Anything, mock.Anything).Return(migrationErr).Once()

		err := migrator.Migrate(t.Context())

		require.ErrorIs(t, err, migrationErr)
		require.ErrorContains(t, err, "auto-migrate finance schema")
	})

	t.Run("wraps current observation migration errors", func(t *testing.T) {
		migrator, autoMigrator := makeMigrator(t)
		migrationErr := errors.New(faker.New().Lorem().Sentence(3))
		autoMigrator.EXPECT().AutoMigrate(mock.Anything, mock.Anything).Return(nil).Once()
		autoMigrator.EXPECT().AutoMigrate(mock.Anything, mock.Anything).Return(migrationErr).Once()

		err := migrator.Migrate(t.Context())

		require.ErrorIs(t, err, migrationErr)
		require.ErrorContains(t, err, "auto-migrate finance current observations")
	})
}
