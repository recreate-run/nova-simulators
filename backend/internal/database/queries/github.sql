-- Repository queries

-- name: CreateGithubRepository :exec
INSERT INTO github_repositories (owner, name, default_branch, description, session_id)
VALUES (?, ?, ?, ?, ?);

-- name: GetGithubRepository :one
SELECT id, owner, name, default_branch, description, created_at
FROM github_repositories
WHERE owner = ? AND name = ? AND session_id = ?;

-- name: ListGithubRepositories :many
SELECT id, owner, name, default_branch, description, created_at
FROM github_repositories
WHERE session_id = ? AND owner = ?
ORDER BY created_at DESC;

-- Issue queries

-- name: CreateGithubIssue :one
INSERT INTO github_issues (repo_owner, repo_name, number, title, body, state, session_id)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id, repo_owner, repo_name, number, title, body, state, created_at, updated_at;

-- name: GetGithubIssue :one
SELECT id, repo_owner, repo_name, number, title, body, state, created_at, updated_at
FROM github_issues
WHERE repo_owner = ? AND repo_name = ? AND number = ? AND session_id = ?;

-- name: ListGithubIssues :many
SELECT id, repo_owner, repo_name, number, title, body, state, created_at, updated_at
FROM github_issues
WHERE repo_owner = ? AND repo_name = ? AND session_id = ?
  AND (sqlc.arg(state_filter) = '' OR state = sqlc.arg(state_filter))
ORDER BY created_at DESC;

-- name: UpdateGithubIssue :exec
UPDATE github_issues
SET title = ?, body = ?, state = ?, updated_at = unixepoch()
WHERE repo_owner = ? AND repo_name = ? AND number = ? AND session_id = ?;

-- name: GetNextIssueNumber :one
SELECT COALESCE(MAX(number), 0) + 1 as next_number
FROM github_issues
WHERE repo_owner = ? AND repo_name = ? AND session_id = ?;

-- Pull Request queries

