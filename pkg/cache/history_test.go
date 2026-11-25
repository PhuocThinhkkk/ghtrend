package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
)

func TestDefaultHistoryPath(t *testing.T) {
	t.Run("honors GHTREND_CACHE_DIR", func(t *testing.T) {
		customDir := filepath.Join(t.TempDir(), "custom-cache")
		t.Setenv("GHTREND_CACHE_DIR", customDir)

		got, err := DefaultHistoryPath()
		if err != nil {
			t.Fatalf("DefaultHistoryPath() error = %v", err)
		}
		want := filepath.Join(customDir, "ghtrend", "history.json")
		if got != want {
			t.Fatalf("DefaultHistoryPath() = %q, want %q", got, want)
		}
	})

	t.Run("falls back when GHTREND_CACHE_DIR is empty", func(t *testing.T) {
		t.Setenv("GHTREND_CACHE_DIR", "   ")

		got, err := DefaultHistoryPath()
		if err != nil {
			t.Fatalf("DefaultHistoryPath() error = %v", err)
		}
		wantSuffix := filepath.Join("ghtrend", "history.json")
		if !strings.HasSuffix(got, wantSuffix) {
			t.Fatalf("DefaultHistoryPath() = %q, want suffix %q", got, wantSuffix)
		}
	})
}

func TestHistoryStore_Load(t *testing.T) {
	t.Run("missing file returns empty slice", func(t *testing.T) {
		store := NewHistoryStore(filepath.Join(t.TempDir(), "missing", "history.json"))
		entries, err := store.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if entries == nil || len(entries) != 0 {
			t.Fatalf("Load() = %#v, want non-nil empty slice", entries)
		}
	})

	t.Run("empty file returns empty slice", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "history.json")
		if err := os.WriteFile(path, []byte("   \n\t "), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}
		store := NewHistoryStore(path)
		entries, err := store.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if entries == nil || len(entries) != 0 {
			t.Fatalf("Load() = %#v, want non-nil empty slice", entries)
		}
	})

	t.Run("corrupt JSON returns wrapped error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "history.json")
		if err := os.WriteFile(path, []byte("{not-json"), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}
		store := NewHistoryStore(path)
		_, err := store.Load()
		if err == nil {
			t.Fatal("Load() expected error for corrupt JSON, got nil")
		}
		var syntaxErr *json.SyntaxError
		if !errors.As(err, &syntaxErr) {
			t.Fatalf("Load() error = %v, want wrapped *json.SyntaxError", err)
		}
	})

	t.Run("sorts newest LastVisited first", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "history.json")
		t1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
		t2 := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
		t3 := time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC)

		raw, err := json.Marshal([]HistoryEntry{
			{ID: "user:oldest", Type: HistoryTypeUser, Target: "oldest", LastVisited: t1, VisitCount: 1},
			{ID: "user:newest", Type: HistoryTypeUser, Target: "newest", LastVisited: t3, VisitCount: 1},
			{ID: "user:middle", Type: HistoryTypeUser, Target: "middle", LastVisited: t2, VisitCount: 1},
		})
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}
		if err := os.WriteFile(path, raw, 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		store := NewHistoryStore(path)
		entries, err := store.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if len(entries) != 3 {
			t.Fatalf("len(entries) = %d, want 3", len(entries))
		}
		wantOrder := []string{"user:newest", "user:middle", "user:oldest"}
		for i, wantID := range wantOrder {
			if entries[i].ID != wantID {
				t.Fatalf("entries[%d].ID = %q, want %q", i, entries[i].ID, wantID)
			}
		}
	})

	t.Run("rejects oversized file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "history.json")
		oversized := make([]byte, maxHistoryFileBytes+1)
		for i := range oversized {
			oversized[i] = ' '
		}
		if err := os.WriteFile(path, oversized, 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		store := NewHistoryStore(path)
		if _, err := store.Load(); err == nil {
			t.Fatal("Load() expected error for oversized file, got nil")
		}
	})
}

