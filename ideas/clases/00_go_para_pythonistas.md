# Clase 00: Go para pythonistas — el modelo mental

> Al terminar esta clase vas a tener un programa CLI que modela el dominio de lazyharness (Harness, Role), serializa a JSON y lee de JSON, con tests. Sin TUI todavia — primero el lenguaje.

## Prerequisitos

- Python intermedio/avanzado (clases, decoradores, typing, pip/poetry).
- Tener una terminal en Linux o WSL2.
- Un editor con soporte Go (VSCode + extension "Go" instala todo automatico).

## Conceptos de Go que vas a aprender

### 1. Instalacion y toolchain

Go es un solo binario que incluye compilador, linker, gestor de dependencias, formateador y linter. No hay virtualenvs ni pip.

```bash
# Descargar e instalar (version 1.24.x o superior)
wget https://go.dev/dl/go1.24.4.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.4.linux-amd64.tar.gz

# Agregar al PATH — poner en ~/.bashrc o ~/.zshrc
export PATH=$PATH:/usr/local/go/bin
export PATH=$PATH:$(go env GOPATH)/bin

# Verificar
go version
# go version go1.24.4 linux/amd64
```

**Equivalente Python:** instalar Python + pip + poetry/uv + ruff + mypy + black. Go trae todo eso en un solo binario.

**Tradeoff:** menos flexibilidad (no podes elegir tu formatter ni tu linter), pero cero configuracion y cero debates de estilo en el equipo.

#### El toolchain completo

```bash
go run .           # compilar y ejecutar (como python main.py)
go build .         # compilar a binario (no existe en Python sin PyInstaller)
go test ./...      # correr todos los tests (como pytest)
go fmt ./...       # formatear codigo — UN solo estilo, no hay discusion
go vet ./...       # analisis estatico basico (como un mypy liviano)
```

`go fmt` es obligatorio por convencion. No hay "tabs vs spaces" ni "single vs double quotes". Go tiene un solo estilo y punto. Esto suena restrictivo pero en la practica elimina el 100% de las discusiones de formato en code review.

> **Tip:** Si usas VSCode, la extension de Go corre `gofmt` automaticamente al guardar. No tenes que pensar en esto nunca mas.

### 2. Modulos y dependencias

```bash
cd ~/stuff/lazyharness
go mod init github.com/Gerardo1909/lazyharness
```

Esto crea `go.mod`, el equivalente a `pyproject.toml`:

```
module github.com/Gerardo1909/lazyharness

go 1.24.4
```

Las dependencias se instalan con `go get`:

```bash
go get github.com/alguna/libreria
```

Y quedan registradas en `go.mod` (como poetry.lock pero en un solo archivo). Tambien se genera `go.sum` con los checksums de cada dependencia (no lo edites manualmente).

**Equivalente Python:** `pyproject.toml` + `poetry.lock` o `uv.lock`.

**Diferencia clave:** en Go no hay virtualenvs. Las dependencias se descargan a `$GOPATH/pkg/mod/` (global), pero cada proyecto usa las versiones que dice su `go.mod`. Es como si pip instalara todo global pero respetara un lockfile por proyecto.

### 3. Tipado estatico con inferencia

Este es el cambio mas grande viniendo de Python. En Go, los tipos se conocen en compilacion — no en runtime.

```go
// Declaracion explicita
var name string = "lazyharness"
var count int = 42
var active bool = true

// Declaracion corta — infiere el tipo (lo mas comun)
name := "lazyharness"    // string
count := 42              // int
active := true           // bool
```

**Equivalente Python:**

```python
name: str = "lazyharness"  # type hint, pero es solo documentacion
count: int = 42            # Python no lo verifica en runtime
```

**Tradeoff:** Go te obliga a declarar tipos. Esto agrega verbosidad, pero el compilador atrapa errores que en Python solo descubris cuando el programa explota en produccion. Si usas mypy en modo estricto, Go se siente similar — pero mypy es opcional y lento, el compilador de Go es obligatorio y rapido (~0.5s para compilar proyectos medianos).

#### Colecciones basicas

```go
// Slice (equivalente a list de Python)
roles := []string{"arquitecto", "reviewer", "dev"}
roles = append(roles, "docs") // append retorna un slice nuevo

// Map (equivalente a dict de Python)
config := map[string]string{
    "provider": "anthropic",
    "model":    "opus",
}
valor := config["provider"] // "anthropic"

// Acceso seguro a un map
valor, existe := config["clave_que_no_existe"]
if !existe {
    fmt.Println("no esta")
}
```

