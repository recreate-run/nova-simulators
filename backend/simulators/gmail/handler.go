package gmail

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
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
	Size         int    `json:"size"`
	Data         string `json:"data,omitempty"`
	AttachmentID string `json:"attachmentId,omitempty"`
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
	// Support both /v1/users/ (when used with StripPrefix in production)
	// and /gmail/v1/users/ (when used directly in tests)
	if strings.HasPrefix(r.URL.Path, "/v1/users/") || strings.HasPrefix(r.URL.Path, "/gmail/v1/users/") {
		h.handleGmailAPI(w, r)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) handleGmailAPI(w http.ResponseWriter, r *http.Request) {
	// Extract the path after /v1/users/{userId}/ or /gmail/v1/users/{userId}/
	path := r.URL.Path
	path = strings.TrimPrefix(path, "/gmail/v1/users/me/")
	if path == r.URL.Path {
		// Didn't match /gmail prefix, try without it
		path = strings.TrimPrefix(path, "/v1/users/me/")
	}

	switch {
	case strings.HasPrefix(path, "messages/send"):
		h.handleSendMessage(w, r)
	case strings.HasPrefix(path, "messages/import"):
		h.handleImportMessage(w, r)
	case strings.HasPrefix(path, "messages/") && strings.Contains(path, "/attachments/") && r.Method == http.MethodGet:
		// Extract message ID and attachment ID from path: messages/{msgId}/attachments/{attachmentId}
		parts := strings.Split(path, "/")
		if len(parts) >= 4 && parts[2] == "attachments" {
			messageID := parts[1]
			attachmentID := parts[3]
			h.handleGetAttachment(w, r, messageID, attachmentID)
		} else {
			http.Error(w, "Invalid attachment path", http.StatusBadRequest)
		}
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

	// Parse email headers and attachments
	parsed := parseEmailWithAttachments(rawMessage)

	// Generate message ID and thread ID
	messageID := generateMessageID()
	threadID := messageID // For new messages, thread ID equals message ID

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Store message in database
	internalDate := time.Now().UnixMilli()
	snippet := generateSnippet(parsed.bodyPlain, parsed.bodyHTML)

	err = h.queries.CreateGmailMessage(context.Background(), database.CreateGmailMessageParams{
		ID:           messageID,
		ThreadID:     threadID,
		FromEmail:    parsed.from,
		ToEmail:      parsed.to,
		Subject:      parsed.subject,
		BodyPlain:    sql.NullString{String: parsed.bodyPlain, Valid: parsed.bodyPlain != ""},
		BodyHtml:     sql.NullString{String: parsed.bodyHTML, Valid: parsed.bodyHTML != ""},
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

	// Store attachments
	for _, att := range parsed.attachments {
		err = h.queries.CreateGmailAttachment(context.Background(), database.CreateGmailAttachmentParams{
			ID:        att.ID,
			MessageID: messageID,
			Filename:  att.Filename,
			MimeType:  att.MimeType,
			Data:      att.Data,
			Size:      int64(att.Size),
			SessionID: sessionID,
		})
		if err != nil {
			log.Printf("[gmail] ✗ Failed to store attachment: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}
	if len(parsed.attachments) > 0 {
		log.Printf("[gmail]   Stored %d attachment(s)", len(parsed.attachments))
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

func (h *Handler) handleImportMessage(w http.ResponseWriter, r *http.Request) {
	log.Println("[gmail] → Received import message request")

	var req struct {
		Raw      string   `json:"raw"`
		LabelIDs []string `json:"labelIds,omitempty"`
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

	// Parse email headers and attachments
	parsed := parseEmailWithAttachments(rawMessage)

	// Generate message ID and thread ID
	messageID := generateMessageID()
	threadID := messageID // For new messages, thread ID equals message ID

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Use provided labels or default to INBOX + UNREAD
	labels := req.LabelIDs
	if len(labels) == 0 {
		labels = []string{"INBOX", "UNREAD"}
	}
	labelJSON, _ := json.Marshal(labels)

	// Store message in database
	internalDate := time.Now().UnixMilli()
	snippet := generateSnippet(parsed.bodyPlain, parsed.bodyHTML)

	err = h.queries.CreateGmailMessage(context.Background(), database.CreateGmailMessageParams{
		ID:           messageID,
		ThreadID:     threadID,
		FromEmail:    parsed.from,
		ToEmail:      parsed.to,
		Subject:      parsed.subject,
		BodyPlain:    sql.NullString{String: parsed.bodyPlain, Valid: parsed.bodyPlain != ""},
		BodyHtml:     sql.NullString{String: parsed.bodyHTML, Valid: parsed.bodyHTML != ""},
		RawMessage:   rawMessage,
		Snippet:      sql.NullString{String: snippet, Valid: true},
		LabelIds:     sql.NullString{String: string(labelJSON), Valid: true},
		InternalDate: internalDate,
		SizeEstimate: int64(len(rawMessage)),
		SessionID:    sessionID,
	})

	if err != nil {
		log.Printf("[gmail] ✗ Failed to store message: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Store attachments
	for _, att := range parsed.attachments {
		err = h.queries.CreateGmailAttachment(context.Background(), database.CreateGmailAttachmentParams{
			ID:        att.ID,
			MessageID: messageID,
			Filename:  att.Filename,
			MimeType:  att.MimeType,
			Data:      att.Data,
			Size:      int64(att.Size),
			SessionID: sessionID,
		})
		if err != nil {
			log.Printf("[gmail] ✗ Failed to store attachment: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}
	if len(parsed.attachments) > 0 {
		log.Printf("[gmail]   Stored %d attachment(s)", len(parsed.attachments))
	}

	response := SendMessageResponse{
		ID:       messageID,
		ThreadID: threadID,
		LabelIDs: labels,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[gmail] ✓ Message imported: %s", messageID)
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

	// Parse page token for offset
	offset := 0
	pageToken := query.Get("pageToken")
	if pageToken != "" {
		if decodedOffset, err := decodePageToken(pageToken); err == nil {
			offset = decodedOffset
		}
	}

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Check if search query is present
	searchQuery := query.Get("q")

	var messages []MessageListItem

	if searchQuery != "" {
		// Use search functionality
		log.Printf("[gmail]   Search query: %s", searchQuery)
		params := parseSearchQuery(searchQuery)

		// Call search query
		dbMessages, err := h.queries.SearchGmailMessages(context.Background(), database.SearchGmailMessagesParams{
			SessionID: sessionID,
			Column2:   params.from,
			Column3:   sql.NullString{String: params.from, Valid: true},
			Column4:   params.to,
			Column5:   sql.NullString{String: params.to, Valid: true},
			Column6:   params.subject,
			Column7:   sql.NullString{String: params.subject, Valid: true},
			Column8:   params.body,
			Column9:   sql.NullString{String: params.body, Valid: true},
			Column10:  params.label,
			Column11:  sql.NullString{String: params.label, Valid: true},
			Limit:     int64(maxResults),
		})
		if err != nil {
			log.Printf("[gmail] ✗ Failed to search messages: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		messages = make([]MessageListItem, 0, len(dbMessages))
		for i := range dbMessages {
			messages = append(messages, MessageListItem{
				ID:       dbMessages[i].ID,
				ThreadID: dbMessages[i].ThreadID,
			})
		}
	} else {
		// List all messages with pagination
		// Request one extra to check if there are more results
		dbMessages, err := h.queries.ListGmailMessages(context.Background(), database.ListGmailMessagesParams{
			SessionID: sessionID,
			Limit:     int64(maxResults + 1),
			Offset:    int64(offset),
		})
		if err != nil {
			log.Printf("[gmail] ✗ Failed to list messages: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		messages = make([]MessageListItem, 0, len(dbMessages))
		for i := range dbMessages {
			messages = append(messages, MessageListItem{
				ID:       dbMessages[i].ID,
				ThreadID: dbMessages[i].ThreadID,
			})
		}
	}

	// Generate next page token if there are more results
	var nextPageToken string
	if len(messages) > maxResults {
		messages = messages[:maxResults]
		nextPageToken = encodePageToken(offset + maxResults)
	}

	response := MessageListResponse{
		Messages:          messages,
		NextPageToken:     nextPageToken,
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

	// Get attachments for this message
	attachments, err := h.queries.ListGmailAttachmentsByMessage(context.Background(), database.ListGmailAttachmentsByMessageParams{
		MessageID: messageID,
		SessionID: sessionID,
	})
	if err == nil {
		// Add attachment parts (data not included, only metadata)
		for i, att := range attachments {
			partID := fmt.Sprintf("att_%d", i)
			parts = append(parts, MessagePart{
				PartID:   partID,
				MimeType: att.MimeType,
				Filename: att.Filename,
				Headers:  []Header{{Name: "Content-Type", Value: att.MimeType}},
				Body: &MessageBody{
					Size:         int(att.Size),
					AttachmentID: att.ID,
				},
			})
		}
	}

	// Determine MIME type
	mimeType := "text/plain"
	if len(parts) > 1 {
		mimeType = "multipart/mixed"
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

func (h *Handler) handleGetAttachment(w http.ResponseWriter, r *http.Request, messageID, attachmentID string) {
	log.Printf("[gmail] → Received get attachment request for message: %s, attachment: %s", messageID, attachmentID)

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Query attachment from database
	attachment, err := h.queries.GetGmailAttachment(context.Background(), database.GetGmailAttachmentParams{
		ID:        attachmentID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[gmail] ✗ Failed to get attachment: %v", err)
		http.NotFound(w, r)
		return
	}

	// Verify attachment belongs to the requested message
	if attachment.MessageID != messageID {
		log.Printf("[gmail] ✗ Attachment does not belong to message")
		http.NotFound(w, r)
		return
	}

	// Encode data as base64url
	encodedData := base64.URLEncoding.EncodeToString(attachment.Data)

	response := struct {
		Size int    `json:"size"`
		Data string `json:"data"`
	}{
		Size: int(attachment.Size),
		Data: encodedData,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[gmail] ✓ Returned attachment: %s", attachmentID)
}

// Helper functions

type attachment struct {
	ID       string
	Filename string
	MimeType string
	Data     []byte
	Size     int
}

type searchParams struct {
	from    string
	to      string
	subject string
	body    string
	label   string
}

type emailParseResult struct {
	from        string
	to          string
	subject     string
	bodyPlain   string
	bodyHTML    string
	attachments []attachment
}

func parseSearchQuery(q string) searchParams {
	params := searchParams{}
	if q == "" {
		return params
	}

	// Simple parser for Gmail search syntax
	// Supports: from:, to:, subject:, is:unread, is:read, label:
	parts := strings.Fields(q)

	for _, part := range parts {
		switch {
		case strings.HasPrefix(part, "from:"):
			params.from = strings.TrimPrefix(part, "from:")
		case strings.HasPrefix(part, "to:"):
			params.to = strings.TrimPrefix(part, "to:")
		case strings.HasPrefix(part, "subject:"):
			params.subject = strings.TrimPrefix(part, "subject:")
		case part == "is:unread":
			params.label = "UNREAD"
		case part == "is:read":
			// Messages without UNREAD label are considered read
			// We'll handle this by searching for messages without UNREAD
			params.label = "!UNREAD"
		case strings.HasPrefix(part, "label:"):
			params.label = strings.TrimPrefix(part, "label:")
		default:
			// Treat as body text search if no prefix
			if params.body != "" {
				params.body += " "
			}
			params.body += part
		}
	}

	return params
}

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

func parseEmailWithAttachments(raw string) emailParseResult {
	// First try simple parsing for non-MIME messages
	if !strings.Contains(raw, "Content-Type: multipart") {
		from, to, subject, bodyPlain, bodyHTML := parseEmail(raw)
		return emailParseResult{
			from:      from,
			to:        to,
			subject:   subject,
			bodyPlain: bodyPlain,
			bodyHTML:  bodyHTML,
		}
	}

	// Parse MIME multipart message
	lines := strings.Split(raw, "\r\n")
	var contentType string
	var boundary string
	var from, to, subject, bodyPlain, bodyHTML string
	var attachments []attachment

	// Parse top-level headers
	i := 0
	for i < len(lines) {
		line := lines[i]
		if line == "" {
			i++
			break
		}

		// Parse header
		switch {
		case strings.HasPrefix(line, "From: "):
			from = strings.TrimPrefix(line, "From: ")
		case strings.HasPrefix(line, "To: "):
			to = strings.TrimPrefix(line, "To: ")
		case strings.HasPrefix(line, "Subject: "):
			subject = strings.TrimPrefix(line, "Subject: ")
		case strings.HasPrefix(line, "Content-Type: "):
			contentType = strings.TrimPrefix(line, "Content-Type: ")
			// Extract boundary
			if strings.Contains(contentType, "boundary=") {
				parts := strings.Split(contentType, "boundary=")
				if len(parts) > 1 {
					boundary = strings.Trim(parts[1], "\"")
				}
			}
		}
		i++
	}

	if boundary == "" {
		// No boundary found, fall back to simple parsing
		from, to, subject, bodyPlain, bodyHTML = parseEmail(raw)
		return emailParseResult{
			from:      from,
			to:        to,
			subject:   subject,
			bodyPlain: bodyPlain,
			bodyHTML:  bodyHTML,
		}
	}

	// Parse MIME parts
	bodyBytes := []byte(strings.Join(lines[i:], "\r\n"))
	reader := multipart.NewReader(bytes.NewReader(bodyBytes), boundary)

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			break
		}

		partContentType := part.Header.Get("Content-Type")
		disposition := part.Header.Get("Content-Disposition")

		// Read part data
		partData, err := io.ReadAll(part)
		if err != nil {
			continue
		}

		// Check if it's an attachment
		switch {
		case strings.Contains(disposition, "attachment") || strings.Contains(disposition, "filename="):
			filename := extractFilename(disposition, partContentType)
			if filename == "" {
				filename = "attachment"
			}

			mimeType := strings.Split(partContentType, ";")[0]
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}

			// Decode if base64 encoded
			encoding := part.Header.Get("Content-Transfer-Encoding")
			var decodedData []byte
			if strings.EqualFold(encoding, "base64") {
				decodedData, _ = base64.StdEncoding.DecodeString(string(partData))
			} else {
				decodedData = partData
			}

			att := attachment{
				ID:       generateMessageID(),
				Filename: filename,
				MimeType: mimeType,
				Data:     decodedData,
				Size:     len(decodedData),
			}
			attachments = append(attachments, att)
		case strings.HasPrefix(partContentType, "text/plain"):
			bodyPlain = string(partData)
		case strings.HasPrefix(partContentType, "text/html"):
			bodyHTML = string(partData)
		}
	}

	return emailParseResult{
		from:        from,
		to:          to,
		subject:     subject,
		bodyPlain:   bodyPlain,
		bodyHTML:    bodyHTML,
		attachments: attachments,
	}
}

func extractFilename(disposition, contentType string) string {
	// Try Content-Disposition first
	if strings.Contains(disposition, "filename=") {
		parts := strings.Split(disposition, "filename=")
		if len(parts) > 1 {
			filename := strings.Trim(parts[1], "\"")
			filename = strings.Split(filename, ";")[0]
			return strings.TrimSpace(filename)
		}
	}

	// Try Content-Type
	if strings.Contains(contentType, "name=") {
		parts := strings.Split(contentType, "name=")
		if len(parts) > 1 {
			filename := strings.Trim(parts[1], "\"")
			filename = strings.Split(filename, ";")[0]
			return strings.TrimSpace(filename)
		}
	}

	return ""
}

func encodePageToken(offset int) string {
	token := fmt.Sprintf("%d", offset)
	return base64.URLEncoding.EncodeToString([]byte(token))
}

func decodePageToken(token string) (int, error) {
	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return 0, err
	}
	offset, err := strconv.Atoi(string(decoded))
	if err != nil {
		return 0, err
	}
	return offset, nil
}
