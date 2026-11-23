package ghclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestFetchLanguagesFromAPI(t *testing.T) {
	mockData := map[string]int64{
		"Go":         8000,
		"TypeScript": 1500,
		"Shell":      500,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/languages" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockData)
	}))
	defer server.Close()

	origURL := githubAPIBaseURL
	githubAPIBaseURL = server.URL
	defer func() { githubAPIBaseURL = origURL }()

	langs, err := fetchLanguagesFromAPI("owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Total = 10,000 bytes
	// Go = 80.0%, TypeScript = 15.0%, Shell = 5.0%
	if langs["Go"] != "80.0%" {
		t.Errorf("expected Go 80.0%%, got %s", langs["Go"])
	}
	if langs["TypeScript"] != "15.0%" {
		t.Errorf("expected TypeScript 15.0%%, got %s", langs["TypeScript"])
	}
	if langs["Shell"] != "5.0%" {
		t.Errorf("expected Shell 5.0%%, got %s", langs["Shell"])
	}
}

func TestLoadLanguageBreakdown_ModernSSRSkeletonFallback(t *testing.T) {
	mockData := map[string]int64{
		"Python": 7200,
		"Rust":   2800,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockData)
	}))
	defer server.Close()

	origURL := githubAPIBaseURL
	githubAPIBaseURL = server.URL
	defer func() { githubAPIBaseURL = origURL }()

	// Modern GitHub HTML with SkeletonText (no li.d-inline)
	html := `<html><body>
		<h2 class="sectionHeading"><span>Languages</span></h2>
		<div class="prc-SkeletonText" style="width:100%"></div>
	</body></html>`

	r := &Repo{
		Owner:        "zhouxiaoka",
		Name:         "autoclip",
		Language:     "Python",
		HtmlPageTerm: html,
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 5)
	signal := make(chan struct{})

	wg.Add(1)
	go r.loadLanguageBreakdown(errChan, &wg, signal)
	close(signal)
	wg.Wait()

	if len(r.LanguagesBreakDown) != 2 {
		t.Fatalf("expected 2 languages, got %d", len(r.LanguagesBreakDown))
	}
	if r.LanguagesBreakDown["Python"] != "72.0%" {
		t.Errorf("expected Python 72.0%%, got %s", r.LanguagesBreakDown["Python"])
	}
	if r.LanguagesBreakDown["Rust"] != "28.0%" {
		t.Errorf("expected Rust 28.0%%, got %s", r.LanguagesBreakDown["Rust"])
	}
}

func TestLoadLanguageBreakdown_APIFailureFallbackToPrimary(t *testing.T) {
	// API returns 500 error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()

	origURL := githubAPIBaseURL
	githubAPIBaseURL = server.URL
	defer func() { githubAPIBaseURL = origURL }()

	r := &Repo{
		Owner:        "offline",
		Name:         "repo",
		Language:     "Python",
		HtmlPageTerm: "<div>no languages in html</div>",
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 5)
	signal := make(chan struct{})

	wg.Add(1)
	go r.loadLanguageBreakdown(errChan, &wg, signal)
	close(signal)
	wg.Wait()

	if len(r.LanguagesBreakDown) != 1 {
		t.Fatalf("expected 1 language in fallback, got %d", len(r.LanguagesBreakDown))
	}
	if r.LanguagesBreakDown["Python"] != "100%" {
		t.Errorf("expected Python 100%%, got %s", r.LanguagesBreakDown["Python"])
	}
}

func TestFetchLanguagesFromAPI_ValidationAndEmpty(t *testing.T) {
	// Empty inputs
	if _, err := fetchLanguagesFromAPI("", "repo"); err == nil {
		t.Errorf("expected error for empty owner")
	}
	if _, err := fetchLanguagesFromAPI("owner", ""); err == nil {
		t.Errorf("expected error for empty repo")
	}

	// Empty repo returning {}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int64{})
	}))
	defer server.Close()

	origURL := githubAPIBaseURL
	githubAPIBaseURL = server.URL
	defer func() { githubAPIBaseURL = origURL }()

	langs, err := fetchLanguagesFromAPI("owner", "empty")
	if err != nil {
		t.Fatalf("unexpected error for empty repo: %v", err)
	}
	if len(langs) != 0 {
		t.Errorf("expected 0 languages for empty repo, got %d", len(langs))
	}
}

func TestParseLanguagesBreakDown_GraphsLanguagesSelector(t *testing.T) {
	html := `<html><body>
		<a href="/owner/repo/graphs/languages">
			<span class="color-fg-default text-bold mr-1">Rust</span>
			<span>100.0%</span>
		</a>
	</body></html>`

	langs, err := parseLanguagesBreakDown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if langs["Rust"] != "100.0%" {
		t.Errorf("expected Rust 100.0%%, got %s", langs["Rust"])
	}

	// Guard against single span without percent
	badHTML := `<html><body>
		<a href="/owner/repo/graphs/languages">
			<span class="color-fg-default text-bold mr-1">Go</span>
		</a>
	</body></html>`

	badLangs, _ := parseLanguagesBreakDown(badHTML)
	if len(badLangs) != 0 {
		t.Errorf("expected 0 languages when percentage is missing, got %v", badLangs)
	}
}

func TestFetchCommitsFromAPI_Success(t *testing.T) {
	mockCommits := []map[string]interface{}{
		{
			"sha": "a1b2c3d4e5f67890",
			"commit": map[string]interface{}{
				"message": "feat: add pixel-block typography\n\nExtended body details",
				"author": map[string]interface{}{
					"date": "2026-11-23T10:00:00Z",
				},
			},
		},
		{
			"sha": "f9e8d7c6b5a43210",
			"commit": map[string]interface{}{
				"message": "fix: correct layout bounds",
				"author": map[string]interface{}{
					"date": "2026-11-23T09:00:00Z",
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/commits" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockCommits)
	}))
	defer server.Close()

	origURL := githubAPIBaseURL
	githubAPIBaseURL = server.URL
	defer func() { githubAPIBaseURL = origURL }()

	commits, err := fetchCommitsFromAPI("owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
	if commits[0].Hash != "a1b2c3d" {
		t.Errorf("expected short hash 'a1b2c3d', got %s", commits[0].Hash)
	}
	if commits[0].Message != "feat: add pixel-block typography" {
		t.Errorf("expected single-line message, got %q", commits[0].Message)
	}
}

func TestRepo_LoadDetails_AlreadyLoaded(t *testing.T) {
	r := &Repo{
		Owner:         "testowner",
		Name:          "testrepo",
		DetailsLoaded: true,
		ReadMe:        "cached readme",
	}

	// Should return immediately without making network calls
	err := r.LoadDetails()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ReadMe != "cached readme" {
		t.Errorf("expected cached readme to remain unchanged, got %q", r.ReadMe)
	}
}


