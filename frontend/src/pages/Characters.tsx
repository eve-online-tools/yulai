import { useQuery, useSuspenseQuery, useMutation } from "@tanstack/react-query";
import type { FeatureInfo, ListRow as Character } from "../../bindings/github.com/eve-online-tools/yulai/feature/character";
import { Service as Sync } from "../../bindings/github.com/eve-online-tools/yulai/feature/sync";
import { charactersQuery, featuresQuery, syncJobsQuery } from "../queries";

export function CharactersPage() {
  const { data: characters } = useSuspenseQuery(charactersQuery);
  const { data: features = [] } = useQuery(featuresQuery);

  return (
    <>
      <header className="page-header">
        <h1>Characters</h1>
      </header>
      {characters.length === 0 && <p className="muted">No characters yet. Add one from the accounts page.</p>}
      <div className="cards">
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

  const have = new Set(c.scopes.split(" ").filter(Boolean));
  const enabled = features.filter((f) => f.scopes.every((s) => have.has(s))).map((f) => f.name);
  const disabled = features.filter((f) => !enabled.includes(f.name)).map((f) => f.name);
  const myJobs = jobs.filter((j) => j.characterId === c.id);

  return (
    <div className="card">
      <div className="card-head">
        <img src={`https://images.evetech.net/characters/${c.id}/portrait?size=64`} alt="" width={48} height={48} />
        <div>
          <div className="name">{c.name}</div>
          <div className="muted small">{c.id}</div>
        </div>
      </div>

      {c.status !== "ok" && <p className="warn">Needs login again. {c.statusError}</p>}

      <div className="row small">
        {enabled.map((n) => (
          <span key={n} className="tag on">{n}</span>
        ))}
        {disabled.map((n) => (
          <span key={n} className="tag" title="Log in again with this feature checked to enable it">{n}</span>
        ))}
      </div>

      <details className="small">
        <summary className="muted">Sync jobs ({myJobs.length})</summary>
        <dl>
          {myJobs.map((j) => (
            <JobRow key={j.job} job={j.job} nextRun={j.nextRun} lastError={j.lastError} onRun={() => runNow.mutate(j.job)} />
          ))}
        </dl>
      </details>
    </div>
  );
}

function JobRow({ job, nextRun, lastError, onRun }: { job: string; nextRun: number | null; lastError: string | null; onRun: () => void }) {
  const label = lastError ? "error" : nextRun ? `next ${new Date(nextRun * 1000).toLocaleTimeString()}` : "paused";
  return (
    <>
      <dt>{job}</dt>
      <dd>
        <span className={lastError ? "error" : "muted"} title={lastError ?? ""}>{label}</span>{" "}
        <button className="ghost" onClick={onRun}>sync now</button>
      </dd>
    </>
  );
}