**Lo que Go NO tiene:** list comprehensions, dict comprehensions, generadores. Todo se hace con `for` loops explicitos:

```go
// Python: names = [r.name for r in roles if r.active]
// Go:
var names []string
for _, r := range roles {
    if r.Active {
        names = append(names, r.Name)
    }
}
```

Es mas verboso, si. Pero tambien es mas explicito — cualquiera que lea el codigo entiende exactamente que pasa sin conocer la sintaxis de comprehensions.

### 4. Structs en vez de clases

Go no tiene clases ni herencia. Tiene **structs** (datos) y **metodos** (funciones asociadas a un struct).

```go
// Python:
// class Role:
//     def __init__(self, name: str, color: str, prompt_file: str):
//         self.name = name
//         self.color = color
//         self.prompt_file = prompt_file
//
//     def display_name(self) -> str:
//         return f"[{self.color}] {self.name}"

// Go:
type Role struct {
    Name       string `json:"name"`
    Color      string `json:"color"`
    PromptFile string `json:"prompt_file"`
}

// Los metodos se definen aparte, asociados al struct
func (r Role) DisplayName() string {
    return fmt.Sprintf("[%s] %s", r.Color, r.Name)
}
```

Las etiquetas entre backticks (`` `json:"name"` ``) le dicen a `encoding/json` como serializar el campo. Son metadata — como un decorador pero para campos.

**Composicion en vez de herencia:**

```go
// Python usaria herencia:
// class EnhancedRole(Role):
//     def __init__(self, ..., parent: str):
//         super().__init__(...)
//         self.parent = parent

// Go usa composicion (embeber un struct dentro de otro):
type EnhancedRole struct {
    Role              // "hereda" todos los campos y metodos de Role
    Parent string     `json:"parent"`
}

// EnhancedRole tiene Name, Color, PromptFile, DisplayName() Y Parent
```

**Tradeoff:** sin herencia no podes tener jerarquias profundas de tipos (Animal → Mammal → Dog). Pero en la practica, la herencia profunda es la fuente del 80% de los problemas de diseno en Python/Java. Composicion te obliga a pensar en "que tiene" en vez de "que es", y produce codigo mas facil de refactorizar.

> **Tip de Python:** si alguna vez usaste dataclasses o pydantic, los structs de Go se sienten muy parecidos. La diferencia es que son el UNICO mecanismo, no una alternativa.

### 5. Errores explicitos — no hay try/except

Este es el aspecto mas controversial de Go para alguien que viene de Python. Las funciones que pueden fallar retornan `(resultado, error)`:

```go
// Python:
// try:
//     data = open("harness.json").read()
// except FileNotFoundError as e:
//     print(f"no encontre el archivo: {e}")

// Go:
data, err := os.ReadFile("harness.json")
if err != nil {
    return fmt.Errorf("no pude leer harness.json: %w", err)
}
// usar data...
```

Vas a escribir `if err != nil` cientos de veces. Es verboso pero tiene ventajas concretas:

1. **No hay errores silenciosos.** En Python, si te olvidas de un try/except, la excepcion sube por el call stack y puede crashear en un lugar inesperado. En Go, si no checkeas `err`, el compilador te avisa (con linters como `errcheck`).

2. **El flujo de errores es visible.** Leyendo el codigo ves EXACTAMENTE donde puede fallar cada operacion. No hay excepciones magicas que aparecen 5 niveles arriba.

3. **El wrapping es explicito.** `%w` envuelve el error original para que puedas inspeccionarlo despues con `errors.Is()` o `errors.As()`.

```go
// Wrapping — agregar contexto sin perder el error original
data, err := os.ReadFile(path)
if err != nil {
    return fmt.Errorf("leyendo prompt de rol %s: %w", roleName, err)
}

// Mas arriba en el call stack, podes preguntar:
if errors.Is(err, os.ErrNotExist) {
    // el archivo no existe — crear uno nuevo
}
```

**Tradeoff:** es MAS verboso que try/except. Un bloque de 5 operaciones en Python se convierte en 15 lineas en Go (5 operaciones + 5 ifs + 5 returns). Pero cada linea es explicita y no hay sorpresas.

