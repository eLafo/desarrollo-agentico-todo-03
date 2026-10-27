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

- La fecha límite es **opcional**. Las tareas existentes no tienen fecha y la issue no la exige.
- Se asigna al crear una tarea y se cambia o se quita al editarla.
- Se muestra en la lista de tareas con el formato `DD/MM/AAAA`.
- Las fechas pasadas están permitidas. Si se rechazaran, una tarea antigua no se podría editar sin cambiar su fecha.
- Si el formulario envía un valor que no es un día válido con formato `AAAA-MM-DD`, la tarea no se guarda y aparece el error `La fecha límite no es válida.`.

Criterios de aceptación:

1. El formulario «Añadir tarea» tiene un campo de fecha opcional (`<input type="date" name="due_date">`). Si se envía con un día, la tarea se crea con esa fecha límite. Si se envía vacío, la tarea se crea sin fecha límite.
2. En la lista, una tarea con fecha límite muestra `Fecha límite: DD/MM/AAAA` dentro de `<time datetime="AAAA-MM-DD">`. Una tarea sin fecha límite no muestra nada nuevo.
3. El formulario de edición muestra la fecha límite actual en el campo de fecha. Al guardar, la fecha cambia o, si el campo está vacío, se quita. El estado completada o pendiente no cambia.
4. Un valor de fecha no válido (por ejemplo `2026-02-30` o `mañana`) devuelve 422 con `La fecha límite no es válida.` y no crea ni modifica la tarea. Si el título también es inválido, se muestran los dos errores.
5. Completar o reabrir una tarea conserva su fecha límite.
6. El día guardado no se desplaza por la zona horaria: `2026-11-15` se guarda en una columna `date` y vuelve como `2026-11-15`.

## Comportamiento actual y propuesta

Estado actual (rama creada desde `origin/main`):

- La tabla `tasks` tiene `id`, `title`, `completed`, `created_at` y `updated_at` (`migrations/20260901000000_create_tasks.up.fizz`).
- `models.Task` no tiene ningún campo de fecha visible para el usuario. `Validate` comprueba solo el título.
- `TasksCreate` enlaza el formulario directamente sobre `models.Task`, porque solo `title` tiene una etiqueta `form` distinta de `-`. `TasksUpdate` enlaza una estructura anónima `{Title}` para no sobrescribir los demás campos.
- `templates/tasks/index.plush.html` muestra cada tarea en un `<li class="task">` con una rejilla de tres columnas: botón de completar, `.task-title` y `.task-actions`. El `.task-status` es `sr-only` y tiene posición absoluta, así que no ocupa una celda. El formulario de alta tiene solo el título.
- `templates/tasks/edit.plush.html` edita solo el título.
- `public/assets/app.js` usa `.task-title` (su `textContent`) para el `aria-label` del botón e inserta `.task-status` antes de `.task-actions`, que es hijo directo de `li.task`.

Propuesta (cambio vertical mínimo, siguiendo los patrones existentes):

- Migración que añade la columna `due_date date NULL`.
- Campo `DueDate *time.Time` en `models.Task`, con funciones auxiliares para interpretar y formatear el día.
- `TasksCreate` y `TasksUpdate` enlazan una misma estructura de entrada (`title`, `due_date` como cadenas). Un helper normaliza la entrada, interpreta la fecha y valida la tarea completa. Después la acción guarda con `tx.Create` o `tx.Update`.
- Campo de fecha en los dos formularios y la fecha en la lista.
- Estilos para el nuevo campo y el texto de la fecha. Se cambia la versión de caché de `app.css`.

## Alcance

Incluido:

- Persistencia (migración reversible) y modelo.
- Creación y edición con fecha límite opcional, incluida la opción de quitarla.
- Visualización en la lista.
- Errores de validación para valores de fecha no válidos.
- CSS del nuevo campo y de la fecha mostrada.
- Tests de modelo y de acciones.

Excluido:

