package session

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

const (
	// RootDir is the base directory for all session working directories
	RootDir = "sessions"
)

// contextKey for directory path
const SessionDirKey contextKey = "session_dir"

// Directory manages session working directories
type Directory struct {
	sessionID string
	rootPath  string
}

// NewDirectory creates a new directory manager for a session
func NewDirectory(sessionID string) *Directory {
	return &Directory{
		sessionID: sessionID,
		rootPath:  filepath.Join(RootDir, sessionID),
	}
}

// Create creates the session directory
// Returns error if directory creation fails
func (d *Directory) Create() error {
	log.Printf("[session] → Creating working directory for session: %s", d.sessionID)

	// Create session root directory
	if err := os.MkdirAll(d.rootPath, 0o750); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	log.Printf("[session] ✓ Working directory created: %s", d.rootPath)
	return nil
}

// Delete removes the entire session directory
func (d *Directory) Delete() error {
	log.Printf("[session] → Deleting working directory for session: %s", d.sessionID)

	if err := os.RemoveAll(d.rootPath); err != nil {
		return fmt.Errorf("failed to delete session directory: %w", err)
	}

	log.Printf("[session] ✓ Working directory deleted: %s", d.rootPath)
	return nil
}

// RootPath returns the root path of the session directory
func (d *Directory) RootPath() string {
	return d.rootPath
}

// Exists checks if the session directory exists
func (d *Directory) Exists() bool {
	info, err := os.Stat(d.rootPath)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && info.IsDir()
}
