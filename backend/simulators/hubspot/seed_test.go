package hubspot_test

import (
	"context"
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

// TestHubSpotInitialStateSeed demonstrates seeding arbitrary initial state for HubSpot simulator
func TestHubSpotInitialStateSeed(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDBForSeed(t)
	ctx := context.Background()

	// Setup: Create a new session
	sessionID := "hubspot-seed-test-session"
	err := queries.CreateSession(ctx, sessionID)
	require.NoError(t, err, "Failed to create session")

	// Seed: Create custom contacts, companies, and deals
	contacts, companies, deals := seedHubSpotTestData(t, ctx, queries, sessionID)

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorHubspot.NewHandler(queries))
	mux := http.NewServeMux()
	mux.Handle("/hubspot/", http.StripPrefix("/hubspot", handler))
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create custom HTTP client that adds session header
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

	// Verify: Check that contacts are queryable
	t.Run("VerifyContacts", func(t *testing.T) {
		verifyContacts(t, client, contacts)
	})

	// Verify: Check that companies are queryable
	t.Run("VerifyCompanies", func(t *testing.T) {
		verifyCompanies(t, client, companies)
	})

	// Verify: Check that deals are queryable
	t.Run("VerifyDeals", func(t *testing.T) {
		verifyDeals(t, client, deals)
	})

	// Verify: Check database isolation - ensure all data is correctly stored
	t.Run("VerifyDatabaseIsolation", func(t *testing.T) {
		verifyDatabaseIsolation(t, ctx, queries, sessionID, contacts, companies, deals)
	})
}

// seedHubSpotTestData creates contacts, companies, and deals for testing
func seedHubSpotTestData(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string) (
	contacts []struct {
		ID        string
		Email     string
		FirstName string
		LastName  string
	},
	companies []struct {
		ID       string
		Name     string
		Domain   string
		City     string
		Industry string
	},
	deals []struct {
		ID        string
		DealName  string
		DealStage string
		Pipeline  string
		Amount    string
	},
) {
	t.Helper()

	// Seed: Create custom contacts (use session-specific IDs to avoid conflicts)
	contacts = []struct {
		ID        string
		Email     string
		FirstName string
		LastName  string
	}{
		{
			ID:        "CONTACT_001_" + sessionID,
			Email:     "alice.johnson@example.com",
			FirstName: "Alice",
			LastName:  "Johnson",
		},
		{
			ID:        "CONTACT_002_" + sessionID,
			Email:     "bob.smith@example.com",
			FirstName: "Bob",
			LastName:  "Smith",
		},
		{
			ID:        "CONTACT_003_" + sessionID,
			Email:     "carol.white@example.com",
			FirstName: "Carol",
			LastName:  "White",
		},
	}

	timestamp := int64(1640000000)
	for _, c := range contacts {
		_, err := queries.CreateHubspotContact(ctx, database.CreateHubspotContactParams{
			ID:        c.ID,
			Email:     sql.NullString{String: c.Email, Valid: true},
			FirstName: sql.NullString{String: c.FirstName, Valid: true},
			LastName:  sql.NullString{String: c.LastName, Valid: true},
			MobilePhone: sql.NullString{Valid: false},
			Website:     sql.NullString{Valid: false},
			SessionID:   sessionID,
			CreatedAt:   timestamp,
			UpdatedAt:   timestamp,
		})
		require.NoError(t, err, "Failed to create contact: %s", c.Email)
	}

	// Seed: Create custom companies (use session-specific IDs to avoid conflicts)
	companies = []struct {
		ID       string
		Name     string
		Domain   string
		City     string
		Industry string
	}{
		{
			ID:       "COMPANY_001_" + sessionID,
			Name:     "Acme Corp",
			Domain:   "acme.com",
			City:     "San Francisco",
			Industry: "Technology",
		},
		{
			ID:       "COMPANY_002_" + sessionID,
			Name:     "Smith Industries",
			Domain:   "smith.com",
			City:     "New York",
			Industry: "Manufacturing",
		},
	}

	for _, c := range companies {
		_, err := queries.CreateHubspotCompany(ctx, database.CreateHubspotCompanyParams{
			ID:        c.ID,
			Name:      sql.NullString{String: c.Name, Valid: true},
			Domain:    sql.NullString{String: c.Domain, Valid: true},
			City:      sql.NullString{String: c.City, Valid: true},
			Industry:  sql.NullString{String: c.Industry, Valid: true},
			SessionID: sessionID,
			CreatedAt: timestamp,
			UpdatedAt: timestamp,
		})
		require.NoError(t, err, "Failed to create company: %s", c.Name)
	}

	// Seed: Create custom deals (use session-specific IDs to avoid conflicts)
	deals = []struct {
		ID        string
		DealName  string
		DealStage string
		Pipeline  string
		Amount    string
	}{
		{
			ID:        "DEAL_001_" + sessionID,
			DealName:  "Enterprise Deal",
			DealStage: "appointmentscheduled",
			Pipeline:  "default",
			Amount:    "50000",
		},
		{
			ID:        "DEAL_002_" + sessionID,
			DealName:  "Small Business Deal",
			DealStage: "qualifiedtobuy",
			Pipeline:  "default",
			Amount:    "10000",
		},
	}

	for _, d := range deals {
		_, err := queries.CreateHubspotDeal(ctx, database.CreateHubspotDealParams{
			ID:        d.ID,
			DealName:  sql.NullString{String: d.DealName, Valid: true},
			DealStage: sql.NullString{String: d.DealStage, Valid: true},
			Pipeline:  sql.NullString{String: d.Pipeline, Valid: true},
			Amount:    sql.NullString{String: d.Amount, Valid: true},
			SessionID: sessionID,
			CreatedAt: timestamp,
			UpdatedAt: timestamp,
		})
		require.NoError(t, err, "Failed to create deal: %s", d.DealName)
	}

	return contacts, companies, deals
}

