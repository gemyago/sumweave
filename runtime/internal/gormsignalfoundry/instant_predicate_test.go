package gormsignalfoundry

import (
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInstantPredicates(t *testing.T) {
	fake := faker.New()
	db, err := gorm.Open(NewGormDialector(":memory:"), &gorm.Config{DryRun: true})
	require.NoError(t, err)

	column := fake.Lorem().Word()
	require.Equal(t,
		"julianday("+column+") >= julianday(?) AND julianday("+column+") < julianday(?)",
		InstantRangePredicate(db, column),
	)
	require.Equal(t,
		"julianday("+column+") >= julianday(?)",
		InstantOnOrAfterPredicate(db, column),
	)
	require.Equal(t,
		"julianday("+column+") <= julianday(?)",
		InstantOnOrBeforePredicate(db, column),
	)
}
