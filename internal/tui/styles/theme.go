package styles

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Bg           lipgloss.Color
	BgAlt        lipgloss.Color
	Panel        lipgloss.Color
	Border       lipgloss.Color
	BorderActive lipgloss.Color
	Text         lipgloss.Color
	Dim          lipgloss.Color
	Muted        lipgloss.Color
	Accent       lipgloss.Color
	Accent2      lipgloss.Color
	Green        lipgloss.Color
}

// CatppuccinMocha is the default theme.
var CatppuccinMocha = Theme{
	Bg:           "#1e1e2e",
	BgAlt:        "#181825",
	Panel:        "#313244",
	Border:       "#45475a",
	BorderActive: "#cba6f7",
	Text:         "#cdd6f4",
	Dim:          "#7f849c",
	Muted:        "#585b70",
	Accent:       "#cba6f7",
	Accent2:      "#89b4fa",
	Green:        "#a6e3a1",
}

var TokyoNight = Theme{
	Bg:           "#1a1b26",
	BgAlt:        "#16161e",
	Panel:        "#24283b",
	Border:       "#414868",
	BorderActive: "#7aa2f7",
	Text:         "#c0caf5",
	Dim:          "#565f89",
	Muted:        "#3b3d57",
	Accent:       "#7aa2f7",
	Accent2:      "#bb9af7",
	Green:        "#9ece6a",
}

var Dracula = Theme{
	Bg:           "#282a36",
	BgAlt:        "#21222c",
	Panel:        "#343746",
	Border:       "#44475a",
	BorderActive: "#bd93f9",
	Text:         "#f8f8f2",
	Dim:          "#6272a4",
	Muted:        "#44475a",
	Accent:       "#bd93f9",
	Accent2:      "#8be9fd",
	Green:        "#50fa7b",
}

var Active = CatppuccinMocha

// panelBorder renders a lipgloss border with the panel title overlapping the top edge,
// matching the OpenCode / Bubble Tea lipgloss aesthetic.
func PanelStyle(t Theme, focused bool) lipgloss.Style {
	borderColor := t.Border
	if focused {
		borderColor = t.BorderActive
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)
}

func TitleStyle(t Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Dim).
		Bold(false)
}

func SelectedStyle(t Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Accent).
		Bold(true)
}

func TextStyle(t Theme) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Text)
}

func DimStyle(t Theme) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Dim)
}
