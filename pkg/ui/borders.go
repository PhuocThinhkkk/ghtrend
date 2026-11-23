package ui

import (
	"strings"

	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/lipgloss"
)

// RenderTitledBorderTop builds the top border line with an embedded title:
// ╭─ Title ──────────────────────╮
func RenderTitledBorderTop(width int, title string, focused bool, th theme.Theme) string {
	if width < 4 {
		return strings.Repeat("─", max(1, width))
	}

	borderColor := th.Border
	if focused {
		borderColor = th.FocusBorder
	}
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	if strings.TrimSpace(title) == "" {
		return borderStyle.Render("╭" + strings.Repeat("─", width-2) + "╮")
	}

	titleStyle := lipgloss.NewStyle().Foreground(th.Text).Bold(true)
	if focused {
		titleStyle = lipgloss.NewStyle().Foreground(th.Primary).Bold(true)
	}

	titleFormatted := " " + title + " "
	titleRendered := titleStyle.Render(titleFormatted)
	titleW := lipgloss.Width(titleRendered)

	// Corner is "╭─" (2 chars)
	leftPart := borderStyle.Render("╭─") + titleRendered
	leftW := 2 + titleW
	remaining := width - leftW - 1 // 1 for "╮"

	if remaining < 0 {
		// Truncate title to fit
		maxTitleW := width - 5 // "╭─ " + " ╮"
		if maxTitleW > 0 && len(title) > maxTitleW {
			title = title[:maxTitleW]
			titleFormatted = " " + title + " "
			titleRendered = titleStyle.Render(titleFormatted)
			leftPart = borderStyle.Render("╭─") + titleRendered
			leftW = 2 + lipgloss.Width(titleRendered)
			remaining = width - leftW - 1
		}
	}
	if remaining < 0 {
		remaining = 0
	}

	return leftPart + borderStyle.Render(strings.Repeat("─", remaining)+"╮")
}

// RenderInnerDivider builds a seamless inner section divider connecting to the side borders:
// ├─ Title ──────────────────────┤
func RenderInnerDivider(width int, title string, focused bool, th theme.Theme) string {
	if width < 4 {
		return strings.Repeat("─", max(1, width))
	}

	borderColor := th.Border
	if focused {
		borderColor = th.FocusBorder
	}
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	if strings.TrimSpace(title) == "" {
		return borderStyle.Render("├" + strings.Repeat("─", width-2) + "┤")
	}

	titleStyle := lipgloss.NewStyle().Foreground(th.Muted).Bold(true)
	if focused {
		titleStyle = lipgloss.NewStyle().Foreground(th.Secondary).Bold(true)
	}

	titleFormatted := " " + title + " "
	titleRendered := titleStyle.Render(titleFormatted)
	titleW := lipgloss.Width(titleRendered)

	leftPart := borderStyle.Render("├─") + titleRendered
	leftW := 2 + titleW
	remaining := width - leftW - 1 // 1 for "┤"

	if remaining < 0 {
		maxTitleW := width - 5
		if maxTitleW > 0 && len(title) > maxTitleW {
			title = title[:maxTitleW]
			titleFormatted = " " + title + " "
			titleRendered = titleStyle.Render(titleFormatted)
			leftPart = borderStyle.Render("├─") + titleRendered
			leftW = 2 + lipgloss.Width(titleRendered)
			remaining = width - leftW - 1
		}
	}
	if remaining < 0 {
		remaining = 0
	}

	return leftPart + borderStyle.Render(strings.Repeat("─", remaining)+"┤")
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
			// truncate safely with lipgloss or rune slice
			line = lipgloss.NewStyle().MaxWidth(innerW).Render(line)
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
