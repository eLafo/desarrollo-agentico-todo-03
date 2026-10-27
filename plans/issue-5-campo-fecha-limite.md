# Feature: fecha límite (solo día) en las tareas

## Encargo y criterios de aceptación

Issue [#5](https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5), «Quiero añadir fecha límite a las tasks»: «Vale con que sea simplemente un día. Sin hora».

Requisitos explícitos:

- Una tarea puede tener una fecha límite.
- La fecha límite es un día del calendario, sin hora.

Inferencias (la issue no las pide, pero son necesarias para que la función sea usable sin romper lo existente):

- La fecha límite es **opcional**. Las tareas existentes y las que se crean sin fecha siguen funcionando igual.
- Se fija al crear la tarea y se puede cambiar o quitar al editarla.
- La lista muestra la fecha límite de las tareas que la tienen.
- Se acepta cualquier día válido, también los pasados (editar una tarea vencida no obliga a cambiar su fecha).

Criterios de aceptación:

1. El formulario de creación de `/` tiene un campo «Fecha límite (opcional)» de tipo `date`. Si se envía con un día, la tarea se guarda con ese día. Si se envía vacío, se guarda sin fecha límite.
2. En la lista, una tarea con fecha límite muestra `Fecha límite: DD/MM/AAAA` dentro de un `<time datetime="AAAA-MM-DD">`. Una tarea sin fecha límite no muestra nada nuevo.
3. `/tasks/{id}/edit` muestra el campo de fecha límite precargado (`value="AAAA-MM-DD"`, o vacío). Al guardar con otro día, la fecha cambia. Al guardar con el campo vacío, la fecha se quita. El estado completada/pendiente no cambia.
4. Si `due_date` no es un día válido con formato `AAAA-MM-DD` (por ejemplo `2026-02-30` o `15/11/2026`), la respuesta es `422` con el mensaje «La fecha límite no es válida. Usa el formato aaaa-mm-dd.» y no se guarda nada.
5. Si falla la validación del título en la creación, el formulario vuelve a mostrar la fecha que se envió.
6. En la base de datos, la fecha se guarda en una columna `DATE` que admite `NULL`. No hay hora ni conversión de zona horaria, así que el día que se guarda es el día que se muestra.

## Comportamiento actual y propuesta

**Actual** (rama base `origin/main`, `868fb30`):

- La tabla `tasks` solo tiene `id`, `title`, `completed`, `created_at` y `updated_at` (`migrations/20260901000000_create_tasks.up.fizz`).
- `models.Task` no tiene ningún campo de fecha de usuario.
- `TasksCreate` hace `c.Bind(task)` sobre el modelo, donde solo `title` tiene etiqueta `form`. `TasksUpdate` hace `Bind` sobre un struct anónimo que solo tiene `Title`. Ese struct actúa de lista blanca: el test `Test_TasksUpdate_PersistsTitleOnly` comprueba que se ignoran `completed`, `id` y `created_at`.
- Las plantillas solo muestran errores de `title`. El campo de título se marca `aria-invalid` cuando hay *cualquier* error (`errors.HasAny()`).

**Propuesta**: un cambio vertical mínimo que siga los patrones actuales:

- Una migración fizz añade `due_date DATE NULL`.
- `Task.DueDate *time.Time` (`db:"due_date"`, `form:"-"`). Un método `SetDueDate(string) *validate.Errors` interpreta el valor del formulario y concentra en el modelo el formato y el mensaje de error.
- Las acciones leen `due_date` como texto, llaman a `SetDueDate` y solo guardan si no hay errores de fecha.
- Las plantillas añaden el campo `type="date"`, muestran los errores por campo y pintan la fecha en la lista.
- Hay CSS mínimo para el campo nuevo y la línea de fecha en la lista.

## Alcance

Incluido:

- La migración `up` y `down` de la columna `due_date`.
- El campo, el parseo y los métodos de presentación en `models.Task`.
- Fijar la fecha al crear la tarea. Cambiarla o quitarla al editarla.
- Mostrar la fecha en la lista.
- Mostrar errores por campo (`title` y `due_date`) en los dos formularios.
- Los estilos del campo `date` y de la fecha en la lista. Subir la versión de caché de `app.css`.
- Tests de modelo y de acciones.

Excluido (no implementar):

- Hora, zona horaria y recordatorios.
- Resaltar tareas vencidas o que vencen hoy.
- Ordenar o filtrar por fecha límite. El orden actual `created_at asc, id asc` no cambia.
- Cambios en `app.js`. El toggle sigue leyendo `.task-title`, que no cambia.
- Cambios en los fixtures (`fixtures/tasks.toml`) y en la migración seed.
- Una API JSON y cambios en las rutas. Las rutas existentes sirven.

## Diseño por capas

### Persistencia

Nueva migración `migrations/20261027130000_add_due_date_to_tasks.up.fizz`:

```
add_column("tasks", "due_date", "date", {"null": true})
```

Y su `.down.fizz`:

```
drop_column("tasks", "due_date")
```

El traductor de fizz para Postgres convierte el tipo `"date"` en `DATE`. La columna admite NULL, así que las filas existentes quedan sin fecha. `migrations/embed.go` (`//go:embed *.fizz *.sql`) recoge los archivos nuevos sin cambios. `cmd/app/main.go` ejecuta `models.Migrate()` al arrancar.

### Modelo (`models/task.go`)

- Constante `DueDateLayout = "2006-01-02"`, el formato que envía y espera `<input type="date">`.
- Campo, entre `Title` y `Completed`:
  `DueDate *time.Time \`json:"due_date" db:"due_date" form:"-"\``.
  Con `form:"-"`, `c.Bind` no intenta decodificar la fecha. Así un valor mal formado no provoca un 500 del binder, y el parseo queda en un solo sitio.
- `func (t *Task) SetDueDate(value string) *validate.Errors`:
  - Hace `strings.TrimSpace(value)`. Si queda vacío, pone `t.DueDate = nil` y devuelve errores vacíos.
  - Si no, aplica `time.Parse(DueDateLayout, value)`, que da medianoche UTC. Si hay error, añade `verrs.Add("due_date", "La fecha límite no es válida. Usa el formato aaaa-mm-dd.")`, **no modifica** `t.DueDate` y devuelve los errores.
  - Si el parseo es correcto, guarda `t.DueDate = &day`.
- Métodos de presentación con **receptor valor**. Plush los invoca sobre `t` (un `Task` del `for`) y sobre `task` (`*Task`):
  - `HasDueDate() bool`
  - `DueDateValue() string`: `AAAA-MM-DD`, o `""` si no hay fecha. Se usa en `value` y en `datetime`.
  - `DueDateLabel() string`: `DD/MM/AAAA` (`"02/01/2006"`), o `""` si no hay fecha.
  - Formatean con `t.DueDate.Format(...)` **sin** `.Local()` ni `.UTC()`, para no desplazar el día.
- `Validate` no cambia.

### Acciones (`actions/tasks.go`)

`TasksCreate`, después de `task.Title = strings.TrimSpace(task.Title)`:

```go
verrs := task.SetDueDate(c.Param("due_date"))
if !verrs.HasAny() {
	var err error
	verrs, err = tx.ValidateAndCreate(task)
	if err != nil {
		return err
	}
}

if verrs.HasAny() {
	// bloque actual sin cambios: recarga tasks, c.Set(...) y render 422 de index
}
```

`c.Param` incluye los valores del cuerpo del formulario (`buffalo/context.go` hace `req.ParseForm()` y los añade a `params`).

`TasksUpdate`:

- Añade `DueDate string \`form:"due_date" json:"due_date"\`` al struct anónimo `input`, que sigue siendo la lista blanca.
- Después de asignar el título:

```go
verrs := task.SetDueDate(input.DueDate)
if !verrs.HasAny() {
	var err error
	verrs, err = tx.ValidateAndUpdate(task)
	if err != nil {
		return err
	}
}
if verrs.HasAny() {
	// bloque actual sin cambios: render 422 de edit
}
```

- Si `due_date` llega vacío **o no llega**, la fecha se quita. El formulario de edición siempre envía el campo.
- Cambia el comentario de la función a: «TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.»

`TasksIndex`, `TasksEdit`, `TasksToggle`, `TasksDestroy` y `actions/app.go` (rutas) no cambian.

Decisión aceptada: si la fecha está mal formada, solo se informa del error de fecha (no se ejecuta la validación del título en esa misma petición). Con `type="date"` en un navegador actual, esto solo ocurre con peticiones manipuladas o con navegadores sin selector de fecha.

### Renderizado (Plush)

`templates/tasks/index.plush.html`:

- Lista: justo después de `<span class="task-title"><%= t.Title %></span>`:

  ```html
  <%= if (t.HasDueDate()) { %>
    <span class="task-due-date">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></span>
  <% } %>
  ```

- Formulario de creación:
  - `.new-task-row` contiene solo `+`, la etiqueta oculta y el campo de título. El botón «Añadir» pasa a la fila nueva, para que el orden del tabulador sea título → fecha → Añadir.
  - El campo de título solo se marca `autofocus aria-invalid="true" aria-describedby="title-error"` cuando `len(errors.Get("title")) > 0`. Hoy se marca con `errors.HasAny()`.
  - Fila nueva, después de `.new-task-row`:

    ```html
    <div class="new-task-options">
      <label for="task-due-date">Fecha límite <span class="field-optional">(opcional)</span></label>
      <input
        id="task-due-date"
        type="date"
        name="due_date"
        value="<%= task.DueDateValue() %>"
        <%= if (len(errors.Get("due_date")) > 0) { %>aria-invalid="true" aria-describedby="due-date-error"<% } %>
      >
      <button type="submit" class="button button-primary">Añadir</button>
    </div>
    ```

  - Errores por campo: el bloque actual pasa a condicionarse con `len(errors.Get("title")) > 0` (sigue con `id="title-error"`). Se añade un bloque análogo `id="due-date-error"` con `errors.Get("due_date")`. Los dos llevan `class="field-error" role="alert"`.

`templates/tasks/edit.plush.html`:

- Campo de título: la condición `errors.HasAny()` pasa a ser `len(errors.Get("title")) > 0`, en el atributo y en el bloque `title-error`. El resto queda igual: `aria-describedby="title-help"` cuando no hay error.
- La pista del título queda en «Hasta 200 caracteres. El estado de la tarea no cambia.» porque ahora la fecha sí se edita.
- Después del bloque de error del título:

  ```html
  <label for="task-due-date">Fecha límite <span class="field-optional">(opcional)</span></label>
  <input
    id="task-due-date"
    type="date"
    name="due_date"
    value="<%= task.DueDateValue() %>"
    <%= if (len(errors.Get("due_date")) > 0) { %>aria-invalid="true" aria-describedby="due-date-error"<% } else { %>aria-describedby="due-date-help"<% } %>
  >
  <p id="due-date-help" class="field-hint">Déjala vacía si la tarea no tiene fecha límite.</p>
  <%= if (len(errors.Get("due_date")) > 0) { %>
    <p id="due-date-error" class="field-error" role="alert">
      <%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %>
    </p>
  <% } %>
  ```

`templates/application.plush.html`: cambia `app.css?v=3` por `app.css?v=4`. No toques la etiqueta de `app.js`.

### CSS (`public/assets/app.css`)

- Amplía el selector `input[type="text"] { … }` a `input[type="text"],\ninput[type="date"] { … }`. Justo después, añade:

  ```css
  input[type="date"] {
    width: auto;
  }
  ```

- En `.task-actions`, añade `grid-column: 3;` y `grid-row: 1;`. Así las acciones se quedan en la primera fila aunque la fecha ocupe una segunda fila de la columna 2. `.task-status` es `position: absolute` y no ocupa celda.
- Reglas nuevas, junto a los estilos de `.task-title` y de `.new-task`:

  ```css
  .task-due-date {
    grid-column: 2;
    margin-top: -0.5rem;
    padding-bottom: 0.4rem;
    color: var(--color-muted);
    font-size: 0.8rem;
  }

  .new-task-options {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.6rem;
    margin-top: 0.5rem;
    padding-left: calc(44px + 0.6rem);
  }

  .new-task-options label {
    color: var(--color-muted);
    font-size: 0.85rem;
  }

  .new-task-options .button {
    margin-left: auto;
  }

  .field-optional {
    font-weight: 400;
    color: var(--color-muted);
  }

  .edit-task label:not(:first-of-type) {
    margin-top: 1.5rem;
  }
  ```

- En `@media (max-width: 760px)`, añade:

  ```css
  .new-task-options {
    gap: 0.4rem;
    padding-left: calc(32px + 0.4rem);
  }
  ```

- En `@media (max-width: 380px)`, elimina las reglas `.new-task-row { flex-wrap: wrap; }`, `.new-task-row input { flex-basis: calc(100% - 44px); }` y `.new-task-row .button { margin-left: auto; }`, que quedan obsoletas porque el botón ya no está en esa fila. Añade:

  ```css
  .new-task-options {
    padding-left: 0;
  }
  ```

  Conserva la regla `.container` de ese bloque.

## Archivos relevantes

| Ruta | Responsabilidad |
| --- | --- |
| `migrations/20261027130000_add_due_date_to_tasks.up.fizz` (nuevo) | Añade `due_date DATE NULL` |
| `migrations/20261027130000_add_due_date_to_tasks.down.fizz` (nuevo) | Elimina `due_date` |
| `migrations/embed.go` | Incrusta las migraciones (sin cambios) |
| `models/task.go` | Campo `DueDate`, `DueDateLayout`, `SetDueDate`, métodos de presentación |
| `models/task_test.go` | Tests de parseo, persistencia y presentación |
| `actions/tasks.go` | Lee `due_date` en creación y edición; 422 si es inválida |
| `actions/tasks_test.go` | Tests HTTP de crear, editar, listar y error de fecha |
| `templates/tasks/index.plush.html` | Campo de fecha al crear, errores por campo, fecha en la lista |
| `templates/tasks/edit.plush.html` | Campo de fecha al editar, errores por campo |
| `templates/application.plush.html` | Versión de caché de `app.css` (`v=4`) |
| `public/assets/app.css` | Estilos del campo `date`, fila de opciones y fecha en la lista |
| `database.yml`, `.github/workflows/ci.yaml` | Referencia del entorno de test (sin cambios) |

## Implementación paso a paso

1. Crea las dos migraciones fizz con el contenido exacto de «Persistencia».
2. En `models/task.go`, añade la constante, el campo, `SetDueDate` y los tres métodos de presentación, con comentarios en español como los existentes. No cambies `Validate`.
3. En `actions/tasks.go`, aplica los cambios de `TasksCreate` y `TasksUpdate` descritos en «Acciones». `validate` ya está importado. No hacen falta imports nuevos en acciones.
4. Actualiza `templates/tasks/index.plush.html` y `templates/tasks/edit.plush.html` como se describe.
5. Cambia `app.css?v=3` por `app.css?v=4` en `templates/application.plush.html`.
6. Aplica los cambios de `public/assets/app.css`.
7. Añade los tests de la sección siguiente.
8. Ejecuta `gofmt -w` solo sobre los `.go` modificados y después la validación completa.

## Tests y validación

### Tests de modelo (`models/task_test.go`)

- `Test_Task_SetDueDate_Valid`: `SetDueDate("2026-11-15")` sin errores. `DueDate` tiene año 2026, mes 11, día 15 y hora 0. `DueDateValue() == "2026-11-15"`, `DueDateLabel() == "15/11/2026"` y `HasDueDate()`.
- `Test_Task_SetDueDate_EmptyClears`: con una fecha ya fijada, `SetDueDate("   ")` no da errores. Después, `DueDate == nil`, `!HasDueDate()`, `DueDateValue() == ""` y `DueDateLabel() == ""`.
- `Test_Task_SetDueDate_Invalid`: para `"2026-02-30"`, `"15/11/2026"`, `"2026-11-15T10:00"` y `"mañana"`, `verrs.Get("due_date")` contiene «La fecha límite no es válida. Usa el formato aaaa-mm-dd.» y la fecha previa sigue igual.
- `Test_Task_DueDate_Persists`: `ValidateAndCreate` con `2026-11-15`. Tras `Find` en una tarea nueva, `DueDateValue() == "2026-11-15"`. Una tarea creada sin fecha, al recargarla, tiene `DueDate == nil`.

### Tests de acciones (`actions/tasks_test.go`)

Usa `url.Values` en las peticiones con `due_date`. Puedes añadir un helper `taskWithDueDate(title, day string) *models.Task` que use `SetDueDate` y `ValidateAndCreate`.

- `Test_TasksCreate_WithDueDate`: `POST /tasks` con `title` y `due_date=2026-11-15` da 303. La tarea guardada tiene `DueDateValue() == "2026-11-15"`.
- `Test_TasksCreate_EmptyDueDate`: `due_date=""` da 303 y `DueDate == nil`.
- `Test_TasksCreate_InvalidDueDate`: `due_date=2026-02-30` da 422. El cuerpo contiene el mensaje de fecha y `aria-describedby="due-date-error"`. Hay 0 tareas.
- `Test_TasksCreate_EmptyTitle_KeepsDueDate`: `title=""` y `due_date=2026-11-15` dan 422. El cuerpo contiene «El título es obligatorio.» y `value="2026-11-15"`.
- `Test_TasksIndex_ShowsDueDate`: una tarea con `2026-11-15` y otra sin fecha. `GET /` contiene `<time datetime="2026-11-15">15/11/2026</time>` y la cadena «Fecha límite:» aparece **una sola vez** (`strings.Count`).
- `Test_TasksEdit_PrefillsDueDate`: la página de edición contiene `name="due_date"` y `value="2026-11-15"`.
- `Test_TasksUpdate_ChangesDueDate`: con una tarea completada y fechada, `PUT` con `title` y `due_date=2026-12-01` da 303. La fecha pasa a `2026-12-01` y `Completed` sigue en `true`.
- `Test_TasksUpdate_ClearsDueDate`: `PUT` con `due_date=""` da 303 y `DueDate == nil`.
- `Test_TasksUpdate_InvalidDueDate`: `PUT` con `title=Otro` y `due_date=15/11/2026` da 422 con el mensaje de fecha. En la base de datos, el título y la fecha originales no cambian.

Los tests existentes deben seguir pasando sin cambios. En particular, `Test_TasksUpdate_PersistsTitleOnly` y `Test_TasksUpdate_EmptyTitle`, que sigue esperando `aria-describedby="title-error"`.

### Entorno de test aislado (obligatorio)

Las bases compartidas del contenedor `db` tienen migraciones ajenas a esta rama:

- `todo_test` ya tiene una columna `due_date` aplicada con la versión `20261027100000` de otra rama.
- `todo_development` tiene una columna `priority`.

No migres ni borres esas bases. Usa una base propia y un `database.yml` temporal fuera del repositorio. Pop lo encuentra antes que el del repo gracias a `POP_PATH`, que se antepone a las rutas de búsqueda en `pop/config.go`.

```sh
docker compose up -d db          # si no está levantado; espera a que esté healthy
mkdir -p /tmp/todo-issue-5-campo-fecha-limite
cat > /tmp/todo-issue-5-campo-fecha-limite/database.yml <<'EOF'
test:
  dialect: postgres
  database: todo_test_issue_5_campo_fecha_limite
  user: postgres
  password: postgres
  host: 127.0.0.1
  pool: 5
EOF
CFG=/tmp/todo-issue-5-campo-fecha-limite/database.yml
SODA="go run github.com/gobuffalo/pop/v6/soda@v6.4.1"   # o soda instalado con go install …@v6.4.1, como en CI
$SODA create  -e test -c $CFG
$SODA migrate -e test -c $CFG -p migrations
$SODA migrate down -e test -c $CFG -p migrations   # revierte solo la migración nueva
$SODA migrate -e test -c $CFG -p migrations
```

Comprueba la columna:

```sh
docker compose exec -T db psql -U postgres -d todo_test_issue_5_campo_fecha_limite -c '\d tasks'
```

Debe aparecer `due_date | date` admitiendo NULL.

`soda migrate` intenta volcar `migrations/schema.sql` con `pg_dump`. Si ese archivo aparece en `git status`, bórralo: no forma parte del cambio.

### Validación

```sh
gofmt -l .                                                    # salida vacía
go vet ./...
POP_PATH=/tmp/todo-issue-5-campo-fecha-limite GO_ENV=test go test -p 1 ./...
go build ./...
git status --short   # solo el plan y los archivos de «Archivos relevantes»
```

Al terminar, puedes eliminar la base aislada con `$SODA drop -e test -c $CFG`.

### Comprobación visual (complementaria, no bloqueante)

Si hay un navegador automatizable disponible, arranca la app contra una base de desarrollo aislada. Añade una entrada `development` análoga en el `database.yml` temporal, con la base `todo_dev_issue_5_campo_fecha_limite` creada con `soda create -e development -c $CFG`. Después ejecuta:

```sh
POP_PATH=/tmp/todo-issue-5-campo-fecha-limite GO_ENV=development PORT=3001 go run ./cmd/app
```

Comprueba en `http://127.0.0.1:3001/` estos puntos, a 1280 px y a 375 px de ancho:

- El campo de fecha aparece bajo el título.
- La fecha se ve bajo el título de la tarea en la lista.
- Las acciones siguen en la primera fila.
- El formulario de edición muestra el campo.

Si no se puede hacer, indícalo como «no ejecutada (complementaria)». Los tests HTTP cubren el comportamiento.

## Riesgos y recuperación

- **Bases compartidas contaminadas**: si `todo_test` o `todo_development` se migraran con esta rama, fallaría `add_column` porque `due_date` ya existe en `todo_test`, o se mezclarían esquemas. Mitigación: la base aislada con `POP_PATH`. Si algo se aplicó por error en la base aislada, ejecuta `soda drop` y `soda create` sobre ella.
- **PRs paralelas de la misma issue**: hay otras ramas `feature/issue-5-*` con migraciones `due_date` de otra versión. Si se fusionan dos, la segunda migración fallará por la columna duplicada. Solo debe fusionarse una implementación de la issue #5.
- **Desplazamiento de día por zona horaria**: `time.Parse` sin zona devuelve UTC y Postgres `DATE` no guarda zona. pgx devuelve medianoche UTC. Formatear sin `.Local()` evita mostrar el día anterior o el siguiente. Los tests de persistencia comprueban el día exacto.
- **Navegadores sin `<input type="date">`**: muestran un campo de texto. El servidor valida el formato `aaaa-mm-dd` y el mensaje lo indica.
- **Caché del CSS**: la subida a `app.css?v=4` fuerza que el navegador descargue los estilos nuevos. Los assets van incrustados en el binario, así que la app desplegada necesita una nueva build.
- **Divergencia con `main` local**: el `main` local tiene un commit sin publicar (`429e9d6`, arreglo del toggle en `app.js`, y `app.js?v=3` en el layout). Esta rama parte de `origin/main` y no toca `app.js` ni su etiqueta `<script>`, así que no se esperan conflictos.
- **Recuperación**: la migración es aditiva y reversible (`soda migrate down` elimina la columna y los datos de fecha). Revertir el commit restaura el comportamiento anterior. En producción, revertir el código sin hacer `down` deja una columna nullable que no se usa y no causa problemas.
