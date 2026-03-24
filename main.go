package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/purplefish32/spofree-cli/internal/api"
	"github.com/purplefish32/spofree-cli/internal/downloader"
	"github.com/purplefish32/spofree-cli/internal/persistence"
	"github.com/purplefish32/spofree-cli/internal/player"
	"github.com/purplefish32/spofree-cli/internal/ui"
)

func main() {
	cfg := persistence.LoadConfig()
	client := api.New()

	p, err := player.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	defer p.Close()

	// Set initial volume from config
	p.SetVolume(cfg.Volume)

	likes, err := persistence.NewLikedStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	dl := downloader.New(client, cfg.Quality, nil)
	if cfg.DownloadDir != "" {
		dl.SetBaseDir(cfg.DownloadDir)
	}

	app := ui.NewApp(client, p, likes, dl, cfg)
	prog := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
