package ui

import (
	"fmt"
	"math/rand"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
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

type pingMsg struct{ online bool }

type trackEndedMsg struct{ gen int }

type queueAlbumMsg struct {
	tracks []types.Track
	err    error
}

type albumArtMsg struct {
	coverID string
	art     string
}

var qualities = []string{"LOW", "HIGH", "LOSSLESS", "HI_RES"}

var spinnerFrames = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█", "▇", "▆", "▅", "▄", "▃", "▂"}


type tickMsg struct{}

type viewTab int

const (
	tabQueue viewTab = iota
	tabLiked
	tabDownloads
	tabPlaylists
)

// inputMode represents the current UI state. Every key event is routed
// through exactly one mode handler, eliminating boolean-soup guards.
type inputMode int

const (
	modeNormal         inputMode = iota // browsing tabs
	modeSearchInput                     // typing in search box
	modeSearchBrowse                    // navigating search results
	modeHelp                            // help overlay
	modeFilter                          // inline filter on queue/liked
	modeCommand                         // vim-style : command line
	modeSavePlaylist                    // save playlist popup
	modeRenamePlaylist                  // rename playlist popup
	modeAddToPlaylist                   // pick playlist to add track to
)

type App struct {
	mode             inputMode
	activeTab        viewTab
	search           searchModel
	nowPlaying       nowPlayingModel
	tracklist        []types.Track
	trackPos         int // index of currently playing track, -1 if none
	likedCursor      int
	queueCursor      int // cursor for browsing the queue view
	queueScrollOffset int
	likedScrollOffset int
	undoTrack        *types.Track
	undoPos          int
	undoTrackPos     int
	loading          bool
	streamRetries    int
	selected         map[int]bool
	online           bool
	tickCount        int
	spinnerIdx       int
	client           *api.Client
	player           *player.Player
	likes            *persistence.LikedStore
	dl               *downloader.Downloader
	config           *persistence.Config
	queueStore       *persistence.QueueStore
	playCounts       *persistence.PlayCountStore
	playlists        *persistence.PlaylistStore
	playlistNames    []string
	playlistCursor   int
	playGen          int
	quality          int
	volume           int
	width            int
	height           int
	err              error
	statusMsg        string
	statusTicks      int // ticks remaining before status clears
	pendingG         bool
	cmdInput         textinput.Model
	filterInput      textinput.Model
	filterText       string
	filteredIndices  []int
	showRemaining    bool
	showLineNumbers  bool
	activePlaylist   string
	playlistDirty    bool
	showPlayCounts   bool
	showAlbumArt     bool
	audioInfo        string
	saveInput        textinput.Model
	saveTracks       []types.Track
	renameInput      textinput.Model
	renameFrom       string
	addToTrack       *types.Track
	addToPickerNames []string
	addToPickerIdx   int
}

func NewApp(client *api.Client, player *player.Player, likes *persistence.LikedStore, dl *downloader.Downloader, cfg *persistence.Config, qs *persistence.QueueStore, pc *persistence.PlayCountStore, ps *persistence.PlaylistStore) App {
	mode := modeNormal
	if len(qs.Tracks) == 0 {
		mode = modeSearchInput
	}
	queueCursor := qs.QueueCursor
	if queueCursor >= len(qs.Tracks) && queueCursor > 0 {
		queueCursor = len(qs.Tracks) - 1
	}
	if queueCursor < 0 {
		queueCursor = 0
	}
	likedCursor := qs.LikedCursor
	if likedCursor >= len(likes.Tracks) && likedCursor > 0 {
		likedCursor = len(likes.Tracks) - 1
	}
	if likedCursor < 0 {
		likedCursor = 0
	}
	sm := newSearchModel()
	if mode == modeSearchInput {
		sm.input.Focus()
	}
	return App{
		mode:        mode,
		activeTab:   viewTab(qs.ActiveTab),
		search:      sm,
		nowPlaying:  newNowPlayingModel(),
		client:      client,
		player:      player,
		likes:       likes,
		dl:          dl,
		config:      cfg,
		queueStore:  qs,
		playCounts:  pc,
		playlists:   ps,
		tracklist:   qs.Tracks,
		trackPos:    qs.Position,
		quality:         cfg.QualityIndex(),
		volume:          cfg.Volume,
		queueCursor:     queueCursor,
		likedCursor:     likedCursor,
		selected:        make(map[int]bool),
		cmdInput:        newCmdInput(),
		online:          true,
		showLineNumbers: cfg.ShowLineNumbers,
		showPlayCounts:  cfg.ShowPlayCounts,
		showRemaining:   cfg.ShowRemaining,
		showAlbumArt:    cfg.ShowAlbumArt,
		filterInput: newFilterInput(),
		saveInput:   newSaveInput(),
		renameInput: newRenameInput(),
	}
}


func newFilterInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "filter..."
	ti.Prompt = "Filter: "
	ti.CharLimit = 50
	ti.Width = 30
	return ti
}

func newSaveInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "playlist name"
	ti.Prompt = "Save as: "
	ti.CharLimit = 30
	ti.Width = 25
	return ti
}

func newRenameInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "new name"
	ti.Prompt = "Rename to: "
	ti.CharLimit = 30
	ti.Width = 25
	return ti
}


func newCmdInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.Prompt = ":"
	ti.CharLimit = 50
	ti.Width = 40
	return ti
}

func tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (a App) doPing() tea.Cmd {
	return func() tea.Msg {
		return pingMsg{online: a.client.Ping()}
	}
}

func (a App) Init() tea.Cmd {
	if a.mode == modeSearchInput {
		return tea.Batch(a.search.input.Focus(), tick(), a.doPing())
	}
	return tea.Batch(tick(), a.doPing())
}

func (a App) makeWaitForTrackEnd(gen int) tea.Cmd {
	return func() tea.Msg {
		a.player.WaitForEnd()
		return trackEndedMsg{gen: gen}
	}
}

func (a App) fetchArt(coverID string) tea.Cmd {
	return func() tea.Msg {
		art := fetchAlbumArt(coverID, 8, 4)
		return albumArtMsg{coverID: coverID, art: art}
	}
}

func (a App) playPos(pos int) (App, tea.Cmd) {
	if pos < 0 || pos >= len(a.tracklist) {
		return a, nil
	}
	a.playGen++
	a.trackPos = pos
	a.audioInfo = ""
	a.queueStore.Save(a.tracklist, a.trackPos)
	track := &a.tracklist[pos]
	a.nowPlaying.track = track
	a.nowPlaying.paused = false
	a.nowPlaying.position = 0
	a.nowPlaying.duration = 0
	a.nowPlaying.albumArt = ""
	a.nowPlaying.coverID = ""
	a.err = nil
	a.loading = true
	a.streamRetries = 0
	trackID := track.ID
	q := qualities[a.quality]
	return a, func() tea.Msg {
		url, err := a.client.GetStreamURL(trackID, q)
		return streamURLMsg{url: url, err: err}
	}
}

