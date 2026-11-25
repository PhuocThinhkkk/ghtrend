package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestCache_LimitSeparation(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "limitcache.json")

	cfg10 := &flags.CmdConfig{Since: flags.Daily, Language: "Go", Limit: 10}
	cfg25 := &flags.CmdConfig{Since: flags.Daily, Language: "Go", Limit: 25}
	cfg50 := &flags.CmdConfig{Since: flags.Daily, Language: "Go", Limit: 50}

	repos10 := []ghclient.Repo{{Name: "repo-10", Owner: "org"}}
	repos25 := []ghclient.Repo{{Name: "repo-25", Owner: "org"}, {Name: "repo-25-b", Owner: "org"}}

	if err := SaveCache(repos10, cachePath, cfg10); err != nil {
		t.Fatalf("SaveCache cfg10 error: %v", err)
	}
	if err := SaveCache(repos25, cachePath, cfg25); err != nil {
		t.Fatalf("SaveCache cfg25 error: %v", err)
	}

	got10, err := LoadCache(cachePath, cfg10)
	if err != nil {
		t.Fatalf("LoadCache cfg10 error: %v", err)
	}
	if len(got10) != 1 || got10[0].Name != "repo-10" {
		t.Fatalf("expected repo-10, got %+v", got10)
	}

	got25, err := LoadCache(cachePath, cfg25)
	if err != nil {
		t.Fatalf("LoadCache cfg25 error: %v", err)
	}
	if len(got25) != 2 || got25[0].Name != "repo-25" {
		t.Fatalf("expected repo-25 entries, got %+v", got25)
	}

	if _, err := LoadCache(cachePath, cfg50); err == nil {
		t.Fatalf("expected cache miss for Limit=50, got nil")
	}

	// Updating cfg10 must preserve cfg25.
	repos10Updated := []ghclient.Repo{{Name: "repo-10-v2", Owner: "org"}}
	if err := SaveCache(repos10Updated, cachePath, cfg10); err != nil {
		t.Fatalf("SaveCache update cfg10 error: %v", err)
	}
	got10v2, err := LoadCache(cachePath, cfg10)
	if err != nil || len(got10v2) != 1 || got10v2[0].Name != "repo-10-v2" {
		t.Fatalf("expected updated repo-10-v2, got (%+v, %v)", got10v2, err)
	}
	got25Still, err := LoadCache(cachePath, cfg25)
	if err != nil || len(got25Still) != 2 {
		t.Fatalf("expected cfg25 entry preserved, got (%+v, %v)", got25Still, err)
	}
}

func TestCache_LoadCacheWithStaleness(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "stalecache.json")

	freshCfg := &flags.CmdConfig{Since: flags.Daily, Language: "Go", Limit: 10}
	staleCfg := &flags.CmdConfig{Since: flags.Weekly, Language: "Rust", Limit: 25}
	missingCfg := &flags.CmdConfig{Since: flags.Monthly, Language: "Python", Limit: 5}

	now := time.Now().Unix()
	entries := []CacheData{
		{
			Timestamp: now - 60,
			Since:     freshCfg.Since,
			Language:  freshCfg.Language,
			Limit:     freshCfg.Limit,
			Data:      []ghclient.Repo{{Name: "fresh-repo"}},
		},
		{
			Timestamp: now - 3600, // exact 1 hour boundary => stale
			Since:     staleCfg.Since,
			Language:  staleCfg.Language,
			Limit:     staleCfg.Limit,
			Data:      []ghclient.Repo{{Name: "stale-repo"}},
		},
	}

	raw, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal entries: %v", err)
	}
	if err := os.WriteFile(cachePath, raw, 0644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}

	tests := []struct {
		name      string
		path      string
		cfg       *flags.CmdConfig
		wantName  string
		wantStale bool
		wantErr   bool
	}{
		{
			name:      "fresh entry",
			path:      cachePath,
			cfg:       freshCfg,
			wantName:  "fresh-repo",
			wantStale: false,
			wantErr:   false,
		},
		{
			name:      "stale entry returns data with stale=true",
			path:      cachePath,
			cfg:       staleCfg,
			wantName:  "stale-repo",
			wantStale: true,
			wantErr:   false,
		},
		{
			name:    "missing key returns error",
			path:    cachePath,
			cfg:     missingCfg,
			wantErr: true,
		},
		{
			name:    "missing file returns error",
			path:    filepath.Join(tmpDir, "missing.json"),
			cfg:     freshCfg,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repos, stale, err := LoadCacheWithStaleness(tc.path, tc.cfg)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if stale != tc.wantStale {
				t.Fatalf("stale=%v, want %v", stale, tc.wantStale)
			}
			if len(repos) != 1 || repos[0].Name != tc.wantName {
				t.Fatalf("repos=%+v, want single repo %q", repos, tc.wantName)
			}
		})
	}
}

