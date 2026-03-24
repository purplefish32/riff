package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/purplefish32/riff/internal/api"
	"github.com/purplefish32/riff/internal/downloader"
	"github.com/purplefish32/riff/internal/persistence"
	"github.com/purplefish32/riff/internal/player"
	"github.com/purplefish32/riff/internal/types"
)

type errMsg struct{ err error }

// DownloadUpdateMsg is sent when download status changes.
type DownloadUpdateMsg struct{}

type streamURLMsg struct {
	url string
	err error
}

type trackEndedMsg struct{ gen int }

type queueAlbumMsg struct {
	tracks []types.Track
	err    error
}

var qualities = []string{"LOW", "HIGH", "LOSSLESS", "HI_RES"}

type tickMsg struct{}

type viewTab int

const (
	tabQueue viewTab = iota
	tabLiked
	tabDownloads
)

// inputMode represents the current UI state. Every key event is routed
// through exactly one mode handler, eliminating boolean-soup guards.
type inputMode int

const (
	modeNormal       inputMode = iota // browsing tabs
	modeSearchInput                   // typing in search box
	modeSearchBrowse                  // navigating search results
	modeHelp                          // help overlay
)

type App struct {
	mode        inputMode
	activeTab   viewTab
	search      searchModel
	nowPlaying  nowPlayingModel
	tracklist   []types.Track
	trackPos    int // index of currently playing track, -1 if none
	likedCursor int
	queueCursor int // cursor for browsing the queue view
	client      *api.Client
	player      *player.Player
	likes       *persistence.LikedStore
	dl          *downloader.Downloader
	config      *persistence.Config
	queueStore  *persistence.QueueStore
	playGen     int
	quality     int
	volume      int
	width       int
	height      int
	err         error
}

