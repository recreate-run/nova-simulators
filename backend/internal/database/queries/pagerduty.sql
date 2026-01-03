-- Services queries
-- name: CreatePagerDutyService :exec
INSERT INTO pagerduty_services (id, name, session_id)
VALUES (?, ?, ?);

-- name: GetPagerDutyServiceByID :one
SELECT id, name, created_at
FROM pagerduty_services
WHERE id = ? AND session_id = ?;

-- name: ListPagerDutyServices :many
SELECT id, name, created_at
FROM pagerduty_services
WHERE session_id = ?
ORDER BY created_at ASC;

-- Escalation Policies queries
-- name: CreatePagerDutyEscalationPolicy :exec
INSERT INTO pagerduty_escalation_policies (id, name, session_id)
VALUES (?, ?, ?);

-- name: GetPagerDutyEscalationPolicyByID :one
SELECT id, name, created_at
FROM pagerduty_escalation_policies
WHERE id = ? AND session_id = ?;

-- name: ListPagerDutyEscalationPolicies :many
SELECT id, name, created_at
FROM pagerduty_escalation_policies
WHERE session_id = ?
ORDER BY created_at ASC;

-- OnCalls queries
-- name: CreatePagerDutyOnCall :exec
INSERT INTO pagerduty_oncalls (id, user_email, escalation_policy_id, session_id)
VALUES (?, ?, ?, ?);

-- name: GetPagerDutyOnCallByID :one
SELECT id, user_email, escalation_policy_id, created_at
FROM pagerduty_oncalls
WHERE id = ? AND session_id = ?;

-- name: ListPagerDutyOnCalls :many
SELECT id, user_email, escalation_policy_id, created_at
FROM pagerduty_oncalls
WHERE session_id = ?
ORDER BY created_at ASC;

-- Incidents queries
-- name: CreatePagerDutyIncident :one
INSERT INTO pagerduty_incidents (id, title, service_id, urgency, status, body_details, session_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, title, service_id, urgency, status, body_details, created_at, updated_at;

-- name: GetPagerDutyIncidentByID :one
SELECT id, title, service_id, urgency, status, body_details, created_at, updated_at
FROM pagerduty_incidents
WHERE id = ? AND session_id = ?;

-- name: UpdatePagerDutyIncidentStatus :exec
UPDATE pagerduty_incidents
SET status = ?,
    updated_at = ?
WHERE id = ? AND session_id = ?;

-- name: ListPagerDutyIncidents :many
SELECT id, title, service_id, urgency, status, body_details, created_at, updated_at
FROM pagerduty_incidents
WHERE session_id = ?
ORDER BY created_at DESC;

-- name: ListPagerDutyIncidentsByStatus :many
SELECT id, title, service_id, urgency, status, body_details, created_at, updated_at
FROM pagerduty_incidents
WHERE status = ? AND session_id = ?
ORDER BY created_at DESC;

-- Session management
-- name: DeletePagerDutySessionData :exec
DELETE FROM pagerduty_incidents WHERE session_id = ?;
DELETE FROM pagerduty_oncalls WHERE session_id = ?;
DELETE FROM pagerduty_escalation_policies WHERE session_id = ?;
DELETE FROM pagerduty_services WHERE session_id = ?;

-- UI data queries
-- name: ListPagerdutyIncidentsBySession :many
SELECT id, title, service_id, urgency, status, body_details, created_at, updated_at
FROM pagerduty_incidents
WHERE session_id = ?
ORDER BY created_at DESC;
