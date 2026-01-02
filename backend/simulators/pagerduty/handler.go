package pagerduty

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
)

// PagerDuty API response structures
type APIObject struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Summary string `json:"summary,omitempty"`
	Self    string `json:"self,omitempty"`
	HTMLURL string `json:"html_url,omitempty"`
}

type APIDetails struct {
	Type    string `json:"type"`
	Details string `json:"details,omitempty"`
}

type Incident struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Title       string     `json:"title"`
	Service     APIObject  `json:"service"`
	Urgency     string     `json:"urgency"`
	Status      string     `json:"status"`
	Body        *APIDetails `json:"body,omitempty"`
	CreatedAt   string     `json:"created_at"`
	UpdatedAt   string     `json:"updated_at,omitempty"`
	HTMLURL     string     `json:"html_url,omitempty"`
	Self        string     `json:"self,omitempty"`
}

type CreateIncidentRequest struct {
	Incident CreateIncidentOptions `json:"incident"`
}

type CreateIncidentOptions struct {
	Type    string     `json:"type"`
	Title   string     `json:"title"`
	Service APIObject  `json:"service"`
	Urgency string     `json:"urgency,omitempty"`
	Body    *APIDetails `json:"body,omitempty"`
}

type CreateIncidentResponse struct {
	Incident Incident `json:"incident"`
}

type GetIncidentResponse struct {
	Incident Incident `json:"incident"`
}

type ListIncidentsResponse struct {
	Incidents []Incident `json:"incidents"`
	Limit     int        `json:"limit"`
	Offset    int        `json:"offset"`
	More      bool       `json:"more"`
	Total     int        `json:"total,omitempty"`
}

type ManageIncidentsRequest struct {
	Incidents []ManageIncidentOptions `json:"incidents"`
}

type ManageIncidentOptions struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

type ManageIncidentsResponse struct {
	Incidents []Incident `json:"incidents"`
}

type Service struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Self    string `json:"self,omitempty"`
	HTMLURL string `json:"html_url,omitempty"`
}

