package gsheets

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
)

// Google Sheets API response structures
type SpreadsheetProperties struct {
	Title string `json:"title"`
}

type SheetProperties struct {
	SheetID int64  `json:"sheetId"`
	Title   string `json:"title"`
	Index   int    `json:"index"`
}

type Sheet struct {
	Properties *SheetProperties `json:"properties"`
}

type Spreadsheet struct {
	SpreadsheetID string                 `json:"spreadsheetId"`
	Properties    *SpreadsheetProperties `json:"properties"`
	Sheets        []Sheet                `json:"sheets"`
}

type ValueRange struct {
	Range          string          `json:"range"`
	MajorDimension string          `json:"majorDimension"`
	Values         [][]interface{} `json:"values"`
}

type UpdateValuesResponse struct {
	SpreadsheetID  string `json:"spreadsheetId"`
	UpdatedRange   string `json:"updatedRange"`
	UpdatedRows    int    `json:"updatedRows"`
	UpdatedColumns int    `json:"updatedColumns"`
	UpdatedCells   int    `json:"updatedCells"`
}

type AppendValuesResponse struct {
	SpreadsheetID string      `json:"spreadsheetId"`
	TableRange    string      `json:"tableRange"`
	Updates       interface{} `json:"updates"`
}

type BatchUpdateRequest struct {
	Requests []Request `json:"requests"`
}

type Request struct {
	AddSheet    *AddSheetRequest    `json:"addSheet,omitempty"`
	DeleteSheet *DeleteSheetRequest `json:"deleteSheet,omitempty"`
}

type AddSheetRequest struct {
	Properties *SheetProperties `json:"properties"`
}

type DeleteSheetRequest struct {
	SheetID int64 `json:"sheetId"`
}

type BatchUpdateResponse struct {
	SpreadsheetID string        `json:"spreadsheetId"`
	Replies       []interface{} `json:"replies"`
}

// Handler implements the Google Sheets simulator HTTP handler
type Handler struct {
	queries *database.Queries
}

// NewHandler creates a new Google Sheets simulator handler
func NewHandler(queries *database.Queries) *Handler {
	return &Handler{
		queries: queries,
	}
}

