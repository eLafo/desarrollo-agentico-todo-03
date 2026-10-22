# Feature: fecha límite (solo día) en las tareas

## Encargo y criterios de aceptación

Issue #5: «Quiero añadir fecha límite a las tasks. Vale con que sea simplemente un día. Sin hora».

Requisitos explícitos:
- Cada tarea puede tener una fecha límite que es un día de calendario, sin hora.

Inferencias (a confirmar en revisión, no bloquean):
- La fecha es **opcional**: las tareas existentes y las creadas sin fecha siguen siendo válidas.
- Se puede fijar al crear la tarea y cambiar o borrar al editarla.
- Se muestra en la lista junto al título.

Criterios de aceptación:
1. El formulario de creación tiene un campo de fecha (`<input type="date">`); crear con fecha la persiste y la lista la muestra con formato `dd/mm/aaaa`.
2. Crear sin fecha sigue funcionando y no muestra ninguna fecha.
3. El formulario de edición muestra la fecha actual; guardar una fecha nueva la cambia; guardar el campo vacío la elimina.
4. Una fecha mal formada (p. ej. `31/02/2026` o `abc`) devuelve 422 con el mensaje `La fecha límite no es válida.`, sin crear ni modificar la tarea, y conserva el valor escrito.
5. Una tarea con fecha anterior a hoy y sin completar se marca visualmente como vencida (clase `task-overdue` y texto accesible «Vencida»); una completada nunca se marca como vencida.
6. Alternar completada (`TasksToggle`) y el resto de acciones conservan la fecha.
7. No se rechazan fechas pasadas al guardar (se pueden registrar tareas atrasadas).

## Comportamiento actual y propuesta

Actual: `tasks` tiene `id`, `title`, `completed`, timestamps (`migrations/20260901000000_create_tasks.up.fizz`). `TasksCreate` enlaza el formulario directamente en `models.Task`; `TasksUpdate` enlaza solo `title` en una struct de entrada y conserva el resto. Las plantillas Plush `tasks/index` y `tasks/edit` solo manejan título.

Propuesta: nueva columna nullable `due_date` de tipo `date`, campo `DueDate nulls.Time` en el modelo, campo `due_date` en ambos formularios, parseo explícito `YYYY-MM-DD` en las acciones y renderizado en la lista.

## Alcance

Incluido: migración, modelo, acciones create/update, plantillas index y edit, CSS, tests de modelo y acciones, fixture.

Excluido: hora, zona horaria por tarea, orden o filtro por fecha, recordatorios/notificaciones, validación de fechas pasadas, API JSON, cambios en `public/assets/app.js` (el toggle solo cambia `.task-completed`; el CSS oculta el estado vencido cuando la tarea está completada, ver abajo).

## Diseño por capas

**Persistencia.** Nueva migración fizz `migrations/<timestamp posterior a 20260901000001>_add_due_date_to_tasks.up.fizz`: `add_column("tasks", "due_date", "date", {"null": true})`; `down.fizz`: `drop_column("tasks", "due_date")`. Seguir el nombre/formato de las migraciones existentes. Tipo `date` evita problemas de zona horaria: no hay hora que desplazar.

**Modelo (`models/task.go`).** Añadir `DueDate nulls.Time `json:"due_date" db:"due_date" form:"-"`` (import `github.com/gobuffalo/nulls`, ya dependencia transitiva de pop; comprobar con `go build` y añadir a `go.mod` como directa si `go mod tidy` lo pide). Añadir métodos:
- `DueDateValue() string`: `YYYY-MM-DD` si `Valid`, si no `""` (para `value` del input).
- `DueDateLabel() string`: `dd/mm/aaaa` o `""`.
- `Overdue(today time.Time) bool`: `!Completed && DueDate.Valid && DueDate.Time` anterior al día de `today` (comparar solo año/mes/día).
Leer `time.Time` de una columna `date` con lib/pq devuelve medianoche UTC; comparar por componentes de fecha y no por instante.

**Acciones (`actions/tasks.go`).** Helper privado `parseDueDate(raw string) (nulls.Time, bool)`: `strings.TrimSpace`; vacío → `nulls.Time{}`, `true`; si no `time.Parse("2006-01-02", raw)`; error → `false`.
- `TasksCreate`: no depender del binder para la fecha (`form:"-"`); leer `c.Param("due_date")` tras `c.Bind(task)`, parsear y asignar. Si el parseo falla, añadir error `due_date` a los `verrs` y renderizar 422 igual que el caso de título inválido (conservar el texto escrito, ver plantilla).
- `TasksUpdate`: ampliar la struct de entrada con `DueDate string `form:"due_date"``; mismo helper; fecha vacía → NULL.
- `TasksToggle`/`TasksDestroy`: sin cambios.
- Como hay que combinar errores de título y fecha, añadir el error de fecha al `*validate.Errors` devuelto por `ValidateAndCreate/Update` (o validar antes y no llamar a la BD). El implementador elige lo más simple siempre que 422 no persista nada.

