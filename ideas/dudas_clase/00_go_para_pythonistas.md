# Clase 00: Go para Pythonistas — dudas y conceptos clave

Registro de conceptos que generaron preguntas durante las primeras clases del proyecto.

---

## 1. `go mod init` y el module path

**Pregunta:** Al ejecutar `go mod init github.com/Gerardo1909/lazyharness`, ¿ese path apunta al repo remoto? ¿Es obligatorio tener GitHub?

**Respuesta:**
- El path es solo un **identificador único**, no una URL que Go consulta al inicializarse.
- La convención `github.com/usuario/repo` se usa porque garantiza unicidad global.
- Go usa ese path cuando alguien hace `go get github.com/...` para descargarlo, pero eso es un paso posterior.
- Sin repo remoto podés usar cualquier path (`lazyharness`, `mi.dominio/lazyharness`, etc.) — Go no verifica que exista durante el desarrollo local.
- Sin git en absoluto también funciona. `go mod` no depende de git para el módulo propio.

**Regla práctica:** si pensás publicar el proyecto, usá el path del repo real desde el inicio para no tener que renombrar todos los imports después.

---

## 2. Errores de compilación en `harness.go`

### 2a. Llaves mal cerradas en `AddRole`

**Problema:** El `append` y el `return nil` quedaron *dentro* del `for`, y la función quedó sin su `}` de cierre.

```go
// roto
func (h *Harness) AddRole(role Role) error {
    for _, existingRole := range h.Roles {
        if existingRole.Name == role.Name {
            return fmt.Errorf(...)
        }
    h.Roles = append(h.Roles, role)  // dentro del for
    return nil
}  // cierra el for, no la función

// correcto
func (h *Harness) AddRole(role Role) error {
    for _, existingRole := range h.Roles {
        if existingRole.Name == role.Name {
            return fmt.Errorf(...)
        }
    }  // cierra el for
    h.Roles = append(h.Roles, role)
    return nil
}  // cierra la función
```

### 2b. Type mismatch en `CreatedAt`

**Problema:** `CreatedAt` es `string` pero `time.Now()` retorna `time.Time`.

```go
// roto
CreatedAt: time.Now()

// correcto
CreatedAt: time.Now().Format(time.RFC3339)
```

---

## 3. Error de compilación en tests — `FindRoleByName` retorna `bool`, no `error`

**Problema:** `FindRoleByName` retorna `(Role, bool)` pero el test trataba el segundo valor como `error`.

```go
// roto — no compila: no se puede comparar bool con nil
foundRole, err := harness.FindRoleByName("arquitecto")
if err != nil { ... }

// correcto
foundRole, ok := harness.FindRoleByName("arquitecto")
if !ok { ... }
```

**Por qué:** En Go no podés comparar un `bool` con `nil`. `nil` solo aplica a punteros, interfaces, slices, maps, channels y funciones.

---

## 4. Orden de parámetros en los tests de `NewHarness`

**Problema:** La firma es `NewHarness(name, projectDir, promptFormat)` pero el test llamaba con los argumentos en el orden equivocado.

```go
// roto — dir termina donde va promptFormat
NewHarness(tt.harnessName, tt.format, tt.dir)

// correcto
NewHarness(tt.harnessName, tt.dir, tt.format)
```

---

## 5. El `_` en los `for range` — no es un error

**Pregunta:** En `for _, role := range h.Roles`, el `_` parece un error descartado como en `_, err := func()`. ¿Es lo mismo?

**Respuesta:** No. El `_` descarta valores en ambos casos, pero lo que descarta es distinto:

| Contexto | Primer valor | Segundo valor |
|---|---|---|
| `for _, role := range slice` | índice (int) | elemento |
| `result, err := funcion()` | dato útil | error |
| `_, ok := funcion()` | dato descartado | bool de existencia |

En el `for range` de un slice, Go siempre devuelve `(índice, valor)`. Como el índice no se necesita, se descarta con `_`.

Notar que el `_` funciona como convención en go cuando le queremos decir al compilador "sé que esto existe pero no me interesa". Es una forma de evitar warnings por variables no usadas.

---

## 6. `json.MarshalIndent` — los parámetros `""` y `"  "`

**Pregunta:** ¿Qué significan el segundo y tercer parámetro de `json.MarshalIndent(harness, "", "  ")`?

**Firma:** `func MarshalIndent(v any, prefix string, indent string) ([]byte, error)`

- **`prefix`** (`""`): cadena que se antepone a *cada línea* del JSON. Útil para embeber JSON dentro de otro texto con sangría base. En uso normal va vacío.
- **`indent`** (`"  "`): los dos espacios usados por cada nivel de anidación.

```json
// resultado con ("", "  ")
{
  "name": "dev-flow",
  "roles": [
    { "name": "arquitecto" }
  ]
}
```

---

## 7. `var loaded domain.Harness` y el `&` en `Unmarshal`

**Pregunta:** ¿Por qué se usa `var` sin asignar valor? ¿Y qué hace el `&` en `json.Unmarshal(data, &loaded)`?

### `var loaded domain.Harness`

Declara la variable e inicializa con el *zero value* del tipo: strings vacíos, slices nil, etc. Es equivalente a `loaded := domain.Harness{}`. La forma con `var` es idiomática cuando la variable va a ser rellenada inmediatamente por otra función.

### El `&` — pasar por puntero

En Go los argumentos se pasan **por copia**. Si `Unmarshal` recibiera `loaded` directo, llenaría la copia y la descartaría — tu `loaded` original quedaría vacío.

El `&` toma la **dirección de memoria** de `loaded`. Así `Unmarshal` sabe exactamente dónde escribir:

```
sin &:  Unmarshal recibe copia → escribe en la copia → loaded sigue vacío
con &:  Unmarshal recibe dirección → escribe en el original → loaded queda cargado
```

Este patrón aparece siempre que una función necesita *modificar* lo que recibe. Lo mismo ocurre con los métodos de puntero:

```go
func (h *Harness) AddRole(role Role) error { ... }
//        ^ puntero porque AddRole modifica el Harness
```

---

## 8. Convención de nombres en tests — Uncle Bob vs Go idiomático

**Pregunta:** ¿Es válida la convención `TestHarnessShouldBeCreatedWhenValidParamsSubmitted`?

**Respuesta:** Funciona y es legible, pero en Go la comunidad prefiere nombres más cortos como `TestNewHarness`. El motivo: cuando usás *table-driven tests* (el patrón idiomático de Go), los nombres descriptivos ya viven dentro de cada caso `tt.name`, no en el nombre de la función.

```go
// estilo Uncle Bob — nombre largo en la función
func TestHarnessShouldBeCreatedWhenValidParamsSubmitted(t *testing.T) { ... }

// estilo Go idiomático — nombre corto en la función, descripción en los casos
func TestNewHarness(t *testing.T) {
    tests := []struct{ ... }{
        {"valido con xml", ...},
        {"falla con nombre vacio", ...},
    }
}
```

No es un error, es preferencia de estilo. Con table-driven tests el estilo Go resulta menos redundante.
