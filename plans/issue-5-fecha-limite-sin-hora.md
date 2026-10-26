# Feature: fecha límite opcional (solo día) en las tareas

Origen: issue [#5](https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5), «Quiero añadir fecha límite a las tasks». Cuerpo: «Vale con que sea simplemente un día. Sin hora».

Rama de trabajo: `feature/issue-5-fecha-limite-sin-hora`, creada desde `origin/main` (`868fb30`).

## Encargo y criterios de aceptación

Requisitos explícitos:

- Una tarea puede tener una fecha límite.
- La fecha límite es solo un día; no tiene hora.

Inferencias, porque la issue no las detalla:

- La fecha es **opcional**. Las tareas existentes y las nuevas sin fecha siguen siendo válidas.
- Se asigna al crear y se puede cambiar o quitar al editar. Sin edición, un error al crear no tendría arreglo.
- Se muestra en la lista. Si no, la funcionalidad no sería observable.
- Se aceptan fechas pasadas. Una tarea vencida debe poder editarse sin forzar otra fecha.

Criterios de aceptación:

1. El formulario «Añadir tarea» tiene un campo de fecha opcional (`<input type="date" name="due_date">`). Si se deja vacío, la tarea se crea igual que hoy.
2. Al crear una tarea con fecha `2026-10-30`, se guarda en `tasks.due_date` (tipo `date`), y la lista muestra `Fecha límite: 30/10/2026` bajo el título dentro de `<time datetime="2026-10-30">`.
3. Las tareas sin fecha no muestran ninguna línea de fecha en la lista.
4. La página de edición muestra el campo de fecha con el valor actual (`value="2026-10-30"`). Permite cambiar la fecha o dejar el campo vacío para quitarla.
5. Un valor de fecha que no tenga el formato `AAAA-MM-DD` o que no sea una fecha real (por ejemplo `2026-02-30`) no se guarda. La respuesta es `422` con el mensaje `La fecha límite no es válida.` asociado al campo. Este caso solo se da fuera del navegador, porque `type="date"` siempre envía un valor válido o vacío.
6. Completar o reabrir una tarea (`PUT /tasks/{id}/toggle`) no cambia su fecha límite.
7. Las validaciones de título no cambian. Un error de título no marca el campo de fecha como inválido, y un error de fecha no marca el título.

## Comportamiento actual y propuesta

Actual (`origin/main`):

- `tasks` tiene `id`, `title`, `completed`, `created_at` y `updated_at`, según `migrations/20260901000000_create_tasks.up.fizz`.
- `models.Task` no tiene ningún campo de fecha de usuario.
- `TasksCreate` enlaza el formulario con `c.Bind(task)`. `TasksUpdate` enlaza solo `title` en una estructura anónima y conserva el resto.
- `templates/tasks/index.plush.html` y `templates/tasks/edit.plush.html` solo tienen el campo de título. Además, usan `errors.HasAny()` para marcar el título como inválido.

Propuesta: un cambio vertical mínimo siguiendo los patrones existentes.

- Una migración fizz añade una columna `due_date date NULL`.
- `Task.DueDate` es de tipo `nulls.Time`, la convención de Buffalo/pop para columnas nulas. Tiene `form:"-"` para que el binder no lo rellene directamente.
- Un método `SetDueDate(string)` interpreta el valor del formulario. `Validate` informa del error en `due_date`, igual que ya hace con `title`. Así, `ValidateAndCreate` y `ValidateAndUpdate` impiden guardar una fecha inválida sin código extra en las acciones.
- Dos métodos con receptor por valor dan formato a la fecha para las plantillas: `DueDateValue()` (`AAAA-MM-DD`) y `DueDateLabel()` (`DD/MM/AAAA`).
- Las plantillas añaden el campo `type="date"` y la línea de fecha en la lista.

## Alcance

Incluido:

- Migración `up`/`down` de la columna `due_date`.
- Campo, parseo, validación y formato en `models/task.go`.
- Lectura de `due_date` en `TasksCreate` y `TasksUpdate`.
- Campo de fecha en los formularios de creación y edición, y línea de fecha en la lista.
- CSS para el campo de fecha y la línea de fecha, incluido el ajuste móvil del formulario de creación. Cambio de `app.css?v=3` a `?v=4`.
- Tests de modelo y de acciones.
- `go.mod`: `github.com/gobuffalo/nulls` pasa de `// indirect` a dependencia directa (misma versión `v0.4.2`) mediante `go mod tidy`.

Excluido:

- Hora o zona horaria en la fecha límite.
- Destacar tareas vencidas o próximas a vencer.
- Ordenar o filtrar por fecha límite (el orden sigue siendo `created_at asc, id asc`).
- Recordatorios o notificaciones.
- Cambios en `public/assets/app.js`. El toggle por JavaScript sigue funcionando con la nueva estructura (ver «Renderizado»).
- Corregir el envío del toggle por `fetch(form.action)` (GET) en `app.js`. Es un bug conocido de `origin/main`, ya corregido en un commit local de `main` que no está publicado. No es parte de esta issue.
- Cambios en la migración de semillas, en `fixtures/tasks.toml`, en `database.yml`, en CI o en Docker.

## Diseño por capas

### Persistencia y migraciones

Nuevos archivos:

`migrations/20261026000000_add_due_date_to_tasks.up.fizz`:

```
add_column("tasks", "due_date", "date", {"null": true})
```

`migrations/20261026000000_add_due_date_to_tasks.down.fizz`:

```
drop_column("tasks", "due_date")
```

El traductor fizz de PostgreSQL pasa el tipo `date` sin cambios, así que se genera `DATE`. `migrations/embed.go` ya embebe `*.fizz`; no hay que tocarlo. `models.Migrate()` aplica la migración al arrancar la aplicación.

### Modelo (`models/task.go`)

- Importa `github.com/gobuffalo/nulls`.
- Añade constantes:
  - `DueDateLayout = "2006-01-02"`: formato del formulario y del atributo `datetime`.
  - `DueDateDisplayLayout = "02/01/2006"`: formato visible.
- Añade a `Task`, después de `Completed`:
  - `DueDate nulls.Time `json:"due_date" db:"due_date" form:"-"``
  - `invalidDueDate bool `json:"-" db:"-" form:"-"``. Es estado transitorio y no se guarda. La etiqueta `db:"-"` es **obligatoria**: sin ella, pop usa el nombre del campo como columna (`columns.TagsFor`).
- `func (t *Task) SetDueDate(value string)`:
  - Aplica `strings.TrimSpace`.
  - Si el valor queda vacío, pone `t.DueDate = nulls.Time{}` e `invalidDueDate = false`.
  - Si `time.Parse(DueDateLayout, value)` falla, pone `t.DueDate = nulls.Time{}` e `invalidDueDate = true`.
  - Si no falla, pone `t.DueDate = nulls.NewTime(d)` e `invalidDueDate = false`. `time.Parse` sin zona devuelve medianoche UTC, y eso basta para una columna `date`.
- En `Validate`, después de las reglas del título: `if t.invalidDueDate { verrs.Add("due_date", "La fecha límite no es válida.") }`.
- `func (t Task) DueDateValue() string`: devuelve `""` si `!t.DueDate.Valid`; si no, `t.DueDate.Time.Format(DueDateLayout)`.
- `func (t Task) DueDateLabel() string`: igual, pero con `DueDateDisplayLayout`.
- Estos dos métodos usan **receptor por valor**. `index.plush.html` itera `models.Tasks` (`[]Task`) y Plush no puede llamar a métodos con receptor puntero sobre esos valores. No hay que convertir la fecha a la zona local; se formatea tal como se lee.

### Acción y ruta (`actions/tasks.go`)

Las rutas no cambian.

- `TasksCreate`: después de `c.Bind(task)` y de recortar el título, añade `task.SetDueDate(c.Param("due_date"))`. `c.Param` incluye los valores del formulario (`buffalo/context.go` mezcla `req.Form` en los parámetros). Si hay errores, la ruta actual ya vuelve a mostrar `tasks/index.plush.html` con `422` y con la `task` enlazada.
- `TasksUpdate`:
  - Añade `DueDate string `form:"due_date" json:"due_date"`` a la estructura `input`.
  - Después de asignar el título, llama a `task.SetDueDate(input.DueDate)`.
  - Cambia el comentario de la función a `// TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.`.
  - Si no llega `due_date` o llega vacío, se quita la fecha. El formulario siempre envía el campo.
- `TasksToggle` no cambia. `tx.Update(task)` guarda la `DueDate` cargada con `tx.Find`, así que la fecha se conserva.

### Renderizado (Plush)

`templates/tasks/index.plush.html`:

- En cada `<li class="task">`, envuelve el título en un contenedor y añade la fecha:

  ```html
  <div class="task-main">
    <span class="task-title"><%= t.Title %></span>
    <%= if (t.DueDate.Valid) { %>
      <span class="task-due">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></span>
    <% } %>
  </div>
  ```

  El resto del `<li>` no cambia. `app.js` sigue funcionando por dos motivos: `task.querySelector(".task-title")` encuentra el título anidado, y `.task-status` se inserta antes de `.task-actions`, que sigue siendo hijo directo del `<li>`. `.task-status` tiene posición absoluta y no ocupa una celda de la rejilla.
- En el formulario `new-task`, entre el `<input id="task-title">` y el botón «Añadir»:

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

- En el input del título, cambia `<%= if (errors.HasAny()) { %>autofocus aria-invalid="true" aria-describedby="title-error"<% } %>` por la misma condición con `len(errors.Get("title")) > 0`.
- Sustituye el bloque de error. El `<p id="title-error">` pasa a depender de `len(errors.Get("title")) > 0`. Después se añade un `<p id="due-date-error" class="field-error" role="alert">`, con la condición `len(errors.Get("due_date")) > 0`, que recorre `errors.Get("due_date")` igual que el bloque del título.

`templates/tasks/edit.plush.html`:

- En el input del título, cambia la condición `errors.HasAny()` por `len(errors.Get("title")) > 0`, en `aria-invalid`/`aria-describedby` y en el `<p id="title-error">`. Los textos no cambian.
- Después del bloque de error del título y antes de `.edit-task-actions`, añade:

  ```html
  <div class="edit-task-field">
    <label for="task-due-date">Fecha límite <span class="label-optional">(opcional)</span></label>
    <input
      id="task-due-date"
      type="date"
      name="due_date"
      value="<%= task.DueDateValue() %>"
      <%= if (len(errors.Get("due_date")) > 0) { %>aria-invalid="true" aria-describedby="due-date-error"<% } else { %>aria-describedby="due-date-help"<% } %>
    >
    <p id="due-date-help" class="field-hint">Solo el día, sin hora. Déjala vacía para quitarla.</p>
    <%= if (len(errors.Get("due_date")) > 0) { %>
      <p id="due-date-error" class="field-error" role="alert">
        <%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %>
      </p>
    <% } %>
  </div>
  ```

`templates/application.plush.html`: cambia `/assets/app.css?v=3` por `/assets/app.css?v=4`.

### CSS (`public/assets/app.css`)

- Cambia `.task-title` a `display: block; overflow-wrap: anywhere; line-height: 1.55;`. Mueve `align-self: center; min-width: 0; padding: 0.65rem 0;` a una nueva regla `.task-main`, que ocupa la columna central de la rejilla.
- Añade `.task-due { display: block; margin-top: 0.1rem; color: var(--color-muted); font-size: 0.8rem; }`.
- El selector `input[type="text"]` pasa a ser `input[type="text"], input[type="date"]`, con las mismas declaraciones. Después añade `input[type="date"] { width: auto; }`.
- El selector `.new-task-row input { flex: 1; }` pasa a ser `.new-task-row input[type="text"] { flex: 1; }`.
- Añade `.edit-task-field { margin-top: 1.25rem; }` y `.label-optional { color: var(--color-muted); font-weight: 400; }`.
- En `@media (max-width: 760px)`, añade:

  ```css
  .new-task-row {
    flex-wrap: wrap;
  }

  .new-task-row input[type="text"] {
    flex-basis: calc(100% - 32px - 0.4rem);
  }

  .new-task-row input[type="date"] {
    margin-left: calc(32px + 0.4rem);
  }

  .new-task-row .button {
    margin-left: auto;
  }
  ```

  El `.add-mark` ocupa 32px en móvil (36px de ancho con `margin-left: -4px`). Así, el título queda en la primera línea, y la fecha y «Añadir» en la segunda, con la fecha alineada con el título.
- En `@media (max-width: 380px)`, elimina las tres reglas de `.new-task-row` (`flex-wrap`, `input { flex-basis: calc(100% - 44px) }` y `.button { margin-left: auto }`). Quedan sustituidas por las de 760px, y la regla `input` ensancharía también el campo de fecha. Se conserva `.container { padding-inline: 0.8rem; }`.

## Archivos relevantes

| Ruta | Responsabilidad |
| --- | --- |
| `migrations/20261026000000_add_due_date_to_tasks.up.fizz` | Nuevo: añade `tasks.due_date date NULL`. |
| `migrations/20261026000000_add_due_date_to_tasks.down.fizz` | Nuevo: elimina la columna. |
| `migrations/embed.go` | Embebe `*.fizz`; no cambia. |
| `models/task.go` | Campo `DueDate`, `SetDueDate`, validación y formato. |
| `models/task_test.go` | Tests de modelo de la fecha límite. |
| `actions/tasks.go` | `TasksCreate` y `TasksUpdate` leen `due_date`. |
| `actions/tasks_test.go` | Tests de creación, listado, edición, actualización y toggle con fecha. |
| `templates/tasks/index.plush.html` | Línea de fecha en la lista y campo en el formulario de creación. |
| `templates/tasks/edit.plush.html` | Campo de fecha precargado y su error. |
| `templates/application.plush.html` | Versión de caché de `app.css`. |
| `public/assets/app.css` | Estilos del campo y de la línea de fecha. |
| `public/assets/app.js` | No cambia; depende de `.task-title`, `.task-actions` y `.task-status`. |
| `go.mod` | `gobuffalo/nulls` pasa a dependencia directa. |

## Implementación paso a paso

1. Crea las dos migraciones fizz con el contenido indicado.
2. Modifica `models/task.go`: importa `nulls`, añade las constantes, los campos `DueDate` e `invalidDueDate`, `SetDueDate`, la regla en `Validate`, `DueDateValue` y `DueDateLabel`.
3. Ejecuta `go mod tidy`. Comprueba que el único cambio en `go.mod` es mover `github.com/gobuffalo/nulls v0.4.2` al bloque directo y que `go.sum` no cambia. Si cambia cualquier otra cosa, detente.
4. Modifica `actions/tasks.go` en `TasksCreate` y `TasksUpdate` como se describe, incluido el comentario de `TasksUpdate`.
5. Modifica `templates/tasks/index.plush.html`, `templates/tasks/edit.plush.html` y `templates/application.plush.html`.
6. Modifica `public/assets/app.css`.
7. Añade los tests de la sección siguiente.
8. Ejecuta la validación.

## Tests y validación

### Tests de modelo (`models/task_test.go`)

- `Test_Task_DueDate_OptionalByDefault`: crea `Task{Title: "Sin fecha"}`, vuelve a leerla con `ms.DB.Find` y comprueba `!DueDate.Valid` y `DueDateValue() == ""`.
- `Test_Task_SetDueDate_Persists`: llama a `SetDueDate(" 2026-10-30 ")`, crea la tarea y la vuelve a leer. Comprueba `DueDate.Valid`, `DueDateValue() == "2026-10-30"` y `DueDateLabel() == "30/10/2026"`. Sirve también de prueba de ida y vuelta contra la columna `date`, sin desfase de zona horaria.
- `Test_Task_SetDueDate_Empty_ClearsDate`: asigna una fecha válida y después `SetDueDate("")`. Comprueba `!DueDate.Valid` y que `ValidateAndCreate` no devuelve errores.
- `Test_Task_SetDueDate_Invalid`: con una tabla de casos `"30/10/2026"`, `"2026-02-30"` y `"mañana"`, comprueba que `ValidateAndCreate` devuelve `verrs.Get("due_date")` con `La fecha límite no es válida.` y que el recuento de tareas es `0`.

### Tests de acciones (`actions/tasks_test.go`)

Usa `url.Values` para enviar `due_date`, igual que los tests de `TasksUpdate`.

- `Test_TasksCreate_WithDueDate`: `POST /tasks` con `title` y `due_date=2026-10-30` devuelve `303`. La primera tarea tiene `DueDateValue() == "2026-10-30"`.
- `Test_TasksCreate_Valid` (existente): añade `as.False(task.DueDate.Valid)`.
- `Test_TasksCreate_InvalidDueDate`: `POST` con `title=Con fecha mala` y `due_date=2026-13-01` devuelve `422`.
  - El cuerpo contiene `La fecha límite no es válida.`, `aria-describedby="due-date-error"` y `value="Con fecha mala"`.
  - El cuerpo no contiene `aria-describedby="title-error"`.
  - El recuento es `0`.
- `Test_TasksCreate_EmptyTitle_KeepsDueDate`: `POST` con `title=""` y `due_date=2026-10-30` devuelve `422`.
  - El cuerpo contiene `value="2026-10-30"` y `El título es obligatorio.`.
  - El cuerpo no contiene `due-date-error`.
- `Test_TasksIndex_ShowsDueDate`: crea una tarea con fecha y otra sin fecha. `GET /` contiene `datetime="2026-10-30"` y `30/10/2026`, y `strings.Count(body, `class="task-due"`)` es `1`.
- `Test_TasksEdit_PrefillsDueDate`: la página de edición de una tarea con fecha contiene `name="due_date"` y `value="2026-10-30"`.
- `Test_TasksUpdate_ChangesDueDate`: `PUT` con `title` y `due_date=2026-11-05` devuelve `303`. Al volver a leer, `DueDateValue() == "2026-11-05"`.
- `Test_TasksUpdate_ClearsDueDate`: `PUT` con `title` y `due_date=""` devuelve `303`. Al volver a leer, `!DueDate.Valid`.
- `Test_TasksUpdate_InvalidDueDate`: `PUT` con `title=Otro título` y `due_date=no-es-fecha` devuelve `422`.
  - El cuerpo contiene `La fecha límite no es válida.` y `aria-describedby="due-date-error"`.
  - Al volver a leer, el título sigue siendo `Título original` y la fecha sigue siendo la original.
- `Test_TasksToggle_KeepsDueDate`: después de un `PUT /tasks/{id}/toggle`, `Completed` es `true` y `DueDateValue()` no cambia.
- Los tests existentes deben seguir pasando sin cambios, salvo la línea añadida en `Test_TasksCreate_Valid`. En particular, `Test_TasksUpdate_EmptyTitle` comprueba `aria-describedby="title-error"`.

### Comandos

Estándar, con PostgreSQL en `${DB_HOST:-127.0.0.1}:5432` y la base `todo_test` disponible:

```sh
gofmt -l .                      # sin salida
go vet ./...
go mod tidy -diff               # sin diferencias tras el paso 3
soda migrate -e test            # aplica add_due_date_to_tasks
soda migrate down -e test && soda migrate -e test   # reversibilidad de la migración
GO_ENV=test go test -p 1 ./...
go build ./...
```

`soda` está en `~/go/bin/soda`. Si no está en el `PATH`, usa esa ruta.

Entorno observado durante la planificación: el puerto `5432` local está ocupado por el contenedor `desarrollo-agentico-todo-04-db-1`, de **otro proyecto**, que no tiene la base `todo_test`. **No lo modifiques ni crees bases en él.** En su lugar:

1. Arranca un PostgreSQL temporal en un puerto libre:
   `docker run -d --rm --name todo03-issue5-test-db -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=todo_test -p 127.0.0.1:<PUERTO>:5432 postgres:17.6-alpine`
   Espera hasta que `docker exec todo03-issue5-test-db pg_isready -U postgres` responda.
2. Crea un directorio temporal (por ejemplo `/tmp/todo03-issue5-db`) con una copia de `database.yml` donde `test` tenga `host: 127.0.0.1` y `port: <PUERTO>`. No cambies el `database.yml` del repositorio.
3. Ejecuta `soda migrate -e test -c /tmp/todo03-issue5-db/database.yml -p migrations` y la prueba de reversibilidad con los mismos parámetros.
4. Ejecuta `GO_ENV=test POP_PATH=/tmp/todo03-issue5-db go test -p 1 ./...`. Pop antepone `POP_PATH` a sus rutas de búsqueda, así que usa la copia temporal.
5. Al terminar, ejecuta `docker stop todo03-issue5-test-db` y borra el directorio temporal.

### Validación manual (recomendada)

Con el mismo PostgreSQL temporal:

1. Ejecuta `go build -o /tmp/todo03-issue5-app ./cmd/app` y arranca la aplicación con `GO_ENV=test POP_PATH=/tmp/todo03-issue5-db PORT=<puerto libre> /tmp/todo03-issue5-app`.
2. Comprueba en el navegador:
   - crear una tarea con fecha y otra sin fecha;
   - la línea `Fecha límite: DD/MM/AAAA`;
   - que la edición precarga la fecha, la cambia y la quita;
   - la disposición del formulario de creación a 375px y a 1280px.

Este paso no sustituye a los tests. Si no puede ejecutarse, indícalo en el reporte.

## Riesgos y recuperación

- **Zona horaria**: `time.Parse` devuelve medianoche UTC. pgx codifica las fechas con los componentes año/mes/día de la propia `time.Time`, y al leer devuelve medianoche UTC. El formato no convierte a hora local. `Test_Task_SetDueDate_Persists` detecta cualquier desfase de un día.
- **Campo transitorio `invalidDueDate`**: si se omite `db:"-"`, pop intentará leer y escribir una columna `invalidDueDate` y los tests fallarán con un error de SQL. La recuperación es añadir la etiqueta.
- **Plush y receptores**: si `DueDateValue`/`DueDateLabel` usan receptor puntero, el listado falla al renderizar. Deben usar receptor por valor.
- **Cliente que no envía `due_date` en `PUT`**: se quita la fecha. Es aceptable porque el único cliente es el formulario, que siempre envía el campo. Queda documentado en `TasksUpdate`.
- **Navegadores sin `type="date"`**: escribirían texto libre. Un valor distinto de `AAAA-MM-DD` recibe el error de validación y no se guarda nada. Todos los navegadores actuales soportan el campo.
- **Caché de CSS**: si no se cambia a `?v=4`, los navegadores con la hoja anterior en caché verían el campo sin estilo.
- **Toggle desde la interfaz en `origin/main`**: `app.js` usa `fetch(form.action)` (GET), así que el cambio de estado no persiste desde la UI. Es un bug previo y queda fuera del alcance. La conservación de la fecha en el toggle se comprueba con el `PUT` real de `Test_TasksToggle_KeepsDueDate`.
- **Trabajo paralelo**: ya hay PR abiertos para la misma issue (#6 a #10). Solo debe integrarse una implementación. Puede haber conflictos con ellos y con el arreglo del toggle pendiente de publicar.
- **Recuperación de datos**: la migración `down` elimina la columna y las fechas guardadas. Es reversible en esquema, pero con pérdida de esas fechas.
