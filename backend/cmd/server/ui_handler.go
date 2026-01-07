package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/recreate-run/nova-simulators/internal/database"
)

// Simulator represents metadata about a registered simulator
type Simulator struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

// UIHandler serves the data API endpoints
type UIHandler struct {
	queries    *database.Queries
	simulators []Simulator
}

// NewUIHandler creates a new UI handler
func NewUIHandler(queries *database.Queries, simulators []Simulator) *UIHandler {
	return &UIHandler{
		queries:    queries,
		simulators: simulators,
	}
}

// ServeHTTP implements http.Handler interface
func (h *UIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/sessions":
		h.handleGetSessions(w, r)
	case r.URL.Path == "/api/simulators":
		h.handleGetSimulators(w)
	case strings.HasPrefix(r.URL.Path, "/api/simulators/sessions/") && strings.HasSuffix(r.URL.Path, "/active"):
		h.handleGetActiveSimulators(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/simulators/"):
		h.handleGetSimulatorData(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *UIHandler) handleGetSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.queries.ListSessions(context.Background())
	if err != nil {
		http.Error(w, "Failed to list sessions", http.StatusInternalServerError)
		return
	}

	// Check if filtering is requested
	filter := r.URL.Query().Get("filter")
	if filter == "active" {
		// Filter sessions to only include those with data
		activeSessions := []database.Session{}
		for _, session := range sessions {
			if h.sessionHasData(session.ID) {
				activeSessions = append(activeSessions, session)
			}
		}
		sessions = activeSessions
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sessions)
}

func (h *UIHandler) handleGetSimulators(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"simulators": h.simulators,
	})
}

// sessionHasData checks if a session has any data in any simulator
func (h *UIHandler) sessionHasData(sessionID string) bool {
	ctx := context.Background()

	// Check Slack
	if channels, _ := h.queries.ListChannels(ctx, sessionID); len(channels) > 0 {
		return true
	}
	if messages, _ := h.queries.ListMessagesBySession(ctx, sessionID); len(messages) > 0 {
		return true
	}

	// Check Gmail
	if messages, _ := h.queries.ListGmailMessagesBySession(ctx, sessionID); len(messages) > 0 {
		return true
	}

	// Check Google Docs
	if docs, _ := h.queries.ListGdocsBySession(ctx, sessionID); len(docs) > 0 {
		return true
	}

	// Check Google Sheets
	if sheets, _ := h.queries.ListGsheetsBySession(ctx, sessionID); len(sheets) > 0 {
		return true
	}

	// Check Datadog
	if metrics, _ := h.queries.ListDatadogMetricsBySession(ctx, database.ListDatadogMetricsBySessionParams{SessionID: sessionID, Limit: 1}); len(metrics) > 0 {
		return true
	}

	// Check Resend
	if emails, _ := h.queries.ListResendEmailsBySession(ctx, sessionID); len(emails) > 0 {
		return true
	}

	// Check Linear
	if issues, _ := h.queries.ListLinearIssuesBySession(ctx, sessionID); len(issues) > 0 {
		return true
	}

	// Check GitHub
	if issues, _ := h.queries.ListGithubIssuesBySession(ctx, sessionID); len(issues) > 0 {
		return true
	}

	// Check Outlook
	if messages, _ := h.queries.ListOutlookMessagesBySession(ctx, sessionID); len(messages) > 0 {
		return true
	}

	// Check PagerDuty
	if incidents, _ := h.queries.ListPagerdutyIncidentsBySession(ctx, sessionID); len(incidents) > 0 {
		return true
	}

	// Check HubSpot
	if contacts, _ := h.queries.ListHubspotContactsBySession(ctx, sessionID); len(contacts) > 0 {
		return true
	}

	// Check Jira
	if issues, _ := h.queries.ListJiraIssuesBySession(ctx, sessionID); len(issues) > 0 {
		return true
	}

	// Check WhatsApp
	if messages, _ := h.queries.ListWhatsappMessagesBySession(ctx, sessionID); len(messages) > 0 {
		return true
	}

	// Check Postgres
	if queries, _ := h.queries.ListPostgresQueriesBySession(ctx, sessionID); len(queries) > 0 {
		return true
	}

	return false
}

