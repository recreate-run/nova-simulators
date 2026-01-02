package github_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/v80/github"
	"github.com/pressly/goose/v3"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
	simulatorGithub "github.com/recreate-run/nova-simulators/simulators/github"
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

// TestGithubInitialStateSeed demonstrates seeding arbitrary initial state for GitHub simulator
func TestGithubInitialStateSeed(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDBForSeed(t)
	ctx := context.Background()

	// Setup: Create a new session
	sessionID := "github-seed-test-session"
	err := queries.CreateSession(ctx, sessionID)
	require.NoError(t, err, "Failed to create session")

	// Seed: Create custom repositories, issues, pull requests, and users
	repos, issues, prs := seedGithubTestData(t, ctx, queries, sessionID)

	// Setup: Start simulator server with session middleware
	handler := session.Middleware(simulatorGithub.NewHandler(queries))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create custom HTTP client that adds session header
	transport := &sessionHTTPTransport{
		sessionID: sessionID,
	}
	customClient := &http.Client{
		Transport: transport,
	}

	// Create GitHub client
	client := github.NewClient(customClient).WithAuthToken("test-token")
	client, err = client.WithEnterpriseURLs(server.URL, server.URL)
	require.NoError(t, err, "Failed to set enterprise URLs")

	// Verify: Check that repositories are queryable
	t.Run("VerifyRepositories", func(t *testing.T) {
		verifyRepositories(t, ctx, client, repos)
	})

	// Verify: Check that issues are queryable
	t.Run("VerifyIssues", func(t *testing.T) {
		verifyIssues(t, ctx, client, repos[0].Owner, repos[0].Name, issues)
	})

	// Verify: Check that pull requests are queryable
	t.Run("VerifyPullRequests", func(t *testing.T) {
		verifyPullRequests(t, ctx, client, repos[0].Owner, repos[0].Name, prs)
	})

	// Verify: Check database isolation - ensure all data is correctly stored
	t.Run("VerifyDatabaseIsolation", func(t *testing.T) {
		verifyDatabaseIsolation(t, ctx, queries, sessionID, repos, issues, prs)
	})
}

// seedGithubTestData creates repositories, issues, and pull requests for testing
func seedGithubTestData(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string) (
	repos []struct{ Owner, Name, DefaultBranch string },
	issues []struct {
		Number int64
		Title  string
		Body   string
		State  string
	},
	prs []struct {
		Number int64
		Title  string
		Body   string
		Head   string
		Base   string
		State  string
	},
) {
	t.Helper()

	// Seed: Create repositories (use session-specific names to avoid conflicts)
	repos = []struct {
		Owner         string
		Name          string
		DefaultBranch string
	}{
		{"OWNER_" + sessionID, "REPO_backend_" + sessionID, "main"},
		{"OWNER_" + sessionID, "REPO_frontend_" + sessionID, "main"},
	}

	for _, repo := range repos {
		err := queries.CreateGithubRepository(ctx, database.CreateGithubRepositoryParams{
			Owner:         repo.Owner,
			Name:          repo.Name,
			DefaultBranch: repo.DefaultBranch,
			Description:   sql.NullString{},
			SessionID:     sessionID,
		})
		require.NoError(t, err, "Failed to create repository: %s/%s", repo.Owner, repo.Name)

		// Create main branch for each repository
		err = queries.CreateGithubBranch(ctx, database.CreateGithubBranchParams{
			RepoOwner: repo.Owner,
			RepoName:  repo.Name,
			Name:      repo.DefaultBranch,
			Sha:       "abc123" + sessionID,
			SessionID: sessionID,
		})
		require.NoError(t, err, "Failed to create main branch for repository: %s/%s", repo.Owner, repo.Name)
	}

	// Seed: Create issues in first repository (all open for easier testing)
	issues = []struct {
		Number int64
		Title  string
		Body   string
		State  string
	}{
		{1, "Bug: Login fails", "Users cannot log in with OAuth", "open"},
		{2, "Feature: Add dark mode", "Implement dark mode theme", "open"},
		{3, "Docs: Update README", "Add setup instructions", "open"},
	}

	for _, issue := range issues {
		_, err := queries.CreateGithubIssue(ctx, database.CreateGithubIssueParams{
			RepoOwner: repos[0].Owner,
			RepoName:  repos[0].Name,
			Number:    issue.Number,
			Title:     issue.Title,
			Body:      sql.NullString{String: issue.Body, Valid: true},
			State:     issue.State,
			SessionID: sessionID,
		})
		require.NoError(t, err, "Failed to create issue #%d", issue.Number)
	}

	// Seed: Create pull requests in first repository
	prs = []struct {
		Number int64
		Title  string
		Body   string
		Head   string
		Base   string
		State  string
	}{
		{1, "Fix: Resolve login bug", "Fixes #1", "fix-login", "main", "open"},
		{2, "Feature: Dark mode implementation", "Implements dark mode", "dark-mode", "main", "open"},
	}

	for _, pr := range prs {
		// Create branch for PR head
		err := queries.CreateGithubBranch(ctx, database.CreateGithubBranchParams{
			RepoOwner: repos[0].Owner,
			RepoName:  repos[0].Name,
			Name:      pr.Head,
			Sha:       "def456" + sessionID,
			SessionID: sessionID,
		})
		require.NoError(t, err, "Failed to create branch: %s", pr.Head)

		_, err = queries.CreateGithubPullRequest(ctx, database.CreateGithubPullRequestParams{
			RepoOwner: repos[0].Owner,
			RepoName:  repos[0].Name,
			Number:    pr.Number,
			Title:     pr.Title,
			Body:      sql.NullString{String: pr.Body, Valid: true},
			Head:      pr.Head,
			Base:      pr.Base,
			State:     pr.State,
			SessionID: sessionID,
		})
		require.NoError(t, err, "Failed to create pull request #%d", pr.Number)
	}

	return repos, issues, prs
}

