package gdocs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
)

// Google Docs API response structures
type Document struct {
	DocumentID  string          `json:"documentId"`
	Title       string          `json:"title"`
	Body        *DocumentBody   `json:"body,omitempty"`
	RevisionID  string          `json:"revisionId"`
	DocumentURL string          `json:"documentUrl,omitempty"`
}

type DocumentBody struct {
	Content []StructuralElement `json:"content"`
}

type StructuralElement struct {
	StartIndex int64      `json:"startIndex"`
	EndIndex   int64      `json:"endIndex"`
	Paragraph  *Paragraph `json:"paragraph,omitempty"`
}

type Paragraph struct {
	Elements []ParagraphElement `json:"elements"`
}

type ParagraphElement struct {
	StartIndex int64    `json:"startIndex"`
	EndIndex   int64    `json:"endIndex"`
	TextRun    *TextRun `json:"textRun,omitempty"`
}

type TextRun struct {
	Content string `json:"content"`
}

type Location struct {
	Index int64 `json:"index"`
}

type Range struct {
	StartIndex int64 `json:"startIndex"`
	EndIndex   int64 `json:"endIndex"`
}

type SubstringMatchCriteria struct {
	Text      string `json:"text"`
	MatchCase bool   `json:"matchCase"`
}

type InsertTextRequest struct {
	Location *Location `json:"location"`
	Text     string    `json:"text"`
}

type DeleteContentRangeRequest struct {
	Range *Range `json:"range"`
}

type ReplaceAllTextRequest struct {
	ContainsText *SubstringMatchCriteria `json:"containsText"`
	ReplaceText  string                  `json:"replaceText"`
}

type Request struct {
	InsertText         *InsertTextRequest         `json:"insertText,omitempty"`
	DeleteContentRange *DeleteContentRangeRequest `json:"deleteContentRange,omitempty"`
	ReplaceAllText     *ReplaceAllTextRequest     `json:"replaceAllText,omitempty"`
}

type BatchUpdateDocumentRequest struct {
	Requests []*Request `json:"requests"`
}

type BatchUpdateDocumentResponse struct {
	DocumentID string `json:"documentId"`
	Replies    []any  `json:"replies,omitempty"`
}

// Handler implements the Google Docs simulator HTTP handler
type Handler struct {
	queries *database.Queries
}

// NewHandler creates a new Google Docs simulator handler
func NewHandler(queries *database.Queries) *Handler {
	return &Handler{
		queries: queries,
	}
}

