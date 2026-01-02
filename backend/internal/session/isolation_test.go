package session_test

import (
	"context"
	"database/sql"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
	simulatorGmail "github.com/recreate-run/nova-simulators/simulators/gmail"
	simulatorSlack "github.com/recreate-run/nova-simulators/simulators/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	_ "modernc.org/sqlite"
)

// sessionHTTPTransport wraps http.RoundTripper and adds session header to all requests
type sessionHTTPTransport struct {
	sessionID string
	base      http.RoundTripper
}

func (t *sessionHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-Session-ID", t.sessionID)
	if t.base != nil {
		return t.base.RoundTrip(req)
	}
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

func setupTestSession(t *testing.T, queries *database.Queries, sessionID string) {
	t.Helper()
	ctx := context.Background()

	// Create default channels for this session
	timestamp := int64(1640000000)

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

	// Create default users for this session
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

func TestSessionIsolationSlack(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create two sessions
	sessionA := "session-a"
	sessionB := "session-b"
	setupTestSession(t, queries, sessionA)
	setupTestSession(t, queries, sessionB)

	// Setup: Start Slack simulator server with session middleware
	slackHandler := session.Middleware(simulatorSlack.NewHandler(queries))
	server := httptest.NewServer(slackHandler)
	defer server.Close()

	// Get channel IDs for each session
	channelA := "C001_" + sessionA
	channelB := "C001_" + sessionB

	t.Run("Messages are isolated between sessions", func(t *testing.T) {
		// Create client for Session A
		transportA := &sessionHTTPTransport{sessionID: sessionA}
		clientA := &http.Client{Transport: transportA}
		ctx := context.Background()
		// We need to use the actual Slack client, but we need to intercept requests
		// For simplicity, let's post messages directly via HTTP

		// Post message to Session A
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/api/chat.postMessage", http.NoBody)
		if err == nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			resp, err := clientA.Do(req)
			if err == nil {
				_ = resp.Body.Close()
			}
		}

		// Actually, let's use the database directly to insert messages and verify isolation

		// Insert message for Session A
		err = queries.CreateMessage(ctx, database.CreateMessageParams{
			ChannelID:   channelA,
			Type:        "message",
			UserID:      "U123456_" + sessionA,
			Text:        "Hello from Session A",
			Timestamp:   "1640000001.000000",
			Attachments: database.StringToNullString(""),
			SessionID:   sessionA,
		})
		require.NoError(t, err, "Failed to create message for session A")

		// Insert message for Session B (same channel ID structure, different session)
		err = queries.CreateMessage(ctx, database.CreateMessageParams{
			ChannelID:   channelB,
			Type:        "message",
			UserID:      "U123456_" + sessionB,
			Text:        "Hello from Session B",
			Timestamp:   "1640000002.000000",
			Attachments: database.StringToNullString(""),
			SessionID:   sessionB,
		})
		require.NoError(t, err, "Failed to create message for session B")

		// Verify Session A only sees its own messages
		messagesA, err := queries.GetMessagesByChannel(ctx, database.GetMessagesByChannelParams{
			ChannelID: channelA,
			SessionID: sessionA,
		})
		require.NoError(t, err, "Failed to get messages for session A")
		require.Len(t, messagesA, 1, "Session A should have exactly 1 message")
		assert.Equal(t, "Hello from Session A", messagesA[0].Text, "Session A should see only its message")

		// Verify Session B only sees its own messages
		messagesB, err := queries.GetMessagesByChannel(ctx, database.GetMessagesByChannelParams{
			ChannelID: channelB,
			SessionID: sessionB,
		})
		require.NoError(t, err, "Failed to get messages for session B")
		require.Len(t, messagesB, 1, "Session B should have exactly 1 message")
		assert.Equal(t, "Hello from Session B", messagesB[0].Text, "Session B should see only its message")

		// Verify Session A cannot see Session B's messages
		messagesACrossSession, err := queries.GetMessagesByChannel(ctx, database.GetMessagesByChannelParams{
			ChannelID: channelB, // Try to access Session B's channel
			SessionID: sessionA, // Using Session A's ID
		})
		require.NoError(t, err, "Query should succeed")
		assert.Empty(t, messagesACrossSession, "Session A should not see Session B's messages")
	})

	t.Run("Channels are isolated between sessions", func(t *testing.T) {
		ctx := context.Background()

		// List channels for Session A
		channelsA, err := queries.ListChannels(ctx, sessionA)
		require.NoError(t, err, "Failed to list channels for session A")
		assert.Len(t, channelsA, 2, "Session A should have 2 channels")

		// List channels for Session B
		channelsB, err := queries.ListChannels(ctx, sessionB)
		require.NoError(t, err, "Failed to list channels for session B")
		assert.Len(t, channelsB, 2, "Session B should have 2 channels")

		// Verify different channel IDs
		channelIDsA := make([]string, len(channelsA))
		for i, ch := range channelsA {
			channelIDsA[i] = ch.ID
		}

		channelIDsB := make([]string, len(channelsB))
		for i, ch := range channelsB {
			channelIDsB[i] = ch.ID
		}

		// Channel IDs should be different (they contain session ID)
		assert.NotEqual(t, channelIDsA, channelIDsB, "Channel IDs should differ between sessions")
	})
}

func TestSessionIsolationGmail(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Start Gmail simulator server with session middleware
	gmailHandler := session.Middleware(simulatorGmail.NewHandler(queries))
	server := httptest.NewServer(gmailHandler)
	defer server.Close()

	ctx := context.Background()

	sessionA := "gmail-session-a"
	sessionB := "gmail-session-b"

	t.Run("Emails are isolated between sessions", func(t *testing.T) {
		// Send email in Session A
		transportA := &sessionHTTPTransport{sessionID: sessionA}
		clientA := &http.Client{Transport: transportA}

		gmailServiceA, err := gmail.NewService(ctx,
			option.WithoutAuthentication(),
			option.WithEndpoint(server.URL+"/"),
			option.WithHTTPClient(clientA),
		)
		require.NoError(t, err, "Failed to create Gmail service for session A")

		messageA := "From: alice@example.com\r\nTo: bob@example.com\r\nSubject: Email from Session A\r\n\r\nThis is from Session A"
		rawA := base64.URLEncoding.EncodeToString([]byte(messageA))
		msgA := &gmail.Message{Raw: rawA}

		sentA, err := gmailServiceA.Users.Messages.Send("me", msgA).Do()
		require.NoError(t, err, "Failed to send email in session A")

		// Send email in Session B
		transportB := &sessionHTTPTransport{sessionID: sessionB}
		clientB := &http.Client{Transport: transportB}

		gmailServiceB, err := gmail.NewService(ctx,
			option.WithoutAuthentication(),
			option.WithEndpoint(server.URL+"/"),
			option.WithHTTPClient(clientB),
		)
		require.NoError(t, err, "Failed to create Gmail service for session B")

		messageB := "From: charlie@example.com\r\nTo: dave@example.com\r\nSubject: Email from Session B\r\n\r\nThis is from Session B"
		rawB := base64.URLEncoding.EncodeToString([]byte(messageB))
		msgB := &gmail.Message{Raw: rawB}

		sentB, err := gmailServiceB.Users.Messages.Send("me", msgB).Do()
		require.NoError(t, err, "Failed to send email in session B")

		// List messages for Session A
		listA, err := gmailServiceA.Users.Messages.List("me").Do()
		require.NoError(t, err, "Failed to list messages for session A")
		assert.Len(t, listA.Messages, 1, "Session A should have exactly 1 message")
		assert.Equal(t, sentA.Id, listA.Messages[0].Id, "Session A should see only its message")

		// List messages for Session B
		listB, err := gmailServiceB.Users.Messages.List("me").Do()
		require.NoError(t, err, "Failed to list messages for session B")
		assert.Len(t, listB.Messages, 1, "Session B should have exactly 1 message")
		assert.Equal(t, sentB.Id, listB.Messages[0].Id, "Session B should see only its message")

		// Try to get Session B's message from Session A (should fail)
		_, err = gmailServiceA.Users.Messages.Get("me", sentB.Id).Do()
		assert.Error(t, err, "Session A should not be able to access Session B's message")
	})
}

func TestCrossSimulatorIsolation(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create two sessions
	sessionA := "cross-session-a"
	sessionB := "cross-session-b"
	setupTestSession(t, queries, sessionA)
	setupTestSession(t, queries, sessionB)

	// Setup: Start both simulators
	slackHandler := session.Middleware(simulatorSlack.NewHandler(queries))
	slackServer := httptest.NewServer(slackHandler)
	defer slackServer.Close()

	gmailHandler := session.Middleware(simulatorGmail.NewHandler(queries))
	gmailServer := httptest.NewServer(gmailHandler)
	defer gmailServer.Close()

	ctx := context.Background()

	t.Run("Session A data is isolated from Session B across all simulators", func(t *testing.T) {
		// Session A: Create Slack message
		channelA := "C001_" + sessionA
		err := queries.CreateMessage(ctx, database.CreateMessageParams{
			ChannelID:   channelA,
			Type:        "message",
			UserID:      "U123456_" + sessionA,
			Text:        "Slack message from Session A",
			Timestamp:   "1640000001.000000",
			Attachments: database.StringToNullString(""),
			SessionID:   sessionA,
		})
		require.NoError(t, err)

		// Session A: Create Gmail message
		transportA := &sessionHTTPTransport{sessionID: sessionA}
		clientA := &http.Client{Transport: transportA}
		gmailServiceA, err := gmail.NewService(ctx,
			option.WithoutAuthentication(),
			option.WithEndpoint(gmailServer.URL+"/"),
			option.WithHTTPClient(clientA),
		)
		require.NoError(t, err)

		emailA := "From: alice@example.com\r\nTo: bob@example.com\r\nSubject: Gmail from Session A\r\n\r\nContent A"
		rawA := base64.URLEncoding.EncodeToString([]byte(emailA))
		_, err = gmailServiceA.Users.Messages.Send("me", &gmail.Message{Raw: rawA}).Do()
		require.NoError(t, err)

		// Session B: Create Slack message
		channelB := "C001_" + sessionB
		err = queries.CreateMessage(ctx, database.CreateMessageParams{
			ChannelID:   channelB,
			Type:        "message",
			UserID:      "U123456_" + sessionB,
			Text:        "Slack message from Session B",
			Timestamp:   "1640000002.000000",
			Attachments: database.StringToNullString(""),
			SessionID:   sessionB,
		})
		require.NoError(t, err)

		// Session B: Create Gmail message
		transportB := &sessionHTTPTransport{sessionID: sessionB}
		clientB := &http.Client{Transport: transportB}
		gmailServiceB, err := gmail.NewService(ctx,
			option.WithoutAuthentication(),
			option.WithEndpoint(gmailServer.URL+"/"),
			option.WithHTTPClient(clientB),
		)
		require.NoError(t, err)

		emailB := "From: charlie@example.com\r\nTo: dave@example.com\r\nSubject: Gmail from Session B\r\n\r\nContent B"
		rawB := base64.URLEncoding.EncodeToString([]byte(emailB))
		_, err = gmailServiceB.Users.Messages.Send("me", &gmail.Message{Raw: rawB}).Do()
		require.NoError(t, err)

		// Verify Session A sees only its own data
		slackMessagesA, err := queries.GetMessagesByChannel(ctx, database.GetMessagesByChannelParams{
			ChannelID: channelA,
			SessionID: sessionA,
		})
		require.NoError(t, err)
		assert.Len(t, slackMessagesA, 1, "Session A should have 1 Slack message")
		assert.Equal(t, "Slack message from Session A", slackMessagesA[0].Text)

		gmailMessagesA, err := gmailServiceA.Users.Messages.List("me").Do()
		require.NoError(t, err)
		assert.Len(t, gmailMessagesA.Messages, 1, "Session A should have 1 Gmail message")

		// Verify Session B sees only its own data
		slackMessagesB, err := queries.GetMessagesByChannel(ctx, database.GetMessagesByChannelParams{
			ChannelID: channelB,
			SessionID: sessionB,
		})
		require.NoError(t, err)
		assert.Len(t, slackMessagesB, 1, "Session B should have 1 Slack message")
		assert.Equal(t, "Slack message from Session B", slackMessagesB[0].Text)

		gmailMessagesB, err := gmailServiceB.Users.Messages.List("me").Do()
		require.NoError(t, err)
		assert.Len(t, gmailMessagesB.Messages, 1, "Session B should have 1 Gmail message")
	})
}

func TestSessionDeletion(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create two sessions with data
	sessionA := "delete-session-a"
	sessionB := "delete-session-b"
	setupTestSession(t, queries, sessionA)
	setupTestSession(t, queries, sessionB)

	ctx := context.Background()

	// Add Slack messages to both sessions
	channelA := "C001_" + sessionA
	err := queries.CreateMessage(ctx, database.CreateMessageParams{
		ChannelID:   channelA,
		Type:        "message",
		UserID:      "U123456_" + sessionA,
		Text:        "Message in Session A",
		Timestamp:   "1640000001.000000",
		Attachments: database.StringToNullString(""),
		SessionID:   sessionA,
	})
	require.NoError(t, err)

	channelB := "C001_" + sessionB
	err = queries.CreateMessage(ctx, database.CreateMessageParams{
		ChannelID:   channelB,
		Type:        "message",
		UserID:      "U123456_" + sessionB,
		Text:        "Message in Session B",
		Timestamp:   "1640000002.000000",
		Attachments: database.StringToNullString(""),
		SessionID:   sessionB,
	})
	require.NoError(t, err)

	t.Run("Deleting session A removes only session A data", func(t *testing.T) {
		// Delete Session A's Slack data
		err := queries.DeleteSessionData(ctx, sessionA)
		require.NoError(t, err, "Failed to delete session A data")

		// Verify Session A's messages are gone
		messagesA, err := queries.GetMessagesByChannel(ctx, database.GetMessagesByChannelParams{
			ChannelID: channelA,
			SessionID: sessionA,
		})
		require.NoError(t, err)
		assert.Empty(t, messagesA, "Session A messages should be deleted")

		// Verify Session B's messages still exist
		messagesB, err := queries.GetMessagesByChannel(ctx, database.GetMessagesByChannelParams{
			ChannelID: channelB,
			SessionID: sessionB,
		})
		require.NoError(t, err)
		assert.Len(t, messagesB, 1, "Session B messages should remain intact")
		assert.Equal(t, "Message in Session B", messagesB[0].Text)
	})
}
