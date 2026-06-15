# Clase 05: Git desde Go — versionado automatico

> Al terminar esta clase, cada harness tiene su propio repo git. Guardar un prompt crea un commit automatico. El header muestra el hash del ultimo commit y si hay cambios sin guardar.

## Prerequisitos

- [Clase 03](./03_filesystem_y_json.md): storage filesystem, leer/escribir archivos.
- [Clase 04](./04_vista_de_harness.md): vista de harness con sidebar y prompt view.

## Conceptos de Go que vas a aprender

### 1. go-git — git puro en Go

go-git implementa git completo en Go, sin depender del binario `git`:

```bash
go get github.com/go-git/go-git/v5
```

```go
import (
    git "github.com/go-git/go-git/v5"
    "github.com/go-git/go-git/v5/plumbing/object"
)

// Inicializar un repo
repo, err := git.PlainInit("/path/.lazyharness", false)

// Abrir un repo existente
repo, err := git.PlainOpen("/path/.lazyharness")

// Obtener el worktree
wt, err := repo.Worktree()

// Agregar archivos al staging
wt.Add("harness.json")
wt.Add("prompts/arquitecto.xml")

// Hacer commit
hash, err := wt.Commit("actualizo prompt de arquitecto", &git.CommitOptions{
    Author: &object.Signature{
        Name:  "lazyharness",
        Email: "lazyharness@local",
        When:  time.Now(),
    },
})
```

**Equivalente Python:**

```python
# GitPython (shell out a git)
from git import Repo
repo = Repo.init("/path/.lazyharness")
repo.index.add(["harness.json"])
repo.index.commit("actualizo prompt")

# Dulwich (pure Python, como go-git)
from dulwich.repo import Repo
repo = Repo.init("/path/.lazyharness")
```

**Tradeoff go-git vs shell out a `git`:**

| Aspecto | go-git (pure Go) | `os/exec` + `git` |
|---------|-------------------|---------------------|
| Dependencia | Ninguna — incluido en el binario | Requiere `git` instalado |
| Performance | Mas lento en repos grandes | Mas rapido (C optimizado) |
| Features | ~90% de git | 100% de git |
| Binario | +~5MB al binario | Sin cambio |
| Testabilidad | Directo — todo en memoria | Necesita `git` en CI |

Para `.lazyharness/` (repos chicos, pocas operaciones), go-git es ideal. No obligamos al usuario a tener git instalado.

### 2. Interfaces como boundaries

Definimos una interface para git que permite mockear en tests:

```go
// En internal/domain/ o internal/storage/
type VersionControl interface {
    Init(path string) error
    CommitAll(path, message string) (string, error) // retorna hash
    Log(path, filename string) ([]CommitInfo, error)
    Diff(path, hashA, hashB, filename string) (string, error)
    IsDirty(path string) (bool, error)
}

type CommitInfo struct {
    Hash    string
    Message string
    When    time.Time
}
```

En produccion, `storage.GitRepo` implementa esta interface con go-git. En tests, `mockGit` retorna datos predefinidos.

**Equivalente Python:** es como definir un `Protocol` con typing:

```python
from typing import Protocol

class VersionControl(Protocol):
    def init(self, path: str) -> None: ...
    def commit_all(self, path: str, msg: str) -> str: ...
```

La diferencia: en Python el Protocol es opcional (duck typing). En Go, si tu struct no tiene los metodos, no compila.

**Este es el patron mas poderoso de Go para testabilidad.** Definir interfaces en el punto de uso (no en el punto de implementacion) y pasar dependencias como parametros. Es dependency injection sin frameworks.

### 3. context.Context para cancelacion

Las operaciones git pueden tardar. Go usa `context.Context` para cancelacion y timeouts:

```go
import "context"

func (g *GitRepo) CommitAll(ctx context.Context, path, message string) (string, error) {
    // Si el contexto se cancela, esta operacion se interrumpe
    select {
    case <-ctx.Done():
        return "", ctx.Err()
    default:
    }

    repo, err := git.PlainOpen(path)
    // ... hacer el commit
}

// Llamar con timeout:
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
hash, err := gitRepo.CommitAll(ctx, path, "mensaje")
```

