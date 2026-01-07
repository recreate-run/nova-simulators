package config_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/config"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/middleware"
	"github.com/recreate-run/nova-simulators/internal/session"
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

func setupTestSession(t *testing.T, queries *database.Queries, sessionID string) {
	t.Helper()
	ctx := context.Background()

	err := queries.CreateSession(ctx, sessionID)
	require.NoError(t, err, "Failed to create session")
}

func TestConfigAPI(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create default config and manager
	defaultCfg := config.Default()
	configManager := config.NewManager(defaultCfg, queries)

	// Setup: Create test handler (simulating main.go setup)
	mux := http.NewServeMux()

	// Simple echo handler to test middleware
	echoHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Register handler with session middleware (to populate session context)
	testHandler := session.Middleware(echoHandler)
	mux.Handle("/test/", testHandler)

	server := httptest.NewServer(mux)
	defer server.Close()

	// Setup: Create test session
	sessionID := "test-session-config"
	setupTestSession(t, queries, sessionID)

	t.Run("GetConfigWithoutOverride", func(t *testing.T) {
		// Get config for slack simulator (no override set)
		cfg := configManager.GetTimeoutConfig(context.Background(), sessionID, "slack")

		// Should return YAML defaults
		assert.Equal(t, defaultCfg.Slack.Timeout.MinMs, cfg.MinMs)
		assert.Equal(t, defaultCfg.Slack.Timeout.MaxMs, cfg.MaxMs)
	})

	t.Run("SetAndGetConfigOverride", func(t *testing.T) {
		ctx := context.Background()

		// Set session-specific override
		customTimeout := &config.TimeoutConfig{
			MinMs: 500,
			MaxMs: 1000,
		}
		customRateLimit := &config.RateLimitConfig{
			PerMinute: 10,
			PerDay:    100,
		}

		err := configManager.SetSessionConfig(ctx, sessionID, "slack", customTimeout, customRateLimit)
		require.NoError(t, err, "SetSessionConfig should succeed")

		// Get config and verify override is returned
		timeout := configManager.GetTimeoutConfig(ctx, sessionID, "slack")
		rateLimit := configManager.GetRateLimitConfig(ctx, sessionID, "slack")

		assert.Equal(t, 500, timeout.MinMs)
		assert.Equal(t, 1000, timeout.MaxMs)
		assert.Equal(t, 10, rateLimit.PerMinute)
		assert.Equal(t, 100, rateLimit.PerDay)
	})

	t.Run("DeleteConfigOverride", func(t *testing.T) {
		ctx := context.Background()

		// First, set an override
		customTimeout := &config.TimeoutConfig{MinMs: 300, MaxMs: 600}
		customRateLimit := &config.RateLimitConfig{PerMinute: 20, PerDay: 200}
		err := configManager.SetSessionConfig(ctx, sessionID, "gmail", customTimeout, customRateLimit)
		require.NoError(t, err)

		// Verify override is active
		timeout := configManager.GetTimeoutConfig(ctx, sessionID, "gmail")
		assert.Equal(t, 300, timeout.MinMs)

		// Delete the override
		err = configManager.DeleteSessionConfig(ctx, sessionID, "gmail")
		require.NoError(t, err, "DeleteSessionConfig should succeed")

		// Verify we're back to defaults
		timeout = configManager.GetTimeoutConfig(ctx, sessionID, "gmail")
		assert.Equal(t, defaultCfg.Gmail.Timeout.MinMs, timeout.MinMs)
		assert.Equal(t, defaultCfg.Gmail.Timeout.MaxMs, timeout.MaxMs)
	})

	t.Run("ListSessionConfigs", func(t *testing.T) {
		ctx := context.Background()
		sessionID2 := "test-session-list"
		setupTestSession(t, queries, sessionID2)

		// Set multiple overrides
		err := configManager.SetSessionConfig(ctx, sessionID2, "slack",
			&config.TimeoutConfig{MinMs: 100, MaxMs: 200},
			&config.RateLimitConfig{PerMinute: 30, PerDay: 300})
		require.NoError(t, err)

		err = configManager.SetSessionConfig(ctx, sessionID2, "gmail",
			&config.TimeoutConfig{MinMs: 150, MaxMs: 250},
			&config.RateLimitConfig{PerMinute: 40, PerDay: 400})
		require.NoError(t, err)

		// List configs
		configs, err := configManager.ListSessionConfigs(ctx, sessionID2)
		require.NoError(t, err, "ListSessionConfigs should succeed")

		// Verify we get both configs
		assert.Len(t, configs, 2, "Should return 2 configs")

		// Verify configs are correct
		configMap := make(map[string]database.SessionConfig)
		for _, cfg := range configs {
			configMap[cfg.SimulatorName] = cfg
		}

		assert.Contains(t, configMap, "slack")
		assert.Equal(t, int64(100), configMap["slack"].TimeoutMinMs)
		assert.Equal(t, int64(30), configMap["slack"].RateLimitPerMinute)

		assert.Contains(t, configMap, "gmail")
		assert.Equal(t, int64(150), configMap["gmail"].TimeoutMinMs)
		assert.Equal(t, int64(40), configMap["gmail"].RateLimitPerMinute)
	})

	t.Run("SessionIsolation", func(t *testing.T) {
		ctx := context.Background()

		// Create two sessions
		session1 := "test-session-isolation-1"
		session2 := "test-session-isolation-2"
		setupTestSession(t, queries, session1)
		setupTestSession(t, queries, session2)

		// Set different configs for each session
		err := configManager.SetSessionConfig(ctx, session1, "slack",
			&config.TimeoutConfig{MinMs: 100, MaxMs: 200},
			&config.RateLimitConfig{PerMinute: 10, PerDay: 100})
		require.NoError(t, err)

		err = configManager.SetSessionConfig(ctx, session2, "slack",
			&config.TimeoutConfig{MinMs: 300, MaxMs: 400},
			&config.RateLimitConfig{PerMinute: 20, PerDay: 200})
		require.NoError(t, err)

		// Verify each session gets its own config
		timeout1 := configManager.GetTimeoutConfig(ctx, session1, "slack")
		timeout2 := configManager.GetTimeoutConfig(ctx, session2, "slack")

		assert.Equal(t, 100, timeout1.MinMs)
		assert.Equal(t, 300, timeout2.MinMs)

		rateLimit1 := configManager.GetRateLimitConfig(ctx, session1, "slack")
		rateLimit2 := configManager.GetRateLimitConfig(ctx, session2, "slack")

		assert.Equal(t, 10, rateLimit1.PerMinute)
		assert.Equal(t, 20, rateLimit2.PerMinute)
	})
}

