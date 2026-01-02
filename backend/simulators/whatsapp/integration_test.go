package whatsapp_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
	simulatorWhatsApp "github.com/recreate-run/nova-simulators/simulators/whatsapp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// WhatsApp client structures (replicate from workflow blocks)
type TextMessage struct {
	MessagingProduct string      `json:"messaging_product"`
	RecipientType    string      `json:"recipient_type"`
	To               string      `json:"to"`
	Type             string      `json:"type"`
	Text             TextContent `json:"text"`
}

type TextContent struct {
	Body string `json:"body"`
}

type TemplateMessage struct {
	MessagingProduct string   `json:"messaging_product"`
	To               string   `json:"to"`
	Type             string   `json:"type"`
	Template         Template `json:"template"`
}

type Template struct {
	Name     string   `json:"name"`
	Language Language `json:"language"`
}

type Language struct {
	Code string `json:"code"`
}

type MediaMessage struct {
	MessagingProduct string       `json:"messaging_product"`
	RecipientType    string       `json:"recipient_type"`
	To               string       `json:"to"`
	Type             string       `json:"type"`
	Image            *MediaObject `json:"image,omitempty"`
	Document         *MediaObject `json:"document,omitempty"`
	Video            *MediaObject `json:"video,omitempty"`
}

type MediaObject struct {
	Link    string `json:"link,omitempty"`
	Caption string `json:"caption,omitempty"`
}

type MessageResponse struct {
	MessagingProduct string `json:"messaging_product"`
	Contacts         []struct {
		Input string `json:"input"`
		WaID  string `json:"wa_id"`
	} `json:"contacts"`
	Messages []struct {
		ID string `json:"id"`
	} `json:"messages"`
}

// Simple WhatsApp client for testing
type Client struct {
	phoneNumberID string
	accessToken   string
	httpClient    *http.Client
	baseURL       string
}

func NewClient(phoneNumberID, accessToken, baseURL string) *Client {
	return &Client{
		phoneNumberID: phoneNumberID,
		accessToken:   accessToken,
		httpClient:    &http.Client{},
		baseURL:       baseURL,
	}
}

