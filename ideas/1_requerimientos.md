# Requerimientos del sistema 

## Visión

Una TUI estilo lazygit/lazydocker para **construir, versionar y operar harnesses de agentes**
sobre proyectos de código. Un harness es un conjunto de prompts/roles con jerarquía sugerida,
que se versiona con git y se ejecuta delegando en CLIs agentic existentes (Claude Code, etc.).

El nicho: hoy armar un buen esquema multi-agente implica archivos sueltos, convenciones ad-hoc
y cero tooling. lazyharness le da a ese flujo lo que lazygit le dio a git: visibilidad,
velocidad y estructura sin abandonar la terminal ni los archivos planos.

## Conceptos del dominio

| Concepto | Definición |
|---|---|
| **Harness** | Conjunto nombrado de roles + workflow + metadata. Vive en `.lazyharness/` dentro de un proyecto, con su propio repo git. |
| **Rol (prompt)** | Un archivo plano (`.xml` recomendado, `.md`, `.txt`) con el system prompt de un agente. Tiene nombre, color y posición en la jerarquía. |
| **Workflow** | Archivo que documenta el orden natural sugerido de invocación (ej: arquitecto → reviewer → devs). Es informativo, nunca impuesto. |
| **Referencia** | Mención a otro rol dentro de un prompt (`@code-reviewer`). En la UI se renderiza con el color del rol referenciado. |
| **Tarea** | Registro en un JSON (mini base de datos por harness) de trabajos realizados, informes y memoria de los agentes. Legible y editable por el usuario. |
| **Sesión** | Conversación con un rol sobre el proyecto. La maneja el CLI delegado; lazyharness la lista, vincula a tareas y permite retomarla. |

## Decisiones de diseño (con sus tradeoffs)

1. **Constructor + uso, runtime delegado.** lazyharness no implementa su propio loop agentic.
   Al invocar un rol, lanza un CLI existente (Claude Code u otros) inyectándole el prompt del
   rol y el directorio de trabajo. *Tradeoff aceptado:* dependemos de las capacidades e
   interfaz del CLI subyacente; a cambio no construimos un motor de tool-use desde cero.

2. **Harness dentro del proyecto.** `.lazyharness/` vive en el repo del código y se puede
   compartir con el equipo. *Tradeoff aceptado:* no es reusable entre proyectos sin copiar
   (posible feature futura: "duplicar harness desde otro proyecto" o templates).

3. **Un repo git por harness, rollback por archivo.** `.lazyharness/` es un repo git propio,
   independiente del repo del código. Guardar = commit con mensaje obligatorio. "Volver atrás
   un rol" = checkout de ese archivo a un commit anterior + nuevo commit. *Tradeoff aceptado:*
   repo anidado (se documenta si conviene gitignorearlo o comitearlo como subcarpeta).

4. **Archivos planos + metadata.** Cada rol es un archivo legible; colores, jerarquía,
   provider y workflow van en metadata aparte (ej: `harness.json`/`.yaml`). Todo editable
   por fuera de la app. Las @referencias son texto con sintaxis simple.

5. **TUI pura, teclado primero.** Paneles navegables con teclado, barra de atajos visible
   abajo, ayuda contextual en una línea (los "tooltips"). Mouse opcional, nunca requerido.

6. **Jerarquía informativa, invocación manual.** El workflow sugiere el orden; el usuario
   decide a quién invocar y cuándo. No hay orquestación automática agente-a-agente en el MVP.

7. **Tareas como JSON editable.** El historial de trabajo de los agentes (tareas, informes)
   se persiste en un archivo JSON por harness que el usuario puede inspeccionar y editar
   desde la TUI o con cualquier editor.

## Requerimientos funcionales

### A. Pantalla inicial — gestión de harnesses
- **A1.** Al abrir la app se listan los harnesses conocidos (proyectos donde existe `.lazyharness/`), con preview: roles, último commit, provider configurado.
- **A2.** Crear un harness: nombre + formato de prompts (XML recomendado, .md, .txt) + directorio del proyecto. Esto inicializa `.lazyharness/` y su repo git.
- **A3.** Seleccionar un harness lleva a la vista de harness, situada sobre el directorio del proyecto.

### B. Versionado
- **B1.** Cada guardado pide un mensaje y genera un commit en el repo del harness.
- **B2.** Historial navegable filtrado por rol; restaurar la versión anterior de un rol no afecta a los demás (rollback por archivo + commit nuevo).
- **B3.** Vista de diff entre versiones de un prompt antes de restaurar.

### C. Vista de harness
- **C1.** Sidebar izquierda: roles con su color y jerarquía (árbol). Navegación con teclado.
- **C2.** Panel principal: prompt del rol seleccionado en modo solo-lectura, con @referencias coloreadas.
- **C3.** Barra inferior de acciones sobre el rol: `editar`, `eliminar`, `mejorar`, `historial`. Al enfocar cada acción, una línea de ayuda explica qué hace y sus riesgos.
- **C4.** "Mejorar": lanza un agente en background que revisa todos los prompts del harness para optimizarlos y alinearlos. El resultado llega como cambios propuestos (diff) que el usuario acepta o rechaza — nunca sobreescribe directo.

### D. Edición de prompts
- **D1.** Editor embebido tipo nano dentro del panel principal.
- **D2.** Opción "¿Querés hacerlo con IA?": habilita una barra de chat inferior. Por detrás se inyecta una skill de *harness engineering* (invisible al usuario) que va haciendo preguntas para profundizar en lo que el usuario quiere.
- **D3.** Mientras la IA trabaja, el panel derecho muestra los prompts poblándose; el usuario puede ir de uno en uno para revisarlos y editarlos.
- **D4.** Autocompletado de @referencias a otros roles; al insertarse toman el color del rol.

### E. Uso / runtime delegado
- **E1.** Configurar provider por harness (API o IA local) — se usa para la asistencia con IA, el "mejorar" y como modelo sugerido al CLI delegado.
- **E2.** Con un rol seleccionado, una acción "invocar" lanza el CLI agentic en el directorio del proyecto con el prompt del rol inyectado. La UI muestra qué provider/modelo está enlazado al rol.
- **E3.** Las sesiones del CLI delegado se listan por harness y se pueden retomar.

### F. Tareas / memoria
- **F1.** Cada harness mantiene `tareas.json`: registro de trabajos que realizan los agentes, informes que generan y estado (pendiente/en curso/hecha).
- **F2.** Panel de tareas en la TUI: revisar, editar, marcar y borrar entradas.
- **F3.** Los agentes invocados reciben (vía prompt) la indicación de registrar su trabajo en ese archivo, para que la memoria se construya sola.

## Fuera de alcance (MVP)
- Runtime agentic propio (tool-use, permisos, ejecución de comandos).
- Orquestación automática agente-a-agente.
- Harnesses globales / compartidos entre proyectos (más allá de copiar).
- Marketplace o templates de harnesses (candidato a v2).

## Preguntas abiertas
1. ¿El repo `.lazyharness/` se comitea dentro del repo del código (compartirlo con el equipo) o se gitignorea (personal)? ¿Lo decidimos por config?
2. Mecanismo concreto de inyección del prompt al CLI delegado (flags, archivos de config del CLI, wrapper). Varía por CLI — definir al elegir los primeros CLIs soportados.
3. ¿"Mejorar" usa el provider configurado directamente o también delega en un CLI en modo headless?
4. ¿Múltiples harnesses por proyecto, o uno solo? (Default propuesto: uno, simple.)
5. Esquema exacto de `tareas.json` (campos mínimos: id, rol, fecha, título, informe, estado).
