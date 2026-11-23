package ui

import (
	"strings"
	"testing"

	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
)

func TestBuildTableRows(t *testing.T) {
	repos := []Repo{
		{
			Owner:       "octocat",
			Name:        "Hello-World",
			Stars:       "42",
			Forks:       "10",
			Language:    "Go",
			Description: "Sample description",
			Pinned:      true,
		},
		{
			Owner:       "octocat",
			Name:        "Spoon-Knife",
			Stars:       "0",
			Forks:       "",
			Language:    "C",
			Description: "",
		},
	}

	// User mode
	userRows := BuildTableRows(repos, 10, true)
	if len(userRows) != 2 {
		t.Fatalf("expected 2 user rows, got %d", len(userRows))
	}
	if userRows[0][0] != "📌 Hello-World" {
		t.Errorf("expected pinned name '📌 Hello-World', got %q", userRows[0][0])
	}
	if userRows[0][1] != "Sample description" {
		t.Errorf("expected description, got %q", userRows[0][1])
	}
	if userRows[1][2] != "·" || userRows[1][3] != "·" {
		t.Errorf("expected '·' for zero stars and forks, got %q, %q", userRows[1][2], userRows[1][3])
	}

	// Trending mode
	trendRows := BuildTableRows(repos, 1, false)
	if len(trendRows) != 1 {
		t.Fatalf("expected 1 row due to limit, got %d", len(trendRows))
	}
	if trendRows[0][0] != "octocat" {
		t.Errorf("expected owner 'octocat', got %q", trendRows[0][0])
	}
}

func TestDebounceAndRepoDetailsLoadedMsg(t *testing.T) {
	th := theme.GetTheme("tokyonight")
	repos := []Repo{
		{
			Owner:         "user1",
			Name:          "repo1",
			DetailsLoaded: true,
			ReadMe:        "# Repo 1 Readme",
		},
		{
			Owner:         "user1",
			Name:          "repo2",
			DetailsLoaded: false,
			Description:   "Repo 2 description",
		},
	}

	m := Model{
		state:    stateDashboard,
		repoList: repos,
		theme:    th,
		table:    InitialTableWithMode(repos, 10, th, false),
		list:     InitialFileList(nil),
		viewport: viewport.New(70, 20),
		width:    80,
		height:   24,
	}

	// Move cursor to repo 2
	m.table.SetCursor(1)
	m.cursorDebounceID = 5

	// Mismatched debounce ID should be ignored
	ignoredMsg := debounceCursorMsg{id: 4, cursor: 1}
	newModel, cmd := m.Update(ignoredMsg)
	m = newModel.(Model)
	if cmd != nil {
		t.Errorf("expected nil cmd for mismatched debounce ID, got %v", cmd)
	}

	// Matching debounce ID dispatches fetchRepoDetailsCmd
	matchMsg := debounceCursorMsg{id: 5, cursor: 1}
	newModel, cmd = m.Update(matchMsg)
	m = newModel.(Model)
	if cmd == nil {
		t.Errorf("expected non-nil cmd for matching debounce ID")
	}

	// Deliver repoDetailsLoadedMsg
	loadedRepo := repos[1]
	loadedRepo.DetailsLoaded = true
	loadedRepo.ReadMe = "# Repo 2 Loaded Readme"

	detailsMsg := repoDetailsLoadedMsg{
		cursor: 1,
		owner:  "user1",
		name:   "repo2",
		repo:   loadedRepo,
	}

	newModel, _ = m.Update(detailsMsg)
	m = newModel.(Model)

	if !m.repoList[1].DetailsLoaded {
		t.Errorf("expected repoList[1].DetailsLoaded to be true")
	}
	if m.repoList[1].ReadMe != "# Repo 2 Loaded Readme" {
		t.Errorf("expected readme to be updated, got %q", m.repoList[1].ReadMe)
	}
}

