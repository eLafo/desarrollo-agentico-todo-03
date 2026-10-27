# Feature: fecha límite (solo día) en las tareas

Origen: issue #5 — <https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5>

> **Quiero añadir fecha límite a las tasks**
> Vale con que sea simplemente un día. Sin hora

## Encargo y criterios de aceptación

### Requisitos explícitos

- R1. Una tarea puede tener una fecha límite.
- R2. La fecha límite es un día (año, mes y día), sin hora.

### Requisitos inferidos (decisiones de este plan)

- I1. La fecha límite es **opcional**. Las tareas existentes no tienen fecha y la issue no la exige.
- I2. Se puede indicar al **crear** la tarea y **cambiar o quitar** al editarla.
- I3. La lista muestra la fecha de cada tarea que la tenga, con el formato `DD/MM/AAAA`. Las tareas completadas también la muestran.
- I4. Un valor que no sea un día válido con formato `AAAA-MM-DD` se rechaza con un 422 y el mensaje `La fecha límite no es válida.`. No se guarda nada.
- I5. Se aceptan fechas pasadas. No hay ninguna regla de negocio sobre el valor.

### Criterios de aceptación

1. En el formulario de la lista hay un campo `<input type="date" name="due_date">` opcional, con la etiqueta accesible `Fecha límite (opcional)`.
2. Si se crea una tarea con `due_date=2026-11-15`, se guarda ese día. La lista muestra `Fecha límite: 15/11/2026` dentro de `<time class="task-due" datetime="2026-11-15">`.
3. Si se crea una tarea con `due_date` vacío o sin ese campo, la tarea no tiene fecha (`NULL`) y la lista no muestra ningún `.task-due` para ella.
4. Si se crea una tarea con una fecha inválida (`2026-02-30`, `15/11/2026`, `2026-11-15T10:00`), la respuesta es 422 con `La fecha límite no es válida.`. No se crea la tarea, se conserva el título escrito y no aparece el error del título.
5. Si se crea una tarea con el título vacío y una fecha válida, la respuesta es 422 con el error del título y el campo de fecha conserva el valor (`value="2026-11-15"`).
6. El formulario de edición muestra un campo `Fecha límite` con el valor actual (`value="AAAA-MM-DD"`) o vacío.
7. Al editar se puede cambiar la fecha o quitarla (campo vacío). El estado completado/pendiente no cambia.
8. Si una edición lleva una fecha inválida, la respuesta es 422 con el mensaje de fecha y no se cambian ni el título ni la fecha guardados.
9. Al completar o reabrir una tarea (`PUT /tasks/{id}/toggle`) se conserva su fecha límite.
10. Ninguna fecha cambia de día por la zona horaria: lo que se guarda es exactamente el día enviado.

## Comportamiento actual y propuesta

### Actual (rama creada desde `origin/main`)

- `migrations/20260901000000_create_tasks.up.fizz` crea `tasks(id uuid, title varchar(200), completed bool default false, created_at, updated_at)`. No hay columna de fecha límite.
- `models/task.go`: `Task{ID, Title, Completed, CreatedAt, UpdatedAt}`. `Validate` solo comprueba el título: obligatorio y como máximo 200 caracteres.
- `actions/app.go` define las rutas `GET /`, `POST /tasks`, `GET /tasks/{task_id}/edit`, `PUT /tasks/{task_id}`, `PUT /tasks/{task_id}/toggle` y `DELETE /tasks/{task_id}`.
- `actions/tasks.go`:
  - `TasksCreate` hace `c.Bind(task)` sobre `models.Task`. Solo se enlaza `title`, porque el resto tiene `form:"-"`. Después llama a `ValidateAndCreate` y, si hay errores, vuelve a renderizar `tasks/index.plush.html` con 422.
  - `TasksUpdate` enlaza un struct anónimo que solo tiene `Title`, para no permitir asignación masiva, y llama a `ValidateAndUpdate`. Si hay errores, responde 422 con `tasks/edit.plush.html`.
  - `TasksToggle` hace `tx.Update(task)` del registro completo.