func (a App) addAndPlay(track *types.Track) (App, tea.Cmd) {
	if a.activePlaylist != "" {
		// Playing from search replaces the playlist queue
		a.activePlaylist = ""
		a.playlistDirty = false
		a.tracklist = []types.Track{*track}
		return a.playPos(0)
	}
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

func (a App) saveUIState() {
	a.queueStore.SaveUIState(int(a.activeTab), a.queueCursor, a.likedCursor)
}

const maxTracklist = 500

func (a App) markDirty() App {
	if a.activePlaylist != "" {
		a.playlistDirty = true
	}
	return a
}

// switchTab changes the active tab. Auto-saves dirty playlists.
func (a App) switchTab(tab viewTab) (App, bool) {
	if a.playlistDirty && a.activePlaylist != "" && a.playlists != nil {
		a.playlists.Save(a.activePlaylist, a.tracklist)
		a.playlistDirty = false
		a = a.withStatus(fmt.Sprintf("Auto-saved: %s", a.activePlaylist))
	}
	a.activeTab = tab
	if tab == tabPlaylists {
		a = a.refreshPlaylists()
	}
	return a, true
}

func (a App) withQueueAdd(track types.Track) App {
	if len(a.tracklist) >= maxTracklist {
		return a.withStatus("Queue full (500 max)")
	}
	a = a.markDirty()
	a.tracklist = append(a.tracklist, track)
	a.saveQueue()
	return a.withStatus(fmt.Sprintf("Queued: %s", track.Title))
}

func (a App) withQueueAddAll(tracks []types.Track) App {
	remaining := maxTracklist - len(a.tracklist)
	if remaining <= 0 {
		return a.withStatus("Queue full (500 max)")
	}
	if len(tracks) > remaining {
		tracks = tracks[:remaining]
	}
	a = a.markDirty()
	a.tracklist = append(a.tracklist, tracks...)
	a.saveQueue()
	return a.withStatus(fmt.Sprintf("Queued %d tracks", len(tracks)))
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
	a.nowPlaying.albumArt = ""
	a.nowPlaying.coverID = ""
	a.audioInfo = ""
	return a
}

func (a App) withStatus(msg string) App {
	a.statusMsg = msg
	a.statusTicks = 3
	return a
}

func (a App) refreshPlaylists() App {
	if a.playlists != nil {
		a.playlistNames = a.playlists.List()
	}
	return a
}

func (a App) syncNowPlaying() App {
	a.nowPlaying.quality = qualities[a.quality]
	a.nowPlaying.volume = a.volume
	a.nowPlaying.showRemaining = a.showRemaining
	a.nowPlaying.audioInfo = a.audioInfo
	a.nowPlaying.showAlbumArt = a.showAlbumArt
	if a.nowPlaying.track != nil {
		a.nowPlaying.liked = a.likes.IsLiked(a.nowPlaying.track.ID)
		a.nowPlaying.coverID = a.nowPlaying.track.Album.Cover
	}
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
		if !a.online {
			a = a.withStatus("Search unavailable — offline")
			return a, nil
		}
		query := a.search.input.Value()
		a.search.addToHistory(query)
		a.search.loading = true
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
		a.search.resetHistoryIndex()
		return a, nil
	case "1":
		a, ok := a.switchTab(tabQueue)
		if ok {
			a.mode = modeNormal
		}
		return a, nil
	case "2":
		a, ok := a.switchTab(tabLiked)
		if ok {
			a.mode = modeNormal
		}
		return a, nil
	case "3":
		a, ok := a.switchTab(tabDownloads)
		if ok {
			a.mode = modeNormal
		}
		return a, nil
	case "4":
		a, ok := a.switchTab(tabPlaylists)
		if ok {
			a.mode = modeNormal
		}
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
	case "P":
		if a.playlists == nil {
			return a, nil
		}
		target := a.search.selectedTrack()
		if target == nil {
			return a, nil
		}
		names := a.playlists.List()
		if len(names) == 0 {
			a = a.withStatus("No playlists. Use :save to create one first")
			return a, nil
		}
		a.addToTrack = target
		a.addToPickerNames = names
		a.addToPickerIdx = 0
		a.mode = modeAddToPlaylist
		return a, nil
	case "S":
		if a.playlists != nil && a.search.mode == modeBrowseAlbum {
			if tracks := a.search.browsingAlbumTracks(); len(tracks) > 0 {
				a.saveTracks = tracks
				// Pre-fill with sanitized album title
				title := strings.ToLower(a.search.albumTitle)
				title = strings.ReplaceAll(title, " ", "-")
				var sanitized strings.Builder
				for _, ch := range title {
					if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
						sanitized.WriteRune(ch)
					}
				}
				a.saveInput.Reset()
				a.saveInput.SetValue(sanitized.String())
				a.saveInput.Focus()
				a.mode = modeSavePlaylist
				return a, nil
			}
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
				if a.dl.IsDownloaded(*target) {
					a = a.withStatus(fmt.Sprintf("Already downloaded: %s", target.Title))
				} else if a.dl.QueueTrack(*target) {
					a = a.withStatus(fmt.Sprintf("Downloading: %s", target.Title))
				} else {
					a = a.withStatus(fmt.Sprintf("Already queued: %s", target.Title))
				}
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
			if a.likes.Toggle(*target) {
				a = a.withStatus(fmt.Sprintf("Liked: %s", target.Title))
			} else {
				a = a.withStatus(fmt.Sprintf("Unliked: %s", target.Title))
			}
		}
		return a, nil
	case "Q":
		a.quality = (a.quality + 1) % len(qualities)
		a.config.Quality = qualities[a.quality]
		a.config.Save()
		a = a.withStatus(fmt.Sprintf("Quality: %s", qualities[a.quality]))
		return a, nil
	case "G":
		if n := a.search.listLen(); n > 0 {
			a.search.cursor = n - 1
		}
		return a, nil
	case "home", "ctrl+a":
		a.search.cursor = 0
		return a, nil
	case "t":
		a.showRemaining = !a.showRemaining
		a.config.ShowRemaining = a.showRemaining
		a.config.Save()
		return a, nil
	}
	// Delegate j/k/up/down/backspace to search model
	var cmd tea.Cmd
	a.search, cmd = a.search.Update(msg)
	return a, cmd
}

