package slack_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
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

func TestSlackSimulatorIntegration(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Start simulator server
	handler := simulatorSlack.NewHandler(queries)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Install HTTP interceptor to route slack.com to test server
	http.DefaultTransport = transport.NewSimulatorTransport(map[string]string{
		"slack.com": server.URL[7:], // Strip "http://" prefix
	})

	// Create Slack client
	client := slack.New("fake-token-12345")

	t.Run("PostMessage", func(t *testing.T) {
		// Post a message to #general
		channel, timestamp, err := client.PostMessage(
			"C001",
			slack.MsgOptionText("Hello from integration test!", false),
		)

		// Assertions
		require.NoError(t, err, "PostMessage should not return error")
		assert.Equal(t, "C001", channel, "Channel ID should match")
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

		assert.Contains(t, channelMap, "C001", "Should contain #general channel")
		assert.Equal(t, "general", channelMap["C001"].Name, "#general name should match")

		assert.Contains(t, channelMap, "C002", "Should contain #random channel")
		assert.Equal(t, "random", channelMap["C002"].Name, "#random name should match")
	})

	t.Run("GetConversationHistory", func(t *testing.T) {
		// First, post a message
		expectedText := "Test message for history"
		_, expectedTimestamp, err := client.PostMessage(
			"C001",
			slack.MsgOptionText(expectedText, false),
		)
		require.NoError(t, err, "PostMessage should succeed")

		// Get conversation history
		history, err := client.GetConversationHistory(&slack.GetConversationHistoryParameters{
			ChannelID: "C001",
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
			assert.Equal(t, "U123456", msg.User, "Message user should match")
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
		assert.Equal(t, "U123456", response.UserID, "User ID should match")
	})
}

func TestSlackSimulatorMessagePersistence(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Start simulator server
	handler := simulatorSlack.NewHandler(queries)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Install HTTP interceptor
	http.DefaultTransport = transport.NewSimulatorTransport(map[string]string{
		"slack.com": server.URL[7:],
	})

	client := slack.New("fake-token-12345")

	// Post multiple messages
	messages := []string{
		"First message",
		"Second message",
		"Third message",
	}

	for _, text := range messages {
		_, _, err := client.PostMessage("C001", slack.MsgOptionText(text, false))
		require.NoError(t, err, "PostMessage should succeed")
	}

	// Retrieve history
	history, err := client.GetConversationHistory(&slack.GetConversationHistoryParameters{
		ChannelID: "C001",
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
