package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Theme struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Primary     lipgloss.Color `json:"primary"`
	Secondary   lipgloss.Color `json:"secondary"`
	Border      lipgloss.Color `json:"border"`
	FocusBorder lipgloss.Color `json:"focus_border"`
	Background  lipgloss.Color `json:"background"`
	Surface     lipgloss.Color `json:"surface"`
	Text        lipgloss.Color `json:"text"`
	Muted       lipgloss.Color `json:"muted"`
	Accent      lipgloss.Color `json:"accent"`
	Glamour     string         `json:"glamour"`
}

var Catalog = []Theme{
	{
		ID:          "catppuccin",
		Name:        "Catppuccin Mocha",
		Primary:     lipgloss.Color("#CBA6F7"),
		Secondary:   lipgloss.Color("#89B4FA"),
		Border:      lipgloss.Color("#6C7086"),
		FocusBorder: lipgloss.Color("#F9E2AF"),
		Background:  lipgloss.Color("#1E1E2E"),
		Surface:     lipgloss.Color("#313244"),
		Text:        lipgloss.Color("#CDD6F4"),
		Muted:       lipgloss.Color("#585B70"),
		Accent:      lipgloss.Color("#A6E3A1"),
		Glamour:     "dark",
	},
	{
		ID:          "tokyonight",
		Name:        "Tokyo Night",
		Primary:     lipgloss.Color("#7AA2F7"),
		Secondary:   lipgloss.Color("#BB9AF7"),
		Border:      lipgloss.Color("#3D59A1"),
		FocusBorder: lipgloss.Color("#E0AF68"),
		Background:  lipgloss.Color("#1A1B26"),
		Surface:     lipgloss.Color("#24283B"),
		Text:        lipgloss.Color("#C0CAF5"),
		Muted:       lipgloss.Color("#565F89"),
		Accent:      lipgloss.Color("#9ECE6A"),
		Glamour:     "tokyo-night",
	},
	{
		ID:          "dracula",
		Name:        "Dracula",
		Primary:     lipgloss.Color("#BD93F9"),
		Secondary:   lipgloss.Color("#8BE9FD"),
		Border:      lipgloss.Color("#6272A4"),
		FocusBorder: lipgloss.Color("#F1FA8C"),
		Background:  lipgloss.Color("#282A36"),
		Surface:     lipgloss.Color("#44475A"),
		Text:        lipgloss.Color("#F8F8F2"),
		Muted:       lipgloss.Color("#6272A4"),
		Accent:      lipgloss.Color("#50FA7B"),
		Glamour:     "dracula",
	},
	{
		ID:          "nord",
		Name:        "Nord",
		Primary:     lipgloss.Color("#88C0D0"),
		Secondary:   lipgloss.Color("#81A1C1"),
		Border:      lipgloss.Color("#4C566A"),
		FocusBorder: lipgloss.Color("#EBCB8B"),
		Background:  lipgloss.Color("#2E3440"),
		Surface:     lipgloss.Color("#3B4252"),
		Text:        lipgloss.Color("#ECEFF4"),
		Muted:       lipgloss.Color("#4C566A"),
		Accent:      lipgloss.Color("#A3BE8C"),
		Glamour:     "nord",
	},
	{
		ID:          "gruvbox",
		Name:        "Gruvbox Dark",
		Primary:     lipgloss.Color("#FE8019"),
		Secondary:   lipgloss.Color("#FABD2F"),
		Border:      lipgloss.Color("#504945"),
		FocusBorder: lipgloss.Color("#B8BB26"),
		Background:  lipgloss.Color("#282828"),
		Surface:     lipgloss.Color("#3C3836"),
		Text:        lipgloss.Color("#EBDBB2"),
		Muted:       lipgloss.Color("#7C6F64"),
		Accent:      lipgloss.Color("#8EC07C"),
		Glamour:     "dark",
	},
	{
		ID:          "cyberpunk",
		Name:        "Cyberpunk Neon",
		Primary:     lipgloss.Color("#FF007F"),
		Secondary:   lipgloss.Color("#00FFFF"),
		Border:      lipgloss.Color("#7D56F4"),
		FocusBorder: lipgloss.Color("#FFE600"),
		Background:  lipgloss.Color("#100028"),
		Surface:     lipgloss.Color("#240046"),
		Text:        lipgloss.Color("#E0AAFF"),
		Muted:       lipgloss.Color("#5A189A"),
		Accent:      lipgloss.Color("#39FF14"),
		Glamour:     "dark",
	},
	{
		ID:          "classic",
		Name:        "Classic Purple",
		Primary:     lipgloss.Color("#7D56F4"),
		Secondary:   lipgloss.Color("#00A7FF"),
		Border:      lipgloss.Color("#7D56F4"),
		FocusBorder: lipgloss.Color("#BA8E23"),
		Background:  lipgloss.Color("#1E1E2E"),
		Surface:     lipgloss.Color("#3A1D70"),
		Text:        lipgloss.Color("#FFFFFF"),
		Muted:       lipgloss.Color("#888888"),
		Accent:      lipgloss.Color("#A5FFD6"),
		Glamour:     "dark",
	},
}

func GetTheme(idOrName string) Theme {
	clean := strings.ToLower(strings.TrimSpace(idOrName))
	for _, t := range Catalog {
		if strings.ToLower(t.ID) == clean || strings.ToLower(t.Name) == clean {
			return t
		}
	}
	return Catalog[0] // default to Catppuccin
}

func AvailableThemeIDs() []string {
	var ids []string
	for _, t := range Catalog {
		ids = append(ids, t.ID)
	}
	return ids
}
