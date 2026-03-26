package ui

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/purplefish32/riff/internal/types"
)

// isNetworkError returns true if the error looks like a connectivity failure.
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "connection refused") ||
		strings.Contains(s, "no such host") ||
		strings.Contains(s, "i/o timeout") ||
		strings.Contains(s, "network") ||
		strings.Contains(s, "all instances failed")
}

// friendlyError returns a user-facing error message, sanitizing raw Go errors.
func friendlyError(err error) string {
	if err == nil {
		return ""
	}
	if isNetworkError(err) {
		return "Network error — check your connection"
	}
	s := err.Error()
	if strings.Contains(s, "context deadline exceeded") {
		return "Request timed out — try again"
	}
	return err.Error()
}

func sendTrackNotification(track types.Track) {
	title := track.Title
	artist := track.Artist.Name
	album := track.Album.Title

	switch runtime.GOOS {
	case "darwin":
		// Try terminal-notifier first (supports album art)
		if tn, err := exec.LookPath("terminal-notifier"); err == nil {
			args := []string{
				"-title", "riff",
				"-subtitle", artist,
				"-message", title + " — " + album,
				"-group", "riff",
			}
			// Download cover for the notification
			if cover := track.Album.Cover; cover != "" {
				coverPath := downloadCoverToTemp(cover)
				if coverPath != "" {
					args = append(args, "-contentImage", coverPath, "-appIcon", coverPath)
				}
			}
			exec.Command(tn, args...).Start()
			return
		}
		// Fallback to osascript
		script := fmt.Sprintf(`display notification "%s — %s" with title "riff" subtitle "%s"`,
			title, album, artist)
		exec.Command("osascript", "-e", script).Start()
	case "linux":
		exec.Command("notify-send", "riff", fmt.Sprintf("%s — %s\n%s", title, artist, album)).Start()
	}
}

func downloadCoverToTemp(coverID string) string {
	urlCover := strings.ReplaceAll(coverID, "-", "/")
	imgURL := fmt.Sprintf("https://resources.tidal.com/images/%s/320x320.jpg", urlCover)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imgURL, nil)
	if err != nil {
		return ""
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}

	tmp, err := os.CreateTemp("", "riff-cover-*.jpg")
	if err != nil {
		return ""
	}
	io.Copy(tmp, resp.Body)
	tmp.Close()
	return tmp.Name()
}

func openBrowser(url string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", url).Start()
	case "linux":
		exec.Command("xdg-open", url).Start()
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
}
