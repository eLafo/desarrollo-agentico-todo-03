# Feature: fecha límite (solo día, sin hora) en las tareas

Issue: [#5 «Quiero añadir fecha límite a las tasks»](https://github.com/eLafo/desarrollo-agentico-todo-03/issues/5). Cuerpo: «Vale con que sea simplemente un día. Sin hora».

## Encargo y criterios de aceptación

### Requisitos explícitos

- Una tarea puede tener una fecha límite.
- La fecha límite es un día del calendario, sin hora.

### Inferencias (decididas en este plan)

- La fecha límite es **opcional**. Las tareas existentes no tienen ninguna y el alta rápida de hoy (solo título) debe seguir funcionando igual.
- Se puede indicar al crear la tarea y se puede añadir, cambiar o quitar al editarla.
- La lista muestra la fecha de las tareas que la tienen, en formato `DD/MM/AAAA` (la interfaz está en español).
- Se admiten días pasados. No se exige que la fecha sea futura: se puede registrar una tarea ya atrasada, y editar el título de una tarea vencida no debe fallar.
- Un valor que no sea un día válido con formato `AAAA-MM-DD` se rechaza con un error de validación, no con un error 500.

### Criterios de aceptación

1. El formulario «Añadir tarea» tiene un campo de fecha (`<input type="date" name="due_date">`) opcional. Si se deja vacío, la tarea se crea sin fecha límite, como hoy.
2. Si se rellena con un día válido, la tarea se guarda con ese día. En la lista aparece `Fecha límite: 15/11/2026` (con `<time datetime="2026-11-15">`). Las tareas sin fecha no muestran nada.
3. La página de edición muestra el campo de fecha relleno con el valor actual. Permite cambiarlo o vaciarlo para quitar la fecha. Al guardar, el estado completada/pendiente no cambia.
4. Solo se guarda el día (columna `date` de PostgreSQL). Al recargar se ve el mismo día, sin desplazamientos por zona horaria.
5. Un valor no válido (`2026-02-30`, `15/11/2026`, `2026-11-15T10:30`, `mañana`) responde `422` con el mensaje `La fecha límite no es válida. Usa el formato AAAA-MM-DD.` asociado al campo (`aria-describedby="due-date-error"`). En ese caso no se crea ni se modifica la tarea.
6. Si el título no es válido pero la fecha sí, el formulario devuelto (`422`) conserva el día introducido.
7. Después de la migración, las tareas existentes no tienen fecha límite. Completar o reabrir una tarea conserva su fecha.

## Comportamiento actual y propuesta

**Actual.**

- `tasks` tiene `id`, `title`, `completed`, `created_at` y `updated_at` (`migrations/20260901000000_create_tasks.up.fizz`).
- `models.Task` (`models/task.go`) solo valida el título.
- `TasksCreate` (`actions/tasks.go`) enlaza el formulario directamente sobre `models.Task`, que solo acepta `title` (`form:"title"`; el resto tiene `form:"-"`).
- `TasksUpdate` enlaza un struct de entrada que solo tiene `Title` y conserva el resto de datos.
- `TasksToggle` invierte `Completed` con `tx.Update` sobre la tarea cargada.
- `templates/tasks/index.plush.html` pinta, por cada tarea, el botón de completar, `<span class="task-title">` y las acciones. Debajo está el formulario de alta: `+`, título y «Añadir».
- `templates/tasks/edit.plush.html` solo edita el título. Su ayuda dice «Los demás datos de la tarea no cambian».
- `TasksIndex` y la ruta de error de `TasksCreate` asignan siempre `task` y `errors`.

**Propuesta.** Cambio vertical mínimo, siguiendo los patrones existentes:

1. Columna `due_date date NULL`.
2. Campo `DueDate *time.Time` en `Task`, con un setter que interpreta el valor del formulario y un error de validación en `Validate`. Se usa `*time.Time` y no `nulls.Time` para no convertir `github.com/gobuffalo/nulls` en dependencia directa. Así `go.mod` y `go.sum` no cambian.
3. `TasksCreate` y `TasksUpdate` leen `due_date` del formulario.
4. Las plantillas añaden el campo de fecha y la etiqueta en la lista. El CSS da estilo a ambos.

Antes de escribir este plan se comprobó el diseño en una copia temporal del repositorio, con una base de datos propia. La columna `date` se guarda y se lee como `2026-11-15 00:00:00 +0000 UTC` en un `*time.Time`, y `nil` se guarda como `NULL`. El campo no exportado con `db:"-"` no rompe pop. Plush acepta `t.DueDateValue() != ""` y `len(errors.Get("title")) > 0`. `c.Param("due_date")` lee el campo del formulario en `POST /tasks`. `go vet ./...` y todos los tests existentes pasan con el cambio.

## Alcance

### Incluido

- Migración SQL que añade `tasks.due_date` (`date`, nullable), con su `down`.
- Modelo: campo, setter, formateadores y validación.
- Acciones `TasksCreate` y `TasksUpdate`: lectura de `due_date`.
- Plantillas de lista, alta y edición: campo de fecha, etiqueta visible y errores por campo.
- CSS de los nuevos elementos y cambio del parámetro de caché de `app.css` (`?v=3` → `?v=4`).
- Tests de modelo y de acciones descritos abajo.

### Excluido

- Hora, zona horaria o fecha y hora combinadas.
- Ordenar, filtrar o agrupar por fecha límite.
- Resaltar tareas vencidas, de hoy o próximas. Recordatorios o notificaciones.
- Hacer obligatoria la fecha o exigir que sea futura.
- Entradas relativas («mañana») o selectores de fecha propios. Se usa el `input type="date"` nativo.
- Cambios en `public/assets/app.js`, en las rutas (`actions/app.go`), en `TasksToggle`, en `fixtures/tasks.toml` o en la migración de seed.
- Cambios en `go.mod` o `go.sum`. No hay que ejecutar `go mod tidy` ni `go get`.
- Eliminar las reglas de `@media (max-width: 380px)` que este cambio vuelve redundantes.
- Resolver o cerrar los otros pull requests abiertos sobre la issue #5.

## Diseño por capas

### Persistencia y migración

Archivos `migrations/20261026211500_add_due_date_to_tasks.up.sql` y `.down.sql`, en SQL de PostgreSQL. El repositorio ya usa migraciones `.sql` (seed), `migrations/embed.go` incluye `*.sql` y todos los entornos de `database.yml` son PostgreSQL.

- `up`: `ALTER TABLE tasks ADD COLUMN IF NOT EXISTS due_date date;`
- `down`: `ALTER TABLE tasks DROP COLUMN IF EXISTS due_date;`

Se usa SQL y no fizz porque `add_column` de fizz no admite `IF NOT EXISTS`. Hoy hay 19 pull requests abiertos para esta misma issue que añaden `tasks.due_date` con otras versiones de migración. La base local `todo_test` ya tiene esa columna (`date`, nullable), creada por la migración ajena `20261026230000`. Con `add_column` de fizz, `soda migrate -e test` fallaría con «column already exists». Con `IF NOT EXISTS`, la migración funciona tanto en una base limpia (CI) como en una que ya tiene la columna, y no hace falta borrar ni recrear bases de datos.

### Modelo (`models/task.go`)

```go
// DueDateLayout es el formato del día límite en los campos de fecha y en el atributo datetime (AAAA-MM-DD).
const DueDateLayout = "2006-01-02"

// DueDateLabelLayout es el formato con el que se muestra el día límite en la lista (DD/MM/AAAA).
const DueDateLabelLayout = "02/01/2006"

// Task representa una tarea de la lista.
type Task struct {
	ID        uuid.UUID  `json:"id" db:"id" form:"-"`
	Title     string     `json:"title" db:"title" form:"title"`
	Completed bool       `json:"completed" db:"completed" form:"-"`
	DueDate   *time.Time `json:"due_date" db:"due_date" form:"-"`
	CreatedAt time.Time  `json:"created_at" db:"created_at" form:"-"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at" form:"-"`

	// dueDateInvalid indica que SetDueDate recibió un valor que no es un día válido.
	dueDateInvalid bool `db:"-"`
}

