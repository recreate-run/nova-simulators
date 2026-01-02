package gdocs_test

import (
	"context"
	"database/sql"
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

// sessionHTTPTransport wraps http.RoundTripper and adds session header to all requests
type sessionHTTPTransport struct {
	sessionID string
}

func (t *sessionHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-Session-ID", t.sessionID)
	return http.DefaultTransport.RoundTrip(req)
}

func setupTestDB(t *testing.T) *database.Queries {
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

func TestGdocsSimulatorCreateDocument(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "gdocs-test-session-1"

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

	// Create Google Docs service pointing to test server with custom client
	ctx := context.Background()
	docsService, err := docs.NewService(ctx,
		option.WithoutAuthentication(),
		option.WithEndpoint(server.URL+"/"),
		option.WithHTTPClient(customClient),
	)
	require.NoError(t, err, "Failed to create Docs service")

	t.Run("CreateEmptyDocument", func(t *testing.T) {
		// Create document
		doc := &docs.Document{
			Title: "Test Document",
		}

		created, err := docsService.Documents.Create(doc).Do()

		// Assertions
		require.NoError(t, err, "Create should not return error")
		assert.NotNil(t, created, "Should return created document")
		assert.NotEmpty(t, created.DocumentId, "Document ID should not be empty")
		assert.Equal(t, "Test Document", created.Title, "Title should match")
		assert.NotEmpty(t, created.RevisionId, "Revision ID should not be empty")
		assert.NotNil(t, created.Body, "Body should not be nil")
		assert.NotEmpty(t, created.Body.Content, "Body content should not be empty")
	})

	t.Run("CreateMultipleDocuments", func(t *testing.T) {
		// Create first document
		doc1 := &docs.Document{Title: "Document 1"}
		created1, err := docsService.Documents.Create(doc1).Do()
		require.NoError(t, err, "First create should succeed")

		// Create second document
		doc2 := &docs.Document{Title: "Document 2"}
		created2, err := docsService.Documents.Create(doc2).Do()
		require.NoError(t, err, "Second create should succeed")

		// Verify they have different IDs
		assert.NotEqual(t, created1.DocumentId, created2.DocumentId, "Document IDs should be different")
	})
}

func TestGdocsSimulatorGetDocument(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "gdocs-test-session-2"

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
	ctx := context.Background()
	docsService, err := docs.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(customClient))
	require.NoError(t, err, "Failed to create Docs service")

	// Create a test document
	doc := &docs.Document{Title: "Test Document for Get"}
	created, err := docsService.Documents.Create(doc).Do()
	require.NoError(t, err, "Create should succeed")

	t.Run("GetExistingDocument", func(t *testing.T) {
		// Get the document
		retrieved, err := docsService.Documents.Get(created.DocumentId).Do()

		// Assertions
		require.NoError(t, err, "Get should not return error")
		assert.NotNil(t, retrieved, "Should return document")
		assert.Equal(t, created.DocumentId, retrieved.DocumentId, "Document ID should match")
		assert.Equal(t, created.Title, retrieved.Title, "Title should match")
		assert.Equal(t, created.RevisionId, retrieved.RevisionId, "Revision ID should match")
		assert.NotNil(t, retrieved.Body, "Body should not be nil")
		assert.NotEmpty(t, retrieved.Body.Content, "Body content should not be empty")
	})

	t.Run("GetNonExistentDocument", func(t *testing.T) {
		// Try to get a non-existent document
		_, err := docsService.Documents.Get("nonexistent").Do()

		// Assertions
		assert.Error(t, err, "Should return error for non-existent document")
	})
}

func TestGdocsSimulatorAppendText(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "gdocs-test-session-3"

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
	ctx := context.Background()
	docsService, err := docs.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(customClient))
	require.NoError(t, err, "Failed to create Docs service")

	// Create a test document
	doc := &docs.Document{Title: "Test Document for Append"}
	created, err := docsService.Documents.Create(doc).Do()
	require.NoError(t, err, "Create should succeed")

	t.Run("AppendText", func(t *testing.T) {
		// Get the document to find the end index
		retrieved, err := docsService.Documents.Get(created.DocumentId).Do()
		require.NoError(t, err, "Get should succeed")

		// The end index is at the last position in the document
		endIndex := retrieved.Body.Content[len(retrieved.Body.Content)-1].EndIndex - 1

		// Append text
		requests := []*docs.Request{
			{
				InsertText: &docs.InsertTextRequest{
					Location: &docs.Location{
						Index: endIndex,
					},
					Text: "Hello, World!",
				},
			},
		}

		batchUpdate := &docs.BatchUpdateDocumentRequest{
			Requests: requests,
		}

		_, err = docsService.Documents.BatchUpdate(created.DocumentId, batchUpdate).Do()
		require.NoError(t, err, "BatchUpdate should succeed")

		// Verify the text was added
		updated, err := docsService.Documents.Get(created.DocumentId).Do()
		require.NoError(t, err, "Get should succeed")

		// Extract text content
		var textContent string
		for _, element := range updated.Body.Content {
			if element.Paragraph != nil && element.Paragraph.Elements != nil {
				for _, elem := range element.Paragraph.Elements {
					if elem.TextRun != nil {
						textContent += elem.TextRun.Content
					}
				}
			}
		}

		assert.Contains(t, textContent, "Hello, World!", "Text should be appended")
	})
}

