package app

import (
	"log"
	"os"
	"path/filepath"

	"github.com/PhuocThinhkkk/ghtrend/pkg/cache"
	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
	"github.com/PhuocThinhkkk/ghtrend/pkg/ui"
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
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		log.Fatal("cant get the cache dir:", err)
	}
	cachePath := filepath.Join(cacheDir, "ghtrend", "cachedata.json")

	fetcher := func() ([]ghclient.Repo, error) {
		if app.cfg.IsCache {
			return cacheFetcher(cachePath, app.cfg)
		}
		return noCacheFetcher(cachePath, app.cfg)
	}

	_, err = ui.RenderWithLoading(app.cfg, fetcher)
	if err != nil {
		log.Fatal("err when render: ", err)
	}
}

func fetchRepos(cfg *flags.CmdConfig) ([]ghclient.Repo, error) {
	if cfg.User != "" {
		return ghclient.GetUserRepositories(cfg)
	}
	return ghclient.GetAllTrendingRepos(cfg)
}

func cacheFetcher(path string, cfg *flags.CmdConfig) ([]ghclient.Repo, error) {
	cacheRepos, err := cache.LoadCache(path, cfg)
	if err != nil {
		repos, err := fetchRepos(cfg)
		if err != nil {
			return []ghclient.Repo{}, err
		}

		err = cache.SaveCache(repos, path, cfg)
		if err != nil {
			return repos, err
		}
		return repos, nil
	}
	return cacheRepos, nil
}

func noCacheFetcher(path string, cfg *flags.CmdConfig) ([]ghclient.Repo, error) {
	repos, err := fetchRepos(cfg)
	if err != nil {
		return []ghclient.Repo{}, err
	}

	err = cache.SaveCache(repos, path, cfg)
	if err != nil {
		return []ghclient.Repo{}, err
	}
	return repos, nil
}
