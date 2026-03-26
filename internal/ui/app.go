package ui

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/purplefish32/riff/internal/api"
	"github.com/purplefish32/riff/internal/downloader"
	"github.com/purplefish32/riff/internal/persistence"
	"github.com/purplefish32/riff/internal/player"
	"github.com/purplefish32/riff/internal/types"
)

type errMsg struct{ err error }

// FifoCommandMsg is sent when a command is received via the control FIFO.
type FifoCommandMsg struct{ Command string }

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

type playAlbumMsg struct {
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
	tabQueue     viewTab = iota
	tabRecent
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
	modeConfirmDelete                   // confirm playlist deletion
)

type App struct {
	mode             inputMode
	activeTab        viewTab
	search           searchModel
	nowPlaying       nowPlayingModel
	tracklist        []types.Track
	trackPos         int // index of currently playing track, -1 if none
	queueCursor      int // cursor for browsing the queue view
	queueScrollOffset int
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
	queueStore          *persistence.QueueStore
	recent              *persistence.RecentStore
	recentCursor        int
	recentScrollOffset  int
	playCounts          *persistence.PlayCountStore
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
	repeat           bool
	shuffle          bool
	shufflePlayed    map[int]bool // track indices played this shuffle pass
	shuffleHistory   []int        // indices in order actually played (for prev)
	notifications    bool
	audioInfo        string
	saveInput        textinput.Model
	saveTracks       []types.Track
	renameInput      textinput.Model
	renameFrom       string
	addToTrack       *types.Track
	addToPickerNames []string
	addToPickerIdx   int
	addToCreating    bool
	deleteTarget     string
}

func NewApp(client *api.Client, player *player.Player, likes *persistence.LikedStore, dl *downloader.Downloader, cfg *persistence.Config, qs *persistence.QueueStore, pc *persistence.PlayCountStore, ps *persistence.PlaylistStore, rs *persistence.RecentStore) App {
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
		recent:      rs,
		playCounts:  pc,
		playlists:       ps,
		playlistNames:   ps.List(),
		tracklist:   qs.Tracks,
		trackPos:    qs.Position,
		quality:         cfg.QualityIndex(),
		volume:          cfg.Volume,
		queueCursor:     queueCursor,
		selected:        make(map[int]bool),
		cmdInput:        newCmdInput(),
		online:          true,
		showLineNumbers: cfg.ShowLineNumbers,
		showPlayCounts:  cfg.ShowPlayCounts,
		showRemaining:   cfg.ShowRemaining,
		showAlbumArt:    cfg.ShowAlbumArt,
		notifications:   true,
		filterInput: newFilterInput(),
		saveInput:   newSaveInput(),
		renameInput: newRenameInput(),
	}
}

// tabAtX returns which tab a horizontal pixel position falls on,
// using the same label logic as renderTabBar for accurate hit targets.
func (a App) tabAtX(x int) (viewTab, bool) {
	tabs := []viewTab{tabQueue, tabRecent, tabPlaylists}
	labels := a.tabLabels()
	offset := 2 // leading "  " in renderTabBar
	for i, label := range labels {
		w := len(label) + 2 + 1 // " label " + "│"
		if x < offset+w {
			return tabs[i], true
		}
		offset += w
	}
	return tabPlaylists, true // click past last tab → last tab
}

// tabLabels returns the display labels for each tab, matching renderTabBar.
func (a App) tabLabels() []string {
	qLabel := "Queue"
	if len(a.tracklist) > 0 {
		if a.activePlaylist != "" {
			dirty := ""
			if a.playlistDirty {
				dirty = "*"
			}
			qLabel = fmt.Sprintf("Queue [%s%s](%d)", a.activePlaylist, dirty, len(a.tracklist))
		} else {
			qLabel = fmt.Sprintf("Queue(%d)", len(a.tracklist))
		}
	}
	rLabel := "Recent"
	if a.recent != nil {
		if n := len(a.recent.List()); n > 0 {
			rLabel = fmt.Sprintf("Recent(%d)", n)
		}
	}
	pLabel := "Playlists"
	if a.playlists != nil {
		if names := a.playlists.List(); len(names) > 0 {
			pLabel = fmt.Sprintf("Playlists(%d)", len(names))
		}
	}
	return []string{qLabel, rLabel, pLabel}
}

