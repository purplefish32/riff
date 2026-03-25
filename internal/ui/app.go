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

type pingMsg struct{ online bool }

type trackEndedMsg struct{ gen int }

type queueAlbumMsg struct {
	tracks []types.Track
	err    error
}

var qualities = []string{"LOW", "HIGH", "LOSSLESS", "HI_RES"}

var spinnerFrames = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█", "▇", "▆", "▅", "▄", "▃", "▂"}

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
	playGen          int
	quality          int
	volume           int
	width            int
	height           int
	err              error
	statusMsg        string
	statusTicks      int // ticks remaining before status clears
}

func NewApp(client *api.Client, player *player.Player, likes *persistence.LikedStore, dl *downloader.Downloader, cfg *persistence.Config, qs *persistence.QueueStore) App {
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
	return App{
		mode:        mode,
		activeTab:   viewTab(qs.ActiveTab),
		search:      newSearchModel(),
		nowPlaying:  newNowPlayingModel(),
		client:      client,
		player:      player,
		likes:       likes,
		dl:          dl,
		config:      cfg,
		queueStore:  qs,
		tracklist:   qs.Tracks,
		trackPos:    qs.Position,
		quality:     cfg.QualityIndex(),
		volume:      cfg.Volume,
		queueCursor: queueCursor,
		likedCursor: likedCursor,
		selected:    make(map[int]bool),
		online:      true,
	}
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

func (a App) withQueueAdd(track types.Track) App {
	if len(a.tracklist) >= maxTracklist {
		return a.withStatus("Queue full (500 max)")
	}
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
	return a
}

func (a App) withStatus(msg string) App {
	a.statusMsg = msg
	a.statusTicks = 3
	return a
}

func (a App) syncNowPlaying() App {
	a.nowPlaying.quality = qualities[a.quality]
	a.nowPlaying.volume = a.volume
	if a.nowPlaying.track != nil {
		a.nowPlaying.liked = a.likes.IsLiked(a.nowPlaying.track.ID)
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
		// Skip API call if query and mode are unchanged
		if query == a.search.lastQuery && a.search.mode == a.search.lastMode {
			return a, nil
		}
		a.search.loading = true
		a.search.lastQuery = query
		a.search.lastMode = a.search.mode
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
		a.saveUIState()
		return a, nil
	case "2":
		a.activeTab = tabLiked
		a.saveUIState()
		return a, nil
	case "3":
		a.activeTab = tabDownloads
		a.saveUIState()
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
		if a.activeTab == tabDownloads && a.dl != nil {
			n := a.dl.RetryFailed()
			if n > 0 {
				a = a.withStatus(fmt.Sprintf("Retrying %d downloads", n))
			} else {
				a = a.withStatus("No failed downloads to retry")
			}
		}
		return a, nil
	case "Q":
		a.quality = (a.quality + 1) % len(qualities)
		a.config.Quality = qualities[a.quality]
		a.config.Save()
		a = a.withStatus(fmt.Sprintf("Quality: %s", qualities[a.quality]))
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
				// Determine which tab was clicked based on approximate x positions
				// Tab labels: "1:Queue(N)", "2:Liked(N)", "3:Downloads"
				// Each tab is separated by "│"; rough x breakpoints
				if msg.X < 15 {
					a.activeTab = tabQueue
					a.saveUIState()
				} else if msg.X < 28 {
					a.activeTab = tabLiked
					a.saveUIState()
				} else {
					a.activeTab = tabDownloads
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

	dimmedAll := a.searchVisible() || a.mode == modeHelp

	selCount := len(a.selected)
	var parts []string
	for _, t := range tabs {
		label := t.label
		if t.tab == tabQueue && len(a.tracklist) > 0 {
			label = fmt.Sprintf("1:Queue(%d)", len(a.tracklist))
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

func (a App) renderQueueView() string {
	if len(a.tracklist) == 0 {
		return dimStyle.Render("  Queue is empty. Press 'a' on a track to add it.")
	}

	visibleRows := a.height - 12
	if visibleRows < 1 {
		visibleRows = 1
	}

	tc := computeTrackCols(a.width)
	s := trackHeader(tc) + "\n"

	end := a.queueScrollOffset + visibleRows
	if end > len(a.tracklist) {
		end = len(a.tracklist)
	}

	if a.queueScrollOffset > 0 {
		s += dimStyle.Render(fmt.Sprintf("  ^ %d more above", a.queueScrollOffset)) + "\n"
	}

	for i := a.queueScrollOffset; i < end; i++ {
		t := a.tracklist[i]
		isPlaying := i == a.trackPos
		isCursor := i == a.queueCursor
		played := a.trackPos >= 0 && i < a.trackPos

		duration := fmt.Sprintf("%d:%02d", t.Duration/60, t.Duration%60)
		num := fmt.Sprintf("%d", i+1)
		isSelected := a.selected[i]
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
			marker = selectionStripe.Render("▸")
			numSt, artSt, albSt, titSt, durSt = normalStyle.Bold(true), normalStyle.Bold(true), normalStyle.Bold(true), normalStyle.Bold(true), normalStyle.Bold(true)
		case played:
			marker = " "
			numSt, artSt, albSt, titSt, durSt = dimStyle, dimStyle, dimStyle, dimStyle, dimStyle
		default:
			marker = " "
			numSt, artSt, albSt, titSt, durSt = dimStyle, artistStyle, dimStyle, normalStyle, dimStyle
		}
		if isSelected {
			marker = titleStyle.Render("●")
		}

		var row string
		if tc.artist == 0 {
			// Ultra-narrow: title only
			row = marker + icons + col(t.Title, tc.title, titSt)
		} else {
			row = marker + icons +
				colRight(num, colNum, numSt) +
				col(t.Artist.Name, tc.artist, artSt) +
				col(t.Title, tc.title, titSt)
			if tc.showAlbum {
				row += col(t.Album.Title, tc.album, albSt)
			}
			if tc.showYear {
				row += colRight(trackYear(t), colYear, durSt)
			}
			row += colRight(duration, colDuration, durSt)
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
	s := trackHeader(tc) + "\n"

	end := a.likedScrollOffset + visibleRows
	if end > len(a.likes.Tracks) {
		end = len(a.likes.Tracks)
	}

	if a.likedScrollOffset > 0 {
		s += dimStyle.Render(fmt.Sprintf("  ^ %d more above", a.likedScrollOffset)) + "\n"
	}

	for i := a.likedScrollOffset; i < end; i++ {
		t := a.likes.Tracks[i]
		row := trackRow(i, t, i == a.likedCursor, true, a.dl != nil && a.dl.IsDownloaded(t), tc)
		if a.selected[t.ID] {
			// Replace leading space/cursor with selection marker
			row = titleStyle.Render("●") + row[1:]
		}
		s += row + "\n"
	}

	if end < len(a.likes.Tracks) {
		s += dimStyle.Render(fmt.Sprintf("  v %d more below", len(a.likes.Tracks)-end)) + "\n"
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
		failStyle := errorStyle
		s += failStyle.Render(fmt.Sprintf("  Failed: %d", st.Failed)) + "\n"
		if st.LastError != "" {
			s += failStyle.Render(fmt.Sprintf("  Last error: %s", st.LastError)) + "\n"
		}
	}

	return s
}

func (a App) View() string {
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

	spinner := spinnerFrames[a.spinnerIdx]
	var np string
	if a.loading {
		np = nowPlayingStyle.Render(dimStyle.Render("  " + spinner + " Loading stream..."))
	} else {
		np = a.nowPlaying.View(a.width)
	}
	tabBar := a.renderTabBar()

	errView := ""
	if a.err != nil {
		errView = "\n" + errorStyle.Render(fmt.Sprintf("  Error: %s", a.err))
	}

	statusLine := ""
	if a.statusMsg != "" {
		var statusStyle lipgloss.Style
		if a.statusTicks <= 1 {
			statusStyle = dimStyle
		} else {
			statusStyle = titleStyle
		}
		statusLine = "  " + statusStyle.Render(a.statusMsg) + "\n"
	}
	help := dimStyle.Render("  ? help  / search  enter play  a queue  p prev  n next  space pause  s stop  q quit")

	var top, bottom string

	switch {
	case a.mode == modeHelp:
		helpOverlay := overlayBorder.
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
		top = fmt.Sprintf("\n%s\n%s\n\n%s\n%s", header, tabBar, helpOverlay, dimStyle.Render("  esc to close"))

	case a.searchVisible():
		searchOverlay := overlayBorder.
			Padding(1, 2).
			Width(a.width - 6).
			Render(a.search.View(a.width-12, a.likes.IsLiked, a.dlCheck(), spinner))
		top = fmt.Sprintf("\n%s\n%s\n\n%s\n%s%s", header, tabBar, searchOverlay, dimStyle.Render("  esc to close"), errView)

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
		top = fmt.Sprintf("\n%s\n%s\n\n%s%s", header, tabBar, content, errView)
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

	bottom = statusLine + dlStatus + np + "\n" + help

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
