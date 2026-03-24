package ui

import (
	"fmt"
	"strings"

	"github.com/purplefish32/riff/internal/types"
)

type nowPlayingModel struct {
	track    *types.Track
	paused   bool
	liked    bool
	position float64
	duration float64
	quality  string
	volume   int
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

	heart := ""
	if m.liked {
		heart = "  " + titleStyle.Render("♥")
	}

	vol := dimStyle.Render(fmt.Sprintf("  vol:%d%%", m.volume))

	info := fmt.Sprintf("  %s  %s — %s  [%s]%s%s%s",
		state,
		titleStyle.Render(m.track.Title),
		artistStyle.Render(m.track.Artist.Name),
		dimStyle.Render(m.track.Album.Title),
		qualityLabel,
		heart,
		vol,
	)

	// Ultra-narrow: hide progress bar entirely
	if width < 40 {
		return nowPlayingStyle.Render(info)
	}

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
		titleStyle.Render(strings.Repeat("━", filled)),
		dimStyle.Render(strings.Repeat("─", empty)),
		dimStyle.Render(formatTime(m.duration)),
	)

	return nowPlayingStyle.Render(info + "\n" + bar)
}
