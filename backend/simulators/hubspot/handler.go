package hubspot

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
)

// HubSpot API response structures matching go-hubspot library

type HsStr string

func NewString(s string) *HsStr {
	hs := HsStr(s)
	return &hs
}

// Contact represents HubSpot contact properties
type Contact struct {
	Email       *HsStr `json:"email,omitempty"`
	FirstName   *HsStr `json:"firstname,omitempty"`
	LastName    *HsStr `json:"lastname,omitempty"`
	MobilePhone *HsStr `json:"mobilephone,omitempty"`
	Website     *HsStr `json:"website,omitempty"`
}

// Deal represents HubSpot deal properties
type Deal struct {
	DealName  *HsStr `json:"dealname,omitempty"`
	DealStage *HsStr `json:"dealstage,omitempty"`
	PipeLine  *HsStr `json:"pipeline,omitempty"`
	Amount    *HsStr `json:"amount,omitempty"`
}

// Company represents HubSpot company properties
type Company struct {
	Name     *HsStr `json:"name,omitempty"`
	Domain   *HsStr `json:"domain,omitempty"`
	City     *HsStr `json:"city,omitempty"`
	Industry *HsStr `json:"industry,omitempty"`
}

// ResponseResource is the generic response wrapper
type ResponseResource struct {
	ID         string      `json:"id"`
	Properties interface{} `json:"properties"`
	CreatedAt  string      `json:"createdAt"`
	UpdatedAt  string      `json:"updatedAt"`
	Archived   bool        `json:"archived"`
}

// SearchResponse is the search result wrapper
type SearchResponse struct {
	Total   int                `json:"total"`
	Results []ResponseResource `json:"results"`
}

// CreateRequest is the generic create/update request
type CreateRequest struct {
	Properties interface{} `json:"properties"`
}

// AssociationRequest for creating associations
type AssociationRequest struct {
	Inputs []AssociationInput `json:"inputs"`
}

type AssociationInput struct {
	From struct {
		ID string `json:"id"`
	} `json:"from"`
	To struct {
		ID string `json:"id"`
	} `json:"to"`
	Types []struct {
		AssociationCategory string `json:"associationCategory"`
		AssociationTypeID   int    `json:"associationTypeId"`
	} `json:"types"`
}

// Handler implements the HubSpot simulator HTTP handler
type Handler struct {
	queries *database.Queries
}

// NewHandler creates a new HubSpot simulator handler
func NewHandler(queries *database.Queries) *Handler {
	return &Handler{
		queries: queries,
	}
}

