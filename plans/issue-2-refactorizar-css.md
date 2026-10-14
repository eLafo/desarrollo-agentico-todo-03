# Chore: Refactorizar `app.css` y eliminar duplicidades

## Encargo y resultado esperado

Issue [#2](https://github.com/eLafo/desarrollo-agentico-todo-03/issues/2), «QUiero refactorizar los css»: «Quiero refactorizar el css y quitar duplicidades».

Resultado esperado:

- `public/assets/app.css` centraliza en propiedades personalizadas de `:root` los valores literales que se repiten porque representan la misma decisión de diseño: radio de borde, tamaño mínimo de objetivo táctil, tinte suave del color primario y duración de las transiciones.
- Los bloques de declaraciones duplicados se fusionan en reglas compartidas: la base común de los controles (`.task-toggle-button`, `.icon-action`, `.button`) y la opacidad visible del icono de completado.
- El aspecto de la aplicación no cambia: en todas las páginas, estados y anchos comprobados, los estilos calculados por el navegador son idénticos antes y después. El paso de validación lo demuestra con 0 diferencias.

## Estado actual y restricciones

**Hoja de estilos única.** El repositorio solo tiene un archivo CSS: `public/assets/app.css`, con 482 líneas en la base `868fb30`. No hay preprocesadores, `package.json`, linters de CSS ni ningún paso de build para CSS. `public/embed.go` lo embebe con `//go:embed *` y `actions/app.go` lo sirve con `app.ServeFiles("/", http.FS(public.FS()))`. `templates/application.plush.html:7` lo enlaza como `/assets/app.css?v=3`.

**Selectores que no se pueden renombrar.** Las plantillas `templates/application.plush.html`, `templates/_flash.plush.html`, `templates/tasks/index.plush.html` y `templates/tasks/edit.plush.html` usan todas las clases definidas en el CSS. `public/assets/app.js` también depende de `.task`, `.task-completed`, `.task-title`, `.task-status`, `.task-actions`, `.toggle-label` y `[data-task-toggle]`. Este chore no renombra ni elimina selectores.

**Duplicidades detectadas** (números de línea de la base):

| Valor o bloque | Apariciones |
|---|---|
| `border-radius: 6px` | 76 (`.nav-link`), 154 (`.task-toggle-button, .icon-action`), 257 (`input[type="text"]`), 281 (`.button`), 324 (`.field-error`), 340 (`.flash`) |
| `44px` (objetivo táctil) | 55, 74, 124, 147, 148, 240, 241, 251, 278, 354 y 468 (`calc(100% - 44px)`) |
| `#f7e9e4` (tinte primario suave) | 77 (`.nav-link`), 177 (`.task-toggle-button:hover .completion-icon`) |
| `120ms` (duración de transición) | 167 (dos veces en la misma declaración), 398 |
| Base de control: `display: inline-flex; align-items: center; justify-content: center; flex-shrink: 0; min-height: 44px; border-radius: 6px; cursor: pointer; text-decoration: none;` | repetida en `.task-toggle-button, .icon-action` (140-157) y en `.button` (273-291) |
| `opacity: 1` sobre `.completion-icon svg` | `.task-toggle-button:hover .completion-icon svg` (182-184) y `.task-completed .completion-icon svg` (197-199) |

**Trampas que no son duplicidades y deben quedar intactas:**

- `.task-actions { opacity: 1; }` dentro de `@media (max-width: 760px)` no es redundante. Anula `opacity: 0` de `@media (hover: hover) and (pointer: fine)` en ventanas de escritorio estrechas. Funciona porque es posterior en el código y tiene la misma especificidad. No muevas ninguno de los dos bloques `@media`.
- `.button { line-height: 1.5; font-family: inherit; }` no es redundante. Los `<button>` no heredan esos valores del `body`.
- `.sr-only, .task-status` debe seguir agrupado. El `<span class="task-status">` no lleva la clase `sr-only`, ni en la plantilla ni cuando lo crea `app.js`.
- Los grises `#858580`, `#96968f`, `#777771`, `#92928a`, `#dadad4`, `#efefeb`, `#e9e9e5` y los rojos `#a03123`, `#912a20`, `#a52b25` son valores distintos. Unificarlos cambiaría el aspecto.
- `.task-toggle-button:hover .completion-icon`, con especificidad (0,3,0), prevalece sobre `.task-completed .completion-icon`, con (0,2,0). La fusión no debe alterar ninguno de esos selectores.
- `.back-link` comparte tres declaraciones con la base de control, pero no se incluye en ella. Añadirle `border-radius` cambiaría la forma del contorno de foco, porque `outline` sigue el radio del borde.

**Validación disponible.** CI (`.github/workflows/ci.yaml`) ejecuta `gofmt -l .`, `go vet ./...`, `soda migrate -e test`, `go test -p 1 ./...`, `go build ./...` y `docker build -t todo .`. Ninguno inspecciona el CSS: los tests de `actions/tasks_test.go` solo comprueban texto del HTML. Para demostrar que el comportamiento observable se conserva, este plan usa una comparación de estilos calculados en Chromium con `playwright-cli` (versión 0.1.20, instalado en `/opt/homebrew/bin/playwright-cli`). No añade nada al repositorio.

Durante la planificación se verificó lo siguiente:

- Las validaciones Go pasan en la base con `GO_ENV=test` y la base de datos `todo_test` del Postgres de `docker compose`.
- El comparador da 26 comprobaciones y 0 diferencias con el CSS sin cambios.
- Un control negativo con tres cambios mínimos (un color de hover, un radio y una duración de transición) produjo 97 diferencias en los estados esperados.
- Una copia de la hoja con exactamente los cambios de este plan dio 0 diferencias en las 26 comprobaciones.

**Entorno local.** Go 1.27.2 en el host. `buffalo` y `soda` no están instalados en el host. `docker compose` tiene `db` en `5432` y `app` en `3000`. La lista de desarrollo tenía 3 tareas durante la planificación.

## Alcance

**Incluido:**

1. Añadir a `:root` estas propiedades: `--color-primary-soft: #f7e9e4`, `--radius: 6px`, `--tap-target: 44px` y `--duration-fast: 120ms`.
2. Sustituir cada aparición literal de esos valores por su `var(...)`:
   - las 6 de `border-radius: 6px`;
   - las 11 de `44px`, incluido `calc(100% - 44px)`;
   - las 2 de `#f7e9e4`;
   - las 3 de `120ms`.
3. Extraer la base común de controles a una regla `.task-toggle-button, .icon-action, .button` y quitar esas 8 declaraciones de las reglas originales.
4. Fusionar las dos reglas `opacity: 1` del SVG del icono de completado en una regla con dos selectores.

**Excluido (con motivo):**

- **Tamaños de fuente** (`0.8rem`, `0.85rem` y `0.9rem` repetidos). El proyecto no define una escala tipográfica, y nombrarla es una decisión de diseño que el encargo no pide.
- **`#ffffff`**. Cumple dos funciones distintas: fondo, y texto sobre un relleno. Tokenizarlo exigiría dos tokens para poco beneficio.
- **`1px solid var(--color-border)`**. El color ya está tokenizado.
- **`36px` en `@media (max-width: 760px)`**. Son dos usos contiguos en el mismo bloque, y el botón de completar sigue midiendo 44px en móvil, así que no es el mismo concepto que `--tap-target`.
- **`display: grid; place-items: center`** compartido por `.completion-icon` y `.add-mark`. Fusionarlo añade una regla para ahorrar dos líneas.
- **Reordenar reglas o mover `:root`**. Reordenar puede alterar la cascada y no aporta al encargo.
- **El parámetro `?v=3` de `templates/application.plush.html`**. El resultado visual es idéntico, así que una copia en caché de la versión anterior se ve igual. No hace falta invalidarla, y el diff queda limitado a un archivo.
- **Cambios en plantillas, JavaScript, código Go, tests, CI o dependencias.**
- **Cualquier cambio visual, aunque parezca una mejora.**

## Archivos relevantes

| Ruta | Responsabilidad |
|---|---|
| `public/assets/app.css` | Única hoja de estilos. Es el único archivo de código que cambia. |
| `public/embed.go` | Embebe `public/` en el binario. No cambia; `go build` confirma que sigue compilando. |
| `templates/application.plush.html` | Enlaza `/assets/app.css?v=3` y define el layout (`.app-layout`, `.sidebar`, `.nav-link`). No cambia. |
| `templates/tasks/index.plush.html`, `templates/tasks/edit.plush.html`, `templates/_flash.plush.html` | Usan las clases afectadas. No cambian; el comparador los recorre. |
| `public/assets/app.js` | Depende de nombres de clase. No cambia. |
| `actions/tasks.go` | Las validaciones fallidas de crear y editar renderizan con error sin persistir. El comparador lo aprovecha para no tocar datos. |
| `.github/workflows/ci.yaml` | Define las validaciones Go y Docker que ejecuta CI. |
| `plans/issue-2-refactorizar-css.md` | Este plan. |

## Implementación paso a paso

Ejecuta todo desde la raíz del repositorio. No toques ningún archivo distinto de `public/assets/app.css`.

### 1. Captura la base y prepara el comparador (antes de editar)

1.1. Copia el CSS de la base y enlaza el del árbol de trabajo:

```bash
mkdir -p /tmp/css-issue-2
git show "$(git merge-base HEAD origin/main)":public/assets/app.css > /tmp/css-issue-2/before.css
ln -sf "$PWD/public/assets/app.css" /tmp/css-issue-2/after.css
cmp /tmp/css-issue-2/before.css public/assets/app.css && echo "base capturada"
```

1.2. Crea `/tmp/css-issue-2/compare.js` con este contenido exacto. El script carga cada página una vez y aplica primero `before.css` y después `after.css` sobre el mismo DOM. Compara todas las propiedades calculadas no personalizadas de cada elemento y de sus pseudoelementos `::before` y `::after`. No crea, edita ni borra datos: los estados completado, flash y vacío se inyectan en el DOM, y los estados de error proceden de envíos con título vacío que el servidor rechaza.

```js
async page => {
  const APP = 'http://127.0.0.1:3100';
  const CSS = 'http://127.0.0.1:3199';
  const AFTER = 'after.css';
  const before = await (await page.request.get(`${CSS}/before.css`)).text();
  const after = await (await page.request.get(`${CSS}/${AFTER}`)).text();
  const diffs = [];
  const checks = [];

  const snapshot = css => page.evaluate(css => {
    document.querySelectorAll('link[rel="stylesheet"]').forEach(l => l.remove());
    const sheet = new CSSStyleSheet();
    sheet.replaceSync(css);
    document.adoptedStyleSheets = [sheet];
    void document.body.offsetHeight;
    document.getAnimations().forEach(a => a.finish());
    const out = {};
    const els = [document.documentElement, ...document.querySelectorAll('body, body *')];
    els.forEach((el, i) => {
      const cls = typeof el.className === 'string' && el.className.trim() ? '.' + el.className.trim().split(/\s+/).join('.') : '';
      const key = `${i}:${el.tagName.toLowerCase()}${cls}`;
      for (const pseudo of [null, '::before', '::after']) {
        const cs = getComputedStyle(el, pseudo);
        const props = {};
        for (let j = 0; j < cs.length; j++) {
          const p = cs[j];
          if (!p.startsWith('--')) props[p] = cs.getPropertyValue(p);
        }
        out[key + (pseudo || '')] = props;
      }
    });
    return out;
  }, css);

  const compare = async label => {
    const a = await snapshot(before);
    const b = await snapshot(after);
    checks.push(label);
    for (const k of Object.keys(a)) {
      for (const p of Object.keys(a[k])) {
        if (!b[k] || a[k][p] !== b[k][p]) diffs.push(`${label} | ${k} | ${p}: ${a[k][p]} -> ${b[k] ? b[k][p] : '(sin elemento)'}`);
      }
    }
  };

  const atWidths = async label => {
    for (const [width, height] of [[1280, 900], [700, 900], [360, 800]]) {
      await page.setViewportSize({ width, height });
      await compare(`${label} @${width}`);
    }
    await page.setViewportSize({ width: 1280, height: 900 });
  };

  const hover = async (selector, label) => {
    const el = page.locator(selector).first();
    if (await el.count() === 0) throw new Error(`No existe ${selector}`);
    await el.hover();
    await compare(`${label} :hover`);
    await page.mouse.move(1, 1);
  };

  await page.setViewportSize({ width: 1280, height: 900 });

  // Índice con al menos una tarea pendiente y una completada (solo en el DOM).
  await page.goto(`${APP}/`);
  const taskCount = await page.locator('.task').count();
  if (taskCount < 2) throw new Error('Se necesitan al menos 2 tareas en la lista');
  await page.evaluate(() => {
    const tasks = document.querySelectorAll('.task');
    tasks[0].classList.remove('task-completed');
    tasks[1].classList.add('task-completed');
  });
  await atWidths('index');
  await hover('.nav-link', 'nav-link');
  await hover('.task:not(.task-completed) .task-toggle-button', 'toggle pendiente');
  await hover('.task.task-completed .task-toggle-button', 'toggle completada');
  await hover('.task .icon-action:not(.icon-danger)', 'editar');
  await hover('.task .icon-danger', 'eliminar');
  await hover('.new-task .button-primary', 'añadir');
  await page.keyboard.press('Tab');
  await compare('index foco teclado');

  // Mensaje flash y estado vacío inyectados en el DOM (sin tocar datos).
  await page.evaluate(() => {
    document.querySelector('main').insertAdjacentHTML('afterbegin', '<p class="flash flash-success" role="status">Tarea creada.</p>');
    document.querySelector('.task-list').insertAdjacentHTML('beforebegin', '<p class="empty-state">No hay tareas todavía. Añade la primera con el formulario.</p>');
  });
  await atWidths('flash y vacío');

  // Error de validación al crear (no crea ninguna tarea).
  await page.goto(`${APP}/`);
  await page.fill('#task-title', '');
  await page.click('.new-task button[type="submit"]');
  await page.waitForSelector('.field-error');
  await atWidths('index error');

  // Edición de la primera tarea.
  await page.goto(`${APP}/`);
  const editHref = await page.locator('.task .icon-action:not(.icon-danger)').first().getAttribute('href');
  await page.goto(`${APP}${editHref}`);
  await atWidths('edit');
  await hover('.back-link', 'volver');
  await hover('.edit-task-actions .button:not(.button-primary)', 'cancelar');
  await hover('.edit-task-actions .button-primary', 'guardar');

  // Error de validación al editar (no modifica la tarea).
  await page.fill('#task-title', '');
  await page.click('.edit-task-actions button[type="submit"]');
  await page.waitForSelector('.field-error');
  await atWidths('edit error');

  // Movimiento reducido.
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.goto(`${APP}/`);
  await compare('index movimiento reducido');
  await page.emulateMedia({ reducedMotion: null });

  return { comprobaciones: checks.length, diferencias: diffs.length, muestra: diffs.slice(0, 40) };
}
```

1.3. Arranca los servicios auxiliares en segundo plano y conserva su salida en `/tmp/css-issue-2/*.log`. El puerto `3000` lo ocupa la app de `docker compose`, así que la rama se sirve en `3100`. Las CSS se sirven en `3199`.

```bash
docker compose up -d db
python3 -m http.server 3199 --bind 127.0.0.1 -d /tmp/css-issue-2 > /tmp/css-issue-2/http.log 2>&1 &
PORT=3100 go run ./cmd/app > /tmp/css-issue-2/app.log 2>&1 &
for i in $(seq 1 90); do curl -sf http://127.0.0.1:3100/healthz >/dev/null && break; sleep 1; done
curl -s -o /dev/null -w 'app %{http_code}\n' http://127.0.0.1:3100/healthz      # espera 200
curl -s -o /dev/null -w 'css %{http_code}\n' http://127.0.0.1:3199/after.css    # espera 200
```

Si `3100` o `3199` ya están ocupados por otro proceso, no lo detengas. Usa otro puerto libre y cambia de forma coherente `PORT`, el puerto de `http.server` y las constantes `APP`/`CSS` de `compare.js`.

1.4. Precondición de datos: `curl -s http://127.0.0.1:3100/ | grep -c 'class="task-title"'` debe devolver `2` o más. Si devuelve menos, crea desde `http://127.0.0.1:3100/` las tareas `Validación CSS 1` y `Validación CSS 2`, apunta que existen y elimínalas en el paso 4.

1.5. Ejecuta el comparador sobre la base. Lánzalo siempre desde `/tmp/css-issue-2`, porque `playwright-cli` crea un directorio `.playwright-cli/` en el directorio actual y no debe ensuciar el repositorio:

```bash
(cd /tmp/css-issue-2 && playwright-cli -s=css-issue-2 open about:blank \
  && playwright-cli -s=css-issue-2 run-code --filename=/tmp/css-issue-2/compare.js > run-before.txt 2>&1; sed -n 2p run-before.txt)
```

Resultado esperado: `{"comprobaciones":26,"diferencias":0,"muestra":[]}`. Si no es exactamente eso, detente: el comparador o el entorno no son fiables.

### 2. Edita `public/assets/app.css`

Haz exactamente estos cambios, en este orden. No modifiques ninguna otra línea, el orden de las reglas, el formato ni los comentarios.

2.1. En `:root`, añade `--color-primary-soft: #f7e9e4;` justo después de `--color-primary-hover: #992b1e;`. Añade `--radius: 6px;`, `--tap-target: 44px;` y `--duration-fast: 120ms;`, en ese orden, justo después de `--font-family: …;`.

2.2. Sustituye los literales por sus tokens:

- Las 6 apariciones de `border-radius: 6px;` pasan a `border-radius: var(--radius);`. No toques `border-radius: 50%` de `.completion-icon`.
- Los `min-height: 44px;` de `.app-brand`, `.nav-link`, `.task-toggle-button, .icon-action`, `input[type="text"]`, `.button` y `.back-link` pasan a `min-height: var(--tap-target);`.
- `.task { grid-template-columns: 44px minmax(0, 1fr) auto; }` pasa a `grid-template-columns: var(--tap-target) minmax(0, 1fr) auto;`. No toques la versión `36px` de la media query.
- `width: 44px;` de `.task-toggle-button, .icon-action` y de `.add-mark`, y `height: 44px;` de `.add-mark`, pasan a `var(--tap-target)`.
- `flex-basis: calc(100% - 44px);` de `@media (max-width: 380px)` pasa a `flex-basis: calc(100% - var(--tap-target));`.
- `background: #f7e9e4;` de `.nav-link` y de `.task-toggle-button:hover .completion-icon` pasa a `background: var(--color-primary-soft);`.
- `transition: background-color 120ms ease, border-color 120ms ease;` pasa a `transition: background-color var(--duration-fast) ease, border-color var(--duration-fast) ease;`.
- `transition: opacity 120ms ease;` pasa a `transition: opacity var(--duration-fast) ease;`. No toques `transition: none !important;`.

2.3. Base común de controles. Sustituye la regla `.task-toggle-button, .icon-action { … }` por estas dos reglas, en la misma posición:

```css
.task-toggle-button,
.icon-action,
.button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  min-height: var(--tap-target);
  border-radius: var(--radius);
  cursor: pointer;
  text-decoration: none;
}

.task-toggle-button,
.icon-action {
  position: relative;
  width: var(--tap-target);
  padding: 0;
  font: inherit;
  color: var(--color-muted);
  background: transparent;
  border: 0;
}
```

Deja la regla `.button`, en su posición actual, así:

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

Esta fusión respeta la cascada. Entre la nueva regla compartida y la posición original de `.button`, ninguna regla con especificidad igual o menor asigna `display`, `align-items`, `justify-content`, `flex-shrink`, `min-height`, `border-radius`, `cursor` ni `text-decoration` a elementos `.button`. Ningún elemento combina `.button` con `.task-toggle-button` o `.icon-action`.

2.4. Icono de completado visible. Sustituye `.task-toggle-button:hover .completion-icon svg { opacity: 1; }` por:

```css
.task-toggle-button:hover .completion-icon svg,
.task-completed .completion-icon svg {
  opacity: 1;
}
```

Después borra la regla independiente `.task-completed .completion-icon svg { opacity: 1; }`, que va justo después de `.task-completed .completion-icon { … }`, junto con la línea en blanco que la precede. Cada selector conserva su especificidad, y entre ambas posiciones ninguna regla asigna `opacity` al SVG.

2.5. El diff de `git diff -- public/assets/app.css` debe coincidir con este, salvo los números de los encabezados `@@`. Pasa de 482 a 480 líneas.

```diff
@@ :root
   --color-primary-hover: #992b1e;
+  --color-primary-soft: #f7e9e4;
   --color-focus: #245ab4;
   --color-danger: #a52b25;
   --font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
+  --radius: 6px;
+  --tap-target: 44px;
+  --duration-fast: 120ms;
 }
@@ .app-brand
-  min-height: 44px;
+  min-height: var(--tap-target);
@@ .nav-link
-  min-height: 44px;
+  min-height: var(--tap-target);
   padding: 0.55rem 0.7rem;
-  border-radius: 6px;
-  background: #f7e9e4;
+  border-radius: var(--radius);
+  background: var(--color-primary-soft);
@@ .task
-  grid-template-columns: 44px minmax(0, 1fr) auto;
+  grid-template-columns: var(--tap-target) minmax(0, 1fr) auto;
@@ controles
 .task-toggle-button,
-.icon-action {
-  position: relative;
+.icon-action,
+.button {
   display: inline-flex;
   align-items: center;
   justify-content: center;
   flex-shrink: 0;
-  width: 44px;
-  min-height: 44px;
+  min-height: var(--tap-target);
+  border-radius: var(--radius);
+  cursor: pointer;
+  text-decoration: none;
+}
+
+.task-toggle-button,
+.icon-action {
+  position: relative;
+  width: var(--tap-target);
   padding: 0;
   font: inherit;
   color: var(--color-muted);
   background: transparent;
   border: 0;
-  border-radius: 6px;
-  cursor: pointer;
-  text-decoration: none;
 }
@@ .completion-icon
-  transition: background-color 120ms ease, border-color 120ms ease;
+  transition: background-color var(--duration-fast) ease, border-color var(--duration-fast) ease;
@@ .task-toggle-button:hover .completion-icon
-  background: #f7e9e4;
+  background: var(--color-primary-soft);
@@ svg visible
-.task-toggle-button:hover .completion-icon svg {
+.task-toggle-button:hover .completion-icon svg,
+.task-completed .completion-icon svg {
   opacity: 1;
 }
@@ tras .task-completed .completion-icon
-.task-completed .completion-icon svg {
-  opacity: 1;
-}
-
@@ .add-mark
-  width: 44px;
-  height: 44px;
+  width: var(--tap-target);
+  height: var(--tap-target);
@@ input[type="text"]
-  min-height: 44px;
+  min-height: var(--tap-target);
@@
-  border-radius: 6px;
+  border-radius: var(--radius);
@@ .button
 .button {
-  display: inline-flex;
-  align-items: center;
-  justify-content: center;
-  flex-shrink: 0;
-  min-height: 44px;
   padding: 0.6rem 0.9rem;
   border: 1px solid #dadad4;
-  border-radius: 6px;
   color: #41413d;
@@
   line-height: 1.5;
-  text-decoration: none;
   white-space: nowrap;
-  cursor: pointer;
 }
@@ .field-error
-  border-radius: 6px;
+  border-radius: var(--radius);
@@ .flash
-  border-radius: 6px;
+  border-radius: var(--radius);
@@ .back-link
-  min-height: 44px;
+  min-height: var(--tap-target);
@@ @media (hover: hover) and (pointer: fine)
-    transition: opacity 120ms ease;
+    transition: opacity var(--duration-fast) ease;
@@ @media (max-width: 380px)
-    flex-basis: calc(100% - 44px);
+    flex-basis: calc(100% - var(--tap-target));
```

### 3. Comprobaciones estáticas tras la edición

```bash
grep -nE '(^|[^0-9])6px' public/assets/app.css   # solo: --radius: 6px;
grep -n '44px' public/assets/app.css              # solo: --tap-target: 44px;
grep -n '#f7e9e4' public/assets/app.css           # solo: --color-primary-soft: #f7e9e4;
grep -n '120ms' public/assets/app.css             # solo: --duration-fast: 120ms;
grep -c 'opacity: 1;' public/assets/app.css       # 3 (antes 4)
wc -l < public/assets/app.css                     # 480
git status --short                                # solo public/assets/app.css y plans/issue-2-refactorizar-css.md
```

### 4. Valida y limpia

Ejecuta las validaciones de la sección siguiente. Después:

```bash
(cd /tmp/css-issue-2 && playwright-cli -s=css-issue-2 close)
lsof -nP -iTCP:3100 -iTCP:3199 -sTCP:LISTEN   # identifica los PID de la app (go run / binario app) y de http.server
kill <pid-app-3100> <pid-go-run> <pid-http-server-3199>
pgrep -fl 'go run ./cmd/app|http.server 3199'   # no debe quedar ninguno de los lanzados en el paso 1.3
rm -rf /tmp/css-issue-2
```

Si en el paso 1.4 creaste tareas `Validación CSS …`, elimínalas desde la UI. No detengas los contenedores de `docker compose` ni otros procesos que no lanzaste tú.

## Tests y validación

**Antes del cambio**, en el paso 1.5, el comparador debe devolver `{"comprobaciones":26,"diferencias":0,"muestra":[]}` con `after.css` idéntico a `before.css`. Esto establece que el arnés es determinista en este entorno.

**Después del cambio**, ejecuta lo siguiente con los servicios del paso 1.3 todavía activos. `after.css` es un enlace al archivo editado y el comparador inyecta el CSS directamente, así que no hace falta reiniciar la app.

```bash
(cd /tmp/css-issue-2 && playwright-cli -s=css-issue-2 run-code --filename=/tmp/css-issue-2/compare.js > run-after.txt 2>&1; sed -n 2p run-after.txt)
```

Criterio de aceptación: exactamente `{"comprobaciones":26,"diferencias":0,"muestra":[]}`. Las comprobaciones cubren:

- índice con tareas pendiente y completada a 1280, 700 y 360 px;
- hover en `.nav-link`, en el botón de completar (pendiente y completada), en editar, en eliminar y en «Añadir»;
- foco con teclado;
- mensaje flash y estado vacío a tres anchos;
- error de validación en el índice a tres anchos;
- página de edición a tres anchos;
- hover en «Volver», «Cancelar» y «Guardar cambios»;
- error de validación en edición a tres anchos;
- `prefers-reduced-motion: reduce`.

Si aparece cualquier diferencia, la `muestra` indica el estado, el elemento y la propiedad. Corrige la edición que la provoca hasta volver a 0. No ajustes el comparador para ocultarla.

**Validaciones del repositorio**, con el Postgres de `docker compose` activo. La base `todo_test` existe y está migrada, y se verificó en verde sobre la base durante la planificación.

```bash
gofmt -l .                       # salida vacía
go vet ./...                     # sin errores
GO_ENV=test go test -p 1 ./...   # ok en todo/actions, todo/adws y todo/models
go build ./...                   # sin errores; confirma que public/ sigue embebiéndose
```

No se ejecuta `docker build -t todo .` localmente. El cambio solo afecta a un recurso estático que `go build` ya embebe, y CI lo construye en el pull request.

## Riesgos y recuperación

- **Regresión de cascada al fusionar reglas.** El comparador la detecta como diferencia en el estado y elemento afectados. Para recuperarte, deshaz solo ese paso de la sección 2 o restaura todo con `git checkout "$(git merge-base HEAD origin/main)" -- public/assets/app.css` y repite.
- **Compatibilidad de navegadores.** La hoja ya usa `var()` en más de 20 declaraciones. `var()` dentro de `calc()`, `grid-template-columns` y `transition` tiene el mismo soporte, así que no hay requisitos nuevos.
- **Acoplamiento futuro.** `--tap-target` gobierna ahora 11 usos, incluidos la columna del botón de completar y el `flex-basis` del campo en pantallas de 380 px o menos. Es intencionado: todos derivan del objetivo táctil de 44px. Quien cambie el token cambiará todos esos tamaños a la vez.
- **Caché.** No se cambia `?v=3`. Como los estilos calculados son idénticos, una copia en caché de la versión anterior se ve igual.
- **Cobertura del comparador.** Solo se usa Chromium, y no se cubren `:active`, `::placeholder`, impresión ni foco en cada control. Las reglas de esos casos (`input::placeholder`, `a:focus-visible, button:focus-visible, input:focus-visible`) no cambian textualmente, y el radio, que afecta al contorno de foco, sí se compara en todos los elementos.
- **Efectos sobre datos de desarrollo.** El comparador no crea, edita ni borra tareas. Si el paso 1.4 obligó a crear tareas, el paso 4 las elimina.
- **Puertos o herramientas no disponibles.** Si `playwright-cli`, `python3` o Go no están disponibles, o la app no responde en `/healthz`, la validación visual no puede ejecutarse. En ese caso, detente e informa: sin ella no hay evidencia de que el comportamiento observable se conserva.
