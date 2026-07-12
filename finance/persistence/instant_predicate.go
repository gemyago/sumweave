package persistence

import (
	"time"

	"gorm.io/gorm"
)

const sqliteDialect = "sqlite"

func instantRangePredicate(db *gorm.DB, column string) string {
	if db.Dialector.Name() == sqliteDialect {
		return "julianday(" + column + ") >= julianday(?) AND julianday(" + column + ") < julianday(?)"
	}
	return column + " >= ? AND " + column + " < ?"
}

func applyInstantAtOrAfter(query *gorm.DB, column string, value time.Time) *gorm.DB {
	if query.Dialector.Name() == sqliteDialect {
		return query.Where("julianday("+column+") >= julianday(?)", value)
	}
	return query.Where(column+" >= ?", value)
}

func applyInstantAtOrBefore(query *gorm.DB, column string, value time.Time) *gorm.DB {
	if query.Dialector.Name() == sqliteDialect {
		return query.Where("julianday("+column+") <= julianday(?)", value)
	}
	return query.Where(column+" <= ?", value)
}

func expiresAfterPredicate(db *gorm.DB) string {
	if db.Dialector.Name() == sqliteDialect {
		return "julianday(expires_at) > julianday(?)"
	}
	return "expires_at > ?"
}
