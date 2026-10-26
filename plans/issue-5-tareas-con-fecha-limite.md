# Feature: fecha límite (solo día) en las tareas

## Encargo y criterios de aceptación

Issue #5 (https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5): «Quiero añadir fecha límite a las tasks». Cuerpo: «Vale con que sea simplemente un día. Sin hora».

Requisitos explícitos:

- Una tarea puede tener una fecha límite.
- La fecha límite es un día, sin hora.

Inferencias (decisiones de este plan, no pedidas literalmente):

- La fecha límite es **opcional**: las tareas existentes no tienen y crear una tarea sin fecha debe seguir funcionando igual.
- Se puede indicar al crear la tarea y poner, cambiar o quitar al editarla.
- Se muestra en la lista de tareas en formato español `DD/MM/AAAA`.
- Se admite cualquier fecha válida, también pasadas (editar una tarea vencida no debe fallar).

Criterios de aceptación:

1. El formulario de creación de `/` incluye un campo de fecha opcional (`<input type="date" name="due_date">`). Al crear una tarea con fecha `2026-11-05`, se guarda y la lista muestra `Fecha límite: 05/11/2026` debajo del título, dentro de un `<time datetime="2026-11-05">`.
2. Crear una tarea con el campo de fecha vacío o ausente la guarda sin fecha límite; la lista no muestra ningún texto de fecha para ella.
3. La página de edición muestra el campo de fecha precargado con la fecha actual (`value="2026-11-05"`) o vacío. Al guardar se puede cambiar o quitar (campo vacío). El estado completada, el `id` y `created_at` no cambian.
4. La fecha se persiste como día sin hora (columna PostgreSQL `date`, nullable) y se lee igual que se escribió, sin desplazamiento de día.
5. Un valor de fecha que no sea `AAAA-MM-DD` válido (solo posible fuera del navegador) responde `422` con el mensaje «La fecha límite no es válida.» y no crea ni modifica la tarea.
6. Si el título es inválido, el formulario se vuelve a mostrar con `422` y conserva la fecha introducida.
7. Las tareas existentes quedan sin fecha límite tras aplicar la migración.

## Comportamiento actual y propuesta

Estado observado en `origin/main`:

- `migrations/20260901000000_create_tasks.up.fizz` crea `tasks` con `id`, `title` (varchar 200), `completed` y timestamps. `migrations/20260901000001_seed_tasks.up.sql` inserta tres tareas sin más columnas. `migrations/embed.go` embebe `*.fizz *.sql`, y `cmd/app/main.go` aplica las migraciones al arrancar mediante `models.Migrate()`.
- `models/task.go` define `Task` con `ID`, `Title`, `Completed`, `CreatedAt`, `UpdatedAt` y `Validate` (título obligatorio, máximo 200 runas).
- `actions/app.go` registra `GET /`, `POST /tasks`, `GET /tasks/{task_id}/edit`, `PUT /tasks/{task_id}`, `PUT /tasks/{task_id}/toggle` y `DELETE /tasks/{task_id}`.
- `actions/tasks.go`: `TasksCreate` hace `c.Bind(task)` sobre `models.Task` (los campos protegidos llevan `form:"-"`). `TasksUpdate` hace bind sobre un struct anónimo que solo tiene `Title`, para no asignar campos protegidos. Ambos renderizan `422` con `errors` si hay errores de validación.
- `templates/tasks/index.plush.html` pinta cada tarea como `<li class="task">` en un grid de 3 columnas (toggle, `.task-title`, `.task-actions`) con `.task-status` oculta visualmente. El formulario de creación es una fila `.new-task-row` con `+`, input de texto y botón. Los errores se muestran si `errors.HasAny()` y solo se leen los de `title`.
- `templates/tasks/edit.plush.html` tiene un único campo `title`, con la ayuda «Hasta 200 caracteres. Los demás datos de la tarea no cambian.».
- `public/assets/app.js` usa `.task-title` y `.task-actions` como hijos de `.task`, que deben seguir existiendo. `templates/application.plush.html` carga `app.css?v=3`.

