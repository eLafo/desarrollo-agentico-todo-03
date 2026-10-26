# Feature: fecha límite (solo día, sin hora) en las tareas

## Encargo y criterios de aceptación

**Encargo:** issue #5 (https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5), «Quiero añadir fecha límite a las tasks». Cuerpo: «Vale con que sea simplemente un día. Sin hora».

### Requisitos explícitos

- Una tarea puede tener una fecha límite.
- La fecha límite es un día. No tiene hora.

### Inferencias (decisiones de este plan)

- La fecha límite es **opcional**. Las tareas actuales y las nuevas sin fecha siguen siendo válidas.
- Se puede fijar al **crear** la tarea y **cambiar o quitar** al editarla.
- La lista muestra la fecha límite de cada tarea que la tenga.
- No se restringen fechas pasadas: registrar una tarea ya vencida es legítimo.

### Criterios de aceptación

1. El formulario de creación de `/` tiene un campo de fecha opcional (`<input type="date" name="due_date">`). Con un día válido, la tarea se crea con esa fecha límite. Si se deja vacío, se crea sin fecha límite.
2. En la lista, una tarea con fecha límite muestra `Fecha límite: <time datetime="AAAA-MM-DD">DD/MM/AAAA</time>`, por ejemplo `Fecha límite: 15/11/2026`. Una tarea sin fecha límite no muestra nada nuevo.
3. `GET /tasks/{id}/edit` muestra un campo «Fecha límite» rellenado con la fecha actual (`value="AAAA-MM-DD"`), o vacío si no tiene.
4. `PUT /tasks/{id}` guarda la fecha enviada, o la quita si llega vacía. Conserva `completed`, `id` y `created_at`, igual que hoy.
5. Un valor que no sea un día válido en formato `AAAA-MM-DD` responde **422**, por ejemplo `2026-02-30`, `30/11/2026` o `2026-11-15T10:00`. La respuesta muestra «La fecha límite debe ser un día válido.» asociado al campo de fecha (`aria-describedby="due-date-error"`) y no guarda nada. Con un título válido, el campo de título no se marca como inválido.
6. Completar o reabrir una tarea (`PUT /tasks/{id}/toggle`) no cambia su fecha límite.
7. La fecha se guarda como `DATE` nullable en PostgreSQL, y la migración se puede revertir.

## Comportamiento actual y propuesta

### Actual

- `migrations/20260901000000_create_tasks.up.fizz` crea `tasks` con `id`, `title` (200), `completed`, `created_at` y `updated_at`. No hay ninguna fecha de vencimiento.
- `models/task.go` define `Task` con esos campos y una única validación (`Validate`) sobre el título.
- `actions/tasks.go`:
  - `TasksCreate` hace `c.Bind(task)`, recorta el título y llama a `tx.ValidateAndCreate`. Con errores, vuelve a renderizar `tasks/index.plush.html` con 422.
  - `TasksUpdate` enlaza un struct `input` que solo tiene `Title`, para no tocar otros campos. Después llama a `tx.ValidateAndUpdate` y, con errores, renderiza `tasks/edit.plush.html` con 422.
  - `TasksToggle` usa `tx.Update(task)`, sin validar.
- Plantillas:
  - `templates/tasks/index.plush.html` pinta cada tarea como `<li class="task">` (rejilla de 3 columnas: toggle, título y acciones). El formulario de creación solo tiene `title`.
  - En ambas plantillas, si hay cualquier error (`errors.HasAny()`), el input de título se marca como inválido y solo se pintan los errores de `title`.
- `public/assets/app.js` usa `task.querySelector(".task-title").textContent` para el `aria-label` del toggle. Por eso `.task-title` debe contener solo el título.

### Propuesta (cambio vertical mínimo)

- **Persistencia:** nueva migración fizz que añade `due_date` de tipo `date`, nullable.
- **Modelo:**
  - `Task.DueDate *time.Time`, con `form:"-"`.
  - `SetDueDate(value string)` interpreta la cadena del formulario y recuerda si era inválida en un campo no exportado.
  - `Validate` añade el error de `due_date`. Así `ValidateAndCreate` y `ValidateAndUpdate` siguen siendo el único camino de validación, como hoy.
  - Helpers de presentación `HasDueDate`, `DueDateValue` y `DueDateLabel`, para que Plush no tenga que tratar punteros ni formatos.
