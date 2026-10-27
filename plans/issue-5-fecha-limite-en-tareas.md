# Feature: fecha límite opcional (solo día) en las tareas

## Encargo y criterios de aceptación

Encargo: issue #5 (https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5), «Quiero añadir fecha límite a las tasks». Cuerpo: «Vale con que sea simplemente un día. Sin hora».

Requisitos explícitos:

- Una tarea puede tener una fecha límite.
- La fecha límite es un día (año, mes y día). No tiene hora.

Inferencias (decididas en este plan):

- La fecha límite es **opcional**. Las tareas existentes no tienen fecha y crear una tarea solo con título debe seguir funcionando.
- Se puede indicar al crear la tarea y al editarla. Al editar se puede cambiar o quitar.
- Se acepta cualquier día válido, también uno pasado. No hay reglas de «no anterior a hoy», porque dependerían de la zona horaria del usuario.
- La lista muestra la fecha límite de las tareas que la tienen, en formato español `DD/MM/AAAA`.

Criterios de aceptación:

1. El formulario «Añadir tarea» de `/` tiene un campo de fecha (`<input type="date" name="due_date">`) con etiqueta accesible «Fecha límite (opcional)». Su orden de tabulación es: título, fecha, «Añadir».
2. Al crear una tarea con título y fecha `2026-11-05`, se guarda con fecha límite 5 de noviembre de 2026. La respuesta es `303` a `/`.
3. Al crear una tarea sin fecha (campo vacío o ausente), se guarda sin fecha límite (`NULL`), como hasta ahora.
4. En la lista, una tarea con fecha límite muestra `Fecha límite: 05/11/2026` debajo del título, dentro de un `<time datetime="2026-11-05">`. Una tarea sin fecha no muestra nada adicional.
5. La página de edición muestra el campo «Fecha límite (opcional)» con el valor actual (`value="2026-11-05"`) o vacío.
6. Al guardar la edición con otra fecha, la fecha cambia. Si se vacía el campo, la fecha límite se elimina. El estado de completada, el `id` y `created_at` no cambian.
7. Un valor de fecha que no tiene el formato `AAAA-MM-DD` o que no existe (por ejemplo, `2026-02-30`) no se guarda. La respuesta es `422` con el mensaje «La fecha límite no es válida.» y el formulario conserva el título escrito. En edición, la tarea guardada no cambia.
8. Completar o reabrir una tarea conserva su fecha límite.
9. Los mensajes de error del título siguen funcionando como ahora (`422`, «El título es obligatorio.», `aria-describedby="title-error"` en edición).

## Comportamiento actual y propuesta

Comportamiento actual (rama creada desde `origin/main`, commit `868fb30`):

- La tabla `tasks` tiene `id`, `title`, `completed`, `created_at` y `updated_at` (`migrations/20260901000000_create_tasks.up.fizz`).
- `models.Task` (`models/task.go`) solo valida el título.
- `TasksCreate` (`actions/tasks.go`) enlaza el formulario directamente sobre `models.Task` con `c.Bind(task)`. `TasksUpdate` enlaza un struct anónimo con solo `title` para no tocar otros campos.
- `templates/tasks/index.plush.html` muestra la lista y el formulario de alta. `templates/tasks/edit.plush.html` muestra la edición del título. Ambos muestran el bloque de error `#title-error` cuando `errors.HasAny()`.
- No existe ningún concepto de fecha límite.

Propuesta (cambio vertical mínimo):

- Migración nueva que añade la columna nullable `due_date` de tipo `date`.
- Campo `DueDate *time.Time` en `models.Task`. Un método `SetDueDate(string)` interpreta el valor del campo de fecha. `Validate` informa si ese valor no era válido. Dos métodos de formato (`DueDateValue`, `DueDateLabel`) para las plantillas.
- `TasksCreate` y `TasksUpdate` enlazan un struct de entrada común (`taskInput`) con `title` y `due_date`, y llaman a `task.SetDueDate`.
- Plantillas: campo de fecha en el alta y en la edición, fecha visible en la lista y errores por campo.
- CSS: estilos del campo de fecha y de la línea de fecha en la lista. Se sube el parámetro de caché de `app.css`.