No existe ningún concepto de fecha límite.

Propuesta: añadir la columna nullable `tasks.due_date` (tipo `date`), el campo `DueDate nulls.Time` en el modelo, un parser `AAAA-MM-DD` en el modelo y un binding común de formulario (`title` + `due_date`) para crear y actualizar. La fecha se muestra en la lista y se edita con `<input type="date">` en los dos formularios.

## Alcance

Incluido:

- Migración Fizz que añade `due_date date NULL` y su reversión.
- Campo `DueDate`, parser y helpers de presentación en `models.Task`.
- Lectura de `due_date` en `TasksCreate` y `TasksUpdate` con un binding común que sustituye a los actuales.
- Campo de fecha en los formularios de creación y edición, errores por campo y visualización en la lista.
- CSS para el nuevo campo y la línea de fecha, más el bump de caché `app.css?v=4`.
- Tests de modelo y de acciones.

Excluido:

- Hora, zona horaria del usuario, recordatorios y notificaciones.
- Resaltado de tareas vencidas o próximas («hoy», «vencida»), ordenación y filtrado por fecha.
- Cambios en `public/assets/app.js`, en el toggle, en el borrado, en la migración seed y en `fixtures/tasks.toml`.
- API JSON y cualquier otra ruta nueva.
- La corrección local del toggle (`429e9d6`, presente en `main` local pero no en `origin/main`); no forma parte de este encargo.

## Diseño por capas

### Persistencia y migración

Nueva migración `migrations/20261026210000_add_due_date_to_tasks.{up,down}.fizz`. Usa una versión propia, posterior a las existentes y distinta de la `20261026000000` que han usado otras ramas.

- up: `add_column("tasks", "due_date", "date", {"null": true})`. El traductor Postgres de Fizz pasa `date` tal cual, así que el tipo resultante es `DATE` nullable.
- down: `drop_column("tasks", "due_date")`.

`migrations/embed.go` ya incluye `*.fizz` y no cambia.

### Modelo (`models/task.go`)

- Importa `github.com/gobuffalo/nulls`. Ya figura en `go.mod` como `// indirect` (v0.4.2) y en `go.sum`, así que no se añade ninguna dependencia nueva.
- Constantes:
  - `DueDateLayout = "2006-01-02"` (exportada): formato `AAAA-MM-DD` de `<input type="date">`.
  - `dueDateLabelLayout = "02/01/2006"`: formato de presentación `DD/MM/AAAA`.
- Campo nuevo, entre `Completed` y `CreatedAt`:
  `DueDate nulls.Time `json:"due_date" db:"due_date" form:"-"``
  No añadas campos no exportados a `Task`: `gobuffalo/httptest` llama a `Interface()` sobre todos los campos cuando los tests envían `&models.Task{...}` y fallaría con un campo no exportado. Además, Pop trata cualquier campo sin etiqueta `db` como columna.
- `func ParseDueDate(value string) (nulls.Time, error)`: aplica `strings.TrimSpace`. Una cadena vacía devuelve `nulls.Time{}, nil` (sin fecha límite). En otro caso usa `time.Parse(DueDateLayout, value)`, que da medianoche UTC del día indicado, y devuelve `nulls.NewTime(d)`. Si falla, devuelve el error.
- `func (t Task) DueDateValue() string`: devuelve `""` si `!t.DueDate.Valid`; si no, `t.DueDate.Time.Format(DueDateLayout)`. Sirve para el `value` de los inputs y el atributo `datetime`.
- `func (t Task) DueDateLabel() string`: igual que el anterior, pero con `dueDateLabelLayout`.
- Usa receptor por valor en estos dos métodos. Plush los invoca sobre elementos `Task` (no punteros) en el `for` de la lista, y sobre `*Task` en la edición; el receptor por valor funciona en ambos casos.
- `Validate` no cambia.

