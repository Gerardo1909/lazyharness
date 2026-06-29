package harness

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/Gerardo1909/lazyharness/internal/domain"
	"github.com/Gerardo1909/lazyharness/internal/storage"
	"github.com/Gerardo1909/lazyharness/internal/tui"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type activePanel int

const (
	panelRoles activePanel = iota
	panelPrompt
)

type harnessState int

const (
	stateNormal harnessState = iota
	stateNewRole
	stateImproving
	stateShowSuggestion
)

type invokeFinishedMsg struct{ err error }
type improveResultMsg struct {
	suggestion string
	err        error
}

// Model representa la vista de un harness: roles + prompt + workflow + acciones.
type Model struct {
	harness     domain.Harness
	selectedIdx int
	promptVP    viewport.Model
	active      activePanel
	promptText  string
	state       harnessState
	roleInput   textinput.Model
	suggestion  string
	errMsg      string
	width       int
	height      int
}

func New(h domain.Harness, width, height int) Model {
	ri := textinput.New()
	ri.Placeholder = "nombre del rol…"
	ri.Width = width / 3

	m := Model{
		harness:   h,
		roleInput: ri,
		width:     width,
		height:    height,
	}
	_, vpH := rightDims(width, height)
	m.promptVP = viewport.New(rightWidth(width)-4, vpH)
	m.loadPrompt()
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		_, vpH := rightDims(m.width, m.height)
		m.promptVP.Width = rightWidth(m.width) - 4
		m.promptVP.Height = vpH
		m.promptVP.SetContent(m.renderPromptContent())

	case invokeFinishedMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("error al invocar: %v", msg.err)
		}
		return m, nil

	case improveResultMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("error al mejorar: %v", msg.err)
			m.state = stateNormal
		} else {
			m.state = stateShowSuggestion
			m.suggestion = msg.suggestion
			m.promptVP.SetContent(msg.suggestion)
			m.promptVP.GotoTop()
		}
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case stateNewRole:
			return m.updateNewRole(msg)
		case stateShowSuggestion:
			return m.updateSuggestion(msg)
		case stateImproving:
			if msg.String() == "esc" {
				m.state = stateNormal
				m.loadPrompt()
				return m, nil
			}
			return m, nil
		default:
			return m.updateNormal(msg)
		}
	}

	var cmd tea.Cmd
	m.promptVP, cmd = m.promptVP.Update(msg)
	return m, cmd
}

func (m Model) updateNormal(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		if m.active == panelRoles {
			m.active = panelPrompt
		} else {
			m.active = panelRoles
		}
		return m, nil
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		return m, func() tea.Msg { return tui.GoBackMsg{} }
	case "n":
		m.state = stateNewRole
		m.roleInput.SetValue("")
		m.roleInput.Focus()
		m.errMsg = ""
		return m, textinput.Blink
	}

	if m.active == panelRoles {
		return m.updateRolesPanel(msg)
	}

	var cmd tea.Cmd
	m.promptVP, cmd = m.promptVP.Update(msg)
	return m, cmd
}

func (m Model) updateRolesPanel(msg tea.KeyMsg) (Model, tea.Cmd) {
	roles := m.harness.Roles
	switch msg.String() {
	case "up", "k":
		if m.selectedIdx > 0 {
			m.selectedIdx--
			m.loadPrompt()
		}
	case "down", "j":
		if m.selectedIdx < len(roles)-1 {
			m.selectedIdx++
			m.loadPrompt()
		}
	case "e":
		if len(roles) == 0 {
			return m, nil
		}
		return m, func() tea.Msg {
			return tui.OpenEditorMsg{Harness: m.harness, RoleName: roles[m.selectedIdx].Name}
		}
	case "h":
		if len(roles) == 0 {
			return m, nil
		}
		return m, func() tea.Msg {
			return tui.OpenHistoryMsg{Harness: m.harness, RoleName: roles[m.selectedIdx].Name}
		}
	case "t":
		return m, func() tea.Msg {
			return tui.OpenWorkspaceMsg{Harness: m.harness}
		}
	case "i":
		if len(roles) == 0 {
			return m, nil
		}
		return m.invokeRole()
	case "m":
		if len(roles) == 0 {
			return m, nil
		}
		return m.improveRole()
	case "s":
		_ = storage.Commit(m.harness.ProjectDir, "guardar desde harness")
		return m, nil
	}
	return m, nil
}