func TestHistoryStore_RecordCanonicalAndDeduplicate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "history.json")
	store := NewHistoryStore(path)
	if store.Path() != path {
		t.Fatalf("Path() = %q, want %q", store.Path(), path)
	}

	t1 := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	t2 := t1.Add(1 * time.Hour)
	t3 := t1.Add(2 * time.Hour)

	// Record user without explicit ID; later record same user with mixed case / @ prefix.
	if err := store.Record(HistoryEntry{
		Type:        HistoryTypeUser,
		Target:      "Torvalds",
		Label:       "Linus Torvalds",
		LastVisited: t1,
	}); err != nil {
		t.Fatalf("Record(user 1) error = %v", err)
	}
	if err := store.Record(HistoryEntry{
		Type:        HistoryTypeUser,
		Target:      "@torvalds",
		Subtitle:    "Linux creator",
		LastVisited: t2,
	}); err != nil {
		t.Fatalf("Record(user 2) error = %v", err)
	}

	// Record repo via slug, then via URL.
	if err := store.Record(HistoryEntry{
		Type:        HistoryTypeRepo,
		Target:      "Charmbracelet/Bubbletea",
		Label:       "charmbracelet/bubbletea",
		LastVisited: t1,
	}); err != nil {
		t.Fatalf("Record(repo 1) error = %v", err)
	}
	if err := store.Record(HistoryEntry{
		Type:        HistoryTypeRepo,
		Target:      "https://github.com/charmbracelet/bubbletea.git",
		LastVisited: t3,
	}); err != nil {
		t.Fatalf("Record(repo 2) error = %v", err)
	}

	// Record trend with empty language vs "All".
	if err := store.Record(HistoryEntry{
		Type:        HistoryTypeTrend,
		Target:      "daily/All",
		Label:       "Daily · All",
		Frequency:   flags.Daily,
		Language:    "",
		LastVisited: t1,
	}); err != nil {
		t.Fatalf("Record(trend 1) error = %v", err)
	}
	if err := store.Record(HistoryEntry{
		Type:        HistoryTypeTrend,
		Target:      "daily/All",
		Label:       "Daily · All",
		Frequency:   flags.Daily,
		Language:    "All",
		LastVisited: t2,
	}); err != nil {
		t.Fatalf("Record(trend 2) error = %v", err)
	}

	// Record with zero LastVisited sets current UTC time.
	if err := store.Record(HistoryEntry{
		Type:      HistoryTypeTrend,
		Target:    "weekly/Go",
		Label:     "Weekly · Go",
		Frequency: flags.Weekly,
		Language:  "Go",
	}); err != nil {
		t.Fatalf("Record(trend zero time) error = %v", err)
	}

	entries, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("len(entries) = %d, want 4", len(entries))
	}

	byID := make(map[string]HistoryEntry, len(entries))
	for _, e := range entries {
		byID[e.ID] = e
	}

	userEntry, ok := byID["user:torvalds"]
	if !ok {
		t.Fatalf("missing canonical user:torvalds in %+v", byID)
	}
	if userEntry.VisitCount != 2 {
		t.Errorf("user VisitCount = %d, want 2", userEntry.VisitCount)
	}
	if userEntry.Label != "Linus Torvalds" || userEntry.Subtitle != "Linux creator" {
		t.Errorf("user merged fields unexpected: %+v", userEntry)
	}
	if !userEntry.LastVisited.Equal(t2) {
		t.Errorf("user LastVisited = %v, want %v", userEntry.LastVisited, t2)
	}

	repoEntry, ok := byID["repo:charmbracelet/bubbletea"]
	if !ok {
		t.Fatalf("missing canonical repo:charmbracelet/bubbletea in %+v", byID)
	}
	if repoEntry.VisitCount != 2 {
		t.Errorf("repo VisitCount = %d, want 2", repoEntry.VisitCount)
	}

	trendAll, ok := byID["trend:daily:all"]
	if !ok {
		t.Fatalf("missing canonical trend:daily:all in %+v", byID)
	}
	if trendAll.VisitCount != 2 {
		t.Errorf("trend:daily:all VisitCount = %d, want 2", trendAll.VisitCount)
	}

	trendGo, ok := byID["trend:weekly:go"]
	if !ok {
		t.Fatalf("missing canonical trend:weekly:go in %+v", byID)
	}
	if trendGo.LastVisited.IsZero() {
		t.Errorf("expected non-zero LastVisited for trend:weekly:go")
	}

	// Ensure no leftover .tmp or .bak files exist in the directory.
	dirEntries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ReadDir error = %v", err)
	}
	for _, de := range dirEntries {
		if de.Name() != "history.json" {
			t.Errorf("unexpected leftover file in store dir: %s", de.Name())
		}
	}
}

