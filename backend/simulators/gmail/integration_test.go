package gmail_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/config"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/middleware"
	"github.com/recreate-run/nova-simulators/internal/session"
	"github.com/recreate-run/nova-simulators/internal/testutil"
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
		message := "From: sender@example.com\r\nTo: recipient@example.com\r\nSubject: Test Email\r\n\r\nThis is a test email body."
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
		message := "From: sender@example.com\r\nTo: recipient@example.com\r\nSubject: HTML Test\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<html><body><h1>Test</h1></body></html>"
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

func TestGmailSimulatorImportMessage(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "gmail-test-import-1"

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

	t.Run("ImportWithDefaultLabels", func(t *testing.T) {
		// Import a message (simulates receiving an email)
		message := "From: external@example.com\r\nTo: me@example.com\r\nSubject: Incoming Email\r\n\r\nThis is an incoming email."
		raw := base64.URLEncoding.EncodeToString([]byte(message))

		msg := &gmail.Message{
			Raw: raw,
		}

		// Import message
		imported, err := gmailService.Users.Messages.Import("me", msg).Do()

		// Assertions
		require.NoError(t, err, "Import should not return error")
		assert.NotNil(t, imported, "Should return imported message")
		assert.NotEmpty(t, imported.Id, "Message ID should not be empty")
		assert.Contains(t, imported.LabelIds, "INBOX", "Should have INBOX label")
		assert.Contains(t, imported.LabelIds, "UNREAD", "Should have UNREAD label")
	})

	t.Run("ImportWithCustomLabels", func(t *testing.T) {
		// Import a message with custom labels
		message := "From: admin@example.com\r\nTo: me@example.com\r\nSubject: Important Notice\r\n\r\nThis is important."
		raw := base64.URLEncoding.EncodeToString([]byte(message))

		msg := &gmail.Message{
			Raw:      raw,
			LabelIds: []string{"INBOX", "IMPORTANT"},
		}

		// Import message
		imported, err := gmailService.Users.Messages.Import("me", msg).Do()

		// Assertions
		require.NoError(t, err, "Import should not return error")
		assert.NotNil(t, imported, "Should return imported message")
		assert.Contains(t, imported.LabelIds, "INBOX", "Should have INBOX label")
		assert.Contains(t, imported.LabelIds, "IMPORTANT", "Should have IMPORTANT label")
		assert.NotContains(t, imported.LabelIds, "UNREAD", "Should not have default UNREAD label when custom labels provided")
	})
}

func TestGmailSimulatorSearch(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "gmail-test-search-1"

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

	// Setup: Import several test messages
	testMessages := []struct {
		from    string
		to      string
		subject string
		body    string
		labels  []string
	}{
		{
			from:    "alice@example.com",
			to:      "me@example.com",
			subject: "Meeting Tomorrow",
			body:    "Let's meet tomorrow at 3pm",
			labels:  []string{"INBOX", "UNREAD"},
		},
		{
			from:    "bob@company.com",
			to:      "me@example.com",
			subject: "Invoice #12345",
			body:    "Please find attached invoice",
			labels:  []string{"INBOX", "UNREAD"},
		},
		{
			from:    "alice@example.com",
			to:      "team@example.com",
			subject: "Project Update",
			body:    "The project is progressing well",
			labels:  []string{"INBOX"},
		},
		{
			from:    "notifications@github.com",
			to:      "me@example.com",
			subject: "PR Merged",
			body:    "Your pull request was merged",
			labels:  []string{"INBOX", "IMPORTANT"},
		},
	}

	for _, tm := range testMessages {
		message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", tm.from, tm.to, tm.subject, tm.body)
		raw := base64.URLEncoding.EncodeToString([]byte(message))
		msg := &gmail.Message{
			Raw:      raw,
			LabelIds: tm.labels,
		}
		_, err := gmailService.Users.Messages.Import("me", msg).Do()
		require.NoError(t, err, "Import should succeed")
	}

	t.Run("SearchByFrom", func(t *testing.T) {
		// Search for messages from alice
		response, err := gmailService.Users.Messages.List("me").Q("from:alice@example.com").Do()

		require.NoError(t, err, "Search should not return error")
		assert.GreaterOrEqual(t, len(response.Messages), 2, "Should find at least 2 messages from alice")
	})

	t.Run("SearchBySubject", func(t *testing.T) {
		// Search for messages with "Invoice" in subject
		response, err := gmailService.Users.Messages.List("me").Q("subject:Invoice").Do()

		require.NoError(t, err, "Search should not return error")
		assert.GreaterOrEqual(t, len(response.Messages), 1, "Should find at least 1 invoice message")
	})

	t.Run("SearchByUnread", func(t *testing.T) {
		// Search for unread messages
		response, err := gmailService.Users.Messages.List("me").Q("is:unread").Do()

		require.NoError(t, err, "Search should not return error")
		assert.GreaterOrEqual(t, len(response.Messages), 2, "Should find at least 2 unread messages")
	})

	t.Run("SearchByLabel", func(t *testing.T) {
		// Search for messages with IMPORTANT label
		response, err := gmailService.Users.Messages.List("me").Q("label:IMPORTANT").Do()

		require.NoError(t, err, "Search should not return error")
		assert.GreaterOrEqual(t, len(response.Messages), 1, "Should find at least 1 important message")
	})

	t.Run("SearchCombinedQuery", func(t *testing.T) {
		// Search for unread messages from alice
		response, err := gmailService.Users.Messages.List("me").Q("from:alice@example.com is:unread").Do()

		require.NoError(t, err, "Search should not return error")
		assert.GreaterOrEqual(t, len(response.Messages), 1, "Should find at least 1 unread message from alice")
	})
}

