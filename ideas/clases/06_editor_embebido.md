# Clase 06: El editor embebido — edicion y guardado

> Al terminar esta clase, al presionar `e` sobre un rol se abre un editor embebido (textarea multilinea). Ctrl+S guarda el archivo, crea un commit git y vuelve al modo read-only. Esc cancela la edicion.

## Prerequisitos

- [Clase 04](./04_vista_de_harness.md): vista de harness con sidebar, prompt view y action bar.
- [Clase 05](./05_git_desde_go.md): operaciones git con go-git, commit automatico.

## Conceptos de Go que vas a aprender

### 1. bubbles/textarea — edicion multilinea

El componente `textarea` de bubbles es un editor de texto embebido con soporte para multiples lineas, cursor, seleccion y scroll:

```go
import "github.com/charmbracelet/bubbles/textarea"

ta := textarea.New()
ta.Placeholder = "Escribi tu prompt aca..."
ta.SetWidth(80)
ta.SetHeight(20)
ta.Focus()                    // activar para recibir input
ta.SetValue(contenidoActual)  // cargar contenido existente

// En Update:
ta, cmd = ta.Update(msg)      // maneja todo: teclas, cursor, scroll

// En View:
output := ta.View()           // retorna el texto renderizado con cursor
```

**Equivalente Python (textual):**

```python
from textual.widgets import TextArea

class EditorScreen(Screen):
    def compose(self):
        yield TextArea(id="editor")

    def on_mount(self):
        editor = self.query_one("#editor")
        editor.load_text(contenido_actual)
```

**Tradeoff textarea embebido vs lanzar $EDITOR externo:**

| Aspecto | textarea embebido | $EDITOR externo (vim/nano) |
|---------|-------------------|---------------------------|
| UX | Sin salir de la TUI | Sale y vuelve (como lazygit para commits) |
| Poder | Basico (sin macros, sin plugins) | Completo (vim motions, etc.) |
| Futuro | Permite colorear @refs en tiempo real | Necesita plugin del editor |
| Complejidad | Moderada (manejar focus, keybindings) | Baja (lanzar proceso y esperar) |
| Portabilidad | Siempre funciona | Depende de que haya un $EDITOR configurado |

**Decision:** usamos textarea embebido. Mantiene al usuario dentro de la TUI y nos permite agregar features como autocompletado de @referencias y coloreado en tiempo real (clase futura). Si un usuario quiere vim, puede abrir el archivo en otra terminal — los prompts son archivos planos.

### 2. tea.Batch — comandos en paralelo

`tea.Batch` agrupa multiples `tea.Cmd` para ejecutarlos en paralelo. Cada uno produce su propio mensaje cuando termina:

```go
// Ejecutar dos operaciones al mismo tiempo
cmd := tea.Batch(
    cargarArchivoCmd(path),
    verificarGitCmd(dir),
)
// Ambos se ejecutan en goroutines separadas.
// Cuando termina uno, llega su mensaje a Update.
// El orden de llegada no esta garantizado.
```

**Equivalente Python:**

```python
import asyncio

# asyncio.gather ejecuta coroutines en paralelo
results = await asyncio.gather(
    cargar_archivo(path),
    verificar_git(dir),
)
```

**Cuando usar Batch:** cuando tenes operaciones independientes que no dependen una de la otra. Por ejemplo, al abrir el editor: cargar el contenido del archivo Y verificar si hay cambios git al mismo tiempo.

### 3. tea.Sequence — comandos secuenciales

`tea.Sequence` ejecuta comandos uno tras otro. El segundo no arranca hasta que el primero termina y su mensaje se procesa:

```go
// Primero guardar, DESPUES commitear
cmd := tea.Sequence(
    guardarArchivoCmd(path, contenido),
    commitCmd(dir, mensaje),
)
```

**Esto es critico para guardar + commitear.** Si usas `tea.Batch`, el commit podria ejecutarse ANTES de que el archivo se guarde — comiteando la version vieja.

