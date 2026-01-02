package outlook

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

// Microsoft Graph API response structures

// EmailAddress represents an email address in Microsoft Graph
type EmailAddress struct {
	Address string `json:"address"`
	Name    string `json:"name,omitempty"`
}

// Recipient represents a message recipient
type Recipient struct {
	EmailAddress *EmailAddress `json:"emailAddress"`
}

// ItemBody represents the body of a message
type ItemBody struct {
	ContentType string `json:"contentType"` // "text" or "html"
	Content     string `json:"content"`
}

// Message represents an Outlook message
type Message struct {
	ID               string       `json:"id"`
	Subject          string       `json:"subject,omitempty"`
	Body             *ItemBody    `json:"body,omitempty"`
	From             *Recipient   `json:"from,omitempty"`
	ToRecipients     []*Recipient `json:"toRecipients,omitempty"`
	IsRead           bool         `json:"isRead"`
	ReceivedDateTime string       `json:"receivedDateTime,omitempty"`
}

// MessageListResponse represents the response from listing messages
type MessageListResponse struct {
	Value         []*Message `json:"value"`
	NextLink      string     `json:"@odata.nextLink,omitempty"`
	ODataContext  string     `json:"@odata.context"`
}

// SendMailRequest represents the request body for sending email
type SendMailRequest struct {
	Message         *Message `json:"message"`
	SaveToSentItems bool     `json:"saveToSentItems,omitempty"`
}

// Handler implements the Outlook simulator HTTP handler
type Handler struct {
	queries *database.Queries
}

// NewHandler creates a new Outlook simulator handler
func NewHandler(queries *database.Queries) *Handler {
	return &Handler{
		queries: queries,
	}
}

