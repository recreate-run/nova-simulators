package jira_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andygrunwald/go-jira"
	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
	simulatorJira "github.com/recreate-run/nova-simulators/simulators/jira"
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

// TestJiraInitialStateSeed demonstrates seeding arbitrary initial state for Jira simulator
func TestJiraInitialStateSeed(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDBForSeed(t)
	ctx := context.Background()

	// Setup: Create a new session
	sessionID := "jira-seed-test-session"
	err := queries.CreateSession(ctx, sessionID)
	require.NoError(t, err, "Failed to create session")

	// Seed: Create custom projects, issues, and users
	projects, issues, users := seedJiraTestData(t, ctx, queries, sessionID)

	// Setup: Start simulator server with session middleware
	mux := http.NewServeMux()
	jiraHandler := session.Middleware(simulatorJira.NewHandler(queries))
	mux.Handle("/jira/", http.StripPrefix("/jira", jiraHandler))
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create Jira client with session header
	transport := jira.BasicAuthTransport{
		Username: "test@example.com",
		Password: "test-token",
		Transport: &sessionHTTPTransport{
			sessionID: sessionID,
		},
	}
	client, err := jira.NewClient(transport.Client(), server.URL+"/jira")
	require.NoError(t, err, "Failed to create Jira client")

	// Verify: Check that projects are queryable
	t.Run("VerifyProjects", func(t *testing.T) {
		verifyProjects(t, client, projects)
	})

	// Verify: Check that issues are queryable
	t.Run("VerifyIssues", func(t *testing.T) {
		verifyIssues(t, client, issues)
	})

	// Verify: Check that users can be seen in assignee fields
	t.Run("VerifyUsers", func(t *testing.T) {
		verifyUsers(t, client, issues, users)
	})

	// Verify: Check database isolation - ensure all data is correctly stored
	t.Run("VerifyDatabaseIsolation", func(t *testing.T) {
		verifyDatabaseIsolation(t, ctx, queries, sessionID, projects, issues, users)
	})
}

// seedJiraTestData creates projects, issues, and users for testing
func seedJiraTestData(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string) (
	projects []struct{ ID, Key, Name string },
	issues []struct {
		ID          string
		Key         string
		ProjectKey  string
		IssueType   string
		Summary     string
		Description string
		Assignee    string
		Status      string
	},
	users []struct{ Name, Email string },
) {
	t.Helper()

	// Seed: Create custom projects (use session-specific IDs to avoid conflicts)
	projects = []struct {
		ID   string
		Key  string
		Name string
	}{
		{"PROJ001_" + sessionID, "PROJ", "Project Management"},
		{"PROJ002_" + sessionID, "DEV", "Development"},
		{"PROJ003_" + sessionID, "OPS", "Operations"},
	}

	for _, proj := range projects {
		err := queries.CreateJiraProject(ctx, database.CreateJiraProjectParams{
			ID:        proj.ID,
			Key:       proj.Key,
			Name:      proj.Name,
			SessionID: sessionID,
		})
		require.NoError(t, err, "Failed to create project: %s", proj.Name)
	}

	// Seed: Define users
	users = []struct {
		Name  string
		Email string
	}{
		{"john.doe", "john@example.com"},
		{"jane.smith", "jane@example.com"},
		{"bob.wilson", "bob@example.com"},
	}

	// Seed: Create custom issues (use session-specific IDs to avoid conflicts)
	issues = []struct {
		ID          string
		Key         string
		ProjectKey  string
		IssueType   string
		Summary     string
		Description string
		Assignee    string
		Status      string
	}{
		{
			ID:          "ISSUE001_" + sessionID,
			Key:         "PROJ-1",
			ProjectKey:  "PROJ",
			IssueType:   "Task",
			Summary:     "Setup project infrastructure",
			Description: "Initialize the project repository and CI/CD pipeline",
			Assignee:    "john.doe",
			Status:      "To Do",
		},
		{
			ID:          "ISSUE002_" + sessionID,
			Key:         "PROJ-2",
			ProjectKey:  "PROJ",
			IssueType:   "Bug",
			Summary:     "Fix login authentication issue",
			Description: "Users are unable to login with correct credentials",
			Assignee:    "jane.smith",
			Status:      "In Progress",
		},
		{
			ID:          "ISSUE003_" + sessionID,
			Key:         "DEV-1",
			ProjectKey:  "DEV",
			IssueType:   "Story",
			Summary:     "Implement user dashboard",
			Description: "As a user, I want to see my personalized dashboard",
			Assignee:    "bob.wilson",
			Status:      "Done",
		},
	}

	for _, issue := range issues {
		err := queries.CreateJiraIssue(ctx, database.CreateJiraIssueParams{
			ID:          issue.ID,
			Key:         issue.Key,
			ProjectKey:  issue.ProjectKey,
			IssueType:   issue.IssueType,
			Summary:     issue.Summary,
			Description: sql.NullString{String: issue.Description, Valid: true},
			Assignee:    sql.NullString{String: issue.Assignee, Valid: true},
			Status:      issue.Status,
			SessionID:   sessionID,
		})
		require.NoError(t, err, "Failed to create issue: %s", issue.Key)
	}

	return projects, issues, users
}

