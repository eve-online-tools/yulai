import type { ReactNode } from "react";
import { Link, type LinkProps } from "@tanstack/react-router";
import { AppIcon, type AppIconName } from "../app-icon";
import styles from "./nav-item.module.scss";

// For sidebar entries that are not links, like the getting started summary.
export const navItemClass = styles.item;

export function NavItem({ to, icon, children }: { to: LinkProps["to"]; icon: AppIconName; children: ReactNode }) {
  return (
    <Link to={to} className={styles.item} activeProps={{ className: `${styles.item} ${styles.active}`, "aria-current": "page" }}>
      <AppIcon name={icon} size={18} />
      <span>{children}</span>
    </Link>
  );
}