func NewApp(client *api.Client, player *player.Player, likes *persistence.LikedStore, dl *downloader.Downloader, cfg *persistence.Config, qs *persistence.QueueStore) App {
	mode := modeNormal
	if len(qs.Tracks) == 0 {
		mode = modeSearchInput
	}
	return App{
		mode:       mode,
		search:     newSearchModel(),
		client:     client,
		player:     player,
		likes:      likes,
		dl:         dl,
		config:     cfg,
		queueStore: qs,
		tracklist:  qs.Tracks,
		trackPos:   qs.Position,
		quality:    cfg.QualityIndex(),
		volume:     cfg.Volume,
	}
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (a App) Init() tea.Cmd {
	if a.mode == modeSearchInput {
		return tea.Batch(a.search.input.Focus(), tick())
	}
	return tick()
}

func (a App) makeWaitForTrackEnd(gen int) tea.Cmd {
	return func() tea.Msg {
		a.player.WaitForEnd()
		return trackEndedMsg{gen: gen}
	}
}

func (a App) playPos(pos int) (App, tea.Cmd) {
	if pos < 0 || pos >= len(a.tracklist) {
		return a, nil
	}
	a.playGen++
	a.trackPos = pos
	a.queueStore.Save(a.tracklist, a.trackPos)
	track := &a.tracklist[pos]
	a.nowPlaying.track = track
	a.nowPlaying.paused = false
	a.nowPlaying.position = 0
	a.nowPlaying.duration = 0
	a.err = nil
	trackID := track.ID
	q := qualities[a.quality]
	return a, func() tea.Msg {
		url, err := a.client.GetStreamURL(trackID, q)
		return streamURLMsg{url: url, err: err}
	}
}

func (a App) addAndPlay(track *types.Track) (App, tea.Cmd) {
	insertPos := a.trackPos + 1
	if insertPos > len(a.tracklist) {
		insertPos = len(a.tracklist)
	}
	a.tracklist = append(a.tracklist[:insertPos], append([]types.Track{*track}, a.tracklist[insertPos:]...)...)
	return a.playPos(insertPos)
}

func (a App) saveQueue() {
	a.queueStore.Save(a.tracklist, a.trackPos)
}

const maxTracklist = 500

func (a App) withQueueAdd(track types.Track) App {
	if len(a.tracklist) >= maxTracklist {
		return a
	}
	a.tracklist = append(a.tracklist, track)
	a.saveQueue()
	return a
}

func (a App) withQueueAddAll(tracks []types.Track) App {
	remaining := maxTracklist - len(a.tracklist)
	if remaining <= 0 {
		return a
	}
	if len(tracks) > remaining {
		tracks = tracks[:remaining]
	}
	a.tracklist = append(a.tracklist, tracks...)
	a.saveQueue()
	return a
}

// targetTrack returns the track under focus based on current mode and tab.
func (a App) targetTrack() *types.Track {
	switch a.mode {
	case modeSearchBrowse:
		return a.search.selectedTrack()
	case modeNormal:
		switch a.activeTab {
		case tabQueue:
			if len(a.tracklist) > 0 {
				return &a.tracklist[a.queueCursor]
			}
		case tabLiked:
			if len(a.likes.Tracks) > 0 {
				return &a.likes.Tracks[a.likedCursor]
			}
		}
	}
	return nil
}

// targetTrackOrNowPlaying returns targetTrack, falling back to now playing.
func (a App) targetTrackOrNowPlaying() *types.Track {
	if t := a.targetTrack(); t != nil {
		return t
	}
	return a.nowPlaying.track
}

func (a App) stopPlayback() App {
	a.playGen++
	a.player.Stop()
	a.nowPlaying.track = nil
	a.nowPlaying.paused = false
	a.nowPlaying.position = 0
	a.nowPlaying.duration = 0
	return a
}

func (a App) adjustVolume(delta int) App {
	a.volume += delta
	if a.volume < 0 {
		a.volume = 0
	}
	if a.volume > 150 {
		a.volume = 150
	}
	a.player.SetVolume(a.volume)
	a.config.Volume = a.volume
	a.config.Save()
	return a
}

// --- Mode handlers ---

func (a App) updateHelp(msg tea.KeyMsg) (App, tea.Cmd) {
	switch msg.String() {
	case "esc", "?":
		a.mode = modeNormal
	}
	return a, nil
}

func (a App) updateSearchInput(msg tea.KeyMsg) (App, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return a, tea.Quit
	case "esc":
		a.search.input.Blur()
		a.mode = modeNormal
		return a, nil
	case "enter":
		if a.search.input.Value() == "" {
			return a, nil
		}
		a.search.loading = true
		query := a.search.input.Value()
		switch a.search.mode {
		case modeAlbum:
			return a, func() tea.Msg {
				albums, err := a.client.SearchAlbums(query)
				return albumSearchResultMsg{albums: albums, err: err}
			}
		case modeArtist:
			return a, func() tea.Msg {
				artists, err := a.client.SearchArtists(query)
				return artistSearchResultMsg{artists: artists, err: err}
			}
		default:
			return a, func() tea.Msg {
				tracks, err := a.client.SearchTracks(query)
				return searchResultMsg{tracks: tracks, err: err}
			}
		}
	}
	// Delegate to search model for text input, tab switching
	var cmd tea.Cmd
	a.search, cmd = a.search.Update(msg)
	return a, cmd
}

