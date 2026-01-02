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

// TestResendInitialStateSeed demonstrates seeding arbitrary initial state for Resend simulator
func TestResendInitialStateSeed(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDBForSeed(t)
	ctx := context.Background()

	// Setup: Create a new session
	sessionID := "resend-seed-test-session"
	err := queries.CreateSession(ctx, sessionID)
	require.NoError(t, err, "Failed to create session")

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorResend.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create Resend client pointing to test server with custom client
	client := setupResendClient(sessionID, server.URL)

	// Seed: Send multiple emails with varying subjects, recipients, and HTML content
	emails := getTestEmails()
	var sentIDs []string

	t.Run("SendInitialEmails", func(t *testing.T) {
		sentIDs = sendTestEmails(t, client, emails)
	})

	// Verify: Check that all emails are queryable via database
	t.Run("VerifyEmailsList", func(t *testing.T) {
		verifyEmailsList(t, ctx, queries, sessionID, sentIDs)
	})

	// Verify: Check that individual emails can be retrieved with full details
	t.Run("VerifyEmailRetrieval", func(t *testing.T) {
		verifyEmailRetrieval(t, ctx, queries, sessionID, sentIDs, emails)
	})

	// Verify: Check different email scenarios
	t.Run("VerifyEmailScenarios", func(t *testing.T) {
		verifyEmailScenarios(t, ctx, queries, sessionID, sentIDs)
	})

	// Verify: Check database isolation - ensure all data has correct session_id
	t.Run("VerifyDatabaseIsolation", func(t *testing.T) {
		verifyDatabaseIsolation(t, ctx, queries, sessionID, sentIDs)
	})

	// Verify: Check pagination works with seeded data
	t.Run("VerifyPagination", func(t *testing.T) {
		verifyPagination(t, ctx, queries, sessionID)
	})
}

// setupResendClient creates a Resend client configured for testing
func setupResendClient(sessionID, serverURL string) *resend.Client {
	transport := &sessionHTTPTransportSeed{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}
	client := resend.NewCustomClient(customClient, "re_test_key")
	baseURL, _ := url.Parse(serverURL + "/resend")
	client.BaseURL = baseURL
	return client
}

// getTestEmails returns test email configurations
func getTestEmails() []struct {
	From    string
	To      []string
	Subject string
	Html    string
	Cc      []string
	Bcc     []string
	ReplyTo string
} {
	return []struct {
		From    string
		To      []string
		Subject string
		Html    string
		Cc      []string
		Bcc     []string
		ReplyTo string
	}{
		{
			From:    "onboarding@example.com",
			To:      []string{"alice@example.com"},
			Subject: "Welcome to Our Platform!",
			Html:    "<h1>Welcome Alice!</h1><p>We're excited to have you on board.</p>",
		},
		{
			From:    "notifications@example.com",
			To:      []string{"bob@example.com", "charlie@example.com"},
			Subject: "New Project Created",
			Html:    "<p>A new project has been created and you've been added as a collaborator.</p>",
			Cc:      []string{"manager@example.com"},
		},
		{
			From:    "alerts@example.com",
			To:      []string{"alice@example.com"},
			Subject: "Server Alert: High CPU Usage",
			Html:    "<div style='color: red;'><strong>Warning:</strong> Server CPU usage is at 90%</div>",
			ReplyTo: "support@example.com",
		},
		{
			From:    "billing@example.com",
			To:      []string{"bob@example.com"},
			Subject: "Invoice #12345",
			Html:    "<h2>Invoice</h2><p>Amount due: $99.00</p><p>Due date: 2026-02-01</p>",
			Bcc:     []string{"accounting@example.com"},
		},
		{
			From:    "reports@example.com",
			To:      []string{"alice@example.com", "charlie@example.com"},
			Subject: "Weekly Report - Week of Jan 2, 2026",
			Html:    "<h1>Weekly Summary</h1><ul><li>Tasks completed: 25</li><li>New issues: 3</li></ul>",
			Cc:      []string{"manager@example.com"},
			ReplyTo: "reports@example.com",
		},
	}
}

