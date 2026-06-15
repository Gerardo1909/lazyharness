# Clase 03: Filesystem y JSON — datos reales

> Al terminar esta clase, la pantalla Home muestra harnesses leidos del disco real. Tambien podes crear un harness nuevo con la tecla `n`.

## Prerequisitos

- [Clase 00](./00_go_para_pythonistas.md): structs, JSON, tests.
- [Clase 01](./01_hola_bubbletea.md): bubbletea basics.
- [Clase 02](./02_listas_y_navegacion.md): lista de harnesses con datos fake, keybar.

## Conceptos de Go que vas a aprender

### 1. El package `os` — operaciones de filesystem

Go tiene todo lo que necesitas para trabajar con archivos en la stdlib:

```go
import "os"

// Leer un archivo completo
data, err := os.ReadFile("harness.json")

// Escribir un archivo completo
err := os.WriteFile("harness.json", data, 0644)

// Crear un directorio (con padres)
err := os.MkdirAll(".lazyharness/prompts", 0755)

// Verificar si un archivo/directorio existe
info, err := os.Stat(".lazyharness")
if os.IsNotExist(err) {
    // no existe
}

// Listar un directorio
entries, err := os.ReadDir(".lazyharness/prompts")
for _, entry := range entries {
    fmt.Println(entry.Name(), entry.IsDir())
}
```

**Equivalente Python:**

```python
from pathlib import Path

data = Path("harness.json").read_text()
Path("harness.json").write_text(data)
Path(".lazyharness/prompts").mkdir(parents=True, exist_ok=True)
Path(".lazyharness").exists()
list(Path(".lazyharness/prompts").iterdir())
```

**Tradeoff:** Python con pathlib es mas conciso. Go es mas verboso pero cada operacion retorna un error explicito que te obliga a manejarlo. En Python, si el disco esta lleno y `write_text` falla, la excepcion sube silenciosamente si no tenes try/except. En Go, el `err` te mira a la cara.

### 2. El package `filepath` — paths portables

```go
import "path/filepath"

// Unir paths (maneja separadores automaticamente)
path := filepath.Join(homeDir, ".lazyharness", "harness.json")
// Linux: /home/user/.lazyharness/harness.json
// Windows: C:\Users\user\.lazyharness\harness.json

// Obtener el directorio padre
dir := filepath.Dir("/home/user/.lazyharness/harness.json")
// /home/user/.lazyharness

// Obtener el nombre del archivo
name := filepath.Base("/home/user/.lazyharness/harness.json")
// harness.json

// Recorrer un arbol de directorios
filepath.WalkDir("/home/user/projects", func(path string, d fs.DirEntry, err error) error {
    if d.Name() == ".lazyharness" && d.IsDir() {
        // encontramos un harness!
    }
    return nil
})
```

**Regla de oro:** NUNCA concatenes paths con `+` ni `/`. Siempre usa `filepath.Join`. Esto garantiza portabilidad entre Linux, macOS y Windows (aunque WSL2 se comporte como Linux, el habito te salva cuando compilas para Windows).

### 3. `encoding/json` — marshaling profundo

Ya usamos JSON basico en la clase 00. Ahora vamos mas profundo:

```go
// Struct tags controlan la serializacion
type HarnessConfig struct {
    Name         string   `json:"name"`
    PromptFormat string   `json:"prompt_format"`
    Provider     string   `json:"provider,omitempty"` // se omite si esta vacio
    Roles        []RoleConfig `json:"roles"`
}

// Serializar con indentacion
data, err := json.MarshalIndent(config, "", "  ")

// Deserializar
var config HarnessConfig
err := json.Unmarshal(data, &config)

// Deserializar desde un io.Reader (archivo abierto)
file, err := os.Open("harness.json")
defer file.Close()
var config HarnessConfig
err = json.NewDecoder(file).Decode(&config)
```

**`omitempty`** omite el campo del JSON si tiene el valor zero (string vacio, 0, nil, false). Esto hace que el JSON sea mas limpio — no llena de `"provider": ""` los harnesses que no tienen provider configurado.

