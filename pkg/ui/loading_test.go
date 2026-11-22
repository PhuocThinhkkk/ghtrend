package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
