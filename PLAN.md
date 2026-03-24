# spofree-cli — Tidal TUI Player

## Overview

A Bubble Tea TUI that streams Tidal music via the public SpoFree/hifi-api, played through mpv.

## Architecture

```
spofree-cli/
├── main.go                 # Entry point, Bubble Tea program setup
├── internal/
│   ├── api/
│   │   └── client.go       # hifi-api client (search, track, album, playlist)
│   ├── player/
│   │   └── mpv.go          # mpv subprocess control (play, pause, stop, next)
│   ├── ui/
│   │   ├── app.go          # Root model, view routing
│   │   ├── search.go       # Search input + results list
│   │   ├── nowplaying.go   # Now playing bar (bottom)
│   │   └── styles.go       # Lip Gloss styles
│   └── types/
│       └── types.go        # Track, Album, Artist, Playlist structs
├── go.mod
├── go.sum
└── PLAN.md
```

## Dependencies

- `github.com/charmbracelet/bubbletea` — TUI framework
- `github.com/charmbracelet/lipgloss` — Styling
- `github.com/charmbracelet/bubbles` — Text input, list, spinner components
- `mpv` (external) — Audio playback via subprocess

## API Backend

- **Base URL:** `https://api.monochrome.tf`
- **No auth required**
- **Fallbacks:** `https://triton.squid.wtf`, `https://wolf.qqdl.site`, `https://arran.monochrome.tf`

### Endpoints Used

| Endpoint | Purpose |
|---|---|
| `GET /search/?s=<query>` | Search tracks |
| `GET /search/?a=<query>` | Search artists |
| `GET /search/?al=<query>` | Search albums |
| `GET /track/?id=<id>&quality=LOSSLESS` | Get stream manifest |
| `GET /album/?id=<id>` | Album tracklist |
| `GET /playlist/?id=<uuid>` | Playlist tracks |

### Stream Resolution

1. Call `/track/?id=<id>&quality=LOSSLESS`
2. Response: `{ data: { manifest: "<base64>" } }`
3. Base64 decode manifest → JSON: `{ urls: ["https://lgf.audio.tidal.com/..."] }`
4. Pass `urls[0]` to mpv

## Implementation Plan

### Phase 1: Core (MVP)

- [ ] **1.1 Types** — Define Track, Album, Artist, SearchResult structs matching API responses
- [ ] **1.2 API Client** — HTTP client with search and track manifest resolution (base64 decode)
- [ ] **1.3 mpv Player** — Spawn mpv subprocess with `--no-video --input-ipc-server=/tmp/spofree-mpv.sock`, control via JSON IPC (play, pause, stop, seek)
- [ ] **1.4 Search View** — Text input (bubbles) + results list, enter to play
- [ ] **1.5 Now Playing Bar** — Bottom bar showing current track, artist, play/pause state
- [ ] **1.6 Wire It Up** — Root model connecting search → API → player → now playing

### Phase 2: Polish

- [ ] **2.1 Instance Failover** — Rotate API instances on 429/5xx/timeout
- [ ] **2.2 Queue** — Add to queue, play next, queue view
- [ ] **2.3 Album/Playlist Browse** — Select album/playlist from search results, show tracklist
- [ ] **2.4 Quality Selection** — Toggle between LOW/HIGH/LOSSLESS/HI_RES
- [ ] **2.5 Progress Bar** — Track progress with seek support via mpv IPC
- [ ] **2.6 Keybindings Help** — `?` shows available shortcuts

### Phase 3: Nice to Have

- [ ] **3.1 Artist View** — Top tracks + discography
- [ ] **3.2 Lyrics** — Synced lyrics from `/lyrics/?id=<id>`
- [ ] **3.3 Cover Art** — Sixel/Kitty protocol album art in terminal
- [ ] **3.4 Recommendations** — Auto-play recommendations when queue ends
- [ ] **3.5 Config File** — Persist preferred quality, default instance, keybindings

## Keybindings (Target)

| Key | Action |
|---|---|
| `/` | Focus search |
| `Enter` | Play selected track |
| `Space` | Toggle pause |
| `n` | Next track |
| `q` | Quit |
| `+`/`-` | Volume up/down |
| `a` | Add to queue |
| `?` | Show help |

## Notes

- mpv is required as an external dependency (`brew install mpv`)
- The API returns FLAC streams for LOSSLESS quality — no DRM for CD quality
- HI_RES returns DASH manifests (mpv handles natively)
- CDN URLs are signed and expire in ~30 min — fetch fresh per play
