-- Incidents (v2 API)

-- name: CreateDatadogIncident :exec
INSERT INTO datadog_incidents (id, title, customer_impacted, severity, session_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetDatadogIncidentByID :one
SELECT id, title, customer_impacted, severity, session_id, created_at, updated_at
FROM datadog_incidents
WHERE id = ? AND session_id = ?;

-- name: UpdateDatadogIncident :exec
UPDATE datadog_incidents
SET title = COALESCE(sqlc.narg('title'), title),
    customer_impacted = COALESCE(sqlc.narg('customer_impacted'), customer_impacted),
    severity = COALESCE(sqlc.narg('severity'), severity),
    updated_at = sqlc.arg('updated_at')
WHERE id = sqlc.arg('id') AND session_id = sqlc.arg('session_id');

-- name: ListDatadogIncidents :many
SELECT id, title, customer_impacted, severity, created_at, updated_at
FROM datadog_incidents
WHERE session_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- Monitors (v1 API)

-- name: CreateDatadogMonitor :one
INSERT INTO datadog_monitors (name, type, query, message, session_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id, name, type, query, message, created_at, updated_at;

-- name: GetDatadogMonitorByID :one
SELECT id, name, type, query, message, session_id, created_at, updated_at
FROM datadog_monitors
WHERE id = ? AND session_id = ?;

-- name: UpdateDatadogMonitor :exec
UPDATE datadog_monitors
SET name = COALESCE(sqlc.narg('name'), name),
    query = COALESCE(sqlc.narg('query'), query),
    message = COALESCE(sqlc.narg('message'), message),
    updated_at = sqlc.arg('updated_at')
WHERE id = sqlc.arg('id') AND session_id = sqlc.arg('session_id');

-- name: DeleteDatadogMonitor :exec
DELETE FROM datadog_monitors
WHERE id = ? AND session_id = ?;

-- name: ListDatadogMonitors :many
SELECT id, name, type, query, message, created_at, updated_at
FROM datadog_monitors
WHERE session_id = ?
ORDER BY created_at DESC;

-- Events (v1 API)

-- name: CreateDatadogEvent :one
INSERT INTO datadog_events (title, text, tags, session_id, created_at)
VALUES (?, ?, ?, ?, ?)
RETURNING id, title, text, tags, created_at;

-- name: GetDatadogEventByID :one
SELECT id, title, text, tags, created_at
FROM datadog_events
WHERE id = ? AND session_id = ?;

-- name: ListDatadogEvents :many
SELECT id, title, text, tags, created_at
FROM datadog_events
WHERE session_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- Metrics (v2 API)

-- name: CreateDatadogMetric :exec
INSERT INTO datadog_metrics (metric_name, value, tags, timestamp, session_id, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: ListDatadogMetrics :many
SELECT id, metric_name, value, tags, timestamp, created_at
FROM datadog_metrics
WHERE session_id = ? AND metric_name = ?
ORDER BY timestamp DESC
LIMIT ?;

-- name: ListDatadogMetricsBySession :many
SELECT id, metric_name, value, tags, timestamp, created_at
FROM datadog_metrics
WHERE session_id = ?
ORDER BY timestamp DESC
LIMIT ?;

-- Cleanup

-- name: DeleteDatadogSessionData :exec
DELETE FROM datadog_incidents WHERE session_id = ?;
DELETE FROM datadog_monitors WHERE session_id = ?;
DELETE FROM datadog_events WHERE session_id = ?;
DELETE FROM datadog_metrics WHERE session_id = ?;