**Tradeoff JSON vs YAML vs TOML:**

| Formato | En stdlib? | Comentarios | Parsing |
|---------|-----------|-------------|---------|
| JSON | Si (`encoding/json`) | No soporta | Rapido, robusto |
| YAML | No (necesita `gopkg.in/yaml.v3`) | Soporta | Ambiguo en edge cases |
| TOML | No (necesita `github.com/BurntSushi/toml`) | Soporta | Bueno para config |

**Decision:** usamos JSON porque esta en la stdlib (cero dependencias extra), es universal (cualquier herramienta lo lee), y para los archivos de config de lazyharness no necesitamos comentarios. Si el usuario quiere anotar sus configs, puede usar un campo `"_comment"`.

### 4. Error wrapping con `%w`

Cuando una funcion interna falla, queres agregar contexto sin perder el error original:

```go
func loadHarness(path string) (HarnessConfig, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return HarnessConfig{}, fmt.Errorf("leyendo %s: %w", path, err)
    }

    var config HarnessConfig
    if err := json.Unmarshal(data, &config); err != nil {
        return HarnessConfig{}, fmt.Errorf("parseando JSON en %s: %w", path, err)
    }

    return config, nil
}
```

El `%w` (wrap) envuelve el error original. Mas arriba podes inspeccionarlo:

```go
config, err := loadHarness(path)
if errors.Is(err, os.ErrNotExist) {
    // el archivo no existe — crear uno nuevo
} else if err != nil {
    // otro error — no podemos continuar
    return err
}
```

**Equivalente Python:**

```python
try:
    data = open(path).read()
except FileNotFoundError:
    # manejar
except json.JSONDecodeError as e:
    raise ValueError(f"parseando JSON en {path}") from e  # exception chaining
```

**Tradeoff en profundidad de wrapping:** cada nivel agrega contexto, pero demasiados niveles producen mensajes como `"abriendo app: cargando harness: leyendo config: abriendo archivo: open /home/...: no such file"`. Regla practica: wrappea cuando el contexto no es obvio. Si una funcion se llama `ReadHarnessConfig`, no necesitas wrappear con "leyendo config" — el nombre de la funcion ya lo dice.

### 5. `t.TempDir()` — tests de filesystem sin cleanup

Go tiene una herramienta genial para testear filesystem:

```go
func TestSaveAndLoad(t *testing.T) {
    dir := t.TempDir() // crea un directorio temporal
    // Go lo borra automaticamente cuando el test termina

    // escribir archivos en dir...
    // leer archivos de dir...
    // verificar...
    // no hace falta defer os.RemoveAll(dir)
}
```

**Equivalente Python:** `tmp_path` de pytest es casi identico:

```python
def test_save_and_load(tmp_path):
    # tmp_path es un Path temporal que pytest limpia
```

### 6. `os.UserConfigDir()` y paths de configuracion

```go
configDir, err := os.UserConfigDir()
// Linux: /home/user/.config
// macOS: /Users/user/Library/Application Support
// Windows: C:\Users\user\AppData\Roaming
```

Para lazyharness, guardamos la lista de harnesses conocidos en `~/.config/lazyharness/known.json`. Esto sigue la convencion XDG en Linux.

**Tradeoff XDG vs dotfile:**
- `~/.config/lazyharness/` — sigue el estandar XDG, no contamina `$HOME`.
- `~/.lazyharness` — mas simple, mas visible, mas "clasico".
- lazygit usa `~/.config/lazygit/`. Seguimos la misma convencion.

## Lo que vamos a construir

1. `internal/storage/filesystem.go` — leer/escribir harnesses del disco.
2. Crear un harness real a mano para probar.
3. Conectar el storage con la pantalla Home — datos reales.
4. La tecla `n` para crear un harness nuevo.

**Archivos a crear:**
- `internal/storage/filesystem.go`
- `internal/storage/filesystem_test.go`

**Archivos a modificar:**
- `main.go` — cargar harnesses reales
- `internal/tui/home/model.go` — accion "nuevo"

## Implementacion paso a paso

