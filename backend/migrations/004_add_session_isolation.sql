-- +goose Up
-- Add session_id columns to all tables for multi-session isolation
-- Note: Session ID is required for all API requests via X-Session-ID header

-- Slack tables
-- Using DEFAULT only for SQLite ALTER TABLE compatibility (cannot add NOT NULL without DEFAULT to tables with existing rows)
-- The DEFAULT value is not used in practice as session ID is always required via X-Session-ID header
ALTER TABLE slack_channels ADD COLUMN session_id TEXT NOT NULL DEFAULT '';
ALTER TABLE slack_messages ADD COLUMN session_id TEXT NOT NULL DEFAULT '';
ALTER TABLE slack_files ADD COLUMN session_id TEXT NOT NULL DEFAULT '';
ALTER TABLE slack_users ADD COLUMN session_id TEXT NOT NULL DEFAULT '';

-- Gmail tables
ALTER TABLE gmail_messages ADD COLUMN session_id TEXT NOT NULL DEFAULT '';

-- Create indexes for session-based queries
CREATE INDEX IF NOT EXISTS idx_slack_channels_session ON slack_channels(session_id);
CREATE INDEX IF NOT EXISTS idx_slack_messages_session ON slack_messages(session_id, channel_id);
CREATE INDEX IF NOT EXISTS idx_slack_files_session ON slack_files(session_id);
CREATE INDEX IF NOT EXISTS idx_slack_users_session ON slack_users(session_id);
CREATE INDEX IF NOT EXISTS idx_gmail_messages_session ON gmail_messages(session_id);

-- Create sessions metadata table
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    last_accessed INTEGER NOT NULL DEFAULT (unixepoch())
);

-- +goose Down
DROP INDEX IF EXISTS idx_gmail_messages_session;
DROP INDEX IF EXISTS idx_slack_users_session;
DROP INDEX IF EXISTS idx_slack_files_session;
DROP INDEX IF EXISTS idx_slack_messages_session;
DROP INDEX IF EXISTS idx_slack_channels_session;

DROP TABLE IF EXISTS sessions;

-- Note: Cannot easily drop columns in SQLite without recreating tables
-- Users should recreate tables from scratch if downgrading
