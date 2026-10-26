# Feature: fecha límite (solo día) en las tareas

Origen: issue #5 — https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5
Rama de trabajo: `feature/issue-5-deadline`, creada desde `origin/main` (`ff94db6` + `868fb30`).

## Encargo y criterios de aceptación

Texto literal de la issue:

> **Quiero añadir fecha límite a las tasks**
> Vale con que sea simplemente un día. Sin hora

Requisitos explícitos:

- R1. Una tarea puede tener una fecha límite.
- R2. La fecha límite es un día (año, mes y día), sin hora ni zona horaria.

Requisitos inferidos (decisiones de este plan, no pedidas literalmente):

- I1. La fecha límite es **opcional**. Las tareas existentes quedan sin fecha y crear una tarea sin fecha sigue funcionando igual que hoy.
- I2. Se puede indicar al crear la tarea (formulario de la lista) y al editarla (página de edición).
- I3. Si se vacía el campo en la edición, se quita la fecha límite.
- I4. La lista muestra la fecha límite de cada tarea que la tenga, en formato `DD/MM/AAAA`.
- I5. No se valida que la fecha sea futura. Una tarea vencida debe poder editarse sin cambiar su fecha.
- I6. Un valor que no sea un día válido con formato `AAAA-MM-DD` se rechaza con 422, un mensaje en el campo y sin guardar nada.

Criterios de aceptación:

- CA1. Al crear una tarea con título y fecha `2026-11-15`, queda guardada con esa fecha límite y la lista muestra `15/11/2026`.
- CA2. Al crear una tarea sin fecha (campo vacío o ausente), se guarda sin fecha límite y la lista no muestra ninguna fecha para ella.
- CA3. La página de edición precarga la fecha actual en el campo de fecha. Al guardar con otra fecha, se actualiza. Al guardar con el campo vacío, la fecha se quita. El estado de completada, el `id` y `created_at` no cambian.
- CA4. Un valor inválido (`2026-02-30`, `15/11/2026`, `mañana`) en crear o editar responde 422 con el mensaje `La fecha límite no es válida. Usa el formato AAAA-MM-DD.`. El campo de fecha queda marcado con `aria-invalid="true"` y `aria-describedby="due-date-error"`, y no se crea ni se modifica ninguna tarea.
- CA5. La fecha se guarda en una columna `DATE` de PostgreSQL y se recupera como el mismo día, sin desplazamientos por zona horaria.
- CA6. Los tests existentes siguen pasando. Los errores de título siguen apareciendo en `title-error`, como ahora.

## Comportamiento actual y propuesta

Comportamiento actual (`origin/main`):

- La tabla `tasks` (`migrations/20260901000000_create_tasks.up.fizz`) tiene `id`, `title`, `completed`, `created_at` y `updated_at`. No tiene fecha límite.
- `models.Task` (`models/task.go`) solo valida el título.
- `TasksCreate` (`actions/tasks.go`) enlaza el formulario directamente sobre `models.Task`, cuyo único campo enlazable es `title` (los demás llevan `form:"-"`).
- `TasksUpdate` enlaza una estructura anónima `{Title}` y solo actualiza el título.
- `templates/tasks/index.plush.html` muestra el título de cada tarea y un formulario con un único campo, `title`. `templates/tasks/edit.plush.html` solo edita el título.
- En ambos templates, cualquier error de validación (`errors.HasAny()`) marca el campo de título como inválido, porque el título es el único campo.

Propuesta (cambio vertical mínimo):

1. Persistencia: una migración fizz nueva añade la columna `due_date` de tipo `date` y nullable.
2. Modelo: `Task.DueDate *time.Time` (`db:"due_date"`), un método `SetDueDate(string)` que interpreta `AAAA-MM-DD` y una validación que rechaza los valores no interpretables. Además, helpers de presentación con receptor por valor: `HasDueDate`, `DueDateValue` (`AAAA-MM-DD`, para `<input type="date">`) y `DueDateLabel` (`DD/MM/AAAA`, para mostrar).
3. Acciones: un tipo `taskForm` compartido (`title`, `due_date`) sustituye al enlace directo en `TasksCreate` y a la estructura anónima de `TasksUpdate`. Se mantienen `ValidateAndCreate` y `ValidateAndUpdate` y el flujo actual de errores.
4. Vistas: un campo `<input type="date" name="due_date">` en la creación y en la edición, errores por campo y la fecha visible en la lista.
5. CSS: estilos para el campo de fecha y para la línea de fecha en la lista. Se sube el parámetro de caché de `app.css`.

