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

// sessionHTTPTransport wraps http.RoundTripper and adds session header to all requests
type sessionHTTPTransport struct {
	sessionID string
	base      http.RoundTripper
}

func (t *sessionHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-Session-ID", t.sessionID)
	if t.base == nil {
		return http.DefaultTransport.RoundTrip(req)
	}
	return t.base.RoundTrip(req)
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

func TestGithubSimulatorRepository(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "github-test-session-1"

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

	// Create GitHub client pointing to test server
	ctx := context.Background()
	client := github.NewClient(customClient).WithAuthToken("test-token")
	client, err := client.WithEnterpriseURLs(server.URL, server.URL)
	require.NoError(t, err, "Failed to set enterprise URLs")

	t.Run("GetRepository", func(t *testing.T) {
		// Get repository (should auto-create)
		repo, _, err := client.Repositories.Get(ctx, "test-owner", "test-repo")

		// Assertions
		require.NoError(t, err, "Get should not return error")
		assert.NotNil(t, repo, "Should return repository")
		assert.Equal(t, "test-repo", repo.GetName(), "Repository name should match")
		assert.Equal(t, "test-owner", repo.Owner.GetLogin(), "Owner should match")
		assert.Equal(t, "main", repo.GetDefaultBranch(), "Default branch should be main")
	})
}

func TestGithubSimulatorIssues(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "github-test-session-2"

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
	ctx := context.Background()
	client := github.NewClient(customClient).WithAuthToken("test-token")
	client, err := client.WithEnterpriseURLs(server.URL, server.URL)
	require.NoError(t, err, "Failed to set enterprise URLs")

	const (
		testOwner = "test-owner"
		testRepo  = "test-repo"
	)
	owner := testOwner
	repo := testRepo

	t.Run("CreateIssue", func(t *testing.T) {
		// Create issue
		issueReq := &github.IssueRequest{
			Title: github.Ptr("Test Issue"),
			Body:  github.Ptr("This is a test issue"),
		}

		issue, _, err := client.Issues.Create(ctx, owner, repo, issueReq)

		// Assertions
		require.NoError(t, err, "Create should not return error")
		assert.NotNil(t, issue, "Should return issue")
		assert.Equal(t, "Test Issue", issue.GetTitle(), "Title should match")
		assert.Equal(t, "This is a test issue", issue.GetBody(), "Body should match")
		assert.Equal(t, 1, issue.GetNumber(), "Issue number should be 1")
		assert.Equal(t, "open", issue.GetState(), "State should be open")
	})

	t.Run("ListIssues", func(t *testing.T) {
		// Create a few issues first
		for i := 1; i <= 3; i++ {
			issueReq := &github.IssueRequest{
				Title: github.Ptr("Issue " + string(rune('A'+i-1))),
				Body:  github.Ptr("Body " + string(rune('A'+i-1))),
			}
			_, _, err := client.Issues.Create(ctx, owner, repo, issueReq)
			require.NoError(t, err, "Create should succeed")
		}

		// List issues
		issues, _, err := client.Issues.ListByRepo(ctx, owner, repo, &github.IssueListByRepoOptions{
			State: "open",
		})

		// Assertions
		require.NoError(t, err, "List should not return error")
		assert.GreaterOrEqual(t, len(issues), 3, "Should have at least 3 issues")
	})

	t.Run("GetIssue", func(t *testing.T) {
		// Create an issue
		issueReq := &github.IssueRequest{
			Title: github.Ptr("Get Test Issue"),
			Body:  github.Ptr("This is for get test"),
		}
		created, _, err := client.Issues.Create(ctx, owner, repo, issueReq)
		require.NoError(t, err, "Create should succeed")

		// Get the issue
		issue, _, err := client.Issues.Get(ctx, owner, repo, created.GetNumber())

		// Assertions
		require.NoError(t, err, "Get should not return error")
		assert.NotNil(t, issue, "Should return issue")
		assert.Equal(t, created.GetNumber(), issue.GetNumber(), "Number should match")
		assert.Equal(t, "Get Test Issue", issue.GetTitle(), "Title should match")
	})

	t.Run("UpdateIssue", func(t *testing.T) {
		// Create an issue
		issueReq := &github.IssueRequest{
			Title: github.Ptr("Original Title"),
			Body:  github.Ptr("Original Body"),
		}
		created, _, err := client.Issues.Create(ctx, owner, repo, issueReq)
		require.NoError(t, err, "Create should succeed")

		// Update the issue
		updateReq := &github.IssueRequest{
			Title: github.Ptr("Updated Title"),
			State: github.Ptr("closed"),
		}
		updated, _, err := client.Issues.Edit(ctx, owner, repo, created.GetNumber(), updateReq)

		// Assertions
		require.NoError(t, err, "Update should not return error")
		assert.Equal(t, "Updated Title", updated.GetTitle(), "Title should be updated")
		assert.Equal(t, "closed", updated.GetState(), "State should be closed")
	})

	t.Run("AddIssueComment", func(t *testing.T) {
		// Create an issue
		issueReq := &github.IssueRequest{
			Title: github.Ptr("Comment Test Issue"),
		}
		created, _, err := client.Issues.Create(ctx, owner, repo, issueReq)
		require.NoError(t, err, "Create should succeed")

		// Add comment
		commentReq := &github.IssueComment{
			Body: github.Ptr("This is a test comment"),
		}
		comment, _, err := client.Issues.CreateComment(ctx, owner, repo, created.GetNumber(), commentReq)

		// Assertions
		require.NoError(t, err, "Create comment should not return error")
		assert.NotNil(t, comment, "Should return comment")
		assert.Equal(t, "This is a test comment", comment.GetBody(), "Comment body should match")
	})
}

func TestGithubSimulatorPullRequests(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "github-test-session-3"

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
	ctx := context.Background()
	client := github.NewClient(customClient).WithAuthToken("test-token")
	client, err := client.WithEnterpriseURLs(server.URL, server.URL)
	require.NoError(t, err, "Failed to set enterprise URLs")

	const (
		testOwner = "test-owner"
		testRepo  = "test-repo"
	)
	owner := testOwner
	repo := testRepo

	t.Run("CreatePullRequest", func(t *testing.T) {
		// Create PR
		prReq := &github.NewPullRequest{
			Title: github.Ptr("Test PR"),
			Body:  github.Ptr("This is a test PR"),
			Head:  github.Ptr("feature-branch"),
			Base:  github.Ptr("main"),
		}

		pr, _, err := client.PullRequests.Create(ctx, owner, repo, prReq)

		// Assertions
		require.NoError(t, err, "Create should not return error")
		assert.NotNil(t, pr, "Should return PR")
		assert.Equal(t, "Test PR", pr.GetTitle(), "Title should match")
		assert.Equal(t, "This is a test PR", pr.GetBody(), "Body should match")
		assert.Equal(t, 1, pr.GetNumber(), "PR number should be 1")
		assert.Equal(t, "open", pr.GetState(), "State should be open")
		assert.Equal(t, "feature-branch", pr.Head.GetRef(), "Head should match")
		assert.Equal(t, "main", pr.Base.GetRef(), "Base should match")
	})

	t.Run("ListPullRequests", func(t *testing.T) {
		// Create a few PRs first
		for i := 1; i <= 3; i++ {
			prReq := &github.NewPullRequest{
				Title: github.Ptr("PR " + string(rune('A'+i-1))),
				Head:  github.Ptr("branch-" + string(rune('a'+i-1))),
				Base:  github.Ptr("main"),
			}
			_, _, err := client.PullRequests.Create(ctx, owner, repo, prReq)
			require.NoError(t, err, "Create should succeed")
		}

		// List PRs
		prs, _, err := client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{
			State: "open",
		})

		// Assertions
		require.NoError(t, err, "List should not return error")
		assert.GreaterOrEqual(t, len(prs), 3, "Should have at least 3 PRs")
	})

	t.Run("GetPullRequest", func(t *testing.T) {
		// Create a PR
		prReq := &github.NewPullRequest{
			Title: github.Ptr("Get Test PR"),
			Head:  github.Ptr("test-branch"),
			Base:  github.Ptr("main"),
		}
		created, _, err := client.PullRequests.Create(ctx, owner, repo, prReq)
		require.NoError(t, err, "Create should succeed")

		// Get the PR
		pr, _, err := client.PullRequests.Get(ctx, owner, repo, created.GetNumber())

		// Assertions
		require.NoError(t, err, "Get should not return error")
		assert.NotNil(t, pr, "Should return PR")
		assert.Equal(t, created.GetNumber(), pr.GetNumber(), "Number should match")
		assert.Equal(t, "Get Test PR", pr.GetTitle(), "Title should match")
	})

	t.Run("MergePullRequest", func(t *testing.T) {
		// Create a PR
		prReq := &github.NewPullRequest{
			Title: github.Ptr("Merge Test PR"),
			Head:  github.Ptr("merge-branch"),
			Base:  github.Ptr("main"),
		}
		created, _, err := client.PullRequests.Create(ctx, owner, repo, prReq)
		require.NoError(t, err, "Create should succeed")

		// Merge the PR
		mergeResult, _, err := client.PullRequests.Merge(ctx, owner, repo, created.GetNumber(), "Merged via test", &github.PullRequestOptions{})

		// Assertions
		require.NoError(t, err, "Merge should not return error")
		assert.NotNil(t, mergeResult, "Should return merge result")
		assert.True(t, mergeResult.GetMerged(), "Should be merged")
	})
}

