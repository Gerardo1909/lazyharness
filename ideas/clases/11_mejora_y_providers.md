# Clase 11: El agente "mejorar" -- background processing

> Al terminar esta clase, presionar `m` en la vista de harness lanza un agente en background que revisa todos los prompts, muestra un spinner con progreso, y al terminar presenta los diffs propuestos para que aceptes o rechaces cada uno. Las mejoras aceptadas se guardan con un commit.

## Prerequisitos

- [Clase 05](./05_git_desde_go.md): commit automatico al guardar.
- [Clase 10](./10_ia_y_streaming.md): AIClient, streaming, provider configurado.

## Conceptos de Go que vas a aprender

### 1. Goroutines de larga duracion con reporte de progreso

Hasta ahora usamos goroutines para tareas cortas (un commit, una request HTTP). La feature "mejorar" lanza una goroutine que puede tardar minutos procesando todos los prompts del harness. Necesita reportar progreso a la UI.

El patron: la goroutine manda mensajes por un channel conforme avanza:

```go
// Tipos de progreso
type improvementProgress struct {
    current int    // cual prompt esta procesando
    total   int    // cuantos prompts hay en total
    role    string // nombre del rol actual
    phase   string // "analizando", "generando", "listo"
}

type improvementResult struct {
    diffs []PromptDiff
    err   error
}

func improveAll(ctx context.Context, client *AIClient, roles []Role, prompts map[string]string) (<-chan improvementProgress, <-chan improvementResult) {
    progressCh := make(chan improvementProgress, len(roles))
    resultCh := make(chan improvementResult, 1)

    go func() {
        defer close(progressCh)
        defer close(resultCh)

        var diffs []PromptDiff

        for i, role := range roles {
            // Verificar cancelacion
            select {
            case <-ctx.Done():
                resultCh <- improvementResult{err: ctx.Err()}
                return
            default:
            }

            // Reportar progreso
            progressCh <- improvementProgress{
                current: i + 1,
                total:   len(roles),
                role:    role.Name,
                phase:   "analizando",
            }

            // Llamar a la IA
            original := prompts[role.Name]
            improved, err := client.SendMessage(ctx, systemPromptMejorar, []Message{
                {Role: "user", Content: fmt.Sprintf("Mejora este prompt:\n\n%s", original)},
            })
            if err != nil {
                resultCh <- improvementResult{err: fmt.Errorf("error en rol %s: %w", role.Name, err)}
                return
            }

            if improved != original {
                diffs = append(diffs, PromptDiff{
                    RoleName: role.Name,
                    Original: original,
                    Improved: improved,
                })
            }
        }

        resultCh <- improvementResult{diffs: diffs}
    }()

    return progressCh, resultCh
}
```

**Equivalente Python:**

```python
import asyncio

async def improve_all(client, roles, prompts):
    diffs = []
    for i, role in enumerate(roles):
        yield {"current": i + 1, "total": len(roles), "role": role.name}
        improved = await client.send(f"Mejora este prompt:\n\n{prompts[role.name]}")
        if improved != prompts[role.name]:
            diffs.append(PromptDiff(role.name, prompts[role.name], improved))
    return diffs
```

En Python usarias un generator asincrono. En Go, usas channels. La diferencia es que el channel permite que el productor y el consumidor corran en goroutines separadas — no hace falta `async/await`.

### 2. `sync.WaitGroup` para coordinar multiples goroutines

Si quisieras procesar los prompts en paralelo (varios a la vez), necesitas sincronizar cuando terminan todos:

```go
import "sync"

func improveParallel(ctx context.Context, client *AIClient, roles []Role, prompts map[string]string) []PromptDiff {
    var mu sync.Mutex     // protege el slice de diffs
    var diffs []PromptDiff
    var wg sync.WaitGroup

    for _, role := range roles {
        wg.Add(1)
        go func(r Role) {
            defer wg.Done()

            original := prompts[r.Name]
            improved, err := client.SendMessage(ctx, systemPromptMejorar, []Message{
                {Role: "user", Content: original},
            })
            if err != nil {
                return // ignorar errores individuales
            }

            if improved != original {
                mu.Lock()
                diffs = append(diffs, PromptDiff{
                    RoleName: r.Name,
                    Original: original,
                    Improved: improved,
                })
                mu.Unlock()
            }
        }(role)
    }

    wg.Wait() // esperar a que terminen todos
    return diffs
}
```

**`sync.WaitGroup` es como un contador:**
- `wg.Add(1)` — "va a haber una goroutine mas"
- `wg.Done()` — "esta goroutine termino" (generalmente en un `defer`)
- `wg.Wait()` — "esperar a que el contador llegue a 0"

**Equivalente Python:**

```python
import asyncio

async def improve_parallel(client, roles, prompts):
    tasks = [improve_one(client, role, prompts) for role in roles]
    results = await asyncio.gather(*tasks)
    return [d for d in results if d is not None]
```

