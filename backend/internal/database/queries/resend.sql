-- name: CreateResendEmail :exec
INSERT INTO resend_emails (id, from_email, to_emails, subject, html, cc_emails, bcc_emails, reply_to, session_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetResendEmailByID :one
SELECT id, from_email, to_emails, subject, html, cc_emails, bcc_emails, reply_to, created_at
FROM resend_emails
WHERE id = ? AND session_id = ?;

-- name: ListResendEmails :many
SELECT id, from_email, to_emails, subject, created_at
FROM resend_emails
WHERE session_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: DeleteResendSessionData :exec
DELETE FROM resend_emails WHERE session_id = ?;

-- UI data queries
-- name: ListResendEmailsBySession :many
SELECT id, from_email, to_emails, subject, html, cc_emails, bcc_emails, reply_to, created_at
FROM resend_emails
WHERE session_id = ?
ORDER BY created_at DESC;
