package app

import (
	"ghtrend/pkg/cache"
	"ghtrend/pkg/ghclient"
	"ghtrend/pkg/types"
	"ghtrend/pkg/ui"
	"log"
	"os"
	"path/filepath"
)

func Run() {
	repos := []types.Repo{}
	cacheDir, _ := os.UserCacheDir()
	ghtrendDir := filepath.Join(cacheDir, "ghtrend")
	cachePath := filepath.Join(ghtrendDir, "cachedata.json")
	cacheRepos, err := cache.LoadCache(cachePath)

	if err != nil {
		repos, err = ghclient.GetAllTrendingRepos()
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

	program, err := ui.Render(repos)
	if err != nil {
		log.Fatal("err when render: ", err)
	}

	_ = program

}
