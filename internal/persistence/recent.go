package persistence

// RecentStore tracks play history in ~/.config/riff/recent.json
// Stores up to 500 entries in reverse chronological order (most recent first).
// No deduplication — every play is a separate entry.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/purplefish32/riff/internal/types"
)

const maxRecent = 500

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

// Add prepends track to the history and caps at maxRecent.
func (s *RecentStore) Add(track types.Track) {
	s.Tracks = append([]types.Track{track}, s.Tracks...)
	if len(s.Tracks) > maxRecent {
		s.Tracks = s.Tracks[:maxRecent]
	}
	s.Save()
}

// List returns the tracks slice (most recent first).
func (s *RecentStore) List() []types.Track {
	return s.Tracks
}

func (s *RecentStore) Save() {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save recent tracks: %s\n", err)
	}
}
