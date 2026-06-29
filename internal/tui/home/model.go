package home

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Gerardo1909/lazyharness/internal/domain"
	"github.com/Gerardo1909/lazyharness/internal/storage"
	"github.com/Gerardo1909/lazyharness/internal/tui"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type homeState int

const (
	stateList       homeState = iota
	stateNewHarness
)

type harnessItem struct {
	summary domain.HarnessSummary
}

func (i harnessItem) FilterValue() string { return i.summary.Name }
func (i harnessItem) Title() string       { return i.summary.Name }
func (i harnessItem) Description() string { return i.summary.ProjectDir }

type Model struct {
	list       list.Model
	summaries  []domain.HarnessSummary
	state      homeState
	inputs     [3]textinput.Model
	inputFocus int
	formErr    string
	width      int
	height     int
}

func New(width, height int) Model {
	summaries := loadSummaries()
	items := toListItems(summaries)

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(tui.ColorBlue).
		BorderLeftForeground(tui.ColorBlue)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(tui.ColorComment).
		BorderLeftForeground(tui.ColorBlue)

	lw, lh := leftDims(width, height)
	l := list.New(items, delegate, lw, lh)
	l.Title = "Harnesses"
	l.Styles.Title = tui.StyleTitle.MarginLeft(1)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)

	return Model{list: l, summaries: summaries, width: width, height: height}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		lw, lh := leftDims(m.width, m.height)
		m.list.SetSize(lw, lh)
		fw := formWidth(m.width)
		for i := range m.inputs {
			m.inputs[i].Width = fw
		}
		return m, nil

	case tea.KeyMsg:
		if m.state == stateNewHarness {
			return m.updateForm(msg)
		}
		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(harnessItem); ok {
				return m, func() tea.Msg { return tui.OpenHarnessMsg{Summary: item.summary} }
			}
		case "n":
			m.state = stateNewHarness
			m.inputs = initFormInputs(m.width)
			m.inputFocus = 0
			m.formErr = ""
			return m, textinput.Blink
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) updateForm(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "down":
		m.inputs[m.inputFocus].Blur()
		m.inputFocus = (m.inputFocus + 1) % 3
		m.inputs[m.inputFocus].Focus()
		return m, textinput.Blink
	case "shift+tab", "up":
		m.inputs[m.inputFocus].Blur()
		m.inputFocus = (m.inputFocus + 2) % 3
		m.inputs[m.inputFocus].Focus()
		return m, textinput.Blink
	case "enter":
		if m.inputFocus < 2 {
			m.inputs[m.inputFocus].Blur()
			m.inputFocus++
			m.inputs[m.inputFocus].Focus()
			return m, textinput.Blink
		}
		return m.submitForm()
	case "esc":
		m.state = stateList
		m.formErr = ""
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.inputs[m.inputFocus], cmd = m.inputs[m.inputFocus].Update(msg)
	return m, cmd
}

func (m Model) submitForm() (Model, tea.Cmd) {
	name := strings.TrimSpace(m.inputs[0].Value())
	dir := expandTilde(strings.TrimSpace(m.inputs[1].Value()))
	format := strings.TrimSpace(m.inputs[2].Value())

	h, err := domain.NewHarness(name, dir, format)
	if err != nil {
		m.formErr = err.Error()
		return m, nil
	}
	if err := storage.SaveHarness(dir, h); err != nil {
		m.formErr = err.Error()
		return m, nil
	}
	_ = storage.Commit(dir, "harness creado: "+h.Name)
	m.summaries = loadSummaries()
	m.list.SetItems(toListItems(m.summaries))
	m.state = stateList
	m.formErr = ""
	return m, nil
}

func (m Model) View() string {
	if m.state == stateNewHarness {
		return m.viewForm()
	}

	lw, lh := leftDims(m.width, m.height)
	rw := m.width - lw - 2

	leftPanel := tui.StyleActiveBorder.
		Width(lw).Height(lh).
		Render(m.list.View())

	rightPanel := tui.StyleBorder.
		Width(rw).Height(lh).
		Render(m.renderDetail(rw - 4))

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	return lipgloss.JoinVertical(lipgloss.Left, body, renderKeybar(m.width))
}

func (m Model) viewForm() string {
	labels := [3]string{"Nombre:", "Directorio del proyecto:", "Formato (xml/md/txt):"}
	var b strings.Builder

	b.WriteString(tui.StyleTitle.Render("Nuevo Harness") + "\n\n")
	for i, inp := range m.inputs {
		label := labels[i]
		if i == m.inputFocus {
			b.WriteString(tui.StyleKeyHi.Render(label) + "\n")
		} else {
			b.WriteString(tui.StyleSubtitle.Render(label) + "\n")
		}
		b.WriteString(inp.View() + "\n\n")
	}
	if m.formErr != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(tui.ColorRed).Render("✗ "+m.formErr) + "\n\n")
	}
	b.WriteString(tui.StyleHelp.Render("tab/↓: siguiente   shift+tab/↑: anterior   enter: crear   esc: cancelar"))

	form := tui.StyleActiveBorder.Padding(1, 3).Width(m.width/2).Render(b.String())

	return lipgloss.NewStyle().
		Width(m.width).Height(m.height - 3).
		Align(lipgloss.Center, lipgloss.Center).
		Render(form) + "\n" + renderKeybar(m.width)
}

