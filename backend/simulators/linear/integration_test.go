package linear_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

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

func setupTestSession(t *testing.T, queries *database.Queries, sessionID string) (teamID, userID, stateID string) {
	t.Helper()
	ctx := context.Background()

	// Create default team
	teamID = "TEAM001_" + sessionID
	err := queries.CreateLinearTeam(ctx, database.CreateLinearTeamParams{
		ID:        teamID,
		Name:      "Engineering",
		Key:       "ENG",
		SessionID: sessionID,
	})
	require.NoError(t, err, "Failed to create team")

	// Create default user
	userID = "USER001_" + sessionID
	err = queries.CreateLinearUser(ctx, database.CreateLinearUserParams{
		ID:        userID,
		Name:      "Test User",
		Email:     "test@example.com",
		SessionID: sessionID,
	})
	require.NoError(t, err, "Failed to create user")

	// Create default state
	stateID = "STATE001_" + sessionID
	err = queries.CreateLinearState(ctx, database.CreateLinearStateParams{
		ID:        stateID,
		Name:      "Todo",
		Type:      "unstarted",
		TeamID:    teamID,
		SessionID: sessionID,
	})
	require.NoError(t, err, "Failed to create state")

	return teamID, userID, stateID
}

func testCreateIssue(t *testing.T, client *graphql.Client, teamID, userID, stateID string) {
	t.Helper()
	// Create issue mutation
	mutation := `
		mutation IssueCreate($teamId: String!, $title: String!, $description: String, $assigneeId: String, $stateId: String) {
			issueCreate(
				input: {
					teamId: $teamId
					title: $title
					description: $description
					assigneeId: $assigneeId
					stateId: $stateId
				}
			) {
				success
				issue {
					id
					title
					description
					url
					assignee {
						id
						name
						email
					}
					state {
						id
						name
						type
					}
				}
			}
		}
	`

	req := graphql.NewRequest(mutation)
	req.Var("teamId", teamID)
	req.Var("title", "Test Issue")
	req.Var("description", "This is a test issue")
	req.Var("assigneeId", userID)
	req.Var("stateId", stateID)

	var response struct {
		IssueCreate struct {
			Success bool `json:"success"`
			Issue   struct {
				ID          string `json:"id"`
				Title       string `json:"title"`
				Description string `json:"description"`
				URL         string `json:"url"`
				Assignee    struct {
					ID    string `json:"id"`
					Name  string `json:"name"`
					Email string `json:"email"`
				} `json:"assignee"`
				State struct {
					ID   string `json:"id"`
					Name string `json:"name"`
					Type string `json:"type"`
				} `json:"state"`
			} `json:"issue"`
		} `json:"issueCreate"`
	}

	err := client.Run(context.Background(), req, &response)

	// Assertions
	require.NoError(t, err, "CreateIssue should not return error")
	assert.True(t, response.IssueCreate.Success, "Create should be successful")
	assert.NotEmpty(t, response.IssueCreate.Issue.ID, "Issue ID should be set")
	assert.Equal(t, "Test Issue", response.IssueCreate.Issue.Title, "Title should match")
	assert.Equal(t, "This is a test issue", response.IssueCreate.Issue.Description, "Description should match")
	assert.NotEmpty(t, response.IssueCreate.Issue.URL, "URL should be set")
	assert.Equal(t, userID, response.IssueCreate.Issue.Assignee.ID, "Assignee ID should match")
	assert.Equal(t, "Test User", response.IssueCreate.Issue.Assignee.Name, "Assignee name should match")
	assert.Equal(t, stateID, response.IssueCreate.Issue.State.ID, "State ID should match")
	assert.Equal(t, "Todo", response.IssueCreate.Issue.State.Name, "State name should match")
}

func testGetIssue(t *testing.T, client *graphql.Client, teamID string) {
	t.Helper()
	// First create an issue
	mutation := `
		mutation IssueCreate($teamId: String!, $title: String!) {
			issueCreate(input: { teamId: $teamId, title: $title }) {
				success
				issue { id }
			}
		}
	`
	createReq := graphql.NewRequest(mutation)
	createReq.Var("teamId", teamID)
	createReq.Var("title", "Issue to Get")

	var createResponse struct {
		IssueCreate struct {
			Issue struct {
				ID string `json:"id"`
			} `json:"issue"`
		} `json:"issueCreate"`
	}
	err := client.Run(context.Background(), createReq, &createResponse)
	require.NoError(t, err, "Create issue should succeed")

	issueID := createResponse.IssueCreate.Issue.ID

	// Now get the issue
	query := `
		query Issue($id: String!) {
			issue(id: $id) {
				id
				title
				description
				url
			}
		}
	`
	getReq := graphql.NewRequest(query)
	getReq.Var("id", issueID)

	var getResponse struct {
		Issue struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			URL         string `json:"url"`
		} `json:"issue"`
	}
	err = client.Run(context.Background(), getReq, &getResponse)

	// Assertions
	require.NoError(t, err, "GetIssue should not return error")
	assert.Equal(t, issueID, getResponse.Issue.ID, "Issue ID should match")
	assert.Equal(t, "Issue to Get", getResponse.Issue.Title, "Title should match")
}

