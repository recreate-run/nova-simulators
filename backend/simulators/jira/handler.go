package jira

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

	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
)

// Jira API response structures matching go-jira package
type Project struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
	Self string `json:"self,omitempty"`
}

type User struct {
	Name string `json:"name"`
}

type IssueType struct {
	Name string `json:"name"`
}

type Status struct {
	Name string `json:"name"`
}

type IssueFields struct {
	Project     Project    `json:"project"`
	Type        IssueType  `json:"issuetype"`
	Summary     string     `json:"summary"`
	Description string     `json:"description,omitempty"`
	Assignee    *User      `json:"assignee,omitempty"`
	Status      *Status    `json:"status,omitempty"`
}

type Issue struct {
	ID     string       `json:"id"`
	Key    string       `json:"key"`
	Self   string       `json:"self,omitempty"`
	Fields *IssueFields `json:"fields"`
}

type Comment struct {
	ID   string `json:"id"`
	Body string `json:"body"`
	Self string `json:"self,omitempty"`
}

type Transition struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	To   *Status `json:"to,omitempty"`
}

type TransitionsResponse struct {
	Transitions []Transition `json:"transitions"`
}

type SearchResults struct {
	Issues     []Issue `json:"issues"`
	StartAt    int     `json:"startAt"`
	MaxResults int     `json:"maxResults"`
	Total      int     `json:"total"`
}

// Handler implements the Jira simulator HTTP handler
type Handler struct {
	queries *database.Queries
}

// NewHandler creates a new Jira simulator handler
func NewHandler(queries *database.Queries) *Handler {
	return &Handler{
		queries: queries,
	}
}