## Alcance

Incluido:

- Persistencia: migración `add_column` / `drop_column` de `tasks.due_date` (`date`, `NULL`).
- Modelo: campo, asignación desde texto, validación del formato y formato de salida.
- Acciones: creación y edición con fecha límite. El toggle no cambia (ya conserva todas las columnas).
- Plantillas: alta, edición y lista.
- CSS de los nuevos elementos y cambio de `?v=3` a `?v=4` en el enlace de `app.css`.
- Tests de modelo y de acciones.

Excluido:

- Hora o zona horaria en la fecha límite.
- Resaltar tareas vencidas o próximas a vencer, ordenar o filtrar por fecha, recordatorios o notificaciones.
- Cambiar el orden de la lista (sigue `created_at asc, id asc`).
- Cambios en `public/assets/app.js`, en las rutas (`actions/app.go`), en las migraciones existentes, en la migración de semillas, en `fixtures/tasks.toml`, en `go.mod`/`go.sum` o en el CI.
- Nombres de meses en texto («5 nov.»). Se usa formato numérico para no añadir tablas de traducción.

## Diseño por capas

### Persistencia y migraciones

Archivos nuevos (mismo estilo fizz que `20260901000000_create_tasks`):

- `migrations/20261027000000_add_due_date_to_tasks.up.fizz`

  ```
  add_column("tasks", "due_date", "date", {"null": true})
  ```

- `migrations/20261027000000_add_due_date_to_tasks.down.fizz`

  ```
  drop_column("tasks", "due_date")
  ```

El traductor PostgreSQL de fizz deja `date` tal cual (`DATE`). La columna es nullable y no tiene valor por defecto, así que las filas existentes y la migración de semillas siguen siendo válidas. `migrations/embed.go` incluye `*.fizz` y `cmd/app/main.go` aplica las migraciones al arrancar, así que no hace falta más cambio.

### Modelo (`models/task.go`)

Se usa `*time.Time` (`nil` = sin fecha). Así no se añade una dependencia directa a `go.mod`. pop guarda `nil` como `NULL`. pgx guarda un `time.Time` en una columna `DATE` usando el año, el mes y el día de ese valor, y lo lee como medianoche UTC. No hay desplazamiento de día.

Cambios:

```go
// DueDateLayout es el formato AAAA-MM-DD que usan los campos <input type="date">.
const DueDateLayout = "2006-01-02"

// Task representa una tarea de la lista.
type Task struct {
	ID        uuid.UUID  `json:"id" db:"id" form:"-"`
	Title     string     `json:"title" db:"title" form:"title"`
	Completed bool       `json:"completed" db:"completed" form:"-"`
	DueDate   *time.Time `json:"due_date" db:"due_date" form:"-"`
	CreatedAt time.Time  `json:"created_at" db:"created_at" form:"-"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at" form:"-"`

	// dueDateInvalid indica que el último valor recibido en SetDueDate no era una fecha válida.
	dueDateInvalid bool `db:"-"`
}
```

- `db:"-"` en `dueDateInvalid` es **obligatorio**. Sin ninguna etiqueta, pop usa el nombre del campo como columna (`columns.TagsFor`). sqlx y el codificador de formularios de los tests ignoran los campos no exportados.
- `form:"-"` en `DueDate` evita que el codificador de `httptest` envíe el campo cuando los tests existentes hacen `Post(&models.Task{...})`.

Métodos nuevos:

```go
// SetDueDate asigna la fecha límite a partir del valor de un campo de fecha (AAAA-MM-DD).
// Un valor vacío quita la fecha límite. Un valor no válido también la deja vacía
// y hace que Validate informe del error.
func (t *Task) SetDueDate(value string) {
	t.DueDate = nil
	t.dueDateInvalid = false

	value = strings.TrimSpace(value)
	if value == "" {
		return
	}

	date, err := time.Parse(DueDateLayout, value)
	if err != nil {
		t.dueDateInvalid = true
		return
	}
	t.DueDate = &date
}

