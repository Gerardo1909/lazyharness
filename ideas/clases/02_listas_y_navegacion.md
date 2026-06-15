# Clase 02: Listas y navegacion — la pantalla Home

> Al terminar esta clase vas a tener la pantalla Home del mockup 01: lista de harnesses a la izquierda, detalle a la derecha, barra de atajos abajo. Navegacion con j/k y busqueda fuzzy.

## Prerequisitos

- Haber completado [Clase 00](./00_go_para_pythonistas.md) (structs, JSON, tests) y [Clase 01](./01_hola_bubbletea.md) (bubbletea, Model/Update/View, lipgloss).
- Dependencias instaladas: bubbletea, lipgloss.

## Conceptos de Go que vas a aprender

### 1. El componente list de bubbles

bubbles es la libreria de componentes pre-hechos para bubbletea. El componente `list` te da una lista navegable con:
- Navegacion con j/k (o flechas)
- Busqueda fuzzy built-in (presiona `/`)
- Paginacion automatica
- Delegado de renderizado personalizable

```bash
go get github.com/charmbracelet/bubbles
```

Para usar `list`, tus items deben implementar la interface `list.Item`:

```go
type Item interface {
    FilterValue() string  // el texto que se usa para filtrar/buscar
}
```

**Equivalente Python:** es como usar un widget de lista de `textual` o `urwid`, pero en vez de herencia, implementas una interface de un solo metodo.

**Tradeoff:** bubbles/list te da mucho gratis (filtering, pagination, keybindings), pero te ata a su modelo de datos y renderizado. Si necesitas algo muy custom (ej: un arbol con nodos colapsables), te conviene construir desde cero con bubbletea puro. Para una lista de harnesses, el componente list es perfecto.

### 2. Layout con lipgloss — JoinHorizontal / JoinVertical

lipgloss tiene funciones para componer layouts:

```go
// Unir horizontalmente (columnas)
row := lipgloss.JoinHorizontal(lipgloss.Top,
    leftPanel,
    rightPanel,
)

// Unir verticalmente (filas)
page := lipgloss.JoinVertical(lipgloss.Left,
    header,
    content,
    footer,
)
```

**Equivalente Python / CSS:**

```python
# Es como flexbox:
# JoinHorizontal = display: flex; flex-direction: row
# JoinVertical   = display: flex; flex-direction: column
```

El primer argumento es la alineacion (`lipgloss.Top`, `lipgloss.Center`, `lipgloss.Bottom` para horizontal; `lipgloss.Left`, `lipgloss.Center`, `lipgloss.Right` para vertical).

**Tradeoff:** lipgloss no tiene layout automatico. Vos calculas los anchos manualmente a partir del `tea.WindowSizeMsg`. Es mas trabajo que CSS flexbox pero tambien mas predecible — no hay sorpresas de overflow o margin collapse.

```go
// Ejemplo: sidebar de 30% del ancho, panel principal de 70%
sidebarWidth := m.width * 30 / 100
mainWidth := m.width - sidebarWidth

sidebar := lipgloss.NewStyle().Width(sidebarWidth).Render(sidebarContent)
main := lipgloss.NewStyle().Width(mainWidth).Render(mainContent)

layout := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main)
```

### 3. Keybindings centralizados

bubbles tiene un sistema de keybindings con `key.Binding`:

```go
import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
    Up     key.Binding
    Down   key.Binding
    Enter  key.Binding
    New    key.Binding
    Delete key.Binding
    Quit   key.Binding
    Help   key.Binding
}

var keys = keyMap{
    Up: key.NewBinding(
        key.WithKeys("up", "k"),
        key.WithHelp("↑/k", "subir"),
    ),
    Down: key.NewBinding(
        key.WithKeys("down", "j"),
        key.WithHelp("↓/j", "bajar"),
    ),
    Enter: key.NewBinding(
        key.WithKeys("enter"),
        key.WithHelp("enter", "abrir"),
    ),
    New: key.NewBinding(
        key.WithKeys("n"),
        key.WithHelp("n", "nuevo"),
    ),
    Delete: key.NewBinding(
        key.WithKeys("d"),
        key.WithHelp("d", "eliminar"),
    ),
    Quit: key.NewBinding(
        key.WithKeys("q", "ctrl+c"),
        key.WithHelp("q", "salir"),
    ),
    Help: key.NewBinding(
        key.WithKeys("?"),
        key.WithHelp("?", "ayuda"),
    ),
}
```

