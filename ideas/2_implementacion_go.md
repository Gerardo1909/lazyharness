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

---

## Estructura de directorios

```
lazyharness/
├── main.go                          # punto de entrada
├── go.mod / go.sum                  # dependencias
│
├── internal/                        # todo el código (no importable desde afuera)
│   ├── domain/                      # modelos de dominio puros
│   │   ├── harness.go               # Harness, Role, Workflow
│   │   ├── task.go                  # Task, estados
│   │   └── session.go               # Session
│   │
│   ├── storage/                     # persistencia: archivos + git
│   │   ├── filesystem.go            # CRUD de .lazyharness/
│   │   └── git.go                   # operaciones git con go-git
│   │
│   ├── runtime/                     # lanzar CLIs agentic externos
│   │   ├── executor.go              # interface Executor
│   │   └── claude.go                # adapter para Claude Code
│   │
│   └── tui/                         # capa de presentación (bubbletea)
│       ├── app.go                   # modelo raíz / router
│       ├── theme.go                 # colores Tokyo Night
│       ├── keys.go                  # keybindings
│       ├── home/                    # pantalla Home
│       ├── harness/                 # vista de harness
│       ├── editor/                  # editor de prompts
│       ├── workspace/               # workspace / runtime
│       ├── history/                 # historial y rollback
│       └── components/              # componentes reutilizables
│
├── ideas/                           # documentación de diseño
│   ├── 1_requerimientos.md
│   ├── 2_implementacion_go.md       # este archivo (índice)
│   └── clases/                      # curso paso a paso
│
└── mockups/                         # mockups SVG
```

---

## Curso: De Python a Go construyendo lazyharness

El contenido de implementación está organizado como un curso progresivo donde cada clase enseña conceptos de Go mientras construye un componente real de la app. Cada clase incluye código completo, tradeoffs, comparación con Python, errores comunes y ejercicios.

### Arco 1 — Fundamentos del lenguaje

| Clase | Título | Fases roadmap | Conceptos Go |
|-------|--------|---------------|--------------|
| [00](clases/00_go_para_pythonistas.md) | Go para pythonistas: el modelo mental | — | Tipos, structs, errores, packages, toolchain |
| [01](clases/01_hola_bubbletea.md) | Hola bubbletea: tu primera TUI | 0 | Interfaces, Elm architecture, lipgloss |

### Arco 2 — Core de la app

| Clase | Título | Fases roadmap | Conceptos Go |
|-------|--------|---------------|--------------|
| [02](clases/02_listas_y_navegacion.md) | Listas y navegación: la pantalla Home | 1 | bubbles/list, layout, keybindings |
| [03](clases/03_filesystem_y_json.md) | Filesystem y JSON: datos reales | 2 + 3 | os, filepath, encoding/json, error wrapping |
| [04](clases/04_vista_de_harness.md) | La vista de harness: composición de paneles | 4 | Composición de modelos, routing, viewport, regexp |
| [05](clases/05_git_desde_go.md) | Git desde Go: versionado automático | 5 | go-git, interfaces como boundaries, context |
| [06](clases/06_editor_embebido.md) | El editor embebido: edición y guardado | 6 | bubbles/textarea, tea.Sequence, atomic writes |
| [07](clases/07_historial_y_rollback.md) | Historial y rollback: viajando en el tiempo | 7 | go-git log/diff, modales, rollback por archivo |

### Arco 3 — Capa operativa + IA

| Clase | Título | Fases roadmap | Conceptos Go |
|-------|--------|---------------|--------------|
| [08](clases/08_tareas_y_workspace.md) | Tareas y workspace: el panel operativo | 8 | time.Time, custom JSON marshal, iota, sort |
| [09](clases/09_procesos_externos.md) | Procesos externos: runtime delegado | 9 + 10 | os/exec, goroutines+channels, tea.ExecProcess |
| [10](clases/10_ia_y_streaming.md) | Integración con IA: chat y streaming | 11 + 13 | net/http, SSE streaming, httptest |
| [11](clases/11_mejora_y_providers.md) | El agente "mejorar": background processing | 12 | WaitGroup, tea.Tick, context.WithCancel |

### Arco 4 — Distribución

| Clase | Título | Fases roadmap | Conceptos Go |
|-------|--------|---------------|--------------|
| [12](clases/12_distribucion.md) | Distribución: de código a binario | 14 | Cross-compilation, ldflags, embed, goreleaser |

---

## Resumen del roadmap

| Fase | Entregable | Clase |
|------|-----------|-------|
| 0 | Terminal vacía con estilo | [01](clases/01_hola_bubbletea.md) |
| 1 | Lista navegable de harnesses (datos fake) | [02](clases/02_listas_y_navegacion.md) |
| 2 + 3 | Harnesses reales del filesystem + crear nuevo | [03](clases/03_filesystem_y_json.md) |
| 4 | Vista de harness con roles y prompt read-only | [04](clases/04_vista_de_harness.md) |
| 5 | Versionado git (init + commit) | [05](clases/05_git_desde_go.md) |
| 6 | Editor de prompts embebido | [06](clases/06_editor_embebido.md) |
| 7 | Historial y rollback por rol | [07](clases/07_historial_y_rollback.md) |
| 8 | Panel de tareas (tareas.json) | [08](clases/08_tareas_y_workspace.md) |
| 9 + 10 | Invocar CLI agentic + sesiones | [09](clases/09_procesos_externos.md) |
| 11 + 13 | Edición asistida con IA + config provider | [10](clases/10_ia_y_streaming.md) |
| 12 | "Mejorar" (agente background) | [11](clases/11_mejora_y_providers.md) |
| 14 | Distribución como binario | [12](clases/12_distribucion.md) |

Las clases 00-07 son el core: al terminar la clase 07 tenés una app funcional para construir, editar y versionar harnesses. Las clases 08-11 agregan las capacidades de runtime y asistencia con IA. La clase 12 prepara la distribución.

Cada clase es independiente en su alcance y produce un incremento visible. Podés hacer una clase por sesión de trabajo y siempre tener algo que funciona.
