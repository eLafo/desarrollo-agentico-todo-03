# Feature: fecha límite (solo día) en las tareas

Origen: issue #5 — <https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5>

> **Título:** Quiero añadir fecha límite a las tasks
> **Cuerpo:** Vale con que sea simplemente un día. Sin hora

## Encargo y criterios de aceptación

### Requisitos explícitos

1. Una tarea puede tener una fecha límite.
2. La fecha límite es un día del calendario, sin hora.

### Inferencias (decididas en este plan)

3. La fecha límite es **opcional**. Se pueden seguir creando tareas sin ella, y las tareas existentes quedan sin fecha límite tras la migración.
4. La fecha se asigna al crear la tarea, en el formulario de la lista. Se puede cambiar o quitar desde el formulario de edición.
5. La lista muestra la fecha de las tareas que la tienen como `Fecha límite: DD/MM/AAAA` (por ejemplo `Fecha límite: 05/11/2026`) dentro de `<time datetime="AAAA-MM-DD">`. Las tareas sin fecha no muestran nada.
6. Completar o reabrir una tarea no cambia su fecha límite.
7. El servidor solo acepta `AAAA-MM-DD` o una cadena vacía. Un valor con otro formato o un día inexistente (`2026-02-30`) responde **422** y no guarda nada.
8. Si al crear una tarea el título no es válido (respuesta 422), el formulario vuelve a mostrar la fecha que se había introducido.

### Criterios de aceptación verificables

- `POST /tasks` con `title=X&due_date=2026-11-05` crea la tarea con fecha `2026-11-05` y redirige (303) a `/`.
- `POST /tasks` sin `due_date`, o con el campo vacío, crea la tarea sin fecha límite (`NULL`).
- `POST /tasks` con `due_date=05/11/2026` responde 422 y no crea ninguna tarea.
- `POST /tasks` con título vacío y `due_date=2026-11-05` responde 422, y el HTML incluye `value="2026-11-05"`.
- `GET /` muestra `Fecha límite: 05/11/2026` y `datetime="2026-11-05"` para una tarea con esa fecha, y no muestra `class="task-due"` para las tareas sin fecha.
- `GET /tasks/{id}/edit` rellena el campo `due_date` con `value="AAAA-MM-DD"`.
- `PUT /tasks/{id}` con `due_date=2026-12-01` cambia la fecha, y con `due_date=` la elimina. En ambos casos conserva `completed`.
- `PUT /tasks/{id}` con una fecha mal formada responde 422 y no cambia ni el título ni la fecha.
- `PUT /tasks/{id}/toggle` conserva la fecha límite.
- Todos los tests existentes siguen pasando.

## Comportamiento actual y propuesta

**Actual.** La tabla `tasks` tiene `id`, `title`, `completed`, `created_at` y `updated_at` (`migrations/20260901000000_create_tasks.up.fizz`). `models.Task` no tiene ningún campo de fecha de negocio. `TasksCreate` hace `c.Bind(task)` sobre `models.Task`, y solo `title` tiene la etiqueta `form`. `TasksUpdate` hace bind de una lista blanca anónima `{Title}` y conserva el resto de los datos. Las plantillas `templates/tasks/index.plush.html` y `edit.plush.html` solo tienen el campo título.

**Propuesta.** Se hace un cambio vertical mínimo que sigue los patrones existentes:

- **Persistencia:** una nueva migración fizz añade la columna `due_date` de tipo `date` que admite `NULL`. El tipo `date` de PostgreSQL no guarda hora ni zona horaria.
- **Modelo:** se añade el campo `DueDate *time.Time` con `form:"-"`, de modo que el bind genérico nunca lo rellena. También se añaden `ParseDueDate` (de texto a fecha) y tres métodos de presentación con receptor por valor para usarlos desde Plush: `HasDueDate`, `DueDateValue` y `DueDateLabel`.
- **Acciones:** `TasksCreate` lee `due_date` con `c.Param`, y `TasksUpdate` lo añade a su lista blanca. Ambas acciones lo convierten con `models.ParseDueDate`. Si el valor es inválido devuelven `c.Error(422, err)`. Si es válido lo asignan antes de `ValidateAndCreate` o `ValidateAndUpdate`.
- **Vistas:** un `<input type="date" name="due_date">` en el formulario de creación y en el de edición, y un `<time class="task-due">` en cada elemento de la lista.
- **CSS:** estilos para el campo de fecha y para la nueva línea de la fecha bajo el título. Se sube la versión del enlace de `app.css` de `v=3` a `v=4`.