- **Acciones:** `TasksCreate` añade `task.SetDueDate(c.Param("due_date"))`. `TasksUpdate` añade `DueDate` a su struct `input` y llama a `task.SetDueDate(input.DueDate)`. No hay rutas nuevas.
- **Plantillas:** un campo de fecha en los dos formularios, la fecha en la lista y los errores separados por campo.
- **CSS:** se agrupan título y fecha en `.task-main`, dentro de la columna del título. Hay estilo para `.task-due` y para `input[type="date"]`, y se incrementa la versión del CSS.

No se usa el decodificador de tiempo del binder de Buffalo (`form:"due_date"` sobre un `time.Time`) por dos motivos: acepta formatos con hora, y con un valor inválido `c.Bind` devolvería un error, que acabaría en 500 en lugar de 422.

## Alcance

### Incluido

- Migración `up`/`down` para `tasks.due_date DATE NULL`.
- Campo, validación y helpers en `models/task.go`.
- Lectura de `due_date` en `TasksCreate` y `TasksUpdate`.
- Campo de fecha en los formularios de creación y edición, la fecha en la lista y errores por campo.
- Estilos mínimos en `public/assets/app.css` y versión `?v=4` en el layout.
- Tests de modelo y de acciones.

### Excluido

- Resaltar tareas vencidas o próximas a vencer, filtrar u ordenar por fecha límite.
- Horas, zonas horarias, recordatorios o notificaciones.
- Restringir fechas pasadas o poner límites `min`/`max` en el input.
- Cambios en `TasksToggle`, `TasksDestroy`, las rutas (`actions/app.go`), `public/assets/app.js`, la migración de semillas o `fixtures/tasks.toml`.
- Cambios en `go.mod`/`go.sum`: solo se usa `time` de la biblioteca estándar, no `gobuffalo/nulls`.
- Arrancar la aplicación contra `todo_development`. `cmd/app/main.go` ejecuta las migraciones al arrancar, y esa base está compartida con otras ramas (hoy tiene una columna `priority` de otra rama).

## Diseño por capas

### Persistencia

- `migrations/20261026220000_add_due_date_to_tasks.up.fizz`:

  ```
  add_column("tasks", "due_date", "date", {"null": true})
  ```

- `migrations/20261026220000_add_due_date_to_tasks.down.fizz`:

  ```
  drop_column("tasks", "due_date")
  ```

- `migrations/embed.go` ya incluye `*.fizz`, así que no cambia.
- La versión `20261026220000` es intencionadamente distinta de `20261026000000`, que ya usan otras ramas de esta issue (ver Riesgos).

### Modelo (`models/task.go`)

- Constantes:
  - `DueDateLayout = "2006-01-02"` (exportada): formato de los campos de fecha HTML.
  - `dueDateLabelLayout = "02/01/2006"`: formato para mostrar.
  - `DueDateInvalidMessage = "La fecha límite debe ser un día válido."` (exportada), para reutilizarla en los tests.
- Campos nuevos en `Task`:
  - `DueDate *time.Time \`json:"due_date" db:"due_date" form:"-"\``, después de `Completed`.
  - `dueDateInvalid bool \`db:"-"\``, al final. **`db:"-"` es obligatorio**: pop trata como columna cualquier campo sin etiqueta `db` (`pop/v6/columns/tags.go`). El formulario y `httptest` ignoran los campos no exportados.
- `func (t *Task) SetDueDate(value string)`:
  1. Recorta los espacios y pone `t.dueDateInvalid = false`.
  2. Si el valor queda vacío, asigna `t.DueDate = nil`.
  3. Si no, usa `time.Parse(DueDateLayout, value)`. Si funciona, asigna `t.DueDate = &day`, que queda a medianoche UTC.
  4. Si falla, pone `t.dueDateInvalid = true` y **no** modifica `t.DueDate`.
- `Validate`: tras las validaciones del título, `if t.dueDateInvalid { verrs.Add("due_date", DueDateInvalidMessage) }`.
- Helpers con **receptor por valor**, necesario porque Plush los llama sobre los `Task` de `range tasks`:
  - `func (t Task) HasDueDate() bool`: indica si `t.DueDate != nil`.
  - `func (t Task) DueDateValue() string`: devuelve `AAAA-MM-DD`, o `""` si no hay fecha.
  - `func (t Task) DueDateLabel() string`: devuelve `DD/MM/AAAA`, o `""` si no hay fecha.
