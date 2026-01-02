package whatsapp_test

import (
	"context"
	"testing"

	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getTestMessages returns test message configurations
func getTestMessages() []struct {
	Type        string
	To          string
	Description string
	SendFunc    func(*Client) (*MessageResponse, error)
} {
	return []struct {
		Type        string
		To          string
		Description string
		SendFunc    func(*Client) (*MessageResponse, error)
	}{
		{
			Type:        "text",
			To:          "+14155551234",
			Description: "Welcome message to new customer",
			SendFunc: func(client *Client) (*MessageResponse, error) {
				return client.SendMessage("+14155551234", "Welcome to our service! We're here to help.")
			},
		},
		{
			Type:        "text",
			To:          "+14155555678",
			Description: "Order confirmation",
			SendFunc: func(client *Client) (*MessageResponse, error) {
				return client.SendMessage("+14155555678", "Your order #12345 has been confirmed and will be delivered tomorrow.")
			},
		},
		{
			Type:        "template",
			To:          "+14155559999",
			Description: "Welcome template",
			SendFunc: func(client *Client) (*MessageResponse, error) {
				return client.SendTemplate("+14155559999", "welcome_message", "en_US")
			},
		},
		{
			Type:        "template",
			To:          "+14155552222",
			Description: "Order shipped template",
			SendFunc: func(client *Client) (*MessageResponse, error) {
				return client.SendTemplate("+14155552222", "order_shipped", "en")
			},
		},
		{
			Type:        "image",
			To:          "+14155553333",
			Description: "Product image",
			SendFunc: func(client *Client) (*MessageResponse, error) {
				return client.SendImage("+14155553333", "https://example.com/product-123.jpg", "Check out our new product!")
			},
		},
		{
			Type:        "document",
			To:          "+14155554444",
			Description: "Invoice document",
			SendFunc: func(client *Client) (*MessageResponse, error) {
				return client.SendDocument("+14155554444", "https://example.com/invoice-789.pdf", "Your invoice is attached")
			},
		},
		{
			Type:        "video",
			To:          "+14155556666",
			Description: "Tutorial video",
			SendFunc: func(client *Client) (*MessageResponse, error) {
				return client.SendVideo("+14155556666", "https://example.com/tutorial.mp4", "Learn how to use our app")
			},
		},
		{
			Type:        "image",
			To:          "+14155557777",
			Description: "Promotional banner",
			SendFunc: func(client *Client) (*MessageResponse, error) {
				return client.SendImage("+14155557777", "https://example.com/promo-banner.png", "50% off this weekend!")
			},
		},
	}
}

// sendInitialMessages sends all test messages and returns their IDs
func sendInitialMessages(t *testing.T, client *Client, messages []struct {
	Type        string
	To          string
	Description string
	SendFunc    func(*Client) (*MessageResponse, error)
}) []string {
	t.Helper()
	sentIDs := make([]string, 0, len(messages))

	for i := range messages {
		msg := messages[i]
		resp, err := msg.SendFunc(client)
		require.NoError(t, err, "Failed to send message %d: %s", i+1, msg.Description)
		assert.NotNil(t, resp, "Response should not be nil")
		assert.Equal(t, "whatsapp", resp.MessagingProduct, "Should be WhatsApp messaging product")
		assert.Len(t, resp.Messages, 1, "Should have one message")
		assert.NotEmpty(t, resp.Messages[0].ID, "Message ID should not be empty")
		assert.Len(t, resp.Contacts, 1, "Should have one contact")
		assert.Equal(t, msg.To, resp.Contacts[0].Input, "Contact input should match")
		assert.Equal(t, msg.To, resp.Contacts[0].WaID, "WhatsApp ID should match")

		sentIDs = append(sentIDs, resp.Messages[0].ID)
	}

	return sentIDs
}

// verifyMessagesList verifies messages appear in database
func verifyMessagesList(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string, sentIDs []string) {
	t.Helper()
	dbMessages, err := queries.ListWhatsAppMessages(ctx, database.ListWhatsAppMessagesParams{
		SessionID: sessionID,
		Limit:     100,
	})
	require.NoError(t, err, "List should succeed")
	assert.Len(t, dbMessages, 8, "Should have 8 messages")

	messageIDs := make(map[string]bool)
	for i := range dbMessages {
		m := dbMessages[i]
		messageIDs[m.ID] = true
	}

	for _, sentID := range sentIDs {
		assert.True(t, messageIDs[sentID], "Sent message %s should appear in database", sentID)
	}
}

// verifyMessageRetrieval verifies individual message retrieval
func verifyMessageRetrieval(t *testing.T, ctx context.Context, queries *database.Queries, sessionID, phoneNumberID string, sentIDs []string, messages []struct {
	Type        string
	To          string
	Description string
	SendFunc    func(*Client) (*MessageResponse, error)
}) {
	t.Helper()
	for i, sentID := range sentIDs {
		retrieved, err := queries.GetWhatsAppMessageByID(ctx, database.GetWhatsAppMessageByIDParams{
			ID:        sentID,
			SessionID: sessionID,
		})
		require.NoError(t, err, "Get should succeed for message: %s", sentID)
		assert.Equal(t, sentID, retrieved.ID, "Message ID should match")
		assert.Equal(t, phoneNumberID, retrieved.PhoneNumberID, "Phone number ID should match")
		assert.Equal(t, messages[i].To, retrieved.ToNumber, "To number should match")
		assert.Equal(t, messages[i].Type, retrieved.MessageType, "Message type should match")
	}
}

// verifyMessageTypeScenarios verifies different message type scenarios
func verifyMessageTypeScenarios(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string, sentIDs []string) {
	t.Helper()

	// Scenario 1: Text message
	retrieved, err := queries.GetWhatsAppMessageByID(ctx, database.GetWhatsAppMessageByIDParams{
		ID:        sentIDs[0],
		SessionID: sessionID,
	})
	require.NoError(t, err, "Should retrieve text message")
	assert.Equal(t, "text", retrieved.MessageType)
	assert.True(t, retrieved.TextBody.Valid, "Text body should be present")
	assert.Contains(t, retrieved.TextBody.String, "Welcome", "Text should contain 'Welcome'")

	// Scenario 2: Template message
	retrieved2, err := queries.GetWhatsAppMessageByID(ctx, database.GetWhatsAppMessageByIDParams{
		ID:        sentIDs[2],
		SessionID: sessionID,
	})
	require.NoError(t, err, "Should retrieve template message")
	assert.Equal(t, "template", retrieved2.MessageType)
	assert.True(t, retrieved2.TemplateName.Valid, "Template name should be present")
	assert.Equal(t, "welcome_message", retrieved2.TemplateName.String)
	assert.True(t, retrieved2.LanguageCode.Valid, "Language code should be present")
	assert.Equal(t, "en_US", retrieved2.LanguageCode.String)

	// Scenario 3: Image message
	retrieved3, err := queries.GetWhatsAppMessageByID(ctx, database.GetWhatsAppMessageByIDParams{
		ID:        sentIDs[4],
		SessionID: sessionID,
	})
	require.NoError(t, err, "Should retrieve image message")
	assert.Equal(t, "image", retrieved3.MessageType)
	assert.True(t, retrieved3.MediaUrl.Valid, "Media URL should be present")
	assert.Contains(t, retrieved3.MediaUrl.String, "product-123.jpg")
	assert.True(t, retrieved3.Caption.Valid, "Caption should be present")
	assert.Contains(t, retrieved3.Caption.String, "product")

	// Scenario 4: Document message
	retrieved4, err := queries.GetWhatsAppMessageByID(ctx, database.GetWhatsAppMessageByIDParams{
		ID:        sentIDs[5],
		SessionID: sessionID,
	})
	require.NoError(t, err, "Should retrieve document message")
	assert.Equal(t, "document", retrieved4.MessageType)
	assert.True(t, retrieved4.MediaUrl.Valid, "Media URL should be present")
	assert.Contains(t, retrieved4.MediaUrl.String, "invoice-789.pdf")
	assert.True(t, retrieved4.Caption.Valid, "Caption should be present")

	// Scenario 5: Video message
	retrieved5, err := queries.GetWhatsAppMessageByID(ctx, database.GetWhatsAppMessageByIDParams{
		ID:        sentIDs[6],
		SessionID: sessionID,
	})
	require.NoError(t, err, "Should retrieve video message")
	assert.Equal(t, "video", retrieved5.MessageType)
	assert.True(t, retrieved5.MediaUrl.Valid, "Media URL should be present")
	assert.Contains(t, retrieved5.MediaUrl.String, "tutorial.mp4")
}

// verifyWhatsAppDatabaseIsolation verifies database isolation
func verifyWhatsAppDatabaseIsolation(t *testing.T, ctx context.Context, queries *database.Queries, sessionID, phoneNumberID string) {
	t.Helper()

	dbMessages, err := queries.ListWhatsAppMessages(ctx, database.ListWhatsAppMessagesParams{
		SessionID: sessionID,
		Limit:     100,
	})
	require.NoError(t, err, "ListWhatsAppMessages should succeed")
	assert.Len(t, dbMessages, 8, "Should have 8 messages in database")

	for i := range dbMessages {
		m := dbMessages[i]
		fullMsg, err := queries.GetWhatsAppMessageByID(ctx, database.GetWhatsAppMessageByIDParams{
			ID:        m.ID,
			SessionID: sessionID,
		})
		require.NoError(t, err, "GetWhatsAppMessageByID should succeed")
		assert.Equal(t, m.ID, fullMsg.ID, "Message ID should match")
		assert.Equal(t, phoneNumberID, fullMsg.PhoneNumberID, "Phone number ID should match")
	}
}

// verifyWhatsAppPagination verifies pagination
func verifyWhatsAppPagination(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string) {
	t.Helper()

	messages1, err := queries.ListWhatsAppMessages(ctx, database.ListWhatsAppMessagesParams{
		SessionID: sessionID,
		Limit:     3,
	})
	require.NoError(t, err, "List with limit should succeed")
	assert.Len(t, messages1, 3, "Should return 3 messages")

	allMessages, err := queries.ListWhatsAppMessages(ctx, database.ListWhatsAppMessagesParams{
		SessionID: sessionID,
		Limit:     100,
	})
	require.NoError(t, err, "List all should succeed")

	for i := 0; i < len(allMessages)-1; i++ {
		assert.GreaterOrEqual(t, allMessages[i].CreatedAt, allMessages[i+1].CreatedAt,
			"Messages should be ordered by created_at DESC")
	}
}

// verifyRecipientVariety verifies unique recipients
func verifyRecipientVariety(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string) {
	t.Helper()

	dbMessages, err := queries.ListWhatsAppMessages(ctx, database.ListWhatsAppMessagesParams{
		SessionID: sessionID,
		Limit:     100,
	})
	require.NoError(t, err, "List should succeed")

	recipients := make(map[string]bool)
	for i := range dbMessages {
		m := dbMessages[i]
		recipients[m.ToNumber] = true
	}
	assert.Len(t, recipients, 8, "Should have 8 unique recipients")
}
