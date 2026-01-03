package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// SSEEvent represents a server-sent event
type SSEEvent struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	SessionID string                 `json:"session_id"`
	Simulator string                 `json:"simulator"`
	Data      map[string]interface{} `json:"data"`
}

// SSEHub manages SSE connections and broadcasts events
type SSEHub struct {
	mu          sync.RWMutex
	connections map[string]map[chan SSEEvent]bool // sessionID -> connections
}

// NewSSEHub creates a new SSE hub
func NewSSEHub() *SSEHub {
	return &SSEHub{
		connections: make(map[string]map[chan SSEEvent]bool),
	}
}

// Subscribe adds a new SSE connection for a session
func (h *SSEHub) Subscribe(sessionID string) chan SSEEvent {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan SSEEvent, 100)
	if h.connections[sessionID] == nil {
		h.connections[sessionID] = make(map[chan SSEEvent]bool)
	}
	h.connections[sessionID][ch] = true

	return ch
}

// Unsubscribe removes an SSE connection
func (h *SSEHub) Unsubscribe(sessionID string, ch chan SSEEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if conns, ok := h.connections[sessionID]; ok {
		delete(conns, ch)
		close(ch)
		if len(conns) == 0 {
			delete(h.connections, sessionID)
		}
	}
}

// Broadcast sends an event to all connections for a session
func (h *SSEHub) Broadcast(event SSEEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if conns, ok := h.connections[event.SessionID]; ok {
		for ch := range conns {
			select {
			case ch <- event:
			default:
				// Channel full, skip
			}
		}
	}
}

// SSEHandler handles Server-Sent Events connections
type SSEHandler struct {
	hub *SSEHub
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler(hub *SSEHub) *SSEHandler {
	return &SSEHandler{hub: hub}
}

// ServeHTTP handles SSE requests
func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract session ID from path: /events/{sessionId}
	path := strings.TrimPrefix(r.URL.Path, "/events/")
	sessionID := path

	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	// Subscribe to events
	ch := h.hub.Subscribe(sessionID)
	defer h.hub.Unsubscribe(sessionID, ch)

	// Get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send initial connection event
	_, _ = fmt.Fprintf(w, "data: %s\n\n", toJSON(map[string]interface{}{
		"type":      "connected",
		"sessionId": sessionID,
		"timestamp": time.Now(),
	}))
	flusher.Flush()

	// Create context for cleanup
	ctx := r.Context()

	// Send heartbeat every 30 seconds to keep connection alive
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			log.Printf("SSE client disconnected: session=%s", sessionID)
			return

		case event := <-ch:
			// Send event to client
			data := toJSON(event)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

		case <-ticker.C:
			// Send heartbeat
			_, _ = fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

// toJSON converts a value to JSON string
func toJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// Global SSE hub instance
var globalSSEHub *SSEHub

// InitSSEHub initializes the global SSE hub
func InitSSEHub() *SSEHub {
	globalSSEHub = NewSSEHub()
	return globalSSEHub
}

// BroadcastEvent broadcasts an event to all SSE clients for a session
func BroadcastEvent(sessionID, simulator, eventType string, data map[string]interface{}) {
	if globalSSEHub == nil {
		return
	}

	event := SSEEvent{
		Type:      eventType,
		Timestamp: time.Now(),
		SessionID: sessionID,
		Simulator: simulator,
		Data:      data,
	}

	globalSSEHub.Broadcast(event)
}

// SSEMiddleware wraps handlers to broadcast events
func SSEMiddleware(simulator string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get session ID from header (set by session middleware)
		sessionID := r.Header.Get("X-Session-ID")
		if sessionID == "" {
			// Try context as fallback
			if ctxVal := r.Context().Value("session_id"); ctxVal != nil {
				if sid, ok := ctxVal.(string); ok {
					sessionID = sid
				}
			}
		}

		// Only broadcast if we have a session ID
		if sessionID != "" {
			// Broadcast request event
			BroadcastEvent(sessionID, simulator, "api_request", map[string]interface{}{
				"method": r.Method,
				"path":   r.URL.Path,
				"time":   time.Now(),
			})
		}

		// Wrap response writer to capture response
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Call next handler
		next.ServeHTTP(wrapped, r)

		// Broadcast response event if we have a session ID
		if sessionID != "" {
			BroadcastEvent(sessionID, simulator, "api_response", map[string]interface{}{
				"method":     r.Method,
				"path":       r.URL.Path,
				"statusCode": wrapped.statusCode,
				"time":       time.Now(),
			})
		}
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}
