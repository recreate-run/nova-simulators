package hubspot_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/belong-inc/go-hubspot"
	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
	simulatorHubspot "github.com/recreate-run/nova-simulators/simulators/hubspot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// sessionHTTPTransport wraps http.RoundTripper and adds session header to all requests
type sessionHTTPTransport struct {
	sessionID  string
	testServer *httptest.Server
}

func (t *sessionHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite HubSpot API requests to point to our test server
	if req.URL.Host == "api.hubapi.com" {
		req.URL.Scheme = "http"
		req.URL.Host = t.testServer.Listener.Addr().String()
		// Add /hubspot prefix - main.go will strip it via http.StripPrefix
		// Our handler then sees /crm/v3/... paths
		req.URL.Path = "/hubspot" + req.URL.Path
	}
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

func TestHubSpotSimulatorContact(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "hubspot-test-session-1"

	// Setup: Start simulator server with session middleware (mimicking main.go setup)
	handler := session.Middleware(simulatorHubspot.NewHandler(queries))
	// Wrap with http.StripPrefix to remove /hubspot prefix, just like in main.go
	mux := http.NewServeMux()
	mux.Handle("/hubspot/", http.StripPrefix("/hubspot", handler))
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create custom HTTP client that adds session header and rewrites URLs
	transport := &sessionHTTPTransport{
		sessionID:  sessionID,
		testServer: server,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create HubSpot client with custom HTTP client
	client, err := hubspot.NewClient(
		hubspot.SetPrivateAppToken("test-token"),
		hubspot.WithHTTPClient(customClient),
	)
	require.NoError(t, err, "Failed to create HubSpot client")

	var contactID string

	t.Run("CreateContact", func(t *testing.T) {
		// Create contact
		contact := &hubspot.Contact{
			Email:     hubspot.NewString("john.doe@example.com"),
			FirstName: hubspot.NewString("John"),
			LastName:  hubspot.NewString("Doe"),
		}

		response, err := client.CRM.Contact.Create(contact)

		// Assertions
		require.NoError(t, err, "Create should not return error")
		assert.NotNil(t, response, "Should return response")
		assert.NotEmpty(t, response.ID, "Contact ID should not be empty")

		contactID = response.ID
	})

	t.Run("GetContact", func(t *testing.T) {
		// Get contact
		response, err := client.CRM.Contact.Get(contactID, &hubspot.Contact{}, nil)

		// Assertions
		require.NoError(t, err, "Get should not return error")
		assert.NotNil(t, response, "Should return response")
		assert.Equal(t, contactID, response.ID, "Contact ID should match")

		props, ok := response.Properties.(*hubspot.Contact)
		require.True(t, ok, "Should be Contact type")
		assert.Equal(t, "john.doe@example.com", props.Email.String(), "Email should match")
		assert.Equal(t, "John", props.FirstName.String(), "First name should match")
		assert.Equal(t, "Doe", props.LastName.String(), "Last name should match")
	})

	t.Run("UpdateContact", func(t *testing.T) {
		// Update contact
		contact := &hubspot.Contact{
			Email:       hubspot.NewString("john.updated@example.com"),
			MobilePhone: hubspot.NewString("+1234567890"),
		}

		response, err := client.CRM.Contact.Update(contactID, contact)

		// Assertions
		require.NoError(t, err, "Update should not return error")
		assert.NotNil(t, response, "Should return response")
		assert.Equal(t, contactID, response.ID, "Contact ID should match")

		// Verify update
		getResp, err := client.CRM.Contact.Get(contactID, &hubspot.Contact{}, nil)
		require.NoError(t, err, "Get should succeed")

		props, ok := getResp.Properties.(*hubspot.Contact)
		require.True(t, ok, "Should be Contact type")
		assert.Equal(t, "john.updated@example.com", props.Email.String(), "Email should be updated")
		assert.Equal(t, "+1234567890", props.MobilePhone.String(), "Mobile phone should be updated")
	})

	t.Run("SearchContacts", func(t *testing.T) {
		// Search for contact by email
		filter := hubspot.Filter{
			PropertyName: "email",
			Operator:     "EQ",
			Value:        hubspot.NewString("john.updated@example.com"),
		}

		searchReq := &hubspot.ContactSearchRequest{
			SearchOptions: hubspot.SearchOptions{
				FilterGroups: []hubspot.FilterGroup{
					{Filters: []hubspot.Filter{filter}},
				},
				Limit: 100,
			},
		}

		response, err := client.CRM.Contact.Search(searchReq)

		// Assertions
		require.NoError(t, err, "Search should not return error")
		assert.NotNil(t, response, "Should return response")
		assert.GreaterOrEqual(t, len(response.Results), 1, "Should find at least one contact")
		assert.Equal(t, "john.updated@example.com", response.Results[0].Properties.Email.String(), "Email should match")
	})
}

func TestHubSpotSimulatorDeal(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "hubspot-test-session-2"

	// Setup: Start simulator server with session middleware (mimicking main.go setup)
	handler := session.Middleware(simulatorHubspot.NewHandler(queries))
	mux := http.NewServeMux()
	mux.Handle("/hubspot/", http.StripPrefix("/hubspot", handler))
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create custom HTTP client
	transport := &sessionHTTPTransport{
		sessionID:  sessionID,
		testServer: server,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create HubSpot client
	client, err := hubspot.NewClient(
		hubspot.SetPrivateAppToken("test-token"),
		hubspot.WithHTTPClient(customClient),
	)
	require.NoError(t, err, "Failed to create HubSpot client")

	var dealID string

	t.Run("CreateDeal", func(t *testing.T) {
		// Create deal
		deal := &hubspot.Deal{
			DealName:  hubspot.NewString("New Business Deal"),
			DealStage: hubspot.NewString("appointmentscheduled"),
			PipeLine:  hubspot.NewString("default"),
			Amount:    hubspot.NewString("10000"),
		}

		response, err := client.CRM.Deal.Create(deal)

		// Assertions
		require.NoError(t, err, "Create should not return error")
		assert.NotNil(t, response, "Should return response")
		assert.NotEmpty(t, response.ID, "Deal ID should not be empty")

		dealID = response.ID
	})

	t.Run("GetDeal", func(t *testing.T) {
		// Get deal
		response, err := client.CRM.Deal.Get(dealID, &hubspot.Deal{}, nil)

		// Assertions
		require.NoError(t, err, "Get should not return error")
		assert.NotNil(t, response, "Should return response")
		assert.Equal(t, dealID, response.ID, "Deal ID should match")

		props, ok := response.Properties.(*hubspot.Deal)
		require.True(t, ok, "Should be Deal type")
		assert.Equal(t, "New Business Deal", props.DealName.String(), "Deal name should match")
		assert.Equal(t, "appointmentscheduled", props.DealStage.String(), "Deal stage should match")
		assert.Equal(t, "10000", props.Amount.String(), "Amount should match")
	})

	t.Run("UpdateDeal", func(t *testing.T) {
		// Update deal
		deal := &hubspot.Deal{
			DealName:  hubspot.NewString("Updated Deal Name"),
			DealStage: hubspot.NewString("qualifiedtobuy"),
			Amount:    hubspot.NewString("15000"),
		}

		response, err := client.CRM.Deal.Update(dealID, deal)

		// Assertions
		require.NoError(t, err, "Update should not return error")
		assert.NotNil(t, response, "Should return response")

		// Verify update
		getResp, err := client.CRM.Deal.Get(dealID, &hubspot.Deal{}, nil)
		require.NoError(t, err, "Get should succeed")

		props, ok := getResp.Properties.(*hubspot.Deal)
		require.True(t, ok, "Should be Deal type")
		assert.Equal(t, "Updated Deal Name", props.DealName.String(), "Deal name should be updated")
		assert.Equal(t, "qualifiedtobuy", props.DealStage.String(), "Deal stage should be updated")
		assert.Equal(t, "15000", props.Amount.String(), "Amount should be updated")
	})
}

func TestHubSpotSimulatorCompany(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "hubspot-test-session-3"

	// Setup: Start simulator server with session middleware (mimicking main.go setup)
	handler := session.Middleware(simulatorHubspot.NewHandler(queries))
	mux := http.NewServeMux()
	mux.Handle("/hubspot/", http.StripPrefix("/hubspot", handler))
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create custom HTTP client
	transport := &sessionHTTPTransport{
		sessionID:  sessionID,
		testServer: server,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create HubSpot client
	client, err := hubspot.NewClient(
		hubspot.SetPrivateAppToken("test-token"),
		hubspot.WithHTTPClient(customClient),
	)
	require.NoError(t, err, "Failed to create HubSpot client")

	var companyID string

	t.Run("CreateCompany", func(t *testing.T) {
		// Create company
		company := &hubspot.Company{
			Name:     hubspot.NewString("Acme Corp"),
			Domain:   hubspot.NewString("acme.com"),
			City:     hubspot.NewString("San Francisco"),
			Industry: hubspot.NewString("Technology"),
		}

		response, err := client.CRM.Company.Create(company)

		// Assertions
		require.NoError(t, err, "Create should not return error")
		assert.NotNil(t, response, "Should return response")
		assert.NotEmpty(t, response.ID, "Company ID should not be empty")

		companyID = response.ID
	})

	t.Run("GetCompany", func(t *testing.T) {
		// Get company
		response, err := client.CRM.Company.Get(companyID, &hubspot.Company{}, nil)

		// Assertions
		require.NoError(t, err, "Get should not return error")
		assert.NotNil(t, response, "Should return response")
		assert.Equal(t, companyID, response.ID, "Company ID should match")

		props, ok := response.Properties.(*hubspot.Company)
		require.True(t, ok, "Should be Company type")
		assert.Equal(t, "Acme Corp", props.Name.String(), "Name should match")
		assert.Equal(t, "acme.com", props.Domain.String(), "Domain should match")
		assert.Equal(t, "San Francisco", props.City.String(), "City should match")
		assert.Equal(t, "Technology", props.Industry.String(), "Industry should match")
	})

	t.Run("UpdateCompany", func(t *testing.T) {
		// Update company
		company := &hubspot.Company{
			Name:     hubspot.NewString("Acme Corporation"),
			City:     hubspot.NewString("New York"),
			Industry: hubspot.NewString("Software"),
		}

		response, err := client.CRM.Company.Update(companyID, company)

		// Assertions
		require.NoError(t, err, "Update should not return error")
		assert.NotNil(t, response, "Should return response")

		// Verify update
		getResp, err := client.CRM.Company.Get(companyID, &hubspot.Company{}, nil)
		require.NoError(t, err, "Get should succeed")

		props, ok := getResp.Properties.(*hubspot.Company)
		require.True(t, ok, "Should be Company type")
		assert.Equal(t, "Acme Corporation", props.Name.String(), "Name should be updated")
		assert.Equal(t, "New York", props.City.String(), "City should be updated")
		assert.Equal(t, "Software", props.Industry.String(), "Industry should be updated")
	})
}

func TestHubSpotSimulatorAssociations(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "hubspot-test-session-4"

	// Setup: Start simulator server with session middleware (mimicking main.go setup)
	handler := session.Middleware(simulatorHubspot.NewHandler(queries))
	mux := http.NewServeMux()
	mux.Handle("/hubspot/", http.StripPrefix("/hubspot", handler))
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create custom HTTP client
	transport := &sessionHTTPTransport{
		sessionID:  sessionID,
		testServer: server,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create HubSpot client
	client, err := hubspot.NewClient(
		hubspot.SetPrivateAppToken("test-token"),
		hubspot.WithHTTPClient(customClient),
	)
	require.NoError(t, err, "Failed to create HubSpot client")

	// Create test contact
	contact := &hubspot.Contact{
		Email:     hubspot.NewString("jane.smith@example.com"),
		FirstName: hubspot.NewString("Jane"),
		LastName:  hubspot.NewString("Smith"),
	}
	contactResp, err := client.CRM.Contact.Create(contact)
	require.NoError(t, err, "Failed to create contact")

	// Create test deal
	deal := &hubspot.Deal{
		DealName:  hubspot.NewString("Jane's Deal"),
		DealStage: hubspot.NewString("closedwon"),
		Amount:    hubspot.NewString("25000"),
	}
	dealResp, err := client.CRM.Deal.Create(deal)
	require.NoError(t, err, "Failed to create deal")

	// Create test company
	company := &hubspot.Company{
		Name:   hubspot.NewString("Smith Industries"),
		Domain: hubspot.NewString("smith.com"),
	}
	companyResp, err := client.CRM.Company.Create(company)
	require.NoError(t, err, "Failed to create company")

	t.Run("AssociateDealToContact", func(t *testing.T) {
		// Associate deal to contact
		assoc := &hubspot.AssociationConfig{
			ToObject:   hubspot.ObjectTypeContact,
			ToObjectID: contactResp.ID,
			Type:       hubspot.AssociationTypeDealToContact,
		}

		response, err := client.CRM.Deal.AssociateAnotherObj(dealResp.ID, assoc)

		// Assertions
		require.NoError(t, err, "Association should not return error")
		assert.NotNil(t, response, "Should return response")
	})

	t.Run("AssociateContactToCompany", func(t *testing.T) {
		// Associate contact to company
		assoc := &hubspot.AssociationConfig{
			ToObject:   hubspot.ObjectTypeCompany,
			ToObjectID: companyResp.ID,
			Type:       hubspot.AssociationTypeContactToCompany,
		}

		response, err := client.CRM.Contact.AssociateAnotherObj(contactResp.ID, assoc)

		// Assertions
		require.NoError(t, err, "Association should not return error")
		assert.NotNil(t, response, "Should return response")
	})
}

func TestHubSpotSimulatorEndToEnd(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "hubspot-test-session-5"

	// Setup: Start simulator server with session middleware (mimicking main.go setup)
	handler := session.Middleware(simulatorHubspot.NewHandler(queries))
	mux := http.NewServeMux()
	mux.Handle("/hubspot/", http.StripPrefix("/hubspot", handler))
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create custom HTTP client
	transport := &sessionHTTPTransport{
		sessionID:  sessionID,
		testServer: server,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create HubSpot client
	client, err := hubspot.NewClient(
		hubspot.SetPrivateAppToken("test-token"),
		hubspot.WithHTTPClient(customClient),
	)
	require.NoError(t, err, "Failed to create HubSpot client")

	// End-to-end workflow: Create Contact -> Create Deal -> Associate -> Verify
	t.Run("CompleteWorkflow", func(t *testing.T) {
		// 1. Create a contact
		contact := &hubspot.Contact{
			Email:     hubspot.NewString("alice@example.com"),
			FirstName: hubspot.NewString("Alice"),
			LastName:  hubspot.NewString("Wonder"),
		}
		contactResp, err := client.CRM.Contact.Create(contact)
		require.NoError(t, err, "Contact creation should succeed")

		// 2. Create a deal
		deal := &hubspot.Deal{
			DealName:  hubspot.NewString("Alice's Big Deal"),
			DealStage: hubspot.NewString("presentationscheduled"),
			Amount:    hubspot.NewString("50000"),
		}
		dealResp, err := client.CRM.Deal.Create(deal)
		require.NoError(t, err, "Deal creation should succeed")

		// 3. Associate deal to contact
		assoc := &hubspot.AssociationConfig{
			ToObject:   hubspot.ObjectTypeContact,
			ToObjectID: contactResp.ID,
			Type:       hubspot.AssociationTypeDealToContact,
		}
		_, err = client.CRM.Deal.AssociateAnotherObj(dealResp.ID, assoc)
		require.NoError(t, err, "Association should succeed")

		// 4. Verify contact exists and has correct data
		getContactResp, err := client.CRM.Contact.Get(contactResp.ID, &hubspot.Contact{}, nil)
		require.NoError(t, err, "Get contact should succeed")
		contactProps, ok := getContactResp.Properties.(*hubspot.Contact)
		require.True(t, ok, "Should be Contact type")
		assert.Equal(t, "alice@example.com", contactProps.Email.String(), "Contact email should match")

		// 5. Verify deal exists and has correct data
		getDealResp, err := client.CRM.Deal.Get(dealResp.ID, &hubspot.Deal{}, nil)
		require.NoError(t, err, "Get deal should succeed")
		dealProps, ok := getDealResp.Properties.(*hubspot.Deal)
		require.True(t, ok, "Should be Deal type")
		assert.Equal(t, "Alice's Big Deal", dealProps.DealName.String(), "Deal name should match")
		assert.Equal(t, "50000", dealProps.Amount.String(), "Deal amount should match")
	})
}
