package session

import (
	"context"
	"net/http"
)

// contextKey is the type for session context keys
type contextKey string

const (
	// SessionIDKey is the context key for session ID
	SessionIDKey contextKey = "session_id"

	// DefaultSessionID is used when no session ID is provided
	DefaultSessionID = "default"

	// SessionHeaderName is the HTTP header name for session ID
	SessionHeaderName = "X-Session-ID"
)

// Middleware extracts session ID from request headers and adds it to context
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract session ID from header
		sessionID := r.Header.Get(SessionHeaderName)

		// Use default if not provided
		if sessionID == "" {
			sessionID = DefaultSessionID
		}

		// Add session ID to context
		ctx := context.WithValue(r.Context(), SessionIDKey, sessionID)

		// Call next handler with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext retrieves session ID from context
func FromContext(ctx context.Context) string {
	sessionID, ok := ctx.Value(SessionIDKey).(string)
	if !ok {
		return DefaultSessionID
	}
	return sessionID
}

// WithSessionID creates a new context with the given session ID
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, SessionIDKey, sessionID)
}
