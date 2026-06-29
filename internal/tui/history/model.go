package history

import (
	"fmt"
	"strings"
	"time"

	"github.com/Gerardo1909/lazyharness/internal/domain"
	"github.com/Gerardo1909/lazyharness/internal/storage"
	"github.com/Gerardo1909/lazyharness/internal/tui"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type historyState int

const (
	stateBrowsing historyState = iota
	stateConfirm               // confirmando restauración
)

// Model muestra el historial de commits de un rol y permite hacer rollback.
type Model struct {
	harness     domain.Harness
	roleName    string
	commits     []storage.CommitEntry
	selectedIdx int
	diffVP      viewport.Model
	state       historyState
	commitInput textinput.Model
	errMsg      string
	width       int
	height      int
}

func (m *Model) loadDiff() {
	if len(m.commits) == 0 {
		m.diffVP.SetContent(tui.StyleSubtitle.Render("(sin historial de commits)"))
		return
	}
	current := ""
	selected := m.commits[m.selectedIdx].Hash
	if m.selectedIdx > 0 {
		current = m.commits[0].Hash
	}
	diff, err := storage.DiffBetweenCommits(m.harness.ProjectDir, m.roleName, current, selected)
	if err != nil {
		m.diffVP.SetContent(lipgloss.NewStyle().Foreground(tui.ColorRed).Render(
			fmt.Sprintf("Error cargando diff: %v", err),
		))
		return
	}
	m.diffVP.SetContent(diff)
	m.diffVP.GotoTop()
}

func New(h domain.Harness, roleName string, width, height int) Model {
	role, _ := h.FindRoleByName(roleName)
	commits, _ := storage.LogForFile(h.ProjectDir, role.PromptFile)

	_, dvH := rightDims(width, height)
	vp := viewport.New(rightWidth(width)-4, dvH)

	ci := textinput.New()
	ci.Placeholder = "mensaje del commit de restauración…"
	ci.Width = rightWidth(width) - 6

	m := Model{
		harness:     h,
		roleName:    roleName,
		commits:     commits,
		diffVP:      vp,
		commitInput: ci,
		width:       width,
		height:      height,
	}
	m.loadDiff()
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		_, dvH := rightDims(m.width, m.height)
		m.diffVP.Width = rightWidth(m.width) - 4
		m.diffVP.Height = dvH
		m.loadDiff()

	case tea.KeyMsg:
		switch m.state {
		case stateConfirm:
			return m.updateConfirm(msg)
		default:
			return m.updateBrowse(msg)
		}
	}

	var cmd tea.Cmd
	m.diffVP, cmd = m.diffVP.Update(msg)
	return m, cmd
}

func (m Model) updateBrowse(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.selectedIdx > 0 {
			m.selectedIdx--
			m.loadDiff()
		}
	case "down", "j":
		if m.selectedIdx < len(m.commits)-1 {
			m.selectedIdx++
			m.loadDiff()
		}
	case "R":
		if len(m.commits) == 0 {
			return m, nil
		}
		m.state = stateConfirm
		m.commitInput.Focus()
		return m, textinput.Blink
	case "esc", "q":
		if msg.String() == "q" {
			return m, tea.Quit
		}
		return m, func() tea.Msg { return tui.GoBackMsg{} }
	case "ctrl+c":
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.diffVP, cmd = m.diffVP.Update(msg)
	return m, cmd
}

func (m Model) updateConfirm(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		commitMsg := strings.TrimSpace(m.commitInput.Value())
		if commitMsg == "" {
			return m, nil
		}
		if m.selectedIdx < len(m.commits) {
			role, _ := m.harness.FindRoleByName(m.roleName)
			if err := storage.RollbackFile(
				m.harness.ProjectDir,
				role.PromptFile,
				m.commits[m.selectedIdx].Hash,
				commitMsg,
			); err != nil {
				m.errMsg = fmt.Sprintf("Error en el rollback: %v", err)
				m.state = stateBrowsing
				m.commitInput.SetValue("")
				return m, nil
			}
		}
		return m, func() tea.Msg { return tui.GoBackMsg{} }
	case "esc":
		m.state = stateBrowsing
		m.commitInput.SetValue("")
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.commitInput, cmd = m.commitInput.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	header := renderHeader(m.harness, m.roleName)
	lw := leftWidth(m.width)
	rw := rightWidth(m.width)

	commitsPanel := tui.StyleActiveBorder.
		Width(lw).Height(m.height - 4).
		Render(m.renderCommitList())

	diffContent := m.diffVP.View()
	if m.state == stateConfirm {
		diffContent = m.renderWithConfirmDialog(diffContent, rw)
	}

	diffPanel := tui.StyleBorder.
		Width(rw).Height(actionsTop(m.height)).
		Render(renderDiffHeader(m.commits, m.selectedIdx) + "\n" + diffContent)

	actionsPanel := tui.StyleBorder.
		Width(rw).Height(actionsHeight(m.height)).
		Render(m.renderActions())

	rightCol := lipgloss.JoinVertical(lipgloss.Left, diffPanel, actionsPanel)
	body := lipgloss.JoinHorizontal(lipgloss.Top, commitsPanel, rightCol)
	keybar := renderKeybar(m.width)

	rows := []string{header, body}
	if m.errMsg != "" {
		errBar := lipgloss.NewStyle().
			Foreground(tui.ColorRed).Background(tui.ColorPanelBg).
			Width(m.width).Padding(0, 1).
			Render("✗ " + m.errMsg)
		rows = append(rows, errBar)
	}
	rows = append(rows, keybar)
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m Model) renderCommitList() string {
	var b strings.Builder
	role, _ := m.harness.FindRoleByName(m.roleName)
	title := fmt.Sprintf("Versiones · %s", role.PromptFile)
	b.WriteString(lipgloss.NewStyle().Foreground(tui.ColorBlue).Bold(true).Render(title) + "\n\n")

	for i, c := range m.commits {
		hashStyle := lipgloss.NewStyle().Foreground(tui.ColorGreen)
		msgStyle := lipgloss.NewStyle().Foreground(tui.ColorFg)
		metaStyle := tui.StyleSubtitle

		if i == m.selectedIdx {
			msgStyle = msgStyle.Background(tui.ColorSelection)
		}

		label := ""
		if i == 0 {
			label = " (actual)"
		}
		version := fmt.Sprintf("v%d", len(m.commits)-i)

		b.WriteString(hashStyle.Render(shortHash(c.Hash)) + " " + msgStyle.Render(c.Message) + "\n")
		b.WriteString(metaStyle.Render(fmt.Sprintf("  %s · %s%s\n\n",
			relativeTime(c.Timestamp), version, label)))
	}
	return b.String()
}

