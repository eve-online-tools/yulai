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

// Collapsed, only the icon shows and the label moves into a tooltip. Disabled, it is greyed out and the
// tooltip gives the reason.
export function NavItem({ to, icon, label, collapsed, disabled }: Props & { collapsed?: boolean; disabled?: string }) {
  const id = useId();
  if (disabled) {
    return (
      <Tooltip id={id} text={collapsed ? `${label}: ${disabled}` : disabled} placement="right" className={styles.tip}>
        <span
          className={`${styles.item} ${styles.disabled}${collapsed ? ` ${styles.collapsed}` : ""}`}
          aria-disabled="true"
          aria-label={collapsed ? label : undefined}
          tabIndex={0}
        >
          <AppIcon name={icon} size={18} />
          {!collapsed && <span className={styles.label}>{label}</span>}
        </span>
      </Tooltip>
    );
  }
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
    <Tooltip id={id} text={label} placement="right" className={styles.tip}>
      {link}
    </Tooltip>
  );
}

// Small icon-only entry for the sidebar foot. Collapsed, the icons stack, so the tooltip goes to the side.
export function NavIcon({ to, icon, label, collapsed }: Props & { collapsed?: boolean }) {
  const id = useId();
  return (
    <Tooltip id={id} text={label} placement={collapsed ? "right" : "top"}>
      <Link to={to} className={styles.icon} activeProps={{ className: styles.active, "aria-current": "page" }} aria-label={label}>
        <AppIcon name={icon} size={14} />
      </Link>
    </Tooltip>
  );
}
