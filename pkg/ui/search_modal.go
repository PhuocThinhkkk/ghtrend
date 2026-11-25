package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	searchModeRepos = "repos"
	searchModeUsers = "users"
)

// SearchService provides GitHub global search operations for the Global Search modal.
type SearchService struct {
	SearchRepos func(ctx context.Context, query string, page int) ([]ghclient.SearchRepoResult, bool, error)
	SearchUsers func(ctx context.Context, query string, page int) ([]ghclient.SearchUserResult, bool, error)
}

func defaultSearchService() SearchService {
	return SearchService{
		SearchRepos: ghclient.SearchRepositories,
		SearchUsers: ghclient.SearchUsers,
	}
}

func (m Model) activeSearchService() SearchService {
	if m.launchOpts.Search != nil {
		svc := *m.launchOpts.Search
		def := defaultSearchService()
		if svc.SearchRepos == nil {
			svc.SearchRepos = def.SearchRepos
		}
		if svc.SearchUsers == nil {
			svc.SearchUsers = def.SearchUsers
		}
		return svc
	}
	return defaultSearchService()
}

type globalSearchDebounceMsg struct {
	debounceID uint64
	seq        uint64
	mode       string
	query      string
}

type globalSearchResultsMsg struct {
	seq   uint64
	mode  string
	query string
	repos []ghclient.SearchRepoResult
	users []ghclient.SearchUserResult
	err   error
}

type searchModalState struct {
	open       bool
	mode       string
	input      textinput.Model
	debouncer  Debouncer
	hasDebounce bool
	seq        uint64
	loading    bool
	err        error
	repos      []ghclient.SearchRepoResult
	users      []ghclient.SearchUserResult
	cursor     int
	offset     int
}

func newSearchModalState() searchModalState {
	ti := textinput.New()
	ti.Placeholder = "Search GitHub repositories or users..."
	ti.Prompt = "🔍 "
	ti.CharLimit = 128
	ti.Width = 48
	return searchModalState{
		mode:        searchModeRepos,
		input:       ti,
		debouncer:   NewDebouncer(180 * time.Millisecond),
		hasDebounce: true,
	}
}

func (m *Model) openGlobalSearchModal() tea.Cmd {
	if !m.searchModal.hasDebounce {
		m.searchModal = newSearchModalState()
	}
	m.searchModal.open = true
	m.searchModal.err = nil
	m.searchModal.input.Width = max(24, min(60, m.width-16))
	m.searchModal.input.Focus()
	return textinput.Blink
}

func (m *Model) closeGlobalSearchModal() {
	m.searchModal.open = false
	m.searchModal.loading = false
	m.searchModal.seq++
	m.searchModal.debouncer.Cancel()
	m.searchModal.input.Blur()
}

func (m *Model) triggerDebouncedGlobalSearch() tea.Cmd {
	query := strings.TrimSpace(m.searchModal.input.Value())
	m.searchModal.seq++
	seq := m.searchModal.seq
	mode := m.searchModal.mode
	m.searchModal.err = nil
	m.searchModal.cursor = 0
	m.searchModal.offset = 0

	if query == "" {
		m.searchModal.loading = false
		m.searchModal.repos = nil
		m.searchModal.users = nil
		m.searchModal.debouncer.Cancel()
		return nil
	}

	m.searchModal.loading = true
	if !m.searchModal.hasDebounce {
		m.searchModal.debouncer = NewDebouncer(180 * time.Millisecond)
		m.searchModal.hasDebounce = true
	}
	return m.searchModal.debouncer.Schedule(func(id uint64) tea.Msg {
		return globalSearchDebounceMsg{debounceID: id, seq: seq, mode: mode, query: query}
	})
}

func executeGlobalSearchCmd(svc SearchService, seq uint64, mode, query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if mode == searchModeUsers {
			users, _, err := svc.SearchUsers(ctx, query, 1)
			return globalSearchResultsMsg{seq: seq, mode: mode, query: query, users: users, err: err}
		}
		repos, _, err := svc.SearchRepos(ctx, query, 1)
		return globalSearchResultsMsg{seq: seq, mode: mode, query: query, repos: repos, err: err}
	}
}

func (m *Model) updateGlobalSearchModal(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.closeGlobalSearchModal()
		return nil
	case "tab":
		if m.searchModal.mode == searchModeRepos {
			m.searchModal.mode = searchModeUsers
		} else {
			m.searchModal.mode = searchModeRepos
		}
		return m.triggerDebouncedGlobalSearch()
	case "up", "ctrl+p":
		if m.searchModal.cursor > 0 {
			m.searchModal.cursor--
			m.clampSearchModalCursor()
		}
		return nil
	case "down", "ctrl+n":
		total := m.searchModalResultLen()
		if m.searchModal.cursor < total-1 {
			m.searchModal.cursor++
			m.clampSearchModalCursor()
		}
		return nil
	case "enter":
		return m.selectGlobalSearchResult()
	default:
		prevVal := m.searchModal.input.Value()
		var cmd tea.Cmd
		m.searchModal.input, cmd = m.searchModal.input.Update(msg)
		if m.searchModal.input.Value() != prevVal {
			searchCmd := m.triggerDebouncedGlobalSearch()
			return tea.Batch(cmd, searchCmd)
		}
		return cmd
	}
}

func (m Model) searchModalResultLen() int {
	if m.searchModal.mode == searchModeUsers {
		return len(m.searchModal.users)
	}
	return len(m.searchModal.repos)
}

