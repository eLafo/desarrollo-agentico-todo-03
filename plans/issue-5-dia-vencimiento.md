# Feature: fecha límite (solo día) en las tareas

## Encargo y criterios de aceptación

Origen: issue #5 — «Quiero añadir fecha límite a las tasks». Cuerpo: «Vale con que sea simplemente un día. Sin hora».

Requisitos explícitos:

- Una tarea puede tener una fecha límite.
- La fecha límite es un día (año, mes y día), sin hora.

Inferencias (no pedidas literalmente, necesarias para que la funcionalidad sea usable y coherente):

- La fecha límite es **opcional**: las tareas existentes y las nuevas sin fecha siguen siendo válidas.
- Se puede fijar al crear la tarea y también **cambiar o quitar** desde la pantalla de edición; si no, las tareas existentes nunca podrían recibir una fecha y un error no tendría arreglo.
- Se muestra en la lista junto a la tarea.
- Se admite cualquier día válido, incluido uno pasado (una tarea puede estar vencida).

Criterios de aceptación:

1. El formulario de «Añadir tarea» incluye un campo de fecha opcional (`<input type="date" name="due_date">`). Al enviarlo con `2026-11-15`, la tarea se guarda con esa fecha límite. Si se envía vacío, se guarda sin fecha.
2. En la lista, una tarea con fecha muestra `Fecha límite: <time datetime="2026-11-15">15/11/2026</time>`. Una tarea sin fecha no muestra nada de fecha.
3. La pantalla de edición muestra el campo de fecha relleno con el valor actual (`value="2026-11-15"`). Guardar otra fecha la cambia y guardarlo vacío la quita. El estado completada/pendiente no cambia.
4. Un valor que no sea exactamente un día `AAAA-MM-DD` (por ejemplo `15/11/2026`, `2026-13-40` o `2026-11-15T10:00`) se rechaza con HTTP 422 y el mensaje «La fecha límite debe ser un día válido.» asociado al campo de fecha (`aria-describedby="due-date-error"`). No se crea ni modifica nada.
5. Completar o reabrir una tarea conserva su fecha límite sin desplazarla de día.
6. La base de datos guarda la fecha en una columna `DATE` nullable. La migración es reversible.

## Comportamiento actual y propuesta

Actual (rama creada desde `origin/main`):

- `tasks` tiene `id`, `title`, `completed`, `created_at` y `updated_at` (`migrations/20260901000000_create_tasks.up.fizz`).
- `models.Task` no tiene ningún campo de fecha. `Validate` solo comprueba el título.
- `TasksCreate` hace `c.Bind(task)` (solo `title` tiene etiqueta `form`). `TasksUpdate` enlaza un struct anónimo con `Title` y solo cambia el título.
- `templates/tasks/index.plush.html` y `edit.plush.html` muestran el error con `errors.HasAny()`, porque hoy solo existe el error de `title`.
- Cada `.task` de la lista es un grid de 3 columnas (toggle | `.task-title` | `.task-actions`). `.task-status` queda fuera del flujo (sr-only, `position: absolute`).

Propuesta (cambio vertical mínimo, siguiendo los patrones existentes):

- Migración fizz que añade `due_date` como `date` nullable.
- `Task.DueDate *time.Time` (`db:"due_date"`, `form:"-"`). La conversión del texto del formulario la hace `Task.SetDueDate`, que solo acepta el layout `2006-01-02`. Un texto inválido se marca en un campo no persistido y `Validate` lo convierte en error de `due_date`. Así la validación sigue en el modelo, como la del título.
- Métodos de presentación con receptor por valor (`HasDueDate`, `DueDateValue`, `DueDateLabel`) para que Plush los invoque tanto sobre `Task` (iteración de `tasks`) como sobre `*Task` (`task`).
- `TasksCreate` y `TasksUpdate` enlazan un único struct `taskInput` (`title`, `due_date`) y llaman a `SetDueDate`.
- Las plantillas añaden el campo de fecha y pasan a comprobar los errores **por campo** (`len(errors.Get("title")) > 0`, `len(errors.Get("due_date")) > 0`).
- En la lista, el título y la fecha se envuelven en `<div class="task-main">`, que pasa a ser la celda central del grid.

No se usa el decodificador de tiempo del binder de Buffalo (`form:"due_date"` sobre un tipo tiempo). Ese decodificador acepta muchos formatos con hora (`2006-01-02T15:04`, RFC3339…), lo que contradice «sin hora». Además, un valor inválido haría fallar `c.Bind` con un 500 en lugar de un 422 con mensaje.

## Alcance

