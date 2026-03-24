package ui

import (
	"fmt"
	"strings"

	"github.com/purplefish32/spofree-cli/internal/types"
)

type nowPlayingModel struct {
	track    *types.Track
	paused   bool
	position float64
	duration float64
	quality  string
}

func formatTime(secs float64) string {
	m := int(secs) / 60
	s := int(secs) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

func (m nowPlayingModel) View(width int) string {
	if m.track == nil {
		return nowPlayingStyle.Render(dimStyle.Render("  Nothing playing"))
	}

	state := "▶"
	if m.paused {
		state = "⏸"
	}

	qualityLabel := ""
	if m.quality != "" {
		qualityLabel = "  " + dimStyle.Render(m.quality)
	}

	info := fmt.Sprintf("  %s  %s — %s  [%s]%s",
		state,
		titleStyle.Render(m.track.Title),
		artistStyle.Render(m.track.Artist.Name),
		dimStyle.Render(m.track.Album.Title),
		qualityLabel,
	)

	// Progress bar
	barWidth := width - 20
	if barWidth < 10 {
		barWidth = 10
	}
	if barWidth > 60 {
		barWidth = 60
	}

	progress := 0.0
	if m.duration > 0 {
		progress = m.position / m.duration
	}
	if progress > 1 {
		progress = 1
	}

	filled := int(float64(barWidth) * progress)
	empty := barWidth - filled

	bar := fmt.Sprintf("  %s %s%s %s",
		dimStyle.Render(formatTime(m.position)),
		selectedStyle.Render(strings.Repeat("━", filled)),
		dimStyle.Render(strings.Repeat("─", empty)),
		dimStyle.Render(formatTime(m.duration)),
	)

	return nowPlayingStyle.Render(info + "\n" + bar)
}
