package middleware

import (
	mathrand "math/rand"
	"net/http"
	"time"

	"github.com/recreate-run/nova-simulators/internal/config"
	"github.com/recreate-run/nova-simulators/internal/session"
)

// Timeout returns a middleware that applies configurable artificial delay to simulate network latency
func Timeout(configManager *config.Manager, simulatorName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get session-specific or default config
			sessionID := session.FromContext(r.Context())
			cfg := configManager.GetTimeoutConfig(r.Context(), sessionID, simulatorName)

			// Apply configured timeout delay
			if cfg.MaxMs > 0 {
				delay := cfg.MinMs
				if cfg.MaxMs > cfg.MinMs {
					//nolint:gosec // G404: Using math/rand for delay simulation, not security-critical
					delay += mathrand.Intn(cfg.MaxMs - cfg.MinMs)
				}
				time.Sleep(time.Duration(delay) * time.Millisecond)
			}

			next.ServeHTTP(w, r)
		})
	}
}