// SetDueDate asigna la fecha límite a partir del valor de un campo de fecha (AAAA-MM-DD).
// Un valor vacío deja la tarea sin fecha límite. Un valor que no es un día válido
// también la deja sin fecha y hace que Validate informe del error.
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

// DueDateValue devuelve la fecha límite en formato AAAA-MM-DD, o "" si la tarea no tiene.
func (t Task) DueDateValue() string {
	if t.DueDate == nil {
		return ""
	}
	return t.DueDate.Format(DueDateLayout)
}

// DueDateLabel devuelve la fecha límite en formato DD/MM/AAAA, o "" si la tarea no tiene.
func (t Task) DueDateLabel() string {
	if t.DueDate == nil {
		return ""
	}
	return t.DueDate.Format(DueDateLabelLayout)
}
```

En `Validate`, después de las comprobaciones del título:

```go
	if t.dueDateInvalid {
		verrs.Add("due_date", "La fecha límite no es válida. Usa el formato AAAA-MM-DD.")
	}
```

Notas:

- `DueDateValue` y `DueDateLabel` tienen receptor por valor a propósito. En `for (t) in tasks`, Plush recibe un `Task` no direccionable y no podría llamar a métodos con receptor puntero. En `edit`, `task` es `*Task` y también puede llamarlos.
- El campo no exportado necesita `db:"-"`. Sin esa etiqueta, pop usaría el nombre del campo como columna.
- `time.Parse` con `DueDateLayout` exige exactamente `AAAA-MM-DD`, rechaza días inexistentes (`2026-02-30`) y produce medianoche UTC. Al leer, pgx devuelve la columna `date` como medianoche UTC. Por eso `Format` devuelve el mismo día.

### Acciones (`actions/tasks.go`)

- `TasksCreate`: justo después de `task.Title = strings.TrimSpace(task.Title)`, añade `task.SetDueDate(c.Param("due_date"))`. `c.Param` incluye los valores del formulario (`buffalo@v1.1.4/context.go`). El resto del flujo (`ValidateAndCreate`, render `422` con `task` y `errors`, redirección `303`) no cambia.
- `TasksUpdate`: cambia su comentario a `// TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.`. Añade `DueDate string `form:"due_date" json:"due_date"`` al struct de entrada y `task.SetDueDate(input.DueDate)` después de asignar el título. Si el formulario envía `due_date` vacío o no lo envía, la tarea queda sin fecha límite. El formulario de edición siempre envía el campo.
- Las rutas no cambian.