func renderDiffHeader(commits []storage.CommitEntry, idx int) string {
	if len(commits) == 0 {
		return tui.StyleSubtitle.Render("(sin historial)")
	}
	current := "actual"
	if idx > 0 && idx < len(commits) {
		current = fmt.Sprintf("v%d", len(commits)-idx)
	}
	return tui.StyleSubtitle.Render(fmt.Sprintf("┤ Diff · actual vs seleccionada (%s) ├", current))
}

func (m Model) renderWithConfirmDialog(diffContent string, width int) string {
	selected := m.commits[m.selectedIdx]
	version := fmt.Sprintf("v%d", len(m.commits)-m.selectedIdx)

	dialog := tui.StyleBorder.
		BorderForeground(tui.ColorYellow).
		Width(width - 8).Padding(0, 1).
		Render(
			lipgloss.NewStyle().Foreground(tui.ColorYellow).Render("Confirmar restauración") + "\n\n" +
				lipgloss.NewStyle().Foreground(tui.ColorFg).Render(
					fmt.Sprintf("Restaurar a versión %s (%s)?", version, selected.Hash[:7]),
				) + "\n" +
				tui.StyleSubtitle.Render("mensaje del commit:") + "\n" +
				m.commitInput.View() + "\n\n" +
				lipgloss.NewStyle().Foreground(tui.ColorGreen).Render("enter") +
				tui.StyleSubtitle.Render(": confirmar   ") +
				lipgloss.NewStyle().Foreground(tui.ColorRed).Render("esc") +
				tui.StyleSubtitle.Render(": cancelar"),
		)

	return diffContent + "\n\n" + dialog
}

func (m Model) renderActions() string {
	hi := lipgloss.NewStyle().Foreground(tui.ColorBlue)
	norm := lipgloss.NewStyle().Foreground(tui.ColorFg)
	tip := tui.StyleSubtitle

	return hi.Render("enter") + norm.Render(" ver diff   ") +
		hi.Render("R") + norm.Render(" restaurar esta versión   ") +
		hi.Render("a") + norm.Render(" historial completo") + "\n" +
		tip.Render("restaurar: rollback granular por rol; crea un commit nuevo, sin efectos sobre otros roles.")
}

func renderHeader(h domain.Harness, roleName string) string {
	name := lipgloss.NewStyle().Foreground(tui.ColorFg).Render(h.Name)
	role := lipgloss.NewStyle().Foreground(tui.ColorYellow).Bold(true).Render(roleName)
	return tui.StyleSubtitle.Background(tui.ColorPanelBg).Render(
		fmt.Sprintf("lazyharness · %s · historial de %s (filtrado por rol)", name, role),
	) + "\n"
}

func renderKeybar(width int) string {
	hi, k := tui.StyleKeyHi, tui.StyleHelp
	return tui.StyleKeybar.Width(width).Render(
		hi.Render("↑↓/jk") + k.Render(": versiones  ") +
			hi.Render("tab") + k.Render(": panel  ") +
			hi.Render("R") + k.Render(": restaurar  ") +
			hi.Render("esc") + k.Render(": volver al harness  ") +
			hi.Render("q") + k.Render(": salir"),
	)
}

func relativeTime(t time.Time) string { return storage.FormatRelativeTime(t) }

func shortHash(hash string) string {
	if len(hash) <= 7 {
		return hash
	}
	return hash[:7]
}

func leftWidth(w int) int      { return w * 37 / 100 }
func rightWidth(w int) int     { return w - leftWidth(w) - 2 }
func actionsTop(h int) int     { return h * 82 / 100 }
func actionsHeight(h int) int  { return h - actionsTop(h) - 4 }
func rightDims(w, h int) (int, int) {
	return rightWidth(w) - 4, actionsTop(h) - 4
}
