package gdocs_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
	simulatorGdocs "github.com/recreate-run/nova-simulators/simulators/gdocs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/option"
	_ "modernc.org/sqlite"
)

func setupTestDBForSeed(t *testing.T) *database.Queries {
	t.Helper()
	// Use in-memory SQLite database for tests
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err, "Failed to open test database")

	// Set goose dialect
	err = goose.SetDialect("sqlite3")
	require.NoError(t, err, "Failed to set goose dialect")

	// Run migrations
	err = goose.Up(db, "../../migrations")
	require.NoError(t, err, "Failed to run migrations")

	return database.New(db)
}

// TestGdocsInitialStateSeed demonstrates seeding arbitrary initial state for GDocs simulator
func TestGdocsInitialStateSeed(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDBForSeed(t)
	ctx := context.Background()

	// Setup: Create a new session
	sessionID := "gdocs-seed-test-session"
	err := queries.CreateSession(ctx, sessionID)
	require.NoError(t, err, "Failed to create session")

	// Seed: Create custom documents with content
	documents := seedGdocsTestData(t, ctx, queries, sessionID)

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorGdocs.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client that adds session header
	transport := &sessionHTTPTransport{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create Google Docs service
	docsService, err := docs.NewService(ctx,
		option.WithoutAuthentication(),
		option.WithEndpoint(server.URL+"/"),
		option.WithHTTPClient(customClient),
	)
	require.NoError(t, err, "Failed to create Docs service")

	// Verify: Check that documents are queryable
	t.Run("VerifyDocuments", func(t *testing.T) {
		verifyDocuments(t, docsService, documents)
	})

	// Verify: Check that content is queryable
	t.Run("VerifyDocumentContent", func(t *testing.T) {
		verifyDocumentContent(t, docsService, documents)
	})

	// Verify: Check database isolation - ensure all data is correctly stored
	t.Run("VerifyDatabaseIsolation", func(t *testing.T) {
		verifyDatabaseIsolation(t, ctx, queries, sessionID, documents)
	})
}

// seedGdocsTestData creates documents with content for testing
func seedGdocsTestData(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string) []struct {
	DocumentID string
	Title      string
	RevisionID string
	Content    string
} {
	t.Helper()

	// Seed: Create custom documents (use session-specific IDs to avoid conflicts)
	documents := []struct {
		DocumentID string
		Title      string
		RevisionID string
		Content    string
	}{
		{
			DocumentID: "DOC001_" + sessionID,
			Title:      "Project Proposal",
			RevisionID: "REV001_" + sessionID,
			Content:    "This is the project proposal document with detailed plans.",
		},
		{
			DocumentID: "DOC002_" + sessionID,
			Title:      "Meeting Notes",
			RevisionID: "REV002_" + sessionID,
			Content:    "Notes from the team meeting on product roadmap.",
		},
		{
			DocumentID: "DOC003_" + sessionID,
			Title:      "Technical Specification",
			RevisionID: "REV003_" + sessionID,
			Content:    "Technical details and architecture diagrams.",
		},
	}

	for _, doc := range documents {
		// Create document record
		err := queries.CreateGdocsDocument(ctx, database.CreateGdocsDocumentParams{
			ID:         doc.DocumentID,
			Title:      doc.Title,
			RevisionID: doc.RevisionID,
			DocumentID: doc.DocumentID,
			SessionID:  sessionID,
		})
		require.NoError(t, err, "Failed to create document: %s", doc.Title)

		// Create content with the document text
		content := []simulatorGdocs.StructuralElement{
			{
				StartIndex: 1,
				EndIndex:   int64(len(doc.Content) + 2), // +1 for start, +1 for newline
				Paragraph: &simulatorGdocs.Paragraph{
					Elements: []simulatorGdocs.ParagraphElement{
						{
							StartIndex: 1,
							EndIndex:   int64(len(doc.Content) + 2),
							TextRun: &simulatorGdocs.TextRun{
								Content: doc.Content + "\n",
							},
						},
					},
				},
			},
		}

		contentJSON, err := json.Marshal(content)
		require.NoError(t, err, "Failed to marshal content for document: %s", doc.Title)

		err = queries.CreateGdocsContent(ctx, database.CreateGdocsContentParams{
			DocumentID:  doc.DocumentID,
			ContentJson: string(contentJSON),
			EndIndex:    int64(len(doc.Content) + 2),
			SessionID:   sessionID,
		})
		require.NoError(t, err, "Failed to create document content: %s", doc.Title)
	}

	return documents
}

// verifyDocuments verifies that documents can be queried
func verifyDocuments(t *testing.T, docsService *docs.Service, documents []struct {
	DocumentID string
	Title      string
	RevisionID string
	Content    string
}) {
	t.Helper()

	// Get each document and verify metadata
	for _, doc := range documents {
		retrieved, err := docsService.Documents.Get(doc.DocumentID).Do()
		require.NoError(t, err, "GetDocument should succeed for: %s", doc.Title)
		assert.Equal(t, doc.DocumentID, retrieved.DocumentId, "Document ID should match")
		assert.Equal(t, doc.Title, retrieved.Title, "Title should match")
		assert.Equal(t, doc.RevisionID, retrieved.RevisionId, "Revision ID should match")
		assert.NotNil(t, retrieved.Body, "Body should not be nil")
		assert.NotEmpty(t, retrieved.Body.Content, "Body content should not be empty")
	}
}

// verifyDocumentContent verifies that document content can be queried
func verifyDocumentContent(t *testing.T, docsService *docs.Service, documents []struct {
	DocumentID string
	Title      string
	RevisionID string
	Content    string
}) {
	t.Helper()

	// Get each document and verify content
	for _, doc := range documents {
		retrieved, err := docsService.Documents.Get(doc.DocumentID).Do()
		require.NoError(t, err, "GetDocument should succeed for: %s", doc.Title)

		// Extract text content
		var textContent string
		for _, element := range retrieved.Body.Content {
			if element.Paragraph != nil && element.Paragraph.Elements != nil {
				for _, elem := range element.Paragraph.Elements {
					if elem.TextRun != nil {
						textContent += elem.TextRun.Content
					}
				}
			}
		}

		assert.Contains(t, textContent, doc.Content, "Document should contain seeded content")
	}
}

// verifyDatabaseIsolation verifies database isolation
func verifyDatabaseIsolation(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string,
	documents []struct {
		DocumentID string
		Title      string
		RevisionID string
		Content    string
	}) {
	t.Helper()

	// Query documents from database
	for _, doc := range documents {
		dbDoc, err := queries.GetGdocsDocumentByID(ctx, database.GetGdocsDocumentByIDParams{
			DocumentID: doc.DocumentID,
			SessionID:  sessionID,
		})
		require.NoError(t, err, "GetGdocsDocumentByID should succeed for: %s", doc.Title)
		assert.Equal(t, doc.Title, dbDoc.Title, "Document title should match in database")
		assert.Equal(t, doc.RevisionID, dbDoc.RevisionID, "Revision ID should match in database")

		// Query content from database
		dbContent, err := queries.GetGdocsContentByDocumentID(ctx, database.GetGdocsContentByDocumentIDParams{
			DocumentID: doc.DocumentID,
			SessionID:  sessionID,
		})
		require.NoError(t, err, "GetGdocsContentByDocumentID should succeed for: %s", doc.Title)

		// Parse and verify content
		var content []simulatorGdocs.StructuralElement
		err = json.Unmarshal([]byte(dbContent.ContentJson), &content)
		require.NoError(t, err, "Content should be valid JSON")

		var textContent string
		for _, element := range content {
			if element.Paragraph != nil {
				for _, elem := range element.Paragraph.Elements {
					if elem.TextRun != nil {
						textContent += elem.TextRun.Content
					}
				}
			}
		}

		assert.Contains(t, textContent, doc.Content, "Database content should match seeded content")
	}
}
