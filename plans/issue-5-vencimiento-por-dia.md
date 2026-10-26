# Feature: fecha límite (solo día) en las tareas

Origen: issue #5 — https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5

## Encargo y criterios de aceptación

Texto de la issue:

> **Quiero añadir fecha límite a las tasks**
> Vale con que sea simplemente un día. Sin hora

Requisitos explícitos:

- Una tarea puede tener una fecha límite.
- La fecha límite es un día de calendario, sin hora.

Inferencias (decididas en este plan):

- La fecha límite es **opcional**: crear una tarea solo con título sigue funcionando y las tareas existentes quedan sin fecha.
- Se asigna al crear la tarea y se puede **cambiar o quitar** desde la pantalla de edición.
- La lista muestra la fecha de las tareas que la tienen, en formato `DD/MM/AAAA`.
- Se admiten fechas pasadas (no hay regla que las prohíba y bloquearían la edición de tareas antiguas).
- No se marca visualmente si una tarea está vencida ni cambia el orden de la lista.

Criterios de aceptación:

1. El formulario «Añadir una tarea…» incluye un campo de fecha (`<input type="date" name="due_date">`) opcional. Al enviar un título válido y `2026-11-15`, se crea la tarea con fecha límite 15/11/2026.
2. Al enviar solo un título (o la fecha vacía), la tarea se crea sin fecha límite, como hasta ahora.
3. En la lista, cada tarea con fecha muestra `Fecha límite: 15/11/2026` dentro de `<time class="task-due" datetime="2026-11-15">`; las tareas sin fecha no muestran ese elemento.
4. La pantalla de edición muestra el campo de fecha precargado. Guardar con otra fecha la cambia; guardarla vacía la quita. El estado completada/pendiente no cambia.
5. Un valor de fecha que no sea un día válido en formato `AAAA-MM-DD` (p. ej. `2026-02-30`, `mañana`) responde 422 con «La fecha límite no es válida.», no crea ni modifica la tarea y conserva el título introducido.
6. Si el título es inválido y la fecha válida, el 422 conserva la fecha en el campo (`value="2026-11-15"`).
7. Se guarda solo el día (columna PostgreSQL `date`) y el día leído coincide con el enviado, sin desplazamientos por zona horaria.
8. Completar o reabrir una tarea conserva su fecha límite.

## Comportamiento actual y propuesta

Actual (rama creada desde `origin/main`):

- Tabla `tasks` (`migrations/20260901000000_create_tasks.up.fizz`): `id`, `title` (200), `completed`, `created_at`, `updated_at`. No hay fecha límite.
- `models.Task` (`models/task.go`) con `Validate` que solo comprueba el título.
- Rutas en `actions/app.go`: `GET /`, `POST /tasks`, `GET /tasks/{task_id}/edit`, `PUT /tasks/{task_id}`, `PUT /tasks/{task_id}/toggle`, `DELETE /tasks/{task_id}`.
- `TasksCreate` hace `c.Bind(task)` (solo `title` tiene tag `form`); `TasksUpdate` enlaza un struct `input` con solo `Title` para no permitir asignación masiva.
- Templates Plush `templates/tasks/index.plush.html` (lista + formulario de alta) y `templates/tasks/edit.plush.html`. Ambos marcan el input de título como inválido con `errors.HasAny()` (asumen que el único error posible es el del título).
- CSS en `public/assets/app.css`, enlazado como `/assets/app.css?v=3` en `templates/application.plush.html`.

Propuesta (cambio vertical mínimo, siguiendo los patrones existentes):

- Migración Fizz que añade `due_date date NULL` a `tasks`.
- Campo `DueDate nulls.Time` en `models.Task` (no enlazado por formulario, `form:"-"`), más `SetDueDate(string)` que interpreta el valor del formulario, y `Validate` que informa del valor inválido en la clave `due_date`.
- `TasksCreate` y `TasksUpdate` llaman a `task.SetDueDate(c.Param("due_date"))` antes de validar y guardar; el resto del flujo (422 / redirect + flash) no cambia.
- Templates: campo de fecha en alta y edición, errores por campo, y fecha visible en la lista.
- CSS mínimo para el input de fecha y la línea de fecha bajo el título.

Por qué no se enlaza `due_date` con `c.Bind`: el decodificador de fechas de Buffalo v1.1.4 está registrado para su tipo interno `github.com/gobuffalo/buffalo/internal/nulls.Time`, no para `github.com/gobuffalo/nulls.Time`, y un valor inválido haría fallar `Bind` (500) en lugar de dar un error de validación 422.

