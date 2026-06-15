# Clase 08: Tareas y workspace — el panel operativo

> Al terminar esta clase, al presionar `t` en la vista de harness se abre la pantalla de workspace con el panel de tareas. Podes crear, editar, marcar como completas y eliminar tareas. Todo se persiste en `tareas.json` con versionado git.

## Prerequisitos

- [Clase 03](./03_filesystem_y_json.md): lectura/escritura de JSON, storage filesystem.
- [Clase 04](./04_vista_de_harness.md): composicion de paneles, routing entre vistas.
- [Clase 05](./05_git_desde_go.md): commit automatico con go-git.

## Conceptos de Go que vas a aprender

### 1. time.Time y el formato unico de Go

Go tiene el sistema de formato de tiempo mas peculiar de todos los lenguajes. En vez de `%Y-%m-%d %H:%M:%S` (como Python), Go usa una **fecha de referencia** que tenes que memorizar:

```
Mon Jan 2 15:04:05 MST 2006
```

Cada componente es un numero unico:
- **1** = mes (January es el mes 1)
- **2** = dia
- **3** = hora (PM, 12h) -- o **15** para 24h
- **4** = minutos
- **5** = segundos
- **6** = ano (2006, los ultimos dos: 06)

```go
import "time"

now := time.Now()

// Formatear
now.Format("2006-01-02")                    // "2026-06-15"
now.Format("2006-01-02 15:04:05")           // "2026-06-15 14:30:45"
now.Format("02/01/2006")                    // "15/06/2026" (dia/mes/ano)
now.Format("Monday, January 2")             // "Sunday, June 15"
now.Format(time.RFC3339)                    // "2026-06-15T14:30:45-03:00"

// Parsear
t, err := time.Parse("2006-01-02", "2026-06-15")
t, err := time.Parse(time.RFC3339, "2026-06-15T14:30:45-03:00")
```

**Equivalente Python:**

```python
from datetime import datetime

now = datetime.now()

# Formatear
now.strftime("%Y-%m-%d")                    # "2026-06-15"
now.strftime("%Y-%m-%d %H:%M:%S")           # "2026-06-15 14:30:45"

# Parsear
datetime.strptime("2026-06-15", "%Y-%m-%d")
```

**Tradeoff Go vs Python en formato de tiempo:**

| Aspecto | Go (fecha de referencia) | Python (strftime) |
|---------|--------------------------|-------------------|
| Memorizacion | Necesitas saber que "1=mes, 2=dia, 15=hora" | Necesitas saber que "%Y=ano, %m=mes" |
| Legibilidad | El formato se parece al output: `"2006-01-02"` | Los codigos son abstractos: `"%Y-%m-%d"` |
| Errores tipicos | Poner 1 donde va 2 (confundir mes con dia) | Olvidar el `%` o confundir `%m` con `%M` |

**Tip mnemotecnico:** la fecha de referencia de Go, en orden, es `01/02 03:04:05 PM '06 -0700`. O sea: mes 01, dia 02, hora 03, minuto 04, segundo 05, ano 06, timezone 07. Es una secuencia 1-2-3-4-5-6-7.

**El zero value de time.Time es peligroso:**

```go
var t time.Time
fmt.Println(t)            // 0001-01-01 00:00:00 +0000 UTC
fmt.Println(t.IsZero())   // true

// Cuidado: el zero value es una fecha valida pero sin sentido
if t.IsZero() {
    // manejar el caso de "no hay fecha"
}
```

```python
# En Python, el equivalente seria None — mas explicito
t: datetime | None = None
if t is None:
    # manejar
```

### 2. iota para enums — el TaskStatus

Go no tiene enums nativos. En su lugar usa `const` + `iota`, un auto-incremento:

```go
type TaskStatus int

const (
    TaskPending    TaskStatus = iota // 0
    TaskInProgress                   // 1
    TaskDone                         // 2
)
```

Pero `iota` tiene un problema: si serializas a JSON, obtienes numeros:

```json
{"status": 1}
```

Eso es ilegible. Queremos:

```json
{"status": "en_curso"}
```

La solucion es implementar un metodo `String()` y custom JSON marshaling:

```go
// String convierte el status a texto legible
func (s TaskStatus) String() string {
    switch s {
    case TaskPending:
        return "pendiente"
    case TaskInProgress:
        return "en_curso"
    case TaskDone:
        return "hecha"
    default:
        return "desconocido"
    }
}

// ParseTaskStatus convierte texto a TaskStatus
func ParseTaskStatus(s string) (TaskStatus, error) {
    switch s {
    case "pendiente":
        return TaskPending, nil
    case "en_curso":
        return TaskInProgress, nil
    case "hecha":
        return TaskDone, nil
    default:
        return TaskPending, fmt.Errorf("status desconocido: %s", s)
    }
}
```

**Equivalente Python:**

```python
from enum import Enum

class TaskStatus(str, Enum):
    PENDING = "pendiente"
    IN_PROGRESS = "en_curso"
    DONE = "hecha"

# Python serializa automaticamente por el valor del string
import json
json.dumps({"status": TaskStatus.PENDING.value})
# '{"status": "pendiente"}'
```

**Tradeoff iota + String() vs string constants:**

| Enfoque | Ventaja | Desventaja |
|---------|---------|------------|
| `iota` + `String()` | Type safety: `TaskStatus` es un tipo distinto a `int` | Mas boilerplate (String, MarshalJSON, ParseStatus) |
| `type TaskStatus string` con constantes | JSON natural, sin marshaler custom | Cualquier string se puede asignar — menos seguro |

```go
// Alternativa con string constants:
type TaskStatus string

const (
    TaskPending    TaskStatus = "pendiente"
    TaskInProgress TaskStatus = "en_curso"
    TaskDone       TaskStatus = "hecha"
)
// No necesita MarshalJSON — el string se serializa directamente.
// Pero alguien podria hacer: status = TaskStatus("inventado") sin error.
```

**Decision:** para lazyharness usamos la variante con string constants. Es mas simple, el JSON es legible por defecto, y con solo 3 valores el riesgo de asignar un string invalido es bajo. Si tuvieramos 20 estados, iriamos con `iota` + validacion.

### 3. Custom JSON marshaling/unmarshaling

Cuando el formato default de Go no sirve, implementas las interfaces `json.Marshaler` y `json.Unmarshaler`:

```go
import "encoding/json"

// MarshalJSON convierte TaskStatus a JSON como string
func (s TaskStatus) MarshalJSON() ([]byte, error) {
    return json.Marshal(s.String())
}

// UnmarshalJSON parsea un string JSON a TaskStatus
func (s *TaskStatus) UnmarshalJSON(data []byte) error {
    var str string
    if err := json.Unmarshal(data, &str); err != nil {
        return err
    }
    parsed, err := ParseTaskStatus(str)
    if err != nil {
        return err
    }
    *s = parsed
    return nil
}
```

