# Feature: fecha límite (solo día) en las tareas

Origen: issue #5 — <https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5>, «Quiero añadir fecha límite a las tasks». Cuerpo: «Vale con que sea simplemente un día. Sin hora».

## Encargo y criterios de aceptación

### Requisitos explícitos

- Una tarea puede tener una fecha límite.
- La fecha límite es un día del calendario, sin hora.

### Inferencias (decididas en este plan)

- La fecha límite es **opcional**: las tareas existentes y las nuevas sin fecha siguen funcionando igual.
- Se puede fijar al crear la tarea y fijar, cambiar o quitar al editarla.
- Se muestra en la lista de tareas.
- Se admiten fechas pasadas (no se valida contra «hoy»): una tarea puede crearse ya vencida y una tarea cuya fecha pasó debe poder editarse sin error.

### Criterios de aceptación

1. El formulario «Añadir tarea» de `/` incluye un campo de fecha (`<input type="date" name="due_date">`) opcional, con etiqueta accesible «Fecha límite (opcional)».
2. Enviar `title=Algo&due_date=2026-11-15` a `POST /tasks` crea la tarea con fecha límite 2026-11-15 y redirige (303) a `/`.
3. Enviar `POST /tasks` sin `due_date` o con `due_date=` vacío crea la tarea sin fecha límite (NULL).
4. Enviar `due_date` con un valor que no sea un día `AAAA-MM-DD` válido (p. ej. `15/11/2026`, `2026-02-30`, `2026-11-15T10:00`) responde 422 con el mensaje «La fecha límite no es válida.» y no crea la tarea.
5. Si el título es inválido y la fecha es válida, la respuesta 422 vuelve a mostrar la fecha elegida en el campo (`value="2026-11-15"`).
6. En la lista, cada tarea con fecha límite muestra «Fecha límite: 15/11/2026» dentro de `<time datetime="2026-11-15">`; las tareas sin fecha no muestran nada nuevo.
7. `GET /tasks/{id}/edit` incluye un campo `due_date` precargado con la fecha actual de la tarea (vacío si no tiene).
8. `PUT /tasks/{id}` con `due_date` válido cambia la fecha; con `due_date` vacío la quita; con un valor inválido responde 422 con «La fecha límite no es válida.» sin modificar la tarea.
9. Completar o reabrir una tarea (`PUT /tasks/{id}/toggle`) conserva su fecha límite.
10. Las validaciones y protecciones existentes del título, del estado `completed`, de `id` y de `created_at` siguen pasando sus tests.

## Comportamiento actual y propuesta

### Actual

- Tabla `tasks` (`migrations/20260901000000_create_tasks.up.fizz`): `id`, `title` (VARCHAR 200), `completed`, `created_at`, `updated_at`. No hay fecha límite.
- `models.Task` (`models/task.go`) solo valida el título.
- `TasksCreate` (`actions/tasks.go`) hace `c.Bind(task)`, recorta el título y llama a `tx.ValidateAndCreate`; ante errores vuelve a pintar `tasks/index.plush.html` con 422.
- `TasksUpdate` enlaza solo `title` mediante un struct de entrada explícito y llama a `tx.ValidateAndUpdate`; ante errores pinta `tasks/edit.plush.html` con 422.
- `TasksToggle` invierte `Completed` y llama a `tx.Update(task)` sobre la tarea cargada.
- Las plantillas muestran el error del título siempre que `errors.HasAny()`, sin distinguir campos.

### Propuesta

Corte vertical mínimo siguiendo los patrones existentes:

1. Migración nueva que añade la columna `due_date DATE NULL`.
2. Campo `DueDate *time.Time` en `models.Task` (sin binding de formulario, como `Completed`), más una función `models.ParseDueDate` estricta con formato `2006-01-02` y tres métodos de presentación (`HasDueDate`, `DueDateValue`, `DueDateLabel`).
3. Las acciones de crear y actualizar interpretan explícitamente el parámetro `due_date` con un helper común `assignDueDate`; un valor inválido produce un error de validación en la clave `due_date` y no se toca la base de datos.
4. Plantillas: campo de fecha en los formularios de alta y edición, errores por campo y fecha visible en la lista.
5. CSS: estilos del campo de fecha y de la línea de fecha en la lista; se incrementa el parámetro de caché de `app.css`.