**Tradeoff serial vs paralelo:**

| Aspecto | Serial | Paralelo |
|---------|--------|----------|
| Implementacion | Mas simple | Necesita sync.Mutex o channels |
| Progreso | Claro (1 de N) | Confuso (todos avanzan a la vez) |
| Velocidad | Lenta (N * latencia) | Rapida (max(latencias)) |
| Tokens/costo | Igual | Igual |
| Rate limiting | No necesita | Puede pegar en limites de la API |

Para el MVP elegimos **serial**. Es mas simple, el progreso es mas claro para el usuario ("procesando rol 3 de 5"), y no tenemos que preocuparnos por rate limits. Con 5-10 roles, la diferencia de tiempo es tolerable.

### 3. `tea.Tick` para actualizaciones periodicas de la UI

La goroutine de mejora manda progreso por channels, pero bubbletea solo actualiza la UI cuando recibe un `tea.Msg`. Necesitamos un tick periodico que chequee el channel:

```go
import "time"

// Mensaje de tick
type tickMsg time.Time

// Cmd que genera ticks cada 100ms
func tickCmd() tea.Cmd {
    return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
        return tickMsg(t)
    })
}

// En Update:
case tickMsg:
    if !m.improving {
        return m, nil // no hacer nada si no estamos mejorando
    }

    // Chequear si hay progreso
    select {
    case progress, ok := <-m.progressCh:
        if !ok {
            break // channel cerrado
        }
        m.improvementProgress = progress
        return m, tickCmd() // seguir tickeando
    default:
        // No hay progreso nuevo, seguir tickeando
    }

    // Chequear si termino
    select {
    case result, ok := <-m.resultCh:
        if !ok {
            break
        }
        m.improving = false
        if result.err != nil {
            m.statusMessage = "Error: " + result.err.Error()
        } else {
            m.pendingDiffs = result.diffs
            m.reviewIndex = 0
            m.mode = modeReview
        }
        return m, nil
    default:
    }

    return m, tickCmd()
```

**El patron es:** lanzar un tick, en cada tick chequear channels con `select` + `default` (no bloqueante), y si todavia hay trabajo, lanzar otro tick.

**Equivalente Python (textual):**

```python
class ImprovementWidget(Widget):
    def on_mount(self):
        self.set_interval(0.1, self.check_progress)

    def check_progress(self):
        if not self.improving:
            return
        # chequear queue...
```

**Tradeoff tea.Tick vs p.Send:**

| Metodo | Como funciona | Cuando usar |
|--------|---------------|-------------|
| `tea.Tick` | Poll periodico | Cuando tenes un channel y queres chequear |
| `p.Send()` | Push desde la goroutine | Cuando podes pasar el `*tea.Program` |

`p.Send()` es mas eficiente (solo manda cuando hay algo), pero requiere acceso al `*tea.Program` desde la goroutine, lo cual puede complicar la arquitectura. `tea.Tick` es mas simple y encapsula mejor.

### 4. `context.WithCancel` para cancelacion por el usuario

El usuario tiene que poder cancelar la mejora si tarda demasiado o se arrepiente:

```go
// Al iniciar la mejora:
case key.Matches(msg, keys.Improve):
    if m.improving {
        // Ya esta mejorando — cancelar
        m.cancelImprovement()
        m.improving = false
        m.statusMessage = "Mejora cancelada"
        return m, nil
    }

    // Iniciar mejora
    ctx, cancel := context.WithCancel(context.Background())
    m.cancelImprovement = cancel
    m.improving = true

    progressCh, resultCh := improveAll(ctx, m.aiClient, m.harness.Roles, m.prompts)
    m.progressCh = progressCh
    m.resultCh = resultCh

    return m, tickCmd()
```

**`context.WithCancel` retorna un contexto y una funcion `cancel`:**
- Guardas `cancel` en el model.
- Cuando el usuario presiona `m` de nuevo (o Esc), llamas `cancel()`.
- La goroutine que tiene el `ctx` detecta la cancelacion via `ctx.Done()`.
- El `select` con `<-ctx.Done()` rompe el loop y retorna.

**Equivalente Python:**

```python
import asyncio

# Crear
task = asyncio.create_task(improve_all(...))

# Cancelar
task.cancel()
try:
    await task
except asyncio.CancelledError:
    print("cancelado")
```

**Tip:** siempre verifica `ctx.Done()` ANTES de cada operacion costosa (la llamada a la IA). Si verificas despues, puede que hagas una llamada de mas antes de enterarte de la cancelacion.

### 5. Diff como maquina de estados

El flujo de revision de mejoras es una maquina de estados:

