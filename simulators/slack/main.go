package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// In-memory storage for messages
type Storage struct {
	mu       sync.RWMutex
	messages map[string][]Message
	channels map[string]Channel
}

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

var storage = &Storage{
	messages: make(map[string][]Message),
	channels: map[string]Channel{
		"C001": {ID: "C001", Name: "general", Created: time.Now().Unix()},
		"C002": {ID: "C002", Name: "random", Created: time.Now().Unix()},
	},
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

func main() {
	// Initialize file logger
	if err := initLogger(); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer closeLogger()

	// Wrap all handlers with logging middleware
	http.HandleFunc("/api/auth.test", loggingMiddleware(handleAuthTest))
	http.HandleFunc("/api/chat.postMessage", loggingMiddleware(handlePostMessage))
	http.HandleFunc("/api/conversations.list", loggingMiddleware(handleConversationsList))
	http.HandleFunc("/api/conversations.history", loggingMiddleware(handleConversationHistory))

	log.Println("Slack simulator starting on :9001")
	log.Println("Logging to: simulator.log")
	log.Fatal(http.ListenAndServe(":9001", nil))
}

func handleAuthTest(w http.ResponseWriter, r *http.Request) {
	log.Println("→ Received auth.test request")

	response := AuthTestResponse{
		OK:     true,
		URL:    "https://test-workspace.slack.com/",
		Team:   "Test Workspace",
		User:   "test-user",
		TeamID: "T123456",
		UserID: "U123456",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	log.Println("✓ Auth test successful")
}

func handlePostMessage(w http.ResponseWriter, r *http.Request) {
	log.Println("→ Received chat.postMessage request")

	// Parse URL-encoded form data (slack-go uses application/x-www-form-urlencoded)
	if err := r.ParseForm(); err != nil {
		log.Printf("✗ Failed to parse form: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SlackResponse{OK: false, Error: "invalid_form_data"})
		return
	}

	token := r.FormValue("token")
	channel := r.FormValue("channel")
	text := r.FormValue("text")

	log.Printf("  Token: %s", token)
	log.Printf("  Channel: %s", channel)
	log.Printf("  Text: %s", text)

	// Store message
	timestamp := fmt.Sprintf("%d.%06d", time.Now().Unix(), time.Now().Nanosecond()/1000)
	message := Message{
		Type:      "message",
		User:      "U123456",
		Text:      text,
		Timestamp: timestamp,
	}

	storage.mu.Lock()
	storage.messages[channel] = append(storage.messages[channel], message)
	storage.mu.Unlock()

	response := SlackResponse{
		OK:      true,
		Channel: channel,
		TS:      timestamp,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	log.Println("✓ Message posted successfully")
}

func handleConversationsList(w http.ResponseWriter, r *http.Request) {
	log.Println("→ Received conversations.list request")

	// Parse form data
	if err := r.ParseForm(); err != nil {
		log.Printf("✗ Failed to parse form: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SlackResponse{OK: false, Error: "invalid_form_data"})
		return
	}

	token := r.FormValue("token")
	log.Printf("  Token: %s", token)

	storage.mu.RLock()
	channels := make([]Channel, 0, len(storage.channels))
	for _, ch := range storage.channels {
		channels = append(channels, ch)
	}
	storage.mu.RUnlock()

	response := ConversationsListResponse{
		OK:       true,
		Channels: channels,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	log.Printf("✓ Returned %d channels", len(channels))
}

func handleConversationHistory(w http.ResponseWriter, r *http.Request) {
	log.Println("→ Received conversations.history request")

	// Parse form data
	if err := r.ParseForm(); err != nil {
		log.Printf("✗ Failed to parse form: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SlackResponse{OK: false, Error: "invalid_form_data"})
		return
	}

	token := r.FormValue("token")
	channelID := r.FormValue("channel")
	log.Printf("  Token: %s", token)
	log.Printf("  Channel: %s", channelID)

	storage.mu.RLock()
	messages := storage.messages[channelID]
	storage.mu.RUnlock()

	response := ConversationHistoryResponse{
		OK:       true,
		Messages: messages,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	log.Printf("✓ Returned %d messages", len(messages))
}