// verifyProjects verifies that projects can be queried
func verifyProjects(t *testing.T, client *jira.Client, projects []struct{ ID, Key, Name string }) {
	t.Helper()

	// List projects
	projectList, _, err := client.Project.GetList()
	require.NoError(t, err, "GetList should succeed")
	require.NotNil(t, projectList, "Project list should not be nil")
	assert.Len(t, *projectList, 3, "Should have 3 projects")

	// Verify project keys
	projectMap := make(map[string]string)
	for i := range *projectList {
		projectMap[(*projectList)[i].Key] = (*projectList)[i].Name
	}

	for _, proj := range projects {
		assert.Contains(t, projectMap, proj.Key, "Should have project: "+proj.Key)
		assert.Equal(t, proj.Name, projectMap[proj.Key], "Project name should match")
	}
}

// verifyIssues verifies that issues can be queried
func verifyIssues(t *testing.T, client *jira.Client, issues []struct {
	ID          string
	Key         string
	ProjectKey  string
	IssueType   string
	Summary     string
	Description string
	Assignee    string
	Status      string
}) {
	t.Helper()

	// Get each issue and verify content
	for _, issue := range issues {
		retrieved, _, err := client.Issue.Get(issue.Key, nil)
		require.NoError(t, err, "Get should succeed for issue: %s", issue.Key)
		assert.Equal(t, issue.Key, retrieved.Key, "Issue key should match")
		assert.Equal(t, issue.Summary, retrieved.Fields.Summary, "Summary should match")
		assert.Equal(t, issue.Description, retrieved.Fields.Description, "Description should match")
		assert.Equal(t, issue.Status, retrieved.Fields.Status.Name, "Status should match")

		if issue.Assignee != "" {
			require.NotNil(t, retrieved.Fields.Assignee, "Assignee should not be nil")
			assert.Equal(t, issue.Assignee, retrieved.Fields.Assignee.Name, "Assignee should match")
		}
	}

	// Search issues by project
	searchResults, _, err := client.Issue.Search("project = PROJ", &jira.SearchOptions{
		MaxResults: 50,
	})
	require.NoError(t, err, "Search should succeed")
	assert.Len(t, searchResults, 2, "Should have 2 issues in PROJ project")
}

// verifyUsers verifies that users can be seen in assignee fields
func verifyUsers(t *testing.T, client *jira.Client, issues []struct {
	ID          string
	Key         string
	ProjectKey  string
	IssueType   string
	Summary     string
	Description string
	Assignee    string
	Status      string
}, users []struct{ Name, Email string }) {
	t.Helper()

	// Collect all assignees from issues
	assigneeMap := make(map[string]bool)
	for _, issue := range issues {
		retrieved, _, err := client.Issue.Get(issue.Key, nil)
		require.NoError(t, err, "Get should succeed for issue: %s", issue.Key)

		if retrieved.Fields.Assignee != nil {
			assigneeMap[retrieved.Fields.Assignee.Name] = true
		}
	}

	// Verify all users are present as assignees
	for _, user := range users {
		assert.True(t, assigneeMap[user.Name], "User should be assigned to at least one issue: "+user.Name)
	}
}

// verifyDatabaseIsolation verifies database isolation
func verifyDatabaseIsolation(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string,
	projects []struct{ ID, Key, Name string },
	issues []struct {
		ID          string
		Key         string
		ProjectKey  string
		IssueType   string
		Summary     string
		Description string
		Assignee    string
		Status      string
	},
	users []struct{ Name, Email string },
) {
	t.Helper()

	// Query projects from database
	dbProjects, err := queries.ListJiraProjects(ctx, sessionID)
	require.NoError(t, err, "ListJiraProjects should succeed")
	assert.Len(t, dbProjects, 3, "Should have 3 projects in database")

	// Verify project details
	for _, proj := range projects {
		dbProject, err := queries.GetJiraProjectByKey(ctx, database.GetJiraProjectByKeyParams{
			Key:       proj.Key,
			SessionID: sessionID,
		})
		require.NoError(t, err, "GetJiraProjectByKey should succeed for: %s", proj.Key)
		assert.Equal(t, proj.Name, dbProject.Name, "Project name should match in database")
	}

	// Query issues from database
	for _, issue := range issues {
		dbIssue, err := queries.GetJiraIssueByKey(ctx, database.GetJiraIssueByKeyParams{
			Key:       issue.Key,
			SessionID: sessionID,
		})
		require.NoError(t, err, "GetJiraIssueByKey should succeed for: %s", issue.Key)
		assert.Equal(t, issue.Summary, dbIssue.Summary, "Issue summary should match in database")
		assert.Equal(t, issue.Status, dbIssue.Status, "Issue status should match in database")

		if dbIssue.Description.Valid {
			assert.Equal(t, issue.Description, dbIssue.Description.String, "Issue description should match in database")
		}

		if dbIssue.Assignee.Valid {
			assert.Equal(t, issue.Assignee, dbIssue.Assignee.String, "Issue assignee should match in database")
		}
	}

	// Search all issues in database
	allIssues, err := queries.SearchJiraIssues(ctx, database.SearchJiraIssuesParams{
		SessionID:  sessionID,
		Column2:    "",
		ProjectKey: "",
		Column4:    "",
		IssueType:  "",
		Column6:    "",
		Column7:    sql.NullString{String: "", Valid: false},
		Column8:    "",
		Assignee:   sql.NullString{String: "", Valid: false},
		Column10:   "",
		Status:     "",
		Limit:      100,
	})
	require.NoError(t, err, "SearchJiraIssues should succeed")
	assert.Len(t, allIssues, 3, "Should have 3 issues in database")
}
