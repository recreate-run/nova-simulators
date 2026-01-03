-- name: CreateOutlookMessage :exec
INSERT INTO outlook_messages (id, from_email, to_email, subject, body_content, body_type, is_read, received_datetime, session_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetOutlookMessageByID :one
SELECT id, from_email, to_email, subject, body_content, body_type, is_read, received_datetime, created_at
FROM outlook_messages
WHERE id = ? AND session_id = ?;

-- name: ListOutlookMessages :many
SELECT id, from_email, to_email, subject, body_content, body_type, is_read, received_datetime
FROM outlook_messages
WHERE session_id = ?
ORDER BY received_datetime DESC
LIMIT ?;

-- name: SearchOutlookMessages :many
SELECT id, from_email, to_email, subject, body_content, body_type, is_read, received_datetime
FROM outlook_messages
WHERE
    session_id = ?
    AND (? = '' OR from_email LIKE '%' || ? || '%')
    AND (? = '' OR to_email LIKE '%' || ? || '%')
    AND (? = '' OR subject LIKE '%' || ? || '%')
    AND (? = '' OR body_content LIKE '%' || ? || '%')
ORDER BY received_datetime DESC
LIMIT ?;

-- name: UpdateOutlookMessageReadStatus :exec
UPDATE outlook_messages
SET is_read = ?
WHERE id = ? AND session_id = ?;

-- name: DeleteOutlookSessionData :exec
DELETE FROM outlook_messages WHERE session_id = ?;

-- UI data queries
-- name: ListOutlookMessagesBySession :many
SELECT id, from_email, to_email, subject, body_content, body_type, is_read, received_datetime, created_at
FROM outlook_messages
WHERE session_id = ?
ORDER BY received_datetime DESC;