### Paso 1: Definir el formato de harness.json

Antes de escribir codigo, definimos el schema. Un `.lazyharness/harness.json` se ve asi:

```json
{
  "name": "dev-flow",
  "project_dir": "/home/user/dev/shop-api",
  "prompt_format": "xml",
  "provider": "anthropic",
  "model": "claude-opus-4-7",
  "roles": [
    {
      "name": "arquitecto",
      "color": "#f7768e",
      "prompt_file": "arquitecto.xml",
      "parent": ""
    },
    {
      "name": "code-reviewer",
      "color": "#7aa2f7",
      "prompt_file": "code-reviewer.xml",
      "parent": "arquitecto"
    }
  ],
  "workflow": ["arquitecto", "code-reviewer", "dev-backend"],
  "created_at": "2026-06-10T14:30:00Z"
}
```

Y la estructura en disco:

```
proyecto/
└── .lazyharness/
    ├── harness.json          # metadata
    ├── prompts/
    │   ├── arquitecto.xml    # prompt del rol
    │   ├── code-reviewer.xml
    │   └── dev-backend.xml
    └── tareas.json           # registro de tareas (clase 08)
```

### Paso 2: Crear el storage

Crea `internal/storage/filesystem.go`:

```go
package storage

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"

    "github.com/Gerardo1909/lazyharness/internal/domain"
)

const (
    harnessDir      = ".lazyharness"
    harnessFile     = "harness.json"
    promptsDir      = "prompts"
    configDirName   = "lazyharness"
    knownFile       = "known.json"
)

// Store maneja la lectura/escritura de harnesses en disco
type Store struct {
    configDir string // ~/.config/lazyharness/
}

// NewStore crea un Store con el directorio de configuracion del sistema
func NewStore() (Store, error) {
    base, err := os.UserConfigDir()
    if err != nil {
        return Store{}, fmt.Errorf("obteniendo config dir: %w", err)
    }
    configDir := filepath.Join(base, configDirName)
    if err := os.MkdirAll(configDir, 0755); err != nil {
        return Store{}, fmt.Errorf("creando config dir: %w", err)
    }
    return Store{configDir: configDir}, nil
}

// NewStoreWithDir crea un Store con un directorio custom (util para tests)
func NewStoreWithDir(configDir string) Store {
    return Store{configDir: configDir}
}

// --- Harnesses conocidos ---

// KnownPaths retorna la lista de directorios de proyecto que tienen harness
func (s Store) KnownPaths() ([]string, error) {
    path := filepath.Join(s.configDir, knownFile)
    data, err := os.ReadFile(path)
    if os.IsNotExist(err) {
        return []string{}, nil
    }
    if err != nil {
        return nil, fmt.Errorf("leyendo known paths: %w", err)
    }

    var paths []string
    if err := json.Unmarshal(data, &paths); err != nil {
        return nil, fmt.Errorf("parseando known paths: %w", err)
    }
    return paths, nil
}

// AddKnownPath agrega un path a la lista de harnesses conocidos
func (s Store) AddKnownPath(projectDir string) error {
    paths, err := s.KnownPaths()
    if err != nil {
        return err
    }

    // Evitar duplicados
    for _, p := range paths {
        if p == projectDir {
            return nil
        }
    }

    paths = append(paths, projectDir)
    data, err := json.MarshalIndent(paths, "", "  ")
    if err != nil {
        return fmt.Errorf("serializando known paths: %w", err)
    }

    return os.WriteFile(filepath.Join(s.configDir, knownFile), data, 0644)
}

// --- Leer harness ---

// LoadHarness lee el harness.json de un directorio de proyecto
func (s Store) LoadHarness(projectDir string) (domain.Harness, error) {
    path := filepath.Join(projectDir, harnessDir, harnessFile)
    data, err := os.ReadFile(path)
    if err != nil {
        return domain.Harness{}, fmt.Errorf("leyendo harness en %s: %w", projectDir, err)
    }

    var h domain.Harness
    if err := json.Unmarshal(data, &h); err != nil {
        return domain.Harness{}, fmt.Errorf("parseando harness en %s: %w", projectDir, err)
    }

    return h, nil
}

// LoadAllSummaries carga un resumen de todos los harnesses conocidos
func (s Store) LoadAllSummaries() ([]domain.HarnessSummary, error) {
    paths, err := s.KnownPaths()
    if err != nil {
        return nil, err
    }

    var summaries []domain.HarnessSummary
    for _, projectDir := range paths {
        h, err := s.LoadHarness(projectDir)
        if err != nil {
            // Si un harness falla, lo saltamos (puede haber sido borrado)
            continue
        }

        provider := h.Provider
        if h.Model != "" {
            provider = provider + "/" + h.Model
        }

        summaries = append(summaries, domain.HarnessSummary{
            Name:       h.Name,
            ProjectDir: h.ProjectDir,
            RoleCount:  len(h.Roles),
            Provider:   provider,
        })
    }

    return summaries, nil
}

// --- Crear harness ---

// InitHarness crea la estructura de un harness nuevo en el directorio del proyecto
func (s Store) InitHarness(name, promptFormat, projectDir string) error {
    h, err := domain.NewHarness(name, promptFormat, projectDir)
    if err != nil {
        return err
    }

    baseDir := filepath.Join(projectDir, harnessDir)

    // Crear directorio .lazyharness/
    if err := os.MkdirAll(filepath.Join(baseDir, promptsDir), 0755); err != nil {
        return fmt.Errorf("creando estructura de directorios: %w", err)
    }

    // Escribir harness.json
    data, err := json.MarshalIndent(h, "", "  ")
    if err != nil {
        return fmt.Errorf("serializando harness: %w", err)
    }
    if err := os.WriteFile(filepath.Join(baseDir, harnessFile), data, 0644); err != nil {
        return fmt.Errorf("escribiendo harness.json: %w", err)
    }

    // Registrar en known paths
    if err := s.AddKnownPath(projectDir); err != nil {
        return fmt.Errorf("registrando harness: %w", err)
    }

    return nil
}

// ReadPrompt lee el contenido de un archivo de prompt
func (s Store) ReadPrompt(projectDir string, promptFile string) (string, error) {
    path := filepath.Join(projectDir, harnessDir, promptsDir, promptFile)
    data, err := os.ReadFile(path)
    if err != nil {
        return "", fmt.Errorf("leyendo prompt %s: %w", promptFile, err)
    }
    return string(data), nil
}
```