func (a App) updateNormal(msg tea.KeyMsg) (App, tea.Cmd) {
	// Clear pending g on any non-g key
	if msg.String() != "g" && a.pendingG {
		a.pendingG = false
	}
	switch msg.String() {
	case "ctrl+c", "q":
		return a, tea.Quit
	case "/":
		a.mode = modeSearchInput
		a.search.input.Focus()
		a.search.resetHistoryIndex()
		return a, nil
	case "?":
		a.mode = modeHelp
		return a, nil
	case "1":
		a, ok := a.switchTab(tabQueue)
		if ok {
			a.saveUIState()
		}
		return a, nil
	case "2":
		a, ok := a.switchTab(tabLiked)
		if ok {
			a.saveUIState()
		}
		return a, nil
	case "3":
		a, ok := a.switchTab(tabDownloads)
		if ok {
			a.saveUIState()
		}
		return a, nil
	case "4":
		a, ok := a.switchTab(tabPlaylists)
		if ok {
			a.saveUIState()
		}
		return a, nil
	case "up", "k":
		if a.activeTab == tabQueue && a.queueCursor > 0 {
			a.queueCursor--
			visibleRows := a.height - 12
			if visibleRows < 1 {
				visibleRows = 1
			}
			if a.queueCursor < a.queueScrollOffset {
				a.queueScrollOffset = a.queueCursor
			}
		}
		if a.activeTab == tabLiked && a.likedCursor > 0 {
			a.likedCursor--
			visibleRows := a.height - 12
			if visibleRows < 1 {
				visibleRows = 1
			}
			if a.likedCursor < a.likedScrollOffset {
				a.likedScrollOffset = a.likedCursor
			}
		}
		if a.activeTab == tabPlaylists && a.playlistCursor > 0 {
			a.playlistCursor--
		}
		return a, nil
	case "down", "j":
		if a.activeTab == tabQueue && a.queueCursor < len(a.tracklist)-1 {
			a.queueCursor++
			visibleRows := a.height - 12
			if visibleRows < 1 {
				visibleRows = 1
			}
			if a.queueCursor >= a.queueScrollOffset+visibleRows {
				a.queueScrollOffset = a.queueCursor - visibleRows + 1
			}
		}
		if a.activeTab == tabLiked && a.likedCursor < len(a.likes.Tracks)-1 {
			a.likedCursor++
			visibleRows := a.height - 12
			if visibleRows < 1 {
				visibleRows = 1
			}
			if a.likedCursor >= a.likedScrollOffset+visibleRows {
				a.likedScrollOffset = a.likedCursor - visibleRows + 1
			}
		}
		if a.activeTab == tabPlaylists && a.playlistCursor < len(a.playlistNames)-1 {
			a.playlistCursor++
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
		if a.activeTab == tabPlaylists && len(a.playlistNames) > 0 {
			name := a.playlistNames[a.playlistCursor]
			tracks, err := a.playlists.Load(name)
			if err == nil && len(tracks) > 0 {
				a.tracklist = tracks
				a.trackPos = -1
				a.queueCursor = 0
				a.queueScrollOffset = 0
				a.activePlaylist = name
				a.playlistDirty = false
				a.saveQueue()
				a = a.withStatus(fmt.Sprintf("Loaded: %s (%d tracks)", name, len(tracks)))
				a.activeTab = tabQueue
			}
			return a, nil
		}
		return a, nil
	case "x":
		if a.activeTab == tabPlaylists && len(a.playlistNames) > 0 {
			name := a.playlistNames[a.playlistCursor]
			a.playlists.Delete(name)
			a = a.refreshPlaylists()
			if a.playlistCursor >= len(a.playlistNames) && a.playlistCursor > 0 {
				a.playlistCursor--
			}
			a = a.withStatus(fmt.Sprintf("Deleted: %s", name))
			return a, nil
		}
		if a.activeTab != tabQueue || len(a.tracklist) == 0 {
			return a, nil
		}
		if len(a.selected) > 0 {
			// Batch remove selected tracks (sort indices descending to avoid index shifting)
			indices := make([]int, 0, len(a.selected))
			for idx := range a.selected {
				if idx >= 0 && idx < len(a.tracklist) {
					indices = append(indices, idx)
				}
			}
			// Sort descending
			for i := 0; i < len(indices)-1; i++ {
				for j := i + 1; j < len(indices); j++ {
					if indices[j] > indices[i] {
						indices[i], indices[j] = indices[j], indices[i]
					}
				}
			}
			removedPlaying := false
			for _, idx := range indices {
				if idx == a.trackPos {
					removedPlaying = true
				}
				a.tracklist = append(a.tracklist[:idx], a.tracklist[idx+1:]...)
				// Adjust trackPos and cursor for removed index
				if idx < a.trackPos {
					a.trackPos--
				}
				if idx <= a.queueCursor && a.queueCursor > 0 {
					a.queueCursor--
				}
			}
			a.selected = make(map[int]bool)
			if a.queueCursor >= len(a.tracklist) && a.queueCursor > 0 {
				a.queueCursor = len(a.tracklist) - 1
			}
			if removedPlaying {
				a = a.stopPlayback()
				a.trackPos = -1
			}
			a.queueStore.Save(a.tracklist, a.trackPos)
			a = a.withStatus(fmt.Sprintf("Removed %d tracks", len(indices)))
			return a, nil
		}
		// Save undo state before single removal
		trackCopy := a.tracklist[a.queueCursor]
		a.undoTrack = &trackCopy
		a.undoPos = a.queueCursor
		a.undoTrackPos = a.trackPos

		a = a.markDirty()
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
	case "ctrl+z":
		if a.undoTrack == nil {
			return a, nil
		}
		// Re-insert the track at its original position
		pos := a.undoPos
		if pos > len(a.tracklist) {
			pos = len(a.tracklist)
		}
		restored := make([]types.Track, len(a.tracklist)+1)
		copy(restored, a.tracklist[:pos])
		restored[pos] = *a.undoTrack
		copy(restored[pos+1:], a.tracklist[pos:])
		a.tracklist = restored
		a.trackPos = a.undoTrackPos
		a.queueCursor = pos
		title := a.undoTrack.Title
		a.undoTrack = nil
		a.queueStore.Save(a.tracklist, a.trackPos)
		a = a.withStatus(fmt.Sprintf("Restored: %s", title))
		return a, nil
	case "a":
		if a.activeTab == tabLiked && len(a.likes.Tracks) > 0 {
			if len(a.selected) > 0 {
				var toQueue []types.Track
				for _, t := range a.likes.Tracks {
					if a.selected[t.ID] {
						toQueue = append(toQueue, t)
					}
				}
				a.selected = make(map[int]bool)
				a = a.withQueueAddAll(toQueue)
			} else {
				a = a.withQueueAdd(a.likes.Tracks[a.likedCursor])
			}
		}
		if a.activeTab == tabPlaylists && len(a.playlistNames) > 0 {
			name := a.playlistNames[a.playlistCursor]
			tracks, err := a.playlists.Load(name)
			if err == nil && len(tracks) > 0 {
				a = a.withQueueAddAll(tracks)
				a = a.withStatus(fmt.Sprintf("Appended: %s (%d tracks)", name, len(tracks)))
			}
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
	case "P":
		if a.playlists == nil {
			return a, nil
		}
		target := a.targetTrackOrNowPlaying()
		if target == nil {
			return a, nil
		}
		names := a.playlists.List()
		if len(names) == 0 {
			a = a.withStatus("No playlists. Use :save to create one first")
			return a, nil
		}
		a.addToTrack = target
		a.addToPickerNames = names
		a.addToPickerIdx = 0
		a.mode = modeAddToPlaylist
		return a, nil
	case "S":
		if a.playlists == nil {
			return a, nil
		}
		if len(a.selected) > 0 {
			var tracks []types.Track
			for i, t := range a.tracklist {
				if a.selected[i] {
					tracks = append(tracks, t)
				}
			}
			a.saveTracks = tracks
		} else if a.activeTab == tabQueue && len(a.tracklist) > 0 {
			// Quick-save: if playing a playlist, auto-save without popup
			if a.activePlaylist != "" {
				a.playlists.Save(a.activePlaylist, a.tracklist)
				a.playlistDirty = false
				a = a.refreshPlaylists()
				a = a.withStatus(fmt.Sprintf("Saved: %s (%d tracks)", a.activePlaylist, len(a.tracklist)))
				return a, nil
			}
			a.saveTracks = a.tracklist
		} else {
			return a, nil
		}
		a.saveInput.Reset()
		a.saveInput.Focus()
		a.mode = modeSavePlaylist
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
			if len(a.selected) > 0 && a.activeTab == tabQueue {
				count := 0
				for idx := range a.selected {
					if idx >= 0 && idx < len(a.tracklist) {
						if a.dl.QueueTrack(a.tracklist[idx]) {
							count++
						}
					}
				}
				a.selected = make(map[int]bool)
				a = a.withStatus(fmt.Sprintf("Downloading %d tracks", count))
			} else if target := a.targetTrackOrNowPlaying(); target != nil {
				if a.dl.IsDownloaded(*target) {
					a = a.withStatus(fmt.Sprintf("Already downloaded: %s", target.Title))
				} else if a.dl.QueueTrack(*target) {
					a = a.withStatus(fmt.Sprintf("Downloading: %s", target.Title))
				} else {
					a = a.withStatus(fmt.Sprintf("Already queued: %s", target.Title))
				}
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
			if a.likes.Toggle(*target) {
				a = a.withStatus(fmt.Sprintf("Liked: %s", target.Title))
			} else {
				a = a.withStatus(fmt.Sprintf("Unliked: %s", target.Title))
			}
		}
		return a, nil
	case "v":
		if a.selected == nil {
			a.selected = make(map[int]bool)
		}
		if a.activeTab == tabQueue && len(a.tracklist) > 0 {
			if a.selected[a.queueCursor] {
				delete(a.selected, a.queueCursor)
			} else {
				a.selected[a.queueCursor] = true
			}
		} else if a.activeTab == tabLiked && len(a.likes.Tracks) > 0 {
			id := a.likes.Tracks[a.likedCursor].ID
			if a.selected[id] {
				delete(a.selected, id)
			} else {
				a.selected[id] = true
			}
		}
		return a, nil
	case "V":
		if a.selected == nil {
			a.selected = make(map[int]bool)
		}
		if a.activeTab == tabQueue {
			// Toggle: if all visible are selected, deselect all; otherwise select all
			allSelected := len(a.tracklist) > 0
			for i := range a.tracklist {
				if !a.selected[i] {
					allSelected = false
					break
				}
			}
			if allSelected {
				a.selected = make(map[int]bool)
			} else {
				for i := range a.tracklist {
					a.selected[i] = true
				}
			}
		} else if a.activeTab == tabLiked {
			allSelected := len(a.likes.Tracks) > 0
			for _, t := range a.likes.Tracks {
				if !a.selected[t.ID] {
					allSelected = false
					break
				}
			}
			if allSelected {
				a.selected = make(map[int]bool)
			} else {
				for _, t := range a.likes.Tracks {
					a.selected[t.ID] = true
				}
			}
		}
		return a, nil
	case "r":
		if a.activeTab == tabPlaylists && len(a.playlistNames) > 0 {
			name := a.playlistNames[a.playlistCursor]
			a.renameFrom = name
			a.renameInput.Reset()
			a.renameInput.SetValue(name)
			a.renameInput.Focus()
			a.mode = modeRenamePlaylist
			return a, nil
		}
		if a.activeTab == tabDownloads && a.dl != nil {
			n := a.dl.RetryFailed()
			if n > 0 {
				a = a.withStatus(fmt.Sprintf("Retrying %d downloads", n))
			} else {
				a = a.withStatus("No failed downloads to retry")
			}
		}
		return a, nil
	case "u":
		if target := a.targetTrackOrNowPlaying(); target != nil {
			openBrowser(fmt.Sprintf("https://monochrome.tf/album/%d", target.Album.ID))
		}
		return a, nil
	case "Q":
		a.quality = (a.quality + 1) % len(qualities)
		a.config.Quality = qualities[a.quality]
		a.config.Save()
		a = a.withStatus(fmt.Sprintf("Quality: %s", qualities[a.quality]))
		return a, nil
	case "f":
		if a.activeTab == tabQueue || a.activeTab == tabLiked {
			a.mode = modeFilter
			a.filterInput.Reset()
			a.filterText = ""
			a.filteredIndices = nil
			a.filterInput.Focus()
			return a, nil
		}
		return a, nil
	case "g":
		if a.pendingG {
			// gg: jump to first line
			a.pendingG = false
			if a.activeTab == tabQueue {
				a.queueCursor = 0
				a.queueScrollOffset = 0
			}
			if a.activeTab == tabLiked {
				a.likedCursor = 0
				a.likedScrollOffset = 0
			}
			return a, nil
		}
		a.pendingG = true
		return a, nil
	case "ctrl+d":
		visibleRows := a.height - 12
		if visibleRows < 1 {
			visibleRows = 1
		}
		halfPage := visibleRows / 2
		if halfPage < 1 {
			halfPage = 1
		}
		if a.activeTab == tabQueue && len(a.tracklist) > 0 {
			a.queueCursor += halfPage
			if a.queueCursor >= len(a.tracklist) {
				a.queueCursor = len(a.tracklist) - 1
			}
			if a.queueCursor >= a.queueScrollOffset+visibleRows {
				a.queueScrollOffset = a.queueCursor - visibleRows + 1
			}
		}
		if a.activeTab == tabLiked && len(a.likes.Tracks) > 0 {
			a.likedCursor += halfPage
			if a.likedCursor >= len(a.likes.Tracks) {
				a.likedCursor = len(a.likes.Tracks) - 1
			}
			if a.likedCursor >= a.likedScrollOffset+visibleRows {
				a.likedScrollOffset = a.likedCursor - visibleRows + 1
			}
		}
		return a, nil
	case "ctrl+u":
		visibleRows := a.height - 12
		if visibleRows < 1 {
			visibleRows = 1
		}
		halfPage := visibleRows / 2
		if halfPage < 1 {
			halfPage = 1
		}
		if a.activeTab == tabQueue {
			a.queueCursor -= halfPage
			if a.queueCursor < 0 {
				a.queueCursor = 0
			}
			if a.queueCursor < a.queueScrollOffset {
				a.queueScrollOffset = a.queueCursor
			}
		}
		if a.activeTab == tabLiked {
			a.likedCursor -= halfPage
			if a.likedCursor < 0 {
				a.likedCursor = 0
			}
			if a.likedCursor < a.likedScrollOffset {
				a.likedScrollOffset = a.likedCursor
			}
		}
		return a, nil
	case "G":
		visibleRows := a.height - 12
		if visibleRows < 1 {
			visibleRows = 1
		}
		if a.activeTab == tabQueue && len(a.tracklist) > 0 {
			a.queueCursor = len(a.tracklist) - 1
			if a.queueCursor >= a.queueScrollOffset+visibleRows {
				a.queueScrollOffset = a.queueCursor - visibleRows + 1
			}
		}
		if a.activeTab == tabLiked && len(a.likes.Tracks) > 0 {
			a.likedCursor = len(a.likes.Tracks) - 1
			if a.likedCursor >= a.likedScrollOffset+visibleRows {
				a.likedScrollOffset = a.likedCursor - visibleRows + 1
			}
		}
		return a, nil
	case "home", "ctrl+a":
		if a.activeTab == tabQueue {
			a.queueCursor = 0
			a.queueScrollOffset = 0
		}
		if a.activeTab == tabLiked {
			a.likedCursor = 0
			a.likedScrollOffset = 0
		}
		return a, nil
	case "t":
		a.showRemaining = !a.showRemaining
		a.config.ShowRemaining = a.showRemaining
		a.config.Save()
		return a, nil
	case "J":
		// Move selected queue track down one position
		if a.activeTab == tabQueue && a.queueCursor < len(a.tracklist)-1 {
			a = a.markDirty()
			i := a.queueCursor
			a.tracklist[i], a.tracklist[i+1] = a.tracklist[i+1], a.tracklist[i]
			if a.trackPos == i {
				a.trackPos = i + 1
			} else if a.trackPos == i+1 {
				a.trackPos = i
			}
			a.queueCursor = i + 1
			visibleRows := a.height - 12
			if visibleRows < 1 {
				visibleRows = 1
			}
			if a.queueCursor >= a.queueScrollOffset+visibleRows {
				a.queueScrollOffset = a.queueCursor - visibleRows + 1
			}
			a.saveQueue()
			a = a.withStatus(fmt.Sprintf("Moved: %s", a.tracklist[a.queueCursor].Title))
		}
		return a, nil
	case "K":
		// Move selected queue track up one position
		if a.activeTab == tabQueue && a.queueCursor > 0 {
			a = a.markDirty()
			i := a.queueCursor
			a.tracklist[i], a.tracklist[i-1] = a.tracklist[i-1], a.tracklist[i]
			if a.trackPos == i {
				a.trackPos = i - 1
			} else if a.trackPos == i-1 {
				a.trackPos = i
			}
			a.queueCursor = i - 1
			if a.queueCursor < a.queueScrollOffset {
				a.queueScrollOffset = a.queueCursor
			}
			a.saveQueue()
			a = a.withStatus(fmt.Sprintf("Moved: %s", a.tracklist[a.queueCursor].Title))
		}
		return a, nil
	case "c":
		// Jump to now playing track in queue
		if a.trackPos >= 0 && a.trackPos < len(a.tracklist) {
			a.queueCursor = a.trackPos
			a.activeTab = tabQueue
			visibleRows := a.height - 12
			if visibleRows < 1 {
				visibleRows = 1
			}
			// Center the playing track in the viewport
			a.queueScrollOffset = a.trackPos - visibleRows/2
			if a.queueScrollOffset < 0 {
				a.queueScrollOffset = 0
			}
			if a.queueScrollOffset+visibleRows > len(a.tracklist) {
				a.queueScrollOffset = len(a.tracklist) - visibleRows
				if a.queueScrollOffset < 0 {
					a.queueScrollOffset = 0
				}
			}
			a = a.withStatus("Jumped to now playing")
		}
		return a, nil
	case ":":
		a.mode = modeCommand
		a.cmdInput.Reset()
		a.cmdInput.Focus()
		return a, nil
	}
	return a, nil
}

func (a App) execCommand(input string) (App, tea.Cmd) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return a, nil
	}
	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "q", "quit":
		return a, tea.Quit
	case "w", "write":
		a.saveQueue()
		return a.withStatus("Queue saved"), nil
	case "shuffle":
		if len(a.tracklist) > 1 {
			rand.Shuffle(len(a.tracklist), func(i, j int) {
				a.tracklist[i], a.tracklist[j] = a.tracklist[j], a.tracklist[i]
			})
			a.trackPos = -1
			a.queueCursor = 0
			a.queueScrollOffset = 0
			a = a.markDirty()
			a.saveQueue()
			return a.withStatus("Queue shuffled"), nil
		}
		return a, nil
	case "discard":
		if a.activePlaylist != "" && a.playlistDirty {
			// Reload from disk
			tracks, err := a.playlists.Load(a.activePlaylist)
			if err == nil {
				a.tracklist = tracks
				a.trackPos = -1
				a.queueCursor = 0
				a.queueScrollOffset = 0
				a.playlistDirty = false
				a.saveQueue()
				return a.withStatus(fmt.Sprintf("Discarded changes to %s", a.activePlaylist)), nil
			}
		}
		a.playlistDirty = false
		a.activePlaylist = ""
		return a.withStatus("Discarded"), nil
	case "clear":
		a.tracklist = nil
		a.trackPos = -1
		a.queueCursor = 0
		a.queueScrollOffset = 0
		a.saveQueue()
		a.player.Stop()
		a.nowPlaying.track = nil
		a.activePlaylist = ""
		return a.withStatus("Queue cleared"), nil
	case "vol", "volume":
		if len(args) > 0 {
			v, err := strconv.Atoi(args[0])
			if err == nil {
				a = a.adjustVolume(v - a.volume)
				return a.withStatus(fmt.Sprintf("Volume: %d%%", a.volume)), nil
			}
		}
		return a.withStatus(fmt.Sprintf("Volume: %d%%", a.volume)), nil
	case "goto":
		if len(args) > 0 {
			line, err := strconv.Atoi(args[0])
			if err == nil && line >= 1 {
				idx := line - 1
				if a.activeTab == tabQueue {
					if idx >= len(a.tracklist) {
						idx = len(a.tracklist) - 1
					}
					a.queueCursor = idx
					visibleRows := a.height - 12
					if visibleRows < 1 {
						visibleRows = 1
					}
					if a.queueCursor < a.queueScrollOffset {
						a.queueScrollOffset = a.queueCursor
					}
					if a.queueCursor >= a.queueScrollOffset+visibleRows {
						a.queueScrollOffset = a.queueCursor - visibleRows + 1
					}
				} else if a.activeTab == tabLiked {
					if idx >= len(a.likes.Tracks) {
						idx = len(a.likes.Tracks) - 1
					}
					a.likedCursor = idx
				}
				return a.withStatus(fmt.Sprintf("Line %d", line)), nil
			}
		}
		return a.withStatus("Usage: goto <number>"), nil
	case "lines":
		a.showLineNumbers = !a.showLineNumbers
		a.config.ShowLineNumbers = a.showLineNumbers
		a.config.Save()
		if a.showLineNumbers {
			return a.withStatus("Line numbers on"), nil
		}
		return a.withStatus("Line numbers off"), nil
	case "playcounts":
		a.showPlayCounts = !a.showPlayCounts
		a.config.ShowPlayCounts = a.showPlayCounts
		a.config.Save()
		if a.showPlayCounts {
			return a.withStatus("Play counts on"), nil
		}
		return a.withStatus("Play counts off"), nil
	case "art":
		a.showAlbumArt = !a.showAlbumArt
		a.config.ShowAlbumArt = a.showAlbumArt
		a.config.Save()
		if a.showAlbumArt {
			return a.withStatus("Album art on"), nil
		}
		return a.withStatus("Album art off"), nil
	case "play", "p":
		if len(args) > 0 {
			// :play N — play track at line N
			n, err := strconv.Atoi(args[0])
			if err == nil && n >= 1 {
				return a.playPos(n - 1)
			}
		}
		// :play — play current cursor position
		if a.activeTab == tabQueue && len(a.tracklist) > 0 {
			return a.playPos(a.queueCursor)
		}
		return a, nil
	case "stop", "s":
		if a.nowPlaying.track != nil {
			a = a.stopPlayback()
			return a.withStatus("Stopped"), nil
		}
		return a, nil
	case "pause":
		if a.nowPlaying.track != nil {
			a.nowPlaying.paused = !a.nowPlaying.paused
			a.player.TogglePause()
			if a.nowPlaying.paused {
				return a.withStatus("Paused"), nil
			}
			return a.withStatus("Resumed"), nil
		}
		return a, nil
	case "next", "n":
		if a.trackPos < len(a.tracklist)-1 {
			return a.playPos(a.trackPos + 1)
		}
		return a.withStatus("End of queue"), nil
	case "prev":
		if a.trackPos > 0 {
			return a.playPos(a.trackPos - 1)
		}
		return a.withStatus("Start of queue"), nil
	case "seek":
		if len(args) > 0 && a.nowPlaying.track != nil {
			secs, err := strconv.Atoi(args[0])
			if err == nil {
				a.player.Seek(float64(secs))
				return a.withStatus(fmt.Sprintf("Seek %+ds", secs)), nil
			}
		}
		return a.withStatus("Usage: seek <seconds>"), nil
	case "quality":
		if len(args) > 0 {
			q := strings.ToUpper(args[0])
			for i, ql := range qualities {
				if ql == q {
					a.quality = i
					a.config.Quality = q
					a.config.Save()
					return a.withStatus(fmt.Sprintf("Quality: %s", q)), nil
				}
			}
			return a.withStatus("Quality: LOW | HIGH | LOSSLESS | HI_RES"), nil
		}
		a.quality = (a.quality + 1) % len(qualities)
		a.config.Quality = qualities[a.quality]
		a.config.Save()
		return a.withStatus(fmt.Sprintf("Quality: %s", qualities[a.quality])), nil
	case "like", "l":
		if target := a.targetTrackOrNowPlaying(); target != nil {
			if a.likes.Toggle(*target) {
				return a.withStatus(fmt.Sprintf("Liked: %s", target.Title)), nil
			}
			return a.withStatus(fmt.Sprintf("Unliked: %s", target.Title)), nil
		}
		return a, nil
	case "download", "dl":
		if a.dl != nil {
			if target := a.targetTrackOrNowPlaying(); target != nil {
				a.dl.QueueTrack(*target)
				return a.withStatus(fmt.Sprintf("Downloading: %s", target.Title)), nil
			}
		}
		return a, nil
	case "retry":
		if a.dl != nil {
			n := a.dl.RetryFailed()
			if n > 0 {
				return a.withStatus(fmt.Sprintf("Retrying %d downloads", n)), nil
			}
		}
		return a.withStatus("No failed downloads"), nil
	case "tab":
		if len(args) > 0 {
			var target viewTab
			switch args[0] {
			case "queue", "1":
				target = tabQueue
			case "liked", "2":
				target = tabLiked
			case "downloads", "3":
				target = tabDownloads
			case "playlists", "4":
				target = tabPlaylists
			default:
				return a.withStatus("Usage: tab queue|liked|downloads|playlists"), nil
			}
			a, _ = a.switchTab(target)
			return a, nil
		}
		return a.withStatus("Usage: tab queue|liked|downloads|playlists"), nil
	case "help":
		a.mode = modeHelp
		return a, nil
	case "save":
		if a.playlists == nil {
			return a.withStatus("Playlists unavailable"), nil
		}
		if len(args) == 0 {
			return a.withStatus("Usage: save <name>"), nil
		}
		name := args[0]
		if err := a.playlists.Save(name, a.tracklist); err != nil {
			return a.withStatus(fmt.Sprintf("Save failed: %s", err)), nil
		}
		a = a.refreshPlaylists()
		return a.withStatus(fmt.Sprintf("Saved: %s (%d tracks)", name, len(a.tracklist))), nil
	case "load":
		if a.playlists == nil {
			return a.withStatus("Playlists unavailable"), nil
		}
		if len(args) == 0 {
			return a.withStatus("Usage: load <name>"), nil
		}
		name := args[0]
		tracks, err := a.playlists.Load(name)
		if err != nil {
			return a.withStatus(fmt.Sprintf("Load failed: %s", err)), nil
		}
		a.tracklist = tracks
		a.trackPos = -1
		a.queueCursor = 0
		a.queueScrollOffset = 0
		a.activePlaylist = name
		a.saveQueue()
		return a.withStatus(fmt.Sprintf("Loaded: %s (%d tracks)", name, len(tracks))), nil
	case "playlists":
		if a.playlists == nil {
			return a.withStatus("Playlists unavailable"), nil
		}
		names := a.playlists.List()
		if len(names) == 0 {
			return a.withStatus("No saved playlists"), nil
		}
		return a.withStatus(strings.Join(names, ", ")), nil
	case "delete":
		if a.playlists == nil {
			return a.withStatus("Playlists unavailable"), nil
		}
		if len(args) == 0 {
			return a.withStatus("Usage: delete <name>"), nil
		}
		name := args[0]
		if err := a.playlists.Delete(name); err != nil {
			return a.withStatus(fmt.Sprintf("Delete failed: %s", err)), nil
		}
		a = a.refreshPlaylists()
		return a.withStatus(fmt.Sprintf("Deleted: %s", name)), nil
	default:
		// Try as a number (":42" = goto 42)
		if n, err := strconv.Atoi(cmd); err == nil && n >= 1 {
			return a.execCommand("goto " + cmd)
		}
		return a.withStatus(fmt.Sprintf("Unknown command: %s", cmd)), nil
	}
}

