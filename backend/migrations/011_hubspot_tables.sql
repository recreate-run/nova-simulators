-- +goose Up
CREATE TABLE IF NOT EXISTS hubspot_contacts (
    id TEXT PRIMARY KEY,
    email TEXT,
    first_name TEXT,
    last_name TEXT,
    mobile_phone TEXT,
    website TEXT,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS hubspot_deals (
    id TEXT PRIMARY KEY,
    deal_name TEXT,
    deal_stage TEXT,
    pipeline TEXT,
    amount TEXT,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS hubspot_companies (
    id TEXT PRIMARY KEY,
    name TEXT,
    domain TEXT,
    city TEXT,
    industry TEXT,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS hubspot_associations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_object_type TEXT NOT NULL,
    from_object_id TEXT NOT NULL,
    to_object_type TEXT NOT NULL,
    to_object_id TEXT NOT NULL,
    association_type TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

-- Create indexes for session-based queries and performance
CREATE INDEX IF NOT EXISTS idx_hubspot_contacts_session ON hubspot_contacts(session_id);
CREATE INDEX IF NOT EXISTS idx_hubspot_contacts_email ON hubspot_contacts(email, session_id);
CREATE INDEX IF NOT EXISTS idx_hubspot_deals_session ON hubspot_deals(session_id);
CREATE INDEX IF NOT EXISTS idx_hubspot_companies_session ON hubspot_companies(session_id);
CREATE INDEX IF NOT EXISTS idx_hubspot_companies_domain ON hubspot_companies(domain, session_id);
CREATE INDEX IF NOT EXISTS idx_hubspot_associations_session ON hubspot_associations(session_id);
CREATE INDEX IF NOT EXISTS idx_hubspot_associations_from ON hubspot_associations(from_object_type, from_object_id, session_id);
CREATE INDEX IF NOT EXISTS idx_hubspot_associations_to ON hubspot_associations(to_object_type, to_object_id, session_id);

-- +goose Down
DROP INDEX IF EXISTS idx_hubspot_associations_to;
DROP INDEX IF EXISTS idx_hubspot_associations_from;
DROP INDEX IF EXISTS idx_hubspot_associations_session;
DROP INDEX IF EXISTS idx_hubspot_companies_domain;
DROP INDEX IF EXISTS idx_hubspot_companies_session;
DROP INDEX IF EXISTS idx_hubspot_deals_session;
DROP INDEX IF EXISTS idx_hubspot_contacts_email;
DROP INDEX IF EXISTS idx_hubspot_contacts_session;
DROP TABLE IF EXISTS hubspot_associations;
DROP TABLE IF EXISTS hubspot_companies;
DROP TABLE IF EXISTS hubspot_deals;
DROP TABLE IF EXISTS hubspot_contacts;
