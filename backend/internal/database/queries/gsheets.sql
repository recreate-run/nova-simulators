-- name: CreateSpreadsheet :exec
INSERT INTO gsheets_spreadsheets (id, title, session_id)
VALUES (?, ?, ?);

-- name: GetSpreadsheet :one
SELECT id, title, created_at
FROM gsheets_spreadsheets
WHERE id = ? AND session_id = ?;

-- name: CreateSheet :exec
INSERT INTO gsheets_sheets (id, spreadsheet_id, title, sheet_id, session_id)
VALUES (?, ?, ?, ?, ?);

-- name: GetSheetsBySpreadsheet :many
SELECT id, spreadsheet_id, title, sheet_id
FROM gsheets_sheets
WHERE spreadsheet_id = ? AND session_id = ?
ORDER BY created_at ASC;

-- name: GetSheetByTitle :one
SELECT id, spreadsheet_id, title, sheet_id
FROM gsheets_sheets
WHERE spreadsheet_id = ? AND title = ? AND session_id = ?;

-- name: DeleteSheet :exec
DELETE FROM gsheets_sheets
WHERE spreadsheet_id = ? AND sheet_id = ? AND session_id = ?;

-- name: SetCellValue :exec
INSERT INTO gsheets_cells (spreadsheet_id, sheet_title, row, col, value, session_id, updated_at)
VALUES (?, ?, ?, ?, ?, ?, unixepoch())
ON CONFLICT(spreadsheet_id, sheet_title, row, col, session_id)
DO UPDATE SET value = excluded.value, updated_at = unixepoch();

-- name: GetCellValue :one
SELECT value
FROM gsheets_cells
WHERE spreadsheet_id = ? AND sheet_title = ? AND row = ? AND col = ? AND session_id = ?;

-- name: GetCellsInRange :many
SELECT row, col, value
FROM gsheets_cells
WHERE spreadsheet_id = ? AND sheet_title = ?
  AND row >= ? AND row <= ?
  AND col >= ? AND col <= ?
  AND session_id = ?
ORDER BY row ASC, col ASC;

-- name: ClearRange :exec
DELETE FROM gsheets_cells
WHERE spreadsheet_id = ? AND sheet_title = ?
  AND row >= ? AND row <= ?
  AND col >= ? AND col <= ?
  AND session_id = ?;

-- name: GetMaxRowInSheet :one
SELECT COALESCE(MAX(row), 0) as max_row
FROM gsheets_cells
WHERE spreadsheet_id = ? AND sheet_title = ? AND session_id = ?;

-- name: DeleteGsheetsSessionData :exec
DELETE FROM gsheets_spreadsheets WHERE session_id = ?;
