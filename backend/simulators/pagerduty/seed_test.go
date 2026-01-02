package pagerduty_test

import (
	"context"
	"database/sql"
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

func setupTestDBForSeed(t *testing.T) *database.Queries {
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

// TestPagerDutyInitialStateSeed demonstrates seeding arbitrary initial state for PagerDuty simulator
func TestPagerDutyInitialStateSeed(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDBForSeed(t)
	ctx := context.Background()

	// Setup: Create a new session
	sessionID := "pagerduty-seed-test-session"
	err := queries.CreateSession(ctx, sessionID)
	require.NoError(t, err, "Failed to create session")

	// Seed: Create custom services, escalation policies, users, and incidents
	services, escalationPolicies, users, incidents := seedPagerDutyTestData(t, ctx, queries, sessionID)

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorPagerDuty.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client that adds session header
	transport := &sessionHTTPTransport{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create PagerDuty client
	pdClient := pagerduty.NewClient("test-token", pagerduty.WithAPIEndpoint(server.URL))
	pdClient.HTTPClient = customClient

	// Verify: Check that services are queryable
	t.Run("VerifyServices", func(t *testing.T) {
		verifyServices(t, pdClient, services)
	})

	// Verify: Check that escalation policies are queryable
	t.Run("VerifyEscalationPolicies", func(t *testing.T) {
		verifyEscalationPolicies(t, pdClient, escalationPolicies)
	})

	// Verify: Check that users (oncalls) are queryable
	t.Run("VerifyUsers", func(t *testing.T) {
		verifyUsers(t, pdClient, users)
	})

	// Verify: Check that incidents are queryable
	t.Run("VerifyIncidents", func(t *testing.T) {
		verifyIncidents(t, pdClient, incidents)
	})

	// Verify: Check database isolation - ensure all data is correctly stored
	t.Run("VerifyDatabaseIsolation", func(t *testing.T) {
		verifyDatabaseIsolation(t, ctx, queries, sessionID, services, escalationPolicies, users, incidents)
	})
}

// seedPagerDutyTestData creates services, escalation policies, users, and incidents for testing
func seedPagerDutyTestData(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string) (
	services []struct {
		ID   string
		Name string
	},
	escalationPolicies []struct {
		ID   string
		Name string
	},
	users []struct {
		ID                 string
		Email              string
		EscalationPolicyID string
	},
	incidents []struct {
		ID        string
		Title     string
		ServiceID string
		Urgency   string
		Status    string
	},
) {
	t.Helper()

	// Seed: Create custom services (use session-specific IDs to avoid conflicts)
	services = []struct {
		ID   string
		Name string
	}{
		{
			ID:   "SERVICE_001_" + sessionID,
			Name: "Production API",
		},
		{
			ID:   "SERVICE_002_" + sessionID,
			Name: "Database Cluster",
		},
	}

	for _, s := range services {
		err := queries.CreatePagerDutyService(ctx, database.CreatePagerDutyServiceParams{
			ID:        s.ID,
			Name:      s.Name,
			SessionID: sessionID,
		})
		require.NoError(t, err, "Failed to create service: %s", s.Name)
	}

	// Seed: Create custom escalation policies (use session-specific IDs to avoid conflicts)
	escalationPolicies = []struct {
		ID   string
		Name string
	}{
		{
			ID:   "POLICY_001_" + sessionID,
			Name: "Default Escalation",
		},
		{
			ID:   "POLICY_002_" + sessionID,
			Name: "Critical Escalation",
		},
	}

	for _, p := range escalationPolicies {
		err := queries.CreatePagerDutyEscalationPolicy(ctx, database.CreatePagerDutyEscalationPolicyParams{
			ID:        p.ID,
			Name:      p.Name,
			SessionID: sessionID,
		})
		require.NoError(t, err, "Failed to create escalation policy: %s", p.Name)
	}

	// Seed: Create custom users (oncalls) (use session-specific IDs to avoid conflicts)
	users = []struct {
		ID                 string
		Email              string
		EscalationPolicyID string
	}{
		{
			ID:                 "USER_001_" + sessionID,
			Email:              "alice@example.com",
			EscalationPolicyID: escalationPolicies[0].ID,
		},
		{
			ID:                 "USER_002_" + sessionID,
			Email:              "bob@example.com",
			EscalationPolicyID: escalationPolicies[1].ID,
		},
	}

	for _, u := range users {
		err := queries.CreatePagerDutyOnCall(ctx, database.CreatePagerDutyOnCallParams{
			ID:                 u.ID,
			UserEmail:          u.Email,
			EscalationPolicyID: u.EscalationPolicyID,
			SessionID:          sessionID,
		})
		require.NoError(t, err, "Failed to create user: %s", u.Email)
	}

	// Seed: Create custom incidents (use session-specific IDs to avoid conflicts)
	incidents = []struct {
		ID        string
		Title     string
		ServiceID string
		Urgency   string
		Status    string
	}{
		{
			ID:        "INCIDENT_001_" + sessionID,
			Title:     "API server down",
			ServiceID: services[0].ID,
			Urgency:   "high",
			Status:    "triggered",
		},
		{
			ID:        "INCIDENT_002_" + sessionID,
			Title:     "Database latency spike",
			ServiceID: services[1].ID,
			Urgency:   "low",
			Status:    "acknowledged",
		},
		{
			ID:        "INCIDENT_003_" + sessionID,
			Title:     "Memory leak detected",
			ServiceID: services[0].ID,
			Urgency:   "high",
			Status:    "resolved",
		},
	}

	timestamp := int64(1640000000)
	for _, i := range incidents {
		_, err := queries.CreatePagerDutyIncident(ctx, database.CreatePagerDutyIncidentParams{
			ID:          i.ID,
			Title:       i.Title,
			ServiceID:   i.ServiceID,
			Urgency:     i.Urgency,
			Status:      i.Status,
			BodyDetails: sql.NullString{Valid: false},
			SessionID:   sessionID,
			CreatedAt:   timestamp,
			UpdatedAt:   timestamp,
		})
		require.NoError(t, err, "Failed to create incident: %s", i.Title)
	}

	return services, escalationPolicies, users, incidents
}

// verifyServices verifies that services can be queried
func verifyServices(t *testing.T, pdClient *pagerduty.Client, services []struct {
	ID   string
	Name string
}) {
	t.Helper()

	response, err := pdClient.ListServicesWithContext(context.Background(), pagerduty.ListServiceOptions{})
	require.NoError(t, err, "ListServices should succeed")
	assert.GreaterOrEqual(t, len(response.Services), len(services), "Should have at least the seeded services")

	// Create a map for easier lookup
	serviceMap := make(map[string]pagerduty.Service)
	for _, s := range response.Services {
		serviceMap[s.ID] = s
	}

	// Verify each seeded service
	for _, s := range services {
		service, found := serviceMap[s.ID]
		assert.True(t, found, "Service should be found: %s", s.ID)
		if found {
			assert.Equal(t, s.Name, service.Name, "Service name should match")
		}
	}
}

// verifyEscalationPolicies verifies that escalation policies can be queried
func verifyEscalationPolicies(t *testing.T, pdClient *pagerduty.Client, escalationPolicies []struct {
	ID   string
	Name string
}) {
	t.Helper()

	response, err := pdClient.ListEscalationPoliciesWithContext(context.Background(), pagerduty.ListEscalationPoliciesOptions{})
	require.NoError(t, err, "ListEscalationPolicies should succeed")
	assert.GreaterOrEqual(t, len(response.EscalationPolicies), len(escalationPolicies), "Should have at least the seeded policies")

	// Create a map for easier lookup
	policyMap := make(map[string]pagerduty.EscalationPolicy)
	for _, p := range response.EscalationPolicies {
		policyMap[p.ID] = p
	}

	// Verify each seeded policy
	for _, p := range escalationPolicies {
		policy, found := policyMap[p.ID]
		assert.True(t, found, "Escalation policy should be found: %s", p.ID)
		if found {
			assert.Equal(t, p.Name, policy.Name, "Policy name should match")
		}
	}
}

// verifyUsers verifies that users (oncalls) can be queried
func verifyUsers(t *testing.T, pdClient *pagerduty.Client, users []struct {
	ID                 string
	Email              string
	EscalationPolicyID string
}) {
	t.Helper()

	response, err := pdClient.ListOnCallsWithContext(context.Background(), pagerduty.ListOnCallOptions{})
	require.NoError(t, err, "ListOnCalls should succeed")
	assert.GreaterOrEqual(t, len(response.OnCalls), len(users), "Should have at least the seeded users")

	// Create a map for easier lookup
	userMap := make(map[string]pagerduty.OnCall)
	for _, u := range response.OnCalls {
		userMap[u.User.Email] = u
	}

	// Verify each seeded user
	for _, u := range users {
		oncall, found := userMap[u.Email]
		assert.True(t, found, "User should be found: %s", u.Email)
		if found {
			assert.Equal(t, u.EscalationPolicyID, oncall.EscalationPolicy.ID, "Escalation policy ID should match")
		}
	}
}

// verifyIncidents verifies that incidents can be queried
func verifyIncidents(t *testing.T, pdClient *pagerduty.Client, incidents []struct {
	ID        string
	Title     string
	ServiceID string
	Urgency   string
	Status    string
}) {
	t.Helper()

	response, err := pdClient.ListIncidentsWithContext(context.Background(), pagerduty.ListIncidentsOptions{})
	require.NoError(t, err, "ListIncidents should succeed")
	assert.GreaterOrEqual(t, len(response.Incidents), len(incidents), "Should have at least the seeded incidents")

	// Create a map for easier lookup
	incidentMap := make(map[string]pagerduty.Incident)
	for _, i := range response.Incidents {
		incidentMap[i.ID] = i
	}

	// Verify each seeded incident
	for _, i := range incidents {
		incident, found := incidentMap[i.ID]
		assert.True(t, found, "Incident should be found: %s", i.ID)
		if found {
			assert.Equal(t, i.Title, incident.Title, "Incident title should match")
			assert.Equal(t, i.ServiceID, incident.Service.ID, "Service ID should match")
			assert.Equal(t, i.Urgency, incident.Urgency, "Urgency should match")
			assert.Equal(t, i.Status, incident.Status, "Status should match")
		}
	}
}

// verifyDatabaseIsolation verifies database isolation
func verifyDatabaseIsolation(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string,
	services []struct {
		ID   string
		Name string
	},
	escalationPolicies []struct {
		ID   string
		Name string
	},
	users []struct {
		ID                 string
		Email              string
		EscalationPolicyID string
	},
	incidents []struct {
		ID        string
		Title     string
		ServiceID string
		Urgency   string
		Status    string
	}) {
	t.Helper()

	// Query services from database
	dbServices, err := queries.ListPagerDutyServices(ctx, sessionID)
	require.NoError(t, err, "ListPagerDutyServices should succeed")
	assert.Len(t, dbServices, len(services), "Should have correct number of services in database")

	// Verify service details
	for _, s := range services {
		dbService, err := queries.GetPagerDutyServiceByID(ctx, database.GetPagerDutyServiceByIDParams{
			ID:        s.ID,
			SessionID: sessionID,
		})
		require.NoError(t, err, "GetPagerDutyServiceByID should succeed for service: %s", s.Name)
		assert.Equal(t, s.Name, dbService.Name, "Service name should match in database")
	}

	// Query escalation policies from database
	dbPolicies, err := queries.ListPagerDutyEscalationPolicies(ctx, sessionID)
	require.NoError(t, err, "ListPagerDutyEscalationPolicies should succeed")
	assert.Len(t, dbPolicies, len(escalationPolicies), "Should have correct number of policies in database")

	// Verify policy details
	for _, p := range escalationPolicies {
		dbPolicy, err := queries.GetPagerDutyEscalationPolicyByID(ctx, database.GetPagerDutyEscalationPolicyByIDParams{
			ID:        p.ID,
			SessionID: sessionID,
		})
		require.NoError(t, err, "GetPagerDutyEscalationPolicyByID should succeed for policy: %s", p.Name)
		assert.Equal(t, p.Name, dbPolicy.Name, "Policy name should match in database")
	}

	// Query users from database
	dbUsers, err := queries.ListPagerDutyOnCalls(ctx, sessionID)
	require.NoError(t, err, "ListPagerDutyOnCalls should succeed")
	assert.Len(t, dbUsers, len(users), "Should have correct number of users in database")

	// Verify user details
	for _, u := range users {
		dbUser, err := queries.GetPagerDutyOnCallByID(ctx, database.GetPagerDutyOnCallByIDParams{
			ID:        u.ID,
			SessionID: sessionID,
		})
		require.NoError(t, err, "GetPagerDutyOnCallByID should succeed for user: %s", u.Email)
		assert.Equal(t, u.Email, dbUser.UserEmail, "User email should match in database")
		assert.Equal(t, u.EscalationPolicyID, dbUser.EscalationPolicyID, "Escalation policy ID should match in database")
	}

	// Query incidents from database
	dbIncidents, err := queries.ListPagerDutyIncidents(ctx, sessionID)
	require.NoError(t, err, "ListPagerDutyIncidents should succeed")
	assert.Len(t, dbIncidents, len(incidents), "Should have correct number of incidents in database")

	// Verify incident details
	for _, i := range incidents {
		dbIncident, err := queries.GetPagerDutyIncidentByID(ctx, database.GetPagerDutyIncidentByIDParams{
			ID:        i.ID,
			SessionID: sessionID,
		})
		require.NoError(t, err, "GetPagerDutyIncidentByID should succeed for incident: %s", i.Title)
		assert.Equal(t, i.Title, dbIncident.Title, "Incident title should match in database")
		assert.Equal(t, i.ServiceID, dbIncident.ServiceID, "Service ID should match in database")
		assert.Equal(t, i.Urgency, dbIncident.Urgency, "Urgency should match in database")
		assert.Equal(t, i.Status, dbIncident.Status, "Status should match in database")
	}
}