func (m *Model) clampSearchModalCursor() {
	total := m.searchModalResultLen()
	if total == 0 {
		m.searchModal.cursor = 0
		m.searchModal.offset = 0
		return
	}
	if m.searchModal.cursor < 0 {
		m.searchModal.cursor = 0
	}
	if m.searchModal.cursor >= total {
		m.searchModal.cursor = total - 1
	}
	vis := m.searchModalVisibleRows()
	if m.searchModal.cursor < m.searchModal.offset {
		m.searchModal.offset = m.searchModal.cursor
	}
	if m.searchModal.cursor >= m.searchModal.offset+vis {
		m.searchModal.offset = m.searchModal.cursor - vis + 1
	}
}

func (m Model) searchModalVisibleRows() int {
	h := m.height
	if h <= 0 {
		h = 24
	}
	return max(4, min(12, h-12))
}

func (m *Model) selectGlobalSearchResult() tea.Cmd {
	if m.searchModal.mode == searchModeUsers {
		if m.searchModal.cursor >= 0 && m.searchModal.cursor < len(m.searchModal.users) {
			u := m.searchModal.users[m.searchModal.cursor]
			m.closeGlobalSearchModal()
			return m.launchUser(u.Login, "All")
		}
		return nil
	}
	if m.searchModal.cursor >= 0 && m.searchModal.cursor < len(m.searchModal.repos) {
		r := m.searchModal.repos[m.searchModal.cursor]
		prev := m.state
		if prev == stateLoading || prev == stateError {
			prev = stateHome
		}
		m.closeGlobalSearchModal()
		return m.openWorkspace(r.Owner, r.Name, "Search", prev)
	}
	return nil
}

func (m Model) renderGlobalSearchModal(width, height int) string {
	modalW := max(44, min(88, width-6))
	innerW := max(20, modalW-4)

	repoPill := lipgloss.NewStyle().Foreground(m.theme.Muted).Background(m.theme.Surface).Padding(0, 1).Render("Repositories")
	userPill := lipgloss.NewStyle().Foreground(m.theme.Muted).Background(m.theme.Surface).Padding(0, 1).Render("Users")
	if m.searchModal.mode == searchModeRepos {
		repoPill = lipgloss.NewStyle().Bold(true).Foreground(m.theme.Background).Background(m.theme.Primary).Padding(0, 1).Render("Repositories")
	} else {
		userPill = lipgloss.NewStyle().Bold(true).Foreground(m.theme.Background).Background(m.theme.Primary).Padding(0, 1).Render("Users")
	}

	modeRow := repoPill + " " + userPill + lipgloss.NewStyle().Foreground(m.theme.Muted).Render("   (Tab switch mode)")

	var lines []string
	lines = append(lines, modeRow)
	lines = append(lines, m.searchModal.input.View())
	lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Border).Render(strings.Repeat("─", innerW)))

	query := strings.TrimSpace(m.searchModal.input.Value())
	if query == "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Muted).Render("Type a query to search GitHub across repositories or users."))
	} else if m.searchModal.loading {
		lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Muted).Render("Searching GitHub..."))
	} else if m.searchModal.err != nil {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Render("⚠ "+m.searchModal.err.Error()))
	} else if m.searchModalResultLen() == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Muted).Render("No results found for '"+query+"'."))
	} else {
		vis := m.searchModalVisibleRows()
		total := m.searchModalResultLen()
		end := min(total, m.searchModal.offset+vis)
		for i := m.searchModal.offset; i < end; i++ {
			selected := i == m.searchModal.cursor
			var row string
			if m.searchModal.mode == searchModeUsers {
				u := m.searchModal.users[i]
				right := u.Type
				left := "@" + u.Login
				if u.Name != "" {
					left += " · " + u.Name
				}
				left = TruncateDisplayWidth(left, max(12, innerW-len(right)-2), "…")
				gap := max(1, innerW-lipgloss.Width(left)-len(right))
				row = TruncateDisplayWidth(left+strings.Repeat(" ", gap)+right, innerW, "…")
			} else {
				r := m.searchModal.repos[i]
				right := fmt.Sprintf("★ %d", r.Stars)
				if r.Language != "" {
					right = r.Language + " · " + right
				}
				left := r.FullName
				if r.Description != "" {
					left += " — " + r.Description
				}
				left = TruncateDisplayWidth(left, max(12, innerW-lipgloss.Width(right)-2), "…")
				gap := max(1, innerW-lipgloss.Width(left)-lipgloss.Width(right))
				row = TruncateDisplayWidth(left+strings.Repeat(" ", gap)+right, innerW, "…")
			}
			if selected {
				lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(m.theme.Background).Background(m.theme.Primary).Width(innerW).Render(row))
			} else {
				lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Text).Width(innerW).Render(row))
			}
		}
	}

	lines = append(lines, "")
	footer := TruncateDisplayWidth("Enter open  •  Tab mode  •  ↑/↓ select  •  Esc close", innerW, "…")
	lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Muted).Render(footer))

	body := strings.Join(lines, "\n")
	modalH := min(height-2, max(12, lipgloss.Height(body)+2))
	pane := WrapInTitledBorder(body, modalW, modalH, "Global GitHub Search", true, m.theme)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, pane,
		lipgloss.WithWhitespaceBackground(m.theme.Background))
}
