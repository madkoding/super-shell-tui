package ui

import "github.com/charmbracelet/lipgloss"

// styles is the theme, built from the configured colors.
type styles struct {
	header, status, armed, scroll lipgloss.Style
	pane, sidebar, label, value   lipgloss.Style
}

func newStyles(accentHex, mutedHex string) styles {
	var accent, muted lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "#5A3FC0", Dark: "#9D7CFF"},
		lipgloss.AdaptiveColor{Light: "#6B6B6B", Dark: "#8A8A8A"}
	if accentHex != "" {
		accent = lipgloss.Color(accentHex)
	}
	if mutedHex != "" {
		muted = lipgloss.Color(mutedHex)
	}
	banner := func(bg string) lipgloss.Style {
		return lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color(bg)).
			Padding(0, 1)
	}
	return styles{
		header: lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(accent).
			Padding(0, 1),
		status:  lipgloss.NewStyle().Foreground(muted).Padding(0, 1),
		armed:   banner("#F5C542"),
		scroll:  banner("#7CC4FF"),
		pane:    lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent),
		sidebar: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(muted).Padding(0, 1),
		label:   lipgloss.NewStyle().Foreground(accent).Bold(true),
		value:   lipgloss.NewStyle().Foreground(muted),
	}
}