**Equivalente Python:**

```python
# Secuencial es el default en Python sincrono
guardar_archivo(path, contenido)
crear_commit(dir, mensaje)

# O con asyncio:
await guardar_archivo(path, contenido)
await crear_commit(dir, mensaje)
```

**Tradeoff Batch vs Sequence:**

| Operacion | Patron |
|-----------|--------|
| Cargar archivo + verificar git status | `tea.Batch` (independientes) |
| Guardar archivo + crear commit | `tea.Sequence` (el commit depende del save) |
| Guardar + commit + actualizar UI | `tea.Sequence` para las dos primeras, el Msg final actualiza la UI |

**Como funciona tea.Sequence internamente:** Sequence es un Cmd que ejecuta el primer Cmd, espera su Msg, y ahi ejecuta el siguiente. Los mensajes intermedios llegan a Update normalmente — podes usarlos para mostrar progreso ("Guardando..." → "Commiteando..." → "Listo").

### 4. Escritura atomica — temp file + rename

Escribir un archivo directamente puede dejar el archivo corrupto si el proceso muere a la mitad:

```go
// PELIGROSO: si el proceso muere aca, el archivo queda a la mitad
os.WriteFile("prompt.xml", contenido, 0644)
```

La solucion es escribir a un archivo temporal y renombrarlo:

```go
import "os"

func atomicWrite(path string, data []byte) error {
    // 1. Escribir a un archivo temporal en el mismo directorio
    //    (mismo directorio = mismo filesystem = rename atomico)
    dir := filepath.Dir(path)
    tmp, err := os.CreateTemp(dir, ".tmp-*")
    if err != nil {
        return fmt.Errorf("creando archivo temporal: %w", err)
    }
    tmpPath := tmp.Name()

    // 2. Si algo falla, limpiar el temporal
    defer func() {
        if err != nil {
            os.Remove(tmpPath)
        }
    }()

    // 3. Escribir el contenido
    if _, err = tmp.Write(data); err != nil {
        tmp.Close()
        return fmt.Errorf("escribiendo contenido: %w", err)
    }

    // 4. Flush al disco (fsync) — asegura que los datos llegaron al disco
    if err = tmp.Sync(); err != nil {
        tmp.Close()
        return fmt.Errorf("sync al disco: %w", err)
    }

    if err = tmp.Close(); err != nil {
        return fmt.Errorf("cerrando temporal: %w", err)
    }

    // 5. Rename atomico: o pasa completo o no pasa
    if err = os.Rename(tmpPath, path); err != nil {
        return fmt.Errorf("renombrando %s a %s: %w", tmpPath, path, err)
    }

    return nil
}
```

**Equivalente Python:**

```python
import tempfile
import os

def atomic_write(path: str, data: bytes):
    dir = os.path.dirname(path)
    # Python no tiene esto built-in — hay que hacerlo manual
    fd, tmp_path = tempfile.mkstemp(dir=dir)
    try:
        os.write(fd, data)
        os.fsync(fd)
        os.close(fd)
        os.rename(tmp_path, path)
    except:
        os.unlink(tmp_path)
        raise
```

**Tradeoff atomicWrite vs WriteFile directo:**

Para lazyharness, los prompts son archivos chicos (<100KB) y el riesgo de corrupcion es bajo. Pero el costo de atomic write es practicamente cero y la proteccion es total. Ademas, si el usuario esta editando en la TUI y su laptop se queda sin bateria, el archivo no queda corrupto.

**Fijate que `os.Rename` tiene una limitacion:** no funciona entre filesystems distintos. Si el directorio temporal esta en `/tmp` (otro filesystem) y el archivo destino esta en `/home`, el rename falla con `invalid cross-device link`. Por eso creamos el temporal en el MISMO directorio que el archivo destino.

## Lo que vamos a construir

