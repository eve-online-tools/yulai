import { Outlet } from "@tanstack/react-router";
import { GettingStarted } from "../getting-started";
import { NavItem } from "../nav-item";
import styles from "./shell.module.scss";

// App shell inside the frame: tool navigation on the left, the page on the right.
export function Shell() {
  return (
    <div className={styles.shell}>
      <aside className={styles.sidebar}>
        <nav className={styles.nav} aria-label="Tools">
          <p className={styles.label}>Tools</p>
          <NavItem to="/characters" icon="user">
            Characters
          </NavItem>
        </nav>
        <div className={styles.foot}>
          <GettingStarted />
          <NavItem to="/accounts" icon="users">
            Accounts
          </NavItem>
          <NavItem to="/settings" icon="settings">
            Settings
          </NavItem>
        </div>
      </aside>
      <main className={styles.content}>
        <Outlet />
      </main>
    </div>
  );
}