func (c *Client) sendRequest(payload interface{}) (*MessageResponse, error) {
	url := fmt.Sprintf("%s/v21.0/%s/messages", c.baseURL, c.phoneNumberID)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var msgResponse MessageResponse
	if err := json.Unmarshal(body, &msgResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &msgResponse, nil
}

func (c *Client) SendMessage(to, text string) (*MessageResponse, error) {
	payload := TextMessage{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               to,
		Type:             "text",
		Text: TextContent{
			Body: text,
		},
	}

	return c.sendRequest(payload)
}

func (c *Client) SendTemplate(to, templateName, languageCode string) (*MessageResponse, error) {
	payload := TemplateMessage{
		MessagingProduct: "whatsapp",
		To:               to,
		Type:             "template",
		Template: Template{
			Name: templateName,
			Language: Language{
				Code: languageCode,
			},
		},
	}

	return c.sendRequest(payload)
}

func (c *Client) SendImage(to, imageURL, caption string) (*MessageResponse, error) {
	payload := MediaMessage{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               to,
		Type:             "image",
		Image: &MediaObject{
			Link:    imageURL,
			Caption: caption,
		},
	}

	return c.sendRequest(payload)
}

func (c *Client) SendDocument(to, documentURL, caption string) (*MessageResponse, error) {
	payload := MediaMessage{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               to,
		Type:             "document",
		Document: &MediaObject{
			Link:    documentURL,
			Caption: caption,
		},
	}

	return c.sendRequest(payload)
}

func (c *Client) SendVideo(to, videoURL, caption string) (*MessageResponse, error) {
	payload := MediaMessage{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               to,
		Type:             "video",
		Video: &MediaObject{
			Link:    videoURL,
			Caption: caption,
		},
	}

	return c.sendRequest(payload)
}

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

func TestWhatsAppSimulatorSendTextMessage(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "whatsapp-test-session-1"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorWhatsApp.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client that adds session header
	transport := &sessionHTTPTransport{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create WhatsApp client pointing to test server
	client := NewClient("123456789", "test-token", server.URL)
	client.httpClient = customClient

	t.Run("SendTextMessage", func(t *testing.T) {
		// Send text message
		resp, err := client.SendMessage("+1234567890", "Hello, World!")

		// Assertions
		require.NoError(t, err, "Send should not return error")
		assert.NotNil(t, resp, "Should return response")
		assert.Equal(t, "whatsapp", resp.MessagingProduct)
		assert.Len(t, resp.Messages, 1, "Should have one message")
		assert.NotEmpty(t, resp.Messages[0].ID, "Message ID should not be empty")
		assert.Len(t, resp.Contacts, 1, "Should have one contact")
		assert.Equal(t, "+1234567890", resp.Contacts[0].Input)
		assert.Equal(t, "+1234567890", resp.Contacts[0].WaID)
	})
}

func TestWhatsAppSimulatorSendTemplateMessage(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "whatsapp-test-session-2"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorWhatsApp.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client that adds session header
	transport := &sessionHTTPTransport{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create WhatsApp client pointing to test server
	client := NewClient("123456789", "test-token", server.URL)
	client.httpClient = customClient

	t.Run("SendTemplateMessage", func(t *testing.T) {
		// Send template message
		resp, err := client.SendTemplate("+1234567890", "welcome_message", "en_US")

		// Assertions
		require.NoError(t, err, "Send should not return error")
		assert.NotNil(t, resp, "Should return response")
		assert.Equal(t, "whatsapp", resp.MessagingProduct)
		assert.Len(t, resp.Messages, 1, "Should have one message")
		assert.NotEmpty(t, resp.Messages[0].ID, "Message ID should not be empty")
		assert.Len(t, resp.Contacts, 1, "Should have one contact")
		assert.Equal(t, "+1234567890", resp.Contacts[0].Input)
	})
}

func TestWhatsAppSimulatorSendImageMessage(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "whatsapp-test-session-3"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorWhatsApp.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client that adds session header
	transport := &sessionHTTPTransport{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create WhatsApp client pointing to test server
	client := NewClient("123456789", "test-token", server.URL)
	client.httpClient = customClient

	t.Run("SendImageWithCaption", func(t *testing.T) {
		// Send image message
		resp, err := client.SendImage("+1234567890", "https://example.com/image.jpg", "Check out this image!")

		// Assertions
		require.NoError(t, err, "Send should not return error")
		assert.NotNil(t, resp, "Should return response")
		assert.Equal(t, "whatsapp", resp.MessagingProduct)
		assert.Len(t, resp.Messages, 1, "Should have one message")
		assert.NotEmpty(t, resp.Messages[0].ID, "Message ID should not be empty")
	})
}

func TestWhatsAppSimulatorSendDocumentMessage(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "whatsapp-test-session-4"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorWhatsApp.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client that adds session header
	transport := &sessionHTTPTransport{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create WhatsApp client pointing to test server
	client := NewClient("123456789", "test-token", server.URL)
	client.httpClient = customClient

	t.Run("SendDocumentWithCaption", func(t *testing.T) {
		// Send document message
		resp, err := client.SendDocument("+1234567890", "https://example.com/document.pdf", "Here's the document")

		// Assertions
		require.NoError(t, err, "Send should not return error")
		assert.NotNil(t, resp, "Should return response")
		assert.Equal(t, "whatsapp", resp.MessagingProduct)
		assert.Len(t, resp.Messages, 1, "Should have one message")
		assert.NotEmpty(t, resp.Messages[0].ID, "Message ID should not be empty")
	})
}

func TestWhatsAppSimulatorSendVideoMessage(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "whatsapp-test-session-5"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorWhatsApp.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client that adds session header
	transport := &sessionHTTPTransport{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create WhatsApp client pointing to test server
	client := NewClient("123456789", "test-token", server.URL)
	client.httpClient = customClient

	t.Run("SendVideoWithCaption", func(t *testing.T) {
		// Send video message
		resp, err := client.SendVideo("+1234567890", "https://example.com/video.mp4", "Watch this video!")

		// Assertions
		require.NoError(t, err, "Send should not return error")
		assert.NotNil(t, resp, "Should return response")
		assert.Equal(t, "whatsapp", resp.MessagingProduct)
		assert.Len(t, resp.Messages, 1, "Should have one message")
		assert.NotEmpty(t, resp.Messages[0].ID, "Message ID should not be empty")
	})
}

func TestWhatsAppSimulatorSessionIsolation(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create two test sessions
	sessionID1 := "whatsapp-test-session-isolation-1"
	sessionID2 := "whatsapp-test-session-isolation-2"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorWhatsApp.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create client for session 1
	transport1 := &sessionHTTPTransport{sessionID: sessionID1}
	client1 := NewClient("123456789", "test-token", server.URL)
	client1.httpClient = &http.Client{Transport: transport1}

	// Create client for session 2
	transport2 := &sessionHTTPTransport{sessionID: sessionID2}
	client2 := NewClient("123456789", "test-token", server.URL)
	client2.httpClient = &http.Client{Transport: transport2}

	t.Run("SessionsAreIsolated", func(t *testing.T) {
		// Send message in session 1
		resp1, err := client1.SendMessage("+1111111111", "Message from session 1")
		require.NoError(t, err, "Session 1 send should succeed")
		assert.NotEmpty(t, resp1.Messages[0].ID)

		// Send message in session 2
		resp2, err := client2.SendMessage("+2222222222", "Message from session 2")
		require.NoError(t, err, "Session 2 send should succeed")
		assert.NotEmpty(t, resp2.Messages[0].ID)

		// Verify messages have different IDs
		assert.NotEqual(t, resp1.Messages[0].ID, resp2.Messages[0].ID, "Messages from different sessions should have different IDs")

		// Verify data stored correctly by checking database directly
		// Session 1 should have 1 message
		messages1, err := queries.ListWhatsAppMessages(context.Background(), database.ListWhatsAppMessagesParams{
			SessionID: sessionID1,
			Limit:     100,
		})
		require.NoError(t, err)
		assert.Len(t, messages1, 1, "Session 1 should have exactly 1 message")
		assert.Equal(t, "+1111111111", messages1[0].ToNumber)

		// Session 2 should have 1 message
		messages2, err := queries.ListWhatsAppMessages(context.Background(), database.ListWhatsAppMessagesParams{
			SessionID: sessionID2,
			Limit:     100,
		})
		require.NoError(t, err)
		assert.Len(t, messages2, 1, "Session 2 should have exactly 1 message")
		assert.Equal(t, "+2222222222", messages2[0].ToNumber)
	})
}
