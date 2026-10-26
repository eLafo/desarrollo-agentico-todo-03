# Feature: fecha límite (solo día) en las tareas

## Encargo y criterios de aceptación

Origen: issue #5 (https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5).

- Título: «Quiero añadir fecha límite a las tasks».
- Cuerpo: «Vale con que sea simplemente un día. Sin hora».

Requisitos explícitos:

- R1. Una tarea puede tener una fecha límite.
- R2. La fecha límite es un día de calendario, sin hora ni zona horaria.

Inferencias (no pedidas literalmente, necesarias para que la funcionalidad sea usable y coherente con la aplicación actual):

- I1. La fecha límite es opcional: las tareas existentes y las nuevas sin fecha siguen funcionando igual.
- I2. Se puede indicar al crear la tarea, y cambiar o quitar desde la pantalla de edición.
- I3. La lista muestra la fecha límite de las tareas que la tienen.
- I4. Se aceptan fechas pasadas (si no, una tarea vencida no podría editarse sin quitarle la fecha).
- I5. Un valor que no sea un día válido con formato `AAAA-MM-DD` produce un error de validación (HTTP 422) y no guarda nada.

Criterios de aceptación:

1. El formulario de creación de `/` muestra un campo de fecha (`<input type="date" name="due_date">`) con etiqueta visible «Fecha límite (opcional)».
2. `POST /tasks` con `title` válido y `due_date=2026-11-15` crea la tarea con fecha límite 15/11/2026 y redirige a `/` (303).
3. `POST /tasks` sin `due_date` o con `due_date` vacío crea la tarea sin fecha límite (comportamiento actual intacto).
4. `POST /tasks` con `due_date` inválido (`2026-02-30`, `15/11/2026`, `2026-11-15T10:00`) responde 422, muestra «La fecha límite no es válida.» y no crea la tarea.
5. Si el título es inválido y la fecha es válida, la respuesta 422 conserva la fecha introducida en el campo.
6. En la lista, una tarea con fecha muestra `Fecha límite: <time datetime="2026-11-15">15/11/2026</time>`; una tarea sin fecha no muestra nada adicional.
7. `GET /tasks/{id}/edit` muestra el campo de fecha precargado con el valor `AAAA-MM-DD` de la tarea (vacío si no tiene).
8. `PUT /tasks/{id}` con `due_date` nuevo lo guarda; con `due_date` vacío quita la fecha; con `due_date` inválido responde 422 con el mensaje y no cambia ni título ni fecha.
9. Completar o reabrir una tarea (`PUT /tasks/{id}/toggle`) conserva su fecha límite.
10. Los errores de título siguen marcando solo el campo título (`aria-describedby="title-error"`), y los de fecha solo el campo fecha (`aria-describedby="due-date-error"`).

## Comportamiento actual y propuesta

Actual:

- La tabla `tasks` (`migrations/20260901000000_create_tasks.up.fizz`) solo tiene `id`, `title`, `completed`, `created_at` y `updated_at`.
- `models.Task` (`models/task.go`) valida únicamente el título.
- `TasksCreate` (`actions/tasks.go`) hace `c.Bind(task)` directamente sobre `models.Task`; `TasksUpdate` hace bind de un struct anónimo con solo `Title` para no tocar otros campos.
- Las plantillas `templates/tasks/index.plush.html` y `templates/tasks/edit.plush.html` solo tienen campo de título y muestran errores con `errors.HasAny()` asociados siempre al título.
- No existe ningún concepto de fecha límite.

Propuesta (cambio vertical mínimo):

