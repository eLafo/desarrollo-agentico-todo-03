# Feature: fecha límite (solo día) en las tareas

## Encargo y criterios de aceptación

Encargo: issue #5 (https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5), «Quiero añadir fecha límite a las tasks». Cuerpo: «Vale con que sea simplemente un día. Sin hora».

Requisitos explícitos:

- Una tarea puede tener una fecha límite.
- La fecha límite es un día del calendario, sin hora.

Inferencias (decididas en este plan):

- La fecha límite es **opcional**: las tareas existentes y las nuevas pueden no tenerla.
- Se puede indicar al crear la tarea, y cambiar o quitar al editarla.
- Se muestra en la lista junto a la tarea.
- Se acepta cualquier día válido, incluido un día pasado (editar una tarea vencida no obliga a cambiarle la fecha).
- No se resalta si la tarea está vencida, no se ordena ni se filtra por fecha (ver «Excluido»).

Criterios de aceptación:

1. El formulario «Añadir tarea» de `/` tiene un campo `type="date"` llamado `due_date` (etiqueta accesible «Fecha límite (opcional)»). Si se envía con un día (`AAAA-MM-DD`), la tarea se crea con esa fecha límite; si se envía vacío o sin el campo, se crea sin fecha límite.
2. En la lista, una tarea con fecha límite muestra bajo su título `Fecha límite: <time datetime="AAAA-MM-DD">DD/MM/AAAA</time>`. Una tarea sin fecha límite no muestra nada (no hay elemento `.task-due`).
3. El formulario de edición (`/tasks/{id}/edit`) tiene un campo `type="date"` `due_date` precargado con la fecha límite actual (o vacío). Al guardar con un día, la fecha cambia; al guardar vacío, la tarea queda sin fecha límite. El estado completada/pendiente se conserva.
4. Un valor que no sea un día válido en formato `AAAA-MM-DD` (p. ej. `30/11/2026`, `2026-02-30`, `mañana`) no se guarda: la respuesta es 422, vuelve a mostrar el formulario con el mensaje «La fecha límite no es válida. Usa el formato AAAA-MM-DD.» en `#due-date-error`, el campo tiene `aria-invalid="true"` y `aria-describedby="due-date-error"`, y la base de datos no cambia.
5. Los errores del título siguen funcionando igual que hoy (mensajes, `aria-describedby="title-error"`), y un error de fecha no marca el título como inválido.
6. La fecha se persiste en una columna `date` nullable: el día guardado es exactamente el elegido, sin desplazamientos por zona horaria.

## Comportamiento actual y propuesta

Actual (rama creada desde `origin/main`, árbol limpio):

- `migrations/20260901000000_create_tasks.up.fizz` crea `tasks(id uuid, title varchar(200), completed bool default false, created_at, updated_at)`. `migrations/20260901000001_seed_tasks.up.sql` inserta tres tareas con lista explícita de columnas.
- `models/task.go`: `Task{ID, Title, Completed, CreatedAt, UpdatedAt}`; `Validate` solo valida el título (obligatorio, máx. `TitleMaxLength` = 200 runas).
- `actions/tasks.go`: `TasksCreate` hace `c.Bind(task)` sobre `models.Task`, recorta el título y llama a `tx.ValidateAndCreate`; con errores re-renderiza `tasks/index.plush.html` con 422. `TasksUpdate` busca la tarea, hace `c.Bind` a un struct anónimo `input{Title}`, asigna el título recortado y llama a `tx.ValidateAndUpdate`; con errores re-renderiza `tasks/edit.plush.html` con 422. Rutas en `actions/app.go` (no cambian).
- `templates/tasks/index.plush.html`: cada `<li class="task">` es un grid de 3 columnas (form de toggle, `<span class="task-title">`, `.task-actions`); `.task-status` («Completada») está posicionado de forma absoluta (sr-only) y no ocupa celda. El formulario de alta muestra errores con `errors.HasAny()` y solo pinta `errors.Get("title")`.
- `templates/tasks/edit.plush.html`: un único campo de título; `errors.HasAny()` decide `aria-invalid`/`aria-describedby`.
- `public/assets/app.js` usa `.task-title` (texto para `aria-label`) y `task.insertBefore(status, task.querySelector(".task-actions"))`; ambos selectores deben seguir funcionando.
- `templates/application.plush.html` carga `/assets/app.css?v=3`.

