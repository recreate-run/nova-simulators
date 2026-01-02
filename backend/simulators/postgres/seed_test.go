package postgres_test

import (
	"context"
	"database/sql"
	"testing"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
	simulatorPostgres "github.com/recreate-run/nova-simulators/simulators/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func setupTestDBForSeed(t *testing.T) *database.Queries {
	t.Helper()
	// Use in-memory SQLite database for metadata
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err, "Failed to open test database")

	// Set goose dialect
	err = goose.SetDialect("sqlite3")
	require.NoError(t, err, "Failed to set goose dialect")

	// Run migrations
	err = goose.Up(db, "../../migrations")
	require.NoError(t, err, "Failed to run migrations")

	return database.New(db)
}

// TestPostgresInitialStateSeed demonstrates seeding arbitrary initial state for Postgres simulator
func TestPostgresInitialStateSeed(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDBForSeed(t)
	ctx := context.Background()

	// Setup: Create a new session
	sessionID := "postgres-seed-test-session"
	err := queries.CreateSession(ctx, sessionID)
	require.NoError(t, err, "Failed to create session")

	// Setup: Start embedded Postgres
	postgres := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(5435). // Use different port to avoid conflicts
		Database("test_simulator_seed").
		Username("postgres").
		Password("postgres"))

	err = postgres.Start()
	require.NoError(t, err, "Failed to start embedded Postgres")
	defer func() { _ = postgres.Stop() }()

	connStr := "host=localhost port=5435 user=postgres password=postgres dbname=test_simulator_seed sslmode=disable"

	// Setup: Create Postgres handler
	handler, err := simulatorPostgres.NewHandler(queries, connStr)
	require.NoError(t, err, "Failed to create handler")
	defer func() { _ = handler.Close() }()

	// Seed: Create custom databases, tables, and track metadata
	databases, tables, rows := seedPostgresTestData(t, ctx, queries, handler, sessionID)

	// Verify: Check that metadata is correctly stored
	t.Run("VerifyDatabases", func(t *testing.T) {
		verifyDatabases(t, ctx, queries, sessionID, databases)
	})

	// Verify: Check that tables metadata is correctly stored
	t.Run("VerifyTables", func(t *testing.T) {
		verifyTables(t, ctx, queries, sessionID, tables)
	})

	// Verify: Check that rows are tracked
	t.Run("VerifyRows", func(t *testing.T) {
		verifyRows(t, ctx, queries, sessionID, rows)
	})

	// Verify: Check database isolation - ensure all data is correctly stored
	t.Run("VerifyDatabaseIsolation", func(t *testing.T) {
		verifyDatabaseIsolation(t, ctx, queries, sessionID, databases, tables)
	})
}