Incluido:

- Migración `up`/`down` de la columna `due_date`.
- Campo, parseo, validación y helpers de presentación en `models.Task`.
- Creación y edición con fecha límite (`TasksCreate`, `TasksUpdate`).
- Visualización en la lista y campo en los formularios de creación y edición.
- CSS mínimo para la nueva maquetación y el campo de fecha. Se incrementa la versión del CSS en el layout.
- Tests de modelo y de acciones descritos abajo.

Excluido:

- Ordenar o filtrar por fecha límite.
- Resaltar tareas vencidas o próximas a vencer, avisos o notificaciones.
- Fechas con hora o zona horaria.
- Cambios en `public/assets/app.js`. El toggle optimista sigue funcionando porque `.task-title`, `.task-status` y `.task-actions` mantienen sus clases y su relación con `.task`.
- Cambios en fixtures (`fixtures/tasks.toml`), en la migración de semillas o en dependencias (`go.mod`/`go.sum`). Se usa `*time.Time` de la librería estándar y no `nulls.Time`.
- API JSON. La etiqueta `json:"due_date"` se añade solo por coherencia con el resto de campos.

## Diseño por capas

### Persistencia y migraciones

Nueva migración `migrations/20261026000000_add_due_date_to_tasks`:

- `up.fizz`: `add_column("tasks", "due_date", "date", {"null": true})`. Fizz traduce `date` a `DATE` en Postgres (tipo por defecto de `colType`) y `{"null": true}` evita el `NOT NULL`.
- `down.fizz`: `drop_column("tasks", "due_date")`.

`migrations/embed.go` ya incluye `*.fizz`. `cmd/app/main.go` aplica las migraciones al arrancar con `models.Migrate()`. Las filas existentes quedan con `NULL`, es decir, sin fecha límite.

### Modelo (`models/task.go`)

- Constante `DueDateLayout = "2006-01-02"`: formato de día de `<input type="date">` y del atributo `datetime`.
- Campos:
  - `DueDate *time.Time` con etiquetas `json:"due_date" db:"due_date" form:"-"`, colocado tras `Completed`.
  - `dueDateInvalid bool` con etiquetas `json:"-" db:"-" form:"-"`. La etiqueta `db:"-"` es **obligatoria**: pop trata los campos sin etiqueta como columnas con el nombre del campo (`columns.TagsFor`), y sin ella el INSERT fallaría.
- `func (t *Task) SetDueDate(value string)`:
  - Recorta espacios.
  - Si el valor queda vacío, pone `DueDate = nil` y `dueDateInvalid = false`.
  - Si no, aplica `time.Parse(DueDateLayout, value)`. Si funciona, asigna `&day` y `dueDateInvalid = false`. El resultado está en UTC a medianoche.
  - Si falla, pone `dueDateInvalid = true` y deja `DueDate` sin cambios.
- `Validate`: además del título, si `t.dueDateInvalid` añade `verrs.Add("due_date", "La fecha límite debe ser un día válido.")`.
- Helpers con **receptor por valor**. Plush itera `tasks` como valores `Task` y solo encuentra los métodos de valor.
  - `func (t Task) HasDueDate() bool`: `t.DueDate != nil`.
  - `func (t Task) DueDateValue() string`: `t.DueDate.Format(DueDateLayout)` o `""`.
  - `func (t Task) DueDateLabel() string`: `t.DueDate.Format("02/01/2006")` o `""`.

Ningún helper convierte de zona horaria. Con pgx v5 (protocolo extendido, modo por defecto de pop), un `time.Time` se codifica como `DATE` usando su año, mes y día, y un `DATE` se lee como medianoche UTC. Por eso el día es estable en la ida y la vuelta. Los tests lo comprueban.

### Acciones (`actions/tasks.go`)

- Nuevo tipo a nivel de paquete, junto a las acciones:

  ```go
  // taskInput recoge los campos que el usuario puede escribir en una tarea.
  type taskInput struct {
  	Title   string `form:"title" json:"title"`
  	DueDate string `form:"due_date" json:"due_date"`
  }
  ```

- `TasksCreate`:
  - Sustituye `task := &models.Task{}; c.Bind(task)` por el enlace de `input := taskInput{}`.
  - Construye `task := &models.Task{Title: strings.TrimSpace(input.Title)}` y llama a `task.SetDueDate(input.DueDate)`.
  - El resto del flujo no cambia: `ValidateAndCreate`, re-render 422 con `tasks`, `task` y `errors`, flash y redirección 303.
