# Clase 09: Procesos externos -- runtime delegado

> Al terminar esta clase, presionar `i` en un rol lanza el CLI de Claude Code con el prompt inyectado, la TUI cede el control al subproceso, y al salir se lista la sesion en el panel de workspace. Tambien vas a poder listar y retomar sesiones anteriores.

## Prerequisitos

- [Clase 05](./05_git_desde_go.md): interfaces como boundaries de testing, goroutines para operaciones lentas.
- [Clase 08](./08_tareas_y_workspace.md): vista de workspace con panel de tareas.
- Tener `claude` (Claude Code CLI) instalado y funcionando en tu terminal.

## Conceptos de Go que vas a aprender

### 1. `os/exec` -- lanzar procesos externos

El paquete `os/exec` es como `subprocess` de Python pero con mas control:

```go
import "os/exec"

// Python: subprocess.run(["claude", "--print", "hola"])
// Go:
cmd := exec.Command("claude", "--print", "hola")
cmd.Dir = "/home/user/proyecto"  // working directory
output, err := cmd.CombinedOutput()
```

La diferencia fundamental: `CombinedOutput()` espera a que el proceso termine y te da todo el stdout+stderr junto. `Output()` te da solo stdout. Pero ninguno de los dos te sirve para un proceso interactivo como Claude Code.

**Metodos de ejecucion:**

```go
// 1. Run() — ejecutar y esperar (como subprocess.run)
err := cmd.Run()

// 2. Start() + Wait() — lanzar sin esperar (como subprocess.Popen)
err := cmd.Start()
// ... hacer otras cosas ...
err = cmd.Wait()

// 3. Output() — ejecutar, esperar, capturar stdout
output, err := cmd.Output()

// 4. CombinedOutput() — stdout + stderr juntos
output, err := cmd.CombinedOutput()
```

**Equivalente Python:**

```python
import subprocess

# Run y esperar
result = subprocess.run(["claude", "--print", "hola"], capture_output=True, text=True)

# Popen (start + wait)
proc = subprocess.Popen(["claude"], cwd="/home/user/proyecto")
proc.wait()

# Capturar output
output = subprocess.check_output(["claude", "--print", "hola"])
```

**Tradeoff CombinedOutput vs Start+Wait:** `CombinedOutput` es simple pero bloquea el thread que lo llama. Para un CLI interactivo como Claude Code que necesita la terminal completa, vas a necesitar algo diferente -- ahi entra `tea.ExecProcess`.

### 2. `tea.ExecProcess` -- ceder la terminal al subproceso

Bubbletea tiene un mecanismo especial para cuando necesitas darle la terminal completa a otro programa (como lazygit hace cuando abris un editor):

```go
import tea "github.com/charmbracelet/bubbletea"

// Crear el comando
c := exec.Command("claude", "--system-prompt", prompt)
c.Dir = workDir

// tea.ExecProcess cede la terminal y la recupera al terminar
cmd := tea.ExecProcess(c, func(err error) tea.Msg {
    return processFinishedMsg{err: err}
})
```

Cuando bubbletea ejecuta un `tea.ExecProcess`:
1. Suspende el rendering de la TUI.
2. Le da stdin/stdout/stderr al subproceso.
3. El subproceso tiene control total de la terminal.
4. Cuando el subproceso termina, bubbletea retoma el control.
5. El callback se ejecuta y manda un mensaje al loop de Update.

**Esto es exactamente lo que necesitamos.** Claude Code es una TUI completa que necesita la terminal. No podemos capturar su output con pipes porque romperia su interfaz.

**Equivalente Python:** no hay equivalente directo. En textual tendrias que pausar la app, hacer `os.system()`, y reiniciar. Es mucho mas manual.

```
                    lazyharness TUI
                          |
                    [usuario presiona 'i']
                          |
                    tea.ExecProcess(claude ...)
                          |
              +-----------+-----------+
              |                       |
         TUI suspendida         Claude Code activo
         (no renderiza)         (controla la terminal)
              |                       |
              |                 [usuario sale de claude]
              |                       |
              +-----------+-----------+
                          |
                    processFinishedMsg
                          |
                    TUI retomada
```

### 3. Goroutines y channels para I/O concurrente

Cuando queres capturar output de un proceso (no interactivo) sin bloquear la TUI, usas goroutines con channels:

