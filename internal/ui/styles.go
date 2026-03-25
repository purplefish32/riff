package ui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

var noColor = os.Getenv("NO_COLOR") != ""

var (
	titleStyle           lipgloss.Style
	artistStyle          lipgloss.Style
	dimStyle             lipgloss.Style
	nowPlayingStyle      lipgloss.Style
	searchPromptStyle    lipgloss.Style
	selectedStyle        lipgloss.Style
	normalStyle          lipgloss.Style
	headerStyle          lipgloss.Style
	playingStyle         lipgloss.Style
	playingSelectedStyle lipgloss.Style
)

var (
	errorStyle      lipgloss.Style
	downloadIcon    lipgloss.Style
	overlayBorder   lipgloss.Style
	accentColor     lipgloss.Color
	selectionStripe lipgloss.Style
	activeTabStyle  lipgloss.Style
	altRowBg        lipgloss.Style
)

func init() {
	if noColor {
		plain := lipgloss.NewStyle()
		titleStyle = plain.Bold(true)
		artistStyle = plain
		dimStyle = plain
		nowPlayingStyle = plain
		searchPromptStyle = plain.Bold(true)
		selectedStyle = plain.Bold(true).Reverse(true)
		normalStyle = plain
		headerStyle = plain.Bold(true).Underline(true)
		playingStyle = plain.Bold(true)
		playingSelectedStyle = plain.Bold(true).Reverse(true)
		errorStyle = plain.Bold(true)
		downloadIcon = plain
		overlayBorder = plain.Border(lipgloss.RoundedBorder())
		selectionStripe = plain.Bold(true)
		activeTabStyle = plain.Bold(true).Underline(true)
		altRowBg = plain
		return
	}

	accentColor = lipgloss.Color("#FF6AC1")
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	downloadIcon = lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B"))
	overlayBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor)

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

	selectionStripe = lipgloss.NewStyle().
		Foreground(accentColor).
		Bold(true)

	activeTabStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(accentColor).
		Underline(true)

	altRowBg = lipgloss.NewStyle().
		Background(lipgloss.Color("#1A1A2E"))
}
