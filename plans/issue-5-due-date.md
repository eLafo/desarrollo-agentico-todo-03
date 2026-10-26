# Feature: fecha límite (solo día) en las tareas

## Encargo y criterios de aceptación

Issue [#5](https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5): «Quiero añadir fecha límite a las tasks». Cuerpo: «Vale con que sea simplemente un día. Sin hora».

Requisitos explícitos:

- Una tarea puede tener una fecha límite.
- La fecha límite es un día, sin hora.

Inferencias (decisiones de este plan):

- La fecha límite es opcional: las tareas existentes no la tienen y no todas las tareas la necesitan.
- Se indica al crear la tarea y se puede cambiar o quitar desde la edición.
- Se muestra en la lista con formato español `DD/MM/AAAA`.
- Un valor mal formado se rechaza con `400 Bad Request`, porque el control nativo `<input type="date">` no puede producirlo.

Criterios de aceptación:

1. En el formulario de creación hay un selector de fecha opcional (`name="due_date"`). Si se rellena, la tarea se crea con ese día; si se deja vacío, se crea sin fecha límite.
2. En la lista, las tareas con fecha límite muestran «Fecha límite: DD/MM/AAAA» bajo el título, dentro de `<time datetime="AAAA-MM-DD">`. Las tareas sin fecha límite no muestran nada adicional.
3. En la edición, el selector de fecha aparece rellenado con la fecha actual. Guardar con otro día la cambia; guardar con el campo vacío la quita. El estado de completada no cambia.
4. Completar o reabrir una tarea conserva su fecha límite.
5. La fecha se guarda en una columna `date` (sin hora) y, al releerla, es el mismo día que se envió, sin desplazamientos por zona horaria.
6. Un `due_date` no vacío que no sea un día válido con formato `AAAA-MM-DD` (por ejemplo `30/10/2026` o `2026-02-30`) responde `400` y no crea ni modifica la tarea.
7. Si el título no es válido, la respuesta `422` vuelve a mostrar en el formulario la fecha enviada.
8. Tras la migración, las tareas existentes quedan sin fecha límite y el comportamiento actual (crear, editar título, completar, eliminar) no cambia.

## Comportamiento actual y propuesta

Actualmente la tabla `tasks` tiene `id`, `title`, `completed`, `created_at` y `updated_at` (`migrations/20260901000000_create_tasks.up.fizz`). `models.Task` refleja esas columnas. `TasksCreate` enlaza el formulario directamente sobre `models.Task`; solo `title` es enlazable porque el resto de campos lleva `form:"-"`. `TasksUpdate` enlaza una estructura anónima que solo contiene `title`. La lista (`templates/tasks/index.plush.html`) muestra el título, y la edición (`templates/tasks/edit.plush.html`) solo permite cambiar el título. No existe ningún concepto de fecha límite.

Propuesta: es un cambio vertical mínimo que sigue los patrones existentes.

- Se añade una columna nullable `due_date` de tipo `date` mediante una migración fizz nueva.
- Se añade el campo `DueDate nulls.Time` a `models.Task`, con `form:"-"`. Así no lo enlaza `c.Bind`, igual que `Completed`.
- `models.ParseDueDate` convierte el valor del formulario: vacío → sin fecha; `AAAA-MM-DD` → día en UTC a medianoche; cualquier otro valor → error.
- Dos métodos de presentación en `Task` (`DueDateValue`, `DueDateLabel`) evitan formatear fechas en Plush.
- `TasksCreate` lee `due_date` con `c.Param` y `TasksUpdate` lo añade a su estructura de entrada. Ambos responden `400` ante un valor mal formado, siguiendo el patrón `c.Error(...)` existente.
- Las plantillas añaden el selector de fecha y la visualización. El CSS añade el estilo del `input[type="date"]` y la línea de fecha bajo el título.

`TasksToggle` no cambia: `tx.Update(task)` reescribe la tarea cargada, así que conserva `due_date`.

## Alcance

Incluido:

- Migración `up`/`down` para la columna `tasks.due_date` (`date`, nullable).
- Campo, parseo y formateo en `models/task.go`. `github.com/gobuffalo/nulls` pasa a ser una dependencia directa en `go.mod`.
- Lectura y persistencia de `due_date` en `TasksCreate` y `TasksUpdate`.
- Selector de fecha en los formularios de creación y edición, y visualización en la lista.
- Estilos en `public/assets/app.css` y cambio de versión del asset CSS en el layout.
- Tests de modelo y de acciones.

Excluido:

- Hora, zona horaria por usuario y recordatorios o notificaciones.
- Resaltar tareas vencidas o próximas a vencer. Exigiría decidir qué es «hoy» para cada usuario.
- Ordenar o filtrar por fecha límite. La lista sigue ordenada por `created_at asc, id asc`.
- Restringir fechas pasadas o imponer un rango (`min`/`max`).
- Cambios en `public/assets/app.js`, en `TasksToggle`, en la migración de semillas o en `fixtures/tasks.toml`.
- Mensaje de validación amigable (`422`) para fechas mal formadas: se responde `400`.
- Cambiar el texto de ayuda actual del título en la edición.

## Diseño por capas

### Persistencia y migraciones

Se crean dos archivos nuevos, que `migrations/embed.go` ya incluye por el patrón `*.fizz`:

- `migrations/20261026000000_add_due_date_to_tasks.up.fizz`:

  ```
  add_column("tasks", "due_date", "date", {"null": true})
  ```

- `migrations/20261026000000_add_due_date_to_tasks.down.fizz`:

  ```
  drop_column("tasks", "due_date")
  ```

El traductor PostgreSQL de fizz pasa `"date"` tal cual como tipo de columna. La columna es nullable y no tiene valor por defecto, así que las filas existentes y las semillas (`20260901000001_seed_tasks`) quedan con `NULL`. `models.Migrate()` (llamado desde `cmd/app/main.go`) aplica la migración al arrancar la aplicación.

### Modelo (`models/task.go`)

- Importa `github.com/gobuffalo/nulls`.
- Añade las constantes:

  ```go
  // DueDateLayout es el formato AAAA-MM-DD con el que se envía y se edita la fecha límite.
  const DueDateLayout = "2006-01-02"

  // dueDateLabelLayout es el formato DD/MM/AAAA con el que se muestra la fecha límite.
  const dueDateLabelLayout = "02/01/2006"
  ```

- Añade a `Task` el campo `DueDate` entre `Completed` y `CreatedAt`, y realinea con `gofmt`:

  ```go
  DueDate   nulls.Time `json:"due_date" db:"due_date" form:"-"`
  ```

- Añade las funciones:

  ```go
  // ParseDueDate interpreta un día con formato AAAA-MM-DD, sin hora.
  // Un valor vacío significa que la tarea no tiene fecha límite.
  func ParseDueDate(value string) (nulls.Time, error) {
  	value = strings.TrimSpace(value)
  	if value == "" {
  		return nulls.Time{}, nil
  	}
  	day, err := time.Parse(DueDateLayout, value)
  	if err != nil {
  		return nulls.Time{}, fmt.Errorf("fecha límite no válida: %w", err)
  	}
  	return nulls.NewTime(day), nil
  }

  // DueDateValue devuelve la fecha límite como AAAA-MM-DD para un campo de tipo date, o "" si no tiene.
  func (t Task) DueDateValue() string {
  	if !t.DueDate.Valid {
  		return ""
  	}
  	return t.DueDate.Time.Format(DueDateLayout)
  }

  // DueDateLabel devuelve la fecha límite como DD/MM/AAAA para mostrarla, o "" si no tiene.
  func (t Task) DueDateLabel() string {
  	if !t.DueDate.Valid {
  		return ""
  	}
  	return t.DueDate.Time.Format(dueDateLabelLayout)
  }
  ```

  Los dos métodos usan receptor por valor a propósito. En `index.plush.html`, la lista itera `models.Tasks` (`[]Task`) y Plush tiene que poder llamarlos sobre esos valores; en la edición, `task` es `*Task`, que también los incluye.

- `Validate` no cambia. La fecha ya llega validada por `ParseDueDate` y no hay otras restricciones.

Notas técnicas verificadas:

- `time.Parse` con este layout devuelve el día a medianoche en UTC.
- pgx v5 codifica `time.Time` para una columna `date` usando solo año, mes y día, y la decodifica como medianoche UTC.
- `nulls.Time` implementa `Scan`/`Value` y escribe `NULL` cuando `Valid` es `false`.

### Acciones (`actions/tasks.go`)

Las rutas de `actions/app.go` no cambian.

`TasksCreate`: después de `task.Title = strings.TrimSpace(task.Title)` y antes de `tx.ValidateAndCreate`, añade:

```go
dueDate, err := models.ParseDueDate(c.Param("due_date"))
if err != nil {
	return c.Error(http.StatusBadRequest, err)
}
task.DueDate = dueDate
```

`c.Param` incluye los valores del cuerpo del formulario: Buffalo hace `req.ParseForm()` al construir el contexto. La línea siguiente, `verrs, err := tx.ValidateAndCreate(task)`, sigue compilando porque `verrs` es una variable nueva. La rama `422` ya vuelve a mostrar `task`, que conserva la fecha.

`TasksUpdate`:

- Cambia el comentario a `// TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.`
- Amplía la estructura de entrada:

  ```go
  input := struct {
  	Title   string `form:"title" json:"title"`
  	DueDate string `form:"due_date" json:"due_date"`
  }{}
  ```

- Después de `c.Bind(&input)` y antes de asignar el título, añade:

  ```go
  dueDate, err := models.ParseDueDate(input.DueDate)
  if err != nil {
  	return c.Error(http.StatusBadRequest, err)
  }
  ```

- Junto a `task.Title = strings.TrimSpace(input.Title)`, añade `task.DueDate = dueDate`. Así una fecha vacía quita la fecha límite.

Si `due_date` no llega en la petición, también se quita la fecha. El formulario de edición siempre envía el campo, así que este caso no se da desde la interfaz.

### Renderizado (Plush)

`templates/tasks/index.plush.html`, en cada `<li class="task">`: sustituye `<span class="task-title"><%= t.Title %></span>` por:

```html
<div class="task-main">
  <span class="task-title"><%= t.Title %></span>
  <%= if (t.DueDate.Valid) { %>
    <span class="task-due">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></span>
  <% } %>
</div>
```

`.task-title` sigue existiendo, así que `app.js` sigue leyendo el título con `task.querySelector(".task-title")` sin incluir la fecha. `.task-status` y `.task-actions` siguen siendo hijos directos de `.task`.

En el formulario de creación (`.new-task-row`), entre el `<input id="task-title" …>` y el botón «Añadir», añade:

```html
<label for="task-due-date" class="sr-only">Fecha límite (opcional)</label>
<input
  id="task-due-date"
  type="date"
  name="due_date"
  value="<%= task.DueDateValue() %>"
  title="Fecha límite (opcional)"
>
```

`templates/tasks/edit.plush.html`: entre el bloque de error del título (`<%= if (errors.HasAny()) { %> … <% } %>`) y `<div class="edit-task-actions">`, añade:

```html
<div class="edit-task-due">
  <label for="task-due-date">Fecha límite</label>
  <input
    id="task-due-date"
    type="date"
    name="due_date"
    value="<%= task.DueDateValue() %>"
    aria-describedby="due-date-help"
  >
  <p id="due-date-help" class="field-hint">Opcional. Déjala vacía si la tarea no tiene fecha límite.</p>
</div>
```

La lógica de errores existente (`errors.HasAny()` → error de título) sigue siendo correcta, porque la única validación con `422` sigue siendo la del título.

### CSS (`public/assets/app.css`)

- Mueve el posicionamiento en la rejilla de `.task-title` a un contenedor nuevo, `.task-main`. Sustituye la regla `.task-title { align-self: center; min-width: 0; padding: 0.65rem 0; overflow-wrap: anywhere; line-height: 1.55; }` por:

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
    color: var(--color-muted);
    font-size: 0.8rem;
    line-height: 1.4;
  }
  ```

  Sin fecha, la fila se ve igual que hoy. Las reglas `.task-completed .task-title` y `@media (max-width: 760px) .task-title` no cambian.

- Añade `input[type="date"]` al selector `input[type="text"] { … }`, que queda como `input[type="text"],\ninput[type="date"] { … }`.
- Después de `.new-task-row input { flex: 1; }`, añade la regla siguiente. Tiene más especificidad que `.new-task-row input`, así que también prevalece en `@media (max-width: 380px)`, donde la fecha pasa a la segunda línea junto al botón:

  ```css
  .new-task-row input[type="date"] {
    flex: 0 0 auto;
    width: auto;
  }
  ```

- Junto a las reglas de `.edit-task`, añade:

  ```css
  .edit-task-due {
    margin-top: 1.5rem;
  }
  ```

`templates/application.plush.html`: cambia `/assets/app.css?v=3` por `/assets/app.css?v=4` para invalidar la caché, como en cambios anteriores.

### Dependencias

`github.com/gobuffalo/nulls v0.4.2` ya figura en `go.mod` como `// indirect`, y `go.sum` ya tiene sus hashes. Después de importarlo en `models/task.go`, ejecuta `go mod tidy`. El único cambio esperado es que esa línea pase al bloque `require` directo, sin `// indirect`. No cambia ninguna versión ni `go.sum`.

