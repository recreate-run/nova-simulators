-- +goose Up
CREATE TABLE IF NOT EXISTS postgres_databases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    UNIQUE(name, session_id)
);

CREATE TABLE IF NOT EXISTS postgres_tables (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    database_name TEXT NOT NULL,
    table_name TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    UNIQUE(database_name, table_name, session_id)
);

CREATE TABLE IF NOT EXISTS postgres_columns (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    database_name TEXT NOT NULL,
    table_name TEXT NOT NULL,
    column_name TEXT NOT NULL,
    data_type TEXT NOT NULL,
    is_nullable TEXT NOT NULL DEFAULT 'YES',
    column_default TEXT,
    ordinal_position INTEGER NOT NULL,
    session_id TEXT NOT NULL,
    UNIQUE(database_name, table_name, column_name, session_id)
);

CREATE TABLE IF NOT EXISTS postgres_rows (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    database_name TEXT NOT NULL,
    table_name TEXT NOT NULL,
    row_data TEXT NOT NULL, -- JSON encoded row data
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS postgres_query_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    database_name TEXT NOT NULL,
    query_text TEXT NOT NULL,
    query_type TEXT NOT NULL, -- SELECT, INSERT, UPDATE, DELETE, CREATE, etc.
    rows_affected INTEGER,
    session_id TEXT NOT NULL,
    executed_at INTEGER NOT NULL DEFAULT (unixepoch())
);

-- Create indexes for session-based queries
CREATE INDEX IF NOT EXISTS idx_postgres_databases_session ON postgres_databases(session_id, name);
CREATE INDEX IF NOT EXISTS idx_postgres_tables_session ON postgres_tables(session_id, database_name, table_name);
CREATE INDEX IF NOT EXISTS idx_postgres_columns_session ON postgres_columns(session_id, database_name, table_name);
CREATE INDEX IF NOT EXISTS idx_postgres_rows_session ON postgres_rows(session_id, database_name, table_name);
CREATE INDEX IF NOT EXISTS idx_postgres_query_log_session ON postgres_query_log(session_id, database_name);

-- +goose Down
DROP INDEX IF EXISTS idx_postgres_query_log_session;
DROP INDEX IF EXISTS idx_postgres_rows_session;
DROP INDEX IF EXISTS idx_postgres_columns_session;
DROP INDEX IF EXISTS idx_postgres_tables_session;
DROP INDEX IF EXISTS idx_postgres_databases_session;

DROP TABLE IF EXISTS postgres_query_log;
DROP TABLE IF EXISTS postgres_rows;
DROP TABLE IF EXISTS postgres_columns;
DROP TABLE IF EXISTS postgres_tables;
DROP TABLE IF EXISTS postgres_databases;