> **Referencia:** lee "Errors are values" de Rob Pike (blog.golang.org/errors-are-values) para entender la filosofia detras de esta decision.

### 6. Packages y visibilidad

Cada directorio es un package. La visibilidad se controla con la primera letra:

```go
// En el archivo internal/domain/harness.go
package domain

type Role struct { ... }       // PUBLICA (mayuscula) — visible fuera del package
type roleIndex struct { ... }  // privada (minuscula) — solo visible dentro de domain/

func LoadHarness() { ... }     // PUBLICA
func parseConfig() { ... }     // privada
```

**Equivalente Python:** `_nombre` como convencion (pero nadie la respeta), `__nombre` con name mangling (pero se puede acceder igual). En Go es binario: mayuscula = publico, minuscula = privado. El compilador lo enforcea.

**La carpeta `internal/`** es especial: Go garantiza que nada externo al modulo puede importar packages dentro de `internal/`. Es como si todo lo que esta ahi fuera "privado del proyecto".

```
lazyharness/
├── main.go                  # package main — punto de entrada
├── internal/
│   ├── domain/              # package domain
│   │   └── harness.go
│   ├── storage/             # package storage
│   │   └── filesystem.go
│   └── tui/                 # package tui
│       └── app.go
```

No hay `__init__.py`. Un package es un directorio con archivos `.go` que empiezan con `package nombre`. Todos los archivos en el mismo directorio deben tener el mismo `package`.

### 7. Punteros — lo justo y necesario

Python maneja todo por referencia internamente. En Go, los structs se copian por defecto:

```go
r1 := Role{Name: "arquitecto", Color: "#f7768e"}
r2 := r1        // r2 es una COPIA de r1
r2.Name = "dev" // r1.Name sigue siendo "arquitecto"

// Si queres modificar el original, usas un puntero:
r3 := &r1        // r3 apunta a r1
r3.Name = "dev"  // AHORA r1.Name es "dev"
```

**Cuando usar punteros:**
- Cuando queres que un metodo modifique el struct (receptor por puntero):
  ```go
  func (r *Role) SetColor(c string) { r.Color = c }
  ```
- Cuando el struct es grande y no queres copiar toda la memoria.
- Cuando necesitas representar "ausencia de valor" (un puntero puede ser `nil`).

**Cuando NO usar punteros:**
- Structs chicos (< 5 campos): copiarlos es gratis y evita bugs de mutacion.

**Tradeoff vs Python:** en Python nunca pensas en esto — todo es referencia. En Go tenes que decidir, pero a cambio tenes control fino sobre cuando algo se copia y cuando se comparte. Para lazyharness, la mayoria de los structs de dominio (Role, Task) van a ser por valor; los modelos de la TUI van a usar punteros porque bubbletea los necesita.

## Lo que vamos a construir

Un programa CLI que:
1. Define los structs del dominio (Harness, Role)
2. Crea un Harness con roles
3. Lo serializa a JSON y lo imprime
4. Lo lee de un string JSON y verifica que los datos coincidan
5. Tiene tests

No hay TUI todavia — solo logica de dominio pura.

**Archivos a crear:**
- `internal/domain/harness.go`
- `internal/domain/harness_test.go`
- `main.go` (temporal, solo para probar — lo reemplazamos en la clase 01)

## Implementacion paso a paso

### Paso 1: Inicializar el proyecto

```bash
cd ~/stuff/lazyharness
go mod init github.com/Gerardo1909/lazyharness
```

Verificar que `go.mod` se creo:

```bash
cat go.mod
# module github.com/Gerardo1909/lazyharness
#
# go 1.24.4
```

### Paso 2: Crear la estructura de directorios

```bash
mkdir -p internal/domain
```

### Paso 3: Definir los structs de dominio

Crea el archivo `internal/domain/harness.go`:

```go
package domain

import (
    "fmt"
    "time"
)

// Role representa un agente con nombre, color y archivo de prompt.
type Role struct {
    Name       string `json:"name"`
    Color      string `json:"color"`
    PromptFile string `json:"prompt_file"`
    Parent     string `json:"parent,omitempty"`
}

// DefaultColor es el color que toma un rol si no se especifica uno.
const DefaultColor = "#c0caf5"

func (r Role) DisplayName() string {
    color := r.Color
    if color == "" {
        color = DefaultColor
    }
    return fmt.Sprintf("[%s] %s", color, r.Name)
}

// Harness es el contenedor principal: un conjunto de roles + metadata.
type Harness struct {
    Name         string    `json:"name"`
    ProjectDir   string    `json:"project_dir"`
    PromptFormat string    `json:"prompt_format"`
    Provider     string    `json:"provider,omitempty"`
    Model        string    `json:"model,omitempty"`
    Roles        []Role    `json:"roles"`
    Workflow     []string  `json:"workflow"`
    CreatedAt    time.Time `json:"created_at"`
}

// HarnessSummary es una vista resumida para la pantalla Home.
type HarnessSummary struct {
    Name       string
    ProjectDir string
    RoleCount  int
    LastCommit string
    Provider   string
}

// NewHarness crea un Harness validado.
func NewHarness(name, promptFormat, projectDir string) (Harness, error) {
    if name == "" {
        return Harness{}, fmt.Errorf("el nombre del harness no puede estar vacio")
    }
    validFormats := map[string]bool{"xml": true, "md": true, "txt": true}
    if !validFormats[promptFormat] {
        return Harness{}, fmt.Errorf("formato invalido %q: debe ser xml, md o txt", promptFormat)
    }
    if projectDir == "" {
        return Harness{}, fmt.Errorf("el directorio del proyecto no puede estar vacio")
    }
    return Harness{
        Name:         name,
        ProjectDir:   projectDir,
        PromptFormat: promptFormat,
        Roles:        []Role{},
        Workflow:     []string{},
        CreatedAt:    time.Now(),
    }, nil
}

// AddRole agrega un rol al harness. Retorna error si ya existe un rol con ese nombre.
func (h *Harness) AddRole(r Role) error {
    for _, existing := range h.Roles {
        if existing.Name == r.Name {
            return fmt.Errorf("ya existe un rol con nombre %q", r.Name)
        }
    }
    h.Roles = append(h.Roles, r)
    return nil
}

// FindRole busca un rol por nombre. Retorna el rol y true si existe, o un Role vacio y false.
func (h Harness) FindRole(name string) (Role, bool) {
    for _, r := range h.Roles {
        if r.Name == name {
            return r, true
        }
    }
    return Role{}, false
}
```

**Cosas para notar:**

1. `package domain` — todos los archivos en `internal/domain/` tienen el mismo package.
2. Los campos de los structs empiezan con mayuscula (publicos). Los tags `json:"name"` controlan como se serializan.
3. `omitempty` hace que el campo se omita del JSON si esta vacio. Util para campos opcionales como Provider.
4. `NewHarness` es una funcion constructora — Go no tiene `__init__`. Es una convencion llamarla `New<Tipo>`.
5. `AddRole` recibe `*Harness` (puntero) porque MODIFICA el harness. `FindRole` recibe `Harness` (valor) porque solo LEE.
6. Los errores se retornan, no se lanzan. Cada caller decide que hacer con ellos.

### Paso 4: Escribir los tests

Crea `internal/domain/harness_test.go`:

```go
package domain

import (
    "encoding/json"
    "testing"
)

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
    if len(h.Roles) != 0 {
        t.Errorf("esperaba 0 roles iniciales, obtuve %d", len(h.Roles))
    }
}

func TestNewHarness_Validaciones(t *testing.T) {
    tests := []struct {
        name        string
        harnessName string
        format      string
        dir         string
        wantErr     bool
    }{
        {"valido", "dev-flow", "xml", "/home/user/proj", false},
        {"nombre vacio", "", "xml", "/home/user/proj", true},
        {"formato invalido", "dev-flow", "yaml", "/home/user/proj", true},
        {"dir vacio", "dev-flow", "xml", "", true},
        {"formato md", "dev-flow", "md", "/home/user/proj", false},
        {"formato txt", "dev-flow", "txt", "/home/user/proj", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := NewHarness(tt.harnessName, tt.format, tt.dir)
            if (err != nil) != tt.wantErr {
                t.Errorf("NewHarness() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}

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

func TestAddRole(t *testing.T) {
    h, _ := NewHarness("test", "xml", "/tmp")

    err := h.AddRole(Role{Name: "arquitecto", Color: "#f7768e", PromptFile: "arquitecto.xml"})
    if err != nil {
        t.Fatalf("no esperaba error al agregar primer rol: %v", err)
    }

    if len(h.Roles) != 1 {
        t.Fatalf("esperaba 1 rol, obtuve %d", len(h.Roles))
    }

    // Intentar agregar un rol duplicado
    err = h.AddRole(Role{Name: "arquitecto", Color: "#ff0000", PromptFile: "otro.xml"})
    if err == nil {
        t.Error("esperaba error al agregar rol duplicado")
    }
}

func TestFindRole(t *testing.T) {
    h, _ := NewHarness("test", "xml", "/tmp")
    h.AddRole(Role{Name: "dev", Color: "#9ece6a", PromptFile: "dev.xml"})

    // Buscar rol que existe
    role, found := h.FindRole("dev")
    if !found {
        t.Fatal("esperaba encontrar el rol 'dev'")
    }
    if role.Color != "#9ece6a" {
        t.Errorf("color esperado '#9ece6a', obtuve '%s'", role.Color)
    }

    // Buscar rol que no existe
    _, found = h.FindRole("no-existe")
    if found {
        t.Error("no esperaba encontrar 'no-existe'")
    }
}

func TestHarnessJSON(t *testing.T) {
    h, _ := NewHarness("dev-flow", "xml", "/home/user/proyecto")
    h.AddRole(Role{Name: "arquitecto", Color: "#f7768e", PromptFile: "arquitecto.xml"})
    h.AddRole(Role{Name: "reviewer", Color: "#7aa2f7", PromptFile: "reviewer.xml"})
    h.Workflow = []string{"arquitecto", "reviewer"}

    // Serializar a JSON
    data, err := json.MarshalIndent(h, "", "  ")
    if err != nil {
        t.Fatalf("error al serializar: %v", err)
    }

    // Deserializar
    var loaded Harness
    if err := json.Unmarshal(data, &loaded); err != nil {
        t.Fatalf("error al deserializar: %v", err)
    }

    // Verificar round-trip
    if loaded.Name != h.Name {
        t.Errorf("nombre: esperaba %q, obtuve %q", h.Name, loaded.Name)
    }
    if len(loaded.Roles) != 2 {
        t.Errorf("roles: esperaba 2, obtuve %d", len(loaded.Roles))
    }
    if loaded.Roles[0].Name != "arquitecto" {
        t.Errorf("primer rol: esperaba 'arquitecto', obtuve '%s'", loaded.Roles[0].Name)
    }
    if len(loaded.Workflow) != 2 {
        t.Errorf("workflow: esperaba 2 pasos, obtuve %d", len(loaded.Workflow))
    }
}
```

**Cosas para notar sobre los tests:**

1. El archivo tiene `package domain` (no `domain_test`) — es un test "de caja blanca" que puede acceder a campos privados.
2. `TestNewHarness_Validaciones` usa **table-driven tests** — el patron mas comun en Go. Defines una tabla de casos, los recorres con un for, y cada caso corre como subtest con `t.Run`.
3. No hay assertions magicas (`assertEqual`, `assertTrue`). Go usa `if` + `t.Errorf`. Es mas verboso que pytest pero no necesitas aprender una API de assertions.
4. `t.Fatalf` termina el test inmediatamente (como `pytest.fail`). `t.Errorf` reporta el error pero sigue ejecutando.

### Paso 5: Crear un main.go temporal

Crea `main.go` en la raiz del proyecto:

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"

    "github.com/Gerardo1909/lazyharness/internal/domain"
)

func main() {
    // Crear un harness de ejemplo
    h, err := domain.NewHarness("dev-flow", "xml", "~/dev/shop-api")
    if err != nil {
        log.Fatal(err)
    }

    // Agregar roles
    roles := []domain.Role{
        {Name: "arquitecto", Color: "#f7768e", PromptFile: "arquitecto.xml"},
        {Name: "code-reviewer", Color: "#7aa2f7", PromptFile: "code-reviewer.xml", Parent: "arquitecto"},
        {Name: "dev-backend", Color: "#9ece6a", PromptFile: "dev-backend.xml", Parent: "arquitecto"},
        {Name: "dev-frontend", Color: "#e0af68", PromptFile: "dev-frontend.xml", Parent: "arquitecto"},
        {Name: "docs", Color: "#bb9af7", PromptFile: "docs.xml"},
    }
    for _, r := range roles {
        if err := h.AddRole(r); err != nil {
            log.Fatal(err)
        }
    }

    h.Workflow = []string{"arquitecto", "code-reviewer", "dev-backend", "dev-frontend", "docs"}

    // Serializar a JSON
    data, err := json.MarshalIndent(h, "", "  ")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("=== Harness como JSON ===")
    fmt.Println(string(data))

    // Deserializar y verificar
    var loaded domain.Harness
    if err := json.Unmarshal(data, &loaded); err != nil {
        log.Fatal(err)
    }

    fmt.Printf("\n=== Verificacion ===\n")
    fmt.Printf("Nombre: %s\n", loaded.Name)
    fmt.Printf("Roles: %d\n", len(loaded.Roles))
    for _, r := range loaded.Roles {
        fmt.Printf("  %s\n", r.DisplayName())
    }
    fmt.Printf("Workflow: %v\n", loaded.Workflow)
}
```

### Paso 6: Ejecutar y verificar

```bash
# Ejecutar el programa
go run .