// ServeHTTP implements http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[gdocs] → %s %s", r.Method, r.URL.Path)

	// Route Google Docs API requests
	// Note: The path prefix /gdocs/ is already stripped by http.StripPrefix in main.go
	if strings.HasPrefix(r.URL.Path, "/v1/documents") {
		h.handleDocsAPI(w, r)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) handleDocsAPI(w http.ResponseWriter, r *http.Request) {
	// Extract the path after /v1/documents (prefix /gdocs/ already stripped)
	path := strings.TrimPrefix(r.URL.Path, "/v1/documents")

	switch {
	case path == "" && r.Method == http.MethodPost:
		// POST /v1/documents - Create document
		h.handleCreateDocument(w, r)
	case strings.HasSuffix(path, ":batchUpdate") && r.Method == http.MethodPost:
		// POST /v1/documents/{documentId}:batchUpdate
		docID := strings.TrimPrefix(path, "/")
		docID = strings.TrimSuffix(docID, ":batchUpdate")
		h.handleBatchUpdate(w, r, docID)
	case path != "" && r.Method == http.MethodGet:
		// GET /v1/documents/{documentId}
		docID := strings.TrimPrefix(path, "/")
		h.handleGetDocument(w, r, docID)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleCreateDocument(w http.ResponseWriter, r *http.Request) {
	log.Println("[gdocs] → Received create document request")

	var req struct {
		Title string `json:"title"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[gdocs] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Generate document ID and revision ID
	documentID := generateDocumentID()
	revisionID := generateRevisionID()

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Create document in database
	err := h.queries.CreateGdocsDocument(context.Background(), database.CreateGdocsDocumentParams{
		ID:         documentID,
		Title:      req.Title,
		RevisionID: revisionID,
		DocumentID: documentID,
		SessionID:  sessionID,
	})

	if err != nil {
		log.Printf("[gdocs] ✗ Failed to create document: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Initialize empty content with single newline character
	initialContent := []StructuralElement{
		{
			StartIndex: 1,
			EndIndex:   2,
			Paragraph: &Paragraph{
				Elements: []ParagraphElement{
					{
						StartIndex: 1,
						EndIndex:   2,
						TextRun: &TextRun{
							Content: "\n",
						},
					},
				},
			},
		},
	}

	contentJSON, err := json.Marshal(initialContent)
	if err != nil {
		log.Printf("[gdocs] ✗ Failed to marshal initial content: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = h.queries.CreateGdocsContent(context.Background(), database.CreateGdocsContentParams{
		DocumentID:  documentID,
		ContentJson: string(contentJSON),
		EndIndex:    2,
		SessionID:   sessionID,
	})

	if err != nil {
		log.Printf("[gdocs] ✗ Failed to create document content: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := Document{
		DocumentID: documentID,
		Title:      req.Title,
		RevisionID: revisionID,
		Body: &DocumentBody{
			Content: initialContent,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[gdocs] ✓ Document created: %s", documentID)
}

func (h *Handler) handleGetDocument(w http.ResponseWriter, r *http.Request, documentID string) {
	log.Printf("[gdocs] → Received get document request for ID: %s", documentID)

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Query document from database
	dbDoc, err := h.queries.GetGdocsDocumentByID(context.Background(), database.GetGdocsDocumentByIDParams{
		DocumentID: documentID,
		SessionID:  sessionID,
	})
	if err != nil {
		log.Printf("[gdocs] ✗ Failed to get document: %v", err)
		http.NotFound(w, r)
		return
	}

	// Query content from database
	dbContent, err := h.queries.GetGdocsContentByDocumentID(context.Background(), database.GetGdocsContentByDocumentIDParams{
		DocumentID: documentID,
		SessionID:  sessionID,
	})
	if err != nil {
		log.Printf("[gdocs] ✗ Failed to get document content: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Parse content JSON
	var content []StructuralElement
	if err := json.Unmarshal([]byte(dbContent.ContentJson), &content); err != nil {
		log.Printf("[gdocs] ✗ Failed to unmarshal content: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := Document{
		DocumentID: dbDoc.DocumentID,
		Title:      dbDoc.Title,
		RevisionID: dbDoc.RevisionID,
		Body: &DocumentBody{
			Content: content,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[gdocs] ✓ Returned document: %s", documentID)
}

func (h *Handler) handleBatchUpdate(w http.ResponseWriter, r *http.Request, documentID string) {
	log.Printf("[gdocs] → Received batch update request for ID: %s", documentID)

	var req BatchUpdateDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[gdocs] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Get current document content
	dbContent, err := h.queries.GetGdocsContentByDocumentID(context.Background(), database.GetGdocsContentByDocumentIDParams{
		DocumentID: documentID,
		SessionID:  sessionID,
	})
	if err != nil {
		log.Printf("[gdocs] ✗ Failed to get document content: %v", err)
		http.NotFound(w, r)
		return
	}

	// Parse content JSON
	var content []StructuralElement
	if err := json.Unmarshal([]byte(dbContent.ContentJson), &content); err != nil {
		log.Printf("[gdocs] ✗ Failed to unmarshal content: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Process each request
	for _, request := range req.Requests {
		switch {
		case request.InsertText != nil:
			content = h.processInsertText(content, request.InsertText)
		case request.DeleteContentRange != nil:
			content = h.processDeleteContentRange(content, request.DeleteContentRange)
		case request.ReplaceAllText != nil:
			content = h.processReplaceAllText(content, request.ReplaceAllText)
		}
	}

	// Calculate new end index
	endIndex := calculateEndIndex(content)

	// Update content in database
	contentJSON, err := json.Marshal(content)
	if err != nil {
		log.Printf("[gdocs] ✗ Failed to marshal content: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = h.queries.UpdateGdocsContent(context.Background(), database.UpdateGdocsContentParams{
		ContentJson: string(contentJSON),
		EndIndex:    endIndex,
		DocumentID:  documentID,
		SessionID:   sessionID,
	})

	if err != nil {
		log.Printf("[gdocs] ✗ Failed to update document content: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := BatchUpdateDocumentResponse{
		DocumentID: documentID,
		Replies:    []any{},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[gdocs] ✓ Batch update completed: %s", documentID)
}

func (h *Handler) processInsertText(content []StructuralElement, req *InsertTextRequest) []StructuralElement {
	index := req.Location.Index
	text := req.Text

	// Find the paragraph containing the index
	for i := range content {
		if content[i].Paragraph != nil && content[i].StartIndex <= index && index <= content[i].EndIndex {
			// Find the text run containing the index
			for j := range content[i].Paragraph.Elements {
				elem := &content[i].Paragraph.Elements[j]
				if elem.TextRun != nil && elem.StartIndex <= index && index <= elem.EndIndex {
					// Insert text at the position within this text run
					pos := int(index - elem.StartIndex)
					elem.TextRun.Content = elem.TextRun.Content[:pos] + text + elem.TextRun.Content[pos:]

					// Update end indices
					textLen := int64(len(text))
					elem.EndIndex += textLen
					content[i].EndIndex += textLen

					// Update all subsequent elements
					for k := j + 1; k < len(content[i].Paragraph.Elements); k++ {
						content[i].Paragraph.Elements[k].StartIndex += textLen
						content[i].Paragraph.Elements[k].EndIndex += textLen
					}
					for k := i + 1; k < len(content); k++ {
						content[k].StartIndex += textLen
						content[k].EndIndex += textLen
					}
					break
				}
			}
			break
		}
	}

	return content
}

func (h *Handler) processDeleteContentRange(content []StructuralElement, req *DeleteContentRangeRequest) []StructuralElement {
	startIndex := req.Range.StartIndex
	endIndex := req.Range.EndIndex
	deleteLen := endIndex - startIndex

	// Find and delete the range
	for i := range content {
		if content[i].Paragraph != nil {
			for j := range content[i].Paragraph.Elements {
				elem := &content[i].Paragraph.Elements[j]
				if elem.TextRun != nil {
					// Check if this element overlaps with the delete range
					if elem.StartIndex < endIndex && elem.EndIndex > startIndex {
						// Calculate which part of the text to delete
						deleteStart := maxInt(0, int(startIndex-elem.StartIndex))
						deleteEnd := minInt(len(elem.TextRun.Content), int(endIndex-elem.StartIndex))

						// Delete the content
						elem.TextRun.Content = elem.TextRun.Content[:deleteStart] + elem.TextRun.Content[deleteEnd:]

						// Update end index
						elem.EndIndex -= deleteLen
					}

					// Update indices for elements after the deleted range
					if elem.StartIndex >= endIndex {
						elem.StartIndex -= deleteLen
						elem.EndIndex -= deleteLen
					}
				}
			}

			// Update paragraph indices
			if content[i].StartIndex >= endIndex {
				content[i].StartIndex -= deleteLen
				content[i].EndIndex -= deleteLen
			} else if content[i].EndIndex > startIndex {
				content[i].EndIndex -= deleteLen
			}
		}
	}

	return content
}

func (h *Handler) processReplaceAllText(content []StructuralElement, req *ReplaceAllTextRequest) []StructuralElement {
	find := req.ContainsText.Text
	replace := req.ReplaceText
	matchCase := req.ContainsText.MatchCase

	for i := range content {
		if content[i].Paragraph != nil {
			for j := range content[i].Paragraph.Elements {
				elem := &content[i].Paragraph.Elements[j]
				if elem.TextRun != nil {
					originalText := elem.TextRun.Content
					var newText string

					if matchCase {
						newText = strings.ReplaceAll(originalText, find, replace)
					} else {
						// Case-insensitive replacement
						newText = replaceAllCaseInsensitive(originalText, find, replace)
					}

					if newText != originalText {
						lenDiff := int64(len(newText) - len(originalText))
						elem.TextRun.Content = newText
						elem.EndIndex += lenDiff
						content[i].EndIndex += lenDiff

						// Update all subsequent elements
						for k := j + 1; k < len(content[i].Paragraph.Elements); k++ {
							content[i].Paragraph.Elements[k].StartIndex += lenDiff
							content[i].Paragraph.Elements[k].EndIndex += lenDiff
						}
						for k := i + 1; k < len(content); k++ {
							content[k].StartIndex += lenDiff
							content[k].EndIndex += lenDiff
						}
					}
				}
			}
		}
	}

	return content
}

// Helper functions

func generateDocumentID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func generateRevisionID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func calculateEndIndex(content []StructuralElement) int64 {
	if len(content) == 0 {
		return 1
	}
	return content[len(content)-1].EndIndex
}

func replaceAllCaseInsensitive(s, old, newStr string) string {
	lowerOld := strings.ToLower(old)

	result := s
	offset := 0
	for {
		idx := strings.Index(strings.ToLower(result[offset:]), lowerOld)
		if idx == -1 {
			break
		}
		actualIdx := offset + idx
		result = result[:actualIdx] + newStr + result[actualIdx+len(old):]
		offset = actualIdx + len(newStr)
	}
	return result
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