// ServeHTTP implements http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[outlook] → %s %s", r.Method, r.URL.Path)

	// Route Microsoft Graph API requests
	if strings.HasPrefix(r.URL.Path, "/v1.0/me/") || strings.HasPrefix(r.URL.Path, "/me/") {
		h.handleGraphAPI(w, r)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) handleGraphAPI(w http.ResponseWriter, r *http.Request) {
	// Normalize path - remove /v1.0 prefix if present
	path := r.URL.Path
	path = strings.TrimPrefix(path, "/v1.0")
	path = strings.TrimPrefix(path, "/me/")

	switch {
	case path == "sendMail" && r.Method == http.MethodPost:
		h.handleSendMail(w, r)
	case strings.HasPrefix(path, "messages/") && r.Method == http.MethodGet:
		// Extract message ID from path
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			messageID := parts[1]
			h.handleGetMessage(w, r, messageID)
		} else {
			http.Error(w, "Invalid message ID", http.StatusBadRequest)
		}
	case strings.HasPrefix(path, "messages/") && r.Method == http.MethodPatch:
		// Extract message ID from path
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			messageID := parts[1]
			h.handleUpdateMessage(w, r, messageID)
		} else {
			http.Error(w, "Invalid message ID", http.StatusBadRequest)
		}
	case path == "messages" && r.Method == http.MethodGet:
		h.handleListMessages(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleSendMail(w http.ResponseWriter, r *http.Request) {
	log.Println("[outlook] → Received send mail request")

	var req SendMailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[outlook] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Message == nil {
		log.Println("[outlook] ✗ Missing message in request")
		http.Error(w, "Missing message", http.StatusBadRequest)
		return
	}

	msg := req.Message

	// Extract from, to, subject, and body
	fromEmail := ""
	if msg.From != nil && msg.From.EmailAddress != nil {
		fromEmail = msg.From.EmailAddress.Address
	}
	if fromEmail == "" {
		fromEmail = "me@example.com" // Default sender
	}

	toEmail := ""
	if len(msg.ToRecipients) > 0 && msg.ToRecipients[0] != nil && msg.ToRecipients[0].EmailAddress != nil {
		toEmail = msg.ToRecipients[0].EmailAddress.Address
	}

	subject := msg.Subject
	bodyContent := ""
	bodyType := "text"
	if msg.Body != nil {
		bodyContent = msg.Body.Content
		bodyType = strings.ToLower(msg.Body.ContentType)
		if bodyType == "" {
			bodyType = "text"
		}
	}

	// Generate message ID
	messageID := generateMessageID()

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Store message in database
	receivedDateTime := time.Now().UTC().Format(time.RFC3339)

	err := h.queries.CreateOutlookMessage(context.Background(), database.CreateOutlookMessageParams{
		ID:               messageID,
		FromEmail:        fromEmail,
		ToEmail:          toEmail,
		Subject:          subject,
		BodyContent:      sql.NullString{String: bodyContent, Valid: bodyContent != ""},
		BodyType:         bodyType,
		IsRead:           0, // New sent messages are unread by default
		ReceivedDatetime: receivedDateTime,
		SessionID:        sessionID,
	})

	if err != nil {
		log.Printf("[outlook] ✗ Failed to store message: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Microsoft Graph sendMail endpoint returns 202 Accepted with no body
	w.WriteHeader(http.StatusAccepted)
	log.Printf("[outlook] ✓ Message sent: %s", messageID)
}

func (h *Handler) handleListMessages(w http.ResponseWriter, r *http.Request) {
	log.Println("[outlook] → Received list messages request")

	query := r.URL.Query()

	// Handle $top parameter (defaults to 10 in Graph API)
	top := 10
	if topStr := query.Get("$top"); topStr != "" {
		if _, err := fmt.Sscanf(topStr, "%d", &top); err != nil {
			// If parsing fails, use default value
			top = 10
		}
	}

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Handle $filter parameter for searching
	filter := query.Get("$filter")

	if filter != "" {
		// Parse simple filters - support common patterns like:
		// from/address eq 'email@example.com'
		// subject eq 'test'
		// contains(subject, 'test')
		fromEmail, toEmail, subject, bodySearch := parseFilter(filter)

		searchResults, err := h.queries.SearchOutlookMessages(context.Background(), database.SearchOutlookMessagesParams{
			SessionID:   sessionID,
			Column2:     fromEmail,
			Column3:     sql.NullString{String: fromEmail, Valid: fromEmail != ""},
			Column4:     toEmail,
			Column5:     sql.NullString{String: toEmail, Valid: toEmail != ""},
			Column6:     subject,
			Column7:     sql.NullString{String: subject, Valid: subject != ""},
			Column8:     bodySearch,
			Column9:     sql.NullString{String: bodySearch, Valid: bodySearch != ""},
			Limit:       int64(top),
		})

		if err != nil {
			log.Printf("[outlook] ✗ Failed to search messages: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Build response
		messageList := make([]*Message, 0, len(searchResults))
		for i := range searchResults {
			messageList = append(messageList, searchRowToGraphMessage(searchResults[i]))
		}

		response := MessageListResponse{
			Value:        messageList,
			ODataContext: "https://graph.microsoft.com/v1.0/$metadata#users('me')/messages",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
		log.Printf("[outlook] ✓ Listed %d messages", len(searchResults))
		return
	}

	// List all messages
	listResults, err := h.queries.ListOutlookMessages(context.Background(), database.ListOutlookMessagesParams{
		SessionID: sessionID,
		Limit:     int64(top),
	})

	if err != nil {
		log.Printf("[outlook] ✗ Failed to list messages: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build response
	messageList := make([]*Message, 0, len(listResults))
	for i := range listResults {
		messageList = append(messageList, listRowToGraphMessage(listResults[i]))
	}

	response := MessageListResponse{
		Value:        messageList,
		ODataContext: "https://graph.microsoft.com/v1.0/$metadata#users('me')/messages",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[outlook] ✓ Listed %d messages", len(listResults))
}

func (h *Handler) handleGetMessage(w http.ResponseWriter, r *http.Request, messageID string) {
	log.Printf("[outlook] → Received get message request for ID: %s", messageID)

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Query message from database
	dbMessage, err := h.queries.GetOutlookMessageByID(context.Background(), database.GetOutlookMessageByIDParams{
		ID:        messageID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[outlook] ✗ Failed to get message: %v", err)
		http.NotFound(w, r)
		return
	}

	message := getRowToGraphMessage(dbMessage)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(message)
	log.Printf("[outlook] ✓ Returned message: %s", messageID)
}

func (h *Handler) handleUpdateMessage(w http.ResponseWriter, r *http.Request, messageID string) {
	log.Printf("[outlook] → Received update message request for ID: %s", messageID)

	var updateReq Message
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		log.Printf("[outlook] ✗ Failed to decode update request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// First, verify the message exists
	_, err := h.queries.GetOutlookMessageByID(context.Background(), database.GetOutlookMessageByIDParams{
		ID:        messageID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[outlook] ✗ Message not found: %v", err)
		http.NotFound(w, r)
		return
	}

	// Update read status if provided
	isReadValue := int64(0)
	if updateReq.IsRead {
		isReadValue = 1
	}

	err = h.queries.UpdateOutlookMessageReadStatus(context.Background(), database.UpdateOutlookMessageReadStatusParams{
		IsRead:    isReadValue,
		ID:        messageID,
		SessionID: sessionID,
	})

	if err != nil {
		log.Printf("[outlook] ✗ Failed to update message: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return the updated message
	dbMessage, err := h.queries.GetOutlookMessageByID(context.Background(), database.GetOutlookMessageByIDParams{
		ID:        messageID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[outlook] ✗ Failed to get updated message: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	message := getRowToGraphMessage(dbMessage)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(message)
	log.Printf("[outlook] ✓ Updated message: %s", messageID)
}

// Helper functions

func generateMessageID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "AAMkAD" + hex.EncodeToString(b) // Microsoft Graph message IDs start with AAMkAD
}

func getRowToGraphMessage(msg database.GetOutlookMessageByIDRow) *Message {
	bodyContent := ""
	if msg.BodyContent.Valid {
		bodyContent = msg.BodyContent.String
	}

	return &Message{
		ID:      msg.ID,
		Subject: msg.Subject,
		Body: &ItemBody{
			ContentType: msg.BodyType,
			Content:     bodyContent,
		},
		From: &Recipient{
			EmailAddress: &EmailAddress{
				Address: msg.FromEmail,
			},
		},
		ToRecipients: []*Recipient{
			{
				EmailAddress: &EmailAddress{
					Address: msg.ToEmail,
				},
			},
		},
		IsRead:           msg.IsRead != 0,
		ReceivedDateTime: msg.ReceivedDatetime,
	}
}

func listRowToGraphMessage(msg database.ListOutlookMessagesRow) *Message {
	bodyContent := ""
	if msg.BodyContent.Valid {
		bodyContent = msg.BodyContent.String
	}

	return &Message{
		ID:      msg.ID,
		Subject: msg.Subject,
		Body: &ItemBody{
			ContentType: msg.BodyType,
			Content:     bodyContent,
		},
		From: &Recipient{
			EmailAddress: &EmailAddress{
				Address: msg.FromEmail,
			},
		},
		ToRecipients: []*Recipient{
			{
				EmailAddress: &EmailAddress{
					Address: msg.ToEmail,
				},
			},
		},
		IsRead:           msg.IsRead != 0,
		ReceivedDateTime: msg.ReceivedDatetime,
	}
}

func searchRowToGraphMessage(msg database.SearchOutlookMessagesRow) *Message {
	bodyContent := ""
	if msg.BodyContent.Valid {
		bodyContent = msg.BodyContent.String
	}

	return &Message{
		ID:      msg.ID,
		Subject: msg.Subject,
		Body: &ItemBody{
			ContentType: msg.BodyType,
			Content:     bodyContent,
		},
		From: &Recipient{
			EmailAddress: &EmailAddress{
				Address: msg.FromEmail,
			},
		},
		ToRecipients: []*Recipient{
			{
				EmailAddress: &EmailAddress{
					Address: msg.ToEmail,
				},
			},
		},
		IsRead:           msg.IsRead != 0,
		ReceivedDateTime: msg.ReceivedDatetime,
	}
}

func parseFilter(filter string) (fromEmail, toEmail, subject, bodySearch string) {
	// Simple filter parser for common patterns
	// Examples:
	// - "from/emailAddress/address eq 'test@example.com'"
	// - "subject eq 'Test Subject'"
	// - "contains(subject, 'test')"

	filter = strings.TrimSpace(filter)

	// Parse "from/emailAddress/address eq 'value'"
	if strings.Contains(filter, "from/emailAddress/address eq") {
		parts := strings.Split(filter, "eq")
		if len(parts) == 2 {
			fromEmail = strings.Trim(strings.TrimSpace(parts[1]), "'\"")
		}
	}

	// Parse "toRecipients/emailAddress/address eq 'value'"
	if strings.Contains(filter, "toRecipients/emailAddress/address eq") || strings.Contains(filter, "to/emailAddress/address eq") {
		parts := strings.Split(filter, "eq")
		if len(parts) == 2 {
			toEmail = strings.Trim(strings.TrimSpace(parts[1]), "'\"")
		}
	}

	// Parse "subject eq 'value'"
	if strings.Contains(filter, "subject eq") {
		parts := strings.Split(filter, "eq")
		if len(parts) == 2 {
			subject = strings.Trim(strings.TrimSpace(parts[1]), "'\"")
		}
	}

	// Parse "contains(subject, 'value')" or "contains(body, 'value')"
	if strings.Contains(filter, "contains(") {
		start := strings.Index(filter, "(")
		end := strings.LastIndex(filter, ")")
		if start != -1 && end != -1 {
			inner := filter[start+1 : end]
			parts := strings.Split(inner, ",")
			if len(parts) == 2 {
				field := strings.TrimSpace(parts[0])
				value := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
				switch field {
				case "subject":
					subject = value
				case "body":
					bodySearch = value
				}
			}
		}
	}

	return
}
