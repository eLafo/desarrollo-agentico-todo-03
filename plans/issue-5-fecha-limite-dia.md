# Feature: fecha límite (solo día) en las tareas

## Encargo y criterios de aceptación

Issue [#5](https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5): «Quiero añadir fecha límite a las tasks». Cuerpo: «Vale con que sea simplemente un día. Sin hora».

Requisitos explícitos:

- Una tarea puede tener una fecha límite.
- La fecha límite es un día del calendario, sin hora.

Inferencias (decisiones de este plan, no pedidas literalmente):

- La fecha límite es opcional. Las tareas existentes quedan sin fecha y se puede seguir creando una tarea solo con título.
- Se fija al crear la tarea y se puede cambiar o quitar desde la pantalla de edición.
- Se muestra en la lista de tareas.
- Se admiten fechas pasadas: una tarea puede estar vencida, y rechazarlas impediría editar el título de una tarea antigua.
- Un valor que no sea un día válido (`aaaa-mm-dd`) se rechaza con 422 y un mensaje en el formulario. No se guarda nada.

Criterios de aceptación:

1. El formulario «Añadir tarea» tiene un campo de fecha opcional con la etiqueta «Fecha límite (opcional)» (`<input type="date" name="due_date">`).
2. `POST /tasks` con `title` y `due_date=2026-11-15` crea la tarea con fecha límite 2026-11-15. Sin `due_date` o con `due_date` vacío la crea sin fecha.
3. En la lista, una tarea con fecha muestra `Fecha límite: 15/11/2026` en un `<time datetime="2026-11-15">`. Una tarea sin fecha no muestra nada.
4. La pantalla de edición muestra el campo «Fecha límite» con el valor actual (`value="2026-11-15"`). `PUT /tasks/{id}` guarda la nueva fecha, y con el campo vacío la quita. No cambia el estado `completed` ni los timestamps de creación.
5. Un `due_date` inválido (por ejemplo `2026-02-30` o `15/11/2026`) en la creación o en la edición devuelve 422 con «La fecha límite no es válida.». No crea la tarea ni modifica la existente, y solo el campo de fecha queda marcado como inválido.
6. Si el título no es válido, el 422 conserva en el formulario la fecha introducida.
7. Se persiste solo el día (columna `date`). El valor leído no depende de la zona horaria del servidor.

## Comportamiento actual y propuesta

Hoy una tarea tiene solo `title`, `completed` y timestamps (`migrations/20260901000000_create_tasks.up.fizz`, `models/task.go`).

- `TasksCreate` hace `c.Bind(task)` sobre `models.Task`. Solo `title` tiene etiqueta `form`.
- `TasksUpdate` enlaza una struct anónima con solo `Title`, para no permitir asignación masiva de otros campos.
- Las plantillas marcan el título como inválido con `errors.HasAny()`, porque hasta ahora era el único campo validado.

Propuesta (cambio vertical mínimo):

- Columna nullable `due_date date` en `tasks`.
- Campo `DueDate *time.Time` en `models.Task`, con `form:"-"`: la fecha nunca se enlaza directamente al modelo. Se añaden helpers para parsear y formatear el día.
- Una struct `taskInput` (`title`, `due_date` como string) compartida por crear y editar. Sustituye al `c.Bind(task)` de la creación y a la struct anónima de la edición. Su método `apply` recorta el título, parsea la fecha y devuelve un error de validación `due_date` si la fecha no es un día válido.
- Plantillas: campo de fecha en los dos formularios, fecha en la lista y errores por campo (`errors.Get("title")` / `errors.Get("due_date")`) en lugar de `errors.HasAny()`.
- CSS para el nuevo campo y la línea de fecha de la lista, y se sube la versión de caché de `app.css`.

¿Por qué no enlazar la fecha con `c.Bind`? El binder de Buffalo solo decodifica `time.Time` y su tipo interno `buffalo/internal/nulls.Time`, que no es importable.

- Con `time.Time` no se puede representar «sin fecha».
- `github.com/gobuffalo/nulls.Time.UnmarshalText` espera JSON RFC3339, así que falla con `2026-11-15`.
- En los dos casos, un valor inválido hace que `Bind` devuelva error y la respuesta sería 500.

Por eso la fecha llega como string y se parsea de forma explícita con el layout `2006-01-02`.

## Alcance

Incluido:

- Migración nueva, reversible, que añade `due_date` nullable.
- Modelo: campo, parseo y formateo.
- Acciones `TasksCreate` y `TasksUpdate`: enlazado y validación de la fecha.
- Plantillas `tasks/index.plush.html`, `tasks/edit.plush.html` y la versión de caché del CSS en `application.plush.html`.
- Estilos en `public/assets/app.css`.
- Tests de modelo y de acciones.

Excluido:

- Hora o zona horaria en la fecha límite.
- Resaltar tareas vencidas o próximas a vencer.
- Ordenar o filtrar por fecha límite. El orden sigue siendo `created_at asc, id asc`.
- Restringir fechas pasadas.
- Cambios en `TasksToggle`, `TasksDestroy`, rutas o `public/assets/app.js`.
- Cambios en la migración de seed (`20260901000001_seed_tasks.*.sql`) o en `fixtures/tasks.toml`. Sus filas quedan con `due_date` NULL.
- Dependencias nuevas o cambios en `go.mod` / `go.sum`.

## Diseño por capas

### Persistencia y migraciones

Archivos nuevos (fizz, mismo estilo que la migración existente):

- `migrations/20261026000000_add_due_date_to_tasks.up.fizz`:
  ```
  add_column("tasks", "due_date", "date", {"null": true})
  ```
- `migrations/20261026000000_add_due_date_to_tasks.down.fizz`:
  ```
  drop_column("tasks", "due_date")
  ```

El traductor PostgreSQL de fizz deja el tipo `date` tal cual (rama `default` de `colType`). `migrations/embed.go` ya incluye `*.fizz`. `cmd/app/main.go` ejecuta `models.Migrate()` al arrancar, así que el despliegue aplica la migración solo. Es aditiva y nullable, segura con datos existentes.

### Modelo (`models/task.go`)

- Constante `DueDateLayout = "2006-01-02"`: formato del campo de fecha HTML y de la columna.
- Campo nuevo entre `Completed` y `CreatedAt`:
  `DueDate *time.Time `json:"due_date" db:"due_date" form:"-"``. Con `nil` = sin fecha límite, pop escribe y lee NULL sin tipos auxiliares.
- `ParseDueDate(value string) (*time.Time, error)`:
  - Recorta espacios.
  - Si queda vacío, devuelve `nil, nil`.
  - Si no, devuelve `time.Parse(DueDateLayout, value)`, que da medianoche UTC. Rechaza días inexistentes (`2026-02-30`) y otros formatos.
- `func (t Task) DueDateValue() string`: `""` si no hay fecha; si la hay, `t.DueDate.Format(DueDateLayout)`. Es el valor para `<input type="date">` y `datetime`.
- `func (t Task) DueDateLabel() string`: `""` si no hay fecha; si la hay, `t.DueDate.Format("02/01/2006")`. Es el texto de la lista.
- Receptor por valor en los dos helpers: en `for (t) in tasks` cada `t` es un `models.Task` (no direccionable) y Plush necesita encontrar el método.
- No convertir nunca la fecha a `Local()`. `time.Parse` devuelve UTC y pgx lee `date` como medianoche UTC, así que formatear en UTC da siempre el mismo día.
- `Validate` no cambia: cualquier día válido es aceptable.

### Acción (`actions/tasks.go`)

Añadir junto a las acciones:

```go
// taskInput recoge los campos de una tarea que se pueden enviar desde los formularios.
type taskInput struct {
	Title   string `form:"title" json:"title"`
	DueDate string `form:"due_date" json:"due_date"`
}

// apply copia en la tarea el título y la fecha límite recibidos. Si la fecha
// límite no es un día válido, conserva la de la tarea y devuelve el error.
func (in taskInput) apply(task *models.Task) *validate.Errors {
	verrs := validate.NewErrors()
	task.Title = strings.TrimSpace(in.Title)

	dueDate, err := models.ParseDueDate(in.DueDate)
	if err != nil {
		verrs.Add("due_date", "La fecha límite no es válida.")
		return verrs
	}
	task.DueDate = dueDate
	return verrs
}
```

`TasksCreate`:

- Sustituir `task := &models.Task{}` + `c.Bind(task)` + `TrimSpace` por:
  - `input := taskInput{}` y `c.Bind(&input)`;
  - `task := &models.Task{}`;
  - `verrs := input.apply(task)`.
- Solo si `!verrs.HasAny()`, ejecutar `verrs, err = tx.ValidateAndCreate(task)`, devolviendo `err` si lo hay. Declarar `var err error` dentro del bloque o equivalente que compile sin sombrear `verrs`.
- El bloque `if verrs.HasAny() { … 422 … }` y la redirección quedan igual.

`TasksUpdate`:

- Sustituir la struct anónima por `input := taskInput{}` y aplicar el mismo patrón: `verrs := input.apply(task)` y, solo si no hay errores, `tx.ValidateAndUpdate(task)`.
- El render 422 y la redirección quedan igual.
- Actualizar el comentario de la función: «TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.»

Con la fecha inválida no se ejecuta la validación del modelo, así que en ese 422 solo aparece el error de fecha. Es aceptable porque un navegador solo envía `""` o `aaaa-mm-dd` desde `type="date"`.

Si un `PUT` no incluye `due_date`, equivale a vaciarlo y quita la fecha. Los formularios siempre envían el campo.

### Renderizado (Plush)

`templates/tasks/index.plush.html`:

1. En cada `<li class="task">`, envolver el título en un contenedor y añadir la fecha. El `<span class="task-title">` debe conservar solo el título, porque `app.js` usa su `textContent` para el `aria-label`. `.task-actions` y `.task-status` siguen siendo hijos directos del `<li>`, porque `app.js` hace `task.insertBefore(status, task.querySelector(".task-actions"))`.
   ```html
   <div class="task-body">
     <span class="task-title"><%= t.Title %></span>
     <%= if (t.DueDateValue() != "") { %>
       <time class="task-due" datetime="<%= t.DueDateValue() %>">Fecha límite: <%= t.DueDateLabel() %></time>
     <% } %>
   </div>
   ```
2. En el formulario de creación:
   - Cambiar la condición del input de título de `errors.HasAny()` a `len(errors.Get("title")) > 0`, tanto en los atributos `autofocus aria-invalid aria-describedby` como en el bloque `<p id="title-error">`.
   - Tras `</div>` de `.new-task-row`, añadir:
   ```html
   <div class="new-task-due">
     <label for="task-due-date">Fecha límite (opcional)</label>
     <input
       id="task-due-date"
       type="date"
       name="due_date"
       value="<%= task.DueDateValue() %>"
       <%= if (len(errors.Get("due_date")) > 0) { %>autofocus aria-invalid="true" aria-describedby="due-date-error"<% } %>
     >
   </div>
   ```
   - Después del bloque `title-error`, añadir uno análogo:
   ```html
   <%= if (len(errors.Get("due_date")) > 0) { %>
     <p id="due-date-error" class="field-error" role="alert">
       <%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %>
     </p>
   <% } %>
   ```

`templates/tasks/edit.plush.html`:

1. Cambiar las dos condiciones `errors.HasAny()` del título a `len(errors.Get("title")) > 0`. Se mantiene el `else` con `aria-describedby="title-help"`.
2. Tras el bloque `title-error` y antes de `.edit-task-actions`, añadir:
   ```html
   <div class="edit-task-due">
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

`templates/application.plush.html`: cambiar `/assets/app.css?v=3` por `/assets/app.css?v=4`. No tocar la línea de `app.js`.

### CSS (`public/assets/app.css`)

Sin comentarios, igual que el resto del archivo:

1. Sustituir la regla `.task-title { align-self… }` por:
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
     color: var(--color-muted);
     font-size: 0.8rem;
   }
   ```
   Las reglas `.task-completed .task-title` y la de `font-size` en `@media (max-width: 760px)` siguen aplicando al `<span>`.
2. Ampliar el selector `input[type="text"] { … }` a `input[type="text"],\ninput[type="date"] { … }`. Justo después, añadir `input[type="date"] { width: auto; }`.
3. Tras `.new-task-row input { flex: 1; }`, añadir:
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
     font-size: 0.85rem;
   }
   ```
4. Tras `.edit-task label { … }`, añadir `.edit-task-due { margin-top: 1.25rem; }`. La regla `.edit-task label` ya da estilo a la nueva etiqueta.
5. Dentro de `@media (max-width: 760px)`, junto a `.new-task-row { gap: 0.4rem; }`, añadir `.new-task-due { gap: 0.4rem; padding-left: calc(32px + 0.4rem); }`. Así queda alineado con el input de título cuando `.add-mark` mide 36px con `margin-left: -4px`.

## Archivos relevantes

| Ruta | Responsabilidad |
| --- | --- |
| `migrations/20261026000000_add_due_date_to_tasks.up.fizz` (nuevo) | Añade la columna `due_date date NULL` |
| `migrations/20261026000000_add_due_date_to_tasks.down.fizz` (nuevo) | Elimina la columna |
| `models/task.go` | Campo `DueDate`, `DueDateLayout`, `ParseDueDate`, `DueDateValue`, `DueDateLabel` |
| `models/task_test.go` | Tests de parseo, formateo y persistencia |
| `actions/tasks.go` | `taskInput`, `apply`, uso en `TasksCreate` y `TasksUpdate` |
| `actions/tasks_test.go` | Tests HTTP de creación, edición, lista y errores |
| `templates/tasks/index.plush.html` | Campo de fecha al crear, fecha en la lista, errores por campo |
| `templates/tasks/edit.plush.html` | Campo de fecha al editar, errores por campo |
| `templates/application.plush.html` | Versión de caché de `app.css` |
| `public/assets/app.css` | Estilos de `.task-body`, `.task-due`, `input[type="date"]`, `.new-task-due`, `.edit-task-due` |
| `public/assets/app.js` | Solo referencia: depende de `.task-title` y de `.task-actions` como hijo directo; no se modifica |
| `fixtures/tasks.toml`, `migrations/20260901000001_seed_tasks.*.sql` | Solo referencia: no se modifican |

## Implementación paso a paso

1. Crear las dos migraciones fizz con el contenido indicado.
2. Modificar `models/task.go`: constante, campo `DueDate` y las tres funciones. `time` y `strings` ya están importados.
3. Modificar `actions/tasks.go`: añadir `taskInput` y `apply`, y adaptar `TasksCreate` y `TasksUpdate` como se describe. `validate` ya está importado.
4. Modificar `templates/tasks/index.plush.html`, `templates/tasks/edit.plush.html` y `templates/application.plush.html`.
5. Modificar `public/assets/app.css`.
6. Añadir los tests de la sección siguiente.
7. Ejecutar la validación completa y corregir lo que falle sin ampliar el alcance.

## Tests y validación

### Tests nuevos en `models/task_test.go` (métodos de `ModelSuite`)

- `Test_ParseDueDate_Empty`: `""` y `"   "` devuelven `nil` sin error.
- `Test_ParseDueDate_Valid`: `"2026-11-15"` devuelve un valor con `DueDateValue()` (sobre un `Task{DueDate: d}`) `"2026-11-15"` y `DueDateLabel()` `"15/11/2026"`.
- `Test_ParseDueDate_Invalid`: `"2026-02-30"`, `"15/11/2026"`, `"2026-11-15T10:00"` y `"mañana"` devuelven error.
- `Test_Task_DueDate_Optional`: crear sin fecha y recargar con `Find` deja `DueDate` en `nil` y `DueDateValue()` en `""`.
- `Test_Task_DueDate_Persists`: crear con `ParseDueDate("2026-11-15")` y recargar con `Find` da `DueDateValue()` `"2026-11-15"`.

### Tests nuevos en `actions/tasks_test.go`

Usar `url.Values` para enviar `due_date`. Añadir un helper `as.dueDate("2026-11-15") *time.Time` basado en `models.ParseDueDate` con `as.NoError`. Para tareas con fecha, crearlas con `as.DB.ValidateAndCreate`.

- `Test_TasksCreate_WithDueDate`: `title` + `due_date=2026-11-15` da 303 y la tarea guardada tiene `DueDateValue()` `"2026-11-15"`.
- `Test_TasksCreate_EmptyDueDate`: `title` + `due_date=""` da 303 y `DueDate` queda en `nil`.
- `Test_TasksCreate_InvalidDueDate`: `title` + `due_date=2026-02-30` da 422. El cuerpo contiene «La fecha límite no es válida.» y `aria-describedby="due-date-error"`, y no contiene `aria-describedby="title-error"`. No se crea ninguna tarea.
- `Test_TasksCreate_EmptyTitle_KeepsDueDate`: `title=""` + `due_date=2026-11-15` da 422. El cuerpo contiene «El título es obligatorio.» y `value="2026-11-15"`.
- `Test_TasksIndex_ShowsDueDate`: con una tarea con fecha y otra sin fecha, `GET /` contiene `datetime="2026-11-15"` y `Fecha límite: 15/11/2026` una sola vez (`strings.Count`).
- `Test_TasksEdit_PrefillsDueDate`: `GET /tasks/{id}/edit` de una tarea con fecha contiene `name="due_date"` y `value="2026-11-15"`.
- `Test_TasksUpdate_SetsDueDate`: tarea completada sin fecha; `PUT` con `title` + `due_date=2026-12-01` da 303, `DueDateValue()` `"2026-12-01"` y `Completed` sigue en `true`.
- `Test_TasksUpdate_ClearsDueDate`: tarea con fecha; `PUT` con `title` + `due_date=""` da 303 y `DueDate` queda en `nil`.
- `Test_TasksUpdate_InvalidDueDate`: tarea con fecha 2026-11-15; `PUT` con `title=Nuevo título` + `due_date=no-es-una-fecha` da 422 con «La fecha límite no es válida.». Tras `Reload`, el título sigue siendo «Título original» y la fecha `"2026-11-15"`.

Los tests existentes deben seguir pasando sin cambios. `Test_TasksCreate_Valid` envía un `models.Task`: `DueDate` tiene `form:"-"` y no se codifica. `Test_TasksUpdate_EmptyTitle` sigue encontrando `aria-describedby="title-error"`.

### Comandos

En el host, desde la raíz:

```sh
gofmt -l .          # debe no listar nada
go vet ./...
go build ./...
```

Los tests de aplicación necesitan PostgreSQL con la base `todo_test` migrada.

- `database.yml` fija el puerto 5432. En esta máquina ese puerto lo ocupa el contenedor de otro proyecto (`desarrollo-agentico-todo-07-db-1`): **no detenerlo ni usar su base de datos**.
- El servicio `db` de este proyecto no está levantado. No se puede levantar en el 5432 sin conflicto.
- Por eso los tests se ejecutan en una red Docker aislada, con imágenes ya presentes localmente (`postgres:17.6-alpine`, `golang:1.26`):

```sh
docker network create todo03-issue5-test
docker run -d --rm --name todo03-issue5-db --network todo03-issue5-test \
  -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=todo_test \
  postgres:17.6-alpine
