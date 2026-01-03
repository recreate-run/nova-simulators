package session_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/recreate-run/nova-simulators/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionManagerCreateWithDirectory(t *testing.T) {
	// Setup: Create test database and manager
	queries := setupTestDB(t)
	manager := session.NewManager(queries)

	// Create test server
	server := httptest.NewServer(manager)
	defer server.Close()

	// Cleanup: Remove any test directories after test
	t.Cleanup(func() {
		_ = os.RemoveAll("sessions")
	})

	t.Run("Creating session creates working directory", func(t *testing.T) {
		// Send POST request to create session
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/sessions", http.NoBody)
		require.NoError(t, err, "Failed to create POST request")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Failed to send POST request")
		defer resp.Body.Close()

		// Verify response
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK")

		var result map[string]string
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err, "Failed to decode response")

		sessionID, ok := result["session_id"]
		require.True(t, ok, "Response should contain session_id")
		require.NotEmpty(t, sessionID, "Session ID should not be empty")

		// Verify working directory was created
		expectedPath := filepath.Join("sessions", sessionID)
		info, err := os.Stat(expectedPath)
		require.NoError(t, err, "Working directory should exist")
		assert.True(t, info.IsDir(), "Path should be a directory")
	})
}

func TestSessionManagerDeleteWithDirectory(t *testing.T) {
	// Setup: Create test database and manager
	queries := setupTestDB(t)
	manager := session.NewManager(queries)

	// Create test server
	server := httptest.NewServer(manager)
	defer server.Close()

	// Cleanup: Remove any test directories after test
	t.Cleanup(func() {
		_ = os.RemoveAll("sessions")
	})

	t.Run("Deleting session removes working directory", func(t *testing.T) {
		// Create session first
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/sessions", http.NoBody)
		require.NoError(t, err, "Failed to create POST request")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Failed to create session")
		defer resp.Body.Close()

		var createResult map[string]string
		err = json.NewDecoder(resp.Body).Decode(&createResult)
		require.NoError(t, err, "Failed to decode create response")

		sessionID := createResult["session_id"]
		require.NotEmpty(t, sessionID, "Session ID should not be empty")

		// Verify directory exists
		expectedPath := filepath.Join("sessions", sessionID)
		_, err = os.Stat(expectedPath)
		require.NoError(t, err, "Working directory should exist after creation")

		// Delete session
		req, err = http.NewRequestWithContext(context.Background(), http.MethodDelete, server.URL+"/sessions/"+sessionID, http.NoBody)
		require.NoError(t, err, "Failed to create DELETE request")

		deleteResp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Failed to send DELETE request")
		defer deleteResp.Body.Close()

		// Verify delete response
		assert.Equal(t, http.StatusOK, deleteResp.StatusCode, "Expected 200 OK")

		var deleteResult map[string]string
		err = json.NewDecoder(deleteResp.Body).Decode(&deleteResult)
		require.NoError(t, err, "Failed to decode delete response")
		assert.Equal(t, "deleted", deleteResult["status"], "Status should be deleted")

		// Verify directory was removed
		_, err = os.Stat(expectedPath)
		assert.True(t, os.IsNotExist(err), "Working directory should not exist after deletion")
	})
}

func TestSessionManagerMultipleSessions(t *testing.T) {
	// Setup: Create test database and manager
	queries := setupTestDB(t)
	manager := session.NewManager(queries)

	// Create test server
	server := httptest.NewServer(manager)
	defer server.Close()

	// Cleanup: Remove any test directories after test
	t.Cleanup(func() {
		_ = os.RemoveAll("sessions")
	})

	t.Run("Multiple sessions have isolated directories", func(t *testing.T) {
		// Create first session
		req1, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/sessions", http.NoBody)
		require.NoError(t, err, "Failed to create POST request 1")
		resp1, err := http.DefaultClient.Do(req1)
		require.NoError(t, err, "Failed to create session 1")
		defer resp1.Body.Close()

		var result1 map[string]string
		err = json.NewDecoder(resp1.Body).Decode(&result1)
		require.NoError(t, err, "Failed to decode response 1")
		sessionID1 := result1["session_id"]

		// Create second session
		req2, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/sessions", http.NoBody)
		require.NoError(t, err, "Failed to create POST request 2")
		resp2, err := http.DefaultClient.Do(req2)
		require.NoError(t, err, "Failed to create session 2")
		defer resp2.Body.Close()

		var result2 map[string]string
		err = json.NewDecoder(resp2.Body).Decode(&result2)
		require.NoError(t, err, "Failed to decode response 2")
		sessionID2 := result2["session_id"]

		// Verify both directories exist
		path1 := filepath.Join("sessions", sessionID1)
		path2 := filepath.Join("sessions", sessionID2)

		info1, err := os.Stat(path1)
		require.NoError(t, err, "Directory 1 should exist")
		assert.True(t, info1.IsDir(), "Path 1 should be a directory")

		info2, err := os.Stat(path2)
		require.NoError(t, err, "Directory 2 should exist")
		assert.True(t, info2.IsDir(), "Path 2 should be a directory")

		// Verify directories are different
		assert.NotEqual(t, sessionID1, sessionID2, "Session IDs should be different")

		// Delete first session
		req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, server.URL+"/sessions/"+sessionID1, http.NoBody)
		require.NoError(t, err, "Failed to create DELETE request")

		deleteResp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Failed to delete session 1")
		defer deleteResp.Body.Close()

		// Verify first directory is gone but second remains
		_, err = os.Stat(path1)
		assert.True(t, os.IsNotExist(err), "Directory 1 should not exist after deletion")

		_, err = os.Stat(path2)
		assert.NoError(t, err, "Directory 2 should still exist")
	})
}

func TestSessionManagerDirectoryWithFiles(t *testing.T) {
	// Setup: Create test database and manager
	queries := setupTestDB(t)
	manager := session.NewManager(queries)

	// Create test server
	server := httptest.NewServer(manager)
	defer server.Close()

	// Cleanup: Remove any test directories after test
	t.Cleanup(func() {
		_ = os.RemoveAll("sessions")
	})

	t.Run("Deleting session removes directory with files", func(t *testing.T) {
		// Create session
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/sessions", http.NoBody)
		require.NoError(t, err, "Failed to create POST request")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Failed to create session")
		defer resp.Body.Close()

		var result map[string]string
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err, "Failed to decode response")
		sessionID := result["session_id"]

		// Create a file in the session directory
		sessionPath := filepath.Join("sessions", sessionID)

		// Write a test file
		testFile := filepath.Join(sessionPath, "seed_data.sql")
		err = os.WriteFile(testFile, []byte("INSERT INTO test VALUES (1);"), 0o600)
		require.NoError(t, err, "Failed to write test file")

		// Verify file exists
		_, err = os.Stat(testFile)
		require.NoError(t, err, "Test file should exist")

		// Delete session
		req, err = http.NewRequestWithContext(context.Background(), http.MethodDelete, server.URL+"/sessions/"+sessionID, http.NoBody)
		require.NoError(t, err, "Failed to create DELETE request")

		deleteResp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Failed to delete session")
		defer deleteResp.Body.Close()

		// Verify entire directory tree was removed
		_, err = os.Stat(sessionPath)
		assert.True(t, os.IsNotExist(err), "Session directory should not exist")
		_, err = os.Stat(testFile)
		assert.True(t, os.IsNotExist(err), "Test file should not exist")
	})
}
