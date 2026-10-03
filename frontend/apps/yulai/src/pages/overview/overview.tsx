import { useQuery, useSuspenseQuery, useMutation } from "@tanstack/react-query";
import type { FeatureInfo, ListRow as Character } from "@bindings/github.com/eve-online-tools/yulai/feature/character";
import { Service as Sync } from "@bindings/github.com/eve-online-tools/yulai/feature/sync";
import { Button, EmptyState, PageHead, Panel, Tag } from "@xaroth.nl/design/react";
import { Portrait, enabledFeatures } from "@yulai/ui";
import { charactersQuery, featuresQuery, syncJobsQuery } from "../../queries";
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

// Feature panels (one per feature that has something to show) go between the tags and the jobs.
function CharacterCard({ character: c, features }: { character: Character; features: FeatureInfo[] }) {
  const { data: jobs = [] } = useQuery(syncJobsQuery);
  const runNow = useMutation({ mutationFn: (job: string) => Sync.RunNow(c.id, job) });

  const enabled = enabledFeatures(features, c.scopes).map((f) => f.name);
  const disabled = features.filter((f) => !enabled.includes(f.name)).map((f) => f.name);
  const myJobs = jobs.filter((j) => j.characterId === c.id);

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
        <summary className="muted">Sync jobs ({myJobs.length})</summary>
        <dl>
          {myJobs.map((j) => (
            <JobRow key={j.job} job={j.job} nextRun={j.nextRun} lastError={j.lastError} onRun={() => runNow.mutate(j.job)} />
          ))}
        </dl>
      </details>
    </Panel>
  );
}

function JobRow({ job, nextRun, lastError, onRun }: { job: string; nextRun: number | null; lastError: string | null; onRun: () => void }) {
  const label = lastError ? "error" : nextRun ? `next ${new Date(nextRun * 1000).toLocaleTimeString()}` : "paused";
  return (
    <>
      <dt>{job}</dt>
      <dd>
        <span className={lastError ? "error" : "muted"} title={lastError ?? ""}>{label}</span>{" "}
        <Button variant="tertiary" size="sm" onClick={onRun}>sync now</Button>
      </dd>
    </>
  );
}
