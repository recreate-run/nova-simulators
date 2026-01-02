-- +goose Up
CREATE TABLE IF NOT EXISTS linear_teams (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    key TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS linear_users (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS linear_states (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    team_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    FOREIGN KEY (team_id) REFERENCES linear_teams(id)
);

CREATE TABLE IF NOT EXISTS linear_issues (
    id TEXT PRIMARY KEY,
    team_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    assignee_id TEXT,
    state_id TEXT,
    url TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    archived_at INTEGER,
    FOREIGN KEY (team_id) REFERENCES linear_teams(id),
    FOREIGN KEY (assignee_id) REFERENCES linear_users(id),
    FOREIGN KEY (state_id) REFERENCES linear_states(id)
);

-- Create indexes for session-based queries and performance
CREATE INDEX IF NOT EXISTS idx_linear_teams_session ON linear_teams(session_id);
CREATE INDEX IF NOT EXISTS idx_linear_users_session ON linear_users(session_id);
CREATE INDEX IF NOT EXISTS idx_linear_states_session ON linear_states(session_id, team_id);
CREATE INDEX IF NOT EXISTS idx_linear_issues_session ON linear_issues(session_id);
CREATE INDEX IF NOT EXISTS idx_linear_issues_team ON linear_issues(team_id, session_id);
CREATE INDEX IF NOT EXISTS idx_linear_issues_assignee ON linear_issues(assignee_id, session_id);
CREATE INDEX IF NOT EXISTS idx_linear_issues_state ON linear_issues(state_id, session_id);

-- +goose Down
DROP INDEX IF EXISTS idx_linear_issues_state;
DROP INDEX IF EXISTS idx_linear_issues_assignee;
DROP INDEX IF EXISTS idx_linear_issues_team;
DROP INDEX IF EXISTS idx_linear_issues_session;
DROP INDEX IF EXISTS idx_linear_states_session;
DROP INDEX IF EXISTS idx_linear_users_session;
DROP INDEX IF EXISTS idx_linear_teams_session;
DROP TABLE IF EXISTS linear_issues;
DROP TABLE IF EXISTS linear_states;
DROP TABLE IF EXISTS linear_users;
DROP TABLE IF EXISTS linear_teams;
