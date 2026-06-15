# Clase 12: Distribucion -- de codigo a binario

> Al terminar esta clase, tenes un binario de lazyharness que se compila para Linux, macOS y Windows, tiene `--version` y `--help`, embede el system prompt de harness engineering dentro del binario, y se publica automaticamente con goreleaser cuando pusheas un tag.

## Prerequisitos

- [Clase 00](./00_go_para_pythonistas.md): `go build`, toolchain basico.
- [Clase 10](./10_ia_y_streaming.md): el system prompt de harness engineering que vamos a embeber.
- Todo el codigo de las clases 00-11 compilando y con tests pasando.

## Conceptos de Go que vas a aprender

### 1. `go build` con `GOOS` y `GOARCH` -- cross-compilation

Go puede compilar para cualquier plataforma desde cualquier plataforma. No necesitas una VM de macOS para compilar para macOS:

```bash
# Compilar para tu plataforma actual
go build -o lazyharness .

# Compilar para macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o lazyharness-darwin-amd64 .

# Compilar para macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o lazyharness-darwin-arm64 .

# Compilar para Windows
GOOS=windows GOARCH=amd64 go build -o lazyharness-windows-amd64.exe .

# Compilar para Linux ARM (Raspberry Pi)
GOOS=linux GOARCH=arm64 go build -o lazyharness-linux-arm64 .
```

Eso es todo. Dos variables de entorno y tenes un binario nativo para otra plataforma. No hay PyInstaller de 80MB, no hay Node embebido de 60MB. Es un binario estatico que corre sin dependencias.

**Equivalente Python:**

```bash
# PyInstaller — funciona a veces, binarios de ~80MB
pyinstaller --onefile main.py

# Nuitka — mejor pero complejo
python -m nuitka --standalone main.py

# Ninguno de los dos puede cross-compilar facilmente
```

**Equivalente Node:**

```bash
# pkg — no se mantiene activamente
npx pkg .

# Binarios de ~60MB con Node runtime embebido
```

**Tradeoff:** Go produce binarios mas grandes que C/C++ (~10-15MB vs ~1-3MB) pero MUCHO mas chicos que Python/Node. Y no requiere runtime instalado.

Para ver todas las plataformas soportadas:

```bash
go tool dist list
# linux/amd64
# linux/arm64
# darwin/amd64
# darwin/arm64
# windows/amd64
# ... y muchas mas
```

### 2. `ldflags` -- inyectar valores en compile time

Queremos que `lazyharness --version` muestre la version, el commit y la fecha de build. Pero no queremos hardcodear esos valores en el codigo (porque cambian en cada release).

`ldflags` (linker flags) te permiten setear variables de Go al momento de compilar:

```go
// main.go
package main

// Estas variables se setean en compile time con ldflags
var (
    version = "dev"       // se reemplaza por "v1.2.3"
    commit  = "unknown"   // se reemplaza por "abc1234"
    date    = "unknown"   // se reemplaza por "2026-06-15"
)
```

```bash
# Compilar con ldflags
go build -ldflags "\
    -X main.version=v1.0.0 \
    -X main.commit=$(git rev-parse --short HEAD) \
    -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
" -o lazyharness .
```

`-X main.version=v1.0.0` significa: "setea la variable `version` del package `main` al valor `v1.0.0`".

**Equivalente Python:** no hay equivalente directo. Tendrias que:
1. Generar un archivo `_version.py` en CI.
2. O leer de `pyproject.toml` en runtime.
3. O usar `importlib.metadata`.

Ninguna es tan limpia como ldflags porque Python no tiene una etapa de compilacion donde inyectar valores.

**Para reducir el tamano del binario:**

```bash
# -s quita la tabla de simbolos
# -w quita la informacion de debug DWARF
go build -ldflags "-s -w -X main.version=v1.0.0" -o lazyharness .
```

Esto reduce el binario de ~15MB a ~10MB. Es una compresion gratis sin afectar funcionalidad.

### 3. `embed` -- archivos dentro del binario

El paquete `embed` de Go permite incluir archivos estaticos directamente en el binario. No necesitas distribuir archivos sueltos junto al ejecutable:

```go
import "embed"

// Embeber un solo archivo
//go:embed prompts/harness_engineering.txt
var harnessEngineeringPrompt string

// Embeber multiples archivos
//go:embed templates/*.xml
var templates embed.FS

// Embeber un directorio entero
//go:embed assets
var assets embed.FS
```