## Alcance

Incluido:

- Persistencia: nueva columna nullable `due_date` de tipo `date` y su migración de bajada.
- Modelo: campo, asignación desde texto, validación y dos métodos de presentación.
- Acciones: crear y actualizar leen `due_date`.
- Vistas: campo en alta y edición, errores por campo, fecha en la lista.
- CSS para el input `date` y la línea de fecha; subida de `?v=3` a `?v=4` del CSS.
- Tests de modelo y de acciones.
- `go.mod`: `github.com/gobuffalo/nulls` pasa de dependencia indirecta a directa (misma versión v0.4.2; `go.sum` ya la contiene).

Excluido:

- Hora del día, zonas horarias del usuario, recordatorios o notificaciones.
- Marcar tareas vencidas, ordenar o filtrar por fecha límite.
- Cambios en `TasksToggle`, `TasksDestroy`, `public/assets/app.js`, rutas, fixtures (`fixtures/tasks.toml`) y migración de semillas (`migrations/20260901000001_seed_tasks.*.sql`).
- API JSON u otros clientes.

## Diseño por capas

### Persistencia y migraciones

- Nuevos archivos (embebidos automáticamente por `migrations/embed.go`, patrón `*.fizz`):
  - `migrations/20261026172800_add_due_date_to_tasks.up.fizz`:
    ```
    add_column("tasks", "due_date", "date", {"null": true})
    ```
  - `migrations/20261026172800_add_due_date_to_tasks.down.fizz`:
    ```
    drop_column("tasks", "due_date")
    ```
- Fizz traduce el tipo `date` tal cual para PostgreSQL (`DATE`). Las filas existentes quedan con `NULL` (sin fecha).
- Producción aplica la migración al arrancar (`models.Migrate()` en `cmd/app/main.go`); CI la aplica con `soda migrate -e test`.

### Modelo (`models/task.go`)

- Importar `github.com/gobuffalo/nulls`.
- Constante y campos:
  ```go
  // DueDateLayout es el formato de día que envía un input type="date".
  const DueDateLayout = "2006-01-02"
  ```
  En `Task`, entre `Completed` y `CreatedAt`:
  ```go
  DueDate   nulls.Time `json:"due_date" db:"due_date" form:"-"`
  ```
  Y al final del struct, un campo no persistido:
  ```go
  // dueDateInvalid indica que el último valor recibido en SetDueDate no era un día válido.
  dueDateInvalid bool `db:"-" form:"-"`
  ```
  El tag `db:"-"` es obligatorio: Pop trata como columna cualquier campo sin tag `db` (usa el nombre del campo), incluidos los no exportados.
- `func (t *Task) SetDueDate(value string)`:
  - Reinicia `t.DueDate = nulls.Time{}` y `t.dueDateInvalid = false`.
  - `strings.TrimSpace(value)`; si queda vacío, termina (sin fecha).
  - `time.Parse(DueDateLayout, value)`; si falla, `t.dueDateInvalid = true` y termina (fecha vacía).
  - Si es válido, `t.DueDate = nulls.NewTime(day)` (medianoche UTC del día).
- `Validate`: tras las comprobaciones del título, `if t.dueDateInvalid { verrs.Add("due_date", "La fecha límite no es válida.") }`.
- Métodos de presentación con **receptor por valor** (la plantilla los llama sobre los elementos de `models.Tasks`, que son valores, y sobre `*Task`):
  - `func (t Task) DueDateValue() string`: `""` si `!t.DueDate.Valid`; si no, `t.DueDate.Time.Format(DueDateLayout)` (valor para `<input type="date">` y atributo `datetime`).
  - `func (t Task) DueDateLabel() string`: `""` si no hay fecha; si no, `t.DueDate.Time.Format("02/01/2006")`.
- Comentarios en español, en la línea de los existentes.

### Acciones (`actions/tasks.go`)

- `TasksCreate`: tras `task.Title = strings.TrimSpace(task.Title)`, añadir `task.SetDueDate(c.Param("due_date"))`. `c.Param` incluye los valores del formulario (Buffalo los copia en `newContext`), igual que ya se usa para `task_id`.
- `TasksUpdate`: tras `task.Title = strings.TrimSpace(input.Title)`, añadir `task.SetDueDate(c.Param("due_date"))`. Actualizar el comentario de la función: «actualiza el título y la fecha límite, conservando los demás datos de la tarea.» Un `due_date` ausente o vacío quita la fecha (el formulario de edición siempre envía el campo).
- Sin cambios en rutas ni en el resto de acciones. `TasksToggle` ya guarda la tarea cargada completa, por lo que conserva la fecha.