func (m Model) invokeRole() (Model, tea.Cmd) {
	role := m.harness.Roles[m.selectedIdx]
	content, err := storage.ReadPromptFile(m.harness.ProjectDir, role.PromptFile)
	if err != nil {
		m.errMsg = fmt.Sprintf("no se pudo leer el prompt: %v", err)
		return m, nil
	}
	c := exec.Command("claude", "--system-prompt", content)
	c.Dir = m.harness.ProjectDir
	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return invokeFinishedMsg{err: err}
	})
}

func (m Model) improveRole() (Model, tea.Cmd) {
	m.state = stateImproving
	role := m.harness.Roles[m.selectedIdx]
	projectDir := m.harness.ProjectDir
	promptFile := role.PromptFile
	return m, func() tea.Msg {
		content, err := storage.ReadPromptFile(projectDir, promptFile)
		if err != nil {
			return improveResultMsg{err: err}
		}
		prompt := "Revisá y mejorá este prompt de agente de IA. " +
			"Devolvé SOLO el prompt mejorado, sin explicaciones ni markdown de wrapping:\n\n" + content
		cmd := exec.Command("claude", "--print", prompt)
		cmd.Dir = projectDir
		out, err := cmd.Output()
		if err != nil {
			return improveResultMsg{err: fmt.Errorf("claude: %w", err)}
		}
		return improveResultMsg{suggestion: string(out)}
	}
}

func (m Model) updateNewRole(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		name := strings.TrimSpace(m.roleInput.Value())
		if name == "" {
			m.errMsg = "el nombre no puede estar vacío"
			return m, nil
		}
		promptFile := name + "." + m.harness.PromptFormat
		role, err := domain.NewRole(name, domain.DefaultColor, promptFile, "")
		if err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		if err := m.harness.AddRole(role); err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		if err := storage.SaveHarness(m.harness.ProjectDir, m.harness); err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		_ = storage.WritePromptFile(m.harness.ProjectDir, promptFile, "")
		_ = storage.Commit(m.harness.ProjectDir, "nuevo rol: "+name)
		m.state = stateNormal
		m.errMsg = ""
		m.selectedIdx = len(m.harness.Roles) - 1
		m.loadPrompt()
		return m, nil
	case "esc":
		m.state = stateNormal
		m.errMsg = ""
		return m, nil
	}

	var cmd tea.Cmd
	m.roleInput, cmd = m.roleInput.Update(msg)
	return m, cmd
}

func (m Model) updateSuggestion(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		role := m.harness.Roles[m.selectedIdx]
		_ = storage.WritePromptFile(m.harness.ProjectDir, role.PromptFile, m.suggestion)
		_ = storage.Commit(m.harness.ProjectDir, "prompt mejorado por IA: "+role.Name)
		m.state = stateNormal
		m.suggestion = ""
		m.loadPrompt()
		return m, nil
	case "n", "esc":
		m.state = stateNormal
		m.suggestion = ""
		m.loadPrompt()
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	header := renderHeader(m.harness)
	lw := leftWidth(m.width)
	rw := rightWidth(m.width)

	rolesBorderStyle := tui.StyleBorder
	if m.active == panelRoles {
		rolesBorderStyle = tui.StyleActiveBorder
	}
	rolesContent := m.renderRoles()
	if m.state == stateNewRole {
		rolesContent += "\n" + m.renderNewRoleForm()
	}
	rolesPanel := rolesBorderStyle.
		Width(lw).Height(rolesHeight(m.height)).
		Render(rolesContent)

	workflowPanel := tui.StyleBorder.
		Width(lw).Height(workflowHeight(m.height)).
		Render(m.renderWorkflow())

	leftCol := lipgloss.JoinVertical(lipgloss.Left, rolesPanel, workflowPanel)

	promptBorderStyle := tui.StyleBorder
	if m.active == panelPrompt {
		promptBorderStyle = tui.StyleActiveBorder
	}
	promptTitle := m.promptTitle()
	promptHeader := tui.StyleSubtitle.Render(promptTitle) + "\n"
	promptPanel := promptBorderStyle.
		Width(rw).Height(promptHeight(m.height)).
		Render(promptHeader + m.promptVP.View())

	rightCol := promptPanel
	if m.errMsg != "" {
		errBar := lipgloss.NewStyle().
			Foreground(tui.ColorRed).Background(tui.ColorPanelBg).
			Width(rw).Padding(0, 1).
			Render("✗ " + m.errMsg)
		rightCol = lipgloss.JoinVertical(lipgloss.Left, rightCol, errBar)
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, rightCol)
	keybar := m.renderKeybar()
	return lipgloss.JoinVertical(lipgloss.Left, header, body, keybar)
}

