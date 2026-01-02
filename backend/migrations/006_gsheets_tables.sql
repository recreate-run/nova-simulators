-- +goose Up
CREATE TABLE IF NOT EXISTS gsheets_spreadsheets (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    session_id TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS gsheets_sheets (
    id TEXT PRIMARY KEY,
    spreadsheet_id TEXT NOT NULL,
    title TEXT NOT NULL,
    sheet_id INTEGER NOT NULL,
    session_id TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    FOREIGN KEY (spreadsheet_id) REFERENCES gsheets_spreadsheets(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS gsheets_cells (
    spreadsheet_id TEXT NOT NULL,
    sheet_title TEXT NOT NULL,
    row INTEGER NOT NULL,
    col INTEGER NOT NULL,
    value TEXT,
    session_id TEXT NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (spreadsheet_id, sheet_title, row, col, session_id),
    FOREIGN KEY (spreadsheet_id) REFERENCES gsheets_spreadsheets(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_gsheets_spreadsheets_session ON gsheets_spreadsheets(session_id);
CREATE INDEX IF NOT EXISTS idx_gsheets_sheets_session ON gsheets_sheets(session_id, spreadsheet_id);
CREATE INDEX IF NOT EXISTS idx_gsheets_cells_session ON gsheets_cells(session_id, spreadsheet_id);
CREATE INDEX IF NOT EXISTS idx_gsheets_cells_lookup ON gsheets_cells(spreadsheet_id, sheet_title, session_id);

-- +goose Down
DROP INDEX IF EXISTS idx_gsheets_cells_lookup;
DROP INDEX IF EXISTS idx_gsheets_cells_session;
DROP INDEX IF EXISTS idx_gsheets_sheets_session;
DROP INDEX IF EXISTS idx_gsheets_spreadsheets_session;
DROP TABLE IF EXISTS gsheets_cells;
DROP TABLE IF EXISTS gsheets_sheets;
DROP TABLE IF EXISTS gsheets_spreadsheets;
