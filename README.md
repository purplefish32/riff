# riff

A terminal music player that streams lossless audio via the Monochrome/hifi-api, played through mpv.

Built with [Bubble Tea v2](https://charm.land/bubbletea).

## Requirements

- Go 1.25+
- [mpv](https://mpv.io/) (`brew install mpv`)

## Install

```bash
go install github.com/purplefish32/riff@latest
```

Or build from source:

```bash
git clone https://github.com/purplefish32/riff.git
cd riff
go build -o riff .
```

## Usage

```bash
riff
```

Press `/` to search, `enter` to play. Press `?` for all keybindings.

## Keybindings

| Key | Action |
|---|---|
| `/` | Open search |
| `tab` | Toggle track/album/artist search |
| `enter` | Play track / browse album / load playlist |
| `esc` | Close popup / cancel |
| `backspace` | Back from album tracklist |
| `space` | Toggle pause |
| `s` | Stop playback |
| `n` / `p` | Next / previous track |
| `a` | Add to queue / append playlist |
| `A` | Queue all album tracks |
| `x` | Remove from queue / delete playlist |
| `left` / `right` | Seek -5s / +5s |
| `+` / `-` | Volume up/down |
| `j` / `k` | Navigate up/down |
| `J` / `K` | Move queue track down/up |
| `gg` / `G` | Jump to first / last item |
| `ctrl+u` / `ctrl+d` | Page up / page down |
| `c` | Jump to now playing |
| `f` | Filter queue |
| `v` / `V` | Select track / select all |
| `l` | Toggle like |
| `d` / `D` | Download track / album |
| `u` | Open album in browser |
| `P` | Add track to playlist |
| `S` | Save queue as playlist |
| `R` | Toggle repeat |
| `Q` | Cycle quality (LOW/HIGH/LOSSLESS/HI_RES) |
| `t` | Toggle elapsed/remaining time |
| `:` | Command mode |
| `1`-`3` | Switch tabs (Queue/Recent/Playlists) |
| `?` | Help |
| `q` | Quit |

## Commands

Press `:` to enter command mode:

| Command | Action |
|---|---|
| `:q` | Quit |
| `:shuffle` | Shuffle queue |
| `:vol <n>` | Set volume |
| `:quality <level>` | Set quality |
| `:save <name>` | Save queue as playlist |
| `:load <name>` | Load playlist |
| `:delete <name>` | Delete playlist |
| `:goto <n>` | Jump to line |
| `:clear queue` | Clear queue |
| `:clear history` | Clear recent history |
| `:repeat` | Toggle repeat |
| `:notifications` | Toggle notifications |
| `:commands` | List all commands |
| `:help` | Show keybindings |

## Features

- Search tracks, albums, and artists
- Lossless streaming via mpv
- Queue with position tracking (Spotify-like)
- Playlists — save, load, rename, delete
- Recently played history
- Download tracks/albums to `~/Music/riff/`
- Like tracks (persisted locally, synced to playlists)
- Play count tracking
- Quality selection (LOW/HIGH/LOSSLESS/HI_RES)
- Repeat mode
- Inline queue filtering
- Multi-select with batch operations
- Album art in now-playing bar
- System notifications on track change (macOS/Linux)
- External control via FIFO (`/tmp/riff.fifo`)
- API failover across multiple instances
- Responsive UI with 4 breakpoints
- NO_COLOR support
- Persistent config, queue, likes, playlists, and play counts

## Config

Stored in `~/.config/riff/`:

- `config.json` — quality, volume, download directory, UI toggles
- `liked.json` — liked tracks
- `queue.json` — tracklist, position, UI state
- `playcounts.json` — per-track play counts
- `recent.json` — recently played history
- `playlists/` — saved playlists (one JSON file each)

## License

MIT
