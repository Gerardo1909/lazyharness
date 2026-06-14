# Plan de implementación en Go

## ¿Por qué Go?

lazyharness es una TUI rica (paneles, árboles, editor embebido, diffs, chat) que se
distribuye como binario standalone y lanza procesos externos en paralelo. Estas tres
características definen la elección del lenguaje:

| Criterio                    | Python                        | TypeScript                 | **Go**                             |
|-----------------------------|-------------------------------|----------------------------|------------------------------------|
| TUI madura                  | textual (buena)               | ink (limitada)             | **bubbletea (excelente)**          |
| Distribución como binario   | PyInstaller — frágil, ~80MB   | pkg + Node embebido ~80MB | **`go build` — nativo, ~10-15MB** |
| Concurrencia                | asyncio (complejo)            | Promises/Workers           | **goroutines (trivial, ~2KB c/u)** |
| Ecosistema "lazy\*" apps    | No hay referentes             | No hay referentes          | **lazygit, lazydocker, k9s**       |
| Curva de aprendizaje        | Ya conocido                   | Ya conocido                | **~2-3 semanas para ser productivo** |

Las apps TUI modernas de terminal (lazygit, lazydocker, k9s) están escritas en Go usando
el ecosistema de [Charm](https://charm.sh). No es casualidad: Go compila a un binario
sin dependencias, las goroutines manejan UI + procesos en background sin esfuerzo, y
bubbletea provee una arquitectura Elm-like que escala a interfaces complejas.

---

## Requisitos de instalación

### Go en WSL2/Linux

```bash
# Descargar e instalar (versión 1.24.x o superior)
wget https://go.dev/dl/go1.24.4.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.4.linux-amd64.tar.gz

# Agregar al PATH — poner en ~/.bashrc o ~/.zshrc
export PATH=$PATH:/usr/local/go/bin
export PATH=$PATH:$(go env GOPATH)/bin

# Verificar
go version
# go version go1.24.4 linux/amd64
```

No se necesita nada más. Go es autocontenido: compilador + linker + gestor de
dependencias + formateador + linter, todo en un solo binario.

### Inicializar el proyecto

```bash
cd ~/stuff/lazyharness
go mod init github.com/Gerardo1909/lazyharness
```

Esto crea `go.mod` (equivalente a `pyproject.toml` o `package.json`). Las dependencias
se instalan con `go get <paquete>`.

### Herramientas de desarrollo recomendadas

```bash
# Linter (equivalente a ruff/eslint)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Hot reload durante desarrollo (equivalente a nodemon)
go install github.com/air-verse/air@latest

# Para que el editor tenga autocompletado (gopls ya viene con la extensión de Go en VSCode)
```

---

## Fundamentos de Go para un pythonista

### 1. Tipado estático — el cambio más grande

```go
// Python: name = "lazyharness"
// Go:
var name string = "lazyharness"
name := "lazyharness" // forma corta, infiere el tipo

// Python: roles = ["arquitecto", "reviewer"]
// Go:
roles := []string{"arquitecto", "reviewer"}

// Python: config = {"provider": "anthropic", "model": "opus"}
// Go:
config := map[string]string{
    "provider": "anthropic",
    "model":    "opus",
}
```

No hay duck typing. Los tipos se declaran o se infieren en compilación. Esto significa
que muchos bugs que en Python descubrís en runtime, en Go los atrapa el compilador.

### 2. Errores explícitos — no hay try/except

Las funciones que pueden fallar retornan `(resultado, error)`. No existen excepciones.

```go
// Python:
// try:
//     data = open("harness.json").read()
// except FileNotFoundError as e:
//     print(f"no encontré el archivo: {e}")

// Go:
data, err := os.ReadFile("harness.json")
if err != nil {
    return fmt.Errorf("no pude leer harness.json: %w", err)
}
// usar data...
```

Vas a escribir `if err != nil` cientos de veces. Es verboso pero hace que el flujo de
errores siempre sea visible — no hay errores silenciosos ni excepciones que nadie catchea.

### 3. Structs en vez de clases — no hay herencia

```go
// Python:
// class Role:
//     def __init__(self, name: str, color: str, prompt_file: str):
//         self.name = name
//         self.color = color
//         self.prompt_file = prompt_file

// Go:
type Role struct {
    Name       string `json:"name"`
    Color      string `json:"color"`
    PromptFile string `json:"prompt_file"`
}

// Los métodos se definen aparte, asociados al struct
func (r Role) DisplayName() string {
    return fmt.Sprintf("[%s] %s", r.Color, r.Name)
}
```

No hay herencia. En su lugar se usa **composición** (embeber un struct dentro de otro)
e **interfaces implícitas**.

### 4. Interfaces implícitas

```go
// Definís una interface
type Displayable interface {
    DisplayName() string
}

// Role ya la implementa porque tiene DisplayName() string.
// No hace falta escribir "implements" — es automático.
// Esto es como duck typing pero verificado en compilación.
```

### 5. Goroutines — concurrencia liviana

```go
// Python: asyncio.create_task(run_agent(prompt))
// Go:
go func() {
    result := runClaudeCode(prompt)
    resultChan <- result // envía el resultado por un channel
}()

// Esperar el resultado
result := <-resultChan
```

Una goroutine pesa ~2KB (un thread del SO pesa ~1MB). Podés lanzar miles.
Los **channels** son el mecanismo de comunicación entre goroutines — como una queue
thread-safe built-in.

### 6. Packages y visibilidad

```go
// En Go, la primera letra define la visibilidad:
type Role struct { ... }       // Role es pública (mayúscula) — exportada
type roleIndex struct { ... }  // roleIndex es privada (minúscula) — solo visible dentro del package

func LoadHarness() { ... }     // pública
func parseConfig() { ... }     // privada
```

Cada directorio es un package. No hay `__init__.py`.

### 7. Lo que Go NO tiene (y cómo se resuelve)

| Python tiene...          | Go...                                           |
|--------------------------|--------------------------------------------------|
| List comprehensions      | Usa `for` loops explícitos                       |
| Decoradores              | Usa funciones que envuelven funciones (middleware)|
| Valores default en args  | Usa "options structs" o variadic options          |
| Enums                    | Usa `const` + `iota`                             |
| `**kwargs`               | Usa structs de opciones                          |
| Generadores / `yield`    | Usa channels o iteradores                        |
| REPL                     | No hay — se compila y ejecuta (`go run .`)       |

### 8. Ciclo de desarrollo

```bash
go run .           # compilar y ejecutar (como python main.py)
go build .         # compilar a binario
go test ./...      # correr todos los tests
go fmt ./...       # formatear código (no hay discusión de estilo — hay UN formato)
go vet ./...       # análisis estático básico
```

`go fmt` es obligatorio por convención. No hay "tabs vs spaces" ni "single vs double
quotes" — Go tiene un solo estilo y punto.

---

## Dependencias principales

```bash
# TUI framework — el corazón de la app
go get github.com/charmbracelet/bubbletea       # arquitectura Elm para TUI
go get github.com/charmbracelet/lipgloss        # estilos (colores, bordes, padding)
go get github.com/charmbracelet/bubbles         # componentes (list, textinput, viewport, etc.)

# Git — para el versionado del harness
go get github.com/go-git/go-git/v5              # git puro en Go, sin depender del binario git

# JSON — ya incluido en la stdlib (encoding/json)
# Procesos externos — ya incluido en la stdlib (os/exec)
# Filesystem — ya incluido en la stdlib (os, filepath)
```

### Cómo funciona bubbletea (el patrón Elm)

bubbletea usa un loop unidireccional: **Model → Update → View**.

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
└──────────────────────────────────────────┘
```

```go
// Cada "pantalla" implementa esta interface:
type Model interface {
    Init() tea.Cmd                       // setup inicial
    Update(msg tea.Msg) (Model, tea.Cmd) // recibe un evento, retorna modelo nuevo
    View() string                        // retorna lo que se renderiza
}
```

Es predecible: dado un modelo y un evento, siempre obtenés el mismo resultado.
Nada muta en background sin pasar por Update.

---

## Estructura de directorios propuesta

```
lazyharness/
├── main.go                          # punto de entrada: parsea flags, crea el modelo raíz, lanza bubbletea
├── go.mod                           # dependencias
├── go.sum                           # checksums de dependencias
│
├── internal/                        # todo el código de la app (internal = no importable desde afuera)
│   │
│   ├── domain/                      # modelos de dominio puros — sin dependencias externas
│   │   ├── harness.go               # struct Harness, Role, Workflow, Reference
│   │   ├── harness_test.go          # tests de lógica de dominio
│   │   ├── task.go                  # struct Task, estados, operaciones sobre tareas.json
│   │   ├── task_test.go
│   │   ├── session.go               # struct Session (metadata de sesiones delegadas)
│   │   └── session_test.go
│   │
│   ├── storage/                     # lectura/escritura de archivos y git — adapters de persistencia
│   │   ├── filesystem.go            # CRUD de archivos .lazyharness/ (prompts, harness.json, tareas.json)
│   │   ├── filesystem_test.go
│   │   ├── git.go                   # operaciones git: init, commit, log, diff, checkout por archivo
│   │   └── git_test.go
│   │
│   ├── runtime/                     # lanzar CLIs agentic externos
│   │   ├── executor.go              # interface Executor + implementación que usa os/exec
│   │   ├── executor_test.go
│   │   ├── claude.go                # adapter específico para Claude Code CLI
│   │   └── claude_test.go
│   │
│   ├── tui/                         # toda la capa de presentación (bubbletea)
│   │   ├── app.go                   # modelo raíz que orquesta las vistas y el routing entre pantallas
│   │   ├── theme.go                 # colores, estilos lipgloss, constantes visuales (Tokyo Night)
│   │   ├── keys.go                  # keybindings centralizados
│   │   │
│   │   ├── home/                    # pantalla inicial (mockup 01)
│   │   │   ├── model.go             # Model, Update, View de la pantalla home
│   │   │   └── model_test.go
│   │   │
│   │   ├── harness/                 # vista de harness (mockup 02)
│   │   │   ├── model.go             # sidebar de roles + panel de prompt read-only + barra de acciones
│   │   │   └── model_test.go
│   │   │
│   │   ├── editor/                  # editor de prompts (mockup 03)
│   │   │   ├── model.go             # editor tipo nano + chat IA + panel de prompts generándose
│   │   │   └── model_test.go
│   │   │
│   │   ├── workspace/               # workspace de uso/runtime (mockup 04)
│   │   │   ├── model.go             # sesiones + panel delegado + tareas
│   │   │   └── model_test.go
│   │   │
│   │   ├── history/                 # historial y rollback (mockup 05)
│   │   │   ├── model.go             # lista de commits filtrada + diff + diálogo de restauración
│   │   │   └── model_test.go
│   │   │
│   │   └── components/              # componentes reutilizables
│   │       ├── sidebar.go           # sidebar genérica (lista navegable con colores)
│   │       ├── actionbar.go         # barra inferior de acciones con tooltips
│   │       ├── keybar.go            # barra de atajos de teclado (fila inferior)
│   │       ├── dialog.go            # diálogo de confirmación (ej: restaurar, eliminar)
│   │       ├── diffview.go          # visor de diffs con colores rojo/verde
│   │       └── promptview.go        # visor de prompts con @referencias coloreadas
│
├── ideas/                           # documentación de diseño (ya existe)
│   ├── 1_requerimientos.md
│   └── 2_implementacion_go.md       # este archivo
│
└── mockups/                         # mockups SVG (ya existen)
    ├── 01_home_harnesses.svg
    ├── 02_vista_harness.svg
    ├── 03_editor_con_ia.svg
    ├── 04_workspace_uso.svg
    └── 05_historial_rollback.svg
```

### Rol de cada archivo/directorio

| Archivo/Directorio | Responsabilidad |
|---|---|
| `main.go` | Punto de entrada. Parsea flags (`--version`, `--help`, path del proyecto). Crea el modelo raíz y lanza `tea.NewProgram()`. Nada de lógica acá. |
| `internal/domain/` | Los structs y funciones que representan el dominio (Harness, Role, Task, Session). Sin dependencias externas — ni bubbletea, ni go-git, ni os. Solo lógica de negocio pura y testeable. |
| `internal/storage/` | Todo lo que toca disco: leer/escribir archivos del harness, operaciones git. Implementa interfaces definidas en `domain/` si hace falta desacoplar. |
| `internal/runtime/` | Lanzar y gestionar procesos de CLIs agentic. Define una interface `Executor` para que se pueda mockear en tests. |
| `internal/tui/app.go` | El "router" de la app. Mantiene el estado global (qué pantalla estamos viendo, qué harness está seleccionado) y delega a los modelos de cada pantalla. |
| `internal/tui/theme.go` | Todos los estilos visuales centralizados. Colores Tokyo Night, bordes, padding. Cuando un componente necesita un color, lo toma de acá. |
| `internal/tui/keys.go` | Definición de keybindings con `key.NewBinding()` de bubbles. Cada pantalla referencia estas constantes en vez de hardcodear strings. |
| `internal/tui/{pantalla}/` | Cada pantalla tiene su propio package con su Model, Update, View. Esto permite que cada pantalla se desarrolle y testee de forma independiente. |
| `internal/tui/components/` | Componentes visuales reutilizables entre pantallas. No tienen estado propio significativo — reciben datos y renderizan. |

### Principios de la estructura

1. **`internal/`** — Go garantiza que nada externo puede importar estos packages. Todo el
   código de la app vive acá; `main.go` es solo el punto de entrada.

2. **Separación dominio / storage / tui** — El dominio no sabe que existe una TUI. El
   storage no sabe que existe bubbletea. La TUI importa a ambos pero no al revés. Esto
   permite testear la lógica de negocio sin levantar la interfaz.

3. **Un package por pantalla** — Cada mockup tiene su propio directorio con su modelo
   independiente. El `app.go` raíz decide cuál mostrar y les pasa datos.

4. **Componentes reutilizables** — La sidebar, la barra de acciones y la barra de teclas
   aparecen en varias pantallas. Se extraen como componentes que reciben datos y renderizan.

---

## Pruebas unitarias en Go

### Convenciones

- Los tests viven **al lado del código** que testean: `harness.go` → `harness_test.go`.
- El archivo debe tener `package` igual al del código (o `package xxx_test` para tests de caja negra).
- Las funciones de test empiezan con `Test` y reciben `*testing.T`.
- Se ejecutan con `go test ./...` (todos) o `go test ./internal/domain/` (un package).

### Ejemplo: testear el dominio

```go
// internal/domain/harness_test.go
package domain

import "testing"

func TestNewHarness(t *testing.T) {
    h, err := NewHarness("dev-flow", "xml", "/home/user/proyecto")
    if err != nil {
        t.Fatalf("no esperaba error: %v", err)
    }
    if h.Name != "dev-flow" {
        t.Errorf("nombre esperado 'dev-flow', obtuve '%s'", h.Name)
    }
    if h.PromptFormat != "xml" {
        t.Errorf("formato esperado 'xml', obtuve '%s'", h.PromptFormat)
    }
}

// Table-driven tests — el patrón más común en Go
func TestRoleDisplayName(t *testing.T) {
    tests := []struct {
        name     string
        role     Role
        expected string
    }{
        {
            name:     "rol con color",
            role:     Role{Name: "arquitecto", Color: "#f7768e"},
            expected: "[#f7768e] arquitecto",
        },
        {
            name:     "rol sin color usa default",
            role:     Role{Name: "docs", Color: ""},
            expected: "[#c0caf5] docs",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := tt.role.DisplayName()
            if got != tt.expected {
                t.Errorf("esperaba %q, obtuve %q", tt.expected, got)
            }
        })
    }
}
```

### Ejemplo: testear storage con filesystem temporal

```go
// internal/storage/filesystem_test.go
package storage

