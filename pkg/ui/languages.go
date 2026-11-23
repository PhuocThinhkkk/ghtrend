package ui

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/lipgloss"
)

type LangPair struct {
	lang  string
	lines string
	pct   float64
}

var languageColors = map[string]string{
	"Go":         "#00ADD8",
	"Python":     "#3572A5",
	"Java":       "#b07219",
	"TypeScript": "#3178c6",
	"JavaScript": "#f1e05a",
	"Rust":       "#dea584",
	"C++":        "#f34b7d",
	"C":          "#555555",
	"C#":         "#178600",
	"HTML":       "#e34c26",
	"CSS":        "#563d7c",
	"Shell":      "#89e051",
	"Bash":       "#89e051",
	"SQL":        "#e38c00",
	"Ruby":       "#701516",
	"PHP":        "#4F5D95",
	"Kotlin":     "#A97BFF",
	"Swift":      "#F05138",
	"Dart":       "#00B4AB",
	"Jupyter":    "#DA5B0B",
	"Vue":        "#41b883",
	"Dockerfile": "#384d54",
	"Makefile":   "#427819",
}

func getLanguageColor(lang string, idx int, th theme.Theme) lipgloss.Color {
	if c, ok := languageColors[lang]; ok {
		return lipgloss.Color(c)
	}
	palette := []lipgloss.Color{th.Primary, th.Secondary, th.Accent, th.FocusBorder}
	return palette[idx%len(palette)]
}

func getSortedLangPairs(repo Repo) []LangPair {
	var pairs []LangPair
	for k, v := range repo.LanguagesBreakDown {
		clean := strings.TrimSpace(strings.TrimSuffix(v, "%"))
		val, _ := strconv.ParseFloat(clean, 64)
		pairs = append(pairs, LangPair{lang: k, lines: v, pct: val})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].pct > pairs[j].pct
	})
	return pairs
}

// renderStackedLanguageBar creates a full-width colored stacked bar (████████░░░░)
func renderStackedLanguageBar(pairs []LangPair, width int, th theme.Theme) string {
	if width <= 0 {
		width = 20
	}
	if len(pairs) == 0 {
		return lipgloss.NewStyle().Foreground(th.Muted).Render(strings.Repeat("░", width))
	}

	var totalPct float64
	for _, p := range pairs {
		totalPct += p.pct
	}
	if totalPct <= 0 {
		totalPct = 100.0
	}

	allocated := 0
	type barSegment struct {
		color lipgloss.Color
		count int
	}
	var segments []barSegment

	for i, p := range pairs {
		segLen := int(math.Round(float64(width) * (p.pct / totalPct)))
		if segLen == 0 && p.pct > 0 && allocated < width {
			segLen = 1
		}
		if allocated+segLen > width {
			segLen = width - allocated
		}
		if segLen > 0 {
			segments = append(segments, barSegment{
				color: getLanguageColor(p.lang, i, th),
				count: segLen,
			})
			allocated += segLen
		}
	}

	var sb strings.Builder
	for _, seg := range segments {
		style := lipgloss.NewStyle().Foreground(seg.color)
		sb.WriteString(style.Render(strings.Repeat("█", seg.count)))
	}

	if allocated < width {
		mutedStyle := lipgloss.NewStyle().Foreground(th.Border)
		sb.WriteString(mutedStyle.Render(strings.Repeat("░", width-allocated)))
	}

	return sb.String()
}

// renderLanguageDots creates wrapped lines of color-coded dots and labels: ● Java 82.4%  ● SQL 17.6%
func renderLanguageDots(pairs []LangPair, width, maxLines int, th theme.Theme) string {
	if len(pairs) == 0 {
		return lipgloss.NewStyle().Foreground(th.Muted).Render("· No language data")
	}
	if maxLines <= 0 {
		maxLines = 1
	}

	var lines []string
	var currentLineParts []string
	currentW := 0

	for i, p := range pairs {
		col := getLanguageColor(p.lang, i, th)
		dot := lipgloss.NewStyle().Foreground(col).Render("●")
		txt := lipgloss.NewStyle().Foreground(th.Text).Render(fmt.Sprintf(" %s %.1f%%", p.lang, p.pct))
		part := dot + txt
		partW := lipgloss.Width(part)

		if currentW+partW > width && len(currentLineParts) > 0 {
			lines = append(lines, strings.Join(currentLineParts, "  "))
			if len(lines) >= maxLines {
				break
			}
			currentLineParts = nil
			currentW = 0
		}

		currentLineParts = append(currentLineParts, part)
		currentW += partW + 2
	}

	if len(currentLineParts) > 0 && len(lines) < maxLines {
		lines = append(lines, strings.Join(currentLineParts, "  "))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderLanguagesBreakDown(totalWidth, height int, th theme.Theme) string {
	if totalWidth <= 0 {
		totalWidth = 44
	}
	lineWidth := max(10, totalWidth-4)
	repo := m.getCursorRepo()
	pairs := getSortedLangPairs(repo)

	bar := renderStackedLanguageBar(pairs, lineWidth, th)
	maxDotLines := max(1, height-4)
	dots := renderLanguageDots(pairs, lineWidth, maxDotLines, th)

	content := bar + "\n" + dots
	return WrapInTitledBorder(content, totalWidth, height, "Languages Breakdown", false, th)
}
