package whatsapp

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
)

// TextMessage represents a text message payload
type TextMessage struct {
	MessagingProduct string      `json:"messaging_product"`
	RecipientType    string      `json:"recipient_type"`
	To               string      `json:"to"`
	Type             string      `json:"type"`
	Text             TextContent `json:"text"`
}

// TextContent represents the text content
type TextContent struct {
	Body string `json:"body"`
}

// TemplateMessage represents a template message payload
type TemplateMessage struct {
	MessagingProduct string   `json:"messaging_product"`
	To               string   `json:"to"`
	Type             string   `json:"type"`
	Template         Template `json:"template"`
}

// Template represents a message template
type Template struct {
	Name     string   `json:"name"`
	Language Language `json:"language"`
}

// Language represents the template language
type Language struct {
	Code string `json:"code"`
}

// MediaMessage represents a media message payload
type MediaMessage struct {
	MessagingProduct string       `json:"messaging_product"`
	RecipientType    string       `json:"recipient_type"`
	To               string       `json:"to"`
	Type             string       `json:"type"`
	Image            *MediaObject `json:"image,omitempty"`
	Document         *MediaObject `json:"document,omitempty"`
	Video            *MediaObject `json:"video,omitempty"`
}

// MediaObject represents a media object
type MediaObject struct {
	Link    string `json:"link,omitempty"`
	Caption string `json:"caption,omitempty"`
}

// MessageResponse represents the API response
type MessageResponse struct {
	MessagingProduct string `json:"messaging_product"`
	Contacts         []struct {
		Input string `json:"input"`
		WaID  string `json:"wa_id"`
	} `json:"contacts"`
	Messages []struct {
		ID string `json:"id"`
	} `json:"messages"`
}

// Handler implements the WhatsApp simulator HTTP handler
type Handler struct {
	queries *database.Queries
}

// NewHandler creates a new WhatsApp simulator handler
func NewHandler(queries *database.Queries) *Handler {
	return &Handler{
		queries: queries,
	}
}

// ServeHTTP implements http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[whatsapp] → %s %s", r.Method, r.URL.Path)

	// Route WhatsApp Cloud API requests: /v21.0/{phone_number_id}/messages
	if strings.HasPrefix(r.URL.Path, "/v21.0/") {
		h.handleWhatsAppAPI(w, r)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) handleWhatsAppAPI(w http.ResponseWriter, r *http.Request) {
	// Extract phone_number_id from path: /v21.0/{phone_number_id}/messages
	path := strings.TrimPrefix(r.URL.Path, "/v21.0/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 || parts[1] != "messages" {
		http.NotFound(w, r)
		return
	}

	phoneNumberID := parts[0]

	if r.Method == http.MethodPost {
		h.handleSendMessage(w, r, phoneNumberID)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) handleSendMessage(w http.ResponseWriter, r *http.Request, phoneNumberID string) {
	log.Println("[whatsapp] → Received send message request")

	// Decode the request body
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("[whatsapp] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Extract common fields
	msgType, _ := payload["type"].(string)
	to, _ := payload["to"].(string)

	if msgType == "" || to == "" {
		log.Printf("[whatsapp] ✗ Missing required fields: type or to")
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Generate message ID
	messageID := generateMessageID()

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Parse message type and extract content
	var textBody, mediaURL, caption, templateName, languageCode sql.NullString

	switch msgType {
	case "text":
		if textObj, ok := payload["text"].(map[string]interface{}); ok {
			if body, ok := textObj["body"].(string); ok {
				textBody = sql.NullString{String: body, Valid: true}
			}
		}
	case "template":
		if templateObj, ok := payload["template"].(map[string]interface{}); ok {
			if name, ok := templateObj["name"].(string); ok {
				templateName = sql.NullString{String: name, Valid: true}
			}
			if langObj, ok := templateObj["language"].(map[string]interface{}); ok {
				if code, ok := langObj["code"].(string); ok {
					languageCode = sql.NullString{String: code, Valid: true}
				}
			}
		}
	case "image", "document", "video":
		if mediaObj, ok := payload[msgType].(map[string]interface{}); ok {
			if link, ok := mediaObj["link"].(string); ok {
				mediaURL = sql.NullString{String: link, Valid: true}
			}
			if captionStr, ok := mediaObj["caption"].(string); ok {
				caption = sql.NullString{String: captionStr, Valid: true}
			}
		}
	default:
		log.Printf("[whatsapp] ✗ Unsupported message type: %s", msgType)
		http.Error(w, "Unsupported message type", http.StatusBadRequest)
		return
	}

	// Store message in database
	err := h.queries.CreateWhatsAppMessage(context.Background(), database.CreateWhatsAppMessageParams{
		ID:            messageID,
		PhoneNumberID: phoneNumberID,
		ToNumber:      to,
		MessageType:   msgType,
		TextBody:      textBody,
		MediaUrl:      mediaURL,
		Caption:       caption,
		TemplateName:  templateName,
		LanguageCode:  languageCode,
		SessionID:     sessionID,
	})

	if err != nil {
		log.Printf("[whatsapp] ✗ Failed to store message: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build response
	response := MessageResponse{
		MessagingProduct: "whatsapp",
		Contacts: []struct {
			Input string `json:"input"`
			WaID  string `json:"wa_id"`
		}{
			{
				Input: to,
				WaID:  to,
			},
		},
		Messages: []struct {
			ID string `json:"id"`
		}{
			{
				ID: messageID,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[whatsapp] ✓ Message sent: %s", messageID)
}

// Helper functions

func generateMessageID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "wamid." + hex.EncodeToString(b)
}