- `templates/tasks/index.plush.html`: cada `<li class="task">` es un grid de 3 columnas (`.task`: `44px | 1fr | auto`) con el formulario de toggle, `.task-title` y `.task-actions`. `.task-status` es `sr-only` (posición absoluta), así que no ocupa celda. El formulario de alta solo tiene el título. El error se pinta con `errors.HasAny()`, sin distinguir el campo.
- `templates/tasks/edit.plush.html`: solo tiene el campo título, con la pista `Hasta 200 caracteres. Los demás datos de la tarea no cambian.` y el mismo patrón `errors.HasAny()`.
- `public/assets/app.js`: el toggle usa `task.querySelector(".task-title").textContent` para el `aria-label` e inserta `.task-status` antes de `.task-actions`. No cambia.
- `templates/application.plush.html` carga `/assets/app.css?v=3`.

### Propuesta

Es un cambio vertical mínimo que sigue los patrones existentes:

1. Migración nueva: columna `due_date` de tipo `date` y nullable.
2. Modelo: campo `DueDate *time.Time` (no enlazable por formulario) y un método `SetDueDate(string)` que interpreta `AAAA-MM-DD`. El método marca la tarea si el valor es inválido, y `Validate` añade el error en `due_date`. Así la validación sigue en el modelo y los errores de título y de fecha salen juntos con el `ValidateAndCreate`/`ValidateAndUpdate` de siempre. También hay métodos de presentación con receptor por valor, para que Plush pueda llamarlos dentro del `for`.
3. Acciones: `TasksCreate` lee `c.Param("due_date")` y `TasksUpdate` añade `DueDate` a su struct de entrada. Las dos llaman a `task.SetDueDate(...)`.
4. Templates: campo `type="date"` en los dos formularios, `<time class="task-due">` en la lista y errores por campo en lugar de `errors.HasAny()`.
5. CSS: estilos del input de fecha y del `.task-due`, que va en la segunda fila del grid, y más versión de caché del CSS.

## Alcance

### Incluido

- Migración `up` y `down` de la columna `tasks.due_date` (`date NULL`).
- Campo, parseo, validación y presentación de la fecha en `models.Task`.
- Alta y edición con fecha límite (`TasksCreate`, `TasksUpdate`).
- Mostrar la fecha en la lista y precargarla en la edición.
- Errores por campo (`title`, `due_date`) en los dos formularios.
- Estilos mínimos y cambio de versión de caché `app.css?v=4`.
- Tests de modelo y de acciones.

### Excluido

- Hora o zona horaria. Queda descartado de forma explícita por la issue.
- Resaltar tareas vencidas o próximas a vencer, avisos y recordatorios.
- Ordenar o filtrar por fecha límite. La lista sigue ordenada por `created_at asc, id asc`.
- Validar que la fecha no sea pasada o poner límites de rango.
- Cambios en `public/assets/app.js`, rutas, fixtures (`fixtures/tasks.toml`), la migración de datos semilla (`20260901000001_seed_tasks.*.sql`) y `go.mod`.
- El fix del toggle por JavaScript que está en el `main` local (commit `429e9d6`, sin publicar en `origin/main`). Es otro encargo.

## Diseño por capas

### Persistencia y migración

Archivos nuevos con un timestamp distinto de los que usaron otras ramas en las bases locales compartidas:

`migrations/20261027100000_add_due_date_to_tasks.up.fizz`

```
add_column("tasks", "due_date", "date", {"null": true})
```

`migrations/20261027100000_add_due_date_to_tasks.down.fizz`

```
drop_column("tasks", "due_date")
```

El traductor Postgres de fizz deja el tipo `date` sin cambios: es `DATE`. `migrations/embed.go` ya incrusta `*.fizz`, y `cmd/app/main.go` aplica las migraciones al arrancar, así que no hay más cambios.

### Modelo (`models/task.go`)

- Se añaden las constantes:
  ```go
  // DueDateLayout es el formato del día límite que envía <input type="date">.
  const DueDateLayout = "2006-01-02"

  // DueDateDisplayLayout es el formato con el que se muestra el día límite.
  const DueDateDisplayLayout = "02/01/2006"
  ```
- Campos nuevos de `Task`. `DueDate` va entre `Completed` y `CreatedAt`; ejecuta `gofmt` para alinear las etiquetas:
  ```go
  DueDate   *time.Time `json:"due_date" db:"due_date" form:"-"`

  // dueDateInvalid indica que el último valor pasado a SetDueDate no era un día válido.
  dueDateInvalid bool
  ```
  `form:"-"` evita que `c.Bind` en `TasksCreate` toque el campo. Pop solo mapea campos con etiqueta `db` y sqlx ignora los no exportados, así que `dueDateInvalid` no se persiste.
