package session

import (
	"path/filepath"
	"testing"
)

func TestDirectoryCreate(t *testing.T) {
	sessionID := "test-session-create-123"
	dir := NewDirectory(sessionID)

	// Cleanup after test
	t.Cleanup(func() {
		_ = dir.Delete()
	})

	// Create directory structure
	err := dir.Create()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Verify root directory exists
	if !dir.Exists() {
		t.Error("Root directory does not exist")
	}
}

func TestDirectoryDelete(t *testing.T) {
	sessionID := "test-session-delete-456"
	dir := NewDirectory(sessionID)

	// Create directory first
	err := dir.Create()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Verify it exists
	if !dir.Exists() {
		t.Fatal("Directory should exist after creation")
	}

	// Delete directory
	err = dir.Delete()
	if err != nil {
		t.Fatalf("Failed to delete directory: %v", err)
	}

	// Verify it no longer exists
	if dir.Exists() {
		t.Error("Directory should not exist after deletion")
	}
}

func TestDirectoryPaths(t *testing.T) {
	sessionID := "test-session-paths-789"
	dir := NewDirectory(sessionID)

	// Test RootPath
	expectedRoot := filepath.Join(RootDir, sessionID)
	if dir.RootPath() != expectedRoot {
		t.Errorf("RootPath() = %s, want %s", dir.RootPath(), expectedRoot)
	}
}

func TestDirectoryExists(t *testing.T) {
	sessionID := "test-session-exists-abc"
	dir := NewDirectory(sessionID)

	// Cleanup after test
	t.Cleanup(func() {
		_ = dir.Delete()
	})

	// Should not exist initially
	if dir.Exists() {
		t.Error("Directory should not exist initially")
	}

	// Create directory
	err := dir.Create()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Should exist after creation
	if !dir.Exists() {
		t.Error("Directory should exist after creation")
	}

	// Delete directory
	err = dir.Delete()
	if err != nil {
		t.Fatalf("Failed to delete directory: %v", err)
	}

	// Should not exist after deletion
	if dir.Exists() {
		t.Error("Directory should not exist after deletion")
	}
}

func TestDirectoryCreateIdempotent(t *testing.T) {
	sessionID := "test-session-idempotent-def"
	dir := NewDirectory(sessionID)

	// Cleanup after test
	t.Cleanup(func() {
		_ = dir.Delete()
	})

	// Create directory first time
	err := dir.Create()
	if err != nil {
		t.Fatalf("First create failed: %v", err)
	}

	// Create directory second time (should not error)
	err = dir.Create()
	if err != nil {
		t.Fatalf("Second create failed: %v", err)
	}

	// Verify directory still exists
	if !dir.Exists() {
		t.Error("Directory should exist after multiple creates")
	}
}

func TestDirectoryDeleteNonExistent(t *testing.T) {
	sessionID := "test-session-nonexistent-ghi"
	dir := NewDirectory(sessionID)

	// Try to delete a directory that was never created
	// This should not error (os.RemoveAll is idempotent)
	err := dir.Delete()
	if err != nil {
		t.Fatalf("Delete of non-existent directory should not error: %v", err)
	}
}
