package jira_test

import (
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

// sessionHTTPTransport wraps http.RoundTripper and adds session header to all requests
type sessionHTTPTransport struct {
	sessionID string
	base      http.RoundTripper
}

func (t *sessionHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-Session-ID", t.sessionID)
	if t.base != nil {
		return t.base.RoundTrip(req)
	}
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

func TestJiraSimulatorCreateIssue(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "jira-test-session-1"

	// Setup: Start simulator server with session middleware
	mux := http.NewServeMux()
	jiraHandler := session.Middleware(simulatorJira.NewHandler(queries))
	mux.Handle("/jira/", http.StripPrefix("/jira", jiraHandler))
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create custom HTTP client that adds session header
	transport := jira.BasicAuthTransport{
		Username: "test@example.com",
		Password: "test-token",
		Transport: &sessionHTTPTransport{
			sessionID: sessionID,
		},
	}

	// Create Jira client pointing to test server
	client, err := jira.NewClient(transport.Client(), server.URL+"/jira")
	require.NoError(t, err, "Failed to create Jira client")

	t.Run("CreateIssue", func(t *testing.T) {
		// Create issue
		issue := jira.Issue{
			Fields: &jira.IssueFields{
				Project: jira.Project{
					Key: "TEST",
				},
				Type: jira.IssueType{
					Name: "Task",
				},
				Summary:     "Test Issue",
				Description: "This is a test issue",
			},
		}

		created, _, err := client.Issue.Create(&issue)
		require.NoError(t, err, "Create should not return error")
		assert.NotNil(t, created, "Should return created issue")
		assert.NotEmpty(t, created.ID, "Issue ID should not be empty")
		assert.NotEmpty(t, created.Key, "Issue key should not be empty")
		assert.Equal(t, "TEST", created.Fields.Project.Key, "Project key should match")
		assert.Equal(t, "Task", created.Fields.Type.Name, "Issue type should match")
		assert.Equal(t, "Test Issue", created.Fields.Summary, "Summary should match")
	})

	t.Run("CreateIssueWithAssignee", func(t *testing.T) {
		// Create issue with assignee
		assignee := "john.doe"
		issue := jira.Issue{
			Fields: &jira.IssueFields{
				Project: jira.Project{
					Key: "TEST",
				},
				Type: jira.IssueType{
					Name: "Bug",
				},
				Summary:     "Bug Report",
				Description: "This is a bug",
				Assignee: &jira.User{
					Name: assignee,
				},
			},
		}

		created, _, err := client.Issue.Create(&issue)
		require.NoError(t, err, "Create should not return error")
		assert.NotNil(t, created.Fields.Assignee, "Should have assignee")
		assert.Equal(t, assignee, created.Fields.Assignee.Name, "Assignee should match")
	})
}

func TestJiraSimulatorGetIssue(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "jira-test-session-2"

	// Setup: Start simulator server with session middleware
	mux := http.NewServeMux()
	jiraHandler := session.Middleware(simulatorJira.NewHandler(queries))
	mux.Handle("/jira/", http.StripPrefix("/jira", jiraHandler))
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create Jira client
	transport := jira.BasicAuthTransport{
		Username: "test@example.com",
		Password: "test-token",
		Transport: &sessionHTTPTransport{
			sessionID: sessionID,
		},
	}
	client, err := jira.NewClient(transport.Client(), server.URL+"/jira")
	require.NoError(t, err, "Failed to create Jira client")

	// Create an issue first
	issue := jira.Issue{
		Fields: &jira.IssueFields{
			Project: jira.Project{
				Key: "PROJ",
			},
			Type: jira.IssueType{
				Name: "Story",
			},
			Summary:     "User Story",
			Description: "As a user, I want to...",
		},
	}
	created, _, err := client.Issue.Create(&issue)
	require.NoError(t, err, "Create should succeed")

	t.Run("GetIssue", func(t *testing.T) {
		// Get the issue
		retrieved, _, err := client.Issue.Get(created.Key, nil)
		require.NoError(t, err, "Get should not return error")
		assert.NotNil(t, retrieved, "Should return issue")
		assert.Equal(t, created.Key, retrieved.Key, "Issue key should match")
		assert.Equal(t, "User Story", retrieved.Fields.Summary, "Summary should match")
		assert.Equal(t, "As a user, I want to...", retrieved.Fields.Description, "Description should match")
	})

	t.Run("GetNonExistentIssue", func(t *testing.T) {
		// Try to get a non-existent issue
		_, _, err := client.Issue.Get("NONEXISTENT-999", nil)
		assert.Error(t, err, "Should return error for non-existent issue")
	})
}

func TestJiraSimulatorUpdateIssue(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "jira-test-session-3"

	// Setup: Start simulator server with session middleware
	mux := http.NewServeMux()
	jiraHandler := session.Middleware(simulatorJira.NewHandler(queries))
	mux.Handle("/jira/", http.StripPrefix("/jira", jiraHandler))
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create Jira client
	transport := jira.BasicAuthTransport{
		Username: "test@example.com",
		Password: "test-token",
		Transport: &sessionHTTPTransport{
			sessionID: sessionID,
		},
	}
	client, err := jira.NewClient(transport.Client(), server.URL+"/jira")
	require.NoError(t, err, "Failed to create Jira client")

	// Create an issue first
	issue := jira.Issue{
		Fields: &jira.IssueFields{
			Project: jira.Project{
				Key: "UPD",
			},
			Type: jira.IssueType{
				Name: "Task",
			},
			Summary:     "Original Summary",
			Description: "Original Description",
		},
	}
	created, _, err := client.Issue.Create(&issue)
	require.NoError(t, err, "Create should succeed")

	t.Run("UpdateIssueSummary", func(t *testing.T) {
		// Update the issue
		updateIssue := jira.Issue{
			Key: created.Key,
			Fields: &jira.IssueFields{
				Summary: "Updated Summary",
			},
		}

		_, _, err := client.Issue.Update(&updateIssue)
		require.NoError(t, err, "Update should not return error")

		// Verify update
		retrieved, _, err := client.Issue.Get(created.Key, nil)
		require.NoError(t, err, "Get should succeed")
		assert.Equal(t, "Updated Summary", retrieved.Fields.Summary, "Summary should be updated")
	})

	t.Run("UpdateIssueAssignee", func(t *testing.T) {
		// Update assignee
		updateIssue := jira.Issue{
			Key: created.Key,
			Fields: &jira.IssueFields{
				Assignee: &jira.User{
					Name: "jane.smith",
				},
			},
		}

		_, _, err := client.Issue.Update(&updateIssue)
		require.NoError(t, err, "Update should not return error")

		// Verify update
		retrieved, _, err := client.Issue.Get(created.Key, nil)
		require.NoError(t, err, "Get should succeed")
		assert.NotNil(t, retrieved.Fields.Assignee, "Should have assignee")
		assert.Equal(t, "jane.smith", retrieved.Fields.Assignee.Name, "Assignee should be updated")
	})
}

func TestJiraSimulatorAddComment(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "jira-test-session-4"

	// Setup: Start simulator server with session middleware
	mux := http.NewServeMux()
	jiraHandler := session.Middleware(simulatorJira.NewHandler(queries))
	mux.Handle("/jira/", http.StripPrefix("/jira", jiraHandler))
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create Jira client
	transport := jira.BasicAuthTransport{
		Username: "test@example.com",
		Password: "test-token",
		Transport: &sessionHTTPTransport{
			sessionID: sessionID,
		},
	}
	client, err := jira.NewClient(transport.Client(), server.URL+"/jira")
	require.NoError(t, err, "Failed to create Jira client")

	// Create an issue first
	issue := jira.Issue{
		Fields: &jira.IssueFields{
			Project: jira.Project{
				Key: "COM",
			},
			Type: jira.IssueType{
				Name: "Task",
			},
			Summary: "Issue for Comments",
		},
	}
	created, _, err := client.Issue.Create(&issue)
	require.NoError(t, err, "Create should succeed")

	t.Run("AddComment", func(t *testing.T) {
		// Add comment
		comment := &jira.Comment{
			Body: "This is a test comment",
		}

		addedComment, _, err := client.Issue.AddComment(created.Key, comment)
		require.NoError(t, err, "AddComment should not return error")
		assert.NotNil(t, addedComment, "Should return added comment")
		assert.NotEmpty(t, addedComment.ID, "Comment ID should not be empty")
		assert.Equal(t, "This is a test comment", addedComment.Body, "Comment body should match")
	})
}

func TestJiraSimulatorSearchIssues(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "jira-test-session-5"

	// Setup: Start simulator server with session middleware
	mux := http.NewServeMux()
	jiraHandler := session.Middleware(simulatorJira.NewHandler(queries))
	mux.Handle("/jira/", http.StripPrefix("/jira", jiraHandler))
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create Jira client
	transport := jira.BasicAuthTransport{
		Username: "test@example.com",
		Password: "test-token",
		Transport: &sessionHTTPTransport{
			sessionID: sessionID,
		},
	}
	client, err := jira.NewClient(transport.Client(), server.URL+"/jira")
	require.NoError(t, err, "Failed to create Jira client")

	// Create multiple issues
	for i := 1; i <= 3; i++ {
		issue := jira.Issue{
			Fields: &jira.IssueFields{
				Project: jira.Project{
					Key: "SEARCH",
				},
				Type: jira.IssueType{
					Name: "Task",
				},
				Summary: "Search Test " + string(rune('A'+i-1)),
			},
		}
		_, _, err := client.Issue.Create(&issue)
		require.NoError(t, err, "Create should succeed")
	}

	t.Run("SearchByProject", func(t *testing.T) {
		// Search issues
		issues, _, err := client.Issue.Search("project = SEARCH", &jira.SearchOptions{
			MaxResults: 50,
		})
		require.NoError(t, err, "Search should not return error")
		assert.GreaterOrEqual(t, len(issues), 3, "Should find at least 3 issues")
	})

	t.Run("SearchWithLimit", func(t *testing.T) {
		// Search with limit
		issues, _, err := client.Issue.Search("project = SEARCH", &jira.SearchOptions{
			MaxResults: 2,
		})
		require.NoError(t, err, "Search should not return error")
		assert.LessOrEqual(t, len(issues), 2, "Should respect max results")
	})

	t.Run("SearchBySummary", func(t *testing.T) {
		// Search by summary
		issues, _, err := client.Issue.Search("summary ~ Test", &jira.SearchOptions{
			MaxResults: 50,
		})
		require.NoError(t, err, "Search should not return error")
		assert.GreaterOrEqual(t, len(issues), 3, "Should find issues with 'Test' in summary")
	})
}

func TestJiraSimulatorTransitions(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "jira-test-session-6"

	// Setup: Start simulator server with session middleware
	mux := http.NewServeMux()
	jiraHandler := session.Middleware(simulatorJira.NewHandler(queries))
	mux.Handle("/jira/", http.StripPrefix("/jira", jiraHandler))
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create Jira client
	transport := jira.BasicAuthTransport{
		Username: "test@example.com",
		Password: "test-token",
		Transport: &sessionHTTPTransport{
			sessionID: sessionID,
		},
	}
	client, err := jira.NewClient(transport.Client(), server.URL+"/jira")
	require.NoError(t, err, "Failed to create Jira client")

	// Create an issue first
	issue := jira.Issue{
		Fields: &jira.IssueFields{
			Project: jira.Project{
				Key: "TRANS",
			},
			Type: jira.IssueType{
				Name: "Task",
			},
			Summary: "Issue for Transitions",
		},
	}
	created, _, err := client.Issue.Create(&issue)
	require.NoError(t, err, "Create should succeed")

	t.Run("GetTransitions", func(t *testing.T) {
		// Get transitions
		transitions, _, err := client.Issue.GetTransitions(created.Key)
		require.NoError(t, err, "GetTransitions should not return error")
		assert.NotEmpty(t, transitions, "Should have transitions")

		// Verify default transitions exist
		transitionNames := make(map[string]bool)
		for i := range transitions {
			transitionNames[transitions[i].Name] = true
		}
		assert.True(t, transitionNames["Start Progress"] || transitionNames["Done"], "Should have default transitions")
	})

	t.Run("ExecuteTransition", func(t *testing.T) {
		// Get transitions
		transitions, _, err := client.Issue.GetTransitions(created.Key)
		require.NoError(t, err, "GetTransitions should succeed")
		require.NotEmpty(t, transitions, "Should have transitions")

		// Find "Start Progress" transition
		var startProgressID string
		for i := range transitions {
			if transitions[i].Name == "Start Progress" {
				startProgressID = transitions[i].ID
				break
			}
		}
		require.NotEmpty(t, startProgressID, "Should find 'Start Progress' transition")

		// Execute transition
		_, err = client.Issue.DoTransition(created.Key, startProgressID)
		require.NoError(t, err, "DoTransition should not return error")

		// Verify status changed
		retrieved, _, err := client.Issue.Get(created.Key, nil)
		require.NoError(t, err, "Get should succeed")
		assert.Equal(t, "In Progress", retrieved.Fields.Status.Name, "Status should be 'In Progress'")
	})
}

func TestJiraSimulatorListProjects(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "jira-test-session-7"

	// Setup: Start simulator server with session middleware
	mux := http.NewServeMux()
	jiraHandler := session.Middleware(simulatorJira.NewHandler(queries))
	mux.Handle("/jira/", http.StripPrefix("/jira", jiraHandler))
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create Jira client
	transport := jira.BasicAuthTransport{
		Username: "test@example.com",
		Password: "test-token",
		Transport: &sessionHTTPTransport{
			sessionID: sessionID,
		},
	}
	client, err := jira.NewClient(transport.Client(), server.URL+"/jira")
	require.NoError(t, err, "Failed to create Jira client")

	// Create issues in different projects
	projectKeys := []string{"PROJ1", "PROJ2", "PROJ3"}
	for _, key := range projectKeys {
		issue := jira.Issue{
			Fields: &jira.IssueFields{
				Project: jira.Project{
					Key:  key,
					Name: "Project " + key,
				},
				Type: jira.IssueType{
					Name: "Task",
				},
				Summary: "Test issue in " + key,
			},
		}
		_, _, err := client.Issue.Create(&issue)
		require.NoError(t, err, "Create should succeed")
	}

	t.Run("ListProjects", func(t *testing.T) {
		// List projects
		projects, _, err := client.Project.GetList()
		require.NoError(t, err, "GetList should not return error")
		assert.NotNil(t, projects, "Should return projects")
		assert.GreaterOrEqual(t, len(*projects), 3, "Should have at least 3 projects")

		// Verify project keys
		projectMap := make(map[string]bool)
		for i := range *projects {
			projectMap[(*projects)[i].Key] = true
		}
		for _, key := range projectKeys {
			assert.True(t, projectMap[key], "Should have project "+key)
		}
	})
}

func TestJiraSimulatorSessionIsolation(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test sessions
	session1 := "jira-test-session-isolation-1"
	session2 := "jira-test-session-isolation-2"

	// Setup: Start simulator server with session middleware
	mux := http.NewServeMux()
	jiraHandler := session.Middleware(simulatorJira.NewHandler(queries))
	mux.Handle("/jira/", http.StripPrefix("/jira", jiraHandler))
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create Jira clients for both sessions
	transport1 := jira.BasicAuthTransport{
		Username: "test@example.com",
		Password: "test-token",
		Transport: &sessionHTTPTransport{
			sessionID: session1,
		},
	}
	client1, err := jira.NewClient(transport1.Client(), server.URL+"/jira")
	require.NoError(t, err, "Failed to create Jira client 1")

	transport2 := jira.BasicAuthTransport{
		Username: "test@example.com",
		Password: "test-token",
		Transport: &sessionHTTPTransport{
			sessionID: session2,
		},
	}
	client2, err := jira.NewClient(transport2.Client(), server.URL+"/jira")
	require.NoError(t, err, "Failed to create Jira client 2")

	t.Run("SessionIsolation", func(t *testing.T) {
		// Create issue in session 1
		issue1 := jira.Issue{
			Fields: &jira.IssueFields{
				Project: jira.Project{
					Key: "SESS1",
				},
				Type: jira.IssueType{
					Name: "Task",
				},
				Summary: "Session 1 Issue",
			},
		}
		created1, _, err := client1.Issue.Create(&issue1)
		require.NoError(t, err, "Create in session 1 should succeed")

		// Create issue in session 2
		issue2 := jira.Issue{
			Fields: &jira.IssueFields{
				Project: jira.Project{
					Key: "SESS2",
				},
				Type: jira.IssueType{
					Name: "Task",
				},
				Summary: "Session 2 Issue",
			},
		}
		created2, _, err := client2.Issue.Create(&issue2)
		require.NoError(t, err, "Create in session 2 should succeed")

		// Session 1 should see only its issue
		issues1, _, err := client1.Issue.Search("project = SESS1", nil)
		require.NoError(t, err, "Search in session 1 should succeed")
		assert.Len(t, issues1, 1, "Session 1 should have 1 issue")
		assert.Equal(t, created1.Key, issues1[0].Key, "Should be session 1 issue")

		// Session 2 should see only its issue
		issues2, _, err := client2.Issue.Search("project = SESS2", nil)
		require.NoError(t, err, "Search in session 2 should succeed")
		assert.Len(t, issues2, 1, "Session 2 should have 1 issue")
		assert.Equal(t, created2.Key, issues2[0].Key, "Should be session 2 issue")

		// Session 1 should not see session 2's issue
		_, _, err = client1.Issue.Get(created2.Key, nil)
		require.Error(t, err, "Session 1 should not see session 2 issue")

		// Session 2 should not see session 1's issue
		_, _, err = client2.Issue.Get(created1.Key, nil)
		assert.Error(t, err, "Session 2 should not see session 1 issue")
	})
}
