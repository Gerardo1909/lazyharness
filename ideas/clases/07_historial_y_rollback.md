# Clase 07: Historial y rollback — viajando en el tiempo

> Al terminar esta clase, al presionar `h` sobre un rol se abre la pantalla de historial con una lista de commits filtrada por archivo, un panel de diff con colores rojo/verde, y un dialogo para restaurar una version anterior. Restaurar crea un commit nuevo — nunca reescribimos la historia.

## Prerequisitos

- [Clase 05](./05_git_desde_go.md): operaciones git con go-git (CommitAll, Log, IsDirty).
- [Clase 06](./06_editor_embebido.md): editor embebido, escritura atomica, tea.Sequence.

## Conceptos de Go que vas a aprender

### 1. Recorrido de commits con go-git

En la clase 05 implementamos `Log` que retorna commits filtrados por archivo. Ahora vamos a profundizar en como funciona el commit walking de go-git:

```go
import (
    git "github.com/go-git/go-git/v5"
    "github.com/go-git/go-git/v5/plumbing/object"
)

// Obtener el iterador de commits
repo, _ := git.PlainOpen(dir)
logOpts := &git.LogOptions{
    Order: git.LogOrderCommitterTime,
}
iter, _ := repo.Log(logOpts)

// Recorrer commit por commit
err := iter.ForEach(func(c *object.Commit) error {
    fmt.Printf("%s %s (%s)\n",
        c.Hash.String()[:7],
        c.Message,
        c.Author.When.Format("2006-01-02 15:04"),
    )
    return nil // retornar nil para continuar, un error para parar
})
```

**Equivalente Python (GitPython):**

```python
from git import Repo

repo = Repo(dir)
for commit in repo.iter_commits():
    print(f"{commit.hexsha[:7]} {commit.message} ({commit.committed_datetime})")
```

**Equivalente Python (Dulwich — pure Python, como go-git):**

```python
from dulwich.repo import Repo
from dulwich.walk import Walker

repo = Repo(dir)
walker = Walker(repo.object_store, [repo.head()])
for entry in walker:
    commit = entry.commit
    print(commit.message.decode())
```

**Fijate algo importante:** `iter.ForEach` en go-git recibe una funcion. Si esa funcion retorna un error, el iterador se detiene. Esto es el patron de "early exit" que en Python harias con un `break`:

```go
// Go: parar despues de 50 commits
count := 0
iter.ForEach(func(c *object.Commit) error {
    count++
    if count > 50 {
        return fmt.Errorf("limite alcanzado") // para la iteracion
    }
    // procesar commit...
    return nil
})
```

```python
# Python: break simple
for i, commit in enumerate(repo.iter_commits()):
    if i > 50:
        break
```

**Tradeoff ForEach vs Next manual:** go-git tambien soporta `iter.Next()` para control manual. ForEach es mas limpio para el caso comun (procesar todos). Next es mejor si necesitas logica compleja entre iteraciones.

### 2. Generando diffs con go-git

Para mostrar que cambio entre dos versiones de un archivo, necesitamos el diff unificado. go-git puede generar patches entre commits:

```go
// Obtener dos commits por su hash
commitA, _ := repo.CommitObject(plumbing.NewHash(hashA))
commitB, _ := repo.CommitObject(plumbing.NewHash(hashB))

// Obtener los trees de cada commit
treeA, _ := commitA.Tree()
treeB, _ := commitB.Tree()

// Generar el patch (diff) entre ambos trees
patch, _ := treeA.Patch(treeB)

// El patch es un string con formato unified diff
diffStr := patch.String()
```

El resultado tiene el formato clasico de unified diff:

```diff
diff --git a/prompts/arquitecto.xml b/prompts/arquitecto.xml
--- a/prompts/arquitecto.xml
+++ b/prompts/arquitecto.xml
@@ -1,5 +1,6 @@
 <role>
-  Sos el arquitecto del proyecto.
+  Sos el arquitecto principal del proyecto.
+  Coordinas con todos los equipos.
   Tu responsabilidad es disenar
   la estructura del codigo.
 </role>
```

**Filtrar el diff por archivo:** `patch.String()` incluye todos los archivos que cambiaron. Para un diff de un solo archivo, filtramos:

```go
// Filtrar los file patches del diff
for _, fp := range patch.FilePatches() {
    from, to := fp.Files()
    name := ""
    if to != nil {
        name = to.Path()
    } else if from != nil {
        name = from.Path()
    }

    if name == targetFile {
        // Renderizar solo este file patch
        var buf strings.Builder
        for _, chunk := range fp.Chunks() {
            switch chunk.Type() {
            case 0: // Equal
                buf.WriteString(chunk.Content())
            case 1: // Add
                buf.WriteString(chunk.Content())
            case 2: // Delete
                buf.WriteString(chunk.Content())
            }
        }
    }
}
```

**Equivalente Python:**

```python
# GitPython
diff = repo.commit(hash_a).diff(repo.commit(hash_b))
for d in diff:
    if d.a_path == "prompts/arquitecto.xml":
        print(d.diff.decode())
```

### 3. Renderizado de diffs con colores ANSI

El diff tiene tres tipos de lineas: contexto (sin cambio), agregadas (+), y eliminadas (-). Las coloreamos con lipgloss:

```go
import "github.com/charmbracelet/lipgloss"

var (
    addedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a"))  // verde
    removedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e"))  // rojo
    contextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))  // gris
    headerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")).Bold(true) // azul
)

func renderDiff(diffText string) string {
    var b strings.Builder
    lines := strings.Split(diffText, "\n")

    for _, line := range lines {
        switch {
        case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
            b.WriteString(headerStyle.Render(line))
        case strings.HasPrefix(line, "@@"):
            b.WriteString(headerStyle.Render(line))
        case strings.HasPrefix(line, "+"):
            b.WriteString(addedStyle.Render(line))
        case strings.HasPrefix(line, "-"):
            b.WriteString(removedStyle.Render(line))
        default:
            b.WriteString(contextStyle.Render(line))
        }
        b.WriteString("\n")
    }

    return b.String()
}
```

