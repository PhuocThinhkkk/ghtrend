package ui

import (
	"strings"

	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// TruncateDisplayWidth truncates s to fit within maxWidth terminal cells,
// appending tail when truncation occurs. It is rune-, grapheme-, and ANSI-aware.
func TruncateDisplayWidth(s string, maxWidth int, tail string) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if lipgloss.Width(tail) > maxWidth {
		tail = ""
	}
	truncated := ansi.Truncate(s, maxWidth, tail)
	if lipgloss.Width(truncated) <= maxWidth {
		return truncated
	}
	// Fallback rune guard in case of width discrepancy.
	var b strings.Builder
	for _, r := range s {
		if lipgloss.Width(b.String()+string(r)+tail) > maxWidth {
			break
		}
		b.WriteRune(r)
	}
	return b.String() + tail
}

func renderHorizontalBorderWithTitle(
	width int,
	title string,
	leftCorner string,
	rightCorner string,
	borderColor lipgloss.Color,
	titleColor lipgloss.Color,
) string {
	if width < 4 {
		return strings.Repeat("─", max(1, width))
	}

	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	if strings.TrimSpace(title) == "" {
		return borderStyle.Render(leftCorner + strings.Repeat("─", width-2) + rightCorner)
	}

	maxTitleW := width - 5 // "<leftCorner>─ " + " <rightCorner>"
	if maxTitleW <= 0 {
		return borderStyle.Render(leftCorner + strings.Repeat("─", width-2) + rightCorner)
	}

	if lipgloss.Width(title) > maxTitleW {
		title = TruncateDisplayWidth(title, maxTitleW, "…")
		if strings.TrimSpace(title) == "" {
			return borderStyle.Render(leftCorner + strings.Repeat("─", width-2) + rightCorner)
		}
	}

	titleStyle := lipgloss.NewStyle().Foreground(titleColor).Bold(true)
	titleFormatted := " " + title + " "
	titleRendered := titleStyle.Render(titleFormatted)
	titleW := lipgloss.Width(titleRendered)

	leftPart := borderStyle.Render(leftCorner+"─") + titleRendered
	leftW := 2 + titleW
	remaining := width - leftW - 1 // 1 for rightCorner
	if remaining < 0 {
		remaining = 0
	}

	return leftPart + borderStyle.Render(strings.Repeat("─", remaining)+rightCorner)
}

// RenderTitledBorderTop builds the top border line with an embedded title:
// ╭─ Title ──────────────────────╮
func RenderTitledBorderTop(width int, title string, focused bool, th theme.Theme) string {
	borderColor := th.Border
	titleColor := th.Text
	if focused {
		borderColor = th.FocusBorder
		titleColor = th.Primary
	}
	return renderHorizontalBorderWithTitle(width, title, "╭", "╮", borderColor, titleColor)
}

// RenderInnerDivider builds a seamless inner section divider connecting to the side borders:
// ├─ Title ──────────────────────┤
func RenderInnerDivider(width int, title string, focused bool, th theme.Theme) string {
	borderColor := th.Border
	titleColor := th.Muted
	if focused {
		borderColor = th.FocusBorder
		titleColor = th.Secondary
	}
	return renderHorizontalBorderWithTitle(width, title, "├", "┤", borderColor, titleColor)
}

// RenderTitledBorderBottom builds the bottom border line:
// ╰──────────────────────────────╯
func RenderTitledBorderBottom(width int, focused bool, th theme.Theme) string {
	if width < 4 {
		return strings.Repeat("─", max(1, width))
	}

	borderColor := th.Border
	if focused {
		borderColor = th.FocusBorder
	}
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	return borderStyle.Render("╰" + strings.Repeat("─", width-2) + "╯")
}

// WrapInTitledBorder wraps multiline content in a box with a top titled border and rounded corners.
func WrapInTitledBorder(content string, width, height int, title string, focused bool, th theme.Theme) string {
	if width <= 0 {
		width = 40
	}

	top := RenderTitledBorderTop(width, title, focused, th)
	bottom := RenderTitledBorderBottom(width, focused, th)

	borderColor := th.Border
	if focused {
		borderColor = th.FocusBorder
	}
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	leftBorder := borderStyle.Render("│")
	rightBorder := borderStyle.Render("│")

	innerW := width - 2
	if innerW < 1 {
		innerW = 1
	}

	contentLines := strings.Split(content, "\n")
	targetBodyH := height - 2
	if targetBodyH < 0 {
		targetBodyH = len(contentLines)
	}

	var bodyLines []string
	for i := 0; i < targetBodyH; i++ {
		line := ""
		if i < len(contentLines) {
			line = contentLines[i]
		}
		lineW := lipgloss.Width(line)
		if lineW > innerW {
			line = TruncateDisplayWidth(line, innerW, "")
			lineW = lipgloss.Width(line)
		}
		pad := innerW - lineW
		if pad < 0 {
			pad = 0
		}
		bodyLines = append(bodyLines, leftBorder+line+strings.Repeat(" ", pad)+rightBorder)
	}

	res := top
	if len(bodyLines) > 0 {
		res += "\n" + strings.Join(bodyLines, "\n")
	}
	res += "\n" + bottom
	return res
}
