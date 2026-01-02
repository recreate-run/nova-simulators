package slack_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
	"github.com/recreate-run/nova-simulators/internal/transport"
	simulatorSlack "github.com/recreate-run/nova-simulators/simulators/slack"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

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

// getTestSessionIDs returns the session-specific IDs for a given session
func getTestSessionIDs(sessionID string) (channelID1, channelID2, userID1, userID2 string) {
	return "C001_" + sessionID, "C002_" + sessionID, "U123456_" + sessionID, "U789012_" + sessionID
}

func setupTestSession(t *testing.T, queries *database.Queries, sessionID string) {
	t.Helper()
	ctx := context.Background()

	// Create default channels for this session with session-specific IDs
	timestamp := int64(1640000000)

	// Generate session-specific channel IDs to avoid primary key conflicts
	channelID1 := "C001_" + sessionID
	channelID2 := "C002_" + sessionID

	err := queries.CreateChannel(ctx, database.CreateChannelParams{
		ID:        channelID1,
		Name:      "general",
		CreatedAt: timestamp,
		SessionID: sessionID,
	})
	require.NoError(t, err, "Failed to create channel")

	err = queries.CreateChannel(ctx, database.CreateChannelParams{
		ID:        channelID2,
		Name:      "random",
		CreatedAt: timestamp,
		SessionID: sessionID,
	})
	require.NoError(t, err, "Failed to create channel")

	// Create default users for this session with session-specific IDs
	userID1 := "U123456_" + sessionID
	userID2 := "U789012_" + sessionID

	err = queries.CreateUser(ctx, database.CreateUserParams{
		ID:              userID1,
		TeamID:          "T021F9ZE2",
		Name:            "test-user",
		RealName:        "Test User",
		Email:           database.StringToNullString("test@example.com"),
		DisplayName:     database.StringToNullString("testuser"),
		FirstName:       database.StringToNullString("Test"),
		LastName:        database.StringToNullString("User"),
		IsAdmin:         0,
		IsOwner:         0,
		IsBot:           0,
		Timezone:        database.StringToNullString("America/Los_Angeles"),
		TimezoneLabel:   database.StringToNullString("Pacific Standard Time"),
		TimezoneOffset:  database.Int64ToNullInt64(-28800),
		Image24:         database.StringToNullString(""),
		Image32:         database.StringToNullString(""),
		Image48:         database.StringToNullString(""),
		Image72:         database.StringToNullString(""),
		Image192:        database.StringToNullString(""),
		Image512:        database.StringToNullString(""),
		SessionID:       sessionID,
	})
	require.NoError(t, err, "Failed to create user")

	err = queries.CreateUser(ctx, database.CreateUserParams{
		ID:              userID2,
		TeamID:          "T021F9ZE2",
		Name:            "bobby",
		RealName:        "Bobby Tables",
		Email:           database.StringToNullString("bobby@example.com"),
		DisplayName:     database.StringToNullString("bobby"),
		FirstName:       database.StringToNullString("Bobby"),
		LastName:        database.StringToNullString("Tables"),
		IsAdmin:         0,
		IsOwner:         0,
		IsBot:           0,
		Timezone:        database.StringToNullString("America/Los_Angeles"),
		TimezoneLabel:   database.StringToNullString("Pacific Standard Time"),
		TimezoneOffset:  database.Int64ToNullInt64(-28800),
		Image24:         database.StringToNullString(""),
		Image32:         database.StringToNullString(""),
		Image48:         database.StringToNullString(""),
		Image72:         database.StringToNullString(""),
		Image192:        database.StringToNullString(""),
		Image512:        database.StringToNullString(""),
		SessionID:       sessionID,
	})
	require.NoError(t, err, "Failed to create user")
}