# Output esperado:
# === Harness como JSON ===
# {
#   "name": "dev-flow",
#   "project_dir": "~/dev/shop-api",
#   ...
# }
# === Verificacion ===
# Nombre: dev-flow
# Roles: 5
#   [#f7768e] arquitecto
#   [#7aa2f7] code-reviewer
#   ...

# Correr los tests
go test ./...

# Output esperado:
# ok  github.com/Gerardo1909/lazyharness/internal/domain  0.002s

# Con verbose para ver cada test individual
go test ./internal/domain/ -v

# Formatear el codigo (deberia no cambiar nada si tu editor ya lo hace)
go fmt ./...
```

## Tradeoffs y decisiones de diseno

### Decision 1: Structs de dominio sin dependencias externas

Los structs en `internal/domain/` no importan nada de bubbletea, go-git ni os. Solo usan la stdlib (`fmt`, `time`, `encoding/json`).

**Por que:** un dominio puro es testeable sin setup (no necesitas levantar una TUI ni crear archivos), y se puede reutilizar si cambias de framework TUI. Es el patron "Clean Architecture" / "Hexagonal" — el dominio no sabe que existe el mundo exterior.

**Lo que perdemos:** no podemos poner logica de "guardar en disco" o "hacer commit" en los structs de dominio. Eso va en `storage/`. Es mas codigo, pero cada pieza es independiente.

**Cuando cambiar de opinion:** si el proyecto es tan chico que la separacion agrega mas complejidad de la que resuelve. Para lazyharness, con 5 pantallas y multiples operaciones, la separacion vale la pena.

### Decision 2: Constructores con validacion (`NewHarness`)

En Python podrias validar en `__init__` o usar un validator como pydantic. En Go, la convencion es tener una funcion `New<Tipo>()` que valida y retorna `(Tipo, error)`.

**Por que:** no podes crear un Harness invalido si siempre usas `NewHarness()`. El compilador no te obliga a usarla (alguien puede hacer `domain.Harness{Name: ""}`), pero la convencion lo deja claro.

**Alternativa:** usar el metodo `Validate()` aparte, como hace pydantic con `model_validate()`. Esto es util cuando el struct viene de JSON (ya deserializado) y queres validar despues. Para lazyharness, hacemos ambos: `NewHarness` para creacion programatica, y un futuro `Validate()` para datos leidos de disco.

### Decision 3: Table-driven tests

Los tests en Go se estructuran como tablas de casos. Es mas codigo que pytest parametrize, pero tiene ventajas:

```go
// Go — table-driven
tests := []struct {
    name    string
    input   string
    want    string
    wantErr bool
}{
    {"caso feliz", "input", "output", false},
    {"error", "", "", true},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) { ... })
}
```

```python
# Python — pytest parametrize
@pytest.mark.parametrize("input,expected,raises", [
    ("input", "output", False),
    ("", "", True),
])
def test_thing(input, expected, raises):
    ...
```

**Tradeoff:** Go es mas verboso (definir el struct de la tabla, el for loop) pero el IDE navega mejor los subtests y cada caso tiene un nombre explicito que aparece en la salida.

## Errores comunes y tips

### Error: "imported and not used"

```
./main.go:4:2: "fmt" imported and not used
```

Go no permite imports sin usar. Si declaras `import "fmt"` y no usas `fmt`, no compila. Parece molesto pero evita dependencias fantasma.

**Fix:** borra el import. Si usas VSCode con la extension de Go, esto se hace automatico al guardar.

### Error: variable shadowing con `:=`

```go
err := doSomething()
if err != nil {
    // ...
}
result, err := doSomethingElse() // reutiliza 'err', crea 'result'
if err != nil {
    // ...
}

