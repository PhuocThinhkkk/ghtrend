package ghclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	rawGitHubAcceptHeader = "application/vnd.github.raw+json"
	maxWorkspaceFileBytes = 512 << 10 // 512 KiB
)

// RepoMetadata holds repository summary metadata returned by the GitHub REST API.
type RepoMetadata struct {
	Owner         string   `json:"owner"`
	Name          string   `json:"name"`
	FullName      string   `json:"full_name"`
	Description   string   `json:"description"`
	DefaultBranch string   `json:"default_branch"`
	License       string   `json:"license"`
	Language      string   `json:"language"`
	HTMLURL       string   `json:"html_url"`
	Stars         int      `json:"stars"`
	Forks         int      `json:"forks"`
	Watchers      int      `json:"watchers"`
	OpenIssues    int      `json:"open_issues"`
	Topics        []string `json:"topics"`
	IsPrivate     bool     `json:"is_private"`
	IsFork        bool     `json:"is_fork"`
	IsArchived    bool     `json:"is_archived"`
}

// RefInfo describes a Git branch or tag in a repository.
type RefInfo struct {
	Name string `json:"name"`
	SHA  string `json:"sha"`
	Kind string `json:"kind"` // "branch" or "tag"
}

// TreeEntry describes a directory or file entry in a repository tree.
type TreeEntry struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Mode string `json:"mode"`
	Type string `json:"type"` // "tree" or "blob"
	SHA  string `json:"sha"`
	Size int64  `json:"size"`
}

// FileContent holds the fetched content and display metadata for a repository file.
type FileContent struct {
	Path      string `json:"path"`
	Ref       string `json:"ref"`
	Content   string `json:"content"`
	Size      int64  `json:"size"`
	IsBinary  bool   `json:"is_binary"`
	Truncated bool   `json:"truncated"`
}

