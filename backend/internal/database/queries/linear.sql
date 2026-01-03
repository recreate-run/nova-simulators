-- Teams queries
-- name: CreateLinearTeam :exec
INSERT INTO linear_teams (id, name, key, session_id)
VALUES (?, ?, ?, ?);

-- name: GetLinearTeamByID :one
SELECT id, name, key, created_at
FROM linear_teams
WHERE id = ? AND session_id = ?;

-- name: ListLinearTeams :many
SELECT id, name, key, created_at
FROM linear_teams
WHERE session_id = ?
ORDER BY created_at ASC;

-- Users queries
-- name: CreateLinearUser :exec
INSERT INTO linear_users (id, name, email, session_id)
VALUES (?, ?, ?, ?);

-- name: GetLinearUserByID :one
SELECT id, name, email, created_at
FROM linear_users
WHERE id = ? AND session_id = ?;

-- name: ListLinearUsers :many
SELECT id, name, email, created_at
FROM linear_users
WHERE session_id = ?
ORDER BY created_at ASC;

-- States queries
-- name: CreateLinearState :exec
INSERT INTO linear_states (id, name, type, team_id, session_id)
VALUES (?, ?, ?, ?, ?);

-- name: GetLinearStateByID :one
SELECT id, name, type, team_id, created_at
FROM linear_states
WHERE id = ? AND session_id = ?;

-- name: ListLinearStatesByTeam :many
SELECT id, name, type, team_id, created_at
FROM linear_states
WHERE team_id = ? AND session_id = ?
ORDER BY created_at ASC;

-- Issues queries
-- name: CreateLinearIssue :one
INSERT INTO linear_issues (id, team_id, title, description, assignee_id, state_id, url, session_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, team_id, title, description, assignee_id, state_id, url, created_at, updated_at, archived_at;

-- name: GetLinearIssueByID :one
SELECT id, team_id, title, description, assignee_id, state_id, url, created_at, updated_at, archived_at
FROM linear_issues
WHERE id = ? AND session_id = ?;

-- name: UpdateLinearIssue :exec
UPDATE linear_issues
SET title = COALESCE(?, title),
    description = COALESCE(?, description),
    assignee_id = COALESCE(?, assignee_id),
    state_id = COALESCE(?, state_id),
    updated_at = ?
WHERE id = ? AND session_id = ?;

-- name: ListLinearIssuesByTeam :many
SELECT id, team_id, title, description, assignee_id, state_id, url, created_at, updated_at, archived_at
FROM linear_issues
WHERE team_id = ? AND session_id = ?
ORDER BY created_at DESC;

-- name: ListLinearIssues :many
SELECT id, team_id, title, description, assignee_id, state_id, url, created_at, updated_at, archived_at
FROM linear_issues
WHERE session_id = ?
ORDER BY created_at DESC;

-- Session management
-- name: DeleteLinearSessionData :exec
DELETE FROM linear_issues WHERE session_id = ?;
DELETE FROM linear_states WHERE session_id = ?;
DELETE FROM linear_users WHERE session_id = ?;
DELETE FROM linear_teams WHERE session_id = ?;

-- UI data queries
-- name: ListLinearIssuesBySession :many
SELECT id, team_id, title, description, assignee_id, state_id, url, created_at, updated_at, archived_at
FROM linear_issues
WHERE session_id = ?
ORDER BY created_at DESC;
