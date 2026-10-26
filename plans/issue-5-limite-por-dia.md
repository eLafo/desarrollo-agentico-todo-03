# Feature: fecha límite (solo día) en las tareas

## Encargo y criterios de aceptación

Issue [#5](https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5): «Quiero añadir fecha límite a las tasks». Cuerpo: «Vale con que sea simplemente un día. Sin hora».

Requisitos explícitos:

- Una tarea puede tener una fecha límite.
- La fecha límite es un día de calendario, sin hora.

Requisitos inferidos (necesarios para que la fecha límite sea útil y coherente con la aplicación actual):

- La fecha límite es opcional: las tareas existentes y las nuevas pueden no tenerla.
- Se puede indicar al crear la tarea y añadir, cambiar o quitar al editarla.
- La lista muestra la fecha límite de las tareas que la tienen.
- Un valor que no sea un día válido se rechaza con un error de validación (422), nunca con un 500.

Criterios de aceptación:

1. La tabla `tasks` tiene una columna `due_date` de tipo `date` que admite `NULL`; la migración es reversible.
2. Al crear una tarea con título válido y `due_date=2026-11-15`, se guarda con esa fecha límite. Si `due_date` llega vacío o no llega, se guarda sin fecha límite.
3. En la lista, una tarea con fecha límite muestra `Fecha límite: <time datetime="2026-11-15">15/11/2026</time>`. Una tarea sin fecha límite no muestra ningún elemento `task-due`.
4. El formulario de edición precarga la fecha límite (`value="2026-11-15"`) en un `<input type="date" name="due_date">`.
5. Al editar, se puede fijar, cambiar o vaciar la fecha límite; el estado de completada, el `id` y `created_at` no cambian.
6. Un `due_date` no válido (`2026-02-30`, `15/11/2026`, `2026-11-15T10:00`, `0000-01-01`) devuelve 422 con el mensaje «La fecha límite no es un día válido.», no crea la tarea y no modifica la existente.
7. Completar o reabrir una tarea conserva su fecha límite.
8. Los errores de título y de fecha límite se muestran cada uno junto a su campo; un error solo de fecha no marca el título como inválido.

## Comportamiento actual y propuesta

Actualmente `models.Task` solo tiene `ID`, `Title`, `Completed`, `CreatedAt` y `UpdatedAt` (`models/task.go`). La tabla se crea en `migrations/20260901000000_create_tasks.up.fizz` sin ninguna columna de fecha. El formulario de creación (`templates/tasks/index.plush.html`) y el de edición (`templates/tasks/edit.plush.html`) solo envían `title`. `TasksUpdate` lee un struct de entrada con solo `Title` para no aceptar otros campos. Las plantillas muestran el bloque de error del título siempre que `errors.HasAny()`.

Propuesta:

- Persistencia: nueva columna `due_date date NULL`.
- Modelo: campo `DueDate nulls.Time` (no enlazable por formulario), un método `SetDueDate(string)` que interpreta el valor del formulario (`AAAA-MM-DD`, vacío = sin fecha) y marca el valor como no válido para que `Validate` lo rechace, y dos métodos de presentación (`DueDateValue` y `DueDateLabel`) que usan las plantillas.
- Acciones: `TasksCreate` y `TasksUpdate` llaman a `task.SetDueDate(...)` antes de validar y guardar. La validación sigue en `Validate`, así que `ValidateAndCreate` y `ValidateAndUpdate` bloquean el guardado y devuelven todos los errores a la vez.
- Vistas: un `<input type="date">` en ambos formularios, la fecha en la lista con formato `DD/MM/AAAA` dentro de `<time>` y errores por campo.
- CSS: estilos para el input de fecha, la línea de fecha bajo el título y el ajuste responsive de la fila de creación.

## Alcance

Incluido:

- Migración para añadir y quitar `due_date`.
- Campo, validación y métodos de presentación en `models.Task`.
- Lectura de `due_date` en `TasksCreate` y `TasksUpdate`.
- Input de fecha en los formularios de creación y edición, y visualización en la lista.
- Estilos en `public/assets/app.css` y cambio del parámetro de caché del CSS.
- Tests de modelo y de acciones.
- `go.mod`: `github.com/gobuffalo/nulls` pasa de dependencia indirecta a directa con `go mod tidy`.

Excluido:

- Hora o zona horaria en la fecha límite.
- Resaltar tareas vencidas o próximas a vencer, ordenar o filtrar por fecha límite.
- Recordatorios o notificaciones.
- Restringir la fecha a días futuros: se aceptan días pasados para poder registrar o editar tareas ya vencidas.
- Cambios en `public/assets/app.js`, en la migración de seeds y en `fixtures/tasks.toml`.
- Distinguir entre `due_date` ausente y vacío en la edición: el formulario siempre envía el campo y ambos casos quitan la fecha límite.

## Diseño por capas

### Persistencia y migraciones

Nueva migración fizz `migrations/20261026200000_add_due_date_to_tasks.up.fizz`:

```
add_column("tasks", "due_date", "date", {"null": true})
```

Y su reversa `migrations/20261026200000_add_due_date_to_tasks.down.fizz`:

```
drop_column("tasks", "due_date")
```

El traductor de fizz para PostgreSQL deja el tipo `date` tal cual (`ALTER TABLE "tasks" ADD COLUMN "due_date" date`). `migrations/embed.go` ya incluye `*.fizz`; no hay que tocarlo. `cmd/app/main.go` aplica las migraciones al arrancar.

### Modelo (`models/task.go`)

- Constante exportada `DueDateLayout = "2006-01-02"` (formato del input `type="date"` y del atributo `datetime`) y constante no exportada `dueDateDisplayLayout = "02/01/2006"` (formato visible).
- Campo `DueDate nulls.Time` con tags `json:"due_date" db:"due_date" form:"-"`, situado entre `Completed` y `CreatedAt`. El tag `form:"-"` mantiene la protección actual: `c.Bind(task)` no lo asigna.
- Campo no exportado `invalidDueDate bool` con tag `db:"-"`. El tag es obligatorio: Pop (`columns.TagsFor`) trata cualquier campo sin tag `db` como una columna con el nombre del campo, y sin él los `SELECT` fallarían.
- `func (t *Task) SetDueDate(value string)`: pone `invalidDueDate = false` y recorta espacios. Si el valor queda vacío, asigna `nulls.Time{}`. Si `time.Parse(DueDateLayout, value)` falla o el año es `< 1`, marca `invalidDueDate = true` y no toca `DueDate`. En otro caso asigna `nulls.NewTime(day)`, con el día a medianoche en UTC (lo que devuelve `time.Parse`).
- `Validate` añade, después de la validación del título, `verrs.Add("due_date", "La fecha límite no es un día válido.")` si `t.invalidDueDate`.
- `func (t Task) DueDateValue() string` devuelve `DueDate.Time.Format(DueDateLayout)` o `""` si `!DueDate.Valid`.
- `func (t Task) DueDateLabel() string` devuelve `DueDate.Time.Format(dueDateDisplayLayout)` o `""` si `!DueDate.Valid`.

Los dos métodos de presentación usan receptor por valor. En `for (t) in tasks`, Plush recorre valores `Task` no direccionables y solo puede llamar métodos de receptor por valor. Con `*Task` (`task` en las plantillas) funcionan igual.

### Acciones (`actions/tasks.go`)

- `TasksCreate`: después de `task.Title = strings.TrimSpace(task.Title)`, añade `task.SetDueDate(c.Param("due_date"))`. `c.Param` incluye los valores del cuerpo del formulario (Buffalo los añade a los params en `newContext`). El resto del flujo no cambia: si `verrs.HasAny()`, se vuelve a renderizar `index` con 422.
- `TasksUpdate`: añade el campo ``DueDate string `form:"due_date" json:"due_date"` `` al struct de entrada anónimo y, tras asignar el título, llama a `task.SetDueDate(input.DueDate)`. Actualiza el comentario de la función: «TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.»
- `TasksToggle` no cambia: `tx.Update(task)` reescribe la fila con el `DueDate` leído, así que lo conserva.

No hay rutas nuevas.

### Renderizado (Plush)

`templates/tasks/index.plush.html`:

- Envuelve el título de cada tarea en un contenedor y añade la fecha límite debajo cuando existe:

  ```html
  <div class="task-main">
    <span class="task-title"><%= t.Title %></span>
    <%= if (t.DueDate.Valid) { %>
      <p class="task-due">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></p>
    <% } %>
  </div>
  ```

  `.task-title` sigue existiendo: `app.js` lo usa para el `aria-label` del toggle, y `.task-completed .task-title` lo tacha.
- En el formulario de creación, tras el input del título y antes del botón «Añadir»:

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
- Errores por campo. En el input del título, cambia la condición `errors.HasAny()` por `len(errors.Get("title")) > 0`; los atributos que añade se mantienen. Haz lo mismo en el bloque `title-error` y añade después uno equivalente para la fecha:

  ```html
  <%= if (len(errors.Get("due_date")) > 0) { %>
    <p id="due-date-error" class="field-error" role="alert">
      <%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %>
    </p>
  <% } %>
  ```

  Si fallan los dos campos, ambos inputs llevan `autofocus` y el navegador enfoca el primero (el título).

`templates/tasks/edit.plush.html`:

- En el input del título y en el bloque `title-error`, cambia `errors.HasAny()` por `len(errors.Get("title")) > 0` (la rama `else` con `aria-describedby="title-help"` se conserva).
- Tras el bloque de error del título y antes de `.edit-task-actions`, añade:

  ```html
  <div class="due-date-field">
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
  </div>
  ```

`templates/application.plush.html`: cambia `/assets/app.css?v=3` por `/assets/app.css?v=4` para invalidar la caché del CSS.

### CSS (`public/assets/app.css`)

1. Sustituye la regla `.task-title { align-self: center; min-width: 0; padding: 0.65rem 0; overflow-wrap: anywhere; line-height: 1.55; }` por:

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
     margin-top: 0.1rem;
     color: var(--color-muted);
     font-size: 0.8rem;
   }
   ```

   `.task-main` pasa a ser la celda central de la rejilla de `.task`, que sigue teniendo tres columnas: toggle, contenido y acciones.
2. Cambia el selector `input[type="text"]` por `input[type="text"],\ninput[type="date"]` (mismas declaraciones).
3. Cambia `.new-task-row input { flex: 1; }` por `.new-task-row input[type="text"] { flex: 1; }` y añade:

   ```css
   .new-task-row input[type="date"] {
     flex: 0 0 auto;
     width: auto;
   }
   ```
4. Tras `.edit-task label { … }`, añade `.due-date-field { margin-top: 1.5rem; }`.
5. En `@media (max-width: 760px)`, la regla `.new-task-row { gap: 0.4rem; }` pasa a `.new-task-row { flex-wrap: wrap; gap: 0.4rem; }`. Añade en ese mismo bloque `.new-task-row input[type="text"] { flex-basis: calc(100% - 44px); }` y `.new-task-row .button { margin-left: auto; }`. En pantallas estrechas, la fecha y el botón pasan a una segunda línea.
6. En `@media (max-width: 380px)`, elimina las tres reglas de `.new-task-row` (ya cubiertas por el paso 5) y conserva `.container { padding-inline: 0.8rem; }`.

## Archivos relevantes

| Ruta | Responsabilidad |
| --- | --- |
| `migrations/20261026200000_add_due_date_to_tasks.up.fizz` | Nuevo: añade `due_date date NULL`. |
| `migrations/20261026200000_add_due_date_to_tasks.down.fizz` | Nuevo: elimina `due_date`. |
| `models/task.go` | Campo `DueDate`, `SetDueDate`, validación y métodos de presentación. |
| `models/task_test.go` | Tests de modelo de la fecha límite. |
| `actions/tasks.go` | `TasksCreate` y `TasksUpdate` leen `due_date`. |
| `actions/tasks_test.go` | Tests de creación, edición, lista, toggle y errores con fecha límite. |
| `templates/tasks/index.plush.html` | Fecha límite en la lista, input en la creación y errores por campo. |
| `templates/tasks/edit.plush.html` | Input de fecha límite precargado y errores por campo. |
| `templates/application.plush.html` | Parámetro de caché del CSS (`v=4`). |
| `public/assets/app.css` | Estilos de la fecha en la lista, del input de fecha y responsive de la fila de creación. |
| `go.mod` | `github.com/gobuffalo/nulls` pasa a dependencia directa (`go mod tidy`). |
| `migrations/embed.go`, `cmd/app/main.go`, `public/assets/app.js` | Solo lectura: embeben y aplican migraciones; el JS del toggle sigue funcionando con `.task-title` y `.task-actions`. |

## Implementación paso a paso

1. Crea las dos migraciones fizz con el contenido de «Persistencia y migraciones».
2. En `models/task.go`, importa `github.com/gobuffalo/nulls` y añade constantes, campos, `SetDueDate`, la regla en `Validate`, `DueDateValue` y `DueDateLabel` según «Modelo». Ejecuta `go mod tidy` y comprueba que el único cambio en `go.mod` es mover `github.com/gobuffalo/nulls v0.4.2` al bloque de dependencias directas (`go.sum` no debería cambiar).
3. En `actions/tasks.go`, aplica los cambios de «Acciones».
4. Edita `templates/tasks/index.plush.html`, `templates/tasks/edit.plush.html` y `templates/application.plush.html` según «Renderizado».
5. Edita `public/assets/app.css` según «CSS».
6. Añade los tests de «Tests y validación».
7. Ejecuta `gofmt -w` sobre los `.go` modificados y la validación completa.

## Tests y validación

### Tests de modelo (`models/task_test.go`)

- `Test_Task_DueDate_OptionalByDefault`: crea una tarea solo con título; tras `ms.DB.Find`, `DueDate.Valid` es `false` y `DueDateValue()` y `DueDateLabel()` devuelven `""`.
- `Test_Task_DueDate_Persists`: `SetDueDate("2026-11-15")`, `ValidateAndCreate` sin errores; tras `Find`, `DueDate.Valid` es `true`, `DueDateValue() == "2026-11-15"` y `DueDateLabel() == "15/11/2026"`.
- `Test_Task_DueDate_EmptyClears`: una tarea con fecha, tras `SetDueDate("   ")`, tiene `DueDate.Valid == false` y valida sin errores.
- `Test_Task_DueDate_Invalid`: para cada valor de `{"2026-02-30", "15/11/2026", "2026-11-15T10:00", "mañana", "0000-01-01"}`, `ValidateAndCreate` devuelve `verrs.Get("due_date")` con «La fecha límite no es un día válido.» y no crea ninguna fila.
- `Test_Task_DueDate_ValidAfterInvalid`: `SetDueDate("2026-02-30")` seguido de `SetDueDate("2026-11-15")` valida sin errores. Comprueba que `SetDueDate` reinicia la marca de valor no válido.

### Tests de acciones (`actions/tasks_test.go`)

Añade un helper `taskWithDueDate(day string) *models.Task` que cree una tarea con título «Con fecha límite», llame a `SetDueDate(day)`, valide sin errores y recargue.

- `Test_TasksCreate_WithDueDate`: `POST /tasks` con `url.Values{"title": {"Entregar informe"}, "due_date": {"2026-11-15"}}` responde 303; la tarea guardada tiene `DueDateValue() == "2026-11-15"`.
- `Test_TasksCreate_EmptyDueDate`: `due_date` vacío responde 303 y la tarea no tiene fecha límite.
- `Test_TasksCreate_InvalidDueDate`: `due_date=2026-02-30` con título válido responde 422. El cuerpo contiene «La fecha límite no es un día válido.», `aria-describedby="due-date-error"` y el título enviado. No contiene `id="title-error"` y no se crea ninguna tarea.
- `Test_TasksIndex_ShowsDueDate`: con `taskWithDueDate("2026-11-15")`, `GET /` contiene `<time datetime="2026-11-15">15/11/2026</time>`.
- `Test_TasksIndex_NoDueDate`: con `taskForEditing(false)`, `GET /` no contiene `class="task-due"`.
- `Test_TasksEdit_PrefillsDueDate`: `GET /tasks/{id}/edit` contiene `name="due_date"` y `value="2026-11-15"`.
- `Test_TasksUpdate_SetsDueDate`: sobre `taskForEditing(true)`, `PUT` con título y `due_date=2026-12-01` responde 303. Tras recargar, la fecha es `2026-12-01` y la tarea sigue completada.
- `Test_TasksUpdate_ClearsDueDate`: sobre `taskWithDueDate("2026-11-15")`, `PUT` con título y `due_date` vacío responde 303 y la tarea queda sin fecha límite.
- `Test_TasksUpdate_InvalidDueDate`: sobre `taskWithDueDate("2026-11-15")`, `PUT` con `title=Otro título` y `due_date=2026-13-01` responde 422 con el mensaje y `aria-describedby="due-date-error"`. Tras recargar, el título sigue siendo «Con fecha límite» y la fecha `2026-11-15`.
- `Test_TasksToggle_PreservesDueDate`: `PUT /tasks/{id}/toggle` sobre `taskWithDueDate("2026-11-15")` responde 303. Tras recargar, la tarea está completada y conserva la fecha.

Los tests existentes deben seguir pasando sin cambios. En especial, `Test_TasksUpdate_EmptyTitle` sigue encontrando `aria-describedby="title-error"`.

### Comandos

Requisitos: Docker con el servicio `db` de `compose.yaml` saludable (`docker compose up -d db`) y `soda` (`$(go env GOPATH)/bin/soda`; alternativa: `go run github.com/gobuffalo/pop/v6/soda@v6.4.1`).

La base `todo_test` compartida tiene aplicada una migración ajena a esta rama (`20261026180000`, de otra ejecución de esta issue) que ya añadió `due_date`; con ella `soda migrate` fallaría por columna duplicada. Recréala antes de migrar. Solo contiene datos de test, que la suite trunca en cada ejecución. No toques `todo_development`.

```sh
docker compose up -d db
SODA=$(go env GOPATH)/bin/soda
$SODA drop -e test && $SODA create -e test && $SODA migrate -e test
$SODA migrate down -e test && $SODA migrate -e test   # la migración es reversible
docker compose exec -T db psql -U postgres -d todo_test -c '\d tasks'   # due_date | date | (nullable)
gofmt -l .                       # sin salida
go vet ./...
go mod tidy -diff                # sin salida
GO_ENV=test go test -p 1 ./...
go build ./...
```

## Riesgos y recuperación

- **Base de test con migraciones ajenas.** Si `soda migrate -e test` falla con `column "due_date" of relation "tasks" already exists` o aparecen versiones que no existen en `migrations/`, recrea `todo_test` como se indica arriba. No intentes revertir migraciones de otras ramas.
- **Conflicto de versión de migración.** Otras ramas abiertas de esta issue usan versiones distintas (`20261026180000`, `20261026000000`, …) para la misma columna. Si se fusiona más de una, la segunda fallaría. Es un riesgo de integración que se informa en el PR, no algo que resolver aquí.
- **Desfase de día por zona horaria.** `SetDueDate` guarda el día a medianoche en UTC y pgx lo codifica como `date` con el protocolo extendido, así que no hay conversión de zona. Si un test mostrara un día desplazado, comprueba la `TimeZone` de la sesión de PostgreSQL antes de cambiar el diseño, y no añadas hora a la columna.
- **Plush y métodos de receptor por valor.** Si Plush no resolviera `t.DueDateValue()` dentro del `for`, los tests de lista fallarán con un error de renderizado. No cambies el receptor a puntero. Detente e informa.
- **Layout de la lista.** Mover el padding de `.task-title` a `.task-main` es necesario porque un `span` en línea dentro de un `div` ignora el padding vertical. Si la fila de la tarea queda desalineada, revisa que `.task-main` sea hijo directo de `li.task`.
- **Edición sin `due_date`.** Un `PUT` sin el campo quita la fecha límite (decisión de alcance). El formulario de edición siempre lo envía, así que no afecta al uso desde la interfaz.
- **Rama `main` local adelantada.** El `main` local tiene un commit sin publicar (`429e9d6`, arreglo del toggle en `app.js` y `application.plush.html`). Esta rama parte de `origin/main` y solo toca la línea del CSS en `application.plush.html`, así que no se espera conflicto con ese commit.
- **Recuperación.** Todo el cambio está en archivos versionados y en una migración reversible: `soda migrate down -e test` y `git restore`/`git clean` sobre los archivos listados devuelven el estado anterior.