// FetchRepoMetadata fetches repository metadata from GET /repos/{owner}/{repo}.
func FetchRepoMetadata(ctx context.Context, owner, repo string) (RepoMetadata, error) {
	if err := validateOwnerRepo(owner, repo); err != nil {
		return RepoMetadata{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	apiPath := fmt.Sprintf("/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo))
	var raw struct {
		Name          string   `json:"name"`
		FullName      string   `json:"full_name"`
		Description   string   `json:"description"`
		DefaultBranch string   `json:"default_branch"`
		Language      string   `json:"language"`
		HTMLURL       string   `json:"html_url"`
		Stars         int      `json:"stargazers_count"`
		Forks         int      `json:"forks_count"`
		WatchersCount int      `json:"watchers_count"`
		Subscribers   *int     `json:"subscribers_count"`
		OpenIssues    int      `json:"open_issues_count"`
		Topics        []string `json:"topics"`
		IsPrivate     bool     `json:"private"`
		IsFork        bool     `json:"fork"`
		IsArchived    bool     `json:"archived"`
		Owner         struct {
			Login string `json:"login"`
		} `json:"owner"`
		License *struct {
			SPDXID string `json:"spdx_id"`
			Name   string `json:"name"`
		} `json:"license"`
	}

	if _, err := activeAPIClient().DoREST(ctx, http.MethodGet, apiPath, nil, "", &raw); err != nil {
		return RepoMetadata{}, err
	}

	ownerName := raw.Owner.Login
	if ownerName == "" {
		ownerName = owner
	}

	license := ""
	if raw.License != nil {
		if raw.License.SPDXID != "" && raw.License.SPDXID != "NOASSERTION" {
			license = raw.License.SPDXID
		} else {
			license = raw.License.Name
		}
	}

	watchers := raw.WatchersCount
	if raw.Subscribers != nil {
		watchers = *raw.Subscribers
	}

	return RepoMetadata{
		Owner:         ownerName,
		Name:          raw.Name,
		FullName:      raw.FullName,
		Description:   raw.Description,
		DefaultBranch: raw.DefaultBranch,
		License:       license,
		Language:      raw.Language,
		HTMLURL:       raw.HTMLURL,
		Stars:         raw.Stars,
		Forks:         raw.Forks,
		Watchers:      watchers,
		OpenIssues:    raw.OpenIssues,
		Topics:        slices.Clone(raw.Topics),
		IsPrivate:     raw.IsPrivate,
		IsFork:        raw.IsFork,
		IsArchived:    raw.IsArchived,
	}, nil
}

// FetchRepoRefs fetches repository branches and tags. If tag fetching fails while
// branches succeed, the branch refs are returned without error; if both fail,
// a combined error is returned.
func FetchRepoRefs(ctx context.Context, owner, repo string) ([]RefInfo, error) {
	if err := validateOwnerRepo(owner, repo); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	escapedOwner := url.PathEscape(owner)
	escapedRepo := url.PathEscape(repo)
	client := activeAPIClient()

	type rawRef struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}

	var rawBranches []rawRef
	branchesPath := fmt.Sprintf("/repos/%s/%s/branches?per_page=100", escapedOwner, escapedRepo)
	_, branchErr := client.DoREST(ctx, http.MethodGet, branchesPath, nil, "", &rawBranches)

	var rawTags []rawRef
	tagsPath := fmt.Sprintf("/repos/%s/%s/tags?per_page=100", escapedOwner, escapedRepo)
	_, tagErr := client.DoREST(ctx, http.MethodGet, tagsPath, nil, "", &rawTags)

	if branchErr != nil && tagErr != nil {
		return nil, errors.Join(
			fmt.Errorf("fetching branches: %w", branchErr),
			fmt.Errorf("fetching tags: %w", tagErr),
		)
	}

	refs := make([]RefInfo, 0, len(rawBranches)+len(rawTags))
	if branchErr == nil {
		for _, b := range rawBranches {
			refs = append(refs, RefInfo{
				Name: b.Name,
				SHA:  b.Commit.SHA,
				Kind: "branch",
			})
		}
	}
	if tagErr == nil {
		for _, t := range rawTags {
			refs = append(refs, RefInfo{
				Name: t.Name,
				SHA:  t.Commit.SHA,
				Kind: "tag",
			})
		}
	}

	return refs, nil
}

// FetchRepoTree fetches the recursive Git tree for a repository ref, sorting
// directories before files case-insensitively. Empty repositories (HTTP 409)
// return nil, false, nil.
func FetchRepoTree(ctx context.Context, owner, repo, ref string) (entries []TreeEntry, truncated bool, err error) {
	if err := validateOwnerRepo(owner, repo); err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(ref) == "" {
		return nil, false, errors.New("ref must not be empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	apiPath := fmt.Sprintf(
		"/repos/%s/%s/git/trees/%s?recursive=1",
		url.PathEscape(owner),
		url.PathEscape(repo),
		url.PathEscape(ref),
	)

	var raw struct {
		SHA       string `json:"sha"`
		Truncated bool   `json:"truncated"`
		Tree      []struct {
			Path string `json:"path"`
			Mode string `json:"mode"`
			Type string `json:"type"`
			SHA  string `json:"sha"`
			Size int64  `json:"size"`
		} `json:"tree"`
	}

	if _, err := activeAPIClient().DoREST(ctx, http.MethodGet, apiPath, nil, "", &raw); err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict {
			return nil, false, nil
		}
		return nil, false, err
	}

	entries = make([]TreeEntry, 0, len(raw.Tree))
	for _, item := range raw.Tree {
		entries = append(entries, TreeEntry{
			Path: item.Path,
			Name: path.Base(item.Path),
			Mode: item.Mode,
			Type: item.Type,
			SHA:  item.SHA,
			Size: item.Size,
		})
	}
	sortTreeEntries(entries)

	return entries, raw.Truncated, nil
}

// FetchRepoDirContents fetches the entries of a single directory via the GitHub
// contents API as a fallback for truncated trees, sorting directories before files.
func FetchRepoDirContents(ctx context.Context, owner, repo, dirPath, ref string) ([]TreeEntry, error) {
	if err := validateOwnerRepo(owner, repo); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	escapedPath, err := escapeRepoPath(dirPath)
	if err != nil {
		return nil, err
	}

	apiPath := fmt.Sprintf("/repos/%s/%s/contents", url.PathEscape(owner), url.PathEscape(repo))
	if escapedPath != "" {
		apiPath += "/" + escapedPath
	}
	apiPath = appendRefQuery(apiPath, ref)

	var items []struct {
		Name string `json:"name"`
		Path string `json:"path"`
		SHA  string `json:"sha"`
		Size int64  `json:"size"`
		Type string `json:"type"`
	}

	if _, err := activeAPIClient().DoREST(ctx, http.MethodGet, apiPath, nil, "", &items); err != nil {
		return nil, err
	}

	entries := make([]TreeEntry, 0, len(items))
	for _, item := range items {
		entryType := item.Type
		switch entryType {
		case "dir":
			entryType = "tree"
		case "file":
			entryType = "blob"
		}
		name := item.Name
		if name == "" {
			name = path.Base(item.Path)
		}
		entries = append(entries, TreeEntry{
			Path: item.Path,
			Name: name,
			Type: entryType,
			SHA:  item.SHA,
			Size: item.Size,
		})
	}
	sortTreeEntries(entries)

	return entries, nil
}

// FetchFileContent fetches raw file content from the GitHub contents API,
// detecting binary files and truncating text content safely at 512 KiB.
func FetchFileContent(ctx context.Context, owner, repo, filePath, ref string) (FileContent, error) {
	if err := validateOwnerRepo(owner, repo); err != nil {
		return FileContent{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	escapedPath, err := escapeRepoPath(filePath)
	if err != nil {
		return FileContent{}, err
	}
	if escapedPath == "" {
		return FileContent{}, errors.New("file path must not be empty")
	}

	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s", url.PathEscape(owner), url.PathEscape(repo), escapedPath)
	apiPath = appendRefQuery(apiPath, ref)

	raw, _, err := activeAPIClient().DoRESTRaw(ctx, http.MethodGet, apiPath, nil, rawGitHubAcceptHeader)
	if err != nil {
		return FileContent{}, err
	}

	cleanPath := strings.Trim(filePath, "/")
	size := int64(len(raw))

	if bytes.IndexByte(raw, 0) >= 0 || !utf8.Valid(raw) {
		return FileContent{
			Path:     cleanPath,
			Ref:      ref,
			Content:  "",
			Size:     size,
			IsBinary: true,
		}, nil
	}

	content, truncated := truncateUTF8(raw, maxWorkspaceFileBytes)
	return FileContent{
		Path:      cleanPath,
		Ref:       ref,
		Content:   content,
		Size:      size,
		IsBinary:  false,
		Truncated: truncated,
	}, nil
}

// FetchRepoREADME fetches the raw README content for a repository at ref.
// Missing READMEs (ErrNotFound) return "", nil.
func FetchRepoREADME(ctx context.Context, owner, repo, ref string) (string, error) {
	if err := validateOwnerRepo(owner, repo); err != nil {
		return "", err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	apiPath := fmt.Sprintf("/repos/%s/%s/readme", url.PathEscape(owner), url.PathEscape(repo))
	apiPath = appendRefQuery(apiPath, ref)

	raw, _, err := activeAPIClient().DoRESTRaw(ctx, http.MethodGet, apiPath, nil, rawGitHubAcceptHeader)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", nil
		}
		return "", err
	}

	return string(raw), nil
}

func validateOwnerRepo(owner, repo string) error {
	if strings.TrimSpace(owner) == "" || strings.TrimSpace(repo) == "" {
		return errors.New("owner and repo must not be empty")
	}
	if owner == "." || owner == ".." || repo == "." || repo == ".." {
		return errors.New("invalid owner or repo")
	}
	if strings.ContainsAny(owner, "/\\") || strings.ContainsAny(repo, "/\\") {
		return errors.New("invalid owner or repo")
	}
	return nil
}

func escapeRepoPath(rawPath string) (string, error) {
	if strings.Contains(rawPath, "\\") {
		return "", errors.New("invalid repository path")
	}
	trimmed := strings.Trim(rawPath, "/")
	if trimmed == "" {
		return "", nil
	}

	parts := strings.Split(trimmed, "/")
	escaped := make([]string, 0, len(parts))
	for _, seg := range parts {
		if seg == "" {
			continue
		}
		if seg == "." || seg == ".." {
			return "", errors.New("invalid repository path")
		}
		escaped = append(escaped, url.PathEscape(seg))
	}
	return strings.Join(escaped, "/"), nil
}

func appendRefQuery(apiPath, ref string) string {
	if ref == "" {
		return apiPath
	}
	return apiPath + "?ref=" + url.QueryEscape(ref)
}

func sortTreeEntries(entries []TreeEntry) {
	sort.Slice(entries, func(i, j int) bool {
		iDir := entries[i].Type == "tree"
		jDir := entries[j].Type == "tree"
		if iDir != jDir {
			return iDir
		}
		iLower := strings.ToLower(entries[i].Path)
		jLower := strings.ToLower(entries[j].Path)
		if iLower != jLower {
			return iLower < jLower
		}
		return entries[i].Path < entries[j].Path
	})
}

func truncateUTF8(b []byte, maxBytes int) (string, bool) {
	if len(b) <= maxBytes {
		return string(b), false
	}
	limit := maxBytes
	for limit > 0 && !utf8.RuneStart(b[limit]) {
		limit--
	}
	return string(b[:limit]), true
}