func (a App) updateSearchBrowse(msg tea.KeyMsg) (App, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return a, tea.Quit
	case "esc":
		a.search.input.Blur()
		a.mode = modeNormal
		return a, nil
	case "/":
		a.mode = modeSearchInput
		a.search.input.Focus()
		return a, nil
	case "1":
		a.activeTab = tabQueue
		a.mode = modeNormal
		return a, nil
	case "2":
		a.activeTab = tabLiked
		a.mode = modeNormal
		return a, nil
	case "3":
		a.activeTab = tabDownloads
		a.mode = modeNormal
		return a, nil
	case "?":
		a.mode = modeHelp
		return a, nil
	case "enter":
		if a.search.mode == modeArtist {
			if artist := a.search.selectedArtist(); artist != nil {
				a.search.loading = true
				name := artist.Name
				return a, func() tea.Msg {
					albums, err := a.client.SearchAlbums(name)
					return albumSearchResultMsg{albums: albums, err: err}
				}
			}
		}
		if a.search.mode == modeAlbum {
			if album := a.search.selectedAlbum(); album != nil {
				a.search.loading = true
				albumID := album.ID
				albumTitle := album.Title
				return a, func() tea.Msg {
					tracks, err := a.client.GetAlbumTracks(albumID)
					return albumTracksMsg{tracks: tracks, title: albumTitle, err: err}
				}
			}
		}
		if track := a.search.selectedTrack(); track != nil {
			return a.addAndPlay(track)
		}
		return a, nil
	case "a":
		if a.search.mode == modeAlbum {
			if album := a.search.selectedAlbum(); album != nil {
				albumID := album.ID
				return a, func() tea.Msg {
					tracks, err := a.client.GetAlbumTracks(albumID)
					return queueAlbumMsg{tracks: tracks, err: err}
				}
			}
		}
		if track := a.search.selectedTrack(); track != nil {
			a = a.withQueueAdd(*track)
		}
		return a, nil
	case "A":
		if tracks := a.search.browsingAlbumTracks(); len(tracks) > 0 {
			a = a.withQueueAddAll(tracks)
		}
		return a, nil
	case "u":
		if track := a.search.selectedTrack(); track != nil {
			openBrowser(fmt.Sprintf("https://monochrome.tf/album/%d", track.Album.ID))
		}
		return a, nil
	case " ":
		if a.nowPlaying.track != nil {
			a.nowPlaying.paused = !a.nowPlaying.paused
			a.player.TogglePause()
		}
		return a, nil
	case "s":
		if a.nowPlaying.track != nil {
			a = a.stopPlayback()
		}
		return a, nil
	case "n":
		if a.trackPos < len(a.tracklist)-1 {
			return a.playPos(a.trackPos + 1)
		}
		return a, nil
	case "p":
		if a.trackPos > 0 {
			return a.playPos(a.trackPos - 1)
		}
		return a, nil
	case "left":
		if a.nowPlaying.track != nil {
			a.player.Seek(-5)
		}
		return a, nil
	case "right":
		if a.nowPlaying.track != nil {
			a.player.Seek(5)
		}
		return a, nil
	case "+", "=":
		a = a.adjustVolume(5)
		return a, nil
	case "-":
		a = a.adjustVolume(-5)
		return a, nil
	case "d":
		if a.dl != nil {
			a.dl.SetQuality(qualities[a.quality])
			if target := a.targetTrackOrNowPlaying(); target != nil {
				a.dl.QueueTrack(*target)
			}
		}
		return a, nil
	case "D":
		if a.dl != nil {
			a.dl.SetQuality(qualities[a.quality])
			if tracks := a.search.browsingAlbumTracks(); len(tracks) > 0 {
				a.dl.QueueAlbum(tracks)
			}
		}
		return a, nil
	case "l":
		if target := a.targetTrackOrNowPlaying(); target != nil {
			a.likes.Toggle(*target)
		}
		return a, nil
	case "Q":
		a.quality = (a.quality + 1) % len(qualities)
		a.config.Quality = qualities[a.quality]
		a.config.Save()
		return a, nil
	}
	// Delegate j/k/up/down/backspace to search model
	var cmd tea.Cmd
	a.search, cmd = a.search.Update(msg)
	return a, cmd
}