- Hora, zona horaria o recordatorios.
- Ordenar o filtrar por fecha límite (el orden sigue siendo `created_at asc, id asc`).
- Resaltar tareas vencidas o próximas a vencer.
- Rechazar fechas pasadas o limitar el rango de años.
- Cambios en `public/assets/app.js` (el toggle sigue funcionando porque `.task-title` y `.task-actions` mantienen su relación con `li.task`).
- Cambios en `fixtures/tasks.toml`, en la migración semilla o en dependencias (`go.mod`/`go.sum`). Solo se usa `time` de la biblioteca estándar.
- La corrección del `fetch` del toggle en `app.js` (commit local `429e9d6`, no publicado en `origin/main`), que pertenece a otro encargo.

## Diseño por capas

### Persistencia

Nueva migración fizz, con nombres de archivo siguiendo el patrón existente:

- `migrations/20261027170000_add_due_date_to_tasks.up.fizz`:
  ```
  add_column("tasks", "due_date", "date", {"null": true})
  ```
- `migrations/20261027170000_add_due_date_to_tasks.down.fizz`:
  ```
  drop_column("tasks", "due_date")
  ```

`migrations/embed.go` ya incluye `*.fizz`, así que no cambia. La aplicación aplica la migración al arrancar (`models.Migrate()` en `cmd/app/main.go`). CI la aplica con `soda migrate -e test`.

Se usa el tipo `date` de PostgreSQL porque guarda un día sin hora. Con pgx v5, un `time.Time` enviado a un parámetro `date` se codifica con su año, mes y día. Al leerlo vuelve como medianoche UTC del mismo día.

### Modelo (`models/task.go`)

- Constantes:
  - `DueDateFormat = "2006-01-02"`: formato del `<input type="date">` y del atributo `datetime`.
  - `DueDateDisplayFormat = "02/01/2006"`: formato que se muestra.
- Campo nuevo, entre `Completed` y `CreatedAt`:
  `DueDate *time.Time \`json:"due_date" db:"due_date" form:"-"\``.
  `nil` significa «sin fecha límite». Un puntero evita añadir `github.com/gobuffalo/nulls` como dependencia directa. `form:"-"` impide que `c.Bind` lo escriba y que los tests que envían `&models.Task{...}` lo serialicen.
- `func ParseDueDate(value string) (*time.Time, error)`: recorta espacios. Si el valor está vacío, devuelve `nil, nil`. Si no, aplica `time.Parse(DueDateFormat, value)`, que devuelve un día en UTC y rechaza días inexistentes como `2026-02-30`.
- Métodos con **receptor por valor**, porque en `index.plush.html` el bucle `for (t) in tasks` recorre valores `Task` y Plush no encuentra métodos con receptor puntero sobre un valor:
  - `func (t Task) DueDateValue() string`: `AAAA-MM-DD` o `""`.
  - `func (t Task) DueDateLabel() string`: `DD/MM/AAAA` o `""`.
- `Validate` no cambia. La fecha ya interpretada siempre es válida. La comprobación del formato pertenece a la conversión de la entrada (ver la capa de acción).

### Acción (`actions/tasks.go`)

Se añade un tipo de entrada y un helper que comparten la creación y la edición:

```go
// taskInput recoge los campos editables del formulario de una tarea.
type taskInput struct {
	Title   string `form:"title" json:"title"`
	DueDate string `form:"due_date" json:"due_date"`
}

// bindTask copia en la tarea el título y la fecha límite del formulario y la valida.
// Una fecha límite que no se puede interpretar se informa como error de validación.
func bindTask(c buffalo.Context, tx *pop.Connection, task *models.Task) (*validate.Errors, error) {
	input := taskInput{}
	if err := c.Bind(&input); err != nil {
		return nil, err
	}
	task.Title = strings.TrimSpace(input.Title)

	dueDate, dueDateErr := models.ParseDueDate(input.DueDate)
	task.DueDate = dueDate

	verrs, err := task.Validate(tx)
	if err != nil {
		return nil, err
	}
	if dueDateErr != nil {
		verrs.Add("due_date", "La fecha límite no es válida.")
	}
	return verrs, nil
}
```