// DueDateValue devuelve la fecha límite en formato AAAA-MM-DD, o "" si la tarea no tiene.
func (t Task) DueDateValue() string {
	if t.DueDate == nil {
		return ""
	}
	return t.DueDate.Format(DueDateLayout)
}

// DueDateLabel devuelve la fecha límite en formato DD/MM/AAAA, o "" si la tarea no tiene.
func (t Task) DueDateLabel() string {
	if t.DueDate == nil {
		return ""
	}
	return t.DueDate.Format("02/01/2006")
}
```

- `DueDateValue` y `DueDateLabel` usan receptor por valor. La lista itera `models.Tasks` (`[]Task`) y Plush no puede llamar métodos de puntero sobre esos valores. `Validate` y `SetDueDate` siguen con receptor puntero.
- `time.Parse` rechaza días inexistentes (`2026-02-30`) y cualquier otro formato (`05/11/2026`, `2026-11-05T10:00`).

En `Validate`, después de las comprobaciones del título:

```go
if t.dueDateInvalid {
	verrs.Add("due_date", "La fecha límite no es válida.")
}
```

### Acción (`actions/tasks.go`)

Las rutas no cambian. Se añade un struct de entrada común a nivel de paquete, con el mismo estilo de etiquetas que el struct anónimo actual de `TasksUpdate`:

```go
// taskInput recoge los campos editables del formulario de una tarea.
type taskInput struct {
	Title   string `form:"title" json:"title"`
	DueDate string `form:"due_date" json:"due_date"`
}
```

`TasksCreate`: reemplaza `task := &models.Task{}` + `c.Bind(task)` + `task.Title = strings.TrimSpace(task.Title)` por:

```go
input := taskInput{}
if err := c.Bind(&input); err != nil {
	return err
}

task := &models.Task{Title: strings.TrimSpace(input.Title)}
task.SetDueDate(input.DueDate)
```

El resto (`ValidateAndCreate`, el 422 que vuelve a pintar la lista y la redirección) no cambia.

`TasksUpdate`: reemplaza el struct anónimo por `input := taskInput{}`. Después de `task.Title = strings.TrimSpace(input.Title)`, añade `task.SetDueDate(input.DueDate)`. Actualiza el comentario de la función a «TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.». Un `due_date` vacío o ausente quita la fecha. El formulario de edición siempre envía el campo.

`TasksToggle` no cambia: `tx.Find` carga `due_date` y `tx.Update` lo vuelve a escribir igual.

### Renderizado (Plush)

`templates/tasks/index.plush.html`:

1. En cada `<li>`, envolver el título en un contenedor y añadir la fecha:

   ```html
   <div class="task-content">
     <span class="task-title"><%= t.Title %></span>
     <%= if (t.DueDateValue() != "") { %>
       <span class="task-due">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></span>
     <% } %>
   </div>
   ```

   El `<span class="task-status">` y `<div class="task-actions">` quedan como hermanos posteriores del contenedor, igual que ahora. `app.js` no cambia: sigue leyendo `.task-title` e insertando `.task-status` antes de `.task-actions`, y ambos selectores siguen siendo válidos.

2. En el formulario de alta, dentro de `.new-task-row`, entre el input de título y el botón «Añadir»:

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

3. Errores por campo. En el input de título, cambiar la condición `errors.HasAny()` por `len(errors.Get("title")) > 0`. Sustituir el bloque `#title-error` actual por dos bloques:

   ```html
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

1. En el input de título, cambiar la condición `errors.HasAny()` por `len(errors.Get("title")) > 0` (se mantiene la rama `else` con `aria-describedby="title-help"`). Hacer lo mismo en el bloque `#title-error`.
2. Cambiar el texto de `#title-help` a «Hasta 200 caracteres. El estado de la tarea no cambia.». La frase anterior («Los demás datos de la tarea no cambian.») deja de ser cierta porque el formulario también edita la fecha.
3. Después del bloque `#title-error` y antes de `.edit-task-actions`:

   ```html
   <div class="edit-task-due">
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
   </div>
   ```

`templates/application.plush.html`: cambiar `/assets/app.css?v=3` por `/assets/app.css?v=4`. No tocar la línea de `app.js`.