```go
// Channel para recibir el resultado
type execResult struct {
    output string
    err    error
}

func runInBackground(cmd *exec.Cmd) <-chan execResult {
    ch := make(chan execResult, 1) // buffer de 1 para no bloquear

    go func() {
        out, err := cmd.CombinedOutput()
        ch <- execResult{output: string(out), err: err}
    }()

    return ch
}
```

**Equivalente Python:**

```python
import asyncio

async def run_in_background(cmd):
    proc = await asyncio.create_subprocess_exec(
        *cmd, stdout=asyncio.subprocess.PIPE, stderr=asyncio.subprocess.PIPE
    )
    stdout, stderr = await proc.communicate()
    return stdout.decode()
```

**Tradeoff buffered vs unbuffered channels:**

| Tipo | Declaracion | Comportamiento |
|------|-------------|----------------|
| Unbuffered | `make(chan T)` | El sender se bloquea hasta que alguien lea |
| Buffered | `make(chan T, n)` | El sender puede poner hasta n valores sin bloquear |

Para resultados de procesos, usa buffer de 1: la goroutine pone el resultado y termina, sin esperar a que alguien lo lea.

### 4. `io.Reader` y streaming de output

Si necesitas leer la salida de un proceso linea por linea (por ejemplo para mostrar un log en tiempo real), usas pipes:

```go
cmd := exec.Command("claude", "--print", "explicame este codigo")
cmd.Dir = workDir

stdout, err := cmd.StdoutPipe()
if err != nil {
    return err
}

if err := cmd.Start(); err != nil {
    return err
}

// Leer linea por linea
scanner := bufio.NewScanner(stdout)
for scanner.Scan() {
    line := scanner.Text()
    // Mandar cada linea como mensaje a la TUI
    p.Send(outputLineMsg{line: line})
}

cmd.Wait()
```

**Equivalente Python:**

```python
proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, text=True)
for line in proc.stdout:
    print(line, end="")
proc.wait()
```

**Ojo:** leer de un pipe y usar `tea.ExecProcess` son mutuamente excluyentes. Si conectas un pipe, el subproceso ya no tiene acceso directo a la terminal. Elegir uno u otro es un tradeoff importante que discutimos abajo.

### 5. Interface para testabilidad: Executor

Definir una interface permite mockear la ejecucion en tests:

```go
// internal/runtime/executor.go
type Executor interface {
    // Exec cede la terminal al proceso (para CLIs interactivos)
    Exec(ctx context.Context, roleName, prompt, workDir string) (*exec.Cmd, error)

    // Run ejecuta sin terminal y retorna el output (para operaciones headless)
    Run(ctx context.Context, prompt, workDir string) (string, error)
}
```

En tests, un mock retorna resultados predefinidos:

```go
type mockExecutor struct {
    output string
    err    error
}

func (m *mockExecutor) Exec(ctx context.Context, role, prompt, dir string) (*exec.Cmd, error) {
    // Para tests, retornamos un comando que simplemente sale
    return exec.Command("echo", "mock session started"), nil
}

func (m *mockExecutor) Run(ctx context.Context, prompt, dir string) (string, error) {
    return m.output, m.err
}
```

**Equivalente Python:** un `Protocol` con un stub:

```python
class Executor(Protocol):
    def exec(self, role: str, prompt: str, work_dir: str) -> subprocess.Popen: ...
    def run(self, prompt: str, work_dir: str) -> str: ...

class MockExecutor:
    def __init__(self, output="", err=None):
        self.output = output
        self.err = err
    def run(self, prompt, work_dir):
        if self.err: raise self.err
        return self.output
```

### 6. `context.Context` para timeout y cancelacion

Un proceso externo puede colgarse. `context.Context` te da control:

```go
import "context"

// Con timeout: si claude no responde en 5 minutos, matar
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

cmd := exec.CommandContext(ctx, "claude", "--print", prompt)
output, err := cmd.Output()

if ctx.Err() == context.DeadlineExceeded {
    log.Println("claude tardo demasiado, proceso cancelado")
}
```

`exec.CommandContext` automaticamente mata el proceso si el contexto se cancela. No tenes que manejar senales manualmente.

**Equivalente Python:**

```python
try:
    result = subprocess.run(["claude", "--print", prompt], timeout=300)
except subprocess.TimeoutExpired:
    print("claude tardo demasiado")
```

**Tradeoff:** para `tea.ExecProcess` (interactivo), NO uses timeout. El usuario esta interactuando y no sabes cuanto va a tardar. Para `Run` (headless), si pone timeout.

## Lo que vamos a construir

