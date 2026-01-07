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
	"github.com/recreate-run/nova-simulators/internal/config"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
	simulatorGmail "github.com/recreate-run/nova-simulators/simulators/gmail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	_ "modernc.org/sqlite"
)

// sessionHTTPTransportSeed wraps http.RoundTripper and adds session header to all requests
type sessionHTTPTransportSeed struct {
	sessionID string
}

func (t *sessionHTTPTransportSeed) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-Session-ID", t.sessionID)
	return http.DefaultTransport.RoundTrip(req)
}

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

// TestGmailInitialStateSeed demonstrates seeding arbitrary initial state for Gmail simulator
func TestGmailInitialStateSeed(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDBForSeed(t)
	ctx := context.Background()

	// Setup: Create a new session
	sessionID := "gmail-seed-test-session"
	err := queries.CreateSession(ctx, sessionID)
	require.NoError(t, err, "Failed to create session")

	// Setup: Start simulator server with session middleware
	cfg := &config.GmailConfig{
		Timeout: config.TimeoutConfig{
			MinMs: 0,
			MaxMs: 0,
		},
		RateLimit: config.RateLimitConfig{
			PerMinute: 1000,
			PerDay:    10000,
		},
	}
	handler := session.Middleware(simulatorGmail.NewHandler(queries, cfg))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client that adds session header
	transport := &sessionHTTPTransportSeed{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create Gmail service pointing to test server with custom client
	gmailService, err := gmail.NewService(ctx,
		option.WithoutAuthentication(),
		option.WithEndpoint(server.URL+"/"),
		option.WithHTTPClient(customClient),
	)
	require.NoError(t, err, "Failed to create Gmail service")

	// Seed: Send multiple emails with varying subjects, bodies, and recipients
	emails := []struct {
		From    string
		To      string
		Subject string
		Body    string
	}{
		{
			From:    "alice@example.com",
			To:      "bob@example.com",
			Subject: "Welcome to the team!",
			Body:    "Hi Bob, welcome aboard! We're excited to have you.",
		},
		{
			From:    "charlie@example.com",
			To:      "alice@example.com",
			Subject: "Project Update",
			Body:    "Hey Alice, the project is progressing well. Let's sync tomorrow.",
		},
		{
			From:    "notifications@github.com",
			To:      "alice@example.com",
			Subject: "Pull Request #123 merged",
			Body:    "Your pull request has been successfully merged into main.",
		},
		{
			From:    "bob@example.com",
			To:      "charlie@example.com",
			Subject: "Meeting Notes",
			Body:    "Here are the notes from today's standup meeting.",
		},
		{
			From:    "david@example.com",
			To:      "alice@example.com",
			Subject: "Invoice #4567",
			Body:    "Please find attached invoice for services rendered.",
		},
	}

	var sentIDs []string

	t.Run("SendInitialEmails", func(t *testing.T) {
		for i, email := range emails {
			// Create email message in RFC 2822 format
			message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
				email.From, email.To, email.Subject, email.Body)
			raw := base64.URLEncoding.EncodeToString([]byte(message))

			msg := &gmail.Message{
				Raw: raw,
			}

			// Send message
			sent, err := gmailService.Users.Messages.Send("me", msg).Do()
			require.NoError(t, err, "Failed to send email %d: %s", i+1, email.Subject)
			assert.NotEmpty(t, sent.Id, "Message ID should not be empty")
			assert.NotEmpty(t, sent.ThreadId, "Thread ID should not be empty")
			assert.Contains(t, sent.LabelIds, "SENT", "Should have SENT label")

			sentIDs = append(sentIDs, sent.Id)
		}
	})

	// Verify: Check that all messages are queryable via list API
	t.Run("VerifyMessagesList", func(t *testing.T) {
		response, err := gmailService.Users.Messages.List("me").Do()
		require.NoError(t, err, "List should succeed")
		assert.GreaterOrEqual(t, len(response.Messages), 5, "Should have at least 5 messages")
		assert.Equal(t, len(response.Messages), int(response.ResultSizeEstimate), "ResultSizeEstimate should match")

		// Verify all sent messages appear in list
		messageIDs := make(map[string]bool)
		for _, m := range response.Messages {
			messageIDs[m.Id] = true
		}

		for _, sentID := range sentIDs {
			assert.True(t, messageIDs[sentID], "Sent message %s should appear in list", sentID)
		}
	})

	// Verify: Check that individual messages can be retrieved with full details
	t.Run("VerifyMessageRetrieval", func(t *testing.T) {
		for i, sentID := range sentIDs {
			retrieved, err := gmailService.Users.Messages.Get("me", sentID).Format("full").Do()
			require.NoError(t, err, "Get should succeed for message: %s", sentID)
			assert.Equal(t, sentID, retrieved.Id, "Message ID should match")
			assert.NotEmpty(t, retrieved.Payload, "Should have payload")
			assert.NotEmpty(t, retrieved.Payload.Headers, "Should have headers")

			// Verify headers match what we sent
			headerMap := make(map[string]string)
			for _, header := range retrieved.Payload.Headers {
				headerMap[header.Name] = header.Value
			}

			expectedEmail := emails[i]
			assert.Equal(t, expectedEmail.From, headerMap["From"], "From header should match for message %d", i+1)
			assert.Equal(t, expectedEmail.To, headerMap["To"], "To header should match for message %d", i+1)
			assert.Equal(t, expectedEmail.Subject, headerMap["Subject"], "Subject header should match for message %d", i+1)
		}
	})

	// Verify: Check different email scenarios
	t.Run("VerifyEmailScenarios", func(t *testing.T) {
		// Scenario 1: Welcome email
		retrieved, err := gmailService.Users.Messages.Get("me", sentIDs[0]).Format("full").Do()
		require.NoError(t, err, "Should retrieve welcome email")

		headerMap := make(map[string]string)
		for _, header := range retrieved.Payload.Headers {
			headerMap[header.Name] = header.Value
		}
		assert.Contains(t, headerMap["Subject"], "Welcome", "Subject should contain 'Welcome'")

		// Scenario 2: GitHub notification
		retrieved2, err := gmailService.Users.Messages.Get("me", sentIDs[2]).Format("full").Do()
		require.NoError(t, err, "Should retrieve GitHub notification")

		headerMap2 := make(map[string]string)
		for _, header := range retrieved2.Payload.Headers {
			headerMap2[header.Name] = header.Value
		}
		assert.Contains(t, headerMap2["From"], "github.com", "From should be from GitHub")
		assert.Contains(t, headerMap2["Subject"], "Pull Request", "Subject should mention Pull Request")

		// Scenario 3: Invoice email
		retrieved3, err := gmailService.Users.Messages.Get("me", sentIDs[4]).Format("full").Do()
		require.NoError(t, err, "Should retrieve invoice email")

		headerMap3 := make(map[string]string)
		for _, header := range retrieved3.Payload.Headers {
			headerMap3[header.Name] = header.Value
		}
		assert.Contains(t, headerMap3["Subject"], "Invoice", "Subject should mention Invoice")
	})

	// Verify: Check database isolation - ensure all data has correct session_id
	t.Run("VerifyDatabaseIsolation", func(t *testing.T) {
		// Query messages from database
		dbMessages, err := queries.ListGmailMessages(ctx, database.ListGmailMessagesParams{
			SessionID: sessionID,
			Limit:     100,
		})
		require.NoError(t, err, "ListGmailMessages should succeed")
		assert.Len(t, dbMessages, 5, "Should have 5 messages in database")

		// Verify all messages can be retrieved individually
		for _, m := range dbMessages {
			fullMsg, err := queries.GetGmailMessageByID(ctx, database.GetGmailMessageByIDParams{
				ID:        m.ID,
				SessionID: sessionID,
			})
			require.NoError(t, err, "GetGmailMessageByID should succeed")
			assert.Equal(t, m.ID, fullMsg.ID, "Message ID should match")
		}
	})

	// Verify: Check that pagination works with seeded data
	t.Run("VerifyPagination", func(t *testing.T) {
		// Get first 2 messages
		response, err := gmailService.Users.Messages.List("me").MaxResults(2).Do()
		require.NoError(t, err, "List with limit should succeed")
		assert.LessOrEqual(t, len(response.Messages), 2, "Should return at most 2 messages")

		// Get next page if available
		if response.NextPageToken != "" {
			response2, err := gmailService.Users.Messages.List("me").MaxResults(2).PageToken(response.NextPageToken).Do()
			require.NoError(t, err, "List next page should succeed")
			assert.NotEmpty(t, response2.Messages, "Next page should have messages")
		}
	})
}
