import { useQuery } from "@tanstack/react-query";
import type { Progress as TaskProgress } from "@bindings/github.com/eve-online-tools/yulai/core/task";
import { Progress } from "@xaroth.nl/design/react";
import { progressQuery } from "../../queries";
import styles from "./data-update.module.scss";

// task.WithProgress key of the SDE Update task.
const key = "sde.update";

const phases: Record<string, string> = {
  download: "Downloading",
  build: "Building",
  index: "Indexing",
};

// Bottom bar from the update's start to its done event. The backend reports one
// bar over the whole update, so it only moves forward.
export function DataUpdate() {
  const { data: p } = useQuery(progressQuery(key));
  if (!p) return null;
  return (
    <div className={styles.bar} role="status">
      <div className={styles.text}>
        <span>Updating data</span>
        {p.phase && <span className="muted small">{describe(p)}</span>}
      </div>
      <Progress label="Updating data" size="sm" value={p.done} max={p.total || 1} showValue />
    </div>
  );
}

function describe(p: TaskProgress) {
  const phase = phases[p.phase] ?? p.phase;
  return p.item ? `${phase} ${p.item}` : phase;
}
