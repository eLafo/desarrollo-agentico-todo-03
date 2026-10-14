# Bug: marcar una tarea como completada no se guarda y se pierde al recargar

## Encargo y resultado esperado

Encargo original: «cuando marco una tarea como completada y recargo el navegador no se muestra completada».

Resultado esperado:

- Al pulsar el botón de completar o reabrir de una tarea en `/`, la interfaz se actualiza en el momento sin recargar (comportamiento introducido en `5fdd6c3 feat(ui): improve task completion feedback`) **y** el cambio se guarda en la base de datos.
- Al recargar la página, la tarea conserva el estado elegido (completada o pendiente).
- Si el servidor rechaza la petición o falla la red, la interfaz vuelve al estado anterior en lugar de mostrar un estado que no se ha guardado.

## Evidencia y causa raíz

### Hechos observados

1. `templates/tasks/index.plush.html:16-31`: cada tarea tiene un formulario `data-task-toggle` con `action="/tasks/<id>/toggle"`, `method="POST"` y campos ocultos `authenticity_token` y `_method=PUT`. Si el navegador lo enviara de forma nativa, la petición sería correcta.
2. `public/assets/app.js:8-34`: un listener de `submit` intercepta esos formularios, llama a `event.preventDefault()`, cambia el DOM (clase `task-completed`, etiqueta y `aria-*`, badge «Completada») y después ejecuta `fetch(form.action);`.
3. `fetch(form.action)` sin opciones hace un **GET** sin cuerpo: no envía `_method=PUT` ni `authenticity_token`.
4. `actions/app.go:55`: la única ruta del toggle es `app.PUT("/tasks/{task_id}/toggle", TasksToggle)`. No hay ruta GET. Buffalo solo sobrescribe el método cuando la petición original es POST (`buffalo@v1.1.4/method_override.go:15-20`).
5. `actions/tasks.go:105-120` (`TasksToggle`) y los tests `Test_TasksToggle_CompletesAndReopens` (`actions/tasks_test.go:62`) y `Test_Task_Toggle_Persists` (`models/task_test.go:59`) muestran que el servidor guarda bien el cambio cuando recibe un PUT. El fallo está solo en el cliente.
6. Historial: `e0551d1` tenía un `app.js` que solo gestionaba `data-confirm`, así que el formulario se enviaba de forma nativa. `5fdd6c3` añadió la intercepción con `fetch(form.action)`. `d8dc18d` solo cambió etiquetas y atributos ARIA del mismo bloque.
7. Reproducción contra la aplicación en marcha (`docker compose`, puerto 3000):
   - `GET /tasks/095bfb69-202a-4b25-a23b-4d65c6f0b3c7/toggle` (la petición que envía el JS) → **404**.
   - El HTML de `/` antes y después de esa petición es idéntico: el estado no cambia.
   - `GET /assets/app.js?v=2` sirve la línea 34 `fetch(form.action);`, es decir, el contenedor ejecuta el código actual del repositorio.
8. `GET /assets/app.js?v=2` responde con `Cache-Control: max-age=31536000`. Los navegadores pueden guardar el script un año, así que corregir `app.js` sin cambiar `?v=2` en `templates/application.plush.html:30` dejaría el bug activo en los navegadores que ya tienen el script en caché.
9. CSRF (`middleware@v1.0.0/csrf/csrf.go`): fuera de `GO_ENV=test`, los métodos no seguros exigen el token, que se puede enviar en la cabecera `X-CSRF-Token` o en el campo de formulario `authenticity_token` (urlencoded o multipart). En `GO_ENV=test` la comprobación se omite.

### Causa raíz

El JavaScript optimista de `public/assets/app.js` cancela el envío nativo del formulario y lo sustituye por un `fetch` GET sin método sobrescrito ni token CSRF. La petición acaba en 404, el servidor nunca ejecuta `TasksToggle` y el cambio solo existe en el DOM hasta que se recarga la página. El código no comprueba la respuesta, así que el usuario no ve ningún error.

### Incertidumbres

- No se ha reproducido con clics en un navegador real. La reproducción equivalente con `curl` (la misma petición GET) y la lectura del código son concluyentes.
- `TasksToggle` invierte el estado guardado en lugar de fijar uno concreto. Con varias pestañas abiertas, la interfaz puede quedar desincronizada. Esto ya pasaba antes de este bug y queda fuera del alcance.

### Reproducción mínima

1. Con la aplicación en marcha, abre `/` y pulsa el botón de completar de una tarea pendiente: el DOM la muestra como completada.
2. Recarga la página: vuelve a aparecer pendiente.
3. Equivalente sin navegador: `curl -s -o /dev/null -w '%{http_code}' http://localhost:3000/tasks/<id>/toggle` devuelve `404` y `/` no cambia.