func TestGithubSimulatorFiles(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "github-test-session-4"

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
	ctx := context.Background()
	client := github.NewClient(customClient).WithAuthToken("test-token")
	client, err := client.WithEnterpriseURLs(server.URL, server.URL)
	require.NoError(t, err, "Failed to set enterprise URLs")

	const (
		testOwner = "test-owner"
		testRepo  = "test-repo"
	)
	owner := testOwner
	repo := testRepo

	t.Run("CreateFile", func(t *testing.T) {
		// Create file
		opts := &github.RepositoryContentFileOptions{
			Message: github.Ptr("Add test file"),
			Content: []byte("# Test File\n\nThis is a test file."),
			Branch:  github.Ptr("main"),
		}

		result, _, err := client.Repositories.CreateFile(ctx, owner, repo, "README.md", opts)

		// Assertions
		require.NoError(t, err, "Create file should not return error")
		assert.NotNil(t, result, "Should return result")
		assert.NotNil(t, result.Content, "Should have content")
		assert.Equal(t, "README.md", result.Content.GetName(), "Name should match")
	})

	t.Run("GetFileContents", func(t *testing.T) {
		// Create file first
		content := "# Test File\n\nThis is content."
		opts := &github.RepositoryContentFileOptions{
			Message: github.Ptr("Add file"),
			Content: []byte(content),
			Branch:  github.Ptr("main"),
		}
		_, _, err := client.Repositories.CreateFile(ctx, owner, repo, "test.md", opts)
		require.NoError(t, err, "Create should succeed")

		// Get file contents
		fileContent, _, _, err := client.Repositories.GetContents(ctx, owner, repo, "test.md", &github.RepositoryContentGetOptions{
			Ref: "main",
		})

		// Assertions
		require.NoError(t, err, "Get contents should not return error")
		assert.NotNil(t, fileContent, "Should return file content")
		assert.Equal(t, "test.md", fileContent.GetName(), "Name should match")
		decodedContent, err := fileContent.GetContent()
		require.NoError(t, err, "Should decode content")
		assert.Equal(t, content, decodedContent, "Content should match")
	})

	t.Run("UpdateFile", func(t *testing.T) {
		// Create file first
		path := "update-test.md"
		opts := &github.RepositoryContentFileOptions{
			Message: github.Ptr("Add file"),
			Content: []byte("Original content"),
			Branch:  github.Ptr("main"),
		}
		created, _, err := client.Repositories.CreateFile(ctx, owner, repo, path, opts)
		require.NoError(t, err, "Create should succeed")

		// Update file
		updateOpts := &github.RepositoryContentFileOptions{
			Message: github.Ptr("Update file"),
			Content: []byte("Updated content"),
			Branch:  github.Ptr("main"),
			SHA:     created.Content.SHA,
		}
		result, _, err := client.Repositories.CreateFile(ctx, owner, repo, path, updateOpts)

		// Assertions
		require.NoError(t, err, "Update file should not return error")
		assert.NotNil(t, result, "Should return result")
	})
}

