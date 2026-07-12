package gormsignalfoundry

import "gorm.io/gorm"

const sqliteDialectName = "sqlite"

// InstantRangePredicate returns a half-open instant range predicate for the active dialect.
func InstantRangePredicate(db *gorm.DB, column string) string {
	if db.Dialector.Name() == sqliteDialectName {
		return "julianday(" + column + ") >= julianday(?) AND julianday(" + column + ") < julianday(?)"
	}
	return column + " >= ? AND " + column + " < ?"
}

// InstantOnOrAfterPredicate returns an inclusive lower instant bound for the active dialect.
func InstantOnOrAfterPredicate(db *gorm.DB, column string) string {
	if db.Dialector.Name() == sqliteDialectName {
		return "julianday(" + column + ") >= julianday(?)"
	}
	return column + " >= ?"
}

// InstantOnOrBeforePredicate returns an inclusive upper instant bound for the active dialect.
func InstantOnOrBeforePredicate(db *gorm.DB, column string) string {
	if db.Dialector.Name() == sqliteDialectName {
		return "julianday(" + column + ") <= julianday(?)"
	}
	return column + " <= ?"
}
