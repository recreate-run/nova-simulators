package outlook_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
	simulatorOutlook "github.com/recreate-run/nova-simulators/simulators/outlook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestOutlookInitialStateSeed demonstrates seeding arbitrary initial state for Outlook simulator
func TestOutlookInitialStateSeed(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDBForSeed(t)
	ctx := context.Background()

	// Setup: Create a new session
	sessionID := "outlook-seed-test-session"
	err := queries.CreateSession(ctx, sessionID)
	require.NoError(t, err, "Failed to create session")

	// Seed: Create custom messages
	messages := seedOutlookTestData(t, ctx, queries, sessionID)

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorOutlook.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Verify: Check that messages are queryable
	t.Run("VerifyMessages", func(t *testing.T) {
		verifyMessages(t, server.URL, sessionID, messages)
	})

	// Verify: Check that individual messages can be retrieved
	t.Run("VerifyIndividualMessages", func(t *testing.T) {
		verifyIndividualMessages(t, server.URL, sessionID, messages)
	})

	// Verify: Check database isolation - ensure all data is correctly stored
	t.Run("VerifyDatabaseIsolation", func(t *testing.T) {
		verifyDatabaseIsolation(t, ctx, queries, sessionID, messages)
	})
}

// seedOutlookTestData creates messages for testing
func seedOutlookTestData(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string) []struct {
	ID          string
	FromEmail   string
	ToEmail     string
	Subject     string
	BodyContent string
	BodyType    string
	IsRead      bool
} {
	t.Helper()

	// Seed: Create custom messages (use session-specific IDs to avoid conflicts)
	messages := []struct {
		ID          string
		FromEmail   string
		ToEmail     string
		Subject     string
		BodyContent string
		BodyType    string
		IsRead      bool
	}{
		{
			ID:          "MSG001_" + sessionID,
			FromEmail:   "alice@example.com",
			ToEmail:     "me@example.com",
			Subject:     "Team Meeting Tomorrow",
			BodyContent: "Hi team, reminder about our meeting tomorrow at 10 AM.",
			BodyType:    "text",
			IsRead:      false,
		},
		{
			ID:          "MSG002_" + sessionID,
			FromEmail:   "bob@example.com",
			ToEmail:     "me@example.com",
			Subject:     "Project Status Update",
			BodyContent: "The project is on track for delivery next week.",
			BodyType:    "text",
			IsRead:      true,
		},
		{
			ID:          "MSG003_" + sessionID,
			FromEmail:   "charlie@example.com",
			ToEmail:     "me@example.com",
			Subject:     "Quarterly Review",
			BodyContent: "<html><body><h1>Q4 Results</h1><p>Excellent performance this quarter!</p></body></html>",
			BodyType:    "html",
			IsRead:      false,
		},
	}

	receivedDateTime := time.Now().UTC().Format(time.RFC3339)

	for _, msg := range messages {
		isReadValue := int64(0)
		if msg.IsRead {
			isReadValue = 1
		}

		err := queries.CreateOutlookMessage(ctx, database.CreateOutlookMessageParams{
			ID:               msg.ID,
			FromEmail:        msg.FromEmail,
			ToEmail:          msg.ToEmail,
			Subject:          msg.Subject,
			BodyContent:      sql.NullString{String: msg.BodyContent, Valid: true},
			BodyType:         msg.BodyType,
			IsRead:           isReadValue,
			ReceivedDatetime: receivedDateTime,
			SessionID:        sessionID,
		})
		require.NoError(t, err, "Failed to create message: %s", msg.Subject)
	}

	return messages
}