### Paso 3: Tests del storage

Crea `internal/storage/filesystem_test.go`:

```go
package storage

import (
    "os"
    "path/filepath"
    "testing"
)

func TestInitAndLoadHarness(t *testing.T) {
    configDir := t.TempDir()
    projectDir := t.TempDir()

    store := NewStoreWithDir(configDir)

    // Crear un harness
    err := store.InitHarness("test-harness", "xml", projectDir)
    if err != nil {
        t.Fatalf("error creando harness: %v", err)
    }

    // Verificar que los archivos se crearon
    harnessPath := filepath.Join(projectDir, ".lazyharness", "harness.json")
    if _, err := os.Stat(harnessPath); os.IsNotExist(err) {
        t.Fatal("harness.json no fue creado")
    }

    promptsPath := filepath.Join(projectDir, ".lazyharness", "prompts")
    if _, err := os.Stat(promptsPath); os.IsNotExist(err) {
        t.Fatal("directorio prompts/ no fue creado")
    }

    // Cargar el harness
    h, err := store.LoadHarness(projectDir)
    if err != nil {
        t.Fatalf("error cargando harness: %v", err)
    }

    if h.Name != "test-harness" {
        t.Errorf("nombre esperado 'test-harness', obtuve '%s'", h.Name)
    }
    if h.PromptFormat != "xml" {
        t.Errorf("formato esperado 'xml', obtuve '%s'", h.PromptFormat)
    }
}

func TestKnownPaths(t *testing.T) {
    configDir := t.TempDir()
    store := NewStoreWithDir(configDir)

    // Inicialmente no hay paths
    paths, err := store.KnownPaths()
    if err != nil {
        t.Fatalf("error leyendo known paths: %v", err)
    }
    if len(paths) != 0 {
        t.Errorf("esperaba 0 paths, obtuve %d", len(paths))
    }

    // Agregar un path
    if err := store.AddKnownPath("/home/user/proyecto"); err != nil {
        t.Fatalf("error agregando path: %v", err)
    }

    // Verificar que se guardo
    paths, _ = store.KnownPaths()
    if len(paths) != 1 || paths[0] != "/home/user/proyecto" {
        t.Errorf("esperaba [/home/user/proyecto], obtuve %v", paths)
    }

    // Agregar duplicado — no deberia duplicar
    store.AddKnownPath("/home/user/proyecto")
    paths, _ = store.KnownPaths()
    if len(paths) != 1 {
        t.Errorf("no deberia duplicar paths, obtuve %d", len(paths))
    }
}

func TestLoadAllSummaries(t *testing.T) {
    configDir := t.TempDir()
    store := NewStoreWithDir(configDir)

    // Crear dos harnesses en directorios distintos
    dir1 := t.TempDir()
    dir2 := t.TempDir()

    store.InitHarness("harness-1", "xml", dir1)
    store.InitHarness("harness-2", "md", dir2)

    summaries, err := store.LoadAllSummaries()
    if err != nil {
        t.Fatalf("error cargando summaries: %v", err)
    }

    if len(summaries) != 2 {
        t.Fatalf("esperaba 2 summaries, obtuve %d", len(summaries))
    }
}

func TestLoadHarness_NoExiste(t *testing.T) {
    configDir := t.TempDir()
    store := NewStoreWithDir(configDir)

    _, err := store.LoadHarness("/no/existe")
    if err == nil {
        t.Error("esperaba error al cargar harness que no existe")
    }
}

func TestInitHarness_Validacion(t *testing.T) {
    configDir := t.TempDir()
    store := NewStoreWithDir(configDir)

    err := store.InitHarness("", "xml", "/tmp")
    if err == nil {
        t.Error("esperaba error con nombre vacio")
    }

    err = store.InitHarness("test", "yaml", "/tmp")
    if err == nil {
        t.Error("esperaba error con formato invalido")
    }
}
```

