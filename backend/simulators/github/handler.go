package github

import (
	"context"
	"crypto/sha1" //nolint:gosec // Used for generating fake SHAs in simulator, not for security
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v80/github"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/session"
)

// Handler implements the GitHub simulator HTTP handler
type Handler struct {
	queries *database.Queries
}

// NewHandler creates a new GitHub simulator handler
func NewHandler(queries *database.Queries) *Handler {
	return &Handler{
		queries: queries,
	}
}

// ServeHTTP implements http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[github] → %s %s", r.Method, r.URL.Path)

	// Route GitHub API requests
	// Expected paths after stripping /github prefix:
	// /api/v3/repos/{owner}/{repo}
	// /api/v3/repos/{owner}/{repo}/issues
	// /api/v3/repos/{owner}/{repo}/pulls
	// /api/v3/repos/{owner}/{repo}/contents/{path}
	// /api/v3/repos/{owner}/{repo}/git/refs
	// /api/v3/repos/{owner}/{repo}/actions/workflows
	// /api/v3/repos/{owner}/{repo}/actions/runs

	// Strip /api/v3 prefix if present
	path := strings.TrimPrefix(r.URL.Path, "/api/v3")
	path = strings.TrimPrefix(path, "/")

	parts := strings.Split(path, "/")

	if len(parts) >= 3 && parts[0] == "repos" {
		owner := parts[1]
		repo := parts[2]

		// Handle different endpoints
		if len(parts) == 3 {
			// GET /repos/{owner}/{repo}
			h.handleGetRepository(w, r, owner, repo)
			return
		}

		switch parts[3] {
		case "issues":
			h.handleIssues(w, r, owner, repo, parts[4:])
		case "pulls":
			h.handlePullRequests(w, r, owner, repo, parts[4:])
		case "contents":
			h.handleContents(w, r, owner, repo, parts[4:])
		case "git":
			h.handleGit(w, r, owner, repo, parts[4:])
		case "actions":
			h.handleActions(w, r, owner, repo, parts[4:])
		default:
			http.NotFound(w, r)
		}
		return
	}

	http.NotFound(w, r)
}

// Repository handlers