- `TasksUpdate`:
  - Sustituye el struct anónimo por `taskInput`.
  - Tras asignar el título, llama a `task.SetDueDate(input.DueDate)`.
  - Actualiza el comentario: «TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.»
  - Un `due_date` vacío o ausente quita la fecha. El formulario de edición siempre envía el campo.
- Rutas: sin cambios.

### Renderizado (Plush)

`templates/tasks/index.plush.html`:

- En cada `<li class="task …">`, sustituye `<span class="task-title"><%= t.Title %></span>` por:

  ```html
  <div class="task-main">
    <span class="task-title"><%= t.Title %></span>
    <%= if (t.HasDueDate()) { %>
      <span class="task-due">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></span>
    <% } %>
  </div>
  ```

  El bloque `task-status` y `.task-actions` siguen siendo hijos directos del `<li>`, en el mismo orden que hoy.
- Formulario de creación, dentro de `.new-task-row`, entre el input del título y el botón «Añadir»:

  ```html
  <label for="task-due-date" class="sr-only">Fecha límite (opcional)</label>
  <input
    id="task-due-date"
    type="date"
    name="due_date"
    value="<%= task.DueDateValue() %>"
    title="Fecha límite (opcional)"
    <%= if (len(errors.Get("due_date")) > 0) { %>aria-invalid="true" aria-describedby="due-date-error"<% } %>
  >
  ```

- El input del título cambia la condición `errors.HasAny()` por `len(errors.Get("title")) > 0`.
- El bloque de error final se divide en dos:
  - `<p id="title-error" …>` solo si hay errores de `title`.
  - `<p id="due-date-error" class="field-error" role="alert">` con `errors.Get("due_date")`, solo si hay errores de `due_date`.

`templates/tasks/edit.plush.html`:

- Input del título:
  - La condición `errors.HasAny()` pasa a ser `len(errors.Get("title")) > 0`. Mantiene `aria-describedby="title-error"` con error y `"title-help"` sin error.
  - El texto de ayuda pasa a «Hasta 200 caracteres. El estado de la tarea no cambia.».
  - El bloque `title-error` se muestra solo con errores de `title`.
- Después del error del título y antes de `.edit-task-actions`:

  ```html
  <label for="task-due-date">Fecha límite</label>
  <input
    id="task-due-date"
    type="date"
    name="due_date"
    value="<%= task.DueDateValue() %>"
    <%= if (len(errors.Get("due_date")) > 0) { %>aria-invalid="true" aria-describedby="due-date-error"<% } else { %>aria-describedby="due-date-help"<% } %>
  >
  <p id="due-date-help" class="field-hint">Opcional. Solo el día, sin hora. Déjala vacía para quitarla.</p>
  <%= if (len(errors.Get("due_date")) > 0) { %>
    <p id="due-date-error" class="field-error" role="alert">
      <%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %>
    </p>
  <% } %>
  ```

`templates/application.plush.html`: cambia `/assets/app.css?v=3` por `/assets/app.css?v=4`.

### CSS (`public/assets/app.css`)

- `.task-title`:
  - Mueve `align-self: center;`, `min-width: 0;` y `padding: 0.65rem 0;` a una nueva regla `.task-main`.
  - En `.task-title` deja `display: block; overflow-wrap: anywhere; line-height: 1.55;`.
- Nueva regla `.task-due { display: block; margin-top: 0.15rem; color: var(--color-muted); font-size: 0.8rem; }`.
- Selector de inputs: `input[type="text"]` pasa a `input[type="text"],\ninput[type="date"]`.
- Nueva regla `.new-task-row input[type="date"] { flex: 0 0 auto; width: auto; }`. Evita que el campo de fecha herede `flex: 1` de `.new-task-row input` y el `flex-basis` del media query de 380px.
- Nueva regla `.edit-task label ~ label { margin-top: 1.25rem; }`. Separa el segundo campo del formulario de edición.
- `.task-completed .task-title` y el media query `.task-title { font-size: 0.9rem; }` no cambian.

## Archivos relevantes

