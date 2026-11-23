package ui

import (
	"strings"
	"testing"

	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/lipgloss"
)

func TestRenderTitledBorderTop(t *testing.T) {
	th := theme.Catalog[1] // TokyoNight
	widths := []int{20, 40, 60, 80}

	for _, w := range widths {
		top := RenderTitledBorderTop(w, "Project Metrics", false, th)
		actualW := lipgloss.Width(top)
		if actualW != w {
			t.Errorf("expected top border width %d, got %d", w, actualW)
		}
		if !strings.Contains(top, "Project Metrics") {
			t.Errorf("expected top border to contain title, got %q", top)
		}
		if !strings.Contains(top, "╭─") || !strings.Contains(top, "╮") {
			t.Errorf("expected top border corners '╭─' and '╮', got %q", top)
		}
	}
}

func TestRenderInnerDivider(t *testing.T) {
	th := theme.Catalog[1]
	widths := []int{20, 40, 60, 80}

	for _, w := range widths {
		div := RenderInnerDivider(w, "Recent Commits", false, th)
		actualW := lipgloss.Width(div)
		if actualW != w {
			t.Errorf("expected divider width %d, got %d", w, actualW)
		}
		if !strings.Contains(div, "Recent Commits") {
			t.Errorf("expected divider to contain title, got %q", div)
		}
		if !strings.Contains(div, "├─") || !strings.Contains(div, "┤") {
			t.Errorf("expected divider edges '├─' and '┤', got %q", div)
		}
	}
}

func TestWrapInTitledBorder_Dimensions(t *testing.T) {
	th := theme.Catalog[1]
	content := "Line 1\nLine 2\nLine 3"
	width := 50
	height := 8

	box := WrapInTitledBorder(content, width, height, "Test Box", true, th)
	actualH := lipgloss.Height(box)
	actualW := lipgloss.Width(box)

	if actualH != height {
		t.Errorf("expected box height %d, got %d", height, actualH)
	}
	if actualW != width {
		t.Errorf("expected box width %d, got %d", width, actualW)
	}
}