// ServeHTTP implements http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[hubspot] → %s %s", r.Method, r.URL.Path)

	// Route HubSpot API requests (paths come with /crm prefix from the HubSpot client library)
	// Check for associations first before checking object-specific paths
	switch {
	case strings.Contains(r.URL.Path, "/associations/"):
		// Handle association path format: /crm/v3/objects/{objectType}/{objectId}/associations/{toObjectType}/{toObjectId}/{associationType}
		h.handleAssociationV3(w, r)
	case strings.HasPrefix(r.URL.Path, "/crm/v4/associations/"):
		h.handleAssociations(w, r)
	case strings.HasPrefix(r.URL.Path, "/crm/v3/objects/contacts"):
		h.handleContacts(w, r)
	case strings.HasPrefix(r.URL.Path, "/crm/v3/objects/deals"):
		h.handleDeals(w, r)
	case strings.HasPrefix(r.URL.Path, "/crm/v3/objects/companies"):
		h.handleCompanies(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleContacts(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/crm/v3/objects/contacts")

	switch r.Method {
	case http.MethodPost:
		if strings.HasSuffix(path, "/search") {
			h.handleSearchContacts(w, r)
		} else {
			h.handleCreateContact(w, r)
		}
	case http.MethodGet:
		if path == "" || path == "/" {
			h.handleListContacts(w, r)
		} else {
			// Extract contact ID from path
			contactID := strings.TrimPrefix(path, "/")
			h.handleGetContact(w, r, contactID)
		}
	case http.MethodPatch:
		// Extract contact ID from path
		contactID := strings.TrimPrefix(path, "/")
		h.handleUpdateContact(w, r, contactID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleDeals(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/crm/v3/objects/deals")

	switch r.Method {
	case http.MethodPost:
		h.handleCreateDeal(w, r)
	case http.MethodGet:
		if path == "" || path == "/" {
			h.handleListDeals(w, r)
		} else {
			// Extract deal ID from path
			dealID := strings.TrimPrefix(path, "/")
			h.handleGetDeal(w, r, dealID)
		}
	case http.MethodPatch:
		// Extract deal ID from path
		dealID := strings.TrimPrefix(path, "/")
		h.handleUpdateDeal(w, r, dealID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleCompanies(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/crm/v3/objects/companies")

	switch r.Method {
	case http.MethodPost:
		h.handleCreateCompany(w, r)
	case http.MethodGet:
		if path == "" || path == "/" {
			h.handleListCompanies(w, r)
		} else {
			// Extract company ID from path
			companyID := strings.TrimPrefix(path, "/")
			h.handleGetCompany(w, r, companyID)
		}
	case http.MethodPatch:
		// Extract company ID from path
		companyID := strings.TrimPrefix(path, "/")
		h.handleUpdateCompany(w, r, companyID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleAssociations(w http.ResponseWriter, r *http.Request) {
	// Path format: /crm/v4/associations/{fromObjectType}/{toObjectType}/batch/create
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract object types from path
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/crm/v4/associations/"), "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid association path", http.StatusBadRequest)
		return
	}

	fromObjectType := parts[0]
	toObjectType := parts[1]

	h.handleCreateAssociation(w, r, fromObjectType, toObjectType)
}

// Contact handlers

func (h *Handler) handleCreateContact(w http.ResponseWriter, r *http.Request) {
	log.Println("[hubspot] → Creating contact")

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[hubspot] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Parse properties
	propsJSON, _ := json.Marshal(req.Properties)
	var contact Contact
	if err := json.Unmarshal(propsJSON, &contact); err != nil {
		log.Printf("[hubspot] ✗ Failed to parse properties: %v", err)
		http.Error(w, "Invalid properties", http.StatusBadRequest)
		return
	}

	contactID := generateID()
	sessionID := session.FromContext(r.Context())
	now := time.Now().UnixMilli()

	// Store in database
	dbContact, err := h.queries.CreateHubspotContact(context.Background(), database.CreateHubspotContactParams{
		ID:          contactID,
		Email:       sqlNullString(contact.Email),
		FirstName:   sqlNullString(contact.FirstName),
		LastName:    sqlNullString(contact.LastName),
		MobilePhone: sqlNullString(contact.MobilePhone),
		Website:     sqlNullString(contact.Website),
		SessionID:   sessionID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		log.Printf("[hubspot] ✗ Failed to create contact: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := ResponseResource{
		ID:         dbContact.ID,
		Properties: buildContact(dbContact.Email, dbContact.FirstName, dbContact.LastName, dbContact.MobilePhone, dbContact.Website),
		CreatedAt:  formatTimestamp(dbContact.CreatedAt),
		UpdatedAt:  formatTimestamp(dbContact.UpdatedAt),
		Archived:   false,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[hubspot] ✓ Contact created: %s", contactID)
}

func (h *Handler) handleGetContact(w http.ResponseWriter, r *http.Request, contactID string) {
	log.Printf("[hubspot] → Getting contact: %s", contactID)

	sessionID := session.FromContext(r.Context())

	dbContact, err := h.queries.GetHubspotContactByID(context.Background(), database.GetHubspotContactByIDParams{
		ID:        contactID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[hubspot] ✗ Failed to get contact: %v", err)
		http.NotFound(w, r)
		return
	}

	response := ResponseResource{
		ID:         dbContact.ID,
		Properties: buildContact(dbContact.Email, dbContact.FirstName, dbContact.LastName, dbContact.MobilePhone, dbContact.Website),
		CreatedAt:  formatTimestamp(dbContact.CreatedAt),
		UpdatedAt:  formatTimestamp(dbContact.UpdatedAt),
		Archived:   false,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[hubspot] ✓ Contact retrieved: %s", contactID)
}

func (h *Handler) handleUpdateContact(w http.ResponseWriter, r *http.Request, contactID string) {
	log.Printf("[hubspot] → Updating contact: %s", contactID)

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[hubspot] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Parse properties
	propsJSON, _ := json.Marshal(req.Properties)
	var contact Contact
	if err := json.Unmarshal(propsJSON, &contact); err != nil {
		log.Printf("[hubspot] ✗ Failed to parse properties: %v", err)
		http.Error(w, "Invalid properties", http.StatusBadRequest)
		return
	}

	sessionID := session.FromContext(r.Context())
	now := time.Now().UnixMilli()

	// Update in database
	err := h.queries.UpdateHubspotContact(context.Background(), database.UpdateHubspotContactParams{
		Email:       sqlNullString(contact.Email),
		FirstName:   sqlNullString(contact.FirstName),
		LastName:    sqlNullString(contact.LastName),
		MobilePhone: sqlNullString(contact.MobilePhone),
		UpdatedAt:   now,
		ID:          contactID,
		SessionID:   sessionID,
	})
	if err != nil {
		log.Printf("[hubspot] ✗ Failed to update contact: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get updated contact
	dbContact, err := h.queries.GetHubspotContactByID(context.Background(), database.GetHubspotContactByIDParams{
		ID:        contactID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[hubspot] ✗ Failed to get updated contact: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := ResponseResource{
		ID:         dbContact.ID,
		Properties: buildContact(dbContact.Email, dbContact.FirstName, dbContact.LastName, dbContact.MobilePhone, dbContact.Website),
		CreatedAt:  formatTimestamp(dbContact.CreatedAt),
		UpdatedAt:  formatTimestamp(dbContact.UpdatedAt),
		Archived:   false,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[hubspot] ✓ Contact updated: %s", contactID)
}

func (h *Handler) handleSearchContacts(w http.ResponseWriter, r *http.Request) {
	log.Println("[hubspot] → Searching contacts")

	var searchReq struct {
		FilterGroups []struct {
			Filters []struct {
				PropertyName string  `json:"propertyName"`
				Operator     string  `json:"operator"`
				Value        *HsStr  `json:"value"`
			} `json:"filters"`
		} `json:"filterGroups"`
		Limit int `json:"limit"`
	}

	if err := json.NewDecoder(r.Body).Decode(&searchReq); err != nil {
		log.Printf("[hubspot] ✗ Failed to decode search request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	sessionID := session.FromContext(r.Context())

	// Extract email from filter
	var email string
	if len(searchReq.FilterGroups) > 0 && len(searchReq.FilterGroups[0].Filters) > 0 {
		filter := searchReq.FilterGroups[0].Filters[0]
		if filter.PropertyName == "email" && filter.Value != nil {
			email = string(*filter.Value)
		}
	}

	// Search contacts
	dbContacts, err := h.queries.SearchHubspotContactsByEmail(context.Background(), database.SearchHubspotContactsByEmailParams{
		Email:     sql.NullString{String: email, Valid: email != ""},
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[hubspot] ✗ Failed to search contacts: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	results := make([]ResponseResource, 0, len(dbContacts))
	for i := range dbContacts {
		results = append(results, ResponseResource{
			ID:         dbContacts[i].ID,
			Properties: buildContact(dbContacts[i].Email, dbContacts[i].FirstName, dbContacts[i].LastName, dbContacts[i].MobilePhone, dbContacts[i].Website),
			CreatedAt:  formatTimestamp(dbContacts[i].CreatedAt),
			UpdatedAt:  formatTimestamp(dbContacts[i].UpdatedAt),
			Archived:   false,
		})
	}

	response := SearchResponse{
		Total:   len(results),
		Results: results,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[hubspot] ✓ Found %d contacts", len(results))
}

func (h *Handler) handleListContacts(w http.ResponseWriter, r *http.Request) {
	log.Println("[hubspot] → Listing contacts")

	sessionID := session.FromContext(r.Context())

	dbContacts, err := h.queries.ListHubspotContacts(context.Background(), sessionID)
	if err != nil {
		log.Printf("[hubspot] ✗ Failed to list contacts: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	results := make([]ResponseResource, 0, len(dbContacts))
	for i := range dbContacts {
		results = append(results, ResponseResource{
			ID:         dbContacts[i].ID,
			Properties: buildContact(dbContacts[i].Email, dbContacts[i].FirstName, dbContacts[i].LastName, dbContacts[i].MobilePhone, dbContacts[i].Website),
			CreatedAt:  formatTimestamp(dbContacts[i].CreatedAt),
			UpdatedAt:  formatTimestamp(dbContacts[i].UpdatedAt),
			Archived:   false,
		})
	}

	response := SearchResponse{
		Total:   len(results),
		Results: results,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[hubspot] ✓ Listed %d contacts", len(results))
}

// Deal handlers

func (h *Handler) handleCreateDeal(w http.ResponseWriter, r *http.Request) {
	log.Println("[hubspot] → Creating deal")

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[hubspot] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Parse properties
	propsJSON, _ := json.Marshal(req.Properties)
	var deal Deal
	if err := json.Unmarshal(propsJSON, &deal); err != nil {
		log.Printf("[hubspot] ✗ Failed to parse properties: %v", err)
		http.Error(w, "Invalid properties", http.StatusBadRequest)
		return
	}

	dealID := generateID()
	sessionID := session.FromContext(r.Context())
	now := time.Now().UnixMilli()

	// Store in database
	dbDeal, err := h.queries.CreateHubspotDeal(context.Background(), database.CreateHubspotDealParams{
		ID:        dealID,
		DealName:  sqlNullString(deal.DealName),
		DealStage: sqlNullString(deal.DealStage),
		Pipeline:  sqlNullString(deal.PipeLine),
		Amount:    sqlNullString(deal.Amount),
		SessionID: sessionID,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		log.Printf("[hubspot] ✗ Failed to create deal: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := ResponseResource{
		ID:         dbDeal.ID,
		Properties: buildDeal(dbDeal.DealName, dbDeal.DealStage, dbDeal.Pipeline, dbDeal.Amount),
		CreatedAt:  formatTimestamp(dbDeal.CreatedAt),
		UpdatedAt:  formatTimestamp(dbDeal.UpdatedAt),
		Archived:   false,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[hubspot] ✓ Deal created: %s", dealID)
}

func (h *Handler) handleGetDeal(w http.ResponseWriter, r *http.Request, dealID string) {
	log.Printf("[hubspot] → Getting deal: %s", dealID)

	sessionID := session.FromContext(r.Context())

	dbDeal, err := h.queries.GetHubspotDealByID(context.Background(), database.GetHubspotDealByIDParams{
		ID:        dealID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[hubspot] ✗ Failed to get deal: %v", err)
		http.NotFound(w, r)
		return
	}

	response := ResponseResource{
		ID:         dbDeal.ID,
		Properties: buildDeal(dbDeal.DealName, dbDeal.DealStage, dbDeal.Pipeline, dbDeal.Amount),
		CreatedAt:  formatTimestamp(dbDeal.CreatedAt),
		UpdatedAt:  formatTimestamp(dbDeal.UpdatedAt),
		Archived:   false,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[hubspot] ✓ Deal retrieved: %s", dealID)
}

func (h *Handler) handleUpdateDeal(w http.ResponseWriter, r *http.Request, dealID string) {
	log.Printf("[hubspot] → Updating deal: %s", dealID)

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[hubspot] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Parse properties
	propsJSON, _ := json.Marshal(req.Properties)
	var deal Deal
	if err := json.Unmarshal(propsJSON, &deal); err != nil {
		log.Printf("[hubspot] ✗ Failed to parse properties: %v", err)
		http.Error(w, "Invalid properties", http.StatusBadRequest)
		return
	}

	sessionID := session.FromContext(r.Context())
	now := time.Now().UnixMilli()

	// Update in database
	err := h.queries.UpdateHubspotDeal(context.Background(), database.UpdateHubspotDealParams{
		DealName:  sqlNullString(deal.DealName),
		DealStage: sqlNullString(deal.DealStage),
		Amount:    sqlNullString(deal.Amount),
		UpdatedAt: now,
		ID:        dealID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[hubspot] ✗ Failed to update deal: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get updated deal
	dbDeal, err := h.queries.GetHubspotDealByID(context.Background(), database.GetHubspotDealByIDParams{
		ID:        dealID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[hubspot] ✗ Failed to get updated deal: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := ResponseResource{
		ID:         dbDeal.ID,
		Properties: buildDeal(dbDeal.DealName, dbDeal.DealStage, dbDeal.Pipeline, dbDeal.Amount),
		CreatedAt:  formatTimestamp(dbDeal.CreatedAt),
		UpdatedAt:  formatTimestamp(dbDeal.UpdatedAt),
		Archived:   false,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[hubspot] ✓ Deal updated: %s", dealID)
}

func (h *Handler) handleListDeals(w http.ResponseWriter, r *http.Request) {
	log.Println("[hubspot] → Listing deals")

	sessionID := session.FromContext(r.Context())

	dbDeals, err := h.queries.ListHubspotDeals(context.Background(), sessionID)
	if err != nil {
		log.Printf("[hubspot] ✗ Failed to list deals: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	results := make([]ResponseResource, 0, len(dbDeals))
	for i := range dbDeals {
		results = append(results, ResponseResource{
			ID:         dbDeals[i].ID,
			Properties: buildDeal(dbDeals[i].DealName, dbDeals[i].DealStage, dbDeals[i].Pipeline, dbDeals[i].Amount),
			CreatedAt:  formatTimestamp(dbDeals[i].CreatedAt),
			UpdatedAt:  formatTimestamp(dbDeals[i].UpdatedAt),
			Archived:   false,
		})
	}

	response := SearchResponse{
		Total:   len(results),
		Results: results,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[hubspot] ✓ Listed %d deals", len(results))
}

// Company handlers

func (h *Handler) handleCreateCompany(w http.ResponseWriter, r *http.Request) {
	log.Println("[hubspot] → Creating company")

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[hubspot] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Parse properties
	propsJSON, _ := json.Marshal(req.Properties)
	var company Company
	if err := json.Unmarshal(propsJSON, &company); err != nil {
		log.Printf("[hubspot] ✗ Failed to parse properties: %v", err)
		http.Error(w, "Invalid properties", http.StatusBadRequest)
		return
	}

	companyID := generateID()
	sessionID := session.FromContext(r.Context())
	now := time.Now().UnixMilli()

	// Store in database
	dbCompany, err := h.queries.CreateHubspotCompany(context.Background(), database.CreateHubspotCompanyParams{
		ID:        companyID,
		Name:      sqlNullString(company.Name),
		Domain:    sqlNullString(company.Domain),
		City:      sqlNullString(company.City),
		Industry:  sqlNullString(company.Industry),
		SessionID: sessionID,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		log.Printf("[hubspot] ✗ Failed to create company: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := ResponseResource{
		ID:         dbCompany.ID,
		Properties: buildCompany(dbCompany.Name, dbCompany.Domain, dbCompany.City, dbCompany.Industry),
		CreatedAt:  formatTimestamp(dbCompany.CreatedAt),
		UpdatedAt:  formatTimestamp(dbCompany.UpdatedAt),
		Archived:   false,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[hubspot] ✓ Company created: %s", companyID)
}

func (h *Handler) handleGetCompany(w http.ResponseWriter, r *http.Request, companyID string) {
	log.Printf("[hubspot] → Getting company: %s", companyID)

	sessionID := session.FromContext(r.Context())

	dbCompany, err := h.queries.GetHubspotCompanyByID(context.Background(), database.GetHubspotCompanyByIDParams{
		ID:        companyID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[hubspot] ✗ Failed to get company: %v", err)
		http.NotFound(w, r)
		return
	}

	response := ResponseResource{
		ID:         dbCompany.ID,
		Properties: buildCompany(dbCompany.Name, dbCompany.Domain, dbCompany.City, dbCompany.Industry),
		CreatedAt:  formatTimestamp(dbCompany.CreatedAt),
		UpdatedAt:  formatTimestamp(dbCompany.UpdatedAt),
		Archived:   false,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[hubspot] ✓ Company retrieved: %s", companyID)
}

func (h *Handler) handleUpdateCompany(w http.ResponseWriter, r *http.Request, companyID string) {
	log.Printf("[hubspot] → Updating company: %s", companyID)

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[hubspot] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Parse properties
	propsJSON, _ := json.Marshal(req.Properties)
	var company Company
	if err := json.Unmarshal(propsJSON, &company); err != nil {
		log.Printf("[hubspot] ✗ Failed to parse properties: %v", err)
		http.Error(w, "Invalid properties", http.StatusBadRequest)
		return
	}

	sessionID := session.FromContext(r.Context())
	now := time.Now().UnixMilli()

	// Update in database
	err := h.queries.UpdateHubspotCompany(context.Background(), database.UpdateHubspotCompanyParams{
		Name:      sqlNullString(company.Name),
		Domain:    sqlNullString(company.Domain),
		City:      sqlNullString(company.City),
		Industry:  sqlNullString(company.Industry),
		UpdatedAt: now,
		ID:        companyID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[hubspot] ✗ Failed to update company: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get updated company
	dbCompany, err := h.queries.GetHubspotCompanyByID(context.Background(), database.GetHubspotCompanyByIDParams{
		ID:        companyID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[hubspot] ✗ Failed to get updated company: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := ResponseResource{
		ID:         dbCompany.ID,
		Properties: buildCompany(dbCompany.Name, dbCompany.Domain, dbCompany.City, dbCompany.Industry),
		CreatedAt:  formatTimestamp(dbCompany.CreatedAt),
		UpdatedAt:  formatTimestamp(dbCompany.UpdatedAt),
		Archived:   false,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[hubspot] ✓ Company updated: %s", companyID)
}

func (h *Handler) handleListCompanies(w http.ResponseWriter, r *http.Request) {
	log.Println("[hubspot] → Listing companies")

	sessionID := session.FromContext(r.Context())

	dbCompanies, err := h.queries.ListHubspotCompanies(context.Background(), sessionID)
	if err != nil {
		log.Printf("[hubspot] ✗ Failed to list companies: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	results := make([]ResponseResource, 0, len(dbCompanies))
	for i := range dbCompanies {
		results = append(results, ResponseResource{
			ID:         dbCompanies[i].ID,
			Properties: buildCompany(dbCompanies[i].Name, dbCompanies[i].Domain, dbCompanies[i].City, dbCompanies[i].Industry),
			CreatedAt:  formatTimestamp(dbCompanies[i].CreatedAt),
			UpdatedAt:  formatTimestamp(dbCompanies[i].UpdatedAt),
			Archived:   false,
		})
	}

	response := SearchResponse{
		Total:   len(results),
		Results: results,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[hubspot] ✓ Listed %d companies", len(results))
}

// Association handlers

func (h *Handler) handleCreateAssociation(w http.ResponseWriter, r *http.Request, fromObjectType, toObjectType string) {
	log.Printf("[hubspot] → Creating association: %s -> %s", fromObjectType, toObjectType)

	var req AssociationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[hubspot] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	sessionID := session.FromContext(r.Context())

	// Process each association
	for _, input := range req.Inputs {
		// Determine association type
		associationType := fmt.Sprintf("%s_to_%s", fromObjectType, toObjectType)

		err := h.queries.CreateHubspotAssociation(context.Background(), database.CreateHubspotAssociationParams{
			FromObjectType:  fromObjectType,
			FromObjectID:    input.From.ID,
			ToObjectType:    toObjectType,
			ToObjectID:      input.To.ID,
			AssociationType: associationType,
			SessionID:       sessionID,
		})
		if err != nil {
			log.Printf("[hubspot] ✗ Failed to create association: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "COMPLETE",
	})
	log.Printf("[hubspot] ✓ Created %d associations", len(req.Inputs))
}

func (h *Handler) handleAssociationV3(w http.ResponseWriter, r *http.Request) {
	// Path format: /crm/v3/objects/{fromObjectType}/{fromObjectId}/associations/{toObjectType}/{toObjectId}/{associationType}
	// Example: /crm/v3/objects/deals/123/associations/contacts/456/deal_to_contact

	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the path to extract the components
	parts := strings.Split(r.URL.Path, "/")
	// parts: ["", "crm", "v3", "objects", "{fromObjectType}", "{fromObjectId}", "associations", "{toObjectType}", "{toObjectId}", "{associationType}"]

	if len(parts) < 10 {
		http.Error(w, "Invalid association path", http.StatusBadRequest)
		return
	}

	fromObjectType := parts[4]
	fromObjectID := parts[5]
	toObjectType := parts[7]
	toObjectID := parts[8]
	associationType := parts[9]

	log.Printf("[hubspot] → Creating association v3: %s/%s -> %s/%s (%s)", fromObjectType, fromObjectID, toObjectType, toObjectID, associationType)

	sessionID := session.FromContext(r.Context())

	// Create the association
	err := h.queries.CreateHubspotAssociation(context.Background(), database.CreateHubspotAssociationParams{
		FromObjectType:  fromObjectType,
		FromObjectID:    fromObjectID,
		ToObjectType:    toObjectType,
		ToObjectID:      toObjectID,
		AssociationType: associationType,
		SessionID:       sessionID,
	})
	if err != nil {
		log.Printf("[hubspot] ✗ Failed to create association: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return success response with the association type details
	response := ResponseResource{
		ID:         toObjectID,
		Properties: map[string]string{"type": associationType},
		CreatedAt:  formatTimestamp(time.Now().UnixMilli()),
		UpdatedAt:  formatTimestamp(time.Now().UnixMilli()),
		Archived:   false,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[hubspot] ✓ Created association: %s/%s -> %s/%s", fromObjectType, fromObjectID, toObjectType, toObjectID)
}

// Helper functions

func generateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if random generation fails
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func sqlNullString(hs *HsStr) sql.NullString {
	if hs == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: string(*hs), Valid: true}
}

func formatTimestamp(unixMilli int64) string {
	return time.UnixMilli(unixMilli).Format(time.RFC3339)
}

func buildContact(email, firstName, lastName, mobilePhone, website sql.NullString) Contact {
	contact := Contact{}
	if email.Valid {
		contact.Email = NewString(email.String)
	}
	if firstName.Valid {
		contact.FirstName = NewString(firstName.String)
	}
	if lastName.Valid {
		contact.LastName = NewString(lastName.String)
	}
	if mobilePhone.Valid {
		contact.MobilePhone = NewString(mobilePhone.String)
	}
	if website.Valid {
		contact.Website = NewString(website.String)
	}
	return contact
}

func buildDeal(dealName, dealStage, pipeline, amount sql.NullString) Deal {
	deal := Deal{}
	if dealName.Valid {
		deal.DealName = NewString(dealName.String)
	}
	if dealStage.Valid {
		deal.DealStage = NewString(dealStage.String)
	}
	if pipeline.Valid {
		deal.PipeLine = NewString(pipeline.String)
	}
	if amount.Valid {
		deal.Amount = NewString(amount.String)
	}
	return deal
}

func buildCompany(name, domain, city, industry sql.NullString) Company {
	company := Company{}
	if name.Valid {
		company.Name = NewString(name.String)
	}
	if domain.Valid {
		company.Domain = NewString(domain.String)
	}
	if city.Valid {
		company.City = NewString(city.String)
	}
	if industry.Valid {
		company.Industry = NewString(industry.String)
	}
	return company
}