El comentario `//go:embed` es una directiva del compilador. El archivo se incluye EN el binario — no se lee del disco en runtime.

```go
// Usar el string embebido directamente
func getHarnessEngineeringPrompt() string {
    return harnessEngineeringPrompt
}

// Usar el filesystem embebido
func readTemplate(name string) (string, error) {
    data, err := templates.ReadFile("templates/" + name)
    if err != nil {
        return "", err
    }
    return string(data), nil
}
```

**Equivalente Python:**

```python
# importlib.resources (Python 3.9+)
from importlib.resources import files

def get_template(name):
    return files("mypackage.templates").joinpath(name).read_text()

# O pkg_resources (antiguo, no recomendado)
import pkg_resources
data = pkg_resources.resource_string("mypackage", "templates/file.txt")
```

**Tradeoff embed vs archivos externos:**

| Aspecto | embed | Archivos externos |
|---------|-------|-------------------|
| Distribucion | Un solo binario | Binario + archivos |
| Tamano binario | Mayor (incluye archivos) | Menor |
| Modificabilidad | Hay que recompilar | Editar el archivo |
| Seguridad | Archivos no expuestos en disco | Archivos visibles |

Para lazyharness, el system prompt de harness engineering y los templates deben ir embebidos. Son parte de la app y no deberian ser modificables por el usuario. El `harness.json` y los prompts del usuario NO se embeben — esos se leen del disco.

**Reglas del path en embed:**

```go
// El path es RELATIVO al archivo .go que tiene la directiva
// Si este archivo es internal/runtime/skills.go:

//go:embed skills/harness_engineering.txt     // busca internal/runtime/skills/harness_engineering.txt
var prompt string

// NO podes usar paths absolutos
//go:embed /etc/something    // ERROR: no se permite

// NO podes subir directorios
//go:embed ../../data/file   // ERROR: no se permite
```

### 4. Makefile / Taskfile para automatizar

Un Makefile centraliza los comandos de build, test y release:

```makefile
# Makefile

# Variables
BINARY_NAME := lazyharness
VERSION := $(shell git describe --tags --always --dirty)
COMMIT := $(shell git rev-parse --short HEAD)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
    -X main.version=$(VERSION) \
    -X main.commit=$(COMMIT) \
    -X main.date=$(DATE)

# Target por defecto
.PHONY: all
all: build

# Compilar para la plataforma actual
.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) .

# Compilar para todas las plataformas
.PHONY: build-all
build-all:
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-arm64 .
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-windows-amd64.exe .

# Correr tests
.PHONY: test
test:
	go test ./... -race -count=1

# Correr tests con cobertura
.PHONY: coverage
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Abrir coverage.html en el browser"

# Lint
.PHONY: lint
lint:
	golangci-lint run ./...

# Formatear
.PHONY: fmt
fmt:
	go fmt ./...

# Limpiar
.PHONY: clean
clean:
	rm -f $(BINARY_NAME)
	rm -rf dist/
	rm -f coverage.out coverage.html

# Instalar localmente
.PHONY: install
install: build
	cp $(BINARY_NAME) $(GOPATH)/bin/

# Desarrollo con hot reload
.PHONY: dev
dev:
	air
```

**Equivalente Python:**

```makefile
# Makefile para Python
.PHONY: test
test:
	pytest

.PHONY: lint
lint:
	ruff check .

.PHONY: format
format:
	ruff format .
```

**Tradeoff Makefile vs Taskfile:**

| Aspecto | Makefile | Taskfile (go-task) |
|---------|----------|--------------------|
| Instalacion | Ya instalado en Linux/macOS | Hay que instalar `task` |
| Sintaxis | Arcana (tabs obligatorios, @, etc) | YAML legible |
| Ecosistema | Universal | Mas nuevo, menos conocido |
| Features | Basico | Variables, deps, watch, etc |

Para un proyecto Go, Makefile es el estandar. Lazygit, bubbletea, k9s -- todos usan Makefile. Usalo a menos que la sintaxis de Make te resulte insoportable.

### 5. goreleaser -- releases automatizados

goreleaser automatiza: compilar para todas las plataformas, crear archives (tar.gz/zip), generar checksums, publicar en GitHub Releases, y generar changelogs.

