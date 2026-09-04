package persistence

import (
	"time"

	"gorm.io/gorm"
)

func instantRangePredicate(column string) string {
	return column + " >= ? AND " + column + " < ?"
}

func applyInstantAtOrAfter(query *gorm.DB, column string, value time.Time) *gorm.DB {
	return query.Where(column+" >= ?", value)
}

func applyInstantAtOrBefore(query *gorm.DB, column string, value time.Time) *gorm.DB {
	return query.Where(column+" <= ?", value)
}

func expiresAfterPredicate() string {
	return "expires_at > ?"
}