## Archivos relevantes

| Ruta | Responsabilidad |
| --- | --- |
| `migrations/20261026000000_add_due_date_to_tasks.up.fizz` (nuevo) | Añade la columna `tasks.due_date` de tipo `date`, nullable. |
| `migrations/20261026000000_add_due_date_to_tasks.down.fizz` (nuevo) | Elimina la columna. |
| `migrations/embed.go` | Embebe `*.fizz` y `*.sql`. No cambia. |
| `models/task.go` | Campo `DueDate`, `ParseDueDate`, `DueDateValue` y `DueDateLabel`. |
| `models/task_test.go` | Tests de parseo y persistencia de la fecha límite. |
| `actions/tasks.go` | `TasksCreate` y `TasksUpdate` leen y guardan `due_date`. |
| `actions/tasks_test.go` | Tests de creación, listado, edición, actualización y toggle con fecha límite. |
| `actions/app.go` | Rutas. No cambia. |
| `templates/tasks/index.plush.html` | Muestra la fecha en la lista y añade el selector al crear. |
| `templates/tasks/edit.plush.html` | Añade el selector de fecha a la edición. |
| `templates/application.plush.html` | Versión del CSS (`v=4`). |
| `public/assets/app.css` | Estilos de `.task-main`, `.task-due`, `input[type="date"]` y `.edit-task-due`. |
| `go.mod` | `github.com/gobuffalo/nulls` pasa a dependencia directa. |
| `public/assets/app.js` | Usa `.task-title` y `.task-actions`. No cambia; la estructura propuesta mantiene esos selectores. |
| `fixtures/tasks.toml` | Escenario «tres tareas» sin fecha límite. No cambia. |

