import { Outlet } from "@tanstack/react-router";
import { Frame } from "@yulai/ui";
import { WindowControls } from "../window-controls";
import { useWindowState } from "./use-window-state";
import styles from "./window-frame.module.scss";

// Base layer for every page: backdrop, title bar, window controls.
// Windows and Linux are frameless and get our controls. macOS keeps its native traffic lights over a
// transparent title bar (main.go), so the bar leaves room for them instead.
export function WindowFrame() {
  const { mac, maximised, fullscreen } = useWindowState();
  return (
    <Frame
      className={styles.window}
      titlebarClassName={styles.titlebar}
      data-platform={mac ? "mac" : undefined}
      data-maximised={maximised || undefined}
      data-fullscreen={fullscreen || undefined}
      end={!mac && <WindowControls maximised={maximised} />}
    >
      <Outlet />
    </Frame>
  );
}