// verifyMessages verifies that messages can be listed
func verifyMessages(t *testing.T, serverURL, sessionID string, messages []struct {
	ID          string
	FromEmail   string
	ToEmail     string
	Subject     string
	BodyContent string
	BodyType    string
	IsRead      bool
}) {
	t.Helper()

	// List messages
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, serverURL+"/v1.0/me/messages", http.NoBody)
	require.NoError(t, err)
	req.Header.Set("X-Session-ID", sessionID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "List messages should succeed")

	var response MessageListResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	assert.Len(t, response.Value, 3, "Should have 3 messages")

	// Verify message subjects
	subjectMap := make(map[string]*Message)
	for i := range response.Value {
		subjectMap[response.Value[i].Subject] = response.Value[i]
	}

	assert.Contains(t, subjectMap, "Team Meeting Tomorrow", "Should have team meeting message")
	assert.Contains(t, subjectMap, "Project Status Update", "Should have project status message")
	assert.Contains(t, subjectMap, "Quarterly Review", "Should have quarterly review message")
}

// verifyIndividualMessages verifies that individual messages can be retrieved
func verifyIndividualMessages(t *testing.T, serverURL, sessionID string, messages []struct {
	ID          string
	FromEmail   string
	ToEmail     string
	Subject     string
	BodyContent string
	BodyType    string
	IsRead      bool
}) {
	t.Helper()

	// Get each message and verify content
	for _, msg := range messages {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, serverURL+"/v1.0/me/messages/"+msg.ID, http.NoBody)
		require.NoError(t, err)
		req.Header.Set("X-Session-ID", sessionID)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Get message should succeed")

		var retrieved Message
		err = json.NewDecoder(resp.Body).Decode(&retrieved)
		require.NoError(t, err)

		assert.Equal(t, msg.ID, retrieved.ID, "Message ID should match")
		assert.Equal(t, msg.Subject, retrieved.Subject, "Subject should match")
		assert.Equal(t, msg.FromEmail, retrieved.From.EmailAddress.Address, "From email should match")
		assert.Equal(t, msg.ToEmail, retrieved.ToRecipients[0].EmailAddress.Address, "To email should match")
		assert.Equal(t, msg.BodyContent, retrieved.Body.Content, "Body content should match")
		assert.Equal(t, msg.BodyType, retrieved.Body.ContentType, "Body type should match")
		assert.Equal(t, msg.IsRead, retrieved.IsRead, "IsRead status should match")
	}
}

// verifyDatabaseIsolation verifies database isolation
func verifyDatabaseIsolation(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string,
	messages []struct {
		ID          string
		FromEmail   string
		ToEmail     string
		Subject     string
		BodyContent string
		BodyType    string
		IsRead      bool
	}) {
	t.Helper()

	// Query messages from database
	dbMessages, err := queries.ListOutlookMessages(ctx, database.ListOutlookMessagesParams{
		SessionID: sessionID,
		Limit:     10,
	})
	require.NoError(t, err, "ListOutlookMessages should succeed")
	assert.Len(t, dbMessages, 3, "Should have 3 messages in database")

	// Verify each message in database
	for _, msg := range messages {
		dbMessage, err := queries.GetOutlookMessageByID(ctx, database.GetOutlookMessageByIDParams{
			ID:        msg.ID,
			SessionID: sessionID,
		})
		require.NoError(t, err, "GetOutlookMessageByID should succeed for: %s", msg.Subject)
		assert.Equal(t, msg.Subject, dbMessage.Subject, "Subject should match in database")
		assert.Equal(t, msg.FromEmail, dbMessage.FromEmail, "From email should match in database")
		assert.Equal(t, msg.ToEmail, dbMessage.ToEmail, "To email should match in database")

		if dbMessage.BodyContent.Valid {
			assert.Equal(t, msg.BodyContent, dbMessage.BodyContent.String, "Body content should match in database")
		}
		assert.Equal(t, msg.BodyType, dbMessage.BodyType, "Body type should match in database")

		expectedIsRead := int64(0)
		if msg.IsRead {
			expectedIsRead = 1
		}
		assert.Equal(t, expectedIsRead, dbMessage.IsRead, "IsRead should match in database")
	}
}
