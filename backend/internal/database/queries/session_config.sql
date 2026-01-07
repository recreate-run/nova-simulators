-- name: GetSessionConfig :one
SELECT timeout_min_ms, timeout_max_ms, rate_limit_per_minute, rate_limit_per_day
FROM session_configs
WHERE session_id = ? AND simulator_name = ?;

-- name: UpsertSessionConfig :exec
INSERT INTO session_configs (session_id, simulator_name, timeout_min_ms, timeout_max_ms, rate_limit_per_minute, rate_limit_per_day, updated_at)
VALUES (?, ?, ?, ?, ?, ?, unixepoch())
ON CONFLICT(session_id, simulator_name) DO UPDATE SET
    timeout_min_ms = excluded.timeout_min_ms,
    timeout_max_ms = excluded.timeout_max_ms,
    rate_limit_per_minute = excluded.rate_limit_per_minute,
    rate_limit_per_day = excluded.rate_limit_per_day,
    updated_at = unixepoch();

-- name: DeleteSessionConfig :exec
DELETE FROM session_configs
WHERE session_id = ? AND simulator_name = ?;

-- name: ListSessionConfigs :many
SELECT session_id, simulator_name, timeout_min_ms, timeout_max_ms, rate_limit_per_minute, rate_limit_per_day, created_at, updated_at
FROM session_configs
WHERE session_id = ?
ORDER BY simulator_name;
