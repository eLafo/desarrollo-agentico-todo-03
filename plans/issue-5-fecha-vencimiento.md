# Feature: fecha límite (solo día) en las tareas

Origen: issue #5 — https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5
Título: «Quiero añadir fecha límite a las tasks». Cuerpo: «Vale con que sea simplemente un día. Sin hora».
Tipo: `feature` (añade una capacidad visible para quien usa la aplicación).

## Encargo y criterios de aceptación

**Requisitos explícitos**

- Una tarea puede tener una fecha límite.
- La fecha límite es un día del calendario, sin hora.

**Inferencias (decididas en este plan)**

- La fecha límite es **opcional**. Las tareas existentes y las nuevas sin fecha siguen funcionando igual.
- Se puede fijar al crear la tarea y cambiar o quitar desde «Editar tarea».
- Se ve en la lista de tareas, debajo del título, con formato `DD/MM/AAAA`.
- Se aceptan fechas pasadas. No se valida que la fecha sea futura, para poder editar tareas ya vencidas.
- El valor se recibe en formato ISO `AAAA-MM-DD`, que es lo que envía `<input type="date">`. Un campo vacío, o ausente, significa «sin fecha límite».
- Un valor con otro formato solo puede llegar desde clientes que no usan el formulario. Se rechaza con `400 Bad Request` sin guardar nada.

**Criterios de aceptación**

1. El formulario «Añadir tarea» de `/` tiene un campo de fecha (`type="date"`, `name="due_date"`) con la etiqueta accesible «Fecha límite (opcional)».
2. `POST /tasks` con `title=Pagar el alquiler&due_date=2026-10-30` crea la tarea con fecha límite `2026-10-30`, responde `303` y redirige a `/`.
3. `POST /tasks` con `due_date` vacío o ausente crea la tarea sin fecha límite (`NULL` en la base de datos).
4. `POST /tasks` con `due_date=30/10/2026` (u otro valor que no sea una fecha `AAAA-MM-DD` válida) responde `400` y no crea ninguna tarea.
5. Si el título no es válido (`422`), el formulario vuelve a mostrar la fecha enviada en el campo de fecha.
6. En la lista, una tarea con fecha límite muestra `Fecha límite: 30/10/2026` en un `<time datetime="2026-10-30">`. Una tarea sin fecha no muestra nada nuevo.
7. `GET /tasks/{id}/edit` incluye un campo «Fecha límite» precargado con `value="2026-10-30"`, o vacío si la tarea no tiene fecha.
8. `PUT /tasks/{id}` con `due_date=2026-11-05` cambia la fecha. Con `due_date` vacío o ausente la quita. Con un valor no válido responde `400` y no cambia ni el título ni la fecha.
9. Completar, reabrir o editar el título de una tarea conserva su fecha límite.
10. La fecha guardada es exactamente el día elegido, sin desplazamientos por zona horaria (se guarda en una columna `date`).

## Comportamiento actual y propuesta

**Actual.** La tabla `tasks` solo tiene `id`, `title`, `completed`, `created_at` y `updated_at` (`migrations/20260901000000_create_tasks.up.fizz`). `models.Task` refleja esas columnas. `TasksCreate` enlaza el formulario con `c.Bind(task)`; solo `title` tiene etiqueta `form`. `TasksUpdate` enlaza solo `title` en una estructura anónima para no permitir asignación masiva. Los templates `tasks/index.plush.html` y `tasks/edit.plush.html` solo muestran y editan el título.

**Propuesta.** Hacer un cambio vertical mínimo siguiendo los patrones existentes:

- Nueva migración fizz que añade la columna `due_date date NULL`.
- Campo `DueDate nulls.Time` en `models.Task` con `form:"-"`, para que el binder no lo rellene. Se añaden una función `ParseDueDate` y dos métodos de presentación.
- Las acciones leen `due_date` como texto, lo convierten con `models.ParseDueDate` y responden `400` si no es válido. `TasksUpdate` añade `DueDate` a su estructura de entrada.
- Los templates añaden el campo de fecha en los dos formularios y la fecha en cada elemento de la lista. Se añade el CSS mínimo y se sube la versión de `app.css` para invalidar la caché.

