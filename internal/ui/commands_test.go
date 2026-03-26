package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestExecCommandQuit(t *testing.T) {
	a := testApp(t, sampleTracks(3))
	_, cmd := a.execCommand("quit")
	if cmd == nil {
		t.Fatal("quit should return a cmd")
	}
	// tea.Quit is a function, verify it produces a QuitMsg
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("quit cmd should produce QuitMsg, got %T", msg)
	}
}

func TestExecCommandQuitAlias(t *testing.T) {
	a := testApp(t, sampleTracks(3))
	_, cmd := a.execCommand("q")
	if cmd == nil {
		t.Fatal("q should return a cmd")
	}
}

func TestExecCommandShuffleToggle(t *testing.T) {
	a := testApp(t, sampleTracks(5))

	a, _ = a.execCommand("shuffle")
	if !a.shuffle {
		t.Error("shuffle should be on after first toggle")
	}
	if a.shufflePlayed == nil {
		t.Error("shufflePlayed should be initialized")
	}
	if !strings.Contains(a.statusMsg, "on") {
		t.Errorf("statusMsg = %q, expected 'on'", a.statusMsg)
	}

	a, _ = a.execCommand("shuffle")
	if a.shuffle {
		t.Error("shuffle should be off after second toggle")
	}
	if !strings.Contains(a.statusMsg, "off") {
		t.Errorf("statusMsg = %q, expected 'off'", a.statusMsg)
	}
}

func TestExecCommandReorder(t *testing.T) {
	tracks := sampleTracks(5)
	a := testApp(t, tracks)

	a, _ = a.execCommand("reorder")
	if len(a.tracklist) != 5 {
		t.Errorf("tracklist len = %d, want 5", len(a.tracklist))
	}
	if !strings.Contains(a.statusMsg, "reordered") {
		t.Errorf("statusMsg = %q, expected 'reordered'", a.statusMsg)
	}
}

func TestExecCommandVolume(t *testing.T) {
	a := testApp(t, nil)
	a.volume = 80

	a, _ = a.execCommand("vol")
	if !strings.Contains(a.statusMsg, "80") {
		t.Errorf("vol without args should show current volume, got %q", a.statusMsg)
	}
}

func TestExecCommandGoto(t *testing.T) {
	a := testApp(t, sampleTracks(5))
	a.activeTab = tabQueue
	a.height = 40

	a, _ = a.execCommand("goto 3")
	if a.queueCursor != 2 {
		t.Errorf("queueCursor = %d, want 2 (goto 3 = index 2)", a.queueCursor)
	}
}

func TestExecCommandGotoBeyondEnd(t *testing.T) {
	a := testApp(t, sampleTracks(5))
	a.activeTab = tabQueue
	a.height = 40

	a, _ = a.execCommand("goto 999")
	if a.queueCursor != 4 {
		t.Errorf("queueCursor = %d, want 4 (clamped to last)", a.queueCursor)
	}
}

func TestExecCommandGotoNoArgs(t *testing.T) {
	a := testApp(t, sampleTracks(5))
	a, _ = a.execCommand("goto")
	if !strings.Contains(a.statusMsg, "Usage") {
		t.Errorf("goto without args should show usage, got %q", a.statusMsg)
	}
}

func TestExecCommandNumericShorthand(t *testing.T) {
	a := testApp(t, sampleTracks(5))
	a.activeTab = tabQueue
	a.height = 40

	a, _ = a.execCommand("4")
	if a.queueCursor != 3 {
		t.Errorf("queueCursor = %d, want 3 (numeric shorthand :4 = goto 4)", a.queueCursor)
	}
}

func TestExecCommandNotifications(t *testing.T) {
	a := testApp(t, nil)
	a.notifications = true

	a, _ = a.execCommand("notifications")
	if a.notifications {
		t.Error("notifications should toggle off")
	}
	if !strings.Contains(a.statusMsg, "off") {
		t.Errorf("statusMsg = %q, expected 'off'", a.statusMsg)
	}

	a, _ = a.execCommand("notifications")
	if !a.notifications {
		t.Error("notifications should toggle on")
	}
}

