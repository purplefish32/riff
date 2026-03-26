package ui

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

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

	resp, err := http.Get(imgURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

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