### CSS (`public/assets/app.css`)

1. `.task-title` deja de ser el elemento de la rejilla. Mover `align-self: center;`, `min-width: 0;` y `padding: 0.65rem 0;` a una regla nueva `.task-content`. `.task-title` queda con `display: block;`, `overflow-wrap: anywhere;` y `line-height: 1.55;`. No cambiar `.task-completed .task-title` ni la regla móvil `.task-title { font-size: 0.9rem; }`.
2. Nueva regla:

   ```css
   .task-due {
     display: block;
     margin-top: 0.15rem;
     color: var(--color-muted);
     font-size: 0.8rem;
   }
   ```

3. Cambiar el selector `input[type="text"]` por `input[type="text"],\ninput[type="date"]` (mismas declaraciones). Justo después, añadir `input[type="date"] { width: auto; }`.
4. Cambiar `.new-task-row input { flex: 1; }` por `.new-task-row input[type="text"] { flex: 1; }`. En `@media (max-width: 380px)`, cambiar `.new-task-row input { flex-basis: calc(100% - 44px); }` por `.new-task-row input[type="text"] { … }`. Así, en pantallas muy estrechas el título ocupa la primera línea y la fecha y el botón pasan a la segunda.
5. Nueva regla `.edit-task-due { margin-top: 1.5rem; }`.

## Archivos relevantes

| Ruta | Responsabilidad |
| --- | --- |
| `migrations/20261027000000_add_due_date_to_tasks.up.fizz` (nuevo) | Añade `tasks.due_date DATE NULL`. |
| `migrations/20261027000000_add_due_date_to_tasks.down.fizz` (nuevo) | Elimina la columna. |
| `migrations/embed.go` | Incluye `*.fizz` en el binario. Sin cambios. |
| `models/task.go` | Campo `DueDate`, `SetDueDate`, `DueDateValue`, `DueDateLabel` y validación. |
| `models/task_test.go` | Tests del modelo. |
| `actions/tasks.go` | `taskInput` y uso en `TasksCreate` y `TasksUpdate`. |
| `actions/tasks_test.go` | Tests de acciones y renderizado. |
| `actions/app.go` | Rutas. Sin cambios. |
| `templates/tasks/index.plush.html` | Lista con fecha, campo de fecha en el alta y errores por campo. |
| `templates/tasks/edit.plush.html` | Campo de fecha en la edición y errores por campo. |
| `templates/application.plush.html` | Parámetro de caché de `app.css`. |
| `public/assets/app.css` | Estilos de `.task-content`, `.task-due`, el input de fecha y `.edit-task-due`. |
| `public/assets/app.js` | Toggle en cliente. Sin cambios (depende de `.task-title`, `.task-status` y `.task-actions`, que se mantienen). |
| `.github/workflows/ci.yaml` | Ya ejecuta `soda migrate -e test` antes de los tests. Sin cambios. |

## Implementación paso a paso

1. Comprobar `git status --short --branch`: rama `feature/issue-5-fecha-limite-en-tareas`, y solo debe aparecer este plan como cambio.
2. Crear las dos migraciones con el contenido indicado.
3. Modificar `models/task.go`: constante `DueDateLayout`, campos `DueDate` y `dueDateInvalid`, métodos `SetDueDate`, `DueDateValue` y `DueDateLabel`, y la comprobación en `Validate`.
4. Modificar `actions/tasks.go`: tipo `taskInput`, `TasksCreate`, `TasksUpdate` y su comentario.
5. Modificar `templates/tasks/index.plush.html`, `templates/tasks/edit.plush.html` y `templates/application.plush.html` como se describe.
6. Modificar `public/assets/app.css` como se describe.
7. Añadir los tests de la sección siguiente. Añadirlos al **final** de `models/task_test.go` y `actions/tasks_test.go`, sin modificar los tests existentes.
8. Preparar la base de datos de test (ver «Tests y validación») y ejecutar todas las validaciones.
9. Revisar `git diff --check`, `git diff` y `git status --short`. Si existe `migrations/schema.sql` (lo genera `soda migrate` cuando hay `pg_dump` disponible), borrarlo: no forma parte del cambio.