No se usa el binder de Buffalo para la fecha. Su decodificador personalizado está registrado para el tipo interno `github.com/gobuffalo/buffalo/internal/nulls.Time`, no para `github.com/gobuffalo/nulls.Time` (`buffalo@v1.1.4/binding/binding.go`). Además, un error de enlace acabaría en un `500`.

## Alcance

**Incluido**

- Migración `up` y `down` de la columna `tasks.due_date` (`date`, nulable).
- Modelo: campo, conversión desde el formulario y formato de presentación.
- Acciones `TasksCreate` y `TasksUpdate`: leer, validar el formato y asignar la fecha.
- Templates: campo de fecha en la creación y en la edición, y la fecha en la lista.
- CSS para el nuevo campo y para la línea de fecha, con versión nueva de `app.css` en el layout.
- `go.mod`: `github.com/gobuffalo/nulls` (ya presente como `// indirect`, v0.4.2) pasa a ser dependencia directa. No cambia ninguna versión.
- Tests de modelo y de acciones.

**Excluido**

- Hora o zona horaria en la fecha límite.
- Marcar visualmente tareas vencidas o próximas a vencer (requiere decidir la zona horaria de «hoy»).
- Ordenar o filtrar por fecha límite. El orden sigue siendo `created_at asc, id asc`.
- Recordatorios o notificaciones.
- Validar que la fecha sea futura.
- Cambios en `public/assets/app.js`, en `fixtures/tasks.toml` o en la migración de datos semilla.
- Mostrar errores de formato de fecha dentro del formulario. El navegador solo envía fechas válidas o vacío; los valores mal formados reciben `400`.

## Diseño por capas

### Persistencia y migraciones

Crea dos archivos nuevos con una versión posterior a las migraciones existentes:

`migrations/20261026000000_add_due_date_to_tasks.up.fizz`

```
add_column("tasks", "due_date", "date", {"null": true})
```

`migrations/20261026000000_add_due_date_to_tasks.down.fizz`

```
drop_column("tasks", "due_date")
```

`migrations/embed.go` ya incluye `*.fizz`. `cmd/app/main.go` aplica las migraciones al arrancar (`models.Migrate()`), así que en desarrollo y en producción la columna se crea sola. La base de test se migra con `soda` (ver «Tests y validación»).

Una columna `date` evita desplazamientos de día. Pop v6 usa `pgx/v5/stdlib`, que codifica un `time.Time` en un parámetro `date` con su año, mes y día, y al leer devuelve medianoche UTC.

### Modelo (`models/task.go`)

- Importa `github.com/gobuffalo/nulls`.
- Añade la constante y el campo. Coloca el campo después de `Completed`:

```go
// DueDateLayout es el formato AAAA-MM-DD con el que se recibe la fecha límite.
const DueDateLayout = "2006-01-02"

// dentro de Task:
DueDate   nulls.Time `json:"due_date" db:"due_date" form:"-"`
```

- Añade la conversión y la presentación. Usa receptores por valor, porque la lista itera `models.Tasks` (valores `Task`) en Plush:

```go
// ParseDueDate convierte el valor de un campo de fecha (AAAA-MM-DD) en una
// fecha límite opcional. Un valor vacío significa que la tarea no tiene fecha límite.
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

// DueDateISO devuelve la fecha límite como AAAA-MM-DD, o "" si no tiene.
func (t Task) DueDateISO() string {
	if !t.DueDate.Valid {
		return ""
	}
	return t.DueDate.Time.Format(DueDateLayout)
}

// DueDateDisplay devuelve la fecha límite como DD/MM/AAAA, o "" si no tiene.
func (t Task) DueDateDisplay() string {
	if !t.DueDate.Valid {
		return ""
	}
	return t.DueDate.Time.Format("02/01/2006")
}
```