- Cada símbolo nuevo lleva un comentario de documentación en español, como el resto del archivo.

### Acción (`actions/tasks.go`)

- `TasksCreate`: después de `task.Title = strings.TrimSpace(task.Title)`, añade `task.SetDueDate(c.Param("due_date"))`.
  - `c.Param` incluye los valores del cuerpo del formulario (`buffalo@v1.1.4/context.go:58-66`).
  - `c.Bind` ignora `due_date`: `formam` solo empareja por nombre de campo exacto o por etiqueta, y `DueDate` tiene `form:"-"`.
- `TasksUpdate`:
  - El struct `input` pasa a ser `{ Title string \`form:"title" json:"title"\`; DueDate string \`form:"due_date" json:"due_date"\` }`.
  - Tras asignar el título, añade `task.SetDueDate(input.DueDate)`.
  - El comentario de la función cambia a «TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.».
- Nada más cambia. Con una fecha inválida, `ValidateAndCreate`/`ValidateAndUpdate` devuelven `verrs` y se reutiliza el render 422 que ya existe.

### Renderizado

**`templates/tasks/index.plush.html`**

- En cada `<li>`, sustituye `<span class="task-title"><%= t.Title %></span>` por:

  ```html
  <div class="task-main">
    <span class="task-title"><%= t.Title %></span>
    <%= if (t.HasDueDate()) { %>
      <span class="task-due">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></span>
    <% } %>
  </div>
  ```

  `.task-title` sigue conteniendo solo el título, que es lo que lee `app.js`.
- Formulario de creación:
  - Condiciones del input de título: cambia `errors.HasAny()` por `len(errors.Get("title")) > 0`. El resto de atributos se mantiene (`autofocus aria-invalid="true" aria-describedby="title-error"`).
  - Entre el input de título y el botón «Añadir», añade:

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

  - El bloque `<p id="title-error">` pasa a depender de `len(errors.Get("title")) > 0`. Debajo se añade un bloque equivalente `<p id="due-date-error" class="field-error" role="alert">`, que pinta `errors.Get("due_date")` cuando no está vacío.

**`templates/tasks/edit.plush.html`**

- Envuelve el bloque del título en `<div class="form-field">`.
  - La condición del input pasa de `errors.HasAny()` a `len(errors.Get("title")) > 0`.
  - El bloque de error del título también depende de esa condición.
  - La ayuda cambia a «Hasta 200 caracteres. El estado de la tarea no cambia.», porque ahora la fecha sí cambia.
- Añade un segundo `<div class="form-field">` antes de `.edit-task-actions`:

  ```html
  <div class="form-field">
    <label for="task-due-date">Fecha límite</label>
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
  </div>
  ```

**`templates/application.plush.html`**

- Cambia `/assets/app.css?v=3` por `/assets/app.css?v=4`, la convención de caché del repositorio.

### CSS (`public/assets/app.css`)

- `.task-title` (líneas 132-138): mueve `align-self: center`, `min-width: 0` y `padding: 0.65rem 0` a una regla nueva `.task-main`. Deja en `.task-title` `display: block; overflow-wrap: anywhere; line-height: 1.55;`.
- Nueva regla `.task-due { display: block; margin-top: 0.1rem; color: var(--color-muted); font-size: 0.8rem; }`.
- El selector `input[type="text"]` (línea 248) pasa a `input[type="text"], input[type="date"]`.
- Nueva regla, después de `.new-task-row input`: `.new-task-row input[type="date"] { flex: 0 0 auto; width: auto; }`. Su especificidad (0,2,1) también prevalece sobre `.new-task-row input { flex-basis: … }` en `@media (max-width: 380px)`.
- Nueva regla, junto a `.edit-task label`: `.form-field + .form-field { margin-top: 1.5rem; }`.
- No cambian `.task-completed .task-title` ni `.task-title` en `@media (max-width: 760px)`: siguen aplicando al mismo elemento.

## Archivos relevantes