```bash
go test ./internal/storage/ -v
```

### Paso 4: Conectar el storage con main.go

Actualiza `main.go`:

```go
package main

import (
    "fmt"
    "os"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/Gerardo1909/lazyharness/internal/storage"
    "github.com/Gerardo1909/lazyharness/internal/tui"
)

func main() {
    store, err := storage.NewStore()
    if err != nil {
        fmt.Fprintf(os.Stderr, "error inicializando storage: %v\n", err)
        os.Exit(1)
    }

    summaries, err := store.LoadAllSummaries()
    if err != nil {
        fmt.Fprintf(os.Stderr, "error cargando harnesses: %v\n", err)
        os.Exit(1)
    }

    app := tui.NewApp(summaries)
    p := tea.NewProgram(app, tea.WithAltScreen())

    if _, err := p.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}
```

### Paso 5: Crear un harness de prueba manualmente

Para probar sin la UI de creacion todavia, crea uno a mano:

```bash
# Elegir un directorio de proyecto (puede ser cualquiera)
mkdir -p /tmp/mi-proyecto/.lazyharness/prompts

# Crear harness.json
cat > /tmp/mi-proyecto/.lazyharness/harness.json << 'EOF'
{
  "name": "mi-primer-harness",
  "project_dir": "/tmp/mi-proyecto",
  "prompt_format": "xml",
  "provider": "anthropic",
  "model": "claude-opus-4-7",
  "roles": [
    {"name": "arquitecto", "color": "#f7768e", "prompt_file": "arquitecto.xml"},
    {"name": "reviewer", "color": "#7aa2f7", "prompt_file": "reviewer.xml"}
  ],
  "workflow": ["arquitecto", "reviewer"],
  "created_at": "2026-06-15T10:00:00Z"
}
EOF

# Crear un prompt de ejemplo
cat > /tmp/mi-proyecto/.lazyharness/prompts/arquitecto.xml << 'EOF'
<role>
  Sos el arquitecto del proyecto. Tu responsabilidad es disenar
  la estructura del codigo y tomar decisiones de alto nivel.
</role>
<constraints>
  Siempre consulta con @code-reviewer antes de aprobar cambios grandes.
</constraints>
EOF

# Registrar el path en known.json
mkdir -p ~/.config/lazyharness
echo '["/tmp/mi-proyecto"]' > ~/.config/lazyharness/known.json
```

