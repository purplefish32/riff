package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/purplefish32/riff/internal/api"
	"github.com/purplefish32/riff/internal/downloader"
	"github.com/purplefish32/riff/internal/persistence"
	"github.com/purplefish32/riff/internal/player"
	"github.com/purplefish32/riff/internal/ui"
)

const version = "0.1.0"

func main() {
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()
	if *showVersion {
		fmt.Println("riff v" + version)
		return
	}

	cfg := persistence.LoadConfig()

	configDir, _ := os.UserConfigDir()
	logPath := filepath.Join(configDir, "riff", "riff.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot open log file: %s\n", err)
		logFile = os.Stderr
	}
	defer logFile.Close()
	logger := log.New(logFile, "", log.LstdFlags)

	client := api.New()

	p, err := player.New(logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	// Ensure mpv is cleaned up on any exit path
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		p.Close()
		os.Exit(0)
	}()
	defer p.Close()

	p.SetVolume(cfg.Volume)

	likes, err := persistence.NewLikedStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	dl := downloader.New(client, cfg.Quality, nil, logger)
	if cfg.DownloadDir != "" {
		dl.SetBaseDir(cfg.DownloadDir)
	}

	qs := persistence.NewQueueStore()
	pc := persistence.NewPlayCountStore()
	ps := persistence.NewPlaylistStore()
	rs := persistence.NewRecentStore()

	app := ui.NewApp(client, p, likes, dl, cfg, qs, pc, ps, rs)
	prog := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())

	dl.SetOnUpdate(func() {
		prog.Send(ui.DownloadUpdateMsg{})
	})

	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
