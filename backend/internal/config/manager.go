package config

import (
	"context"
	"sync"

	"github.com/recreate-run/nova-simulators/internal/database"
)

// Manager handles runtime configuration with session-specific overrides
type Manager struct {
	defaultConfig *Config
	queries       *database.Queries
	mu            sync.RWMutex
}

// NewManager creates a new configuration manager
func NewManager(defaultConfig *Config, queries *database.Queries) *Manager {
	return &Manager{
		defaultConfig: defaultConfig,
		queries:       queries,
	}
}

// GetTimeoutConfig returns timeout config for a session/simulator (override or default)
func (m *Manager) GetTimeoutConfig(ctx context.Context, sessionID, simulator string) *TimeoutConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Try to get session-specific override from database
	if sessionID != "" && m.queries != nil {
		cfg, err := m.queries.GetSessionConfig(ctx, database.GetSessionConfigParams{
			SessionID:     sessionID,
			SimulatorName: simulator,
		})
		if err == nil {
			return &TimeoutConfig{
				MinMs: int(cfg.TimeoutMinMs),
				MaxMs: int(cfg.TimeoutMaxMs),
			}
		}
	}

	// Fall back to YAML default
	return m.getDefaultTimeoutConfig(simulator)
}

// GetRateLimitConfig returns rate limit config for a session/simulator (override or default)
func (m *Manager) GetRateLimitConfig(ctx context.Context, sessionID, simulator string) *RateLimitConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Try to get session-specific override from database
	if sessionID != "" && m.queries != nil {
		cfg, err := m.queries.GetSessionConfig(ctx, database.GetSessionConfigParams{
			SessionID:     sessionID,
			SimulatorName: simulator,
		})
		if err == nil {
			return &RateLimitConfig{
				PerMinute: int(cfg.RateLimitPerMinute),
				PerDay:    int(cfg.RateLimitPerDay),
			}
		}
	}

	// Fall back to YAML default
	return m.getDefaultRateLimitConfig(simulator)
}

// SetSessionConfig saves session-specific config override to database
func (m *Manager) SetSessionConfig(ctx context.Context, sessionID, simulator string, timeout *TimeoutConfig, rateLimit *RateLimitConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.queries.UpsertSessionConfig(ctx, database.UpsertSessionConfigParams{
		SessionID:           sessionID,
		SimulatorName:       simulator,
		TimeoutMinMs:        int64(timeout.MinMs),
		TimeoutMaxMs:        int64(timeout.MaxMs),
		RateLimitPerMinute:  int64(rateLimit.PerMinute),
		RateLimitPerDay:     int64(rateLimit.PerDay),
	})
}

// DeleteSessionConfig removes session-specific config override
func (m *Manager) DeleteSessionConfig(ctx context.Context, sessionID, simulator string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.queries.DeleteSessionConfig(ctx, database.DeleteSessionConfigParams{
		SessionID:     sessionID,
		SimulatorName: simulator,
	})
}

// ListSessionConfigs returns all config overrides for a session
func (m *Manager) ListSessionConfigs(ctx context.Context, sessionID string) ([]database.SessionConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	configs, err := m.queries.ListSessionConfigs(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Handle empty results
	if configs == nil {
		return []database.SessionConfig{}, nil
	}

	return configs, nil
}

// getDefaultTimeoutConfig returns default timeout config for a simulator
func (m *Manager) getDefaultTimeoutConfig(simulator string) *TimeoutConfig {
	switch simulator {
	case "slack":
		return &m.defaultConfig.Slack.Timeout
	case "gmail":
		return &m.defaultConfig.Gmail.Timeout
	case "gdocs":
		return &m.defaultConfig.GoogleDocs.Timeout
	case "gsheets":
		return &m.defaultConfig.GoogleSheets.Timeout
	case "datadog":
		return &m.defaultConfig.Datadog.Timeout
	case "resend":
		return &m.defaultConfig.Resend.Timeout
	case "linear":
		return &m.defaultConfig.Linear.Timeout
	case "github":
		return &m.defaultConfig.GitHub.Timeout
	case "outlook":
		return &m.defaultConfig.Outlook.Timeout
	case "pagerduty":
		return &m.defaultConfig.PagerDuty.Timeout
	case "hubspot":
		return &m.defaultConfig.HubSpot.Timeout
	case "jira":
		return &m.defaultConfig.Jira.Timeout
	case "whatsapp":
		return &m.defaultConfig.WhatsApp.Timeout
	default:
		return &TimeoutConfig{MinMs: 0, MaxMs: 0}
	}
}

// getDefaultRateLimitConfig returns default rate limit config for a simulator
func (m *Manager) getDefaultRateLimitConfig(simulator string) *RateLimitConfig {
	switch simulator {
	case "slack":
		return &m.defaultConfig.Slack.RateLimit
	case "gmail":
		return &m.defaultConfig.Gmail.RateLimit
	case "gdocs":
		return &m.defaultConfig.GoogleDocs.RateLimit
	case "gsheets":
		return &m.defaultConfig.GoogleSheets.RateLimit
	case "datadog":
		return &m.defaultConfig.Datadog.RateLimit
	case "resend":
		return &m.defaultConfig.Resend.RateLimit
	case "linear":
		return &m.defaultConfig.Linear.RateLimit
	case "github":
		return &m.defaultConfig.GitHub.RateLimit
	case "outlook":
		return &m.defaultConfig.Outlook.RateLimit
	case "pagerduty":
		return &m.defaultConfig.PagerDuty.RateLimit
	case "hubspot":
		return &m.defaultConfig.HubSpot.RateLimit
	case "jira":
		return &m.defaultConfig.Jira.RateLimit
	case "whatsapp":
		return &m.defaultConfig.WhatsApp.RateLimit
	default:
		return &RateLimitConfig{PerMinute: 60, PerDay: 1000}
	}
}
