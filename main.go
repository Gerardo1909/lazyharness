package main

import (
	"fmt"
	"os"
	"time"

	"github.com/Gerardo1909/lazyharness/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const version = "0.1.0"

// tickMsg se emite cada segundo para actualizar el contador.
type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type model struct {
	width    int
	height   int
	ready    bool
	showHelp bool
	elapsed  int
}

func (m model) Init() tea.Cmd {
	// se retorna para hacer que el loop empiece
	return tickCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
	case tickMsg:
		m.elapsed++
		return m, tickCmd()
	}
	return m, nil
}

// View retorna el string que se renderiza en la terminal.
func (m model) View() string {
	if !m.ready {
		return "Cargando..."
	}

	title := tui.StyleTitle.Render(fmt.Sprintf("lazyharness v%s", version))

	subtitle := tui.StyleSubtitle.Render(
		"Tu harness de agentes,\ndirecto desde la terminal.",
	)

	help := tui.StyleHelp.Render("(presiona q para salir)\nEscribe '?' para comandos de ayuda.")

	if m.showHelp {
		help = tui.StyleHelp.Render(
			"Comandos:\n" +
				"  q: salir\n" +
				"  ?: mostrar/ocultar ayuda\n",
		)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		title,
		"",
		subtitle,
		"",
		help,
	)

	// Contenido principal centrado, dejando la última línea para el contador.
	mainArea := lipgloss.Place(
		m.width, m.height-1,
		lipgloss.Center, lipgloss.Center,
		content,
	)

	counter := tui.StyleHelp.Render(fmt.Sprintf("⏱ %ds", m.elapsed))
	footer := lipgloss.NewStyle().Width(m.width).Align(lipgloss.Right).Render(counter)

	return mainArea + "\n" + footer
}

func main() {
	p := tea.NewProgram(
		model{},
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