| Ruta | Responsabilidad |
| --- | --- |
| `migrations/20261026000000_add_due_date_to_tasks.up.fizz` | Nuevo: añade la columna `due_date DATE NULL`. |
| `migrations/20261026000000_add_due_date_to_tasks.down.fizz` | Nuevo: elimina la columna. |
| `models/task.go` | `DueDate`, `SetDueDate`, validación y helpers de presentación. |
| `models/task_test.go` | Tests de modelo de la fecha límite. |
| `actions/tasks.go` | `taskInput` y uso de `SetDueDate` en crear y actualizar. |
| `actions/tasks_test.go` | Tests HTTP de creación, lista, edición, actualización y toggle con fecha. |
| `templates/tasks/index.plush.html` | Muestra la fecha en la lista y añade el campo al formulario de creación. Errores por campo. |
| `templates/tasks/edit.plush.html` | Campo de fecha editable y errores por campo. |
| `templates/application.plush.html` | Cache-busting del CSS (`v=4`). |
| `public/assets/app.css` | Maquetación de `.task-main`/`.task-due` y estilo del input de fecha. |
| `migrations/embed.go`, `cmd/app/main.go` | Sin cambios. Embeben y aplican las migraciones. |
| `public/assets/app.js` | Sin cambios. Depende de `.task-title`, `.task-status` y `.task-actions`. |

## Implementación paso a paso

1. Crea las dos migraciones fizz con el contenido indicado en «Persistencia y migraciones».
2. En `models/task.go`, añade `DueDateLayout`, los campos `DueDate` y `dueDateInvalid`, `SetDueDate`, la comprobación en `Validate` y los tres helpers con receptor por valor. Cada uno lleva un comentario en español, como el resto del archivo. No hacen falta imports nuevos (`strings` y `time` ya están).
3. En `actions/tasks.go`, añade `taskInput` y adapta `TasksCreate` y `TasksUpdate` según «Acciones». No toques `TasksIndex`, `TasksEdit`, `TasksToggle` ni `TasksDestroy`.
4. Actualiza `templates/tasks/index.plush.html` y `templates/tasks/edit.plush.html` según «Renderizado». Sube la versión del CSS en `templates/application.plush.html`.
5. Aplica los cambios de CSS en `public/assets/app.css`.
6. Añade los tests de la sección siguiente.
7. Prepara la base de datos de test y ejecuta la validación completa.

## Tests y validación

### Tests de modelo (`models/task_test.go`)

- `Test_Task_DueDate_OptionalByDefault`: crea una tarea sin fecha, la recarga con `Find` y comprueba `ms.Nil(reloaded.DueDate)` y `ms.False(reloaded.HasDueDate())`.
- `Test_Task_SetDueDate_PersistsDay`: aplica `SetDueDate("2026-11-15")` y `ValidateAndCreate` sin errores. Recarga y comprueba:
  - `DueDateValue() == "2026-11-15"`.
  - `DueDateLabel() == "15/11/2026"`.
  - `Hour()`, `Minute()` y `Second()` son 0.
- `Test_Task_SetDueDate_EmptyClears`: con fecha previa, `SetDueDate("")` deja `DueDate == nil`, y `Validate` no da errores.
- `Test_Task_SetDueDate_Invalid`: prueba en bucle `"15/11/2026"`, `"2026-13-40"`, `"2026-11-15T10:00"` y `"mañana"`. En cada caso, `ValidateAndCreate` devuelve `verrs.Get("due_date")` con «La fecha límite debe ser un día válido.». Al final, `Count` es 0.

### Tests de acciones (`actions/tasks_test.go`)

- `Test_TasksCreate_Valid` (existente): añade `as.Nil(task.DueDate)`.
- `Test_TasksCreate_WithDueDate`: `Post(url.Values{"title": {"Con fecha"}, "due_date": {"2026-11-15"}})` responde 303. La primera tarea tiene `DueDateValue() == "2026-11-15"`.
- `Test_TasksCreate_InvalidDueDate`: título válido y `due_date` `"15/11/2026"`. Comprueba:
  - Responde 422.
  - El cuerpo contiene «La fecha límite debe ser un día válido.», `aria-describedby="due-date-error"` y `value="Con fecha inválida"` (se conserva el título).
  - El cuerpo **no** contiene `id="title-error"`.
  - `Count` es 0.
- `Test_TasksIndex_ShowsDueDate`: crea una tarea con `SetDueDate("2026-11-15")` y otra sin fecha. `GET /` contiene `Fecha límite:`, `datetime="2026-11-15"` y `15/11/2026`. Además, `strings.Count(body, `class="task-due"`)` es 1.
- `Test_TasksEdit_PrefillsDueDate`: con una tarea con fecha, `GET /tasks/{id}/edit` contiene `name="due_date"` y `value="2026-11-15"`.
- `Test_TasksUpdate_ChangesDueDate`: parte de una tarea completada con fecha y envía `PUT` con `title` y `due_date` `"2026-12-01"`. Responde 303. Tras recargar, la fecha es `"2026-12-01"` y `Completed` sigue siendo `true`.
- `Test_TasksUpdate_ClearsDueDate`: con una tarea con fecha, envía `PUT` con `title` y `due_date` `""`. Responde 303 y, tras recargar, `DueDate` es `nil`.
- `Test_TasksUpdate_InvalidDueDate`: con una tarea con fecha `2026-11-15`, envía `PUT` con `due_date` `"2026-11-15T10:00"`. Responde 422 y el cuerpo contiene el mensaje. Tras recargar, el título y la fecha no han cambiado.
- `Test_TasksToggle_KeepsDueDate`: con una tarea con fecha `2026-11-15`, ejecuta el toggle dos veces (PUT). Tras cada recarga, `DueDateValue()` sigue siendo `"2026-11-15"`.

