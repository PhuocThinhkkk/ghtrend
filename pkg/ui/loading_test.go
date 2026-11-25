package ui

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestNewLoadingModel(t *testing.T) {
	loading := newLoadingModel()
	if loading.message == "" {
		t.Errorf("expected loading message to be non-empty")
	}
	if loading.percent <= 0 {
		t.Errorf("expected positive percent, got %f", loading.percent)
	}
}

func TestLoadingViews_RenderWithoutPanic(t *testing.T) {
	loading := newLoadingModel()
	m := Model{
		state:   stateLoading,
		loading: loading,
		width:   80,
		height:  24,
	}

	loadingView := m.renderLoadingView(80, 24)
	if !strings.Contains(loadingView, "G H T R E N D") {
		t.Errorf("expected loading view to contain title, got %q", loadingView)
	}

	m.state = stateError
	m.err = errors.New("network connection timeout")
	errorView := m.renderErrorView(80, 24)
	if !strings.Contains(errorView, "network connection timeout") {
		t.Errorf("expected error view to contain error message, got %q", errorView)
	}
}

func TestModel_LoadingStateTransitions(t *testing.T) {
	loading := newLoadingModel()
	m := Model{
		state:   stateLoading,
		loading: loading,
		width:   80,
		height:  24,
	}

	// Progress tick update
	updated, _ := m.Update(progressTickMsg{percent: 0.50, message: "Parsing..."})
	model := updated.(Model)
	if model.loading.percent != 0.50 || model.loading.message != "Parsing..." {
		t.Errorf("expected updated progress, got %f and %s", model.loading.percent, model.loading.message)
	}

	// Error transition
	errModel, _ := m.Update(reposLoadedMsg{repos: nil, err: errors.New("failed")})
	eModel := errModel.(Model)
	if eModel.state != stateError {
		t.Errorf("expected stateError, got %v", eModel.state)
	}

	// Successful repos transition
	mockRepos := []Repo{
		{
			Owner: "charmbracelet",
			Name:  "bubbletea",
			Stars: "25k",
			Forks: "1.2k",
			RootInfor: []EntryInfor{
				{Type: "file", Name: "main.go"},
			},
		},
	}
	successModel, _ := m.Update(reposLoadedMsg{repos: mockRepos, err: nil})
	sModel := successModel.(Model)
	if sModel.state != stateDashboard {
		t.Errorf("expected stateDashboard, got %v", sModel.state)
	}
	if len(sModel.repoList) != 1 {
		t.Errorf("expected 1 repo loaded, got %d", len(sModel.repoList))
	}

	// Retry from error state
	retriedModel, _ := eModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	rModel := retriedModel.(Model)
	if rModel.state != stateLoading {
		t.Errorf("expected retry to return to stateLoading, got %v", rModel.state)
	}
}

func TestLoadingViews_UserSubtitle(t *testing.T) {
	loading := newLoadingModel()
	m := Model{
		state:   stateLoading,
		loading: loading,
		width:   80,
		height:  24,
		cfg: &flags.CmdConfig{
			User: "PhuocThinhkkk",
		},
	}

	loadingView := m.renderLoadingView(80, 24)
	if !strings.Contains(loadingView, "@PhuocThinhkkk") {
		t.Errorf("expected loading view to contain user '@PhuocThinhkkk', got %q", loadingView)
	}
}

func TestNewLoadingModel_WithThemeAndFallback(t *testing.T) {
	tests := []struct {
		name              string
		th                theme.Theme
		wantSpinnerColor  lipgloss.Color
		wantProgressWidth int
	}{
		{
			name:              "zero value theme uses fallback",
			th:                theme.Theme{},
			wantSpinnerColor:  lipgloss.Color("#BA8E23"),
			wantProgressWidth: 44,
		},
		{
			name:              "tokyonight theme uses accent",
			th:                theme.Catalog[1],
			wantSpinnerColor:  theme.Catalog[1].Accent,
			wantProgressWidth: 44,
		},
		{
			name: "non-hex colors fall back safely for gradient",
			th: theme.Theme{
				Primary: lipgloss.Color("12"),
				Accent:  lipgloss.Color("10"),
			},
			wantSpinnerColor:  lipgloss.Color("10"),
			wantProgressWidth: 44,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lm := newLoadingModelWithTheme(tc.th)
			if lm.message == "" {
				t.Errorf("expected non-empty message")
			}
			if lm.percent <= 0 {
				t.Errorf("expected positive percent, got %f", lm.percent)
			}
			if lm.progress.Width != tc.wantProgressWidth {
				t.Errorf("expected progress width %d, got %d", tc.wantProgressWidth, lm.progress.Width)
			}
			if gotColor := lm.spinner.Style.GetForeground(); gotColor != tc.wantSpinnerColor {
				t.Errorf("expected spinner color %v, got %v", tc.wantSpinnerColor, gotColor)
			}
			if view := lm.progress.View(); view == "" {
				t.Errorf("expected non-empty progress bar view")
			}
		})
	}
}