func (a App) updateNormal(msg tea.KeyMsg) (App, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return a, tea.Quit
	case "/":
		a.mode = modeSearchInput
		a.search.input.Focus()
		return a, nil
	case "?":
		a.mode = modeHelp
		return a, nil
	case "1":
		a.activeTab = tabQueue
		return a, nil
	case "2":
		a.activeTab = tabLiked
		return a, nil
	case "3":
		a.activeTab = tabDownloads
		return a, nil
	case "up", "k":
		if a.activeTab == tabQueue && a.queueCursor > 0 {
			a.queueCursor--
		}
		if a.activeTab == tabLiked && a.likedCursor > 0 {
			a.likedCursor--
		}
		return a, nil
	case "down", "j":
		if a.activeTab == tabQueue && a.queueCursor < len(a.tracklist)-1 {
			a.queueCursor++
		}
		if a.activeTab == tabLiked && a.likedCursor < len(a.likes.Tracks)-1 {
			a.likedCursor++
		}
		return a, nil
	case "enter":
		if a.activeTab == tabQueue && len(a.tracklist) > 0 {
			return a.playPos(a.queueCursor)
		}
		if a.activeTab == tabLiked && len(a.likes.Tracks) > 0 {
			track := a.likes.Tracks[a.likedCursor]
			return a.addAndPlay(&track)
		}
		return a, nil
	case "x":
		if a.activeTab != tabQueue || len(a.tracklist) == 0 {
			return a, nil
		}
		removingPlaying := a.queueCursor == a.trackPos
		a.tracklist = append(a.tracklist[:a.queueCursor], a.tracklist[a.queueCursor+1:]...)

		if removingPlaying {
			a = a.stopPlayback()
			if a.queueCursor >= len(a.tracklist) && a.queueCursor > 0 {
				a.queueCursor--
			}
			if len(a.tracklist) > 0 {
				pos := a.queueCursor
				a.queueStore.Save(a.tracklist, pos)
				return a.playPos(pos)
			}
			a.trackPos = -1
		} else {
			if a.queueCursor < a.trackPos {
				a.trackPos--
			}
			if a.queueCursor >= len(a.tracklist) && a.queueCursor > 0 {
				a.queueCursor--
			}
		}
		a.queueStore.Save(a.tracklist, a.trackPos)
		return a, nil
	case "a":
		if a.activeTab == tabLiked && len(a.likes.Tracks) > 0 {
			a = a.withQueueAdd(a.likes.Tracks[a.likedCursor])
		}
		return a, nil
	case " ":
		if a.nowPlaying.track != nil {
			a.nowPlaying.paused = !a.nowPlaying.paused
			a.player.TogglePause()
		}
		return a, nil
	case "s":
		if a.nowPlaying.track != nil {
			a = a.stopPlayback()
		}
		return a, nil
	case "n":
		if a.trackPos < len(a.tracklist)-1 {
			return a.playPos(a.trackPos + 1)
		}
		return a, nil
	case "p":
		if a.trackPos > 0 {
			return a.playPos(a.trackPos - 1)
		}
		return a, nil
	case "left":
		if a.nowPlaying.track != nil {
			a.player.Seek(-5)
		}
		return a, nil
	case "right":
		if a.nowPlaying.track != nil {
			a.player.Seek(5)
		}
		return a, nil
	case "+", "=":
		a = a.adjustVolume(5)
		return a, nil
	case "-":
		a = a.adjustVolume(-5)
		return a, nil
	case "d":
		if a.dl != nil {
			a.dl.SetQuality(qualities[a.quality])
			if target := a.targetTrackOrNowPlaying(); target != nil {
				a.dl.QueueTrack(*target)
			}
		}
		return a, nil
	case "D":
		if a.dl != nil {
			a.dl.SetQuality(qualities[a.quality])
			if tracks := a.search.browsingAlbumTracks(); len(tracks) > 0 {
				a.dl.QueueAlbum(tracks)
			}
		}
		return a, nil
	case "l":
		if target := a.targetTrackOrNowPlaying(); target != nil {
			a.likes.Toggle(*target)
		}
		return a, nil
	case "Q":
		a.quality = (a.quality + 1) % len(qualities)
		a.config.Quality = qualities[a.quality]
		a.config.Save()
		return a, nil
	}
	return a, nil
}

