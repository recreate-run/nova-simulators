-- +goose Up
-- Add session_configs table for per-session rate limit and timeout configuration
-- YAML config provides defaults; this table stores per-session overrides

CREATE TABLE IF NOT EXISTS session_configs (
    session_id TEXT NOT NULL,
    simulator_name TEXT NOT NULL,
    timeout_min_ms INTEGER NOT NULL DEFAULT 0,
    timeout_max_ms INTEGER NOT NULL DEFAULT 0,
    rate_limit_per_minute INTEGER NOT NULL DEFAULT 60,
    rate_limit_per_day INTEGER NOT NULL DEFAULT 1000,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (session_id, simulator_name),
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

-- Index for querying configs by session
CREATE INDEX IF NOT EXISTS idx_session_configs_session ON session_configs(session_id);

-- +goose Down
DROP INDEX IF EXISTS idx_session_configs_session;
DROP TABLE IF EXISTS session_configs;
