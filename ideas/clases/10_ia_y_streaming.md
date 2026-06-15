# Clase 10: Integracion con IA -- chat y streaming

> Al terminar esta clase, el editor tiene un chat bar (Ctrl+A) que se comunica con la API de Anthropic via HTTP, los tokens llegan en streaming y se renderizan en tiempo real en un viewport, y la configuracion del provider vive en `harness.json`.

## Prerequisitos

- [Clase 06](./06_editor_embebido.md): editor de prompts con textarea y guardado.
- [Clase 09](./09_procesos_externos.md): goroutines, channels, `context.Context`.

## Conceptos de Go que vas a aprender

### 1. `net/http` -- cliente HTTP en Go

Go tiene un cliente HTTP completo en la stdlib. No necesitas requests ni axios:

```go
import "net/http"

// GET simple
resp, err := http.Get("https://api.anthropic.com/v1/models")
if err != nil {
    return err
}
defer resp.Body.Close() // SIEMPRE cerrar el body

body, err := io.ReadAll(resp.Body)
```

**El `defer resp.Body.Close()` es critico.** Si no cerras el body, la conexion TCP queda abierta y eventualmente te quedas sin file descriptors. Es el equivalente a no cerrar un archivo.

Para requests mas complejas (POST con headers):

```go
// Crear la request manualmente
reqBody := strings.NewReader(`{"model":"claude-sonnet-4-20250514","max_tokens":1024}`)
req, err := http.NewRequestWithContext(ctx, "POST", url, reqBody)
if err != nil {
    return err
}

// Headers
req.Header.Set("Content-Type", "application/json")
req.Header.Set("x-api-key", apiKey)
req.Header.Set("anthropic-version", "2023-06-01")

// Enviar
client := &http.Client{Timeout: 30 * time.Second}
resp, err := client.Do(req)
if err != nil {
    return err
}
defer resp.Body.Close()
```

**Equivalente Python:**

```python
import requests

resp = requests.post(
    "https://api.anthropic.com/v1/messages",
    headers={
        "x-api-key": api_key,
        "anthropic-version": "2023-06-01",
        "content-type": "application/json",
    },
    json={"model": "claude-sonnet-4-20250514", "max_tokens": 1024, ...},
)
```

**Tradeoff stdlib vs librerias externas:** Go tiene todo en la stdlib (`net/http`, `encoding/json`). Python necesita `requests` o `httpx` porque `urllib` es horrible. En Go no necesitas dependencias extra para HTTP.

### 2. Streaming HTTP -- Server-Sent Events (SSE)

La API de Anthropic soporta streaming: en vez de esperar la respuesta completa, recibis tokens a medida que el modelo los genera. Esto usa el protocolo SSE (Server-Sent Events).

El flujo es:

```
Cliente                    API Anthropic
  |                            |
  |--- POST /v1/messages ----->|
  |    stream: true            |
  |                            |
  |<--- event: message_start --|
  |<--- event: content_block_delta (token: "Sos")
  |<--- event: content_block_delta (token: " el")
  |<--- event: content_block_delta (token: " arqu")
  |<--- event: content_block_delta (token: "itecto")
  |<--- event: message_stop ---|
  |                            |
```

En Go, leer SSE es leer lineas del response body:

```go
// Enviar request con stream: true
reqData := map[string]interface{}{
    "model":      "claude-sonnet-4-20250514",
    "max_tokens": 4096,
    "stream":     true,
    "messages":   messages,
}

// ... crear y enviar la request ...

// Leer el stream linea por linea
scanner := bufio.NewScanner(resp.Body)
for scanner.Scan() {
    line := scanner.Text()

    // Las lineas SSE tienen formato "event: tipo" y "data: json"
    if strings.HasPrefix(line, "data: ") {
        data := line[6:] // quitar "data: "

        if data == "[DONE]" {
            break
        }

        var event StreamEvent
        if err := json.Unmarshal([]byte(data), &event); err != nil {
            continue // ignorar lineas que no parsean
        }

        if event.Type == "content_block_delta" {
            // Extraer el token del delta
            token := event.Delta.Text
            // Mandar a la UI
            tokenCh <- token
        }
    }
}
```

**Equivalente Python:**

```python
import anthropic

client = anthropic.Anthropic()
with client.messages.stream(
    model="claude-sonnet-4-20250514",
    max_tokens=4096,
    messages=messages,
) as stream:
    for text in stream.text_stream:
        print(text, end="", flush=True)
```

Python con el SDK de Anthropic es mas simple porque el SDK maneja el parseo de SSE. En Go hacemos el parseo manual porque no tenemos un SDK obligatorio (y aprender el protocolo es valioso).

