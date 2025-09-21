package ghclient

import (
	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
	"strings"
)

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
