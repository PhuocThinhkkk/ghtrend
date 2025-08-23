package ghclient

import (
)

func GetAllTrendingRepos() (RepoList, error) {

	res, err := fetch("https://github.com/trending")
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
