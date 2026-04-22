// Command clisense is a terminal UI for managing a Typesense instance.
package main

import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/config"
	"clisense/internal/tui"
)

func main() {
	path, err := config.DefaultPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config path error:", err)
		os.Exit(1)
	}
	cfg, err := config.Load(path)
	inSetup := false
	switch {
	case err == nil:
		// ok
	case errors.Is(err, config.ErrNotFound), errors.Is(err, config.ErrCorrupt):
		inSetup = true
	default:
		fmt.Fprintln(os.Stderr, "config load error:", err)
		os.Exit(1)
	}

	app := tui.NewApp(cfg, path, inSetup)
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tui error:", err)
		os.Exit(1)
	}
}
