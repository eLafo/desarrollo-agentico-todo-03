# Feature: fecha límite (solo día) en las tareas

Origen: issue #5 — https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5

## Encargo y criterios de aceptación

Encargo literal: «Quiero añadir fecha límite a las tasks». Cuerpo: «Vale con que sea simplemente un día. Sin hora».

Requisitos explícitos:

- Una tarea puede tener una fecha límite.
- La fecha límite es un día del calendario, sin hora.

Inferencias (decisiones de este plan, no pedidas literalmente):

- La fecha límite es **opcional**: las tareas existentes y las nuevas pueden no tenerla.
- Se fija al crear la tarea y se puede cambiar o quitar desde la pantalla de edición.
- Se muestra en la lista junto al título.
- Se acepta cualquier día válido, incluido uno pasado (si no, editar una tarea vencida obligaría a cambiar su fecha).

Criterios de aceptación:

1. El formulario «Añadir tarea» de `/` tiene un campo de fecha opcional (`<input type="date" name="due_date">`). Al crear una tarea con título y fecha, la tarea se guarda con ese día; sin fecha, se guarda sin fecha límite.
2. En la lista, cada tarea con fecha límite muestra bajo el título `Fecha límite: <time datetime="AAAA-MM-DD">D de <mes> de AAAA</time>` (p. ej. «3 de noviembre de 2026»). Las tareas sin fecha no muestran nada adicional.
3. La pantalla `/tasks/{id}/edit` tiene el campo «Fecha límite (opcional)» precargado con la fecha actual de la tarea (o vacío). Guardar con otra fecha la cambia; guardar con el campo vacío la elimina.
4. El servidor solo acepta el formato `AAAA-MM-DD` de un día real. Cualquier otro valor (`2026-02-30`, `03/11/2026`, `2026-11-03T10:00`) responde 422 con el mensaje «La fecha límite no es válida.» y no crea ni modifica la tarea.
5. Si la creación falla por el título, el formulario vuelve a mostrar la fecha válida introducida.
6. Completar o reabrir una tarea conserva su fecha límite.
7. El comportamiento existente no cambia: validaciones del título, orden de la lista, protección de `id`, `completed` y `created_at` frente al formulario, 404 en ids inexistentes o inválidos.

## Comportamiento actual y propuesta

Actual (rama basada en `origin/main`):

- `migrations/20260901000000_create_tasks.up.fizz` crea `tasks` con `id`, `title`, `completed` y timestamps. No hay ninguna columna de fecha de usuario.
- `models.Task` (`models/task.go`) solo valida el título.
- `TasksCreate` (`actions/tasks.go`) hace `c.Bind(task)` directamente sobre `models.Task`; los campos protegidos usan `form:"-"`.
- `TasksUpdate` hace `c.Bind` sobre un struct anónimo con solo `Title`, para que el formulario no pueda tocar otros campos.
- `templates/tasks/index.plush.html` y `edit.plush.html` solo tienen el campo título. El bloque de error se muestra si `errors.HasAny()` y siempre muestra `errors.Get("title")`.
- `cmd/app/main.go` ejecuta `models.Migrate()` al arrancar, así que la migración nueva se aplica sola en desarrollo, Docker y producción.

Propuesta (cambio vertical mínimo):

- Persistencia: columna nueva `due_date` de tipo `date` que admite NULL.
- Modelo: campo `DueDate nulls.Time` con `form:"-"`; función `ParseDueDate` y métodos de presentación `DueDateValue` y `DueDateLabel`.
- Acciones: un struct `taskInput {Title, DueDate string}`, compartido por crear y editar, sustituye el binding directo y el struct anónimo. La fecha se interpreta explícitamente; si no es válida, la acción devuelve 422 sin tocar la base de datos.
- Vistas: campo de fecha en los dos formularios, errores por campo y fecha visible en la lista.
- CSS: estilos para el input de fecha y para la línea de fecha bajo el título; se sube la versión de caché de `app.css`.

## Alcance

Incluido:

