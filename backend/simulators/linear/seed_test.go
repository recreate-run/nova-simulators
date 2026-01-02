package linear_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/machinebox/graphql"
	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
	"github.com/recreate-run/nova-simulators/internal/transport"
	simulatorLinear "github.com/recreate-run/nova-simulators/simulators/linear"
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

// TestLinearInitialStateSeed demonstrates seeding arbitrary initial state for Linear simulator
func TestLinearInitialStateSeed(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDBForSeed(t)
	ctx := context.Background()

	// Setup: Create a new session
	sessionID := "linear-seed-test-session"
	err := queries.CreateSession(ctx, sessionID)
	require.NoError(t, err, "Failed to create session")

	// Seed: Create custom teams, users, states, and issues
	teams, users, states, issues := seedLinearTestData(t, ctx, queries, sessionID)

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorLinear.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Install HTTP interceptor to route api.linear.app to test server with session ID
	http.DefaultTransport = transport.NewSimulatorTransport(map[string]string{
		"api.linear.app": server.URL[7:], // Strip "http://" prefix
	}).WithSessionID(sessionID)

	// Create Linear GraphQL client
	client := graphql.NewClient("https://api.linear.app/graphql")

	// Verify: Check that teams are queryable
	t.Run("VerifyTeams", func(t *testing.T) {
		verifyTeams(t, ctx, client, teams)
	})

	// Verify: Check that users are queryable
	t.Run("VerifyUsers", func(t *testing.T) {
		verifyUsers(t, ctx, client, users)
	})

	// Verify: Check that issues are queryable
	t.Run("VerifyIssues", func(t *testing.T) {
		verifyIssues(t, ctx, client, teams[0].ID, issues)
	})

	// Verify: Check database isolation - ensure all data is correctly stored
	t.Run("VerifyDatabaseIsolation", func(t *testing.T) {
		verifyDatabaseIsolation(t, ctx, queries, sessionID, teams, users, states, issues)
	})
}

