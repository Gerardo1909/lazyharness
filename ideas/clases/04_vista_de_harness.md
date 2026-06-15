# Clase 04: La vista de harness — composicion de paneles

> Al terminar esta clase, al seleccionar un harness en Home se navega a la vista de harness con sidebar de roles, prompt read-only con @referencias coloreadas, y barra de acciones. Esc vuelve a Home.

## Prerequisitos

- [Clase 02](./02_listas_y_navegacion.md): pantalla Home, app.go con routing basico.
- [Clase 03](./03_filesystem_y_json.md): storage para leer harnesses reales.

## Conceptos de Go que vas a aprender

### 1. Composicion de modelos en bubbletea

Una app compleja tiene multiples sub-modelos. Cada pantalla es un Model que el App raiz orquesta:

```go
type App struct {
    currentScreen screen
    home          home.Model
    harness       harness.Model  // nuevo en esta clase
    width, height int
}
```

Cada sub-modelo tiene su propio Init/Update/View. El App raiz decide cual esta activo y le delega los mensajes.

**Equivalente Python:** es como componentes en React o widgets en textual — cada uno maneja su propio estado, el padre decide cual se renderiza.

**Tradeoff composicion vs un solo model gigante:** un solo model es mas simple (todo en un lugar) pero no escala. Con 5 pantallas, un model unico tendria 50+ campos. Separar en sub-modelos permite desarrollar y testear cada pantalla independientemente.

### 2. Routing con state machine

El routing entre pantallas es un switch simple:

```go
type screen int

const (
    screenHome screen = iota
    screenHarness
    screenEditor
    screenHistory
    screenWorkspace
)
```

`iota` es el mecanismo de Go para enums auto-incrementales:

```go
const (
    a = iota  // 0
    b         // 1
    c         // 2
)
```

**Equivalente Python:** `enum.IntEnum` con `auto()`:

```python
class Screen(IntEnum):
    HOME = auto()
    HARNESS = auto()
```

**Tradeoff enum vs stack:** lazygit usa un stack de contextos (push al entrar, pop al salir). Es mas flexible para navegacion profunda (A → B → C → B → A). Un enum simple funciona para lazyharness donde la navegacion es plana (Home ↔ Harness ↔ Editor). Si la navegacion se vuelve mas profunda, migramos a stack.

### 3. Viewport — contenido scrolleable

El componente `viewport` de bubbles es un panel que puede scrollear contenido largo:

```go
import "github.com/charmbracelet/bubbles/viewport"

vp := viewport.New(80, 20)       // ancho, alto
vp.SetContent(longString)        // el contenido a mostrar
vp.GotoTop()                     // scrollear al inicio

// En Update:
vp, cmd = vp.Update(msg)         // maneja scroll con j/k/PageUp/PageDown

// En View:
output := vp.View()              // retorna solo las lineas visibles
```

**Tradeoff viewport vs strings.Builder sin scroll:** para un prompt de 20 lineas, no hace falta scroll. Para uno de 200, si. El viewport es la opcion correcta por default — no agrega complejidad y funciona para cualquier largo.

### 4. Strings y regexp para @referencias

Los prompts pueden contener `@nombre-de-rol`. Necesitamos detectarlos y colorearlos:

```go
import "regexp"

var refPattern = regexp.MustCompile(`@([\w-]+)`)

func colorizeRefs(text string, roles map[string]string) string {
    return refPattern.ReplaceAllStringFunc(text, func(match string) string {
        name := match[1:]  // quitar el @
        if color, ok := roles[name]; ok {
            style := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true)
            return style.Render(match)
        }
        return match
    })
}
```

**Equivalente Python:**

```python
import re
def colorize_refs(text, roles):
    def replace(m):
        name = m.group(1)
        if name in roles:
            return f"\033[1;38;2;...m@{name}\033[0m"  # ANSI
        return m.group(0)
    return re.sub(r'@([\w-]+)', replace, text)
```