**Equivalente Python:** `asyncio.wait_for(coro, timeout=10)` o threading con `Event`.

**Tradeoff:** para operaciones de go-git en repos chicos, el timeout no es estrictamente necesario. Pero es buena practica tenerlo para no bloquear la TUI si algo sale mal (disco lento, NFS, etc.).

### 4. Goroutines para operaciones lentas

En la clase 01 vimos que `Update` debe ser rapido. Las operaciones git (commit, log, diff) pueden tardar >100ms. Solucion: lanzar una goroutine que hace el trabajo y manda el resultado como un `tea.Msg`:

```go
// Definir el mensaje de resultado
type commitDoneMsg struct {
    hash string
    err  error
}

// Crear un Cmd que ejecuta el commit en background
func commitCmd(vc VersionControl, path, message string) tea.Cmd {
    return func() tea.Msg {
        hash, err := vc.CommitAll(context.Background(), path, message)
        return commitDoneMsg{hash: hash, err: err}
    }
}

// En Update:
case tea.KeyMsg:
    if key.Matches(msg, keys.Save) {
        return m, commitCmd(m.git, m.harnessPath, m.commitMessage)
    }

case commitDoneMsg:
    if msg.err != nil {
        m.statusMessage = "Error al guardar: " + msg.err.Error()
    } else {
        m.statusMessage = "Guardado: " + msg.hash[:7]
        m.lastCommit = msg.hash
    }
```

**La UI no se congela:** mientras el commit ocurre en una goroutine, la TUI sigue respondiendo a teclas. El resultado llega como un mensaje cuando termina.

## Lo que vamos a construir

1. `internal/storage/git.go` — operaciones git con go-git.
2. Inicializacion del repo git al crear un harness.
3. Commit automatico al guardar.
4. Estado dirty/clean en el header.

**Archivos a crear:**
- `internal/storage/git.go`
- `internal/storage/git_test.go`

**Archivos a modificar:**
- `internal/storage/filesystem.go` — init git al crear harness
- `internal/tui/harness/model.go` — mostrar estado git en header

## Implementacion paso a paso

### Paso 1: Implementar las operaciones git

Crea `internal/storage/git.go`:

```go
package storage

import (
    "fmt"
    "strings"
    "time"

    git "github.com/go-git/go-git/v5"
    "github.com/go-git/go-git/v5/plumbing"
    "github.com/go-git/go-git/v5/plumbing/object"
)

// CommitInfo es la informacion de un commit
type CommitInfo struct {
    Hash    string
    Message string
    When    time.Time
}

// GitRepo implementa operaciones git sobre un directorio .lazyharness
type GitRepo struct{}

func NewGitRepo() GitRepo {
    return GitRepo{}
}

// Init inicializa un repo git en el directorio dado
func (g GitRepo) Init(dir string) error {
    _, err := git.PlainInit(dir, false)
    if err != nil {
        return fmt.Errorf("inicializando repo git: %w", err)
    }
    return nil
}

// CommitAll agrega todos los archivos y crea un commit
func (g GitRepo) CommitAll(dir, message string) (string, error) {
    repo, err := git.PlainOpen(dir)
    if err != nil {
        return "", fmt.Errorf("abriendo repo: %w", err)
    }

    wt, err := repo.Worktree()
    if err != nil {
        return "", fmt.Errorf("obteniendo worktree: %w", err)
    }

    // Agregar todos los archivos
    if err := wt.AddGlob("."); err != nil {
        return "", fmt.Errorf("staging archivos: %w", err)
    }

    // Crear commit
    hash, err := wt.Commit(message, &git.CommitOptions{
        Author: &object.Signature{
            Name:  "lazyharness",
            Email: "lazyharness@local",
            When:  time.Now(),
        },
    })
    if err != nil {
        return "", fmt.Errorf("creando commit: %w", err)
    }

    return hash.String(), nil
}

// Log retorna el historial de commits, opcionalmente filtrado por archivo
func (g GitRepo) Log(dir string, filename string) ([]CommitInfo, error) {
    repo, err := git.PlainOpen(dir)
    if err != nil {
        return nil, fmt.Errorf("abriendo repo: %w", err)
    }

    logOpts := &git.LogOptions{
        Order: git.LogOrderCommitterTime,
    }
    if filename != "" {
        logOpts.PathFilter = func(path string) bool {
            return strings.HasSuffix(path, filename) || path == filename
        }
    }

    iter, err := repo.Log(logOpts)
    if err != nil {
        return nil, fmt.Errorf("obteniendo log: %w", err)
    }

    var commits []CommitInfo
    iter.ForEach(func(c *object.Commit) error {
        commits = append(commits, CommitInfo{
            Hash:    c.Hash.String(),
            Message: strings.TrimSpace(c.Message),
            When:    c.Author.When,
        })
        return nil
    })

    return commits, nil
}

// IsDirty verifica si hay cambios sin commitear
func (g GitRepo) IsDirty(dir string) (bool, error) {
    repo, err := git.PlainOpen(dir)
    if err != nil {
        return false, fmt.Errorf("abriendo repo: %w", err)
    }

    wt, err := repo.Worktree()
    if err != nil {
        return false, fmt.Errorf("obteniendo worktree: %w", err)
    }

    status, err := wt.Status()
    if err != nil {
        return false, fmt.Errorf("obteniendo status: %w", err)
    }

    return !status.IsClean(), nil
}

// LastCommit retorna la info del ultimo commit
func (g GitRepo) LastCommit(dir string) (CommitInfo, error) {
    repo, err := git.PlainOpen(dir)
    if err != nil {
        return CommitInfo{}, fmt.Errorf("abriendo repo: %w", err)
    }

    ref, err := repo.Head()
    if err != nil {
        if err == plumbing.ErrReferenceNotFound {
            return CommitInfo{}, nil // repo vacio, no hay commits
        }
        return CommitInfo{}, fmt.Errorf("obteniendo HEAD: %w", err)
    }

    commit, err := repo.CommitObject(ref.Hash())
    if err != nil {
        return CommitInfo{}, fmt.Errorf("obteniendo commit: %w", err)
    }

    return CommitInfo{
        Hash:    commit.Hash.String(),
        Message: strings.TrimSpace(commit.Message),
        When:    commit.Author.When,
    }, nil
}
```

### Paso 2: Tests de git

Crea `internal/storage/git_test.go`:

```go
package storage

import (
    "os"
    "path/filepath"
    "testing"
)

func TestGitInitAndCommit(t *testing.T) {
    dir := t.TempDir()
    g := NewGitRepo()

    // Init
    if err := g.Init(dir); err != nil {
        t.Fatalf("error en init: %v", err)
    }

    // Crear un archivo
    if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hola"), 0644); err != nil {
        t.Fatal(err)
    }

    // Commit
    hash, err := g.CommitAll(dir, "primer commit")
    if err != nil {
        t.Fatalf("error en commit: %v", err)
    }
    if len(hash) != 40 {
        t.Errorf("hash esperado de 40 chars, obtuve %d: %s", len(hash), hash)
    }

    // Verificar que no esta dirty
    dirty, err := g.IsDirty(dir)
    if err != nil {
        t.Fatal(err)
    }
    if dirty {
        t.Error("despues del commit, no deberia estar dirty")
    }
}

func TestGitLog(t *testing.T) {
    dir := t.TempDir()
    g := NewGitRepo()
    g.Init(dir)

    // Hacer 3 commits
    for i := 0; i < 3; i++ {
        content := []byte(fmt.Sprintf("version %d", i))
        os.WriteFile(filepath.Join(dir, "test.txt"), content, 0644)
        g.CommitAll(dir, fmt.Sprintf("commit %d", i))
    }

    // Log sin filtro
    commits, err := g.Log(dir, "")
    if err != nil {
        t.Fatal(err)
    }
    if len(commits) != 3 {
        t.Errorf("esperaba 3 commits, obtuve %d", len(commits))
    }

    // El primer commit en el log es el mas reciente
    if commits[0].Message != "commit 2" {
        t.Errorf("ultimo commit esperado 'commit 2', obtuve '%s'", commits[0].Message)
    }
}

func TestGitIsDirty(t *testing.T) {
    dir := t.TempDir()
    g := NewGitRepo()
    g.Init(dir)

    // Crear archivo y commitear
    os.WriteFile(filepath.Join(dir, "test.txt"), []byte("v1"), 0644)
    g.CommitAll(dir, "initial")

    // No deberia estar dirty
    dirty, _ := g.IsDirty(dir)
    if dirty {
        t.Error("no deberia estar dirty despues del commit")
    }

    // Modificar sin commitear
    os.WriteFile(filepath.Join(dir, "test.txt"), []byte("v2"), 0644)

    // Ahora si deberia estar dirty
    dirty, _ = g.IsDirty(dir)
    if !dirty {
        t.Error("deberia estar dirty despues de modificar")
    }
}

func TestGitLastCommit(t *testing.T) {
    dir := t.TempDir()
    g := NewGitRepo()
    g.Init(dir)

    // Repo vacio
    info, err := g.LastCommit(dir)
    if err != nil {
        t.Fatal(err)
    }
    if info.Hash != "" {
        t.Error("repo vacio no deberia tener ultimo commit")
    }

    // Despues de un commit
    os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hola"), 0644)
    g.CommitAll(dir, "primer commit")

    info, err = g.LastCommit(dir)
    if err != nil {
        t.Fatal(err)
    }
    if info.Message != "primer commit" {
        t.Errorf("mensaje esperado 'primer commit', obtuve '%s'", info.Message)
    }
}
```