func TestExecCommandRepeat(t *testing.T) {
	a := testApp(t, nil)

	a, _ = a.execCommand("repeat")
	if !a.repeat {
		t.Error("repeat should toggle on")
	}

	a, _ = a.execCommand("repeat")
	if a.repeat {
		t.Error("repeat should toggle off")
	}
}

func TestExecCommandLines(t *testing.T) {
	a := testApp(t, nil)

	a, _ = a.execCommand("lines")
	if !a.showLineNumbers {
		t.Error("lines should toggle on")
	}

	a, _ = a.execCommand("lines")
	if a.showLineNumbers {
		t.Error("lines should toggle off")
	}
}

func TestExecCommandPlaycounts(t *testing.T) {
	a := testApp(t, nil)

	a, _ = a.execCommand("playcounts")
	if !a.showPlayCounts {
		t.Error("playcounts should toggle on")
	}
}

func TestExecCommandArt(t *testing.T) {
	a := testApp(t, nil)

	a, _ = a.execCommand("art")
	if !a.showAlbumArt {
		t.Error("art should toggle on")
	}

	a, _ = a.execCommand("art")
	if a.showAlbumArt {
		t.Error("art should toggle off")
	}
}

func TestExecCommandTab(t *testing.T) {
	a := testApp(t, nil)

	a, _ = a.execCommand("tab recent")
	if a.activeTab != tabRecent {
		t.Errorf("activeTab = %d, want tabRecent", a.activeTab)
	}

	a, _ = a.execCommand("tab playlists")
	if a.activeTab != tabPlaylists {
		t.Errorf("activeTab = %d, want tabPlaylists", a.activeTab)
	}

	a, _ = a.execCommand("tab queue")
	if a.activeTab != tabQueue {
		t.Errorf("activeTab = %d, want tabQueue", a.activeTab)
	}

	// Numeric aliases
	a, _ = a.execCommand("tab 2")
	if a.activeTab != tabRecent {
		t.Errorf("tab 2 should switch to recent, got %d", a.activeTab)
	}
}

func TestExecCommandTabInvalid(t *testing.T) {
	a := testApp(t, nil)
	a, _ = a.execCommand("tab invalid")
	if !strings.Contains(a.statusMsg, "Usage") {
		t.Errorf("invalid tab should show usage, got %q", a.statusMsg)
	}
}

func TestExecCommandTabNoArgs(t *testing.T) {
	a := testApp(t, nil)
	a, _ = a.execCommand("tab")
	if !strings.Contains(a.statusMsg, "Usage") {
		t.Errorf("tab without args should show usage, got %q", a.statusMsg)
	}
}

func TestExecCommandHelp(t *testing.T) {
	a := testApp(t, nil)
	a, _ = a.execCommand("help")
	if a.mode != modeHelp {
		t.Errorf("mode = %d, want modeHelp", a.mode)
	}
}

func TestExecCommandWrite(t *testing.T) {
	a := testApp(t, sampleTracks(3))
	a, _ = a.execCommand("w")
	if !strings.Contains(a.statusMsg, "saved") {
		t.Errorf("statusMsg = %q, expected 'saved'", a.statusMsg)
	}
}

func TestExecCommandClearNoArgs(t *testing.T) {
	a := testApp(t, nil)
	a, _ = a.execCommand("clear")
	if !strings.Contains(a.statusMsg, "Usage") {
		t.Errorf("clear without args should show usage, got %q", a.statusMsg)
	}
}

func TestExecCommandClearInvalid(t *testing.T) {
	a := testApp(t, nil)
	a, _ = a.execCommand("clear foo")
	if !strings.Contains(a.statusMsg, "Usage") {
		t.Errorf("clear foo should show usage, got %q", a.statusMsg)
	}
}

