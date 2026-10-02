import type { HTMLAttributes, ReactNode } from "react";
import styles from "./frame.module.scss";

type FrameProps = HTMLAttributes<HTMLDivElement> & {
  titlebarClassName?: string;
  // End of the title bar, e.g. window controls.
  end?: ReactNode;
};

// Backdrop and brand title bar shared by the app window and the webserver pages.
// The bar height and left inset come from --titlebar-h and --titlebar-inset.
export function Frame({ className, titlebarClassName, end, children, ...rest }: FrameProps) {
  return (
    <div className={className ? `${styles.frame} ${className}` : styles.frame} {...rest}>
      <header className={titlebarClassName ? `${styles.titlebar} ${titlebarClassName}` : styles.titlebar}>
        <div className={styles.brand}>
          <span className={styles.mark} aria-hidden="true" />
          Yulai
        </div>
        {end}
      </header>
      <div className={styles.body}>{children}</div>
    </div>
  );
}
