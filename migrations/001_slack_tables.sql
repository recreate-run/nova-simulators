-- +goose Up
CREATE TABLE IF NOT EXISTS slack_channels (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS slack_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'message',
    user_id TEXT NOT NULL,
    text TEXT NOT NULL,
    timestamp TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    FOREIGN KEY (channel_id) REFERENCES slack_channels(id)
);

CREATE INDEX IF NOT EXISTS idx_slack_messages_channel_id ON slack_messages(channel_id);
CREATE INDEX IF NOT EXISTS idx_slack_messages_timestamp ON slack_messages(timestamp);

-- Insert default channels
INSERT INTO slack_channels (id, name, created_at) VALUES
    ('C001', 'general', unixepoch()),
    ('C002', 'random', unixepoch());

-- +goose Down
DROP INDEX IF EXISTS idx_slack_messages_timestamp;
DROP INDEX IF EXISTS idx_slack_messages_channel_id;
DROP TABLE IF EXISTS slack_messages;
DROP TABLE IF EXISTS slack_channels;