Propuesta (cambio vertical mínimo, siguiendo los patrones existentes):

- Nueva migración fizz que añade `due_date date NULL` a `tasks`.
- `Task.DueDate nulls.Time` (`github.com/gobuffalo/nulls`, ya presente en `go.mod` como indirecta) y un método `SetDueDate(string)` que interpreta el valor del campo `type="date"`; si no es válido, deja la fecha vacía y `Validate` añade el error en la clave `due_date`. Así el flujo de validación de las acciones (`ValidateAndCreate`/`ValidateAndUpdate` → 422) no cambia.
- Métodos de presentación `DueDateValue()` (`AAAA-MM-DD`, para `value` y `datetime`) y `DueDateLabel()` (`DD/MM/AAAA`) con **receptor por valor**, porque la plantilla de la lista itera `Tasks` (valores `Task`) y Plush no encuentra métodos de receptor puntero sobre valores no direccionables.
- Las acciones solo añaden una línea cada una para leer `due_date`.
- Plantillas: campo de fecha en alta y edición, errores por campo, fecha bajo el título en la lista.
- CSS para el campo `date`, el nuevo envoltorio `.task-body` y el móvil; subir la versión de `app.css` en el layout.

## Alcance

Incluido:

- Migración up/down de la columna `due_date`.
- Modelo: campo, asignación desde formulario, validación de formato y métodos de presentación.
- Acciones `TasksCreate` y `TasksUpdate`.
- Plantillas `tasks/index.plush.html`, `tasks/edit.plush.html` y la versión de `app.css` en `application.plush.html`.
- Estilos en `public/assets/app.css`.
- `go.mod`: `github.com/gobuffalo/nulls` pasa de indirecta a directa (vía `go mod tidy`).
- Tests de modelo y de acciones descritos abajo.

Excluido:

- Hora, zona horaria, recordatorios o notificaciones.
- Resaltar tareas vencidas o «vence hoy» (depende de qué es «hoy» en cada zona horaria; no se pide).
- Ordenar o filtrar por fecha límite; el orden sigue siendo `created_at asc, id asc`.
- Cambios en `public/assets/app.js`, en rutas (`actions/app.go`), en `fixtures/tasks.toml` y en la migración de seed.
- Arreglar el `fetch(form.action)` del toggle en `app.js` u otros problemas ajenos (el commit local `429e9d6` de `main` sin publicar no forma parte de esta rama).
- La base de datos de desarrollo (`todo_development`), que hoy contiene esquema de otra rama (columna `priority`, versión `20261014000000`); no se toca ni se usa para validar.

## Diseño por capas

### Persistencia y migraciones

`migrations/20261026230000_add_due_date_to_tasks.up.fizz`:

```
add_column("tasks", "due_date", "date", {"null": true})
```

`migrations/20261026230000_add_due_date_to_tasks.down.fizz`:

```
drop_column("tasks", "due_date")
```

`migrations/embed.go` ya incluye `*.fizz`; no cambia. La seed SQL usa lista explícita de columnas, así que no se ve afectada. No usar la versión `20261026210000` (la base de test local la tiene registrada por una rama anterior).

### Modelo (`models/task.go`)

- Importar `github.com/gobuffalo/nulls`.
- Constantes nuevas junto a `TitleMaxLength`:

```go
// DueDateLayout es el formato, solo día y sin hora, de la fecha límite en los campos date.
const DueDateLayout = "2006-01-02"

// DueDateLabelLayout es el formato con el que se muestra la fecha límite en la lista.
const DueDateLabelLayout = "02/01/2006"
```

- Struct (orden de campos: `DueDate` tras `Title`; campo no exportado al final):