### Acciones (`actions/tasks.go`)

Añade un formulario común y un helper. Siguen el patrón actual de `TasksUpdate`: bind sobre un struct propio para no asignar campos protegidos.

```go
// taskForm recoge los campos de una tarea que envían los formularios.
type taskForm struct {
	Title   string `form:"title" json:"title"`
	DueDate string `form:"due_date" json:"due_date"`
}

// bindTaskForm copia en la tarea el título y la fecha límite del formulario.
// Si la fecha límite no es válida, conserva la anterior y la devuelve como error de validación.
func bindTaskForm(c buffalo.Context, task *models.Task) (*validate.Errors, error) {
	form := taskForm{}
	if err := c.Bind(&form); err != nil {
		return nil, err
	}
	task.Title = strings.TrimSpace(form.Title)

	verrs := validate.NewErrors()
	dueDate, err := models.ParseDueDate(form.DueDate)
	if err != nil {
		verrs.Add("due_date", "La fecha límite no es válida.")
		return verrs, nil
	}
	task.DueDate = dueDate
	return verrs, nil
}
```

- `TasksCreate`: sustituye `task := &models.Task{}` + `c.Bind(task)` + `TrimSpace` por `task := &models.Task{}` y `verrs, err := bindTaskForm(c, task)` (devuelve `err` si no es nil). Después, solo `if !verrs.HasAny()`, ejecuta `verrs, err = tx.ValidateAndCreate(task)` y devuelve `err` si no es nil. El bloque `if verrs.HasAny() { … 422 … }` y la redirección no cambian.
- `TasksUpdate`: tras el `tx.Find`, sustituye el struct anónimo, el bind y el `TrimSpace` por `verrs, err := bindTaskForm(c, task)`. Luego aplica el mismo patrón: `ValidateAndUpdate` solo si no hay errores de formulario. El bloque `422` y la redirección no cambian. Actualiza el comentario de la función: «TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.».
- Un `due_date` ausente equivale a vacío: la tarea queda sin fecha límite. Los dos formularios envían siempre el campo.
- Con una fecha inválida no se intenta guardar y solo se informa del error de fecha. En la edición, `task.DueDate` conserva el valor cargado de la base de datos, que es el que se vuelve a mostrar.

Las rutas de `actions/app.go` no cambian.

### Templates

`templates/tasks/index.plush.html`, lista. Envuelve el título en un contenedor y añade la fecha debajo. `.task-status` y `.task-actions` siguen siendo hijos directos de `<li>`, porque `app.js` inserta el estado antes de `.task-actions`:

```html
<div class="task-main">
  <span class="task-title"><%= t.Title %></span>
  <%= if (t.DueDateValue() != "") { %>
    <span class="task-due">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></span>
  <% } %>
</div>
```

`templates/tasks/index.plush.html`, formulario de creación:

- En el input de título, cambia la condición `errors.HasAny()` por `len(errors.Get("title")) > 0`, tanto para `autofocus aria-invalid aria-describedby="title-error"` como para el bloque `<p id="title-error">`.
- Tras el input de título y antes del botón, añade:
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
- Tras el bloque de error del título, añade `<%= if (len(errors.Get("due_date")) > 0) { %><p id="due-date-error" class="field-error" role="alert"><%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %></p><% } %>`.

`templates/tasks/edit.plush.html`:

- Envuelve el bloque del título (label, input, ayuda y error) en `<div class="form-field">`. Cambia sus condiciones `errors.HasAny()` por `len(errors.Get("title")) > 0` y su ayuda por «Hasta 200 caracteres.».
- Añade un segundo `<div class="form-field">` antes de `.edit-task-actions`:
  ```html
  <label for="task-due-date">Fecha límite</label>
  <input
    id="task-due-date"
    type="date"
    name="due_date"
    value="<%= task.DueDateValue() %>"
    <%= if (len(errors.Get("due_date")) > 0) { %>aria-invalid="true" aria-describedby="due-date-error"<% } else { %>aria-describedby="due-date-help"<% } %>
  >
  <p id="due-date-help" class="field-hint">Opcional. Déjala vacía si la tarea no tiene fecha límite.</p>
  <%= if (len(errors.Get("due_date")) > 0) { %>
    <p id="due-date-error" class="field-error" role="alert"><%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %></p>
  <% } %>
  ```

`templates/application.plush.html`: cambia `app.css?v=3` por `app.css?v=4`.

### CSS (`public/assets/app.css`)

- Sustituye la regla `.task-title` por:
  ```css
  .task-main {
    align-self: center;
    min-width: 0;
    padding: 0.65rem 0;
  }

  .task-title {
    display: block;
    overflow-wrap: anywhere;
    line-height: 1.55;
  }

  .task-due {
    display: block;
    margin-top: 0.1rem;
    color: var(--color-muted);
    font-size: 0.8rem;
  }
  ```
  Las reglas `.task-completed .task-title` y `.task-title` del media query de 760px no cambian.
- Amplía el selector `input[type="text"]` a `input[type="text"],\ninput[type="date"]`.
- Cambia `.new-task-row input { flex: 1; }` por `.new-task-row input[type="text"] { flex: 1; }` y añade `.new-task-row input[type="date"] { width: auto; }`.
- En el media query de 380px, cambia `.new-task-row input { flex-basis: calc(100% - 44px); }` por `.new-task-row input[type="text"] { … }`. Así la fecha y el botón pasan juntos a la segunda línea.
- Añade `.form-field + .form-field { margin-top: 1.5rem; }`.

## Archivos relevantes

| Ruta | Responsabilidad |
| --- | --- |
| `migrations/20261026210000_add_due_date_to_tasks.up.fizz` (nuevo) | Añade `tasks.due_date date NULL` |
| `migrations/20261026210000_add_due_date_to_tasks.down.fizz` (nuevo) | Elimina la columna |
| `models/task.go` | Campo `DueDate`, `ParseDueDate`, `DueDateValue`, `DueDateLabel` |
| `models/task_test.go` | Tests de persistencia y parseo de la fecha |
| `actions/tasks.go` | `taskForm`, `bindTaskForm`, uso en `TasksCreate` y `TasksUpdate` |
| `actions/tasks_test.go` | Tests HTTP de creación, edición, listado y errores |
| `templates/tasks/index.plush.html` | Fecha en la lista, campo en el formulario de creación y errores por campo |
| `templates/tasks/edit.plush.html` | Campo de fecha en la edición y errores por campo |
| `templates/application.plush.html` | Bump `app.css?v=4` |
| `public/assets/app.css` | Estilos de `.task-main`, `.task-due`, `input[type="date"]` y `.form-field` |
| `go.mod` | `github.com/gobuffalo/nulls` pasa de indirecta a directa |
| `actions/app.go`, `public/assets/app.js`, `fixtures/tasks.toml`, `migrations/embed.go` | Solo lectura: no cambian |

## Implementación paso a paso

1. Crea los dos ficheros de migración con el contenido indicado en «Persistencia y migración».
2. Modifica `models/task.go`: imports (`github.com/gobuffalo/nulls`), constantes, campo `DueDate`, `ParseDueDate`, `DueDateValue` y `DueDateLabel`, cada uno con un comentario breve en español como el resto del archivo.
3. Ejecuta `go mod tidy -diff`. El único cambio esperado es mover `github.com/gobuffalo/nulls v0.4.2` del bloque `// indirect` al bloque de dependencias directas. Si coincide, aplica `go mod tidy`. Si propone otros cambios, no lo apliques: mueve solo esa línea a mano. `go.sum` no debe cambiar.
4. Modifica `actions/tasks.go` según «Acciones».
5. Modifica `templates/tasks/index.plush.html`, `templates/tasks/edit.plush.html` y `templates/application.plush.html` según «Templates».
6. Modifica `public/assets/app.css` según «CSS».
7. Añade los tests de «Tests y validación».
8. Prepara la base de datos de test y ejecuta las validaciones.