Instalar:

```bash
go install github.com/goreleaser/goreleaser/v2@latest
```

Crear `.goreleaser.yaml`:

```yaml
# .goreleaser.yaml
version: 2

project_name: lazyharness

before:
  hooks:
    - go mod tidy
    - go test ./...

builds:
  - id: lazyharness
    main: .
    binary: lazyharness
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.ShortCommit}}
      - -X main.date={{.Date}}

archives:
  - id: default
    format: tar.gz
    format_overrides:
      - goos: windows
        format: zip
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: "checksums.txt"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^ci:"

release:
  github:
    owner: Gerardo1909
    name: lazyharness
```

Flujo de release:

```bash
# 1. Taggear la version
git tag -a v1.0.0 -m "v1.0.0: primera version estable"
git push origin v1.0.0

# 2. Correr goreleaser (localmente o en CI)
goreleaser release --clean

# Esto:
# - Compila para linux/darwin/windows x amd64/arm64
# - Crea archives .tar.gz y .zip
# - Genera checksums
# - Publica en GitHub Releases con changelog automatico
```

**Para probar sin publicar:**

```bash
goreleaser release --snapshot --clean
# Compila todo pero no publica — para verificar que funciona
```

### 6. `flag` -- argumentos de linea de comandos

El paquete `flag` de la stdlib parsea argumentos CLI:

```go
import "flag"

func main() {
    // Definir flags
    showVersion := flag.Bool("version", false, "Mostrar version y salir")
    projectDir := flag.String("dir", ".", "Directorio del proyecto")
    debug := flag.Bool("debug", false, "Modo debug con logging verboso")

    // Parsear
    flag.Parse()

    // Usar
    if *showVersion {
        fmt.Printf("lazyharness %s (commit: %s, fecha: %s)\n", version, commit, date)
        os.Exit(0)
    }

    // flag.Args() retorna los argumentos posicionales (sin flags)
    // lazyharness /path/to/project → flag.Args() = ["/path/to/project"]
    if args := flag.Args(); len(args) > 0 {
        *projectDir = args[0]
    }
}
```

**Equivalente Python:**

```python
import argparse

parser = argparse.ArgumentParser(description="lazyharness")
parser.add_argument("--version", action="store_true")
parser.add_argument("--dir", default=".")
parser.add_argument("--debug", action="store_true")
args = parser.parse_args()
```

**Tradeoff `flag` vs cobra:**

| Aspecto | flag (stdlib) | cobra |
|---------|---------------|-------|
| Dependencia | Ninguna | +1 dep grande |
| Subcomandos | No soporta | `lazyharness init`, `lazyharness run` |
| Autocompletado | No | Si (bash, zsh, fish, powershell) |
| Ayuda | Basica | Rica, con grupos y examples |
| Complejidad | ~20 lineas | ~100 lineas de setup |

Para lazyharness, `flag` es suficiente. No tenemos subcomandos — la app abre la TUI directamente. Si en el futuro agregas `lazyharness init` o `lazyharness export`, migra a cobra.

## Lo que vamos a construir

1. Actualizar `main.go` con flags y version info.
2. Embeber el system prompt de harness engineering.
3. Makefile con todos los targets.
4. Configuracion de goreleaser.
5. Build para todas las plataformas.

```
  $ lazyharness --version
  lazyharness v1.0.0 (commit: abc1234, fecha: 2026-06-15T10:30:00Z)

  $ lazyharness --help
  Usage of lazyharness:
    -debug
          Modo debug con logging verboso
    -dir string
          Directorio del proyecto (default ".")
    -version
          Mostrar version y salir

  $ lazyharness ~/dev/shop-api
  (abre la TUI centrada en ese proyecto)

  $ make build-all
  dist/lazyharness-linux-amd64      (12MB)
  dist/lazyharness-linux-arm64      (11MB)
  dist/lazyharness-darwin-amd64     (12MB)
  dist/lazyharness-darwin-arm64     (11MB)
  dist/lazyharness-windows-amd64.exe (12MB)
```

**Archivos a crear:**
- `internal/runtime/skills/harness_engineering.txt` -- el prompt embebido
- `Makefile`
- `.goreleaser.yaml`