-- name: CreateGithubPullRequest :one
INSERT INTO github_pull_requests (repo_owner, repo_name, number, title, body, head, base, state, session_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, repo_owner, repo_name, number, title, body, head, base, state, merged, created_at, updated_at;

-- name: GetGithubPullRequest :one
SELECT id, repo_owner, repo_name, number, title, body, head, base, state, merged, created_at, updated_at
FROM github_pull_requests
WHERE repo_owner = ? AND repo_name = ? AND number = ? AND session_id = ?;

-- name: ListGithubPullRequests :many
SELECT id, repo_owner, repo_name, number, title, body, head, base, state, merged, created_at, updated_at
FROM github_pull_requests
WHERE repo_owner = ? AND repo_name = ? AND session_id = ?
  AND (sqlc.arg(state_filter) = '' OR state = sqlc.arg(state_filter))
ORDER BY created_at DESC;

-- name: MergeGithubPullRequest :exec
UPDATE github_pull_requests
SET state = 'closed', merged = 1, updated_at = unixepoch()
WHERE repo_owner = ? AND repo_name = ? AND number = ? AND session_id = ?;

-- name: GetNextPRNumber :one
SELECT COALESCE(MAX(number), 0) + 1 as next_number
FROM github_pull_requests
WHERE repo_owner = ? AND repo_name = ? AND session_id = ?;

-- File queries

-- name: CreateOrUpdateGithubFile :exec
INSERT INTO github_files (repo_owner, repo_name, path, content, sha, branch, session_id, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, unixepoch())
ON CONFLICT(repo_owner, repo_name, path, branch, session_id)
DO UPDATE SET content = excluded.content, sha = excluded.sha, updated_at = unixepoch();

-- name: GetGithubFile :one
SELECT id, repo_owner, repo_name, path, content, sha, branch, updated_at
FROM github_files
WHERE repo_owner = ? AND repo_name = ? AND path = ? AND branch = ? AND session_id = ?;

-- Branch queries

-- name: CreateGithubBranch :exec
INSERT INTO github_branches (repo_owner, repo_name, name, sha, session_id)
VALUES (?, ?, ?, ?, ?);

-- name: GetGithubBranch :one
SELECT id, repo_owner, repo_name, name, sha, created_at
FROM github_branches
WHERE repo_owner = ? AND repo_name = ? AND name = ? AND session_id = ?;

-- name: UpdateGithubBranchSHA :exec
UPDATE github_branches
SET sha = ?
WHERE repo_owner = ? AND repo_name = ? AND name = ? AND session_id = ?;

-- Workflow queries

-- name: CreateGithubWorkflow :exec
INSERT INTO github_workflows (repo_owner, repo_name, workflow_id, name, path, state, session_id)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetGithubWorkflow :one
SELECT id, repo_owner, repo_name, workflow_id, name, path, state, created_at
FROM github_workflows
WHERE repo_owner = ? AND repo_name = ? AND workflow_id = ? AND session_id = ?;

-- name: ListGithubWorkflows :many
SELECT id, repo_owner, repo_name, workflow_id, name, path, state, created_at
FROM github_workflows
WHERE repo_owner = ? AND repo_name = ? AND session_id = ?
ORDER BY created_at DESC;

-- Workflow Run queries

-- name: CreateGithubWorkflowRun :one
INSERT INTO github_workflow_runs (repo_owner, repo_name, run_id, workflow_id, status, conclusion, head_branch, head_sha, session_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, repo_owner, repo_name, run_id, workflow_id, status, conclusion, head_branch, head_sha, created_at, updated_at;

-- name: GetGithubWorkflowRun :one
SELECT id, repo_owner, repo_name, run_id, workflow_id, status, conclusion, head_branch, head_sha, created_at, updated_at
FROM github_workflow_runs
WHERE repo_owner = ? AND repo_name = ? AND run_id = ? AND session_id = ?;

-- name: ListGithubWorkflowRuns :many
SELECT id, repo_owner, repo_name, run_id, workflow_id, status, conclusion, head_branch, head_sha, created_at, updated_at
FROM github_workflow_runs
WHERE repo_owner = ? AND repo_name = ? AND session_id = ?
ORDER BY created_at DESC;

-- name: ListGithubWorkflowRunsForWorkflow :many
SELECT id, repo_owner, repo_name, run_id, workflow_id, status, conclusion, head_branch, head_sha, created_at, updated_at
FROM github_workflow_runs
WHERE repo_owner = ? AND repo_name = ? AND workflow_id = ? AND session_id = ?
ORDER BY created_at DESC;

-- name: GetNextWorkflowRunID :one
SELECT COALESCE(MAX(run_id), 0) + 1 as next_id
FROM github_workflow_runs
WHERE repo_owner = ? AND repo_name = ? AND session_id = ?;

-- Issue Comment queries

-- name: CreateGithubIssueComment :one
INSERT INTO github_issue_comments (repo_owner, repo_name, issue_number, comment_id, body, session_id)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id, repo_owner, repo_name, issue_number, comment_id, body, created_at;

-- name: GetNextCommentID :one
SELECT COALESCE(MAX(comment_id), 0) + 1 as next_id
FROM github_issue_comments
WHERE repo_owner = ? AND repo_name = ? AND session_id = ?;

-- name: ListGithubIssueComments :many
SELECT id, repo_owner, repo_name, issue_number, comment_id, body, created_at
FROM github_issue_comments
WHERE repo_owner = ? AND repo_name = ? AND issue_number = ? AND session_id = ?
ORDER BY created_at ASC;

-- Cleanup queries

-- name: DeleteGithubSessionData :exec
DELETE FROM github_repositories WHERE session_id = ?;
DELETE FROM github_issues WHERE session_id = ?;
DELETE FROM github_pull_requests WHERE session_id = ?;
DELETE FROM github_files WHERE session_id = ?;
DELETE FROM github_branches WHERE session_id = ?;
DELETE FROM github_workflows WHERE session_id = ?;
DELETE FROM github_workflow_runs WHERE session_id = ?;
DELETE FROM github_issue_comments WHERE session_id = ?;

-- UI data queries
-- name: ListGithubIssuesBySession :many
SELECT id, repo_owner, repo_name, number, title, body, state, created_at, updated_at
FROM github_issues
WHERE session_id = ?
ORDER BY created_at DESC;