- `TasksCreate`: crea `task := &models.Task{}` y llama a `bindTask`. Si hay errores, conserva el render 422 actual de `tasks/index.plush.html`, con `tasks`, `task` y `errors`. Si no hay errores, guarda con `tx.Create(task)`, porque la tarea ya está validada, y mantiene el flash `Tarea creada.` y el redirect 303 a `/`. Desaparecen el `c.Bind(task)` directo y el `TrimSpace` suelto, porque los hace `bindTask`.
- `TasksUpdate`: después de `tx.Find`, llama a `bindTask` en vez de enlazar la estructura anónima `{Title}`. Si hay errores, conserva el render 422 de `tasks/edit.plush.html`. Si no hay errores, guarda con `tx.Update(task)` y mantiene el flash `Tarea actualizada.` y el redirect 303. El comentario de la función pasa a decir que actualiza el título y la fecha límite y conserva los demás datos. `id`, `completed` y `created_at` no se pueden cambiar porque no forman parte de `taskInput`.
- Semántica de la edición: si `due_date` falta en la petición o llega vacío, se quita la fecha límite. El formulario de edición siempre envía el campo.
- `TasksToggle`, `TasksDestroy`, `TasksEdit`, `TasksIndex` y las rutas de `actions/app.go` no cambian. `tx.Update` en el toggle reescribe `due_date` con el valor cargado, así que la fecha se conserva.

### Renderizado

`templates/tasks/index.plush.html`:

- En cada `<li class="task">`, sustituye `<span class="task-title"><%= t.Title %></span>` por un contenedor que ocupa la celda central de la rejilla:
  ```html
  <div class="task-body">
    <span class="task-title"><%= t.Title %></span>
    <%= if (t.DueDateLabel() != "") { %>
      <span class="task-due">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></span>
    <% } %>
  </div>
  ```
  `.task-title` conserva solo el título, así que el `textContent` que lee `app.js` no cambia. `.task-actions` sigue siendo hijo directo de `li.task`.
- En el formulario de alta, dentro de `.new-task-row` y entre el input del título y el botón «Añadir», añade:
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
- Los atributos de error dependen del campo. En el input del título, cambia la condición `errors.HasAny()` por `len(errors.Get("title")) > 0`, para los atributos `autofocus aria-invalid="true" aria-describedby="title-error"`. Sustituye el bloque `<%= if (errors.HasAny()) { %> <p id="title-error" …>` por dos bloques independientes:
  - `title-error`, si `len(errors.Get("title")) > 0`, con los mensajes de `errors.Get("title")`;
  - `due-date-error`, si `len(errors.Get("due_date")) > 0`, con los mensajes de `errors.Get("due_date")`.

  Ambos conservan `class="field-error" role="alert"`.

`templates/tasks/edit.plush.html`:

- En el input del título, cambia la condición `errors.HasAny()` por `len(errors.Get("title")) > 0`, igual que en el alta. Cuando no hay error de título, conserva `aria-describedby="title-help"`. El bloque `title-error` muestra solo `errors.Get("title")` con esa misma condición.
- Después del bloque de error del título y antes de `.edit-task-actions`, añade:
  ```html
  <label for="task-due-date">Fecha límite (opcional)</label>
  <input
    id="task-due-date"
    type="date"
    name="due_date"
    value="<%= task.DueDateValue() %>"
    <%= if (len(errors.Get("due_date")) > 0) { %>aria-invalid="true" aria-describedby="due-date-error"<% } else { %>aria-describedby="due-date-help"<% } %>
  >
  <p id="due-date-help" class="field-hint">Solo el día, sin hora. Déjala vacía para quitarla.</p>
  <%= if (len(errors.Get("due_date")) > 0) { %>
    <p id="due-date-error" class="field-error" role="alert">
      <%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %>
    </p>
  <% } %>
  ```

`templates/application.plush.html`: cambia `/assets/app.css?v=3` por `/assets/app.css?v=4` para invalidar la caché del CSS. `app.js` mantiene `?v=2`.

