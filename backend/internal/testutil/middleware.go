package testutil

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/recreate-run/nova-simulators/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HandlerFactory creates a handler with the given config manager
type HandlerFactory func(configManager *config.Manager) http.Handler

// RequestFactory creates a test HTTP request for the given session ID
type RequestFactory func(sessionID string) *http.Request

// buildClientRequest converts a test request to a client-compatible request
func buildClientRequest(t *testing.T, testReq *http.Request, baseURL string) *http.Request {
	t.Helper()

	// Build full URL
	fullURL := baseURL + testReq.URL.Path
	if testReq.URL.RawQuery != "" {
		fullURL += "?" + testReq.URL.RawQuery
	}

	// Read body if present and recreate from bytes
	var reqBody io.Reader = http.NoBody
	if testReq.Body != nil {
		bodyBytes, err := io.ReadAll(testReq.Body)
		require.NoError(t, err, "Should read request body")
		_ = testReq.Body.Close()

		if len(bodyBytes) > 0 {
			reqBody = bytes.NewReader(bodyBytes)
		}
	}

	clientReq, err := http.NewRequestWithContext(testReq.Context(), testReq.Method, fullURL, reqBody)
	require.NoError(t, err, "Should create client request")

	// Copy headers
	for k, v := range testReq.Header {
		clientReq.Header[k] = v
	}

	return clientReq
}

// TestMiddlewareTimeout tests that timeout middleware applies configured delays
func TestMiddlewareTimeout(t *testing.T, makeHandler HandlerFactory, makeRequest RequestFactory, sessionIDPrefix string) {
	t.Helper()

	// Setup: Create config with test timeout values
	cfg := config.Default()
	// Update all simulator configs with test timeout values
	testTimeout := config.TimeoutConfig{MinMs: 100, MaxMs: 200}
	testRateLimit := config.RateLimitConfig{PerMinute: 100, PerDay: 1000}

	cfg.Gmail.Timeout = testTimeout
	cfg.Gmail.RateLimit = testRateLimit
	cfg.Slack.Timeout = testTimeout
	cfg.Slack.RateLimit = testRateLimit

	// Setup: Create config manager (no queries needed for these tests)
	configManager := config.NewManager(cfg, nil)

	// Setup: Create handler with middleware
	handler := makeHandler(configManager)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Setup: Create test request
	sessionID := sessionIDPrefix + "-timeout-test"
	testReq := makeRequest(sessionID)

	// Build client-compatible request
	clientReq := buildClientRequest(t, testReq, server.URL)

	// Execute: Measure request duration
	start := time.Now()
	resp, err := http.DefaultClient.Do(clientReq)
	duration := time.Since(start)

	// Verify: Request succeeded
	require.NoError(t, err, "Request should not return error")
	defer resp.Body.Close()

	// Verify: Response includes configured delay
	assert.GreaterOrEqual(t, duration.Milliseconds(), int64(100), "Response should take at least 100ms")
	assert.LessOrEqual(t, duration.Milliseconds(), int64(300), "Response should take at most 300ms (200ms + overhead)")
}

// TestMiddlewareRateLimit tests that rate limiting middleware enforces per-minute limits
func TestMiddlewareRateLimit(t *testing.T, makeHandler HandlerFactory, makeRequest RequestFactory, sessionIDPrefix string) {
	t.Helper()

	// Setup: Create config with low rate limit for testing
	cfg := config.Default()
	testTimeout := config.TimeoutConfig{MinMs: 0, MaxMs: 0}
	testRateLimit := config.RateLimitConfig{PerMinute: 5, PerDay: 100}

	cfg.Gmail.Timeout = testTimeout
	cfg.Gmail.RateLimit = testRateLimit
	cfg.Slack.Timeout = testTimeout
	cfg.Slack.RateLimit = testRateLimit

	// Setup: Create config manager (no queries needed for these tests)
	configManager := config.NewManager(cfg, nil)

	// Setup: Create handler with middleware
	handler := makeHandler(configManager)
	server := httptest.NewServer(handler)
	defer server.Close()

	sessionID := sessionIDPrefix + "-ratelimit-test"

	// Execute: Make requests up to the limit
	for i := 0; i < 5; i++ {
		testReq := makeRequest(sessionID)
		clientReq := buildClientRequest(t, testReq, server.URL)

		resp, err := http.DefaultClient.Do(clientReq)
		require.NoError(t, err, "Request %d should succeed", i+1)
		_ = resp.Body.Close()
		assert.NotEqual(t, http.StatusTooManyRequests, resp.StatusCode, "Request %d should not be rate limited", i+1)
	}

	// Execute: Next request should be rate limited
	testReq := makeRequest(sessionID)
	clientReq := buildClientRequest(t, testReq, server.URL)

	resp, err := http.DefaultClient.Do(clientReq)
	require.NoError(t, err, "Request should succeed (HTTP level)")
	defer resp.Body.Close()

	// Verify: Should receive 429 status
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode, "Request should be rate limited with 429 status")
}

// TestMiddlewareRateLimitIsolation tests that rate limits are isolated per session
func TestMiddlewareRateLimitIsolation(t *testing.T, makeHandler HandlerFactory, makeRequest RequestFactory, sessionIDPrefix string) {
	t.Helper()

	// Setup: Create config with low rate limit for testing
	cfg := config.Default()
	testTimeout := config.TimeoutConfig{MinMs: 0, MaxMs: 0}
	testRateLimit := config.RateLimitConfig{PerMinute: 5, PerDay: 100}

	cfg.Gmail.Timeout = testTimeout
	cfg.Gmail.RateLimit = testRateLimit
	cfg.Slack.Timeout = testTimeout
	cfg.Slack.RateLimit = testRateLimit

	// Setup: Create config manager (no queries needed for these tests)
	configManager := config.NewManager(cfg, nil)

	// Setup: Create handler with middleware
	handler := makeHandler(configManager)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Setup: First session hits the rate limit
	sessionID1 := sessionIDPrefix + "-ratelimit-test-1"
	for i := 0; i < 5; i++ {
		testReq := makeRequest(sessionID1)
		clientReq := buildClientRequest(t, testReq, server.URL)
		resp, _ := http.DefaultClient.Do(clientReq)
		if resp != nil {
			_ = resp.Body.Close()
		}
	}

	// Verify: First session is rate limited
	testReq1 := makeRequest(sessionID1)
	clientReq1 := buildClientRequest(t, testReq1, server.URL)
	resp1, err := http.DefaultClient.Do(clientReq1)
	require.NoError(t, err, "Request should succeed (HTTP level)")
	defer resp1.Body.Close()
	assert.Equal(t, http.StatusTooManyRequests, resp1.StatusCode, "Session 1 should be rate limited")

	// Execute: Second session should be able to make requests
	sessionID2 := sessionIDPrefix + "-ratelimit-test-2"
	testReq2 := makeRequest(sessionID2)
	clientReq2 := buildClientRequest(t, testReq2, server.URL)

	resp2, err := http.DefaultClient.Do(clientReq2)
	require.NoError(t, err, "Session 2 request should succeed")
	defer resp2.Body.Close()

	// Verify: Second session is NOT rate limited (isolation works)
	assert.NotEqual(t, http.StatusTooManyRequests, resp2.StatusCode, "Session 2 should not be rate limited despite session 1 being limited")
}
