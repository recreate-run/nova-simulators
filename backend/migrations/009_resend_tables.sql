-- +goose Up
CREATE TABLE IF NOT EXISTS resend_emails (
    id TEXT PRIMARY KEY,
    from_email TEXT NOT NULL,
    to_emails TEXT NOT NULL,
    subject TEXT NOT NULL,
    html TEXT NOT NULL,
    cc_emails TEXT,
    bcc_emails TEXT,
    reply_to TEXT,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX IF NOT EXISTS idx_resend_emails_session_id ON resend_emails(session_id);
CREATE INDEX IF NOT EXISTS idx_resend_emails_from_email ON resend_emails(from_email);
CREATE INDEX IF NOT EXISTS idx_resend_emails_created_at ON resend_emails(created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_resend_emails_created_at;
DROP INDEX IF EXISTS idx_resend_emails_from_email;
DROP INDEX IF EXISTS idx_resend_emails_session_id;
DROP TABLE IF EXISTS resend_emails;
