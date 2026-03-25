package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Quality        string `json:"quality"`
	Volume         int    `json:"volume"`
	DownloadDir    string `json:"download_dir"`
	ShowLineNumbers bool  `json:"show_line_numbers"`
	ShowPlayCounts  bool  `json:"show_play_counts"`
	ShowRemaining   bool  `json:"show_remaining"`
	path           string
}

func LoadConfig() *Config {
	configDir, _ := os.UserConfigDir()
	dir := filepath.Join(configDir, "riff")
	os.MkdirAll(dir, 0o755)

	c := &Config{
		Quality:     "LOSSLESS",
		Volume:      100,
		DownloadDir: "",
		path:        filepath.Join(dir, "config.json"),
	}

	data, err := os.ReadFile(c.path)
	if err == nil {
		if err := json.Unmarshal(data, c); err != nil {
			fmt.Fprintf(os.Stderr, "warning: corrupt config.json, using defaults: %s\n", err)
		}
	}

	// Validate
	switch c.Quality {
	case "LOW", "HIGH", "LOSSLESS", "HI_RES":
	default:
		c.Quality = "LOSSLESS"
	}
	if c.Volume < 0 {
		c.Volume = 0
	}
	if c.Volume > 150 {
		c.Volume = 150
	}

	return c
}

func (c *Config) Save() {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(c.path, data, 0o644)
}

func (c *Config) QualityIndex() int {
	qualities := []string{"LOW", "HIGH", "LOSSLESS", "HI_RES"}
	for i, q := range qualities {
		if q == c.Quality {
			return i
		}
	}
	return 2
}