Decisión técnica clave: se usa `*time.Time` y no `nulls.Time`. `nulls` figura como dependencia `// indirect` en `go.mod` y usarla directamente obligaría a modificar `go.mod`. Con `*time.Time` no se toca ninguna dependencia. Se ha comprobado que pgx v5 (`stdlib.Conn.CheckNamedValue` acepta cualquier valor) codifica `*time.Time` como `DATE` y `NULL`, y que `database/sql` escanea `DATE` en `*time.Time` (`time.Time` a medianoche UTC). Pop ignora los campos con `db:"-"` (`columns/columns_for_struct.go`), y sqlx y el codificador de formularios de `httptest` ignoran los campos no exportados.

## Alcance

Incluido:

- Migración `up` y `down` para la columna `tasks.due_date` (`date`, `NULL`).
- Campo, interpretación, validación y helpers de presentación en `models.Task`.
- Enlace de `due_date` en `TasksCreate` y `TasksUpdate`.
- Campo de fecha en `templates/tasks/index.plush.html` y `templates/tasks/edit.plush.html`, y fecha visible en la lista.
- Estilos mínimos en `public/assets/app.css` y el parámetro de caché `app.css?v=4` en `templates/application.plush.html`.
- Tests de modelo y de acciones de la funcionalidad.

Excluido:

- Hora, zona horaria o recordatorios.
- Resaltar tareas vencidas o próximas a vencer. Exige definir "hoy" y la zona horaria, y no se pide.
- Ordenar o filtrar por fecha límite. El orden sigue siendo `created_at asc, id asc`.
- Cambios en `public/assets/app.js`. Su manejador del toggle sigue funcionando porque `.task-title` y `.task-actions` se conservan y `.task-actions` sigue siendo hijo directo de `li.task`.
- Cambios en `fixtures/tasks.toml`, en la migración de semillas (`20260901000001_seed_tasks`), en `go.mod` o en `go.sum`.
- El commit local `429e9d6` (fix del toggle en `app.js`), que está en `main` local pero no en `origin/main`. Esta rama no lo incluye ni debe tocar `app.js`. Ese commit cambia la línea del `<script>` de `application.plush.html`, y este plan solo cambia la línea del `<link>` del CSS.

## Diseño por capas

### Persistencia y migraciones

Archivos nuevos, siguiendo el formato fizz de `20260901000000_create_tasks`:

- `migrations/20261026000000_add_due_date_to_tasks.up.fizz`:

  ```
  add_column("tasks", "due_date", "date", {"null": true})
  ```

- `migrations/20261026000000_add_due_date_to_tasks.down.fizz`:

  ```
  drop_column("tasks", "due_date")
  ```

`migrations/embed.go` ya incluye `*.fizz`, así que no cambia. La aplicación aplica las migraciones al arrancar (`cmd/app/main.go` → `models.Migrate()`). La base de test se migra con soda (ver "Tests y validación").

### Modelo (`models/task.go`)

- Constantes:
  - `DueDateLayout = "2006-01-02"`: formato de la fecha límite (un día sin hora), el mismo que envía `<input type="date">`.
  - `dueDateLabelLayout = "02/01/2006"`: formato de presentación.
- Campos de `Task`, añadidos después de `Completed`:

  ```go
  DueDate        *time.Time `json:"due_date" db:"due_date" form:"-"`
  dueDateInvalid bool       `db:"-"`
  ```

  `dueDateInvalid` no se persiste ni se enlaza. Solo registra que el último `SetDueDate` recibió un valor no interpretable, para que `Validate` lo informe.
- `func (t *Task) SetDueDate(value string)`:
  - Recorta los espacios del valor y pone `t.dueDateInvalid = false`.
  - Si queda vacío, asigna `t.DueDate = nil` (sin fecha límite).
  - Si `time.Parse(DueDateLayout, value)` falla, asigna `t.dueDateInvalid = true` y **no** modifica `t.DueDate`.
  - Si no falla, asigna `t.DueDate = &d`. `time.Parse` devuelve medianoche UTC. No uses `Local()` ni `In()`.