**Tradeoff:** centralizar keybindings en un solo lugar (como hace lazygit) vs definirlos por pantalla. Centralizarlos evita conflictos y hace facil mostrar la barra de ayuda. Definirlos por pantalla permite que cada vista tenga atajos distintos. **Solucion: ambos.** Un keyMap global para atajos comunes (quit, help) y keyMaps por pantalla para atajos especificos.

### 4. Slices — operaciones comunes

En Go, los slices (equivalente a listas de Python) son el tipo de coleccion mas usado:

```go
// Filtrar
var active []Harness
for _, h := range allHarnesses {
    if h.HasRoles() {
        active = append(active, h)
    }
}

// Mapear (convertir a otro tipo)
items := make([]list.Item, len(harnesses))
for i, h := range harnesses {
    items[i] = harnessItem{h}
}

// Buscar
func find(harnesses []Harness, name string) (Harness, bool) {
    for _, h := range harnesses {
        if h.Name == name {
            return h, true
        }
    }
    return Harness{}, false
}
```

**Equivalente Python:**

```python
active = [h for h in all_harnesses if h.has_roles()]
items = [HarnessItem(h) for h in harnesses]
found = next((h for h in harnesses if h.name == name), None)
```

Go es mas verboso — no hay comprehensions. Pero cada operacion es explicita y el rendimiento es predecible (no hay generadores lazy ni overhead de iteradores).

**Tip sobre `make`:** `make([]T, len)` crea un slice con longitud predefinida. Es mas eficiente que `append` repetido porque evita re-allocations. Usalo cuando sabes el tamaño de antemano.

## Lo que vamos a construir

La pantalla Home del mockup 01:

```
┌─ Harnesses ──────────────┬─ Detalle ─────────────────────────────┐
│                          │                                       │
│ > dev-flow          ~/…  │ Nombre: dev-flow                      │
│   data-pipeline     ~/…  │ Proyecto: ~/dev/shop-api              │
│   blog-writer       ~/…  │ Formato: xml                          │
│   infra-ops         ~/…  │ Provider: anthropic/claude-opus-4-7   │
│                          │                                       │
│                          │ Roles (5):                             │
│                          │   ● arquitecto                        │
│                          │   ● code-reviewer                     │
│                          │   ● dev-backend                       │
│                          │   ● dev-frontend                      │
│                          │   ● docs                              │
│                          │                                       │
│                          │ Workflow: arquitecto → reviewer → …   │
│                          │ Ultimo commit: abc1234 hace 2h        │
│                          │                                       │
├──────────────────────────┴───────────────────────────────────────┤
│ n nuevo  enter abrir  d eliminar  ↑↓/jk navegar  ? ayuda  q salir│
└──────────────────────────────────────────────────────────────────┘
```

**Archivos a crear:**
- `internal/tui/keys.go` — keybindings centralizados
- `internal/tui/home/model.go` — Model/Update/View de la pantalla Home
- `internal/tui/components/keybar.go` — barra inferior de atajos
- `internal/tui/app.go` — modelo raiz (reemplaza la logica que estaba en main.go)

**Archivos a modificar:**
- `main.go` — ahora solo crea el programa y delega a `app.go`
- `internal/domain/harness.go` — agregar `HarnessSummary` si no existe

**Mockup de referencia:** `mockups/01_home_harnesses.svg`

## Implementacion paso a paso

### Paso 1: Crear los keybindings centralizados

