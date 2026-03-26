# riff

A Bubble Tea TUI that streams Tidal music via the Monochrome/hifi-api, played through mpv.

## Architecture

```
riff/
├── main.go                      # Entry point, flag parsing, signal handling, FIFO control
├── internal/
│   ├── api/
│   │   └── client.go            # HTTP client with instance failover (search, stream, albums, artists)
│   ├── player/
│   │   └── mpv.go               # mpv IPC socket control (play, pause, stop, seek, volume, position)
│   ├── ui/
│   │   ├── app.go               # Root model, struct, NewApp, Init, Update, shared helpers
│   │   ├── keyhandlers.go       # Key event handlers per input mode (normal, search, filter, etc.)
│   │   ├── commands.go          # Vim-style : command parser (execCommand)
│   │   ├── views.go             # View(), tab bar, queue/recent/playlist renderers, help overlay
│   │   ├── notifications.go     # System notifications, browser open, network error detection
│   │   ├── search.go            # Search popup (tracks/albums/artists), album browse
│   │   ├── nowplaying.go        # Now playing bar with progress
│   │   ├── albumart.go          # Album art fetching and sixel/block rendering
│   │   ├── styles.go            # Lip Gloss style definitions (with NO_COLOR support)
│   │   └── table.go             # Column truncation/formatting helpers
│   ├── downloader/
│   │   └── downloader.go        # Background download with 3-worker pool
│   ├── persistence/
│   │   ├── config.go            # Quality, volume, download dir, UI toggles (~/.config/riff/config.json)
│   │   ├── likes.go             # Liked tracks store (~/.config/riff/liked.json)
│   │   ├── queue.go             # Tracklist + position + UI state store (~/.config/riff/queue.json)
│   │   ├── playcounts.go        # Per-track play count store (~/.config/riff/playcounts.json)
│   │   ├── playlists.go         # Named playlist CRUD (~/.config/riff/playlists/*.json)
│   │   └── recent.go            # Recently played history (~/.config/riff/recent.json)
│   └── types/
│       └── types.go             # Track, Album, Artist, API response structs
├── go.mod
├── go.sum
└── README.md
```

## Tech Stack

- **Go 1.25+**
- **Bubble Tea v2** (`charm.land/bubbletea/v2`) — TUI framework
- **Lip Gloss v2** (`charm.land/lipgloss/v2`) — Styling
- **Bubbles v2** (`charm.land/bubbles/v2`) — Text input, progress bar components
- **mpv** — Audio playback via JSON IPC over Unix socket

## API

- Base: `https://api.monochrome.tf` (with failover to 3 backup instances)
- No auth required
- Stream resolution: `/track/?id=<id>&quality=LOSSLESS` → base64 manifest → FLAC URL → mpv

## Key Design Decisions

- **Value receivers everywhere** — bubbletea Model interface requires consistent receivers. All App methods use value receivers. State mutations return modified App copies.
- **Generation counter for skip** — prevents stale `end-file` events from auto-advancing the queue when user manually skips tracks.
- **Single reader goroutine for mpv IPC** — routes command responses via `map[requestID]chan` and events via dedicated channel. Prevents concurrent read conflicts.
- **Tracklist with position pointer** — Spotify-like queue where tracks stay in the list after playing. Position moves forward/backward.
- **Search as popup overlay** — search floats over any tab, dismissed with esc.
- **State machine for input modes** — single `inputMode` enum controls all key routing. Modes: `modeNormal`, `modeSearchInput`, `modeSearchBrowse`, `modeHelp`, `modeFilter`, `modeCommand`, `modeSavePlaylist`, `modeRenamePlaylist`, `modeAddToPlaylist`, `modeConfirmDelete`. Each mode has its own key handler method. Add new modes by adding a const + handler method + case in Update(). Key events use `tea.KeyPressMsg` (v2), mouse events use `tea.MouseClickMsg`/`tea.MouseWheelMsg`. Space key matches as `"space"` not `" "`.
- **Never call prog.Send() from Update()** — calling `prog.Send()` during the bubbletea Update cycle deadlocks because the event loop can't drain the message channel. The downloader's `notify()` callback must only be called from background goroutines, never synchronously from key handlers.

## UI Conventions

Follow these when modifying the TUI:

- **View() must be pure** — no state mutations in View(). Returns `tea.View` (v2) with `AltScreen` and `MouseMode` set declaratively. All state sync (nowPlaying quality/volume/liked) happens in the tick handler via `syncNowPlaying()`.
- **Shared styles only** — all colors defined in `styles.go`. No inline `lipgloss.NewStyle().Foreground(lipgloss.Color(...))`. Use named styles: `titleStyle`, `artistStyle`, `dimStyle`, `errorStyle`, `downloadIcon`, `overlayBorder`, `selectionStripe`, `activeTabStyle`, `altRowBg`. Use `bgColor` constant for whitespace backgrounds.
- **NO_COLOR support** — `styles.go` checks `NO_COLOR` env var. When set, styles use bold/reverse instead of color. All new styles must have a no-color variant in the `init()` function.
- **Responsive breakpoints** — `computeTrackCols(width)` adapts columns at 3 thresholds:
  - `< 40`: title only, no progress bar
  - `< 60`: artist + title, hide album/year
  - `< 90`: hide year column
  - `>= 90`: full layout
- **Compact header** — ASCII art banner at normal height, falls back to single-line `"riff"` when terminal height < 20.
- **Right-align numeric columns** — use `colRight()` for `#`, `Year`, `Time`, `Tracks`. Text columns use `col()` (left-aligned).
- **Progressive disclosure** — status line only renders when non-empty. Don't show UI elements with zero content.
- **Status feedback** — every user action (queue, like, download, quality change) shows a 3-tick status message via `withStatus()`. Messages fade: bright → dim → gone.
- **Spinner for async ops** — loading states use a block-element spinner (`▁▂▃▄▅▆▇█▇▆▅▄▃▂`) cycling on tick. Thread the current frame via `spinnerFrames[a.spinnerIdx]`.
- **Tick interval is 100ms** — for smooth spinner. Position/duration updates run every 10th tick. Status countdown runs every 10th tick. Use `a.tickCount % 10 == 0` guards.
- **Download cache** — `IsDownloaded()` checks an in-memory map before `os.Stat()`. Cache is populated on download completion and first filesystem check. Never call `os.Stat()` per track per render without caching.
- **Borders only on overlays** — search popup and help popup use `overlayBorder`. Tab content areas use whitespace separation, not borders.
- **Dim inactive panels** — when search/help overlay is open, render all tabs in dimStyle to show the tab bar is inactive.
- **Use `visibleRows()` helper** — for scroll calculations. Never inline `a.height - 12` with clamping; call `a.visibleRows()` instead.

## Dev Commands

```bash
go build -o riff .     # Build
go run .               # Run
go vet ./...           # Lint
./riff --version       # Version check
```

## Config Location

`~/.config/riff/` — config.json, liked.json, queue.json, playcounts.json, recent.json, playlists/, riff.log

## Downloads Location

`~/Music/riff/Artist/Album/01 - Title.flac`