func TestGdocsSimulatorInsertText(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "gdocs-test-session-4"

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
	ctx := context.Background()
	docsService, err := docs.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(customClient))
	require.NoError(t, err, "Failed to create Docs service")

	// Create a test document and add some initial text
	doc := &docs.Document{Title: "Test Document for Insert"}
	created, err := docsService.Documents.Create(doc).Do()
	require.NoError(t, err, "Create should succeed")

	// Add initial text
	retrieved, err := docsService.Documents.Get(created.DocumentId).Do()
	require.NoError(t, err, "Get should succeed")
	endIndex := retrieved.Body.Content[len(retrieved.Body.Content)-1].EndIndex - 1

	initialText := "Hello World"
	_, err = docsService.Documents.BatchUpdate(created.DocumentId, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{
			{
				InsertText: &docs.InsertTextRequest{
					Location: &docs.Location{Index: endIndex},
					Text:     initialText,
				},
			},
		},
	}).Do()
	require.NoError(t, err, "Initial text insert should succeed")

	t.Run("InsertTextAtIndex", func(t *testing.T) {
		// Insert text at index 7 (after "Hello ")
		requests := []*docs.Request{
			{
				InsertText: &docs.InsertTextRequest{
					Location: &docs.Location{
						Index: 7,
					},
					Text: "Beautiful ",
				},
			},
		}

		_, err = docsService.Documents.BatchUpdate(created.DocumentId, &docs.BatchUpdateDocumentRequest{
			Requests: requests,
		}).Do()
		require.NoError(t, err, "InsertText should succeed")

		// Verify the text was inserted
		updated, err := docsService.Documents.Get(created.DocumentId).Do()
		require.NoError(t, err, "Get should succeed")

		var textContent string
		for _, element := range updated.Body.Content {
			if element.Paragraph != nil && element.Paragraph.Elements != nil {
				for _, elem := range element.Paragraph.Elements {
					if elem.TextRun != nil {
						textContent += elem.TextRun.Content
					}
				}
			}
		}

		assert.Contains(t, textContent, "Hello Beautiful World", "Text should be inserted at correct position")
	})
}