### CSS (`public/assets/app.css`)

- `.task-title`: deja de ser una celda de la rejilla. Sustituye sus reglas por `display: block; overflow-wrap: anywhere; line-height: 1.55;`. Retira `align-self`, `min-width` y `padding`, que pasan a `.task-body`. La regla `.task-completed .task-title` y la de `font-size` en `@media (max-width: 760px)` no cambian.
- Añade después de `.task-title`:
  ```css
  .task-body {
    align-self: center;
    min-width: 0;
    padding: 0.65rem 0;
  }

  .task-due {
    display: block;
    margin-top: 0.1rem;
    color: var(--color-muted);
    font-size: 0.8rem;
  }
  ```
- Amplía el selector `input[type="text"]` a `input[type="text"], input[type="date"]`, con un selector por línea como en el resto del archivo, para que el campo de fecha use los mismos bordes, altura y tipografía.
- Después de `.new-task-row input`, añade:
  ```css
  .new-task-row input[type="date"] {
    flex: 0 0 auto;
    width: auto;
  }
  ```
  Su especificidad (0,2,1) es mayor que la de `.new-task-row input` (0,1,1), también dentro de `@media (max-width: 380px)`. Así, en móvil estrecho el título ocupa la primera línea y la fecha y el botón pasan a la segunda.
- Añade después de `.edit-task label`:
  ```css
  .edit-task label:not(:first-of-type) {
    margin-top: 1.25rem;
  }
  ```

## Archivos relevantes

| Ruta | Responsabilidad | Cambio |
| --- | --- | --- |
| `migrations/20261027170000_add_due_date_to_tasks.up.fizz` | Añade `tasks.due_date` (`date`, nullable) | Nuevo |
| `migrations/20261027170000_add_due_date_to_tasks.down.fizz` | Elimina `tasks.due_date` | Nuevo |
| `models/task.go` | Modelo `Task`, validación, formato de la fecha | Modificado |
| `models/task_test.go` | Tests del modelo | Modificado |
| `actions/tasks.go` | Alta y edición con fecha límite | Modificado |
| `actions/tasks_test.go` | Tests de acciones y renderizado | Modificado |
| `templates/tasks/index.plush.html` | Lista y formulario de alta | Modificado |
| `templates/tasks/edit.plush.html` | Formulario de edición | Modificado |
| `templates/application.plush.html` | Versión de caché de `app.css` | Modificado |
| `public/assets/app.css` | Estilos del campo y de la fecha | Modificado |
| `actions/app.go` | Rutas (sin cambios) | Referencia |
| `public/assets/app.js` | Toggle sin recarga; depende de `.task-title` y `.task-actions` (sin cambios) | Referencia |
| `migrations/embed.go` | Incluye `*.fizz` (sin cambios) | Referencia |
| `.github/workflows/ci.yaml` | Formato, vet, migraciones, tests y build (sin cambios) | Referencia |

## Implementación paso a paso

1. Comprueba `git status --short --branch`: rama `feature/issue-5-tareas-con-vencimiento`, sin más cambios que este plan.
2. Crea los dos archivos de migración con el contenido de «Persistencia».
3. Modifica `models/task.go`: constantes, campo `DueDate`, `ParseDueDate`, `DueDateValue` y `DueDateLabel`, como se describe en «Modelo». Respeta el estilo existente: comentarios en español y alineación de etiquetas con `gofmt`.
4. Modifica `actions/tasks.go`: añade `taskInput` y `bindTask` y adapta `TasksCreate` y `TasksUpdate` como se describe en «Acción». Elimina las importaciones que dejen de usarse; `strings` sigue usándose en `bindTask`.
5. Modifica `templates/tasks/index.plush.html`, `templates/tasks/edit.plush.html` y `templates/application.plush.html` como se describe en «Renderizado».
6. Modifica `public/assets/app.css` como se describe en «CSS».
7. Añade los tests de «Tests y validación».
8. Prepara la base de datos de test y ejecuta las validaciones de la sección siguiente.
9. Revisa `git diff --check`, `git diff` y `git status --short`. Solo deben aparecer los archivos de la tabla marcados como «Nuevo» o «Modificado» y este plan.

