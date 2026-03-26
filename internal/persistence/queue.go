package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/purplefish32/riff/internal/types"
)

type QueueStore struct {
	Tracks       []types.Track `json:"tracks"`
	Position     int           `json:"position"`
	ActiveTab    int           `json:"active_tab"`
	QueueCursor  int           `json:"queue_cursor"`
	LikedCursor  int           `json:"liked_cursor"`
	path         string
}

// NewQueueStoreAt creates a QueueStore writing to the given directory.
func NewQueueStoreAt(dir string) *QueueStore {
	os.MkdirAll(dir, 0o755)
	return &QueueStore{
		Position: -1,
		path:     filepath.Join(dir, "queue.json"),
	}
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
	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save queue: %s\n", err)
	}
}

func (s *QueueStore) SaveUIState(activeTab, queueCursor, likedCursor int) {
	s.ActiveTab = activeTab
	s.QueueCursor = queueCursor
	s.LikedCursor = likedCursor

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save UI state: %s\n", err)
	}
}
