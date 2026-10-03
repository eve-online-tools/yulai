import { useQuery } from "@tanstack/react-query";
import type { Progress as TaskProgress } from "@bindings/github.com/eve-online-tools/yulai/core/task";
import { Progress } from "@xaroth.nl/design/react";
import { sdeQuery } from "../../queries";
import styles from "./data-update.module.scss";

const phases: Record<string, string> = {
  download: "Downloading",
  build: "Building",
  index: "Indexing",
};

// Bottom bar while static data updates. Renders nothing otherwise. The backend
// reports one bar over the whole update, so it only moves forward.
export function DataUpdate() {
  const { data: status } = useQuery(sdeQuery);
  if (!status?.updating) return null;
  const p = status.progress;
  return (
    <div className={styles.bar} role="status">
      <div className={styles.text}>
        <span>Updating data</span>
        {p && <span className="muted small">{describe(p)}</span>}
      </div>
      <Progress label="Updating data" size="sm" value={p?.done ?? 0} max={p?.total || 1} showValue />
    </div>
  );
}

function describe(p: TaskProgress) {
  const phase = phases[p.phase] ?? p.phase;
  return p.item ? `${phase} ${p.item}` : phase;
}
