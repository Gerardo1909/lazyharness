package workspace

import (
	"fmt"
	"strings"
	"time"

	"github.com/Gerardo1909/lazyharness/internal/domain"
	"github.com/Gerardo1909/lazyharness/internal/storage"
	"github.com/Gerardo1909/lazyharness/internal/tui"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type wsState int

const (
	stateList wsState = iota
	stateNewTask
)

type Model struct {
	harness     domain.Harness
	tasks       []domain.Task
	selectedIdx int
	state       wsState
	titleInput  textinput.Model
	roleInput   textinput.Model
	inputFocus  int
	errMsg      string
	width       int
	height      int
}

func New(h domain.Harness, width, height int) Model {
	tasks, _ := storage.LoadTasks(h.ProjectDir)
	if tasks == nil {
		tasks = []domain.Task{}
	}

	ti := textinput.New()
	ti.Placeholder = "título de la tarea…"
	ti.Width = width/2 - 8

	ri := textinput.New()
	ri.Placeholder = "rol asociado (opcional)…"
	ri.Width = width/2 - 8

	return Model{
		harness:    h,
		tasks:      tasks,
		titleInput: ti,
		roleInput:  ri,
		width:      width,
		height:     height,
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyMsg:
		if m.state == stateNewTask {
			return m.updateForm(msg)
		}
		switch msg.String() {
		case "up", "k":
			if m.selectedIdx > 0 {
				m.selectedIdx--
			}
		case "down", "j":
			if m.selectedIdx < len(m.tasks)-1 {
				m.selectedIdx++
			}
		case "n":
			m.state = stateNewTask
			m.titleInput.SetValue("")
			m.roleInput.SetValue("")
			m.inputFocus = 0
			m.titleInput.Focus()
			m.errMsg = ""
			return m, textinput.Blink
		case " ":
			if len(m.tasks) > 0 {
				m.cycleStatus()
				m.save()
			}
		case "d":
			if len(m.tasks) > 0 {
				m.tasks = append(m.tasks[:m.selectedIdx], m.tasks[m.selectedIdx+1:]...)
				if m.selectedIdx >= len(m.tasks) && m.selectedIdx > 0 {
					m.selectedIdx--
				}
				m.save()
			}
		case "esc":
			return m, func() tea.Msg { return tui.GoBackMsg{} }
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *Model) cycleStatus() {
	t := &m.tasks[m.selectedIdx]
	switch t.Status {
	case domain.TaskPending:
		t.Status = domain.TaskInProgress
	case domain.TaskInProgress:
		t.Status = domain.TaskDone
	case domain.TaskDone:
		t.Status = domain.TaskPending
	}
}

func (m *Model) save() {
	_ = storage.SaveTasks(m.harness.ProjectDir, m.tasks)
}

func (m Model) updateForm(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		if m.inputFocus == 0 {
			m.titleInput.Blur()
			m.inputFocus = 1
			m.roleInput.Focus()
		} else {
			m.roleInput.Blur()
			m.inputFocus = 0
			m.titleInput.Focus()
		}
		return m, textinput.Blink
	case "enter":
		title := strings.TrimSpace(m.titleInput.Value())
		if title == "" {
			m.errMsg = "el título no puede estar vacío"
			return m, nil
		}
		role := strings.TrimSpace(m.roleInput.Value())
		id := fmt.Sprintf("t%d", time.Now().UnixNano())
		task := domain.NewTask(id, role, title)
		m.tasks = append(m.tasks, task)
		m.save()
		m.state = stateList
		m.errMsg = ""
		m.selectedIdx = len(m.tasks) - 1
		return m, nil
	case "esc":
		m.state = stateList
		m.errMsg = ""
		return m, nil
	}

	var cmd tea.Cmd
	if m.inputFocus == 0 {
		m.titleInput, cmd = m.titleInput.Update(msg)
	} else {
		m.roleInput, cmd = m.roleInput.Update(msg)
	}
	return m, cmd
}

func (m Model) View() string {
	header := renderHeader(m.harness)

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(tui.ColorBlue).Bold(true).Render("Tareas") + "\n\n")

	if len(m.tasks) == 0 && m.state != stateNewTask {
		b.WriteString(tui.StyleSubtitle.Render("  Sin tareas. Presioná 'n' para crear una.") + "\n")
	}

	for i, t := range m.tasks {
		icon := statusIcon(t.Status)
		style := lipgloss.NewStyle().Foreground(tui.ColorFg)
		if i == m.selectedIdx {
			style = style.Background(tui.ColorSelection)
		}

		rolePart := ""
		if t.Role != "" {
			rolePart = tui.StyleSubtitle.Render(fmt.Sprintf(" [%s]", t.Role))
		}
		timePart := tui.StyleSubtitle.Render(" · " + storage.FormatRelativeTime(t.CreatedAt))

		b.WriteString(fmt.Sprintf("  %s %s%s%s\n", icon, style.Render(t.Title), rolePart, timePart))
	}

	if m.state == stateNewTask {
		b.WriteString("\n" + m.renderNewTaskForm())
	}

	panel := tui.StyleActiveBorder.
		Width(m.width - 4).Height(m.height - 4).
		Render(b.String())

	keybar := m.renderKeybar()
	return lipgloss.JoinVertical(lipgloss.Left, header, panel, keybar)
}

func statusIcon(s domain.TaskStatus) string {
	switch s {
	case domain.TaskPending:
		return lipgloss.NewStyle().Foreground(tui.ColorComment).Render("○")
	case domain.TaskInProgress:
		return lipgloss.NewStyle().Foreground(tui.ColorYellow).Render("◐")
	case domain.TaskDone:
		return lipgloss.NewStyle().Foreground(tui.ColorGreen).Render("●")
	default:
		return "?"
	}
}

func (m Model) renderNewTaskForm() string {
	var b strings.Builder
	b.WriteString(tui.StyleSubtitle.Render("Nueva tarea:") + "\n")
	label0, label1 := tui.StyleSubtitle, tui.StyleSubtitle
	if m.inputFocus == 0 {
		label0 = tui.StyleKeyHi
	} else {
		label1 = tui.StyleKeyHi
	}
	b.WriteString("  " + label0.Render("Título:") + " " + m.titleInput.View() + "\n")
	b.WriteString("  " + label1.Render("Rol:") + "    " + m.roleInput.View() + "\n")
	if m.errMsg != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(tui.ColorRed).Render("  ✗ "+m.errMsg) + "\n")
	}
	b.WriteString(tui.StyleHelp.Render("  tab: siguiente  enter: crear  esc: cancelar"))
	return b.String()
}

func renderHeader(h domain.Harness) string {
	name := lipgloss.NewStyle().Foreground(tui.ColorFg).Render(h.Name)
	return tui.StyleSubtitle.Background(tui.ColorPanelBg).Render(
		fmt.Sprintf("lazyharness · %s · tareas", name),
	) + "\n"
}

func (m Model) renderKeybar() string {
	hi, k := tui.StyleKeyHi, tui.StyleHelp
	if m.state == stateNewTask {
		return tui.StyleKeybar.Width(m.width).Render(
			hi.Render("enter") + k.Render(": crear  ") +
				hi.Render("tab") + k.Render(": siguiente campo  ") +
				hi.Render("esc") + k.Render(": cancelar"),
		)
	}
	return tui.StyleKeybar.Width(m.width).Render(
		hi.Render("n") + k.Render(": nueva tarea  ") +
			hi.Render("space") + k.Render(": cambiar estado  ") +
			hi.Render("d") + k.Render(": eliminar  ") +
			hi.Render("↑↓/jk") + k.Render(": navegar  ") +
			hi.Render("esc") + k.Render(": volver"),
	)
}