### Renderizado (Plush)

`templates/tasks/index.plush.html`:

- En cada `<li class="task…">`, justo después de `<span class="task-title"><%= t.Title %></span>`:
  ```html
  <%= if (t.DueDate.Valid) { %>
    <time class="task-due" datetime="<%= t.DueDateValue() %>">Fecha límite: <%= t.DueDateLabel() %></time>
  <% } %>
  ```
  No meter la fecha dentro de `.task-title`: `app.js` usa su `textContent` para el `aria-label` del botón de completar, y el tachado de completadas se aplica a `.task-title`.
- Formulario de alta, dentro de `.new-task-row`, entre el input de título y el botón «Añadir»:
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
- Errores por campo: en el input de título sustituir `errors.HasAny()` por `len(errors.Get("title")) > 0` (mantiene `autofocus aria-invalid="true" aria-describedby="title-error"`). El bloque `<p id="title-error" …>` se muestra solo con `len(errors.Get("title")) > 0`, y se añade debajo un bloque equivalente:
  ```html
  <%= if (len(errors.Get("due_date")) > 0) { %>
    <p id="due-date-error" class="field-error" role="alert">
      <%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %>
    </p>
  <% } %>
  ```

`templates/tasks/edit.plush.html`:

- En el input de título, sustituir `errors.HasAny()` por `len(errors.Get("title")) > 0` (en ambos usos: atributos y bloque `#title-error`).
- Cambiar el texto de `#title-help` a «Hasta 200 caracteres. El estado de la tarea no cambia.» (la fecha ahora sí se edita en este formulario).
- Después del bloque de error del título y antes de `.edit-task-actions`:
  ```html
  <label for="task-due-date">Fecha límite (opcional)</label>
  <input
    id="task-due-date"
    type="date"
    name="due_date"
    value="<%= task.DueDateValue() %>"
    <%= if (len(errors.Get("due_date")) > 0) { %>aria-invalid="true" aria-describedby="due-date-error"<% } else { %>aria-describedby="due-date-help"<% } %>
  >
  <p id="due-date-help" class="field-hint">Solo el día, sin hora. Déjala vacía si la tarea no tiene fecha límite.</p>
  <%= if (len(errors.Get("due_date")) > 0) { %>
    <p id="due-date-error" class="field-error" role="alert">
      <%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %>
    </p>
  <% } %>
  ```

`templates/application.plush.html`: cambiar `/assets/app.css?v=3` por `/assets/app.css?v=4`.

### CSS (`public/assets/app.css`)

- Ampliar el selector `input[type="text"] {` a `input[type="text"],\ninput[type="date"] {` para heredar borde, altura y tipografía.
- Añadir tras `.new-task-row input { flex: 1; }`:
  ```css
  .new-task-row input[type="date"] {
    flex: 0 0 auto;
    width: auto;
  }
  ```
  (más específico que `.new-task-row input` y que la regla de `max-width: 380px`, por lo que el campo de fecha conserva su ancho natural).
- Añadir tras `.task-title { … }`:
  ```css
  .task-due {
    grid-column: 2;
    grid-row: 2;
    margin-top: -0.55rem;
    padding-bottom: 0.3rem;
    color: var(--color-muted);
    font-size: 0.8rem;
  }
  ```
  Con fila y columna explícitas, la fecha se coloca bajo el título y el botón de completar y las acciones siguen en la fila 1 sin tocar sus reglas (`.task-status` es absoluto y no participa en la rejilla).
- Añadir tras `.edit-task label { … }`:
  ```css
  .edit-task label:not(:first-of-type) {
    margin-top: 1.25rem;
  }
  .edit-task input[type="date"] {
    width: auto;
  }
  ```
- No hay cambios de JavaScript.

## Archivos relevantes

