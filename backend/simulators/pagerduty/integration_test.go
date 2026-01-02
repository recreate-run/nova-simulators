package pagerduty_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
	simulatorPagerDuty "github.com/recreate-run/nova-simulators/simulators/pagerduty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// sessionHTTPTransport wraps http.RoundTripper and adds session header to all requests
type sessionHTTPTransport struct {
	sessionID string
	base      http.RoundTripper
}

func (t *sessionHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-Session-ID", t.sessionID)
	if t.base == nil {
		return http.DefaultTransport.RoundTrip(req)
	}
	return t.base.RoundTrip(req)
}

func setupTestDB(t *testing.T) *database.Queries {
	t.Helper()
	// Use in-memory SQLite database for tests
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err, "Failed to open test database")

	// Set goose dialect
	err = goose.SetDialect("sqlite3")
	require.NoError(t, err, "Failed to set goose dialect")

	// Run migrations
	err = goose.Up(db, "../../migrations")
	require.NoError(t, err, "Failed to run migrations")

	return database.New(db)
}

func setupTestServer(t *testing.T, queries *database.Queries, sessionID string) (*httptest.Server, *pagerduty.Client) {
	t.Helper()

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorPagerDuty.NewHandler(queries))
	server := httptest.NewServer(handler)

	// Create custom HTTP client that adds session header
	transport := &sessionHTTPTransport{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create PagerDuty client pointing to test server
	pdClient := pagerduty.NewClient("test-token", pagerduty.WithAPIEndpoint(server.URL))
	pdClient.HTTPClient = customClient

	return server, pdClient
}

// Helper function to create test data
func createTestService(t *testing.T, queries *database.Queries, sessionID string) string {
	t.Helper()
	serviceID := "SVC001"
	err := queries.CreatePagerDutyService(context.Background(), database.CreatePagerDutyServiceParams{
		ID:        serviceID,
		Name:      "Test Service",
		SessionID: sessionID,
	})
	require.NoError(t, err, "Failed to create test service")
	return serviceID
}

func createTestEscalationPolicy(t *testing.T, queries *database.Queries, sessionID string) string {
	t.Helper()
	policyID := "EP001"
	err := queries.CreatePagerDutyEscalationPolicy(context.Background(), database.CreatePagerDutyEscalationPolicyParams{
		ID:        policyID,
		Name:      "Default Escalation Policy",
		SessionID: sessionID,
	})
	require.NoError(t, err, "Failed to create test escalation policy")
	return policyID
}

func createTestOnCall(t *testing.T, queries *database.Queries, sessionID, policyID string) {
	t.Helper()
	err := queries.CreatePagerDutyOnCall(context.Background(), database.CreatePagerDutyOnCallParams{
		ID:                  "OC001",
		UserEmail:           "oncall@example.com",
		EscalationPolicyID:  policyID,
		SessionID:           sessionID,
	})
	require.NoError(t, err, "Failed to create test oncall")
}

func TestPagerDutySimulatorCreateIncident(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "pagerduty-test-session-1"

	// Create test service
	serviceID := createTestService(t, queries, sessionID)

	// Setup test server and client
	server, pdClient := setupTestServer(t, queries, sessionID)
	defer server.Close()

	t.Run("CreateIncident", func(t *testing.T) {
		// Create incident
		incident := pagerduty.CreateIncidentOptions{
			Title:   "Database server down",
			Service: &pagerduty.APIReference{ID: serviceID, Type: "service_reference"},
			Urgency: "high",
			Body: &pagerduty.APIDetails{
				Type:    "incident_body",
				Details: "Database server is not responding",
			},
		}

		createdIncident, err := pdClient.CreateIncidentWithContext(context.Background(), "", &incident)

		// Assertions
		require.NoError(t, err, "CreateIncident should not return error")
		assert.NotNil(t, createdIncident, "Should return created incident")
		assert.NotEmpty(t, createdIncident.ID, "Incident ID should not be empty")
		assert.Equal(t, "Database server down", createdIncident.Title, "Title should match")
		assert.Equal(t, "high", createdIncident.Urgency, "Urgency should match")
		assert.Equal(t, "triggered", createdIncident.Status, "Status should be triggered")
		assert.Equal(t, serviceID, createdIncident.Service.ID, "Service ID should match")
	})

	t.Run("CreateIncidentWithoutBody", func(t *testing.T) {
		// Create incident without body
		incident := pagerduty.CreateIncidentOptions{
			Title:   "Server alert",
			Service: &pagerduty.APIReference{ID: serviceID, Type: "service_reference"},
			Urgency: "low",
		}

		createdIncident, err := pdClient.CreateIncidentWithContext(context.Background(), "", &incident)

		// Assertions
		require.NoError(t, err, "CreateIncident should not return error")
		assert.NotNil(t, createdIncident, "Should return created incident")
		assert.Equal(t, "Server alert", createdIncident.Title, "Title should match")
		assert.Equal(t, "low", createdIncident.Urgency, "Urgency should match")
	})
}