func TestGdocsSimulatorReplaceAllText(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "gdocs-test-session-5"

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
	ctx := context.Background()
	docsService, err := docs.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(customClient))
	require.NoError(t, err, "Failed to create Docs service")

	// Create a test document and add some initial text
	doc := &docs.Document{Title: "Test Document for Replace"}
	created, err := docsService.Documents.Create(doc).Do()
	require.NoError(t, err, "Create should succeed")

	// Add initial text with multiple occurrences
	retrieved, err := docsService.Documents.Get(created.DocumentId).Do()
	require.NoError(t, err, "Get should succeed")
	endIndex := retrieved.Body.Content[len(retrieved.Body.Content)-1].EndIndex - 1

	initialText := "foo bar foo baz foo"
	_, err = docsService.Documents.BatchUpdate(created.DocumentId, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{
			{
				InsertText: &docs.InsertTextRequest{
					Location: &docs.Location{Index: endIndex},
					Text:     initialText,
				},
			},
		},
	}).Do()
	require.NoError(t, err, "Initial text insert should succeed")

	t.Run("ReplaceAllText", func(t *testing.T) {
		// Replace all occurrences of "foo" with "qux"
		requests := []*docs.Request{
			{
				ReplaceAllText: &docs.ReplaceAllTextRequest{
					ContainsText: &docs.SubstringMatchCriteria{
						Text:      "foo",
						MatchCase: true,
					},
					ReplaceText: "qux",
				},
			},
		}

		_, err = docsService.Documents.BatchUpdate(created.DocumentId, &docs.BatchUpdateDocumentRequest{
			Requests: requests,
		}).Do()
		require.NoError(t, err, "ReplaceAllText should succeed")

		// Verify all occurrences were replaced
		updated, err := docsService.Documents.Get(created.DocumentId).Do()
		require.NoError(t, err, "Get should succeed")

		var textContent string
		for _, element := range updated.Body.Content {
			if element.Paragraph != nil && element.Paragraph.Elements != nil {
				for _, elem := range element.Paragraph.Elements {
					if elem.TextRun != nil {
						textContent += elem.TextRun.Content
					}
				}
			}
		}

		assert.Contains(t, textContent, "qux bar qux baz qux", "All occurrences should be replaced")
		assert.NotContains(t, textContent, "foo", "Original text should not exist")
	})
}

func TestGdocsSimulatorDeleteRange(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "gdocs-test-session-6"

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
	ctx := context.Background()
	docsService, err := docs.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(customClient))
	require.NoError(t, err, "Failed to create Docs service")

	// Create a test document and add some initial text
	doc := &docs.Document{Title: "Test Document for Delete"}
	created, err := docsService.Documents.Create(doc).Do()
	require.NoError(t, err, "Create should succeed")

	// Add initial text
	retrieved, err := docsService.Documents.Get(created.DocumentId).Do()
	require.NoError(t, err, "Get should succeed")
	endIndex := retrieved.Body.Content[len(retrieved.Body.Content)-1].EndIndex - 1

	initialText := "Hello Beautiful World"
	_, err = docsService.Documents.BatchUpdate(created.DocumentId, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{
			{
				InsertText: &docs.InsertTextRequest{
					Location: &docs.Location{Index: endIndex},
					Text:     initialText,
				},
			},
		},
	}).Do()
	require.NoError(t, err, "Initial text insert should succeed")

	t.Run("DeleteContentRange", func(t *testing.T) {
		// Delete "Beautiful " (indices 6-16)
		requests := []*docs.Request{
			{
				DeleteContentRange: &docs.DeleteContentRangeRequest{
					Range: &docs.Range{
						StartIndex: 6,
						EndIndex:   16,
					},
				},
			},
		}

		_, err = docsService.Documents.BatchUpdate(created.DocumentId, &docs.BatchUpdateDocumentRequest{
			Requests: requests,
		}).Do()
		require.NoError(t, err, "DeleteContentRange should succeed")

		// Verify the text was deleted
		updated, err := docsService.Documents.Get(created.DocumentId).Do()
		require.NoError(t, err, "Get should succeed")

		var textContent string
		for _, element := range updated.Body.Content {
			if element.Paragraph != nil && element.Paragraph.Elements != nil {
				for _, elem := range element.Paragraph.Elements {
					if elem.TextRun != nil {
						textContent += elem.TextRun.Content
					}
				}
			}
		}

		assert.Contains(t, textContent, "Hello World", "Text should be deleted")
		assert.NotContains(t, textContent, "Beautiful", "Deleted text should not exist")
	})
}