```
                    ┌──────────┐
                    │ IDLE     │
                    └────┬─────┘
                         │ presiona 'm'
                         v
                    ┌──────────┐
            ┌───── │IMPROVING │ <── tick (chequea progreso)
            │      └────┬─────┘
            │           │ termina
     presiona 'm'      v
     (cancelar)    ┌──────────┐
            │      │ REVIEW   │ <── navega entre diffs
            │      └──┬───┬───┘
            │         │   │
            │    acepta   rechaza
            │         │   │
            │         v   v
            │      ┌──────────┐
            └─────>│ IDLE     │ (commit si acepto algo)
                   └──────────┘
```

```go
type improveMode int

const (
    improveModeIdle      improveMode = iota
    improveModeWorking   // goroutine corriendo, spinner visible
    improveModeReview    // mostrando diffs, esperando accept/reject
)
```

La maquina de estados se maneja en el Update con un switch sobre el mode.

### 6. Generar diffs legibles

Para mostrar que cambio, generamos un diff sencillo linea por linea:

```go
// PromptDiff representa una mejora propuesta
type PromptDiff struct {
    RoleName string
    Original string
    Improved string
    Accepted bool
}

// SimpleDiff genera un diff legible entre dos textos
func SimpleDiff(original, improved string) string {
    origLines := strings.Split(original, "\n")
    imprLines := strings.Split(improved, "\n")

    var diff strings.Builder

    // Algoritmo simple: comparar linea por linea
    maxLen := len(origLines)
    if len(imprLines) > maxLen {
        maxLen = len(imprLines)
    }

    for i := 0; i < maxLen; i++ {
        origLine := ""
        imprLine := ""
        if i < len(origLines) {
            origLine = origLines[i]
        }
        if i < len(imprLines) {
            imprLine = imprLines[i]
        }

        if origLine == imprLine {
            diff.WriteString("  " + origLine + "\n")
        } else {
            if origLine != "" {
                diff.WriteString("- " + origLine + "\n")
            }
            if imprLine != "" {
                diff.WriteString("+ " + imprLine + "\n")
            }
        }
    }

    return diff.String()
}
```

Para colorear en la TUI:

```go
func renderDiff(diff string) string {
    var colored strings.Builder
    for _, line := range strings.Split(diff, "\n") {
        switch {
        case strings.HasPrefix(line, "+ "):
            colored.WriteString(
                lipgloss.NewStyle().Foreground(tui.ColorGreen).Render(line) + "\n",
            )
        case strings.HasPrefix(line, "- "):
            colored.WriteString(
                lipgloss.NewStyle().Foreground(tui.ColorRed).Render(line) + "\n",
            )
        default:
            colored.WriteString(line + "\n")
        }
    }
    return colored.String()
}
```

## Lo que vamos a construir

La feature "mejorar" (requerimiento C4) completa:

```
  Estado 1: Procesando
  ┌─ dev-flow ── mejorando prompts... ────────────────────────────┐
  │                                                               │
  │  ┌─ Roles ──────────┐ ┌─ Progreso ────────────────────────┐ │
  │  │ ✓ arquitecto     │ │                                    │ │
  │  │ ● code-reviewer  │ │  Analizando: code-reviewer         │ │
  │  │   dev-backend    │ │                                    │ │
  │  │   dev-frontend   │ │  ████████░░░░░░░░░  2 / 4 roles   │ │
  │  │   docs           │ │                                    │ │
  │  └──────────────────┘ │  Tiempo: 45s                       │ │
  │                       │                                    │ │
  │                       └────────────────────────────────────┘ │
  │  m cancelar                                                  │
  └───────────────────────────────────────────────────────────────┘

  Estado 2: Revision de diffs
  ┌─ dev-flow ── revision de mejoras ─────────────────────────────┐
  │                                                               │
  │  Rol: arquitecto (1 / 3 cambios)                             │
  │                                                               │
  │  ┌─ Diff ──────────────────────────────────────────────────┐ │
  │  │   <role>                                                │ │
  │  │ -   Sos el arquitecto del proyecto.                     │ │
  │  │ +   Sos el arquitecto principal del proyecto shop-api.  │ │
  │  │ +   Tu ambito abarca decisiones de estructura,          │ │
  │  │ +   tecnologia y patrones de diseno.                    │ │
  │  │     Tu responsabilidad es disenar...                    │ │
  │  │   </role>                                               │ │
  │  └─────────────────────────────────────────────────────────┘ │
  │                                                               │
  │  a aceptar  r rechazar  n/p siguiente/anterior  q terminar   │
  └───────────────────────────────────────────────────────────────┘
```

**Archivos a crear:**
- `internal/runtime/improver.go`
- `internal/runtime/improver_test.go`
- `internal/tui/components/diffview.go` (si no existe de la clase 07)

**Archivos a modificar:**
- `internal/tui/harness/model.go` -- agregar el flujo de mejora
- `internal/tui/harness/improve.go` -- extraer la logica de mejora a un archivo separado
- `internal/storage/filesystem.go` -- funcion para leer todos los prompts del harness

## Implementacion paso a paso

### Paso 1: Implementar el improver

Crea `internal/runtime/improver.go`:

```go
package runtime

import (
    "context"
    "fmt"
    "strings"

    "github.com/Gerardo1909/lazyharness/internal/domain"
)

// PromptDiff representa una mejora propuesta para un prompt
type PromptDiff struct {
    RoleName string
    Original string
    Improved string
    Accepted bool
}

// ImprovementProgress reporta el avance de la mejora
type ImprovementProgress struct {
    Current int
    Total   int
    Role    string
    Phase   string // "analizando", "generando", "listo"
}

// ImprovementResult es el resultado final de la mejora
type ImprovementResult struct {
    Diffs []PromptDiff
    Err   error
}

// El system prompt para el agente de mejora
const systemPromptMejorar = `Sos un experto en prompt engineering y harness design.
Tu tarea es mejorar un system prompt para un agente de IA.

Reglas:
1. Mantene el idioma original del prompt.
2. Mantene la estructura (XML tags, secciones, formato).
3. Mejora la claridad, especificidad y efectividad.
4. Agrega constraints que falten si son obvios.
5. Corrige ambiguedades.
6. NO cambies el proposito fundamental del rol.
7. Retorna SOLO el prompt mejorado, sin explicaciones ni comentarios.`

// ImproveAll analiza y mejora todos los prompts de un harness
func ImproveAll(
    ctx context.Context,
    client *AIClient,
    roles []domain.Role,
    prompts map[string]string,
) (<-chan ImprovementProgress, <-chan ImprovementResult) {
    progressCh := make(chan ImprovementProgress, len(roles))
    resultCh := make(chan ImprovementResult, 1)

    go func() {
        defer close(progressCh)
        defer close(resultCh)

        var diffs []PromptDiff

        for i, role := range roles {
            // Verificar cancelacion ANTES de la llamada costosa
            select {
            case <-ctx.Done():
                resultCh <- ImprovementResult{
                    Diffs: diffs, // retornar lo que ya tenemos
                    Err:   ctx.Err(),
                }
                return
            default:
            }

            original, ok := prompts[role.Name]
            if !ok || strings.TrimSpace(original) == "" {
                continue // saltar prompts vacios
            }

            // Reportar progreso
            progressCh <- ImprovementProgress{
                Current: i + 1,
                Total:   len(roles),
                Role:    role.Name,
                Phase:   "analizando",
            }

            // Llamar a la IA
            improved, err := client.SendMessage(ctx, systemPromptMejorar, []Message{
                {Role: "user", Content: fmt.Sprintf(
                    "Prompt actual del rol '%s':\n\n%s", role.Name, original,
                )},
            })
            if err != nil {
                resultCh <- ImprovementResult{
                    Err: fmt.Errorf("error mejorando rol '%s': %w", role.Name, err),
                }
                return
            }

            // Solo agregar diff si realmente cambio
            improved = strings.TrimSpace(improved)
            if improved != strings.TrimSpace(original) {
                diffs = append(diffs, PromptDiff{
                    RoleName: role.Name,
                    Original: original,
                    Improved: improved,
                })
            }

            progressCh <- ImprovementProgress{
                Current: i + 1,
                Total:   len(roles),
                Role:    role.Name,
                Phase:   "listo",
            }
        }

        resultCh <- ImprovementResult{Diffs: diffs}
    }()

    return progressCh, resultCh
}

// SimpleDiff genera un diff legible entre dos textos
func SimpleDiff(original, improved string) string {
    origLines := strings.Split(original, "\n")
    imprLines := strings.Split(improved, "\n")

    var diff strings.Builder
    maxLen := len(origLines)
    if len(imprLines) > maxLen {
        maxLen = len(imprLines)
    }

    for i := 0; i < maxLen; i++ {
        var origLine, imprLine string
        if i < len(origLines) {
            origLine = origLines[i]
        }
        if i < len(imprLines) {
            imprLine = imprLines[i]
        }

        if origLine == imprLine {
            diff.WriteString("  " + origLine + "\n")
        } else {
            if origLine != "" {
                diff.WriteString("- " + origLine + "\n")
            }
            if imprLine != "" {
                diff.WriteString("+ " + imprLine + "\n")
            }
        }
    }

    return diff.String()
}
```

### Paso 2: Tests del improver

Crea `internal/runtime/improver_test.go`:

```go
package runtime

import (
    "context"
    "fmt"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/Gerardo1909/lazyharness/internal/domain"
)

func TestImproveAll_BasicFlow(t *testing.T) {
    // Servidor mock que "mejora" el prompt agregandole una linea
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprint(w, `{
            "content": [{"type": "text", "text": "<role>\nSos el arquitecto principal.\nMejora aplicada.\n</role>"}]
        }`)
    }))
    defer server.Close()

    client := NewAIClient(server.URL, "test-key", "test-model")
    roles := []domain.Role{
        {Name: "arquitecto"},
        {Name: "reviewer"},
    }
    prompts := map[string]string{
        "arquitecto": "<role>\nSos el arquitecto.\n</role>",
        "reviewer":   "<role>\nSos el reviewer.\n</role>",
    }

    ctx := context.Background()
    progressCh, resultCh := ImproveAll(ctx, client, roles, prompts)

    // Consumir progreso
    var progressUpdates []ImprovementProgress
    for p := range progressCh {
        progressUpdates = append(progressUpdates, p)
    }

    // Verificar resultado
    result := <-resultCh
    if result.Err != nil {
        t.Fatalf("error inesperado: %v", result.Err)
    }

    if len(result.Diffs) != 2 {
        t.Errorf("esperaba 2 diffs, obtuve %d", len(result.Diffs))
    }

    // Verificar que hubo actualizaciones de progreso
    if len(progressUpdates) < 2 {
        t.Errorf("esperaba al menos 2 updates de progreso, obtuve %d", len(progressUpdates))
    }
}

func TestImproveAll_NoDiffWhenUnchanged(t *testing.T) {
    originalPrompt := "<role>\nSos el arquitecto.\n</role>"

    // Servidor que retorna el prompt sin cambios
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprintf(w, `{"content": [{"type": "text", "text": %q}]}`, originalPrompt)
    }))
    defer server.Close()

    client := NewAIClient(server.URL, "test-key", "test-model")
    roles := []domain.Role{{Name: "arquitecto"}}
    prompts := map[string]string{"arquitecto": originalPrompt}

    ctx := context.Background()
    progressCh, resultCh := ImproveAll(ctx, client, roles, prompts)

    // Consumir progreso
    for range progressCh {
    }

    result := <-resultCh
    if result.Err != nil {
        t.Fatal(result.Err)
    }
    if len(result.Diffs) != 0 {
        t.Errorf("no deberia haber diffs cuando el prompt no cambio, obtuve %d", len(result.Diffs))
    }
}

func TestImproveAll_Cancellation(t *testing.T) {
    // Servidor que tarda mucho en responder
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        time.Sleep(5 * time.Second) // simular latencia
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprint(w, `{"content": [{"type": "text", "text": "mejorado"}]}`)
    }))
    defer server.Close()

    client := NewAIClient(server.URL, "test-key", "test-model")
    roles := []domain.Role{
        {Name: "rol1"},
        {Name: "rol2"},
        {Name: "rol3"},
    }
    prompts := map[string]string{
        "rol1": "prompt 1",
        "rol2": "prompt 2",
        "rol3": "prompt 3",
    }

    // Cancelar despues de 200ms
    ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
    defer cancel()

    progressCh, resultCh := ImproveAll(ctx, client, roles, prompts)

    // Consumir progreso
    for range progressCh {
    }

    result := <-resultCh
    if result.Err == nil {
        t.Error("esperaba error por cancelacion")
    }
}

func TestImproveAll_SkipsEmptyPrompts(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprint(w, `{"content": [{"type": "text", "text": "mejorado"}]}`)
    }))
    defer server.Close()

    client := NewAIClient(server.URL, "test-key", "test-model")
    roles := []domain.Role{
        {Name: "con-prompt"},
        {Name: "sin-prompt"},
    }
    prompts := map[string]string{
        "con-prompt": "un prompt real",
        "sin-prompt": "",
    }

    ctx := context.Background()
    _, resultCh := ImproveAll(ctx, client, roles, prompts)

    result := <-resultCh
    if result.Err != nil {
        t.Fatal(result.Err)
    }
    // Solo deberia haber un diff (el que tenia prompt)
    if len(result.Diffs) != 1 {
        t.Errorf("esperaba 1 diff, obtuve %d", len(result.Diffs))
    }
}

func TestSimpleDiff(t *testing.T) {
    original := "linea 1\nlinea 2\nlinea 3"
    improved := "linea 1\nlinea 2 mejorada\nlinea 3\nlinea 4 nueva"

    diff := SimpleDiff(original, improved)

    if !strings.Contains(diff, "- linea 2") {
        t.Error("diff deberia contener la linea eliminada")
    }
    if !strings.Contains(diff, "+ linea 2 mejorada") {
        t.Error("diff deberia contener la linea mejorada")
    }
    if !strings.Contains(diff, "+ linea 4 nueva") {
        t.Error("diff deberia contener la linea nueva")
    }
    if !strings.Contains(diff, "  linea 1") {
        t.Error("diff deberia contener las lineas sin cambio")
    }
}
```

### Paso 3: Integrar la maquina de estados en la vista de harness

Agrega a `internal/tui/harness/model.go` (o extraelo a `improve.go`):