**Fijate el pattern:** `MarshalJSON` recibe el valor (no puntero), `UnmarshalJSON` recibe puntero (necesita modificar el receptor). Esto es una convencion de Go para metodos que mutan vs los que no.

**Otro caso comun: time.Time en JSON.** Por default, Go serializa `time.Time` como RFC3339:

```json
{"created_at": "2026-06-15T14:30:45.123Z"}
```

Esto esta bien para nuestro caso. Pero si quisieramos otro formato (por ejemplo, solo fecha):

```go
type DateOnly struct {
    time.Time
}

func (d DateOnly) MarshalJSON() ([]byte, error) {
    return json.Marshal(d.Format("2006-01-02"))
}

func (d *DateOnly) UnmarshalJSON(data []byte) error {
    var s string
    if err := json.Unmarshal(data, &s); err != nil {
        return err
    }
    t, err := time.Parse("2006-01-02", s)
    if err != nil {
        return err
    }
    d.Time = t
    return nil
}
```

**Equivalente Python:**

```python
import json
from datetime import datetime

class TaskEncoder(json.JSONEncoder):
    def default(self, obj):
        if isinstance(obj, datetime):
            return obj.strftime("%Y-%m-%d")
        return super().default(obj)

json.dumps(data, cls=TaskEncoder)
```

### 4. sort.Slice — ordenar con funcion custom

Go tiene `sort.Slice` que ordena un slice in-place con una funcion de comparacion:

```go
import "sort"

// Ordenar tareas por fecha, las mas recientes primero
sort.Slice(tasks, func(i, j int) bool {
    return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
})

// Ordenar por status (pendientes primero, hechas al final)
sort.Slice(tasks, func(i, j int) bool {
    return tasks[i].Status < tasks[j].Status
})

// Ordenar por multiples criterios: primero status, despues fecha
sort.Slice(tasks, func(i, j int) bool {
    if tasks[i].Status != tasks[j].Status {
        return tasks[i].Status < tasks[j].Status
    }
    return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
})
```

**Equivalente Python:**

```python
# Python es mas conciso con key functions
tasks.sort(key=lambda t: t.created_at, reverse=True)

# Multiples criterios
tasks.sort(key=lambda t: (t.status.value, -t.created_at.timestamp()))
```

**Tradeoff Go vs Python en sorting:**

Go es mas verboso pero explicito — ves exactamente la logica de comparacion. Python es mas conciso con `key=lambda` pero el truco del `-timestamp()` para invertir es menos obvio.

**sort.Slice es inestable.** Si necesitas que elementos iguales mantengan su orden original, usa `sort.SliceStable`:

```go
// Estable: mantiene el orden original entre elementos iguales
sort.SliceStable(tasks, func(i, j int) bool {
    return tasks[i].Status < tasks[j].Status
})
```

### 5. Generando IDs unicos

Las tareas necesitan un ID unico. Opciones:

```go
// Opcion 1: UUID (necesita dependencia)
import "github.com/google/uuid"
id := uuid.New().String() // "550e8400-e29b-41d4-a716-446655440000"

// Opcion 2: timestamp + random (sin dependencia)
import (
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "time"
)

func generateID() string {
    b := make([]byte, 4)
    rand.Read(b)
    return fmt.Sprintf("%d-%s", time.Now().UnixMilli(), hex.EncodeToString(b))
    // "1718451045123-a1b2c3d4"
}

// Opcion 3: auto-incremento basado en el maximo actual
func nextID(tasks []Task) int {
    max := 0
    for _, t := range tasks {
        if t.ID > max {
            max = t.ID
        }
    }
    return max + 1
}
```

**Decision:** usamos auto-incremento con `int`. Es simple, legible en el JSON, y para un archivo local con decenas de tareas no necesitamos UUIDs.

## Lo que vamos a construir

La pantalla de workspace del mockup 04 (panel de tareas):

```
+-- dev-flow -- workspace -----------------------------------------------+
|                                                                         |
| +-- Tareas -------------------+ +-- Detalle tarea -------------------+ |
| |                              | |                                    | |
| | > #1 [pendiente] Refactori- | | #1 Refactorizar modulo de auth    | |
| |     zar modulo de auth       | |                                    | |
| |   #2 [en_curso] Implementar | | Rol: arquitecto                   | |
| |     tests de integracion     | | Estado: pendiente                 | |
| |   #3 [hecha] Configurar CI  | | Creada: 2026-06-14                | |
| |                              | |                                    | |
| |                              | | Informe:                          | |
| |                              | | Se necesita separar la logica     | |
| |                              | | de autenticacion del controlador  | |
| |                              | | principal para mejorar...         | |
| |                              | |                                    | |
| +------------------------------+ +------------------------------------+ |
|                                                                         |
+-------------------------------------------------------------------------+
|  n nueva  e editar  d marcar hecha  x eliminar  esc volver              |
+-------------------------------------------------------------------------+
```

**Archivos a crear:**
- `internal/domain/task.go`
- `internal/domain/task_test.go`
- `internal/storage/tasks.go`
- `internal/storage/tasks_test.go`
- `internal/tui/workspace/model.go`
- `internal/tui/workspace/model_test.go`

**Archivos a modificar:**
- `internal/tui/app.go` — agregar routing a screenWorkspace
- `internal/tui/keys.go` — agregar WorkspaceKeys

## Implementacion paso a paso

### Paso 1: Definir el struct Task en el dominio

Crea `internal/domain/task.go`:

```go
package domain

import (
    "fmt"
    "time"
)

// TaskStatus representa el estado de una tarea
type TaskStatus string

const (
    TaskPending    TaskStatus = "pendiente"
    TaskInProgress TaskStatus = "en_curso"
    TaskDone       TaskStatus = "hecha"
)

// ValidStatuses lista los estados validos para validacion
var ValidStatuses = []TaskStatus{TaskPending, TaskInProgress, TaskDone}

// IsValid verifica si un status es valido
func (s TaskStatus) IsValid() bool {
    for _, valid := range ValidStatuses {
        if s == valid {
            return true
        }
    }
    return false
}

// DisplayColor retorna el color para mostrar en la TUI
func (s TaskStatus) DisplayColor() string {
    switch s {
    case TaskPending:
        return "#ff9e64" // naranja
    case TaskInProgress:
        return "#7aa2f7" // azul
    case TaskDone:
        return "#9ece6a" // verde
    default:
        return "#565f89" // gris
    }
}

// Task representa una tarea del harness
type Task struct {
    ID        int        `json:"id"`
    Role      string     `json:"role"`
    Title     string     `json:"title"`
    Report    string     `json:"report,omitempty"`
    Status    TaskStatus `json:"status"`
    CreatedAt time.Time  `json:"created_at"`
    UpdatedAt time.Time  `json:"updated_at"`
}

// TaskList es el wrapper para serializar/deserializar el archivo completo
type TaskList struct {
    Tasks []Task `json:"tasks"`
}

// NewTask crea una tarea nueva con valores por defecto
func NewTask(id int, role, title string) (Task, error) {
    if title == "" {
        return Task{}, fmt.Errorf("el titulo no puede estar vacio")
    }
    if role == "" {
        return Task{}, fmt.Errorf("el rol no puede estar vacio")
    }

    now := time.Now()
    return Task{
        ID:        id,
        Role:      role,
        Title:     title,
        Status:    TaskPending,
        CreatedAt: now,
        UpdatedAt: now,
    }, nil
}

// NextStatus retorna el siguiente estado en el ciclo:
// pendiente -> en_curso -> hecha -> pendiente
func (t Task) NextStatus() TaskStatus {
    switch t.Status {
    case TaskPending:
        return TaskInProgress
    case TaskInProgress:
        return TaskDone
    case TaskDone:
        return TaskPending
    default:
        return TaskPending
    }
}

// FormatCreatedAt retorna la fecha de creacion formateada
func (t Task) FormatCreatedAt() string {
    if t.CreatedAt.IsZero() {
        return "sin fecha"
    }
    return t.CreatedAt.Format("2006-01-02 15:04")
}

// FormatUpdatedAt retorna la fecha de actualizacion formateada
func (t Task) FormatUpdatedAt() string {
    if t.UpdatedAt.IsZero() {
        return "sin fecha"
    }
    return t.UpdatedAt.Format("2006-01-02 15:04")
}
```