| Ruta | Responsabilidad | Cambio |
| --- | --- | --- |
| `migrations/20261026220000_add_due_date_to_tasks.up.fizz` | Añade `tasks.due_date DATE NULL` | Nuevo |
| `migrations/20261026220000_add_due_date_to_tasks.down.fizz` | Revierte la columna | Nuevo |
| `migrations/embed.go` | Embebe `*.fizz` y `*.sql` | Sin cambios |
| `models/task.go` | Modelo `Task`, validación y helpers | Modificar |
| `models/task_test.go` | Tests del modelo | Añadir tests |
| `actions/tasks.go` | `TasksCreate` y `TasksUpdate` leen `due_date` | Modificar |
| `actions/tasks_test.go` | Tests HTTP de tareas | Añadir tests |
| `actions/app.go` | Rutas | Sin cambios |
| `templates/tasks/index.plush.html` | Lista y formulario de creación | Modificar |
| `templates/tasks/edit.plush.html` | Formulario de edición | Modificar |
| `templates/application.plush.html` | Layout, versión del CSS | `?v=3` → `?v=4` |
| `public/assets/app.css` | Estilos de lista y formularios | Modificar |
| `public/assets/app.js` | Toggle optimista (lee `.task-title`) | Sin cambios |
| `fixtures/tasks.toml`, `migrations/20260901000001_seed_tasks.*.sql` | Datos de prueba y semillas | Sin cambios (`due_date` queda `NULL`) |

## Implementación paso a paso

1. Crea los dos archivos de migración con el contenido indicado en «Persistencia».
2. En `models/task.go`, añade:
   - las constantes;
   - los campos `DueDate` y `dueDateInvalid`;
   - `SetDueDate`;
   - la comprobación en `Validate`;
   - los helpers `HasDueDate`, `DueDateValue` y `DueDateLabel`.
3. En `actions/tasks.go`, añade `task.SetDueDate(c.Param("due_date"))` en `TasksCreate`. En `TasksUpdate`, amplía el struct `input`, llama a `task.SetDueDate(input.DueDate)` y actualiza el comentario.
4. Edita `templates/tasks/index.plush.html`: añade `.task-main` con `.task-due`, el input de fecha y los errores por campo.
5. Edita `templates/tasks/edit.plush.html`: añade los `.form-field`, el input de fecha, la ayuda, los errores por campo y el nuevo texto de ayuda del título.
6. Edita `public/assets/app.css` según «CSS» y cambia `?v=3` por `?v=4` en `templates/application.plush.html`.
7. Añade los tests de «Tests y validación».
8. Ejecuta la validación completa.

## Tests y validación

### Tests de modelo (`models/task_test.go`)

- `Test_Task_DueDate_OptionalByDefault`: `ValidateAndCreate` de una tarea sin llamar a `SetDueDate` no da errores. Tras `Find`, `HasDueDate()` es `false` y `DueDate` es `nil`.
- `Test_Task_SetDueDate_PersistsDay`:
  - Ejecuta `SetDueDate(" 2026-11-15 ")` y `ValidateAndCreate`, sin errores.
  - Tras `Find`, `DueDateValue() == "2026-11-15"` y `DueDateLabel() == "15/11/2026"`.
- `Test_Task_SetDueDate_EmptyClears`:
  - Crea una tarea con fecha, ejecuta `SetDueDate("")` y `ValidateAndUpdate`, sin errores.
  - Tras `Find`, `HasDueDate()` es `false`.
- `Test_Task_SetDueDate_Invalid`:
  - Para cada valor de `"2026-02-30"`, `"30/11/2026"`, `"2026-11-15T10:00"` y `"mañana"`, ejecuta `SetDueDate` y `ValidateAndCreate`.
  - `verrs.Get("due_date")` contiene `DueDateInvalidMessage`, y no se crea ninguna fila.

### Tests de acciones (`actions/tasks_test.go`)

- `Test_TasksCreate_WithDueDate`:
  - `POST /tasks` con `url.Values{"title": {"Con fecha"}, "due_date": {"2026-11-15"}}` responde 303.
  - La tarea guardada tiene `DueDateValue() == "2026-11-15"`.
- `Test_TasksCreate_EmptyDueDate`: con `"due_date": {""}` responde 303, y la tarea tiene `HasDueDate() == false`.
- `Test_TasksCreate_InvalidDueDate`: con `"title": {"Fecha mala"}, "due_date": {"2026-02-30"}`:
  - responde 422;
  - el cuerpo contiene `DueDateInvalidMessage`, `aria-describedby="due-date-error"` y `value="Fecha mala"`;
  - el cuerpo **no** contiene `aria-describedby="title-error"`;
  - el recuento es 0.
- `Test_TasksIndex_ShowsDueDate`:
  - Hay una tarea con fecha `2026-11-15` y otra sin fecha.
  - `GET /` contiene `Fecha límite: <time datetime="2026-11-15">15/11/2026</time>` exactamente una vez (`strings.Count`).