func TestLoadingViews_ThemedAndZeroValueTheme(t *testing.T) {
	themes := []struct {
		name string
		th   theme.Theme
	}{
		{name: "zero theme", th: theme.Theme{}},
		{name: "catppuccin", th: theme.Catalog[0]},
		{name: "tokyonight", th: theme.Catalog[1]},
		{name: "cyberpunk", th: theme.Catalog[5]},
		{
			name: "partial theme",
			th: theme.Theme{
				Primary: lipgloss.Color("#7AA2F7"),
				Border:  lipgloss.Color("#3D59A1"),
			},
		},
	}

	for _, tc := range themes {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{
				state:   stateLoading,
				loading: newLoadingModelWithTheme(tc.th),
				theme:   tc.th,
				width:   80,
				height:  24,
			}

			lv := m.renderLoadingView(80, 24)
			if !utf8.ValidString(lv) {
				t.Fatalf("loading view produced invalid UTF-8")
			}
			if gotW := lipgloss.Width(lv); gotW != 80 {
				t.Errorf("loading view width: expected 80, got %d", gotW)
			}
			if gotH := lipgloss.Height(lv); gotH != 24 {
				t.Errorf("loading view height: expected 24, got %d", gotH)
			}
			if !strings.Contains(lv, "G H T R E N D") {
				t.Errorf("expected loading title, got %q", lv)
			}
			if !strings.Contains(lv, "[q] Quit") {
				t.Errorf("expected quit hint, got %q", lv)
			}

			m.state = stateError
			m.err = errors.New("rate limit exceeded")
			ev := m.renderErrorView(80, 24)
			if !utf8.ValidString(ev) {
				t.Fatalf("error view produced invalid UTF-8")
			}
			if gotW := lipgloss.Width(ev); gotW != 80 {
				t.Errorf("error view width: expected 80, got %d", gotW)
			}
			if gotH := lipgloss.Height(ev); gotH != 24 {
				t.Errorf("error view height: expected 24, got %d", gotH)
			}
			if !strings.Contains(ev, "rate limit exceeded") {
				t.Errorf("expected error message, got %q", ev)
			}
			if !strings.Contains(ev, "[q] Quit  [r] Retry") {
				t.Errorf("expected error hint, got %q", ev)
			}
		})
	}
}

func TestLoadingViews_BoundedDimensions(t *testing.T) {
	sizes := []struct {
		name   string
		width  int
		height int
	}{
		{name: "compact 60x14", width: 60, height: 14},
		{name: "narrow 40x10", width: 40, height: 10},
		{name: "very narrow 24x7", width: 24, height: 7},
		{name: "standard 80x24", width: 80, height: 24},
	}

	for _, sz := range sizes {
		t.Run(sz.name, func(t *testing.T) {
			lm := newLoadingModelWithTheme(theme.Catalog[1])
			lm.message = "🚀 Đang tải dữ liệu kho lưu trữ GitHub với tên cực kỳ dài vượt khung hiển thị 🔥"

			m := Model{
				state:   stateLoading,
				loading: lm,
				theme:   theme.Catalog[1],
				cfg:     &flags.CmdConfig{User: "very-long-github-username-for-narrow-terminal-check"},
				width:   sz.width,
				height:  sz.height,
			}

			lv := m.renderLoadingView(sz.width, sz.height)
			if !utf8.ValidString(lv) {
				t.Fatalf("loading view produced invalid UTF-8 at %dx%d", sz.width, sz.height)
			}
			if gotW := lipgloss.Width(lv); gotW > sz.width {
				t.Errorf("loading view width %d exceeds bound %d", gotW, sz.width)
			}
			if gotH := lipgloss.Height(lv); gotH > sz.height {
				t.Errorf("loading view height %d exceeds bound %d", gotH, sz.height)
			}
			if sz.width >= 60 && sz.height >= 14 && !strings.Contains(lv, "[q] Quit") {
				t.Errorf("expected '[q] Quit' hint on 60x14, got %q", lv)
			}

			m.state = stateError
			m.err = errors.New(strings.Repeat("GitHub API connection timeout while fetching trending repositories; ", 6))
			ev := m.renderErrorView(sz.width, sz.height)
			if !utf8.ValidString(ev) {
				t.Fatalf("error view produced invalid UTF-8 at %dx%d", sz.width, sz.height)
			}
			if gotW := lipgloss.Width(ev); gotW > sz.width {
				t.Errorf("error view width %d exceeds bound %d", gotW, sz.width)
			}
			if gotH := lipgloss.Height(ev); gotH > sz.height {
				t.Errorf("error view height %d exceeds bound %d", gotH, sz.height)
			}
		})
	}
}

