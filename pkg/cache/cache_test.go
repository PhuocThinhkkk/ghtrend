package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
)

func TestCache_TrendingSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "testcache.json")

	cfg := &flags.CmdConfig{
		Since:    flags.Daily,
		Language: "Go",
		User:     "",
	}

	repos := []ghclient.Repo{
		{Name: "ghtrend", Owner: "PhuocThinhkkk", Language: "Go"},
	}

	err := SaveCache(repos, cachePath, cfg)
	if err != nil {
		t.Fatalf("unexpected SaveCache error: %v", err)
	}

	loaded, err := LoadCache(cachePath, cfg)
	if err != nil {
		t.Fatalf("unexpected LoadCache error: %v", err)
	}
	if len(loaded) != 1 || loaded[0].Name != "ghtrend" {
		t.Fatalf("expected loaded repo 'ghtrend', got %v", loaded)
	}
}

func TestCache_UserSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "testcache.json")

	userCfg := &flags.CmdConfig{
		Since:    flags.Daily,
		Language: "Go",
		User:     "torvalds",
	}

	repos := []ghclient.Repo{
		{Name: "linux", Owner: "torvalds", Language: "C"},
	}

	err := SaveCache(repos, cachePath, userCfg)
	if err != nil {
		t.Fatalf("unexpected SaveCache error: %v", err)
	}

	loaded, err := LoadCache(cachePath, userCfg)
	if err != nil {
		t.Fatalf("unexpected LoadCache error: %v", err)
	}
	if len(loaded) != 1 || loaded[0].Name != "linux" {
		t.Fatalf("expected loaded repo 'linux', got %v", loaded)
	}

	// Different user should miss
	otherUserCfg := &flags.CmdConfig{
		Since:    flags.Daily,
		Language: "Go",
		User:     "octocat",
	}
	_, err = LoadCache(cachePath, otherUserCfg)
	if err == nil {
		t.Errorf("expected cache miss for different user, got nil error")
	}

	// Trending (empty user) should also miss
	trendingCfg := &flags.CmdConfig{
		Since:    flags.Daily,
		Language: "Go",
		User:     "",
	}
	_, err = LoadCache(cachePath, trendingCfg)
	if err == nil {
		t.Errorf("expected cache miss for trending vs user, got nil error")
	}
}

func TestCache_Expired(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "testcache.json")

	cfg := &flags.CmdConfig{
		Since:    flags.Daily,
		Language: "Go",
	}

	repos := []ghclient.Repo{
		{Name: "ghtrend", Owner: "PhuocThinhkkk"},
	}

	expiredData := []CacheData{
		{
			Timestamp: time.Now().Unix() - 3700, // > 1 hour ago
			Since:     cfg.Since,
			Language:  cfg.Language,
			User:      cfg.User,
			Data:      repos,
		},
	}

	func() {
		f, err := os.Create(cachePath)
		if err != nil {
			t.Fatalf("failed to create cache file: %v", err)
		}
		defer f.Close()

		if err := json.NewEncoder(f).Encode(expiredData); err != nil {
			t.Fatalf("failed to encode expired cache: %v", err)
		}
	}()

	// Loading expired cache should return error
	_, err := LoadCache(cachePath, cfg)
	if err == nil {
		t.Errorf("expected cache miss for expired cache, got nil error")
	}

	// Loading non-existent cache should return error
	_, err = LoadCache(filepath.Join(tmpDir, "nonexistent.json"), cfg)
	if err == nil {
		t.Errorf("expected error loading non-existent cache, got nil")
	}
}