**Equivalente Python (rich):**

```python
from rich.syntax import Syntax
from rich.console import Console

console = Console()
# Rich detecta el formato diff automaticamente
syntax = Syntax(diff_text, "diff", theme="monokai")
console.print(syntax)
```

**Tradeoff parser manual vs libreria de diffs:**

| Enfoque | Ventaja | Desventaja |
|---------|---------|------------|
| Parsear las lineas manualmente (lo que hacemos) | Cero dependencias, control total del renderizado | No maneja edge cases (binary, rename) |
| Usar una libreria como `sergi/go-diff` | Genera diffs desde texto plano, no necesita git | Dependencia extra, mas pesado |
| Usar el patch de go-git directamente | Integrado con git, correcto por definicion | API un poco verbosa |

Para lazyharness, parsear el output de `patch.String()` linea por linea es suficiente. Los prompts son texto plano — no hay binarios ni renames complicados.

### 4. Dialogos modales como overlay en bubbletea

Un dialogo de confirmacion es un "overlay" que intercepta todo el input hasta que el usuario responde:

```go
type Model struct {
    // ... campos existentes ...
    overlay     overlayType
    overlayData interface{}
}

type overlayType int

const (
    overlayNone overlayType = iota
    overlayRestore          // dialogo de confirmacion para restaurar
)

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    // Si hay overlay activo, TODO el input va al overlay
    if m.overlay != overlayNone {
        return m.updateOverlay(msg)
    }

    // Input normal...
}

func (m Model) updateOverlay(msg tea.Msg) (Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "s", "y": // si/yes
            m.overlay = overlayNone
            return m, m.executeRestore()
        case "n", "esc": // no/cancelar
            m.overlay = overlayNone
            return m, nil
        }
    }
    return m, nil
}
```

**Equivalente Python (textual):**

```python
class ConfirmDialog(Screen):
    def compose(self):
        yield Static("Restaurar a esta version?")
        yield Button("Si", id="yes")
        yield Button("No", id="no")

    def on_button_pressed(self, event):
        if event.button.id == "yes":
            self.dismiss(True)
        else:
            self.dismiss(False)

# Abrir como modal:
confirmed = await self.app.push_screen(ConfirmDialog())
```

**En bubbletea no hay stack de screens nativo.** El patron es tener un campo `overlay` en el Model que, cuando no es `overlayNone`, intercepta todo el input. Es mas manual que textual pero da control total.

**Tradeoff overlay vs pantalla separada:** un overlay mantiene el contexto visual — el usuario ve el diff detras del dialogo. Una pantalla separada pierde ese contexto. Para "restaurar si/no", el overlay es mejor.

### 5. Checkout de un archivo en un commit especifico

Para restaurar un archivo a una version anterior sin afectar los demas:

```go
func (g GitRepo) RestoreFile(dir, hash, filename string) error {
    repo, err := git.PlainOpen(dir)
    if err != nil {
        return fmt.Errorf("abriendo repo: %w", err)
    }

    // 1. Obtener el commit historico
    commitObj, err := repo.CommitObject(plumbing.NewHash(hash))
    if err != nil {
        return fmt.Errorf("obteniendo commit %s: %w", hash[:7], err)
    }

    // 2. Obtener el tree del commit
    tree, err := commitObj.Tree()
    if err != nil {
        return fmt.Errorf("obteniendo tree: %w", err)
    }

    // 3. Buscar el archivo en el tree
    file, err := tree.File(filename)
    if err != nil {
        return fmt.Errorf("archivo %s no encontrado en commit %s: %w", filename, hash[:7], err)
    }

    // 4. Leer el contenido
    content, err := file.Contents()
    if err != nil {
        return fmt.Errorf("leyendo contenido: %w", err)
    }

    // 5. Escribir al worktree actual
    fullPath := filepath.Join(dir, filename)
    if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
        return fmt.Errorf("escribiendo archivo: %w", err)
    }

    return nil
}
```

**Equivalente Python:**

```python
# GitPython
commit = repo.commit(hash)
blob = commit.tree / "prompts/arquitecto.xml"
Path("prompts/arquitecto.xml").write_bytes(blob.data_stream.read())
```

**Fijate que NO usamos `git checkout` ni `git restore`.** Simplemente leemos el contenido del archivo en un commit viejo y lo escribimos al worktree. Despues hacemos un commit nuevo con ese contenido. Esto es mas simple que las operaciones de checkout de git y tiene el efecto exacto que queremos: "volver el contenido de este archivo a como era en esa version".

**Tradeoff restaurar + commit nuevo vs git revert:**

| Enfoque | Que hace | Cuando usarlo |
|---------|----------|---------------|
| Leer archivo viejo + commit nuevo (lo nuestro) | Restaura UN archivo, commit con mensaje claro | Rollback granular por archivo |
| `git revert` | Invierte un commit ENTERO | Cuando queres deshacer todos los cambios de un commit |
| `git checkout <hash> -- <file>` | Restaura un archivo en el index | Similar al nuestro pero mas gitero |

Para lazyharness, el patron de "leer + escribir + commit nuevo" es el mas simple y cumple con el requerimiento B2: rollback por archivo sin afectar a los demas.

## Lo que vamos a construir

La pantalla de historial del mockup 05:

```
+-- dev-flow -- historial: arquitecto.xml --------------------------+
|                                                                    |
| +-- Commits --------+ +-- Diff (abc1234 vs actual) ------------+ |
| |                    | |                                         | |
| | > abc1234 (hoy)   | | --- a/prompts/arquitecto.xml            | |
| |   actualizo prompt | | +++ b/prompts/arquitecto.xml            | |
| |                    | | @@ -1,5 +1,6 @@                        | |
| |   def5678 (ayer)   | |  <role>                                 | |
| |   agrego constraints| | -  Sos el arquitecto del proyecto.     | |
| |                    | | +  Sos el arquitecto principal.         | |
| |   9ab0123 (3 dias) | | +  Coordinas con todos los equipos.     | |
| |   version inicial  | |  </role>                                | |
| |                    | |                                         | |
| +--------------------+ +-----------------------------------------+ |
|                                                                    |
+--------------------------------------------------------------------+
|  enter ver diff  r restaurar  esc volver                           |
+--------------------------------------------------------------------+
```