No se usa el binding automático de Buffalo para fechas porque acepta muchos formatos con hora (RFC3339, `2006-01-02T15:04`, …), lo que contradice «sin hora», y porque un formato inválido haría fallar `c.Bind` con un 500.

## Alcance

### Incluido

- Persistencia: migración `up`/`down` de la columna `due_date`.
- Modelo: campo, parser y métodos de presentación.
- Acciones `TasksCreate` y `TasksUpdate`; `TasksToggle` sin cambios de código (solo test de regresión).
- Plantillas `tasks/index.plush.html`, `tasks/edit.plush.html` y el parámetro de caché en `application.plush.html`.
- CSS en `public/assets/app.css`.
- Tests de modelo y de acciones.

### Excluido

- Resaltar tareas vencidas o próximas a vencer, ordenar o filtrar por fecha (requeriría decidir la zona horaria de «hoy»; no se pide).
- Fechas con hora, recordatorios o notificaciones.
- Cambios en `public/assets/app.js`: el script del toggle sigue funcionando con la nueva estructura (ver «Diseño por capas»). Tampoco se corrige aquí ningún otro defecto del script.
- Cambios en la migración de semillas (`20260901000001_seed_tasks.*`) y en `fixtures/tasks.toml`.
- Cambios en `go.mod`/`go.sum` (no se añade `gobuffalo/nulls`; se usa `*time.Time`).
- Renombrar tests existentes.

## Diseño por capas

### Persistencia

Archivos nuevos:

- `migrations/20261026180000_add_due_date_to_tasks.up.fizz`:

  ```
  add_column("tasks", "due_date", "date", {"null": true})
  ```

- `migrations/20261026180000_add_due_date_to_tasks.down.fizz`:

  ```
  drop_column("tasks", "due_date")
  ```

Fizz traduce `"date"` a `DATE` en PostgreSQL. `migrations/embed.go` ya embebe `*.fizz`, y `cmd/app/main.go` aplica las migraciones al arrancar (`models.Migrate`). Las filas existentes quedan con `due_date` NULL.

### Modelo (`models/task.go`)

- Añadir al struct, después de `Completed`:

  ```go
  DueDate   *time.Time `json:"due_date" db:"due_date" form:"-"`
  ```

  `form:"-"` evita que `c.Bind` lo rellene; la acción lo asigna explícitamente.

- Añadir:

  ```go
  // DueDateLayout es el formato AAAA-MM-DD de la fecha límite en formularios.
  const DueDateLayout = "2006-01-02"

  // ParseDueDate interpreta una fecha límite sin hora en formato AAAA-MM-DD.
  // Una cadena vacía significa que la tarea no tiene fecha límite.
  func ParseDueDate(value string) (*time.Time, error) {
  	value = strings.TrimSpace(value)
  	if value == "" {
  		return nil, nil
  	}
  	dueDate, err := time.Parse(DueDateLayout, value)
  	if err != nil {
  		return nil, err
  	}
  	return &dueDate, nil
  }

  // HasDueDate indica si la tarea tiene fecha límite.
  func (t Task) HasDueDate() bool {
  	return t.DueDate != nil
  }

  // DueDateValue devuelve la fecha límite en formato AAAA-MM-DD, o "" si no tiene.
  func (t Task) DueDateValue() string {
  	if t.DueDate == nil {
  		return ""
  	}
  	return t.DueDate.Format(DueDateLayout)
  }

  // DueDateLabel devuelve la fecha límite en formato DD/MM/AAAA, o "" si no tiene.
  func (t Task) DueDateLabel() string {
  	if t.DueDate == nil {
  		return ""
  	}
  	return t.DueDate.Format("02/01/2006")
  }
  ```

  Los métodos de presentación usan receptor por valor porque `index.plush.html` itera `for (t) in tasks` sobre valores `Task`; en `edit.plush.html` `task` es `*Task` y también los puede invocar. `time.Parse` con `2006-01-02` exige dos dígitos en mes y día y rechaza días inexistentes (`2026-02-30`) y sufijos de hora.

- `Validate` no cambia: la fecha es opcional y cualquier día válido es aceptable.