func (m Model) renderDetail(width int) string {
	item, ok := m.list.SelectedItem().(harnessItem)
	if !ok {
		return tui.StyleSubtitle.Render("Sin harnesses.\n\nPresioná 'n' para crear uno.")
	}
	s := item.summary

	var b strings.Builder
	b.WriteString(tui.StyleTitle.Render(s.Name) + "\n")
	b.WriteString(tui.StyleSubtitle.Render(
		fmt.Sprintf("%s · formato: %s · provider: %s / %s",
			s.ProjectDir, s.PromptFormat, s.Provider, s.Model),
	) + "\n\n")

	b.WriteString(lipgloss.NewStyle().Foreground(tui.ColorBlue).Render(
		fmt.Sprintf("Roles (%d)", s.RoleCount),
	) + "\n")
	for _, r := range s.Roles {
		color := r.Color
		if color == "" {
			color = domain.DefaultColor
		}
		bullet := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render("●")
		desc := tui.StyleSubtitle.Render("  " + roleTip(r))
		b.WriteString(fmt.Sprintf("  %s %s%s\n", bullet, r.Name, desc))
	}

	if len(s.Workflow) > 0 {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(tui.ColorBlue).Render("Workflow sugerido") + "\n")
		b.WriteString("  " + renderWorkflowLine(s) + "\n")
		b.WriteString(tui.StyleSubtitle.Render("  (informativo: podés invocar cualquier rol directo)") + "\n")
	}

	if s.LastCommit != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(tui.ColorBlue).Render("Último commit") + "\n")
		b.WriteString(fmt.Sprintf("  %s\n", s.LastCommit))
		if s.TotalCommits > 0 {
			b.WriteString(tui.StyleSubtitle.Render(
				fmt.Sprintf("  %d commits en total", s.TotalCommits),
			) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(tui.ColorBlue).Render("Tareas") + "\n")
	b.WriteString(fmt.Sprintf("  %d en curso · %d hechas · tareas.json\n",
		s.TasksInProgress, s.TasksDone))

	return b.String()
}

func renderWorkflowLine(s domain.HarnessSummary) string {
	roleColorMap := make(map[string]string)
	for _, r := range s.Roles {
		roleColorMap[r.Name] = r.Color
	}
	parts := make([]string, len(s.Workflow))
	for i, name := range s.Workflow {
		color := roleColorMap[name]
		if color == "" {
			color = domain.DefaultColor
		}
		parts[i] = lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(name)
	}
	return strings.Join(parts, " → ")
}

func roleTip(r domain.Role) string {
	if r.Parent != "" {
		return fmt.Sprintf("→ hijo de %s", r.Parent)
	}
	return ""
}

func renderKeybar(width int) string {
	hi, k := tui.StyleKeyHi, tui.StyleHelp
	return tui.StyleKeybar.Width(width).Render(
		hi.Render("n") + k.Render(": nuevo harness  ") +
			hi.Render("enter") + k.Render(": abrir  ") +
			hi.Render("d") + k.Render(": eliminar  ") +
			hi.Render("↑↓/jk") + k.Render(": navegar  ") +
			hi.Render("?") + k.Render(": ayuda  ") +
			hi.Render("q") + k.Render(": salir"),
	)
}

func leftDims(w, h int) (int, int) { return w * 32 / 100, h - 3 }
func formWidth(w int) int          { return w/2 - 8 }

func loadSummaries() []domain.HarnessSummary {
	home, _ := os.UserHomeDir()
	roots := []string{
		home,
		home + "/dev",
		home + "/work",
		home + "/projects",
		home + "/personal",
		home + "/trabajo",
	}
	return storage.FindHarnesses(roots)
}

func toListItems(summaries []domain.HarnessSummary) []list.Item {
	items := make([]list.Item, len(summaries))
	for i, s := range summaries {
		items[i] = harnessItem{s}
	}
	return items
}

func initFormInputs(width int) [3]textinput.Model {
	homeDir, _ := os.UserHomeDir()
	placeholders := [3]string{"mi-harness", filepath.Join(homeDir, "dev", ""), "md"}
	var inputs [3]textinput.Model
	for i := range inputs {
		t := textinput.New()
		t.Placeholder = placeholders[i]
		t.Width = formWidth(width)
		inputs[i] = t
	}
	inputs[2].SetValue("md")
	inputs[0].Focus()
	return inputs
}

func expandTilde(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}