- `Validate`: después de las comprobaciones actuales del título, añade:

  ```go
  if t.dueDateInvalid {
  	verrs.Add("due_date", "La fecha límite no es válida. Usa el formato AAAA-MM-DD.")
  }
  ```

  La clave del error es `due_date`. `time.Parse` ya rechaza los días inexistentes, como `2026-02-30`.
- Helpers con **receptor por valor**. Hace falta así porque en la lista Plush itera `models.Tasks` como valores `Task`, y en la edición los métodos se invocan sobre `*Task`:
  - `func (t Task) HasDueDate() bool` devuelve `t.DueDate != nil`.
  - `func (t Task) DueDateValue() string` devuelve `t.DueDate.Format(DueDateLayout)`, o `""` si no hay fecha.
  - `func (t Task) DueDateLabel() string` devuelve `t.DueDate.Format(dueDateLabelLayout)`, o `""` si no hay fecha.
- Comentarios en español, como el resto del archivo.

### Acciones (`actions/tasks.go`)

- Tipo nuevo, junto a las acciones:

  ```go
  // taskForm recoge los campos editables de una tarea desde el formulario.
  type taskForm struct {
  	Title   string `form:"title" json:"title"`
  	DueDate string `form:"due_date" json:"due_date"`
  }

  // apply copia el formulario en la tarea, normalizando el título y la fecha límite.
  func (f taskForm) apply(task *models.Task) {
  	task.Title = strings.TrimSpace(f.Title)
  	task.SetDueDate(f.DueDate)
  }
  ```

- `TasksCreate`: sustituye `c.Bind(task)` y `task.Title = strings.TrimSpace(task.Title)` por:

  ```go
  input := taskForm{}
  if err := c.Bind(&input); err != nil {
  	return err
  }
  task := &models.Task{}
  input.apply(task)
  ```

  El resto queda igual: `ValidateAndCreate`, la rama 422 que vuelve a renderizar `tasks/index.plush.html` con `tasks`, `task` y `errors`, el flash y la redirección 303.
- `TasksUpdate`: sustituye la estructura anónima y la asignación del título por `input := taskForm{}`, `c.Bind(&input)` e `input.apply(task)`. Actualiza el comentario de la función a: `// TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.` El resto (`Find` con 404, `ValidateAndUpdate`, 422 con `tasks/edit.plush.html`, flash y 303) queda igual.
- Ni las rutas (`actions/app.go`) ni las acciones `TasksIndex`, `TasksEdit`, `TasksToggle` y `TasksDestroy` cambian.

Consecuencia aceptada: un `PUT /tasks/{id}` sin el campo `due_date` quita la fecha límite. El formulario de edición siempre envía el campo.

### Renderizado (Plush)

`templates/tasks/index.plush.html`:

- En cada `li.task`, envuelve el título en un contenedor y añade la fecha cuando exista. El `form.task-toggle`, el `span.task-status` y el `div.task-actions` siguen siendo hijos directos del `li`:

  ```html
  <div class="task-body">
    <span class="task-title"><%= t.Title %></span>
    <%= if (t.HasDueDate()) { %>
      <span class="task-due">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></span>
    <% } %>
  </div>
  ```

- En el formulario `.new-task`, dentro de `.new-task-row`, entre el input `task-title` y el botón `Añadir`:

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

- El input `task-title` pasa de condicionar sus atributos con `errors.HasAny()` a hacerlo con `len(errors.Get("title")) > 0`. Mantiene los mismos atributos: `autofocus aria-invalid="true" aria-describedby="title-error"`.
- Sustituye el bloque `<%= if (errors.HasAny()) { %> <p id="title-error" …> … <% } %>` por dos bloques independientes:
  - `<p id="title-error" class="field-error" role="alert">`, con los mensajes de `errors.Get("title")`, si hay errores de título;
  - `<p id="due-date-error" class="field-error" role="alert">`, con los mensajes de `errors.Get("due_date")`, si hay errores de fecha.

`templates/tasks/edit.plush.html`:

- Agrupa el campo de título actual (label, input, hint y error) en `<div class="form-field">` y añade debajo otro `<div class="form-field">` para la fecha:

  ```html
  <div class="form-field">
    <label for="task-due-date">Fecha límite <span class="field-optional">(opcional)</span></label>
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
  </div>
  ```

- En el campo de título, sustituye `errors.HasAny()` por `len(errors.Get("title")) > 0`, tanto en los atributos del input como en el bloque `title-error`.
- Cambia el texto de `title-help` de `Hasta 200 caracteres. Los demás datos de la tarea no cambian.` a `Hasta 200 caracteres.`. La edición ya no cambia solo el título.

`templates/application.plush.html`: cambia `/assets/app.css?v=3` a `/assets/app.css?v=4`. No toques la línea del `<script>`.

### CSS (`public/assets/app.css`)

- Sustituye la regla `.task-title` por estas reglas: el contenedor hereda la alineación, el `min-width` y el padding, y el título conserva el ajuste de línea.

  ```css
  .task-body {
    display: flex;
    flex-direction: column;
    align-self: center;
    min-width: 0;
    padding: 0.65rem 0;
  }

  .task-title {
    overflow-wrap: anywhere;
    line-height: 1.55;
  }

  .task-due {
    color: var(--color-muted);
    font-size: 0.8rem;
  }
  ```

- Amplía el selector de la regla `input[type="text"]` a `input[type="text"], input[type="date"]` (un selector por línea, como el resto del archivo). Su contenido no cambia.
- Después de `.new-task-row input { flex: 1; }`, añade:

  ```css
  .new-task-row input[type="date"] {
    flex: 0 0 auto;
    width: auto;
  }
  ```

- Dentro de `@media (max-width: 760px)`, añade estas reglas para que el título ocupe la primera línea y la fecha y el botón pasen a la segunda, alineados con el título:

  ```css
  .new-task-row {
    flex-wrap: wrap;
  }

  .new-task-row input[type="text"] {
    flex-basis: calc(100% - 3rem);
  }

  .new-task-row input[type="date"] {
    flex: 1 1 auto;
    margin-left: calc(32px + 0.4rem);
  }
  ```

- Añade `.form-field + .form-field { margin-top: 1.25rem; }` y `.field-optional { color: var(--color-muted); font-weight: 400; }` junto a las reglas de `.edit-task`.

Las demás reglas no cambian. `.task-completed .task-title` sigue tachando solo el título.

## Archivos relevantes

| Ruta | Responsabilidad | Cambio |
| --- | --- | --- |
| `migrations/20261026000000_add_due_date_to_tasks.up.fizz` | Añadir la columna `due_date` | Nuevo |
| `migrations/20261026000000_add_due_date_to_tasks.down.fizz` | Revertir la columna | Nuevo |
| `migrations/embed.go` | Embeber `*.fizz` y `*.sql` | Sin cambios |
| `models/task.go` | Modelo `Task`, validación y helpers | Modificado |
| `models/task_test.go` | Tests de modelo (`ModelSuite`) | Modificado (tests nuevos) |
| `actions/tasks.go` | Acciones de tareas | Modificado (`taskForm`, `TasksCreate`, `TasksUpdate`) |
| `actions/tasks_test.go` | Tests de acciones (`ActionSuite`) | Modificado (tests nuevos) |
| `actions/app.go` | Rutas | Sin cambios |
| `templates/tasks/index.plush.html` | Lista y formulario de creación | Modificado |
| `templates/tasks/edit.plush.html` | Formulario de edición | Modificado |
| `templates/application.plush.html` | Layout y versión de assets | Modificado (`app.css?v=4`) |
| `public/assets/app.css` | Estilos | Modificado |
| `public/assets/app.js` | Toggle optimista | Sin cambios |
| `database.yml` | Conexiones `development` y `test` (PostgreSQL en `127.0.0.1`) | Sin cambios |
| `.github/workflows/ci.yaml` | Referencia de validación (gofmt, vet, `soda migrate -e test`, `go test -p 1`, build) | Sin cambios |

## Implementación paso a paso