// --- Main Update dispatcher ---

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case tickMsg:
		if a.nowPlaying.track != nil && !a.nowPlaying.paused {
			pos, dur, err := a.player.GetPosition()
			if err == nil {
				a.nowPlaying.position = pos
				a.nowPlaying.duration = dur
			}
		}
		return a, tick()

	case tea.KeyMsg:
		switch a.mode {
		case modeHelp:
			a, cmd := a.updateHelp(msg)
			return a, cmd
		case modeSearchInput:
			a, cmd := a.updateSearchInput(msg)
			return a, cmd
		case modeSearchBrowse:
			a, cmd := a.updateSearchBrowse(msg)
			return a, cmd
		default:
			a, cmd := a.updateNormal(msg)
			return a, cmd
		}

	case streamURLMsg:
		if msg.err != nil {
			a.err = msg.err
			return a, nil
		}
		if err := a.player.Play(msg.url); err != nil {
			a.err = err
			return a, nil
		}
		return a, a.makeWaitForTrackEnd(a.playGen)

	case trackEndedMsg:
		if msg.gen != a.playGen {
			return a, nil
		}
		if a.trackPos < len(a.tracklist)-1 {
			return a.playPos(a.trackPos + 1)
		}
		a.nowPlaying.track = nil
		a.nowPlaying.paused = false
		return a, nil

	case queueAlbumMsg:
		if msg.err != nil {
			a.err = msg.err
			return a, nil
		}
		a = a.withQueueAddAll(msg.tracks)
		return a, nil

	case DownloadUpdateMsg:
		return a, nil

	case errMsg:
		a.err = msg.err
		return a, nil
	}

	// Pass non-key messages to search model (handles search result messages)
	if a.mode == modeSearchInput || a.mode == modeSearchBrowse {
		var cmd tea.Cmd
		a.search, cmd = a.search.Update(msg)
		// Detect search results arriving: input blurs → transition to browse
		if a.mode == modeSearchInput && !a.search.input.Focused() {
			a.mode = modeSearchBrowse
		}
		return a, cmd
	}
	return a, nil
}

// --- View helpers ---

func (a App) searchVisible() bool {
	return a.mode == modeSearchInput || a.mode == modeSearchBrowse
}

func (a App) dlCheck() func(types.Track) bool {
	if a.dl == nil {
		return func(types.Track) bool { return false }
	}
	return a.dl.IsDownloaded
}

func (a App) renderTabBar() string {
	tabs := []struct {
		label string
		tab   viewTab
	}{
		{"1:Queue", tabQueue},
		{"2:Liked", tabLiked},
		{"3:Downloads", tabDownloads},
	}

	var parts []string
	for _, t := range tabs {
		label := t.label
		if t.tab == tabQueue && len(a.tracklist) > 0 {
			label = fmt.Sprintf("1:Queue(%d)", len(a.tracklist))
		}
		if t.tab == tabLiked && len(a.likes.Tracks) > 0 {
			label = fmt.Sprintf("2:Liked(%d)", len(a.likes.Tracks))
		}
		if t.tab == a.activeTab {
			parts = append(parts, selectedStyle.Render(" "+label+" "))
		} else {
			parts = append(parts, dimStyle.Render(" "+label+" "))
		}
	}
	return "  " + strings.Join(parts, dimStyle.Render("│"))
}

