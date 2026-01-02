package linear

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

// GraphQL request/response structures
type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type GraphQLResponse struct {
	Data   interface{}            `json:"data,omitempty"`
	Errors []GraphQLError         `json:"errors,omitempty"`
}

type GraphQLError struct {
	Message string `json:"message"`
}

// Linear API structures matching the workflow client
type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
}

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type State struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type Issue struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	ArchivedAt  *time.Time `json:"archivedAt"`
	URL         string     `json:"url"`
	Assignee    *User      `json:"assignee"`
	State       *State     `json:"state"`
}

// Handler implements the Linear simulator HTTP handler
type Handler struct {
	queries *database.Queries
}

// NewHandler creates a new Linear simulator handler
func NewHandler(queries *database.Queries) *Handler {
	return &Handler{
		queries: queries,
	}
}

// ServeHTTP implements http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[linear] → %s %s", r.Method, r.URL.Path)

	// Route Linear GraphQL API requests
	if r.URL.Path == "/graphql" && r.Method == http.MethodPost {
		h.handleGraphQL(w, r)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) handleGraphQL(w http.ResponseWriter, r *http.Request) {
	var req GraphQLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[linear] ✗ Failed to decode request: %v", err)
		h.sendError(w, "Invalid request body")
		return
	}

	log.Printf("[linear]   GraphQL query: %s", strings.TrimSpace(req.Query))
	log.Printf("[linear]   Variables: %+v", req.Variables)

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Determine query/mutation type
	if strings.Contains(req.Query, "mutation") {
		h.handleMutation(w, r, req, sessionID)
	} else {
		h.handleQuery(w, r, req, sessionID)
	}
}

func (h *Handler) handleQuery(w http.ResponseWriter, _ *http.Request, req GraphQLRequest, sessionID string) {
	query := strings.TrimSpace(req.Query)

	switch {
	case strings.Contains(query, "query Issue("):
		h.handleGetIssue(w, req, sessionID)
	case strings.Contains(query, "query Team("):
		h.handleGetTeam(w, req, sessionID)
	case strings.Contains(query, "query TeamIssues("):
		h.handleListIssuesByTeam(w, req, sessionID)
	case strings.Contains(query, "query Teams"):
		h.handleListTeams(w, req, sessionID)
	case strings.Contains(query, "query Users"):
		h.handleListUsers(w, req, sessionID)
	case strings.Contains(query, "query Me"):
		h.handleGetViewer(w, req, sessionID)
	default:
		log.Printf("[linear] ✗ Unknown query type")
		h.sendError(w, "Unknown query type")
	}
}

func (h *Handler) handleMutation(w http.ResponseWriter, _ *http.Request, req GraphQLRequest, sessionID string) {
	query := strings.TrimSpace(req.Query)

	switch {
	case strings.Contains(query, "mutation IssueCreate"):
		h.handleCreateIssue(w, req, sessionID)
	case strings.Contains(query, "mutation IssueUpdate"):
		h.handleUpdateIssue(w, req, sessionID)
	default:
		log.Printf("[linear] ✗ Unknown mutation type")
		h.sendError(w, "Unknown mutation type")
	}
}

