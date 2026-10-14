import { useQuery, useSuspenseQuery } from "@tanstack/react-query";
import type { ListRow as Character } from "@bindings/github.com/eve-online-tools/yulai/feature/character";
import type { Status } from "@bindings/github.com/eve-online-tools/yulai/core/task";
import type { Feature } from "@bindings/github.com/eve-online-tools/yulai/identity/login";
import { EmptyState, PageHead, Panel, Tag } from "@xaroth.nl/design/react";
import { Portrait, enabledFeatures } from "@yulai/ui";
import { characterTasks, charactersQuery, featuresQuery, hasRun, tasksQuery } from "../../queries";
import styles from "./overview.module.scss";

export function OverviewPage() {
  const { data: characters } = useSuspenseQuery(charactersQuery);
  const { data: features = [] } = useQuery(featuresQuery);

  return (
    <>
      <PageHead title="Overview" />
      {characters.length === 0 && <EmptyState title="No characters yet">Add one from the characters page.</EmptyState>}
      <div className={styles.cards}>
        {characters.map((c) => (
          <CharacterCard key={c.id} character={c} features={features} />
        ))}
      </div>
    </>
  );
}

// Feature panels (one per feature that has something to show) go between the tags and the tasks.
function CharacterCard({ character: c, features }: { character: Character; features: Feature[] }) {
  const { data: tasks = [] } = useQuery(tasksQuery);

  const enabled = enabledFeatures(features, c.scopes).map((f) => f.name);
  const disabled = features.filter((f) => !enabled.includes(f.name)).map((f) => f.name);
  const myTasks = characterTasks(tasks, c.id);

  return (
    <Panel className={styles.card}>
      <div className={styles.head}>
        <Portrait id={c.id} size={48} />
        <div>
          <div className={styles.name}>{c.name}</div>
          <div className="muted small">{c.id}</div>
        </div>
      </div>

      {c.status !== "ok" && <p className="warn">Needs login again. {c.statusError}</p>}

      <div className="row">
        {enabled.map((n) => (
          <Tag key={n} active>{n}</Tag>
        ))}
        {disabled.map((n) => (
          <Tag key={n} title="Log in again with this feature checked to enable it">{n}</Tag>
        ))}
      </div>

      <details className={`small ${styles.jobs}`}>
        <summary className="muted">Tasks ({myTasks.length})</summary>
        <dl>
          {myTasks.map((t) => (
            <TaskRow key={t.task} task={t} />
          ))}
        </dl>
      </details>
    </Panel>
  );
}

function TaskRow({ task: t }: { task: Status }) {
  const label = t.lastError ? "error" : t.running ? "running" : hasRun(t) ? `last ${new Date(t.lastRun).toLocaleTimeString()}` : "pending";
  return (
    <>
      <dt>{t.task}</dt>
      <dd>
        <span className={t.lastError ? "error" : "muted"} title={t.lastError}>{label}</span>
      </dd>
    </>
  );
}