- Nuevo método:
  ```go
  // SetDueDate asigna la fecha límite a partir del valor del formulario (AAAA-MM-DD).
  // Un valor vacío quita la fecha; uno inválido la deja vacía y hace fallar Validate.
  func (t *Task) SetDueDate(value string) {
  	t.DueDate = nil
  	t.dueDateInvalid = false

  	value = strings.TrimSpace(value)
  	if value == "" {
  		return
  	}

  	day, err := time.Parse(DueDateLayout, value)
  	if err != nil {
  		t.dueDateInvalid = true
  		return
  	}
  	t.DueDate = &day
  }
  ```
  `time.Parse` devuelve medianoche UTC y rechaza días imposibles como `2026-02-30`.
- Métodos de presentación con **receptor por valor**. Hace falta porque en `for (t) in tasks` cada `t` es un `models.Task` no direccionable:
  ```go
  // HasDueDate indica si la tarea tiene fecha límite.
  func (t Task) HasDueDate() bool {
  	return t.DueDate != nil
  }

  // DueDateValue devuelve la fecha límite como AAAA-MM-DD, o "" si no tiene.
  func (t Task) DueDateValue() string {
  	if t.DueDate == nil {
  		return ""
  	}
  	return t.DueDate.Format(DueDateLayout)
  }

  // DueDateLabel devuelve la fecha límite como DD/MM/AAAA, o "" si no tiene.
  func (t Task) DueDateLabel() string {
  	if t.DueDate == nil {
  		return ""
  	}
  	return t.DueDate.Format(DueDateDisplayLayout)
  }
  ```
  No llames a `.Local()` ni a `.In(...)`: el día debe salir tal como se guardó.
- En `Validate`, después de las comprobaciones del título:
  ```go
  if t.dueDateInvalid {
  	verrs.Add("due_date", "La fecha límite no es válida.")
  }
  ```

### Acciones (`actions/tasks.go`)

- `TasksCreate`: justo después de `task.Title = strings.TrimSpace(task.Title)`, añade `task.SetDueDate(c.Param("due_date"))`. En Buffalo, `c.Param` incluye los valores del cuerpo del formulario. El resto del flujo, incluido el 422 con `tasks/index.plush.html`, no cambia.
- `TasksUpdate`:
  - Comentario: `// TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.`
  - Struct de entrada:
    ```go
    input := struct {
    	Title   string `form:"title" json:"title"`
    	DueDate string `form:"due_date" json:"due_date"`
    }{}
    ```
  - Después de asignar el título: `task.SetDueDate(input.DueDate)`. Un `due_date` vacío o ausente quita la fecha. El formulario de edición siempre lo envía.
- `TasksToggle`, `TasksIndex`, `TasksEdit`, `TasksDestroy` y las rutas no cambian.

### Renderizado

`templates/tasks/index.plush.html`

- En cada `<li class="task">`, después del bloque `<%= if (t.Completed) { %>…task-status…<% } %>` y antes de `<div class="task-actions">`:
  ```html
  <%= if (t.HasDueDate()) { %>
    <time class="task-due" datetime="<%= t.DueDateValue() %>">Fecha límite: <%= t.DueDateLabel() %></time>
  <% } %>
  ```
  No metas la fecha dentro de `.task-title`. `app.js` usa su `textContent` para el `aria-label` y el tachado de completadas se aplica a ese elemento.
- En el formulario `.new-task`, dentro de `.new-task-row`, entre el input del título y el botón `Añadir`:
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
- En el input del título, cambia la condición `errors.HasAny()` por `len(errors.Get("title")) > 0`. Conserva `autofocus aria-invalid="true" aria-describedby="title-error"`.
- Bloque de errores. Sustituye el `<%= if (errors.HasAny()) { %>…title-error…<% } %>` por dos bloques condicionados por campo:
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

`templates/tasks/edit.plush.html`

- En el input del título, cambia `errors.HasAny()` por `len(errors.Get("title")) > 0`, en el atributo `aria-*` y en el bloque `#title-error`.
- Cambia la pista del título a `Hasta 200 caracteres. El estado de la tarea no cambia.`. La frase antigua («Los demás datos…») sería engañosa junto a un campo de fecha editable.
- Después del bloque `#title-error` y antes de `.edit-task-actions`:
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