func TestGmailSimulatorAttachments(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "gmail-test-session-attachments"

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
	gmailService, err := gmail.NewService(ctx,
		option.WithoutAuthentication(),
		option.WithEndpoint(server.URL+"/"),
		option.WithHTTPClient(customClient),
	)
	require.NoError(t, err, "Failed to create Gmail service")

	t.Run("SendEmailWithSingleAttachment", func(t *testing.T) {
		// Create email with attachment (using multipart/mixed MIME)
		attachmentData := []byte("This is a test file content")
		attachmentDataB64 := base64.StdEncoding.EncodeToString(attachmentData)

		boundary := "boundary123"
		message := fmt.Sprintf("From: sender@example.com\r\n"+
			"To: recipient@example.com\r\n"+
			"Subject: Test with Attachment\r\n"+
			"Content-Type: multipart/mixed; boundary=\"%s\"\r\n"+
			"\r\n"+
			"--%s\r\n"+
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n"+
			"\r\n"+
			"Email body text\r\n"+
			"--%s\r\n"+
			"Content-Type: text/plain; name=\"test.txt\"\r\n"+
			"Content-Disposition: attachment; filename=\"test.txt\"\r\n"+
			"Content-Transfer-Encoding: base64\r\n"+
			"\r\n"+
			"%s\r\n"+
			"--%s--\r\n",
			boundary, boundary, boundary, attachmentDataB64, boundary)

		raw := base64.URLEncoding.EncodeToString([]byte(message))
		msg := &gmail.Message{Raw: raw}

		// Send message
		sent, err := gmailService.Users.Messages.Send("me", msg).Do()

		// Assertions
		require.NoError(t, err, "Send should not return error")
		assert.NotEmpty(t, sent.Id, "Message ID should not be empty")

		// Get the message with full format to check attachments
		retrieved, err := gmailService.Users.Messages.Get("me", sent.Id).Format("full").Do()
		require.NoError(t, err, "Get should not return error")

		// Verify message has attachment parts
		assert.NotEmpty(t, retrieved.Payload.Parts, "Should have parts")

		// Find attachment part
		var attachmentPart *gmail.MessagePart
		for _, part := range retrieved.Payload.Parts {
			if part.Filename == "test.txt" {
				attachmentPart = part
				break
			}
		}

		require.NotNil(t, attachmentPart, "Should have attachment part")
		assert.Equal(t, "test.txt", attachmentPart.Filename, "Filename should match")
		assert.NotEmpty(t, attachmentPart.Body.AttachmentId, "Should have attachment ID")
		assert.Positive(t, attachmentPart.Body.Size, "Size should be greater than 0")
	})

	t.Run("SendEmailWithMultipleAttachments", func(t *testing.T) {
		// Create email with multiple attachments
		attachment1Data := []byte("First file content")
		attachment1DataB64 := base64.StdEncoding.EncodeToString(attachment1Data)

		attachment2Data := []byte("Second file content with more text")
		attachment2DataB64 := base64.StdEncoding.EncodeToString(attachment2Data)

		boundary := "boundary456"
		message := fmt.Sprintf("From: sender@example.com\r\n"+
			"To: recipient@example.com\r\n"+
			"Subject: Test with Multiple Attachments\r\n"+
			"Content-Type: multipart/mixed; boundary=\"%s\"\r\n"+
			"\r\n"+
			"--%s\r\n"+
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n"+
			"\r\n"+
			"Email body\r\n"+
			"--%s\r\n"+
			"Content-Type: text/plain; name=\"file1.txt\"\r\n"+
			"Content-Disposition: attachment; filename=\"file1.txt\"\r\n"+
			"Content-Transfer-Encoding: base64\r\n"+
			"\r\n"+
			"%s\r\n"+
			"--%s\r\n"+
			"Content-Type: application/pdf; name=\"file2.pdf\"\r\n"+
			"Content-Disposition: attachment; filename=\"file2.pdf\"\r\n"+
			"Content-Transfer-Encoding: base64\r\n"+
			"\r\n"+
			"%s\r\n"+
			"--%s--\r\n",
			boundary, boundary, boundary, attachment1DataB64, boundary, attachment2DataB64, boundary)

		raw := base64.URLEncoding.EncodeToString([]byte(message))
		msg := &gmail.Message{Raw: raw}

		// Send message
		sent, err := gmailService.Users.Messages.Send("me", msg).Do()
		require.NoError(t, err, "Send should not return error")

		// Get the message
		retrieved, err := gmailService.Users.Messages.Get("me", sent.Id).Format("full").Do()
		require.NoError(t, err, "Get should not return error")

		// Count attachment parts
		attachmentCount := 0
		for _, part := range retrieved.Payload.Parts {
			if part.Filename != "" {
				attachmentCount++
			}
		}

		assert.Equal(t, 2, attachmentCount, "Should have 2 attachments")
	})

	t.Run("DownloadAttachment", func(t *testing.T) {
		// Create and send email with attachment
		attachmentData := []byte("Download test content")
		attachmentDataB64 := base64.StdEncoding.EncodeToString(attachmentData)

		boundary := "boundary789"
		message := fmt.Sprintf("From: sender@example.com\r\n"+
			"To: recipient@example.com\r\n"+
			"Subject: Download Test\r\n"+
			"Content-Type: multipart/mixed; boundary=\"%s\"\r\n"+
			"\r\n"+
			"--%s\r\n"+
			"Content-Type: text/plain\r\n"+
			"\r\n"+
			"Body\r\n"+
			"--%s\r\n"+
			"Content-Type: text/plain; name=\"download.txt\"\r\n"+
			"Content-Disposition: attachment; filename=\"download.txt\"\r\n"+
			"Content-Transfer-Encoding: base64\r\n"+
			"\r\n"+
			"%s\r\n"+
			"--%s--\r\n",
			boundary, boundary, boundary, attachmentDataB64, boundary)

		raw := base64.URLEncoding.EncodeToString([]byte(message))
		msg := &gmail.Message{Raw: raw}

		sent, err := gmailService.Users.Messages.Send("me", msg).Do()
		require.NoError(t, err, "Send should succeed")

		// Get message to find attachment ID
		retrieved, err := gmailService.Users.Messages.Get("me", sent.Id).Format("full").Do()
		require.NoError(t, err, "Get should succeed")

		// Find attachment
		var attachmentID string
		for _, part := range retrieved.Payload.Parts {
			if part.Filename == "download.txt" {
				attachmentID = part.Body.AttachmentId
				break
			}
		}

		require.NotEmpty(t, attachmentID, "Should have attachment ID")

		// Download attachment
		attachment, err := gmailService.Users.Messages.Attachments.Get("me", sent.Id, attachmentID).Do()
		require.NoError(t, err, "Download should succeed")
		assert.NotEmpty(t, attachment.Data, "Attachment data should not be empty")

		// Decode and verify content
		decodedData, err := base64.URLEncoding.DecodeString(attachment.Data)
		require.NoError(t, err, "Should decode attachment data")
		assert.Equal(t, attachmentData, decodedData, "Attachment content should match")
	})
}

