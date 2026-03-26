package ui

import (
	"encoding/json"
	"os"
)

const stateFilePath = "/tmp/riff-state.json"

type stateFileData struct {
	Playing  bool    `json:"playing"`
	Paused   bool    `json:"paused"`
	Title    string  `json:"title"`
	Artist   string  `json:"artist"`
	Album    string  `json:"album"`
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
	Quality  string  `json:"quality"`
	Volume   int     `json:"volume"`
	Shuffle  bool    `json:"shuffle"`
	Repeat   bool    `json:"repeat"`
	Radio    bool    `json:"radio"`
}

// writeStateFile writes the current playback state to /tmp/riff-state.json.
// Called every ~1 second from the tick handler. Errors are silently ignored
// since this is a best-effort feature for external tool integration.
func (a App) writeStateFile() {
	state := stateFileData{
		Quality: qualities[a.quality],
		Volume:  a.volume,
		Shuffle: false, // will be set by shuffle feature branch
		Repeat:  a.repeat,
		Radio:   false, // will be set by radio feature branch
	}

	if a.nowPlaying.track != nil {
		state.Playing = true
		state.Paused = a.nowPlaying.paused
		state.Title = a.nowPlaying.track.Title
		state.Artist = a.nowPlaying.track.Artist.Name
		state.Album = a.nowPlaying.track.Album.Title
		state.Position = a.nowPlaying.position
		state.Duration = a.nowPlaying.duration
	}

	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	// Write to temp file then rename for atomicity
	tmp := stateFilePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	os.Rename(tmp, stateFilePath)
}

// RemoveStateFile cleans up the state file on exit.
func RemoveStateFile() {
	os.Remove(stateFilePath)
	os.Remove(stateFilePath + ".tmp")
}