Cuando presionas `r` sobre un commit, aparece el overlay de confirmacion:

```
+-- Restaurar version? ----+
|                           |
| Volver arquitecto.xml     |
| a la version def5678?     |
|                           |
| Esto crea un commit       |
| nuevo con el contenido    |
| de esa version.           |
|                           |
|   [s] si    [n] no        |
+---------------------------+
```

**Archivos a crear:**
- `internal/tui/history/model.go`
- `internal/tui/history/model_test.go`
- `internal/tui/components/diffview.go`
- `internal/tui/components/dialog.go`

**Archivos a modificar:**
- `internal/storage/git.go` — agregar `Diff` y `RestoreFile`
- `internal/storage/git_test.go` — tests de diff y restore
- `internal/tui/harness/model.go` — abrir historial al presionar `h`
- `internal/tui/app.go` — agregar routing a screenHistory

## Implementacion paso a paso

### Paso 1: Implementar Diff y RestoreFile en git.go

Agrega a `internal/storage/git.go`:

```go
// Diff retorna el diff unificado entre dos commits para un archivo especifico.
// Si hashA esta vacio, compara el primer commit contra un tree vacio (muestra todo como agregado).
func (g GitRepo) Diff(dir, hashA, hashB, filename string) (string, error) {
    repo, err := git.PlainOpen(dir)
    if err != nil {
        return "", fmt.Errorf("abriendo repo: %w", err)
    }

    commitB, err := repo.CommitObject(plumbing.NewHash(hashB))
    if err != nil {
        return "", fmt.Errorf("obteniendo commit %s: %w", hashB[:7], err)
    }
    treeB, err := commitB.Tree()
    if err != nil {
        return "", fmt.Errorf("obteniendo tree B: %w", err)
    }

    var treeA *object.Tree
    if hashA != "" {
        commitA, err := repo.CommitObject(plumbing.NewHash(hashA))
        if err != nil {
            return "", fmt.Errorf("obteniendo commit %s: %w", hashA[:7], err)
        }
        treeA, err = commitA.Tree()
        if err != nil {
            return "", fmt.Errorf("obteniendo tree A: %w", err)
        }
    }

    // Generar el patch
    patch, err := treeA.Patch(treeB)
    if err != nil {
        return "", fmt.Errorf("generando diff: %w", err)
    }

    // Filtrar por archivo si se especifica
    if filename == "" {
        return patch.String(), nil
    }

    var result strings.Builder
    for _, fp := range patch.FilePatches() {
        from, to := fp.Files()
        name := ""
        if to != nil {
            name = to.Path()
        } else if from != nil {
            name = from.Path()
        }

        if name == filename || strings.HasSuffix(name, filename) {
            // Reconstruir el diff para este archivo
            result.WriteString(fmt.Sprintf("--- a/%s\n", name))
            result.WriteString(fmt.Sprintf("+++ b/%s\n", name))
            for _, chunk := range fp.Chunks() {
                content := chunk.Content()
                lines := strings.Split(content, "\n")
                for _, line := range lines {
                    if line == "" {
                        continue
                    }
                    switch chunk.Type() {
                    case 0: // Equal
                        result.WriteString(" " + line + "\n")
                    case 1: // Add
                        result.WriteString("+" + line + "\n")
                    case 2: // Delete
                        result.WriteString("-" + line + "\n")
                    }
                }
            }
        }
    }

    return result.String(), nil
}

// RestoreFile restaura un archivo a la version de un commit especifico.
// No crea commit — eso lo hace el llamador despues.
func (g GitRepo) RestoreFile(dir, hash, filename string) error {
    repo, err := git.PlainOpen(dir)
    if err != nil {
        return fmt.Errorf("abriendo repo: %w", err)
    }

    commitObj, err := repo.CommitObject(plumbing.NewHash(hash))
    if err != nil {
        return fmt.Errorf("obteniendo commit %s: %w", hash[:7], err)
    }

    tree, err := commitObj.Tree()
    if err != nil {
        return fmt.Errorf("obteniendo tree: %w", err)
    }

    file, err := tree.File(filename)
    if err != nil {
        return fmt.Errorf("archivo %s no encontrado en %s: %w", filename, hash[:7], err)
    }

    content, err := file.Contents()
    if err != nil {
        return fmt.Errorf("leyendo contenido: %w", err)
    }

    fullPath := filepath.Join(dir, filename)
    if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
        return fmt.Errorf("creando directorio: %w", err)
    }

    if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
        return fmt.Errorf("escribiendo archivo: %w", err)
    }

    return nil
}
```

### Paso 2: Tests de Diff y RestoreFile

Agrega a `internal/storage/git_test.go`:

```go
func TestGitDiff(t *testing.T) {
    dir := t.TempDir()
    g := NewGitRepo()
    g.Init(dir)

    // Version 1
    os.WriteFile(filepath.Join(dir, "test.txt"), []byte("linea 1\nlinea 2\n"), 0644)
    hashA, _ := g.CommitAll(dir, "version 1")

    // Version 2
    os.WriteFile(filepath.Join(dir, "test.txt"), []byte("linea 1\nlinea modificada\nlinea 3\n"), 0644)
    hashB, _ := g.CommitAll(dir, "version 2")

    diff, err := g.Diff(dir, hashA, hashB, "test.txt")
    if err != nil {
        t.Fatalf("error generando diff: %v", err)
    }

    if diff == "" {
        t.Error("el diff no deberia estar vacio")
    }

    // El diff deberia contener la linea eliminada y la agregada
    if !strings.Contains(diff, "-") || !strings.Contains(diff, "+") {
        t.Error("el diff deberia tener lineas agregadas y eliminadas")
    }
}

func TestGitRestoreFile(t *testing.T) {
    dir := t.TempDir()
    g := NewGitRepo()
    g.Init(dir)

    // Version 1
    os.WriteFile(filepath.Join(dir, "test.txt"), []byte("version original"), 0644)
    hashV1, _ := g.CommitAll(dir, "version 1")

    // Version 2
    os.WriteFile(filepath.Join(dir, "test.txt"), []byte("version modificada"), 0644)
    g.CommitAll(dir, "version 2")

    // Version 3
    os.WriteFile(filepath.Join(dir, "test.txt"), []byte("version mas nueva"), 0644)
    g.CommitAll(dir, "version 3")

    // Restaurar a version 1
    err := g.RestoreFile(dir, hashV1, "test.txt")
    if err != nil {
        t.Fatalf("error restaurando: %v", err)
    }

    // Verificar contenido
    data, _ := os.ReadFile(filepath.Join(dir, "test.txt"))
    if string(data) != "version original" {
        t.Errorf("contenido esperado 'version original', obtuve '%s'", string(data))
    }

    // Commitear la restauracion
    hash, err := g.CommitAll(dir, "restaurar test.txt a version 1")
    if err != nil {
        t.Fatalf("error commiteando restauracion: %v", err)
    }
    if len(hash) != 40 {
        t.Errorf("hash invalido: %s", hash)
    }

    // Verificar que hay 4 commits (3 originales + 1 de restauracion)
    commits, _ := g.Log(dir, "")
    if len(commits) != 4 {
        t.Errorf("esperaba 4 commits, obtuve %d", len(commits))
    }
}

func TestGitRestoreNoReescribeHistoria(t *testing.T) {
    dir := t.TempDir()
    g := NewGitRepo()
    g.Init(dir)

    // Crear 3 versiones
    for i := 1; i <= 3; i++ {
        content := fmt.Sprintf("version %d", i)
        os.WriteFile(filepath.Join(dir, "test.txt"), []byte(content), 0644)
        g.CommitAll(dir, fmt.Sprintf("commit %d", i))
    }

    // Contar commits antes
    commitsBefore, _ := g.Log(dir, "")

    // Restaurar a v1
    commits, _ := g.Log(dir, "")
    hashV1 := commits[len(commits)-1].Hash // el mas viejo
    g.RestoreFile(dir, hashV1, "test.txt")
    g.CommitAll(dir, "restaurar a v1")

    // Verificar que tenemos MAS commits (no menos)
    commitsAfter, _ := g.Log(dir, "")
    if len(commitsAfter) != len(commitsBefore)+1 {
        t.Errorf("restaurar deberia agregar un commit, no quitar: antes=%d, despues=%d",
            len(commitsBefore), len(commitsAfter))
    }
}
```

```bash
go test ./internal/storage/ -v -run "TestGitDiff|TestGitRestore"
```

### Paso 3: Crear el componente DiffView

Crea `internal/tui/components/diffview.go`:

```go
package components

import (
    "strings"

    "github.com/charmbracelet/bubbles/viewport"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

var (
    diffAddedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a"))
    diffRemovedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e"))
    diffContextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
    diffHeaderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")).Bold(true)
)

// DiffView renderiza un diff unificado con colores
type DiffView struct {
    viewport viewport.Model
    title    string
    width    int
    height   int
    empty    bool
}

func NewDiffView(title, diffText string, width, height int) DiffView {
    vp := viewport.New(width-2, height-3)

    empty := strings.TrimSpace(diffText) == ""
    if empty {
        vp.SetContent(diffContextStyle.Render("(sin diferencias)"))
    } else {
        vp.SetContent(colorizeDiff(diffText))
    }

    return DiffView{
        viewport: vp,
        title:    title,
        width:    width,
        height:   height,
        empty:    empty,
    }
}

func (d *DiffView) Update(msg tea.Msg) (*DiffView, tea.Cmd) {
    var cmd tea.Cmd
    d.viewport, cmd = d.viewport.Update(msg)
    return d, cmd
}

func (d DiffView) View() string {
    header := diffHeaderStyle.Render(d.title)
    return lipgloss.JoinVertical(lipgloss.Left,
        header,
        d.viewport.View(),
    )
}

// colorizeDiff aplica colores a cada linea del diff
func colorizeDiff(diffText string) string {
    var b strings.Builder
    lines := strings.Split(diffText, "\n")

    for _, line := range lines {
        switch {
        case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
            b.WriteString(diffHeaderStyle.Render(line))
        case strings.HasPrefix(line, "@@"):
            b.WriteString(diffHeaderStyle.Render(line))
        case strings.HasPrefix(line, "+"):
            b.WriteString(diffAddedStyle.Render(line))
        case strings.HasPrefix(line, "-"):
            b.WriteString(diffRemovedStyle.Render(line))
        default:
            b.WriteString(diffContextStyle.Render(line))
        }
        b.WriteString("\n")
    }

    return b.String()
}
```

### Paso 4: Crear el componente Dialog

Crea `internal/tui/components/dialog.go`:

```go
package components

import (
    "github.com/charmbracelet/lipgloss"
    "github.com/Gerardo1909/lazyharness/internal/tui"
)

// Dialog renderiza un dialogo de confirmacion centrado
type Dialog struct {
    Title   string
    Message string
    Width   int
}

func (d Dialog) View() string {
    titleStyle := lipgloss.NewStyle().
        Bold(true).
        Foreground(tui.ColorFg).
        Align(lipgloss.Center).
        Width(d.Width - 4)

    messageStyle := lipgloss.NewStyle().
        Foreground(tui.ColorComment).
        Align(lipgloss.Center).
        Width(d.Width - 4)

    keyStyle := lipgloss.NewStyle().
        Bold(true).
        Foreground(tui.ColorGreen).
        Padding(0, 1)

    descStyle := lipgloss.NewStyle().
        Foreground(tui.ColorComment)

    keys := keyStyle.Render("[s]") + " " + descStyle.Render("si") + "    " +
        keyStyle.Render("[n]") + " " + descStyle.Render("no")

    keysLine := lipgloss.NewStyle().
        Align(lipgloss.Center).
        Width(d.Width - 4).
        Render(keys)

    content := lipgloss.JoinVertical(lipgloss.Center,
        "",
        titleStyle.Render(d.Title),
        "",
        messageStyle.Render(d.Message),
        "",
        keysLine,
        "",
    )

    border := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(tui.ColorBlue).
        Padding(0, 1).
        Width(d.Width)

    return border.Render(content)
}
```

### Paso 5: Crear el modelo de historial

Crea `internal/tui/history/model.go`:

```go
package history

import (
    "fmt"
    "strings"
    "time"

    "github.com/charmbracelet/bubbles/key"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    maintui "github.com/Gerardo1909/lazyharness/internal/tui"
    "github.com/Gerardo1909/lazyharness/internal/tui/components"
    "github.com/Gerardo1909/lazyharness/internal/storage"
)

// --- Mensajes ---

// BackToHarnessMsg indica que el usuario salio del historial
type BackToHarnessMsg struct {
    Restored bool
}

// commitsLoadedMsg llega cuando se terminaron de cargar los commits
type commitsLoadedMsg struct {
    commits []storage.CommitInfo
    err     error
}

// diffLoadedMsg llega cuando se termino de generar el diff
type diffLoadedMsg struct {
    diff string
    err  error
}

// restoreDoneMsg llega cuando se termino la restauracion
type restoreDoneMsg struct {
    hash string
    err  error
}

// --- Funciones inyectadas ---

type LogFunc func(filename string) ([]storage.CommitInfo, error)
type DiffFunc func(hashA, hashB, filename string) (string, error)
type RestoreFunc func(hash, filename string) error
type CommitFunc func(message string) (string, error)

// --- Overlay ---

type overlayType int

const (
    overlayNone overlayType = iota
    overlayRestore
)

// --- Modelo ---

type Model struct {
    roleName    string
    promptFile  string
    commits     []storage.CommitInfo
    selected    int
    diffView    *components.DiffView
    overlay     overlayType
    statusMsg   string
    loading     bool
    width       int
    height      int

    logFn     LogFunc
    diffFn    DiffFunc
    restoreFn RestoreFunc
    commitFn  CommitFunc
}

func NewModel(
    roleName, promptFile string,
    width, height int,
    logFn LogFunc,
    diffFn DiffFunc,
    restoreFn RestoreFunc,
    commitFn CommitFunc,
) Model {
    return Model{
        roleName:   roleName,
        promptFile: promptFile,
        selected:   0,
        loading:    true,
        width:      width,
        height:     height,
        logFn:      logFn,
        diffFn:     diffFn,
        restoreFn:  restoreFn,
        commitFn:   commitFn,
    }
}

func (m Model) Init() tea.Cmd {
    // Cargar los commits al iniciar
    return func() tea.Msg {
        commits, err := m.logFn(m.promptFile)
        return commitsLoadedMsg{commits: commits, err: err}
    }
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    // Si hay overlay, todo el input va al overlay
    if m.overlay != overlayNone {
        return m.updateOverlay(msg)
    }

    switch msg := msg.(type) {
    case commitsLoadedMsg:
        m.loading = false
        if msg.err != nil {
            m.statusMsg = "Error cargando historial: " + msg.err.Error()
            return m, nil
        }
        m.commits = msg.commits
        if len(m.commits) > 0 {
            return m, m.loadDiffCmd()
        }

    case diffLoadedMsg:
        if msg.err != nil {
            m.statusMsg = "Error generando diff: " + msg.err.Error()
            return m, nil
        }
        title := fmt.Sprintf("Diff: %s", m.commits[m.selected].Hash[:7])
        dw := m.diffPanelWidth()
        dh := m.diffPanelHeight()
        dv := components.NewDiffView(title, msg.diff, dw, dh)
        m.diffView = &dv

    case restoreDoneMsg:
        if msg.err != nil {
            m.statusMsg = "Error restaurando: " + msg.err.Error()
            return m, nil
        }
        m.statusMsg = "Restaurado con commit " + msg.hash[:7]
        return m, func() tea.Msg {
            return BackToHarnessMsg{Restored: true}
        }

    case tea.KeyMsg:
        switch {
        case key.Matches(msg, maintui.HarnessKeyMap.Back):
            return m, func() tea.Msg {
                return BackToHarnessMsg{Restored: false}
            }

        case key.Matches(msg, maintui.HarnessKeyMap.Up):
            if m.selected > 0 {
                m.selected--
                return m, m.loadDiffCmd()
            }

        case key.Matches(msg, maintui.HarnessKeyMap.Down):
            if m.selected < len(m.commits)-1 {
                m.selected++
                return m, m.loadDiffCmd()
            }

        case msg.String() == "r":
            if len(m.commits) > 0 && m.selected > 0 {
                // No permitir restaurar al commit mas reciente (ya es la version actual)
                m.overlay = overlayRestore
            }
        }
    }

    // Delegar al diff view si existe
    if m.diffView != nil {
        var cmd tea.Cmd
        m.diffView, cmd = m.diffView.Update(msg)
        return m, cmd
    }

    return m, nil
}

func (m Model) updateOverlay(msg tea.Msg) (Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "s", "y":
            m.overlay = overlayNone
            commit := m.commits[m.selected]
            return m, m.restoreCmd(commit.Hash)
        case "n", "esc":
            m.overlay = overlayNone
        }
    }
    return m, nil
}

// loadDiffCmd carga el diff del commit seleccionado contra el anterior
func (m Model) loadDiffCmd() tea.Cmd {
    if len(m.commits) == 0 {
        return nil
    }

    selected := m.commits[m.selected]
    // Diff contra el commit anterior (si existe)
    var prevHash string
    if m.selected < len(m.commits)-1 {
        prevHash = m.commits[m.selected+1].Hash
    }

    return func() tea.Msg {
        diff, err := m.diffFn(prevHash, selected.Hash, m.promptFile)
        return diffLoadedMsg{diff: diff, err: err}
    }
}

// restoreCmd ejecuta la restauracion secuencial: restore file + commit
func (m Model) restoreCmd(hash string) tea.Cmd {
    promptFile := m.promptFile
    roleName := m.roleName
    return func() tea.Msg {
        // 1. Restaurar el archivo
        if err := m.restoreFn(hash, promptFile); err != nil {
            return restoreDoneMsg{err: err}
        }
        // 2. Crear commit
        msg := fmt.Sprintf("restaurar %s a version %s", roleName, hash[:7])
        commitHash, err := m.commitFn(msg)
        return restoreDoneMsg{hash: commitHash, err: err}
    }
}

// --- Helpers de layout ---

func (m Model) commitListWidth() int {
    w := m.width * 30 / 100
    if w < 25 {
        w = 25
    }
    return w
}

func (m Model) diffPanelWidth() int {
    return m.width - m.commitListWidth() - 2
}

func (m Model) diffPanelHeight() int {
    return m.height - 6
}

// --- View ---

func (m Model) View() string {
    // Header
    headerStyle := lipgloss.NewStyle().Bold(true).Foreground(maintui.ColorFg)
    header := headerStyle.Render(
        fmt.Sprintf("Historial: %s (%s)", m.roleName, m.promptFile),
    )

    if m.loading {
        return lipgloss.JoinVertical(lipgloss.Left,
            header,
            "",
            maintui.StyleComment().Render("Cargando historial..."),
        )
    }

    if len(m.commits) == 0 {
        return lipgloss.JoinVertical(lipgloss.Left,
            header,
            "",
            maintui.StyleComment().Render("No hay historial para este archivo."),
        )
    }

    // Commit list
    commitList := m.renderCommitList()

    // Diff panel
    diffPanel := ""
    if m.diffView != nil {
        diffPanel = m.diffView.View()
    } else {
        diffPanel = maintui.StyleComment().Render("Selecciona un commit para ver el diff")
    }

    content := lipgloss.JoinHorizontal(lipgloss.Top, commitList, " ", diffPanel)

    // Status
    status := maintui.StyleComment().Italic(true).Render(m.statusMsg)

    // Keybar
    keyStyle := lipgloss.NewStyle().
        Bold(true).
        Foreground(maintui.ColorFg).
        Background(maintui.ColorSelection).
        Padding(0, 1)
    descStyle := lipgloss.NewStyle().
        Foreground(maintui.ColorComment).
        MarginRight(2)

    keybar := keyStyle.Render("j/k") + " " + descStyle.Render("navegar") +
        keyStyle.Render("r") + " " + descStyle.Render("restaurar") +
        keyStyle.Render("esc") + " " + descStyle.Render("volver")

    result := lipgloss.JoinVertical(lipgloss.Left,
        header,
        "",
        content,
        "",
        status,
        keybar,
    )

    // Overlay de restauracion
    if m.overlay == overlayRestore {
        commit := m.commits[m.selected]
        dialog := components.Dialog{
            Title:   "Restaurar version?",
            Message: fmt.Sprintf(
                "Volver %s a la version %s?\n\nEsto crea un commit nuevo con el contenido de esa version.",
                m.promptFile, commit.Hash[:7],
            ),
            Width: 40,
        }
        // Centrar el dialogo sobre el contenido
        dialogView := lipgloss.Place(
            m.width, m.height,
            lipgloss.Center, lipgloss.Center,
            dialog.View(),
        )
        return dialogView
    }

    return result
}

func (m Model) renderCommitList() string {
    w := m.commitListWidth()
    var b strings.Builder

    titleStyle := lipgloss.NewStyle().Bold(true).Foreground(maintui.ColorFg)
    b.WriteString(titleStyle.Render("Commits") + "\n\n")

    hashStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7")) // violeta
    msgStyle := lipgloss.NewStyle().Foreground(maintui.ColorFg)
    timeStyle := lipgloss.NewStyle().Foreground(maintui.ColorComment)
    selectedBg := lipgloss.NewStyle().Background(maintui.ColorSelection)

    for i, c := range m.commits {
        prefix := "  "
        if i == m.selected {
            prefix = "> "
        }

        hash := hashStyle.Render(c.Hash[:7])
        msg := msgStyle.Render(truncate(c.Message, w-15))
        relTime := timeStyle.Render(relativeTime(c.When))

        line := fmt.Sprintf("%s%s %s\n   %s", prefix, hash, relTime, msg)

        if i == m.selected {
            line = selectedBg.Render(line)
        }

        b.WriteString(line + "\n")
    }

    return lipgloss.NewStyle().Width(w).Render(b.String())
}

// relativeTime formatea un time.Time como "hace 2 horas", "ayer", etc.
func relativeTime(t time.Time) string {
    diff := time.Since(t)

    switch {
    case diff < time.Minute:
        return "ahora"
    case diff < time.Hour:
        mins := int(diff.Minutes())
        if mins == 1 {
            return "hace 1 min"
        }
        return fmt.Sprintf("hace %d min", mins)
    case diff < 24*time.Hour:
        hours := int(diff.Hours())
        if hours == 1 {
            return "hace 1 hora"
        }
        return fmt.Sprintf("hace %d horas", hours)
    case diff < 48*time.Hour:
        return "ayer"
    case diff < 7*24*time.Hour:
        days := int(diff.Hours() / 24)
        return fmt.Sprintf("hace %d dias", days)
    default:
        return t.Format("2006-01-02")
    }
}

// truncate corta un string a maxLen caracteres, agregando "..." si es necesario
func truncate(s string, maxLen int) string {
    if maxLen <= 3 {
        return s
    }
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen-3] + "..."
}
```

