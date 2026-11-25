package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

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

func TestTruncateDisplayWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
		tail     string
		wantExact string
	}{
		{
			name:      "zero maxWidth",
			input:     "hello",
			maxWidth:  0,
			tail:      "…",
			wantExact: "",
		},
		{
			name:      "negative maxWidth",
			input:     "hello",
			maxWidth:  -5,
			tail:      "…",
			wantExact: "",
		},
		{
			name:      "fits without truncation",
			input:     "🚀 Dự án",
			maxWidth:  20,
			tail:      "…",
			wantExact: "🚀 Dự án",
		},
		{
			name:     "vietnamese emoji truncation",
			input:    "🚀 Dự án tiếng Việt 🔥",
			maxWidth: 12,
			tail:     "…",
		},
		{
			name:     "tail wider than maxWidth",
			input:    "abcdef",
			maxWidth: 2,
			tail:     "...",
		},
		{
			name:     "cjk odd width",
			input:    "日本語テスト",
			maxWidth: 5,
			tail:     "…",
		},
		{
			name:     "combining diacritics",
			input:    "e\u0301clair cafe\u0301",
			maxWidth: 6,
			tail:     "…",
		},
		{
			name:     "ansi styled string",
			input:    lipgloss.NewStyle().Foreground(lipgloss.Color("#7AA2F7")).Render("🚀 Dự án tiếng Việt 🔥"),
			maxWidth: 10,
			tail:     "…",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TruncateDisplayWidth(tc.input, tc.maxWidth, tc.tail)
			if !utf8.ValidString(got) {
				t.Fatalf("expected valid UTF-8, got invalid bytes: %q", got)
			}
			if tc.maxWidth > 0 && lipgloss.Width(got) > tc.maxWidth {
				t.Errorf("expected width <= %d, got %d (%q)", tc.maxWidth, lipgloss.Width(got), got)
			}
			if tc.wantExact != "" || tc.maxWidth <= 0 {
				if got != tc.wantExact {
					t.Errorf("expected %q, got %q", tc.wantExact, got)
				}
			}
		})
	}
}

func TestRenderTitledBorderTop_UTF8Truncation(t *testing.T) {
	th := theme.Catalog[1]
	title := "🚀 Dự án tiếng Việt 🔥"
	widths := []int{4, 5, 6, 8, 10, 14, 20, 30, 40}

	for _, focused := range []bool{false, true} {
		for _, w := range widths {
			top := RenderTitledBorderTop(w, title, focused, th)
			if !utf8.ValidString(top) {
				t.Fatalf("width=%d focused=%v produced invalid UTF-8: %q", w, focused, top)
			}
			if gotW := lipgloss.Width(top); gotW != w {
				t.Errorf("width=%d focused=%v: expected width %d, got %d (%q)", w, focused, w, gotW, top)
			}
			if !strings.Contains(top, "╭") || !strings.Contains(top, "╮") {
				t.Errorf("width=%d focused=%v: expected rounded corners, got %q", w, focused, top)
			}
			if w >= 30 && !strings.Contains(top, title) {
				t.Errorf("width=%d focused=%v: expected full title preserved, got %q", w, focused, top)
			}
			if w >= 6 && w < 26 && !strings.Contains(top, "…") {
				t.Errorf("width=%d focused=%v: expected ellipsis for truncated title, got %q", w, focused, top)
			}
		}
	}
}

func TestRenderInnerDivider_UTF8Truncation(t *testing.T) {
	th := theme.Catalog[1]
	title := "🚀 Dự án tiếng Việt 🔥"
	widths := []int{4, 5, 6, 8, 10, 14, 20, 30, 40}

	for _, focused := range []bool{false, true} {
		for _, w := range widths {
			div := RenderInnerDivider(w, title, focused, th)
			if !utf8.ValidString(div) {
				t.Fatalf("width=%d focused=%v produced invalid UTF-8: %q", w, focused, div)
			}
			if gotW := lipgloss.Width(div); gotW != w {
				t.Errorf("width=%d focused=%v: expected width %d, got %d (%q)", w, focused, w, gotW, div)
			}
			if !strings.Contains(div, "├") || !strings.Contains(div, "┤") {
				t.Errorf("width=%d focused=%v: expected divider edges, got %q", w, focused, div)
			}
			if w >= 30 && !strings.Contains(div, title) {
				t.Errorf("width=%d focused=%v: expected full title preserved, got %q", w, focused, div)
			}
		}
	}
}

func TestWrapInTitledBorder_UTF8AndNarrowBounds(t *testing.T) {
	th := theme.Catalog[1]
	title := "🚀 Dự án tiếng Việt 🔥"
	content := "🚀 Dòng nội dung tiếng Việt rất dài vượt quá khung hiển thị 🔥\nDòng thứ hai"

	cases := []struct {
		width  int
		height int
	}{
		{width: 12, height: 4},
		{width: 20, height: 5},
		{width: 60, height: 6},
	}

	for _, tc := range cases {
		box := WrapInTitledBorder(content, tc.width, tc.height, title, true, th)
		if !utf8.ValidString(box) {
			t.Fatalf("%dx%d produced invalid UTF-8", tc.width, tc.height)
		}
		if gotW := lipgloss.Width(box); gotW != tc.width {
			t.Errorf("%dx%d: expected width %d, got %d", tc.width, tc.height, tc.width, gotW)
		}
		if gotH := lipgloss.Height(box); gotH != tc.height {
			t.Errorf("%dx%d: expected height %d, got %d", tc.width, tc.height, tc.height, gotH)
		}
	}
}