// ServeHTTP implements http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[jira] → %s %s", r.Method, r.URL.Path)

	// Route Jira REST API requests
	if strings.HasPrefix(r.URL.Path, "/rest/api/2/") {
		h.handleJiraAPI(w, r)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) handleJiraAPI(w http.ResponseWriter, r *http.Request) {
	// Extract the path after /rest/api/2/
	path := strings.TrimPrefix(r.URL.Path, "/rest/api/2/")

	switch {
	case path == "project" && r.Method == http.MethodGet:
		h.handleListProjects(w, r)
	case path == "issue" && r.Method == http.MethodPost:
		h.handleCreateIssue(w, r)
	case path == "search" && r.Method == http.MethodGet:
		h.handleSearchIssues(w, r)
	case strings.HasPrefix(path, "issue/") && strings.HasSuffix(path, "/transitions") && r.Method == http.MethodGet:
		issueKey := extractIssueKey(path)
		issueKey = strings.TrimSuffix(issueKey, "/transitions")
		h.handleGetTransitions(w, r, issueKey)
	case strings.HasPrefix(path, "issue/") && strings.HasSuffix(path, "/transitions") && r.Method == http.MethodPost:
		issueKey := extractIssueKey(path)
		issueKey = strings.TrimSuffix(issueKey, "/transitions")
		h.handleExecuteTransition(w, r, issueKey)
	case strings.HasPrefix(path, "issue/") && strings.HasSuffix(path, "/comment") && r.Method == http.MethodPost:
		issueKey := extractIssueKey(path)
		issueKey = strings.TrimSuffix(issueKey, "/comment")
		h.handleAddComment(w, r, issueKey)
	case strings.HasPrefix(path, "issue/") && r.Method == http.MethodGet:
		issueKey := extractIssueKey(path)
		if issueKey != "" && !strings.Contains(issueKey, "/") {
			h.handleGetIssue(w, r, issueKey)
		} else {
			http.NotFound(w, r)
		}
	case strings.HasPrefix(path, "issue/") && r.Method == http.MethodPut:
		issueKey := extractIssueKey(path)
		if issueKey != "" && !strings.Contains(issueKey, "/") {
			h.handleUpdateIssue(w, r, issueKey)
		} else {
			http.NotFound(w, r)
		}
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleListProjects(w http.ResponseWriter, r *http.Request) {
	log.Println("[jira] → Received list projects request")

	sessionID := session.FromContext(r.Context())

	// Initialize default transitions for this session if not already present
	h.initializeDefaultTransitions(sessionID)

	dbProjects, err := h.queries.ListJiraProjects(context.Background(), sessionID)
	if err != nil {
		log.Printf("[jira] ✗ Failed to list projects: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	projects := make([]Project, 0, len(dbProjects))
	for _, p := range dbProjects {
		projects = append(projects, Project{
			ID:   p.ID,
			Key:  p.Key,
			Name: p.Name,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(projects)
	log.Printf("[jira] ✓ Listed %d projects", len(projects))
}

func (h *Handler) handleCreateIssue(w http.ResponseWriter, r *http.Request) {
	log.Println("[jira] → Received create issue request")

	sessionID := session.FromContext(r.Context())

	var req Issue
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[jira] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Fields == nil {
		http.Error(w, "Fields are required", http.StatusBadRequest)
		return
	}

	projectKey := req.Fields.Project.Key
	issueType := req.Fields.Type.Name
	summary := req.Fields.Summary
	description := req.Fields.Description

	if projectKey == "" || issueType == "" || summary == "" {
		http.Error(w, "Project key, issue type, and summary are required", http.StatusBadRequest)
		return
	}

	// Get or create project
	_, err := h.queries.GetJiraProjectByKey(context.Background(), database.GetJiraProjectByKeyParams{
		Key:       projectKey,
		SessionID: sessionID,
	})
	if err != nil {
		// Project doesn't exist, create it
		projectID := generateID()
		projectName := req.Fields.Project.Name
		if projectName == "" {
			projectName = projectKey
		}
		err = h.queries.CreateJiraProject(context.Background(), database.CreateJiraProjectParams{
			ID:        projectID,
			Key:       projectKey,
			Name:      projectName,
			SessionID: sessionID,
		})
		if err != nil {
			log.Printf("[jira] ✗ Failed to create project: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	// Generate issue ID and key
	issueID := generateID()
	issueKey := h.generateIssueKey(sessionID, projectKey)

	// Extract assignee
	var assignee sql.NullString
	if req.Fields.Assignee != nil && req.Fields.Assignee.Name != "" {
		assignee = sql.NullString{String: req.Fields.Assignee.Name, Valid: true}
	}

	// Create issue
	err = h.queries.CreateJiraIssue(context.Background(), database.CreateJiraIssueParams{
		ID:          issueID,
		Key:         issueKey,
		ProjectKey:  projectKey,
		IssueType:   issueType,
		Summary:     summary,
		Description: sql.NullString{String: description, Valid: description != ""},
		Assignee:    assignee,
		Status:      "To Do",
		SessionID:   sessionID,
	})

	if err != nil {
		log.Printf("[jira] ✗ Failed to create issue: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build response
	response := Issue{
		ID:  issueID,
		Key: issueKey,
		Fields: &IssueFields{
			Project: Project{
				Key: projectKey,
			},
			Type: IssueType{
				Name: issueType,
			},
			Summary:     summary,
			Description: description,
			Assignee:    req.Fields.Assignee,
			Status: &Status{
				Name: "To Do",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[jira] ✓ Issue created: %s", issueKey)
}

func (h *Handler) handleGetIssue(w http.ResponseWriter, r *http.Request, issueKey string) {
	log.Printf("[jira] → Received get issue request for key: %s", issueKey)

	sessionID := session.FromContext(r.Context())

	dbIssue, err := h.queries.GetJiraIssueByKey(context.Background(), database.GetJiraIssueByKeyParams{
		Key:       issueKey,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[jira] ✗ Failed to get issue: %v", err)
		http.NotFound(w, r)
		return
	}

	// Build response
	issue := Issue{
		ID:  dbIssue.ID,
		Key: dbIssue.Key,
		Fields: &IssueFields{
			Project: Project{
				Key: dbIssue.ProjectKey,
			},
			Type: IssueType{
				Name: dbIssue.IssueType,
			},
			Summary:     dbIssue.Summary,
			Description: dbIssue.Description.String,
			Status: &Status{
				Name: dbIssue.Status,
			},
		},
	}

	if dbIssue.Assignee.Valid {
		issue.Fields.Assignee = &User{
			Name: dbIssue.Assignee.String,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(issue)
	log.Printf("[jira] ✓ Returned issue: %s", issueKey)
}

func (h *Handler) handleUpdateIssue(w http.ResponseWriter, r *http.Request, issueKey string) {
	log.Printf("[jira] → Received update issue request for key: %s", issueKey)

	sessionID := session.FromContext(r.Context())

	var req struct {
		Fields map[string]interface{} `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[jira] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Get current issue
	dbIssue, err := h.queries.GetJiraIssueByKey(context.Background(), database.GetJiraIssueByKeyParams{
		Key:       issueKey,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[jira] ✗ Failed to get issue: %v", err)
		http.NotFound(w, r)
		return
	}

	// Extract fields to update
	summary := dbIssue.Summary
	description := dbIssue.Description.String
	assignee := dbIssue.Assignee

	if v, ok := req.Fields["summary"].(string); ok {
		summary = v
	}
	if v, ok := req.Fields["description"].(string); ok {
		description = v
	}
	if v, ok := req.Fields["assignee"]; ok {
		if userMap, ok := v.(map[string]interface{}); ok {
			if name, ok := userMap["name"].(string); ok {
				assignee = sql.NullString{String: name, Valid: true}
			}
		}
	}

	// Update issue
	err = h.queries.UpdateJiraIssue(context.Background(), database.UpdateJiraIssueParams{
		Summary:     summary,
		Description: sql.NullString{String: description, Valid: description != ""},
		Assignee:    assignee,
		Key:         issueKey,
		SessionID:   sessionID,
	})

	if err != nil {
		log.Printf("[jira] ✗ Failed to update issue: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	log.Printf("[jira] ✓ Issue updated: %s", issueKey)
}

func (h *Handler) handleGetTransitions(w http.ResponseWriter, r *http.Request, issueKey string) {
	log.Printf("[jira] → Received get transitions request for issue: %s", issueKey)

	sessionID := session.FromContext(r.Context())

	// Initialize default transitions if needed
	h.initializeDefaultTransitions(sessionID)

	// Verify issue exists
	_, err := h.queries.GetJiraIssueByKey(context.Background(), database.GetJiraIssueByKeyParams{
		Key:       issueKey,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[jira] ✗ Failed to get issue: %v", err)
		http.NotFound(w, r)
		return
	}

	// Get all transitions for this session
	dbTransitions, err := h.queries.ListJiraTransitions(context.Background(), sessionID)
	if err != nil {
		log.Printf("[jira] ✗ Failed to list transitions: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	transitions := make([]Transition, 0, len(dbTransitions))
	for _, t := range dbTransitions {
		transitions = append(transitions, Transition{
			ID:   t.ID,
			Name: t.Name,
			To: &Status{
				Name: t.ToStatus,
			},
		})
	}

	response := TransitionsResponse{
		Transitions: transitions,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[jira] ✓ Returned %d transitions", len(transitions))
}

func (h *Handler) handleExecuteTransition(w http.ResponseWriter, r *http.Request, issueKey string) {
	log.Printf("[jira] → Received execute transition request for issue: %s", issueKey)

	sessionID := session.FromContext(r.Context())

	var req struct {
		Transition struct {
			ID string `json:"id"`
		} `json:"transition"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[jira] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Get transition to find target status
	dbTransitions, err := h.queries.ListJiraTransitions(context.Background(), sessionID)
	if err != nil {
		log.Printf("[jira] ✗ Failed to list transitions: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var targetStatus string
	for _, t := range dbTransitions {
		if t.ID == req.Transition.ID {
			targetStatus = t.ToStatus
			break
		}
	}

	if targetStatus == "" {
		http.Error(w, "Transition not found", http.StatusNotFound)
		return
	}

	// Update issue status
	err = h.queries.UpdateJiraIssueStatus(context.Background(), database.UpdateJiraIssueStatusParams{
		Status:    targetStatus,
		Key:       issueKey,
		SessionID: sessionID,
	})

	if err != nil {
		log.Printf("[jira] ✗ Failed to update issue status: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	log.Printf("[jira] ✓ Issue transitioned: %s -> %s", issueKey, targetStatus)
}

func (h *Handler) handleAddComment(w http.ResponseWriter, r *http.Request, issueKey string) {
	log.Printf("[jira] → Received add comment request for issue: %s", issueKey)

	sessionID := session.FromContext(r.Context())

	var req Comment
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[jira] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Verify issue exists
	_, err := h.queries.GetJiraIssueByKey(context.Background(), database.GetJiraIssueByKeyParams{
		Key:       issueKey,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[jira] ✗ Failed to get issue: %v", err)
		http.NotFound(w, r)
		return
	}

	// Create comment
	commentID := generateID()
	err = h.queries.CreateJiraComment(context.Background(), database.CreateJiraCommentParams{
		ID:        commentID,
		IssueKey:  issueKey,
		Body:      req.Body,
		SessionID: sessionID,
	})

	if err != nil {
		log.Printf("[jira] ✗ Failed to create comment: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := Comment{
		ID:   commentID,
		Body: req.Body,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[jira] ✓ Comment added: %s", commentID)
}

func (h *Handler) handleSearchIssues(w http.ResponseWriter, r *http.Request) {
	log.Println("[jira] → Received search issues request")

	sessionID := session.FromContext(r.Context())

	// Parse query parameters
	query := r.URL.Query()
	jql := query.Get("jql")
	maxResultsStr := query.Get("maxResults")
	startAtStr := query.Get("startAt")

	maxResults := 50
	if maxResultsStr != "" {
		if mr, err := strconv.Atoi(maxResultsStr); err == nil {
			maxResults = mr
		}
	}

	startAt := 0
	if startAtStr != "" {
		if sa, err := strconv.Atoi(startAtStr); err == nil {
			startAt = sa
		}
	}

	// Parse JQL (simplified - support basic filters)
	projectKey := ""
	issueType := ""
	summary := ""
	assignee := ""
	status := ""

	if jql != "" {
		// Simple JQL parsing for common patterns
		parts := strings.Split(jql, " AND ")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			switch {
			case strings.Contains(part, "project ="):
				projectKey = extractValue(part)
			case strings.Contains(part, "type ="), strings.Contains(part, "issuetype ="):
				issueType = extractValue(part)
			case strings.Contains(part, "summary ~"):
				summary = extractValue(part)
			case strings.Contains(part, "assignee ="):
				assignee = extractValue(part)
			case strings.Contains(part, "status ="):
				status = extractValue(part)
			}
		}
	}

	// Search issues
	dbIssues, err := h.queries.SearchJiraIssues(context.Background(), database.SearchJiraIssuesParams{
		SessionID:  sessionID,
		Column2:    projectKey,
		ProjectKey: projectKey,
		Column4:    issueType,
		IssueType:  issueType,
		Column6:    summary,
		Column7:    sql.NullString{String: summary, Valid: summary != ""},
		Column8:    assignee,
		Assignee:   sql.NullString{String: assignee, Valid: assignee != ""},
		Column10:   status,
		Status:     status,
		Limit:      int64(maxResults + startAt),
	})

	if err != nil {
		log.Printf("[jira] ✗ Failed to search issues: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Apply pagination
	total := len(dbIssues)
	if startAt >= total {
		dbIssues = []database.SearchJiraIssuesRow{}
	} else {
		end := startAt + maxResults
		if end > total {
			end = total
		}
		dbIssues = dbIssues[startAt:end]
	}

	// Build response
	issues := make([]Issue, 0, len(dbIssues))
	for i := range dbIssues {
		issue := Issue{
			ID:  dbIssues[i].ID,
			Key: dbIssues[i].Key,
			Fields: &IssueFields{
				Project: Project{
					Key: dbIssues[i].ProjectKey,
				},
				Type: IssueType{
					Name: dbIssues[i].IssueType,
				},
				Summary:     dbIssues[i].Summary,
				Description: dbIssues[i].Description.String,
				Status: &Status{
					Name: dbIssues[i].Status,
				},
			},
		}
		if dbIssues[i].Assignee.Valid {
			issue.Fields.Assignee = &User{
				Name: dbIssues[i].Assignee.String,
			}
		}
		issues = append(issues, issue)
	}

	response := SearchResults{
		Issues:     issues,
		StartAt:    startAt,
		MaxResults: maxResults,
		Total:      total,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[jira] ✓ Found %d issues", len(issues))
}

// Helper functions

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *Handler) generateIssueKey(sessionID, projectKey string) string {
	// Count existing issues in project to get next number
	dbIssues, err := h.queries.SearchJiraIssues(context.Background(), database.SearchJiraIssuesParams{
		SessionID:  sessionID,
		Column2:    projectKey,
		ProjectKey: projectKey,
		Column4:    "",
		IssueType:  "",
		Column6:    "",
		Column7:    sql.NullString{String: "", Valid: false},
		Column8:    "",
		Assignee:   sql.NullString{String: "", Valid: false},
		Column10:   "",
		Status:     "",
		Limit:      10000,
	})

	issueNum := 1
	if err == nil {
		issueNum = len(dbIssues) + 1
	}

	return fmt.Sprintf("%s-%d", projectKey, issueNum)
}

func extractIssueKey(path string) string {
	path = strings.TrimPrefix(path, "issue/")
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func extractValue(jqlPart string) string {
	// Extract value from "field = value" or "field ~ value"
	for _, sep := range []string{" = ", " ~ "} {
		if idx := strings.Index(jqlPart, sep); idx != -1 {
			value := strings.TrimSpace(jqlPart[idx+len(sep):])
			// Remove quotes if present
			value = strings.Trim(value, `"'`)
			return value
		}
	}
	return ""
}

func (h *Handler) initializeDefaultTransitions(sessionID string) {
	// Check if transitions already exist
	existing, err := h.queries.ListJiraTransitions(context.Background(), sessionID)
	if err == nil && len(existing) > 0 {
		return
	}

	// Create default transitions
	defaultTransitions := []struct {
		name     string
		toStatus string
	}{
		{"Start Progress", "In Progress"},
		{"Stop Progress", "To Do"},
		{"Done", "Done"},
		{"Reopen", "To Do"},
		{"In Review", "In Review"},
	}

	for _, t := range defaultTransitions {
		_ = h.queries.CreateJiraTransition(context.Background(), database.CreateJiraTransitionParams{
			ID:        generateID(),
			Name:      t.name,
			ToStatus:  t.toStatus,
			SessionID: sessionID,
		})
	}
}
