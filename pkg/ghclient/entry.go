package ghclient

import (
	"fmt"
	"strings"

	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
)

// fetchUserReposHook allows tests to stub out HTTP fetching for user repos.
var fetchUserReposHook = Fetch

func GetAllTrendingRepos(cfg *flags.CmdConfig) (RepoList, error) {
	url := "https://github.com/trending"
	if cfg.Language != "All" {
		url += "/" + strings.ToLower(cfg.Language)
	}
	if cfg.Since != "daily" {
		url += "?since=" + string(cfg.Since)
	}

	res, err := Fetch(url)
	if err != nil {
		return nil, err
	}

	html := string(res)

	repos, err := parseTrendingPage(html)
	if err != nil {
		return nil, err
	}

	err = repos.loadDetails()
	if err != nil {
		return nil, err
	}

	return repos, nil
}

// GetUserRepositories fetches and populates detailed repository information for a specified GitHub user.
func GetUserRepositories(cfg *flags.CmdConfig) (RepoList, error) {
	username := strings.TrimSpace(cfg.User)
	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	limit := cfg.Limit
	if limit <= 0 {
		limit = 30
	}

	var allRepos RepoList
	page := 1
	maxPages := 5

	for len(allRepos) < limit && page <= maxPages {
		u := fmt.Sprintf("https://github.com/%s?tab=repositories&page=%d", username, page)
		if cfg.Language != "" && cfg.Language != "All" {
			u += "&language=" + strings.ToLower(cfg.Language)
		}

		res, err := fetchUserReposHook(u)
		if err != nil {
			if len(allRepos) > 0 {
				break
			}
			return nil, fmt.Errorf("failed to fetch repositories for user '%s': %w", username, err)
		}

		pageRepos, err := ParseUserRepositoriesPage(string(res))
		if err != nil {
			if len(allRepos) > 0 {
				break
			}
			return nil, err
		}

		if len(pageRepos) == 0 {
			break
		}

		allRepos = append(allRepos, pageRepos...)
		if len(pageRepos) < 30 {
			break
		}
		page++
	}

	if len(allRepos) == 0 {
		return nil, fmt.Errorf("no repositories found for user '%s'", username)
	}

	if len(allRepos) > limit {
		allRepos = allRepos[:limit]
	}

	for i := range allRepos {
		allRepos[i].Index = i
	}

	err := allRepos.loadDetails()
	if err != nil {
		return nil, err
	}

	return allRepos, nil
}
