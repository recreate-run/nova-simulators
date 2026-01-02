-- +goose Up
CREATE TABLE IF NOT EXISTS whatsapp_messages (
    id TEXT PRIMARY KEY,
    phone_number_id TEXT NOT NULL,
    to_number TEXT NOT NULL,
    message_type TEXT NOT NULL,
    text_body TEXT,
    media_url TEXT,
    caption TEXT,
    template_name TEXT,
    language_code TEXT,
    session_id TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX IF NOT EXISTS idx_whatsapp_messages_phone_number_id ON whatsapp_messages(phone_number_id);
CREATE INDEX IF NOT EXISTS idx_whatsapp_messages_to_number ON whatsapp_messages(to_number);
CREATE INDEX IF NOT EXISTS idx_whatsapp_messages_session ON whatsapp_messages(session_id);
CREATE INDEX IF NOT EXISTS idx_whatsapp_messages_created_at ON whatsapp_messages(created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_whatsapp_messages_created_at;
DROP INDEX IF EXISTS idx_whatsapp_messages_session;
DROP INDEX IF EXISTS idx_whatsapp_messages_to_number;
DROP INDEX IF EXISTS idx_whatsapp_messages_phone_number_id;
DROP TABLE IF EXISTS whatsapp_messages;