### Paso 2: Tests del dominio de tareas

Crea `internal/domain/task_test.go`:

```go
package domain

import (
    "testing"
    "time"
)

func TestNewTask(t *testing.T) {
    task, err := NewTask(1, "arquitecto", "Refactorizar modulo de auth")
    if err != nil {
        t.Fatalf("error creando tarea: %v", err)
    }

    if task.ID != 1 {
        t.Errorf("ID esperado 1, obtuve %d", task.ID)
    }
    if task.Role != "arquitecto" {
        t.Errorf("rol esperado 'arquitecto', obtuve '%s'", task.Role)
    }
    if task.Status != TaskPending {
        t.Errorf("status esperado 'pendiente', obtuve '%s'", task.Status)
    }
    if task.CreatedAt.IsZero() {
        t.Error("created_at no deberia ser zero")
    }
}

func TestNewTask_Validacion(t *testing.T) {
    tests := []struct {
        name  string
        role  string
        title string
    }{
        {"titulo vacio", "arquitecto", ""},
        {"rol vacio", "", "Tarea importante"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := NewTask(1, tt.role, tt.title)
            if err == nil {
                t.Error("esperaba error de validacion")
            }
        })
    }
}

func TestTaskStatus_IsValid(t *testing.T) {
    tests := []struct {
        status TaskStatus
        valid  bool
    }{
        {TaskPending, true},
        {TaskInProgress, true},
        {TaskDone, true},
        {TaskStatus("inventado"), false},
        {TaskStatus(""), false},
    }

    for _, tt := range tests {
        t.Run(string(tt.status), func(t *testing.T) {
            if got := tt.status.IsValid(); got != tt.valid {
                t.Errorf("IsValid() = %v, esperaba %v", got, tt.valid)
            }
        })
    }
}

func TestTaskNextStatus(t *testing.T) {
    tests := []struct {
        current  TaskStatus
        expected TaskStatus
    }{
        {TaskPending, TaskInProgress},
        {TaskInProgress, TaskDone},
        {TaskDone, TaskPending},
    }

    for _, tt := range tests {
        t.Run(string(tt.current), func(t *testing.T) {
            task := Task{Status: tt.current}
            got := task.NextStatus()
            if got != tt.expected {
                t.Errorf("NextStatus() de %s = %s, esperaba %s", tt.current, got, tt.expected)
            }
        })
    }
}

func TestTaskFormatDates(t *testing.T) {
    task := Task{
        CreatedAt: time.Date(2026, 6, 15, 14, 30, 0, 0, time.UTC),
        UpdatedAt: time.Date(2026, 6, 15, 16, 45, 0, 0, time.UTC),
    }

    if got := task.FormatCreatedAt(); got != "2026-06-15 14:30" {
        t.Errorf("FormatCreatedAt() = %q, esperaba '2026-06-15 14:30'", got)
    }

    // Zero value
    empty := Task{}
    if got := empty.FormatCreatedAt(); got != "sin fecha" {
        t.Errorf("FormatCreatedAt() con zero time = %q, esperaba 'sin fecha'", got)
    }
}
```

```bash
go test ./internal/domain/ -v -run "TestTask|TestNewTask"
```

### Paso 3: CRUD de tareas en storage

Crea `internal/storage/tasks.go`:

```go
package storage

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "time"

    "github.com/Gerardo1909/lazyharness/internal/domain"
)

const tasksFile = "tareas.json"

// LoadTasks lee las tareas del archivo tareas.json
func (s Store) LoadTasks(projectDir string) ([]domain.Task, error) {
    path := filepath.Join(projectDir, harnessDir, tasksFile)
    data, err := os.ReadFile(path)
    if os.IsNotExist(err) {
        return []domain.Task{}, nil // no hay tareas todavia
    }
    if err != nil {
        return nil, fmt.Errorf("leyendo tareas: %w", err)
    }

    var taskList domain.TaskList
    if err := json.Unmarshal(data, &taskList); err != nil {
        return nil, fmt.Errorf("parseando tareas: %w", err)
    }

    return taskList.Tasks, nil
}

// SaveTasks guarda las tareas al archivo tareas.json
func (s Store) SaveTasks(projectDir string, tasks []domain.Task) error {
    path := filepath.Join(projectDir, harnessDir, tasksFile)

    // Ordenar antes de guardar: pendientes primero, despues en_curso, despues hechas
    // Dentro de cada grupo, las mas recientes primero
    sorted := make([]domain.Task, len(tasks))
    copy(sorted, tasks)

    sort.SliceStable(sorted, func(i, j int) bool {
        if sorted[i].Status != sorted[j].Status {
            return statusOrder(sorted[i].Status) < statusOrder(sorted[j].Status)
        }
        return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
    })

    taskList := domain.TaskList{Tasks: sorted}
    data, err := json.MarshalIndent(taskList, "", "  ")
    if err != nil {
        return fmt.Errorf("serializando tareas: %w", err)
    }

    if err := os.WriteFile(path, data, 0644); err != nil {
        return fmt.Errorf("escribiendo tareas: %w", err)
    }

    return nil
}

// AddTask agrega una tarea nueva y la guarda
func (s Store) AddTask(projectDir, role, title string) (domain.Task, error) {
    tasks, err := s.LoadTasks(projectDir)
    if err != nil {
        return domain.Task{}, err
    }

    // Generar siguiente ID
    nextID := 1
    for _, t := range tasks {
        if t.ID >= nextID {
            nextID = t.ID + 1
        }
    }

    task, err := domain.NewTask(nextID, role, title)
    if err != nil {
        return domain.Task{}, err
    }

    tasks = append(tasks, task)
    if err := s.SaveTasks(projectDir, tasks); err != nil {
        return domain.Task{}, err
    }

    return task, nil
}

// UpdateTaskStatus cambia el estado de una tarea
func (s Store) UpdateTaskStatus(projectDir string, taskID int, newStatus domain.TaskStatus) error {
    tasks, err := s.LoadTasks(projectDir)
    if err != nil {
        return err
    }

    found := false
    for i := range tasks {
        if tasks[i].ID == taskID {
            tasks[i].Status = newStatus
            tasks[i].UpdatedAt = time.Now()
            found = true
            break
        }
    }

    if !found {
        return fmt.Errorf("tarea #%d no encontrada", taskID)
    }

    return s.SaveTasks(projectDir, tasks)
}

// UpdateTaskReport actualiza el informe de una tarea
func (s Store) UpdateTaskReport(projectDir string, taskID int, report string) error {
    tasks, err := s.LoadTasks(projectDir)
    if err != nil {
        return err
    }

    found := false
    for i := range tasks {
        if tasks[i].ID == taskID {
            tasks[i].Report = report
            tasks[i].UpdatedAt = time.Now()
            found = true
            break
        }
    }

    if !found {
        return fmt.Errorf("tarea #%d no encontrada", taskID)
    }

    return s.SaveTasks(projectDir, tasks)
}

// DeleteTask elimina una tarea por ID
func (s Store) DeleteTask(projectDir string, taskID int) error {
    tasks, err := s.LoadTasks(projectDir)
    if err != nil {
        return err
    }

    filtered := make([]domain.Task, 0, len(tasks))
    found := false
    for _, t := range tasks {
        if t.ID == taskID {
            found = true
            continue
        }
        filtered = append(filtered, t)
    }

    if !found {
        return fmt.Errorf("tarea #%d no encontrada", taskID)
    }

    return s.SaveTasks(projectDir, filtered)
}

// FilterTasksByRole retorna solo las tareas de un rol especifico
func (s Store) FilterTasksByRole(projectDir, role string) ([]domain.Task, error) {
    tasks, err := s.LoadTasks(projectDir)
    if err != nil {
        return nil, err
    }

    var filtered []domain.Task
    for _, t := range tasks {
        if t.Role == role {
            filtered = append(filtered, t)
        }
    }

    return filtered, nil
}

// statusOrder retorna el orden de prioridad para sorting
func statusOrder(s domain.TaskStatus) int {
    switch s {
    case domain.TaskPending:
        return 0
    case domain.TaskInProgress:
        return 1
    case domain.TaskDone:
        return 2
    default:
        return 3
    }
}
```

### Paso 4: Tests del storage de tareas

Crea `internal/storage/tasks_test.go`:

```go
package storage

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/Gerardo1909/lazyharness/internal/domain"
)

func setupTasksTest(t *testing.T) (Store, string) {
    t.Helper()
    configDir := t.TempDir()
    projectDir := t.TempDir()

    // Crear la estructura .lazyharness/
    os.MkdirAll(filepath.Join(projectDir, ".lazyharness"), 0755)

    return NewStoreWithDir(configDir), projectDir
}

func TestLoadTasks_ArchivoNoExiste(t *testing.T) {
    store, projectDir := setupTasksTest(t)

    tasks, err := store.LoadTasks(projectDir)
    if err != nil {
        t.Fatalf("error inesperado: %v", err)
    }
    if len(tasks) != 0 {
        t.Errorf("esperaba 0 tareas, obtuve %d", len(tasks))
    }
}

func TestAddTask(t *testing.T) {
    store, projectDir := setupTasksTest(t)

    // Agregar primera tarea
    task, err := store.AddTask(projectDir, "arquitecto", "Refactorizar auth")
    if err != nil {
        t.Fatalf("error agregando tarea: %v", err)
    }
    if task.ID != 1 {
        t.Errorf("primer ID deberia ser 1, obtuve %d", task.ID)
    }
    if task.Status != domain.TaskPending {
        t.Errorf("status inicial deberia ser pendiente, obtuve %s", task.Status)
    }

    // Agregar segunda tarea
    task2, err := store.AddTask(projectDir, "reviewer", "Revisar PR #42")
    if err != nil {
        t.Fatalf("error agregando segunda tarea: %v", err)
    }
    if task2.ID != 2 {
        t.Errorf("segundo ID deberia ser 2, obtuve %d", task2.ID)
    }

    // Verificar que se guardaron
    tasks, _ := store.LoadTasks(projectDir)
    if len(tasks) != 2 {
        t.Fatalf("esperaba 2 tareas, obtuve %d", len(tasks))
    }
}

func TestUpdateTaskStatus(t *testing.T) {
    store, projectDir := setupTasksTest(t)

    store.AddTask(projectDir, "arquitecto", "Tarea de prueba")

    // Cambiar estado
    err := store.UpdateTaskStatus(projectDir, 1, domain.TaskInProgress)
    if err != nil {
        t.Fatalf("error actualizando status: %v", err)
    }

    // Verificar
    tasks, _ := store.LoadTasks(projectDir)
    if tasks[0].Status != domain.TaskInProgress {
        t.Errorf("status esperado en_curso, obtuve %s", tasks[0].Status)
    }

    // Tarea que no existe
    err = store.UpdateTaskStatus(projectDir, 999, domain.TaskDone)
    if err == nil {
        t.Error("esperaba error al actualizar tarea inexistente")
    }
}

func TestDeleteTask(t *testing.T) {
    store, projectDir := setupTasksTest(t)

    store.AddTask(projectDir, "arquitecto", "Tarea 1")
    store.AddTask(projectDir, "reviewer", "Tarea 2")
    store.AddTask(projectDir, "dev", "Tarea 3")

    // Eliminar la del medio
    err := store.DeleteTask(projectDir, 2)
    if err != nil {
        t.Fatalf("error eliminando: %v", err)
    }

    tasks, _ := store.LoadTasks(projectDir)
    if len(tasks) != 2 {
        t.Fatalf("esperaba 2 tareas despues de eliminar, obtuve %d", len(tasks))
    }

    // Verificar que la tarea 2 no esta
    for _, task := range tasks {
        if task.ID == 2 {
            t.Error("la tarea 2 deberia haber sido eliminada")
        }
    }

    // Eliminar tarea inexistente
    err = store.DeleteTask(projectDir, 999)
    if err == nil {
        t.Error("esperaba error al eliminar tarea inexistente")
    }
}

func TestFilterTasksByRole(t *testing.T) {
    store, projectDir := setupTasksTest(t)

    store.AddTask(projectDir, "arquitecto", "Tarea de arquitecto 1")
    store.AddTask(projectDir, "reviewer", "Tarea de reviewer")
    store.AddTask(projectDir, "arquitecto", "Tarea de arquitecto 2")

    filtered, err := store.FilterTasksByRole(projectDir, "arquitecto")
    if err != nil {
        t.Fatalf("error filtrando: %v", err)
    }
    if len(filtered) != 2 {
        t.Errorf("esperaba 2 tareas de arquitecto, obtuve %d", len(filtered))
    }

    // Rol sin tareas
    empty, _ := store.FilterTasksByRole(projectDir, "dev")
    if len(empty) != 0 {
        t.Errorf("esperaba 0 tareas de dev, obtuve %d", len(empty))
    }
}

func TestSaveTasks_OrdenaPorStatusYFecha(t *testing.T) {
    store, projectDir := setupTasksTest(t)

    // Crear tareas en orden mezclado
    store.AddTask(projectDir, "dev", "Tarea hecha")
    store.UpdateTaskStatus(projectDir, 1, domain.TaskDone)

    store.AddTask(projectDir, "dev", "Tarea pendiente")

    store.AddTask(projectDir, "dev", "Tarea en curso")
    store.UpdateTaskStatus(projectDir, 3, domain.TaskInProgress)

    // Cargar y verificar orden
    tasks, _ := store.LoadTasks(projectDir)

    if tasks[0].Status != domain.TaskPending {
        t.Errorf("primera tarea deberia ser pendiente, es %s", tasks[0].Status)
    }
    if tasks[1].Status != domain.TaskInProgress {
        t.Errorf("segunda tarea deberia ser en_curso, es %s", tasks[1].Status)
    }
    if tasks[2].Status != domain.TaskDone {
        t.Errorf("tercera tarea deberia ser hecha, es %s", tasks[2].Status)
    }
}

func TestTasksJSON_EsLegible(t *testing.T) {
    store, projectDir := setupTasksTest(t)

    store.AddTask(projectDir, "arquitecto", "Disenar API")
    store.UpdateTaskStatus(projectDir, 1, domain.TaskInProgress)

    // Leer el archivo raw para verificar que es legible
    path := filepath.Join(projectDir, ".lazyharness", "tareas.json")
    data, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("error leyendo archivo: %v", err)
    }

    content := string(data)

    // Verificar que usa strings legibles, no numeros
    if !strings.Contains(content, `"en_curso"`) {
        t.Error("el JSON deberia contener 'en_curso' como string, no como numero")
    }
    if !strings.Contains(content, `"arquitecto"`) {
        t.Error("el JSON deberia contener el nombre del rol")
    }
}
```