func (a App) updateCommand(msg tea.KeyMsg) (App, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.cmdInput.Blur()
		a.mode = modeNormal
		return a, nil
	case "enter":
		a.cmdInput.Blur()
		a.mode = modeNormal
		return a.execCommand(a.cmdInput.Value())
	}
	var cmd tea.Cmd
	a.cmdInput, cmd = a.cmdInput.Update(msg)
	return a, cmd
}

func (a App) updateSavePlaylist(msg tea.KeyMsg) (App, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.saveInput.Blur()
		a.mode = modeNormal
		return a, nil
	case "enter":
		name := strings.TrimSpace(a.saveInput.Value())
		if name == "" {
			a = a.withStatus("Name cannot be empty")
			return a, nil
		}
		if err := a.playlists.Save(name, a.saveTracks); err != nil {
			a = a.withStatus(fmt.Sprintf("Save failed: %s", err))
			a.saveInput.Blur()
			a.mode = modeNormal
			return a, nil
		}
		n := len(a.saveTracks)
		a.saveTracks = nil
		a = a.refreshPlaylists()
		a.saveInput.Blur()
		a.mode = modeNormal
		a = a.withStatus(fmt.Sprintf("Saved: %s (%d tracks)", name, n))
		return a, nil
	}
	var cmd tea.Cmd
	a.saveInput, cmd = a.saveInput.Update(msg)
	return a, cmd
}

