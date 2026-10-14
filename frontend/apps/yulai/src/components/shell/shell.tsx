import type { CSSProperties } from "react";
import { Outlet, useMatches } from "@tanstack/react-router";
import { presenceFeature, skillsFeature, useFeature } from "../../features";
import { DataUpdate } from "../data-update";
import { GettingStarted } from "../getting-started";
import { NavIcon, NavItem } from "../nav-item";
import { useSidebarWidth } from "./use-sidebar-width";
import styles from "./shell.module.scss";

// App shell inside the frame: tool navigation on the left, the page on the right.
export function Shell() {
  const { width, collapsed, handleProps } = useSidebarWidth();
  const skills = useFeature(skillsFeature);
  const presence = useFeature(presenceFeature);
  const fullBleed = useMatches({ select: (matches) => matches.some((m) => m.staticData.fullBleed) });
  return (
    <div className={styles.shell} style={{ "--sidebar-w": `${width}px` } as CSSProperties}>
      <aside className={styles.sidebar} data-collapsed={collapsed || undefined}>
        <nav className={styles.nav} aria-label="Tools">
          <NavItem to="/overview" icon="user" label="Overview" collapsed={collapsed} />
          <NavItem
            to="/map"
            icon="map"
            label="New Eden"
            collapsed={collapsed}
            disabled={presence.characters.length === 0 ? `Needs a character with ${presence.name}` : undefined}
          />
          <NavItem
            to="/skills"
            icon="school"
            label="Skills"
            collapsed={collapsed}
            disabled={skills.characters.length === 0 ? `Needs a character with ${skills.name}` : undefined}
          />
          <NavItem
            to="/ship-tree"
            icon="ship"
            label="Ship Tree"
            collapsed={collapsed}
            disabled={skills.characters.length === 0 ? `Needs a character with ${skills.name}` : undefined}
          />
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
        <main className={styles.content} data-full-bleed={fullBleed || undefined}>
          <Outlet />
        </main>
        <DataUpdate />
      </div>
    </div>
  );
}
