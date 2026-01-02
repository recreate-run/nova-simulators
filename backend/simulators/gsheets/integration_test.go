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

func TestGsheetsSimulatorCreateSpreadsheet(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "gsheets-test-session-1"

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorGsheets.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client that adds session header
	transport := &sessionHTTPTransport{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create Sheets service pointing to test server with custom client
	ctx := context.Background()
	sheetsService, err := sheets.NewService(ctx,
		option.WithoutAuthentication(),
		option.WithEndpoint(server.URL+"/"),
		option.WithHTTPClient(customClient),
	)
	require.NoError(t, err, "Failed to create Sheets service")

	t.Run("CreateBasicSpreadsheet", func(t *testing.T) {
		// Create spreadsheet
		spreadsheet := &sheets.Spreadsheet{
			Properties: &sheets.SpreadsheetProperties{
				Title: "Test Spreadsheet",
			},
		}

		created, err := sheetsService.Spreadsheets.Create(spreadsheet).Do()

		// Assertions
		require.NoError(t, err, "Create should not return error")
		assert.NotNil(t, created, "Should return created spreadsheet")
		assert.NotEmpty(t, created.SpreadsheetId, "Spreadsheet ID should not be empty")
		assert.Equal(t, "Test Spreadsheet", created.Properties.Title, "Title should match")
		assert.Len(t, created.Sheets, 1, "Should have default Sheet1")
		assert.Equal(t, "Sheet1", created.Sheets[0].Properties.Title, "Default sheet should be Sheet1")
	})

	t.Run("CreateSpreadsheetWithoutTitle", func(t *testing.T) {
		// Create spreadsheet without title
		spreadsheet := &sheets.Spreadsheet{
			Properties: &sheets.SpreadsheetProperties{},
		}

		created, err := sheetsService.Spreadsheets.Create(spreadsheet).Do()

		// Assertions
		require.NoError(t, err, "Create should not return error")
		assert.NotEmpty(t, created.SpreadsheetId, "Should have ID")
		assert.Equal(t, "Untitled spreadsheet", created.Properties.Title, "Should have default title")
	})
}

func TestGsheetsSimulatorGetSpreadsheet(t *testing.T) {
	// Setup
	queries := setupTestDB(t)
	sessionID := "gsheets-test-session-2"
	handler := session.Middleware(simulatorGsheets.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	transport := &sessionHTTPTransport{sessionID: sessionID}
	customClient := &http.Client{Transport: transport}

	ctx := context.Background()
	sheetsService, err := sheets.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(customClient))
	require.NoError(t, err)

	// Create a spreadsheet first
	created, err := sheetsService.Spreadsheets.Create(&sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{Title: "My Spreadsheet"},
	}).Do()
	require.NoError(t, err)

	t.Run("GetExistingSpreadsheet", func(t *testing.T) {
		// Get the spreadsheet
		retrieved, err := sheetsService.Spreadsheets.Get(created.SpreadsheetId).Do()

		// Assertions
		require.NoError(t, err, "Get should not return error")
		assert.Equal(t, created.SpreadsheetId, retrieved.SpreadsheetId, "IDs should match")
		assert.Equal(t, "My Spreadsheet", retrieved.Properties.Title, "Title should match")
		assert.Len(t, retrieved.Sheets, 1, "Should have 1 sheet")
	})

	t.Run("GetNonExistentSpreadsheet", func(t *testing.T) {
		// Try to get a non-existent spreadsheet
		_, err := sheetsService.Spreadsheets.Get("nonexistent").Do()

		// Assertions
		assert.Error(t, err, "Should return error for non-existent spreadsheet")
	})
}