**Tradeoff SDK vs HTTP directo:**

| Aspecto | SDK (si existiera en Go) | HTTP directo |
|---------|--------------------------|--------------|
| Codigo | Menos lineas | Mas lineas |
| Control | Menos (caja negra) | Total |
| Dependencia | +1 dep que puede romper | Solo stdlib |
| Aprendizaje | Bajo | Alto (entendes SSE) |

Para lazyharness elegimos HTTP directo. El protocolo es simple (JSON lines) y no queremos depender de un SDK que puede cambiar.

### 3. Variables de entorno para API keys

Nunca hardcodees una API key. Leela del entorno:

```go
import "os"

apiKey := os.Getenv("ANTHROPIC_API_KEY")
if apiKey == "" {
    return fmt.Errorf("ANTHROPIC_API_KEY no esta definida en el entorno")
}
```

En `harness.json`, guardamos el NOMBRE de la variable, no el valor:

```json
{
    "name": "dev-flow",
    "provider": {
        "name": "anthropic",
        "model": "claude-sonnet-4-20250514",
        "api_key_env": "ANTHROPIC_API_KEY"
    }
}
```

**Equivalente Python:**

```python
import os
api_key = os.environ.get("ANTHROPIC_API_KEY")
if not api_key:
    raise ValueError("ANTHROPIC_API_KEY no esta definida")
```

**Tradeoff env vars vs keyring vs config file:**

| Metodo | Seguridad | Portabilidad | Complejidad |
|--------|-----------|--------------|-------------|
| Env vars | Media (visible en /proc) | Alta | Cero |
| Keyring (OS) | Alta | Baja (cada OS diferente) | Alta |
| Config file | Baja (archivo en disco) | Media | Media |

Env vars es el estandar de la industria para CLIs. Docker, AWS CLI, Claude Code -- todos usan env vars. Es lo mas simple y portable.

### 4. Goroutines para requests HTTP en background

Una request HTTP puede tardar segundos. No la hagas en Update -- lanzala en una goroutine via `tea.Cmd`:

```go
// Mensaje con el resultado
type aiResponseMsg struct {
    token string  // un token del stream
    done  bool    // true cuando termino
    err   error
}

// Cmd que lanza el stream en background
func streamAIResponse(client *AIClient, messages []Message) tea.Cmd {
    return func() tea.Msg {
        ctx := context.Background()
        tokenCh, errCh := client.StreamMessage(ctx, messages)

        // Leer el primer token y mandarlo como mensaje
        // Los siguientes tokens se manejan con un Cmd encadenado
        select {
        case token, ok := <-tokenCh:
            if !ok {
                return aiResponseMsg{done: true}
            }
            return aiResponseMsg{token: token}
        case err := <-errCh:
            return aiResponseMsg{err: err}
        }
    }
}
```

Pero aca hay un problema: un `tea.Cmd` solo puede retornar UN mensaje. Si queremos mandar multiples tokens, necesitamos encadenar Cmds o usar un patron mas avanzado. La solucion es usar `p.Send()` desde una goroutine:

```go
// Alternativa: usar el programa para mandar mensajes directamente
func startStreaming(p *tea.Program, client *AIClient, messages []Message) {
    go func() {
        ctx := context.Background()
        ch := client.StreamTokens(ctx, messages)

        for token := range ch {
            p.Send(aiResponseMsg{token: token})
        }
        p.Send(aiResponseMsg{done: true})
    }()
}
```

**Equivalente Python (textual):**

```python
# En textual, usarias un Worker
class ChatWidget(Widget):
    async def stream_response(self, messages):
        async for token in client.stream(messages):
            self.post_message(TokenReceived(token))
```

### 5. `httptest.NewServer` para testing

Go tiene un servidor HTTP de test en la stdlib. No necesitas librerias de mocking:

```go
import "net/http/httptest"

func TestStreamingResponse(t *testing.T) {
    // Crear un servidor que simula la API de Anthropic
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verificar que llego el header correcto
        if r.Header.Get("x-api-key") == "" {
            t.Error("falta x-api-key")
        }

        // Simular respuesta SSE
        w.Header().Set("Content-Type", "text/event-stream")
        flusher := w.(http.Flusher)

        events := []string{
            `{"type":"message_start","message":{"id":"msg_01"}}`,
            `{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hola"}}`,
            `{"type":"content_block_delta","delta":{"type":"text_delta","text":" mundo"}}`,
            `{"type":"message_stop"}`,
        }

        for _, event := range events {
            fmt.Fprintf(w, "event: message\ndata: %s\n\n", event)
            flusher.Flush()
        }
    }))
    defer server.Close()

    // Usar server.URL como endpoint en vez de la API real
    client := NewAIClient(server.URL, "test-key", "claude-sonnet-4-20250514")
    // ... testear el streaming ...
}
```

**Equivalente Python:**

```python
# responses o pytest-httpserver
import responses

