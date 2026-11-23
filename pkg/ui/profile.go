package ui

import (
	"fmt"
	"strings"

	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderUserProfileCard(width, height int, th theme.Theme) string {
	if m.userProfile == nil {
		return ""
	}

	usableW := max(20, width-4)

	avatar := m.userProfile.AvatarASCII
	if avatar == "" {
		if m.cfg != nil && m.cfg.User != "" {
			avatar = m.cfg.User
		} else {
			avatar = m.userProfile.Username
		}
	}

	infoW := usableW
	if usableW >= 42 {
		infoW = max(16, usableW-24)
	}

	name := m.userProfile.Name
	if name == "" {
		name = m.userProfile.Username
	}
	nameStyled := lipgloss.NewStyle().
		Bold(true).
		Foreground(th.Primary).
		MaxWidth(infoW).
		Render(name)

	handleStyled := lipgloss.NewStyle().
		Foreground(th.Muted).
		MaxWidth(infoW).
		Render(fmt.Sprintf("@%s", m.userProfile.Username))

	bio := strings.TrimSpace(m.userProfile.Bio)
	var bioStyled string
	if bio != "" {
		bioStyled = lipgloss.NewStyle().
			Foreground(th.Text).
			MaxWidth(infoW).
			Render(bio)
	}

	var stats []string
	if m.userProfile.Followers != "" {
		stats = append(stats, fmt.Sprintf("👥 %s followers", m.userProfile.Followers))
	}
	if m.userProfile.Location != "" {
		stats = append(stats, fmt.Sprintf("📍 %s", m.userProfile.Location))
	}
	if m.userProfile.Company != "" {
		stats = append(stats, fmt.Sprintf("🏢 %s", m.userProfile.Company))
	}

	statsStyled := lipgloss.NewStyle().
		Foreground(th.Accent).
		MaxWidth(infoW).
		Render(strings.Join(stats, "  "))

	header := lipgloss.JoinVertical(
		lipgloss.Left,
		nameStyled,
		handleStyled,
	)

	var profileContent string
	if usableW >= 42 {
		leftAvatar := avatar
		var infoItems []string
		infoItems = append(infoItems, header)
		if bioStyled != "" {
			infoItems = append(infoItems, bioStyled)
		}
		if statsStyled != "" {
			infoItems = append(infoItems, statsStyled)
		}
		rightInfo := lipgloss.JoinVertical(lipgloss.Left, infoItems...)
		profileContent = lipgloss.JoinHorizontal(lipgloss.Center, leftAvatar, "  ", rightInfo)
	} else {
		var allItems []string
		allItems = append(allItems, avatar, header)
		if bioStyled != "" {
			allItems = append(allItems, bioStyled)
		}
		if statsStyled != "" {
			allItems = append(allItems, statsStyled)
		}
		profileContent = lipgloss.JoinVertical(lipgloss.Left, allItems...)
	}

	cardStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(th.Border).
		Padding(0, 1).
		Width(max(10, width-2)).
		MaxWidth(width)

	if height > 0 {
		cardStyle = cardStyle.Height(max(1, height-2)).MaxHeight(height)
	}

	return cardStyle.Render(strings.TrimRight(profileContent, "\n"))
}