**Plantillas.**
- `templates/tasks/index.plush.html`: en el formulario `new-task`, añadir `<input type="date" name="due_date" aria-label="Fecha límite" value="...">` dentro de `.new-task-row` antes del botón; mensaje de error con `id="due-date-error"` y `aria-describedby` cuando exista error de fecha (separar `errors.Get("title")` y `errors.Get("due_date")`: hoy `errors.HasAny()` se usa para el título y debe pasar a `errors.Get("title")` no vacío, para no marcar el título inválido cuando solo falla la fecha). En cada tarea, tras el título: `<%= if (t.DueDateLabel() != "") { %><span class="task-due<%= if (t.Overdue(now)) { %> task-overdue<% } %>">...` con texto `Fecha límite: dd/mm/aaaa` y, si vencida, ` (Vencida)`. `Overdue` necesita «hoy»: pasar `c.Set("today", time.Now())` desde `TasksIndex` (y desde la rama 422 de `TasksCreate`) y llamar `t.Overdue(today)`. Si Plush no permite pasar el argumento tal cual, calcularlo en la acción con un helper o campo no persistido; elegir la opción más simple.
- `templates/tasks/edit.plush.html`: campo `<label for="task-due-date">Fecha límite</label>` + `<input id="task-due-date" type="date" name="due_date" value="<%= task.DueDateValue() %>">`, con hint «Opcional. Deja vacío para quitarla.» y error. En la rama 422 de `TasksUpdate`, rellenar `task.DueDate` con el valor enviado cuando sea válido; si es inválido, pasar el texto crudo con `c.Set("dueDateRaw", raw)` y usarlo como `value`. Igual en `index` para creación.
- Actualizar el hint del título de edición («Los demás datos de la tarea no cambian») para que no sea falso.

**CSS (`public/assets/app.css`).** Estilos `.task-due` (color `--color-muted`, tamaño pequeño), `.task-overdue` (color `--color-danger`), `.task-completed .task-due` sin énfasis de vencida (regla que anula el color) y ajuste de `.new-task-row` para el input de fecha (y bloque `@media` existente en móvil, líneas ~449-471). Mantener contraste y el patrón de variables existente.

## Archivos relevantes

- `migrations/*_create_tasks.up.fizz`, `migrations/embed.go`: esquema actual y formato/embedding de migraciones.
- `models/task.go`: modelo y validación.
- `actions/tasks.go`: acciones; `actions/app.go`: rutas (sin cambios).
- `templates/tasks/index.plush.html`, `templates/tasks/edit.plush.html`: vistas.
- `public/assets/app.css`: estilos.
- `models/task_test.go`, `actions/tasks_test.go`: tests existentes; `fixtures/tasks.toml`: fixtures.

## Implementación paso a paso

1. Crear la migración up/down con timestamp posterior a la última.
2. Añadir `DueDate` y los tres métodos al modelo.
3. Implementar `parseDueDate` y cablear `TasksCreate`/`TasksUpdate`, incluido el 422 con error de fecha.
4. Pasar `today` a la vista en `TasksIndex` y `TasksCreate`(422).
5. Editar las plantillas `index` y `edit`; separar los errores de título y fecha.
6. Añadir CSS.
7. Añadir fixture con tareas con fecha (una vencida pendiente, una futura, una vencida completada) en un escenario nuevo, sin alterar el escenario «tres tareas» que usan los tests actuales.
8. Escribir los tests y ejecutarlos.

## Tests y validación

Modelo (`models/task_test.go`):
- Crear con y sin `DueDate`; persiste y se recupera el mismo día.
- `Overdue`: pendiente con fecha ayer → true; hoy → false; mañana → false; completada con fecha ayer → false; sin fecha → false.
- `DueDateValue`/`DueDateLabel` con y sin fecha.

Acciones (`actions/tasks_test.go`):
- Create con `due_date=2026-12-31` persiste y la lista contiene `31/12/2026`.
- Create sin `due_date` → NULL.
- Create con `due_date=abc` → 422, mensaje `La fecha límite no es válida.`, 0 tareas.
- Update cambia la fecha, update con vacío la borra, update con inválida → 422 sin modificar nada.
- Update y Toggle conservan la fecha cuando corresponde; los tests existentes de update que no envían `due_date` deben revisarse: ahora enviar el campo vacío borra la fecha. Decisión: si `due_date` **no está presente** en el formulario, conservar la fecha (`c.Request().Form` / `PostForm` con `_, ok :=`); si está presente y vacío, borrarla. Añadir test de ambos casos.
- Lista marca `task-overdue` solo en pendientes vencidas.

Comandos (requieren PostgreSQL y `GO_ENV=test soda migrate -e test` aplicado con la migración nueva, como en `.github/workflows/ci.yaml`):
`gofmt -l .`, `go vet ./...`, `go build ./...`, `go test -p 1 ./...`.
Comprobar la migración también con `soda migrate down` y `up` en test.

## Riesgos y recuperación

- Enlace de `nulls.Time` desde formulario: por eso se parsea a mano y el campo es `form:"-"`.
- Zona horaria: `Overdue` compara fechas por componentes con «hoy» en hora local del servidor; un servidor en UTC puede marcar vencida unas horas antes/después para usuarios en otras zonas. Aceptado para esta versión.
- Orden de migraciones: el timestamp debe ser mayor que `20260901000001`, o `soda` no la aplicará en bases ya migradas.
- Recuperación: `soda migrate down` deshace la columna; revertir el commit elimina el resto sin pérdida de datos previos (la columna es aditiva y nullable).
- Incertidumbre abierta no bloqueante: la issue no pide mostrar «vencida» ni orden por fecha; se incluye solo el marcado visual mínimo (criterio 5). Si se considera ampliación, eliminarlo junto con `Overdue` y el CSS asociado.
