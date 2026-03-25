package persistence

// RecentStore tracks recently played tracks in ~/.config/riff/recent.json
// Stores up to 100 tracks in reverse chronological order (most recent first).
// Deduplicates by track ID (if a track is played again, it moves to the top).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/purplefish32/riff/internal/types"
)

const maxRecent = 100

type RecentStore struct {
	path   string
	Tracks []types.Track `json:"tracks"`
}

func NewRecentStore() *RecentStore {
	configDir, _ := os.UserConfigDir()
	dir := filepath.Join(configDir, "riff")
	os.MkdirAll(dir, 0o755)

	s := &RecentStore{
		path: filepath.Join(dir, "recent.json"),
	}

	data, err := os.ReadFile(s.path)
	if err == nil {
		if err := json.Unmarshal(data, s); err != nil {
			fmt.Fprintf(os.Stderr, "warning: corrupt recent.json, starting fresh: %s\n", err)
			s.Tracks = nil
		}
	}

	return s
}

// Add prepends track to the front (most recent), deduplicates by ID, and
// caps the list at maxRecent before persisting.
func (s *RecentStore) Add(track types.Track) {
	// Remove any existing entry with the same ID
	filtered := s.Tracks[:0]
	for _, t := range s.Tracks {
		if t.ID != track.ID {
			filtered = append(filtered, t)
		}
	}
	// Prepend to front
	s.Tracks = append([]types.Track{track}, filtered...)
	// Cap at maxRecent
	if len(s.Tracks) > maxRecent {
		s.Tracks = s.Tracks[:maxRecent]
	}
	s.save()
}

// List returns the tracks slice (most recent first).
func (s *RecentStore) List() []types.Track {
	return s.Tracks
}

func (s *RecentStore) save() {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(s.path, data, 0o644)
}