- Columna nueva `due_date` de tipo `date`, nullable, mediante una migración fizz nueva.
- Campo `DueDate nulls.Time` en `models.Task` con `form:"-"`; el valor del formulario se interpreta de forma explícita con `models.ParseDueDate` para poder devolver 422 (con el binder de Buffalo un valor inválido acabaría en error 500).
- Las acciones de crear y actualizar leen un struct común `taskForm{Title, DueDate string}`, validan título (modelo) y fecha (parseo) a la vez y solo persisten si no hay errores.
- Las plantillas añaden el campo de fecha, muestran la fecha en la lista y separan los errores por campo.
- CSS mínimo para el nuevo bloque y la línea de fecha; se sube la versión de caché de `app.css`.

## Alcance

Incluido:

- Migración `add_column` de `tasks.due_date` (`date`, `NULL`) y su `down`.
- Modelo: campo, constantes, `ParseDueDate`, `DueDateValue`, `DueDateLabel`.
- Acciones `TasksCreate` y `TasksUpdate` (y su comentario de cabecera).
- Plantillas de lista/creación y edición.
- `public/assets/app.css` y el parámetro de caché en `templates/application.plush.html`.
- `go.mod`: `github.com/gobuffalo/nulls` pasa de dependencia indirecta a directa (misma versión v0.4.2; `go.sum` no debe cambiar).
- Tests de modelo y de acciones.

Excluido:

- Hora, zona horaria o recordatorios.
- Resaltar tareas vencidas o próximas a vencer, ordenar o filtrar por fecha.
- Cambios en `public/assets/app.js`, en el toggle o en `TasksToggle`/`TasksDestroy`.
- Cambios en la migración de seed (`20260901000001_seed_tasks.*.sql`) o en `fixtures/tasks.toml`.
- API JSON nueva (el campo solo aparece en el `json` del struct por coherencia con los demás).
- Localización de fechas más allá del formato fijo `DD/MM/AAAA`.

## Diseño por capas

### Persistencia y migraciones

Nueva migración fizz, con marca de tiempo posterior a las existentes:

- `migrations/20261026000000_add_due_date_to_tasks.up.fizz`:
  ```
  add_column("tasks", "due_date", "date", {"null": true})
  ```
- `migrations/20261026000000_add_due_date_to_tasks.down.fizz`:
  ```
  drop_column("tasks", "due_date")
  ```

El traductor fizz de PostgreSQL devuelve el tipo tal cual en su rama `default`, así que `"date"` genera una columna `DATE`. `migrations/embed.go` ya embebe `*.fizz`, y `cmd/app/main.go` ejecuta `models.Migrate()` al arrancar, así que no hace falta tocar nada más para que se aplique. Las filas existentes quedan con `due_date = NULL`.

### Modelo (`models/task.go`)

Añadir import `github.com/gobuffalo/nulls` y:

```go
// DueDateLayout es el formato de día, sin hora, de la fecha límite en formularios.
const DueDateLayout = "2006-01-02"

// DueDateDisplayLayout es el formato con el que se muestra la fecha límite.
const DueDateDisplayLayout = "02/01/2006"

// DueDateInvalidMessage es el error que se muestra cuando la fecha límite no es un día válido.
const DueDateInvalidMessage = "La fecha límite no es válida."
```

Campo nuevo en `Task`, entre `Completed` y `CreatedAt`:

```go
DueDate   nulls.Time `json:"due_date" db:"due_date" form:"-"`
```

(`gofmt` realineará las etiquetas del struct.)

Funciones y métodos (receptor por valor, para que Plush pueda invocarlos tanto sobre `models.Task` en el bucle de la lista como sobre `*models.Task`):

```go
// ParseDueDate interpreta una fecha límite con formato AAAA-MM-DD.
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

// DueDateValue devuelve la fecha límite con el formato del campo de formulario, o "" si no tiene.
func (t Task) DueDateValue() string {
	if !t.DueDate.Valid {
		return ""
	}
	return t.DueDate.Time.Format(DueDateLayout)
}

// DueDateLabel devuelve la fecha límite con formato DD/MM/AAAA, o "" si no tiene.
func (t Task) DueDateLabel() string {
	if !t.DueDate.Valid {
		return ""
	}
	return t.DueDate.Time.Format(DueDateDisplayLayout)
}
```