Crea `internal/tui/keys.go`:

```go
package tui

import "github.com/charmbracelet/bubbles/key"

// GlobalKeys son atajos que funcionan en todas las pantallas
type GlobalKeys struct {
    Quit key.Binding
    Help key.Binding
}

var GlobalKeyMap = GlobalKeys{
    Quit: key.NewBinding(
        key.WithKeys("q", "ctrl+c"),
        key.WithHelp("q", "salir"),
    ),
    Help: key.NewBinding(
        key.WithKeys("?"),
        key.WithHelp("?", "ayuda"),
    ),
}

// HomeKeys son atajos especificos de la pantalla Home
type HomeKeys struct {
    Up     key.Binding
    Down   key.Binding
    Enter  key.Binding
    New    key.Binding
    Delete key.Binding
}

var HomeKeyMap = HomeKeys{
    Up: key.NewBinding(
        key.WithKeys("up", "k"),
        key.WithHelp("↑/k", "subir"),
    ),
    Down: key.NewBinding(
        key.WithKeys("down", "j"),
        key.WithHelp("↓/j", "bajar"),
    ),
    Enter: key.NewBinding(
        key.WithKeys("enter"),
        key.WithHelp("enter", "abrir"),
    ),
    New: key.NewBinding(
        key.WithKeys("n"),
        key.WithHelp("n", "nuevo"),
    ),
    Delete: key.NewBinding(
        key.WithKeys("d"),
        key.WithHelp("d", "eliminar"),
    ),
}
```

### Paso 2: Crear el componente keybar

Crea `internal/tui/components/keybar.go`:

```go
package components

import (
    "strings"

    "github.com/charmbracelet/bubbles/key"
    "github.com/charmbracelet/lipgloss"
    "github.com/Gerardo1909/lazyharness/internal/tui"
)

// KeyBarItem es un par tecla-descripcion para mostrar en la barra
type KeyBarItem struct {
    Key  string
    Desc string
}

// KeyBarFromBindings convierte key.Bindings en items para la barra
func KeyBarFromBindings(bindings ...key.Binding) []KeyBarItem {
    items := make([]KeyBarItem, 0, len(bindings))
    for _, b := range bindings {
        help := b.Help()
        items = append(items, KeyBarItem{Key: help.Key, Desc: help.Desc})
    }
    return items
}

// RenderKeyBar renderiza la barra de atajos en el ancho dado
func RenderKeyBar(items []KeyBarItem, width int) string {
    keyStyle := lipgloss.NewStyle().
        Bold(true).
        Foreground(tui.ColorFg).
        Background(tui.ColorSelection).
        Padding(0, 1)

    descStyle := lipgloss.NewStyle().
        Foreground(tui.ColorComment).
        MarginRight(2)

    var parts []string
    for _, item := range items {
        part := keyStyle.Render(item.Key) + " " + descStyle.Render(item.Desc)
        parts = append(parts, part)
    }

    bar := strings.Join(parts, "")

    return lipgloss.NewStyle().
        Width(width).
        Padding(0, 1).
        Render(bar)
}
```

### Paso 3: Crear el modelo de la pantalla Home

Crea el directorio y archivo `internal/tui/home/model.go`:

```go
package home

import (
    "fmt"
    "strings"

    "github.com/charmbracelet/bubbles/key"
    "github.com/charmbracelet/bubbles/list"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    maintui "github.com/Gerardo1909/lazyharness/internal/tui"
    "github.com/Gerardo1909/lazyharness/internal/tui/components"
    "github.com/Gerardo1909/lazyharness/internal/domain"
)

// harnessItem adapta HarnessSummary para la lista de bubbles
type harnessItem struct {
    summary domain.HarnessSummary
}

func (i harnessItem) Title() string       { return i.summary.Name }
func (i harnessItem) Description() string { return i.summary.ProjectDir }
func (i harnessItem) FilterValue() string { return i.summary.Name }

// Model es el estado de la pantalla Home
type Model struct {
    list      list.Model
    summaries []domain.HarnessSummary
    width     int
    height    int
}

// NewModel crea la pantalla Home con los harnesses dados
func NewModel(summaries []domain.HarnessSummary) Model {
    items := make([]list.Item, len(summaries))
    for i, s := range summaries {
        items[i] = harnessItem{s}
    }

    l := list.New(items, list.NewDefaultDelegate(), 0, 0)
    l.Title = "Harnesses"
    l.SetShowStatusBar(false)
    l.SetFilteringEnabled(true)
    l.Styles.Title = maintui.StyleTitle

    return Model{
        list:      l,
        summaries: summaries,
    }
}

func (m Model) Init() tea.Cmd {
    return nil
}

// HarnessSelectedMsg se envia cuando el usuario presiona Enter en un harness
type HarnessSelectedMsg struct {
    Name string
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // No interceptar teclas si la lista esta filtrando
        if m.list.FilterState() == list.Filtering {
            break
        }
        switch {
        case key.Matches(msg, maintui.HomeKeyMap.Enter):
            if item, ok := m.list.SelectedItem().(harnessItem); ok {
                return m, func() tea.Msg {
                    return HarnessSelectedMsg{Name: item.summary.Name}
                }
            }
        case key.Matches(msg, maintui.GlobalKeyMap.Quit):
            return m, tea.Quit
        }
    }

    var cmd tea.Cmd
    m.list, cmd = m.list.Update(msg)
    return m, cmd
}

func (m Model) View() string {
    sidebarWidth := m.width * 35 / 100
    if sidebarWidth < 30 {
        sidebarWidth = 30
    }
    detailWidth := m.width - sidebarWidth - 4 // 4 por bordes

    // Panel izquierdo: lista de harnesses
    m.list.SetSize(sidebarWidth-2, m.height-4) // -2 por bordes, -4 por keybar
    sidebar := maintui.StyleActiveBorder.
        Width(sidebarWidth).
        Height(m.height - 3).
        Render(m.list.View())

    // Panel derecho: detalle del harness seleccionado
    detail := m.renderDetail(detailWidth, m.height-3)

    // Layout horizontal
    content := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, detail)

    // Barra de atajos inferior
    keyItems := components.KeyBarFromBindings(
        maintui.HomeKeyMap.New,
        maintui.HomeKeyMap.Enter,
        maintui.HomeKeyMap.Delete,
        maintui.HomeKeyMap.Up,
        maintui.GlobalKeyMap.Help,
        maintui.GlobalKeyMap.Quit,
    )
    keybar := components.RenderKeyBar(keyItems, m.width)

    return lipgloss.JoinVertical(lipgloss.Left, content, keybar)
}

func (m Model) renderDetail(width, height int) string {
    item, ok := m.list.SelectedItem().(harnessItem)
    if !ok {
        return maintui.StyleBorder.
            Width(width).
            Height(height).
            Render("Selecciona un harness")
    }

    s := item.summary

    var b strings.Builder
    fmt.Fprintf(&b, "%s\n", maintui.StyleTitle.Render("Detalle"))
    fmt.Fprintf(&b, "\n")
    fmt.Fprintf(&b, "Nombre:   %s\n", s.Name)
    fmt.Fprintf(&b, "Proyecto: %s\n", s.ProjectDir)

    if s.Provider != "" {
        fmt.Fprintf(&b, "Provider: %s\n", s.Provider)
    }

    fmt.Fprintf(&b, "Roles:    %d\n", s.RoleCount)

    if s.LastCommit != "" {
        fmt.Fprintf(&b, "\nUltimo commit: %s\n", s.LastCommit)
    }

    return maintui.StyleBorder.
        Width(width).
        Height(height).
        Render(b.String())
}

// SetSize actualiza las dimensiones de la pantalla Home
func (m *Model) SetSize(width, height int) {
    m.width = width
    m.height = height
}
```

