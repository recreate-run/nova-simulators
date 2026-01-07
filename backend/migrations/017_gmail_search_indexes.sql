-- +goose Up
-- Add index on subject column for better search performance
CREATE INDEX IF NOT EXISTS idx_gmail_messages_subject ON gmail_messages(subject);

-- +goose Down
DROP INDEX IF EXISTS idx_gmail_messages_subject;