```bash
go test ./internal/storage/ -v -run "TestTask|TestLoad|TestAdd|TestDelete|TestFilter|TestSave"
```

### Paso 5: Agregar keybindings del workspace

Agrega a `internal/tui/keys.go`:

```go
// WorkspaceKeys son atajos de la pantalla de workspace
type WorkspaceKeys struct {
    Up     key.Binding
    Down   key.Binding
    New    key.Binding
    Edit   key.Binding
    Toggle key.Binding
    Delete key.Binding
    Back   key.Binding
}

var WorkspaceKeyMap = WorkspaceKeys{
    Up: key.NewBinding(
        key.WithKeys("up", "k"),
        key.WithHelp("k", "subir"),
    ),
    Down: key.NewBinding(
        key.WithKeys("down", "j"),
        key.WithHelp("j", "bajar"),
    ),
    New: key.NewBinding(
        key.WithKeys("n"),
        key.WithHelp("n", "nueva"),
    ),
    Edit: key.NewBinding(
        key.WithKeys("e"),
        key.WithHelp("e", "editar"),
    ),
    Toggle: key.NewBinding(
        key.WithKeys("d"),
        key.WithHelp("d", "cambiar estado"),
    ),
    Delete: key.NewBinding(
        key.WithKeys("x"),
        key.WithHelp("x", "eliminar"),
    ),
    Back: key.NewBinding(
        key.WithKeys("esc"),
        key.WithHelp("esc", "volver"),
    ),
}
```

### Paso 6: Crear el modelo del workspace

Crea `internal/tui/workspace/model.go`:

```go
package workspace

import (
    "fmt"
    "strings"

    "github.com/charmbracelet/bubbles/key"
    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    maintui "github.com/Gerardo1909/lazyharness/internal/tui"
    "github.com/Gerardo1909/lazyharness/internal/domain"
)

// --- Mensajes ---

type BackToHarnessMsg struct{}

type tasksLoadedMsg struct {
    tasks []domain.Task
    err   error
}

type taskSavedMsg struct {
    err error
}

// --- Funciones inyectadas ---

type LoadTasksFunc func() ([]domain.Task, error)
type AddTaskFunc func(role, title string) (domain.Task, error)
type UpdateStatusFunc func(id int, status domain.TaskStatus) error
type DeleteTaskFunc func(id int) error

// --- Input mode ---

type inputMode int

const (
    inputNone inputMode = iota
    inputNewTask
)

// --- Modelo ---

type Model struct {
    tasks       []domain.Task
    roles       []string // nombres de roles disponibles
    selected    int
    statusMsg   string
    inputMode   inputMode
    titleInput  textinput.Model
    selectedRole int // indice del rol para nueva tarea
    width       int
    height      int

    loadFn   LoadTasksFunc
    addFn    AddTaskFunc
    updateFn UpdateStatusFunc
    deleteFn DeleteTaskFunc
}

func NewModel(
    roles []string,
    width, height int,
    loadFn LoadTasksFunc,
    addFn AddTaskFunc,
    updateFn UpdateStatusFunc,
    deleteFn DeleteTaskFunc,
) Model {
    ti := textinput.New()
    ti.Placeholder = "Titulo de la tarea..."
    ti.CharLimit = 200

    return Model{
        roles:    roles,
        selected: 0,
        width:    width,
        height:   height,
        titleInput: ti,
        loadFn:   loadFn,
        addFn:    addFn,
        updateFn: updateFn,
        deleteFn: deleteFn,
    }
}

func (m Model) Init() tea.Cmd {
    return func() tea.Msg {
        tasks, err := m.loadFn()
        return tasksLoadedMsg{tasks: tasks, err: err}
    }
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    // Si estamos en modo input, delegar al input
    if m.inputMode != inputNone {
        return m.updateInput(msg)
    }

    switch msg := msg.(type) {
    case tasksLoadedMsg:
        if msg.err != nil {
            m.statusMsg = "Error cargando tareas: " + msg.err.Error()
            return m, nil
        }
        m.tasks = msg.tasks

    case taskSavedMsg:
        if msg.err != nil {
            m.statusMsg = "Error: " + msg.err.Error()
            return m, nil
        }
        // Recargar tareas
        return m, func() tea.Msg {
            tasks, err := m.loadFn()
            return tasksLoadedMsg{tasks: tasks, err: err}
        }

    case tea.KeyMsg:
        switch {
        case key.Matches(msg, maintui.WorkspaceKeyMap.Back):
            return m, func() tea.Msg { return BackToHarnessMsg{} }

        case key.Matches(msg, maintui.WorkspaceKeyMap.Up):
            if m.selected > 0 {
                m.selected--
            }

        case key.Matches(msg, maintui.WorkspaceKeyMap.Down):
            if m.selected < len(m.tasks)-1 {
                m.selected++
            }

        case key.Matches(msg, maintui.WorkspaceKeyMap.New):
            m.inputMode = inputNewTask
            m.titleInput.Reset()
            m.titleInput.Focus()
            m.selectedRole = 0
            return m, textinput.Blink

        case key.Matches(msg, maintui.WorkspaceKeyMap.Toggle):
            if len(m.tasks) > 0 {
                task := m.tasks[m.selected]
                newStatus := task.NextStatus()
                return m, func() tea.Msg {
                    err := m.updateFn(task.ID, newStatus)
                    return taskSavedMsg{err: err}
                }
            }

        case key.Matches(msg, maintui.WorkspaceKeyMap.Delete):
            if len(m.tasks) > 0 {
                task := m.tasks[m.selected]
                return m, func() tea.Msg {
                    err := m.deleteFn(task.ID)
                    return taskSavedMsg{err: err}
                }
            }
        }
    }

    return m, nil
}

func (m Model) updateInput(msg tea.Msg) (Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.Type {
        case tea.KeyEscape:
            m.inputMode = inputNone
            return m, nil

        case tea.KeyEnter:
            title := m.titleInput.Value()
            if title == "" {
                m.statusMsg = "El titulo no puede estar vacio"
                return m, nil
            }
            role := ""
            if len(m.roles) > 0 {
                role = m.roles[m.selectedRole]
            }
            m.inputMode = inputNone
            return m, func() tea.Msg {
                _, err := m.addFn(role, title)
                return taskSavedMsg{err: err}
            }

        case tea.KeyTab:
            // Ciclar entre roles
            if len(m.roles) > 0 {
                m.selectedRole = (m.selectedRole + 1) % len(m.roles)
            }
            return m, nil
        }
    }

    // Delegar al textinput
    var cmd tea.Cmd
    m.titleInput, cmd = m.titleInput.Update(msg)
    return m, cmd
}

func (m Model) View() string {
    // Header
    headerStyle := lipgloss.NewStyle().Bold(true).Foreground(maintui.ColorFg)
    header := headerStyle.Render("Workspace — Tareas")

    if len(m.tasks) == 0 && m.inputMode == inputNone {
        emptyMsg := maintui.StyleComment().Render(
            "No hay tareas todavia. Presiona 'n' para crear una.",
        )
        keybar := m.renderKeybar()
        return lipgloss.JoinVertical(lipgloss.Left,
            header, "", emptyMsg, "", keybar,
        )
    }

    // Layout de dos paneles
    listWidth := m.width * 40 / 100
    if listWidth < 30 {
        listWidth = 30
    }
    detailWidth := m.width - listWidth - 3

    // Panel izquierdo: lista de tareas
    taskList := m.renderTaskList(listWidth)

    // Panel derecho: detalle de la tarea seleccionada
    taskDetail := m.renderTaskDetail(detailWidth)

    content := lipgloss.JoinHorizontal(lipgloss.Top,
        taskList,
        " ",
        taskDetail,
    )

    // Input de nueva tarea (si esta activo)
    inputBar := ""
    if m.inputMode == inputNewTask {
        roleName := "(sin rol)"
        if len(m.roles) > 0 {
            roleName = m.roles[m.selectedRole]
        }
        inputBar = fmt.Sprintf(
            "\n  Rol: %s (tab para cambiar)  |  %s  |  enter confirmar  esc cancelar",
            lipgloss.NewStyle().Bold(true).Foreground(maintui.ColorBlue).Render(roleName),
            m.titleInput.View(),
        )
    }

    // Status
    status := ""
    if m.statusMsg != "" {
        status = maintui.StyleComment().Italic(true).Render(m.statusMsg)
    }

    // Keybar
    keybar := m.renderKeybar()

    return lipgloss.JoinVertical(lipgloss.Left,
        header,
        "",
        content,
        inputBar,
        "",
        status,
        keybar,
    )
}

func (m Model) renderTaskList(width int) string {
    var b strings.Builder

    titleStyle := lipgloss.NewStyle().Bold(true).Foreground(maintui.ColorFg)
    b.WriteString(titleStyle.Render("Tareas") + "\n\n")

    for i, task := range m.tasks {
        prefix := "  "
        if i == m.selected {
            prefix = "> "
        }

        statusColor := lipgloss.Color(task.Status.DisplayColor())
        statusStyle := lipgloss.NewStyle().
            Foreground(statusColor).
            Bold(true)

        idStyle := lipgloss.NewStyle().Foreground(maintui.ColorComment)
        titleStyle := lipgloss.NewStyle().Foreground(maintui.ColorFg)

        if i == m.selected {
            titleStyle = titleStyle.Background(maintui.ColorSelection)
        }

        statusTag := statusStyle.Render(fmt.Sprintf("[%s]", task.Status))
        id := idStyle.Render(fmt.Sprintf("#%d", task.ID))
        title := titleStyle.Render(truncateStr(task.Title, width-25))

        line := fmt.Sprintf("%s%s %s %s", prefix, id, statusTag, title)
        b.WriteString(line + "\n")
    }

    return lipgloss.NewStyle().Width(width).Render(b.String())
}

func (m Model) renderTaskDetail(width int) string {
    if len(m.tasks) == 0 || m.selected >= len(m.tasks) {
        return maintui.StyleComment().Render("Selecciona una tarea")
    }

    task := m.tasks[m.selected]

    titleStyle := lipgloss.NewStyle().Bold(true).Foreground(maintui.ColorFg)
    labelStyle := lipgloss.NewStyle().Foreground(maintui.ColorComment)
    valueStyle := lipgloss.NewStyle().Foreground(maintui.ColorFg)
    statusColor := lipgloss.Color(task.Status.DisplayColor())
    statusStyle := lipgloss.NewStyle().Bold(true).Foreground(statusColor)

    var b strings.Builder
    b.WriteString(titleStyle.Render(fmt.Sprintf("#%d %s", task.ID, task.Title)) + "\n\n")
    b.WriteString(labelStyle.Render("Rol: ") + valueStyle.Render(task.Role) + "\n")
    b.WriteString(labelStyle.Render("Estado: ") + statusStyle.Render(string(task.Status)) + "\n")
    b.WriteString(labelStyle.Render("Creada: ") + valueStyle.Render(task.FormatCreatedAt()) + "\n")
    b.WriteString(labelStyle.Render("Actualizada: ") + valueStyle.Render(task.FormatUpdatedAt()) + "\n")

    if task.Report != "" {
        b.WriteString("\n" + labelStyle.Render("Informe:") + "\n")
        b.WriteString(valueStyle.Render(task.Report) + "\n")
    }

    return lipgloss.NewStyle().Width(width).Render(b.String())
}

func (m Model) renderKeybar() string {
    keyStyle := lipgloss.NewStyle().
        Bold(true).
        Foreground(maintui.ColorFg).
        Background(maintui.ColorSelection).
        Padding(0, 1)
    descStyle := lipgloss.NewStyle().
        Foreground(maintui.ColorComment).
        MarginRight(2)

    return keyStyle.Render("n") + " " + descStyle.Render("nueva") +
        keyStyle.Render("e") + " " + descStyle.Render("editar") +
        keyStyle.Render("d") + " " + descStyle.Render("cambiar estado") +
        keyStyle.Render("x") + " " + descStyle.Render("eliminar") +
        keyStyle.Render("esc") + " " + descStyle.Render("volver")
}

func truncateStr(s string, maxLen int) string {
    if maxLen <= 3 {
        return s
    }
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen-3] + "..."
}
```

