-- name: CreateWhatsAppMessage :exec
INSERT INTO whatsapp_messages (id, phone_number_id, to_number, message_type, text_body, media_url, caption, template_name, language_code, session_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetWhatsAppMessageByID :one
SELECT id, phone_number_id, to_number, message_type, text_body, media_url, caption, template_name, language_code, created_at
FROM whatsapp_messages
WHERE id = ? AND session_id = ?;

-- name: ListWhatsAppMessages :many
SELECT id, phone_number_id, to_number, message_type, text_body, media_url, caption, template_name, language_code, created_at
FROM whatsapp_messages
WHERE session_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: DeleteWhatsAppSessionData :exec
DELETE FROM whatsapp_messages WHERE session_id = ?;