**Tradeoff regex vs scan manual:** regex es mas expresivo pero tiene overhead de compilacion. Para un patron simple como `@palabra`, un scan manual con `strings.Index` seria mas rapido. Pero `regexp.MustCompile` compila una vez (a nivel de package) y despues cada `ReplaceAllStringFunc` es O(n) donde n es el largo del texto. Para prompts (<10KB), la diferencia es imperceptible.

**Go vs Python en regex:** Go usa RE2 (no backtracking, siempre O(n)). Python usa PCRE (backtracking, puede ser O(2^n) con patrones patologicos). Para nuestro uso son equivalentes, pero es bueno saber que Go no tiene lookbehinds ni backreferences.

## Lo que vamos a construir

La vista de harness del mockup 02:

```
┌─ dev-flow ── ~/dev/shop-api ── limpio ── abc1234 ──────────────────┐
│                                                                     │
│ ┌─ Roles ──────────┐ ┌─ arquitecto.xml (read-only) ─────────────┐ │
│ │                   │ │                                           │ │
│ │ ▸ arquitecto      │ │ <role>                                   │ │
│ │   ├ code-reviewer │ │   Sos el arquitecto del proyecto.        │ │
│ │   ├ dev-backend   │ │   Tu responsabilidad es disenar...       │ │
│ │   └ dev-frontend  │ │ </role>                                  │ │
│ │ ▸ docs            │ │ <constraints>                            │ │
│ │                   │ │   Consulta con @code-reviewer antes...   │ │
│ │                   │ │ </constraints>                           │ │
│ │                   │ │                                           │ │
│ └───────────────────┘ └───────────────────────────────────────────┘ │
│                                                                     │
│  e editar  x eliminar  m mejorar  h historial  i invocar           │
├─────────────────────────────────────────────────────────────────────┤
│  ↑↓/jk navegar  tab panel  s guardar  t tareas  esc volver  q salir│
└─────────────────────────────────────────────────────────────────────┘
```

**Archivos a crear:**
- `internal/tui/harness/model.go`
- `internal/tui/components/sidebar.go`
- `internal/tui/components/promptview.go`
- `internal/tui/components/actionbar.go`

**Archivos a modificar:**
- `internal/tui/app.go` — agregar routing a la vista de harness
- `internal/tui/keys.go` — agregar HarnessKeys

## Implementacion paso a paso

### Paso 1: Agregar keybindings de la vista harness

Agrega a `internal/tui/keys.go`:

```go
// HarnessKeys son atajos de la vista de harness
type HarnessKeys struct {
    Up      key.Binding
    Down    key.Binding
    Tab     key.Binding
    Edit    key.Binding
    Delete  key.Binding
    Improve key.Binding
    History key.Binding
    Invoke  key.Binding
    Save    key.Binding
    Tasks   key.Binding
    Back    key.Binding
}

var HarnessKeyMap = HarnessKeys{
    Up: key.NewBinding(
        key.WithKeys("up", "k"),
        key.WithHelp("↑/k", "subir"),
    ),
    Down: key.NewBinding(
        key.WithKeys("down", "j"),
        key.WithHelp("↓/j", "bajar"),
    ),
    Tab: key.NewBinding(
        key.WithKeys("tab"),
        key.WithHelp("tab", "panel"),
    ),
    Edit: key.NewBinding(
        key.WithKeys("e"),
        key.WithHelp("e", "editar"),
    ),
    Delete: key.NewBinding(
        key.WithKeys("x"),
        key.WithHelp("x", "eliminar"),
    ),
    Improve: key.NewBinding(
        key.WithKeys("m"),
        key.WithHelp("m", "mejorar"),
    ),
    History: key.NewBinding(
        key.WithKeys("h"),
        key.WithHelp("h", "historial"),
    ),
    Invoke: key.NewBinding(
        key.WithKeys("i"),
        key.WithHelp("i", "invocar"),
    ),
    Save: key.NewBinding(
        key.WithKeys("s"),
        key.WithHelp("s", "guardar"),
    ),
    Tasks: key.NewBinding(
        key.WithKeys("t"),
        key.WithHelp("t", "tareas"),
    ),
    Back: key.NewBinding(
        key.WithKeys("esc"),
        key.WithHelp("esc", "volver"),
    ),
}
```