func TestPagerDutySimulatorGetIncident(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "pagerduty-test-session-2"

	// Create test service
	serviceID := createTestService(t, queries, sessionID)

	// Setup test server and client
	server, pdClient := setupTestServer(t, queries, sessionID)
	defer server.Close()

	// Create a test incident
	incident := pagerduty.CreateIncidentOptions{
		Title:   "Test Incident",
		Service: &pagerduty.APIReference{ID: serviceID, Type: "service_reference"},
		Urgency: "high",
		Body: &pagerduty.APIDetails{
			Type:    "incident_body",
			Details: "Test details",
		},
	}

	created, err := pdClient.CreateIncidentWithContext(context.Background(), "", &incident)
	require.NoError(t, err, "CreateIncident should succeed")

	t.Run("GetIncident", func(t *testing.T) {
		// Get the incident
		retrieved, err := pdClient.GetIncidentWithContext(context.Background(), created.ID)

		// Assertions
		require.NoError(t, err, "GetIncident should not return error")
		assert.NotNil(t, retrieved, "Should return incident")
		assert.Equal(t, created.ID, retrieved.ID, "Incident ID should match")
		assert.Equal(t, "Test Incident", retrieved.Title, "Title should match")
		assert.Equal(t, "high", retrieved.Urgency, "Urgency should match")
		assert.Equal(t, "triggered", retrieved.Status, "Status should match")
		assert.Equal(t, serviceID, retrieved.Service.ID, "Service ID should match")
	})

	t.Run("GetNonExistentIncident", func(t *testing.T) {
		// Try to get a non-existent incident
		_, err := pdClient.GetIncidentWithContext(context.Background(), "nonexistent")

		// Assertions
		assert.Error(t, err, "Should return error for non-existent incident")
	})
}

func TestPagerDutySimulatorUpdateIncident(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "pagerduty-test-session-3"

	// Create test service
	serviceID := createTestService(t, queries, sessionID)

	// Setup test server and client
	server, pdClient := setupTestServer(t, queries, sessionID)
	defer server.Close()

	// Create a test incident
	incident := pagerduty.CreateIncidentOptions{
		Title:   "Test Incident",
		Service: &pagerduty.APIReference{ID: serviceID, Type: "service_reference"},
		Urgency: "high",
	}

	created, err := pdClient.CreateIncidentWithContext(context.Background(), "", &incident)
	require.NoError(t, err, "CreateIncident should succeed")

	t.Run("AcknowledgeIncident", func(t *testing.T) {
		// Acknowledge the incident
		_, err := pdClient.ManageIncidentsWithContext(context.Background(), "test@example.com", []pagerduty.ManageIncidentsOptions{
			{
				ID:     created.ID,
				Type:   "incident_reference",
				Status: "acknowledged",
			},
		})

		require.NoError(t, err, "ManageIncidents should not return error")

		// Verify status changed
		updated, err := pdClient.GetIncidentWithContext(context.Background(), created.ID)
		require.NoError(t, err, "GetIncident should succeed")
		assert.Equal(t, "acknowledged", updated.Status, "Status should be acknowledged")
	})

	t.Run("ResolveIncident", func(t *testing.T) {
		// Resolve the incident
		_, err := pdClient.ManageIncidentsWithContext(context.Background(), "test@example.com", []pagerduty.ManageIncidentsOptions{
			{
				ID:     created.ID,
				Type:   "incident_reference",
				Status: "resolved",
			},
		})

		require.NoError(t, err, "ManageIncidents should not return error")

		// Verify status changed
		updated, err := pdClient.GetIncidentWithContext(context.Background(), created.ID)
		require.NoError(t, err, "GetIncident should succeed")
		assert.Equal(t, "resolved", updated.Status, "Status should be resolved")
	})
}