### Acciones (`actions/tasks.go`)

- Helper nuevo al final del archivo:

  ```go
  // assignDueDate asigna a la tarea la fecha límite recibida del formulario.
  // Devuelve un error de validación si el valor no es un día AAAA-MM-DD.
  func assignDueDate(task *models.Task, value string) *validate.Errors {
  	verrs := validate.NewErrors()
  	dueDate, err := models.ParseDueDate(value)
  	if err != nil {
  		verrs.Add("due_date", "La fecha límite no es válida.")
  		return verrs
  	}
  	task.DueDate = dueDate
  	return verrs
  }
  ```

- `TasksCreate`: tras `task.Title = strings.TrimSpace(task.Title)`, sustituir la llamada directa a `tx.ValidateAndCreate` por:

  ```go
  verrs := assignDueDate(task, c.Param("due_date"))
  if !verrs.HasAny() {
  	var err error
  	verrs, err = tx.ValidateAndCreate(task)
  	if err != nil {
  		return err
  	}
  }
  ```

  El bloque `if verrs.HasAny() { … 422 … }` existente se conserva tal cual. `c.Param` incluye los valores del formulario (Buffalo fusiona `req.Form` en los parámetros del contexto). Si la fecha es inválida se devuelve solo ese error; el del título aparecerá al corregirla (aceptable: un `<input type="date">` de navegador nunca envía un formato inválido).

- `TasksUpdate`:
  - Actualizar el comentario a «actualiza el título y la fecha límite, conservando los demás datos de la tarea».
  - Ampliar el struct de entrada con `DueDate string \`form:"due_date" json:"due_date"\``.
  - Tras recortar el título, sustituir la llamada directa a `tx.ValidateAndUpdate` por el mismo patrón: `verrs := assignDueDate(task, input.DueDate)`; si no hay errores, `verrs, err = tx.ValidateAndUpdate(task)` (devolviendo `err` si lo hay). El bloque 422 existente se conserva.
  - Un `PUT` sin `due_date` (o vacío) deja la tarea sin fecha: el formulario de edición siempre envía el campo.

- `TasksToggle`, `TasksIndex`, `TasksEdit`, `TasksDestroy` y las rutas de `actions/app.go` no cambian. `tx.Update` en el toggle reescribe `due_date` con el valor cargado, así que se conserva.

### Renderizado

`templates/tasks/index.plush.html`:

- En cada `<li class="task …">`, envolver el título en un contenedor y añadir la fecha debajo:

  ```html
  <div class="task-main">
    <span class="task-title"><%= t.Title %></span>
    <%= if (t.HasDueDate()) { %>
      <p class="task-due">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></p>
    <% } %>
  </div>
  ```

  `.task-status` y `.task-actions` siguen siendo hijos directos del `<li>`. `app.js` busca `.task-title` con `querySelector` (sigue encontrándolo dentro del contenedor) e inserta `.task-status` antes de `.task-actions` (sigue siendo hijo directo), así que no requiere cambios. El `.task` mantiene sus tres columnas de grid: botón, `.task-main`, acciones (`.task-status` es `sr-only`, posicionado en absoluto).

- En el formulario `.new-task`, dentro de `.new-task-row`, entre el input del título y el botón «Añadir»:

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

- Errores por campo: la condición del input del título pasa de `errors.HasAny()` a `len(errors.Get("title")) > 0` (mismos atributos que hoy), y el bloque de error se divide en dos:

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

- Título: la condición de sus atributos pasa de `errors.HasAny()` a `len(errors.Get("title")) > 0` (si hay errores de título: `aria-invalid="true" aria-describedby="title-error"`; si no: `aria-describedby="title-help"`). El texto de ayuda pasa de «Hasta 200 caracteres. Los demás datos de la tarea no cambian.» a «Hasta 200 caracteres.». El bloque de error del título usa la misma condición por campo.
- Después del bloque de error del título y antes de `.edit-task-actions`, añadir:

  ```html
  <label for="task-due-date">Fecha límite</label>
  <input
    id="task-due-date"
    type="date"
    name="due_date"
    value="<%= task.DueDateValue() %>"
    <%= if (len(errors.Get("due_date")) > 0) { %>aria-invalid="true" aria-describedby="due-date-error"<% } else { %>aria-describedby="due-date-help"<% } %>
  >
  <p id="due-date-help" class="field-hint">Opcional. Déjala vacía si la tarea no tiene fecha límite.</p>
  <%= if (len(errors.Get("due_date")) > 0) { %>
    <p id="due-date-error" class="field-error" role="alert">
      <%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %>
    </p>
  <% } %>
  ```