- Migración nueva (up/down) para `tasks.due_date`.
- Campo, parseo y formateo de la fecha en `models/task.go`.
- Crear y editar con fecha en `actions/tasks.go`.
- Plantillas `index` y `edit`, `public/assets/app.css` y el número de versión de la hoja de estilos en `templates/application.plush.html`.
- `go.mod`: `github.com/gobuffalo/nulls` pasa de indirecta a directa (ya está en v0.4.2; no se añaden módulos).
- Tests de modelo y de acciones.

Excluido:

- Hora o zona horaria en la fecha límite.
- Marcar tareas vencidas o próximas a vencer, recordatorios o notificaciones.
- Ordenar o filtrar por fecha límite (la lista sigue ordenada por `created_at asc, id asc`).
- Cambios en `public/assets/app.js`, en el seed (`20260901000001_seed_tasks.up.sql`) y en `fixtures/tasks.toml`.
- Restricciones de rango (mínimo o máximo) en la fecha.
- Corregir el toggle por JavaScript de `origin/main` (`fetch(form.action)` hace un GET). Ese arreglo vive en otro commit local de `main` sin publicar y no forma parte de este encargo.

## Diseño por capas

### Persistencia y migración

Archivos nuevos, siguiendo el formato fizz de la migración existente:

`migrations/20261026180000_add_due_date_to_tasks.up.fizz`

```
add_column("tasks", "due_date", "date", {"null": true})
```

`migrations/20261026180000_add_due_date_to_tasks.down.fizz`

```
drop_column("tasks", "due_date")
```

Notas:

- El traductor PostgreSQL de fizz deja `date` tal cual (`DATE`). Sin `null: true`, fizz crea la columna `NOT NULL`.
- `migrations/embed.go` ya incluye `*.fizz`; no hay que tocarlo.
- Las filas existentes quedan con `due_date = NULL`, es decir, sin fecha límite.

### Modelo (`models/task.go`)

- Importar `github.com/gobuffalo/nulls`.
- Añadir el campo tras `Completed`:

  ```go
  DueDate   nulls.Time `json:"due_date" db:"due_date" form:"-"`
  ```

  Con `form:"-"`, `c.Bind` nunca lo rellena y `httptest` no lo codifica al enviar un `models.Task` en los tests existentes.
- Constantes y funciones nuevas:

  ```go
  // DueDateLayout es el formato de la fecha límite: solo el día, sin hora.
  const DueDateLayout = "2006-01-02"

  // DueDateInvalidMessage es el error que se muestra cuando la fecha límite no es un día válido.
  const DueDateInvalidMessage = "La fecha límite no es válida."

  var monthNames = [...]string{"enero", "febrero", "marzo", "abril", "mayo", "junio",
      "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}

  // ParseDueDate interpreta una fecha límite en formato AAAA-MM-DD.
  // Una cadena vacía significa que la tarea no tiene fecha límite.
  func ParseDueDate(value string) (nulls.Time, error) {
      value = strings.TrimSpace(value)
      if value == "" {
          return nulls.Time{}, nil
      }
      day, err := time.Parse(DueDateLayout, value)
      if err != nil {
          return nulls.Time{}, err
      }
      return nulls.NewTime(day), nil
  }

  // DueDateValue devuelve la fecha límite en formato AAAA-MM-DD, o "" si no tiene.
  func (t Task) DueDateValue() string { ... t.DueDate.Time.Format(DueDateLayout) ... }

  // DueDateLabel devuelve la fecha límite en español, p. ej. "3 de noviembre de 2026", o "" si no tiene.
  func (t Task) DueDateLabel() string { ... fmt.Sprintf("%d de %s de %d", d.Day(), monthNames[d.Month()-1], d.Year()) ... }
  ```

- `DueDateValue` y `DueDateLabel` **deben tener receptor por valor**: en `index.plush.html`, `t` es un `models.Task` no direccionable (elemento de `models.Tasks`), y Plush no encontraría un método con receptor puntero.
- `time.Parse` con ese layout exige año de 4 dígitos y un día que exista, y rechaza cualquier hora. Así se cumple «sin hora» también en el servidor.
- `Validate` no cambia: la fecha ya llega interpretada y cualquier valor válido es aceptable.

