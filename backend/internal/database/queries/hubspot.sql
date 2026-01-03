-- Contacts queries
-- name: CreateHubspotContact :one
INSERT INTO hubspot_contacts (id, email, first_name, last_name, mobile_phone, website, session_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, email, first_name, last_name, mobile_phone, website, created_at, updated_at;

-- name: GetHubspotContactByID :one
SELECT id, email, first_name, last_name, mobile_phone, website, created_at, updated_at
FROM hubspot_contacts
WHERE id = ? AND session_id = ?;

-- name: UpdateHubspotContact :exec
UPDATE hubspot_contacts
SET email = COALESCE(?, email),
    first_name = COALESCE(?, first_name),
    last_name = COALESCE(?, last_name),
    mobile_phone = COALESCE(?, mobile_phone),
    updated_at = ?
WHERE id = ? AND session_id = ?;

-- name: SearchHubspotContactsByEmail :many
SELECT id, email, first_name, last_name, mobile_phone, website, created_at, updated_at
FROM hubspot_contacts
WHERE email = ? AND session_id = ?;

-- name: ListHubspotContacts :many
SELECT id, email, first_name, last_name, mobile_phone, website, created_at, updated_at
FROM hubspot_contacts
WHERE session_id = ?
ORDER BY created_at DESC;

-- Deals queries
-- name: CreateHubspotDeal :one
INSERT INTO hubspot_deals (id, deal_name, deal_stage, pipeline, amount, session_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, deal_name, deal_stage, pipeline, amount, created_at, updated_at;

-- name: GetHubspotDealByID :one
SELECT id, deal_name, deal_stage, pipeline, amount, created_at, updated_at
FROM hubspot_deals
WHERE id = ? AND session_id = ?;

-- name: UpdateHubspotDeal :exec
UPDATE hubspot_deals
SET deal_name = COALESCE(?, deal_name),
    deal_stage = COALESCE(?, deal_stage),
    amount = COALESCE(?, amount),
    updated_at = ?
WHERE id = ? AND session_id = ?;

-- name: ListHubspotDeals :many
SELECT id, deal_name, deal_stage, pipeline, amount, created_at, updated_at
FROM hubspot_deals
WHERE session_id = ?
ORDER BY created_at DESC;

-- Companies queries
-- name: CreateHubspotCompany :one
INSERT INTO hubspot_companies (id, name, domain, city, industry, session_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, name, domain, city, industry, created_at, updated_at;

-- name: GetHubspotCompanyByID :one
SELECT id, name, domain, city, industry, created_at, updated_at
FROM hubspot_companies
WHERE id = ? AND session_id = ?;

-- name: UpdateHubspotCompany :exec
UPDATE hubspot_companies
SET name = COALESCE(?, name),
    domain = COALESCE(?, domain),
    city = COALESCE(?, city),
    industry = COALESCE(?, industry),
    updated_at = ?
WHERE id = ? AND session_id = ?;

-- name: ListHubspotCompanies :many
SELECT id, name, domain, city, industry, created_at, updated_at
FROM hubspot_companies
WHERE session_id = ?
ORDER BY created_at DESC;

-- Associations queries
-- name: CreateHubspotAssociation :exec
INSERT INTO hubspot_associations (from_object_type, from_object_id, to_object_type, to_object_id, association_type, session_id)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetHubspotAssociations :many
SELECT from_object_type, from_object_id, to_object_type, to_object_id, association_type, created_at
FROM hubspot_associations
WHERE from_object_type = ? AND from_object_id = ? AND session_id = ?;

-- name: GetHubspotAssociationsByType :many
SELECT from_object_type, from_object_id, to_object_type, to_object_id, association_type, created_at
FROM hubspot_associations
WHERE from_object_type = ? AND from_object_id = ? AND to_object_type = ? AND session_id = ?;

-- Session management
-- name: DeleteHubspotSessionData :exec
DELETE FROM hubspot_associations WHERE session_id = ?;
DELETE FROM hubspot_companies WHERE session_id = ?;
DELETE FROM hubspot_deals WHERE session_id = ?;
DELETE FROM hubspot_contacts WHERE session_id = ?;

-- UI data queries
-- name: ListHubspotContactsBySession :many
SELECT id, email, first_name, last_name, mobile_phone, website, created_at, updated_at
FROM hubspot_contacts
WHERE session_id = ?
ORDER BY created_at DESC;