// ServeHTTP implements http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[gsheets] → %s %s", r.Method, r.URL.Path)

	// Route Google Sheets API requests
	if strings.HasPrefix(r.URL.Path, "/v4/spreadsheets") {
		h.handleSheetsAPI(w, r)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) handleSheetsAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v4/spreadsheets")

	switch {
	case path == "" && r.Method == http.MethodPost:
		// Create spreadsheet
		h.handleCreateSpreadsheet(w, r)
	case regexp.MustCompile(`^/[^/]+$`).MatchString(path) && r.Method == http.MethodGet:
		// Get spreadsheet
		spreadsheetID := strings.TrimPrefix(path, "/")
		h.handleGetSpreadsheet(w, r, spreadsheetID)
	case regexp.MustCompile(`^/[^/]+/values/.+:append$`).MatchString(path):
		// Append rows (check this before update range since it has :append suffix)
		h.handleAppendRows(w, r, path)
	case regexp.MustCompile(`^/[^/]+/values/.+$`).MatchString(path) && r.Method == http.MethodGet:
		// Read range
		h.handleReadRange(w, r, path)
	case regexp.MustCompile(`^/[^/]+/values/.+$`).MatchString(path) && r.Method == http.MethodPut:
		// Update range
		h.handleUpdateRange(w, r, path)
	case regexp.MustCompile(`^/[^/]+:batchUpdate$`).MatchString(path):
		// Batch update
		spreadsheetID := strings.TrimSuffix(strings.TrimPrefix(path, "/"), ":batchUpdate")
		h.handleBatchUpdate(w, r, spreadsheetID)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleCreateSpreadsheet(w http.ResponseWriter, r *http.Request) {
	log.Println("[gsheets] → Received create spreadsheet request")

	var req Spreadsheet
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[gsheets] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Generate spreadsheet ID
	spreadsheetID := generateID()

	// Extract session ID from context
	sessionID := session.FromContext(r.Context())

	// Create spreadsheet in database
	title := "Untitled spreadsheet"
	if req.Properties != nil && req.Properties.Title != "" {
		title = req.Properties.Title
	}

	err := h.queries.CreateSpreadsheet(context.Background(), database.CreateSpreadsheetParams{
		ID:        spreadsheetID,
		Title:     title,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[gsheets] ✗ Failed to create spreadsheet: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Create default "Sheet1"
	sheetID := generateSheetID()
	err = h.queries.CreateSheet(context.Background(), database.CreateSheetParams{
		ID:            generateID(),
		SpreadsheetID: spreadsheetID,
		Title:         "Sheet1",
		SheetID:       sheetID,
		SessionID:     sessionID,
	})
	if err != nil {
		log.Printf("[gsheets] ✗ Failed to create default sheet: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := Spreadsheet{
		SpreadsheetID: spreadsheetID,
		Properties: &SpreadsheetProperties{
			Title: title,
		},
		Sheets: []Sheet{
			{
				Properties: &SheetProperties{
					SheetID: sheetID,
					Title:   "Sheet1",
					Index:   0,
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[gsheets] ✓ Spreadsheet created: %s", spreadsheetID)
}

func (h *Handler) handleGetSpreadsheet(w http.ResponseWriter, r *http.Request, spreadsheetID string) {
	log.Printf("[gsheets] → Received get spreadsheet request for ID: %s", spreadsheetID)

	sessionID := session.FromContext(r.Context())

	// Get spreadsheet from database
	dbSpreadsheet, err := h.queries.GetSpreadsheet(context.Background(), database.GetSpreadsheetParams{
		ID:        spreadsheetID,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[gsheets] ✗ Failed to get spreadsheet: %v", err)
		http.NotFound(w, r)
		return
	}

	// Get sheets for this spreadsheet
	dbSheets, err := h.queries.GetSheetsBySpreadsheet(context.Background(), database.GetSheetsBySpreadsheetParams{
		SpreadsheetID: spreadsheetID,
		SessionID:     sessionID,
	})
	if err != nil {
		log.Printf("[gsheets] ✗ Failed to get sheets: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	sheets := make([]Sheet, 0, len(dbSheets))
	for idx, dbSheet := range dbSheets {
		sheets = append(sheets, Sheet{
			Properties: &SheetProperties{
				SheetID: dbSheet.SheetID,
				Title:   dbSheet.Title,
				Index:   idx,
			},
		})
	}

	response := Spreadsheet{
		SpreadsheetID: dbSpreadsheet.ID,
		Properties: &SpreadsheetProperties{
			Title: dbSpreadsheet.Title,
		},
		Sheets: sheets,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[gsheets] ✓ Returned spreadsheet: %s", spreadsheetID)
}

func (h *Handler) handleReadRange(w http.ResponseWriter, r *http.Request, path string) {
	log.Printf("[gsheets] → Received read range request for path: %s", path)

	// Parse path: /spreadsheetId/values/range
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/values/")
	if len(parts) != 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	spreadsheetID := parts[0]
	rangeNotation := parts[1]

	sessionID := session.FromContext(r.Context())

	// Parse range notation (e.g., "Sheet1!A1:B2")
	parsedRange, err := parseRange(rangeNotation)
	if err != nil {
		log.Printf("[gsheets] ✗ Failed to parse range: %v", err)
		http.Error(w, "Invalid range notation", http.StatusBadRequest)
		return
	}

	// Get cells from database
	dbCells, err := h.queries.GetCellsInRange(context.Background(), database.GetCellsInRangeParams{
		SpreadsheetID: spreadsheetID,
		SheetTitle:    parsedRange.SheetTitle,
		Row:           int64(parsedRange.StartRow),
		Row_2:         int64(parsedRange.EndRow),
		Col:           int64(parsedRange.StartCol),
		Col_2:         int64(parsedRange.EndCol),
		SessionID:     sessionID,
	})
	if err != nil {
		log.Printf("[gsheets] ✗ Failed to get cells: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Convert to 2D array
	values := make([][]interface{}, 0)
	cellMap := make(map[int]map[int]string)

	for _, cell := range dbCells {
		rowIdx := int(cell.Row) - parsedRange.StartRow
		colIdx := int(cell.Col) - parsedRange.StartCol

		if cellMap[rowIdx] == nil {
			cellMap[rowIdx] = make(map[int]string)
		}
		if cell.Value.Valid {
			cellMap[rowIdx][colIdx] = cell.Value.String
		}
	}

	// Build 2D array with proper dimensions
	numRows := parsedRange.EndRow - parsedRange.StartRow + 1
	numCols := parsedRange.EndCol - parsedRange.StartCol + 1

	for r := 0; r < numRows; r++ {
		row := make([]interface{}, 0)
		for c := 0; c < numCols; c++ {
			if cellMap[r] != nil && cellMap[r][c] != "" {
				row = append(row, cellMap[r][c])
			} else {
				row = append(row, "")
			}
		}
		// Only add row if it has non-empty values
		hasValue := false
		for _, v := range row {
			if v != "" {
				hasValue = true
				break
			}
		}
		if hasValue {
			values = append(values, row)
		}
	}

	response := ValueRange{
		Range:          rangeNotation,
		MajorDimension: "ROWS",
		Values:         values,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[gsheets] ✓ Returned %d rows for range %s", len(values), rangeNotation)
}

func (h *Handler) handleUpdateRange(w http.ResponseWriter, r *http.Request, path string) {
	log.Printf("[gsheets] → Received update range request for path: %s", path)

	// Parse path
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/values/")
	if len(parts) != 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	spreadsheetID := parts[0]
	rangeNotation := parts[1]

	sessionID := session.FromContext(r.Context())

	// Parse request
	var req ValueRange
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[gsheets] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Parse range notation
	parsedRange, err := parseRange(rangeNotation)
	if err != nil {
		log.Printf("[gsheets] ✗ Failed to parse range: %v", err)
		http.Error(w, "Invalid range notation", http.StatusBadRequest)
		return
	}

	// Write values to database
	updatedCells := 0
	for rowIdx, row := range req.Values {
		for colIdx, val := range row {
			value := fmt.Sprintf("%v", val)
			err := h.queries.SetCellValue(context.Background(), database.SetCellValueParams{
				SpreadsheetID: spreadsheetID,
				SheetTitle:    parsedRange.SheetTitle,
				Row:           int64(parsedRange.StartRow + rowIdx),
				Col:           int64(parsedRange.StartCol + colIdx),
				Value:         sql.NullString{String: value, Valid: true},
				SessionID:     sessionID,
			})
			if err != nil {
				log.Printf("[gsheets] ✗ Failed to set cell value: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			updatedCells++
		}
	}

	response := UpdateValuesResponse{
		SpreadsheetID:  spreadsheetID,
		UpdatedRange:   rangeNotation,
		UpdatedRows:    len(req.Values),
		UpdatedColumns: 0,
		UpdatedCells:   updatedCells,
	}

	if len(req.Values) > 0 {
		response.UpdatedColumns = len(req.Values[0])
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[gsheets] ✓ Updated %d cells in range %s", updatedCells, rangeNotation)
}

func (h *Handler) handleAppendRows(w http.ResponseWriter, r *http.Request, path string) {
	log.Printf("[gsheets] → Received append rows request for path: %s", path)

	// Parse path: /spreadsheetId/values/range:append
	path = strings.TrimSuffix(path, ":append")
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/values/")
	if len(parts) != 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	spreadsheetID := parts[0]
	rangeNotation := parts[1]

	sessionID := session.FromContext(r.Context())

	// Parse request
	var req ValueRange
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[gsheets] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Parse range notation to get sheet title
	parsedRange, err := parseRange(rangeNotation)
	if err != nil {
		log.Printf("[gsheets] ✗ Failed to parse range: %v", err)
		http.Error(w, "Invalid range notation", http.StatusBadRequest)
		return
	}

	// Find the next available row
	maxRowResult, err := h.queries.GetMaxRowInSheet(context.Background(), database.GetMaxRowInSheetParams{
		SpreadsheetID: spreadsheetID,
		SheetTitle:    parsedRange.SheetTitle,
		SessionID:     sessionID,
	})
	if err != nil {
		log.Printf("[gsheets] ✗ Failed to get max row: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Convert maxRowResult to int64
	var maxRow int64
	switch v := maxRowResult.(type) {
	case int64:
		maxRow = v
	case int:
		maxRow = int64(v)
	default:
		maxRow = 0
	}

	startRow := int(maxRow) + 1

	// Append values
	updatedCells := 0
	for rowIdx, row := range req.Values {
		for colIdx, val := range row {
			value := fmt.Sprintf("%v", val)
			err := h.queries.SetCellValue(context.Background(), database.SetCellValueParams{
				SpreadsheetID: spreadsheetID,
				SheetTitle:    parsedRange.SheetTitle,
				Row:           int64(startRow + rowIdx),
				Col:           int64(parsedRange.StartCol + colIdx),
				Value:         sql.NullString{String: value, Valid: true},
				SessionID:     sessionID,
			})
			if err != nil {
				log.Printf("[gsheets] ✗ Failed to set cell value: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			updatedCells++
		}
	}

	// Calculate the actual range that was updated
	actualRange := fmt.Sprintf("%s!%s%d:%s%d",
		parsedRange.SheetTitle,
		columnToLetter(parsedRange.StartCol),
		startRow,
		columnToLetter(parsedRange.EndCol),
		startRow+len(req.Values)-1)

	response := AppendValuesResponse{
		SpreadsheetID: spreadsheetID,
		TableRange:    actualRange,
		Updates: UpdateValuesResponse{
			SpreadsheetID:  spreadsheetID,
			UpdatedRange:   actualRange,
			UpdatedRows:    len(req.Values),
			UpdatedColumns: len(req.Values[0]),
			UpdatedCells:   updatedCells,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[gsheets] ✓ Appended %d rows starting at row %d", len(req.Values), startRow)
}

func (h *Handler) handleBatchUpdate(w http.ResponseWriter, r *http.Request, spreadsheetID string) {
	log.Printf("[gsheets] → Received batch update request for spreadsheet: %s", spreadsheetID)

	sessionID := session.FromContext(r.Context())

	// Parse request
	var req BatchUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[gsheets] ✗ Failed to decode request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	replies := make([]interface{}, 0)

	// Process each request
	for _, request := range req.Requests {
		if request.AddSheet != nil {
			// Add sheet
			sheetID := generateSheetID()
			if request.AddSheet.Properties != nil && request.AddSheet.Properties.SheetID != 0 {
				sheetID = request.AddSheet.Properties.SheetID
			}

			title := "Sheet"
			if request.AddSheet.Properties != nil && request.AddSheet.Properties.Title != "" {
				title = request.AddSheet.Properties.Title
			}

			err := h.queries.CreateSheet(context.Background(), database.CreateSheetParams{
				ID:            generateID(),
				SpreadsheetID: spreadsheetID,
				Title:         title,
				SheetID:       sheetID,
				SessionID:     sessionID,
			})
			if err != nil {
				log.Printf("[gsheets] ✗ Failed to add sheet: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			replies = append(replies, map[string]interface{}{
				"addSheet": map[string]interface{}{
					"properties": map[string]interface{}{
						"sheetId": sheetID,
						"title":   title,
					},
				},
			})
		} else if request.DeleteSheet != nil {
			// Delete sheet
			err := h.queries.DeleteSheet(context.Background(), database.DeleteSheetParams{
				SpreadsheetID: spreadsheetID,
				SheetID:       request.DeleteSheet.SheetID,
				SessionID:     sessionID,
			})
			if err != nil {
				log.Printf("[gsheets] ✗ Failed to delete sheet: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			replies = append(replies, map[string]interface{}{})
		}
	}

	response := BatchUpdateResponse{
		SpreadsheetID: spreadsheetID,
		Replies:       replies,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[gsheets] ✓ Batch update completed with %d operations", len(req.Requests))
}

// Helper functions

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func generateSheetID() int64 {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	id := int64(b[0]) | int64(b[1])<<8 | int64(b[2])<<16 | int64(b[3])<<24
	if id < 0 {
		id = -id
	}
	return id
}

// ParsedRange holds the parsed range information
type ParsedRange struct {
	SheetTitle string
	StartRow   int
	StartCol   int
	EndRow     int
	EndCol     int
}

// parseRange parses a range notation like "Sheet1!A1:B2" or "A1:B2"
func parseRange(rangeNotation string) (ParsedRange, error) {
	var result ParsedRange
	var err error
	// Default sheet title
	result.SheetTitle = "Sheet1"

	// Check if range includes sheet title
	parts := strings.Split(rangeNotation, "!")
	var cellRange string
	if len(parts) == 2 {
		result.SheetTitle = parts[0]
		cellRange = parts[1]
	} else {
		cellRange = rangeNotation
	}

	// Parse cell range (e.g., "A1:B2" or "A1")
	cellParts := strings.Split(cellRange, ":")
	switch len(cellParts) {
	case 1:
		// Single cell (e.g., "A1")
		result.StartRow, result.StartCol, err = parseCell(cellParts[0])
		if err != nil {
			return result, err
		}
		result.EndRow = result.StartRow
		result.EndCol = result.StartCol
	case 2:
		// Range (e.g., "A1:B2")
		result.StartRow, result.StartCol, err = parseCell(cellParts[0])
		if err != nil {
			return result, err
		}
		result.EndRow, result.EndCol, err = parseCell(cellParts[1])
		if err != nil {
			return result, err
		}
	default:
		return result, fmt.Errorf("invalid range notation: %s", rangeNotation)
	}

	return result, nil
}

// parseCell parses a cell reference like "A1" into row and column indices (1-based)
func parseCell(cell string) (row, col int, err error) {
	// Extract column letters and row number
	re := regexp.MustCompile(`^([A-Z]+)(\d+)$`)
	matches := re.FindStringSubmatch(cell)
	if len(matches) != 3 {
		err = fmt.Errorf("invalid cell reference: %s", cell)
		return
	}

	// Convert column letters to number (1-based)
	col = letterToColumn(matches[1])

	// Parse row number
	row, err = strconv.Atoi(matches[2])
	if err != nil {
		err = fmt.Errorf("invalid row number in cell reference: %s", cell)
		return
	}

	return
}

// letterToColumn converts column letters (A, B, ..., Z, AA, AB, ...) to 1-based column index
func letterToColumn(letters string) int {
	col := 0
	for _, ch := range letters {
		col = col*26 + int(ch-'A'+1)
	}
	return col
}

// columnToLetter converts 1-based column index to column letters
func columnToLetter(col int) string {
	result := ""
	for col > 0 {
		col--
		result = string(rune('A'+col%26)) + result
		col /= 26
	}
	return result
}