```bash
go test ./internal/storage/ -v
```

### Paso 3: Integrar git con el flujo de creacion de harness

Modifica `internal/storage/filesystem.go`, en `InitHarness`:

```go
func (s Store) InitHarness(name, promptFormat, projectDir string) error {
    // ... (codigo existente de crear directorios y harness.json) ...

    // Inicializar repo git
    g := NewGitRepo()
    baseDir := filepath.Join(projectDir, harnessDir)
    if err := g.Init(baseDir); err != nil {
        return fmt.Errorf("inicializando repo git: %w", err)
    }

    // Primer commit
    if _, err := g.CommitAll(baseDir, "harness creado: "+name); err != nil {
        return fmt.Errorf("commit inicial: %w", err)
    }

    // ... (registrar en known paths) ...
}
```

### Paso 4: Mostrar estado git en el header de la vista de harness

En `internal/tui/harness/model.go`, agregar informacion de git al header:

```go
// Agregar campos al Model:
type Model struct {
    // ... campos existentes ...
    lastCommit storage.CommitInfo
    isDirty    bool
}

// En View(), actualizar el header:
func (m Model) renderHeader() string {
    parts := []string{
        maintui.StyleTitle.Render(m.harness.Name),
        maintui.StyleComment().Render(m.harness.ProjectDir),
    }

    if m.isDirty {
        parts = append(parts, lipgloss.NewStyle().
            Foreground(tui.ColorYellow).
            Bold(true).
            Render("● sin guardar"))
    } else {
        parts = append(parts, lipgloss.NewStyle().
            Foreground(tui.ColorGreen).
            Render("✓ limpio"))
    }

    if m.lastCommit.Hash != "" {
        parts = append(parts, maintui.StyleComment().
            Render(m.lastCommit.Hash[:7]+" — "+m.lastCommit.Message))
    }

    return strings.Join(parts, "  ")
}
```

## Tradeoffs y decisiones de diseno

### Decision 1: go-git pure Go vs `os/exec` + `git`

Elegimos go-git. El usuario no necesita tener git instalado — todo va dentro del binario.

**Cuando cambiar:** si necesitamos operaciones avanzadas (rebase, merge complicado, sparse checkout) que go-git no soporta. Para el scope de lazyharness (init, add, commit, log, diff, checkout de un archivo), go-git cubre todo.

### Decision 2: Un repo git POR harness