func TestGsheetsSimulatorUpdateRange(t *testing.T) {
	// Setup
	queries := setupTestDB(t)
	sessionID := "gsheets-test-session-3"
	handler := session.Middleware(simulatorGsheets.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	transport := &sessionHTTPTransport{sessionID: sessionID}
	customClient := &http.Client{Transport: transport}

	ctx := context.Background()
	sheetsService, err := sheets.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(customClient))
	require.NoError(t, err)

	// Create a spreadsheet
	created, err := sheetsService.Spreadsheets.Create(&sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{Title: "Update Test"},
	}).Do()
	require.NoError(t, err)

	t.Run("UpdateSingleCell", func(t *testing.T) {
		// Update a single cell
		valueRange := &sheets.ValueRange{
			Values: [][]interface{}{
				{"Hello World"},
			},
		}

		resp, err := sheetsService.Spreadsheets.Values.Update(
			created.SpreadsheetId,
			"Sheet1!A1",
			valueRange,
		).ValueInputOption("RAW").Do()

		// Assertions
		require.NoError(t, err, "Update should not return error")
		assert.Equal(t, 1, int(resp.UpdatedRows), "Should update 1 row")
		assert.Equal(t, 1, int(resp.UpdatedCells), "Should update 1 cell")
	})

	t.Run("UpdateMultipleCells", func(t *testing.T) {
		// Update a range of cells
		valueRange := &sheets.ValueRange{
			Values: [][]interface{}{
				{"Name", "Age", "City"},
				{"Alice", "30", "NYC"},
				{"Bob", "25", "LA"},
			},
		}

		resp, err := sheetsService.Spreadsheets.Values.Update(
			created.SpreadsheetId,
			"Sheet1!A1:C3",
			valueRange,
		).ValueInputOption("RAW").Do()

		// Assertions
		require.NoError(t, err, "Update should not return error")
		assert.Equal(t, 3, int(resp.UpdatedRows), "Should update 3 rows")
		assert.Equal(t, 9, int(resp.UpdatedCells), "Should update 9 cells")
	})
}

func TestGsheetsSimulatorReadRange(t *testing.T) {
	// Setup
	queries := setupTestDB(t)
	sessionID := "gsheets-test-session-4"
	handler := session.Middleware(simulatorGsheets.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	transport := &sessionHTTPTransport{sessionID: sessionID}
	customClient := &http.Client{Transport: transport}

	ctx := context.Background()
	sheetsService, err := sheets.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(customClient))
	require.NoError(t, err)

	// Create a spreadsheet and add data
	created, err := sheetsService.Spreadsheets.Create(&sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{Title: "Read Test"},
	}).Do()
	require.NoError(t, err)

	// Add test data
	testData := &sheets.ValueRange{
		Values: [][]interface{}{
			{"Header1", "Header2", "Header3"},
			{"Value1", "Value2", "Value3"},
			{"Value4", "Value5", "Value6"},
		},
	}
	_, err = sheetsService.Spreadsheets.Values.Update(created.SpreadsheetId, "Sheet1!A1:C3", testData).ValueInputOption("RAW").Do()
	require.NoError(t, err)

	t.Run("ReadFullRange", func(t *testing.T) {
		// Read the range
		resp, err := sheetsService.Spreadsheets.Values.Get(created.SpreadsheetId, "Sheet1!A1:C3").Do()

		// Assertions
		require.NoError(t, err, "Get should not return error")
		assert.Len(t, resp.Values, 3, "Should have 3 rows")
		assert.Equal(t, "Header1", resp.Values[0][0], "First cell should match")
		assert.Equal(t, "Value6", resp.Values[2][2], "Last cell should match")
	})

	t.Run("ReadPartialRange", func(t *testing.T) {
		// Read a subset
		resp, err := sheetsService.Spreadsheets.Values.Get(created.SpreadsheetId, "Sheet1!B2:C3").Do()

		// Assertions
		require.NoError(t, err, "Get should not return error")
		assert.Len(t, resp.Values, 2, "Should have 2 rows")
		assert.Equal(t, "Value2", resp.Values[0][0], "First cell should match")
	})

	t.Run("ReadEmptyRange", func(t *testing.T) {
		// Read an empty range
		resp, err := sheetsService.Spreadsheets.Values.Get(created.SpreadsheetId, "Sheet1!Z1:Z10").Do()

		// Assertions
		require.NoError(t, err, "Get should not return error")
		assert.Empty(t, resp.Values, "Should have no values")
	})
}