### Acciones (`actions/tasks.go`)

Añadir junto a las acciones:

```go
// taskInput recoge los campos de una tarea que el usuario puede enviar desde los formularios.
type taskInput struct {
	Title   string `form:"title" json:"title"`
	DueDate string `form:"due_date" json:"due_date"`
}

// applyTo asigna el título y la fecha límite a la tarea. Si la fecha no es válida,
// devuelve los errores de validación de la tarea junto con el de la fecha, sin guardar nada.
func (in taskInput) applyTo(tx *pop.Connection, task *models.Task) (*validate.Errors, error) {
	task.Title = strings.TrimSpace(in.Title)

	dueDate, err := models.ParseDueDate(in.DueDate)
	if err == nil {
		task.DueDate = dueDate
		return validate.NewErrors(), nil
	}

	verrs, err := task.Validate(tx)
	if err != nil {
		return nil, err
	}
	verrs.Add("due_date", models.DueDateInvalidMessage)
	return verrs, nil
}
```

`TasksCreate`:

- Sustituir `task := &models.Task{}` + `c.Bind(task)` + `TrimSpace` por:

  ```go
  input := taskInput{}
  if err := c.Bind(&input); err != nil {
  	return err
  }

  task := &models.Task{}
  verrs, err := input.applyTo(tx, task)
  if err != nil {
  	return err
  }
  if !verrs.HasAny() {
  	if verrs, err = tx.ValidateAndCreate(task); err != nil {
  		return err
  	}
  }
  ```

- El bloque `if verrs.HasAny() { ... 422 ... }` y la redirección se mantienen igual.

`TasksUpdate`:

- Sustituir el struct anónimo por `input := taskInput{}`.
- Sustituir `task.Title = strings.TrimSpace(input.Title)` + `tx.ValidateAndUpdate(task)` por el mismo patrón `applyTo` → `ValidateAndUpdate` solo si no hay errores.
- Actualizar el comentario de la función: «TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.»
- Un campo `due_date` vacío o ausente deja la tarea sin fecha límite. El formulario de edición siempre lo envía.

`TasksToggle` no cambia: `tx.Update(task)` reescribe la fecha que acaba de leer con `Find`.

Rutas: sin cambios.

### Renderizado (Plush)

`templates/tasks/index.plush.html`:

1. En cada `<li class="task ...">`, envolver el título en un contenedor que ocupe la segunda columna del grid y añadir la fecha:

   ```html
   <div class="task-content">
     <span class="task-title"><%= t.Title %></span>
     <%= if (t.DueDate.Valid) { %>
       <span class="task-due">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></span>
     <% } %>
   </div>
   ```

   La clase `.task-title` debe seguir conteniendo **solo** el título: `app.js` usa su `textContent` para el `aria-label` del botón de completar. `.task-status` y `.task-actions` siguen siendo hijos directos del `<li>`, porque `app.js` inserta el estado antes de `.task-actions`.
2. En el formulario «Añadir tarea»:
   - El input del título pasa de `errors.HasAny()` a `len(errors.Get("title")) > 0` para `autofocus aria-invalid="true" aria-describedby="title-error"`.
   - Después del input del título y antes del botón «Añadir», añadir:

     ```html
     <label for="task-due-date" class="sr-only">Fecha límite (opcional)</label>
     <input
       id="task-due-date"
       type="date"
       name="due_date"
       value="<%= task.DueDateValue() %>"
       title="Fecha límite (opcional)"
       <%= if (len(errors.Get("due_date")) > 0) { %><%= if (len(errors.Get("title")) == 0) { %>autofocus <% } %>aria-invalid="true" aria-describedby="due-date-error"<% } %>
     >
     ```

   - Sustituir el bloque de error único por uno por campo:
     - `<p id="title-error" class="field-error" role="alert">`, solo si `len(errors.Get("title")) > 0`, con el mismo bucle de mensajes;
     - `<p id="due-date-error" class="field-error" role="alert">`, solo si `len(errors.Get("due_date")) > 0`, con el bucle sobre `errors.Get("due_date")`.