func (h *Handler) handleGetIssue(w http.ResponseWriter, req GraphQLRequest, sessionID string) {
	issueID, ok := req.Variables["id"].(string)
	if !ok {
		h.sendError(w, "Invalid or missing issue ID")
		return
	}
	log.Printf("[linear] → Get issue: %s", issueID)

	dbIssue, err := h.queries.GetLinearIssueByID(context.Background(), database.GetLinearIssueByIDParams{
		ID:        issueID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[linear] ✗ Issue not found: %v", err)
		h.sendError(w, "Issue not found")
		return
	}

	issue := h.convertIssueFromGetByIDRow(dbIssue, sessionID)

	response := map[string]interface{}{
		"issue": issue,
	}
	h.sendSuccess(w, response)
	log.Printf("[linear] ✓ Returned issue: %s", issueID)
}

func (h *Handler) handleGetTeam(w http.ResponseWriter, req GraphQLRequest, sessionID string) {
	teamID, ok := req.Variables["id"].(string)
	if !ok {
		h.sendError(w, "Invalid or missing team ID")
		return
	}
	log.Printf("[linear] → Get team: %s", teamID)

	dbTeam, err := h.queries.GetLinearTeamByID(context.Background(), database.GetLinearTeamByIDParams{
		ID:        teamID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[linear] ✗ Team not found: %v", err)
		h.sendError(w, "Team not found")
		return
	}

	team := Team{
		ID:   dbTeam.ID,
		Name: dbTeam.Name,
		Key:  dbTeam.Key,
	}

	response := map[string]interface{}{
		"team": team,
	}
	h.sendSuccess(w, response)
	log.Printf("[linear] ✓ Returned team: %s", teamID)
}

func (h *Handler) handleListIssuesByTeam(w http.ResponseWriter, req GraphQLRequest, sessionID string) {
	teamID, ok := req.Variables["teamId"].(string)
	if !ok {
		h.sendError(w, "Invalid or missing team ID")
		return
	}
	log.Printf("[linear] → List issues for team: %s", teamID)

	dbIssues, err := h.queries.ListLinearIssuesByTeam(context.Background(), database.ListLinearIssuesByTeamParams{
		TeamID:    teamID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[linear] ✗ Failed to list issues: %v", err)
		h.sendError(w, "Failed to list issues")
		return
	}

	issues := make([]Issue, 0, len(dbIssues))
	for i := range dbIssues {
		issues = append(issues, h.convertIssueFromListByTeamRow(dbIssues[i], sessionID))
	}

	response := map[string]interface{}{
		"team": map[string]interface{}{
			"issues": map[string]interface{}{
				"nodes": issues,
			},
		},
	}
	h.sendSuccess(w, response)
	log.Printf("[linear] ✓ Listed %d issues", len(issues))
}

func (h *Handler) handleListTeams(w http.ResponseWriter, _ GraphQLRequest, sessionID string) {
	log.Printf("[linear] → List teams")

	dbTeams, err := h.queries.ListLinearTeams(context.Background(), sessionID)
	if err != nil {
		log.Printf("[linear] ✗ Failed to list teams: %v", err)
		h.sendError(w, "Failed to list teams")
		return
	}

	teams := make([]Team, 0, len(dbTeams))
	for _, dbTeam := range dbTeams {
		teams = append(teams, Team{
			ID:   dbTeam.ID,
			Name: dbTeam.Name,
			Key:  dbTeam.Key,
		})
	}

	response := map[string]interface{}{
		"teams": map[string]interface{}{
			"nodes": teams,
		},
	}
	h.sendSuccess(w, response)
	log.Printf("[linear] ✓ Listed %d teams", len(teams))
}

func (h *Handler) handleListUsers(w http.ResponseWriter, _ GraphQLRequest, sessionID string) {
	log.Printf("[linear] → List users")

	dbUsers, err := h.queries.ListLinearUsers(context.Background(), sessionID)
	if err != nil {
		log.Printf("[linear] ✗ Failed to list users: %v", err)
		h.sendError(w, "Failed to list users")
		return
	}

	users := make([]User, 0, len(dbUsers))
	for _, dbUser := range dbUsers {
		users = append(users, User{
			ID:    dbUser.ID,
			Name:  dbUser.Name,
			Email: dbUser.Email,
		})
	}

	response := map[string]interface{}{
		"users": map[string]interface{}{
			"nodes": users,
		},
	}
	h.sendSuccess(w, response)
	log.Printf("[linear] ✓ Listed %d users", len(users))
}

func (h *Handler) handleGetViewer(w http.ResponseWriter, _ GraphQLRequest, sessionID string) {
	log.Printf("[linear] → Get viewer")

	// Return the first user as the viewer (simulating authenticated user)
	dbUsers, err := h.queries.ListLinearUsers(context.Background(), sessionID)
	if err != nil || len(dbUsers) == 0 {
		log.Printf("[linear] ✗ No users found")
		h.sendError(w, "No users found")
		return
	}

	user := User{
		ID:    dbUsers[0].ID,
		Name:  dbUsers[0].Name,
		Email: dbUsers[0].Email,
	}

	response := map[string]interface{}{
		"viewer": user,
	}
	h.sendSuccess(w, response)
	log.Printf("[linear] ✓ Returned viewer: %s", user.ID)
}

func (h *Handler) handleCreateIssue(w http.ResponseWriter, req GraphQLRequest, sessionID string) {
	log.Printf("[linear] → Create issue")

	teamID, ok := req.Variables["teamId"].(string)
	if !ok {
		h.sendError(w, "Invalid or missing team ID")
		return
	}
	title, ok := req.Variables["title"].(string)
	if !ok {
		h.sendError(w, "Invalid or missing title")
		return
	}

	var description sql.NullString
	if desc, ok := req.Variables["description"].(string); ok && desc != "" {
		description = sql.NullString{String: desc, Valid: true}
	}

	var assigneeID sql.NullString
	if aid, ok := req.Variables["assigneeId"].(string); ok && aid != "" {
		assigneeID = sql.NullString{String: aid, Valid: true}
	}

	var stateID sql.NullString
	if sid, ok := req.Variables["stateId"].(string); ok && sid != "" {
		stateID = sql.NullString{String: sid, Valid: true}
	}

	// Generate issue ID
	issueID := generateID()
	now := time.Now().UnixMilli()

	// Get team to build URL
	dbTeam, err := h.queries.GetLinearTeamByID(context.Background(), database.GetLinearTeamByIDParams{
		ID:        teamID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[linear] ✗ Team not found: %v", err)
		h.sendError(w, "Team not found")
		return
	}

	// Generate issue URL (format: linear.app/{team-key}/issue/{team-key}-{number})
	url := fmt.Sprintf("https://linear.app/%s/issue/%s-%s", dbTeam.Key, dbTeam.Key, issueID[:8])

	// Create issue
	dbIssue, err := h.queries.CreateLinearIssue(context.Background(), database.CreateLinearIssueParams{
		ID:          issueID,
		TeamID:      teamID,
		Title:       title,
		Description: description,
		AssigneeID:  assigneeID,
		StateID:     stateID,
		Url:         url,
		SessionID:   sessionID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		log.Printf("[linear] ✗ Failed to create issue: %v", err)
		h.sendError(w, "Failed to create issue")
		return
	}

	issue := h.convertIssueFromCreateRow(dbIssue, sessionID)

	response := map[string]interface{}{
		"issueCreate": map[string]interface{}{
			"success": true,
			"issue":   issue,
		},
	}
	h.sendSuccess(w, response)
	log.Printf("[linear] ✓ Created issue: %s", issueID)
}

func (h *Handler) handleUpdateIssue(w http.ResponseWriter, req GraphQLRequest, sessionID string) {
	issueID, ok := req.Variables["id"].(string)
	if !ok {
		h.sendError(w, "Invalid or missing issue ID")
		return
	}
	log.Printf("[linear] → Update issue: %s", issueID)

	// Verify issue exists
	_, err := h.queries.GetLinearIssueByID(context.Background(), database.GetLinearIssueByIDParams{
		ID:        issueID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[linear] ✗ Issue not found: %v", err)
		h.sendError(w, "Issue not found")
		return
	}

	// Prepare update parameters
	// Get existing issue to use as defaults
	existingIssue, err := h.queries.GetLinearIssueByID(context.Background(), database.GetLinearIssueByIDParams{
		ID:        issueID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[linear] ✗ Failed to get existing issue: %v", err)
		h.sendError(w, "Failed to get issue")
		return
	}

	// Use provided values or fallback to existing
	title := existingIssue.Title
	if t, ok := req.Variables["title"].(string); ok && t != "" {
		title = t
	}

	var description sql.NullString
	if d, ok := req.Variables["description"].(string); ok {
		description = sql.NullString{String: d, Valid: true}
	} else {
		description = existingIssue.Description
	}

	var assigneeID sql.NullString
	if a, ok := req.Variables["assigneeId"].(string); ok {
		assigneeID = sql.NullString{String: a, Valid: true}
	} else {
		assigneeID = existingIssue.AssigneeID
	}

	var stateID sql.NullString
	if s, ok := req.Variables["stateId"].(string); ok {
		stateID = sql.NullString{String: s, Valid: true}
	} else {
		stateID = existingIssue.StateID
	}

	now := time.Now().UnixMilli()

	// Update issue
	err = h.queries.UpdateLinearIssue(context.Background(), database.UpdateLinearIssueParams{
		Title:       title,
		Description: description,
		AssigneeID:  assigneeID,
		StateID:     stateID,
		UpdatedAt:   now,
		ID:          issueID,
		SessionID:   sessionID,
	})
	if err != nil {
		log.Printf("[linear] ✗ Failed to update issue: %v", err)
		h.sendError(w, "Failed to update issue")
		return
	}

	// Get updated issue
	updatedIssue, err := h.queries.GetLinearIssueByID(context.Background(), database.GetLinearIssueByIDParams{
		ID:        issueID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[linear] ✗ Failed to get updated issue: %v", err)
		h.sendError(w, "Failed to get updated issue")
		return
	}

	issue := h.convertIssueFromGetByIDRow(updatedIssue, sessionID)

	response := map[string]interface{}{
		"issueUpdate": map[string]interface{}{
			"success": true,
			"issue":   issue,
		},
	}
	h.sendSuccess(w, response)
	log.Printf("[linear] ✓ Updated issue: %s", issueID)
}

// Helper functions

func (h *Handler) convertIssueFromGetByIDRow(dbIssue database.GetLinearIssueByIDRow, sessionID string) Issue {
	issue := Issue{
		ID:          dbIssue.ID,
		Title:       dbIssue.Title,
		Description: dbIssue.Description.String,
		CreatedAt:   time.UnixMilli(dbIssue.CreatedAt),
		UpdatedAt:   time.UnixMilli(dbIssue.UpdatedAt),
		URL:         dbIssue.Url,
	}

	if dbIssue.ArchivedAt.Valid {
		archivedAt := time.UnixMilli(dbIssue.ArchivedAt.Int64)
		issue.ArchivedAt = &archivedAt
	}

	// Get assignee if exists
	if dbIssue.AssigneeID.Valid {
		if dbUser, err := h.queries.GetLinearUserByID(context.Background(), database.GetLinearUserByIDParams{
			ID:        dbIssue.AssigneeID.String,
			SessionID: sessionID,
		}); err == nil {
			issue.Assignee = &User{
				ID:    dbUser.ID,
				Name:  dbUser.Name,
				Email: dbUser.Email,
			}
		}
	}

	// Get state if exists
	if dbIssue.StateID.Valid {
		if dbState, err := h.queries.GetLinearStateByID(context.Background(), database.GetLinearStateByIDParams{
			ID:        dbIssue.StateID.String,
			SessionID: sessionID,
		}); err == nil {
			issue.State = &State{
				ID:   dbState.ID,
				Name: dbState.Name,
				Type: dbState.Type,
			}
		}
	}

	return issue
}

func (h *Handler) convertIssueFromListByTeamRow(dbIssue database.ListLinearIssuesByTeamRow, sessionID string) Issue {
	issue := Issue{
		ID:          dbIssue.ID,
		Title:       dbIssue.Title,
		Description: dbIssue.Description.String,
		CreatedAt:   time.UnixMilli(dbIssue.CreatedAt),
		UpdatedAt:   time.UnixMilli(dbIssue.UpdatedAt),
		URL:         dbIssue.Url,
	}

	if dbIssue.ArchivedAt.Valid {
		archivedAt := time.UnixMilli(dbIssue.ArchivedAt.Int64)
		issue.ArchivedAt = &archivedAt
	}

	// Get assignee if exists
	if dbIssue.AssigneeID.Valid {
		if dbUser, err := h.queries.GetLinearUserByID(context.Background(), database.GetLinearUserByIDParams{
			ID:        dbIssue.AssigneeID.String,
			SessionID: sessionID,
		}); err == nil {
			issue.Assignee = &User{
				ID:    dbUser.ID,
				Name:  dbUser.Name,
				Email: dbUser.Email,
			}
		}
	}

	// Get state if exists
	if dbIssue.StateID.Valid {
		if dbState, err := h.queries.GetLinearStateByID(context.Background(), database.GetLinearStateByIDParams{
			ID:        dbIssue.StateID.String,
			SessionID: sessionID,
		}); err == nil {
			issue.State = &State{
				ID:   dbState.ID,
				Name: dbState.Name,
				Type: dbState.Type,
			}
		}
	}

	return issue
}

func (h *Handler) convertIssueFromCreateRow(dbIssue database.CreateLinearIssueRow, sessionID string) Issue {
	issue := Issue{
		ID:          dbIssue.ID,
		Title:       dbIssue.Title,
		Description: dbIssue.Description.String,
		CreatedAt:   time.UnixMilli(dbIssue.CreatedAt),
		UpdatedAt:   time.UnixMilli(dbIssue.UpdatedAt),
		URL:         dbIssue.Url,
	}

	if dbIssue.ArchivedAt.Valid {
		archivedAt := time.UnixMilli(dbIssue.ArchivedAt.Int64)
		issue.ArchivedAt = &archivedAt
	}

	// Get assignee if exists
	if dbIssue.AssigneeID.Valid {
		if dbUser, err := h.queries.GetLinearUserByID(context.Background(), database.GetLinearUserByIDParams{
			ID:        dbIssue.AssigneeID.String,
			SessionID: sessionID,
		}); err == nil {
			issue.Assignee = &User{
				ID:    dbUser.ID,
				Name:  dbUser.Name,
				Email: dbUser.Email,
			}
		}
	}

	// Get state if exists
	if dbIssue.StateID.Valid {
		if dbState, err := h.queries.GetLinearStateByID(context.Background(), database.GetLinearStateByIDParams{
			ID:        dbIssue.StateID.String,
			SessionID: sessionID,
		}); err == nil {
			issue.State = &State{
				ID:   dbState.ID,
				Name: dbState.Name,
				Type: dbState.Type,
			}
		}
	}

	return issue
}

func (h *Handler) sendSuccess(w http.ResponseWriter, data interface{}) {
	response := GraphQLResponse{
		Data: data,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (h *Handler) sendError(w http.ResponseWriter, message string) {
	response := GraphQLResponse{
		Errors: []GraphQLError{
			{Message: message},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(response)
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