La vista de edicion del mockup 03 (version simplificada — sin la asistencia con IA por ahora):

```
+-- dev-flow -- ~/dev/shop-api -- editando: arquitecto.xml --+
|                                                              |
| +-- Roles --------+ +-- EDITOR: arquitecto.xml ----------+ |
| |                  | |                                     | |
| | > arquitecto     | | <role>                              | |
| |   +- reviewer    | |   Sos el arquitecto del proyecto.   | |
| |   +- dev-back    | |   Tu responsabilidad es disenar|    | |
| |                  | |   la estructura del codigo.         | |
| |                  | | </role>                             | |
| |                  | | <constraints>                       | |
| |                  | |   Consulta con @code-reviewer       | |
| |                  | | </constraints>                      | |
| |                  | |                                     | |
| +------------------+ +-------------------------------------+ |
|                                                              |
|  [*] modificado (sin guardar)                                |
+----------------------------------------------+---------------+
|  ctrl+s guardar  esc cancelar  ctrl+z deshacer               |
+--------------------------------------------------------------+
```

**Archivos a crear:**
- `internal/tui/editor/model.go`
- `internal/tui/editor/model_test.go`

**Archivos a modificar:**
- `internal/storage/filesystem.go` — agregar `SavePromptAtomic`
- `internal/tui/harness/model.go` — abrir editor al presionar `e`
- `internal/tui/app.go` — agregar routing a screenEditor
- `internal/tui/keys.go` — agregar EditorKeys

## Implementacion paso a paso

### Paso 1: Agregar escritura atomica al storage

Agrega a `internal/storage/filesystem.go`:

```go
// SavePromptAtomic escribe un prompt de forma atomica (temp + rename)
func (s Store) SavePromptAtomic(projectDir, promptFile string, content []byte) error {
    path := filepath.Join(projectDir, harnessDir, promptsDir, promptFile)

    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("creando directorio: %w", err)
    }

    // Crear temporal en el mismo directorio
    tmp, err := os.CreateTemp(dir, ".tmp-*")
    if err != nil {
        return fmt.Errorf("creando temporal: %w", err)
    }
    tmpPath := tmp.Name()

    // Limpiar si falla
    success := false
    defer func() {
        if !success {
            os.Remove(tmpPath)
        }
    }()

    if _, err := tmp.Write(content); err != nil {
        tmp.Close()
        return fmt.Errorf("escribiendo: %w", err)
    }

    if err := tmp.Sync(); err != nil {
        tmp.Close()
        return fmt.Errorf("sync: %w", err)
    }

    if err := tmp.Close(); err != nil {
        return fmt.Errorf("cerrando temporal: %w", err)
    }

    if err := os.Rename(tmpPath, path); err != nil {
        return fmt.Errorf("rename atomico: %w", err)
    }

    success = true
    return nil
}
```

### Paso 2: Agregar keybindings del editor

Agrega a `internal/tui/keys.go`:

```go
// EditorKeys son atajos de la vista de edicion
type EditorKeys struct {
    Save   key.Binding
    Cancel key.Binding
}

var EditorKeyMap = EditorKeys{
    Save: key.NewBinding(
        key.WithKeys("ctrl+s"),
        key.WithHelp("ctrl+s", "guardar"),
    ),
    Cancel: key.NewBinding(
        key.WithKeys("esc"),
        key.WithHelp("esc", "cancelar"),
    ),
}
```

### Paso 3: Crear el modelo del editor

Crea `internal/tui/editor/model.go`:

```go
package editor

import (
    "github.com/charmbracelet/bubbles/key"
    "github.com/charmbracelet/bubbles/textarea"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    maintui "github.com/Gerardo1909/lazyharness/internal/tui"
)

// --- Mensajes ---

// BackToHarnessMsg indica que el editor se cerro (con o sin guardar)
type BackToHarnessMsg struct {
    Saved bool
}

// fileSavedMsg indica que el archivo se guardo exitosamente
type fileSavedMsg struct {
    err error
}

// commitDoneMsg indica que el commit termino
type commitDoneMsg struct {
    hash string
    err  error
}

// --- SaveFunc y CommitFunc permiten inyectar dependencias ---

// SaveFunc es una funcion que guarda contenido en un archivo
type SaveFunc func(content []byte) error

// CommitFunc es una funcion que crea un commit git
type CommitFunc func(message string) (string, error)

// --- Modelo ---

// Model es el estado del editor embebido
type Model struct {
    textarea      textarea.Model
    roleName      string
    promptFile    string
    originalContent string  // contenido al abrir — para detectar cambios
    modified      bool
    saving        bool
    statusMsg     string
    width         int
    height        int

    // Funciones inyectadas — permite testear sin filesystem real
    saveFn   SaveFunc
    commitFn CommitFunc
}

// NewModel crea un editor con el contenido actual del prompt
func NewModel(
    roleName, promptFile, content string,
    width, height int,
    saveFn SaveFunc,
    commitFn CommitFunc,
) Model {
    ta := textarea.New()
    ta.SetValue(content)
    ta.SetWidth(width - 4)   // margen para bordes
    ta.SetHeight(height - 8) // espacio para header, status y keybar
    ta.Focus()
    ta.CharLimit = 0 // sin limite de caracteres

    return Model{
        textarea:        ta,
        roleName:        roleName,
        promptFile:      promptFile,
        originalContent: content,
        modified:        false,
        width:           width,
        height:          height,
        saveFn:          saveFn,
        commitFn:        commitFn,
    }
}

func (m Model) Init() tea.Cmd {
    return textarea.Blink // iniciar el cursor parpadeante
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch {
        case key.Matches(msg, maintui.EditorKeyMap.Save):
            if m.saving {
                return m, nil // ya estamos guardando
            }
            if !m.modified {
                m.statusMsg = "Sin cambios para guardar"
                return m, nil
            }
            m.saving = true
            m.statusMsg = "Guardando..."

            // tea.Sequence: primero guardar, despues commitear
            content := []byte(m.textarea.Value())
            return m, tea.Sequence(
                m.saveFileCmd(content),
                m.commitFileCmd(),
            )

        case key.Matches(msg, maintui.EditorKeyMap.Cancel):
            return m, func() tea.Msg {
                return BackToHarnessMsg{Saved: false}
            }
        }

    case fileSavedMsg:
        if msg.err != nil {
            m.saving = false
            m.statusMsg = "Error al guardar: " + msg.err.Error()
            return m, nil
        }
        m.statusMsg = "Archivo guardado, commiteando..."
        return m, nil // el commit viene por tea.Sequence

    case commitDoneMsg:
        m.saving = false
        if msg.err != nil {
            m.statusMsg = "Guardado pero error en commit: " + msg.err.Error()
            // El archivo se guardo, pero el commit fallo.
            // No es catastrofico — el usuario puede reintentar.
        } else {
            m.statusMsg = "Guardado y commiteado: " + msg.hash[:7]
            m.originalContent = m.textarea.Value()
            m.modified = false
        }
        // Volver a la vista de harness
        return m, func() tea.Msg {
            return BackToHarnessMsg{Saved: true}
        }

    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.textarea.SetWidth(msg.Width - 4)
        m.textarea.SetHeight(msg.Height - 8)
    }

    // Delegar al textarea
    var cmd tea.Cmd
    m.textarea, cmd = m.textarea.Update(msg)

    // Detectar si el contenido cambio
    m.modified = m.textarea.Value() != m.originalContent

    return m, cmd
}

// saveFileCmd crea un Cmd que guarda el archivo
func (m Model) saveFileCmd(content []byte) tea.Cmd {
    return func() tea.Msg {
        err := m.saveFn(content)
        return fileSavedMsg{err: err}
    }
}

// commitFileCmd crea un Cmd que hace el commit
func (m Model) commitFileCmd() tea.Cmd {
    return func() tea.Msg {
        message := "actualizo prompt de " + m.roleName
        hash, err := m.commitFn(message)
        return commitDoneMsg{hash: hash, err: err}
    }
}

func (m Model) View() string {
    // Header
    headerStyle := lipgloss.NewStyle().Bold(true).Foreground(maintui.ColorFg)
    header := headerStyle.Render("EDITOR: "+m.promptFile) + " "

    if m.modified {
        modStyle := lipgloss.NewStyle().
            Foreground(maintui.ColorYellow).
            Bold(true)
        header += modStyle.Render("[*] modificado")
    }

    // Textarea
    editorPanel := m.textarea.View()

    // Status
    statusStyle := lipgloss.NewStyle().
        Foreground(maintui.ColorComment).
        Italic(true)
    status := statusStyle.Render(m.statusMsg)

    // Keybar
    keyStyle := lipgloss.NewStyle().
        Bold(true).
        Foreground(maintui.ColorFg).
        Background(maintui.ColorSelection).
        Padding(0, 1)
    descStyle := lipgloss.NewStyle().
        Foreground(maintui.ColorComment).
        MarginRight(2)

    keybar := keyStyle.Render("ctrl+s") + " " + descStyle.Render("guardar") +
        keyStyle.Render("esc") + " " + descStyle.Render("cancelar")

    return lipgloss.JoinVertical(lipgloss.Left,
        header,
        "",
        editorPanel,
        "",
        status,
        keybar,
    )
}
```

