package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/purplefish32/spofree-cli/internal/api"
	"github.com/purplefish32/spofree-cli/internal/player"
	"github.com/purplefish32/spofree-cli/internal/types"
)

type errMsg struct{ err error }

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

type App struct {
	search     searchModel
	nowPlaying nowPlayingModel
	queue      []types.Track
	history    []types.Track
	client     *api.Client
	player     *player.Player
	playGen    int
	quality    int
	showHelp   bool
	width      int
	height     int
	err        error
}

func NewApp(client *api.Client, player *player.Player) App {
	return App{
		search:  newSearchModel(),
		client:  client,
		player:  player,
		quality: 2, // LOSSLESS
	}
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (a App) Init() tea.Cmd {
	return tea.Batch(a.search.input.Focus(), tick())
}

func (a App) makeWaitForTrackEnd(gen int) tea.Cmd {
	return func() tea.Msg {
		a.player.WaitForEnd()
		return trackEndedMsg{gen: gen}
	}
}

func (a App) playTrack(track *types.Track) (App, tea.Cmd) {
	a.playGen++
	if a.nowPlaying.track != nil {
		a.history = append(a.history, *a.nowPlaying.track)
	}
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
		switch msg.String() {
		case "ctrl+c", "q":
			if !a.search.input.Focused() {
				return a, tea.Quit
			}
		case "enter":
			if a.search.input.Focused() && a.search.input.Value() != "" {
				a.search.loading = true
				query := a.search.input.Value()
				if a.search.mode == modeAlbum {
					return a, func() tea.Msg {
						albums, err := a.client.SearchAlbums(query)
						return albumSearchResultMsg{albums: albums, err: err}
					}
				}
				return a, func() tea.Msg {
					tracks, err := a.client.SearchTracks(query)
					return searchResultMsg{tracks: tracks, err: err}
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
				return a.playTrack(track)
			}
		case " ":
			if !a.search.input.Focused() && a.nowPlaying.track != nil {
				a.nowPlaying.paused = !a.nowPlaying.paused
				a.player.TogglePause()
				return a, nil
			}
		case "s":
			if !a.search.input.Focused() && a.nowPlaying.track != nil {
				a.playGen++
				a.player.Stop()
				a.nowPlaying.track = nil
				a.nowPlaying.paused = false
				a.nowPlaying.position = 0
				a.nowPlaying.duration = 0
				return a, nil
			}
		case "a":
			if !a.search.input.Focused() {
				// In album browse mode: queue selected track
				// In album search mode: fetch and queue all album tracks
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
					a.queue = append(a.queue, *track)
				}
				return a, nil
			}
		case "A":
			if !a.search.input.Focused() {
				if tracks := a.search.browsingAlbumTracks(); len(tracks) > 0 {
					a.queue = append(a.queue, tracks...)
				}
				return a, nil
			}
		case "n":
			if !a.search.input.Focused() {
				if len(a.queue) > 0 {
					next := a.queue[0]
					a.queue = a.queue[1:]
					return a.playTrack(&next)
				}
				return a, nil
			}
		case "p":
			if !a.search.input.Focused() {
				if len(a.history) > 0 {
					prev := a.history[len(a.history)-1]
					a.history = a.history[:len(a.history)-1]
					// Put current track back at front of queue
					if a.nowPlaying.track != nil {
						a.queue = append([]types.Track{*a.nowPlaying.track}, a.queue...)
					}
					a.playGen++
					a.nowPlaying.track = &prev
					a.nowPlaying.paused = false
					a.nowPlaying.position = 0
					a.nowPlaying.duration = 0
					a.err = nil
					trackID := prev.ID
					q := qualities[a.quality]
					return a, func() tea.Msg {
						url, err := a.client.GetStreamURL(trackID, q)
						return streamURLMsg{url: url, err: err}
					}
				}
				return a, nil
			}
		case "left":
			if !a.search.input.Focused() && a.nowPlaying.track != nil {
				a.player.Seek(-5)
				return a, nil
			}
		case "right":
			if !a.search.input.Focused() && a.nowPlaying.track != nil {
				a.player.Seek(5)
				return a, nil
			}
		case "Q":
			if !a.search.input.Focused() {
				a.quality = (a.quality + 1) % len(qualities)
				return a, nil
			}
		case "?":
			if !a.search.input.Focused() {
				a.showHelp = !a.showHelp
				return a, nil
			}
		case "esc":
			if a.showHelp {
				a.showHelp = false
				return a, nil
			}
			a.search.input.Blur()
			return a, nil
		case "/":
			if !a.search.input.Focused() {
				a.showHelp = false
				a.search.input.Focus()
				return a, nil
			}
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
		if len(a.queue) > 0 {
			next := a.queue[0]
			a.queue = a.queue[1:]
			return a.playTrack(&next)
		}
		a.nowPlaying.track = nil
		a.nowPlaying.paused = false
		return a, nil

	case queueAlbumMsg:
		if msg.err != nil {
			a.err = msg.err
			return a, nil
		}
		a.queue = append(a.queue, msg.tracks...)
		return a, nil

	case errMsg:
		a.err = msg.err
		return a, nil
	}

	var cmd tea.Cmd
	a.search, cmd = a.search.Update(msg)
	return a, cmd
}

func (a App) View() string {
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF6AC1")).
		Render("♫ spofree-cli")

	search := a.search.View(a.width)
	a.nowPlaying.quality = qualities[a.quality]
	np := a.nowPlaying.View(a.width)

	queueView := ""
	if len(a.queue) > 0 {
		queueView = "\n" + dimStyle.Render(fmt.Sprintf("  Queue: %d track", len(a.queue)))
		if len(a.queue) != 1 {
			queueView += dimStyle.Render("s")
		}
		limit := len(a.queue)
		if limit > 3 {
			limit = 3
		}
		for i, t := range a.queue[:limit] {
			queueView += "\n" + dimStyle.Render(fmt.Sprintf("    %d. %s — %s", i+1, t.Title, t.Artist.Name))
		}
		if len(a.queue) > 3 {
			queueView += "\n" + dimStyle.Render(fmt.Sprintf("    ... and %d more", len(a.queue)-3))
		}
	}

	errView := ""
	if a.err != nil {
		errView = "\n" + lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Render(fmt.Sprintf("  Error: %s", a.err))
	}

	if a.showHelp {
		helpOverlay := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF6AC1")).
			Padding(1, 2).
			Render(
				titleStyle.Render("Keybindings") + "\n\n" +
					helpLine("/", "Focus search") +
					helpLine("tab", "Toggle track/album search") +
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
					helpLine("left/right", "Seek -5s / +5s") +
					"\n" +
					helpLine("j/k", "Navigate up/down") +
					helpLine("Q", "Cycle quality") +
					helpLine("?", "Toggle this help") +
					helpLine("q", "Quit"),
			)

		return fmt.Sprintf("\n  %s\n\n%s\n%s\n", header, helpOverlay, np)
	}

	help := dimStyle.Render("  ? help  / search  enter play  a queue  p prev  n next  space pause  s stop  q quit")

	return fmt.Sprintf("\n  %s\n\n%s%s%s\n%s\n%s\n", header, search, queueView, errView, np, help)
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
