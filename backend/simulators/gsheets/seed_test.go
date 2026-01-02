package gsheets_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
	simulatorGsheets "github.com/recreate-run/nova-simulators/simulators/gsheets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	_ "modernc.org/sqlite"
)

// sessionHTTPTransportSeed wraps http.RoundTripper and adds session header to all requests
type sessionHTTPTransportSeed struct {
	sessionID string
}

func (t *sessionHTTPTransportSeed) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-Session-ID", t.sessionID)
	return http.DefaultTransport.RoundTrip(req)
}

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

// seedEmployeeDirectory creates and seeds an employee directory spreadsheet
func seedEmployeeDirectory(t *testing.T, sheetsService *sheets.Service) string {
	t.Helper()

	// Create spreadsheet for employee directory
	spreadsheet, err := sheetsService.Spreadsheets.Create(&sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{
			Title: "Employee Directory",
		},
	}).Do()
	require.NoError(t, err, "Failed to create Employee Directory spreadsheet")

	// Add headers
	headers := &sheets.ValueRange{
		Values: [][]interface{}{
			{"Name", "Email", "Department", "Start Date"},
		},
	}
	_, err = sheetsService.Spreadsheets.Values.Update(
		spreadsheet.SpreadsheetId,
		"Sheet1!A1:D1",
		headers,
	).ValueInputOption("RAW").Do()
	require.NoError(t, err, "Failed to add headers")

	// Add employee data
	employees := &sheets.ValueRange{
		Values: [][]interface{}{
			{"Alice Johnson", "alice@company.com", "Engineering", "2022-01-15"},
			{"Bob Smith", "bob@company.com", "Sales", "2021-06-01"},
			{"Charlie Davis", "charlie@company.com", "Marketing", "2023-03-10"},
			{"Diana Martinez", "diana@company.com", "Engineering", "2020-09-20"},
			{"Eva Chen", "eva@company.com", "HR", "2022-07-01"},
		},
	}
	_, err = sheetsService.Spreadsheets.Values.Append(
		spreadsheet.SpreadsheetId,
		"Sheet1!A1",
		employees,
	).ValueInputOption("RAW").Do()
	require.NoError(t, err, "Failed to add employee data")

	return spreadsheet.SpreadsheetId
}

// seedSalesData creates and seeds a sales tracking spreadsheet
func seedSalesData(t *testing.T, sheetsService *sheets.Service) string {
	t.Helper()

	// Create spreadsheet for sales tracking
	spreadsheet, err := sheetsService.Spreadsheets.Create(&sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{
			Title: "Q1 2024 Sales",
		},
	}).Do()
	require.NoError(t, err, "Failed to create Sales spreadsheet")

	// Add headers and sales data
	salesData := &sheets.ValueRange{
		Values: [][]interface{}{
			{"Date", "Product", "Units Sold", "Revenue"},
			{"2024-01-05", "Widget A", "120", "12000"},
			{"2024-01-12", "Widget B", "85", "17000"},
			{"2024-01-20", "Widget A", "150", "15000"},
			{"2024-02-03", "Widget C", "200", "40000"},
			{"2024-02-15", "Widget B", "95", "19000"},
			{"2024-03-01", "Widget A", "180", "18000"},
		},
	}
	_, err = sheetsService.Spreadsheets.Values.Update(
		spreadsheet.SpreadsheetId,
		"Sheet1!A1:D7",
		salesData,
	).ValueInputOption("RAW").Do()
	require.NoError(t, err, "Failed to add sales data")

	return spreadsheet.SpreadsheetId
}

// seedProjectTracker creates and seeds a project tracker spreadsheet with multiple sheets
func seedProjectTracker(t *testing.T, sheetsService *sheets.Service) string {
	t.Helper()

	// Create spreadsheet for project tracking
	spreadsheet, err := sheetsService.Spreadsheets.Create(&sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{
			Title: "Project Tracker",
		},
	}).Do()
	require.NoError(t, err, "Failed to create Project Tracker spreadsheet")

	// Add project data
	projectData := &sheets.ValueRange{
		Values: [][]interface{}{
			{"Task", "Assignee", "Status", "Priority", "Due Date"},
			{"Setup CI/CD pipeline", "Alice", "In Progress", "High", "2024-02-15"},
			{"Design new landing page", "Bob", "Completed", "Medium", "2024-02-01"},
			{"Implement user authentication", "Charlie", "Not Started", "High", "2024-02-20"},
			{"Write API documentation", "Diana", "In Progress", "Low", "2024-03-01"},
			{"Conduct user testing", "Eva", "Not Started", "Medium", "2024-03-15"},
		},
	}
	_, err = sheetsService.Spreadsheets.Values.Update(
		spreadsheet.SpreadsheetId,
		"Sheet1!A1:E6",
		projectData,
	).ValueInputOption("RAW").Do()
	require.NoError(t, err, "Failed to add project data")

	// Add a second sheet for completed tasks
	batchUpdate := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{
			{
				AddSheet: &sheets.AddSheetRequest{
					Properties: &sheets.SheetProperties{
						Title: "Completed Tasks",
					},
				},
			},
		},
	}
	_, err = sheetsService.Spreadsheets.BatchUpdate(spreadsheet.SpreadsheetId, batchUpdate).Do()
	require.NoError(t, err, "Failed to add Completed Tasks sheet")

	// Add data to the completed tasks sheet
	completedData := &sheets.ValueRange{
		Values: [][]interface{}{
			{"Task", "Assignee", "Completed Date"},
			{"Design new landing page", "Bob", "2024-01-28"},
			{"Fix login bug", "Alice", "2024-01-15"},
		},
	}
	_, err = sheetsService.Spreadsheets.Values.Update(
		spreadsheet.SpreadsheetId,
		"Completed Tasks!A1:C3",
		completedData,
	).ValueInputOption("RAW").Do()
	require.NoError(t, err, "Failed to add completed tasks data")

	return spreadsheet.SpreadsheetId
}

