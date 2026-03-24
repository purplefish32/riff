package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/purplefish32/riff/internal/types"
)

type LikedStore struct {
	path   string
	Tracks []types.Track `json:"tracks"`
	ids    map[int]bool
}

func NewLikedStore() (*LikedStore, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(configDir, "riff")
	os.MkdirAll(dir, 0o755)

	s := &LikedStore{
		path: filepath.Join(dir, "liked.json"),
		ids:  make(map[int]bool),
	}

	data, err := os.ReadFile(s.path)
	if err == nil {
		json.Unmarshal(data, s)
	}

	for _, t := range s.Tracks {
		s.ids[t.ID] = true
	}

	return s, nil
}

func (s *LikedStore) IsLiked(trackID int) bool {
	return s.ids[trackID]
}

func (s *LikedStore) Toggle(track types.Track) bool {
	if s.ids[track.ID] {
		// Unlike
		delete(s.ids, track.ID)
		filtered := s.Tracks[:0]
		for _, t := range s.Tracks {
			if t.ID != track.ID {
				filtered = append(filtered, t)
			}
		}
		s.Tracks = filtered
		s.save()
		return false
	}

	// Like
	s.ids[track.ID] = true
	s.Tracks = append(s.Tracks, track)
	s.save()
	return true
}

func (s *LikedStore) save() {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(s.path, data, 0o644)
}
