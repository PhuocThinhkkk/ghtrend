package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PhuocThinhkkk/ghtrend/pkg/cache"
	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func execBatchMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, sub := range batch {
			out = append(out, execBatchMsgs(sub)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

func testSampleRepos(owner, name string) []Repo {
	return []Repo{
		{
			Owner:         owner,
			Name:          name,
			Language:      "Go",
			Stars:         "1200",
			Url:           "https://github.com/" + owner + "/" + name,
			DetailsLoaded: true,
			ReadMe:        "# " + name + "\n\nSample readme",
		},
	}
}

func newTestHomeModel(store *cache.HistoryStore) Model {
	cfg := &flags.CmdConfig{
		Since:      flags.Daily,
		Language:   "All",
		Limit:      2202,
		LaunchHome: true,
		IsCache:    true,
	}
	opts := LaunchOptions{
		Config:       cfg,
		HistoryStore: store,
		FetchTrending: func(since flags.Frequency, language string, useCache bool) (RepoFetchResult, error) {
			return RepoFetchResult{Repos: testSampleRepos("org", "trend-"+string(since)+"-"+strings.ToLower(language))}, nil
		},
		FetchUser: func(username, language string, useCache bool) (RepoFetchResult, error) {
			repos := testSampleRepos(username, "profile-repo")
			repos[0].UserProfile = &ghclient.UserProfile{
				Username: username,
				Name:     "Tester " + username,
			}
			return RepoFetchResult{Repos: repos}, nil
		},
		FetchDirectRepo: func(owner, repo string) ([]Repo, error) {
			return testSampleRepos(owner, repo), nil
		},
	}
	return Model{
		state:      stateHome,
		cfg:        cfg,
		theme:      theme.GetTheme("catppuccin"),
		width:      120,
		height:     34,
		launchOpts: opts,
		home:       newHomeModel(cfg),
	}
}

func TestHomeView_NeverExceedsBoundsAllThemes(t *testing.T) {
	sizes := []struct {
		w int
		h int
	}{
		{60, 14},
		{80, 24},
		{100, 30},
		{140, 40},
	}
	sampleHistory := []cache.HistoryEntry{
		{ID: "repo:charmbracelet/bubbletea", Type: cache.HistoryTypeRepo, Target: "charmbracelet/bubbletea", Label: "charmbracelet/bubbletea", LastVisited: time.Now().Add(-5 * time.Minute), VisitCount: 3},
		{ID: "user:torvalds", Type: cache.HistoryTypeUser, Target: "torvalds", Label: "@torvalds", LastVisited: time.Now().Add(-2 * time.Hour), VisitCount: 1},
		{ID: "trend:weekly:go", Type: cache.HistoryTypeTrend, Target: "weekly/Go", Label: "Weekly Trending (Go)", Frequency: flags.Weekly, Language: "Go", LastVisited: time.Now().Add(-24 * time.Hour)},
	}

	for _, th := range theme.Catalog {
		for _, sz := range sizes {
			for _, withInput := range []bool{false, true} {
				m := newTestHomeModel(nil)
				m.theme = th
				m.width = sz.w
				m.height = sz.h
				if withInput {
					m.home.history = sampleHistory
					m.home.beginInput(homeInputRepo, "owner/repo", "charmbracelet/lipgloss")
				}
				out := m.View()
				lines := strings.Split(out, "\n")
				if len(lines) > sz.h {
					t.Fatalf("theme=%s size=%dx%d input=%v height=%d > %d", th.ID, sz.w, sz.h, withInput, len(lines), sz.h)
				}
				for i, line := range lines {
					if lw := lipgloss.Width(line); lw > sz.w {
						t.Fatalf("theme=%s size=%dx%d input=%v line %d width=%d > %d", th.ID, sz.w, sz.h, withInput, i+1, lw, sz.w)
					}
				}
			}
		}
	}
}

func TestHomeInput_ProtectsGlobalHotkeys(t *testing.T) {
	m := newTestHomeModel(nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	m = updated.(Model)
	if !m.home.isInputActive() {
		t.Fatalf("expected 'u' shortcut to activate username input")
	}

	for _, ch := range []rune{'q', 'H', 't', 'h', '?'} {
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(Model)
		for _, sub := range execBatchMsgs(cmd) {
			if _, isQuit := sub.(tea.QuitMsg); isQuit {
				t.Fatalf("typing %q should not quit during text input", ch)
			}
		}
		if m.showThemeModal || m.showHelpModal {
			t.Fatalf("typing %q opened modal during text input", ch)
		}
		if m.state != stateHome {
			t.Fatalf("typing %q left home state", ch)
		}
	}
	if !strings.Contains(m.home.input.Value(), "qHth?") {
		t.Fatalf("expected input to capture typed hotkeys, got %q", m.home.input.Value())
	}

	// Cancel returns cleanly without leaving home
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.home.isInputActive() {
		t.Fatalf("expected Esc to cancel home input")
	}
}

func TestHomeHub_LaunchTrendingAndValidateInputs(t *testing.T) {
	store := cache.NewHistoryStore(filepath.Join(t.TempDir(), "history.json"))
	m := newTestHomeModel(store)

	// Cycle trending to Weekly and set Language = "Go"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = updated.(Model)
	if m.home.trendSince != flags.Weekly {
		t.Fatalf("expected Weekly, got %v", m.home.trendSince)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = updated.(Model)
	for _, r := range "Go" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.home.trendLang != "Go" {
		t.Fatalf("expected trendLang=Go, got %q", m.home.trendLang)
	}

	// Launch Trending
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.state != stateLoading {
		t.Fatalf("expected stateLoading after launch, got %v", m.state)
	}
	for _, msg := range execBatchMsgs(cmd) {
		if _, ok := msg.(reposLoadedMsg); ok {
			updated, postCmd := m.Update(msg)
			m = updated.(Model)
			for _, hmsg := range execBatchMsgs(postCmd) {
				updated, _ = m.Update(hmsg)
				m = updated.(Model)
			}
		}
	}
	if m.state != stateDashboard || m.cfg.Since != flags.Weekly || m.cfg.Language != "Go" {
		t.Fatalf("expected Weekly Go dashboard, state=%v since=%v lang=%q", m.state, m.cfg.Since, m.cfg.Language)
	}
	if len(m.home.history) != 1 || m.home.history[0].Type != cache.HistoryTypeTrend {
		t.Fatalf("expected recorded TREND history entry, got %+v", m.home.history)
	}

	// Validate invalid user rejection keeps input open with error
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}})
	m = updated.(Model)
	if m.state != stateHome {
		t.Fatalf("expected H to return to stateHome, got %v", m.state)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	m = updated.(Model)
	for _, r := range "bad user!" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !m.home.isInputActive() || m.home.inputError == "" {
		t.Fatalf("expected invalid username to stay in input with error")
	}

	// Valid URL in Repo launcher opens charmbracelet/bubbletea
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updated.(Model)
	for _, r := range "https://github.com/charmbracelet/bubbletea" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	for _, msg := range execBatchMsgs(cmd) {
		if _, ok := msg.(reposLoadedMsg); ok {
			updated, _ = m.Update(msg)
			m = updated.(Model)
		}
	}
	if m.state != stateDashboard || m.cfg.DirectRepo != "charmbracelet/bubbletea" {
		t.Fatalf("expected DirectRepo dashboard, state=%v direct=%q", m.state, m.cfg.DirectRepo)
	}
}

func TestHomeHistory_StaleRefreshDeleteClearAndProfileReset(t *testing.T) {
	store := cache.NewHistoryStore(filepath.Join(t.TempDir(), "history.json"))
	_ = store.Record(cache.HistoryEntry{
		Type:      cache.HistoryTypeTrend,
		Target:    "daily/All",
		Label:     "Daily Trending (All)",
		Frequency: flags.Daily,
		Language:  "All",
	})
	_ = store.Record(cache.HistoryEntry{
		Type:   cache.HistoryTypeUser,
		Target: "torvalds",
		Label:  "@torvalds",
	})

	refreshedCalled := false
	m := newTestHomeModel(store)
	m.launchOpts.FetchUser = func(username, language string, useCache bool) (RepoFetchResult, error) {
		repos := testSampleRepos(username, "linux")
		repos[0].UserProfile = &ghclient.UserProfile{Username: username, Name: "Linus Torvalds"}
		return RepoFetchResult{Repos: repos, FromCache: true, Stale: true}, nil
	}
	m.launchOpts.RefreshUser = func(username, language string) ([]Repo, error) {
		refreshedCalled = true
		repos := testSampleRepos(username, "linux")
		repos = append(repos, testSampleRepos(username, "subsurface")[0])
		repos[0].UserProfile = &ghclient.UserProfile{Username: username, Name: "Linus Torvalds"}
		return repos, nil
	}

	// Init loads persisted history
	for _, msg := range execBatchMsgs(m.Init()) {
		updated, _ := m.Update(msg)
		m = updated.(Model)
	}
	if len(m.home.history) != 2 {
		t.Fatalf("expected 2 history entries loaded, got %d", len(m.home.history))
	}

	// Focus history pane and reopen most recent (@torvalds) which returns stale cache
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	var bgMsgs []tea.Msg
	for _, msg := range execBatchMsgs(cmd) {
		if _, ok := msg.(reposLoadedMsg); ok {
			updated, postCmd := m.Update(msg)
			m = updated.(Model)
			bgMsgs = append(bgMsgs, execBatchMsgs(postCmd)...)
		}
	}
	if m.userProfile == nil || !m.refreshingStale {
		t.Fatalf("expected userProfile set and refreshingStale=true, got profile=%v refreshing=%v", m.userProfile, m.refreshingStale)
	}

	// Deliver background refresh result
	for _, bmsg := range bgMsgs {
		if rl, ok := bmsg.(reposLoadedMsg); ok && rl.isBackgroundRefresh {
			updated, _ = m.Update(rl)
			m = updated.(Model)
		}
	}
	if !refreshedCalled || m.refreshingStale || len(m.repoList) != 2 {
		t.Fatalf("expected background refresh applied (called=%v refreshing=%v repos=%d)", refreshedCalled, m.refreshingStale, len(m.repoList))
	}

	// Return Home with '~', then open Trending entry -> must clear m.userProfile
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'~'}})
	m = updated.(Model)
	if m.state != stateHome {
		t.Fatalf("expected '~' to return Home, got %v", m.state)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	for _, msg := range execBatchMsgs(cmd) {
		if _, ok := msg.(reposLoadedMsg); ok {
			updated, _ = m.Update(msg)
			m = updated.(Model)
		}
	}
	if m.userProfile != nil {
		t.Fatalf("expected m.userProfile to be cleared when switching to Trending, got %+v", m.userProfile)
	}

	// Back to Home -> delete selected history item with 'd', then clear all with 'D'
	updated, initCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}})
	m = updated.(Model)
	for _, msg := range execBatchMsgs(initCmd) {
		updated, _ = m.Update(msg)
		m = updated.(Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = updated.(Model)
	updated, delCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(Model)
	for _, msg := range execBatchMsgs(delCmd) {
		updated, _ = m.Update(msg)
		m = updated.(Model)
	}
	if len(m.home.history) != 1 {
		t.Fatalf("expected 1 history item after delete, got %d", len(m.home.history))
	}

	updated, clearCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	m = updated.(Model)
	for _, msg := range execBatchMsgs(clearCmd) {
		updated, _ = m.Update(msg)
		m = updated.(Model)
	}
	if len(m.home.history) != 0 {
		t.Fatalf("expected 0 history items after clear, got %d", len(m.home.history))
	}
}

func TestHome_AsyncSeqDropsStaleResultsAndPreservesDetails(t *testing.T) {
	m := newTestHomeModel(nil)
	m.launchSeq = 1

	// Launch Daily Trending (launchSeq becomes 2)
	updated, cmd1 := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	seq1Msgs := execBatchMsgs(cmd1)

	// Go Home immediately before seq1 response arrives -> launchSeq becomes 3, toast clears
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}})
	m = updated.(Model)
	if m.state != stateHome || m.launchSeq != 3 {
		t.Fatalf("expected stateHome with launchSeq=3, got state=%v seq=%d", m.state, m.launchSeq)
	}

	// Stale seq1 reposLoadedMsg arrives while back on Home -> must be discarded!
	for _, msg := range seq1Msgs {
		if rl, ok := msg.(reposLoadedMsg); ok {
			updated, _ = m.Update(rl)
			m = updated.(Model)
		}
	}
	if m.state != stateHome {
		t.Fatalf("stale reposLoadedMsg illegally transitioned Home to %v", m.state)
	}

	// Launch DirectRepo with loaded ExtraInfor/LanguagesBreakDown/RecentCommits, then simulate stale refresh
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updated.(Model)
	for _, r := range "charmbracelet/lipgloss" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	m.launchOpts.FetchDirectRepo = func(owner, repo string) ([]Repo, error) {
		r := testSampleRepos(owner, repo)[0]
		r.ExtraInfor = ExtraInfor{Issues: "12", TotalCommits: 42}
		r.LanguagesBreakDown = map[string]string{"Go": "100%"}
		r.RecentCommits = []ghclient.Commit{{Hash: "abc1234", Message: "init"}}
		return []Repo{r}, nil
	}
	updated, directCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	for _, msg := range execBatchMsgs(directCmd) {
		if rl, ok := msg.(reposLoadedMsg); ok {
			updated, _ = m.Update(rl)
			m = updated.(Model)
		}
	}

	// Simulate background refresh returning summary without DetailsLoaded
	freshSummary := Repo{Owner: "charmbracelet", Name: "lipgloss", Stars: "9999"}
	updated, _ = m.Update(reposLoadedMsg{
		launchSeq:           m.launchSeq,
		targetID:            m.activeTargetID,
		repos:               []Repo{freshSummary},
		isBackgroundRefresh: true,
	})
	m = updated.(Model)
	if len(m.repoList) != 1 || !m.repoList[0].DetailsLoaded || m.repoList[0].ExtraInfor.TotalCommits != 42 || len(m.repoList[0].RecentCommits) != 1 {
		t.Fatalf("expected applyRefreshedRepos to preserve ExtraInfor & RecentCommits, got %+v", m.repoList)
	}
	// Wrong-seq details/pagination must be ignored
	updated, _ = m.Update(repoDetailsLoadedMsg{launchSeq: m.launchSeq - 1, owner: "charmbracelet", name: "lipgloss", err: tea.ErrProgramKilled})
	m = updated.(Model)
	if strings.Contains(m.toastMessage, "Failed to load details") {
		t.Fatalf("stale repoDetailsLoadedMsg leaked error toast")
	}

	// Status bar in DirectRepo mode renders 📦 pill, not 📅 Daily
	sb := m.renderStatusBar(140)
	if !strings.Contains(sb, "charmbracelet/lipgloss") || strings.Contains(sb, "📅 Daily") {
		t.Fatalf("unexpected DirectRepo status bar: %q", sb)
	}
}