### Renderizado (Plush)

`templates/tasks/index.plush.html`:

1. En cada `<li class="task">`, sustituye `<span class="task-title"><%= t.Title %></span>` por:

   ```html
          <div class="task-body">
            <span class="task-title"><%= t.Title %></span>
            <%= if (t.DueDateValue() != "") { %>
              <span class="task-due">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></span>
            <% } %>
          </div>
   ```

   El contenedor es necesario porque `.task` es un grid de tres columnas (botón, título y acciones). Un elemento visible más pasaría a otra fila. `public/assets/app.js` sigue funcionando sin cambios: busca `.task-title` con `querySelector` dentro de la tarea, y `.task-actions` sigue siendo hijo directo de `<li>` para `insertBefore`.

2. En el formulario de alta:

   - El atributo condicional del título pasa a ser `<%= if (len(errors.Get("title")) > 0) { %>autofocus aria-invalid="true" aria-describedby="title-error"<% } %>`.
   - Entre el input del título y el botón «Añadir», añade:

     ```html
    <label for="task-due-date" class="sr-only">Fecha límite (opcional)</label>
    <input
      id="task-due-date"
      type="date"
      name="due_date"
      value="<%= task.DueDateValue() %>"
      title="Fecha límite (opcional)"
      <%= if (len(errors.Get("due_date")) > 0) { %><%= if (len(errors.Get("title")) == 0) { %>autofocus <% } %>aria-invalid="true" aria-describedby="due-date-error"<% } %>
    >
     ```

   - Sustituye el bloque de error actual (`<%= if (errors.HasAny()) { %> <p id="title-error" …> … <% } %>`) por dos bloques independientes:

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

