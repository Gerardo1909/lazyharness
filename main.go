package main

import (
	"fmt"
	"os"

	"github.com/Gerardo1909/lazyharness/internal/app"
	tea "github.com/charmbracelet/bubbletea"
)

const version = "0.1.0"

func main() {
	p := tea.NewProgram(app.NewApp(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