func (a App) updateAddToPlaylist(msg tea.KeyMsg) (App, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.mode = modeNormal
		a.addToTrack = nil
		return a, nil
	case "enter":
		if a.addToTrack != nil && a.addToPickerIdx < len(a.addToPickerNames) {
			name := a.addToPickerNames[a.addToPickerIdx]
			tracks, err := a.playlists.Load(name)
			if err != nil {
				tracks = nil
			}
			tracks = append(tracks, *a.addToTrack)
			a.playlists.Save(name, tracks)
			a = a.refreshPlaylists()
			a = a.withStatus(fmt.Sprintf("Added to %s: %s", name, a.addToTrack.Title))
		}
		a.addToTrack = nil
		a.mode = modeNormal
		return a, nil
	case "up", "k":
		if a.addToPickerIdx > 0 {
			a.addToPickerIdx--
		}
		return a, nil
	case "down", "j":
		if a.addToPickerIdx < len(a.addToPickerNames)-1 {
			a.addToPickerIdx++
		}
		return a, nil
	}
	return a, nil
}

func (a App) updateRenamePlaylist(msg tea.KeyMsg) (App, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.renameInput.Blur()
		a.mode = modeNormal
		return a, nil
	case "enter":
		newName := strings.TrimSpace(a.renameInput.Value())
		if newName == "" {
			a = a.withStatus("Name cannot be empty")
			return a, nil
		}
		oldName := a.renameFrom
		tracks, err := a.playlists.Load(oldName)
		if err != nil {
			a = a.withStatus(fmt.Sprintf("Load failed: %s", err))
			a.renameInput.Blur()
			a.mode = modeNormal
			return a, nil
		}
		if err := a.playlists.Save(newName, tracks); err != nil {
			a = a.withStatus(fmt.Sprintf("Save failed: %s", err))
			a.renameInput.Blur()
			a.mode = modeNormal
			return a, nil
		}
		if oldName != newName {
			a.playlists.Delete(oldName)
		}
		a = a.refreshPlaylists()
		if a.playlistCursor >= len(a.playlistNames) && a.playlistCursor > 0 {
			a.playlistCursor = len(a.playlistNames) - 1
		}
		a.renameInput.Blur()
		a.renameFrom = ""
		a.mode = modeNormal
		a = a.withStatus(fmt.Sprintf("Renamed: %s -> %s", oldName, newName))
		return a, nil
	}
	var cmd tea.Cmd
	a.renameInput, cmd = a.renameInput.Update(msg)
	return a, cmd
}

// computeFilteredIndices builds the filteredIndices slice based on filterText
// and the current tab. Call this any time filterText or the underlying list changes.
func (a App) computeFilteredIndices() App {
	query := strings.ToLower(a.filterText)
	a.filteredIndices = nil
	if query == "" {
		return a
	}
	switch a.activeTab {
	case tabQueue:
		for i, t := range a.tracklist {
			if strings.Contains(strings.ToLower(t.Title), query) ||
				strings.Contains(strings.ToLower(t.Artist.Name), query) {
				a.filteredIndices = append(a.filteredIndices, i)
			}
		}
	case tabLiked:
		for i, t := range a.likes.Tracks {
			if strings.Contains(strings.ToLower(t.Title), query) ||
				strings.Contains(strings.ToLower(t.Artist.Name), query) {
				a.filteredIndices = append(a.filteredIndices, i)
			}
		}
	}
	return a
}

