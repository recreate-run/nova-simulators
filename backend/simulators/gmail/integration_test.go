package gmail_test

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
	simulatorGmail "github.com/recreate-run/nova-simulators/simulators/gmail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/gmail/v1"
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

func TestGmailSimulatorSendMessage(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "gmail-test-session-1"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorGmail.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client that adds session header
	transport := &sessionHTTPTransport{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create Gmail service pointing to test server with custom client
	ctx := context.Background()
	gmailService, err := gmail.NewService(ctx,
		option.WithoutAuthentication(),
		option.WithEndpoint(server.URL+"/"),
		option.WithHTTPClient(customClient),
	)
	require.NoError(t, err, "Failed to create Gmail service")

	t.Run("SendPlainTextEmail", func(t *testing.T) {
		// Create email message
		message := fmt.Sprintf("From: sender@example.com\r\nTo: recipient@example.com\r\nSubject: Test Email\r\n\r\nThis is a test email body.")
		raw := base64.URLEncoding.EncodeToString([]byte(message))

		msg := &gmail.Message{
			Raw: raw,
		}

		// Send message
		sent, err := gmailService.Users.Messages.Send("me", msg).Do()

		// Assertions
		require.NoError(t, err, "Send should not return error")
		assert.NotNil(t, sent, "Should return sent message")
		assert.NotEmpty(t, sent.Id, "Message ID should not be empty")
		assert.NotEmpty(t, sent.ThreadId, "Thread ID should not be empty")
		assert.Contains(t, sent.LabelIds, "SENT", "Should have SENT label")
	})

	t.Run("SendHTMLEmail", func(t *testing.T) {
		// Create HTML email message
		message := fmt.Sprintf("From: sender@example.com\r\nTo: recipient@example.com\r\nSubject: HTML Test\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<html><body><h1>Test</h1></body></html>")
		raw := base64.URLEncoding.EncodeToString([]byte(message))

		msg := &gmail.Message{
			Raw: raw,
		}

		// Send message
		sent, err := gmailService.Users.Messages.Send("me", msg).Do()

		// Assertions
		require.NoError(t, err, "Send should not return error")
		assert.NotNil(t, sent, "Should return sent message")
		assert.NotEmpty(t, sent.Id, "Message ID should not be empty")
	})
}

func TestGmailSimulatorListMessages(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "gmail-test-session-2"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorGmail.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client that adds session header
	transport := &sessionHTTPTransport{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create Gmail service
	ctx := context.Background()
	gmailService, err := gmail.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(customClient))
	require.NoError(t, err, "Failed to create Gmail service")

	// Send a few test messages first
	for i := 1; i <= 3; i++ {
		message := fmt.Sprintf("From: sender%d@example.com\r\nTo: recipient@example.com\r\nSubject: Test %d\r\n\r\nMessage body %d", i, i, i)
		raw := base64.URLEncoding.EncodeToString([]byte(message))
		msg := &gmail.Message{Raw: raw}
		_, err := gmailService.Users.Messages.Send("me", msg).Do()
		require.NoError(t, err, "Send should succeed")
	}

	t.Run("ListAllMessages", func(t *testing.T) {
		// List messages
		response, err := gmailService.Users.Messages.List("me").Do()

		// Assertions
		require.NoError(t, err, "List should not return error")
		assert.NotNil(t, response, "Should return response")
		assert.GreaterOrEqual(t, len(response.Messages), 3, "Should have at least 3 messages")
		assert.Equal(t, len(response.Messages), int(response.ResultSizeEstimate), "ResultSizeEstimate should match")
	})

	t.Run("ListMessagesWithLimit", func(t *testing.T) {
		// List with limit
		response, err := gmailService.Users.Messages.List("me").MaxResults(2).Do()

		// Assertions
		require.NoError(t, err, "List should not return error")
		assert.LessOrEqual(t, len(response.Messages), 2, "Should respect max results")
	})
}

func TestGmailSimulatorGetMessage(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "gmail-test-session-3"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorGmail.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client that adds session header
	transport := &sessionHTTPTransport{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create Gmail service
	ctx := context.Background()
	gmailService, err := gmail.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(customClient))
	require.NoError(t, err, "Failed to create Gmail service")

	// Send a test message
	testSubject := "Get Message Test"
	testBody := "This is the message body for retrieval test"
	message := fmt.Sprintf("From: sender@example.com\r\nTo: recipient@example.com\r\nSubject: %s\r\n\r\n%s", testSubject, testBody)
	raw := base64.URLEncoding.EncodeToString([]byte(message))
	msg := &gmail.Message{Raw: raw}
	sent, err := gmailService.Users.Messages.Send("me", msg).Do()
	require.NoError(t, err, "Send should succeed")

	t.Run("GetMessageFull", func(t *testing.T) {
		// Get the message
		retrieved, err := gmailService.Users.Messages.Get("me", sent.Id).Format("full").Do()

		// Assertions
		require.NoError(t, err, "Get should not return error")
		assert.NotNil(t, retrieved, "Should return message")
		assert.Equal(t, sent.Id, retrieved.Id, "Message ID should match")
		assert.NotEmpty(t, retrieved.Payload, "Should have payload")
		assert.NotEmpty(t, retrieved.Payload.Headers, "Should have headers")

		// Verify headers
		headerMap := make(map[string]string)
		for _, header := range retrieved.Payload.Headers {
			headerMap[header.Name] = header.Value
		}
		assert.Equal(t, "sender@example.com", headerMap["From"], "From header should match")
		assert.Equal(t, "recipient@example.com", headerMap["To"], "To header should match")
		assert.Equal(t, testSubject, headerMap["Subject"], "Subject header should match")

		// Verify body parts
		assert.NotEmpty(t, retrieved.Payload.Parts, "Should have message parts")

		// Decode body
		if len(retrieved.Payload.Parts) > 0 {
			bodyData := retrieved.Payload.Parts[0].Body.Data
			decodedBody, err := base64.URLEncoding.DecodeString(bodyData)
			require.NoError(t, err, "Should decode body")
			assert.Contains(t, string(decodedBody), testBody, "Body should contain test text")
		}
	})

	t.Run("GetNonExistentMessage", func(t *testing.T) {
		// Try to get a non-existent message
		_, err := gmailService.Users.Messages.Get("me", "nonexistent").Do()

		// Assertions
		assert.Error(t, err, "Should return error for non-existent message")
	})
}

func TestGmailSimulatorEndToEnd(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "gmail-test-session-4"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorGmail.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client that adds session header
	transport := &sessionHTTPTransport{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create Gmail service
	ctx := context.Background()
	gmailService, err := gmail.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(customClient))
	require.NoError(t, err, "Failed to create Gmail service")

	// End-to-end workflow: Send -> List -> Get
	t.Run("SendListGet", func(t *testing.T) {
		// 1. Send a message
		testSubject := "E2E Test"
		message := fmt.Sprintf("From: alice@example.com\r\nTo: bob@example.com\r\nSubject: %s\r\n\r\nHello Bob!", testSubject)
		raw := base64.URLEncoding.EncodeToString([]byte(message))
		msg := &gmail.Message{Raw: raw}

		sent, err := gmailService.Users.Messages.Send("me", msg).Do()
		require.NoError(t, err, "Send should succeed")
		sentID := sent.Id

		// 2. List messages and find our message
		listResp, err := gmailService.Users.Messages.List("me").Do()
		require.NoError(t, err, "List should succeed")

		found := false
		for _, m := range listResp.Messages {
			if m.Id == sentID {
				found = true
				break
			}
		}
		assert.True(t, found, "Sent message should appear in list")

		// 3. Get the full message
		retrieved, err := gmailService.Users.Messages.Get("me", sentID).Format("full").Do()
		require.NoError(t, err, "Get should succeed")

		// Verify it's the same message
		assert.Equal(t, sentID, retrieved.Id, "Retrieved message ID should match")

		headerMap := make(map[string]string)
		for _, header := range retrieved.Payload.Headers {
			headerMap[header.Name] = header.Value
		}
		assert.Equal(t, testSubject, headerMap["Subject"], "Subject should match")
		assert.Equal(t, "alice@example.com", headerMap["From"], "From should match")
		assert.Equal(t, "bob@example.com", headerMap["To"], "To should match")
	})
}