**Decisión sobre el formato inválido.** El navegador envía `<input type="date">` siempre como `AAAA-MM-DD` o como cadena vacía. Solo una petición manipulada puede enviar otro formato. Por eso el plan no añade mensajes de error por campo a las plantillas: responde 422 con `c.Error` sin guardar nada. Así las plantillas mantienen su lógica de errores actual, que solo cubre el título.

## Alcance

### Incluido

- Migración `up`/`down` para `tasks.due_date` (`date`, admite `NULL`).
- Campo y funciones auxiliares en `models.Task`.
- Lectura, validación de formato y persistencia de `due_date` en `TasksCreate` y `TasksUpdate`.
- Campo de fecha en los formularios de creación y edición, y visualización en la lista.
- Estilos CSS mínimos y cambio de versión de `app.css`.
- Tests de modelo y de acciones.

### Excluido

- Hora, zonas horarias y recordatorios o notificaciones.
- Resaltar las tareas vencidas o las que vencen hoy.
- Ordenar o filtrar por fecha límite. El orden sigue siendo `created_at asc, id asc`.
- Prohibir fechas pasadas.
- Nombres de mes localizados. Se usa el formato numérico `DD/MM/AAAA`.
- Mensajes de error por campo en las plantillas.
- Cambios en `public/assets/app.js`. Su lógica de toggle no depende de la estructura que cambia (ver Riesgos).
- Cambios en `fixtures/tasks.toml`, en la migración de semillas y en `go.mod`.
- El commit local `429e9d6` (`fix(ui): guardar el estado al completar o reabrir una tarea`). Está en `main` local pero no en `origin/main`, la base de esta rama, y no forma parte de este encargo.

## Diseño por capas

### Persistencia y migración

Archivo nuevo `migrations/20261027120000_add_due_date_to_tasks.up.fizz`:

```
add_column("tasks", "due_date", "date", {"null": true})
```

Archivo nuevo `migrations/20261027120000_add_due_date_to_tasks.down.fizz`:

```
drop_column("tasks", "due_date")
```

- El traductor PostgreSQL de fizz deja el tipo `date` tal cual: `colType` usa `default: return c.ColType`. Con `{"null": true}` la columna admite `NULL`, y las filas existentes quedan con `NULL`.
- `migrations/embed.go` ya incluye `*.fizz`, así que no hace falta tocarlo. `cmd/app/main.go` aplica las migraciones al arrancar (`models.Migrate`), y CI ejecuta `soda migrate -e test`.
- La migración de semillas (`20260901000001_seed_tasks.up.sql`) enumera sus columnas, por lo que no le afecta la columna nueva.

### Modelo (`models/task.go`)

Hay que añadir el campo **entre `Completed` y `CreatedAt`**:

```go
DueDate   *time.Time `json:"due_date" db:"due_date" form:"-"`
```

Hay que añadir las constantes y funciones siguientes. Los comentarios van en español, como en el resto del archivo:

```go
// DueDateFormat es el formato AAAA-MM-DD del campo de fecha del formulario y del atributo datetime.
const DueDateFormat = "2006-01-02"

// DueDateDisplayFormat es el formato DD/MM/AAAA con el que se muestra la fecha límite.
const DueDateDisplayFormat = "02/01/2006"

// ParseDueDate convierte un día AAAA-MM-DD en fecha límite. Una cadena vacía significa sin fecha límite.
func ParseDueDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	day, err := time.Parse(DueDateFormat, value)
	if err != nil {
		return nil, err
	}
	return &day, nil
}

// HasDueDate indica si la tarea tiene fecha límite.
func (t Task) HasDueDate() bool {
	return t.DueDate != nil
}

// DueDateValue devuelve la fecha límite como AAAA-MM-DD, o una cadena vacía si no tiene.
func (t Task) DueDateValue() string {
	if t.DueDate == nil {
		return ""
	}
	return t.DueDate.Format(DueDateFormat)
}

// DueDateLabel devuelve la fecha límite como DD/MM/AAAA, o una cadena vacía si no tiene.
func (t Task) DueDateLabel() string {
	if t.DueDate == nil {
		return ""
	}
	return t.DueDate.Format(DueDateDisplayFormat)
}
```

Notas de diseño:

- Se usa `*time.Time` y no `nulls.Time` para no añadir `github.com/gobuffalo/nulls` como dependencia directa en `go.mod`. `database/sql` y pgx escriben un puntero `nil` como `NULL`, y al leer un `NULL` dejan el puntero a `nil`.
- `time.Parse` con `DueDateFormat` devuelve la medianoche UTC. pgx codifica un `date` a partir de año, mes y día, y al leerlo devuelve la medianoche UTC de ese día. Por eso `Format` produce siempre el mismo día, sin desfases de zona horaria.
- Los métodos de presentación usan **receptor por valor** porque `for (t) in tasks` itera sobre valores `Task`. Así se pueden llamar desde Plush tanto sobre `Task` como sobre `*Task`. No hay que comprobar en Plush `if (t.DueDate)`: un puntero `nil` con tipo no es `nil` dentro de una interfaz, y Plush podría evaluarlo como verdadero.
- `Validate` no cambia. Cualquier día que se pueda convertir con `ParseDueDate` es válido, incluidas las fechas pasadas.
- El campo va antes de `CreatedAt` y `UpdatedAt` por una razón concreta. El helper `httptest` de los tests existentes (`as.HTML(...).Post(&models.Task{...})`) agrupa todos los campos `form:"-"` bajo la clave `"-"`, y se queda con el último. Con este orden el último sigue siendo `UpdatedAt` y el comportamiento de esos tests no cambia.

### Acciones (`actions/tasks.go`)

`TasksCreate`, después de `task.Title = strings.TrimSpace(task.Title)` y antes de `tx.ValidateAndCreate(task)`:

```go
dueDate, err := models.ParseDueDate(c.Param("due_date"))
if err != nil {
	return c.Error(http.StatusUnprocessableEntity, err)
}
task.DueDate = dueDate
```

`c.Param` ya incluye los valores del formulario: Buffalo hace `req.ParseForm()` y los combina en `params`. `c.Bind(task)` no toca `DueDate` porque el campo lleva `form:"-"`. El bloque 422 existente para errores de título no cambia. Como `task.DueDate` ya está asignado cuando se vuelve a renderizar la plantilla, el formulario conserva la fecha.

`TasksUpdate`:

- Hay que actualizar el comentario de la función: `// TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.`
- Hay que ampliar la lista blanca:

  ```go
  input := struct {
  	Title   string `form:"title" json:"title"`
  	DueDate string `form:"due_date" json:"due_date"`
  }{}
  ```

- Después de `task.Title = strings.TrimSpace(input.Title)` y antes de `tx.ValidateAndUpdate(task)`:

  ```go
  dueDate, err := models.ParseDueDate(input.DueDate)
  if err != nil {
  	return c.Error(http.StatusUnprocessableEntity, err)
  }
  task.DueDate = dueDate
  ```

- La semántica es que el formulario de edición envía el estado completo de los campos editables. Un `due_date` vacío o ausente elimina la fecha.

`TasksToggle` y `TasksDestroy` no cambian. `TasksToggle` carga la tarea con `tx.Find` y la guarda completa con `tx.Update`, así que conserva `due_date`. Las rutas de `actions/app.go` tampoco cambian.

### Renderizado (Plush)

`templates/tasks/index.plush.html`, dentro del `<li class="task …">` de la lista: hay que sustituir

```html
<span class="task-title"><%= t.Title %></span>
```

por

```html
<div class="task-main">
  <span class="task-title"><%= t.Title %></span>
  <%= if (t.HasDueDate()) { %>
    <time class="task-due" datetime="<%= t.DueDateValue() %>">Fecha límite: <%= t.DueDateLabel() %></time>
  <% } %>
</div>
```

El `<span class="task-status">` y el `<div class="task-actions">` siguen siendo hijos directos del `<li>`, igual que ahora.

En el formulario de creación (`.new-task-row`), entre el `<input id="task-title" …>` y el botón `Añadir`:

```html
<label for="task-due-date" class="sr-only">Fecha límite (opcional)</label>
<input
  id="task-due-date"
  type="date"
  name="due_date"
  value="<%= task.DueDateValue() %>"
  title="Fecha límite (opcional)"
>
```

`templates/tasks/edit.plush.html`: después del bloque `<%= if (errors.HasAny()) { %> … title-error … <% } %>` y antes de `<div class="edit-task-actions">`:

