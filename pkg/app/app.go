package app

import (
	"ghtrend/pkg/cache"
	"ghtrend/pkg/configs/flags"
	"ghtrend/pkg/ghclient"
	"ghtrend/pkg/ui"
	"log"
	"os"
	"path/filepath"
)

type App struct {
	cfg *flags.CmdConfig
}

func NewApp(cfg *flags.CmdConfig) *App {
	return &App{
		cfg: cfg,
	}
}

func (app *App) Start() {
	repos := []ghclient.Repo{}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		log.Fatal("cant get the cache dir:", err)
	}
	cachePath := filepath.Join(cacheDir, "ghtrend", "cachedata.json")
	var error error
	if app.cfg.IsCache {
		repos, error = cacheFetcher(cachePath, app.cfg)
	} else {
		repos, error = noCacheFetcher(cachePath, app.cfg)
	}
	if error != nil {
		log.Fatal(err)
	}

	program, err := ui.Render(app.cfg, repos)
	if err != nil {
		log.Fatal("err when render: ", err)
	}

	_ = program

}

func cacheFetcher(path string, cfg *flags.CmdConfig) ([]ghclient.Repo, error) {
	cacheRepos, err := cache.LoadCache(path)

	if err != nil {
		repos, err := ghclient.GetAllTrendingRepos(cfg)
		if err != nil {
			return []ghclient.Repo{}, err
		}

		err = cache.SaveCache(repos, path)
		if err != nil {
			return repos, err
		}
		return repos, nil
	}
	return cacheRepos, nil

}

func noCacheFetcher(path string, cfg *flags.CmdConfig) ([]ghclient.Repo, error) {
	repos, err := ghclient.GetAllTrendingRepos(cfg)
	if err != nil {
		return []ghclient.Repo{}, err
	}

	err = cache.SaveCache(repos, path)
	if err != nil {
		return []ghclient.Repo{}, err
	}
	return repos, nil
}
