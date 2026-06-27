package ui

import "github.com/charmbracelet/lipgloss"

// Panel border styles — inactive panels use a muted gray; the active panel
// gets a vibrant purple highlight so the user always knows where focus sits.
var (
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238"))

	ActivePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62"))

	// Sidebar tree items
	SelectedItemStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230"))

	CollectionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	DimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	// Panel chrome
	TitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255"))

	SeparatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238"))

	// Request panel
	URLStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	HeaderKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("141"))

	HeaderValStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	// HTTP execution state
	LoadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	// SavedStyle is shown briefly after a successful collection write.
	SavedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82"))

	// Footer help line
	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	// Per-method colour map — accessed via MethodStyle().
	methodColors = map[string]string{
		"GET":     "39",
		"POST":    "40",
		"PUT":     "220",
		"PATCH":   "51",
		"DELETE":  "196",
		"HEAD":    "141",
		"OPTIONS": "208",
	}
)

// MethodStyle returns a colour-coded Lip Gloss style for the given HTTP
// method. Unknown methods fall back to bright white.
func MethodStyle(method string) lipgloss.Style {
	if color, ok := methodColors[method]; ok {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
}

// StatusStyle returns a style whose colour reflects the HTTP status class:
// 2xx → green, 3xx → cyan, 4xx → yellow, 5xx → red.
func StatusStyle(code int) lipgloss.Style {
	switch {
	case code >= 200 && code < 300:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("40"))
	case code >= 300 && code < 400:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	case code >= 400 && code < 500:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	case code >= 500:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	default:
		return DimStyle
	}
}
