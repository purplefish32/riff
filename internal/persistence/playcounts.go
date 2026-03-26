package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// PlayCountStore tracks how many times each track (by ID) has been played.
type PlayCountStore struct {
	path   string
	Counts map[int]int `json:"counts"`
}

// NewPlayCountStore loads play counts from ~/.config/riff/playcounts.json.
func NewPlayCountStore() *PlayCountStore {
	configDir, _ := os.UserConfigDir()
	dir := filepath.Join(configDir, "riff")
	os.MkdirAll(dir, 0o755)

	s := &PlayCountStore{
		path:   filepath.Join(dir, "playcounts.json"),
		Counts: make(map[int]int),
	}

	data, err := os.ReadFile(s.path)
	if err == nil {
		if err := json.Unmarshal(data, s); err != nil {
			fmt.Fprintf(os.Stderr, "warning: corrupt playcounts.json, starting fresh: %s\n", err)
		}
		if s.Counts == nil {
			s.Counts = make(map[int]int)
		}
	}

	return s
}

// Increment adds one play to the given track ID and saves.
func (s *PlayCountStore) Increment(trackID int) {
	s.Counts[trackID]++
	s.save()
}

// Get returns the play count for a track ID (0 if never played).
func (s *PlayCountStore) Get(trackID int) int {
	return s.Counts[trackID]
}

func (s *PlayCountStore) save() {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save play counts: %s\n", err)
	}
}
