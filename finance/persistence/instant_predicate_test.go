package persistence

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestInstantPredicates(t *testing.T) {
	now := time.Date(2026, time.June, 30, 13, 0, 0, 0, time.FixedZone("test", 2*60*60))

	for _, dialector := range []gorm.Dialector{
		postgres.New(postgres.Config{}),
		sqlite.Open(":memory:"),
	} {
		db := &gorm.DB{Config: &gorm.Config{Dialector: dialector}}
		query := &gorm.DB{
			Config:    db.Config,
			Statement: &gorm.Statement{DB: db, Clauses: map[string]clause.Clause{}},
		}

		assert.NotEmpty(t, instantRangePredicate(db, "effective_at"))
		assert.NotEmpty(t, expiresAfterPredicate(db))
		assert.Same(t, query, applyInstantAtOrAfter(query, "effective_at", now))
		assert.Same(t, query, applyInstantAtOrBefore(query, "effective_at", now))
	}
}
