package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/recreate-run/nova-simulators/internal/database"
)

type Message struct {
	Type      string `json:"type"`
	User      string `json:"user"`
	Text      string `json:"text"`
	Timestamp string `json:"ts"`
}

type Channel struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Created int64  `json:"created"`
}

// Slack API response structures
type SlackResponse struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Channel string `json:"channel,omitempty"`
	TS      string `json:"ts,omitempty"`
}

type AuthTestResponse struct {
	OK     bool   `json:"ok"`
	URL    string `json:"url"`
	Team   string `json:"team"`
	User   string `json:"user"`
	TeamID string `json:"team_id"`
	UserID string `json:"user_id"`
}

type ConversationsListResponse struct {
	OK       bool      `json:"ok"`
	Channels []Channel `json:"channels"`
}

type ConversationHistoryResponse struct {
	OK       bool      `json:"ok"`
	Messages []Message `json:"messages"`
}

// Handler implements the Slack simulator HTTP handler
type Handler struct {
	queries *database.Queries
}

// NewHandler creates a new Slack simulator handler
func NewHandler(queries *database.Queries) *Handler {
	return &Handler{
		queries: queries,
	}
}

// ServeHTTP implements http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth.test", h.handleAuthTest)
	mux.HandleFunc("/api/chat.postMessage", h.handlePostMessage)
	mux.HandleFunc("/api/conversations.list", h.handleConversationsList)
	mux.HandleFunc("/api/conversations.history", h.handleConversationHistory)
	mux.ServeHTTP(w, r)
}

func (h *Handler) handleAuthTest(w http.ResponseWriter, r *http.Request) {
	log.Println("[slack] → Received auth.test request")

	response := AuthTestResponse{
		OK:     true,
		URL:    "https://test-workspace.slack.com/",
		Team:   "Test Workspace",
		User:   "test-user",
		TeamID: "T123456",
		UserID: "U123456",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Println("[slack] ✓ Auth test successful")
}

func (h *Handler) handlePostMessage(w http.ResponseWriter, r *http.Request) {
	log.Println("[slack] → Received chat.postMessage request")

	// Parse URL-encoded form data (slack-go uses application/x-www-form-urlencoded)
	if err := r.ParseForm(); err != nil {
		log.Printf("[slack] ✗ Failed to parse form: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(SlackResponse{OK: false, Error: "invalid_form_data"})
		return
	}

	token := r.FormValue("token")
	channel := r.FormValue("channel")
	text := r.FormValue("text")

	log.Printf("[slack]   Token: %s", token)
	log.Printf("[slack]   Channel: %s", channel)
	log.Printf("[slack]   Text: %s", text)

	// Store message in database
	timestamp := fmt.Sprintf("%d.%06d", time.Now().Unix(), time.Now().Nanosecond()/1000)

	err := h.queries.CreateMessage(context.Background(), database.CreateMessageParams{
		ChannelID: channel,
		Type:      "message",
		UserID:    "U123456",
		Text:      text,
		Timestamp: timestamp,
	})

	if err != nil {
		log.Printf("[slack] ✗ Failed to insert message: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(SlackResponse{OK: false, Error: "internal_error"})
		return
	}

	response := SlackResponse{
		OK:      true,
		Channel: channel,
		TS:      timestamp,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Println("[slack] ✓ Message posted successfully")
}

func (h *Handler) handleConversationsList(w http.ResponseWriter, r *http.Request) {
	log.Println("[slack] → Received conversations.list request")

	// Parse form data
	if err := r.ParseForm(); err != nil {
		log.Printf("[slack] ✗ Failed to parse form: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(SlackResponse{OK: false, Error: "invalid_form_data"})
		return
	}

	token := r.FormValue("token")
	log.Printf("[slack]   Token: %s", token)

	// Query channels from database
	dbChannels, err := h.queries.ListChannels(context.Background())
	if err != nil {
		log.Printf("[slack] ✗ Failed to query channels: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(SlackResponse{OK: false, Error: "internal_error"})
		return
	}

	// Convert to response format
	channels := make([]Channel, 0, len(dbChannels))
	for _, ch := range dbChannels {
		channels = append(channels, Channel{
			ID:      ch.ID,
			Name:    ch.Name,
			Created: ch.CreatedAt,
		})
	}

	response := ConversationsListResponse{
		OK:       true,
		Channels: channels,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[slack] ✓ Returned %d channels", len(channels))
}

func (h *Handler) handleConversationHistory(w http.ResponseWriter, r *http.Request) {
	log.Println("[slack] → Received conversations.history request")

	// Parse form data
	if err := r.ParseForm(); err != nil {
		log.Printf("[slack] ✗ Failed to parse form: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(SlackResponse{OK: false, Error: "invalid_form_data"})
		return
	}

	token := r.FormValue("token")
	channelID := r.FormValue("channel")
	log.Printf("[slack]   Token: %s", token)
	log.Printf("[slack]   Channel: %s", channelID)

	// Query messages from database
	dbMessages, err := h.queries.GetMessagesByChannel(context.Background(), channelID)
	if err != nil {
		log.Printf("[slack] ✗ Failed to query messages: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(SlackResponse{OK: false, Error: "internal_error"})
		return
	}

	// Convert to response format
	messages := make([]Message, 0, len(dbMessages))
	for _, msg := range dbMessages {
		messages = append(messages, Message{
			Type:      msg.Type,
			User:      msg.UserID,
			Text:      msg.Text,
			Timestamp: msg.Timestamp,
		})
	}

	response := ConversationHistoryResponse{
		OK:       true,
		Messages: messages,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[slack] ✓ Returned %d messages", len(messages))
}