// verifyRepositories verifies that repositories can be queried via API
func verifyRepositories(t *testing.T, ctx context.Context, client *github.Client, repos []struct{ Owner, Name, DefaultBranch string }) {
	t.Helper()

	for _, repo := range repos {
		repository, _, err := client.Repositories.Get(ctx, repo.Owner, repo.Name)
		require.NoError(t, err, "Get repository should succeed for %s/%s", repo.Owner, repo.Name)
		assert.Equal(t, repo.Name, repository.GetName(), "Repository name should match")
		assert.Equal(t, repo.Owner, repository.Owner.GetLogin(), "Repository owner should match")
		assert.Equal(t, repo.DefaultBranch, repository.GetDefaultBranch(), "Default branch should match")
	}
}

// verifyIssues verifies that issues can be queried via API
func verifyIssues(t *testing.T, ctx context.Context, client *github.Client, owner, repo string, issues []struct {
	Number int64
	Title  string
	Body   string
	State  string
}) {
	t.Helper()

	// List open issues (default state filter in GitHub simulator)
	// Note: all test issues are created with "open" state
	allIssues, _, err := client.Issues.ListByRepo(ctx, owner, repo, &github.IssueListByRepoOptions{
		State: "open",
	})
	require.NoError(t, err, "List issues should succeed")
	assert.Len(t, allIssues, len(issues), "Should have expected number of issues")

	// Verify each issue
	for _, issue := range issues {
		dbIssue, _, err := client.Issues.Get(ctx, owner, repo, int(issue.Number))
		require.NoError(t, err, "Get issue should succeed for issue #%d", issue.Number)
		assert.Equal(t, issue.Title, dbIssue.GetTitle(), "Issue title should match")
		assert.Equal(t, issue.Body, dbIssue.GetBody(), "Issue body should match")
		assert.Equal(t, issue.State, dbIssue.GetState(), "Issue state should match")
	}
}

// verifyPullRequests verifies that pull requests can be queried via API
func verifyPullRequests(t *testing.T, ctx context.Context, client *github.Client, owner, repo string, prs []struct {
	Number int64
	Title  string
	Body   string
	Head   string
	Base   string
	State  string
}) {
	t.Helper()

	// List open pull requests (default state filter in GitHub simulator)
	// Note: all test PRs are created with "open" state
	allPRs, _, err := client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{
		State: "open",
	})
	require.NoError(t, err, "List pull requests should succeed")
	assert.Len(t, allPRs, len(prs), "Should have expected number of pull requests")

	// Verify each pull request
	for _, pr := range prs {
		dbPR, _, err := client.PullRequests.Get(ctx, owner, repo, int(pr.Number))
		require.NoError(t, err, "Get pull request should succeed for PR #%d", pr.Number)
		assert.Equal(t, pr.Title, dbPR.GetTitle(), "PR title should match")
		assert.Equal(t, pr.Body, dbPR.GetBody(), "PR body should match")
		assert.Equal(t, pr.Head, dbPR.Head.GetRef(), "PR head should match")
		assert.Equal(t, pr.Base, dbPR.Base.GetRef(), "PR base should match")
		assert.Equal(t, pr.State, dbPR.GetState(), "PR state should match")
	}
}

// verifyDatabaseIsolation verifies database isolation
func verifyDatabaseIsolation(t *testing.T, ctx context.Context, queries *database.Queries, sessionID string,
	repos []struct{ Owner, Name, DefaultBranch string },
	issues []struct {
		Number int64
		Title  string
		Body   string
		State  string
	},
	prs []struct {
		Number int64
		Title  string
		Body   string
		Head   string
		Base   string
		State  string
	}) {
	t.Helper()

	// Query repositories from database
	dbRepos, err := queries.ListGithubRepositories(ctx, database.ListGithubRepositoriesParams{
		SessionID: sessionID,
		Owner:     repos[0].Owner,
	})
	require.NoError(t, err, "ListGithubRepositories should succeed")
	assert.Len(t, dbRepos, len(repos), "Should have correct number of repositories in database")

	// Verify repository names
	repoNames := make(map[string]bool)
	for _, repo := range dbRepos {
		repoNames[repo.Name] = true
	}
	for _, repo := range repos {
		assert.True(t, repoNames[repo.Name], "Should have repository: %s", repo.Name)
	}

	// Query issues from database
	dbIssues, err := queries.ListGithubIssues(ctx, database.ListGithubIssuesParams{
		RepoOwner:   repos[0].Owner,
		RepoName:    repos[0].Name,
		SessionID:   sessionID,
		StateFilter: "",
	})
	require.NoError(t, err, "ListGithubIssues should succeed")
	assert.Len(t, dbIssues, len(issues), "Should have correct number of issues in database")

	// Verify issue titles
	for i, issue := range dbIssues {
		assert.Equal(t, issues[i].Title, issue.Title, "Issue title should match in database")
	}

	// Query pull requests from database
	dbPRs, err := queries.ListGithubPullRequests(ctx, database.ListGithubPullRequestsParams{
		RepoOwner:   repos[0].Owner,
		RepoName:    repos[0].Name,
		SessionID:   sessionID,
		StateFilter: "",
	})
	require.NoError(t, err, "ListGithubPullRequests should succeed")
	assert.Len(t, dbPRs, len(prs), "Should have correct number of pull requests in database")

	// Verify PR titles
	for i, pr := range dbPRs {
		assert.Equal(t, prs[i].Title, pr.Title, "PR title should match in database")
	}
}