@responses.activate
def test_streaming():
    responses.add(
        responses.POST,
        "https://api.anthropic.com/v1/messages",
        body="data: ...",
        stream=True,
    )
```

`httptest.NewServer` es mas potente porque es un servidor HTTP real corriendo en localhost. Podes testear todo el flujo HTTP de punta a punta, incluyendo streaming.

### 6. JSON streaming (line-delimited JSON)

SSE es esencialmente JSON delimitado por lineas con metadata. Parseamos cada linea individualmente:

```go
// Structs para los eventos de la API
type StreamEvent struct {
    Type    string       `json:"type"`
    Delta   *DeltaEvent  `json:"delta,omitempty"`
    Message *MessageInfo `json:"message,omitempty"`
}

type DeltaEvent struct {
    Type string `json:"type"`
    Text string `json:"text"`
}

type MessageInfo struct {
    ID    string `json:"id"`
    Model string `json:"model"`
    Usage *Usage `json:"usage,omitempty"`
}

type Usage struct {
    InputTokens  int `json:"input_tokens"`
    OutputTokens int `json:"output_tokens"`
}
```

**Tip:** usa `omitempty` para campos opcionales. Asi el JSON no incluye campos vacios y el parseo no falla si faltan.

## Lo que vamos a construir

1. Configuracion de provider en `harness.json`.
2. Cliente HTTP que habla con la API de Anthropic.
3. Streaming SSE con rendering en tiempo real.
4. Chat bar en el editor (Ctrl+A activa el modo chat).

```
  Editor con chat bar activado (mockup 03 simplificado)
  ┌─ dev-flow ── arquitecto.xml ── editando ─────────────────────┐
  │                                                               │
  │  ┌─ Editor ─────────────────────────────────────────────────┐ │
  │  │ <role>                                                   │ │
  │  │   Sos el arquitecto del proyecto.                        │ │
  │  │   Tu responsabilidad es disenar la estructura general    │ │
  │  │   y tomar decisiones de alto nivel.                      │ │
  │  │ </role>                                                  │ │
  │  │ <constraints>                                            │ │
  │  │   Consulta con @code-reviewer antes de|                  │ │
  │  │                                                          │ │
  │  └──────────────────────────────────────────────────────────┘ │
  │  ┌─ Chat IA ── claude-sonnet-4 ─────────────────────────────┐ │
  │  │ > Quiero que el arquitecto tambien maneje la seguridad   │ │
  │  │                                                          │ │
  │  │ Entendido. Voy a agregar responsabilidades de seguridad  │ │
  │  │ al rol. Te sugiero agregar una seccion <security> que... │ │
  │  │ ~~~ (streaming) ~~~                                      │ │
  │  └──────────────────────────────────────────────────────────┘ │
  │  ctrl+a chat  ctrl+s guardar  esc cancelar                   │
  └───────────────────────────────────────────────────────────────┘
```

**Archivos a crear:**
- `internal/runtime/aiclient.go`
- `internal/runtime/aiclient_test.go`
- `internal/runtime/provider.go`

**Archivos a modificar:**
- `internal/domain/harness.go` -- agregar ProviderConfig al struct
- `internal/tui/editor/model.go` -- agregar chat bar con streaming
- `internal/tui/keys.go` -- agregar tecla Ctrl+A para chat

## Implementacion paso a paso

### Paso 1: Definir la configuracion del provider

Agrega a `internal/runtime/provider.go`:

```go
package runtime

import (
    "fmt"
    "os"
)

// ProviderConfig es la configuracion del proveedor de IA
type ProviderConfig struct {
    Name      string `json:"name"`          // "anthropic", "openai", etc
    Model     string `json:"model"`         // "claude-sonnet-4-20250514"
    APIKeyEnv string `json:"api_key_env"`   // nombre de la env var
    BaseURL   string `json:"base_url,omitempty"` // override para proxy o local
}

// DefaultProvider retorna la configuracion por defecto
func DefaultProvider() ProviderConfig {
    return ProviderConfig{
        Name:      "anthropic",
        Model:     "claude-sonnet-4-20250514",
        APIKeyEnv: "ANTHROPIC_API_KEY",
        BaseURL:   "https://api.anthropic.com",
    }
}

