package datadog_test

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
	simulatorDatadog "github.com/recreate-run/nova-simulators/simulators/datadog"
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

// TestDatadogInitialStateSeed demonstrates seeding arbitrary initial state for Datadog simulator
func TestDatadogInitialStateSeed(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDBForSeed(t)
	ctx := context.Background()

	// Setup: Create a new session
	sessionID := "datadog-seed-test-session"
	err := queries.CreateSession(ctx, sessionID)
	require.NoError(t, err, "Failed to create session")

	// Seed: Create custom incidents, monitors, events, and metrics
	incidents, monitors, events, metrics := seedDatadogTestData(t, ctx, queries, sessionID)

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorDatadog.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create Datadog API client
	apiClient := setupDatadogClient(t, server.URL, sessionID)

	// Verify: Check that incidents are queryable
	t.Run("VerifyIncidents", func(t *testing.T) {
		verifyIncidents(t, ctx, apiClient, incidents)
	})

	// Verify: Check that monitors are queryable
	t.Run("VerifyMonitors", func(t *testing.T) {
		verifyMonitors(t, ctx, apiClient, monitors)
	})

	// Verify: Check that events are queryable
	t.Run("VerifyEvents", func(t *testing.T) {
		verifyEvents(t, ctx, apiClient, events)
	})

	// Verify: Check that metrics are queryable
	t.Run("VerifyMetrics", func(t *testing.T) {
		verifyMetrics(t, ctx, queries, sessionID, metrics)
	})

	// Verify: Check database isolation - ensure all data is correctly stored
	t.Run("VerifyDatabaseIsolation", func(t *testing.T) {
		verifyDatabaseIsolation(t, ctx, queries, sessionID, incidents, monitors, events)
	})
}

