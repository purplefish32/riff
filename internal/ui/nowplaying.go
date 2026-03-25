package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
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

	state := "▶"
	if m.paused {
		state = "⏸"
	}

	sep := dimStyle.Render(" · ")

	parts := []string{
		"  " + state,
		titleStyle.Render(m.track.Title),
		artistStyle.Render(m.track.Artist.Name),
		dimStyle.Render(m.track.Album.Title),
	}
	if m.audioInfo != "" {
		parts = append(parts, dimStyle.Render(m.audioInfo))
	} else if m.quality != "" {
		parts = append(parts, dimStyle.Render(m.quality))
	}
	if m.liked {
		parts = append(parts, titleStyle.Render("♥"))
	}
	parts = append(parts, dimStyle.Render(fmt.Sprintf("vol:%d%%", m.volume)))

	info := strings.Join(parts, sep)

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
	bar := fmt.Sprintf("  %s %s %s",
		dimStyle.Render(leftTime),
		m.progress.ViewAs(pct),
		dimStyle.Render(formatTime(m.duration)),
	)

	textBlock := nowPlayingStyle.Render(info + "\n" + bar)

	// Render album art to the left when enabled and available.
	if m.showAlbumArt && !noColor && m.albumArt != "" {
		return lipgloss.JoinHorizontal(lipgloss.Top, m.albumArt, textBlock)
	}

	return textBlock
}
