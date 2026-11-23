package ui

import (
	"strings"
	"testing"

	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
)

func TestRenderMetricsLanguagesAndCommits(t *testing.T) {
	th := theme.Catalog[1] // tokyonight
	repo := Repo{
		Owner: "PhuocThinhkkk",
		Name:  "ghtrend",
		ExtraInfor: ExtraInfor{
			TotalCommits: 42,
			PullRequests: "", // Blank in API -> should display 0
			Issues:       "3",
			Contributors: 2,
		},
		LanguagesBreakDown: map[string]string{
			"Go":    "85.0%",
			"Shell": "15.0%",
		},
		RecentCommits: []ghclient.Commit{
			{Hash: "7a1b2c3", Message: "feat: add pixel-block typography"},
			{Hash: "4d5e6f7", Message: "fix: correct layout bounds"},
		},
	}

	m := Model{
		repoList: []Repo{repo},
		theme:    th,
	}

	out := m.renderMetricsLanguagesAndCommits(44, 18, th)
	if len(out) == 0 {
		t.Fatal("expected non-empty rendered metrics column")
	}

	// Verify PR count 0 fallback
	if !strings.Contains(out, "Pull Requests:") {
		t.Error("expected 'Pull Requests:' label")
	}
	if !strings.Contains(out, "0") {
		t.Error("expected '0' for blank pull requests")
	}

	// Verify inner dividers
	if !strings.Contains(out, "├─ Languages Breakdown") {
		t.Error("expected Languages Breakdown inner divider")
	}
	if !strings.Contains(out, "├─ Recent Commits") {
		t.Error("expected Recent Commits inner divider")
	}

	// Verify commit hashes and messages
	if !strings.Contains(out, "7a1b2c3") || !strings.Contains(out, "4d5e6f7") {
		t.Errorf("expected recent commit hashes in output, got: %q", out)
	}

	// Verify stacked bar
	if !strings.Contains(out, "█") {
		t.Error("expected stacked bar full block '█'")
	}
}

func TestRenderContributionHeatmap(t *testing.T) {
	th := theme.Catalog[1]
	days := []int{0, 1, 2, 3, 4, 0, 1, 2, 3, 4, 0, 1, 2, 3}
	out := renderContributionHeatmap(days, "847", 18, 10, th)

	if len(out) == 0 {
		t.Fatal("expected non-empty heatmap")
	}
	if !strings.Contains(out, "Contributions: 847/yr") {
		t.Errorf("expected contributions title, got: %q", out)
	}
	if !strings.Contains(out, "▪") && !strings.Contains(out, "▫") {
		t.Errorf("expected heatmap cells '▪' or '▫', got: %q", out)
	}
	if !strings.Contains(out, "Less") || !strings.Contains(out, "More") {
		t.Errorf("expected heatmap legend, got: %q", out)
	}
}

func TestRenderUserProfileCard_WithBannerAndHeatmap(t *testing.T) {
	th := theme.Catalog[1]
	profile := &ghclient.UserProfile{
		Username:           "thingdaguy",
		Name:               "Thin Ngoo",
		Bio:                "Terminal UI enthusiast",
		Followers:          "42",
		Following:          "10",
		PublicRepos:        15,
		TotalContributions: "350",
	}

	m := Model{
		userProfile: profile,
		theme:       th,
	}

	card := m.renderUserProfileCard(70, 16, th)
	if len(card) == 0 {
		t.Fatal("expected non-empty profile card")
	}

	// Should contain titled border
	if !strings.Contains(card, "╭─ Profile") {
		t.Errorf("expected titled border '╭─ Profile', got: %q", card)
	}

	// Should contain username and bio
	if !strings.Contains(card, "@thingdaguy") {
		t.Errorf("expected username in profile card, got: %q", card)
	}
	if !strings.Contains(card, "Terminal UI enthusiast") {
		t.Errorf("expected bio in profile card, got: %q", card)
	}
}