func (a App) updateFilter(msg tea.KeyMsg) (App, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.filterInput.Blur()
		a.filterInput.Reset()
		a.filterText = ""
		a.filteredIndices = nil
		a.mode = modeNormal
		return a, nil
	case "up", "k":
		if len(a.filteredIndices) > 0 {
			// find current cursor position in filtered list and move up
			cursorIdx := -1
			var cursor int
			if a.activeTab == tabQueue {
				cursor = a.queueCursor
			} else {
				cursor = a.likedCursor
			}
			for fi, idx := range a.filteredIndices {
				if idx == cursor {
					cursorIdx = fi
					break
				}
			}
			if cursorIdx > 0 {
				newIdx := a.filteredIndices[cursorIdx-1]
				if a.activeTab == tabQueue {
					a.queueCursor = newIdx
					visibleRows := a.height - 12
					if visibleRows < 1 {
						visibleRows = 1
					}
					if a.queueCursor < a.queueScrollOffset {
						a.queueScrollOffset = a.queueCursor
					}
				} else {
					a.likedCursor = newIdx
					visibleRows := a.height - 12
					if visibleRows < 1 {
						visibleRows = 1
					}
					if a.likedCursor < a.likedScrollOffset {
						a.likedScrollOffset = a.likedCursor
					}
				}
			}
		}
		return a, nil
	case "down", "j":
		if len(a.filteredIndices) > 0 {
			cursorIdx := -1
			var cursor int
			if a.activeTab == tabQueue {
				cursor = a.queueCursor
			} else {
				cursor = a.likedCursor
			}
			for fi, idx := range a.filteredIndices {
				if idx == cursor {
					cursorIdx = fi
					break
				}
			}
			if cursorIdx == -1 {
				cursorIdx = -1 // will move to first
			}
			nextFI := cursorIdx + 1
			if nextFI < len(a.filteredIndices) {
				newIdx := a.filteredIndices[nextFI]
				visibleRows := a.height - 12
				if visibleRows < 1 {
					visibleRows = 1
				}
				if a.activeTab == tabQueue {
					a.queueCursor = newIdx
					if a.queueCursor >= a.queueScrollOffset+visibleRows {
						a.queueScrollOffset = a.queueCursor - visibleRows + 1
					}
				} else {
					a.likedCursor = newIdx
					if a.likedCursor >= a.likedScrollOffset+visibleRows {
						a.likedScrollOffset = a.likedCursor - visibleRows + 1
					}
				}
			}
		}
		return a, nil
	case "enter":
		// Play the currently focused track
		if a.activeTab == tabQueue && len(a.tracklist) > 0 {
			a.mode = modeNormal
			a.filterInput.Blur()
			return a.playPos(a.queueCursor)
		}
		if a.activeTab == tabLiked && len(a.likes.Tracks) > 0 {
			a.mode = modeNormal
			a.filterInput.Blur()
			track := a.likes.Tracks[a.likedCursor]
			return a.addAndPlay(&track)
		}
		return a, nil
	}
	// Delegate text input
	var cmd tea.Cmd
	a.filterInput, cmd = a.filterInput.Update(msg)
	a.filterText = a.filterInput.Value()
	if a.filterText == "" {
		a.filteredIndices = nil
		a.mode = modeNormal
		a.filterInput.Blur()
		return a, cmd
	}
	a = a.computeFilteredIndices()
	// Jump cursor to first match
	if len(a.filteredIndices) > 0 {
		newIdx := a.filteredIndices[0]
		visibleRows := a.height - 12
		if visibleRows < 1 {
			visibleRows = 1
		}
		if a.activeTab == tabQueue {
			a.queueCursor = newIdx
			if a.queueCursor >= a.queueScrollOffset+visibleRows {
				a.queueScrollOffset = a.queueCursor - visibleRows + 1
			}
			if a.queueCursor < a.queueScrollOffset {
				a.queueScrollOffset = a.queueCursor
			}
		} else if a.activeTab == tabLiked {
			a.likedCursor = newIdx
			if a.likedCursor >= a.likedScrollOffset+visibleRows {
				a.likedScrollOffset = a.likedCursor - visibleRows + 1
			}
			if a.likedCursor < a.likedScrollOffset {
				a.likedScrollOffset = a.likedCursor
			}
		}
	}
	return a, cmd
}

