import type { CSSProperties } from "react";
import { Outlet } from "@tanstack/react-router";
import { DataUpdate } from "../data-update";
import { GettingStarted } from "../getting-started";
import { NavIcon, NavItem } from "../nav-item";
import { useSidebarWidth } from "./use-sidebar-width";
import styles from "./shell.module.scss";

// App shell inside the frame: tool navigation on the left, the page on the right.
export function Shell() {
  const { width, collapsed, handleProps } = useSidebarWidth();
  return (
    <div className={styles.shell} style={{ "--sidebar-w": `${width}px` } as CSSProperties}>
      <aside className={styles.sidebar} data-collapsed={collapsed || undefined}>
        <nav className={styles.nav} aria-label="Tools">
          <NavItem to="/overview" icon="user" label="Overview" collapsed={collapsed} />
        </nav>
        <div className={styles.foot}>
          <GettingStarted collapsed={collapsed} />
          <div className={styles.icons}>
            <NavIcon to="/characters" icon="users" label="Characters" collapsed={collapsed} />
            <NavIcon to="/settings" icon="settings" label="Settings" collapsed={collapsed} />
          </div>
        </div>
        <div className={styles.handle} {...handleProps} />
      </aside>
      <div className={styles.main}>
        <main className={styles.content}>
          <Outlet />
        </main>
        <DataUpdate />
      </div>
    </div>
  );
}
