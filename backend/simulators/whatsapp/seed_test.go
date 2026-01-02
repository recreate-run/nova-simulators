package whatsapp_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
	simulatorWhatsApp "github.com/recreate-run/nova-simulators/simulators/whatsapp"
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

// TestWhatsAppInitialStateSeed demonstrates seeding arbitrary initial state for WhatsApp simulator
func TestWhatsAppInitialStateSeed(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDBForSeed(t)
	ctx := context.Background()

	// Setup: Create a new session
	sessionID := "whatsapp-seed-test-session"
	err := queries.CreateSession(ctx, sessionID)
	require.NoError(t, err, "Failed to create session")

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorWhatsApp.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client that adds session header
	transport := &sessionHTTPTransportSeed{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create WhatsApp client pointing to test server
	phoneNumberID := "123456789012345"
	client := NewClient(phoneNumberID, "test-access-token", server.URL)
	client.httpClient = customClient

	// Seed: Send various types of WhatsApp messages
	messages := getTestMessages()
	var sentIDs []string

	t.Run("SendInitialMessages", func(t *testing.T) {
		sentIDs = sendInitialMessages(t, client, messages)
	})

	// Verify: Check that all messages are stored in database
	t.Run("VerifyMessagesList", func(t *testing.T) {
		verifyMessagesList(t, ctx, queries, sessionID, sentIDs)
	})

	// Verify: Check that individual messages can be retrieved with full details
	t.Run("VerifyMessageRetrieval", func(t *testing.T) {
		verifyMessageRetrieval(t, ctx, queries, sessionID, phoneNumberID, sentIDs, messages)
	})

	// Verify: Check different message type scenarios
	t.Run("VerifyMessageTypeScenarios", func(t *testing.T) {
		verifyMessageTypeScenarios(t, ctx, queries, sessionID, sentIDs)
	})

	// Verify: Check database isolation - ensure all data has correct session_id
	t.Run("VerifyDatabaseIsolation", func(t *testing.T) {
		verifyWhatsAppDatabaseIsolation(t, ctx, queries, sessionID, phoneNumberID)
	})

	// Verify: Check that pagination works with seeded data
	t.Run("VerifyPagination", func(t *testing.T) {
		verifyWhatsAppPagination(t, ctx, queries, sessionID)
	})

	// Verify: Test different recipient phone numbers
	t.Run("VerifyRecipientVariety", func(t *testing.T) {
		verifyRecipientVariety(t, ctx, queries, sessionID)
	})
}
