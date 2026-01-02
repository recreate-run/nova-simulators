-- +goose Up
CREATE TABLE IF NOT EXISTS jira_projects (
    id TEXT PRIMARY KEY,
    key TEXT NOT NULL,
    name TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS jira_issues (
    id TEXT PRIMARY KEY,
    key TEXT NOT NULL,
    project_key TEXT NOT NULL,
    issue_type TEXT NOT NULL,
    summary TEXT NOT NULL,
    description TEXT,
    assignee TEXT,
    status TEXT NOT NULL DEFAULT 'To Do',
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS jira_comments (
    id TEXT PRIMARY KEY,
    issue_key TEXT NOT NULL,
    body TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS jira_transitions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    to_status TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

-- Create indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_jira_projects_session ON jira_projects(session_id);
CREATE INDEX IF NOT EXISTS idx_jira_projects_key ON jira_projects(key, session_id);
CREATE INDEX IF NOT EXISTS idx_jira_issues_session ON jira_issues(session_id);
CREATE INDEX IF NOT EXISTS idx_jira_issues_key ON jira_issues(key, session_id);
CREATE INDEX IF NOT EXISTS idx_jira_issues_project ON jira_issues(project_key, session_id);
CREATE INDEX IF NOT EXISTS idx_jira_comments_issue ON jira_comments(issue_key, session_id);
CREATE INDEX IF NOT EXISTS idx_jira_transitions_session ON jira_transitions(session_id);

-- +goose Down
DROP INDEX IF EXISTS idx_jira_transitions_session;
DROP INDEX IF EXISTS idx_jira_comments_issue;
DROP INDEX IF EXISTS idx_jira_issues_project;
DROP INDEX IF EXISTS idx_jira_issues_key;
DROP INDEX IF EXISTS idx_jira_issues_session;
DROP INDEX IF EXISTS idx_jira_projects_key;
DROP INDEX IF EXISTS idx_jira_projects_session;
DROP TABLE IF EXISTS jira_transitions;
DROP TABLE IF EXISTS jira_comments;
DROP TABLE IF EXISTS jira_issues;
DROP TABLE IF EXISTS jira_projects;