# esperar hasta que responda:
docker exec todo03-issue5-db pg_isready -U postgres
docker run --rm --network todo03-issue5-test \
  -v "$PWD":/src -v "$(go env GOMODCACHE)":/go/pkg/mod -w /src \
  -e GO_ENV=test -e DB_HOST=todo03-issue5-db golang:1.26 \
  sh -c 'go install github.com/gobuffalo/pop/v6/soda@v6.4.1 \
    && soda migrate -e test \
    && soda migrate down -e test \
    && soda migrate -e test \
    && go test -p 1 ./...'
# limpieza:
docker stop todo03-issue5-db
docker network rm todo03-issue5-test
```

- `soda migrate down` seguido de `migrate` comprueba que la migración nueva es reversible.
- Si `pg_isready` aún no responde, repetir la comprobación antes de lanzar los tests.
- Si ya existe una red o un contenedor con esos nombres, usar otros nombres en lugar de borrar los existentes.

Comprobación visual opcional (no obligatoria): solo si puede hacerse sin usar el puerto 5432 ocupado.

## Riesgos y recuperación

- **Zona horaria.** Si algún código convierte la fecha a hora local, podría mostrar el día anterior. Mitigación: los helpers formatean sin `Local()` y `Test_Task_DueDate_Persists` comprueba el ida y vuelta contra PostgreSQL.
- **Plush y métodos.** Si Plush no resuelve `t.DueDateValue()` sobre los elementos de `tasks`, el render fallará y los tests de índice lo detectarán. Los métodos deben tener receptor por valor.
- **`PUT` sin `due_date` quita la fecha.** Es aceptable porque los formularios siempre envían el campo. Queda documentado aquí por si en el futuro hay otros clientes.
- **Base de datos de test compartida.** No usar el PostgreSQL del puerto 5432, que pertenece a otro proyecto. Si Docker no está disponible, informar de que los tests no se han ejecutado; no darlos por buenos.
- **Integración con otro trabajo.**
  - La rama parte de `origin/main`. El `main` local tiene un commit sin publicar (`429e9d6`, toggle por `fetch`) que añade tests en `actions/tasks_test.go`; al integrar puede haber un conflicto trivial en ese archivo.
  - Hay otras PR abiertas para esta misma issue (#6 y #7). Hay que elegir una sola.
- **Recuperación.** La migración es aditiva y reversible (`soda migrate down` elimina `due_date`). Revertir el commit deja la aplicación como estaba. No hay pérdida de datos previos porque la columna es nueva y nullable.
