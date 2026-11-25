package ghclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const defaultSearchPerPage = 15

// SearchRepoResult represents a repository returned by GitHub repository search.
type SearchRepoResult struct {
	FullName    string `json:"full_name"`
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Language    string `json:"language"`
	Stars       int    `json:"stars"`
	Forks       int    `json:"forks"`
	HTMLURL     string `json:"html_url"`
}

// SearchUserResult represents a user or organization returned by GitHub user search.
type SearchUserResult struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
	Type      string `json:"type"`
}

type restSearchRepoItem struct {
	FullName    string    `json:"full_name"`
	Name        string    `json:"name"`
	Owner       *restUser `json:"owner"`
	Description string    `json:"description"`
	Language    string    `json:"language"`
	Stars       int       `json:"stargazers_count"`
	Forks       int       `json:"forks_count"`
	HTMLURL     string    `json:"html_url"`
}

type restSearchReposResponse struct {
	TotalCount int                  `json:"total_count"`
	Items      []restSearchRepoItem `json:"items"`
}

type restSearchUserItem struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
	Type      string `json:"type"`
}

type restSearchUsersResponse struct {
	TotalCount int                  `json:"total_count"`
	Items      []restSearchUserItem `json:"items"`
}

// SearchRepositories searches GitHub repositories matching query for the given 1-based page.
// It returns matching repositories, whether a next page exists, and any error.
func SearchRepositories(ctx context.Context, query string, page int) ([]SearchRepoResult, bool, error) {
	if strings.TrimSpace(query) == "" {
		return nil, false, errors.New("search query must not be empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if page <= 0 {
		page = 1
	}

	apiPath := fmt.Sprintf(
		"/search/repositories?q=%s&per_page=%d&page=%d",
		url.QueryEscape(query),
		defaultSearchPerPage,
		page,
	)

	var raw restSearchReposResponse
	headers, err := activeAPIClient().DoREST(ctx, http.MethodGet, apiPath, nil, "", &raw)
	if err != nil {
		return nil, false, err
	}

	results := make([]SearchRepoResult, 0, len(raw.Items))
	for _, item := range raw.Items {
		ownerLogin := ""
		if item.Owner != nil {
			ownerLogin = item.Owner.Login
		}
		results = append(results, SearchRepoResult{
			FullName:    item.FullName,
			Owner:       ownerLogin,
			Name:        item.Name,
			Description: item.Description,
			Language:    item.Language,
			Stars:       item.Stars,
			Forks:       item.Forks,
			HTMLURL:     item.HTMLURL,
		})
	}

	hasNext := HasNextPage(headers.Get("Link")) || int64(raw.TotalCount) > int64(page)*int64(defaultSearchPerPage)
	return results, hasNext, nil
}

// SearchUsers searches GitHub users and organizations matching query for the given 1-based page.
// It returns matching users, whether a next page exists, and any error.
func SearchUsers(ctx context.Context, query string, page int) ([]SearchUserResult, bool, error) {
	if strings.TrimSpace(query) == "" {
		return nil, false, errors.New("search query must not be empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if page <= 0 {
		page = 1
	}

	apiPath := fmt.Sprintf(
		"/search/users?q=%s&per_page=%d&page=%d",
		url.QueryEscape(query),
		defaultSearchPerPage,
		page,
	)

	var raw restSearchUsersResponse
	headers, err := activeAPIClient().DoREST(ctx, http.MethodGet, apiPath, nil, "", &raw)
	if err != nil {
		return nil, false, err
	}

	results := make([]SearchUserResult, 0, len(raw.Items))
	for _, item := range raw.Items {
		results = append(results, SearchUserResult{
			Login:     item.Login,
			Name:      item.Name,
			AvatarURL: item.AvatarURL,
			HTMLURL:   item.HTMLURL,
			Type:      item.Type,
		})
	}

	hasNext := HasNextPage(headers.Get("Link")) || int64(raw.TotalCount) > int64(page)*int64(defaultSearchPerPage)
	return results, hasNext, nil
}