func TestHistoryStore_CapAt50(t *testing.T) {
	store := NewHistoryStore(filepath.Join(t.TempDir(), "history.json"))
	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 65; i++ {
		err := store.Record(HistoryEntry{
			Type:        HistoryTypeUser,
			Target:      fmt.Sprintf("user%02d", i),
			Label:       fmt.Sprintf("user%02d", i),
			LastVisited: base.Add(time.Duration(i) * time.Minute),
		})
		if err != nil {
			t.Fatalf("Record(%d) error = %v", i, err)
		}
	}

	entries, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(entries) != maxHistoryEntries {
		t.Fatalf("len(entries) = %d, want %d", len(entries), maxHistoryEntries)
	}
	if entries[0].ID != "user:user64" {
		t.Errorf("newest entry ID = %q, want user:user64", entries[0].ID)
	}
	if entries[len(entries)-1].ID != "user:user15" {
		t.Errorf("oldest retained entry ID = %q, want user:user15", entries[len(entries)-1].ID)
	}
}

func TestHistoryStore_DeleteAndClear(t *testing.T) {
	store := NewHistoryStore(filepath.Join(t.TempDir(), "history.json"))

	// Delete on non-existent file should not error.
	if err := store.Delete("user:missing"); err != nil {
		t.Fatalf("Delete on missing file error = %v", err)
	}

	t1 := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	if err := store.Record(HistoryEntry{Type: HistoryTypeUser, Target: "alice", LastVisited: t1}); err != nil {
		t.Fatalf("Record(alice) error = %v", err)
	}
	if err := store.Record(HistoryEntry{Type: HistoryTypeUser, Target: "bob", LastVisited: t1.Add(time.Minute)}); err != nil {
		t.Fatalf("Record(bob) error = %v", err)
	}

	// Delete missing ID should not remove anything or error.
	if err := store.Delete("user:charlie"); err != nil {
		t.Fatalf("Delete(missing) error = %v", err)
	}
	if err := store.Delete(""); err != nil {
		t.Fatalf("Delete(empty) error = %v", err)
	}

	// Delete alice.
	if err := store.Delete("user:alice"); err != nil {
		t.Fatalf("Delete(alice) error = %v", err)
	}
	entries, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "user:bob" {
		t.Fatalf("after Delete(alice), entries = %+v, want only user:bob", entries)
	}

	// Clear should persist empty JSON array [].
	if err := store.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	raw, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	if strings.TrimSpace(string(raw)) != "[]" {
		t.Fatalf("after Clear(), file content = %q, want []", strings.TrimSpace(string(raw)))
	}
	entries, err = store.Load()
	if err != nil {
		t.Fatalf("Load() after Clear error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("Load() after Clear len = %d, want 0", len(entries))
	}
}

func TestParseRepoSlug(t *testing.T) {
	maxOwner := strings.Repeat("a", 39)
	maxRepo := strings.Repeat("b", 100)

	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
		wantOK    bool
	}{
		{name: "simple slug", input: "charmbracelet/bubbletea", wantOwner: "charmbracelet", wantRepo: "bubbletea", wantOK: true},
		{name: "slug with whitespace", input: "  charmbracelet/bubbletea  ", wantOwner: "charmbracelet", wantRepo: "bubbletea", wantOK: true},
		{name: "slug with dots and underscores", input: "org.name_1/repo-name.v2", wantOwner: "org.name_1", wantRepo: "repo-name.v2", wantOK: true},
		{name: "slug with .git", input: "owner/repo.git", wantOwner: "owner", wantRepo: "repo", wantOK: true},
		{name: "github.com prefix", input: "github.com/charmbracelet/bubbletea", wantOwner: "charmbracelet", wantRepo: "bubbletea", wantOK: true},
		{name: "github.com with subpath", input: "github.com/charmbracelet/bubbletea/tree/main", wantOwner: "charmbracelet", wantRepo: "bubbletea", wantOK: true},
		{name: "https url", input: "https://github.com/charmbracelet/bubbletea", wantOwner: "charmbracelet", wantRepo: "bubbletea", wantOK: true},
		{name: "https url with .git and path", input: "https://github.com/charmbracelet/bubbletea.git/issues/1", wantOwner: "charmbracelet", wantRepo: "bubbletea", wantOK: true},
		{name: "https url mixed case host", input: "HTTPS://GitHub.com/Owner/Repo/", wantOwner: "Owner", wantRepo: "Repo", wantOK: true},
		{name: "max lengths", input: maxOwner + "/" + maxRepo, wantOwner: maxOwner, wantRepo: maxRepo, wantOK: true},

		// Rejections
		{name: "empty", input: "", wantOK: false},
		{name: "single segment", input: "torvalds", wantOK: false},
		{name: "bare three segments", input: "a/b/c", wantOK: false},
		{name: "dot owner", input: "./repo", wantOK: false},
		{name: "dotdot owner", input: "../repo", wantOK: false},
		{name: "dot repo", input: "owner/.", wantOK: false},
		{name: "dotdot repo", input: "owner/..", wantOK: false},
		{name: "only .git repo", input: "owner/.git", wantOK: false},
		{name: "url traversal", input: "https://github.com/owner/repo/../../etc/passwd", wantOK: false},
		{name: "url encoded traversal", input: "https://github.com/%2e%2e/repo", wantOK: false},
		{name: "double dot in repo", input: "owner/foo..bar", wantOK: false},
		{name: "backslash", input: `owner\repo`, wantOK: false},
		{name: "leading slash", input: "/owner/repo", wantOK: false},
		{name: "non github host", input: "https://gitlab.com/owner/repo", wantOK: false},
		{name: "ssh scheme", input: "ssh://github.com/owner/repo", wantOK: false},
		{name: "userinfo in url", input: "https://user@github.com/owner/repo", wantOK: false},
		{name: "owner too long", input: strings.Repeat("a", 40) + "/repo", wantOK: false},
		{name: "repo too long", input: "owner/" + strings.Repeat("b", 101), wantOK: false},
		{name: "invalid chars space", input: "owner/my repo", wantOK: false},
		{name: "invalid chars query in bare", input: "owner/repo?foo=bar", wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			owner, repo, ok := ParseRepoSlug(tc.input)
			if ok != tc.wantOK {
				t.Fatalf("ParseRepoSlug(%q) ok = %v, want %v (got %q/%q)", tc.input, ok, tc.wantOK, owner, repo)
			}
			if ok && (owner != tc.wantOwner || repo != tc.wantRepo) {
				t.Fatalf("ParseRepoSlug(%q) = (%q, %q), want (%q, %q)", tc.input, owner, repo, tc.wantOwner, tc.wantRepo)
			}
		})
	}
}