// SimulatorInfo represents a simulator with data count
type SimulatorInfo struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func (h *UIHandler) handleGetActiveSimulators(w http.ResponseWriter, r *http.Request) {
	// Parse URL: /api/simulators/sessions/{sessionID}/active
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/simulators/sessions/"), "/")
	if len(parts) != 2 || parts[1] != "active" {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	sessionID := parts[0]
	ctx := context.Background()

	var activeSimulators []SimulatorInfo

	// Check Slack
	slackChannels, _ := h.queries.ListChannels(ctx, sessionID)
	slackMessages, _ := h.queries.ListMessagesBySession(ctx, sessionID)
	slackCount := len(slackChannels) + len(slackMessages)
	if slackCount > 0 {
		activeSimulators = append(activeSimulators, SimulatorInfo{Name: "slack", Count: slackCount})
	}

	// Check Gmail
	gmailMessages, _ := h.queries.ListGmailMessagesBySession(ctx, sessionID)
	if len(gmailMessages) > 0 {
		activeSimulators = append(activeSimulators, SimulatorInfo{Name: "gmail", Count: len(gmailMessages)})
	}

	// Check Google Docs
	gdocs, _ := h.queries.ListGdocsBySession(ctx, sessionID)
	if len(gdocs) > 0 {
		activeSimulators = append(activeSimulators, SimulatorInfo{Name: "gdocs", Count: len(gdocs)})
	}

	// Check Google Sheets
	gsheets, _ := h.queries.ListGsheetsBySession(ctx, sessionID)
	if len(gsheets) > 0 {
		activeSimulators = append(activeSimulators, SimulatorInfo{Name: "gsheets", Count: len(gsheets)})
	}

	// Check Datadog
	datadogMetrics, _ := h.queries.ListDatadogMetricsBySession(ctx, database.ListDatadogMetricsBySessionParams{
		SessionID: sessionID,
		Limit:     1000,
	})
	if len(datadogMetrics) > 0 {
		activeSimulators = append(activeSimulators, SimulatorInfo{Name: "datadog", Count: len(datadogMetrics)})
	}

	// Check Resend
	resendEmails, _ := h.queries.ListResendEmailsBySession(ctx, sessionID)
	if len(resendEmails) > 0 {
		activeSimulators = append(activeSimulators, SimulatorInfo{Name: "resend", Count: len(resendEmails)})
	}

	// Check Linear
	linearIssues, _ := h.queries.ListLinearIssuesBySession(ctx, sessionID)
	if len(linearIssues) > 0 {
		activeSimulators = append(activeSimulators, SimulatorInfo{Name: "linear", Count: len(linearIssues)})
	}

	// Check GitHub
	githubIssues, _ := h.queries.ListGithubIssuesBySession(ctx, sessionID)
	if len(githubIssues) > 0 {
		activeSimulators = append(activeSimulators, SimulatorInfo{Name: "github", Count: len(githubIssues)})
	}

	// Check Outlook
	outlookMessages, _ := h.queries.ListOutlookMessagesBySession(ctx, sessionID)
	if len(outlookMessages) > 0 {
		activeSimulators = append(activeSimulators, SimulatorInfo{Name: "outlook", Count: len(outlookMessages)})
	}

	// Check PagerDuty
	pagerdutyIncidents, _ := h.queries.ListPagerdutyIncidentsBySession(ctx, sessionID)
	if len(pagerdutyIncidents) > 0 {
		activeSimulators = append(activeSimulators, SimulatorInfo{Name: "pagerduty", Count: len(pagerdutyIncidents)})
	}

	// Check HubSpot
	hubspotContacts, _ := h.queries.ListHubspotContactsBySession(ctx, sessionID)
	if len(hubspotContacts) > 0 {
		activeSimulators = append(activeSimulators, SimulatorInfo{Name: "hubspot", Count: len(hubspotContacts)})
	}

	// Check Jira
	jiraIssues, _ := h.queries.ListJiraIssuesBySession(ctx, sessionID)
	if len(jiraIssues) > 0 {
		activeSimulators = append(activeSimulators, SimulatorInfo{Name: "jira", Count: len(jiraIssues)})
	}

	// Check WhatsApp
	whatsappMessages, _ := h.queries.ListWhatsappMessagesBySession(ctx, sessionID)
	if len(whatsappMessages) > 0 {
		activeSimulators = append(activeSimulators, SimulatorInfo{Name: "whatsapp", Count: len(whatsappMessages)})
	}

	// Check Postgres
	postgresQueries, _ := h.queries.ListPostgresQueriesBySession(ctx, sessionID)
	if len(postgresQueries) > 0 {
		activeSimulators = append(activeSimulators, SimulatorInfo{Name: "postgres", Count: len(postgresQueries)})
	}

	response := map[string]interface{}{
		"simulators": activeSimulators,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (h *UIHandler) handleGetSimulatorData(w http.ResponseWriter, r *http.Request) {
	// Parse URL: /api/simulators/{simulator}/sessions/{sessionID}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/simulators/"), "/")
	if len(parts) != 3 || parts[1] != "sessions" {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	simulator := parts[0]
	sessionID := parts[2]

	var data map[string]interface{}
	var err error

	switch simulator {
	case "slack":
		data, err = h.getSlackData(sessionID)
	case "gmail":
		data, err = h.getGmailData(sessionID)
	case "gdocs":
		data, err = h.getGDocsData(sessionID)
	case "gsheets":
		data, err = h.getGSheetsData(sessionID)
	case "datadog":
		data, err = h.getDatadogData(sessionID)
	case "resend":
		data, err = h.getResendData(sessionID)
	case "linear":
		data, err = h.getLinearData(sessionID)
	case "github":
		data, err = h.getGitHubData(sessionID)
	case "outlook":
		data, err = h.getOutlookData(sessionID)
	case "pagerduty":
		data, err = h.getPagerDutyData(sessionID)
	case "hubspot":
		data, err = h.getHubSpotData(sessionID)
	case "jira":
		data, err = h.getJiraData(sessionID)
	case "whatsapp":
		data, err = h.getWhatsAppData(sessionID)
	case "postgres":
		data, err = h.getPostgresData(sessionID)
	default:
		http.Error(w, "Unknown simulator", http.StatusNotFound)
		return
	}

	if err != nil {
		http.Error(w, "Failed to get data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func (h *UIHandler) getSlackData(sessionID string) (map[string]interface{}, error) {
	ctx := context.Background()

	channels, err := h.queries.ListChannels(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	messages, err := h.queries.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	users, err := h.queries.ListUsersBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	files, err := h.queries.ListFilesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"channels": channels,
		"messages": messages,
		"users":    users,
		"files":    files,
	}, nil
}

func (h *UIHandler) getGmailData(sessionID string) (map[string]interface{}, error) {
	ctx := context.Background()

	messages, err := h.queries.ListGmailMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"messages": messages,
	}, nil
}

func (h *UIHandler) getGDocsData(sessionID string) (map[string]interface{}, error) {
	ctx := context.Background()

	documents, err := h.queries.ListGdocsBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"documents": documents,
	}, nil
}

func (h *UIHandler) getGSheetsData(sessionID string) (map[string]interface{}, error) {
	ctx := context.Background()

	spreadsheets, err := h.queries.ListGsheetsBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"spreadsheets": spreadsheets,
	}, nil
}

func (h *UIHandler) getDatadogData(sessionID string) (map[string]interface{}, error) {
	ctx := context.Background()

	metrics, err := h.queries.ListDatadogMetricsBySession(ctx, database.ListDatadogMetricsBySessionParams{
		SessionID: sessionID,
		Limit:     1000,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"metrics": metrics,
	}, nil
}

func (h *UIHandler) getResendData(sessionID string) (map[string]interface{}, error) {
	ctx := context.Background()

	emails, err := h.queries.ListResendEmailsBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"emails": emails,
	}, nil
}

func (h *UIHandler) getLinearData(sessionID string) (map[string]interface{}, error) {
	ctx := context.Background()

	issues, err := h.queries.ListLinearIssuesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"issues": issues,
	}, nil
}

func (h *UIHandler) getGitHubData(sessionID string) (map[string]interface{}, error) {
	ctx := context.Background()

	issues, err := h.queries.ListGithubIssuesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"issues": issues,
	}, nil
}

func (h *UIHandler) getOutlookData(sessionID string) (map[string]interface{}, error) {
	ctx := context.Background()

	messages, err := h.queries.ListOutlookMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"messages": messages,
	}, nil
}

func (h *UIHandler) getPagerDutyData(sessionID string) (map[string]interface{}, error) {
	ctx := context.Background()

	incidents, err := h.queries.ListPagerdutyIncidentsBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"incidents": incidents,
	}, nil
}

func (h *UIHandler) getHubSpotData(sessionID string) (map[string]interface{}, error) {
	ctx := context.Background()

	contacts, err := h.queries.ListHubspotContactsBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"contacts": contacts,
	}, nil
}

func (h *UIHandler) getJiraData(sessionID string) (map[string]interface{}, error) {
	ctx := context.Background()

	issues, err := h.queries.ListJiraIssuesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"issues": issues,
	}, nil
}

func (h *UIHandler) getWhatsAppData(sessionID string) (map[string]interface{}, error) {
	ctx := context.Background()

	messages, err := h.queries.ListWhatsappMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"messages": messages,
	}, nil
}

func (h *UIHandler) getPostgresData(sessionID string) (map[string]interface{}, error) {
	ctx := context.Background()

	queries, err := h.queries.ListPostgresQueriesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"queries": queries,
	}, nil
}