// --- Main Update dispatcher ---

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case tea.MouseMsg:
		if a.mode != modeNormal {
			return a, nil
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if a.activeTab == tabQueue && a.queueCursor > 0 {
				a.queueCursor--
				visibleRows := a.height - 12
				if visibleRows < 1 {
					visibleRows = 1
				}
				if a.queueCursor < a.queueScrollOffset {
					a.queueScrollOffset = a.queueCursor
				}
			}
			if a.activeTab == tabLiked && a.likedCursor > 0 {
				a.likedCursor--
				visibleRows := a.height - 12
				if visibleRows < 1 {
					visibleRows = 1
				}
				if a.likedCursor < a.likedScrollOffset {
					a.likedScrollOffset = a.likedCursor
				}
			}
		case tea.MouseButtonWheelDown:
			if a.activeTab == tabQueue && a.queueCursor < len(a.tracklist)-1 {
				a.queueCursor++
				visibleRows := a.height - 12
				if visibleRows < 1 {
					visibleRows = 1
				}
				if a.queueCursor >= a.queueScrollOffset+visibleRows {
					a.queueScrollOffset = a.queueCursor - visibleRows + 1
				}
			}
			if a.activeTab == tabLiked && a.likedCursor < len(a.likes.Tracks)-1 {
				a.likedCursor++
				visibleRows := a.height - 12
				if visibleRows < 1 {
					visibleRows = 1
				}
				if a.likedCursor >= a.likedScrollOffset+visibleRows {
					a.likedScrollOffset = a.likedCursor - visibleRows + 1
				}
			}
		case tea.MouseButtonLeft:
			// Tab bar is at row 2 (0-indexed)
			if msg.Y == 2 {
				var target viewTab
				if msg.X < 15 {
					target = tabQueue
				} else if msg.X < 28 {
					target = tabLiked
				} else if msg.X < 44 {
					target = tabDownloads
				} else {
					target = tabPlaylists
				}
				a, ok := a.switchTab(target)
				if ok {
					a.saveUIState()
				}
			} else if msg.Y >= 5 {
				// Content area starts around row 5 (header row 4, then tracks)
				// Row 5 = track header, row 6 onwards = tracks
				contentRow := msg.Y - 6
				if contentRow >= 0 {
					if a.activeTab == tabQueue {
						idx := a.queueScrollOffset + contentRow
						if idx >= 0 && idx < len(a.tracklist) {
							a.queueCursor = idx
						}
					} else if a.activeTab == tabLiked {
						idx := a.likedScrollOffset + contentRow
						if idx >= 0 && idx < len(a.likes.Tracks) {
							a.likedCursor = idx
						}
					}
				}
			}
		}
		return a, nil

	case pingMsg:
		a.online = msg.online
		return a, nil

	case tickMsg:
		a.tickCount++
		a.spinnerIdx = (a.spinnerIdx + 1) % len(spinnerFrames)
		// Every 10th tick (~1 second): update position, status, sync
		if a.tickCount%10 == 0 {
			a = a.syncNowPlaying()
			if a.statusTicks > 0 {
				a.statusTicks--
				if a.statusTicks == 0 {
					a.statusMsg = ""
				}
			}
			if a.nowPlaying.track != nil && !a.nowPlaying.paused {
				pos, dur, err := a.player.GetPosition()
				if err == nil {
					a.nowPlaying.position = pos
					a.nowPlaying.duration = dur
				}
			}
			// Fetch audio format/rate once per track
			if a.nowPlaying.track != nil && a.audioInfo == "" {
				format, err1 := a.player.GetAudioFormat()
				rate, err2 := a.player.GetSampleRate()
				if err1 == nil && err2 == nil && format != "" && rate > 0 {
					a.audioInfo = fmt.Sprintf("%s %dkHz", strings.ToUpper(format), rate/1000)
				}
			}
			// Re-ping every 100 ticks (~10 seconds) when offline
			if !a.online && a.tickCount%100 == 0 {
				return a, tea.Batch(tick(), a.doPing())
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
		case modeFilter:
			a, cmd := a.updateFilter(msg)
			return a, cmd
		case modeCommand:
			a, cmd := a.updateCommand(msg)
			return a, cmd
		case modeSavePlaylist:
			a, cmd := a.updateSavePlaylist(msg)
			return a, cmd
		case modeRenamePlaylist:
			a, cmd := a.updateRenamePlaylist(msg)
			return a, cmd
		case modeAddToPlaylist:
			a, cmd := a.updateAddToPlaylist(msg)
			return a, cmd
		default:
			a, cmd := a.updateNormal(msg)
			return a, cmd
		}

	case streamURLMsg:
		if msg.err != nil {
			if isNetworkError(msg.err) {
				a.online = false
			}
			if a.streamRetries < 1 {
				a.streamRetries++
				// Auto-retry once on error
				if a.trackPos >= 0 && a.trackPos < len(a.tracklist) {
					track := &a.tracklist[a.trackPos]
					trackID := track.ID
					q := qualities[a.quality]
					return a, func() tea.Msg {
						url, err := a.client.GetStreamURL(trackID, q)
						return streamURLMsg{url: url, err: err}
					}
				}
			}
			a.loading = false
			a.streamRetries = 0
			a.err = msg.err
			return a, nil
		}
		a.loading = false
		a.streamRetries = 0
		a.online = true
		if err := a.player.Play(msg.url); err != nil {
			a.err = err
			return a, nil
		}
		cmds := []tea.Cmd{a.makeWaitForTrackEnd(a.playGen)}
		if a.trackPos >= 0 && a.trackPos < len(a.tracklist) {
			if coverID := a.tracklist[a.trackPos].Album.Cover; coverID != "" {
				cmds = append(cmds, a.fetchArt(coverID))
			}
		}
		return a, tea.Batch(cmds...)

	case trackEndedMsg:
		if msg.gen != a.playGen {
			return a, nil
		}
		// Increment play count for the track that just finished
		if a.trackPos >= 0 && a.trackPos < len(a.tracklist) && a.playCounts != nil {
			a.playCounts.Increment(a.tracklist[a.trackPos].ID)
		}
		if a.trackPos < len(a.tracklist)-1 {
			return a.playPos(a.trackPos + 1)
		}
		a.nowPlaying.track = nil
		a.nowPlaying.paused = false
		return a, nil

	case queueAlbumMsg:
		if msg.err != nil {
			if isNetworkError(msg.err) {
				a.online = false
			}
			a.err = msg.err
			return a, nil
		}
		a.online = true
		a = a.withQueueAddAll(msg.tracks)
		return a, nil

	case DownloadUpdateMsg:
		return a, nil

	case albumArtMsg:
		if a.nowPlaying.track != nil && a.nowPlaying.track.Album.Cover == msg.coverID {
			a.nowPlaying.albumArt = msg.art
		}
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

func (a App) hasDownloadActivity() bool {
	if a.dl == nil {
		return false
	}
	st := a.dl.Status()
	return st.Active+st.Completed+st.Failed+st.Queued > 0
}

func (a App) renderTabBar() string {
	type tabEntry struct {
		label string
		tab   viewTab
	}
	tabs := []tabEntry{
		{"1:Queue", tabQueue},
		{"2:Liked", tabLiked},
	}
	if a.hasDownloadActivity() {
		tabs = append(tabs, tabEntry{"3:Downloads", tabDownloads})
	}
	plLabel := "4:Playlists"
	if a.playlists != nil {
		if names := a.playlists.List(); len(names) > 0 {
			plLabel = fmt.Sprintf("4:Playlists(%d)", len(names))
		}
	}
	tabs = append(tabs, tabEntry{plLabel, tabPlaylists})

	dimmedAll := a.searchVisible() || a.mode == modeHelp || a.mode == modeFilter || a.mode == modeSavePlaylist || a.mode == modeRenamePlaylist || a.mode == modeAddToPlaylist

	selCount := len(a.selected)
	var parts []string
	for _, t := range tabs {
		label := t.label
		if t.tab == tabQueue && len(a.tracklist) > 0 {
			if a.activePlaylist != "" {
				dirty := ""
				if a.playlistDirty {
					dirty = "*"
				}
				label = fmt.Sprintf("1:Queue [%s%s](%d)", a.activePlaylist, dirty, len(a.tracklist))
			} else {
				label = fmt.Sprintf("1:Queue(%d)", len(a.tracklist))
			}
		}
		if t.tab == tabLiked && len(a.likes.Tracks) > 0 {
			label = fmt.Sprintf("2:Liked(%d)", len(a.likes.Tracks))
		}
		if selCount > 0 && t.tab == a.activeTab {
			label += fmt.Sprintf(" [%d sel]", selCount)
		}
		if !dimmedAll && t.tab == a.activeTab {
			parts = append(parts, activeTabStyle.Render(" "+label+" "))
		} else {
			parts = append(parts, dimStyle.Render(" "+label+" "))
		}
	}
	return "  " + strings.Join(parts, dimStyle.Render("│"))
}

// isFilterMatch reports whether index i is in the filtered set.
// When filter is inactive (filterText == ""), all indices match.
func (a App) isFilterMatch(i int) bool {
	if a.filterText == "" {
		return true
	}
	for _, fi := range a.filteredIndices {
		if fi == i {
			return true
		}
	}
	return false
}

func (a App) renderQueueView() string {
	if len(a.tracklist) == 0 {
		return dimStyle.Render("  Queue is empty. Press 'a' on a track to add it.")
	}

	visibleRows := a.height - 12
	if visibleRows < 1 {
		visibleRows = 1
	}

	tc := computeTrackCols(a.width)
	s := ""
	if a.mode == modeFilter {
		s += "  " + a.filterInput.View() + "\n"
	}
	// Queue header (respects showLineNumbers)
	qh := "   "
	if a.showLineNumbers {
		qh += colRight("#", colNum, headerStyle)
	}
	qh += col("Artist", tc.artist, headerStyle) +
		col("Title", tc.title, headerStyle)
	if tc.showAlbum {
		qh += col("Album", tc.album, headerStyle)
	}
	if tc.showYear {
		qh += colRight("Year", colYear, headerStyle)
	}
	qh += colRight("Time", colDuration, headerStyle)
	s += qh + "\n"

	end := a.queueScrollOffset + visibleRows
	if end > len(a.tracklist) {
		end = len(a.tracklist)
	}

	if a.queueScrollOffset > 0 {
		s += dimStyle.Render(fmt.Sprintf("  ^ %d more above", a.queueScrollOffset)) + "\n"
	}

	for displayIdx, i := 0, a.queueScrollOffset; i < end; i, displayIdx = i+1, displayIdx+1 {
		t := a.tracklist[i]
		isPlaying := i == a.trackPos
		isCursor := i == a.queueCursor
		played := a.trackPos >= 0 && i < a.trackPos
		isMatch := a.isFilterMatch(i)

		duration := fmt.Sprintf("%d:%02d", t.Duration/60, t.Duration%60)
		num := fmt.Sprintf("%d", i+1)
		isSelected := a.selected[i]
		icons := statusIcons(a.likes.IsLiked(t.ID), a.dl != nil && a.dl.IsDownloaded(t))

		var numSt, artSt, albSt, titSt, durSt lipgloss.Style
		var marker string

		if !isMatch {
			marker = " "
			numSt, artSt, albSt, titSt, durSt = dimStyle, dimStyle, dimStyle, dimStyle, dimStyle
		} else {
			switch {
			case isCursor && isPlaying:
				marker = titleStyle.Render("▸")
				numSt, artSt, albSt, titSt, durSt = playingSelectedStyle, playingSelectedStyle, playingSelectedStyle, playingSelectedStyle, playingSelectedStyle
			case isPlaying:
				marker = playingStyle.Render("♫")
				numSt, artSt, albSt, titSt, durSt = playingStyle, playingStyle, playingStyle, playingStyle, playingStyle
			case isCursor:
				marker = selectionStripe.Render("▸")
				numSt, artSt, albSt, titSt, durSt = dimStyle.Bold(true), artistStyle.Bold(true), dimStyle.Bold(true), normalStyle.Bold(true), dimStyle.Bold(true)
			case played:
				marker = " "
				numSt, artSt, albSt, titSt, durSt = dimStyle, dimStyle, dimStyle, dimStyle, dimStyle
			default:
				marker = " "
				numSt, artSt, albSt, titSt, durSt = dimStyle, artistStyle, dimStyle, normalStyle, dimStyle
			}
		}
		if isSelected {
			marker = titleStyle.Render("●")
		}

		// Prepend album track number to title when available
		titleText := t.Title
		if t.TrackNumber > 0 {
			titleText = fmt.Sprintf("%02d. %s", t.TrackNumber, t.Title)
		}

		// Play count suffix
		playCountSuffix := ""
		if a.showPlayCounts && a.playCounts != nil {
			if cnt := a.playCounts.Get(t.ID); cnt > 0 {
				playCountSuffix = dimStyle.Render(fmt.Sprintf(" ×%d", cnt))
			}
		}

		var row string
		if tc.artist == 0 {
			// Ultra-narrow: title only
			row = marker + icons + col(titleText, tc.title, titSt) + playCountSuffix
		} else {
			row = marker + icons
			if a.showLineNumbers {
				row += colRight(num, colNum, numSt)
			}
			row +=	col(t.Artist.Name, tc.artist, artSt) +
				col(titleText, tc.title, titSt)
			if tc.showAlbum {
				row += col(t.Album.Title, tc.album, albSt)
			}
			if tc.showYear {
				row += colRight(trackYear(t), colYear, durSt)
			}
			row += colRight(duration, colDuration, durSt) + playCountSuffix
		}
		s += row + "\n"
	}

	if end < len(a.tracklist) {
		s += dimStyle.Render(fmt.Sprintf("  v %d more below", len(a.tracklist)-end)) + "\n"
	}

	return s
}

func (a App) renderLikedView() string {
	if len(a.likes.Tracks) == 0 {
		return dimStyle.Render("  No liked tracks yet. Press 'l' on a track to like it.")
	}

	visibleRows := a.height - 12
	if visibleRows < 1 {
		visibleRows = 1
	}

	tc := computeTrackCols(a.width)
	s := ""
	if a.mode == modeFilter {
		s += "  " + a.filterInput.View() + "\n"
	}
	s += trackHeader(tc) + "\n"

	end := a.likedScrollOffset + visibleRows
	if end > len(a.likes.Tracks) {
		end = len(a.likes.Tracks)
	}

	if a.likedScrollOffset > 0 {
		s += dimStyle.Render(fmt.Sprintf("  ^ %d more above", a.likedScrollOffset)) + "\n"
	}

	for displayIdx, i := 0, a.likedScrollOffset; i < end; i, displayIdx = i+1, displayIdx+1 {
		t := a.likes.Tracks[i]
		isCursor := i == a.likedCursor
		isMatch := a.isFilterMatch(i)
		// When filter is active and this track doesn't match, show dimmed
		if !isMatch {
			row := trackRow(i, t, false, true, a.dl != nil && a.dl.IsDownloaded(t), tc)
			row = dimStyle.Render(row)
			s += row + "\n"
			continue
		}
		row := trackRow(i, t, isCursor, true, a.dl != nil && a.dl.IsDownloaded(t), tc)
		if a.selected[t.ID] {
			// Replace leading space/cursor with selection marker
			row = titleStyle.Render("●") + row[1:]
		}
		s += row + "\n"
	}

	if end < len(a.likes.Tracks) {
		s += dimStyle.Render(fmt.Sprintf("  v %d more below", len(a.likes.Tracks)-end)) + "\n"
	}

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
		failStyle := errorStyle
		s += failStyle.Render(fmt.Sprintf("  Failed: %d", st.Failed)) + "\n"
		if st.LastError != "" {
			s += failStyle.Render(fmt.Sprintf("  Last error: %s", st.LastError)) + "\n"
		}
	}

	return s
}