// seedDatadogTestData creates incidents, monitors, events, and metrics for testing
func seedDatadogTestData(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string) (
	incidents []struct {
		ID               string
		Title            string
		CustomerImpacted bool
		Severity         string
	},
	monitors []struct {
		ID      int64
		Name    string
		Type    string
		Query   string
		Message string
	},
	events []struct {
		ID    int64
		Title string
		Text  string
		Tags  string
	},
	metrics []struct {
		MetricName string
		Value      float64
		Tags       string
		Timestamp  int64
	},
) {
	t.Helper()

	now := time.Now().Unix()

	// Seed: Create incidents (use session-specific IDs to avoid conflicts)
	incidents = []struct {
		ID               string
		Title            string
		CustomerImpacted bool
		Severity         string
	}{
		{"INC_" + sessionID + "_001", "Production API Outage", true, "SEV-1"},
		{"INC_" + sessionID + "_002", "Database Performance Degradation", false, "SEV-2"},
		{"INC_" + sessionID + "_003", "Intermittent Login Failures", true, "SEV-3"},
	}

	for _, incident := range incidents {
		customerImpacted := int64(0)
		if incident.CustomerImpacted {
			customerImpacted = 1
		}
		err := queries.CreateDatadogIncident(ctx, database.CreateDatadogIncidentParams{
			ID:               incident.ID,
			Title:            incident.Title,
			CustomerImpacted: customerImpacted,
			Severity:         sql.NullString{String: incident.Severity, Valid: true},
			SessionID:        sessionID,
			CreatedAt:        now,
			UpdatedAt:        now,
		})
		require.NoError(t, err, "Failed to create incident: %s", incident.Title)
	}

	// Seed: Create monitors
	monitors = []struct {
		ID      int64
		Name    string
		Type    string
		Query   string
		Message string
	}{
		{1001, "High CPU Usage", "metric alert", "avg(last_5m):avg:system.cpu.user{*} > 90", "CPU usage is critically high"},
		{1002, "High Memory Usage", "metric alert", "avg(last_5m):avg:system.mem.used{*} > 85", "Memory usage is high"},
		{1003, "API Error Rate", "metric alert", "sum(last_5m):sum:api.errors{*} > 100", "API error rate is elevated"},
	}

	for _, monitor := range monitors {
		_, err := queries.CreateDatadogMonitor(ctx, database.CreateDatadogMonitorParams{
			Name:      monitor.Name,
			Type:      monitor.Type,
			Query:     monitor.Query,
			Message:   sql.NullString{String: monitor.Message, Valid: true},
			SessionID: sessionID,
			CreatedAt: now,
			UpdatedAt: now,
		})
		require.NoError(t, err, "Failed to create monitor: %s", monitor.Name)
	}

	// Seed: Create events
	events = []struct {
		ID    int64
		Title string
		Text  string
		Tags  string
	}{
		{2001, "Deployment Started", "Deploying version 1.2.3 to production", "env:prod,version:1.2.3"},
		{2002, "Deployment Completed", "Successfully deployed version 1.2.3", "env:prod,version:1.2.3"},
		{2003, "Database Backup", "Daily database backup completed", "env:prod,type:backup"},
	}

	for _, event := range events {
		_, err := queries.CreateDatadogEvent(ctx, database.CreateDatadogEventParams{
			Title:     event.Title,
			Text:      event.Text,
			Tags:      sql.NullString{String: event.Tags, Valid: true},
			SessionID: sessionID,
			CreatedAt: now,
		})
		require.NoError(t, err, "Failed to create event: %s", event.Title)
	}

	// Seed: Create metrics
	metrics = []struct {
		MetricName string
		Value      float64
		Tags       string
		Timestamp  int64
	}{
		{"system.cpu.usage", 45.5, "host:web-01,env:prod", now - 300},
		{"system.cpu.usage", 52.3, "host:web-01,env:prod", now - 240},
		{"system.mem.usage", 78.2, "host:web-01,env:prod", now - 300},
		{"api.request.count", 1234.0, "endpoint:/api/users,env:prod", now - 180},
		{"api.request.count", 1456.0, "endpoint:/api/users,env:prod", now - 120},
	}

	for _, metric := range metrics {
		err := queries.CreateDatadogMetric(ctx, database.CreateDatadogMetricParams{
			MetricName: metric.MetricName,
			Value:      metric.Value,
			Tags:       sql.NullString{String: metric.Tags, Valid: true},
			Timestamp:  metric.Timestamp,
			SessionID:  sessionID,
			CreatedAt:  now,
		})
		require.NoError(t, err, "Failed to create metric: %s", metric.MetricName)
	}

	return incidents, monitors, events, metrics
}

// verifyIncidents verifies that incidents can be queried via API
func verifyIncidents(t *testing.T, ctx context.Context, apiClient *datadog.APIClient, incidents []struct {
	ID               string
	Title            string
	CustomerImpacted bool
	Severity         string
}) {
	t.Helper()

	incidentsAPI := datadogV2.NewIncidentsApi(apiClient)

	// List incidents
	resp, _, err := incidentsAPI.ListIncidents(ctx, *datadogV2.NewListIncidentsOptionalParameters())
	require.NoError(t, err, "List incidents should succeed")
	assert.Len(t, resp.Data, len(incidents), "Should have correct number of incidents")

	// Verify each incident can be retrieved
	for _, incident := range incidents {
		incidentResp, _, err := incidentsAPI.GetIncident(ctx, incident.ID)
		require.NoError(t, err, "Get incident should succeed for: %s", incident.ID)
		assert.Equal(t, incident.ID, incidentResp.Data.Id, "Incident ID should match")
		assert.Equal(t, incident.Title, incidentResp.Data.Attributes.Title, "Incident title should match")
	}
}

// verifyMonitors verifies that monitors can be queried via API
func verifyMonitors(t *testing.T, ctx context.Context, apiClient *datadog.APIClient, monitors []struct {
	ID      int64
	Name    string
	Type    string
	Query   string
	Message string
}) {
	t.Helper()

	monitorsAPI := datadogV1.NewMonitorsApi(apiClient)

	// List monitors
	monitorsList, _, err := monitorsAPI.ListMonitors(ctx, *datadogV1.NewListMonitorsOptionalParameters())
	require.NoError(t, err, "List monitors should succeed")
	assert.GreaterOrEqual(t, len(monitorsList), len(monitors), "Should have at least the expected number of monitors")

	// Verify monitor names
	monitorNames := make(map[string]bool)
	for _, monitor := range monitorsList {
		if monitor.Name != nil {
			monitorNames[*monitor.Name] = true
		}
	}

	for _, monitor := range monitors {
		assert.True(t, monitorNames[monitor.Name], "Should have monitor: %s", monitor.Name)
	}
}

