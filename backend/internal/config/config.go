package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the entire simulator configuration
type Config struct {
	Gmail GmailConfig `yaml:"gmail"`
}

// GmailConfig contains Gmail simulator settings
type GmailConfig struct {
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
	}
}