`Validate` no cambia: la fecha es opcional y su formato se controla en `ParseDueDate`.

### Acciones (`actions/tasks.go`)

Las rutas no cambian (`actions/app.go`).

`TasksCreate`: después de `task.Title = strings.TrimSpace(task.Title)` y antes de `tx.ValidateAndCreate(task)`:

```go
dueDate, err := models.ParseDueDate(c.Param("due_date"))
if err != nil {
	return c.Error(http.StatusBadRequest, err)
}
task.DueDate = dueDate
```

`c.Param` incluye los campos del cuerpo del formulario (`buffalo@v1.1.4/context.go`, `newContext`). La rama `422` ya vuelve a poner `task` en el contexto, así que la fecha enviada se vuelve a mostrar sin más cambios.

`TasksUpdate`:

- Actualiza el comentario: «TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.»
- Amplía la estructura de entrada:

```go
input := struct {
	Title   string `form:"title" json:"title"`
	DueDate string `form:"due_date" json:"due_date"`
}{}
```

- Después de `c.Bind(&input)` y **antes** de modificar `task`:

```go
dueDate, err := models.ParseDueDate(input.DueDate)
if err != nil {
	return c.Error(http.StatusBadRequest, err)
}
task.Title = strings.TrimSpace(input.Title)
task.DueDate = dueDate
```

`TasksToggle`, `TasksEdit`, `TasksDestroy` y `TasksIndex` no cambian. `TasksToggle` carga la tarea completa con `tx.Find` y la guarda con `tx.Update`, por lo que conserva la fecha.

### Renderizado (`templates/tasks/*.plush.html`)

`templates/tasks/index.plush.html`, lista:

- Envuelve el título en un contenedor nuevo y añade la fecha debajo. `span.task-title` se conserva igual, porque `app.js` lee su `textContent` para el `aria-label`. `span.task-status` y `div.task-actions` siguen siendo hijos directos del `li`, porque `app.js` inserta el estado antes de `.task-actions`:

```html
<div class="task-content">
  <span class="task-title"><%= t.Title %></span>
  <%= if (t.DueDate.Valid) { %>
    <span class="task-due">Fecha límite: <time datetime="<%= t.DueDateISO() %>"><%= t.DueDateDisplay() %></time></span>
  <% } %>
</div>
```

`templates/tasks/index.plush.html`, formulario «Añadir tarea»: dentro de `.new-task-row`, entre el `input#task-title` y el botón «Añadir»:

```html
<label for="task-due-date" class="sr-only">Fecha límite (opcional)</label>
<input
  id="task-due-date"
  type="date"
  name="due_date"
  value="<%= task.DueDateISO() %>"
  title="Fecha límite (opcional)"
>
```

`templates/tasks/edit.plush.html`: después del bloque `title-error` y antes de `.edit-task-actions`:

```html
<label for="task-due-date">Fecha límite</label>
<input
  id="task-due-date"
  type="date"
  name="due_date"
  value="<%= task.DueDateISO() %>"
  aria-describedby="due-date-help"
>
<p id="due-date-help" class="field-hint">Opcional. Solo el día, sin hora. Déjala vacía para quitarla.</p>
```

El resto de los templates, incluido el manejo de errores del título, no cambia.

### CSS (`public/assets/app.css`) y layout

En `templates/application.plush.html`, cambia `/assets/app.css?v=3` por `/assets/app.css?v=4`. No toques la línea de `app.js`.

En `app.css`:

1. Sustituye la regla `.task-title` por estas tres reglas. Sin fecha, el aspecto es el mismo que ahora:

```css
.task-content {
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

2. Amplía la regla `input[type="text"] { … }` para que su selector sea `input[type="text"], input[type="date"]`, con un selector por línea como en el resto del archivo.
3. Cambia `.new-task-row input { flex: 1; }` por `.new-task-row input[type="text"] { flex: 1; }` y añade:

```css
.new-task-row input[type="date"] {
  width: auto;
  flex-shrink: 0;
}
```

4. Añade espacio antes de la segunda etiqueta del formulario de edición:

```css
.edit-task label:not(:first-of-type) {
  margin-top: 1.5rem;
}
```

5. Dentro de `@media (max-width: 760px)`, en móvil la fecha y el botón pasan a una segunda línea alineada con el título. Añade:

```css
  .new-task-row {
    flex-wrap: wrap;
  }

  .new-task-row input[type="text"] {
    flex-basis: calc(100% - 36px - 0.4rem);
  }

  .new-task-row input[type="date"] {
    flex: 1;
    margin-left: calc(36px + 0.4rem);
  }
```

Para no repetir `gap`, combínalo con la regla `.new-task-row { gap: 0.4rem; }` que ya existe en ese bloque.

6. Dentro de `@media (max-width: 380px)`, elimina `.new-task-row { flex-wrap: wrap; }` y `.new-task-row input { flex-basis: calc(100% - 44px); }`, porque el bloque de 760px ya cubre ese caso. Conserva `.new-task-row .button { margin-left: auto; }`.

No se necesita JavaScript.

## Archivos relevantes

| Ruta | Responsabilidad |
| --- | --- |
| `migrations/20261026000000_add_due_date_to_tasks.up.fizz` (nuevo) | Añade `tasks.due_date date NULL`. |
| `migrations/20261026000000_add_due_date_to_tasks.down.fizz` (nuevo) | Elimina la columna. |
| `migrations/embed.go` | Incrusta las migraciones (sin cambios; ya incluye `*.fizz`). |
| `models/task.go` | Campo `DueDate`, `DueDateLayout`, `ParseDueDate`, `DueDateISO`, `DueDateDisplay`. |
| `models/task_test.go` | Tests de persistencia y conversión de la fecha. |
| `actions/tasks.go` | `TasksCreate` y `TasksUpdate` leen y validan `due_date`. |
| `actions/tasks_test.go` | Tests HTTP de creación, edición, lista y toggle con fecha. |
| `actions/app.go` | Rutas (sin cambios). |
| `templates/tasks/index.plush.html` | Campo de fecha en la creación; fecha en la lista. |
| `templates/tasks/edit.plush.html` | Campo de fecha en la edición. |
| `templates/application.plush.html` | Versión de `app.css` (`?v=4`). |
| `public/assets/app.css` | Estilos del contenedor, de la fecha y de los campos `date`. |
| `public/assets/app.js` | Usa `.task-title`, `.task-status` y `.task-actions`; no cambia, pero el HTML debe respetar esos selectores. |
| `go.mod` | `github.com/gobuffalo/nulls v0.4.2` pasa al bloque de dependencias directas. |
| `.github/workflows/ci.yaml` | CI: `gofmt`, `go vet`, `soda migrate -e test`, `go test -p 1 ./...`, `go build ./...`. |

## Implementación paso a paso

1. Crea las dos migraciones fizz del apartado «Persistencia y migraciones».
2. Modifica `models/task.go`: añade el import, la constante, el campo, `ParseDueDate`, `DueDateISO` y `DueDateDisplay`.
3. Ejecuta `go mod tidy`. El único cambio esperado en `go.mod` es que `github.com/gobuffalo/nulls v0.4.2` pase del bloque `// indirect` al bloque directo; `go.sum` no debe cambiar. Antes de este cambio, `go mod tidy -diff` no mostraba diferencias. Si aparece otro cambio, detente y revisa.
4. Modifica `actions/tasks.go`: `TasksCreate` y `TasksUpdate` como se describe en «Acciones».
5. Modifica `templates/tasks/index.plush.html` (lista y formulario) y `templates/tasks/edit.plush.html`.
6. Modifica `public/assets/app.css` y sube `app.css` a `?v=4` en `templates/application.plush.html`.
7. Añade los tests del apartado siguiente.
8. Ejecuta la validación completa.