// sendTestEmails sends all test emails and returns their IDs
func sendTestEmails(t *testing.T, client *resend.Client, emails []struct {
	From    string
	To      []string
	Subject string
	Html    string
	Cc      []string
	Bcc     []string
	ReplyTo string
}) []string {
	t.Helper()
	sentIDs := make([]string, 0, len(emails))
	for i := range emails {
		email := emails[i]
		params := &resend.SendEmailRequest{
			From:    email.From,
			To:      email.To,
			Subject: email.Subject,
			Html:    email.Html,
		}

		if len(email.Cc) > 0 {
			params.Cc = email.Cc
		}
		if len(email.Bcc) > 0 {
			params.Bcc = email.Bcc
		}
		if email.ReplyTo != "" {
			params.ReplyTo = email.ReplyTo
		}

		sent, err := client.Emails.Send(params)
		require.NoError(t, err, "Failed to send email %d: %s", i+1, email.Subject)
		assert.NotEmpty(t, sent.Id, "Email ID should not be empty")
		sentIDs = append(sentIDs, sent.Id)
	}
	return sentIDs
}

// verifyEmailsList verifies that all emails appear in the database list
func verifyEmailsList(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string, sentIDs []string) {
	t.Helper()
	dbEmails, err := queries.ListResendEmails(ctx, database.ListResendEmailsParams{
		SessionID: sessionID,
		Limit:     100,
	})
	require.NoError(t, err, "ListResendEmails should succeed")
	assert.GreaterOrEqual(t, len(dbEmails), 5, "Should have at least 5 emails")

	emailIDs := make(map[string]bool)
	for _, e := range dbEmails {
		emailIDs[e.ID] = true
	}

	for _, sentID := range sentIDs {
		assert.True(t, emailIDs[sentID], "Sent email %s should appear in list", sentID)
	}
}

// verifyEmailRetrieval verifies individual email retrieval
func verifyEmailRetrieval(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string, sentIDs []string, emails []struct {
	From    string
	To      []string
	Subject string
	Html    string
	Cc      []string
	Bcc     []string
	ReplyTo string
}) {
	t.Helper()
	for i, sentID := range sentIDs {
		retrieved, err := queries.GetResendEmailByID(ctx, database.GetResendEmailByIDParams{
			ID:        sentID,
			SessionID: sessionID,
		})
		require.NoError(t, err, "GetResendEmailByID should succeed for email: %s", sentID)
		assert.Equal(t, sentID, retrieved.ID, "Email ID should match")

		expectedEmail := emails[i]
		assert.Equal(t, expectedEmail.From, retrieved.FromEmail, "From email should match for email %d", i+1)
		assert.Equal(t, expectedEmail.Subject, retrieved.Subject, "Subject should match for email %d", i+1)
		assert.Equal(t, expectedEmail.Html, retrieved.Html, "HTML should match for email %d", i+1)
	}
}