## Tests y validación

### Tests de modelo (`models/task_test.go`)

- `Test_Task_DueDate_OptionalByDefault`: crea `&Task{Title: "Sin fecha"}`, recarga con `ms.DB.Find` y comprueba `False(reloaded.DueDate.Valid)`, `DueDateValue() == ""` y `DueDateLabel() == ""`.
- `Test_Task_DueDate_Persists`: obtiene `d, err := ParseDueDate("2026-11-05")` (sin error). Crea la tarea con `DueDate: d` y recarga. Comprueba `DueDateValue() == "2026-11-05"` y `DueDateLabel() == "05/11/2026"`, lo que demuestra que no hay desplazamiento de día.
- `Test_Task_DueDate_CanBeCleared`: crea la tarea con fecha, asigna `nulls.Time{}`, hace `Update`, recarga y comprueba `False(DueDate.Valid)`.
- `Test_ParseDueDate`, sin BD:
  - `""` y `"   "` devuelven `Valid == false` sin error.
  - `" 2026-11-05 "` es válido.
  - `"2026-02-30"`, `"05/11/2026"`, `"2026-11-05T10:00"` y `"mañana"` devuelven error.

### Tests de acciones (`actions/tasks_test.go`)

Para crear tareas con fecha en los tests, usa `nulls.NewTime(time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC))` con `as.DB.ValidateAndCreate`, o `models.ParseDueDate`.

- `Test_TasksCreate_WithDueDate`: `Post(url.Values{"title": {"Con fecha"}, "due_date": {"2026-11-05"}})` devuelve `303`. `as.DB.First(task)` y `DueDateValue() == "2026-11-05"`.
- `Test_TasksCreate_EmptyDueDate`: `"due_date": {""}` devuelve `303` y la tarea queda con `False(DueDate.Valid)`.
- Añade `as.False(task.DueDate.Valid)` a `Test_TasksCreate_Valid`, que no envía `due_date`.
- `Test_TasksCreate_InvalidDueDate`: `"due_date": {"2026-13-40"}` devuelve `422`, el cuerpo contiene «La fecha límite no es válida.» y `id="due-date-error"`, y el número de tareas es 0.
- `Test_TasksCreate_EmptyTitle_KeepsDueDate`: título vacío y fecha `2026-11-05` devuelven `422`, «El título es obligatorio.», `value="2026-11-05"` y 0 tareas.
- `Test_TasksIndex_ShowsDueDate`: una tarea con fecha. `GET /` devuelve `200` y el cuerpo contiene `class="task-due"`, `datetime="2026-11-05"` y `05/11/2026`.
- `Test_TasksIndex_WithoutDueDate`: una tarea sin fecha. El cuerpo no contiene `class="task-due"`.
- `Test_TasksEdit_PrefillsDueDate`: una tarea con fecha. `GET /tasks/{id}/edit` contiene `name="due_date"` y `value="2026-11-05"`.
- `Test_TasksUpdate_ChangesDueDate`: una tarea completada con fecha. `PUT` con título y `"due_date": {"2026-12-01"}` devuelve `303`. Tras recargar: `DueDateValue() == "2026-12-01"`, `Completed` sigue en `true` y el título está actualizado.
- `Test_TasksUpdate_ClearsDueDate`: una tarea con fecha. `PUT` con título y `"due_date": {""}` devuelve `303` y, tras recargar, `False(DueDate.Valid)`.
- `Test_TasksUpdate_InvalidDueDate`: una tarea con fecha. `PUT` con `"title": {"Otro"}, "due_date": {"mañana"}` devuelve `422` con «La fecha límite no es válida.». Tras recargar, el título sigue siendo el original y la fecha sigue siendo `2026-11-05`.