`templates/tasks/edit.plush.html`:

1. El input del título pasa de `errors.HasAny()` a `len(errors.Get("title")) > 0` en su condicional `aria-invalid`/`aria-describedby`, y el bloque `title-error` solo se muestra con errores de título. `autofocus` se mantiene.
2. Tras el bloque de error del título y antes de `.edit-task-actions`, añadir:

   ```html
   <label for="task-due-date">Fecha límite (opcional)</label>
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

El texto de ayuda del título («Hasta 200 caracteres. Los demás datos de la tarea no cambian.») no cambia.

### CSS (`public/assets/app.css`) y caché

- `.task-title`: quitar `align-self`, `min-width` y `padding` y añadir `display: block`. Se mantienen `overflow-wrap: anywhere` y `line-height: 1.55`.
- Nuevas reglas después de `.task-title`:

  ```css
  .task-content {
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

- Extender el selector `input[type="text"]` a `input[type="text"], input[type="date"]` para que el campo de fecha tenga el mismo aspecto.
- Después de `.new-task-row input { flex: 1; }`, añadir:

  ```css
  .new-task-row input[type="date"] {
    flex: 0 0 auto;
    width: auto;
  }
  ```

  Su especificidad (0,2,1) le gana a `.new-task-row input` también dentro de `@media (max-width: 380px)`. Allí la fila ya hace wrap, así que fecha y botón pasan a la segunda línea.
- Separación del segundo campo en edición: `.edit-task p + label { margin-top: 1.25rem; }`.
- `templates/application.plush.html`: cambiar `/assets/app.css?v=3` por `/assets/app.css?v=4`. `app.js?v=2` no cambia.

## Archivos relevantes

| Ruta | Responsabilidad |
| --- | --- |
| `migrations/20261026180000_add_due_date_to_tasks.up.fizz` (nuevo) | Añade `tasks.due_date DATE NULL` |
| `migrations/20261026180000_add_due_date_to_tasks.down.fizz` (nuevo) | Elimina la columna |
| `migrations/embed.go` | Incrusta `*.fizz`; sin cambios |
| `models/task.go` | Campo `DueDate`, `ParseDueDate`, `DueDateValue`, `DueDateLabel`, constantes |
| `models/task_test.go` | Tests de parseo, formato y persistencia de la fecha |
| `actions/tasks.go` | `taskInput`, `applyTo`, `TasksCreate` y `TasksUpdate` con fecha |
| `actions/tasks_test.go` | Tests HTTP de crear, editar, listar y completar con fecha |
| `actions/app.go` | Rutas; sin cambios |
| `templates/tasks/index.plush.html` | Campo de fecha al crear, errores por campo, fecha en la lista |
| `templates/tasks/edit.plush.html` | Campo de fecha precargado, ayuda y error |
| `templates/application.plush.html` | Versión de caché de `app.css` |
| `public/assets/app.css` | Estilos de `.task-content`, `.task-due` e `input[type="date"]` |
| `public/assets/app.js` | Usa `.task-title`, `.task-status` y `.task-actions`; **no se modifica**, pero el marcado debe respetar esos selectores |
| `go.mod` | `github.com/gobuffalo/nulls` pasa a dependencia directa |
| `cmd/app/main.go` | Aplica migraciones al arrancar; sin cambios |
| `.github/workflows/ci.yaml` | gofmt, vet, `soda migrate -e test`, tests y build; sin cambios |

## Implementación paso a paso

1. Crear las dos migraciones fizz con el contenido indicado.
2. En `models/task.go`, añadir el import de `nulls`, el campo `DueDate`, las constantes `DueDateLayout` y `DueDateInvalidMessage`, `monthNames`, `ParseDueDate`, `DueDateValue` y `DueDateLabel` (estos dos con receptor por valor).
3. Ejecutar `go mod tidy` y comprobar con `git diff go.mod go.sum` que el único cambio es mover `github.com/gobuffalo/nulls v0.4.2` al bloque de dependencias directas. Si cambia algo más, revertir y detenerse.
4. En `actions/tasks.go`, añadir `taskInput` y `applyTo` y adaptar `TasksCreate` y `TasksUpdate` como se describe. `TasksIndex`, `TasksEdit`, `TasksToggle` y `TasksDestroy` no cambian.
5. Actualizar `templates/tasks/index.plush.html` (contenedor `.task-content`, `.task-due`, input de fecha y errores por campo).
6. Actualizar `templates/tasks/edit.plush.html` (errores del título por campo, campo de fecha, ayuda y error).
7. Actualizar `public/assets/app.css` y subir `app.css?v=4` en `templates/application.plush.html`.
8. Añadir los tests de la sección siguiente.
9. Preparar la base de test y ejecutar las validaciones.

## Tests y validación

### Tests de modelo (`models/task_test.go`, `ModelSuite`)

- `Test_ParseDueDate_Empty`: `""` y `"   "` devuelven `Valid == false` sin error.
- `Test_ParseDueDate_Valid`: `"2026-11-03"` devuelve `Valid == true`, año 2026, mes noviembre, día 3, y hora, minuto y segundo a cero.
- `Test_ParseDueDate_Invalid`: `"2026-02-30"`, `"03/11/2026"`, `"2026-11-03T10:00"` y `"mañana"` devuelven error.
- `Test_Task_DueDate_Optional`: `ValidateAndCreate` sin fecha y luego `Find` dan `DueDate.Valid == false` y `DueDateValue() == ""`.
- `Test_Task_DueDate_Persists`: crear con `DueDate` de `ParseDueDate("2026-11-03")` y recargar con `Find` da `DueDateValue() == "2026-11-03"`.
- `Test_Task_DueDateLabel`: `"2026-11-03"` da `"3 de noviembre de 2026"`, `"2027-01-15"` da `"15 de enero de 2027"` y una tarea sin fecha da `""`.

### Tests de acciones (`actions/tasks_test.go`, `ActionSuite`)

- `Test_TasksCreate_Valid` (existente): añadir `as.False(task.DueDate.Valid)`.
- `Test_TasksCreate_WithDueDate`: POST `url.Values{"title": {"Con fecha"}, "due_date": {"2026-11-03"}}` da 303 y la tarea guardada tiene `DueDateValue() == "2026-11-03"`.
- `Test_TasksCreate_InvalidDueDate`: con `due_date` `"2026-02-30"` da 422, el cuerpo contiene `"La fecha límite no es válida."` y `aria-describedby="due-date-error"`, y no se crea ninguna tarea.
- `Test_TasksCreate_EmptyTitle_KeepsDueDate`: título `""` y fecha `"2026-11-03"` dan 422, el cuerpo contiene `"El título es obligatorio."` y `value="2026-11-03"`, y no se crea ninguna tarea.
- `Test_TasksIndex_ShowsDueDate`: con una tarea con fecha `2026-11-03` y otra sin fecha, GET `/` da 200 y contiene `datetime="2026-11-03"` y `3 de noviembre de 2026`, y `strings.Count(body, `class="task-due"`) == 1`.
- `Test_TasksIndex_ListsTasks` (existente): añadir `as.NotContains(body, `class="task-due"`)`, porque el fixture no tiene fechas.
- `Test_TasksEdit_PrefillsDueDate`: una tarea con fecha da en la edición `name="due_date"` y `value="2026-11-03"`.
- `Test_TasksUpdate_ChangesDueDate`: PUT con título y `due_date` `"2026-12-24"` da 303, la fecha recargada es `"2026-12-24"` y `Completed` no cambia.
- `Test_TasksUpdate_ClearsDueDate`: PUT con título y `due_date` `""` sobre una tarea con fecha da 303 y `DueDate.Valid == false`.
- `Test_TasksUpdate_InvalidDueDate`: PUT con `due_date` `"2026-13-01"` da 422 con el mensaje; tras recargar, título y fecha siguen como antes.
- `Test_TasksToggle_KeepsDueDate`: completar una tarea con fecha da 303, `Completed == true` y la misma `DueDateValue()`.

Los tests existentes deben seguir pasando sin más cambios. `Test_TasksUpdate_PersistsTitleOnly` envía solo título y campos protegidos sobre una tarea sin fecha, así que `due_date` ausente la deja sin fecha, como antes.

### Condiciones para ejecutar los tests

- PostgreSQL accesible en `127.0.0.1:5432` (`docker compose up -d db`; ahora mismo el contenedor `desarrollo-agentico-todo-03-db-1` está levantado) y `GO_ENV=test`.
- La base `todo_test` debe tener aplicadas **exactamente** las migraciones de esta rama. **Estado local observado:** `todo_test` tiene aplicada la versión `20261026000000`, que no existe en esta rama, y ya tiene una columna `due_date` de otra rama. Con ese estado, `soda migrate -e test` fallaría con «column "due_date" already exists». La base de test es desechable (la suite trunca las tablas en cada test), así que hay que recrearla:

  ```sh
  GO_ENV=test go run github.com/gobuffalo/pop/v6/soda@v6.4.1 reset -e test
  ```

  `soda reset` borra, crea y migra `todo_test` desde `./migrations`. Si falla porque hay conexiones abiertas, cerrar procesos de test colgados y reintentar. Después, comprobar con `docker exec desarrollo-agentico-todo-03-db-1 psql -U postgres -d todo_test -Atc "select version from schema_migration order by 1"` que solo aparecen `20260901000000`, `20260901000001` y `20261026180000`.
- No tocar `todo_development`: tiene una columna `priority` de otra rama (con valor por defecto), lo que no afecta a esta feature. Al arrancar la app, `models.Migrate()` le añadirá `due_date`.

### Validaciones a ejecutar

```sh
gofmt -l .                      # sin salida
go vet ./...
GO_ENV=test go test -p 1 ./...
go build ./...
git diff go.mod go.sum          # solo nulls pasa a directa
```

Comprobación manual opcional (no bloqueante): arrancar con `/env:start`, crear una tarea con fecha, verla en la lista, editarla, quitarle la fecha y revisar el formulario a 380 px de ancho.

## Riesgos y recuperación

- **Desfase de un día por zona horaria.** `ParseDueDate` produce medianoche UTC. pgx v5 codifica `time.Time` en `date` usando su año, mes y día, y lee `date` como medianoche UTC; el formateo usa `Day()`, `Month()` y `Year()` sin convertir zona. `Test_Task_DueDate_Persists` y `Test_TasksUpdate_ChangesDueDate` comprueban el viaje de ida y vuelta. Si fallaran por un día, revisar el modo de ejecución de consultas antes de tocar el diseño.
- **Base de test contaminada por otras ramas** (ver arriba). Recuperación: `soda reset -e test`.
- **Plush y métodos con receptor puntero.** Si `DueDateValue` o `DueDateLabel` se declaran con `*Task`, el render de la lista falla. Mantener receptor por valor.
- **`app.js` depende del marcado.** Si `.task-title` incluyera la fecha, el `aria-label` del botón de completar la leería. Mantener la fecha fuera de `.task-title` y `.task-actions` como hijo directo del `<li>`.
- **Clientes que envían PUT sin `due_date`** borran la fecha. Solo el formulario de edición usa esta ruta y siempre envía el campo; queda documentado en el comentario de `TasksUpdate`.
- **Caché del CSS.** Sin subir `?v=4`, los navegadores con la hoja anterior verían la fecha sin estilo.
- **Rollback.** La migración down elimina la columna y pierde las fechas guardadas. Para revertir el código basta con revertir el commit; la columna nullable sobrante no rompe la versión anterior, porque pop solo lee las columnas del struct.