```go
type Task struct {
	ID        uuid.UUID  `json:"id" db:"id" form:"-"`
	Title     string     `json:"title" db:"title" form:"title"`
	DueDate   nulls.Time `json:"due_date" db:"due_date" form:"-"`
	Completed bool       `json:"completed" db:"completed" form:"-"`
	CreatedAt time.Time  `json:"created_at" db:"created_at" form:"-"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at" form:"-"`

	// invalidDueDate indica que SetDueDate recibió un valor que no es un día válido.
	invalidDueDate bool `db:"-" form:"-"`
}
```

  `db:"-"` es obligatorio: pop usa el nombre del campo como columna cuando no hay etiqueta `db`. `form:"-"` en `DueDate` evita que el binder de Buffalo intente decodificarlo (su decodificador registra un `nulls.Time` interno distinto).

- Métodos:

```go
// SetDueDate asigna la fecha límite a partir del valor de un campo date (AAAA-MM-DD).
// Un valor vacío deja la tarea sin fecha límite. Un valor que no es un día válido
// también la deja vacía y hace que Validate informe del error.
func (t *Task) SetDueDate(value string) {
	t.DueDate = nulls.Time{}
	t.invalidDueDate = false

	value = strings.TrimSpace(value)
	if value == "" {
		return
	}

	day, err := time.Parse(DueDateLayout, value)
	if err != nil {
		t.invalidDueDate = true
		return
	}
	t.DueDate = nulls.NewTime(day)
}

// DueDateValue devuelve la fecha límite en formato AAAA-MM-DD, o "" si no tiene.
func (t Task) DueDateValue() string {
	if !t.DueDate.Valid {
		return ""
	}
	return t.DueDate.Time.Format(DueDateLayout)
}

// DueDateLabel devuelve la fecha límite en formato DD/MM/AAAA, o "" si no tiene.
func (t Task) DueDateLabel() string {
	if !t.DueDate.Valid {
		return ""
	}
	return t.DueDate.Time.Format(DueDateLabelLayout)
}
```

  `time.Parse` con ese layout devuelve medianoche UTC y rechaza días inexistentes (`2026-02-30`). pgx codifica `time.Time` en una columna `date` usando año/mes/día del propio valor y devuelve las fechas como medianoche UTC, así que el día se conserva.

- En `Validate`, tras la validación del título:

```go
	if t.invalidDueDate {
		verrs.Add("due_date", "La fecha límite no es válida. Usa el formato AAAA-MM-DD.")
	}
```

### Acciones (`actions/tasks.go`)

- `TasksCreate`: tras `task.Title = strings.TrimSpace(task.Title)` añadir `task.SetDueDate(c.Param("due_date"))` (`c.Param` incluye los valores del cuerpo del formulario). El resto no cambia.
- `TasksUpdate`: ampliar el struct de entrada y asignar la fecha:

```go
	input := struct {
		Title   string `form:"title" json:"title"`
		DueDate string `form:"due_date" json:"due_date"`
	}{}
	...
	task.Title = strings.TrimSpace(input.Title)
	task.SetDueDate(input.DueDate)
```

  Actualizar el comentario de la función a: `// TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.` El formulario de edición siempre envía `due_date`; si falta o está vacío, la tarea queda sin fecha límite.

Rutas sin cambios.

### Renderizado (Plush)

`templates/tasks/index.plush.html`, en cada `<li>` sustituir `<span class="task-title"><%= t.Title %></span>` por:

```html
          <div class="task-body">
            <span class="task-title"><%= t.Title %></span>
            <%= if (t.DueDate.Valid) { %>
              <span class="task-due">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></span>
            <% } %>
          </div>
```

(`.task-title` y `.task-actions` mantienen nombre y posición relativa, así que `app.js` sigue funcionando.)

Formulario de alta (`<form action="/tasks" ...>`): sustituir su contenido tras el `authenticity_token` por:

```html
  <div class="new-task-row">
    <span class="add-mark" aria-hidden="true">+</span>
    <label for="task-title" class="sr-only">Nueva tarea</label>
    <input
      id="task-title"
      type="text"
      name="title"
      value="<%= task.Title %>"
      placeholder="Añadir una tarea…"
      maxlength="200"
      autocomplete="off"
      <%= if (len(errors.Get("title")) > 0) { %>autofocus aria-invalid="true" aria-describedby="title-error"<% } %>
    >
    <label for="task-due-date" class="sr-only">Fecha límite (opcional)</label>
    <input
      id="task-due-date"
      type="date"
      name="due_date"
      value="<%= task.DueDateValue() %>"
      title="Fecha límite (opcional)"
      <%= if (len(errors.Get("due_date")) > 0) { %>autofocus aria-invalid="true" aria-describedby="due-date-error"<% } %>
    >
    <button type="submit" class="button button-primary">Añadir</button>
  </div>
  <%= if (len(errors.Get("title")) > 0) { %>
    <p id="title-error" class="field-error" role="alert">
      <%= for (msg) in errors.Get("title") { %><%= msg %><% } %>
    </p>
  <% } %>
  <%= if (len(errors.Get("due_date")) > 0) { %>
    <p id="due-date-error" class="field-error" role="alert">
      <%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %>
    </p>
  <% } %>
```

`templates/tasks/edit.plush.html`:

- En el input del título, cambiar la condición `errors.HasAny()` por `len(errors.Get("title")) > 0` (mismo contenido en ambas ramas: `aria-invalid="true" aria-describedby="title-error"` / `aria-describedby="title-help"`).
- Texto de ayuda del título: `Hasta 200 caracteres. El estado de la tarea no cambia.`
- El bloque `title-error` pasa a condicionarse con `len(errors.Get("title")) > 0`.
- Tras ese bloque y antes de `<div class="edit-task-actions">`, añadir:

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

`templates/application.plush.html`: `/assets/app.css?v=3` → `/assets/app.css?v=4`.

### CSS (`public/assets/app.css`)

1. Regla `.task-title`: mover `align-self`, `min-width` y `padding` a un envoltorio nuevo y añadir la fecha:

```css
.task-body {
  align-self: center;
  min-width: 0;
  padding: 0.65rem 0;
}

.task-title {
  overflow-wrap: anywhere;
  line-height: 1.55;
}

.task-due {
  display: block;
  color: var(--color-muted);
  font-size: 0.8rem;
}
```

2. Selector `input[type="text"] { ... }` → `input[type="text"],\ninput[type="date"] { ... }` (mismas declaraciones), y justo después:

```css
input[type="date"] {
  width: auto;
}
```

3. Tras `.new-task-row input { flex: 1; }`:

```css
.new-task-row input[type="date"] {
  flex: 0 0 auto;
}
```

4. Tras la regla `.edit-task label { ... }`:

```css
.edit-task label:not(:first-of-type) {
  margin-top: 1.5rem;
}
```

5. En `@media (max-width: 760px)`, ampliar la regla existente `.new-task-row { gap: 0.4rem; }` con `flex-wrap: wrap;` y añadir dentro del mismo bloque (título en la primera línea; fecha y botón en la segunda):

```css
  .new-task-row input[type="text"] {
    flex-basis: calc(100% - 44px);
  }

  .new-task-row input[type="date"] {
    margin-left: calc(32px + 0.4rem);
  }

  .new-task-row .button {
    margin-left: auto;
  }
```

La regla de `@media (max-width: 380px)` se deja como está.

## Archivos relevantes

| Ruta | Responsabilidad | Cambio |
| --- | --- | --- |
| `migrations/20261026230000_add_due_date_to_tasks.up.fizz` | Añadir columna `due_date date NULL` | Nuevo |
| `migrations/20261026230000_add_due_date_to_tasks.down.fizz` | Eliminar la columna | Nuevo |
| `migrations/embed.go` | Embebe `*.fizz *.sql` para `models.Migrate` | Sin cambios |
| `models/task.go` | Modelo, validación, parseo y formato de la fecha | Modificar |
| `models/task_test.go` | Tests de modelo | Ampliar |
| `actions/tasks.go` | Alta y edición leen `due_date` | Modificar |
| `actions/tasks_test.go` | Tests de acciones y renderizado | Ampliar |
| `templates/tasks/index.plush.html` | Lista y formulario de alta | Modificar |
| `templates/tasks/edit.plush.html` | Formulario de edición | Modificar |
| `templates/application.plush.html` | Layout; versión de `app.css` | `?v=3` → `?v=4` |
| `public/assets/app.css` | Estilos | Modificar |
| `public/assets/app.js` | Toggle optimista; depende de `.task-title` y `.task-actions` | Sin cambios |
| `go.mod` | `github.com/gobuffalo/nulls` pasa a dependencia directa | Modificar (vía `go mod tidy`) |
| `database.yml` | `test` → `todo_test` en `127.0.0.1:5432` | Sin cambios |