// visibleRows returns the number of content rows available for track lists.
func (a App) visibleRows() int {
	v := a.height - 12
	if v < 1 {
		return 1
	}
	return v
}

func newFilterInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "filter..."
	ti.Prompt = "Filter: "
	ti.CharLimit = 50
	ti.SetWidth(30)
	return ti
}

func newSaveInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "playlist name"
	ti.Prompt = "Save as: "
	ti.CharLimit = 30
	ti.SetWidth(25)
	return ti
}

func newRenameInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "new name"
	ti.Prompt = "Rename to: "
	ti.CharLimit = 30
	ti.SetWidth(25)
	return ti
}


func newCmdInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.Prompt = ":"
	ti.CharLimit = 50
	ti.SetWidth(40)
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
	// Add previous track to recent if played for at least 10 seconds
	if a.recent != nil && a.nowPlaying.track != nil && a.nowPlaying.position >= 10 {
		a.recent.Add(*a.nowPlaying.track)
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
	a.queueStore.SaveUIState(int(a.activeTab), a.queueCursor, 0)
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
		case tabRecent:
			if a.recent != nil {
				tracks := a.recent.List()
				if len(tracks) > 0 && a.recentCursor < len(tracks) {
					t := tracks[a.recentCursor]
					return &t
				}
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
	if a.recent != nil && a.nowPlaying.track != nil && a.nowPlaying.position >= 10 {
		a.recent.Add(*a.nowPlaying.track)
	}
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
	a.nowPlaying.repeat = a.repeat
	a.nowPlaying.shuffle = a.shuffle
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

// nextTrack returns the next track position to play.
// In shuffle mode, picks a random unplayed track. In normal mode, returns trackPos+1.
// Returns -1 if no next track is available.
func (a App) nextTrack() int {
	if !a.shuffle {
		if a.trackPos < len(a.tracklist)-1 {
			return a.trackPos + 1
		}
		return -1
	}
	// Shuffle mode: pick random unplayed track
	if a.shufflePlayed == nil {
		a.shufflePlayed = make(map[int]bool)
	}
	// Mark current as played
	if a.trackPos >= 0 {
		a.shufflePlayed[a.trackPos] = true
	}
	// Collect unplayed indices
	var unplayed []int
	for i := range a.tracklist {
		if !a.shufflePlayed[i] {
			unplayed = append(unplayed, i)
		}
	}
	if len(unplayed) > 0 {
		return unplayed[rand.Intn(len(unplayed))]
	}
	// All played — if repeat, reshuffle
	if a.repeat && len(a.tracklist) > 0 {
		a.shufflePlayed = make(map[int]bool)
		if a.trackPos >= 0 {
			a.shufflePlayed[a.trackPos] = true
		}
		// Rebuild unplayed after reset
		for i := range a.tracklist {
			if !a.shufflePlayed[i] {
				unplayed = append(unplayed, i)
			}
		}
		if len(unplayed) > 0 {
			return unplayed[rand.Intn(len(unplayed))]
		}
	}
	return -1
}

// playNext plays the next track (shuffle-aware) and records history.
func (a App) playNext() (App, tea.Cmd) {
	next := a.nextTrack()
	if next < 0 {
		if !a.shuffle && a.repeat && len(a.tracklist) > 0 {
			return a.playPos(0)
		}
		a.nowPlaying.track = nil
		a.nowPlaying.paused = false
		return a, nil
	}
	if a.shuffle {
		a.shuffleHistory = append(a.shuffleHistory, a.trackPos)
	}
	return a.playPos(next)
}

// playPrev plays the previous track (shuffle-aware using history).
func (a App) playPrev() (App, tea.Cmd) {
	if a.shuffle && len(a.shuffleHistory) > 0 {
		prev := a.shuffleHistory[len(a.shuffleHistory)-1]
		a.shuffleHistory = a.shuffleHistory[:len(a.shuffleHistory)-1]
		// Remove from played set so it can be picked again later
		delete(a.shufflePlayed, prev)
		return a.playPos(prev)
	}
	if a.trackPos > 0 {
		return a.playPos(a.trackPos - 1)
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

	case tea.MouseWheelMsg:
		if a.mode != modeNormal {
			return a, nil
		}
		if msg.Button == tea.MouseWheelUp {
			if a.activeTab == tabQueue && a.queueCursor > 0 {
				a.queueCursor--
				if a.queueCursor < a.queueScrollOffset {
					a.queueScrollOffset = a.queueCursor
				}
			}
			if a.activeTab == tabRecent && a.recentCursor > 0 {
				a.recentCursor--
				if a.recentCursor < a.recentScrollOffset {
					a.recentScrollOffset = a.recentCursor
				}
			}
		} else {
			if a.activeTab == tabQueue && a.queueCursor < len(a.tracklist)-1 {
				a.queueCursor++
				visibleRows := a.visibleRows()
				if a.queueCursor >= a.queueScrollOffset+visibleRows {
					a.queueScrollOffset = a.queueCursor - visibleRows + 1
				}
			}
			if a.activeTab == tabRecent && a.recent != nil {
				tracks := a.recent.List()
				if a.recentCursor < len(tracks)-1 {
					a.recentCursor++
					visibleRows := a.visibleRows()
					if a.recentCursor >= a.recentScrollOffset+visibleRows {
						a.recentScrollOffset = a.recentCursor - visibleRows + 1
					}
				}
			}
		}
		return a, nil

	case tea.MouseClickMsg:
		if a.mode != modeNormal {
			return a, nil
		}
		if msg.Button == tea.MouseLeft {
			m := msg.Mouse()
			// Tab bar is at row 2 (0-indexed)
			if m.Y == 2 {
				if target, ok := a.tabAtX(m.X); ok {
					a, ok = a.switchTab(target)
					if ok {
						a.saveUIState()
					}
				}
			} else if m.Y >= 5 {
				// Content area starts around row 5 (header row 4, then tracks)
				// Row 5 = track header, row 6 onwards = tracks
				contentRow := m.Y - 6
				if contentRow >= 0 {
					if a.activeTab == tabQueue {
						idx := a.queueScrollOffset + contentRow
						if idx >= 0 && idx < len(a.tracklist) {
							a.queueCursor = idx
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

	case tea.KeyPressMsg:
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
		case modeConfirmDelete:
			a, cmd := a.updateConfirmDelete(msg)
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
			a.err = fmt.Errorf("%s", friendlyError(msg.err))
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
			track := a.tracklist[a.trackPos]
			if coverID := track.Album.Cover; coverID != "" {
				cmds = append(cmds, a.fetchArt(coverID))
			}
			if a.notifications {
				go sendTrackNotification(track)
			}
		}
		return a, tea.Batch(cmds...)

	case trackEndedMsg:
		if msg.gen != a.playGen {
			return a, nil
		}
		// Increment play count and add to recent for the track that just finished
		if a.trackPos >= 0 && a.trackPos < len(a.tracklist) {
			track := a.tracklist[a.trackPos]
			if a.playCounts != nil {
				a.playCounts.Increment(track.ID)
			}
			if a.recent != nil && a.nowPlaying.position >= 10 {
				a.recent.Add(track)
			}
		}
		return a.playNext()

	case queueAlbumMsg:
		if msg.err != nil {
			if isNetworkError(msg.err) {
				a.online = false
			}
			a.err = fmt.Errorf("%s", friendlyError(msg.err))
			return a, nil
		}
		a.online = true
		a = a.withQueueAddAll(msg.tracks)
		return a, nil

	case playAlbumMsg:
		if msg.err != nil {
			if isNetworkError(msg.err) {
				a.online = false
			}
			a.err = fmt.Errorf("%s", friendlyError(msg.err))
			return a, nil
		}
		a.online = true
		if len(msg.tracks) == 0 {
			a = a.withStatus("Album has no tracks")
			return a, nil
		}
		a.tracklist = msg.tracks
		a.activePlaylist = ""
		a.playlistDirty = false
		a.queueCursor = 0
		a.queueScrollOffset = 0
		a.activeTab = tabQueue
		a.saveQueue()
		return a.playPos(0)

	case DownloadUpdateMsg:
		return a, nil

	case albumArtMsg:
		if a.nowPlaying.track != nil && a.nowPlaying.track.Album.Cover == msg.coverID {
			a.nowPlaying.albumArt = msg.art
		}
		return a, nil

	case errMsg:
		a.err = fmt.Errorf("%s", friendlyError(msg.err))
		return a, nil

	case FifoCommandMsg:
		return a.execCommand(msg.Command)
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