// verifyContacts verifies that contacts can be queried
func verifyContacts(t *testing.T, client *hubspot.Client, contacts []struct {
	ID        string
	Email     string
	FirstName string
	LastName  string
}) {
	t.Helper()

	// Get each contact by ID
	for _, c := range contacts {
		response, err := client.CRM.Contact.Get(c.ID, &hubspot.Contact{}, nil)
		require.NoError(t, err, "GetContact should succeed for contact: %s", c.Email)
		assert.Equal(t, c.ID, response.ID, "Contact ID should match")

		props, ok := response.Properties.(*hubspot.Contact)
		require.True(t, ok, "Should be Contact type")
		assert.Equal(t, c.Email, props.Email.String(), "Email should match")
		assert.Equal(t, c.FirstName, props.FirstName.String(), "First name should match")
		assert.Equal(t, c.LastName, props.LastName.String(), "Last name should match")
	}
}

// verifyCompanies verifies that companies can be queried
func verifyCompanies(t *testing.T, client *hubspot.Client, companies []struct {
	ID       string
	Name     string
	Domain   string
	City     string
	Industry string
}) {
	t.Helper()

	// Get each company by ID
	for _, c := range companies {
		response, err := client.CRM.Company.Get(c.ID, &hubspot.Company{}, nil)
		require.NoError(t, err, "GetCompany should succeed for company: %s", c.Name)
		assert.Equal(t, c.ID, response.ID, "Company ID should match")

		props, ok := response.Properties.(*hubspot.Company)
		require.True(t, ok, "Should be Company type")
		assert.Equal(t, c.Name, props.Name.String(), "Name should match")
		assert.Equal(t, c.Domain, props.Domain.String(), "Domain should match")
		assert.Equal(t, c.City, props.City.String(), "City should match")
		assert.Equal(t, c.Industry, props.Industry.String(), "Industry should match")
	}
}

