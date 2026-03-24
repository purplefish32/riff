package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/purplefish32/spofree-cli/internal/api"
	"github.com/purplefish32/spofree-cli/internal/player"
	"github.com/purplefish32/spofree-cli/internal/ui"
)

func main() {
	client := api.New()

	p, err := player.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	defer p.Close()

	app := ui.NewApp(client, p)
	prog := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
