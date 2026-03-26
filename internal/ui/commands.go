package ui

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

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
		a.shuffle = !a.shuffle
		if a.shuffle {
			a.shufflePlayed = make(map[int]bool)
			a.shuffleHistory = nil
			if a.trackPos >= 0 {
				a.shufflePlayed[a.trackPos] = true
			}
			return a.withStatus("Shuffle: on ⤮"), nil
		}
		a.shufflePlayed = nil
		a.shuffleHistory = nil
		return a.withStatus("Shuffle: off"), nil
	case "reorder":
		if len(a.tracklist) > 1 {
			rand.Shuffle(len(a.tracklist), func(i, j int) {
				a.tracklist[i], a.tracklist[j] = a.tracklist[j], a.tracklist[i]
			})
			a.trackPos = -1
			a.queueCursor = 0
			a.queueScrollOffset = 0
			a = a.markDirty()
			a.saveQueue()
			return a.withStatus("Queue reordered"), nil
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
		if len(args) == 0 {
			return a.withStatus("Usage: clear queue|history"), nil
		}
		switch args[0] {
		case "queue":
			a.tracklist = nil
			a.trackPos = -1
			a.queueCursor = 0
			a.queueScrollOffset = 0
			a.saveQueue()
			a.player.Stop()
			a.nowPlaying.track = nil
			a.activePlaylist = ""
			return a.withStatus("Queue cleared"), nil
		case "history":
			if a.recent != nil {
				a.recent.Tracks = nil
				a.recent.Save()
				a.recentCursor = 0
				a.recentScrollOffset = 0
			}
			return a.withStatus("History cleared"), nil
		default:
			return a.withStatus("Usage: clear queue|history"), nil
		}
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
					visibleRows := a.visibleRows()
					if a.queueCursor < a.queueScrollOffset {
						a.queueScrollOffset = a.queueCursor
					}
					if a.queueCursor >= a.queueScrollOffset+visibleRows {
						a.queueScrollOffset = a.queueCursor - visibleRows + 1
					}
				}
				return a.withStatus(fmt.Sprintf("Line %d", line)), nil
			}
		}
		return a.withStatus("Usage: goto <number>"), nil
	case "notifications":
		a.notifications = !a.notifications
		if a.notifications {
			return a.withStatus("Notifications: on"), nil
		}
		return a.withStatus("Notifications: off"), nil
	case "repeat":
		a.repeat = !a.repeat
		if a.repeat {
			return a.withStatus("Repeat: on ↻"), nil
		}
		return a.withStatus("Repeat: off"), nil
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
			case "recent", "2":
				target = tabRecent
			case "playlists", "3":
				target = tabPlaylists
			default:
				return a.withStatus("Usage: tab queue|recent|playlists"), nil
			}
			a, _ = a.switchTab(target)
			return a, nil
		}
		return a.withStatus("Usage: tab queue|recent|playlists"), nil
	case "help":
		a.mode = modeHelp
		return a, nil
	case "commands":
		return a.withStatus("q vol goto shuffle save load delete tab quality repeat lines playcounts art notifications seek next prev pause stop clear discard help"), nil
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
		if name == "liked" {
			return a.withStatus("Cannot delete liked playlist"), nil
		}
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
