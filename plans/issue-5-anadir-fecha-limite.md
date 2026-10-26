# Feature: fecha límite opcional (solo día) en las tareas

Origen: [issue #5](https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5), «Quiero añadir fecha límite a las tasks». Cuerpo: «Vale con que sea simplemente un día. Sin hora».

## Encargo y criterios de aceptación

Requisitos explícitos:

- Una tarea puede tener una fecha límite.
- La fecha límite es un día, sin hora.

Inferencias (decididas en este plan):

- La fecha límite es **opcional**. Las tareas existentes y las nuevas sin fecha siguen siendo válidas.
- Se puede indicar al crear la tarea y también fijar, cambiar o quitar desde la pantalla de edición. Sin la edición, las tareas que ya existen no podrían tener fecha límite.
- Se muestra en la lista de tareas.
- No se valida que la fecha sea futura: una fecha pasada indica una tarea vencida y es un dato legítimo.

Criterios de aceptación:

1. El formulario de creación de `/` incluye un campo de fecha (`<input type="date" name="due_date">`) opcional, con etiqueta accesible «Fecha límite (opcional)».
2. Si se crea una tarea con `due_date=2026-11-15`, se guarda con esa fecha. Si `due_date` está vacío o no se envía, se guarda sin fecha límite (`NULL`).
3. En la lista, una tarea con fecha límite muestra `Fecha límite: 15/11/2026` dentro de `<time datetime="2026-11-15">`. Una tarea sin fecha no muestra nada adicional.
4. La pantalla de edición incluye el campo «Fecha límite» precargado con el valor actual (`value="2026-11-15"`) o vacío. Al guardar se puede fijar, cambiar o quitar (vaciando el campo). El estado de completada, el `id` y `created_at` no cambian.
5. Un valor de `due_date` que no sea un día válido con formato `aaaa-mm-dd` (por ejemplo `15/11/2026`, `2026-02-30` o `2026-11-15T10:30`) no se guarda. Se responde 422 y se muestra «La fecha límite no es válida.» junto al campo, con `aria-invalid="true"`.
6. Completar o reabrir una tarea conserva su fecha límite.
7. Si el título no es válido y la fecha sí, el 422 conserva la fecha introducida en el campo.

## Comportamiento actual y propuesta

**Actual.** La tabla `tasks` (`migrations/20260901000000_create_tasks.up.fizz`) tiene `id`, `title`, `completed`, `created_at` y `updated_at`. `models.Task` refleja esas columnas. `TasksCreate` enlaza el formulario sobre `models.Task` (`c.Bind(task)`), y `TasksUpdate` enlaza solo `title` en un struct anónimo para no tocar otros datos. Las plantillas muestran el título y el estado. No hay ninguna fecha visible ni editable.

**Propuesta.** Cambio vertical mínimo, sin dependencias nuevas:

- Nueva columna `due_date DATE NULL`.
- Campo `DueDate *time.Time` en `models.Task` (puntero: `nil` ⇔ `NULL`). Se usa un puntero en vez de `nulls.Time` para no añadir `github.com/gobuffalo/nulls` como dependencia directa.
- Parseo explícito del formulario con el layout `2006-01-02`. No se usa el binder de Buffalo, porque acepta también formatos con hora (`2006-01-02T15:04`, RFC3339…) y ante un formato inválido hace que `c.Bind` devuelva error (500).
- Métodos de presentación en el modelo, usados por las plantillas Plush.
- Campos de fecha en los formularios de creación y edición, y la fecha en la lista.

## Alcance

Incluido:

- Migración Fizz up/down para la columna.
- Modelo: campo, constantes, parseo y métodos de presentación.
- Acciones `TasksCreate` y `TasksUpdate`: leer, validar y guardar `due_date`.
- Plantillas `tasks/index.plush.html` y `tasks/edit.plush.html`.
- CSS del campo de fecha y de la fecha en la lista. Cambio del parámetro de caché de `app.css`.
- Tests de modelo y de acciones.

Excluido:

- Hora o zona horaria en la fecha límite.
- Resaltar tareas vencidas o próximas a vencer.
- Ordenar o filtrar por fecha límite (la lista sigue ordenada por `created_at asc, id asc`).
- Recordatorios y notificaciones.
- Cambios en `public/assets/app.js`: el toggle sigue igual. Solo lee `.task-title` y `.task-actions`, que se conservan.
- Cambios en `fixtures/tasks.toml` y en la migración de semillas (la nueva columna es nullable).

## Diseño por capas

### Persistencia

- `migrations/20261026000000_add_due_date_to_tasks.up.fizz`:
  ```
  add_column("tasks", "due_date", "date", {"null": true})
  ```
  Fizz traduce `"date"` a `DATE` en Postgres (`translators/postgres.go`, rama `default`). Con `{"null": true}` la columna admite `NULL`.
- `migrations/20261026000000_add_due_date_to_tasks.down.fizz`:
  ```
  drop_column("tasks", "due_date")
  ```
- `migrations/embed.go` ya incrusta `*.fizz`. `cmd/app/main.go` ejecuta `models.Migrate()` al arrancar, así que en producción la migración se aplica sola.

### Modelo (`models/task.go`)

Añadir, junto a `TitleMaxLength`:

```go
// DueDateLayout es el formato de día, sin hora, del campo de formulario de la fecha límite.
const DueDateLayout = "2006-01-02"

// DueDateDisplayLayout es el formato con el que se muestra la fecha límite.
const DueDateDisplayLayout = "02/01/2006"

// DueDateInvalidMessage es el error de validación de una fecha límite mal formada.
const DueDateInvalidMessage = "La fecha límite no es válida."
```

Campo nuevo en `Task`, entre `Completed` y `CreatedAt`:

```go
DueDate   *time.Time `json:"due_date" db:"due_date" form:"-"`
```

`form:"-"` impide que `c.Bind(task)` intente decodificarlo. La fecha la leen las acciones de forma explícita.

Funciones y métodos nuevos (los de presentación con receptor por valor, para que Plush pueda invocarlos sobre los elementos `Task` del bucle de `models.Tasks` y sobre `*Task`):

```go
// ParseDueDate interpreta la fecha límite recibida del formulario.
// Un valor vacío significa que la tarea no tiene fecha límite.
func ParseDueDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	day, err := time.Parse(DueDateLayout, value)
	if err != nil {
		return nil, err
	}
	return &day, nil
}

// HasDueDate indica si la tarea tiene fecha límite.
func (t Task) HasDueDate() bool {
	return t.DueDate != nil
}

// DueDateValue devuelve la fecha límite en el formato del campo de formulario, o "" si no tiene.
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
	return t.DueDate.Format(DueDateDisplayLayout)
}
```

`Validate` no cambia: un `*time.Time` ya parseado siempre es válido. El formato se valida en la frontera del formulario (acciones).

### Acciones (`actions/tasks.go`)

Rutas: sin cambios (`POST /tasks`, `PUT /tasks/{task_id}`).

`TasksCreate`: después de `task.Title = strings.TrimSpace(task.Title)`, sustituir el bloque `verrs, err := tx.ValidateAndCreate(task)` / `if err != nil { return err }` por:

```go
dueDate, err := models.ParseDueDate(c.Param("due_date"))
verrs := validate.NewErrors()
if err != nil {
	verrs.Add("due_date", models.DueDateInvalidMessage)
} else {
	task.DueDate = dueDate
	if verrs, err = tx.ValidateAndCreate(task); err != nil {
		return err
	}
}
```

El bloque `if verrs.HasAny() { … 422 … }` que sigue no cambia. `c.Param` incluye los valores del cuerpo del formulario en POST y PUT (`buffalo/context.go`, `newContext`).

`TasksUpdate`:

- Actualizar el comentario a «actualiza el título y la fecha límite, conservando los demás datos de la tarea.».
- Ampliar el struct de entrada:
  ```go
  input := struct {
  	Title   string `form:"title" json:"title"`
  	DueDate string `form:"due_date" json:"due_date"`
  }{}
  ```
- Tras `task.Title = strings.TrimSpace(input.Title)`, sustituir `verrs, err := tx.ValidateAndUpdate(task)` / `if err != nil { return err }` por:
  ```go
  dueDate, err := models.ParseDueDate(input.DueDate)
  verrs := validate.NewErrors()
  if err != nil {
  	verrs.Add("due_date", models.DueDateInvalidMessage)
  } else {
  	task.DueDate = dueDate
  	if verrs, err = tx.ValidateAndUpdate(task); err != nil {
  		return err
  	}
  }
  ```
  El bloque `if verrs.HasAny() { … 422 edit … }` no cambia. Si la fecha no es válida, `task.DueDate` conserva el valor guardado y la plantilla lo muestra.

Decisión: si la fecha es inválida, solo se informa del error de fecha, sin validar también el título. Un navegador con `type="date"` solo envía `aaaa-mm-dd` o vacío, así que este caso solo ocurre con peticiones manipuladas. Así no hace falta validar dos veces.

`TasksToggle` no cambia: `tx.Find` + `tx.Update` reescriben todas las columnas con los valores cargados, de modo que la fecha se conserva.

### Renderizado

`templates/tasks/index.plush.html`:

1. En cada `<li class="task…">`, sustituir `<span class="task-title"><%= t.Title %></span>` por:
   ```html
   <div class="task-main">
     <span class="task-title"><%= t.Title %></span>
     <%= if (t.HasDueDate()) { %>
       <span class="task-due">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></span>
     <% } %>
   </div>
   ```
   La fecha **no** va dentro de `.task-title`, porque `app.js` usa su `textContent` para el `aria-label` del toggle. `.task-actions` sigue siendo hijo directo del `<li>`, como necesita `app.js`.
2. En el formulario de creación:
   - En el `<input id="task-title">`, cambiar la condición `errors.HasAny()` por `len(errors.Get("title")) > 0`.
   - Entre el `<input id="task-title" …>` y el botón «Añadir», insertar:
     ```html
     <label for="task-due-date" class="sr-only">Fecha límite (opcional)</label>
     <input
       id="task-due-date"
       type="date"
       name="due_date"
       value="<%= task.DueDateValue() %>"
       title="Fecha límite (opcional)"
       <%= if (len(errors.Get("due_date")) > 0) { %>autofocus aria-invalid="true" aria-describedby="due-date-error"<% } %>
     >
     ```
   - Cambiar la condición del `<p id="title-error">` a `len(errors.Get("title")) > 0` y añadir después:
     ```html
     <%= if (len(errors.Get("due_date")) > 0) { %>
       <p id="due-date-error" class="field-error" role="alert">
         <%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %>
       </p>
     <% } %>
     ```

`templates/tasks/edit.plush.html`:

1. En el `<input id="task-title">` y en el `<p id="title-error">`, cambiar `errors.HasAny()` por `len(errors.Get("title")) > 0`. Así un error solo de fecha no marca el título como inválido.
2. Cambiar el texto de `#title-help` a `Hasta 200 caracteres.`, porque la frase «Los demás datos de la tarea no cambian» deja de ser cierta con un segundo campo editable.
3. Antes de `<div class="edit-task-actions">`, insertar:
   ```html
   <label for="task-due-date">Fecha límite</label>
   <input
     id="task-due-date"
     type="date"
     name="due_date"
     value="<%= task.DueDateValue() %>"
     <%= if (len(errors.Get("due_date")) > 0) { %>aria-invalid="true" aria-describedby="due-date-error"<% } else { %>aria-describedby="due-date-help"<% } %>
   >
   <p id="due-date-help" class="field-hint">Opcional. Déjala vacía para quitar la fecha límite.</p>
   <%= if (len(errors.Get("due_date")) > 0) { %>
     <p id="due-date-error" class="field-error" role="alert">
       <%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %>
     </p>
   <% } %>
   ```

`templates/application.plush.html`: cambiar `/assets/app.css?v=3` por `/assets/app.css?v=4` para invalidar la caché del CSS.

### CSS (`public/assets/app.css`)

- Sustituir la regla `.task-title { … }` (línea ~132) por:
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
    margin-top: 0.15rem;
    color: var(--color-muted);
    font-size: 0.8rem;
  }
  ```
  Así `.task-main` ocupa la columna 2 del grid de `.task` con la misma alineación que antes tenía el título. Sin fecha, el aspecto es idéntico. `.task-completed .task-title` y la regla móvil `.task-title { font-size: 0.9rem; }` se mantienen.
- Cambiar el selector `input[type="text"] {` por `input[type="text"],\ninput[type="date"] {`, para que el campo de fecha comparta borde, altura y tipografía.
- Después de `.new-task-row input { flex: 1; }`, añadir:
  ```css
  .new-task-row input[type="date"] {
    flex: 0 0 auto;
    width: auto;
  }
  ```
  La especificidad de esta regla es mayor que la de `.new-task-row input`, así que también prevalece sobre `flex-basis` en `@media (max-width: 380px)`. En pantallas estrechas, la fila se parte: título en la primera línea; fecha y botón en la segunda.
- Después de `.edit-task label { … }`, añadir:
  ```css
  .edit-task label:not(:first-of-type) {
    margin-top: 1.5rem;
  }
  ```

## Archivos relevantes

| Ruta | Responsabilidad |
| --- | --- |
| `migrations/20261026000000_add_due_date_to_tasks.up.fizz` (nuevo) | Añade `due_date DATE NULL` |
| `migrations/20261026000000_add_due_date_to_tasks.down.fizz` (nuevo) | Elimina la columna |
| `models/task.go` | Campo `DueDate`, constantes, `ParseDueDate`, `HasDueDate`, `DueDateValue`, `DueDateLabel` |
| `models/task_test.go` | Tests de parseo y persistencia |
| `actions/tasks.go` | Lectura y validación de `due_date` en `TasksCreate` y `TasksUpdate` |
| `actions/tasks_test.go` | Tests de creación, edición, lista y toggle con fecha |
| `templates/tasks/index.plush.html` | Campo en el alta y fecha en la lista |
| `templates/tasks/edit.plush.html` | Campo en la edición; errores por campo |
| `templates/application.plush.html` | `app.css?v=4` |
| `public/assets/app.css` | Estilos de `.task-main`, `.task-due`, campo de fecha |
| `actions/app.go` | Rutas (sin cambios; referencia) |
| `public/assets/app.js` | Toggle (sin cambios; depende de `.task-title` y `.task-actions`) |

## Implementación paso a paso

1. Crear las dos migraciones Fizz de la sección «Persistencia».
2. Modificar `models/task.go` como en «Modelo».
3. Modificar `actions/tasks.go` como en «Acciones».
4. Modificar las plantillas y `application.plush.html` como en «Renderizado».
5. Modificar `public/assets/app.css` como en «CSS».
6. Añadir los tests de «Tests y validación».
7. Preparar la base de test y ejecutar las validaciones.

## Tests y validación

### Tests nuevos

`models/task_test.go` (métodos de `ModelSuite`; importar `time` si hace falta):

- `Test_ParseDueDate`: tabla con `"2026-11-15"` → fecha válida y `DueDateValue()` = `"2026-11-15"`; `""` y `"   "` → `nil, nil`; `"15/11/2026"`, `"2026-02-30"`, `"2026-11-15T10:30"` → error.
- `Test_Task_DueDate_OptionalByDefault`: crear `&Task{Title: "Sin fecha"}`, recargar con `Find`, comprobar que `DueDate` es `nil`, que `HasDueDate()` es `false` y que `DueDateValue()` es `""`.
- `Test_Task_DueDate_Persists`: crear con `ParseDueDate("2026-11-15")`, recargar con `Find` y comprobar `DueDateValue() == "2026-11-15"` y `DueDateLabel() == "15/11/2026"`. Esto demuestra que el día no se desplaza por la zona horaria.

`actions/tasks_test.go` (enviar `url.Values` explícitos; añadir un helper `taskWithDueDate(day string) *models.Task` que cree la tarea con `models.ParseDueDate` y `ValidateAndCreate`):

- `Test_TasksCreate_WithDueDate`: `POST /tasks` con `title` y `due_date=2026-11-15` → 303. La tarea guardada tiene `DueDateValue() == "2026-11-15"`.
- `Test_TasksCreate_EmptyDueDate`: `due_date=""` → 303 y `DueDate` es `nil`.
- `Test_TasksCreate_InvalidDueDate`: `due_date=15/11/2026` → 422. El cuerpo contiene `models.DueDateInvalidMessage`, `id="due-date-error"` y `value="<título enviado>"`, y no se ha creado ninguna tarea.
- `Test_TasksCreate_EmptyTitle_KeepsDueDate`: `title=""`, `due_date=2026-11-15` → 422. El cuerpo contiene «El título es obligatorio.» y `value="2026-11-15"`.
- `Test_TasksIndex_ShowsDueDate`: con una tarea con fecha, `GET /` contiene `<time datetime="2026-11-15">15/11/2026</time>`.
- `Test_TasksIndex_WithoutDueDate`: con una tarea sin fecha, `GET /` no contiene `class="task-due"`.
- `Test_TasksEdit_PrefillsDueDate`: `GET /tasks/{id}/edit` contiene `name="due_date"` y `value="2026-11-15"`.
- `Test_TasksUpdate_SetsDueDate`: tarea sin fecha → `PUT` con `title` y `due_date=2026-12-01` → 303, y la fecha queda guardada.
- `Test_TasksUpdate_ClearsDueDate`: tarea con fecha → `PUT` con `title` y `due_date=""` → 303, y `DueDate` es `nil`.
- `Test_TasksUpdate_InvalidDueDate`: tarea con fecha → `PUT` con `title=Otro` y `due_date=2026-13-01` → 422. El cuerpo contiene el mensaje. Tras recargar, el título y la fecha originales no han cambiado.
- `Test_TasksToggle_KeepsDueDate`: tarea con fecha → `PUT /tasks/{id}/toggle` → 303. Tras recargar, está completada y la fecha no ha cambiado.

Los tests existentes deben seguir en verde sin cambios (incluido `Test_TasksUpdate_EmptyTitle`, que comprueba `aria-describedby="title-error"`).

### Preparación del entorno

Los tests necesitan PostgreSQL (`database.yml`, entorno `test`) y la base `todo_test` migrada.

**Atención:** la base `todo_test` local contiene restos de otra rama (`schema_migration` incluye `20261026200000` y la tabla ya tiene `due_date`). Hay que recrearla antes de migrar o la nueva migración fallará con «column already exists». Es una base de test y no contiene datos que preservar.

```sh
docker compose up -d db
docker compose exec db dropdb -U postgres --if-exists todo_test
docker compose exec db createdb -U postgres todo_test
GO_ENV=test go run github.com/gobuffalo/pop/v6/soda@v6.4.1 migrate -e test
```

### Comprobaciones

1. Reversibilidad de la migración:
   `GO_ENV=test go run github.com/gobuffalo/pop/v6/soda@v6.4.1 migrate down -e test` y después `GO_ENV=test go run github.com/gobuffalo/pop/v6/soda@v6.4.1 migrate -e test`. Ambos sin error.
2. `gofmt -l .` → sin salida.
3. `go vet ./...` → sin errores.
4. `GO_ENV=test go test -p 1 ./...` → todo en verde.
5. `go build ./...` → sin errores.

Validación manual (opcional, no bloqueante): arrancar la app contra una base de desarrollo limpia y comprobar en `/` que el campo de fecha aparece junto al título, que una tarea creada con fecha muestra «Fecha límite: dd/mm/aaaa» y que la edición permite cambiarla y quitarla. La base `todo_development` local también tiene migraciones de otras ramas (`20261014000000`), así que puede requerir recrearla.

## Riesgos y recuperación

- **Base de test contaminada por otras ramas**: ver «Preparación del entorno». Si `migrate` falla con «column already exists», recrear `todo_test`.
- **Desplazamiento del día por zona horaria**: `time.Parse` devuelve medianoche UTC, y pgx codifica `time.Time` en una columna `DATE` usando año, mes y día. Las bases locales y de CI usan `TimeZone=UTC`. `Test_Task_DueDate_Persists` y los tests de acción verifican el ida y vuelta. Si fallaran con un día menos, la causa sería la codificación como `timestamptz`. En ese caso, detener la implementación e informar; no se debe parchear sumando horas.
- **Interfaz en pantallas estrechas**: el campo de fecha ocupa espacio en la fila de alta. Las reglas CSS hacen que la fila se parta por debajo de 380 px. Revisión visual opcional.
- **Navegadores sin `type="date"`**: muestran un campo de texto. El usuario tendría que escribir `aaaa-mm-dd`, y cualquier otro formato da un 422 con mensaje claro.
- **Errores combinados**: si fecha y título son inválidos a la vez, solo se informa de la fecha. Decisión deliberada (ver «Acciones»).
- **Otras ramas de la misma issue**: el repositorio tiene otras ramas y PRs abiertos para la issue #5 con migraciones propias. Fusionar más de uno añadiría la columna dos veces y rompería el arranque. Hay que fusionar solo uno.
- **Recuperación**: `git revert` del commit. La columna es nullable y el código anterior no la referencia (pop solo usa las columnas del struct), así que revertir el código sin deshacer la migración es seguro. Para eliminar la columna: `soda migrate down -e <entorno>` antes del revert.
