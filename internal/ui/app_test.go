package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/purplefish32/riff/internal/persistence"
	"github.com/purplefish32/riff/internal/types"
)

// testApp returns an App with minimal dependencies for testing.
// It uses a temp dir for persistence so tests don't touch real config.
func testApp(t *testing.T, tracks []types.Track) App {
	t.Helper()
	dir := t.TempDir()

	// Create required subdirs
	os.MkdirAll(filepath.Join(dir, "playlists"), 0o755)

	cfg := &persistence.Config{Quality: "LOSSLESS", Volume: 100}
	qs := persistence.NewQueueStoreAt(dir)
	qs.Tracks = tracks
	qs.Position = -1
	ps := persistence.NewPlaylistStoreAt(dir)

	return App{
		mode:          modeNormal,
		activeTab:     tabQueue,
		tracklist:     tracks,
		trackPos:      -1,
		queueCursor:   0,
		selected:      make(map[int]bool),
		online:        true,
		quality:       2, // LOSSLESS
		volume:        100,
		width:         120,
		height:        40,
		config:        cfg,
		queueStore:    qs,
		playlists:     ps,
		playlistNames: ps.List(),
		notifications: true,
		search:        newSearchModel(),
		nowPlaying:    newNowPlayingModel(),
		cmdInput:      newCmdInput(),
		filterInput:   newFilterInput(),
		saveInput:     newSaveInput(),
		renameInput:   newRenameInput(),
	}
}

func sampleTracks(n int) []types.Track {
	tracks := make([]types.Track, n)
	names := []string{"Bohemian Rhapsody", "Stairway to Heaven", "Hotel California", "Comfortably Numb", "Imagine"}
	artists := []string{"Queen", "Led Zeppelin", "Eagles", "Pink Floyd", "John Lennon"}
	for i := range tracks {
		tracks[i] = types.Track{
			ID:       i + 1,
			Title:    names[i%len(names)],
			Duration: 200 + i*30,
			Artist:   types.Artist{ID: i + 1, Name: artists[i%len(artists)]},
			Album:    types.Album{ID: i + 1, Title: "Greatest Hits", ReleaseDate: "1975-01-01"},
		}
	}
	return tracks
}

// --- visibleRows ---

func TestVisibleRows(t *testing.T) {
	tests := []struct {
		height int
		want   int
	}{
		{40, 28},
		{12, 1},  // exactly 12 → 0 clamped to 1
		{10, 1},  // below 12 → clamped to 1
		{0, 1},   // zero → clamped
		{100, 88},
	}
	for _, tt := range tests {
		a := App{height: tt.height}
		if got := a.visibleRows(); got != tt.want {
			t.Errorf("visibleRows(height=%d) = %d, want %d", tt.height, got, tt.want)
		}
	}
}

// --- withStatus ---

func TestWithStatus(t *testing.T) {
	a := App{}
	a = a.withStatus("hello")
	if a.statusMsg != "hello" {
		t.Errorf("statusMsg = %q, want %q", a.statusMsg, "hello")
	}
	if a.statusTicks != 3 {
		t.Errorf("statusTicks = %d, want 3", a.statusTicks)
	}
}

// --- withQueueAdd ---

func TestWithQueueAdd(t *testing.T) {
	a := testApp(t, nil)
	track := sampleTracks(1)[0]

	a = a.withQueueAdd(track)
	if len(a.tracklist) != 1 {
		t.Fatalf("tracklist len = %d, want 1", len(a.tracklist))
	}
	if a.tracklist[0].Title != track.Title {
		t.Errorf("tracklist[0].Title = %q, want %q", a.tracklist[0].Title, track.Title)
	}
	if a.statusMsg == "" {
		t.Error("expected status message after queue add")
	}
}

func TestWithQueueAddFull(t *testing.T) {
	tracks := sampleTracks(5)
	a := testApp(t, nil)
	// Fill to max
	for i := 0; i < maxTracklist; i++ {
		a.tracklist = append(a.tracklist, tracks[i%len(tracks)])
	}

	a = a.withQueueAdd(tracks[0])
	if len(a.tracklist) != maxTracklist {
		t.Errorf("tracklist should stay at %d, got %d", maxTracklist, len(a.tracklist))
	}
	if a.statusMsg != "Queue full (500 max)" {
		t.Errorf("statusMsg = %q, want queue full message", a.statusMsg)
	}
}