## Tests y validación

### Tests de modelo (`models/task_test.go`)

- `Test_Task_DueDate_OptionalByDefault`: crea `&Task{Title: "Sin fecha"}`, la recarga con `ms.DB.Find` y comprueba que `DueDate` es `nil`, `DueDateValue()` es `""` y `DueDateLabel()` es `""`.
- `Test_Task_DueDate_PersistsDay`: obtiene `day` con `ParseDueDate("2026-11-15")` (sin error), crea `&Task{Title: "Con fecha", DueDate: day}`, recarga en otra variable y comprueba `DueDateValue() == "2026-11-15"` y `DueDateLabel() == "15/11/2026"`.
- `Test_ParseDueDate`:
  - `""` y `"   "` devuelven `nil, nil`;
  - `" 2026-11-15 "` devuelve el día `2026-11-15`;
  - `"2026-02-30"`, `"15/11/2026"`, `"2026-11-15T10:00"` y `"mañana"` devuelven error.

### Tests de acciones (`actions/tasks_test.go`)

Usan `url.Values`, como los tests de update existentes. Para crear tareas con fecha, añade el helper `as.taskWithDueDate(day string) *models.Task`. Usa `models.ParseDueDate` y `as.DB.ValidateAndCreate` y comprueba que no hay errores.

- `Test_TasksCreate_Valid` (existente): añade `as.Nil(task.DueDate)`.
- `Test_TasksCreate_WithDueDate`: `Post(url.Values{"title": {"Con fecha"}, "due_date": {"2026-11-15"}})` devuelve 303. La primera tarea tiene `DueDateValue() == "2026-11-15"`.
- `Test_TasksCreate_InvalidDueDate`: `due_date` `"2026-02-30"` y título válido devuelven 422. El body contiene `La fecha límite no es válida.` y `aria-describedby="due-date-error"`. El contador de tareas es 0.
- `Test_TasksCreate_InvalidDueDateAndEmptyTitle`: título `""` y `due_date` `"mañana"` devuelven 422. El body contiene los dos mensajes y el contador es 0.
- `Test_TasksIndex_ShowsDueDate`: con `taskWithDueDate("2026-11-15")`, `GET /` contiene `Fecha límite: <time datetime="2026-11-15">15/11/2026</time>`.
- `Test_TasksIndex_ListsTasks` (existente, fixture sin fechas): añade `as.NotContains(body, "task-due")`.
- `Test_TasksEdit_PrefillsDueDate`: con `taskWithDueDate("2026-11-15")`, `GET /tasks/{id}/edit` contiene `value="2026-11-15"` y `name="due_date"`.
- `Test_TasksUpdate_ChangesDueDate`: una tarea completada con fecha `2026-11-15` recibe `PUT` con título y `due_date` `"2026-12-01"`. Devuelve 303. Tras recargar, la fecha es `2026-12-01` y `Completed` sigue en `true`.
- `Test_TasksUpdate_ClearsDueDate`: una tarea con fecha recibe `PUT` con título y `due_date` `""`. Devuelve 303. Tras recargar, `DueDate` es `nil`.
- `Test_TasksUpdate_InvalidDueDate`: una tarea con fecha `2026-11-15` recibe `PUT` con `due_date` `"2026-13-01"`. Devuelve 422 y el body contiene `La fecha límite no es válida.` Tras recargar, el título y la fecha no cambian.
- `Test_TasksToggle_KeepsDueDate`: una tarea con fecha recibe `PUT /tasks/{id}/toggle`. Devuelve 303. Tras recargar, `Completed` es `true` y la fecha sigue siendo `2026-11-15`.

Los tests existentes deben seguir pasando sin cambios, salvo las dos aserciones añadidas arriba. Entre ellos están `Test_TasksUpdate_EmptyTitle` (`aria-describedby="title-error"`) y `Test_TasksUpdate_PersistsTitleOnly`.