func TestSlackSimulatorIntegration(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "test-session-1"
	setupTestSession(t, queries, sessionID)
	channelID1, channelID2, _, _ := getTestSessionIDs(sessionID)

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorSlack.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Install HTTP interceptor to route slack.com to test server with session ID
	http.DefaultTransport = transport.NewSimulatorTransport(map[string]string{
		"slack.com": server.URL[7:], // Strip "http://" prefix
	}).WithSessionID(sessionID)

	// Create Slack client
	client := slack.New("fake-token-12345")

	t.Run("PostMessage", func(t *testing.T) {
		// Post a message to #general
		channel, timestamp, err := client.PostMessage(
			channelID1,
			slack.MsgOptionText("Hello from integration test!", false),
		)

		// Assertions
		require.NoError(t, err, "PostMessage should not return error")
		assert.Equal(t, channelID1, channel, "Channel ID should match")
		assert.NotEmpty(t, timestamp, "Timestamp should be returned")
	})

	t.Run("GetConversations", func(t *testing.T) {
		// Get channel list
		channels, _, err := client.GetConversations(&slack.GetConversationsParameters{
			ExcludeArchived: true,
			Types:           []string{"public_channel", "private_channel"},
		})

		// Assertions
		require.NoError(t, err, "GetConversations should not return error")
		require.Len(t, channels, 2, "Should return 2 channels")

		// Verify channel details
		channelMap := make(map[string]slack.Channel)
		for _, ch := range channels {
			channelMap[ch.ID] = ch
		}

		assert.Contains(t, channelMap, channelID1, "Should contain #general channel")
		assert.Equal(t, "general", channelMap[channelID1].Name, "#general name should match")

		assert.Contains(t, channelMap, channelID2, "Should contain #random channel")
		assert.Equal(t, "random", channelMap[channelID2].Name, "#random name should match")
	})

	t.Run("GetConversationHistory", func(t *testing.T) {
		// First, post a message
		expectedText := "Test message for history"
		_, expectedTimestamp, err := client.PostMessage(
			channelID1,
			slack.MsgOptionText(expectedText, false),
		)
		require.NoError(t, err, "PostMessage should succeed")

		// Get conversation history
		history, err := client.GetConversationHistory(&slack.GetConversationHistoryParameters{
			ChannelID: channelID1,
			Limit:     10,
		})

		// Assertions
		require.NoError(t, err, "GetConversationHistory should not return error")
		require.NotEmpty(t, history.Messages, "Should return at least one message")

		// Find our message in the history
		found := false
		for _, msg := range history.Messages {
			if msg.Timestamp != expectedTimestamp {
				continue
			}
			assert.Equal(t, expectedText, msg.Text, "Message text should match")
			assert.Equal(t, "message", msg.Type, "Message type should be 'message'")
			assert.Equal(t, "U123456", msg.User, "Message user should be default user")
			found = true
			break
		}
		assert.True(t, found, "Posted message should appear in history")
	})

	t.Run("AuthTest", func(t *testing.T) {
		// Test authentication endpoint
		response, err := client.AuthTest()

		// Assertions
		require.NoError(t, err, "AuthTest should not return error")
		assert.NotNil(t, response, "AuthTest should return response")
		assert.Equal(t, "Test Workspace", response.Team, "Team name should match")
		assert.Equal(t, "U123456", response.UserID, "User ID should be default user")
	})
}

func TestSlackSimulatorMessagePersistence(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "test-session-2"
	setupTestSession(t, queries, sessionID)
	channelID1, _, _, _ := getTestSessionIDs(sessionID)

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorSlack.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Install HTTP interceptor to route slack.com to test server with session ID
	http.DefaultTransport = transport.NewSimulatorTransport(map[string]string{
		"slack.com": server.URL[7:], // Strip "http://" prefix
	}).WithSessionID(sessionID)

	client := slack.New("fake-token-12345")

	// Post multiple messages
	messages := []string{
		"First message",
		"Second message",
		"Third message",
	}

	for _, text := range messages {
		_, _, err := client.PostMessage(channelID1, slack.MsgOptionText(text, false))
		require.NoError(t, err, "PostMessage should succeed")
	}

	// Retrieve history
	history, err := client.GetConversationHistory(&slack.GetConversationHistoryParameters{
		ChannelID: channelID1,
		Limit:     10,
	})

	// Assertions
	require.NoError(t, err, "GetConversationHistory should succeed")
	assert.GreaterOrEqual(t, len(history.Messages), 3, "Should have at least 3 messages")

	// Verify all messages are present
	messageTexts := make([]string, 0, len(history.Messages))
	for _, msg := range history.Messages {
		messageTexts = append(messageTexts, msg.Text)
	}

	for _, expectedText := range messages {
		assert.Contains(t, messageTexts, expectedText, "Message should be in history")
	}
}