// ResolveAPIKey lee la API key del entorno
func (p ProviderConfig) ResolveAPIKey() (string, error) {
    key := os.Getenv(p.APIKeyEnv)
    if key == "" {
        return "", fmt.Errorf(
            "la variable de entorno %s no esta definida.\n"+
                "Configurala con: export %s=tu-api-key",
            p.APIKeyEnv, p.APIKeyEnv,
        )
    }
    return key, nil
}

// Endpoint retorna la URL base del provider
func (p ProviderConfig) Endpoint() string {
    if p.BaseURL != "" {
        return p.BaseURL
    }
    switch p.Name {
    case "anthropic":
        return "https://api.anthropic.com"
    default:
        return p.BaseURL
    }
}
```

Modifica `internal/domain/harness.go` para incluir el provider:

```go
type Harness struct {
    Name         string           `json:"name"`
    PromptFormat string           `json:"prompt_format"`
    ProjectDir   string           `json:"project_dir"`
    Roles        []Role           `json:"roles"`
    Workflow     []string         `json:"workflow,omitempty"`
    Provider     *ProviderConfig  `json:"provider,omitempty"`  // nuevo
}
```

El puntero `*ProviderConfig` permite que sea `nil` (harness sin provider configurado = no tiene asistencia IA).

### Paso 2: Implementar el cliente de IA

Crea `internal/runtime/aiclient.go`:

```go
package runtime

import (
    "bufio"
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"
)

// Message es un mensaje en el formato de la API
type Message struct {
    Role    string `json:"role"`    // "user" o "assistant"
    Content string `json:"content"`
}

// StreamEvent representa un evento SSE de la API
type StreamEvent struct {
    Type    string      `json:"type"`
    Delta   *Delta      `json:"delta,omitempty"`
    Message *MsgInfo    `json:"message,omitempty"`
    Usage   *TokenUsage `json:"usage,omitempty"`
}

type Delta struct {
    Type string `json:"type"`
    Text string `json:"text"`
}

type MsgInfo struct {
    ID    string `json:"id"`
    Model string `json:"model"`
}

type TokenUsage struct {
    InputTokens  int `json:"input_tokens"`
    OutputTokens int `json:"output_tokens"`
}

// AIClient maneja la comunicacion con la API del provider
type AIClient struct {
    baseURL    string
    apiKey     string
    model      string
    httpClient *http.Client
}

// NewAIClient crea un cliente de IA
func NewAIClient(baseURL, apiKey, model string) *AIClient {
    return &AIClient{
        baseURL: baseURL,
        apiKey:  apiKey,
        model:   model,
        httpClient: &http.Client{
            Timeout: 5 * time.Minute, // streaming puede tardar
        },
    }
}

// NewAIClientFromConfig crea un cliente desde la configuracion del provider
func NewAIClientFromConfig(config ProviderConfig) (*AIClient, error) {
    apiKey, err := config.ResolveAPIKey()
    if err != nil {
        return nil, err
    }

    return NewAIClient(config.Endpoint(), apiKey, config.Model), nil
}

// messagesRequest es el body de la request a la API
type messagesRequest struct {
    Model     string    `json:"model"`
    MaxTokens int       `json:"max_tokens"`
    Stream    bool      `json:"stream"`
    System    string    `json:"system,omitempty"`
    Messages  []Message `json:"messages"`
}

// StreamTokens envia mensajes y retorna un channel que emite tokens
// El channel se cierra cuando el stream termina
func (c *AIClient) StreamTokens(ctx context.Context, system string, messages []Message) (<-chan string, <-chan error) {
    tokenCh := make(chan string, 64)  // buffer para no bloquear el parseo
    errCh := make(chan error, 1)

    go func() {
        defer close(tokenCh)
        defer close(errCh)

        reqData := messagesRequest{
            Model:     c.model,
            MaxTokens: 4096,
            Stream:    true,
            System:    system,
            Messages:  messages,
        }

        body, err := json.Marshal(reqData)
        if err != nil {
            errCh <- fmt.Errorf("serializando request: %w", err)
            return
        }

        req, err := http.NewRequestWithContext(
            ctx, "POST",
            c.baseURL+"/v1/messages",
            bytes.NewReader(body),
        )
        if err != nil {
            errCh <- fmt.Errorf("creando request: %w", err)
            return
        }

        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("x-api-key", c.apiKey)
        req.Header.Set("anthropic-version", "2023-06-01")

        resp, err := c.httpClient.Do(req)
        if err != nil {
            errCh <- fmt.Errorf("enviando request: %w", err)
            return
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            respBody, _ := io.ReadAll(resp.Body)
            errCh <- fmt.Errorf("API retorno %d: %s", resp.StatusCode, string(respBody))
            return
        }

        // Parsear el stream SSE
        scanner := bufio.NewScanner(resp.Body)
        for scanner.Scan() {
            line := scanner.Text()

            if !strings.HasPrefix(line, "data: ") {
                continue
            }

            data := line[6:]
            if data == "[DONE]" {
                break
            }

            var event StreamEvent
            if err := json.Unmarshal([]byte(data), &event); err != nil {
                continue // ignorar lineas que no parsean
            }

            if event.Type == "content_block_delta" && event.Delta != nil {
                select {
                case tokenCh <- event.Delta.Text:
                case <-ctx.Done():
                    return
                }
            }
        }

        if err := scanner.Err(); err != nil {
            errCh <- fmt.Errorf("leyendo stream: %w", err)
        }
    }()

    return tokenCh, errCh
}