## Implementación paso a paso

1. Comprueba que la rama parte limpia: `git status --short --branch`.
2. Crea las dos migraciones fizz con el contenido exacto de «Persistencia y migraciones».
3. Edita `models/task.go`: añade el import de `nulls`, las constantes, el campo `DueDate`, `ParseDueDate`, `DueDateValue` y `DueDateLabel`, tal como se describen en «Modelo».
4. Ejecuta `go mod tidy` y comprueba con `git diff go.mod go.sum` que solo cambia la línea de `github.com/gobuffalo/nulls`, que pasa al bloque directo.
5. Edita `actions/tasks.go` (`TasksCreate`, `TasksUpdate` y el comentario de `TasksUpdate`) tal como se describe en «Acciones».
6. Edita `templates/tasks/index.plush.html` y `templates/tasks/edit.plush.html` tal como se describe en «Renderizado».
7. Edita `public/assets/app.css` y cambia la versión del CSS en `templates/application.plush.html`.
8. Añade los tests de «Tests y validación» y la aserción nueva en `Test_TasksCreate_Valid`.
9. Ejecuta `gofmt -w` sobre los archivos Go modificados y después todas las validaciones.
10. Revisa `git diff --check`, `git diff` y `git status --short`. Solo deben aparecer los archivos listados en «Archivos relevantes» como modificados o nuevos, además de este plan.

