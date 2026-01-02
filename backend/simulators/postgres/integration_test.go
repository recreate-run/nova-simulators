package postgres_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
	simulatorPostgres "github.com/recreate-run/nova-simulators/simulators/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *database.Queries {
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

func setupEmbeddedPostgres(t *testing.T) (pg *embeddedpostgres.EmbeddedPostgres, connStr string) {
	t.Helper()

	// Start embedded Postgres on a random port for testing
	postgres := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(5434). // Use different port to avoid conflicts
		Database("test_simulator").
		Username("postgres").
		Password("postgres"))

	err := postgres.Start()
	require.NoError(t, err, "Failed to start embedded Postgres")

	connStr = "host=localhost port=5434 user=postgres password=postgres dbname=test_simulator sslmode=disable"
	return postgres, connStr
}

func TestPostgresSimulatorExec(t *testing.T) {
	// Setup: Create test database and embedded Postgres
	queries := setupTestDB(t)
	postgres, connStr := setupEmbeddedPostgres(t)
	defer func() { _ = postgres.Stop() }()

	// Setup: Create Postgres handler
	handler, err := simulatorPostgres.NewHandler(queries, connStr)
	require.NoError(t, err, "Failed to create handler")
	defer func() { _ = handler.Close() }()

	// Setup: Create test session
	sessionID := "postgres-test-session-1"

	// Setup: Start simulator server with session middleware
	wrappedHandler := session.Middleware(handler)
	server := httptest.NewServer(wrappedHandler)
	defer server.Close()

	ctx := context.Background()
	_ = ctx

	t.Run("CreateTable", func(t *testing.T) {
		// Create a table using exec endpoint
		reqBody := map[string]interface{}{
			"sql": "CREATE TABLE users (id SERIAL PRIMARY KEY, name TEXT NOT NULL, email TEXT)",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/exec", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-ID", sessionID)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Request should not error")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

		var execResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&execResp)
		require.NoError(t, err, "Should decode response")
	})

	t.Run("InsertData", func(t *testing.T) {
		// Insert data using exec endpoint
		reqBody := map[string]interface{}{
			"sql": "INSERT INTO users (name, email) VALUES ('Alice', 'alice@example.com'), ('Bob', 'bob@example.com')",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/exec", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-ID", sessionID)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Request should not error")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

		var execResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&execResp)
		require.NoError(t, err, "Should decode response")
		assert.InDelta(t, float64(2), execResp["rows_affected"], 0.01, "Should insert 2 rows")
	})
}

func TestPostgresSimulatorQuery(t *testing.T) {
	// Setup: Create test database and embedded Postgres
	queries := setupTestDB(t)
	postgres, connStr := setupEmbeddedPostgres(t)
	defer func() { _ = postgres.Stop() }()

	// Setup: Create Postgres handler
	handler, err := simulatorPostgres.NewHandler(queries, connStr)
	require.NoError(t, err, "Failed to create handler")
	defer func() { _ = handler.Close() }()

	// Setup: Create test session
	sessionID := "postgres-test-session-2"

	// Setup: Start simulator server with session middleware
	wrappedHandler := session.Middleware(handler)
	server := httptest.NewServer(wrappedHandler)
	defer server.Close()

	ctx := context.Background()

	// Setup: Create table and insert data
	createReq := map[string]interface{}{
		"sql": "CREATE TABLE products (id SERIAL PRIMARY KEY, name TEXT, price DECIMAL(10,2))",
	}
	jsonBody, _ := json.Marshal(createReq)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/exec", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-ID", sessionID)
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()

	insertReq := map[string]interface{}{
		"sql": "INSERT INTO products (name, price) VALUES ('Laptop', 999.99), ('Mouse', 29.99), ('Keyboard', 79.99)",
	}
	jsonBody, _ = json.Marshal(insertReq)
	req, _ = http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/exec", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-ID", sessionID)
	resp, _ = http.DefaultClient.Do(req)
	_ = resp.Body.Close()

	t.Run("SelectAll", func(t *testing.T) {
		// Query all products
		reqBody := map[string]interface{}{
			"sql": "SELECT id, name, price FROM products ORDER BY id",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/query", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-ID", sessionID)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Request should not error")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

		var queryResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&queryResp)
		require.NoError(t, err, "Should decode response")

		rows, ok := queryResp["rows"].([]interface{})
		require.True(t, ok, "rows should be []interface{}")
		assert.Len(t, rows, 3, "Should return 3 rows")

		firstRow, ok := rows[0].(map[string]interface{})
		require.True(t, ok, "row should be map[string]interface{}")
		assert.Equal(t, "Laptop", firstRow["name"], "First product should be Laptop")
	})

	t.Run("SelectFiltered", func(t *testing.T) {
		// Query products with price > 50
		reqBody := map[string]interface{}{
			"sql": "SELECT name, price FROM products WHERE price > 50 ORDER BY price DESC",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/query", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-ID", sessionID)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Request should not error")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

		var queryResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&queryResp)
		require.NoError(t, err, "Should decode response")

		rows, ok := queryResp["rows"].([]interface{})
		require.True(t, ok, "rows should be []interface{}")
		assert.Len(t, rows, 2, "Should return 2 rows (Laptop and Keyboard)")

		firstRow, ok := rows[0].(map[string]interface{})
		require.True(t, ok, "row should be map[string]interface{}")
		assert.Equal(t, "Laptop", firstRow["name"], "First product should be Laptop")
	})
}

