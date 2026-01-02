package datadog

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
)

// Datadog API response structures

// Incidents (v2 API)
type IncidentFieldAttributesSingleValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type IncidentFieldAttributes struct {
	IncidentFieldAttributesSingleValue *IncidentFieldAttributesSingleValue `json:",omitempty"`
}

type IncidentCreateAttributes struct {
	Title            string                             `json:"title"`
	CustomerImpacted bool                               `json:"customer_impacted"`
	Fields           map[string]IncidentFieldAttributes `json:"fields,omitempty"`
}

type IncidentUpdateAttributes struct {
	Title            *string                            `json:"title,omitempty"`
	CustomerImpacted *bool                              `json:"customer_impacted,omitempty"`
	Fields           map[string]IncidentFieldAttributes `json:"fields,omitempty"`
}

type IncidentCreateData struct {
	Type       string                   `json:"type"`
	Attributes IncidentCreateAttributes `json:"attributes"`
}

type IncidentUpdateData struct {
	ID         string                    `json:"id"`
	Type       string                    `json:"type"`
	Attributes *IncidentUpdateAttributes `json:"attributes,omitempty"`
}

type IncidentCreateRequest struct {
	Data IncidentCreateData `json:"data"`
}

type IncidentUpdateRequest struct {
	Data IncidentUpdateData `json:"data"`
}

type IncidentResponseAttributes struct {
	Title            string                             `json:"title"`
	CustomerImpacted *bool                              `json:"customer_impacted,omitempty"`
	Fields           map[string]IncidentFieldAttributes `json:"fields,omitempty"`
	Created          *string                            `json:"created,omitempty"`
	Modified         *string                            `json:"modified,omitempty"`
}

type IncidentResponseData struct {
	ID         string                     `json:"id"`
	Type       string                     `json:"type"`
	Attributes IncidentResponseAttributes `json:"attributes"`
}

type IncidentResponse struct {
	Data IncidentResponseData `json:"data"`
}

type IncidentListResponse struct {
	Data []IncidentResponseData `json:"data"`
}

// Monitors (v1 API)
type Monitor struct {
	ID      *int64  `json:"id,omitempty"`
	Name    *string `json:"name,omitempty"`
	Type    string  `json:"type"`
	Query   string  `json:"query"`
	Message *string `json:"message,omitempty"`
	Created *string `json:"created,omitempty"`
	Modified *string `json:"modified,omitempty"`
}

type MonitorUpdateRequest struct {
	Name    *string `json:"name,omitempty"`
	Query   *string `json:"query,omitempty"`
	Message *string `json:"message,omitempty"`
}

// Events (v1 API)
type EventCreateRequest struct {
	Title string   `json:"title"`
	Text  string   `json:"text"`
	Tags  []string `json:"tags,omitempty"`
}

type EventCreateResponse struct {
	Status *string                `json:"status,omitempty"`
	Event  *EventCreateResponseEvent `json:"event,omitempty"`
}

type EventCreateResponseEvent struct {
	ID    *int64    `json:"id,omitempty"`
	Title *string   `json:"title,omitempty"`
	Text  *string   `json:"text,omitempty"`
	Tags  []string  `json:"tags,omitempty"`
	DateHappened *int64 `json:"date_happened,omitempty"`
}

// Metrics (v2 API)
type MetricPoint struct {
	Timestamp *int64   `json:"timestamp,omitempty"`
	Value     *float64 `json:"value,omitempty"`
}

type MetricSeries struct {
	Metric string        `json:"metric"`
	Points []MetricPoint `json:"points"`
	Tags   []string      `json:"tags,omitempty"`
}

type MetricPayload struct {
	Series []MetricSeries `json:"series"`
}

type MetricSubmitResponse struct {
	Errors []string `json:"errors,omitempty"`
}

// Handler implements the Datadog simulator HTTP handler
type Handler struct {
	queries *database.Queries
}

// NewHandler creates a new Datadog simulator handler
func NewHandler(queries *database.Queries) *Handler {
	return &Handler{
		queries: queries,
	}
}