### Paso 2: Crear la sidebar de roles

Crea `internal/tui/components/sidebar.go`:

```go
package components

import (
    "fmt"
    "strings"

    "github.com/charmbracelet/lipgloss"
    "github.com/Gerardo1909/lazyharness/internal/domain"
    "github.com/Gerardo1909/lazyharness/internal/tui"
)

// RoleSidebar renderiza la lista de roles con jerarquia de arbol
type RoleSidebar struct {
    Roles    []domain.Role
    Selected int
    Width    int
    Height   int
}

func (s RoleSidebar) View() string {
    var b strings.Builder

    // Construir el arbol: roles raiz y sus hijos
    type node struct {
        role     domain.Role
        children []domain.Role
    }

    var roots []node
    childMap := make(map[string][]domain.Role)

    for _, r := range s.Roles {
        if r.Parent == "" {
            roots = append(roots, node{role: r})
        } else {
            childMap[r.Parent] = append(childMap[r.Parent], r)
        }
    }
    for i := range roots {
        roots[i].children = childMap[roots[i].role.Name]
    }

    // Renderizar
    idx := 0
    for _, root := range roots {
        line := s.renderRole(root.role, idx, "▸ ")
        b.WriteString(line + "\n")
        idx++

        for ci, child := range root.children {
            prefix := "  ├ "
            if ci == len(root.children)-1 {
                prefix = "  └ "
            }
            line := s.renderRole(child, idx, prefix)
            b.WriteString(line + "\n")
            idx++
        }
    }

    content := b.String()

    return tui.StyleBorder.
        Width(s.Width).
        Height(s.Height).
        Render(content)
}

func (s RoleSidebar) renderRole(r domain.Role, idx int, prefix string) string {
    color := r.Color
    if color == "" {
        color = domain.DefaultColor
    }

    nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
    if idx == s.Selected {
        nameStyle = nameStyle.Bold(true).Background(lipgloss.Color(tui.ColorSelection.String()))
    }

    return fmt.Sprintf("%s%s", prefix, nameStyle.Render(r.Name))
}
```

### Paso 3: Crear el prompt viewer

Crea `internal/tui/components/promptview.go`:

```go
package components

import (
    "regexp"

    "github.com/charmbracelet/bubbles/viewport"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/Gerardo1909/lazyharness/internal/domain"
    "github.com/Gerardo1909/lazyharness/internal/tui"
)

var refPattern = regexp.MustCompile(`@([\w-]+)`)

// PromptView muestra el contenido de un prompt en un viewport scrolleable
type PromptView struct {
    viewport viewport.Model
    title    string
    width    int
    height   int
}

// NewPromptView crea un viewer con el contenido dado
func NewPromptView(title, content string, roles []domain.Role, width, height int) PromptView {
    vp := viewport.New(width-2, height-3) // -2 bordes, -3 titulo+bordes
    colored := colorizeRefs(content, roles)
    vp.SetContent(colored)

    return PromptView{
        viewport: vp,
        title:    title,
        width:    width,
        height:   height,
    }
}

func (p *PromptView) Update(msg tea.Msg) (*PromptView, tea.Cmd) {
    var cmd tea.Cmd
    p.viewport, cmd = p.viewport.Update(msg)
    return p, cmd
}

func (p PromptView) View() string {
    header := tui.StyleTitle.Render(p.title) + " " +
        tui.StyleComment().Render("(read-only)")

    content := lipgloss.JoinVertical(lipgloss.Left, header, p.viewport.View())

    return tui.StyleBorder.
        Width(p.width).
        Height(p.height).
        Render(content)
}

func colorizeRefs(text string, roles []domain.Role) string {
    roleColors := make(map[string]string)
    for _, r := range roles {
        color := r.Color
        if color == "" {
            color = domain.DefaultColor
        }
        roleColors[r.Name] = color
    }

    return refPattern.ReplaceAllStringFunc(text, func(match string) string {
        name := match[1:]
        if color, ok := roleColors[name]; ok {
            style := lipgloss.NewStyle().
                Foreground(lipgloss.Color(color)).
                Bold(true)
            return style.Render(match)
        }
        return match
    })
}
```