func testUpdateIssue(t *testing.T, client *graphql.Client, teamID string) {
	t.Helper()
	// First create an issue
	mutation := `
		mutation IssueCreate($teamId: String!, $title: String!) {
			issueCreate(input: { teamId: $teamId, title: $title }) {
				success
				issue { id }
			}
		}
	`
	createReq := graphql.NewRequest(mutation)
	createReq.Var("teamId", teamID)
	createReq.Var("title", "Issue to Update")

	var createResponse struct {
		IssueCreate struct {
			Issue struct {
				ID string `json:"id"`
			} `json:"issue"`
		} `json:"issueCreate"`
	}
	err := client.Run(context.Background(), createReq, &createResponse)
	require.NoError(t, err, "Create issue should succeed")

	issueID := createResponse.IssueCreate.Issue.ID

	// Now update the issue
	updateMutation := `
		mutation IssueUpdate($id: String!, $title: String, $description: String) {
			issueUpdate(
				id: $id
				input: {
					title: $title
					description: $description
				}
			) {
				success
				issue {
					id
					title
					description
				}
			}
		}
	`
	updateReq := graphql.NewRequest(updateMutation)
	updateReq.Var("id", issueID)
	updateReq.Var("title", "Updated Title")
	updateReq.Var("description", "Updated description")

	var updateResponse struct {
		IssueUpdate struct {
			Success bool `json:"success"`
			Issue   struct {
				ID          string `json:"id"`
				Title       string `json:"title"`
				Description string `json:"description"`
			} `json:"issue"`
		} `json:"issueUpdate"`
	}
	err = client.Run(context.Background(), updateReq, &updateResponse)

	// Assertions
	require.NoError(t, err, "UpdateIssue should not return error")
	assert.True(t, updateResponse.IssueUpdate.Success, "Update should be successful")
	assert.Equal(t, issueID, updateResponse.IssueUpdate.Issue.ID, "Issue ID should match")
	assert.Equal(t, "Updated Title", updateResponse.IssueUpdate.Issue.Title, "Title should be updated")
	assert.Equal(t, "Updated description", updateResponse.IssueUpdate.Issue.Description, "Description should be updated")
}

func testListIssuesByTeam(t *testing.T, client *graphql.Client, teamID string) {
	t.Helper()
	// Create multiple issues for the team
	for i := 1; i <= 3; i++ {
		mutation := `
			mutation IssueCreate($teamId: String!, $title: String!) {
				issueCreate(input: { teamId: $teamId, title: $title }) {
					success
					issue { id }
				}
			}
		`
		req := graphql.NewRequest(mutation)
		req.Var("teamId", teamID)
		req.Var("title", "Issue "+string(rune('A'+i-1)))

		var response struct {
			IssueCreate struct {
				Success bool `json:"success"`
			} `json:"issueCreate"`
		}
		err := client.Run(context.Background(), req, &response)
		require.NoError(t, err, "Create issue should succeed")
	}

	// List issues for the team
	query := `
		query TeamIssues($teamId: String!) {
			team(id: $teamId) {
				issues {
					nodes {
						id
						title
					}
				}
			}
		}
	`
	listReq := graphql.NewRequest(query)
	listReq.Var("teamId", teamID)

	var listResponse struct {
		Team struct {
			Issues struct {
				Nodes []struct {
					ID    string `json:"id"`
					Title string `json:"title"`
				} `json:"nodes"`
			} `json:"issues"`
		} `json:"team"`
	}
	err := client.Run(context.Background(), listReq, &listResponse)

	// Assertions
	require.NoError(t, err, "ListIssues should not return error")
	assert.GreaterOrEqual(t, len(listResponse.Team.Issues.Nodes), 3, "Should have at least 3 issues")
}

func testGetTeams(t *testing.T, client *graphql.Client, teamID string) {
	t.Helper()
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
	req := graphql.NewRequest(query)

	var response struct {
		Teams struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Key  string `json:"key"`
			} `json:"nodes"`
		} `json:"teams"`
	}
	err := client.Run(context.Background(), req, &response)

	// Assertions
	require.NoError(t, err, "GetTeams should not return error")
	assert.Len(t, response.Teams.Nodes, 1, "Should have 1 team")
	assert.Equal(t, teamID, response.Teams.Nodes[0].ID, "Team ID should match")
	assert.Equal(t, "Engineering", response.Teams.Nodes[0].Name, "Team name should match")
	assert.Equal(t, "ENG", response.Teams.Nodes[0].Key, "Team key should match")
}