Cada `.lazyharness/` tiene su propio repo git, independiente del repo del proyecto. Esto es un diseno deliberado del [documento de requerimientos](../1_requerimientos.md) (decision 3).

**Ventaja:** la historia del harness no se mezcla con la del codigo. Podes tener 50 commits en el harness sin ensuciar el log del proyecto.

**Desventaja:** si compartis el harness con el equipo (`.lazyharness/` comiteado en el repo del proyecto), tenes un repo anidado. Esto se maneja con `.gitignore` (personal) o documentando que es una subcarpeta (compartido).

### Decision 3: Commit en cada save vs batch

El requerimiento B1 dice que cada guardado es un commit. Esto genera muchos commits pero da historial granular — podes volver a cualquier punto.

**Eficiencia:** un commit en go-git sobre un repo con 10 archivos tarda ~5ms. Imperceptible. Incluso con 1000 commits, el repo pesa pocos KB (solo texto).

## Errores comunes y tips

### Error: `ErrEmptyCommit`

go-git falla si intentas commitear sin cambios. Verifica con `IsDirty` antes:

```go
dirty, _ := g.IsDirty(dir)
if !dirty {
    return "", nil // nada que commitear
}
```

### Error: `wt.Add()` necesita paths relativos

```go
wt.Add("harness.json")              // BIEN — relativo al root del repo
wt.Add("/home/user/.lazyharness/harness.json")  // MAL — path absoluto
```

### Error: author faltante en commit

Si no pones `Author` en `CommitOptions`, go-git intenta leerlo de `.gitconfig`. Si no existe, falla. Siempre especifica el author explicitamente.

### Tip: testing con repos temporales

`t.TempDir()` te da un directorio limpio para cada test. Inicializa un repo ahi y tene un entorno aislado. Nunca testees contra tu repo real.

### Tip: `go test -race` para detectar concurrencia

Si lanzas operaciones git en goroutines, corre `go test -race ./internal/storage/` para detectar race conditions.

## Ejercicios

### 1. Basico: commit con timestamp

Modifica `CommitAll` para que el mensaje incluya un timestamp: `"[2026-06-15 10:30] actualizo prompt"`. Actualiza los tests.

### 2. Intermedio: log filtrado por archivo

Verifica que `Log(dir, "prompts/arquitecto.xml")` solo retorna commits donde se modifico ese archivo. Crea un test con commits que tocan archivos distintos.

### 3. Avanzado: diff entre versiones

Implementa `Diff(dir, hashA, hashB, filename string) (string, error)` que retorne el diff unificado entre dos commits para un archivo especifico. Usa `object.Commit.Patch()` de go-git.

Este diff lo vamos a usar en la [Clase 07](./07_historial_y_rollback.md) para mostrar cambios entre versiones.

### 4. Bonus: restaurar archivo a version anterior

Implementa `RestoreFile(dir, hash, filename string) error` que:
1. Lee el contenido del archivo en el commit especificado.
2. Lo escribe en el worktree actual.
3. Hace un nuevo commit: `"restaurar [filename] a version [hash[:7]]"`.

## Para profundizar

- [go-git README](https://github.com/go-git/go-git): documentacion principal.
- [go-git examples](https://github.com/go-git/go-git/tree/master/_examples): init, commit, log, diff, checkout — todos con codigo completo.
- [Pro Git — Appendix B: go-git](https://git-scm.com/book/en/v2/Appendix-B:-Embedding-Git-in-your-Applications-go-git): go-git explicado en el libro oficial de git.
- [Go by Example — Interfaces](https://gobyexample.com/interfaces): interfaces como mecanismo de testing.
- [learn-go-with-tests — Dependency Injection](https://quii.gitbook.io/learn-go-with-tests/go-fundamentals/dependency-injection): el patron de inyectar interfaces.

## Que sigue

En la [Clase 06](./06_editor_embebido.md) construimos el editor de prompts embebido. Presionar `e` abre un textarea donde editas el prompt, Ctrl+S guarda y genera un commit, y Esc cancela. Vas a aprender `bubbles/textarea`, escritura atomica de archivos, y coordinacion de operaciones secuenciales con `tea.Sequence`.