func TestParseUserTarget(t *testing.T) {
	maxUser := strings.Repeat("u", 39)

	tests := []struct {
		name     string
		input    string
		wantUser string
		wantOK   bool
	}{
		{name: "plain username", input: "torvalds", wantUser: "torvalds", wantOK: true},
		{name: "at prefix", input: "@torvalds", wantUser: "torvalds", wantOK: true},
		{name: "whitespace trimmed", input: "  @Octo-Cat99  ", wantUser: "Octo-Cat99", wantOK: true},
		{name: "https url", input: "https://github.com/torvalds", wantUser: "torvalds", wantOK: true},
		{name: "https url trailing slash", input: "https://github.com/torvalds/", wantUser: "torvalds", wantOK: true},
		{name: "double hyphen allowed", input: "foo--bar", wantUser: "foo--bar", wantOK: true},
		{name: "max length 39", input: maxUser, wantUser: maxUser, wantOK: true},

		// Rejections
		{name: "empty", input: "", wantOK: false},
		{name: "just at", input: "@", wantOK: false},
		{name: "leading hyphen", input: "-torvalds", wantOK: false},
		{name: "trailing hyphen", input: "torvalds-", wantOK: false},
		{name: "dot", input: ".", wantOK: false},
		{name: "dotdot", input: "..", wantOK: false},
		{name: "traversal", input: "../torvalds", wantOK: false},
		{name: "repo slug rejected", input: "torvalds/linux", wantOK: false},
		{name: "url with two segments rejected", input: "https://github.com/torvalds/linux", wantOK: false},
		{name: "non github url", input: "https://gitlab.com/torvalds", wantOK: false},
		{name: "backslash", input: `\torvalds`, wantOK: false},
		{name: "too long 40", input: strings.Repeat("u", 40), wantOK: false},
		{name: "invalid chars", input: "user_name", wantOK: false},
		{name: "double at", input: "@@torvalds", wantOK: false},
		{name: "at url", input: "@https://github.com/torvalds", wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			user, ok := ParseUserTarget(tc.input)
			if ok != tc.wantOK {
				t.Fatalf("ParseUserTarget(%q) ok = %v, want %v (got %q)", tc.input, ok, tc.wantOK, user)
			}
			if ok && user != tc.wantUser {
				t.Fatalf("ParseUserTarget(%q) = %q, want %q", tc.input, user, tc.wantUser)
			}
		})
	}
}

