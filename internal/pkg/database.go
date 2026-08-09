package pkg

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitializeDatabase initializes the database connection using GORM and PostgreSQL.
func InitializeDatabase(dsn string) (*gorm.DB, error) {

	// PreferSimpleProtocol disables the extended query protocol (and with
	// it, named prepared statements) at the pgx driver level. Required
	// when dsn points at a PgBouncer/Supavisor transaction-mode pooler
	// (e.g. Supabase's pooler on :6543) — under transaction pooling, the
	// "connection" GORM thinks it owns can be handed to a different
	// client between queries, so a statement name it prepared earlier
	// can collide with one another client already prepared there,
	// surfacing as "prepared statement ... already exists" (SQLSTATE 42P05).
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger:      logger.Default.LogMode(logger.Warn),
		PrepareStmt: false, // also disable GORM's own statement cache
	})
	if err != nil {
		return nil, fmt.Errorf("db: failed to connect: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("db: failed to get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	return gormDB, nil
}