Agrega el helper que falta en `theme.go`:

```go
// En internal/tui/theme.go, agregar:
func StyleComment() lipgloss.Style {
    return lipgloss.NewStyle().Foreground(ColorComment)
}
```

### Paso 4: Crear la action bar

Crea `internal/tui/components/actionbar.go`:

```go
package components

import (
    "github.com/charmbracelet/bubbles/key"
    "github.com/charmbracelet/lipgloss"
    "github.com/Gerardo1909/lazyharness/internal/tui"
)

// Action es una accion disponible en la barra con su tooltip
type Action struct {
    Binding key.Binding
    Tooltip string
}

// ActionBar renderiza acciones con tooltip de la accion seleccionada
type ActionBar struct {
    Actions  []Action
    Selected int
    Width    int
}

func (a ActionBar) View() string {
    keyStyle := lipgloss.NewStyle().
        Bold(true).
        Foreground(tui.ColorFg).
        Background(tui.ColorSelection).
        Padding(0, 1)

    descStyle := lipgloss.NewStyle().
        Foreground(tui.ColorComment).
        MarginRight(1)

    selectedStyle := lipgloss.NewStyle().
        Bold(true).
        Foreground(tui.ColorBlue).
        Background(tui.ColorSelection).
        Padding(0, 1)

    var parts string
    for i, action := range a.Actions {
        help := action.Binding.Help()
        ks := keyStyle
        if i == a.Selected {
            ks = selectedStyle
        }
        parts += ks.Render(help.Key) + " " + descStyle.Render(help.Desc)
    }

    // Tooltip de la accion seleccionada
    tooltip := ""
    if a.Selected >= 0 && a.Selected < len(a.Actions) {
        tooltip = tui.StyleComment().
            Italic(true).
            Render("  " + a.Actions[a.Selected].Tooltip)
    }

    bar := lipgloss.JoinVertical(lipgloss.Left, parts, tooltip)

    return lipgloss.NewStyle().Width(a.Width).Render(bar)
}
```

### Paso 5: Crear el modelo de la vista de harness

Crea `internal/tui/harness/model.go`:

```go
package harness

import (
    "github.com/charmbracelet/bubbles/key"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    maintui "github.com/Gerardo1909/lazyharness/internal/tui"
    "github.com/Gerardo1909/lazyharness/internal/tui/components"
    "github.com/Gerardo1909/lazyharness/internal/domain"
)

type panel int

const (
    panelSidebar panel = iota
    panelPrompt
)

// BackToHomeMsg se envia cuando el usuario presiona Esc
type BackToHomeMsg struct{}

// Model es el estado de la vista de harness
type Model struct {
    harness      domain.Harness
    promptContent string
    activePanel  panel
    selectedRole int
    width        int
    height       int
    promptView   *components.PromptView
}

func NewModel(h domain.Harness, promptContent string) Model {
    return Model{
        harness:       h,
        promptContent: promptContent,
        activePanel:   panelSidebar,
        selectedRole:  0,
    }
}

func (m Model) Init() tea.Cmd {
    return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch {
        case key.Matches(msg, maintui.HarnessKeyMap.Back):
            return m, func() tea.Msg { return BackToHomeMsg{} }

        case key.Matches(msg, maintui.HarnessKeyMap.Tab):
            if m.activePanel == panelSidebar {
                m.activePanel = panelPrompt
            } else {
                m.activePanel = panelSidebar
            }

        case key.Matches(msg, maintui.HarnessKeyMap.Up):
            if m.activePanel == panelSidebar && m.selectedRole > 0 {
                m.selectedRole--
                m.updatePromptView()
            }

        case key.Matches(msg, maintui.HarnessKeyMap.Down):
            if m.activePanel == panelSidebar && m.selectedRole < len(m.harness.Roles)-1 {
                m.selectedRole++
                m.updatePromptView()
            }

        case key.Matches(msg, maintui.GlobalKeyMap.Quit):
            return m, tea.Quit
        }
    }

    // Delegar al prompt view si esta activo
    if m.activePanel == panelPrompt && m.promptView != nil {
        var cmd tea.Cmd
        m.promptView, cmd = m.promptView.Update(msg)
        return m, cmd
    }

    return m, nil
}

func (m *Model) updatePromptView() {
    if len(m.harness.Roles) == 0 {
        return
    }
    // Nota: el contenido real se cargaria del filesystem
    // Por ahora usamos el promptContent pasado al modelo
    sidebarWidth := m.width * 25 / 100
    promptWidth := m.width - sidebarWidth - 2
    pv := components.NewPromptView(
        m.harness.Roles[m.selectedRole].PromptFile,
        m.promptContent,
        m.harness.Roles,
        promptWidth,
        m.height-8,
    )
    m.promptView = &pv
}

func (m Model) View() string {
    sidebarWidth := m.width * 25 / 100
    if sidebarWidth < 20 {
        sidebarWidth = 20
    }
    promptWidth := m.width - sidebarWidth - 2

    // Header
    header := maintui.StyleTitle.Render(m.harness.Name) + "  " +
        maintui.StyleComment().Render(m.harness.ProjectDir)

    // Sidebar
    sidebarStyle := maintui.StyleBorder
    if m.activePanel == panelSidebar {
        sidebarStyle = maintui.StyleActiveBorder
    }

    sidebar := components.RoleSidebar{
        Roles:    m.harness.Roles,
        Selected: m.selectedRole,
        Width:    sidebarWidth,
        Height:   m.height - 8,
    }

    // Prompt view
    promptPanel := ""
    if m.promptView != nil {
        promptPanel = m.promptView.View()
    } else {
        promptPanel = maintui.StyleBorder.
            Width(promptWidth).
            Height(m.height - 8).
            Render("Selecciona un rol")
    }

    content := lipgloss.JoinHorizontal(lipgloss.Top,
        sidebarStyle.Render(sidebar.View()),
        promptPanel,
    )

    // Action bar
    actions := components.ActionBar{
        Actions: []components.Action{
            {Binding: maintui.HarnessKeyMap.Edit, Tooltip: "Abrir el editor embebido para modificar este prompt"},
            {Binding: maintui.HarnessKeyMap.Delete, Tooltip: "Eliminar este rol del harness (pide confirmacion)"},
            {Binding: maintui.HarnessKeyMap.Improve, Tooltip: "Lanzar agente IA que revisa y alinea todos los prompts"},
            {Binding: maintui.HarnessKeyMap.History, Tooltip: "Ver historial de versiones de este rol"},
            {Binding: maintui.HarnessKeyMap.Invoke, Tooltip: "Invocar un CLI agentic con este prompt"},
        },
        Width: m.width,
    }

    // Keybar
    keyItems := components.KeyBarFromBindings(
        maintui.HarnessKeyMap.Up,
        maintui.HarnessKeyMap.Tab,
        maintui.HarnessKeyMap.Save,
        maintui.HarnessKeyMap.Tasks,
        maintui.HarnessKeyMap.Back,
        maintui.GlobalKeyMap.Quit,
    )
    keybar := components.RenderKeyBar(keyItems, m.width)

    return lipgloss.JoinVertical(lipgloss.Left,
        header,
        content,
        actions.View(),
        keybar,
    )
}

func (m *Model) SetSize(width, height int) {
    m.width = width
    m.height = height
    m.updatePromptView()
}
```

### Paso 6: Actualizar el routing en app.go

Modifica `internal/tui/app.go` para soportar navegacion Home → Harness:

```go
// Agregar al Update de App:
case home.HarnessSelectedMsg:
    // Cargar el harness seleccionado
    h, err := a.store.LoadHarness(msg.ProjectDir)
    if err != nil {
        // TODO: mostrar error en la UI
        break
    }
    content := "" // cargar prompt del primer rol
    if len(h.Roles) > 0 {
        content, _ = a.store.ReadPrompt(h.ProjectDir, h.Roles[0].PromptFile)
    }
    a.harness = harness.NewModel(h, content)
    a.harness.SetSize(a.width, a.height)
    a.currentScreen = screenHarness

case harness.BackToHomeMsg:
    a.currentScreen = screenHome
```