Para crear tareas con fecha en los tests, sigue el patrón de `taskForEditing`: construye la tarea, llama a `SetDueDate` y después a `ValidateAndCreate`. Puedes añadir un helper `taskWithDueDate(day string, completed bool) *models.Task` en `actions/tasks_test.go`.

Los tests existentes deben seguir pasando sin cambios, salvo la aserción añadida a `Test_TasksCreate_Valid`. En particular, `Test_TasksUpdate_EmptyTitle` sigue comprobando `aria-describedby="title-error"`.

### Condiciones del entorno

- PostgreSQL con `docker compose up -d db` (servicio `db` en `127.0.0.1:5432`).
- `soda` y `buffalo` no están en el `PATH`, pero existen en `$(go env GOPATH)/bin`. Usa `"$(go env GOPATH)/bin/soda"`. La CI instala `soda@v6.4.1`.
- `GO_ENV=test` es **obligatorio** al ejecutar `go test`. Sin él, `models.init` conecta con `development` y la suite truncaría las tablas de desarrollo.
- La base `todo_test` local es compartida y contiene migraciones de otras ramas que no existen en esta: `20261026211500`, `20261026230000` y una columna `due_date` ya creada. Con ese estado, `soda migrate` fallaría al añadir la columna. Antes de migrar, recréala. Es una base de test cuyo contenido la suite trunca en cada ejecución.

  ```bash
  docker compose exec -T db dropdb -U postgres --if-exists todo_test
  docker compose exec -T db createdb -U postgres todo_test
  ```

  No toques `todo_development` ni otras bases.

### Comandos de validación

```bash
gofmt -l .                                              # sin salida
go vet ./...
GO_ENV=test "$(go env GOPATH)/bin/soda" migrate -e test
GO_ENV=test "$(go env GOPATH)/bin/soda" migrate down -e test   # revierte la nueva migración
GO_ENV=test "$(go env GOPATH)/bin/soda" migrate -e test        # la vuelve a aplicar
GO_ENV=test go test -p 1 ./...
go build ./...
git diff --check
```

Tras el `migrate down`, `\d tasks` en `todo_test` no debe tener `due_date`. Tras volver a aplicar, debe tenerla como `date` nullable. Compruébalo con `docker compose exec -T db psql -U postgres -d todo_test -c '\d tasks'`.

No arranques la aplicación contra `todo_development`. Al arrancar se aplican las migraciones y esa base compartida ya contiene cambios de otras ramas.

## Riesgos y recuperación

- **Base de test con esquema ajeno.** Ya se ha comprobado en la preparación. Si `soda migrate` falla con «column already exists» o con migraciones desconocidas, recrea `todo_test` como se indica arriba. Si falla por otro motivo, detente e informa.
- **Desplazamiento de día por zona horaria.** El diseño evita conversiones: se parsea en UTC y se formatea el valor leído tal cual. Si `Test_Task_SetDueDate_PersistsDay` o `Test_TasksToggle_KeepsDueDate` muestran un día desplazado, el supuesto sobre pgx no se cumple. En ese caso, detente e informa sin cambiar el tipo de columna.
- **Métodos en Plush.** Los helpers deben tener receptor por valor. Si Plush no los encuentra (error de render «could not find …» en los tests de lista), revisa el receptor antes que la plantilla.
- **Campo no persistido en pop.** Si el INSERT falla con una columna `dueDateInvalid`, falta la etiqueta `db:"-"`.
- **Maquetación.** Los tests HTTP no verifican visualmente la fila con `.task-main` ni el input de fecha en móvil (≤760px y ≤380px). Revísalo en la revisión del PR.
- **Semántica de edición.** Un `PUT` sin `due_date` quita la fecha. Es correcto para el formulario, que siempre envía el campo, pero conviene conocerlo si en el futuro hay otros clientes.
- **Recuperación.** Para revertir: `soda migrate down` elimina la columna (no hay pérdida de datos previos a esta funcionalidad) y se revierte el commit.
