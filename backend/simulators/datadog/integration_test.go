package datadog_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

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

// sessionHTTPTransport wraps http.RoundTripper and adds session header to all requests
type sessionHTTPTransport struct {
	sessionID string
}

func (t *sessionHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-Session-ID", t.sessionID)
	return http.DefaultTransport.RoundTrip(req)
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

func setupDatadogClient(t *testing.T, serverURL, sessionID string) *datadog.APIClient {
	t.Helper()

	configuration := datadog.NewConfiguration()
	configuration.Servers = datadog.ServerConfigurations{
		{
			URL: serverURL + "/datadog",
		},
	}

	// Enable unstable incident operations
	configuration.SetUnstableOperationEnabled("v2.CreateIncident", true)
	configuration.SetUnstableOperationEnabled("v2.ListIncidents", true)
	configuration.SetUnstableOperationEnabled("v2.GetIncident", true)
	configuration.SetUnstableOperationEnabled("v2.UpdateIncident", true)

	// Set custom HTTP client with session header
	configuration.HTTPClient = &http.Client{
		Transport: &sessionHTTPTransport{
			sessionID: sessionID,
		},
	}

	return datadog.NewAPIClient(configuration)
}

// Incidents Tests (v2 API)

func TestDatadogIncidents(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "datadog-test-session-1"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorDatadog.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create Datadog API client
	apiClient := setupDatadogClient(t, server.URL, sessionID)
	incidentsAPI := datadogV2.NewIncidentsApi(apiClient)

	ctx := context.Background()

	t.Run("CreateIncident", func(t *testing.T) {
		title := "Production API Outage"
		customerImpacted := true

		body := datadogV2.IncidentCreateRequest{
			Data: datadogV2.IncidentCreateData{
				Type: datadogV2.INCIDENTTYPE_INCIDENTS,
				Attributes: datadogV2.IncidentCreateAttributes{
					Title:            title,
					CustomerImpacted: customerImpacted,
				},
			},
		}

		resp, r, err := incidentsAPI.CreateIncident(ctx, body)
		if err == nil {
			defer r.Body.Close()
		}

		// Assertions
		require.NoError(t, err, "CreateIncident should not return error")
		assert.Equal(t, http.StatusCreated, r.StatusCode, "Should return 201 Created")
		assert.NotNil(t, resp.Data, "Should return incident data")
		assert.NotEmpty(t, resp.Data.Id, "Incident ID should not be empty")
		assert.Equal(t, title, resp.Data.Attributes.Title, "Title should match")
		assert.NotNil(t, resp.Data.Attributes.CustomerImpacted, "CustomerImpacted should not be nil")
		assert.Equal(t, customerImpacted, *resp.Data.Attributes.CustomerImpacted, "CustomerImpacted should match")

		// TODO: Test severity field - Datadog's IncidentFieldAttributes uses complex union types
		// that require special marshaling handling which is not critical for the simulator MVP
	})

	t.Run("GetIncident", func(t *testing.T) {
		// Create an incident first
		title := "Database Performance Degradation"
		body := datadogV2.IncidentCreateRequest{
			Data: datadogV2.IncidentCreateData{
				Type: datadogV2.INCIDENTTYPE_INCIDENTS,
				Attributes: datadogV2.IncidentCreateAttributes{
					Title:            title,
					CustomerImpacted: false,
				},
			},
		}

		createResp, createR, err := incidentsAPI.CreateIncident(ctx, body)
		if err == nil {
			defer createR.Body.Close()
		}
		require.NoError(t, err, "CreateIncident should succeed")
		incidentID := createResp.Data.Id

		// Get the incident
		getResp, r, err := incidentsAPI.GetIncident(ctx, incidentID, *datadogV2.NewGetIncidentOptionalParameters())
		if err == nil {
			defer r.Body.Close()
		}

		// Assertions
		require.NoError(t, err, "GetIncident should not return error")
		assert.Equal(t, http.StatusOK, r.StatusCode, "Should return 200 OK")
		assert.Equal(t, incidentID, getResp.Data.Id, "Incident ID should match")
		assert.Equal(t, title, getResp.Data.Attributes.Title, "Title should match")
		assert.NotNil(t, getResp.Data.Attributes.CustomerImpacted, "CustomerImpacted should not be nil")
		assert.False(t, *getResp.Data.Attributes.CustomerImpacted, "CustomerImpacted should be false")
	})

	t.Run("UpdateIncident", func(t *testing.T) {
		// Create an incident first
		body := datadogV2.IncidentCreateRequest{
			Data: datadogV2.IncidentCreateData{
				Type: datadogV2.INCIDENTTYPE_INCIDENTS,
				Attributes: datadogV2.IncidentCreateAttributes{
					Title:            "Initial Title",
					CustomerImpacted: false,
				},
			},
		}

		createResp, createR, err := incidentsAPI.CreateIncident(ctx, body)
		if err == nil {
			defer createR.Body.Close()
		}
		require.NoError(t, err, "CreateIncident should succeed")
		incidentID := createResp.Data.Id

		// Update the incident
		newTitle := "Updated Title"
		newCustomerImpacted := true

		updateBody := datadogV2.IncidentUpdateRequest{
			Data: datadogV2.IncidentUpdateData{
				Id:   incidentID,
				Type: datadogV2.INCIDENTTYPE_INCIDENTS,
				Attributes: &datadogV2.IncidentUpdateAttributes{
					Title:            &newTitle,
					CustomerImpacted: &newCustomerImpacted,
				},
			},
		}

		updateResp, r, err := incidentsAPI.UpdateIncident(ctx, incidentID, updateBody)
		if err == nil {
			defer r.Body.Close()
		}

		// Assertions
		require.NoError(t, err, "UpdateIncident should not return error")
		assert.Equal(t, http.StatusOK, r.StatusCode, "Should return 200 OK")
		assert.Equal(t, newTitle, updateResp.Data.Attributes.Title, "Title should be updated")
		assert.NotNil(t, updateResp.Data.Attributes.CustomerImpacted, "CustomerImpacted should not be nil")
		assert.True(t, *updateResp.Data.Attributes.CustomerImpacted, "CustomerImpacted should be updated")

		// TODO: Test severity field - complex union type marshaling not implemented in MVP
	})

	t.Run("ListIncidents", func(t *testing.T) {
		// Create multiple incidents
		for i := 1; i <= 3; i++ {
			body := datadogV2.IncidentCreateRequest{
				Data: datadogV2.IncidentCreateData{
					Type: datadogV2.INCIDENTTYPE_INCIDENTS,
					Attributes: datadogV2.IncidentCreateAttributes{
						Title:            "List Test Incident",
						CustomerImpacted: false,
					},
				},
			}
			_, createR, err := incidentsAPI.CreateIncident(ctx, body)
			require.NoError(t, err, "CreateIncident should succeed")
			if createR != nil && createR.Body != nil {
				_ = createR.Body.Close()
			}
		}

		// List incidents
		opts := datadogV2.NewListIncidentsOptionalParameters()
		pageSize := int64(10)
		opts.PageSize = &pageSize

		resp, r, err := incidentsAPI.ListIncidents(ctx, *opts)
		if err == nil {
			defer r.Body.Close()
		}

		// Assertions
		require.NoError(t, err, "ListIncidents should not return error")
		assert.Equal(t, http.StatusOK, r.StatusCode, "Should return 200 OK")
		assert.GreaterOrEqual(t, len(resp.GetData()), 3, "Should have at least 3 incidents")
	})
}

// Monitors Tests (v1 API)

func TestDatadogMonitors(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "datadog-test-session-2"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorDatadog.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create Datadog API client
	apiClient := setupDatadogClient(t, server.URL, sessionID)
	monitorsAPI := datadogV1.NewMonitorsApi(apiClient)

	ctx := context.Background()

	t.Run("CreateMonitor", func(t *testing.T) {
		name := "CPU Usage Monitor"
		monitorType := datadogV1.MONITORTYPE_METRIC_ALERT
		query := "avg(last_5m):avg:system.cpu.user{*} > 90"
		message := "CPU usage is above 90%"

		body := datadogV1.Monitor{
			Name:    &name,
			Type:    monitorType,
			Query:   query,
			Message: &message,
		}

		resp, r, err := monitorsAPI.CreateMonitor(ctx, body)
		if err == nil {
			defer r.Body.Close()
		}

		// Assertions
		require.NoError(t, err, "CreateMonitor should not return error")
		assert.Equal(t, http.StatusCreated, r.StatusCode, "Should return 201 Created")
		assert.NotNil(t, resp.Id, "Should have monitor ID")
		assert.Positive(t, *resp.Id, "Monitor ID should be positive")
		assert.Equal(t, name, *resp.Name, "Name should match")
		assert.Equal(t, query, resp.Query, "Query should match")
		assert.Equal(t, message, *resp.Message, "Message should match")
	})

	t.Run("GetMonitor", func(t *testing.T) {
		// Create a monitor first
		name := "Memory Usage Monitor"
		monitorType := datadogV1.MONITORTYPE_METRIC_ALERT
		query := "avg(last_5m):avg:system.mem.used{*} > 80"

		body := datadogV1.Monitor{
			Name:  &name,
			Type:  monitorType,
			Query: query,
		}

		createResp, createR, err := monitorsAPI.CreateMonitor(ctx, body)
		if err == nil {
			defer createR.Body.Close()
		}
		require.NoError(t, err, "CreateMonitor should succeed")
		monitorID := *createResp.Id

		// Get the monitor
		getResp, r, err := monitorsAPI.GetMonitor(ctx, monitorID, *datadogV1.NewGetMonitorOptionalParameters())
		if err == nil {
			defer r.Body.Close()
		}

		// Assertions
		require.NoError(t, err, "GetMonitor should not return error")
		assert.Equal(t, http.StatusOK, r.StatusCode, "Should return 200 OK")
		assert.Equal(t, monitorID, *getResp.Id, "Monitor ID should match")
		assert.Equal(t, name, *getResp.Name, "Name should match")
		assert.Equal(t, query, getResp.Query, "Query should match")
	})

	t.Run("UpdateMonitor", func(t *testing.T) {
		// Create a monitor first
		name := "Disk Space Monitor"
		monitorType := datadogV1.MONITORTYPE_METRIC_ALERT
		query := "avg(last_5m):avg:system.disk.used{*} > 85"

		body := datadogV1.Monitor{
			Name:  &name,
			Type:  monitorType,
			Query: query,
		}

		createResp, createR, err := monitorsAPI.CreateMonitor(ctx, body)
		if err == nil {
			defer createR.Body.Close()
		}
		require.NoError(t, err, "CreateMonitor should succeed")
		monitorID := *createResp.Id

		// Update the monitor
		newName := "Updated Disk Space Monitor"
		newQuery := "avg(last_5m):avg:system.disk.used{*} > 90"
		newMessage := "Disk space is critically low"

		updateBody := datadogV1.MonitorUpdateRequest{
			Name:    &newName,
			Query:   &newQuery,
			Message: &newMessage,
		}

		updateResp, r, err := monitorsAPI.UpdateMonitor(ctx, monitorID, updateBody)
		if err == nil {
			defer r.Body.Close()
		}

		// Assertions
		require.NoError(t, err, "UpdateMonitor should not return error")
		assert.Equal(t, http.StatusOK, r.StatusCode, "Should return 200 OK")
		assert.Equal(t, newName, *updateResp.Name, "Name should be updated")
		assert.Equal(t, newQuery, updateResp.Query, "Query should be updated")
		assert.Equal(t, newMessage, *updateResp.Message, "Message should be updated")
	})

	t.Run("DeleteMonitor", func(t *testing.T) {
		// Create a monitor first
		name := "Temporary Monitor"
		monitorType := datadogV1.MONITORTYPE_METRIC_ALERT
		query := "avg(last_5m):avg:system.load.1{*} > 5"

		body := datadogV1.Monitor{
			Name:  &name,
			Type:  monitorType,
			Query: query,
		}

		createResp, createR, err := monitorsAPI.CreateMonitor(ctx, body)
		if err == nil {
			defer createR.Body.Close()
		}
		require.NoError(t, err, "CreateMonitor should succeed")
		monitorID := *createResp.Id

		// Delete the monitor
		_, r, err := monitorsAPI.DeleteMonitor(ctx, monitorID, *datadogV1.NewDeleteMonitorOptionalParameters())
		if err == nil {
			defer r.Body.Close()
		}

		// Assertions
		require.NoError(t, err, "DeleteMonitor should not return error")
		assert.Equal(t, http.StatusNoContent, r.StatusCode, "Should return 204 No Content")

		// Verify deletion
		_, getR, getErr := monitorsAPI.GetMonitor(ctx, monitorID, *datadogV1.NewGetMonitorOptionalParameters())
		if getErr == nil {
			defer getR.Body.Close()
		}
		assert.Error(t, getErr, "Getting deleted monitor should return error")
	})

	t.Run("ListMonitors", func(t *testing.T) {
		// Create multiple monitors
		for i := 1; i <= 3; i++ {
			name := "List Test Monitor"
			monitorType := datadogV1.MONITORTYPE_METRIC_ALERT
			query := "avg(last_5m):avg:system.cpu.user{*} > 50"

			body := datadogV1.Monitor{
				Name:  &name,
				Type:  monitorType,
				Query: query,
			}
			_, createR, err := monitorsAPI.CreateMonitor(ctx, body)
			require.NoError(t, err, "CreateMonitor should succeed")
			if createR != nil && createR.Body != nil {
				_ = createR.Body.Close()
			}
		}

		// List monitors
		resp, r, err := monitorsAPI.ListMonitors(ctx, *datadogV1.NewListMonitorsOptionalParameters())
		if err == nil {
			defer r.Body.Close()
		}

		// Assertions
		require.NoError(t, err, "ListMonitors should not return error")
		assert.Equal(t, http.StatusOK, r.StatusCode, "Should return 200 OK")
		assert.GreaterOrEqual(t, len(resp), 3, "Should have at least 3 monitors")
	})
}

// Events Tests (v1 API)

func TestDatadogEvents(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "datadog-test-session-3"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorDatadog.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create Datadog API client
	apiClient := setupDatadogClient(t, server.URL, sessionID)
	eventsAPI := datadogV1.NewEventsApi(apiClient)

	ctx := context.Background()

	t.Run("PostEvent", func(t *testing.T) {
		title := "Deployment Completed"
		text := "Successfully deployed version 2.3.4 to production"
		tags := []string{"env:production", "service:api", "version:2.3.4"}

		body := datadogV1.EventCreateRequest{
			Title: title,
			Text:  text,
			Tags:  tags,
		}

		resp, r, err := eventsAPI.CreateEvent(ctx, body)
		if err == nil {
			defer r.Body.Close()
		}

		// Assertions
		require.NoError(t, err, "CreateEvent should not return error")
		assert.Equal(t, http.StatusAccepted, r.StatusCode, "Should return 202 Accepted")
		assert.NotNil(t, resp.Status, "Status should not be nil")
		assert.Equal(t, "ok", *resp.Status, "Status should be ok")
		assert.NotNil(t, resp.Event, "Should have event data")
		assert.NotNil(t, resp.Event.Id, "Event ID should not be nil")
		assert.Positive(t, *resp.Event.Id, "Event ID should be positive")
		assert.NotNil(t, resp.Event.Title, "Title should not be nil")
		assert.Equal(t, title, *resp.Event.Title, "Title should match")
		assert.NotNil(t, resp.Event.Text, "Text should not be nil")
		assert.Equal(t, text, *resp.Event.Text, "Text should match")
		assert.ElementsMatch(t, tags, resp.Event.Tags, "Tags should match")
	})

	t.Run("PostEventWithoutTags", func(t *testing.T) {
		title := "Manual Restart"
		text := "Server manually restarted by operator"

		body := datadogV1.EventCreateRequest{
			Title: title,
			Text:  text,
		}

		resp, r, err := eventsAPI.CreateEvent(ctx, body)
		if err == nil {
			defer r.Body.Close()
		}

		// Assertions
		require.NoError(t, err, "CreateEvent should not return error")
		assert.Equal(t, http.StatusAccepted, r.StatusCode, "Should return 202 Accepted")
		assert.NotNil(t, resp.Status, "Status should not be nil")
		assert.Equal(t, "ok", *resp.Status, "Status should be ok")
		assert.NotNil(t, resp.Event, "Should have event data")
		assert.NotNil(t, resp.Event.Title, "Title should not be nil")
		assert.Equal(t, title, *resp.Event.Title, "Title should match")
		assert.NotNil(t, resp.Event.Text, "Text should not be nil")
		assert.Equal(t, text, *resp.Event.Text, "Text should match")
	})
}

// Metrics Tests (v2 API)

func TestDatadogMetrics(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "datadog-test-session-4"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorDatadog.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create Datadog API client
	apiClient := setupDatadogClient(t, server.URL, sessionID)
	metricsAPI := datadogV2.NewMetricsApi(apiClient)

	ctx := context.Background()

	t.Run("SubmitMetrics", func(t *testing.T) {
		metricName := "custom.api.response_time"
		value := 125.5
		tags := []string{"env:production", "endpoint:/api/users"}

		point := datadogV2.MetricPoint{
			Value: &value,
		}

		series := datadogV2.MetricSeries{
			Metric: metricName,
			Points: []datadogV2.MetricPoint{point},
			Tags:   tags,
		}

		body := datadogV2.MetricPayload{
			Series: []datadogV2.MetricSeries{series},
		}

		_, r, err := metricsAPI.SubmitMetrics(ctx, body, *datadogV2.NewSubmitMetricsOptionalParameters())
		if err == nil {
			defer r.Body.Close()
		}

		// Assertions
		require.NoError(t, err, "SubmitMetrics should not return error")
		assert.Equal(t, http.StatusAccepted, r.StatusCode, "Should return 202 Accepted")
	})

	t.Run("SubmitMultipleMetrics", func(t *testing.T) {
		series := []datadogV2.MetricSeries{}

		for i := 1; i <= 3; i++ {
			value := float64(i * 100)
			point := datadogV2.MetricPoint{
				Value: &value,
			}

			s := datadogV2.MetricSeries{
				Metric: "custom.batch.metric",
				Points: []datadogV2.MetricPoint{point},
				Tags:   []string{"batch:true"},
			}

			series = append(series, s)
		}

		body := datadogV2.MetricPayload{
			Series: series,
		}

		_, r, err := metricsAPI.SubmitMetrics(ctx, body, *datadogV2.NewSubmitMetricsOptionalParameters())
		if err == nil {
			defer r.Body.Close()
		}

		// Assertions
		require.NoError(t, err, "SubmitMetrics should not return error")
		assert.Equal(t, http.StatusAccepted, r.StatusCode, "Should return 202 Accepted")
	})
}
