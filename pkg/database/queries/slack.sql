-- name: CreateMessage :exec
INSERT INTO slack_messages (channel_id, type, user_id, text, timestamp)
VALUES (?, ?, ?, ?, ?);

-- name: GetMessagesByChannel :many
SELECT type, user_id, text, timestamp
FROM slack_messages
WHERE channel_id = ?
ORDER BY timestamp DESC;

-- name: ListChannels :many
SELECT id, name, created_at
FROM slack_channels
ORDER BY created_at ASC;

-- name: GetChannelByID :one
SELECT id, name, created_at
FROM slack_channels
WHERE id = ?;
