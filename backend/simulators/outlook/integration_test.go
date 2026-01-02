package outlook_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
	simulatorOutlook "github.com/recreate-run/nova-simulators/simulators/outlook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// Test structures matching Microsoft Graph API
type EmailAddress struct {
	Address string `json:"address"`
}

type Recipient struct {
	EmailAddress *EmailAddress `json:"emailAddress"`
}

type ItemBody struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

type Message struct {
	ID               string       `json:"id,omitempty"`
	Subject          string       `json:"subject,omitempty"`
	Body             *ItemBody    `json:"body,omitempty"`
	From             *Recipient   `json:"from,omitempty"`
	ToRecipients     []*Recipient `json:"toRecipients,omitempty"`
	IsRead           bool         `json:"isRead"`
	ReceivedDateTime string       `json:"receivedDateTime,omitempty"`
}

type SendMailRequest struct {
	Message         *Message `json:"message"`
	SaveToSentItems bool     `json:"saveToSentItems,omitempty"`
}

type MessageListResponse struct {
	Value        []*Message `json:"value"`
	ODataContext string     `json:"@odata.context"`
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

func TestOutlookSimulatorSendMail(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "outlook-test-session-1"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorOutlook.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	t.Run("SendPlainTextEmail", func(t *testing.T) {
		// Create email message
		message := &Message{
			Subject: "Test Email",
			Body: &ItemBody{
				ContentType: "text",
				Content:     "This is a test email body.",
			},
			ToRecipients: []*Recipient{
				{
					EmailAddress: &EmailAddress{
						Address: "recipient@example.com",
					},
				},
			},
		}

		reqBody := &SendMailRequest{
			Message: message,
		}

		jsonBody, err := json.Marshal(reqBody)
		require.NoError(t, err)

		// Send request
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/v1.0/me/sendMail", bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-ID", sessionID)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Assertions
		assert.Equal(t, http.StatusAccepted, resp.StatusCode, "Should return 202 Accepted")
	})

	t.Run("SendHTMLEmail", func(t *testing.T) {
		// Create HTML email message
		message := &Message{
			Subject: "HTML Test",
			Body: &ItemBody{
				ContentType: "html",
				Content:     "<html><body><h1>Test</h1></body></html>",
			},
			ToRecipients: []*Recipient{
				{
					EmailAddress: &EmailAddress{
						Address: "recipient@example.com",
					},
				},
			},
		}

		reqBody := &SendMailRequest{
			Message: message,
		}

		jsonBody, err := json.Marshal(reqBody)
		require.NoError(t, err)

		// Send request
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/v1.0/me/sendMail", bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-ID", sessionID)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Assertions
		assert.Equal(t, http.StatusAccepted, resp.StatusCode, "Should return 202 Accepted")
	})
}

func TestOutlookSimulatorListMessages(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "outlook-test-session-2"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorOutlook.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Send a few test messages first
	for i := 1; i <= 3; i++ {
		message := &Message{
			Subject: fmt.Sprintf("Test %d", i),
			Body: &ItemBody{
				ContentType: "text",
				Content:     fmt.Sprintf("Message body %d", i),
			},
			ToRecipients: []*Recipient{
				{
					EmailAddress: &EmailAddress{
						Address: "recipient@example.com",
					},
				},
			},
		}

		reqBody := &SendMailRequest{Message: message}
		jsonBody, _ := json.Marshal(reqBody)

		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/v1.0/me/sendMail", bytes.NewBuffer(jsonBody))
	require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-ID", sessionID)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
	}

	t.Run("ListAllMessages", func(t *testing.T) {
		// List messages
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/v1.0/me/messages", http.NoBody)
		require.NoError(t, err)
		req.Header.Set("X-Session-ID", sessionID)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Assertions
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response MessageListResponse
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, len(response.Value), 3, "Should have at least 3 messages")
	})

	t.Run("ListMessagesWithLimit", func(t *testing.T) {
		// List with limit
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/v1.0/me/messages?$top=2", http.NoBody)
		require.NoError(t, err)
		req.Header.Set("X-Session-ID", sessionID)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Assertions
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response MessageListResponse
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		assert.LessOrEqual(t, len(response.Value), 2, "Should respect max results")
	})
}

