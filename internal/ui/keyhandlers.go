package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/purplefish32/riff/internal/types"
)

func (a App) updateHelp(msg tea.KeyPressMsg) (App, tea.Cmd) {
	switch msg.String() {
	case "esc", "?":
		a.mode = modeNormal
	}
	return a, nil
}

func (a App) updateSearchInput(msg tea.KeyPressMsg) (App, tea.Cmd) {
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

func (a App) updateSearchBrowse(msg tea.KeyPressMsg) (App, tea.Cmd) {
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
		a, ok := a.switchTab(tabRecent)
		if ok {
			a.mode = modeNormal
		}
		return a, nil
	case "3":
		a, ok := a.switchTab(tabPlaylists)
		if ok {
			a.mode = modeNormal
		}
		return a, nil
	case "tab", "]":
		tabs := []viewTab{tabQueue, tabRecent, tabPlaylists}
		next := tabs[(int(a.activeTab)+1)%len(tabs)]
		a, ok := a.switchTab(next)
		if ok {
			a.mode = modeNormal
		}
		return a, nil
	case "shift+tab", "[":
		tabs := []viewTab{tabQueue, tabRecent, tabPlaylists}
		prev := tabs[(int(a.activeTab)+len(tabs)-1)%len(tabs)]
		a, ok := a.switchTab(prev)
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
	case "space":
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

func (a App) updateNormal(msg tea.KeyPressMsg) (App, tea.Cmd) {
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
		a, ok := a.switchTab(tabRecent)
		if ok {
			a.saveUIState()
		}
		return a, nil
	case "3":
		a, ok := a.switchTab(tabPlaylists)
		if ok {
			a.saveUIState()
		}
		return a, nil
	case "tab", "]":
		tabs := []viewTab{tabQueue, tabRecent, tabPlaylists}
		next := tabs[(int(a.activeTab)+1)%len(tabs)]
		a, ok := a.switchTab(next)
		if ok {
			a.saveUIState()
		}
		return a, nil
	case "shift+tab", "[":
		tabs := []viewTab{tabQueue, tabRecent, tabPlaylists}
		prev := tabs[(int(a.activeTab)+len(tabs)-1)%len(tabs)]
		a, ok := a.switchTab(prev)
		if ok {
			a.saveUIState()
		}
		return a, nil
	case "up", "k":
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
		if a.activeTab == tabPlaylists && a.playlistCursor > 0 {
			a.playlistCursor--
		}
		return a, nil
	case "down", "j":
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
		if a.activeTab == tabPlaylists && a.playlistCursor < len(a.playlistNames)-1 {
			a.playlistCursor++
		}
		return a, nil
	case "enter":
		if a.activeTab == tabQueue && len(a.tracklist) > 0 {
			return a.playPos(a.queueCursor)
		}
		if a.activeTab == tabRecent && a.recent != nil {
			tracks := a.recent.List()
			if len(tracks) > 0 && a.recentCursor < len(tracks) {
				return a.addAndPlay(&tracks[a.recentCursor])
			}
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
			if name == "liked" {
				a = a.withStatus("Cannot delete liked playlist")
				return a, nil
			}
			a.deleteTarget = name
			a.mode = modeConfirmDelete
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
		if a.activeTab == tabRecent && a.recent != nil {
			tracks := a.recent.List()
			if len(tracks) > 0 && a.recentCursor < len(tracks) {
				a = a.withQueueAdd(tracks[a.recentCursor])
			}
			return a, nil
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
	case "space":
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
	case "R":
		a.repeat = !a.repeat
		if a.repeat {
			a = a.withStatus("Repeat: on ↻")
		} else {
			a = a.withStatus("Repeat: off")
		}
		return a, nil
	case "f":
		if a.activeTab == tabQueue {
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
			if a.activeTab == tabRecent {
				a.recentCursor = 0
				a.recentScrollOffset = 0
			}
			return a, nil
		}
		a.pendingG = true
		return a, nil
	case "ctrl+d":
		visibleRows := a.visibleRows()
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
		if a.activeTab == tabRecent && a.recent != nil {
			tracks := a.recent.List()
			if len(tracks) > 0 {
				a.recentCursor += halfPage
				if a.recentCursor >= len(tracks) {
					a.recentCursor = len(tracks) - 1
				}
				if a.recentCursor >= a.recentScrollOffset+visibleRows {
					a.recentScrollOffset = a.recentCursor - visibleRows + 1
				}
			}
		}
		return a, nil
	case "ctrl+u":
		visibleRows := a.visibleRows()
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
		if a.activeTab == tabRecent {
			a.recentCursor -= halfPage
			if a.recentCursor < 0 {
				a.recentCursor = 0
			}
			if a.recentCursor < a.recentScrollOffset {
				a.recentScrollOffset = a.recentCursor
			}
		}
		return a, nil
	case "G":
		visibleRows := a.visibleRows()
		if a.activeTab == tabQueue && len(a.tracklist) > 0 {
			a.queueCursor = len(a.tracklist) - 1
			if a.queueCursor >= a.queueScrollOffset+visibleRows {
				a.queueScrollOffset = a.queueCursor - visibleRows + 1
			}
		}
		if a.activeTab == tabRecent && a.recent != nil {
			tracks := a.recent.List()
			if len(tracks) > 0 {
				a.recentCursor = len(tracks) - 1
				if a.recentCursor >= a.recentScrollOffset+visibleRows {
					a.recentScrollOffset = a.recentCursor - visibleRows + 1
				}
			}
		}
		return a, nil
	case "home", "ctrl+a":
		if a.activeTab == tabQueue {
			a.queueCursor = 0
			a.queueScrollOffset = 0
		}
		if a.activeTab == tabRecent {
			a.recentCursor = 0
			a.recentScrollOffset = 0
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
			visibleRows := a.visibleRows()
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
			visibleRows := a.visibleRows()
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

func (a App) updateCommand(msg tea.KeyPressMsg) (App, tea.Cmd) {
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

func (a App) updateSavePlaylist(msg tea.KeyPressMsg) (App, tea.Cmd) {
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

func (a App) updateConfirmDelete(msg tea.KeyPressMsg) (App, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		a.playlists.Delete(a.deleteTarget)
		a = a.refreshPlaylists()
		if a.playlistCursor >= len(a.playlistNames) && a.playlistCursor > 0 {
			a.playlistCursor--
		}
		a = a.withStatus(fmt.Sprintf("Deleted: %s", a.deleteTarget))
		a.deleteTarget = ""
		a.mode = modeNormal
		return a, nil
	case "n", "esc":
		a.deleteTarget = ""
		a.mode = modeNormal
		return a, nil
	}
	return a, nil
}

func (a App) updateAddToPlaylist(msg tea.KeyPressMsg) (App, tea.Cmd) {
	// Creating new playlist: text input mode
	if a.addToCreating {
		switch msg.String() {
		case "esc":
			a.addToCreating = false
			a.saveInput.Blur()
			return a, nil
		case "enter":
			name := strings.TrimSpace(a.saveInput.Value())
			if name != "" && a.addToTrack != nil {
				a.playlists.Save(name, []types.Track{*a.addToTrack})
				a = a.refreshPlaylists()
				a = a.withStatus(fmt.Sprintf("Created %s with: %s", name, a.addToTrack.Title))
			}
			a.addToCreating = false
			a.addToTrack = nil
			a.saveInput.Blur()
			a.mode = modeNormal
			return a, nil
		}
		var cmd tea.Cmd
		a.saveInput, cmd = a.saveInput.Update(msg)
		return a, cmd
	}

	// Picker mode
	switch msg.String() {
	case "esc":
		a.mode = modeNormal
		a.addToTrack = nil
		return a, nil
	case "enter":
		if a.addToPickerIdx == 0 {
			// "+ New playlist" selected
			a.addToCreating = true
			a.saveInput.Reset()
			a.saveInput.Focus()
			return a, nil
		}
		if a.addToTrack != nil && a.addToPickerIdx-1 < len(a.addToPickerNames) {
			name := a.addToPickerNames[a.addToPickerIdx-1]
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
		if a.addToPickerIdx < len(a.addToPickerNames) {
			a.addToPickerIdx++
		}
		return a, nil
	}
	return a, nil
}

func (a App) updateRenamePlaylist(msg tea.KeyPressMsg) (App, tea.Cmd) {
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
	}
	return a
}

func (a App) updateFilter(msg tea.KeyPressMsg) (App, tea.Cmd) {
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
			for fi, idx := range a.filteredIndices {
				if idx == a.queueCursor {
					cursorIdx = fi
					break
				}
			}
			if cursorIdx > 0 {
				newIdx := a.filteredIndices[cursorIdx-1]
				a.queueCursor = newIdx
				if a.queueCursor < a.queueScrollOffset {
					a.queueScrollOffset = a.queueCursor
				}
			}
		}
		return a, nil
	case "down", "j":
		if len(a.filteredIndices) > 0 {
			cursorIdx := -1
			for fi, idx := range a.filteredIndices {
				if idx == a.queueCursor {
					cursorIdx = fi
					break
				}
			}
			nextFI := cursorIdx + 1
			if nextFI < len(a.filteredIndices) {
				newIdx := a.filteredIndices[nextFI]
				visibleRows := a.visibleRows()
				a.queueCursor = newIdx
				if a.queueCursor >= a.queueScrollOffset+visibleRows {
					a.queueScrollOffset = a.queueCursor - visibleRows + 1
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
		visibleRows := a.visibleRows()
		if a.activeTab == tabQueue {
			a.queueCursor = newIdx
			if a.queueCursor >= a.queueScrollOffset+visibleRows {
				a.queueScrollOffset = a.queueCursor - visibleRows + 1
			}
			if a.queueCursor < a.queueScrollOffset {
				a.queueScrollOffset = a.queueCursor
			}
		}
	}
	return a, cmd
}
