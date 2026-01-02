package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	_ "github.com/lib/pq"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
)

// Handler implements the Postgres simulator HTTP handler
type Handler struct {
	queries  *database.Queries
	pgDB     *sql.DB
	mu       sync.RWMutex
	sessions map[string]bool // Track created session schemas
}

// NewHandler creates a new Postgres simulator handler
func NewHandler(queries *database.Queries, pgConnStr string) (*Handler, error) {
	// Connect to embedded Postgres instance
	db, err := sql.Open("postgres", pgConnStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return &Handler{
		queries:  queries,
		pgDB:     db,
		sessions: make(map[string]bool),
	}, nil
}

// ServeHTTP implements http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[postgres] → %s %s", r.Method, r.URL.Path)

	// Route Postgres API requests
	// Expected paths:
	// POST /query - Execute SELECT query
	// POST /exec - Execute INSERT/UPDATE/DELETE
	// GET /schema - Get database schema
	// POST /seed - Seed test data
	// DELETE /session - Clean up session data

	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 {
		http.NotFound(w, r)
		return
	}

	switch parts[0] {
	case "query":
		h.handleQuery(w, r)
	case "exec":
		h.handleExec(w, r)
	case "schema":
		h.handleGetSchema(w, r)
	case "seed":
		h.handleSeed(w, r)
	case "session":
		if r.Method == http.MethodDelete {
			h.handleDeleteSession(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	default:
		http.NotFound(w, r)
	}
}

// Query request/response structures
type QueryRequest struct {
	SQL      string `json:"sql"`
	Database string `json:"database,omitempty"`
}

type QueryResponse struct {
	Rows []map[string]interface{} `json:"rows"`
}

type ExecRequest struct {
	SQL      string `json:"sql"`
	Database string `json:"database,omitempty"`
}

type ExecResponse struct {
	RowsAffected int64 `json:"rows_affected"`
}

type SchemaResponse struct {
	Columns []SchemaColumn `json:"columns"`
}

type SchemaColumn struct {
	TableName     string  `json:"table_name"`
	ColumnName    string  `json:"column_name"`
	DataType      string  `json:"data_type"`
	IsNullable    string  `json:"is_nullable"`
	ColumnDefault *string `json:"column_default"`
}

// ensureSessionSchema creates a schema for the session if it doesn't exist
func (h *Handler) ensureSessionSchema(ctx context.Context, sessionID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.sessions[sessionID] {
		return nil
	}

	// Create schema for this session
	schemaName := fmt.Sprintf("session_%s", sanitizeIdentifier(sessionID))
	query := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schemaName)

	_, err := h.pgDB.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	h.sessions[sessionID] = true
	log.Printf("[postgres] ✓ Created schema: %s", schemaName)
	return nil
}

// getSchemaName returns the schema name for a session
func getSchemaName(sessionID string) string {
	return fmt.Sprintf("session_%s", sanitizeIdentifier(sessionID))
}

// sanitizeIdentifier sanitizes a string to be used as a PostgreSQL identifier
func sanitizeIdentifier(s string) string {
	// Remove any characters that aren't alphanumeric or underscore
	reg := regexp.MustCompile("[^a-zA-Z0-9_]")
	return reg.ReplaceAllString(s, "_")
}

// executeInSession executes SQL in the context of a session schema
func (h *Handler) executeInSession(ctx context.Context, sessionID, sqlQuery string) ([]map[string]interface{}, error) {
	// Ensure session schema exists
	if err := h.ensureSessionSchema(ctx, sessionID); err != nil {
		return nil, err
	}

	schemaName := getSchemaName(sessionID)

	// Set search_path to session schema and execute query
	tx, err := h.pgDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Set search_path for this transaction
	_, err = tx.ExecContext(ctx, fmt.Sprintf("SET search_path TO %s", schemaName))
	if err != nil {
		return nil, fmt.Errorf("failed to set search_path: %w", err)
	}

	// Execute the query
	rows, err := tx.QueryContext(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	// Scan results
	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return results, nil
}

// execInSession executes a write SQL statement in the context of a session schema
func (h *Handler) execInSession(ctx context.Context, sessionID, sqlQuery string) (int64, error) {
	// Ensure session schema exists
	if err := h.ensureSessionSchema(ctx, sessionID); err != nil {
		return 0, err
	}

	schemaName := getSchemaName(sessionID)

	// Set search_path to session schema and execute query
	tx, err := h.pgDB.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Set search_path for this transaction
	_, err = tx.ExecContext(ctx, fmt.Sprintf("SET search_path TO %s", schemaName))
	if err != nil {
		return 0, fmt.Errorf("failed to set search_path: %w", err)
	}

	// Execute the statement
	result, err := tx.ExecContext(ctx, sqlQuery)
	if err != nil {
		return 0, fmt.Errorf("exec failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Log the query
	queryType := detectQueryType(sqlQuery)
	_ = h.queries.LogPostgresQuery(context.Background(), database.LogPostgresQueryParams{
		DatabaseName: "default",
		QueryText:    sqlQuery,
		QueryType:    queryType,
		RowsAffected: sql.NullInt64{Int64: rowsAffected, Valid: true},
		SessionID:    sessionID,
	})

	return rowsAffected, nil
}

// detectQueryType detects the type of SQL query
func detectQueryType(sqlQuery string) string {
	upperSQL := strings.ToUpper(strings.TrimSpace(sqlQuery))
	switch {
	case strings.HasPrefix(upperSQL, "SELECT"):
		return "SELECT"
	case strings.HasPrefix(upperSQL, "INSERT"):
		return "INSERT"
	case strings.HasPrefix(upperSQL, "UPDATE"):
		return "UPDATE"
	case strings.HasPrefix(upperSQL, "DELETE"):
		return "DELETE"
	case strings.HasPrefix(upperSQL, "CREATE"):
		return "CREATE"
	case strings.HasPrefix(upperSQL, "DROP"):
		return "DROP"
	case strings.HasPrefix(upperSQL, "ALTER"):
		return "ALTER"
	default:
		return "OTHER"
	}
}

func (h *Handler) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := session.FromContext(r.Context())
	ctx := context.Background()

	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SQL == "" {
		http.Error(w, "SQL query is required", http.StatusBadRequest)
		return
	}

	// Execute query
	results, err := h.executeInSession(ctx, sessionID, req.SQL)
	if err != nil {
		log.Printf("[postgres] ✗ Query failed: %v", err)
		http.Error(w, fmt.Sprintf("Query failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Log the query
	_ = h.queries.LogPostgresQuery(ctx, database.LogPostgresQueryParams{
		DatabaseName: "default",
		QueryText:    req.SQL,
		QueryType:    "SELECT",
		RowsAffected: sql.NullInt64{Int64: int64(len(results)), Valid: true},
		SessionID:    sessionID,
	})

	response := QueryResponse{Rows: results}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[postgres] ✓ Query executed: %d rows returned", len(results))
}

func (h *Handler) handleExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := session.FromContext(r.Context())
	ctx := context.Background()

	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SQL == "" {
		http.Error(w, "SQL statement is required", http.StatusBadRequest)
		return
	}

	// Execute statement
	rowsAffected, err := h.execInSession(ctx, sessionID, req.SQL)
	if err != nil {
		log.Printf("[postgres] ✗ Exec failed: %v", err)
		http.Error(w, fmt.Sprintf("Exec failed: %v", err), http.StatusInternalServerError)
		return
	}

	response := ExecResponse{RowsAffected: rowsAffected}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[postgres] ✓ Exec completed: %d rows affected", rowsAffected)
}

func (h *Handler) handleGetSchema(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := session.FromContext(r.Context())
	ctx := context.Background()

	// Ensure session schema exists
	if err := h.ensureSessionSchema(ctx, sessionID); err != nil {
		http.Error(w, "Failed to initialize session", http.StatusInternalServerError)
		return
	}

	schemaName := getSchemaName(sessionID)

	// Query information_schema for tables in this schema
	query := `
		SELECT
			c.table_name,
			c.column_name,
			c.data_type,
			c.is_nullable,
			c.column_default
		FROM information_schema.columns c
		WHERE c.table_schema = $1
		ORDER BY c.table_name, c.ordinal_position
	`

	rows, err := h.pgDB.QueryContext(ctx, query, schemaName)
	if err != nil {
		log.Printf("[postgres] ✗ Schema query failed: %v", err)
		http.Error(w, "Failed to get schema", http.StatusInternalServerError)
		return
	}
	defer func() { _ = rows.Close() }()

	var columns []SchemaColumn
	for rows.Next() {
		var col SchemaColumn
		var colDefault sql.NullString

		err := rows.Scan(&col.TableName, &col.ColumnName, &col.DataType, &col.IsNullable, &colDefault)
		if err != nil {
			log.Printf("[postgres] ✗ Failed to scan schema row: %v", err)
			continue
		}

		if colDefault.Valid {
			col.ColumnDefault = &colDefault.String
		}

		columns = append(columns, col)
	}

	if err := rows.Err(); err != nil {
		log.Printf("[postgres] ✗ Rows iteration error: %v", err)
		http.Error(w, "Failed to get schema", http.StatusInternalServerError)
		return
	}

	response := SchemaResponse{Columns: columns}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[postgres] ✓ Schema returned: %d columns", len(columns))
}

type SeedRequest struct {
	Tables []TableSeed `json:"tables"`
}

type TableSeed struct {
	Name    string                   `json:"name"`
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
}

func (h *Handler) handleSeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := session.FromContext(r.Context())
	ctx := context.Background()

	var req SeedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Ensure session schema exists
	if err := h.ensureSessionSchema(ctx, sessionID); err != nil {
		http.Error(w, "Failed to initialize session", http.StatusInternalServerError)
		return
	}

	schemaName := getSchemaName(sessionID)

	// Begin transaction for seeding
	tx, err := h.pgDB.BeginTx(ctx, nil)
	if err != nil {
		http.Error(w, "Failed to begin transaction", http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Set search_path
	_, err = tx.ExecContext(ctx, fmt.Sprintf("SET search_path TO %s", schemaName))
	if err != nil {
		http.Error(w, "Failed to set search_path", http.StatusInternalServerError)
		return
	}

	totalRows := 0
	for _, table := range req.Tables {
		for _, rowData := range table.Rows {
			// Build INSERT statement
			columns := []string{}
			placeholders := []string{}
			values := []interface{}{}
			i := 1

			for col, val := range rowData {
				columns = append(columns, col)
				placeholders = append(placeholders, "$"+strconv.Itoa(i))
				values = append(values, val)
				i++
			}

			// #nosec G201 -- Table and column names from structured seed request, values are parameterized
			insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
				table.Name,
				strings.Join(columns, ", "),
				strings.Join(placeholders, ", "))

			_, err := tx.ExecContext(ctx, insertSQL, values...)
			if err != nil {
				log.Printf("[postgres] ✗ Failed to insert row: %v", err)
				http.Error(w, fmt.Sprintf("Failed to insert row: %v", err), http.StatusInternalServerError)
				return
			}
			totalRows++
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"rows_inserted": totalRows,
	})
	log.Printf("[postgres] ✓ Seeded %d rows across %d tables", totalRows, len(req.Tables))
}

func (h *Handler) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID := session.FromContext(r.Context())
	ctx := context.Background()

	schemaName := getSchemaName(sessionID)

	// Drop the schema
	query := fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName)
	_, err := h.pgDB.ExecContext(ctx, query)
	if err != nil {
		log.Printf("[postgres] ✗ Failed to drop schema: %v", err)
		http.Error(w, "Failed to delete session", http.StatusInternalServerError)
		return
	}

	// Remove from sessions map
	h.mu.Lock()
	delete(h.sessions, sessionID)
	h.mu.Unlock()

	// Clean up metadata in SQLite
	_ = h.queries.DeletePostgresSessionData(ctx, sessionID)

	w.WriteHeader(http.StatusNoContent)
	log.Printf("[postgres] ✓ Deleted session schema: %s", schemaName)
}

// Close closes the database connection
func (h *Handler) Close() error {
	return h.pgDB.Close()
}