## Implementación paso a paso

1. Comprobar `git status --short --branch` (rama `feature/issue-5-vencimiento-tareas`, solo este plan sin commit).
2. Crear las dos migraciones fizz con el contenido indicado.
3. Modificar `models/task.go`: import de `nulls`, constantes, campos `DueDate` e `invalidDueDate`, `SetDueDate`, `DueDateValue`, `DueDateLabel` y el error en `Validate`.
4. Ejecutar `go mod tidy` y comprobar con `git diff go.mod go.sum` que el único cambio es mover `github.com/gobuffalo/nulls v0.4.2` al bloque de dependencias directas (sin `// indirect`). Si aparece cualquier otro cambio, detenerse e informar.
5. Modificar `actions/tasks.go` (`TasksCreate`, `TasksUpdate` y su comentario).
6. Modificar `templates/tasks/index.plush.html`, `templates/tasks/edit.plush.html` y `templates/application.plush.html`.
7. Modificar `public/assets/app.css`.
8. Añadir los tests de las secciones siguientes.
9. Preparar la base de test y ejecutar la validación.

## Tests y validación

### Tests de modelo (`models/task_test.go`)

Comparar fechas con `DueDateValue()`/`DueDateLabel()` (no hace falta importar `time`).

- `Test_Task_DueDate_Optional`: `&Task{Title: "Sin fecha"}` → `ValidateAndCreate` sin errores; recargar con `ms.DB.Find(reloaded, task.ID)` → `reloaded.DueDate.Valid` es `false` y `DueDateValue()` es `""`.
- `Test_Task_SetDueDate_PersistsDay`: `SetDueDate("2026-11-15")` → sin errores; recargado: `DueDate.Valid`, `DueDateValue() == "2026-11-15"`, `DueDateLabel() == "15/11/2026"`.
- `Test_Task_SetDueDate_Blank`: `SetDueDate("   ")` → sin errores y `DueDate.Valid == false`.
- `Test_Task_SetDueDate_Invalid`: para cada valor de `[]string{"30/11/2026", "2026-02-30", "mañana", "2026-11-15T10:00"}`, una tarea nueva con título válido y `SetDueDate(valor)` → `verrs.Get("due_date")` contiene «La fecha límite no es válida. Usa el formato AAAA-MM-DD.»; al final `ms.DB.Count(&Task{})` es 0.
- `Test_Task_SetDueDate_ClearsPreviousError`: `SetDueDate("mañana")` y después `SetDueDate("2026-11-15")` → `Validate` sin errores.

### Tests de acciones (`actions/tasks_test.go`)

Añadir un helper:

```go
func (as *ActionSuite) taskWithDueDate(dueDate string) *models.Task {
	task := &models.Task{Title: "Título original"}
	task.SetDueDate(dueDate)
	verrs, err := as.DB.ValidateAndCreate(task)
	as.NoError(err)
	as.False(verrs.HasAny())
	as.NoError(as.DB.Reload(task))
	return task
}
```

