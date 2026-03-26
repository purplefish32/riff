package downloader

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/purplefish32/riff/internal/api"
	"github.com/purplefish32/riff/internal/types"
)

const maxConcurrent = 3

type Status struct {
	Active    int
	Completed int
	Failed    int
	Queued    int
	Current   string
	LastError string
}

type Downloader struct {
	client       *api.Client
	baseDir      string
	quality      string
	mu           sync.Mutex
	status       Status
	onUpdate     func()
	sem          chan struct{}
	log          *log.Logger
	downloaded   map[int]bool
	queued       map[int]bool
	failedTracks []types.Track
}

func New(client *api.Client, quality string, onUpdate func(), logger *log.Logger) *Downloader {
	home, _ := os.UserHomeDir()
	return &Downloader{
		client:     client,
		baseDir:    filepath.Join(home, "Music", "riff"),
		quality:    quality,
		onUpdate:   onUpdate,
		sem:        make(chan struct{}, maxConcurrent),
		log:        logger,
		downloaded: make(map[int]bool),
		queued:     make(map[int]bool),
	}
}

func (d *Downloader) SetBaseDir(dir string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.baseDir = dir
}

func (d *Downloader) Status() Status {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.status
}

func (d *Downloader) SetQuality(q string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.quality = q
}

func (d *Downloader) SetOnUpdate(fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onUpdate = fn
}

// QueueTrack enqueues a track for download. Returns false if already queued or downloaded.
func (d *Downloader) QueueTrack(track types.Track) bool {
	d.mu.Lock()
	if d.queued[track.ID] || d.downloaded[track.ID] {
		d.mu.Unlock()
		return false
	}
	d.queued[track.ID] = true
	d.status.Queued++
	d.mu.Unlock()
	go d.downloadTrack(track)
	return true
}

func (d *Downloader) QueueAlbum(tracks []types.Track) {
	for _, t := range tracks {
		track := t
		d.mu.Lock()
		if d.queued[track.ID] || d.downloaded[track.ID] {
			d.mu.Unlock()
			continue
		}
		d.queued[track.ID] = true
		d.status.Queued++
		d.mu.Unlock()
		go d.downloadTrack(track)
	}
}

var unsafeChars = regexp.MustCompile(`[<>:"/\\|?*]`)

func sanitize(s string) string {
	s = unsafeChars.ReplaceAllString(s, "_")
	return strings.TrimSpace(s)
}

func (d *Downloader) trackPath(track types.Track) string {
	artist := sanitize(track.Artist.Name)
	album := sanitize(track.Album.Title)
	title := sanitize(track.Title)
	ext := "flac"

	d.mu.Lock()
	q := d.quality
	d.mu.Unlock()

	if q == "LOW" || q == "HIGH" {
		ext = "m4a"
	}

	filename := fmt.Sprintf("%02d - %s.%s", track.TrackNumber, title, ext)
	return filepath.Join(d.baseDir, artist, album, filename)
}

func (d *Downloader) IsDownloaded(track types.Track) bool {
	d.mu.Lock()
	if d.downloaded[track.ID] {
		d.mu.Unlock()
		return true
	}
	d.mu.Unlock()

	path := d.trackPath(track)
	if _, err := os.Stat(path); err == nil {
		d.mu.Lock()
		d.downloaded[track.ID] = true
		d.mu.Unlock()
		return true
	}
	return false
}

func (d *Downloader) downloadTrack(track types.Track) {
	// Acquire semaphore slot
	d.sem <- struct{}{}
	defer func() { <-d.sem }()

	path := d.trackPath(track)

	// Skip if exists
	if _, err := os.Stat(path); err == nil {
		d.mu.Lock()
		d.status.Queued--
		d.status.Completed++
		d.downloaded[track.ID] = true
		delete(d.queued, track.ID)
		d.mu.Unlock()
		d.notify()
		return
	}

	d.mu.Lock()
	d.status.Queued--
	d.status.Active++
	d.status.Current = fmt.Sprintf("%s - %s", track.Artist.Name, track.Title)
	d.mu.Unlock()
	d.notify()

	d.mu.Lock()
	q := d.quality
	d.mu.Unlock()

	url, err := d.client.GetStreamURL(track.ID, q)
	if err != nil {
		d.log.Printf("download failed (stream URL): %s - %s: %s", track.Artist.Name, track.Title, err)
		d.mu.Lock()
		d.status.Active--
		d.status.Failed++
		d.status.LastError = fmt.Sprintf("%s: %s", track.Title, err)
		delete(d.queued, track.ID)
		d.failedTracks = append(d.failedTracks, track)
		d.mu.Unlock()
		d.notify()
		return
	}

	if err := d.downloadFile(url, path); err != nil {
		d.log.Printf("download failed (file write): %s - %s: %s", track.Artist.Name, track.Title, err)
		d.mu.Lock()
		d.status.Active--
		d.status.Failed++
		d.status.LastError = fmt.Sprintf("%s: %s", track.Title, err)
		delete(d.queued, track.ID)
		d.failedTracks = append(d.failedTracks, track)
		d.mu.Unlock()
		d.notify()
		return
	}

	d.mu.Lock()
	d.status.Active--
	d.status.Completed++
	d.downloaded[track.ID] = true
	delete(d.queued, track.ID)
	d.mu.Unlock()
	d.notify()
}

func (d *Downloader) downloadFile(url, path string) error {
	os.MkdirAll(filepath.Dir(path), 0o755)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	_, err = io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmp)
		return err
	}

	return os.Rename(tmp, path)
}

// RetryFailed re-queues all previously failed tracks and resets the failed count.
// Returns the number of tracks re-queued.
func (d *Downloader) RetryFailed() int {
	d.mu.Lock()
	tracks := make([]types.Track, len(d.failedTracks))
	copy(tracks, d.failedTracks)
	d.failedTracks = nil
	d.status.Failed = 0
	d.status.LastError = ""
	d.mu.Unlock()

	for _, t := range tracks {
		track := t
		d.mu.Lock()
		d.queued[track.ID] = true
		d.status.Queued++
		d.mu.Unlock()
		go d.downloadTrack(track)
	}
	return len(tracks)
}

// FailedCount returns the number of failed downloads.
func (d *Downloader) FailedCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.failedTracks)
}

func (d *Downloader) notify() {
	if d.onUpdate != nil {
		d.onUpdate()
	}
}
