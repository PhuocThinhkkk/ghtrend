package ui

import (
	"fmt"
	"strings"

	"github.com/PhuocThinhkkk/ghtrend/pkg/ascii"
	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderUserProfileCard(width, height int, th theme.Theme) string {
	if m.userProfile == nil {
		return ""
	}

	usableW := max(20, width-4)
	innerH := max(6, height-2)

	// Determine avatar dimensions (half-block 1:2 aspect ratio)
	avatarH := min(15, max(8, innerH))
	avatarW := min(30, max(20, avatarH*2))
	if usableW < 54 {
		avatarW = min(24, max(16, usableW/2))
		avatarH = avatarW / 2
	}

	var avatar string
	if m.avatarMode == AvatarModeBraille {
		if m.userProfile.AvatarBraille != "" {
			storedW := lipgloss.Width(strings.Split(m.userProfile.AvatarBraille, "\n")[0])
			if storedW <= usableW-16 {
				avatar = m.userProfile.AvatarBraille
			}
		}
		if avatar == "" {
			if m.userProfile.AvatarURL != "" {
				m.userProfile.AvatarBraille = ascii.AvatarToBraille(m.userProfile.AvatarURL, avatarW, avatarH)
				avatar = m.userProfile.AvatarBraille
			} else {
				avatar = ascii.FallbackSilhouetteBraille(avatarW, avatarH)
			}
		}
	} else {
		if m.userProfile.AvatarHalfBlock != "" {
			storedW := lipgloss.Width(strings.Split(m.userProfile.AvatarHalfBlock, "\n")[0])
			if storedW <= usableW-16 {
				avatar = m.userProfile.AvatarHalfBlock
			}
		}
		if avatar == "" {
			if m.userProfile.AvatarURL != "" {
				m.userProfile.AvatarHalfBlock = ascii.AvatarToHalfBlock(m.userProfile.AvatarURL, avatarW, avatarH)
				avatar = m.userProfile.AvatarHalfBlock
			} else {
				avatar = ascii.FallbackSilhouetteHalfBlock(avatarW, avatarH)
			}
		}
	}

	actualAvatarW := lipgloss.Width(strings.Split(avatar, "\n")[0])
	rightW := usableW - actualAvatarW - 2
	if rightW < 14 {
		rightW = usableW
	}

	// 1. Pixel-block user name banner
	name := m.userProfile.Name
	if name == "" {
		name = m.userProfile.Username
	}

	// TokyoNight complementary colors: #a9b1d6 (silver/taupe) and #7aa2f7 (bright blue)
	col1 := "#a9b1d6"
	col2 := "#7aa2f7"
	shadowCol := "#414868"
	if th.ID == "catppuccin" {
		col1 = "#cdd6f4"
		col2 = "#cba6f7"
		shadowCol = "#45475a"
	} else if th.ID == "dracula" {
		col1 = "#f8f8f2"
		col2 = "#bd93f9"
		shadowCol = "#44475a"
	} else if th.ID == "nord" {
		col1 = "#eceff4"
		col2 = "#88c0d0"
		shadowCol = "#3b4252"
	}

	maxBannerW := rightW - 2
	var bannerView string
	if maxBannerW >= 16 {
		candidate := strings.TrimSpace(name)
		// 1. Try full name
		if lines := ascii.RenderPixelBanner(candidate, col1, col2, shadowCol); lines[0] != "" && lipgloss.Width(lines[0]) <= maxBannerW {
			bannerView = lines[0] + "\n" + lines[1] + "\n" + lines[2]
		}

		// 2. If full name is too wide, try shorter version (e.g. last two words)
		if bannerView == "" {
			words := strings.Fields(candidate)
			if len(words) > 2 {
				shortCandidate := strings.Join(words[len(words)-2:], " ")
				if lines := ascii.RenderPixelBanner(shortCandidate, col1, col2, shadowCol); lines[0] != "" && lipgloss.Width(lines[0]) <= maxBannerW {
					bannerView = lines[0] + "\n" + lines[1] + "\n" + lines[2]
				}
			}
		}

		// 3. If still too wide, try @username
		if bannerView == "" && m.userProfile.Username != "" && m.userProfile.Username != candidate {
			userCandidate := m.userProfile.Username
			if lines := ascii.RenderPixelBanner(userCandidate, col1, col2, shadowCol); lines[0] != "" && lipgloss.Width(lines[0]) <= maxBannerW {
				bannerView = lines[0] + "\n" + lines[1] + "\n" + lines[2]
			}
		}

		// 4. If none of the complete candidates fit, try RenderPixelBannerString with maxBannerW
		if bannerView == "" {
			bannerView = ascii.RenderPixelBannerString(candidate, col1, col2, shadowCol, maxBannerW)
		}
	}
	if bannerView == "" {
		bannerView = lipgloss.NewStyle().
			Bold(true).
			Foreground(th.Primary).
			MaxWidth(rightW).
			Render(name)
	}

	// 2. User details underneath banner
	fullNameStyled := lipgloss.NewStyle().
		Bold(true).
		Foreground(th.Text).
		MaxWidth(rightW).
		Render(name)

	handleStyled := lipgloss.NewStyle().
		Foreground(th.Muted).
		MaxWidth(rightW).
		Render(fmt.Sprintf("@%s", m.userProfile.Username))

	var nameHandleLine string
	if lipgloss.Width(fullNameStyled)+lipgloss.Width(handleStyled)+2 <= rightW {
		nameHandleLine = fullNameStyled + "  " + handleStyled
	} else {
		nameHandleLine = fullNameStyled
	}

	bio := strings.TrimSpace(m.userProfile.Bio)
	var bioStyled string
	if bio != "" {
		bioStyled = lipgloss.NewStyle().
			Foreground(th.Text).
			MaxWidth(rightW).
			Render(bio)
	}

	// Stats line: followers, following, public repos
	var stats []string
	if m.userProfile.Followers != "" {
		stats = append(stats, fmt.Sprintf("👥 %s followers", m.userProfile.Followers))
	}
	if m.userProfile.Following != "" && m.userProfile.Following != "0" {
		stats = append(stats, fmt.Sprintf("%s following", m.userProfile.Following))
	}
	if m.userProfile.PublicRepos > 0 {
		stats = append(stats, fmt.Sprintf("📦 %d repos", m.userProfile.PublicRepos))
	}

	statsStyled := lipgloss.NewStyle().
		Foreground(th.Accent).
		MaxWidth(rightW).
		Render(strings.Join(stats, "  •  "))

	// Meta line: location, company, blog
	var meta []string
	if m.userProfile.Location != "" {
		meta = append(meta, fmt.Sprintf("📍 %s", m.userProfile.Location))
	}
	if m.userProfile.Company != "" {
		meta = append(meta, fmt.Sprintf("🏢 %s", m.userProfile.Company))
	}
	if m.userProfile.Blog != "" {
		blog := m.userProfile.Blog
		runes := []rune(blog)
		if len(runes) > 22 {
			blog = string(runes[:20]) + ".."
		}
		meta = append(meta, fmt.Sprintf("🔗 %s", blog))
	}

	var metaStyled string
	if len(meta) > 0 {
		metaStyled = lipgloss.NewStyle().
			Foreground(th.Muted).
			MaxWidth(rightW).
			Render(strings.Join(meta, "  "))
	}

	// Assemble right side
	var infoItems []string
	infoItems = append(infoItems, bannerView, nameHandleLine)
	if bioStyled != "" {
		infoItems = append(infoItems, bioStyled)
	}
	if statsStyled != "" {
		infoItems = append(infoItems, statsStyled)
	}
	if metaStyled != "" {
		infoItems = append(infoItems, metaStyled)
	}

	// Compact GitHub Contribution Heatmap grid
	currentDetails := lipgloss.JoinVertical(lipgloss.Left, infoItems...)
	detailsH := lipgloss.Height(currentDetails)
	remainingH := innerH - detailsH

	// Heatmap needs 5 lines (header row with title and legend + 4 grid rows)
	if rightW >= 22 && remainingH >= 5 {
		maxWeeks := min(28, max(6, (rightW-2)/2))
		heatmap := renderContributionHeatmap(
			m.userProfile.ContributionDays,
			m.userProfile.TotalContributions,
			m.userProfile.PublicRepos,
			maxWeeks,
			rightW,
			th,
		)
		if heatmap != "" {
			infoItems = append(infoItems, heatmap)
		}
	}

	rightInfo := lipgloss.JoinVertical(lipgloss.Left, infoItems...)

	var profileContent string
	if usableW >= 46 {
		profileContent = lipgloss.JoinHorizontal(lipgloss.Top, avatar, "  ", rightInfo)
	} else {
		profileContent = lipgloss.JoinVertical(lipgloss.Left, avatar, rightInfo)
	}

	return WrapInTitledBorder(profileContent, width, height, "Profile", false, th)
}

func renderContributionHeatmap(days []int, totalContr string, publicRepos, maxWeeks, lineWidth int, th theme.Theme) string {
	if len(days) == 0 {
		return ""
	}

	levelColors := []lipgloss.Color{
		lipgloss.Color("#24283b"), // 0: dark slate
		lipgloss.Color("#3d59a1"), // 1: deep blue
		lipgloss.Color("#7aa2f7"), // 2: bright periwinkle
		lipgloss.Color("#73daca"), // 3: teal
		lipgloss.Color("#9ece6a"), // 4: vibrant green
	}

	numRows := 4 // Compact 4-row grid for terminal fit
	if maxWeeks <= 0 {
		maxWeeks = 8
	}
	totalCells := numRows * maxWeeks

	grid := make([]int, totalCells)
	if len(days) >= totalCells {
		copy(grid, days[len(days)-totalCells:])
	} else {
		copy(grid[totalCells-len(days):], days)
	}

	title := "Activity Heatmap"
	if totalContr != "" {
		title = fmt.Sprintf("Contributions: %s/yr", totalContr)
	}
	titleStyled := lipgloss.NewStyle().Foreground(th.Muted).Bold(true).Render(title)

	legend := fmt.Sprintf("Less %s %s %s %s %s More",
		lipgloss.NewStyle().Foreground(levelColors[0]).Render("▫"),
		lipgloss.NewStyle().Foreground(levelColors[1]).Render("▪"),
		lipgloss.NewStyle().Foreground(levelColors[2]).Render("▪"),
		lipgloss.NewStyle().Foreground(levelColors[3]).Render("▪"),
		lipgloss.NewStyle().Foreground(levelColors[4]).Render("▪"),
	)
	legendStyled := lipgloss.NewStyle().Foreground(th.Border).Render(legend)

	// Right-align legend on the same line as the title
	titleW := lipgloss.Width(titleStyled)
	legendW := lipgloss.Width(legendStyled)
	gap := lineWidth - titleW - legendW
	var headerLine string
	if gap >= 2 {
		headerLine = titleStyled + strings.Repeat(" ", gap) + legendStyled
	} else if lineWidth >= titleW {
		headerLine = titleStyled
	} else {
		headerLine = lipgloss.NewStyle().MaxWidth(lineWidth).Render(titleStyled)
	}

	var rowLines []string
	for r := 0; r < numRows; r++ {
		var rowSb strings.Builder
		for c := 0; c < maxWeeks; c++ {
			idx := c*numRows + r
			lvl := 0
			if idx < len(grid) {
				lvl = grid[idx]
			}
			if lvl < 0 || lvl >= len(levelColors) {
				lvl = 0
			}
			col := levelColors[lvl]
			char := "▪"
			if lvl == 0 {
				char = "▫"
			}
			rowSb.WriteString(lipgloss.NewStyle().Foreground(col).Render(char))
			if c < maxWeeks-1 {
				rowSb.WriteByte(' ')
			}
		}
		rowLines = append(rowLines, rowSb.String())
	}

	parts := []string{headerLine}
	parts = append(parts, rowLines...)
	return strings.Join(parts, "\n")
}