## Alcance

### Incluido

- Corregir la petición del toggle en `public/assets/app.js` para que envíe un POST con los campos del propio formulario (`_method=PUT` y `authenticity_token`), igual que el envío nativo.
- Revertir el cambio optimista del DOM si la respuesta no es correcta (`!response.ok`) o la petición falla.
- Cambiar `?v=2` por `?v=3` en la URL del script en `templates/application.plush.html` para que los navegadores descarguen el script corregido.
- Añadir tests en `actions/tasks_test.go` (ver «Tests y validación»).

### Excluido

- Cambiar `TasksToggle`, las rutas, el modelo, las migraciones o el marcado de `templates/tasks/index.plush.html`.
- Hacer que el toggle fije un estado concreto en lugar de invertirlo, o sincronizar varias pestañas.
- Añadir infraestructura de tests de JavaScript (npm, Playwright, etc.) o dependencias nuevas.
- Desactivar el botón durante la petición, añadir mensajes flash o cambiar estilos (`public/assets/app.css`).
- Cambiar el comportamiento de `data-confirm`.

## Archivos relevantes

| Ruta | Responsabilidad |
| --- | --- |
| `public/assets/app.js` | Listener de `submit`: confirmación de borrado y toggle optimista. **Contiene el bug.** |
| `templates/application.plush.html` | Layout. Carga `/assets/app.js?v=2` (línea 30). Hay que cambiar la versión para invalidar la caché. |
| `templates/tasks/index.plush.html` | Formulario `data-task-toggle` con `_method=PUT` y `authenticity_token` (solo lectura, no se modifica). |
| `actions/app.go` | Rutas. `PUT /tasks/{task_id}/toggle` y `ServeFiles("/")` para los assets (solo lectura). |
| `actions/tasks.go` | `TasksToggle`: invierte `Completed`, guarda y redirige con 303 a `/` (solo lectura). |
| `actions/tasks_test.go` | Tests de acciones. Aquí van los tests nuevos. |
| `database.yml`, `.github/workflows/ci.yaml` | Configuración de la base de datos de test y pasos de CI. |

## Implementación paso a paso

1. **Prepara la base de datos de test** (en local, `todo_test` no existe todavía). Con el servicio `db` de `compose.yaml` en marcha:
   - `GO_ENV=test go run github.com/gobuffalo/pop/v6/soda@v6.4.1 create -e test` (si la base de datos ya existe, el error se puede ignorar).
   - `GO_ENV=test go run github.com/gobuffalo/pop/v6/soda@v6.4.1 migrate -e test`.

   CI hace lo mismo con `go install ...soda@v6.4.1` y `soda migrate -e test`. Cualquiera de las dos formas vale. No cambies la configuración del repositorio.

2. **Añade primero los tests** descritos en «Tests y validación» y comprueba que `Test_ActionSuite/Test_TaskToggleScript_SendsPersistentRequest` **falla** con el código actual (`GO_ENV=test go test -p 1 ./actions/ -run 'Test_ActionSuite/Test_TaskToggleScript' -count=1`). `Test_TasksToggle_FormMethodOverride_Persists` debe pasar ya, porque documenta el contrato del servidor.

3. **Corrige `public/assets/app.js`**, manteniendo el estilo actual (ES5, `var`, `function`, comentarios en español):
   - Extrae a una función `setTaskCompleted(task, form, completed)` la actualización del DOM que hoy está en las líneas 15-32: clase `task-completed`, `.toggle-label`, `aria-pressed`, `aria-label`, `title` y creación o eliminación del badge `.task-status` «Completada». Usa `task.classList.toggle("task-completed", completed)` para fijar el estado de forma explícita.
   - En el handler, calcula `var completed = !task.classList.contains("task-completed");` y llama a `setTaskCompleted(task, form, completed);`.
   - Sustituye `fetch(form.action);` por:
     ```js
     fetch(form.action, {
       method: "POST",
       body: new URLSearchParams(new FormData(form)),
       credentials: "same-origin"
     })
       .then(function (response) {
         if (!response.ok) {
           throw new Error("No se pudo guardar el estado de la tarea");
         }
       })
       .catch(function () {
         setTaskCompleted(task, form, !completed);
       });
     ```
     `new FormData(form)` incluye los campos ocultos `_method=PUT` y `authenticity_token`. `URLSearchParams` los envía como `application/x-www-form-urlencoded`, igual que el envío nativo. El 303 a `/` se sigue de forma automática y `response.ok` refleja la respuesta final (200 si se guardó, 404 o 403 si no).
   - No cambies el bloque `data-confirm` (líneas 2-6) ni el `return` para formularios que no son de toggle.