// seedLinearTestData creates teams, users, states, and issues for testing
func seedLinearTestData(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string) (
	teams []struct{ ID, Name, Key string },
	users []struct{ ID, Name, Email string },
	states []struct{ ID, Name, Type, TeamID string },
	issues []struct {
		ID          string
		TeamID      string
		Title       string
		Description string
		AssigneeID  sql.NullString
		StateID     sql.NullString
		URL         string
	},
) {
	t.Helper()

	// Seed: Create teams (use session-specific IDs to avoid conflicts)
	teams = []struct {
		ID   string
		Name string
		Key  string
	}{
		{"TEAM_ENG_" + sessionID, "Engineering", "ENG"},
		{"TEAM_DESIGN_" + sessionID, "Design", "DES"},
	}

	for _, team := range teams {
		err := queries.CreateLinearTeam(ctx, database.CreateLinearTeamParams{
			ID:        team.ID,
			Name:      team.Name,
			Key:       team.Key,
			SessionID: sessionID,
		})
		require.NoError(t, err, "Failed to create team: %s", team.Name)
	}

	// Seed: Create users (use session-specific IDs to avoid conflicts)
	users = []struct {
		ID    string
		Name  string
		Email string
	}{
		{"USER_ALICE_" + sessionID, "Alice Johnson", "alice@example.com"},
		{"USER_BOB_" + sessionID, "Bob Smith", "bob@example.com"},
		{"USER_CHARLIE_" + sessionID, "Charlie Brown", "charlie@example.com"},
	}

	for _, user := range users {
		err := queries.CreateLinearUser(ctx, database.CreateLinearUserParams{
			ID:        user.ID,
			Name:      user.Name,
			Email:     user.Email,
			SessionID: sessionID,
		})
		require.NoError(t, err, "Failed to create user: %s", user.Name)
	}

	// Seed: Create states for first team
	states = []struct {
		ID     string
		Name   string
		Type   string
		TeamID string
	}{
		{"STATE_TODO_" + sessionID, "Todo", "unstarted", teams[0].ID},
		{"STATE_INPROGRESS_" + sessionID, "In Progress", "started", teams[0].ID},
		{"STATE_DONE_" + sessionID, "Done", "completed", teams[0].ID},
	}

	for _, state := range states {
		err := queries.CreateLinearState(ctx, database.CreateLinearStateParams{
			ID:        state.ID,
			Name:      state.Name,
			Type:      state.Type,
			TeamID:    state.TeamID,
			SessionID: sessionID,
		})
		require.NoError(t, err, "Failed to create state: %s", state.Name)
	}

	// Seed: Create issues in first team
	issues = []struct {
		ID          string
		TeamID      string
		Title       string
		Description string
		AssigneeID  sql.NullString
		StateID     sql.NullString
		URL         string
	}{
		{
			ID:          "ISSUE_1_" + sessionID,
			TeamID:      teams[0].ID,
			Title:       "Implement user authentication",
			Description: "Add OAuth2 support for user login",
			AssigneeID:  sql.NullString{String: users[0].ID, Valid: true},
			StateID:     sql.NullString{String: states[1].ID, Valid: true}, // In Progress
			URL:         "https://linear.app/issue/ISSUE_1",
		},
		{
			ID:          "ISSUE_2_" + sessionID,
			TeamID:      teams[0].ID,
			Title:       "Fix login bug",
			Description: "Users cannot log in with Google",
			AssigneeID:  sql.NullString{String: users[1].ID, Valid: true},
			StateID:     sql.NullString{String: states[0].ID, Valid: true}, // Todo
			URL:         "https://linear.app/issue/ISSUE_2",
		},
		{
			ID:          "ISSUE_3_" + sessionID,
			TeamID:      teams[0].ID,
			Title:       "Update documentation",
			Description: "Document new API endpoints",
			AssigneeID:  sql.NullString{},
			StateID:     sql.NullString{String: states[2].ID, Valid: true}, // Done
			URL:         "https://linear.app/issue/ISSUE_3",
		},
	}

	now := time.Now().Unix()
	for _, issue := range issues {
		_, err := queries.CreateLinearIssue(ctx, database.CreateLinearIssueParams{
			ID:          issue.ID,
			TeamID:      issue.TeamID,
			Title:       issue.Title,
			Description: sql.NullString{String: issue.Description, Valid: true},
			AssigneeID:  issue.AssigneeID,
			StateID:     issue.StateID,
			Url:         issue.URL,
			SessionID:   sessionID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		require.NoError(t, err, "Failed to create issue: %s", issue.Title)
	}

	return teams, users, states, issues
}

// verifyTeams verifies that teams can be queried via GraphQL API
func verifyTeams(t *testing.T, ctx context.Context, client *graphql.Client, teams []struct{ ID, Name, Key string }) {
	t.Helper()

	// Query teams
	query := `
		query Teams {
			teams {
				nodes {
					id
					name
					key
				}
			}
		}
	`

	var response struct {
		Teams struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Key  string `json:"key"`
			} `json:"nodes"`
		} `json:"teams"`
	}

	err := client.Run(ctx, graphql.NewRequest(query), &response)
	require.NoError(t, err, "Teams query should succeed")
	assert.Len(t, response.Teams.Nodes, len(teams), "Should have correct number of teams")

	// Verify team names
	teamNames := make(map[string]bool)
	for _, team := range response.Teams.Nodes {
		teamNames[team.Name] = true
	}
	for _, team := range teams {
		assert.True(t, teamNames[team.Name], "Should have team: %s", team.Name)
	}
}