func TestOutlookSimulatorGetMessage(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "outlook-test-session-3"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorOutlook.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Send a test message
	testSubject := "Get Message Test"
	testBody := "This is the message body for retrieval test"

	message := &Message{
		Subject: testSubject,
		Body: &ItemBody{
			ContentType: "text",
			Content:     testBody,
		},
		ToRecipients: []*Recipient{
			{
				EmailAddress: &EmailAddress{
					Address: "recipient@example.com",
				},
			},
		},
	}

	reqBody := &SendMailRequest{Message: message}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/v1.0/me/sendMail", bytes.NewBuffer(jsonBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-ID", sessionID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	// List messages to get the ID
	listReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/v1.0/me/messages", http.NoBody)
	require.NoError(t, err)
	listReq.Header.Set("X-Session-ID", sessionID)

	listResp, err := http.DefaultClient.Do(listReq)
	require.NoError(t, err)
	defer listResp.Body.Close()

	var listResponse MessageListResponse
	err = json.NewDecoder(listResp.Body).Decode(&listResponse)
	require.NoError(t, err)
	require.NotEmpty(t, listResponse.Value, "Should have at least one message")

	messageID := listResponse.Value[0].ID

	t.Run("GetMessageFull", func(t *testing.T) {
		// Get the message
		getReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/v1.0/me/messages/"+messageID, http.NoBody)
		require.NoError(t, err)
		getReq.Header.Set("X-Session-ID", sessionID)

		getResp, err := http.DefaultClient.Do(getReq)
		require.NoError(t, err)
		defer getResp.Body.Close()

		// Assertions
		assert.Equal(t, http.StatusOK, getResp.StatusCode)

		var retrieved Message
		err = json.NewDecoder(getResp.Body).Decode(&retrieved)
		require.NoError(t, err)

		assert.Equal(t, messageID, retrieved.ID, "Message ID should match")
		assert.Equal(t, testSubject, retrieved.Subject, "Subject should match")
		assert.Equal(t, testBody, retrieved.Body.Content, "Body should match")
	})

	t.Run("GetNonExistentMessage", func(t *testing.T) {
		// Try to get a non-existent message
		getReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/v1.0/me/messages/nonexistent", http.NoBody)
		require.NoError(t, err)
		getReq.Header.Set("X-Session-ID", sessionID)

		getResp, err := http.DefaultClient.Do(getReq)
		require.NoError(t, err)
		defer getResp.Body.Close()

		// Assertions
		assert.Equal(t, http.StatusNotFound, getResp.StatusCode, "Should return 404 for non-existent message")
	})
}

func TestOutlookSimulatorMarkAsRead(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "outlook-test-session-4"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorOutlook.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Send a test message
	message := &Message{
		Subject: "Mark as Read Test",
		Body: &ItemBody{
			ContentType: "text",
			Content:     "Test message",
		},
		ToRecipients: []*Recipient{
			{
				EmailAddress: &EmailAddress{
					Address: "recipient@example.com",
				},
			},
		},
	}

	reqBody := &SendMailRequest{Message: message}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/v1.0/me/sendMail", bytes.NewBuffer(jsonBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-ID", sessionID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	// Get message ID
	listReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/v1.0/me/messages", http.NoBody)
	require.NoError(t, err)
	listReq.Header.Set("X-Session-ID", sessionID)

	listResp, err := http.DefaultClient.Do(listReq)
	require.NoError(t, err)
	defer listResp.Body.Close()

	var listResponse MessageListResponse
	err = json.NewDecoder(listResp.Body).Decode(&listResponse)
	require.NoError(t, err)
	messageID := listResponse.Value[0].ID

	t.Run("MarkMessageAsRead", func(t *testing.T) {
		// Mark as read
		updateMessage := &Message{
			IsRead: true,
		}

		updateJSON, _ := json.Marshal(updateMessage)

		patchReq, err := http.NewRequestWithContext(context.Background(), http.MethodPatch, server.URL+"/v1.0/me/messages/"+messageID, bytes.NewBuffer(updateJSON))
		require.NoError(t, err)
		patchReq.Header.Set("Content-Type", "application/json")
		patchReq.Header.Set("X-Session-ID", sessionID)

		patchResp, err := http.DefaultClient.Do(patchReq)
		require.NoError(t, err)
		defer patchResp.Body.Close()

		// Assertions
		assert.Equal(t, http.StatusOK, patchResp.StatusCode)

		var updated Message
		err = json.NewDecoder(patchResp.Body).Decode(&updated)
		require.NoError(t, err)

		assert.True(t, updated.IsRead, "Message should be marked as read")
	})

	t.Run("MarkMessageAsUnread", func(t *testing.T) {
		// Mark as unread
		updateMessage := &Message{
			IsRead: false,
		}

		updateJSON, _ := json.Marshal(updateMessage)

		patchReq, err := http.NewRequestWithContext(context.Background(), http.MethodPatch, server.URL+"/v1.0/me/messages/"+messageID, bytes.NewBuffer(updateJSON))
		require.NoError(t, err)
		patchReq.Header.Set("Content-Type", "application/json")
		patchReq.Header.Set("X-Session-ID", sessionID)

		patchResp, err := http.DefaultClient.Do(patchReq)
		require.NoError(t, err)
		defer patchResp.Body.Close()

		// Assertions
		assert.Equal(t, http.StatusOK, patchResp.StatusCode)

		var updated Message
		err = json.NewDecoder(patchResp.Body).Decode(&updated)
		require.NoError(t, err)

		assert.False(t, updated.IsRead, "Message should be marked as unread")
	})
}

