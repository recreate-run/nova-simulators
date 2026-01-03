-- name: CreateGmailMessage :exec
INSERT INTO gmail_messages (id, thread_id, from_email, to_email, subject, body_plain, body_html, raw_message, snippet, label_ids, internal_date, size_estimate, session_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetGmailMessageByID :one
SELECT id, thread_id, from_email, to_email, subject, body_plain, body_html, raw_message, snippet, label_ids, internal_date, size_estimate, created_at
FROM gmail_messages
WHERE id = ? AND session_id = ?;

-- name: ListGmailMessages :many
SELECT id, thread_id, snippet, label_ids, internal_date
FROM gmail_messages
WHERE session_id = ?
ORDER BY internal_date DESC
LIMIT ?;

-- name: SearchGmailMessages :many
SELECT id, thread_id, from_email, to_email, subject, snippet, label_ids, internal_date
FROM gmail_messages
WHERE
    session_id = ?
    AND (? = '' OR from_email LIKE '%' || ? || '%')
    AND (? = '' OR to_email LIKE '%' || ? || '%')
    AND (? = '' OR subject LIKE '%' || ? || '%')
    AND (? = '' OR body_plain LIKE '%' || ? || '%')
ORDER BY internal_date DESC
LIMIT ?;

-- name: DeleteGmailSessionData :exec
DELETE FROM gmail_messages WHERE session_id = ?;

-- UI data queries
-- name: ListGmailMessagesBySession :many
SELECT id, thread_id, from_email, to_email, subject, body_plain, body_html, snippet, label_ids, internal_date, size_estimate, created_at
FROM gmail_messages
WHERE session_id = ?
ORDER BY internal_date DESC;
