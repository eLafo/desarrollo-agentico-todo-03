# Feature: fecha límite (solo día) en las tareas

## Encargo y criterios de aceptación

Issue [#5](https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5), «Quiero añadir fecha límite a las tasks»: «Vale con que sea simplemente un día. Sin hora».

Requisitos explícitos:

- Una tarea puede tener una fecha límite.
- La fecha límite es un día (año, mes y día), sin hora ni zona horaria.

Inferencias (no las pide la issue, pero sin ellas la funcionalidad queda incompleta o rompe lo existente):

- La fecha límite es **opcional**: las tareas existentes y las nuevas sin fecha siguen funcionando igual.
- Se asigna al crear la tarea y se puede cambiar o quitar al editarla.
- Se ve en la lista de tareas.

Criterios de aceptación:

1. El formulario de creación de `/` tiene un campo de fecha (`<input type="date" name="due_date">`) con etiqueta visible «Fecha límite (opcional)».
2. Crear una tarea con `due_date=2026-10-30` guarda `tasks.due_date = 2026-10-30` (columna `date`). Crear una tarea sin `due_date` o con `due_date` vacío guarda `NULL`.
3. En la lista, una tarea con fecha muestra `Fecha límite: <time datetime="2026-10-30">30/10/2026</time>` bajo su título. Una tarea sin fecha no muestra esa línea.
4. Un `due_date` que no sea `AAAA-MM-DD` o que no sea un día real (p. ej. `mañana`, `30/10/2026`, `2026-02-30`) devuelve `422`, muestra «La fecha límite no es válida.», conserva el título escrito y no guarda nada.
5. Los errores se marcan por campo: un error solo de fecha no marca el título como inválido y viceversa.
6. La página de edición muestra la fecha actual en un `<input type="date">`. Al guardar se puede cambiar la fecha o quitarla dejando el campo vacío. Con fecha inválida, `422` y la tarea queda intacta en la base de datos.
7. Completar o reabrir una tarea (`PUT /tasks/{id}/toggle`) conserva su fecha límite. Editar la fecha no cambia `completed`, `id` ni `created_at`.

## Comportamiento actual y propuesta

Actual (`origin/main`):

- `tasks` tiene `id`, `title`, `completed`, `created_at` y `updated_at` (`migrations/20260901000000_create_tasks.up.fizz`). No hay ninguna noción de fecha límite.
- `TasksCreate` (`actions/tasks.go`) hace `c.Bind(task)` directamente sobre `models.Task`, recorta el título y llama a `tx.ValidateAndCreate`. Si hay errores, vuelve a renderizar `tasks/index.plush.html` con `422`.
- `TasksUpdate` enlaza un struct anónimo solo con `title` y actualiza solo el título.
- `templates/tasks/index.plush.html` y `templates/tasks/edit.plush.html` muestran el error de título cuando `errors.HasAny()`, sin distinguir campos.
- `public/assets/app.js` usa `.task-title` para componer el `aria-label` del botón de completar al hacer toggle desde el navegador; la línea de la tarea es una rejilla `toggle | título | acciones` (`.task` en `public/assets/app.css`).

Propuesta (cambio vertical mínimo, siguiendo los patrones existentes):

- Migración nueva que añade `tasks.due_date` de tipo `date`, nulable.
- `models.Task` gana `DueDate *time.Time` y métodos para interpretar el valor del formulario (`SetDueDate`) y formatearlo (`DueDateValue`, `DueDateLabel`). `Validate` informa del valor inválido en el campo `due_date`, de modo que las acciones siguen usando `ValidateAndCreate` / `ValidateAndUpdate`.
- `TasksCreate` y `TasksUpdate` enlazan un struct de entrada explícito con `title` y `due_date` (el patrón que ya usa `TasksUpdate`) y llaman a `task.SetDueDate`.
- Las plantillas añaden el campo de fecha, la línea de fecha en la lista y los errores por campo. CSS para la nueva fila y la línea de fecha; versión del CSS a `v=4`.

## Alcance

Incluido:

- Migración `up`/`down` de la columna `due_date`.
- Modelo, validación y formateo de la fecha.
- Creación y edición con fecha (asignar, cambiar y quitar).
- Visualización en la lista.
- Errores por campo en los formularios de creación y edición.
- Tests de modelo y de acciones.

Excluido:

- Marcar tareas vencidas o próximas a vencer, colores por urgencia.
- Ordenar, filtrar o agrupar por fecha límite (el orden sigue siendo `created_at asc, id asc`).
- Recordatorios o notificaciones.
- Horas, zonas horarias y nombres de mes localizados.
- Cambios en `public/assets/app.js` (incluido el toggle con `fetch(form.action)`, que es un bug aparte; ver riesgos).
- Cambios en `fixtures/tasks.toml`, en la migración de semillas, en `go.mod`/`go.sum` y en la configuración (`database.yml`, CI).

## Diseño por capas

### Persistencia

Archivos nuevos `migrations/20261026000000_add_due_date_to_tasks.up.fizz` y `.down.fizz`:

```
add_column("tasks", "due_date", "date", {"null": true})
```

```
drop_column("tasks", "due_date")
```

El traductor Postgres de fizz deja pasar `date` tal cual (columna `DATE`). `migrations/embed.go` ya incluye `*.fizz`, y `cmd/app/main.go` aplica las migraciones al arrancar. Las filas existentes quedan con `NULL`.

### Modelo (`models/task.go`)

- Constantes nuevas:
  - `DueDateLayout = "2006-01-02"`: formato de `<input type="date">` y de `datetime`.
  - `DueDateDisplayLayout = "02/01/2006"`: formato visible.
  - `DueDateInvalidMessage = "La fecha límite no es válida."`.
- Campos de `Task`:
  - `DueDate *time.Time` con tags `json:"due_date" db:"due_date" form:"-"`, situado justo después de `Completed`. Se usa puntero (nulable) en vez de `nulls.Time` para no convertir `github.com/gobuffalo/nulls` en dependencia directa ni tocar `go.mod`.
  - Campo no exportado `dueDateInvalid bool` con tag `db:"-"`, situado al final del struct. El tag es obligatorio: Pop toma como columna cualquier campo sin tag `db` (`columns.TagsFor` usa el nombre del campo por defecto).
- `func (t *Task) SetDueDate(value string)`:
  - Recorta espacios.
  - Si el valor queda vacío, pone `DueDate = nil` y `dueDateInvalid = false`.
  - Si `time.Parse(DueDateLayout, value)` falla, pone `DueDate = nil` y `dueDateInvalid = true`.
  - Si no, guarda la fecha resultante (medianoche UTC) y `dueDateInvalid = false`.
  - `time.Parse` rechaza días inexistentes como `2026-02-30`.
- `Validate`: además de las reglas de título actuales, si `t.dueDateInvalid` añade `verrs.Add("due_date", DueDateInvalidMessage)`.
- `func (t *Task) DueDateValue() string`: `""` si `DueDate == nil`; si no, `t.DueDate.Format(DueDateLayout)`.
- `func (t *Task) DueDateLabel() string`: `""` si `DueDate == nil`; si no, `t.DueDate.Format(DueDateDisplayLayout)`.

Receptores de puntero en todos los métodos, como `Validate`. Plush llama a métodos de puntero sobre los elementos de `for (t) in tasks` (crea un puntero si el valor no tiene el método; `plush/v5 compiler.go`, `evalCallExpression`).

Notas sobre zona horaria:

- `time.Parse` sin zona devuelve UTC.
- pgx v5 (driver de Pop v6) codifica `time.Time` para un parámetro `date` usando el año, mes y día del propio valor.
- Al leer un `date` devuelve medianoche UTC.

Formatear sin convertir de zona conserva el día.

### Acciones (`actions/tasks.go`)

`TasksCreate`:

- Sustituir `c.Bind(task)` por un struct de entrada como el de `TasksUpdate`:

  ```go
  input := struct {
  	Title   string `form:"title" json:"title"`
  	DueDate string `form:"due_date" json:"due_date"`
  }{}
  ```

- Construir `task := &models.Task{Title: strings.TrimSpace(input.Title)}` y llamar a `task.SetDueDate(input.DueDate)`.
- El resto (`ValidateAndCreate`, re-render `422` con `tasks`, `task` y `errors`, flash y redirect) no cambia.
- Enlazar a un struct explícito evita además que el formulario asigne campos del modelo (*mass assignment*).

`TasksUpdate`:

- Añadir `DueDate string `form:"due_date" json:"due_date"`` al struct de entrada.
- Tras recortar el título, llamar a `task.SetDueDate(input.DueDate)`.
- Actualizar el comentario de la función: «actualiza el título y la fecha límite, conservando los demás datos de la tarea».
- Con `422` re-renderiza como ahora. La transacción por petición (`popmw.Transaction`) no guarda nada si no hay `Update`.

`TasksToggle`, `TasksDestroy` y `TasksIndex` no cambian. `TasksToggle` hace `tx.Update(task)` con la tarea cargada de la base de datos, así que `due_date` se reescribe con su mismo valor.

Rutas (`actions/app.go`): sin cambios.

### Renderizado (Plush)

`templates/tasks/index.plush.html`:

- Lista: envolver el título en un contenedor para que la rejilla siga teniendo tres columnas y `.task-title` siga conteniendo solo el título, que `app.js` lee con `textContent`:

  ```html
  <div class="task-body">
    <span class="task-title"><%= t.Title %></span>
    <%= if (t.DueDate) { %>
      <span class="task-due">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></span>
    <% } %>
  </div>
  ```

  `isTruthy` de Plush trata un puntero nil como falso. El bloque `<%= if (t.Completed) { %><span class="task-status">…` queda donde está, después de `.task-body`. `app.js` inserta `.task-status` antes de `.task-actions`; sigue funcionando.

- Formulario de creación:
  - En el input de título, sustituir la condición `errors.HasAny()` por `len(errors.Get("title")) > 0`, tanto en los atributos (`autofocus aria-invalid="true" aria-describedby="title-error"`) como en el `<p id="title-error">`.
  - Después de `.new-task-row`, añadir:

    ```html
    <div class="new-task-due">
      <label for="task-due-date">Fecha límite <span class="field-optional">(opcional)</span></label>
      <input
        id="task-due-date"
        type="date"
        name="due_date"
        value="<%= task.DueDateValue() %>"
        <%= if (len(errors.Get("due_date")) > 0) { %>aria-invalid="true" aria-describedby="due-date-error"<% } %>
      >
    </div>
    ```

  - Después del `<p id="title-error">`, añadir:

    ```html
    <%= if (len(errors.Get("due_date")) > 0) { %>
      <p id="due-date-error" class="field-error" role="alert">
        <%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %>
      </p>
    <% } %>
    ```

  El botón «Añadir» sigue en la fila del título. Pulsar Intro en cualquiera de los dos campos envía el formulario.

`templates/tasks/edit.plush.html`:

- Título:
  - Sustituir `errors.HasAny()` por `len(errors.Get("title")) > 0` en los atributos del input y en el bloque `<p id="title-error">`.
  - Cambiar el texto de `#title-help` a «Hasta 200 caracteres. El estado de la tarea no cambia.», porque la fecha ahora sí puede cambiar.
- Después del bloque de error del título y antes de `.edit-task-actions`, añadir:

  ```html
  <div class="edit-task-due">
    <label for="task-due-date">Fecha límite <span class="field-optional">(opcional)</span></label>
    <input
      id="task-due-date"
      type="date"
      name="due_date"
      value="<%= task.DueDateValue() %>"
      <%= if (len(errors.Get("due_date")) > 0) { %>aria-invalid="true" aria-describedby="due-date-error"<% } else { %>aria-describedby="due-date-help"<% } %>
    >
    <p id="due-date-help" class="field-hint">Déjala vacía si la tarea no tiene fecha límite.</p>
    <%= if (len(errors.Get("due_date")) > 0) { %>
      <p id="due-date-error" class="field-error" role="alert">
        <%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %>
      </p>
    <% } %>
  </div>
  ```

`templates/application.plush.html`: cambiar `/assets/app.css?v=3` por `/assets/app.css?v=4` para invalidar la caché del CSS.

### CSS (`public/assets/app.css`)

- `.task-title`: sus reglas actuales (`align-self: center; min-width: 0; padding: 0.65rem 0; overflow-wrap: anywhere; line-height: 1.55;`) se reparten:

  ```css
  .task-body {
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

  `.task-completed .task-title` y `.task-title { font-size: 0.9rem; }` del bloque móvil se mantienen.

- Inputs: ampliar el selector `input[type="text"]` a `input[type="text"],\ninput[type="date"]` (mismo bloque de estilos).
- Cambiar `.new-task-row input { flex: 1; }` por `.new-task-row input[type="text"] { flex: 1; }`. En el bloque `@media (max-width: 380px)`, cambiar `.new-task-row input { flex-basis: … }` por `.new-task-row input[type="text"] { flex-basis: … }`.
- Reglas nuevas, después de `.new-task-row input[type="text"]`:

  ```css
  .new-task-due {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem 0.6rem;
    margin-top: 0.5rem;
    padding-left: calc(44px + 0.6rem);
  }

  .new-task-due label {
    color: var(--color-muted);
    font-size: 0.8rem;
  }

  .new-task-due input {
    width: auto;
  }

  .field-optional {
    font-weight: 400;
    color: var(--color-muted);
  }
  ```

  `padding-left` alinea la fila con el input de título: `.add-mark` mide 44px más el `gap` de 0.6rem.

- Después de `.edit-task label`:

  ```css
  .edit-task-due {
    margin-top: 1.25rem;
  }
  ```

- En `@media (max-width: 760px)`, añadir `.new-task-due { padding-left: calc(32px + 0.4rem); }`. Allí `.add-mark` mide 36px con `margin-left: -4px` y el `gap` es 0.4rem.

JavaScript: sin cambios.

## Archivos relevantes

| Ruta | Responsabilidad | Cambio |
| --- | --- | --- |
| `migrations/20261026000000_add_due_date_to_tasks.up.fizz` | Añade `tasks.due_date` (`date`, nulable) | nuevo |
| `migrations/20261026000000_add_due_date_to_tasks.down.fizz` | Elimina la columna | nuevo |
| `models/task.go` | Modelo `Task`, validación, parseo y formateo de la fecha | modificar |
| `models/task_test.go` | Tests del modelo | ampliar |
| `actions/tasks.go` | `TasksCreate` y `TasksUpdate` leen `due_date` | modificar |
| `actions/tasks_test.go` | Tests de acciones | ampliar |
| `templates/tasks/index.plush.html` | Lista (línea de fecha) y formulario de creación | modificar |
| `templates/tasks/edit.plush.html` | Formulario de edición | modificar |
| `templates/application.plush.html` | Versión del CSS (`v=4`) | modificar |
| `public/assets/app.css` | Estilos de la línea de fecha y de los campos | modificar |
| `actions/app.go`, `public/assets/app.js`, `fixtures/tasks.toml`, `migrations/embed.go` | Contexto; no cambian | — |

## Implementación paso a paso

1. Crear las dos migraciones fizz con el contenido de «Persistencia».
2. En `models/task.go`, añadir:
   - las constantes `DueDateLayout`, `DueDateDisplayLayout` y `DueDateInvalidMessage`;
   - los campos `DueDate` y `dueDateInvalid` con sus tags;
   - los métodos `SetDueDate`, `DueDateValue` y `DueDateLabel`, con comentarios en español como los existentes;
   - la comprobación de `dueDateInvalid` en `Validate`.
3. En `actions/tasks.go`, cambiar `TasksCreate` y `TasksUpdate` como se describe en «Acciones».
4. Actualizar `templates/tasks/index.plush.html`, `templates/tasks/edit.plush.html` y `templates/application.plush.html` según «Renderizado».
5. Actualizar `public/assets/app.css` según «CSS».
6. Añadir los tests de «Tests y validación».
7. Preparar la base de datos de test y ejecutar la validación completa.

## Tests y validación

### Tests de modelo (`models/task_test.go`)

- `Test_Task_DueDate_OptionalByDefault`: crear con `ValidateAndCreate` sin fecha. Sin errores; tras `Find`, `DueDate` es `nil` y `DueDateValue()` es `""`.
- `Test_Task_DueDate_PersistsDay`:
  - `SetDueDate("2026-10-30")` y `ValidateAndCreate` sin errores.
  - Tras `Find`, `DueDateValue()` es `"2026-10-30"` y `DueDateLabel()` es `"30/10/2026"`.
- `Test_Task_DueDate_Invalid`: con `"mañana"`, `"30/10/2026"` y `"2026-02-30"` (un subcaso por valor):
  - `verrs.Get("due_date")` contiene `DueDateInvalidMessage`;
  - `verrs.Get("title")` está vacío;
  - no se crea ninguna fila.
- `Test_Task_DueDate_EmptyClears`:
  - `SetDueDate("2026-10-30")` seguido de `SetDueDate("  ")` deja `DueDate == nil`.
  - `ValidateAndCreate` no da errores.

### Tests de acciones (`actions/tasks_test.go`)

Añadir un helper `taskWithDueDate(dueDate string) *models.Task`. Crea la tarea «Con fecha» con `SetDueDate` + `ValidateAndCreate`, comprueba que no hay errores y hace `Reload`.

- `Test_TasksCreate_Valid` (existente): añadir `as.Nil(task.DueDate)`.
- `Test_TasksCreate_WithDueDate`:
  - `POST /tasks` con `url.Values{"title": {"Con fecha"}, "due_date": {"2026-10-30"}}` responde `303`.
  - La tarea guardada tiene `DueDateValue() == "2026-10-30"`.
- `Test_TasksCreate_InvalidDueDate`: con `due_date=mañana`:
  - responde `422`;
  - el cuerpo contiene `La fecha límite no es válida.`, `aria-describedby="due-date-error"` y `value="Con fecha"`;
  - el cuerpo no contiene `title-error`;
  - hay 0 tareas.
- `Test_TasksCreate_EmptyTitle_Invalid` (existente): añadir `as.NotContains(body, "due-date-error")`.
- `Test_TasksIndex_ShowsDueDate`:
  - Una tarea con fecha `2026-10-30` y otra sin fecha, creada con `taskForEditing(false)`.
  - `GET /` contiene `datetime="2026-10-30"` y `30/10/2026`.
  - `class="task-due"` aparece exactamente una vez (`strings.Count`).
- `Test_TasksIndex_DueDateField`: `GET /` contiene `type="date"` y `name="due_date"`.
- `Test_TasksEdit_PrefillsDueDate`: `GET /tasks/{id}/edit` de una tarea con fecha contiene `value="2026-10-30"`.
- `Test_TasksUpdate_ChangesDueDate`:
  - Tarea con fecha marcada como completada (`Completed = true` + `as.DB.Update`).
  - `PUT` con `title` y `due_date=2026-11-05` responde `303`.
  - Tras `Reload`: `DueDateValue() == "2026-11-05"`, `Completed` sigue en `true` y `CreatedAt` no cambia.
- `Test_TasksUpdate_ClearsDueDate`: `PUT` con `title` y `due_date=""` responde `303`. Tras `Reload`, `DueDate` es `nil`.
- `Test_TasksUpdate_InvalidDueDate`:
  - `PUT` con `title=Otro título` y `due_date=2026-02-30` responde `422`.
  - El cuerpo contiene el mensaje.
  - Tras `Reload`, el título sigue en «Con fecha» y la fecha en `2026-10-30`.
- `Test_TasksToggle_KeepsDueDate`:
  - `PUT /tasks/{id}/toggle` responde `303`.
  - Tras `Reload`, `Completed` es `true` y `DueDateValue()` sigue en `"2026-10-30"`.

Los tests existentes deben seguir pasando sin más cambios. En particular, `Test_TasksUpdate_PersistsTitleOnly` y `Test_TasksEdit_EscapesTitle`.

### Preparación de la base de datos de test

Los tests de `actions` y `models` necesitan PostgreSQL con `todo_test` migrado, incluida la migración nueva. `database.yml` apunta a `127.0.0.1:5432`.

- Si el PostgreSQL de este proyecto (`docker compose up -d db`) está disponible en 5432, crear `todo_test` si no existe y migrarlo:

  ```sh
  GO_ENV=test go run github.com/gobuffalo/pop/v6/soda@v6.4.1 migrate -e test
  ```

  `soda` no está instalado en el host; esta versión es la misma que usa la CI.

- Si el 5432 lo ocupa un contenedor de **otro** proyecto (al planificar lo ocupaba `desarrollo-agentico-todo-04-db-1`), no detenerlo ni migrar su base de datos. Usar un entorno temporal:
  1. Arrancar un PostgreSQL desechable en un puerto libre:

     ```sh
     docker run -d --rm --name todo03-issue5-pg -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=todo_test -p 15432:5432 postgres:17.6-alpine
     ```

     Esperar a que `pg_isready` responda dentro del contenedor.
  2. Copiar `database.yml` a un directorio temporal (p. ej. `/tmp/todo03-issue5/database.yml`) y añadir `port: 15432` a la conexión `test`.
  3. Exportar `POP_PATH=/tmp/todo03-issue5` en todos los comandos. Pop antepone `POP_PATH` a sus rutas de búsqueda (`pop/v6 config.go`, `AddLookupPaths`).
  4. Migrar con `-c /tmp/todo03-issue5/database.yml -p migrations`.
  5. Al terminar, eliminar el contenedor y el directorio temporal.

  No modificar `database.yml` del repositorio.

### Validación obligatoria

Desde la raíz, con el `POP_PATH` del entorno temporal si se usa:

1. `gofmt -l .` no devuelve nada.
2. `go vet ./...`.
3. Migración `up` aplicada en test. Después, `migrate down` seguido de `migrate` otra vez (`-e test`), sin errores. Así se comprueba que el `down` funciona.
4. `GO_ENV=test go test -p 1 -count=1 ./...`: todos los paquetes en verde, incluidos los tests nuevos.
5. `go build ./...`.
6. `git diff --stat origin/main -- go.mod go.sum` no muestra cambios.

### Revisión visual (no bloqueante)

Arrancar la aplicación contra una base de datos de desarrollo migrada y revisar `/` y la página de edición a ~1280px y ~375px:

- la fila de fecha queda alineada con el input de título;
- la línea «Fecha límite: …» aparece bajo el título;
- el formulario no desborda en móvil.

Si el entorno no lo permite, indicarlo en el informe; no bloquea la entrega porque los criterios funcionales están cubiertos por tests.

## Riesgos y recuperación

- **Base de datos de test sin migrar:** los tests fallan con «column "due_date" does not exist». Aplicar la migración en `todo_test` (ver «Preparación»). En la CI ya se ejecuta `soda migrate -e test` antes de los tests.
- **Puerto 5432 ocupado por otro proyecto:** no tocar ese contenedor ni su base de datos. Usar el PostgreSQL temporal con `POP_PATH`.
- **Desfase de un día por zona horaria:**
  - Mitigado: se parsea en UTC, pgx codifica la parte de fecha del valor y se formatea sin convertir de zona.
  - Si un test detecta un día desplazado, comprobar que `SetDueDate` usa `time.Parse` y no `time.ParseInLocation` con zona local, y que los métodos de formato no llaman a `.Local()`.
- **Cliente que no envía `due_date` en el `PUT`:** se quita la fecha, porque «ausente» y «vacío» se tratan igual. Los formularios siempre envían el campo; se acepta y se documenta.
- **Valor inválido en el re-render:** tras un `422` por fecha inválida, el campo de fecha aparece vacío, porque no se conserva el texto inválido. Los navegadores con `type="date"` no envían valores inválidos, así que solo afecta a clientes manuales.
- **Rollback:**
  - `soda migrate down` elimina la columna.
  - Revertir el commit restaura el código.
  - Las fechas guardadas se pierden con el `down`.
- **Trabajo en paralelo:**
  - Ya hay PRs abiertos para la misma issue (#6 a #11). Solo debe integrarse uno.
  - La rama local `main` tiene un commit sin publicar (`429e9d6`, toggle de `app.js`) que toca `actions/tasks_test.go` y `templates/application.plush.html`. Puede haber conflictos triviales al integrar; resolver conservando ambos cambios.
- **Toggle desde la interfaz:** en `origin/main`, `app.js` hace `fetch(form.action)` (GET), así que el toggle desde el navegador no persiste. Es un bug previo y queda fuera de alcance. La conservación de la fecha en el toggle se prueba con el `PUT` real en `Test_TasksToggle_KeepsDueDate`.