func TestGsheetsSimulatorAppendRows(t *testing.T) {
	// Setup
	queries := setupTestDB(t)
	sessionID := "gsheets-test-session-5"
	handler := session.Middleware(simulatorGsheets.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	transport := &sessionHTTPTransport{sessionID: sessionID}
	customClient := &http.Client{Transport: transport}

	ctx := context.Background()
	sheetsService, err := sheets.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(customClient))
	require.NoError(t, err)

	// Create a spreadsheet
	created, err := sheetsService.Spreadsheets.Create(&sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{Title: "Append Test"},
	}).Do()
	require.NoError(t, err)

	t.Run("AppendToEmptySheet", func(t *testing.T) {
		// Append first row
		valueRange := &sheets.ValueRange{
			Values: [][]interface{}{
				{"Name", "Score"},
			},
		}

		resp, err := sheetsService.Spreadsheets.Values.Append(
			created.SpreadsheetId,
			"Sheet1!A1",
			valueRange,
		).ValueInputOption("RAW").Do()

		// Assertions
		require.NoError(t, err, "Append should not return error")
		assert.NotNil(t, resp.Updates, "Should have updates")
	})

	t.Run("AppendMultipleRows", func(t *testing.T) {
		// Append more rows
		valueRange := &sheets.ValueRange{
			Values: [][]interface{}{
				{"Alice", "95"},
				{"Bob", "87"},
			},
		}

		resp, err := sheetsService.Spreadsheets.Values.Append(
			created.SpreadsheetId,
			"Sheet1!A1",
			valueRange,
		).ValueInputOption("RAW").Do()

		// Assertions
		require.NoError(t, err, "Append should not return error")
		assert.NotNil(t, resp.Updates, "Should have updates")

		// Verify all data is present
		allData, err := sheetsService.Spreadsheets.Values.Get(created.SpreadsheetId, "Sheet1!A1:B10").Do()
		require.NoError(t, err)
		assert.Len(t, allData.Values, 3, "Should have 3 rows total")
	})
}

func TestGsheetsSimulatorBatchUpdate(t *testing.T) {
	// Setup
	queries := setupTestDB(t)
	sessionID := "gsheets-test-session-6"
	handler := session.Middleware(simulatorGsheets.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	transport := &sessionHTTPTransport{sessionID: sessionID}
	customClient := &http.Client{Transport: transport}

	ctx := context.Background()
	sheetsService, err := sheets.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(customClient))
	require.NoError(t, err)

	// Create a spreadsheet
	created, err := sheetsService.Spreadsheets.Create(&sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{Title: "Batch Test"},
	}).Do()
	require.NoError(t, err)

	t.Run("AddSheet", func(t *testing.T) {
		// Add a new sheet
		batchUpdate := &sheets.BatchUpdateSpreadsheetRequest{
			Requests: []*sheets.Request{
				{
					AddSheet: &sheets.AddSheetRequest{
						Properties: &sheets.SheetProperties{
							Title: "NewSheet",
						},
					},
				},
			},
		}

		resp, err := sheetsService.Spreadsheets.BatchUpdate(created.SpreadsheetId, batchUpdate).Do()

		// Assertions
		require.NoError(t, err, "BatchUpdate should not return error")
		assert.Len(t, resp.Replies, 1, "Should have 1 reply")

		// Verify sheet was added
		spreadsheet, err := sheetsService.Spreadsheets.Get(created.SpreadsheetId).Do()
		require.NoError(t, err)
		assert.Len(t, spreadsheet.Sheets, 2, "Should have 2 sheets")
	})

	t.Run("DeleteSheet", func(t *testing.T) {
		// Get the current sheets
		spreadsheet, err := sheetsService.Spreadsheets.Get(created.SpreadsheetId).Do()
		require.NoError(t, err)

		// Find the sheet ID to delete (delete the second sheet)
		sheetIDToDelete := spreadsheet.Sheets[1].Properties.SheetId

		// Delete the sheet
		batchUpdate := &sheets.BatchUpdateSpreadsheetRequest{
			Requests: []*sheets.Request{
				{
					DeleteSheet: &sheets.DeleteSheetRequest{
						SheetId: sheetIDToDelete,
					},
				},
			},
		}

		resp, err := sheetsService.Spreadsheets.BatchUpdate(created.SpreadsheetId, batchUpdate).Do()

		// Assertions
		require.NoError(t, err, "BatchUpdate should not return error")
		assert.Len(t, resp.Replies, 1, "Should have 1 reply")

		// Verify sheet was deleted
		spreadsheet, err = sheetsService.Spreadsheets.Get(created.SpreadsheetId).Do()
		require.NoError(t, err)
		assert.Len(t, spreadsheet.Sheets, 1, "Should have 1 sheet")
	})
}