import (
    "os"
    "path/filepath"
    "testing"
)

func TestSaveAndLoadHarness(t *testing.T) {
    // t.TempDir() crea un directorio temporal que se borra al terminar el test
    dir := t.TempDir()

    store := NewFilesystemStore(dir)

    harness := domain.Harness{
        Name:         "test-harness",
        PromptFormat: "xml",
    }

    // Guardar
    if err := store.Save(harness); err != nil {
        t.Fatalf("error al guardar: %v", err)
    }

    // Verificar que el archivo existe
    path := filepath.Join(dir, ".lazyharness", "harness.json")
    if _, err := os.Stat(path); os.IsNotExist(err) {
        t.Fatal("harness.json no fue creado")
    }

    // Cargar
    loaded, err := store.Load()
    if err != nil {
        t.Fatalf("error al cargar: %v", err)
    }
    if loaded.Name != "test-harness" {
        t.Errorf("nombre esperado 'test-harness', obtuve '%s'", loaded.Name)
    }
}
```

### Ejemplo: testear una vista de bubbletea

```go
// internal/tui/home/model_test.go
package home

import (
    "strings"
    "testing"

    tea "github.com/charmbracelet/bubbletea"
)

func TestHomeView_MuestraHarnesses(t *testing.T) {
    m := NewModel([]domain.HarnessSummary{
        {Name: "dev-flow", ProjectDir: "~/dev/shop-api"},
        {Name: "data-pipeline", ProjectDir: "~/dev/etl"},
    })

    view := m.View()

    if !strings.Contains(view, "dev-flow") {
        t.Error("la vista debería mostrar 'dev-flow'")
    }
    if !strings.Contains(view, "data-pipeline") {
        t.Error("la vista debería mostrar 'data-pipeline'")
    }
}

