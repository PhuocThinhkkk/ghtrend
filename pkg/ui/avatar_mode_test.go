package ui

import (
	"strings"
	"testing"

	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	tea "github.com/charmbracelet/bubbletea"
)

func TestModel_AvatarModeToggle(t *testing.T) {
	th := theme.GetTheme("tokyonight")
	profile := &ghclient.UserProfile{
		Username: "testuser",
		Name:     "Test User",
	}
	profile.RefreshAvatars(28, 14)

	m := Model{
		state:       stateDashboard,
		theme:       th,
		userProfile: profile,
		cfg:         &flags.CmdConfig{User: "testuser"},
		width:       100,
		height:      30,
		avatarMode:  AvatarModeHalfBlock,
	}

	// Initial render should use half block
	view1 := m.renderUserProfileCard(60, 20, th)
	if !strings.Contains(view1, "▀") && !strings.Contains(view1, "▄") {
		t.Errorf("expected half block characters in initial view")
	}

	// Press 'a' to toggle to Braille mode
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updated.(Model)
	if m.avatarMode != AvatarModeBraille {
		t.Fatalf("expected avatarMode to be AvatarModeBraille, got %v", m.avatarMode)
	}
	if !strings.Contains(m.toastMessage, "Braille") {
		t.Errorf("expected toast message mentioning Braille, got %q", m.toastMessage)
	}
	if cmd == nil {
		t.Errorf("expected toast clear command")
	}

	// Render in Braille mode
	view2 := m.renderUserProfileCard(60, 20, th)
	// Braille mode should not have background color codes
	if strings.Contains(view2, "48;2;") {
		t.Errorf("braille mode should not have background color codes")
	}

	// Press 'a' again to toggle back to HalfBlock mode
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updated.(Model)
	if m.avatarMode != AvatarModeHalfBlock {
		t.Fatalf("expected avatarMode to be AvatarModeHalfBlock, got %v", m.avatarMode)
	}
	if !strings.Contains(m.toastMessage, "half-block") {
		t.Errorf("expected toast message mentioning half-block, got %q", m.toastMessage)
	}
}

func TestUserProfile_RefreshAvatars(t *testing.T) {
	profile := &ghclient.UserProfile{
		Username: "octocat",
	}
	profile.RefreshAvatars(28, 14)

	if profile.AvatarHalfBlock == "" {
		t.Errorf("expected non-empty AvatarHalfBlock after RefreshAvatars")
	}
	if profile.AvatarBraille == "" {
		t.Errorf("expected non-empty AvatarBraille after RefreshAvatars")
	}
	if profile.AvatarASCII == "" {
		t.Errorf("expected non-empty AvatarASCII after RefreshAvatars")
	}
}
