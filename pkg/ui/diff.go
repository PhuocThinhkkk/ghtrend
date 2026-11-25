package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/lipgloss"
)

// RenderUnifiedDiff formats a unified git patch with aligned old/new line numbers
// and color-coded hunk, addition, deletion, and context lines.
func RenderUnifiedDiff(patch string, th theme.Theme, width int) string {
	if strings.TrimSpace(patch) == "" {
		return lipgloss.NewStyle().Foreground(th.Muted).Render("No textual diff available (binary file, rename-only, or large diff omitted).")
	}
	if width < 24 {
		width = 24
	}

	lineNumStyle := lipgloss.NewStyle().Foreground(th.Muted)
	hunkStyle := lipgloss.NewStyle().Foreground(th.Secondary).Bold(true)
	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a"))
	delStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e"))
	ctxStyle := lipgloss.NewStyle().Foreground(th.Text)
	metaStyle := lipgloss.NewStyle().Foreground(th.Muted).Italic(true)

	lines := strings.Split(strings.ReplaceAll(patch, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))

	oldLine, newLine := 0, 0
	inHunk := false

	for i, raw := range lines {
		if i == len(lines)-1 && raw == "" {
			continue
		}
		if strings.HasPrefix(raw, "@@") {
			if o, n, ok := parseUnifiedHunkHeader(raw); ok {
				oldLine, newLine = o, n
				inHunk = true
			}
			out = append(out, hunkStyle.Render(TruncateDisplayWidth(raw, width, "…")))
			continue
		}
		if !inHunk {
			out = append(out, metaStyle.Render(TruncateDisplayWidth(raw, width, "…")))
			continue
		}

		if strings.HasPrefix(raw, `\`) {
			out = append(out, metaStyle.Render("         │ "+raw))
			continue
		}

		contentWidth := max(10, width-12)
		if strings.HasPrefix(raw, "+") {
			prefix := lineNumStyle.Render(fmt.Sprintf("    %4d │ ", newLine))
			body := addStyle.Render(TruncateDisplayWidth(raw, contentWidth, "…"))
			out = append(out, prefix+body)
			newLine++
		} else if strings.HasPrefix(raw, "-") {
			prefix := lineNumStyle.Render(fmt.Sprintf("%4d     │ ", oldLine))
			body := delStyle.Render(TruncateDisplayWidth(raw, contentWidth, "…"))
			out = append(out, prefix+body)
			oldLine++
		} else {
			prefix := lineNumStyle.Render(fmt.Sprintf("%4d %4d│ ", oldLine, newLine))
			body := ctxStyle.Render(TruncateDisplayWidth(raw, contentWidth, "…"))
			out = append(out, prefix+body)
			oldLine++
			newLine++
		}
	}

	if len(out) == 0 {
		return lipgloss.NewStyle().Foreground(th.Muted).Render("Empty diff.")
	}
	return strings.Join(out, "\n")
}

func parseUnifiedHunkHeader(line string) (oldStart, newStart int, ok bool) {
	// Format: @@ -oldStart[,oldCount] +newStart[,newCount] @@...
	if !strings.HasPrefix(line, "@@") {
		return 0, 0, false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, "@@"))
	endIdx := strings.Index(rest, "@@")
	if endIdx >= 0 {
		rest = strings.TrimSpace(rest[:endIdx])
	}
	fields := strings.Fields(rest)
	if len(fields) < 2 {
		return 0, 0, false
	}
	oStr := strings.TrimPrefix(fields[0], "-")
	if comma := strings.IndexByte(oStr, ','); comma >= 0 {
		oStr = oStr[:comma]
	}
	nStr := strings.TrimPrefix(fields[1], "+")
	if comma := strings.IndexByte(nStr, ','); comma >= 0 {
		nStr = nStr[:comma]
	}
	o, err1 := strconv.Atoi(oStr)
	n, err2 := strconv.Atoi(nStr)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return o, n, true
}