`time.Parse` con `DueDateLayout` devuelve medianoche UTC y rechaza días inexistentes (`2026-02-30`) y cualquier texto con hora. `Validate` no cambia: la fecha ya llega parseada o se informa el error desde la acción.

`go.mod`: mover `github.com/gobuffalo/nulls v0.4.2` del bloque `// indirect` al bloque de dependencias directas (sin `// indirect`). Ejecutar `go mod tidy` y comprobar que el único cambio en `go.mod` es ese y que `go.sum` no cambia.

### Acciones (`actions/tasks.go`)

Struct común a nivel de paquete, que sustituye al struct anónimo de `TasksUpdate`:

```go
// taskForm recoge los campos editables del formulario de tareas.
type taskForm struct {
	Title   string `form:"title" json:"title"`
	DueDate string `form:"due_date" json:"due_date"`
}
```

`TasksCreate`:

```go
	input := taskForm{}
	if err := c.Bind(&input); err != nil {
		return err
	}
	task := &models.Task{Title: strings.TrimSpace(input.Title)}
	dueDate, dueDateErr := models.ParseDueDate(input.DueDate)
	if dueDateErr == nil {
		task.DueDate = dueDate
	}

	verrs, err := task.Validate(tx)
	if err != nil {
		return err
	}
	if dueDateErr != nil {
		verrs.Add("due_date", models.DueDateInvalidMessage)
	}

	if verrs.HasAny() {
		// bloque actual sin cambios: recarga tasks, c.Set("task", task), c.Set("errors", verrs), 422
	}

	if err := tx.Create(task); err != nil {
		return err
	}
	// flash y redirect actuales sin cambios
```

`TasksUpdate` (después del `tx.Find` actual):

```go
	input := taskForm{}
	if err := c.Bind(&input); err != nil {
		return err
	}
	task.Title = strings.TrimSpace(input.Title)
	dueDate, dueDateErr := models.ParseDueDate(input.DueDate)
	if dueDateErr == nil {
		task.DueDate = dueDate
	}

	verrs, err := task.Validate(tx)
	if err != nil {
		return err
	}
	if dueDateErr != nil {
		verrs.Add("due_date", models.DueDateInvalidMessage)
	}
	if verrs.HasAny() {
		// bloque actual sin cambios: c.Set("task", task), c.Set("errors", verrs), 422 con edit.plush.html
	}

	if err := tx.Update(task); err != nil {
		return err
	}
	// flash y redirect actuales sin cambios
```

Notas:

- `task.Validate(tx)` + `tx.Create`/`tx.Update` equivale a `ValidateAndCreate`/`ValidateAndUpdate` porque `Task` no define `ValidateCreate` ni `ValidateUpdate`; el cambio es necesario para no persistir cuando solo falla la fecha.
- Si la fecha es inválida, `task.DueDate` conserva su valor previo (vacío al crear, el guardado al editar), de modo que el formulario re-renderizado no muestra un valor imposible.
- El formulario de edición envía siempre `due_date`; un valor vacío (o ausente) significa «sin fecha límite».
- Cambiar el comentario de `TasksUpdate` a: `// TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.`
- `id`, `completed`, `created_at` y demás campos enviados en el formulario siguen ignorándose porque `taskForm` no los incluye.
- Rutas: sin cambios.

### Renderizado (`templates/tasks/index.plush.html`)

Lista: sustituir `<span class="task-title"><%= t.Title %></span>` por un contenedor que mantenga la rejilla de 3 columnas del `<li>` y deje intacto `.task-title` (el JS del toggle lee su `textContent` para el `aria-label`):

```html
          <div class="task-main">
            <span class="task-title"><%= t.Title %></span>
            <%= if (t.DueDate.Valid) { %>
              <span class="task-due">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></span>
            <% } %>
          </div>
```