### Paso 4: Crear el modelo raiz (app.go)

Crea `internal/tui/app.go`:

```go
package tui

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/Gerardo1909/lazyharness/internal/domain"
    "github.com/Gerardo1909/lazyharness/internal/tui/home"
)

type screen int

const (
    screenHome screen = iota
    // screenHarness — clase 04
    // screenEditor  — clase 06
)

// App es el modelo raiz que orquesta todas las pantallas
type App struct {
    currentScreen screen
    home          home.Model
    width         int
    height        int
    ready         bool
}

// NewApp crea la aplicacion con datos iniciales
func NewApp(summaries []domain.HarnessSummary) App {
    return App{
        currentScreen: screenHome,
        home:          home.NewModel(summaries),
    }
}

func (a App) Init() tea.Cmd {
    return nil
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        a.width = msg.Width
        a.height = msg.Height
        a.ready = true
        a.home.SetSize(msg.Width, msg.Height)
    case home.HarnessSelectedMsg:
        // Por ahora solo imprimimos — en clase 04 navegamos a la vista de harness
        _ = msg.Name
    }

    switch a.currentScreen {
    case screenHome:
        var cmd tea.Cmd
        a.home, cmd = a.home.Update(msg)
        return a, cmd
    }

    return a, nil
}

func (a App) View() string {
    if !a.ready {
        return "Cargando..."
    }

    switch a.currentScreen {
    case screenHome:
        return a.home.View()
    }

    return ""
}
```

### Paso 5: Actualizar main.go

```go
package main

import (
    "fmt"
    "os"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/Gerardo1909/lazyharness/internal/domain"
    "github.com/Gerardo1909/lazyharness/internal/tui"
)

func main() {
    // Datos fake por ahora — en la clase 03 leemos del filesystem
    summaries := []domain.HarnessSummary{
        {
            Name:       "dev-flow",
            ProjectDir: "~/dev/shop-api",
            RoleCount:  5,
            LastCommit: "abc1234 — refactor prompts (hace 2h)",
            Provider:   "anthropic/claude-opus-4-7",
        },
        {
            Name:       "data-pipeline",
            ProjectDir: "~/dev/etl",
            RoleCount:  3,
            LastCommit: "def5678 — add validation role (hace 1d)",
            Provider:   "anthropic/claude-sonnet-4-6",
        },
        {
            Name:       "blog-writer",
            ProjectDir: "~/dev/blog",
            RoleCount:  2,
            Provider:   "openai/gpt-4o",
        },
        {
            Name:       "infra-ops",
            ProjectDir: "~/dev/infra",
            RoleCount:  4,
            LastCommit: "789abcd — initial harness (hace 5d)",
            Provider:   "anthropic/claude-opus-4-7",
        },
    }

    app := tui.NewApp(summaries)
    p := tea.NewProgram(app, tea.WithAltScreen())

    if _, err := p.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}
```

### Paso 6: Crear los directorios necesarios y ejecutar

```bash
mkdir -p internal/tui/home internal/tui/components
go run .
```

Deberias ver la lista de harnesses a la izquierda, el detalle a la derecha, y la barra de atajos abajo. Proba:
- `j`/`k` o flechas para navegar
- `/` para buscar (escribi "data" y mira como filtra)
- `Esc` para salir del filtro
- `q` para cerrar

### Paso 7: Test de la pantalla Home

Crea `internal/tui/home/model_test.go`:

