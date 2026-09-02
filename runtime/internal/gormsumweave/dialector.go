package gormsumweave

import (
	"database/sql"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Dialector wraps a concrete GORM dialector without exposing the interface as a return type.
type Dialector struct {
	gorm.Dialector
}

// NewGormDialector returns the PostgreSQL GORM dialector for the given DSN.
func NewGormDialector(dsn string) Dialector {
	return Dialector{Dialector: postgres.Open(dsn)}
}

// NewGormDialectorWithConn returns the PostgreSQL GORM dialector for the given DSN and connection pool.
func NewGormDialectorWithConn(dsn string, conn *sql.DB) Dialector {
	if conn == nil {
		return NewGormDialector(dsn)
	}
	return Dialector{Dialector: postgres.New(postgres.Config{DSN: dsn, Conn: conn})}
}

// Translate forwards error translation when the wrapped dialector supports it.
func (d Dialector) Translate(err error) error {
	if translator, ok := d.Dialector.(gorm.ErrorTranslator); ok {
		return translator.Translate(err)
	}
	return err
}