**Archivos a modificar:**
- `main.go` -- agregar flags, version, embed
- `internal/runtime/aiclient.go` -- usar el prompt embebido

## Implementacion paso a paso

### Paso 1: Actualizar main.go con flags y version

```go
package main

import (
    "flag"
    "fmt"
    "os"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/Gerardo1909/lazyharness/internal/tui"
)

// Seteadas en compile time con ldflags
var (
    version = "dev"
    commit  = "unknown"
    date    = "unknown"
)

func main() {
    // Flags
    showVersion := flag.Bool("version", false, "Mostrar version y salir")
    projectDir := flag.String("dir", ".", "Directorio del proyecto")
    debug := flag.Bool("debug", false, "Modo debug con logging verboso")

    flag.Usage = func() {
        fmt.Fprintf(os.Stderr, "lazyharness — TUI para construir y operar harnesses de agentes\n\n")
        fmt.Fprintf(os.Stderr, "Uso:\n")
        fmt.Fprintf(os.Stderr, "  lazyharness [flags] [directorio]\n\n")
        fmt.Fprintf(os.Stderr, "Flags:\n")
        flag.PrintDefaults()
        fmt.Fprintf(os.Stderr, "\nEjemplos:\n")
        fmt.Fprintf(os.Stderr, "  lazyharness                  # abrir en el directorio actual\n")
        fmt.Fprintf(os.Stderr, "  lazyharness ~/dev/proyecto    # abrir en un proyecto especifico\n")
        fmt.Fprintf(os.Stderr, "  lazyharness --version         # mostrar version\n")
    }

    flag.Parse()

    // --version
    if *showVersion {
        fmt.Printf("lazyharness %s (commit: %s, fecha: %s)\n", version, commit, date)
        os.Exit(0)
    }

    // Argumento posicional como directorio
    if args := flag.Args(); len(args) > 0 {
        *projectDir = args[0]
    }

    // Resolver path absoluto
    dir, err := resolveDir(*projectDir)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    // Logging en modo debug
    var opts []tea.ProgramOption
    if *debug {
        f, err := tea.LogToFile("lazyharness-debug.log", "debug")
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error creando log: %v\n", err)
            os.Exit(1)
        }
        defer f.Close()
    }
    opts = append(opts, tea.WithAltScreen())

    // Crear y arrancar la app
    model := tui.NewApp(dir, version)
    p := tea.NewProgram(model, opts...)
    if _, err := p.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}

func resolveDir(dir string) (string, error) {
    // Expandir ~ a home directory
    if dir == "~" || len(dir) > 1 && dir[:2] == "~/" {
        home, err := os.UserHomeDir()
        if err != nil {
            return "", fmt.Errorf("no se pudo resolver ~: %w", err)
        }
        dir = home + dir[1:]
    }

    // Verificar que existe
    info, err := os.Stat(dir)
    if err != nil {
        return "", fmt.Errorf("directorio '%s' no existe: %w", dir, err)
    }
    if !info.IsDir() {
        return "", fmt.Errorf("'%s' no es un directorio", dir)
    }

    // Resolver a path absoluto
    abs, err := filepath.Abs(dir)
    if err != nil {
        return "", fmt.Errorf("resolviendo path: %w", err)
    }

    return abs, nil
}
```

### Paso 2: Embeber el system prompt

Crea el archivo del prompt:

```
internal/runtime/skills/harness_engineering.txt
```

Con el contenido:

```
Sos un experto en harness engineering — el arte de disenar system prompts 
efectivos para agentes de IA que trabajan en equipo sobre un codebase.

Tu trabajo es ayudar al usuario a crear, mejorar y alinear los prompts de 
su harness. Un harness es un conjunto de roles (agentes) con jerarquia, 
donde cada rol tiene un system prompt que define su personalidad, 
responsabilidades y constraints.

Principios que seguis:
1. Cada rol debe tener un proposito claro y unico.
2. Los roles deben referenciarse entre si con @nombre para crear 
   dependencias explicitas.
3. Los constraints deben ser especificos y verificables.
4. El lenguaje del prompt debe ser directo y sin ambiguedades.
5. Cada rol debe saber que NO hacer (boundaries negativas).

Cuando el usuario te pide ayuda:
- Pregunta para entender el contexto del proyecto.
- Sugiere mejoras concretas con el texto exacto a cambiar.
- Si falta un rol, sugerilo con nombre, color y prompt inicial.
- Si hay redundancia entre roles, senalalo.
- Responde en espanol.
```