## Tests y validación

### Tests del modelo (`models/task_test.go`)

Añadir `import "time"` solo si algún test lo necesita.

- `Test_Task_DueDate_Optional`: crear `&Task{Title: "Sin fecha"}` → sin errores. Recargar con `ms.DB.Find` y comprobar `ms.Nil(reloaded.DueDate)`.
- `Test_Task_DueDate_PersistsDay`: `task.SetDueDate("2026-11-05")`, crear y recargar. Comprobar `ms.Equal("2026-11-05", reloaded.DueDateValue())` y `ms.Equal("05/11/2026", reloaded.DueDateLabel())`.
- `Test_Task_DueDate_Invalid`: para cada valor en `{"2026-02-30", "05/11/2026", "2026-11-05T10:00", "mañana"}`, crear una tarea con título válido y `SetDueDate(valor)`. Comprobar que `ValidateAndCreate` devuelve `verrs.Get("due_date")` con «La fecha límite no es válida.», que `DueDate` es `nil` y que `ms.DB.Count(&Task{})` es `0`.
- `Test_Task_SetDueDate_BlankClears`: `SetDueDate("2026-11-05")` y luego `SetDueDate("   ")` → `DueDate` es `nil` y `ValidateAndCreate` no da errores.

### Tests de acciones (`actions/tasks_test.go`)

Añadir un helper:

```go
func (as *ActionSuite) taskWithDueDate(dueDate string) *models.Task {
	task := &models.Task{Title: "Tarea con fecha"}
	task.SetDueDate(dueDate)
	verrs, err := as.DB.ValidateAndCreate(task)
	as.NoError(err)
	as.False(verrs.HasAny())
	as.NoError(as.DB.Reload(task))
	return task
}
```

- `Test_TasksIndex_DueDateField`: `GET /` contiene `type="date"` y `name="due_date"`.
- `Test_TasksIndex_ShowsDueDate`: con `taskWithDueDate("2026-11-05")` y otra tarea sin fecha, `GET /` contiene `datetime="2026-11-05"` y `Fecha límite: <time datetime="2026-11-05">05/11/2026</time>`. Comprobar también que `Fecha límite:` aparece una sola vez (`strings.Count`).
- `Test_TasksIndex_NoDueDate`: con una sola tarea sin fecha, `GET /` no contiene `class="task-due"`.
- `Test_TasksCreate_WithDueDate`: `POST /tasks` con `url.Values{"title": {"Con fecha"}, "due_date": {"2026-11-05"}}` → `303`, `Location` `/`. `as.DB.First(task)` tiene `DueDateValue()` igual a `"2026-11-05"`.
- `Test_TasksCreate_EmptyDueDate`: `POST` con `due_date` vacío → `303`, `task.DueDate` es `nil`.
- `Test_TasksCreate_InvalidDueDate`: `POST` con `title` `"Tarea con fecha mala"` y `due_date` `"2026-02-30"` → `422`. El cuerpo contiene «La fecha límite no es válida.», `id="due-date-error"` y `value="Tarea con fecha mala"`. El recuento de tareas es `0`.
- `Test_TasksEdit_PrefillsDueDate`: `GET /tasks/{id}/edit` de `taskWithDueDate("2026-11-05")` contiene `name="due_date"` y `value="2026-11-05"`.
- `Test_TasksUpdate_ChangesDueDate`: con `taskWithDueDate("2026-11-05")` marcada como completada (`task.Completed = true`; `as.NoError(as.DB.Update(task))`), hacer `PUT` con `title` y `due_date` `"2026-12-24"` → `303`. Tras `Reload`: `DueDateValue()` es `"2026-12-24"` y `Completed` sigue `true`.
- `Test_TasksUpdate_ClearsDueDate`: `PUT` con `title` y `due_date` `""` → `303`. Tras `Reload`, `DueDate` es `nil`.
- `Test_TasksUpdate_InvalidDueDate`: `PUT` con `title` `"Nuevo título"` y `due_date` `"2026-13-01"` → `422`. El cuerpo contiene «La fecha límite no es válida.» y `aria-describedby="due-date-error"`. Tras `Reload`, el título sigue `"Tarea con fecha"` y `DueDateValue()` sigue `"2026-11-05"`.
- `Test_TasksToggle_PreservesDueDate`: `PUT /tasks/{id}/toggle` sobre `taskWithDueDate("2026-11-05")` → `303`. Tras `Reload`, `Completed` es `true` y `DueDateValue()` sigue `"2026-11-05"`.

