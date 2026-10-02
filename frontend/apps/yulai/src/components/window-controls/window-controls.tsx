import { Window } from "@wailsio/runtime";
import styles from "./window-controls.module.scss";

// Runtime calls reject outside a Wails window (server mode); nothing to do there.
const ignore = () => {};

// Minimise, maximise and close for frameless windows (Windows, Linux).
export function WindowControls({ maximised }: { maximised: boolean }) {
  return (
    <div className={styles.controls}>
      <button type="button" className={styles.button} aria-label="Minimise" onClick={() => Window.Minimise().catch(ignore)}>
        <Glyph d="M1 6h10" />
      </button>
      <button
        type="button"
        className={styles.button}
        aria-label={maximised ? "Restore" : "Maximise"}
        onClick={() => Window.ToggleMaximise().catch(ignore)}
      >
        <Glyph d={maximised ? "M3 3V1h8v8H9M1 3h8v8H1z" : "M1.5 1.5h9v9h-9z"} />
      </button>
      <button type="button" className={`${styles.button} ${styles.close}`} aria-label="Close" onClick={() => Window.Close().catch(ignore)}>
        <Glyph d="M1.5 1.5l9 9M10.5 1.5l-9 9" />
      </button>
    </div>
  );
}

function Glyph({ d }: { d: string }) {
  return (
    <svg viewBox="0 0 12 12" width="12" height="12" fill="none" stroke="currentColor" strokeWidth="1" aria-hidden="true">
      <path d={d} />
    </svg>
  );
}
