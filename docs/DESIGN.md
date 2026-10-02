# Design

Look comes from `@xaroth.nl/design` with the `eve-online` theme, loaded once in `main.tsx`. Use design parts (`Button`, `Panel`, `Table`, `PageHead`, ...)
before writing new CSS. App CSS uses the theme's `--x-*` tokens.

## Files

Components and pages are kebab-case folders: `components/nav-item/nav-item.tsx`, its styles in
`nav-item.module.scss` and exports in `index.ts`. Pages follow the same shape under `pages/`. Only small global
utilities (`.muted`, `.row`, ...) live in `@yulai/ui/styles.css`.

## Layers

| Layer | Where | What |
| --- | --- | --- |
| Frame | `components/frame`, root route | Backdrop, title bar, window controls. Every page. |
| Shell | `components/shell`, `layout` route | Sidebar (tools on top; checklist, accounts, settings in the foot) and the content area. |
| Page | `pages/*` | Content. `/welcome` mounts straight in the frame, without the shell. |

## Window chrome

The frame draws the title bar on every platform. `useWindowState` reports platform, maximised and fullscreen; the
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

## Login pages

The loopback login pages (`identity/login/pages.html`) open in the system browser and should match the frame layer:
same backdrop, a brand bar without window controls, and design tokens. Not done yet: they carry their own inline CSS.