## Tests y validación

### Tests de modelo (`models/task_test.go`, métodos de `ModelSuite`)

- `Test_ParseDueDate_Empty`: `""` y `"   "` devuelven `Valid == false` sin error.
- `Test_ParseDueDate_Valid`: `"2026-10-30"` devuelve `Valid == true`, `Time.Format(DueDateLayout) == "2026-10-30"`, `Time.Hour() == 0` y `Time.Location() == time.UTC`.
- `Test_ParseDueDate_Invalid`: `"30/10/2026"`, `"2026-02-30"`, `"2026-10-30T10:00:00Z"` y `"mañana"` devuelven error y `Valid == false`.
- `Test_Task_DueDate_PersistsAsDay`: crea una tarea con `ParseDueDate("2026-10-30")` usando `ValidateAndCreate` y la vuelve a leer con `ms.DB.Find`. Espera `DueDate.Valid`, `DueDateValue() == "2026-10-30"` y `DueDateLabel() == "30/10/2026"`.
- `Test_Task_WithoutDueDate_PersistsNull`: crea una tarea sin fecha y la vuelve a leer. Espera `DueDate.Valid == false`, `DueDateValue() == ""` y `DueDateLabel() == ""`.

### Tests de acciones (`actions/tasks_test.go`, métodos de `ActionSuite`)