`templates/tasks/edit.plush.html`: agrupa cada campo en un `<div class="form-field">`.

- Campo título: mismo `label` e `input`. La condición de su `aria-*` pasa a ser `len(errors.Get("title")) > 0` en lugar de `errors.HasAny()`. Mantiene `autofocus` y la rama `else` con `aria-describedby="title-help"`. La ayuda `title-help` pasa a decir `Hasta 200 caracteres.`, porque la fecha ahora también es editable. El bloque `title-error` queda condicionado a `len(errors.Get("title")) > 0`.
- Nuevo campo fecha, después del título y antes de `.edit-task-actions`:

  ```html
  <div class="form-field">
    <label for="task-due-date">Fecha límite <span class="label-optional">(opcional)</span></label>
    <input
      id="task-due-date"
      type="date"
      name="due_date"
      value="<%= task.DueDateValue() %>"
      <%= if (len(errors.Get("due_date")) > 0) { %>aria-invalid="true" aria-describedby="due-date-error"<% } else { %>aria-describedby="due-date-help"<% } %>
    >
    <p id="due-date-help" class="field-hint">Solo el día, sin hora. Déjala vacía si la tarea no tiene fecha límite.</p>
    <%= if (len(errors.Get("due_date")) > 0) { %>
      <p id="due-date-error" class="field-error" role="alert">
        <%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %>
      </p>
    <% } %>
  </div>
  ```

`templates/application.plush.html`: `/assets/app.css?v=3` → `/assets/app.css?v=4`, siguiendo la convención del repositorio para invalidar la caché. `app.js` no cambia y su `?v=` tampoco.

### CSS (`public/assets/app.css`)

1. Sustituye la regla `.task-title { align-self: center; min-width: 0; padding: 0.65rem 0; overflow-wrap: anywhere; line-height: 1.55; }` por:

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

2. Cambia el selector `input[type="text"] {` por `input[type="text"],\ninput[type="date"] {` (misma declaración). Justo después de esa regla, añade:

   ```css
   input[type="date"] {
     width: auto;
   }
   ```

3. Después de `.new-task-row input { flex: 1; }`, añade:

   ```css
   .new-task-row input[type="date"] {
     flex: 0 0 auto;
   }
   ```

4. Después de la regla `.edit-task label { … }`, añade:

   ```css
   .form-field + .form-field {
     margin-top: 1.5rem;
   }

   .label-optional {
     color: var(--color-muted);
     font-weight: 400;
   }
   ```

5. Dentro de `@media (max-width: 760px)`, el bloque existente `.new-task-row { gap: 0.4rem; }` pasa a incluir `flex-wrap: wrap;`. Justo después, añade:

   ```css
     .new-task-row input[type="text"] {
       flex-basis: calc(100% - 36px - 0.4rem);
     }

     .new-task-row input[type="date"] {
       margin-left: calc(32px + 0.4rem);
     }

     .new-task-row .button {
       margin-left: auto;
     }
   ```

   Hasta 760 px, el título ocupa la primera línea. Debajo van la fecha, alineada con el título, y el botón a la derecha. El bloque de 380 px queda como está. Sus reglas sobre `.new-task-row input` pierden frente a los selectores `[type=…]`, que son más específicos.

## Archivos relevantes

