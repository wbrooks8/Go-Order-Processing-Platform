// Opens and closes the PostgreSQL connection. Reads DATABASE_DSN from the
// environment, with a local development connection string as the fallback.

package config

import (
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect() (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		dsn = "host=localhost user=app password=localdev dbname=app port=5432 sslmode=disable"
	}

	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	postgresDB, err := db.DB()
	if err != nil {
		return err
	}

	return postgresDB.Close()
}