1. `internal/runtime/executor.go` -- interface Executor y tipos compartidos.
2. `internal/runtime/claude.go` -- implementacion que lanza Claude Code CLI.
3. `internal/domain/session.go` -- struct Session para registrar sesiones.
4. Integracion con la vista workspace: invocar con `i`, listar sesiones.

```
  Vista de Harness                     Vista de Workspace
  ┌─ dev-flow ──────────────────┐     ┌─ dev-flow ── workspace ──────┐
  │ ┌─ Roles ──┐ ┌─ prompt ──┐ │     │ ┌─ Sesiones ─┐ ┌─ Tareas ─┐ │
  │ │▸ arquit. │ │ <role>    │ │     │ │ #1 arquit. │ │ T-001... │ │
  │ │  review  │ │ Sos el ...│ │     │ │ #2 review  │ │ T-002... │ │
  │ │  dev     │ │           │ │     │ │ #3 dev     │ │          │ │
  │ └──────────┘ └───────────┘ │     │ └────────────┘ └──────────┘ │
  │  [i] invocar               │     │  [r] retomar  [enter] ver   │
  └────────────────────────────┘     └────────────────────────────┘
         |                                    |
         | presiona 'i'                       | presiona 'r'
         v                                    v
  ┌────────────────────────────────────────────────────────────────┐
  │                                                                │
  │              Claude Code (controla la terminal)                │
  │              prompt del rol inyectado                          │
  │              working dir = directorio del proyecto             │
  │                                                                │
  └────────────────────────────────────────────────────────────────┘
```

**Archivos a crear:**
- `internal/runtime/executor.go`
- `internal/runtime/executor_test.go`
- `internal/runtime/claude.go`
- `internal/runtime/claude_test.go`
- `internal/domain/session.go`
- `internal/domain/session_test.go`

**Archivos a modificar:**
- `internal/tui/workspace/model.go` -- agregar panel de sesiones e invocar
- `internal/tui/harness/model.go` -- accion `i` navega al workspace con invocar
- `internal/tui/keys.go` -- agregar WorkspaceKeys con retomar

## Implementacion paso a paso

### Paso 1: Definir la interface Executor y tipos compartidos

Crea `internal/runtime/executor.go`:

```go
package runtime

import (
    "context"
    "os/exec"
)

// ExecResult es el resultado de una ejecucion headless
type ExecResult struct {
    Output   string
    ExitCode int
}

// Executor define como lanzar CLIs agentic
type Executor interface {
    // Exec construye el comando para ceder la terminal (tea.ExecProcess)
    Exec(ctx context.Context, roleName, prompt, workDir string) (*exec.Cmd, error)

    // Run ejecuta en modo headless y retorna el output
    Run(ctx context.Context, prompt, workDir string) (ExecResult, error)

    // Name retorna el nombre del CLI (para mostrar en la UI)
    Name() string
}
```

Fijate que `Exec` retorna un `*exec.Cmd` en vez de ejecutarlo directamente. Esto es porque bubbletea necesita el `*exec.Cmd` para pasarselo a `tea.ExecProcess`. La ejecucion real la maneja bubbletea.

### Paso 2: Implementar el adapter de Claude Code

Crea `internal/runtime/claude.go`:

```go
package runtime

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "strings"
)

// ClaudeExecutor lanza el CLI de Claude Code
type ClaudeExecutor struct {
    binaryPath string // path al binario de claude (default: "claude")
}

// NewClaudeExecutor crea un executor que busca "claude" en el PATH
func NewClaudeExecutor() *ClaudeExecutor {
    return &ClaudeExecutor{binaryPath: "claude"}
}

// NewClaudeExecutorWithPath crea un executor con un path especifico al binario
func NewClaudeExecutorWithPath(path string) *ClaudeExecutor {
    return &ClaudeExecutor{binaryPath: path}
}

func (c *ClaudeExecutor) Name() string {
    return "Claude Code"
}

// Exec construye el comando para ejecucion interactiva
func (c *ClaudeExecutor) Exec(ctx context.Context, roleName, prompt, workDir string) (*exec.Cmd, error) {
    // Verificar que el binario existe
    if _, err := exec.LookPath(c.binaryPath); err != nil {
        return nil, fmt.Errorf("no se encontro '%s' en el PATH: %w", c.binaryPath, err)
    }

    // Construir los argumentos
    // Claude Code acepta --system-prompt para inyectar el prompt del rol
    args := []string{
        "--system-prompt", prompt,
    }

    cmd := exec.CommandContext(ctx, c.binaryPath, args...)
    cmd.Dir = workDir

    // Heredar el entorno del proceso padre
    cmd.Env = os.Environ()

    // Para tea.ExecProcess, NO conectamos pipes
    // bubbletea va a conectar stdin/stdout/stderr directamente
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    return cmd, nil
}

// Run ejecuta en modo headless (sin terminal) y retorna el output
func (c *ClaudeExecutor) Run(ctx context.Context, prompt, workDir string) (ExecResult, error) {
    if _, err := exec.LookPath(c.binaryPath); err != nil {
        return ExecResult{}, fmt.Errorf("no se encontro '%s' en el PATH: %w", c.binaryPath, err)
    }

    // --print ejecuta sin TUI y retorna el resultado como texto
    args := []string{
        "--print",
        "--system-prompt", prompt,
    }

    cmd := exec.CommandContext(ctx, c.binaryPath, args...)
    cmd.Dir = workDir
    cmd.Env = os.Environ()

    output, err := cmd.CombinedOutput()

    exitCode := 0
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            exitCode = exitErr.ExitCode()
        } else {
            return ExecResult{}, fmt.Errorf("ejecutando claude: %w", err)
        }
    }

    return ExecResult{
        Output:   strings.TrimSpace(string(output)),
        ExitCode: exitCode,
    }, nil
}
```

