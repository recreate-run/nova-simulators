package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/recreate-run/nova-simulators/internal/config"
	"github.com/recreate-run/nova-simulators/internal/session"
)

// rateLimitState tracks request counts per session
type rateLimitState struct {
	minuteCount int
	minuteReset time.Time
	dailyCount  int
	dailyReset  time.Time
}

// RateLimiter manages rate limiting state across sessions
type RateLimiter struct {
	configManager *config.Manager
	simulator     string
	rateLimits    map[string]*rateLimitState
	mu            sync.Mutex
}

// NewRateLimiter creates a new rate limiter with the given configuration manager
func NewRateLimiter(configManager *config.Manager, simulator string) *RateLimiter {
	return &RateLimiter{
		configManager: configManager,
		simulator:     simulator,
		rateLimits:    make(map[string]*rateLimitState),
	}
}

// checkRateLimit enforces per-minute and per-day rate limits
func (rl *RateLimiter) checkRateLimit(ctx context.Context, sessionID string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Get session-specific or default config
	cfg := rl.configManager.GetRateLimitConfig(ctx, sessionID, rl.simulator)

	now := time.Now()
	state := rl.rateLimits[sessionID]

	// Initialize or reset state if needed
	if state == nil {
		rl.rateLimits[sessionID] = &rateLimitState{
			minuteCount: 1,
			minuteReset: now.Add(1 * time.Minute),
			dailyCount:  1,
			dailyReset:  now.Add(24 * time.Hour),
		}
		return nil
	}

	// Reset minute window if expired
	if now.After(state.minuteReset) {
		state.minuteCount = 0
		state.minuteReset = now.Add(1 * time.Minute)
	}

	// Reset daily window if expired
	if now.After(state.dailyReset) {
		state.dailyCount = 0
		state.dailyReset = now.Add(24 * time.Hour)
	}

	// Check limits using session-specific config
	if state.minuteCount >= cfg.PerMinute {
		return fmt.Errorf("per-minute rate limit exceeded")
	}
	if state.dailyCount >= cfg.PerDay {
		return fmt.Errorf("per-day rate limit exceeded")
	}

	// Increment counters
	state.minuteCount++
	state.dailyCount++

	return nil
}

// RateLimit returns a middleware that enforces rate limits per session
func RateLimit(configManager *config.Manager, simulatorName string) func(http.Handler) http.Handler {
	limiter := NewRateLimiter(configManager, simulatorName)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check rate limits
			sessionID := session.FromContext(r.Context())
			if err := limiter.checkRateLimit(r.Context(), sessionID); err != nil {
				log.Printf("[%s] ✗ Rate limit exceeded for session %s", simulatorName, sessionID)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]interface{}{
						"code":    429,
						"message": "Rate limit exceeded. Please try again later.",
						"status":  "RESOURCE_EXHAUSTED",
					},
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
