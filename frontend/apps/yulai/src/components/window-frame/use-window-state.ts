import { useEffect, useState } from "react";
import { System, Window } from "@wailsio/runtime";

type WindowState = { mac: boolean; maximised: boolean; fullscreen: boolean };

// Maximising and fullscreen always resize the viewport, so resize is enough to keep this in sync.
// Runtime calls reject outside a Wails window (server mode); the defaults stand there.
export function useWindowState(): WindowState {
  const [state, setState] = useState<WindowState>({ mac: System.IsMac(), maximised: false, fullscreen: false });

  useEffect(() => {
    const check = () =>
      Promise.all([Window.IsMaximised(), Window.IsFullscreen()]).then(
        ([maximised, fullscreen]) => setState((s) => ({ ...s, maximised, fullscreen })),
        () => {},
      );
    check();
    // IsMac reads a runtime-injected global; confirm in case it landed after first render.
    System.Environment().then((env) => setState((s) => ({ ...s, mac: env.OS === "darwin" })), () => {});
    window.addEventListener("resize", check);
    return () => window.removeEventListener("resize", check);
  }, []);

  return state;
}
