package ui

import (
	"github.com/charmbracelet/lipgloss"
)

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
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
	if width <= 0 {
		return ""
	}
	return style.Width(width).MaxWidth(width).Render(truncate(s, width-1))
}

func colRight(s string, width int, style lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	return style.Width(width).MaxWidth(width).AlignHorizontal(lipgloss.Right).Render(truncate(s, width-1))
}