`templates/application.plush.html`

- `/assets/app.css?v=3` pasa a `/assets/app.css?v=4`. Es el mismo patrón de invalidación de caché que ya se usa con `app.js`.

### CSS (`public/assets/app.css`)

- Comparte los estilos base del input de texto: el selector `input[type="text"] {` pasa a ser `input[type="text"],\ninput[type="date"] {`.
- Justo después de la regla `.new-task-row input { flex: 1; }`:
  ```css
  .new-task-row input[type="date"] {
    flex: none;
    width: auto;
  }
  ```
  Así el título sigue ocupando el espacio flexible. En el breakpoint de 380 px la fila ya hace `flex-wrap`, y esta regla es más específica que `.new-task-row input` de la media query, así que la fecha pasa a la segunda línea con su ancho natural.
- En `.task-actions` (la regla base, con `display: flex`), añade `grid-column: 3;` y `grid-row: 1;`. Las acciones se quedan en la primera fila aunque exista `.task-due`.
- Regla nueva junto a `.task-title`:
  ```css
  .task-due {
    grid-column: 2;
    margin-top: -0.55rem;
    padding-bottom: 0.4rem;
    color: var(--color-muted);
    font-size: 0.8rem;
  }
  ```
  Con el auto-placement del grid, `.task-due` cae en la fila 2, columna 2, bajo el título. `.task-status` y el `.task-status` que inserta `app.js` siguen fuera del flujo del grid porque son `sr-only`.
- Separa el segundo campo del formulario de edición:
  ```css
  .edit-task label:not(:first-of-type) {
    margin-top: 1.5rem;
  }
  ```
  Colócala junto a `.edit-task label`.

## Archivos relevantes

| Ruta | Responsabilidad | Cambio |
|------|-----------------|--------|
| `migrations/20261027100000_add_due_date_to_tasks.up.fizz` | Añade `tasks.due_date date NULL` | Nuevo |
| `migrations/20261027100000_add_due_date_to_tasks.down.fizz` | Elimina `tasks.due_date` | Nuevo |
| `models/task.go` | Modelo `Task`, parseo, validación y presentación de la fecha | Modificar |
| `models/task_test.go` | Tests de modelo | Ampliar |
| `actions/tasks.go` | `TasksCreate` y `TasksUpdate` leen `due_date` | Modificar |
| `actions/tasks_test.go` | Tests de acciones y renderizado | Ampliar |
| `templates/tasks/index.plush.html` | Lista (`.task-due`) y formulario de alta | Modificar |
| `templates/tasks/edit.plush.html` | Formulario de edición | Modificar |
| `templates/application.plush.html` | Versión de caché de `app.css` | Modificar |
| `public/assets/app.css` | Estilos del input de fecha, `.task-due` y espaciado de la edición | Modificar |
| `actions/app.go` | Rutas (referencia) | Sin cambios |
| `public/assets/app.js` | Toggle (depende de `.task-title` y `.task-actions`) | Sin cambios |
| `models/models.go`, `migrations/embed.go`, `cmd/app/main.go` | Conexión y migraciones al arrancar (referencia) | Sin cambios |
| `database.yml`, `compose.yaml`, `.github/workflows/ci.yaml` | Entorno de tests (referencia) | Sin cambios |

## Implementación paso a paso

1. Comprueba que estás en `feature/issue-5-fecha-limite-por-tarea` y que el árbol solo contiene este plan sin commit.
2. Crea las dos migraciones de la sección «Persistencia y migración».
3. Modifica `models/task.go` según «Modelo».
4. Modifica `actions/tasks.go` según «Acciones».
5. Modifica `templates/tasks/index.plush.html`, `templates/tasks/edit.plush.html` y `templates/application.plush.html` según «Renderizado».
6. Modifica `public/assets/app.css` según «CSS».
7. Añade los tests de la sección siguiente.
8. Prepara la base de test y ejecuta la validación completa (sección siguiente).
9. Revisa `git diff --check`, `git diff` y `git status --short`. Solo deben aparecer los archivos de la tabla marcados como nuevos o modificados, más este plan.

## Tests y validación

### Tests de modelo (`models/task_test.go`)

