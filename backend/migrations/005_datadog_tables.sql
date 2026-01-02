-- +goose Up
-- Datadog Incidents Table (v2 API)
CREATE TABLE IF NOT EXISTS datadog_incidents (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    customer_impacted INTEGER NOT NULL DEFAULT 0,
    severity TEXT,
    session_id TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX IF NOT EXISTS idx_datadog_incidents_session ON datadog_incidents(session_id);
CREATE INDEX IF NOT EXISTS idx_datadog_incidents_created_at ON datadog_incidents(created_at);

-- Datadog Monitors Table (v1 API)
CREATE TABLE IF NOT EXISTS datadog_monitors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    query TEXT NOT NULL,
    message TEXT,
    session_id TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX IF NOT EXISTS idx_datadog_monitors_session ON datadog_monitors(session_id);
CREATE INDEX IF NOT EXISTS idx_datadog_monitors_type ON datadog_monitors(type);

-- Datadog Events Table (v1 API)
CREATE TABLE IF NOT EXISTS datadog_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    text TEXT NOT NULL,
    tags TEXT,
    session_id TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX IF NOT EXISTS idx_datadog_events_session ON datadog_events(session_id);
CREATE INDEX IF NOT EXISTS idx_datadog_events_created_at ON datadog_events(created_at);

-- Datadog Metrics Table (v2 API)
CREATE TABLE IF NOT EXISTS datadog_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    metric_name TEXT NOT NULL,
    value REAL NOT NULL,
    tags TEXT,
    timestamp INTEGER NOT NULL,
    session_id TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX IF NOT EXISTS idx_datadog_metrics_session ON datadog_metrics(session_id);
CREATE INDEX IF NOT EXISTS idx_datadog_metrics_metric_name ON datadog_metrics(metric_name);
CREATE INDEX IF NOT EXISTS idx_datadog_metrics_timestamp ON datadog_metrics(timestamp);

-- +goose Down
DROP INDEX IF EXISTS idx_datadog_metrics_timestamp;
DROP INDEX IF EXISTS idx_datadog_metrics_metric_name;
DROP INDEX IF EXISTS idx_datadog_metrics_session;
DROP TABLE IF EXISTS datadog_metrics;

DROP INDEX IF EXISTS idx_datadog_events_created_at;
DROP INDEX IF EXISTS idx_datadog_events_session;
DROP TABLE IF EXISTS datadog_events;

DROP INDEX IF EXISTS idx_datadog_monitors_type;
DROP INDEX IF EXISTS idx_datadog_monitors_session;
DROP TABLE IF EXISTS datadog_monitors;

DROP INDEX IF EXISTS idx_datadog_incidents_created_at;
DROP INDEX IF EXISTS idx_datadog_incidents_session;
DROP TABLE IF EXISTS datadog_incidents;