func (m Model) promptTitle() string {
	switch m.state {
	case stateImproving:
		return "┤ mejorando con IA… ├"
	case stateShowSuggestion:
		return "┤ sugerencia IA · y: aceptar · n: rechazar ├"
	default:
		selectedRole := m.currentRole()
		return fmt.Sprintf("┤ %s · solo lectura ├", selectedRole.PromptFile)
	}
}

func (m Model) renderRoles() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(tui.ColorBlue).Bold(true).Render("Roles") + "\n\n")
	if len(m.harness.Roles) == 0 {
		b.WriteString(tui.StyleSubtitle.Render("  Sin roles. Presioná 'n' para crear uno.") + "\n")
		return b.String()
	}
	for i, role := range m.harness.Roles {
		color := role.Color
		if color == "" {
			color = domain.DefaultColor
		}
		bullet := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render("●")
		prefix := "  "
		if role.Parent != "" {
			if i == len(m.harness.Roles)-1 || m.harness.Roles[i+1].Parent == "" {
				prefix = tui.StyleSubtitle.Render("└─") + " "
			} else {
				prefix = tui.StyleSubtitle.Render("├─") + " "
			}
		}

		nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
		if i == m.selectedIdx {
			nameStyle = nameStyle.Bold(true).
				Background(tui.ColorSelection)
		}
		line := prefix + bullet + " " + nameStyle.Render(role.Name)
		b.WriteString(line + "\n")
	}
	return b.String()
}

func (m Model) renderNewRoleForm() string {
	var b strings.Builder
	b.WriteString(tui.StyleSubtitle.Render("Nuevo rol:") + "\n")
	b.WriteString("  " + m.roleInput.View() + "\n")
	if m.errMsg != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(tui.ColorRed).Render("  ✗ "+m.errMsg) + "\n")
	}
	b.WriteString(tui.StyleHelp.Render("  enter: crear   esc: cancelar"))
	return b.String()
}

func (m Model) renderWorkflow() string {
	var b strings.Builder
	b.WriteString(tui.StyleSubtitle.Render("orden sugerido:") + "\n\n")
	for i, roleName := range m.harness.Workflow {
		role, ok := m.harness.FindRoleByName(roleName)
		color := domain.DefaultColor
		if ok {
			color = role.Color
		}
		styled := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(roleName)
		b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, styled))
	}
	if len(m.harness.Workflow) == 0 {
		b.WriteString(tui.StyleSubtitle.Render("  (sin workflow definido)"))
	}
	return b.String()
}

func (m *Model) loadPrompt() {
	m.promptText = m.renderPromptContent()
	m.promptVP.SetContent(m.promptText)
	m.promptVP.GotoTop()
}