```go
package home

import (
    "strings"
    "testing"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/Gerardo1909/lazyharness/internal/domain"
)

func testSummaries() []domain.HarnessSummary {
    return []domain.HarnessSummary{
        {Name: "primero", ProjectDir: "~/a", RoleCount: 3},
        {Name: "segundo", ProjectDir: "~/b", RoleCount: 5},
        {Name: "tercero", ProjectDir: "~/c", RoleCount: 1},
    }
}

func TestHomeView_MuestraHarnesses(t *testing.T) {
    m := NewModel(testSummaries())
    m.SetSize(100, 30)

    view := m.View()

    if !strings.Contains(view, "primero") {
        t.Error("la vista deberia mostrar 'primero'")
    }
    if !strings.Contains(view, "segundo") {
        t.Error("la vista deberia mostrar 'segundo'")
    }
}

func TestHomeView_MuestraDetalle(t *testing.T) {
    m := NewModel(testSummaries())
    m.SetSize(100, 30)

    view := m.View()

    // El primer item esta seleccionado por default
    if !strings.Contains(view, "~/a") {
        t.Error("el detalle deberia mostrar el directorio del harness seleccionado")
    }
}

func TestHomeView_SinHarnesses(t *testing.T) {
    m := NewModel([]domain.HarnessSummary{})
    m.SetSize(100, 30)

    view := m.View()

    if !strings.Contains(view, "Selecciona") || !strings.Contains(view, "Harnesses") {
        // Deberia mostrar algun mensaje cuando no hay items
        t.Log("Vista sin harnesses:", view)
    }
}

func TestHomeUpdate_QuitOnQ(t *testing.T) {
    m := NewModel(testSummaries())
    m.SetSize(100, 30)

    _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

    if cmd == nil {
        t.Error("presionar 'q' deberia generar un comando de quit")
    }
}
```

```bash
go test ./internal/tui/home/ -v
```

## Tradeoffs y decisiones de diseno

### Decision 1: bubbles/list vs lista custom

Usamos el componente `list` de bubbles que nos da gratis:
- Navegacion con j/k/flechas
- Busqueda fuzzy con `/`
- Paginacion si hay muchos items
- Estilos configurables

**Alternativa:** construir nuestra propia lista desde cero. Tendriamos control total del renderizado pero escribiriamos ~200 lineas de logica de navegacion que ya existe.

**Cuando cambiar:** si necesitamos un arbol (roles jerarquicos en la sidebar de la vista de harness, [Clase 04](./04_vista_de_harness.md)), ahi si conviene una lista custom porque bubbles/list no soporta indentacion ni nodos colapsables.

### Decision 2: Layout con porcentajes fijos

El sidebar ocupa 35% del ancho (minimo 30 columnas). Es simple y funciona bien.

**Alternativa:** anchos adaptativos segun el contenido (medir el string mas largo). Mas elegante pero mas complejo y con edge cases (que pasa si un nombre de harness es muy largo?).

**Eficiencia:** calcular porcentajes es O(1). Medir strings es O(n). Para 4 harnesses no importa, pero es buena practica pensar en esto desde el principio.

### Decision 3: HarnessSelectedMsg como mensaje custom

Cuando el usuario presiona Enter, el modelo Home no navega directamente a la vista de harness. En cambio, emite un `HarnessSelectedMsg` que el `App` raiz recibe y decide que hacer.

**Por que:** separacion de responsabilidades. Home no sabe que existe la vista de harness. Home solo dice "el usuario eligio este harness" y el router decide a donde ir. Esto permite testear Home sin importar las demas pantallas.

**Equivalente Python:** es como emitir un evento custom en un EventEmitter. El componente no decide la navegacion, solo emite la intencion.

### Decision 4: strings.Builder vs concatenacion

En `renderDetail` usamos `strings.Builder`:

```go
var b strings.Builder
fmt.Fprintf(&b, "Nombre: %s\n", name)
```

**Alternativa:** concatenar strings con `+`:

```go
result := "Nombre: " + name + "\n"
```

**Eficiencia:** la concatenacion con `+` crea un string nuevo por cada operacion (inmutabilidad de strings en Go, igual que en Python). `strings.Builder` acumula en un buffer y crea un solo string al final. Para 5-10 lineas la diferencia es despreciable, pero `Builder` es la practica idiomatica de Go para construir strings largos.

## Errores comunes y tips

### Error: "FilterValue() not found"