func TestMiddlewareWithSessionConfig(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create default config and manager
	defaultCfg := config.Default()
	configManager := config.NewManager(defaultCfg, queries)

	// Setup: Create test session
	sessionID := "test-session-middleware"
	setupTestSession(t, queries, sessionID)

	t.Run("TimeoutMiddlewareUsesSessionConfig", func(t *testing.T) {
		ctx := context.Background()

		// Set custom timeout for this session
		customTimeout := &config.TimeoutConfig{
			MinMs: 200,
			MaxMs: 200, // Same min/max for predictable testing
		}
		customRateLimit := &config.RateLimitConfig{
			PerMinute: 100,
			PerDay:    1000,
		}
		err := configManager.SetSessionConfig(ctx, sessionID, "slack", customTimeout, customRateLimit)
		require.NoError(t, err)

		// Create handler with timeout middleware
		echoHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})

		handler := session.Middleware(
			middleware.Timeout(configManager, "slack")(echoHandler))

		server := httptest.NewServer(handler)
		defer server.Close()

		// Make request with session ID
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/test", http.NoBody)
		require.NoError(t, err)
		req.Header.Set("X-Session-ID", sessionID)

		// Measure request duration
		start := time.Now()
		resp, err := http.DefaultClient.Do(req)
		duration := time.Since(start)

		require.NoError(t, err)
		defer resp.Body.Close()

		// Verify timeout was applied (200ms + overhead)
		assert.GreaterOrEqual(t, duration.Milliseconds(), int64(200), "Should apply configured timeout")
		assert.LessOrEqual(t, duration.Milliseconds(), int64(300), "Should not exceed timeout + overhead")
	})

	t.Run("RateLimitMiddlewareUsesSessionConfig", func(t *testing.T) {
		ctx := context.Background()
		sessionID2 := "test-session-ratelimit"
		setupTestSession(t, queries, sessionID2)

		// Set custom rate limit for this session
		customTimeout := &config.TimeoutConfig{MinMs: 0, MaxMs: 0}
		customRateLimit := &config.RateLimitConfig{
			PerMinute: 3, // Very low limit for testing
			PerDay:    100,
		}
		err := configManager.SetSessionConfig(ctx, sessionID2, "gmail", customTimeout, customRateLimit)
		require.NoError(t, err)

		// Create handler with rate limit middleware
		echoHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})

		handler := session.Middleware(
			middleware.RateLimit(configManager, "gmail")(echoHandler))

		server := httptest.NewServer(handler)
		defer server.Close()

		// Make 3 requests (should all succeed)
		for i := 0; i < 3; i++ {
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/test", http.NoBody)
			req.Header.Set("X-Session-ID", sessionID2)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			_ = resp.Body.Close()
			assert.NotEqual(t, http.StatusTooManyRequests, resp.StatusCode, "Request %d should succeed", i+1)
		}

		// 4th request should be rate limited
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/test", http.NoBody)
		req.Header.Set("X-Session-ID", sessionID2)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode, "4th request should be rate limited")
	})

	t.Run("MiddlewareUsesDefaultsWithoutOverride", func(t *testing.T) {
		sessionID3 := "test-session-defaults"
		setupTestSession(t, queries, sessionID3)

		// Don't set any override - should use YAML defaults

		// Create handler with timeout middleware
		echoHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})

		// Use default timeout (0ms from config.Default())
		handler := session.Middleware(
			middleware.Timeout(configManager, "slack")(echoHandler))

		server := httptest.NewServer(handler)
		defer server.Close()

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/test", http.NoBody)
		req.Header.Set("X-Session-ID", sessionID3)

		start := time.Now()
		resp, err := http.DefaultClient.Do(req)
		duration := time.Since(start)

		require.NoError(t, err)
		defer resp.Body.Close()

		// Default timeout is 0, so request should be very fast (< 50ms overhead)
		assert.Less(t, duration.Milliseconds(), int64(50), "Should use default (no delay)")
	})
}
