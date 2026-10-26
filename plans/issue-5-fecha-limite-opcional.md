# Feature: fecha límite opcional (solo día) en las tareas

## Encargo y criterios de aceptación

Origen: issue [#5](https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5), «Quiero añadir fecha límite a las tasks». Cuerpo: «Vale con que sea simplemente un día. Sin hora».

Requisitos explícitos:

- Una tarea puede tener una fecha límite.
- La fecha límite es un día, sin hora.

Inferencias (decididas en este plan):

- La fecha límite es **opcional**. Las tareas existentes y las nuevas sin fecha siguen siendo válidas.
- Se puede fijar al crear la tarea y cambiar o quitar al editarla.
- La lista muestra la fecha límite de las tareas que la tienen.

Criterios de aceptación:

1. El formulario «Añadir tarea» tiene un campo de fecha visible y etiquetado, «Fecha límite (opcional)» (`<input type="date" name="due_date">`). Si se rellena, la tarea se crea con ese día. Si se deja vacío, se crea sin fecha.
2. Cada tarea con fecha muestra en la lista `Fecha límite: DD/MM/AAAA` dentro de `<time datetime="AAAA-MM-DD">`. Las tareas sin fecha no muestran nada nuevo.
3. La pantalla de edición tiene el mismo campo, precargado con la fecha actual (`value="AAAA-MM-DD"`). Al guardar se puede cambiar la fecha o quitarla dejando el campo vacío. El estado completada o pendiente nunca cambia al editar.
4. Una fecha que no sea un día válido con formato `AAAA-MM-DD` (por ejemplo `2026-02-30`, `15/11/2026` o `2026-11-15T10:00`) produce un 422 con el mensaje «La fecha límite no es válida.» y no se crea ni modifica nada. Solo el campo de fecha queda marcado como inválido.
5. Si el título es inválido y la fecha es válida, el 422 conserva la fecha introducida en el campo.
6. La base de datos guarda la fecha en una columna `due_date` de tipo `date` que admite `NULL`. La migración es reversible.

## Comportamiento actual y propuesta

**Actual.** `Task` (`models/task.go`) solo tiene `ID`, `Title`, `Completed`, `CreatedAt` y `UpdatedAt`. `TasksCreate` enlaza el formulario sobre `models.Task`. `TasksUpdate` enlaza un struct anónimo que solo tiene `Title`. Las plantillas solo muestran o editan el título. Los errores de validación marcan el campo de título como inválido siempre que haya cualquier error (`errors.HasAny()`).

**Propuesta.** Se añade la columna nullable `due_date date` y el campo `DueDate *time.Time` en el modelo, con `form:"-"`. La fecha llega del formulario como texto y se parsea de forma explícita con `models.ParseDueDate` usando el formato `2006-01-02`.

No se usa el binder de Buffalo para la fecha por dos motivos:

- Acepta también formatos con hora (`2006-01-02T15:04`, RFC3339…).
- Con un valor inválido devuelve un error de `c.Bind`, y eso acaba en un 500.

Además, el decoder de `nulls.Time` del binder usa el paquete interno `github.com/gobuffalo/buffalo/internal/nulls`, no `github.com/gobuffalo/nulls`.

El modelo expone dos helpers de presentación con receptor por valor, para que Plush pueda llamarlos tanto sobre `Task` como sobre `*Task`. Los errores pasan a mostrarse por campo.

## Alcance

Incluido:

- Migración fizz que añade y quita `tasks.due_date` (`date`, nullable).
- Campo `DueDate`, constante de formato, mensaje de error, `ParseDueDate` y helpers `DueDateValue()` y `DueDateLabel()` en `models/task.go`.
- Lectura y validación de `due_date` en `TasksCreate` y `TasksUpdate`.
- Campo de fecha en los formularios de creación y edición, y visualización en la lista.
- Errores por campo (`title` y `due_date`) en ambos formularios.
- Estilos mínimos para el campo de fecha y la línea de fecha en la lista; subir la versión de caché de `app.css`.
- Tests de modelo y de acciones.

Excluido:

- Ordenar o filtrar por fecha límite.
- Resaltar tareas vencidas o próximas a vencer: requiere decidir la zona horaria de «hoy».
- Hora, recordatorios y notificaciones.
- Cambios en `public/assets/app.js`, en los fixtures o en la migración de seed.
- Hacer obligatoria la fecha.

## Diseño por capas

### Persistencia y migraciones

Archivos nuevos:

- `migrations/20261026000000_add_due_date_to_tasks.up.fizz`:
  ```
  add_column("tasks", "due_date", "date", {"null": true})
  ```
- `migrations/20261026000000_add_due_date_to_tasks.down.fizz`:
  ```
  drop_column("tasks", "due_date")
  ```

`migrations/embed.go` ya incluye `*.fizz`, así que la aplicación aplica la migración al arrancar mediante `models.Migrate()`. La CI la aplica con `soda migrate -e test`.

### Modelo (`models/task.go`)

- Añadir el campo entre `Completed` y `CreatedAt`:
  ```go
  DueDate   *time.Time `json:"due_date" db:"due_date" form:"-"`
  ```
  `nil` equivale a `NULL`, es decir, sin fecha. Se usa un puntero para no añadir dependencias directas a `go.mod`.
- Añadir:
  ```go
  // DueDateLayout es el formato AAAA-MM-DD con el que se envía y se recibe la fecha límite.
  const DueDateLayout = "2006-01-02"

  // DueDateInvalidMessage es el error que se muestra si la fecha límite no es un día válido.
  const DueDateInvalidMessage = "La fecha límite no es válida."

  // ParseDueDate interpreta el valor de un campo <input type="date">. Un valor vacío
  // significa que la tarea no tiene fecha límite; ok es false si el valor no es un día válido.
  func ParseDueDate(value string) (dueDate *time.Time, ok bool)

  // DueDateValue devuelve la fecha límite en formato AAAA-MM-DD, o "" si no tiene.
  func (t Task) DueDateValue() string

  // DueDateLabel devuelve la fecha límite en formato DD/MM/AAAA, o "" si no tiene.
  func (t Task) DueDateLabel() string
  ```
  `ParseDueDate` aplica `strings.TrimSpace`. Con un valor vacío devuelve `(nil, true)`. Si no, llama a `time.Parse(DueDateLayout, value)`: si falla devuelve `(nil, false)` y si funciona devuelve `(&d, true)`. El resultado es medianoche UTC del día indicado.
- `Validate` no cambia.

### Acciones (`actions/tasks.go`)

No cambian las rutas: se reutilizan `POST /tasks` y `PUT /tasks/{task_id}`.

`TasksCreate`, después de `c.Bind(task)` y del `TrimSpace` del título:

```go
dueDate, ok := models.ParseDueDate(c.Param("due_date"))
task.DueDate = dueDate

verrs := validate.NewErrors()
if !ok {
	verrs.Add("due_date", models.DueDateInvalidMessage)
} else {
	var err error
	verrs, err = tx.ValidateAndCreate(task)
	if err != nil {
		return err
	}
}

if verrs.HasAny() {
	// …rama existente sin cambios: recargar tareas, c.Set y 422 con tasks/index.plush.html
}
```

En `TasksUpdate`:

- Añadir el campo ``DueDate string `form:"due_date" json:"due_date"` `` al struct anónimo `input`.
- Tras asignar el título, aplicar el mismo patrón: `ParseDueDate(input.DueDate)`. Si el valor no es válido, añadir el error en `verrs` y no llamar a `ValidateAndUpdate`. Si es válido, asignar `task.DueDate` y llamar a `ValidateAndUpdate`.
- La rama de 422 existente no cambia.
- Ajustar el comentario de la función: «actualiza el título y la fecha límite, conservando los demás datos de la tarea».

Semántica: un `due_date` vacío o ausente deja la tarea sin fecha. Los formularios HTML siempre envían el campo.

Con una fecha inválida, el 422 solo muestra el error de la fecha; el título se valida en el siguiente envío. Ese caso solo se da con peticiones manipuladas, porque `<input type="date">` envía `""` o `AAAA-MM-DD`.

### Renderizado (Plush)

`templates/tasks/index.plush.html`, dentro de cada `<li class="task">`:

- Envolver el título en un contenedor y añadir la fecha:
  ```html
  <div class="task-main">
    <span class="task-title"><%= t.Title %></span>
    <%= if (t.DueDateLabel() != "") { %>
      <span class="task-due">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></span>
    <% } %>
  </div>
  ```
  `.task-status` y `.task-actions` siguen siendo hermanos de `.task-main`, en el mismo orden. `app.js` sigue funcionando: usa `.task-title` para el `aria-label` e inserta `.task-status` antes de `.task-actions`.

`templates/tasks/index.plush.html`, formulario `new-task`:

- Input de título: cambiar la condición `errors.HasAny()` por `len(errors.Get("title")) > 0` para `autofocus aria-invalid="true" aria-describedby="title-error"`.
- Después de `.new-task-row`, añadir:
  ```html
  <div class="new-task-due">
    <label for="task-due-date">Fecha límite (opcional)</label>
    <input
      id="task-due-date"
      type="date"
      name="due_date"
      value="<%= task.DueDateValue() %>"
      <%= if (len(errors.Get("due_date")) > 0) { %>aria-invalid="true" aria-describedby="due-date-error"<% } %>
    >
  </div>
  ```
- Mensajes de error por campo:
  - `<p id="title-error" class="field-error" role="alert">` solo si `len(errors.Get("title")) > 0`. Su contenido no cambia.
  - `<p id="due-date-error" class="field-error" role="alert">` con los mensajes de `errors.Get("due_date")`, solo si los hay.

`templates/tasks/edit.plush.html`:

- Input de título:
  - `aria-invalid` y `aria-describedby="title-error"` solo si `len(errors.Get("title")) > 0`; si no, `aria-describedby="title-help"`.
  - El bloque `title-error` se muestra con la misma condición.
  - Cambiar el texto de `title-help` a «Hasta 200 caracteres.».
- Tras el bloque del título y antes de `.edit-task-actions`, añadir:
  ```html
  <div class="edit-task-field">
    <label for="task-due-date">Fecha límite (opcional)</label>
    <input
      id="task-due-date"
      type="date"
      name="due_date"
      value="<%= task.DueDateValue() %>"
      <%= if (len(errors.Get("due_date")) > 0) { %>aria-invalid="true" aria-describedby="due-date-error"<% } else { %>aria-describedby="due-date-help"<% } %>
    >
    <p id="due-date-help" class="field-hint">Un día, sin hora. Déjala vacía para quitarla. El estado de la tarea no cambia.</p>
    <%= if (len(errors.Get("due_date")) > 0) { %>
      <p id="due-date-error" class="field-error" role="alert">
        <%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %>
      </p>
    <% } %>
  </div>
  ```

`templates/application.plush.html`: cambiar `app.css?v=3` por `app.css?v=4`. No tocar la línea de `app.js`.

### CSS (`public/assets/app.css`)

- Mover `align-self: center;`, `min-width: 0;` y `padding: 0.65rem 0;` de `.task-title` a una regla nueva `.task-main`. `.task-title` queda con `display: block; overflow-wrap: anywhere; line-height: 1.55;`.
- Añadir:
  ```css
  .task-due {
    display: block;
    color: var(--color-muted);
    font-size: 0.8rem;
  }
  ```
- Añadir `input[type="date"]` al selector de `input[type="text"]` para compartir el estilo base, y justo después:
  ```css
  input[type="date"] {
    width: auto;
  }
  ```
  El foco ya lo cubre la regla existente `input:focus-visible`.
- Añadir:
  ```css
  .new-task-due {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.6rem;
    margin-top: 0.5rem;
    padding-left: calc(44px + 0.6rem);
  }

  .new-task-due label {
    color: var(--color-muted);
    font-size: 0.8rem;
  }

  .edit-task-field {
    margin-top: 1.25rem;
  }
  ```
- En `@media (max-width: 760px)`, añadir `.new-task-due { padding-left: calc(36px + 0.4rem - 4px); }`, que alinea con el input de título en móvil.
- No cambiar las reglas `.new-task-row input`: el campo de fecha no está dentro de `.new-task-row`.

## Archivos relevantes

| Ruta | Responsabilidad |
| --- | --- |
| `migrations/20261026000000_add_due_date_to_tasks.up.fizz` | Nuevo. Añade `due_date date NULL`. |
| `migrations/20261026000000_add_due_date_to_tasks.down.fizz` | Nuevo. Quita `due_date`. |
| `migrations/embed.go` | Embebe `*.fizz` y `*.sql` (sin cambios). |
| `models/task.go` | Campo `DueDate`, formato, mensaje, `ParseDueDate`, `DueDateValue` y `DueDateLabel`. |
| `models/task_test.go` | Tests de parseo y persistencia de la fecha. |
| `actions/tasks.go` | Lectura y validación de `due_date` en creación y edición. |
| `actions/tasks_test.go` | Tests HTTP de crear, listar, editar y actualizar con fecha. |
| `actions/app.go` | Rutas (sin cambios). |
| `templates/tasks/index.plush.html` | Fecha en la lista; campo de fecha y errores por campo en el formulario de alta. |
| `templates/tasks/edit.plush.html` | Campo de fecha y errores por campo en la edición. |
| `templates/application.plush.html` | Versión de caché de `app.css`. |
| `public/assets/app.css` | Estilos de `.task-main`, `.task-due`, `.new-task-due` (y su `label`), `.edit-task-field` y `input[type="date"]`. |
| `public/assets/app.js` | Toggle en cliente; depende de `.task-title`, `.task-status` y `.task-actions` (sin cambios). |
| `fixtures/tasks.toml`, `migrations/20260901000001_seed_tasks.*.sql` | Sin cambios: la columna nueva es nullable. |

## Implementación paso a paso

1. Crear las dos migraciones fizz con el contenido indicado.
2. En `models/task.go`, añadir el campo `DueDate`, las constantes, `ParseDueDate`, `DueDateValue` y `DueDateLabel`. El formato de `DueDateLabel` es `"02/01/2006"`. Mantener el orden y el estilo de comentarios en español.
3. En `actions/tasks.go`, modificar `TasksCreate` y `TasksUpdate` como se describe en «Acciones». No cambiar `TasksIndex`, `TasksEdit`, `TasksToggle` ni `TasksDestroy`.
4. Actualizar `templates/tasks/index.plush.html`: contenedor `.task-main` con la fecha, campo `.new-task-due` y errores por campo.
5. Actualizar `templates/tasks/edit.plush.html`: errores por campo, texto de `title-help` y bloque `.edit-task-field`.
6. Actualizar `public/assets/app.css` y subir `app.css?v=4` en `templates/application.plush.html`.
7. Añadir los tests descritos en «Tests y validación».
8. Ejecutar `gofmt -w` sobre los `.go` modificados y la validación completa.

## Tests y validación

### Tests de modelo (`models/task_test.go`, métodos de `ModelSuite`)

- `Test_ParseDueDate`:
  - `""` y `"   "` devuelven `(nil, true)`.
  - `"2026-11-15"` devuelve `ok == true` y una fecha con año 2026, mes 11 y día 15.
  - `"2026-02-30"`, `"15/11/2026"`, `"2026-11-15T10:00"` y `"mañana"` devuelven `(nil, false)`.
- `Test_Task_DueDate_Optional`: una tarea creada sin fecha, recargada con `Find`, tiene `DueDate == nil`, `DueDateValue() == ""` y `DueDateLabel() == ""`.
- `Test_Task_DueDate_Persists`: crear con `ParseDueDate("2026-11-15")` y recargar con `Find`. Debe quedar `DueDateValue() == "2026-11-15"` y `DueDateLabel() == "15/11/2026"`.

### Tests de acciones (`actions/tasks_test.go`, métodos de `ActionSuite`)

Usar `url.Values` en los POST y PUT que llevan `due_date`. Los `Post(&models.Task{…})` no envían la fecha porque el campo tiene `form:"-"`.

- `Test_TasksCreate_WithDueDate`: POST con `title` y `due_date=2026-11-15` responde 303. La tarea guardada tiene `DueDateValue() == "2026-11-15"`.
- `Test_TasksCreate_EmptyDueDate`: POST con `due_date=""` responde 303 y deja `DueDate == nil`.
- `Test_TasksCreate_InvalidDueDate`: POST con `due_date=2026-02-30` responde 422. El cuerpo contiene «La fecha límite no es válida.» y `aria-describedby="due-date-error"`, y no se crea ninguna tarea.
- `Test_TasksCreate_EmptyTitle_KeepsDueDate`: POST con `title=""` y `due_date=2026-11-15` responde 422. El cuerpo contiene «El título es obligatorio.» y `value="2026-11-15"`.
- `Test_TasksIndex_ShowsDueDate`: con una tarea con fecha y otra sin ella, GET `/` contiene `datetime="2026-11-15"` y `15/11/2026`. Las tareas sin fecha no añaden texto `Fecha límite:`: contar exactamente una aparición con `strings.Count`.
- `Test_TasksEdit_PrefillsDueDate`: GET de edición de una tarea con fecha contiene `type="date"` y `value="2026-11-15"`.
- `Test_TasksUpdate_ChangesDueDate`: PUT de una tarea completada con `title` y `due_date=2026-12-01` responde 303. La tarea queda con la fecha nueva y `Completed` sigue en `true`.
- `Test_TasksUpdate_ClearsDueDate`: PUT con `due_date=""` sobre una tarea con fecha responde 303 y deja `DueDate == nil`.
- `Test_TasksUpdate_InvalidDueDate`: PUT con `title=Otro título` y `due_date=no-es-fecha` responde 422 con el mensaje. Al recargar, el título y la fecha originales no han cambiado.

Los tests existentes deben seguir pasando sin modificaciones; por ejemplo, `Test_TasksUpdate_EmptyTitle` sigue comprobando `aria-describedby="title-error"`.

### Comandos

En el host:

- `gofmt -l .` (sin salida)
- `go vet ./...`
- `go build ./...`

Los tests necesitan PostgreSQL con las migraciones aplicadas. En esta máquina el puerto `5432` está ocupado por la base de datos de otro proyecto (`desarrollo-agentico-todo-07-db-1`). **No** se deben ejecutar migraciones ni tests contra ella. Como `database.yml` fija el puerto 5432 y solo permite cambiar `DB_HOST`, los tests se ejecutan en Docker con una red aislada y efímera:

```sh
docker network create todo03-issue5
docker run -d --rm --name todo03-issue5-db --network todo03-issue5 \
  -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=todo_test \
  postgres:17.6-alpine
# esperar a que acepte conexiones: docker exec todo03-issue5-db pg_isready -U postgres
docker run --rm --network todo03-issue5 \
  -e GO_ENV=test -e DB_HOST=todo03-issue5-db \
  -v "$PWD":/src -w /src golang:1.26 sh -c '
    go install github.com/gobuffalo/pop/v6/soda@v6.4.1 &&
    soda migrate -e test &&
    soda migrate down -e test &&
    soda migrate -e test &&
    go test -p 1 ./...'
docker stop todo03-issue5-db && docker network rm todo03-issue5
```

La secuencia up, down, up comprueba que la migración es reversible. Si el entorno ya ofrece un PostgreSQL de test propio de este repositorio en `127.0.0.1:5432`, se puede usar `GO_ENV=test soda migrate -e test && go test -p 1 ./...` directamente.

Comprobación manual opcional: arrancar la app con `/env:start`, crear una tarea con fecha, editarla, quitar la fecha y comprobar el diseño en móvil (≤380px).

## Riesgos y recuperación

- **Zona horaria al guardar un `date`.** `ParseDueDate` devuelve medianoche UTC, y pgx codifica `time.Time` en una columna `date` por año, mes y día. `Test_Task_DueDate_Persists` detecta cualquier desplazamiento de día. Si fallara, normalizar en `DueDateValue` y `DueDateLabel` con `t.DueDate.UTC()` y revisar el parámetro antes de cambiar el tipo de columna.
- **Errores por campo.** Cambiar `errors.HasAny()` por condiciones por campo podría dejar sin marcar el input de título. Lo cubren los tests existentes (`aria-describedby="title-error"`) y los nuevos de fecha.
- **Diseño de la fila de tarea.** El contenedor `.task-main` pasa a ocupar la columna del título en el grid. Si el diseño se rompe, la recuperación es revertir los cambios de `app.css` y de la plantilla de la lista, que son independientes del resto.
- **Fecha inválida enmascara el error de título.** Es una limitación aceptada y solo se da con peticiones manipuladas.
- **Base de datos compartida.** Ejecutar migraciones contra el PostgreSQL en `127.0.0.1:5432` alteraría la base de datos de otro proyecto. Por eso hay que usar el contenedor aislado.
- **Integración con `main` local.** La rama parte de `origin/main`. El `main` local tiene un commit sin publicar (`429e9d6`) que añade tests al final de `actions/tasks_test.go` y cambia `app.js?v=` en el layout. Al integrar puede haber un conflicto trivial en `actions/tasks_test.go`; hay que conservar ambos bloques de tests.
- **PR duplicados.** La issue #5 ya tiene PR abiertos (#6, #7 y #8) de ejecuciones anteriores. Solo uno debe integrarse.
- **Recuperación general.** Todo el cambio es aditivo. `soda migrate down` elimina la columna, y revertir el commit deja la aplicación como antes.
