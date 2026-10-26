# Feature: fecha límite (solo día) en las tareas

Origen: issue #5 — https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5
Rama: `feature/issue-5-fecha-limite` (creada desde `origin/main`).

## Encargo y criterios de aceptación

Texto literal de la issue:

> **Quiero añadir fecha límite a las tasks**
> Vale con que sea simplemente un día. Sin hora

### Requisitos explícitos

- R1. Una tarea puede tener una fecha límite.
- R2. La fecha límite es un día del calendario, sin hora ni zona horaria.

### Requisitos inferidos (decisiones de este plan)

- I1. La fecha límite es **opcional**. Las tareas existentes y las nuevas sin fecha siguen siendo válidas. La issue no la pide obligatoria y hay datos sembrados sin ella.
- I2. La fecha se asigna al **crear** la tarea y se puede **cambiar o quitar** al **editarla**.
- I3. La lista muestra la fecha de cada tarea que la tenga, en formato español `DD/MM/AAAA`.
- I4. Se acepta cualquier día válido, también uno pasado. No hay restricción de rango.
- I5. Un valor con formato no válido (solo posible con peticiones manipuladas, porque `<input type="date">` envía `""` o `AAAA-MM-DD`) produce un error de validación 422 en español y no cambia nada.

### Criterios de aceptación

1. En el formulario de creación de `/` hay un campo de fecha (`type="date"`, `name="due_date"`) opcional.
2. Si se crea una tarea con fecha `2026-11-05`, se guarda exactamente ese día. Al recargarla, su valor es `2026-11-05`, sin desplazamiento por zona horaria.
3. Si se crea una tarea sin fecha, se guarda sin fecha límite (`NULL`).
4. En la lista, una tarea con fecha muestra `Fecha límite: 05/11/2026` en un elemento `<time datetime="2026-11-05">`. Una tarea sin fecha no muestra nada.
5. El formulario de edición precarga la fecha actual (`value="2026-11-05"`) o queda vacío si no hay fecha.
6. Al guardar la edición con otra fecha, se cambia. Si se guarda con el campo vacío, se quita. El estado `Completed`, el `ID` y `CreatedAt` no cambian.
7. Completar o reabrir una tarea conserva su fecha límite.
8. Un `due_date` no válido (p. ej. `2026-02-30`, `05/11/2026`, `mañana`) responde 422 con `La fecha límite no es válida.` y no crea ni modifica la tarea. Si el título también es inválido, se muestran los dos errores.
9. Si la creación falla por un título inválido, el formulario conserva la fecha válida introducida.

## Comportamiento actual y propuesta

### Actual

- Tabla `tasks` (`migrations/20260901000000_create_tasks.up.fizz`): `id`, `title`, `completed`, `created_at`, `updated_at`. No existe ninguna columna de fecha límite.
- `models.Task` (`models/task.go`) solo valida el título.
- `TasksCreate` (`actions/tasks.go:30`) hace `c.Bind` sobre `models.Task`. Los campos protegidos llevan `form:"-"`. Después llama a `tx.ValidateAndCreate`.
- `TasksUpdate` (`actions/tasks.go:75`) hace `Bind` en un struct anónimo con solo `Title`. Es una lista blanca que conserva `Completed`, `ID` y `CreatedAt` (`Test_TasksUpdate_PersistsTitleOnly`). Después llama a `tx.ValidateAndUpdate`.
- `templates/tasks/index.plush.html` y `templates/tasks/edit.plush.html` muestran el error de título con la condición `errors.HasAny()`, sin distinguir el campo.
- `cmd/app/main.go` aplica las migraciones pendientes al arrancar (`models.Migrate()`). En CI se aplican con `soda migrate -e test`.

### Propuesta (cambio vertical mínimo)

1. **Persistencia**: una migración fizz nueva añade `tasks.due_date` de tipo `date`, admite `NULL` y no tiene valor por defecto.
2. **Modelo**:
   - Campo `DueDate *time.Time` (`db:"due_date"`, `form:"-"`).
   - Función `ParseDueDate(string) (*time.Time, error)` para el formato `AAAA-MM-DD`.
   - Métodos de presentación `HasDueDate`, `DueDateValue` y `DueDateLabel`.
