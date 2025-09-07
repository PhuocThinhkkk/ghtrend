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

	cacheDir, _ := os.UserCacheDir()
	ghtrendDir := filepath.Join(cacheDir, "ghtrend")
	cachePath := filepath.Join(ghtrendDir, "cachedata.json")
	if app.cfg.IsCache {
		cacheRepos, err := cache.LoadCache(cachePath)

		if err != nil {
			repos, err = ghclient.GetAllTrendingRepos(app.cfg)
			if err != nil {
				log.Fatal(err)
			}

			err = cache.SaveCache(repos, cachePath)
			if err != nil {
				log.Fatal(err)
			}

		} else {
			repos = cacheRepos
		}
	} else {
		repos, err := ghclient.GetAllTrendingRepos(app.cfg)
		if err != nil {
			log.Fatal(err)
		}

		err = cache.SaveCache(repos, cachePath)
		if err != nil {
			log.Fatal(err)
		}
	}

	program, err := ui.Render(app.cfg, repos)
	if err != nil {
		log.Fatal("err when render: ", err)
	}

	_ = program

}
