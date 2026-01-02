-- +goose Up
CREATE TABLE IF NOT EXISTS outlook_messages (
    id TEXT PRIMARY KEY,
    from_email TEXT NOT NULL,
    to_email TEXT NOT NULL,
    subject TEXT NOT NULL,
    body_content TEXT,
    body_type TEXT NOT NULL DEFAULT 'text',
    is_read INTEGER NOT NULL DEFAULT 0,
    received_datetime TEXT NOT NULL,
    session_id TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX IF NOT EXISTS idx_outlook_messages_session ON outlook_messages(session_id);
CREATE INDEX IF NOT EXISTS idx_outlook_messages_from_email ON outlook_messages(from_email);
CREATE INDEX IF NOT EXISTS idx_outlook_messages_to_email ON outlook_messages(to_email);
CREATE INDEX IF NOT EXISTS idx_outlook_messages_received_datetime ON outlook_messages(received_datetime);

-- +goose Down
DROP INDEX IF EXISTS idx_outlook_messages_received_datetime;
DROP INDEX IF EXISTS idx_outlook_messages_to_email;
DROP INDEX IF EXISTS idx_outlook_messages_from_email;
DROP INDEX IF EXISTS idx_outlook_messages_session;
DROP TABLE IF EXISTS outlook_messages;
