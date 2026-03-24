package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/purplefish32/spofree-cli/internal/types"
)

const defaultInstance = "https://api.monochrome.tf"

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New() *Client {
	return &Client{
		baseURL: defaultInstance,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) SearchTracks(query string) ([]types.Track, error) {
	u := fmt.Sprintf("%s/search/?s=%s", c.baseURL, url.QueryEscape(query))

	resp, err := c.httpClient.Get(u)
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

func (c *Client) GetStreamURL(trackID int, quality string) (string, error) {
	if quality == "" {
		quality = "LOSSLESS"
	}

	u := fmt.Sprintf("%s/track/?id=%d&quality=%s", c.baseURL, trackID, url.QueryEscape(quality))

	resp, err := c.httpClient.Get(u)
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

func (c *Client) SearchAlbums(query string) ([]types.AlbumFull, error) {
	u := fmt.Sprintf("%s/search/?al=%s", c.baseURL, url.QueryEscape(query))

	resp, err := c.httpClient.Get(u)
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
	u := fmt.Sprintf("%s/album/?id=%d", c.baseURL, albumID)

	resp, err := c.httpClient.Get(u)
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
			tracks = append(tracks, item.Item)
		}
	}

	return tracks, nil
}