// TestGsheetsInitialStateSeed demonstrates seeding arbitrary initial state for Google Sheets simulator
func TestGsheetsInitialStateSeed(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDBForSeed(t)
	ctx := context.Background()

	// Setup: Create a new session
	sessionID := "gsheets-seed-test-session"
	err := queries.CreateSession(ctx, sessionID)
	require.NoError(t, err, "Failed to create session")

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorGsheets.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client that adds session header
	transport := &sessionHTTPTransportSeed{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create Sheets service pointing to test server with custom client
	sheetsService, err := sheets.NewService(ctx,
		option.WithoutAuthentication(),
		option.WithEndpoint(server.URL+"/"),
		option.WithHTTPClient(customClient),
	)
	require.NoError(t, err, "Failed to create Sheets service")

	// Seed spreadsheets
	var spreadsheetIDs []string
	t.Run("SeedEmployeeDirectory", func(t *testing.T) {
		spreadsheetIDs = append(spreadsheetIDs, seedEmployeeDirectory(t, sheetsService))
	})
	t.Run("SeedSalesData", func(t *testing.T) {
		spreadsheetIDs = append(spreadsheetIDs, seedSalesData(t, sheetsService))
	})
	t.Run("SeedProjectTracker", func(t *testing.T) {
		spreadsheetIDs = append(spreadsheetIDs, seedProjectTracker(t, sheetsService))
	})

	// Verify: Check that all spreadsheets are accessible
	t.Run("VerifyAllSpreadsheetsCreated", func(t *testing.T) {
		assert.Len(t, spreadsheetIDs, 3, "Should have created 3 spreadsheets")

		// Verify each spreadsheet can be retrieved
		expectedTitles := []string{"Employee Directory", "Q1 2024 Sales", "Project Tracker"}
		for i, id := range spreadsheetIDs {
			spreadsheet, err := sheetsService.Spreadsheets.Get(id).Do()
			require.NoError(t, err, "Failed to get spreadsheet %s", id)
			assert.Equal(t, expectedTitles[i], spreadsheet.Properties.Title, "Title should match")
		}
	})

	// Verify: Check Employee Directory data
	t.Run("VerifyEmployeeDirectory", func(t *testing.T) {
		spreadsheetID := spreadsheetIDs[0]

		// Read all employee data
		response, err := sheetsService.Spreadsheets.Values.Get(spreadsheetID, "Sheet1!A1:D10").Do()
		require.NoError(t, err, "Failed to read employee data")
		assert.Len(t, response.Values, 6, "Should have 6 rows (1 header + 5 employees)")

		// Verify headers
		headers := response.Values[0]
		assert.Equal(t, "Name", headers[0], "First header should be Name")
		assert.Equal(t, "Email", headers[1], "Second header should be Email")
		assert.Equal(t, "Department", headers[2], "Third header should be Department")
		assert.Equal(t, "Start Date", headers[3], "Fourth header should be Start Date")

		// Verify first employee
		firstEmployee := response.Values[1]
		assert.Equal(t, "Alice Johnson", firstEmployee[0], "First employee name should match")
		assert.Equal(t, "alice@company.com", firstEmployee[1], "First employee email should match")
		assert.Equal(t, "Engineering", firstEmployee[2], "First employee department should match")
	})

	// Verify: Check Sales data
	t.Run("VerifySalesData", func(t *testing.T) {
		spreadsheetID := spreadsheetIDs[1]

		// Read sales data
		response, err := sheetsService.Spreadsheets.Values.Get(spreadsheetID, "Sheet1!A1:D10").Do()
		require.NoError(t, err, "Failed to read sales data")
		assert.Len(t, response.Values, 7, "Should have 7 rows (1 header + 6 sales records)")

		// Verify we can read partial ranges
		partialResponse, err := sheetsService.Spreadsheets.Values.Get(spreadsheetID, "Sheet1!B2:C4").Do()
		require.NoError(t, err, "Failed to read partial range")
		assert.Len(t, partialResponse.Values, 3, "Should have 3 rows in partial range")
		assert.Equal(t, "Widget A", partialResponse.Values[0][0], "First product should be Widget A")
	})

	// Verify: Check Project Tracker with multiple sheets
	t.Run("VerifyProjectTrackerMultipleSheets", func(t *testing.T) {
		spreadsheetID := spreadsheetIDs[2]

		// Verify spreadsheet has 2 sheets
		spreadsheet, err := sheetsService.Spreadsheets.Get(spreadsheetID).Do()
		require.NoError(t, err, "Failed to get Project Tracker")
		assert.Len(t, spreadsheet.Sheets, 2, "Should have 2 sheets")

		sheetTitles := make([]string, len(spreadsheet.Sheets))
		for i, sheet := range spreadsheet.Sheets {
			sheetTitles[i] = sheet.Properties.Title
		}
		assert.Contains(t, sheetTitles, "Sheet1", "Should have Sheet1")
		assert.Contains(t, sheetTitles, "Completed Tasks", "Should have Completed Tasks sheet")

		// Read data from main sheet
		mainSheetData, err := sheetsService.Spreadsheets.Values.Get(spreadsheetID, "Sheet1!A1:E10").Do()
		require.NoError(t, err, "Failed to read main sheet")
		assert.Len(t, mainSheetData.Values, 6, "Should have 6 rows in main sheet")

		// Read data from completed tasks sheet
		completedData, err := sheetsService.Spreadsheets.Values.Get(spreadsheetID, "Completed Tasks!A1:C10").Do()
		require.NoError(t, err, "Failed to read completed tasks sheet")
		assert.Len(t, completedData.Values, 3, "Should have 3 rows in completed tasks sheet")
	})

	// Verify: Test data modification on seeded data
	t.Run("ModifySeededData", func(t *testing.T) {
		spreadsheetID := spreadsheetIDs[0] // Employee Directory

		// Add a new employee
		newEmployee := &sheets.ValueRange{
			Values: [][]interface{}{
				{"Frank Wilson", "frank@company.com", "Sales", "2024-02-01"},
			},
		}
		_, err := sheetsService.Spreadsheets.Values.Append(
			spreadsheetID,
			"Sheet1!A1",
			newEmployee,
		).ValueInputOption("RAW").Do()
		require.NoError(t, err, "Failed to add new employee")

		// Verify the new employee is present
		allData, err := sheetsService.Spreadsheets.Values.Get(spreadsheetID, "Sheet1!A1:D10").Do()
		require.NoError(t, err, "Failed to read updated data")
		assert.Len(t, allData.Values, 7, "Should have 7 rows now (1 header + 6 employees)")

		lastEmployee := allData.Values[6]
		assert.Equal(t, "Frank Wilson", lastEmployee[0], "New employee should be Frank Wilson")
	})

	// Verify: Check database isolation
	t.Run("VerifyDatabaseIsolation", func(t *testing.T) {
		// Query spreadsheets from database
		for _, id := range spreadsheetIDs {
			dbSpreadsheet, err := queries.GetSpreadsheet(ctx, database.GetSpreadsheetParams{
				ID:        id,
				SessionID: sessionID,
			})
			require.NoError(t, err, "GetSpreadsheet should succeed for %s", id)
			assert.Equal(t, id, dbSpreadsheet.ID, "Spreadsheet ID should match")

			// Get sheets for this spreadsheet
			dbSheets, err := queries.GetSheetsBySpreadsheet(ctx, database.GetSheetsBySpreadsheetParams{
				SpreadsheetID: id,
				SessionID:     sessionID,
			})
			require.NoError(t, err, "GetSheetsBySpreadsheet should succeed")
			assert.NotEmpty(t, dbSheets, "Should have at least one sheet")
		}
	})

	// Verify: Complex query scenarios
	t.Run("VerifyComplexQueries", func(t *testing.T) {
		salesSpreadsheetID := spreadsheetIDs[1]

		// Read specific columns (Product and Revenue)
		response, err := sheetsService.Spreadsheets.Values.Get(salesSpreadsheetID, "Sheet1!B2:D7").Do()
		require.NoError(t, err, "Failed to read product and revenue columns")
		assert.Len(t, response.Values, 6, "Should have 6 sales records")

		// Verify we can update specific cells
		update := &sheets.ValueRange{
			Values: [][]interface{}{
				{"Widget A", "200", "20000"},
			},
		}
		_, err = sheetsService.Spreadsheets.Values.Update(
			salesSpreadsheetID,
			"Sheet1!B2:D2",
			update,
		).ValueInputOption("RAW").Do()
		require.NoError(t, err, "Failed to update sales record")

		// Verify the update
		updatedRow, err := sheetsService.Spreadsheets.Values.Get(salesSpreadsheetID, "Sheet1!B2:D2").Do()
		require.NoError(t, err, "Failed to read updated row")
		assert.Equal(t, "200", updatedRow.Values[0][1], "Units should be updated to 200")
		assert.Equal(t, "20000", updatedRow.Values[0][2], "Revenue should be updated to 20000")
	})
}