## Tests y validación

### Tests de modelo (`models/task_test.go`, métodos de `ModelSuite`)

Importa `time` y `github.com/gobuffalo/nulls`.

- `Test_Task_DueDate_OptionalByDefault`: crea `&Task{Title: "Sin fecha"}`, recárgala con `Find` y comprueba que `DueDate.Valid` es `false`.
- `Test_Task_DueDate_PersistsDay`: crea una tarea con `DueDate: nulls.NewTime(time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC))`, recárgala con `Find` y comprueba que `Valid` es `true` y que `DueDate.Time.Format(DueDateLayout) == "2026-10-30"`.
- `Test_ParseDueDate`, sin base de datos y con aserciones directas:
  - `""` y `"   "` → sin error y `Valid == false`.
  - `"2026-10-30"` → sin error, `Valid == true`, día `2026-10-30`.
  - `"30/10/2026"`, `"2026-02-30"`, `"2026-10-30T10:00"` y `"mañana"` → error.
- `Test_Task_DueDateFormatting`: con fecha, `DueDateISO() == "2026-10-30"` y `DueDateDisplay() == "30/10/2026"`; sin fecha, los dos devuelven `""`.

### Tests de acciones (`actions/tasks_test.go`, métodos de `ActionSuite`)

Importa `time` y `github.com/gobuffalo/nulls`. Para crear tareas con fecha, usa `as.DB.ValidateAndCreate(&models.Task{Title: …, DueDate: nulls.NewTime(time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC))})`. Para enviar la fecha, usa `url.Values`: `Post(&models.Task{…})` no envía `due_date`, porque el campo tiene `form:"-"`.

- `Test_TasksIndex_HasDueDateField`: `GET /` contiene `type="date"`, `name="due_date"` y `Fecha límite (opcional)`.
- `Test_TasksIndex_ShowsDueDate`: crea una tarea con fecha y otra sin fecha. `GET /` contiene `datetime="2026-10-30"` y `30/10/2026`, y `strings.Count(body, "class=\"task-due\"") == 1`.
- `Test_TasksCreate_WithDueDate`: `POST /tasks` con `title` y `due_date=2026-10-30` → `303` y `Location: /`. `as.DB.First(task)` tiene `DueDate.Valid` y el día `2026-10-30`.
- `Test_TasksCreate_WithoutDueDate`: `due_date` vacío → `303` y `DueDate.Valid == false`.
- `Test_TasksCreate_InvalidDueDate`: `due_date=30/10/2026` → `400` y `Count == 0`.
- `Test_TasksCreate_EmptyTitle_KeepsDueDate`: `title=""` y `due_date=2026-10-30` → `422`; el cuerpo contiene `value="2026-10-30"` y `Count == 0`.
- `Test_TasksEdit_PrefillsDueDate`: la tarea tiene fecha → el cuerpo de `GET /tasks/{id}/edit` contiene `value="2026-10-30"` y `name="due_date"`.
- `Test_TasksUpdate_SetsDueDate`: la tarea no tiene fecha. `PUT` con `title` y `due_date=2026-11-05` → `303`; tras recargar, el día es `2026-11-05` y el título está actualizado.
- `Test_TasksUpdate_ClearsDueDate`: la tarea tiene fecha. `PUT` con `title` y `due_date=""` → `303`; tras recargar, `DueDate.Valid == false`.
- `Test_TasksUpdate_InvalidDueDate`: la tarea tiene fecha `2026-10-30`. `PUT` con `title=Otro título` y `due_date=mañana` → `400`; tras recargar, el título y la fecha no cambian.
- `Test_TasksUpdate_EmptyTitle_KeepsSubmittedDueDate`: `PUT` con `title=""` y `due_date=2026-11-05` → `422`; el cuerpo contiene `value="2026-11-05"` y la base de datos no cambia.
- `Test_TasksToggle_KeepsDueDate`: la tarea tiene fecha. Haz `PUT /tasks/{id}/toggle`; tras recargar, sigue completada y con fecha `2026-10-30`.