1. Comprueba que estás en `feature/issue-5-deadline` con el árbol limpio, salvo este plan (`git status --short --branch`).
2. Crea las dos migraciones fizz de "Persistencia y migraciones".
3. Modifica `models/task.go` según "Modelo".
4. Modifica `actions/tasks.go` según "Acciones".
5. Modifica `templates/tasks/index.plush.html`, `templates/tasks/edit.plush.html` y `templates/application.plush.html` según "Renderizado".
6. Modifica `public/assets/app.css` según "CSS".
7. Añade los tests de "Tests y validación" a `models/task_test.go` y `actions/tasks_test.go`, sin modificar los tests existentes.
8. Ejecuta `gofmt -w` solo sobre los `.go` modificados y luego la validación completa.
9. Revisa `git diff --check`, `git diff` y `git status --short`. Deben aparecer solo los archivos de la tabla marcados como nuevos o modificados, además de este plan. Si aparece `migrations/schema.sql` (soda lo genera solo si `pg_dump` está instalado), bórralo: es un artefacto, no forma parte del cambio.

## Tests y validación

### Tests de modelo (`models/task_test.go`, métodos de `ModelSuite`)

- `Test_Task_DueDate_OptionalByDefault`: crea `&Task{Title: "Sin fecha"}`, comprueba que es válido y, tras recargarla con `Find`, que `DueDate` es `nil`, `HasDueDate()` es `false` y `DueDateValue()` es `""`.
- `Test_Task_DueDate_PersistsDay`: `SetDueDate("2026-11-15")` y `ValidateAndCreate` sin errores. Tras `Find`: `DueDateValue() == "2026-11-15"`, `DueDateLabel() == "15/11/2026"`, y hora, minuto y segundo a cero.
- `Test_Task_DueDate_Invalid`: para `"2026-02-30"`, `"15/11/2026"` y `"mañana"`, `SetDueDate(v)` y `ValidateAndCreate` producen `verrs.Get("due_date")`, que contiene `La fecha límite no es válida. Usa el formato AAAA-MM-DD.`. Al final, `Count` es 0.
- `Test_Task_DueDate_Clear`: `SetDueDate("2026-11-15")` seguido de `SetDueDate("  ")` deja `DueDate == nil` y sin errores de validación. Un valor inválido seguido de uno vacío también queda sin error.

### Tests de acciones (`actions/tasks_test.go`, métodos de `ActionSuite`)

Usa `url.Values` en los POST y PUT, como los tests existentes.

- `Test_TasksCreate_WithDueDate`: POST `/tasks` con `title=Con fecha` y `due_date=2026-11-15` responde 303 a `/`. La tarea guardada tiene `DueDateValue() == "2026-11-15"`.
- `Test_TasksCreate_EmptyDueDate`: POST con `title=Sin fecha` y `due_date=` responde 303. La tarea guardada no tiene fecha límite (`HasDueDate()` es `false`).
- `Test_TasksCreate_InvalidDueDate`: POST con `title=Con fecha mala` y `due_date=2026-02-30` responde 422. El cuerpo contiene el mensaje de CA4, `id="due-date-error"` y `aria-describedby="due-date-error"`, y no contiene `aria-describedby="title-error"`. `Count` es 0.
- `Test_TasksIndex_ShowsDueDate`: crea una tarea con `SetDueDate("2026-11-15")`. `GET /` responde 200 y el cuerpo contiene `datetime="2026-11-15"`, `15/11/2026` y `Fecha límite:`.
- `Test_TasksIndex_WithoutDueDate`: con una tarea sin fecha, `GET /` no contiene `class="task-due"`.
- `Test_TasksEdit_PrefillsDueDate`: con una tarea con fecha, `GET /tasks/{id}/edit` contiene `name="due_date"` y `value="2026-11-15"`.
- `Test_TasksUpdate_SetsDueDate`: con una tarea completada sin fecha, PUT con `title=Título actualizado` y `due_date=2026-12-01` responde 303. Tras `Reload`: fecha `2026-12-01`, título actualizado y `Completed` sigue en `true`.
- `Test_TasksUpdate_ClearsDueDate`: con una tarea con fecha, PUT con `title=Título original` y `due_date=` responde 303 y, tras `Reload`, `HasDueDate()` es `false`.
- `Test_TasksUpdate_InvalidDueDate`: con una tarea con fecha `2026-11-15`, PUT con `title=Otro título` y `due_date=mañana` responde 422. El cuerpo contiene el mensaje de CA4 y `aria-describedby="due-date-error"`. Tras `Reload`, el título sigue en `Título original` y la fecha en `2026-11-15`.