El `<span class="task-status">` y `.task-actions` quedan donde están (el JS inserta el estado antes de `.task-actions`).

Formulario de creación:

- En el input de título, cambiar las dos condiciones `errors.HasAny()` por `len(errors.Get("title")) > 0` (atributos `autofocus aria-invalid aria-describedby="title-error"` y el bloque `<p id="title-error">`).
- Después de `</div>` de `.new-task-row` y antes del bloque de error de título, añadir:

```html
  <div class="new-task-due">
    <label for="task-due-date">Fecha límite <span class="field-optional">(opcional)</span></label>
    <input
      id="task-due-date"
      type="date"
      name="due_date"
      value="<%= task.DueDateValue() %>"
      <%= if (len(errors.Get("due_date")) > 0) { %>autofocus aria-invalid="true" aria-describedby="due-date-error"<% } %>
    >
  </div>
```

- Después del bloque de error de título, añadir:

```html
  <%= if (len(errors.Get("due_date")) > 0) { %>
    <p id="due-date-error" class="field-error" role="alert">
      <%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %>
    </p>
  <% } %>
```

### Renderizado (`templates/tasks/edit.plush.html`)

- En el input de título, cambiar `errors.HasAny()` por `len(errors.Get("title")) > 0` en los atributos y en el bloque de error (el texto de ayuda `title-help` no cambia).
- Antes de `<div class="edit-task-actions">`, añadir:

```html
  <div class="field-group">
    <label for="task-due-date">Fecha límite <span class="field-optional">(opcional)</span></label>
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

### CSS (`public/assets/app.css`) y caché

- Sustituir la regla `.task-title { align-self: center; min-width: 0; padding: 0.65rem 0; overflow-wrap: anywhere; line-height: 1.55; }` por:

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
  margin-top: 0.1rem;
  color: var(--color-muted);
  font-size: 0.8rem;
}
```

- Ampliar el selector `input[type="text"]` a `input[type="text"], input[type="date"]` y, justo después, añadir:

```css
input[type="date"] {
  width: auto;
}
```

- Añadir el bloque del formulario de creación y del grupo de edición (junto a `.new-task-row`/`.edit-task label`):

```css
.new-task-due {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.6rem;
  margin-top: 0.5rem;
  margin-left: calc(44px + 0.6rem);
  color: var(--color-muted);
  font-size: 0.85rem;
}

.field-group {
  margin-top: 1.25rem;
}

.field-optional {
  color: var(--color-muted);
  font-weight: 400;
}
```

- En `@media (max-width: 760px)` añadir `.new-task-due { margin-left: calc(36px + 0.4rem); }`.
- `.new-task-row input { flex: 1; }` no afecta al campo de fecha porque está fuera de `.new-task-row`.
- En `templates/application.plush.html` cambiar `/assets/app.css?v=3` por `/assets/app.css?v=4` para invalidar la caché del CSS.

## Archivos relevantes

| Ruta | Responsabilidad |
| --- | --- |
| `migrations/20261026000000_add_due_date_to_tasks.up.fizz` | Nuevo: añade `tasks.due_date DATE NULL`. |
| `migrations/20261026000000_add_due_date_to_tasks.down.fizz` | Nuevo: elimina la columna. |
| `migrations/embed.go` | Embebe `*.fizz`; sin cambios. |
| `models/task.go` | Campo `DueDate`, constantes, `ParseDueDate`, `DueDateValue`, `DueDateLabel`. |
| `models/task_test.go` | Tests de persistencia, opcionalidad, parseo y formato. |
| `actions/tasks.go` | `taskForm`; crear y actualizar con fecha límite y error 422. |
| `actions/tasks_test.go` | Tests HTTP de creación, lista, edición, actualización y toggle. |
| `actions/app.go` | Rutas; sin cambios. |
| `templates/tasks/index.plush.html` | Fecha en la lista, campo en el formulario de creación, errores por campo. |
| `templates/tasks/edit.plush.html` | Campo de fecha precargado, ayuda y error. |
| `templates/application.plush.html` | Versión de caché de `app.css` (`v=4`). |
| `public/assets/app.css` | Estilos de `.task-main`, `.task-due`, `.new-task-due`, `.field-group`, `.field-optional`, `input[type="date"]`. |
| `public/assets/app.js` | Toggle; sin cambios (sigue usando `.task-title`, `.task-status`, `.task-actions`). |
| `go.mod` | `github.com/gobuffalo/nulls` pasa a dependencia directa. |
| `cmd/app/main.go` | Aplica migraciones al arrancar; sin cambios. |