func TestOutlookSimulatorEndToEnd(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "outlook-test-session-5"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorOutlook.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// End-to-end workflow: Send -> List -> Get -> Update
	t.Run("SendListGetUpdate", func(t *testing.T) {
		// 1. Send a message
		testSubject := "E2E Test"
		message := &Message{
			Subject: testSubject,
			Body: &ItemBody{
				ContentType: "text",
				Content:     "Hello World!",
			},
			From: &Recipient{
				EmailAddress: &EmailAddress{
					Address: "alice@example.com",
				},
			},
			ToRecipients: []*Recipient{
				{
					EmailAddress: &EmailAddress{
						Address: "bob@example.com",
					},
				},
			},
		}

		reqBody := &SendMailRequest{Message: message}
		jsonBody, _ := json.Marshal(reqBody)

		sendReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/v1.0/me/sendMail", bytes.NewBuffer(jsonBody))
	require.NoError(t, err)
		sendReq.Header.Set("Content-Type", "application/json")
		sendReq.Header.Set("X-Session-ID", sessionID)

		sendResp, err := http.DefaultClient.Do(sendReq)
		require.NoError(t, err, "Send should succeed")
		_ = sendResp.Body.Close()
		assert.Equal(t, http.StatusAccepted, sendResp.StatusCode)

		// 2. List messages and find our message
		listReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/v1.0/me/messages", http.NoBody)
	require.NoError(t, err)
		listReq.Header.Set("X-Session-ID", sessionID)

		listResp, err := http.DefaultClient.Do(listReq)
		require.NoError(t, err, "List should succeed")
		defer listResp.Body.Close()

		var listResponse MessageListResponse
		err = json.NewDecoder(listResp.Body).Decode(&listResponse)
		require.NoError(t, err)

		require.NotEmpty(t, listResponse.Value, "Should have at least one message")

		// Find our message by subject
		var sentMessage *Message
		for _, m := range listResponse.Value {
			if m.Subject == testSubject {
				sentMessage = m
				break
			}
		}
		require.NotNil(t, sentMessage, "Should find sent message in list")

		// 3. Get the full message
		messageID := sentMessage.ID
		getReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/v1.0/me/messages/"+messageID, http.NoBody)
		require.NoError(t, err)
		getReq.Header.Set("X-Session-ID", sessionID)

		getResp, err := http.DefaultClient.Do(getReq)
		require.NoError(t, err, "Get should succeed")
		defer getResp.Body.Close()

		var retrieved Message
		err = json.NewDecoder(getResp.Body).Decode(&retrieved)
		require.NoError(t, err)

		// Verify it's the same message
		assert.Equal(t, messageID, retrieved.ID, "Retrieved message ID should match")
		assert.Equal(t, testSubject, retrieved.Subject, "Subject should match")
		assert.Equal(t, "Hello World!", retrieved.Body.Content, "Body should match")
		assert.Equal(t, "alice@example.com", retrieved.From.EmailAddress.Address, "From should match")

		// 4. Update the message (mark as read)
		updateMsg := &Message{IsRead: true}
		updateJSON, _ := json.Marshal(updateMsg)

		patchReq, err := http.NewRequestWithContext(context.Background(), http.MethodPatch, server.URL+"/v1.0/me/messages/"+messageID, bytes.NewBuffer(updateJSON))
		require.NoError(t, err)
		patchReq.Header.Set("Content-Type", "application/json")
		patchReq.Header.Set("X-Session-ID", sessionID)

		patchResp, err := http.DefaultClient.Do(patchReq)
		require.NoError(t, err, "Update should succeed")
		defer patchResp.Body.Close()

		var updated Message
		err = json.NewDecoder(patchResp.Body).Decode(&updated)
		require.NoError(t, err)

		assert.True(t, updated.IsRead, "Message should be marked as read")
	})
}
