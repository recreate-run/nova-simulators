-- +goose Up
CREATE TABLE IF NOT EXISTS pagerduty_services (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS pagerduty_escalation_policies (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS pagerduty_oncalls (
    id TEXT PRIMARY KEY,
    user_email TEXT NOT NULL,
    escalation_policy_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    FOREIGN KEY (escalation_policy_id) REFERENCES pagerduty_escalation_policies(id)
);

CREATE TABLE IF NOT EXISTS pagerduty_incidents (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    service_id TEXT NOT NULL,
    urgency TEXT NOT NULL,
    status TEXT NOT NULL,
    body_details TEXT,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    FOREIGN KEY (service_id) REFERENCES pagerduty_services(id)
);

-- Create indexes for session-based queries and performance
CREATE INDEX IF NOT EXISTS idx_pagerduty_services_session ON pagerduty_services(session_id);
CREATE INDEX IF NOT EXISTS idx_pagerduty_escalation_policies_session ON pagerduty_escalation_policies(session_id);
CREATE INDEX IF NOT EXISTS idx_pagerduty_oncalls_session ON pagerduty_oncalls(session_id);
CREATE INDEX IF NOT EXISTS idx_pagerduty_oncalls_escalation_policy ON pagerduty_oncalls(escalation_policy_id, session_id);
CREATE INDEX IF NOT EXISTS idx_pagerduty_incidents_session ON pagerduty_incidents(session_id);
CREATE INDEX IF NOT EXISTS idx_pagerduty_incidents_service ON pagerduty_incidents(service_id, session_id);
CREATE INDEX IF NOT EXISTS idx_pagerduty_incidents_status ON pagerduty_incidents(status, session_id);

-- +goose Down
DROP INDEX IF EXISTS idx_pagerduty_incidents_status;
DROP INDEX IF EXISTS idx_pagerduty_incidents_service;
DROP INDEX IF EXISTS idx_pagerduty_incidents_session;
DROP INDEX IF EXISTS idx_pagerduty_oncalls_escalation_policy;
DROP INDEX IF EXISTS idx_pagerduty_oncalls_session;
DROP INDEX IF EXISTS idx_pagerduty_escalation_policies_session;
DROP INDEX IF EXISTS idx_pagerduty_services_session;
DROP TABLE IF EXISTS pagerduty_incidents;
DROP TABLE IF EXISTS pagerduty_oncalls;
DROP TABLE IF EXISTS pagerduty_escalation_policies;
DROP TABLE IF EXISTS pagerduty_services;
