package ui

import (
	"testing"

	"github.com/purplefish32/riff/internal/types"
)

func TestComputeTrackCols(t *testing.T) {
	tests := []struct {
		width     int
		wantAlbum bool
		wantYear  bool
	}{
		{30, false, false},  // ultra narrow
		{50, false, false},  // narrow
		{70, true, false},   // medium
		{100, true, true},   // full
	}
	for _, tt := range tests {
		tc := computeTrackCols(tt.width)
		if tc.showAlbum != tt.wantAlbum {
			t.Errorf("width=%d: showAlbum=%v, want %v", tt.width, tc.showAlbum, tt.wantAlbum)
		}
		if tc.showYear != tt.wantYear {
			t.Errorf("width=%d: showYear=%v, want %v", tt.width, tc.showYear, tt.wantYear)
		}
		// Title should always have some width
		if tc.title <= 0 {
			t.Errorf("width=%d: title width=%d, should be > 0", tt.width, tc.title)
		}
	}
}

func TestComputeTrackColsBreakpoints(t *testing.T) {
	// Below 40: no artist column
	tc := computeTrackCols(39)
	if tc.artist != 0 {
		t.Errorf("width=39: artist=%d, want 0 (ultra narrow)", tc.artist)
	}

	// At 40: should have artist
	tc = computeTrackCols(40)
	if tc.artist <= 0 {
		t.Errorf("width=40: artist=%d, should be > 0", tc.artist)
	}
}

func TestSearchModelSelectedTrack(t *testing.T) {
	m := newSearchModel()
	m.results = []types.Track{
		{ID: 1, Title: "Track A"},
		{ID: 2, Title: "Track B"},
		{ID: 3, Title: "Track C"},
	}
	m.cursor = 1

	track := m.selectedTrack()
	if track == nil {
		t.Fatal("selectedTrack returned nil")
	}
	if track.ID != 2 {
		t.Errorf("selectedTrack ID = %d, want 2", track.ID)
	}
}

func TestSearchModelSelectedTrackEmpty(t *testing.T) {
	m := newSearchModel()
	if m.selectedTrack() != nil {
		t.Error("selectedTrack should return nil for empty results")
	}
}

func TestSearchModelSelectedAlbum(t *testing.T) {
	m := newSearchModel()
	m.mode = modeAlbum
	m.albums = []types.AlbumFull{
		{ID: 10, Title: "Album A"},
		{ID: 20, Title: "Album B"},
	}
	m.cursor = 0

	album := m.selectedAlbum()
	if album == nil {
		t.Fatal("selectedAlbum returned nil")
	}
	if album.ID != 10 {
		t.Errorf("selectedAlbum ID = %d, want 10", album.ID)
	}
}

func TestSearchModelSelectedAlbumWrongMode(t *testing.T) {
	m := newSearchModel()
	m.mode = modeTrack
	m.albums = []types.AlbumFull{{ID: 1}}
	if m.selectedAlbum() != nil {
		t.Error("selectedAlbum should return nil in track mode")
	}
}

func TestSearchModelBrowsingAlbumTracks(t *testing.T) {
	m := newSearchModel()
	m.mode = modeBrowseAlbum
	m.albumTracks = []types.Track{{ID: 1}, {ID: 2}}

	tracks := m.browsingAlbumTracks()
	if len(tracks) != 2 {
		t.Errorf("browsingAlbumTracks len = %d, want 2", len(tracks))
	}
}

func TestSearchModelBrowsingAlbumTracksWrongMode(t *testing.T) {
	m := newSearchModel()
	m.mode = modeTrack
	m.albumTracks = []types.Track{{ID: 1}}
	if m.browsingAlbumTracks() != nil {
		t.Error("browsingAlbumTracks should return nil in track mode")
	}
}

func TestSearchModelListLen(t *testing.T) {
	m := newSearchModel()
	m.results = []types.Track{{}, {}, {}}
	m.albums = []types.AlbumFull{{}, {}}
	m.artists = []types.ArtistFull{{}}
	m.albumTracks = []types.Track{{}, {}, {}, {}}

	m.mode = modeTrack
	if m.listLen() != 3 {
		t.Errorf("modeTrack listLen = %d, want 3", m.listLen())
	}

	m.mode = modeAlbum
	if m.listLen() != 2 {
		t.Errorf("modeAlbum listLen = %d, want 2", m.listLen())
	}

	m.mode = modeArtist
	if m.listLen() != 1 {
		t.Errorf("modeArtist listLen = %d, want 1", m.listLen())
	}

	m.mode = modeBrowseAlbum
	if m.listLen() != 4 {
		t.Errorf("modeBrowseAlbum listLen = %d, want 4", m.listLen())
	}
}

func TestSearchHistory(t *testing.T) {
	m := newSearchModel()

	m.addToHistory("first")
	m.addToHistory("second")
	if len(m.searchHistory) != 2 {
		t.Errorf("history len = %d, want 2", len(m.searchHistory))
	}

	// Duplicate should move to end
	m.addToHistory("first")
	if len(m.searchHistory) != 2 {
		t.Errorf("history len = %d, want 2 after dedup", len(m.searchHistory))
	}
	if m.searchHistory[1] != "first" {
		t.Errorf("last history entry = %q, want 'first'", m.searchHistory[1])
	}

	// Empty string should be ignored
	m.addToHistory("")
	if len(m.searchHistory) != 2 {
		t.Error("empty string should not be added to history")
	}
}

func TestSearchHistoryMax(t *testing.T) {
	m := newSearchModel()
	for i := 0; i < maxSearchHistory+10; i++ {
		m.addToHistory(string(rune('A' + i)))
	}
	if len(m.searchHistory) != maxSearchHistory {
		t.Errorf("history len = %d, want %d (capped)", len(m.searchHistory), maxSearchHistory)
	}
}

func TestStatusIcons(t *testing.T) {
	// Neither liked nor downloaded
	icons := statusIcons(false, false)
	if icons != "  " {
		t.Errorf("no icons should be two spaces, got %q", icons)
	}
}
