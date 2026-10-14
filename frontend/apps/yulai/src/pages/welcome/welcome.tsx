import { useEffect } from "react";
import { useMutation, useSuspenseQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import type { ListRow as Character } from "@bindings/github.com/eve-online-tools/yulai/feature/character";
import type { Status } from "@bindings/github.com/eve-online-tools/yulai/core/task";
import { Consent } from "@bindings/github.com/eve-online-tools/yulai/identity/login";
import { Alert, Button, Panel } from "@xaroth.nl/design/react";
import { Portrait } from "@yulai/ui";
import { DataUpdate } from "../../components/data-update";
import { characterTasks, charactersQuery, hasRun, sdeQuery, setupQuery, tasksQuery } from "../../queries";
import styles from "./welcome.module.scss";

const steps = ["Add a character", "Synchronizing", "Complete"];

// Covers only what blocks using the app. Optional steps live in the sidebar checklist.
export function WelcomePage() {
  const { data: setup } = useSuspenseQuery(setupQuery);
  const { data: characters } = useSuspenseQuery(charactersQuery);
  const { data: tasks } = useSuspenseQuery(tasksQuery);
  const { data: sde } = useSuspenseQuery(sdeQuery);
  const navigate = useNavigate();

  const first = characters[0];
  const myTasks = first ? characterTasks(tasks, first.id) : [];
  const synced = myTasks.filter(hasRun).length;
  const sdeInstalled = sde.build > 0;
  // No tasks yet means the login's refresh has not dispatched them.
  const current = !first ? 0 : myTasks.length === 0 || synced < myTasks.length || !sdeInstalled ? 1 : 2;

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
            {current === 1 && <Syncing character={first} tasks={myTasks} sdeInstalled={sdeInstalled} sdeError={sde.lastError} />}
          </div>
        </Panel>
      </div>
      <DataUpdate />
    </div>
  );
}

function AddFirst({ loginUrl }: { loginUrl: string }) {
  const add = useMutation({ mutationFn: () => Consent.AddCharacter() });
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

type SyncingProps = {
  character: Character;
  tasks: Status[];
  sdeInstalled: boolean;
  sdeError: string;
};

function Syncing({ character, tasks, sdeInstalled, sdeError }: SyncingProps) {
  const failed = tasks.find((t) => t.lastError);
  return (
    <>
      <div className="row">
        <Portrait id={character.id} size={48} />
        <span>Welcome, {character.name}. Fetching your character data and the static game data.</span>
      </div>
      {!sdeInstalled && <p className="muted small">Static data is downloading, see progress below.</p>}
      {failed && <Alert tone="warning">{failed.task}: {failed.lastError}</Alert>}
      {!sdeInstalled && sdeError && <Alert tone="warning">Static data: {sdeError}</Alert>}
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