func TestHistoryStore_ConcurrentRace(t *testing.T) {
	store := NewHistoryStore(filepath.Join(t.TempDir(), "history.json"))

	const workers = 8
	const opsPerWorker = 15

	var wg sync.WaitGroup
	errCh := make(chan error, workers*opsPerWorker*3)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < opsPerWorker; i++ {
				target := fmt.Sprintf("user-%d", (workerID+i)%10)
				if err := store.Record(HistoryEntry{
					Type:   HistoryTypeUser,
					Target: target,
					Label:  target,
				}); err != nil {
					errCh <- fmt.Errorf("Record(%s): %w", target, err)
				}

				if _, err := store.Load(); err != nil {
					errCh <- fmt.Errorf("Load(): %w", err)
				}

				if i%4 == 0 {
					if err := store.Delete("user:" + target); err != nil {
						errCh <- fmt.Errorf("Delete(%s): %w", target, err)
					}
				}
			}
		}(w)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent operation error: %v", err)
	}

	entries, err := store.Load()
	if err != nil {
		t.Fatalf("final Load() error = %v", err)
	}
	if len(entries) > maxHistoryEntries {
		t.Fatalf("final len(entries) = %d, exceeds max %d", len(entries), maxHistoryEntries)
	}
}

func TestHistoryStore_RecoversCorruptFileAndSchemeLessUserURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := os.WriteFile(path, []byte("{corrupt"), 0644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	store := NewHistoryStore(path)
	if err := store.Record(HistoryEntry{
		Type:   HistoryTypeUser,
		Target: "github.com/octocat",
		Label:  "@octocat",
	}); err != nil {
		t.Fatalf("Record after corrupt file failed: %v", err)
	}
	if _, err := os.Stat(path + ".corrupt"); err != nil {
		t.Fatalf("expected quarantined .corrupt backup, stat err=%v", err)
	}
	entries, err := store.Load()
	if err != nil || len(entries) != 1 || entries[0].ID != "user:octocat" {
		t.Fatalf("expected recovered history with user:octocat, got entries=%+v err=%v", entries, err)
	}

	if u, ok := ParseUserTarget("github.com/torvalds"); !ok || u != "torvalds" {
		t.Fatalf("expected ParseUserTarget scheme-less URL to return torvalds, got %q ok=%v", u, ok)
	}
	if _, _, ok := ParseRepoSlug("owner/repo.git.git"); ok {
		t.Fatalf("expected double .git suffix owner/repo.git.git to be rejected")
	}

	// Numeric timestamp fails Time.UnmarshalJSON and should be quarantined and recovered on Record.
	badTSPath := filepath.Join(t.TempDir(), "history.json")
	if err := os.WriteFile(badTSPath, []byte(`[{"id":"user:a","type":"USER","target":"a","last_visited":123}]`), 0644); err != nil {
		t.Fatalf("WriteFile bad timestamp error = %v", err)
	}
	badTSStore := NewHistoryStore(badTSPath)
	if err := badTSStore.Record(HistoryEntry{Type: HistoryTypeUser, Target: "alice", Label: "@alice"}); err != nil {
		t.Fatalf("Record after corrupt timestamp failed: %v", err)
	}

	// Stale generation write after Clear must not resurrect cleared history.
	genPath := filepath.Join(t.TempDir(), "gen-history.json")
	genStore := NewHistoryStore(genPath)
	genBefore := genStore.CurrentGen()
	if err := genStore.Clear(); err != nil {
		t.Fatalf("Clear error = %v", err)
	}
	if err := genStore.RecordIfGen(HistoryEntry{Type: HistoryTypeUser, Target: "stale", Label: "@stale"}, genBefore); err != nil {
		t.Fatalf("RecordIfGen stale error = %v", err)
	}
	genEntries, err := genStore.Load()
	if err != nil || len(genEntries) != 0 {
		t.Fatalf("expected stale RecordIfGen to be skipped after Clear, got %+v err=%v", genEntries, err)
	}
}