Los tests existentes deben seguir pasando sin cambios, en particular `Test_TasksCreate_Valid`, `Test_TasksUpdate_PersistsTitleOnly` y `Test_TasksUpdate_EmptyTitle` (que comprueba `aria-describedby="title-error"`).

### Preparación de la base de datos de test

Los tests necesitan PostgreSQL en `127.0.0.1:5432` (servicio `db` de `compose.yaml`, levantado con `docker compose up -d db` si no responde) y la base `todo_test` migrada con las migraciones de **esta** rama.

Estado observado al planificar: `todo_test` ya tiene una versión `20261026000000` aplicada y una columna `due_date`, restos de ejecuciones anteriores de otras ramas. Ese estado no es fiable, así que hay que recrear la base de test. Desde la raíz del repositorio, **siempre con `-e test`** (sin `-e test` soda actuaría sobre `todo_development`):

```sh
go run github.com/gobuffalo/pop/v6/soda@v6.4.1 drop -e test
go run github.com/gobuffalo/pop/v6/soda@v6.4.1 create -e test
go run github.com/gobuffalo/pop/v6/soda@v6.4.1 migrate -e test
go run github.com/gobuffalo/pop/v6/soda@v6.4.1 migrate down -e test
go run github.com/gobuffalo/pop/v6/soda@v6.4.1 migrate -e test
docker exec desarrollo-agentico-todo-03-db-1 psql -U postgres -d todo_test -c '\d tasks'
```

`migrate down` seguido de `migrate` comprueba que la migración es reversible. `\d tasks` debe mostrar `due_date | date` nullable y `schema_migration` debe contener solo `20260901000000`, `20260901000001` y `20261026000000`. Nunca ejecutes `drop` sobre `development` ni `production`.

### Comandos de validación (en este orden)

```sh
gofmt -l .                       # salida vacía
go vet ./...
GO_ENV=test go test -p 1 ./...   # GO_ENV=test es obligatorio: models.DB usa "development" por defecto
go build ./...
git diff --check
```

## Riesgos y recuperación

- **Base de test contaminada por otras ramas.** Recréala como se indica arriba. Si `soda drop` falla por conexiones abiertas, cierra los procesos de test colgados y repite. No toques `todo_development`.
- **Desfase de zona horaria.** La fecha debe ir y volver siempre como `time.Time` a medianoche UTC (`time.Parse`) contra una columna `DATE`. `Test_Task_DueDate_PersistsDay` lo comprueba. Si fallara, revisa que no se use `Local()` o `In()` y que la columna sea `date` y no `timestamp`.
- **Plush y receptores.** Si Plush no encuentra `HasDueDate`, `DueDateValue` o `DueDateLabel` en la lista, revisa que tengan receptor por valor (`func (t Task)`). Con receptor puntero no son invocables sobre los elementos de `models.Tasks`.
- **Maquetación de la fila de la lista.** Si `.task-actions` deja de ser hijo directo de `li.task`, el `insertBefore` de `app.js` falla. El contenedor `.task-body` solo debe envolver el título y la fecha.
- **PUT sin `due_date`.** Quita la fecha (consecuencia aceptada en "Acciones"). Si se considerase un problema, distinguir campo ausente de vacío queda fuera de este plan.
- **Caché de assets.** Sin el cambio a `app.css?v=4`, los navegadores podrían seguir usando el CSS anterior.
- **Conflicto con `main` local.** El commit `429e9d6` (sin publicar en `origin/main`) cambia la línea del `<script>` de `application.plush.html` y el archivo `app.js`. Este plan no toca ninguno de los dos, así que la integración posterior no debería generar conflictos.
- **Recuperación.** Toda la funcionalidad está en los archivos de la tabla. Para revertirla en una base ya migrada, ejecuta `soda migrate down -e <entorno>` (que elimina `due_date`) y descarta los cambios de la rama.