// --- withQueueAddAll ---

func TestWithQueueAddAll(t *testing.T) {
	a := testApp(t, sampleTracks(3))
	newTracks := sampleTracks(2)

	a = a.withQueueAddAll(newTracks)
	if len(a.tracklist) != 5 {
		t.Errorf("tracklist len = %d, want 5", len(a.tracklist))
	}
}

// --- switchTab ---

func TestSwitchTab(t *testing.T) {
	a := testApp(t, sampleTracks(3))
	a.activeTab = tabQueue

	a, ok := a.switchTab(tabRecent)
	if !ok {
		t.Error("switchTab returned false")
	}
	if a.activeTab != tabRecent {
		t.Errorf("activeTab = %d, want %d", a.activeTab, tabRecent)
	}
	if a.recentCursor != 0 {
		t.Error("recentCursor should reset to 0")
	}
}

func TestSwitchTabAutoSavesDirtyPlaylist(t *testing.T) {
	a := testApp(t, sampleTracks(3))
	a.activePlaylist = "test-playlist"
	a.playlistDirty = true

	a, _ = a.switchTab(tabRecent)
	if a.playlistDirty {
		t.Error("playlistDirty should be false after auto-save")
	}
}

// --- computeFilteredIndices ---

func TestComputeFilteredIndices(t *testing.T) {
	tracks := sampleTracks(5)
	a := testApp(t, tracks)
	a.activeTab = tabQueue

	// Filter for "queen" should match the Queen tracks
	a.filterText = "queen"
	a = a.computeFilteredIndices()
	if len(a.filteredIndices) == 0 {
		t.Fatal("expected at least one filtered match for 'queen'")
	}
	for _, idx := range a.filteredIndices {
		if tracks[idx].Artist.Name != "Queen" && tracks[idx].Title != "Queen" {
			t.Errorf("filtered track at %d doesn't match 'queen': artist=%q title=%q",
				idx, tracks[idx].Artist.Name, tracks[idx].Title)
		}
	}

	// Empty filter returns nil
	a.filterText = ""
	a = a.computeFilteredIndices()
	if a.filteredIndices != nil {
		t.Error("expected nil filteredIndices for empty filter")
	}
}

// --- markDirty ---

func TestMarkDirty(t *testing.T) {
	a := testApp(t, nil)

	// No active playlist → no dirty
	a = a.markDirty()
	if a.playlistDirty {
		t.Error("should not be dirty without active playlist")
	}

	// With active playlist → dirty
	a.activePlaylist = "test"
	a = a.markDirty()
	if !a.playlistDirty {
		t.Error("should be dirty with active playlist")
	}
}

// --- targetTrack ---

func TestTargetTrack(t *testing.T) {
	tracks := sampleTracks(3)
	a := testApp(t, tracks)
	a.queueCursor = 1

	track := a.targetTrack()
	if track == nil {
		t.Fatal("targetTrack returned nil")
	}
	if track.ID != tracks[1].ID {
		t.Errorf("targetTrack ID = %d, want %d", track.ID, tracks[1].ID)
	}
}

func TestTargetTrackEmptyQueue(t *testing.T) {
	a := testApp(t, nil)
	if a.targetTrack() != nil {
		t.Error("targetTrack should return nil for empty queue")
	}
}

// --- isFilterMatch ---

func TestIsFilterMatch(t *testing.T) {
	a := testApp(t, sampleTracks(5))

	// No filter → everything matches
	if !a.isFilterMatch(0) {
		t.Error("should match when no filter active")
	}

	// With filter
	a.filterText = "test"
	a.filteredIndices = []int{1, 3}
	if !a.isFilterMatch(1) {
		t.Error("index 1 should match")
	}
	if a.isFilterMatch(2) {
		t.Error("index 2 should not match")
	}
}