- `Test_TasksEdit_PrefillsDueDate`: para una tarea con fecha, `GET /tasks/{id}/edit` contiene `type="date"`, `name="due_date"` y `value="2026-11-15"`.
- `Test_TasksUpdate_ChangesDueDate`:
  - La tarea está completada y tiene fecha.
  - `PUT` con `title` y `"due_date": {"2026-12-01"}` responde 303.
  - Persisten la nueva fecha y el título, y `Completed` sigue siendo `true`.
- `Test_TasksUpdate_ClearsDueDate`: un `PUT` con `"due_date": {""}` responde 303 y la tarea queda sin fecha.
- `Test_TasksUpdate_InvalidDueDate`:
  - `PUT` con `"due_date": {"15/11/2026"}` responde 422.
  - El cuerpo contiene `DueDateInvalidMessage`.
  - En BD, el título y la fecha originales no cambian.
- `Test_TasksToggle_KeepsDueDate`: tras `PUT /tasks/{id}/toggle`, `Completed` cambia y `DueDateValue()` sigue igual.
- Los tests existentes deben pasar sin modificarse. En especial, `Test_TasksUpdate_EmptyTitle` sigue comprobando `aria-describedby="title-error"`.

Para crear tareas con fecha en los tests, usa `task.SetDueDate("2026-11-15")` antes de `as.DB.ValidateAndCreate(task)`.

### Preparación de la base de test

Los tests de `actions` y `models` necesitan PostgreSQL con las migraciones aplicadas en `todo_test`. La base local **está compartida y contaminada** por otras ramas: hoy tiene aplicada la versión `20261026000000` y una columna `due_date` de otra rama. Por eso se recrea antes de migrar.

```sh
docker compose up -d db          # si no está levantado; esperar a "healthy" en `docker compose ps`
docker compose exec -T db dropdb -U postgres --if-exists todo_test
docker compose exec -T db createdb -U postgres todo_test
SODA="$(go env GOPATH)/bin/soda"  # si no existe: go install github.com/gobuffalo/pop/v6/soda@v6.4.1
GO_ENV=test "$SODA" migrate -e test
docker compose exec -T db psql -U postgres -d todo_test -c '\d tasks'   # due_date | date | nullable
GO_ENV=test "$SODA" migrate down -e test                                # due_date desaparece
GO_ENV=test "$SODA" migrate -e test                                     # vuelve a aplicarse
```

No toques `todo_development`.

### Comandos de validación

```sh
gofmt -l .                   # sin salida
go vet ./...
GO_ENV=test go test -p 1 ./...
go build ./...
git diff --check
```

## Riesgos y recuperación

- **Base de test compartida.** Recrear `todo_test` borra su esquema y sus datos, que son desechables: cada test los trunca. Si otro proceso usa esa base a la vez, los tests pueden fallar de forma intermitente. En ese caso, se repite la preparación y la ejecución.
- **Otras ramas abiertas para la issue #5.** Hay muchos PR que añaden una columna `due_date`, varios con la versión `20261026000000`. Este plan usa `20261026220000` para no chocar con ninguno de ellos. Aun así, solo debe integrarse una implementación: dos migraciones que añadan `due_date` harían fallar `migrate`.
- **Fechas y zona horaria.**
  - `SetDueDate` produce medianoche UTC.
  - pgx v5 (`go.mod`: `jackc/pgx/v5 v5.11.0`) codifica `time.Time` en una columna `date` usando año, mes y día, y la lee como medianoche UTC. Por eso el día no depende de la zona horaria del servidor.
  - `Test_Task_SetDueDate_PersistsDay` y `Test_TasksCreate_WithDueDate` lo comprueban. Si fallan por un desfase de un día, detente y avisa: no cambies el diseño en `implement`.
- **Un `PUT /tasks/{id}` sin `due_date` quita la fecha.** Es lo correcto para el formulario de edición, que siempre envía el campo. No hay otros clientes de esa ruta.
- **Maquetación.** Los tests HTTP no cubren la nueva fila de la lista ni el input de fecha en anchuras ≤760px y ≤380px. Debe revisarse visualmente en el PR.
- **Recuperación.** Revertir el commit y, en una base ya migrada, ejecutar `soda migrate down -e <entorno>`. Esto elimina la columna y las fechas límite guardadas, sin afectar al resto de datos de las tareas.