func (h *Handler) handleGetRepository(w http.ResponseWriter, r *http.Request, owner, repo string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := session.FromContext(r.Context())
	ctx := context.Background()

	// Try to get repository from database
	dbRepo, err := h.queries.GetGithubRepository(ctx, database.GetGithubRepositoryParams{
		Owner:     owner,
		Name:      repo,
		SessionID: sessionID,
	})

	if err != nil {
		// Repository doesn't exist, create it
		err = h.queries.CreateGithubRepository(ctx, database.CreateGithubRepositoryParams{
			Owner:         owner,
			Name:          repo,
			DefaultBranch: "main",
			Description:   sql.NullString{},
			SessionID:     sessionID,
		})
		if err != nil {
			log.Printf("[github] ✗ Failed to create repository: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Create main branch with initial commit
		initialSHA := generateSHA("initial commit")
		err = h.queries.CreateGithubBranch(ctx, database.CreateGithubBranchParams{
			RepoOwner: owner,
			RepoName:  repo,
			Name:      "main",
			Sha:       initialSHA,
			SessionID: sessionID,
		})
		if err != nil {
			log.Printf("[github] ✗ Failed to create main branch: %v", err)
		}

		// Get the newly created repository
		dbRepo, _ = h.queries.GetGithubRepository(ctx, database.GetGithubRepositoryParams{
			Owner:     owner,
			Name:      repo,
			SessionID: sessionID,
		})
	}

	// Convert to GitHub API format
	repository := &github.Repository{
		ID:            github.Ptr(dbRepo.ID),
		Owner:         &github.User{Login: github.Ptr(owner)},
		Name:          github.Ptr(repo),
		FullName:      github.Ptr(fmt.Sprintf("%s/%s", owner, repo)),
		DefaultBranch: github.Ptr(dbRepo.DefaultBranch),
		Private:       github.Ptr(false),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(repository)
	log.Printf("[github] ✓ Returned repository: %s/%s", owner, repo)
}

// Issues handlers

func (h *Handler) handleIssues(w http.ResponseWriter, r *http.Request, owner, repo string, parts []string) {
	sessionID := session.FromContext(r.Context())

	if len(parts) == 0 {
		// List or create issues
		switch r.Method {
		case http.MethodGet:
			h.handleListIssues(w, r, owner, repo, sessionID)
		case http.MethodPost:
			h.handleCreateIssue(w, r, owner, repo, sessionID)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// Handle specific issue
	issueNum, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "Invalid issue number", http.StatusBadRequest)
		return
	}

	if len(parts) == 1 {
		// GET or PATCH /repos/{owner}/{repo}/issues/{number}
		switch r.Method {
		case http.MethodGet:
			h.handleGetIssue(w, r, owner, repo, issueNum, sessionID)
		case http.MethodPatch:
			h.handleUpdateIssue(w, r, owner, repo, issueNum, sessionID)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	if len(parts) == 2 && parts[1] == "comments" {
		// POST /repos/{owner}/{repo}/issues/{number}/comments
		if r.Method == http.MethodPost {
			h.handleCreateIssueComment(w, r, owner, repo, issueNum, sessionID)
			return
		}
	}

	http.NotFound(w, r)
}

func (h *Handler) handleListIssues(w http.ResponseWriter, r *http.Request, owner, repo, sessionID string) {
	ctx := context.Background()
	state := r.URL.Query().Get("state")
	if state == "" {
		state = "open"
	}

	dbIssues, err := h.queries.ListGithubIssues(ctx, database.ListGithubIssuesParams{
		RepoOwner:   owner,
		RepoName:    repo,
		SessionID:   sessionID,
		StateFilter: state,
	})

	if err != nil {
		log.Printf("[github] ✗ Failed to list issues: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	issues := make([]*github.Issue, 0, len(dbIssues))
	for _, dbIssue := range dbIssues {
		issue := &github.Issue{
			ID:        github.Ptr(dbIssue.ID),
			Number:    github.Ptr(int(dbIssue.Number)),
			Title:     github.Ptr(dbIssue.Title),
			State:     github.Ptr(dbIssue.State),
			CreatedAt: github.Ptr(github.Timestamp{Time: time.Unix(dbIssue.CreatedAt, 0)}),
			UpdatedAt: github.Ptr(github.Timestamp{Time: time.Unix(dbIssue.UpdatedAt, 0)}),
		}
		if dbIssue.Body.Valid {
			issue.Body = github.Ptr(dbIssue.Body.String)
		}
		issues = append(issues, issue)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(issues)
	log.Printf("[github] ✓ Listed %d issues for %s/%s", len(issues), owner, repo)
}

func (h *Handler) handleCreateIssue(w http.ResponseWriter, r *http.Request, owner, repo, sessionID string) {
	ctx := context.Background()

	var req github.IssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get next issue number
	nextNum, err := h.queries.GetNextIssueNumber(ctx, database.GetNextIssueNumberParams{
		RepoOwner: owner,
		RepoName:  repo,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[github] ✗ Failed to get next issue number: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	body := sql.NullString{}
	if req.Body != nil {
		body = sql.NullString{String: *req.Body, Valid: true}
	}

	dbIssue, err := h.queries.CreateGithubIssue(ctx, database.CreateGithubIssueParams{
		RepoOwner: owner,
		RepoName:  repo,
		Number:    nextNum,
		Title:     *req.Title,
		Body:      body,
		State:     "open",
		SessionID: sessionID,
	})

	if err != nil {
		log.Printf("[github] ✗ Failed to create issue: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	issue := &github.Issue{
		ID:        github.Ptr(dbIssue.ID),
		Number:    github.Ptr(int(dbIssue.Number)),
		Title:     github.Ptr(dbIssue.Title),
		State:     github.Ptr(dbIssue.State),
		CreatedAt: github.Ptr(github.Timestamp{Time: time.Unix(dbIssue.CreatedAt, 0)}),
		UpdatedAt: github.Ptr(github.Timestamp{Time: time.Unix(dbIssue.UpdatedAt, 0)}),
	}
	if dbIssue.Body.Valid {
		issue.Body = github.Ptr(dbIssue.Body.String)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(issue)
	log.Printf("[github] ✓ Created issue #%d for %s/%s", dbIssue.Number, owner, repo)
}

func (h *Handler) handleGetIssue(w http.ResponseWriter, r *http.Request, owner, repo string, number int, sessionID string) {
	ctx := context.Background()

	dbIssue, err := h.queries.GetGithubIssue(ctx, database.GetGithubIssueParams{
		RepoOwner: owner,
		RepoName:  repo,
		Number:    int64(number),
		SessionID: sessionID,
	})

	if err != nil {
		http.NotFound(w, r)
		return
	}

	issue := &github.Issue{
		ID:        github.Ptr(dbIssue.ID),
		Number:    github.Ptr(int(dbIssue.Number)),
		Title:     github.Ptr(dbIssue.Title),
		State:     github.Ptr(dbIssue.State),
		CreatedAt: github.Ptr(github.Timestamp{Time: time.Unix(dbIssue.CreatedAt, 0)}),
		UpdatedAt: github.Ptr(github.Timestamp{Time: time.Unix(dbIssue.UpdatedAt, 0)}),
	}
	if dbIssue.Body.Valid {
		issue.Body = github.Ptr(dbIssue.Body.String)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(issue)
	log.Printf("[github] ✓ Returned issue #%d for %s/%s", number, owner, repo)
}

func (h *Handler) handleUpdateIssue(w http.ResponseWriter, r *http.Request, owner, repo string, number int, sessionID string) {
	ctx := context.Background()

	var req github.IssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get current issue
	dbIssue, err := h.queries.GetGithubIssue(ctx, database.GetGithubIssueParams{
		RepoOwner: owner,
		RepoName:  repo,
		Number:    int64(number),
		SessionID: sessionID,
	})

	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Update fields
	title := dbIssue.Title
	if req.Title != nil {
		title = *req.Title
	}

	body := dbIssue.Body
	if req.Body != nil {
		body = sql.NullString{String: *req.Body, Valid: true}
	}

	state := dbIssue.State
	if req.State != nil {
		state = *req.State
	}

	err = h.queries.UpdateGithubIssue(ctx, database.UpdateGithubIssueParams{
		Title:     title,
		Body:      body,
		State:     state,
		RepoOwner: owner,
		RepoName:  repo,
		Number:    int64(number),
		SessionID: sessionID,
	})

	if err != nil {
		log.Printf("[github] ✗ Failed to update issue: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return updated issue
	dbIssue, _ = h.queries.GetGithubIssue(ctx, database.GetGithubIssueParams{
		RepoOwner: owner,
		RepoName:  repo,
		Number:    int64(number),
		SessionID: sessionID,
	})

	issue := &github.Issue{
		ID:        github.Ptr(dbIssue.ID),
		Number:    github.Ptr(int(dbIssue.Number)),
		Title:     github.Ptr(dbIssue.Title),
		State:     github.Ptr(dbIssue.State),
		CreatedAt: github.Ptr(github.Timestamp{Time: time.Unix(dbIssue.CreatedAt, 0)}),
		UpdatedAt: github.Ptr(github.Timestamp{Time: time.Unix(dbIssue.UpdatedAt, 0)}),
	}
	if dbIssue.Body.Valid {
		issue.Body = github.Ptr(dbIssue.Body.String)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(issue)
	log.Printf("[github] ✓ Updated issue #%d for %s/%s", number, owner, repo)
}

func (h *Handler) handleCreateIssueComment(w http.ResponseWriter, r *http.Request, owner, repo string, number int, sessionID string) {
	ctx := context.Background()

	var req github.IssueComment
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get next comment ID
	nextID, err := h.queries.GetNextCommentID(ctx, database.GetNextCommentIDParams{
		RepoOwner: owner,
		RepoName:  repo,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[github] ✗ Failed to get next comment ID: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	dbComment, err := h.queries.CreateGithubIssueComment(ctx, database.CreateGithubIssueCommentParams{
		RepoOwner:   owner,
		RepoName:    repo,
		IssueNumber: int64(number),
		CommentID:   nextID,
		Body:        *req.Body,
		SessionID:   sessionID,
	})

	if err != nil {
		log.Printf("[github] ✗ Failed to create comment: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	comment := &github.IssueComment{
		ID:        github.Ptr(dbComment.CommentID),
		Body:      github.Ptr(dbComment.Body),
		CreatedAt: github.Ptr(github.Timestamp{Time: time.Unix(dbComment.CreatedAt, 0)}),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(comment)
	log.Printf("[github] ✓ Created comment on issue #%d for %s/%s", number, owner, repo)
}

// Pull Request handlers

func (h *Handler) handlePullRequests(w http.ResponseWriter, r *http.Request, owner, repo string, parts []string) {
	sessionID := session.FromContext(r.Context())

	if len(parts) == 0 {
		// List or create PRs
		switch r.Method {
		case http.MethodGet:
			h.handleListPullRequests(w, r, owner, repo, sessionID)
		case http.MethodPost:
			h.handleCreatePullRequest(w, r, owner, repo, sessionID)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// Handle specific PR
	prNum, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "Invalid PR number", http.StatusBadRequest)
		return
	}

	if len(parts) == 1 {
		// GET /repos/{owner}/{repo}/pulls/{number}
		if r.Method == http.MethodGet {
			h.handleGetPullRequest(w, r, owner, repo, prNum, sessionID)
			return
		}
	}

	if len(parts) == 2 && parts[1] == "merge" {
		// PUT /repos/{owner}/{repo}/pulls/{number}/merge
		if r.Method == http.MethodPut {
			h.handleMergePullRequest(w, r, owner, repo, prNum, sessionID)
			return
		}
	}

	if len(parts) == 2 && parts[1] == "comments" {
		// POST /repos/{owner}/{repo}/pulls/{number}/comments (same as issue comments)
		if r.Method == http.MethodPost {
			h.handleCreateIssueComment(w, r, owner, repo, prNum, sessionID)
			return
		}
	}

	http.NotFound(w, r)
}

func (h *Handler) handleListPullRequests(w http.ResponseWriter, r *http.Request, owner, repo, sessionID string) {
	ctx := context.Background()
	state := r.URL.Query().Get("state")
	if state == "" {
		state = "open"
	}

	dbPRs, err := h.queries.ListGithubPullRequests(ctx, database.ListGithubPullRequestsParams{
		RepoOwner:   owner,
		RepoName:    repo,
		SessionID:   sessionID,
		StateFilter: state,
	})

	if err != nil {
		log.Printf("[github] ✗ Failed to list PRs: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	prs := make([]*github.PullRequest, 0, len(dbPRs))
	for i := range dbPRs {
		pr := &github.PullRequest{
			ID:        github.Ptr(dbPRs[i].ID),
			Number:    github.Ptr(int(dbPRs[i].Number)),
			Title:     github.Ptr(dbPRs[i].Title),
			State:     github.Ptr(dbPRs[i].State),
			Head:      &github.PullRequestBranch{Ref: github.Ptr(dbPRs[i].Head)},
			Base:      &github.PullRequestBranch{Ref: github.Ptr(dbPRs[i].Base)},
			Merged:    github.Ptr(dbPRs[i].Merged == 1),
			CreatedAt: github.Ptr(github.Timestamp{Time: time.Unix(dbPRs[i].CreatedAt, 0)}),
			UpdatedAt: github.Ptr(github.Timestamp{Time: time.Unix(dbPRs[i].UpdatedAt, 0)}),
		}
		if dbPRs[i].Body.Valid {
			pr.Body = github.Ptr(dbPRs[i].Body.String)
		}
		prs = append(prs, pr)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(prs)
	log.Printf("[github] ✓ Listed %d PRs for %s/%s", len(prs), owner, repo)
}

func (h *Handler) handleCreatePullRequest(w http.ResponseWriter, r *http.Request, owner, repo, sessionID string) {
	ctx := context.Background()

	var req github.NewPullRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get next PR number
	nextNum, err := h.queries.GetNextPRNumber(ctx, database.GetNextPRNumberParams{
		RepoOwner: owner,
		RepoName:  repo,
		SessionID: sessionID,
	})
	if err != nil {
		log.Printf("[github] ✗ Failed to get next PR number: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	body := sql.NullString{}
	if req.Body != nil {
		body = sql.NullString{String: *req.Body, Valid: true}
	}

	dbPR, err := h.queries.CreateGithubPullRequest(ctx, database.CreateGithubPullRequestParams{
		RepoOwner: owner,
		RepoName:  repo,
		Number:    nextNum,
		Title:     *req.Title,
		Body:      body,
		Head:      *req.Head,
		Base:      *req.Base,
		State:     "open",
		SessionID: sessionID,
	})

	if err != nil {
		log.Printf("[github] ✗ Failed to create PR: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	pr := &github.PullRequest{
		ID:        github.Ptr(dbPR.ID),
		Number:    github.Ptr(int(dbPR.Number)),
		Title:     github.Ptr(dbPR.Title),
		State:     github.Ptr(dbPR.State),
		Head:      &github.PullRequestBranch{Ref: github.Ptr(dbPR.Head)},
		Base:      &github.PullRequestBranch{Ref: github.Ptr(dbPR.Base)},
		Merged:    github.Ptr(dbPR.Merged == 1),
		CreatedAt: github.Ptr(github.Timestamp{Time: time.Unix(dbPR.CreatedAt, 0)}),
		UpdatedAt: github.Ptr(github.Timestamp{Time: time.Unix(dbPR.UpdatedAt, 0)}),
	}
	if dbPR.Body.Valid {
		pr.Body = github.Ptr(dbPR.Body.String)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(pr)
	log.Printf("[github] ✓ Created PR #%d for %s/%s", dbPR.Number, owner, repo)
}

func (h *Handler) handleGetPullRequest(w http.ResponseWriter, r *http.Request, owner, repo string, number int, sessionID string) {
	ctx := context.Background()

	dbPR, err := h.queries.GetGithubPullRequest(ctx, database.GetGithubPullRequestParams{
		RepoOwner: owner,
		RepoName:  repo,
		Number:    int64(number),
		SessionID: sessionID,
	})

	if err != nil {
		http.NotFound(w, r)
		return
	}

	pr := &github.PullRequest{
		ID:        github.Ptr(dbPR.ID),
		Number:    github.Ptr(int(dbPR.Number)),
		Title:     github.Ptr(dbPR.Title),
		State:     github.Ptr(dbPR.State),
		Head:      &github.PullRequestBranch{Ref: github.Ptr(dbPR.Head)},
		Base:      &github.PullRequestBranch{Ref: github.Ptr(dbPR.Base)},
		Merged:    github.Ptr(dbPR.Merged == 1),
		CreatedAt: github.Ptr(github.Timestamp{Time: time.Unix(dbPR.CreatedAt, 0)}),
		UpdatedAt: github.Ptr(github.Timestamp{Time: time.Unix(dbPR.UpdatedAt, 0)}),
	}
	if dbPR.Body.Valid {
		pr.Body = github.Ptr(dbPR.Body.String)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(pr)
	log.Printf("[github] ✓ Returned PR #%d for %s/%s", number, owner, repo)
}

func (h *Handler) handleMergePullRequest(w http.ResponseWriter, _ *http.Request, owner, repo string, number int, sessionID string) {
	ctx := context.Background()

	err := h.queries.MergeGithubPullRequest(ctx, database.MergeGithubPullRequestParams{
		RepoOwner: owner,
		RepoName:  repo,
		Number:    int64(number),
		SessionID: sessionID,
	})

	if err != nil {
		log.Printf("[github] ✗ Failed to merge PR: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"sha":     generateSHA(fmt.Sprintf("merge-%d", number)),
		"merged":  true,
		"message": "Pull Request successfully merged",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	log.Printf("[github] ✓ Merged PR #%d for %s/%s", number, owner, repo)
}

// Git handlers (refs, files)

func (h *Handler) handleGit(w http.ResponseWriter, r *http.Request, owner, repo string, parts []string) {
	if len(parts) < 1 {
		http.NotFound(w, r)
		return
	}

	switch parts[0] {
	case "refs", "ref":
		// Support both /git/refs and /git/ref
		h.handleRefs(w, r, owner, repo, parts[1:])
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleRefs(w http.ResponseWriter, r *http.Request, owner, repo string, parts []string) {
	sessionID := session.FromContext(r.Context())
	ctx := context.Background()

	if len(parts) == 0 {
		// POST /git/refs (create new ref)
		if r.Method == http.MethodPost {
			var req github.CreateRef
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}

			// Extract branch name from ref
			refStr := req.Ref
			if !strings.HasPrefix(refStr, "refs/heads/") {
				http.Error(w, "Only branch refs supported", http.StatusBadRequest)
				return
			}
			newBranch := strings.TrimPrefix(refStr, "refs/heads/")

			err := h.queries.CreateGithubBranch(ctx, database.CreateGithubBranchParams{
				RepoOwner: owner,
				RepoName:  repo,
				Name:      newBranch,
				Sha:       req.SHA,
				SessionID: sessionID,
			})

			if err != nil {
				log.Printf("[github] ✗ Failed to create branch: %v", err)
				http.Error(w, "Failed to create branch", http.StatusInternalServerError)
				return
			}

			// Return a Reference response
			response := &github.Reference{
				Ref: &req.Ref,
				Object: &github.GitObject{
					SHA: &req.SHA,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(response)
			log.Printf("[github] ✓ Created branch %s in %s/%s", newBranch, owner, repo)
			return
		}
		http.NotFound(w, r)
		return
	}

	// Handle /git/refs/heads/{branch}
	if parts[0] == "heads" && len(parts) == 2 {
		branchName := parts[1]

		switch r.Method {
		case http.MethodGet:
			// GET /repos/{owner}/{repo}/git/refs/heads/{branch}
			dbBranch, err := h.queries.GetGithubBranch(ctx, database.GetGithubBranchParams{
				RepoOwner: owner,
				RepoName:  repo,
				Name:      branchName,
				SessionID: sessionID,
			})

			if err != nil {
				http.NotFound(w, r)
				return
			}

			ref := &github.Reference{
				Ref: github.Ptr("refs/heads/" + branchName),
				Object: &github.GitObject{
					SHA: github.Ptr(dbBranch.Sha),
				},
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(ref)
			log.Printf("[github] ✓ Returned ref for branch %s in %s/%s", branchName, owner, repo)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	http.NotFound(w, r)
}

// Contents handlers

func (h *Handler) handleContents(w http.ResponseWriter, r *http.Request, owner, repo string, parts []string) {
	if len(parts) == 0 {
		http.Error(w, "Path required", http.StatusBadRequest)
		return
	}

	sessionID := session.FromContext(r.Context())
	ctx := context.Background()
	path := strings.Join(parts, "/")
	branch := r.URL.Query().Get("ref")
	if branch == "" {
		branch = "main"
	}

	switch r.Method {
	case http.MethodGet:
		// GET /repos/{owner}/{repo}/contents/{path}
		dbFile, err := h.queries.GetGithubFile(ctx, database.GetGithubFileParams{
			RepoOwner: owner,
			RepoName:  repo,
			Path:      path,
			Branch:    branch,
			SessionID: sessionID,
		})

		if err != nil {
			http.NotFound(w, r)
			return
		}

		content := &github.RepositoryContent{
			Type:    github.Ptr("file"),
			Name:    github.Ptr(path),
			Path:    github.Ptr(path),
			SHA:     github.Ptr(dbFile.Sha),
			Content: github.Ptr(dbFile.Content),
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(content)
		log.Printf("[github] ✓ Returned file %s from %s/%s@%s", path, owner, repo, branch)

	case http.MethodPut:
		// PUT /repos/{owner}/{repo}/contents/{path} (create or update file)
		var req github.RepositoryContentFileOptions
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		fileBranch := branch
		if req.Branch != nil {
			fileBranch = *req.Branch
		}

		content := string(req.Content)
		sha := generateSHA(content)

		err := h.queries.CreateOrUpdateGithubFile(ctx, database.CreateOrUpdateGithubFileParams{
			RepoOwner: owner,
			RepoName:  repo,
			Path:      path,
			Content:   content,
			Sha:       sha,
			Branch:    fileBranch,
			SessionID: sessionID,
		})

		if err != nil {
			log.Printf("[github] ✗ Failed to create/update file: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		response := &github.RepositoryContentResponse{
			Content: &github.RepositoryContent{
				Name: github.Ptr(path),
				Path: github.Ptr(path),
				SHA:  github.Ptr(sha),
			},
			Commit: github.Commit{
				SHA: github.Ptr(sha),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(response)
		log.Printf("[github] ✓ Created/updated file %s in %s/%s@%s", path, owner, repo, fileBranch)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Actions handlers (workflows and runs)

func (h *Handler) handleActions(w http.ResponseWriter, r *http.Request, owner, repo string, parts []string) {
	if len(parts) < 1 {
		http.NotFound(w, r)
		return
	}

	switch parts[0] {
	case "workflows":
		h.handleWorkflows(w, r, owner, repo, parts[1:])
	case "runs":
		h.handleWorkflowRuns(w, r, owner, repo, parts[1:])
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleWorkflows(w http.ResponseWriter, r *http.Request, owner, repo string, parts []string) {
	sessionID := session.FromContext(r.Context())
	ctx := context.Background()

	if len(parts) == 0 {
		// GET /repos/{owner}/{repo}/actions/workflows
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		dbWorkflows, err := h.queries.ListGithubWorkflows(ctx, database.ListGithubWorkflowsParams{
			RepoOwner: owner,
			RepoName:  repo,
			SessionID: sessionID,
		})

		if err != nil {
			log.Printf("[github] ✗ Failed to list workflows: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		workflows := make([]*github.Workflow, 0, len(dbWorkflows))
		for _, dbWf := range dbWorkflows {
			wf := &github.Workflow{
				ID:        github.Ptr(dbWf.WorkflowID),
				Name:      github.Ptr(dbWf.Name),
				Path:      github.Ptr(dbWf.Path),
				State:     github.Ptr(dbWf.State),
				CreatedAt: github.Ptr(github.Timestamp{Time: time.Unix(dbWf.CreatedAt, 0)}),
			}
			workflows = append(workflows, wf)
		}

		response := &github.Workflows{
			TotalCount: github.Ptr(len(workflows)),
			Workflows:  workflows,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
		log.Printf("[github] ✓ Listed %d workflows for %s/%s", len(workflows), owner, repo)
		return
	}

	// Handle specific workflow by ID
	workflowID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "Invalid workflow ID", http.StatusBadRequest)
		return
	}

	if len(parts) == 1 {
		// GET /repos/{owner}/{repo}/actions/workflows/{id}
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		dbWf, err := h.queries.GetGithubWorkflow(ctx, database.GetGithubWorkflowParams{
			RepoOwner:  owner,
			RepoName:   repo,
			WorkflowID: workflowID,
			SessionID:  sessionID,
		})

		if err != nil {
			http.NotFound(w, r)
			return
		}

		wf := &github.Workflow{
			ID:        github.Ptr(dbWf.WorkflowID),
			Name:      github.Ptr(dbWf.Name),
			Path:      github.Ptr(dbWf.Path),
			State:     github.Ptr(dbWf.State),
			CreatedAt: github.Ptr(github.Timestamp{Time: time.Unix(dbWf.CreatedAt, 0)}),
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(wf)
		log.Printf("[github] ✓ Returned workflow %d for %s/%s", workflowID, owner, repo)
		return
	}

	if len(parts) == 2 && parts[1] == "dispatches" {
		// POST /repos/{owner}/{repo}/actions/workflows/{id}/dispatches
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req github.CreateWorkflowDispatchEventRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Get next run ID
		nextRunID, err := h.queries.GetNextWorkflowRunID(ctx, database.GetNextWorkflowRunIDParams{
			RepoOwner: owner,
			RepoName:  repo,
			SessionID: sessionID,
		})
		if err != nil {
			log.Printf("[github] ✗ Failed to get next run ID: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Create workflow run
		_, err = h.queries.CreateGithubWorkflowRun(ctx, database.CreateGithubWorkflowRunParams{
			RepoOwner:  owner,
			RepoName:   repo,
			RunID:      nextRunID,
			WorkflowID: workflowID,
			Status:     "completed",
			Conclusion: sql.NullString{String: "success", Valid: true},
			HeadBranch: sql.NullString{String: req.Ref, Valid: true},
			HeadSha:    sql.NullString{String: generateSHA(req.Ref), Valid: true},
			SessionID:  sessionID,
		})

		if err != nil {
			log.Printf("[github] ✗ Failed to create workflow run: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
		log.Printf("[github] ✓ Triggered workflow %d for %s/%s", workflowID, owner, repo)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) handleWorkflowRuns(w http.ResponseWriter, r *http.Request, owner, repo string, parts []string) {
	sessionID := session.FromContext(r.Context())
	ctx := context.Background()

	if len(parts) == 0 {
		// GET /repos/{owner}/{repo}/actions/runs
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		dbRuns, err := h.queries.ListGithubWorkflowRuns(ctx, database.ListGithubWorkflowRunsParams{
			RepoOwner: owner,
			RepoName:  repo,
			SessionID: sessionID,
		})

		if err != nil {
			log.Printf("[github] ✗ Failed to list workflow runs: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		runs := make([]*github.WorkflowRun, 0, len(dbRuns))
		for i := range dbRuns {
			run := &github.WorkflowRun{
				ID:         github.Ptr(dbRuns[i].RunID),
				WorkflowID: github.Ptr(dbRuns[i].WorkflowID),
				Status:     github.Ptr(dbRuns[i].Status),
				CreatedAt:  github.Ptr(github.Timestamp{Time: time.Unix(dbRuns[i].CreatedAt, 0)}),
				UpdatedAt:  github.Ptr(github.Timestamp{Time: time.Unix(dbRuns[i].UpdatedAt, 0)}),
			}
			if dbRuns[i].Conclusion.Valid {
				run.Conclusion = github.Ptr(dbRuns[i].Conclusion.String)
			}
			if dbRuns[i].HeadBranch.Valid {
				run.HeadBranch = github.Ptr(dbRuns[i].HeadBranch.String)
			}
			if dbRuns[i].HeadSha.Valid {
				run.HeadSHA = github.Ptr(dbRuns[i].HeadSha.String)
			}
			runs = append(runs, run)
		}

		response := &github.WorkflowRuns{
			TotalCount:   github.Ptr(len(runs)),
			WorkflowRuns: runs,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
		log.Printf("[github] ✓ Listed %d workflow runs for %s/%s", len(runs), owner, repo)
		return
	}

	// Handle specific run by ID
	runID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "Invalid run ID", http.StatusBadRequest)
		return
	}

	if len(parts) == 1 {
		// GET /repos/{owner}/{repo}/actions/runs/{id}
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		dbRun, err := h.queries.GetGithubWorkflowRun(ctx, database.GetGithubWorkflowRunParams{
			RepoOwner: owner,
			RepoName:  repo,
			RunID:     runID,
			SessionID: sessionID,
		})

		if err != nil {
			http.NotFound(w, r)
			return
		}

		run := &github.WorkflowRun{
			ID:         github.Ptr(dbRun.RunID),
			WorkflowID: github.Ptr(dbRun.WorkflowID),
			Status:     github.Ptr(dbRun.Status),
			CreatedAt:  github.Ptr(github.Timestamp{Time: time.Unix(dbRun.CreatedAt, 0)}),
			UpdatedAt:  github.Ptr(github.Timestamp{Time: time.Unix(dbRun.UpdatedAt, 0)}),
		}
		if dbRun.Conclusion.Valid {
			run.Conclusion = github.Ptr(dbRun.Conclusion.String)
		}
		if dbRun.HeadBranch.Valid {
			run.HeadBranch = github.Ptr(dbRun.HeadBranch.String)
		}
		if dbRun.HeadSha.Valid {
			run.HeadSHA = github.Ptr(dbRun.HeadSha.String)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(run)
		log.Printf("[github] ✓ Returned workflow run %d for %s/%s", runID, owner, repo)
		return
	}

	http.NotFound(w, r)
}

// Helper functions

func generateSHA(content string) string {
	hash := sha1.Sum([]byte(content)) //nolint:gosec // Used for generating fake SHAs in simulator, not for security
	return fmt.Sprintf("%x", hash)
}
