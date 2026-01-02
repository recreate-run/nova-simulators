-- +goose Up
-- Table for Slack files
CREATE TABLE IF NOT EXISTS slack_files (
    id TEXT PRIMARY KEY,
    filename TEXT NOT NULL,
    title TEXT,
    filetype TEXT,
    size INTEGER NOT NULL,
    upload_url TEXT,
    channel_id TEXT,
    user_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

-- Table for Slack users
CREATE TABLE IF NOT EXISTS slack_users (
    id TEXT PRIMARY KEY,
    team_id TEXT NOT NULL DEFAULT 'T021F9ZE2',
    name TEXT NOT NULL,
    real_name TEXT NOT NULL,
    email TEXT,
    display_name TEXT,
    first_name TEXT,
    last_name TEXT,
    is_admin INTEGER NOT NULL DEFAULT 0,
    is_owner INTEGER NOT NULL DEFAULT 0,
    is_bot INTEGER NOT NULL DEFAULT 0,
    timezone TEXT DEFAULT 'America/Los_Angeles',
    timezone_label TEXT DEFAULT 'Pacific Standard Time',
    timezone_offset INTEGER DEFAULT -28800,
    image_24 TEXT,
    image_32 TEXT,
    image_48 TEXT,
    image_72 TEXT,
    image_192 TEXT,
    image_512 TEXT,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

-- Add attachments column to slack_messages (stored as JSON)
ALTER TABLE slack_messages ADD COLUMN attachments TEXT;

CREATE INDEX IF NOT EXISTS idx_slack_files_channel_id ON slack_files(channel_id);
CREATE INDEX IF NOT EXISTS idx_slack_files_user_id ON slack_files(user_id);

-- Insert default users
INSERT INTO slack_users (id, name, real_name, email, display_name, first_name, last_name) VALUES
    ('U123456', 'test-user', 'Test User', 'test@example.com', 'testuser', 'Test', 'User'),
    ('U789012', 'bobby', 'Bobby Tables', 'bobby@example.com', 'bobby', 'Bobby', 'Tables');

-- +goose Down
DROP INDEX IF EXISTS idx_slack_files_user_id;
DROP INDEX IF EXISTS idx_slack_files_channel_id;
DROP TABLE IF EXISTS slack_users;
DROP TABLE IF EXISTS slack_files;
-- Note: Cannot easily drop column in SQLite, would need to recreate table