4. **Invalida la caché del script**: en `templates/application.plush.html:30`, cambia `/assets/app.js?v=2` por `/assets/app.js?v=3`. No cambies la versión de `app.css`.

5. Ejecuta la validación completa de la sección siguiente.

## Tests y validación

### Tests nuevos en `actions/tasks_test.go`

1. `Test_TaskToggleScript_SendsPersistentRequest` (**regresión: falla antes del arreglo**):
   - `res := as.HTML("/assets/app.js").Get()` → `as.Equal(200, res.Code)`.
   - `as.NotContains(body, "fetch(form.action);")`.
   - `as.Contains(body, `method: "POST"`)`.
   - `as.Contains(body, "new FormData(form)")`.

   Es un test de contrato sobre el texto del script, porque el repositorio no tiene infraestructura de tests de JavaScript. Comprueba que el toggle ya no hace un GET sin cuerpo y que reenvía los campos del formulario.

2. `Test_TasksToggle_FormMethodOverride_Persists` (contrato del servidor del que depende el JS):
   - Crea una tarea pendiente como en `Test_TasksToggle_CompletesAndReopens`.
   - `as.HTML("/tasks/%s/toggle", task.ID).Post(url.Values{"_method": {"PUT"}, "authenticity_token": {"test"}})` → `303` con `Location: /`.
   - `as.DB.Reload(task)` → `task.Completed == true`.

### Comandos

Con `GO_ENV=test` y la base de datos de test migrada (paso 1):

- `gofmt -l .` → sin salida.
- `go vet ./...` → sin errores.
- `GO_ENV=test go test -p 1 ./...` → todo en verde, incluidos los dos tests nuevos.
- `go build ./...` → sin errores.

### Verificación manual contra la aplicación

La aplicación de `:3000` es el contenedor `app` de `compose.yaml`, construido a partir del código. Hay que reconstruirlo para que sirva el script corregido:

1. `docker compose up -d --build app` y espera a que `curl -fsS http://localhost:3000/healthz` responda. No detengas otros contenedores ni borres volúmenes.
2. `curl -s 'http://localhost:3000/assets/app.js?v=3' | grep -n 'method: "POST"'` → encuentra la línea. `curl -s http://localhost:3000/ | grep -n 'app.js?v=3'` → encuentra la línea.
3. Reproduce de forma manual la petición que envía el JS corregido, con cookie de sesión y token reales:
   - `curl -s -c jar.txt http://localhost:3000/ -o index.html` en el scratchpad o en un directorio temporal, no en el repositorio.
   - Saca un `<id>` de tarea pendiente y el `authenticity_token` de su formulario de toggle en `index.html`.
   - `curl -s -b jar.txt -c jar.txt -o /dev/null -w '%{http_code}' -X POST --data-urlencode '_method=PUT' --data-urlencode "authenticity_token=<token>" http://localhost:3000/tasks/<id>/toggle` → `303`.
   - `curl -s http://localhost:3000/` → esa tarea aparece con `task-completed` y «Completada».
   - Repite el POST con un token nuevo para devolver la tarea a su estado original y no dejar cambios en los datos semilla.
4. Opcional, si hay un navegador disponible: en `/`, completa una tarea, recarga y comprueba que sigue completada. Después reábrela, recarga y comprueba que sigue pendiente.
5. Opcional, si `node` está instalado en el host (no es una dependencia del repositorio): `node --check public/assets/app.js` para descartar errores de sintaxis.

## Riesgos y recuperación

- **Caché del navegador**: si no se cambia `?v=3`, los navegadores con el script en caché (un año) mantienen el bug. El paso 4 y la verificación manual 2 lo cubren.
- **Seguir el 303 renderiza `/` de nuevo**: la petición del toggle hace un render extra del índice y su respuesta se descarta. Además, si hubiera un flash pendiente en la sesión, ese render lo consumiría. `TasksToggle` no añade flash, así que el impacto práctico es nulo. Este cambio no lo optimiza.
- **Clics rápidos repetidos**: cada clic envía un toggle. Como el servidor invierte el estado, el resultado final coincide con el número de clics. Si una petición falla, solo se revierte su cambio en el DOM. Puede haber una desincronización puntual, que se corrige al recargar.
- **Test por texto**: `Test_TaskToggleScript_SendsPersistentRequest` depende de cadenas concretas del script. Si en el futuro se reescribe el toggle de otra forma, hay que actualizar el test junto con el script.
- **Recuperación**: el cambio se limita a `public/assets/app.js`, una línea de `templates/application.plush.html` y tests. Revertir el commit restaura el comportamiento anterior. No hay migraciones ni cambios de datos.