func (a App) renderQueueView() string {
	if len(a.tracklist) == 0 {
		return dimStyle.Render("  Queue is empty. Press 'a' on a track to add it.")
	}

	tc := computeTrackCols(a.width)
	s := trackHeader(tc) + "\n"
	for i, t := range a.tracklist {
		isPlaying := i == a.trackPos
		isCursor := i == a.queueCursor
		played := a.trackPos >= 0 && i < a.trackPos

		duration := fmt.Sprintf("%d:%02d", t.Duration/60, t.Duration%60)
		num := fmt.Sprintf("%d", i+1)
		icons := statusIcons(a.likes.IsLiked(t.ID), a.dl != nil && a.dl.IsDownloaded(t))

		var numSt, artSt, albSt, titSt, durSt lipgloss.Style
		var marker string

		switch {
		case isCursor && isPlaying:
			marker = titleStyle.Render("▸")
			numSt, artSt, albSt, titSt, durSt = playingSelectedStyle, playingSelectedStyle, playingSelectedStyle, playingSelectedStyle, playingSelectedStyle
		case isPlaying:
			marker = playingStyle.Render("♫")
			numSt, artSt, albSt, titSt, durSt = playingStyle, playingStyle, playingStyle, playingStyle, playingStyle
		case isCursor:
			marker = titleStyle.Render("▸")
			numSt, artSt, albSt, titSt, durSt = selectedStyle, selectedStyle, selectedStyle, selectedStyle, selectedStyle
		case played:
			marker = " "
			numSt, artSt, albSt, titSt, durSt = dimStyle, dimStyle, dimStyle, dimStyle, dimStyle
		default:
			marker = " "
			numSt, artSt, albSt, titSt, durSt = dimStyle, artistStyle, dimStyle, normalStyle, dimStyle
		}

		year := trackYear(t)

		s += marker + icons +
			col(num, colNum, numSt) +
			col(t.Artist.Name, tc.artist, artSt) +
			col(t.Title, tc.title, titSt) +
			col(t.Album.Title, tc.album, albSt) +
			col(year, colYear, durSt) +
			col(duration, colDuration, durSt) + "\n"
	}
	s += "\n" + dimStyle.Render("  enter play  x remove  / search")
	return s
}

func (a App) renderLikedView() string {
	if len(a.likes.Tracks) == 0 {
		return dimStyle.Render("  No liked tracks yet. Press 'l' on a track to like it.")
	}

	tc := computeTrackCols(a.width)
	s := trackHeader(tc) + "\n"
	for i, t := range a.likes.Tracks {
		s += trackRow(i, t, i == a.likedCursor, true, a.dl != nil && a.dl.IsDownloaded(t), tc) + "\n"
	}
	s += "\n" + dimStyle.Render("  enter play  a queue  l unlike")
	return s
}

func (a App) renderDownloadsView() string {
	if a.dl == nil {
		return dimStyle.Render("  Downloads not available.")
	}

	st := a.dl.Status()
	if st.Active == 0 && st.Completed == 0 && st.Failed == 0 && st.Queued == 0 {
		return dimStyle.Render("  No downloads yet. Press 'd' on a track or 'D' on an album.")
	}

	s := ""
	if st.Active > 0 {
		s += titleStyle.Render("  Downloading") + "\n"
		if st.Current != "" {
			s += "  " + normalStyle.Render(st.Current) + "\n"
		}
		s += "\n"
	}
	if st.Queued > 0 {
		s += dimStyle.Render(fmt.Sprintf("  Waiting: %d", st.Queued)) + "\n"
	}
	if st.Completed > 0 {
		s += dimStyle.Render(fmt.Sprintf("  Completed: %d", st.Completed)) + "\n"
	}
	if st.Failed > 0 {
		failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
		s += failStyle.Render(fmt.Sprintf("  Failed: %d", st.Failed)) + "\n"
		if st.LastError != "" {
			s += failStyle.Render(fmt.Sprintf("  Last error: %s", st.LastError)) + "\n"
		}
	}

	return s
}