func TestGithubSimulatorBranches(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "github-test-session-5"

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
	ctx := context.Background()
	client := github.NewClient(customClient).WithAuthToken("test-token")
	client, err := client.WithEnterpriseURLs(server.URL, server.URL)
	require.NoError(t, err, "Failed to set enterprise URLs")

	const (
		testOwner = "test-owner"
		testRepo  = "test-repo"
	)
	owner := testOwner
	repo := testRepo

	t.Run("GetDefaultBranch", func(t *testing.T) {
		// Get repository to trigger creation of main branch
		_, _, err := client.Repositories.Get(ctx, owner, repo)
		require.NoError(t, err, "Get repo should succeed")

		// Get main branch ref
		ref, _, err := client.Git.GetRef(ctx, owner, repo, "refs/heads/main")

		// Assertions
		require.NoError(t, err, "Get ref should not return error")
		assert.NotNil(t, ref, "Should return ref")
		assert.Equal(t, "refs/heads/main", ref.GetRef(), "Ref should match")
		assert.NotEmpty(t, ref.Object.GetSHA(), "SHA should not be empty")
	})

	t.Run("CreateBranch", func(t *testing.T) {
		// Get repository first to ensure main branch exists
		_, _, err := client.Repositories.Get(ctx, owner, repo)
		require.NoError(t, err, "Get repo should succeed")

		// Get main branch SHA
		mainRef, _, err := client.Git.GetRef(ctx, owner, repo, "refs/heads/main")
		require.NoError(t, err, "Get main ref should succeed")

		// Create new branch
		newRef := github.CreateRef{
			Ref: "refs/heads/feature-branch",
			SHA: mainRef.Object.GetSHA(),
		}

		// Note: The actual endpoint should be POST /git/refs not POST /git/refs/heads/feature-branch
		// But we'll test what the handler supports
		_, _, err = client.Git.CreateRef(ctx, owner, repo, newRef)

		// Assertions
		require.NoError(t, err, "Create ref should not return error")
	})
}

