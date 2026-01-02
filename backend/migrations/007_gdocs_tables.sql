-- +goose Up
CREATE TABLE IF NOT EXISTS gdocs_documents (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    revision_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS gdocs_content (
    document_id TEXT NOT NULL,
    content_json TEXT NOT NULL,
    end_index INTEGER NOT NULL DEFAULT 1,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (document_id, session_id),
    FOREIGN KEY (document_id) REFERENCES gdocs_documents(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_gdocs_documents_session ON gdocs_documents(session_id);
CREATE INDEX IF NOT EXISTS idx_gdocs_content_session ON gdocs_content(session_id);

-- +goose Down
DROP INDEX IF EXISTS idx_gdocs_content_session;
DROP INDEX IF EXISTS idx_gdocs_documents_session;
DROP TABLE IF EXISTS gdocs_content;
DROP TABLE IF EXISTS gdocs_documents;
