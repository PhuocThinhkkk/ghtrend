package ghclient

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
)

// fetchUserReposHook allows tests to stub out HTTP fetching for user repos.
var fetchUserReposHook = Fetch

func GetAllTrendingRepos(cfg *flags.CmdConfig) (RepoList, error) {
	urlStr := "https://github.com/trending"
	if cfg.Language != "All" && strings.TrimSpace(cfg.Language) != "" {
		urlStr += "/" + url.PathEscape(strings.ToLower(strings.TrimSpace(cfg.Language)))
	}
	if cfg.Since != "daily" {
		urlStr += "?since=" + url.QueryEscape(string(cfg.Since))
	}

	res, err := Fetch(urlStr)
	if err != nil {
		return nil, err
	}

	html := string(res)

	repos, err := parseTrendingPage(html)
	if err != nil {
		return nil, err
	}

	if cfg.Limit > 0 && len(repos) > cfg.Limit {
		repos = repos[:cfg.Limit]
	}

	for i := range repos {
		repos[i].Index = i
	}

	// Only load details for the first repo on initial launch;
	// subsequent repositories load details on demand with debounce and caching.
	if len(repos) > 0 {
		_ = repos[0].LoadDetails()
	}

	return repos, nil
}

// FetchUserRepositoriesPage fetches a single page of repositories for a GitHub user.
// It returns the list of repositories on that page, a boolean indicating if more pages are likely available, and any error.
func FetchUserRepositoriesPage(username, language string, page int) (RepoList, bool, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, false, fmt.Errorf("username cannot be empty")
	}
	if page < 1 {
		page = 1
	}

	u := fmt.Sprintf("https://github.com/%s?tab=repositories&page=%d", url.PathEscape(username), page)
	if language != "" && language != "All" {
		u += "&language=" + url.QueryEscape(strings.ToLower(strings.TrimSpace(language)))
	}

	res, err := fetchUserReposHook(u)
	if err != nil {
		return nil, false, fmt.Errorf("failed to fetch page %d for user '%s': %w", page, username, err)
	}

	pageRepos, err := ParseUserRepositoriesPage(string(res))
	if err != nil {
		return nil, false, err
	}

	hasMore := len(pageRepos) >= 30
	return pageRepos, hasMore, nil
}

// GetUserRepositories fetches and populates repository information for a specified GitHub user.
// It loads page 1 initially (up to 20 repos) and populates details for the first repository.
func GetUserRepositories(cfg *flags.CmdConfig) (RepoList, error) {
	username := strings.TrimSpace(cfg.User)
	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	limit := cfg.Limit
	if limit <= 0 {
		limit = 20
	}

	repos, _, err := FetchUserRepositoriesPage(username, cfg.Language, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repositories for user '%s': %w", username, err)
	}
	if len(repos) == 0 {
		return nil, fmt.Errorf("no repositories found for user '%s'", username)
	}

	profile, _ := GetUserProfile(username)

	if profile != nil && len(profile.PinnedRepos) > 0 {
		pinnedMap := make(map[string]bool)
		for _, name := range profile.PinnedRepos {
			pinnedMap[strings.ToLower(name)] = true
		}

		repoMap := make(map[string]Repo)
		for _, r := range repos {
			repoMap[strings.ToLower(r.Name)] = r
		}

		var pinnedList RepoList
		var regularList RepoList

		for _, name := range profile.PinnedRepos {
			if r, ok := repoMap[strings.ToLower(name)]; ok {
				r.Pinned = true
				pinnedList = append(pinnedList, r)
			}
		}
		for _, r := range repos {
			if !pinnedMap[strings.ToLower(r.Name)] {
				regularList = append(regularList, r)
			}
		}
		repos = append(pinnedList, regularList...)
	}

	initialBatch := 20
	if limit > 0 && limit < initialBatch {
		initialBatch = limit
	}
	if limit <= initialBatch && len(repos) > initialBatch {
		repos = repos[:initialBatch]
	}

	for i := range repos {
		repos[i].Index = i
	}
	if len(repos) > 0 {
		repos[0].UserProfile = profile
		_ = repos[0].LoadDetails()
	}

	return repos, nil
}