**Tradeoff inyeccion de prompt: --system-prompt vs archivo temporal vs stdin:**

| Metodo | Ventaja | Desventaja |
|--------|---------|------------|
| `--system-prompt` flag | Simple, directo | El prompt aparece en `ps aux` (seguridad) |
| Archivo temporal | Prompts largos, no visible en ps | Hay que limpiar el archivo |
| stdin pipe | No visible, no archivo | Rompe la interaccion con la TUI |

Elegimos `--system-prompt` porque es lo mas simple y los prompts no son secretos. Para prompts muy largos, se podria usar `--system-prompt-file` si el CLI lo soporta.

### Paso 3: Definir el struct Session

Crea `internal/domain/session.go`:

```go
package domain

import "time"

// SessionStatus es el estado de una sesion
type SessionStatus string

const (
    SessionActive   SessionStatus = "active"
    SessionFinished SessionStatus = "finished"
    SessionFailed   SessionStatus = "failed"
)

// Session representa una sesion de trabajo con un CLI agentic
type Session struct {
    ID        string        `json:"id"`
    RoleName  string        `json:"role_name"`
    Provider  string        `json:"provider"`   // "claude", "aider", etc
    StartedAt time.Time     `json:"started_at"`
    EndedAt   *time.Time    `json:"ended_at,omitempty"`
    Status    SessionStatus `json:"status"`
    ExitCode  int           `json:"exit_code,omitempty"`
    Notes     string        `json:"notes,omitempty"` // notas del usuario
}

// NewSession crea una sesion nueva con estado active
func NewSession(id, roleName, provider string) Session {
    return Session{
        ID:        id,
        RoleName:  roleName,
        Provider:  provider,
        StartedAt: time.Now(),
        Status:    SessionActive,
    }
}

// Finish marca la sesion como terminada
func (s *Session) Finish(exitCode int) {
    now := time.Now()
    s.EndedAt = &now
    s.ExitCode = exitCode
    if exitCode == 0 {
        s.Status = SessionFinished
    } else {
        s.Status = SessionFailed
    }
}

// Duration retorna la duracion de la sesion
func (s Session) Duration() time.Duration {
    end := time.Now()
    if s.EndedAt != nil {
        end = *s.EndedAt
    }
    return end.Sub(s.StartedAt)
}

// IsActive retorna true si la sesion esta en curso
func (s Session) IsActive() bool {
    return s.Status == SessionActive
}

// DisplayDuration retorna la duracion formateada para la UI
func (s Session) DisplayDuration() string {
    d := s.Duration()
    if d < time.Minute {
        return fmt.Sprintf("%ds", int(d.Seconds()))
    }
    if d < time.Hour {
        return fmt.Sprintf("%dm", int(d.Minutes()))
    }
    return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
```

Fijate el uso de `*time.Time` para `EndedAt`: un puntero puede ser `nil`, lo que representa "todavia no termino". En JSON se serializa como `null`. En Python usarias `Optional[datetime]`.

### Paso 4: Tests del executor con mock

Crea `internal/runtime/executor_test.go`:

```go
package runtime

import (
    "context"
    "os/exec"
    "testing"
    "time"
)

// MockExecutor para tests
type MockExecutor struct {
    ExecCalled bool
    RunCalled  bool
    RunOutput  string
    RunErr     error
    LastPrompt string
    LastDir    string
}

func (m *MockExecutor) Name() string { return "mock" }

func (m *MockExecutor) Exec(ctx context.Context, role, prompt, dir string) (*exec.Cmd, error) {
    m.ExecCalled = true
    m.LastPrompt = prompt
    m.LastDir = dir
    return exec.Command("echo", "mock"), nil
}

func (m *MockExecutor) Run(ctx context.Context, prompt, dir string) (ExecResult, error) {
    m.RunCalled = true
    m.LastPrompt = prompt
    m.LastDir = dir
    return ExecResult{Output: m.RunOutput}, m.RunErr
}

func TestMockExecutor_Exec(t *testing.T) {
    mock := &MockExecutor{}
    ctx := context.Background()

    cmd, err := mock.Exec(ctx, "arquitecto", "Sos el arquitecto", "/tmp/proyecto")
    if err != nil {
        t.Fatalf("no esperaba error: %v", err)
    }

    if !mock.ExecCalled {
        t.Error("Exec no fue llamado")
    }
    if mock.LastPrompt != "Sos el arquitecto" {
        t.Errorf("prompt esperado 'Sos el arquitecto', obtuve '%s'", mock.LastPrompt)
    }
    if mock.LastDir != "/tmp/proyecto" {
        t.Errorf("dir esperado '/tmp/proyecto', obtuve '%s'", mock.LastDir)
    }
    if cmd == nil {
        t.Error("cmd no deberia ser nil")
    }
}

func TestMockExecutor_Run(t *testing.T) {
    mock := &MockExecutor{RunOutput: "respuesta del agente"}
    ctx := context.Background()

    result, err := mock.Run(ctx, "analiza este codigo", "/tmp/proyecto")
    if err != nil {
        t.Fatalf("no esperaba error: %v", err)
    }

    if result.Output != "respuesta del agente" {
        t.Errorf("output esperado 'respuesta del agente', obtuve '%s'", result.Output)
    }
}
```

Crea `internal/runtime/claude_test.go`:

```go
package runtime

import (
    "context"
    "os/exec"
    "testing"
)

func TestClaudeExecutor_ExecBinaryNotFound(t *testing.T) {
    // Usar un path que no existe
    ce := NewClaudeExecutorWithPath("/no/existe/claude")
    ctx := context.Background()

    _, err := ce.Exec(ctx, "test", "prompt", "/tmp")
    if err == nil {
        t.Error("esperaba error cuando el binario no existe")
    }
}

func TestClaudeExecutor_ExecBuildsCorrectCommand(t *testing.T) {
    // Solo testear si 'echo' existe (siempre existe en Linux)
    // Esto verifica la logica de construccion del comando
    ce := &ClaudeExecutor{binaryPath: "echo"}
    ctx := context.Background()

    cmd, err := ce.Exec(ctx, "arquitecto", "Sos el arquitecto", "/tmp")
    if err != nil {
        t.Fatalf("no esperaba error: %v", err)
    }

    if cmd.Dir != "/tmp" {
        t.Errorf("Dir esperado '/tmp', obtuve '%s'", cmd.Dir)
    }

    // Verificar que los args incluyen --system-prompt
    found := false
    for _, arg := range cmd.Args {
        if arg == "--system-prompt" {
            found = true
            break
        }
    }
    if !found {
        t.Errorf("args no incluyen --system-prompt: %v", cmd.Args)
    }
}

func TestClaudeExecutor_RunWithContext(t *testing.T) {
    // Verificar que un contexto cancelado cancela la ejecucion
    ctx, cancel := context.WithCancel(context.Background())
    cancel() // cancelar inmediatamente

    ce := &ClaudeExecutor{binaryPath: "sleep"}
    _, err := ce.Run(ctx, "test", "/tmp")

    // Deberia fallar porque el contexto ya esta cancelado
    if err == nil {
        t.Error("esperaba error con contexto cancelado")
    }
}
```

### Paso 5: Tests de Session

Crea `internal/domain/session_test.go`:

```go
package domain

import (
    "encoding/json"
    "testing"
    "time"
)

func TestNewSession(t *testing.T) {
    s := NewSession("sess-001", "arquitecto", "claude")

    if s.ID != "sess-001" {
        t.Errorf("ID esperado 'sess-001', obtuve '%s'", s.ID)
    }
    if s.Status != SessionActive {
        t.Errorf("status esperado 'active', obtuve '%s'", s.Status)
    }
    if s.EndedAt != nil {
        t.Error("EndedAt deberia ser nil para sesion nueva")
    }
}

func TestSession_Finish(t *testing.T) {
    s := NewSession("sess-001", "arquitecto", "claude")

    s.Finish(0)

    if s.Status != SessionFinished {
        t.Errorf("status esperado 'finished', obtuve '%s'", s.Status)
    }
    if s.EndedAt == nil {
        t.Error("EndedAt no deberia ser nil despues de Finish")
    }
    if s.ExitCode != 0 {
        t.Errorf("exit code esperado 0, obtuve %d", s.ExitCode)
    }
}

func TestSession_FinishWithError(t *testing.T) {
    s := NewSession("sess-001", "dev", "claude")

    s.Finish(1)

    if s.Status != SessionFailed {
        t.Errorf("status esperado 'failed', obtuve '%s'", s.Status)
    }
}

func TestSession_Duration(t *testing.T) {
    s := NewSession("sess-001", "arquitecto", "claude")
    // Forzar un StartedAt en el pasado
    s.StartedAt = time.Now().Add(-5 * time.Minute)

    d := s.Duration()
    if d < 4*time.Minute || d > 6*time.Minute {
        t.Errorf("duracion esperada ~5m, obtuve %v", d)
    }
}

func TestSession_JSON(t *testing.T) {
    s := NewSession("sess-001", "arquitecto", "claude")

    data, err := json.Marshal(s)
    if err != nil {
        t.Fatalf("error serializando: %v", err)
    }

    var loaded Session
    if err := json.Unmarshal(data, &loaded); err != nil {
        t.Fatalf("error deserializando: %v", err)
    }

    if loaded.ID != s.ID {
        t.Errorf("ID no coincide: esperado '%s', obtuve '%s'", s.ID, loaded.ID)
    }
    if loaded.Status != SessionActive {
        t.Errorf("status esperado 'active', obtuve '%s'", loaded.Status)
    }
}

func TestSession_JSONWithEndedAt(t *testing.T) {
    s := NewSession("sess-001", "arquitecto", "claude")
    s.Finish(0)

    data, err := json.Marshal(s)
    if err != nil {
        t.Fatal(err)
    }

    var loaded Session
    json.Unmarshal(data, &loaded)

    if loaded.EndedAt == nil {
        t.Error("EndedAt deberia estar presente despues de Finish")
    }
}
```

### Paso 6: Integrar la invocacion en la TUI

En `internal/tui/harness/model.go`, agregar el manejo de la tecla `i`:

```go
// Mensajes para invocar un rol
type invokeRoleMsg struct {
    roleName string
    prompt   string
}

type processFinishedMsg struct {
    err      error
    roleName string
}

// En el Update del harness model:
case tea.KeyMsg:
    switch {
    case key.Matches(msg, maintui.HarnessKeyMap.Invoke):
        if len(m.harness.Roles) == 0 {
            break
        }
        role := m.harness.Roles[m.selectedRole]
        return m, func() tea.Msg {
            return invokeRoleMsg{
                roleName: role.Name,
                prompt:   m.promptContent,
            }
        }
    }

// En app.go, manejar el mensaje:
case harness.InvokeRoleMsg:
    // Construir el comando
    cmd, err := a.executor.Exec(
        context.Background(),
        msg.RoleName,
        msg.Prompt,
        a.currentHarness.ProjectDir,
    )
    if err != nil {
        // Mostrar error en la UI
        break
    }

    // Registrar la sesion
    sessionID := fmt.Sprintf("sess-%d", time.Now().UnixMilli())
    session := domain.NewSession(sessionID, msg.RoleName, a.executor.Name())
    a.sessions = append(a.sessions, session)

    // Ceder la terminal a Claude Code
    return a, tea.ExecProcess(cmd, func(err error) tea.Msg {
        return harness.ProcessFinishedMsg{
            Err:      err,
            RoleName: msg.RoleName,
        }
    })

case harness.ProcessFinishedMsg:
    // El subproceso termino — actualizar la sesion
    for i := range a.sessions {
        if a.sessions[i].RoleName == msg.RoleName && a.sessions[i].IsActive() {
            exitCode := 0
            if msg.Err != nil {
                exitCode = 1
            }
            a.sessions[i].Finish(exitCode)
            break
        }
    }
    // La TUI se reanuda automaticamente
```

### Paso 7: Panel de sesiones en el workspace

Modifica `internal/tui/workspace/model.go` para agregar la lista de sesiones:

