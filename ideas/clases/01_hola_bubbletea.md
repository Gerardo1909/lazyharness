# Clase 01: Hola bubbletea — tu primera TUI

> Al terminar esta clase vas a tener una TUI fullscreen que muestra "lazyharness v0.1.0" con colores Tokyo Night, responde al resize de la terminal y se cierra con `q`.

## Prerequisitos

- Haber completado la [Clase 00](./00_go_para_pythonistas.md): sabes crear structs, tests y correr `go run .`.
- Proyecto inicializado con `go mod init`.

## Conceptos de Go que vas a aprender

### 1. Interfaces — el mecanismo mas poderoso de Go

En Python, el duck typing funciona asi: "si camina como pato y hace cuac como pato, es un pato". No declaras que algo implementa una interfaz — solo lo usas y si falla, falla en runtime.

Go tiene duck typing pero verificado en compilacion. Se llaman **interfaces implicitas**:

```go
// Defines una interface: un contrato de metodos
type Displayable interface {
    DisplayName() string
}

// Cualquier struct que tenga DisplayName() string la implementa AUTOMATICAMENTE
// No hay "implements", no hay herencia, no hay decoradores
type Role struct {
    Name  string
    Color string
}

func (r Role) DisplayName() string {
    return fmt.Sprintf("[%s] %s", r.Color, r.Name)
}

// Role implementa Displayable sin declararlo explicitamente.
// Si le sacas el metodo DisplayName, deja de implementarla — error de compilacion.
```

**Tradeoff vs Python:** en Python, si un metodo falta, lo descubris en runtime. En Go, el compilador te dice "Role no implementa Displayable: falta el metodo X" antes de que ejecutes. Menos flexibilidad, mas seguridad.

**Tradeoff vs Java:** en Java escribis `class Role implements Displayable`. En Go no — la interface se satisface implicitamente. Esto permite que interfaces como `io.Reader` funcionen con cualquier struct que tenga `Read([]byte) (int, error)`, sin que el autor del struct supiera que `io.Reader` existe.

La interface que nos importa ahora es `tea.Model`:

```go
type Model interface {
    Init() tea.Cmd
    Update(msg tea.Msg) (Model, tea.Cmd)
    View() string
}
```

Cualquier struct que tenga esos 3 metodos es un Model de bubbletea.

### 2. La arquitectura Elm — Model / Update / View

bubbletea implementa The Elm Architecture (TEA), un patron de UI funcional:

```
┌──────────────────────────────────────────┐
│                                          │
│  Evento (tecla, resize, tick, mensaje)   │
│         │                                │
│         ▼                                │
│  Update(msg) → nuevo Model               │
│         │                                │
│         ▼                                │
│  View(model) → string (lo que se ve)     │
│         │                                │
│         └──────── loop ──────────────────│
│                                          │
└──────────────────────────────────────────┘
```

1. **Model**: un struct con TODO el estado de la UI. Nada vive fuera del model.
2. **Update**: recibe el model actual + un mensaje (evento), retorna un model NUEVO. No muta — crea uno nuevo.
3. **View**: recibe el model, retorna un string que es exactamente lo que se renderiza en la terminal.

**Analogia Python:** imagina un framework donde tu UI es:

```python
# Pseudocodigo
state = {"screen": "home", "selected": 0}

def update(state, event):
    if event.key == "j":
        return {**state, "selected": state["selected"] + 1}
    return state

def view(state):
    return render_home(state) if state["screen"] == "home" else render_other(state)

while True:
    event = wait_for_input()
    state = update(state, event)
    print(view(state))
```

Es exactamente eso, pero tipado y con Go. El framework de Python `textual` usa un patron similar (reactivo, message-based).

**Por que funciona:** dado un modelo y un evento, SIEMPRE obtenes el mismo resultado. No hay side effects ocultos, no hay estado global, no hay mutaciones inesperadas. Esto hace que la TUI sea predecible y facil de testear.

**Tradeoff vs MVC:** MVC permite mutacion directa del modelo desde el controller, lo que es mas facil al principio pero genera bugs sutiles en UIs complejas. Elm te obliga a pasar por Update, lo cual es mas verboso pero mas seguro.