func TestGmailSimulatorPagination(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "gmail-test-session-pagination"

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
	gmailService, err := gmail.NewService(ctx,
		option.WithoutAuthentication(),
		option.WithEndpoint(server.URL+"/"),
		option.WithHTTPClient(customClient),
	)
	require.NoError(t, err, "Failed to create Gmail service")

	// Seed 10 test messages
	for i := 1; i <= 10; i++ {
		message := fmt.Sprintf("From: sender%d@example.com\r\nTo: recipient@example.com\r\nSubject: Message %d\r\n\r\nBody %d", i, i, i)
		raw := base64.URLEncoding.EncodeToString([]byte(message))
		msg := &gmail.Message{Raw: raw}
		_, err := gmailService.Users.Messages.Send("me", msg).Do()
		require.NoError(t, err, "Send should succeed")
	}

	t.Run("ListWithMaxResults", func(t *testing.T) {
		// List with maxResults=3
		response, err := gmailService.Users.Messages.List("me").MaxResults(3).Do()

		// Assertions
		require.NoError(t, err, "List should succeed")
		assert.Len(t, response.Messages, 3, "Should return 3 messages")
		assert.NotEmpty(t, response.NextPageToken, "Should have next page token")
	})

	t.Run("PaginateThroughAllPages", func(t *testing.T) {
		allMessages := make([]string, 0)
		pageToken := ""
		pageCount := 0

		for {
			var response *gmail.ListMessagesResponse
			var err error

			if pageToken == "" {
				response, err = gmailService.Users.Messages.List("me").MaxResults(3).Do()
			} else {
				response, err = gmailService.Users.Messages.List("me").MaxResults(3).PageToken(pageToken).Do()
			}

			require.NoError(t, err, "List should succeed")
			pageCount++

			for _, msg := range response.Messages {
				allMessages = append(allMessages, msg.Id)
			}

			if response.NextPageToken == "" {
				break
			}
			pageToken = response.NextPageToken
		}

		// Verify we got all messages
		assert.GreaterOrEqual(t, len(allMessages), 10, "Should retrieve at least 10 messages")
		assert.GreaterOrEqual(t, pageCount, 4, "Should have made at least 4 page requests")
	})

	t.Run("SecondPageHasDifferentMessages", func(t *testing.T) {
		// Get first page
		page1, err := gmailService.Users.Messages.List("me").MaxResults(3).Do()
		require.NoError(t, err, "First page should succeed")
		require.NotEmpty(t, page1.NextPageToken, "Should have next page token")

		// Get second page
		page2, err := gmailService.Users.Messages.List("me").MaxResults(3).PageToken(page1.NextPageToken).Do()
		require.NoError(t, err, "Second page should succeed")

		// Verify messages are different
		page1IDs := make(map[string]bool)
		for _, msg := range page1.Messages {
			page1IDs[msg.Id] = true
		}

		for _, msg := range page2.Messages {
			assert.False(t, page1IDs[msg.Id], "Page 2 should have different messages than page 1")
		}
	})

	t.Run("LastPageHasNoNextToken", func(t *testing.T) {
		// Paginate to the last page
		pageToken := ""
		var lastResponse *gmail.ListMessagesResponse

		for {
			var response *gmail.ListMessagesResponse
			var err error

			if pageToken == "" {
				response, err = gmailService.Users.Messages.List("me").MaxResults(3).Do()
			} else {
				response, err = gmailService.Users.Messages.List("me").MaxResults(3).PageToken(pageToken).Do()
			}

			require.NoError(t, err, "List should succeed")
			lastResponse = response

			if response.NextPageToken == "" {
				break
			}
			pageToken = response.NextPageToken
		}

		// Last page should have no next token
		assert.Empty(t, lastResponse.NextPageToken, "Last page should have no next token")
	})
}