type ListServicesResponse struct {
	Services []Service `json:"services"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
	More     bool      `json:"more"`
	Total    int       `json:"total,omitempty"`
}

type EscalationPolicy struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Self    string `json:"self,omitempty"`
	HTMLURL string `json:"html_url,omitempty"`
}

type ListEscalationPoliciesResponse struct {
	EscalationPolicies []EscalationPolicy `json:"escalation_policies"`
	Limit              int                `json:"limit"`
	Offset             int                `json:"offset"`
	More               bool               `json:"more"`
	Total              int                `json:"total,omitempty"`
}

type User struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Email   string `json:"email"`
	Summary string `json:"summary,omitempty"`
	Self    string `json:"self,omitempty"`
	HTMLURL string `json:"html_url,omitempty"`
}

type OnCall struct {
	User             User             `json:"user"`
	EscalationPolicy EscalationPolicy `json:"escalation_policy"`
	EscalationLevel  int              `json:"escalation_level"`
	Start            string           `json:"start,omitempty"`
	End              string           `json:"end,omitempty"`
}

type ListOnCallsResponse struct {
	OnCalls []OnCall `json:"oncalls"`
	Limit   int      `json:"limit"`
	Offset  int      `json:"offset"`
	More    bool     `json:"more"`
	Total   int      `json:"total,omitempty"`
}

// Handler implements the PagerDuty simulator HTTP handler
type Handler struct {
	queries *database.Queries
}

// NewHandler creates a new PagerDuty simulator handler
func NewHandler(queries *database.Queries) *Handler {
	return &Handler{
		queries: queries,
	}
}

// ServeHTTP implements http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[pagerduty] → %s %s", r.Method, r.URL.Path)

	// Route PagerDuty API requests
	path := strings.TrimPrefix(r.URL.Path, "/")

	switch {
	case strings.HasPrefix(path, "incidents") && r.Method == http.MethodPost && !strings.Contains(path, "/"):
		h.handleCreateIncident(w, r)
	case strings.HasPrefix(path, "incidents/") && r.Method == http.MethodGet:
		// Extract incident ID from path
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			incidentID := parts[1]
			h.handleGetIncident(w, r, incidentID)
		} else {
			http.Error(w, "Invalid incident ID", http.StatusBadRequest)
		}
	case strings.HasPrefix(path, "incidents/") && r.Method == http.MethodPut:
		// PUT /incidents/{id} - used by ManageIncidentsWithContext
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			incidentID := parts[1]
			h.handleUpdateIncidentStatus(w, r, incidentID)
		} else {
			http.Error(w, "Invalid incident ID", http.StatusBadRequest)
		}
	case path == "incidents" && r.Method == http.MethodPut:
		// PUT /incidents - used by ManageIncidentsWithContext for bulk updates
		h.handleManageIncidents(w, r)
	case path == "incidents" && r.Method == http.MethodGet:
		h.handleListIncidents(w, r)
	case path == "services" && r.Method == http.MethodGet:
		h.handleListServices(w, r)
	case path == "escalation_policies" && r.Method == http.MethodGet:
		h.handleListEscalationPolicies(w, r)
	case path == "oncalls" && r.Method == http.MethodGet:
		h.handleListOnCalls(w, r)
	default:
		log.Printf("[pagerduty] ✗ Unhandled route: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}
}

func (h *Handler) handleCreateIncident(w http.ResponseWriter, r *http.Request) {
	log.Println("[pagerduty] → Received create incident request")

	var req CreateIncidentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[pagerduty] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Generate incident ID
	incidentID := generateID()
	now := time.Now()

	// Default values
	status := "triggered"
	urgency := req.Incident.Urgency
	if urgency == "" {
		urgency = "high"
	}

	bodyDetails := ""
	if req.Incident.Body != nil {
		bodyDetails = req.Incident.Body.Details
	}

	// Store incident in database
	dbIncident, err := h.queries.CreatePagerDutyIncident(context.Background(), database.CreatePagerDutyIncidentParams{
		ID:          incidentID,
		Title:       req.Incident.Title,
		ServiceID:   req.Incident.Service.ID,
		Urgency:     urgency,
		Status:      status,
		BodyDetails: sql.NullString{String: bodyDetails, Valid: bodyDetails != ""},
		SessionID:   sessionID,
		CreatedAt:   now.Unix(),
		UpdatedAt:   now.Unix(),
	})

	if err != nil {
		log.Printf("[pagerduty] ✗ Failed to store incident: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build response
	incident := Incident{
		ID:      dbIncident.ID,
		Type:    "incident",
		Title:   dbIncident.Title,
		Service: APIObject{
			ID:   dbIncident.ServiceID,
			Type: "service_reference",
		},
		Urgency:   dbIncident.Urgency,
		Status:    dbIncident.Status,
		CreatedAt: time.Unix(dbIncident.CreatedAt, 0).Format(time.RFC3339),
		UpdatedAt: time.Unix(dbIncident.UpdatedAt, 0).Format(time.RFC3339),
		HTMLURL:   "https://example.pagerduty.com/incidents/" + incidentID,
		Self:      "https://api.pagerduty.com/incidents/" + incidentID,
	}

	if dbIncident.BodyDetails.Valid && dbIncident.BodyDetails.String != "" {
		incident.Body = &APIDetails{
			Type:    "incident_body",
			Details: dbIncident.BodyDetails.String,
		}
	}

	response := CreateIncidentResponse{
		Incident: incident,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[pagerduty] ✓ Incident created: %s", incidentID)
}

func (h *Handler) handleGetIncident(w http.ResponseWriter, r *http.Request, incidentID string) {
	log.Printf("[pagerduty] → Received get incident request for ID: %s", incidentID)

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Query incident from database
	dbIncident, err := h.queries.GetPagerDutyIncidentByID(context.Background(), database.GetPagerDutyIncidentByIDParams{
		ID:        incidentID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[pagerduty] ✗ Failed to get incident: %v", err)
		http.NotFound(w, r)
		return
	}

	// Build response
	incident := Incident{
		ID:      dbIncident.ID,
		Type:    "incident",
		Title:   dbIncident.Title,
		Service: APIObject{
			ID:   dbIncident.ServiceID,
			Type: "service_reference",
		},
		Urgency:   dbIncident.Urgency,
		Status:    dbIncident.Status,
		CreatedAt: time.Unix(dbIncident.CreatedAt, 0).Format(time.RFC3339),
		UpdatedAt: time.Unix(dbIncident.UpdatedAt, 0).Format(time.RFC3339),
		HTMLURL:   "https://example.pagerduty.com/incidents/" + incidentID,
		Self:      "https://api.pagerduty.com/incidents/" + incidentID,
	}

	if dbIncident.BodyDetails.Valid && dbIncident.BodyDetails.String != "" {
		incident.Body = &APIDetails{
			Type:    "incident_body",
			Details: dbIncident.BodyDetails.String,
		}
	}

	response := GetIncidentResponse{
		Incident: incident,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[pagerduty] ✓ Returned incident: %s", incidentID)
}

func (h *Handler) handleUpdateIncidentStatus(w http.ResponseWriter, r *http.Request, incidentID string) {
	log.Printf("[pagerduty] → Received update incident status request for ID: %s", incidentID)

	var req struct {
		Incident struct {
			Status string `json:"status"`
		} `json:"incident"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[pagerduty] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Update incident status
	now := time.Now()
	err := h.queries.UpdatePagerDutyIncidentStatus(context.Background(), database.UpdatePagerDutyIncidentStatusParams{
		Status:    req.Incident.Status,
		UpdatedAt: now.Unix(),
		ID:        incidentID,
		SessionID: sessionID,
	})

	if err != nil {
		log.Printf("[pagerduty] ✗ Failed to update incident: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return updated incident
	h.handleGetIncident(w, r, incidentID)
	log.Printf("[pagerduty] ✓ Updated incident status: %s -> %s", incidentID, req.Incident.Status)
}

func (h *Handler) handleManageIncidents(w http.ResponseWriter, r *http.Request) {
	log.Println("[pagerduty] → Received manage incidents request")

	var req ManageIncidentsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[pagerduty] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Update each incident
	updatedIncidents := make([]Incident, 0, len(req.Incidents))
	now := time.Now()

	for _, incidentUpdate := range req.Incidents {
		err := h.queries.UpdatePagerDutyIncidentStatus(context.Background(), database.UpdatePagerDutyIncidentStatusParams{
			Status:    incidentUpdate.Status,
			UpdatedAt: now.Unix(),
			ID:        incidentUpdate.ID,
			SessionID: sessionID,
		})

		if err != nil {
			log.Printf("[pagerduty] ✗ Failed to update incident %s: %v", incidentUpdate.ID, err)
			continue
		}

		// Get updated incident
		dbIncident, err := h.queries.GetPagerDutyIncidentByID(context.Background(), database.GetPagerDutyIncidentByIDParams{
			ID:        incidentUpdate.ID,
			SessionID: sessionID,
		})
		if err != nil {
			log.Printf("[pagerduty] ✗ Failed to get incident %s: %v", incidentUpdate.ID, err)
			continue
		}

		incident := Incident{
			ID:      dbIncident.ID,
			Type:    "incident",
			Title:   dbIncident.Title,
			Service: APIObject{
				ID:   dbIncident.ServiceID,
				Type: "service_reference",
			},
			Urgency:   dbIncident.Urgency,
			Status:    dbIncident.Status,
			CreatedAt: time.Unix(dbIncident.CreatedAt, 0).Format(time.RFC3339),
			UpdatedAt: time.Unix(dbIncident.UpdatedAt, 0).Format(time.RFC3339),
		}

		if dbIncident.BodyDetails.Valid && dbIncident.BodyDetails.String != "" {
			incident.Body = &APIDetails{
				Type:    "incident_body",
				Details: dbIncident.BodyDetails.String,
			}
		}

		updatedIncidents = append(updatedIncidents, incident)
	}

	response := ManageIncidentsResponse{
		Incidents: updatedIncidents,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[pagerduty] ✓ Managed %d incidents", len(updatedIncidents))
}

func (h *Handler) handleListIncidents(w http.ResponseWriter, r *http.Request) {
	log.Println("[pagerduty] → Received list incidents request")

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Query incidents from database
	dbIncidents, err := h.queries.ListPagerDutyIncidents(context.Background(), sessionID)
	if err != nil {
		log.Printf("[pagerduty] ✗ Failed to list incidents: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	incidents := make([]Incident, 0, len(dbIncidents))
	for _, dbIncident := range dbIncidents {
		incident := Incident{
			ID:      dbIncident.ID,
			Type:    "incident",
			Title:   dbIncident.Title,
			Service: APIObject{
				ID:   dbIncident.ServiceID,
				Type: "service_reference",
			},
			Urgency:   dbIncident.Urgency,
			Status:    dbIncident.Status,
			CreatedAt: time.Unix(dbIncident.CreatedAt, 0).Format(time.RFC3339),
			UpdatedAt: time.Unix(dbIncident.UpdatedAt, 0).Format(time.RFC3339),
		}

		if dbIncident.BodyDetails.Valid && dbIncident.BodyDetails.String != "" {
			incident.Body = &APIDetails{
				Type:    "incident_body",
				Details: dbIncident.BodyDetails.String,
			}
		}

		incidents = append(incidents, incident)
	}

	response := ListIncidentsResponse{
		Incidents: incidents,
		Limit:     100,
		Offset:    0,
		More:      false,
		Total:     len(incidents),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[pagerduty] ✓ Listed %d incidents", len(incidents))
}

func (h *Handler) handleListServices(w http.ResponseWriter, r *http.Request) {
	log.Println("[pagerduty] → Received list services request")

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Query services from database
	dbServices, err := h.queries.ListPagerDutyServices(context.Background(), sessionID)
	if err != nil {
		log.Printf("[pagerduty] ✗ Failed to list services: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	services := make([]Service, 0, len(dbServices))
	for _, dbService := range dbServices {
		service := Service{
			ID:      dbService.ID,
			Type:    "service",
			Name:    dbService.Name,
			Self:    "https://api.pagerduty.com/services/" + dbService.ID,
			HTMLURL: "https://example.pagerduty.com/services/" + dbService.ID,
		}
		services = append(services, service)
	}

	response := ListServicesResponse{
		Services: services,
		Limit:    100,
		Offset:   0,
		More:     false,
		Total:    len(services),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[pagerduty] ✓ Listed %d services", len(services))
}

func (h *Handler) handleListEscalationPolicies(w http.ResponseWriter, r *http.Request) {
	log.Println("[pagerduty] → Received list escalation policies request")

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Query escalation policies from database
	dbPolicies, err := h.queries.ListPagerDutyEscalationPolicies(context.Background(), sessionID)
	if err != nil {
		log.Printf("[pagerduty] ✗ Failed to list escalation policies: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	policies := make([]EscalationPolicy, 0, len(dbPolicies))
	for _, dbPolicy := range dbPolicies {
		policy := EscalationPolicy{
			ID:      dbPolicy.ID,
			Type:    "escalation_policy",
			Name:    dbPolicy.Name,
			Self:    "https://api.pagerduty.com/escalation_policies/" + dbPolicy.ID,
			HTMLURL: "https://example.pagerduty.com/escalation_policies/" + dbPolicy.ID,
		}
		policies = append(policies, policy)
	}

	response := ListEscalationPoliciesResponse{
		EscalationPolicies: policies,
		Limit:              100,
		Offset:             0,
		More:               false,
		Total:              len(policies),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[pagerduty] ✓ Listed %d escalation policies", len(policies))
}

func (h *Handler) handleListOnCalls(w http.ResponseWriter, r *http.Request) {
	log.Println("[pagerduty] → Received list oncalls request")

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Query oncalls from database
	dbOnCalls, err := h.queries.ListPagerDutyOnCalls(context.Background(), sessionID)
	if err != nil {
		log.Printf("[pagerduty] ✗ Failed to list oncalls: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	oncalls := make([]OnCall, 0, len(dbOnCalls))
	for _, dbOnCall := range dbOnCalls {
		// Get escalation policy details
		epID := dbOnCall.EscalationPolicyID
		ep, err := h.queries.GetPagerDutyEscalationPolicyByID(context.Background(), database.GetPagerDutyEscalationPolicyByIDParams{
			ID:        epID,
			SessionID: sessionID,
		})

		epName := "Default"
		if err == nil {
			epName = ep.Name
		}

		oncall := OnCall{
			User: User{
				ID:      strconv.FormatInt(dbOnCall.CreatedAt, 10), // Use timestamp as user ID for simplicity
				Type:    "user_reference",
				Email:   dbOnCall.UserEmail,
				Summary: dbOnCall.UserEmail,
			},
			EscalationPolicy: EscalationPolicy{
				ID:   epID,
				Type: "escalation_policy_reference",
				Name: epName,
			},
			EscalationLevel: 1,
		}
		oncalls = append(oncalls, oncall)
	}

	response := ListOnCallsResponse{
		OnCalls: oncalls,
		Limit:   100,
		Offset:  0,
		More:    false,
		Total:   len(oncalls),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[pagerduty] ✓ Listed %d oncalls", len(oncalls))
}

// Helper functions

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
