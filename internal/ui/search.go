package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/purplefish32/riff/internal/types"
)

type searchMode int

const (
	modeTrack searchMode = iota
	modeAlbum
	modeArtist
	modeBrowseAlbum
)

type searchModel struct {
	input       textinput.Model
	mode        searchMode
	results     []types.Track
	albums      []types.AlbumFull
	artists     []types.ArtistFull
	albumTracks []types.Track
	albumTitle  string
	cursor      int
	loading     bool
	lastQuery   string
	lastMode    searchMode
}

type searchResultMsg struct {
	tracks []types.Track
	err    error
}

type albumSearchResultMsg struct {
	albums []types.AlbumFull
	err    error
}

type artistSearchResultMsg struct {
	artists []types.ArtistFull
	err     error
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
					m.mode = modeArtist
					m.input.Placeholder = "Search artists..."
				case modeArtist:
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
			m.mode = modeAlbum
			m.cursor = 0
			m.input.Blur()
		}
	case artistSearchResultMsg:
		m.loading = false
		if msg.err == nil {
			m.artists = msg.artists
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
	case modeArtist:
		return len(m.artists)
	case modeAlbum:
		return len(m.albums)
	case modeBrowseAlbum:
		return len(m.albumTracks)
	default:
		return len(m.results)
	}
}

// Fixed column widths
const (
	colLike     = 2
	colNum      = 4
	colDuration = 6
	colYear     = 8
	colTracks   = 8
)

type trackCols struct {
	artist, album, title int
	showAlbum, showYear  bool
}

func computeTrackCols(width int) trackCols {
	// Below 60 cols: hide album and year columns
	if width < 60 {
		fixed := colLike + colNum + colDuration + 4
		avail := width - fixed
		if avail < 20 {
			avail = 20
		}
		return trackCols{
			artist:    avail * 40 / 100,
			title:     avail * 60 / 100,
			showAlbum: false,
			showYear:  false,
		}
	}
	// Below 90 cols: hide year column
	if width < 90 {
		fixed := colLike + colNum + colDuration + 4
		avail := width - fixed
		return trackCols{
			artist:    avail * 25 / 100,
			title:     avail * 40 / 100,
			album:     avail * 35 / 100,
			showAlbum: true,
			showYear:  false,
		}
	}
	// Full width
	fixed := colLike + colNum + colYear + colDuration + 4
	avail := width - fixed
	return trackCols{
		artist:    avail * 30 / 100,
		title:     avail * 40 / 100,
		album:     avail * 30 / 100,
		showAlbum: true,
		showYear:  true,
	}
}

type albumCols struct {
	title, artist int
}

func computeAlbumCols(width int) albumCols {
	fixed := colYear + colTracks + 4
	avail := width - fixed
	if avail < 30 {
		avail = 30
	}
	return albumCols{
		title:  avail * 50 / 100,
		artist: avail * 50 / 100,
	}
}

func trackHeader(tc trackCols) string {
	s := "  " +
		col("#", colNum, headerStyle) +
		col("Artist", tc.artist, headerStyle) +
		col("Title", tc.title, headerStyle)
	if tc.showAlbum {
		s += col("Album", tc.album, headerStyle)
	}
	if tc.showYear {
		s += col("Year", colYear, headerStyle)
	}
	s += col("Time", colDuration, headerStyle)
	return s
}

func statusIcons(liked bool, downloaded bool) string {
	l := " "
	if liked {
		l = titleStyle.Render("♥")
	}
	d := " "
	if downloaded {
		d = downloadIcon.Render("↓")
	}
	return l + d
}

func trackYear(track types.Track) string {
	if len(track.Album.ReleaseDate) >= 4 {
		return track.Album.ReleaseDate[:4]
	}
	return ""
}

func trackRow(i int, track types.Track, selected bool, liked bool, downloaded bool, tc trackCols) string {
	duration := fmt.Sprintf("%d:%02d", track.Duration/60, track.Duration%60)
	num := fmt.Sprintf("%d", i+1)
	icons := statusIcons(liked, downloaded)

	if selected {
		s := titleStyle.Render("▸") + icons +
			col(num, colNum, selectedStyle) +
			col(track.Artist.Name, tc.artist, selectedStyle) +
			col(track.Title, tc.title, selectedStyle)
		if tc.showAlbum {
			s += col(track.Album.Title, tc.album, selectedStyle)
		}
		if tc.showYear {
			s += col(trackYear(track), colYear, selectedStyle)
		}
		s += col(duration, colDuration, selectedStyle)
		return s
	}

	s := " " + icons +
		col(num, colNum, dimStyle) +
		col(track.Artist.Name, tc.artist, artistStyle) +
		col(track.Title, tc.title, normalStyle)
	if tc.showAlbum {
		s += col(track.Album.Title, tc.album, dimStyle)
	}
	if tc.showYear {
		s += col(trackYear(track), colYear, dimStyle)
	}
	s += col(duration, colDuration, dimStyle)
	return s
}

