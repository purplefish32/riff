# riff

A Bubble Tea TUI that streams Tidal music via the Monochrome/hifi-api, played through mpv.

## Architecture

```
riff/
├── main.go                      # Entry point, flag parsing, signal handling
├── internal/
│   ├── api/
│   │   └── client.go            # HTTP client with instance failover (search, stream, albums, artists)
│   ├── player/
│   │   └── mpv.go               # mpv IPC socket control (play, pause, stop, seek, volume, position)
│   ├── ui/
│   │   ├── app.go               # Root Bubble Tea model, tab navigation, key handling
│   │   ├── search.go            # Search popup (tracks/albums/artists), album browse
│   │   ├── nowplaying.go        # Now playing bar with progress
│   │   ├── styles.go            # Lip Gloss style definitions
│   │   └── table.go             # Column truncation/formatting helpers
│   ├── downloader/
│   │   └── downloader.go        # Background download with 3-worker pool
│   ├── persistence/
│   │   ├── config.go            # Quality, volume, download dir (~/.config/riff/config.json)
│   │   ├── likes.go             # Liked tracks store (~/.config/riff/liked.json)
│   │   └── queue.go             # Tracklist + position store (~/.config/riff/queue.json)
│   └── types/
│       └── types.go             # Track, Album, Artist, API response structs
├── go.mod
├── go.sum
└── README.md
```

## Tech Stack

- **Go 1.24+**
- **Bubble Tea** — TUI framework
- **Lip Gloss** — Styling
- **Bubbles** — Text input component
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

## Dev Commands

```bash
go build -o riff .     # Build
go run .               # Run
go vet ./...           # Lint
./riff --version       # Version check
```

## Config Location

`~/.config/riff/` — config.json, liked.json, queue.json

## Downloads Location

`~/Music/riff/Artist/Album/01 - Title.flac`
