import { useId } from "react";
import { Link, type LinkProps } from "@tanstack/react-router";
import { Tooltip } from "@xaroth.nl/design/react";
import { AppIcon, type AppIconName } from "@yulai/ui";
import styles from "./nav-item.module.scss";

type Props = { to: LinkProps["to"]; icon: AppIconName; label: string };

// For sidebar entries that are not links, like the getting started summary.
export const navItemClass = styles.item;
export const navItemCollapsedClass = styles.collapsed;
export const navItemLabelClass = styles.label;

// Collapsed, only the icon shows and the label moves into a tooltip.
export function NavItem({ to, icon, label, collapsed }: Props & { collapsed?: boolean }) {
  const id = useId();
  const link = (
    <Link
      to={to}
      className={collapsed ? `${styles.item} ${styles.collapsed}` : styles.item}
      activeProps={{ className: styles.active, "aria-current": "page" }}
      aria-label={collapsed ? label : undefined}
    >
      <AppIcon name={icon} size={18} />
      {!collapsed && <span className={styles.label}>{label}</span>}
    </Link>
  );
  if (!collapsed) return link;
  return (
    <Tooltip id={id} text={label} className={styles.tip}>
      {link}
    </Tooltip>
  );
}

// Small icon-only entry for the sidebar foot.
export function NavIcon({ to, icon, label }: Props) {
  const id = useId();
  return (
    <Tooltip id={id} text={label}>
      <Link to={to} className={styles.icon} activeProps={{ className: styles.active, "aria-current": "page" }} aria-label={label}>
        <AppIcon name={icon} size={14} />
      </Link>
    </Tooltip>
  );
}
