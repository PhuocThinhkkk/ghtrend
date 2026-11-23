package ghclient

import (
	"errors"
	"testing"

	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
)

const sampleUserReposHTML = `
<div id="user-repositories-list">
  <ul>
    <li class="col-12 border-bottom public source" itemprop="owns" itemscope itemtype="http://schema.org/Code">
      <div class="col-10">
        <h3>
          <a href="/octocat/Hello-World" itemprop="name codeRepository">Hello-World</a>
        </h3>
        <p itemprop="description">My first open source project</p>
        <div>
          <span itemprop="programmingLanguage">Go</span>
          <a href="/octocat/Hello-World/stargazers"> 42 </a>
          <a href="/octocat/Hello-World/forks"> 12 </a>
        </div>
      </div>
    </li>
    <li class="col-12 border-bottom public fork" itemprop="owns" itemscope itemtype="http://schema.org/Code">
      <div class="col-10">
        <h3>
          <a href="/octocat/Spoon-Knife" itemprop="name codeRepository">Spoon-Knife</a>
        </h3>
      </div>
    </li>
  </ul>
</div>
`

func TestParseUserRepositoriesPage_Success(t *testing.T) {
	repos, err := ParseUserRepositoriesPage(sampleUserReposHTML)
	if err != nil {
		t.Fatalf("unexpected error parsing user repos page: %v", err)
	}

	if len(repos) != 2 {
		t.Fatalf("expected 2 repositories, got %d", len(repos))
	}

	r1 := repos[0]
	if r1.Name != "Hello-World" {
		t.Errorf("expected name 'Hello-World', got '%s'", r1.Name)
	}
	if r1.Owner != "octocat" {
		t.Errorf("expected owner 'octocat', got '%s'", r1.Owner)
	}
	if r1.Language != "Go" {
		t.Errorf("expected language 'Go', got '%s'", r1.Language)
	}
	if r1.Description != "My first open source project" {
		t.Errorf("expected description 'My first open source project', got '%s'", r1.Description)
	}
	if r1.Stars != "42" {
		t.Errorf("expected stars '42', got '%s'", r1.Stars)
	}
	if r1.Forks != "12" {
		t.Errorf("expected forks '12', got '%s'", r1.Forks)
	}
	if r1.Url != "https://github.com/octocat/Hello-World" {
		t.Errorf("expected URL 'https://github.com/octocat/Hello-World', got '%s'", r1.Url)
	}

	r2 := repos[1]
	if r2.Name != "Spoon-Knife" {
		t.Errorf("expected name 'Spoon-Knife', got '%s'", r2.Name)
	}
	if r2.Owner != "octocat" {
		t.Errorf("expected owner 'octocat', got '%s'", r2.Owner)
	}
	if r2.Language != "" {
		t.Errorf("expected empty language, got '%s'", r2.Language)
	}
	if r2.Description != "" {
		t.Errorf("expected empty description, got '%s'", r2.Description)
	}
}

func TestParseUserRepositoriesPage_Empty(t *testing.T) {
	html := `<div id="user-repositories-list"><ul></ul></div>`
	repos, err := ParseUserRepositoriesPage(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(repos))
	}
}

func TestGetUserRepositories_EmptyUser(t *testing.T) {
	cfg := &flags.CmdConfig{
		User:  "",
		Limit: 10,
	}
	_, err := GetUserRepositories(cfg)
	if err == nil {
		t.Errorf("expected error for empty username, got nil")
	}
}

func TestGetUserRepositories_FetchError(t *testing.T) {
	oldHook := fetchUserReposHook
	defer func() { fetchUserReposHook = oldHook }()

	fetchUserReposHook = func(url string) ([]byte, error) {
		return nil, errors.New("network failure")
	}

	cfg := &flags.CmdConfig{
		User:  "nonexistent",
		Limit: 10,
	}

	_, err := GetUserRepositories(cfg)
	if err == nil {
		t.Errorf("expected error when fetch fails, got nil")
	}
}

func TestGetUserRepositories_NoReposFound(t *testing.T) {
	oldHook := fetchUserReposHook
	defer func() { fetchUserReposHook = oldHook }()

	fetchUserReposHook = func(url string) ([]byte, error) {
		return []byte(`<div id="user-repositories-list"><ul></ul></div>`), nil
	}

	cfg := &flags.CmdConfig{
		User:  "emptyuser",
		Limit: 10,
	}

	_, err := GetUserRepositories(cfg)
	if err == nil {
		t.Errorf("expected error when no repos found, got nil")
	}
}

func TestFetchUserRepositoriesPage_EmptyUser(t *testing.T) {
	_, _, err := FetchUserRepositoriesPage("", "All", 1)
	if err == nil {
		t.Errorf("expected error for empty user, got nil")
	}
}

func TestFetchUserRepositoriesPage_Success(t *testing.T) {
	oldHook := fetchUserReposHook
	defer func() { fetchUserReposHook = oldHook }()

	fetchUserReposHook = func(url string) ([]byte, error) {
		return []byte(sampleUserReposHTML), nil
	}

	repos, hasMore, err := FetchUserRepositoriesPage("octocat", "Go", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(repos))
	}
	if hasMore {
		t.Errorf("expected hasMore=false for 2 repos, got true")
	}
}