```go
// Modos de la vista de harness
type harnessMode int

const (
    modeNormal   harnessMode = iota
    modeImprove  // goroutine corriendo
    modeReview   // mostrando diffs
)

// Agregar campos al Model:
type Model struct {
    // ... campos existentes ...

    // Mejora
    mode               harnessMode
    improving          bool
    cancelImprovement  context.CancelFunc
    progressCh         <-chan runtime.ImprovementProgress
    resultCh           <-chan runtime.ImprovementResult
    currentProgress    runtime.ImprovementProgress
    pendingDiffs       []runtime.PromptDiff
    reviewIndex        int
    startTime          time.Time
}

// Mensajes internos
type improvementTickMsg time.Time

func improvementTickCmd() tea.Cmd {
    return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
        return improvementTickMsg(t)
    })
}

// En Update:
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    switch msg := msg.(type) {

    case tea.KeyMsg:
        switch m.mode {
        case modeNormal:
            switch {
            case key.Matches(msg, maintui.HarnessKeyMap.Improve):
                // Iniciar mejora
                if m.aiClient == nil {
                    m.statusMessage = "Provider no configurado"
                    return m, nil
                }

                ctx, cancel := context.WithCancel(context.Background())
                m.cancelImprovement = cancel
                m.mode = modeImprove
                m.improving = true
                m.startTime = time.Now()

                // Cargar todos los prompts
                prompts := m.loadAllPrompts()

                m.progressCh, m.resultCh = runtime.ImproveAll(
                    ctx, m.aiClient, m.harness.Roles, prompts,
                )

                return m, improvementTickCmd()
            }

        case modeImprove:
            switch {
            case key.Matches(msg, maintui.HarnessKeyMap.Improve):
                // Cancelar
                if m.cancelImprovement != nil {
                    m.cancelImprovement()
                }
                m.mode = modeNormal
                m.improving = false
                m.statusMessage = "Mejora cancelada"
                return m, nil
            }

        case modeReview:
            switch msg.String() {
            case "a": // aceptar
                if m.reviewIndex < len(m.pendingDiffs) {
                    m.pendingDiffs[m.reviewIndex].Accepted = true
                    m.reviewIndex++
                    if m.reviewIndex >= len(m.pendingDiffs) {
                        return m, m.applyAcceptedDiffs()
                    }
                }

            case "r": // rechazar
                if m.reviewIndex < len(m.pendingDiffs) {
                    m.reviewIndex++
                    if m.reviewIndex >= len(m.pendingDiffs) {
                        return m, m.applyAcceptedDiffs()
                    }
                }

            case "n": // siguiente
                if m.reviewIndex < len(m.pendingDiffs)-1 {
                    m.reviewIndex++
                }

            case "p": // anterior
                if m.reviewIndex > 0 {
                    m.reviewIndex--
                }

            case "q": // terminar revision (aplicar lo aceptado)
                return m, m.applyAcceptedDiffs()
            }
        }

    case improvementTickMsg:
        if !m.improving {
            return m, nil
        }

        // Chequear progreso (no bloqueante)
        select {
        case progress, ok := <-m.progressCh:
            if ok {
                m.currentProgress = progress
            }
        default:
        }

        // Chequear resultado (no bloqueante)
        select {
        case result, ok := <-m.resultCh:
            if ok {
                m.improving = false
                if result.Err != nil {
                    m.mode = modeNormal
                    m.statusMessage = "Error: " + result.Err.Error()
                    return m, nil
                }
                if len(result.Diffs) == 0 {
                    m.mode = modeNormal
                    m.statusMessage = "No se encontraron mejoras"
                    return m, nil
                }
                m.pendingDiffs = result.Diffs
                m.reviewIndex = 0
                m.mode = modeReview
                return m, nil
            }
        default:
        }

        return m, improvementTickCmd() // seguir tickeando
    }

    return m, nil
}

// Aplicar los diffs aceptados
func (m *Model) applyAcceptedDiffs() tea.Cmd {
    return func() tea.Msg {
        accepted := 0
        for _, diff := range m.pendingDiffs {
            if diff.Accepted {
                // Escribir el prompt mejorado
                err := m.store.WritePrompt(
                    m.harness.ProjectDir,
                    m.harness.RoleByName(diff.RoleName).PromptFile,
                    diff.Improved,
                )
                if err != nil {
                    return improvementErrorMsg{err: err}
                }
                accepted++
            }
        }

        if accepted > 0 {
            // Commit con los cambios
            msg := fmt.Sprintf("mejorar: %d prompt(s) actualizados", accepted)
            _, err := m.git.CommitAll(m.harnessPath, msg)
            if err != nil {
                return improvementErrorMsg{err: err}
            }
        }

        return improvementAppliedMsg{accepted: accepted}
    }
}
```

### Paso 4: Renderizar el progreso y los diffs