Ahora ejecuta `go run .` y deberia aparecer tu harness en la lista.

### Paso 6: Agregar la accion "nuevo harness" (simplificada)

Esto es un preview — la version completa con dialogo modal la haremos cuando tengamos mas componentes. Por ahora, agregar la logica de crear desde la terminal seria una buena opcion para testear el flujo.

Agrega un flag a `main.go`:

```go
import "flag"

func main() {
    newHarness := flag.String("new", "", "Crear un nuevo harness: --new nombre:formato:path")
    flag.Parse()

    store, err := storage.NewStore()
    if err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }

    if *newHarness != "" {
        parts := strings.SplitN(*newHarness, ":", 3)
        if len(parts) != 3 {
            fmt.Fprintf(os.Stderr, "uso: --new nombre:formato:path\n")
            os.Exit(1)
        }
        if err := store.InitHarness(parts[0], parts[1], parts[2]); err != nil {
            fmt.Fprintf(os.Stderr, "error: %v\n", err)
            os.Exit(1)
        }
        fmt.Printf("Harness '%s' creado en %s\n", parts[0], parts[2])
        return
    }

    // ... resto del codigo TUI
}
```

```bash
# Crear un harness desde la terminal
go run . --new "test-flow:xml:/tmp/otro-proyecto"

# Verificar
go run .  # deberia aparecer en la lista
```

## Tradeoffs y decisiones de diseno

### Decision 1: JSON como "base de datos"

Usamos archivos JSON planos para toda la persistencia. No hay SQLite ni base de datos real.

**Por que:** un harness tiene ~5-20 roles y ~0-100 tareas. Es una cantidad trivial de datos. JSON es legible con cualquier editor, versionable con git, y no requiere dependencias. El usuario puede abrir `harness.json` con nano y editarlo — esto es un feature, no un bug.

**Cuando cambiar:** si un harness llegara a tener miles de tareas, la lectura secuencial de JSON se vuelve lenta. La migracion natural seria a SQLite con `modernc.org/sqlite` (pure Go, sin CGO). Pero para el MVP esto es overkill.

**Eficiencia:** `json.Unmarshal` de un archivo de 10KB tarda ~0.1ms. Leer 20 harnesses al inicio tarda ~2ms. Imperceptible.

### Decision 2: Store como struct, no funciones sueltas

Pusimos las operaciones en un struct `Store` en vez de funciones de package:

```go
// Lo que hicimos (struct)
store := storage.NewStore()
store.LoadHarness(path)

// Alternativa (funciones sueltas)
storage.LoadHarness(path, configDir)
```

**Por que struct:** el `Store` encierra el `configDir` — no tenemos que pasarlo en cada llamada. Tambien permite crear un `NewStoreWithDir(dir)` para tests que usen directorios temporales sin tocar la config real del usuario.

**Equivalente Python:** es como tener una clase `HarnessStore` con `__init__(self, config_dir)` vs funciones sueltas con un parametro extra. El patron es identico.

### Decision 3: `continue` silencioso al cargar summaries

En `LoadAllSummaries`, si un harness falla al cargar, lo saltamos:

```go
h, err := s.LoadHarness(projectDir)
if err != nil {
    continue // saltear harnesses rotos
}
```

**Tradeoff:** el usuario no se entera si un harness esta corrupto. Alternativa: acumular errores y mostrar un warning en la UI. Para el MVP, el skip silencioso evita que un harness roto bloquee toda la app.

## Errores comunes y tips