func TestGmailSimulatorTimeout(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Handler factory: Creates Gmail handler with middleware
	makeHandler := func(configManager *config.Manager) http.Handler {
		return session.Middleware(
			middleware.RateLimit(configManager, "gmail")(
				middleware.Timeout(configManager, "gmail")(
					simulatorGmail.NewHandler(queries))))
	}

	// Request factory: Creates Gmail send message request
	makeRequest := func(sessionID string) *http.Request {
		message := "From: sender@example.com\r\nTo: recipient@example.com\r\nSubject: Timeout Test\r\n\r\nTesting timeout."
		raw := base64.URLEncoding.EncodeToString([]byte(message))
		body := fmt.Sprintf(`{"raw":%q}`, raw)

		req := httptest.NewRequest(http.MethodPost, "/gmail/v1/users/me/messages/send", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-ID", sessionID)
		return req
	}

	// Run common timeout test
	testutil.TestMiddlewareTimeout(t, makeHandler, makeRequest, "gmail")
}

func TestGmailSimulatorRateLimit(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Handler factory: Creates Gmail handler with middleware
	makeHandler := func(configManager *config.Manager) http.Handler {
		return session.Middleware(
			middleware.RateLimit(configManager, "gmail")(
				middleware.Timeout(configManager, "gmail")(
					simulatorGmail.NewHandler(queries))))
	}

	// Request factory: Creates Gmail send message request
	makeRequest := func(sessionID string) *http.Request {
		message := "From: sender@example.com\r\nTo: recipient@example.com\r\nSubject: Rate Limit Test\r\n\r\nTesting rate limits."
		raw := base64.URLEncoding.EncodeToString([]byte(message))
		body := fmt.Sprintf(`{"raw":%q}`, raw)

		req := httptest.NewRequest(http.MethodPost, "/gmail/v1/users/me/messages/send", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-ID", sessionID)
		return req
	}

	t.Run("EnforcesPerMinuteRateLimit", func(t *testing.T) {
		testutil.TestMiddlewareRateLimit(t, makeHandler, makeRequest, "gmail")
	})

	t.Run("PerSessionRateLimitIsolation", func(t *testing.T) {
		testutil.TestMiddlewareRateLimitIsolation(t, makeHandler, makeRequest, "gmail")
	})
}
