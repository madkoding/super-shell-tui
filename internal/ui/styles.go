package ui

import "github.com/charmbracelet/lipgloss"

const sidebarWidth = 30

var (
	accent = lipgloss.AdaptiveColor{Light: "#5A3FC0", Dark: "#9D7CFF"}
	muted  = lipgloss.AdaptiveColor{Light: "#6B6B6B", Dark: "#8A8A8A"}

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(accent).
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().Foreground(muted).Padding(0, 1)

	armedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#F5C542")).
			Padding(0, 1)

	scrollStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#7CC4FF")).
			Padding(0, 1)

	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent)

	sidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(muted).
			Padding(0, 1)

	labelStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)
	valueStyle = lipgloss.NewStyle().Foreground(muted)
)