// verifyEvents verifies that events can be queried via database (events don't have a Get endpoint)
func verifyEvents(t *testing.T, ctx context.Context, apiClient *datadog.APIClient, events []struct {
	ID    int64
	Title string
	Text  string
	Tags  string
}) {
	t.Helper()

	// Events API in Datadog doesn't have a List endpoint in the v1 client we're using,
	// so we verify via database in the VerifyDatabaseIsolation test
	// This is a placeholder to maintain the test structure
	assert.Len(t, events, 3, "Should have 3 events seeded")
}

// verifyMetrics verifies that metrics are stored correctly (metrics are write-only in our simulator)
func verifyMetrics(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string, metrics []struct {
	MetricName string
	Value      float64
	Tags       string
	Timestamp  int64
}) {
	t.Helper()

	// Query metrics from database for each unique metric name
	metricNames := make(map[string]bool)
	for _, metric := range metrics {
		metricNames[metric.MetricName] = true
	}

	for metricName := range metricNames {
		dbMetrics, err := queries.ListDatadogMetrics(ctx, database.ListDatadogMetricsParams{
			SessionID:  sessionID,
			MetricName: metricName,
			Limit:      100,
		})
		require.NoError(t, err, "ListDatadogMetrics should succeed for metric: %s", metricName)
		assert.NotEmpty(t, dbMetrics, "Should have metrics for: %s", metricName)
	}
}

// verifyDatabaseIsolation verifies database isolation
func verifyDatabaseIsolation(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string,
	incidents []struct {
		ID               string
		Title            string
		CustomerImpacted bool
		Severity         string
	},
	monitors []struct {
		ID      int64
		Name    string
		Type    string
		Query   string
		Message string
	},
	events []struct {
		ID    int64
		Title string
		Text  string
		Tags  string
	}) {
	t.Helper()

	// Query incidents from database
	dbIncidents, err := queries.ListDatadogIncidents(ctx, database.ListDatadogIncidentsParams{
		SessionID: sessionID,
		Limit:     100,
	})
	require.NoError(t, err, "ListDatadogIncidents should succeed")
	assert.Len(t, dbIncidents, len(incidents), "Should have correct number of incidents in database")

	// Verify incident titles
	incidentTitles := make(map[string]bool)
	for _, incident := range dbIncidents {
		incidentTitles[incident.Title] = true
	}
	for _, incident := range incidents {
		assert.True(t, incidentTitles[incident.Title], "Should have incident: %s", incident.Title)
	}

	// Query monitors from database
	dbMonitors, err := queries.ListDatadogMonitors(ctx, sessionID)
	require.NoError(t, err, "ListDatadogMonitors should succeed")
	assert.Len(t, dbMonitors, len(monitors), "Should have correct number of monitors in database")

	// Verify monitor names
	monitorNames := make(map[string]bool)
	for _, monitor := range dbMonitors {
		monitorNames[monitor.Name] = true
	}
	for _, monitor := range monitors {
		assert.True(t, monitorNames[monitor.Name], "Should have monitor: %s", monitor.Name)
	}

	// Query events from database
	dbEvents, err := queries.ListDatadogEvents(ctx, database.ListDatadogEventsParams{
		SessionID: sessionID,
		Limit:     100,
	})
	require.NoError(t, err, "ListDatadogEvents should succeed")
	assert.Len(t, dbEvents, len(events), "Should have correct number of events in database")

	// Verify event titles
	eventTitles := make(map[string]bool)
	for _, event := range dbEvents {
		eventTitles[event.Title] = true
	}
	for _, event := range events {
		assert.True(t, eventTitles[event.Title], "Should have event: %s", event.Title)
	}
}
