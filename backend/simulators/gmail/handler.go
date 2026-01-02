package gmail

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
)

// Gmail API response structures
type SendMessageResponse struct {
	ID       string   `json:"id"`
	ThreadID string   `json:"threadId"`
	LabelIDs []string `json:"labelIds"`
}

type MessageListResponse struct {
	Messages          []MessageListItem `json:"messages,omitempty"`
	NextPageToken     string            `json:"nextPageToken,omitempty"`
	ResultSizeEstimate int              `json:"resultSizeEstimate"`
}

type MessageListItem struct {
	ID       string `json:"id"`
	ThreadID string `json:"threadId"`
}

type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type MessagePart struct {
	PartID   string        `json:"partId"`
	MimeType string        `json:"mimeType"`
	Filename string        `json:"filename,omitempty"`
	Headers  []Header      `json:"headers,omitempty"`
	Body     *MessageBody  `json:"body,omitempty"`
	Parts    []MessagePart `json:"parts,omitempty"`
}

type MessageBody struct {
	Size int    `json:"size"`
	Data string `json:"data,omitempty"`
}

type MessagePayload struct {
	PartID   string        `json:"partId"`
	MimeType string        `json:"mimeType"`
	Filename string        `json:"filename"`
	Headers  []Header      `json:"headers"`
	Body     *MessageBody  `json:"body"`
	Parts    []MessagePart `json:"parts,omitempty"`
}

type Message struct {
	ID           string          `json:"id"`
	ThreadID     string          `json:"threadId"`
	LabelIDs     []string        `json:"labelIds,omitempty"`
	Snippet      string          `json:"snippet,omitempty"`
	Payload      *MessagePayload `json:"payload,omitempty"`
	SizeEstimate int             `json:"sizeEstimate,omitempty"`
	InternalDate string          `json:"internalDate,omitempty"`
}

// Handler implements the Gmail simulator HTTP handler
type Handler struct {
	queries *database.Queries
}

// NewHandler creates a new Gmail simulator handler
func NewHandler(queries *database.Queries) *Handler {
	return &Handler{
		queries: queries,
	}
}