func (a App) View() string {
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF6AC1")).
		Render("♫ riff")

	a.nowPlaying.quality = qualities[a.quality]
	a.nowPlaying.volume = a.volume
	if a.nowPlaying.track != nil {
		a.nowPlaying.liked = a.likes.IsLiked(a.nowPlaying.track.ID)
	}
	np := a.nowPlaying.View(a.width)
	tabBar := a.renderTabBar()

	errView := ""
	if a.err != nil {
		errView = "\n" + lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Render(fmt.Sprintf("  Error: %s", a.err))
	}

	help := dimStyle.Render("  ? help  / search  enter play  a queue  p prev  n next  space pause  s stop  q quit")

	var top, bottom string

	switch {
	case a.mode == modeHelp:
		helpOverlay := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF6AC1")).
			Padding(1, 2).
			Render(
				titleStyle.Render("Keybindings") + "\n\n" +
					helpLine("1-3", "Switch tabs") +
					helpLine("/", "Focus search") +
					helpLine("tab", "Toggle track/album/artist search") +
					helpLine("enter", "Play track / browse album") +
					helpLine("esc", "Blur search / close help") +
					helpLine("backspace", "Back from album tracklist") +
					"\n" +
					helpLine("space", "Toggle pause") +
					helpLine("s", "Stop playback") +
					helpLine("n", "Next track in queue") +
					helpLine("p", "Previous track") +
					helpLine("a", "Queue track / queue album") +
					helpLine("A", "Queue all album tracks") +
					helpLine("x", "Remove from queue") +
					helpLine("left/right", "Seek -5s / +5s") +
					helpLine("+/-", "Volume up/down") +
					"\n" +
					helpLine("j/k", "Navigate up/down") +
					helpLine("d", "Download track") +
					helpLine("D", "Download album") +
					helpLine("l", "Toggle like") +
					helpLine("u", "Open album in browser") +
					helpLine("Q", "Cycle quality") +
					helpLine("?", "Toggle this help") +
					helpLine("q", "Quit"),
			)
		top = fmt.Sprintf("\n  %s\n%s\n\n%s\n%s", header, tabBar, helpOverlay, dimStyle.Render("  esc to close"))

	case a.searchVisible():
		searchOverlay := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF6AC1")).
			Padding(1, 2).
			Width(a.width - 6).
			Render(a.search.View(a.width-12, a.likes.IsLiked, a.dlCheck()))
		top = fmt.Sprintf("\n  %s\n%s\n\n%s\n%s%s", header, tabBar, searchOverlay, dimStyle.Render("  esc to close"), errView)

	default:
		var content string
		switch a.activeTab {
		case tabQueue:
			content = a.renderQueueView()
		case tabLiked:
			content = a.renderLikedView()
		case tabDownloads:
			content = a.renderDownloadsView()
		}
		top = fmt.Sprintf("\n  %s\n%s\n\n%s%s", header, tabBar, content, errView)
	}

	dlStatus := ""
	if a.dl != nil {
		st := a.dl.Status()
		if st.Active > 0 || st.Queued > 0 {
			dlStatus = dimStyle.Render(fmt.Sprintf("  DL: %d active, %d queued, %d done", st.Active, st.Queued, st.Completed))
			if st.Current != "" {
				dlStatus += dimStyle.Render("  " + st.Current)
			}
			dlStatus += "\n"
		} else if st.Completed > 0 || st.Failed > 0 {
			parts := []string{}
			if st.Completed > 0 {
				parts = append(parts, fmt.Sprintf("%d done", st.Completed))
			}
			if st.Failed > 0 {
				parts = append(parts, fmt.Sprintf("%d failed", st.Failed))
			}
			dlStatus = dimStyle.Render("  DL: "+strings.Join(parts, ", ")) + "\n"
		}
	}

	bottom = dlStatus + np + "\n" + help

	topHeight := lipgloss.Height(top)
	bottomHeight := lipgloss.Height(bottom)
	gap := a.height - topHeight - bottomHeight
	if gap < 1 {
		gap = 1
	}

	return top + strings.Repeat("\n", gap) + bottom
}

func helpLine(key, desc string) string {
	return fmt.Sprintf("  %s  %s\n",
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6AC1")).
			Bold(true).
			Width(12).
			Render(key),
		desc,
	)
}

func openBrowser(url string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", url).Start()
	case "linux":
		exec.Command("xdg-open", url).Start()
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
}
