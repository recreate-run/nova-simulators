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

// TestSlackInitialStateSeed demonstrates seeding arbitrary initial state for Slack simulator
func TestSlackInitialStateSeed(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDBForSeed(t)
	ctx := context.Background()

	// Setup: Create a new session
	sessionID := "slack-seed-test-session"
	err := queries.CreateSession(ctx, sessionID)
	require.NoError(t, err, "Failed to create session")

	// Seed: Create custom channels (use session-specific IDs to avoid conflicts)
	channels := []struct {
		ID   string
		Name string
	}{
		{"C001_" + sessionID, "general"},
		{"C002_" + sessionID, "random"},
		{"C003_" + sessionID, "engineering"},
	}

	timestamp := int64(1640000000)
	for _, ch := range channels {
		err = queries.CreateChannel(ctx, database.CreateChannelParams{
			ID:        ch.ID,
			Name:      ch.Name,
			CreatedAt: timestamp,
			SessionID: sessionID,
		})
		require.NoError(t, err, "Failed to create channel: %s", ch.Name)
	}

	// Seed: Create custom users with different profiles (use session-specific IDs to avoid conflicts)
	users := []struct {
		ID          string
		Name        string
		RealName    string
		Email       string
		DisplayName string
		FirstName   string
		LastName    string
	}{
		{
			ID:          "U001_" + sessionID,
			Name:        "alice",
			RealName:    "Alice Johnson",
			Email:       "alice@example.com",
			DisplayName: "alice.j",
			FirstName:   "Alice",
			LastName:    "Johnson",
		},
		{
			ID:          "U002_" + sessionID,
			Name:        "bob",
			RealName:    "Bob Smith",
			Email:       "bob@example.com",
			DisplayName: "bsmith",
			FirstName:   "Bob",
			LastName:    "Smith",
		},
		{
			ID:          "U003_" + sessionID,
			Name:        "charlie",
			RealName:    "Charlie Brown",
			Email:       "charlie@example.com",
			DisplayName: "cbrown",
			FirstName:   "Charlie",
			LastName:    "Brown",
		},
	}

	for _, u := range users {
		err = queries.CreateUser(ctx, database.CreateUserParams{
			ID:              u.ID,
			TeamID:          "T021F9ZE2",
			Name:            u.Name,
			RealName:        u.RealName,
			Email:           database.StringToNullString(u.Email),
			DisplayName:     database.StringToNullString(u.DisplayName),
			FirstName:       database.StringToNullString(u.FirstName),
			LastName:        database.StringToNullString(u.LastName),
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
		require.NoError(t, err, "Failed to create user: %s", u.Name)
	}

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

	// Seed: Post initial messages to channels using Slack API
	t.Run("PostInitialMessages", func(t *testing.T) {
		// Post welcome message to general
		_, ts1, err := client.PostMessage(
			channels[0].ID,
			slack.MsgOptionText("Welcome to the workspace!", false),
		)
		require.NoError(t, err, "Failed to post message to general")
		assert.NotEmpty(t, ts1, "Message timestamp should be returned")

		// Post message to engineering
		_, ts2, err := client.PostMessage(
			channels[2].ID,
			slack.MsgOptionText("Engineering team standup at 10am", false),
		)
		require.NoError(t, err, "Failed to post message to engineering")
		assert.NotEmpty(t, ts2, "Message timestamp should be returned")
	})

	// Verify: Check that channels are queryable
	t.Run("VerifyChannels", func(t *testing.T) {
		channelsList, _, err := client.GetConversations(&slack.GetConversationsParameters{
			ExcludeArchived: true,
			Types:           []string{"public_channel", "private_channel"},
		})

		require.NoError(t, err, "GetConversations should succeed")
		assert.Len(t, channelsList, 3, "Should have 3 channels")

		// Verify channel names
		channelMap := make(map[string]slack.Channel)
		for _, ch := range channelsList {
			channelMap[ch.Name] = ch
		}

		assert.Contains(t, channelMap, "general", "Should have general channel")
		assert.Contains(t, channelMap, "random", "Should have random channel")
		assert.Contains(t, channelMap, "engineering", "Should have engineering channel")
	})

	// Verify: Check that users are queryable
	t.Run("VerifyUsers", func(t *testing.T) {
		// Get user info for each user
		for _, u := range users {
			user, err := client.GetUserInfo(u.ID)
			require.NoError(t, err, "GetUserInfo should succeed for user: %s", u.Name)
			assert.Equal(t, u.ID, user.ID, "User ID should match")
			assert.Equal(t, u.Name, user.Name, "User name should match")
			assert.Equal(t, u.RealName, user.RealName, "Real name should match")
			assert.Equal(t, u.Email, user.Profile.Email, "Email should match")
		}
	})

	// Verify: Check that messages are queryable
	t.Run("VerifyMessages", func(t *testing.T) {
		// Get conversation history for general
		history, err := client.GetConversationHistory(&slack.GetConversationHistoryParameters{
			ChannelID: channels[0].ID,
			Limit:     10,
		})
		require.NoError(t, err, "GetConversationHistory should succeed")
		assert.GreaterOrEqual(t, len(history.Messages), 1, "Should have at least 1 message in general")

		// Verify message content
		found := false
		for _, msg := range history.Messages {
			if msg.Text == "Welcome to the workspace!" {
				found = true
				break
			}
		}
		assert.True(t, found, "Welcome message should be in history")
	})

	// Verify: Check database isolation - ensure all data is correctly stored
	t.Run("VerifyDatabaseIsolation", func(t *testing.T) {
		// Query channels from database
		dbChannels, err := queries.ListChannels(ctx, sessionID)
		require.NoError(t, err, "ListChannels should succeed")
		assert.Len(t, dbChannels, 3, "Should have 3 channels in database")

		// Verify channel names
		channelNames := make(map[string]bool)
		for _, ch := range dbChannels {
			channelNames[ch.Name] = true
		}
		assert.True(t, channelNames["general"], "Should have general channel")
		assert.True(t, channelNames["random"], "Should have random channel")
		assert.True(t, channelNames["engineering"], "Should have engineering channel")

		// Query users from database
		for _, u := range users {
			dbUser, err := queries.GetUserByID(ctx, database.GetUserByIDParams{
				ID:        u.ID,
				SessionID: sessionID,
			})
			require.NoError(t, err, "GetUserByID should succeed for user: %s", u.Name)
			assert.Equal(t, u.Name, dbUser.Name, "User name should match in database")
		}

		// Query messages from database
		dbMessages, err := queries.GetMessagesByChannel(ctx, database.GetMessagesByChannelParams{
			ChannelID: channels[0].ID,
			SessionID: sessionID,
		})
		require.NoError(t, err, "GetMessagesByChannel should succeed")
		assert.GreaterOrEqual(t, len(dbMessages), 1, "Should have at least 1 message in database")

		// Verify message content
		found := false
		for _, m := range dbMessages {
			if m.Text == "Welcome to the workspace!" {
				found = true
				break
			}
		}
		assert.True(t, found, "Welcome message should be in database")
	})
}