// PELIGRO: dentro de un bloque nuevo, := crea una variable NUEVA
if condition {
    err := doThirdThing() // esta es una NUEVA variable 'err', no la de afuera!
    // la 'err' de afuera no se ve afectada
}
```

**Tip:** usa `go vet` — detecta muchos casos de shadowing. Tambien `golangci-lint` con el check `shadow` activado.

### Error: JSON no serializa campos con minuscula

```go
type role struct {
    name string // minuscula — privada — JSON la IGNORA
}
```

`encoding/json` solo serializa campos exportados (mayuscula). Si tu JSON sale vacio, chequeá las mayusculas.

### Error: confundir `=` con `:=`

```go
x := 5  // declara x y le asigna 5
x = 10  // asigna 10 a x (ya existe)
y = 10  // ERROR: y no fue declarada
```

### Tip: `go test -race` desde el dia uno

```bash
go test ./... -race
```

El flag `-race` detecta accesos concurrentes a memoria compartida. Todavia no estamos usando goroutines, pero acostumbrate a correrlo siempre. Cuando lleguemos a concurrencia (clase 05+), te va a salvar de bugs invisibles.

## Ejercicios

### 1. Basico: completar el dominio

Agrega al struct `Harness` un metodo `RoleNames()` que retorne `[]string` con los nombres de todos los roles. Escribi el test correspondiente.

### 2. Intermedio: validacion de roles

Crea una funcion `ValidateRole(r Role) error` que verifique:
- Name no vacio
- Color es un hex valido (empieza con `#`, 7 caracteres) o vacio
- PromptFile termina en `.xml`, `.md` o `.txt`

Escribi table-driven tests cubriendo todos los casos.

### 3. Avanzado: estadisticas del harness

Crea un metodo `Stats() HarnessStats` que retorne:

```go
type HarnessStats struct {
    TotalRoles    int
    RolesWithParent int
    UniqueColors  int
}
```

Escribi tests que verifiquen con harnesses de distintos tamanos.

### 4. Bonus: benchmark

Go tiene benchmarks built-in:

```go
func BenchmarkFindRole(b *testing.B) {
    h, _ := NewHarness("bench", "xml", "/tmp")
    for i := 0; i < 100; i++ {
        h.AddRole(Role{Name: fmt.Sprintf("role-%d", i)})
    }
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        h.FindRole("role-50")
    }
}
```

Correr con `go test -bench=. ./internal/domain/`. Medir cuantas operaciones por segundo hace tu `FindRole`. Pensar: si tuvieras 10.000 roles, convendria usar un map en vez de un slice?

## Para profundizar

- [Tour of Go — Basics](https://go.dev/tour/basics/1): los primeros 15 ejercicios cubren todo lo que vimos en tipos, structs y slices.
- [Effective Go — Formatting, Names](https://go.dev/doc/effective_go#formatting): la filosofia detras de gofmt y las convenciones de nombres.
- [Go by Example — Structs, JSON, Errors](https://gobyexample.com): ejemplos minimos y ejecutables de cada concepto.
- [learn-go-with-tests — Structs, Methods, Interfaces](https://quii.gitbook.io/learn-go-with-tests/go-fundamentals/structs-methods-and-interfaces): tutorial TDD que ensena structs construyendo cosas.
- [Errors are values — Rob Pike](https://go.dev/blog/errors-are-values): la filosofia de errores en Go explicada por uno de sus creadores.
- [Dive into Go: A Python Programmer's Guide](https://dev.to/getvm/dive-into-go-a-python-programmers-guide-46d5): comparacion lado a lado Python/Go.

## Que sigue

En la [Clase 01](./01_hola_bubbletea.md) vamos a reemplazar este `main.go` temporal por una TUI real usando bubbletea. Vas a aprender la arquitectura Elm (Model/Update/View) que es el corazon de toda la app, y vas a ver tu primer programa renderizando texto estilizado en la terminal con colores Tokyo Night.

El dominio que definimos aca (`Harness`, `Role`) lo vamos a usar en TODAS las clases siguientes. Tene a mano este archivo porque es la base de todo.
