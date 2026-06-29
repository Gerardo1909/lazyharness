package editor

import (
	"fmt"
	"strings"

	"github.com/Gerardo1909/lazyharness/internal/domain"
	"github.com/Gerardo1909/lazyharness/internal/storage"
	"github.com/Gerardo1909/lazyharness/internal/tui"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type editorState int

const (
	stateEditing     editorState = iota
	stateCommitMsg               // ingresando mensaje de commit
	stateAIChat                  // IA activa
)

// Model es el editor embebido de prompts.
type Model struct {
	harness     domain.Harness
	roleName    string
	textarea    textarea.Model
	commitInput textinput.Model
	state       editorState
	showAI      bool
	aiChat      []string // historial del chat con IA (stub)
	saved       bool
	errMsg      string
	width       int
	height      int
}

func New(h domain.Harness, roleName string, width, height int) Model {
	role, ok := h.FindRoleByName(roleName)

	var content, errMsg string
	if !ok {
		errMsg = fmt.Sprintf("rol '%s' no encontrado en el harness", roleName)
	} else {
		var err error
		content, err = storage.ReadPromptFile(h.ProjectDir, role.PromptFile)
		if err != nil {
			errMsg = fmt.Sprintf("no se pudo leer el prompt: %v", err)
		}
	}

	ta := textarea.New()
	ta.SetValue(content)
	ta.SetWidth(editorWidth(width))
	ta.SetHeight(editorHeight(height))
	ta.Focus()
	ta.CharLimit = 0

	ci := textinput.New()
	ci.Placeholder = "mensaje del commit (obligatorio)…"
	ci.Width = editorWidth(width) - 4

	return Model{
		harness:     h,
		roleName:    roleName,
		textarea:    ta,
		commitInput: ci,
		state:       stateEditing,
		errMsg:      errMsg,
		width:       width,
		height:      height,
	}
}

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.textarea.SetWidth(editorWidth(m.width))
		m.textarea.SetHeight(editorHeight(m.height))

	case tea.KeyMsg:
		switch m.state {
		case stateCommitMsg:
			return m.updateCommitInput(msg)
		case stateEditing, stateAIChat:
			return m.updateEditing(msg)
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m Model) updateEditing(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+s":
		// pasar a pedir mensaje de commit
		m.state = stateCommitMsg
		m.commitInput.Focus()
		return m, textinput.Blink
	case "f2":
		m.showAI = !m.showAI
		if m.showAI {
			m.state = stateAIChat
		} else {
			m.state = stateEditing
		}
		return m, nil
	case "esc":
		if m.showAI {
			m.showAI = false
			m.state = stateEditing
			return m, nil
		}
		// auto-save file before leaving (no commit)
		role, _ := m.harness.FindRoleByName(m.roleName)
		if role.PromptFile != "" {
			_ = storage.WritePromptFile(m.harness.ProjectDir, role.PromptFile, m.textarea.Value())
		}
		return m, func() tea.Msg { return tui.GoBackMsg{} }
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m Model) updateCommitInput(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		commitMsg := strings.TrimSpace(m.commitInput.Value())
		if commitMsg == "" {
			return m, nil
		}
		role, _ := m.harness.FindRoleByName(m.roleName)
		if err := storage.WritePromptFile(m.harness.ProjectDir, role.PromptFile, m.textarea.Value()); err != nil {
			m.errMsg = fmt.Sprintf("Error al guardar: %v", err)
			m.state = stateEditing
			m.commitInput.SetValue("")
			m.textarea.Focus()
			return m, nil
		}
		_ = storage.Commit(m.harness.ProjectDir, commitMsg)
		m.saved = true
		m.errMsg = ""
		return m, func() tea.Msg { return tui.SaveAndBackMsg{CommitMessage: commitMsg} }
	case "esc":
		m.state = stateEditing
		m.commitInput.SetValue("")
		m.textarea.Focus()
		return m, nil
	}

	var cmd tea.Cmd
	m.commitInput, cmd = m.commitInput.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	header := renderHeader(m.harness, m.roleName, m.saved)
	lw := editorWidth(m.width)

	role, _ := m.harness.FindRoleByName(m.roleName)
	editorTitle := tui.StyleSubtitle.Render(
		fmt.Sprintf("┤ %s · editando ├", role.PromptFile),
	)

	editorPanel := tui.StyleActiveBorder.
		Width(lw).Height(editorPanelHeight(m.height, m.showAI)).
		Render(editorTitle + "\n" + m.textarea.View())

	var panels []string
	panels = append(panels, header)
	panels = append(panels, editorPanel)

	if m.errMsg != "" {
		errPanel := lipgloss.NewStyle().
			Foreground(tui.ColorRed).
			Background(tui.ColorPanelBg).
			Width(lw).Padding(0, 1).
			Render("✗ " + m.errMsg)
		panels = append(panels, errPanel)
	}

	if m.state == stateCommitMsg {
		commitPanel := tui.StyleBorder.
			BorderForeground(tui.ColorYellow).
			Width(lw).Padding(0, 1).
			Render(
				lipgloss.NewStyle().Foreground(tui.ColorYellow).Render("Mensaje del commit:") + "\n" +
					m.commitInput.View() + "\n" +
					tui.StyleHelp.Render("enter: confirmar   esc: cancelar"),
			)
		panels = append(panels, commitPanel)
	} else if m.showAI {
		aiPanel := tui.StyleBorder.
			BorderForeground(tui.ColorPurple).
			Width(lw).Height(aiChatHeight(m.height)).
			Render(renderAIPanel(m.aiChat))
		panels = append(panels, aiPanel)
	}

	panels = append(panels, renderKeybar(m.width, m.showAI))
	return lipgloss.JoinVertical(lipgloss.Left, panels...)
}

func renderAIPanel(chat []string) string {
	if len(chat) == 0 {
		return tui.StyleSubtitle.Render(
			"▸ Describí el harness que querés construir y la IA poblará los prompts.\n" +
				"  Ejemplo: quiero un flujo con arquitecto líder, reviewer y dos devs",
		)
	}
	return strings.Join(chat, "\n")
}

func renderHeader(h domain.Harness, roleName string, saved bool) string {
	status := lipgloss.NewStyle().Foreground(tui.ColorYellow).Render("sin guardar")
	if saved {
		status = lipgloss.NewStyle().Foreground(tui.ColorGreen).Render("guardado")
	}
	name := lipgloss.NewStyle().Foreground(tui.ColorFg).Render(h.Name)
	role := lipgloss.NewStyle().Foreground(tui.ColorFg).Render(roleName)
	return tui.StyleSubtitle.Background(tui.ColorPanelBg).Render(
		fmt.Sprintf("lazyharness · %s · editando: %s · %s", name, role, status),
	) + "\n"
}

func renderKeybar(width int, aiActive bool) string {
	hi, k := tui.StyleKeyHi, tui.StyleHelp
	aiLabel := "activar IA"
	if aiActive {
		aiLabel = "ocultar IA"
	}
	return tui.StyleKeybar.Width(width).Render(
		hi.Render("F2") + k.Render(": "+aiLabel+"  ") +
			hi.Render("ctrl+s") + k.Render(": guardar (commit)  ") +
			hi.Render("esc") + k.Render(": volver (auto-guarda)"),
	)
}

func editorWidth(w int) int          { return w * 70 / 100 }
func editorHeight(h int) int         { return h * 55 / 100 }
func editorPanelHeight(h int, ai bool) int {
	if ai {
		return h * 55 / 100
	}
	return h * 78 / 100
}
func aiChatHeight(h int) int { return h * 18 / 100 }
