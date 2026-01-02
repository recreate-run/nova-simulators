package session

import (
	"context"
	"encoding/json"
	"net/http"
)

// contextKey is the type for session context keys
type contextKey string

const (
	// SessionIDKey is the context key for session ID
	SessionIDKey contextKey = "session_id"

	// SessionHeaderName is the HTTP header name for session ID
	SessionHeaderName = "X-Session-ID"
)

// Middleware extracts session ID from request headers and adds it to context
// Returns 400 Bad Request if session ID is not provided
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract session ID from header
		sessionID := r.Header.Get(SessionHeaderName)

		// Require session ID - no default fallback
		if sessionID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":    false,
				"error": "missing_session_id",
				"message": "X-Session-ID header is required",
			})
			return
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
		return ""
	}
	return sessionID
}

// WithSessionID creates a new context with the given session ID
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, SessionIDKey, sessionID)
}
