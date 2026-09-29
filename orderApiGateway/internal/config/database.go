package config

import (
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect() (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		dsn = "host=localhost user=app password=localdev dbname=simplerest port=5432 sslmode=disable"
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
