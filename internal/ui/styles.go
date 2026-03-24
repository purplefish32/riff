package ui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6AC1"))

	artistStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9AEDFE"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	nowPlayingStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(lipgloss.Color("#444444")).
			Padding(0, 1)

	searchPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF6AC1")).
				Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#44475A")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Bold(true).
			Underline(true)

	playingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6AC1")).
			Bold(true)

	playingSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF6AC1")).
				Background(lipgloss.Color("#44475A")).
				Bold(true)
)
