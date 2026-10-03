import { useEffect } from "react";
import { useMutation, useSuspenseQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Service as Characters, type ListRow as Character } from "@bindings/github.com/eve-online-tools/yulai/feature/character";
import type { SyncJob } from "@bindings/github.com/eve-online-tools/yulai/feature/sync";
import { Alert, Button, Panel, Progress } from "@xaroth.nl/design/react";
import { Portrait } from "@yulai/ui";
import { DataUpdate } from "../../components/data-update";
import { charactersQuery, setupQuery, syncJobsQuery } from "../../queries";
import styles from "./welcome.module.scss";

const steps = ["Add a character", "Synchronizing", "Complete"];

// Covers only what blocks using the app. Optional steps live in the sidebar checklist.
export function WelcomePage() {
  const { data: setup } = useSuspenseQuery(setupQuery);
  const { data: characters } = useSuspenseQuery(charactersQuery);
  const { data: jobs } = useSuspenseQuery(syncJobsQuery);
  const navigate = useNavigate();

  const first = characters[0];
  const myJobs = first ? jobs.filter((j) => j.characterId === first.id) : [];
  const synced = myJobs.filter((j) => j.lastRun != null).length;
  const current = !first ? 0 : synced < myJobs.length ? 1 : 2;

  useEffect(() => {
    if (current === 2) navigate({ to: "/overview", replace: true });
  }, [current, navigate]);

  return (
    <div className={styles.page}>
      <div className={styles.welcome}>
        <Panel variant="raised" marks padding="lg" className={styles.panel}>
          <p className={styles.eyebrow}>First run</p>
          <h1>Welcome to Yulai</h1>
          <ol className={styles.stepper}>
            {steps.map((title, i) => (
              <li key={title} className={styles.step} data-state={i < current ? "done" : i === current ? "active" : "todo"}>
                <span className={styles.n}>{i < current ? "✓" : i + 1}</span>
                <span className={styles.title}>{title}</span>
              </li>
            ))}
          </ol>
          <div className={styles.content}>
            {current === 0 && <AddFirst loginUrl={setup.loginUrl} />}
            {current === 1 && <Syncing character={first} jobs={myJobs} synced={synced} />}
          </div>
          <p className="muted small">
            Login runs in your own browser, so Yulai never sees your password. Tokens are stored encrypted on this machine.
          </p>
        </Panel>
      </div>
      <DataUpdate />
    </div>
  );
}

function AddFirst({ loginUrl }: { loginUrl: string }) {
  const add = useMutation({ mutationFn: () => Characters.AddCharacter() });
  return (
    <>
      <p className="muted small">Pick the features you want, then log in with EVE SSO in your browser.</p>
      <div className="row">
        <Button onClick={() => add.mutate()} loading={add.isPending}>
          Add character
        </Button>
        {add.isSuccess && <span className="muted small">Waiting for login in your browser...</span>}
      </div>
      {add.isError && <Alert tone="danger">{String(add.error)}</Alert>}
      {(add.isSuccess || add.isError) && (
        <p className="muted small">
          Browser did not open? Go to <Copyable text={loginUrl} />
        </p>
      )}
    </>
  );
}

function Syncing({ character, jobs, synced }: { character: Character; jobs: SyncJob[]; synced: number }) {
  const failed = jobs.find((j) => j.lastError);
  return (
    <>
      <div className="row">
        <Portrait id={character.id} size={48} />
        <span>Welcome, {character.name}. Loading your character data.</span>
      </div>
      <Progress label="Synchronizing" value={synced} max={jobs.length} showValue />
      {failed && <Alert tone="warning">{failed.job}: {failed.lastError}</Alert>}
    </>
  );
}

function Copyable({ text }: { text: string }) {
  return (
    <code className={styles.copyable} title="Click to copy" onClick={() => navigator.clipboard.writeText(text)}>
      {text}
    </code>
  );
}