### 3. tea.Cmd y tea.Msg — side effects controlados

`Update` retorna `(Model, tea.Cmd)`. El `Cmd` es un side effect diferido — algo que bubbletea ejecuta POR VOS despues del update.

```go
// Un Cmd es una funcion que retorna un Msg
type Cmd func() Msg

// Ejemplo: un Cmd que lee un archivo
func readFileCmd(path string) tea.Cmd {
    return func() tea.Msg {
        data, err := os.ReadFile(path)
        if err != nil {
            return errMsg{err}
        }
        return fileLoadedMsg{data}
    }
}
```

**Por que no ejecutar el side effect directo en Update?** Porque Update debe ser rapido y puro. Si lees un archivo o haces un HTTP request adentro de Update, la UI se congela. Los Cmds se ejecutan en goroutines — la UI sigue respondiendo mientras el Cmd trabaja en background.

**Equivalente Python:** es como `asyncio.create_task()` pero manejado por el framework. Vos retornas el Cmd, bubbletea se encarga de ejecutarlo y mandarte el resultado como un Msg.

### 4. Type switches — pattern matching para mensajes

En Update, recibis un `tea.Msg` (interface vacia) y necesitas saber que tipo de mensaje es:

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        }
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    case fileLoadedMsg:
        m.content = string(msg.data)
    }
    return m, nil
}
```

**Equivalente Python:**

```python
match event:
    case KeyEvent(key="q"):
        return state, quit
    case ResizeEvent(width=w, height=h):
        state["width"] = w
```

El `switch msg := msg.(type)` es un "type assertion" que ademas asigna el valor al tipo correcto. Dentro del `case tea.KeyMsg:`, `msg` ya es de tipo `tea.KeyMsg`, no `tea.Msg`.

### 5. lipgloss — estilos para la terminal

lipgloss es como CSS inline para la terminal. Cada estilo es inmutable (como styled-components):

```go
import "github.com/charmbracelet/lipgloss"

// Definir un estilo
titleStyle := lipgloss.NewStyle().
    Bold(true).
    Foreground(lipgloss.Color("#7aa2f7")).
    Background(lipgloss.Color("#1a1b26")).
    Padding(1, 2)

// Aplicarlo a un string
styled := titleStyle.Render("lazyharness v0.1.0")
```

**Equivalente Python:** imagina `rich` de Will McGuigan pero con una API builder:

```python
# Rich
console.print("[bold blue on #1a1b26]lazyharness v0.1.0[/]")

# lipgloss es mas como:
# style = Style(bold=True, fg="#7aa2f7", bg="#1a1b26", padding=(1,2))
# styled = style.render("lazyharness v0.1.0")
```

**Tradeoff:** lipgloss no tiene cascading (no hay herencia de estilos como en CSS). Cada estilo es independiente. Esto es mas predecible pero requiere definir estilos completos para cada elemento.

Los estilos son inmutables — `style.Bold(true)` retorna un estilo NUEVO, no modifica el original. Esto permite componerlos sin efectos secundarios:

```go
base := lipgloss.NewStyle().Foreground(lipgloss.Color("#c0caf5"))
highlighted := base.Bold(true).Background(lipgloss.Color("#33467c"))
// base no cambio
```

## Lo que vamos a construir

Una TUI fullscreen que muestra:

```
┌────────────────────────────────────────┐
│                                        │
│         lazyharness v0.1.0             │
│                                        │
│     Tu harness de agentes,             │
│     directo desde la terminal.         │
│                                        │
│         (presiona q para salir)        │
│                                        │
└────────────────────────────────────────┘
```

Con colores Tokyo Night, centrado en la terminal, que responde al resize.

**Archivos a crear/modificar:**
- `internal/tui/theme.go` (nuevo)
- `main.go` (reescribir — ahora lanza bubbletea)

**Mockup de referencia:** todavia no corresponde a ningun mockup SVG — es el paso previo.

## Implementacion paso a paso

### Paso 1: Instalar las dependencias

```bash
cd ~/stuff/lazyharness