Los tests existentes (`Test_TasksUpdate_PersistsTitleOnly`, `Test_TasksUpdate_EmptyTitle`, que espera `aria-describedby="title-error"`, y el resto) deben seguir pasando sin cambios, salvo la línea añadida a `Test_TasksCreate_Valid`.

### Preparación de la base de datos de test

Los tests necesitan PostgreSQL en `127.0.0.1:5432`: el contenedor `db` de `compose.yaml`; arráncalo con `docker compose up -d db` si no responde. También necesitan la base `todo_test` migrada con **solo** las migraciones de esta rama. Hay que vigilar dos cosas:

- La `todo_test` local puede contener migraciones de otras ramas. Se observó la versión `20261026000000` y una columna `due_date` ya creada, y con ese estado la nueva migración fallaría por columna duplicada. Compara `select version from schema_migration` con los ficheros de `migrations/`. Si hay versiones ajenas, recrea la base de test: es desechable, y la suite la vacía en cada test.
- No toques `todo_development` y no arranques la aplicación contra ella. Contiene migraciones de otras ramas, y `cmd/app` migraría al arrancar. Esta funcionalidad se valida con los tests automáticos.

Usa `soda` como en CI: `go run github.com/gobuffalo/pop/v6/soda@v6.4.1` o, si no está disponible, `$(go env GOPATH)/bin/soda`. Ejecútalo siempre con `-e test` desde la raíz del repositorio:

```sh
soda drop -e test      # solo si hay que recrear la base
soda create -e test    # solo si hay que recrear la base
soda migrate -e test
soda migrate down -e test   # comprueba la reversión de la nueva migración
soda migrate -e test
```

Después comprueba que la columna existe con `docker compose exec db psql -U postgres -d todo_test -c '\d tasks'`, donde debe aparecer `due_date | date`, y que no queda un `migrations/schema.sql` generado. Si lo hubiera, bórralo: no está versionado ni forma parte del alcance.

### Validación

```sh
gofmt -l .                       # sin salida
go vet ./...
GO_ENV=test go test -p 1 ./...
go build ./...
git diff --check
```

## Riesgos y recuperación

- **Base de test contaminada por otras ramas.** Las migraciones y columnas ajenas en `todo_test` hacen fallar la migración o enmascaran errores. Mitigación: recrear `todo_test` como se indica arriba.
- **Desplazamiento de día por zona horaria.** `ParseDueDate` produce medianoche UTC, y pgx codifica `time.Time` como `date` usando año, mes y día; PostgreSQL usa `UTC` en el contenedor. `Test_Task_DueDate_Persists` y los tests HTTP comprueban el viaje de ida y vuelta. Si fallaran por un día de diferencia, detente e informa: no cambies el tipo de columna sin replantear el plan.
- **Llamadas a métodos desde Plush.** Si Plush no resolviera `t.DueDateValue()` sobre elementos de la lista, los tests de listado fallarán. El receptor por valor está elegido para evitarlo. No cambies a otro mecanismo (helpers globales) sin informar.
- **Bind de campos protegidos.** Crear ya no hace bind sobre `models.Task`, sino sobre `taskForm`, lo que reduce la superficie de asignación masiva. `Test_TasksUpdate_PersistsTitleOnly` sigue cubriendo que `id`, `completed` y `created_at` no se pueden modificar.
- **PRs paralelas para la misma issue.** Hay otras ramas y PRs abiertas para la issue #5 que añaden la misma columna con otras versiones de migración. Fusionar más de una haría fallar la segunda migración por columna duplicada. Solo debe fusionarse una.
- **Recuperación.** El cambio es un único commit revertible. La migración down elimina la columna, y con ella las fechas límite guardadas. Las tareas y el resto de columnas no se ven afectados.
