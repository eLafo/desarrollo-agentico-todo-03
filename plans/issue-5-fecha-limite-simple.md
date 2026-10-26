# Feature: fecha límite (solo día) en las tareas

## Encargo y criterios de aceptación

Issue [#5](https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5): «Quiero añadir fecha límite a las tasks». Cuerpo: «Vale con que sea simplemente un día. Sin hora».

Requisitos explícitos:

- Una tarea puede tener una fecha límite.
- La fecha límite es un día del calendario, sin hora.

Inferencias (decididas en este plan):

- La fecha límite es **opcional**: las tareas existentes no tienen y crear una tarea sin fecha sigue funcionando igual.
- Se puede indicar al crear la tarea y cambiar o quitar al editarla.
- Se muestra en la lista de tareas junto al título.
- Se admiten fechas pasadas (no se valida contra «hoy»).

Criterios de aceptación:

1. El formulario «Añadir tarea» de `/` incluye un campo de fecha (`<input type="date" name="due_date">`) opcional.
2. `POST /tasks` con `due_date=2026-11-15` crea la tarea con fecha límite 15/11/2026; sin `due_date` (o vacío) la crea sin fecha límite.
3. Si `due_date` no es una fecha `AAAA-MM-DD` válida, responde `422`, no crea la tarea, vuelve a mostrar el formulario con el título introducido y el mensaje «La fecha límite no es válida.» asociado al campo de fecha. Si además el título es inválido, se muestran ambos errores.
4. La lista muestra, bajo el título de cada tarea con fecha, `Fecha límite: 15/11/2026` (formato `dd/mm/aaaa`) dentro de un `<time datetime="2026-11-15">`. Las tareas sin fecha no muestran nada nuevo.
5. `GET /tasks/{id}/edit` muestra un campo «Fecha límite» precargado con la fecha actual (`value="2026-11-15"`) o vacío.
6. `PUT /tasks/{id}` guarda la nueva fecha; con `due_date` vacío la elimina; con una fecha inválida responde `422` sin modificar título ni fecha en la base de datos.
7. Completar o reabrir una tarea conserva su fecha límite.
8. La columna se guarda como `DATE` en PostgreSQL: no se almacena hora ni zona horaria.

## Comportamiento actual y propuesta

Actual (rama base `origin/main`):

- Tabla `tasks` (`migrations/20260901000000_create_tasks.up.fizz`): `id`, `title` (200), `completed`, timestamps. No existe ninguna fecha límite.
- `models.Task` (`models/task.go`) solo valida el título.
- `TasksCreate` (`actions/tasks.go`) hace `c.Bind(task)` y `tx.ValidateAndCreate`. `TasksUpdate` enlaza un struct anónimo con `Title` y hace `tx.ValidateAndUpdate`; el resto de campos no cambia.
- `templates/tasks/index.plush.html` muestra los errores del formulario con `errors.HasAny()` y siempre bajo el campo título (`title-error`). `templates/tasks/edit.plush.html` hace lo mismo y su ayuda dice «Los demás datos de la tarea no cambian».
- `public/assets/app.js` usa `task.querySelector(".task-title").textContent` para construir el `aria-label` del botón de completar: el texto de `.task-title` debe seguir siendo solo el título.

Propuesta (cambio vertical mínimo, siguiendo los patrones existentes):

- Nueva migración fizz que añade `due_date DATE NULL`.
- Campo `DueDate *time.Time` en `models.Task` (`form:"-"`, como los demás campos no editables por `Bind`), más una función `ParseDueDate` y dos métodos de presentación (`DueDateValue`, `DueDateLabel`).
- Las acciones leen `due_date` del formulario, lo interpretan con `ParseDueDate` y añaden un error de validación `due_date` si no es válido. Para mostrar a la vez errores de título y de fecha, crear/actualizar pasan de `ValidateAndCreate`/`ValidateAndUpdate` a `task.Validate(tx)` + `tx.Create`/`tx.Update` (equivalente: `Task` solo define `Validate`).
- Plantillas: campo de fecha en ambos formularios, errores por campo y fecha visible en la lista. CSS mínimo para maquetar el campo y la fecha.

## Alcance

Incluido:

- Migración up/down de la columna `due_date`.
- Modelo, parseo y presentación de la fecha.
- Crear, editar y quitar la fecha límite desde los formularios existentes.
- Mostrar la fecha en la lista.
- Tests de modelo y de acciones.
- CSS de los nuevos elementos y bump de caché de `app.css`.

Excluido:

- Hora, zona horaria, recordatorios o notificaciones.
- Resaltar tareas vencidas o próximas a vencer, filtros u ordenación por fecha (el orden sigue siendo `created_at asc, id asc`).
- Cambios en `public/assets/app.js`, en la migración de semillas (`20260901000001_seed_tasks`), en `fixtures/tasks.toml`, en `go.mod`/`go.sum` o en el workflow de CI.
- Validar que la fecha sea futura.

## Diseño por capas

### Persistencia y migraciones

- `migrations/20261026120000_add_due_date_to_tasks.up.fizz`:
  ```
  add_column("tasks", "due_date", "date", {"null": true})
  ```
- `migrations/20261026120000_add_due_date_to_tasks.down.fizz`:
  ```
  drop_column("tasks", "due_date")
  ```
- `migrations/embed.go` ya embebe `*.fizz`; `models.Migrate()` (llamado desde `cmd/app/main.go`) aplicará la migración al arrancar. No se tocan las migraciones existentes.
- La versión `20261026120000` se elige distinta de `20261026000000`, que figura en la base de test local por un intento anterior (ver «Riesgos»).

### Modelo (`models/task.go`)

- Añadir constante y campo:
  ```go
  // DueDateLayout es el formato de la fecha límite en formularios: solo el día, sin hora.
  const DueDateLayout = "2006-01-02"

  type Task struct {
  	...
  	Completed bool       `json:"completed" db:"completed" form:"-"`
  	DueDate   *time.Time `json:"due_date" db:"due_date" form:"-"`
  	CreatedAt ...
  }
  ```
  `*time.Time` representa `NULL` sin añadir dependencias (`github.com/gobuffalo/nulls` es indirecta en `go.mod`; no se usa).
- `ParseDueDate`:
  ```go
  // ParseDueDate interpreta una fecha límite con formato AAAA-MM-DD.
  // Una cadena vacía significa que la tarea no tiene fecha límite.
  func ParseDueDate(value string) (*time.Time, error) {
  	value = strings.TrimSpace(value)
  	if value == "" {
  		return nil, nil
  	}
  	d, err := time.Parse(DueDateLayout, value)
  	if err != nil {
  		return nil, err
  	}
  	if d.Year() < 1 {
  		return nil, fmt.Errorf("año fuera de rango: %d", d.Year())
  	}
  	return &d, nil
  }
  ```
  `time.Parse` devuelve la fecha a medianoche UTC y rechaza días inexistentes (`2026-02-30`), horas añadidas y años de más de 4 cifras. El año 0 se rechaza porque PostgreSQL no lo admite.
- Métodos de presentación con receptor **por valor** (la plantilla los llama sobre elementos de `models.Tasks`, que son `Task` y no `*Task`):
  ```go
  // DueDateValue devuelve la fecha límite en formato AAAA-MM-DD, o "" si no tiene.
  func (t Task) DueDateValue() string {
  	if t.DueDate == nil {
  		return ""
  	}
  	return t.DueDate.Format(DueDateLayout)
  }

  // DueDateLabel devuelve la fecha límite en formato dd/mm/aaaa, o "" si no tiene.
  func (t Task) DueDateLabel() string {
  	if t.DueDate == nil {
  		return ""
  	}
  	return t.DueDate.Format("02/01/2006")
  }
  ```
  No se convierte de zona horaria: el valor se lee y se formatea tal cual (medianoche UTC).
- `Validate` no cambia.

### Acciones (`actions/tasks.go`)

- Helper nuevo:
  ```go
  // validateTask valida la tarea y añade un error si la fecha límite recibida no es válida.
  func validateTask(tx *pop.Connection, task *models.Task, dueDateErr error) (*validate.Errors, error) {
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
- `TasksCreate`: tras `c.Bind(task)` y el `TrimSpace` del título:
  ```go
  dueDate, dueDateErr := models.ParseDueDate(c.Param("due_date"))
  task.DueDate = dueDate

  verrs, err := validateTask(tx, task, dueDateErr)
  if err != nil {
  	return err
  }
  if verrs.HasAny() {
  	// bloque 422 existente sin cambios
  }
  if err := tx.Create(task); err != nil {
  	return err
  }
  ```
  `c.Param` incluye los valores del cuerpo del formulario en Buffalo v1.1.4 (`context.go`, `req.ParseForm`). `c.Bind` ignora `due_date` porque el campo tiene `form:"-"`.
- `TasksUpdate`: añadir `DueDate string `form:"due_date" json:"due_date"`` al struct anónimo de entrada y, tras asignar el título:
  ```go
  dueDate, dueDateErr := models.ParseDueDate(input.DueDate)
  if dueDateErr == nil {
  	task.DueDate = dueDate
  }

  verrs, err := validateTask(tx, task, dueDateErr)
  if err != nil {
  	return err
  }
  if verrs.HasAny() {
  	// bloque 422 existente sin cambios
  }
  if err := tx.Update(task); err != nil {
  	return err
  }
  ```
  Actualizar el comentario de la función a: `// TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.`
- `TasksIndex`, `TasksEdit`, `TasksToggle` y `TasksDestroy` no cambian (`TasksToggle` hace `Find` + `Update`, que conserva `due_date`). Rutas en `actions/app.go` sin cambios.

### Renderizado (`templates/tasks/*.plush.html`)

`templates/tasks/index.plush.html`:

- En cada `<li>`, envolver el título en un contenedor y añadir la fecha:
  ```html
  <div class="task-main">
    <span class="task-title"><%= t.Title %></span>
    <%= if (t.DueDateLabel() != "") { %>
      <span class="task-due-date">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></span>
    <% } %>
  </div>
  ```
  `.task-title` sigue siendo un elemento propio con solo el título (lo necesita `app.js`), y `.task-status`/`.task-actions` siguen siendo hijos directos del `<li>`.
- En el formulario `new-task`:
  - Cambiar las condiciones del input de título de `errors.HasAny()` a `len(errors.Get("title")) > 0`.
  - Añadir, entre el input de título y el botón «Añadir»:
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
  - Sustituir el bloque de error por dos bloques independientes: `title-error` (si `len(errors.Get("title")) > 0`, mismo contenido que hoy) y `due-date-error` (si `len(errors.Get("due_date")) > 0`, `<p id="due-date-error" class="field-error" role="alert">` con los mensajes de `errors.Get("due_date")`).

`templates/tasks/edit.plush.html`:

- Input de título: cambiar `errors.HasAny()` por `len(errors.Get("title")) > 0` (en el atributo `aria-*` y en el bloque `title-error`). Ayuda del título: «Hasta 200 caracteres.» (se elimina «Los demás datos de la tarea no cambian.», que contradiría el nuevo campo).
- Tras el bloque de error del título y antes de `edit-task-actions`:
  ```html
  <div class="edit-task-field">
    <label for="task-due-date">Fecha límite</label>
    <input
      id="task-due-date"
      type="date"
      name="due_date"
      value="<%= task.DueDateValue() %>"
      <%= if (len(errors.Get("due_date")) > 0) { %>aria-invalid="true" aria-describedby="due-date-error"<% } else { %>aria-describedby="due-date-help"<% } %>
    >
    <p id="due-date-help" class="field-hint">Opcional. Solo el día, sin hora. Déjala vacía para quitar la fecha límite.</p>
    <%= if (len(errors.Get("due_date")) > 0) { %>
      <p id="due-date-error" class="field-error" role="alert">
        <%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %>
      </p>
    <% } %>
  </div>
  ```

`templates/application.plush.html`: `app.css?v=3` → `app.css?v=4`.

### CSS (`public/assets/app.css`)

- Sustituir la regla `.task-title` por:
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

  .task-due-date {
    display: block;
    margin-top: 0.15rem;
    color: var(--color-muted);
    font-size: 0.8rem;
  }
  ```
  `.task-completed .task-title` y la regla móvil `.task-title { font-size: 0.9rem; }` se mantienen.
- Ampliar el selector `input[type="text"]` a `input[type="text"], input[type="date"]` y añadir a continuación:
  ```css
  input[type="date"] {
    width: auto;
  }
  ```
- Tras `.new-task-row input { flex: 1; }`:
  ```css
  .new-task-row input[type="date"] {
    flex: none;
  }
  ```
  (su especificidad supera también a `.new-task-row input { flex-basis: … }` de la media query de 380px).
- Junto a las reglas de `.edit-task`:
  ```css
  .edit-task-field {
    margin-top: 1.5rem;
  }
  ```

## Archivos relevantes

| Ruta | Responsabilidad |
| --- | --- |
| `migrations/20261026120000_add_due_date_to_tasks.up.fizz` (nuevo) | Añade `tasks.due_date DATE NULL`. |
| `migrations/20261026120000_add_due_date_to_tasks.down.fizz` (nuevo) | Elimina la columna. |
| `models/task.go` | Campo `DueDate`, `DueDateLayout`, `ParseDueDate`, `DueDateValue`, `DueDateLabel`. |
| `models/task_test.go` | Tests de parseo, persistencia y presentación. |
| `actions/tasks.go` | Lectura y validación de `due_date` en crear/actualizar; helper `validateTask`. |
| `actions/tasks_test.go` | Tests HTTP de crear, listar, editar, actualizar y completar con fecha. |
| `templates/tasks/index.plush.html` | Campo de fecha al crear, errores por campo, fecha en la lista. |
| `templates/tasks/edit.plush.html` | Campo de fecha al editar, errores por campo, ayuda del título. |
| `templates/application.plush.html` | Bump de caché `app.css?v=4`. |
| `public/assets/app.css` | Maquetación de `.task-main`, `.task-due-date`, inputs de fecha y `.edit-task-field`. |
| `plans/issue-5-fecha-limite-simple.md` | Este plan. |

Solo lectura (contexto): `actions/app.go` (rutas), `public/assets/app.js` (depende de `.task-title`, `.task-status`, `.task-actions`), `migrations/embed.go`, `cmd/app/main.go`, `fixtures/tasks.toml`, `.github/workflows/ci.yaml`, `database.yml`.

## Implementación paso a paso

1. Crear las dos migraciones fizz con el contenido indicado.
2. Modificar `models/task.go`: constante, campo `DueDate` (entre `Completed` y `CreatedAt`), `ParseDueDate`, `DueDateValue` y `DueDateLabel`.
3. Modificar `actions/tasks.go`: helper `validateTask`, cambios en `TasksCreate` y `TasksUpdate` y comentario de `TasksUpdate`.
4. Modificar `templates/tasks/index.plush.html`, `templates/tasks/edit.plush.html` y `templates/application.plush.html` como se describe.
5. Modificar `public/assets/app.css` como se describe.
6. Añadir los tests de «Tests y validación».
7. Preparar la base de test y ejecutar la validación.

## Tests y validación

### Tests de modelo (`models/task_test.go`, en `ModelSuite`)

- `Test_ParseDueDate_Empty`: `""` y `"   "` → `nil, nil`.
- `Test_ParseDueDate_Valid`: `"2026-11-15"` → no nil y `Format(DueDateLayout) == "2026-11-15"`.
- `Test_ParseDueDate_Invalid`: `"15/11/2026"`, `"2026-02-30"`, `"2026-11-15T10:00"`, `"0000-01-01"`, `"mañana"` → error y `nil`.
- `Test_Task_DueDate_Persists`: crear con `DueDate` 2026-11-15, recargar con `Find` y comprobar `DueDateValue() == "2026-11-15"` y `DueDateLabel() == "15/11/2026"`.
- `Test_Task_DueDate_Optional`: crear sin fecha, recargar: `DueDate` nil, `DueDateValue()` y `DueDateLabel()` vacíos.

### Tests de acciones (`actions/tasks_test.go`, enviando `url.Values`)

- `Test_TasksIndex_NewTaskFormHasDueDate`: `GET /` contiene `type="date"` y `name="due_date"`.
- `Test_TasksIndex_ShowsDueDate`: dos tareas, una con fecha 2026-11-15 y otra sin; el cuerpo contiene `datetime="2026-11-15"` y `15/11/2026`, y `strings.Count(body, "task-due-date") == 1`.
- `Test_TasksCreate_Valid` (existente): añadir `as.Nil(task.DueDate)`.
- `Test_TasksCreate_WithDueDate`: `title=Con fecha`, `due_date=2026-11-15` → `303` y fecha guardada `2026-11-15`.
- `Test_TasksCreate_InvalidDueDate`: `title=Con fecha rota`, `due_date=15/11/2026` → `422`, contiene «La fecha límite no es válida.», `aria-describedby="due-date-error"` y `value="Con fecha rota"`; no hay tareas.
- `Test_TasksCreate_EmptyTitleAndInvalidDueDate`: `title=""`, `due_date=nope` → `422` con ambos mensajes; no hay tareas.
- `Test_TasksEdit_PrefillsDueDate`: tarea con fecha 2026-11-15 → `GET /tasks/{id}/edit` contiene `value="2026-11-15"`.
- `Test_TasksUpdate_SetsDueDate`: tarea sin fecha, `PUT` con `title` y `due_date=2026-12-01` → `303`, fecha `2026-12-01`.
- `Test_TasksUpdate_ClearsDueDate`: tarea con fecha, `PUT` con `due_date=""` → `303`, `DueDate` nil.
- `Test_TasksUpdate_InvalidDueDate`: tarea «Título original» con fecha 2026-11-15, `PUT` con `title=Otro título`, `due_date=2026-02-30` → `422` con el mensaje; en BD siguen el título y la fecha originales.
- `Test_TasksToggle_KeepsDueDate`: tarea con fecha, `PUT /tasks/{id}/toggle` → `303`, `Completed` true y fecha intacta.

Los tests existentes deben seguir pasando sin cambios (salvo la línea añadida a `Test_TasksCreate_Valid`); en particular `Test_TasksUpdate_EmptyTitle` sigue esperando `aria-describedby="title-error"`.

### Preparación de la base de test

Requiere PostgreSQL en `127.0.0.1:5432` (`docker compose up -d db`) y `soda` (`$(go env GOPATH)/bin/soda`; CI usa `github.com/gobuffalo/pop/v6/soda@v6.4.1`).

La base local `todo_test` contiene restos de un intento anterior (columna `due_date` y versión `20261026000000` en `schema_migration`), así que debe recrearse antes de migrar; es una base desechable:

```bash
soda drop -e test
soda create -e test
soda migrate -e test
soda migrate down -e test   # comprueba la migración down
soda migrate -e test
```

Si `soda migrate` genera `migrations/schema.sql` (solo ocurre si `pg_dump` está disponible), bórralo: no forma parte del cambio.

### Comandos de validación

```bash
gofmt -l .                         # sin salida
go vet ./...
GO_ENV=test go test -p 1 ./...
go build ./...
git diff --check
git status --short                 # solo los archivos de «Archivos relevantes»
```

Comprobación manual opcional: arrancar la app (`/env:start`) aplica la migración a `todo_development`; crear una tarea con fecha, editarla, quitarla y completar una tarea con fecha.

## Riesgos y recuperación

- **Base de test local desfasada**: sin recrearla, `soda migrate -e test` falla con «column "due_date" already exists». Recuperación: los comandos de «Preparación de la base de test».
- **PRs duplicados para la misma issue**: hay varios PR abiertos para la issue #5 con migraciones de versiones distintas. Fusionar más de uno añadiría dos veces `due_date` y rompería las migraciones. Solo debe fusionarse uno; los demás se cierran.
- **Zona horaria**: la fecha viaja como `AAAA-MM-DD`, se parsea a medianoche UTC, se guarda como `DATE` y se formatea sin conversión. Si en el futuro se convirtiera a hora local, podría mostrarse el día anterior.
- **Compatibilidad de `<input type="date">`**: los navegadores actuales lo soportan; si alguno lo degradara a texto, un formato distinto de `AAAA-MM-DD` produce el error de validación en lugar de un 500.
- **CSS en caché**: mitigado con `app.css?v=4`.
- **Accesibilidad del campo en el alta**: la etiqueta es `sr-only` más `title`, igual que el título (que usa `sr-only` + placeholder); los date inputs no admiten placeholder.
- **Reversión**: `soda migrate down` elimina la columna (se pierden las fechas guardadas) y revertir el commit deja el código como estaba.