func TestGithubSimulatorWorkflows(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "github-test-session-6"

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
	ctx := context.Background()
	client := github.NewClient(customClient).WithAuthToken("test-token")
	client, err := client.WithEnterpriseURLs(server.URL, server.URL)
	require.NoError(t, err, "Failed to set enterprise URLs")

	const (
		testOwner = "test-owner"
		testRepo  = "test-repo"
	)
	owner := testOwner
	repo := testRepo

	// Note: The simulator doesn't auto-create workflows, so we would need to
	// manually insert test workflows into the database for a complete test.
	// For now, we'll test that the endpoints respond correctly.

	t.Run("ListWorkflows", func(t *testing.T) {
		// List workflows (may be empty)
		workflows, _, err := client.Actions.ListWorkflows(ctx, owner, repo, &github.ListOptions{})

		// Assertions
		require.NoError(t, err, "List workflows should not return error")
		assert.NotNil(t, workflows, "Should return workflows response")
	})

	t.Run("ListWorkflowRuns", func(t *testing.T) {
		// List workflow runs (may be empty)
		runs, _, err := client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, &github.ListWorkflowRunsOptions{})

		// Assertions
		require.NoError(t, err, "List workflow runs should not return error")
		assert.NotNil(t, runs, "Should return runs response")
	})
}