Los tests existentes deben seguir en verde sin cambios. En particular, `Test_TasksUpdate_PersistsTitleOnly` no envía `due_date` y la tarea sigue sin fecha.

### Requisitos para ejecutar los tests

Necesitas PostgreSQL y la base `todo_test` migrada con la nueva migración. `soda` está en `$(go env GOPATH)/bin`, que no está en el `PATH`:

```sh
docker compose up -d db
$(go env GOPATH)/bin/soda create -e test   # si todo_test no existe; si ya existe, ignora el error
$(go env GOPATH)/bin/soda migrate -e test
```

### Comprobaciones

- `gofmt -l .` → sin salida.
- `go vet ./...`
- `go mod tidy -diff` → sin salida (tras el paso 3).
- `GO_ENV=test go test -p 1 ./...` → todo en verde.
- `go build ./...`
- Reversibilidad de la migración: `$(go env GOPATH)/bin/soda migrate down -e test` y después `$(go env GOPATH)/bin/soda migrate -e test`. Las dos deben terminar sin error.

### Validación manual en navegador

`public/` y `templates/` van incrustados en el binario: reconstruye con `docker compose up --build -d` o `/env:start`. Abre `http://localhost:3000/` y comprueba lo siguiente:

- Las tareas semilla se ven igual que antes y no muestran fecha.
- Crea una tarea con fecha: aparece `Fecha límite: DD/MM/AAAA` bajo el título.
- Crea otra sin fecha: no aparece nada nuevo.
- Edita la tarea: la fecha aparece precargada. Cámbiala y guarda; después vacíala y guarda. La lista refleja los dos cambios.
- Completa y reabre una tarea con fecha: la fecha se conserva al recargar.
- Con un ancho de 375 px, la fecha y el botón «Añadir» quedan en una segunda línea alineada con el título.

## Riesgos y recuperación

- **Desplazamiento de un día por zona horaria.** Se mitiga con la columna `date` y con `time.Parse`, que usa UTC. `Test_Task_DueDate_PersistsDay` y los tests de acciones lo verifican tras recargar desde la base de datos.
- **Navegadores sin `<input type="date">`.** Mostrarían un campo de texto, y un formato distinto de `AAAA-MM-DD` produciría `400`. Todos los navegadores actuales soportan este campo; el riesgo se acepta.
- **Migración en producción.** `add_column` nulable no reescribe datos y es compatible con el código anterior. Para revertir, despliega la versión anterior. Si hace falta quitar la columna, ejecuta `soda migrate down` (pierde las fechas guardadas).
- **Caché del CSS.** Sin `?v=4`, los navegadores podrían usar el CSS antiguo y descolocar la fecha. El cambio de versión lo evita.
- **Selectores de `app.js`.** Si `.task-title`, `.task-status` o `.task-actions` cambian de posición, el toggle optimista dejaría de actualizar etiquetas. El diseño los conserva; compruébalo en la validación manual.
- **Trabajo paralelo.** Hay otras ramas o PR abiertos para la issue #5 y un commit local en `main` (`429e9d6`) que todavía no está en `origin/main`. Ese commit toca `actions/tasks_test.go`, `public/assets/app.js` y la línea de `app.js` en `templates/application.plush.html`. Al integrar puede haber conflictos triviales en `actions/tasks_test.go` (tests añadidos al final): resuélvelos conservando ambos bloques. Solo debe integrarse una implementación de esta feature.