func albumHeader(ac albumCols) string {
	return "  " +
		col("Album", ac.title, headerStyle) +
		col("Artist", ac.artist, headerStyle) +
		col("Year", colYear, headerStyle) +
		col("Tracks", colTracks, headerStyle)
}

func albumRow(i int, album types.AlbumFull, selected bool, ac albumCols) string {
	artist := ""
	if len(album.Artists) > 0 {
		artist = album.Artists[0].Name
	}
	year := ""
	if len(album.ReleaseDate) >= 4 {
		year = album.ReleaseDate[:4]
	}
	tracks := fmt.Sprintf("%d", album.NumberOfTracks)

	if selected {
		return titleStyle.Render("▸ ") +
			col(album.Title, ac.title, selectedStyle) +
			col(artist, ac.artist, selectedStyle) +
			col(year, colYear, selectedStyle) +
			col(tracks, colTracks, selectedStyle)
	}

	return "  " +
		col(album.Title, ac.title, normalStyle) +
		col(artist, ac.artist, artistStyle) +
		col(year, colYear, dimStyle) +
		col(tracks, colTracks, dimStyle)
}

func artistHeader(width int) string {
	w := width - 8
	if w < 20 {
		w = 20
	}
	return "  " +
		col("#", colNum, headerStyle) +
		col("Artist", w, headerStyle)
}

func artistRow(i int, artist types.ArtistFull, selected bool, width int) string {
	w := width - 8
	if w < 20 {
		w = 20
	}
	num := fmt.Sprintf("%d", i+1)

	if selected {
		return titleStyle.Render("▸ ") +
			col(num, colNum, selectedStyle) +
			col(artist.Name, w, selectedStyle)
	}

	return "  " +
		col(num, colNum, dimStyle) +
		col(artist.Name, w, artistStyle)
}

func (m searchModel) View(width int, isLiked func(int) bool, isDownloaded func(types.Track) bool) string {
	modeLabels := map[searchMode]string{
		modeTrack: "tracks", modeAlbum: "albums", modeArtist: "artists",
		modeBrowseAlbum: "albums",
	}
	modeLabel := modeLabels[m.mode]

	s := searchPromptStyle.Render("Search: ") + m.input.View()
	if m.input.Focused() {
		s += dimStyle.Render(fmt.Sprintf("  [tab: %s]", modeLabel))
	}
	s += "\n\n"

	if m.loading {
		s += dimStyle.Render("  Searching...")
		return s
	}

	tc := computeTrackCols(width)
	ac := computeAlbumCols(width)

	switch m.mode {
	case modeBrowseAlbum:
		s += titleStyle.Render(fmt.Sprintf("  %s", m.albumTitle))
		s += dimStyle.Render("  (backspace to go back)") + "\n\n"
		s += trackHeader(tc) + "\n"
		for i, track := range m.albumTracks {
			s += trackRow(i, track, i == m.cursor, isLiked(track.ID), isDownloaded(track), tc) + "\n"
		}
	case modeArtist:
		if len(m.artists) > 0 {
			s += artistHeader(width) + "\n"
			for i, artist := range m.artists {
				s += artistRow(i, artist, i == m.cursor, width) + "\n"
			}
		}
	case modeAlbum:
		if len(m.albums) > 0 {
			s += albumHeader(ac) + "\n"
			for i, album := range m.albums {
				s += albumRow(i, album, i == m.cursor, ac) + "\n"
			}
		}
	default:
		if len(m.results) > 0 {
			s += trackHeader(tc) + "\n"
			for i, track := range m.results {
				s += trackRow(i, track, i == m.cursor, isLiked(track.ID), isDownloaded(track), tc) + "\n"
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

func (m searchModel) selectedArtist() *types.ArtistFull {
	if m.mode != modeArtist || len(m.artists) == 0 {
		return nil
	}
	return &m.artists[m.cursor]
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