func TestHelpModal_ContextualAndFits80x24(t *testing.T) {
	m := newTestHomeModel(nil)
	m.width = 80
	m.height = 24
	m.showHelpModal = true
	homeHelp := m.View()
	if lipgloss.Height(homeHelp) > 24 || lipgloss.Width(homeHelp) > 80 {
		t.Fatalf("home help modal exceeds 80x24: %dx%d", lipgloss.Width(homeHelp), lipgloss.Height(homeHelp))
	}
	if !strings.Contains(homeHelp, "HOME NAVIGATION") || !strings.Contains(homeHelp, "[Esc] [?] [q] Close") {
		t.Fatalf("expected HOME NAVIGATION & close hint in home help modal")
	}

	m.state = stateDashboard
	m.repoList = testSampleRepos("org", "repo")
	m.filteredRepos = m.repoList
	dashHelp := m.View()
	if lipgloss.Height(dashHelp) > 24 || lipgloss.Width(dashHelp) > 80 {
		t.Fatalf("dashboard help modal exceeds 80x24: %dx%d", lipgloss.Width(dashHelp), lipgloss.Height(dashHelp))
	}
	if !strings.Contains(dashHelp, "[Esc] [?] [q] Close") {
		t.Fatalf("expected dashboard help close hint to remain unclipped at 80x24")
	}
}
