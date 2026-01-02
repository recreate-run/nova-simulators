package database

import (
	"context"
	"database/sql"
	"sync"

	"github.com/pressly/goose/v3"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

var (
	db      *sql.DB
	queries *Queries
	once    sync.Once
	errInit error
)

// InitDB initializes the shared libSQL database connection
func InitDB(dbPath string) error {
	once.Do(func() {
		db, errInit = sql.Open("libsql", dbPath)
		if errInit != nil {
			return
		}

		// Test connection
		errInit = db.PingContext(context.Background())
		if errInit != nil {
			return
		}

		// Set connection pool settings
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)

		// Run goose migrations
		if err := goose.SetDialect("sqlite3"); err != nil {
			errInit = err
			return
		}
		if err := goose.Up(db, "migrations"); err != nil {
			errInit = err
			return
		}

		// Initialize sqlc queries
		queries = New(db)
	})
	return errInit
}

// GetDB returns the shared database connection
func GetDB() *sql.DB {
	return db
}

// GetQueries returns the sqlc Queries instance
func GetQueries() *Queries {
	return queries
}

// Close closes the shared database connection
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}