// verifyEmailScenarios verifies different email scenarios
func verifyEmailScenarios(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string, sentIDs []string) {
	t.Helper()
	// Scenario 1: Welcome email (simple)
	retrieved, err := queries.GetResendEmailByID(ctx, database.GetResendEmailByIDParams{
		ID:        sentIDs[0],
		SessionID: sessionID,
	})
	require.NoError(t, err, "Should retrieve welcome email")
	assert.Contains(t, retrieved.Subject, "Welcome", "Subject should contain 'Welcome'")
	assert.Contains(t, retrieved.Html, "Welcome Alice", "HTML should contain welcome message")
	assert.False(t, retrieved.CcEmails.Valid, "Should not have CC")
	assert.False(t, retrieved.BccEmails.Valid, "Should not have BCC")
	assert.False(t, retrieved.ReplyTo.Valid, "Should not have ReplyTo")

	// Scenario 2: Notification with CC (multiple recipients and CC)
	retrieved2, err := queries.GetResendEmailByID(ctx, database.GetResendEmailByIDParams{
		ID:        sentIDs[1],
		SessionID: sessionID,
	})
	require.NoError(t, err, "Should retrieve notification email")
	assert.Contains(t, retrieved2.Subject, "Project Created", "Subject should mention project")
	assert.Contains(t, retrieved2.ToEmails, "bob@example.com", "Should include bob in recipients")
	assert.Contains(t, retrieved2.ToEmails, "charlie@example.com", "Should include charlie in recipients")
	assert.True(t, retrieved2.CcEmails.Valid, "Should have CC")
	assert.Contains(t, retrieved2.CcEmails.String, "manager@example.com", "Should CC manager")

	// Scenario 3: Alert with ReplyTo
	retrieved3, err := queries.GetResendEmailByID(ctx, database.GetResendEmailByIDParams{
		ID:        sentIDs[2],
		SessionID: sessionID,
	})
	require.NoError(t, err, "Should retrieve alert email")
	assert.Contains(t, retrieved3.Subject, "Alert", "Subject should mention Alert")
	assert.True(t, retrieved3.ReplyTo.Valid, "Should have ReplyTo")
	assert.Equal(t, "support@example.com", retrieved3.ReplyTo.String, "ReplyTo should be support email")

	// Scenario 4: Invoice with BCC
	retrieved4, err := queries.GetResendEmailByID(ctx, database.GetResendEmailByIDParams{
		ID:        sentIDs[3],
		SessionID: sessionID,
	})
	require.NoError(t, err, "Should retrieve invoice email")
	assert.Contains(t, retrieved4.Subject, "Invoice", "Subject should mention Invoice")
	assert.True(t, retrieved4.BccEmails.Valid, "Should have BCC")
	assert.Contains(t, retrieved4.BccEmails.String, "accounting@example.com", "Should BCC accounting")

	// Scenario 5: Report with CC and ReplyTo
	retrieved5, err := queries.GetResendEmailByID(ctx, database.GetResendEmailByIDParams{
		ID:        sentIDs[4],
		SessionID: sessionID,
	})
	require.NoError(t, err, "Should retrieve report email")
	assert.Contains(t, retrieved5.Subject, "Weekly Report", "Subject should mention report")
	assert.True(t, retrieved5.CcEmails.Valid, "Should have CC")
	assert.True(t, retrieved5.ReplyTo.Valid, "Should have ReplyTo")
}

// verifyDatabaseIsolation verifies database session isolation
func verifyDatabaseIsolation(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string, sentIDs []string) {
	t.Helper()
	dbEmails, err := queries.ListResendEmails(ctx, database.ListResendEmailsParams{
		SessionID: sessionID,
		Limit:     100,
	})
	require.NoError(t, err, "ListResendEmails should succeed")
	assert.Len(t, dbEmails, 5, "Should have 5 emails in database")

	for _, e := range dbEmails {
		fullEmail, err := queries.GetResendEmailByID(ctx, database.GetResendEmailByIDParams{
			ID:        e.ID,
			SessionID: sessionID,
		})
		require.NoError(t, err, "GetResendEmailByID should succeed")
		assert.Equal(t, e.ID, fullEmail.ID, "Email ID should match")
	}

	wrongSessionID := "wrong-session"
	for _, sentID := range sentIDs {
		_, err := queries.GetResendEmailByID(ctx, database.GetResendEmailByIDParams{
			ID:        sentID,
			SessionID: wrongSessionID,
		})
		assert.Error(t, err, "Should not retrieve email from different session")
	}
}

// verifyPagination verifies pagination functionality
func verifyPagination(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string) {
	t.Helper()
	dbEmails, err := queries.ListResendEmails(ctx, database.ListResendEmailsParams{
		SessionID: sessionID,
		Limit:     2,
	})
	require.NoError(t, err, "List with limit should succeed")
	assert.LessOrEqual(t, len(dbEmails), 2, "Should return at most 2 emails")

	allEmails, err := queries.ListResendEmails(ctx, database.ListResendEmailsParams{
		SessionID: sessionID,
		Limit:     100,
	})
	require.NoError(t, err, "List all should succeed")

	for i := 1; i < len(allEmails); i++ {
		assert.GreaterOrEqual(t, allEmails[i-1].CreatedAt, allEmails[i].CreatedAt,
			"Emails should be ordered by created_at DESC")
	}
}