```html
<label for="task-due-date" class="edit-task-due-label">Fecha límite</label>
<input
  id="task-due-date"
  type="date"
  name="due_date"
  value="<%= task.DueDateValue() %>"
  aria-describedby="due-date-help"
>
<p id="due-date-help" class="field-hint">Opcional. Solo el día, sin hora. Déjala vacía para quitarla.</p>
```

El texto de ayuda del título (`Hasta 200 caracteres. Los demás datos de la tarea no cambian.`) se queda como está.

`templates/application.plush.html`: hay que cambiar `/assets/app.css?v=3` por `/assets/app.css?v=4`. `app.js` mantiene `v=2`.

### CSS (`public/assets/app.css`)

1. Hay que mover el posicionamiento de `.task-title` al nuevo contenedor. Hay que sustituir la regla actual

   ```css
   .task-title {
     align-self: center;
     min-width: 0;
     padding: 0.65rem 0;
     overflow-wrap: anywhere;
     line-height: 1.55;
   }
   ```

   por

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
     display: block;
     margin-top: 0.15rem;
     color: var(--color-muted);
     font-size: 0.8rem;
   }
   ```

   El `.task` es una rejilla de tres columnas: toggle, título y acciones. `.task-main` ocupa la columna central. `.task-status` usa `position: absolute` y no ocupa celda.

2. Hay que extender el selector de campos: `input[type="text"] {` pasa a ser `input[type="text"],\ninput[type="date"] {`, con las mismas declaraciones.

3. Después de `.new-task-row input { flex: 1; }` hay que añadir:

   ```css
   .new-task-row input[type="date"] {
     flex: 0 0 auto;
     width: auto;
   }
   ```

   Su especificidad (0,2,1) supera a `.new-task-row input` (0,1,1), también en el `@media (max-width: 380px)`. Ahí la fila hace `wrap`, y el campo de fecha baja a la segunda línea junto al botón.

4. Después de la regla `.edit-task label { … }` hay que añadir:

   ```css
   .edit-task-due-label {
     margin-top: 1.25rem;
   }
   ```

Las reglas `.task-completed .task-title` y `@media (max-width: 760px) .task-title { font-size: 0.9rem; }` siguen siendo válidas y no cambian.

### JavaScript

No hay cambios. `public/assets/app.js` usa `task.querySelector(".task-title")`, que busca entre los descendientes y sigue encontrando el título dentro de `.task-main`. También usa `task.insertBefore(status, task.querySelector(".task-actions"))`, y `.task-actions` sigue siendo hijo directo del `<li>`.

## Archivos relevantes

| Ruta | Responsabilidad | Cambio |
|---|---|---|
| `migrations/20261027120000_add_due_date_to_tasks.up.fizz` | Añade `tasks.due_date` (`date`, admite `NULL`) | Nuevo |
| `migrations/20261027120000_add_due_date_to_tasks.down.fizz` | Revierte la columna | Nuevo |
| `models/task.go` | Modelo `Task`: campo, conversión y presentación de la fecha | Modificar |
| `models/task_test.go` | Tests de modelo (`ModelSuite`) | Ampliar |
| `actions/tasks.go` | `TasksCreate` y `TasksUpdate` leen y validan `due_date` | Modificar |
| `actions/tasks_test.go` | Tests HTTP (`ActionSuite`) | Ampliar |
| `templates/tasks/index.plush.html` | Lista (muestra la fecha) y formulario de creación (campo de fecha) | Modificar |
| `templates/tasks/edit.plush.html` | Formulario de edición (campo de fecha) | Modificar |
| `templates/application.plush.html` | Layout, con la versión de `app.css` | `v=3` → `v=4` |
| `public/assets/app.css` | Estilos de la lista y los formularios | Modificar |
| `actions/app.go` | Rutas | Sin cambios (referencia) |
| `public/assets/app.js` | Toggle optimista y confirmación de borrado | Sin cambios (comprobar compatibilidad) |
| `migrations/embed.go` | Incluye `*.fizz *.sql` en el binario | Sin cambios |
| `fixtures/tasks.toml` | Escenario `tres tareas` (sin fechas) | Sin cambios |

## Implementación paso a paso

1. **Migración.** Crear los dos archivos fizz con el contenido exacto de la sección *Persistencia y migración*.
2. **Modelo.** En `models/task.go`, añadir el campo `DueDate` entre `Completed` y `CreatedAt`, las dos constantes, `ParseDueDate` y los tres métodos de presentación tal como figuran arriba. `strings` y `time` ya están importados.
3. **Acciones.** En `actions/tasks.go`, aplicar los cambios de `TasksCreate` y `TasksUpdate` descritos en *Acciones*: la conversión, el `c.Error(http.StatusUnprocessableEntity, err)`, la lista blanca ampliada y el comentario actualizado. No hace falta ningún import nuevo.
4. **Plantillas.** Aplicar los cambios de `index.plush.html`, `edit.plush.html` y `application.plush.html` descritos en *Renderizado*.
5. **CSS.** Aplicar los cuatro cambios de `public/assets/app.css` descritos en *CSS*.
6. **Tests de modelo.** Añadir a `models/task_test.go`:
   - `Test_ParseDueDate`: `""` y `"   "` devuelven `nil, nil`. `"2026-11-05"` devuelve una fecha con `Format(DueDateFormat) == "2026-11-05"`. `"05/11/2026"`, `"2026-02-30"` y `"2026-11-05T10:00"` devuelven error.
   - `Test_Task_DueDate_OptionalByDefault`: crear `&Task{Title: "Sin fecha"}` con `ValidateAndCreate`, recargarla con `Find` y comprobar que `DueDate` es `nil`, que `HasDueDate()` es `false` y que `DueDateValue()` y `DueDateLabel()` son `""`.
   - `Test_Task_DueDate_PersistsDay`: obtener la fecha con `ParseDueDate("2026-11-05")`, crear la tarea con ella, recargarla en un `&Task{}` nuevo con `Find` y comprobar que `HasDueDate()` es `true`, que `DueDateValue() == "2026-11-05"` y que `DueDateLabel() == "05/11/2026"`.
7. **Tests de acciones.** Añadir a `actions/tasks_test.go`. Usar `url.Values` para los cuerpos con `due_date` y comparar las fechas con `DueDateValue()`, no con igualdad de `time.Time`. Si hace falta, añadir un helper local `taskWithDueDate(day string, completed bool) *models.Task` siguiendo el estilo de `taskForEditing`: `ParseDueDate`, `ValidateAndCreate` y `Reload`.
   - `Test_TasksCreate_WithDueDate`: `Post(url.Values{"title": {"Con fecha"}, "due_date": {"2026-11-05"}})` devuelve 303. `as.DB.First(task)` tiene `DueDateValue() == "2026-11-05"`.
   - `Test_TasksCreate_EmptyDueDate`: `due_date` vacío devuelve 303 y la tarea queda con `DueDate == nil`.
   - `Test_TasksCreate_InvalidDueDate`: `due_date=05/11/2026` devuelve 422 y `Count == 0`.
   - `Test_TasksCreate_InvalidTitle_KeepsDueDate`: `title=""` y `due_date=2026-11-05` devuelven 422. El cuerpo contiene `El título es obligatorio.` y `value="2026-11-05"`, y `Count == 0`.
   - `Test_TasksIndex_ShowsDueDate`: crear una tarea con fecha `2026-11-05` y otra sin fecha. `GET /` devuelve 200 y el cuerpo contiene `datetime="2026-11-05"` y `Fecha límite: 05/11/2026`.
   - En `Test_TasksIndex_ListsTasks`, cuyas fixtures no tienen fecha, añadir `as.NotContains(body, "class=\"task-due\"")`.
   - `Test_TasksEdit_PrefillsDueDate`: para una tarea con fecha, `GET /tasks/{id}/edit` contiene `name="due_date"` y `value="2026-11-05"`.
   - `Test_TasksUpdate_ChangesDueDate`: para una tarea completada con fecha `2026-11-05`, `PUT` con `title=Nuevo título` y `due_date=2026-12-01` devuelve 303. Tras `Reload`, `DueDateValue() == "2026-12-01"`, el título está actualizado y `Completed` sigue siendo `true`.
   - `Test_TasksUpdate_ClearsDueDate`: para una tarea con fecha, `PUT` con `title` y `due_date=""` devuelve 303. Tras `Reload`, `DueDate == nil`.
   - `Test_TasksUpdate_InvalidDueDate`: para una tarea con fecha `2026-11-05` y título `Título original`, `PUT` con `title=Otro` y `due_date=2026-13-40` devuelve 422. Tras `Reload`, el título sigue siendo `Título original` y la fecha `2026-11-05`.
   - `Test_TasksToggle_KeepsDueDate`: para una tarea pendiente con fecha, el toggle devuelve 303. Tras `Reload`, `Completed` es `true` y `DueDateValue() == "2026-11-05"`.
8. **Formato.** Ejecutar `gofmt -w` sobre los `.go` modificados.
9. Ejecutar las validaciones de la sección siguiente.

## Tests y validación

Requisitos previos:

- PostgreSQL de `compose.yaml` levantado y sano: `docker compose up -d db` y `docker compose ps`. Hoy ya está levantado.
- `soda` no está en el `PATH`. El binario está en `$(go env GOPATH)/bin/soda`.
- Los tests deben ejecutarse con `GO_ENV=test`. Si no, `models.DB` (`models/models.go`) se conecta a `todo_development` mientras las suites usan `todo_test`.
- **Estado local de `todo_test`.** La base de test local es compartida y ahora tiene aplicada una migración de otra rama: la versión `20261027100000`, que ya añadió `tasks.due_date`. Con ese estado, `soda migrate -e test` fallaría al intentar añadir de nuevo la columna. Esa base es desechable, porque las suites truncan las tablas en cada test. Hay que recrearla para que refleje solo las migraciones de esta rama, como en CI:

  ```sh
  SODA=$(go env GOPATH)/bin/soda
  $SODA drop -e test
  $SODA create -e test
  $SODA migrate -e test
  ```

  Si `drop` falla por conexiones activas de otro proceso, no hay que forzarlo: se detiene la validación y se informa.

Comandos de validación, desde la raíz del repositorio:

```sh
gofmt -l .                        # debe salir vacío
go vet ./...
GO_ENV=test go test -p 1 ./...    # modelos y acciones contra todo_test
go build ./...
```

Comprobación adicional de la migración, opcional pero recomendable: tras `soda migrate -e test`, `docker compose exec -T db psql -U postgres -d todo_test -c '\d tasks'` debe mostrar `due_date | date` sin `not null`. `$SODA migrate down -e test` seguido de `$SODA migrate -e test` debe funcionar sin errores.

## Riesgos y recuperación

- **Base de test compartida.** Otras ramas del mismo issue usan la misma `todo_test` con migraciones distintas. Tras validar esta rama, la base queda con la versión `20261027120000`, y otra rama tendrá que recrearla igual. Esto solo afecta al entorno local; CI usa una base nueva en cada ejecución.
- **Migración en la base de desarrollo.** Al arrancar, la aplicación ejecuta `models.Migrate()` sobre `todo_development`, que hoy tiene una migración de otra rama (`priority`). La columna nueva no entra en conflicto con ella. No hace falta arrancar la aplicación para validar este plan.
- **Binding de formularios.** `due_date` no debe enlazarse automáticamente sobre `models.Task`, porque el decodificador de tiempo de Buffalo aceptaría otros formatos. Por eso el campo lleva `form:"-"` y la conversión es explícita con `ParseDueDate`. Si un test de creación recibe `due_date` y la tarea se guarda sin fecha, hay que revisar que `TasksCreate` usa `c.Param("due_date")`.
- **Desfase de zona horaria.** La fecha se formatea en la ubicación del `time.Time` (UTC, tanto al convertir como al leer de pgx). No se debe usar `time.Local` ni `.Local()` al presentar la fecha. Si un test muestra el día anterior, revisa que no se haya introducido una conversión de zona horaria.
- **Truthiness en Plush.** No se debe usar `if (t.DueDate)` en las plantillas. Un puntero `nil` con tipo puede evaluarse como verdadero. Hay que usar `t.HasDueDate()`.
- **CSS sin tests automáticos.** La disposición visual (rejilla de `.task`, fila del formulario en móvil) no la cubren los tests. El cambio de versión a `v=4` evita que el navegador sirva un CSS antiguo desde caché.
- **JS de toggle en `origin/main`.** `app.js` en la base de esta rama tiene un defecto conocido: el toggle hace un `fetch` GET, y se corrige en el commit local `429e9d6`, que no está en `origin/main`. Al probar manualmente puede parecer que completar una tarea no persiste. Es ajeno a este plan y no hay que corregirlo aquí.
- **Recuperación.** Todo el cambio vive en esta rama. Para revertir la base de test basta con `$SODA migrate down -e test` (elimina la columna) o con recrear la base. Para revertir el código, se descarta la rama o se revierte el commit.