**Fijate como inyectamos `saveFn` y `commitFn`** en vez de pasar el Store y GitRepo directamente. Esto hace que el editor sea testeable: en tests pasamos funciones fake, en produccion pasamos las reales.

### Paso 4: Integrar el editor con la vista de harness

Modifica `internal/tui/harness/model.go` para abrir el editor al presionar `e`:

```go
// Agregar al Update, dentro del case tea.KeyMsg:
case key.Matches(msg, maintui.HarnessKeyMap.Edit):
    if len(m.harness.Roles) == 0 {
        return m, nil
    }
    role := m.harness.Roles[m.selectedRole]

    // Enviar mensaje para que app.go abra el editor
    return m, func() tea.Msg {
        return OpenEditorMsg{
            RoleName:   role.Name,
            PromptFile: role.PromptFile,
        }
    }
```

Y definir el mensaje:

```go
// OpenEditorMsg solicita abrir el editor para un rol
type OpenEditorMsg struct {
    RoleName   string
    PromptFile string
}
```

### Paso 5: Actualizar el routing en app.go

```go
// En el Update de App:
case harness.OpenEditorMsg:
    // Cargar contenido del prompt
    content, err := a.store.ReadPrompt(a.currentProjectDir, msg.PromptFile)
    if err != nil {
        // TODO: mostrar error
        break
    }

    // Crear funciones de save y commit
    projectDir := a.currentProjectDir
    saveFn := func(data []byte) error {
        return a.store.SavePromptAtomic(projectDir, msg.PromptFile, data)
    }
    commitFn := func(message string) (string, error) {
        baseDir := filepath.Join(projectDir, ".lazyharness")
        return a.git.CommitAll(baseDir, message)
    }

    a.editor = editor.NewModel(
        msg.RoleName, msg.PromptFile, content,
        a.width, a.height,
        saveFn, commitFn,
    )
    a.currentScreen = screenEditor

case editor.BackToHarnessMsg:
    a.currentScreen = screenHarness
    if msg.Saved {
        // Recargar el prompt para que la vista read-only muestre lo nuevo
        // ... actualizar la vista de harness ...
    }
```

Y en View:

```go
case screenEditor:
    return a.editor.View()
```

### Paso 6: Tests del editor