| Ruta | Responsabilidad |
| --- | --- |
| `migrations/20261026211500_add_due_date_to_tasks.up.sql` (nuevo) | Añade `tasks.due_date date NULL` si no existe. |
| `migrations/20261026211500_add_due_date_to_tasks.down.sql` (nuevo) | Elimina `tasks.due_date` si existe. |
| `migrations/embed.go` | Embebe `*.fizz *.sql`. Sin cambios, pero confirma que la migración `.sql` se aplica al arrancar (`models.Migrate`). |
| `models/task.go` | Campo `DueDate`, `SetDueDate`, `DueDateValue`, `DueDateLabel` y validación `due_date`. |
| `models/task_test.go` | Tests de persistencia, vaciado y validación de la fecha. |
| `actions/tasks.go` | `TasksCreate` y `TasksUpdate` leen `due_date`. |
| `actions/tasks_test.go` | Tests de alta, edición, lista y toggle con fecha. |
| `templates/tasks/index.plush.html` | Etiqueta de fecha en la lista, campo de fecha en el alta y errores por campo. |
| `templates/tasks/edit.plush.html` | Campo de fecha en la edición y errores por campo. |
| `templates/application.plush.html` | `app.css?v=4`. |
| `public/assets/app.css` | Estilos de `.task-body`, `.task-due`, `input[type="date"]`, `.form-field` y `.label-optional`, y ajuste responsive del alta. |
| `public/assets/app.js` | Sin cambios. Se ha comprobado que sigue funcionando con el contenedor `.task-body`. |

## Implementación paso a paso

1. Comprueba `git status --short --branch`. Debe estar limpio, en la rama del encargo.
2. Crea las dos migraciones SQL con el contenido exacto de «Persistencia y migración».
3. Edita `models/task.go`: constantes, campo `DueDate`, campo `dueDateInvalid`, `SetDueDate`, `DueDateValue`, `DueDateLabel` y la comprobación en `Validate`. Mantén el campo `DueDate` después de `Completed`.
4. Edita `actions/tasks.go`: `SetDueDate` en `TasksCreate` y en `TasksUpdate`, el campo `DueDate` del struct de entrada y el comentario de `TasksUpdate`.
5. Edita `templates/tasks/index.plush.html` y `templates/tasks/edit.plush.html` según «Renderizado».
6. Edita `public/assets/app.css` según «CSS» y cambia `app.css?v=3` por `?v=4` en `templates/application.plush.html`.
7. Añade los tests de «Tests y validación».
8. Ejecuta las validaciones en el orden indicado.
9. Revisa `git diff --check`, `git diff` y `git status --short`. Solo deben aparecer los archivos de la tabla anterior (y este plan). `go.mod` y `go.sum` deben estar intactos.

## Tests y validación

### Tests de modelo (`models/task_test.go`)

Añade, usando las convenciones de la suite (`ms.DB.ValidateAndCreate`, `ms.DB.Find`):

- `Test_Task_DueDate_OptionalByDefault`: crea una tarea sin fecha. Al recargarla, `DueDate` es `nil` y `DueDateValue()` y `DueDateLabel()` devuelven `""`.
- `Test_Task_SetDueDate_PersistsDayOnly`: hace `SetDueDate(" 2026-11-15 ")` y la crea. Al recargarla, `ms.Require().NotNil(reloaded.DueDate)`, `DueDateValue() == "2026-11-15"`, `DueDateLabel() == "15/11/2026"`, y `Hour()` y `Minute()` son cero.
- `Test_Task_SetDueDate_PastDayAllowed`: `SetDueDate("2020-01-31")` es válida y se crea.
- `Test_Task_SetDueDate_EmptyClears`: crea la tarea con `2026-11-15`, hace `SetDueDate("   ")` y `ValidateAndUpdate` sin errores. Al recargarla, `DueDate` es `nil`.
- `Test_Task_SetDueDate_Invalid`: para `"2026-02-30"`, `"15/11/2026"`, `"2026-11-15T10:30"` y `"mañana"`, `ValidateAndCreate` devuelve errores, `verrs.Get("due_date")` contiene `La fecha límite no es válida. Usa el formato AAAA-MM-DD.` y `task.DueDate` es `nil`. Al final, `Count` es `0`.
- `Test_Task_SetDueDate_ValidAfterInvalid`: `SetDueDate("2026-13-01")` y después `SetDueDate("2026-12-01")`. La tarea es válida, porque el setter reinicia el indicador de error, y `DueDateValue() == "2026-12-01"`.