func TestPagerDutySimulatorListIncidents(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "pagerduty-test-session-4"

	// Create test service
	serviceID := createTestService(t, queries, sessionID)

	// Setup test server and client
	server, pdClient := setupTestServer(t, queries, sessionID)
	defer server.Close()

	// Create a few test incidents
	for i := 1; i <= 3; i++ {
		incident := pagerduty.CreateIncidentOptions{
			Title:   fmt.Sprintf("Test Incident %d", i),
			Service: &pagerduty.APIReference{ID: serviceID, Type: "service_reference"},
			Urgency: "high",
		}
		_, err := pdClient.CreateIncidentWithContext(context.Background(), "", &incident)
		require.NoError(t, err, "CreateIncident should succeed")
	}

	t.Run("ListAllIncidents", func(t *testing.T) {
		// List incidents
		response, err := pdClient.ListIncidentsWithContext(context.Background(), pagerduty.ListIncidentsOptions{})

		// Assertions
		require.NoError(t, err, "ListIncidents should not return error")
		assert.NotNil(t, response, "Should return response")
		assert.GreaterOrEqual(t, len(response.Incidents), 3, "Should have at least 3 incidents")
	})
}

func TestPagerDutySimulatorListServices(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "pagerduty-test-session-5"

	// Create test services
	for i := 1; i <= 3; i++ {
		err := queries.CreatePagerDutyService(context.Background(), database.CreatePagerDutyServiceParams{
			ID:        fmt.Sprintf("SVC%03d", i),
			Name:      fmt.Sprintf("Service %d", i),
			SessionID: sessionID,
		})
		require.NoError(t, err, "Failed to create test service")
	}

	// Setup test server and client
	server, pdClient := setupTestServer(t, queries, sessionID)
	defer server.Close()

	t.Run("ListServices", func(t *testing.T) {
		// List services
		response, err := pdClient.ListServicesWithContext(context.Background(), pagerduty.ListServiceOptions{})

		// Assertions
		require.NoError(t, err, "ListServices should not return error")
		assert.NotNil(t, response, "Should return response")
		assert.GreaterOrEqual(t, len(response.Services), 3, "Should have at least 3 services")
	})
}

func TestPagerDutySimulatorListEscalationPolicies(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "pagerduty-test-session-6"

	// Create test escalation policies
	for i := 1; i <= 2; i++ {
		err := queries.CreatePagerDutyEscalationPolicy(context.Background(), database.CreatePagerDutyEscalationPolicyParams{
			ID:        fmt.Sprintf("EP%03d", i),
			Name:      fmt.Sprintf("Escalation Policy %d", i),
			SessionID: sessionID,
		})
		require.NoError(t, err, "Failed to create test escalation policy")
	}

	// Setup test server and client
	server, pdClient := setupTestServer(t, queries, sessionID)
	defer server.Close()

	t.Run("ListEscalationPolicies", func(t *testing.T) {
		// List escalation policies
		response, err := pdClient.ListEscalationPoliciesWithContext(context.Background(), pagerduty.ListEscalationPoliciesOptions{})

		// Assertions
		require.NoError(t, err, "ListEscalationPolicies should not return error")
		assert.NotNil(t, response, "Should return response")
		assert.GreaterOrEqual(t, len(response.EscalationPolicies), 2, "Should have at least 2 escalation policies")
	})
}

