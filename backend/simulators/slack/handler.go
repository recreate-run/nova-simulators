package slack

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
	"time"

	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
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

type GetUploadURLResponse struct {
	OK        bool   `json:"ok"`
	UploadURL string `json:"upload_url"`
	FileID    string `json:"file_id"`
}

type CompleteUploadResponse struct {
	OK    bool          `json:"ok"`
	Files []UploadedFile `json:"files"`
}

type UploadedFile struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type UserProfile struct {
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	RealName    string `json:"real_name"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Image24     string `json:"image_24"`
	Image32     string `json:"image_32"`
	Image48     string `json:"image_48"`
	Image72     string `json:"image_72"`
	Image192    string `json:"image_192"`
	Image512    string `json:"image_512"`
}

type User struct {
	ID             string      `json:"id"`
	TeamID         string      `json:"team_id"`
	Name           string      `json:"name"`
	Deleted        bool        `json:"deleted"`
	RealName       string      `json:"real_name"`
	TZ             string      `json:"tz"`
	TZLabel        string      `json:"tz_label"`
	TZOffset       int         `json:"tz_offset"`
	Profile        UserProfile `json:"profile"`
	IsAdmin        bool        `json:"is_admin"`
	IsOwner        bool        `json:"is_owner"`
	IsBot          bool        `json:"is_bot"`
	Updated        int64       `json:"updated"`
}

type UserInfoResponse struct {
	OK   bool `json:"ok"`
	User User `json:"user"`
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
	mux.HandleFunc("/api/files.getUploadURLExternal", h.handleGetUploadURL)
	mux.HandleFunc("/api/files.completeUploadExternal", h.handleCompleteUpload)
	mux.HandleFunc("/api/users.info", h.handleUserInfo)
	mux.HandleFunc("/upload/", h.handleFileUpload)
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
	attachments := r.FormValue("attachments")

	log.Printf("[slack]   Token: %s", token)
	log.Printf("[slack]   Channel: %s", channel)
	log.Printf("[slack]   Text: %s", text)
	if attachments != "" {
		log.Printf("[slack]   Attachments: %s", attachments)
	}

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Store message in database
	timestamp := fmt.Sprintf("%d.%06d", time.Now().Unix(), time.Now().Nanosecond()/1000)

	// Convert attachments to database-compatible format
	var attachmentsJSON sql.NullString
	if attachments != "" {
		attachmentsJSON = sql.NullString{String: attachments, Valid: true}
	}

	err := h.queries.CreateMessage(context.Background(), database.CreateMessageParams{
		ChannelID:   channel,
		Type:        "message",
		UserID:      "U123456",
		Text:        text,
		Timestamp:   timestamp,
		Attachments: attachmentsJSON,
		SessionID:   sessionID,
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

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Query channels from database
	dbChannels, err := h.queries.ListChannels(context.Background(), sessionID)
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

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Query messages from database
	dbMessages, err := h.queries.GetMessagesByChannel(context.Background(), database.GetMessagesByChannelParams{
		ChannelID: channelID,
		SessionID: sessionID,
	})
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

func (h *Handler) handleGetUploadURL(w http.ResponseWriter, r *http.Request) {
	log.Println("[slack] → Received files.getUploadURLExternal request")

	// Parse form data
	if err := r.ParseForm(); err != nil {
		log.Printf("[slack] ✗ Failed to parse form: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(SlackResponse{OK: false, Error: "invalid_form_data"})
		return
	}

	filename := r.FormValue("filename")
	lengthStr := r.FormValue("length")
	length, _ := strconv.ParseInt(lengthStr, 10, 64)

	log.Printf("[slack]   Filename: %s", filename)
	log.Printf("[slack]   Length: %d", length)

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Generate file ID
	fileID := generateFileID()
	// Use files.slack.com so the HTTP interceptor can route it properly in tests
	uploadURL := fmt.Sprintf("http://files.slack.com/upload/%s", fileID)

	// Store file metadata in database
	err := h.queries.CreateFile(context.Background(), database.CreateFileParams{
		ID:        fileID,
		Filename:  filename,
		Title:     sql.NullString{String: filename, Valid: true},
		Size:      length,
		UploadUrl: sql.NullString{String: uploadURL, Valid: true},
		UserID:    "U123456",
		SessionID: sessionID,
	})

	if err != nil {
		log.Printf("[slack] ✗ Failed to store file metadata: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(SlackResponse{OK: false, Error: "internal_error"})
		return
	}

	response := GetUploadURLResponse{
		OK:        true,
		UploadURL: uploadURL,
		FileID:    fileID,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[slack] ✓ Generated upload URL for file ID: %s", fileID)
}

func (h *Handler) handleCompleteUpload(w http.ResponseWriter, r *http.Request) {
	log.Println("[slack] → Received files.completeUploadExternal request")

	// Parse form data
	if err := r.ParseForm(); err != nil {
		log.Printf("[slack] ✗ Failed to parse form: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(SlackResponse{OK: false, Error: "invalid_form_data"})
		return
	}

	filesJSON := r.FormValue("files")
	channelID := r.FormValue("channel_id")

	log.Printf("[slack]   Files JSON: %s", filesJSON)
	log.Printf("[slack]   Channel ID: %s", channelID)

	// Parse files array
	var files []map[string]string
	if err := json.Unmarshal([]byte(filesJSON), &files); err != nil {
		log.Printf("[slack] ✗ Failed to parse files JSON: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(SlackResponse{OK: false, Error: "invalid_json"})
		return
	}

	uploadedFiles := make([]UploadedFile, 0, len(files))
	for _, file := range files {
		fileID := file["id"]
		title := file["title"]

		uploadedFiles = append(uploadedFiles, UploadedFile{
			ID:    fileID,
			Title: title,
		})

		log.Printf("[slack]   Completed upload for file: %s (%s)", fileID, title)
	}

	response := CompleteUploadResponse{
		OK:    true,
		Files: uploadedFiles,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[slack] ✓ Completed upload for %d files", len(uploadedFiles))
}

func (h *Handler) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	log.Println("[slack] → Received users.info request")

	// Parse form data
	if err := r.ParseForm(); err != nil {
		log.Printf("[slack] ✗ Failed to parse form: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(SlackResponse{OK: false, Error: "invalid_form_data"})
		return
	}

	userID := r.FormValue("user")
	log.Printf("[slack]   User ID: %s", userID)

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Query user from database
	dbUser, err := h.queries.GetUserByID(context.Background(), database.GetUserByIDParams{
		ID:        userID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[slack] ✗ Failed to query user: %v", err)
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(SlackResponse{OK: false, Error: "user_not_found"})
		return
	}

	// Convert to response format
	user := User{
		ID:       dbUser.ID,
		TeamID:   dbUser.TeamID,
		Name:     dbUser.Name,
		Deleted:  false,
		RealName: dbUser.RealName,
		TZ:       dbUser.Timezone.String,
		TZLabel:  dbUser.TimezoneLabel.String,
		TZOffset: int(dbUser.TimezoneOffset.Int64),
		Profile: UserProfile{
			FirstName:   dbUser.FirstName.String,
			LastName:    dbUser.LastName.String,
			RealName:    dbUser.RealName,
			DisplayName: dbUser.DisplayName.String,
			Email:       dbUser.Email.String,
			Image24:     dbUser.Image24.String,
			Image32:     dbUser.Image32.String,
			Image48:     dbUser.Image48.String,
			Image72:     dbUser.Image72.String,
			Image192:    dbUser.Image192.String,
			Image512:    dbUser.Image512.String,
		},
		IsAdmin: dbUser.IsAdmin != 0,
		IsOwner: dbUser.IsOwner != 0,
		IsBot:   dbUser.IsBot != 0,
		Updated: dbUser.CreatedAt,
	}

	response := UserInfoResponse{
		OK:   true,
		User: user,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[slack] ✓ Returned user info for: %s", userID)
}

func (h *Handler) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	log.Println("[slack] → Received file upload request")

	// For the simulator, we just need to accept the upload and return success
	// We don't actually need to store the file content
	w.WriteHeader(http.StatusOK)
	log.Println("[slack] ✓ File upload successful")
}

// generateFileID generates a random file ID
func generateFileID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "F" + hex.EncodeToString(b)
}
