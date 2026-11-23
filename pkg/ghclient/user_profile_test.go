package ghclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScrapeUserProfile_Success(t *testing.T) {
	html := `<html><body>
		<span class="p-name">Linus Torvalds</span>
		<span class="p-nickname">torvalds</span>
		<div class="user-profile-bio">Creator of Linux and Git</div>
		<img class="avatar-user" src="https://avatars.githubusercontent.com/u/1024025?s=64&v=4" />
		<a href="/torvalds?tab=followers"><span class="text-bold">230k</span></a>
		<a href="/torvalds?tab=following"><span class="text-bold">0</span></a>
		<li itemprop="worksFor"><span>Linux Foundation</span></li>
		<li itemprop="homeLocation"><span>Portland, OR</span></li>
		<ol class="d-flex">
			<li class="pinned-item-list-item">
				<span class="repo">linux</span>
			</li>
			<li class="pinned-item-list-item">
				<span class="repo">GuitarPedal</span>
			</li>
		</ol>
	</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	origWeb := githubWebBaseURL
	githubWebBaseURL = server.URL
	defer func() { githubWebBaseURL = origWeb }()

	origClient := userProfileHTTPClient
	userProfileHTTPClient = server.Client()
	defer func() { userProfileHTTPClient = origClient }()

	profile := &UserProfile{Username: "torvalds"}
	err := scrapeUserProfile("torvalds", profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if profile.Name != "Linus Torvalds" {
		t.Errorf("expected name 'Linus Torvalds', got %q", profile.Name)
	}
	if profile.Bio != "Creator of Linux and Git" {
		t.Errorf("expected bio 'Creator of Linux and Git', got %q", profile.Bio)
	}
	if len(profile.PinnedRepos) != 2 || profile.PinnedRepos[0] != "linux" || profile.PinnedRepos[1] != "GuitarPedal" {
		t.Errorf("expected pinned repos [linux, GuitarPedal], got %v", profile.PinnedRepos)
	}
	if profile.Followers != "230k" {
		t.Errorf("expected followers '230k', got %q", profile.Followers)
	}
	if profile.Location != "Portland, OR" {
		t.Errorf("expected location 'Portland, OR', got %q", profile.Location)
	}
	if profile.Company != "Linux Foundation" {
		t.Errorf("expected company 'Linux Foundation', got %q", profile.Company)
	}
}

func TestFetchUserProfileFromAPI_Success(t *testing.T) {
	apiData := map[string]interface{}{
		"name":         "Ada Lovelace",
		"bio":          "First computer programmer",
		"avatar_url":   "https://example.com/avatar.png",
		"company":      "Analytical Engine Corp",
		"location":     "London, UK",
		"followers":    42000,
		"following":    10,
		"public_repos": 15,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(apiData)
	}))
	defer server.Close()

	origURL := githubAPIBaseURL
	githubAPIBaseURL = server.URL
	defer func() { githubAPIBaseURL = origURL }()

	profile := &UserProfile{Username: "ada"}
	err := fetchUserProfileFromAPI("ada", profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if profile.Name != "Ada Lovelace" {
		t.Errorf("expected name 'Ada Lovelace', got %q", profile.Name)
	}
	if profile.Bio != "First computer programmer" {
		t.Errorf("expected bio, got %q", profile.Bio)
	}
	if profile.Followers != "42000" {
		t.Errorf("expected followers '42000', got %q", profile.Followers)
	}
	if profile.PublicRepos != 15 {
		t.Errorf("expected 15 public repos, got %d", profile.PublicRepos)
	}
}

func TestGetUserProfile_EmptyUsername(t *testing.T) {
	_, err := GetUserProfile("")
	if err == nil {
		t.Errorf("expected error for empty username")
	}
}

func TestPinnedReposReordering(t *testing.T) {
	allRepos := RepoList{
		{Name: "regular-repo-1"},
		{Name: "my-pinned-project"},
		{Name: "regular-repo-2"},
		{Name: "cool-pinned-tool"},
	}

	profile := &UserProfile{
		Username:    "testuser",
		PinnedRepos: []string{"my-pinned-project", "cool-pinned-tool"},
	}

	pinnedMap := make(map[string]bool)
	for _, name := range profile.PinnedRepos {
		pinnedMap[strings.ToLower(name)] = true
	}

	var pinnedList RepoList
	var regularList RepoList

	for _, r := range allRepos {
		if pinnedMap[strings.ToLower(r.Name)] {
			r.Pinned = true
			pinnedList = append(pinnedList, r)
		} else {
			regularList = append(regularList, r)
		}
	}
	reordered := append(pinnedList, regularList...)

	if len(reordered) != 4 {
		t.Fatalf("expected 4 repos, got %d", len(reordered))
	}
	if reordered[0].Name != "my-pinned-project" || !reordered[0].Pinned {
		t.Errorf("expected first repo to be pinned 'my-pinned-project', got %q", reordered[0].Name)
	}
	if reordered[1].Name != "cool-pinned-tool" || !reordered[1].Pinned {
		t.Errorf("expected second repo to be pinned 'cool-pinned-tool', got %q", reordered[1].Name)
	}
	if reordered[2].Pinned || reordered[3].Pinned {
		t.Errorf("expected remaining repos not to be pinned")
	}
}