Y agregar el case en View:

```go
case screenHarness:
    return a.harness.View()
```

## Tradeoffs y decisiones de diseno

### Decision 1: Router por enum vs stack

Usamos un enum `screen` con un switch. Cada transicion es explicita.

**Alternativa stack:**

```go
type App struct {
    screens []screen // stack
}
func (a *App) push(s screen) { a.screens = append(a.screens, s) }
func (a *App) pop()          { a.screens = a.screens[:len(a.screens)-1] }
```

El stack es mejor para A → B → C → pop → B → pop → A. El enum es mas simple y suficiente para lazyharness donde la navegacion es Home ↔ Harness ↔ sub-vistas.

### Decision 2: Componentes reutilizables sin estado significativo

`RoleSidebar`, `ActionBar`, `KeyBar` no tienen estado propio — reciben datos y renderizan. El estado (selected index, roles, etc.) vive en el Model padre.

**Por que:** componentes stateless son predecibles y fáciles de testear. El padre controla todo. Es el patron de "presentational components" de React.

### Decision 3: Panel activo con Tab

Tab alterna entre sidebar y prompt view. El panel activo recibe las teclas de navegacion; el otro las ignora.

**Tradeoff:** con muchos paneles (3+), Tab ciclico puede ser confuso. Alternativa: `Ctrl+H/L` para ir a izquierda/derecha, `Tab` solo para el siguiente. Para 2 paneles, Tab es suficiente.

## Errores comunes y tips

### Error: imports circulares

Go NO permite imports circulares. Si `tui/harness` importa `tui/home`, y `tui/home` importa `tui/harness`, no compila.

**Solucion:** usar mensajes (como `HarnessSelectedMsg`, `BackToHomeMsg`) que el padre (`app.go`) maneja. Los hijos nunca se importan entre si — solo el padre los conoce.

### Error: viewport no scrollea

Si el viewport no responde a j/k, es porque los KeyMsg no le llegan. Verifica que cuando `activePanel == panelPrompt`, delegas el Update al prompt view.

### Tip: borders consumen espacio

Un borde `lipgloss.RoundedBorder()` agrega 2 caracteres de ancho y 2 de alto. Si calculas `Width(sidebarWidth)` y pones un borde, el contenido interno es `sidebarWidth - 2`. Tene esto en cuenta para evitar desbordamientos.

## Ejercicios

### 1. Basico: mostrar workflow en el header

Agrega el workflow como breadcrumb en el header: `arquitecto → code-reviewer → dev-backend`.

### 2. Intermedio: cargar prompt al cambiar de rol

Cuando el usuario navega por los roles con j/k, el prompt view deberia cargar el contenido del prompt del rol seleccionado desde disco. Necesitas pasar el `Store` al modelo de harness.

### 3. Avanzado: borde activo visual

Cambia el color del borde del panel activo a azul (`StyleActiveBorder`) y el inactivo a gris (`StyleBorder`). Esto da feedback visual de donde esta el foco.

### 4. Bonus: responsive sidebar

Si la terminal tiene menos de 100 columnas, colapsa la sidebar a solo iconos/iniciales (primera letra de cada rol con su color). Expandir con Tab.

## Para profundizar

- [bubbletea — Composing Models](https://github.com/charmbracelet/bubbletea#composing-models): patron oficial de composicion.
- [bubbles/viewport](https://github.com/charmbracelet/bubbles/tree/master/viewport): documentacion del componente viewport.
- [lazygit — contexts](https://github.com/jesseduffield/lazygit/tree/master/pkg/gui/context): como lazygit maneja la navegacion entre paneles.
- [Go by Example — Interfaces](https://gobyexample.com/interfaces): profundizar en interfaces implicitas.
- [Effective Go — Embedding](https://go.dev/doc/effective_go#embedding): composicion de structs.

## Que sigue

En la [Clase 05](./05_git_desde_go.md) agregamos versionado automatico con git. Cada vez que guardas un prompt, se crea un commit en el repo del harness. Vas a aprender go-git, interfaces como boundaries de testing, y goroutines para operaciones que pueden tardar.
