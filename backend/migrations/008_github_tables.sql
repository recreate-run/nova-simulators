-- +goose Up
CREATE TABLE IF NOT EXISTS github_repositories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    owner TEXT NOT NULL,
    name TEXT NOT NULL,
    default_branch TEXT NOT NULL DEFAULT 'main',
    description TEXT,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    UNIQUE(owner, name, session_id)
);

CREATE TABLE IF NOT EXISTS github_issues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_owner TEXT NOT NULL,
    repo_name TEXT NOT NULL,
    number INTEGER NOT NULL,
    title TEXT NOT NULL,
    body TEXT,
    state TEXT NOT NULL DEFAULT 'open',
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    UNIQUE(repo_owner, repo_name, number, session_id)
);

CREATE TABLE IF NOT EXISTS github_pull_requests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_owner TEXT NOT NULL,
    repo_name TEXT NOT NULL,
    number INTEGER NOT NULL,
    title TEXT NOT NULL,
    body TEXT,
    head TEXT NOT NULL,
    base TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'open',
    merged INTEGER NOT NULL DEFAULT 0,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    UNIQUE(repo_owner, repo_name, number, session_id)
);

CREATE TABLE IF NOT EXISTS github_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_owner TEXT NOT NULL,
    repo_name TEXT NOT NULL,
    path TEXT NOT NULL,
    content TEXT NOT NULL,
    sha TEXT NOT NULL,
    branch TEXT NOT NULL,
    session_id TEXT NOT NULL,
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    UNIQUE(repo_owner, repo_name, path, branch, session_id)
);

CREATE TABLE IF NOT EXISTS github_branches (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_owner TEXT NOT NULL,
    repo_name TEXT NOT NULL,
    name TEXT NOT NULL,
    sha TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    UNIQUE(repo_owner, repo_name, name, session_id)
);

CREATE TABLE IF NOT EXISTS github_workflows (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_owner TEXT NOT NULL,
    repo_name TEXT NOT NULL,
    workflow_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'active',
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    UNIQUE(repo_owner, repo_name, workflow_id, session_id)
);

CREATE TABLE IF NOT EXISTS github_workflow_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_owner TEXT NOT NULL,
    repo_name TEXT NOT NULL,
    run_id INTEGER NOT NULL,
    workflow_id INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'completed',
    conclusion TEXT,
    head_branch TEXT,
    head_sha TEXT,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    UNIQUE(repo_owner, repo_name, run_id, session_id)
);

CREATE TABLE IF NOT EXISTS github_issue_comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_owner TEXT NOT NULL,
    repo_name TEXT NOT NULL,
    issue_number INTEGER NOT NULL,
    comment_id INTEGER NOT NULL,
    body TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    UNIQUE(repo_owner, repo_name, comment_id, session_id)
);

-- Create indexes for session-based queries
CREATE INDEX IF NOT EXISTS idx_github_repositories_session ON github_repositories(session_id, owner, name);
CREATE INDEX IF NOT EXISTS idx_github_issues_session ON github_issues(session_id, repo_owner, repo_name);
CREATE INDEX IF NOT EXISTS idx_github_pull_requests_session ON github_pull_requests(session_id, repo_owner, repo_name);
CREATE INDEX IF NOT EXISTS idx_github_files_session ON github_files(session_id, repo_owner, repo_name, path);
CREATE INDEX IF NOT EXISTS idx_github_branches_session ON github_branches(session_id, repo_owner, repo_name);
CREATE INDEX IF NOT EXISTS idx_github_workflows_session ON github_workflows(session_id, repo_owner, repo_name);
CREATE INDEX IF NOT EXISTS idx_github_workflow_runs_session ON github_workflow_runs(session_id, repo_owner, repo_name);
CREATE INDEX IF NOT EXISTS idx_github_issue_comments_session ON github_issue_comments(session_id, repo_owner, repo_name, issue_number);

-- +goose Down
DROP INDEX IF EXISTS idx_github_issue_comments_session;
DROP INDEX IF EXISTS idx_github_workflow_runs_session;
DROP INDEX IF EXISTS idx_github_workflows_session;
DROP INDEX IF EXISTS idx_github_branches_session;
DROP INDEX IF EXISTS idx_github_files_session;
DROP INDEX IF EXISTS idx_github_pull_requests_session;
DROP INDEX IF EXISTS idx_github_issues_session;
DROP INDEX IF EXISTS idx_github_repositories_session;

DROP TABLE IF EXISTS github_issue_comments;
DROP TABLE IF EXISTS github_workflow_runs;
DROP TABLE IF EXISTS github_workflows;
DROP TABLE IF EXISTS github_branches;
DROP TABLE IF EXISTS github_files;
DROP TABLE IF EXISTS github_pull_requests;
DROP TABLE IF EXISTS github_issues;
DROP TABLE IF EXISTS github_repositories;
