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
		m.createSession(w)
	case http.MethodGet:
		m.listSessions(w)
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
		m.resetSession(w, sessionID)
		return
	}

	// Handle DELETE for session deletion
	if r.Method == http.MethodDelete {
		m.deleteSession(w, sessionID)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (m *Manager) createSession(w http.ResponseWriter) {
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

	response := map[string]string{
		"session_id": sessionID,
		"status":     "created",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[session] ✓ Session created: %s", sessionID)
}

func (m *Manager) deleteSession(w http.ResponseWriter, sessionID string) {
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
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[session] ✓ Session deleted: %s", sessionID)
}

func (m *Manager) resetSession(w http.ResponseWriter, sessionID string) {
	log.Printf("[session] → Resetting session: %s", sessionID)

	// Delete and recreate is simpler than selective cleanup
	m.deleteSession(w, sessionID)
	// Note: We don't recreate the session entry itself, just clear the data

	log.Printf("[session] ✓ Session reset: %s", sessionID)
}

func (m *Manager) listSessions(w http.ResponseWriter) {
	// For now, return simple message
	// In a real implementation, we'd query all sessions from the database
	response := map[string]string{
		"message": "Use POST /sessions to create a new session",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// generateSessionID generates a random session ID
func generateSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("session-%s", hex.EncodeToString(b))
}
