package slack_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
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

// TestSeedSlackData demonstrates seeding complete initial state using SQL only
func TestSeedSlackData(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDBForSeed(t)
	ctx := context.Background()

	// Setup: Create a new session
	sessionID := "slack-seed-test-session"
	err := queries.CreateSession(ctx, sessionID)
	require.NoError(t, err, "Failed to create session")

	// Seed: Create channels, users, and messages via SQL
	channels, users := seedSlackTestData(t, ctx, queries, sessionID)
	seedSlackMessages(t, ctx, queries, sessionID, channels)

	// Verify: Check database state - ensure all data is correctly stored
	verifyDatabaseState(t, ctx, queries, sessionID, channels, users)
}

// seedSlackTestData creates channels and users for testing
func seedSlackTestData(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string) (
	channels []struct{ ID, Name string },
	users []struct {
		ID          string
		Name        string
		RealName    string
		Email       string
		DisplayName string
		FirstName   string
		LastName    string
	},
) {
	t.Helper()

	// Seed: Create custom channels (use session-specific IDs to avoid conflicts)
	channels = []struct {
		ID   string
		Name string
	}{
		{"C001_" + sessionID, "general"},
		{"C002_" + sessionID, "random"},
		{"C003_" + sessionID, "engineering"},
	}

	timestamp := int64(1640000000)
	for _, ch := range channels {
		err := queries.CreateChannel(ctx, database.CreateChannelParams{
			ID:        ch.ID,
			Name:      ch.Name,
			CreatedAt: timestamp,
			SessionID: sessionID,
		})
		require.NoError(t, err, "Failed to create channel: %s", ch.Name)
	}

	// Seed: Create custom users with different profiles (use session-specific IDs to avoid conflicts)
	users = []struct {
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
		err := queries.CreateUser(ctx, database.CreateUserParams{
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

	return channels, users
}

// seedSlackMessages creates messages via SQL
func seedSlackMessages(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string, channels []struct{ ID, Name string }) {
	t.Helper()

	// Create messages in different channels
	messages := []struct {
		text      string
		channelID string
		userID    string
		timestamp string
	}{
		{"Welcome to the workspace!", channels[0].ID, "U001_" + sessionID, "1640000100.000000"},
		{"Hello everyone!", channels[0].ID, "U002_" + sessionID, "1640000101.000000"},
		{"Engineering team standup at 10am", channels[2].ID, "U003_" + sessionID, "1640000102.000000"},
	}

	for _, msg := range messages {
		err := queries.CreateMessage(ctx, database.CreateMessageParams{
			ChannelID:   msg.channelID,
			Type:        "message",
			UserID:      msg.userID,
			Text:        msg.text,
			Timestamp:   msg.timestamp,
			Attachments: database.StringToNullString(""),
			SessionID:   sessionID,
		})
		require.NoError(t, err, "Failed to create message: %s", msg.text)
	}
}

// verifyDatabaseState verifies all seeded data exists in database
func verifyDatabaseState(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string,
	channels []struct{ ID, Name string }, users []struct {
		ID          string
		Name        string
		RealName    string
		Email       string
		DisplayName string
		FirstName   string
		LastName    string
	}) {
	t.Helper()

	// Verify channels
	dbChannels, err := queries.ListChannels(ctx, sessionID)
	require.NoError(t, err, "ListChannels should succeed")
	assert.Len(t, dbChannels, 3, "Should have 3 channels in database")

	channelNames := make(map[string]bool)
	for _, ch := range dbChannels {
		channelNames[ch.Name] = true
	}
	assert.True(t, channelNames["general"], "Should have general channel")
	assert.True(t, channelNames["random"], "Should have random channel")
	assert.True(t, channelNames["engineering"], "Should have engineering channel")

	// Verify users
	for _, u := range users {
		dbUser, err := queries.GetUserByID(ctx, database.GetUserByIDParams{
			ID:        u.ID,
			SessionID: sessionID,
		})
		require.NoError(t, err, "GetUserByID should succeed for user: %s", u.Name)
		assert.Equal(t, u.Name, dbUser.Name, "User name should match in database")
		assert.Equal(t, u.RealName, dbUser.RealName, "Real name should match in database")
	}

	// Verify messages in general channel
	dbMessages, err := queries.GetMessagesByChannel(ctx, database.GetMessagesByChannelParams{
		ChannelID: channels[0].ID,
		SessionID: sessionID,
	})
	require.NoError(t, err, "GetMessagesByChannel should succeed")
	assert.GreaterOrEqual(t, len(dbMessages), 2, "Should have at least 2 messages in general channel")

	// Verify specific message content
	found := false
	for _, m := range dbMessages {
		if m.Text == "Welcome to the workspace!" {
			found = true
			break
		}
	}
	assert.True(t, found, "Welcome message should be in database")

	// Verify engineering channel messages
	dbEngMessages, err := queries.GetMessagesByChannel(ctx, database.GetMessagesByChannelParams{
		ChannelID: channels[2].ID,
		SessionID: sessionID,
	})
	require.NoError(t, err, "GetMessagesByChannel should succeed for engineering")
	assert.GreaterOrEqual(t, len(dbEngMessages), 1, "Should have at least 1 message in engineering channel")
}