| Ruta | Responsabilidad | Cambio |
| --- | --- | --- |
| `migrations/20261026172800_add_due_date_to_tasks.up.fizz` | Añadir columna `due_date` | Nuevo |
| `migrations/20261026172800_add_due_date_to_tasks.down.fizz` | Revertir la columna | Nuevo |
| `migrations/embed.go` | Embebe `*.fizz` y `*.sql` | Sin cambios (referencia) |
| `models/task.go` | Modelo `Task` y validación | Campo, `SetDueDate`, `Validate`, `DueDateValue`, `DueDateLabel` |
| `models/task_test.go` | Tests del modelo | Nuevos tests |
| `actions/tasks.go` | Handlers de tareas | `SetDueDate` en crear y actualizar |
| `actions/tasks_test.go` | Tests HTTP | Nuevos tests y una aserción extra |
| `templates/tasks/index.plush.html` | Lista y alta | Fecha en lista, campo en alta, errores por campo |
| `templates/tasks/edit.plush.html` | Edición | Campo de fecha, errores por campo, texto de ayuda |
| `templates/application.plush.html` | Layout | `app.css?v=4` |
| `public/assets/app.css` | Estilos | Input `date`, `.task-due`, espaciado en edición |
| `go.mod` | Dependencias | `github.com/gobuffalo/nulls v0.4.2` a requisito directo |
| `actions/app.go` | Rutas | Sin cambios (referencia) |
| `public/assets/app.js` | Toggle optimista | Sin cambios (depende de `.task-title`) |

## Implementación paso a paso

1. Crear las dos migraciones Fizz con el contenido exacto de «Persistencia y migraciones».
2. Modificar `models/task.go` como se describe en «Modelo» (import de `nulls`, constante, campos con sus tags, `SetDueDate`, regla en `Validate`, `DueDateValue`, `DueDateLabel`).
3. Ejecutar `go mod tidy` y comprobar con `git diff go.mod go.sum` que el único cambio es mover `github.com/gobuffalo/nulls v0.4.2` del bloque `// indirect` al bloque directo. Si cambia cualquier otra línea, detenerse e informar.
4. Modificar `actions/tasks.go` (dos llamadas a `SetDueDate` y comentario de `TasksUpdate`).
5. Modificar `templates/tasks/index.plush.html`, `templates/tasks/edit.plush.html` y `templates/application.plush.html` según «Renderizado».
6. Modificar `public/assets/app.css` según «CSS».
7. Añadir los tests de «Tests y validación».
8. Preparar la base de test y ejecutar la validación.

## Tests y validación

### Tests de modelo (`models/task_test.go`)

Usan `ms.DB` como los existentes; no requieren nuevos imports salvo los necesarios.

- `Test_Task_DueDate_Optional`: crear `&Task{Title: "Sin fecha"}`; sin errores; recargar con `ms.DB.Find`; `ms.False(reloaded.DueDate.Valid)`; `ms.Equal("", reloaded.DueDateValue())`.
- `Test_Task_DueDate_PersistsDay`: `task.SetDueDate("2026-11-15")`; crear; recargar con `ms.DB.Find`; `ms.True(reloaded.DueDate.Valid)`; `ms.Equal("2026-11-15", reloaded.DueDateValue())`; `ms.Equal("15/11/2026", reloaded.DueDateLabel())`. (Cubre el criterio 7: el día no se desplaza en el viaje a la base de datos.)
- `Test_Task_SetDueDate_Invalid`: para `"2026-02-30"` y `"mañana"`: `task.SetDueDate(v)`; `ValidateAndCreate` devuelve error en `verrs.Get("due_date")` con «La fecha límite no es válida.»; el recuento de tareas es 0.
- `Test_Task_SetDueDate_EmptyClears`: `SetDueDate("2026-11-15")` y luego `SetDueDate("   ")`; `ms.False(task.DueDate.Valid)`; `Validate` sin errores.

### Tests de acciones (`actions/tasks_test.go`)

Añadir un helper junto a `taskForEditing`:

```go
func (as *ActionSuite) taskWithDueDate(day string) *models.Task {
	task := &models.Task{Title: "Tarea con fecha"}
	task.SetDueDate(day)
	verrs, err := as.DB.ValidateAndCreate(task)
	as.NoError(err)
	as.False(verrs.HasAny())
	as.NoError(as.DB.Reload(task))
	return task
}
```