func TestGdocsSimulatorEndToEnd(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "gdocs-test-session-7"

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
	ctx := context.Background()
	docsService, err := docs.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(customClient))
	require.NoError(t, err, "Failed to create Docs service")

	t.Run("CreateGetUpdateGet", func(t *testing.T) {
		// 1. Create a document
		doc := &docs.Document{Title: "E2E Test Document"}
		created, err := docsService.Documents.Create(doc).Do()
		require.NoError(t, err, "Create should succeed")
		docID := created.DocumentId

		// 2. Get the document and verify it's empty
		retrieved, err := docsService.Documents.Get(docID).Do()
		require.NoError(t, err, "Get should succeed")
		assert.Equal(t, docID, retrieved.DocumentId, "Document ID should match")
		assert.Equal(t, "E2E Test Document", retrieved.Title, "Title should match")

		// 3. Add some text
		endIndex := retrieved.Body.Content[len(retrieved.Body.Content)-1].EndIndex - 1
		_, err = docsService.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
			Requests: []*docs.Request{
				{
					InsertText: &docs.InsertTextRequest{
						Location: &docs.Location{Index: endIndex},
						Text:     "This is a test document with some content.",
					},
				},
			},
		}).Do()
		require.NoError(t, err, "Insert text should succeed")

		// 4. Get the document again and verify content
		updated, err := docsService.Documents.Get(docID).Do()
		require.NoError(t, err, "Get updated document should succeed")

		var textContent string
		for _, element := range updated.Body.Content {
			if element.Paragraph != nil && element.Paragraph.Elements != nil {
				for _, elem := range element.Paragraph.Elements {
					if elem.TextRun != nil {
						textContent += elem.TextRun.Content
					}
				}
			}
		}

		assert.Contains(t, textContent, "This is a test document with some content.", "Document should contain the added text")
	})
}

func TestGdocsSimulatorSessionIsolation(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorGdocs.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create two different sessions
	sessionID1 := "gdocs-test-session-isolation-1"
	sessionID2 := "gdocs-test-session-isolation-2"

	// Create custom HTTP clients for each session
	transport1 := &sessionHTTPTransport{sessionID: sessionID1}
	customClient1 := &http.Client{Transport: transport1}

	transport2 := &sessionHTTPTransport{sessionID: sessionID2}
	customClient2 := &http.Client{Transport: transport2}

	// Create Google Docs services for each session
	ctx := context.Background()
	docsService1, err := docs.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(customClient1))
	require.NoError(t, err, "Failed to create Docs service 1")

	docsService2, err := docs.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(customClient2))
	require.NoError(t, err, "Failed to create Docs service 2")

	t.Run("DocumentsAreIsolatedBetweenSessions", func(t *testing.T) {
		// Create a document in session 1
		doc1 := &docs.Document{Title: "Session 1 Document"}
		created1, err := docsService1.Documents.Create(doc1).Do()
		require.NoError(t, err, "Create in session 1 should succeed")

		// Create a document in session 2
		doc2 := &docs.Document{Title: "Session 2 Document"}
		created2, err := docsService2.Documents.Create(doc2).Do()
		require.NoError(t, err, "Create in session 2 should succeed")

		// Try to get session 1's document from session 2 (should fail)
		_, err = docsService2.Documents.Get(created1.DocumentId).Do()
		require.Error(t, err, "Should not be able to access session 1's document from session 2")

		// Try to get session 2's document from session 1 (should fail)
		_, err = docsService1.Documents.Get(created2.DocumentId).Do()
		require.Error(t, err, "Should not be able to access session 2's document from session 1")

		// Verify each session can access its own document
		retrieved1, err := docsService1.Documents.Get(created1.DocumentId).Do()
		require.NoError(t, err, "Session 1 should access its own document")
		assert.Equal(t, "Session 1 Document", retrieved1.Title, "Title should match")

		retrieved2, err := docsService2.Documents.Get(created2.DocumentId).Do()
		require.NoError(t, err, "Session 2 should access its own document")
		assert.Equal(t, "Session 2 Document", retrieved2.Title, "Title should match")
	})
}