func TestPagerDutySimulatorListOnCalls(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "pagerduty-test-session-7"

	// Create test escalation policy
	policyID := createTestEscalationPolicy(t, queries, sessionID)

	// Create test oncalls
	for i := 1; i <= 2; i++ {
		err := queries.CreatePagerDutyOnCall(context.Background(), database.CreatePagerDutyOnCallParams{
			ID:                 fmt.Sprintf("OC%03d", i),
			UserEmail:          fmt.Sprintf("oncall%d@example.com", i),
			EscalationPolicyID: policyID,
			SessionID:          sessionID,
		})
		require.NoError(t, err, "Failed to create test oncall")
	}

	// Setup test server and client
	server, pdClient := setupTestServer(t, queries, sessionID)
	defer server.Close()

	t.Run("ListOnCalls", func(t *testing.T) {
		// List oncalls
		response, err := pdClient.ListOnCallsWithContext(context.Background(), pagerduty.ListOnCallOptions{})

		// Assertions
		require.NoError(t, err, "ListOnCalls should not return error")
		assert.NotNil(t, response, "Should return response")
		assert.GreaterOrEqual(t, len(response.OnCalls), 2, "Should have at least 2 oncalls")

		// Verify oncall data
		if len(response.OnCalls) > 0 {
			oncall := response.OnCalls[0]
			assert.NotEmpty(t, oncall.User.Email, "User email should not be empty")
			assert.NotEmpty(t, oncall.EscalationPolicy.ID, "Escalation policy ID should not be empty")
		}
	})
}

func TestPagerDutySimulatorEndToEnd(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "pagerduty-test-session-8"

	// Create test service and escalation policy
	serviceID := createTestService(t, queries, sessionID)
	policyID := createTestEscalationPolicy(t, queries, sessionID)
	createTestOnCall(t, queries, sessionID, policyID)

	// Setup test server and client
	server, pdClient := setupTestServer(t, queries, sessionID)
	defer server.Close()

	t.Run("CompleteWorkflow", func(t *testing.T) {
		// 1. List services
		servicesResp, err := pdClient.ListServicesWithContext(context.Background(), pagerduty.ListServiceOptions{})
		require.NoError(t, err, "ListServices should succeed")
		assert.GreaterOrEqual(t, len(servicesResp.Services), 1, "Should have at least 1 service")

		// 2. List oncalls
		oncallsResp, err := pdClient.ListOnCallsWithContext(context.Background(), pagerduty.ListOnCallOptions{})
		require.NoError(t, err, "ListOnCalls should succeed")
		assert.GreaterOrEqual(t, len(oncallsResp.OnCalls), 1, "Should have at least 1 oncall")

		// 3. Create an incident
		incident := pagerduty.CreateIncidentOptions{
			Title:   "Production outage",
			Service: &pagerduty.APIReference{ID: serviceID, Type: "service_reference"},
			Urgency: "high",
			Body: &pagerduty.APIDetails{
				Type:    "incident_body",
				Details: "API server returning 500 errors",
			},
		}
		created, err := pdClient.CreateIncidentWithContext(context.Background(), "", &incident)
		require.NoError(t, err, "CreateIncident should succeed")
		assert.Equal(t, "triggered", created.Status, "Initial status should be triggered")

		// 4. Acknowledge the incident
		_, err = pdClient.ManageIncidentsWithContext(context.Background(), "responder@example.com", []pagerduty.ManageIncidentsOptions{
			{
				ID:     created.ID,
				Type:   "incident_reference",
				Status: "acknowledged",
			},
		})
		require.NoError(t, err, "ManageIncidents (acknowledge) should succeed")

		// 5. Get the incident to verify acknowledgement
		retrieved, err := pdClient.GetIncidentWithContext(context.Background(), created.ID)
		require.NoError(t, err, "GetIncident should succeed")
		assert.Equal(t, "acknowledged", retrieved.Status, "Status should be acknowledged")

		// 6. Resolve the incident
		_, err = pdClient.ManageIncidentsWithContext(context.Background(), "responder@example.com", []pagerduty.ManageIncidentsOptions{
			{
				ID:     created.ID,
				Type:   "incident_reference",
				Status: "resolved",
			},
		})
		require.NoError(t, err, "ManageIncidents (resolve) should succeed")

		// 7. Verify resolution
		resolved, err := pdClient.GetIncidentWithContext(context.Background(), created.ID)
		require.NoError(t, err, "GetIncident should succeed")
		assert.Equal(t, "resolved", resolved.Status, "Status should be resolved")

		// 8. List incidents and find our incident
		incidentsResp, err := pdClient.ListIncidentsWithContext(context.Background(), pagerduty.ListIncidentsOptions{})
		require.NoError(t, err, "ListIncidents should succeed")

		found := false
		for _, inc := range incidentsResp.Incidents {
			if inc.ID == created.ID {
				found = true
				assert.Equal(t, "resolved", inc.Status, "Listed incident should be resolved")
				break
			}
		}
		assert.True(t, found, "Created incident should appear in list")
	})
}