`templates/application.plush.html`: cambiar `/assets/app.css?v=3` por `/assets/app.css?v=4`. No tocar el parámetro de `app.js`.

### CSS (`public/assets/app.css`)

- Regla base de inputs: el selector `input[type="text"]` pasa a `input[type="text"],\ninput[type="date"]` (mismo bloque de declaraciones).
- Justo después de esa regla, añadir:

  ```css
  input[type="date"] {
    width: auto;
  }
  ```

- `.new-task-row input { flex: 1; }` pasa a `.new-task-row input[type="text"] { flex: 1; }` para que el campo de fecha no se estire.
- En `@media (max-width: 380px)`, `.new-task-row input { flex-basis: … }` pasa a `.new-task-row input[type="text"] { flex-basis: … }`; la fecha y el botón bajan juntos a la segunda línea.
- `.task-title`: quitar `align-self: center;` y `min-width: 0;` y añadir `display: block;` (conserva `padding`, `overflow-wrap` y `line-height`).
- Añadir junto a `.task-title`:

  ```css
  .task-main {
    align-self: center;
    min-width: 0;
  }

  .task-due {
    margin-top: -0.45rem;
    padding-bottom: 0.55rem;
    color: var(--color-muted);
    font-size: 0.8rem;
  }
  ```

- Separar la segunda etiqueta del formulario de edición:

  ```css
  .edit-task label ~ label {
    margin-top: 1.25rem;
  }
  ```

## Archivos relevantes

| Ruta | Responsabilidad |
| --- | --- |
| `migrations/20261026180000_add_due_date_to_tasks.up.fizz` (nuevo) | Añade la columna `due_date DATE NULL`. |
| `migrations/20261026180000_add_due_date_to_tasks.down.fizz` (nuevo) | Elimina la columna. |
| `migrations/embed.go` | Embebe `*.fizz` y `*.sql`; sin cambios. |
| `models/task.go` | Campo `DueDate`, `DueDateLayout`, `ParseDueDate`, `HasDueDate`, `DueDateValue`, `DueDateLabel`. |
| `models/task_test.go` | Tests del parser y de la persistencia. |
| `actions/tasks.go` | `assignDueDate`; uso en `TasksCreate` y `TasksUpdate`. |
| `actions/tasks_test.go` | Tests de alta, listado, edición, actualización y toggle con fecha. |
| `actions/app.go` | Rutas; sin cambios. |
| `templates/tasks/index.plush.html` | Campo de fecha en el alta, errores por campo, fecha en la lista. |
| `templates/tasks/edit.plush.html` | Campo de fecha en la edición, errores por campo. |
| `templates/application.plush.html` | Parámetro de caché de `app.css` (`v=4`). |
| `public/assets/app.css` | Estilos del campo de fecha, `.task-main`, `.task-due`, separación en edición. |
| `public/assets/app.js` | Usa `.task-title` y `.task-actions`; sin cambios, comprobar que la estructura los conserva. |
| `database.yml` | Base `todo_test` en `DB_HOST` (por defecto `127.0.0.1`). |
| `.github/workflows/ci.yaml` | Referencia de la secuencia de validación (gofmt, vet, `soda migrate -e test`, tests, build). |

## Implementación paso a paso

1. Crear las dos migraciones con el contenido indicado en «Persistencia».
2. Editar `models/task.go`: campo `DueDate`, constante, `ParseDueDate` y los tres métodos. No cambiar `Validate`.
3. Editar `actions/tasks.go`: añadir `assignDueDate`, aplicarlo en `TasksCreate` y `TasksUpdate` (struct de entrada ampliado y comentario actualizado).
4. Editar `templates/tasks/index.plush.html`: contenedor `.task-main` con la fecha, campo de fecha en el alta y errores por campo.
5. Editar `templates/tasks/edit.plush.html`: errores del título por campo, texto de ayuda del título y bloque del campo de fecha.
6. Editar `templates/application.plush.html`: `app.css?v=4`.
7. Editar `public/assets/app.css` según «CSS».
8. Añadir los tests de «Tests y validación».
9. Preparar la base de test y ejecutar la validación completa.