## Implementación paso a paso

1. Crear las dos migraciones fizz indicadas.
2. Modificar `models/task.go` (import, constantes, campo, `ParseDueDate`, `DueDateValue`, `DueDateLabel`) y ajustar `go.mod` con `go mod tidy`; verificar con `git diff go.mod go.sum` que solo se mueve `nulls` a directa.
3. Modificar `actions/tasks.go` (`taskForm`, `TasksCreate`, `TasksUpdate`, comentario).
4. Modificar `templates/tasks/index.plush.html` y `templates/tasks/edit.plush.html`.
5. Modificar `public/assets/app.css` y subir `app.css?v=4` en `templates/application.plush.html`.
6. Añadir los tests de modelo y de acciones descritos abajo.
7. Preparar la base de datos de test y ejecutar las validaciones.

## Tests y validación

Tests de modelo (`models/task_test.go`):

- `Test_Task_DueDate_Optional`: crear tarea sin fecha; tras recargar, `DueDate.Valid` es `false` y `DueDateValue()` y `DueDateLabel()` devuelven `""`.
- `Test_Task_DueDate_PersistsDay`: `ParseDueDate("2026-11-15")`, crear, recargar con `Find`; `DueDateValue() == "2026-11-15"` y `DueDateLabel() == "15/11/2026"`.
- `Test_ParseDueDate` (puede ser método del suite o `func Test_ParseDueDate(t *testing.T)`):
  - `""` y `"   "` → sin error, `Valid == false`;
  - `" 2026-11-15 "` → válido, `2026-11-15`;
  - `"2026-02-30"`, `"15/11/2026"`, `"2026-11-15T10:00"`, `"mañana"` → error.

Tests de acciones (`actions/tasks_test.go`, usando `url.Values` en las peticiones con fecha):

- `Test_TasksCreate_Valid`: añadir `as.False(task.DueDate.Valid)`.
- `Test_TasksCreate_WithDueDate`: `title` + `due_date=2026-11-15` → 303 a `/`; la tarea guardada tiene `DueDateValue() == "2026-11-15"`.
- `Test_TasksCreate_InvalidDueDate`: `due_date=2026-02-30` → 422, cuerpo contiene `La fecha límite no es válida.` y `aria-describedby="due-date-error"`; no contiene `aria-describedby="title-error"`; `Count == 0`.
- `Test_TasksCreate_EmptyTitle_KeepsDueDate`: `title=""`, `due_date=2026-11-15` → 422, contiene `El título es obligatorio.` y `value="2026-11-15"`; `Count == 0`.
- `Test_TasksIndex_ShowsDueDate`: crear una tarea con fecha y otra sin; `GET /` contiene `datetime="2026-11-15"`, `15/11/2026` y `name="due_date"`, y el marcador `class="task-due"` aparece exactamente una vez (`as.Equal(1, strings.Count(body, "class=\"task-due\""))`).
- `Test_TasksEdit_PrefillsDueDate`: editar una tarea con fecha muestra `type="date"` y `value="2026-11-15"`.
- `Test_TasksUpdate_ChangesDueDate`: `PUT` con `title` y `due_date=2026-12-01` → 303; fecha guardada `2026-12-01`, título actualizado.
- `Test_TasksUpdate_ClearsDueDate`: tarea con fecha, `PUT` con `due_date=""` → 303; `DueDate.Valid == false`.
- `Test_TasksUpdate_InvalidDueDate`: tarea con fecha `2026-11-15`, `PUT` con `title=Otro título` y `due_date=no-es-fecha` → 422 con el mensaje; tras recargar, título `Título original` y fecha `2026-11-15`.
- `Test_TasksToggle_KeepsDueDate`: tarea con fecha, `PUT /tasks/{id}/toggle` → 303; sigue `Completed == true` y la fecha `2026-11-15`.
- Los tests existentes (`Test_TasksUpdate_EmptyTitle` con `aria-describedby="title-error"`, `Test_TasksUpdate_PersistsTitleOnly`, etc.) deben seguir pasando sin cambios.