```go
func (m Model) View() string {
    switch m.mode {
    case modeImprove:
        return m.renderImproving()
    case modeReview:
        return m.renderReview()
    default:
        return m.renderNormal() // la vista normal de harness
    }
}

func (m Model) renderImproving() string {
    elapsed := time.Since(m.startTime).Truncate(time.Second)

    // Barra de progreso
    progress := m.currentProgress
    barWidth := 30
    filled := 0
    if progress.Total > 0 {
        filled = barWidth * progress.Current / progress.Total
    }
    bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

    content := lipgloss.JoinVertical(lipgloss.Left,
        "",
        fmt.Sprintf("  Analizando: %s", progress.Role),
        "",
        fmt.Sprintf("  %s  %d / %d roles", bar, progress.Current, progress.Total),
        "",
        fmt.Sprintf("  Tiempo: %s", elapsed),
        "",
    )

    header := maintui.StyleTitle.Render(m.harness.Name) + "  " +
        lipgloss.NewStyle().Foreground(tui.ColorYellow).Render("mejorando prompts...")

    keybar := "  m cancelar"

    return lipgloss.JoinVertical(lipgloss.Left, header, content, keybar)
}

func (m Model) renderReview() string {
    if m.reviewIndex >= len(m.pendingDiffs) {
        return "No hay mas diffs"
    }

    diff := m.pendingDiffs[m.reviewIndex]
    diffText := runtime.SimpleDiff(diff.Original, diff.Improved)
    coloredDiff := renderDiff(diffText)

    header := fmt.Sprintf("  Rol: %s (%d / %d cambios)",
        diff.RoleName, m.reviewIndex+1, len(m.pendingDiffs))

    statusLine := ""
    if diff.Accepted {
        statusLine = lipgloss.NewStyle().
            Foreground(tui.ColorGreen).
            Render("  ✓ aceptado")
    }

    diffPanel := maintui.StyleBorder.
        Width(m.width - 4).
        Height(m.height - 10).
        Render(coloredDiff)

    keybar := "  a aceptar  r rechazar  n/p siguiente/anterior  q terminar"

    return lipgloss.JoinVertical(lipgloss.Left,
        maintui.StyleTitle.Render(m.harness.Name) + "  revision de mejoras",
        header,
        statusLine,
        diffPanel,
        keybar,
    )
}

func renderDiff(diff string) string {
    var colored strings.Builder
    for _, line := range strings.Split(diff, "\n") {
        switch {
        case strings.HasPrefix(line, "+ "):
            colored.WriteString(
                lipgloss.NewStyle().Foreground(tui.ColorGreen).Render(line) + "\n")
        case strings.HasPrefix(line, "- "):
            colored.WriteString(
                lipgloss.NewStyle().Foreground(tui.ColorRed).Render(line) + "\n")
        default:
            colored.WriteString(
                lipgloss.NewStyle().Foreground(tui.ColorComment).Render(line) + "\n")
        }
    }
    return colored.String()
}
```

## Tradeoffs y decisiones de diseno

### Decision 1: Serial vs paralelo para procesar prompts

Elegimos **serial**. Las razones:

1. **UX de progreso:** "procesando rol 3 de 5" es claro. "procesando 3 roles a la vez, 2 terminaron, 1 fallo" es confuso.
2. **Rate limiting:** Anthropic tiene limites de requests por minuto. En paralelo podemos pegarle.
3. **Costo predecible:** serial procesa uno a la vez; si el usuario cancela despues de 2 roles, gasto tokens de 2 roles. En paralelo, cancelo pero las 5 requests ya salieron.
4. **Complejidad:** serial no necesita `sync.Mutex` ni `sync.WaitGroup`.

**Cuando pasar a paralelo:** si el harness tiene 20+ roles y la latencia individual es baja. En ese caso, usar un semaforo (channel con buffer) para limitar la concurrencia:

```go
sem := make(chan struct{}, 3) // maximo 3 en paralelo
for _, role := range roles {
    sem <- struct{}{} // adquirir slot
    go func(r Role) {
        defer func() { <-sem }() // liberar slot
        // ... procesar ...
    }(role)
}
```

### Decision 2: Diff linea a linea vs algoritmo LCS

Nuestro `SimpleDiff` compara posiciones. El algoritmo real de diff (usado por git) es LCS (Longest Common Subsequence) que detecta inserciones y eliminaciones de forma mas inteligente.

**Para el MVP, linea a linea es suficiente.** Los prompts son textos cortos y las diferencias suelen ser obvias. Si necesitamos algo mejor, podemos importar un paquete como `github.com/sergi/go-diff`.

### Decision 3: Accept/reject individual vs todo o nada

El usuario revisa cada diff individualmente. Podria aceptar la mejora del arquitecto pero rechazar la del reviewer. Esto da control granular.

**Alternativa todo-o-nada:** mas simple pero menos util. Si una mejora es mala, el usuario rechaza todo y pierde las buenas. La revision individual vale la complejidad extra.

### Decision 4: Commit unico despues de todas las mejoras

Todas las mejoras aceptadas van en un solo commit: `"mejorar: 3 prompt(s) actualizados"`. No un commit por prompt.

**Razon:** si despues queres revertir, un solo commit es mas simple. Y semanticamente, "mejorar" es una sola operacion que toca varios archivos.

## Errores comunes y tips

