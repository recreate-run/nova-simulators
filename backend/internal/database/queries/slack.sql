-- name: CreateMessage :exec
INSERT INTO slack_messages (channel_id, type, user_id, text, timestamp, attachments, session_id)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetMessagesByChannel :many
SELECT type, user_id, text, timestamp, attachments
FROM slack_messages
WHERE channel_id = ? AND session_id = ?
ORDER BY timestamp DESC;

-- name: ListChannels :many
SELECT id, name, created_at
FROM slack_channels
WHERE session_id = ?
ORDER BY created_at ASC;

-- name: GetChannelByID :one
SELECT id, name, created_at
FROM slack_channels
WHERE id = ? AND session_id = ?;

-- name: CreateFile :exec
INSERT INTO slack_files (id, filename, title, filetype, size, upload_url, channel_id, user_id, session_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetFileByID :one
SELECT id, filename, title, filetype, size, upload_url, channel_id, user_id, created_at
FROM slack_files
WHERE id = ? AND session_id = ?;

-- name: GetUserByID :one
SELECT id, team_id, name, real_name, email, display_name, first_name, last_name,
       is_admin, is_owner, is_bot, timezone, timezone_label, timezone_offset,
       image_24, image_32, image_48, image_72, image_192, image_512, created_at
FROM slack_users
WHERE id = ? AND session_id = ?;

-- name: CreateUser :exec
INSERT INTO slack_users (id, team_id, name, real_name, email, display_name, first_name, last_name,
                         is_admin, is_owner, is_bot, timezone, timezone_label, timezone_offset,
                         image_24, image_32, image_48, image_72, image_192, image_512, session_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateChannel :exec
INSERT INTO slack_channels (id, name, created_at, session_id)
VALUES (?, ?, ?, ?);

-- Session management queries
-- name: CreateSession :exec
INSERT INTO sessions (id) VALUES (?);

-- name: GetSession :one
SELECT id, created_at, last_accessed FROM sessions WHERE id = ?;

-- name: DeleteSessionData :exec
DELETE FROM slack_messages WHERE session_id = ?;
DELETE FROM slack_files WHERE session_id = ?;

-- name: UpdateSessionAccess :exec
UPDATE sessions SET last_accessed = unixepoch() WHERE id = ?;