// seedPostgresTestData creates databases, tables, and rows for testing
func seedPostgresTestData(t *testing.T, ctx context.Context, queries *database.Queries, handler *simulatorPostgres.Handler, sessionID string) (
	databases []struct {
		Name string
	},
	tables []struct {
		DatabaseName string
		TableName    string
	},
	rows []struct {
		DatabaseName string
		TableName    string
		RowData      string
	},
) {
	t.Helper()

	// Seed: Create custom databases (use session-specific names to avoid conflicts)
	databases = []struct {
		Name string
	}{
		{
			Name: "DB_001_" + sessionID,
		},
		{
			Name: "DB_002_" + sessionID,
		},
	}

	for _, d := range databases {
		err := queries.CreatePostgresDatabase(ctx, database.CreatePostgresDatabaseParams{
			Name:      d.Name,
			SessionID: sessionID,
		})
		require.NoError(t, err, "Failed to create database: %s", d.Name)
	}

	// Seed: Create custom tables (use session-specific names to avoid conflicts)
	tables = []struct {
		DatabaseName string
		TableName    string
	}{
		{
			DatabaseName: databases[0].Name,
			TableName:    "users",
		},
		{
			DatabaseName: databases[0].Name,
			TableName:    "products",
		},
		{
			DatabaseName: databases[1].Name,
			TableName:    "orders",
		},
	}

	for _, tbl := range tables {
		err := queries.CreatePostgresTable(ctx, database.CreatePostgresTableParams{
			DatabaseName: tbl.DatabaseName,
			TableName:    tbl.TableName,
			SessionID:    sessionID,
		})
		require.NoError(t, err, "Failed to create table: %s.%s", tbl.DatabaseName, tbl.TableName)

		// Create columns for each table
		if tbl.TableName == "users" {
			_ = queries.CreatePostgresColumn(ctx, database.CreatePostgresColumnParams{
				DatabaseName:    tbl.DatabaseName,
				TableName:       tbl.TableName,
				ColumnName:      "id",
				DataType:        "integer",
				IsNullable:      "NO",
				ColumnDefault:   sql.NullString{Valid: false},
				OrdinalPosition: 1,
				SessionID:       sessionID,
			})
			_ = queries.CreatePostgresColumn(ctx, database.CreatePostgresColumnParams{
				DatabaseName:    tbl.DatabaseName,
				TableName:       tbl.TableName,
				ColumnName:      "name",
				DataType:        "text",
				IsNullable:      "NO",
				ColumnDefault:   sql.NullString{Valid: false},
				OrdinalPosition: 2,
				SessionID:       sessionID,
			})
			_ = queries.CreatePostgresColumn(ctx, database.CreatePostgresColumnParams{
				DatabaseName:    tbl.DatabaseName,
				TableName:       tbl.TableName,
				ColumnName:      "email",
				DataType:        "text",
				IsNullable:      "YES",
				ColumnDefault:   sql.NullString{Valid: false},
				OrdinalPosition: 3,
				SessionID:       sessionID,
			})
		} else if tbl.TableName == "products" {
			_ = queries.CreatePostgresColumn(ctx, database.CreatePostgresColumnParams{
				DatabaseName:    tbl.DatabaseName,
				TableName:       tbl.TableName,
				ColumnName:      "id",
				DataType:        "integer",
				IsNullable:      "NO",
				ColumnDefault:   sql.NullString{Valid: false},
				OrdinalPosition: 1,
				SessionID:       sessionID,
			})
			_ = queries.CreatePostgresColumn(ctx, database.CreatePostgresColumnParams{
				DatabaseName:    tbl.DatabaseName,
				TableName:       tbl.TableName,
				ColumnName:      "name",
				DataType:        "text",
				IsNullable:      "NO",
				ColumnDefault:   sql.NullString{Valid: false},
				OrdinalPosition: 2,
				SessionID:       sessionID,
			})
		} else if tbl.TableName == "orders" {
			_ = queries.CreatePostgresColumn(ctx, database.CreatePostgresColumnParams{
				DatabaseName:    tbl.DatabaseName,
				TableName:       tbl.TableName,
				ColumnName:      "id",
				DataType:        "integer",
				IsNullable:      "NO",
				ColumnDefault:   sql.NullString{Valid: false},
				OrdinalPosition: 1,
				SessionID:       sessionID,
			})
			_ = queries.CreatePostgresColumn(ctx, database.CreatePostgresColumnParams{
				DatabaseName:    tbl.DatabaseName,
				TableName:       tbl.TableName,
				ColumnName:      "status",
				DataType:        "text",
				IsNullable:      "NO",
				ColumnDefault:   sql.NullString{Valid: false},
				OrdinalPosition: 2,
				SessionID:       sessionID,
			})
		}
	}

	// Seed: Create custom rows (metadata tracking)
	rows = []struct {
		DatabaseName string
		TableName    string
		RowData      string
	}{
		{
			DatabaseName: databases[0].Name,
			TableName:    "users",
			RowData:      `{"id": 1, "name": "Alice", "email": "alice@example.com"}`,
		},
		{
			DatabaseName: databases[0].Name,
			TableName:    "users",
			RowData:      `{"id": 2, "name": "Bob", "email": "bob@example.com"}`,
		},
		{
			DatabaseName: databases[0].Name,
			TableName:    "products",
			RowData:      `{"id": 1, "name": "Widget"}`,
		},
	}

	for _, r := range rows {
		_, err := queries.InsertPostgresRow(ctx, database.InsertPostgresRowParams{
			DatabaseName: r.DatabaseName,
			TableName:    r.TableName,
			RowData:      r.RowData,
			SessionID:    sessionID,
		})
		require.NoError(t, err, "Failed to insert row into %s.%s", r.DatabaseName, r.TableName)
	}

	return databases, tables, rows
}

// verifyDatabases verifies that databases can be queried
func verifyDatabases(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string, databases []struct {
	Name string
}) {
	t.Helper()

	dbList, err := queries.ListPostgresDatabases(ctx, sessionID)
	require.NoError(t, err, "ListPostgresDatabases should succeed")
	assert.Len(t, dbList, len(databases), "Should have correct number of databases")

	// Verify each database
	for _, d := range databases {
		db, err := queries.GetPostgresDatabase(ctx, database.GetPostgresDatabaseParams{
			Name:      d.Name,
			SessionID: sessionID,
		})
		require.NoError(t, err, "GetPostgresDatabase should succeed for database: %s", d.Name)
		assert.Equal(t, d.Name, db.Name, "Database name should match")
	}
}

