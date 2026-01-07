package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/recreate-run/nova-simulators/internal/config"
)

// ConfigHandler serves the configuration API endpoints
type ConfigHandler struct {
	configManager *config.Manager
}

// NewConfigHandler creates a new config handler
func NewConfigHandler(configManager *config.Manager) *ConfigHandler {
	return &ConfigHandler{
		configManager: configManager,
	}
}

// ConfigRequest represents the request body for updating config
type ConfigRequest struct {
	Timeout   config.TimeoutConfig   `json:"timeout"`
	RateLimit config.RateLimitConfig `json:"rate_limit"`
}

// ConfigResponse represents the response body for config requests
type ConfigResponse struct {
	SessionID  string                 `json:"session_id"`
	Simulator  string                 `json:"simulator"`
	Timeout    config.TimeoutConfig   `json:"timeout"`
	RateLimit  config.RateLimitConfig `json:"rate_limit"`
}

// ServeHTTP implements http.Handler interface
func (h *ConfigHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse URL: /api/sessions/{sessionID}/config[/{simulator}]
	path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 || parts[1] != "config" {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	sessionID := parts[0]

	// List all configs for a session
	if len(parts) == 2 {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.handleListConfigs(w, r, sessionID)
		return
	}

	// Single simulator config
	if len(parts) == 3 {
		simulator := parts[2]
		switch r.Method {
		case http.MethodGet:
			h.handleGetConfig(w, r, sessionID, simulator)
		case http.MethodPut:
			h.handleSetConfig(w, r, sessionID, simulator)
		case http.MethodDelete:
			h.handleDeleteConfig(w, r, sessionID, simulator)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	http.Error(w, "Invalid URL format", http.StatusBadRequest)
}

func (h *ConfigHandler) handleGetConfig(w http.ResponseWriter, _ *http.Request, sessionID, simulator string) {
	ctx := context.Background()

	timeout := h.configManager.GetTimeoutConfig(ctx, sessionID, simulator)
	rateLimit := h.configManager.GetRateLimitConfig(ctx, sessionID, simulator)

	response := ConfigResponse{
		SessionID:  sessionID,
		Simulator:  simulator,
		Timeout:    *timeout,
		RateLimit:  *rateLimit,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (h *ConfigHandler) handleSetConfig(w http.ResponseWriter, r *http.Request, sessionID, simulator string) {
	var req ConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	if err := h.configManager.SetSessionConfig(ctx, sessionID, simulator, &req.Timeout, &req.RateLimit); err != nil {
		http.Error(w, "Failed to set config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := ConfigResponse{
		SessionID:  sessionID,
		Simulator:  simulator,
		Timeout:    req.Timeout,
		RateLimit:  req.RateLimit,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

func (h *ConfigHandler) handleDeleteConfig(w http.ResponseWriter, _ *http.Request, sessionID, simulator string) {
	ctx := context.Background()
	if err := h.configManager.DeleteSessionConfig(ctx, sessionID, simulator); err != nil {
		http.Error(w, "Failed to delete config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ConfigHandler) handleListConfigs(w http.ResponseWriter, _ *http.Request, sessionID string) {
	ctx := context.Background()

	configs, err := h.configManager.ListSessionConfigs(ctx, sessionID)
	if err != nil {
		http.Error(w, "Failed to list configs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert database configs to response format
	responses := make([]ConfigResponse, 0, len(configs))
	for _, cfg := range configs {
		responses = append(responses, ConfigResponse{
			SessionID: cfg.SessionID,
			Simulator: cfg.SimulatorName,
			Timeout: config.TimeoutConfig{
				MinMs: int(cfg.TimeoutMinMs),
				MaxMs: int(cfg.TimeoutMaxMs),
			},
			RateLimit: config.RateLimitConfig{
				PerMinute: int(cfg.RateLimitPerMinute),
				PerDay:    int(cfg.RateLimitPerDay),
			},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"configs": responses,
	})
}
