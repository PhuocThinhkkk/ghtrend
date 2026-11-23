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
	seenRepos := make(map[string]bool)
	page := 1
	maxPages := 5

	for len(allRepos) < limit && page <= maxPages {
		u := fmt.Sprintf("https://github.com/%s?tab=repositories&page=%d", url.PathEscape(username), page)
		if cfg.Language != "" && cfg.Language != "All" {
			u += "&language=" + url.QueryEscape(strings.ToLower(strings.TrimSpace(cfg.Language)))
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

		for _, pageRepo := range pageRepos {
			key := strings.ToLower(pageRepo.Owner + "/" + pageRepo.Name)
			if !seenRepos[key] {
				seenRepos[key] = true
				allRepos = append(allRepos, pageRepo)
			}
		}

		if len(pageRepos) < 30 {
			break
		}
		page++
	}

	if len(allRepos) == 0 {
		return nil, fmt.Errorf("no repositories found for user '%s'", username)
	}

	profile, _ := GetUserProfile(username)

	if profile != nil && len(profile.PinnedRepos) > 0 {
		pinnedMap := make(map[string]bool)
		for _, name := range profile.PinnedRepos {
			pinnedMap[strings.ToLower(name)] = true
		}

		repoMap := make(map[string]Repo)
		for _, r := range allRepos {
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
		for _, r := range allRepos {
			if !pinnedMap[strings.ToLower(r.Name)] {
				regularList = append(regularList, r)
			}
		}
		allRepos = append(pinnedList, regularList...)
	}

	if len(allRepos) > limit {
		allRepos = allRepos[:limit]
	}

	for i := range allRepos {
		allRepos[i].Index = i
	}
	if len(allRepos) > 0 {
		allRepos[0].UserProfile = profile
	}

	err := allRepos.loadDetails()
	if err != nil {
		return nil, err
	}

	return allRepos, nil
}