## Tests y validación

### Tests de modelo (`models/task_test.go`)

Añadir (importar `time` si hace falta):

- `Test_ParseDueDate_Empty`: `""` y `"   "` devuelven `nil, nil`.
- `Test_ParseDueDate_Valid`: `"2026-11-15"` devuelve una fecha con año 2026, mes noviembre y día 15, sin error.
- `Test_ParseDueDate_Invalid`: `"15/11/2026"`, `"2026-02-30"`, `"2026-11-15T10:00"`, `"mañana"` devuelven error y `nil`.
- `Test_Task_DueDate_Persists`: crear con `ValidateAndCreate` una tarea con `DueDate` de `ParseDueDate("2026-11-15")`, recargarla con `Find` y comprobar `DueDateValue() == "2026-11-15"` y `DueDateLabel() == "15/11/2026"`.
- `Test_Task_DueDate_OptionalByDefault`: crear una tarea sin fecha, recargarla y comprobar `DueDate == nil`, `HasDueDate() == false`, `DueDateValue() == ""` y `DueDateLabel() == ""`.

### Tests de acciones (`actions/tasks_test.go`)

Añadir un helper que cree una tarea con fecha (por ejemplo `taskWithDueDate(title, dueDate string) *models.Task`, usando `models.ParseDueDate` y `ValidateAndCreate`, y recargándola) y estos tests, enviando formularios con `url.Values`:

- `Test_TasksCreate_WithDueDate`: `POST /tasks` con `title` y `due_date=2026-11-15` → 303; la tarea guardada tiene `DueDateValue() == "2026-11-15"`.
- `Test_TasksCreate_EmptyDueDate`: `due_date=""` → 303; la tarea guardada tiene `DueDate == nil`.
- `Test_TasksCreate_InvalidDueDate`: `due_date=15/11/2026` → 422, el cuerpo contiene «La fecha límite no es válida.» y `aria-describedby="due-date-error"`; `Count` de tareas es 0.
- `Test_TasksCreate_EmptyTitle_KeepsDueDate`: `title=""`, `due_date=2026-11-15` → 422, el cuerpo contiene «El título es obligatorio.» y `value="2026-11-15"`.
- `Test_TasksIndex_ShowsDueDate`: una tarea con fecha `2026-11-15` y otra sin fecha → 200; el cuerpo contiene `datetime="2026-11-15"` y `15/11/2026`, y `strings.Count(body, \`class="task-due"\`) == 1`.
- `Test_TasksEdit_PrefillsDueDate`: `GET /tasks/{id}/edit` de una tarea con fecha → contiene `name="due_date"` y `value="2026-11-15"`.
- `Test_TasksUpdate_ChangesDueDate`: `PUT` con `title` y `due_date=2026-12-01` → 303; recargada, `DueDateValue() == "2026-12-01"`.
- `Test_TasksUpdate_ClearsDueDate`: `PUT` con `title` y `due_date=""` sobre una tarea con fecha → 303; recargada, `DueDate == nil`.
- `Test_TasksUpdate_InvalidDueDate`: `PUT` con `title=Nuevo título` y `due_date=2026-13-01` → 422 con «La fecha límite no es válida.»; recargada, el título y la fecha no cambian.
- `Test_TasksToggle_KeepsDueDate`: toggle de una tarea con fecha → 303; recargada, `Completed` es `true` y `DueDateValue()` sigue siendo la misma.

Los tests existentes deben seguir pasando sin modificaciones (en particular `Test_TasksUpdate_EmptyTitle`, que comprueba `aria-describedby="title-error"`, y `Test_TasksUpdate_PersistsTitleOnly`).

### Preparar la base de test