# El framework TUI
go get github.com/charmbracelet/bubbletea

# Estilos (colores, bordes, padding)
go get github.com/charmbracelet/lipgloss
```

Despues de esto, tu `go.mod` va a tener las dependencias listadas y `go.sum` va a tener los checksums.

### Paso 2: Crear el tema visual

Crea `internal/tui/theme.go`:

```go
package tui

import "github.com/charmbracelet/lipgloss"

// Paleta Tokyo Night
var (
    ColorBg        = lipgloss.Color("#1a1b26")
    ColorFg        = lipgloss.Color("#c0caf5")
    ColorBlue      = lipgloss.Color("#7aa2f7")
    ColorGreen     = lipgloss.Color("#9ece6a")
    ColorRed       = lipgloss.Color("#f7768e")
    ColorYellow    = lipgloss.Color("#e0af68")
    ColorPurple    = lipgloss.Color("#bb9af7")
    ColorCyan      = lipgloss.Color("#7dcfff")
    ColorComment   = lipgloss.Color("#565f89")
    ColorSelection = lipgloss.Color("#33467c")
)

// Estilos base reutilizables
var (
    StyleTitle = lipgloss.NewStyle().
        Bold(true).
        Foreground(ColorBlue)

    StyleSubtitle = lipgloss.NewStyle().
        Foreground(ColorComment)

    StyleHighlight = lipgloss.NewStyle().
        Bold(true).
        Foreground(ColorFg).
        Background(ColorSelection)

    StyleHelp = lipgloss.NewStyle().
        Foreground(ColorComment)

    StyleBorder = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(ColorComment)

    StyleActiveBorder = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(ColorBlue)
)
```

**Notas:**

- Todos los colores y estilos son `var` a nivel de package — accesibles desde cualquier archivo en `package tui` y desde fuera como `tui.ColorBlue`.
- `lipgloss.Color("#hex")` acepta colores hex de 24 bits. La terminal debe soportar true color (la mayoria de las modernas lo hacen).
- Los estilos son inmutables — definirlos como `var` es seguro porque nadie puede modificarlos.
- Centralizamos todo en un archivo. Cuando quieras cambiar el tema, solo tocas este archivo.

**Tradeoff:** definir los colores como variables globales del package es simple pero no permite temas dinamicos (ej: cambiar de Tokyo Night a Catppuccin en runtime). Para lazyharness, un solo tema es suficiente para el MVP. Si algun dia queres temas, lo cambias a un struct `Theme` que se pasa como parametro.

### Paso 3: Crear el modelo raiz

Reescribi `main.go`:

```go
package main

import (
    "fmt"
    "os"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/Gerardo1909/lazyharness/internal/tui"
)

const version = "0.1.0"

// model contiene todo el estado de la aplicacion
type model struct {
    width  int
    height int
    ready  bool
}

// Init se ejecuta una sola vez al arrancar.
// Retorna nil porque no tenemos side effects iniciales.
func (m model) Init() tea.Cmd {
    return nil
}

// Update recibe eventos y retorna el modelo actualizado.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        }
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.ready = true
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

    help := tui.StyleHelp.Render("(presiona q para salir)")

    content := lipgloss.JoinVertical(
        lipgloss.Center,
        "",
        title,
        "",
        subtitle,
        "",
        help,
    )

    // Centrar en la terminal
    return lipgloss.Place(
        m.width, m.height,
        lipgloss.Center, lipgloss.Center,
        content,
    )
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
```

### Paso 4: Ejecutar

```bash
go run .
```

Deberias ver un texto centrado en tu terminal con colores azules. Presiona `q` para salir. Redimensiona la terminal y mira como se recentra.

**Si no ves colores:** tu terminal probablemente no soporta true color. Proba con Windows Terminal, Alacritty o Kitty. La terminal default de Ubuntu en WSL2 suele funcionar bien.

### Paso 5: Entender el flujo

Cuando ejecutas `go run .`, esto es lo que pasa:

1. `main()` crea un `tea.NewProgram` con un model vacio y `tea.WithAltScreen()` (pantalla alternativa, como vim/less).
2. bubbletea llama a `Init()` — retornamos `nil` (sin side effects).
3. La terminal manda un `tea.WindowSizeMsg` — nuestro `Update` guarda el tamaño y pone `ready = true`.
4. bubbletea llama a `View()` — renderizamos el contenido centrado.
5. Cada tecla genera un `tea.KeyMsg` que pasa por `Update`. Solo respondemos a `q` y `ctrl+c`.

**El flujo es siempre:** evento → Update → nuevo model → View → render. Nunca hay mutacion fuera de este ciclo.

### Paso 6: Agregar un test

Crea `main_test.go`:

```go
package main

