package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/recreate-run/nova-simulators/internal/database"
)

// Manager handles session lifecycle operations
type Manager struct {
	queries *database.Queries
}

// NewManager creates a new session manager
func NewManager(queries *database.Queries) *Manager {
	return &Manager{
		queries: queries,
	}
}

// ServeHTTP implements http.Handler interface for session management endpoints
func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sessions", m.handleSessions)
	mux.HandleFunc("/sessions/", m.handleSessionDetail)
	mux.ServeHTTP(w, r)
}

func (m *Manager) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		m.createSession(w, r)
	case http.MethodGet:
		m.listSessions(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	// Extract session ID from path: /sessions/{id} or /sessions/{id}/reset
	path := strings.TrimPrefix(r.URL.Path, "/sessions/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	sessionID := parts[0]

	// Check for /reset suffix
	if len(parts) > 1 && parts[1] == "reset" {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		m.resetSession(w, r, sessionID)
		return
	}

	// Handle DELETE for session deletion
	if r.Method == http.MethodDelete {
		m.deleteSession(w, r, sessionID)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (m *Manager) createSession(w http.ResponseWriter, r *http.Request) {
	log.Println("[session] → Creating new session")

	// Generate random session ID
	sessionID := generateSessionID()

	// Create session in database
	err := m.queries.CreateSession(context.Background(), sessionID)
	if err != nil {
		log.Printf("[session] ✗ Failed to create session: %v", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Create default channels for Slack in this session
	timestamp := int64(1640000000) // Default timestamp
	err = m.queries.CreateChannel(context.Background(), database.CreateChannelParams{
		ID:        "C001",
		Name:      "general",
		CreatedAt: timestamp,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[session] ✗ Failed to create default channel: %v", err)
	}

	err = m.queries.CreateChannel(context.Background(), database.CreateChannelParams{
		ID:        "C002",
		Name:      "random",
		CreatedAt: timestamp,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[session] ✗ Failed to create default channel: %v", err)
	}

	// Create default users for Slack in this session
	err = m.queries.CreateUser(context.Background(), database.CreateUserParams{
		ID:              "U123456",
		TeamID:          "T021F9ZE2",
		Name:            "test-user",
		RealName:        "Test User",
		Email:           database.StringToNullString("test@example.com"),
		DisplayName:     database.StringToNullString("testuser"),
		FirstName:       database.StringToNullString("Test"),
		LastName:        database.StringToNullString("User"),
		IsAdmin:         0,
		IsOwner:         0,
		IsBot:           0,
		Timezone:        database.StringToNullString("America/Los_Angeles"),
		TimezoneLabel:   database.StringToNullString("Pacific Standard Time"),
		TimezoneOffset:  database.Int64ToNullInt64(-28800),
		Image24:         database.StringToNullString(""),
		Image32:         database.StringToNullString(""),
		Image48:         database.StringToNullString(""),
		Image72:         database.StringToNullString(""),
		Image192:        database.StringToNullString(""),
		Image512:        database.StringToNullString(""),
		SessionID:       sessionID,
	})
	if err != nil {
		log.Printf("[session] ✗ Failed to create default user: %v", err)
	}

	err = m.queries.CreateUser(context.Background(), database.CreateUserParams{
		ID:              "U789012",
		TeamID:          "T021F9ZE2",
		Name:            "bobby",
		RealName:        "Bobby Tables",
		Email:           database.StringToNullString("bobby@example.com"),
		DisplayName:     database.StringToNullString("bobby"),
		FirstName:       database.StringToNullString("Bobby"),
		LastName:        database.StringToNullString("Tables"),
		IsAdmin:         0,
		IsOwner:         0,
		IsBot:           0,
		Timezone:        database.StringToNullString("America/Los_Angeles"),
		TimezoneLabel:   database.StringToNullString("Pacific Standard Time"),
		TimezoneOffset:  database.Int64ToNullInt64(-28800),
		Image24:         database.StringToNullString(""),
		Image32:         database.StringToNullString(""),
		Image48:         database.StringToNullString(""),
		Image72:         database.StringToNullString(""),
		Image192:        database.StringToNullString(""),
		Image512:        database.StringToNullString(""),
		SessionID:       sessionID,
	})
	if err != nil {
		log.Printf("[session] ✗ Failed to create default user: %v", err)
	}

	response := map[string]string{
		"session_id": sessionID,
		"status":     "created",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	log.Printf("[session] ✓ Session created: %s", sessionID)
}

func (m *Manager) deleteSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	log.Printf("[session] → Deleting session: %s", sessionID)

	// Delete all Slack data for this session
	err := m.queries.DeleteSessionData(context.Background(), sessionID)
	if err != nil {
		log.Printf("[session] ✗ Failed to delete Slack session data: %v", err)
	}

	// Delete all Gmail data for this session
	err = m.queries.DeleteGmailSessionData(context.Background(), sessionID)
	if err != nil {
		log.Printf("[session] ✗ Failed to delete Gmail session data: %v", err)
	}

	response := map[string]string{
		"session_id": sessionID,
		"status":     "deleted",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	log.Printf("[session] ✓ Session deleted: %s", sessionID)
}

func (m *Manager) resetSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	log.Printf("[session] → Resetting session: %s", sessionID)

	// Delete and recreate is simpler than selective cleanup
	m.deleteSession(w, r, sessionID)
	// Note: We don't recreate the session entry itself, just clear the data

	log.Printf("[session] ✓ Session reset: %s", sessionID)
}

func (m *Manager) listSessions(w http.ResponseWriter, r *http.Request) {
	// For now, return simple message
	// In a real implementation, we'd query all sessions from the database
	response := map[string]string{
		"message": "Use POST /sessions to create a new session",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// generateSessionID generates a random session ID
func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("session-%s", hex.EncodeToString(b))
}