```go
// Agregar al Model:
type Model struct {
    // ... campos existentes de tareas ...
    sessions    []domain.Session
    selectedSes int
    activePanel workspacePanel
    executor    runtime.Executor
}

type workspacePanel int

const (
    panelSessions workspacePanel = iota
    panelTasks
)

// Renderizar la lista de sesiones:
func (m Model) renderSessions() string {
    var b strings.Builder

    for i, s := range m.sessions {
        prefix := "  "
        if i == m.selectedSes {
            prefix = "▸ "
        }

        // Icono de estado
        icon := "●" // active
        switch s.Status {
        case domain.SessionFinished:
            icon = "✓"
        case domain.SessionFailed:
            icon = "✗"
        }

        line := fmt.Sprintf("%s%s %s [%s] %s",
            prefix, icon, s.RoleName, s.DisplayDuration(), s.Status)

        if i == m.selectedSes {
            line = lipgloss.NewStyle().Bold(true).Render(line)
        }

        b.WriteString(line + "\n")
    }

    return b.String()
}

// Retomar sesion con 'r':
case key.Matches(msg, keys.Resume):
    if len(m.sessions) == 0 {
        break
    }
    s := m.sessions[m.selectedSes]
    // Claude Code soporta retomar con --resume <session-id>
    // Esto lo manejamos con un mensaje al app.go
    return m, func() tea.Msg {
        return resumeSessionMsg{sessionID: s.ID, roleName: s.RoleName}
    }
```

## Tradeoffs y decisiones de diseno

### Decision 1: `tea.ExecProcess` vs pipes + viewport

La decision mas importante de esta clase.

**Opcion A -- tea.ExecProcess (elegida):**
- Claude Code tiene control total de la terminal.
- El usuario interactua con Claude Code como si lo hubiera abierto normalmente.
- No podemos capturar ni mostrar el output dentro de nuestra TUI.
- Es como Alt-Tab entre dos apps.

**Opcion B -- Pipes + viewport:**
- Capturamos stdout/stderr y lo mostramos en un viewport de bubbletea.
- Podemos hacer overlay, mostrar tareas al costado, etc.
- Pero rompemos la TUI de Claude Code (que necesita la terminal raw).
- Solo funciona para `--print` (modo headless).

**Por que elegimos A:** Claude Code es una TUI completa. Si le robas la terminal, su interfaz se rompe. Es mejor ceder el control y retomarlo cuando termina. Lazygit hace exactamente esto cuando abris un editor externo.

### Decision 2: Interface Executor con dos metodos

`Exec` para interactivo, `Run` para headless. Son dos use cases distintos:

- **Exec:** el usuario presiona `i` e interactua con Claude Code. Necesita la terminal.
- **Run:** la feature "mejorar" (clase 11) llama a la IA en background sin terminal.

Podriamos haber tenido una sola interface pero los contratos son tan diferentes (uno retorna un `*exec.Cmd`, el otro retorna un string) que separarlos es mas claro.

### Decision 3: Sesiones como concepto propio vs leer del CLI

Mantenemos nuestro propio registro de sesiones en vez de parsear los archivos internos de Claude Code. Las razones:

- **Portabilidad:** si cambiamos de CLI, nuestras sesiones siguen funcionando.
- **Simpleza:** no dependemos de la estructura interna de Claude Code (que puede cambiar).
- **Extension:** podemos agregar campos propios (notas, vinculacion con tareas).

El tradeoff es que no podemos listar sesiones que el usuario abrio fuera de lazyharness.

## Errores comunes y tips

### Error: goroutine leak por channel sin leer

```go
// MAL: si nadie lee del channel, la goroutine queda colgada para siempre
ch := make(chan string)
go func() {
    result := heavyWork()
    ch <- result  // se bloquea aca si nadie lee
}()
// ... olvidamos leer de ch

// BIEN: usar channel con buffer
ch := make(chan string, 1)
go func() {
    result := heavyWork()
    ch <- result  // no se bloquea, pone en el buffer
}()
// Si nadie lee, la goroutine termina igual
```

### Error: deadlock leyendo pipe despues de que el proceso termino

```go
// MAL: si el proceso ya termino, leer del pipe puede bloquear
cmd.Start()
cmd.Wait()              // esperar a que termine
scanner.Scan()           // deadlock! el pipe ya se cerro

// BIEN: leer primero, esperar despues
cmd.Start()
for scanner.Scan() {    // leer hasta EOF
    // procesar
}
cmd.Wait()              // ahora si esperar
```