### Tests de acciones (`actions/tasks_test.go`)

Añade un helper que siga el patrón de `taskForEditing`:

```go
func (as *ActionSuite) taskWithDueDate(dueDate string) *models.Task {
	task := &models.Task{Title: "Tarea con fecha"}
	task.SetDueDate(dueDate)
	verrs, err := as.DB.ValidateAndCreate(task)
	as.NoError(err)
	as.False(verrs.HasAny())
	as.NoError(as.DB.Reload(task))
	return task
}
```

Tests (con `url.Values` en los formularios, como los tests existentes):

- `Test_TasksIndex_ShowsDueDate`: `taskWithDueDate("2026-11-15")` y una tarea sin fecha creada con `ValidateAndCreate`. `GET /` → `200`, el cuerpo contiene `<time datetime="2026-11-15">15/11/2026</time>` y `strings.Count(body, `class="task-due"`) == 1`.
- `Test_TasksIndex_FormHasDueDateField`: `GET /` contiene `type="date"`, `name="due_date"` y `Fecha límite (opcional)`.
- `Test_TasksCreate_WithDueDate`: `POST /tasks` con `title=Tarea con fecha` y `due_date=2026-11-15` → `303`. La tarea creada tiene `DueDateValue() == "2026-11-15"`.
- `Test_TasksCreate_EmptyDueDate`: `due_date=""` → `303`. La tarea creada tiene `DueDate == nil`.
- `Test_TasksCreate_InvalidDueDate`: `title=Tarea` y `due_date=2026-02-30` → `422`. El cuerpo contiene el mensaje de fecha no válida, `aria-describedby="due-date-error"` y `value="Tarea"`, y no contiene `id="title-error"`. `Count` es `0`.
- `Test_TasksCreate_EmptyTitle_KeepsDueDate`: `title=""` y `due_date=2026-11-15` → `422`. El cuerpo contiene `El título es obligatorio.` y `value="2026-11-15"`, y no contiene `id="due-date-error"`. `Count` es `0`.
- `Test_TasksEdit_PrefillsDueDate`: `GET /tasks/{id}/edit` de `taskWithDueDate("2026-11-15")` → `200`, contiene `name="due_date"` y `value="2026-11-15"`.
- `Test_TasksUpdate_ChangesDueDate`: `PUT` con el mismo título y `due_date=2026-12-01` → `303`. Tras `Reload`, `DueDateValue() == "2026-12-01"` y la tarea sigue pendiente.
- `Test_TasksUpdate_AddsDueDateKeepsCompleted`: `taskForEditing(true)` y `PUT` con `title=Título original` y `due_date=2026-11-15` → `303`. Tras `Reload`, la fecha es `2026-11-15` y `Completed` sigue siendo `true`.
- `Test_TasksUpdate_ClearsDueDate`: `taskWithDueDate("2026-11-15")` y `PUT` con `due_date=""` → `303`. Tras `Reload`, `DueDate == nil`.
- `Test_TasksUpdate_InvalidDueDate`: `taskWithDueDate("2026-11-15")` y `PUT` con `title=Título nuevo` y `due_date=mañana` → `422`. El cuerpo contiene el mensaje y `aria-describedby="due-date-error"`. Tras `Reload`, el título sigue siendo `Tarea con fecha` y la fecha `2026-11-15`.
- `Test_TasksToggle_KeepsDueDate`: `taskWithDueDate("2026-11-15")` y `PUT /tasks/{id}/toggle` → `303`. Tras `Reload`, `Completed` es `true` y la fecha sigue siendo `2026-11-15`.