Ahora embelelo en Go:

```go
// internal/runtime/skills.go
package runtime

import _ "embed"

//go:embed skills/harness_engineering.txt
var HarnessEngineeringPrompt string
```

Y usalo en el AIClient:

```go
// En internal/tui/editor/model.go, cuando se activa el chat:
func buildSystemPrompt(currentPrompt string) string {
    return fmt.Sprintf("%s\n\nPrompt actual del rol:\n---\n%s\n---",
        runtime.HarnessEngineeringPrompt,
        currentPrompt,
    )
}
```

### Paso 3: Agregar CGO_ENABLED=0

Para que el binario sea verdaderamente estatico (sin dependencia de glibc), necesitas deshabilitar CGO:

```bash
CGO_ENABLED=0 go build -ldflags "-s -w" -o lazyharness .
```

Sin `CGO_ENABLED=0`, el binario puede depender de glibc y fallar en distros con versiones diferentes. Con `CGO_ENABLED=0`, todo se compila en Go puro y el binario corre en CUALQUIER Linux.

**Tradeoff:**

| Con CGO | Sin CGO (CGO_ENABLED=0) |
|---------|-------------------------|
| Puede usar librerias C | Solo Go puro |
| Binario depende de glibc | Binario 100% estatico |
| DNS usa resolver del SO | DNS usa resolver de Go |
| Puede tener mejor performance | Portabilidad total |

Para lazyharness no usamos librerias C, asi que `CGO_ENABLED=0` no pierde nada.

### Paso 4: Crear el Makefile

Crea el Makefile en la raiz del proyecto (el contenido completo esta en la seccion de conceptos arriba). Verificar que funciona:

```bash
# Compilar
make build
./lazyharness --version
# lazyharness v0.1.0-dirty (commit: abc1234, fecha: 2026-06-15T10:30:00Z)

# Tests
make test

# Compilar para todas las plataformas
make build-all
ls dist/
# lazyharness-linux-amd64
# lazyharness-linux-arm64
# lazyharness-darwin-amd64
# lazyharness-darwin-arm64
# lazyharness-windows-amd64.exe

# Ver tamano
ls -lh dist/
# 12M lazyharness-linux-amd64
# 11M lazyharness-linux-arm64
# etc
```

### Paso 5: Configurar goreleaser

Crea `.goreleaser.yaml` (el contenido completo esta en la seccion de conceptos arriba).

Probar localmente:

```bash
# Verificar configuracion
goreleaser check

# Build local sin publicar
goreleaser release --snapshot --clean

# Ver que genero
ls dist/
# lazyharness_0.1.0-SNAPSHOT_linux_amd64.tar.gz
# lazyharness_0.1.0-SNAPSHOT_darwin_arm64.tar.gz
# lazyharness_0.1.0-SNAPSHOT_windows_amd64.zip
# checksums.txt
```

### Paso 6: GitHub Actions para CI/CD (opcional)

Crea `.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: "1.24"

      - name: Run tests
        run: go test ./... -race -count=1

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Con esto, cada vez que pusheas un tag (`git push origin v1.0.0`), GitHub Actions compila para todas las plataformas y publica el release automaticamente.

### Paso 7: Optimizar tamano del binario

Verificar el tamano actual:

```bash
# Sin optimizar
go build -o lazyharness .
ls -lh lazyharness
# ~15MB

# Con strip de simbolos
go build -ldflags "-s -w" -o lazyharness .
ls -lh lazyharness
# ~10MB

