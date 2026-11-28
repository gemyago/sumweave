package domain

import (
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestBankAuthorization(t *testing.T) {
	fake := faker.New()
	makeBinding := func() BankAccountBinding {
		return BankAccountBinding{
			CanonicalID: fake.UUID().V4(),
			RemoteID:    fake.UUID().V4(),
			PrimaryHash: fake.UUID().V4(),
			Currency:    fake.Currency().Code(),
			Selected:    true,
		}
	}
	makeAuthorization := func(bindings ...BankAccountBinding) BankAuthorization {
		return BankAuthorization{
			Version:      1,
			ASPSPName:    fake.Company().Name(),
			ASPSPCountry: fake.Address().CountryCode(),
			Accounts:     bindings,
		}
	}
	candidateFor := func(a BankAuthorization, bindings ...BankAccountBinding) BankAuthorization {
		a.Accounts = bindings
		return a
	}
	now := time.Now().Truncate(time.Microsecond)

	t.Run("retains canonical identity across repeated changes before first sync", func(t *testing.T) {
		a := makeBinding()
		original := makeAuthorization(a)
		for range 3 {
			remote := a
			remote.RemoteID, remote.CanonicalID = fake.UUID().V4(), fake.UUID().V4()
			merged, replay, err := original.Merge(candidateFor(original, remote), now, now.AddDate(0, 0, -30))
			require.NoError(t, err)
			require.False(t, replay)
			require.Equal(t, a.CanonicalID, merged.Accounts[0].CanonicalID)
			require.Equal(t, remote.RemoteID, merged.Accounts[0].RemoteID)
			original = merged
		}
	})

	t.Run("keeps interrupted initial replay through deselection other account success and readd", func(t *testing.T) {
		a, b := makeBinding(), makeBinding()
		current := makeAuthorization(b)
		current, replay, err := current.Merge(candidateFor(current, b, a), now, now.AddDate(0, 0, -30))
		require.NoError(t, err)
		require.True(t, replay)
		canonical := current.Accounts[1].CanonicalID
		floor := now.AddDate(-3, 0, 0)
		require.Equal(t, &floor, current.ReplayStart())
		later := now.AddDate(0, 0, 12)
		current, _, err = current.Merge(candidateFor(current, b), later, later.AddDate(0, 0, -30))
		require.NoError(t, err)
		require.False(t, current.Accounts[1].Selected)
		require.Equal(t, &floor, current.Accounts[1].ReplayFrom)
		require.Nil(t, current.ReplayStart())
		b.RemoteID = fake.UUID().V4()
		current, _, err = current.Merge(candidateFor(current, b), later, later.AddDate(0, 0, -30))
		require.NoError(t, err)
		a.RemoteID = fake.UUID().V4()
		current, replay, err = current.Merge(candidateFor(current, b, a), later, later.AddDate(0, 0, -30))
		require.NoError(t, err)
		require.True(t, replay)
		require.Equal(t, canonical, current.Accounts[1].CanonicalID)
		require.Equal(t, &floor, current.ReplayStart())
	})

	t.Run("remembers catchup before connection checkpoint advances", func(t *testing.T) {
		a, b := makeBinding(), makeBinding()
		current := makeAuthorization(a, b)
		catchup := now.AddDate(0, 0, -30)
		current, _, err := current.Merge(candidateFor(current, b), now, catchup)
		require.NoError(t, err)
		later := now.AddDate(0, 2, 0)
		current, _, err = current.Merge(candidateFor(current, a, b), later, later.AddDate(0, 0, -30))
		require.NoError(t, err)
		require.Equal(t, &catchup, current.ReplayStart())
	})

	t.Run("rejects conflicting bank currency duplicate primary and canonical reuse", func(t *testing.T) {
		a := makeBinding()
		current := makeAuthorization(a)
		for _, change := range []func(*BankAuthorization){
			func(c *BankAuthorization) { c.ASPSPName += fake.UUID().V4() },
			func(c *BankAuthorization) { c.Accounts[0].PrimaryHash = "" },
			func(c *BankAuthorization) { c.Accounts[0].RemoteID = "" },
			func(c *BankAuthorization) { c.Version++ },
			func(c *BankAuthorization) { c.Accounts[0].Currency += fake.UUID().V4() },
			func(c *BankAuthorization) { c.Accounts = append(c.Accounts, c.Accounts[0]) },
			func(c *BankAuthorization) {
				c.Accounts[0].PrimaryHash = fake.UUID().V4()
				c.Accounts[0].RemoteID = a.CanonicalID
			},
		} {
			candidate := candidateFor(current, a)
			change(&candidate)
			_, _, err := current.Merge(candidate, now, now.AddDate(0, 0, -30))
			require.ErrorIs(t, err, ErrBankAuthorizationInvalid)
		}
	})
}