3. **Acciones**: crear y actualizar comparten un único helper. El helper hace `Bind` sobre un struct con lista blanca `{title, due_date}`, aplica los valores y valida. Un `due_date` no válido se añade como error `due_date`.
4. **Templates**: campo de fecha en los formularios de creación y edición, fecha visible en la lista y errores mostrados por campo.
5. **CSS**: estilos del campo de fecha y de la fecha en la fila de la tarea. Se sube `app.css?v=3` a `?v=4`.

Sin dependencias nuevas: se usa `*time.Time` en lugar de `nulls.Time`, así que `go.mod` no cambia. Sin JavaScript nuevo.

## Alcance

### Incluido

- Migración `up`/`down` de la columna `due_date`.
- Campo, parseo y formateo de la fecha en `models.Task`.
- Crear, editar (cambiar o quitar) y mostrar la fecha límite.
- Validación del formato con un mensaje en español.
- Tests de modelo y de acciones.
- CSS mínimo para el nuevo campo y la nueva línea de la fila.

### Excluido

- Resaltar tareas vencidas o próximas a vencer. Requiere definir «hoy» y una zona horaria que la issue no menciona.
- Ordenar o filtrar por fecha límite. El orden sigue siendo `created_at asc, id asc`.
- Hora, recordatorios y notificaciones.
- Cambios en la migración de semillas (`20260901000001_seed_tasks.*`) y en `fixtures/tasks.toml`. Sus filas quedan sin fecha.
- API JSON, `database.yml`, `compose.yaml`, CI, `go.mod` y `public/assets/app.js`.

## Diseño por capas

### Persistencia

`migrations/20261026000000_add_due_date_to_tasks.up.fizz`:

```
add_column("tasks", "due_date", "date", {"null": true})
```

`migrations/20261026000000_add_due_date_to_tasks.down.fizz`:

```
drop_column("tasks", "due_date")
```

El traductor PostgreSQL de fizz deja `date` tal cual (columna `DATE`). `migrations/embed.go` ya incrusta `*.fizz`, así que no hay que tocarlo.

### Modelo (`models/task.go`)

```go
// DueDateLayout es el formato AAAA-MM-DD que envía el campo de fecha del formulario.
const DueDateLayout = "2006-01-02"

// dueDateLabelLayout es el formato DD/MM/AAAA con el que se muestra la fecha límite.
const dueDateLabelLayout = "02/01/2006"

type Task struct {
	ID        uuid.UUID  `json:"id" db:"id" form:"-"`
	Title     string     `json:"title" db:"title" form:"title"`
	Completed bool       `json:"completed" db:"completed" form:"-"`
	DueDate   *time.Time `json:"due_date" db:"due_date" form:"-"`
	CreatedAt time.Time  `json:"created_at" db:"created_at" form:"-"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at" form:"-"`
}

// ParseDueDate convierte el valor AAAA-MM-DD de un campo de fecha en una fecha límite.
// Un valor vacío o con solo espacios devuelve nil: la tarea no tiene fecha límite.
func ParseDueDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	date, err := time.Parse(DueDateLayout, value)
	if err != nil {
		return nil, err
	}
	return &date, nil
}

// HasDueDate indica si la tarea tiene fecha límite.
func (t Task) HasDueDate() bool { ... }

// DueDateValue devuelve la fecha límite en formato AAAA-MM-DD, o "" si no tiene.
func (t Task) DueDateValue() string { ... }

// DueDateLabel devuelve la fecha límite en formato DD/MM/AAAA, o "" si no tiene.
func (t Task) DueDateLabel() string { ... }
```

Decisiones:

- **`*time.Time` y no `nulls.Time`**: `database/sql` escanea `NULL` como `nil` y pgx codifica un puntero `nil` como `NULL`. Así no se añade una dependencia directa a `go.mod`.
- **Sin desplazamiento de día**: `time.Parse` devuelve medianoche UTC. pgx codifica una columna `date` con el año, el mes y el día del valor, y la lee como medianoche UTC. **No uses `.Local()` ni `time.Now()`** en este flujo. `time.Parse` ya rechaza días inexistentes como `2026-02-30`.
- **Receptor por valor en los tres métodos de presentación**: Plush itera `for (t) in tasks` con copias no direccionables de `models.Task`. Con receptor puntero, `t.DueDateLabel()` no se resolvería en la lista. `Validate` mantiene su receptor puntero actual.
- `Validate` no cambia: la fecha no tiene reglas propias una vez parseada.

### Acciones (`actions/tasks.go`)

Se añade un formulario con lista blanca y un helper compartido:

```go
// taskForm recoge los campos editables de una tarea desde los formularios.
type taskForm struct {
	Title   string `form:"title" json:"title"`
	DueDate string `form:"due_date" json:"due_date"`
}