func TestGsheetsSimulatorEndToEnd(t *testing.T) {
	// Setup
	queries := setupTestDB(t)
	sessionID := "gsheets-test-session-7"
	handler := session.Middleware(simulatorGsheets.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	transport := &sessionHTTPTransport{sessionID: sessionID}
	customClient := &http.Client{Transport: transport}

	ctx := context.Background()
	sheetsService, err := sheets.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(customClient))
	require.NoError(t, err)

	t.Run("CompleteWorkflow", func(t *testing.T) {
		// 1. Create spreadsheet
		created, err := sheetsService.Spreadsheets.Create(&sheets.Spreadsheet{
			Properties: &sheets.SpreadsheetProperties{Title: "E2E Test"},
		}).Do()
		require.NoError(t, err)
		spreadsheetID := created.SpreadsheetId

		// 2. Add headers
		headers := &sheets.ValueRange{
			Values: [][]interface{}{
				{"Name", "Email", "Score"},
			},
		}
		_, err = sheetsService.Spreadsheets.Values.Update(spreadsheetID, "Sheet1!A1:C1", headers).ValueInputOption("RAW").Do()
		require.NoError(t, err)

		// 3. Append data rows
		data := &sheets.ValueRange{
			Values: [][]interface{}{
				{"Alice", "alice@example.com", "95"},
				{"Bob", "bob@example.com", "87"},
			},
		}
		_, err = sheetsService.Spreadsheets.Values.Append(spreadsheetID, "Sheet1!A1", data).ValueInputOption("RAW").Do()
		require.NoError(t, err)

		// 4. Read all data
		allData, err := sheetsService.Spreadsheets.Values.Get(spreadsheetID, "Sheet1!A1:C10").Do()
		require.NoError(t, err)
		assert.Len(t, allData.Values, 3, "Should have 3 rows (1 header + 2 data)")

		// 5. Add a new sheet
		batchUpdate := &sheets.BatchUpdateSpreadsheetRequest{
			Requests: []*sheets.Request{
				{
					AddSheet: &sheets.AddSheetRequest{
						Properties: &sheets.SheetProperties{
							Title: "Summary",
						},
					},
				},
			},
		}
		_, err = sheetsService.Spreadsheets.BatchUpdate(spreadsheetID, batchUpdate).Do()
		require.NoError(t, err)

		// 6. Verify final state
		spreadsheet, err := sheetsService.Spreadsheets.Get(spreadsheetID).Do()
		require.NoError(t, err)
		assert.Len(t, spreadsheet.Sheets, 2, "Should have 2 sheets")
		assert.Equal(t, "E2E Test", spreadsheet.Properties.Title)
	})
}

func TestGsheetsSimulatorSessionIsolation(t *testing.T) {
	// Setup
	queries := setupTestDB(t)
	session1ID := "gsheets-session-1"
	session2ID := "gsheets-session-2"

	handler := session.Middleware(simulatorGsheets.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create two clients with different sessions
	transport1 := &sessionHTTPTransport{sessionID: session1ID}
	client1 := &http.Client{Transport: transport1}

	transport2 := &sessionHTTPTransport{sessionID: session2ID}
	client2 := &http.Client{Transport: transport2}

	ctx := context.Background()
	sheetsService1, err := sheets.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(client1))
	require.NoError(t, err)

	sheetsService2, err := sheets.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(client2))
	require.NoError(t, err)

	t.Run("SessionsAreIsolated", func(t *testing.T) {
		// Create spreadsheet in session 1
		created1, err := sheetsService1.Spreadsheets.Create(&sheets.Spreadsheet{
			Properties: &sheets.SpreadsheetProperties{Title: "Session 1 Spreadsheet"},
		}).Do()
		require.NoError(t, err)

		// Create spreadsheet in session 2
		created2, err := sheetsService2.Spreadsheets.Create(&sheets.Spreadsheet{
			Properties: &sheets.SpreadsheetProperties{Title: "Session 2 Spreadsheet"},
		}).Do()
		require.NoError(t, err)

		// Session 1 should be able to access its spreadsheet
		s1, err := sheetsService1.Spreadsheets.Get(created1.SpreadsheetId).Do()
		require.NoError(t, err)
		assert.Equal(t, "Session 1 Spreadsheet", s1.Properties.Title)

		// Session 2 should be able to access its spreadsheet
		s2, err := sheetsService2.Spreadsheets.Get(created2.SpreadsheetId).Do()
		require.NoError(t, err)
		assert.Equal(t, "Session 2 Spreadsheet", s2.Properties.Title)

		// Session 1 should NOT be able to access session 2's spreadsheet
		_, err = sheetsService1.Spreadsheets.Get(created2.SpreadsheetId).Do()
		require.Error(t, err, "Session 1 should not access Session 2's spreadsheet")

		// Session 2 should NOT be able to access session 1's spreadsheet
		_, err = sheetsService2.Spreadsheets.Get(created1.SpreadsheetId).Do()
		require.Error(t, err, "Session 2 should not access Session 1's spreadsheet")
	})
}