func TestSlackSimulatorFileUpload(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "test-session-3"
	setupTestSession(t, queries, sessionID)
	channelID1, _, _, _ := getTestSessionIDs(sessionID)

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorSlack.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Install HTTP interceptor to route slack.com to test server with session ID
	http.DefaultTransport = transport.NewSimulatorTransport(map[string]string{
		"slack.com":       server.URL[7:], // Strip "http://" prefix
		"files.slack.com": server.URL[7:], // Also route files.slack.com for upload URL
	}).WithSessionID(sessionID)

	client := slack.New("fake-token-12345")

	t.Run("UploadFileV2", func(t *testing.T) {
		// Create a test file content
		fileContent := "This is a test file"

		// Upload file
		summary, err := client.UploadFileV2(slack.UploadFileV2Parameters{
			Channel:  channelID1,
			Filename: "test.txt",
			FileSize: len(fileContent),
			Title:    "Test File",
			Content:  fileContent,
		})

		// Assertions
		require.NoError(t, err, "UploadFileV2 should not return error")
		assert.NotNil(t, summary, "Should return upload summary")
		assert.NotEmpty(t, summary.ID, "Should have file ID")
		assert.Equal(t, "Test File", summary.Title, "File title should match")
	})
}

func TestSlackSimulatorUserInfo(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "test-session-4"
	setupTestSession(t, queries, sessionID)
	_, _, userID1, userID2 := getTestSessionIDs(sessionID)

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorSlack.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Install HTTP interceptor to route slack.com to test server with session ID
	http.DefaultTransport = transport.NewSimulatorTransport(map[string]string{
		"slack.com": server.URL[7:], // Strip "http://" prefix
	}).WithSessionID(sessionID)

	client := slack.New("fake-token-12345")

	t.Run("GetUserInfo", func(t *testing.T) {
		// Get user info for default user
		user, err := client.GetUserInfo(userID1)

		// Assertions
		require.NoError(t, err, "GetUserInfo should not return error")
		assert.NotNil(t, user, "Should return user info")
		assert.Equal(t, userID1, user.ID, "User ID should match")
		assert.Equal(t, "test-user", user.Name, "User name should match")
		assert.Equal(t, "Test User", user.RealName, "Real name should match")
		assert.Equal(t, "test@example.com", user.Profile.Email, "Email should match")
	})

	t.Run("GetUserInfo_SecondUser", func(t *testing.T) {
		// Get user info for second default user
		user, err := client.GetUserInfo(userID2)

		// Assertions
		require.NoError(t, err, "GetUserInfo should not return error")
		assert.NotNil(t, user, "Should return user info")
		assert.Equal(t, userID2, user.ID, "User ID should match")
		assert.Equal(t, "bobby", user.Name, "User name should match")
		assert.Equal(t, "Bobby Tables", user.RealName, "Real name should match")
	})
}

func TestSlackSimulatorAttachments(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "test-session-5"
	setupTestSession(t, queries, sessionID)
	channelID1, _, _, _ := getTestSessionIDs(sessionID)

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorSlack.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Install HTTP interceptor to route slack.com to test server with session ID
	http.DefaultTransport = transport.NewSimulatorTransport(map[string]string{
		"slack.com": server.URL[7:], // Strip "http://" prefix
	}).WithSessionID(sessionID)

	client := slack.New("fake-token-12345")

	t.Run("PostMessageWithAttachments", func(t *testing.T) {
		// Create attachment
		attachment := slack.Attachment{
			Color:      "good",
			AuthorName: "Test Author",
			Title:      "Test Attachment",
			Text:       "This is a test attachment",
			Footer:     "Test Footer",
		}

		// Post message with attachment
		channel, timestamp, err := client.PostMessage(
			channelID1,
			slack.MsgOptionText("Message with attachment", false),
			slack.MsgOptionAttachments(attachment),
		)

		// Assertions
		require.NoError(t, err, "PostMessage with attachments should not return error")
		assert.Equal(t, channelID1, channel, "Channel ID should match")
		assert.NotEmpty(t, timestamp, "Timestamp should be returned")

		// Verify message was stored with attachments by retrieving history
		history, err := client.GetConversationHistory(&slack.GetConversationHistoryParameters{
			ChannelID: channelID1,
			Limit:     10,
		})

		require.NoError(t, err, "GetConversationHistory should succeed")

		// Find our message in history
		found := false
		for _, msg := range history.Messages {
			if msg.Timestamp == timestamp {
				assert.Equal(t, "Message with attachment", msg.Text, "Message text should match")
				found = true
				break
			}
		}
		assert.True(t, found, "Message with attachment should appear in history")
	})
}