// applyTaskForm copia en la tarea los campos del formulario y la valida.
// Una fecha límite con formato no válido no se aplica y se informa como error de "due_date".
func applyTaskForm(c buffalo.Context, tx *pop.Connection, task *models.Task) (*validate.Errors, error) {
	form := taskForm{}
	if err := c.Bind(&form); err != nil {
		return nil, err
	}
	task.Title = strings.TrimSpace(form.Title)

	dueDate, dueDateErr := models.ParseDueDate(form.DueDate)
	if dueDateErr == nil {
		task.DueDate = dueDate
	}

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

- **`TasksCreate`**: `task := &models.Task{}` y después `verrs, err := applyTaskForm(c, tx, task)`. Si `verrs.HasAny()`, mantiene exactamente el render 422 actual de `tasks/index.plush.html` con `tasks`, `task` y `errors`. En otro caso llama a `tx.Create(task)`, añade el flash `Tarea creada.` y redirige con 303 a `/`. Ya no hace `Bind` sobre `models.Task`.
- **`TasksUpdate`**: tras `tx.Find`, `verrs, err := applyTaskForm(c, tx, task)` sustituye al struct anónimo actual. Si `verrs.HasAny()`, mantiene el render 422 de `tasks/edit.plush.html`. En otro caso llama a `tx.Update(task)` y conserva el flash y el 303. Actualiza el comentario: «actualiza el título y la fecha límite, conservando los demás datos de la tarea».
- **Por qué `Validate` + `Create`/`Update` en lugar de `ValidateAndCreate`/`ValidateAndUpdate`**: el error de formato de la fecha debe impedir que se guarde aunque el título sea válido, y debe mostrarse junto a los errores del título. Pop no hace más que esto en `ValidateAndCreate` para este modelo, porque `Task` no define `ValidateCreate` ni `ValidateUpdate`.
- **Semántica de un `due_date` ausente en la petición**: equivale a vacío y quita la fecha. Los formularios HTML siempre envían el campo. No se distingue «ausente» de «vacío».
- `TasksToggle`, `TasksDestroy`, `TasksIndex`, `TasksEdit` y `actions/app.go` (rutas) no cambian. `TasksToggle` carga la tarea con `Find`, así que `tx.Update` reescribe la fecha que ya tenía.

### Renderizado

**`templates/tasks/index.plush.html`**

- En cada `<li class="task">`, justo después de `<span class="task-title">…</span>`, añade:

  ```html
  <%= if (t.HasDueDate()) { %>
    <time class="task-due" datetime="<%= t.DueDateValue() %>">Fecha límite: <%= t.DueDateLabel() %></time>
  <% } %>
  ```

- En el formulario de creación, dentro de `.new-task-row`, entre el input de título y el botón «Añadir», añade:

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

- Los errores pasan a mostrarse por campo:
  - Input de título: la condición `errors.HasAny()` pasa a `len(errors.Get("title")) > 0`. Conserva `autofocus aria-invalid="true" aria-describedby="title-error"`.
  - El bloque `<p id="title-error" …>` se muestra solo con esa misma condición.
  - Debajo, un bloque `<p id="due-date-error" class="field-error" role="alert">` con `errors.Get("due_date")`, mostrado solo si hay errores de `due_date`.

**`templates/tasks/edit.plush.html`**

- Cambia el texto de `title-help` a `Hasta 200 caracteres. El estado de la tarea no cambia.`
- El input de título y el bloque `title-error` usan la condición `len(errors.Get("title")) > 0` en lugar de `errors.HasAny()`. Conserva el resto de atributos (`aria-describedby="title-error"` o `"title-help"`).
- Tras el bloque de error del título y antes de `.edit-task-actions`, añade:

  ```html
  <label for="task-due-date" class="due-date-label">Fecha límite</label>
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

**`templates/application.plush.html`**: `app.css?v=3` pasa a `app.css?v=4`. No toques la línea de `app.js`.

### CSS (`public/assets/app.css`)

- El selector `input[type="text"] { … }` pasa a `input[type="text"], input[type="date"] { … }` (un selector por línea, como el resto del archivo), con el mismo bloque. Añade después:

  ```css
  input[type="date"] {
    width: auto;
  }
  ```

- `.new-task-row input { flex: 1; }` pasa a `.new-task-row input[type="text"] { flex: 1; }` para que el campo de fecha no se estire. Añade:

  ```css
  .new-task-row input[type="date"] {
    flex: 0 0 auto;
  }
  ```

- Fecha en la fila. La rejilla `.task` tiene tres columnas: toggle, título y acciones. Se coloca de forma explícita en la fila 2, columna 2, para no desplazar las acciones:

  ```css
  .task-due {
    grid-column: 2;
    grid-row: 2;
    margin-top: -0.4rem;
    padding-bottom: 0.4rem;
    color: var(--color-muted);
    font-size: 0.8rem;
  }
  ```

- Etiqueta de fecha en edición:

  ```css
  .edit-task .due-date-label {
    margin-top: 1.5rem;
  }
  ```

- Móvil: dentro de `@media (max-width: 760px)`, haz que la fila de creación se parta. Título a ancho completo junto al `+`, y en la segunda línea, la fecha (alineada con el título) y el botón:

  ```css
  .new-task-row {
    flex-wrap: wrap;
  }

  .new-task-row input[type="text"] {
    flex-basis: calc(100% - 36px - 0.4rem);
  }

  .new-task-row input[type="date"] {
    flex: 1;
    margin-left: calc(36px + 0.4rem);
  }
  ```

  Las reglas existentes de `.new-task-row` en ese bloque (`gap: 0.4rem`) se mantienen. En `@media (max-width: 380px)`, el selector `.new-task-row input` pasa a `.new-task-row input[type="text"]`. El resto del bloque queda igual.

## Archivos relevantes

| Ruta | Responsabilidad | Cambio |
|---|---|---|
| `migrations/20261026000000_add_due_date_to_tasks.up.fizz` | Añade la columna `due_date DATE NULL` | Nuevo |
| `migrations/20261026000000_add_due_date_to_tasks.down.fizz` | Elimina la columna | Nuevo |
| `models/task.go` | Modelo `Task`: campo, parseo y formateo | Modificar |
| `models/task_test.go` | Tests de modelo | Añadir tests |
| `actions/tasks.go` | Crear y editar con `applyTaskForm` | Modificar |
| `actions/tasks_test.go` | Tests de acciones | Añadir tests y una aserción |
| `templates/tasks/index.plush.html` | Lista y formulario de creación | Modificar |
| `templates/tasks/edit.plush.html` | Formulario de edición | Modificar |
| `templates/application.plush.html` | Layout; versión de `app.css` | `?v=3` → `?v=4` |
| `public/assets/app.css` | Estilos | Modificar |
| `migrations/embed.go`, `cmd/app/main.go`, `actions/app.go` | Incrustan y aplican migraciones; definen las rutas | Solo lectura (sin cambios) |

## Implementación paso a paso

1. Comprueba que estás en `feature/issue-5-fecha-limite`, que el árbol está limpio salvo este plan, y que `migrations/` solo contiene las migraciones `20260901000000` y `20260901000001`.
2. Crea las dos migraciones fizz con el contenido exacto de «Persistencia».
3. Modifica `models/task.go`: constantes, campo `DueDate`, `ParseDueDate` y los tres métodos con receptor por valor. Hace falta el import `strings`, que ya existe, y `time`, que ya existe.
4. Modifica `actions/tasks.go`: añade `taskForm` y `applyTaskForm`, y adapta `TasksCreate` y `TasksUpdate` como se describe. Conserva los mensajes flash, los códigos de estado y los templates.
5. Modifica `templates/tasks/index.plush.html`, `templates/tasks/edit.plush.html` y `templates/application.plush.html`.
6. Modifica `public/assets/app.css`.
7. Añade los tests de «Tests y validación».
8. Ejecuta `gofmt -w` sobre los `.go` tocados y después todas las validaciones.

## Tests y validación

### Tests de modelo (`models/task_test.go`, métodos de `ModelSuite`)

- `Test_ParseDueDate_Empty`: `""` y `"   "` devuelven `nil, nil`.
- `Test_ParseDueDate_Valid`: `"2026-11-05"` devuelve un puntero igual a `time.Date(2026, time.November, 5, 0, 0, 0, 0, time.UTC)`.
- `Test_ParseDueDate_Invalid`: `"2026-02-30"`, `"05/11/2026"`, `"2026-11-05T10:00"` y `"mañana"` devuelven error y `nil`.
- `Test_Task_DueDate_Persists`: crea una tarea con `DueDate` de `ParseDueDate("2026-11-05")` y la recarga con `Find`. Comprueba `HasDueDate()`, `DueDateValue() == "2026-11-05"` y `DueDateLabel() == "05/11/2026"`.
- `Test_Task_DueDate_OptionalByDefault`: crea una tarea sin fecha y la recarga. Comprueba `DueDate == nil`, `HasDueDate() == false` y `DueDateValue() == ""`.

### Tests de acciones (`actions/tasks_test.go`, métodos de `ActionSuite`)

Para preparar tareas con fecha, usa `models.ParseDueDate` en el test. Puedes añadir un helper pequeño, p. ej. `as.taskWithDueDate(value string) *models.Task`, en el estilo de `taskForEditing`.

- `Test_TasksCreate_Valid` (existente): añade `as.Nil(task.DueDate)`.
- `Test_TasksCreate_WithDueDate`: `Post(url.Values{"title": {"Con fecha"}, "due_date": {"2026-11-05"}})` responde 303. `DB.First` da `DueDateValue() == "2026-11-05"`.
- `Test_TasksCreate_InvalidDueDate`: título válido y `due_date` `"2026-02-30"` responden 422. El cuerpo contiene `La fecha límite no es válida.` y `aria-describedby="due-date-error"`, y `Count == 0`.
- `Test_TasksCreate_EmptyTitle_KeepsDueDate`: título `""` y `due_date` `"2026-11-05"` responden 422. El cuerpo contiene `El título es obligatorio.` y `value="2026-11-05"`, y `Count == 0`.
- `Test_TasksCreate_InvalidTitleAndDueDate`: título `""` y `due_date` `"x"` responden 422. El cuerpo contiene los dos mensajes.
- `Test_TasksIndex_ShowsDueDate`: una tarea con fecha `2026-11-05` en `GET /` muestra `datetime="2026-11-05"` y `Fecha límite: 05/11/2026`.
- `Test_TasksIndex_WithoutDueDate`: con el fixture `tres tareas`, el cuerpo no contiene `class="task-due"`.
- `Test_TasksIndex_NewTaskFormHasDueDate`: el cuerpo de `GET /` contiene `type="date"` y `name="due_date"`.
- `Test_TasksEdit_PrefillsDueDate`: la edición de una tarea con fecha contiene `value="2026-11-05"`.
- `Test_TasksUpdate_ChangesDueDate`: una tarea completada con fecha. `Put({"title": {"Título"}, "due_date": {"2026-12-01"}})` responde 303. Al recargar, la fecha es `"2026-12-01"` y `Completed` sigue siendo `true`.
- `Test_TasksUpdate_ClearsDueDate`: una tarea con fecha y `Put({"title": {"Título"}, "due_date": {""}})` responden 303. Al recargar, `DueDate == nil`.
- `Test_TasksUpdate_InvalidDueDate`: una tarea con fecha `2026-11-05` y `Put({"title": {"Otro"}, "due_date": {"05/11/2026"}})` responden 422 con el mensaje. Al recargar, el título sigue siendo el original y la fecha `"2026-11-05"`.
- `Test_TasksToggle_KeepsDueDate`: alterna una tarea con fecha. Al recargar, conserva `"2026-11-05"`.

Los tests existentes deben seguir en verde sin cambios, salvo la aserción añadida a `Test_TasksCreate_Valid`. En particular, `Test_TasksUpdate_PersistsTitleOnly`, `Test_TasksUpdate_EmptyTitle` (`aria-describedby="title-error"`) y los de toggle.

### Entorno de tests

Los tests necesitan PostgreSQL con la base `todo_test` migrada (`database.yml`: usuario y contraseña `postgres`, host `DB_HOST`, puerto fijo 5432).

**No uses servicios de otros proyectos.** Al planificar, el puerto 5432 del host lo ocupaba el contenedor `desarrollo-agentico-todo-07-db-1`, de otro repositorio. No lo detengas ni crees bases en él.

- **Si el puerto 5432 está libre**: `docker compose up -d db`, espera a que esté saludable, crea `todo_test` si hace falta y aplica las migraciones con `GO_ENV=test soda migrate -e test` (instala `soda` con `go install github.com/gobuffalo/pop/v6/soda@v6.4.1`, como en CI). Después ejecuta `GO_ENV=test go test -p 1 ./...`.
- **Si el puerto 5432 está ocupado por otro proyecto**: usa un entorno aislado en una red Docker propia (las imágenes `postgres:17.6-alpine` y `golang:1.26` ya están en local):

  ```sh
  docker network create todo03-issue5-test
  docker run -d --rm --name todo03-issue5-db --network todo03-issue5-test \
    -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=todo_test \
    --health-cmd "pg_isready -U postgres" --health-interval 2s postgres:17.6-alpine
  # espera a que docker inspect -f '{{.State.Health.Status}}' todo03-issue5-db devuelva healthy
  docker run --rm --network todo03-issue5-test \
    -e GO_ENV=test -e DB_HOST=todo03-issue5-db \
    -v "$PWD":/src -v "$HOME/go/pkg/mod":/go/pkg/mod -w /src golang:1.26 \
    sh -c 'go install github.com/gobuffalo/pop/v6/soda@v6.4.1 && soda migrate -e test && go test -p 1 ./...'
  docker stop todo03-issue5-db && docker network rm todo03-issue5-test
  ```

  `soda migrate` solo aplica la migración `up`. Para probar también la `down`, ejecuta antes de `go test` `soda migrate down -e test && soda migrate -e test` (opcional).

### Comandos de validación

- `gofmt -l .` no muestra nada.
- `go vet ./...`
- `GO_ENV=test go test -p 1 ./...` (en el entorno descrito) deja todo en verde.
- `go build ./...`

### Validación manual (opcional, no bloqueante)

Arranca la app con una base de desarrollo propia, sin reutilizar la de otro proyecto. La migración se aplica sola al arrancar. Comprueba:

- crear con y sin fecha;
- ver `Fecha límite: DD/MM/AAAA` bajo el título;
- editar, cambiar y quitar la fecha;
- completar una tarea, que conserva la fecha;
- el diseño a menos de 760px.

## Riesgos y recuperación

- **Desplazamiento de un día por zona horaria**: queda mitigado porque se parsea en UTC, la columna es `DATE` y no se convierte a la hora local. `Test_Task_DueDate_Persists` y `Test_TasksCreate_WithDueDate` lo comprueban con un ida y vuelta real contra PostgreSQL. Si fallan con un día de diferencia, no lo corrijas sumando horas: detente e informa.
- **Métodos de Plush sobre elementos de la lista**: dependen del receptor por valor. Si `t.HasDueDate()` falla al renderizar, revisa el receptor antes de cambiar el diseño.
- **Errores por campo en los templates**: cambiar `errors.HasAny()` por condiciones por campo podría dejar de mostrar el error de título. `Test_TasksCreate_EmptyTitle_Invalid`, `Test_TasksUpdate_EmptyTitle` y `Test_TasksCreate_InvalidTitleAndDueDate` lo cubren.
- **Cambio de `ValidateAndCreate`/`ValidateAndUpdate` a `Validate` + `Create`/`Update`**: es equivalente para `Task`, que no tiene hooks `ValidateCreate` ni `ValidateUpdate`. Si en el futuro se añaden, habrá que invocarlos en `applyTaskForm`.
- **Caché del CSS**: queda mitigado al subir `app.css?v=4`.
- **Conflicto con trabajo local no publicado**: `main` local tiene un commit sin publicar (`429e9d6`, arreglo del toggle) que añade tests al final de `actions/tasks_test.go` y cambia la línea de `app.js` en `application.plush.html`. Esta rama sale de `origin/main`. Cuando se publiquen ambos, `actions/tasks_test.go` puede necesitar una resolución trivial de conflicto: se conservan los dos bloques de tests. La línea de `app.css` no se solapa con la de `app.js`.
- **Migración en producción**: `ADD COLUMN … NULL` sin valor por defecto es instantánea en PostgreSQL y no reescribe la tabla. El arranque la aplica sola.
- **Recuperación**: `git revert` del commit. Si la migración ya se aplicó en algún entorno, primero ejecuta `soda migrate down -e <entorno>` (o aplica `drop_column`), porque al revertir el código la columna sobrante no rompe nada pero queda huérfana.