// verifyTables verifies that tables can be queried
func verifyTables(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string, tables []struct {
	DatabaseName string
	TableName    string
}) {
	t.Helper()

	// Group tables by database
	tablesByDB := make(map[string][]struct {
		DatabaseName string
		TableName    string
	})
	for _, tbl := range tables {
		tablesByDB[tbl.DatabaseName] = append(tablesByDB[tbl.DatabaseName], tbl)
	}

	// Verify tables for each database
	for dbName, expectedTables := range tablesByDB {
		tblList, err := queries.ListPostgresTables(ctx, database.ListPostgresTablesParams{
			DatabaseName: dbName,
			SessionID:    sessionID,
		})
		require.NoError(t, err, "ListPostgresTables should succeed for database: %s", dbName)
		assert.Len(t, tblList, len(expectedTables), "Should have correct number of tables in database: %s", dbName)

		// Verify each table
		for _, tbl := range expectedTables {
			table, err := queries.GetPostgresTable(ctx, database.GetPostgresTableParams{
				DatabaseName: tbl.DatabaseName,
				TableName:    tbl.TableName,
				SessionID:    sessionID,
			})
			require.NoError(t, err, "GetPostgresTable should succeed for table: %s.%s", tbl.DatabaseName, tbl.TableName)
			assert.Equal(t, tbl.TableName, table.TableName, "Table name should match")

			// Verify columns
			columns, err := queries.ListPostgresColumns(ctx, database.ListPostgresColumnsParams{
				DatabaseName: tbl.DatabaseName,
				TableName:    tbl.TableName,
				SessionID:    sessionID,
			})
			require.NoError(t, err, "ListPostgresColumns should succeed for table: %s.%s", tbl.DatabaseName, tbl.TableName)
			assert.GreaterOrEqual(t, len(columns), 2, "Should have at least 2 columns in table: %s.%s", tbl.DatabaseName, tbl.TableName)
		}
	}
}

// verifyRows verifies that rows can be queried
func verifyRows(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string, rows []struct {
	DatabaseName string
	TableName    string
	RowData      string
}) {
	t.Helper()

	// Group rows by table
	rowsByTable := make(map[string]int)
	for _, r := range rows {
		key := r.DatabaseName + "." + r.TableName
		rowsByTable[key]++
	}

	// Verify row counts for each table
	for key, expectedCount := range rowsByTable {
		parts := []string{}
		if len(key) > 0 {
			idx := 0
			for i, c := range key {
				if c == '.' {
					parts = append(parts, key[idx:i])
					idx = i + 1
				}
			}
			parts = append(parts, key[idx:])
		}

		if len(parts) == 2 {
			dbName := parts[0]
			tableName := parts[1]

			count, err := queries.CountPostgresRows(ctx, database.CountPostgresRowsParams{
				DatabaseName: dbName,
				TableName:    tableName,
				SessionID:    sessionID,
			})
			require.NoError(t, err, "CountPostgresRows should succeed for table: %s.%s", dbName, tableName)
			assert.Equal(t, int64(expectedCount), count, "Should have correct number of rows in table: %s.%s", dbName, tableName)

			// List rows
			rowList, err := queries.ListPostgresRows(ctx, database.ListPostgresRowsParams{
				DatabaseName: dbName,
				TableName:    tableName,
				SessionID:    sessionID,
			})
			require.NoError(t, err, "ListPostgresRows should succeed for table: %s.%s", dbName, tableName)
			assert.Len(t, rowList, expectedCount, "Should have correct number of rows in list for table: %s.%s", dbName, tableName)
		}
	}
}

// verifyDatabaseIsolation verifies database isolation
func verifyDatabaseIsolation(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string,
	databases []struct {
		Name string
	},
	tables []struct {
		DatabaseName string
		TableName    string
	}) {
	t.Helper()

	// Query databases from database
	dbList, err := queries.ListPostgresDatabases(ctx, sessionID)
	require.NoError(t, err, "ListPostgresDatabases should succeed")
	assert.Len(t, dbList, len(databases), "Should have correct number of databases in database")

	// Verify database details
	for _, d := range databases {
		db, err := queries.GetPostgresDatabase(ctx, database.GetPostgresDatabaseParams{
			Name:      d.Name,
			SessionID: sessionID,
		})
		require.NoError(t, err, "GetPostgresDatabase should succeed for database: %s", d.Name)
		assert.Equal(t, d.Name, db.Name, "Database name should match in database")
	}

	// Verify no data leaks between sessions - create another session and verify it's empty
	otherSessionID := "other-session"
	err = queries.CreateSession(ctx, otherSessionID)
	require.NoError(t, err, "Failed to create other session")

	otherDBList, err := queries.ListPostgresDatabases(ctx, otherSessionID)
	require.NoError(t, err, "ListPostgresDatabases should succeed for other session")
	assert.Len(t, otherDBList, 0, "Other session should have no databases")
}