Añade un helper que siga el patrón de `taskForEditing`:

```go
func (as *ActionSuite) taskWithDueDate(day string) *models.Task {
	dueDate, err := models.ParseDueDate(day)
	as.NoError(err)
	task := &models.Task{Title: "Tarea con fecha límite", DueDate: dueDate}
	verrs, err := as.DB.ValidateAndCreate(task)
	as.NoError(err)
	as.False(verrs.HasAny())
	return task
}
```

Tests:

- `Test_TasksCreate_Valid` (existente): añade `as.False(task.DueDate.Valid)`.
- `Test_TasksCreate_WithDueDate`: `Post(url.Values{"title": {"Con fecha"}, "due_date": {"2026-10-30"}})` responde `303`. `as.DB.First(task)` da `DueDateValue() == "2026-10-30"`.
- `Test_TasksCreate_InvalidDueDate`: `due_date` `"30/10/2026"` responde `400` y el número de tareas es `0`.
- `Test_TasksCreate_EmptyTitle_KeepsDueDate`: título vacío y `due_date` `"2026-10-30"` responde `422`. El cuerpo contiene `value="2026-10-30"` y el número de tareas es `0`.
- `Test_TasksIndex_ShowsDueDate`: con `taskWithDueDate("2026-10-30")`, `GET /` responde `200` y el cuerpo contiene `Fecha límite: <time datetime="2026-10-30">30/10/2026</time>`.
- `Test_TasksIndex_WithoutDueDate`: con `LoadFixture("tres tareas")`, el cuerpo no contiene `class="task-due"` y sí contiene `name="due_date"`.
- `Test_TasksEdit_PrefillsDueDate`: con `taskWithDueDate("2026-10-30")`, `GET /tasks/{id}/edit` contiene `name="due_date"` y `value="2026-10-30"`.
- `Test_TasksUpdate_ChangesDueDate`: con `taskWithDueDate("2026-10-30")` marcada como completada y `as.DB.Update`, `PUT` con `title` `"Nuevo título"` y `due_date` `"2026-11-15"` responde `303`. Tras recargar, `DueDateValue() == "2026-11-15"`, el título está actualizado y `Completed` sigue en `true`.
- `Test_TasksUpdate_ClearsDueDate`: `PUT` con título válido y `due_date` `""` responde `303`. Tras recargar, `DueDate.Valid == false`.
- `Test_TasksUpdate_InvalidDueDate`: `PUT` con `due_date` `"2026-02-30"` responde `400`. Tras recargar, el título y la fecha no cambian (`"2026-10-30"`).
- `Test_TasksToggle_KeepsDueDate`: con `taskWithDueDate("2026-10-30")`, `PUT /tasks/{id}/toggle` responde `303`. Tras recargar, `Completed == true` y `DueDateValue() == "2026-10-30"`.