// ServeHTTP implements http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[datadog] → %s %s", r.Method, r.URL.Path)

	// Strip /datadog prefix if present (when not using http.StripPrefix in main.go)
	path := r.URL.Path
	path = strings.TrimPrefix(path, "/datadog")

	// Route Datadog API requests
	if strings.HasPrefix(path, "/api/v2/incidents") {
		h.handleIncidentsV2(w, r)
		return
	}

	if strings.HasPrefix(path, "/api/v1/monitor") {
		h.handleMonitorsV1(w, r)
		return
	}

	if strings.HasPrefix(path, "/api/v1/events") {
		h.handleEventsV1(w, r)
		return
	}

	if strings.HasPrefix(path, "/api/v2/series") {
		h.handleMetricsV2(w, r)
		return
	}

	http.NotFound(w, r)
}

// Incidents V2 handlers

func (h *Handler) handleIncidentsV2(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	path = strings.TrimPrefix(path, "/datadog")
	path = strings.TrimPrefix(path, "/api/v2/incidents")

	switch {
	case path == "" && r.Method == http.MethodPost:
		h.handleCreateIncident(w, r)
	case path == "" && r.Method == http.MethodGet:
		h.handleListIncidents(w, r)
	case strings.HasPrefix(path, "/") && r.Method == http.MethodGet:
		incidentID := strings.TrimPrefix(path, "/")
		h.handleGetIncident(w, r, incidentID)
	case strings.HasPrefix(path, "/") && r.Method == http.MethodPatch:
		incidentID := strings.TrimPrefix(path, "/")
		h.handleUpdateIncident(w, r, incidentID)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleCreateIncident(w http.ResponseWriter, r *http.Request) {
	log.Println("[datadog] → Received create incident request")

	var req IncidentCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[datadog] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Generate incident ID
	incidentID := generateIncidentID()
	sessionID := session.FromContext(r.Context())
	now := time.Now().Unix()

	// Extract severity from fields
	var severity sql.NullString
	if req.Data.Attributes.Fields != nil {
		if sevField, ok := req.Data.Attributes.Fields["severity"]; ok {
			if sevField.IncidentFieldAttributesSingleValue != nil {
				severity = sql.NullString{
					String: sevField.IncidentFieldAttributesSingleValue.Value,
					Valid:  true,
				}
			}
		}
	}

	customerImpacted := int64(0)
	if req.Data.Attributes.CustomerImpacted {
		customerImpacted = 1
	}

	// Store incident in database
	err := h.queries.CreateDatadogIncident(context.Background(), database.CreateDatadogIncidentParams{
		ID:               incidentID,
		Title:            req.Data.Attributes.Title,
		CustomerImpacted: customerImpacted,
		Severity:         severity,
		SessionID:        sessionID,
		CreatedAt:        now,
		UpdatedAt:        now,
	})

	if err != nil {
		log.Printf("[datadog] ✗ Failed to store incident: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build response
	createdTime := time.Unix(now, 0).Format(time.RFC3339)
	modifiedTime := time.Unix(now, 0).Format(time.RFC3339)
	customerImpactedBool := req.Data.Attributes.CustomerImpacted

	attrs := IncidentResponseAttributes{
		Title:            req.Data.Attributes.Title,
		CustomerImpacted: &customerImpactedBool,
		Created:          &createdTime,
		Modified:         &modifiedTime,
	}

	if severity.Valid {
		attrs.Fields = map[string]IncidentFieldAttributes{
			"severity": {
				IncidentFieldAttributesSingleValue: &IncidentFieldAttributesSingleValue{
					Type:  "dropdown",
					Value: severity.String,
				},
			},
		}
	}

	response := IncidentResponse{
		Data: IncidentResponseData{
			ID:         incidentID,
			Type:       "incidents",
			Attributes: attrs,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[datadog] ✓ Incident created: %s", incidentID)
}

func (h *Handler) handleGetIncident(w http.ResponseWriter, r *http.Request, incidentID string) {
	log.Printf("[datadog] → Received get incident request for ID: %s", incidentID)

	sessionID := session.FromContext(r.Context())

	incident, err := h.queries.GetDatadogIncidentByID(context.Background(), database.GetDatadogIncidentByIDParams{
		ID:        incidentID,
		SessionID: sessionID,
	})

	if err != nil {
		log.Printf("[datadog] ✗ Failed to get incident: %v", err)
		http.NotFound(w, r)
		return
	}

	createdTime := time.Unix(incident.CreatedAt, 0).Format(time.RFC3339)
	modifiedTime := time.Unix(incident.UpdatedAt, 0).Format(time.RFC3339)
	customerImpactedBool := incident.CustomerImpacted != 0

	attrs := IncidentResponseAttributes{
		Title:            incident.Title,
		CustomerImpacted: &customerImpactedBool,
		Created:          &createdTime,
		Modified:         &modifiedTime,
	}

	if incident.Severity.Valid {
		attrs.Fields = map[string]IncidentFieldAttributes{
			"severity": {
				IncidentFieldAttributesSingleValue: &IncidentFieldAttributesSingleValue{
					Type:  "dropdown",
					Value: incident.Severity.String,
				},
			},
		}
	}

	response := IncidentResponse{
		Data: IncidentResponseData{
			ID:         incident.ID,
			Type:       "incidents",
			Attributes: attrs,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[datadog] ✓ Returned incident: %s", incidentID)
}

func (h *Handler) handleUpdateIncident(w http.ResponseWriter, r *http.Request, incidentID string) {
	log.Printf("[datadog] → Received update incident request for ID: %s", incidentID)

	var req IncidentUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[datadog] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	sessionID := session.FromContext(r.Context())
	now := time.Now().Unix()

	// Prepare update params
	var title sql.NullString
	if req.Data.Attributes != nil && req.Data.Attributes.Title != nil {
		title = sql.NullString{String: *req.Data.Attributes.Title, Valid: true}
	}

	var customerImpacted sql.NullInt64
	if req.Data.Attributes != nil && req.Data.Attributes.CustomerImpacted != nil {
		val := int64(0)
		if *req.Data.Attributes.CustomerImpacted {
			val = 1
		}
		customerImpacted = sql.NullInt64{Int64: val, Valid: true}
	}

	var severity sql.NullString
	if req.Data.Attributes != nil && req.Data.Attributes.Fields != nil {
		if sevField, ok := req.Data.Attributes.Fields["severity"]; ok {
			if sevField.IncidentFieldAttributesSingleValue != nil {
				severity = sql.NullString{
					String: sevField.IncidentFieldAttributesSingleValue.Value,
					Valid:  true,
				}
			}
		}
	}

	// Update incident
	err := h.queries.UpdateDatadogIncident(context.Background(), database.UpdateDatadogIncidentParams{
		Title:            title,
		CustomerImpacted: customerImpacted,
		Severity:         severity,
		UpdatedAt:        now,
		ID:               incidentID,
		SessionID:        sessionID,
	})

	if err != nil {
		log.Printf("[datadog] ✗ Failed to update incident: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get updated incident
	incident, err := h.queries.GetDatadogIncidentByID(context.Background(), database.GetDatadogIncidentByIDParams{
		ID:        incidentID,
		SessionID: sessionID,
	})

	if err != nil {
		log.Printf("[datadog] ✗ Failed to get updated incident: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	createdTime := time.Unix(incident.CreatedAt, 0).Format(time.RFC3339)
	modifiedTime := time.Unix(incident.UpdatedAt, 0).Format(time.RFC3339)
	customerImpactedBool := incident.CustomerImpacted != 0

	attrs := IncidentResponseAttributes{
		Title:            incident.Title,
		CustomerImpacted: &customerImpactedBool,
		Created:          &createdTime,
		Modified:         &modifiedTime,
	}

	if incident.Severity.Valid {
		attrs.Fields = map[string]IncidentFieldAttributes{
			"severity": {
				IncidentFieldAttributesSingleValue: &IncidentFieldAttributesSingleValue{
					Type:  "dropdown",
					Value: incident.Severity.String,
				},
			},
		}
	}

	response := IncidentResponse{
		Data: IncidentResponseData{
			ID:         incident.ID,
			Type:       "incidents",
			Attributes: attrs,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[datadog] ✓ Incident updated: %s", incidentID)
}

func (h *Handler) handleListIncidents(w http.ResponseWriter, r *http.Request) {
	log.Println("[datadog] → Received list incidents request")

	sessionID := session.FromContext(r.Context())
	pageSize := int64(100)

	if ps := r.URL.Query().Get("page[size]"); ps != "" {
		if val, err := strconv.ParseInt(ps, 10, 64); err == nil {
			pageSize = val
		}
	}

	incidents, err := h.queries.ListDatadogIncidents(context.Background(), database.ListDatadogIncidentsParams{
		SessionID: sessionID,
		Limit:     pageSize,
	})

	if err != nil {
		log.Printf("[datadog] ✗ Failed to list incidents: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := make([]IncidentResponseData, 0, len(incidents))
	for _, incident := range incidents {
		createdTime := time.Unix(incident.CreatedAt, 0).Format(time.RFC3339)
		modifiedTime := time.Unix(incident.UpdatedAt, 0).Format(time.RFC3339)
		customerImpactedBool := incident.CustomerImpacted != 0

		attrs := IncidentResponseAttributes{
			Title:            incident.Title,
			CustomerImpacted: &customerImpactedBool,
			Created:          &createdTime,
			Modified:         &modifiedTime,
		}

		if incident.Severity.Valid {
			attrs.Fields = map[string]IncidentFieldAttributes{
				"severity": {
					IncidentFieldAttributesSingleValue: &IncidentFieldAttributesSingleValue{
						Type:  "dropdown",
						Value: incident.Severity.String,
					},
				},
			}
		}

		data = append(data, IncidentResponseData{
			ID:         incident.ID,
			Type:       "incidents",
			Attributes: attrs,
		})
	}

	response := IncidentListResponse{
		Data: data,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[datadog] ✓ Listed %d incidents", len(incidents))
}

// Monitors V1 handlers

func (h *Handler) handleMonitorsV1(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	path = strings.TrimPrefix(path, "/datadog")
	path = strings.TrimPrefix(path, "/api/v1/monitor")

	switch {
	case path == "" && r.Method == http.MethodPost:
		h.handleCreateMonitor(w, r)
	case path == "" && r.Method == http.MethodGet:
		h.handleListMonitors(w, r)
	case strings.HasPrefix(path, "/") && r.Method == http.MethodGet:
		monitorIDStr := strings.TrimPrefix(path, "/")
		if monitorID, err := strconv.ParseInt(monitorIDStr, 10, 64); err == nil {
			h.handleGetMonitor(w, r, monitorID)
		} else {
			http.NotFound(w, r)
		}
	case strings.HasPrefix(path, "/") && r.Method == http.MethodPut:
		monitorIDStr := strings.TrimPrefix(path, "/")
		if monitorID, err := strconv.ParseInt(monitorIDStr, 10, 64); err == nil {
			h.handleUpdateMonitor(w, r, monitorID)
		} else {
			http.NotFound(w, r)
		}
	case strings.HasPrefix(path, "/") && r.Method == http.MethodDelete:
		monitorIDStr := strings.TrimPrefix(path, "/")
		if monitorID, err := strconv.ParseInt(monitorIDStr, 10, 64); err == nil {
			h.handleDeleteMonitor(w, r, monitorID)
		} else {
			http.NotFound(w, r)
		}
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleCreateMonitor(w http.ResponseWriter, r *http.Request) {
	log.Println("[datadog] → Received create monitor request")

	var req Monitor
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[datadog] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	sessionID := session.FromContext(r.Context())
	now := time.Now().Unix()

	var message sql.NullString
	if req.Message != nil {
		message = sql.NullString{String: *req.Message, Valid: true}
	}

	name := ""
	if req.Name != nil {
		name = *req.Name
	}

	monitor, err := h.queries.CreateDatadogMonitor(context.Background(), database.CreateDatadogMonitorParams{
		Name:      name,
		Type:      req.Type,
		Query:     req.Query,
		Message:   message,
		SessionID: sessionID,
		CreatedAt: now,
		UpdatedAt: now,
	})

	if err != nil {
		log.Printf("[datadog] ✗ Failed to store monitor: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	createdTime := time.Unix(monitor.CreatedAt, 0).Format(time.RFC3339)
	modifiedTime := time.Unix(monitor.UpdatedAt, 0).Format(time.RFC3339)

	response := Monitor{
		ID:       &monitor.ID,
		Name:     &monitor.Name,
		Type:     monitor.Type,
		Query:    monitor.Query,
		Created:  &createdTime,
		Modified: &modifiedTime,
	}

	if monitor.Message.Valid {
		response.Message = &monitor.Message.String
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[datadog] ✓ Monitor created: %d", monitor.ID)
}

func (h *Handler) handleGetMonitor(w http.ResponseWriter, r *http.Request, monitorID int64) {
	log.Printf("[datadog] → Received get monitor request for ID: %d", monitorID)

	sessionID := session.FromContext(r.Context())

	monitor, err := h.queries.GetDatadogMonitorByID(context.Background(), database.GetDatadogMonitorByIDParams{
		ID:        monitorID,
		SessionID: sessionID,
	})

	if err != nil {
		log.Printf("[datadog] ✗ Failed to get monitor: %v", err)
		http.NotFound(w, r)
		return
	}

	createdTime := time.Unix(monitor.CreatedAt, 0).Format(time.RFC3339)
	modifiedTime := time.Unix(monitor.UpdatedAt, 0).Format(time.RFC3339)

	response := Monitor{
		ID:       &monitor.ID,
		Name:     &monitor.Name,
		Type:     monitor.Type,
		Query:    monitor.Query,
		Created:  &createdTime,
		Modified: &modifiedTime,
	}

	if monitor.Message.Valid {
		response.Message = &monitor.Message.String
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[datadog] ✓ Returned monitor: %d", monitorID)
}

func (h *Handler) handleUpdateMonitor(w http.ResponseWriter, r *http.Request, monitorID int64) {
	log.Printf("[datadog] → Received update monitor request for ID: %d", monitorID)

	var req MonitorUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[datadog] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	sessionID := session.FromContext(r.Context())
	now := time.Now().Unix()

	var name, query, message sql.NullString
	if req.Name != nil {
		name = sql.NullString{String: *req.Name, Valid: true}
	}
	if req.Query != nil {
		query = sql.NullString{String: *req.Query, Valid: true}
	}
	if req.Message != nil {
		message = sql.NullString{String: *req.Message, Valid: true}
	}

	err := h.queries.UpdateDatadogMonitor(context.Background(), database.UpdateDatadogMonitorParams{
		Name:      name,
		Query:     query,
		Message:   message,
		UpdatedAt: now,
		ID:        monitorID,
		SessionID: sessionID,
	})

	if err != nil {
		log.Printf("[datadog] ✗ Failed to update monitor: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get updated monitor
	monitor, err := h.queries.GetDatadogMonitorByID(context.Background(), database.GetDatadogMonitorByIDParams{
		ID:        monitorID,
		SessionID: sessionID,
	})

	if err != nil {
		log.Printf("[datadog] ✗ Failed to get updated monitor: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	createdTime := time.Unix(monitor.CreatedAt, 0).Format(time.RFC3339)
	modifiedTime := time.Unix(monitor.UpdatedAt, 0).Format(time.RFC3339)

	response := Monitor{
		ID:       &monitor.ID,
		Name:     &monitor.Name,
		Type:     monitor.Type,
		Query:    monitor.Query,
		Created:  &createdTime,
		Modified: &modifiedTime,
	}

	if monitor.Message.Valid {
		response.Message = &monitor.Message.String
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[datadog] ✓ Monitor updated: %d", monitorID)
}

func (h *Handler) handleDeleteMonitor(w http.ResponseWriter, r *http.Request, monitorID int64) {
	log.Printf("[datadog] → Received delete monitor request for ID: %d", monitorID)

	sessionID := session.FromContext(r.Context())

	err := h.queries.DeleteDatadogMonitor(context.Background(), database.DeleteDatadogMonitorParams{
		ID:        monitorID,
		SessionID: sessionID,
	})

	if err != nil {
		log.Printf("[datadog] ✗ Failed to delete monitor: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	log.Printf("[datadog] ✓ Monitor deleted: %d", monitorID)
}

func (h *Handler) handleListMonitors(w http.ResponseWriter, r *http.Request) {
	log.Println("[datadog] → Received list monitors request")

	sessionID := session.FromContext(r.Context())

	monitors, err := h.queries.ListDatadogMonitors(context.Background(), sessionID)

	if err != nil {
		log.Printf("[datadog] ✗ Failed to list monitors: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := make([]Monitor, 0, len(monitors))
	for _, monitor := range monitors {
		createdTime := time.Unix(monitor.CreatedAt, 0).Format(time.RFC3339)
		modifiedTime := time.Unix(monitor.UpdatedAt, 0).Format(time.RFC3339)

		m := Monitor{
			ID:       &monitor.ID,
			Name:     &monitor.Name,
			Type:     monitor.Type,
			Query:    monitor.Query,
			Created:  &createdTime,
			Modified: &modifiedTime,
		}

		if monitor.Message.Valid {
			m.Message = &monitor.Message.String
		}

		response = append(response, m)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[datadog] ✓ Listed %d monitors", len(monitors))
}

// Events V1 handlers

func (h *Handler) handleEventsV1(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.handlePostEvent(w, r)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) handlePostEvent(w http.ResponseWriter, r *http.Request) {
	log.Println("[datadog] → Received post event request")

	var req EventCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[datadog] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	sessionID := session.FromContext(r.Context())
	now := time.Now().Unix()

	var tags sql.NullString
	if len(req.Tags) > 0 {
		tagsJSON, _ := json.Marshal(req.Tags)
		tags = sql.NullString{String: string(tagsJSON), Valid: true}
	}

	event, err := h.queries.CreateDatadogEvent(context.Background(), database.CreateDatadogEventParams{
		Title:     req.Title,
		Text:      req.Text,
		Tags:      tags,
		SessionID: sessionID,
		CreatedAt: now,
	})

	if err != nil {
		log.Printf("[datadog] ✗ Failed to store event: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var responseTags []string
	if event.Tags.Valid {
		_ = json.Unmarshal([]byte(event.Tags.String), &responseTags)
	}

	statusOk := "ok"
	response := EventCreateResponse{
		Status: &statusOk,
		Event: &EventCreateResponseEvent{
			ID:           &event.ID,
			Title:        &event.Title,
			Text:         &event.Text,
			Tags:         responseTags,
			DateHappened: &event.CreatedAt,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[datadog] ✓ Event posted: %d", event.ID)
}

// Metrics V2 handlers

func (h *Handler) handleMetricsV2(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.handleSubmitMetrics(w, r)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) handleSubmitMetrics(w http.ResponseWriter, r *http.Request) {
	log.Println("[datadog] → Received submit metrics request")

	var req MetricPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[datadog] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	sessionID := session.FromContext(r.Context())
	now := time.Now().Unix()

	for _, series := range req.Series {
		for _, point := range series.Points {
			var tags sql.NullString
			if len(series.Tags) > 0 {
				tagsJSON, _ := json.Marshal(series.Tags)
				tags = sql.NullString{String: string(tagsJSON), Valid: true}
			}

			timestamp := now
			if point.Timestamp != nil {
				timestamp = *point.Timestamp
			}

			value := 0.0
			if point.Value != nil {
				value = *point.Value
			}

			err := h.queries.CreateDatadogMetric(context.Background(), database.CreateDatadogMetricParams{
				MetricName: series.Metric,
				Value:      value,
				Tags:       tags,
				Timestamp:  timestamp,
				SessionID:  sessionID,
				CreatedAt:  now,
			})

			if err != nil {
				log.Printf("[datadog] ✗ Failed to store metric: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}
	}

	response := MetricSubmitResponse{
		Errors: []string{},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[datadog] ✓ Metrics submitted")
}

// Helper functions

func generateIncidentID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]))
}