import (
    "strings"
    "testing"

    tea "github.com/charmbracelet/bubbletea"
)

func TestUpdate_QuitOnQ(t *testing.T) {
    m := model{width: 80, height: 24, ready: true}

    // Simular presionar 'q'
    updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

    // Verificar que el cmd es Quit
    if cmd == nil {
        t.Fatal("esperaba un Cmd de quit, obtuve nil")
    }

    // Verificar que el model no cambio
    um := updated.(model)
    if um.width != 80 {
        t.Errorf("width no deberia cambiar: esperaba 80, obtuve %d", um.width)
    }
}

func TestUpdate_Resize(t *testing.T) {
    m := model{}

    updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
    um := updated.(model)

    if um.width != 120 || um.height != 40 {
        t.Errorf("tamaño esperado 120x40, obtuve %dx%d", um.width, um.height)
    }
    if !um.ready {
        t.Error("despues de resize, ready deberia ser true")
    }
}

func TestView_ShowsVersion(t *testing.T) {
    m := model{width: 80, height: 24, ready: true}
    view := m.View()

    if !strings.Contains(view, version) {
        t.Errorf("la vista deberia contener la version %q", version)
    }
}

func TestView_Loading(t *testing.T) {
    m := model{ready: false}
    view := m.View()

    if !strings.Contains(view, "Cargando") {
        t.Error("antes de recibir WindowSizeMsg, deberia mostrar 'Cargando...'")
    }
}
```

```bash
go test ./... -v
```

**Notar:** podemos testear la logica de la TUI sin lanzar la TUI real. `Update` y `View` son funciones puras — les pasas datos, te retornan datos. Esta es la ventaja del patron Elm.

## Tradeoffs y decisiones de diseno

### Decision 1: `tea.WithAltScreen()` — pantalla alternativa

Usamos `tea.WithAltScreen()` que activa la pantalla alternativa de la terminal (como vim, less, htop). Cuando la app cierra, la terminal vuelve a como estaba antes.

**Alternativa:** sin `WithAltScreen`, la app renderiza inline (como un `ls` con colores). Esto es util para CLIs simples pero no para una TUI de paneles. lazygit, lazydocker y k9s usan alt screen.

### Decision 2: Tema como variables de package

Los colores viven en `internal/tui/theme.go` como variables publicas del package.

**Alternativa 1:** un struct `Theme` que se pasa como parametro a cada componente. Mas flexible (permite multiples temas) pero agrega complejidad que no necesitamos ahora.

**Alternativa 2:** constantes en vez de variables. Las constantes en Go solo pueden ser tipos basicos (string, int, bool), no structs como `lipgloss.Color`. Por eso usamos `var`.

### Decision 3: model en main.go — temporal

El model vive en `main.go` por simplicidad. En la [Clase 02](./02_listas_y_navegacion.md) lo moveremos a `internal/tui/app.go` cuando agreguemos routing entre pantallas.

## Errores comunes y tips

### Error: "cannot use m (variable of type model) as tea.Model"

Esto pasa si tu struct no implementa los 3 metodos de `tea.Model`: `Init()`, `Update()` y `View()`. Revisa que los 3 esten definidos con las firmas exactas.

### Error: la pantalla parpadea o no se actualiza

Si ves parpadeo, probablemente estas creando muchos strings grandes en `View()`. lipgloss optimiza internamente, pero evita concatenar strings en un loop — usa `lipgloss.JoinVertical` o `strings.Builder`.

Si la pantalla no se actualiza despues de un evento, verifica que `Update` esta retornando un model NUEVO (no el viejo). Un error comun:

```go
// MAL — retorna el model viejo, el resize no se refleja
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width // modifica una COPIA porque m es por valor
    }
    return m, nil // retorna la copia modificada — esto SI funciona en Go
    // Pero si m fuera un puntero, tendrias que tener cuidado
}
```

Aca hay una sutileza: como `m` se recibe por valor (no por puntero), Go crea una copia automaticamente. Modificar `m.width` modifica la copia, y al retornar `m` retornas la copia modificada. Esto funciona correctamente. El problema seria si devolvemos un model viejo guardado en alguna variable.

### Error: "alt screen" no se desactiva al crashear

Si tu programa panics sin cerrar bubbletea, la terminal queda en alt screen mode. Solucion rapida: escribe `reset` y presiona Enter (aunque no veas el texto).

**Prevencion:** usa `defer` en main para asegurar cleanup:

```go
p := tea.NewProgram(model{}, tea.WithAltScreen())
// Si algo panic, la terminal se restaura porque bubbletea maneja signals
```

bubbletea ya maneja SIGINT/SIGTERM, pero si tu codigo hace `os.Exit(1)` directo sin pasar por bubbletea, la terminal puede quedar rota.

### Tip: `go run .` vs `go build . && ./lazyharness`

`go run .` compila a un temporal y ejecuta. Es rapido para desarrollo pero no te da un binario.

`go build -o lazyharness .` crea el binario `lazyharness` que podes mover a cualquier maquina Linux y ejecutar sin Go instalado. Probalo: copia el binario a otro directorio y ejecutalo.

## Ejercicios

### 1. Basico: toggle de ayuda

Agrega una funcionalidad: al presionar `?`, se muestra un texto de ayuda adicional. Al presionar `?` de nuevo, se oculta. Necesitas un campo `showHelp bool` en el model.

### 2. Intermedio: animacion con Tick

bubbletea tiene un mecanismo para ejecutar codigo periodicamente:

```go
func tickCmd() tea.Cmd {
    return tea.Tick(time.Second, func(t time.Time) tea.Msg {
        return tickMsg(t)
    })
}
```

Agrega un contador de segundos que se actualiza cada segundo en la esquina inferior derecha. Vas a necesitar un campo `elapsed int` en el model, un `tickMsg` type, y manejar el tick en Init y Update.

### 3. Avanzado: multiples estilos

Crea 3 "temas" en theme.go (Tokyo Night, Catppuccin, Nord) y permite cambiar entre ellos con la tecla `t`. Para esto necesitas un campo `theme int` en el model y funciones que retornen los estilos segun el tema.

Piensa: por que un struct `Theme` seria mejor que variables globales para esto?

## Para profundizar

- [bubbletea README y ejemplos](https://github.com/charmbracelet/bubbletea): el README explica la arquitectura. La carpeta `examples/` tiene desde "hello world" hasta apps complejas.
- [The Elm Architecture](https://guide.elm-lang.org/architecture/): la guia original del patron que bubbletea implementa. Leerla te ayuda a entender POR QUE bubbletea funciona como funciona.
- [lipgloss README](https://github.com/charmbracelet/lipgloss): todos los estilos disponibles (bordes, padding, margin, alineacion).
- [Charm's tutorial: Your first Bubble Tea app](https://charm.sh/blog/bubbletea-tutorial/): tutorial oficial paso a paso.
- [Tokyo Night theme](https://github.com/enkia/tokyo-night-vscode-theme): la paleta de colores que usamos.

## Que sigue

En la [Clase 02](./02_listas_y_navegacion.md) vamos a convertir esta pantalla de bienvenida en la pantalla Home real — con una lista navegable de harnesses (primero con datos fake), el componente `list` de bubbles, layout de dos paneles, y la barra de atajos en la parte inferior. Ahi es donde lazyharness empieza a parecerse al mockup 01.