### Paso 7: Tests del workspace

Crea `internal/tui/workspace/model_test.go`:

```go
package workspace

import (
    "testing"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/Gerardo1909/lazyharness/internal/domain"
)

func fakeTasks() []domain.Task {
    now := time.Now()
    return []domain.Task{
        {ID: 1, Role: "arquitecto", Title: "Refactorizar auth", Status: domain.TaskPending, CreatedAt: now},
        {ID: 2, Role: "reviewer", Title: "Revisar PR #42", Status: domain.TaskInProgress, CreatedAt: now},
        {ID: 3, Role: "dev", Title: "Configurar CI", Status: domain.TaskDone, CreatedAt: now},
    }
}

func fakeLoadTasks(tasks []domain.Task) LoadTasksFunc {
    return func() ([]domain.Task, error) {
        return tasks, nil
    }
}

func fakeAddTask() AddTaskFunc {
    return func(role, title string) (domain.Task, error) {
        return domain.Task{ID: 99, Role: role, Title: title, Status: domain.TaskPending}, nil
    }
}

func fakeUpdateStatus() UpdateStatusFunc {
    return func(id int, status domain.TaskStatus) error {
        return nil
    }
}

func fakeDeleteTask() DeleteTaskFunc {
    return func(id int) error {
        return nil
    }
}

func TestWorkspace_CargaTareas(t *testing.T) {
    tasks := fakeTasks()
    m := NewModel(
        []string{"arquitecto", "reviewer", "dev"},
        80, 24,
        fakeLoadTasks(tasks), fakeAddTask(), fakeUpdateStatus(), fakeDeleteTask(),
    )

    m, _ = m.Update(tasksLoadedMsg{tasks: tasks, err: nil})

    if len(m.tasks) != 3 {
        t.Errorf("esperaba 3 tareas, obtuve %d", len(m.tasks))
    }
}

func TestWorkspace_Navegacion(t *testing.T) {
    tasks := fakeTasks()
    m := NewModel(
        []string{"arquitecto"},
        80, 24,
        fakeLoadTasks(tasks), fakeAddTask(), fakeUpdateStatus(), fakeDeleteTask(),
    )
    m.tasks = tasks

    // Bajar
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
    if m.selected != 1 {
        t.Errorf("despues de j, selected deberia ser 1, es %d", m.selected)
    }

    // No pasar del final
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
    if m.selected != 2 {
        t.Errorf("no deberia pasar del ultimo, selected es %d", m.selected)
    }

    // Subir
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
    if m.selected != 1 {
        t.Errorf("despues de k, selected deberia ser 1, es %d", m.selected)
    }
}

func TestWorkspace_NuevaTareaAbreInput(t *testing.T) {
    m := NewModel(
        []string{"arquitecto", "reviewer"},
        80, 24,
        fakeLoadTasks(nil), fakeAddTask(), fakeUpdateStatus(), fakeDeleteTask(),
    )

    // Presionar n
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
    if m.inputMode != inputNewTask {
        t.Error("deberia abrir el input de nueva tarea")
    }
}

func TestWorkspace_CancelarInput(t *testing.T) {
    m := NewModel(
        []string{"arquitecto"},
        80, 24,
        fakeLoadTasks(nil), fakeAddTask(), fakeUpdateStatus(), fakeDeleteTask(),
    )

    m.inputMode = inputNewTask
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})

    if m.inputMode != inputNone {
        t.Error("esc deberia cancelar el input")
    }
}

func TestWorkspace_CambiarEstado(t *testing.T) {
    tasks := fakeTasks()
    m := NewModel(
        []string{"arquitecto"},
        80, 24,
        fakeLoadTasks(tasks), fakeAddTask(), fakeUpdateStatus(), fakeDeleteTask(),
    )
    m.tasks = tasks
    m.selected = 0 // pendiente

    // Presionar d para cambiar estado
    m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
    if cmd == nil {
        t.Fatal("deberia retornar un comando de actualizacion")
    }
}

func TestWorkspace_EliminarTarea(t *testing.T) {
    tasks := fakeTasks()
    m := NewModel(
        []string{"arquitecto"},
        80, 24,
        fakeLoadTasks(tasks), fakeAddTask(), fakeUpdateStatus(), fakeDeleteTask(),
    )
    m.tasks = tasks
    m.selected = 1

    // Presionar x para eliminar
    m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
    if cmd == nil {
        t.Fatal("deberia retornar un comando de eliminacion")
    }
}

func TestWorkspace_VistaMuestraTareas(t *testing.T) {
    tasks := fakeTasks()
    m := NewModel(
        []string{"arquitecto", "reviewer", "dev"},
        80, 24,
        fakeLoadTasks(tasks), fakeAddTask(), fakeUpdateStatus(), fakeDeleteTask(),
    )
    m.tasks = tasks

    view := m.View()

    if !strings.Contains(view, "Refactorizar auth") {
        t.Error("la vista deberia mostrar el titulo de la primera tarea")
    }
    if !strings.Contains(view, "pendiente") {
        t.Error("la vista deberia mostrar el status")
    }
}
```

```bash
go test ./internal/tui/workspace/ -v
```

## Tradeoffs y decisiones de diseno

### Decision 1: JSON como base de datos de tareas

`tareas.json` es un archivo JSON que leemos completo, modificamos en memoria y reescribimos completo. No hay operaciones incrementales.

**Equivalente mental:** es como si fuera un SQLite con una sola tabla, pero sin SQL. Leemos todo a un slice, operamos sobre el slice, y guardamos el slice.

**Cuando migrar a SQLite:** si un harness tuviera 10,000+ tareas, leer/escribir el JSON completo en cada operacion seria lento (~100ms+). Con `modernc.org/sqlite` (pure Go, sin CGO) podrias hacer queries incrementales. Para el MVP con decenas de tareas, JSON es perfecto.

**Ventaja clave del JSON:** el usuario puede abrir `tareas.json` con cualquier editor, ver el estado, copiar informes, o incluso agregar tareas a mano. Esto es un feature, no un bug — va de la mano con la filosofia de archivos planos de lazyharness.

El archivo se ve asi:

```json
{
  "tasks": [
    {
      "id": 1,
      "role": "arquitecto",
      "title": "Refactorizar modulo de auth",
      "report": "Se separó la logica de autenticacion...",
      "status": "en_curso",
      "created_at": "2026-06-14T10:30:00Z",
      "updated_at": "2026-06-15T09:15:00Z"
    },
    {
      "id": 2,
      "role": "reviewer",
      "title": "Revisar PR #42",
      "status": "pendiente",
      "created_at": "2026-06-15T11:00:00Z",
      "updated_at": "2026-06-15T11:00:00Z"
    }
  ]
}
```

### Decision 2: TaskStatus como string constant vs iota

Usamos `type TaskStatus string` con constantes en vez de `iota`. El JSON es legible sin marshaler custom:

```json
"status": "en_curso"
```

En vez de:

```json
"status": 1
```

**Tradeoff:** perdemos la garantia de que solo valores validos se asignen (alguien podria hacer `TaskStatus("inventado")`). Lo compensamos con el metodo `IsValid()` y validacion en el storage.

**Si tuvieramos 20+ estados** (como en un sistema de tickets complejo), usariamos `iota` con `MarshalJSON`/`UnmarshalJSON` para tener type safety completa. Para 3 estados, las string constants son mas simples.

### Decision 3: Ordenar al guardar vs al mostrar

Ordenamos las tareas (pendientes primero, hechas al final) en `SaveTasks`, no en la UI. Esto tiene dos ventajas:

1. El archivo JSON siempre esta ordenado — legible para humanos.
2. La UI no necesita logica de sorting — muestra lo que carga.

**Desventaja:** si quisieramos cambiar el orden en la UI (por ejemplo, "mostrar hechas primero" como opcion del usuario), tendriamos que separar el sort del save. Para el MVP, un solo orden esta bien.

### Decision 4: Input inline vs dialogo modal

Para crear una tarea nueva, usamos un textinput inline en la parte inferior de la pantalla (similar a como vim abre una linea de comando con `:`). Es mas liviano que un dialogo modal y mantiene el contexto visible.

**Tradeoff:** el input inline es bueno para un solo campo (titulo). Si necesitaramos multiples campos (titulo + descripcion + prioridad), un dialogo modal seria mejor. Para el MVP, titulo + rol (con Tab para ciclar) es suficiente.

## Errores comunes y tips

### Error: zero value de time.Time en JSON

Si creas un `Task` sin setear `CreatedAt`, el JSON va a tener:

```json
"created_at": "0001-01-01T00:00:00Z"
```

Eso es el zero value de `time.Time` — valido pero sin sentido. Siempre inicializa con `time.Now()`:

```go
task := Task{
    CreatedAt: time.Now(),
    UpdatedAt: time.Now(),
}
```

Y en la UI, verifica con `t.IsZero()` antes de formatear.

### Error: JSON marshal de time.Time con zona horaria

Por default, `time.Now()` usa la zona horaria local del sistema. Si tus tests corren en UTC y tu maquina esta en UTC-3, las fechas van a diferir.

**Solucion para tests:** usa `time.Date(2026, 6, 15, 14, 30, 0, 0, time.UTC)` para crear fechas deterministas.

### Error: olvidar guardar despues de modificar

Un pattern comun es leer las tareas, modificar una, y olvidarse de llamar `SaveTasks`:

```go
// MAL
tasks, _ := store.LoadTasks(dir)
tasks[0].Status = domain.TaskDone
// ... y se perdio el cambio

// BIEN
tasks, _ := store.LoadTasks(dir)
tasks[0].Status = domain.TaskDone
store.SaveTasks(dir, tasks)
```

Por eso nuestras funciones de storage (`UpdateTaskStatus`, `DeleteTask`) encapsulan el load + modify + save.

### Tip: el format string de Go es una mnemotecnica

Si te cuesta recordar `2006-01-02 15:04:05`, pensalo como la secuencia americana: `01/02 03:04:05 PM '06`:

```
Mes 01 / Dia 02   Hora 03 : Min 04 : Seg 05   PM   Ano '06
```

O mas facil: 1, 2, 3, 4, 5, 6 — en orden.

### Tip: sort.SliceStable para mantener el orden de insercion

Cuando dos tareas tienen el mismo status, `sort.Slice` puede reordenarlas de forma impredecible. `sort.SliceStable` mantiene el orden original entre elementos iguales — util para que las tareas no "salten" al cambiar el estado de otra.

## Ejercicios

### 1. Basico: definir Task struct y CRUD

Implementa `internal/domain/task.go` con el struct Task y los metodos basicos. Implementa `internal/storage/tasks.go` con Load/Save/Add/Update/Delete. Correr los tests deberia pasar todo.

### 2. Intermedio: construir el panel de tareas en la TUI

Implementa `internal/tui/workspace/model.go` con el layout de dos paneles. Verifica que:
- La lista muestra tareas con colores por status.
- j/k navega entre tareas.
- El panel derecho muestra el detalle de la tarea seleccionada.

### 3. Avanzado: CRUD completo desde la TUI

Implementa las acciones:
- `n` para crear (con input de titulo + selector de rol con Tab).
- `d` para ciclar el estado (pendiente -> en_curso -> hecha -> pendiente).
- `x` para eliminar (con confirmacion).

### 4. Bonus: table-driven tests para CRUD

Escribe un test que:
1. Cree 5 tareas.
2. Actualice el status de 2.
3. Elimine 1.
4. Verifique que quedan 4 tareas con los estados correctos.

Usa subtests con `t.Run` para cada operacion.

### 5. Bonus: filtrar tareas por rol

Agrega la tecla `f` que cicla entre "todas" y filtrar por cada rol. Cuando esta activo un filtro, la lista solo muestra tareas de ese rol. El titulo del panel indica el filtro activo.

## Para profundizar

- [Go by Example — Time](https://gobyexample.com/time): operaciones con tiempo.
- [Go by Example — Time Formatting/Parsing](https://gobyexample.com/time-formatting-parsing): el formato de referencia en detalle.
- [Go by Example — Sorting](https://gobyexample.com/sorting): sort.Slice y sort.SliceStable.
- [Go by Example — JSON](https://gobyexample.com/json): struct tags, marshal, unmarshal.
- [Effective Go — Constants (iota)](https://go.dev/doc/effective_go#constants): el patron de iota para enums.
- [Go JSON and struct tags](https://go.dev/blog/json): blog oficial sobre encoding/json.
- [Documentacion de requerimientos — seccion F](../1_requerimientos.md): los requerimientos de tareas y memoria.

## Que sigue

En la [Clase 09](./09_runtime_delegado.md) construimos el runtime delegado. Al presionar `i` sobre un rol, lazyharness lanza Claude Code (u otro CLI agentic) con el prompt del rol inyectado. Vas a aprender `os/exec` para lanzar procesos externos, goroutines para no bloquear la TUI, y la interface `Executor` para mockear en tests.