- `Test_TasksCreate_WithDueDate`: POST `url.Values{"title": {"Con fecha"}, "due_date": {"2026-11-15"}}` → 303 a `/`; `as.DB.First(task)` → `DueDateValue() == "2026-11-15"`.
- `Test_TasksCreate_EmptyDueDate`: POST `title` + `due_date` `""` → 303; la tarea no tiene fecha (`DueDate.Valid == false`).
- `Test_TasksCreate_InvalidDueDate`: POST `title` «Con fecha» + `due_date` «30/11/2026» → 422; el cuerpo contiene el mensaje de error, `id="due-date-error"`, `aria-describedby="due-date-error"` y `value="Con fecha"`; no contiene `aria-describedby="title-error"`; `Count` es 0.
- `Test_TasksIndex_ShowsDueDate`: `taskWithDueDate("2026-11-15")` → GET `/` contiene `datetime="2026-11-15"` y `15/11/2026`.
- `Test_TasksIndex_WithoutDueDate`: `taskForEditing(false)` → GET `/` no contiene `class="task-due"`.
- `Test_TasksIndex_DueDateField`: GET `/` contiene `name="due_date"` y `type="date"`.
- `Test_TasksEdit_PrefillsDueDate`: `taskWithDueDate("2026-11-15")` → GET edit contiene `name="due_date"` y `value="2026-11-15"`.
- `Test_TasksUpdate_ChangesDueDate`: `taskForEditing(true)` → PUT `title` «Título original» + `due_date` «2026-12-01» → 303; recargada: `DueDateValue() == "2026-12-01"` y `Completed` sigue `true`.
- `Test_TasksUpdate_ClearsDueDate`: `taskWithDueDate("2026-11-15")` → PUT `title` + `due_date` `""` → 303; recargada sin fecha.
- `Test_TasksUpdate_InvalidDueDate`: `taskWithDueDate("2026-11-15")` → PUT `title` «Nuevo título» + `due_date` «mañana» → 422; cuerpo contiene el mensaje y `aria-describedby="due-date-error"`; recargada: título «Título original» y `DueDateValue() == "2026-11-15"`.

Los tests existentes deben seguir pasando sin cambios (incluidos `Test_TasksUpdate_EmptyTitle`, que espera `aria-describedby="title-error"`, y `Test_TasksUpdate_PersistsTitleOnly`).

### Condiciones de ejecución

- PostgreSQL en `127.0.0.1:5432` con usuario/contraseña `postgres` (en local: contenedor `desarrollo-agentico-todo-03-db-1` de `compose.yaml`; si no está levantado, `docker compose up -d db`).
- `soda`: usar `soda` del `PATH` o `$(go env GOPATH)/bin/soda`; si no existe, `go install github.com/gobuffalo/pop/v6/soda@v6.4.1` (la misma versión que CI).
- La base `todo_test` es desechable y hoy contiene esquema de otra rama (columna `due_date` y versión `20261026210000` ajenas a esta rama), por lo que `soda migrate` fallaría con «column already exists». Recrearla desde las migraciones de esta rama (no usar `soda reset`, que carga `schema.sql`):

```sh
soda drop -e test
soda create -e test
soda migrate -e test
```

- Los tests deben ejecutarse con `GO_ENV=test`; sin él, `models.DB` (que usan las acciones) se conecta a `todo_development`.

### Comandos de validación

```sh
gofmt -l .                              # salida vacía
go vet ./...
go mod tidy -diff                       # salida vacía tras el paso 4
soda drop -e test && soda create -e test && soda migrate -e test
soda migrate down -e test && soda migrate -e test   # la migración down/up es reversible
GO_ENV=test go test -p 1 ./...
go build ./...
git diff --check
```

No se requiere verificación manual en navegador ni contra la base de desarrollo.

## Riesgos y recuperación

- **Esquema residual en la base de test**: mitigado recreando `todo_test` como se indica. Si `soda drop` falla por conexiones abiertas, cerrar procesos de tests/servidor que usen `todo_test` y repetir.
- **Zona horaria**: la fecha se parsea a medianoche UTC y se guarda como `date`; pgx usa año/mes/día del valor, y al leer devuelve medianoche UTC, así que el día no se desplaza. Los tests de persistencia (`2026-11-15` ida y vuelta) lo verifican.
- **Navegadores sin `type="date"`**: muestran un campo de texto; si el usuario escribe otro formato, recibe el error con el formato esperado. Tras un error de fecha el campo se vacía (el valor inválido no se conserva); se acepta.
- **Petición de edición sin `due_date`**: borra la fecha límite. El formulario de edición siempre envía el campo, así que solo afecta a peticiones manuales.
- **Plush y receptores**: si `DueDateValue`/`DueDateLabel` se declararan con receptor puntero, la lista fallaría al renderizar; deben ser por valor.
- **Recuperación**: revertir el commit. Si la migración ya se aplicó en algún entorno, ejecutar antes `soda migrate down -e <entorno>` (pierde las fechas límite guardadas; el resto de datos queda intacto).