func TestNextPageLoadedMsg_AppendAndPreserveCursor(t *testing.T) {
	th := theme.GetTheme("tokyonight")
	initialRepos := []Repo{
		{Owner: "user1", Name: "repo1"},
		{Owner: "user1", Name: "repo2"},
	}

	cfg := &flags.CmdConfig{
		User:  "user1",
		Limit: 2202,
	}

	m := Model{
		state:        stateDashboard,
		repoList:     initialRepos,
		cfg:          cfg,
		theme:        th,
		userProfile:  &ghclient.UserProfile{Username: "user1"},
		table:        InitialTableWithMode(initialRepos, 10, th, true),
		list:         InitialFileList(nil),
		viewport:     viewport.New(70, 20),
		currentPage:  1,
		hasMorePages: true,
		width:        80,
		height:       24,
	}

	m.table.SetCursor(1)

	nextPageRepos := []Repo{
		{Owner: "user1", Name: "repo3"},
		{Owner: "user1", Name: "repo4"},
	}

	msg := nextPageLoadedMsg{
		page:    2,
		repos:   nextPageRepos,
		hasMore: true,
	}

	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	if len(m.repoList) != 4 {
		t.Fatalf("expected 4 repos after pagination, got %d", len(m.repoList))
	}
	if m.currentPage != 2 {
		t.Errorf("expected currentPage 2, got %d", m.currentPage)
	}
	if !m.hasMorePages {
		t.Errorf("expected hasMorePages to remain true")
	}
	if m.table.Cursor() != 1 {
		t.Errorf("expected cursor to stay at 1, got %d", m.table.Cursor())
	}
	if len(m.table.Rows()) != 4 {
		t.Errorf("expected 4 table rows, got %d", len(m.table.Rows()))
	}
}

func TestStatusBar_LoadingMoreIndicator(t *testing.T) {
	th := theme.GetTheme("tokyonight")
	m := Model{
		state:           stateDashboard,
		theme:           th,
		loadingNextPage: true,
		cfg:             &flags.CmdConfig{User: "user1"},
		width:           100,
		height:          24,
	}

	bar := m.renderStatusBar(100)
	if !strings.Contains(bar, "Loading more...") {
		t.Errorf("expected status bar to contain 'Loading more...', got %q", bar)
	}
}

func TestRepoDetailsLoadedMsg_IdentityMatch(t *testing.T) {
	th := theme.GetTheme("tokyonight")
	repos := []Repo{
		{Owner: "user1", Name: "repo1", DetailsLoaded: true, ReadMe: "Repo 1 Readme"},
		{Owner: "user1", Name: "repo2", DetailsLoaded: false},
	}

	m := Model{
		state:    stateDashboard,
		repoList: repos,
		theme:    th,
		table:    InitialTableWithMode(repos, 10, th, false),
		list:     InitialFileList(nil),
		viewport: viewport.New(70, 20),
		width:    80,
		height:   24,
	}

	// Move cursor to repo2 (cursor = 1)
	m.table.SetCursor(1)

	// User types a filter selecting ONLY repo2 (cursor becomes 0 in filtered view)
	m.filteredRepos = []Repo{repos[1]}
	m.table = InitialTableWithMode(m.filteredRepos, 10, th, false)
	m.table.SetCursor(0)

	// Details arrive for repo2 (with original msg.cursor = 1)
	loadedRepo := repos[1]
	loadedRepo.DetailsLoaded = true
	loadedRepo.ReadMe = "# Repo 2 Readme After Filter"

	detailsMsg := repoDetailsLoadedMsg{
		cursor: 1, // original cursor before filter
		owner:  "user1",
		name:   "repo2",
		repo:   loadedRepo,
	}

	newModel, _ := m.Update(detailsMsg)
	m = newModel.(Model)

	// Viewport should update because the identity (user1/repo2) matches current cursor repo!
	if !strings.Contains(m.viewport.View(), "Repo 2 Readme") {
		t.Errorf("expected viewport to update based on identity match, got %q", m.viewport.View())
	}
}

func TestApplyFilter_SchedulesDebounceForUnloadedRepo(t *testing.T) {
	th := theme.GetTheme("tokyonight")
	repos := []Repo{
		{Owner: "user1", Name: "repo1", DetailsLoaded: true},
		{Owner: "user1", Name: "repo2", DetailsLoaded: false, Description: "unloaded repo"},
	}

	ti := textinput.New()
	ti.SetValue("repo2")

	m := Model{
		state:       stateDashboard,
		repoList:    repos,
		theme:       th,
		filterInput: ti,
		table:       InitialTableWithMode(repos, 10, th, false),
		list:        InitialFileList(nil),
		viewport:    viewport.New(70, 20),
		width:       80,
		height:      24,
	}

	cmd := m.applyFilter()
	if cmd == nil {
		t.Errorf("expected debounce command to be scheduled when filter selects an unloaded repo")
	}
}