func TestPostgresSimulatorSchema(t *testing.T) {
	// Setup: Create test database and embedded Postgres
	queries := setupTestDB(t)
	postgres, connStr := setupEmbeddedPostgres(t)
	defer func() { _ = postgres.Stop() }()

	// Setup: Create Postgres handler
	handler, err := simulatorPostgres.NewHandler(queries, connStr)
	require.NoError(t, err, "Failed to create handler")
	defer func() { _ = handler.Close() }()

	// Setup: Create test session
	sessionID := "postgres-test-session-3"

	// Setup: Start simulator server with session middleware
	wrappedHandler := session.Middleware(handler)
	server := httptest.NewServer(wrappedHandler)
	defer server.Close()

	ctx := context.Background()

	// Setup: Create tables
	createReq := map[string]interface{}{
		"sql": "CREATE TABLE customers (id SERIAL PRIMARY KEY, name TEXT NOT NULL, email TEXT UNIQUE)",
	}
	jsonBody, _ := json.Marshal(createReq)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/exec", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-ID", sessionID)
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()

	createReq2 := map[string]interface{}{
		"sql": "CREATE TABLE orders (id SERIAL PRIMARY KEY, customer_id INTEGER, amount DECIMAL(10,2), created_at TIMESTAMP DEFAULT NOW())",
	}
	jsonBody, _ = json.Marshal(createReq2)
	req, _ = http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/exec", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-ID", sessionID)
	resp, _ = http.DefaultClient.Do(req)
	_ = resp.Body.Close()

	t.Run("GetSchema", func(t *testing.T) {
		// Get schema
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/schema", http.NoBody)
		req.Header.Set("X-Session-ID", sessionID)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Request should not error")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

		var schemaResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&schemaResp)
		require.NoError(t, err, "Should decode response")

		columns, ok := schemaResp["columns"].([]interface{})
		require.True(t, ok, "columns should be []interface{}")
		assert.NotEmpty(t, columns, "Should have columns")

		// Verify we have columns from both tables
		hasCustomersTable := false
		hasOrdersTable := false
		for _, col := range columns {
			colMap, ok := col.(map[string]interface{})
			require.True(t, ok, "column should be map[string]interface{}")
			tableName, ok := colMap["table_name"].(string)
			require.True(t, ok, "table_name should be string")
			if tableName == "customers" {
				hasCustomersTable = true
			}
			if tableName == "orders" {
				hasOrdersTable = true
			}
		}
		assert.True(t, hasCustomersTable, "Should have customers table in schema")
		assert.True(t, hasOrdersTable, "Should have orders table in schema")
	})
}

func TestPostgresSimulatorSessionIsolation(t *testing.T) {
	// Setup: Create test database and embedded Postgres
	queries := setupTestDB(t)
	postgres, connStr := setupEmbeddedPostgres(t)
	defer func() { _ = postgres.Stop() }()

	// Setup: Create Postgres handler
	handler, err := simulatorPostgres.NewHandler(queries, connStr)
	require.NoError(t, err, "Failed to create handler")
	defer func() { _ = handler.Close() }()

	// Setup: Create two test sessions
	sessionID1 := "postgres-test-session-4a"
	sessionID2 := "postgres-test-session-4b"

	// Setup: Start simulator server with session middleware
	wrappedHandler := session.Middleware(handler)
	server := httptest.NewServer(wrappedHandler)
	defer server.Close()

	ctx := context.Background()

	// Session 1: Create table and insert data
	createReq := map[string]interface{}{
		"sql": "CREATE TABLE items (id SERIAL PRIMARY KEY, name TEXT)",
	}
	jsonBody, _ := json.Marshal(createReq)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/exec", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-ID", sessionID1)
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()

	insertReq := map[string]interface{}{
		"sql": "INSERT INTO items (name) VALUES ('Item from Session 1')",
	}
	jsonBody, _ = json.Marshal(insertReq)
	req, _ = http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/exec", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-ID", sessionID1)
	resp, _ = http.DefaultClient.Do(req)
	_ = resp.Body.Close()

	// Session 2: Create table and insert different data
	jsonBody, _ = json.Marshal(createReq)
	req, _ = http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/exec", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-ID", sessionID2)
	resp, _ = http.DefaultClient.Do(req)
	_ = resp.Body.Close()

	insertReq2 := map[string]interface{}{
		"sql": "INSERT INTO items (name) VALUES ('Item from Session 2')",
	}
	jsonBody, _ = json.Marshal(insertReq2)
	req, _ = http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/exec", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-ID", sessionID2)
	resp, _ = http.DefaultClient.Do(req)
	_ = resp.Body.Close()

	t.Run("Session1Data", func(t *testing.T) {
		// Query from session 1
		queryReq := map[string]interface{}{
			"sql": "SELECT name FROM items",
		}
		jsonBody, _ := json.Marshal(queryReq)
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/query", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-ID", sessionID1)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Request should not error")
		defer func() { _ = resp.Body.Close() }()

		var queryResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&queryResp)
		require.NoError(t, err, "Should decode response")

		rows, ok := queryResp["rows"].([]interface{})
		require.True(t, ok, "rows should be []interface{}")
		assert.Len(t, rows, 1, "Session 1 should only see its own data")
		firstRow, ok := rows[0].(map[string]interface{})
		require.True(t, ok, "row should be map[string]interface{}")
		assert.Equal(t, "Item from Session 1", firstRow["name"], "Should see session 1 data")
	})

	t.Run("Session2Data", func(t *testing.T) {
		// Query from session 2
		queryReq := map[string]interface{}{
			"sql": "SELECT name FROM items",
		}
		jsonBody, _ := json.Marshal(queryReq)
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/query", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-ID", sessionID2)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Request should not error")
		defer func() { _ = resp.Body.Close() }()

		var queryResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&queryResp)
		require.NoError(t, err, "Should decode response")

		rows, ok := queryResp["rows"].([]interface{})
		require.True(t, ok, "rows should be []interface{}")
		assert.Len(t, rows, 1, "Session 2 should only see its own data")
		firstRow, ok := rows[0].(map[string]interface{})
		require.True(t, ok, "row should be map[string]interface{}")
		assert.Equal(t, "Item from Session 2", firstRow["name"], "Should see session 2 data")
	})
}