Usa `ms.DB` como los tests existentes:

- `Test_Task_DueDate_OptionalByDefault`: crea `&Task{Title: "Sin fecha"}`, recárgalo con `ms.DB.Find` y comprueba `DueDate == nil`, `HasDueDate() == false` y `DueDateValue() == ""`.
- `Test_Task_DueDate_Persists`: llama a `SetDueDate(" 2026-11-15 ")`, ejecuta `ValidateAndCreate` sin errores y recarga con `ms.DB.Find` en otra variable. Comprueba `DueDateValue() == "2026-11-15"` y `DueDateLabel() == "15/11/2026"`.
- `Test_Task_DueDate_Invalid`: recorre `"2026-02-30"`, `"15/11/2026"`, `"2026-11-15T10:00"` y `"mañana"`. En cada caso, `SetDueDate` + `ValidateAndCreate` da `verrs.Get("due_date")` con `"La fecha límite no es válida."` y `DueDate == nil`. Al final, `ms.DB.Count(&Task{}) == 0`.
- `Test_Task_DueDate_EmptyClears`: llama a `SetDueDate("2026-11-15")` y luego a `SetDueDate("   ")`. Comprueba `DueDate == nil` y que `task.Validate(ms.DB)` no da errores en `due_date`.

### Tests de acciones (`actions/tasks_test.go`)

Añade un helper que cree una tarea con fecha, por ejemplo `taskWithDueDate(completed bool, day string) *models.Task`. Debe usar `SetDueDate` + `as.DB.ValidateAndCreate` + `as.DB.Reload`, como `taskForEditing`. Usa `url.Values` para enviar `due_date`.

- `Test_TasksIndex_NewTaskFormHasDueDate`: `GET /` contiene `type="date"`, `name="due_date"` y `Fecha límite (opcional)`.
- `Test_TasksIndex_ShowsDueDate`: crea una tarea con `2026-11-15` y otra sin fecha (`taskForEditing(false)`). El body contiene `datetime="2026-11-15"` y `Fecha límite: 15/11/2026`, y `strings.Count(body, "class=\"task-due\"") == 1`.
- `Test_TasksCreate_WithDueDate`: `POST /tasks` con `title=Entregar el informe` y `due_date=2026-11-15` devuelve 303 a `/`. `as.DB.First(task)` tiene `DueDateValue() == "2026-11-15"`.
- `Test_TasksCreate_EmptyDueDate`: `POST` con `due_date=""` devuelve 303 y la tarea tiene `DueDate == nil`.
- `Test_TasksCreate_InvalidDueDate`: `POST` con `title=Entregar el informe` y `due_date=2026-02-30` devuelve 422. El body contiene `La fecha límite no es válida.`, contiene `value="Entregar el informe"` y no contiene `id="title-error"`. Count == 0.
- `Test_TasksCreate_EmptyTitle_KeepsDueDate`: `POST` con `title=""` y `due_date=2026-11-15` devuelve 422. El body contiene `El título es obligatorio.` y `value="2026-11-15"`. Count == 0.
- `Test_TasksEdit_PrefillsDueDate`: `GET /tasks/{id}/edit` de una tarea con `2026-11-15` contiene `name="due_date"` y `value="2026-11-15"`.
- `Test_TasksUpdate_ChangesDueDate`: con una tarea completada y fecha `2026-11-15`, `PUT` con `title=Título actualizado` y `due_date=2026-12-01` devuelve 303. Tras recargar, `DueDateValue() == "2026-12-01"`, el título está actualizado y `Completed` sigue en `true`.
- `Test_TasksUpdate_ClearsDueDate`: `PUT` con `due_date=""` devuelve 303 y, tras recargar, `DueDate == nil`.
- `Test_TasksUpdate_InvalidDueDate`: `PUT` con `title=Nuevo título` y `due_date=2026-13-01` devuelve 422. El body contiene `La fecha límite no es válida.` y no contiene `id="title-error"`. Tras recargar, el título sigue siendo el original y `DueDateValue() == "2026-11-15"`.
- `Test_TasksToggle_PreservesDueDate`: `PUT /tasks/{id}/toggle` sobre una tarea con fecha devuelve 303. Tras recargar, `Completed == true` y la fecha no cambia.