Crea `internal/tui/editor/model_test.go`:

```go
package editor

import (
    "fmt"
    "testing"

    tea "github.com/charmbracelet/bubbletea"
)

// fakeSave simula guardar sin tocar disco
func fakeSave(shouldFail bool) SaveFunc {
    return func(content []byte) error {
        if shouldFail {
            return fmt.Errorf("disco lleno")
        }
        return nil
    }
}

// fakeCommit simula un commit git
func fakeCommit(shouldFail bool) CommitFunc {
    return func(message string) (string, error) {
        if shouldFail {
            return "", fmt.Errorf("error git")
        }
        return "abc1234567890abcdef1234567890abcdef123456", nil
    }
}

func TestEditor_DetectaModificacion(t *testing.T) {
    m := NewModel("arquitecto", "arquitecto.xml", "contenido original",
        80, 24, fakeSave(false), fakeCommit(false))

    // Al inicio no esta modificado
    if m.modified {
        t.Error("no deberia estar modificado al inicio")
    }

    // Simular escritura — insertar un caracter
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})

    if !m.modified {
        t.Error("deberia estar modificado despues de escribir")
    }
}

func TestEditor_CancelSinGuardar(t *testing.T) {
    m := NewModel("arquitecto", "arquitecto.xml", "contenido",
        80, 24, fakeSave(false), fakeCommit(false))

    // Presionar Esc
    _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})

    if cmd == nil {
        t.Fatal("deberia retornar un comando")
    }

    msg := cmd()
    backMsg, ok := msg.(BackToHarnessMsg)
    if !ok {
        t.Fatal("deberia retornar BackToHarnessMsg")
    }
    if backMsg.Saved {
        t.Error("no deberia marcar como guardado")
    }
}

func TestEditor_GuardarSinCambios(t *testing.T) {
    m := NewModel("arquitecto", "arquitecto.xml", "contenido",
        80, 24, fakeSave(false), fakeCommit(false))

    // Intentar guardar sin modificar
    m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

    if cmd != nil {
        t.Error("no deberia lanzar comando de guardado sin cambios")
    }
    if m.statusMsg != "Sin cambios para guardar" {
        t.Errorf("status esperado 'Sin cambios para guardar', obtuve '%s'", m.statusMsg)
    }
}

func TestEditor_GuardarExitoso(t *testing.T) {
    m := NewModel("arquitecto", "arquitecto.xml", "original",
        80, 24, fakeSave(false), fakeCommit(false))

    // Modificar
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})

    // Guardar
    m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

    if !m.saving {
        t.Error("deberia estar en estado saving")
    }
    if cmd == nil {
        t.Fatal("deberia retornar un comando de guardado")
    }
}

func TestEditor_ErrorAlGuardar(t *testing.T) {
    m := NewModel("arquitecto", "arquitecto.xml", "original",
        80, 24, fakeSave(true), fakeCommit(false))

    // Simular que llego el mensaje de error al guardar
    m, _ = m.Update(fileSavedMsg{err: fmt.Errorf("disco lleno")})

    if m.saving {
        t.Error("no deberia seguir en estado saving despues del error")
    }
    if m.statusMsg != "Error al guardar: disco lleno" {
        t.Errorf("mensaje de error inesperado: '%s'", m.statusMsg)
    }
}

func TestEditor_CommitExitoso(t *testing.T) {
    m := NewModel("arquitecto", "arquitecto.xml", "original",
        80, 24, fakeSave(false), fakeCommit(false))
    m.saving = true
    m.modified = true

    hash := "abc1234567890abcdef1234567890abcdef123456"
    m, cmd := m.Update(commitDoneMsg{hash: hash, err: nil})

    if m.saving {
        t.Error("no deberia seguir en estado saving")
    }
    if m.modified {
        t.Error("no deberia estar modificado despues de commit exitoso")
    }
    if cmd == nil {
        t.Fatal("deberia retornar comando BackToHarnessMsg")
    }
}
```

