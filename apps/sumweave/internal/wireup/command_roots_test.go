package wireup

import (
	"errors"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestCommandRoots(t *testing.T) {
	fake := faker.New()

	t.Run("rejects missing application database settings before construction", func(t *testing.T) {
		_, err := BuildUsers(UsersOptions{Environment: fake.UUID().V4()})
		require.Error(t, err)
		_, err = BuildFinanceFixtures(FinanceFixturesOptions{Environment: fake.UUID().V4()})
		require.Error(t, err)
	})

	t.Run("closes command roots without storage", func(t *testing.T) {
		expectedErr := errors.New(fake.Lorem().Sentence(3))
		usersRoot := &UsersRoot{closeDatabase: func() error { return expectedErr }}
		financeRoot := &FinanceFixturesRoot{closeDatabase: func() error { return expectedErr }}
		require.ErrorIs(t, usersRoot.Close(), expectedErr)
		require.ErrorIs(t, financeRoot.Close(), expectedErr)
	})
}