Los tests existentes deben seguir pasando sin cambios, salvo la aserción añadida en `Test_TasksCreate_Valid`.

### Preparación de la base de datos de test

Los tests de `models` y `actions` necesitan PostgreSQL en `127.0.0.1:5432`, la base `todo_test` y las migraciones aplicadas, igual que en CI (`.github/workflows/ci.yaml`).

1. `docker compose up -d db` y espera a que `docker compose ps` muestre `db` como `healthy`. Si el puerto `5432` está ocupado por otro proceso, detente e informa. No lo finalices.
2. Crea `todo_test` si no existe: `docker compose exec -T db psql -U postgres -tAc "SELECT 1 FROM pg_database WHERE datname='todo_test'" | grep -q 1 || docker compose exec -T db createdb -U postgres todo_test`.
3. Aplica las migraciones: `GO_ENV=test go run github.com/gobuffalo/pop/v6/soda@v6.4.1 migrate -e test`.
4. Comprueba que la migración es reversible: `GO_ENV=test go run github.com/gobuffalo/pop/v6/soda@v6.4.1 migrate down -e test` (deshace solo la última), y después vuelve a ejecutar el comando del paso 3.

### Comandos de validación

Todos deben terminar con éxito:

- `gofmt -l .` → sin salida.
- `go vet ./...`
- `go mod tidy -diff` → sin salida, después del paso 4 de la implementación.
- `GO_ENV=test go test -p 1 ./...`
- `go build ./...`
- `git diff --check`

Verificación manual opcional, si el entorno lo permite: arranca la aplicación (`/env:start`, o `docker compose up --build -d` en modo Docker completo) y comprueba en `http://localhost:3000` los puntos siguientes:

- Puedes crear una tarea con fecha y otra sin fecha.
- La fecha aparece bajo el título.
- Puedes cambiar y quitar la fecha desde la edición.
- Completar la tarea conserva la fecha.

## Riesgos y recuperación

- **Desplazamiento de día por zona horaria**: la columna es `date` y el valor se parsea en UTC. pgx codifica solo año, mes y día. `Test_Task_DueDate_PersistsAsDay` y los tests de acciones con lectura de vuelta lo protegen. Si fallasen, no cambies a `timestamp`. Revisa el layout de `ParseDueDate` y la ubicación (`time.UTC`).
- **Semántica de actualización**: un `PUT /tasks/{id}` sin `due_date` quita la fecha límite. La interfaz siempre envía el campo. No hay API JSON ni otros clientes.
- **Valores mal formados**: responden `400` con la página de error de Buffalo, no con un mensaje en el formulario. Solo afecta a peticiones manipuladas o a navegadores sin `<input type="date">`. Si más adelante se quiere un `422` amigable, es una mejora aparte.
- **Migración en producción**: `ALTER TABLE … ADD COLUMN` nullable sin valor por defecto es inmediata y no reescribe la tabla. La migración `down` elimina la columna y las fechas guardadas. Para revertir, ejecuta `soda migrate down` antes de desplegar la versión anterior, o despliega la versión anterior directamente: el código anterior no lee `due_date` y la columna sobrante no le afecta.
- **Maquetación de la fila**: `.task-title` pasa a ser hijo de `.task-main`. Si la fila sin fecha cambiase visualmente, compara con `.task-title` original: el padding y el `align-self` se movieron a `.task-main`.
- **Caché del CSS**: sin cambiar la versión (`?v=4`), los navegadores podrían usar el CSS anterior y mostrar el selector sin estilo.
- **Trabajo paralelo**: existen otras ramas y PR abiertas para la issue #5 (#6–#12) que tocan los mismos archivos. Solo una debería integrarse. Las demás entrarán en conflicto en `models/task.go`, `actions/tasks.go`, las plantillas y las migraciones.
