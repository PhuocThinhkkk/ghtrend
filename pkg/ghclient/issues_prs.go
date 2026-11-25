package ghclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

const defaultIssuesPRsPerPage = 30

// Label represents a GitHub issue or pull request label.
type Label struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// PullRequest represents a GitHub pull request summary and status metadata.
type PullRequest struct {
	Number         int       `json:"number"`
	Title          string    `json:"title"`
	Body           string    `json:"body"`
	State          string    `json:"state"`
	Author         string    `json:"author"`
	HeadRef        string    `json:"head_ref"`
	BaseRef        string    `json:"base_ref"`
	HTMLURL        string    `json:"html_url"`
	ReviewDecision string    `json:"review_decision"`
	CheckState     string    `json:"check_state"`
	Draft          bool      `json:"draft"`
	Merged         bool      `json:"merged"`
	CommentCount   int       `json:"comment_count"`
	Additions      int       `json:"additions"`
	Deletions      int       `json:"deletions"`
	ChangedFiles   int       `json:"changed_files"`
	Labels         []Label   `json:"labels"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Issue represents a GitHub issue.
type Issue struct {
	Number       int       `json:"number"`
	Title        string    `json:"title"`
	Body         string    `json:"body"`
	State        string    `json:"state"`
	StateReason  string    `json:"state_reason"`
	Author       string    `json:"author"`
	HTMLURL      string    `json:"html_url"`
	Labels       []Label   `json:"labels"`
	Assignees    []string  `json:"assignees"`
	CommentCount int       `json:"comment_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TimelineComment represents a comment or review entry in an issue or pull request timeline.
type TimelineComment struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	Role      string    `json:"role"`
	Kind      string    `json:"kind"` // "issue_comment", "review", "review_comment"
	State     string    `json:"state"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

type restLabel struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

type restUser struct {
	Login string `json:"login"`
}

type restIssueItem struct {
	Number      int             `json:"number"`
	Title       string          `json:"title"`
	Body        string          `json:"body"`
	State       string          `json:"state"`
	StateReason string          `json:"state_reason"`
	HTMLURL     string          `json:"html_url"`
	Comments    int             `json:"comments"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	User        *restUser       `json:"user"`
	Labels      []restLabel     `json:"labels"`
	Assignees   []restUser      `json:"assignees"`
	PullRequest json.RawMessage `json:"pull_request"`
}

type restIssueComment struct {
	ID                int64     `json:"id"`
	Body              string    `json:"body"`
	AuthorAssociation string    `json:"author_association"`
	CreatedAt         time.Time `json:"created_at"`
	User              *restUser `json:"user"`
}