Los tests existentes deben pasar sin cambios. Son una regresión de los criterios 3 y 9.

### Preparar la base de datos de test

Los tests necesitan PostgreSQL (`docker compose up -d db`, servicio `db` saludable) y la base `todo_test` migrada con las migraciones de **esta** rama.

En este equipo, `todo_test` tiene aplicada una migración `20261026220000` de otro experimento, que ya añadió una columna `due_date`. Si se aplica encima la migración nueva, fallará con «column "due_date" already exists». Hay que recrear **solo** la base de test (los tests ya la vacían en cada caso) desde la raíz del repositorio:

```sh
SODA="$(go env GOPATH)/bin/soda"   # o `soda` si está en el PATH; en CI se instala soda v6.4.1
"$SODA" drop -e test
"$SODA" create -e test
"$SODA" migrate -e test
"$SODA" migrate status -e test     # debe listar 20260901000000, 20260901000001 y 20261027000000 como aplicadas
```

No usar `soda reset` (carga `schema.sql`, no las migraciones). No tocar la base `todo_development`. Si `soda` no está disponible, instalarlo con `go install github.com/gobuffalo/pop/v6/soda@v6.4.1`, que es la misma versión del CI.

### Comandos de validación

Con `GO_ENV=test`, desde la raíz:

```sh
gofmt -l .                 # sin salida
go vet ./...
GO_ENV=test go test -p 1 ./...
go build ./...
```

Comprobar también la migración de bajada y volver a subir, para validar el `.down.fizz`:

```sh
"$SODA" migrate down -e test && "$SODA" migrate -e test
```

## Riesgos y recuperación

- **Estado local de las bases de datos.** `todo_test` y `todo_development` tienen columnas de otras ramas (`due_date` de la migración `20261026220000`, `priority` en desarrollo). Las migraciones de esta rama solo funcionan sobre una base limpia. Recuperación: recrear `todo_test` como se indica. Este plan no exige validar manualmente contra `todo_development`. Si alguien arranca la app en local y falla la migración por columnas existentes, debe recrear esa base de forma deliberada; no forma parte de esta tarea.
- **`migrations/schema.sql` accidental.** `soda migrate` intenta volcar el esquema. Sin `pg_dump`, muestra un aviso y borra el archivo. Con `pg_dump`, el archivo se queda. No debe incluirse en el commit.
- **Desfase de día por zona horaria.** Se evita usando `DATE` en la base, `time.Parse` (UTC) en la entrada y formateando el valor leído (medianoche UTC) sin convertir de zona. Los tests `PersistsDay` y `ChangesDueDate` lo comprueban de extremo a extremo.
- **Campo no exportado en el modelo.** Si se olvida `db:"-"` en `dueDateInvalid`, pop intentará leer y escribir una columna `dueDateInvalid` y los tests fallarán con un error SQL. La corrección es añadir la etiqueta.
- **Navegadores.** Todos los navegadores actuales tienen `<input type="date">` y envían siempre `AAAA-MM-DD` o vacío. Cualquier otro valor (navegadores antiguos o peticiones manuales) recibe un `422` controlado, no un `500`.
- **Commit local sin publicar en `main`.** `429e9d6` (arreglo de `app.js`) toca la línea de `app.js` en `application.plush.html` y añade tests tras `Test_TasksToggle_InvalidID`. Este plan toca la línea de `app.css` y añade tests al final del archivo, así que no se esperan conflictos. Si aparecen al integrar, conservar ambos cambios.
- **Reversión.** Revertir el commit y ejecutar `soda migrate down -e <entorno>` donde ya se haya aplicado la migración. La columna es nullable y nada más depende de ella.