### Paso 6: Tests del historial

Crea `internal/tui/history/model_test.go`:

```go
package history

import (
    "testing"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/Gerardo1909/lazyharness/internal/storage"
)

func fakeCommits() []storage.CommitInfo {
    now := time.Now()
    return []storage.CommitInfo{
        {Hash: "abc1234567890abcdef1234567890abcdef123456", Message: "actualizo prompt", When: now},
        {Hash: "def5678901234567890abcdef1234567890abcdef", Message: "agrego constraints", When: now.Add(-24 * time.Hour)},
        {Hash: "9ab0123456789012345678901234567890abcdef0", Message: "version inicial", When: now.Add(-72 * time.Hour)},
    }
}

func fakeLogFn(commits []storage.CommitInfo) LogFunc {
    return func(filename string) ([]storage.CommitInfo, error) {
        return commits, nil
    }
}

func fakeDiffFn() DiffFunc {
    return func(hashA, hashB, filename string) (string, error) {
        return "-linea vieja\n+linea nueva\n", nil
    }
}

func fakeRestoreFn(shouldFail bool) RestoreFunc {
    return func(hash, filename string) error {
        if shouldFail {
            return fmt.Errorf("error restaurando")
        }
        return nil
    }
}

func fakeCommitFn() CommitFunc {
    return func(message string) (string, error) {
        return "new1234567890abcdef1234567890abcdef123456", nil
    }
}

func TestHistory_CargaCommits(t *testing.T) {
    commits := fakeCommits()
    m := NewModel("arquitecto", "prompts/arquitecto.xml",
        80, 24,
        fakeLogFn(commits), fakeDiffFn(), fakeRestoreFn(false), fakeCommitFn())

    // Simular que llega el mensaje de commits cargados
    m, _ = m.Update(commitsLoadedMsg{commits: commits, err: nil})

    if len(m.commits) != 3 {
        t.Errorf("esperaba 3 commits, obtuve %d", len(m.commits))
    }
    if m.loading {
        t.Error("no deberia estar loading despues de cargar")
    }
}

func TestHistory_NavegacionCommits(t *testing.T) {
    commits := fakeCommits()
    m := NewModel("arquitecto", "prompts/arquitecto.xml",
        80, 24,
        fakeLogFn(commits), fakeDiffFn(), fakeRestoreFn(false), fakeCommitFn())
    m.commits = commits
    m.loading = false

    // Bajar
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
    if m.selected != 1 {
        t.Errorf("despues de j, selected deberia ser 1, es %d", m.selected)
    }

    // Bajar otra vez
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
    if m.selected != 2 {
        t.Errorf("despues de j, selected deberia ser 2, es %d", m.selected)
    }

    // No pasar del final
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
    if m.selected != 2 {
        t.Error("no deberia pasar del ultimo commit")
    }

    // Subir
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
    if m.selected != 1 {
        t.Errorf("despues de k, selected deberia ser 1, es %d", m.selected)
    }
}

func TestHistory_RestaurarAbreDialogo(t *testing.T) {
    commits := fakeCommits()
    m := NewModel("arquitecto", "prompts/arquitecto.xml",
        80, 24,
        fakeLogFn(commits), fakeDiffFn(), fakeRestoreFn(false), fakeCommitFn())
    m.commits = commits
    m.loading = false
    m.selected = 1 // no el mas reciente

    // Presionar r
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
    if m.overlay != overlayRestore {
        t.Error("deberia abrir el dialogo de restauracion")
    }
}

func TestHistory_RestaurarNoPermitidaEnActual(t *testing.T) {
    commits := fakeCommits()
    m := NewModel("arquitecto", "prompts/arquitecto.xml",
        80, 24,
        fakeLogFn(commits), fakeDiffFn(), fakeRestoreFn(false), fakeCommitFn())
    m.commits = commits
    m.loading = false
    m.selected = 0 // el commit mas reciente

    // Presionar r sobre el commit actual
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
    if m.overlay != overlayNone {
        t.Error("no deberia abrir dialogo para el commit mas reciente")
    }
}

func TestHistory_CancelarDialogo(t *testing.T) {
    m := NewModel("arquitecto", "prompts/arquitecto.xml",
        80, 24,
        fakeLogFn(fakeCommits()), fakeDiffFn(), fakeRestoreFn(false), fakeCommitFn())
    m.commits = fakeCommits()
    m.overlay = overlayRestore

    // Presionar n para cancelar
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
    if m.overlay != overlayNone {
        t.Error("deberia cerrar el dialogo al presionar n")
    }
}

func TestRelativeTime(t *testing.T) {
    tests := []struct {
        name     string
        when     time.Time
        expected string
    }{
        {"ahora", time.Now(), "ahora"},
        {"hace 5 min", time.Now().Add(-5 * time.Minute), "hace 5 min"},
        {"hace 1 hora", time.Now().Add(-1 * time.Hour), "hace 1 hora"},
        {"hace 3 horas", time.Now().Add(-3 * time.Hour), "hace 3 horas"},
        {"ayer", time.Now().Add(-30 * time.Hour), "ayer"},
        {"hace 5 dias", time.Now().Add(-5 * 24 * time.Hour), "hace 5 dias"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := relativeTime(tt.when)
            if got != tt.expected {
                t.Errorf("esperaba %q, obtuve %q", tt.expected, got)
            }
        })
    }
}

func TestTruncate(t *testing.T) {
    tests := []struct {
        input    string
        maxLen   int
        expected string
    }{
        {"corto", 20, "corto"},
        {"este mensaje es demasiado largo", 15, "este mensaje..."},
        {"exacto", 6, "exacto"},
    }

    for _, tt := range tests {
        got := truncate(tt.input, tt.maxLen)
        if got != tt.expected {
            t.Errorf("truncate(%q, %d) = %q, esperaba %q", tt.input, tt.maxLen, got, tt.expected)
        }
    }
}
```