Correr los tests:

```bash
go test ./internal/tui/editor/ -v
```

## Tradeoffs y decisiones de diseno

### Decision 1: Textarea embebido vs $EDITOR

Elegimos textarea embebido. Es menos poderoso que vim/nano pero permite colorear @referencias en tiempo real (feature futuro), no necesita configuracion del usuario, y mantiene la experiencia dentro de la TUI.

**Si quisieramos lanzar $EDITOR externo**, seria asi:

```go
cmd := exec.Command(os.Getenv("EDITOR"), promptPath)
cmd.Stdin = os.Stdin
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
// Suspender bubbletea:
return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
    return editorFinishedMsg{err: err}
})
```

bubbletea tiene `tea.ExecProcess` exactamente para esto — suspende la TUI, ejecuta el proceso, y vuelve. lazygit lo usa para editar mensajes de commit. Si queremos agregar esto como alternativa en el futuro, el camino esta claro.

### Decision 2: Inyectar funciones vs pasar structs

El editor recibe `SaveFunc` y `CommitFunc` en vez de `*storage.Store` y `*storage.GitRepo`. Esto es dependency injection al estilo Go:

```go
// Lo que hicimos — funciones
type SaveFunc func(content []byte) error

// Alternativa — interface
type FileSaver interface {
    Save(content []byte) error
}
```

**Tradeoff funciones vs interface:** para una sola operacion, una funcion es mas liviana. Una interface conviene cuando tenes multiples metodos relacionados (como `VersionControl` con Init, CommitAll, Log, Diff). Para el editor que solo necesita "guardar" y "commitear", dos funciones son perfectas.

### Decision 3: tea.Sequence para guardar + commitear

El flujo de guardado es estrictamente secuencial:

```
1. Guardar archivo al disco     → fileSavedMsg
2. Crear commit git             → commitDoneMsg
3. Actualizar UI + volver
```

Si usaramos `tea.Batch`, el commit podria ejecutarse antes de que el archivo se guarde. Con `tea.Sequence`, el commit solo arranca cuando `fileSavedMsg` ya llego a Update.

**Fijate que tea.Sequence es relativamente nuevo en bubbletea.** Antes del 2024, la solucion era encadenar manualmente: en el handler de `fileSavedMsg`, lanzar el commit como un nuevo Cmd. Sequence simplifica esto.

### Decision 4: Escritura atomica

Usamos temp file + rename para prevenir corrupcion. El costo extra es despreciable (~0.1ms mas) y la proteccion es real: si el proceso muere durante la escritura, el archivo original sigue intacto.

## Errores comunes y tips

### Error: el textarea no recibe teclas

Si abris el editor y las teclas no hacen nada, probablemente el textarea no esta focuseado o el parent esta consumiendo las teclas primero.

**Solucion:** asegurate de llamar `ta.Focus()` al crear el textarea, y en `Update`, delegar los KeyMsg al textarea DESPUES de verificar tus propios keybindings (ctrl+s, esc):

```go
// BIEN: primero verificar mis keys, despues delegar
case tea.KeyMsg:
    switch {
    case key.Matches(msg, maintui.EditorKeyMap.Save):
        // manejar save
    case key.Matches(msg, maintui.EditorKeyMap.Cancel):
        // manejar cancel
    }

// Delegar TODO lo demas al textarea (incluyendo las teclas que no matchearon)
m.textarea, cmd = m.textarea.Update(msg)
```

```go
// MAL: delegar primero — ctrl+s nunca llega a mi handler
m.textarea, cmd = m.textarea.Update(msg)
// El textarea ya consumio el evento...
```

### Error: os.Rename falla con "invalid cross-device link"

Esto pasa cuando el archivo temporal esta en un filesystem distinto al destino. Siempre crea el temporal en el mismo directorio:

```go
// BIEN: mismo directorio = mismo filesystem
tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")

// MAL: /tmp puede ser otro filesystem
tmp, err := os.CreateTemp("", ".tmp-*")
```

### Error: olvidar recargar el contenido al abrir el editor

Si el archivo cambio en disco (por ejemplo, el usuario lo edito con otro editor), y vos abris el textarea con el contenido cacheado, vas a estar editando una version vieja.

**Solucion:** siempre leer el archivo fresco al abrir el editor. En `app.go`, cuando recibis `OpenEditorMsg`, llamas a `store.ReadPrompt()` en ese momento — nunca uses un cache viejo.

### Tip: el textarea tiene undo built-in

`bubbles/textarea` soporta Ctrl+Z para deshacer. No necesitas implementar nada — viene gratis. El undo funciona por caracter, no por palabra (es basico pero funcional).

### Tip: cursor en la primera linea

Cuando abrimos el editor con contenido existente, el cursor queda al final del texto. Si queres que arranque al principio:

```go
ta.SetValue(content)
ta.CursorStart() // mover al inicio
```

## Ejercicios

### 1. Basico: editor con save/cancel

Implementa el editor completo siguiendo los pasos de arriba. Verifica que:
- `e` abre el editor con el contenido del prompt seleccionado.
- Podes escribir texto.
- `ctrl+s` guarda y commitea.
- `esc` vuelve sin guardar.

### 2. Intermedio: indicador de modificacion

Agrega el indicador `[*] modificado` al header cuando el contenido difiere del original. Tiene que desaparecer despues de guardar. El codigo ya esta en el modelo de arriba — verifica que funcione visualmente.

### 3. Avanzado: dialogo de confirmacion al salir con cambios

Cuando el usuario presiona `esc` y hay cambios sin guardar, mostra un dialogo de confirmacion:

```
+-- Cambios sin guardar --+
|                          |
| Descartar cambios?       |
|                          |
|   [s] si    [n] no       |
+--------------------------+
```

Para esto necesitas un campo `showConfirmDialog bool` en el Model. Cuando `showConfirmDialog` es true, el Update intercepta todas las teclas y solo responde a `s` (descartar y salir) y `n` (volver al editor).

**Pista:** este es el patron de "modal overlay" que usamos en la [Clase 07](./07_historial_y_rollback.md) para el dialogo de restauracion.

### 4. Bonus: tests simulando input de texto

Escribe un test que:
1. Cree un editor con contenido "version 1".
2. Simule borrar todo y escribir "version 2".
3. Simule ctrl+s.
4. Verifique que la funcion de save recibio "version 2".

**Pista:** para simular borrado completo, manda `ctrl+a` (seleccionar todo) + `backspace`. Para escribir texto, manda `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("version 2")}`.

## Para profundizar

- [bubbles/textarea](https://github.com/charmbracelet/bubbles/tree/master/textarea): documentacion del componente, opciones de configuracion.
- [bubbletea commands](https://github.com/charmbracelet/bubbletea/tree/master/tutorials/commands): tutorial oficial sobre Cmds, Batch y Sequence.
- [tea.ExecProcess](https://pkg.go.dev/github.com/charmbracelet/bubbletea#ExecProcess): para lanzar un editor externo suspendiendo la TUI.
- [Atomic file writes in Go](https://github.com/natefinch/atomic): libreria que implementa escritura atomica (para produccion real, pero entender el patron manualmente es mas valioso).
- [Go by Example — Writing Files](https://gobyexample.com/writing-files): las distintas formas de escribir archivos en Go.

## Que sigue

En la [Clase 07](./07_historial_y_rollback.md) construimos la pantalla de historial y rollback. Al presionar `h` sobre un rol, ves su historial de versiones con lista de commits, diff con colores rojo/verde, y un dialogo para restaurar una version anterior. Vas a aprender a recorrer commits con go-git, generar diffs unificados, y construir dialogos modales en bubbletea.
