-- +goose Up
CREATE TABLE IF NOT EXISTS gmail_attachments (
    id TEXT PRIMARY KEY,
    message_id TEXT NOT NULL,
    filename TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    data BLOB NOT NULL,
    size INTEGER NOT NULL,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    FOREIGN KEY (message_id) REFERENCES gmail_messages(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_gmail_attachments_message_id ON gmail_attachments(message_id);
CREATE INDEX IF NOT EXISTS idx_gmail_attachments_session ON gmail_attachments(session_id);

-- +goose Down
DROP INDEX IF EXISTS idx_gmail_attachments_session;
DROP INDEX IF EXISTS idx_gmail_attachments_message_id;
DROP TABLE IF EXISTS gmail_attachments;
