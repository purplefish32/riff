package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/purplefish32/riff/internal/types"
)

type nowPlayingModel struct {
	track         *types.Track
	paused        bool
	liked         bool
	position      float64
	duration      float64
	quality       string
	volume        int
	audioInfo     string
	progress      progress.Model
	showRemaining bool
	coverID       string
	albumArt      string
	showAlbumArt  bool
}

func newNowPlayingModel() nowPlayingModel {
	prog := progress.New(
		progress.WithGradient("#C084FC", "#38BDF8"),
		progress.WithoutPercentage(),
	)
	return nowPlayingModel{progress: prog}
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

	state := titleStyle.Render("▶")
	if m.paused {
		state = dimStyle.Render("⏸")
	}

	// Line 1: state + title
	line1 := fmt.Sprintf("  %s  %s", state, titleStyle.Render(m.track.Title))

	// Line 2: metadata
	var meta []string
	meta = append(meta, artistStyle.Render(m.track.Artist.Name))
	meta = append(meta, dimStyle.Render(m.track.Album.Title))
	if m.audioInfo != "" {
		meta = append(meta, dimStyle.Render(m.audioInfo))
	} else if m.quality != "" {
		meta = append(meta, dimStyle.Render(m.quality))
	}
	if m.liked {
		meta = append(meta, titleStyle.Render("♥"))
	}
	meta = append(meta, dimStyle.Render(fmt.Sprintf("vol:%d%%", m.volume)))
	line2 := "     " + strings.Join(meta, dimStyle.Render(" · "))

	// Ultra-narrow: no progress bar
	if width < 40 {
		return nowPlayingStyle.Render(line1 + "\n" + line2)
	}

	// Line 3: progress bar
	barWidth := width - 20
	if barWidth < 10 {
		barWidth = 10
	}
	if barWidth > 60 {
		barWidth = 60
	}

	pct := 0.0
	if m.duration > 0 {
		pct = m.position / m.duration
	}
	if pct > 1 {
		pct = 1
	}

	m.progress.Width = barWidth
	leftTime := formatTime(m.position)
	if m.showRemaining && m.duration > 0 {
		remaining := m.duration - m.position
		if remaining < 0 {
			remaining = 0
		}
		leftTime = "-" + formatTime(remaining)
	}
	line3 := fmt.Sprintf("  %s %s %s",
		dimStyle.Render(leftTime),
		m.progress.ViewAs(pct),
		dimStyle.Render(formatTime(m.duration)),
	)

	if m.showAlbumArt && !noColor && m.albumArt != "" {
		// Manually join art lines with text lines to avoid alignment issues
		artLines := strings.Split(m.albumArt, "\n")
		textLines := []string{line1, line2, line3}
		var combined []string
		for i := 0; i < len(artLines) || i < len(textLines); i++ {
			art := ""
			if i < len(artLines) {
				art = artLines[i]
			} else {
				art = strings.Repeat(" ", 8)
			}
			text := ""
			if i < len(textLines) {
				text = textLines[i]
			}
			combined = append(combined, "  "+art+" "+text)
		}
		return nowPlayingStyle.Render(strings.Join(combined, "\n"))
	}

	return nowPlayingStyle.Render(line1 + "\n" + line2 + "\n" + line3)
}
