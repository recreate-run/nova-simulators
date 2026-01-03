-- name: CreateJiraProject :exec
INSERT INTO jira_projects (id, key, name, session_id)
VALUES (?, ?, ?, ?);

-- name: GetJiraProjectByKey :one
SELECT id, key, name, created_at
FROM jira_projects
WHERE key = ? AND session_id = ?;

-- name: ListJiraProjects :many
SELECT id, key, name, created_at
FROM jira_projects
WHERE session_id = ?
ORDER BY created_at DESC;

-- name: CreateJiraIssue :exec
INSERT INTO jira_issues (id, key, project_key, issue_type, summary, description, assignee, status, session_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetJiraIssueByKey :one
SELECT id, key, project_key, issue_type, summary, description, assignee, status, created_at, updated_at
FROM jira_issues
WHERE key = ? AND session_id = ?;

-- name: UpdateJiraIssue :exec
UPDATE jira_issues
SET summary = ?, description = ?, assignee = ?, updated_at = unixepoch()
WHERE key = ? AND session_id = ?;

-- name: UpdateJiraIssueStatus :exec
UPDATE jira_issues
SET status = ?, updated_at = unixepoch()
WHERE key = ? AND session_id = ?;

-- name: SearchJiraIssues :many
SELECT id, key, project_key, issue_type, summary, description, assignee, status, created_at, updated_at
FROM jira_issues
WHERE session_id = ?
    AND (? = '' OR project_key = ?)
    AND (? = '' OR issue_type = ?)
    AND (? = '' OR summary LIKE '%' || ? || '%')
    AND (? = '' OR assignee = ?)
    AND (? = '' OR status = ?)
ORDER BY created_at DESC
LIMIT ?;

-- name: CreateJiraComment :exec
INSERT INTO jira_comments (id, issue_key, body, session_id)
VALUES (?, ?, ?, ?);

-- name: ListJiraComments :many
SELECT id, issue_key, body, created_at
FROM jira_comments
WHERE issue_key = ? AND session_id = ?
ORDER BY created_at ASC;

-- name: CreateJiraTransition :exec
INSERT INTO jira_transitions (id, name, to_status, session_id)
VALUES (?, ?, ?, ?);

-- name: ListJiraTransitions :many
SELECT id, name, to_status, created_at
FROM jira_transitions
WHERE session_id = ?
ORDER BY created_at ASC;

-- name: GetJiraTransitionByName :one
SELECT id, name, to_status, created_at
FROM jira_transitions
WHERE name = ? AND session_id = ?;

-- name: DeleteJiraSessionData :exec
DELETE FROM jira_projects WHERE session_id = ?;
DELETE FROM jira_issues WHERE session_id = ?;
DELETE FROM jira_comments WHERE session_id = ?;
DELETE FROM jira_transitions WHERE session_id = ?;

-- UI data queries
-- name: ListJiraIssuesBySession :many
SELECT id, key, project_key, issue_type, summary, description, assignee, status, created_at, updated_at
FROM jira_issues
WHERE session_id = ?
ORDER BY created_at DESC;