# Con UPX (compresion de ejecutable)
upx --best lazyharness
ls -lh lazyharness
# ~4MB
```

**Tradeoff UPX:**

| Aspecto | Sin UPX | Con UPX |
|---------|---------|---------|
| Tamano | ~10MB | ~4MB |
| Startup | Inmediato | +50-100ms (descompresion) |
| Firma digital | OK | Rompe la firma |
| Antivirus | OK | Algunos lo marcan como sospechoso |

Para una TUI que se abre una vez y queda abierta, 50ms extra de startup es imperceptible. Pero los falsos positivos de antivirus en Windows pueden ser problematicos. Recomendacion: **no usar UPX en los releases oficiales**. Dejalo para uso personal.

## Tradeoffs y decisiones de diseno

### Decision 1: `flag` stdlib vs cobra

Elegimos `flag` porque lazyharness no tiene subcomandos. La interfaz es:

```
lazyharness [flags] [directorio]
```

Si en el futuro necesitamos subcomandos (`lazyharness init`, `lazyharness export template`), migramos a cobra. La migracion es mecanica: mover el codigo de flag.Parse a cobra commands.

### Decision 2: embed vs archivos de config externos

El system prompt de harness engineering va embebido. No es configurable por el usuario — es parte de la "inteligencia" de la app.

Los templates de harness (si los agregamos) tambien van embebidos. Son defaults que la app necesita para funcionar.

Los archivos del usuario (`harness.json`, prompts, `tareas.json`) NUNCA van embebidos — esos se leen del disco.

**Regla:** si el archivo es parte de la app, embebelo. Si es parte de los datos del usuario, leelo del disco.

### Decision 3: goreleaser vs scripts de CI manuales

goreleaser centraliza toda la logica de release en un archivo YAML declarativo. La alternativa es escribir scripts de bash en el CI que hagan lo mismo:

```bash
# Script manual — hay que mantenerlo
for os in linux darwin windows; do
    for arch in amd64 arm64; do
        GOOS=$os GOARCH=$arch go build -o "dist/lazyharness-$os-$arch" .
        tar czf "dist/lazyharness-$os-$arch.tar.gz" "dist/lazyharness-$os-$arch"
    done