### Preparación de la base de datos de test

Los tests necesitan PostgreSQL (`docker compose up -d db`, servicio `db` saludable) y la base `todo_test` migrada **exactamente** con las migraciones de esta rama.

Aviso sobre el estado observado: la base local `todo_test` compartida tiene aplicada la migración `20261027120000`, que no existe en esta rama, y ya tiene una columna `due_date` creada por otra rama. No reutilices ese esquema. Recrea **solo** la base de test. Es desechable: los tests la vacían en cada ejecución y CI la crea desde cero. No toques `todo_development`.

```sh
docker compose exec -T db psql -U postgres -c 'DROP DATABASE IF EXISTS todo_test' -c 'CREATE DATABASE todo_test'
GO_ENV=test "$(go env GOPATH)/bin/soda" migrate -e test
# comprueba que la migración es reversible y vuelve a aplicarla
GO_ENV=test "$(go env GOPATH)/bin/soda" migrate down -e test
GO_ENV=test "$(go env GOPATH)/bin/soda" migrate -e test
docker compose exec -T db psql -U postgres -d todo_test -c '\d tasks'
```

Resultado esperado: `\d tasks` muestra `due_date | date` nullable, y `schema_migration` contiene solo `20260901000000`, `20260901000001` y `20261027170000`. Si `soda` no está en `$(go env GOPATH)/bin`, instálalo como en CI: `go install github.com/gobuffalo/pop/v6/soda@v6.4.1`. Si `DROP DATABASE` falla porque hay conexiones abiertas a `todo_test`, detente e infórmalo. No uses `WITH (FORCE)`.

### Comandos de validación

Ejecútalos desde la raíz del repositorio, en este orden:

1. `gofmt -l .`: salida vacía.
2. `go vet ./...`: sin errores.
3. `GO_ENV=test go test -p 1 ./...`: todo en verde. `GO_ENV=test` es obligatorio; sin él, los tests usarían `todo_development`.
4. `go build ./...`: sin errores.
5. `git diff --check`: sin errores.

No hace falta una verificación manual en el navegador. Arrancar la aplicación aplicaría la migración a `todo_development`, cuyo esquema local también tiene cambios de otros experimentos. Los tests de acciones cubren el HTML renderizado.

## Riesgos y recuperación

- **Base de test local contaminada.** Si no se recrea `todo_test`, los tests pueden pasar o fallar por un esquema ajeno. Recuperación: aplicar la preparación descrita arriba.
- **Desplazamiento de día por zona horaria.** `time.Parse` devuelve medianoche UTC y pgx codifica el `date` con año, mes y día. El test `Test_Task_DueDate_PersistsDay` lo detecta. Si fallara, detente e informa en vez de cambiar el tipo de columna.
- **Llamadas a métodos en Plush.** Los métodos `DueDateValue` y `DueDateLabel` deben tener receptor por valor para el bucle de `index.plush.html`. Si Plush falla al renderizarlos, los tests de índice y de edición lo detectan.
- **Quitar la fecha al omitir el campo.** Un `PUT` sin `due_date` quita la fecha límite. Es intencionado: el formulario siempre envía el campo y no hay otros clientes. Queda documentado en el comentario de `TasksUpdate`.
- **Navegadores sin `<input type="date">`.** El campo se ve como texto. Un formato distinto de `AAAA-MM-DD` produce el error 422 explícito, no un 500.
- **Despliegue.** La migración solo añade una columna nullable, sin reescribir datos. `models.Migrate()` la aplica al arrancar. Para deshacerla, usa `soda migrate down` (elimina la columna y las fechas guardadas) y revierte el commit.
- **Ramas paralelas del mismo issue.** El repositorio tiene muchas ramas `feature/issue-5-*` con migraciones `add_due_date_to_tasks` de otros timestamps. Fusionar más de una añadiría la columna dos veces y fallaría. Solo se debe fusionar una de ellas.