La base `todo_test` del contenedor `db` de `compose.yaml` está contaminada por otra rama: tiene aplicada la versión `20261026160000`, que no existe en esta rama, y ya contiene una columna `due_date`. Aplicar sobre ella la migración de este plan fallaría por columna duplicada, y los tests no reflejarían el esquema de la rama. Recréala desde cero, como hace CI. Su contenido es desechable: la suite vacía las tablas en cada test.

```sh
docker compose ps db   # debe estar healthy; si no, docker compose up -d db
docker compose exec -T db dropdb -U postgres --if-exists todo_test
docker compose exec -T db createdb -U postgres todo_test
go run github.com/gobuffalo/pop/v6/soda@v6.4.1 migrate -e test
docker compose exec -T db psql -U postgres -d todo_test -Atc "select version from schema_migration order by version"
```

La última consulta debe listar exactamente `20260901000000`, `20260901000001` y `20261026180000`. Ejecuta los comandos desde la raíz del repositorio (`soda` necesita `database.yml`). `soda` no está instalado en el host; `go run …@v6.4.1` funciona. No toques la base `todo_development`.

### Comprobar la migración `down`

```sh
go run github.com/gobuffalo/pop/v6/soda@v6.4.1 migrate down -e test
docker compose exec -T db psql -U postgres -d todo_test -Atc "select column_name from information_schema.columns where table_name='tasks' and column_name='due_date'"   # vacío
go run github.com/gobuffalo/pop/v6/soda@v6.4.1 migrate -e test
```

### Validación completa (desde la raíz, con `GO_ENV=test`)

```sh
gofmt -l .                 # sin salida
go vet ./...
GO_ENV=test go test -p 1 ./...
go build ./...
```

### Comprobación manual opcional

Si se arranca la aplicación (`/env:start`), crear una tarea con fecha y otra sin ella, comprobar que la lista muestra «Fecha límite: DD/MM/AAAA» solo en la primera, editar la fecha, quitarla y completar la tarea. Arrancar la app en desarrollo aplica la migración sobre `todo_development`, que también tiene una migración ajena (`20261014000000`, columna `priority`). Esa migración no impide añadir `due_date`, pero deja el esquema de desarrollo distinto al de la rama. Por eso esta comprobación es opcional y no sustituye a los tests.

## Riesgos y recuperación

- **Base de test compartida entre ramas.** Recrear `todo_test` borra el esquema que dejaron otras ramas. Es una base desechable y cualquier rama la restaura con `soda migrate -e test` desde su checkout. Si `dropdb` falla por conexiones abiertas, cierra los procesos de test en curso y repite.
- **Zona horaria al escribir y leer `DATE`.** `ParseDueDate` produce medianoche UTC y pgx codifica el día por sus componentes. Al leer, PostgreSQL devuelve el día a medianoche UTC. Los tests comparan por `DueDateValue()` (cadena `AAAA-MM-DD`) y no por igualdad de `time.Time`. Si el test de persistencia mostrara un día desplazado, revisa la zona horaria de la sesión (`SHOW TimeZone`) antes de cambiar el diseño.
- **Sintaxis Plush.** Durante la planificación se verificó en un módulo aparte con plush v5.0.11 y validate v3.3.3, las versiones de `go.mod`. `len(errors.Get("title")) > 0` evalúa a falso sin errores y a verdadero con ellos. `t.HasDueDate()` y `t.DueDateValue()` funcionan dentro de `for (t) in tasks` sobre valores `Task`, y `task.DueDateValue()` funciona sobre `*Task`. Si aun así fallara el renderizado, los tests de acciones lo detectarán. Detente e informa en lugar de cambiar el diseño.
- **Mezcla de receptores en `Task`.** `Validate` usa un receptor puntero y los métodos nuevos usan receptores por valor, a propósito, para Plush. `go vet` no lo marca.
- **Toggle en JavaScript.** La estructura de la lista cambia (aparece `.task-main`). Si una revisión manual mostrara que el toggle en línea ya no actualiza la etiqueta, revisa que `.task-title` y `.task-actions` sigan donde indica este plan antes de tocar `app.js`.
- **Recuperación.** Todo el cambio vive en esta rama. Para revertir el esquema en una base ya migrada, ejecuta `soda migrate down` (aplica `drop_column("tasks", "due_date")`). Para revertir el código, descarta los cambios de los archivos listados.
