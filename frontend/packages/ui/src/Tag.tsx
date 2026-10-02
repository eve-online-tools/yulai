import type { ReactNode } from "react";

export function Tag({ on, title, children }: { on?: boolean; title?: string; children: ReactNode }) {
  return (
    <span className={on ? "tag on" : "tag"} title={title}>
      {children}
    </span>
  );
}
