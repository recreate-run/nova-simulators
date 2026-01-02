-- +goose Up
CREATE TABLE IF NOT EXISTS gmail_messages (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    from_email TEXT NOT NULL,
    to_email TEXT NOT NULL,
    subject TEXT NOT NULL,
    body_plain TEXT,
    body_html TEXT,
    raw_message TEXT NOT NULL,
    snippet TEXT,
    label_ids TEXT,
    internal_date INTEGER NOT NULL,
    size_estimate INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX IF NOT EXISTS idx_gmail_messages_thread_id ON gmail_messages(thread_id);
CREATE INDEX IF NOT EXISTS idx_gmail_messages_from_email ON gmail_messages(from_email);
CREATE INDEX IF NOT EXISTS idx_gmail_messages_to_email ON gmail_messages(to_email);
CREATE INDEX IF NOT EXISTS idx_gmail_messages_internal_date ON gmail_messages(internal_date);

-- +goose Down
DROP INDEX IF EXISTS idx_gmail_messages_internal_date;
DROP INDEX IF EXISTS idx_gmail_messages_to_email;
DROP INDEX IF EXISTS idx_gmail_messages_from_email;
DROP INDEX IF EXISTS idx_gmail_messages_thread_id;
DROP TABLE IF EXISTS gmail_messages;