func testGetUsers(t *testing.T, client *graphql.Client, userID string) {
	t.Helper()
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
	req := graphql.NewRequest(query)

	var response struct {
		Users struct {
			Nodes []struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"nodes"`
		} `json:"users"`
	}
	err := client.Run(context.Background(), req, &response)

	// Assertions
	require.NoError(t, err, "GetUsers should not return error")
	assert.Len(t, response.Users.Nodes, 1, "Should have 1 user")
	assert.Equal(t, userID, response.Users.Nodes[0].ID, "User ID should match")
	assert.Equal(t, "Test User", response.Users.Nodes[0].Name, "User name should match")
	assert.Equal(t, "test@example.com", response.Users.Nodes[0].Email, "User email should match")
}

func TestLinearSimulatorIntegration(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "test-session-1"
	teamID, userID, stateID := setupTestSession(t, queries, sessionID)

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

	t.Run("CreateIssue", func(t *testing.T) {
		testCreateIssue(t, client, teamID, userID, stateID)
	})

	t.Run("GetIssue", func(t *testing.T) {
		testGetIssue(t, client, teamID)
	})

	t.Run("UpdateIssue", func(t *testing.T) {
		testUpdateIssue(t, client, teamID)
	})

	t.Run("ListIssuesByTeam", func(t *testing.T) {
		testListIssuesByTeam(t, client, teamID)
	})

	t.Run("GetTeams", func(t *testing.T) {
		testGetTeams(t, client, teamID)
	})

	t.Run("GetUsers", func(t *testing.T) {
		testGetUsers(t, client, userID)
	})
}

func TestLinearSimulatorSessionIsolation(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create two separate sessions
	sessionID1 := "test-session-iso-1"
	sessionID2 := "test-session-iso-2"

	teamID1, _, _ := setupTestSession(t, queries, sessionID1)
	teamID2, _, _ := setupTestSession(t, queries, sessionID2)

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorLinear.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Test session 1
	http.DefaultTransport = transport.NewSimulatorTransport(map[string]string{
		"api.linear.app": server.URL[7:],
	}).WithSessionID(sessionID1)

	client1 := graphql.NewClient("https://api.linear.app/graphql")

	// Create issue in session 1
	mutation := `
		mutation IssueCreate($teamId: String!, $title: String!) {
			issueCreate(input: { teamId: $teamId, title: $title }) {
				success
				issue { id, title }
			}
		}
	`
	req1 := graphql.NewRequest(mutation)
	req1.Var("teamId", teamID1)
	req1.Var("title", "Session 1 Issue")

	var response1 struct {
		IssueCreate struct {
			Success bool `json:"success"`
		} `json:"issueCreate"`
	}
	err := client1.Run(context.Background(), req1, &response1)
	require.NoError(t, err, "Create issue in session 1 should succeed")
	assert.True(t, response1.IssueCreate.Success, "Create should be successful")

	// Test session 2
	http.DefaultTransport = transport.NewSimulatorTransport(map[string]string{
		"api.linear.app": server.URL[7:],
	}).WithSessionID(sessionID2)

	client2 := graphql.NewClient("https://api.linear.app/graphql")

	// Create issue in session 2
	req2 := graphql.NewRequest(mutation)
	req2.Var("teamId", teamID2)
	req2.Var("title", "Session 2 Issue")

	var response2 struct {
		IssueCreate struct {
			Success bool `json:"success"`
		} `json:"issueCreate"`
	}
	err = client2.Run(context.Background(), req2, &response2)
	require.NoError(t, err, "Create issue in session 2 should succeed")
	assert.True(t, response2.IssueCreate.Success, "Create should be successful")

	// Verify session 1 only sees its own issues
	http.DefaultTransport = transport.NewSimulatorTransport(map[string]string{
		"api.linear.app": server.URL[7:],
	}).WithSessionID(sessionID1)

	query := `
		query TeamIssues($teamId: String!) {
			team(id: $teamId) {
				issues {
					nodes {
						id
						title
					}
				}
			}
		}
	`
	listReq1 := graphql.NewRequest(query)
	listReq1.Var("teamId", teamID1)

	var listResponse1 struct {
		Team struct {
			Issues struct {
				Nodes []struct {
					Title string `json:"title"`
				} `json:"nodes"`
			} `json:"issues"`
		} `json:"team"`
	}
	err = client1.Run(context.Background(), listReq1, &listResponse1)
	require.NoError(t, err, "List issues in session 1 should succeed")

	// Check that session 1 only has "Session 1 Issue"
	for _, issue := range listResponse1.Team.Issues.Nodes {
		assert.NotEqual(t, "Session 2 Issue", issue.Title, "Session 1 should not see session 2's issues")
	}
}
