-- Database queries

-- name: CreatePostgresDatabase :exec
INSERT INTO postgres_databases (name, session_id)
VALUES (?, ?);

-- name: GetPostgresDatabase :one
SELECT id, name, created_at
FROM postgres_databases
WHERE name = ? AND session_id = ?;

-- name: ListPostgresDatabases :many
SELECT id, name, created_at
FROM postgres_databases
WHERE session_id = ?
ORDER BY created_at DESC;

-- Table queries

-- name: CreatePostgresTable :exec
INSERT INTO postgres_tables (database_name, table_name, session_id)
VALUES (?, ?, ?);

-- name: GetPostgresTable :one
SELECT id, database_name, table_name, created_at
FROM postgres_tables
WHERE database_name = ? AND table_name = ? AND session_id = ?;

-- name: ListPostgresTables :many
SELECT id, database_name, table_name, created_at
FROM postgres_tables
WHERE database_name = ? AND session_id = ?
ORDER BY table_name ASC;

-- name: DeletePostgresTable :exec
DELETE FROM postgres_tables
WHERE database_name = ? AND table_name = ? AND session_id = ?;

-- Column queries

-- name: CreatePostgresColumn :exec
INSERT INTO postgres_columns (database_name, table_name, column_name, data_type, is_nullable, column_default, ordinal_position, session_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListPostgresColumns :many
SELECT id, database_name, table_name, column_name, data_type, is_nullable, column_default, ordinal_position
FROM postgres_columns
WHERE database_name = ? AND table_name = ? AND session_id = ?
ORDER BY ordinal_position ASC;

-- name: DeletePostgresColumns :exec
DELETE FROM postgres_columns
WHERE database_name = ? AND table_name = ? AND session_id = ?;

-- Row queries

-- name: InsertPostgresRow :one
INSERT INTO postgres_rows (database_name, table_name, row_data, session_id)
VALUES (?, ?, ?, ?)
RETURNING id, database_name, table_name, row_data, created_at, updated_at;

-- name: ListPostgresRows :many
SELECT id, database_name, table_name, row_data, created_at, updated_at
FROM postgres_rows
WHERE database_name = ? AND table_name = ? AND session_id = ?
ORDER BY id ASC;

-- name: UpdatePostgresRow :exec
UPDATE postgres_rows
SET row_data = ?, updated_at = unixepoch()
WHERE id = ? AND session_id = ?;

-- name: DeletePostgresRow :exec
DELETE FROM postgres_rows
WHERE id = ? AND session_id = ?;

-- name: DeletePostgresRowsByTable :exec
DELETE FROM postgres_rows
WHERE database_name = ? AND table_name = ? AND session_id = ?;

-- name: CountPostgresRows :one
SELECT COUNT(*) as count
FROM postgres_rows
WHERE database_name = ? AND table_name = ? AND session_id = ?;

-- Query log queries

-- name: LogPostgresQuery :exec
INSERT INTO postgres_query_log (database_name, query_text, query_type, rows_affected, session_id)
VALUES (?, ?, ?, ?, ?);

-- name: ListPostgresQueryLog :many
SELECT id, database_name, query_text, query_type, rows_affected, executed_at
FROM postgres_query_log
WHERE session_id = ? AND database_name = ?
ORDER BY executed_at DESC
LIMIT ?;

-- Cleanup queries

-- name: DeletePostgresSessionData :exec
DELETE FROM postgres_databases WHERE session_id = ?;
DELETE FROM postgres_tables WHERE session_id = ?;
DELETE FROM postgres_columns WHERE session_id = ?;
DELETE FROM postgres_rows WHERE session_id = ?;
DELETE FROM postgres_query_log WHERE session_id = ?;
