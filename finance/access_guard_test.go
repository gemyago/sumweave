package finance

import (
	"errors"
	"testing"

	"github.com/jaswdr/faker/v2"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAccessGuard(t *testing.T) {
	makeIDs := func() (string, string) {
		fake := faker.New()
		return "tenant-" + fake.UUID().V4(), "user-" + fake.UUID().V4()
	}

	t.Run("allows tenant members", func(t *testing.T) {
		tenantID, userID := makeIDs()
		store := newMockaccessGuardStore(t)
		store.EXPECT().IsTenantMember(testifymock.Anything, tenantID, userID).Return(true, nil)
		guard := newAccessGuard(store)

		err := guard.requireTenantMember(t.Context(), tenantID, userID)

		require.NoError(t, err)
	})

	t.Run("denies non members", func(t *testing.T) {
		tenantID, userID := makeIDs()
		store := newMockaccessGuardStore(t)
		store.EXPECT().IsTenantMember(testifymock.Anything, tenantID, userID).Return(false, nil)
		guard := newAccessGuard(store)

		err := guard.requireTenantMember(t.Context(), tenantID, userID)

		require.ErrorIs(t, err, ErrTenantAccessDenied)
	})

	t.Run("wraps membership lookup errors", func(t *testing.T) {
		tenantID, userID := makeIDs()
		lookupErr := errors.New("lookup failed")
		store := newMockaccessGuardStore(t)
		store.EXPECT().IsTenantMember(testifymock.Anything, tenantID, userID).Return(false, lookupErr)
		guard := newAccessGuard(store)

		err := guard.requireTenantMember(t.Context(), tenantID, userID)

		require.ErrorIs(t, err, lookupErr)
		require.ErrorContains(t, err, "check tenant membership")
	})
}