// FetchPullRequests lists pull requests for a repository, using GraphQL on page 1
// for rich status/diff metadata and falling back to REST on non-404 errors or page > 1.
func FetchPullRequests(ctx context.Context, owner, repo, state string, page int) ([]PullRequest, bool, error) {
	if err := validateOwnerRepo(owner, repo); err != nil {
		return nil, false, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	normState := normalizeListState(state)
	normPage := normalizePage(page)

	if normPage <= 1 {
		prs, hasMore, err := fetchPullRequestsGraphQL(ctx, owner, repo, normState)
		if err == nil {
			return prs, hasMore, nil
		}
		if errors.Is(err, ErrNotFound) {
			return nil, false, err
		}
	}

	return fetchPullRequestsREST(ctx, owner, repo, normState, normPage)
}

func fetchPullRequestsGraphQL(ctx context.Context, owner, repo, state string) ([]PullRequest, bool, error) {
	const query = `query($owner: String!, $repo: String!, $states: [PullRequestState!]) {
  repository(owner: $owner, name: $repo) {
    pullRequests(first: 30, states: $states, orderBy: {field: CREATED_AT, direction: DESC}) {
      nodes {
        number
        title
        body
        state
        isDraft
        merged
        url
        headRefName
        baseRefName
        reviewDecision
        additions
        deletions
        changedFiles
        createdAt
        updatedAt
        author {
          login
        }
        comments {
          totalCount
        }
        labels(first: 20) {
          nodes {
            name
            color
            description
          }
        }
        commits(last: 1) {
          nodes {
            commit {
              statusCheckRollup {
                state
              }
            }
          }
        }
      }
      pageInfo {
        hasNextPage
      }
    }
  }
}`

	gqlStates := graphQLPullRequestStates(state)
	vars := map[string]any{
		"owner":  owner,
		"repo":   repo,
		"states": gqlStates,
	}

	var resp struct {
		Repository *struct {
			PullRequests struct {
				Nodes []struct {
					Number         int       `json:"number"`
					Title          string    `json:"title"`
					Body           string    `json:"body"`
					State          string    `json:"state"`
					IsDraft        bool      `json:"isDraft"`
					Merged         bool      `json:"merged"`
					URL            string    `json:"url"`
					HeadRefName    string    `json:"headRefName"`
					BaseRefName    string    `json:"baseRefName"`
					ReviewDecision string    `json:"reviewDecision"`
					Additions      int       `json:"additions"`
					Deletions      int       `json:"deletions"`
					ChangedFiles   int       `json:"changedFiles"`
					CreatedAt      time.Time `json:"createdAt"`
					UpdatedAt      time.Time `json:"updatedAt"`
					Author         *struct {
						Login string `json:"login"`
					} `json:"author"`
					Comments struct {
						TotalCount int `json:"totalCount"`
					} `json:"comments"`
					Labels struct {
						Nodes []restLabel `json:"nodes"`
					} `json:"labels"`
					Commits struct {
						Nodes []struct {
							Commit struct {
								StatusCheckRollup *struct {
									State string `json:"state"`
								} `json:"statusCheckRollup"`
							} `json:"commit"`
						} `json:"nodes"`
					} `json:"commits"`
				} `json:"nodes"`
				PageInfo struct {
					HasNextPage bool `json:"hasNextPage"`
				} `json:"pageInfo"`
			} `json:"pullRequests"`
		} `json:"repository"`
	}

	if err := activeAPIClient().DoGraphQL(ctx, query, vars, &resp); err != nil {
		return nil, false, err
	}
	if resp.Repository == nil {
		return nil, false, &APIError{
			StatusCode: http.StatusNotFound,
			Resource:   "graphql",
			Message:    "repository not found",
			Err:        ErrNotFound,
		}
	}

	nodes := resp.Repository.PullRequests.Nodes
	prs := make([]PullRequest, 0, len(nodes))
	for _, n := range nodes {
		author := ""
		if n.Author != nil {
			author = n.Author.Login
		}

		checkState := ""
		if len(n.Commits.Nodes) > 0 && n.Commits.Nodes[0].Commit.StatusCheckRollup != nil {
			checkState = n.Commits.Nodes[0].Commit.StatusCheckRollup.State
		}

		prState := strings.ToLower(strings.TrimSpace(n.State))
		if prState == "merged" {
			prState = "closed"
		}

		prs = append(prs, PullRequest{
			Number:         n.Number,
			Title:          n.Title,
			Body:           n.Body,
			State:          prState,
			Author:         author,
			HeadRef:        n.HeadRefName,
			BaseRef:        n.BaseRefName,
			HTMLURL:        n.URL,
			ReviewDecision: n.ReviewDecision,
			CheckState:     checkState,
			Draft:          n.IsDraft,
			Merged:         n.Merged,
			CommentCount:   n.Comments.TotalCount,
			Additions:      n.Additions,
			Deletions:      n.Deletions,
			ChangedFiles:   n.ChangedFiles,
			Labels:         convertLabels(n.Labels.Nodes),
			CreatedAt:      n.CreatedAt,
			UpdatedAt:      n.UpdatedAt,
		})
	}

	return prs, resp.Repository.PullRequests.PageInfo.HasNextPage, nil
}

func fetchPullRequestsREST(ctx context.Context, owner, repo, state string, page int) ([]PullRequest, bool, error) {
	apiPath := fmt.Sprintf(
		"/repos/%s/%s/pulls?state=%s&per_page=%d&page=%d",
		url.PathEscape(owner),
		url.PathEscape(repo),
		url.QueryEscape(state),
		defaultIssuesPRsPerPage,
		page,
	)

	var raw []struct {
		Number       int         `json:"number"`
		Title        string      `json:"title"`
		Body         string      `json:"body"`
		State        string      `json:"state"`
		Draft        bool        `json:"draft"`
		HTMLURL      string      `json:"html_url"`
		MergedAt     *time.Time  `json:"merged_at"`
		Comments     int         `json:"comments"`
		Additions    int         `json:"additions"`
		Deletions    int         `json:"deletions"`
		ChangedFiles int         `json:"changed_files"`
		CreatedAt    time.Time   `json:"created_at"`
		UpdatedAt    time.Time   `json:"updated_at"`
		User         *restUser   `json:"user"`
		Labels       []restLabel `json:"labels"`
		Head         struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	}

	headers, err := activeAPIClient().DoREST(ctx, http.MethodGet, apiPath, nil, "", &raw)
	if err != nil {
		return nil, false, err
	}

	prs := make([]PullRequest, 0, len(raw))
	for _, item := range raw {
		author := ""
		if item.User != nil {
			author = item.User.Login
		}
		prs = append(prs, PullRequest{
			Number:       item.Number,
			Title:        item.Title,
			Body:         item.Body,
			State:        strings.ToLower(strings.TrimSpace(item.State)),
			Author:       author,
			HeadRef:      item.Head.Ref,
			BaseRef:      item.Base.Ref,
			HTMLURL:      item.HTMLURL,
			Draft:        item.Draft,
			Merged:       item.MergedAt != nil,
			CommentCount: item.Comments,
			Additions:    item.Additions,
			Deletions:    item.Deletions,
			ChangedFiles: item.ChangedFiles,
			Labels:       convertLabels(item.Labels),
			CreatedAt:    item.CreatedAt,
			UpdatedAt:    item.UpdatedAt,
		})
	}

	return prs, HasNextPage(headers.Get("Link")), nil
}

// FetchPullRequestTimeline fetches issue comments, PR reviews, and PR review comments
// for a pull request, filters out empty non-status entries, and sorts chronologically.
func FetchPullRequestTimeline(ctx context.Context, owner, repo string, number int) ([]TimelineComment, error) {
	if err := validateOwnerRepo(owner, repo); err != nil {
		return nil, err
	}
	if number <= 0 {
		return nil, errors.New("pull request number must be positive")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	issueComments, err := fetchIssueCommentsREST(ctx, owner, repo, number, true)
	if err != nil {
		return nil, err
	}

	escapedOwner := url.PathEscape(owner)
	escapedRepo := url.PathEscape(repo)
	client := activeAPIClient()

	var rawReviews []struct {
		ID                int64     `json:"id"`
		Body              string    `json:"body"`
		State             string    `json:"state"`
		AuthorAssociation string    `json:"author_association"`
		SubmittedAt       time.Time `json:"submitted_at"`
		CreatedAt         time.Time `json:"created_at"`
		User              *restUser `json:"user"`
	}
	reviewsPath := fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews?per_page=100", escapedOwner, escapedRepo, number)
	if _, err := client.DoREST(ctx, http.MethodGet, reviewsPath, nil, "", &rawReviews); err != nil {
		return nil, err
	}

	var rawReviewComments []struct {
		ID                int64     `json:"id"`
		Body              string    `json:"body"`
		Path              string    `json:"path"`
		AuthorAssociation string    `json:"author_association"`
		CreatedAt         time.Time `json:"created_at"`
		User              *restUser `json:"user"`
	}
	reviewCommentsPath := fmt.Sprintf("/repos/%s/%s/pulls/%d/comments?per_page=100", escapedOwner, escapedRepo, number)
	if _, err := client.DoREST(ctx, http.MethodGet, reviewCommentsPath, nil, "", &rawReviewComments); err != nil {
		return nil, err
	}

	timeline := make([]TimelineComment, 0, len(issueComments)+len(rawReviews)+len(rawReviewComments))
	timeline = append(timeline, issueComments...)

	for _, r := range rawReviews {
		state := strings.ToUpper(strings.TrimSpace(r.State))
		if strings.TrimSpace(r.Body) == "" && state != "APPROVED" && state != "CHANGES_REQUESTED" {
			continue
		}
		author := ""
		if r.User != nil {
			author = r.User.Login
		}
		createdAt := r.SubmittedAt
		if createdAt.IsZero() {
			createdAt = r.CreatedAt
		}
		timeline = append(timeline, TimelineComment{
			ID:        strconv.FormatInt(r.ID, 10),
			Author:    author,
			Body:      r.Body,
			Role:      r.AuthorAssociation,
			Kind:      "review",
			State:     state,
			CreatedAt: createdAt,
		})
	}

	for _, rc := range rawReviewComments {
		if strings.TrimSpace(rc.Body) == "" {
			continue
		}
		author := ""
		if rc.User != nil {
			author = rc.User.Login
		}
		timeline = append(timeline, TimelineComment{
			ID:        strconv.FormatInt(rc.ID, 10),
			Author:    author,
			Body:      rc.Body,
			Role:      rc.AuthorAssociation,
			Kind:      "review_comment",
			Path:      rc.Path,
			CreatedAt: rc.CreatedAt,
		})
	}

	slices.SortStableFunc(timeline, func(a, b TimelineComment) int {
		return a.CreatedAt.Compare(b.CreatedAt)
	})

	return timeline, nil
}

// FetchIssues lists repository issues, excluding any pull request entries returned
// by the GitHub /issues endpoint.
func FetchIssues(ctx context.Context, owner, repo, state string, page int) ([]Issue, bool, error) {
	if err := validateOwnerRepo(owner, repo); err != nil {
		return nil, false, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	normState := normalizeListState(state)
	normPage := normalizePage(page)

	apiPath := fmt.Sprintf(
		"/repos/%s/%s/issues?state=%s&per_page=%d&page=%d",
		url.PathEscape(owner),
		url.PathEscape(repo),
		url.QueryEscape(normState),
		defaultIssuesPRsPerPage,
		normPage,
	)

	var raw []restIssueItem
	headers, err := activeAPIClient().DoREST(ctx, http.MethodGet, apiPath, nil, "", &raw)
	if err != nil {
		return nil, false, err
	}

	issues := make([]Issue, 0, len(raw))
	for _, item := range raw {
		if isNonNullJSON(item.PullRequest) {
			continue
		}
		issues = append(issues, convertIssue(item))
	}

	return issues, HasNextPage(headers.Get("Link")), nil
}

// FetchIssueTimeline fetches issue comments for a given issue number, sorted chronologically.
func FetchIssueTimeline(ctx context.Context, owner, repo string, number int) ([]TimelineComment, error) {
	if err := validateOwnerRepo(owner, repo); err != nil {
		return nil, err
	}
	if number <= 0 {
		return nil, errors.New("issue number must be positive")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	comments, err := fetchIssueCommentsREST(ctx, owner, repo, number, false)
	if err != nil {
		return nil, err
	}

	slices.SortStableFunc(comments, func(a, b TimelineComment) int {
		return a.CreatedAt.Compare(b.CreatedAt)
	})

	return comments, nil
}

// CreateIssueComment posts a new comment on an issue or pull request.
func CreateIssueComment(ctx context.Context, owner, repo string, number int, body string) (TimelineComment, error) {
	if err := validateOwnerRepo(owner, repo); err != nil {
		return TimelineComment{}, err
	}
	if number <= 0 {
		return TimelineComment{}, errors.New("issue number must be positive")
	}
	if strings.TrimSpace(body) == "" {
		return TimelineComment{}, errors.New("comment body must not be empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	apiPath := fmt.Sprintf(
		"/repos/%s/%s/issues/%d/comments",
		url.PathEscape(owner),
		url.PathEscape(repo),
		number,
	)

	reqBody := struct {
		Body string `json:"body"`
	}{
		Body: body,
	}

	var raw restIssueComment
	if _, err := activeAPIClient().DoREST(ctx, http.MethodPost, apiPath, reqBody, "", &raw); err != nil {
		return TimelineComment{}, err
	}

	return convertIssueComment(raw), nil
}

// UpdateIssueState updates an issue's state to "open" or "closed".
func UpdateIssueState(ctx context.Context, owner, repo string, number int, state string) (Issue, error) {
	if err := validateOwnerRepo(owner, repo); err != nil {
		return Issue{}, err
	}
	if number <= 0 {
		return Issue{}, errors.New("issue number must be positive")
	}

	normState := strings.ToLower(strings.TrimSpace(state))
	if normState != "open" && normState != "closed" {
		return Issue{}, fmt.Errorf("invalid issue state %q: must be open or closed", state)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	apiPath := fmt.Sprintf(
		"/repos/%s/%s/issues/%d",
		url.PathEscape(owner),
		url.PathEscape(repo),
		number,
	)

	reqBody := struct {
		State string `json:"state"`
	}{
		State: normState,
	}

	var raw restIssueItem
	if _, err := activeAPIClient().DoREST(ctx, http.MethodPatch, apiPath, reqBody, "", &raw); err != nil {
		return Issue{}, err
	}

	return convertIssue(raw), nil
}

func fetchIssueCommentsREST(ctx context.Context, owner, repo string, number int, skipEmpty bool) ([]TimelineComment, error) {
	apiPath := fmt.Sprintf(
		"/repos/%s/%s/issues/%d/comments?per_page=100",
		url.PathEscape(owner),
		url.PathEscape(repo),
		number,
	)

	var raw []restIssueComment
	if _, err := activeAPIClient().DoREST(ctx, http.MethodGet, apiPath, nil, "", &raw); err != nil {
		return nil, err
	}

	comments := make([]TimelineComment, 0, len(raw))
	for _, item := range raw {
		if skipEmpty && strings.TrimSpace(item.Body) == "" {
			continue
		}
		comments = append(comments, convertIssueComment(item))
	}

	return comments, nil
}

func convertIssueComment(item restIssueComment) TimelineComment {
	author := ""
	if item.User != nil {
		author = item.User.Login
	}
	return TimelineComment{
		ID:        strconv.FormatInt(item.ID, 10),
		Author:    author,
		Body:      item.Body,
		Role:      item.AuthorAssociation,
		Kind:      "issue_comment",
		CreatedAt: item.CreatedAt,
	}
}

func convertIssue(item restIssueItem) Issue {
	author := ""
	if item.User != nil {
		author = item.User.Login
	}
	assignees := make([]string, 0, len(item.Assignees))
	for _, a := range item.Assignees {
		if a.Login != "" {
			assignees = append(assignees, a.Login)
		}
	}
	return Issue{
		Number:       item.Number,
		Title:        item.Title,
		Body:         item.Body,
		State:        strings.ToLower(strings.TrimSpace(item.State)),
		StateReason:  item.StateReason,
		Author:       author,
		HTMLURL:      item.HTMLURL,
		Labels:       convertLabels(item.Labels),
		Assignees:    assignees,
		CommentCount: item.Comments,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

func convertLabels(raw []restLabel) []Label {
	labels := make([]Label, 0, len(raw))
	for _, l := range raw {
		labels = append(labels, Label{
			Name:        l.Name,
			Color:       l.Color,
			Description: l.Description,
		})
	}
	return labels
}

func normalizeListState(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "open", "closed", "all":
		return strings.ToLower(strings.TrimSpace(state))
	default:
		return "open"
	}
}

func normalizePage(page int) int {
	if page <= 0 {
		return 1
	}
	return page
}

func graphQLPullRequestStates(state string) []string {
	switch state {
	case "closed":
		return []string{"CLOSED", "MERGED"}
	case "all":
		return []string{"OPEN", "CLOSED", "MERGED"}
	default:
		return []string{"OPEN"}
	}
}

func isNonNullJSON(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}