- `Test_TasksCreate_Valid` (existente): añadir `as.False(task.DueDate.Valid)`.
- `Test_TasksCreate_WithDueDate`: `Post(url.Values{"title": {"Pagar el alquiler"}, "due_date": {"2026-11-15"}})` → 303 a `/`; `as.DB.First(task)`; `DueDateValue() == "2026-11-15"`.
- `Test_TasksCreate_InvalidDueDate`: `title` «Pagar el alquiler», `due_date` «2026-02-30» → 422; el cuerpo contiene «La fecha límite no es válida.», `aria-describedby="due-date-error"` y `value="Pagar el alquiler"`; no contiene `aria-describedby="title-error"`; recuento 0.
- `Test_TasksCreate_EmptyTitle_KeepsDueDate`: `title` vacío, `due_date` «2026-11-15» → 422; contiene «El título es obligatorio.» y `value="2026-11-15"`; recuento 0.
- `Test_TasksIndex_ShowsDueDate`: `taskWithDueDate("2026-11-15")` y una tarea sin fecha (`taskForEditing(false)`); `GET /` → 200; contiene `datetime="2026-11-15"` y `Fecha límite: 15/11/2026`; `strings.Count(body, `class="task-due"`)` es 1.
- `Test_TasksEdit_PrefillsDueDate`: `taskWithDueDate("2026-11-15")`; `GET /tasks/{id}/edit` → 200; contiene `name="due_date"` y `value="2026-11-15"`.
- `Test_TasksUpdate_ChangesDueDate`: tarea con fecha 2026-11-15 marcada como completada (`task.Completed = true; as.NoError(as.DB.Update(task))`); `PUT` con `title` «Pagar el alquiler» y `due_date` «2026-12-01» → 303; recargar; `DueDateValue() == "2026-12-01"`; `task.Completed` sigue `true`.
- `Test_TasksUpdate_ClearsDueDate`: tarea con fecha; `PUT` con `title` «Pagar el alquiler» y `due_date` vacío → 303; recargar; `as.False(task.DueDate.Valid)`.
- `Test_TasksUpdate_InvalidDueDate`: tarea con fecha 2026-11-15; `PUT` con `title` «Nuevo título» y `due_date` «mañana» → 422; contiene «La fecha límite no es válida.»; recargar; el título sigue siendo «Tarea con fecha» y la fecha «2026-11-15».
- `Test_TasksToggle_KeepsDueDate`: tarea con fecha; `PUT /tasks/{id}/toggle` → 303; recargar; `Completed` es `true` y `DueDateValue() == "2026-11-15"`.

Los tests existentes deben seguir pasando sin cambios (salvo la aserción añadida en `Test_TasksCreate_Valid`); en particular `Test_TasksUpdate_EmptyTitle` sigue encontrando `aria-describedby="title-error"`.

### Preparar la base de test

Los tests necesitan PostgreSQL (`docker compose up -d db`) y la base `todo_test` migrada con las migraciones **de esta rama**. En el checkout local, `todo_test` tiene aplicada una migración ajena a esta rama (`20261026180000`, de otra rama de la issue #5) que ya añadió `due_date`; aplicar encima la nueva migración fallaría con «column already exists». Recrear solo la base de test (sus datos se truncan en cada test; no tocar `todo_development`):

```sh
docker compose up -d db
SODA="$(go env GOPATH)/bin/soda"   # si no existe: go install github.com/gobuffalo/pop/v6/soda@v6.4.1
"$SODA" drop -e test
"$SODA" create -e test
"$SODA" migrate -e test
docker compose exec db psql -U postgres -d todo_test -c '\d tasks'   # due_date | date | nullable
"$SODA" migrate down -e test   # valida la migración de bajada (elimina due_date)
"$SODA" migrate -e test        # vuelve a aplicarla
```

### Comandos de validación

```sh
gofmt -l .                      # sin salida
go vet ./...
go mod tidy -diff               # sin salida
GO_ENV=test go test -p 1 ./...  # GO_ENV=test hace que models.DB (usado por las acciones) apunte a todo_test, como en CI
go build ./...
git diff --check
```

## Riesgos y recuperación

- **Desplazamiento del día por zona horaria**: `SetDueDate` produce medianoche UTC y pgx codifica/decodifica `date` con año, mes y día; `Test_Task_DueDate_PersistsDay` lo comprueba con un viaje real a PostgreSQL. Si fallara (día anterior), detenerse e informar en vez de compensar horas.
- **Estado de la base de test local**: contiene migraciones de otras ramas; por eso se recrea `todo_test` antes de validar. `todo_development` también tiene una migración ajena (`20261014000000`) y no se modifica; Pop ignora versiones registradas sin archivo, así que el arranque local aplicaría igualmente la nueva migración.
- **PRs paralelas de la misma issue**: hay otras PRs abiertas para la issue #5 con migraciones propias que añaden `due_date`. Solo una debe fusionarse; fusionar dos provocaría una migración duplicada sobre la misma columna.
- **Semántica de `PUT /tasks/{id}`**: un `due_date` ausente equivale a vacío y quita la fecha. El único cliente es el formulario de edición, que siempre envía el campo.
- **Navegadores sin `input type="date"`**: muestran un campo de texto; un valor mal escrito produce el 422 con mensaje claro, sin perder el título.
- **Recuperación**: la migración es reversible (`soda migrate down` elimina `due_date`, con pérdida de las fechas guardadas). El resto del cambio se revierte con `git revert` del commit.