### Error: race condition entre goroutine y UI

```go
// MAL: la goroutine modifica el model directamente
go func() {
    result := doWork()
    m.result = result  // RACE CONDITION! m se usa en el render loop
}()

// BIEN: la goroutine manda el resultado por un channel
go func() {
    result := doWork()
    resultCh <- result  // seguro, el channel es thread-safe
}()
// En Update, leer del channel y actualizar m
```

En bubbletea, NUNCA modifiques el Model desde una goroutine. Siempre usa channels o `p.Send()`. El Model solo se modifica en `Update()`, que corre en el goroutine principal.

### Error: progress bar que no se actualiza

Si la barra de progreso no se mueve, probablemente el `tea.Tick` no se esta encadenando. Verifica que cada handler de `improvementTickMsg` retorne otro `tickCmd()`:

```go
case improvementTickMsg:
    // ... chequear channels ...
    return m, improvementTickCmd() // IMPORTANTE: no olvidar esto
```

### Error: olvidar commitear despues de aceptar mejoras

Si aplicas los cambios al filesystem pero no haces commit, el usuario pierde la capacidad de hacer rollback. Siempre guarda + commit en una sola operacion.

### Error: channels que nunca se cierran

```go
// MAL: si la funcion retorna antes de cerrar el channel, el lector queda bloqueado
func process() <-chan int {
    ch := make(chan int)
    go func() {
        // si hay un error aca, el channel nunca se cierra
        if err != nil {
            return // LEAK! ch nunca se cierra
        }
        ch <- result
    }()
    return ch
}

// BIEN: defer close al inicio de la goroutine
func process() <-chan int {
    ch := make(chan int)
    go func() {
        defer close(ch) // siempre se cierra
        if err != nil {
            return
        }
        ch <- result
    }()
    return ch
}
```

### Tip: `select` con `default` para lectura no bloqueante

```go
// Lectura bloqueante (espera hasta que haya un valor)
value := <-ch

// Lectura no bloqueante (retorna inmediatamente)
select {
case value := <-ch:
    // hay un valor
case <-ctx.Done():
    // contexto cancelado
default:
    // no hay nada, seguir
}
```

Siempre usa la version no bloqueante en `Update()`. Si bloqueas, la TUI se congela.

### Tip: mostrar costo estimado antes de empezar

```go
func estimateCost(roles []domain.Role, prompts map[string]string) string {
    totalChars := 0
    for _, r := range roles {
        totalChars += len(prompts[r.Name])
    }
    // Estimacion muy burda: ~4 chars por token, input + output
    estimatedTokens := (totalChars / 4) * 3 // input * 1.5 para output estimado
    // Sonnet: ~$3 per million input, ~$15 per million output
    costUSD := float64(estimatedTokens) * 0.000015
    return fmt.Sprintf("~%d tokens, ~$%.4f USD", estimatedTokens, costUSD)
}
```

Esto es una estimacion burda pero le da al usuario una idea antes de gastar tokens.

## Ejercicios

### 1. Basico: contador de mejoras aceptadas

Agrega un contador en la barra de estado de la pantalla de review que muestre "3 aceptados, 1 rechazado" conforme el usuario va decidiendo.

### 2. Intermedio: aceptar/rechazar todo

Agrega una tecla `A` (mayuscula) que acepte todos los diffs pendientes de una vez, y `R` que rechace todos. Mostrar un dialogo de confirmacion antes.

### 3. Avanzado: mejora en paralelo con semaforo

Implementa una version paralela de `ImproveAll` que procese hasta 3 roles simultaneamente usando un semaforo (channel con buffer de 3). El reporte de progreso debe seguir siendo coherente.

### 4. Bonus: diff con contexto colapsable

Muestra solo las lineas que cambiaron mas 2 lineas de contexto arriba y abajo. Las secciones sin cambios se muestran como `... (15 lineas sin cambios) ...`. Usa una tecla para expandir/colapsar.

## Para profundizar

- [Go Concurrency Patterns](https://go.dev/blog/pipelines): fan-out, fan-in, cancellation.
- [sync package](https://pkg.go.dev/sync): WaitGroup, Mutex, Once, y otros primitivos.
- [bubbletea Tick example](https://github.com/charmbracelet/bubbletea/tree/master/examples/spinner): ejemplo oficial de spinner con Tick.
- [context package](https://pkg.go.dev/context): cancellation, timeouts, values.
- [go-diff](https://github.com/sergi/go-diff): algoritmo de diff (Myers) implementado en Go.
- [Data Race Detector](https://go.dev/doc/articles/race_detector): guia oficial del detector de race conditions.

## Que sigue

En la [Clase 12](./12_distribucion.md) convertimos el proyecto en un binario distribuible. Vas a aprender cross-compilation con `GOOS/GOARCH`, `embed` para incluir assets, `ldflags` para inyectar la version en compile time, y goreleaser para releases automatizados en GitHub.