```bash
go test ./internal/tui/history/ -v
```

## Tradeoffs y decisiones de diseno

### Decision 1: Rollback por archivo + commit nuevo

Elegimos leer el contenido viejo y crear un commit nuevo, en vez de `git revert` o reescribir historia.

**Ventaja:** el historial es monotonicamente creciente. Si restauras y despues te arrepentis, podes restaurar a la version de antes del rollback — todo esta en el log.

**Desventaja:** el historial se llena de commits de restauracion. Para un repo chico de prompts, esto no es problema.

**Fijate que esto es la misma filosofia que usa Wikipedia:** cada edicion crea una version nueva, un "revert" es simplemente editar con el contenido viejo.

### Decision 2: Diff parsing manual vs libreria

Parseamos el output de `patch.String()` linea por linea buscando prefijos `+`, `-`, `@@`. Es simple y funciona para texto plano.

**Alternativa `sergi/go-diff`:** esta libreria genera diffs desde dos strings sin necesitar git. Util si quisieramos comparar contenido que no esta en commits (por ejemplo, draft vs guardado). Por ahora, el diff de go-git es suficiente.

### Decision 3: Cargar toda la historia vs paginar

Cargamos todos los commits del archivo con `Log`. Para un repo de prompts con <100 commits, esto tarda <10ms.

**Cuando paginar:** si un archivo tuviera 1000+ commits, cargar todos seria lento. La solucion seria agregar un parametro `limit` a `Log` y cargar mas al hacer scroll down. Para el MVP, carga completa.

### Decision 4: Overlay vs pantalla separada para el dialogo

El dialogo de restauracion es un overlay que se dibuja ENCIMA de la pantalla de historial. El usuario sigue viendo el diff detras del dialogo.

**Por que:** el contexto importa. Cuando decidis si restaurar o no, queres ver el diff. Un dialogo que reemplaza toda la pantalla pierde esa info.

**Implementacion:** el campo `overlay overlayType` en el Model intercepta todo el input cuando no es `overlayNone`. Es un patron simple que escala a multiples tipos de dialogos.

## Errores comunes y tips

### Error: commit tree walking vs orden cronologico

En git, el log no siempre es cronologico. Si hubo merges, los commits pueden estar en orden topologico. Para lazyharness (un repo lineal sin branches), esto no es problema. Pero si ves commits en orden raro, verifica que estas usando `LogOrderCommitterTime`.

### Error: diff con binary noise

Si alguien sube un archivo binario al repo del harness, el diff va a tener basura. Los prompts son texto plano, asi que no deberia pasar. Pero como proteccion, podes verificar:

```go
if !utf8.ValidString(diffText) {
    return "(archivo binario — no se puede mostrar diff)"
}
```

### Error: dialogo de restauracion no se limpia

Si despues de cancelar el dialogo la pantalla se corrompe, probablemente no estas seteando `m.overlay = overlayNone` en todos los caminos de salida. Verifica que tanto `s/y` como `n/esc` lo limpien.

### Tip: el diff del primer commit

El primer commit no tiene commit anterior. Cuando `m.selected` apunta al ultimo commit (el mas viejo), el diff deberia comparar contra un tree vacio — mostrando todo como "agregado". Pasa `hashA` vacio a `Diff` para este caso.

### Tip: time.Since para tiempos relativos

`time.Since(t)` es equivalente a `time.Now().Sub(t)`. Retorna un `time.Duration` que podes comparar con constantes como `time.Hour`, `24*time.Hour`, etc. Es mucho mas legible que calcular diferencias de segundos manualmente.

## Ejercicios

### 1. Basico: lista de commits con hash, mensaje y tiempo relativo

Implementa la lista de commits a la izquierda. Cada entrada muestra:
- Hash corto (7 caracteres) en violeta.
- Mensaje truncado.
- Tiempo relativo ("hace 2 horas", "ayer").

Verifica que la navegacion con j/k funciona y que el commit seleccionado esta resaltado.

### 2. Intermedio: visor de diff con rojo/verde

Implementa el panel derecho que muestra el diff del commit seleccionado. Las lineas con `+` en verde, `-` en rojo, y el contexto en gris. Verifica que al navegar por los commits, el diff se actualiza.

### 3. Avanzado: flujo completo de restauracion

Implementa el flujo completo:
1. Presionar `r` abre el dialogo.
2. `s` confirma: restaura el archivo y crea un commit.
3. `n` cancela: cierra el dialogo sin hacer nada.
4. Despues de restaurar, volver a la vista de harness.

### 4. Bonus: test de restauracion end-to-end

Escribe un test que:
1. Cree un repo con 5 versiones de un archivo.
2. Restaure a la version 3.
3. Verifique que el contenido del archivo es el de la version 3.
4. Verifique que hay 6 commits (5 originales + 1 de restauracion).
5. Verifique que el ultimo commit tiene el mensaje "restaurar X a version Y".

## Para profundizar

- [go-git _examples/log](https://github.com/go-git/go-git/tree/master/_examples/log): ejemplo oficial de commit walking.
- [go-git _examples/revision](https://github.com/go-git/go-git/tree/master/_examples/revision): resolver hashes y referencias.
- [Unified diff format](https://en.wikipedia.org/wiki/Diff#Unified_format): la especificacion del formato que parseamos.
- [lazygit — diff rendering](https://github.com/jesseduffield/lazygit/blob/master/pkg/gui/controllers/helpers/diff_helper.go): como lazygit renderiza diffs (mucho mas sofisticado, pero inspirador).
- [Go by Example — Time](https://gobyexample.com/time): operaciones con tiempo en Go.

## Que sigue

En la [Clase 08](./08_tareas_y_workspace.md) construimos el panel de tareas y la pantalla de workspace. Vas a aprender `time.Time` con el formato unico de Go, enums con `iota` y JSON marshaling custom, `sort.Slice` para ordenar tareas, y el layout de tres paneles del mockup 04.
