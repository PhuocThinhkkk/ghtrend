package ascii

import (
	"strings"
	"testing"
)

func TestFontMapCoverage(t *testing.T) {
	// Verify A-Z
	for r := 'A'; r <= 'Z'; r++ {
		g, ok := fontMap[r]
		if !ok {
			t.Errorf("missing glyph for %c", r)
		}
		if g.Width <= 0 {
			t.Errorf("glyph %c has non-positive width %d", r, g.Width)
		}
		for line := 0; line < 3; line++ {
			if len(g.Rows[line]) != g.Width {
				t.Errorf("glyph %c row %d length %d != width %d", r, line, len(g.Rows[line]), g.Width)
			}
		}
	}

	// Verify 0-9
	for r := '0'; r <= '9'; r++ {
		g, ok := fontMap[r]
		if !ok {
			t.Errorf("missing glyph for %c", r)
		}
		if g.Width <= 0 {
			t.Errorf("glyph %c has non-positive width %d", r, g.Width)
		}
	}

	// Verify symbols
	symbols := []rune{' ', '-', '_', '.', '!', '/', '@'}
	for _, s := range symbols {
		if _, ok := fontMap[s]; !ok {
			t.Errorf("missing glyph for symbol %c", s)
		}
	}
}

func TestRenderPixelBanner(t *testing.T) {
	lines := RenderPixelBanner("THIN NGOO", "#a9b1d6", "#7aa2f7", "#414868")
	for i, l := range lines {
		if len(l) == 0 {
			t.Errorf("line %d of banner is empty", i)
		}
	}

	// Check that ANSI colors are present
	full := lines[0] + lines[1] + lines[2]
	if !strings.Contains(full, "\x1b[38;2;") {
		t.Errorf("expected ANSI 24-bit TrueColor codes in banner")
	}

	// Verify shadow color is used
	// #414868 in RGB is 65, 72, 104
	if !strings.Contains(full, "65;72;104") {
		t.Errorf("expected shadow color RGB(65, 72, 104) in banner, got %q", full)
	}

	// Test single word split
	singleWordLines := RenderPixelBanner("THINGDAGUY", "#a9b1d6", "#7aa2f7", "#414868")
	for i, l := range singleWordLines {
		if len(l) == 0 {
			t.Errorf("line %d of single word banner is empty", i)
		}
	}

	// Test empty text
	emptyLines := RenderPixelBanner("", "", "", "")
	if emptyLines[0] != "" || emptyLines[1] != "" || emptyLines[2] != "" {
		t.Errorf("empty text should return empty lines")
	}
}

func TestRenderPixelBannerString_MaxCols(t *testing.T) {
	out := RenderPixelBannerString("THIN NGOO", "#a9b1d6", "#7aa2f7", "#414868", 25)
	if len(out) == 0 {
		t.Fatal("expected non-empty output for maxCols=25")
	}
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	for _, l := range lines {
		w := visibleWidth(l)
		if w > 25 {
			t.Errorf("visible width %d exceeds maxCols 25", w)
		}
	}
}

func TestVisibleWidth(t *testing.T) {
	styled := "\x1b[38;2;120;140;200mHello\x1b[0m World"
	w := visibleWidth(styled)
	if w != 11 {
		t.Errorf("expected visible width 11, got %d", w)
	}
}
