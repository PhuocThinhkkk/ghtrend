package ui

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestChromaStyleNameAndResolve(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		themeID  string
		wantName string
	}{
		{name: "catppuccin", themeID: "catppuccin", wantName: "catppuccin-mocha"},
		{name: "tokyonight", themeID: "tokyonight", wantName: "tokyonight-night"},
		{name: "tokyo-night", themeID: "tokyo-night", wantName: "tokyonight-night"},
		{name: "dracula", themeID: "dracula", wantName: "dracula"},
		{name: "gruvbox", themeID: "gruvbox", wantName: "gruvbox"},
		{name: "nord", themeID: "nord", wantName: "nord"},
		{name: "solarized-dark", themeID: "solarized-dark", wantName: "solarized-dark"},
		{name: "cyberpunk", themeID: "cyberpunk", wantName: "witchhazel"},
		{name: "classic", themeID: "classic", wantName: "monokai"},
		{name: "unknown fallback", themeID: "does-not-exist", wantName: "monokai"},
		{name: "empty fallback", themeID: "", wantName: "monokai"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			th := theme.Theme{ID: tc.themeID}
			got := chromaStyleName(th)
			if got != tc.wantName {
				t.Errorf("chromaStyleName(%q) = %q, want %q", tc.themeID, got, tc.wantName)
			}
			if resolved := resolveChromaStyle(th); resolved == nil {
				t.Errorf("resolveChromaStyle(%q) returned nil", tc.themeID)
			}
		})
	}
}

func TestRenderHighlightedCodeLanguagesAndThemes(t *testing.T) {
	t.Parallel()

	themes := append([]theme.Theme{}, theme.Catalog...)
	themes = append(themes,
		theme.Theme{ID: "tokyo-night", Glamour: "tokyo-night"},
		theme.Theme{ID: "solarized-dark", Glamour: "dark"},
		theme.Theme{}, // empty theme fallback
	)

	samples := []struct {
		name     string
		filename string
		source   string
		wantSub  string
	}{
		{
			name:     "go",
			filename: "main.go",
			source:   "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n",
			wantSub:  "package main",
		},
		{
			name:     "json",
			filename: "config.json",
			source:   "{\n  \"name\": \"ghtrend\",\n  \"stars\": 42\n}\n",
			wantSub:  "\"ghtrend\"",
		},
		{
			name:     "plain-text",
			filename: "notes.txt",
			source:   "hello world\nsecond line",
			wantSub:  "hello world",
		},
	}

	for _, th := range themes {
		for _, sample := range samples {
			name := fmt.Sprintf("%s/%s", th.ID, sample.name)
			if th.ID == "" {
				name = "empty/" + sample.name
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				out := RenderHighlightedCode(sample.filename, sample.source, th, 0)
				if out == "" {
					t.Fatal("expected non-empty highlighted output")
				}
				stripped := ansi.Strip(out)
				if !strings.Contains(stripped, "   1 │ ") {
					t.Errorf("expected line-number gutter '   1 │ ' in output, got:\n%s", stripped)
				}
				if !strings.Contains(stripped, sample.wantSub) {
					t.Errorf("expected %q in stripped output, got:\n%s", sample.wantSub, stripped)
				}
			})
		}
	}
}

func TestRenderHighlightedCodeNormalization(t *testing.T) {
	t.Parallel()

	src := "line1\r\n\tindented\r\nline3\r\n"
	out := RenderHighlightedCode("test.txt", src, theme.Catalog[0], 0)
	stripped := ansi.Strip(out)

	if strings.Contains(stripped, "\r") {
		t.Errorf("expected CRLF to be normalized, found CR in %q", stripped)
	}
	if strings.Contains(stripped, "\t") {
		t.Errorf("expected tabs to be expanded to 4 spaces, found tab in %q", stripped)
	}
	if !strings.Contains(stripped, "   2 │     indented") {
		t.Errorf("expected line 2 with 4-space indent, got:\n%s", stripped)
	}
	lines := strings.Split(stripped, "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d:\n%s", len(lines), stripped)
	}
}

func TestRenderHighlightedCodeLineCapAndNarrowBounds(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	for i := 1; i <= 4050; i++ {
		fmt.Fprintf(&b, "const line_%d = %d // some extra text for width clamping\n", i, i)
	}

	out := RenderHighlightedCode("big.go", b.String(), theme.Catalog[0], 32)
	stripped := ansi.Strip(out)
	lines := strings.Split(stripped, "\n")

	// 4000 code lines + 1 truncation footer
	if len(lines) != maxHighlightLines+1 {
		t.Fatalf("expected %d lines (4000 + footer), got %d", maxHighlightLines+1, len(lines))
	}
	if !strings.Contains(lines[maxHighlightLines-1], "4000 │ ") {
		t.Errorf("expected line 4000 gutter, got %q", lines[maxHighlightLines-1])
	}
	if strings.Contains(stripped, "4001 │ ") {
		t.Errorf("did not expect line 4001 gutter in capped output")
	}
	if !strings.Contains(lines[maxHighlightLines], "50 more lines") {
		t.Errorf("expected truncation footer mentioning 50 more lines, got %q", lines[maxHighlightLines])
	}

	for idx, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(line); w > 32 {
			t.Errorf("line %d exceeded maxWidth 32: width=%d (%q)", idx+1, w, ansi.Strip(line))
			break
		}
	}
}

func TestRenderFullMarkdownPreservesCodeBlocksAndTables(t *testing.T) {
	t.Parallel()

	md := strings.Join([]string{
		"# Title",
		"",
		"```go",
		"func hello() string { return \"world\" }",
		"```",
		"",
		"| ColumnA | ColumnB |",
		"| ------- | ------- |",
		"| alpha   | beta    |",
		"",
		"[link](https://example.com)",
	}, "\n")

	cases := []struct {
		name      string
		th        theme.Theme
		wrapWidth int
	}{
		{name: "catppuccin normal", th: theme.GetTheme("catppuccin"), wrapWidth: 80},
		{name: "dracula narrow", th: theme.GetTheme("dracula"), wrapWidth: 10},
		{name: "empty theme fallback", th: theme.Theme{}, wrapWidth: 60},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := RenderFullMarkdown(md, tc.th, tc.wrapWidth)
			if out == "" {
				t.Fatal("expected non-empty rendered markdown")
			}
			stripped := ansi.Strip(out)
			for _, want := range []string{"Title", "func hello()", "Colu", "alpha", "beta", "│"} {
				if !strings.Contains(stripped, want) {
					t.Errorf("expected %q in rendered markdown, got:\n%s", want, stripped)
				}
			}
		})
	}

	if got := RenderFullMarkdown("   \n\t  ", theme.Catalog[0], 80); got != "" {
		t.Errorf("expected empty string for whitespace-only markdown, got %q", got)
	}
}

func TestHighlightConcurrentRaceSafety(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			th := theme.Catalog[idx%len(theme.Catalog)]
			code := RenderHighlightedCode("main.go", "package main\nfunc main() {}\n", th, 60)
			md := RenderFullMarkdown("# Heading\n\n```json\n{\"ok\": true}\n```\n", th, 60)
			if code == "" || md == "" {
				t.Errorf("unexpected empty output in goroutine %d", idx)
			}
		}(i)
	}
	wg.Wait()
}
