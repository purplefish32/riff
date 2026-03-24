package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/purplefish32/spofree-cli/internal/types"
)

type searchMode int

const (
	modeTrack searchMode = iota
	modeAlbum
	modeBrowseAlbum
)

type searchModel struct {
	input       textinput.Model
	mode        searchMode
	results     []types.Track
	albums      []types.AlbumFull
	albumTracks []types.Track
	albumTitle  string
	cursor      int
	loading     bool
}

type searchResultMsg struct {
	tracks []types.Track
	err    error
}

type albumSearchResultMsg struct {
	albums []types.AlbumFull
	err    error
}

type albumTracksMsg struct {
	tracks []types.Track
	title  string
	err    error
}

func newSearchModel() searchModel {
	ti := textinput.New()
	ti.Placeholder = "Search tracks..."
	ti.Prompt = "🔍 "
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 40

	return searchModel{input: ti}
}

func (m searchModel) Update(msg tea.Msg) (searchModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			max := m.listLen() - 1
			if m.cursor < max {
				m.cursor++
			}
		case "tab":
			if m.input.Focused() {
				switch m.mode {
				case modeTrack:
					m.mode = modeAlbum
					m.input.Placeholder = "Search albums..."
				case modeAlbum:
					m.mode = modeTrack
					m.input.Placeholder = "Search tracks..."
				}
				return m, nil
			}
		case "backspace":
			if !m.input.Focused() && m.mode == modeBrowseAlbum {
				m.mode = modeAlbum
				m.cursor = 0
				return m, nil
			}
		}
	case searchResultMsg:
		m.loading = false
		if msg.err == nil {
			m.results = msg.tracks
			m.cursor = 0
			m.input.Blur()
		}
	case albumSearchResultMsg:
		m.loading = false
		if msg.err == nil {
			m.albums = msg.albums
			m.cursor = 0
			m.input.Blur()
		}
	case albumTracksMsg:
		m.loading = false
		if msg.err == nil {
			m.albumTracks = msg.tracks
			m.albumTitle = msg.title
			m.mode = modeBrowseAlbum
			m.cursor = 0
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m searchModel) listLen() int {
	switch m.mode {
	case modeAlbum:
		return len(m.albums)
	case modeBrowseAlbum:
		return len(m.albumTracks)
	default:
		return len(m.results)
	}
}

func (m searchModel) View(width int) string {
	modeLabel := "tracks"
	if m.mode == modeAlbum || m.mode == modeBrowseAlbum {
		modeLabel = "albums"
	}

	s := searchPromptStyle.Render("Search: ") + m.input.View()
	if m.input.Focused() {
		s += dimStyle.Render(fmt.Sprintf("  [tab: %s]", modeLabel))
	}
	s += "\n\n"

	if m.loading {
		s += dimStyle.Render("  Searching...")
		return s
	}

	switch m.mode {
	case modeBrowseAlbum:
		s += titleStyle.Render(fmt.Sprintf("  %s", m.albumTitle))
		s += dimStyle.Render("  (backspace to go back)") + "\n\n"
		for i, track := range m.albumTracks {
			duration := fmt.Sprintf("%d:%02d", track.Duration/60, track.Duration%60)
			num := fmt.Sprintf("%2d.", track.TrackNumber)
			if i == m.cursor {
				s += selectedStyle.Render(fmt.Sprintf("▸ %s %s", num, track.Title)) +
					"  " + dimStyle.Render(duration) + "\n"
			} else {
				s += fmt.Sprintf("  %s %s  %s\n",
					dimStyle.Render(num),
					track.Title,
					dimStyle.Render(duration),
				)
			}
		}
	case modeAlbum:
		for i, album := range m.albums {
			artist := ""
			if len(album.Artists) > 0 {
				artist = album.Artists[0].Name
			}
			year := ""
			if len(album.ReleaseDate) >= 4 {
				year = album.ReleaseDate[:4]
			}
			info := fmt.Sprintf("%d tracks", album.NumberOfTracks)
			if year != "" {
				info = year + " · " + info
			}
			if i == m.cursor {
				s += selectedStyle.Render(fmt.Sprintf("▸ %s — %s", album.Title, artist)) +
					"  " + dimStyle.Render(info) + "\n"
			} else {
				s += fmt.Sprintf("  %s — %s  %s\n",
					album.Title,
					artist,
					dimStyle.Render(info),
				)
			}
		}
	default:
		for i, track := range m.results {
			duration := fmt.Sprintf("%d:%02d", track.Duration/60, track.Duration%60)
			if i == m.cursor {
				s += selectedStyle.Render(fmt.Sprintf("▸ %s — %s", track.Title, track.Artist.Name)) +
					"  " + dimStyle.Render(duration) + "\n"
			} else {
				s += fmt.Sprintf("  %s — %s  %s\n",
					track.Title,
					track.Artist.Name,
					dimStyle.Render(duration),
				)
			}
		}
	}

	return s
}

func (m searchModel) selectedTrack() *types.Track {
	switch m.mode {
	case modeBrowseAlbum:
		if len(m.albumTracks) == 0 {
			return nil
		}
		return &m.albumTracks[m.cursor]
	default:
		if len(m.results) == 0 {
			return nil
		}
		return &m.results[m.cursor]
	}
}

func (m searchModel) selectedAlbum() *types.AlbumFull {
	if m.mode != modeAlbum || len(m.albums) == 0 {
		return nil
	}
	return &m.albums[m.cursor]
}

func (m searchModel) browsingAlbumTracks() []types.Track {
	if m.mode != modeBrowseAlbum {
		return nil
	}
	return m.albumTracks
}