Los tests existentes deben seguir pasando sin modificarlos. Entre ellos están `Test_TasksUpdate_EmptyTitle`, que comprueba `aria-describedby="title-error"`, y `Test_TasksEdit_PrefillsTitle`.

### Condiciones para ejecutar los tests

- PostgreSQL en `127.0.0.1:5432` (`postgres`/`postgres`), por ejemplo el servicio `db` de `compose.yaml`. Debe existir la base `todo_test`.
- **`GO_ENV=test` es obligatorio** al ejecutar los tests. `models/models.go` conecta en `init` con `GO_ENV`, que por defecto es `development`. Sin esa variable, los tests de acciones usarían `todo_development`.
- `soda` puede no estar en el `PATH`. Si no lo está, usa `~/go/bin/soda`. Si no existe, instálalo con `go install github.com/gobuffalo/pop/v6/soda@v6.4.1`, como en CI.
- No borres, recrees ni resetees ninguna base de datos (`soda drop`, `soda reset`, `DROP`). Tampoco arranques la aplicación contra `todo_development`, que tiene cambios de esquema ajenos (columna `priority`). La migración con `IF NOT EXISTS` funciona sobre el estado actual de `todo_test`.

### Comandos de validación (desde la raíz del repositorio)

1. `gofmt -l .`: sin salida.
2. `go vet ./...`
3. `soda migrate -e test`: aplica `20261026211500`. En la `todo_test` local, la columna ya existe y la sentencia no hace nada. Comprueba después que la columna existe (`docker exec desarrollo-agentico-todo-03-db-1 psql -U postgres -d todo_test -c '\d tasks'` muestra `due_date | date`). El aviso de que no se encuentra `pg_dump` es inocuo.
4. `GO_ENV=test go test -p 1 ./...`
5. `go build ./...`
6. `git diff --exit-code go.mod go.sum`: sin diferencias.

No hay herramientas de test de JavaScript ni de navegador en el repositorio. El renderizado queda cubierto por los tests de acciones. La maquetación visual (CSS responsive) no se valida automáticamente.

## Riesgos y recuperación

- **Columna previa con otro tipo.** `IF NOT EXISTS` no comprueba el tipo. Si un entorno tuviera `due_date` con un tipo distinto de `date` (por ejemplo, por otro de los PR abiertos), la migración no lo corregiría. En `todo_test` ya se comprobó que es `date` nullable. Si los tests fallan por el tipo, detente e informa. No modifiques el esquema a mano.
- **PRs paralelos para la misma issue.** Hay otros 19 PR abiertos que añaden `due_date` con otras versiones de migración. Fusionar más de uno provocará conflictos en `models/task.go` y en las plantillas. La migración idempotente evita el fallo de esquema, pero no el conflicto de código. Esta decisión queda fuera de este encargo.
- **Uso concurrente de `todo_test`.** Si otra ejecución usa `todo_test` a la vez, los tests pueden fallar de forma intermitente porque la suite trunca las tablas. En ese caso, repite `GO_ENV=test go test -p 1 ./...` cuando la otra ejecución termine. No cambies `database.yml`.
- **Toolchain local más nueva que `go.mod`** (Go 1.27.x frente a `go 1.26.3`). Los comandos de validación no modifican `go.mod`. Si aparece algún cambio en `go.mod` o `go.sum`, reviértelo con `git checkout -- go.mod go.sum` y no lo incluyas.
- **Navegadores sin `input type="date"`.** Muestran un campo de texto. Un valor mal escrito recibe el `422` con el mensaje de formato, no un error 500.
- **Recuperación general.** Todos los cambios son aditivos. Revertir el commit y aplicar la migración `down` (`ALTER TABLE tasks DROP COLUMN IF EXISTS due_date;`) devuelve el estado anterior. La columna es nullable, así que el código anterior sigue funcionando aunque la columna exista.