Los tests existentes deben seguir pasando sin cambios. Entre ellos están `Test_TasksUpdate_EmptyTitle`, que comprueba `aria-describedby="title-error"`, y `Test_TasksIndex_ListsTasks`, cuyos fixtures no tienen `due_date`.

### Preparación de la base de test

Los tests necesitan PostgreSQL (`compose.yaml`, servicio `db`) y `todo_test` migrada con **las migraciones de esta rama**. Hay que tener en cuenta lo siguiente:

- `todo_test` es compartida entre ramas. Durante la planificación tenía ya una columna `due_date date` y la versión `20261027000000` en `schema_migration`, procedentes de otra rama. Hay que recrearla.
- Recrea **solo** `todo_test`. Nunca toques `todo_development`.
- Los tests vacían todas las tablas en cada ejecución (`suite`), así que `todo_test` no contiene datos que conservar.

```sh
docker compose up -d db          # si el contenedor no está ya en marcha y healthy
docker compose exec -T db psql -U postgres -v ON_ERROR_STOP=1 \
  -c 'DROP DATABASE IF EXISTS todo_test' -c 'CREATE DATABASE todo_test'
go run github.com/gobuffalo/pop/v6/soda@v6.4.1 migrate -e test
docker compose exec -T db psql -U postgres -d todo_test -c '\d tasks'
```

El último comando debe mostrar exactamente `id`, `title`, `completed`, `created_at`, `updated_at` y `due_date | date` nullable.

Comprueba también que la migración se puede revertir y volver a aplicar:

```sh
go run github.com/gobuffalo/pop/v6/soda@v6.4.1 migrate down -e test   # \d tasks ya no muestra due_date
go run github.com/gobuffalo/pop/v6/soda@v6.4.1 migrate -e test        # due_date vuelve a aparecer
```

### Comandos de validación

Desde la raíz del repositorio:

1. `gofmt -l .`: sin salida.
2. `go vet ./...`: sin errores.
3. `GO_ENV=test go test -p 1 ./...`: todo en verde. `GO_ENV=test` es obligatorio. Sin él, `models.DB` (que usan las acciones mediante `popmw.Transaction`) se conecta a `todo_development`.
4. `go build ./...`: sin errores.
5. `git diff --check`: sin salida.

La disposición visual del CSS (fila 2 del grid y ancho del input de fecha) no tiene test automático. Los tests de acciones cubren el HTML renderizado. Indica en el PR que el aspecto visual se debe revisar a mano.

## Riesgos y recuperación

- **Bases locales compartidas entre ramas.** `todo_test` y `todo_development` tienen cambios de esquema de otras ramas: `due_date` en test y `priority` en development. Por eso se recrea `todo_test` antes de validar, y la migración usa el timestamp `20261027100000`, que no coincide con el de otras ramas. Así nadie la da por aplicada por error. Si `DROP DATABASE` falla porque hay conexiones abiertas, detente e infórmalo. No termines procesos ajenos.
- **Zona horaria.** La fecha se interpreta en UTC (`time.Parse`). La columna `date` no tiene zona y pgx devuelve el día tal como se guardó. Si se añadiera `.Local()` o `.In(...)` al formatear, el día podría desplazarse. Los tests de ida y vuelta (`2026-11-15` → `15/11/2026`) lo vigilan.
- **Quitar la fecha al editar.** `PUT /tasks/{id}` sin `due_date` quita la fecha. El formulario de edición siempre envía el campo, y no hay otros clientes (no hay API JSON). Este comportamiento se acepta y está documentado aquí.
- **Navegadores sin `type="date"`.** El input se ve como texto. Si el usuario escribe algo que no es `AAAA-MM-DD`, recibe `La fecha límite no es válida.` y no se guarda nada.
- **Caché del CSS.** Se pasa a `app.css?v=4` para que los clientes no sigan usando estilos antiguos.
- **Conflicto con el `main` local.** El commit `429e9d6` (fix del toggle, no publicado) cambia `app.js?v=3` en `templates/application.plush.html`. Esa línea es distinta de la de `app.css`, así que no se espera conflicto, pero conviene revisarlo al integrar.
- **Recuperación.** Haz `git revert` del commit. Si la migración ya se aplicó en algún entorno, ejecuta antes `soda migrate down` (o aplica `drop_column`) para eliminar `tasks.due_date`. Se pierden las fechas guardadas, pero no el resto de datos de las tareas.
