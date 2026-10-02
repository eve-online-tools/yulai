# Design

Look comes from `@xaroth.nl/design` with the `eve-online` theme, loaded once in each app's `main.tsx`. Use design parts (`Button`, `Panel`, `Table`, `PageHead`, ...)
before writing new CSS. App CSS uses the theme's `--x-*` tokens.

## Files

Components and pages are kebab-case folders: `components/nav-item/nav-item.tsx`, its styles in
`nav-item.module.scss` and exports in `index.ts`. Pages follow the same shape under `pages/`. Only small global
utilities (`.muted`, `.row`, ...) live in `@yulai/ui/styles.css`.

Components used by more than one app (`apps/yulai`, `apps/webserver`) live in `@yulai/ui` in the same shape:
`Frame`, `AppIcon`, `Portrait`. Anything that needs Wails, bindings or the router stays in the app.

## Layers

| Layer | Where | What |
| --- | --- | --- |
| Frame | `components/window-frame` (on `@yulai/ui` `Frame`), root route | Backdrop, title bar, window controls. Every page. |
| Shell | `components/shell`, `layout` route | Sidebar (tools on top; checklist, characters, settings in the foot) and the content area. |
| Page | `pages/*` | Content. `/welcome` mounts straight in the frame, without the shell. |

## Window chrome

The frame draws the title bar on every platform. `@yulai/ui` `Frame` has the backdrop, title bar and brand;
`window-frame` adds the window edge, drag region, controls and the macOS sizing through `--titlebar-h` and
`--titlebar-inset`. `useWindowState` reports platform, maximised and fullscreen; the
frame exposes them as `data-platform`, `data-maximised` and `data-fullscreen`.

Windows and Linux: `Frameless: true`, with `components/window-controls` for minimise, maximise and close.

- Drag: elements with `--wails-draggable: drag` move the window (the title bar). Interactive children set `no-drag`.
- Resize: Wails handles edge resizing on Windows and on Linux when frameless.
- Controls: `Window.Minimise`, `Window.ToggleMaximise`, `Window.Close` from `@wailsio/runtime`. Maximised state is
  re-read on `resize` and drops the window edge.
- Windows keeps the shadow and rounded corners unless `DisableFramelessWindowDecorations` is set.

macOS: not frameless. A transparent title bar with a unified compact toolbar keeps the native traffic lights over the
webview. The frame hides its own controls, grows the bar to 38px and pads it 78px on the left for the lights, except in
fullscreen where macOS hides them. Double-click on the bar zooms through the Wails drag handler.

## Webserver pages

`apps/webserver` holds the pages of the loopback server (`identity/login`), which open in the system browser. They
use the same `Frame` without window controls, centred in a raised `Panel`. The server inlines page data as JSON in
the `#page-data` element of the built `index.html`; `src/page-data.ts` picks the page from it. Its CSP allows only
same-origin scripts, styles and fonts, so the build does not inline assets.