func TestExecCommandQualitySet(t *testing.T) {
	a := testApp(t, nil)
	a, _ = a.execCommand("quality high")
	if a.quality != 1 {
		t.Errorf("quality = %d, want 1 (HIGH)", a.quality)
	}
	if !strings.Contains(a.statusMsg, "HIGH") {
		t.Errorf("statusMsg = %q, expected HIGH", a.statusMsg)
	}
}

func TestExecCommandQualityCycle(t *testing.T) {
	a := testApp(t, nil)
	a.quality = 2 // LOSSLESS

	a, _ = a.execCommand("quality")
	if a.quality != 3 {
		t.Errorf("quality = %d, want 3 (HI_RES after cycle)", a.quality)
	}

	a, _ = a.execCommand("quality")
	if a.quality != 0 {
		t.Errorf("quality = %d, want 0 (LOW after wrap)", a.quality)
	}
}

func TestExecCommandQualityInvalid(t *testing.T) {
	a := testApp(t, nil)
	a, _ = a.execCommand("quality garbage")
	if !strings.Contains(a.statusMsg, "LOW | HIGH") {
		t.Errorf("invalid quality should show options, got %q", a.statusMsg)
	}
}

func TestExecCommandUnknown(t *testing.T) {
	a := testApp(t, nil)
	a, _ = a.execCommand("nonexistent")
	if !strings.Contains(a.statusMsg, "Unknown command") {
		t.Errorf("statusMsg = %q, expected unknown command", a.statusMsg)
	}
}

func TestExecCommandEmpty(t *testing.T) {
	a := testApp(t, nil)
	a, cmd := a.execCommand("")
	if cmd != nil {
		t.Error("empty command should return nil cmd")
	}
	if a.statusMsg != "" {
		t.Errorf("empty command should not set status, got %q", a.statusMsg)
	}
}

func TestExecCommandSavePlaylist(t *testing.T) {
	tracks := sampleTracks(3)
	a := testApp(t, tracks)

	a, _ = a.execCommand("save test-list")
	if !strings.Contains(a.statusMsg, "Saved") {
		t.Errorf("statusMsg = %q, expected 'Saved'", a.statusMsg)
	}

	// Verify it was actually saved
	loaded, err := a.playlists.Load("test-list")
	if err != nil {
		t.Fatalf("failed to load saved playlist: %v", err)
	}
	if len(loaded) != 3 {
		t.Errorf("loaded playlist has %d tracks, want 3", len(loaded))
	}
}

func TestExecCommandLoadPlaylist(t *testing.T) {
	tracks := sampleTracks(3)
	a := testApp(t, tracks)

	// Save first
	a.playlists.Save("my-list", tracks)

	// Load into empty queue
	a.tracklist = nil
	a, _ = a.execCommand("load my-list")
	if len(a.tracklist) != 3 {
		t.Errorf("tracklist len = %d, want 3 after load", len(a.tracklist))
	}
	if a.activePlaylist != "my-list" {
		t.Errorf("activePlaylist = %q, want 'my-list'", a.activePlaylist)
	}
}

func TestExecCommandDeletePlaylist(t *testing.T) {
	a := testApp(t, sampleTracks(3))
	a.playlists.Save("to-delete", sampleTracks(2))

	a, _ = a.execCommand("delete to-delete")
	if !strings.Contains(a.statusMsg, "Deleted") {
		t.Errorf("statusMsg = %q, expected 'Deleted'", a.statusMsg)
	}

	// Verify it's gone
	_, err := a.playlists.Load("to-delete")
	if err == nil {
		t.Error("playlist should be deleted")
	}
}

func TestExecCommandDeleteLiked(t *testing.T) {
	a := testApp(t, nil)
	a, _ = a.execCommand("delete liked")
	if !strings.Contains(a.statusMsg, "Cannot delete") {
		t.Errorf("statusMsg = %q, expected cannot delete", a.statusMsg)
	}
}

func TestExecCommandDiscard(t *testing.T) {
	a := testApp(t, sampleTracks(3))
	a.activePlaylist = "test"
	a.playlistDirty = true

	a, _ = a.execCommand("discard")
	if a.playlistDirty {
		t.Error("playlistDirty should be false after discard")
	}
}