func (a App) renderPlaylistsView() string {
	names := a.playlistNames
	if len(names) == 0 && a.playlists != nil {
		names = a.playlists.List()
	}
	if len(names) == 0 {
		return dimStyle.Render("  No saved playlists. Use :save <name> to create one.")
	}

	s := "   " + headerStyle.Render("Playlist") + "\n"
	for i, name := range names {
		if i == a.playlistCursor {
			s += selectionStripe.Render("▸") + "  " + normalStyle.Bold(true).Render(name) + "\n"
		} else {
			s += "   " + normalStyle.Render(name) + "\n"
		}
	}
	return s
}

func (a App) View() string {
	// --- Header ---
	var header string
	if a.height < 20 {
		onlineSuffix := ""
		if !a.online {
			onlineSuffix = "  " + errorStyle.Render("OFFLINE")
		}
		header = "  " + titleStyle.Render("riff") + onlineSuffix
	} else {
		logo := "  " + titleStyle.Render("╦═╗╦╔═╗╔═╗") + "\n" +
			"  " + titleStyle.Render("╠╦╝║╠╣ ╠╣ ") + "\n" +
			"  " + titleStyle.Render("╩╚═╩╚  ╚  ")
		if !a.online {
			logo += "  " + errorStyle.Render("OFFLINE")
		}
		header = logo
	}

	// --- Tab bar ---
	tabBar := a.renderTabBar()

	// --- Now playing / loading bar ---
	spinner := spinnerFrames[a.spinnerIdx]
	var np string
	if a.loading {
		np = nowPlayingStyle.Render(dimStyle.Render("  " + spinner + " Loading stream..."))
	} else {
		np = a.nowPlaying.View(a.width)
	}

	// --- Error view ---
	errView := ""
	if a.err != nil {
		errView = errorStyle.Render(fmt.Sprintf("  Error: %s", a.err))
	}

	// --- Status line ---
	statusLine := ""
	if a.statusMsg != "" {
		var statusStyle lipgloss.Style
		if a.statusTicks <= 1 {
			statusStyle = dimStyle
		} else {
			statusStyle = titleStyle
		}
		statusLine = "  " + statusStyle.Render(a.statusMsg)
	}

	// --- Download status ---
	dlStatus := ""
	if a.dl != nil {
		st := a.dl.Status()
		if st.Active > 0 || st.Queued > 0 {
			dlStatus = dimStyle.Render(fmt.Sprintf("  DL: %d active, %d queued, %d done", st.Active, st.Queued, st.Completed))
			if st.Current != "" {
				dlStatus += dimStyle.Render("  " + st.Current)
			}
		} else if st.Completed > 0 || st.Failed > 0 {
			parts := []string{}
			if st.Completed > 0 {
				parts = append(parts, fmt.Sprintf("%d done", st.Completed))
			}
			if st.Failed > 0 {
				parts = append(parts, fmt.Sprintf("%d failed", st.Failed))
			}
			dlStatus = dimStyle.Render("  DL: " + strings.Join(parts, ", "))
		}
	}

	// --- Help bar / command line ---
	var help string
	if a.mode == modeCommand {
		help = "  " + a.cmdInput.View()
	} else {
		help = a.contextHelp()
	}

	// --- Separator lines ---
	separator := dimStyle.Render(strings.Repeat("─", a.width))

	// --- Footer (fixed bottom) ---
	var footerParts []string
	footerParts = append(footerParts, separator)
	if statusLine != "" {
		footerParts = append(footerParts, statusLine)
	}
	if dlStatus != "" {
		footerParts = append(footerParts, dlStatus)
	}
	footerParts = append(footerParts, np, help)
	footer := strings.Join(footerParts, "\n")

	// --- Fixed top (header + tab bar + separator) ---
	topFixed := lipgloss.JoinVertical(lipgloss.Left, "", header, tabBar, separator)

	// --- Calculate remaining content height ---
	topHeight := lipgloss.Height(topFixed)
	footerHeight := lipgloss.Height(footer)
	contentHeight := a.height - topHeight - footerHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	// --- Content area ---
	var content string
	switch {
	case a.mode == modeSavePlaylist:
		savePopup := overlayBorder.Padding(1, 2).Render(a.saveInput.View())
		if noColor {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, savePopup)
		} else {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, savePopup,
				lipgloss.WithWhitespaceBackground(lipgloss.Color("#0D0D0D")),
			)
		}

	case a.mode == modeRenamePlaylist:
		renamePopup := overlayBorder.Padding(1, 2).Render(a.renameInput.View())
		if noColor {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, renamePopup)
		} else {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, renamePopup,
				lipgloss.WithWhitespaceBackground(lipgloss.Color("#0D0D0D")),
			)
		}

	case a.mode == modeAddToPlaylist:
		title := "Add to playlist"
		if a.addToTrack != nil {
			title = fmt.Sprintf("Add \"%s\" to:", a.addToTrack.Title)
		}
		var pickerContent string
		pickerContent = titleStyle.Render(title) + "\n\n"
		for i, name := range a.addToPickerNames {
			if i == a.addToPickerIdx {
				pickerContent += selectionStripe.Render("▸") + " " + normalStyle.Bold(true).Render(name) + "\n"
			} else {
				pickerContent += "  " + normalStyle.Render(name) + "\n"
			}
		}
		addPopup := overlayBorder.Padding(1, 2).Render(pickerContent)
		if noColor {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, addPopup)
		} else {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, addPopup,
				lipgloss.WithWhitespaceBackground(lipgloss.Color("#0D0D0D")),
			)
		}

	case a.mode == modeHelp:
		helpOverlay := overlayBorder.
			Padding(1, 2).
			Render(
				titleStyle.Render("Keybindings") + "\n\n" +
					helpLine("1-4", "Switch tabs") +
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
					helpLine("J/K", "Move queue track down/up") +
					helpLine("c", "Jump to now playing") +
					helpLine("t", "Toggle elapsed/remaining time") +
					helpLine("G / home", "Jump to last / first item") +
					helpLine("ctrl+u/d", "Page up / page down") +
					helpLine("f", "Filter current list") +
					helpLine("d", "Download track") +
					helpLine("D", "Download album") +
					helpLine("l", "Toggle like") +
					helpLine("u", "Open album in browser") +
					helpLine("Q", "Cycle quality") +
					helpLine("?", "Toggle this help") +
					helpLine("q", "Quit"),
			)
		if noColor {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, helpOverlay)
		} else {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, helpOverlay,
				lipgloss.WithWhitespaceBackground(lipgloss.Color("#0D0D0D")),
			)
		}

	case a.searchVisible():
		searchOverlay := overlayBorder.
			Padding(1, 2).
			Width(a.width - 6).
			Render(a.search.View(a.width-12, a.likes.IsLiked, a.dlCheck(), spinner))
		popup := searchOverlay
		if errView != "" {
			popup = lipgloss.JoinVertical(lipgloss.Left, searchOverlay, errView)
		}
		if noColor {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, popup)
		} else {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, popup,
				lipgloss.WithWhitespaceBackground(lipgloss.Color("#0D0D0D")),
			)
		}

	default:
		var tabContent string
		switch a.activeTab {
		case tabQueue:
			tabContent = a.renderQueueView()
		case tabLiked:
			tabContent = a.renderLikedView()
		case tabDownloads:
			tabContent = a.renderDownloadsView()
		case tabPlaylists:
			tabContent = a.renderPlaylistsView()
		}
		if errView != "" {
			tabContent = lipgloss.JoinVertical(lipgloss.Left, tabContent, errView)
		}
		content = lipgloss.NewStyle().Height(contentHeight).Render(tabContent)
	}

	return lipgloss.JoinVertical(lipgloss.Left, topFixed, content, footer)
}

func (a App) contextHelp() string {
	switch a.mode {
	case modeSearchInput:
		return dimStyle.Render("  enter search  tab mode  esc close")
	case modeSearchBrowse:
		return dimStyle.Render("  enter select  a queue  d download  esc close")
	case modeHelp:
		return dimStyle.Render("  esc close")
	case modeFilter:
		return dimStyle.Render("  type to filter  enter play  esc clear")
	case modeSavePlaylist:
		return dimStyle.Render("  enter save  esc cancel")
	case modeRenamePlaylist:
		return dimStyle.Render("  enter rename  esc cancel")
	case modeAddToPlaylist:
		return dimStyle.Render("  ↑↓ select  enter add  esc cancel")
	default:
		switch a.activeTab {
		case tabLiked:
			return dimStyle.Render("  enter play  a queue  l unlike  / search  ? more  q quit")
		case tabDownloads:
			return dimStyle.Render("  r retry  / search  ? more  q quit")
		case tabPlaylists:
			return dimStyle.Render("  enter load  a append  r rename  x delete  / search  ? more  q quit")
		default:
			return dimStyle.Render("  enter play  x remove  S save  J/K move  / search  ? more  q quit")
		}
	}
}

func helpLine(key, desc string) string {
	return fmt.Sprintf("  %s  %s\n",
		titleStyle.Width(12).Render(key),
		desc,
	)
}

// isNetworkError returns true if the error looks like a connectivity failure.
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "connection refused") ||
		strings.Contains(s, "no such host") ||
		strings.Contains(s, "i/o timeout") ||
		strings.Contains(s, "network") ||
		strings.Contains(s, "all instances failed")
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
