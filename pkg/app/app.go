package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	ghclient.SetDefaultCache(cache.NewSessionCache(256, 5*time.Minute))

	cacheDir, err := cache.DefaultCacheDir()
	if err != nil {
		cacheDir = filepath.Join(os.TempDir(), "ghtrend")
	}
	cachePath := filepath.Join(cacheDir, "cachedata.json")

	var historyStore *cache.HistoryStore
	if histPath, hErr := cache.DefaultHistoryPath(); hErr == nil {
		historyStore = cache.NewHistoryStore(histPath)
	}

	baseLimit := 2202
	if app.cfg != nil && app.cfg.Limit > 0 {
		baseLimit = app.cfg.Limit
	}

	opts := ui.LaunchOptions{
		Config:       app.cfg,
		HistoryStore: historyStore,
		FetchTrending: func(since flags.Frequency, language string, useCache bool) (ui.RepoFetchResult, error) {
			cfgCopy := flags.CmdConfig{
				IsCache:  useCache,
				Limit:    baseLimit,
				Since:    since,
				Language: language,
			}
			return loadReposWithCache(cachePath, &cfgCopy, ghclient.GetAllTrendingRepos)
		},
		RefreshTrending: func(since flags.Frequency, language string) ([]ghclient.Repo, error) {
			cfgCopy := flags.CmdConfig{
				IsCache:  false,
				Limit:    baseLimit,
				Since:    since,
				Language: language,
			}
			return refreshAndCacheRepos(cachePath, &cfgCopy, ghclient.GetAllTrendingRepos)
		},
		FetchUser: func(username, language string, useCache bool) (ui.RepoFetchResult, error) {
			cfgCopy := flags.CmdConfig{
				IsCache:  useCache,
				Limit:    baseLimit,
				Since:    flags.Daily,
				Language: language,
				User:     username,
			}
			return loadReposWithCache(cachePath, &cfgCopy, ghclient.GetUserRepositories)
		},
		RefreshUser: func(username, language string) ([]ghclient.Repo, error) {
			cfgCopy := flags.CmdConfig{
				IsCache:  false,
				Limit:    baseLimit,
				Since:    flags.Daily,
				Language: language,
				User:     username,
			}
			return refreshAndCacheRepos(cachePath, &cfgCopy, ghclient.GetUserRepositories)
		},
		FetchDirectRepo: fetchDirectRepoSummary,
	}

	if _, err := ui.RenderWithOptions(opts); err != nil {
		log.Fatal("err when render: ", err)
	}
}

func loadReposWithCache(
	path string,
	cfg *flags.CmdConfig,
	liveFn func(*flags.CmdConfig) (ghclient.RepoList, error),
) (ui.RepoFetchResult, error) {
	if cfg != nil && cfg.IsCache {
		cached, stale, err := cache.LoadCacheWithStaleness(path, cfg)
		if err == nil && len(cached) > 0 {
			return ui.RepoFetchResult{
				Repos:     cached,
				FromCache: true,
				Stale:     stale,
			}, nil
		}
	}
	repos, err := refreshAndCacheRepos(path, cfg, liveFn)
	if err != nil {
		return ui.RepoFetchResult{}, err
	}
	return ui.RepoFetchResult{Repos: repos}, nil
}

func refreshAndCacheRepos(
	path string,
	cfg *flags.CmdConfig,
	liveFn func(*flags.CmdConfig) (ghclient.RepoList, error),
) ([]ghclient.Repo, error) {
	repos, err := liveFn(cfg)
	if err != nil {
		return nil, err
	}
	_ = cache.SaveCache(repos, path, cfg)
	return repos, nil
}

func fetchDirectRepoSummary(owner, repo string) ([]ghclient.Repo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	var meta struct {
		Name        string `json:"name"`
		HTMLURL     string `json:"html_url"`
		Description string `json:"description"`
		Language    string `json:"language"`
		Stars       int    `json:"stargazers_count"`
		Forks       int    `json:"forks_count"`
		Owner       struct {
			Login string `json:"login"`
		} `json:"owner"`
	}

	apiPath := fmt.Sprintf("/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo))
	_, apiErr := ghclient.DefaultClient.DoREST(ctx, http.MethodGet, apiPath, nil, "", &meta)

	resolvedOwner := owner
	resolvedName := repo
	if strings.TrimSpace(meta.Owner.Login) != "" {
		resolvedOwner = meta.Owner.Login
	}
	if strings.TrimSpace(meta.Name) != "" {
		resolvedName = meta.Name
	}

	htmlURL := meta.HTMLURL
	if htmlURL == "" {
		htmlURL = "https://github.com/" + resolvedOwner + "/" + resolvedName
	}

	r := ghclient.Repo{
		Owner:       resolvedOwner,
		Name:        resolvedName,
		Url:         htmlURL,
		Description: meta.Description,
		Language:    meta.Language,
		Stars:       fmt.Sprintf("%d", meta.Stars),
		Forks:       fmt.Sprintf("%d", meta.Forks),
	}

	detailsErr := r.LoadDetails()
	if apiErr != nil && detailsErr != nil {
		return nil, apiErr
	}
	return []ghclient.Repo{r}, nil
}
