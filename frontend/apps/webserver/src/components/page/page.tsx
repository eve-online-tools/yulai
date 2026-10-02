import type { ReactNode } from "react";
import { Panel } from "@xaroth.nl/design/react";
import { Frame } from "@yulai/ui";
import styles from "./page.module.scss";

// Browser tab, so no window controls in the title bar.
export function Page({ children }: { children: ReactNode }) {
  return (
    <Frame>
      <main className={styles.page}>
        <Panel variant="raised" marks padding="lg" className={styles.panel}>
          {children}
        </Panel>
      </main>
    </Frame>
  );
}
