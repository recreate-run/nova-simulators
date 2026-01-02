package resend

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

// Resend API request/response structures
type SendEmailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Html    string   `json:"html"`
	Cc      []string `json:"cc,omitempty"`
	Bcc     []string `json:"bcc,omitempty"`
	ReplyTo string   `json:"reply_to,omitempty"`
}

type SendEmailResponse struct {
	ID string `json:"id"`
}

// Handler implements the Resend simulator HTTP handler
type Handler struct {
	queries *database.Queries
}

// NewHandler creates a new Resend simulator handler
func NewHandler(queries *database.Queries) *Handler {
	return &Handler{
		queries: queries,
	}
}

// ServeHTTP implements http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[resend] → %s %s", r.Method, r.URL.Path)

	// Route Resend API requests
	if strings.HasPrefix(r.URL.Path, "/emails") {
		h.handleEmails(w, r)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) handleEmails(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/emails" {
		h.handleSendEmail(w, r)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) handleSendEmail(w http.ResponseWriter, r *http.Request) {
	log.Println("[resend] → Received send email request")

	var req SendEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[resend] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.From == "" {
		log.Println("[resend] ✗ Missing from field")
		http.Error(w, "from field is required", http.StatusBadRequest)
		return
	}
	if len(req.To) == 0 {
		log.Println("[resend] ✗ Missing to field")
		http.Error(w, "to field is required", http.StatusBadRequest)
		return
	}
	if req.Subject == "" {
		log.Println("[resend] ✗ Missing subject field")
		http.Error(w, "subject field is required", http.StatusBadRequest)
		return
	}
	if req.Html == "" {
		log.Println("[resend] ✗ Missing html field")
		http.Error(w, "html field is required", http.StatusBadRequest)
		return
	}

	// Generate email ID
	emailID := generateEmailID()

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Convert arrays to JSON strings for storage
	toJSON, _ := json.Marshal(req.To)
	var ccJSON, bccJSON sql.NullString
	if len(req.Cc) > 0 {
		cc, _ := json.Marshal(req.Cc)
		ccJSON = sql.NullString{String: string(cc), Valid: true}
	}
	if len(req.Bcc) > 0 {
		bcc, _ := json.Marshal(req.Bcc)
		bccJSON = sql.NullString{String: string(bcc), Valid: true}
	}
	var replyTo sql.NullString
	if req.ReplyTo != "" {
		replyTo = sql.NullString{String: req.ReplyTo, Valid: true}
	}

	// Store email in database
	err := h.queries.CreateResendEmail(context.Background(), database.CreateResendEmailParams{
		ID:        emailID,
		FromEmail: req.From,
		ToEmails:  string(toJSON),
		Subject:   req.Subject,
		Html:      req.Html,
		CcEmails:  ccJSON,
		BccEmails: bccJSON,
		ReplyTo:   replyTo,
		SessionID: sessionID,
	})

	if err != nil {
		log.Printf("[resend] ✗ Failed to store email: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := SendEmailResponse{
		ID: emailID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[resend] ✓ Email sent: %s", emailID)
}

// Helper functions

func generateEmailID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
