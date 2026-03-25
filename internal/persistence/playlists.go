package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/purplefish32/riff/internal/types"
)

// PlaylistStore manages named playlists in ~/.config/riff/playlists/.
// Each playlist is a JSON file: playlists/<name>.json containing []Track.
type PlaylistStore struct {
	dir string
}

func NewPlaylistStore() *PlaylistStore {
	configDir, _ := os.UserConfigDir()
	dir := filepath.Join(configDir, "riff", "playlists")
	os.MkdirAll(dir, 0o755)
	return &PlaylistStore{dir: dir}
}

// sanitizeName cleans the name for use as a filename.
// Converts spaces to dashes, strips invalid characters.
func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ToLower(name)
	// Strip anything not alphanumeric, dash, or underscore
	var clean strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			clean.WriteRune(r)
		}
	}
	return clean.String()
}

func (s *PlaylistStore) Save(name string, tracks []types.Track) error {
	name = sanitizeName(name)
	if name == "" {
		return os.ErrInvalid
	}
	data, err := json.MarshalIndent(tracks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, name+".json"), data, 0o644)
}

func (s *PlaylistStore) Load(name string) ([]types.Track, error) {
	name = sanitizeName(name)
	if name == "" {
		return nil, os.ErrInvalid
	}
	data, err := os.ReadFile(filepath.Join(s.dir, name+".json"))
	if err != nil {
		return nil, err
	}
	var tracks []types.Track
	if err := json.Unmarshal(data, &tracks); err != nil {
		return nil, err
	}
	return tracks, nil
}

// List returns all saved playlist names (without .json extension).
func (s *PlaylistStore) List() []string {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return names
}

func (s *PlaylistStore) Delete(name string) error {
	name = sanitizeName(name)
	if name == "" {
		return os.ErrInvalid
	}
	return os.Remove(filepath.Join(s.dir, name+".json"))
}