// ServeHTTP implements http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[gmail] → %s %s", r.Method, r.URL.Path)

	// Route Gmail API requests
	if strings.HasPrefix(r.URL.Path, "/gmail/v1/users/") {
		h.handleGmailAPI(w, r)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) handleGmailAPI(w http.ResponseWriter, r *http.Request) {
	// Extract the path after /gmail/v1/users/{userId}/
	path := strings.TrimPrefix(r.URL.Path, "/gmail/v1/users/me/")

	switch {
	case strings.HasPrefix(path, "messages/send"):
		h.handleSendMessage(w, r)
	case strings.HasPrefix(path, "messages/") && r.Method == http.MethodGet:
		// Extract message ID from path
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			messageID := parts[1]
			h.handleGetMessage(w, r, messageID)
		} else {
			http.Error(w, "Invalid message ID", http.StatusBadRequest)
		}
	case path == "messages" && r.Method == http.MethodGet:
		h.handleListMessages(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	log.Println("[gmail] → Received send message request")

	var req struct {
		Raw string `json:"raw"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[gmail] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Decode base64url message
	rawBytes, err := base64.URLEncoding.DecodeString(req.Raw)
	if err != nil {
		log.Printf("[gmail] ✗ Failed to decode base64: %v", err)
		http.Error(w, "Invalid base64 encoding", http.StatusBadRequest)
		return
	}

	rawMessage := string(rawBytes)
	log.Printf("[gmail]   Raw message: %s", rawMessage)

	// Parse email headers
	from, to, subject, bodyPlain, bodyHTML := parseEmail(rawMessage)

	// Generate message ID and thread ID
	messageID := generateMessageID()
	threadID := messageID // For new messages, thread ID equals message ID

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Store message in database
	internalDate := time.Now().UnixMilli()
	snippet := generateSnippet(bodyPlain, bodyHTML)

	err = h.queries.CreateGmailMessage(context.Background(), database.CreateGmailMessageParams{
		ID:           messageID,
		ThreadID:     threadID,
		FromEmail:    from,
		ToEmail:      to,
		Subject:      subject,
		BodyPlain:    sql.NullString{String: bodyPlain, Valid: bodyPlain != ""},
		BodyHtml:     sql.NullString{String: bodyHTML, Valid: bodyHTML != ""},
		RawMessage:   rawMessage,
		Snippet:      sql.NullString{String: snippet, Valid: true},
		LabelIds:     sql.NullString{String: `["SENT"]`, Valid: true},
		InternalDate: internalDate,
		SizeEstimate: int64(len(rawMessage)),
		SessionID:    sessionID,
	})

	if err != nil {
		log.Printf("[gmail] ✗ Failed to store message: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := SendMessageResponse{
		ID:       messageID,
		ThreadID: threadID,
		LabelIDs: []string{"SENT"},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[gmail] ✓ Message sent: %s", messageID)
}

func (h *Handler) handleListMessages(w http.ResponseWriter, r *http.Request) {
	log.Println("[gmail] → Received list messages request")

	query := r.URL.Query()
	maxResultsStr := query.Get("maxResults")
	maxResults := 100
	if maxResultsStr != "" {
		if mr, err := strconv.Atoi(maxResultsStr); err == nil {
			maxResults = mr
		}
	}

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Query messages from database
	dbMessages, err := h.queries.ListGmailMessages(context.Background(), database.ListGmailMessagesParams{
		SessionID: sessionID,
		Limit:     int64(maxResults),
	})
	if err != nil {
		log.Printf("[gmail] ✗ Failed to list messages: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	messages := make([]MessageListItem, 0, len(dbMessages))
	for _, msg := range dbMessages {
		messages = append(messages, MessageListItem{
			ID:       msg.ID,
			ThreadID: msg.ThreadID,
		})
	}

	response := MessageListResponse{
		Messages:          messages,
		ResultSizeEstimate: len(messages),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[gmail] ✓ Listed %d messages", len(messages))
}

func (h *Handler) handleGetMessage(w http.ResponseWriter, r *http.Request, messageID string) {
	log.Printf("[gmail] → Received get message request for ID: %s", messageID)

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Query message from database
	dbMessage, err := h.queries.GetGmailMessageByID(context.Background(), database.GetGmailMessageByIDParams{
		ID:        messageID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[gmail] ✗ Failed to get message: %v", err)
		http.NotFound(w, r)
		return
	}

	// Parse label IDs
	var labelIDs []string
	if dbMessage.LabelIds.Valid && dbMessage.LabelIds.String != "" {
		_ = json.Unmarshal([]byte(dbMessage.LabelIds.String), &labelIDs)
	}

	// Build headers
	headers := []Header{
		{Name: "From", Value: dbMessage.FromEmail},
		{Name: "To", Value: dbMessage.ToEmail},
		{Name: "Subject", Value: dbMessage.Subject},
		{Name: "Date", Value: time.UnixMilli(dbMessage.InternalDate).Format(time.RFC1123Z)},
	}

	// Build message parts
	var parts []MessagePart
	if dbMessage.BodyPlain.Valid && dbMessage.BodyPlain.String != "" {
		plainData := base64.URLEncoding.EncodeToString([]byte(dbMessage.BodyPlain.String))
		parts = append(parts, MessagePart{
			PartID:   "0",
			MimeType: "text/plain",
			Filename: "",
			Headers:  []Header{{Name: "Content-Type", Value: "text/plain; charset=\"UTF-8\""}},
			Body: &MessageBody{
				Size: len(dbMessage.BodyPlain.String),
				Data: plainData,
			},
		})
	}

	if dbMessage.BodyHtml.Valid && dbMessage.BodyHtml.String != "" {
		htmlData := base64.URLEncoding.EncodeToString([]byte(dbMessage.BodyHtml.String))
		parts = append(parts, MessagePart{
			PartID:   "1",
			MimeType: "text/html",
			Filename: "",
			Headers:  []Header{{Name: "Content-Type", Value: "text/html; charset=\"UTF-8\""}},
			Body: &MessageBody{
				Size: len(dbMessage.BodyHtml.String),
				Data: htmlData,
			},
		})
	}

	// Determine MIME type
	mimeType := "text/plain"
	if len(parts) > 1 {
		mimeType = "multipart/alternative"
	}

	message := Message{
		ID:           dbMessage.ID,
		ThreadID:     dbMessage.ThreadID,
		LabelIDs:     labelIDs,
		Snippet:      dbMessage.Snippet.String,
		InternalDate: fmt.Sprintf("%d", dbMessage.InternalDate),
		SizeEstimate: int(dbMessage.SizeEstimate),
		Payload: &MessagePayload{
			PartID:   "",
			MimeType: mimeType,
			Filename: "",
			Headers:  headers,
			Body:     &MessageBody{Size: 0},
			Parts:    parts,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(message)
	log.Printf("[gmail] ✓ Returned message: %s", messageID)
}

// Helper functions

func generateMessageID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func parseEmail(raw string) (from, to, subject, bodyPlain, bodyHTML string) {
	lines := strings.Split(raw, "\r\n")
	inBody := false
	bodyLines := []string{}

	for _, line := range lines {
		if !inBody {
			if line == "" {
				inBody = true
				continue
			}

			// Parse headers
			switch {
			case strings.HasPrefix(line, "From: "):
				from = strings.TrimPrefix(line, "From: ")
			case strings.HasPrefix(line, "To: "):
				to = strings.TrimPrefix(line, "To: ")
			case strings.HasPrefix(line, "Subject: "):
				subject = strings.TrimPrefix(line, "Subject: ")
			}
		} else {
			bodyLines = append(bodyLines, line)
		}
	}

	body := strings.Join(bodyLines, "\r\n")

	// Check if body contains HTML
	if strings.Contains(body, "<html>") || strings.Contains(body, "<HTML>") {
		bodyHTML = body
		bodyPlain = stripHTML(body)
	} else {
		bodyPlain = body
	}

	return
}

func stripHTML(html string) string {
	// Simple HTML tag removal
	re := regexp.MustCompile(`<[^>]*>`)
	return strings.TrimSpace(re.ReplaceAllString(html, ""))
}

func generateSnippet(bodyPlain, bodyHTML string) string {
	text := bodyPlain
	if text == "" {
		text = stripHTML(bodyHTML)
	}

	if len(text) > 200 {
		return text[:200] + "..."
	}
	return text
}
