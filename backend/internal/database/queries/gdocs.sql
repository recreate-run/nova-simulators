-- name: CreateGdocsDocument :exec
INSERT INTO gdocs_documents (id, title, revision_id, document_id, session_id)
VALUES (?, ?, ?, ?, ?);

-- name: GetGdocsDocumentByID :one
SELECT id, title, revision_id, document_id, session_id, created_at
FROM gdocs_documents
WHERE document_id = ? AND session_id = ?;

-- name: CreateGdocsContent :exec
INSERT INTO gdocs_content (document_id, content_json, end_index, session_id)
VALUES (?, ?, ?, ?);

-- name: GetGdocsContentByDocumentID :one
SELECT document_id, content_json, end_index, session_id, created_at, updated_at
FROM gdocs_content
WHERE document_id = ? AND session_id = ?;

-- name: UpdateGdocsContent :exec
UPDATE gdocs_content
SET content_json = ?, end_index = ?, updated_at = unixepoch()
WHERE document_id = ? AND session_id = ?;

-- name: DeleteGdocsSessionData :exec
DELETE FROM gdocs_documents WHERE session_id = ?;

-- UI data queries
-- name: ListGdocsBySession :many
SELECT id, title, revision_id, document_id, created_at
FROM gdocs_documents
WHERE session_id = ?
ORDER BY created_at DESC;