// verifyDeals verifies that deals can be queried
func verifyDeals(t *testing.T, client *hubspot.Client, deals []struct {
	ID        string
	DealName  string
	DealStage string
	Pipeline  string
	Amount    string
}) {
	t.Helper()

	// Get each deal by ID
	for _, d := range deals {
		response, err := client.CRM.Deal.Get(d.ID, &hubspot.Deal{}, nil)
		require.NoError(t, err, "GetDeal should succeed for deal: %s", d.DealName)
		assert.Equal(t, d.ID, response.ID, "Deal ID should match")

		props, ok := response.Properties.(*hubspot.Deal)
		require.True(t, ok, "Should be Deal type")
		assert.Equal(t, d.DealName, props.DealName.String(), "Deal name should match")
		assert.Equal(t, d.DealStage, props.DealStage.String(), "Deal stage should match")
		assert.Equal(t, d.Amount, props.Amount.String(), "Amount should match")
	}
}

// verifyDatabaseIsolation verifies database isolation
func verifyDatabaseIsolation(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string,
	contacts []struct {
		ID        string
		Email     string
		FirstName string
		LastName  string
	},
	companies []struct {
		ID       string
		Name     string
		Domain   string
		City     string
		Industry string
	},
	deals []struct {
		ID        string
		DealName  string
		DealStage string
		Pipeline  string
		Amount    string
	}) {
	t.Helper()

	// Query contacts from database
	dbContacts, err := queries.ListHubspotContacts(ctx, sessionID)
	require.NoError(t, err, "ListHubspotContacts should succeed")
	assert.Len(t, dbContacts, len(contacts), "Should have correct number of contacts in database")

	// Verify contact details
	for _, c := range contacts {
		dbContact, err := queries.GetHubspotContactByID(ctx, database.GetHubspotContactByIDParams{
			ID:        c.ID,
			SessionID: sessionID,
		})
		require.NoError(t, err, "GetHubspotContactByID should succeed for contact: %s", c.Email)
		assert.Equal(t, c.Email, dbContact.Email.String, "Email should match in database")
		assert.Equal(t, c.FirstName, dbContact.FirstName.String, "First name should match in database")
		assert.Equal(t, c.LastName, dbContact.LastName.String, "Last name should match in database")
	}

	// Query companies from database
	dbCompanies, err := queries.ListHubspotCompanies(ctx, sessionID)
	require.NoError(t, err, "ListHubspotCompanies should succeed")
	assert.Len(t, dbCompanies, len(companies), "Should have correct number of companies in database")

	// Verify company details
	for _, c := range companies {
		dbCompany, err := queries.GetHubspotCompanyByID(ctx, database.GetHubspotCompanyByIDParams{
			ID:        c.ID,
			SessionID: sessionID,
		})
		require.NoError(t, err, "GetHubspotCompanyByID should succeed for company: %s", c.Name)
		assert.Equal(t, c.Name, dbCompany.Name.String, "Name should match in database")
		assert.Equal(t, c.Domain, dbCompany.Domain.String, "Domain should match in database")
		assert.Equal(t, c.City, dbCompany.City.String, "City should match in database")
		assert.Equal(t, c.Industry, dbCompany.Industry.String, "Industry should match in database")
	}

	// Query deals from database
	dbDeals, err := queries.ListHubspotDeals(ctx, sessionID)
	require.NoError(t, err, "ListHubspotDeals should succeed")
	assert.Len(t, dbDeals, len(deals), "Should have correct number of deals in database")

	// Verify deal details
	for _, d := range deals {
		dbDeal, err := queries.GetHubspotDealByID(ctx, database.GetHubspotDealByIDParams{
			ID:        d.ID,
			SessionID: sessionID,
		})
		require.NoError(t, err, "GetHubspotDealByID should succeed for deal: %s", d.DealName)
		assert.Equal(t, d.DealName, dbDeal.DealName.String, "Deal name should match in database")
		assert.Equal(t, d.DealStage, dbDeal.DealStage.String, "Deal stage should match in database")
		assert.Equal(t, d.Amount, dbDeal.Amount.String, "Amount should match in database")
	}
}