func (m Model) renderPromptContent() string {
	role := m.currentRole()
	if role.PromptFile == "" {
		return tui.StyleSubtitle.Render("(sin prompt asignado)")
	}
	content, err := storage.ReadPromptFile(m.harness.ProjectDir, role.PromptFile)
	if err != nil {
		return tui.StyleSubtitle.Render(fmt.Sprintf("(no se pudo leer %s: %v)", role.PromptFile, err))
	}
	if content == "" {
		return tui.StyleSubtitle.Render("(prompt vacío — presioná 'e' para editar)")
	}
	return colorizeReferences(content, m.harness.Roles)
}

func colorizeReferences(content string, roles []domain.Role) string {
	colorMap := make(map[string]string)
	for _, r := range roles {
		colorMap[r.Name] = r.Color
	}
	lines := strings.Split(content, "\n")
	result := make([]string, len(lines))
	for i, line := range lines {
		words := strings.Fields(line)
		for j, word := range words {
			if strings.HasPrefix(word, "@") {
				roleName := strings.TrimPrefix(word, "@")
				for _, p := range []string{".", ",", ":", ";"} {
					roleName = strings.TrimSuffix(roleName, p)
				}
				if color, ok := colorMap[roleName]; ok {
					words[j] = lipgloss.NewStyle().
						Foreground(lipgloss.Color(color)).
						Bold(true).
						Render(word)
				}
			}
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		result[i] = line[:indent] + strings.Join(words, " ")
	}
	return strings.Join(result, "\n")
}

func renderHeader(h domain.Harness) string {
	style := tui.StyleSubtitle.Background(tui.ColorPanelBg)
	name := lipgloss.NewStyle().Foreground(tui.ColorFg).Render(h.Name)
	dir := lipgloss.NewStyle().Foreground(tui.ColorFg).Render(h.ProjectDir)
	return style.Render(fmt.Sprintf("lazyharness · %s @ %s", name, dir)) + "\n"
}

func (m Model) renderKeybar() string {
	hi, k := tui.StyleKeyHi, tui.StyleHelp
	switch m.state {
	case stateNewRole:
		return tui.StyleKeybar.Width(m.width).Render(
			hi.Render("enter") + k.Render(": crear rol  ") +
				hi.Render("esc") + k.Render(": cancelar"),
		)
	case stateImproving:
		return tui.StyleKeybar.Width(m.width).Render(
			k.Render("mejorando con IA…  ") +
				hi.Render("esc") + k.Render(": cancelar"),
		)
	case stateShowSuggestion:
		return tui.StyleKeybar.Width(m.width).Render(
			hi.Render("y") + k.Render(": aceptar sugerencia  ") +
				hi.Render("n") + k.Render(": rechazar  ") +
				hi.Render("esc") + k.Render(": cancelar"),
		)
	default:
		return tui.StyleKeybar.Width(m.width).Render(
			hi.Render("n") + k.Render(": nuevo rol  ") +
				hi.Render("e") + k.Render(": editar  ") +
				hi.Render("m") + k.Render(": mejorar  ") +
				hi.Render("h") + k.Render(": historial  ") +
				hi.Render("i") + k.Render(": invocar  ") +
				hi.Render("t") + k.Render(": tareas  ") +
				hi.Render("s") + k.Render(": commit  ") +
				hi.Render("esc") + k.Render(": volver  ") +
				hi.Render("q") + k.Render(": salir"),
		)
	}
}

func (m Model) HarnessSummary() domain.HarnessSummary {
	return m.harness.Summary(nil)
}

func (m Model) currentRole() domain.Role {
	if len(m.harness.Roles) == 0 || m.selectedIdx >= len(m.harness.Roles) {
		return domain.Role{}
	}
	return m.harness.Roles[m.selectedIdx]
}

func leftWidth(w int) int      { return w * 26 / 100 }
func rightWidth(w int) int     { return w - leftWidth(w) - 2 }
func rolesHeight(h int) int    { return h * 60 / 100 }
func workflowHeight(h int) int { return h - rolesHeight(h) - 4 }
func promptHeight(h int) int   { return h * 88 / 100 }
func rightDims(w, h int) (int, int) {
	return rightWidth(w) - 4, promptHeight(h) - 4
}
