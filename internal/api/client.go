package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/purplefish32/riff/internal/types"
)

var instances = []string{
	"https://api.monochrome.tf",
	"https://triton.squid.wtf",
	"https://wolf.qqdl.site",
	"https://arran.monochrome.tf",
}

type Client struct {
	instances  []string
	current    int
	mu         sync.Mutex
	httpClient *http.Client
}

func New() *Client {
	return &Client{
		instances: instances,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) baseURL() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.instances[c.current]
}

func (c *Client) failover() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = (c.current + 1) % len(c.instances)
}

func (c *Client) get(path string) (*http.Response, error) {
	var lastErr error
	for range len(c.instances) {
		u := c.baseURL() + path
		resp, err := c.httpClient.Get(u)
		if err != nil {
			lastErr = err
			c.failover()
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("status %d from %s", resp.StatusCode, u)
			c.failover()
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("all instances failed: %w", lastErr)
}

func (c *Client) SearchTracks(query string) ([]types.Track, error) {
	resp, err := c.get(fmt.Sprintf("/search/?s=%s", url.QueryEscape(query)))
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search returned status %d", resp.StatusCode)
	}

	var result types.SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding search response: %w", err)
	}

	return result.Data.Items, nil
}

func (c *Client) SearchArtists(query string) ([]types.ArtistFull, error) {
	resp, err := c.get(fmt.Sprintf("/search/?a=%s", url.QueryEscape(query)))
	if err != nil {
		return nil, fmt.Errorf("artist search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("artist search returned status %d", resp.StatusCode)
	}

	var result types.ArtistSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding artist search: %w", err)
	}

	return result.Data.Artists.Items, nil
}

func (c *Client) SearchAlbums(query string) ([]types.AlbumFull, error) {
	resp, err := c.get(fmt.Sprintf("/search/?al=%s", url.QueryEscape(query)))
	if err != nil {
		return nil, fmt.Errorf("album search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("album search returned status %d", resp.StatusCode)
	}

	var result types.AlbumSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding album search: %w", err)
	}

	return result.Data.Albums.Items, nil
}

func (c *Client) GetAlbumTracks(albumID int) ([]types.Track, error) {
	resp, err := c.get(fmt.Sprintf("/album/?id=%d", albumID))
	if err != nil {
		return nil, fmt.Errorf("album request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("album returned status %d", resp.StatusCode)
	}

	var result types.AlbumResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding album response: %w", err)
	}

	var tracks []types.Track
	for _, item := range result.Data.Items {
		if item.Type == "track" {
			t := item.Item
			if t.Album.ReleaseDate == "" {
				t.Album.ReleaseDate = result.Data.ReleaseDate
			}
			tracks = append(tracks, t)
		}
	}

	return tracks, nil
}

func (c *Client) GetStreamURL(trackID int, quality string) (string, error) {
	if quality == "" {
		quality = "LOSSLESS"
	}

	resp, err := c.get(fmt.Sprintf("/track/?id=%d&quality=%s", trackID, url.QueryEscape(quality)))
	if err != nil {
		return "", fmt.Errorf("stream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("stream returned status %d", resp.StatusCode)
	}

	var result types.StreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding stream response: %w", err)
	}

	manifestBytes, err := base64.StdEncoding.DecodeString(result.Data.Manifest)
	if err != nil {
		return "", fmt.Errorf("decoding manifest base64: %w", err)
	}

	var manifest types.StreamManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return "", fmt.Errorf("parsing manifest JSON: %w", err)
	}

	if len(manifest.URLs) == 0 {
		return "", fmt.Errorf("manifest contains no stream URLs")
	}

	return manifest.URLs[0], nil
}