func TestGithubSimulatorEndToEnd(t *testing.T) {
	// Setup: Create test database
	queries := setupTestDB(t)

	// Setup: Create test session
	sessionID := "github-test-session-e2e"

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
	ctx := context.Background()
	client := github.NewClient(customClient).WithAuthToken("test-token")
	client, err := client.WithEnterpriseURLs(server.URL, server.URL)
	require.NoError(t, err, "Failed to set enterprise URLs")

	owner := "e2e-owner"
	repo := "e2e-repo"

	t.Run("CompleteWorkflow", func(t *testing.T) {
		// 1. Get repository (auto-creates)
		repository, _, err := client.Repositories.Get(ctx, owner, repo)
		require.NoError(t, err, "Get repo should succeed")
		assert.Equal(t, repo, repository.GetName())

		// 2. Create an issue
		issueReq := &github.IssueRequest{
			Title: github.Ptr("Bug: Login fails"),
			Body:  github.Ptr("Users cannot log in"),
		}
		issue, _, err := client.Issues.Create(ctx, owner, repo, issueReq)
		require.NoError(t, err, "Create issue should succeed")
		assert.Equal(t, 1, issue.GetNumber())

		// 3. Create a file
		fileOpts := &github.RepositoryContentFileOptions{
			Message: github.Ptr("Fix login bug"),
			Content: []byte("def login():\n    return True"),
			Branch:  github.Ptr("main"),
		}
		_, _, err = client.Repositories.CreateFile(ctx, owner, repo, "login.py", fileOpts)
		require.NoError(t, err, "Create file should succeed")

		// 4. Get the main branch ref
		mainRef, _, err := client.Git.GetRef(ctx, owner, repo, "refs/heads/main")
		require.NoError(t, err, "Get main ref should succeed")

		// 5. Create a feature branch
		featureRef := github.CreateRef{
			Ref: "refs/heads/fix-login",
			SHA: mainRef.Object.GetSHA(),
		}
		_, _, err = client.Git.CreateRef(ctx, owner, repo, featureRef)
		require.NoError(t, err, "Create branch should succeed")

		// 6. Create a pull request
		prReq := &github.NewPullRequest{
			Title: github.Ptr("Fix: Resolve login issue"),
			Body:  github.Ptr("Fixes #1"),
			Head:  github.Ptr("fix-login"),
			Base:  github.Ptr("main"),
		}
		pr, _, err := client.PullRequests.Create(ctx, owner, repo, prReq)
		require.NoError(t, err, "Create PR should succeed")
		assert.Equal(t, 1, pr.GetNumber())

		// 7. Add a comment to the PR
		commentReq := &github.IssueComment{
			Body: github.Ptr("LGTM!"),
		}
		_, _, err = client.Issues.CreateComment(ctx, owner, repo, pr.GetNumber(), commentReq)
		require.NoError(t, err, "Add comment should succeed")

		// 8. Merge the PR
		mergeResult, _, err := client.PullRequests.Merge(ctx, owner, repo, pr.GetNumber(), "Merged", &github.PullRequestOptions{})
		require.NoError(t, err, "Merge should succeed")
		assert.True(t, mergeResult.GetMerged())

		// 9. Close the issue
		updateReq := &github.IssueRequest{
			State: github.Ptr("closed"),
		}
		closedIssue, _, err := client.Issues.Edit(ctx, owner, repo, issue.GetNumber(), updateReq)
		require.NoError(t, err, "Close issue should succeed")
		assert.Equal(t, "closed", closedIssue.GetState())
	})
}
