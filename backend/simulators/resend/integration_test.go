package resend_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
	simulatorResend "github.com/recreate-run/nova-simulators/simulators/resend"
	"github.com/resend/resend-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestResendSimulatorSendBasicEmail(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "resend-test-session-1"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorResend.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client that adds session header
	transport := &sessionHTTPTransport{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create Resend client pointing to test server with custom client
	client := resend.NewCustomClient(customClient, "re_test_key")
	baseURL, _ := url.Parse(server.URL + "/resend")
	client.BaseURL = baseURL

	t.Run("SendBasicEmail", func(t *testing.T) {
		// Send email
		params := &resend.SendEmailRequest{
			From:    "onboarding@resend.dev",
			To:      []string{"user@example.com"},
			Subject: "Test Email",
			Html:    "<p>Hello World!</p>",
		}

		sent, err := client.Emails.Send(params)

		// Assertions
		require.NoError(t, err, "Send should not return error")
		assert.NotNil(t, sent, "Should return sent email")
		assert.NotEmpty(t, sent.Id, "Email ID should not be empty")
	})
}

func TestResendSimulatorSendEmailWithCC(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "resend-test-session-2"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorResend.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client
	transport := &sessionHTTPTransport{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create Resend client
	client := resend.NewCustomClient(customClient, "re_test_key")
	baseURL, _ := url.Parse(server.URL + "/resend")
	client.BaseURL = baseURL

	t.Run("SendEmailWithCC", func(t *testing.T) {
		// Send email with CC
		params := &resend.SendEmailRequest{
			From:    "onboarding@resend.dev",
			To:      []string{"user@example.com"},
			Subject: "Test Email with CC",
			Html:    "<p>Hello with CC!</p>",
			Cc:      []string{"cc1@example.com", "cc2@example.com"},
		}

		sent, err := client.Emails.Send(params)

		// Assertions
		require.NoError(t, err, "Send should not return error")
		assert.NotNil(t, sent, "Should return sent email")
		assert.NotEmpty(t, sent.Id, "Email ID should not be empty")
	})
}

func TestResendSimulatorSendEmailWithBCC(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "resend-test-session-3"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorResend.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client
	transport := &sessionHTTPTransport{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create Resend client
	client := resend.NewCustomClient(customClient, "re_test_key")
	baseURL, _ := url.Parse(server.URL + "/resend")
	client.BaseURL = baseURL

	t.Run("SendEmailWithBCC", func(t *testing.T) {
		// Send email with BCC
		params := &resend.SendEmailRequest{
			From:    "onboarding@resend.dev",
			To:      []string{"user@example.com"},
			Subject: "Test Email with BCC",
			Html:    "<p>Hello with BCC!</p>",
			Bcc:     []string{"bcc1@example.com", "bcc2@example.com"},
		}

		sent, err := client.Emails.Send(params)

		// Assertions
		require.NoError(t, err, "Send should not return error")
		assert.NotNil(t, sent, "Should return sent email")
		assert.NotEmpty(t, sent.Id, "Email ID should not be empty")
	})
}

func TestResendSimulatorSendEmailMultipleRecipients(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "resend-test-session-4"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorResend.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client
	transport := &sessionHTTPTransport{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create Resend client
	client := resend.NewCustomClient(customClient, "re_test_key")
	baseURL, _ := url.Parse(server.URL + "/resend")
	client.BaseURL = baseURL

	t.Run("SendEmailToMultipleRecipients", func(t *testing.T) {
		// Send email to multiple recipients
		params := &resend.SendEmailRequest{
			From:    "onboarding@resend.dev",
			To:      []string{"user1@example.com", "user2@example.com", "user3@example.com"},
			Subject: "Test Email Multiple Recipients",
			Html:    "<p>Hello everyone!</p>",
		}

		sent, err := client.Emails.Send(params)

		// Assertions
		require.NoError(t, err, "Send should not return error")
		assert.NotNil(t, sent, "Should return sent email")
		assert.NotEmpty(t, sent.Id, "Email ID should not be empty")
	})
}

func TestResendSimulatorSendEmailWithAllParameters(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "resend-test-session-5"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorResend.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client
	transport := &sessionHTTPTransport{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create Resend client
	client := resend.NewCustomClient(customClient, "re_test_key")
	baseURL, _ := url.Parse(server.URL + "/resend")
	client.BaseURL = baseURL

	t.Run("SendEmailWithAllParameters", func(t *testing.T) {
		// Send email with all optional parameters
		params := &resend.SendEmailRequest{
			From:    "onboarding@resend.dev",
			To:      []string{"user1@example.com", "user2@example.com"},
			Subject: "Test Email with All Parameters",
			Html:    "<p>Hello with all params!</p>",
			Cc:      []string{"cc@example.com"},
			Bcc:     []string{"bcc@example.com"},
			ReplyTo: "reply@example.com",
		}

		sent, err := client.Emails.Send(params)

		// Assertions
		require.NoError(t, err, "Send should not return error")
		assert.NotNil(t, sent, "Should return sent email")
		assert.NotEmpty(t, sent.Id, "Email ID should not be empty")
	})
}

func TestResendSimulatorSessionIsolation(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorResend.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create two clients with different sessions
	session1ID := "resend-test-session-6a"
	transport1 := &sessionHTTPTransport{
		sessionID: session1ID,
	}
	client1 := resend.NewCustomClient(&http.Client{Transport: transport1}, "re_test_key")
	baseURL1, _ := url.Parse(server.URL + "/resend")
	client1.BaseURL = baseURL1

	session2ID := "resend-test-session-6b"
	transport2 := &sessionHTTPTransport{
		sessionID: session2ID,
	}
	client2 := resend.NewCustomClient(&http.Client{Transport: transport2}, "re_test_key")
	baseURL2, _ := url.Parse(server.URL + "/resend")
	client2.BaseURL = baseURL2

	t.Run("SessionIsolation", func(t *testing.T) {
		// Send email in session 1
		params1 := &resend.SendEmailRequest{
			From:    "onboarding@resend.dev",
			To:      []string{"session1@example.com"},
			Subject: "Session 1 Email",
			Html:    "<p>Session 1</p>",
		}
		sent1, err := client1.Emails.Send(params1)
		require.NoError(t, err, "Session 1 send should succeed")

		// Send email in session 2
		params2 := &resend.SendEmailRequest{
			From:    "onboarding@resend.dev",
			To:      []string{"session2@example.com"},
			Subject: "Session 2 Email",
			Html:    "<p>Session 2</p>",
		}
		sent2, err := client2.Emails.Send(params2)
		require.NoError(t, err, "Session 2 send should succeed")

		// Verify emails have different IDs
		assert.NotEqual(t, sent1.Id, sent2.Id, "Email IDs should be different")

		// Verify session isolation by checking database directly
		ctx := context.Background()
		email1, err := queries.GetResendEmailByID(ctx, database.GetResendEmailByIDParams{
			ID:        sent1.Id,
			SessionID: session1ID,
		})
		require.NoError(t, err, "Should find email in session 1")
		assert.Contains(t, email1.ToEmails, "session1@example.com", "Should match session 1 recipient")

		// Verify email from session 1 is not accessible in session 2
		_, err = queries.GetResendEmailByID(ctx, database.GetResendEmailByIDParams{
			ID:        sent1.Id,
			SessionID: session2ID,
		})
		assert.Error(t, err, "Session 1 email should not be accessible in session 2")
	})
}
