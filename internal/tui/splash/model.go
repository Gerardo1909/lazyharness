package splash

import (
	"github.com/Gerardo1909/lazyharness/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	width  int
	height int
}

func New(width, height int) Model { return Model{width, height} }
func (m Model) Init() tea.Cmd    { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			return m, func() tea.Msg { return tui.SplashDoneMsg{} }
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

const logo = `
  _                      _
 | | __ _ _____   _  __ | |__   __ _ _ __ _ __   ___  ___ ___
 | |/ _  |_  / | | |/ / | '_ \ / _  | '__| '_ \ / _ \/ __/ __|
 | | (_| |/ /| |_| / <  | | | | (_| | |  | | | |  __/\__ \__ \
 |_|\__,_/___|\__, \_\ |_| |_|\__,_|_|  |_| |_|\___||___/___/
              |___/
`

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	name := lipgloss.NewStyle().Foreground(tui.ColorBlue).Bold(true).Render(logo)
	sub := tui.StyleSubtitle.Render("multi-agent harness manager · v0.1.0")
	hint := tui.StyleHelp.Render("presioná enter para continuar")

	content := lipgloss.JoinVertical(lipgloss.Left, name, sub, "", hint)

	box := tui.StyleActiveBorder.Padding(1, 4).Render(content)

	return lipgloss.NewStyle().
		Width(m.width).Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(box)
}
