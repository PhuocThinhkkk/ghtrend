package ghclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultCommitsPerPage = 30

// CommitSummary represents a repository commit entry in a commit history list.
type CommitSummary struct {
	SHA         string    `json:"sha"`
	ShortSHA    string    `json:"short_sha"`
	Message     string    `json:"message"`
	Body        string    `json:"body"`
	Author      string    `json:"author"`
	AuthorLogin string    `json:"author_login"`
	Date        time.Time `json:"date"`
	HTMLURL     string    `json:"html_url"`
}

// CommitFileDiff represents a single file change within a commit diff.
type CommitFileDiff struct {
	Filename         string `json:"filename"`
	Status           string `json:"status"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	Changes          int    `json:"changes"`
	Patch            string `json:"patch"`
	PreviousFilename string `json:"previous_filename"`
}

// CommitDiff represents the full diff and metadata for a single commit.
type CommitDiff struct {
	SHA          string           `json:"sha"`
	ShortSHA     string           `json:"short_sha"`
	Message      string           `json:"message"`
	Author       string           `json:"author"`
	AuthorLogin  string           `json:"author_login"`
	Date         time.Time        `json:"date"`
	HTMLURL      string           `json:"html_url"`
	Additions    int              `json:"additions"`
	Deletions    int              `json:"deletions"`
	ChangedFiles int              `json:"changed_files"`
	Files        []CommitFileDiff `json:"files"`
}

type restCommitPerson struct {
	Name string    `json:"name"`
	Date time.Time `json:"date"`
}

type restCommitDetails struct {
	Message   string           `json:"message"`
	Author    restCommitPerson `json:"author"`
	Committer restCommitPerson `json:"committer"`
}

type restCommitListItem struct {
	SHA     string            `json:"sha"`
	HTMLURL string            `json:"html_url"`
	Commit  restCommitDetails `json:"commit"`
	Author  *restUser         `json:"author"`
}

type restCommitStats struct {
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
}

type restCommitFile struct {
	Filename         string `json:"filename"`
	Status           string `json:"status"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	Changes          int    `json:"changes"`
	Patch            string `json:"patch"`
	PreviousFilename string `json:"previous_filename"`
}

type restCommitDetailResponse struct {
	SHA     string            `json:"sha"`
	HTMLURL string            `json:"html_url"`
	Commit  restCommitDetails `json:"commit"`
	Author  *restUser         `json:"author"`
	Stats   restCommitStats   `json:"stats"`
	Files   []restCommitFile  `json:"files"`
}

// FetchRepoCommits fetches a paginated list of commits for owner/repo, optionally filtered by ref.
// It returns the commits, whether a next page exists, and any error.
func FetchRepoCommits(ctx context.Context, owner, repo, ref string, page int) ([]CommitSummary, bool, error) {
	if err := validateOwnerRepo(owner, repo); err != nil {
		return nil, false, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if page <= 0 {
		page = 1
	}

	apiPath := fmt.Sprintf(
		"/repos/%s/%s/commits?per_page=%d&page=%d",
		url.PathEscape(owner),
		url.PathEscape(repo),
		defaultCommitsPerPage,
		page,
	)
	if trimmedRef := strings.TrimSpace(ref); trimmedRef != "" {
		apiPath += "&sha=" + url.QueryEscape(trimmedRef)
	}

	var raw []restCommitListItem
	headers, err := activeAPIClient().DoREST(ctx, http.MethodGet, apiPath, nil, "", &raw)
	if err != nil {
		return nil, false, err
	}

	commits := make([]CommitSummary, 0, len(raw))
	for _, item := range raw {
		authorLogin := ""
		if item.Author != nil {
			authorLogin = item.Author.Login
		}
		authorName := item.Commit.Author.Name
		if authorName == "" {
			authorName = authorLogin
		}
		commitDate := item.Commit.Author.Date
		if commitDate.IsZero() {
			commitDate = item.Commit.Committer.Date
		}

		commits = append(commits, CommitSummary{
			SHA:         item.SHA,
			ShortSHA:    shortSHA(item.SHA),
			Message:     firstLine(item.Commit.Message),
			Body:        item.Commit.Message,
			Author:      authorName,
			AuthorLogin: authorLogin,
			Date:        commitDate,
			HTMLURL:     item.HTMLURL,
		})
	}

	return commits, HasNextPage(headers.Get("Link")), nil
}

// FetchCommitDiff fetches the metadata and file diffs for a single commit SHA in owner/repo.
func FetchCommitDiff(ctx context.Context, owner, repo, sha string) (CommitDiff, error) {
	if err := validateOwnerRepo(owner, repo); err != nil {
		return CommitDiff{}, err
	}
	if err := validateCommitSHA(sha); err != nil {
		return CommitDiff{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	apiPath := fmt.Sprintf(
		"/repos/%s/%s/commits/%s",
		url.PathEscape(owner),
		url.PathEscape(repo),
		url.PathEscape(sha),
	)

	var raw restCommitDetailResponse
	if _, err := activeAPIClient().DoREST(ctx, http.MethodGet, apiPath, nil, "", &raw); err != nil {
		return CommitDiff{}, err
	}

	authorLogin := ""
	if raw.Author != nil {
		authorLogin = raw.Author.Login
	}
	authorName := raw.Commit.Author.Name
	if authorName == "" {
		authorName = authorLogin
	}
	commitDate := raw.Commit.Author.Date
	if commitDate.IsZero() {
		commitDate = raw.Commit.Committer.Date
	}

	files := make([]CommitFileDiff, 0, len(raw.Files))
	for _, f := range raw.Files {
		files = append(files, CommitFileDiff{
			Filename:         f.Filename,
			Status:           f.Status,
			Additions:        f.Additions,
			Deletions:        f.Deletions,
			Changes:          f.Changes,
			Patch:            f.Patch,
			PreviousFilename: f.PreviousFilename,
		})
	}

	return CommitDiff{
		SHA:          raw.SHA,
		ShortSHA:     shortSHA(raw.SHA),
		Message:      raw.Commit.Message,
		Author:       authorName,
		AuthorLogin:  authorLogin,
		Date:         commitDate,
		HTMLURL:      raw.HTMLURL,
		Additions:    raw.Stats.Additions,
		Deletions:    raw.Stats.Deletions,
		ChangedFiles: len(files),
		Files:        files,
	}, nil
}

func validateCommitSHA(sha string) error {
	if strings.TrimSpace(sha) == "" {
		return errors.New("commit sha must not be empty")
	}
	if strings.ContainsAny(sha, " \t\r\n/\\") || strings.Contains(sha, "..") || sha == "." {
		return errors.New("invalid commit sha")
	}
	return nil
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return strings.TrimRight(line, "\r")
}