func TestCache_NilConfigAndCorruptJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	if err := SaveCache([]ghclient.Repo{{Name: "a"}}, cachePath, nil); err == nil {
		t.Fatalf("expected error for nil cfg in SaveCache")
	}
	if _, err := LoadCache(cachePath, nil); err == nil {
		t.Fatalf("expected error for nil cfg in LoadCache")
	}
	if _, _, err := LoadCacheWithStaleness(cachePath, nil); err == nil {
		t.Fatalf("expected error for nil cfg in LoadCacheWithStaleness")
	}

	cfg := &flags.CmdConfig{Since: flags.Daily, Language: "Go", Limit: 10}

	// Non-empty corrupt JSON must return error and not clobber existing file.
	corruptContent := []byte("{not-valid-json")
	if err := os.WriteFile(cachePath, corruptContent, 0644); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	if err := SaveCache([]ghclient.Repo{{Name: "a"}}, cachePath, cfg); err == nil {
		t.Fatalf("expected error saving over corrupt non-empty JSON cache")
	}
	after, err := os.ReadFile(cachePath)
	if err != nil || string(after) != string(corruptContent) {
		t.Fatalf("corrupt cache file was modified unexpectedly: got %q, err=%v", after, err)
	}

	// Empty file is treated as fresh start.
	emptyPath := filepath.Join(tmpDir, "empty.json")
	if err := os.WriteFile(emptyPath, []byte("   \n"), 0644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	if err := SaveCache([]ghclient.Repo{{Name: "fresh"}}, emptyPath, cfg); err != nil {
		t.Fatalf("expected SaveCache to succeed on empty file, got %v", err)
	}
	loaded, err := LoadCache(emptyPath, cfg)
	if err != nil || len(loaded) != 1 || loaded[0].Name != "fresh" {
		t.Fatalf("expected loaded repo 'fresh', got (%+v, %v)", loaded, err)
	}
}

func TestCache_ConcurrentDiskAccess(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "concurrent.json")

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		id := g
		wg.Add(1)
		go func() {
			defer wg.Done()
			cfg := &flags.CmdConfig{
				Since:    flags.Daily,
				Language: fmt.Sprintf("Lang-%d", id%4),
				Limit:    10 + id%2,
			}
			repos := []ghclient.Repo{{Name: fmt.Sprintf("repo-%d", id), Owner: "org"}}
			for i := 0; i < 10; i++ {
				if err := SaveCache(repos, cachePath, cfg); err != nil {
					t.Errorf("SaveCache error: %v", err)
					return
				}
				if _, err := LoadCache(cachePath, cfg); err != nil {
					t.Errorf("LoadCache error: %v", err)
					return
				}
				if _, stale, err := LoadCacheWithStaleness(cachePath, cfg); err != nil || stale {
					t.Errorf("LoadCacheWithStaleness got stale=%v err=%v", stale, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestCache_RejectsOversizedSingleEntryAndPreservesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "size_guard.json")

	smallCfg := &flags.CmdConfig{Since: flags.Daily, Language: "Go", Limit: 5}
	if err := SaveCache([]ghclient.Repo{{Name: "keep-me", Owner: "org"}}, cachePath, smallCfg); err != nil {
		t.Fatalf("initial SaveCache failed: %v", err)
	}

	hugeCfg := &flags.CmdConfig{Since: flags.Weekly, Language: "Go", Limit: 5}
	hugeReadme := strings.Repeat("A", maxCacheFileBytes+1024)
	if err := SaveCache([]ghclient.Repo{{Name: "huge", Owner: "org", ReadMe: hugeReadme}}, cachePath, hugeCfg); err == nil {
		t.Fatalf("expected SaveCache to reject entry exceeding maxCacheFileBytes")
	}

	// Existing usable cache must remain intact.
	loaded, err := LoadCache(cachePath, smallCfg)
	if err != nil || len(loaded) != 1 || loaded[0].Name != "keep-me" {
		t.Fatalf("expected existing cache preserved, got (%+v, %v)", loaded, err)
	}
}
