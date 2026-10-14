# Chore: refactorizar `app.css` y eliminar duplicidades

## Encargo y resultado esperado

Issue [#2](https://github.com/eLafo/desarrollo-agentico-todo-03/issues/2), «QUiero refactorizar los css»: «Quiero refactorizar el css y quitar duplicidades».

Resultado esperado: `public/assets/app.css` deja de repetir los valores de diseño compartidos y el bloque de declaraciones común a los controles interactivos, sin cambiar ningún estilo calculado. Concretamente:

- Los valores literales que se repiten y representan la misma decisión de diseño pasan a ser custom properties en el bloque `:root` existente: radio (`6px`), objetivo táctil (`44px`), superficie blanca, texto sobre relleno, rosa suave primario y gris de tarea completada.
- Las ocho declaraciones repetidas en `.task-toggle-button, .icon-action` y `.button` pasan a una única regla agrupada.
- Ningún estilo calculado cambia en ningún elemento, ni en estado normal ni en `:hover`, `:focus-visible` o `:focus-within`, con anchos de 1280, 700 y 360 px. Se demuestra comparando instantáneas de `getComputedStyle` tomadas antes y después con Chrome headless.

## Estado actual y restricciones

Evidencia del repositorio (rama `chore/issue-2-refactor-css`, creada desde `origin/main` en `868fb30`):

- La aplicación usa una sola hoja de estilos: `public/assets/app.css`, de 482 líneas y sin comentarios. Su hash de blob es `git hash-object public/assets/app.css` = `e2f3947d95f9c56a0784380108877cf13ed7050c`. No hay preprocesadores, linters de CSS, `package.json` ni herramientas de front-end en el repositorio.
- `templates/application.plush.html:7` la enlaza como `/assets/app.css?v=3`. `public/embed.go` la incrusta en el binario con `//go:embed *` y `actions/app.go:59` la sirve con `app.ServeFiles`.
- El archivo ya usa custom properties (`--color-text`, `--color-muted`, `--color-border`, `--color-sidebar`, `--color-primary`, `--color-primary-hover`, `--color-focus`, `--color-danger`, `--font-family`) dentro de `:root` (líneas 17-27). Introducir más tokens no añade requisitos de compatibilidad nuevos. `calc()` con `var()` funciona en todos los navegadores que soportan custom properties.
- Todas las clases definidas en el CSS se usan en `templates/` o en `public/assets/app.js`. No hay reglas muertas que eliminar. `app.js` depende de los nombres `.task`, `.task-completed`, `.task-status`, `.task-title`, `.task-actions` y `.toggle-label`. Este cambio no renombra clases.
- Duplicidades detectadas (números de línea actuales):
  - `border-radius: 6px` aparece 6 veces: 76, 154, 257, 281, 324 y 340.
  - `44px` aparece 10 veces como objetivo táctil o columna del conmutador: 55, 74, 124, 147, 148, 240, 241, 251, 278, 354 y en `calc(100% - 44px)` (468).
  - `#ffffff` aparece 4 veces con dos papeles. Es fondo de superficie en 35 y 255, y texto sobre relleno en 166 (`.completion-icon`) y 300 (`.button-primary`).
  - `#f7e9e4` aparece 2 veces: 77 (`.nav-link`) y 177 (`.task-toggle-button:hover .completion-icon`).
  - `#777771` aparece 2 veces: 193-194 (`.task-completed .completion-icon`).
  - Las declaraciones `display: inline-flex`, `align-items: center`, `justify-content: center`, `flex-shrink: 0`, `min-height: 44px`, `border-radius: 6px`, `cursor: pointer` y `text-decoration: none` se repiten literalmente en `.task-toggle-button, .icon-action` (140-157) y en `.button` (273-291).
- Los demás valores repetidos son coincidencias entre componentes distintos y se quedan fuera (ver Alcance): tamaños de fuente `0.8rem`, `0.85rem` y `0.9rem`, pesos `600` y `750`, `transition: … 120ms ease` y el patrón `display: flex; align-items: center` de `.app-brand`, `.nav-link` y `.back-link`.
- Cascada: ningún elemento tiene a la vez `.button` y `.icon-action` o `.task-toggle-button`. Entre la posición actual de `.task-toggle-button, .icon-action` (140) y la de `.button` (273), ninguna regla afecta a elementos `.button`. Adelantar sus declaraciones comunes no cambia la cascada. Se comprobó con el prototipo descrito en «Tests y validación».
- Línea base verificada en esta máquina durante la planificación:
  - `gofmt -l .` no devuelve archivos.
  - `go vet ./...` pasa.
  - `GO_ENV=test go test -p 1 ./...` pasa: `todo/actions`, `todo/adws` y `todo/models` en `ok`.
  - Postgres corre con `docker compose`, con la base `todo_test` creada y migrada.
- Los tests Go (`actions/tasks_test.go`) comprueban textos y atributos del HTML, pero no el CSS. La equivalencia visual se valida aparte.
- CI (`.github/workflows/ci.yaml`) ejecuta `gofmt -l .`, `go vet ./...`, migraciones, `go test -p 1 ./...`, `go build ./...` y `docker build -t todo .`.
- Restricción operativa: en este entorno `cp` es un alias de `cp -i` y pide confirmación. En los comandos de este plan se usa `command cp -f`.

## Alcance

Incluido:

- Añadir seis custom properties al bloque `:root` existente de `public/assets/app.css`: `--color-surface`, `--color-primary-soft`, `--color-on-accent`, `--color-completed`, `--radius` y `--touch-target`.
- Sustituir por esos tokens todas las apariciones literales de `6px` (radio), `44px`, `#ffffff`, `#f7e9e4` y `#777771`.
- Extraer las ocho declaraciones comunes de `.task-toggle-button, .icon-action` y `.button` a una regla agrupada `.task-toggle-button, .icon-action, .button`.

Excluido, con motivo:

- Escala tipográfica (tokens para `font-size` y `font-weight`). Los valores coinciden entre componentes sin relación y nombrarlos es una decisión de diseño que la issue no pide.
- Fusionar `.app-brand`, `.nav-link` y `.back-link`, o `.list-count` y `.field-hint`, aunque compartan dos o tres declaraciones. Acoplaría componentes independientes por coincidencia.
- Los colores de un solo uso, como `#efefeb` en el borde de `.sidebar`, distinto de `--color-border`. Unificarlos cambiaría el aspecto.
- Los `36px` del bloque `@media (max-width: 760px)`, y cualquier reordenación, renombrado de clases o cambio de selectores fuera de los pasos indicados.
- El parámetro de caché `?v=3` de `templates/application.plush.html`. La hoja resultante produce estilos calculados idénticos, así que una copia en caché de la versión anterior se ve igual. No se toca ninguna plantilla.
- Plantillas, `app.js`, código Go, tests, configuración y dependencias.

## Archivos relevantes

| Ruta | Responsabilidad | Cambia |
| --- | --- | --- |
| `public/assets/app.css` | Única hoja de estilos de la aplicación. | Sí |
| `plans/issue-2-refactor-css.md` | Este plan. | Ya creado |
| `templates/application.plush.html` | Layout. Enlaza `/assets/app.css?v=3`. | No |
| `templates/tasks/index.plush.html` | Lista de tareas, formulario de alta y acciones. Consume la mayoría de clases. | No |
| `templates/tasks/edit.plush.html` | Formulario de edición (`.back-link`, `.edit-task`, `.field-hint`, `.button`). | No |
| `templates/_flash.plush.html` | Mensajes `.flash .flash-<clave>`. Solo se usa la clave `success` (`actions/tasks.go:56,101,135`). | No |
| `public/assets/app.js` | Alterna `.task-completed` y crea `.task-status` en el cliente. | No |
| `public/embed.go` | Incrusta `public/` en el binario. Por eso `go build`/`go test` cubren el empaquetado. | No |
| `actions/app.go` | Sirve los estáticos (`app.ServeFiles`). | No |
| `actions/tasks_test.go` | Tests HTTP de las páginas. No inspeccionan el CSS. | No |
| `.github/workflows/ci.yaml` | Validaciones de CI. | No |

## Implementación paso a paso

Ejecuta todos los comandos desde la raíz del repositorio.

### Paso 0. Comprobar el estado de partida

```sh
git status --short --branch
git hash-object public/assets/app.css
```

La rama debe ser `chore/issue-2-refactor-css`. El único cambio pendiente puede ser este plan. El hash debe ser `e2f3947d95f9c56a0784380108877cf13ed7050c`. Si no coincide, detente: el CSS cambió desde la planificación y los números de línea y bloques de este plan ya no son fiables.

### Paso 1. Preparar el arnés de comparación y tomar la instantánea «antes»

El arnés vive fuera del repositorio, en `/tmp/issue-2-css`, y no se versiona. Usa el Google Chrome instalado (`/Applications/Google Chrome.app`). Chrome headless impone un ancho mínimo de ventana de 500 px, así que `harness.html` carga la página de prueba en un `iframe` del ancho pedido. `--allow-file-access-from-files` permite leer el `iframe` desde `file://`.

`fixture.html` reproduce el marcado de las tres plantillas y del flash, con todas las clases y estados que estiliza el CSS: tarea pendiente y completada, `.task-status`, campo inválido y válido, placeholder, `.field-error`, `.field-hint`, `.empty-state`, `.flash-success`, botones normal y primario. Su script toma dos instantáneas de `getComputedStyle` de todos los elementos, incluido `::placeholder` de los `input`, descartando las custom properties:

1. `default`: estado normal.
2. `hover-focus`: reescribe la hoja manteniendo el orden de reglas. Cambia `:hover` por `[data-hover]` y `:focus-visible`/`:focus-within` por `[data-focus]`, marca todos los elementos y finaliza las transiciones antes de medir.

```sh
mkdir -p /tmp/issue-2-css

cat > /tmp/issue-2-css/fixture.html <<'HTML'
<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="app.css">
</head>
<body>
<div class="app-layout">
  <aside class="sidebar">
    <a href="#" class="app-brand"><span>todo<span class="brand-dot">.</span></span></a>
    <nav><a href="#" class="nav-link"><svg viewBox="0 0 24 24"><path d="m8 12 3 3 5-6" /></svg>Tareas</a></nav>
  </aside>
  <main class="container">
    <p class="flash flash-success" role="status">Tarea creada.</p>
    <header class="page-header">
      <a href="#" class="back-link"><span>←</span> Volver a la lista</a>
      <h1>Tareas</h1>
      <p class="list-count">2 tareas en tu lista</p>
    </header>
    <section>
      <h2 class="sr-only">Lista de tareas</h2>
      <p class="empty-state">No hay tareas todavía.</p>
      <ul class="task-list">
        <li class="task">
          <form class="task-toggle"><button type="submit" class="task-toggle-button"><span class="completion-icon"><svg viewBox="0 0 24 24"><path d="m6 12 4 4 8-8" /></svg></span><span class="toggle-label sr-only">Completar</span></button></form>
          <span class="task-title">Tarea pendiente</span>
          <div class="task-actions">
            <a href="#" class="icon-action"><svg viewBox="0 0 24 24"><path d="M4 20 20 4" /></svg></a>
            <form><button type="submit" class="icon-action icon-danger"><svg viewBox="0 0 24 24"><path d="M4 7h16" /></svg></button></form>
          </div>
        </li>
        <li class="task task-completed">
          <form class="task-toggle"><button type="submit" class="task-toggle-button"><span class="completion-icon"><svg viewBox="0 0 24 24"><path d="m6 12 4 4 8-8" /></svg></span><span class="toggle-label sr-only">Reabrir</span></button></form>
          <span class="task-title">Tarea completada</span>
          <span class="task-status">Completada</span>
          <div class="task-actions">
            <a href="#" class="icon-action"><svg viewBox="0 0 24 24"><path d="M4 20 20 4" /></svg></a>
            <form><button type="submit" class="icon-action icon-danger"><svg viewBox="0 0 24 24"><path d="M4 7h16" /></svg></button></form>
          </div>
        </li>
      </ul>
    </section>
    <form class="new-task">
      <div class="new-task-row">
        <span class="add-mark">+</span>
        <label for="new-title" class="sr-only">Nueva tarea</label>
        <input id="new-title" type="text" placeholder="Añadir una tarea…" aria-invalid="true">
        <button type="submit" class="button button-primary">Añadir</button>
      </div>
      <p class="field-error" role="alert">El título es obligatorio.</p>
    </form>
    <form class="edit-task">
      <label for="edit-title">Título</label>
      <input id="edit-title" type="text" value="Título original">
      <p class="field-hint">Hasta 200 caracteres.</p>
      <div class="edit-task-actions">
        <a href="#" class="button">Cancelar</a>
        <button type="submit" class="button button-primary">Guardar cambios</button>
      </div>
    </form>
  </main>
</div>
<script>
  (function () {
    var lines = [
      "viewport " + window.innerWidth + "x" + window.innerHeight,
      "hover-fine " + matchMedia("(hover: hover) and (pointer: fine)").matches,
      "reduced-motion " + matchMedia("(prefers-reduced-motion: reduce)").matches
    ];
    var elements = document.querySelectorAll("html, body, body *:not(script)");
    function snapshot(state) {
      var targets = [];
      elements.forEach(function (el) {
        targets.push([el, null]);
        if (el.tagName === "INPUT") targets.push([el, "::placeholder"]);
      });
      targets.forEach(function (t, i) {
        var el = t[0];
        var cls = el.getAttribute("class") || "";
        lines.push("## " + state + " " + i + " " + el.tagName.toLowerCase() + (cls ? "." + cls.split(/\s+/).join(".") : "") + (t[1] || ""));
        var cs = getComputedStyle(el, t[1]);
        var props = [];
        for (var j = 0; j < cs.length; j++) {
          if (cs[j].indexOf("--") !== 0) props.push(cs[j]);
        }
        props.sort().forEach(function (p) {
          lines.push(p + ": " + cs.getPropertyValue(p));
        });
      });
    }
    snapshot("default");
    // Fuerza :hover, :focus-visible y :focus-within en todos los elementos,
    // conservando el orden de las reglas para no alterar la cascada.
    var sheet = document.styleSheets[0];
    var css = Array.prototype.map.call(sheet.cssRules, function (rule) {
      return rule.cssText;
    }).join("\n")
      .replace(/:hover/g, "[data-hover]")
      .replace(/:focus-visible|:focus-within/g, "[data-focus]");
    sheet.disabled = true;
    var forced = document.createElement("style");
    forced.textContent = css;
    document.head.appendChild(forced);
    elements.forEach(function (el) {
      el.setAttribute("data-hover", "");
      el.setAttribute("data-focus", "");
    });
    document.getAnimations().forEach(function (animation) {
      animation.finish();
    });
    snapshot("hover-focus");
    var pre = document.createElement("pre");
    pre.id = "snapshot";
    pre.textContent = lines.join("\n");
    document.body.replaceChildren(pre);
  })();
</script>
</body>
</html>
HTML

cat > /tmp/issue-2-css/harness.html <<'HTML'
<!DOCTYPE html>
<html>
<body style="margin:0">
<iframe id="frame" style="border:0;height:900px"></iframe>
<script>
  var frame = document.getElementById("frame");
  frame.style.width = location.hash.slice(1) + "px";
  frame.onload = function () {
    var text = frame.contentDocument.getElementById("snapshot").textContent;
    var pre = document.createElement("pre");
    pre.textContent = text;
    document.body.replaceChildren(pre);
  };
  frame.src = "fixture.html";
</script>
</body>
</html>
HTML

git show HEAD:public/assets/app.css > /tmp/issue-2-css/app.css

CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
for w in 1280 700 360; do
  "$CHROME" --headless=new --disable-gpu --hide-scrollbars --allow-file-access-from-files \
    --window-size=1400,1000 --virtual-time-budget=3000 \
    --dump-dom "file:///tmp/issue-2-css/harness.html#$w" 2>/dev/null > "/tmp/issue-2-css/before-$w.html"
done

grep -ho "viewport [0-9]*x[0-9]*" /tmp/issue-2-css/before-*.html
grep -c "^## " /tmp/issue-2-css/before-1280.html
```

Resultado esperado:

- Las líneas `viewport 1280x900`, `viewport 700x900` y `viewport 360x900`, en cualquier orden.
- `142` bloques: 71 objetivos por cada uno de los dos estados.

Si Chrome no está, si los anchos no coinciden o si sale 0 bloques, detente. Sin la línea base no se puede validar el cambio.

### Paso 2. Ampliar `:root`

En `public/assets/app.css`, sustituye el bloque `:root` (líneas 17-27) por exactamente:

```css
:root {
  --color-text: #282826;
  --color-muted: #686864;
  --color-border: #e9e9e5;
  --color-sidebar: #f7f7f5;
  --color-surface: #ffffff;
  --color-primary: #b73526;
  --color-primary-hover: #992b1e;
  --color-primary-soft: #f7e9e4;
  --color-on-accent: #ffffff;
  --color-completed: #777771;
  --color-focus: #245ab4;
  --color-danger: #a52b25;
  --font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  --radius: 6px;
  --touch-target: 44px;
}
```

El bloque mantiene su posición. Las propiedades existentes conservan su orden y valores. Los tokens nuevos se insertan junto a los de su familia.

### Paso 3. Sustituir los valores literales por tokens

Cambia solo el valor indicado y deja intactas las líneas vecinas. Los números de línea son los del archivo original.

| Regla | Línea original | Antes | Después |
| --- | --- | --- | --- |
| `body` | 35 | `background: #ffffff;` | `background: var(--color-surface);` |
| `.app-brand` | 55 | `min-height: 44px;` | `min-height: var(--touch-target);` |
| `.nav-link` | 74 | `min-height: 44px;` | `min-height: var(--touch-target);` |
| `.nav-link` | 76 | `border-radius: 6px;` | `border-radius: var(--radius);` |
| `.nav-link` | 77 | `background: #f7e9e4;` | `background: var(--color-primary-soft);` |
| `.task` | 124 | `grid-template-columns: 44px minmax(0, 1fr) auto;` | `grid-template-columns: var(--touch-target) minmax(0, 1fr) auto;` |
| `.completion-icon` | 166 | `color: #ffffff;` | `color: var(--color-on-accent);` |
| `.task-toggle-button:hover .completion-icon` | 177 | `background: #f7e9e4;` | `background: var(--color-primary-soft);` |
| `.task-completed .completion-icon` | 193 | `background: #777771;` | `background: var(--color-completed);` |
| `.task-completed .completion-icon` | 194 | `border-color: #777771;` | `border-color: var(--color-completed);` |
| `.add-mark` | 240 | `width: 44px;` | `width: var(--touch-target);` |
| `.add-mark` | 241 | `height: 44px;` | `height: var(--touch-target);` |
| `input[type="text"]` | 251 | `min-height: 44px;` | `min-height: var(--touch-target);` |
| `input[type="text"]` | 255 | `background: #ffffff;` | `background: var(--color-surface);` |
| `input[type="text"]` | 257 | `border-radius: 6px;` | `border-radius: var(--radius);` |
| `.button-primary` | 300 | `color: #ffffff;` | `color: var(--color-on-accent);` |
| `.field-error` | 324 | `border-radius: 6px;` | `border-radius: var(--radius);` |
| `.flash` | 340 | `border-radius: 6px;` | `border-radius: var(--radius);` |
| `.back-link` | 354 | `min-height: 44px;` | `min-height: var(--touch-target);` |
| `@media (max-width: 380px)` → `.new-task-row input` | 468 | `flex-basis: calc(100% - 44px);` | `flex-basis: calc(100% - var(--touch-target));` |

Las líneas 147-148, 154 y 278-281 no se tocan aquí. Las reescribe el paso 4.

### Paso 4. Unificar la base común de los controles

1. Sustituye la regla original `.task-toggle-button, .icon-action { … }` (líneas 140-157) por estas dos reglas, en este orden y en la misma posición:

```css
.task-toggle-button,
.icon-action,
.button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  min-height: var(--touch-target);
  border-radius: var(--radius);
  cursor: pointer;
  text-decoration: none;
}

.task-toggle-button,
.icon-action {
  position: relative;
  width: var(--touch-target);
  padding: 0;
  font: inherit;
  color: var(--color-muted);
  background: transparent;
  border: 0;
}
```

2. Sustituye la regla original `.button { … }` (líneas 273-291) por la siguiente. Mantén su posición, justo antes de `.button:hover`:

```css
.button {
  padding: 0.6rem 0.9rem;
  border: 1px solid #dadad4;
  color: #41413d;
  background: #f6f6f2;
  font-family: inherit;
  font-size: 0.85rem;
  font-weight: 600;
  line-height: 1.5;
  white-space: nowrap;
}
```

No muevas, renombres ni reordenes ninguna otra regla, incluidas las de `:hover`, `:focus-visible` y las de los bloques `@media`.

### Paso 5. Comprobaciones estáticas

```sh
grep -nE '(^|[^0-9.])6px|44px|#ffffff|#f7e9e4|#777771' public/assets/app.css
git diff --check
git diff --stat
```

Resultado esperado:

- El `grep` devuelve solo las seis definiciones de `:root`: `--color-surface`, `--color-primary-soft`, `--color-on-accent`, `--color-completed`, `--radius` y `--touch-target`, en las líneas 22, 25, 26, 27, 31 y 32. El patrón excluye `36px`, `1.5px` y `-4px`.
- `git diff --check` no devuelve nada.
- `git diff --stat` muestra solo `public/assets/app.css`, con 38 inserciones y 35 borrados. El archivo pasa de 482 a 485 líneas.

### Paso 6. Instantánea «después» y comparación

```sh
command cp -f public/assets/app.css /tmp/issue-2-css/app.css

CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
for w in 1280 700 360; do
  "$CHROME" --headless=new --disable-gpu --hide-scrollbars --allow-file-access-from-files \
    --window-size=1400,1000 --virtual-time-budget=3000 \
    --dump-dom "file:///tmp/issue-2-css/harness.html#$w" 2>/dev/null > "/tmp/issue-2-css/after-$w.html"
done

for w in 1280 700 360; do
  diff -q "/tmp/issue-2-css/before-$w.html" "/tmp/issue-2-css/after-$w.html" && echo "$w idéntico"
done
```

Resultado esperado: `1280 idéntico`, `700 idéntico` y `360 idéntico`, sin salida de `diff`. Si algún ancho difiere, revisa la diferencia con `diff /tmp/issue-2-css/before-<w>.html /tmp/issue-2-css/after-<w>.html` y corrige solo la desviación respecto al texto de los pasos 2-4. Si no se puede alcanzar la igualdad aplicando el plan tal como está, detente y repórtalo.

## Tests y validación

No se añaden ni modifican tests. La equivalencia del CSS no es observable desde los tests Go: comprueban el HTML, no los estilos. Se valida con el arnés de los pasos 1 y 6.

Validaciones a ejecutar, en este orden, después del paso 6:

1. Equivalencia visual: el paso 6 debe devolver «idéntico» en los tres anchos. Es el criterio decisivo de que no cambia el comportamiento observable. Cubre:
   - todos los estilos calculados de todos los elementos del marcado real;
   - los estados de hover y foco;
   - los tres tramos de media queries: escritorio, `≤760px` y `≤380px`;
   - en escritorio, la rama `(hover: hover) and (pointer: fine)`, que se evalúa como `true`.
2. Estáticas del paso 5: el `grep` solo encuentra líneas de `:root`, `git diff --check` sale limpio y el diff solo toca `public/assets/app.css`.
3. Go, como en CI. El CSS se incrusta en el binario mediante `public/embed.go`:

   ```sh
   gofmt -l .
   go vet ./...
   go build ./...
   docker compose ps db
   GO_ENV=test go test -count=1 -p 1 ./...
   ```

   Resultado esperado:

   - `gofmt -l .` sin salida.
   - `go vet ./...` y `go build ./...` sin errores.
   - `docker compose ps db` muestra el servicio `healthy`. Si no lo está, ejecuta `docker compose up -d db` y espera a que lo esté.
   - Tests: `ok` en `todo/actions`, `todo/adws` y `todo/models`.

   Si Postgres o la base `todo_test` no están disponibles, la validación queda sin ejecutar. Repórtalo como tal y no lo des por bueno.

`docker build -t todo .` no se ejecuta en local: el `Dockerfile` no cambia y `go build ./...` ya cubre la incrustación del CSS. Lo ejecuta CI en el job `docker`.

Comprobación del propio arnés, ya hecha durante la planificación con un prototipo idéntico: con el CSS refactorizado, las tres instantáneas salieron idénticas a las originales. Como control negativo, al cambiar solo el fondo de `.task-toggle-button:hover .completion-icon` (`#f7e9e4` → `#f7e9e5`) el `diff` detectó 2 diferencias en `background-color`, y al quitar `.button` de la regla agrupada detectó cambios en `display`, `min-height`, `border-radius`, etc. La implementación no necesita repetir estos controles.

## Riesgos y recuperación

- **Cambio involuntario de cascada al agrupar `.button`.** Mitigación: el análisis del estado actual y el paso 6 lo detectan en estado normal, hover y foco. Recuperación: `git restore public/assets/app.css` y volver a aplicar los pasos 2-4.
- **Error tipográfico en un `var()`.** Un nombre inexistente deja la propiedad en su valor inicial sin error visible. Mitigación: el paso 6 lo detecta como diferencia de estilos calculados. El `grep` del paso 5 confirma que no quedan literales.
- **Cobertura del arnés.**
  - La página de prueba es una réplica a mano de las plantillas actuales. Si las plantillas cambian antes de implementar, el paso 0 no lo detecta, pero este plan no las toca.
  - No cubre `@media (prefers-reduced-motion: reduce)`, que no cambia en este plan, ni claves de flash distintas de `success`, que la aplicación no usa.
  - Se mide solo en Chrome. Al ser una refactorización sin cambios de valores, el riesgo de divergencia entre navegadores es bajo, porque `var()` y `calc()` con `var()` ya se usaban o tienen el mismo soporte.
- **Caché de navegador.** El enlace sigue en `?v=3`. Un navegador con la versión anterior en caché ve el mismo resultado, así que no hace falta invalidarla.
- **Entorno.**
  - El arnés depende de `/Applications/Google Chrome.app` (Chrome 155 durante la planificación). Si no existe, la validación del paso 6 no puede ejecutarse y hay que detenerse.
  - El alias `cp -i` obliga a usar `command cp -f`.
  - Los archivos de `/tmp/issue-2-css` son temporales, quedan fuera del repositorio y no deben añadirse al commit.
- **Recuperación general.** El cambio afecta a un único archivo estático. `git restore public/assets/app.css` vuelve al estado de partida, cuyo hash de blob es `e2f3947d95f9c56a0784380108877cf13ed7050c`.