func TestPostgresSimulatorUpdate(t *testing.T) {
	// Setup: Create test database and embedded Postgres
	queries := setupTestDB(t)
	postgres, connStr := setupEmbeddedPostgres(t)
	defer func() { _ = postgres.Stop() }()

	// Setup: Create Postgres handler
	handler, err := simulatorPostgres.NewHandler(queries, connStr)
	require.NoError(t, err, "Failed to create handler")
	defer func() { _ = handler.Close() }()

	// Setup: Create test session
	sessionID := "postgres-test-session-5"

	// Setup: Start simulator server with session middleware
	wrappedHandler := session.Middleware(handler)
	server := httptest.NewServer(wrappedHandler)
	defer server.Close()

	ctx := context.Background()

	// Setup: Create table and insert data
	createReq := map[string]interface{}{
		"sql": "CREATE TABLE accounts (id SERIAL PRIMARY KEY, balance DECIMAL(10,2), active BOOLEAN DEFAULT true)",
	}
	jsonBody, _ := json.Marshal(createReq)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/exec", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-ID", sessionID)
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()

	insertReq := map[string]interface{}{
		"sql": "INSERT INTO accounts (balance, active) VALUES (100.00, true), (200.00, true), (300.00, false)",
	}
	jsonBody, _ = json.Marshal(insertReq)
	req, _ = http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/exec", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-ID", sessionID)
	resp, _ = http.DefaultClient.Do(req)
	_ = resp.Body.Close()

	t.Run("UpdateRows", func(t *testing.T) {
		// Update active accounts
		updateReq := map[string]interface{}{
			"sql": "UPDATE accounts SET balance = balance * 1.1 WHERE active = true",
		}
		jsonBody, _ := json.Marshal(updateReq)

		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/exec", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-ID", sessionID)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Request should not error")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

		var execResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&execResp)
		require.NoError(t, err, "Should decode response")
		assert.InDelta(t, float64(2), execResp["rows_affected"], 0.01, "Should update 2 rows")
	})

	t.Run("VerifyUpdate", func(t *testing.T) {
		// Query to verify update
		queryReq := map[string]interface{}{
			"sql": "SELECT id, balance, active FROM accounts ORDER BY id",
		}
		jsonBody, _ := json.Marshal(queryReq)

		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/query", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-ID", sessionID)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Request should not error")
		defer func() { _ = resp.Body.Close() }()

		var queryResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&queryResp)
		require.NoError(t, err, "Should decode response")

		rows, ok := queryResp["rows"].([]interface{})
		require.True(t, ok, "rows should be []interface{}")
		assert.Len(t, rows, 3, "Should have 3 accounts")

		// First two should be updated (110.00, 220.00)
		// Third should be unchanged (300.00)
		firstRow, ok := rows[0].(map[string]interface{})
		require.True(t, ok, "row should be map[string]interface{}")
		assert.Equal(t, "110.00", firstRow["balance"], "First account should be updated")

		thirdRow, ok := rows[2].(map[string]interface{})
		require.True(t, ok, "row should be map[string]interface{}")
		assert.Equal(t, "300.00", thirdRow["balance"], "Third account should be unchanged")
	})
}