### Error: "permission denied" al crear directorios

```
creando estructura de directorios: mkdir .lazyharness: permission denied
```

Verifica los permisos del directorio del proyecto. `os.MkdirAll` usa el permiso `0755` (lectura para todos, escritura solo para el owner).

### Error: JSON invalido por trailing comma

```json
{
  "roles": [
    {"name": "dev"},  // trailing comma — JSON no lo permite!
  ]
}
```

A diferencia de JavaScript, JSON estandar NO permite trailing commas. Go's `encoding/json` falla si lo encuentra.

### Error: paths absolutos vs relativos

Si guardas `/home/user/proyecto` en `known.json` y despues corres la app desde otro usuario, no va a encontrar el directorio. Usa paths absolutos siempre — son mas robustos.

Usa `filepath.Abs()` para resolver paths relativos:

```go
absPath, err := filepath.Abs(relativePath)
```

### Error: `os.ReadFile` vs `os.Open`

- `os.ReadFile(path)` — lee TODO el archivo a memoria. Rapido y simple para archivos chicos (<1MB).
- `os.Open(path)` + `json.NewDecoder` — lee en streaming. Mejor para archivos grandes.

Para `harness.json` (unos pocos KB), `ReadFile` es perfecto. No uses streaming donde no hace falta — agrega complejidad sin beneficio.

### Tip: `os.IsNotExist` vs `errors.Is`

Ambos funcionan, pero `errors.Is` es el enfoque moderno:

```go
// Clasico
if os.IsNotExist(err) { ... }

// Moderno (funciona con errores wrapeados)
if errors.Is(err, os.ErrNotExist) { ... }
```

Usa `errors.Is` porque funciona incluso si el error fue wrapeado con `fmt.Errorf("...: %w", err)`. `os.IsNotExist` solo funciona con el error directo.

## Ejercicios

### 1. Basico: listar prompts de un harness

Agrega un metodo `ListPrompts(projectDir string) ([]string, error)` a `Store` que retorne los nombres de archivos en la carpeta `prompts/`. Escribi el test.

### 2. Intermedio: guardar prompt

Agrega `SavePrompt(projectDir, filename, content string) error` que escriba un archivo de prompt. Si el archivo ya existe, lo sobreescribe. Si no, lo crea. Test con `t.TempDir()`.

### 3. Avanzado: detectar harnesses automaticamente

Agrega un metodo `ScanDir(rootDir string) ([]string, error)` que recorra recursivamente un directorio buscando carpetas `.lazyharness/`. Usa `filepath.WalkDir`. Limita la profundidad a 3 niveles para no escanear todo el disco.

Piensa en la eficiencia: WalkDir visita cada archivo. Con un disco con 100K archivos, puede tardar segundos. Compara con un enfoque de solo buscar en `~/dev/` vs escanear todo `$HOME`.

### 4. Bonus: harness corrupto

Escribe un test que cree un `harness.json` con JSON invalido y verifique que `LoadHarness` retorna un error descriptivo. Verifica que `LoadAllSummaries` no falla — solo salta el harness roto.

## Para profundizar

- [Go by Example — Reading Files](https://gobyexample.com/reading-files): las distintas formas de leer archivos.
- [Go by Example — JSON](https://gobyexample.com/json): encoding/decoding con struct tags.
- [Go by Example — Errors](https://gobyexample.com/errors): el patron de error handling que usamos.
- [learn-go-with-tests — Mocking](https://quii.gitbook.io/learn-go-with-tests/go-fundamentals/mocking): como mockear filesystem para tests.
- [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/latest/): el estandar que seguimos para config dirs.
- [lazygit config location](https://github.com/jesseduffield/lazygit/blob/master/docs/Config.md): como lazygit resuelve sus config paths.

## Que sigue

En la [Clase 04](./04_vista_de_harness.md) construimos la vista de harness — el mockup 02 con tres paneles: sidebar de roles en arbol, prompt read-only con @referencias coloreadas, y barra de acciones. Vas a aprender composicion de modelos, routing entre vistas, y el componente viewport para contenido scrolleable.
