package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the entire simulator configuration
type Config struct {
	Gmail       GmailConfig       `yaml:"gmail"`
	Slack       SlackConfig       `yaml:"slack"`
	Datadog     DatadogConfig     `yaml:"datadog"`
	Resend      ResendConfig      `yaml:"resend"`
	Linear      LinearConfig      `yaml:"linear"`
	GitHub      GitHubConfig      `yaml:"github"`
	Outlook     OutlookConfig     `yaml:"outlook"`
	PagerDuty   PagerDutyConfig   `yaml:"pagerduty"`
	HubSpot     HubSpotConfig     `yaml:"hubspot"`
	Jira        JiraConfig        `yaml:"jira"`
	WhatsApp    WhatsAppConfig    `yaml:"whatsapp"`
	GoogleDocs  GoogleDocsConfig  `yaml:"googledocs"`
	GoogleSheets GoogleSheetsConfig `yaml:"googlesheets"`
}

// SimulatorConfig is a generic configuration for simulators
type SimulatorConfig struct {
	Timeout   TimeoutConfig   `yaml:"timeout"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// GmailConfig contains Gmail simulator settings
type GmailConfig struct {
	Timeout   TimeoutConfig   `yaml:"timeout"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// SlackConfig contains Slack simulator settings
type SlackConfig struct {
	Timeout   TimeoutConfig   `yaml:"timeout"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// DatadogConfig contains Datadog simulator settings
type DatadogConfig struct {
	Timeout   TimeoutConfig   `yaml:"timeout"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// ResendConfig contains Resend simulator settings
type ResendConfig struct {
	Timeout   TimeoutConfig   `yaml:"timeout"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// LinearConfig contains Linear simulator settings
type LinearConfig struct {
	Timeout   TimeoutConfig   `yaml:"timeout"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// GitHubConfig contains GitHub simulator settings
type GitHubConfig struct {
	Timeout   TimeoutConfig   `yaml:"timeout"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// OutlookConfig contains Outlook simulator settings
type OutlookConfig struct {
	Timeout   TimeoutConfig   `yaml:"timeout"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// PagerDutyConfig contains PagerDuty simulator settings
type PagerDutyConfig struct {
	Timeout   TimeoutConfig   `yaml:"timeout"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// HubSpotConfig contains HubSpot simulator settings
type HubSpotConfig struct {
	Timeout   TimeoutConfig   `yaml:"timeout"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// JiraConfig contains Jira simulator settings
type JiraConfig struct {
	Timeout   TimeoutConfig   `yaml:"timeout"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// WhatsAppConfig contains WhatsApp simulator settings
type WhatsAppConfig struct {
	Timeout   TimeoutConfig   `yaml:"timeout"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// GoogleDocsConfig contains Google Docs simulator settings
type GoogleDocsConfig struct {
	Timeout   TimeoutConfig   `yaml:"timeout"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// GoogleSheetsConfig contains Google Sheets simulator settings
type GoogleSheetsConfig struct {
	Timeout   TimeoutConfig   `yaml:"timeout"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// TimeoutConfig defines artificial delay ranges
type TimeoutConfig struct {
	MinMs int `yaml:"min_ms"`
	MaxMs int `yaml:"max_ms"`
}

// RateLimitConfig defines request limits per time window
type RateLimitConfig struct {
	PerMinute int `yaml:"per_minute"`
	PerDay    int `yaml:"per_day"`
}

// Load reads and parses the YAML configuration file
func Load(path string) (*Config, error) {
	//nolint:gosec // G304: Reading config file path is intentional
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// Default returns a configuration with sensible defaults
func Default() *Config {
	return &Config{
		Gmail: GmailConfig{
			Timeout: TimeoutConfig{
				MinMs: 0,
				MaxMs: 0,
			},
			RateLimit: RateLimitConfig{
				PerMinute: 60,
				PerDay:    250,
			},
		},
		Slack: SlackConfig{
			Timeout: TimeoutConfig{
				MinMs: 0,
				MaxMs: 0,
			},
			RateLimit: RateLimitConfig{
				PerMinute: 60,
				PerDay:    1000,
			},
		},
		Datadog: DatadogConfig{
			Timeout: TimeoutConfig{
				MinMs: 0,
				MaxMs: 0,
			},
			RateLimit: RateLimitConfig{
				PerMinute: 30,
				PerDay:    300,
			},
		},
		Resend: ResendConfig{
			Timeout: TimeoutConfig{
				MinMs: 0,
				MaxMs: 0,
			},
			RateLimit: RateLimitConfig{
				PerMinute: 60,
				PerDay:    500,
			},
		},
		Linear: LinearConfig{
			Timeout: TimeoutConfig{
				MinMs: 0,
				MaxMs: 0,
			},
			RateLimit: RateLimitConfig{
				PerMinute: 60,
				PerDay:    500,
			},
		},
		GitHub: GitHubConfig{
			Timeout: TimeoutConfig{
				MinMs: 0,
				MaxMs: 0,
			},
			RateLimit: RateLimitConfig{
				PerMinute: 60,
				PerDay:    500,
			},
		},
		Outlook: OutlookConfig{
			Timeout: TimeoutConfig{
				MinMs: 0,
				MaxMs: 0,
			},
			RateLimit: RateLimitConfig{
				PerMinute: 60,
				PerDay:    1000,
			},
		},
		PagerDuty: PagerDutyConfig{
			Timeout: TimeoutConfig{
				MinMs: 0,
				MaxMs: 0,
			},
			RateLimit: RateLimitConfig{
				PerMinute: 30,
				PerDay:    300,
			},
		},
		HubSpot: HubSpotConfig{
			Timeout: TimeoutConfig{
				MinMs: 0,
				MaxMs: 0,
			},
			RateLimit: RateLimitConfig{
				PerMinute: 60,
				PerDay:    500,
			},
		},
		Jira: JiraConfig{
			Timeout: TimeoutConfig{
				MinMs: 0,
				MaxMs: 0,
			},
			RateLimit: RateLimitConfig{
				PerMinute: 60,
				PerDay:    500,
			},
		},
		WhatsApp: WhatsAppConfig{
			Timeout: TimeoutConfig{
				MinMs: 0,
				MaxMs: 0,
			},
			RateLimit: RateLimitConfig{
				PerMinute: 60,
				PerDay:    1000,
			},
		},
		GoogleDocs: GoogleDocsConfig{
			Timeout: TimeoutConfig{
				MinMs: 0,
				MaxMs: 0,
			},
			RateLimit: RateLimitConfig{
				PerMinute: 60,
				PerDay:    500,
			},
		},
		GoogleSheets: GoogleSheetsConfig{
			Timeout: TimeoutConfig{
				MinMs: 0,
				MaxMs: 0,
			},
			RateLimit: RateLimitConfig{
				PerMinute: 60,
				PerDay:    500,
			},
		},
	}
}
