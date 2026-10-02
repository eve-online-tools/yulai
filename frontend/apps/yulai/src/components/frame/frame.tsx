import { Outlet } from "@tanstack/react-router";
import { WindowControls } from "../window-controls";
import { useWindowState } from "./use-window-state";
import styles from "./frame.module.scss";

// Base layer for every page: backdrop, title bar, window controls.
// Windows and Linux are frameless and get our controls. macOS keeps its native traffic lights over a
// transparent title bar (main.go), so the bar leaves room for them instead.
export function Frame() {
  const { mac, maximised, fullscreen } = useWindowState();
  return (
    <div
      className={styles.frame}
      data-platform={mac ? "mac" : undefined}
      data-maximised={maximised || undefined}
      data-fullscreen={fullscreen || undefined}
    >
      <header className={styles.titlebar}>
        <div className={styles.brand}>
          <span className={styles.mark} aria-hidden="true" />
          Yulai
        </div>
        {!mac && <WindowControls maximised={maximised} />}
      </header>
      <div className={styles.body}>
        <Outlet />
      </div>
    </div>
  );
}