// verifyUsers verifies that users can be queried via GraphQL API
func verifyUsers(t *testing.T, ctx context.Context, client *graphql.Client, users []struct{ ID, Name, Email string }) {
	t.Helper()

	// Query users
	query := `
		query Users {
			users {
				nodes {
					id
					name
					email
				}
			}
		}
	`

	var response struct {
		Users struct {
			Nodes []struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"nodes"`
		} `json:"users"`
	}

	err := client.Run(ctx, graphql.NewRequest(query), &response)
	require.NoError(t, err, "Users query should succeed")
	assert.Len(t, response.Users.Nodes, len(users), "Should have correct number of users")

	// Verify user names
	userEmails := make(map[string]bool)
	for _, user := range response.Users.Nodes {
		userEmails[user.Email] = true
	}
	for _, user := range users {
		assert.True(t, userEmails[user.Email], "Should have user: %s", user.Email)
	}
}

// verifyIssues verifies that issues can be queried via GraphQL API
func verifyIssues(t *testing.T, ctx context.Context, client *graphql.Client, teamID string, issues []struct {
	ID          string
	TeamID      string
	Title       string
	Description string
	AssigneeID  sql.NullString
	StateID     sql.NullString
	URL         string
}) {
	t.Helper()

	// Query issues by team
	query := `
		query TeamIssues($teamId: String!) {
			team(id: $teamId) {
				issues {
					nodes {
						id
						title
						description
						url
					}
				}
			}
		}
	`

	req := graphql.NewRequest(query)
	req.Var("teamId", teamID)

	var response struct {
		Team struct {
			Issues struct {
				Nodes []struct {
					ID          string `json:"id"`
					Title       string `json:"title"`
					Description string `json:"description"`
					URL         string `json:"url"`
				} `json:"nodes"`
			} `json:"issues"`
		} `json:"team"`
	}

	err := client.Run(ctx, req, &response)
	require.NoError(t, err, "Team issues query should succeed")
	assert.Len(t, response.Team.Issues.Nodes, len(issues), "Should have correct number of issues")

	// Verify issue titles
	issueTitles := make(map[string]bool)
	for _, issue := range response.Team.Issues.Nodes {
		issueTitles[issue.Title] = true
	}
	for _, issue := range issues {
		assert.True(t, issueTitles[issue.Title], "Should have issue: %s", issue.Title)
	}
}

// verifyDatabaseIsolation verifies database isolation
func verifyDatabaseIsolation(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string,
	teams []struct{ ID, Name, Key string },
	users []struct{ ID, Name, Email string },
	states []struct{ ID, Name, Type, TeamID string },
	issues []struct {
		ID          string
		TeamID      string
		Title       string
		Description string
		AssigneeID  sql.NullString
		StateID     sql.NullString
		URL         string
	}) {
	t.Helper()

	// Query teams from database
	dbTeams, err := queries.ListLinearTeams(ctx, sessionID)
	require.NoError(t, err, "ListLinearTeams should succeed")
	assert.Len(t, dbTeams, len(teams), "Should have correct number of teams in database")

	// Verify team names
	teamNames := make(map[string]bool)
	for _, team := range dbTeams {
		teamNames[team.Name] = true
	}
	for _, team := range teams {
		assert.True(t, teamNames[team.Name], "Should have team: %s", team.Name)
	}

	// Query users from database
	dbUsers, err := queries.ListLinearUsers(ctx, sessionID)
	require.NoError(t, err, "ListLinearUsers should succeed")
	assert.Len(t, dbUsers, len(users), "Should have correct number of users in database")

	// Verify user emails
	userEmails := make(map[string]bool)
	for _, user := range dbUsers {
		userEmails[user.Email] = true
	}
	for _, user := range users {
		assert.True(t, userEmails[user.Email], "Should have user: %s", user.Email)
	}

	// Query states from database
	dbStates, err := queries.ListLinearStatesByTeam(ctx, database.ListLinearStatesByTeamParams{
		TeamID:    teams[0].ID,
		SessionID: sessionID,
	})
	require.NoError(t, err, "ListLinearStatesByTeam should succeed")
	assert.Len(t, dbStates, len(states), "Should have correct number of states in database")

	// Query issues from database
	dbIssues, err := queries.ListLinearIssuesByTeam(ctx, database.ListLinearIssuesByTeamParams{
		TeamID:    teams[0].ID,
		SessionID: sessionID,
	})
	require.NoError(t, err, "ListLinearIssuesByTeam should succeed")
	assert.Len(t, dbIssues, len(issues), "Should have correct number of issues in database")

	// Verify issue titles
	for _, dbIssue := range dbIssues {
		found := false
		for _, issue := range issues {
			if dbIssue.Title == issue.Title {
				found = true
				break
			}
		}
		assert.True(t, found, "Issue title should match: %s", dbIssue.Title)
	}
}