done
sha256sum dist/*.tar.gz > dist/checksums.txt
gh release create $TAG dist/*.tar.gz dist/checksums.txt
```

goreleaser hace todo eso y mas (changelog, formula de homebrew, docker images) con menos codigo y menos errores.

### Decision 4: Binario estatico con CGO_ENABLED=0

El binario de lazyharness no depende de librerias C. go-git es pure Go, bubbletea es pure Go, todo es pure Go. No hay razon para habilitar CGO.

El unico caso donde CGO importaria es si usaramos SQLite (que es una libreria C) o alguna libreria nativa. Como no lo hacemos, `CGO_ENABLED=0` nos da portabilidad total gratis.

## Errores comunes y tips

### Error: olvidar `CGO_ENABLED=0`

```bash
# En Linux, esto puede generar un binario que depende de glibc
GOOS=linux go build -o lazyharness .

# En un container Alpine o distro vieja:
# ./lazyharness: not a dynamic executable

# Solucion:
CGO_ENABLED=0 GOOS=linux go build -o lazyharness .
# Ahora es 100% estatico
```

### Error: paths de embed relativos al archivo Go, no al proyecto

```go
// Si este archivo es internal/runtime/skills.go:

//go:embed skills/harness_engineering.txt  // BIEN: relativo a internal/runtime/
var prompt string

//go:embed ../../assets/prompt.txt  // MAL: no podes subir directorios
```

La solucion es poner los archivos embebidos cerca del archivo Go que los usa.

### Error: goreleaser config que no matchea

```yaml
# MAL: el binary name no matchea con el build
builds:
  - binary: lazyharness
archives:
  - name_template: "lh_{{ .Version }}"  # usa "lh" en vez de "lazyharness"
```

Siempre corre `goreleaser check` antes de publicar para validar la configuracion.

### Error: `flag.Parse()` despues de `flag.Args()`

```go
// MAL: flag.Args() retorna vacio si no parseaste primero
args := flag.Args()
flag.Parse()

// BIEN: parsear primero, usar despues
flag.Parse()
args := flag.Args()
```

### Tip: `go build -v` para ver que se compila

```bash
go build -v .
# github.com/charmbracelet/lipgloss
# github.com/charmbracelet/bubbletea
# github.com/go-git/go-git/v5
# github.com/Gerardo1909/lazyharness/internal/domain
# ...
```

Util para detectar dependencias inesperadas o ver el orden de compilacion.

### Tip: verificar el binario con `file` y `ldd`

```bash
# Verificar tipo de binario
file lazyharness
# lazyharness: ELF 64-bit LSB executable, x86-64, statically linked, ...

# Verificar dependencias dinamicas
ldd lazyharness
# not a dynamic executable  <-- BIEN, es estatico

# Si dice algo como:
# libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6
# Entonces NO es estatico — necesitas CGO_ENABLED=0
```

### Tip: `go tool nm` para ver que hay en el binario

```bash
# Ver simbolos del binario (util para debug)
go tool nm lazyharness | grep "main.version"
# 5a3210 D main.version
```

Esto confirma que ldflags seteo la variable correctamente.

### Tip: `tea.LogToFile` para debugging

```go
// En modo debug, bubbletea puede loguear a un archivo
if *debug {
    f, _ := tea.LogToFile("debug.log", "debug")
    defer f.Close()
}
```

Despues podes leer el log mientras la TUI corre:

```bash
# En otra terminal:
tail -f lazyharness-debug.log
```

## Ejercicios

### 1. Basico: mostrar version en el footer de la TUI

Agrega la version (`v1.0.0`) y el commit corto (`abc1234`) en la esquina inferior derecha de la barra de atajos de la pantalla Home. Usa lipgloss para renderizarlo en un color sutil.

### 2. Intermedio: flag `--export` para exportar un harness

Agrega un flag `--export <formato>` que exporte un harness como un archivo unico (JSON o tar.gz) en vez de abrir la TUI. Esto permite integrar lazyharness en scripts de CI.

```bash
lazyharness --export json ~/dev/shop-api > harness-backup.json
lazyharness --export tar ~/dev/shop-api > harness.tar.gz
```

### 3. Avanzado: auto-update

Implementa un mecanismo de auto-update que al ejecutar `lazyharness --update` descargue la ultima version de GitHub Releases y reemplace el binario actual. Usa la API de GitHub (`https://api.github.com/repos/Gerardo1909/lazyharness/releases/latest`) para encontrar la URL del binario correcto segun `runtime.GOOS` y `runtime.GOARCH`.

### 4. Bonus: Homebrew formula

goreleaser puede generar una formula de Homebrew automaticamente. Configuralo para que al publicar un release, se genere la formula y se pushee a un repositorio `homebrew-tap`. Asi los usuarios de macOS pueden instalar con `brew install Gerardo1909/tap/lazyharness`.

## Para profundizar

- [Go embed package](https://pkg.go.dev/embed): documentacion oficial de embed.
- [goreleaser docs](https://goreleaser.com/): guia completa de goreleaser.
- [Go Build Constraints](https://pkg.go.dev/go/build#hdr-Build_Constraints): compilacion condicional con build tags.
- [Shrink Go Binaries](https://blog.filippo.io/shrink-your-go-binaries-with-this-one-weird-trick/): tecnicas para reducir tamano.
- [lazygit Makefile](https://github.com/jesseduffield/lazygit/blob/master/Makefile): como lazygit maneja sus builds.
- [cobra](https://github.com/spf13/cobra): si en el futuro necesitas subcomandos.

## Que sigue

Con esto terminas el curso. Tenes una app TUI completa que:

- **Construye** harnesses de agentes con editor embebido y asistencia IA.
- **Versiona** cada cambio con git, con historial y rollback por rol.
- **Opera** invocando CLIs agentic con los prompts del harness.
- **Se distribuye** como un binario estatico para cualquier plataforma.

Proximos pasos para seguir mejorando lazyharness:

1. **Templates de harness** -- presets para casos comunes (equipo de desarrollo, documentacion, data pipeline). Embeberlos con `embed.FS`.
2. **Duplicar harness entre proyectos** -- `lazyharness --clone-from ~/otro-proyecto`.
3. **Marketplace** -- compartir harnesses con la comunidad via un repositorio de templates.
4. **Soporte para mas CLIs** -- Aider, Goose, Cline. Implementar un `Executor` para cada uno.
5. **Orquestacion automatica** -- en vez de invocar roles manualmente, definir un pipeline que ejecute el workflow automaticamente (requiere motor de tool-use propio).
6. **Plugins** -- permitir que usuarios extiendan lazyharness con sus propios comandos.

La base esta. Cada feature nueva es un modulo que se integra con la arquitectura que ya construiste: dominio puro en `internal/domain/`, adapters en `internal/storage/` y `internal/runtime/`, presentacion en `internal/tui/`. La separacion de concerns que aprendiste en este curso te va a permitir agregar cualquiera de estas features sin reescribir lo existente.

Anda, compila tu binario y empeza a usarlo.
