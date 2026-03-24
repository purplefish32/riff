package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/purplefish32/riff/internal/types"
)

type QueueStore struct {
	Tracks   []types.Track `json:"tracks"`
	Position int           `json:"position"`
	path     string
}

func NewQueueStore() *QueueStore {
	configDir, _ := os.UserConfigDir()
	dir := filepath.Join(configDir, "riff")
	os.MkdirAll(dir, 0o755)

	s := &QueueStore{
		Position: -1,
		path:     filepath.Join(dir, "queue.json"),
	}

	data, err := os.ReadFile(s.path)
	if err == nil {
		if err := json.Unmarshal(data, s); err != nil {
			fmt.Fprintf(os.Stderr, "warning: corrupt queue.json, starting fresh: %s\n", err)
		}
	}

	// Validate position
	if s.Position >= len(s.Tracks) {
		s.Position = len(s.Tracks) - 1
	}

	return s
}

func (s *QueueStore) Save(tracks []types.Track, position int) {
	s.Tracks = tracks
	s.Position = position

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(s.path, data, 0o644)
}
