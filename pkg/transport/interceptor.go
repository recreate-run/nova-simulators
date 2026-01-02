package transport

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

// SimulatorTransport intercepts HTTP requests and redirects them to local simulators
type SimulatorTransport struct {
	baseTransport http.RoundTripper
	routingMap    map[string]string // Maps real host to simulator URL (e.g., "slack.com" -> "localhost:9000/slack")
}

// NewSimulatorTransport creates a new transport that routes requests to simulators
func NewSimulatorTransport(routingMap map[string]string) *SimulatorTransport {
	return &SimulatorTransport{
		baseTransport: http.DefaultTransport,
		routingMap:    routingMap,
	}
}

// RoundTrip implements http.RoundTripper interface
func (t *SimulatorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Check if we need to route this request to a simulator
	originalHost := req.URL.Host

	// Handle both slack.com and api.slack.com
	for realHost, simulatorURL := range t.routingMap {
		if strings.Contains(originalHost, realHost) {
			// Clone the request to avoid modifying the original
			clonedReq := req.Clone(req.Context())

			// Parse simulator URL to separate host and path prefix
			// Format: "localhost:9000/slack" -> host="localhost:9000", pathPrefix="/slack"
			parts := strings.SplitN(simulatorURL, "/", 2)
			simulatorHost := parts[0]
			pathPrefix := ""
			if len(parts) > 1 {
				pathPrefix = "/" + parts[1]
			}

			// Update the URL to point to the simulator
			clonedReq.URL.Scheme = "http"
			clonedReq.URL.Host = simulatorHost
			clonedReq.URL.Path = pathPrefix + clonedReq.URL.Path
			clonedReq.Host = simulatorHost

			log.Printf("[Interceptor] Routing %s %s → http://%s%s",
				req.Method, originalHost, clonedReq.URL.Host, clonedReq.URL.Path)

			// Forward the request to the simulator
			resp, err := t.baseTransport.RoundTrip(clonedReq)
			if err != nil {
				return nil, fmt.Errorf("simulator request failed: %w", err)
			}

			return resp, nil
		}
	}

	// Not a simulator request, pass through normally
	return t.baseTransport.RoundTrip(req)
}