// SendMessage envia un mensaje y espera la respuesta completa (sin streaming)
func (c *AIClient) SendMessage(ctx context.Context, system string, messages []Message) (string, error) {
    reqData := messagesRequest{
        Model:     c.model,
        MaxTokens: 4096,
        Stream:    false,
        System:    system,
        Messages:  messages,
    }

    body, err := json.Marshal(reqData)
    if err != nil {
        return "", fmt.Errorf("serializando request: %w", err)
    }

    req, err := http.NewRequestWithContext(
        ctx, "POST",
        c.baseURL+"/v1/messages",
        bytes.NewReader(body),
    )
    if err != nil {
        return "", fmt.Errorf("creando request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("x-api-key", c.apiKey)
    req.Header.Set("anthropic-version", "2023-06-01")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return "", fmt.Errorf("enviando request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        respBody, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("API retorno %d: %s", resp.StatusCode, string(respBody))
    }

    // Parsear la respuesta completa
    var result struct {
        Content []struct {
            Type string `json:"type"`
            Text string `json:"text"`
        } `json:"content"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", fmt.Errorf("parseando respuesta: %w", err)
    }

    var text strings.Builder
    for _, block := range result.Content {
        if block.Type == "text" {
            text.WriteString(block.Text)
        }
    }

    return text.String(), nil
}
```

### Paso 3: Tests del cliente con httptest

Crea `internal/runtime/aiclient_test.go`:

```go
package runtime

import (
    "context"
    "fmt"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"
)

func TestStreamTokens(t *testing.T) {
    // Servidor mock que simula streaming SSE
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verificar headers
        if r.Header.Get("x-api-key") != "test-key" {
            t.Error("falta x-api-key")
        }
        if r.Header.Get("anthropic-version") != "2023-06-01" {
            t.Error("falta anthropic-version")
        }

        w.Header().Set("Content-Type", "text/event-stream")
        flusher := w.(http.Flusher)

        tokens := []string{"Hola", " ", "mundo"}
        for _, token := range tokens {
            data := fmt.Sprintf(
                `{"type":"content_block_delta","delta":{"type":"text_delta","text":"%s"}}`,
                token,
            )
            fmt.Fprintf(w, "event: content_block_delta\ndata: %s\n\n", data)
            flusher.Flush()
        }

        fmt.Fprintf(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
        flusher.Flush()
    }))
    defer server.Close()

    client := NewAIClient(server.URL, "test-key", "test-model")
    ctx := context.Background()

    messages := []Message{{Role: "user", Content: "saludame"}}
    tokenCh, errCh := client.StreamTokens(ctx, "", messages)

    var result strings.Builder
    for token := range tokenCh {
        result.WriteString(token)
    }

    // Verificar que no hubo errores
    select {
    case err := <-errCh:
        if err != nil {
            t.Fatalf("error inesperado: %v", err)
        }
    default:
    }

    expected := "Hola mundo"
    if result.String() != expected {
        t.Errorf("esperaba %q, obtuve %q", expected, result.String())
    }
}

func TestStreamTokens_APIError(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusUnauthorized)
        w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
    }))
    defer server.Close()

    client := NewAIClient(server.URL, "bad-key", "test-model")
    ctx := context.Background()

    _, errCh := client.StreamTokens(ctx, "", []Message{{Role: "user", Content: "hola"}})

    err := <-errCh
    if err == nil {
        t.Error("esperaba error con API key invalida")
    }
    if !strings.Contains(err.Error(), "401") {
        t.Errorf("error deberia mencionar 401, obtuve: %v", err)
    }
}

func TestStreamTokens_ContextCancellation(t *testing.T) {
    // Servidor que tarda mucho
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/event-stream")
        flusher := w.(http.Flusher)

        // Mandar tokens lentamente
        for i := 0; i < 100; i++ {
            data := `{"type":"content_block_delta","delta":{"type":"text_delta","text":"token "}}`
            fmt.Fprintf(w, "data: %s\n\n", data)
            flusher.Flush()
            time.Sleep(100 * time.Millisecond)
        }
    }))
    defer server.Close()

    client := NewAIClient(server.URL, "test-key", "test-model")

    // Cancelar despues de 200ms
    ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
    defer cancel()

    tokenCh, _ := client.StreamTokens(ctx, "", []Message{{Role: "user", Content: "hola"}})

    count := 0
    for range tokenCh {
        count++
    }

    // Deberia haber recibido solo unos pocos tokens antes de la cancelacion
    if count > 10 {
        t.Errorf("esperaba pocos tokens con cancelacion, obtuve %d", count)
    }
}

func TestSendMessage(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprint(w, `{
            "content": [
                {"type": "text", "text": "Soy el asistente de harness engineering."}
            ]
        }`)
    }))
    defer server.Close()

    client := NewAIClient(server.URL, "test-key", "test-model")
    ctx := context.Background()

    result, err := client.SendMessage(ctx, "", []Message{{Role: "user", Content: "quien sos?"}})
    if err != nil {
        t.Fatalf("error: %v", err)
    }

    if result != "Soy el asistente de harness engineering." {
        t.Errorf("respuesta inesperada: %q", result)
    }
}

func TestProviderConfig_ResolveAPIKey(t *testing.T) {
    // Setear una variable de entorno temporal
    t.Setenv("TEST_API_KEY", "sk-test-123")

    config := ProviderConfig{
        Name:      "anthropic",
        APIKeyEnv: "TEST_API_KEY",
    }

    key, err := config.ResolveAPIKey()
    if err != nil {
        t.Fatalf("error: %v", err)
    }
    if key != "sk-test-123" {
        t.Errorf("key esperada 'sk-test-123', obtuve '%s'", key)
    }
}

func TestProviderConfig_ResolveAPIKey_Missing(t *testing.T) {
    config := ProviderConfig{
        Name:      "anthropic",
        APIKeyEnv: "VARIABLE_QUE_NO_EXISTE_12345",
    }

    _, err := config.ResolveAPIKey()
    if err == nil {
        t.Error("esperaba error cuando la variable no existe")
    }
}
```

### Paso 4: Integrar el chat bar en el editor

Modifica `internal/tui/editor/model.go` para agregar el modo chat:

```go
type editorMode int

const (
    modeEdit editorMode = iota
    modeChat
)

type Model struct {
    // ... campos existentes ...
    mode         editorMode
    chatInput    textinput.Model
    chatHistory  []runtime.Message
    chatViewport viewport.Model
    aiClient     *runtime.AIClient
    streaming    bool
    streamBuffer strings.Builder
}

// Inicializar el chat input:
func NewModel(/* ... */) Model {
    ci := textinput.New()
    ci.Placeholder = "Preguntale a la IA sobre este prompt..."
    ci.CharLimit = 500

    chatVP := viewport.New(80, 10)

    return Model{
        // ... existentes ...
        chatInput:    ci,
        chatViewport: chatVP,
    }
}

// En Update, manejar Ctrl+A para activar el chat:
case tea.KeyMsg:
    // Ctrl+A activa/desactiva el chat
    if msg.String() == "ctrl+a" {
        if m.mode == modeEdit {
            m.mode = modeChat
            m.chatInput.Focus()
        } else {
            m.mode = modeEdit
            m.chatInput.Blur()
        }
        return m, nil
    }

    if m.mode == modeChat {
        switch msg.Type {
        case tea.KeyEnter:
            if m.chatInput.Value() == "" || m.streaming {
                break
            }
            // Enviar mensaje al IA
            userMsg := m.chatInput.Value()
            m.chatHistory = append(m.chatHistory, runtime.Message{
                Role:    "user",
                Content: userMsg,
            })
            m.chatInput.SetValue("")
            m.streaming = true

            // Construir el system prompt de harness engineering
            systemPrompt := buildHarnessEngineeringPrompt(m.promptContent)

            return m, m.startStreamingCmd(systemPrompt)

        case tea.KeyEsc:
            m.mode = modeEdit
            m.chatInput.Blur()
            return m, nil
        }

        // Delegar al chat input
        m.chatInput, cmd = m.chatInput.Update(msg)
        return m, cmd
    }

// Manejar tokens del stream:
case aiTokenMsg:
    m.streamBuffer.WriteString(msg.token)
    m.updateChatViewport()
    return m, nil

case aiDoneMsg:
    // Guardar la respuesta completa en el historial
    m.chatHistory = append(m.chatHistory, runtime.Message{
        Role:    "assistant",
        Content: m.streamBuffer.String(),
    })
    m.streamBuffer.Reset()
    m.streaming = false
    m.updateChatViewport()
    return m, nil

case aiErrorMsg:
    m.streaming = false
    m.statusMessage = "Error de IA: " + msg.err.Error()
    return m, nil
```

El system prompt de "harness engineering" se inyecta invisible al usuario:

```go
func buildHarnessEngineeringPrompt(currentPrompt string) string {
    return fmt.Sprintf(`Sos un experto en harness engineering — el arte de disenar
system prompts efectivos para agentes de IA. Tu trabajo es ayudar al usuario a
mejorar el prompt que esta editando.

Prompt actual del rol:
---
%s
---

Reglas:
- Responde en espanol.
- Se conciso y directo.
- Cuando sugieras cambios, muestra el fragmento exacto a modificar.
- Pregunta para profundizar si la instruccion del usuario es vaga.`, currentPrompt)
}
```

### Paso 5: Renderizar el chat en el View

```go
func (m Model) renderChatPanel() string {
    if m.mode != modeChat {
        return ""
    }

    // Historial del chat
    var chatContent strings.Builder
    for _, msg := range m.chatHistory {
        if msg.Role == "user" {
            chatContent.WriteString(
                lipgloss.NewStyle().
                    Bold(true).
                    Foreground(tui.ColorBlue).
                    Render("> "+msg.Content) + "\n\n",
            )
        } else {
            chatContent.WriteString(msg.Content + "\n\n")
        }
    }

    // Si esta streaming, agregar el buffer parcial
    if m.streaming {
        chatContent.WriteString(m.streamBuffer.String())
        chatContent.WriteString(
            lipgloss.NewStyle().
                Foreground(tui.ColorComment).
                Render(" ~~~"),
        )
    }

    m.chatViewport.SetContent(chatContent.String())

    header := lipgloss.NewStyle().
        Bold(true).
        Foreground(tui.ColorPurple).
        Render("Chat IA") +
        "  " +
        lipgloss.NewStyle().
            Foreground(tui.ColorComment).
            Render(m.aiClient.model)

    return lipgloss.JoinVertical(lipgloss.Left,
        tui.StyleBorder.Width(m.width).Render(
            lipgloss.JoinVertical(lipgloss.Left,
                header,
                m.chatViewport.View(),
                m.chatInput.View(),
            ),
        ),
    )
}
```

## Tradeoffs y decisiones de diseno

### Decision 1: API directa vs delegacion al CLI

En la [Clase 09](./09_procesos_externos.md) delegamos al CLI para invocar roles. Aca usamos la API directamente. Por que la diferencia?

| Caso de uso | Metodo | Razon |
|-------------|--------|-------|
| Invocar un rol (interactivo) | CLI delegado | El usuario necesita interactuar libremente |
| Chat de asistencia en el editor | API directa | Necesitamos control sobre la UI (streaming en viewport) |
| "Mejorar" prompts (background) | API directa | No hay interaccion del usuario, solo procesamiento |

**Regla general:** si el usuario va a interactuar directamente con el agente, delega al CLI. Si es la app la que consume la respuesta, usa la API directamente.

### Decision 2: Streaming vs respuesta completa

Para el chat bar, streaming es obligatorio. Esperar 10-30 segundos sin feedback es inaceptable. El usuario necesita ver que la IA esta "pensando" y que tokens van llegando.

Para `SendMessage` (sin streaming), la respuesta completa es mas simple de manejar y suficiente para operaciones background donde la latencia no se siente.

### Decision 3: `api_key_env` en vez del valor directo

Guardar `ANTHROPIC_API_KEY` (el nombre) en vez de `sk-ant-...` (el valor) es una decision de seguridad. El `harness.json` se puede commitear en el repo del proyecto. Si tuviera la API key adentro, estarias publicando tus credenciales.

### Decision 4: La skill invisible de harness engineering

El system prompt de "harness engineering" se inyecta sin que el usuario lo vea. Esto es deliberado: el usuario le habla a la IA como si fuera un colega, y la IA responde con expertise en prompts. Si el system prompt fuera visible, romperia la ilusion.

**Tradeoff de transparencia:** algunos usuarios quieren saber que se manda a la IA. Podriamos agregar un modo "verbose" que muestra el system prompt completo. Para el MVP, invisible es suficiente.

## Errores comunes y tips

### Error: HTTP client sin timeout

```go
// MAL: si la API no responde, tu app se cuelga para siempre
client := &http.Client{}

// BIEN: siempre pone un timeout
client := &http.Client{Timeout: 5 * time.Minute}
```

Para streaming, el timeout es sobre la conexion completa, no sobre cada chunk. 5 minutos es razonable para una respuesta larga.

### Error: no cerrar el response body

```go
// MAL: resource leak — la conexion TCP queda abierta
resp, err := client.Do(req)
// usar resp.Body sin cerrar...

// BIEN: defer close inmediatamente despues de verificar err
resp, err := client.Do(req)
if err != nil {
    return err
}
defer resp.Body.Close()
```

El defer se ejecuta al salir de la funcion, sin importar si hay error o return temprano. Es el equivalente a `with` en Python.

### Error: parseo de SSE asumiendo formato fijo

Las lineas SSE pueden tener:
- Lineas vacias (separador entre eventos)
- Lineas `event: tipo`
- Lineas `data: json`
- Lineas `id: id`
- Lineas `retry: ms`

No asumas que cada linea es un `data:`. Filtra las que no necesitas:

```go
if !strings.HasPrefix(line, "data: ") {
    continue // ignorar lineas que no son data
}
```

### Error: API key no encontrada

Siempre verifica la key ANTES de mostrar el chat bar:

```go
// En el momento de activar Ctrl+A:
if m.aiClient == nil {
    _, err := m.provider.ResolveAPIKey()
    if err != nil {
        m.statusMessage = err.Error()
        return m, nil // no activar el chat
    }
    client, _ := runtime.NewAIClientFromConfig(m.provider)
    m.aiClient = client
}
```

### Tip: `t.Setenv` para tests con env vars

Go 1.17+ tiene `t.Setenv` que setea una variable de entorno solo durante el test y la restaura al terminar:

```go
func TestResolveAPIKey(t *testing.T) {
    t.Setenv("MY_API_KEY", "sk-test-123")
    // ... el test ...
    // Al terminar, MY_API_KEY se restaura al valor anterior
}
```

### Tip: `strings.Builder` para acumular tokens

```go
var buffer strings.Builder
for token := range tokenCh {
    buffer.WriteString(token)
}
result := buffer.String()
```

`strings.Builder` es mas eficiente que concatenar strings (`result += token`) porque no crea un string nuevo en cada iteracion.

## Ejercicios

### 1. Basico: mostrar usage de tokens

Agrega un contador que muestre cuantos tokens se usaron despues de cada mensaje. La API manda un evento `message_delta` al final con el usage. Parsealo y mostralo en la UI como "42 in / 156 out".

### 2. Intermedio: historial del chat persistente

Guarda el historial del chat en un archivo JSON (`chat_history.json` en `.lazyharness/`). Al reabrir el editor del mismo rol, carga el historial previo. Esto le da "memoria" al asistente.

### 3. Avanzado: multiples providers

Extiende `AIClient` para soportar OpenAI ademas de Anthropic. La diferencia principal es el formato de la request (campo `system` vs mensaje con role `system`) y los headers (`Authorization: Bearer` vs `x-api-key`). Crea un `ProviderAdapter` interface con implementaciones para cada uno.

### 4. Bonus: rate limiting

Implementa un rate limiter que espere entre requests si el usuario manda mensajes muy rapido. Usa `time.Ticker` para limitar a N requests por minuto. Muestra "esperando..." si se alcanza el limite.

## Para profundizar

- [Anthropic API Reference](https://docs.anthropic.com/en/api/messages): documentacion completa de la API de mensajes.
- [Anthropic Streaming](https://docs.anthropic.com/en/api/messages-streaming): detalles del protocolo SSE para streaming.
- [Go net/http](https://pkg.go.dev/net/http): documentacion del cliente HTTP de la stdlib.
- [Go httptest](https://pkg.go.dev/net/http/httptest): servidores de test para HTTP.
- [Server-Sent Events spec](https://html.spec.whatwg.org/multipage/server-sent-events.html): la especificacion completa de SSE.
- [Go by Example — HTTP Client](https://gobyexample.com/http-client): ejemplo basico de HTTP en Go.

## Que sigue

En la [Clase 11](./11_mejora_y_providers.md) construimos la feature "mejorar" -- un agente en background que lee todos los prompts del harness, los envia al provider para optimizarlos, y presenta los diffs propuestos para que aceptes o rechaces cada uno. Vas a aprender `sync.WaitGroup`, `tea.Tick` para progreso, y `context.WithCancel` para cancelacion por el usuario.