func TestHomeView_NavegacionConTeclas(t *testing.T) {
    m := NewModel([]domain.HarnessSummary{
        {Name: "primero", ProjectDir: "~/a"},
        {Name: "segundo", ProjectDir: "~/b"},
    })

    // Simular tecla "j" (bajar)
    updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
    model := updated.(Model)

    if model.SelectedIndex != 1 {
        t.Errorf("después de 'j', el índice debería ser 1, es %d", model.SelectedIndex)
    }
}
```

### Comandos útiles de testing

```bash
go test ./...                          # correr todos los tests
go test ./internal/domain/ -v          # tests de un package, verbose
go test ./... -run TestNewHarness      # correr un test específico
go test ./... -count=1                 # sin cache
go test ./... -cover                   # ver % de cobertura
go test ./... -race                    # detectar race conditions (MUY útil)
```

El flag `-race` es oro puro: detecta accesos concurrentes a memoria compartida.
Usarlo siempre durante desarrollo.

---

## Roadmap iterativo

Cada paso produce algo que funciona y se puede probar. Nunca estás más de 1-2 días
sin ver algo en la pantalla.

### Fase 0 — Setup y "hola mundo" en la terminal

**Objetivo:** verificar que Go, bubbletea y el proyecto compilan y corren.

**Pasos:**
1. Instalar Go, inicializar `go mod`, instalar bubbletea + lipgloss.
2. Crear `main.go` con un programa bubbletea mínimo: una ventana que muestra
   "lazyharness v0.1.0" con el estilo Tokyo Night y se cierra con `q`.
3. Crear `internal/tui/theme.go` con los colores base del tema.
4. Verificar: `go run .` muestra texto estilizado, `q` cierra. `go test ./...` pasa.

**Lo que aprendés:** ciclo go run/build, bubbletea Model/Update/View, lipgloss básico.

```
┌────────────────────────────────────────┐
│ lazyharness v0.1.0                     │
│                                        │
│ (presioná q para salir)                │
│                                        │
└────────────────────────────────────────┘
```

---

### Fase 1 — Lista navegable de harnesses (pantalla Home)

**Objetivo:** mostrar una lista hardcodeada de harnesses y navegar con j/k.

**Pasos:**
1. Crear `internal/domain/harness.go` con el struct `Harness` y `HarnessSummary`.
2. Crear `internal/tui/home/model.go` usando el componente `list` de bubbles.
3. Implementar la sidebar izquierda con harnesses hardcodeados.
4. Agregar el panel derecho con detalle del harness seleccionado (nombre, path, roles).
5. Agregar `internal/tui/components/keybar.go` — la barra inferior de atajos.
6. Escribir tests: navegación con j/k, que el View contenga los nombres.

**Lo que aprendés:** bubbles/list, layout con lipgloss (JoinHorizontal), keybindings.

**Resultado:** se parece al mockup 01 pero con datos fake.

---

### Fase 2 — Leer harnesses reales del filesystem

**Objetivo:** reemplazar los datos hardcodeados por lectura de `.lazyharness/` real.

**Pasos:**
1. Crear `internal/storage/filesystem.go` — buscar directorios con `.lazyharness/`,
   leer `harness.json`, listar archivos de prompts.
2. Definir el formato de `harness.json` (nombre, formato de prompts, roles con color
   y jerarquía, provider, workflow).
3. Crear un harness de ejemplo a mano en algún directorio de prueba.
4. Conectar el storage con la pantalla Home: al arrancar, escanear paths conocidos.
5. Definir dónde se guardan los paths conocidos (ej: `~/.config/lazyharness/known.json`).
6. Escribir tests de filesystem con `t.TempDir()`.

**Lo que aprendés:** `encoding/json`, `os`, `filepath`, manejo de errores de I/O.

---

### Fase 3 — Crear un harness nuevo

**Objetivo:** desde la Home, presionar `n` y crear un harness con nombre + formato + path.

**Pasos:**
1. Crear `internal/tui/components/dialog.go` — diálogo modal con inputs de texto.
2. Al confirmar, llamar a `storage.InitHarness()` que crea la carpeta `.lazyharness/`,
   el `harness.json` inicial y el primer archivo de prompt vacío.
3. Refrescar la lista de harnesses después de crear.
4. Tests: crear harness, verificar que los archivos existan, verificar estructura.

**Lo que aprendés:** bubbles/textinput, modales/overlays en bubbletea, crear directorios.

---

### Fase 4 — Vista de harness con roles y prompt read-only

**Objetivo:** al seleccionar un harness, navegar a la vista del mockup 02.

**Pasos:**
1. Crear `internal/tui/harness/model.go` — layout de tres zonas: sidebar de roles
   (árbol), panel principal (prompt read-only), barra de acciones.
2. Crear `internal/tui/components/sidebar.go` — lista con colores e indentación (árbol).
3. Crear `internal/tui/components/promptview.go` — viewport scrolleable que colorea
   las @referencias según el color del rol referenciado.
4. Crear `internal/tui/components/actionbar.go` — barra de acciones con tooltip.
5. Implementar navegación entre paneles con Tab.
6. Implementar el routing en `app.go`: Home → seleccionar → vista Harness; Esc → volver.

**Lo que aprendés:** routing entre vistas, composición de modelos, bubbles/viewport.

---

### Fase 5 — Versionado con git: init, commit al guardar

**Objetivo:** cada harness tiene su repo git; guardar = commit.

**Pasos:**
1. Crear `internal/storage/git.go` usando go-git: `Init()`, `CommitAll(message)`,
   `Log(filename)`, `Diff(commitA, commitB, filename)`.
2. Al crear un harness (Fase 3), inicializar el repo git en `.lazyharness/`.
3. En la vista de harness, la acción `s` (guardar) pide un mensaje y genera un commit.
4. En el header, mostrar el último commit y el estado (limpio / sin guardar).
5. Tests: crear repo, hacer commits, verificar log.

**Lo que aprendés:** go-git, goroutines para operaciones que pueden tardar.

---

### Fase 6 — Editor de prompts embebido

**Objetivo:** presionar `e` en un rol abre un editor tipo nano en el panel principal.

**Pasos:**
1. Crear `internal/tui/editor/model.go` — usar bubbles/textarea para edición multilínea.
2. Implementar ctrl+s para guardar (escribe el archivo + pide mensaje de commit).
3. Implementar Esc para cancelar edición.
4. Colorear @referencias en tiempo real mientras se edita (esto puede venir después).
5. Tests: simular escritura, verificar que el contenido se guarda.

**Lo que aprendés:** bubbles/textarea, escribir archivos, coordinar save + git commit.

---

### Fase 7 — Historial y rollback por rol

**Objetivo:** presionar `h` en un rol muestra su historial de versiones (mockup 05).

**Pasos:**
1. Crear `internal/tui/history/model.go` — lista de commits filtrada por archivo,
   panel de diff, diálogo de confirmación para restaurar.
2. Crear `internal/tui/components/diffview.go` — renderizar diffs con líneas rojas/verdes.
3. Implementar restauración: checkout del archivo a una versión anterior + commit nuevo.
4. Tests: crear varias versiones, restaurar, verificar contenido y que el historial
   tenga el commit de restauración.

**Lo que aprendés:** git diff/log filtrado, renderizado de diffs, diálogos de confirmación.

---

### Fase 8 — Tareas (tareas.json)

**Objetivo:** panel de tareas visible en la vista de workspace (mockup 04).

**Pasos:**
1. Crear `internal/domain/task.go` — struct Task con id, rol, fecha, título, informe, estado.
2. Crear lectura/escritura de `tareas.json` en storage.
3. Crear `internal/tui/workspace/model.go` con el panel de tareas: listar, editar estado,
   ver informe, borrar.
4. Tests: CRUD de tareas, serialización JSON.

**Lo que aprendés:** trabajar con JSON estructurado, time.Time, enums con iota.

---

### Fase 9 — Runtime delegado: invocar un CLI agentic

**Objetivo:** presionar `i` en un rol lanza Claude Code con el prompt inyectado.

**Pasos:**
1. Crear `internal/runtime/executor.go` — interface `Executor` con método `Run(prompt, workDir)`.
2. Crear `internal/runtime/claude.go` — implementación que lanza `claude` con el prompt
   del rol inyectado (vía flag o archivo temporal según el CLI soporte).
3. En la vista workspace (mockup 04), mostrar sesiones activas y permitir retomar.
4. Manejar el proceso en una goroutine para que la TUI no se bloquee.
5. Tests: mockear Executor, verificar que se construye el comando correctamente.

**Lo que aprendés:** os/exec, goroutines para I/O, interface + mock para testing.

---

### Fase 10 — Sesiones del CLI delegado

**Objetivo:** listar, retomar y vincular sesiones a roles.

**Pasos:**
1. Crear `internal/domain/session.go` — struct Session con id, rol, fecha, estado.
2. Investigar cómo Claude Code lista sesiones (probablemente leyendo su directorio
   de datos local).
3. En la vista workspace, sidebar de sesiones (mockup 04, panel izquierdo inferior).
4. Acción `r` para retomar una sesión.
5. Tests: parseo de sesiones, vinculación con roles.

---

### Fase 11 — Edición asistida con IA ("Hacelo con IA")

**Objetivo:** al editar, ctrl+a activa una barra de chat donde una IA ayuda a crear prompts.

**Pasos:**
1. Extender el editor (Fase 6) con un panel de chat inferior.
2. Implementar la comunicación con el provider configurado (API de Anthropic o el que sea).
3. Inyectar la skill de "harness engineering" invisiblemente.
4. Panel derecho que muestra prompts generándose (mockup 03).
5. La IA hace preguntas interactivas que el usuario responde con 1/2/3.
6. Tests: mockear el provider, verificar que las preguntas se renderizan.

**Lo que aprendés:** HTTP clients en Go, streaming de respuestas, UI asincrónica.

---

### Fase 12 — "Mejorar" (agente en background)

**Objetivo:** la acción `m` lanza un agente que revisa y alinea todos los prompts.

**Pasos:**
1. Implementar el agente de mejora como una goroutine que lee todos los prompts,
   los envía al provider con instrucciones de revisión, y genera diffs propuestos.
2. Mostrar progreso en la UI.
3. Al terminar, presentar los diffs propuestos. El usuario acepta o rechaza cada uno.
4. Si acepta, aplicar cambios + commit.
5. Tests: mockear provider, verificar que los diffs se generan correctamente.

---

### Fase 13 — Configuración de provider

**Objetivo:** cada harness configura su provider (API key, modelo, endpoint).

**Pasos:**
1. Agregar campos de provider a `harness.json`: provider, model, api_key_env (nombre
   de la variable de entorno, nunca el valor directo).
2. Pantalla/diálogo de configuración accesible desde la vista de harness.
3. Validar que la API key exista en el entorno al invocar.
4. Tests: validación de config, resolución de variables de entorno.

---

### Fase 14 — Pulido y distribución

**Objetivo:** binario listo para que otros lo usen.

**Pasos:**
1. Agregar flags de CLI: `--version`, `--help`, path opcional del proyecto.
2. Agregar manejo de resize de terminal.
3. Compilar para múltiples plataformas: `GOOS=linux GOARCH=amd64 go build -o lazyharness`.
4. Crear un Makefile o Taskfile con targets: build, test, lint, release.
5. Configurar goreleaser para builds automatizados y releases en GitHub.

---

## Resumen del roadmap

| Fase | Entregable | Dependencia |
|------|-----------|-------------|
| 0 | Terminal vacía con estilo | Ninguna |
| 1 | Lista navegable de harnesses (datos fake) | Fase 0 |
| 2 | Harnesses leídos del filesystem | Fase 1 |
| 3 | Crear harness nuevo | Fase 2 |
| 4 | Vista de harness con roles y prompt read-only | Fase 2 |
| 5 | Versionado git (init + commit) | Fase 3, 4 |
| 6 | Editor de prompts embebido | Fase 4 |
| 7 | Historial y rollback por rol | Fase 5 |
| 8 | Panel de tareas (tareas.json) | Fase 4 |
| 9 | Invocar CLI agentic | Fase 4 |
| 10 | Listar y retomar sesiones | Fase 9 |
| 11 | Edición asistida con IA | Fase 6, 13 |
| 12 | "Mejorar" (agente background) | Fase 5, 13 |
| 13 | Configuración de provider | Fase 4 |
| 14 | Distribución como binario | Todas |

Las fases 0-7 son el core: al terminar la fase 7 tenés una app funcional para construir,
editar y versionar harnesses. Las fases 8-13 agregan las capacidades de runtime y asistencia
con IA. La fase 14 prepara la distribución.

Cada fase es independiente en su alcance y produce un incremento visible. Podés hacer
una fase por sesión de trabajo y siempre tener algo que funciona para mostrar.