### Error: `exec.Command` no encuentra el binario

En WSL, el PATH puede no incluir los binarios de npm global o los instalados con curl. Siempre verifica con `exec.LookPath` antes de ejecutar:

```go
path, err := exec.LookPath("claude")
if err != nil {
    return fmt.Errorf("claude no esta instalado o no esta en el PATH: %w", err)
}
// Usar path en vez del nombre
cmd := exec.Command(path, args...)
```

### Error: no propagar la cancelacion del contexto

```go
// MAL: ignorar el contexto
func (c *ClaudeExecutor) Run(ctx context.Context, prompt, dir string) (ExecResult, error) {
    cmd := exec.Command(c.binaryPath, args...)  // sin contexto!
    // ...
}

// BIEN: usar CommandContext
func (c *ClaudeExecutor) Run(ctx context.Context, prompt, dir string) (ExecResult, error) {
    cmd := exec.CommandContext(ctx, c.binaryPath, args...)  // se cancela con el contexto
    // ...
}
```

### Tip: `exec.ExitError` para obtener el exit code

```go
output, err := cmd.CombinedOutput()
if err != nil {
    if exitErr, ok := err.(*exec.ExitError); ok {
        // El proceso salio con codigo != 0
        fmt.Printf("exit code: %d\n", exitErr.ExitCode())
        fmt.Printf("stderr: %s\n", exitErr.Stderr)
    } else {
        // Error al lanzar el proceso (binario no encontrado, etc)
        return err
    }
}
```

### Tip: testar con `echo` como proceso dummy

En tests, podes usar `echo` como proceso dummy que siempre funciona:

```go
func TestExecProcess(t *testing.T) {
    cmd := exec.Command("echo", "hola mundo")
    output, err := cmd.Output()
    if err != nil {
        t.Fatal(err)
    }
    if string(output) != "hola mundo\n" {
        t.Errorf("output inesperado: %q", string(output))
    }
}
```

## Ejercicios

### 1. Basico: verificar que Claude esta instalado

Escribi una funcion `CheckCLI(name string) error` que use `exec.LookPath` para verificar que un binario existe en el PATH. Si no existe, que retorne un error descriptivo con el nombre del binario y una sugerencia de instalacion. Escribi un test.

### 2. Intermedio: executor generico para multiples CLIs

Crea un `GenericExecutor` que reciba el nombre del binario y los flags como parametros de configuracion. Deberia funcionar para Claude Code, Aider, o cualquier otro CLI agentic. El constructor recibe un struct de config:

```go
type CLIConfig struct {
    Binary         string
    SystemPromptFlag string   // "--system-prompt" para claude
    PrintFlag      string     // "--print" para modo headless
    ResumeFlag     string     // "--resume" para retomar sesion
}
```

### 3. Avanzado: streaming de output con channels

Implementa una funcion `RunStreaming(ctx, cmd) <-chan string` que lance un proceso con `StdoutPipe`, lea linea por linea en una goroutine, y mande cada linea por un channel. El channel se cierra cuando el proceso termina. Verifica que no hay goroutine leaks.

### 4. Bonus: timeout con barra de progreso

Combina `context.WithTimeout` con `tea.Tick` para mostrar una barra de progreso mientras un proceso headless corre. Si se pasa del timeout, mostrar un mensaje y ofrecer cancelar o esperar.

## Para profundizar

- [os/exec package docs](https://pkg.go.dev/os/exec): documentacion oficial del paquete exec.
- [bubbletea ExecProcess example](https://github.com/charmbracelet/bubbletea/tree/master/examples/exec): ejemplo oficial de como ceder la terminal.
- [Go Concurrency Patterns](https://go.dev/blog/pipelines): pipelines, cancellation, fan-out/fan-in.
- [lazygit — RunSubprocess](https://github.com/jesseduffield/lazygit/blob/master/pkg/gui/gui.go): como lazygit implementa la cesion de terminal para editores.
- [Go by Example — Exec'ing Processes](https://gobyexample.com/execing-processes): diferencia entre exec y spawn.
- [context package](https://pkg.go.dev/context): documentacion oficial de context.

## Que sigue

En la [Clase 10](./10_ia_y_streaming.md) agregamos integracion directa con APIs de IA. En vez de delegar al CLI, vamos a hacer llamadas HTTP para streaming de tokens, parsear respuestas SSE, y construir un chat bar dentro del editor. Vas a aprender `net/http`, streaming HTTP, y testing con `httptest`.
