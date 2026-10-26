# Feature: fecha límite (solo día) en las tareas

## Encargo y criterios de aceptación

Issue [#5](https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5): «Quiero añadir fecha límite a las tasks». Cuerpo: «Vale con que sea simplemente un día. Sin hora».

Requisitos explícitos:

- Una tarea puede tener una fecha límite.
- La fecha límite es un día, sin hora.

Inferencias (decididas en este plan):

- La fecha límite es **opcional**: las tareas existentes y las nuevas sin fecha siguen siendo válidas.
- Se puede indicar al crear la tarea y añadir, cambiar o quitar al editarla; si solo se pudiera fijar al crear, las tareas existentes nunca podrían tener fecha.
- Se admiten fechas pasadas (registrar algo ya vencido es legítimo). No hay resaltado de vencidas ni ordenación por fecha.

Criterios de aceptación:

1. El formulario de creación de `/` tiene un campo de fecha opcional (`<input type="date" name="due_date">`). Si se rellena, la tarea se crea con ese día; si se deja vacío, se crea sin fecha límite.
2. En la lista, una tarea con fecha límite muestra «Fecha límite: DD/MM/AAAA» (con `<time datetime="AAAA-MM-DD">`) debajo del título. Una tarea sin fecha no muestra nada adicional.
3. `/tasks/{id}/edit` muestra el campo de fecha precargado con la fecha actual (o vacío). Al guardar se persiste el nuevo día; si se vacía el campo, la tarea queda sin fecha límite. `completed`, `id` y `created_at` siguen sin poder modificarse desde el formulario.
4. Un valor que no sea un día válido en formato `AAAA-MM-DD` (por ejemplo `2026-02-30`, `15/11/2026` o `2026-11-15T10:00`) responde 422, muestra «La fecha límite debe ser un día válido.» junto al campo y no crea ni modifica la tarea. Si además el título es inválido, se muestran ambos errores.
5. Si la creación falla por el título, el formulario re-renderizado conserva la fecha válida introducida.
6. En base de datos la columna es `DATE` nullable; no se guarda ninguna hora.

## Comportamiento actual y propuesta

Actual (rama creada desde `origin/main`):

- `migrations/20260901000000_create_tasks.up.fizz` crea `tasks` con `id`, `title`, `completed` y timestamps. No hay fecha límite.
- `models/task.go` define `Task` (`ID`, `Title`, `Completed`, `CreatedAt`, `UpdatedAt`) y `Validate` solo comprueba el título.
- `actions/tasks.go`: `TasksCreate` hace `c.Bind(task)` + `tx.ValidateAndCreate`; `TasksUpdate` hace `Bind` a un struct `input` con solo `Title` (lista blanca) + `tx.ValidateAndUpdate`. En error ambas re-renderizan con 422 y `errors`.
- Templates: `templates/tasks/index.plush.html` (lista + formulario de creación) y `templates/tasks/edit.plush.html`. Ambos marcan el input de título como inválido con `errors.HasAny()` y pintan los errores de `errors.Get("title")`.
- `public/assets/app.js` alterna el estado completado en cliente y lee `.task-title` para el `aria-label`; no se toca.

Propuesta: columna `due_date DATE NULL`, campo `DueDate *time.Time` en el modelo, parseo explícito y estricto del día `AAAA-MM-DD` en el modelo (`models.ParseDueDate`), un helper en acciones que asigna la fecha o añade el error de validación `due_date`, y campos `type="date"` en ambos formularios. Los errores de título y fecha se muestran por campo.

Decisiones técnicas:

- **`*time.Time` y no `nulls.Time`**: evita tocar `go.mod` (`github.com/gobuffalo/nulls` solo es dependencia indirecta). Se comprobó con un spike contra PostgreSQL 17 y pop v6.4.1 que una columna `date` nullable se escribe y lee correctamente como `*time.Time` (día → `2026-10-30 00:00:00 UTC`; `nil` ↔ `NULL`; poner `nil` y `Update` la vacía).
- **Parseo explícito, no `c.Bind`**: el binder de Buffalo solo registra decoders para `time.Time` y su `nulls.Time` interno, y acepta también formatos con hora (`2006-01-02T15:04`, RFC3339…), lo que contradice «sin hora»; además un error de binding acabaría en 500. Por eso `DueDate` lleva `form:"-"` y se parsea con `time.Parse("2006-01-02", …)`, que rechaza días imposibles y cualquier hora.
- **`Validate` + `Create`/`Update` en lugar de `ValidateAndCreate`/`ValidateAndUpdate`**: hay que juntar en un mismo `*validate.Errors` los errores del modelo y el de la fecha antes de persistir; `ValidateAndCreate` persistiría aunque la fecha fuera inválida. Para este modelo (sin `ValidateCreate`/`ValidateUpdate` ni asociaciones) es equivalente.
- **Zona horaria**: la fecha se parsea en UTC y se formatea sin convertir a hora local (`Format` directo sobre el valor leído). Nunca usar `.Local()`.

## Alcance

Incluido:

- Migración nueva que añade `tasks.due_date` (`date`, nullable) y su `down`.
- Modelo: campo `DueDate`, constante de formato, error de validación, `ParseDueDate` y métodos de presentación `HasDueDate`, `DueDateValue`, `DueDateLabel`.
- Acciones `TasksCreate` y `TasksUpdate`, más el helper `assignDueDate`.
- Templates `tasks/index.plush.html` (formulario de creación y fila de la lista) y `tasks/edit.plush.html`.
- CSS para el input de fecha, el contenedor del título y la línea de fecha; bump de `app.css?v=3` → `?v=4` en el layout.
- Tests de modelo y de acciones descritos abajo.

Excluido:

- Resaltado de tareas vencidas o próximas, ordenación o filtros por fecha.
- Hora, recordatorios, notificaciones o zona horaria del usuario.
- Cambios en `public/assets/app.js`, rutas (`actions/app.go`), seeds (`migrations/20260901000001_seed_tasks.*.sql`), `fixtures/tasks.toml`, `go.mod`/`go.sum`, CI, Docker.
- Cambios en la base de datos de desarrollo `todo_development` (ver Riesgos).

## Diseño por capas

### Persistencia

`migrations/20261026160000_add_due_date_to_tasks.up.fizz`:

```
add_column("tasks", "due_date", "date", {"null": true})
```

`migrations/20261026160000_add_due_date_to_tasks.down.fizz`:

```
drop_column("tasks", "due_date")
```

El traductor Postgres de fizz pasa `date` tal cual (`DATE`). `migrations/embed.go` ya incluye `*.fizz`, y `models.Migrate()` la aplica al arrancar la app.

### Modelo (`models/task.go`)

- Importar `errors`.
- Constantes/variables junto a `TitleMaxLength`:

  ```go
  // DueDateLayout es el formato de día, sin hora, con el que se recibe la fecha límite en formularios.
  const DueDateLayout = "2006-01-02"

  // ErrInvalidDueDate indica que la fecha límite recibida no es un día válido en formato DueDateLayout.
  var ErrInvalidDueDate = errors.New("La fecha límite debe ser un día válido.")
  ```

  (El mensaje es de cara al usuario, en español, igual que los de `Validate`; si `go vet`/staticcheck no lo marca, se deja así. `go vet` no comprueba mayúsculas en mensajes de error.)

- Campo nuevo en `Task`, entre `Completed` y `CreatedAt`, alineado por `gofmt`:

  ```go
  DueDate   *time.Time `json:"due_date" db:"due_date" form:"-"`
  ```

- Funciones y métodos (receptor **por valor** en los métodos de presentación, para que Plush pueda llamarlos sobre los `models.Task` del bucle `for (t) in tasks`):

  ```go
  // ParseDueDate convierte un día en formato AAAA-MM-DD en fecha límite.
  // Un valor vacío o con solo espacios significa que la tarea no tiene fecha límite.
  func ParseDueDate(value string) (*time.Time, error) {
  	value = strings.TrimSpace(value)
  	if value == "" {
  		return nil, nil
  	}
  	dueDate, err := time.Parse(DueDateLayout, value)
  	if err != nil {
  		return nil, ErrInvalidDueDate
  	}
  	return &dueDate, nil
  }

  // HasDueDate indica si la tarea tiene fecha límite.
  func (t Task) HasDueDate() bool {
  	return t.DueDate != nil
  }

  // DueDateValue devuelve la fecha límite en formato AAAA-MM-DD, o una cadena vacía si no tiene.
  func (t Task) DueDateValue() string {
  	if t.DueDate == nil {
  		return ""
  	}
  	return t.DueDate.Format(DueDateLayout)
  }

  // DueDateLabel devuelve la fecha límite en formato DD/MM/AAAA, o una cadena vacía si no tiene.
  func (t Task) DueDateLabel() string {
  	if t.DueDate == nil {
  		return ""
  	}
  	return t.DueDate.Format("02/01/2006")
  }
  ```

- `Validate` no cambia.

### Acciones (`actions/tasks.go`)

- Helper nuevo al final del archivo:

  ```go
  // assignDueDate asigna a la tarea la fecha límite del formulario o anota un error si no es un día válido.
  func assignDueDate(task *models.Task, value string, verrs *validate.Errors) {
  	dueDate, err := models.ParseDueDate(value)
  	if err != nil {
  		verrs.Add("due_date", err.Error())
  		return
  	}
  	task.DueDate = dueDate
  }
  ```

  Con fecha inválida, `task.DueDate` conserva su valor previo (en edición, la fecha guardada; en creación, `nil`).

- `TasksCreate`: tras `task.Title = strings.TrimSpace(task.Title)`, sustituir `tx.ValidateAndCreate(task)` por:

  ```go
  verrs, err := task.Validate(tx)
  if err != nil {
  	return err
  }
  assignDueDate(task, c.Param("due_date"), verrs)

  if verrs.HasAny() {
  	// bloque de re-render 422 actual, sin cambios
  }

  if err := tx.Create(task); err != nil {
  	return err
  }
  ```

  `c.Param` incluye los valores del formulario (Buffalo hace `req.ParseForm()` y los añade a los params). `c.Bind(task)` ignora `due_date` por `form:"-"`.

- `TasksUpdate`:
  - Comentario: `// TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.`
  - Añadir al struct `input`: `DueDate string \`form:"due_date" json:"due_date"\`` (gofmt alinea las etiquetas).
  - Tras `task.Title = strings.TrimSpace(input.Title)`, sustituir `tx.ValidateAndUpdate(task)` por `task.Validate(tx)` + `assignDueDate(task, input.DueDate, verrs)`; si `verrs.HasAny()`, el re-render 422 actual; si no, `if err := tx.Update(task); err != nil { return err }`, flash y redirect como ahora.
  - Un `PUT` sin `due_date` deja la tarea sin fecha límite: el formulario de edición siempre envía el campo, así que ausente y vacío significan lo mismo.

- `TasksIndex`, `TasksEdit`, `TasksToggle`, `TasksDestroy` y rutas: sin cambios.

### Renderizado

`templates/tasks/index.plush.html`:

- Fila de la lista: envolver el título en un contenedor y añadir la fecha debajo. Sustituir `<span class="task-title"><%= t.Title %></span>` por:

  ```html
  <div class="task-main">
    <span class="task-title"><%= t.Title %></span>
    <%= if (t.HasDueDate()) { %>
      <span class="task-due-date">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></span>
    <% } %>
  </div>
  ```

  `.task-title` sigue existiendo con solo el título (lo usa `app.js` para el `aria-label`) y `.task-actions` sigue siendo hijo directo del `<li>` (lo usa `app.js` en `insertBefore`).

- Formulario de creación, dentro de `.new-task-row`, entre el input de título y el botón «Añadir»:

  ```html
  <label for="task-due-date" class="sr-only">Fecha límite (opcional)</label>
  <input
    id="task-due-date"
    type="date"
    name="due_date"
    value="<%= task.DueDateValue() %>"
    class="due-date-input"
    title="Fecha límite (opcional)"
    <%= if (len(errors.Get("due_date")) > 0) { %>aria-invalid="true" aria-describedby="due-date-error"<% } %>
  >
  ```

- Input de título: cambiar la condición `errors.HasAny()` por `len(errors.Get("title")) > 0` (mantiene `autofocus aria-invalid="true" aria-describedby="title-error"`).
- Errores: sustituir el bloque `<%= if (errors.HasAny()) { %> … <% } %>` por dos bloques independientes:

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

- Input de título: misma sustitución de condición (`len(errors.Get("title")) > 0` → `aria-invalid="true" aria-describedby="title-error"`; si no, `aria-describedby="title-help"`).
- `#title-help`: texto `Hasta 200 caracteres.` (la frase «Los demás datos de la tarea no cambian.» deja de ser cierta).
- Bloque de error de título: condición `len(errors.Get("title")) > 0`.
- Después del bloque de error de título y antes de `.edit-task-actions`:

  ```html
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
  ```

`templates/application.plush.html`: `/assets/app.css?v=3` → `/assets/app.css?v=4`.

### CSS (`public/assets/app.css`)

- Reglas de la fila: mover `align-self: center;`, `min-width: 0;` y `padding: 0.65rem 0;` de `.task-title` a una regla nueva `.task-main` (situada justo antes de `.task-title`); `.task-title` queda con `display: block; overflow-wrap: anywhere; line-height: 1.55;`.
- Regla nueva tras `.task-title`:

  ```css
  .task-due-date {
    display: block;
    margin-top: 0.1rem;
    color: var(--color-muted);
    font-size: 0.8rem;
  }
  ```

- Ampliar el selector `input[type="text"]` a `input[type="text"],\ninput[type="date"]` para compartir estilos.
- Tras `.new-task-row input { flex: 1; }`:

  ```css
  .new-task-row .due-date-input {
    flex: 0 0 auto;
    width: auto;
  }
  ```

  (Especificidad 0,2,0: gana a `.new-task-row input` y a la regla de `flex-basis` del media query de 380px.)
- Tras `.edit-task label { … }`:

  ```css
  .edit-task label[for="task-due-date"] {
    margin-top: 1.25rem;
  }
  ```

## Archivos relevantes

| Ruta | Responsabilidad | Cambio |
| --- | --- | --- |
| `migrations/20261026160000_add_due_date_to_tasks.up.fizz` | Añade `tasks.due_date DATE NULL` | Nuevo |
| `migrations/20261026160000_add_due_date_to_tasks.down.fizz` | Revierte la columna | Nuevo |
| `models/task.go` | Modelo `Task`, validación y presentación de la fecha | Modificado |
| `models/task_test.go` | Tests de modelo | Modificado |
| `actions/tasks.go` | Crear/editar tareas con fecha límite | Modificado |
| `actions/tasks_test.go` | Tests HTTP de las acciones | Modificado |
| `templates/tasks/index.plush.html` | Lista y formulario de creación | Modificado |
| `templates/tasks/edit.plush.html` | Formulario de edición | Modificado |
| `templates/application.plush.html` | Layout; versión del CSS | Modificado |
| `public/assets/app.css` | Estilos del input de fecha y de la línea de fecha | Modificado |
| `migrations/embed.go`, `cmd/app/main.go`, `actions/app.go`, `public/assets/app.js` | Contexto (embed de migraciones, migración al arrancar, rutas, JS de toggle) | Sin cambios |

## Implementación paso a paso

1. Comprobar `git status --short --branch` (rama `feature/issue-5-plazo-tareas`, solo este plan sin commitear).
2. Crear las dos migraciones con el contenido exacto de «Persistencia».
3. Modificar `models/task.go` según «Modelo».
4. Modificar `actions/tasks.go` según «Acciones».
5. Modificar `templates/tasks/index.plush.html`, `templates/tasks/edit.plush.html` y `templates/application.plush.html` según «Renderizado».
6. Modificar `public/assets/app.css` según «CSS».
7. Añadir/actualizar los tests de «Tests y validación».
8. Preparar la base de datos de test y ejecutar la validación (sección siguiente).
9. Revisar `git diff --check`, `git diff` y `git status --short`: solo deben aparecer este plan, las dos migraciones nuevas y los archivos de la tabla marcados como modificados.

## Tests y validación

### Tests de modelo (`models/task_test.go`)

Añadir (métodos de `ModelSuite`, importando `time`):

- `Test_Task_DueDate_Persists`: crear `Task{Title: "Pagar el seguro", DueDate: &d}` con `d := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)`; `Find` en una tarea nueva; `HasDueDate()` true y `DueDateValue() == "2026-11-15"`.
- `Test_Task_DueDate_Optional`: crear sin fecha; tras `Find`, `DueDate` es `nil`, `HasDueDate()` false, `DueDateValue()` y `DueDateLabel()` vacíos.
- `Test_Task_DueDate_Clear`: crear con fecha, poner `DueDate = nil`, `Update`, `Find` → `nil`.
- `Test_ParseDueDate`: tabla de casos — `""` y `"   "` → `nil, nil`; `"2026-11-15"` → `DueDateValue() == "2026-11-15"` y `DueDateLabel() == "15/11/2026"` (asignando el resultado a una `Task`); `"2026-02-30"`, `"15/11/2026"`, `"2026-11-15T10:00"`, `"mañana"` → `ErrInvalidDueDate` (`ms.ErrorIs`).

### Tests de acciones (`actions/tasks_test.go`)

Importar `time`. Añadir un helper `func (as *ActionSuite) taskWithDueDate(title string) *models.Task` que cree la tarea con fecha `2026-11-15` (UTC) mediante `as.DB.ValidateAndCreate` y la recargue.

- `Test_TasksCreate_Valid` (existente): añadir `as.Nil(task.DueDate)`.
- `Test_TasksCreate_WithDueDate`: `Post(url.Values{"title": {"Con fecha"}, "due_date": {"2026-11-15"}})` → 303; la tarea guardada tiene `DueDateValue() == "2026-11-15"`.
- `Test_TasksCreate_InvalidDueDate`: `due_date` `"2026-02-30"` con título válido → 422, cuerpo contiene «La fecha límite debe ser un día válido.» y `aria-describedby="due-date-error"`; no contiene `id="title-error"`; `Count == 0`.
- `Test_TasksCreate_EmptyTitle_KeepsDueDate`: título vacío y `due_date` `"2026-11-15"` → 422, contiene «El título es obligatorio.» y `value="2026-11-15"`; `Count == 0`.
- `Test_TasksIndex_ShowsDueDate`: `taskWithDueDate("Con fecha")` y otra tarea sin fecha → `GET /` contiene `Fecha límite: <time datetime="2026-11-15">15/11/2026</time>` y aparece una sola vez `class="task-due-date"` (`strings.Count`).
- `Test_TasksIndex_ListsTasks` (existente, fixture sin fechas): añadir `as.NotContains(body, `class="task-due-date"`)` (no basta con `task-due-date`: es también el `id` del input de fecha del formulario).
- `Test_TasksEdit_PrefillsDueDate`: `taskWithDueDate` → `GET /tasks/{id}/edit` contiene `name="due_date"` y `value="2026-11-15"`.
- `Test_TasksUpdate_SetsDueDate`: `taskForEditing(false)` → `PUT` con `title` y `due_date` `"2026-12-01"` → 303; recarga con `DueDateValue() == "2026-12-01"`.
- `Test_TasksUpdate_ClearsDueDate`: `taskWithDueDate` → `PUT` con `title` y `due_date` `""` → 303; recarga con `DueDate` `nil`.
- `Test_TasksUpdate_InvalidDueDate`: `taskWithDueDate` → `PUT` con `title` `"Otro título"` y `due_date` `"15/11/2026"` → 422 con el mensaje de fecha; recarga: título y fecha originales intactos.
- `Test_TasksUpdate_EmptyTitle` (existente): sigue verificando `aria-describedby="title-error"`; no cambiar.

Los demás tests existentes deben pasar sin cambios.

### Preparación de la base de datos de test

Los tests de aplicación necesitan PostgreSQL (`docker compose up -d db` si no está activo) y la BD `todo_test` migrada **con las migraciones de esta rama**. En este equipo `todo_test` contiene restos de otras ramas (una columna `due_date` y la versión `20261026120000` que no existen aquí), de modo que `soda migrate` fallaría con «column already exists». Recrear solo la BD de test:

```sh
SODA=$(command -v soda || echo ~/go/bin/soda)   # si no existe: go install github.com/gobuffalo/pop/v6/soda@v6.4.1
$SODA drop -e test && $SODA create -e test && $SODA migrate -e test
```

No tocar `todo_development`.

### Comandos de validación

```sh
gofmt -l .                          # sin salida
go vet ./...
GO_ENV=test go test -p 1 ./...      # GO_ENV=test es obligatorio: sin él models.DB apunta a todo_development
go build ./...
```

Comprobación adicional de la migración (sobre `todo_test`, tras los tests): `$SODA migrate down -e test` seguido de `$SODA migrate -e test`; ambos sin error y `\d tasks` muestra `due_date | date` nullable al final.

No se exige verificación manual en navegador: los tests de acciones cubren el HTML renderizado, y arrancar la app migraría `todo_development`, que comparte estado con otras ramas (ver Riesgos).

## Riesgos y recuperación

- **Estado compartido de las BD locales**: `todo_test` y `todo_development` tienen migraciones de otras ramas (`due_date` en test, `priority` en desarrollo). Mitigación: recrear solo `todo_test` como se indica; no arrancar la app contra `todo_development`. Si `soda drop` falla por conexiones abiertas, detenerse e informar en vez de forzar.
- **Plush y métodos del modelo**: los métodos de presentación usan receptor por valor para funcionar sobre `models.Task` (bucle) y `*models.Task` (`task`). Si Plush no resolviera alguno, los tests de renderizado fallarán; no sustituir por lógica en el template sin revisar el plan.
- **Zona horaria**: la fecha se lee como medianoche UTC (verificado en el spike). Formatear sin `.Local()`; de lo contrario, en zonas con desfase negativo se mostraría el día anterior.
- **Navegadores sin `input type="date"`**: muestran un campo de texto; el servidor solo acepta `AAAA-MM-DD` y devuelve el error de validación. Aceptado.
- **Cambio de contrato de `TasksUpdate`**: un `PUT` sin `due_date` borra la fecha. El único cliente es el formulario de edición, que siempre envía el campo.
- **Recuperación**: el cambio es aditivo. Para revertir, `soda migrate down` elimina la columna y se revierte el commit; los datos de fecha límite se perderían.
