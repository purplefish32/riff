package ui

import "github.com/charmbracelet/lipgloss"

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}

func col(s string, width int, style lipgloss.Style) string {
	return style.Width(width).MaxWidth(width).Render(truncate(s, width-1))
}