Para crear tareas con fecha en los tests, ampliar el helper existente o crear la tarea directamente:

```go
dueDate, err := models.ParseDueDate("2026-11-15")
as.NoError(err)
task := &models.Task{Title: "Título original", DueDate: dueDate}
```

Preparación (PostgreSQL de `compose.yaml` en `127.0.0.1:5432`; el contenedor `db` debe estar sano):

```sh
docker compose up -d db
GO_ENV=test go run github.com/gobuffalo/pop/v6/soda@v6.4.1 create -e test   # si todo_test no existe
GO_ENV=test go run github.com/gobuffalo/pop/v6/soda@v6.4.1 migrate -e test
```

Validación (desde la raíz, con `GO_ENV=test` para los tests):

```sh
gofmt -l .                 # debe no listar nada
go vet ./...
GO_ENV=test go test -p 1 ./...
go build ./...
git diff go.sum            # debe estar vacío
```

Comprobar además el `down` de la migración: `GO_ENV=test go run github.com/gobuffalo/pop/v6/soda@v6.4.1 migrate down -e test` seguido de `migrate -e test` termina sin errores.

## Riesgos y recuperación

- Zona horaria al guardar o leer `date`: `ParseDueDate` produce medianoche UTC y solo se formatea la parte de día; `Test_Task_DueDate_PersistsDay` detectaría un desplazamiento de día. Si fallara, normalizar en `DueDateValue`/`DueDateLabel` con `t.DueDate.Time.UTC()` antes de formatear.
- Plush y métodos con receptor por valor: si Plush no resolviera `t.DueDateValue()` sobre el elemento del bucle, el test de lista fallaría con error de render; los métodos deben tener receptor por valor como se indica.
- Errores por campo: el cambio de `errors.HasAny()` a `len(errors.Get("title")) > 0` lo cubre `Test_TasksUpdate_EmptyTitle` (sigue esperando `aria-describedby="title-error"`) y `Test_TasksCreate_InvalidDueDate` (no debe marcar el título).
- Clientes que envíen `PUT /tasks/{id}` sin `due_date` quitarán la fecha. Las únicas personas cliente son los formularios propios, que siempre envían el campo; se acepta.
- Migración en entornos ya desplegados: `cmd/app/main.go` la aplica al arrancar; es aditiva (`NULL` permitido) y no reescribe datos. Recuperación: `soda migrate down` elimina la columna y revertir el commit restaura el código.
- `go.mod`: si `go mod tidy` introdujera cambios distintos de mover `nulls` a directa, revertirlos y editar `go.mod` a mano solo con ese movimiento.
- La rama sale de `origin/main`. La rama local `main` tiene un commit sin publicar (`429e9d6`, toggle de JS) que también añade tests al final de `actions/tasks_test.go` y toca `templates/application.plush.html` (versión de `app.js`); al integrar ambos puede haber un conflicto trivial en esos archivos.
- Diseño del formulario en pantallas estrechas: `.new-task-due` usa `flex-wrap`, así que etiqueta y campo pasan a dos líneas si no caben; revisar visualmente con la app arrancada si es posible (no bloquea la validación automática).