Si tus items de lista no implementan `FilterValue() string`, no compilan con `list.New`. Es el unico metodo requerido de `list.Item`. Si usas `list.NewDefaultDelegate()`, tambien necesitas `Title() string` y `Description() string`.

### Error: la lista no responde a las teclas

Si delegas el `Update` a la lista pero sigues interceptando la tecla antes:

```go
// MAL — interceptas "j" antes de que llegue a la lista
case key.Matches(msg, keys.Down):
    // haces algo manual
    break
```

La solucion: deja que la lista maneje j/k internamente. Solo intercepta teclas que la lista no maneja (como Enter para navegar, n para crear).

### Error: el panel se desborda

Si el texto del detalle es mas largo que el panel, se desborda visualmente. Solucion: usa `viewport` de bubbles para hacer el contenido scrolleable. Lo veremos en la [Clase 04](./04_vista_de_harness.md).

### Tip: mover el cursor a la pantalla completa con AltScreen

Si la app no ocupa toda la terminal, verifica que estas usando `tea.WithAltScreen()` en `tea.NewProgram`. Sin esto, la app se renderiza inline.

### Tip: depurar con `tea.LogToFile`

bubbletea puede loguear a un archivo para depuracion:

```go
f, _ := tea.LogToFile("debug.log", "debug")
defer f.Close()
```

Esto es invaluable cuando algo no se renderiza bien. Abre otra terminal y hace `tail -f debug.log` mientras usas la app.

## Ejercicios

### 1. Basico: agregar indicador de roles

En la lista de la izquierda, muestra la cantidad de roles al lado del nombre: `dev-flow (5 roles)`. Para esto, modifica `harnessItem.Description()`.

### 2. Intermedio: estilos del item seleccionado

Personaliza el delegate de la lista para que el item seleccionado tenga fondo azul oscuro (como en el mockup). Busca `list.NewDefaultDelegate()` y mira como se configura el estilo de seleccion.

```go
delegate := list.NewDefaultDelegate()
delegate.Styles.SelectedTitle = lipgloss.NewStyle().
    Foreground(tui.ColorFg).
    Background(tui.ColorSelection).
    Bold(true)
```

### 3. Avanzado: panel de detalle con roles coloreados

Extiende `HarnessSummary` para incluir una lista de `RoleSummary` con nombre y color. En el panel de detalle, renderiza cada rol con su color usando lipgloss:

```go
type RoleSummary struct {
    Name  string
    Color string
}

// En renderDetail:
for _, role := range s.Roles {
    color := lipgloss.Color(role.Color)
    bullet := lipgloss.NewStyle().Foreground(color).Render("●")
    fmt.Fprintf(&b, "  %s %s\n", bullet, role.Name)
}
```

### 4. Bonus: responsive layout

Haz que cuando la terminal sea muy angosta (< 80 columnas), el layout cambie a una sola columna (la lista arriba, el detalle abajo) en vez de dos columnas lado a lado. Esto es como un media query de CSS pero manual.

## Para profundizar

- [bubbles/list examples](https://github.com/charmbracelet/bubbles/tree/master/list): el componente que usamos, con ejemplos de customizacion.
- [lipgloss layout examples](https://github.com/charmbracelet/lipgloss#layout): JoinHorizontal, JoinVertical, Place.
- [lazygit source — context management](https://github.com/jesseduffield/lazygit): mira como lazygit maneja las multiples pantallas y paneles.
- [bubbletea — composing models](https://github.com/charmbracelet/bubbletea#composing-models): patron oficial para componer multiples modelos en una app.
- [Go by Example — Slices](https://gobyexample.com/slices): referencia rapida de operaciones con slices.

## Que sigue

En la [Clase 03](./03_filesystem_y_json.md) reemplazamos los datos hardcodeados por lectura real del filesystem. Vas a aprender a leer directorios, parsear JSON, crear archivos, y manejar errores de I/O — todo lo que necesitas para que la app trabaje con harnesses reales en disco.
