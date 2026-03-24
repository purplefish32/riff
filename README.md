# riff

A terminal music player that streams lossless audio via the Monochrome/hifi-api, played through mpv.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Requirements

- Go 1.24+
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
| `enter` | Play track / browse album |
| `esc` | Close popup |
| `backspace` | Back from album tracklist |
| `space` | Toggle pause |
| `s` | Stop playback |
| `n` | Next track |
| `p` | Previous track |
| `a` | Add to queue |
| `A` | Queue all album tracks |
| `x` | Remove from queue |
| `left`/`right` | Seek -5s / +5s |
| `+`/`-` | Volume up/down |
| `j`/`k` | Navigate up/down |
| `d` | Download track |
| `D` | Download album |
| `l` | Toggle like |
| `u` | Open album in browser |
| `Q` | Cycle quality (LOW/HIGH/LOSSLESS/HI_RES) |
| `1`-`3` | Switch tabs (Queue/Liked/Downloads) |
| `?` | Help |
| `q` | Quit |

## Features

- Search tracks, albums, and artists
- Lossless streaming via mpv
- Queue with position tracking (Spotify-like)
- Download tracks/albums to `~/Music/riff/`
- Like tracks (persisted locally)
- Quality selection (LOW/HIGH/LOSSLESS/HI_RES)
- API failover across multiple instances
- Persistent config, queue, and likes
- Responsive table UI

## Config

Stored in `~/.config/riff/`:

- `config.json` — quality, volume, download directory
- `liked.json` — liked tracks
- `queue.json` — tracklist and position

## License

MIT
